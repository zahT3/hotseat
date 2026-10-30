import Foundation

/// Errors from talking to the hotseat CLI, worded for the menu.
public enum HotseatError: Error, Equatable, Sendable {
    case notFound(tried: [String])
    case failed(code: Int32, stderr: String)
    case timedOut(seconds: Int)
    case badOutput(String)
}

/// The output of `hotseat panel --json`.
public struct Snapshot: Decodable, Sendable {
    public var generatedAt: Int64
    public var current: String
    public var recommend: String?
    public var desktop: DesktopInfo
    public var accounts: [AccountInfo]

    public static func decode(_ data: Data) throws -> Snapshot {
        do {
            return try JSONDecoder().decode(Snapshot.self, from: data)
        } catch {
            throw HotseatError.badOutput(String(String(decoding: data, as: UTF8.self).prefix(200)))
        }
    }
}

public struct DesktopInfo: Decodable, Sendable {
    public var managed: Bool
    public var running: Bool
    public var runningKnown: Bool
    public var pendingRecovery: Bool
    public var active: String?
}

public struct AccountInfo: Decodable, Identifiable, Sendable {
    public var account: String
    public var current: Bool
    public var fiveHour: UsageWindow?
    public var sevenDay: UsageWindow?
    public var limit: LimitHit?
    public var limitActive: Bool
    public var error: String?
    public var desktop: String
    public var desktopProblem: String?
    public var email: String?
    public var note: String?
    public var configDir: String?
    /// Set when the config dir is logged out or logged in to another account.
    public var loginProblem: String?
    /// An SF Symbol name or emoji the user picked.
    public var icon: String?

    public var id: String { account }

    /// The user's note, else the name part of the email, else Hotseat's
    /// own name for the account (claude2...).
    public var displayName: String {
        nonEmpty(note) ?? nonEmpty(email).map { String($0.prefix { $0 != "@" }) } ?? account
    }

    /// The full email, under the display name.
    public var detail: String? {
        nonEmpty(email)
    }

    /// Whether the limit still holds now (the snapshot may be old).
    public func isLimited(now: Date) -> Bool {
        guard limitActive else { return false }
        guard let resets = limit?.resetsAt, resets > 0 else { return true }
        return Double(resets) > now.timeIntervalSince1970
    }

    /// When the newer of the two usage windows was observed.
    public var observedAt: Int64? {
        [fiveHour?.observedAt, sevenDay?.observedAt].compactMap { $0 }.filter { $0 > 0 }.max()
    }
}

private func nonEmpty(_ s: String?) -> String? {
    guard let s, !s.trimmingCharacters(in: .whitespaces).isEmpty else { return nil }
    return s
}

public struct UsageWindow: Decodable, Sendable {
    public var usedPercent: Double
    public var resetsAt: Int64
    public var observedAt: Int64
    public var source: String
    public var reset: Bool

    /// Whether the window has reset by `now`, as reported or because its
    /// reset time has passed since the snapshot was taken.
    public func isReset(now: Date) -> Bool {
        reset || (resetsAt > 0 && Double(resetsAt) <= now.timeIntervalSince1970)
    }

    /// Used percent as of `now`.
    public func percent(now: Date) -> Double {
        isReset(now: now) ? 0 : usedPercent
    }

    public init(usedPercent: Double, resetsAt: Int64, observedAt: Int64, source: String, reset: Bool) {
        self.usedPercent = usedPercent
        self.resetsAt = resetsAt
        self.observedAt = observedAt
        self.source = source
        self.reset = reset
    }
}

public struct LimitHit: Decodable, Sendable {
    public var message: String
    public var resetsAt: Int64
    public var observedAt: Int64

    public init(message: String, resetsAt: Int64, observedAt: Int64) {
        self.message = message
        self.resetsAt = resetsAt
        self.observedAt = observedAt
    }
}
