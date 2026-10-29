import XCTest
@testable import AgentDeck

/// Phone microphone (personal build), behind `tailscale serve`: a device on
/// the tailnet reaches no more than a paired phone does, route by route,
/// including those added since the phone mic was first written. Offline.
final class TailnetRouteTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private static func source(_ path: String) throws -> String {
        try String(contentsOf: repo.appendingPathComponent(path), encoding: .utf8)
    }

    /// What `tailscale serve` hands this Mac: from its loopback, the
    /// tailnet name kept as Host, and who it's for in forwarding headers.
    private func throughTailscale(_ method: String, _ path: String, query: [String: String] = [:],
                                  host: String = "my-mac.tail1234.ts.net", extra: [String: String] = [:]) -> Request {
        var headers = ["host": host, "x-forwarded-for": "100.101.102.103", "x-forwarded-proto": "https",
                       "x-forwarded-host": "my-mac.tail1234.ts.net", "tailscale-user-login": "someone@example.com"]
        headers.merge(extra) { _, given in given }
        return Request.addressed(method: method, path: path, query: query, headers: headers, body: Data(),
                                 fromLoopback: true, remoteAddress: "127.0.0.1")
    }

    private func fromThisMac(_ method: String, _ path: String, headers: [String: String] = [:]) -> Request {
        Request.addressed(method: method, path: path, query: [:], headers: ["host": "localhost:7710"].merging(headers) { _, b in b },
                          body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")
    }

    /// Setup's routes, those since d544984 among them: the viewer, layout
    /// and profiles, the phone mic's own, read with the setup key's header.
    private static let setupRoutes: [(String, String)] = [
        ("GET", "/setup"), ("POST", "/setup/session"), ("GET", "/setup/state"), ("GET", "/setup/preview"),
        ("GET", "/setup/viewer"), ("POST", "/setup/viewer"), ("POST", "/setup/viewer/secret"),
        ("POST", "/setup/viewer/check"), ("POST", "/setup/viewer/switch"), ("POST", "/setup/viewer/rotate"),
        ("POST", "/setup/viewer/wipe"), ("GET", "/setup/viewer/qr.png"),
        ("POST", "/setup/layout"), ("POST", "/setup/profiles"), ("GET", "/setup/qr.png"),
        ("POST", "/setup/phonemic"), ("POST", "/setup/pair"), ("POST", "/setup/unpair"), ("POST", "/setup/roles"),
        ("POST", "/setup/hiring"), ("POST", "/setup/hiring/choose"), ("POST", "/setup/hooks"), ("POST", "/setup/summaries"),
        ("POST", "/setup/summarykey"), ("POST", "/setup/notify"), ("POST", "/setup/maxconfirm"),
    ]

    // MARK: Local-only routes

    func testNoSetupRouteIsLocalThroughTailscaleEvenWithItsKeys() {
        let keys = [DeckRouter.setupHeader: "1", DeckRouter.setupKeyHeader: "the-real-setup-key", "cookie": "smolcorp-setup=the-real-setup-key"]
        for (method, path) in Self.setupRoutes {
            XCTAssertTrue(DeckRouter.isSetupPath(path), path)
            XCTAssertFalse(throughTailscale(method, path, extra: keys).isLocal, path)
            // A device on the tailnet naming this Mac itself as Host still comes through the proxy.
            for host in ["localhost", "127.0.0.1", "[::1]", "localhost:7710"] {
                XCTAssertFalse(throughTailscale(method, path, host: host, extra: keys).isLocal, "\(path) as \(host)")
            }
        }
        XCTAssertFalse(throughTailscale("GET", "/setup/qr.png", query: ["via": "tailnet"]).isLocal)
    }

    func testTheHookIsntLocalThroughTailscaleNorItsFloorTwoReports() {
        let hook = ["x-agentdeck-token": "the-real-hook-key", "x-agentdeck-pid": "4242", "x-agentdeck-parent": "parent-session",
                    "x-agentdeck-agent": "claude", "x-agentdeck-tab": "tty:/dev/ttys003"]
        XCTAssertFalse(throughTailscale("POST", "/hook", extra: hook).isLocal)
        XCTAssertFalse(throughTailscale("POST", "/hook", host: "localhost", extra: hook).isLocal)
    }

    func testAnyOneForwardingHeaderIsEnough() {
        for header in ["x-forwarded-for", "x-forwarded-host", "x-forwarded-proto", "forwarded", "tailscale-user-login"] {
            XCTAssertFalse(fromThisMac("GET", "/setup/viewer", headers: [header: "x"]).isLocal, header)
        }
    }

    func testThisMacItselfIsStillLocal() {
        XCTAssertTrue(fromThisMac("GET", "/setup/viewer").isLocal)
        XCTAssertTrue(fromThisMac("POST", "/hook").isLocal)
        let fromNetwork = Request.addressed(method: "GET", path: "/setup", query: [:], headers: ["host": "localhost"], body: Data(),
                                            fromLoopback: false, remoteAddress: "192.168.1.9")
        XCTAssertFalse(fromNetwork.isLocal)
    }

    /// Every route is setup's, behind `isLocal` and the setup key, or the
    /// deck's, behind a paired phone's key: none was added outside both.
    func testEveryRouteIsSetupsOrNeedsAPairedPhone() throws {
        let router = try Self.source("Sources/AgentDeck/DeckRouter.swift")
        let pattern = try NSRegularExpression(pattern: #"case \("[A-Z]+", "(/[^"]*)"\)"#)
        let routes = pattern.matches(in: router, range: NSRange(router.startIndex..., in: router)).map {
            String(router[Range($0.range(at: 1), in: router)!])
        }
        XCTAssertGreaterThan(routes.count, 30)
        let paired: Set = ["/events", "/focus", "/act", "/dictate", "/peek", "/summary", "/glossary", "/choice", "/choose",
                           "/conversation", "/notifications", "/session/info"]
        for route in routes {
            XCTAssertTrue(DeckRouter.isSetupPath(route) || paired.contains(route), "\(route) is neither setup's nor behind a paired key")
        }
        for route in paired { XCTAssertTrue(routes.contains(route), route) }

        // The order that makes it so.
        func at(_ text: String) throws -> String.Index { try XCTUnwrap(router.range(of: text)?.lowerBound, text) }
        XCTAssertNotNil(router.range(of: "if DeckRouter.isSetupPath(request.path) {\n            if let refused = DeckRouter.setupRefusal("
                                     + "request, isKey: deck.access.isSetupKey) { return respond(refused) }\n            return setup(request, respond: respond)"))
        XCTAssertLessThan(try at("guard request.isLocal else { return .status(404) }"), try at("guard isKey(key) else { return .status(401) }"))
        XCTAssertLessThan(try at("guard request.method == \"GET\" || request.headers[setupHeader] == \"1\" else"), try at("guard isKey(key)"))
        XCTAssertEqual(router.components(separatedBy: "setup(request, respond:").count, 2, "setup is reached one way only")
        XCTAssertLessThan(try at(#"if request.path == "/hook" {"#), try at(#"guard request.isLocal, request.method == "POST" else"#))
        XCTAssertLessThan(try at("guard request.isLocal, request.method == \"POST\" else"), try at("deck.ingest("))
        let phoneGate = try at("guard let phone else { return respond(.status(401)) }")
        XCTAssertLessThan(phoneGate, try at(#"case ("POST", "/dictate"):"#))
        XCTAssertLessThan(phoneGate, try at(#"case ("POST", "/peek"):"#))
        XCTAssertLessThan(phoneGate, try at(#"case ("GET", "/choice"):"#))
        XCTAssertLessThan(phoneGate, try at(#"case ("POST", "/choose"):"#))
        XCTAssertLessThan(phoneGate, try at(#"case ("GET", "/events"):"#))
        XCTAssertLessThan(phoneGate, try at(#"case ("GET", "/conversation"):"#))
        XCTAssertLessThan(phoneGate, try at(#"case ("POST", "/notifications"):"#))
        XCTAssertLessThan(phoneGate, try at(#"case ("GET", "/session/info"):"#))
        // iPhone app: the conversation and the device token, through Tailscale only, checked before anything is read or kept.
        // The Mac's own window reads it too, unredacted: only that window, from this Mac.
        XCTAssertLessThan(try at("let here = DeckRouter.isOfficeWindow(request, phone: phone)"),
                          try at("guard here || DeckRouter.showsConversation(request, phone: phone) else"))
        XCTAssertLessThan(try at("guard here || DeckRouter.showsConversation(request, phone: phone) else"), try at("deck.conversation(session"))
        XCTAssertTrue(router.contains("deck.conversation(session, before: before, known: known, redacted: !here, done: respond)"))
        // ⓘ Info: the Mac's window, or through Tailscale, checked before anything is read.
        XCTAssertLessThan(try at("guard DeckRouter.showsInfo(request, phone: phone) else"), try at("deck.info("))
        XCTAssertLessThan(try at("guard DeckRouter.takesPushToken(request, phone: phone) else"), try at("deck.access.phones.setPush("))
        // Only the office's own window skips pairing, and only from this Mac.
        XCTAssertTrue(router.contains("let window = request.isLocal && key.map { constantTimeEqual($0, deck.windowKey) } == true"))
    }

    // MARK: Who the deck answers

    func testATailnetNameIsLetInOnlyWithThePhoneMicOn() {
        let tailnet = throughTailscale("GET", "/events")
        XCTAssertTrue(DeckRouter.admits(tailnet, phoneMic: true))
        XCTAssertFalse(DeckRouter.admits(tailnet, phoneMic: false))
    }

    func testATailnetNameStraightFromTheNetworkIsntBelieved() {
        let straight = Request.addressed(method: "GET", path: "/events", query: [:], headers: ["host": "my-mac.tail1234.ts.net"],
                                         body: Data(), fromLoopback: false, remoteAddress: "100.101.102.103")
        XCTAssertFalse(DeckRouter.admits(straight, phoneMic: true))
        let rebound = Request.addressed(method: "GET", path: "/events", query: [:], headers: ["host": "evil.example.com"],
                                        body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")
        XCTAssertFalse(DeckRouter.admits(rebound, phoneMic: true))
        let lan = Request.addressed(method: "GET", path: "/events", query: [:], headers: ["host": "192.168.1.4:7710"],
                                    body: Data(), fromLoopback: false, remoteAddress: "192.168.1.9")
        XCTAssertTrue(DeckRouter.admits(lan, phoneMic: false), "the Wi-Fi deck is as before")
    }

    /// `tailscale serve` passes on whatever Host the device sent: as this
    /// Mac's name or an IP, it's still the tailnet's, and shut out with the
    /// setting off.
    func testThroughTailscaleAnyHostIsTheTailnets() {
        for host in ["localhost", "127.0.0.1", "[::1]", "192.168.1.4:7710", "100.101.102.103", "my-mac.local", "evil.example.com"] {
            let request = throughTailscale("GET", "/events", host: host)
            XCTAssertTrue(request.isTailnet, host)
            XCTAssertFalse(request.isAddressedDirectly, host)
            XCTAssertFalse(DeckRouter.admits(request, phoneMic: false), "\(host): the setting off shuts the tailnet out")
            XCTAssertTrue(DeckRouter.admits(request, phoneMic: true), host)
            XCTAssertEqual(DeckRouter.pairingAddress(throughTailscale("POST", "/pair", host: host)), "tailnet:100.101.102.103", host)
        }
        // A forwarding header from the network proves nothing: it's the Wi-Fi's, as before.
        let lan = Request.addressed(method: "GET", path: "/events", query: [:], headers: ["host": "192.168.1.4", "x-forwarded-for": "1.2.3.4"],
                                    body: Data(), fromLoopback: false, remoteAddress: "192.168.1.9")
        XCTAssertFalse(lan.isTailnet)
        XCTAssertTrue(DeckRouter.admits(lan, phoneMic: false))
    }

    // MARK: Dictation

    /// The phone's words are typed only from the Tailscale deck, over HTTPS:
    /// a key seen on the Wi-Fi's plain http can't type a prompt of its own.
    func testDictationIsTakenOnlyThroughTailscale() throws {
        let wifi = Request.addressed(method: "POST", path: "/dictate", query: ["t": "a-paired-key", "session": "s1"],
                                     headers: ["host": "192.168.1.4:7710"], body: Data(), fromLoopback: false, remoteAddress: "192.168.1.9")
        XCTAssertFalse(DeckRouter.takesDictation(wifi))
        XCTAssertFalse(DeckRouter.takesDictation(fromThisMac("POST", "/dictate")))
        XCTAssertTrue(DeckRouter.takesDictation(throughTailscale("POST", "/dictate", query: ["t": "a-paired-key", "session": "s1"])))
        // And the route asks it first, answering 403.
        let router = try Self.source("Sources/AgentDeck/DeckRouter.swift")
        let route = try XCTUnwrap(router.range(of: #"case ("POST", "/dictate"):"#))
        let first = router[route.upperBound...].split(separator: "\n").prefix(3).map { $0.trimmingCharacters(in: .whitespaces) }
        XCTAssertEqual(first.first, "guard DeckRouter.takesDictation(request) else {")
        XCTAssertTrue(first.dropFirst().first?.hasPrefix("return respond(.status(403,") == true)
    }

    /// The peek at an agent's input line is what its terminal shows: given
    /// only through Tailscale, as dictation is, never over the Wi-Fi's plain
    /// http, nor to this Mac's own loopback without the proxy.
    func testThePeekIsShownOnlyThroughTailscale() throws {
        let wifi = Request.addressed(method: "POST", path: "/peek", query: ["t": "a-paired-key", "session": "s1"],
                                     headers: ["host": "192.168.1.4:7710"], body: Data(), fromLoopback: false, remoteAddress: "192.168.1.9")
        XCTAssertFalse(DeckRouter.showsTerminal(wifi))
        XCTAssertFalse(DeckRouter.showsTerminal(fromThisMac("POST", "/peek")))
        let lanWithForwarding = Request.addressed(method: "POST", path: "/peek", query: ["t": "a-paired-key", "session": "s1"],
                                                  headers: ["host": "my-mac.tail1234.ts.net", "x-forwarded-for": "100.101.102.103"],
                                                  body: Data(), fromLoopback: false, remoteAddress: "192.168.1.9")
        XCTAssertFalse(DeckRouter.showsTerminal(lanWithForwarding), "a forwarding header from the network proves nothing")
        XCTAssertTrue(DeckRouter.showsTerminal(throughTailscale("POST", "/peek", query: ["t": "a-paired-key", "session": "s1"])))
        for host in ["localhost", "127.0.0.1", "192.168.1.4:7710"] {
            XCTAssertTrue(DeckRouter.showsTerminal(throughTailscale("POST", "/peek", host: host)), host)
            XCTAssertFalse(DeckRouter.admits(throughTailscale("POST", "/peek", host: host), phoneMic: false), "\(host): off, shut out")
        }
        // And the route asks it first, answering 403.
        let router = try Self.source("Sources/AgentDeck/DeckRouter.swift")
        let route = try XCTUnwrap(router.range(of: #"case ("POST", "/peek"):"#))
        let first = router[route.upperBound...].split(separator: "\n").prefix(3).map { $0.trimmingCharacters(in: .whitespaces) }
        XCTAssertEqual(first.first, "guard DeckRouter.showsTerminal(request) else {")
        XCTAssertTrue(first.dropFirst().first?.hasPrefix("return respond(.status(403,") == true)
        XCTAssertTrue(router.contains("static func showsTerminal(_ request: Request) -> Bool {\n        request.isTailnet\n    }"))
        // Read only while the agent is in front: a shell's screen is never read.
        let deck = try Self.source("Sources/AgentDeck/Deck.swift")
        let peek = try XCTUnwrap(deck.range(of: "func peek(_ id: String"))
        let body = deck[peek.upperBound...].prefix(1400)
        XCTAssertTrue(body.contains("guard Config.shared.phoneMicEnabled else"))
        XCTAssertTrue(body.contains("Terminal.place(of: agent, on: $0) }) == .front else"))
        XCTAssertLessThan(try XCTUnwrap(body.range(of: "== .front else")).lowerBound, try XCTUnwrap(body.range(of: "driver.screen(of:")).lowerBound)
        // The visible screen only: Terminal.app's and iTerm2's `contents`, never their `history`
        // (tmux, WezTerm and kitty are pinned by their stand-ins, in TmuxTests and WezTermKittyTests).
        let terminal = try Self.source("Sources/AgentDeck/Terminal.swift")
        let contents = try XCTUnwrap(terminal.range(of: "static func contents(tty: String) -> String? {"))
        let read = terminal[contents.upperBound...].prefix(700)
        XCTAssertTrue(read.contains(#"if tty of t is target then return "yes" & (contents of t)"#))
        XCTAssertFalse(read.contains("history"))
        let drivers = try Self.source("Sources/AgentDeck/TerminalDrivers.swift")
        XCTAssertTrue(drivers.contains(#"withSession(tab.id, "return \"ok\" & (contents of s)")"#))
        XCTAssertFalse(drivers.lowercased().contains("history"))
        XCTAssertTrue(drivers.contains(#"run(tab, ["capture-pane", "-p", "-t", tab.id])"#))
    }

    /// A permission prompt's command or file, for the iPhone app's choice
    /// sheet: given only through Tailscale, as the peek is, once a second at
    /// most, and only while the session asks.
    func testAChoicesDetailIsShownOnlyThroughTailscale() throws {
        let wifi = Request.addressed(method: "GET", path: "/choice", query: ["t": "a-paired-key", "session": "s1"],
                                     headers: ["host": "192.168.1.4:7710"], body: Data(), fromLoopback: false, remoteAddress: "192.168.1.9")
        XCTAssertFalse(DeckRouter.showsTerminal(wifi))
        XCTAssertFalse(DeckRouter.showsTerminal(fromThisMac("GET", "/choice")))
        XCTAssertTrue(DeckRouter.showsTerminal(throughTailscale("GET", "/choice", query: ["t": "a-paired-key", "session": "s1"])))
        let router = try Self.source("Sources/AgentDeck/DeckRouter.swift")
        let route = try XCTUnwrap(router.range(of: #"case ("GET", "/choice"):"#))
        let first = router[route.upperBound...].split(separator: "\n").prefix(3).map { $0.trimmingCharacters(in: .whitespaces) }
        XCTAssertEqual(first.first, "guard DeckRouter.showsTerminal(request) else {")
        XCTAssertTrue(first.dropFirst().first?.hasPrefix("return respond(.status(403,") == true)
        let deck = try Self.source("Sources/AgentDeck/Deck.swift")
        let look = try XCTUnwrap(deck.range(of: "func choiceDetail(_ id: String"))
        let body = deck[look.upperBound...].prefix(900)
        XCTAssertTrue(body.contains("guard Config.shared.phoneMicEnabled else"))
        XCTAssertTrue(body.contains("PhoneDictation.peekTooSoon(last: lastChoiceLook[id], now: now)"))
        XCTAssertTrue(body.contains("guard Deck.isAsking(session.state), let choice = session.choice else"))
        // And the snapshot, which the Wi-Fi gets too, never carries it.
        let entry = try XCTUnwrap(deck.range(of: "private func sessionEntry(_ s: Session)"))
        XCTAssertTrue(deck[entry.upperBound...].prefix(4000).contains("s.choice?.forPhone"))
        let choice = try Self.source("Sources/AgentDeck/Choice.swift")
        let forPhone = try XCTUnwrap(choice.range(of: "func forPhone(menuSeen: Bool) -> [String: Any] {"))
        XCTAssertFalse(choice[forPhone.upperBound...].prefix(300).contains("detail"))
    }

    /// A typed answer to a menu's "Type something" is text typed into an
    /// agent, as dictation is: taken only through Tailscale, never over the
    /// Wi-Fi's plain http (P-1), and the route asks it first.
    func testATypedAnswerIsTakenOnlyThroughTailscale() throws {
        let wifi = Request.addressed(method: "POST", path: "/choose", query: ["t": "a-paired-key", "session": "s1", "option": "other"],
                                     headers: ["host": "192.168.1.4:7710"], body: Data(), fromLoopback: false, remoteAddress: "192.168.1.9")
        XCTAssertFalse(DeckRouter.takesDictation(wifi))
        XCTAssertFalse(DeckRouter.takesDictation(fromThisMac("POST", "/choose")))
        XCTAssertTrue(DeckRouter.takesDictation(throughTailscale("POST", "/choose", query: ["t": "a-paired-key", "session": "s1"])))
        let router = try Self.source("Sources/AgentDeck/DeckRouter.swift")
        let route = try XCTUnwrap(router.range(of: #"case ("POST", "/choose"):"#))
        let first = router[route.upperBound...].split(separator: "\n").prefix(3).map { $0.trimmingCharacters(in: .whitespaces) }
        XCTAssertEqual(first.first, "guard DeckRouter.takesDictation(request) else {")
        XCTAssertTrue(first.dropFirst().first?.hasPrefix("return respond(.status(403,") == true)
        // The deck's /act, which the Wi-Fi reaches, never takes text for it.
        let deck = try Self.source("Sources/AgentDeck/Deck.swift")
        XCTAssertEqual(deck.components(separatedBy: "answerOther(").count, 2, "defined, and called by /choose only")
        XCTAssertEqual(router.components(separatedBy: "deck.answerOther(").count, 2)
    }

    // MARK: Pairing

    func testWrongCodesThroughTailscaleAreCountedPerDevice() {
        XCTAssertEqual(DeckRouter.pairingAddress(throughTailscale("POST", "/pair")), "tailnet:100.101.102.103")
        // The proxy's own entry is the last: one the device wrote itself, before it, isn't believed.
        let spoofed = throughTailscale("POST", "/pair", extra: ["x-forwarded-for": "1.2.3.4, 100.101.102.103"])
        XCTAssertEqual(DeckRouter.pairingAddress(spoofed), "tailnet:100.101.102.103")
        var none = throughTailscale("POST", "/pair")
        none = Request.addressed(method: "POST", path: "/pair", query: [:], headers: none.headers.filter { $0.key != "x-forwarded-for" },
                                 body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")
        XCTAssertEqual(DeckRouter.pairingAddress(none), "tailnet:127.0.0.1", "kept apart from this Mac's own")
        let lan = Request.addressed(method: "POST", path: "/pair", query: [:], headers: ["host": "192.168.1.4", "x-forwarded-for": "9.9.9.9"],
                                    body: Data(), fromLoopback: false, remoteAddress: "192.168.1.9")
        XCTAssertEqual(DeckRouter.pairingAddress(lan), "192.168.1.9", "on the Wi-Fi, as before")
    }

    // MARK: No Funnel

    func testSmolcorpOnlyEverReadsTailscalesStatus() throws {
        let sources = try FileManager.default.contentsOfDirectory(atPath: Self.repo.appendingPathComponent("Sources/AgentDeck").path)
        for file in sources where file.hasSuffix(".swift") {
            let text = try Self.source("Sources/AgentDeck/" + file).lowercased()
            XCTAssertFalse(text.contains("\"funnel\""), file)
            // `serve` only ever to read its status: never to start, change or stop serving.
            let read = #"["serve", "status", "--json"]"#
            XCTAssertFalse(text.replacingOccurrences(of: read, with: "").contains("\"serve\""), file)
        }
        // Two looks, both read-only, and nothing else.
        let tailnet = try Self.source("Sources/AgentDeck/PhoneDictation.swift")
        XCTAssertEqual(tailnet.components(separatedBy: "run(tool, ").count, 3)
        XCTAssertTrue(tailnet.contains(#"run(tool, ["status", "--json"])"#))
        XCTAssertTrue(tailnet.contains(#"run(tool, ["serve", "status", "--json"])"#))
    }

    // MARK: Told apart on the Home Screen

    func testTheTailscaleDecksIconIsBadged() throws {
        // At the Tailscale address, each logo has its own, badged TAIL
        for logo in Logo.allCases {
            XCTAssertEqual(logo.tailIcon.lastPathComponent, "icon-\(logo.rawValue)-tail.png")
            let tail = try Data(contentsOf: Self.repo.appendingPathComponent("Resources/web/icon-\(logo.rawValue)-tail.png"))
            XCTAssertNotEqual(tail, try Data(contentsOf: Self.repo.appendingPathComponent("Resources/web/icon-\(logo.rawValue).png")))
        }
        let router = try Self.source("Sources/AgentDeck/DeckRouter.swift")
        XCTAssertTrue(router.contains("icon: request.isTailnet ? Config.shared.logo.tailIcon : Config.shared.logo.icon"))
    }

    func testTheTailscaleDeckIsMarkedAndTheWiFiOneIsnt() throws {
        for (page, title) in [("deck.html", "<title>Smolcorp Tail</title>"), ("pair.html", "<title>Pair Smolcorp Tail</title>")] {
            let original = try Self.source("Resources/web/" + page)
            let marked = String(decoding: DeckRouter.markedForTailnet(Response(contentType: "text/html", body: Data(original.utf8))).body, as: UTF8.self)
            XCTAssertTrue(marked.contains(#"<meta name="apple-mobile-web-app-title" content="Smolcorp Tail">"#), page)
            XCTAssertTrue(marked.contains(title), page)
            XCTAssertTrue(marked.contains(#"<html lang="en" data-tail>"#), page)
            XCTAssertFalse(original.contains("data-tail>"), page)
            XCTAssertFalse(original.contains("Smolcorp Tail"), page)
        }
        let deck = try Self.source("Resources/web/deck.html")
        XCTAssertTrue(deck.contains("TAIL · phone mic"))
        XCTAssertTrue(deck.contains(".tail-mark { display: none; }"), "hidden on the Wi-Fi deck")
        let notFound = Response.status(404)
        XCTAssertEqual(DeckRouter.markedForTailnet(notFound).body, notFound.body)
    }
}
