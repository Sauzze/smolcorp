import XCTest
@testable import AgentDeck

/// The approvals list: who waits for you, read only. And the proof that nothing
/// the GM can use answers a prompt (that is your tap alone).
final class ApprovalsTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    /// The page lists what waits, answers only with the deck's own /act, and hides itself when the route isn't there.
    func testTheDeckPageListsTheWaitsWithItsOwnAnswerKeys() throws {
        let page = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/deck.html"), encoding: .utf8)
        XCTAssertTrue(page.contains("Waiting for you"))
        let list = try XCTUnwrap(page.range(of: "async function checkWaiting() {"))
        let check = String(page[list.upperBound...].prefix(900))
        XCTAssertTrue(check.contains("get('/approvals')"))
        // Older Mac, Linux/Windows app or the Wi-Fi's address: not ok means hidden, and not asked again.
        XCTAssertTrue(check.contains("if (!res.ok) {"))
        XCTAssertTrue(check.contains("$('waiting').hidden = true;"))
        XCTAssertTrue(check.contains("clearInterval(waitingTimer);"))
        // The answer is 1.23's /act with accept or reject; no new path.
        XCTAssertTrue(page.contains("await send('/act', { session: item.session, action });"))
        XCTAssertTrue(page.contains("[['reject', 'No'], ['accept', 'Yes']]"))
        XCTAssertFalse(page.contains("/setup/gm"), "the deck never goes through the GM's routes")
        // A menu is answered at its desk.
        XCTAssertTrue(page.contains("selected = item.session; render();"))
    }

    /// The office tags a waiting desk (the viewer draws with the same file, copied in when it's deployed).
    func testTheOfficeTagsAWaitingDesk() throws {
        let web = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/office.js"), encoding: .utf8)
        XCTAssertTrue(web.contains("s.session.state === 'asking' && !DESKS[desk].off"))
        XCTAssertTrue(web.contains("'WAITING'"))
    }
}
