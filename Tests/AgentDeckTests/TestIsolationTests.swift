import XCTest
@testable import AgentDeck

/// No test may touch the owner's own Smolcorp folder: a test that built a whole
/// `Deck` once rewrote the real setup key, and the office in use refused it.
final class TestIsolationTests: XCTestCase {
    func testTheSuiteNeverUsesTheRealSupportFolder() {
        XCTAssertTrue(Paths.underTest)
        let real = Paths.realSupport().standardizedFileURL.path
        for url in [Paths.support, Paths.setupKey, Paths.sessions, Paths.config, Paths.gmToken, Paths.hookHeader] {
            XCTAssertFalse(url.standardizedFileURL.path.hasPrefix(real), "\(url.path) is the real Smolcorp's")
        }
        XCTAssertEqual(Paths.support, Paths.testSupport)
        XCTAssertTrue(Paths.support.path.hasPrefix(FileManager.default.temporaryDirectory.path))
    }

    func testAWholeDeckWritesItsSetupKeyInTheTestFolder() throws {
        let deck = Deck(quiet: true)
        let kept = try String(contentsOf: Paths.setupKey, encoding: .utf8).trimmingCharacters(in: .whitespacesAndNewlines)
        XCTAssertEqual(kept, deck.access.setupKey)
        XCTAssertTrue(Paths.setupKey.path.hasPrefix(Paths.testSupport.path))
    }

    func testTheGMTokenIsNeverTheRealKeychainItemInTests() {
        XCTAssertFalse(GMIdentity.defaultStore(testCopy: false, support: nil) is GMKeychain)
    }
}
