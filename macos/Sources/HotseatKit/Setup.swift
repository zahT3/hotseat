import Foundation

/// The output of `hotseat setup status --json`. Fields a CLI may not know
/// yet decode to their defaults.
public struct SetupStatus: Decodable, Sendable {
    public var version: String
    public var accounts: Int
    public var loginsFound: Int
    public var shareActions: Int
    public var shareConflicts: Int
    public var shim: Bool
    public var statusline: Bool
    public var hook: Bool
    /// adopted, available or absent
    public var desktop: String

    enum CodingKeys: String, CodingKey {
        case version, accounts, loginsFound, shareActions, shareConflicts, shim, statusline, hook, desktop
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        version = try c.decodeIfPresent(String.self, forKey: .version) ?? ""
        accounts = try c.decodeIfPresent(Int.self, forKey: .accounts) ?? 0
        loginsFound = try c.decodeIfPresent(Int.self, forKey: .loginsFound) ?? 0
        shareActions = try c.decodeIfPresent(Int.self, forKey: .shareActions) ?? 0
        shareConflicts = try c.decodeIfPresent(Int.self, forKey: .shareConflicts) ?? 0
        shim = try c.decodeIfPresent(Bool.self, forKey: .shim) ?? false
        statusline = try c.decodeIfPresent(Bool.self, forKey: .statusline) ?? false
        hook = try c.decodeIfPresent(Bool.self, forKey: .hook) ?? false
        desktop = try c.decodeIfPresent(String.self, forKey: .desktop) ?? "absent"
    }

    /// The steps the CLI performs, in order (the app adds the CLI step).
    public func steps() -> [SetupStep] {
        var shareDetail = "Links sessions, settings and skills of every account to ~/.claude. Close running claude sessions first."
        if shareConflicts > 0 {
            shareDetail = "\(shareConflicts) files differ between accounts; ~/.claude's copy is used and the others stay where they are (see: hotseat share plan)."
        }
        return [
            SetupStep(id: "accounts", title: "Find your Claude logins", done: accounts > 0,
                      detail: loginsFound == 0
                          ? "No Claude Code login found yet: run claude once in Terminal and log in, then click Check Again."
                          : "Registers the ~/.claude* dirs you are logged in to.",
                      commands: [["init"]]),
            SetupStep(id: "share", title: "Share your work data", done: accounts > 0 && shareActions == 0,
                      detail: shareDetail, commands: [["share", "apply", "--json"]]),
            SetupStep(id: "shim", title: "Follow the current account", done: shim,
                      detail: "New terminals start claude as the account you pick",
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

    /// The command line tool step, decided by the app: every other step
    /// runs that CLI, so it must exist and not be older than the app's.
    public static func cli(installedVersion: String?, installedPath: String?, usingBundled: Bool,
                           statusWorks: Bool, bundledVersion: String) -> SetupStep {
        let title = "Command line tool"
        if usingBundled || installedVersion == nil {
            return SetupStep(id: "cli", title: title, done: false,
                             detail: "Installs hotseat \(bundledVersion) to ~/.local/bin", commands: nil)
        }
        let where_ = installedPath ?? "the installed hotseat"
        if !statusWorks {
            return SetupStep(id: "cli", title: title, done: false,
                             detail: "\(where_) is older than this app; installs hotseat \(bundledVersion) to ~/.local/bin, which comes first", commands: nil)
        }
        if let v = installedVersion, Version.isOlder(v, than: bundledVersion) {
            return SetupStep(id: "cli", title: title, done: false,
                             detail: "Updates \(where_) (\(v)) with \(bundledVersion), installed to ~/.local/bin", commands: nil)
        }
        return SetupStep(id: "cli", title: title, done: true, detail: "hotseat \(installedVersion ?? "") is installed", commands: nil)
    }
}

/// Dotted version numbers compared field by field ("0.10.0" > "0.9.1").
public enum Version {
    public static func isOlder(_ a: String, than b: String) -> Bool {
        let x = parts(a), y = parts(b)
        for i in 0..<max(x.count, y.count) {
            let l = i < x.count ? x[i] : 0, r = i < y.count ? y[i] : 0
            if l != r { return l < r }
        }
        return false
    }

    private static func parts(_ v: String) -> [Int] {
        v.split(whereSeparator: { $0 == "." || $0 == "-" }).map { Int($0) ?? 0 }
    }
}
