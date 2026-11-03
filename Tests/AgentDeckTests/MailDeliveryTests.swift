import XCTest
@testable import AgentDeck

/// Mail delivery with a stand-in tmux and socket: nothing is typed anywhere.
final class MailDeliveryTests: XCTestCase {
    private static let rule = String(repeating: "─", count: 60)
    private static let footer = "  ⏵⏵ auto mode on (shift+tab to cycle)"
    /// A window shaped like Claude Code's: some output, then the box between two rules, then the footer.
    private static func window(_ box: String, footer: String = MailDeliveryTests.footer) -> String {
        "some work\n\(rule)\n\(box)\n\(rule)\n\(footer)"
    }
    private static let idle = window("❯\u{a0}Try \"fix a bug\"")
    private static let menu = "some work\n\(rule)\n Bash command\n rm -rf build\n Do you want to proceed?\n ❯ 1. Yes\n   2. No\n\n Esc to cancel · Tab to amend"

    /// A capture Claude Code 2.1.316 really made, in Tests/fixtures/panes/.
    private static func pane(_ name: String) throws -> String {
        let folder = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("fixtures/panes")
        return try String(contentsOf: folder.appendingPathComponent(name + ".txt"), encoding: .utf8)
    }
    private var calls: [[String]] = []
    private var capture = MailDeliveryTests.idle
    /// What the pane shows after the paste: by default a box holding the message.
    private var captureAfter: String? = MailDeliveryTests.window("❯ the message")
    private var pasteState = "0 1"
    private var asking = false
    private var inboxOn = false
    private var inbox: (socket: String, version: String)?
    private var posted: [(String, Data)] = []
    private var postAnswer: String?
    private var failing: Set<String> = []

    private let hire = FrontDesk.Hire(id: "abcdef123456", name: "Reviewer", folder: "/f", tmuxSocket: "/tmp/tmux-501/smolcorp-abcdef123456", state: .working)

    private func runner() -> HireTmux.Runner {
        { arguments in
            self.calls.append(arguments)
            let verb = arguments.dropFirst(2).first ?? ""
            if self.failing.contains(verb) { return (1, "") }
            switch verb {
            case "has-session": return (0, "")
            case "capture-pane":
                let pasted = self.calls.contains { $0.contains("paste-buffer") }
                return (0, pasted ? (self.captureAfter ?? self.capture) : self.capture)
            case "display-message": return (0, self.pasteState)
            default: return (0, "")
            }
        }
    }

    private func ways() -> MailDelivery.Ways {
        MailDelivery.Ways(
            tmux: { _ in HireTmux(socketName: "smolcorp-abcdef123456", testPort: nil, runner: self.runner()) },
            deckAsking: { _ in self.asking },
            inbox: { _ in self.inbox },
            inboxOn: { self.inboxOn },
            post: { path, line in self.posted.append((path, line)); return self.postAnswer },
            pause: { _ in })
    }

    private func deliver(_ text: String = "— from the GM (Smolcorp), message m1\n\nPlease review.") -> MailRouter.Delivery {
        MailDelivery.deliver(hire, text, ways: ways())
    }

    private var verbs: [String] { calls.map { $0.dropFirst(2).first ?? "" } }
    private var pasted: Bool { verbs.contains("paste-buffer") }
    private var pressedReturn: Bool { calls.contains { $0.contains("send-keys") && $0.contains("Enter") } }

    // MARK: tmux

    func testAnIdleEmptyBoxIsPastedThenReturn() {
        XCTAssertEqual(deliver(), .delivered)
        let paste = calls.first { $0.contains("paste-buffer") }
        XCTAssertEqual(paste, ["-L", "smolcorp-abcdef123456", "paste-buffer", "-p", "-r", "-d", "-b", HireTmux.buffer, "-t", HireTmux.pane])
        XCTAssertTrue(pressedReturn)
        // Return comes after the paste, and nothing else was pressed.
        XCTAssertLessThan(verbs.firstIndex(of: "paste-buffer")!, verbs.lastIndex(of: "send-keys")!)
        XCTAssertEqual(verbs.filter { $0 == "send-keys" }.count, 1)
    }

    func testAMenuIsNeverWrittenInto() {
        capture = (try? Self.pane("permission-menu")) ?? ""
        guard case .waiting = deliver() else { return XCTFail("a menu must make it wait") }
        XCTAssertFalse(pasted)
        XCTAssertFalse(pressedReturn)
    }

    func testTheDeckAskingKeepsItWaitingEvenOverAnIdleLookingBox() {
        asking = true
        guard case .waiting(let why) = deliver() else { return XCTFail("asking must make it wait") }
        XCTAssertTrue(why.contains("question"), why)
        XCTAssertFalse(pasted)
        XCTAssertFalse(pressedReturn)
    }

