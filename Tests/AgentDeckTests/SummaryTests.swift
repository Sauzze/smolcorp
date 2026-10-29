import XCTest
@testable import AgentDeck

/// ✨ Summary: the transcript's tail, its redaction, and the call, on sample
/// transcripts of the tests' own. The network is stubbed: nothing is sent.
final class SummaryTests: XCTestCase {
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

    private func user(_ text: String, meta: Bool = false, uuid: String? = nil) -> String {
        var entry: [String: Any] = ["type": "user", "message": ["role": "user", "content": text]]
        if meta { entry["isMeta"] = true }
        if let uuid { entry["uuid"] = uuid }
        return json(entry)
    }

    private func claude(_ text: String) -> String {
        json(["type": "assistant", "message": ["role": "assistant", "model": "claude-opus-5-5",
                                               "content": [["type": "thinking", "thinking": "private musing"], ["type": "text", "text": text]]]])
    }

    private func toolUse(_ name: String, _ input: [String: Any]) -> String {
        json(["type": "assistant", "message": ["role": "assistant", "content": [["type": "tool_use", "id": "t1", "name": name, "input": input]]]])
    }

    private func toolResult(_ content: Any, error: Bool = false) -> String {
        json(["type": "user", "message": ["role": "user", "content": [["type": "tool_result", "tool_use_id": "t1", "content": content, "is_error": error]]]])
    }

    private func lines(_ lines: [String]) -> [Data] { lines.map { Data($0.utf8) } }

    private func file(_ lines: [String]) throws -> String {
        let url = folder.appendingPathComponent("session.jsonl")
        try (lines.joined(separator: "\n") + "\n").write(to: url, atomically: true, encoding: .utf8)
        return url.path
    }

    // MARK: The tail

    func testTheTailIsShortLinesOldestFirst() {
        let extract = TranscriptTail.extract(lines([
            json(["type": "summary", "summary": "old"]),
            user("Fix the login bug please"),
            claude("Looking at the login code."),
            toolUse("Read", ["file_path": "/repo/Login.swift"]),
            toolResult("1\timport Foundation\n2\tstruct Login {}\n3\t// … 400 more lines"),
            toolUse("Bash", ["command": "swift test\n--parallel", "description": "Run tests"]),
            toolResult("error: 2 tests failed\nmore output", error: true),
            claude("Two tests fail; should I update them?"),
        ]))
        XCTAssertEqual(extract.text, """
        LATEST REQUEST: Fix the login bug please
        CLAUDE: Looking at the login code.
        TOOL Read: /repo/Login.swift
        RESULT: 1\timport Foundation
        TOOL Bash: swift test
        ERROR: error: 2 tests failed
        CLAUDE: Two tests fail; should I update them?
        """)
        XCTAssertEqual(extract.language, "English")
        XCTAssertFalse(extract.text.contains("private musing"))
    }

    func testAFilesContentsAreNeverSentWholesale() {
        let body = (1...500).map { "line \($0) of a long file" }.joined(separator: "\n")
        let extract = TranscriptTail.extract(lines([
            user("Write it"),
            toolUse("Write", ["file_path": "/repo/Big.swift", "content": body]),
            toolResult(body),
        ]))
        XCTAssertFalse(extract.text.contains("line 2 of"))
        XCTAssertTrue(extract.text.contains("TOOL Write: /repo/Big.swift"))
        XCTAssertLessThan(extract.text.count, 200)
    }

    func testLongTextIsClipped() {
        let extract = TranscriptTail.extract(lines([claude(String(repeating: "x", count: 5_000))]))
        XCTAssertLessThanOrEqual(extract.text.count, "CLAUDE: ".count + TranscriptTail.textMost)
        XCTAssertTrue(extract.text.hasSuffix("…"))
    }

    func testOnlyTheEndFitsTheBudgetAndTheLastAskIsKept() {
        var log = [user("Migrate the database to Postgres")]
        for i in 1...400 { log.append(claude("Step \(i): moved one more table across, checked it, carried on.")) }
        let extract = TranscriptTail.extract(lines(log), budget: 2_000)
        XCTAssertLessThanOrEqual(extract.text.count, 2_000)
        XCTAssertTrue(extract.text.hasPrefix("LATEST REQUEST: Migrate the database to Postgres\n…"))
        XCTAssertTrue(extract.text.hasSuffix("Step 400: moved one more table across, checked it, carried on."))
        XCTAssertFalse(extract.text.contains("Step 1:"))
    }

    func testOnlyTheLatestAskIsMarked() {
        let extract = TranscriptTail.extract(lines([
            user("Fix the elevator", uuid: "u1"),
            claude("Done."),
            user("Now the summary", uuid: "u2"),
        ]))
        XCTAssertEqual(extract.text, "USER: Fix the elevator\nCLAUDE: Done.\nLATEST REQUEST: Now the summary")
        XCTAssertEqual(extract.latest, "u2")
        XCTAssertNil(TranscriptTail.extract(lines([claude("Nothing asked yet.")])).latest)
    }

