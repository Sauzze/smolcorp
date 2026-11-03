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
        let rows = SetupRoutes.table.filter { $0.path.hasPrefix("/setup/gm/") }
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

    func testThePhoneAppIsNoLongerBuild5() throws {
        let ios = docs.deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("ios")
        let yml = try String(contentsOf: ios.appendingPathComponent("project.yml"), encoding: .utf8)
        let plist = try String(contentsOf: ios.appendingPathComponent("Smolcorp/Info.plist"), encoding: .utf8)
        XCTAssertTrue(yml.contains("CFBundleShortVersionString: \"0.4\"") && yml.contains("CFBundleVersion: \"6\""))
        XCTAssertTrue(plist.contains("<string>0.4</string>") && plist.contains("<string>6</string>"))
    }
}
