import Foundation

/// The output of `hotseat setup status --json`.
public struct SetupStatus: Decodable, Sendable {
    public var version: String
    public var accounts: Int
    public var loginsFound: Int
    public var shareActions: Int
    public var shim: Bool
    public var statusline: Bool
    public var hook: Bool
    /// adopted, available or absent
    public var desktop: String

    /// The setup checklist, in order. `usingBundled` means only the CLI
    /// inside the app was found, so the command line tool is not installed.
    public func steps(bundledVersion: String, usingBundled: Bool) -> [SetupStep] {
        [
            SetupStep(id: "cli", title: "Command line tool",
                      done: !usingBundled && version == bundledVersion,
                      detail: usingBundled || version == bundledVersion
                          ? "Installs `hotseat` to ~/.local/bin"
                          : "Updates `hotseat` in ~/.local/bin from \(version) to \(bundledVersion)",
                      commands: nil),
            SetupStep(id: "accounts", title: "Find your Claude logins", done: accounts > 0,
                      detail: loginsFound == 0
                          ? "No Claude Code login found yet: run `claude` once in Terminal and log in, then come back."
                          : "Registers the ~/.claude* dirs you are logged in to.",
                      commands: [["init"]]),
            SetupStep(id: "share", title: "Share your work data", done: accounts > 0 && shareActions == 0,
                      detail: "Links sessions, settings and skills of every account to ~/.claude. Close running claude sessions first.",
                      commands: [["share", "apply", "--json"]]),
            SetupStep(id: "shim", title: "Follow the current account", done: shim,
                      detail: "New terminals start `claude` as the account you pick",
                      commands: [["shim", "install"]]),
            SetupStep(id: "tracking", title: "Usage and limit handoff", done: statusline && hook,
                      detail: "Records usage from Claude Code's status line; offers another account when one runs out",
                      commands: [["statusline", "install"], ["hook", "install"]]),
            SetupStep(id: "desktop", title: "Claude Desktop", done: desktop != "available",
                      detail: desktop == "absent" ? "Claude Desktop is not installed or never opened" : "Lets Hotseat switch Desktop's account",
                      commands: [["desktop", "adopt"]]),
        ]
    }
}

public struct SetupStep: Identifiable, Sendable {
    public var id: String
    public var title: String
    public var done: Bool
    public var detail: String
    /// hotseat invocations that perform the step, run in order; nil when
    /// the app does it itself (installing the CLI).
    public var commands: [[String]]?
}
