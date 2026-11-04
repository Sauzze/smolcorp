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

    func testEveryFloorUpstairsHasNineDesksInThreeRowsAsTheOfficeDrawsThem() throws {
        XCTAssertEqual(Floor.desks, 9)
        XCTAssertEqual(Deck.deskRow, 3)
        // The office counts a floor's desks from the ground floor's three rows of three, as the Mac does.
        let office = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/office.js"), encoding: .utf8)
        XCTAssertTrue(office.contains("const DESK_COLUMNS = [22, 88, 154];") && office.contains("const DESK_ROWS = [58, 102, 146];"))
        XCTAssertTrue(office.contains("const UPSTAIRS_MOST = STANDARD_DESKS.length;"))
        // Ten put on 3F: nine sit there, the tenth goes where there's room.
        let crowd = (0...9).map { Upstairs(person: "p\($0)", id: "p\($0)", floor: 2) }
        let seated = Deck.seat(placed: crowd, rest: [])
        XCTAssertEqual(seated.floors[2].count, 9)
        XCTAssertEqual(seated.seats["p9"], Seat(floor: 1, desk: 0))
        XCTAssertTrue(seated.seats.values.allSatisfy { $0.desk < 9 }, "nobody at a desk past the ninth")
    }

    // MARK: Each floor arranged its own way

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

    // MARK: Desks 10 to 12

    func testPeopleAtDesksTenToTwelveMoveAsTheGoAppMovesThem() throws {
        let cases = try XCTUnwrap(try fixture("desks-10-to-12.json") as? [[String: Any]])
        XCTAssertEqual(cases.count, 3)
        for one in cases {
            let name = one["case"] as? String ?? ""
            let seats = try JSONDecoder().decode([String: Seat].self, from: JSONSerialization.data(withJSONObject: one["seats"] as Any))
            let floors = try JSONDecoder().decode([String: Int].self, from: JSONSerialization.data(withJSONObject: one["floors"] as Any))
            let moved = Deck.onFloorDesks(seats: seats, floors: floors)
            guard let after = one["after"] as? [String: Any] else {
                XCTAssertNil(moved, name)
                continue
            }
            let got = try XCTUnwrap(moved, name)
            XCTAssertEqual(try json(got.seats) as? NSDictionary, after["seats"] as? NSDictionary, name)
            XCTAssertEqual(try json(got.floors) as? NSDictionary, after["floors"] as? NSDictionary, name)
            XCTAssertNil(Deck.onFloorDesks(seats: got.seats, floors: got.floors), "moved once: \(name)")
        }
    }

    /// The deck saved when floors had twelve desks is moved as it's read, and written back: the next read moves nothing.
    func testASavedDeckIsMovedOnceAtLoad() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let file = folder.appendingPathComponent("sessions.json")
        let first = try XCTUnwrap((try fixture("desks-10-to-12.json") as? [[String: Any]])?.first)
        let saved: [String: Any] = [
            "savedAt": 1_762_218_000_000.0, "keys": ["s0": 0], "roles": ["m3": "reviewer"], "floors": first["floors"] as Any,
            "seats": first["seats"] as Any,
            "sessions": [["id": "m3", "agent": "claude", "folder": "/work/m3", "project": "/work/m3", "tty": "", "tab": "headless:m3",
                          "state": "idle", "effort": "", "model": "", "transcript": "", "startedAt": 1.0, "lastEventAt": 2.0]],
        ]
        try JSONSerialization.data(withJSONObject: saved).write(to: file)
        let after = try XCTUnwrap(first["after"] as? [String: Any])

        let loaded = try XCTUnwrap(Deck.savedDeck(at: file))
        XCTAssertEqual(try json(loaded.seats) as? NSDictionary, after["seats"] as? NSDictionary)
        XCTAssertEqual(try json(loaded.floors) as? NSDictionary, after["floors"] as? NSDictionary)
        XCTAssertEqual(loaded.roles?["m3"], "reviewer", "a role is kept by session, so it goes with them")
        XCTAssertEqual(loaded.sessions.map(\.id), ["m3"])
        // Written back as moved, the rest as it was
        let written = try JSONDecoder().decode(Deck.SavedDeck.self, from: Data(contentsOf: file))
        XCTAssertEqual(written.seats, loaded.seats)
        XCTAssertEqual(written.keys, ["s0": 0])
        XCTAssertEqual(written.savedAt, 1_762_218_000_000)

        // Read again: nothing moves, and the file stays as it is
        let bytes = try Data(contentsOf: file)
        let changedAt = try FileManager.default.attributesOfItem(atPath: file.path)[.modificationDate] as? Date
        let again = try XCTUnwrap(Deck.savedDeck(at: file))
        XCTAssertEqual(again.seats, loaded.seats)
        XCTAssertEqual(try Data(contentsOf: file), bytes)
        XCTAssertEqual(try FileManager.default.attributesOfItem(atPath: file.path)[.modificationDate] as? Date, changedAt)
        XCTAssertNil(Deck.savedDeck(at: folder.appendingPathComponent("none.json")))
    }
}