    /// A long session on one thing, then a question on something else.
    func testALongSessionStartsAtItsLatestRequest() {
        var log = [user("Fix the elevator so the room slides through one window")]
        for i in 1...300 { log.append(claude("Elevator step \(i): moved the window, checked the slide, carried on.")) }
        log.append(claude(String(repeating: "Elevator notes. ", count: 80) + "Anything else?"))
        log.append(user("Why is my summary still about the elevator?"))
        log.append(toolUse("Read", ["file_path": "/repo/Summary.swift"]))
        log.append(claude("The prompt never says which request is the latest."))
        let text = TranscriptTail.extract(lines(log)).text
        let parts = text.components(separatedBy: "LATEST REQUEST: ")
        XCTAssertEqual(parts.count, 2)
        // Only a short lead-in comes before it: the end of what Claude said last, where it asked.
        XCTAssertLessThanOrEqual(parts[0].count, TranscriptTail.leadIn)
        XCTAssertTrue(parts[0].hasPrefix("…\nCLAUDE: …"))
        XCTAssertTrue(parts[0].hasSuffix("Anything else?\n"))
        XCTAssertFalse(text.contains("Elevator step"))
        XCTAssertEqual(parts[1], """
        Why is my summary still about the elevator?
        TOOL Read: /repo/Summary.swift
        CLAUDE: The prompt never says which request is the latest.
        """)
    }

    func testTheLeadInIsWholeLinesWhenTheyFit() {
        var log: [String] = []
        for i in 1...50 { log.append(claude("Old \(i).")) }
        log.append(claude("Shall I?"))
        log.append(user("yes"))
        let text = TranscriptTail.extract(lines(log)).text
        XCTAssertTrue(text.hasPrefix("…\n"))
        XCTAssertTrue(text.hasSuffix("CLAUDE: Old 50.\nCLAUDE: Shall I?\nLATEST REQUEST: yes"))
        XCTAssertFalse(text.contains("Old 1."))
        XCTAssertLessThanOrEqual(text.components(separatedBy: "LATEST REQUEST: ")[0].count, TranscriptTail.leadIn)
    }

    // MARK: Requests as Claude Code writes them (field names from real transcripts, words made up)

    /// A prompt sent while the agent works: an attachment queued into its turn.
    private func queued(_ prompt: Any, uuid: String, origin: [String: Any] = ["kind": "human"], mode: String = "prompt",
                        meta: Bool = false) -> String {
        var attachment: [String: Any] = ["type": "queued_command", "commandMode": mode, "origin": origin, "prompt": prompt,
                                         "delivery_id": "d-\(uuid)", "source_uuid": "s-\(uuid)", "timestamp": "2026-10-22T01:00:00.000Z"]
        if origin["kind"] as? String == "human" { attachment["humanTurn"] = true }
        if meta { attachment["isMeta"] = true }
        return json(["type": "attachment", "uuid": uuid, "timestamp": "2026-10-22T01:00:00.000Z", "attachment": attachment])
    }

    private let peerOrigin: [String: Any] = ["kind": "peer", "name": "peer-one", "from": "uds:/tmp/cc-socks/1.sock",
                                             "fromMode": "prompting", "msg_id": "m1", "verifiedPeerPid": 1, "body": "Commit both now."]
    private let peerWrapped = """
        <cross-session-message from="uds:/tmp/cc-socks/1.sock" from-name="peer-one" from-mode="prompting">
        Commit both now.
        </cross-session-message>
        """

    func testAPromptQueuedWhileTheAgentWorksIsTheLatest() {
        let extract = TranscriptTail.extract(lines([
            user("Fix the elevator", uuid: "u1"),
            claude("On it."),
            queued("Stop, look at the summary instead", uuid: "q1"),
            claude("Looking at the summary."),
            queued("<task-notification>\n<task-id>b1</task-id>\n</task-notification>", uuid: "t1",
                   origin: ["kind": "task-notification", "producer": "bash"], mode: "task-notification"),
        ]))
        XCTAssertEqual(extract.text, """
        USER: Fix the elevator
        CLAUDE: On it.
        LATEST REQUEST: Stop, look at the summary instead
        CLAUDE: Looking at the summary.
        """)
        XCTAssertEqual(extract.latest, "q1")
        // With an image, only its words.
        let image = TranscriptTail.extract(lines([
            queued([["type": "image", "source": ["type": "base64", "data": "AAAA"]], ["type": "text", "text": "What's wrong here?"]], uuid: "q2"),
        ]))
        XCTAssertEqual(image.text, "LATEST REQUEST: What's wrong here?")
    }

