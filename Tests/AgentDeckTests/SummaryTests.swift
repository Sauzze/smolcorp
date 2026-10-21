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

    private func user(_ text: String, meta: Bool = false) -> String {
        var entry: [String: Any] = ["type": "user", "message": ["role": "user", "content": text]]
        if meta { entry["isMeta"] = true }
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
        USER: Fix the login bug please
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
        XCTAssertTrue(extract.text.hasPrefix("USER: Migrate the database to Postgres\n…"))
        XCTAssertTrue(extract.text.hasSuffix("Step 400: moved one more table across, checked it, carried on."))
        XCTAssertFalse(extract.text.contains("Step 1:"))
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
        XCTAssertEqual(TranscriptTail.read(path)?.text, "USER: Hello there, can you help\nCLAUDE: Sure.")
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
        USER: [redacted]
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
        XCTAssertTrue(text.hasPrefix("USER: [redacted]"))
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
