import AppKit
import HotseatKit
import ServiceManagement
import SwiftUI

/// Runs `body` on the main queue outside any Swift task. Modal alerts go
/// through here: a runModal() nested inside a Task let AppKit call back
/// into the app mid-task, and the app crashed in Swift's executor check.
@MainActor
private func later(_ body: @escaping @MainActor () -> Void) {
    DispatchQueue.main.async {
        MainActor.assumeIsolated { body() }
    }
}

/// The hotseat CLI shipped inside the app (Contents/Resources/bin).
let bundledCLI = Bundle.main.bundleURL.appendingPathComponent("Contents/Resources/bin/hotseat")

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate, NSMenuDelegate, NSWindowDelegate {
    private let model = AppModel()
    private var statusItem: NSStatusItem!
    private let menu = NSMenu()
    private var gate = RefreshGate()
    private var timer: Timer?
    private var settingsWindow: NSWindow?
    private var setupWindow: NSWindow?
    private let setup = SetupModel()

    /// What to switch, asked for in the open menu and done once it closed.
    private enum Move {
        case both(String)
        case cliOnly(String)
        case desktopOnly(String)
    }
    private var pendingMove: Move?
    private var menuOpen = false
    /// What the current menu items were built for; while the menu is open
    /// they are only updated in place, which keeps its size right.
    private var builtFor = ""
    private var cards: [String: MenuCardView] = [:]
    private var errorItem: NSMenuItem?

    func applicationDidFinishLaunching(_ notification: Notification) {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        statusItem.button?.imagePosition = .imageLeading
        menu.delegate = self
        statusItem.menu = menu
        loadPreferences()
        render()
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated { self?.refresh() }
        }
        Task { await openSetupIfNeeded() }
    }

    private var appVersion: String {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "dev"
    }

    /// Opens Setup by itself once per app version while a step is left.
    private func openSetupIfNeeded() async {
        await reloadSetup()
        let pending = setup.steps.contains { !$0.done }
        if pending && UserDefaults.standard.string(forKey: "setupShownForVersion") != appVersion {
            UserDefaults.standard.set(appVersion, forKey: "setupShownForVersion")
            openSetup()
        }
    }

    private func reloadSetup() async {
        setup.bundledVersion = appVersion
        do {
            let cli = try cli()
            setup.usingBundled = cli.executable.path == bundledCLI.path
            setup.cliPath = cli.executable.path
            setup.cliVersion = try? await cli.version()
            setup.loadError = nil
            do {
                setup.status = try await cli.setupStatus()
            } catch {
                // An older CLI without `setup`: the CLI step offers the update.
                setup.status = nil
            }
        } catch {
            setup.loadError = hotseatError(error).message
        }
    }

    @objc private func openSetup() {
        if setupWindow == nil {
            let view = SetupView(
                model: setup,
                run: { [weak self] step in self?.runSetupStep(step) },
                reload: { [weak self] in Task { await self?.reloadSetup() } },
                close: { [weak self] in later { self?.setupWindow?.close() } })
            let window = NSWindow(contentViewController: NSHostingController(rootView: view))
            window.title = "Hotseat Setup"
            window.styleMask = [.titled, .closable]
            window.isReleasedWhenClosed = false
            window.delegate = self
            window.center()
            setupWindow = window
        }
        NSApp.activate()
        setupWindow?.makeKeyAndOrderFront(nil)
        Task { await reloadSetup() }
    }

    private func runSetupStep(_ step: SetupStep) {
        guard setup.running == nil else { return }
        setup.running = step.id
        setup.errors[step.id] = nil
        Task {
            do {
                if let commands = step.commands {
                    let cli = try cli()
                    for args in commands {
                        try await cli.runStep(args)
                    }
                } else {
                    let dest = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".local/bin/hotseat")
                    try HotseatCLI.installBundled(from: bundledCLI, to: dest)
                }
            } catch {
                setup.errors[step.id] = hotseatError(error).message
            }
            setup.running = nil
            await reloadSetup()
            refresh()
        }
    }

    func menuWillOpen(_ menu: NSMenu) {
        menuOpen = true
        refresh()
    }

    func menuDidClose(_ menu: NSMenu) {
        menuOpen = false
        render()
        if let move = pendingMove {
            pendingMove = nil
            later { self.perform(move) }
        }
    }

    private func cli() throws -> HotseatCLI {
        try HotseatCLI.locate(
            override: UserDefaults.standard.string(forKey: "hotseatPath"),
            home: FileManager.default.homeDirectoryForCurrentUser,
            bundled: bundledCLI.path)
    }

    private func loadPreferences() {
        model.launchAtLogin = SMAppService.mainApp.status == .enabled
        model.askBeforeDesktopRestart = !UserDefaults.standard.bool(forKey: "skipDesktopConfirm")
        model.cliPath = (try? cli().executable.path) ?? ""
    }

    @objc private func refresh() {
        guard gate.request() else { return }
        startRefresh()
    }

    private func startRefresh() {
        Task {
            do {
                model.snapshot = try await cli().snapshot()
                model.lastError = nil
            } catch {
                model.lastError = hotseatError(error)
            }
            render()
            if gate.finish() {
                startRefresh()
            }
        }
    }

    // MARK: - Menu

    private func render() {
        renderStatusButton()
        let key = structureKey()
        if key == builtFor || menuOpen {
            updateInPlace()
        } else {
            rebuild(key)
        }
    }

    private func renderStatusButton() {
        guard let button = statusItem.button else { return }
        let s = model.snapshot
        if model.switching != nil {
            button.image = StatusIcon.switching
            button.title = ""
            return
        }
        if let current = s?.accounts.first(where: { $0.account == s?.current }) {
            button.image = StatusIcon.avatar(icon: current.icon, name: current.displayName)
        } else {
            button.image = StatusIcon.meter(five: nil, week: nil)
        }
        let title = Format.title(s, failed: model.lastError != nil)
        button.title = title.isEmpty ? "" : " " + title
    }

    private func structureKey() -> String {
        guard let s = model.snapshot else { return "none|\(model.lastError != nil)" }
        return s.accounts.map(\.account).joined(separator: ",") + "|\(s.desktop.pendingRecovery)|\(model.lastError != nil)"
    }

    private func card(_ a: AccountInfo, _ s: Snapshot, index: Int) -> AccountCard {
        AccountCard(
            info: a, recommended: s.recommend == a.account, now: Date(),
            shortcut: index < 9 ? index + 1 : nil, switching: model.switching,
            onTap: { [weak self] in self?.clicked(.both(a.account)) },
            onCLIOnly: { [weak self] in self?.clicked(.cliOnly(a.account)) },
            onDesktopOnly: { [weak self] in self?.clicked(.desktopOnly(a.account)) })
    }

    private func rebuild(_ key: String) {
        builtFor = key
        menu.removeAllItems()
        cards = [:]
        errorItem = nil
        if let s = model.snapshot {
            for (i, a) in s.accounts.enumerated() {
                let host = MenuCardView(card(a, s, index: i))
                let item = NSMenuItem(title: a.displayName, action: #selector(switchFromKey(_:)), keyEquivalent: i < 9 ? "\(i + 1)" : "")
                item.target = self
                item.representedObject = a.account
                item.view = host
                menu.addItem(item)
                cards[a.account] = host
            }
            if s.accounts.isEmpty {
                menu.addItem(note("No accounts yet — add one in Settings", symbol: "info.circle"))
            }
            if s.desktop.pendingRecovery {
                menu.addItem(note("Desktop switch was interrupted — run: hotseat desktop recover", symbol: "exclamationmark.triangle"))
            }
        } else if model.lastError == nil {
            menu.addItem(note("Loading…", symbol: nil))
        }
        if let error = model.lastError {
            let item = note(error.message, symbol: "exclamationmark.triangle")
            menu.addItem(item)
            errorItem = item
        }
        menu.addItem(.separator())
        menu.addItem(action("Settings…", symbol: "gearshape", #selector(openSettings), key: ","))
        menu.addItem(action("Setup…", symbol: "checklist", #selector(openSetup), key: ""))
        menu.addItem(action("Refresh", symbol: "arrow.clockwise", #selector(refresh), key: "r"))
        menu.addItem(action("Quit Hotseat", symbol: "power", #selector(NSApplication.terminate(_:)), key: "q", target: NSApp))
        updateInPlace()
    }

    private func updateInPlace() {
        if let s = model.snapshot {
            for (i, a) in s.accounts.enumerated() {
                cards[a.account]?.update(card(a, s, index: i))
            }
        }
        if let errorItem, let error = model.lastError {
            errorItem.title = error.message
        }
    }

    private func note(_ text: String, symbol: String?) -> NSMenuItem {
        let item = NSMenuItem(title: text, action: nil, keyEquivalent: "")
        item.isEnabled = false
        if let symbol {
            item.image = NSImage(systemSymbolName: symbol, accessibilityDescription: nil)
        }
        return item
    }

    private func action(_ title: String, symbol: String?, _ selector: Selector, key: String, target: AnyObject? = nil) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: selector, keyEquivalent: key)
        item.target = target ?? self
        if let symbol {
            item.image = NSImage(systemSymbolName: symbol, accessibilityDescription: nil)
        }
        return item
    }

    // MARK: - Switching

    private func clicked(_ move: Move) {
        guard model.switching == nil else { return }
        pendingMove = move
        menu.cancelTracking()
    }

    @objc private func switchFromKey(_ sender: NSMenuItem) {
        guard let name = sender.representedObject as? String else { return }
        later { self.perform(.both(name)) }
    }

    private func perform(_ move: Move) {
        switch move {
        case .both(let name): beginSwitch(name)
        case .cliOnly(let name): switchCLIOnly(name)
        case .desktopOnly(let name): switchDesktopOnly(name)
        }
    }

    private func switchCLIOnly(_ name: String) {
        guard model.switching == nil, model.snapshot?.current != name else { return }
        model.switching = name
        render()
        Task {
            do {
                try await cli().useCLIOnly(name)
            } catch {
                report(error)
            }
            finishSwitch()
        }
    }

    private func switchDesktopOnly(_ name: String) {
        guard model.switching == nil, let s = model.snapshot, s.desktop.active != name else { return }
        if s.desktop.running && model.askBeforeDesktopRestart {
            let label = s.accounts.first { $0.account == name }?.displayName ?? name
            let alert = NSAlert()
            alert.messageText = "Restart Claude Desktop as \(label)?"
            alert.informativeText = "Hotseat will quit Claude Desktop and reopen it as \(label) on the session you were in. Anything running in Desktop right now will stop. Terminal sessions are not affected."
            alert.addButton(withTitle: "Restart Desktop")
            alert.addButton(withTitle: "Cancel")
            NSApp.activate()
            guard alert.runModal() == .alertFirstButtonReturn else { return }
        }
        model.switching = name
        render()
        Task {
            do {
                try await cli().switchDesktop(name)
            } catch {
                report(error)
            }
            finishSwitch()
        }
    }

    private func beginSwitch(_ name: String) {
        guard model.switching == nil, let s = model.snapshot else { return }
        if name == s.current && (s.desktop.active == nil || s.desktop.active == name) { return }
        model.switching = name
        render()
        Task {
            do {
                // hotseat decides on live state whether Desktop must restart.
                if try await cli().use(name, restartDesktop: !model.askBeforeDesktopRestart) == .needsDesktopRestart {
                    later { self.askToRestartDesktop(name) }
                    return
                }
            } catch {
                report(error)
            }
            finishSwitch()
        }
    }

    private func askToRestartDesktop(_ name: String) {
        let label = model.snapshot?.accounts.first { $0.account == name }?.displayName ?? name
        let alert = NSAlert()
        alert.messageText = "Restart Claude Desktop as \(label)?"
        alert.informativeText = "New terminal sessions already use \(label). Claude Desktop is running as another account: Hotseat will quit it and reopen it as \(label) on the session you were in. Anything running in Desktop right now will stop."
        alert.addButton(withTitle: "Restart Desktop")
        alert.addButton(withTitle: "Not Now")
        alert.showsSuppressionButton = true
        alert.suppressionButton?.title = "Don't ask again"
        NSApp.activate()
        guard alert.runModal() == .alertFirstButtonReturn else {
            finishSwitch()
            return
        }
        if alert.suppressionButton?.state == .on {
            setAskBeforeDesktopRestart(false)
        }
        Task {
            do {
                _ = try await cli().use(name, restartDesktop: true)
            } catch {
                report(error)
            }
            finishSwitch()
        }
    }

    private func finishSwitch() {
        model.switching = nil
        render()
        refresh()
    }

    // MARK: - Settings

    @objc private func openSettings() {
        loadPreferences()
        if settingsWindow == nil {
            let actions = SettingsActions(
                saveNote: { [weak self] in self?.saveNote($0, $1) },
                addAccount: { [weak self] in later { self?.promptAddAccount() } },
                setUpDesktop: { [weak self] name in later { self?.setUpDesktop(name) } },
                remove: { [weak self] name in later { self?.confirmRemove(name) } },
                setIcon: { [weak self] in self?.setIcon($0, $1) },
                setLaunchAtLogin: { [weak self] in self?.setLaunchAtLogin($0) },
                setAskBeforeDesktopRestart: { [weak self] in self?.setAskBeforeDesktopRestart($0) })
            let window = NSWindow(contentViewController: NSHostingController(rootView: SettingsView(model: model, actions: actions)))
            window.title = "Hotseat Settings"
            window.styleMask = [.titled, .closable]
            window.isReleasedWhenClosed = false
            window.delegate = self
            window.center()
            settingsWindow = window
        }
        NSApp.activate()
        settingsWindow?.makeKeyAndOrderFront(nil)
        refresh()
    }

    // Drop the window and its views when closed, so nothing of Settings
    // keeps running in the background.
    func windowWillClose(_ notification: Notification) {
        if (notification.object as? NSWindow) === settingsWindow {
            settingsWindow?.contentViewController = nil
            settingsWindow = nil
        }
        if (notification.object as? NSWindow) === setupWindow {
            setupWindow?.contentViewController = nil
            setupWindow = nil
        }
    }

    private func setIcon(_ name: String, _ icon: String?) {
        Task {
            do {
                try await cli().setIcon(name, icon)
            } catch {
                report(error)
            }
            refresh()
        }
    }

    private func saveNote(_ name: String, _ text: String) {
        Task {
            do {
                try await cli().setNote(name, text)
                model.noteDrafts[name] = nil
            } catch {
                report(error)
            }
            refresh()
        }
    }

    private func promptAddAccount() {
        let alert = NSAlert()
        alert.messageText = "Add a Claude account"
        alert.informativeText = "Pick a short name for it (letters, digits, . _ -). Terminal opens with the official Claude login; sign in there with the new account."
        alert.addButton(withTitle: "Continue")
        alert.addButton(withTitle: "Cancel")
        let field = NSTextField(frame: NSRect(x: 0, y: 0, width: 280, height: 24))
        field.placeholderString = "e.g. work"
        alert.accessoryView = field
        alert.window.initialFirstResponder = field
        NSApp.activate()
        guard alert.runModal() == .alertFirstButtonReturn else { return }
        let name = field.stringValue.trimmingCharacters(in: .whitespaces)
        guard AddAccount.isValidName(name) else {
            show("\"\(name)\" is not a valid account name: use letters, digits, . _ - and start with a letter or digit.")
            return
        }
        do {
            let script = FileManager.default.temporaryDirectory.appendingPathComponent("hotseat-add-\(name).command")
            try AddAccount.script(hotseat: try cli().executable, name: name).write(to: script, atomically: true, encoding: .utf8)
            try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
            NSWorkspace.shared.open(script)
        } catch {
            show(hotseatError(error).message)
        }
    }

    private func setUpDesktop(_ name: String) {
        let label = model.snapshot?.accounts.first { $0.account == name }?.displayName ?? name
        let alert = NSAlert()
        alert.messageText = "Set up Claude Desktop for \(label)?"
        alert.informativeText = "Claude Desktop quits if it is running and reopens with an empty profile for this account. Sign in there with \(label). Your current Desktop data is kept for its own account."
        alert.addButton(withTitle: "Set Up")
        alert.addButton(withTitle: "Cancel")
        NSApp.activate()
        guard alert.runModal() == .alertFirstButtonReturn else { return }
        Task {
            do {
                try await cli().setUpDesktop(name)
            } catch {
                report(error)
            }
            refresh()
        }
    }

    private func confirmRemove(_ name: String) {
        let label = model.snapshot?.accounts.first { $0.account == name }?.displayName ?? name
        let alert = NSAlert()
        alert.messageText = "Remove \(label)?"
        alert.informativeText = "Hotseat signs this account out (claude auth logout) and moves its config dir and Claude Desktop profile into a Hotseat backup; nothing is deleted. Close any terminal using it first. Your shared work (projects, history) stays."
        alert.addButton(withTitle: "Remove")
        alert.addButton(withTitle: "Cancel")
        alert.buttons.first?.hasDestructiveAction = true
        NSApp.activate()
        guard alert.runModal() == .alertFirstButtonReturn else { return }
        Task {
            do {
                try await cli().remove(name)
            } catch {
                report(error)
            }
            refresh()
        }
    }

    private func setLaunchAtLogin(_ on: Bool) {
        do {
            if on {
                try SMAppService.mainApp.register()
            } else {
                try SMAppService.mainApp.unregister()
            }
        } catch {
            show("Launch at Login: \(error.localizedDescription)")
        }
        model.launchAtLogin = SMAppService.mainApp.status == .enabled
    }

    private func setAskBeforeDesktopRestart(_ ask: Bool) {
        UserDefaults.standard.set(!ask, forKey: "skipDesktopConfirm")
        model.askBeforeDesktopRestart = ask
    }

    // MARK: - Errors

    private func hotseatError(_ error: Error) -> HotseatError {
        error as? HotseatError ?? .failed(code: -1, stderr: "\(error)")
    }

    /// Reports an error from async work; the alert is shown outside the task.
    private func report(_ error: Error) {
        let text = hotseatError(error).message
        later { self.show(text) }
    }

    private func show(_ text: String) {
        let alert = NSAlert()
        alert.messageText = "Hotseat"
        alert.informativeText = text
        NSApp.activate()
        alert.runModal()
    }
}
