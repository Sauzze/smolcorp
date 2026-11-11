import XCTest
@testable import AgentDeck

/// The GM's mail core, with a stand-in delivery step: nothing is typed anywhere.
final class MailRouterTests: XCTestCase {
    private var root: URL!

    override func tearDownWithError() throws { try? FileManager.default.removeItem(at: root) }

    func testTheDefaultsAreTheOwnersHigherOnesAndCannotBeZero() {
        XCTAssertEqual(Config().mailMaxBytes, 16384)
        XCTAssertEqual(Config().mailPerHour, 200)
        root = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("mail-\(UUID().uuidString)")
        try? FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        let file = root.appendingPathComponent("config.json")
        try? Data(#"{"mailMaxBytes": 0, "mailPerHour": -3}"#.utf8).write(to: file)
        XCTAssertEqual(Config.load(from: file).mailMaxBytes, 1)
        let config = Config()
        config.mailMaxBytes = 4096
        config.save(to: file)
        XCTAssertEqual(Config.load(from: file).mailMaxBytes, 4096)
    }
}
