import XCTest
@testable import AgentDeck

/// The first-run guide (Resources/web/guide.js): kept beside hasCompletedSetup, shown on a first run only,
/// and again from Show Me Around Again in the Help menu and the menu bar's.
final class GuideTests: XCTestCase {
    private var file: URL!

    override func setUpWithError() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        file = folder.appendingPathComponent("config.json")
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: file.deletingLastPathComponent())
    }

    private func source(_ path: String) throws -> String {
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        return try String(contentsOf: root.appendingPathComponent(path), encoding: .utf8)
    }

    /// A new Mac sees the guide; a file from before it, of someone who had set up (setup opened, a hires
    /// folder, or a GM hired, by hand too), counts as done; what's saved is kept.
    func testTheGuideIsForAFirstRunOnly() throws {
        XCTAssertFalse(Config.load(from: file).guideDone, "no config yet")
        for text in [#"{"hasCompletedSetup": true}"#, #"{"hireFolder": "/work"}"#, #"{"managerEnabled": true}"#, #"{"gmByHand": true}"#] {
            try text.write(to: file, atomically: true, encoding: .utf8)
            XCTAssertTrue(Config.load(from: file).guideDone, "\(text), from before the guide")
        }
        try #"{"hasCompletedSetup": false, "hireFolder": ""}"#.write(to: file, atomically: true, encoding: .utf8)
        XCTAssertFalse(Config.load(from: file).guideDone, "someone who never set up")
        // Setup opened (hasCompletedSetup) during the guide doesn't end it: what's saved is.
        try #"{"hasCompletedSetup": true, "hireFolder": "/work", "guideDone": false}"#.write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        XCTAssertFalse(config.guideDone)
        config.guideDone = true
        config.save(to: file)
        XCTAssertTrue(Config.load(from: file).guideDone, "finished, saved and loaded again")
        XCTAssertFalse(FileManager.default.fileExists(atPath: file.appendingPathExtension("broken").path))
    }

    /// Not done, beside a setup that's done: saved and loaded again, it's still not done. Were guideDone
    /// not written, the file would load as from before the guide, and count as done.
    func testAGuideNotDoneIsSavedBesideASetupThatIs() throws {
        let config = Config()
        config.hasCompletedSetup = true
        config.guideDone = false
        config.save(to: file)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: file)) as? [String: Any])
        XCTAssertEqual(saved["guideDone"] as? Bool, false, "written")
        let loaded = Config.load(from: file)
        XCTAssertTrue(loaded.hasCompletedSetup)
        XCTAssertFalse(loaded.guideDone, "loaded again: still to be shown")
    }

    /// Show Me Around Again turns the guide on again, saved, and setup opens on #guide, which the page starts it on.
    func testShowMeAroundAgainTurnsTheGuideOnAndOpensSetupOnIt() throws {
        try #"{"hasCompletedSetup": true, "guideDone": true}"#.write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        SettingsWindow.showGuideAgain(config, file: file)
        XCTAssertFalse(config.guideDone)
        XCTAssertFalse(Config.load(from: file).guideDone, "saved")
        XCTAssertEqual(SettingsWindow.address(port: 7710, then: SettingsWindow.guide).fragment, "guide")

        for (path, show) in [("Sources/AgentDeck/App.swift", "settings.show(then: SettingsWindow.guide)"),
                             ("Sources/AgentDeck/StatusMenu.swift", "showSettings(then: SettingsWindow.guide)")] {
            let menu = try source(path)
            XCTAssertTrue(menu.contains(#""Show Me Around Again", #selector(showMeAround))"#), "\(path): the menu item")
            let action = try XCTUnwrap(menu.range(of: "@objc private func showMeAround() {"), path)
            let body = menu[action.upperBound...].prefix(160)
            XCTAssertTrue(body.contains("SettingsWindow.showGuideAgain()") && body.contains(show), "\(path): turns it on, then opens setup on it")
        }
        XCTAssertTrue(try source("Sources/AgentDeck/App.swift").contains(#"let help = submenu(of: bar, "Help")"#), "in the Help menu")
    }

    // MARK: POST /setup/guide, on a stand-in server

    /// Setup's gate and the guide's route, as DeckRouter has them, without the deck (whose full state
    /// reaches into the Keychain): the answer is the state's "guide" part, as setupState gives it.
    private final class StandIn: Router {
        let key = "stand-in-key"
        let sessions = SetupSessions()
        let config: Config
        let file: URL

        init(config: Config, file: URL) { (self.config, self.file) = (config, file) }

        func handle(_ request: Request, reply: @escaping (Reply) -> Void) {
            let respond = { reply(.response($0)) }
            guard DeckRouter.isSetupPath(request.path) else { return respond(.status(404)) }
            let gate = DeckRouter.setupGate(request, sessions: sessions, isSetupKey: { $0 == self.key })
            if let refused = gate.refusal { return respond(refused) }
            guard request.method == "POST", request.path == "/setup/guide" else { return respond(.status(404)) }
            DeckRouter.saveGuide(request, config: config, file: file)
            respond(.json(["guide": DeckRouter.guideState(config)]))
        }
    }

    /// A port nothing listens on, as the system hands one out.
    private func sparePort() throws -> UInt16 {
        let fd = socket(AF_INET, SOCK_STREAM, 0)
        defer { close(fd) }
        var address = sockaddr_in(sin_len: UInt8(MemoryLayout<sockaddr_in>.size), sin_family: sa_family_t(AF_INET), sin_port: 0,
                                  sin_addr: in_addr(s_addr: inet_addr("127.0.0.1")), sin_zero: (0, 0, 0, 0, 0, 0, 0, 0))
        var length = socklen_t(MemoryLayout<sockaddr_in>.size)
        try withUnsafeMutablePointer(to: &address) { pointer in
            try pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                guard Darwin.bind(fd, $0, length) == 0, Darwin.getsockname(fd, $0, &length) == 0 else { throw URLError(.cannotConnectToHost) }
            }
        }
        let port = UInt16(bigEndian: address.sin_port)
        XCTAssertNotEqual(port, 7710)
        return port
    }

    private func send(_ path: String, port: UInt16, key: String?, action: Bool = true) -> (Int, Data) {
        var request = URLRequest(url: URL(string: "http://localhost:\(port)\(path)")!)
        request.httpMethod = "POST"
        if let key { request.setValue(key, forHTTPHeaderField: DeckRouter.setupKeyHeader) }
        if action { request.setValue("1", forHTTPHeaderField: DeckRouter.setupHeader) }
        let done = expectation(description: "POST \(path)")
        var result = (0, Data())
        URLSession.shared.dataTask(with: request) { data, response, _ in
            result = ((response as? HTTPURLResponse)?.statusCode ?? 0, data ?? Data())
            done.fulfill()
        }.resume()
        wait(for: [done], timeout: 10)
        return result
    }

    /// The route, over HTTP: setup's own requests only; done=1 ends the guide and done=0 shows it again,
    /// saved each time; the answer says so as setup's state does ("guide": {"done": …}, as Go's).
    func testTheGuidesRouteKeepsItOverHTTP() throws {
        try #"{"hasCompletedSetup": true}"#.write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        config.guideDone = false
        let port = try sparePort()
        let standIn = StandIn(config: config, file: file)
        let server = HTTPServer(port: port, router: standIn)
        let ready = expectation(description: "listening")
        server.onReady = { ready.fulfill() }
        server.start()
        defer { server.stop() }
        wait(for: [ready], timeout: 10)

        XCTAssertEqual(send("/setup/guide?done=1", port: port, key: nil).0, 401, "no key")
        XCTAssertEqual(send("/setup/guide?done=1", port: port, key: standIn.key, action: false).0, 403, "no setup header")
        XCTAssertFalse(config.guideDone, "refused: nothing changed")

        let done = { (answer: (Int, Data)) -> Bool? in
            XCTAssertEqual(answer.0, 200)
            return ((try? JSONSerialization.jsonObject(with: answer.1)) as? [String: Any])
                .flatMap { $0["guide"] as? [String: Any] }.flatMap { $0["done"] as? Bool }
        }
        XCTAssertEqual(done(send("/setup/guide?done=1", port: port, key: standIn.key)), true)
        XCTAssertTrue(Config.load(from: file).guideDone, "finished: saved")
        XCTAssertEqual(done(send("/setup/guide?done=0", port: port, key: standIn.key)), false)
        XCTAssertFalse(config.guideDone)
        XCTAssertFalse(Config.load(from: file).guideDone, "shown again: saved, beside a setup that's done")
    }

    /// DeckRouter's own route and state go through the same two calls the stand-in does; guide.js is served
    /// as office.js is: no key, nothing of yours.
    func testTheRouteTheStateAndTheScriptAreThere() throws {
        let router = try source("Sources/AgentDeck/DeckRouter.swift")
        let route = try XCTUnwrap(router.range(of: #"case ("POST", "/setup/guide"):"#))
        XCTAssertTrue(router[route.upperBound...].prefix(160).contains("DeckRouter.saveGuide(request, config: config)\n            respond(.json(setupState(caller)))"))
        XCTAssertTrue(router.contains(#""guide": DeckRouter.guideState(config),"#))
        XCTAssertTrue(router.contains(#"request.path == "/guide.js""#))
        let script = try source("Resources/web/guide.js")
        XCTAssertTrue(script.contains("SmolcorpGuide"))
        let setup = try source("Resources/web/setup.html")
        XCTAssertTrue(setup.contains(#"<script src="/guide.js"></script>"#))
        for id in ["#first-folder-use", "#first-folder", "#first-gm-checks", "#start-gm", "#hs-hire", "first-gm-next"] {
            XCTAssertTrue(setup.contains(id), "a step on \(id)")
        }
    }

    /// The bubble keeps off the page's title and Start here's header, and has no count of its own beside
    /// Start here's; off Easy setup it's a pill, and Skip says Show me around again brings it back.
    func testTheGuidesBubbleKeepsOffTheTitleAndCountsNothing() throws {
        let setup = try source("Resources/web/setup.html")
        let from = try XCTUnwrap(setup.range(of: "SmolcorpGuide.create({"))
        let to = try XCTUnwrap(setup.range(of: "onFinish:", range: from.upperBound..<setup.endIndex))
        let guide = setup[from.upperBound..<to.lowerBound]
        XCTAssertTrue(guide.contains("keep: '.sidebar > .brand, .sidebar > .search, .topbar, .pane.current > .pane-head, #easy-attention, #start-here > .card-head'"))
        XCTAssertTrue(guide.contains("awayText: 'Next step in Easy setup'"))
        XCTAssertTrue(guide.contains("skipSays: 'Skip: Show me around again brings it back'"))
        XCTAssertFalse(guide.contains("total:"), "no count of its own")
        let script = try source("Resources/web/guide.js")
        XCTAssertFalse(script.contains("Step ${"), "the bubble says no step number")
        XCTAssertTrue(script.contains(".smol-guide.pill .smol-guide-bubble"))
    }

    /// Fresh eyes, round 2: the bubble keeps off any text near what's lit and goes out of the way once its
    /// control is pressed; the pill's × only hides it, and on a phone Show me and × are 44 px each; the
    /// welcome's last step names the General Manager while none is there. Where it all lands is checked in
    /// node (Tests/fixtures/guide/engine.test.js, run by the Go tests).
    func testTheGuideKeepsOffTextAndTheWelcomeNamesWhatsMissing() throws {
        let script = try source("Resources/web/guide.js")
        for want in ["function lines(litEl, view) {", "{ text, aside: pressed === shown, chip }", "pressed = shown;\n        position();",
                     "pillHiddenIn = where();\n        return hide();", "min-width: 44px; min-height: 44px;", ".smol-guide.pill .smol-guide-go { order: 1; min-height: 44px;"] {
            XCTAssertTrue(script.contains(want), "guide.js has no \(want)")
        }
        let setup = try source("Resources/web/setup.html")
        for want in ["#first-gm-why, #start-said'", "hideSays: \"Hide for now: it's back in the next section you open\"",
                     "<button class=\"btn primary\" id=\"welcome-gm\">Hire your General Manager</button>",
                     "'Next: hire your General Manager' : 'All set'", "go('easy', $('start-gm'));"] {
            XCTAssertTrue(setup.contains(want), "setup.html has no \(want)")
        }
        XCTAssertFalse(setup.contains("on a paired phone. Want desks"), "the last step no longer says ready whatever is missing")
    }

    /// Fresh eyes, round 3: the bubble never sits on the logo or Search; a step whose control is off the
    /// window's edge shows a "Continue below" chip, not a parked bubble; on a phone the bubble is against its
    /// arrow, and the pill shrinks to a chip once you scroll. Where it lands is checked in node (run by Go).
    func testTheGuideStaysByItsControlOrSaysContinueBelow() throws {
        let script = try source("Resources/web/guide.js")
        for want in ["textContent: '↓ Continue below'", "more.textContent = at.point === 'up' ? '↑ Continue above' : '↓ Continue below';",
                     "if (cy < 0 || cy > view.height) return chip(cy < 0);", "if (how.aside || view.width > PHONE) return result('edge'",
                     ".smol-guide.pill.small .smol-guide-bubble", "go.textContent = pillSmall ? 'Next step →' : 'Show me';"] {
            XCTAssertTrue(script.contains(want), "guide.js has no \(want)")
        }
        let setup = try source("Resources/web/setup.html")
        XCTAssertTrue(setup.contains("keep: '.sidebar > .brand, .sidebar > .search, .topbar, "), "the bubble keeps off the logo and Search")
        for want in ["<div class=\"brand\">", "<div class=\"topbar\">", "<nav class=\"sidebar\""] {
            XCTAssertTrue(setup.contains(want), "setup.html has no \(want)")
        }
    }
}
