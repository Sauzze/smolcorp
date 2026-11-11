import Foundation
import XCTest
@testable import AgentDeck

/// A test copy never brings another app forward or touches a real terminal app (RTCH N1): the terminal
/// actions, the Grok desk tap, T3 Code's open, Accessibility's prompt and System Settings, the self-test's
/// fixes and the Tiler each answer in a plain sentence that a test copy doesn't do that. (The bridge's and
/// T3's own refusals are tested with their subjects: HostBridgeTests, T3DeskOfficeTests.)
final class TestCopyOtherAppsTests: XCTestCase {
    private func read(_ name: String) throws -> String {
        let sources = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("Sources/AgentDeck")
        return try String(contentsOf: sources.appendingPathComponent(name), encoding: .utf8)
    }

    func testEachRefusalIsOneSentenceThatSaysATestCopyDoesNotDoThis() {
        XCTAssertEqual(TestCopy.OtherApp.allCases.count, 8)
        for app in TestCopy.OtherApp.allCases {
            XCTAssertTrue(app.sentence.hasPrefix("This is a test copy of Smolcorp: it never "), "\(app)")
            XCTAssertTrue(app.sentence.hasSuffix("."), "\(app)")
            XCTAssertEqual(TestCopy.refusal(for: app, testCopy: true), app.sentence)
            XCTAssertNil(TestCopy.refusal(for: app, testCopy: false), "the real app does all of them")
        }
        XCTAssertNil(TestCopy.refusal(for: .terminal), "the suite is not a test copy: nothing here is refused")
    }

    func testASelfTestFixThatOpensAnotherAppIsRefusedOnATestCopy() {
        for (fix, app) in [(SelfTest.accessibility, TestCopy.OtherApp.accessibility), (SelfTest.tailscale, .fix), (GMPreflight.automationFix, .fix)] {
            XCTAssertThrowsError(try SelfTest.fix(fix, testCopy: true), fix) { error in
                XCTAssertEqual(error as? SelfTest.FixError, .otherApp(app))
                XCTAssertEqual(error.localizedDescription, app.sentence)
            }
        }
        // The ones that stay in the copy's own folder are as they were: an unknown name is still unknown.
        XCTAssertThrowsError(try SelfTest.fix("nope", testCopy: true)) { XCTAssertEqual($0 as? SelfTest.FixError, .unknown) }
    }

    func testATestCopysHiresAlwaysRunInTmuxNeverInATerminalWindow() {
        var config = Config()
        config.hiresInBackground = false
        XCTAssertFalse(Choices.background(config, hasTmux: true, testCopy: false))
        XCTAssertTrue(Choices.background(config, hasTmux: true, testCopy: true))
        XCTAssertFalse(Choices.background(config, hasTmux: false, testCopy: true), "with no tmux the hire is refused instead")
        // RWTC N8: that refusal, before anything is made; the real app opens its window as ever.
        XCTAssertEqual(Deck.testCopyWindowRefusal(config, hasTmux: false, testCopy: true), TestCopy.OtherApp.terminal.sentence)
        XCTAssertNil(Deck.testCopyWindowRefusal(config, hasTmux: true, testCopy: true), "in tmux, whatever the switch says")
        XCTAssertNil(Deck.testCopyWindowRefusal(config, hasTmux: false, testCopy: false))
        config.hiresInBackground = true
        XCTAssertEqual(Deck.testCopyWindowRefusal(config, hasTmux: false, testCopy: true), TestCopy.OtherApp.terminal.sentence)
    }

    func testATestCopyOpensNoWindowOfAHiresTerminal() {
        let deck = Deck(quiet: true)
        var answered: String?
        deck.openHireWindow("any", testCopy: true) { answered = $0 ?? "opened" }
        XCTAssertEqual(answered, TestCopy.OtherApp.terminal.sentence)
    }

    func testATestCopyIsNeverInTheDock() {
        XCTAssertEqual(Dock.policy(showing: true, testCopy: false), .regular)
        XCTAssertEqual(Dock.policy(showing: true, testCopy: true), .accessory)
        XCTAssertEqual(Dock.policy(showing: false, testCopy: true), .accessory)
    }

    // MARK: The desk's tap

