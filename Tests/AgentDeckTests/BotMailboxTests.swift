import XCTest
@testable import AgentDeck

/// Mail to and from a cloud bot through a folder (T404), in temp folders: nothing is typed, no network.
final class BotMailboxTests: XCTestCase {
    private var root: URL!
    private var box: URL!
    private var support: URL!
    private var worktree: URL!
    private var clock = Date(timeIntervalSince1970: 1_761_000_000)
    private var bots: [CloudBot] = []
    private var replies: [(String, BotMailbox.Reply)] = []
    private var limits = MailRouter.Limits(maxBytes: MailRouter.defaultMaxBytes, perHour: MailRouter.defaultPerHour)
    private let fm = FileManager.default

    override func setUpWithError() throws {
        root = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("botmail-\(UUID().uuidString)").resolvingSymlinksInPath()
        box = root.appendingPathComponent("box")
        support = root.appendingPathComponent("support")
        worktree = root.appendingPathComponent("tree")
        for folder in [box!, support!, worktree!] { try fm.createDirectory(at: folder, withIntermediateDirectories: true) }
        bots = [CloudBot(id: "scout", name: "Scout", mailbox: box.path)]
    }

    override func tearDownWithError() throws { try? fm.removeItem(at: root) }

    private func router() -> MailRouter {
        let mail = MailRouter(folder: support, limits: { self.limits }, hires: { [] }, bots: { self.bots },
                              barred: { [self.support, self.worktree] }, now: { self.clock })
        mail.onBotReply = { self.replies.append(($0.id, $1)) }
        return mail
    }

    private func send(_ mail: MailRouter, _ object: [String: Any]) -> (status: Int, json: [String: Any]) {
        let response = mail.send(from: .gm, body: try! JSONSerialization.data(withJSONObject: object))
        return (response.status, (try? JSONSerialization.jsonObject(with: response.body)) as? [String: Any] ?? [:])
    }

    private func leave(_ name: String, _ text: String, age: TimeInterval = 60) throws {
        let folder = box.appendingPathComponent("from-bot")
        try fm.createDirectory(at: folder, withIntermediateDirectories: true)
        let file = folder.appendingPathComponent(name)
        try text.write(to: file, atomically: true, encoding: .utf8)
        try fm.setAttributes([.modificationDate: clock.addingTimeInterval(-age)], ofItemAtPath: file.path)
    }

    private func names(_ folder: String) -> [String] {
        ((try? fm.contentsOfDirectory(atPath: box.appendingPathComponent(folder).path)) ?? []).sorted()
    }

    // MARK: To the bot

    func testAMessageBecomesADatedFileWithTheSenderLineAndIsRecorded() throws {
        let mail = router()
        let answer = send(mail, ["to": "Scout", "text": "Check the feed.", "id": "m1"])
        XCTAssertEqual(answer.status, 200)
        XCTAssertEqual(answer.json["state"] as? String, "delivered")
        XCTAssertEqual(answer.json["to"] as? String, "bot:scout")
        let files = names("to-bot")
        XCTAssertEqual(files.count, 1)
        XCTAssertTrue(files[0].hasSuffix("-m1.md"), files[0])
        XCTAssertTrue(files[0].hasPrefix("20251020T"), files[0])      // the message's time, in UTC
        let text = try String(contentsOf: box.appendingPathComponent("to-bot/\(files[0])"), encoding: .utf8)
        XCTAssertEqual(text, "— from the GM (Smolcorp), message m1\n\nCheck the feed.\n")
        // The outbox records it like any other, and keeps the text for the GM.
        let outbox = mail.outboxJSON(includeText: true)["messages"] as? [[String: Any]]
        XCTAssertEqual(outbox?.first?["to"] as? String, "bot:scout")
        XCTAssertEqual(outbox?.first?["toName"] as? String, "Scout")
        XCTAssertEqual(outbox?.first?["state"] as? String, "delivered")
        XCTAssertEqual(outbox?.first?["size"] as? Int, 15)
    }

    func testABotIsFoundByIdNameOrPrefixedIdAndTheSameIdWritesOnce() {
        let mail = router()
        XCTAssertEqual(send(mail, ["to": "scout", "text": "a", "id": "a"]).status, 200)
        XCTAssertEqual(send(mail, ["to": "bot:scout", "text": "b", "id": "b"]).status, 200)
        XCTAssertEqual(send(mail, ["to": "bot:scout", "text": "b", "id": "b"]).json["duplicate"] as? Bool, true)
        XCTAssertEqual(names("to-bot").count, 2)
        XCTAssertEqual(send(mail, ["to": "nobody", "text": "x"]).status, 404)
    }

