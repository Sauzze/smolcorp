import XCTest
@testable import AgentDeck

/// tmux through a stand-in that records what it's asked, as the Go tests do.
final class TmuxTests: XCTestCase {
    private var folder: URL!
    private var saved: String?
    private var log: URL { folder.appendingPathComponent("asked.txt") }

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let tmux = folder.appendingPathComponent("tmux")
        try """
            #!/bin/sh
            echo "$*" >> "\(log.path)"
            case "$*" in
              *pane_id*) case "$*" in *%9*) exit 1 ;; *) echo %3 ;; esac ;;
              *pane_tty*) echo /dev/ttys042 ;;
              *list-clients*) echo /dev/ttys099 ;;
            esac
            """.write(to: tmux, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: tmux.path)
        saved = Tmux.path
        Tmux.path = tmux.path
    }

    override func tearDownWithError() throws {
        Tmux.path = saved
        try? FileManager.default.removeItem(at: folder)
    }

    private var asked: [String] {
        ((try? String(contentsOf: log, encoding: .utf8)) ?? "").split(separator: "\n").map(String.init)
    }

    private let pane = TabAddress("tmux:%3@/tmp/tmux-501/default")!

    func testEachKeyByTmuxsName() {
        XCTAssertTrue(Tmux().press(.init(key: .returnKey), in: pane))
        XCTAssertTrue(Tmux().press(.init(key: .tab, modifiers: .maskShift), in: pane))
        XCTAssertTrue(Tmux().press(.init(key: .c, modifiers: .maskControl), in: pane))
        XCTAssertTrue(Tmux().press(.init(key: .escape, times: 2), in: pane))
        XCTAssertEqual(asked, [
            "-S /tmp/tmux-501/default send-keys -t %3 Enter",
            "-S /tmp/tmux-501/default send-keys -t %3 BTab",
            "-S /tmp/tmux-501/default send-keys -t %3 C-c",
            "-S /tmp/tmux-501/default send-keys -t %3 Escape",
            "-S /tmp/tmux-501/default send-keys -t %3 Escape",
        ])
    }

    func testTextIsLiteralThenReturn() {
        XCTAssertTrue(Tmux().type("-n run the tests;", in: pane))
        XCTAssertEqual(asked, [
            #"-S /tmp/tmux-501/default send-keys -t %3 -l -- -n run the tests\;"#,
            "-S /tmp/tmux-501/default send-keys -t %3 Enter",
        ])
    }

    /// Dictation's text: a bracketed paste, without Return.
    func testDictatedTextIsPastedWithoutReturn() {
        XCTAssertTrue(Tmux().insert("ajoute un test; puis lance-le", in: pane))
        XCTAssertEqual(asked.count, 2)
        XCTAssertTrue(asked[0].hasPrefix("-S /tmp/tmux-501/default set-buffer -b smolcorp-") && asked[0].hasSuffix(" -- ajoute un test; puis lance-le"))
        XCTAssertTrue(asked[1].contains("paste-buffer -p -d -b smolcorp-") && asked[1].hasSuffix("-t %3"))
        XCTAssertFalse(asked.contains { $0.hasSuffix(" Enter") })
    }

    func testThePaneIsAskedAbout() {
        XCTAssertEqual(Tmux().isOpen(pane), true)
        XCTAssertEqual(Tmux().isOpen(TabAddress("tmux:%9@/tmp/tmux-501/default")!), false)
        XCTAssertEqual(Tmux().tty(of: pane), "/dev/ttys042")
    }

    func testFiringAPaneThatsGoneLeavesIt() {
        XCTAssertEqual(Tmux().fire(TabAddress("tmux:%9@/tmp/tmux-501/default")!, agent: .claude), .gone)
        XCTAssertFalse(asked.contains { $0.contains("kill-pane") })
    }

    func testFiringAPaneThatCantBeLookedAtLeavesIt() {
        // The stand-in tmux can't say what runs in the pane: unknown, so nothing is killed.
        XCTAssertEqual(Tmux().fire(pane, agent: .claude), .notStopped)
        XCTAssertFalse(asked.contains { $0.contains("kill-pane") })
    }

    func testScrollingUsesCopyMode() {
        XCTAssertTrue(Tmux().scroll(lines: -5, in: pane))
        XCTAssertEqual(asked.suffix(2), [
            "-S /tmp/tmux-501/default copy-mode -e -t %3",
            "-S /tmp/tmux-501/default send-keys -t %3 -X -N 5 scroll-up",
        ])
    }

    func testAKeyTmuxHasNoNameForIsntSent() {
        XCTAssertFalse(Tmux().press(.init(key: .pageUp), in: pane))
        XCTAssertTrue(asked.isEmpty)
    }

    func testATmuxThatHangsIsStopped() throws {
        let hung = folder.appendingPathComponent("tmux-hung")
        try "#!/bin/sh\nexec sleep 60\n".write(to: hung, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: hung.path)
        Tmux.path = hung.path
        let started = Date()
        XCTAssertNil(Tmux().isOpen(pane), "unknown, not closed")
        XCTAssertLessThan(Date().timeIntervalSince(started), Tool.timeout + 2)
    }

    func testKeysAsBytesForTerminalsThatTakeThem() {
        XCTAssertEqual(Terminal.Keystroke(key: .up).bytes, "\u{1b}[A")
        XCTAssertEqual(Terminal.Keystroke(key: .tab, modifiers: .maskShift).bytes, "\u{1b}[Z")
        XCTAssertEqual(Terminal.Keystroke(key: .space).bytes, " ")
        XCTAssertNil(Terminal.Keystroke(key: .pageDown).bytes)
    }
}

