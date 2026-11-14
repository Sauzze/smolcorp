import Foundation
import XCTest
@testable import AgentDeck

/// A test copy can never disturb the owner (TCH): it never comes forward or
/// shows a window or an alert by itself, only logs it, and listens on this Mac
/// only unless SMOLCORP_TEST_LISTEN=lan asks for the network.
final class TestCopyQuietTests: XCTestCase {
    // MARK: Nothing in front by itself

    func testATestCopyLogsInsteadOfComingForward() {
        var logged: [String] = []
        XCTAssertFalse(TestCopy.mayComeForward("first-run Settings", testCopy: true) { logged.append($0) })
        XCTAssertEqual(logged, ["Smolcorp Test: first-run Settings not shown, a test copy never comes forward by itself"])
    }

    func testTheRealAppComesForwardAndLogsNothing() {
        var logged: [String] = []
        XCTAssertTrue(TestCopy.mayComeForward("first-run Settings", testCopy: false) { logged.append($0) })
        XCTAssertEqual(logged, [])
    }

    /// Every activation of the app goes through `Foreground.activate`, so through the check: it's the
    /// one `NSApp.activate` left in the app's code.
    func testEveryActivationGoesThroughTheCheck() throws {
        var found: [String] = []
        for name in try FileManager.default.contentsOfDirectory(atPath: sources.path) where name.hasSuffix(".swift") {
            let text = try source(name)
            for line in text.components(separatedBy: "\n") where line.contains("NSApp.activate(") {
                found.append(name + ": " + line.trimmingCharacters(in: .whitespaces))
            }
        }
        XCTAssertEqual(found, ["App.swift: NSApp.activate(ignoringOtherApps: true)"])
        let app = try source("App.swift")
        XCTAssertTrue(app.contains("""
                guard TestCopy.mayComeForward(what) else { return }
                NSApp.activate(ignoringOtherApps: true)
        """))
    }

    /// What either shell would show by itself: first-run Settings (both), the Go engine's stopped
    /// alert, setup's Open the office (both) and the folder panel (a page's, through the deck or the bridge).
    func testWhatWouldShowByItselfIsCheckedFirst() throws {
        let shell = try source("GoShell.swift"), deck = try source("Deck.swift")
        XCTAssertTrue(shell.contains("if !Config.shared.hasCompletedSetup, TestCopy.mayComeForward(\"first-run Settings\") { settings?.show() }"))
        XCTAssertTrue(shell.contains("if TestCopy.mayComeForward(\"the office (from setup)\") { self?.window?.show() }"))
        XCTAssertTrue(deck.contains("guard TestCopy.mayComeForward(\"the folder panel\") else { return done(nil) }"))

        // The engine-stopped alert: checked before it's made, and its windows closed either way.
        let stopped = try XCTUnwrap(shell.range(of: "private func coreStopped")).upperBound
        let body = String(shell[stopped...].prefix(1200))
        let check = try XCTUnwrap(body.range(of: "guard TestCopy.mayComeForward(\"the alert “\\(said)”\") else { return }"))
        XCTAssertLessThan(try XCTUnwrap(body.range(of: "settings = nil")).lowerBound, check.lowerBound, "windows closed first")
        XCTAssertLessThan(check.lowerBound, try XCTUnwrap(body.range(of: "NSAlert()")).lowerBound)
    }

    // MARK: Helpers

    private var sources: URL {
        URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Sources/AgentDeck")
    }

    private func source(_ name: String) throws -> String {
        try String(contentsOf: sources.appendingPathComponent(name), encoding: .utf8)
    }
}
