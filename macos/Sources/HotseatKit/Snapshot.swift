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

    public var id: String { account }

    /// When the newer of the two usage windows was observed.
    public var observedAt: Int64? {
        [fiveHour?.observedAt, sevenDay?.observedAt].compactMap { $0 }.filter { $0 > 0 }.max()
    }
}

public struct UsageWindow: Decodable, Sendable {
    public var usedPercent: Double
    public var resetsAt: Int64
    public var observedAt: Int64
    public var source: String
    public var reset: Bool

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
}
