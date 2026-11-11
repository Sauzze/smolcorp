import XCTest

/// Two pages the Go app shares with the Mac: setup's example folder follows the app's own platform,
/// and the roles under a held desk can always be reached with arrows.
final class SharedPageFixesTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private func read(_ path: String) throws -> String {
        try String(contentsOf: Self.repo.appendingPathComponent(path), encoding: .utf8)
    }

    func testSetupShowsAFolderInThePlatformsOwnStyle() throws {
        let page = try read("Resources/web/setup.html")
        XCTAssertTrue(page.contains("placeholder: `Add a project folder, as ${exampleFolder()}`"))
        XCTAssertTrue(page.contains("state.platform === 'windows'"))
        XCTAssertTrue(page.contains("return '/home/you/project'"))
        XCTAssertFalse(page.contains("'Add a project folder, as /Users/you/project'"))
        // The Mac says so in its own setup state
        XCTAssertTrue(try read("Sources/AgentDeck/DeckRouter.swift").contains(#""platform": "darwin""#))
    }

    func testTheRolesUnderAHeldDeskHaveArrows() throws {
        let web = try read("Resources/web/office.js")
        XCTAssertTrue(web.contains("row.arrows.push({ step,"))
        XCTAssertTrue(web.contains("consider({ kind: 'role', chip: false, step: r.step }, r, -1)"))
        XCTAssertTrue(web.contains("target.step && rolesAt"))
        XCTAssertTrue(web.contains("e.key === 'ArrowRight' ? 1 : -1"), "the keyboard slides the roles too")
        XCTAssertTrue(web.contains("const stepRoles = (dir)"), "the arrows and keys share one step")
    }

    /// T739: the Office pane opens cheap, for a person and for a screen reader (Accessibility), and a refresh with
    /// nothing new changes nothing in it.
    func testTheOfficePaneIsDrawnOnlyWhenSeenAndLeftAloneByARefresh() throws {
        let page = try read("Resources/web/setup.html")
        // Style pictures: drawn when they come into view, once, and kept; never at once when the browser can wait.
        XCTAssertTrue(page.contains("new IntersectionObserver("))
        XCTAssertTrue(page.contains("if (seen) { waitingToShow.set(canvas, still); seen.observe(canvas); } else still();"))
        XCTAssertFalse(page.contains("    still();\n    const over = host || canvas;"), "no picture is drawn as it's made")
        // A tile is its name; its picture is only to look at.
        XCTAssertTrue(page.contains("shown.setAttribute('aria-hidden', 'true');"))
        // The state is asked for one request at a time.
        XCTAssertTrue(page.contains("if (loading) { loadAgain = true; return; }"))
        // Given up on after 10 s in every browser, not only where AbortSignal.timeout exists.
        XCTAssertTrue(page.contains("const timer = setTimeout(() => giveUp.abort(), 10000);"))
        XCTAssertTrue(page.contains("signal: giveUp.signal"))
        // What a refresh rebuilt each time is rebuilt only when its data changed.
        XCTAssertTrue(page.contains("if (!changed($('floors'), [state.floors, state.presets, state.floorRules, state.platform])) return;"))
        XCTAssertTrue(page.contains("if (!changed($('bots'), [bots, state.floors, state.platform])) return;"))
        XCTAssertTrue(page.contains("if (more.textContent !== word) more.textContent = word;"))
        XCTAssertTrue(page.contains("if ($('step-ontop').hidden === w.available) $('step-ontop').hidden = !w.available;"))
        // The welcome's mics are drawn only while the welcome is up (openWelcome draws them as it opens).
        XCTAssertTrue(page.contains("if (!$('invite').hidden) catalog('welcome-mic', mics());"))
    }
}