/// Firing in real tmux panes, on a server of the test's own: only the agent
/// goes, and someone's work in a pane is left alone.
final class TmuxFiringTests: XCTestCase {
    private var folder: URL!
    private var socket: String { folder.appendingPathComponent("s").path }
    private var tmux: String!

    override func setUpWithError() throws {
        tmux = try XCTUnwrap(Tool.find(["/opt/homebrew/bin/tmux", "/usr/local/bin/tmux"]), "needs tmux")
        Tmux.path = tmux
        // tmux's sockets live in a short path.
        folder = URL(fileURLWithPath: "/tmp/smolcorp-test-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        // An agent as `ps` names it: a program called node that waits. A copy
        // of /bin/sleep won't do: macOS stops copies of its own programs.
        let source = folder.appendingPathComponent("node.c")
        try "#include <unistd.h>\nint main(void) { for (;;) pause(); }\n".write(to: source, atomically: true, encoding: .utf8)
        let built = try XCTUnwrap(Tool.run("/usr/bin/cc", ["-o", folder.appendingPathComponent("node").path, source.path]))
        XCTAssertEqual(built.status, 0)
    }

    override func tearDownWithError() throws {
        _ = Tool.run(tmux, ["-S", socket, "kill-server"])
        try? FileManager.default.removeItem(at: folder)
    }

    /// A pane started with `command`, and `typed` typed into it once its shell is up.
    private func pane(_ command: String, typed: String? = nil) throws -> TabAddress {
        let made = try XCTUnwrap(Tool.run(tmux, ["-S", socket, "new-session", "-d", "-P", "-F", "#{pane_id}", "-x", "80", "-y", "24", command]))
        let address = try XCTUnwrap(TabAddress("tmux:\(made.output)@\(socket)"))
        if let typed {
            Thread.sleep(forTimeInterval: 0.5)
            _ = Tool.run(tmux, ["-S", socket, "send-keys", "-t", address.id, typed, "Enter"])
        }
        Thread.sleep(forTimeInterval: 0.8)
        return address
    }

    private func isThere(_ pane: TabAddress) -> Bool { Tmux().isOpen(pane) == true }

    /// Whether the stand-in runs in the pane: the agent is really there before it's fired.
    private func agentRuns(in pane: TabAddress) -> Bool {
        Tmux().tty(of: pane).flatMap { Terminal.isRunning("node", on: $0) } == true
    }

    /// The stand-in as Claude Code from npm: node running the agent's own package.
    private var claude: String { "\(folder.path)/node /opt/homebrew/lib/node_modules/@anthropic-ai/claude-code/cli.js" }
    /// The stand-in as a dev server: node too, running vite.
    private var devServer: String { "\(folder.path)/node /Users/me/app/node_modules/.bin/vite" }

    func testTheAgentAsThePanesOwnProgramGoesWithThePane() throws {
        let agent = try pane(claude)
        XCTAssertTrue(agentRuns(in: agent))
        XCTAssertEqual(Tmux().fire(agent, agent: .claude), .fired)
        XCTAssertFalse(isThere(agent))
    }

    func testTheAgentAsAJobIsStoppedThenThePaneClosed() throws {
        let agent = try pane("/bin/sh", typed: claude)
        XCTAssertTrue(agentRuns(in: agent))
        XCTAssertEqual(Tmux().fire(agent, agent: .claude), .fired)
        XCTAssertFalse(isThere(agent))
    }

    func testAShellAloneIsClosed() throws {
        let shell = try pane("/bin/sh")
        XCTAssertEqual(Tmux().fire(shell, agent: .claude), .fired)
        XCTAssertFalse(isThere(shell))
    }

    func testSomeonesWorkInThePaneIsLeftAlone() throws {
        // The agent crashed, and a dev server, here sleep, was started in its pane.
        let work = try pane("/bin/sh", typed: "/bin/sleep 300")
        XCTAssertEqual(Tmux().fire(work, agent: .claude), .notStopped)
        XCTAssertTrue(isThere(work))
        let program = try pane("/bin/sleep 300")
        XCTAssertEqual(Tmux().fire(program, agent: .claude), .notStopped, "a pane started with someone else's program")
        XCTAssertTrue(isThere(program))
    }

    /// The agent crashed and `npm run dev` took its place: node, but not the agent.
    func testANodeDevServerInTheAgentsPaneIsLeftAlone() throws {
        let job = try pane("/bin/sh", typed: devServer)
        XCTAssertTrue(agentRuns(in: job))
        XCTAssertEqual(Tmux().fire(job, agent: .claude), .notStopped)
        XCTAssertTrue(isThere(job))
        XCTAssertEqual(Tmux().tty(of: job).flatMap { Terminal.isRunning(.claude, on: $0) }, false, "the desk is free to go")
        let program = try pane(devServer)
        XCTAssertEqual(Tmux().fire(program, agent: .claude), .notStopped, "a pane started with the dev server")
        XCTAssertTrue(isThere(program))
    }

    func testTheAgentIsSeenRunning() throws {
        let agent = try pane(claude)
        XCTAssertEqual(Tmux().tty(of: agent).flatMap { Terminal.isRunning(.claude, on: $0) }, true)
    }
}
