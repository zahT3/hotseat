import HotseatKit
import Observation

/// State shared by the menu and the Settings window.
@MainActor
@Observable
final class AppModel {
    var snapshot: Snapshot?
    var lastError: HotseatError?
    /// The account being switched to; other actions wait meanwhile.
    var switching: String?
    /// Notes being typed in Settings, by account, until saved.
    var noteDrafts: [String: String] = [:]
    var launchAtLogin = false
    var askBeforeDesktopRestart = true
    var cliPath = ""
}

// The Command Line Tools ship no SwiftUIMacros plugin, so @State cannot be
// used; small view state lives in @Observable objects instead.
@MainActor
@Observable
final class Hover {
    var on = false
}
