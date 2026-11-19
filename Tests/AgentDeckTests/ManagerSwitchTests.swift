import XCTest
@testable import AgentDeck

/// The manager's switch: off by default, what the GM's routes say while it is off, and the
/// version answer, whose wire format is a shared fixture (Tests/fixtures/frontdesk/version.json).
final class ManagerSwitchTests: XCTestCase {
    private func fixture(_ name: String) throws -> [String: Any] {
        try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: FrontDeskFixtures.url(name))) as? [String: Any])
    }

    private func routes() throws -> [[String: Any]] {
        try XCTUnwrap(try fixture("routes.json")["routes"] as? [[String: Any]])
    }

    func testTheManagerIsOffUntilTurnedOnAndStaysSaved() throws {
        let file = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString + ".json")
        defer { try? FileManager.default.removeItem(at: file) }
        XCTAssertFalse(Config().managerEnabled)
        // A 1.23 file never said, so it is off.
        try #"{"hireFolder": "/work"}"#.write(to: file, atomically: true, encoding: .utf8)
        XCTAssertFalse(Config.load(from: file).managerEnabled)
        let config = Config.load(from: file)
        config.managerEnabled = true
        config.save(to: file)
        XCTAssertTrue(Config.load(from: file).managerEnabled)
    }
}