    func testADraftAndWorkAreNeverWrittenOver() {
        capture = (try? Self.pane("draft")) ?? ""
        guard case .waiting = deliver() else { return XCTFail("a draft must make it wait") }
        capture = (try? Self.pane("busy-1")) ?? ""
        guard case .waiting = deliver() else { return XCTFail("work must make it wait") }
        capture = "nothing that looks like a box\n"
        guard case .waiting = deliver() else { return XCTFail("an unknown screen must make it wait") }
        XCTAssertFalse(pasted)
        XCTAssertFalse(pressedReturn)
    }

    func testTheWindowIsLookedAtBeforeTheDeckIsAskedAndBothBeforeThePaste() {
        var order: [String] = []
        var w = ways()
        w.deckAsking = { _ in order.append("deck"); return false }
        let mark = calls.count
        _ = MailDelivery.deliver(hire, "hi", ways: w)
        let verbsNow = calls.dropFirst(mark).map { $0.dropFirst(2).first ?? "" }
        XCTAssertEqual(order, ["deck", "deck"]) // before the paste, and again before Return
        XCTAssertLessThan(verbsNow.firstIndex(of: "capture-pane")!, verbsNow.firstIndex(of: "paste-buffer")!)
    }

    func testMultiLineWithoutBracketedModeGoesAsOneLineAndTheOutboxSaysSo() {
        pasteState = "0 0"
        let result = deliver("— from the GM (Smolcorp), message m1\n\nLine one\n\nLine two")
        guard case .deliveredAs(let note) = result else { return XCTFail("\(result)") }
        XCTAssertTrue(note.contains("one line"), note)
        let set = calls.first { $0.contains("set-buffer") }
        XCTAssertEqual(set?.last, "— from the GM (Smolcorp), message m1 / Line one / Line two")
        XCTAssertTrue(pressedReturn)
    }

    func testMultiLineWithBracketedModeIsPastedWhole() {
        pasteState = "0 1"
        XCTAssertEqual(deliver("a\nb"), .delivered)
        XCTAssertEqual(calls.first { $0.contains("set-buffer") }?.last, "a\nb")
    }

    func testAPaneInCopyModeIsNotWrittenInto() {
        pasteState = "1 1"
        guard case .waiting = deliver() else { return XCTFail("copy mode must make it wait") }
        XCTAssertFalse(pasted)
    }

    func testAControlCharacterIsRefusedAndNothingIsPasted() {
        let result = deliver("hello\u{1B}[201~\rrm -rf")
        guard case .failed = result else { return XCTFail("\(result)") }
        XCTAssertFalse(pasted)
        XCTAssertFalse(verbs.contains("set-buffer"))
        XCTAssertFalse(pressedReturn)
    }

    func testAQuestionThatAppearsAfterThePasteMeansNoReturn() {
        captureAfter = Self.menu
        guard case .failed(let why) = deliver() else { return XCTFail("must not press Return") }
        XCTAssertTrue(why.contains("not sent"), why)
        XCTAssertFalse(pressedReturn)
    }

    func testAMessageThatAsksAQuestionIsDeliveredWithOneReturn() {
        let text = "— from the GM (Smolcorp), message m2\n\nDo you want me to retry? (y/n)"
        // After the paste the box holds the message: its words are in the window, but the shape is a box.
        captureAfter = Self.window("❯ — from the GM (Smolcorp), message m2\n  Do you want me to retry? (y/n)")
        XCTAssertEqual(deliver(text), .delivered)
        XCTAssertEqual(verbs.filter { $0 == "send-keys" }.count, 1)
        XCTAssertTrue(pressedReturn)
    }

    func testAOneLineMessageWithQuestionWordsInTheBoxIsDelivered() {
        captureAfter = Self.window("❯ Do you want to proceed? press enter to continue (y/n) allow once")
        XCTAssertEqual(deliver("Do you want to proceed? press enter to continue (y/n) allow once"), .delivered)
        XCTAssertTrue(pressedReturn)
    }

    func testOldMailInTheScrollbackDoesNotBlockTheNext() {
        capture = "— from the GM (Smolcorp), message m1\n\nDo you want me to retry? (y/n)\nPress Enter to continue\n" + Self.idle
        XCTAssertEqual(deliver(), .delivered)
        XCTAssertTrue(pressedReturn)
    }

    func testAMenuAfterThePasteMeansNoReturnAndTheOutboxSaysTheBoxNeedsClearing() {
        captureAfter = (try? Self.pane("permission-menu")) ?? ""
        guard case .failed(let why) = deliver() else { return XCTFail("must not press Return") }
        XCTAssertTrue(why.contains("by hand"), why)
        XCTAssertFalse(pressedReturn)
    }