    func testAWrittenFileIsNeverWrittenAgainEvenIfTheSaveWasLost() throws {
        let mail = router()
        _ = send(mail, ["to": "scout", "text": "a", "id": "once"])
        // A crash between the write and the save: the message is still queued, its file is there.
        let out = box.appendingPathComponent("to-bot")
        let delivery = BotMailbox.drop("a", id: "once", at: clock, mailbox: box.path, barred: [], commit: { XCTFail("no second try"); return true })
        XCTAssertEqual(delivery, .delivered)
        XCTAssertEqual(try fm.contentsOfDirectory(atPath: out.path).count, 1)
    }

    func testABotWithoutAUsableMailboxIsRefusedAtOnce() {
        bots.append(CloudBot(id: "bare", name: "Bare"))
        bots.append(CloudBot(id: "mine", name: "Mine", mailbox: support.appendingPathComponent("mail").path))
        bots.append(CloudBot(id: "tree", name: "Tree", mailbox: worktree.path))
        bots.append(CloudBot(id: "wide", name: "Wide", mailbox: root.path))
        try? fm.createDirectory(at: support.appendingPathComponent("mail"), withIntermediateDirectories: true)
        let mail = router()
        for name in ["bare", "mine", "tree", "wide"] {
            XCTAssertEqual(send(mail, ["to": name, "text": "x"]).status, 409, name)
        }
        XCTAssertTrue(mail.messages.isEmpty)
        XCTAssertTrue(names("to-bot").isEmpty)
    }

    func testAFolderNotThereYetWaitsAndIsTriedAgain() throws {
        let away = root.appendingPathComponent("synced")
        bots = [CloudBot(id: "scout", name: "Scout", mailbox: away.path)]
        let mail = router()
        let first = send(mail, ["to": "scout", "text": "later", "id": "w1"])
        XCTAssertEqual(first.status, 200)
        XCTAssertEqual(first.json["state"] as? String, "queued")
        try fm.createDirectory(at: away, withIntermediateDirectories: true)
        clock += 31
        mail.pump()
        XCTAssertEqual(mail.messages.last?.state, .delivered)
        XCTAssertEqual(try fm.contentsOfDirectory(atPath: away.appendingPathComponent("to-bot").path).count, 1)
    }

    func testABotTakenOffTheListFailsItsQueuedMail() {
        let away = root.appendingPathComponent("synced")
        bots = [CloudBot(id: "scout", name: "Scout", mailbox: away.path)]
        let mail = router()
        _ = send(mail, ["to": "scout", "text": "x", "id": "g1"])
        bots = []
        clock += 31
        mail.pump()
        XCTAssertEqual(mail.messages.last?.state, .failed)
        XCTAssertEqual(mail.messages.last?.note, "Scout is gone.")
    }

    func testAToBotThatIsALinkIsNotFollowed() throws {
        let elsewhere = root.appendingPathComponent("elsewhere")
        try fm.createDirectory(at: elsewhere, withIntermediateDirectories: true)
        try fm.createSymbolicLink(at: box.appendingPathComponent("to-bot"), withDestinationURL: elsewhere)
        let mail = router()
        XCTAssertEqual(send(mail, ["to": "scout", "text": "x", "id": "l1"]).json["state"] as? String, "failed")
        XCTAssertTrue(try fm.contentsOfDirectory(atPath: elsewhere.path).isEmpty)
    }

    // MARK: From the bot

    func testAReplyBecomesOneMessageFromThatBotAndIsMovedAside() throws {
        try leave("b.md", "Second.", age: 30)
        try leave("a.md", "First.", age: 60)
        let mail = router()
        mail.pump()
        XCTAssertEqual(replies.map(\.0), ["scout", "scout"])
        XCTAssertEqual(replies.map(\.1.text), ["First.", "Second."])      // oldest first
        XCTAssertEqual(names("from-bot"), ["read"])
        XCTAssertEqual(names("from-bot/read"), ["a.md", "b.md"])
        // Never read again, on the next pump or from a new router (an app restart).
        mail.pump()
        router().pump()
        XCTAssertEqual(replies.count, 2)
    }

    func testAFileStillArrivingALinkAndOtherKindsAreLeftAlone() throws {
        try leave("fresh.md", "Half a sync", age: 0)
        try leave("notes.pdf", "not mail")
        let outside = root.appendingPathComponent("secret.md")
        try "do not read".write(to: outside, atomically: true, encoding: .utf8)
        try fm.setAttributes([.modificationDate: clock.addingTimeInterval(-60)], ofItemAtPath: outside.path)
        try fm.createSymbolicLink(at: box.appendingPathComponent("from-bot/link.md"), withDestinationURL: outside)
        try leave(".hidden.md", "hidden")
        let mail = router()
        mail.pump()
        XCTAssertTrue(replies.isEmpty)
        XCTAssertEqual(names("from-bot").sorted(), [".hidden.md", "fresh.md", "link.md", "notes.pdf"].sorted())
        // The fresh one is taken once it has been still for a few seconds.
        clock += 10
        mail.pump()
        XCTAssertEqual(replies.map(\.1.name), ["fresh.md"])
        XCTAssertTrue(fm.fileExists(atPath: outside.path))
    }

