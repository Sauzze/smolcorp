import AppKit
import XCTest
@testable import AgentDeck

/// Show in Dock: on unless you turn it off, and kept as you left it.
final class DockTests: XCTestCase {
    private var file: URL!

    override func setUpWithError() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        file = folder.appendingPathComponent("config.json")
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: file.deletingLastPathComponent())
    }

    func testItsInTheDockUntilTurnedOff() throws {
        XCTAssertTrue(Config().showInDock)
        // A file from before it existed: in the Dock, and nothing lost
        try #"{"scrollEnabled": true}"#.write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        XCTAssertTrue(config.showInDock)
        XCTAssertFalse(config.lostOnLoad)
    }

    func testTurnedOffItStaysOff() throws {
        let config = Config()
        config.showInDock = false
        try JSONEncoder().encode(config).write(to: file)
        XCTAssertFalse(Config.load(from: file).showInDock)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: file)) as? [String: Any])
        XCTAssertEqual(saved["showInDock"] as? Bool, false)
    }

    func testOneItCantReadIsOnAgain() throws {
        try #"{"showInDock": "no"}"#.write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        XCTAssertTrue(config.showInDock)
        XCTAssertTrue(config.lostOnLoad)
    }

    func testOnItsAFullAppAndOffMenuBarOnly() {
        XCTAssertEqual(Dock.policy(showing: true), .regular)
        XCTAssertEqual(Dock.policy(showing: false), .accessory)
    }
}
