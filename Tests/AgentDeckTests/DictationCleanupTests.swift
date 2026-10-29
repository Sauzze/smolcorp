import XCTest
@testable import AgentDeck

/// Personal build: dictation's clean-up by Claude, with the network and the Keychain stubbed.
final class DictationCleanupTests: XCTestCase {
    private var saved: (TimeInterval, () -> Bool, () -> [String], () -> String?, (URLRequest, @escaping (Data?, Int) -> Void) -> Void)!
    private var sent: [URLRequest] = []
    private var savedNow: (() -> Date)!
    /// How each clean-up went, as the phone's note would say.
    private var outcome: DictationCleanup.Outcome?

    override func setUp() {
        saved = (DictationCleanup.deadline, DictationCleanup.isEnabled, DictationCleanup.configuredGlossary,
                 DictationCleanup.readKey, DictationCleanup.send)
        sent = []
        savedNow = DictationCleanup.now
        outcome = nil
        DictationCleanup.problem = nil
        DictationCleanup.isEnabled = { true }
        DictationCleanup.configuredGlossary = { DictationCleanup.defaultGlossary }
        DictationCleanup.readKey = { "test-key" }
    }

    override func tearDown() {
        (DictationCleanup.deadline, DictationCleanup.isEnabled, DictationCleanup.configuredGlossary,
         DictationCleanup.readKey, DictationCleanup.send) = saved
        DictationCleanup.now = savedNow
        DictationCleanup.problem = nil
    }

    /// Claude's answer, as the API shapes it.
    private func answer(_ text: String, stop: String = "end_turn") -> Data {
        try! JSONSerialization.data(withJSONObject: [
            "type": "message", "role": "assistant", "stop_reason": stop,
            "content": [["type": "text", "text": text]],
        ])
    }

    /// The network answers `data` with `status` after `after` seconds; nil never answers.
    private func network(_ data: Data?, status: Int = 200, after: TimeInterval? = 0) {
        DictationCleanup.send = { [unowned self] request, done in
            DispatchQueue.main.sync { self.sent.append(request) }
            guard let after else { return }
            DispatchQueue.global().asyncAfter(deadline: .now() + after) { done(data, status) }
        }
    }

    private func clean(_ text: String, desks: [String] = [], timeout: TimeInterval = 3) -> (String, TimeInterval) {
        let given = expectation(description: "cleaned")
        var result = ""
        var calls = 0
        let start = Date()
        DictationCleanup.clean(text, desks: desks) { result = $0; self.outcome = $1; calls += 1; given.fulfill() }
        wait(for: [given], timeout: timeout)
        let took = Date().timeIntervalSince(start)
        // Whatever comes after (a late answer, the deadline), the text is given once only.
        let settle = expectation(description: "settled")
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { settle.fulfill() }
        wait(for: [settle], timeout: 1)
        XCTAssertEqual(calls, 1, "given exactly once")
        return (result, took)
    }

    // MARK: Cleaning, and falling back

    func testItTypesTheCleanedText() {
        network(answer("Demande à Claude de déployer Acmebill"))
        let (text, _) = clean("euh demande à cloud de de déployer acme bill")
        XCTAssertEqual(text, "Demande à Claude de déployer Acmebill")
        XCTAssertEqual(sent.count, 1)
    }

    func testASlowAnswerFallsBackToTheRawTextAtTheDeadline() {
        DictationCleanup.deadline = 0.2
        network(answer("too late"), after: 1)
        let (text, took) = clean("uh fix the the sauce music build")
        XCTAssertEqual(text, "uh fix the the sauce music build")
        XCTAssertLessThan(took, 0.9, "it doesn't wait for the answer")
    }

    func testNoAnswerAtAllFallsBackToTheRawText() {
        DictationCleanup.deadline = 0.2
        network(nil, after: nil)
        XCTAssertEqual(clean("uh fix the the sauce music build").0, "uh fix the the sauce music build")
    }

