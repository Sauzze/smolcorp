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

    // MARK: Agent windows

    /// A tmux that answers as each hire's server would, and writes down every call.
    private final class FakeTmux {
        var calls: [[String]] = []
        let viewers: [String: [String]]
        /// Servers whose list-clients fails.
        var unasked: Set<String> = []
        init(viewers: [String: [String]]) { self.viewers = viewers }

        func desk(_ socketName: String) -> HireTmux? {
            HireTmux(socketName: socketName, testPort: nil) { [self] arguments in
                calls.append(arguments)
                if arguments.contains("list-clients") {
                    return unasked.contains(socketName) ? (1, "") : (0, (viewers[socketName] ?? []).joined(separator: "\n"))
                }
                return (0, "")
            }
        }
    }

    /// Windows by the ids recorded, each showing a tty; closes write down what they were given.
    private final class FakeCloser: HireWindows.Closer {
        var ttys: [String: String] = [:]
        /// Windows whose terminal couldn't be asked.
        var unknown: Set<String> = []
        var refuse: [String: String] = [:]
        var asked: [String] = []
        var closed: [String] = []
        func look(_ window: String) -> HireWindows.Shown {
            asked.append(window)
            if unknown.contains(window) { return .unknown }
            return ttys[window].map { .open($0) } ?? .gone
        }
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
        closer.ttys = ["tty:/dev/ttys010#w101": "/dev/ttys010", "iterm:AB-12": "/dev/ttys020", "ghostty-id:9F": "/dev/ttys040"]
        closer.refuse = ["ghostty-id:9F": "something still runs in it"]
        let hires = [
            HireWindows.Hire(id: "alpha", name: "Ada", socket: "/private/tmp/tmux-501/smolcorp-alpha"),
            HireWindows.Hire(id: "bravo", name: "Bo", socket: "/private/tmp/tmux-501/smolcorp-bravo"),
            HireWindows.Hire(id: "charlie", name: "Cy", socket: "/private/tmp/tmux-501/smolcorp-charlie"),
            HireWindows.Hire(id: "delta", name: "Di", socket: "/private/tmp/tmux-501/smolcorp-delta"),
            HireWindows.Hire(id: "echo", name: "Ed", socket: ""),
        ]
        let windows = ["alpha": ["tty:/dev/ttys010#w101"], "bravo": ["iterm:AB-12"], "charlie": ["tty:/dev/ttys099#w102"],
                       "delta": ["ghostty-id:9F"], "echo": ["tty:/dev/ttys050#w103"]]
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
        XCTAssertEqual(closer.closed, ["tty:/dev/ttys010#w101", "ghostty-id:9F"])
        XCTAssertFalse(closer.asked.contains("tty:/dev/ttys050#w103"), "a hire without tmux is left in its window")
        XCTAssertEqual(result.forget, ["alpha": ["tty:/dev/ttys010#w101"], "charlie": ["tty:/dev/ttys099#w102"]])

        let said = result.said.joined(separator: "\n")
        XCTAssertTrue(said.contains("Ada: detached; its window wasn't opened by Smolcorp while it ran, so it's left open."), said)
        XCTAssertTrue(said.contains("Bo: detached; a window Smolcorp opened for it shows something else now, so it's left; "
                                    + "its window wasn't opened by Smolcorp while it ran, so it's left open."), said)
        XCTAssertTrue(said.contains("Di: detached; its window is left open: something still runs in it."), said)
        XCTAssertTrue(said.contains("Ed runs in its window without tmux, so it stays there."), said)
        XCTAssertFalse(said.contains("Cy"), said)
        XCTAssertFalse(said.contains("team grid"), "no grid, nothing said of it")
    }

    /// Not knowing isn't "not open": a terminal that couldn't be asked keeps its record, nothing of it is closed, and
    /// that's said; its viewer isn't said to be someone else's window. A tmux that couldn't be asked, and a hire
    /// without tmux and with no record (as after a restart), are said too.
    func testWhatCouldntBeAskedIsKeptAndSaid() {
        let tmux = FakeTmux(viewers: ["smolcorp-alpha": ["/dev/ttys010"], "smolcorp-bravo": ["/dev/ttys020"]])
        tmux.unasked = ["smolcorp-bravo"]
        let closer = FakeCloser()
        closer.unknown = ["ghostty-id:9F"]
        let hires = [
            HireWindows.Hire(id: "alpha", name: "Ada", socket: "smolcorp-alpha"),
            HireWindows.Hire(id: "bravo", name: "Bo", socket: "smolcorp-bravo"),
            HireWindows.Hire(id: "echo", name: "Ed", socket: ""),
        ]
        let result = HireWindows.toBackground(hires, windows: ["alpha": ["ghostty-id:9F"], "bravo": ["tty:/dev/ttys020#w7"]],
                                              tmux: tmux.desk, closer: closer)
        XCTAssertEqual(closer.closed, [])
        XCTAssertEqual(result.forget, [:], "every record kept")
        XCTAssertEqual(result.said, [
            "Ada: detached; Smolcorp couldn't ask Ghostty about the window it opened for it, so it's left.",
            "Bo's tmux couldn't be asked who's watching it, so its window is left.",
            "Ed runs in its window without tmux, so it stays there.",
        ])
        XCTAssertFalse(tmux.calls.contains { $0.contains("smolcorp-bravo") && $0.contains("detach-client") }, "not asked, not detached")
    }

    /// A tmux that can't be reached, or won't detach: nothing is closed, and it's said.
    func testNothingIsClosedWhenTheViewerWasntDetached() {
        let closer = FakeCloser()
        closer.ttys = ["tty:/dev/ttys010#w1": "/dev/ttys010"]
        let stubborn: (String) -> HireTmux? = { name in
            HireTmux(socketName: name, testPort: nil) { $0.contains("list-clients") ? (0, "/dev/ttys010") : (1, "") }
        }
        let hire = HireWindows.Hire(id: "alpha", name: "Ada", socket: "smolcorp-alpha")
        let result = HireWindows.toBackground([hire], windows: ["alpha": ["tty:/dev/ttys010#w1"]], tmux: stubborn, closer: closer)
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
        XCTAssertNil(HireWindows.Terminals.parse("tty:/dev/ttys004")?.window, "recorded without its window's id")
        let terminal = HireWindows.Terminals.parse("tty:/dev/ttys004#w1234")
        XCTAssertEqual(terminal?.id, "/dev/ttys004")
        XCTAssertEqual(terminal?.window, "1234")
        XCTAssertEqual(HireWindows.Terminals.parse("iterm:AB-12")?.id, "AB-12")
        XCTAssertEqual(HireWindows.Terminals.parse("ghostty-id:9F3C")?.id, "9F3C")
        for odd in ["Terminal", "window 1", "ghostty-id:", "ghostty-id:a b", "tty:ttys004", "iterm:nothex!", "tmux:%1@/s",
                    "tty:/dev/ttys004#w", "tty:/dev/ttys004#wfront", "tty:/dev/ttys004#w1 or 2", "iterm:AB-12#w1"] {
            XCTAssertNil(HireWindows.Terminals.parse(odd), odd)
        }
        // A Terminal window as recorded, and its tab as the deck knows it.
        XCTAssertEqual(HireWindows.terminalRecord(tty: "/dev/ttys004", windowID: "1234"), "tty:/dev/ttys004#w1234")
        XCTAssertEqual(HireWindows.terminalRecord(tty: "/dev/ttys004", windowID: nil), "tty:/dev/ttys004")
        XCTAssertEqual(HireWindows.tab(ofRecord: "tty:/dev/ttys004#w1234"), "tty:/dev/ttys004")
        XCTAssertEqual(HireWindows.tab(ofRecord: "iterm:AB-12"), "iterm:AB-12")
        XCTAssertEqual(HireWindows.terminalName("ghostty-id:9F"), "Ghostty")
        XCTAssertEqual(HireWindows.terminalName("iterm:AB-12"), "iTerm2")
        XCTAssertEqual(HireWindows.terminalName("tty:/dev/ttys004#w1"), "Terminal")
        // Without its window's id, a Terminal window is never closed (a tty alone may be a new tab's).
        XCTAssertEqual(HireWindows.Terminals(settle: 0).close("tty:/dev/ttys004", tty: "/dev/ttys004"),
                       .left("Smolcorp didn't get its window's id from Terminal"))
    }

    /// The scripts that look and close: each finds its one window by the id recorded (Terminal's window id and its
    /// tab's tty, iTerm2's unique ID, Ghostty's terminal id), never by a name, never every window, never windows at once.
    func testTheScriptsFindOneWindowByItsId() {
        let scripts = [HireWindows.Terminals.terminalLook, HireWindows.Terminals.terminalClose, HireWindows.Terminals.itermClose,
                       HireWindows.Terminals.ghosttyLook, HireWindows.Terminals.ghosttyClose]
        for script in scripts {
            for never in ["whose name", "name of", "every window", "close windows", "close every", "front window", "window 1"] {
                XCTAssertFalse(script.contains(never), "\(never) in\n\(script)")
            }
        }
        XCTAssertTrue(HireWindows.Terminals.terminalLook.contains("exists window id wid"))
        XCTAssertTrue(HireWindows.Terminals.terminalLook.contains("if tty of t is (item 2 of argv) then return \"open\""))
        let terminal = HireWindows.Terminals.terminalClose
        for words in ["set w to window id wid", "if (count of tabs of w) is not 1 then return \"tabs\"",
                      "if tty of tab 1 of w is not (item 2 of argv) then return \"moved\"", "close w"] {
            XCTAssertTrue(terminal.contains(words), words)
        }
        XCTAssertTrue(HireWindows.Terminals.itermClose.contains("if unique ID of s is (item 1 of argv) then"))
        XCTAssertTrue(HireWindows.Terminals.itermClose.contains("if tty of s is not (item 2 of argv) then return \"moved\""))
        XCTAssertTrue(HireWindows.Terminals.ghosttyClose.contains("set t to terminal id (item 1 of argv)"))
        XCTAssertTrue(HireWindows.Terminals.ghosttyClose.contains("if (tty of t) is not (item 2 of argv) then return \"moved\""))
        // Ghostty 1.3 can't say a tty: the look is in a try, so that's "couldn't ask", never "not open".
        XCTAssertTrue(HireWindows.Terminals.ghosttyLook.contains("try\n        return \"tty:\" & (tty of terminal id (item 1 of argv))\n    on error\n        return \"?\""))
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
        // Next to T3 Code's key (T792), in the same row: ⧉ window in the Mac's own window alone.
        let keys = try XCTUnwrap(deck.range(of: "function withMacKeys(data) {"))
        let withMacKeys = String(deck[keys.upperBound...].prefix(600))
        XCTAssertTrue(withMacKeys.contains("...(bare && windowKey ? [{ id: WINDOW_BUTTON, label: '⧉ window' }] : []),"))
        XCTAssertTrue(withMacKeys.contains("...(t3Here ? [{ id: T3_BUTTON, label: 'T3 Code' }] : [])];"))
        XCTAssertTrue(deck.contains("if (t3Here && action === `button:${T3_BUTTON}`) return openT3(selected);"))
        XCTAssertTrue(deck.contains("    windowSnapshot = data;\n    t3Snapshot = data;\n"), "both keys shown again with the last snapshot")
    }

    // MARK: The T3 Code gate on its routes

    /// Setup's team row: Open in T3 Code only while T3 Code is on, and nothing asked of it while it's off.
    func testSetupsTeamRowShowsT3CodeOnlyWhenItsOn() throws {
        let page = try source("Resources/web/setup.html")
        XCTAssertTrue(page.contains("const t3Key = state.t3CodeEnabled ? t3Button(r) : null;"))
        XCTAssertTrue(page.contains("if (rest.length && state.t3CodeEnabled) t3State(false).then(paintT3);"))
        XCTAssertTrue(page.contains("if (t3Key) row.addEventListener('pointerenter', () => t3State(true));"))
        XCTAssertTrue(page.contains("if (state.t3CodeEnabled && role.id !== 'gm' && (!role.agent || role.agent === 'claude')) {"), "the hire sheet asks only while it's on")
        XCTAssertEqual(page.components(separatedBy: "t3State(").count - 1, 4, "asked in three places, all gated, and defined once")
    }

    private func source(_ path: String) throws -> String {
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        return try String(contentsOf: root.appendingPathComponent(path), encoding: .utf8)
    }
}
