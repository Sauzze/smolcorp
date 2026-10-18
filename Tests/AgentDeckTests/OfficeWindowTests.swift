import XCTest
@testable import AgentDeck

/// The office's own window keeps the office's shape as it's resized.
final class OfficeWindowTests: XCTestCase {
    func testSidewaysStaysSidewaysInItsProportions() {
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 800, height: 300), by: .width), NSSize(width: 800, height: 450))
    }

    func testDrawnTallerThanWideItTurnsUpright() {
        // Dragged down from 640 × 360 to 640 × 700: upright, its height kept
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 640, height: 700), by: .height), NSSize(width: 422, height: 700))
        // And back: dragged wider than tall, sideways again
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 800, height: 700), by: .width), NSSize(width: 800, height: 450))
    }

    func testNeverSmallerThanTheOfficeDrawnOnce() {
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 100, height: 50), by: .width), OfficeWindow.wide)
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 100, height: 200), by: .height), OfficeWindow.tall)
    }

    func testOnlyTheOfficesOwnPagesOpen() {
        let address = URL(string: "http://127.0.0.1:7710/office?window&t=key")!
        XCTAssertTrue(OfficeWindow.isOwnPage(URL(string: "http://127.0.0.1:7710/events?t=key"), of: address))
        XCTAssertFalse(OfficeWindow.isOwnPage(URL(string: "http://127.0.0.1:7711/office"), of: address), "another port")
        XCTAssertFalse(OfficeWindow.isOwnPage(URL(string: "http://localhost:7710/office"), of: address))
        XCTAssertFalse(OfficeWindow.isOwnPage(URL(string: "https://example.com/?t=key"), of: address))
        XCTAssertFalse(OfficeWindow.isOwnPage(URL(string: "http://127.0.0.1.example.com:7710/"), of: address))
        XCTAssertFalse(OfficeWindow.isOwnPage(nil, of: address))
    }

    func testAResizeGoesByWhatsDragged() {
        let start = NSSize(width: 640, height: 360)
        // A bottom edge dragged: the height, whatever the width does meanwhile
        XCTAssertEqual(OfficeWindow.drive(from: start, to: NSSize(width: 640, height: 900), width: false, height: true), .height)
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 640, height: 900), by: .height), NSSize(width: 542, height: 900), "dragged down past its width, it's upright")
        XCTAssertEqual(OfficeWindow.drive(from: start, to: NSSize(width: 1000, height: 360), width: true, height: false), .width)
        // A corner: the one changed the more
        XCTAssertEqual(OfficeWindow.drive(from: start, to: NSSize(width: 700, height: 600), width: true, height: true), .height)
    }
}
