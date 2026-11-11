import XCTest
@testable import AgentDeck

/// A desk's keys and text never wait behind Terminal.app (I2): each terminal
/// has its own queue, Terminal's looks run on pollers of their own with a
/// time limit, and an input its terminal doesn't get to in time is dropped
/// and said, never typed late. All with stand-ins: no terminal is ever asked
/// anything for real.
final class TerminalQueueTests: XCTestCase {
    private var folder: URL!
    private var savedOsascript = ""
    private var savedTmux: String?
    private var tmuxLog: URL { folder.appendingPathComponent("tmux-asked.txt") }
    private var osascriptPids: URL { folder.appendingPathComponent("osascript-pids.txt") }
    private var tab: String { "tmux:%3@\(folder.appendingPathComponent("s").path)" }

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("i2-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        // Terminal.app not answering, as while macOS asks whether Smolcorp may control it: osascript hangs.
        let osascript = folder.appendingPathComponent("osascript")
        try "#!/bin/sh\necho $$ >> '\(osascriptPids.path)'\nexec sleep 60\n".write(to: osascript, atomically: true, encoding: .utf8)
        // tmux answering at once, and saying what it was asked, text loaded from its standard input
        // included; no pane tty, so nothing is looked at with ps.
        // Its session is shown in a Terminal.app tab (a client on a tty), as hires in tmux are.
        let tmux = folder.appendingPathComponent("tmux")
        try """
            #!/bin/sh
            echo "$*" >> '\(tmuxLog.path)'
            case "$*" in
                *pane_tty*) exit 1 ;;
                *session_id*) echo '$1' ;;
                *client_tty*) echo /dev/ttys998 ;;
                *load-buffer*) { printf 'stdin: '; cat; echo; } >> '\(tmuxLog.path)' ;;
            esac
            """.write(to: tmux, atomically: true, encoding: .utf8)
        for file in [osascript, tmux] { try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: file.path) }
        savedOsascript = Terminal.osascript
        savedTmux = Tmux.path
        Terminal.osascript = osascript.path
        Tmux.path = tmux.path
    }

    override func tearDownWithError() throws {
        // What still runs on Terminal's queue, and iTerm2's after it, finishes against the stand-in, then the real ones are back.
        Terminal.queue.sync {}
        Terminals.queue(kind: "iterm").sync {}
        Terminal.osascript = savedOsascript
        Tmux.path = savedTmux
        try? FileManager.default.removeItem(at: folder)
    }

    private var tmuxAsked: String { (try? String(contentsOf: tmuxLog, encoding: .utf8)) ?? "" }

    private var osascripts: [pid_t] {
        ((try? String(contentsOf: osascriptPids, encoding: .utf8)) ?? "").split(separator: "\n").compactMap { pid_t($0) }
    }

    /// The stand-in osascripts once `count` have started, or past `limit` however many have: a
    /// script's first run waits a moment while macOS checks it, more on a busy Mac.
    private func osascripts(started count: Int, within limit: TimeInterval = 3) -> [pid_t] {
        let deadline = Date().addingTimeInterval(limit)
        while osascripts.count < count, Date() < deadline { Thread.sleep(forTimeInterval: 0.02) }
        return osascripts
    }

    private func isGone(_ pid: pid_t) -> Bool { kill(pid, 0) == -1 && errno == ESRCH }

    private func text(_ response: Response) -> String { String(decoding: response.body, as: UTF8.self) }

    /// A tmux desk's tap brings its Terminal tab forward on Terminal's queue;
    /// one Terminal gets to only after the input wait is skipped, so a
    /// Terminal answering again never comes forward seconds after the taps.
    func testALateTerminalFocusIsSkipped() throws {
        let address = try XCTUnwrap(TabAddress(tab))
        let release = DispatchSemaphore(value: 0)
        Terminal.queue.async { release.wait() } // Terminal busy, as while it hangs
        XCTAssertTrue(Tmux().focus(address))
        Thread.sleep(forTimeInterval: Terminals.inputWait + 0.3)
        release.signal()
        Terminal.queue.sync {}
        Terminals.queue(kind: "iterm").sync {}
        XCTAssertTrue(tmuxAsked.contains("select-pane"), tmuxAsked)
        XCTAssertEqual(osascripts, [], "Terminal was never asked to bring the tab forward")
    }

    /// Terminal's look skips a round while the last is stuck, so hung osascripts never pile up.
    func testThePollSkipsARoundWhileTheLastIsStuck() {
        let poll = Poller("test-stuck")
        let first = expectation(description: "first round")
        let started = Date()
        XCTAssertTrue(poll.round({ Terminal.openTTYs() }) { XCTAssertNil($0); first.fulfill() })
        Thread.sleep(forTimeInterval: 0.2)
        XCTAssertFalse(poll.round({ Terminal.openTTYs() }) { _ in XCTFail("a second round ran") }, "skipped while the first is stuck")
        wait(for: [first], timeout: Terminal.pollTimeout + 2)
        XCTAssertGreaterThan(Date().timeIntervalSince(started), Terminal.pollTimeout - 0.5, "it was stuck until its time limit")
        XCTAssertEqual(osascripts.count, 1, "one osascript, not one a round")
        XCTAssertTrue(osascripts.allSatisfy(isGone), "ended by its pid")
        // Once it has ended, the next round runs.
        let next = expectation(description: "next round")
        XCTAssertTrue(poll.round({ 1 }) { XCTAssertEqual($0, 1); next.fulfill() })
        wait(for: [next], timeout: 1)
    }

    /// Each terminal its own queue; Terminal.app keeps `Terminal.queue`, where windows open too.
    func testEachTerminalHasItsOwnQueue() {
        XCTAssertTrue(Terminals.queue(for: "tty:/dev/ttys003") === Terminal.queue)
        XCTAssertTrue(Terminals.queue(for: nil) === Terminal.queue)
        XCTAssertFalse(Terminals.queue(for: tab) === Terminal.queue)
        XCTAssertTrue(Terminals.queue(for: tab) === Terminals.queue(for: "tmux:%9@/tmp/other"), "one tmux queue: its keys stay in order")
        XCTAssertFalse(Terminals.queue(for: "iterm:AB12") === Terminals.queue(for: tab))
        // A window opens on its own terminal's queue: a slow iTerm2 one never holds up Terminal.app's desks.
        XCTAssertTrue(Terminals.queue(opening: "terminal") === Terminal.queue)
        XCTAssertTrue(Terminals.queue(opening: "iterm") === Terminals.queue(for: "iterm:AB12"))
        XCTAssertFalse(Terminals.queue(opening: "ghostty") === Terminal.queue)
    }

    /// The phone page gives up on a desk's keys and on dictation with a message, rather than waiting silently.
    func testThePhonePageSaysWhenTheComputerDoesntAnswer() throws {
        let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let page = try String(contentsOf: repo.appendingPathComponent("Resources/web/deck.html"), encoding: .utf8)
        XCTAssertTrue(page.contains("const INPUT_WAIT = 20000;"))
        XCTAssertTrue(page.contains(#"const NO_ANSWER = "Your computer didn't answer in time. Look at the desk before you try again.";"#))
        // Over the Mac's own waits: 5 s to start typing, then a few seconds' keys.
        XCTAssertGreaterThan(20, Terminals.inputWait + 2 * Terminal.scriptTimeout + 1)
        // A timed-out request is told apart from a computer that can't be reached.
        XCTAssertTrue(page.contains(".catch(() => (signal && signal.aborted ? { timedOut: true } : undefined));"))
        let dictate = try XCTUnwrap(page.range(of: "async function dictate(session, text, send) {"))
        let body = page[dictate.upperBound...].prefix(500)
        XCTAssertTrue(body.contains("post('/dictate', { session }, timeLimit(INPUT_WAIT),"))
        XCTAssertTrue(body.contains("if (res.timedOut) return flash(NO_ANSWER);"))
        let send = try XCTUnwrap(page.range(of: "async function send(pathname, params, signal, said) {"))
        let sending = page[send.upperBound...].prefix(700)
        XCTAssertTrue(sending.contains("const input = pathname === '/act' && INPUT_ACTIONS.has(params.action);"))
        XCTAssertTrue(sending.contains("signal || (input ? timeLimit(INPUT_WAIT) : undefined)"))
        XCTAssertTrue(sending.contains("flash(NO_ANSWER);"))
        // A desk's keys; not a hire or a fire, which take their time.
        XCTAssertTrue(page.contains("const INPUT_ACTIONS = new Set(['accept', 'send', 'reject', 'esc', 'up', 'down', 'choose', 'button', 'clear', 'scroll']);"))
    }
}
