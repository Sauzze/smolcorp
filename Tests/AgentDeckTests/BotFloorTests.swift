import XCTest
@testable import AgentDeck

/// Bots on any floor (T762): each where you put it, or where a floor rule
/// for its folder says, or where new bots go; a full floor sends it where
/// there's room; and an old config's bots stay where they sat. The Go app
/// loads the same configs the same way (Tests/fixtures/bots/configs.json).
final class BotFloorTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private struct Case: Decodable {
        struct After: Decodable {
            let botFloor: Int
            let bots: [[String]]
            let pinned: [String]?
            let floorNames: [String]?
            let hireFolder: String?
        }
        let about: String
        /// The desks saved across a restart (sessions.json's `seats`), by desk id: where each bot sat.
        let seats: [String: Seat]?
        let after: After
    }

    /// Each config loads, and as the app starts, its bots still "upstairs" go on the floors they sat on, by the desks
    /// saved for them, with the floor new bots go to; saved and started again, nothing changes.
    func testOldConfigsLoadWithTheirBotsWhereTheySitAndLoadAgainUnchanged() throws {
        let data = try Data(contentsOf: Self.repo.appendingPathComponent("Tests/fixtures/bots/configs.json"))
        let cases = try JSONDecoder().decode([Case].self, from: data)
        let befores = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [[String: Any]]).map { $0["before"] ?? [:] }
        XCTAssertEqual(cases.count, 6)
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        for (n, (one, before)) in zip(cases, befores).enumerated() {
            let file = folder.appendingPathComponent("config-\(n).json"), again = folder.appendingPathComponent("again-\(n).json")
            try JSONSerialization.data(withJSONObject: before).write(to: file)
            let config = Config.load(from: file)
            _ = config.settleBots(seats: one.seats ?? [:])
            config.save(to: again)
            let started = Config.load(from: again)
            XCTAssertFalse(started.settleBots(seats: one.seats ?? [:]), "\(one.about): settled once")
            for (when, loaded) in [("as the app starts", config), ("saved and started again", started)] {
                XCTAssertEqual(loaded.cloudBots.map { [$0.id, $0.floor] }, one.after.bots, "\(one.about) (\(when))")
                XCTAssertEqual(loaded.botFloor, one.after.botFloor, "\(one.about) (\(when))")
                if let pinned = one.after.pinned { XCTAssertEqual(loaded.cloudBots.filter(\.pinned).map(\.id), pinned, one.about) }
                if let names = one.after.floorNames { XCTAssertEqual(loaded.floors.map(\.name), names, one.about) }
                if let hireFolder = one.after.hireFolder { XCTAssertEqual(loaded.hireFolder, hireFolder, one.about) }
            }
        }
    }

    private func bots(_ count: Int, from first: Int = 0) -> [CloudBot] {
        (first..<first + count).map { CloudBot(id: "b\($0)", name: "B\($0)") }
    }

    func testSetupsBotsCardOffersEveryFloorAndWhereNewBotsGo() throws {
        let page = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/setup.html"), encoding: .utf8)
        XCTAssertTrue(page.contains("textContent: 'New bots go to'"))
        XCTAssertTrue(page.contains("send({ botFloor: Number(newFloor.value) })"))
        XCTAssertTrue(page.contains("[['', `Its folder’s floor, or where new bots go (${floorName(newBots)})`], ['ground', floorName(0)]"))
        // What makes room now, in the words of the bots' own buttons (Hide only for a bot the Mac found in the Grok Bot app):
        // "Hide or remove" where one of them says Hide. No desk is added any more.
        XCTAssertTrue(page.contains("const hides = bots.detects && (bots.bots || []).some((bot) => bot.fromApp);"))
        XCTAssertTrue(page.contains("every floor is full. ${hides ? 'Hide or remove' : 'Remove'} a bot, or free a desk, to make room."))
        XCTAssertTrue(page.contains("textContent = bots.detects && bot.fromApp ? 'Hide' : 'Remove'"))
        XCTAssertFalse(page.contains("add a desk"))
        XCTAssertFalse(page.contains("there can be 9 cloud bots"))
        XCTAssertFalse(page.contains("Upstairs, where there’s room"))
    }
}
