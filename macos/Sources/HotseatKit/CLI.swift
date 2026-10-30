import Foundation

extension HotseatError {
    /// One line for the menu or an alert.
    public var message: String {
        switch self {
        case .notFound(let tried):
            return "hotseat CLI not found; tried: " + tried.joined(separator: ", ")
        case .failed(let code, let stderr):
            return stderr.isEmpty ? "hotseat exited with \(code)" : "hotseat exited with \(code): \(stderr)"
        case .timedOut(let seconds):
            return "hotseat did not answer within \(seconds) s"
        case .badOutput(let prefix):
            return "hotseat printed something unexpected: \(prefix)"
        }
    }
}

public enum UseOutcome: Equatable, Sendable {
    case done
    /// The CLI switched, but Claude Desktop is running and would have to
    /// quit; ask, then call `use` again with `restartDesktop: true`.
    case needsDesktopRestart
}

/// Runs the hotseat command line tool. The app reads and changes nothing
/// itself; every action goes through here.
public struct HotseatCLI: Sendable {
    public let executable: URL

    public init(executable: URL) {
        self.executable = executable
    }

    /// Finds hotseat by absolute path: apps started from Finder or at login
    /// do not get the shell's PATH.
    public static func locate(
        override: String?, home: URL,
        isExecutable: (String) -> Bool = { FileManager.default.isExecutableFile(atPath: $0) }
    ) throws -> HotseatCLI {
        var tried: [String] = []
        if let override, !override.isEmpty {
            tried.append(override)
        }
        tried += [home.appendingPathComponent(".local/bin/hotseat").path, "/opt/homebrew/bin/hotseat", "/usr/local/bin/hotseat"]
        guard let found = tried.first(where: isExecutable) else {
            throw HotseatError.notFound(tried: tried)
        }
        return HotseatCLI(executable: URL(fileURLWithPath: found))
    }

    public func snapshot() async throws -> Snapshot {
        try Snapshot.decode(try await run(["panel", "--json"], timeout: .seconds(30)))
    }

    /// Switches the CLI and Claude Desktop to `account`. Without
    /// `restartDesktop`, a running Desktop is left alone and the result says
    /// so; hotseat checks that itself, on live process state.
    public func use(_ account: String, restartDesktop: Bool) async throws -> UseOutcome {
        var args = ["use", account, "--json"]
        if restartDesktop {
            args.insert("--yes", at: 2)
        }
        let (status, _, stderr) = try await runRaw(args, timeout: .seconds(120))
        switch status {
        case 0: return .done
        case 3 where !restartDesktop: return .needsDesktopRestart
        default: throw HotseatError.failed(code: status, stderr: stderr)
        }
    }

    /// Sets the user's label for `account`; blank text clears it.
    public func setNote(_ account: String, _ note: String) async throws {
        let text = note.trimmingCharacters(in: .whitespacesAndNewlines)
        _ = try await run(["note", account] + (text.isEmpty ? [] : [text]) + ["--json"], timeout: .seconds(30))
    }

    /// Gives `account` an empty Claude Desktop profile and opens Desktop on
    /// it to log in (quits a running Desktop first).
    public func setUpDesktop(_ account: String) async throws {
        _ = try await run(["desktop", "add", account, "--yes"], timeout: .seconds(120))
    }

    /// Moves only new terminal sessions to `account`; Desktop stays.
    public func useCLIOnly(_ account: String) async throws {
        _ = try await run(["use", account, "--cli-only", "--json"], timeout: .seconds(30))
    }

    /// Moves only Claude Desktop to `account`, reopening the session that
    /// was focused (quits a running Desktop first).
    public func switchDesktop(_ account: String) async throws {
        _ = try await run(["desktop", "switch", account, "--resume", "current", "--json"], timeout: .seconds(120))
    }

    /// Signs `account` out and moves its files into a Hotseat backup.
    public func remove(_ account: String) async throws {
        _ = try await run(["remove", account, "--yes", "--json"], timeout: .seconds(60))
    }

    /// Sets the symbol shown for `account`; nil clears it.
    public func setIcon(_ account: String, _ icon: String?) async throws {
        _ = try await run(["icon", account] + (icon.map { [$0] } ?? []) + ["--json"], timeout: .seconds(30))
    }

    /// Runs hotseat off the main thread and returns its stdout.
    public func run(_ args: [String], timeout: Duration) async throws -> Data {
        let (status, out, stderr) = try await runRaw(args, timeout: timeout)
        if status != 0 {
            throw HotseatError.failed(code: status, stderr: stderr)
        }
        return out
    }

    private func runRaw(_ args: [String], timeout: Duration) async throws -> (Int32, Data, String) {
        let exe = executable
        return try await withCheckedThrowingContinuation { cont in
            DispatchQueue.global().async {
                cont.resume(with: Result { try Self.runBlocking(exe, args, timeout: timeout) })
            }
        }
    }

    private final class Buffer: @unchecked Sendable {
        var data = Data()
    }

    private static func runBlocking(_ exe: URL, _ args: [String], timeout: Duration) throws -> (Int32, Data, String) {
        let p = Process()
        p.executableURL = exe
        p.arguments = args
        p.standardInput = FileHandle.nullDevice
        let out = Pipe(), err = Pipe()
        p.standardOutput = out
        p.standardError = err
        let exited = DispatchSemaphore(value: 0)
        p.terminationHandler = { _ in exited.signal() }
        do {
            try p.run()
        } catch {
            throw HotseatError.failed(code: -1, stderr: error.localizedDescription)
        }
        // Drain both pipes while the process runs, so neither can fill up
        // and block it.
        let outBuf = Buffer(), errBuf = Buffer()
        let drained = DispatchGroup()
        for (pipe, buf) in [(out, outBuf), (err, errBuf)] {
            DispatchQueue.global().async(group: drained) {
                buf.data = pipe.fileHandleForReading.readDataToEndOfFile()
            }
        }
        let seconds = Int(timeout.components.seconds)
        if exited.wait(timeout: .now() + .seconds(seconds)) == .timedOut {
            p.terminate()
            throw HotseatError.timedOut(seconds: seconds)
        }
        drained.wait()
        let text = String(decoding: errBuf.data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
        return (p.terminationStatus, outBuf.data, text)
    }
}
