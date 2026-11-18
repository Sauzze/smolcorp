import XCTest
@testable import AgentDeck

/// Fresh eyes, round 2 (FX6): similar settings that behave alike. Floors chooses and says "Saved" as the hires
/// folder does, long paths keep their end, a fresh office is in Classic, a style's own pets are said, and the
/// token's switch sits on the right like every other.
final class SetupConsistencyPageTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private func file(_ path: String) throws -> String {
        try String(contentsOf: Self.repo.appendingPathComponent(path), encoding: .utf8)
    }

    /// The text from `from` up to `to`, both found in the page.
    private func part(_ page: String, from: String, to: String) throws -> Substring {
        let start = try XCTUnwrap(page.range(of: from), from)
        let end = try XCTUnwrap(page.range(of: to, range: start.upperBound..<page.endIndex), to)
        return page[start.lowerBound..<end.lowerBound]
    }

    // MARK: Floors, like the hires folder

    func testAFloorChoosesWithTheMacsPanelAndSaysSaved() throws {
        let page = try file("Resources/web/setup.html")
        let rules = try part(page, from: "function floorRulesRow(", to: "function pathLine(")
        // Choose…, on a Mac only: the Go app's route would keep the folder as the hires folder.
        XCTAssertTrue(rules.contains("if (onMac() && state.hiring && state.hiring.canChoose) {"))
        XCTAssertTrue(rules.contains("textContent: 'Choose…' }"))
        XCTAssertTrue(rules.contains("fetch('/setup/hiring/choose?pick=1&for=floor', { method: 'POST', headers: setupHeaders(true) })"))
        XCTAssertTrue(rules.contains("if (answer && answer.picked) keepFolder(answer.picked);"))
        XCTAssertTrue(rules.contains("choose.blur();"), "out of the rows, so the new chip is drawn")
        // Typed or chosen, the same: kept, then "Saved ✓", which outlasts the list drawn again.
        XCTAssertTrue(rules.contains("keepFolder(folder);"))
        XCTAssertTrue(rules.contains("if (res && res.ok) floorSaved(i);"))
        XCTAssertTrue(rules.contains("className: 'hire-saved', id: `floor-saved-${i}`"))
        XCTAssertTrue(rules.contains("if (floorSavedAt === i) Object.assign(saved, { textContent: 'Saved ✓', className: 'hire-saved good' });"))
        XCTAssertTrue(rules.contains("said(`floor-saved-${i}`, 'Saved ✓', 'good');"))
        XCTAssertTrue(page.contains("return post('/setup/floors', { floors,"), "sendFloors answers, for Saved")
    }

    func testTheFloorsPanelSaysWhatItsFor() throws {
        XCTAssertEqual(Deck.hireFolderMessage, "New hires from the office start their work in this folder.")
        XCTAssertEqual(Deck.floorFolderMessage, "The agents working in this project's folder sit on the floor you chose it for.")
        let bridge = try file("Sources/AgentDeck/HostBridge.swift")
        XCTAssertNotNil(bridge.range(of: #"let messages = ["hires": Deck.hireFolderMessage, "floor": Deck.floorFolderMessage]"#))
        let deck = try file("Sources/AgentDeck/Deck.swift")
        XCTAssertNotNil(deck.range(of: "panel.message = message\n"))
    }

    // MARK: Long paths keep their end

    func testLongPathsKeepTheirEndEverywhere() throws {
        let page = try file("Resources/web/setup.html")
        XCTAssertTrue(page.contains(".path-mid > .head { flex: 0 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; }"))
        XCTAssertTrue(page.contains(".path-mid > .tail { flex: 0 0 auto;"))
        let helper = try part(page, from: "function pathLine(el, shown, full = shown) {", to: "function shortFolder(")
        XCTAssertTrue(helper.contains("el.title = full || '';"), "whole on hover")
        // Each chip or line that shows a path.
        for use in ["pathLine(document.createElement('span'), shortFolder(rule.folder) + (rule.exact ? ' (alone)' : ''), says)",
                    "pathLine($('first-folder-short'), tildePath(f.folder, f.home), f.folder);",
                    "pathLine($('first-folder-path'), tildePath(f.folder || f.suggested, f.home), f.folder || f.suggested);",
                    "pathLine(Object.assign(document.createElement('span'), { className: 'dir' }), p.dir);",
                    "pathLine(Object.assign(document.createElement('span'), { className: 'dir' }), f.dir));"] {
            XCTAssertTrue(page.contains(use), use)
        }
        // None of them wraps anywhere any more, nor shows only its start.
        XCTAssertFalse(page.contains(".floor-rules .rule .path { overflow: hidden;"))
        XCTAssertFalse(page.contains(".token-file code { font-size: 12px; overflow-wrap: anywhere;"))
        XCTAssertFalse(page.contains(#".ff-path { font: 600 14px ui-monospace, "SF Mono", Menlo, monospace; overflow-wrap: anywhere; }"#))
    }

    /// The splitter, run: the head gives way, the folder's name stays.
    func testThePathSplitsBeforeItsLastFolder() throws {
        let page = try file("Resources/web/setup.html")
        let helper = try part(page, from: "function pathLine(el, shown, full = shown) {", to: ".exec(shown || '')")
        let pattern = try XCTUnwrap(helper.range(of: "/^").map { String(helper[$0.lowerBound...]).dropFirst().dropLast() })
        let regex = try NSRegularExpression(pattern: String(pattern))
        func split(_ path: String) -> [String] {
            guard let m = regex.firstMatch(in: path, range: NSRange(path.startIndex..., in: path)) else { return ["", path] }
            return (1...2).map { String(path[Range(m.range(at: $0), in: path)!]) }
        }
        XCTAssertEqual(split("~/Projects/smolcorp/.team/scratch/demo-project"), ["~/Projects/smolcorp/.team/scratch", "/demo-project"])
        XCTAssertEqual(split("~/Projects/demo (alone)"), ["~/Projects", "/demo (alone)"])
        XCTAssertEqual(split(#"C:\Users\you\project"#), [#"C:\Users\you"#, #"\project"#])
        XCTAssertEqual(split("~"), ["", "~"])
    }

    // MARK: A fresh office is in Classic

    func testAFreshOfficeIsInClassicAndYoursOnlyOnceSomethingsChanged() throws {
        let page = try file("Resources/web/setup.html")
        let helpers = try part(page, from: "const ownUntouched = (room) =>", to: "/** The pets as kept")
        XCTAssertTrue(helpers.contains("[...STYLED].every((part) => room[part] === undefined || JSON.stringify(room[part]) === JSON.stringify(STANDARD_ROOM[part]))"))
        XCTAssertTrue(helpers.contains("return preset === 'yours' && ownUntouched(room) ? 'classic' : preset;"))
        XCTAssertTrue(helpers.contains("function roomArranged(room) {"))
        // The Style card, its "Now:" and the welcome read Classic; the Yours tile only once it holds something.
        let easy = try part(page, from: "function drawEasy(room, shown) {", to: "for (const part of ['sky', 'season'])")
        XCTAssertTrue(easy.contains("const style = styleOf(room);"))
        XCTAssertTrue(page.contains("items, pinned: [...(part === 'preset' && !ownUntouched(room) ? ['yours'] : []),"))
        XCTAssertTrue(page.contains("roomCatalog('preset', room, styleOf(room),"))
        // Arranged as the welcome tells it.
        XCTAssertTrue(helpers.contains("return !!plan && (['wide', 'tall'].some((side) => Object.keys(plan[side] || {}).length"))
        // Classic is the standard, so the two look the same.
        XCTAssertTrue(try file("Resources/web/office.js").contains("classic: { name: 'Classic', room: {} },"))
    }

    // MARK: A style's pets, said

    func testAStyleThatBringsPetsSaysSo() throws {
        let page = try file("Resources/web/setup.html")
        let card = try part(page, from: #"<div class="card style-card" id="look-style""#, to: #"<div class="catalog" id="easy-style"></div>"#)
        XCTAssertTrue(card.contains(#"<p class="hint" id="easy-style-pets" hidden></p>"#))
        let easy = try part(page, from: "function drawEasy(room, shown) {", to: "for (const part of ['sky', 'season'])")
        XCTAssertTrue(easy.contains("const stylePets = style !== 'yours' && !petsOf(room.pet).length ? petsOf(shown.pet) : [];"))
        XCTAssertTrue(easy.contains("brings its own ${stylePets.length === 1 ? 'pet' : 'pets'}: ${petNames}. Pick pets below to have yours instead."))
        XCTAssertTrue(easy.contains("$('easy-style-pets').hidden = !stylePets.length;"))
    }

    // MARK: The token's switch on the right

    func testTheTokensSwitchSitsOnTheRightLikeTheOthers() throws {
        let page = try file("Resources/web/setup.html")
        XCTAssertFalse(page.contains(".gm-token .pad { display: grid;"), "its switch rows aren't stacked")
        XCTAssertTrue(page.contains(".gm-token .pad:not(.field) { display: grid; grid-template-columns: minmax(0, 1fr); gap: 6px; }"))
        let block = try part(page, from: #"<div class="box gm-token" id="gm-token""#, to: #"<div id="manager-tools" hidden>"#)
        XCTAssertTrue(block.contains(#"<div class="field pad">"# + "\n" + #"          <span class="text">Allow a General Manager started by hand"#))
        XCTAssertTrue(page.contains(".field { display: flex; align-items: center; gap: 12px; justify-content: space-between; min-width: 0; }"))
    }
}