    /// A deck of its own, with a stand-in for every terminal: desk "s1" a Claude Code session in `tab`.
    private func deck(tab: String, testCopy: Bool) -> (Deck, TellingTerminal) {
        let deck = Deck(quiet: true)
        deck.layoutConfig = Config()
        deck.cloud.configure([])
        deck.isTestCopy = testCopy
        deck.sessions.apply(["hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/work/api"], agent: .claude, tab: tab, startedIn: "", at: 10)
        deck.sessions.apply(["hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"], agent: .claude, tab: tab, startedIn: "", at: 20)
        let terminal = TellingTerminal()
        Terminals.standIn = terminal
        addTeardownBlock { Terminals.standIn = nil }
        return (deck, terminal)
    }

    private func answered(_ act: (@escaping (Response) -> Void) -> Void) -> Response? {
        var got: Response?
        let done = expectation(description: "answered")
        act { got = $0; done.fulfill() }
        wait(for: [done], timeout: 10)
        return got
    }

    private func text(_ response: Response?) -> String { response.map { String(decoding: $0.body, as: UTF8.self) } ?? "" }

    func testATestCopysTapOnAGrokBotsDeskOpensNoAppButSeesItsNews() throws {
        let deck = Deck(quiet: true)
        deck.layoutConfig = Config()
        var bot = CloudBot(id: "grok-1", name: "Grok one")
        bot.source = CloudBot.Source(kind: "grokBotApp", bot: "Grok one", counts: "unprompted")
        deck.cloud.configure([bot])
        XCTAssertTrue(deck.cloud.isGrokApp(session: bot.sessionID))
        XCTAssertFalse(deck.cloud.isGrokApp(session: "cloud-nobody"))
        _ = deck.cloud.apply(.report, to: bot.id, action: CloudAction(text: "Sent a report", count: 1, at: Date()))
        XCTAssertEqual(deck.cloud.seated().first?.session.state, .unread)
        deck.isTestCopy = true
        let response = answered { deck.focus(bot.sessionID, done: $0) }
        XCTAssertEqual(response?.status, 409)
        XCTAssertEqual(text(response), TestCopy.OtherApp.grok.sentence)
        // RWTC N9: only the open is left out; the tap still means it was looked at.
        XCTAssertEqual(deck.cloud.seated().first?.session.state, .thinking, "its news is seen")
    }

    /// A test copy brings no terminal forward, its own hires' panes included (their window would be the owner's
    /// terminal app); the real app does.
    func testATestCopysTapOnADeskNeverBringsItsTerminalForward() {
        for tab in ["iterm:AB12-CD34", "tmux:%3@/private/tmp/tmux-501/smolcorp-7781-h1"] {
            let (copy, terminal) = deck(tab: tab, testCopy: true)
            let refused = answered { copy.focus("s1", done: $0) }
            XCTAssertEqual(refused?.status, 409, tab)
            XCTAssertEqual(text(refused), TestCopy.OtherApp.terminal.sentence, tab)
            XCTAssertEqual(terminal.told, [], "\(tab): the terminal was told nothing")
        }
        let (office, terminal) = deck(tab: "iterm:AB12-CD34", testCopy: false)
        _ = answered { office.focus("s1", done: $0) }
        XCTAssertEqual(terminal.told, ["focus iterm:AB12-CD34"])
    }

    // MARK: Tabs that aren't its own (RWTC N1)

    func testATabIsTheCopysOwnOnlyAsATmuxPaneOnItsOwnHireSocket() {
        let own = "tmux:%3@/private/tmp/tmux-501/smolcorp-7781-h1"
        XCTAssertNil(TestCopy.refusal(forTab: own, testCopy: true, testPort: 7781))
        for tab in ["iterm:AB12-CD34", "tty:/dev/ttys003", "ghostty:/dev/ttys004", "other:/dev/ttys005", "kitty:3@/tmp/kitty-501",
                    "wezterm:4@/tmp/wezterm", "tmux:%3@/private/tmp/tmux-501/default", "tmux:%3", "launcher:L1", "nonsense", "",
                    "tmux:%3@/private/tmp/tmux-501/smolcorp-h1", // the real app's hire
                    "tmux:%3@/private/tmp/tmux-501/smolcorp-7782-h1"] { // another copy's
            XCTAssertEqual(TestCopy.refusal(forTab: tab, testCopy: true, testPort: 7781), TestCopy.OtherApp.terminal.sentence, tab)
            XCTAssertNil(TestCopy.refusal(forTab: tab, testCopy: false, testPort: nil), "\(tab): the real app does everything")
        }
    }

