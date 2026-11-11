import XCTest
@testable import AgentDeck

/// Phone microphone (personal build): who a request is for behind Tailscale's
/// HTTPS, the text the phone sends, and the tailnet address setup shows. Offline.
final class PhoneMicTests: XCTestCase {
    // MARK: Addressing

    func testTheHostLosesItsPortAndCase() {
        XCTAssertEqual(Addressing.host("My-Mac.Tail1234.TS.NET:443"), "my-mac.tail1234.ts.net")
        XCTAssertEqual(Addressing.host("192.168.1.4:7710"), "192.168.1.4")
        XCTAssertEqual(Addressing.host(nil), "")
    }

    func testLanAddressesAreDirectAsBefore() {
        XCTAssertTrue(Addressing.isDirectAddress("192.168.1.4"))
        XCTAssertTrue(Addressing.isDirectAddress("100.101.102.103")) // a tailnet IP, over plain http
        XCTAssertTrue(Addressing.isDirectAddress("my-mac.local"))
        XCTAssertTrue(Addressing.isDirectAddress("localhost"))
        XCTAssertFalse(Addressing.isDirectAddress("my-mac.tail1234.ts.net"))
        XCTAssertFalse(Addressing.isDirectAddress("evil.example.com"))
    }

    func testATailnetNameCountsOnlyFromThisMacsLoopback() {
        XCTAssertTrue(Addressing.isTailnet("my-mac.tail1234.ts.net", fromLoopback: true))
        XCTAssertTrue(Addressing.isTailnet("mac.cat-crocodile.ts.net", fromLoopback: true))
        // Straight from the network, as only `tailscale serve` on this Mac should bring it.
        XCTAssertFalse(Addressing.isTailnet("my-mac.tail1234.ts.net", fromLoopback: false))
    }

    func testLookalikesAreNotTailnetNames() {
        for host in ["ts.net", "tail1234.ts.net", "a.b.c.ts.net", "my-mac.tail1234.ts.net.evil.com", "evilts.net",
                     "my-mac.tail1234.ts.network", "-bad.tail1234.ts.net", "my_mac.tail1234.ts.net"] {
            XCTAssertFalse(Addressing.isTailnet(host, fromLoopback: true), host)
        }
    }

    func testAProxiedRequestIsSeenAsOne() {
        XCTAssertTrue(Addressing.isProxied(["x-forwarded-for": "100.64.0.2"]))
        XCTAssertTrue(Addressing.isProxied(["x-forwarded-host": "my-mac.tail1234.ts.net"]))
        XCTAssertTrue(Addressing.isProxied(["tailscale-user-login": "me@example.com"]))
        XCTAssertFalse(Addressing.isProxied(["host": "localhost:7710", "user-agent": "Safari"]))
    }

    // MARK: The text

    func testTheTextIsFlattenedToOneLine() {
        XCTAssertEqual(PhoneDictation.prepared("  fix the\nlogin\tbug \r\n "), "fix the login bug")
        XCTAssertEqual(PhoneDictation.prepared("café, déjà vu"), "café, déjà vu")
        // Unicode's own line and paragraph separators, and the next-line character, break no line either.
        XCTAssertEqual(PhoneDictation.prepared("fix\u{2028}the\u{2029}login\u{85}bug"), "fix the login bug")
        XCTAssertEqual(PhoneDictation.prepared("go\u{1b}[A\u{0b}on\u{0c}"), "go [A on")
    }

    func testNothingOrTooMuchIsRefused() {
        XCTAssertNil(PhoneDictation.prepared(""))
        XCTAssertNil(PhoneDictation.prepared(" \n\t "))
        XCTAssertNil(PhoneDictation.prepared(String(repeating: "a", count: PhoneDictation.longest + 1)))
        XCTAssertNotNil(PhoneDictation.prepared(String(repeating: "a", count: PhoneDictation.longest)))
    }

