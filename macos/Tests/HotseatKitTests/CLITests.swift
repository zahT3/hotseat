import Foundation
import Testing
@testable import HotseatKit

/// Writes an executable shell script standing in for the hotseat binary.
private func fakeHotseat(_ body: String) throws -> HotseatCLI {
    let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    let url = dir.appendingPathComponent("hotseat")
    try ("#!/bin/sh\n" + body + "\n").write(to: url, atomically: true, encoding: .utf8)
    try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
    return HotseatCLI(executable: url)
}

private let home = URL(fileURLWithPath: "/Users/someone")

@Test func locateOrder() throws {
    let cli = try HotseatCLI.locate(override: "/nope/hotseat", home: home, isExecutable: { $0 == "/opt/homebrew/bin/hotseat" })
    #expect(cli.executable.path == "/opt/homebrew/bin/hotseat")
    let first = try HotseatCLI.locate(override: "/custom/hotseat", home: home, isExecutable: { _ in true })
    #expect(first.executable.path == "/custom/hotseat")
}

@Test func locateNotFound() {
    let defaults = ["/Users/someone/.local/bin/hotseat", "/opt/homebrew/bin/hotseat", "/usr/local/bin/hotseat"]
    #expect(throws: HotseatError.notFound(tried: ["/nope/hotseat"] + defaults)) {
        try HotseatCLI.locate(override: "/nope/hotseat", home: home, isExecutable: { _ in false })
    }
    #expect(throws: HotseatError.notFound(tried: defaults)) {
        try HotseatCLI.locate(override: nil, home: home, isExecutable: { _ in false })
    }
}

@Test func snapshotParses() async throws {
    let fixture = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString + ".json")
    try panelFixture.write(to: fixture, atomically: true, encoding: .utf8)
    let cli = try fakeHotseat("[ \"$1 $2\" = \"panel --json\" ] || exit 9\ncat '\(fixture.path)'")
    let s = try await cli.snapshot()
    #expect(s.current == "claude3" && s.accounts.count == 3)
}

@Test func failureCarriesStderr() async throws {
    let cli = try fakeHotseat("echo boom >&2; exit 1")
    await #expect(throws: HotseatError.failed(code: 1, stderr: "boom")) {
        try await cli.use("claude2", restartDesktop: true)
    }
    #expect(HotseatError.failed(code: 1, stderr: "boom").message == "hotseat exited with 1: boom")
}

@Test func timeoutKills() async throws {
    let cli = try fakeHotseat("exec sleep 30")
    let start = Date()
    await #expect(throws: HotseatError.timedOut(seconds: 1)) {
        _ = try await cli.run(["panel", "--json"], timeout: .seconds(1))
    }
    #expect(Date().timeIntervalSince(start) < 2)
}

@Test func largeOutputDoesNotDeadlock() async throws {
    let cli = try fakeHotseat("head -c 1048576 /dev/zero; head -c 70000 /dev/zero >&2")
    let data = try await cli.run(["panel"], timeout: .seconds(10))
    #expect(data.count == 1_048_576)
}

@Test func useAsksBeforeRestartingDesktop() async throws {
    // Like `hotseat use`: without --yes a running Desktop is left alone and
    // the exit code is 3.
    let cli = try fakeHotseat("""
    [ "$1 $2" = "use claude2" ] || exit 9
    if [ "$3" = "--yes" ]; then echo '{"current":"claude2"}'; exit 0; fi
    echo '{"current":"claude2","desktopNeedsYes":true}'; exit 3
    """)
    #expect(try await cli.use("claude2", restartDesktop: false) == .needsDesktopRestart)
    #expect(try await cli.use("claude2", restartDesktop: true) == .done)
}

@Test func setNotePassesTextAsOneArgument() async throws {
    let cli = try fakeHotseat("""
    [ "$1" = note ] && [ "$2" = claude2 ] && [ "$3" = "Work laptop" ] && [ "$4" = --json ] || { echo "bad args: $*" >&2; exit 9; }
    """)
    try await cli.setNote("claude2", "Work laptop")
    let clear = try fakeHotseat("""
    [ "$#" = 3 ] && [ "$3" = --json ] || { echo "bad args: $*" >&2; exit 9; }
    """)
    try await clear.setNote("claude2", "  ")
}

@Test func addAccountNames() {
    #expect(AddAccount.isValidName("work"))
    #expect(AddAccount.isValidName("claude4.b_2-x"))
    #expect(!AddAccount.isValidName(""))
    #expect(!AddAccount.isValidName("-x"))
    #expect(!AddAccount.isValidName("a b"))
    #expect(!AddAccount.isValidName("a'b"))
}

@Test func addAccountScriptQuotesPath() {
    let s = AddAccount.script(hotseat: URL(fileURLWithPath: "/Users/o'neil/bin/hotseat"), name: "work")
    #expect(s.hasPrefix("#!/bin/zsh -il\n"))
    #expect(s.contains("'/Users/o'\\''neil/bin/hotseat' add 'work'"))
}

@Test func setUpDesktopRunsDesktopAdd() async throws {
    let cli = try fakeHotseat("""
    [ "$*" = "desktop add claude2 --yes" ] || { echo "bad args: $*" >&2; exit 9; }
    """)
    try await cli.setUpDesktop("claude2")
}
