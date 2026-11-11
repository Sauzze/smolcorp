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

    /// A dictation to a tmux desk goes in its usual time while Terminal.app's
    /// look at its tabs and a Terminal.app desk's key both hang.
    func testATmuxDesksTextGoesThroughWhileTerminalHangs() throws {
        let poll = Poller("test-tabs")
        let looked = expectation(description: "Terminal's tabs looked at")
        var open: Set<String>? = ["not asked"]
        XCTAssertTrue(poll.round({ Terminal.openTTYs() }) { open = $0; looked.fulfill() })
        Terminal.queue.async { _ = TerminalApp().focus(TabAddress.terminalApp(tty: "/dev/ttys999")) }
        XCTAssertEqual(osascripts(started: 2).count, 2, "both osascripts are hanging now")

        let started = Date()
        let answered = expectation(description: "answered")
        var answer: Response?
        let address = try XCTUnwrap(TabAddress(tab))
        Deck.deliver(to: tab, done: { answer = $0; answered.fulfill() }) { Tmux().insert("bonjour ça avance", in: address) }
        wait(for: [answered], timeout: 2)
        XCTAssertEqual(answer?.status, 204)
        XCTAssertLessThan(Date().timeIntervalSince(started), 1.5, "it didn't wait for Terminal")
        // The text on tmux's standard input, whole, then pasted.
        XCTAssertTrue(tmuxAsked.contains("load-buffer"), tmuxAsked)
        XCTAssertTrue(tmuxAsked.contains("stdin: bonjour ça avance\n"), tmuxAsked)
        XCTAssertTrue(tmuxAsked.contains("paste-buffer"), tmuxAsked)

        // Terminal's look ends at its time limit, unknown rather than "no tabs", and its osascript is ended by its pid.
        wait(for: [looked], timeout: Terminal.pollTimeout + 2)
        XCTAssertNil(open)
        Terminal.queue.sync {}
        XCTAssertEqual(osascripts.count, 2)
        for pid in osascripts { XCTAssertTrue(isGone(pid), "osascript \(pid) still runs") }
    }

    /// An input its terminal doesn't get to within the wait is answered as
    /// such, and never typed once the terminal is free: it could land in the
    /// middle of what's sent next.
    func testALateInputIsDroppedAndSaid() {
        let release = DispatchSemaphore(value: 0)
        Terminals.queue(for: tab).async { release.wait() } // a key ahead of it, stuck
        let answered = expectation(description: "answered")
        var answers: [Response] = []
        var typed = false
        Deck.deliver(to: tab, within: 0.3, done: { answers.append($0); answered.fulfill() }) {
            typed = true
            return true
        }
        wait(for: [answered], timeout: 2)
        release.signal()
        Terminals.queue(for: tab).sync {}
        RunLoop.main.run(until: Date().addingTimeInterval(0.2))
        XCTAssertFalse(typed, "not typed once its terminal was free")
        XCTAssertEqual(answers.map(\.status), [503], "answered once")
        XCTAssertEqual(answers.first.map(text), "tmux isn't answering, so nothing was typed. Try again in a moment.")
        XCTAssertEqual(Terminals.notAnswering("tty:/dev/ttys003"), "Terminal isn't answering, so nothing was typed. Try again in a moment.")
        XCTAssertEqual(Terminals.inputWait, 5)
    }

    /// The check is right before the first key: given up there, nothing is
    /// typed, even with the wait's own answer not yet given.
    func testTheDeadlineIsCheckedRightBeforeTyping() {
        let gaveUp = expectation(description: "answered")
        var answers: [Int] = []
        let late = Delivery(within: 0.05, late: .status(503, "late"), done: { answers.append($0.status); gaveUp.fulfill() })
        Thread.sleep(forTimeInterval: 0.1) // past its time, on a queue that got to it late; the main queue hasn't run yet
        XCTAssertFalse(late.begin())
        wait(for: [gaveUp], timeout: 1)
        RunLoop.main.run(until: Date().addingTimeInterval(0.1))
        XCTAssertEqual(answers, [503])

        // Begun in time, it's seen through and answered as it went, however long its keys take.
        let done = expectation(description: "done")
        var status: [Int] = []
        Deck.deliver(to: tab, within: 0.1, done: { status.append($0.status); done.fulfill() }) {
            Thread.sleep(forTimeInterval: 0.3)
            return true
        }
        wait(for: [done], timeout: 2)
        RunLoop.main.run(until: Date().addingTimeInterval(0.1))
        XCTAssertEqual(status, [204])

        // Refused before any key (gone, behind): that's the answer, and nothing runs.
        let refused = expectation(description: "refused")
        var ran = false
        Deck.deliver(to: tab, done: { XCTAssertEqual($0.status, 410); refused.fulfill() }, check: { .status(410, "That agent has ended.") }) {
            ran = true
            return true
        }
        wait(for: [refused], timeout: 2)
        XCTAssertFalse(ran)
    }

    /// A key whose terminal stopped answering part way says so, rather than "tab not found".
    func testAKeyWhoseTerminalHungIsSaidSo() {
        let answered = expectation(description: "answered")
        var answer: Response?
        let started = Date()
        Deck.deliver(to: "tty:/dev/ttys999", done: { answer = $0; answered.fulfill() }) {
            Terminal.run("return 1", timeout: 0.3) != nil
        }
        wait(for: [answered], timeout: 3)
        XCTAssertLessThan(Date().timeIntervalSince(started), 2)
        XCTAssertEqual(answer?.status, 503)
        XCTAssertEqual(answer.map(text), "Terminal stopped answering part way. Look at the desk before you try again.")
        XCTAssertTrue(Terminal.noticingTimeouts { Terminal.run("return 1", timeout: 0.2) }.timedOut)
        XCTAssertFalse(Terminal.noticingTimeouts { Tool.run("/bin/echo", ["hi"]) }.timedOut)
    }

    /// "Part way" only when something may have reached the tab: a key whose
    /// tab couldn't even be selected was never typed, and is said so.
    func testAKeyThatNeverWentSaysNothingWasTyped() {
        let answered = expectation(description: "answered")
        var answer: Response?
        // Terminal.app's own key: selecting its tab is what hangs, so no key was posted.
        Deck.deliver(to: "tty:/dev/ttys999", done: { answer = $0; answered.fulfill() }) {
            Terminal.press(.escape, tty: "/dev/ttys999")
        }
        wait(for: [answered], timeout: Terminal.scriptTimeout + 2)
        XCTAssertEqual(answer?.status, 503)
        XCTAssertEqual(answer.map(text), "Terminal isn't answering, so nothing was typed. Try again in a moment.")
        // A step that looks types nothing; one that may type, once started, counts even if it's ended.
        let looked = Terminal.noticingTimeouts { Terminal.onlyLooking { Tool.run("/bin/sleep", ["5"], timeout: 0.2) } }
        XCTAssertTrue(looked.timedOut)
        XCTAssertFalse(looked.partWay)
        let typed = Terminal.noticingTimeouts { () -> Bool in
            _ = Tool.run("/bin/echo", ["typed"])
            return Terminal.onlyLooking { Tool.run("/bin/sleep", ["5"], timeout: 0.2) } != nil
        }
        XCTAssertTrue(typed.partWay, "something went before it stopped answering")
        XCTAssertTrue(Terminal.noticingTimeouts { Tool.run("/bin/sleep", ["5"], timeout: 0.2) }.partWay, "it may have typed")
    }

    /// Terminal.app's text goes as key events, which no process carries: once
    /// they're posted, a later step that hangs is "part way", never "nothing
    /// was typed", which would invite a retry that types it twice. The posts
    /// are stood in for: no event ever reaches a real Terminal.
    func testTextThatWentBeforeAHangIsPartWay() throws {
        // Terminal answers the text's tab select, then hangs on Return's.
        let osascript = folder.appendingPathComponent("osascript-once")
        let answered = folder.appendingPathComponent("answered-once")
        try """
            #!/bin/sh
            echo $$ >> '\(osascriptPids.path)'
            [ -e '\(answered.path)' ] && exec sleep 60
            touch '\(answered.path)'
            echo yes
            """.write(to: osascript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: osascript.path)
        Terminal.osascript = osascript.path
        let savedPost = Terminal.postEvent, savedPID = Terminal.runningPID
        defer { Terminal.postEvent = savedPost; Terminal.runningPID = savedPID }
        var posted: [pid_t?] = []
        Terminal.queue.sync {
            Terminal.runningPID = { 999_999 }
            Terminal.postEvent = { _, pid in posted.append(pid) }
        }

        let done = expectation(description: "answered")
        var answer: Response?
        Deck.deliver(to: "tty:/dev/ttys999", done: { answer = $0; done.fulfill() }) {
            Terminal.insert("bonjour", tty: "/dev/ttys999") && Terminal.press(.returnKey, tty: "/dev/ttys999")
        }
        wait(for: [done], timeout: Terminal.scriptTimeout + 3)
        Terminal.queue.sync {}
        XCTAssertEqual(posted, [999_999, 999_999], "the text's key down and up, to the stand-in pid; no Return")
        XCTAssertEqual(osascripts.count, 2)
        XCTAssertTrue(osascripts.allSatisfy(isGone), "Return's select ended by its pid")
        XCTAssertEqual(answer?.status, 503)
        XCTAssertEqual(answer.map(text), "Terminal stopped answering part way. Look at the desk before you try again.")
    }

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

    /// Focusing a tmux desk shown in Terminal.app asks only tmux on tmux's
    /// queue: the Terminal tab is brought forward on Terminal's own. So with
    /// Terminal not answering, a tmux desk's dictation right after still goes
    /// in its usual time, and a Terminal.app desk's input is refused naming
    /// Terminal, not tmux.
    func testATmuxFocusNeverHoldsTmuxBehindTerminal() throws {
        let address = try XCTUnwrap(TabAddress(tab))
        let focused = expectation(description: "focused")
        var focus: Response?
        let started = Date()
        Deck.deliver(to: tab, done: { focus = $0; focused.fulfill() }) { Tmux().focus(address) }
        wait(for: [focused], timeout: 2)
        XCTAssertEqual(focus?.status, 204)
        let dictated = expectation(description: "dictated")
        var dictation: Response?
        Deck.deliver(to: tab, done: { dictation = $0; dictated.fulfill() }) { Tmux().insert("on y va", in: address) }
        wait(for: [dictated], timeout: 2)
        XCTAssertEqual(dictation?.status, 204)
        XCTAssertLessThan(Date().timeIntervalSince(started), 1.5, "it didn't wait for Terminal")
        XCTAssertTrue(tmuxAsked.contains("select-pane"), tmuxAsked)
        XCTAssertTrue(tmuxAsked.contains("stdin: on y va\n"), tmuxAsked)
        // Terminal's tab is being brought forward meanwhile, on Terminal's queue, and hangs there.
        XCTAssertEqual(osascripts(started: 1).count, 1, "Terminal was asked for the tmux client's tab")
        Thread.sleep(forTimeInterval: 0.3)
        XCTAssertEqual(osascripts.count, 1, "once")
        let refused = expectation(description: "refused")
        var refusal: Response?
        var typed = false
        Deck.deliver(to: "tty:/dev/ttys997", within: 0.3, done: { refusal = $0; refused.fulfill() }) {
            typed = true
            return true
        }
        wait(for: [refused], timeout: 2)
        XCTAssertEqual(refusal?.status, 503)
        XCTAssertEqual(refusal.map(text), "Terminal isn't answering, so nothing was typed. Try again in a moment.")
        // Done with it: iTerm2's turn after Terminal (not found there) fails at once, never a real osascript.
        Terminal.osascript = "/usr/bin/false"
        for pid in osascripts { kill(pid, SIGKILL) }
        Terminal.queue.sync {}
        Terminals.queue(kind: "iterm").sync {}
        XCTAssertFalse(typed, "never typed once Terminal was free")
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

    /// The Mac's own mic dictation goes as the phone's input does, and text
    /// dropped because its terminal didn't answer is said over its desk.
    func testMacDictationDroppedIsSaidOnTheDesk() throws {
        let body = try PhoneMicTests.body(of: "private func typeDictation(", until: "func typePhoneDictation(")
        XCTAssertTrue(body.contains("Deck.deliver(to: tab, done:"), "the same wait and check right before typing")
        XCTAssertTrue(body.contains("guard response.status == 503 else { return }"))
        XCTAssertTrue(body.contains("self?.noteVoice(String(decoding: response.body, as: UTF8.self), cleaned: false, in: tab)"))
        XCTAssertFalse(body.contains("Terminals.queue(for:"), "not around the wait")
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
        let send = try XCTUnwrap(page.range(of: "async function send(pathname, params, signal) {"))
        let sending = page[send.upperBound...].prefix(700)
        XCTAssertTrue(sending.contains("const input = pathname === '/act' && INPUT_ACTIONS.has(params.action);"))
        XCTAssertTrue(sending.contains("signal || (input ? timeLimit(INPUT_WAIT) : undefined)"))
        XCTAssertTrue(sending.contains("flash(NO_ANSWER);"))
        // A desk's keys; not a hire or a fire, which take their time.
        XCTAssertTrue(page.contains("const INPUT_ACTIONS = new Set(['accept', 'send', 'reject', 'esc', 'up', 'down', 'choose', 'button', 'clear', 'scroll']);"))
    }
}
