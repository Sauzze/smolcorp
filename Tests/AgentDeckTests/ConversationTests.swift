import XCTest
@testable import AgentDeck

/// iPhone app (personal build): the conversation the phone's blackboard
/// shows, read from transcripts of the tests' own: its redaction, line by
/// line, its pages, and who may read it. Offline.
final class ConversationTests: XCTestCase {
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    // MARK: Sample transcript lines, as Claude Code writes them

    private func json(_ object: [String: Any]) -> String {
        String(data: try! JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]), encoding: .utf8)!
    }

    private func user(_ text: String) -> String {
        json(["type": "user", "message": ["role": "user", "content": text], "timestamp": "2026-10-22T10:00:00.000Z"])
    }

    private func claude(_ text: String) -> String {
        json(["type": "assistant", "message": ["role": "assistant",
                                               "content": [["type": "thinking", "thinking": "private musing"], ["type": "text", "text": text]]]])
    }

    private func toolUse(_ name: String, _ input: [String: Any], id: String = "t1") -> String {
        json(["type": "assistant", "message": ["role": "assistant", "content": [["type": "tool_use", "id": id, "name": name, "input": input]]]])
    }

    private func toolResult(_ content: Any, id: String = "t1", error: Bool = false) -> String {
        json(["type": "user", "message": ["role": "user", "content": [["type": "tool_result", "tool_use_id": id, "content": content, "is_error": error]]]])
    }

    private func file(_ lines: [String], name: String = "session.jsonl") throws -> String {
        let url = folder.appendingPathComponent(name)
        try (lines.joined(separator: "\n") + "\n").write(to: url, atomically: true, encoding: .utf8)
        return url.path
    }

    private func page(_ lines: [String], before: UInt64? = nil, most: Int = Conversation.pageMost) throws -> Conversation.Page {
        try XCTUnwrap(Conversation.page(try file(lines), before: before, most: most))
    }

    /// Every line of every text the phone would get, as the route sends it.
    private func sentLines(_ page: Conversation.Page) throws -> [String] {
        let data = try JSONSerialization.data(withJSONObject: page.jsonObject)
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let items = try XCTUnwrap(object["items"] as? [[String: Any]])
        return items.flatMap { item -> [String] in
            [(item["text"] as? String ?? ""), (item["from"] as? String ?? "")]
                .flatMap { $0.split(separator: "\n", omittingEmptySubsequences: false).map(String.init) }
        }
    }

    // MARK: What it shows

    func testItShowsTheUsersWordsTheAgentsRepliesAndEachToolInALine() throws {
        let page = try page([
            json(["type": "summary", "summary": "old"]),
            user("Fix the login bug please"),
            claude("Looking at the login code."),
            toolUse("Read", ["file_path": "/repo/Login.swift"]),
            toolResult("1\timport Foundation\n2\tstruct Login {}\n3\t// … 400 more lines"),
            toolUse("Write", ["file_path": "/repo/Login.swift", "content": "the whole new file\nwith its lines"], id: "t2"),
            toolResult("File written", id: "t2"),
            claude("Fixed."),
        ])
        XCTAssertEqual(page.items.map(\.kind), [.user, .agent, .tool, .result, .tool, .result, .agent])
        XCTAssertEqual(page.items.map(\.text), [
            "Fix the login bug please", "Looking at the login code.", "Read: /repo/Login.swift", "1\timport Foundation",
            "Write: /repo/Login.swift", "File written", "Fixed.",
        ])
        XCTAssertEqual(page.items[2].from, "Read")
        XCTAssertNotNil(page.items[0].at)
        XCTAssertNil(page.before, "the whole file fits")
        // What a tool was given to write never shows; nor its thinking.
        let all = try sentLines(page).joined(separator: "\n")
        XCTAssertFalse(all.contains("the whole new file"))
        XCTAssertFalse(all.contains("private musing"))
        XCTAssertFalse(all.contains("400 more lines"), "a result's first line only")
    }

    func testAQuestionShowsWithItsOptionsAndItsAnswer() throws {
        let page = try page([
            toolUse("AskUserQuestion", ["questions": [["question": "Which colour?", "options": [["label": "Red"], ["label": "Blue"]]]]], id: "q1"),
            toolResult("User has answered your questions: \"Which colour?\"=\"Blue\". You can now continue with the user's answers in mind.",
                       id: "q1"),
        ])
        XCTAssertEqual(page.items.map(\.kind), [.question, .answer])
        XCTAssertEqual(page.items[0].text, "Which colour?\n1. Red\n2. Blue")
        XCTAssertEqual(page.items[1].text, "\"Which colour?\"=\"Blue\"")
    }

    func testCommandsAndInterruptionsAreNotesAndToolErrorsSaySo() throws {
        let page = try page([
            user("<command-name>/clear</command-name>"),
            user("[Request interrupted by user]"),
            toolUse("Bash", ["command": "swift test"]),
            toolResult("error: no such module\nmore", error: true),
        ])
        XCTAssertEqual(page.items.map(\.kind), [.note, .note, .tool, .result])
        XCTAssertEqual(page.items.map(\.text), ["ran /clear", "interrupted", "Bash: swift test", "Error: error: no such module"])
    }

    func testALongMessageIsCutWithMore() throws {
        let long = String(repeating: "word ", count: 2_000)
        let page = try page([user(long), claude("short")])
        XCTAssertEqual(page.items[0].text.count, Conversation.textMost)
        XCTAssertTrue(page.items[0].more)
        XCTAssertFalse(page.items[1].more)
        XCTAssertEqual((page.jsonObject["items"] as? [[String: Any]])?.first?["more"] as? Bool, true)
    }

    // MARK: Redaction, on every line

    func testEveryLineSentIsRedactedWhereverItComesFrom() throws {
        let secrets = ["sk-ant-abc123", "API_KEY=hunter2", "my password is swordfish", "Bearer xyz.abc", "ghp_abcdef123456",
                       "-----BEGIN RSA PRIVATE KEY-----", "the token is 42", "client_secret: shh"]
        var lines: [String] = []
        for (n, secret) in secrets.enumerated() {
            // A secret on a line of its own, between clean ones, in each kind of message.
            lines.append(user("first clean line\n\(secret)\nlast clean line"))
            lines.append(claude("before\n\(secret)\nafter"))
            lines.append(toolUse("Bash", ["command": "echo \(secret)"], id: "b\(n)"))
            lines.append(toolResult("\(secret)\nsecond line", id: "b\(n)"))
            lines.append(toolUse("AskUserQuestion", ["questions": [["question": "Use \(secret)?", "options": [["label": secret]]]]], id: "q\(n)"))
            lines.append(toolResult("User has answered your questions: \"x\"=\"\(secret)\".", id: "q\(n)"))
            lines.append(json(["type": "user", "origin": ["kind": "peer", "name": "gm \(secret)", "body": "hi\n\(secret)"],
                               "message": ["role": "user", "content": "hi"]]))
        }
        let page = try page(lines, most: 1_000)
        XCTAssertGreaterThan(page.items.count, secrets.count * 6)
        let sent = try sentLines(page)
        for line in sent {
            XCTAssertFalse(Redact.isSecret(line), "sent as is: \(line)")
            for secret in ["abc123", "hunter2", "swordfish", "xyz.abc", "abcdef123456", "RSA PRIVATE", "is 42", "shh"] {
                XCTAssertFalse(line.contains(secret), "\(secret) in \(line)")
            }
        }
        // The clean lines round them still show.
        XCTAssertTrue(sent.contains("first clean line"))
        XCTAssertTrue(sent.contains("last clean line"))
        XCTAssertTrue(sent.contains(Redact.placeholder))
        XCTAssertTrue(sent.contains("Bash: \(Redact.placeholder)"), "the tool's name stays, as Summary keeps it")
    }

    func testACutDoesntLetASecretThrough() throws {
        // The cut falls inside a secret's line: the line was redacted whole before.
        let text = String(repeating: "a", count: Conversation.textMost - 5) + " sk-ant-" + "0123456789" // in two, so no key-shaped string is committed whole
        let page = try page([user(text)])
        XCTAssertFalse(page.items[0].text.contains("sk-"))
        XCTAssertFalse(try sentLines(page).contains { Redact.isSecret($0) })
    }

    func testAResultAnAnswerOrACommandIsRedactedWholeBeforeItsCut() throws {
        // The secret's value before the cut, the word that gives it away after it.
        let padding = String(repeating: " x", count: 120)
        let result = "export DATABASE_URL=postgres://app:Pr0dPassw0rdXYZ@db.internal:5432/app" + padding + "; export STRIPE_SECRET_KEY=sk_live_1"
        let answered = "User has answered your questions: \"Which URL?\"=\"postgres://app:Pr0dPassw0rdXYZ@db\"" + padding
            + String(repeating: " y", count: TranscriptTail.textMost) + " token"
        let command = "<command-name>/deploy Pr0dPassw0rdXYZ" + String(repeating: "z", count: 80) + " --secret</command-name>"
        let page = try page([
            toolUse("Bash", ["command": "env"], id: "b1"), toolResult(result + "\nsecond line", id: "b1"),
            toolUse("AskUserQuestion", ["questions": [["question": "Which URL?"]]], id: "q1"), toolResult(answered, id: "q1"),
            user(command),
        ])
        XCTAssertEqual(page.items.map(\.kind), [.tool, .result, .question, .answer, .note])
        XCTAssertGreaterThan(result.count, TranscriptTail.resultMost, "the cut falls inside it")
        for item in page.items {
            XCTAssertFalse(item.text.contains("Pr0dPassw0rd"), "\(item.kind) sent \(item.text)")
        }
        XCTAssertEqual(page.items[1].text, Redact.placeholder)
        XCTAssertEqual(page.items[3].text, Redact.placeholder)
        XCTAssertEqual(page.items[4].text, "ran " + Redact.placeholder)
    }

    // MARK: Pages

    func testPagesGoBackWithoutGapsOrRepeats() throws {
        var lines: [String] = []
        for n in 1...250 { lines.append(n % 2 == 0 ? claude("reply \(n)") : user("ask \(n)")) }
        // Lines that show nothing in between, as a transcript has many.
        for n in stride(from: 240, to: 0, by: -10) { lines.insert(json(["type": "progress", "n": n]), at: n) }
        let path = try file(lines)

        var seen: [String] = []
        var before: UInt64?
        var pages = 0
        repeat {
            let page = try XCTUnwrap(Conversation.page(path, before: before, most: 40))
            XCTAssertLessThanOrEqual(page.items.count, 40)
            XCTAssertFalse(page.items.isEmpty)
            seen = page.items.map(\.text) + seen
            if let next = page.before { XCTAssertLessThan(next, before ?? .max) }
            before = page.before
            pages += 1
        } while before != nil && pages < 20
        XCTAssertNil(before, "the first page reaches the file's start")
        XCTAssertEqual(pages, 7)
        XCTAssertEqual(seen, (1...250).map { $0 % 2 == 0 ? "reply \($0)" : "ask \($0)" })
    }

    func testTheNewestPageEndsAtTheEndAndSaysTheFilesSize() throws {
        var lines: [String] = []
        for n in 1...100 { lines.append(user("ask \(n)")) }
        let path = try file(lines)
        let page = try XCTUnwrap(Conversation.page(path))
        XCTAssertEqual(page.items.count, Conversation.pageMost)
        XCTAssertEqual(page.items.last?.text, "ask 100")
        XCTAssertEqual(page.items.first?.text, "ask 41")
        let size = try FileManager.default.attributesOfItem(atPath: path)[.size] as? NSNumber
        XCTAssertEqual(page.size, size?.uint64Value)
        // Ids stay put as the file grows, for the phone to add only what's new.
        let ids = page.items.map { "\($0.offset).\($0.index)" }
        let handle = try XCTUnwrap(FileHandle(forWritingAtPath: path))
        handle.seekToEndOfFile()
        handle.write(Data((user("ask 101") + "\n").utf8))
        try handle.close()
        let grown = try XCTUnwrap(Conversation.page(path))
        XCTAssertEqual(grown.items.last?.text, "ask 101")
        XCTAssertEqual(Array(grown.items.map { "\($0.offset).\($0.index)" }.dropLast()), Array(ids.dropFirst()))
    }

    func testAToolsWholeOutputIsSkippedNotRead() throws {
        let huge = toolResult(String(repeating: "x", count: Conversation.lineMost + 10))
        let page = try page([user("before"), huge, user("after")])
        XCTAssertEqual(page.items.map(\.text), ["before", "after"])
    }

    func testAHalfWrittenLastLineIsLeftForLater() throws {
        let url = folder.appendingPathComponent("half.jsonl")
        try (user("done") + "\n" + #"{"type":"user","message":{"ro"#).write(to: url, atomically: true, encoding: .utf8)
        let page = try XCTUnwrap(Conversation.page(url.path))
        XCTAssertEqual(page.items.map(\.text), ["done"])
    }

    func testNoFileIsNoPage() {
        XCTAssertNil(Conversation.page(""))
        XCTAssertNil(Conversation.page(folder.appendingPathComponent("missing.jsonl").path))
    }

    func testAPagesPlaceIsDigitsOnly() {
        XCTAssertEqual(DeckRouter.offset(nil), .some(nil))
        XCTAssertEqual(DeckRouter.offset("1234"), .some(1234))
        for wrong in ["", "-1", "12a", "1e5", "99999999999999999999999", " 12"] {
            XCTAssertNil(DeckRouter.offset(wrong) as UInt64??, wrong)
        }
    }

    // MARK: Who may read it

    private func throughTailscale(_ path: String) -> Request {
        Request.addressed(method: "GET", path: path, query: [:],
                          headers: ["host": "my-mac.tail1234.ts.net", "x-forwarded-for": "100.101.102.103", "x-forwarded-proto": "https"],
                          body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")
    }

    private func overWiFi(_ path: String) -> Request {
        Request.addressed(method: "GET", path: path, query: [:], headers: ["host": "192.168.1.20:7710"], body: Data(),
                          fromLoopback: false, remoteAddress: "192.168.1.9")
    }

    private let phone = PairedPhone(id: "p1", name: "iPhone app", pairedAt: Date(), lastSeen: Date(), keyHash: "x")

    func testTheConversationGoesOnlyThroughTailscaleToAPairedPhone() {
        XCTAssertTrue(DeckRouter.showsConversation(throughTailscale("/conversation"), phone: phone))
        XCTAssertFalse(DeckRouter.showsConversation(overWiFi("/conversation"), phone: phone))
        XCTAssertFalse(DeckRouter.showsConversation(throughTailscale("/conversation"), phone: .officeWindow))
        let fromThisMac = Request.addressed(method: "GET", path: "/conversation", query: [:], headers: ["host": "localhost:7710"],
                                            body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")
        XCTAssertFalse(DeckRouter.showsConversation(fromThisMac, phone: .officeWindow))
        XCTAssertFalse(DeckRouter.showsConversation(fromThisMac, phone: phone))
    }

    /// The Mac's own office window reads it as it was written: nothing leaves the Mac. The phone's stays redacted.
    func testOnlyTheMacsOwnWindowReadsItUnredacted() throws {
        let path = try file([user("deploy with\nsk-ant-abc123"), claude("the password is swordfish"),
                             toolUse("Bash", ["command": "ls"]), toolResult("API_KEY=hunter2\nmore")])
        let mac = try XCTUnwrap(Conversation.page(path, redacted: false))
        XCTAssertEqual(mac.items.map(\.text), ["deploy with\nsk-ant-abc123", "the password is swordfish", "Bash: ls", "API_KEY=hunter2"])
        let phone = try XCTUnwrap(Conversation.page(path))
        XCTAssertFalse(phone.items.contains { $0.text.contains("abc123") || $0.text.contains("swordfish") || $0.text.contains("hunter2") })
        XCTAssertEqual(phone.items.count, mac.items.count)

        let fromThisMac = Request.addressed(method: "GET", path: "/conversation", query: [:], headers: ["host": "localhost:7710"],
                                            body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")
        XCTAssertTrue(DeckRouter.isOfficeWindow(fromThisMac, phone: .officeWindow))
        XCTAssertFalse(DeckRouter.isOfficeWindow(fromThisMac, phone: self.phone))
        XCTAssertFalse(DeckRouter.isOfficeWindow(throughTailscale("/conversation"), phone: .officeWindow))
        XCTAssertFalse(DeckRouter.isOfficeWindow(overWiFi("/conversation"), phone: .officeWindow))
    }

    /// ⓘ Info: to the Mac's window, or through Tailscale to a paired phone, as the conversation goes.
    func testTheInfoGoesToTheMacsWindowOrThroughTailscale() {
        let fromThisMac = Request.addressed(method: "GET", path: "/session/info", query: [:], headers: ["host": "localhost:7710"],
                                            body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")
        XCTAssertTrue(DeckRouter.showsInfo(fromThisMac, phone: .officeWindow))
        XCTAssertTrue(DeckRouter.showsInfo(throughTailscale("/session/info"), phone: phone))
        XCTAssertFalse(DeckRouter.showsInfo(overWiFi("/session/info"), phone: phone))
        XCTAssertFalse(DeckRouter.showsInfo(overWiFi("/session/info"), phone: .officeWindow))
    }

    func testTheDeviceTokenComesOnlyThroughTailscaleFromAPairedPhone() {
        XCTAssertTrue(DeckRouter.takesPushToken(throughTailscale("/notifications"), phone: phone))
        XCTAssertFalse(DeckRouter.takesPushToken(overWiFi("/notifications"), phone: phone))
        XCTAssertFalse(DeckRouter.takesPushToken(throughTailscale("/notifications"), phone: .officeWindow))
    }
}
