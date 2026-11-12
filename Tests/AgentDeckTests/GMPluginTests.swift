import XCTest
@testable import AgentDeck

/// The General Manager's plugin: its manifest, that everything it names is inside it,
/// and that the GM's start line loads it.
final class GMPluginTests: XCTestCase {
    private let plugin = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        .deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("Resources/gm/plugin")

    func testEveryFileOfThePluginStaysInsideIt() throws {
        let root = plugin.resolvingSymlinksInPath().path
        let files = try XCTUnwrap(FileManager.default.enumerator(at: plugin, includingPropertiesForKeys: nil))
        var count = 0
        for case let file as URL in files {
            XCTAssertTrue(file.resolvingSymlinksInPath().path.hasPrefix(root + "/"), file.path)
            count += 1
        }
        XCTAssertGreaterThan(count, 8)
        // The app's build copies the whole of Resources/gm.
        let build = try String(contentsOf: plugin.deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("scripts/build-app.sh"), encoding: .utf8)
        XCTAssertTrue(build.contains("cp -R Resources/gm \"$APP/Contents/Resources/\""))
    }

    func testTheReviewerBriefAsksForAVerdictAndExactFixes() throws {
        let brief = try String(contentsOf: plugin.deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("briefs/reviewer.md"), encoding: .utf8)
        XCTAssertTrue(brief.contains("read_only: true"))
        XCTAssertTrue(brief.contains("`PASS`"))
        XCTAssertTrue(brief.contains("`FAIL`"))
        XCTAssertTrue(brief.contains("**exact fix**"))
    }
}
