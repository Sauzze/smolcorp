import XCTest
@testable import AgentDeck

/// Open in T3 Code (T792): what's found, the folder from the registry and its refusals, and `t3` run with exactly
/// its arguments, never through a shell. A fake home folder holds a fake `t3` that writes down what it was given;
/// the real `t3` and the real app are never run.
final class T3CodeTests: XCTestCase {
    private var home: URL!
    private var calls: URL { home.appendingPathComponent("calls") }
    private var mode: URL { home.appendingPathComponent("mode") }
    private var threads: URL { home.appendingPathComponent("threads") }
    private var next: URL { home.appendingPathComponent("next") }
    private var opened = 0

    override func setUpWithError() throws {
        home = FileManager.default.temporaryDirectory.appendingPathComponent("t3-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)
        home = home.resolvingSymlinksInPath()
        opened = 0
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: home)
    }

    private func folder(_ name: String) throws -> String {
        let url = home.appendingPathComponent(name, isDirectory: true)
        try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
        return url.path
    }

    private func json(_ response: Response) -> NSDictionary? {
        try? JSONSerialization.jsonObject(with: response.body) as? NSDictionary
    }

    private func text(_ response: Response) -> String { String(decoding: response.body, as: UTF8.self) }

    /// The run that lists `folder`'s threads.
    private func list(_ folder: String) -> [String] { ["thread", "list", "--project", folder, "--json"] }

    private func web(_ name: String) throws -> String {
        try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .appendingPathComponent("../../Resources/web/\(name)"), encoding: .utf8)
    }

    func testSetupsTeamListHasTheButtonItsHintsAndFitsAPhoneWidth() throws {
        let page = try web("setup.html")
        let row = try XCTUnwrap(page.range(of: "function teamRow(r) {"))
        // 5,500: M5b's Take over / Hand back menu made the row longer (M5c).
        let teamRow = String(page[row.lowerBound...].prefix(5500))
        XCTAssertTrue(teamRow.contains("const t3Key = state.t3CodeEnabled ? t3Button(r) : null;"), "T3 Code on (T796)")
        XCTAssertTrue(teamRow.contains("head.style.flexWrap = 'wrap';"), "390 px: the buttons go under the name")
        XCTAssertTrue(teamRow.contains("row.append(head, ...(t3Key ? [t3Key.line] : []), ...(why ? [why] : []));"), "the hint on its own line under the button")
        for words in ["textContent: 'Open in T3 Code', disabled: true", "fetch('/setup/t3code/open'", "JSON.stringify({ id: r.id })",
                      "'Opening T3 Code…'", "'T3 Code opens its own agent there, separate from this hire.'",
                      "Added to T3 Code's projects: open T3 Code to see it.", "t3.state === 'found' ? T3_NOTE : ''", ": t3.hint;",
                      #"unsure: "T3 Code has this hire's folder open as a project; it didn't confirm a new thread, so look in T3 Code.""#,
                      "const T3_EVERY = 30000", "`/setup/t3code${fresh ? '?fresh=1' : ''}`", "if (t3Key) row.addEventListener('pointerenter', () => t3State(true));"] {
            XCTAssertTrue(page.contains(words), words)
        }
        // Greyed for appOnly and none: only found and cliOnly may click.
        // (A hire in T3 Code, T797b, opens its own thread by T3 Code's link: that takes the app, and is greyed otherwise.)
        XCTAssertTrue(page.contains("      : t3.state === 'found' || t3.state === 'cliOnly');"))
        XCTAssertTrue(page.contains("button.disabled = open && open.busy ? true : !usable;"))
        // Fire… opened, the buttons' span shrinks, so its sentences wrap in the row (measured at 390 and 1280 px).
        XCTAssertTrue(teamRow.contains("keys.style.cssText = 'flex:0 1 auto;min-width:0;"))
        XCTAssertFalse(teamRow.contains("flex:none"))
        // The open outlives Setup's redraw every 1.5 s: kept by hire id outside the row, and restored as it's drawn.
        let button = try XCTUnwrap(page.range(of: "function t3Button(r) {"))
        let t3Button = String(page[button.lowerBound...].prefix(1600))
        for words in ["const t3Opening = new Map();"] { XCTAssertTrue(page.contains(words), words) }
        for words in ["paintT3Row(button, line);\n    button.onclick", "if ((t3Opening.get(r.id) || {}).busy) return;",
                      "t3Opening.set(r.id, { busy: true, text: 'Opening T3 Code…', at: Date.now() });",
                      "t3Opening.set(r.id, { busy: false, text, at: Date.now() });\n      paintT3();"] {
            XCTAssertTrue(t3Button.contains(words), words)
        }
        XCTAssertFalse(t3Button.contains("line.textContent ="), "the answer goes to the hire's open, never to the row it was clicked in")
        XCTAssertTrue(page.contains("const open = t3Opening.get(button.dataset.t3);"))
        XCTAssertFalse(teamRow.contains("worktree:") && teamRow.contains("/setup/t3code/open', { method: 'POST', headers: setupHeaders(true), body: JSON.stringify({ id: r.id, "),
                       "the id alone is sent")
    }

    func testTheOfficeOffersItOnlyInItsWindowOnThisComputerNeverOnAPhone() throws {
        let page = try web("deck.html")
        // The key exists only once the computer has answered /t3code, which it does to its own window alone; a phone never asks,
        // nor does a browser paired with its own key: only the window whose address marks it as the computer's own (M4b).
        XCTAssertTrue(page.contains("const ownWindow = bare || new URLSearchParams(location.search).has('own');"))
        XCTAssertTrue(page.contains("if (ownWindow && whole && !preview && !saver && !matchMedia('(pointer: coarse)').matches) {"))
        XCTAssertTrue(page.contains("if (!res || !res.ok) return;\n      t3Here = true;"))
        XCTAssertTrue(page.contains("...(t3Here ? [{ id: T3_BUTTON, label: 'T3 Code' }] : [])"))
        XCTAssertTrue(page.contains("if (t3Here && action === `button:${T3_BUTTON}`) return openT3(selected);"))
        XCTAssertEqual(page.components(separatedBy: "t3Here = true").count, 2, "set in one place")
        // The card: asked again as it opens, greyed with the hint, the line about T3 Code's agent, and the answers.
        for words in ["get('/t3code', { session: id, fresh: '1' })", "post('/t3code/open', { session: id })", "$('t3-open').disabled = !usable;",
                      "t3Say(usable ? (found.state === 'found' ? T3_NOTE : '') : found.hint);", "t3Say('Opening T3 Code…');",
                      "'T3 Code opens its own agent there, separate from this hire.'", "Added to T3 Code's projects: open T3 Code to see it.",
                      #"unsure: "T3 Code has this hire's folder open as a project; it didn't confirm a new thread, so look in T3 Code.""#,
                      #"<button class="t3-open" id="t3-open" disabled>Open in T3 Code</button>"#] {
            XCTAssertTrue(page.contains(words), words)
        }
        // 390 px: the card is a summary card, as wide as the screen allows.
        XCTAssertTrue(page.contains(#"<div class="summary" id="t3""#))
        XCTAssertTrue(page.contains("width: min(92vw, 420px)"))
    }
}