    /// Only control characters go: the invisible joiner that makes 👩‍💻 one
    /// emoji is a format character, kept, as are accents and French quotes.
    func testEmojiAndAccentsArriveWhole() {
        let french = "Café à l’été, ça va ? « Œuvre » é è à ç ô œ 🎉👍🏽 👩‍💻 ❤️ 👨‍👩‍👧"
        XCTAssertEqual(PhoneDictation.prepared(french), french)
        XCTAssertEqual(PhoneDictation.prepared("a\u{200D}b\u{00AD}c"), "a\u{200D}b\u{00AD}c")
        XCTAssertEqual(PhoneDictation.prepared(#"He said "hi" $HOME `x` $(y) ; a && b \"#), #"He said "hi" $HOME `x` $(y) ; a && b \"#)
    }

    /// Too long is said as such: "Nothing to type" for 2,001 characters sent the owner looking for nothing.
    func testTooLongSaysSo() {
        XCTAssertEqual(PhoneDictation.refusal(String(repeating: "a", count: PhoneDictation.longest + 1)), "Too long: at most 2000 characters at a time.")
        XCTAssertEqual(PhoneDictation.refusal(" \n "), "Nothing to type")
        XCTAssertEqual(PhoneDictation.refusal(""), "Nothing to type")
        // One character can carry any number of combining marks: the bytes are capped too.
        let heavy = "e" + String(repeating: "\u{301}", count: 20_000)
        XCTAssertEqual(heavy.count, 1)
        XCTAssertNil(PhoneDictation.prepared(heavy))
        XCTAssertEqual(PhoneDictation.refusal(heavy), PhoneDictation.tooLong)
        // 2,000 emoji of several pieces still go.
        XCTAssertNotNil(PhoneDictation.prepared(String(repeating: "👍🏽", count: PhoneDictation.longest)))
    }

    /// Typed into Claude Code (or Codex, Gemini CLI, OpenCode), "!" first switches it
    /// to its shell, and Send would run the rest there: refused before any key goes.
    func testTextStartingWithABangIsRefused() throws {
        XCTAssertTrue(PhoneDictation.runsInShell("!rm -rf build"))
        XCTAssertFalse(PhoneDictation.runsInShell("ship it!"))
        XCTAssertFalse(PhoneDictation.runsInShell("¡hola!"))
        // "!" with an accent, a joiner or an emoji's variation selector on it is one character to
        // Swift, but Claude Code looks at the first code point and still goes into its shell.
        for bang in ["!\u{301}x", "!\u{200D}x", "!\u{FE0F}x"] {
            XCTAssertFalse(bang.hasPrefix("!"), "what let it through")
            XCTAssertTrue(PhoneDictation.runsInShell(bang), bang.unicodeScalars.map { String($0.value, radix: 16) }.joined(separator: " "))
            XCTAssertTrue(PhoneDictation.runsInShell(try XCTUnwrap(PhoneDictation.prepared("  " + bang))), "trimmed first")
        }
        XCTAssertFalse(PhoneDictation.runsInShell("！x"), "a full-width one is text")
        let body = try Self.body(of: "func typePhoneDictation(", until: "static let answerFirst")
        func at(_ text: String) throws -> String.Index { try XCTUnwrap(body.range(of: text)?.lowerBound, text) }
        XCTAssertLessThan(try at("guard let words = PhoneDictation.prepared(text) else { return done(.status(400, PhoneDictation.refusal(text))) }"),
                          try at("guard !PhoneDictation.runsInShell(words) else { return done(.status(400, Deck.noBangPrompt)) }"),
                          "checked on the text as it would be typed: spaces in front are trimmed first")
        XCTAssertLessThan(try at("guard !PhoneDictation.runsInShell(words)"), try at("driver.insert(words, in: tab)"))
    }

    // MARK: On a deck of its own, with a stand-in terminal

    /// A deck that restores, saves and tells nothing, its desk "s1" a Claude Code session in
    /// a terminal that records what it's given.
    private func deckWithDesk(agent: Agent = .claude) -> (Deck, RecordingTerminal) {
        let deck = Deck(quiet: true)
        deck.sessions.apply(["hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/work/api"],
                            agent: agent, tab: "tmux:%3@/tmp/standin-socket", startedIn: "", at: 10)
        deck.sessions.apply(["hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"],
                            agent: agent, tab: "tmux:%3@/tmp/standin-socket", startedIn: "", at: 20)
        let terminal = RecordingTerminal()
        Terminals.standIn = terminal
        addTeardownBlock { Terminals.standIn = nil }
        return (deck, terminal)
    }

    private func asks(_ deck: Deck, at time: Double) {
        deck.sessions.apply(["hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/api", "tool_name": "Bash",
                             "tool_input": ["command": "touch x"]], agent: .claude, tab: "tmux:%3@/tmp/standin-socket", startedIn: "", at: time)
    }

    private func dictate(_ deck: Deck, _ text: String, send: Bool = false) -> Response {
        let answered = expectation(description: "answered")
        var response: Response?
        deck.typePhoneDictation(text, into: "s1", send: send) {
            response = $0
            answered.fulfill()
        }
        wait(for: [answered], timeout: 10)
        return response!
    }

    func testTheTextGoesToAFreeDesk() {
        guard Config.shared.phoneMicEnabled else { return XCTFail("the phone mic is on by default") }
        let (deck, terminal) = deckWithDesk()
        XCTAssertEqual(dictate(deck, "Café, ça va ? 👩‍💻 fin;").status, 204)
        XCTAssertEqual(terminal.inserted, ["Café, ça va ? 👩‍💻 fin;"])
        XCTAssertEqual(dictate(deck, "!\u{301};touch canary").status, 400, "never typed")
        XCTAssertEqual(terminal.inserted.count, 1)
    }

    /// The desk free when the request came, asked something while it waited for the terminal (I1
    /// measured waits of minutes behind Terminal.app): checked again right before the paste, so
    /// the words never reach the menu.
    func testADeskThatStartedAskingMeanwhileGetsNothing() {
        guard Config.shared.phoneMicEnabled else { return XCTFail("the phone mic is on by default") }
        let (deck, terminal) = deckWithDesk()
        // Between the check on the request and the paste: as the terminal is asked where the agent is.
        terminal.whenLookedUp = { DispatchQueue.main.sync { self.asks(deck, at: 30) } }
        let response = dictate(deck, "fais un test court", send: true)
        XCTAssertEqual(response.status, 409)
        XCTAssertEqual(String(decoding: response.body, as: UTF8.self), Deck.answerFirst)
        XCTAssertEqual(terminal.inserted, [], "nothing pasted")
        XCTAssertEqual(terminal.pressed, [], "no Return")
        XCTAssertEqual(deck.sessions["s1"]?.state, .asking)
    }

    /// ✗ or Esc from the deck on a Claude Code prompt: Claude ends the turn and waits for a
    /// prompt, and no hook says so. The deck frees the desk once the key has gone, so the
    /// phone may type what to do instead; for another agent, nothing is assumed.
    func testRejectingFromTheDeckFreesAClaudeDesk() {
        for agent in [Agent.claude, .codex] {
            let (deck, terminal) = deckWithDesk(agent: agent)
            deck.sessions.apply(["hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/api", "tool_name": "Bash"],
                                agent: agent, tab: "tmux:%3@/tmp/standin-socket", startedIn: "", at: 30)
            XCTAssertEqual(deck.sessions["s1"]?.state, .asking)
            let answered = expectation(description: "answered")
            deck.perform("reject", on: "s1", query: [:]) {
                XCTAssertEqual($0.status, 204)
                answered.fulfill()
            }
            wait(for: [answered], timeout: 10)
            XCTAssertEqual(terminal.pressed.count, 1, "\(agent)")
            XCTAssertEqual(deck.sessions["s1"]?.state, agent == .claude ? .idle : .asking, "\(agent)")
        }
        // A key that didn't go leaves it asking.
        let (deck, terminal) = deckWithDesk()
        asks(deck, at: 30)
        terminal.takesKeys = false
        let answered = expectation(description: "answered")
        deck.perform("esc", on: "s1", query: [:]) {
            XCTAssertGreaterThanOrEqual($0.status, 400)
            answered.fulfill()
        }
        wait(for: [answered], timeout: 10)
        XCTAssertEqual(deck.sessions["s1"]?.state, .asking)
    }

    func testItSendsFromThreeWordsWithTheSettingOn() {
        XCTAssertFalse(PhoneDictation.sends("yes", autoSend: true))
        XCTAssertFalse(PhoneDictation.sends("yes please", autoSend: true))
        XCTAssertTrue(PhoneDictation.sends("yes please go", autoSend: true))
        XCTAssertFalse(PhoneDictation.sends("yes please go", autoSend: false))
    }

    /// Return goes on its own, after the text has settled in, as the ⏎ key
    /// presses it: sent with the text, Claude Code took both for a paste and
    /// Return for a new line in it.
    func testReturnGoesAloneAfterTheTextSettles() throws {
        let body = try Self.body(of: "func typePhoneDictation(", until: "static let answerFirst")
        XCTAssertFalse(body.contains("driver.type("), "text and Return in one write")
        // Return only under `if sends`: Type only never presses it.
        XCTAssertEqual(body.components(separatedBy: "driver.press(").count, 2, "one press")
        let block = try XCTUnwrap(Self.block(after: "if sends {", in: body))
        XCTAssertTrue(block.contains("driver.press(.init(key: .returnKey), in: tab)"))
        XCTAssertTrue(block.contains("Thread.sleep(forTimeInterval: PhoneDictation.settle)"))
        func at(_ text: String) throws -> String.Index { try XCTUnwrap(body.range(of: text)?.lowerBound, text) }
        XCTAssertLessThan(try at("driver.insert(words, in: tab)"), try at("Thread.sleep(forTimeInterval: PhoneDictation.settle)"))
        XCTAssertLessThan(try at("Thread.sleep(forTimeInterval: PhoneDictation.settle)"), try at("driver.press(.init(key: .returnKey), in: tab)"))
        XCTAssertGreaterThanOrEqual(PhoneDictation.settle, 0.15)
        XCTAssertLessThanOrEqual(PhoneDictation.settle, 0.3)
    }

    // MARK: An agent asking you something

    /// While an agent asks, its menu would take the words (a digit picks an
    /// option) and Return the option highlighted: nothing goes, Send or Type
    /// only, and a Return already on its way is held back.
    func testNothingGoesToAnAgentAskingYou() throws {
        XCTAssertTrue(Deck.isAsking(.asking))
        for state in [SessionState.idle, .thinking, .unread, .error] { XCTAssertFalse(Deck.isAsking(state), state.rawValue) }
        let now = Date()
        XCTAssertTrue(Deck.mayPressReturn(state: .thinking, last: nil, now: now))
        XCTAssertTrue(Deck.mayPressReturn(state: .idle, last: now.addingTimeInterval(-Deck.returnRepeat), now: now))
        XCTAssertFalse(Deck.mayPressReturn(state: .asking, last: nil, now: now), "began asking during the settle")
        XCTAssertFalse(Deck.mayPressReturn(state: .idle, last: now.addingTimeInterval(-0.2), now: now), "a ⏎ just went")
        XCTAssertFalse(Deck.mayPressReturn(state: nil, last: nil, now: now), "the session has gone")

        let body = try Self.body(of: "func typePhoneDictation(", until: "static let answerFirst")
        func at(_ text: String) throws -> String.Index { try XCTUnwrap(body.range(of: text)?.lowerBound, text) }
        let guardAsking = try at("guard !Deck.isAsking(session.state) else { return done(.status(409, Deck.answerFirst)) }")
        XCTAssertLessThan(guardAsking, try at("driver.insert(words, in: tab)"), "checked before any key")
        XCTAssertLessThan(guardAsking, try at("toAgent(session"))
        // Again right before Return, on the main queue, where the state and the last Return live.
        let settle = try at("Thread.sleep(forTimeInterval: PhoneDictation.settle)")
        let claim = try at("guard DispatchQueue.main.sync(execute: { self.claimReturn(id) }) else {")
        let press = try at("driver.press(.init(key: .returnKey), in: tab)")
        XCTAssertLessThan(settle, claim)
        XCTAssertLessThan(claim, press)
        let claimer = try Self.body(of: "private func claimReturn(", until: "func peek(")
        XCTAssertTrue(claimer.contains("guard Deck.mayPressReturn(state: sessions[id]?.state, last: lastReturn[id], now: now) else { return false }"))
        XCTAssertTrue(claimer.contains("lastReturn[id] = now"))
    }

    /// The page holds the box's Send ⏎ and Type only while that agent asks, and says why.
    func testTheBoxWaitsWhileTheAgentAsks() throws {
        let page = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/deck.html"), encoding: .utf8)
        let check = try XCTUnwrap(page.range(of: "function confirmable() {"))
        let rule = page[check.upperBound...].prefix(400)
        XCTAssertTrue(rule.contains("const asking = !!s && s.state === 'asking';"))
        XCTAssertTrue(rule.contains("$('confirm-send').disabled = asking;"))
        XCTAssertTrue(rule.contains("$('confirm-type').disabled = asking;"))
        XCTAssertTrue(rule.contains("$('confirm-why').hidden = !asking;"))
        XCTAssertTrue(page.contains("if (!session || $('confirm-send').disabled) return;"), "a tap on a held button sends nothing")
        // As the computer's word on the state comes in, and when the box opens.
        let receive = try XCTUnwrap(page.range(of: "function receive(data) {"))
        XCTAssertTrue(page[receive.upperBound...].prefix(1800).contains("confirmable();"))
        XCTAssertTrue(page.contains("$('confirm').hidden = false;\n    confirmable();"))
        XCTAssertTrue(page.contains("It's asking you something: answer it on the deck first."))
    }

    // MARK: The peek

    func testAPeekAtMostOnceASecond() throws {
        let now = Date()
        XCTAssertFalse(PhoneDictation.peekTooSoon(last: nil, now: now))
        XCTAssertTrue(PhoneDictation.peekTooSoon(last: now.addingTimeInterval(-0.5), now: now))
        XCTAssertFalse(PhoneDictation.peekTooSoon(last: now.addingTimeInterval(-PhoneDictation.peekGap), now: now))
        let body = try Self.body(of: "func peek(_ id: String", until: "static func dictates(_ agent")
        let limit = try XCTUnwrap(body.range(of: "guard !PhoneDictation.peekTooSoon(last: lastPeek[id], now: now) else { return done(.status(429,"))
        XCTAssertLessThan(limit.lowerBound, try XCTUnwrap(body.range(of: "driver.screen(of: address)")).lowerBound)
    }

    func testThePeekIsTheLastLinesAsText() {
        let screen = (1...30).map { "line \($0)" }.joined(separator: "\n") + "\n\n   \n"
        XCTAssertEqual(PhoneDictation.peek(screen), (21...30).map { "line \($0)" })
        XCTAssertEqual(PhoneDictation.peek("> fix the login bug\u{1b}[0m\u{7}"), ["> fix the login bug[0m"])
        XCTAssertEqual(PhoneDictation.peek(String(repeating: "─", count: 500)).first?.count, PhoneDictation.peekWidth)
        XCTAssertEqual(PhoneDictation.peek(""), [])
        XCTAssertEqual(PhoneDictation.peek("\n\n"), [])
        XCTAssertEqual(PhoneDictation.peek(String(repeating: "\n", count: 12) + "──\n❯ hi\n──\n"), ["──", "❯ hi", "──"])
    }

    static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    /// Deck.swift from `start` up to `end`.
    static func body(of start: String, until end: String) throws -> String {
        let deck = try String(contentsOf: repo.appendingPathComponent("Sources/AgentDeck/Deck.swift"), encoding: .utf8)
        let from = try XCTUnwrap(deck.range(of: start), start)
        let to = try XCTUnwrap(deck.range(of: end, range: from.upperBound..<deck.endIndex), end)
        return String(deck[from.upperBound..<to.lowerBound])
    }

    /// What's inside the braces `opening` opens, matched brace for brace.
    static func block(after opening: String, in text: String) -> String? {
        guard let start = text.range(of: opening) else { return nil }
        var depth = 1
        var index = start.upperBound
        while index < text.endIndex {
            switch text[index] {
            case "{": depth += 1
            case "}":
                depth -= 1
                if depth == 0 { return String(text[start.upperBound..<index]) }
            default: break
            }
            index = text.index(after: index)
        }
        return nil
    }

    // MARK: The tailnet address

    func testTheAddressComesFromTailscalesStatus() {
        let status = Data(#"{"BackendState":"Running","Self":{"DNSName":"My-Mac.tail1234.ts.net.","HostName":"My-Mac"}}"#.utf8)
        XCTAssertEqual(Tailnet.url(fromStatus: status), "https://my-mac.tail1234.ts.net")
    }

    func testTailscaleRunsWithATerm() {
        // Opened from Finder, the app has no TERM, and the Mac app's Tailscale
        // then starts its window instead of answering.
        let fromFinder = Tailnet.environment(["HOME": "/Users/me", "PATH": "/usr/bin:/bin"])
        XCTAssertEqual(fromFinder["TERM"], "dumb")
        XCTAssertEqual(fromFinder["HOME"], "/Users/me")
        XCTAssertEqual(fromFinder["PATH"], "/usr/bin:/bin")
        XCTAssertEqual(Tailnet.environment(["TERM": ""])["TERM"], "dumb")
        // From a terminal, its own TERM is kept.
        XCTAssertEqual(Tailnet.environment(["TERM": "xterm-256color"])["TERM"], "xterm-256color")
    }

    /// The setup check waits for Tailscale's first answer, so it's right on its first run.
    func testTheFirstSetupCheckWaitsForTailscale() {
        let real = Tailnet.ask
        defer { Tailnet.ask = real; Tailnet.forget() }
        Tailnet.forget()
        var asked = 0
        Tailnet.ask = {
            asked += 1
            Thread.sleep(forTimeInterval: 0.1)
            return ("https://my-mac.tail1234.ts.net", true)
        }
        var changed = 0
        let first = expectation(description: "answered")
        Tailnet.whenFresh(within: 2, changed: { changed += 1 }) {
            XCTAssertEqual(Tailnet.url, "https://my-mac.tail1234.ts.net")
            XCTAssertEqual(Tailnet.serving, true)
            first.fulfill()
        }
        wait(for: [first], timeout: 10)
        XCTAssertEqual(changed, 1)

        // Fresh: answered at once, and Tailscale isn't asked again.
        var now = false
        Tailnet.whenFresh(within: 2, changed: {}) { now = true }
        XCTAssertTrue(now)
        XCTAssertEqual(asked, 1)
    }

    /// A Tailscale that doesn't answer holds the check up only as long as it's given, and once.
    func testASlowTailscaleHoldsTheCheckUpNoLongerThanItsWait() {
        let real = Tailnet.ask
        defer { Tailnet.ask = real }
        Tailnet.forget()
        Tailnet.ask = {
            Thread.sleep(forTimeInterval: 1)
            return ("https://my-mac.tail1234.ts.net", nil)
        }
        var answers = 0
        let answered = expectation(description: "answered without it")
        Tailnet.whenFresh(within: 0.2, changed: {}) {
            answers += 1
            // Answered before Tailscale's answer, which takes a second, came in: not by the
            // clock, which a busy Mac can hold up, but by what's known when it answers.
            XCTAssertNil(Tailnet.url)
            answered.fulfill()
        }
        wait(for: [answered], timeout: 10)
        // Its late answer is kept for next time, and `then` isn't called again.
        let late = expectation(description: "the late answer")
        Tailnet.refresh(changed: {}) { late.fulfill() }
        wait(for: [late], timeout: 10)
        XCTAssertEqual(Tailnet.url, "https://my-mac.tail1234.ts.net")
        XCTAssertEqual(answers, 1)
        Tailnet.forget()
    }

    func testNoAddressWithoutAName() {
        XCTAssertNil(Tailnet.url(fromStatus: Data(#"{"Self":{"DNSName":""}}"#.utf8)))
        XCTAssertNil(Tailnet.url(fromStatus: Data(#"{"BackendState":"NeedsLogin"}"#.utf8)))
        XCTAssertNil(Tailnet.url(fromStatus: Data("not json".utf8)))
        XCTAssertNil(Tailnet.url(fromStatus: Data(#"{"Self":{"DNSName":"evil.example.com."}}"#.utf8)))
    }

    func testWhetherTailscaleServesTheDeck() {
        let served = Data(#"{"TCP":{"443":{"HTTPS":true}},"Web":{"my-mac.tail1234.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:7710"}}}}}"#.utf8)
        XCTAssertEqual(Tailnet.servesDeck(fromServeStatus: served), true)
        let local = Data(#"{"Web":{"h:443":{"Handlers":{"/":{"Proxy":"http://localhost:7710/"}}}}}"#.utf8)
        XCTAssertEqual(Tailnet.servesDeck(fromServeStatus: local), true)
        let forwarded = Data(#"{"TCP":{"443":{"TCPForward":"127.0.0.1:7710"}}}"#.utf8)
        XCTAssertEqual(Tailnet.servesDeck(fromServeStatus: forwarded), true)
        let other = Data(#"{"Web":{"h:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:77100"}}}}}"#.utf8)
        XCTAssertEqual(Tailnet.servesDeck(fromServeStatus: other), false)
        XCTAssertEqual(Tailnet.servesDeck(fromServeStatus: Data("{}".utf8)), false, "nothing served")
        XCTAssertNil(Tailnet.servesDeck(fromServeStatus: Data("not json".utf8)))
    }

    // MARK: Settings

    func testThePhoneMicIsOnByDefaultAndKeepsItsSettings() throws {
        let fresh = try JSONDecoder().decode(Config.self, from: Data("{}".utf8))
        XCTAssertTrue(fresh.phoneMicEnabled)
        XCTAssertFalse(fresh.phoneMicAutoSend, "the confirm box is the default")
        XCTAssertFalse(Config().phoneMicAutoSend)
        let off = try JSONDecoder().decode(Config.self, from: Data(#"{"phoneMicEnabled":false,"phoneMicSkipsConfirm":false}"#.utf8))
        XCTAssertFalse(off.phoneMicEnabled)
        XCTAssertFalse(off.phoneMicAutoSend)
        let again = try JSONDecoder().decode(Config.self, from: JSONEncoder().encode(off))
        XCTAssertFalse(again.phoneMicEnabled)
        XCTAssertFalse(again.phoneMicAutoSend)
        let on = try JSONDecoder().decode(Config.self, from: Data(#"{"phoneMicSkipsConfirm":true}"#.utf8))
        XCTAssertTrue(try JSONDecoder().decode(Config.self, from: JSONEncoder().encode(on)).phoneMicAutoSend)
    }

    /// Every earlier build saved "Send automatically" as true without asking:
    /// that file starts with the confirm box, and drops the old name.
    func testAnEarlierBuildsAutoSendGivesWayToTheBox() throws {
        let earlier = try JSONDecoder().decode(Config.self, from: Data(#"{"phoneMicEnabled":true,"phoneMicAutoSend":true}"#.utf8))
        XCTAssertTrue(earlier.phoneMicEnabled)
        XCTAssertFalse(earlier.phoneMicAutoSend)
        XCTAssertFalse(earlier.lostOnLoad)
        let saved = String(decoding: try JSONEncoder().encode(earlier), as: UTF8.self)
        XCTAssertTrue(saved.contains(#""phoneMicSkipsConfirm":false"#))
        XCTAssertFalse(saved.contains(#""phoneMicAutoSend""#))
    }
}

/// A terminal that records the text and keys it's given, and can run something as it's asked where the agent is.
private final class RecordingTerminal: TerminalDriver {
    private(set) var inserted: [String] = []
    private(set) var pressed: [Terminal.Key] = []
    var takesKeys = true
    var whenLookedUp: (() -> Void)?

    var abilities: TerminalAbilities { [.keys, .text] }
    var keysNeedAccessibility: Bool { false }
    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard takesKeys else { return false }
        pressed.append(keystroke.key)
        return true
    }
    func type(_ text: String, in tab: TabAddress) -> Bool { false }
    func insert(_ text: String, in tab: TabAddress) -> Bool {
        inserted.append(text)
        return true
    }
    func focus(_ tab: TabAddress) -> Bool { false }
    func scroll(lines: Int, in tab: TabAddress) -> Bool { false }
    func scrollToBottom(in tab: TabAddress) -> Bool { false }
    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired { .notStopped }
    /// Unknown: the keys go as they always did (see `Deck.toAgent`), and nothing real is asked.
    func tty(of tab: TabAddress) -> String? {
        whenLookedUp?()
        return nil
    }
    func isOpen(_ tab: TabAddress) -> Bool? { nil }
    func screen(of tab: TabAddress) -> String? { nil }
}
