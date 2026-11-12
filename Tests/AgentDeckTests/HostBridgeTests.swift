import XCTest
@testable import AgentDeck

/// The bridge the Go core asks this app through: the token, what each action
/// may be given, and the socket, on stand-ins for the terminals and the stores.
final class HostBridgeTests: XCTestCase {
    private let token = "fake-token-fake-token-fake-token-0"
    private var folder: URL!
    private var hires: URL!
    /// What the stand-ins were asked, in order.
    private var asked: [String] = []

    override func setUpWithError() throws {
        // Short, for a socket's path (103 bytes at most).
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("sb-\(UUID().uuidString.prefix(8))", isDirectory: true)
        hires = folder.appendingPathComponent("hires", isDirectory: true)
        try FileManager.default.createDirectory(at: hires, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        asked = []
        // The phone's text's gates on stand-ins: keys allowed, the Mac unlocked, the agent alone in front of its tab.
        let gates = (Deck.keyboardAllowed, Deck.macLocked, Deck.tabFront)
        Deck.keyboardAllowed = { true }
        Deck.macLocked = { false }
        Deck.tabFront = { _, _ in Terminal.TabFront(place: .front, readers: []) }
        addTeardownBlock { (Deck.keyboardAllowed, Deck.macLocked, Deck.tabFront) = gates }
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    private var kept: [String: String] = ["summary-key": "fake-summary-key", "viewer-id": "AbCdEfGhIjKlMnOpQrStUv",
                                          "gm-token": "fake-gm-token-from-1-24-0000000000000000000"]
    private var login = false
    /// Where the stand-ins' tab actions run, as the terminals' queue.
    private let terminals = DispatchQueue(label: "test.terminals")
    /// Where a window opens: a queue of its own, so a test sees `open` go there and not to a tab's queue (RSY3's nit 1),
    /// and the terminal each window's queue was asked for.
    private let windows = DispatchQueue(label: "test.windows")
    private var windowQueues: [String] = []

    private var actions: HostBridge.Actions {
        var actions = HostBridge.Actions(
            press: { [unowned self] key, tab in asked.append("press \(key.key) \(key.modifiers.rawValue) \(key.times) \(tab.text)"); return true },
            type: { [unowned self] text, tab in asked.append("type \(text) \(tab.text)"); return true },
            insert: { [unowned self] text, tab in asked.append("insert \(text) \(tab.text)"); return false },
            focus: { [unowned self] tab in asked.append("focus \(tab.text)"); return tab.kind == "tmux" ? true : nil },
            scroll: { [unowned self] lines, tab in asked.append("scroll \(lines) \(tab.text)"); return true },
            scrollToBottom: { [unowned self] tab in asked.append("bottom \(tab.text)"); return true },
            fire: { [unowned self] tab, agent in
                asked.append("fire \(agent.rawValue) \(tab.text)")
                return ["%1": .fired, "%2": .gone, "%3": .notStopped][tab.id]
            },
            hires: hires,
            open: { [unowned self] terminal, folder, script in asked.append("open \(terminal) \(folder) \(script)"); return Openers.Opened(tab: "iterm:AB12", window: "iterm:AB12") },
            secrets: [
                "summary-key": HostBridge.Secret(read: { [unowned self] in kept["summary-key"] },
                                                 write: { [unowned self] in kept["summary-key"] = $0; return true },
                                                 delete: { [unowned self] in kept["summary-key"] = nil }),
                // The portable office's, as the Mac keeps them: an id, no key yet, and a write secret there but
                // unreadable (a denied prompt); none removed this way.
                "viewer-id": HostBridge.Secret(read: { [unowned self] in kept["viewer-id"] },
                                               write: { [unowned self] in kept["viewer-id"] = $0; return true },
                                               delete: nil, absent: { [unowned self] in kept["viewer-id"] == nil }),
                "viewer-key": HostBridge.Secret(read: { nil }, write: { _ in false }, delete: nil, absent: { true }),
                "viewer-write-secret": HostBridge.Secret(read: { nil }, write: { _ in false }, delete: nil, absent: { false }),
                // The GM's token (M7TOK), a stand-in for GMIdentity's store.
                "gm-token": HostBridge.Secret(read: { [unowned self] in kept["gm-token"] },
                                              write: { [unowned self] in kept["gm-token"] = $0; return true },
                                              delete: { [unowned self] in kept["gm-token"] = nil }),
            ],
            loginItem: (get: { [unowned self] in login }, set: { [unowned self] in login = $0 }),
            queue: { [terminals] _ in terminals },
            windowQueue: { [unowned self] terminal in windowQueues.append(terminal); return windows })
        // No real terminal is looked at or told anything: a test gives its own stand-in.
        actions.driver = { _ in nil }
        actions.place = { _, _ in nil }
        actions.screen = { _ in nil }
        // No AppleScript ever runs, and no screen is read: a test that tiles gives its own stand-in.
        actions.tileScript = { _, _ in XCTFail("an AppleScript would have run"); return .failed }
        actions.screens = { [] }
        actions.openTTYs = { [] } // Terminal with no tab open: its tab list is never read
        // The team's registry, as the core keeps it: the fixture's hire, and the one these tests ask about.
        actions.openWorktrees = { ["/Users/ada/repo-hires/ada-abcdef", "/Users/ada/wt"] }
        return actions
    }

    private func ask(_ request: [String: Any], token given: String? = nil, actions these: HostBridge.Actions? = nil) -> [String: Any] {
        var request = request
        request["token"] = given ?? token
        let line = try! JSONSerialization.data(withJSONObject: request)
        return HostBridge.answer(line, token: token, actions: these ?? actions)
    }

    private func isOK(_ answer: [String: Any]) -> Bool { answer["ok"] as? Bool == true }

    /// RTCH N1: on a test copy the bridge refuses what would bring another app or a system window forward, in a
    /// plain sentence, and asks the Mac nothing; on the real app the same actions go through.
    func testATestCopyRefusesWhatWouldBringAnotherAppForward() throws {
        var told: [String] = []
        func copy(_ on: Bool) -> HostBridge.Actions {
            var actions = self.actions
            actions.testCopy = on
            actions.main = terminals
            actions.askAccessibility = { told.append("accessibility") }
            actions.t3OpenApp = { told.append("t3-app") }
            actions.t3OpenLink = { _ in told.append("t3-link"); return true }
            return actions
        }
        let script = hires.appendingPathComponent("hire-1.sh")
        try Data("#!/bin/sh\n".utf8).write(to: script)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
        let link = "t3code://threads/env-1/mcp%3Aabc-1"
        let requests: [(String, [String: Any], TestCopy.OtherApp)] = [
            ("focus", ["action": "focus", "tab": "tmux:%1@/tmp/s"], .terminal),
            ("open", ["action": "open", "terminal": "terminal", "folder": folder.path, "script": script.path], .terminal),
            ("accessibility", ["action": "accessibility"], .accessibility),
            ("t3-open-app", ["action": "t3-open-app"], .t3Code),
            ("t3-link", ["action": "t3-link", "link": link], .t3Code),
        ]
        for (name, request, app) in requests {
            var line = request
            line["token"] = token
            let data = try JSONSerialization.data(withJSONObject: line)
            let refused = HostBridge.answer(data, token: token, actions: copy(true))
            XCTAssertEqual(refused["ok"] as? Bool, false, name)
            XCTAssertEqual(refused["error"] as? String, app.sentence, name)
        }
        XCTAssertTrue(told.isEmpty, "the Mac was asked nothing")
        XCTAssertTrue(asked.isEmpty, "no tab was driven, no window opened")
        // The real app's actions still go through.
        for (name, request, _) in requests {
            var line = request
            line["token"] = token
            let data = try JSONSerialization.data(withJSONObject: line)
            let answered = HostBridge.answer(data, token: token, actions: copy(false))
            XCTAssertEqual(answered["ok"] as? Bool, true, name)
        }
        XCTAssertEqual(told, ["accessibility", "t3-app", "t3-link"])
        // Looking at T3 Code's app is not opening it.
        XCTAssertEqual(HostBridge.answer(try JSONSerialization.data(withJSONObject: ["token": token, "action": "t3-app"]),
                                         token: token, actions: copy(true))["ok"] as? Bool, true)
    }

    /// RWTC N1 and N2: on a test copy the bridge does nothing in a tab that isn't one of its own hires' tmux panes
    /// (keys, text, a paste, a scroll, a close, the phone's words, a peek), and leaves the owner's music, volume and
    /// microphone alone; each in a sentence, with nothing asked of the Mac. Its own panes are driven as ever.
    func testATestCopyDoesNothingInTheOwnersTabsNorToTheirMusicOrMicrophone() throws {
        var told: [String] = []
        func copy(_ on: Bool) -> HostBridge.Actions {
            var actions = self.actions
            actions.testCopy = on
            actions.main = terminals
            actions.keyboard = { true }
            actions.pressMusic = { told.append("music \($0)") }
            actions.setVolume = { told.append("volume \($0)") }
            actions.musicState = { told.append("music-state"); return (nil, nil) }
            actions.prepareDictation = { language, done in told.append("dictation \(language)"); done(nil) }
            actions.driver = { _ in told.append("driver"); return nil }
            return actions
        }
        func answer(_ request: [String: Any], _ actions: HostBridge.Actions) throws -> [String: Any] {
            var line = request
            line["token"] = token
            return HostBridge.answer(try JSONSerialization.data(withJSONObject: line), token: token, actions: actions)
        }
        let owners = ["iterm:AB12-CD34", "tty:/dev/ttys003", "ghostty:/dev/ttys004", "kitty:3@/tmp/kitty-501",
                      "tmux:%1@/private/tmp/tmux-501/default", "launcher:L1", ""]
        let tabActions: [[String: Any]] = [
            ["action": "press", "key": "enter"], ["action": "type", "text": "/compact"], ["action": "insert", "text": "hello"],
            ["action": "scroll", "lines": 3], ["action": "scroll-bottom"], ["action": "close", "agent": "claude"],
            ["action": "phone-text", "text": "hello", "agent": "claude"], ["action": "peek", "agent": "claude"],
        ]
        for tab in owners {
            for var request in tabActions {
                request["tab"] = tab
                let refused = try answer(request, copy(true))
                XCTAssertEqual(refused["ok"] as? Bool, false, "\(request)")
                XCTAssertEqual(refused["error"] as? String, TestCopy.OtherApp.terminal.sentence, "\(request)")
            }
        }
        for (request, app) in [(["action": "music", "music": "playpause"], TestCopy.OtherApp.music), (["action": "volume", "level": 30], .music),
                               (["action": "music-state"], .music), (["action": "dictation", "language": "en_US"], .microphone)] as [([String: Any], TestCopy.OtherApp)] {
            let refused = try answer(request, copy(true))
            XCTAssertEqual(refused["error"] as? String, app.sentence, "\(request)")
        }
        XCTAssertTrue(told.isEmpty, "the Mac was asked nothing: \(told)")
        XCTAssertTrue(asked.isEmpty, "no tab was driven: \(asked)")

        // Its own hire's pane, on a socket of its own: driven as ever.
        let own = "tmux:%1@/private/tmp/tmux-501/smolcorp-h1"
        XCTAssertNil(TestCopy.refusal(forTab: own, testCopy: true))
        for request in [["action": "press", "key": "enter", "tab": own], ["action": "close", "agent": "claude", "tab": own]] as [[String: Any]] {
            XCTAssertEqual(try answer(request, copy(true))["ok"] as? Bool, true, "\(request)")
        }
        XCTAssertEqual(asked.count, 2, "its own pane took both: \(asked)")
        XCTAssertTrue(asked.first?.hasPrefix("press ") == true && asked.first?.hasSuffix(own) == true, "\(asked)")
        XCTAssertEqual(asked.last, "fire claude \(own)")
        // The real app: every tab, and the jukebox and dictation, as before.
        asked = []
        XCTAssertEqual(try answer(["action": "press", "key": "enter", "tab": "iterm:AB12-CD34"], copy(false))["ok"] as? Bool, true)
        XCTAssertEqual(asked.count, 1)
        XCTAssertEqual(try answer(["action": "music-state"], copy(false))["ok"] as? Bool, true)
        XCTAssertEqual(try answer(["action": "volume", "level": 30], copy(false))["ok"] as? Bool, true)
        XCTAssertEqual(try answer(["action": "dictation", "language": "en_US"], copy(false))["ok"] as? Bool, true)
        XCTAssertEqual(told, ["music-state", "volume 30", "dictation en_US"])
    }

    /// RFXC1a N5: the facts say how this Mac writes a short time, so the core writes a webhook's delivery time as the Mac does.
    /// RWTC N7: how the Mac writes a short time is asked alone (`time-format`), not with the whole facts.
    func testTheMacSaysHowItWritesAShortTime() throws {
        let english = HostBridge.shortTimeFormat(locale: Locale(identifier: "en_US"))
        XCTAssertTrue(try XCTUnwrap(english["pattern"]).hasPrefix("h:mm"), "\(english)")
        XCTAssertEqual(english["am"], "AM")
        XCTAssertEqual(english["pm"], "PM")
        XCTAssertEqual(HostBridge.shortTimeFormat(locale: Locale(identifier: "fr_FR"))["pattern"], "HH:mm")
        var mac = actions
        mac.timeFormat = { HostBridge.shortTimeFormat(locale: Locale(identifier: "en_GB")) }
        mac.facts = { [unowned self] _ in asked.append("facts"); return [:] }
        let line = try JSONSerialization.data(withJSONObject: ["token": token, "action": "time-format"])
        let answered = HostBridge.answer(line, token: token, actions: mac)
        XCTAssertEqual((answered["timeFormat"] as? [String: String])?["pattern"], "HH:mm")
        XCTAssertFalse(asked.contains("facts"), "the facts aren't read for it")
    }

    func testNothingWithoutTheToken() {
        XCTAssertFalse(isOK(ask(["action": "hello"], token: "wrong-token-wrong-token-wrong-tok")))
        XCTAssertFalse(isOK(ask(["action": "hello"], token: "")))
        XCTAssertFalse(isOK(HostBridge.answer(Data(#"{"action":"hello"}"#.utf8), token: token, actions: actions)))
        XCTAssertFalse(isOK(HostBridge.answer(Data("not json".utf8), token: token, actions: actions)))
        XCTAssertFalse(isOK(ask(["action": "focus", "tab": "tmux:%1@/tmp/s"], token: String(token.dropLast()))))
        XCTAssertTrue(asked.isEmpty, "nothing was done")
        XCTAssertTrue(isOK(ask(["action": "hello"])))
        XCTAssertTrue(HostBridge.sameToken(token, token))
        XCTAssertFalse(HostBridge.sameToken("", ""))
    }

    func testTheKeysAreTheDecksOnly() {
        XCTAssertTrue(isOK(ask(["action": "press", "tab": "tty:/dev/ttys003", "key": "escape", "times": 2])))
        XCTAssertTrue(isOK(ask(["action": "press", "tab": "tty:/dev/ttys003", "key": "space"])))
        XCTAssertTrue(isOK(ask(["action": "press", "tab": "tty:/dev/ttys003", "key": "shift-tab"])))
        XCTAssertEqual(asked, ["press escape 0 2 tty:/dev/ttys003", "press space 0 1 tty:/dev/ttys003",
                               "press tab \(CGEventFlags.maskShift.rawValue) 1 tty:/dev/ttys003"])
        for refused: [String: Any] in [
            ["action": "press", "tab": "tty:/dev/ttys003", "key": "cmd-q"],
            ["action": "press", "tab": "tty:/dev/ttys003", "key": "enter", "times": 10],
            ["action": "press", "tab": "tty:/dev/ttys003", "key": "enter", "times": 0],
            ["action": "press", "tab": "/etc/passwd", "key": "enter"],
            ["action": "press", "tab": "tty:/dev/ttys003\nx", "key": "enter"],
            ["action": "type", "tab": "tty:/dev/ttys003", "text": "rm -rf ~\n"],
            ["action": "type", "tab": "tty:/dev/ttys003", "text": ""],
            ["action": "type", "tab": "tty:/dev/ttys003", "text": String(repeating: "a", count: HostBridge.textMost + 1)],
            ["action": "scroll", "tab": "tty:/dev/ttys003", "lines": 0],
            ["action": "close", "tab": "tmux:%1@/tmp/s", "agent": "bash"],
            ["action": "notify", "text": "hello"],
            ["action": "dictate"],
        ] {
            XCTAssertFalse(isOK(ask(refused)), "\(refused)")
        }
        XCTAssertEqual(asked.count, 3, "none of those reached a terminal")
    }

    func testEachTabActionIsTheDrivers() {
        XCTAssertTrue(isOK(ask(["action": "type", "tab": "iterm:BD22-06F6", "text": "hello there"])))
        let notTaken = ask(["action": "insert", "tab": "iterm:BD22-06F6", "text": "hello"])
        XCTAssertFalse(isOK(notTaken), "the terminal didn't take it")
        XCTAssertNil(notTaken["unsupported"])
        XCTAssertTrue(isOK(ask(["action": "focus", "tab": "tmux:%1@/tmp/s"])))
        let cant = ask(["action": "focus", "tab": "ghostty:/dev/ttys004"])
        XCTAssertEqual(cant["error"] as? String, "This terminal can't do that.")
        XCTAssertEqual(cant["unsupported"] as? Bool, true, "the core's errUnsupported, not a tab that's gone")
        XCTAssertTrue(isOK(ask(["action": "scroll", "tab": "tmux:%1@/tmp/s", "lines": -5000])))
        XCTAssertTrue(isOK(ask(["action": "scroll-bottom", "tab": "tmux:%1@/tmp/s"])))
        XCTAssertEqual(asked, ["type hello there iterm:BD22-06F6", "insert hello iterm:BD22-06F6", "focus tmux:%1@/tmp/s",
                               "focus ghostty:/dev/ttys004", "scroll -\(Terminal.maxScroll) tmux:%1@/tmp/s", "bottom tmux:%1@/tmp/s"])
    }

    func testCloseSaysWhatTheFiringDid() {
        XCTAssertEqual(ask(["action": "close", "tab": "tmux:%1@/tmp/s", "agent": "claude"])["fired"] as? String, "fired")
        XCTAssertEqual(ask(["action": "close", "tab": "tmux:%2@/tmp/s", "agent": "codex"])["fired"] as? String, "gone")
        XCTAssertEqual(ask(["action": "close", "tab": "tmux:%3@/tmp/s", "agent": "grok"])["fired"] as? String, "not-stopped")
        let none = ask(["action": "close", "tab": "tmux:%4@/tmp/s", "agent": "claude"])
        XCTAssertFalse(isOK(none), "no driver")
        XCTAssertEqual(none["unsupported"] as? Bool, true)
    }

    /// A strict close (FXHIRE B1 a, a hire in a window of its own): the tab is fired only with its agent in front;
    /// a shell alone there (a tty used again), the agent behind, or a tab that can't be looked at, is never touched.
    func testAStrictCloseFiresOnlyAnAgentInFront() {
        var mac = actions
        mac.place = { tab, _ in ["%1": .front, "%2": .gone, "%3": .behind][tab.id] ?? nil }
        func close(_ tab: String, strict: Bool = true) -> String? {
            var request: [String: Any] = ["action": "close", "tab": tab, "agent": "claude", "token": token]
            if strict { request["strict"] = true }
            return HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: mac)["fired"] as? String
        }
        XCTAssertEqual(close("tmux:%1@/tmp/s"), "fired")
        XCTAssertEqual(close("tmux:%2@/tmp/s"), "gone", "only a shell left: answered gone")
        XCTAssertEqual(close("tmux:%3@/tmp/s"), "not-stopped")
        XCTAssertEqual(close("tmux:%4@/tmp/s"), "not-stopped", "unknown is not the same as none")
        XCTAssertEqual(asked, ["fire claude tmux:%1@/tmp/s"], "only the agent in front was fired")
        XCTAssertEqual(close("tmux:%2@/tmp/s", strict: false), "gone")
        XCTAssertEqual(asked.last, "fire claude tmux:%2@/tmp/s", "without strict, the close is the office's as before")
    }

    /// Whether each desk's agent has left its tab (FXHIRE B1 b): asked in order, read only; a test copy never asks
    /// iTerm2 or Ghostty, only `ps`.
    func testAgentsEndedAreAnsweredInOrderAndATestCopyAsksNoOtherApp() {
        var mac = actions
        var looked: [String] = []
        mac.ended = { tab, agent, testCopy in looked.append("\(tab.text) \(agent.rawValue) \(testCopy)"); return tab.id == "/dev/ttys005" }
        func ask(_ request: [String: Any]) -> [String: Any] {
            var request = request
            request["action"] = "agents-ended"
            request["token"] = token
            return HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: mac)
        }
        let answer = ask(["tabs": ["tty:/dev/ttys005", "iterm:AB12", "not a tab"], "agents": ["claude", "codex", "claude"]])
        XCTAssertEqual(answer["ended"] as? [Bool], [true, false, false])
        XCTAssertEqual(looked, ["tty:/dev/ttys005 claude false", "iterm:AB12 codex false"], "a tab that isn't one is never looked at")
        XCTAssertFalse(isOK(ask(["tabs": ["tty:/dev/ttys005"], "agents": []])), "a tab without its agent")
        XCTAssertFalse(isOK(ask(["tabs": "tty:/dev/ttys005", "agents": ["claude"]])))

        // The Mac's own rule: iTerm2 asked only outside a test copy.
        var opens: [String] = []
        let standIn = PhoneTerminal(pastes: .pasted) { _ in }
        standIn.open = { opens.append($0.text); return false }
        let saved = Terminals.standIn
        Terminals.standIn = standIn
        defer { Terminals.standIn = saved }
        let iterm = TabAddress("iterm:AB12")!
        XCTAssertFalse(HostBridge.agentEnded(iterm, .claude, testCopy: true))
        XCTAssertEqual(opens, [], "a test copy never asks another terminal app")
        XCTAssertTrue(HostBridge.agentEnded(iterm, .claude, testCopy: false), "closed")
        XCTAssertEqual(opens, ["iterm:AB12"])
    }

    /// RHIRE2 nit 1, as the Swift engine's `Deck.pruneClosedTabs`: a Terminal tab is asked of `ps` only once it has left
    /// Terminal's own tab list, read once for all; unknown is not gone, and a test copy never reads that list. A tab
    /// Smolcorp can only watch, its address a tty, is judged by `ps` (`pruneClosedPanes`), in a test copy too.
    func testATerminalTabIsAskedOfPsOnlyOnceTerminalNoLongerListsIt() {
        var mac = actions
        var looked: [String] = [], listed = 0
        mac.ended = { tab, _, _ in looked.append(tab.text); return true }
        mac.openTTYs = { listed += 1; return ["/dev/ttys004"] }
        func ask() -> [Bool]? {
            let request: [String: Any] = ["action": "agents-ended", "token": token,
                                          "tabs": ["tty:/dev/ttys004", "tty:/dev/ttys005", "iterm:AB12"], "agents": ["claude", "claude", "codex"]]
            return HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: mac)["ended"] as? [Bool]
        }
        XCTAssertEqual(ask(), [false, true, true], "ttys004 is still one of Terminal's tabs: its agent is kept, whatever ps says")
        XCTAssertEqual(looked, ["tty:/dev/ttys005", "iterm:AB12"])
        XCTAssertEqual(listed, 1, "Terminal is asked once for all its tabs")
        looked = []
        mac.openTTYs = { listed += 1; return nil }
        XCTAssertEqual(ask(), [false, false, true], "Terminal not answering: unknown is not gone")
        XCTAssertEqual(looked, ["iterm:AB12"])
        looked = []
        listed = 0
        mac.testCopy = true
        XCTAssertEqual(ask(), [false, false, true])
        XCTAssertEqual(listed, 0, "a test copy never reads Terminal's tab list")
        XCTAssertEqual(looked, ["iterm:AB12"])

        // Nothing runs on a tty no tab has: an agent there has left it, by ps alone (no other app asked).
        XCTAssertTrue(HostBridge.agentEnded(TabAddress("other:/dev/ttys987")!, .claude, testCopy: true))
        XCTAssertFalse(HostBridge.agentEnded(TabAddress("tmux:%3@/tmp/tmux-501/default")!, .claude, testCopy: true), "a pane is no tty: unknown")
    }

    /// The core gives up at 20 s: an action that hasn't started by the bridge's deadline is never run.
    func testATabActionTheCoreGaveUpOnIsNeverRunLate() {
        // A slow AppleScript ahead of it holds the terminals' queue.
        let release = DispatchSemaphore(value: 0)
        terminals.async { release.wait() }
        let line = try! JSONSerialization.data(withJSONObject: ["token": token, "action": "type", "tab": "tty:/dev/ttys003", "text": "hello"])
        let late = HostBridge.answer(line, token: token, actions: actions, until: .now() + 0.2)
        XCTAssertEqual(late["error"] as? String, "Terminal isn't answering, so nothing was typed. Try again in a moment.")
        release.signal()
        terminals.sync {}
        XCTAssertTrue(asked.isEmpty, "not typed once the queue was free: the core had given up")
        // In time, it runs.
        XCTAssertTrue(isOK(HostBridge.answer(line, token: token, actions: actions, until: .now() + 5)))
        XCTAssertEqual(asked, ["type hello tty:/dev/ttys003"])
        XCTAssertLessThan(HostBridge.actionWait, 20, "under the core's own wait (desktop/hostbridge_darwin.go)")
        // Running, it's waited for; done, its answer is given.
        if case .done(let value) = HostBridge.run(on: terminals, until: .now() + 5, { 42 }) { XCTAssertEqual(value, 42) } else { XCTFail("not run") }
    }

    /// A tab action runs on its own terminal's queue, as the deck's keys do: a tmux tab's never waits behind
    /// Terminal.app, and one its terminal doesn't get to within the deck's input wait is never run.
    func testATabActionWaitsOnlyOnItsOwnTerminal() {
        let tmux = DispatchQueue(label: "test.tmux")
        var actions = self.actions
        actions.queue = { [terminals] tab in tab?.kind == "tmux" ? tmux : terminals }
        let release = DispatchSemaphore(value: 0)
        terminals.async { release.wait() } // Terminal.app not answering
        func line(_ tab: String) -> Data {
            try! JSONSerialization.data(withJSONObject: ["token": token, "action": "type", "tab": tab, "text": "hello"])
        }
        let started = Date()
        XCTAssertTrue(isOK(HostBridge.answer(line("tmux:%1@/tmp/s"), token: token, actions: actions, until: .now() + HostBridge.actionWait)))
        XCTAssertLessThan(Date().timeIntervalSince(started), 1)
        XCTAssertEqual(asked, ["type hello tmux:%1@/tmp/s"])
        // Terminal.app's: given the deck's input wait to start, not the bridge's whole 18 s.
        let late = HostBridge.answer(line("tty:/dev/ttys003"), token: token, actions: actions, until: .now() + HostBridge.actionWait)
        XCTAssertEqual(late["error"] as? String, "Terminal isn't answering, so nothing was typed. Try again in a moment.")
        XCTAssertLessThan(Date().timeIntervalSince(started), Terminals.inputWait + 1.5)
        release.signal()
        terminals.sync {}
        XCTAssertEqual(asked, ["type hello tmux:%1@/tmp/s"], "never typed once Terminal was free")
        XCTAssertTrue(Terminals.inputWait < HostBridge.actionWait)
    }

    /// A tab action is given the deck's input wait to start, not to finish: one
    /// that starts in time is waited for to its end and answered as it went, as
    /// a close that takes longer than that; one that never starts is refused and
    /// never runs. The core is never told "failed" for keys that went.
    func testATabActionStartedInTimeIsAnsweredAsItWent() {
        let tmux = DispatchQueue(label: "test.tmux"), iterm = DispatchQueue(label: "test.iterm")
        let log = NSLock()
        var actions = self.actions
        actions.queue = { [terminals] tab in ["tmux": tmux, "iterm": iterm][tab?.kind ?? ""] ?? terminals }
        actions.type = { [unowned self] text, tab in
            Thread.sleep(forTimeInterval: 1) // its keys take a while
            log.lock(); asked.append("type \(text) \(tab.text)"); log.unlock()
            return true
        }
        actions.fire = { [unowned self] tab, agent in
            Thread.sleep(forTimeInterval: Terminals.inputWait + 1) // stopping the agent, then the tab closed
            log.lock(); asked.append("fire \(agent.rawValue) \(tab.text)"); log.unlock()
            return .fired
        }
        func line(_ request: [String: Any]) -> Data { try! JSONSerialization.data(withJSONObject: request.merging(["token": token]) { $1 }) }
        terminals.async { Thread.sleep(forTimeInterval: Terminals.inputWait - 0.5) } // Terminal.app slow, but in time
        let release = DispatchSemaphore(value: 0)
        tmux.async { release.wait() } // tmux not answering at all
        var answers: [String: [String: Any]] = [:]
        let group = DispatchGroup()
        let started = Date()
        for (name, request) in [
            ("slow", ["action": "type", "tab": "tty:/dev/ttys003", "text": "hello"]),
            ("never", ["action": "type", "tab": "tmux:%1@/tmp/s", "text": "hello"]),
            ("close", ["action": "close", "tab": "iterm:AB12", "agent": "claude"]),
        ] as [(String, [String: Any])] {
            DispatchQueue.global().async(group: group) { [token, actions] in
                let answer = HostBridge.answer(line(request), token: token, actions: actions, until: .now() + HostBridge.actionWait)
                log.lock(); answers[name] = answer; log.unlock()
            }
        }
        group.wait()
        XCTAssertLessThan(Date().timeIntervalSince(started), HostBridge.actionWait - 5)
        XCTAssertTrue(isOK(answers["slow"] ?? [:]), "\(answers["slow"] ?? [:])")
        XCTAssertEqual(answers["close"]?["fired"] as? String, "fired")
        XCTAssertEqual(answers["never"]?["error"] as? String, "tmux isn't answering, so nothing was typed. Try again in a moment.")
        release.signal()
        tmux.sync {}
        XCTAssertEqual(Set(asked), ["type hello tty:/dev/ttys003", "fire claude iterm:AB12"], "the tmux one never ran")

        // Still running at the bridge's own deadline: said to have stopped part way, not "nothing typed".
        if case .unfinished = HostBridge.run(on: terminals, startBy: .now() + 1, until: .now() + 0.2, { Thread.sleep(forTimeInterval: 0.5) }) {} else {
            XCTFail("not unfinished")
        }
        // A script that timed out once something may have gone: the deck's own words.
        actions.type = { _, _ in Tool.run("/bin/sleep", ["5"], timeout: 0.2) != nil }
        let partWay = HostBridge.answer(line(["action": "type", "tab": "iterm:AB12", "text": "hello"]), token: token, actions: actions,
                                        until: .now() + 5)
        XCTAssertEqual(partWay["error"] as? String, "iTerm2 stopped answering part way. Look at the desk before you try again.")
        actions.type = { _, _ in Terminal.onlyLooking { Tool.run("/bin/sleep", ["5"], timeout: 0.2) != nil } }
        let nothing = HostBridge.answer(line(["action": "type", "tab": "iterm:AB12", "text": "hello"]), token: token, actions: actions,
                                        until: .now() + 5)
        XCTAssertEqual(nothing["error"] as? String, "iTerm2 isn't answering, so nothing was typed. Try again in a moment.")
    }

    /// A window, as a tab action: on the terminals' queue, and never once the core has given up,
    /// so a retry doesn't open a second one.
    func testAWindowTheCoreGaveUpOnIsNeverOpened() throws {
        let script = hires.appendingPathComponent("hire-1.sh")
        try Data("#!/bin/sh\n".utf8).write(to: script)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
        let line = try JSONSerialization.data(withJSONObject: ["token": token, "action": "open", "terminal": "iterm",
                                                               "folder": folder.path, "script": script.path])
        let release = DispatchSemaphore(value: 0)
        windows.async { release.wait() }
        let late = HostBridge.answer(line, token: token, actions: actions, until: .now() + 0.2)
        XCTAssertEqual(late["error"] as? String, "The terminal didn't answer in time.")
        release.signal()
        windows.sync {}
        XCTAssertTrue(asked.isEmpty, "not opened once the queue was free")
        var onQueue = false
        var actions = self.actions
        actions.open = { [windows] _, _, _ in
            dispatchPrecondition(condition: .onQueue(windows))
            onQueue = true
            return Openers.Opened(tab: "iterm:AB12", window: "iterm:AB12")
        }
        XCTAssertEqual(HostBridge.answer(line, token: token, actions: actions, until: .now() + 5)["tab"] as? String, "iterm:AB12")
        XCTAssertTrue(onQueue, "opened on the window's own queue, not a tab's")
        XCTAssertEqual(windowQueues, ["iterm", "iterm"], "the queue of the terminal the window opens in")
    }

    /// Hires switched to the background under the core (desktop/hirewindows_close.go): only a window this app opened
    /// for the core, by the record `open` answered, is looked at or closed, on its terminal's queue, by the closer
    /// (here a stand-in: no real terminal is asked anything); what the closer says is answered, and a window closed
    /// or gone is forgotten.
    func testOnlyAWindowOpenedForTheCoreIsLookedAtOrClosed() throws {
        final class StandIn: HireWindows.Closer {
            var asked: [String] = []
            var shown = HireWindows.Shown.open("/dev/ttys007")
            var closed = HireWindows.Closed.left("something still runs in it")
            func look(_ window: String) -> HireWindows.Shown { asked.append("look \(window)"); return shown }
            func close(_ window: String, tty: String) -> HireWindows.Closed { asked.append("close \(window) \(tty)"); return closed }
        }
        let closer = StandIn()
        var actions = self.actions
        actions.closer = closer
        func send(_ request: [String: Any]) -> [String: Any] {
            var request = request
            request["token"] = token
            return HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: actions)
        }
        let record = "iterm:AB12" // what the stand-in opener answers
        for window in [record, "tty:/dev/ttys004#w12", "nonsense"] {
            XCTAssertFalse(isOK(send(["action": "window-look", "window": window])), "\(window), before it was opened for the core")
        }
        XCTAssertTrue(closer.asked.isEmpty, "nothing looked at")
        let script = hires.appendingPathComponent("hire-1.sh")
        try Data("#!/bin/sh\n".utf8).write(to: script)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
        XCTAssertEqual(send(["action": "open", "terminal": "iterm", "folder": folder.path, "script": script.path])["window"] as? String, record)
        let look = send(["action": "window-look", "window": record])
        XCTAssertEqual(look["shown"] as? String, "open")
        XCTAssertEqual(look["tty"] as? String, "/dev/ttys007")
        XCTAssertEqual(send(["action": "window-close", "window": record, "tty": "not a tty"])["error"] as? String, "Not a tty.")
        let left = send(["action": "window-close", "window": record, "tty": "/dev/ttys007"])
        XCTAssertEqual(left["closed"] as? Bool, false)
        XCTAssertEqual(left["left"] as? String, "something still runs in it")
        closer.closed = .closed
        XCTAssertEqual(send(["action": "window-close", "window": record, "tty": "/dev/ttys007"])["closed"] as? Bool, true)
        XCTAssertEqual(send(["action": "window-look", "window": record])["error"] as? String, "Not a window Smolcorp opened for the engine.",
                       "a closed window is forgotten")
        XCTAssertEqual(closer.asked, ["look \(record)", "close \(record) /dev/ttys007", "close \(record) /dev/ttys007"])
        XCTAssertEqual(windowQueues, ["iterm", "iterm", "iterm", "iterm"], "each on the window's terminal's queue")
        // A window gone is forgotten too.
        XCTAssertTrue(isOK(send(["action": "open", "terminal": "iterm", "folder": folder.path, "script": script.path])))
        closer.shown = .gone
        XCTAssertEqual(send(["action": "window-look", "window": record])["shown"] as? String, "gone")
        XCTAssertFalse(isOK(send(["action": "window-look", "window": record])), "a gone window is forgotten")
    }

    /// A test copy looks at and closes no window (RMSY4 nit 5), not even one recorded as opened for the core: the
    /// terminal's sentence, and the closer is asked nothing.
    func testATestCopyLooksAtAndClosesNoWindow() {
        final class StandIn: HireWindows.Closer {
            var asked: [String] = []
            func look(_ window: String) -> HireWindows.Shown { asked.append("look \(window)"); return .open("/dev/ttys007") }
            func close(_ window: String, tty: String) -> HireWindows.Closed { asked.append("close \(window)"); return .closed }
        }
        let closer = StandIn()
        var copy = actions
        copy.closer = closer
        copy.testCopy = true
        let record = "iterm:AB12"
        copy.opened.add(record)
        for request in [["action": "window-look", "window": record], ["action": "window-close", "window": record, "tty": "/dev/ttys007"]] {
            var request = request
            request["token"] = token
            let answer = HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: copy)
            XCTAssertFalse(isOK(answer), request["action"] ?? "")
            XCTAssertEqual(answer["error"] as? String, TestCopy.OtherApp.terminal.sentence, request["action"] ?? "")
        }
        XCTAssertEqual(closer.asked, [], "no terminal was asked")
        // The real app looks at the same window, as before.
        copy.testCopy = false
        let look = HostBridge.answer(try! JSONSerialization.data(withJSONObject: ["action": "window-look", "window": record, "token": token]),
                                     token: token, actions: copy)
        XCTAssertEqual(look["shown"] as? String, "open")
        XCTAssertEqual(closer.asked, ["look \(record)"])
    }

    func testATerminalOpensOnlyOnAHireScript() throws {
        let script = hires.appendingPathComponent("abc/hire-1.sh")
        try FileManager.default.createDirectory(at: script.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("#!/bin/sh\n".utf8).write(to: script)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
        let open: [String: Any] = ["action": "open", "terminal": "iterm", "folder": folder.path, "script": script.path]
        let answer = ask(open)
        XCTAssertEqual(answer["tab"] as? String, "iterm:AB12")
        XCTAssertEqual(asked, ["open iterm \(folder.path) \(script.path)"])

        let outside = folder.appendingPathComponent("hire.sh")
        try Data("#!/bin/sh\n".utf8).write(to: outside)
        let link = hires.appendingPathComponent("link.sh")
        try FileManager.default.createSymbolicLink(at: link, withDestinationURL: outside)
        let shared = hires.appendingPathComponent("shared.sh")
        try Data("#!/bin/sh\n".utf8).write(to: shared)
        try FileManager.default.setAttributes([.posixPermissions: 0o772], ofItemAtPath: shared.path)
        for (key, value) in [("script", outside.path), ("script", link.path), ("script", shared.path),
                             ("script", hires.path + "/../hire.sh"), ("script", "echo hi; rm -rf ~"), ("script", "abc/hire-1.sh"),
                             ("terminal", "xterm"), ("folder", "relative"), ("folder", script.path)] {
            var request = open
            request[key] = value
            XCTAssertFalse(isOK(ask(request)), "\(key) \(value)")
        }
        XCTAssertEqual(asked.count, 1, "no other window opened")

        // A folder inside hires that's a link to somewhere else: the script behind it isn't a hire's.
        let elsewhere = folder.appendingPathComponent("elsewhere", isDirectory: true)
        try FileManager.default.createDirectory(at: elsewhere, withIntermediateDirectories: true)
        let behind = elsewhere.appendingPathComponent("hire-2.sh")
        try Data("#!/bin/sh\n".utf8).write(to: behind)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: behind.path)
        try FileManager.default.createSymbolicLink(at: hires.appendingPathComponent("linked"), withDestinationURL: elsewhere)
        XCTAssertFalse(HostBridge.isHireScript(hires.appendingPathComponent("linked/hire-2.sh").path, in: hires))
        XCTAssertTrue(HostBridge.isHireScript(script.path, in: hires))
    }

    /// The opener gets the script's path as it is, as the deck's hire gives it: each
    /// terminal's AppleScript quotes it itself, and a path quoted twice never runs.
    func testTheOpenerIsGivenTheRawPath() throws {
        let spaced = hires.appendingPathComponent("hire dir/it's.sh")
        try FileManager.default.createDirectory(at: spaced.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("#!/bin/sh\n".utf8).write(to: spaced)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: spaced.path)
        var opened: [String] = []
        // Not a test copy: a test copy opens no window of a terminal (RTCH N1), which testATestCopyRefusesWhatWouldBringAnotherAppForward checks.
        var actions = HostBridge.Actions.onThisMac(testCopy: false, opener: { command, folder, terminal in
            opened.append("\(terminal)|\(folder)|\(command)")
            return Openers.Opened(tab: "iterm:AB12", window: nil)
        })
        actions.hires = hires
        let line = try JSONSerialization.data(withJSONObject: ["token": token, "action": "open", "terminal": "iterm",
                                                               "folder": folder.path, "script": spaced.path])
        XCTAssertEqual(HostBridge.answer(line, token: token, actions: actions)["tab"] as? String, "iterm:AB12")
        XCTAssertEqual(opened, ["iterm|\(folder.path)|\(spaced.path)"], "not \(Openers.quoted(spaced.path))")
    }

    /// A hire's window, as the deck records it (T796): the tab the deck knows, and the window's own record,
    /// which for Terminal carries the window's id beside the tty. A tab with no window of its own gives none.
    func testAnOpenedWindowIsAnsweredWithItsRecord() throws {
        let script = hires.appendingPathComponent("hire-3.sh")
        try Data("#!/bin/sh\n".utf8).write(to: script)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
        var actions = self.actions
        let line = try JSONSerialization.data(withJSONObject: ["token": token, "action": "open", "terminal": "terminal",
                                                               "folder": folder.path, "script": script.path])
        actions.open = { _, _, _ in Openers.Opened(tab: "tty:/dev/ttys004", window: HireWindows.terminalRecord(tty: "/dev/ttys004", windowID: "1234")) }
        let answer = HostBridge.answer(line, token: token, actions: actions)
        XCTAssertEqual(answer["tab"] as? String, "tty:/dev/ttys004")
        XCTAssertEqual(answer["window"] as? String, "tty:/dev/ttys004#w1234")
        actions.open = { _, _, _ in Openers.Opened(tab: "tty:/dev/ttys005", window: nil) }
        let bare = HostBridge.answer(line, token: token, actions: actions)
        XCTAssertEqual(bare["tab"] as? String, "tty:/dev/ttys005")
        XCTAssertNil(bare["window"], "no window record when the terminal gave none")
    }

    /// Which of the three terminals are here: what the core offers as setup's choices.
    func testTheInstalledTerminalsAreSaid() {
        var actions = self.actions
        actions.terminals = { ["terminal", "ghostty"] }
        let line = try! JSONSerialization.data(withJSONObject: ["token": token, "action": "terminals"])
        let answer = HostBridge.answer(line, token: token, actions: actions)
        XCTAssertTrue(isOK(answer))
        XCTAssertEqual(answer["terminals"] as? [String], ["terminal", "ghostty"])
        XCTAssertEqual(HostBridge.answer(line, token: "other-token-of-32-characters-long!", actions: actions)["ok"] as? Bool, false)
    }

    func testTheAppsOwnItemsOnly() {
        XCTAssertEqual(ask(["action": "secret-read", "item": "summary-key"])["value"] as? String, "fake-summary-key")
        XCTAssertTrue(isOK(ask(["action": "secret-write", "item": "summary-key", "value": "fake-other-key"])))
        XCTAssertEqual(kept["summary-key"], "fake-other-key")
        XCTAssertTrue(isOK(ask(["action": "secret-delete", "item": "summary-key"])))
        let none = ask(["action": "secret-read", "item": "summary-key"])
        XCTAssertTrue(isOK(none))
        XCTAssertEqual(none["found"] as? Bool, false)
        XCTAssertFalse(isOK(ask(["action": "secret-read", "item": "push-key"])), "not an item the engine may have")
        XCTAssertFalse(isOK(ask(["action": "secret-write", "item": "viewer-key", "value": "x"])), "not kept")
        XCTAssertFalse(isOK(ask(["action": "secret-write", "item": "summary-key", "value": "a\nb"])))
        XCTAssertFalse(isOK(ask(["action": "secret-delete", "item": "viewer-key"])))
        XCTAssertEqual(Set(HostBridge.Actions.mac.secrets.keys), ["summary-key", "gm-token", "viewer-id", "viewer-key", "viewer-write-secret"],
                       "as desktop/hostbridge.go's bridgeSecrets")
    }

    /// The portable office's items, as the Swift engine kept them (RM7 B1): none is "found": false, so the core
    /// makes the first id and key; one there but unreadable is the app's no, so the core never writes over it and
    /// every link given out keeps working; none is removed this way.
    func testTheViewersItemsSayNoneOnlyWhenThereIsNone() {
        XCTAssertEqual(ask(["action": "secret-read", "item": "viewer-id"])["value"] as? String, "AbCdEfGhIjKlMnOpQrStUv")
        let none = ask(["action": "secret-read", "item": "viewer-key"])
        XCTAssertTrue(isOK(none))
        XCTAssertEqual(none["found"] as? Bool, false)
        let unreadable = ask(["action": "secret-read", "item": "viewer-write-secret"])
        XCTAssertFalse(isOK(unreadable))
        XCTAssertEqual(unreadable["error"] as? String, "It couldn't be read.")
        XCTAssertNil(unreadable["value"])
        XCTAssertTrue(isOK(ask(["action": "secret-write", "item": "viewer-id", "value": "ZyXwVuTsRqPoNmLkJiHgFe"])))
        XCTAssertEqual(kept["viewer-id"], "ZyXwVuTsRqPoNmLkJiHgFe")
        for item in ["viewer-id", "viewer-key", "viewer-write-secret"] {
            XCTAssertEqual(ask(["action": "secret-delete", "item": item])["error"] as? String, "That item isn't removed this way.", item)
        }
        XCTAssertEqual(kept["viewer-id"], "ZyXwVuTsRqPoNmLkJiHgFe", "not removed")
        // The Mac's own: the Keychain's "not found" alone is none (ViewerKeychain.absent), in a test copy no file.
        for item in ["viewer-id", "viewer-key", "viewer-write-secret"] {
            XCTAssertNotNil(HostBridge.Actions.mac.secrets[item]?.absent, item)
        }
    }

    /// When you're back at the Mac (RM7 B2), as the Swift engine's portable office watched it: the screen's unlocks
    /// and the Mac's and its screens' wakes are counted from the start, and the idle time and the lock read when
    /// asked. On centres of the test's own: nothing is posted to the Mac's.
    func testWhereYouAreIsCountedFromTheStartAndReadWhenAsked() {
        let distributed = NotificationCenter(), workspace = NotificationCenter()
        var idle: TimeInterval = 42, locked = true
        let presence = MacPresence(distributed: distributed, workspace: workspace, idle: { idle }, locked: { locked })
        let unlock = Notification.Name("com.apple.screenIsUnlocked")
        distributed.post(name: unlock, object: nil)
        XCTAssertEqual(presence.now(), MacPresence.Now(idle: 42, locked: true, unlocks: 0, wakes: 0), "nothing before the start")
        presence.start()
        presence.start()
        distributed.post(name: unlock, object: nil)
        workspace.post(name: NSWorkspace.didWakeNotification, object: nil)
        workspace.post(name: NSWorkspace.screensDidWakeNotification, object: nil)
        workspace.post(name: NSWorkspace.willSleepNotification, object: nil)
        idle = 1.5
        locked = false
        XCTAssertEqual(presence.now(), MacPresence.Now(idle: 1.5, locked: false, unlocks: 1, wakes: 2), "started once, each counted once")

        // As the core is told: never used since the Mac started is a year, never infinity.
        var mac = actions
        mac.presence = { MacPresence.Now(idle: .infinity, locked: true, unlocks: 1, wakes: 2) }
        let line = try! JSONSerialization.data(withJSONObject: ["token": token, "action": "presence"])
        let answer = HostBridge.answer(line, token: token, actions: mac)
        XCTAssertTrue(isOK(answer))
        XCTAssertEqual(answer["idle"] as? TimeInterval, MacPresence.idleMost)
        XCTAssertEqual(answer["locked"] as? Bool, true)
        XCTAssertEqual(answer["unlocks"] as? Int, 1)
        XCTAssertEqual(answer["wakes"] as? Int, 2)
        XCTAssertNotNil(try? JSONSerialization.data(withJSONObject: answer))
    }

    /// The GM's token the Keychain has but couldn't read is said as such, never as "none", so the core neither makes
    /// a new one nor writes over it (RTOK N2); one it read, or none, is said as before.
    func testTheGMsTokenTheKeychainCouldntReadIsSaidSo() {
        var actions = self.actions
        var held: GMTokenRead = .failed
        actions.secrets["gm-token"] = HostBridge.Secret(read: { nil }, write: { _ in true }, delete: {}, look: { held })
        func read() -> [String: Any] {
            let line = try! JSONSerialization.data(withJSONObject: ["token": token, "action": "secret-read", "item": "gm-token"])
            return HostBridge.answer(line, token: token, actions: actions)
        }
        let failed = read()
        XCTAssertTrue(isOK(failed))
        XCTAssertEqual(failed["failed"] as? Bool, true)
        XCTAssertEqual(failed["found"] as? Bool, false)
        XCTAssertNil(failed["value"])
        held = .none
        XCTAssertEqual(read()["found"] as? Bool, false)
        XCTAssertNil(read()["failed"])
        held = .token("fake-gm-token-from-1-24-0000000000000000000")
        XCTAssertEqual(read()["value"] as? String, "fake-gm-token-from-1-24-0000000000000000000")
        XCTAssertNil(read()["failed"])
    }

    func testTheLoginItem() {
        XCTAssertEqual(ask(["action": "login-item"])["on"] as? Bool, false)
        XCTAssertEqual(ask(["action": "login-item", "on": true])["on"] as? Bool, true)
        XCTAssertTrue(login)
    }

    /// A test copy never moves its bundle (FXC1d N-2): `install` on `onThisMac(testCopy: true)` is refused in the
    /// Mac's words before `SelfTest.moveToApplications` is reached, whatever `TestCopy.isOn` says in a test run.
    /// Only the test copy's side is asked here: the other would move this test's own bundle.
    func testATestCopyRefusesToMoveToApplications() {
        var copy = HostBridge.Actions.onThisMac(testCopy: true)
        copy.main = terminals // not the main queue, which this test is on
        var request: [String: Any] = ["action": "install", "token": token]
        let answer = HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: copy)
        XCTAssertFalse(isOK(answer))
        XCTAssertEqual(answer["error"] as? String, SelfTest.FixError.testCopy.localizedDescription)
        XCTAssertEqual(answer["error"] as? String, "A test copy stays where it was built.")
        request["action"] = "install-check"
        let line = HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: copy)
        XCTAssertEqual((line["check"] as? [String: Any])?["detail"] as? String, SelfTest.installCheck(testCopy: true).detail)
    }

    /// No `Actions` of a test run can move a bundle (FXC1e N-1): one built by hand refuses by default, and `.mac`
    /// (not a test copy, but in the test suite) refuses too, in the Mac's words, before /Applications is reached.
    func testNoActionsOfATestRunMovesToApplications() {
        // `actions` is built memberwise and leaves the mover alone; `.mac` is the real one, in a test run.
        for (name, base) in [("by hand", actions), ("mac", HostBridge.Actions.mac)] {
            var copy = base
            copy.main = terminals
            let request: [String: Any] = ["action": "install", "token": token]
            let answer = HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: copy)
            XCTAssertFalse(isOK(answer), name)
            XCTAssertEqual(answer["error"] as? String, "A test copy stays where it was built.", name)
        }
    }

    /// A test copy's login item is the copy in use's: the bridge reads it and never sets it, saying so in the
    /// Mac's words (SelfTest.testCopyLogin).
    func testATestCopyLeavesTheLoginItemAlone() {
        var set: [Bool] = []
        let item: (get: () -> Bool, set: (Bool) -> Void) = (get: { true }, set: { set.append($0) })
        let copy = HostBridge.Actions.onThisMac(testCopy: true, loginItem: item)
        func ask(_ request: [String: Any], _ actions: HostBridge.Actions) -> [String: Any] {
            var request = request
            request["token"] = token
            return HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: actions)
        }
        let read = ask(["action": "login-item"], copy)
        XCTAssertTrue(isOK(read))
        XCTAssertEqual(read["on"] as? Bool, true, "the copy in use's, read")
        for on in [false, true] {
            let answer = ask(["action": "login-item", "on": on], copy)
            XCTAssertFalse(isOK(answer))
            XCTAssertEqual(answer["error"] as? String, SelfTest.testCopyLogin)
        }
        XCTAssertEqual(set, [], "never set")
        XCTAssertEqual(SelfTest.testCopyLogin, "A test copy leaves the login item to the Smolcorp you use.")
        // The copy in use sets its own.
        XCTAssertTrue(isOK(ask(["action": "login-item", "on": false], HostBridge.Actions.onThisMac(testCopy: false, loginItem: item))))
        XCTAssertEqual(set, [false])
        // An app without one: nothing read or set.
        var none = copy
        none.loginItem = nil
        XCTAssertFalse(isOK(ask(["action": "login-item"], none)))
    }

    /// A test copy never records from the Mac's microphone (RGAPA B1): the office's mic is refused before any tab,
    /// driver or recording is looked at, in the test copy's words. Stand-ins only; nothing records.
    func testATestCopyNeverRecords() {
        var copy = actions
        copy.testCopy = true
        var recorded: [String] = []
        copy.startDictation = { language, tab, done in recorded.append("\(language) \(tab)"); done(true) }
        var request: [String: Any] = ["action": "dictation-start", "tab": "tty:/dev/ttys003", "language": "en_US", "token": token]
        let answer = HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: copy)
        XCTAssertFalse(isOK(answer))
        XCTAssertEqual(answer["error"] as? String, TestCopy.OtherApp.microphone.sentence)
        XCTAssertEqual(TestCopy.OtherApp.microphone.sentence, "This is a test copy of Smolcorp: it never asks for the microphone or turns dictation on.")
        // Not even a request the real app would refuse gets further.
        request["tab"] = "nonsense"
        let other = HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: copy)
        XCTAssertEqual(other["error"] as? String, TestCopy.OtherApp.microphone.sentence)
        XCTAssertEqual(recorded, [], "never recorded")
    }

    /// The office's window and the folder panel: each checked, then run on the main queue, as the Swift
    /// engine's routes run them. Nothing here shows a window or opens a panel: stand-ins.
    func testTheOfficeWindowAndTheFolderPanelAreTheShellsOwn() {
        var mac = actions
        mac.main = terminals
        var shown: [String] = []
        mac.showOffice = { shown.append("office") }
        mac.officeOnTop = { shown.append("on top \($0)") }
        mac.closeFolderPanel = { shown.append("close \($0 == Deck.hireFolderMessage ? "hires" : "floor")") }
        var opened: [String?] = []
        mac.chooseFolder = { start, _, done in opened.append(start); done("/Users/example/project") }
        func ask(_ request: [String: Any]) -> [String: Any] {
            var request = request
            request["token"] = token
            return HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: mac)
        }
        XCTAssertFalse(isOK(ask(["action": "office-ontop"])), "on or off, said")
        XCTAssertFalse(isOK(ask(["action": "office-ontop", "on": "yes"])))
        XCTAssertFalse(isOK(ask(["action": "choose-folder"])), "for what, said")
        XCTAssertFalse(isOK(ask(["action": "choose-folder", "for": "anything"])))
        XCTAssertFalse(isOK(ask(["action": "choose-folder", "for": "hires", "folder": "relative/path"])))
        XCTAssertFalse(isOK(ask(["action": "choose-folder", "for": "hires", "folder": "/Users/a\nb"])))
        XCTAssertEqual(shown, [])
        XCTAssertEqual(opened, [])
        XCTAssertEqual(ask(["action": "choose-folder", "for": "hires", "folder": ""])["folder"] as? String, "/Users/example/project")
        XCTAssertEqual(opened, [nil], "an empty folder: the panel's own choice")
        XCTAssertTrue(isOK(ask(["action": "office-ontop", "on": false])))
        XCTAssertEqual(shown, ["on top false"])

        // A panel left open past its time: closed as if cancelled, and nothing picked.
        let wait = HostBridge.panelWait
        HostBridge.panelWait = 0.2
        defer { HostBridge.panelWait = wait }
        mac.chooseFolder = { _, _, _ in }
        let left = ask(["action": "choose-folder", "for": "floor"])
        XCTAssertTrue(isOK(left))
        XCTAssertNil(left["folder"])
        terminals.sync {}
        XCTAssertEqual(shown, ["on top false", "close floor"])
    }

    /// Under the Go core, the office's window on top is settled as the Swift engine settles it (on once set up,
    /// unless the menu said otherwise), with only that one key put in the core's file: the rest as it was.
    func testTheOnTopSettingIsSettledForTheCoreWithOneKey() throws {
        let file = folder.appendingPathComponent("config.json")
        let suite = "test.onTop.\(UUID().uuidString)"
        let defaults = try XCTUnwrap(UserDefaults(suiteName: suite))
        defer { defaults.removePersistentDomain(forName: suite) }
        let written = #"{"engine": "go", "hasCompletedSetup": true, "somethingOfTheCore": {"kept": [1, 2]}}"#
        try Data(written.utf8).write(to: file)
        let config = Config.load(from: file)
        OfficeWindow.settleForCore(config, defaults: defaults, file: file)
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: file)) as? [String: Any])
        XCTAssertEqual(object.keys.sorted(), ["alwaysOnTop", "engine", "hasCompletedSetup", "somethingOfTheCore"], "one key added, nothing else")
        XCTAssertEqual(object["alwaysOnTop"] as? Bool, true)
        XCTAssertEqual((object["somethingOfTheCore"] as? [String: Any])?["kept"] as? [Int], [1, 2])
        XCTAssertEqual(config.officeOnTop, true)
        // Once set, never again: the file is left alone.
        let before = try Data(contentsOf: file)
        OfficeWindow.settleForCore(Config.load(from: file), defaults: defaults, file: file)
        XCTAssertEqual(try Data(contentsOf: file), before)
        // The menu's choice from before, kept; and not set up yet: off.
        defaults.set(false, forKey: "officeWindow.alwaysOnTop")
        try Data(written.utf8).write(to: file)
        OfficeWindow.settleForCore(Config.load(from: file), defaults: defaults, file: file)
        XCTAssertEqual((try JSONSerialization.jsonObject(with: Data(contentsOf: file)) as? [String: Any])?["alwaysOnTop"] as? Bool, false)
        // No file yet: none made.
        try FileManager.default.removeItem(at: file)
        OfficeWindow.settleForCore(Config(), defaults: defaults, file: file)
        XCTAssertFalse(FileManager.default.fileExists(atPath: file.path))
    }

    /// The Go shell's office window: shown, and kept on top in memory only, never saved from the shell (the
    /// core keeps the setting in config.json).
    func testTheGoShellsOfficeLinkKeepsTheSettingInMemoryOnly() throws {
        let file = folder.appendingPathComponent("config.json")
        let config = Config()
        config.officeOnTop = false
        config.save(to: file)
        let before = try Data(contentsOf: file)
        let link = OfficeLink()
        let actions = link.actions(actions, config: config)
        actions.officeOnTop(true)
        actions.showOffice()
        XCTAssertEqual(config.officeOnTop, true)
        XCTAssertEqual(try Data(contentsOf: file), before, "the config file is the core's")
    }

    /// The Go shell's office keys in a test copy (TKEY): the bridge's office-key saves the choice in memory and
    /// holds nothing (the stand-in is never asked), and the facts say nothing is held.
    func testTheGoShellsOfficeKeysAreNeverHeldInATestCopy() throws {
        let config = Config()
        var asked = 0
        let link = OfficeLink(keys: { _, _ in asked += 1; return NSObject() }, labels: { [:] }, testCopy: true)
        var mac = actions
        mac.facts = { _ in ["testCopy": true] }
        let actions = link.actions(mac, config: config)
        XCTAssertNil(actions.officeKey(OfficeKey(modifiers: ["ctrl", "alt"], key: "K")))
        XCTAssertEqual(config.officeKey, "ctrl+alt+K", "the choice is still kept")
        XCTAssertNil(actions.officeKey(nil))
        XCTAssertEqual(config.officeKey, "off")
        XCTAssertNil(actions.officeKey(.standard))
        XCTAssertNil(config.officeKey)
        XCTAssertEqual(asked, 0, "no hot key asked for")
        XCTAssertNil(link.keys.current)
    }

    /// The Go shell's office keys (KSC): held at once by its holder (a stand-in here, never a real key), then kept
    /// in memory for the menu; keys it can't hold are its refusal, and nothing changes. Never saved from the shell.
    /// Setup's facts carry the keys held now and the layout's names for the keys (FXKSC B2, N2), for the core.
    func testTheGoShellsOfficeKeysAreHeldAtOnceAndKeptInMemoryOnly() throws {
        let file = folder.appendingPathComponent("config.json")
        let config = Config()
        config.save(to: file)
        let before = try Data(contentsOf: file)
        var held: [String] = []
        let link = OfficeLink(keys: { key, _ in
            held.append(key.text)
            return key.text == "ctrl+alt+T" ? nil : NSObject()
        }, labels: { ["Q": "A", "A": "Q"] })
        var mac = actions
        mac.facts = { _ in ["accessibility": true] }
        let actions = link.actions(mac, config: config)
        XCTAssertEqual(actions.officeKeyLabels(), ["Q": "A", "A": "Q"])
        let k = OfficeKey(modifiers: ["ctrl", "alt"], key: "K")
        XCTAssertNil(actions.officeKey(k))
        XCTAssertEqual(config.officeKey, "ctrl+alt+K")
        XCTAssertEqual(actions.officeKey(OfficeKey(modifiers: ["ctrl", "alt"], key: "T")),
                       "⌃⌥T is taken: macOS has it switched on, or won't let Smolcorp hold it. Choose another.")
        XCTAssertEqual(config.officeKey, "ctrl+alt+K", "as it was")
        XCTAssertEqual(link.keys.current, k, "the keys before are held again")
        var facts = actions.facts([])
        XCTAssertEqual(facts["accessibility"] as? Bool, true, "the Mac's facts, as they were")
        XCTAssertEqual(facts["officeKey"] as? NSDictionary, ["held": "ctrl+alt+K", "labels": ["Q": "A", "A": "Q"]] as NSDictionary)
        XCTAssertNil(actions.officeKey(nil))
        XCTAssertEqual(config.officeKey, "off")
        facts = actions.facts([])
        XCTAssertEqual((facts["officeKey"] as? [String: Any])?["held"] as? String, "off")
        XCTAssertNil(actions.officeKey(.standard))
        XCTAssertNil(config.officeKey)
        XCTAssertEqual(held, ["ctrl+alt+K", "ctrl+alt+T", "ctrl+alt+K", "alt+cmd+O"])
        XCTAssertEqual(try Data(contentsOf: file), before, "the config file is the core's")
    }

    // MARK: The socket

    /// One request on the socket, as the core sends it.
    private func send(_ line: String, to path: String) -> String? {
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        defer { close(fd) }
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        withUnsafeMutableBytes(of: &address.sun_path) { $0.copyBytes(from: path.utf8.prefix($0.count - 1)) }
        let connected = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
        }
        guard connected == 0 else { return nil }
        let data = Array((line + "\n").utf8)
        guard write(fd, data, data.count) == data.count else { return nil }
        var answer = Data()
        var chunk = [UInt8](repeating: 0, count: 4096)
        while true {
            let count = read(fd, &chunk, chunk.count)
            if count <= 0 { break }
            answer.append(chunk, count: count)
        }
        return String(data: answer, encoding: .utf8)
    }

    func testTheSocketIsYoursAloneAndAsksForTheToken() throws {
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: folder.path)
        let socket = folder.appendingPathComponent("bridge.sock")
        let bridge = HostBridge(socket: socket, token: token, actions: actions)
        try bridge.start()
        defer { bridge.stop() }
        bridge.core = getpid() // this test stands in for the core
        let mode = { (path: String) in (try FileManager.default.attributesOfItem(atPath: path)[.posixPermissions] as? NSNumber)?.intValue }
        XCTAssertEqual(try mode(socket.path), 0o600)
        XCTAssertEqual(try mode(folder.path), 0o700, "its folder made yours alone")

        let answer = try XCTUnwrap(send(#"{"token":"\#(token)","action":"focus","tab":"tmux:%1@/tmp/s"}"#, to: socket.path))
        XCTAssertEqual(answer, "{\"ok\":true}\n")
        let refused = try XCTUnwrap(send(#"{"token":"nope","action":"focus","tab":"tmux:%1@/tmp/s"}"#, to: socket.path))
        XCTAssertTrue(refused.contains("\"ok\":false"))
        XCTAssertEqual(asked, ["focus tmux:%1@/tmp/s"])

        bridge.stop()
        XCTAssertFalse(FileManager.default.fileExists(atPath: socket.path), "removed as it stops")
        XCTAssertNil(send(#"{"token":"\#(token)","action":"hello"}"#, to: socket.path))
    }

    func testAnEarlierSocketIsReplacedButNothingElse() throws {
        let socket = folder.appendingPathComponent("bridge.sock")
        // One a launch left behind: nothing listens there any more.
        let left = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        withUnsafeMutableBytes(of: &address.sun_path) { $0.copyBytes(from: socket.path.utf8.prefix($0.count - 1)) }
        let bound = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { Darwin.bind(left, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
        }
        XCTAssertEqual(bound, 0)
        close(left)
        XCTAssertTrue(FileManager.default.fileExists(atPath: socket.path))
        let first = HostBridge(socket: socket, token: token, actions: actions)
        try first.start()
        defer { first.stop() }
        first.core = getpid() // this test stands in for the core
        XCTAssertNotNil(try XCTUnwrap(send(#"{"token":"\#(token)","action":"hello"}"#, to: socket.path)).range(of: "true"))

        // A live one is another Smolcorp's on this folder: never taken over.
        let second = HostBridge(socket: socket, token: "fake-token-second-second-second-00", actions: actions)
        XCTAssertThrowsError(try second.start()) { XCTAssertEqual($0.localizedDescription, HostBridge.Failure.inUse.localizedDescription) }
        XCTAssertNotNil(try XCTUnwrap(send(#"{"token":"\#(token)","action":"hello"}"#, to: socket.path)).range(of: "true"),
                        "the first still has its bridge")

        let file = folder.appendingPathComponent("plain")
        try Data().write(to: file)
        XCTAssertThrowsError(try HostBridge(socket: file, actions: actions).start(), "a file that isn't a socket is left alone")
        XCTAssertTrue(FileManager.default.fileExists(atPath: file.path))
        let long = folder.appendingPathComponent(String(repeating: "x", count: 120))
        XCTAssertThrowsError(try HostBridge(socket: long, actions: actions).start())
    }

    /// A request has `requestWait` in all, however slowly it's sent, and only so many are served at once.
    func testASlowOrCrowdingClientIsCutOff() throws {
        let socket = folder.appendingPathComponent("bridge.sock")
        let bridge = HostBridge(socket: socket, token: token, actions: actions)
        try bridge.start()
        defer { bridge.stop() }
        bridge.core = getpid() // this test stands in for the core
        func connect() -> Int32 {
            let fd = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
            var address = sockaddr_un()
            address.sun_family = sa_family_t(AF_UNIX)
            withUnsafeMutableBytes(of: &address.sun_path) { $0.copyBytes(from: socket.path.utf8.prefix($0.count - 1)) }
            _ = withUnsafePointer(to: &address) {
                $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { Darwin.connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
            }
            return fd
        }
        // As many as are served, each sending nothing yet.
        let held = (0..<HostBridge.connectionsMost).map { _ in connect() }
        defer { held.forEach { close($0) } }
        Thread.sleep(forTimeInterval: 0.3)
        // One more is closed at once.
        let extra = connect()
        var byte: UInt8 = 0
        var wait = timeval(tv_sec: 2, tv_usec: 0)
        setsockopt(extra, SOL_SOCKET, SO_RCVTIMEO, &wait, socklen_t(MemoryLayout<timeval>.size))
        XCTAssertEqual(read(extra, &byte, 1), 0, "closed, not left waiting")
        close(extra)

        // One that sends a byte a second never gets its whole request in.
        let start = Date()
        let slow = held[0]
        setsockopt(slow, SOL_SOCKET, SO_RCVTIMEO, &wait, socklen_t(MemoryLayout<timeval>.size))
        var closed = false
        for character in Array(#"{"token":"\#(token)","action":"hello"}"#.utf8) {
            var one = character
            if write(slow, &one, 1) != 1 { closed = true; break }
            if read(slow, &byte, 1) == 0 { closed = true; break } // waits up to 2 s, the next byte's pace
        }
        XCTAssertTrue(closed)
        XCTAssertLessThan(Date().timeIntervalSince(start), HostBridge.requestWait + 3)
    }

    /// The token alone opens nothing: only the core this app started is served,
    /// at whatever name its socket has been moved to.
    func testOnlyTheCoreIsServedTokenOrNot() throws {
        let socket = folder.appendingPathComponent("bridge.sock")
        let bridge = HostBridge(socket: socket, token: token, actions: actions)
        try bridge.start()
        defer { bridge.stop() }
        let request = #"{"token":"\#(token)","action":"secret-read","item":"summary-key"}"#
        XCTAssertNil(bridge.core)
        XCTAssertEqual(send(request, to: socket.path), "", "no core started yet: closed unanswered")
        bridge.core = getppid()
        XCTAssertEqual(send(request, to: socket.path), "", "not the core's pid")

        // Moved aside, as another process of yours would to listen in its place: still only the core's.
        let kept = folder.appendingPathComponent("kept.sock")
        try FileManager.default.moveItem(at: socket, to: kept)
        bridge.core = getpid()
        // Another process, with the right token.
        let other = Process()
        other.executableURL = URL(fileURLWithPath: "/usr/bin/nc")
        other.arguments = ["-U", kept.path]
        let toOther = Pipe(), fromOther = Pipe()
        other.standardInput = toOther
        other.standardOutput = fromOther
        other.standardError = FileHandle.nullDevice
        try other.run()
        try toOther.fileHandleForWriting.write(contentsOf: Data((request + "\n").utf8))
        let heard = fromOther.fileHandleForReading.readDataToEndOfFile()
        other.waitUntilExit()
        XCTAssertEqual(heard, Data(), "another process is given nothing")
        try? toOther.fileHandleForWriting.close()

        // The core itself, at the new name.
        XCTAssertNotNil(try XCTUnwrap(send(request, to: kept.path)).range(of: "fake-summary-key"))
        try FileManager.default.moveItem(at: kept, to: socket)
    }

    /// Setup's Mac parts and the desks' abilities, on stand-ins for what asks macOS: each request of the shared
    /// fixture is answered exactly as it says (desktop/hostbridge_test.go checks the core sends and reads the same).
    func testSetupsMacPartsAnswerAsTheSharedFixture() throws {
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("fixtures/bridge/actions.json")
        let fixture = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])
        let cases = try XCTUnwrap(fixture["cases"] as? [[String: Any]])
        let facts = try XCTUnwrap((cases.first?["answer"] as? [String: Any])?["facts"] as? [String: Any])
        var mac = actions
        mac.main = terminals
        mac.facts = { [unowned self] apps in asked.append("facts \(apps.joined(separator: ","))"); return facts }
        mac.dock = { [unowned self] in asked.append("dock \($0)") }
        mac.askAccessibility = { [unowned self] in asked.append("accessibility") }
        mac.keystrokesOn = { [unowned self] in asked.append("keystrokes on") }
        // The install line (FXC1c), by the real check on stand-in bundles: a copy run from Downloads, a
        // translocated one with a quarantined Smolcorp in Applications, one in Applications, a test copy. Its fix: moved, then refused as a test copy's is.
        var installLooks = 0
        mac.installCheck = {
            installLooks += 1
            let (bundle, testCopy, quarantined): (String, Bool, String) = switch installLooks {
            case 1: ("/Users/example/Downloads/Smolcorp.app", false, "")
            case 2: ("/private/var/folders/ab/T/AppTranslocation/1234/d/Smolcorp.app", false, "/Applications/Smolcorp.app")
            case 3: ("/Applications/Smolcorp.app", false, "")
            default: ("/Users/example/dev/Smolcorp.app", true, "")
            }
            return SelfTest.installCheck(bundle: URL(fileURLWithPath: bundle), testCopy: testCopy, home: "/Users/example",
                                         quarantined: { $0 == quarantined })
        }
        mac.moveToApplications = { [unowned self] in
            asked.append("install")
            if asked.filter({ $0 == "install" }).count == 2 { throw SelfTest.FixError.testCopy }
        }
        mac.prepareDictation = { [unowned self] language, done in
            asked.append("dictation \(language)")
            done(language == "fr_FR" ? "Apple's model for French couldn't be downloaded." : nil)
        }
        mac.terminalCommand = { [unowned self] add in
            asked.append("terminal command \(add)")
            throw TerminalCommand.Problem.testCopy
        }
        mac.roster = { [GrokBotApp.Row(id: "0a1b-2c3d", name: "Smolcorp Mini Manager"), GrokBotApp.Row(id: "9f8e", name: "Two bots", isGroup: true)] }
        // The Go shell's office window and the folder panel: a floor's picks a project, the hires' is cancelled.
        mac.showOffice = { [unowned self] in asked.append("office") }
        mac.officeOnTop = { [unowned self] in asked.append("on top \($0)") }
        // The office's keys (KSC): held by a stand-in, never a real key; macOS has ⌃⌥T on. The layout in use is a
        // French AZERTY's (FXKSC B2): ⌘ on the US A key is its ⌘Q, refused before the holder is asked.
        mac.officeKey = { [unowned self] key in
            asked.append("keys \(key?.text ?? "off")")
            return key?.text == "ctrl+alt+T" ? key.map { OfficeKey.takenRefusal($0) } : nil
        }
        mac.officeKeyLabels = { ["Q": "A", "A": "Q"] }
        mac.chooseFolder = { [unowned self] start, message, done in
            asked.append("choose \(start ?? "-") \(message == Deck.floorFolderMessage ? "floor" : message == Deck.hireFolderMessage ? "hires" : message)")
            done(message == Deck.floorFolderMessage ? "/Users/example/project" : nil)
        }
        mac.closeFolderPanel = { [unowned self] in asked.append("close \($0 == Deck.hireFolderMessage ? "hires" : $0)") }
        // The API-key account (M4c): offered, out of a test copy, with a stand-in key.
        mac.hireKey = HostBridge.HireKey(offered: { true }, testCopy: false, read: { "fk-m4c-0123456789" })
        // The jukebox: playing at 45, without Accessibility, so its keys are refused and only the volume is set.
        mac.musicState = { (true, (45, false)) }
        mac.pressMusic = { [unowned self] in asked.append("music \($0)") }
        mac.setVolume = { [unowned self] in asked.append("volume \($0)") }
        // How the Mac writes a short time, as an American English Mac does.
        mac.timeFormat = { ["pattern": "h:mm a", "am": "AM", "pm": "PM"] }
        // Where you are, for the portable office: in use 2.5 s ago, unlocked, after 3 unlocks and a wake.
        mac.presence = { MacPresence.Now(idle: 2.5, locked: false, unlocks: 3, wakes: 1) }
        // The phone's text and the peek (I5): the agent in front but on ttys005, where it has ended; Codex on
        // ttys004, not known to take several lines as one paste; iTerm2's CD34 doesn't read its ⌘V in time.
        mac.place = { tab, _ in tab.id == "/dev/ttys005" ? .gone : .front }
        // FXHIRE: the agent has left ttys005 (so a strict close there touches nothing), not iTerm2's AB12.
        mac.ended = { tab, _, _ in tab.id == "/dev/ttys005" }
        var typed: [String] = []
        mac.driver = { tab in PhoneTerminal(pastes: tab.id == "CD34" ? .notRead : .pasted) { typed.append($0) } }
        mac.screen = { _ in .some("> Bonjour l’équipe\n  ? for shortcuts\n\n\n") }
        // The office's mic (M7GAPA): recording starts for ttys003 only; stopped, it heard a few words, cleaned up
        // in 0.9 s; the text then goes into the tab where its agent is in front (ttys005's has ended).
        var heard = ["Fix the uh small corp build"]
        mac.startDictation = { [unowned self] language, tab, done in asked.append("record \(language) \(tab)"); done(tab == "tty:/dev/ttys003") }
        mac.stopDictation = { [unowned self] done in asked.append("stop recording"); done(heard.isEmpty ? "" : heard.removeFirst()) }
        mac.cleanDictation = { [unowned self] text, desks, cleanup, glossary, done in
            asked.append("clean \(text) \(desks) \(cleanup) \(glossary == DictationCleanup.defaultGlossary ? "default" : "\(glossary)")")
            cleanup ? done("Fix the Smolcorp build", .cleaned(0.9)) : done(text, .off)
        }
        mac.clearCleanupProblem = { [unowned self] in asked.append("clean-up problem cleared") }
        mac.insert = { [unowned self] text, tab in asked.append("insert \(text) \(tab.text)"); return true }
        // The processes (M5c): three in the hire's worktree or naming its session; a Mac that can't list them without one.
        mac.processes = { folder, session in
            guard folder == "/Users/ada/repo-hires/ada-abcdef", !session.isEmpty else { return nil }
            return [(4242, "claude", "folder"), (4343, "node", "add-dir"), (4444, "tail", "session")]
        }
        // T3 Code's app (C4 A1): there, opened, and handed only its own thread's link.
        mac.looking = terminals
        mac.t3App = { true }
        mac.t3OpenApp = { [unowned self] in asked.append("t3 app opened") }
        mac.t3OpenLink = { [unowned self] in asked.append("t3 link \($0.absoluteString)"); return true }
        // The GM's tiling (M7GAPC): the fixture's own screens; Terminal shows ttys003 in window 101 and ttys009 in 102,
        // and moves 101 within a character of where it's asked; iTerm2 refuses Smolcorp its Automation permission.
        // Only `Tiler.tile` runs, on a stand-in: no AppleScript is ever run.
        let screens = try XCTUnwrap(cases.first { $0["name"] as? String == "the Mac's screens, read afresh" }.flatMap { ($0["answer"] as? [String: Any])?["screens"] as? [[String: Any]] })
        mac.screens = { screens }
        mac.tileScript = { [unowned self] source, arguments in
            let app = source.contains("tell application \"iTerm\"") ? "iTerm" : "Terminal"
            if source.contains("repeat with w in windows") {
                asked.append("list \(app)")
                return app == "iTerm" ? .notAllowed : .ok("101 /dev/ttys003\n102 /dev/ttys009")
            }
            asked.append("move \(app) \(arguments.joined(separator: " "))")
            return .ok("0 30 476 566")
        }
        mac.placeOffice = { [unowned self] rect, mainHeight in asked.append("office \(Int(rect.minX)) \(Int(rect.minY)) \(Int(rect.width)) \(Int(rect.height)) \(Int(mainHeight))") }
        func text(_ object: Any) -> String {
            String(decoding: (try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])) ?? Data(), as: UTF8.self)
        }
        XCTAssertGreaterThanOrEqual(cases.count, 10)
        for one in cases {
            var request = try XCTUnwrap(one["request"] as? [String: Any])
            request["token"] = token
            // The core says the desk waits at its prompt, but ttys006's began asking as its terminal got to the words.
            let held = request["tab"] as? String == "tty:/dev/ttys006"
            // A test copy's tile is refused before any terminal is told anything (B1, RGAPC).
            var here = mac
            here.testCopy = one["name"] as? String == "tiling refused in a test copy"
            let answer = HostBridge.answer(try JSONSerialization.data(withJSONObject: request), token: token, actions: here,
                                           deskState: { held ? .asking : .unread })
            XCTAssertEqual(text(answer), text(one["answer"] ?? NSNull()), one["name"] as? String ?? "")
        }
        // Only what each asked for was done; a language it doesn't know and a refused app did nothing.
        XCTAssertEqual(asked, ["facts com.example.notes", "dock false", "office", "on top true", "keys ctrl+alt+K", "keys ctrl+alt+T", "choose /Users/example/hires floor", "choose - hires",
                               "close hires", "accessibility", "install", "install", "keystrokes on", "dictation fr_FR",
                               "record fr_FR tty:/dev/ttys003", "record en_US tty:/dev/ttys004", "stop recording", "stop recording",
                               "clean Fix the uh small corp build [\"smolcorp\", \"website\"] true default", "clean hello there [] false []",
                               "clean-up problem cleared",
                               "insert Fix the Smolcorp build tty:/dev/ttys003", "terminal command true",
                               "t3 app opened", "t3 link t3code://threads/local/mcp%3Aabc-1", "volume 30",
                               "list Terminal", "move Terminal 101 0 30 480 573", "list iTerm", "office 0 1750 1440 810 2560"])
        XCTAssertTrue(login, "Open at login, switched on")
        // The phone's text, by `Deck.typePhoneWords` on the stand-in drivers: nothing for an agent that has ended,
        // nor for "!", nor for Codex, nor to a desk asking. ⌘V that wasn't read in time went.
        XCTAssertEqual(typed, ["insert tty:/dev/ttys003 Bonjour l’équipe « ça marche » 👩‍💻 ;",
                               "paste iterm:AB12 first line\n    indented\tand a tab", "paste iterm:CD34 one\ntwo"])
    }

    /// The GM's tiling refuses what isn't its own (M7GAPC): a terminal that can't place its windows, a place that isn't
    /// a tty's or a rectangle, more than the nine desks, an office with nowhere to go. Nothing is run for any of them.
    func testTilingRefusesWhatIsntAPlaceOnAScriptableTerminal() {
        let place: [String: Any] = ["tty": "/dev/ttys003", "rect": ["x": 0, "y": 30, "width": 480, "height": 573]]
        for (request, refusal) in [
            (["action": "tile", "terminal": "ghostty", "places": [place]], "Not a terminal Smolcorp can place the windows of."),
            (["action": "tile", "terminal": "terminal", "places": [["tty": "ttys003; rm", "rect": place["rect"]!]]], "Not a list of places."),
            (["action": "tile", "terminal": "terminal", "places": [["tty": "/dev/ttys003", "rect": ["x": 0, "y": 30]]]], "Not a list of places."),
            (["action": "tile", "terminal": "terminal", "places": Array(repeating: place, count: 10)], "Not a list of places."),
            (["action": "office-place", "office": ["x": 0, "y": 0, "width": -1, "height": 4], "mainHeight": 900], "Say where the office goes."),
        ] as [([String: Any], String)] {
            let answer = ask(request)
            XCTAssertFalse(isOK(answer), "\(request)")
            XCTAssertEqual(answer["error"] as? String, refusal, "\(request)")
        }
        XCTAssertEqual(windowQueues, [], "nothing was run on a terminal's queue")
    }

    /// A test copy never moves another app's windows (B1, RGAPC): `tile` and `office-place` are refused with the Swift
    /// engine's sentence before any terminal's queue, AppleScript or window is touched. The screens are only read. The
    /// same requests are done for the Mac in use.
    func testATestCopyRefusesToTileAndToPlaceTheOffice() {
        var copy = actions
        copy.main = terminals // the test is on the main queue, which would wait for itself
        copy.testCopy = true
        var scripts = 0, placed = 0
        copy.tileScript = { _, _ in scripts += 1; return .failed }
        copy.placeOffice = { _, _ in placed += 1 }
        copy.screens = { [["id": "1", "name": "Built-in"]] }
        let place: [String: Any] = ["tty": "/dev/ttys003", "rect": ["x": 0, "y": 30, "width": 480, "height": 573]]
        let office: [String: Any] = ["x": 0, "y": 1750, "width": 1440, "height": 810]
        for request in [["action": "tile", "terminal": "terminal", "places": [place]],
                        ["action": "tile", "terminal": "iterm", "places": [place]],
                        ["action": "tile", "terminal": "ghostty", "places": [place]],
                        ["action": "office-place", "office": office, "mainHeight": 2560]] as [[String: Any]] {
            let answer = ask(request, actions: copy)
            XCTAssertFalse(isOK(answer), "\(request)")
            XCTAssertEqual(answer["error"] as? String, TestCopy.OtherApp.tiler.sentence, "\(request)")
        }
        XCTAssertEqual(TestCopy.OtherApp.tiler.sentence, "This is a test copy of Smolcorp: it never moves or tiles the windows of other apps.")
        XCTAssertEqual(scripts + placed, 0, "nothing was run")
        XCTAssertEqual(windowQueues, [], "nothing was run on a terminal's queue")
        XCTAssertTrue(isOK(ask(["action": "screens"], actions: copy)), "the screens are only read")
    }

    /// The API-key account for a hire (M4c): whether it's offered and in what words, never reading the key; the key
    /// only while it's offered, and the Swift engine's refusals (a test copy's own) when it isn't or can't be read.
    func testTheAPIKeyAccountIsOfferedInTheMacsWordsAndItsKeyGivenOnlyWhileOffered() {
        var offered = false, reads = 0
        var mac = actions
        func stand(testCopy: Bool, key: String?) {
            mac.hireKey = HostBridge.HireKey(offered: { offered }, testCopy: testCopy, read: { reads += 1; return key })
        }
        func answer(_ action: String, token given: String? = nil) -> [String: Any] {
            HostBridge.answer(try! JSONSerialization.data(withJSONObject: ["action": action, "token": given ?? token]), token: token, actions: mac)
        }
        stand(testCopy: false, key: "fk-m4c-1")
        let notInKeychain = HireDesk.apiKeyRefusal(picked: true, offered: false, testCopy: false)?.message
        let off = answer("api-key-offer")
        XCTAssertTrue(isOK(off))
        XCTAssertEqual(off["available"] as? Bool, false)
        XCTAssertEqual(off["notice"] as? String, HireDesk.apiKeyNotice)
        XCTAssertEqual(off["refusal"] as? String, notInKeychain)
        XCTAssertNotNil(notInKeychain)
        let refusedRead = answer("api-key-read")
        XCTAssertFalse(isOK(refusedRead))
        XCTAssertEqual(refusedRead["error"] as? String, notInKeychain)
        XCTAssertNil(refusedRead["value"])
        XCTAssertEqual(reads, 0, "not offered: the key is never read")

        offered = true
        let on = answer("api-key-offer")
        XCTAssertEqual(on["available"] as? Bool, true)
        XCTAssertNil(on["refusal"])
        XCTAssertNil(on["value"])
        XCTAssertEqual(reads, 0, "the offer never reads the key")
        let read = answer("api-key-read")
        XCTAssertEqual(read["value"] as? String, "fk-m4c-1")
        XCTAssertEqual(read["found"] as? Bool, true)
        XCTAssertEqual(reads, 1)
        XCTAssertFalse(isOK(answer("api-key-read", token: "wrong-token-wrong-token-wrong-tok")))
        XCTAssertEqual(reads, 1, "nothing without the token")

        // A test copy: its own words, and a key that can't be read is the Swift engine's refusal, never an empty key.
        stand(testCopy: true, key: nil)
        XCTAssertEqual(answer("api-key-offer")["notice"] as? String, HireDesk.testCopyApiKeyNotice)
        let unread = answer("api-key-read")
        XCTAssertFalse(isOK(unread))
        XCTAssertEqual(unread["error"] as? String, HireDesk.keyRefusalText(testCopy: true).message)
        XCTAssertNil(unread["value"])
    }

    /// The phone's text (I5): checked again as the Swift engine prepares it, then only by `Deck.typePhoneWords`,
    /// on the terminal's queue, while the agent isn't known to have left; several lines only by `insertLines`.
    func testThePhonesTextIsCheckedAgainAndTypedByTheDrivers() {
        var mac = actions
        var typed: [String] = []
        mac.place = { tab, _ in ["/dev/ttys004": .behind, "/dev/ttys005": nil][tab.id] ?? .front }
        mac.driver = { tab in tab.kind == "tty" ? PhoneTerminal(pastes: .failed) { typed.append($0) } : nil }
        mac.screen = { tab in tab.kind == "tty" ? .some(nil) : nil }
        func phone(_ text: Any?, tab: String = "tty:/dev/ttys003", agent: String? = "claude") -> [String: Any] {
            var request: [String: Any] = ["action": "phone-text", "tab": tab, "token": token]
            if let text { request["text"] = text }
            if let agent { request["agent"] = agent }
            return HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: mac, deskState: { .idle })
        }
        for refused: Any? in ["", "  padded  ", "one\u{1b}[201~", "tab\there", "a\r\nb", "!ls", String(repeating: "a", count: 2001), 42, nil] {
            XCTAssertFalse(isOK(phone(refused)), "\(String(describing: refused))")
        }
        XCTAssertFalse(isOK(phone("hello", agent: nil)), "no agent")
        XCTAssertFalse(isOK(phone("hello", agent: "nobody")), "not an agent")
        XCTAssertFalse(isOK(phone("hello", tab: "not a tab")))
        XCTAssertTrue(typed.isEmpty, "nothing typed")
        let behind = phone("hello", tab: "tty:/dev/ttys004")
        XCTAssertEqual(behind["place"] as? String, "behind")
        XCTAssertTrue(typed.isEmpty, "nothing typed behind the shell")
        // Where it can't be told, the words go, as the deck's keys do.
        XCTAssertEqual(phone("hello", tab: "tty:/dev/ttys005")["pasted"] as? String, "pasted")
        // A paste that failed may have typed: said so, for the core's 502, never a 409 that offers the words again.
        let failed = phone("one\ntwo")
        XCTAssertFalse(isOK(failed))
        XCTAssertEqual(failed["mayHaveTyped"] as? Bool, true)
        XCTAssertEqual(typed, ["insert tty:/dev/ttys005 hello", "paste tty:/dev/ttys003 one\ntwo"])
        // A terminal the app can't drive: the core is told so, and nothing is typed.
        XCTAssertEqual(phone("hello", tab: "iterm:AB12")["unsupported"] as? Bool, true)
        // With no core to ask, it's asking: nothing goes.
        let unasked = HostBridge.answer(try! JSONSerialization.data(withJSONObject: ["action": "phone-text", "tab": "tty:/dev/ttys003", "agent": "claude",
                                                                                     "text": "hello", "token": token]), token: token, actions: mac)
        XCTAssertEqual(unasked["error"] as? String, Deck.answerFirst)
        XCTAssertNil(unasked["mayHaveTyped"])
        XCTAssertEqual(typed.count, 2)
        // The peek: only in front, and only where the terminal can show its screen.
        func peek(_ tab: String) -> [String: Any] {
            HostBridge.answer(try! JSONSerialization.data(withJSONObject: ["action": "peek", "tab": tab, "agent": "claude", "token": token]),
                              token: token, actions: mac)
        }
        XCTAssertEqual(peek("tty:/dev/ttys005")["place"] as? String, "behind", "unknown isn't in front")
        XCTAssertEqual(peek("tty:/dev/ttys003")["error"] as? String, "This terminal can't show its input line.")
        XCTAssertEqual(peek("iterm:AB12")["unsupported"] as? Bool, true)
    }

    /// The owner's rule, never answer a waiting prompt, under the Go core: the desk began asking while the words
    /// waited for its terminal's queue. The core is asked from that queue, after the agent's place and right
    /// before the words, as `Deck.typePhoneDictation` looks at its sessions there: nothing is typed.
    func testTheCoreIsAskedOnTheTerminalsQueueRightBeforeTheWords() {
        let log = NSLock()
        var done: [String] = []
        var asking = false
        func note(_ what: String) { log.lock(); done.append(what); log.unlock() }
        var mac = actions
        mac.place = { _, _ in note("place"); return .front }
        mac.driver = { _ in PhoneTerminal(pastes: .pasted, note: note) }
        let deskState = { () -> SessionState? in
            log.lock(); defer { log.unlock() }
            done.append("asked the core")
            return asking ? .asking : .idle
        }
        for (words, typed) in [("fix the login bug", "insert tty:/dev/ttys003 fix the login bug"), ("one\n  two", "paste tty:/dev/ttys003 one\n  two")] {
            // Another desk's AppleScript holds Terminal's queue; the prompt comes meanwhile.
            let release = DispatchSemaphore(value: 0)
            terminals.async { release.wait() }
            var answer: [String: Any] = [:]
            let answered = DispatchSemaphore(value: 0)
            let line = try! JSONSerialization.data(withJSONObject: ["action": "phone-text", "tab": "tty:/dev/ttys003", "agent": "claude", "text": words, "token": token])
            DispatchQueue.global().async { [token] in
                answer = HostBridge.answer(line, token: token, actions: mac, until: .now() + HostBridge.actionWait, deskState: deskState)
                answered.signal()
            }
            Thread.sleep(forTimeInterval: 0.2)
            log.lock(); XCTAssertEqual(done, [], "nothing before the queue"); asking = true; log.unlock()
            release.signal()
            answered.wait()
            XCTAssertEqual(answer["error"] as? String, Deck.answerFirst)
            XCTAssertEqual(done, ["place", "asked the core"], "nothing typed")
            // Answered on the deck: the next words go, after the same question.
            log.lock(); asking = false; done = []; log.unlock()
            let free = HostBridge.answer(line, token: token, actions: mac, until: .now() + HostBridge.actionWait, deskState: deskState)
            XCTAssertEqual(free["pasted"] as? String, "pasted")
            XCTAssertEqual(done, ["place", "asked the core", typed])
            done = []
        }
    }

    /// The question goes on the request's own connection: the desk's state, as the core has it. The words go
    /// as `Deck.typePhoneWords` lets them on that state; asking, no answer in time, anything but a state, or an
    /// answer sent before it was asked holds them. A paste asks again right before ⌘V, `CoreQuestion.most` times
    /// at most: past that, it's asking, and the core isn't asked.
    func testTheCoreIsAskedTheDesksStateOnTheRequestsOwnConnection() throws {
        let socket = folder.appendingPathComponent("bridge.sock")
        var mac = actions
        let log = NSLock()
        var typed: [String] = []
        var readies = 1
        mac.place = { _, _ in .front }
        mac.driver = { _ in PhoneTerminal(pastes: .pasted, readies: readies) { what in log.lock(); typed.append(what); log.unlock() } }
        let bridge = HostBridge(socket: socket, token: token, actions: mac)
        try bridge.start()
        defer { bridge.stop() }
        bridge.core = getpid() // this test stands in for the core
        func request(_ text: String) -> String {
            let line = try! JSONSerialization.data(withJSONObject: ["token": token, "action": "phone-text", "tab": "tty:/dev/ttys003",
                                                                    "agent": "claude", "text": text])
            return String(decoding: line, as: UTF8.self)
        }
        /// The request, then each of `replies` once the app has said something, as the core answers.
        func converse(_ first: String, replies: [String]) -> [String] {
            let fd = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
            defer { close(fd) }
            var address = sockaddr_un()
            address.sun_family = sa_family_t(AF_UNIX)
            withUnsafeMutableBytes(of: &address.sun_path) { $0.copyBytes(from: socket.path.utf8.prefix($0.count - 1)) }
            _ = withUnsafePointer(to: &address) {
                $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { Darwin.connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
            }
            var wait = timeval(tv_sec: 5, tv_usec: 0)
            setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &wait, socklen_t(MemoryLayout<timeval>.size))
            // A reply to an app that has answered and closed mustn't raise SIGPIPE in the tests.
            var noSignal: Int32 = 1
            setsockopt(fd, SOL_SOCKET, SO_NOSIGPIPE, &noSignal, socklen_t(MemoryLayout<Int32>.size))
            _ = HostBridge.write(Data(first.utf8), to: fd)
            var heard: [String] = [], pending = Data(), replies = replies
            var chunk = [UInt8](repeating: 0, count: 4096)
            while true {
                let count = read(fd, &chunk, chunk.count)
                guard count > 0 else { break }
                pending.append(chunk, count: count)
                while let end = pending.firstIndex(of: 0x0A) {
                    let line = String(decoding: pending[pending.startIndex..<end], as: UTF8.self)
                    heard.append(line)
                    pending = Data(pending[(end + 1)...])
                    // Only a question is answered.
                    if line == #"{"ready":true}"#, !replies.isEmpty { _ = HostBridge.write(Data((replies.removeFirst() + "\n").utf8), to: fd) }
                }
            }
            return heard
        }
        /// The app's answer, parsed: JSONSerialization doesn't keep key order.
        func answer(_ heard: [String]) -> NSDictionary? {
            heard.last.flatMap { try? JSONSerialization.jsonObject(with: Data($0.utf8)) as? NSDictionary }
        }
        let pasted = ["ok": true, "pasted": "pasted"] as NSDictionary
        let held = ["ok": false, "error": Deck.answerFirst] as NSDictionary
        // One line: the core asked once; a desk working or gone (as the Swift engine's own sessions say nil) takes it.
        for state in [#"{"state":"idle"}"#, #"{"state":"thinking"}"#, #"{"state":""}"#] {
            let heard = converse(request("hello") + "\n", replies: [state])
            XCTAssertEqual(heard.count, 2, "\(state): \(heard)")
            XCTAssertEqual(heard.first, #"{"ready":true}"#)
            XCTAssertEqual(answer(heard), pasted, "\(state): \(heard)")
        }
        XCTAssertEqual(typed, Array(repeating: "insert tty:/dev/ttys003 hello", count: 3))
        for no in [#"{"state":"asking"}"#, #"{"go":true}"#, #"{"state":"sleeping"}"#, #"{"state":"idle"} {"state":"idle"}"#, "nonsense"] {
            let heard = converse(request("hello") + "\n", replies: [no])
            XCTAssertEqual(heard.first, #"{"ready":true}"#, no)
            XCTAssertEqual(answer(heard), held, "\(no): \(heard)")
        }
        // Said before it was asked: not an answer, and not asked at all.
        XCTAssertEqual(converse(request("hello") + "\n" + #"{"state":"idle"}"# + "\n", replies: []).map { answer([$0]) }, [held])
        // No answer in time.
        let started = Date()
        let silent = converse(request("hello") + "\n", replies: [])
        XCTAssertEqual(silent.first, #"{"ready":true}"#)
        XCTAssertEqual(answer(silent), held, "\(silent)")
        XCTAssertLessThan(Date().timeIntervalSince(started), HostBridge.CoreQuestion.wait + 1.5)
        XCTAssertEqual(typed.count, 3, "typed only on a state that lets it")
        // Several lines through the clipboard: asked before and right before ⌘V, as `ClipboardPaste.paste` asks.
        readies = 2
        let twice = converse(request("one\ntwo") + "\n", replies: [#"{"state":"unread"}"#, #"{"state":"unread"}"#])
        XCTAssertEqual(twice.filter { $0 == #"{"ready":true}"# }.count, 2)
        XCTAssertEqual(answer(twice), pasted, "\(twice)")
        let working = converse(request("one\ntwo") + "\n", replies: [#"{"state":"unread"}"#, #"{"state":"thinking"}"#])
        XCTAssertEqual(answer(working), ["ok": false, "error": Deck.linesWhileWorking] as NSDictionary, "a turn began as it was brought forward")
        // Past `most`, the core isn't asked again: asking, and nothing goes.
        readies = HostBridge.CoreQuestion.most + 1
        let many = converse(request("one\ntwo") + "\n", replies: Array(repeating: #"{"state":"unread"}"#, count: readies))
        XCTAssertEqual(many.filter { $0 == #"{"ready":true}"# }.count, HostBridge.CoreQuestion.most)
        XCTAssertEqual(answer(many), held, "\(many)")
        XCTAssertEqual(typed.suffix(1), ["paste tty:/dev/ttys003 one\ntwo"])
        XCTAssertEqual(typed.count, 4, "one paste only")
    }

    /// The processes (M5c): a full path that isn't `/` and has no `..`, the worktree of a hire open in the core's
    /// registry, and a session's id (its UUID shape) or none; anything else is refused before anything is looked at
    /// (RM5c S2). What's found is answered, the first `processesMost`; a Mac that can't look says so.
    func testTheProcessesTakeOnlyAFolderAndASession() {
        let session = "6f1c2a9e-0b3d-4e5f-8a7b-1C2D3E4F5A6B"
        var mac = actions
        var looked: [String] = []
        let looking = DispatchQueue(label: "test.looking")
        mac.looking = looking
        mac.processes = { folder, session in
            dispatchPrecondition(condition: .onQueue(looking))
            looked.append("\(folder) \(session)")
            return folder == "/nowhere" ? nil : (1...70).map { (pid_t($0), "zsh", "folder") }
        }
        for refused: [String: Any] in [
            ["action": "processes"],
            ["action": "processes", "folder": "relative/wt"],
            ["action": "processes", "folder": "/"],
            ["action": "processes", "folder": "/Users/ada/../bob"],
            ["action": "processes", "folder": "/Users/ada/wt\n"],
            ["action": "processes", "folder": "/" + String(repeating: "a", count: 1024)],
            ["action": "processes", "folder": 42],
            ["action": "processes", "folder": "/Users/ada/wt", "session": "a/b"],
            ["action": "processes", "folder": "/Users/ada/wt", "session": "a b"],
            ["action": "processes", "folder": "/Users/ada/wt", "session": String(repeating: "a", count: 65)],
            ["action": "processes", "folder": "/Users/ada/wt", "session": 7],
            // Not a session's id: a letter, part of one, one too long, or not hex.
            ["action": "processes", "folder": "/Users/ada/wt", "session": "e"],
            ["action": "processes", "folder": "/Users/ada/wt", "session": "6f1c2a9e-0b3d"],
            ["action": "processes", "folder": "/Users/ada/wt", "session": session + "0"],
            ["action": "processes", "folder": "/Users/ada/wt", "session": "6f1c2a9e-0b3d-4e5f-8a7b-1c2d3e4f5a6g"],
            ["action": "processes", "folder": "/Users/ada/wt", "session": "6f1c2a9e0b3d-4e5f-8a7b-1c2d3e4f5a6b-"],
            // Not a hire's worktree: above one, inside one, beside one.
            ["action": "processes", "folder": "/Users"],
            ["action": "processes", "folder": "/Users/ada"],
            ["action": "processes", "folder": "/Users/ada/wt/sub"],
            ["action": "processes", "folder": "/Users/ada/repo-hires/bob-123456", "session": session],
        ] {
            var request = refused
            request["token"] = token
            XCTAssertFalse(isOK(HostBridge.answer(line(request), token: token, actions: mac)), "\(refused)")
        }
        XCTAssertEqual(looked, [], "nothing looked at for a refused request")
        // A registry that can't be read: nothing looked at, and the core can't look.
        var unread = mac
        unread.openWorktrees = { nil }
        let blind = HostBridge.answer(line(["action": "processes", "folder": "/Users/ada/wt"]), token: token, actions: unread)
        XCTAssertEqual(blind["error"] as? String, "This Mac couldn't read the team's registry.")
        XCTAssertEqual(looked, [])
        let some = HostBridge.answer(line(["action": "processes", "folder": "/Users/ada/wt", "session": session]), token: token, actions: mac)
        let named = some["processes"] as? [[String: Any]]
        XCTAssertEqual(named?.count, HostBridge.processesMost)
        XCTAssertEqual(named?.first?["pid"] as? Int, 1)
        XCTAssertEqual(named?.first?["why"] as? String, "folder")
        XCTAssertTrue(isOK(HostBridge.answer(line(["action": "processes", "folder": "/Users/ada/wt"]), token: token, actions: mac)))
        var nowhere = mac
        nowhere.openWorktrees = { ["/nowhere"] }
        let cant = HostBridge.answer(line(["action": "processes", "folder": "/nowhere"]), token: token, actions: nowhere)
        XCTAssertEqual(cant["error"] as? String, "This Mac couldn't list its processes.")
        XCTAssertEqual(looked, ["/Users/ada/wt \(session)", "/Users/ada/wt ", "/nowhere "])
    }

    /// The registry `processes` reads (RM5c S2): the open hires' worktrees, as the core writes them; none without a
    /// registry; nil for one that can't be read, which is left where it is, as it is.
    func testTheProcessesReadTheOpenHiresWorktreesFromTheRegistry() throws {
        let file = folder.appendingPathComponent("registry.json")
        XCTAssertEqual(HostBridge.openWorktrees(in: file), [])
        let registry: [String: Any] = ["version": 1, "hires": [
            ["id": "a", "state": "working", "worktree": "/Users/ada/repo-hires/ada-abcdef", "engine": "t3", "t3Session": "x"],
            ["id": "b", "state": "starting", "worktree": "/Users/ada/repo-hires/bob-123456"],
            ["id": "c", "state": "fired", "worktree": "/Users/ada/repo-hires/cy-654321"],
            ["id": "d", "state": "failed", "worktree": "/Users/ada/repo-hires/di-111111"],
            ["id": "e", "state": "working", "worktree": ""],
            ["id": "f", "state": "idle"],
        ]]
        try JSONSerialization.data(withJSONObject: registry).write(to: file)
        XCTAssertEqual(HostBridge.openWorktrees(in: file), ["/Users/ada/repo-hires/ada-abcdef", "/Users/ada/repo-hires/bob-123456"])
        for broken in ["{", "[]", #"{"hires":7}"#] {
            try Data(broken.utf8).write(to: file)
            XCTAssertNil(HostBridge.openWorktrees(in: file), broken)
            XCTAssertEqual(try Data(contentsOf: file), Data(broken.utf8), "left as it is")
        }
        XCTAssertTrue(HostBridge.isSessionID("6f1c2a9e-0b3d-4e5f-8a7b-1c2d3e4f5a6b"))
        XCTAssertFalse(HostBridge.isSessionID(""))
    }

    /// The processes, for real (M5c): the kernel's own list, by the code the Mac's Let go uses. A process working in
    /// the folder, one given it with --add-dir and one naming the session are each found, with why and their names;
    /// without the session, only the first two. Nothing is signalled: each is still running after.
    func testTheProcessesAreReadFromTheKernel() throws {
        let wt = folder.appendingPathComponent("wt/sub", isDirectory: true)
        let other = folder.appendingPathComponent("other", isDirectory: true)
        try FileManager.default.createDirectory(at: wt, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: other, withIntermediateDirectories: true)
        let session = "6f1c2a9e-0b3d-4e5f-8a7b-" + String(UUID().uuidString.suffix(12)).lowercased()
        let transcript = other.appendingPathComponent(session + ".jsonl")
        try Data().write(to: transcript)
        func start(_ program: String, _ arguments: [String], in place: URL) throws -> Process {
            let process = Process()
            process.executableURL = URL(fileURLWithPath: program)
            process.arguments = arguments
            process.currentDirectoryURL = place
            try process.run()
            return process
        }
        let worktree = folder.appendingPathComponent("wt").path
        var mac = actions
        mac.openWorktrees = { [worktree] }
        let sleeper = try start("/bin/sleep", ["30"], in: wt)
        let adder = try start("/bin/sh", ["-c", "/bin/sleep 30; :", "sh", "--add-dir", worktree], in: other)
        let tail = try start("/usr/bin/tail", ["-f", transcript.path], in: other)
        let all = [sleeper, adder, tail]
        defer { for process in all { process.terminate(); process.waitUntilExit() } }
        func ask(_ session: String) -> [Int32: [String: Any]] {
            var request: [String: Any] = ["action": "processes", "folder": worktree, "token": token]
            if !session.isEmpty { request["session"] = session }
            let answer = HostBridge.answer(line(request), token: token, actions: mac)
            var byPid: [Int32: [String: Any]] = [:]
            for one in answer["processes"] as? [[String: Any]] ?? [] { byPid[Int32(one["pid"] as? Int ?? 0)] = one }
            return byPid
        }
        var found: [Int32: [String: Any]] = [:]
        let by = Date().addingTimeInterval(5)
        repeat {
            found = ask(session)
            if all.allSatisfy({ found[$0.processIdentifier] != nil }) { break }
            Thread.sleep(forTimeInterval: 0.05)
        } while Date() < by
        XCTAssertEqual(found[sleeper.processIdentifier]?["why"] as? String, "folder")
        XCTAssertEqual(found[sleeper.processIdentifier]?["name"] as? String, "sleep")
        XCTAssertEqual(found[adder.processIdentifier]?["why"] as? String, "add-dir")
        XCTAssertEqual(found[tail.processIdentifier]?["why"] as? String, "session")
        XCTAssertEqual(found[tail.processIdentifier]?["name"] as? String, "tail")
        let alone = ask("")
        XCTAssertNotNil(alone[sleeper.processIdentifier])
        XCTAssertNotNil(alone[adder.processIdentifier])
        XCTAssertNil(alone[tail.processIdentifier], "no session asked: not looked for")
        XCTAssertEqual(Set(Processes.working(in: worktree)), Set(alone.keys), "the same as the Mac's own Let go finds")
        XCTAssertTrue(all.allSatisfy(\.isRunning), "nothing signalled")
    }

    /// T3 Code's app (C4 A1): only T3 Code's own link to a thread, exactly as `T3Desk.deepLink` makes it, reaches the
    /// opener; the app looked for and opened by the stand-ins only.
    func testT3CodesAppAndOnlyItsThreadLinks() {
        var mac = actions
        mac.main = terminals
        mac.looking = terminals
        var opened: [String] = []
        mac.t3App = { false }
        mac.t3OpenLink = { opened.append($0.absoluteString); return $0.absoluteString.hasSuffix("abc-1") }
        let made = T3Desk.deepLink(thread: "mcp:abc-1", link: "https://app.t3.codes/threads/local/mcp:abc-1")!.absoluteString
        XCTAssertEqual(made, "t3code://threads/local/mcp%3Aabc-1")
        XCTAssertEqual(HostBridge.t3ThreadLink(made)?.absoluteString, made)
        for refused in ["https://app.t3.codes/threads/local/mcp%3Aabc-1", "t3code://threads/local/mcp:abc-1", "t3code://threads/lo cal/x",
                        "t3code://threads/local/x?y=1", "t3code://threads/local/x#y", "t3code://evil/local/x", "t3code://threads/local",
                        "t3code://threads/local/x/y", "t3code://threads//x", "t3code://threads/local/", "file:///etc/passwd",
                        "t3code://u@threads/local/x", "t3code://threads:80/local/x", "t3code://threads/local/" + String(repeating: "a", count: 1100)] {
            XCTAssertNil(HostBridge.t3ThreadLink(refused), refused)
            XCTAssertFalse(isOK(HostBridge.answer(line(["action": "t3-link", "link": refused]), token: token, actions: mac)), refused)
        }
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "t3-link"]), token: token, actions: mac)))
        XCTAssertEqual(opened, [], "no refused link reached the opener")
        XCTAssertTrue(isOK(HostBridge.answer(line(["action": "t3-link", "link": made]), token: token, actions: mac)))
        let other = T3Desk.deepLink(thread: "mcp:abc-2", link: "https://app.t3.codes/threads/local/x")!.absoluteString
        XCTAssertEqual(HostBridge.answer(line(["action": "t3-link", "link": other]), token: token, actions: mac)["error"] as? String,
                       "T3 Code didn't take the link to the hire's thread.")
        XCTAssertEqual(opened, [made, other])
        XCTAssertEqual(HostBridge.answer(line(["action": "t3-app"]), token: token, actions: mac)["found"] as? Bool, false)
    }

    func testSetupsMacPartsTakeOnlyWhatTheyShould() {
        var mac = actions
        mac.main = terminals
        mac.facts = { _ in ["accessibility": true] }
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "facts", "apps": "com.example.notes"]), token: token, actions: mac)), "apps not a list")
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "facts", "apps": Array(repeating: "a.b", count: 51)]), token: token, actions: mac)))
        XCTAssertTrue(isOK(HostBridge.answer(line(["action": "facts"]), token: token, actions: mac)), "no apps: none")
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "dock"]), token: token, actions: mac)), "neither on nor off")
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "abilities", "tabs": Array(repeating: "tty:/dev/ttys001", count: 257)]), token: token, actions: mac)))
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "bots-resolve", "bots": [["bot": "No id"]]]), token: token, actions: mac)))
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "bots-resolve", "bots": [["botId": "../0a1b", "bot": "Mini"]]]), token: token, actions: mac)),
                       "an id no bot can have")
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "keystroke-apps", "app": "com.example.mail"]), token: token, actions: mac)), "neither on nor off")
        // No roster to read: nobody found, as the Swift engine answers.
        let none = HostBridge.answer(line(["action": "bots-resolve", "bots": [["botId": "", "bot": "Mini"]]]), token: token, actions: mac)
        XCTAssertEqual((none["resolved"] as? [Any])?.first is NSNull, true)
        // A tab Smolcorp can only watch, or none at all: nothing offered.
        XCTAssertEqual(HostBridge.abilities(of: "headless:3f2a9c1e"), ["keys": false, "text": false, "fire": false, "screen": false])
        XCTAssertEqual(HostBridge.abilities(of: "not a tab"), ["keys": false, "text": false, "fire": false, "screen": false])
    }

    private func line(_ request: [String: Any]) -> Data {
        var request = request
        request["token"] = token
        return try! JSONSerialization.data(withJSONObject: request)
    }

    func testEachLaunchHasItsOwnToken() {
        let one = HostBridge.newToken(), two = HostBridge.newToken()
        XCTAssertEqual(one.count, 64)
        XCTAssertNotEqual(one, two)
    }
}

