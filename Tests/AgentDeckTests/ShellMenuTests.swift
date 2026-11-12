import AppKit
import XCTest
@testable import AgentDeck

/// The Go shell's menus, as the Swift app's were (StatusMenu and AppDelegate before 1.25.0), built from the menu's
/// state the core sends: the shared fixture's menus (desktop/core_darwin_test.go checks the core's side), the
/// Dock's, the icon and the count beside it, and the ops the app asks. No status item is made, and no menu shown.
final class ShellMenuTests: XCTestCase {
    private func fixture() throws -> [String: Any] {
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("fixtures/shell/ops.json")
        return try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])
    }

    /// Each item's title, "-" for a separator, " (off)" after one that can't be chosen, " ✓" after one ticked, a
    /// submenu's items indented by two spaces and an indented item by four, as the fixture writes them.
    private func lines(_ menu: NSMenu, indent: String = "") -> [String] {
        menu.items.flatMap { item -> [String] in
            if item.isSeparatorItem { return [indent + "-"] }
            var line = indent + String(repeating: "    ", count: item.indentationLevel) + item.title
            if item.action == nil, item.submenu == nil { line += " (off)" }
            if item.state == .on { line += " ✓" }
            return [line] + (item.submenu.map { lines($0, indent: indent + "  ") } ?? [])
        }
    }

    func testTheAppAsksOnlyTheFixturesOps() throws {
        let ops = try XCTUnwrap(fixture()["ops"] as? [[String: Any]])
        XCTAssertEqual(GoCore.Op.allCases.map(\.rawValue), ops.compactMap { ($0["request"] as? [String: Any])?["op"] as? String })
    }

    func testEachMenuIsTheFixtures() throws {
        let menus = try XCTUnwrap(fixture()["menus"] as? [[String: Any]])
        XCTAssertEqual(menus.count, 4)
        for menu in menus {
            let name = menu["name"] as? String ?? "?"
            let state = try XCTUnwrap(ShellMenu((menu["pushed"] as? [String: Any])?["menu"]), name)
            let office = try XCTUnwrap(menu["office"] as? [String: Bool], name)
            let window = OfficeMenu(shown: office["shown"]!, onTop: office["onTop"]!, allSpaces: office["allSpaces"]!,
                                    clickThrough: office["clickThrough"]!, upright: office["upright"]!)
            let built = NSMenu()
            GoShellDelegate.fill(built, headline: GoShellDelegate.headline(port: 7779, problem: nil, sessions: state.sessions, testCopy: false),
                                 state: state, office: window, openAtLogin: menu["openAtLogin"] as? Bool ?? false, target: nil)
            XCTAssertEqual(lines(built), menu["lines"] as? [String], name)
            XCTAssertEqual(GoShellDelegate.beside(state), menu["beside"] as? String, name)
        }
    }

    /// Before the core is ready, or once it has stopped: why, and what needs no core.
    func testWithoutTheCoreOnlyWhatNeedsNone() {
        let built = NSMenu()
        GoShellDelegate.fill(built, headline: GoShellDelegate.headline(port: 7779, problem: "The Go engine is starting…", sessions: 3),
                             state: nil, office: nil, openAtLogin: true, target: nil)
        XCTAssertEqual(lines(built), ["The Go engine is starting… (off)", "-", "Install Screen Saver…", "Open at Login ✓", "-", "Quit Smolcorp"])
        // Ready, before the core has said how the deck is: no phones to unpair, no portable office yet.
        GoShellDelegate.fill(built, headline: GoShellDelegate.headline(port: 7779, problem: nil, sessions: nil, testCopy: false),
                             state: nil, office: OfficeMenu(shown: false, onTop: false, allSpaces: false, clickThrough: false, upright: false),
                             openAtLogin: false, target: nil)
        XCTAssertEqual(Array(lines(built).prefix(8)), ["Smolcorp is running (off)", "-", "Settings…", "Open Settings in Browser",
                                                       "Pair a Phone…", "Show Me Around Again", "-", "Show Office"])
    }

    func testTheFirstLinesNameATestCopyAndTheSessions() {
        XCTAssertEqual(GoShellDelegate.headline(port: 7790, problem: nil, sessions: nil, testCopy: true), ["Smolcorp Test · port 7790"])
        XCTAssertEqual(GoShellDelegate.headline(port: 7790, problem: nil, sessions: 1, testCopy: true), ["Smolcorp Test · port 7790", "1 agent session"])
        XCTAssertEqual(GoShellDelegate.headline(port: 7710, problem: nil, sessions: nil, testCopy: false), ["Smolcorp is running"])
        XCTAssertEqual(GoShellDelegate.headline(port: 7710, problem: nil, sessions: 0, testCopy: false), ["0 agent sessions"])
        XCTAssertEqual(GoShellDelegate.headline(port: 7710, problem: "Gone.", sessions: 2, testCopy: true), ["Gone."])
    }

    /// RMENU N1: Settings, before the core is ready (or once it's gone), opens the setup page in the browser, as the
    /// Swift app did; with the window, never.
    func testSettingsFallsBackToTheBrowserWithoutTheWindow() {
        XCTAssertEqual(GoShellDelegate.browserFallback(windowReady: false, port: 7779, then: nil)?.absoluteString, "http://localhost:7779/setup")
        XCTAssertEqual(GoShellDelegate.browserFallback(windowReady: false, port: 7779, then: "pair")?.absoluteString, "http://localhost:7779/setup#pair")
        XCTAssertNil(GoShellDelegate.browserFallback(windowReady: true, port: 7779, then: "pair"))
    }

    func testTheShowOfficeKeysAreTheChosenOnes() throws {
        let built = NSMenu()
        GoShellDelegate.fill(built, headline: [], state: nil,
                             office: OfficeMenu(shown: true, onTop: false, allSpaces: false, clickThrough: false, upright: false,
                                                key: "o", modifiers: [.command, .option]), openAtLogin: false, target: nil)
        let show = try XCTUnwrap(built.items.first { $0.title == "Show Office" })
        XCTAssertEqual(show.keyEquivalent, "o")
        XCTAssertEqual(show.keyEquivalentModifierMask, [.command, .option])
        XCTAssertEqual(built.items.first { $0.title == "Settings…" }?.keyEquivalent, ",")
        XCTAssertEqual(built.items.first { $0.title == "Quit Smolcorp" }?.keyEquivalent, "q")
    }

    func testTheDocksMenu() {
        let menu = GoShellDelegate.dockMenu(officeShown: true, target: nil)
        XCTAssertEqual(lines(menu), ["Show Office ✓", "Settings…", "Pair a Phone…"])
        XCTAssertEqual(lines(GoShellDelegate.dockMenu(officeShown: false, target: nil)).first, "Show Office")
    }

    func testTheIconHasADotWhileThePortableOfficeIsOn() throws {
        var state = try XCTUnwrap(ShellMenu(["logo": "keys", "waiting": 2, "portable": ["on": true]]))
        XCTAssertTrue(GoShellDelegate.icon(for: state) === StatusMenu.dotted(.keys))
        XCTAssertTrue(StatusMenu.dotted(.keys).isTemplate)
        XCTAssertEqual(StatusMenu.dotted(.keys).size, StatusMenu.icon(for: .keys).size)
        state.portable.on = false
        XCTAssertTrue(GoShellDelegate.icon(for: state) === StatusMenu.icon(for: .keys))
        XCTAssertEqual(GoShellDelegate.beside(state), " 2")
        XCTAssertEqual(GoShellDelegate.beside(nil), "")
        // The mark the app had before it was Smolcorp is now the hire, as config.json reads it.
        XCTAssertEqual(ShellMenu(["logo": "sauzze"])?.logo, .smolcorp)
        XCTAssertNil(ShellMenu("not a menu"))
    }

    /// Copy viewer link: on a pasteboard of the test's own, never yours.
    func testTheViewerLinkIsCopiedForThisMacOnly() {
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("smolcorp-test-\(UUID().uuidString)"))
        defer { pasteboard.releaseGlobally() }
        GoShellDelegate.copyForThisMacOnly("https://viewer.example/v/abc#k=fake", to: pasteboard)
        XCTAssertEqual(pasteboard.string(forType: .string), "https://viewer.example/v/abc#k=fake")
        let types = pasteboard.types?.map(\.rawValue) ?? []
        XCTAssertTrue(types.contains("org.nspasteboard.ConcealedType"), "\(types)")
        XCTAssertTrue(types.contains("org.nspasteboard.TransientType"), "\(types)")
    }

    /// Open at Login is pointed at this copy of the app as it starts, as the Swift app did; never by a test copy.
    func testTheLoginItemIsRefreshedAtLaunchButNotByATestCopy() {
        var enabled = 0
        LoginItem.refresh(testCopy: true, isEnabled: { true }, enable: { enabled += 1 })
        XCTAssertEqual(enabled, 0, "a test copy leaves it to the copy in use")
        LoginItem.refresh(testCopy: false, isEnabled: { false }, enable: { enabled += 1 })
        XCTAssertEqual(enabled, 0, "off stays off")
        LoginItem.refresh(testCopy: false, isEnabled: { true }, enable: { enabled += 1 })
        XCTAssertEqual(enabled, 1)
    }
}