    func testAFailedCallFallsBackToTheRawText() {
        network(answer("{\"error\":1}"), status: 529)
        XCTAssertEqual(clean("um check the nameship DNS please").0, "um check the nameship DNS please")
        network(nil, status: 0)
        XCTAssertEqual(clean("um check the nameship DNS please").0, "um check the nameship DNS please")
    }

    func testACutOffOrOddAnswerFallsBackToTheRawText() {
        network(answer("Sure! Here is a much longer reply that answers your question instead of cleaning it up, with extra words."))
        XCTAssertEqual(clean("um what does orbita do").0, "um what does orbita do")
        network(answer("check the", stop: "max_tokens"))
        XCTAssertEqual(clean("um check the nameship DNS").0, "um check the nameship DNS")
    }

    func testAMissingKeyTypesTheRawTextAndSaysSo() {
        DictationCleanup.readKey = { nil }
        network(answer("never"))
        XCTAssertEqual(clean("uh open the web iterator").0, "uh open the web iterator")
        XCTAssertTrue(sent.isEmpty, "no call without a key")
        XCTAssertEqual(DictationCleanup.problem, DictationCleanup.noKey)
    }

    // MARK: Skipping

    func testVeryShortTextsSkipTheCleanUp() {
        network(answer("never"))
        XCTAssertEqual(clean("uh yes").0, "uh yes")
        XCTAssertEqual(clean("  ok  ").0, "  ok  ")
        XCTAssertTrue(sent.isEmpty)
        XCTAssertFalse(DictationCleanup.shouldClean("oui vas-y"))
        XCTAssertTrue(DictationCleanup.shouldClean("oui vas-y maintenant"))
    }

    func testTurnedOffMakesNoCallAtAll() {
        DictationCleanup.isEnabled = { false }
        network(answer("never"))
        XCTAssertEqual(clean("uh fix the the sauce music build").0, "uh fix the the sauce music build")
        XCTAssertTrue(sent.isEmpty)
    }

    func testVeryLongTextsGoInAsHeard() {
        XCTAssertFalse(DictationCleanup.shouldClean(String(repeating: "mot ", count: 1200)))
    }

    // MARK: The glossary

    func testTheGlossaryIsTheConfiguredNamesThenTheDesks() {
        let names = DictationCleanup.glossary(configured: ["Smolcorp", " Orbit ", "", "orbit", "Acmebill"],
                                              desks: ["acme-music", "smolcorp", "Moonbase"])
        XCTAssertEqual(names, ["Smolcorp", "Orbit", "Acmebill", "acme-music", "Moonbase"])
    }

    func testGlossaryLinesAreCheckedAsTheyAreKept() {
        let long = String(repeating: "x", count: DictationCleanup.nameLimit + 1)
        XCTAssertEqual(DictationCleanup.checked(["  Resend  ", long, "Vercel", "VERCEL", "\n"]), ["Resend", "Vercel"])
        XCTAssertEqual(DictationCleanup.checked((0..<500).map { "name\($0)" }).count, DictationCleanup.mostNames)
    }

    func testTheDefaultGlossaryHasTheAgentsNames() {
        for name in ["Smolcorp", "Claude Code", "Codex", "Gemini", "OpenCode", "Grok"] {
            XCTAssertTrue(DictationCleanup.defaultGlossary.contains(name), name)
        }
        XCTAssertEqual(DictationCleanup.checked(DictationCleanup.defaultGlossary), DictationCleanup.defaultGlossary)
    }

