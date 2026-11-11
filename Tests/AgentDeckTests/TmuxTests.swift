import XCTest
@testable import AgentDeck

/// tmux through a stand-in that records what it's asked, as the Go tests do.
final class TmuxTests: XCTestCase {
    private var folder: URL!
    private var saved: String?
    private var log: URL { folder.appendingPathComponent("asked.txt") }
    private var stdin: URL { folder.appendingPathComponent("stdin.bin") }

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let tmux = folder.appendingPathComponent("tmux")
        try """
            #!/bin/sh
            echo "$*" >> "\(log.path)"
            case "$*" in *load-buffer*) cat > "\(stdin.path)" ;; esac
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

    /// The phone mic's peek reads the visible pane only: no `-S`, which
    /// would reach into the history, and no `-e` escapes.
    func testTheScreenIsTheVisiblePaneOnly() {
        XCTAssertNotNil(Tmux().screen(of: pane))
        XCTAssertEqual(asked, ["-S /tmp/tmux-501/default capture-pane -p -t %3"])
    }

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

    /// Dictation's text: a bracketed paste, without Return, the text on tmux's standard input.
    func testDictatedTextIsPastedWithoutReturn() throws {
        XCTAssertTrue(Tmux().insert("ajoute un test; puis lance-le", in: pane))
        XCTAssertEqual(asked.count, 2)
        XCTAssertTrue(asked[0].hasPrefix("-S /tmp/tmux-501/default load-buffer -b smolcorp-") && asked[0].hasSuffix(" -"), asked[0])
        XCTAssertTrue(asked[1].contains("paste-buffer -p -d -b smolcorp-") && asked[1].hasSuffix("-t %3"))
        XCTAssertEqual(asked[0].split(separator: " ")[4], asked[1].split(separator: " ")[6], "the same buffer")
        XCTAssertFalse(asked.contains { $0.hasSuffix(" Enter") })
        XCTAssertEqual(try Data(contentsOf: stdin), Data("ajoute un test; puis lance-le".utf8))
    }

    /// As an argument, macOS takes accents apart (é as e and a combining accent) and tmux drops a ";"
    /// at the end: the phone's text goes on tmux's standard input, byte for byte, never as an argument.
    func testAccentsAndASemicolonArriveByteForByte() throws {
        let text = "Café à l’été, ça va ? « œ » 👩‍💻 fin;"
        XCTAssertTrue(Tmux().insert(text, in: pane))
        XCTAssertEqual(try Data(contentsOf: stdin), Data(text.utf8))
        XCTAssertFalse(asked.joined().contains("Caf"), "never an argument")
        try FileManager.default.removeItem(at: stdin)
        XCTAssertTrue(Tmux().typeKeys("oui, ça marche;", in: pane))
        XCTAssertEqual(try Data(contentsOf: stdin), Data("oui, ça marche;".utf8))
        XCTAssertTrue(asked.last!.contains("paste-buffer -r -d -b smolcorp-"), "as keys: no brackets, a line feed as itself")
        // Text that could press Enter or end a bracket never reaches tmux.
        let before = asked.count
        XCTAssertFalse(Tmux().typeKeys("a\u{1B}[201~\rb", in: pane))
        XCTAssertEqual(asked.count, before)
    }

    /// tmux reads an argument ending in ";" as the end of its command and
    /// drops that ";" (a lone one leaves nothing): the phone's "echo hi;"
    /// reached the agent as "echo hi". Text given as an argument keeps it.
    func testASemicolonAtTheEndIsKept() {
        XCTAssertEqual(Tmux.literal("fin de phrase;"), #"fin de phrase\;"#)
        XCTAssertEqual(Tmux.literal(";"), #"\;"#)
        XCTAssertEqual(Tmux.literal(#"a\;"#), #"a\\;"#, "tmux takes one backslash away, so the text's own stays")
        XCTAssertEqual(Tmux.literal("a; b && c"), "a; b && c", "only at the end")
        XCTAssertEqual(Tmux.literal(#"end\"#), #"end\"#)
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
        let found = Tool.find(["/opt/homebrew/bin/tmux", "/usr/local/bin/tmux"])
        // GitHub's Mac runners have no tmux (CI is "true" there). Anywhere else, a missing tmux is a failure.
        if found == nil, ProcessInfo.processInfo.environment["CI"] == "true" {
            throw XCTSkip("needs real tmux panes, and this CI runner has no tmux")
        }
        tmux = try XCTUnwrap(found, "needs tmux")
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
        // Also run after a setUp that stopped early: without tmux, or before the folder was made.
        guard let tmux, let folder else { return }
        _ = Tool.run(tmux, ["-S", socket, "kill-server"])
        try? FileManager.default.removeItem(at: folder)
    }

    /// A pane started with `command`, and `typed` typed into it once its shell is up.
    /// It's handed back once its program runs: on a busy Mac that can take a while.
    private func pane(_ command: String, typed: String? = nil) throws -> TabAddress {
        let made = try XCTUnwrap(Tool.run(tmux, ["-S", socket, "new-session", "-d", "-P", "-F", "#{pane_id}", "-x", "80", "-y", "24", command]))
        let address = try XCTUnwrap(TabAddress("tmux:\(made.output)@\(socket)"))
        let tty = try XCTUnwrap(Tmux().tty(of: address))
        let shell = command == "/bin/sh"
        if shell {
            // Its prompt is shown: the shell reads what's typed.
            XCTAssertTrue(eventually { !(Tool.run(tmux, ["-S", socket, "capture-pane", "-p", "-t", address.id])?.output ?? "")
                .trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }, "the shell's prompt")
        }
        if let typed {
            _ = Tool.run(tmux, ["-S", socket, "send-keys", "-t", address.id, typed, "Enter"])
        }
        // The program typed, or the pane's own, runs on its tty.
        if let program = (typed ?? (shell ? nil : command))?.split(separator: " ").first.map({ ($0 as NSString).lastPathComponent }) {
            XCTAssertTrue(eventually { Terminal.isRunning(program, on: tty) == true }, "\(program) runs in the pane")
        }
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

    private func place(in pane: TabAddress) -> Terminal.Place? {
        Tmux().tty(of: pane).flatMap { Terminal.place(of: .claude, on: $0) }
    }

    func testTheAgentStoppedWithCtrlZIsBehindAndKeepsItsPane() throws {
        let agent = try pane(claude)
        XCTAssertEqual(place(in: agent), .front, "the pane's own program")
        let job = try pane("/bin/sh", typed: claude)
        XCTAssertEqual(place(in: job), .front, "a job in front")
        _ = Tool.run(tmux, ["-S", socket, "send-keys", "-t", job.id, "C-z"])
        XCTAssertTrue(eventually { place(in: job) == .behind }, "its text would run in the shell")
        XCTAssertEqual(Tmux().fire(job, agent: .claude), .notStopped, "closing the pane would hang it up")
        XCTAssertTrue(isThere(job))
    }

    func testAShellWithAJobInTheBackgroundIsLeftOpen() throws {
        let shell = try pane("/bin/sh", typed: "/bin/sleep 300 &")
        XCTAssertEqual(Tmux().fire(shell, agent: .claude), .notStopped)
        XCTAssertTrue(isThere(shell))
    }

    /// A hire's script lives in Application Support: its path has a space.
    func testAHireInTmuxStartsFromAPathWithASpace() throws {
        let place = folder.appendingPathComponent("Application Support", isDirectory: true)
        try FileManager.default.createDirectory(at: place, withIntermediateDirectories: true)
        let script = place.appendingPathComponent("hire.sh"), started = folder.appendingPathComponent("started")
        try "#!/bin/sh\ntouch '\(started.path)'\n".write(to: script, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
        let made = try XCTUnwrap(Tool.run(tmux, ["-S", socket] + Openers.session("hire", in: folder.path, shell: "/bin/sh", running: script.path)))
        XCTAssertEqual(made.status, 0)
        XCTAssertTrue(eventually { FileManager.default.fileExists(atPath: started.path) }, "the hire's script ran")
    }
}
