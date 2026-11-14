import XCTest
@testable import AgentDeck

/// Headless claude runs, started with SMOLCORP_HEADLESS=1: named by their
/// session ID, never taken for a terminal's tab, and given desks upstairs.
final class HeadlessTests: XCTestCase {

    func testAHeadlessAddressIsReadInItsOwnShapeOnly() {
        let address = TabAddress("headless:abc-123")
        XCTAssertEqual(address?.kind, "headless")
        XCTAssertEqual(address?.id, "abc-123")
        XCTAssertEqual(address?.isHeadless, true)
        XCTAssertEqual(address?.text, "headless:abc-123")
        for bad in ["headless:", "headless:abc 123", "headless:abc@sock", "headless:../x", "headless:abc_123",
                    "headless:abc\r\nX-Other: 1", "headless:é"] {
            XCTAssertNil(TabAddress(bad), bad)
        }
        XCTAssertFalse(TabAddress.isHeadless("tty:/dev/ttys003"))
    }

    func testAHeadlessRunHasNothingToDriveOrWatch() {
        let address = TabAddress("headless:abc-123")!
        XCTAssertNil(Terminals.driver(for: address))
        XCTAssertEqual(Terminals.abilities(of: address.text), .none)
        XCTAssertNil(Terminals.ttyInAddress(of: address.text))
        XCTAssertNil(Terminals.tty(of: address.text))
    }
}