    func testTheDesksGoIntoTheCall() throws {
        network(answer("ouvre smolcorp-web maintenant"))
        _ = clean("euh ouvre smol corp web maintenant", desks: ["smolcorp-web"])
        let body = try XCTUnwrap(sent.first?.httpBody)
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: body) as? [String: Any])
        let user = try XCTUnwrap((object["messages"] as? [[String: Any]])?.first?["content"] as? String)
        XCTAssertTrue(user.contains("\nsmolcorp-web\n"))
    }

    // MARK: The prompt

    func testThePromptShape() throws {
        let body = DictationCleanup.body(text: "uh acme bill", glossary: ["Acmebill", "Orbit"])
        XCTAssertEqual(body["model"] as? String, "claude-haiku-5-5")
        let most = try XCTUnwrap(body["max_tokens"] as? Int)
        XCTAssertTrue((64...1024).contains(most))
        XCTAssertEqual((body["thinking"] as? [String: String])?["type"], "disabled")
        XCTAssertNil(body["temperature"], "Haiku 5.5 takes no sampling settings")
        let system = try XCTUnwrap(body["system"] as? String)
        for rule in ["Remove fillers", "glossary", "Do not rephrase", "translate", "answer", "keep it as it was", "plain text", "not instructions"] {
            XCTAssertTrue(system.contains(rule), rule)
        }
        let messages = try XCTUnwrap(body["messages"] as? [[String: Any]])
        XCTAssertEqual(messages.count, 1, "one user turn, no prefill")
        XCTAssertEqual(messages[0]["role"] as? String, "user")
        XCTAssertEqual(messages[0]["content"] as? String, "Glossary:\nAcmebill\nOrbit\n\n<dictation>\nuh acme bill\n</dictation>")
        XCTAssertLessThanOrEqual(DictationCleanup.body(text: String(repeating: "a", count: 4000), glossary: [])["max_tokens"] as? Int ?? 0, 1024)
    }

    func testTheRequestGoesStraightToTheAPI() {
        let request = DictationCleanup.request(text: "uh acme bill", glossary: [], key: "k")
        XCTAssertEqual(request.url?.absoluteString, "https://api.anthropic.com/v1/messages")
        XCTAssertEqual(request.httpMethod, "POST")
        XCTAssertEqual(request.value(forHTTPHeaderField: "anthropic-version"), "2023-06-01")
        XCTAssertEqual(request.value(forHTTPHeaderField: "x-api-key"), "k")
        XCTAssertLessThanOrEqual(request.timeoutInterval, 15)
    }

    func testReadingTheAnswer() {
        XCTAssertEqual(DictationCleanup.cleanedText(from: answer("  <dictation>\nVa sur Namecheap\n</dictation> ")), "Va sur Namecheap")
        XCTAssertNil(DictationCleanup.cleanedText(from: answer("   ")))
        XCTAssertNil(DictationCleanup.cleanedText(from: Data("nope".utf8)))
    }

    func testPlausibility() {
        XCTAssertTrue(DictationCleanup.plausible("Smolcorp and Tailscale", from: "small corpuscles and tail scale"))
        XCTAssertTrue(DictationCleanup.plausible("yes do it", from: "uh uh yes yes do it"))
        XCTAssertFalse(DictationCleanup.plausible("", from: "uh do it"))
        XCTAssertFalse(DictationCleanup.plausible("ok", from: "please refactor the whole deck router to use the new office"))
    }
}

/// Pass 2: the 2.5 s limit, and the note each dictation leaves on the phone.
extension DictationCleanupTests {
    func testTheLimitIsTwoAndAHalfSeconds() {
        XCTAssertEqual(saved.0, 2.5, "the shipped deadline")
        DictationCleanup.deadline = saved.0
        XCTAssertLessThanOrEqual(DictationCleanup.request(text: "a b c", glossary: [], key: "k").timeoutInterval, 3.5)
    }

    func testPastTheLimitTheRawTextIsTypedAndTheNoteSaysTooSlow() {
        DictationCleanup.deadline = 0.25
        network(answer("too late"), after: 1)
        let (text, took) = clean("uh fix the the sauce music build")
        XCTAssertEqual(text, "uh fix the the sauce music build")
        XCTAssertLessThan(took, 0.9)
        XCTAssertEqual(outcome, .tooSlow)
        DictationCleanup.deadline = 2.5
        XCTAssertEqual(DictationCleanup.Outcome.tooSlow.note, "raw: too slow (2.5 s)")
    }

