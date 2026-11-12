import XCTest
@testable import AgentDeck

/// Setup's brief and hire routes: what each answers, and that only setup's
/// own requests get in, tried on a stand-in server on a spare port (never
/// the app's own).
final class BriefRouteTests: XCTestCase {
    /// Stands in for your home folder.
    private var home: URL!

    override func setUpWithError() throws {
        home = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
            .resolvingSymlinksInPath()
        for folder in ["Projects/app/.team/hire", "Library/Keys", ".ssh"] {
            try FileManager.default.createDirectory(at: home.appendingPathComponent(folder), withIntermediateDirectories: true)
        }
        try "".write(to: home.appendingPathComponent("Projects/notes.md"), atomically: true, encoding: .utf8)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: home)
    }

    func testAHireIsNotedFirstAmongTheRecentFolders() {
        let config = Config()
        for folder in ["/a", "/b", "/a"] + (1...10).map { "/f\($0)" } + ["/b"] { config.noteHire(in: folder) }
        XCTAssertEqual(config.recentHireFolders, ["/b", "/f10", "/f9", "/f8", "/f7", "/f6", "/f5", "/f4"])
    }
}
