import XCTest
@testable import AgentDeck

/// The front desk: the registry kept in a file, the team joined from it and
/// the sessions the deck follows, and what may be edited by hand. The wire
/// formats are the shared fixtures in Tests/fixtures/frontdesk/, which the
/// Linux and Windows app tests against too.
final class FrontDeskTests: XCTestCase {
    private var folder: URL!
    private let fixtures = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        .deletingLastPathComponent().appendingPathComponent("fixtures/frontdesk")

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    private func fixture(_ name: String) throws -> [String: Any] {
        let data = try Data(contentsOf: fixtures.appendingPathComponent(name))
        return try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
    }

    func testTheRouteTableNamesTheRoutesThatExist() throws {
        let routes = try XCTUnwrap(try fixture("routes.json")["routes"] as? [[String: Any]])
        // routes.json is the one route table (SetupRoutes); the front desk's two routes are in it.
        func callers(_ method: String, _ path: String) -> [String]? {
            routes.first { $0["method"] as? String == method && $0["path"] as? String == path }?["callers"] as? [String]
        }
        XCTAssertEqual(callers("GET", "/setup/gm/team"), ["gm", "page", "tool"])
        XCTAssertEqual(callers("POST", "/setup/gm/registry"), ["gm", "page"])
    }
}
