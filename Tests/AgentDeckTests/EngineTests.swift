import XCTest
@testable import AgentDeck

/// The engine switch, and the Go core as this app starts, asks and stops it,
/// on a stand-in core: a few lines of shell that answer as the core does.
final class EngineTests: XCTestCase {
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("se-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    func testOnlyATestCopyRunsTheGoEngine() {
        for setting in [nil, "swift", "rust", "GO", ""] as [String?] {
            let (engine, refusal) = Engine.chosen(setting, testCopy: true)
            XCTAssertEqual(engine, .swift, "\(String(describing: setting))")
            XCTAssertNil(refusal)
        }
        XCTAssertEqual(Engine.chosen("go", testCopy: true).engine, .go)
        let (engine, refusal) = Engine.chosen("go", testCopy: false)
        XCTAssertEqual(engine, .swift, "the copy in use keeps its own engine")
        XCTAssertEqual(refusal, Engine.refusal)
        XCTAssertEqual(refusal?.filter { $0 == "." }.count, 1, "one sentence")
    }

    func testTheSettingIsKeptAndNeverAdded() throws {
        let file = folder.appendingPathComponent("config.json")
        try Data(#"{"hasCompletedSetup": true}"#.utf8).write(to: file)
        let config = Config.load(from: file)
        XCTAssertNil(config.engine)
        config.save(to: file)
        XCTAssertFalse(try String(contentsOf: file).contains("engine"), "a file without it stays without it")

        try Data(#"{"engine": "go"}"#.utf8).write(to: file)
        let set = Config.load(from: file)
        XCTAssertEqual(set.engine, "go")
        set.save(to: file)
        XCTAssertEqual(Config.load(from: file).engine, "go")
        XCTAssertFalse(set.lostOnLoad)
    }

    private func core(_ script: String, port: UInt16 = 7779) throws -> GoCore {
        let program = folder.appendingPathComponent("fake-core")
        try Data("#!/bin/sh\n\(script)\n".utf8).write(to: program)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: program.path)
        let bridge = HostBridge(socket: folder.appendingPathComponent("bridge.sock"), token: "fake-token-fake-token-fake-token-0")
        return GoCore(program: program, port: port, support: folder, bridge: bridge)
    }

    /// A stand-in that reads the token first, as the core does, then says it's
    /// ready and answers each line with its id. It ends at once (9) without the token.
    private let answering = #"""
        read token
        [ "$token" = "fake-token-fake-token-fake-token-0" ] || exit 9
        echo '{"ready":true,"port":7779,"cookie":"fake-cookie"}'
        while read line; do
          id=$(echo "$line" | sed -E 's/.*"id":([0-9]+).*/\1/')
          echo "{\"id\":$id,\"key\":\"fake/key\",\"session\":\"fake-session\",\"cookie\":\"smolcorp-setup\",\"link\":\"http://localhost:7779/setup#o=fake\"}"
        done
        """#

    func testTheTokenIsNeitherOnTheCommandLineNorInTheEnvironment() throws {
        let core = try core("exit 0")
        XCTAssertEqual(core.arguments, ["core", "--port", "7779", "--support", folder.path, "--bridge", folder.appendingPathComponent("bridge.sock").path])
        XCTAssertFalse(core.arguments.joined(separator: " ").contains(core.bridge.token))
        let environment = core.environment(base: ["HOME": "/Users/me", "PATH": "/usr/bin:/bin", "SHELL": "/bin/zsh",
                                                  "GH_TOKEN": "fake-gh", "ANTHROPIC_API_KEY": "sk-ant-fake", "SMOLCORP_SESSION": "s1"])
        XCTAssertFalse(environment.contains { $0.key.contains(core.bridge.token) || $0.value.contains(core.bridge.token) },
                       "other processes of yours read a process's environment (ps -E)")
        XCTAssertFalse(environment.keys.contains("SMOLCORP_BRIDGE_TOKEN"))
        XCTAssertEqual(environment["SMOLCORP_PORT"], "7779")
        XCTAssertEqual(environment["SMOLCORP_TEST_SUPPORT_DIR"], folder.path)
        XCTAssertEqual(environment["HOME"], "/Users/me")
        XCTAssertEqual(Set(environment.keys), ["HOME", "PATH", "SHELL", "SMOLCORP_PORT", "SMOLCORP_TEST_SUPPORT_DIR"],
                       "nothing else of a terminal Smolcorp was opened from: not its tokens")
    }

    func testItStartsAsksAndStops() throws {
        let core = try core(answering)
        try core.start(waitingUpTo: 5)
        let pid = try XCTUnwrap(core.pid)
        XCTAssertEqual(core.bridge.core, pid, "the only process its bridge serves")
        XCTAssertEqual(core.setupCookie, "fake-cookie", "as the core named it when ready")
        XCTAssertEqual(core.officeAddress()?.absoluteString, "http://127.0.0.1:7779/office?window&t=fake%2Fkey")
        XCTAssertEqual(core.setupSession()?.session, "fake-session")
        XCTAssertEqual(core.setupSession()?.cookie, "smolcorp-setup")
        XCTAssertEqual(core.setupLink(then: "pair")?.absoluteString, "http://localhost:7779/setup#o=fake")
        XCTAssertTrue(core.stop(grace: 2), "its input ended, and it went")
        XCTAssertTrue(GoCore.isGone(pid))
        XCTAssertNil(core.bridge.core, "and none once it's gone")
        XCTAssertNil(core.ask("office"), "nothing to ask once stopped")
    }

    /// What the real one sees: its arguments, its environment and its first line of input, written to a file.
    func testTheTokenIsTheFirstLineOnItsInputAndNowhereElse() throws {
        let seen = folder.appendingPathComponent("seen")
        let core = try core("""
            { echo "$@"; env; } > '\(seen.path)'
            read token
            echo "first line: $token" >> '\(seen.path)'
            echo '{"ready":true}'
            while read line; do :; done
            """)
        try core.start(waitingUpTo: 5)
        core.stop(grace: 1)
        let text = try String(contentsOf: seen)
        XCTAssertTrue(text.hasSuffix("first line: \(core.bridge.token)\n"), "on its input, before anything else")
        XCTAssertEqual(text.components(separatedBy: core.bridge.token).count, 2, "and only there: not in its arguments or its environment")
    }

    func testOneThatWontGoIsStoppedByItsPid() throws {
        let core = try core("trap '' TERM\necho '{\"ready\":true}'\nwhile true; do sleep 1; done")
        try core.start(waitingUpTo: 5)
        let pid = try XCTUnwrap(core.pid)
        XCTAssertTrue(core.stop(grace: 0.3))
        XCTAssertTrue(GoCore.isGone(pid))
    }

    func testOneThatNeverGetsReadyIsSaidSo() throws {
        XCTAssertThrowsError(try core("exit 3").start(waitingUpTo: 5)) { error in
            XCTAssertTrue(error.localizedDescription.contains("ended (3)"), error.localizedDescription)
        }
        let silent = try core("sleep 30")
        XCTAssertThrowsError(try silent.start(waitingUpTo: 0.5))
        XCTAssertTrue(silent.pid.map(GoCore.isGone) ?? true, "and stopped")
        let missing = GoCore(program: folder.appendingPathComponent("none"), port: 7779, support: folder,
                             bridge: HostBridge(socket: folder.appendingPathComponent("bridge.sock")))
        XCTAssertThrowsError(try missing.start()) { XCTAssertEqual($0.localizedDescription, GoCore.Failure.noProgram.localizedDescription) }
    }

    func testAnUnansweredQuestionIsNil() throws {
        let core = try core("echo '{\"ready\":true}'\nwhile read line; do echo '{\"id\":99}'; done")
        try core.start(waitingUpTo: 5)
        XCTAssertNil(core.ask("office", waitingUpTo: 0.3), "an answer to another question isn't this one's")
        core.stop(grace: 1)
    }

    func testTheSettingsWindowUsesTheEnginesCookie() throws {
        let cookie = try XCTUnwrap(SettingsWindow.sessionCookie(session: "abc", name: "smolcorp-setup"))
        XCTAssertEqual(cookie.name, "smolcorp-setup")
        XCTAssertEqual(SettingsWindow.sessionCookie(session: "abc")?.name, DeckRouter.setupCookie, "the Swift engine's, as before")
    }
}
