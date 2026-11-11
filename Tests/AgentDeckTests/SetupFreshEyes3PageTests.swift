import XCTest
@testable import AgentDeck

/// Fresh eyes, round 3 (FX9): the Setup pages. A hidden folder is refused for what it is (HireFolderTests has
/// each case), the Mac's folder window closes once a typed folder is kept, the token's box fits a phone, a floor
/// chip's hover says what it covers (R790), and Floors says what it holds with one name per floor.
final class SetupFreshEyes3PageTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private func file(_ path: String) throws -> String {
        try String(contentsOf: Self.repo.appendingPathComponent(path), encoding: .utf8)
    }

    /// The text from `from` up to `to`, both found in the file.
    private func part(_ text: String, from: String, to: String) throws -> Substring {
        let start = try XCTUnwrap(text.range(of: from), from)
        let end = try XCTUnwrap(text.range(of: to, range: start.upperBound..<text.endIndex), to)
        return text[start.lowerBound..<end.lowerBound]
    }

    // MARK: The hires folder

    func testKeepingATypedFolderClosesTheMacsFolderWindow() throws {
        // The Go core asks the bridge's folder-panel-close: only the hires folder's panel, as if cancelled, so its
        // caller gets nothing picked; a floor's stays.
        let bridge = try file("Sources/AgentDeck/HostBridge.swift")
        XCTAssertTrue(bridge.contains("actions.closeFolderPanel(Deck.hireFolderMessage)"))
        let deck = try file("Sources/AgentDeck/Deck.swift")
        let close = try part(deck, from: "static func closeFolderPanel(for message: String) {", to: "\n    }")
        XCTAssertTrue(close.contains("guard let open = hirePanel, open.message == message else { return }"))
        XCTAssertTrue(close.contains("open.cancel(nil)"))
        // The page says so while the window is open.
        let page = try file("Resources/web/setup.html")
        XCTAssertTrue(page.contains("A folder window opened on your Mac: pick a folder there. Or type its path here and use it: the window then closes by itself.</p>"))
    }

    // MARK: The token's box on a phone

    func testTheTokensBoxFitsAPhone() throws {
        let page = try file("Resources/web/setup.html")
        // One column no wider than the box: the path can't widen it, so the text wraps inside, and the path is cut in the middle.
        XCTAssertTrue(page.contains(".gm-token .pad:not(.field) { display: grid; grid-template-columns: minmax(0, 1fr); gap: 6px; }"))
        XCTAssertTrue(page.contains(".token-file code { font-size: 12px; user-select: all; flex: 0 1 auto; min-width: 0; max-width: 100%; }"))
        XCTAssertTrue(page.contains(".token-file { display: flex; flex-wrap: wrap;"), "Copy path goes under the path when there's no room")
        XCTAssertTrue(page.contains("pathLine($('gm-token-file'), m.gmTokenFileShown || file, file);"), "whole on hover; Copy path copies it whole")
        XCTAssertTrue(page.contains("$('gm-token-copy').onclick = () => copyText(file, 'gm-token-copied');"))
    }

    // MARK: Floors

    func testAFloorChipsHoverSaysWhatItCovers() throws {
        let page = try file("Resources/web/setup.html")
        let rules = try part(page, from: "function floorRulesRow(", to: "function pathLine(")
        // The path's own title is the chip's words, so hovering the path says it too (R790, finding 2).
        XCTAssertTrue(rules.contains("const says = rule.exact ? `${rule.folder}, this folder alone` : `${rule.folder}, and the folders in it`;"))
        XCTAssertTrue(rules.contains("pathLine(document.createElement('span'), shortFolder(rule.folder) + (rule.exact ? ' (alone)' : ''), says)"))
        XCTAssertTrue(rules.contains("chip.title = says;"))
    }

    func testFloorsSaysWhatItHoldsWithOneNamePerFloor() throws {
        let page = try file("Resources/web/setup.html")
        XCTAssertTrue(page.contains("<h1>Floors</h1><p>Each floor of the office: its name, its style, and which projects sit there.</p>"))
        // No promise of cloud bots: they have their own group, shown only where there are any.
        let pane = try part(page, from: #"<section class="pane" data-pane="floors""#, to: #"<div class="group" id="bots-group""#)
        XCTAssertFalse(pane.lowercased().contains("cloud bot"))
        XCTAssertFalse(pane.contains("above the office"))
        XCTAssertTrue(pane.contains("A floor's name is up to you: with none, the office shows its number, such as 2F."))
        let floors = try part(page, from: "function renderFloors() {", to: "function renderBots() {")
        XCTAssertTrue(floors.contains("placeholder: 'Name (optional)'"))
        XCTAssertFalse(floors.contains("placeholder: `${i + 1}F`"), "no second name beside Floor 2")
        XCTAssertTrue(floors.contains("name.setAttribute('aria-label', `${label.textContent}'s name (optional)`);"))
    }
}
