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
        for app in [TestCopy.OtherApp.terminal, .tiler, .grok, .t3Code, .accessibility, .fix] {
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

    func testATestCopysTapOnAGrokBotsDeckOpensNoApp() throws {
        let deck = Deck(quiet: true)
        deck.layoutConfig = Config()
        var bot = CloudBot(id: "grok-1", name: "Grok one")
        bot.source = CloudBot.Source(kind: "grokBotApp", bot: "Grok one", counts: "unprompted")
        deck.cloud.configure([bot])
        XCTAssertTrue(deck.cloud.isGrokApp(session: bot.sessionID))
        XCTAssertFalse(deck.cloud.isGrokApp(session: "cloud-nobody"))
        var response: Response?
        deck.focus(bot.sessionID, testCopy: true) { response = $0 }
        XCTAssertEqual(response?.status, 409)
        XCTAssertEqual(String(decoding: try XCTUnwrap(response).body, as: UTF8.self), TestCopy.OtherApp.grok.sentence)
    }

    func testATestCopysTapOnADeskNeverBringsItsTerminalForward() throws {
        let deck = try read("Deck.swift")
        let focus = try XCTUnwrap(deck.range(of: "func focus(_ id: String, testCopy: Bool = TestCopy.isOn")).upperBound
        let body = String(deck[focus...].prefix(1800))
        let refusal = try XCTUnwrap(body.range(of: "if testCopy { return done(.status(409, TestCopy.OtherApp.terminal.sentence)) }"))
        let drive = try XCTUnwrap(body.range(of: "driver.focus(tab)"))
        XCTAssertLessThan(refusal.lowerBound, drive.lowerBound, "refused before the driver is asked")
        let grok = try XCTUnwrap(body.range(of: "TestCopy.OtherApp.grok.sentence"))
        XCTAssertLessThan(grok.lowerBound, try XCTUnwrap(body.range(of: "cloud.tapped(session: id)")).lowerBound, "before the Grok app is opened")
    }

    // MARK: The routes

    /// The setup page's routes that would raise a system prompt or move another app's windows answer before they act.
    func testTheRoutesThatMoveOrPromptRefuseFirst() throws {
        let router = try read("DeckRouter.swift")
        let accessibility = try XCTUnwrap(router.range(of: "case (\"POST\", \"/setup/accessibility\"):")).upperBound
        let body = String(router[accessibility...].prefix(600))
        XCTAssertLessThan(try XCTUnwrap(body.range(of: "TestCopy.refusal(for: .accessibility)")).lowerBound,
                          try XCTUnwrap(body.range(of: "Permissions.requestAccessibility()")).lowerBound)
        let tile = try XCTUnwrap(router.range(of: "case (\"POST\", \"/setup/gm/tile\"):")).upperBound
        let tiling = String(router[tile...].prefix(600))
        XCTAssertLessThan(try XCTUnwrap(tiling.range(of: "TestCopy.refusal(for: .tiler)")).lowerBound,
                          try XCTUnwrap(tiling.range(of: "Tiler.screens()")).lowerBound)
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

    /// RTCH N4: `swift test` with SMOLCORP_TEST_LISTEN=lan in its environment never puts the suite's servers on the
    /// network: the suite's scope is read without the environment.
    func testTheSuiteIgnoresAnInheritedLAN() throws {
        XCTAssertEqual(TestCopy.listensOn, .thisMac)
        XCTAssertTrue(try read("Paths.swift").contains("environment: isOn ? ProcessInfo.processInfo.environment : [:])"))
    }
}
