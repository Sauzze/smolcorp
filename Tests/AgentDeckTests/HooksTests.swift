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

    /// The first backup is the file from before Smolcorp, however often it
    /// changes the file after; the last is the file from before its latest change.
    func testTheFirstBackupKeepsYourSettingsFromBeforeSmolcorp() throws {
        let settings = try file("settings.json", #"{"model": "opus"}"#)
        try Hooks.write(["model": "opus", "hooks": [:]], to: settings, backup: true)
        try #"{"theme": "dark"}"#.write(to: settings, atomically: true, encoding: .utf8)
        try Hooks.write(["theme": "dark", "hooks": [:]], to: settings, backup: true)

        let (first, last) = Hooks.backups(of: settings)
        XCTAssertEqual(try String(contentsOf: first, encoding: .utf8), #"{"model": "opus"}"#)
        XCTAssertEqual(try String(contentsOf: last, encoding: .utf8), #"{"theme": "dark"}"#)
    }

    func testNothingIsWrittenWithoutItsBackup() throws {
        let settings = try file("settings.json", #"{"model": "opus"}"#)
        // A backup that can't be made: a locked folder sits where it goes. The
        // settings' own folder stays writable, so only the backup is in the way.
        let backup = Hooks.backups(of: settings).last
        try FileManager.default.createDirectory(at: backup, withIntermediateDirectories: true)
        try "x".write(to: backup.appendingPathComponent("inside"), atomically: false, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o555], ofItemAtPath: backup.path)
        defer { try? FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: backup.path) }

        XCTAssertThrowsError(try Hooks.write(["hooks": [:]], to: settings, backup: true))
        XCTAssertEqual(try String(contentsOf: settings, encoding: .utf8), #"{"model": "opus"}"#)
    }

    // MARK: Claude Code's voice (SIERRA 1.22.1 m9)

    /// Connecting turns voice on only when it isn't set at all, and a setting
    /// of yours, on or off, is kept as it is.
    func testConnectingTurnsVoiceOnOnlyWhenItIsntSet() {
        let on = Hooks.withSmolcorpVoice(["model": "opus"])
        XCTAssertEqual(on?["voice"] as? NSDictionary, Hooks.smolcorpVoice as NSDictionary)
        XCTAssertEqual(on?["model"] as? String, "opus")
        XCTAssertNil(Hooks.withSmolcorpVoice(["voice": ["enabled": false]]))
        XCTAssertNil(Hooks.withSmolcorpVoice(["voice": ["enabled": true, "mode": "hold"]]))
        XCTAssertTrue(Hooks.voiceNotice.contains("voice input is now on"))
    }

    /// Disconnecting turns voice off only when Smolcorp turned it on and it's
    /// still as written; anything else is yours, and kept.
    func testDisconnectingTurnsOffOnlyTheVoiceSmolcorpTurnedOn() {
        let ours: [String: Any] = ["model": "opus", "voice": Hooks.smolcorpVoice]
        let off = Hooks.withoutSmolcorpVoice(ours, marked: true)
        XCTAssertNil(off["voice"])
        XCTAssertEqual(off["model"] as? String, "opus")
        // Not marked: set by you before connecting, or the marker went with a reconnect
        XCTAssertNotNil(Hooks.withoutSmolcorpVoice(ours, marked: false)["voice"])
        // Changed since
        for yours in [["enabled": true, "mode": "hold"], ["enabled": false, "mode": "tap"], ["enabled": true, "mode": "tap", "x": 1]] as [[String: Any]] {
            XCTAssertNotNil(Hooks.withoutSmolcorpVoice(["voice": yours], marked: true)["voice"], "\(yours)")
        }
        XCTAssertNil(Hooks.withoutSmolcorpVoice(["model": "opus"], marked: true)["voice"])
    }

    // MARK: A disconnect that only partly worked (T107)

    /// The partial error names the agents left connected, and says what to do.
    func testAPartlyFailedDisconnectNamesWhatFailed() {
        let text = Hooks.HookError.partlyDisconnected(["Codex", "Gemini CLI"]).localizedDescription
        XCTAssertTrue(text.hasPrefix("Disconnected, except Codex and Gemini CLI"), text)
        XCTAssertTrue(text.contains("disconnect again"))
    }

    /// Disconnecting is remembered in config.json, and connecting again forgets it; nothing is written when it hasn't changed.
    func testTheDisconnectIsSavedForTheConfig() throws {
        let saved = folder.appendingPathComponent("config.json")
        let config = Config.load(from: saved)
        DeckRouter.keepDisconnected(true, in: config, to: saved)
        XCTAssertTrue(Config.load(from: saved).hooksDisconnected)
        DeckRouter.keepDisconnected(true, in: config, to: saved)
        DeckRouter.keepDisconnected(false, in: config, to: saved)
        XCTAssertFalse(Config.load(from: saved).hooksDisconnected)
    }
}
