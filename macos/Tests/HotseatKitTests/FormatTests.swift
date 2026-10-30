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
    // Beside the account's avatar: its 5-hour use, LIMIT, or "!" on errors.
    let s = try Snapshot.decode(Data(panelFixture.utf8))
    let generated = Date(timeIntervalSince1970: TimeInterval(s.generatedAt))
    #expect(Format.title(nil, failed: true, now: generated) == "!")
    #expect(Format.title(nil, failed: false, now: generated) == "")
    #expect(Format.title(s, failed: false, now: generated) == "20%")
    #expect(Format.title(s, failed: true, now: generated) == "!")
    let reset = try Snapshot.decode(Data(panelFixture.replacingOccurrences(of: "\"current\": \"claude3\"", with: "\"current\": \"claude2\"").utf8))
    #expect(Format.title(reset, failed: false, now: generated) == "0%")
    let limited = try Snapshot.decode(Data(panelFixture.replacingOccurrences(of: "\"current\": \"claude3\"", with: "\"current\": \"claude\"").utf8))
    #expect(Format.title(limited, failed: false, now: generated) == "LIMIT")
}

@Test func titleWithoutCurrentAccount() throws {
    let empty = try Snapshot.decode(Data("""
    {"generatedAt":1,"current":"","desktop":{"managed":false,"running":false,"runningKnown":true,"pendingRecovery":false},"accounts":[]}
    """.utf8))
    #expect(Format.title(empty, failed: false) == "")
}

@Test func windowValueText() {
    #expect(Format.value(nil, now: now, calendar: shanghai) == "?")
    #expect(Format.value(window(0, resets: "2026-10-30 08:00", reset: true), now: now, calendar: shanghai) == "0% (reset)")
    #expect(Format.value(window(12.4, resets: "2026-10-30 14:30"), now: now, calendar: shanghai) == "12% · in 4h 30m")
    #expect(Format.value(window(12, resets: "2026-10-30 18:00"), now: now, calendar: shanghai) == "12% · in 8h")
    #expect(Format.value(window(12, resets: "2026-10-30 10:42"), now: now, calendar: shanghai) == "12% · in 42m")
    #expect(Format.value(window(65, resets: "2026-11-04 03:00"), now: now, calendar: shanghai) == "65% · Wed 03:00")
    #expect(Format.value(window(79, resets: nil), now: now, calendar: shanghai) == "79%")
}

@Test func limitText() {
    #expect(Format.limit(LimitHit(message: "", resetsAt: ts("2026-10-30 15:30"), observedAt: 0), now: now, calendar: shanghai) == "Limit · resets in 5h 30m")
    #expect(Format.limit(LimitHit(message: "", resetsAt: ts("2026-11-01 18:00"), observedAt: 0), now: now, calendar: shanghai) == "Limit until Sun 18:00")
    #expect(Format.limit(LimitHit(message: "", resetsAt: 0, observedAt: 0), now: now, calendar: shanghai) == "Limit reached")
    #expect(Format.limit(nil, now: now, calendar: shanghai) == "Limit reached")
}

@Test func displayNames() throws {
    func info(_ note: String?, _ email: String?) throws -> AccountInfo {
        var fields = ["\"account\":\"claude2\"", "\"current\":false", "\"limitActive\":false", "\"desktop\":\"none\""]
        if let note { fields.append("\"note\":\"\(note)\"") }
        if let email { fields.append("\"email\":\"\(email)\"") }
        return try JSONDecoder().decode(AccountInfo.self, from: Data(("{" + fields.joined(separator: ",") + "}").utf8))
    }
    // Hotseat's internal names (claude2...) only show when nothing else is known.
    let both = try info("Work", "sam.w@example.com")
    #expect(both.displayName == "Work" && both.detail == "sam.w@example.com")
    let noteOnly = try info("Work", nil)
    #expect(noteOnly.displayName == "Work" && noteOnly.detail == nil)
    let emailOnly = try info(nil, "sam.w@example.com")
    #expect(emailOnly.displayName == "sam.w" && emailOnly.detail == "sam.w@example.com")
    let bare = try info(nil, nil)
    #expect(bare.displayName == "claude2" && bare.detail == nil)
}

// A snapshot can be old when refreshing keeps failing; what has reset by
// now must not show as used or limited.
@Test func staleWindowPastReset() throws {
    #expect(Format.value(window(80, resets: "2026-10-30 09:00"), now: now, calendar: shanghai) == "0% (reset)")
    #expect(window(80, resets: "2026-10-30 09:00").percent(now: now) == 0)
    #expect(window(80, resets: "2026-10-30 11:00").percent(now: now) == 80)
    let s = try Snapshot.decode(Data(panelFixture.utf8))
    let limited = s.accounts[0]  // limit resets 1793527200
    #expect(limited.isLimited(now: Date(timeIntervalSince1970: 1793527100)))
    #expect(!limited.isLimited(now: Date(timeIntervalSince1970: 1793527300)))
    let later = Date(timeIntervalSince1970: 1793345500)  // after claude3's 5h reset
    #expect(Format.title(s, failed: false, now: later) == "0%")
}
