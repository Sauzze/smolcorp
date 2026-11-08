import XCTest
@testable import AgentDeck

/// The first-run guide (Resources/web/guide.js): kept beside hasCompletedSetup, shown on a first run only,
/// and again from Show Me Around Again in the Help menu and the menu bar's.
final class GuideTests: XCTestCase {
    private var file: URL!

    override func setUpWithError() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        file = folder.appendingPathComponent("config.json")
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: file.deletingLastPathComponent())
    }

    private func source(_ path: String) throws -> String {
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        return try String(contentsOf: root.appendingPathComponent(path), encoding: .utf8)
    }

    /// A new Mac sees the guide; a file from before it, of someone who had set up (setup opened, a hires
    /// folder, or a GM hired), counts as done; what's saved is kept.
    func testTheGuideIsForAFirstRunOnly() throws {
        XCTAssertFalse(Config.load(from: file).guideDone, "no config yet")
        for text in [#"{"hasCompletedSetup": true}"#, #"{"hireFolder": "/work"}"#, #"{"managerEnabled": true}"#] {
            try text.write(to: file, atomically: true, encoding: .utf8)
            XCTAssertTrue(Config.load(from: file).guideDone, "\(text), from before the guide")
        }
        try #"{"hasCompletedSetup": false, "hireFolder": ""}"#.write(to: file, atomically: true, encoding: .utf8)
        XCTAssertFalse(Config.load(from: file).guideDone, "someone who never set up")
        // Setup opened (hasCompletedSetup) during the guide doesn't end it: what's saved is.
        try #"{"hasCompletedSetup": true, "hireFolder": "/work", "guideDone": false}"#.write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        XCTAssertFalse(config.guideDone)
        config.guideDone = true
        config.save(to: file)
        XCTAssertTrue(Config.load(from: file).guideDone, "finished, saved and loaded again")
        XCTAssertFalse(FileManager.default.fileExists(atPath: file.appendingPathExtension("broken").path))
    }

    /// Show Me Around Again turns the guide on again, saved, and setup opens on #guide, which the page starts it on.
    func testShowMeAroundAgainTurnsTheGuideOnAndOpensSetupOnIt() throws {
        try #"{"hasCompletedSetup": true, "guideDone": true}"#.write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        SettingsWindow.showGuideAgain(config, file: file)
        XCTAssertFalse(config.guideDone)
        XCTAssertFalse(Config.load(from: file).guideDone, "saved")
        XCTAssertEqual(SettingsWindow.address(port: 7710, then: SettingsWindow.guide).fragment, "guide")

        for (path, show) in [("Sources/AgentDeck/App.swift", "settings.show(then: SettingsWindow.guide)"),
                             ("Sources/AgentDeck/StatusMenu.swift", "showSettings(then: SettingsWindow.guide)")] {
            let menu = try source(path)
            XCTAssertTrue(menu.contains(#""Show Me Around Again", #selector(showMeAround))"#), "\(path): the menu item")
            let action = try XCTUnwrap(menu.range(of: "@objc private func showMeAround() {"), path)
            let body = menu[action.upperBound...].prefix(160)
            XCTAssertTrue(body.contains("SettingsWindow.showGuideAgain()") && body.contains(show), "\(path): turns it on, then opens setup on it")
        }
        XCTAssertTrue(try source("Sources/AgentDeck/App.swift").contains(#"let help = submenu(of: bar, "Help")"#), "in the Help menu")
    }

    /// The route keeps it, setup's state says it, and guide.js is served as office.js is: no key, nothing of yours.
    func testTheRouteTheStateAndTheScriptAreThere() throws {
        let router = try source("Sources/AgentDeck/DeckRouter.swift")
        XCTAssertTrue(router.contains(#"case ("POST", "/setup/guide"):"#))
        XCTAssertTrue(router.contains(#"config.guideDone = request.query["done"] == "1""#))
        XCTAssertTrue(router.contains(#""guide": ["done": config.guideDone]"#))
        XCTAssertTrue(router.contains(#"request.path == "/guide.js""#))
        let script = try source("Resources/web/guide.js")
        XCTAssertTrue(script.contains("SmolcorpGuide"))
        let setup = try source("Resources/web/setup.html")
        XCTAssertTrue(setup.contains(#"<script src="/guide.js"></script>"#))
        for id in ["#first-folder-use", "#first-folder", "#first-gm-checks", "#start-gm", "#hs-hire", "first-gm-next"] {
            XCTAssertTrue(setup.contains(id), "a step on \(id)")
        }
    }
}
