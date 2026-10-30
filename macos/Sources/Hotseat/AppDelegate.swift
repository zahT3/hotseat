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
    /// The account being switched to; rows ignore clicks meanwhile.
    private var switching: String?
    /// An account clicked in the open menu, switched to once the menu closes.
    private var pendingSwitch: String?
    private var timer: Timer?

    func applicationDidFinishLaunching(_ notification: Notification) {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        menu.delegate = self
        statusItem.menu = menu
        render()
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated { self?.refresh() }
        }
    }

    func menuWillOpen(_ menu: NSMenu) {
        refresh()
    }

    func menuDidClose(_ menu: NSMenu) {
        guard let name = pendingSwitch else { return }
        pendingSwitch = nil
        // Out of the menu's tracking loop before any alert or rebuild.
        Task { self.beginSwitch(name) }
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

    private func render() {
        if let switching {
            statusItem.button?.title = "→ \(switching)…"
        } else {
            statusItem.button?.title = Format.title(snapshot, failed: lastError != nil)
        }
        menu.removeAllItems()
        let now = Date()
        if let s = snapshot {
            for a in s.accounts {
                let row = AccountRow(
                    info: a, recommended: s.recommend == a.account, now: now,
                    disabled: switching != nil,
                    onTap: { [weak self] in self?.tapped(a.account) })
                let host = NSHostingView(rootView: row)
                host.frame = NSRect(origin: .zero, size: host.fittingSize)
                let item = NSMenuItem()
                item.view = host
                menu.addItem(item)
            }
            if s.accounts.isEmpty {
                menu.addItem(note("No accounts — run: hotseat init"))
            }
            if s.desktop.pendingRecovery {
                menu.addItem(note("Desktop switch was interrupted — run: hotseat desktop recover"))
            }
        } else if lastError == nil {
            menu.addItem(note("Loading…"))
        }
        if let lastError {
            menu.addItem(note(lastError.message))
        }
        menu.addItem(.separator())
        menu.addItem(action("Refresh", #selector(refresh), key: "r"))
        let login = action("Launch at Login", #selector(toggleLaunchAtLogin), key: "")
        login.state = SMAppService.mainApp.status == .enabled ? .on : .off
        menu.addItem(login)
        menu.addItem(action("Quit Hotseat", #selector(NSApplication.terminate(_:)), key: "q", target: NSApp))
    }

    private func note(_ text: String) -> NSMenuItem {
        let item = NSMenuItem(title: text, action: nil, keyEquivalent: "")
        item.isEnabled = false
        return item
    }

    private func action(_ title: String, _ selector: Selector, key: String, target: AnyObject? = nil) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: selector, keyEquivalent: key)
        item.target = target ?? self
        return item
    }

    private func tapped(_ name: String) {
        guard switching == nil else { return }
        pendingSwitch = name
        menu.cancelTracking()
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
