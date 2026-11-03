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

    func testTheVersionAnswerIsTheFixture() throws {
        let answer = ManagerSwitch.versionJSON(version: "1.24.0", enabled: false)
        XCTAssertTrue((answer as NSDictionary).isEqual(to: try fixture("version.json")))
        XCTAssertEqual(ManagerSwitch.versionJSON(version: "1.24.0", enabled: true)["managerEnabled"] as? Bool, true)
    }

    func testEveryGMRouteSaysTheManagerIsOffExceptTheVersionAndTheSwitch() throws {
        for row in try routes() {
            let method = row["method"] as! String, path = row["path"] as! String
            guard path.hasPrefix("/setup/gm/") else { continue }
            // A prefix row stands for any route under it.
            let sample = path.hasSuffix("/*") ? String(path.dropLast()) + "anything" : path
            let asked = method == "*" ? "POST" : method
            let off = ManagerSwitch.refusal(asked, sample, enabled: false)
            if (path == "/setup/gm/version" || path == "/setup/gm/manager" || path == "/setup/gm/by-hand") {
                XCTAssertNil(off, "\(path) stays open while the manager is off")
            } else {
                XCTAssertEqual(off?.status, 409, path)
                XCTAssertEqual(off?.body, Data("The manager is off. Turn it on in Smolcorp's setup.".utf8), path)
            }
            XCTAssertNil(ManagerSwitch.refusal(asked, sample, enabled: true), "\(path) with the manager on")
        }
    }

    func testOnlyTheGMRoutesAreAffected() {
        for path in ["/setup/state", "/setup/hire/gm", "/setup/hire", "/setup/pair", "/setup/gm", "/hook", "/bots/x"] {
            XCTAssertNil(ManagerSwitch.refusal("POST", path, enabled: false), path)
        }
        // Only the version is open for reading; the switch is open for turning on.
        XCTAssertNotNil(ManagerSwitch.refusal("POST", "/setup/gm/version", enabled: false))
        XCTAssertNotNil(ManagerSwitch.refusal("GET", "/setup/gm/manager", enabled: false))
    }

    func testTheSwitchAndTheVersionAreInTheRouteTable() {
        XCTAssertEqual(SetupRoutes.row("POST", "/setup/gm/manager")?.callers, [.gm, .page])
        XCTAssertEqual(SetupRoutes.row("GET", "/setup/gm/version")?.kind, "read")
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

    func testTheFeaturesNameWhatTheGMsToolsCheck() {
        XCTAssertTrue(Set(["events", "team", "registry", "token", "screens", "tile"]).isSubset(of: ManagerSwitch.features))
    }
}
