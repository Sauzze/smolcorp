import XCTest
@testable import AgentDeck

/// Mail to and from a cloud bot through a folder (T404), in temp folders: nothing is typed, no network.
final class BotMailboxTests: XCTestCase {
    private var root: URL!
    private var box: URL!
    private var support: URL!
    private var worktree: URL!
    private var bots: [CloudBot] = []
    private let fm = FileManager.default

    override func setUpWithError() throws {
        root = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("botmail-\(UUID().uuidString)").resolvingSymlinksInPath()
        box = root.appendingPathComponent("box")
        support = root.appendingPathComponent("support")
        worktree = root.appendingPathComponent("tree")
        for folder in [box!, support!, worktree!] { try fm.createDirectory(at: folder, withIntermediateDirectories: true) }
        bots = [CloudBot(id: "scout", name: "Scout", mailbox: box.path)]
    }

    override func tearDownWithError() throws { try? fm.removeItem(at: root) }

    func testTheMailboxIsKeptThroughTheConfigAndShownToThePage() throws {
        let bot = CloudBot(id: "scout", name: "Scout", mailbox: " \(box.path) ")
        XCTAssertEqual(bot.checked?.mailbox, box.path)
        let data = try JSONEncoder().encode(bot.checked)
        XCTAssertEqual(try JSONDecoder().decode(CloudBot.self, from: data).mailbox, box.path)
        // A config from before mailboxes has none.
        XCTAssertEqual(try JSONDecoder().decode(CloudBot.self, from: Data(#"{"id":"x","name":"X"}"#.utf8)).mailbox, "")
        XCTAssertEqual(bot.checked?.setupJSON["mailbox"] as? String, box.path)
    }
}
