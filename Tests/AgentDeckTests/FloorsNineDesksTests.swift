import XCTest
@testable import AgentDeck

/// Every floor upstairs as the ground floor is: nine desks in three rows and a
/// chill area, each floor arranged its own way, and those at desks 10 to 12
/// from when floors had twelve moved to a desk there still is. The formats are
/// the shared fixtures in Tests/fixtures/floors/, which the Windows and Linux
/// app's tests read too.
final class FloorsNineDesksTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private func fixture(_ name: String) throws -> Any {
        try JSONSerialization.jsonObject(with: Data(contentsOf: Self.repo.appendingPathComponent("Tests/fixtures/floors/\(name)")))
    }

    /// `value` as JSON's own types, to compare with a fixture's.
    private func json<T: Encodable>(_ value: T) throws -> Any {
        try JSONSerialization.jsonObject(with: JSONEncoder().encode(value), options: [.fragmentsAllowed])
    }

    func testEachFloorKeepsItsOwnArrangementAsTheGoAppDoes() throws {
        let given = try XCTUnwrap(try fixture("floor-plans.json") as? [String: Any])
        let room = try JSONDecoder().decode(Room.self, from: JSONSerialization.data(withJSONObject: given["given"] as Any))
        XCTAssertEqual(try json(room.floorPlans) as? NSDictionary, given["kept"] as? NSDictionary)
        XCTAssertEqual(room.plan.wide["desk1"]?.x, 5, "the ground floor's own stays its own")
        // As the setup page sends it, in the whole layout
        let layout = try XCTUnwrap(Layout.parse(JSONSerialization.data(withJSONObject: ["room": given["given"] as Any])))
        XCTAssertEqual(layout.room.floorPlans, room.floorPlans)
        // And as it's kept, the same again
        let again = try JSONDecoder().decode(Room.self, from: JSONEncoder().encode(room))
        XCTAssertEqual(again.floorPlans, room.floorPlans)
    }

    /// A config from before floors were arranged: as it was, every floor upstairs as it comes.
    func testAnOldConfigLoadsAsBeforeWithEveryFloorAsItComes() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let file = folder.appendingPathComponent("config.json")
        try ##"""
        {"layout": {"office": true, "room": {"corner": "jukebox", "preset": "yours",
          "plan": {"wide": {"desk2": {"x": 4, "y": -2}}, "added": {"wide": [{"kind": "fishtank", "x": 3, "y": 1, "size": 1.5}]}}}},
         "floors": [{"name": "1F", "preset": ""}, {"name": "Lab", "preset": "server"}]}
        """##.write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        XCTAssertEqual(config.layout.room.floorPlans, [:])
        XCTAssertEqual(config.layout.room.corner, "jukebox")
        XCTAssertEqual(config.layout.room.plan.wide["desk2"]?.x, 4)
        XCTAssertEqual(config.layout.room.plan.added.wide, [Plan.AddedPiece(kind: "fishtank", x: 3, y: 1, size: 1.5)])
        XCTAssertEqual(config.floors[1].name, "Lab")
        // Written with no floor arranged, and read back the same
        let room = try XCTUnwrap(try json(config.layout.room) as? [String: Any])
        XCTAssertEqual(room["floorPlans"] as? NSDictionary, [:] as NSDictionary)
        XCTAssertEqual(try JSONDecoder().decode(Room.self, from: JSONEncoder().encode(config.layout.room)), config.layout.room)
    }
}
