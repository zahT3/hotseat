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
    private var refreshing = false
    /// The account being switched to; rows ignore clicks meanwhile.
    private var switching: String?
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

    private func cli() throws -> HotseatCLI {
        try HotseatCLI.locate(
            override: UserDefaults.standard.string(forKey: "hotseatPath"),
            home: FileManager.default.homeDirectoryForCurrentUser)
    }

    @objc private func refresh() {
        guard !refreshing else { return }
        refreshing = true
        Task {
            do {
                snapshot = try await cli().snapshot()
                lastError = nil
            } catch {
                lastError = error as? HotseatError ?? .failed(code: -1, stderr: "\(error)")
            }
            refreshing = false
            render()
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
        menu.cancelTracking()
        guard switching == nil, let s = snapshot, let target = s.accounts.first(where: { $0.account == name }) else { return }
        if name == s.current && (s.desktop.active == nil || s.desktop.active == name) { return }
        // Desktop is restarted only when it runs and the target has a parked profile.
        if s.desktop.running && target.desktop == "parked" && !UserDefaults.standard.bool(forKey: "skipDesktopConfirm") {
            guard confirmDesktopRestart(name) else { return }
        }
        switching = name
        render()
        Task {
            do {
                try await cli().use(name)
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
        alert.messageText = "Switch to \(name)?"
        alert.informativeText = "Claude Desktop will quit and reopen as \(name) on the session you were in. Anything running in Desktop right now will stop."
        alert.addButton(withTitle: "Switch")
        alert.addButton(withTitle: "Cancel")
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
