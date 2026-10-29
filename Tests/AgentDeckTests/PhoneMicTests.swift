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