    func testAPastedPromptIsARequest() {
        let pasted = json(["type": "user", "uuid": "u2", "promptSource": "typed", "origin": ["kind": "human"],
                           "message": ["role": "user", "content": "<pasted_content>a log</pasted_content>\nWhy does this fail?"]])
        let extract = TranscriptTail.extract(lines([user("Fix the elevator", uuid: "u1"), pasted]))
        XCTAssertEqual(extract.latest, "u2")
        XCTAssertTrue(extract.text.hasSuffix("LATEST REQUEST: <pasted_content>a log</pasted_content>\nWhy does this fail?"))
    }

    func testACompactionSummaryIsNoRequest() {
        let compacted = json(["type": "user", "uuid": "c1", "isCompactSummary": true, "isVisibleInTranscriptOnly": true,
                              "message": ["role": "user", "content": "This session is being continued from a previous conversation that ran out of context. The elevator…"]])
        let extract = TranscriptTail.extract(lines([user("Look at the summary", uuid: "u1"), claude("Reading it."), compacted, claude("Carrying on.")]))
        XCTAssertEqual(extract.latest, "u1")
        XCTAssertEqual(extract.text, "LATEST REQUEST: Look at the summary\nCLAUDE: Reading it.\nCLAUDE: Carrying on.")
    }

    func testAPeerMessageAfterTheRequestIsMarkedAsSuch() {
        let asUser = json(["type": "user", "uuid": "p1", "isMeta": true, "promptSource": "system", "origin": peerOrigin,
                           "message": ["role": "user", "content": "Another Claude session sent a message:\n" + peerWrapped]])
        let extract = TranscriptTail.extract(lines([
            user("Corrige le résumé, s'il te plaît", uuid: "u1"),
            claude("Done."),
            asUser,
            claude("Committing."),
        ]))
        // The request stays the user's; the peer's message, newer, is marked as what it is.
        XCTAssertEqual(extract.text, """
        LATEST REQUEST: Corrige le résumé, s'il te plaît
        CLAUDE: Done.
        LATEST PEER MESSAGE (from peer-one): Commit both now.
        CLAUDE: Committing.
        """)
        XCTAssertEqual(extract.latest, "u1 p1")
        // The summary is in the user's language, not the peer's.
        XCTAssertEqual(extract.language, "French")

        // Sent while the agent works: queued, its isMeta in the attachment. Only the newest is marked.
        let queuedPeer = TranscriptTail.extract(lines([asUser, claude("Committing."), queued(peerWrapped, uuid: "p2", origin: peerOrigin, meta: true)]))
        XCTAssertEqual(queuedPeer.text, "PEER peer-one: Commit both now.\nCLAUDE: Committing.\nLATEST PEER MESSAGE (from peer-one): Commit both now.")
        XCTAssertEqual(queuedPeer.latest, "p2")
        // One from before the request isn't marked.
        let before = TranscriptTail.extract(lines([asUser, user("Now the README", uuid: "u2")]))
        XCTAssertEqual(before.text, "PEER peer-one: Commit both now.\nLATEST REQUEST: Now the README")
        XCTAssertEqual(before.latest, "u2")
    }

    func testAReportHandedBackIsNoRequest() {
        var origin = peerOrigin
        origin["name"] = "summary-fixer"
        origin["handback"] = true
        origin["handbackReportText"] = "All done, tests pass."
        origin["body"] = "A background session finished:\nAll done, tests pass."
        origin["senderTaskId"] = "t1"
        let report = json(["type": "user", "uuid": "r1", "isMeta": true, "promptSource": "system", "origin": origin,
                           "message": ["role": "user", "content": "<agent-message from=\"summary-fixer\">All done, tests pass.</agent-message>"]])
        let extract = TranscriptTail.extract(lines([user("Fix the summary", uuid: "u1"), claude("Sent it to a fixer."), report]))
        XCTAssertEqual(extract.text, "LATEST REQUEST: Fix the summary\nCLAUDE: Sent it to a fixer.\nREPORT from summary-fixer: All done, tests pass.")
        XCTAssertEqual(extract.latest, "u1")
        let queuedReport = TranscriptTail.extract(lines([queued("<agent-message>…</agent-message>", uuid: "r2", origin: origin, meta: true)]))
        XCTAssertEqual(queuedReport.text, "REPORT from summary-fixer: All done, tests pass.")
        XCTAssertNil(queuedReport.latest)
    }

    func testWhenTooLongTheRequestAndALaterPeerMessageBothLead() {
        var log = [user("Fix the elevator", uuid: "u1")]
        for i in 1...200 { log.append(claude("Step \(i): moved one more table across, checked it, carried on.")) }
        log.append(json(["type": "user", "uuid": "p1", "isMeta": true, "origin": peerOrigin, "message": ["role": "user", "content": peerWrapped]]))
        for i in 201...400 { log.append(claude("Step \(i): moved one more table across, checked it, carried on.")) }
        let extract = TranscriptTail.extract(lines(log), budget: 2_000)
        XCTAssertTrue(extract.text.hasPrefix("LATEST REQUEST: Fix the elevator\n…\nLATEST PEER MESSAGE (from peer-one): Commit both now.\n…\n"))
        XCTAssertTrue(extract.text.hasSuffix("Step 400: moved one more table across, checked it, carried on."))
        XCTAssertLessThanOrEqual(extract.text.count, 2_000)
        XCTAssertEqual(extract.latest, "u1 p1")
    }