/// A stand-in terminal for the phone's text over the bridge: nothing real is asked. Several lines go through
/// the clipboard with ⌘V, as Terminal's and iTerm2's do; once its checks (`ready`, asked `readies` times, as
/// `ClipboardPaste.paste` asks before the clipboard is touched and right before ⌘V) let the paste go, it
/// answers `pastes`, noting what it typed: "insert <tab> <text>", or "paste <tab> <text>" where ⌘V went.
private final class PhoneTerminal: TerminalDriver {
    let linesWay: LinesWay
    private let pastes: LinesPasted
    private let readies: Int
    private let note: (String) -> Void

    init(way: LinesWay = .clipboard(inFront: true), pastes: LinesPasted, readies: Int = 1, note: @escaping (String) -> Void) {
        linesWay = way
        self.pastes = pastes
        self.readies = readies
        self.note = note
    }

    var abilities: TerminalAbilities { [.keys, .text] }
    var keysNeedAccessibility: Bool { true }
    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool { false }
    func type(_ text: String, in tab: TabAddress) -> Bool { false }
    func insert(_ text: String, in tab: TabAddress) -> Bool {
        note("insert \(tab.text) \(text)")
        return true
    }
    func insertLines(_ text: String, in tab: TabAddress, ready: () -> String?) -> LinesPasted {
        for _ in 0..<readies {
            if let why = ready() { return .refused(why) }
        }
        if [.pasted, .notRead, .failed].contains(pastes) { note("paste \(tab.text) \(text)") }
        return pastes
    }
    func focus(_ tab: TabAddress) -> Bool { false }
    func scroll(lines: Int, in tab: TabAddress) -> Bool { false }
    func scrollToBottom(in tab: TabAddress) -> Bool { false }
    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired { .notStopped }
    func tty(of tab: TabAddress) -> String? { nil }
    /// Whether a tab is open, for the agents-ended look: unknown unless a test says.
    var open: ((TabAddress) -> Bool?)?
    func isOpen(_ tab: TabAddress) -> Bool? { open?(tab) ?? nil }
    func screen(of tab: TabAddress) -> String? { nil }
}
