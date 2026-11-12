import XCTest
@testable import AgentDeck

/// The office's event log: what hooks leave in events.jsonl, and what
/// /setup/gm/events answers with. The fixtures under Tests/fixtures/frontdesk
/// are the wire formats the Linux/Windows port tests against too.
final class EventLogTests: XCTestCase {
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("eventlog-\(UUID().uuidString)", isDirectory: true)
    }

    override func tearDown() {
        try? FileManager.default.removeItem(at: folder)
    }

    private static let fixtures = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
        .appendingPathComponent("fixtures/frontdesk")

    func testClaudeCodeIsAskedForPermissionDenied() {
        XCTAssertTrue(Hooks.claudeEvents.contains("PermissionDenied"))
    }
}
