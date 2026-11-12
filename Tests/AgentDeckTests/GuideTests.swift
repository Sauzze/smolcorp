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
    /// folder, or a GM hired, by hand too), counts as done; what's saved is kept.
    func testTheGuideIsForAFirstRunOnly() throws {
        XCTAssertFalse(Config.load(from: file).guideDone, "no config yet")
        for text in [#"{"hasCompletedSetup": true}"#, #"{"hireFolder": "/work"}"#, #"{"managerEnabled": true}"#, #"{"gmByHand": true}"#] {
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

    /// Not done, beside a setup that's done: saved and loaded again, it's still not done. Were guideDone
    /// not written, the file would load as from before the guide, and count as done.
    func testAGuideNotDoneIsSavedBesideASetupThatIs() throws {
        let config = Config()
        config.hasCompletedSetup = true
        config.guideDone = false
        config.save(to: file)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: file)) as? [String: Any])
        XCTAssertEqual(saved["guideDone"] as? Bool, false, "written")
        let loaded = Config.load(from: file)
        XCTAssertTrue(loaded.hasCompletedSetup)
        XCTAssertFalse(loaded.guideDone, "loaded again: still to be shown")
    }

    // MARK: POST /setup/guide, on a stand-in server

    /// The bubble keeps off the page's title and Start here's header, and has no count of its own beside
    /// Start here's; off Easy setup it's a pill, and Skip says Show me around again brings it back.
    func testTheGuidesBubbleKeepsOffTheTitleAndCountsNothing() throws {
        let setup = try source("Resources/web/setup.html")
        let from = try XCTUnwrap(setup.range(of: "SmolcorpGuide.create({"))
        let to = try XCTUnwrap(setup.range(of: "onFinish:", range: from.upperBound..<setup.endIndex))
        let guide = setup[from.upperBound..<to.lowerBound]
        XCTAssertTrue(guide.contains("keep: '.sidebar > .brand, .sidebar > .search, .topbar, .pane.current > .pane-head, #easy-attention, #start-here > .card-head'"))
        XCTAssertTrue(guide.contains("awayText: 'Next step in Easy setup'"))
        XCTAssertTrue(guide.contains("skipSays: 'Skip: Show me around again brings it back'"))
        XCTAssertFalse(guide.contains("total:"), "no count of its own")
        let script = try source("Resources/web/guide.js")
        XCTAssertFalse(script.contains("Step ${"), "the bubble says no step number")
        XCTAssertTrue(script.contains(".smol-guide.pill .smol-guide-bubble"))
    }

    /// Fresh eyes, round 2: the bubble keeps off any text near what's lit and goes out of the way once its
    /// control is pressed; the pill's × only hides it, and on a phone Show me and × are 44 px each; the
    /// welcome's last step names the General Manager while none is there. Where it all lands is checked in
    /// node (Tests/fixtures/guide/engine.test.js, run by the Go tests).
    func testTheGuideKeepsOffTextAndTheWelcomeNamesWhatsMissing() throws {
        let script = try source("Resources/web/guide.js")
        for want in ["function lines(litEl, view) {", "{ text, aside: pressed === shown, chip }", "pressed = shown;\n        position();",
                     "pillHiddenIn = where();\n        return hide();", "min-width: 44px; min-height: 44px;", ".smol-guide.pill .smol-guide-go { order: 1; min-height: 44px;"] {
            XCTAssertTrue(script.contains(want), "guide.js has no \(want)")
        }
        let setup = try source("Resources/web/setup.html")
        for want in ["#first-gm-why, #start-said'", "hideSays: \"Hide for now: it's back in the next section you open\"",
                     "<button class=\"btn primary\" id=\"welcome-gm\">Hire your General Manager</button>",
                     "'Next: hire your General Manager' : 'All set'", "go('easy', $('start-gm'));"] {
            XCTAssertTrue(setup.contains(want), "setup.html has no \(want)")
        }
        XCTAssertFalse(setup.contains("on a paired phone. Want desks"), "the last step no longer says ready whatever is missing")
    }

    /// Fresh eyes, round 3: the bubble never sits on the logo or Search; a step whose control is off the
    /// window's edge shows a "Continue below" chip, not a parked bubble; on a phone the bubble is against its
    /// arrow, and the pill shrinks to a chip once you scroll. Where it lands is checked in node (run by Go).
    func testTheGuideStaysByItsControlOrSaysContinueBelow() throws {
        let script = try source("Resources/web/guide.js")
        for want in ["textContent: '↓ Continue below'", "more.textContent = at.point === 'up' ? '↑ Continue above' : '↓ Continue below';",
                     "if (cy < 0 || cy > view.height) return chip(cy < 0);", "if (how.aside || view.width > PHONE) return result('edge'",
                     ".smol-guide.pill.small .smol-guide-bubble", "go.textContent = pillSmall ? 'Next step →' : 'Show me';"] {
            XCTAssertTrue(script.contains(want), "guide.js has no \(want)")
        }
        let setup = try source("Resources/web/setup.html")
        XCTAssertTrue(setup.contains("keep: '.sidebar > .brand, .sidebar > .search, .topbar, "), "the bubble keeps off the logo and Search")
        for want in ["<div class=\"brand\">", "<div class=\"topbar\">", "<nav class=\"sidebar\""] {
            XCTAssertTrue(setup.contains(want), "setup.html has no \(want)")
        }
    }
}
