import XCTest
@testable import AgentDeck

/// The two plain choices (T796): T3 Code on or off, and agent windows, in
/// first setup and Settings; and switching to the background, which detaches
/// each hire's viewers on its own socket and closes only the windows Smolcorp
/// recorded opening.
final class ChoicesTests: XCTestCase {
    private var folder: URL!
    private var file: URL { folder.appendingPathComponent("config.json") }

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    // MARK: Settings

    func testBothChoicesAreSavedAndLoaded() throws {
        let config = Config.load(from: file)
        config.t3CodeEnabled = true
        config.hiresInBackground = true
        config.choicesAsked = true
        config.save(to: file)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: file)) as? [String: Any])
        XCTAssertEqual(saved["t3CodeEnabled"] as? Bool, true)
        XCTAssertEqual(saved["hiresInBackground"] as? Bool, true)
        XCTAssertEqual(saved["choicesAsked"] as? Bool, true)
        let again = Config.load(from: file)
        XCTAssertTrue(again.t3CodeEnabled)
        XCTAssertTrue(again.hiresInBackground)
        XCTAssertTrue(again.choicesAsked)
    }

    /// A new install is asked, with both off; an existing config without the keys keeps today's behaviour, unasked.
    func testANewInstallIsAskedAndAnOldConfigKeepsTodaysBehaviour() throws {
        let fresh = Config.load(from: file)
        XCTAssertFalse(fresh.choicesAsked, "first setup asks")
        XCTAssertFalse(fresh.t3CodeEnabled)
        XCTAssertFalse(fresh.hiresInBackground, "a window for each hire until chosen")

        try #"{"hireFolder": "/work", "hireInTmux": true, "hiringEnabled": true}"#.write(to: file, atomically: true, encoding: .utf8)
        let old = Config.load(from: file)
        XCTAssertTrue(old.choicesAsked, "an existing user isn't asked")
        XCTAssertFalse(old.t3CodeEnabled, "T3 Code off until turned on")
        XCTAssertFalse(old.hiresInBackground, "a window for each hire, as before")
        XCTAssertTrue(old.hireInTmux)
    }

    func testBackgroundNeedsTmux() {
        let config = Config()
        config.hiresInBackground = true
        XCTAssertTrue(Choices.background(config, hasTmux: true))
        XCTAssertFalse(Choices.background(config, hasTmux: false))
        let state = Choices.json(config, hasTmux: false, installed: { false })
        XCTAssertEqual(state["background"] as? Bool, false)
        XCTAssertEqual(state["hasTmux"] as? Bool, false)
    }

    // MARK: The T3 Code gate

    /// Off and asked: the gate says no, and T3 Code isn't even looked for.
    func testTheT3GateHidesEverythingWhenOff() {
        let config = Config()
        config.choicesAsked = true
        XCTAssertFalse(Choices.t3Code(config))
        let state = Choices.json(config, hasTmux: true, installed: { XCTFail("T3 Code was looked for while off"); return true })
        XCTAssertNil(state["t3Installed"], "nothing of T3 Code's is said")
        XCTAssertEqual(state["t3Code"] as? Bool, false)

        // First setup looks, to know whether to ask; on, it looks for the install hint.
        var looked = 0
        config.choicesAsked = false
        XCTAssertEqual(Choices.json(config, hasTmux: true, installed: { looked += 1; return true })["t3Installed"] as? Bool, true)
        config.choicesAsked = true
        config.t3CodeEnabled = true
        XCTAssertTrue(Choices.t3Code(config))
        XCTAssertEqual(Choices.json(config, hasTmux: true, installed: { looked += 1; return false })["t3Installed"] as? Bool, false)
        XCTAssertEqual(looked, 2)
    }

    /// The app by its bundle id, or a `t3` file in T3 Code's own folders, then the PATH's: looked at, never run.
    func testT3CodeIsFoundByItsAppOrAT3FileAndNeverRun() throws {
        let home = folder.appendingPathComponent("home").path
        XCTAssertEqual(Choices.t3Folders(home: home, path: "/usr/bin:relative:/opt/homebrew/bin"),
                       [home + "/.local/bin", home + "/bin", home + "/.t3/bin", "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin"])
        XCTAssertTrue(Choices.t3Installed(home: home, path: "", appFound: { true }, isExecutable: { _ in XCTFail("the app was enough"); return false }))
        XCTAssertFalse(Choices.t3Installed(home: home, path: "/usr/bin", appFound: { false }, isExecutable: { _ in false }))

        // A real file that would leave a mark if it ran.
        let bin = folder.appendingPathComponent("home/.t3/bin", isDirectory: true)
        try FileManager.default.createDirectory(at: bin, withIntermediateDirectories: true)
        let ran = folder.appendingPathComponent("ran")
        try "#!/bin/sh\ntouch '\(ran.path)'\n".write(to: bin.appendingPathComponent("t3"), atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: bin.appendingPathComponent("t3").path)
        XCTAssertTrue(Choices.t3Installed(home: home, path: "", appFound: { false }))
        XCTAssertFalse(FileManager.default.fileExists(atPath: ran.path), "t3 was never run")
    }

    // MARK: Agent windows

    /// A tmux that answers as each hire's server would, and writes down every call.
    private final class FakeTmux {
        var calls: [[String]] = []
        let viewers: [String: [String]]
        init(viewers: [String: [String]]) { self.viewers = viewers }

        func desk(_ socketName: String) -> HireTmux? {
            HireTmux(socketName: socketName, testPort: nil) { [self] arguments in
                calls.append(arguments)
                if arguments.contains("list-clients") { return (0, (viewers[socketName] ?? []).joined(separator: "\n")) }
                return (0, "")
            }
        }
    }

    /// Windows by the ids recorded, each showing a tty; closes write down what they were given.
    private final class FakeCloser: HireWindows.Closer {
        var ttys: [String: String] = [:]
        var refuse: [String: String] = [:]
        var asked: [String] = []
        var closed: [String] = []
        func tty(of window: String) -> String? { asked.append(window); return ttys[window] }
        func close(_ window: String, tty: String) -> HireWindows.Closed {
            closed.append(window)
            return refuse[window].map { .left($0) } ?? .closed
        }
    }

    func testSwitchingToTheBackgroundDetachesEachHireOnItsOwnSocketAndClosesOnlyRecordedWindows() {
        let tmux = FakeTmux(viewers: [
            "smolcorp-alpha": ["/dev/ttys010", "/dev/ttys011"],
            "smolcorp-bravo": ["/dev/ttys021"],
            "smolcorp-delta": ["/dev/ttys040"],
        ])
        let closer = FakeCloser()
        closer.ttys = ["tty:/dev/ttys010": "/dev/ttys010", "iterm:AB-12": "/dev/ttys020", "ghostty-id:9F": "/dev/ttys040"]
        closer.refuse = ["ghostty-id:9F": "something still runs in it"]
        let hires = [
            HireWindows.Hire(id: "alpha", name: "Ada", socket: "/private/tmp/tmux-501/smolcorp-alpha"),
            HireWindows.Hire(id: "bravo", name: "Bo", socket: "/private/tmp/tmux-501/smolcorp-bravo"),
            HireWindows.Hire(id: "charlie", name: "Cy", socket: "/private/tmp/tmux-501/smolcorp-charlie"),
            HireWindows.Hire(id: "delta", name: "Di", socket: "/private/tmp/tmux-501/smolcorp-delta"),
            HireWindows.Hire(id: "echo", name: "Ed", socket: ""),
        ]
        let windows = ["alpha": ["tty:/dev/ttys010"], "bravo": ["iterm:AB-12"], "charlie": ["tty:/dev/ttys099"],
                       "delta": ["ghostty-id:9F"], "echo": ["tty:/dev/ttys050"]]
        let result = HireWindows.toBackground(hires, windows: windows, tmux: tmux.desk, closer: closer)

        // Each call on the hire's own socket; viewers listed and detached by the session, nothing else.
        for call in tmux.calls {
            XCTAssertEqual(Array(call.prefix(1)), ["-L"])
            XCTAssertFalse(call.contains("kill-session") || call.contains("kill-server") || call.contains("kill-pane"), "\(call)")
        }
        let detached = tmux.calls.filter { $0.contains("detach-client") }
        XCTAssertEqual(detached, [
            ["-L", "smolcorp-alpha", "detach-client", "-s", "=hire"],
            ["-L", "smolcorp-bravo", "detach-client", "-s", "=hire"],
            ["-L", "smolcorp-delta", "detach-client", "-s", "=hire"],
        ], "charlie had no viewer, so nothing is detached there")
        XCTAssertEqual(Set(tmux.calls.map { $0[2] }), ["list-clients", "detach-client"])

        // Only recorded windows, by their ids; one that shows something else now, or isn't open, isn't closed.
        XCTAssertEqual(closer.closed, ["tty:/dev/ttys010", "ghostty-id:9F"])
        XCTAssertFalse(closer.asked.contains("tty:/dev/ttys050"), "a hire without tmux is left in its window")
        XCTAssertEqual(result.forget, ["alpha": ["tty:/dev/ttys010"], "charlie": ["tty:/dev/ttys099"]])

        let said = result.said.joined(separator: "\n")
        XCTAssertTrue(said.contains("Ada: detached; its window wasn't opened by Smolcorp while it ran, so it's left open."), said)
        XCTAssertTrue(said.contains("Bo: detached; a window Smolcorp opened for it shows something else now, so it's left; "
                                    + "its window wasn't opened by Smolcorp while it ran, so it's left open."), said)
        XCTAssertTrue(said.contains("Di: detached; its window is left open: something still runs in it."), said)
        XCTAssertTrue(said.contains("Ed runs in its window without tmux, so it stays there."), said)
        XCTAssertFalse(said.contains("Cy"), said)
    }

    /// A tmux that can't be reached, or won't detach: nothing is closed, and it's said.
    func testNothingIsClosedWhenTheViewerWasntDetached() {
        let closer = FakeCloser()
        closer.ttys = ["tty:/dev/ttys010": "/dev/ttys010"]
        let stubborn: (String) -> HireTmux? = { name in
            HireTmux(socketName: name, testPort: nil) { $0.contains("list-clients") ? (0, "/dev/ttys010") : (1, "") }
        }
        let hire = HireWindows.Hire(id: "alpha", name: "Ada", socket: "smolcorp-alpha")
        let result = HireWindows.toBackground([hire], windows: ["alpha": ["tty:/dev/ttys010"]], tmux: stubborn, closer: closer)
        XCTAssertEqual(closer.closed, [])
        XCTAssertEqual(result.said, ["Ada's window couldn't be detached, so it's left."])

        let none = HireWindows.toBackground([HireWindows.Hire(id: "x", name: "Xi", socket: "/elsewhere/default")], windows: [:],
                                            tmux: { _ in nil }, closer: closer)
        XCTAssertEqual(none.said, ["Xi's tmux couldn't be reached, so its window is left."])
    }

    /// Only a hire's own socket: the user's default, or another app's, is never reached.
    func testOnlyOurOwnSocketsAreReached() {
        XCTAssertNil(HireTmux(socketName: "default", testPort: nil, runner: { _ in XCTFail("ran"); return nil }))
        XCTAssertNil(HireTmux(socketName: "t796test", testPort: nil, runner: { _ in XCTFail("ran"); return nil }))
    }

    /// The recorded window ids the Mac's terminals are closed by: Terminal's tab by its tty, iTerm2's session by its
    /// unique ID, Ghostty's terminal by its id. Anything else, as a name, can't be closed.
    func testWindowsAreKnownOnlyByTheIdsTheirTerminalGave() {
        XCTAssertEqual(HireWindows.Terminals.parse("tty:/dev/ttys004")?.kind, "terminal")
        XCTAssertEqual(HireWindows.Terminals.parse("iterm:AB-12")?.id, "AB-12")
        XCTAssertEqual(HireWindows.Terminals.parse("ghostty-id:9F3C")?.id, "9F3C")
        for odd in ["Terminal", "window 1", "ghostty-id:", "ghostty-id:a b", "tty:ttys004", "iterm:nothex!", "tmux:%1@/s"] {
            XCTAssertNil(HireWindows.Terminals.parse(odd), odd)
        }
    }

    /// In the background a hire starts in tmux and no window is opened: only tmux is run, never a terminal.
    func testAHireInTheBackgroundOpensNoWindow() throws {
        let log = folder.appendingPathComponent("tmux-calls")
        let fake = folder.appendingPathComponent("tmux")
        try """
            #!/bin/sh
            echo "$@" >> '\(log.path)'
            case "$*" in *new-session*) echo "%7 /private/tmp/tmux-501/smolcorp-bgtest" ;; esac
            """.write(to: fake, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: fake.path)
        let kept = Tmux.path
        Tmux.path = fake.path
        defer { Tmux.path = kept }
        let opened = try XCTUnwrap(Openers.open("echo hi", in: folder.path, terminal: "terminal", inTmux: true, window: false, hireID: "bgtest"))
        XCTAssertEqual(opened, Openers.Opened(tab: "tmux:%7@/private/tmp/tmux-501/smolcorp-bgtest", window: nil))
        let calls = try String(contentsOf: log, encoding: .utf8).split(separator: "\n")
        XCTAssertEqual(calls.count, 1)
        XCTAssertTrue(calls[0].contains("new-session -d"), String(calls[0]))
        XCTAssertFalse(calls[0].contains("attach"), "no viewer is started")
    }

    /// Once, by hand (the owner asked for it): a real Terminal window attached to a session on our own socket,
    /// t796test, opened by the same opener code; then switched to the background. The client is detached, only
    /// that window closes, the session lives on. Run with SMOLCORP_T796_REAL=1; it cleans up its own socket.
    func testARealAttachedWindowIsDetachedAndClosedAlone() throws {
        guard ProcessInfo.processInfo.environment["SMOLCORP_T796_REAL"] == "1" else { throw XCTSkip("by hand: SMOLCORP_T796_REAL=1") }
        let tmux = try XCTUnwrap(Tmux.path)
        let own = { (arguments: [String]) in Tool.run(tmux, ["-L", "t796test"] + arguments) }
        defer { _ = own(["kill-server"]) }
        XCTAssertEqual(own(["new-session", "-d", "-s", HireTmux.session, "sleep", "600"])?.status, 0)
        let before = try XCTUnwrap(Terminal.openTTYs())
        let line = "exec \(Openers.quoted(tmux)) -L \(Openers.quoted("t796test")) attach-session -t \(Openers.quoted("=" + HireTmux.session))"
        let window = try XCTUnwrap(Openers.script(line).flatMap { Openers.window($0, in: NSHomeDirectory(), terminal: "terminal") })
        let tty = try XCTUnwrap(HireWindows.Terminals.parse(window)?.id)
        var clients = ""
        for _ in 0..<50 where !clients.contains(tty) {
            Thread.sleep(forTimeInterval: 0.1)
            clients = own(["list-clients", "-F", "#{client_tty}"])?.output ?? ""
        }
        print("T796 real: opened \(window); clients before: [\(clients)]; other Terminal tabs: \(before.count)")
        XCTAssertTrue(clients.contains(tty), "attached")

        // As the hire's own server: the runner puts our test socket where a hire's would be.
        let desk = { (_: String) in HireTmux(hireID: "t796test", testPort: nil) { own(Array($0.dropFirst(2))) } }
        let result = HireWindows.toBackground([HireWindows.Hire(id: "t796test", name: "T796 test", socket: "smolcorp-t796test")],
                                              windows: ["t796test": [window]], tmux: desk, closer: HireWindows.Terminals())
        let after = try XCTUnwrap(Terminal.openTTYs())
        let left = own(["list-clients", "-F", "#{client_tty}"])?.output ?? "?"
        let alive = own(["ls"])?.output ?? "?"
        print("T796 real: said \(result.said); forget \(result.forget); clients after: [\(left)]; window's tab open: \(after.contains(tty)); "
              + "other tabs as before: \(before == after) (\(after.count)); tmux -L t796test ls: \(alive)")
        XCTAssertEqual(left, "", "the client was detached")
        XCTAssertFalse(after.contains(tty), "that window closed")
        XCTAssertEqual(before, after, "every other tab is as it was")
        XCTAssertTrue(alive.hasPrefix(HireTmux.session + ":"), "the session is still alive")
        XCTAssertEqual(result.forget, ["t796test": [window]])
    }

    // MARK: Routes

    func testTheChoicesAndAHiresWindowAreThePagesAlone() {
        for path in ["/setup/choices", "/setup/hire/window"] {
            XCTAssertTrue(SetupRoutes.allows(.page, "POST", path), path)
            for caller in [SetupSessions.Caller.gm, .tool, .hire] {
                XCTAssertFalse(SetupRoutes.allows(caller, "POST", path), "\(caller) \(path)")
            }
        }
    }

    func testTheOfficesWindowKeyIsTheMacsOfficeWindowAlone() throws {
        let router = try source("Sources/AgentDeck/DeckRouter.swift")
        for route in [#"case ("GET", "/hire/window"):"#, #"case ("POST", "/hire/window"):"#] {
            let start = try XCTUnwrap(router.range(of: route)?.upperBound, route)
            XCTAssertTrue(router[start...].prefix(200).contains("guard DeckRouter.isOfficeWindow(request, phone: phone)"), route)
        }
    }

    // MARK: The pages, read from the source

    func testBothChoicesAreInFirstSetupAndSettingsWithOneLineEach() throws {
        let page = try source("Resources/web/setup.html")
        let html = String(page[..<(page.range(of: #"<script src="/office.js">"#)?.lowerBound ?? page.endIndex)])
        // First setup: in Start here, after the folder; T3 Code asked only where it's installed.
        let start = try XCTUnwrap(html.range(of: #"<div class="first-choices" id="first-choices" hidden>"#))
        XCTAssertLessThan(try XCTUnwrap(html.range(of: #"id="first-folder""#)).lowerBound, start.lowerBound)
        let first = String(html[start.lowerBound...].prefix(1400))
        let settingsStart = try XCTUnwrap(html.range(of: #"<div class="step" id="step-t3code""#))
        let settings = String(html[settingsStart.lowerBound...].prefix(1700))
        for block in [first, settings] {
            XCTAssertTrue(block.contains(#"<button class="btn" data-t3="1">On</button><button class="btn" data-t3="0">Off</button>"#))
            XCTAssertTrue(block.contains(#"<button class="btn" data-background="0">Open a window for each hire</button><button class="btn" data-background="1">Run hires in the background</button>"#))
            XCTAssertEqual(block.components(separatedBy: #"<p class="choice-line""#).count - 1, 2, "one line of explanation each")
        }
        XCTAssertTrue(first.contains(#"<div class="fc-q" id="first-t3" hidden>"#))
        XCTAssertTrue(page.contains("$('first-t3').hidden = !c.t3Installed;"))
        XCTAssertTrue(settings.contains(#"id="step-windows""#))
        // Background needs tmux: greyed, with a line saying so; recommended with T3 Code on.
        XCTAssertTrue(page.contains("if (key === 'background' && b.dataset.background === '1') b.disabled = !!greyed;"))
        XCTAssertTrue(page.contains(#"Running hires in the background needs tmux, which isn't on this computer."#))
        XCTAssertTrue(page.contains("Recommended with T3 Code: your hires work out of sight while you work in T3 Code."))
        // The guide points at it on a first run.
        XCTAssertTrue(page.contains("{ id: 'choices', target: '#first-choices-use', area: '#first-choices', ...inStartHere,"))
        // A background hire's desk in the team list: Open a window.
        XCTAssertTrue(page.contains("if (state.choices && state.choices.background && r.tmuxSocket) {"))
        XCTAssertTrue(page.contains("fetch('/setup/hire/window', { method: 'POST'"))
    }

    /// 390 px: the answers wrap under the question, the lines wrap, and the team row's buttons go under the name.
    func testTheChoicesFitA390PxScreen() throws {
        let page = try source("Resources/web/setup.html")
        XCTAssertTrue(page.contains(".choices { display: flex; flex-wrap: wrap; gap: 6px; min-width: 0; }"))
        XCTAssertTrue(page.contains(".choice-line { margin: 0; color: var(--dim); font-size: 12.5px; overflow-wrap: anywhere; }"))
        XCTAssertTrue(page.contains(".first-choices .fc-q { display: grid; gap: 4px; min-width: 0; }"))
        XCTAssertTrue(page.contains("head.style.flexWrap = 'wrap';"))
        // No fixed width wider than a phone in either block.
        let start = try XCTUnwrap(page.range(of: "/* Two plain choices (T796)"))
        let css = String(page[start.lowerBound...].prefix(700))
        XCTAssertNil(css.range(of: #"width: \d{3,}px"#, options: .regularExpression), css)
    }

    /// The office's ⧉ window key: the Mac's own office window only, never a phone's office.
    func testTheWindowKeyIsNeverOnAPhone() throws {
        let deck = try source("Resources/web/deck.html")
        XCTAssertTrue(deck.contains("if (bare && action === `button:${WINDOW_BUTTON}`) return openHireWindow(selected);"))
        XCTAssertTrue(deck.contains("if (bare && !preview) {\n    askWindowKey();"))
        let keys = try XCTUnwrap(deck.range(of: "function withMacKeys(data) {"))
        XCTAssertTrue(deck[keys.upperBound...].prefix(80).contains("if (!bare) return data;"))
    }

    private func source(_ path: String) throws -> String {
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        return try String(contentsOf: root.appendingPathComponent(path), encoding: .utf8)
    }
}
