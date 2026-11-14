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

    /// A translocated launch of an app that sits in Applications with the download's mark: told the command and still offered the move (it may be the Downloads copy).
    func testATranslocatedAppInApplicationsWithQuarantineIsToldTheCommand() {
        let home = "/Users/someone"
        let moved = URL(fileURLWithPath: "/private/var/folders/xy/T/AppTranslocation/1234-5678/d/Smolcorp.app")
        for original in ["/Applications/Smolcorp.app", "\(home)/Applications/Smolcorp.app"] {
            let check = SelfTest.installCheck(bundle: moved, testCopy: false, home: home, quarantined: { $0 == original })
            XCTAssertEqual(check.status, .fix, original)
            XCTAssertEqual(check.fix, SelfTest.install, "the move stays: the running copy may be the one just downloaded")
            XCTAssertEqual(check.label, "Move to Applications")
            XCTAssertTrue(check.detail.contains("xattr -d com.apple.quarantine '\(original)'"), check.detail)
            XCTAssertTrue(check.detail.contains("If this is the copy you just downloaded, move it to Applications."), check.detail)
        }
        let spaced = "/Users/Jean Dupont/Applications/Smolcorp.app"
        let spacedCheck = SelfTest.installCheck(bundle: moved, testCopy: false, home: "/Users/Jean Dupont",
                                                quarantined: { $0 == spaced })
        XCTAssertTrue(spacedCheck.detail.contains("xattr -d com.apple.quarantine '\(spaced)'"), spacedCheck.detail)
        let unknown = SelfTest.installCheck(bundle: moved, testCopy: false, home: home, quarantined: { _ in false })
        XCTAssertEqual(unknown.fix, SelfTest.install, "no marked copy found: still offered the move")
        XCTAssertTrue(unknown.detail.contains("xattr -d com.apple.quarantine '/Applications/Smolcorp.app'"))
        let download = SelfTest.installCheck(bundle: URL(fileURLWithPath: "\(home)/Downloads/Smolcorp.app"), testCopy: false,
                                             home: home, quarantined: { _ in true })
        XCTAssertEqual(download.fix, SelfTest.install, "from Downloads it is still moved, whatever the mark")
        XCTAssertFalse(download.detail.contains("xattr"))
    }
}
