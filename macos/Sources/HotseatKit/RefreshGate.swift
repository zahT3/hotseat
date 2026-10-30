/// Lets one refresh run at a time without losing a request: one asked for
/// while another is in flight (say, right after a switch) runs when it ends,
/// so a result fetched mid-switch is never the last word.
public struct RefreshGate: Sendable {
    private var running = false
    private var pending = false

    public init() {}

    /// Whether the caller should start a refresh now.
    public mutating func request() -> Bool {
        if running {
            pending = true
            return false
        }
        running = true
        return true
    }

    /// Call when a refresh ends; whether to start another one right away.
    public mutating func finish() -> Bool {
        if pending {
            pending = false
            return true
        }
        running = false
        return false
    }
}