    func testANewerPeerMessageIsSummarisedAfresh() throws {
        var log = [user("Fix the elevator", uuid: "u1"), claude("On it.")]
        let path = try file(log)
        var calls = 0
        let summaries = Summaries(transport: { _, done in
            calls += 1
            done(Data(#"{"content":[{"type":"text","text":"Summary \#(calls)"}]}"#.utf8), 200, nil)
        }, key: { "test-key" })
        XCTAssertEqual(wait(summaries, "s1", path), .success("Summary 1"))
        log.append(queued(peerWrapped, uuid: "p1", origin: peerOrigin, meta: true))
        _ = try file(log)
        XCTAssertEqual(wait(summaries, "s1", path), .success("Summary 2"))
    }

    // MARK: Redacted before it's cut

    func testALineIsRedactedBeforeItsCut() {
        // Each secret comes before the word that gives it away, which is past where its line is cut.
        let pad = { (n: Int) in String(repeating: "x", count: n) }
        let text = TranscriptTail.extract(lines([
            claude(pad(1_480) + " hunter2a is the password"),
            toolUse("Bash", ["command": "echo " + pad(120) + " hunter2b # the password"]),
            toolResult(pad(140) + " hunter2c is the password"),
            user(pad(1_480) + " hunter2d is my password"),
        ])).text
        for secret in ["hunter2a", "hunter2b", "hunter2c", "hunter2d"] { XCTAssertFalse(text.contains(secret), secret) }
        XCTAssertEqual(text, "CLAUDE: [redacted]\nTOOL Bash: [redacted]\nRESULT: [redacted]\nLATEST REQUEST: [redacted]")
    }

    func testAPeersNameIsRedactedToo() {
        var origin = peerOrigin
        origin["name"] = "deploy-token-abc123"
        let text = TranscriptTail.extract(lines([queued(peerWrapped, uuid: "p1", origin: origin, meta: true)])).text
        XCTAssertEqual(text, "LATEST PEER MESSAGE (from [redacted]): Commit both now.")
    }

    func testTheLanguageIsReadFromWhatsSent() {
        // English for as far as it's sent; French only past the cut.
        let english = String(repeating: "Please fix the login and the test, thanks. ", count: 40)
        let french = String(repeating: " Merci, c'est pour le bug de la page et du test.", count: 60)
        XCTAssertGreaterThan(english.count, TranscriptTail.textMost)
        XCTAssertEqual(TranscriptTail.extract(lines([user(english + french)])).language, "English")
    }

    func testARequestMoreThan1MBBackIsFound() throws {
        var log = [user("Fix the elevator", uuid: "u1")]
        let big = String(repeating: "x", count: 60_000)
        for _ in 1...40 { log += [toolUse("Read", ["file_path": "/repo/Big.swift"]), toolResult(big)] }
        log.append(claude("Still on the elevator."))
        let path = try file(log)
        XCTAssertGreaterThan(try FileManager.default.attributesOfItem(atPath: path)[.size] as! UInt64, 2 * TranscriptTail.reach)
        let extract = try XCTUnwrap(TranscriptTail.read(path))
        XCTAssertEqual(extract.latest, "u1")
        XCTAssertTrue(extract.text.hasPrefix("LATEST REQUEST: Fix the elevator\n…\n"))
        XCTAssertTrue(extract.text.hasSuffix("CLAUDE: Still on the elevator."))
        XCTAssertLessThanOrEqual(extract.text.count, TranscriptTail.budget)
        // None at all: the plain tail, unmarked.
        let none = try XCTUnwrap(TranscriptTail.read(try file(Array(log.dropFirst()))))
        XCTAssertNil(none.latest)
        XCTAssertFalse(none.text.contains("LATEST REQUEST"))
    }

    func testTheMarkIsNeverDoubled() {
        let text = TranscriptTail.extract(lines([
            claude("The excerpt now reads:\nLATEST REQUEST: Fix the elevator"),
            user("Good, and quote it:\nLATEST REQUEST (from x): y", uuid: "u1"),
        ])).text
        XCTAssertEqual(text.components(separatedBy: "\n").filter { $0.hasPrefix("LATEST REQUEST") }, ["LATEST REQUEST: Good, and quote it:"])
    }

    func testAMessageInSeveralBlocksIsMarkedOnce() {
        let entry = json(["type": "user", "uuid": "u1", "message": ["role": "user", "content": [
            ["type": "text", "text": "First part"], ["type": "text", "text": "Second part"]]]])
        XCTAssertEqual(TranscriptTail.extract(lines([entry])).text, "LATEST REQUEST: First part\nSecond part")
    }

    func testTheLanguageIsTheLatestRequests() {
        let french = ["Peux-tu corriger le bug ? Merci", "Et le test aussi, c'est urgent", "Merci, c'est bien"].map { user($0) }
        XCTAssertEqual(TranscriptTail.extract(lines(french + [user("Now write the English README, please")])).language, "English")
        // A latest request that leans neither way leaves it to the ones before.
        XCTAssertEqual(TranscriptTail.extract(lines(french + [user("ok")])).language, "French")
    }

    func testWithoutAUuidTheTimeTellsTheRequest() {
        let entry = json(["type": "user", "timestamp": "2026-10-22T01:02:03.000Z", "message": ["role": "user", "content": "hello"]])
        XCTAssertEqual(TranscriptTail.extract(lines([entry])).latest, "2026-10-22T01:02:03.000Z")
    }

    func testClaudeCodesOwnNotesAndSubagentsAreLeftOut() {
        let extract = TranscriptTail.extract(lines([
            user("<system-reminder>hidden</system-reminder>"),
            user("Caveat: the messages below were generated by the user while running local commands."),
            user("<command-name>/clear</command-name>\n<command-message>clear</command-message>"),
            user("<local-command-stdout>done</local-command-stdout>"),
            user("expanded skill text", meta: true),
            json(["type": "assistant", "isSidechain": true, "message": ["role": "assistant", "content": "a subagent's words"]]),
            user("[Request interrupted by user]"),
            "not json at all",
        ]))
        XCTAssertEqual(extract.text, "USER ran /clear\n(the user interrupted)")
    }

    func testFrenchIsTold() {
        let extract = TranscriptTail.extract(lines([
            user("Peux-tu corriger le bug de connexion ? C'est urgent, merci."),
            claude("I'll look at it."),
        ]))
        XCTAssertEqual(extract.language, "French")
        XCTAssertEqual(Language.of(["Can you fix the login bug? Thanks."]), "English")
    }

    func testReadingAFile() throws {
        let path = try file([user("Hello there, can you help"), claude("Sure.")])
        XCTAssertEqual(TranscriptTail.read(path)?.text, "LATEST REQUEST: Hello there, can you help\nCLAUDE: Sure.")
        XCTAssertNil(TranscriptTail.read(folder.appendingPathComponent("missing.jsonl").path))
        XCTAssertNil(TranscriptTail.read(""))
        XCTAssertNil(TranscriptTail.read(try file([json(["type": "summary"])])))
    }

    // MARK: Redaction

    func testSecretsAreRedactedLineByLine() {
        let text = TranscriptTail.extract(lines([
            // Put together here, so no file holds a key-shaped string whole.
            user("Use this key " + "sk-" + "ant-api03-abcdef please"),
            toolUse("Bash", ["command": "export RESEND_API_KEY=re_123abc && npm run send"]),
            toolResult("re_9aZ live key"),
            claude("Set the password to hunter2.\nThen deploy."),
            claude("The GitHub token is ghp_abcdefghijkl"),
            toolResult("Authorization: Bearer abc.def"),
            toolUse("Read", ["file_path": "/repo/task-runner.swift"]),
            claude("Stored in store_result and are_ok; the desk-sk-mode works."),
        ])).text
        XCTAssertEqual(text, """
        LATEST REQUEST: [redacted]
        TOOL Bash: [redacted]
        RESULT: [redacted]
        CLAUDE: [redacted]
        Then deploy.
        CLAUDE: [redacted]
        RESULT: [redacted]
        TOOL Read: /repo/task-runner.swift
        CLAUDE: Stored in store_result and are_ok; the desk-sk-mode works.
        """)
        for secret in ["sk-ant", "re_123", "re_9aZ", "hunter2", "ghp_", "abc.def"] {
            XCTAssertFalse(text.contains(secret), secret)
        }
    }

    /// The review's additions: Stripe keys, JWTs, a URL's password, a bare AWS
    /// secret key, and a private key from its BEGIN through its END. Each made
    /// up and put together here, so no file holds one whole.
    func testMoreKindsOfSecretAreRedacted() {
        let stripe = "sk_" + "live_" + "4eC39HqLyjWDarjtT1zdp7dc"
        let restricted = "rk_" + "live_" + "51H8abcDEF"
        let jwt = "eyJ" + "hbGciOiJIUzI1NiJ9" + "." + "eyJzdWIiOiIxMjM0In0" + "." + "abc123"
        let aws = "wJalrXUtnFEMI/" + "K7MDENG/bPxRfiCY" + "EXAMPLEKEY"
        let lines = [
            "STRIPE=" + stripe, "the restricted one " + restricted, "Authorization header " + jwt,
            "DATABASE_URL=postgres://app:" + "hunter2" + "@db.local:5432/app", "aws " + aws,
            "-----BEGIN " + "OPENSSH PRIVATE KEY-----", "b3BlbnNzaC1rZXktdjEAAAAA", "AAAABG5vbmUAAAAEbm9uZQ",
            "-----END " + "OPENSSH PRIVATE KEY-----", "after the key",
            "commit 8d2f1c9a4b7e6d5c3b2a1f0e9d8c7b6a5f4e3d2c",
        ]
        let redacted = Redact.text(lines.joined(separator: "\n")).split(separator: "\n").map(String.init)
        XCTAssertEqual(redacted, Array(repeating: Redact.placeholder, count: 9) + ["after the key", lines.last!],
                       "a git commit's hash isn't a secret")
    }

    func testALongLineIsRedactedQuickly() {
        // Every line is redacted whole before it's cut, and a tool's result can be one line of megabytes.
        let line = String(repeating: "x", count: 200_000)
        let start = Date()
        XCTAssertFalse(Redact.isSecret(line))
        XCTAssertTrue(Redact.isSecret(line + " mongodb+srv://app:" + "hunter2" + "@db.local/app"))
        XCTAssertLessThan(Date().timeIntervalSince(start), 2)
    }

    func testARunOfJWTStartsIsRedactedQuickly() {
        let run = String(repeating: "eyJ", count: 66_667)
        let start = Date()
        XCTAssertFalse(Redact.isSecret(run))
        XCTAssertLessThan(Date().timeIntervalSince(start), 2)
        // A JWT is still caught, alone or after a word; put together here, so no file holds one whole.
        let jwt = "eyJ" + "hbGciOiJIUzI1NiJ9" + "." + "eyJ" + "zdWIiOiIxMjM0In0" + "." + "c2lnbmF0dXJl"
        for line in [jwt, "jwt " + jwt, "\"" + jwt + "\"", "cookie=" + jwt] { XCTAssertTrue(Redact.isSecret(line), line) }
    }

    func testAPasswordOnlyURLIsRedacted() {
        XCTAssertTrue(Redact.isSecret("REDIS_URL=redis://:" + "hunter2" + "@cache:6379"))
        XCTAssertFalse(Redact.isSecret("see https://example.com/a:b@c"))
    }

    func testTheHeadsFitASmallBudget() {
        var log = [user(String(repeating: "Fix the elevator. ", count: 80), uuid: "u1")]
        for i in 1...100 { log.append(claude("Step \(i): moved one more table across, checked it, carried on.")) }
        log.append(queued(String(repeating: "Commit both now. ", count: 80), uuid: "p1", origin: peerOrigin, meta: true))
        for i in 101...200 { log.append(claude("Step \(i): moved one more table across, checked it, carried on.")) }
        for budget in [700, 1_200, 1_500] {
            let text = TranscriptTail.extract(lines(log), budget: budget).text
            XCTAssertLessThanOrEqual(text.count, budget)
            XCTAssertTrue(text.hasPrefix("LATEST REQUEST: Fix the elevator."))
            XCTAssertTrue(text.contains("\nLATEST PEER MESSAGE (from peer-one): "))
        }
    }

    func testWhatLooksLikeASecret() {
        for line in ["OPENAI sk-proj-xyz", "API_KEY=abc", "\"apiKey\": \"x\"", "my Password is", "access_token", "client secret",
                     "AKIAIOSFODNN7EXAMPLE", "xoxb-123", "-----BEGIN RSA PRIVATE KEY-----", "re_AbC123"] {
            XCTAssertTrue(Redact.isSecret(line), line)
        }
        for line in ["fix the task-list", "prepare_build", "the monkey ran", "Building for debugging"] {
            XCTAssertFalse(Redact.isSecret(line), line)
        }
    }

    func testTheLastAskIsRedactedToo() {
        var log = [user("Deploy with token abc123")]
        for i in 1...200 { log.append(claude("Working on step \(i) of the deploy, nothing to report yet.")) }
        let text = TranscriptTail.extract(lines(log), budget: 1_000).text
        XCTAssertTrue(text.hasPrefix("LATEST REQUEST: [redacted]"))
        XCTAssertFalse(text.contains("abc123"))
    }

    // MARK: The request and its answer

    func testTheRequestCarriesOnlyTheTail() throws {
        let extract = TranscriptTail.Extract(text: "USER: hi", language: "French")
        let request = SummaryRequest.request(extract, project: "smolcorp", key: "test-key")
        XCTAssertEqual(request.url?.absoluteString, "https://api.anthropic.com/v1/messages")
        XCTAssertEqual(request.value(forHTTPHeaderField: "anthropic-version"), "2023-06-01")
        XCTAssertEqual(request.timeoutInterval, 15)
        let body = try XCTUnwrap(JSONSerialization.jsonObject(with: request.httpBody!) as? [String: Any])
        XCTAssertEqual(body["model"] as? String, "claude-haiku-5-5")
        XCTAssertEqual(body["max_tokens"] as? Int, 300)
        XCTAssertEqual((body["thinking"] as? [String: String])?["type"], "disabled")
        XCTAssertTrue((body["system"] as? String ?? "").contains("Write in French"))
        // The summary is about the latest request; what came before is background.
        let system = body["system"] as? String ?? ""
        XCTAssertTrue(system.contains("If a line starts with LATEST REQUEST, it is what the user last asked: summarise that request and the work since."))
        XCTAssertTrue(system.contains("if it gives the agent its current task, say what that task is in one line."))
        XCTAssertTrue(system.contains("Lines starting REPORT are reports other sessions handed back, not requests."))
        XCTAssertTrue(system.contains("Lines before it are background only: mention older work only if it is still in progress."))
        // And it holds where no line is marked.
        XCTAssertTrue(system.contains("If no line starts with LATEST REQUEST, summarise the most recent work in the excerpt."))
        let message = try XCTUnwrap((body["messages"] as? [[String: Any]])?.first)
        XCTAssertEqual(message["content"] as? String, "Project: smolcorp\n\nTranscript excerpt, oldest first:\nUSER: hi")
        // The key goes in its header only.
        XCTAssertFalse(String(data: request.httpBody!, encoding: .utf8)!.contains("test-key"))
    }

    func testAnswers() {
        let ok = Data(#"{"content":[{"type":"text","text":"  Asked: fix login.\nDid: ran tests.  "}]}"#.utf8)
        XCTAssertEqual(SummaryRequest.answer(ok, status: 200, error: nil), .success("Asked: fix login.\nDid: ran tests."))
        XCTAssertEqual(SummaryRequest.answer(nil, status: nil, error: URLError(.timedOut)), .failure(.timeout))
        XCTAssertEqual(SummaryRequest.answer(nil, status: nil, error: URLError(.notConnectedToInternet)), .failure(.unreachable))
        XCTAssertEqual(SummaryRequest.answer(Data(), status: 401, error: nil), .failure(.badKey))
        XCTAssertEqual(SummaryRequest.answer(Data(), status: 403, error: nil), .failure(.badKey))
        XCTAssertEqual(SummaryRequest.answer(Data(), status: 429, error: nil), .failure(.busy))
        XCTAssertEqual(SummaryRequest.answer(Data(), status: 529, error: nil), .failure(.busy))
        XCTAssertEqual(SummaryRequest.answer(Data(), status: 500, error: nil).failureMessage, "Anthropic said no (500).")
        XCTAssertEqual(SummaryRequest.answer(Data(#"{"content":[]}"#.utf8), status: 200, error: nil), .failure(.empty))
    }

    // MARK: Summaries, with the network stubbed

    private func wait(_ summaries: Summaries, _ id: String, _ path: String) -> Result<String, SummaryError> {
        let done = expectation(description: "summary")
        var got: Result<String, SummaryError>!
        summaries.summarize(id, transcript: path, project: "p") { got = $0; done.fulfill() }
        wait(for: [done], timeout: 5)
        return got
    }

    func testASecondTapWithin30SecondsIsKept() throws {
        let path = try file([user("hello"), claude("hi")])
        var calls = 0
        var clock = Date(timeIntervalSince1970: 1_000)
        let summaries = Summaries(transport: { _, done in
            calls += 1
            done(Data(#"{"content":[{"type":"text","text":"Summary \#(calls)"}]}"#.utf8), 200, nil)
        }, key: { "test-key" }, now: { clock })
        XCTAssertEqual(wait(summaries, "s1", path), .success("Summary 1"))
        clock += 20
        XCTAssertEqual(wait(summaries, "s1", path), .success("Summary 1"))
        XCTAssertEqual(calls, 1)
        clock += 15
        XCTAssertEqual(wait(summaries, "s1", path), .success("Summary 2"))
        XCTAssertEqual(calls, 2)
    }

    func testANewRequestWithin30SecondsIsSummarisedAfresh() throws {
        var log = [user("Fix the elevator", uuid: "u1"), claude("On it.")]
        let path = try file(log)
        var calls = 0
        var clock = Date(timeIntervalSince1970: 1_000)
        let summaries = Summaries(transport: { _, done in
            calls += 1
            done(Data(#"{"content":[{"type":"text","text":"Summary \#(calls)"}]}"#.utf8), 200, nil)
        }, key: { "test-key" }, now: { clock })
        XCTAssertEqual(wait(summaries, "s1", path), .success("Summary 1"))
        // The agent carrying on with the same request keeps it.
        log.append(claude("Still on the elevator."))
        _ = try file(log)
        clock += 5
        XCTAssertEqual(wait(summaries, "s1", path), .success("Summary 1"))
        // A new message, even the same words, is a new request.
        log.append(user("Fix the elevator", uuid: "u2"))
        _ = try file(log)
        clock += 5
        XCTAssertEqual(wait(summaries, "s1", path), .success("Summary 2"))
        XCTAssertEqual(calls, 2)
    }

    func testTwoTapsInFlightMakeOneCall() throws {
        let path = try file([user("hello"), claude("hi")])
        var calls = 0
        var release: (() -> Void)?
        let sent = expectation(description: "sent")
        let summaries = Summaries(transport: { _, done in
            calls += 1
            release = { done(Data(#"{"content":[{"type":"text","text":"One"}]}"#.utf8), 200, nil) }
            sent.fulfill()
        }, key: { "test-key" })
        let both = expectation(description: "both")
        both.expectedFulfillmentCount = 2
        var results: [Result<String, SummaryError>] = []
        summaries.summarize("s1", transcript: path, project: "p") { results.append($0); both.fulfill() }
        summaries.summarize("s1", transcript: path, project: "p") { results.append($0); both.fulfill() }
        wait(for: [sent], timeout: 5)
        release?()
        wait(for: [both], timeout: 5)
        XCTAssertEqual(calls, 1)
        XCTAssertEqual(results, [.success("One"), .success("One")])
    }

    func testNoKeyOrNoHistorySendsNothing() throws {
        var calls = 0
        let transport: Summaries.Transport = { _, _ in calls += 1 }
        let path = try file([user("hello")])
        XCTAssertEqual(wait(Summaries(transport: transport, key: { nil }), "s1", path), .failure(.noKey))
        XCTAssertEqual(wait(Summaries(transport: transport, key: { "k" }), "s2", folder.appendingPathComponent("none").path), .failure(.noHistory))
        XCTAssertEqual(calls, 0)
    }

    func testAFailureIsNotKept() throws {
        let path = try file([user("hello")])
        var calls = 0
        let summaries = Summaries(transport: { _, done in
            calls += 1
            done(nil, nil, URLError(.timedOut))
        }, key: { "k" })
        XCTAssertEqual(wait(summaries, "s1", path), .failure(.timeout))
        XCTAssertEqual(wait(summaries, "s1", path), .failure(.timeout))
        XCTAssertEqual(calls, 2)
    }

    func testNoMoreThanTwentySummariesInTenMinutes() throws {
        let path = try file([user("hello"), claude("hi")])
        var calls = 0
        var clock = Date(timeIntervalSince1970: 1_000)
        let summaries = Summaries(transport: { _, done in
            calls += 1
            done(Data(#"{"content":[{"type":"text","text":"Summary"}]}"#.utf8), 200, nil)
        }, key: { "k" }, now: { clock })
        for n in 0..<20 { XCTAssertEqual(wait(summaries, "s\(n)", path), .success("Summary")) }
        XCTAssertEqual(wait(summaries, "s-over", path), .failure(.tooMany))
        XCTAssertEqual(calls, 20, "the one over the limit is never sent")
        XCTAssertEqual(wait(summaries, "s0", path), .success("Summary"), "a summary kept costs no call")
        XCTAssertEqual(calls, 20)
        clock += Summaries.window
        XCTAssertEqual(wait(summaries, "s-later", path), .success("Summary"))
        XCTAssertEqual(calls, 21)
    }

    func testAnErrorIsNeverRetried() throws {
        let path = try file([user("hello")])
        var calls = 0
        let summaries = Summaries(transport: { _, done in
            calls += 1
            done(Data(), 529, nil)
        }, key: { "k" })
        XCTAssertEqual(wait(summaries, "s1", path), .failure(.busy))
        XCTAssertEqual(calls, 1)
    }

    /// Off until turned on, under its own name: a personal build's "summariesEnabled" never turns it on.
    func testItIsOffByDefaultAndAnOldSettingDoesntTurnItOn() throws {
        XCTAssertFalse(Config().summaryOptIn)
        let old = try JSONDecoder().decode(Config.self, from: Data(#"{"summariesEnabled": true, "musicEnabled": true}"#.utf8))
        XCTAssertFalse(old.summaryOptIn)
        let on = try JSONDecoder().decode(Config.self, from: Data(#"{"summaryOptIn": true}"#.utf8))
        XCTAssertTrue(on.summaryOptIn)
    }

    /// Only a test copy, and only to this computer, can be pointed at a stand-in for Anthropic.
    func testOnlyATestCopyTalksToAStandIn() {
        let local = ["SMOLCORP_TEST_ANTHROPIC_URL": "http://127.0.0.1:7781/v1/messages"]
        XCTAssertEqual(SummaryRequest.testEndpoint(local, testCopy: true)?.absoluteString, "http://127.0.0.1:7781/v1/messages")
        XCTAssertNil(SummaryRequest.testEndpoint(local, testCopy: false), "the app in use always talks to Anthropic")
        for elsewhere in ["https://example.com/v1/messages", "http://192.168.1.4/v1/messages", "not a url"] {
            XCTAssertNil(SummaryRequest.testEndpoint(["SMOLCORP_TEST_ANTHROPIC_URL": elsewhere], testCopy: true), elsewhere)
        }
        XCTAssertEqual(SummaryRequest.endpoint.absoluteString, "https://api.anthropic.com/v1/messages")
    }
}

extension Result where Failure == SummaryError {
    var failureMessage: String? { if case let .failure(error) = self { return error.message }; return nil }
}
