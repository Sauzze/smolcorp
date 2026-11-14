import XCTest
@testable import AgentDeck

/// `Disclaimed.process`: the app's programs start through smolcorp-spawn, on their own account (SANDBOX-PLAN §3.5,
/// ST1s). These run only stand-ins in the test's own folder: a shell script, and the real smolcorp-spawn and ST1's C
/// stand-in (internal/disclaim/testdata/standin.c) built here with cc. No permission is asked for and no TCC
/// database is read or changed: who is responsible for a process is read with macOS's own call, on stand-ins.
final class DisclaimedTests: XCTestCase {
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    private func script(_ name: String, _ body: String) throws -> URL {
        let file = folder.appendingPathComponent(name)
        try "#!/bin/bash\n\(body)\n".write(to: file, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: file.path)
        return file
    }

    private func ran(_ process: Process) throws -> String {
        let out = Pipe()
        process.standardOutput = out
        try process.run()
        let data = out.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        return String(decoding: data, as: UTF8.self)
    }

    /// A program that leaves a mark when it runs, and the mark.
    private func marking() throws -> (program: URL, mark: URL) {
        let mark = folder.appendingPathComponent("ran")
        return (try script("program", "touch '\(mark.path)'; printf '%s|' \"$0\" \"$@\""), mark)
    }

    func testTheSuiteIsNoAppAndTheAppsSpawnIsBesideTheCore() {
        XCTAssertFalse(Disclaimed.app.inApp, "swift test isn't Smolcorp.app")
        XCTAssertEqual(Disclaimed.app.spawn, Paths.bundledCLI.deletingLastPathComponent().appendingPathComponent("smolcorp-spawn"))
    }

    func testWithSmolcorpSpawnTheProgramGoesThroughItWithItsWordsAsTheyAre() throws {
        let (program, mark) = try marking()
        // A stand-in smolcorp-spawn: notes what it was given, then becomes PROGRAM with ARGV0, as the real one does.
        let given = folder.appendingPathComponent("given")
        let spawn = try script("smolcorp-spawn", "printf '%s|' \"$@\" > '\(given.path)'; exec -a \"$2\" \"$1\" \"${@:3}\"")
        let words = ["two words", "", "$(touch nope)", "*", "-c"]
        let process = try Disclaimed.process(program, words, place: .init(spawn: spawn, inApp: true))
        XCTAssertEqual(process.executableURL, spawn)
        XCTAssertEqual(process.arguments, [program.path, program.path] + words)
        XCTAssertEqual(try ran(process), ([program.path] + words).map { $0 + "|" }.joined())
        XCTAssertEqual(try String(contentsOf: given, encoding: .utf8), ([program.path, program.path] + words).map { $0 + "|" }.joined())
        XCTAssertTrue(FileManager.default.fileExists(atPath: mark.path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: folder.appendingPathComponent("nope").path), "no shell")
    }

    func testInAnAppWithoutSmolcorpSpawnNothingStarts() throws {
        let (program, mark) = try marking()
        let notRunnable = try script("not-runnable", "true")
        try FileManager.default.setAttributes([.posixPermissions: 0o644], ofItemAtPath: notRunnable.path)
        for spawn in [folder.appendingPathComponent("missing"), folder!, notRunnable] {
            XCTAssertThrowsError(try Disclaimed.process(program, ["x"], place: .init(spawn: spawn, inApp: true)), spawn.path) { error in
                XCTAssertEqual(error.localizedDescription,
                               "Smolcorp didn't start program: its smolcorp-spawn is missing, so it would have held Smolcorp's permissions. Reinstall Smolcorp.")
            }
        }
        XCTAssertFalse(FileManager.default.fileExists(atPath: mark.path), "the program never ran")
    }

    func testOutsideAnAppWithoutSmolcorpSpawnTheProgramRunsAsBefore() throws {
        let (program, mark) = try marking()
        let process = try Disclaimed.process(program, ["a b"], place: .init(spawn: folder.appendingPathComponent("missing"), inApp: false))
        XCTAssertEqual(process.executableURL, program)
        XCTAssertEqual(process.arguments, ["a b"])
        XCTAssertEqual(try ran(process), "\(program.path)|a b|")
        XCTAssertTrue(FileManager.default.fileExists(atPath: mark.path))
    }