    /// Every phone control, the phone's words, a typed answer and a peek: refused on a test copy in an owner's tab,
    /// with nothing said to the terminal.
    func testATestCopyDoesNothingInATabThatIsntItsOwn() throws {
        let (deck, terminal) = deck(tab: "iterm:AB12-CD34", testCopy: true)
        var controls = ["accept", "send", "reject", "esc", "up", "down", "clear", "fire", "effort", "mic", "button"]
        controls += ["scroll", "choose"]
        for action in controls {
            let response = answered { deck.perform(action, on: "s1", query: ["level": "high", "lines": "3", "button": "clear", "force": "1",
                                                                              "phase": "down", "option": "1"], done: $0) }
            XCTAssertEqual(response?.status, 409, action)
            XCTAssertEqual(text(response), TestCopy.OtherApp.terminal.sentence, action)
        }
        let typed = answered { deck.typePhoneDictation("hello", into: "s1", send: true, done: $0) }
        XCTAssertEqual(text(typed), TestCopy.OtherApp.terminal.sentence)
        let peeked = answered { deck.peek("s1", done: $0) }
        XCTAssertEqual(text(peeked), TestCopy.OtherApp.terminal.sentence)
        XCTAssertEqual(terminal.told, [], "the terminal was told nothing")
    }

    /// Its own hire's pane is driven as ever.
    func testATestCopysOwnHiresPaneIsDrivenAsEver() {
        let (deck, terminal) = deck(tab: "tmux:%3@/private/tmp/tmux-501/smolcorp-h1", testCopy: true)
        XCTAssertNil(TestCopy.refusal(forTab: "tmux:%3@/private/tmp/tmux-501/smolcorp-h1", testCopy: true, testPort: nil))
        let response = answered { deck.perform("accept", on: "s1", query: [:], done: $0) }
        XCTAssertEqual(response?.status, 204, text(response))
        XCTAssertEqual(terminal.told, ["press tmux:%3@/private/tmp/tmux-501/smolcorp-h1"])
    }

    // MARK: The jukebox (RWTC N2)

    func testATestCopysJukeboxPlaysNothingAndLeavesTheVolume() {
        let deck = Deck(quiet: true)
        deck.isTestCopy = true
        for query in [["do": "playpause"], ["do": "next"], ["do": "volume", "level": "30"]] {
            let response = answered { deck.perform("music", on: "", query: query, done: $0) }
            XCTAssertEqual(response?.status, 409, "\(query)")
            XCTAssertEqual(text(response), TestCopy.OtherApp.music.sentence, "\(query)")
        }
    }

    // MARK: The routes

    /// The setup routes that would raise a system prompt, move another app's windows or reach the owner's music or
    /// microphone answer in a sentence on a test copy; turning the jukebox or dictation off is as ever (RWTC N2, N8).
    func testTheRoutesThatMoveOrPromptRefuseOnATestCopy() {
        let refused: [(String, [String: String], TestCopy.OtherApp)] = [
            ("/setup/accessibility", [:], .accessibility), ("/setup/gm/tile", [:], .tiler),
            ("/setup/music", ["enabled": "1"], .music), ("/setup/dictation", ["enabled": "1", "language": "en_US"], .microphone),
        ]
        for (path, query, app) in refused {
            XCTAssertEqual(TestCopy.routeRefusal("POST", path, query: query, testCopy: true), app.sentence, path)
            XCTAssertNil(TestCopy.routeRefusal("POST", path, query: query, testCopy: false), "\(path): the real app does it")
            XCTAssertNil(TestCopy.routeRefusal("GET", path, query: query, testCopy: true), "\(path): only a POST acts")
        }
        for (path, query) in [("/setup/music", ["enabled": "0"]), ("/setup/dictation", ["enabled": "0"]), ("/setup/dictation", ["language": "fr_FR"]),
                              ("/setup/hiring", [:])] {
            XCTAssertNil(TestCopy.routeRefusal("POST", path, query: query, testCopy: true), "\(path) \(query)")
        }
    }

