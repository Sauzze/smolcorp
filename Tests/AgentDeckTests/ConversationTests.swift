import XCTest
@testable import AgentDeck

/// iPhone app (personal build): the conversation the phone's blackboard
/// shows, read from transcripts of the tests' own: its redaction, line by
/// line, its pages, and who may read it. Offline.
final class ConversationTests: XCTestCase {
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    private func json(_ object: [String: Any]) -> String {
        String(data: try! JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]), encoding: .utf8)!
    }

    private func user(_ text: String) -> String {
        json(["type": "user", "message": ["role": "user", "content": text], "timestamp": "2026-10-22T10:00:00.000Z"])
    }

    private func throughTailscale(_ path: String) -> Request {
        Request.addressed(method: "GET", path: path, query: [:],
                          headers: ["host": "my-mac.tail1234.ts.net", "x-forwarded-for": "100.101.102.103", "x-forwarded-proto": "https"],
                          body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")
    }

    private func overWiFi(_ path: String) -> Request {
        Request.addressed(method: "GET", path: path, query: [:], headers: ["host": "192.168.1.20:7710"], body: Data(),
                          fromLoopback: false, remoteAddress: "192.168.1.9")
    }

    #if IPHONE_APP
    func testTheDeviceTokenComesOnlyThroughTailscaleFromAPairedPhone() {
        XCTAssertTrue(DeckRouter.takesPushToken(throughTailscale("/notifications"), phone: phone))
        XCTAssertFalse(DeckRouter.takesPushToken(overWiFi("/notifications"), phone: phone))
        XCTAssertFalse(DeckRouter.takesPushToken(throughTailscale("/notifications"), phone: .officeWindow))
    }
    #endif
}
