import Foundation

/// The text the menu shows. Times follow the CLI's `hotseat usage`.
public enum Format {
    /// A usage window's number and reset time, shown beside its bar.
    public static func value(_ w: UsageWindow?, now: Date, calendar: Calendar = .current) -> String {
        guard let w else { return "?" }
        if w.reset { return "0% (reset)" }
        var s = "\(String(format: "%.0f", w.usedPercent))%"
        if w.resetsAt > 0 {
            s += " · " + resets(w.resetsAt, now: now, calendar: calendar)
        }
        return s
    }

    /// A countdown within a day ("in 3h 53m"), a weekday and time after.
    public static func resets(_ unix: Int64, now: Date, calendar: Calendar = .current) -> String {
        let left = unix - Int64(now.timeIntervalSince1970)
        guard left < 86400 else { return when(unix, now: now, calendar: calendar) }
        let minutes = max(0, (left + 59) / 60)
        switch (minutes / 60, minutes % 60) {
        case (0, let m): return "in \(m)m"
        case (let h, 0): return "in \(h)h"
        case (let h, let m): return "in \(h)h \(m)m"
        }
    }

    public static func limit(_ l: LimitHit?, now: Date, calendar: Calendar = .current) -> String {
        guard let l, l.resetsAt > 0 else { return "Limit reached" }
        if l.resetsAt - Int64(now.timeIntervalSince1970) < 86400 {
            return "Limit · resets " + resets(l.resetsAt, now: now, calendar: calendar)
        }
        return "Limit until " + when(l.resetsAt, now: now, calendar: calendar)
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

    /// The text beside the current account's avatar in the menu bar: its
    /// 5-hour use, LIMIT, or "!" on errors. Never the email: the menu bar
    /// ends up in every screenshot.
    public static func title(_ s: Snapshot?, failed: Bool) -> String {
        if failed { return "!" }
        guard let s, let a = s.accounts.first(where: { $0.account == s.current }) else { return "" }
        if a.limitActive { return "LIMIT" }
        guard let w = a.fiveHour else { return "" }
        return "\(String(format: "%.0f", w.reset ? 0 : w.usedPercent))%"
    }
}
