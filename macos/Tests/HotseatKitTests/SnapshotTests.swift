import Foundation
import Testing
@testable import HotseatKit

@Test func decodesRealShape() throws {
    let s = try Snapshot.decode(Data(panelFixture.utf8))
    #expect(s.current == "claude3")
    #expect(s.recommend == "claude2")
    #expect(s.desktop.managed && s.desktop.active == "claude3" && !s.desktop.running)
    #expect(s.accounts.map(\.account) == ["claude", "claude2", "claude3"])
    #expect(s.accounts[0].fiveHour == nil)
    #expect(s.accounts[0].limitActive && s.accounts[0].limit?.resetsAt == 1793527200)
    #expect(s.accounts[0].desktopProblem == nil && s.accounts[0].error == nil)
    #expect(s.accounts[1].fiveHour?.reset == true)
    #expect(s.accounts[1].desktopProblem == "not logged in yet")
    #expect(s.accounts[1].configDir == "/Users/u/.claude2" && s.accounts[0].configDir == nil)
    #expect(s.accounts[1].loginProblem == "not logged in" && s.accounts[0].loginProblem == nil)
    #expect(s.accounts[2].sevenDay?.usedPercent == 79)
    #expect(s.accounts[2].desktop == "active")
}

@Test func badOutputKeepsPrefix() {
    let junk = String(repeating: "x", count: 300)
    #expect(throws: HotseatError.badOutput(String(repeating: "x", count: 200))) {
        try Snapshot.decode(Data(junk.utf8))
    }
}
