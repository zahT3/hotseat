import Foundation
import Testing
@testable import HotseatKit

private func status(_ json: String) throws -> SetupStatus {
    try JSONDecoder().decode(SetupStatus.self, from: Data(json.utf8))
}

private let partial = """
{"version":"0.1.0","accounts":0,"loginsFound":2,"shareActions":0,"shim":false,"statusline":true,"hook":true,"desktop":"absent"}
"""

@Test func setupStepsFromStatus() throws {
    let steps = try status(partial).steps(bundledVersion: "0.1.0", usingBundled: false)
    #expect(steps.map(\.id) == ["cli", "accounts", "share", "shim", "tracking", "desktop"])
    #expect(steps.map(\.done) == [true, false, false, false, true, true])
    #expect(steps[1].commands == [["init"]])
    #expect(steps[2].commands == [["share", "apply", "--json"]])
    #expect(steps[3].commands == [["shim", "install"]])
    #expect(steps[4].commands == [["statusline", "install"], ["hook", "install"]])
    #expect(steps[5].commands == [["desktop", "adopt"]])
    #expect(steps[0].commands == nil)
}

@Test func blankMachineAccountsDetail() throws {
    let s = try status(partial.replacingOccurrences(of: "\"loginsFound\":2", with: "\"loginsFound\":0"))
    let accounts = s.steps(bundledVersion: "0.1.0", usingBundled: false)[1]
    #expect(accounts.detail.contains("run `claude` once"))
}

@Test func cliStepNeedsMatchingVersion() throws {
    let s = try status(partial)
    #expect(!s.steps(bundledVersion: "0.1.0", usingBundled: true)[0].done)
    #expect(!s.steps(bundledVersion: "0.2.0", usingBundled: false)[0].done)
    #expect(s.steps(bundledVersion: "0.1.0", usingBundled: false)[0].done)
}

@Test func desktopAvailableIsPending() throws {
    let s = try status(partial.replacingOccurrences(of: "\"desktop\":\"absent\"", with: "\"desktop\":\"available\""))
    #expect(!s.steps(bundledVersion: "0.1.0", usingBundled: false)[5].done)
}
