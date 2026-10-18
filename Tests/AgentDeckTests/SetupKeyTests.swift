import XCTest
@testable import AgentDeck

/// The setup page's cookie, read from what the browser sends.
final class SetupKeyTests: XCTestCase {
    private func request(cookie: String?) -> Request {
        Request(method: "GET", path: "/setup/state", query: [:], headers: cookie.map { ["cookie": $0] } ?? [:], body: Data(),
                isLocal: true, isAddressedDirectly: true, remoteAddress: "127.0.0.1")
    }

    func testTheSetupCookieIsFoundAmongOthers() {
        let sent = request(cookie: "theme=dark; smolcorp-setup=abc_DEF-123; other=1")
        XCTAssertEqual(DeckRouter.cookie(DeckRouter.setupCookie, in: sent), "abc_DEF-123")
    }

    func testACookieThatOnlyStartsTheSameIsntIt() {
        XCTAssertNil(DeckRouter.cookie(DeckRouter.setupCookie, in: request(cookie: "smolcorp-setup-old=abc")))
        XCTAssertNil(DeckRouter.cookie(DeckRouter.setupCookie, in: request(cookie: nil)))
    }
}
