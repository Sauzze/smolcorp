import XCTest
@testable import AgentDeck

/// What the Swift engine's tiling left: the team's screen in config.json, which the Go core keeps, and the
/// windows' code that never presses keys or talks to System Events.
final class TilerTests: XCTestCase {

    func testTheChosenScreenIsSavedInTheConfigAndOptional() throws {
        let config = Config()
        XCTAssertNil(config.teamScreen)
        config.teamScreen = "2"
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("tiler-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: file) }
        config.save(to: file)
        XCTAssertEqual(Config.load(from: file).teamScreen, "2")
    }

    /// Only the tiling files: the opt-in keystrokes (T505) have a module of their own and may use Accessibility there.
    func testTheTilingFilesNeverTalkToSystemEventsOrPressKeys() throws {
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        for name in ["OfficeWindow.swift", "TerminalDrivers.swift"] {
            let source = try String(contentsOf: root.appendingPathComponent("Sources/AgentDeck/" + name), encoding: .utf8)
            XCTAssertFalse(source.contains("System " + "Events"), name)
            XCTAssertFalse(source.contains("AXUI" + "Element"), name)
            XCTAssertFalse(source.contains("AXIsProcess" + "Trusted"), name)
            XCTAssertFalse(source.contains("key " + "code"), name)
            XCTAssertFalse(source.contains("keystroke \""), name + " has no AppleScript keystroke")
        }
    }
}

final class TeamGridTests: XCTestCase {
}
