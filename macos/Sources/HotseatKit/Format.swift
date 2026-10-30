import Foundation

/// The text the menu shows. Times follow the CLI's `hotseat usage`.
public enum Format {
    public static func window(_ label: String, _ w: UsageWindow?, now: Date, calendar: Calendar = .current) -> String {
        guard let w else { return "\(label) ?" }
        if w.reset { return "\(label) 0% (reset)" }
        var s = "\(label) \(String(format: "%.0f", w.usedPercent))%"
        if w.resetsAt > 0 {
            s += " · resets " + when(w.resetsAt, now: now, calendar: calendar)
        }
        return s
    }

    public static func when(_ unix: Int64, now: Date, calendar: Calendar = .current) -> String {
        let t = Date(timeIntervalSince1970: TimeInterval(unix))
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.calendar = calendar
        f.timeZone = calendar.timeZone
        if calendar.isDate(t, inSameDayAs: now) {
            f.dateFormat = "HH:mm"
        } else if t.timeIntervalSince(now) < 6 * 86400 {
            f.dateFormat = "EEE HH:mm"
        } else {
            f.dateFormat = "MMM d HH:mm"
        }
        return f.string(from: t)
    }

    public static func age(_ observedAt: Int64?, now: Date) -> String {
        guard let observedAt, observedAt > 0 else { return "no data yet" }
        let d = max(0, Int64(now.timeIntervalSince1970) - observedAt)
        switch d {
        case ..<60: return "updated just now"
        case ..<3600: return "updated \(d / 60) min ago"
        case ..<(48 * 3600): return "updated \(d / 3600) h ago"
        default: return "updated \(d / 86400) d ago"
        }
    }

    /// The status bar title.
    public static func title(_ s: Snapshot?, failed: Bool) -> String {
        if failed { return "Hotseat !" }
        guard let s, !s.current.isEmpty else { return "Hotseat" }
        guard let a = s.accounts.first(where: { $0.account == s.current }) else { return s.current }
        if a.limitActive { return "\(a.account) LIMIT" }
        if let w = a.fiveHour {
            return "\(a.account) \(String(format: "%.0f", w.reset ? 0 : w.usedPercent))%"
        }
        return a.account
    }
}
