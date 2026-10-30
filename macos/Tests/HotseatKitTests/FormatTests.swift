import Foundation
import Testing
@testable import HotseatKit

private let shanghai: Calendar = {
    var c = Calendar(identifier: .gregorian)
    c.timeZone = TimeZone(identifier: "Asia/Shanghai")!
    return c
}()

/// "yyyy-MM-dd HH:mm" in Asia/Shanghai, as a Unix time.
private func ts(_ s: String) -> Int64 {
    let f = DateFormatter()
    f.locale = Locale(identifier: "en_US_POSIX")
    f.timeZone = shanghai.timeZone
    f.dateFormat = "yyyy-MM-dd HH:mm"
    return Int64(f.date(from: s)!.timeIntervalSince1970)
}

private let now = Date(timeIntervalSince1970: TimeInterval(ts("2026-10-30 10:00")))

private func window(_ pct: Double, resets: String?, reset: Bool = false) -> UsageWindow {
    UsageWindow(usedPercent: pct, resetsAt: resets.map(ts) ?? 0, observedAt: ts("2026-10-30 09:50"), source: "statusline", reset: reset)
}

@Test func windowText() {
    #expect(Format.window("5h", nil, now: now, calendar: shanghai) == "5h ?")
    #expect(Format.window("5h", window(0, resets: "2026-10-30 08:00", reset: true), now: now, calendar: shanghai) == "5h 0% (reset)")
    #expect(Format.window("5h", window(20.4, resets: "2026-10-30 14:30"), now: now, calendar: shanghai) == "5h 20% · resets 14:30")
    #expect(Format.window("7d", window(79, resets: nil), now: now, calendar: shanghai) == "7d 79%")
}

@Test func whenText() {
    #expect(Format.when(ts("2026-10-30 14:30"), now: now, calendar: shanghai) == "14:30")
    #expect(Format.when(ts("2026-11-02 19:00"), now: now, calendar: shanghai) == "Mon 19:00")
    #expect(Format.when(ts("2026-11-09 10:00"), now: now, calendar: shanghai) == "Nov 9 10:00")
}

@Test func ageText() {
    let t = Int64(now.timeIntervalSince1970)
    #expect(Format.age(nil, now: now) == "no data yet")
    #expect(Format.age(0, now: now) == "no data yet")
    #expect(Format.age(t - 59, now: now) == "updated just now")
    #expect(Format.age(t - 59 * 60, now: now) == "updated 59 min ago")
    #expect(Format.age(t - 47 * 3600, now: now) == "updated 47 h ago")
    #expect(Format.age(t - 3 * 86400, now: now) == "updated 3 d ago")
}

@Test func accountAgeUsesNewestWindow() throws {
    let s = try Snapshot.decode(Data(panelFixture.utf8))
    #expect(s.accounts[2].observedAt == 1793328072)
    #expect(s.accounts[0].observedAt == nil)
}

@Test func titleText() throws {
    let s = try Snapshot.decode(Data(panelFixture.utf8))
    #expect(Format.title(nil, failed: true) == "Hotseat !")
    #expect(Format.title(nil, failed: false) == "Hotseat")
    #expect(Format.title(s, failed: false) == "claude3 20%")
    #expect(Format.title(s, failed: true) == "Hotseat !")
    let limited = try Snapshot.decode(Data(panelFixture.replacingOccurrences(of: "\"current\": \"claude3\"", with: "\"current\": \"claude\"").utf8))
    #expect(Format.title(limited, failed: false) == "claude LIMIT")
}
