import AppKit
import HotseatKit
import ServiceManagement
import SwiftUI

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate, NSMenuDelegate {
    private var statusItem: NSStatusItem!
    private let menu = NSMenu()
    private var snapshot: Snapshot?
    private var lastError: HotseatError?
    private var gate = RefreshGate()
    /// The account being switched to; actions are disabled meanwhile.
    private var switching: String?
    private var timer: Timer?

    /// Something asked for in the open menu, done once the menu has closed
    /// (alerts and long work must not run inside menu tracking).
    private enum Action {
        case switchTo(String)
        case editNote(String)
    }
    private var pending: Action?

    private var menuOpen = false
    /// What the current menu items were built for; while the menu is open
    /// they are only updated in place, which keeps its size right.
    private var builtFor = ""
    private var cards: [String: NSHostingView<AccountCard>] = [:]
    private var errorItem: NSMenuItem?
    private var loginItem: NSMenuItem?

    func applicationDidFinishLaunching(_ notification: Notification) {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        statusItem.button?.imagePosition = .imageLeading
        menu.delegate = self
        statusItem.menu = menu
        render()
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated { self?.refresh() }
        }
    }

    func menuWillOpen(_ menu: NSMenu) {
        menuOpen = true
        refresh()
    }

    func menuDidClose(_ menu: NSMenu) {
        menuOpen = false
        render()
        guard let action = pending else { return }
        pending = nil
        Task { self.perform(action) }
    }

    private func cli() throws -> HotseatCLI {
        try HotseatCLI.locate(
            override: UserDefaults.standard.string(forKey: "hotseatPath"),
            home: FileManager.default.homeDirectoryForCurrentUser)
    }

    @objc private func refresh() {
        guard gate.request() else { return }
        startRefresh()
    }

    private func startRefresh() {
        Task {
            do {
                snapshot = try await cli().snapshot()
                lastError = nil
            } catch {
                lastError = error as? HotseatError ?? .failed(code: -1, stderr: "\(error)")
            }
            render()
            if gate.finish() {
                startRefresh()
            }
        }
    }

    // MARK: - Drawing

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
        if let switching {
            let name = snapshot?.accounts.first { $0.account == switching }?.shortName ?? switching
            button.image = StatusIcon.switching
            button.title = " " + name
            return
        }
        let current = snapshot?.accounts.first { $0.account == snapshot?.current }
        let five = current?.fiveHour.map { $0.reset ? 0 : $0.usedPercent / 100 }
        let week = current?.sevenDay.map { $0.reset ? 0 : $0.usedPercent / 100 }
        button.image = StatusIcon.meter(five: five, week: week)
        button.title = " " + Format.title(snapshot, failed: lastError != nil)
    }

    private func structureKey() -> String {
        guard let s = snapshot else { return "none|\(lastError != nil)" }
        return s.accounts.map(\.account).joined(separator: ",") + "|\(s.desktop.pendingRecovery)|\(lastError != nil)"
    }

    private func card(_ a: AccountInfo, _ s: Snapshot) -> AccountCard {
        AccountCard(
            info: a, recommended: s.recommend == a.account, now: Date(), switching: switching,
            onSwitch: { [weak self] in self?.request(.switchTo(a.account)) },
            onEditNote: { [weak self] in self?.request(.editNote(a.account)) })
    }

    private func rebuild(_ key: String) {
        builtFor = key
        menu.removeAllItems()
        cards = [:]
        errorItem = nil
        if let s = snapshot {
            for a in s.accounts {
                let host = NSHostingView(rootView: card(a, s))
                host.frame = NSRect(origin: .zero, size: host.fittingSize)
                let item = NSMenuItem()
                item.view = host
                menu.addItem(item)
                cards[a.account] = host
            }
            if s.accounts.isEmpty {
                menu.addItem(note("No accounts yet — use Add Account…", symbol: "info.circle"))
            }
            if s.desktop.pendingRecovery {
                menu.addItem(note("Desktop switch was interrupted — run: hotseat desktop recover", symbol: "exclamationmark.triangle"))
            }
        } else if lastError == nil {
            menu.addItem(note("Loading…", symbol: nil))
        }
        if let lastError {
            let item = note(lastError.message, symbol: "exclamationmark.triangle")
            menu.addItem(item)
            errorItem = item
        }
        menu.addItem(.separator())
        menu.addItem(action("Add Account…", symbol: "person.crop.circle.badge.plus", #selector(addAccount), key: ""))
        menu.addItem(action("Refresh", symbol: "arrow.clockwise", #selector(refresh), key: "r"))
        let login = action("Launch at Login", symbol: "power.circle", #selector(toggleLaunchAtLogin), key: "")
        menu.addItem(login)
        loginItem = login
        menu.addItem(.separator())
        menu.addItem(action("Quit Hotseat", symbol: nil, #selector(NSApplication.terminate(_:)), key: "q", target: NSApp))
        updateInPlace()
    }

    private func updateInPlace() {
        if let s = snapshot {
            for a in s.accounts {
                guard let host = cards[a.account] else { continue }
                host.rootView = card(a, s)
                host.frame.size = host.fittingSize
            }
        }
        if let errorItem, let lastError {
            errorItem.title = lastError.message
        }
        loginItem?.state = SMAppService.mainApp.status == .enabled ? .on : .off
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

    // MARK: - Actions

    private func request(_ action: Action) {
        guard switching == nil else { return }
        pending = action
        menu.cancelTracking()
    }

    @objc private func addAccount() {
        // Menu item actions arrive after menu tracking has ended.
        Task { self.promptAddAccount() }
    }

    private func perform(_ action: Action) {
        switch action {
        case .switchTo(let name): beginSwitch(name)
        case .editNote(let name): editNote(name)
        }
    }

    private func beginSwitch(_ name: String) {
        guard switching == nil, let s = snapshot else { return }
        if name == s.current && (s.desktop.active == nil || s.desktop.active == name) { return }
        switching = name
        render()
        Task {
            do {
                let cli = try cli()
                // hotseat decides on live state whether Desktop must restart.
                let skipAsk = UserDefaults.standard.bool(forKey: "skipDesktopConfirm")
                if try await cli.use(name, restartDesktop: skipAsk) == .needsDesktopRestart, confirmDesktopRestart(name) {
                    _ = try await cli.use(name, restartDesktop: true)
                }
            } catch {
                switching = nil
                render()
                show(error: error as? HotseatError ?? .failed(code: -1, stderr: "\(error)"))
            }
            switching = nil
            render()
            refresh()
        }
    }

    private func confirmDesktopRestart(_ name: String) -> Bool {
        let alert = NSAlert()
        alert.messageText = "Restart Claude Desktop as \(name)?"
        alert.informativeText = "New terminal sessions already use \(name). Claude Desktop is running as another account: Hotseat will quit it and reopen it as \(name) on the session you were in. Anything running in Desktop right now will stop."
        alert.addButton(withTitle: "Restart Desktop")
        alert.addButton(withTitle: "Not Now")
        alert.showsSuppressionButton = true
        alert.suppressionButton?.title = "Don't ask again"
        NSApp.activate()
        guard alert.runModal() == .alertFirstButtonReturn else { return false }
        if alert.suppressionButton?.state == .on {
            UserDefaults.standard.set(true, forKey: "skipDesktopConfirm")
        }
        return true
    }

    private func editNote(_ name: String) {
        let currentNote = snapshot?.accounts.first { $0.account == name }?.note ?? ""
        guard let text = prompt(
            title: "Note for \(name)",
            message: "Shown instead of the email. Leave empty to show the email again.",
            initial: currentNote, placeholder: "e.g. Work", ok: "Save")
        else { return }
        Task {
            do {
                try await cli().setNote(name, text)
            } catch {
                show(error: error as? HotseatError ?? .failed(code: -1, stderr: "\(error)"))
            }
            refresh()
        }
    }

    private func promptAddAccount() {
        guard let name = prompt(
            title: "Add a Claude account",
            message: "Pick a short name for it (letters, digits, . _ -). Terminal opens with the official Claude login; sign in there with the new account.",
            initial: "", placeholder: "e.g. work", ok: "Continue")?
            .trimmingCharacters(in: .whitespaces), !name.isEmpty
        else { return }
        guard AddAccount.isValidName(name) else {
            show(error: .failed(code: -1, stderr: "\"\(name)\" is not a valid account name: use letters, digits, . _ - and start with a letter or digit."))
            return
        }
        do {
            let script = FileManager.default.temporaryDirectory.appendingPathComponent("hotseat-add-\(name).command")
            try AddAccount.script(hotseat: try cli().executable, name: name).write(to: script, atomically: true, encoding: .utf8)
            try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
            NSWorkspace.shared.open(script)
        } catch {
            show(error: error as? HotseatError ?? .failed(code: -1, stderr: "\(error)"))
        }
    }

    private func prompt(title: String, message: String, initial: String, placeholder: String, ok: String) -> String? {
        let alert = NSAlert()
        alert.messageText = title
        alert.informativeText = message
        alert.addButton(withTitle: ok)
        alert.addButton(withTitle: "Cancel")
        let field = NSTextField(frame: NSRect(x: 0, y: 0, width: 280, height: 24))
        field.stringValue = initial
        field.placeholderString = placeholder
        alert.accessoryView = field
        alert.window.initialFirstResponder = field
        NSApp.activate()
        guard alert.runModal() == .alertFirstButtonReturn else { return nil }
        return field.stringValue
    }

    private func show(error: HotseatError) {
        let alert = NSAlert()
        alert.messageText = "Hotseat"
        alert.informativeText = error.message
        NSApp.activate()
        alert.runModal()
    }

    @objc private func toggleLaunchAtLogin() {
        do {
            if SMAppService.mainApp.status == .enabled {
                try SMAppService.mainApp.unregister()
            } else {
                try SMAppService.mainApp.register()
            }
        } catch {
            show(error: .failed(code: -1, stderr: "Launch at Login: \(error.localizedDescription)"))
        }
        render()
    }
}
