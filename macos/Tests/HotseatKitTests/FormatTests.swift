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

@Test func titleWithoutCurrentAccount() throws {
    let empty = try Snapshot.decode(Data("""
    {"generatedAt":1,"current":"","desktop":{"managed":false,"running":false,"runningKnown":true,"pendingRecovery":false},"accounts":[]}
    """.utf8))
    #expect(Format.title(empty, failed: false) == "Hotseat")
}

@Test func windowValueText() {
    #expect(Format.value(nil, now: now, calendar: shanghai) == "?")
    #expect(Format.value(window(0, resets: "2026-10-30 08:00", reset: true), now: now, calendar: shanghai) == "0% (reset)")
    #expect(Format.value(window(12.4, resets: "2026-10-30 18:00"), now: now, calendar: shanghai) == "12% · 18:00")
    #expect(Format.value(window(65, resets: "2026-11-04 03:00"), now: now, calendar: shanghai) == "65% · Wed 03:00")
    #expect(Format.value(window(79, resets: nil), now: now, calendar: shanghai) == "79%")
}