    func testALongReplyIsCutAndAnEmptyOneIsMovedWithoutAMessage() throws {
        limits.maxBytes = 10
        try leave("long.md", String(repeating: "x", count: 50), age: 60)
        try leave("empty.md", "  \n", age: 50)
        let mail = router()
        mail.pump()
        XCTAssertEqual(replies.count, 1)
        XCTAssertEqual(replies[0].1.text, String(repeating: "x", count: 10) + "\n[cut at 10 bytes]")
        XCTAssertEqual(names("from-bot/read"), ["empty.md", "long.md"])
    }

    func testOnlySomeRepliesAreTakenAtATimeAndANameTakenInReadIsKept() throws {
        for i in 0..<(BotMailbox.mostPerLook + 3) { try leave(String(format: "r%02d.md", i), "n\(i)", age: 100 - Double(i)) }
        let read = box.appendingPathComponent("from-bot/read")
        try fm.createDirectory(at: read, withIntermediateDirectories: true)
        try "old".write(to: read.appendingPathComponent("r00.md"), atomically: true, encoding: .utf8)
        let mail = router()
        mail.pump()
        XCTAssertEqual(replies.count, BotMailbox.mostPerLook)
        mail.pump()
        XCTAssertEqual(replies.count, BotMailbox.mostPerLook + 3)
        XCTAssertEqual(try String(contentsOf: read.appendingPathComponent("r00.md"), encoding: .utf8), "old")
        XCTAssertEqual(try fm.contentsOfDirectory(atPath: read.path).count, BotMailbox.mostPerLook + 4)
    }

    func testNothingIsReadFromAMailboxThatIsNotAllowedNow() throws {
        try leave("a.md", "hi")
        // The folder became a hire's worktree: its files are not read.
        let mail = MailRouter(folder: support, limits: { self.limits }, hires: { [] }, bots: { self.bots },
                              barred: { [self.box] }, now: { self.clock })
        mail.onBotReply = { self.replies.append(($0.id, $1)) }
        mail.pump()
        XCTAssertTrue(replies.isEmpty)
        XCTAssertEqual(names("from-bot"), ["a.md"])
    }

    func testRepliesAreReadOnTheGivenQueueAndHandedBackOnTheMainOne() throws {
        try leave("a.md", "hi")
        let mail = router()
        mail.queue = DispatchQueue(label: "test.botmail")
        let done = expectation(description: "reply")
        mail.onBotReply = { _, reply in
            XCTAssertTrue(Thread.isMainThread)
            XCTAssertEqual(reply.text, "hi")
            done.fulfill()
        }
        mail.pump()
        wait(for: [done], timeout: 5)
    }

    // MARK: Choosing the folder

    func testChoosingAFolderIsCheckedAndKeptAsItsRealPath() throws {
        let barred = [support!, worktree!]
        let link = root.appendingPathComponent("link")
        try fm.createSymbolicLink(at: link, withDestinationURL: box)
        XCTAssertEqual(try BotMailbox.chosen(link.path, barred: barred).get(), box.path)
        XCTAssertNotNil(try? BotMailbox.chosen(" \(box.path) ", barred: barred).get())
        for bad in ["relative/folder", "", root.appendingPathComponent("nope").path, support.path,
                    support.appendingPathComponent("sub").path, worktree.path, root.path] {
            if case .success = BotMailbox.chosen(bad, barred: barred) { XCTFail("\(bad) was accepted") }
        }
        // A link into a barred folder is that folder.
        let sneaky = root.appendingPathComponent("sneaky")
        try fm.createSymbolicLink(at: sneaky, withDestinationURL: support)
        if case .success = BotMailbox.chosen(sneaky.path, barred: barred) { XCTFail("a link into the support folder was accepted") }
        let file = root.appendingPathComponent("file.txt")
        try "x".write(to: file, atomically: true, encoding: .utf8)
        if case .success = BotMailbox.chosen(file.path, barred: barred) { XCTFail("a file was accepted") }
    }

    func testTheMailboxIsKeptThroughTheConfigAndShownToThePage() throws {
        let bot = CloudBot(id: "scout", name: "Scout", mailbox: " \(box.path) ")
        XCTAssertEqual(bot.checked?.mailbox, box.path)
        let data = try JSONEncoder().encode(bot.checked)
        XCTAssertEqual(try JSONDecoder().decode(CloudBot.self, from: data).mailbox, box.path)
        // A config from before mailboxes has none.
        XCTAssertEqual(try JSONDecoder().decode(CloudBot.self, from: Data(#"{"id":"x","name":"X"}"#.utf8)).mailbox, "")
        XCTAssertEqual(bot.checked?.setupJSON["mailbox"] as? String, box.path)
    }
}