    func testABoxThatIsNotHoldingTheMessageAfterThePasteMeansNoReturn() {
        captureAfter = Self.idle // the paste did not land
        guard case .failed = deliver() else { return XCTFail("must not press Return") }
        XCTAssertFalse(pressedReturn)
    }

    func testTheDeckAskingAfterThePasteMeansNoReturn() {
        captureAfter = Self.window("❯ hello")
        var asks = 0
        var w = ways()
        w.deckAsking = { _ in asks += 1; return asks >= 2 } // not asking before the paste, asking after
        guard case .failed(let why) = MailDelivery.deliver(hire, "hello", ways: w) else { return XCTFail("must not press Return") }
        XCTAssertTrue(why.contains("question"), why)
        XCTAssertTrue(pasted)
        XCTAssertFalse(pressedReturn)
    }

    func testAFailedReturnIsNotDeliveredAndASessionGoneFails() {
        failing = ["send-keys"]
        guard case .failed = deliver() else { return XCTFail("a Return that failed isn't delivered") }
        failing = ["has-session"]
        guard case .trouble = deliver() else { return XCTFail("no session counts toward the attempts") }
    }

    func testAHireWithoutTmuxAndWithoutASocketFails() {
        var other = hire
        other.tmuxSocket = ""
        guard case .failed = MailDelivery.deliver(other, "hi", ways: ways()) else { return XCTFail("nothing to deliver with") }
    }

    // MARK: The inbox socket

    func testTheSocketIsOffUnlessSwitchedOnAndTheVersionIsKnown() {
        inbox = ("/tmp/cc-socks/1.sock", "2.1.309")
        XCTAssertEqual(deliver(), .delivered)
        XCTAssertTrue(posted.isEmpty, "switched off: tmux only")
        XCTAssertTrue(pasted)

        calls = []; inboxOn = true; inbox = ("/tmp/cc-socks/1.sock", "2.1.300")
        XCTAssertEqual(deliver(), .delivered)
        XCTAssertTrue(posted.isEmpty, "a version before the tested one: tmux")

        calls = []; inbox = ("/tmp/cc-socks/1.sock", "")
        _ = deliver()
        XCTAssertTrue(posted.isEmpty, "an unreadable version: tmux")
    }

