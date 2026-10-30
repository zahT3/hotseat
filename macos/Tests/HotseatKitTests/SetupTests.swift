import Foundation
import Testing
@testable import HotseatKit

private func status(_ json: String) throws -> SetupStatus {
    try JSONDecoder().decode(SetupStatus.self, from: Data(json.utf8))
}

private let partial = """
{"version":"0.1.0","accounts":0,"loginsFound":2,"shareActions":0,"shareConflicts":0,"shim":false,"statusline":true,"hook":true,"desktop":"absent"}
"""

@Test func setupStepsFromStatus() throws {
    let steps = try status(partial).steps()
    #expect(steps.map(\.id) == ["accounts", "share", "shim", "tracking", "desktop"])
    #expect(steps.map(\.done) == [false, false, false, true, true])
    #expect(steps[0].commands == [["init"]])
    #expect(steps[1].commands == [["share", "apply", "--json"]])
    #expect(steps[2].commands == [["shim", "install"]])
    #expect(steps[3].commands == [["statusline", "install"], ["hook", "install"]])
    #expect(steps[4].commands == [["desktop", "adopt"]])
}

@Test func blankMachineAccountsDetail() throws {
    let s = try status(partial.replacingOccurrences(of: "\"loginsFound\":2", with: "\"loginsFound\":0"))
    #expect(s.steps()[0].detail.contains("run claude once"))
}

@Test func desktopAvailableIsPending() throws {
    let s = try status(partial.replacingOccurrences(of: "\"desktop\":\"absent\"", with: "\"desktop\":\"available\""))
    #expect(!s.steps()[4].done)
}

// An older CLI may not know newer fields.
@Test func decodesStatusWithoutNewerFields() throws {
    let s = try status(#"{"version":"0.1.0","accounts":1,"loginsFound":1,"shareActions":0,"shim":true,"statusline":true,"hook":true,"desktop":"absent"}"#)
    #expect(s.shareConflicts == 0)
}

@Test func shareConflictsDoNotBlock() throws {
    let s = try status(partial.replacingOccurrences(of: "\"accounts\":0", with: "\"accounts\":2")
        .replacingOccurrences(of: "\"shareConflicts\":0", with: "\"shareConflicts\":2"))
    let share = s.steps()[1]
    #expect(share.done && share.detail.contains("2 files differ"))
}

@Test func cliStepCases() {
    func cli(_ installed: String?, bundledOnly: Bool = false, statusWorks: Bool = true) -> SetupStep {
        SetupStep.cli(installedVersion: installed, installedPath: "/opt/homebrew/bin/hotseat",
                      usingBundled: bundledOnly, statusWorks: statusWorks, bundledVersion: "0.2.0")
    }
    #expect(!cli(nil, bundledOnly: true).done)
    #expect(!cli("0.1.0").done && cli("0.1.0").detail.contains("0.1.0"))
    #expect(cli("0.2.0").done)
    #expect(cli("0.3.0").done)  // newer: never offer a downgrade
    let old = cli("0.2.0", statusWorks: false)  // same number, but no `setup` command
    #expect(!old.done && old.detail.contains("/opt/homebrew/bin/hotseat"))
}

@Test func versionOrder() {
    #expect(Version.isOlder("0.1.0", than: "0.2.0"))
    #expect(!Version.isOlder("0.10.0", than: "0.9.1"))
    #expect(!Version.isOlder("1.0", than: "1.0.0"))
    #expect(Version.isOlder("0.1.0", than: "0.1.1"))
}
