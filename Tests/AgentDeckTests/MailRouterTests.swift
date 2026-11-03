import XCTest
@testable import AgentDeck

/// The GM's mail core, with a stand-in delivery step: nothing is typed anywhere.
final class MailRouterTests: XCTestCase {
    private var root: URL!
    private var clock = Date(timeIntervalSince1970: 1_761_000_000)
    private var limits = MailRouter.Limits(maxBytes: MailRouter.defaultMaxBytes, perHour: MailRouter.defaultPerHour)
    private var hires: [FrontDesk.Hire] = []
    private var given: [(String, String)] = []
    private var answers: [MailRouter.Delivery] = []
    /// The contacts the router is told of, as "from>to".
    private var pairs: Set<String> = []
    /// The attempt count the outbox file held at the moment each try was committed (just before typing).
    private var onDiskAtCommit: [Int] = []

    private func hire(_ id: String, _ name: String, nickname: String = "", role: String = "", state: FrontDesk.State = .working) -> FrontDesk.Hire {
        FrontDesk.Hire(id: id, name: name, nickname: nickname, role: role, folder: "/f", state: state)
    }

    override func setUpWithError() throws {
        root = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("mail-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        hires = [hire("abcdef123456", "Reviewer", nickname: "Rev", role: "reviewer"), hire("gm0000000001", "General Manager", role: "gm")]
    }

    override func tearDownWithError() throws { try? FileManager.default.removeItem(at: root) }

    private func router() -> MailRouter {
        MailRouter(folder: root, limits: { self.limits }, hires: { self.hires }, mayWrite: { self.pairs.contains("\($0)>\($1)") }, deliver: { hire, text, commit in
            self.given.append((hire.id, text))
            let answer: MailRouter.Delivery = self.answers.isEmpty ? .delivered : self.answers.removeFirst()
            // Like the real step: the try is counted just before anything is typed, never for a wait.
            if case .waiting = answer { return answer }
            if case .failed = answer { return answer }
            guard commit() else { return .trouble("not typed") }
            self.onDiskAtCommit.append(self.attemptsOnDisk())
            return answer
        }, now: { self.clock })
    }

    private func attemptsOnDisk() -> Int {
        guard let data = try? Data(contentsOf: root.appendingPathComponent("outbox.json")),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let all = json["messages"] as? [[String: Any]], let last = all.last else { return -1 }
        return last["attempts"] as? Int ?? -1
    }

    private func outboxStamp() -> (Date?, Data?) {
        let file = root.appendingPathComponent("outbox.json")
        return ((try? FileManager.default.attributesOfItem(atPath: file.path))?[.modificationDate] as? Date, try? Data(contentsOf: file))
    }

    private func send(_ mail: MailRouter, _ object: [String: Any], from: MailRouter.Sender = .gm,
                      as signer: FrontDesk.Hire? = nil) -> (status: Int, json: [String: Any]) {
        let response = mail.send(from: from, signer: signer, body: try! JSONSerialization.data(withJSONObject: object))
        let json = (try? JSONSerialization.jsonObject(with: response.body)) as? [String: Any] ?? [:]
        return (response.status, json)
    }

    private func fixture(_ name: String) throws -> [String: Any] {
        try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: FrontDeskFixtures.url(name))) as? [String: Any])
    }

    func testSendingDeliversOnceWithTheSenderLineAndIsAnsweredAsTheFixture() throws {
        let mail = router()
        let spec = try fixture("send.json")
        let answer = send(mail, spec["request"] as! [String: Any])
        XCTAssertEqual(answer.status, 200)
        XCTAssertTrue((answer.json as NSDictionary).isEqual(to: spec["answer"] as! [String: Any]), "\(answer.json)")
        XCTAssertEqual(given.count, 1)
        XCTAssertEqual(given[0].0, "abcdef123456")
        XCTAssertEqual(given[0].1, spec["text-the-hire-gets"] as? String)
    }

    func testTheSameIdTwiceDeliversOnceEvenAfterItWasLeft() throws {
        let mail = router()
        let request: [String: Any] = ["to": "Rev", "text": "Hi", "id": "one"]
        _ = send(mail, request)
        let again = send(mail, request)
        XCTAssertEqual(again.json["duplicate"] as? Bool, true)
        XCTAssertEqual(given.count, 1)
        XCTAssertEqual(mail.messages.count, 1)
        // A left message stays left: the same id never starts it again.
        answers = Array(repeating: .trouble("tmux couldn't be asked."), count: 10)
        _ = send(mail, ["to": "Rev", "text": "Hi", "id": "two"])
        for _ in 0..<MailRouter.maxAttempts { clock += 31; mail.pump() }
        XCTAssertEqual(mail.messages.last?.state, .left)
        let tries = given.count
        XCTAssertEqual(send(mail, ["to": "Rev", "text": "Hi", "id": "two"]).json["state"] as? String, "left")
        clock += 3600; mail.pump()
        XCTAssertEqual(given.count, tries)
    }

    func testAnIdIsMadeWhenNoneIsGivenAndMustBePlain() {
        let mail = router()
        let made = send(mail, ["to": "Rev", "text": "Hi"])
        XCTAssertTrue((made.json["id"] as? String ?? "").hasPrefix("m-"))
        XCTAssertEqual(send(mail, ["to": "Rev", "text": "Hi", "id": "a b"]).status, 400)
        XCTAssertEqual(send(mail, ["to": "Rev", "text": "Hi", "id": String(repeating: "a", count: 65)]).status, 400)
        XCTAssertEqual(send(mail, ["to": "Rev", "text": "  \n"]).status, 400)
        XCTAssertEqual(send(mail, ["text": "x"]).status, 400)
    }

    func testThePageIsSignedAsTheOwner() {
        let mail = router()
        _ = send(mail, ["to": "Rev", "text": "Hello", "id": "p1"], from: .owner)
        XCTAssertEqual(given[0].1, "— from the owner (Smolcorp), message p1\n\nHello")
    }

    func testNeverIntoTheGMOrAGoneHire() {
        let mail = router()
        XCTAssertEqual(send(mail, ["to": "gm", "text": "x"]).status, 403)
        XCTAssertEqual(send(mail, ["to": "gm0000000001", "text": "x"]).status, 403)
        XCTAssertEqual(send(mail, ["to": "General Manager", "text": "x"]).status, 403)
        XCTAssertEqual(send(mail, ["to": "nobody", "text": "x"]).status, 404)
        hires[0].state = .fired
        XCTAssertEqual(send(mail, ["to": "Rev", "text": "x"]).status, 409)
        XCTAssertTrue(given.isEmpty)
        XCTAssertTrue(mail.messages.isEmpty)
    }

    func testAHireIsFoundByIdNicknameOrNameAndTwinsAreRefused() {
        hires.append(hire("bbbbbb222222", "Reviewer", nickname: "Rev2", role: "reviewer"))
        let mail = router()
        XCTAssertEqual(send(mail, ["to": "bbbbbb222222", "text": "x"]).json["to"] as? String, "bbbbbb222222")
        XCTAssertEqual(send(mail, ["to": "rev2", "text": "x"]).json["to"] as? String, "bbbbbb222222")
        XCTAssertEqual(send(mail, ["to": "Reviewer", "text": "x"]).status, 409)
    }

    func testTheLimitsAreSettingsAndNamedWhenHit() {
        let mail = router()
        limits = .init(maxBytes: 10, perHour: 2)
        let big = send(mail, ["to": "Rev", "text": String(repeating: "é", count: 6)])
        XCTAssertEqual(big.status, 413)
        XCTAssertEqual(MailRouter.defaultMaxBytes, 16384)
        XCTAssertEqual(MailRouter.defaultPerHour, 200)
        _ = send(mail, ["to": "Rev", "text": "a"]); _ = send(mail, ["to": "Rev", "text": "b"])
        let third = mail.send(from: .gm, body: Data(#"{"to":"Rev","text":"c"}"#.utf8))
        XCTAssertEqual(third.status, 429)
        XCTAssertTrue(String(decoding: third.body, as: UTF8.self).contains("mailPerHour"))
        // The limit is per sender, and per hour.
        XCTAssertEqual(send(mail, ["to": "Rev", "text": "c"], from: .owner).status, 200)
        clock += 3601
        XCTAssertEqual(send(mail, ["to": "Rev", "text": "d"]).status, 200)
    }

    func testTheDefaultsAreTheOwnersHigherOnesAndCannotBeZero() {
        XCTAssertEqual(Config().mailMaxBytes, 16384)
        XCTAssertEqual(Config().mailPerHour, 200)
        let file = root.appendingPathComponent("config.json")
        try? Data(#"{"mailMaxBytes": 0, "mailPerHour": -3}"#.utf8).write(to: file)
        XCTAssertEqual(Config.load(from: file).mailMaxBytes, 1)
        let config = Config()
        config.mailMaxBytes = 4096
        config.save(to: file)
        XCTAssertEqual(Config.load(from: file).mailMaxBytes, 4096)
    }

    func testRealTroubleIsRetriedThenLeftAndNeverResent() {
        let mail = router()
        answers = [.trouble("tmux couldn't be asked."), .trouble("tmux couldn't be asked."), .delivered]
        let first = send(mail, ["to": "Rev", "text": "Hi", "id": "w"])
        XCTAssertEqual(first.json["state"] as? String, "queued")
        mail.pump()                          // too soon
        XCTAssertEqual(given.count, 1)
        clock += 31; mail.pump()
        XCTAssertEqual(mail.messages[0].state, .queued)
        clock += 31; mail.pump()
        XCTAssertEqual(mail.messages[0].state, .delivered)
        XCTAssertEqual(given.count, 3)
        // Waiting every time: left after maxAttempts, and the note says it will not be sent again.
        answers = Array(repeating: .trouble("tmux didn't take the paste."), count: 20)
        _ = send(mail, ["to": "Rev", "text": "Hi", "id": "l"])
        for _ in 0..<(MailRouter.maxAttempts + 3) { clock += 31; mail.pump() }
        let left = mail.messages.last!
        XCTAssertEqual(left.state, .left)
        XCTAssertEqual(left.attempts, MailRouter.maxAttempts)
        XCTAssertTrue(left.note.contains("not sent again"))
    }

    func testWaitingForABusyHireIsNotAnAttemptAndLastsUntilTheLimit() {
        let mail = router()
        answers = Array(repeating: .waiting("waiting: at work"), count: 100_000)
        let first = send(mail, ["to": "Rev", "text": "Hi", "id": "w"])
        XCTAssertEqual(first.json["state"] as? String, "queued")
        XCTAssertEqual(first.json["note"] as? String, "waiting: at work")
        // Far more pumps than maxAttempts, every few seconds for hours: still queued, 0 attempts, tried on every pump.
        let tries = given.count
        for _ in 0..<(MailRouter.maxAttempts * 20) { clock += 5; mail.pump() }
        XCTAssertEqual(mail.messages[0].state, .queued)
        XCTAssertEqual(mail.messages[0].attempts, 0)
        XCTAssertEqual(given.count, tries + MailRouter.maxAttempts * 20)
        XCTAssertEqual(mail.outboxJSON(includeText: false)["messages"].flatMap { ($0 as? [[String: Any]])?.first?["note"] as? String }, "waiting: at work")
        // The hire is free at last: delivered.
        answers = [.delivered]
        clock += 5; mail.pump()
        XCTAssertEqual(mail.messages[0].state, .delivered)
        XCTAssertEqual(mail.messages[0].attempts, 1)
        // Almost a day of waiting is still waiting; a day is `left`, with the note, and never sent again.
        answers = Array(repeating: .waiting("waiting: at a question"), count: 100_000)
        _ = send(mail, ["to": "Rev", "text": "Hi", "id": "d"])
        let sent = mail.messages.last!.at
        clock = Date(timeIntervalSince1970: sent / 1000 + 24 * 3600 - 10); mail.pump()
        XCTAssertEqual(mail.messages.last?.state, .queued)
        clock += 10; mail.pump()
        XCTAssertEqual(mail.messages.last?.state, .left)
        XCTAssertTrue(mail.messages.last!.note.contains("not sent again"))
        XCTAssertTrue(mail.messages.last!.note.contains("waiting: at a question"))
        let after = given.count
        clock += 3600; mail.pump()
        XCTAssertEqual(given.count, after)
    }

    func testTheCountIsOnDiskBeforeAnythingIsTyped() {
        let mail = router()
        answers = [.trouble("tmux didn't take the paste."), .delivered]
        _ = send(mail, ["to": "Rev", "text": "Hi", "id": "c"])
        clock += 31; mail.pump()
        XCTAssertEqual(onDiskAtCommit, [1, 2], "each try was already counted in the file when it was committed")
        XCTAssertEqual(mail.messages[0].attempts, 2)
    }

    func testATroubleBeforeTypingStillCounts() {
        let mail = router()
        answers = [.trouble("tmux couldn't be asked.")]      // the stand-in commits for trouble too, so use the real order below
        _ = send(mail, ["to": "Rev", "text": "Hi", "id": "t"])
        XCTAssertEqual(mail.messages[0].attempts, 1)
        let counted = MailRouter.Message(id: "x", from: .gm, to: "a", toName: "A", text: "t", at: 0, state: .queued, size: 1)
        var message = counted
        MailRouter.settle(&message, .trouble("no session"), at: 1000, counted: false)
        XCTAssertEqual(message.attempts, 1, "not committed, so settle counts it")
        MailRouter.settle(&message, .trouble("no session"), at: 1000, counted: true)
        XCTAssertEqual(message.attempts, 1, "committed already, so settle doesn't count it twice")
    }

    func testWaitingForAHireWritesTheOutboxOnlyWhenSomethingChanges() {
        let mail = router()
        answers = Array(repeating: .waiting("waiting: at work"), count: 100_000)
        _ = send(mail, ["to": "Rev", "text": "Hi", "id": "q"])
        let before = outboxStamp()
        XCTAssertNotNil(before.0)
        for _ in 0..<120 { clock += 5; mail.pump() }
        let after = outboxStamp()
        XCTAssertEqual(after.0, before.0, "the file was not written again")
        XCTAssertEqual(after.1, before.1)
        // A new reason is a change: it is saved.
        answers = Array(repeating: .waiting("waiting: at a question"), count: 10)
        clock += 5; mail.pump()
        XCTAssertNotEqual(outboxStamp().1, before.1)
        XCTAssertTrue(String(decoding: outboxStamp().1 ?? Data(), as: UTF8.self).contains("waiting: at a question"))
    }

    func testTheWaitLimitIsASetting() {
        limits.waitHours = 1
        let mail = router()
        answers = Array(repeating: .waiting("waiting: at work"), count: 100)
        _ = send(mail, ["to": "Rev", "text": "Hi", "id": "s"])
        clock += 3601; mail.pump()
        XCTAssertEqual(mail.messages[0].state, .left)
        let file = root.appendingPathComponent("config.json")
        try? Data(#"{"mailWaitHours": 0}"#.utf8).write(to: file)
        XCTAssertEqual(Config.load(from: file).mailWaitHours, 1)
        XCTAssertEqual(Config().mailWaitHours, 24)
    }

    func testAGoneHireIsNotFailedWhileItsDeliveryIsStillOut() {
        let queue = DispatchQueue(label: "test-delivery")
        let release = DispatchSemaphore(value: 0)
        let mail = MailRouter(folder: root, limits: { self.limits }, hires: { self.hires }, deliver: { _, _, commit in
            release.wait(); _ = commit(); return .delivered
        }, now: { self.clock })
        mail.queue = queue
        _ = send(mail, ["to": "Rev", "text": "Hi", "id": "f"])
        hires[0].state = .fired
        clock += 3600; mail.pump()
        XCTAssertEqual(mail.messages[0].state, .queued, "in flight: left alone")
        release.signal()
        let done = expectation(description: "recorded")
        DispatchQueue.main.async { queue.async { DispatchQueue.main.async { done.fulfill() } } }
        wait(for: [done], timeout: 5)
        XCTAssertEqual(mail.messages[0].state, .delivered)
    }

    func testAFailureAndAHireGoneWhileQueuedAreFailed() {
        let mail = router()
        answers = [.failed("The window can't be reached.")]
        XCTAssertEqual(send(mail, ["to": "Rev", "text": "a"]).json["state"] as? String, "failed")
        answers = [.waiting("busy")]
        _ = send(mail, ["to": "Rev", "text": "b"])
        hires[0].state = .fired
        clock += 31; mail.pump()
        XCTAssertEqual(mail.messages.last?.state, .failed)
    }

    func testWithoutADeliveryStepAMessageStaysQueued() {
        let mail = MailRouter(folder: root, limits: { self.limits }, hires: { self.hires }, now: { self.clock })
        XCTAssertEqual(send(mail, ["to": "Rev", "text": "Hi"]).json["state"] as? String, "queued")
    }

    func testTheOutboxIsWrittenFirstKeptAcrossARestartAndReadAsTheFixture() throws {
        let mail = router()
        answers = [.waiting("busy")]
        _ = send(mail, ["to": "Rev", "text": "Hi", "id": "k"])
        let file = try JSONSerialization.jsonObject(with: Data(contentsOf: root.appendingPathComponent("outbox.json"))) as! [String: Any]
        XCTAssertEqual(file["version"] as? Int, 1)
        let stored = (file["messages"] as! [[String: Any]])[0]
        for key in ["id", "from", "to", "at", "state", "size", "text"] { XCTAssertNotNil(stored[key], key) }
        let attributes = try FileManager.default.attributesOfItem(atPath: root.appendingPathComponent("outbox.json").path)
        XCTAssertEqual((attributes[.posixPermissions] as? NSNumber)?.intValue, 0o600)
        // After a restart the message is still there, and still queued.
        let again = router()
        XCTAssertEqual(again.messages.map(\.id), ["k"])
        XCTAssertEqual(again.messages[0].state, .queued)

        let want = (try fixture("outbox.json")["answer"] as! [String: Any])["messages"] as! [[String: Any]]
        let shape = Set(want.flatMap { $0.keys })
        let shown = mail.outboxJSON(includeText: true)["messages"] as! [[String: Any]]
        XCTAssertTrue(Set(shown[0].keys).isSubset(of: shape), "\(shown[0].keys)")
        XCTAssertNil((mail.outboxJSON(includeText: false)["messages"] as! [[String: Any]])[0]["text"])
        XCTAssertEqual((mail.outboxJSON(limit: 1, includeText: true)["messages"] as! [Any]).count, 1)
    }

    /// The Go side reads `at` as a whole number (M4): a clock with a fraction of a millisecond must not leak into the answer.
    func testTheOutboxAnswersWholeMilliseconds() throws {
        clock = Date(timeIntervalSince1970: 1_793_524_230.0381179)
        let mail = router()
        answers = [.waiting("busy")]
        _ = send(mail, ["to": "Rev", "text": "Hi", "id": "frac"])
        let data = try JSONSerialization.data(withJSONObject: mail.outboxJSON(includeText: true))
        XCTAssertFalse(String(decoding: data, as: UTF8.self).contains("."), String(decoding: data, as: UTF8.self))
        let shown = try XCTUnwrap(mail.outboxJSON(includeText: true)["messages"] as? [[String: Any]])
        XCTAssertTrue(shown[0]["at"] is Int64, "\(type(of: shown[0]["at"] as Any))")
    }

    // MARK: A hire writes (T403)

    private func twoHires() -> (rev: FrontDesk.Hire, bob: FrontDesk.Hire) {
        hires.append(hire("bbbbbb222222", "Builder", nickname: "Bob", role: "builder"))
        return (hires[0], hires[2])
    }

    func testAHireMayWriteToTheGMAndNobodyElseByDefault() throws {
        let (rev, _) = twoHires()
        let mail = router()
        let spec = try XCTUnwrap(try fixture("send.json")["from-a-hire"] as? [String: Any])["to-the-gm"] as? [String: Any]
        let toGM = send(mail, try XCTUnwrap(spec?["request"] as? [String: Any]), from: .hire, as: rev)
        XCTAssertEqual(toGM.status, 200)
        XCTAssertTrue((toGM.json as NSDictionary).isEqual(to: try XCTUnwrap(spec?["answer"] as? [String: Any])), "\(toGM.json)")
        XCTAssertTrue(given.isEmpty, "never typed into the GM's session")
        XCTAssertEqual(send(mail, ["to": "General Manager", "text": "x"], from: .hire, as: rev).status, 200, "the GM by its name too")
        // Nobody else: not a hire, not an unknown name, not itself, all alike so a hire can't learn the team.
        for name in ["Bob", "bbbbbb222222", "nobody", "Rev"] {
            XCTAssertEqual(send(mail, ["to": name, "text": "x"], from: .hire, as: rev).status, 403, name)
        }
        XCTAssertTrue(given.isEmpty)
        let stored = try XCTUnwrap(mail.messages.first)
        XCTAssertEqual(stored.from, .hire)
        XCTAssertEqual(stored.fromID, rev.id)
        XCTAssertEqual(stored.state, .delivered)
    }

    func testAHireToGMMessageWakesTheInboxOnceAndOnlyThat() {
        let (rev, bob) = twoHires()
        let mail = router()
        let log = EventLog(folder: root.appendingPathComponent("events"))
        mail.onHireToGM = { hire, message in log.append(MailRouter.inboxEntry(from: hire, message)) }
        pairs = ["\(rev.id)>\(bob.id)"]
        // A GM already waiting on the inbox wakes for the message.
        var woken: [EventLog.Entry] = []
        log.wait(since: 0, limit: 10, timeout: 5) { woken = $0 }
        XCTAssertEqual(send(mail, ["to": "gm", "text": "Review done.", "id": "g1"], from: .hire, as: rev).status, 200)
        XCTAssertEqual(woken.count, 1)
        XCTAssertEqual(woken.first?.kind, .mail)
        XCTAssertEqual(woken.first?.detail, "Rev (hire \(rev.id)) wrote to you: Review done.")
        // The same id again is the same message: no second event.
        XCTAssertEqual(send(mail, ["to": "gm", "text": "Review done.", "id": "g1"], from: .hire, as: rev).json["duplicate"] as? Bool, true)
        XCTAssertEqual(log.lastID, 1)
        // Hire to hire (an allowed pair) is not for the GM's inbox.
        XCTAssertEqual(send(mail, ["to": "Bob", "text": "Hi", "id": "h1"], from: .hire, as: rev).status, 200)
        XCTAssertEqual(log.lastID, 1)
        // A refused message makes none either.
        XCTAssertEqual(send(mail, ["to": "nobody", "text": "x"], from: .hire, as: rev).status, 403)
        XCTAssertEqual(log.lastID, 1)
    }

    func testAHireWritesToAnotherOnlyForAnAllowedPairAndOneWayIsOneWay() {
        let (rev, bob) = twoHires()
        let mail = router()
        pairs = ["\(rev.id)>\(bob.id)"]
        XCTAssertEqual(send(mail, ["to": "Bob", "text": "Hi", "id": "c1"], from: .hire, as: rev).json["state"] as? String, "delivered")
        XCTAssertEqual(given.last?.0, bob.id)
        let back = mail.send(from: .hire, signer: bob, body: Data(#"{"to":"Rev","text":"x"}"#.utf8))
        XCTAssertEqual(back.status, 403, "the other way is not allowed")
        XCTAssertTrue(String(decoding: back.body, as: UTF8.self).contains("allowed that pair"))
        // A fired hire can't be written to even in an allowed pair.
        hires[2].state = .fired
        XCTAssertEqual(send(mail, ["to": "Bob", "text": "Hi"], from: .hire, as: rev).status, 403)
    }

    func testTheSenderLineNamesTheRealHireAndNeverTheGM() {
        let (_, bob) = twoHires()
        // A nickname meant to pass for the GM.
        hires[0].nickname = "the GM"
        let impostor = hires[0]
        let mail = router()
        pairs = ["\(impostor.id)>\(bob.id)", "\(bob.id)>\(impostor.id)"]
        _ = send(mail, ["to": "Bob", "text": "Pay me.", "id": "s1"], from: .hire, as: impostor)
        XCTAssertEqual(given.last?.1, "— from hire the GM [abcdef123456] (Smolcorp), message s1\n\nPay me.")
        XCTAssertFalse(given.last?.1.hasPrefix("— from the GM") ?? true)
        // The sender is whoever the key is for: the body can't name another.
        _ = send(mail, ["to": "abcdef123456", "text": "Hi", "id": "s2", "from": "gm", "fromID": impostor.id], from: .hire, as: bob)
        XCTAssertEqual(mail.messages.last?.fromID, bob.id)
        XCTAssertTrue(given.last?.1.hasPrefix("— from hire Bob [bbbbbb222222]") ?? false, given.last?.1 ?? "")
        // And without a hire behind the key, nothing is sent at all.
        XCTAssertEqual(send(mail, ["to": "gm", "text": "x"], from: .hire).status, 403)
    }

    func testAHiresLimitIsItsOwnAndItsIdsAreItsOwn() {
        let (rev, bob) = twoHires()
        limits = .init(maxBytes: 100, perHour: 1)
        let mail = router()
        XCTAssertEqual(send(mail, ["to": "gm", "text": "a", "id": "x1"], from: .hire, as: rev).status, 200)
        XCTAssertEqual(send(mail, ["to": "gm", "text": "b"], from: .hire, as: rev).status, 429)
        XCTAssertEqual(send(mail, ["to": "gm", "text": "c"], from: .hire, as: bob).status, 200, "Bob's own hour")
        XCTAssertEqual(send(mail, ["to": "gm", "text": "a", "id": "x1"], from: .hire, as: rev).json["duplicate"] as? Bool, true)
        // Bob can't learn what Rev's message was by asking with its id.
        XCTAssertEqual(send(mail, ["to": "gm", "text": "a", "id": "x1"], from: .hire, as: bob).status, 409)
    }

    func testAnOutboxWithoutTheNewFieldsStillLoads() throws {
        let old = #"{"version":1,"messages":[{"id":"o1","from":"gm","to":"abcdef123456","toName":"Rev","text":"Hi","at":1,"state":"delivered","size":2,"attempts":0,"nextAt":0,"note":""}]}"#
        try Data(old.utf8).write(to: root.appendingPathComponent("outbox.json"))
        let mail = router()
        XCTAssertEqual(mail.messages.map(\.id), ["o1"])
        XCTAssertNil(mail.messages[0].fromID)
        XCTAssertFalse(FileManager.default.fileExists(atPath: root.appendingPathComponent("outbox.broken.json").path))
    }

    func testABrokenOutboxIsSetAsideNotWiped() throws {
        try Data("not json".utf8).write(to: root.appendingPathComponent("outbox.json"))
        XCTAssertTrue(router().messages.isEmpty)
        XCTAssertTrue(FileManager.default.fileExists(atPath: root.appendingPathComponent("outbox.broken.json").path))
    }

    func testNothingIsWrittenOverAQuestionOrADraftOrWork() {
        let idle = HireFireTests.idle
        XCTAssertNil(MailRouter.gate(idle, deckAsking: false))
        // The deck knows it's asking even when the window looks idle and empty: it waits.
        guard case .waiting? = MailRouter.gate(idle, deckAsking: true) else { return XCTFail("an asking session was written into") }
        for capture in [HireFireTests.menu, "work\n│ > half typed │\n", "Thinking… (esc to interrupt)\n│ > │", "no box here", nil] {
            guard case .waiting? = MailRouter.gate(capture, deckAsking: false) else { return XCTFail("\(capture ?? "nil")") }
        }
    }

    func testTheRoutesAreInTheTableForTheGMAndThePage() {
        XCTAssertTrue(SetupRoutes.allows(.gm, "POST", "/setup/gm/send"))
        XCTAssertTrue(SetupRoutes.allows(.page, "POST", "/setup/gm/send"))
        XCTAssertFalse(SetupRoutes.allows(.tool, "POST", "/setup/gm/send"))
        XCTAssertTrue(SetupRoutes.allows(.tool, "GET", "/setup/gm/outbox"))
    }
}
