import XCTest
@testable import AgentDeck

/// The hire engine, with temp repos and homes and a stand-in tmux: nothing real is started.
final class HireDeskTests: XCTestCase {

    func testTheApiKeyAccountNeedsAKeyInTheKeychain() {
        XCTAssertEqual(HireDesk.apiKeyRefusal(picked: true, offered: false)?.status, 409)
        XCTAssertNil(HireDesk.apiKeyRefusal(picked: true, offered: true))
        XCTAssertNil(HireDesk.apiKeyRefusal(picked: false, offered: false))
    }

    func testTheApiKeyNoticeSaysTheHireCanReadTheKey() {
        XCTAssertTrue(HireDesk.apiKeyNotice.contains("The hire can read this key."))
    }
}