    func testTheSocketIsUsedFirstWhenOnAndNewEnough() throws {
        inboxOn = true
        inbox = ("/tmp/cc-socks/1.sock", "2.2.0")
        XCTAssertEqual(deliver("hi"), .delivered)
        XCTAssertEqual(posted.count, 1)
        XCTAssertTrue(calls.isEmpty, "tmux wasn't touched")
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: posted[0].1) as? [String: Any])
        XCTAssertEqual(object["type"] as? String, "user")
        XCTAssertEqual((object["message"] as? [String: Any])?["content"] as? String, "hi")
        XCTAssertEqual(posted[0].1.last, 0x0A)
    }

    func testARefusingSocketFallsThroughToTmuxAndItsGate() {
        inboxOn = true
        inbox = ("/tmp/cc-socks/1.sock", "2.1.309")
        postAnswer = "refused: Connection refused"
        guard case .delivered = deliver() else { return XCTFail("the refused socket falls through to tmux") }
        XCTAssertEqual(posted.count, 1)
        XCTAssertTrue(pasted)
        calls = []; posted = []; capture = MailDeliveryTests.menu
        guard case .waiting = deliver() else { return XCTFail("tmux's gate still applies") }
        XCTAssertFalse(pasted)
    }

    func testTheSocketIsNeverUsedWhileTheDeckSaysItIsAsking() {
        inboxOn = true
        inbox = ("/tmp/cc-socks/1.sock", "2.1.309")
        asking = true
        guard case .waiting(let why) = deliver() else { return XCTFail("asking must make it wait") }
        XCTAssertEqual(why, "waiting: at a question")
        XCTAssertTrue(posted.isEmpty)
        XCTAssertFalse(pasted)
    }

    func testTabsBecomeSpacesInTheOneLineForm() {
        XCTAssertEqual(MailDelivery.oneLine("a\tb\n\n c\t"), "a b / c")
        pasteState = "0 0"
        _ = deliver("— from the GM\n\nstep\tone\nstep two")
        XCTAssertEqual(calls.first { $0.contains("set-buffer") }?.last, "— from the GM / step one / step two")
    }

    func testTmuxTroubleCountsAndAWindowThatIsBusyDoesNot() {
        failing = ["paste-buffer"]
        guard case .trouble = deliver() else { return XCTFail("a paste tmux didn't take is trouble") }
        failing = []; calls = []
        capture = (try? Self.pane("busy-1")) ?? ""
        guard case .waiting(let why) = deliver() else { return XCTFail("work is only waiting") }
        XCTAssertTrue(why.hasPrefix("waiting:"), why)
    }

    func testVersionsCompareByNumber() {
        XCTAssertTrue(MailDelivery.versionAtLeast("2.1.309", "2.1.309"))
        XCTAssertTrue(MailDelivery.versionAtLeast("2.1.1000", "2.1.309"))
        XCTAssertTrue(MailDelivery.versionAtLeast("2.2", "2.1.309"))
        XCTAssertFalse(MailDelivery.versionAtLeast("2.1.99", "2.1.309"))
        XCTAssertFalse(MailDelivery.versionAtLeast("2.1.x", "2.1.309"))
        XCTAssertFalse(MailDelivery.versionAtLeast("", "2.1.309"))
    }

    func testPostRefusesWhatIsNotASocketOfOurs() throws {
        let folder = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("sock-\(UUID().uuidString.prefix(6))")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let plain = folder.appendingPathComponent("a.sock")
        try Data("x".utf8).write(to: plain)
        XCTAssertEqual(MailDelivery.post(folder.appendingPathComponent("none.sock").path, Data()), "its socket isn't there")
        XCTAssertEqual(MailDelivery.post(plain.path, Data()), "that isn't a socket of yours")
    }

    // MARK: Through the router, off the main queue

    func testTheRouterCountsTheTryAtCommitAndRecordsTheResultOnMain() throws {
        let root = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("maildel-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: root) }
        let hires = [FrontDesk.Hire(id: "abcdef123456", name: "Reviewer", folder: "/f", state: .working)]
        let file = root.appendingPathComponent("outbox.json")
        var attemptsOnDisk: Int?
        var ranOnMain: Bool?
        let done = expectation(description: "delivered")
        let mail = MailRouter(folder: root, limits: { .init(maxBytes: 1000, perHour: 10) }, hires: { hires })
        mail.queue = DispatchQueue(label: "test.mail")
        mail.deliver = { _, _, commit in
            ranOnMain = Thread.isMainThread
            // The step counts the try just before typing; what a crash right now would find on disk.
            XCTAssertTrue(commit())
            let data = try? Data(contentsOf: file)
            let found = (data.flatMap { try? JSONSerialization.jsonObject(with: $0) } as? [String: Any])?["messages"] as? [[String: Any]]
            attemptsOnDisk = found?.first?["attempts"] as? Int
            return .deliveredAs("Sent as one line.")
        }
        let response = mail.send(from: .gm, body: try JSONSerialization.data(withJSONObject: ["to": "abcdef123456", "text": "hi", "id": "x1"]))
        XCTAssertEqual(response.status, 200)
        // Not delivered yet as far as the answer goes; it comes back to the main queue to be recorded.
        DispatchQueue.main.async { DispatchQueue.main.async { done.fulfill() } }
        wait(for: [done], timeout: 5)
        let end = Date().addingTimeInterval(5)
        while mail.messages[0].state == .queued, Date() < end { RunLoop.main.run(until: Date().addingTimeInterval(0.02)) }
        XCTAssertEqual(ranOnMain, false)
        XCTAssertEqual(attemptsOnDisk, 1)
        XCTAssertEqual(mail.messages[0].state, .delivered)
        XCTAssertEqual(mail.messages[0].note, "Sent as one line.")
    }

    // MARK: The typing helpers

    func testTmuxInsertRefusesControlCharactersToo() {
        XCTAssertFalse(HireTmux.canPaste("a\u{1B}[201~b"))
        XCTAssertFalse(HireTmux.canPaste("a\rb"))
        XCTAssertTrue(HireTmux.canPaste("a\nb\tc — é"))
        let before = Tmux.path
        defer { Tmux.path = before }
        Tmux.path = "/usr/bin/false"
        XCTAssertFalse(Tmux().insert("x\u{1B}[201~\r", in: TabAddress(kind: "tmux", id: "%1")))
    }

    /// The setup routes never reach the code that types: mail goes through `deck.mail.send` only.
    func testNoSetupRouteCallsTheTypingHelpersDirectly() throws {
        let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let router = try String(contentsOf: repo.appendingPathComponent("Sources/AgentDeck/DeckRouter.swift"), encoding: .utf8)
        let start = try XCTUnwrap(router.range(of: "private func setup(_ request: Request, caller:"))
        // T503's tiling only reads a hire's tmux clients (`HireTmux(socketName:)`); it types nothing.
        let setup = router[start.lowerBound...].replacingOccurrences(of: "HireTmux(socketName:", with: "")
        for call in ["MailDelivery.", "HireTmux", ".paste(", ".submit(", "pasteState(", "startMail("] {
            XCTAssertFalse(setup.contains(call), "setup calls \(call)")
        }
    }
}
