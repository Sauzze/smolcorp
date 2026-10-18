import XCTest
@testable import AgentDeck

/// Agents' settings files: the deck's entries go in, and nothing of yours is lost.
/// Every test works in a folder of its own, never in your home folder.
final class HooksTests: XCTestCase {
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    private func file(_ name: String, _ text: String) throws -> URL {
        let url = folder.appendingPathComponent(name)
        try text.write(to: url, atomically: true, encoding: .utf8)
        return url
    }

    func testAMissingFileIsEmpty() throws {
        XCTAssertTrue(try Hooks.read(folder.appendingPathComponent("settings.json")).isEmpty)
    }

    func testAFileThatCantBeReadIsAnError() throws {
        let settings = try file("settings.json", #"{"model": "opus"}"#)
        try FileManager.default.setAttributes([.posixPermissions: 0o000], ofItemAtPath: settings.path)
        defer { try? FileManager.default.setAttributes([.posixPermissions: 0o644], ofItemAtPath: settings.path) }
        XCTAssertThrowsError(try Hooks.read(settings)) { error in
            guard case Hooks.HookError.unreadableSettings = error else { return XCTFail("got \(error)") }
        }
    }

    func testALinkToNothingIsAnError() throws {
        let settings = folder.appendingPathComponent("settings.json")
        try FileManager.default.createSymbolicLink(at: settings, withDestinationURL: folder.appendingPathComponent("gone.json"))
        XCTAssertThrowsError(try Hooks.read(settings))
    }

    /// Broken JSON is named like any file that can't be read, so connecting
    /// carries on with the other agents instead of stopping at this one.
    func testBrokenJSONIsNamedAsUnreadable() throws {
        for text in [#"{"theme": "dark","#, "", "[1, 2]"] {
            let settings = try file("settings.json", text)
            XCTAssertThrowsError(try Hooks.read(settings), "for \(text.debugDescription)") { error in
                guard case Hooks.HookError.unreadableSettings(let files) = error else { return XCTFail("got \(error)") }
                XCTAssertEqual(files, [settings])
            }
        }
    }

    func testWritingKeepsADotfilesLink() throws {
        let kept = try file("claude-settings.json", #"{"model": "opus"}"#)
        let settings = folder.appendingPathComponent("settings.json")
        try FileManager.default.createSymbolicLink(at: settings, withDestinationURL: kept)

        var read = try Hooks.read(settings)
        read["hooks"] = ["Stop": [["hooks": [["type": "command", "command": "report.sh"]]]]]
        try Hooks.write(read, to: settings, backup: true)

        XCTAssertEqual(try FileManager.default.destinationOfSymbolicLink(atPath: settings.path), kept.path)
        let saved = try Hooks.read(kept)
        XCTAssertEqual(saved["model"] as? String, "opus")
        XCTAssertNotNil(saved["hooks"])
        XCTAssertEqual(try String(contentsOf: kept.appendingPathExtension("agentdeck.bak"), encoding: .utf8), #"{"model": "opus"}"#)
    }

    func testNothingIsWrittenWithoutItsBackup() throws {
        let settings = try file("settings.json", #"{"model": "opus"}"#)
        // A backup that can't be made: a locked folder sits where it goes. The
        // settings' own folder stays writable, so only the backup is in the way.
        let backup = settings.appendingPathExtension("agentdeck.bak")
        try FileManager.default.createDirectory(at: backup, withIntermediateDirectories: true)
        try "x".write(to: backup.appendingPathComponent("inside"), atomically: false, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o555], ofItemAtPath: backup.path)
        defer { try? FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: backup.path) }

        XCTAssertThrowsError(try Hooks.write(["hooks": [:]], to: settings, backup: true))
        XCTAssertEqual(try String(contentsOf: settings, encoding: .utf8), #"{"model": "opus"}"#)
    }
}
