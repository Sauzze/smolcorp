import Foundation
import XCTest
@testable import AgentDeck

/// A test copy never brings another app forward or touches a real terminal app (RTCH N1): the terminal
/// actions, the Grok desk tap, T3 Code's open, Accessibility's prompt and System Settings, the self-test's
/// fixes and the Tiler each answer in a plain sentence that a test copy doesn't do that. Since 1.25.0 the Go
/// core makes those refusals (desktop/testcopyapps_test.go) and the bridge makes them again on this Mac
/// (HostBridgeTests): here are the sentences both share, the rule for a copy's own tab, and the Mac app's own.
final class TestCopyOtherAppsTests: XCTestCase {
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

    func testATestCopyIsNeverInTheDock() {
        XCTAssertEqual(Dock.policy(showing: true, testCopy: false), .regular)
        XCTAssertEqual(Dock.policy(showing: true, testCopy: true), .accessory)
        XCTAssertEqual(Dock.policy(showing: false, testCopy: true), .accessory)
    }

    // MARK: Tabs that aren't its own (RWTC N1)

    func testATabIsTheCopysOwnOnlyAsATmuxPaneOnItsOwnHireSocket() {
        let own = "tmux:%3@/private/tmp/tmux-501/smolcorp-7781-h1"
        XCTAssertNil(TestCopy.refusal(forTab: own, testCopy: true, testPort: 7781))
        // A copy with no port of its own owns the plain hire sockets, as HireTmux says.
        XCTAssertNil(TestCopy.refusal(forTab: "tmux:%3@/private/tmp/tmux-501/smolcorp-h1", testCopy: true, testPort: nil))
        for tab in ["iterm:AB12-CD34", "tty:/dev/ttys003", "ghostty:/dev/ttys004", "other:/dev/ttys005", "kitty:3@/tmp/kitty-501",
                    "wezterm:4@/tmp/wezterm", "tmux:%3@/private/tmp/tmux-501/default", "tmux:%3", "launcher:L1", "nonsense", "",
                    "tmux:%3@/private/tmp/tmux-501/smolcorp-h1", // the real app's hire
                    "tmux:%3@/private/tmp/tmux-501/smolcorp-7782-h1"] { // another copy's
            XCTAssertEqual(TestCopy.refusal(forTab: tab, testCopy: true, testPort: 7781), TestCopy.OtherApp.terminal.sentence, tab)
            XCTAssertNil(TestCopy.refusal(forTab: tab, testCopy: false, testPort: nil), "\(tab): the real app does everything")
        }
    }

    // MARK: Where a copy listens

    /// RTCH N3: both engines, and the script, read `lan` the same way: spaces and letter case don't matter.
    func testLANIsReadTheSameWayEverywhere() {
        for text in ["lan", "LAN", " lan", "lan\n", "  Lan  "] {
            XCTAssertTrue(TestCopy.wantsLAN(["SMOLCORP_TEST_LISTEN": text]), text)
        }
        for text in ["", "all", "la n", "lans"] {
            XCTAssertFalse(TestCopy.wantsLAN(["SMOLCORP_TEST_LISTEN": text]), text)
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
}
