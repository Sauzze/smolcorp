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

    func testEveryMacRunsTheGoEngineWhateverTheFileSays() {
        for setting in [nil, "go"] as [String?] {
            XCTAssertNil(Engine.ignored(setting), "nothing to say about \(String(describing: setting))")
        }
        for setting in ["swift", "rust", "GO", ""] {
            XCTAssertEqual(Engine.ignored(setting)?.split(separator: "\n").count, 1, "one log line for \(setting)")
        }
        XCTAssertTrue(try XCTUnwrap(Engine.ignored("swift")).contains(#""engine": "swift" is ignored"#))
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
    /// ready and answers each line with its id, the menu's state sent unasked
    /// before each answer, as the core may. It ends at once (9) without the token.
    private let answering = #"""
        read token
        [ "$token" = "fake-token-fake-token-fake-token-0" ] || exit 9
        echo '{"ready":true,"port":7779,"cookie":"fake-cookie"}'
        while read line; do
          id=$(echo "$line" | sed -E 's/.*"id":([0-9]+).*/\1/')
          echo "{\"menu\":{\"sessions\":$id,\"waiting\":1,\"portable\":{\"on\":true}}}"
          echo "{\"id\":$id,\"key\":\"fake/key\",\"session\":\"fake-session\",\"cookie\":\"smolcorp-setup\",\"link\":\"http://localhost:7779/setup#o=fake\",\"menu\":{\"sessions\":$id,\"phones\":2}}"
        done
        """#

    func testTheTokenIsNeitherOnTheCommandLineNorInTheEnvironment() throws {
        let core = try core("exit 0")
        XCTAssertEqual(core.arguments, ["core", "--port", "7779", "--support", folder.path, "--bridge", folder.appendingPathComponent("bridge.sock").path])
        XCTAssertFalse(core.arguments.joined(separator: " ").contains(core.bridge.token))
        let environment = core.environment(base: ["HOME": "/Users/me", "PATH": "/usr/bin:/bin", "SHELL": "/bin/zsh",
                                                  "GH_TOKEN": "fake-gh", "ANTHROPIC_API_KEY": "fake-api-key", "SMOLCORP_SESSION": "s1",
                                                  "SSH_AUTH_SOCK": "/fake/agent.sock", "LOGNAME": "me", "LC_CTYPE": "UTF-8", "LC_ALL": "fr_FR.UTF-8",
                                                  "__CF_USER_TEXT_ENCODING": "0x1F5:0x0:0x0", "TMUX_TMPDIR": "/fake/tmux",
                                                  "XPC_SERVICE_NAME": "0", "LCX": "no", "LC_TERMINAL": "iTerm2", "LC_TOKEN": "fake-secret", "SMOLCORP_HIRE_KEY": "fake-send-key"])
        XCTAssertFalse(environment.contains { $0.key.contains(core.bridge.token) || $0.value.contains(core.bridge.token) },
                       "other processes of yours read a process's environment (ps -E)")
        XCTAssertFalse(environment.keys.contains("SMOLCORP_BRIDGE_TOKEN"))
        XCTAssertEqual(environment["SMOLCORP_PORT"], "7779")
        XCTAssertEqual(environment["SMOLCORP_TEST_SUPPORT_DIR"], folder.path)
        XCTAssertEqual(environment["HOME"], "/Users/me")
        XCTAssertEqual(environment["SSH_AUTH_SOCK"], "/fake/agent.sock", "a hire's git push over SSH uses your agent, as on the Swift engine")
        XCTAssertEqual(Set(environment.keys), ["HOME", "PATH", "SHELL", "SMOLCORP_PORT", "SMOLCORP_TEST_SUPPORT_DIR", "SSH_AUTH_SOCK", "LOGNAME",
                                               "LC_CTYPE", "LC_ALL", "__CF_USER_TEXT_ENCODING", "TMUX_TMPDIR"],
                       "nothing else of a terminal Smolcorp was opened from: not its tokens")
    }

    /// FXAGENTHOMEMAC: the core is given each variable that moves an agent's folder when this app has it, as it is,
    /// so a desk's sandbox holds the folder where it went; and nothing else new. Without them it's given none.
    func testTheCoreIsGivenWhereYourAgentsFoldersWereMovedAndNothingElse() throws {
        let core = try core("exit 0")
        XCTAssertEqual(GoCore.agentPlaces.count, 18)
        XCTAssertTrue(GoCore.agentPlaces.contains("CODEX_HOME") && GoCore.agentPlaces.contains("XDG_CONFIG_HOME"))
        var base = ["PATH": "/usr/bin", "CODEX_API_KEY": "fake-key", "OPENAI_API_KEY": "fake-key", "GEMINI_API_KEY": "fake-key",
                    "XAI_API_KEY": "fake-key", "XDG_RUNTIME_DIR": "/fake/run", "XDG_SESSION_ID": "c1", "OPENCODE_SERVER_PASSWORD": "fake-key"]
        for name in GoCore.agentPlaces { base[name] = "/Users/ada/moved/" + name.lowercased() }
        let environment = core.environment(base: base)
        for name in GoCore.agentPlaces {
            XCTAssertEqual(environment[name], "/Users/ada/moved/" + name.lowercased(), name)
        }
        XCTAssertEqual(Set(environment.keys), Set(["PATH", "SMOLCORP_PORT", "SMOLCORP_TEST_SUPPORT_DIR"] + GoCore.agentPlaces),
                       "nothing else new: no key, no runtime folder")
        XCTAssertFalse(environment.values.contains("fake-key"))
        XCTAssertEqual(Set(core.environment(base: ["PATH": "/usr/bin", "XDG_RUNTIME_DIR": "/fake/run"]).keys),
                       ["PATH", "SMOLCORP_PORT", "SMOLCORP_TEST_SUPPORT_DIR"], "none when this app has none")
    }

    /// FXSWEEP3P1 (RT126SWEEP3 P1's B1): the core is given ZDOTDIR, where zsh's start files are when you moved them, so a
    /// desk's sandbox holds them there; only as a clean whole path, like an agent's folder, else by its name alone.
    func testTheCoreIsGivenWhereZshsStartFilesWereMoved() throws {
        let core = try core("exit 0")
        XCTAssertEqual(GoCore.agentPlaces.last, "ZDOTDIR")
        XCTAssertTrue(GoCore.inherited.contains("ZDOTDIR"))
        let moved = core.environment(base: ["PATH": "/usr/bin", "ZDOTDIR": "/Users/ada/.config/zsh"])
        XCTAssertEqual(moved["ZDOTDIR"], "/Users/ada/.config/zsh")
        XCTAssertNil(moved[GoCore.agentPlacesDroppedVariable])
        XCTAssertNil(core.environment(base: ["PATH": "/usr/bin", "ZDOTDIR": "zsh"])["ZDOTDIR"], "not a whole path: not passed")
        let dotted = core.environment(base: ["PATH": "/usr/bin", "ZDOTDIR": "/Users/ada/x/../zsh"])
        XCTAssertNil(dotted["ZDOTDIR"])
        XCTAssertEqual(dotted[GoCore.agentPlacesDroppedVariable], "ZDOTDIR")
    }

    /// FXAGENTHOMEMACR2 (RT126AGENTHOMEMACR1 B1): a variable that moves an agent's folder reaches the core only as a
    /// whole path. A relative or empty one would be read in the temp folder, where desks write; it isn't passed.
    func testTheCoreIsGivenAnAgentsMovedFolderOnlyAsAWholePath() throws {
        let core = try core("exit 0")
        let environment = core.environment(base: ["PATH": "/usr/bin", "XDG_CONFIG_HOME": "rel", "CODEX_HOME": "",
                                                  "GROK_AUTH_PATH": "./auth.json", "OPENCODE_DB": "~/opencode.db",
                                                  "GEMINI_CLI_HOME": "/Users/ada/moved/gemini"])
        XCTAssertEqual(Set(environment.keys), Set(["PATH", "SMOLCORP_PORT", "SMOLCORP_TEST_SUPPORT_DIR", "GEMINI_CLI_HOME"]))
        XCTAssertEqual(environment["GEMINI_CLI_HOME"], "/Users/ada/moved/gemini")
    }

    /// FXAGENTHOMEMACR3 (RT126AGENTHOMEMACR2 B1): nor one spelled with a `.` or `..` in it, or a doubled `/`: a desk's
    /// sandbox holds it as cleaned, while the Mac takes the `..` after a link a desk could make. Its name alone is
    /// passed, so the core refuses each desk's start in words that name it. A `/` at the end is no such spelling.
    func testTheCoreIsGivenNoAgentsFolderWithADotDotInItButItsName() throws {
        let core = try core("exit 0")
        let environment = core.environment(base: ["PATH": "/usr/bin", "XDG_CONFIG_HOME": "/a/../b", "CODEX_HOME": "/Users/ada/a/./b",
                                                  "OPENCODE_DB": "/Users/ada//x", "GEMINI_CLI_HOME": "/Users/ada/moved/gemini/",
                                                  "GROK_HOME": "rel/../x"])
        XCTAssertEqual(Set(environment.keys), Set(["PATH", "SMOLCORP_PORT", "SMOLCORP_TEST_SUPPORT_DIR", "GEMINI_CLI_HOME",
                                                   GoCore.agentPlacesDroppedVariable]))
        XCTAssertEqual(environment["GEMINI_CLI_HOME"], "/Users/ada/moved/gemini/")
        XCTAssertEqual(environment[GoCore.agentPlacesDroppedVariable], "CODEX_HOME,XDG_CONFIG_HOME,OPENCODE_DB",
                       "the names alone, in the core's order")
        XCTAssertFalse(environment.values.contains { $0.contains("..") || $0.contains("/./") })
        XCTAssertNil(core.environment(base: ["PATH": "/usr/bin", "XDG_CONFIG_HOME": "/Users/ada/cfg"])[GoCore.agentPlacesDroppedVariable])
        // Read by its bytes, as the core reads it (RT126AGENTHOMEMACR3 N1): a combining mark after a `.`, a `..` or a
        // doubled `/` is one Character with it, and the value is dropped all the same. One that only starts a name is kept.
        for spelling in ["/Users/ada/x/../\u{301}b", "/Users/ada/x//\u{301}b", "/Users/ada/x/./\u{301}b"] {
            let dropped = core.environment(base: ["PATH": "/usr/bin", "CODEX_HOME": spelling])
            XCTAssertNil(dropped["CODEX_HOME"], spelling)
            XCTAssertEqual(dropped[GoCore.agentPlacesDroppedVariable], "CODEX_HOME", spelling)
        }
        let kept = core.environment(base: ["PATH": "/usr/bin", "CODEX_HOME": "/Users/ada/x/\u{301}b"])
        XCTAssertEqual(kept["CODEX_HOME"], "/Users/ada/x/\u{301}b")
        XCTAssertNil(kept[GoCore.agentPlacesDroppedVariable])
    }

    /// ENVSCRUB1: a Smolcorp opened inside a Claude Code session tells its core the names of the session's
    /// variables, never their values, and nothing else of them; one opened outside tells it nothing.
    func testTheCoreIsToldOnlyTheNamesOfTheClaudeVariablesItWasOpenedWith() throws {
        let core = try core("exit 0")
        let environment = core.environment(base: ["PATH": "/usr/bin", "CLAUDECODE": "1", "CLAUDE_CODE_ENTRYPOINT": "cli",
                                                  "CLAUDE_CODE_CHILD_SESSION": "fake-child", "CLAUDE_CODE_OAUTH_TOKEN": "fake-oauth",
                                                  "CLAUDE_CONFIG_DIR": "/fake/claude"])
        XCTAssertEqual(environment[GoCore.claudeNamesVariable], "CLAUDECODE,CLAUDE_CODE_CHILD_SESSION,CLAUDE_CODE_ENTRYPOINT,CLAUDE_CODE_OAUTH_TOKEN")
        XCTAssertFalse(environment.values.contains { $0.contains("fake-") }, "a name only, never a value")
        XCTAssertNil(environment["CLAUDECODE"])
        XCTAssertNil(environment["CLAUDE_CONFIG_DIR"])
        XCTAssertNil(core.environment(base: ["PATH": "/usr/bin", "CLAUDE_CONFIG_DIR": "/fake/claude"])[GoCore.claudeNamesVariable])
    }

    /// The core listens on this Mac only in a test copy (desktop/listen.go), unless the copy was
    /// opened with SMOLCORP_TEST_LISTEN=lan, which it's given then, and only then.
    func testTheCoreIsLetOntoTheLANOnlyWhenTheCopyWas() throws {
        let core = try core("exit 0")
        XCTAssertEqual(core.environment(base: ["PATH": "/usr/bin", "SMOLCORP_TEST_LISTEN": "lan"])["SMOLCORP_TEST_LISTEN"], "lan")
        XCTAssertEqual(core.environment(base: ["PATH": "/usr/bin", "SMOLCORP_TEST_LISTEN": "LAN"])["SMOLCORP_TEST_LISTEN"], "lan")
        // " lan" is lan for the core, as for the Swift engine (RTCH N3).
        XCTAssertEqual(core.environment(base: ["PATH": "/usr/bin", "SMOLCORP_TEST_LISTEN": " Lan "])["SMOLCORP_TEST_LISTEN"], "lan")
        XCTAssertNil(core.environment(base: ["PATH": "/usr/bin", "SMOLCORP_TEST_LISTEN": "all"])["SMOLCORP_TEST_LISTEN"])
        XCTAssertNil(core.environment(base: ["PATH": "/usr/bin"])["SMOLCORP_TEST_LISTEN"])
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
        XCTAssertNil(core.ask(.office), "nothing to ask once stopped")
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

    /// FXUPGRADESTART126: the update to 1.26's first start asks the Keychain through the bridge before the core says
    /// it's ready (record-key-new, the items sealed as found), and a prompt holds that request until it's answered.
    /// A core slower than its wait while its bridge answers a Keychain request still starts; without one, it doesn't.
    func testACoreSlowerThanItsWaitWhileTheKeychainIsAskedStillStarts() throws {
        XCTAssertEqual(GoCore.keychainWaitMost, 600, "past the core's own 5 minutes for a Keychain request (bridgeKeychainWait)")
        let slow = "read token\nsleep 2\necho '{\"ready\":true}'\nwhile read line; do :; done"
        let unasked = try core(slow)
        XCTAssertThrowsError(try unasked.start(waitingUpTo: 1)) { error in
            XCTAssertEqual(error.localizedDescription, "The Go engine didn't get ready: no answer in 1 s")
        }

        let core = try core(slow)
        // The core's record-key-new at the update, held 2.5 s as a prompt would hold it, answered as the bridge answers
        // the core's own request (`HostBridge.serve`), on its clock.
        var held = HostBridge.Actions.mac
        held.records = HeldRecordKeys(wait: 2.5)
        let request = try JSONSerialization.data(withJSONObject: ["token": core.bridge.token, "action": "record-key-new", "item": "update"])
        let answered = expectation(description: "the Keychain's answer")
        DispatchQueue.global().async {
            let answer = HostBridge.answer(request, token: core.bridge.token, actions: held, keychain: core.bridge.keychain)
            XCTAssertEqual(answer["ok"] as? Bool, true)
            answered.fulfill()
        }
        while core.bridge.keychain.seconds == 0 { Thread.sleep(forTimeInterval: 0.01) }
        let started = Date()
        try core.start(waitingUpTo: 1)
        XCTAssertGreaterThan(Date().timeIntervalSince(started), 1.5, "it was ready past its wait")
        XCTAssertNotNil(core.pid)
        wait(for: [answered], timeout: 5)
        XCTAssertTrue(core.stop(grace: 1))
    }

    /// FXUPGRADESTART126: a core that didn't get ready is started once more before the menu says so, as quitting and
    /// opening Smolcorp again did by hand; a dead one is tried twice, then said. A build without the engine, once.
    func testACoreThatDidntGetReadyIsStartedOnceMore() throws {
        XCTAssertEqual(GoShellDelegate.startTries, 2)
        let tries = folder.appendingPathComponent("tries"), marker = folder.appendingPathComponent("tried")
        func count() -> Int { ((try? String(contentsOf: tries)) ?? "").split(separator: "\n").count }
        var said: [String] = []

        let once = try core("""
            echo try >> '\(tries.path)'
            if [ ! -e '\(marker.path)' ]; then touch '\(marker.path)'; exit 3; fi
            \(answering)
            """)
        XCTAssertNil(GoShellDelegate.start(once, waitingUpTo: 5, waiting: { said.append($0) }), "the second start serves")
        XCTAssertEqual(count(), 2)
        XCTAssertEqual(once.setupSession()?.session, "fake-session", "it answers")
        XCTAssertTrue(once.stop(grace: 2))

        try? FileManager.default.removeItem(at: tries)
        let dead = try core("echo try >> '\(tries.path)'\nexit 3")
        XCTAssertEqual(GoShellDelegate.start(dead, waitingUpTo: 5, waiting: { said.append($0) }),
                       "The Go engine didn't get ready: it ended (3)")
        XCTAssertEqual(count(), 2, "tried once more, then said")

        try? FileManager.default.removeItem(at: tries)
        let silent = try core("echo try >> '\(tries.path)'\nsleep 30")
        XCTAssertEqual(GoShellDelegate.start(silent, waitingUpTo: 0.5, waiting: { said.append($0) }),
                       "The Go engine didn't get ready: no answer in 0 s")
        XCTAssertEqual(count(), 2)
        XCTAssertTrue(silent.pid.map(GoCore.isGone) ?? true, "and stopped")

        let missing = GoCore(program: folder.appendingPathComponent("none"), port: 7779, support: folder,
                             bridge: HostBridge(socket: folder.appendingPathComponent("bridge.sock")))
        XCTAssertEqual(GoShellDelegate.start(missing, waiting: { said.append($0) }), GoCore.Failure.noProgram.localizedDescription)
        XCTAssertEqual(said, [], "none of these is a taken port")

        // A taken port is said and waited out as before, and isn't one of the tries.
        try? FileManager.default.removeItem(at: tries)
        try? FileManager.default.removeItem(at: marker)
        let second = folder.appendingPathComponent("tried-twice")
        let taken = try core("""
            echo try >> '\(tries.path)'
            if [ ! -e '\(marker.path)' ]; then touch '\(marker.path)'; exit \(GoCore.portInUseStatus); fi
            if [ ! -e '\(second.path)' ]; then touch '\(second.path)'; exit \(GoCore.portInUseStatus); fi
            \(answering)
            """, port: 7710)
        XCTAssertNil(GoShellDelegate.start(taken, waitingUpTo: 5, waiting: { said.append($0) }, retryDelay: 0))
        XCTAssertEqual(said, Array(repeating: GoShellDelegate.portInUseLine(port: 7710), count: 2))
        XCTAssertEqual(count(), 3)
        XCTAssertTrue(taken.stop(grace: 2))
    }

    /// A core whose port is taken ends with EADDRINUSE's status: the shell says so and tries again (M7WORDS).
    func testACoreWhosePortIsTakenIsSaidSo() throws {
        XCTAssertEqual(GoCore.portInUseStatus, EADDRINUSE, "as the core ends (desktop/core_darwin.go)")
        XCTAssertThrowsError(try core("exit \(GoCore.portInUseStatus)", port: 7710).start(waitingUpTo: 5)) { error in
            guard case GoCore.Failure.portInUse(let port) = error else { return XCTFail("not the port: \(error)") }
            XCTAssertEqual(port, 7710)
            XCTAssertEqual(error.localizedDescription, "Port 7710 is in use. Is Smolcorp already running?")
        }
        // Only that status: another end is the engine's own trouble, said as it was.
        for status in [1, 2, 3] {
            XCTAssertThrowsError(try core("exit \(status)").start(waitingUpTo: 5)) { error in
                guard case GoCore.Failure.notReady = error else { return XCTFail("status \(status): \(error)") }
            }
        }
        XCTAssertEqual(GoShellDelegate.portInUseLine(port: 7710), "Port 7710 is in use. Is Smolcorp already running? Trying again…",
                       "the Swift app's words, as StatusMenu.showPortInUse had them")
        XCTAssertEqual(GoShellDelegate.retryDelay, 2)
    }

    /// The same core is started again once the port is free, and the first try's end isn't "the engine stopped".
    func testACoreStartedAgainAfterATakenPortServes() throws {
        let marker = folder.appendingPathComponent("tried")
        let core = try core("""
            if [ ! -e '\(marker.path)' ]; then touch '\(marker.path)'; exit \(GoCore.portInUseStatus); fi
            \(answering)
            """)
        let stoppedByItself = expectation(description: "onExit")
        stoppedByItself.isInverted = true
        core.onExit = { _ in stoppedByItself.fulfill() }
        XCTAssertThrowsError(try core.start(waitingUpTo: 5)) { error in
            guard case GoCore.Failure.portInUse = error else { return XCTFail("\(error)") }
        }
        try core.start(waitingUpTo: 5)
        let pid = try XCTUnwrap(core.pid)
        XCTAssertEqual(core.bridge.core, pid)
        XCTAssertEqual(core.setupSession()?.session, "fake-session", "it answers")
        wait(for: [stoppedByItself], timeout: 0.5)
        XCTAssertTrue(core.stop(grace: 2))
    }

    func testAnUnansweredQuestionIsNil() throws {
        let core = try core("echo '{\"ready\":true}'\nwhile read line; do echo '{\"id\":99}'; done")
        try core.start(waitingUpTo: 5)
        XCTAssertNil(core.ask(.office, waitingUpTo: 0.3), "an answer to another question isn't this one's")
        core.stop(grace: 1)
    }

    /// The menu's state, sent unasked between answers, goes to `onMenu` on the main queue, never taken for an
    /// answer; asked, it's the answer's. A question the menu asks as it opens waits only a moment.
    func testTheMenusStateComesUnaskedAndAsked() throws {
        let core = try core(answering)
        var told: [ShellMenu] = []
        let heard = expectation(description: "the menu's state, unasked")
        heard.assertForOverFulfill = false
        core.onMenu = {
            XCTAssertTrue(Thread.isMainThread)
            told.append($0)
            heard.fulfill()
        }
        try core.start(waitingUpTo: 5)
        let asked = try XCTUnwrap(core.menu(waitingUpTo: 2), "the answer, not the line before it")
        XCTAssertEqual(asked.phones, 2)
        XCTAssertEqual(core.ask(.setupLink, then: "pair")?["link"] as? String, "http://localhost:7779/setup#o=fake")
        wait(for: [heard], timeout: 5)
        XCTAssertEqual(told.first?.waiting, 1)
        XCTAssertEqual(told.first?.portable.on, true)
        XCTAssertEqual(told.first?.phones, 0)
        core.stop(grace: 1)
    }

    /// RMENU N3: a core that is alive but stuck leaves the menu's question unread. The menu, opening on the main
    /// queue, writes it once, not at each opening, so the pipe never fills; any word from the core lets it ask again.
    func testTheMenuDoesntAskAgainWhileItsQuestionIsUnanswered() throws {
        let seen = folder.appendingPathComponent("menus")
        let core = try core("""
            read token
            echo '{"ready":true}'
            while read line; do
              case "$line" in
                *'"op":"menu"'*) echo x >> '\(seen.path)';;
                *) echo '{"menu":{"sessions":1}}';;
              esac
            done
            """)
        try core.start(waitingUpTo: 5)
        XCTAssertNil(core.menu(waitingUpTo: 0.2), "no answer in time")
        XCTAssertTrue(core.isMenuUnanswered)
        let begun = Date()
        for _ in 0..<500 { XCTAssertNil(core.menu(waitingUpTo: 0.2)) }
        XCTAssertLessThan(Date().timeIntervalSince(begun), 2, "not even waited for")
        // Something from the core (a push, here after another question): the menu may ask once more.
        XCTAssertNil(core.ask(.office, waitingUpTo: 0.3), "the push isn't the answer")
        XCTAssertFalse(core.isMenuUnanswered)
        XCTAssertNil(core.menu(waitingUpTo: 0.2))
        core.stop(grace: 1)
        XCTAssertEqual(try String(contentsOf: seen).split(separator: "\n").count, 2, "two questions written, not 502")
    }

    /// The menu waits only a moment for an earlier question (the lock `ask` keeps) too, and then asks nothing.
    func testTheMenuGivesUpOnAnEarlierQuestionAtOnce() throws {
        let core = try core("read token\necho '{\"ready\":true}'\nwhile read line; do :; done")
        try core.start(waitingUpTo: 5)
        let slow = expectation(description: "the earlier question")
        DispatchQueue.global().async { _ = core.ask(.office, waitingUpTo: 2); slow.fulfill() }
        Thread.sleep(forTimeInterval: 0.3)
        let begun = Date()
        XCTAssertNil(core.menu(waitingUpTo: 0.3))
        XCTAssertLessThan(Date().timeIntervalSince(begun), 1)
        XCTAssertFalse(core.isMenuUnanswered, "nothing was written")
        wait(for: [slow], timeout: 5)
        core.stop(grace: 1)
    }

    func testTheSettingsWindowUsesTheEnginesCookie() throws {
        let cookie = try XCTUnwrap(SettingsWindow.sessionCookie(session: "abc", name: "smolcorp-setup"))
        XCTAssertEqual(cookie.name, "smolcorp-setup")
        XCTAssertEqual(SettingsWindow.sessionCookie(session: "abc")?.name, SettingsWindow.setupCookie, "this port's, as the core names it")
        XCTAssertEqual(SettingsWindow.setupCookie, Deck.port == 7710 ? "smolcorp-setup" : "smolcorp-setup-\(Deck.port)")
    }
}
