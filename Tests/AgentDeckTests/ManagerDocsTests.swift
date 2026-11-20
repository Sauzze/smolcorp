import XCTest
@testable import AgentDeck

/// docs/manager/API.md must mention every /setup/gm/ route of the table, so the doc can't drift.
final class ManagerDocsTests: XCTestCase {
    private var docs: URL {
        URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("docs/manager")
    }

    func testAPIDocMentionsEveryGMRoute() throws {
        let text = try String(contentsOf: docs.appendingPathComponent("API.md"), encoding: .utf8)
        // Setup's route table as the Go app keeps it (desktop/routes.go), in its shared copy.
        let table = docs.deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("Tests/fixtures/frontdesk/routes.json")
        let routes = try XCTUnwrap((try JSONSerialization.jsonObject(with: Data(contentsOf: table)) as? [String: Any])?["routes"] as? [[String: Any]])
        let rows = routes.compactMap { row -> (method: String, path: String)? in
            guard let method = row["method"] as? String, let path = row["path"] as? String, path.hasPrefix("/setup/gm/") else { return nil }
            return (method, path)
        }
        XCTAssertGreaterThan(rows.count, 15)
        for row in rows {
            XCTAssertTrue(text.contains("`\(row.method) \(row.path)`"), "docs/manager/API.md doesn't mention \(row.method) \(row.path)")
        }
    }

    func testTheDocsAreAllThere() {
        for name in ["README.md", "HIRING-GUIDE.md", "API.md", "examples/fix.md", "examples/review.md", "examples/docs.md"] {
            XCTAssertTrue(FileManager.default.fileExists(atPath: docs.appendingPathComponent(name).path), name)
        }
    }

    func testGuideSaysAGroupSwitchedOffLeavesTheDenyRulesToo() throws {
        let guide = try String(contentsOf: docs.appendingPathComponent("HIRING-GUIDE.md"), encoding: .utf8)
        XCTAssertTrue(guide.contains("drops it from the guard **and** from these deny rules"))
        let setup = try String(contentsOf: docs.deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Resources/web/setup.html"), encoding: .utf8)
        XCTAssertTrue(setup.contains("dropped from Claude's own deny rules too"))
    }

    func testReadmeSaysHiringTheGMTurnsTheManagerOn() throws {
        let readme = try String(contentsOf: docs.appendingPathComponent("README.md"), encoding: .utf8)
        XCTAssertTrue(readme.contains("Hiring the GM turns the manager on"))
    }
}