    // MARK: Where a copy listens

    /// RTCH N3: both engines, and the script, read `lan` the same way: spaces and letter case don't matter.
    func testLANIsReadTheSameWayEverywhere() {
        for text in ["lan", "LAN", " lan", "lan\n", "  Lan  "] {
            XCTAssertTrue(TestCopy.wantsLAN(["SMOLCORP_TEST_LISTEN": text]), text)
            XCTAssertEqual(TestCopy.listenScope(testCopy: true, environment: ["SMOLCORP_TEST_LISTEN": text]), .everywhere, text)
        }
        for text in ["", "all", "la n", "lans"] {
            XCTAssertFalse(TestCopy.wantsLAN(["SMOLCORP_TEST_LISTEN": text]), text)
            XCTAssertEqual(TestCopy.listenScope(testCopy: true, environment: ["SMOLCORP_TEST_LISTEN": text]), .thisMac, text)
        }
        XCTAssertFalse(TestCopy.wantsLAN([:]))
    }

    /// RWTC N6: scripts/test-copy.sh says where the copy listens by the same reading: its own line, run by bash.
    func testTheScriptReadsLANAsBothEnginesDo() throws {
        let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let script = try String(contentsOf: repo.appendingPathComponent("scripts/test-copy.sh"), encoding: .utf8)
        let line = try XCTUnwrap(script.split(separator: "\n").first { $0.hasSuffix(#"&& WHERE="the local network too""#) })
        let condition = try XCTUnwrap(line.components(separatedBy: " && WHERE=").first)
        for text in ["lan", "LAN", " lan", "lan\n", "  Lan  ", "\tlan\r\n", "", "all", "la n", "la\nn", "lans", "l an"] {
            let bash = Process()
            bash.executableURL = URL(fileURLWithPath: "/bin/bash")
            bash.arguments = ["-c", condition + " && echo yes || echo no"]
            bash.environment = ["SMOLCORP_TEST_LISTEN": text, "PATH": "/usr/bin:/bin"]
            let output = Pipe()
            bash.standardOutput = output
            try bash.run()
            let said = String(decoding: output.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
            bash.waitUntilExit()
            XCTAssertEqual(said == "yes\n", TestCopy.wantsLAN(["SMOLCORP_TEST_LISTEN": text]), text.debugDescription)
        }
    }

    /// RTCH N4: `swift test` with SMOLCORP_TEST_LISTEN=lan in its environment never puts the suite's servers on the
    /// network: the suite's scope is read without the environment.
    func testTheSuiteIgnoresAnInheritedLAN() throws {
        XCTAssertEqual(TestCopy.listensOn, .thisMac)
        XCTAssertTrue(try read("Paths.swift").contains("environment: isOn ? ProcessInfo.processInfo.environment : [:])"))
    }
}

/// A terminal for every tab that keeps what it's told and reaches nothing real.
private final class TellingTerminal: TerminalDriver {
    private(set) var told: [String] = []
    var abilities: TerminalAbilities { [.keys, .text, .focus, .fire, .scroll] }
    var keysNeedAccessibility: Bool { false }
    var showsScreen: Bool { true }
    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool { told.append("press \(tab.text)"); return true }
    func type(_ text: String, in tab: TabAddress) -> Bool { told.append("type \(tab.text)"); return true }
    func insert(_ text: String, in tab: TabAddress) -> Bool { told.append("insert \(tab.text)"); return true }
    func focus(_ tab: TabAddress) -> Bool { told.append("focus \(tab.text)"); return true }
    func scroll(lines: Int, in tab: TabAddress) -> Bool { told.append("scroll \(tab.text)"); return true }
    func scrollToBottom(in tab: TabAddress) -> Bool { told.append("bottom \(tab.text)"); return true }
    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired { told.append("fire \(tab.text)"); return .notStopped }
    func tty(of tab: TabAddress) -> String? { nil }
    func isOpen(_ tab: TabAddress) -> Bool? { nil }
    func screen(of tab: TabAddress) -> String? { told.append("screen \(tab.text)"); return "" }
}
