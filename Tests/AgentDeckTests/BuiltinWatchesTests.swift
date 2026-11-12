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

    func testSettingsDefaultToOnWithTheManager() throws {
        XCTAssertTrue(Config().builtinWatches)
        root = FileManager.default.temporaryDirectory.appendingPathComponent("bw-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        let file = root.appendingPathComponent("config.json")
        try Data(#"{"builtinWatches": false}"#.utf8).write(to: file)
        XCTAssertFalse(Config.load(from: file).builtinWatches)
    }
}