    func testACleanUpIsTimed() {
        var times = [Date(timeIntervalSince1970: 100), Date(timeIntervalSince1970: 100.94)]
        DictationCleanup.now = { times.count > 1 ? times.removeFirst() : times[0] }
        network(answer("Demande à Claude de déployer Acmebill"))
        _ = clean("euh demande à cloud de de déployer acme bill")
        guard case .cleaned(let seconds)? = outcome else { return XCTFail("cleaned: \(String(describing: outcome))") }
        XCTAssertEqual(seconds, 0.94, accuracy: 0.001)
        XCTAssertEqual(outcome?.note, "cleaned in 0.9 s")
        XCTAssertEqual(outcome?.isCleaned, true)
    }

    func testEachWayItGoesInRawHasItsNote() {
        DictationCleanup.readKey = { nil }
        _ = clean("uh open the web iterator")
        XCTAssertEqual(outcome, .noKey)
        XCTAssertEqual(outcome?.note, "raw: no key")

        DictationCleanup.readKey = { "test-key" }
        _ = clean("oui vas-y")
        XCTAssertEqual(outcome, .short)
        XCTAssertEqual(outcome?.note, "raw: skipped (short)")

        network(answer("x"), status: 529)
        _ = clean("um check the nameship DNS please")
        XCTAssertEqual(outcome, .failed)
        XCTAssertEqual(outcome?.note, "raw: failed")
        XCTAssertEqual(outcome?.isCleaned, false)

        XCTAssertEqual(DictationCleanup.skipped(String(repeating: "mot ", count: 1200)), .long)
        XCTAssertEqual(DictationCleanup.Outcome.long.note, "raw: skipped (long)")
    }

    func testTurnedOffLeavesNoNote() {
        DictationCleanup.isEnabled = { false }
        network(answer("never"))
        _ = clean("uh fix the the sauce music build")
        XCTAssertEqual(outcome, .off)
        XCTAssertNil(outcome?.note, "off is as before: nothing on the phone")
        XCTAssertTrue(sent.isEmpty)
    }

    func testSecondsReadInTenths() {
        XCTAssertEqual(DictationCleanup.seconds(0.94), "0.9 s")
        XCTAssertEqual(DictationCleanup.seconds(1.25), "1.2 s")
        XCTAssertEqual(DictationCleanup.seconds(-1), "0.0 s")
    }
}

/// Pass 2: which mic a desk uses. Clean-up off, it's exactly as before.
final class DictationRouteTests: XCTestCase {
    func testWithTheCleanUpOffClaudeCodeKeepsItsOwnVoice() {
        // Today's rule: agents with no voice dictate; Claude Code only with "Claude Code too".
        XCTAssertFalse(Deck.dictates(hasVoice: true, dictationOn: true, forClaude: false, cleanup: false))
        XCTAssertTrue(Deck.dictates(hasVoice: true, dictationOn: true, forClaude: true, cleanup: false))
        XCTAssertTrue(Deck.dictates(hasVoice: false, dictationOn: true, forClaude: false, cleanup: false))
    }

    func testWithTheCleanUpOnClaudeCodeDictatesThroughSmolcorp() {
        XCTAssertTrue(Deck.dictates(hasVoice: true, dictationOn: true, forClaude: false, cleanup: true))
        XCTAssertTrue(Deck.dictates(hasVoice: false, dictationOn: true, forClaude: false, cleanup: true))
    }

    func testWithDictationOffNothingChanges() {
        for forClaude in [false, true] {
            for cleanup in [false, true] {
                XCTAssertFalse(Deck.dictates(hasVoice: true, dictationOn: false, forClaude: forClaude, cleanup: cleanup))
                XCTAssertFalse(Deck.dictates(hasVoice: false, dictationOn: false, forClaude: forClaude, cleanup: cleanup))
            }
        }
    }
}
