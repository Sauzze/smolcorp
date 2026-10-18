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

    func testThePaneIsAskedAbout() {
        XCTAssertEqual(Tmux().isOpen(pane), true)
        XCTAssertEqual(Tmux().isOpen(TabAddress("tmux:%9@/tmp/tmux-501/default")!), false)
        XCTAssertEqual(Tmux().tty(of: pane), "/dev/ttys042")
    }

    func testFiringKillsOnlyAPaneThatsThere() {
        XCTAssertEqual(Tmux().fire(pane, agent: .claude), .fired)
        XCTAssertTrue(asked.contains("-S /tmp/tmux-501/default kill-pane -t %3"))
        XCTAssertEqual(Tmux().fire(TabAddress("tmux:%9@/tmp/tmux-501/default")!, agent: .claude), .gone)
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
