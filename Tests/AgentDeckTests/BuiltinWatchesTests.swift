import XCTest
@testable import AgentDeck

/// The app's own watches for the hires at work, and the hourly digest: temp folders and a fake clock.
final class BuiltinWatchesTests: XCTestCase {
    private var root: URL!
    private var folder: String!

    override func tearDownWithError() throws { try? FileManager.default.removeItem(at: root) }

    private func write(_ text: String, to name: String = "STATUS.md", append: Bool = false) throws {
        let url = URL(fileURLWithPath: folder).appendingPathComponent(name)
        if append, let handle = try? FileHandle(forWritingTo: url) {
            try handle.seekToEnd()
            try handle.write(contentsOf: Data(text.utf8))
            try handle.close()
        } else {
            try text.write(to: url, atomically: false, encoding: .utf8)
        }
    }

    /// T406's firing is what T411's list reads: the same line, whose hire may have a colon in its name. The list is the
    /// iPhone app's, in the personal build only (IPHONE_APP, M6).
    #if IPHONE_APP
    func testARealNeedsReviewFiringShowsInTheReviewsList() throws {
        hires[0].nickname = "Rev: two"
        try write("start\n")
        let watchers = make()
        watchers.sync()
        try write("- NEEDS-REVIEW: T406 ready: see PR\n", append: true)
        watchers.tick(inline: true)
        let item = try XCTUnwrap(logged.first.flatMap(Reviews.item))
        XCTAssertEqual(item.hire, "Rev: two")
        XCTAssertEqual(item.file, "STATUS.md")
        XCTAssertEqual(item.text, "T406 ready: see PR")
        XCTAssertEqual(Reviews.recent(logged).count, 1)
    }
    #endif

    func testSettingsDefaultToOnWithTheManager() throws {
        XCTAssertTrue(Config().builtinWatches)
        root = FileManager.default.temporaryDirectory.appendingPathComponent("bw-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        let file = root.appendingPathComponent("config.json")
        try Data(#"{"builtinWatches": false}"#.utf8).write(to: file)
        XCTAssertFalse(Config.load(from: file).builtinWatches)
    }
}