    func testAProgramThatIsntThereOrCantRunThrowsAsProcessWould() throws {
        let spawn = try script("smolcorp-spawn", "exit 0")
        let notRunnable = try script("not-runnable", "true")
        try FileManager.default.setAttributes([.posixPermissions: 0o644], ofItemAtPath: notRunnable.path)
        for program in [folder.appendingPathComponent("missing"), folder!, notRunnable] {
            XCTAssertThrowsError(try Disclaimed.process(program, [], place: .init(spawn: spawn, inApp: true)), program.path)
        }
    }

    // MARK: The real smolcorp-spawn

    private var repo: URL { URL(fileURLWithPath: #filePath).deletingLastPathComponent().appendingPathComponent("../..").standardized }

    /// A C file built into the test's folder, or the test skipped if this Mac can't build C. The developer folder is
    /// checked first: with none, /usr/bin/cc would offer to install the tools in a window.
    private func built(_ name: String, from source: URL) throws -> URL {
        let select = Process()
        select.executableURL = URL(fileURLWithPath: "/usr/bin/xcode-select")
        select.arguments = ["-p"]
        let developer = (try? ran(select))?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard select.terminationStatus == 0, !developer.isEmpty, FileManager.default.fileExists(atPath: developer) else {
            throw XCTSkip("no developer tools here")
        }
        let program = folder.appendingPathComponent(name)
        let cc = Process()
        cc.executableURL = URL(fileURLWithPath: "/usr/bin/cc")
        cc.arguments = ["-Wall", "-Wextra", "-Werror", "-o", program.path, source.path]
        cc.standardError = FileHandle.nullDevice
        _ = try ran(cc)
        guard cc.terminationStatus == 0 else { throw XCTSkip("cc can't build \(name) here") }
        return program
    }

    /// smolcorp-spawn, built and signed ad hoc with the hardened runtime as the app's copy is, and the stand-in.
    private func spawnAndStandIn() throws -> (spawn: URL, standIn: URL) {
        let spawn = try built("smolcorp-spawn", from: repo.appendingPathComponent("packaging/macos/smolcorp-spawn.c"))
        let sign = Process()
        sign.executableURL = URL(fileURLWithPath: "/usr/bin/codesign")
        sign.arguments = ["--force", "--sign", "-", "--options", "runtime", spawn.path]
        sign.standardError = FileHandle.nullDevice
        _ = try ran(sign)
        XCTAssertEqual(sign.terminationStatus, 0, "signing the stand-in smolcorp-spawn ad hoc")
        return (spawn, try built("standin", from: repo.appendingPathComponent("internal/disclaim/testdata/standin.c")))
    }

    /// What the stand-in said of itself: its pid and the pid responsible for it.
    private static func pids(_ said: String) -> (pid: Int32, responsible: Int32)? {
        guard let match = said.range(of: #"pid=(\d+) responsible=(-?\d+)"#, options: .regularExpression) else { return nil }
        let numbers = said[match].split(whereSeparator: { !$0.isNumber && $0 != "-" }).compactMap { Int32($0) }
        return numbers.count == 2 ? (numbers[0], numbers[1]) : nil
    }

    /// The pid, signals and exit status the app sees are the program's own: smolcorp-spawn becomes it in place.
    func testThroughTheRealSmolcorpSpawnThePidAndTheStatusAreTheProgramsOwn() throws {
        let (spawn, standIn) = try spawnAndStandIn()
        let place = Disclaimed.Place(spawn: spawn, inApp: true)
        let exiting = try Disclaimed.process(standIn, ["exit", "3"], place: place)
        let said = try ran(exiting)
        XCTAssertEqual(exiting.terminationStatus, 3)
        XCTAssertEqual(Self.pids(said)?.pid, exiting.processIdentifier, said)
        XCTAssertTrue(said.contains("arg[0]=[\(standIn.path)]"), said)

        let waiting = try Disclaimed.process(standIn, ["wait"], place: place)
        let out = Pipe()
        waiting.standardOutput = out
        try waiting.run()
        _ = out.fileHandleForReading.availableData // it has started, and said so
        waiting.terminate()
        waiting.waitUntilExit()
        XCTAssertEqual(waiting.terminationReason, .uncaughtSignal)
        XCTAssertEqual(waiting.terminationStatus, SIGTERM)
    }

    /// The proof (SANDBOX-PLAN §3.5), as ST1's for the core: a program the app starts with `Disclaimed.process` is
    /// responsible for itself, not for the app. The holder stands in for Smolcorp.app: this test bundle run again,
    /// started through smolcorp-spawn so it is responsible for itself, as launchd starts an app. It then starts the
    /// stand-in twice: as the app did before (a plain Process: charged to the holder, which shows the check can see
    /// a charge at all), and with `Disclaimed.process`.
    func testAProgramTheAppStartsIsResponsibleForItselfNotForSmolcorp() throws {
        let (spawn, standIn) = try spawnAndStandIn()
        let xctest = CommandLine.arguments[0]
        guard (xctest as NSString).lastPathComponent == "xctest" else { throw XCTSkip("not run by xctest: \(xctest)") }
        let holder = Process()
        holder.executableURL = spawn
        holder.arguments = [xctest, xctest, "-XCTest", "AgentDeckTests.DisclaimedTests/testHolderForTheProof",
                            Bundle(for: Self.self).bundlePath]
        holder.environment = ProcessInfo.processInfo.environment.merging(
            ["SMOLCORP_PROOF_HOLDER": folder.path, "SMOLCORP_PROOF_SPAWN": spawn.path, "SMOLCORP_PROOF_STANDIN": standIn.path]) { $1 }
        let log = folder.appendingPathComponent("holder.log")
        FileManager.default.createFile(atPath: log.path, contents: nil)
        let logHandle = try FileHandle(forWritingTo: log)
        holder.standardOutput = logHandle
        holder.standardError = logHandle
        try holder.run()
        let stop = DispatchWorkItem { if holder.isRunning { holder.terminate() } }
        DispatchQueue.global().asyncAfter(deadline: .now() + 120, execute: stop)
        holder.waitUntilExit()
        stop.cancel()
        try logHandle.close()
        let output = (try? String(contentsOf: log, encoding: .utf8)) ?? ""
        let said = try XCTUnwrap(try? String(contentsOf: folder.appendingPathComponent("said"), encoding: .utf8),
                                 "the holder said nothing (\(holder.terminationStatus)):\n\(output.suffix(2000))")
        print(said) // for the report: the pids seen
        let lines = said.split(separator: "\n").map(String.init)
        let holderSaid = try XCTUnwrap(lines.first { $0.hasPrefix("holder ") }.flatMap(Self.pids), said)
        let plain = try XCTUnwrap(lines.first { $0.hasPrefix("plain ") }.flatMap(Self.pids), said)
        let disclaimed = try XCTUnwrap(lines.first { $0.hasPrefix("disclaimed ") }.flatMap(Self.pids), said)
        XCTAssertEqual(holderSaid.responsible, holderSaid.pid, "the holder is responsible for itself, as an app is")
        XCTAssertEqual(plain.responsible, holderSaid.pid, "the control: a plain start is charged to the holder")
        XCTAssertEqual(disclaimed.responsible, disclaimed.pid, "started with Disclaimed.process: responsible for itself")
        XCTAssertNotEqual(disclaimed.responsible, holderSaid.pid, "never charged to the holder")
    }

    /// The holder's part of the proof above; skipped in the suite's own run.
    func testHolderForTheProof() throws {
        let environment = ProcessInfo.processInfo.environment
        guard let folder = environment["SMOLCORP_PROOF_HOLDER"], let spawn = environment["SMOLCORP_PROOF_SPAWN"],
              let standIn = environment["SMOLCORP_PROOF_STANDIN"] else { throw XCTSkip("only run as the proof's holder") }
        typealias Responsible = @convention(c) (pid_t) -> pid_t
        let call = dlsym(dlopen(nil, RTLD_NOW), "responsibility_get_pid_responsible_for_pid")
        let mine = call.map { unsafeBitCast($0, to: Responsible.self)(getpid()) } ?? -1
        let program = URL(fileURLWithPath: standIn)
        let plain = Process()
        plain.executableURL = program
        let disclaimed = try Disclaimed.process(program, [], place: .init(spawn: URL(fileURLWithPath: spawn), inApp: true))
        let said = "holder pid=\(getpid()) responsible=\(mine)\nplain \(try ran(plain))disclaimed \(try ran(disclaimed))"
        try said.write(toFile: (folder as NSString).appendingPathComponent("said"), atomically: true, encoding: .utf8)
    }
}
