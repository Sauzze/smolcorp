import XCTest
@testable import AgentDeck

final class SelfTestTests: XCTestCase {
    /// In Applications it's installed; from Downloads, a disk image or App Translocation's copy, it's moved there.
    func testOnlyACopyInApplicationsIsInstalled() {
        let home = "/Users/someone"
        for installed in ["/Applications/Smolcorp.app", "\(home)/Applications/Smolcorp.app"] {
            let check = SelfTest.installCheck(bundle: URL(fileURLWithPath: installed), testCopy: false, home: home)
            XCTAssertEqual(check.status, .ok, installed)
        }
        for elsewhere in ["\(home)/Downloads/Smolcorp.app", "/Volumes/Smolcorp/Smolcorp.app",
                          "/private/var/folders/xy/T/AppTranslocation/1234-5678/d/Smolcorp.app"] {
            let check = SelfTest.installCheck(bundle: URL(fileURLWithPath: elsewhere), testCopy: false, home: home)
            XCTAssertEqual(check.status, .fix, elsewhere)
            XCTAssertEqual(check.fix, SelfTest.install, elsewhere)
            XCTAssertEqual(check.label, "Move to Applications")
        }
        let test = SelfTest.installCheck(bundle: URL(fileURLWithPath: "/work/build/Smolcorp Test.app"), testCopy: true, home: home)
        XCTAssertEqual(test.status, .ok, "a test copy stays where it was built")
        XCTAssertNil(test.fix)
    }

    func testEveryAgentFoundMustBeConnected() {
        let none = SelfTest.hooksCheck(found: [], connected: [], scriptThere: true, testCopy: false)
        XCTAssertEqual(none.status, .fix)
        XCTAssertNil(none.fix, "installing an agent is yours to do")
        let one = SelfTest.hooksCheck(found: ["Claude Code", "Codex"], connected: ["Claude Code"], scriptThere: true, testCopy: false)
        XCTAssertEqual(one.status, .fix)
        XCTAssertEqual(one.fix, SelfTest.repair)
        XCTAssertEqual(one.detail, "Codex isn't connected.")
        let noScript = SelfTest.hooksCheck(found: ["Claude Code"], connected: ["Claude Code"], scriptThere: false, testCopy: false)
        XCTAssertEqual(noScript.fix, SelfTest.repair)
        let all = SelfTest.hooksCheck(found: ["Claude Code", "Codex"], connected: ["Claude Code", "Codex"], scriptThere: true, testCopy: false)
        XCTAssertEqual(all.status, .ok)
        XCTAssertTrue(all.detail.hasPrefix("Claude Code, Codex:"))
        XCTAssertEqual(SelfTest.hooksCheck(found: [], connected: [], scriptThere: false, testCopy: true).status, .info)
    }

    /// A step still to take, as pairing a phone or allowing the keys, isn't in the way; a fix is.
    func testOnlyAFixIsInTheWay() {
        XCTAssertEqual(SelfTest.accessibilityCheck(false).status, .todo)
        XCTAssertEqual(SelfTest.accessibilityCheck(false).fix, SelfTest.accessibility)
        XCTAssertEqual(SelfTest.phoneCheck(0).status, .todo)
        XCTAssertEqual(SelfTest.phoneCheck(1).status, .ok)
        XCTAssertEqual(SelfTest.networkCheck(tailnet: nil).status, .info)
        XCTAssertTrue(SelfTest.networkCheck(tailnet: "https://mac.example.ts.net").detail.contains("mac.example.ts.net"))
        XCTAssertEqual(SelfTest.loginCheck(false, testCopy: false).fix, SelfTest.login)
        XCTAssertEqual(SelfTest.loginCheck(false, testCopy: true).status, .info)
        let checks = [SelfTest.phoneCheck(0), SelfTest.accessibilityCheck(false), SelfTest.networkCheck(tailnet: nil)]
        XCTAssertTrue(SelfTest.ready(checks))
        XCTAssertFalse(SelfTest.ready(checks + [SelfTest.hooksCheck(found: [], connected: [], scriptThere: true, testCopy: false)]))
    }

    /// The page and the Windows app's terminal read the same JSON.
    func testTheReportIsTheWindowsAppsJSON() throws {
        let report = SelfTest.report([SelfTest.phoneCheck(0), SelfTest.loginCheck(false, testCopy: false)])
        XCTAssertEqual(report["ready"] as? Bool, true)
        let checks = try XCTUnwrap(report["checks"] as? [[String: Any]])
        XCTAssertEqual(checks.map { $0["id"] as? String }, ["phone", "login"])
        XCTAssertEqual(checks[0]["status"] as? String, "todo")
        XCTAssertNil(checks[0]["fix"], "no fix, no key")
        XCTAssertEqual(checks[1]["fix"] as? String, "login")
        XCTAssertEqual(checks[1]["label"] as? String, "Open at login")
        XCTAssertNoThrow(try JSONSerialization.data(withJSONObject: report))
    }

    func testAnUnknownFixIsRefused() {
        XCTAssertThrowsError(try SelfTest.fix("teleport")) { error in
            XCTAssertEqual(error as? SelfTest.FixError, .unknown)
        }
    }
}
