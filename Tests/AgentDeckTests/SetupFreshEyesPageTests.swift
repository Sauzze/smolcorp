import XCTest

/// Fresh eyes, round 1 (FX3): the Setup pages a newcomer stumbled on. The GM's token said plainly, the
/// hires folder in a row of its own, Floors that fit and fold, Office and look folded under Style, and pairing's words.
final class SetupFreshEyesPageTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private func page() throws -> String {
        try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/setup.html"), encoding: .utf8)
    }

    /// The text from `from` up to `to`, both found in the page.
    private func part(_ page: String, from: String, to: String) throws -> Substring {
        let start = try XCTUnwrap(page.range(of: from), from)
        let end = try XCTUnwrap(page.range(of: to, range: start.upperBound..<page.endIndex), to)
        return page[start.lowerBound..<end.lowerBound]
    }

    // MARK: D. The GM's token

    func testTheTokenBlockSaysWhatItIsWhoGetsItAndHowOneStartedByHandAsks() throws {
        let page = try page()
        let block = try part(page, from: #"<div class="box gm-token" id="gm-token""#, to: #"<div id="manager-tools" hidden>"#)
        XCTAssertTrue(block.contains("data-keywords=\"gm general manager token"), "search \"token\" finds it")
        XCTAssertTrue(block.contains("The token is your General Manager's key to Smolcorp"))
        XCTAssertTrue(block.contains("This page never shows it."))
        XCTAssertTrue(block.contains("A General Manager you hire from Smolcorp (Hire your General Manager, on Easy setup) gets it by itself"))
        XCTAssertTrue(block.contains("For a General Manager you start yourself, turn on <b>Allow a General Manager started by hand</b>"))
        // No file holds it (1.26, SK2): one started by hand asks with smolcorp gm login, and you Allow it here.
        XCTAssertTrue(block.contains("then start it in Terminal with <code>smolcorp gm login</code>"))
        XCTAssertTrue(block.contains("check that Terminal shows the same one, then click Allow"))
        XCTAssertTrue(block.contains("no file holds it"))
        XCTAssertFalse(block.contains("gm-token-copy"), "no path to copy: there is no file")
        XCTAssertTrue(block.contains(#"id="gm-byhand""#), "the switch sits with the words about it, manager on or off")
        // Find a setting lands on it, and go() lights what it lands on.
        XCTAssertTrue(page.contains(#"<button class="jump" data-go="team" data-at="gm-token"><b>Your General Manager's token</b>"#))
        // Never the token, and no path: there is no file.
        let render = try part(page, from: "function renderGmToken(m)", to: "function renderManagerTools")
        XCTAssertFalse(render.contains("gm-token-file"))
        XCTAssertTrue(render.contains("$('gm-rotate-row').hidden = !m.enabled;"), "a new token only where the app makes one")
        // Off a Mac (Smolcorp on Linux and Windows, no GM started by hand), nothing of it is offered.
        XCTAssertTrue(render.contains("$('gm-token-head').hidden = $('gm-token').hidden = !onMac();\n    if (!onMac()) return;"))
        // Drawn with the manager off too, before renderManager stops there.
        let manager = try part(page, from: "function renderManager()", to: "if (!m.enabled) return;")
        XCTAssertTrue(manager.contains("renderGmToken(m);"))
    }

    func testTheSwitchSaysHireYourGeneralManagerTurnsItOn() throws {
        let page = try page()
        let step = try part(page, from: #"<div class="step" id="step-manager""#, to: #"<button class="switch" id="manager""#)
        XCTAssertTrue(step.contains(#"data-pending="Off: Smolcorp works as before. Hire your General Manager, on Easy setup, turns this on for you."#))
        XCTAssertTrue(step.contains("Hire your General Manager, on Easy setup, turns this on for you. Turning it off fires no one."))
    }

    func testTheTeamLinesDontContradictEachOther() throws {
        let page = try page()
        XCTAssertFalse(page.contains("No one is hired besides the General Manager."))
        XCTAssertFalse(page.contains("'No General Manager is hired.'"))
        let tools = try part(page, from: "function renderManagerTools", to: "function resetRotate")
        XCTAssertTrue(tools.contains("gm || byHand ? 'No one else is on the team yet.' : 'No one is hired yet.'"))
        // The small headings sit between the boxes, with room: not the old hint headings crammed against them.
        XCTAssertFalse(page.contains(#"<h3 class="hint">Hired, its token, and by hand</h3>"#))
        XCTAssertTrue(page.contains(#"<h3 class="sub" id="gm-token-head">Its token</h3>"#))
        XCTAssertTrue(page.contains(#"<h3 class="sub">Team</h3>"#))
        XCTAssertTrue(page.contains(".group > h3.sub, #manager-tools > h3.sub { font-size: 12px; font-weight: 600; color: var(--dim); margin: 8px 2px 0; }"))
    }

    /// "Yes, make a new token" waits long enough for a slow reader, and an earlier ask's timer can't put it back early.
    func testTheNewTokenConfirmWaitsForASlowReader() throws {
        let page = try page()
        let wait = try part(page, from: "const ROTATE_WAIT = ", to: ";")
        XCTAssertGreaterThanOrEqual(Int(wait.dropFirst("const ROTATE_WAIT = ".count)) ?? 0, 8000)
        let rotate = try part(page, from: "function resetRotate()", to: "/** One hire on the team")
        XCTAssertTrue(rotate.contains("clearTimeout(rotateTimer);"))
        XCTAssertTrue(rotate.contains("if (button.matches(':hover')) return later();"), "never put back under the pointer")
    }

    // MARK: A. The hires folder

    func testTheHiresFolderHasARowOfItsOwn() throws {
        let page = try page()
        let row = try part(page, from: #"<div class="step" id="step-hire-folder""#, to: #"<div class="step" id="step-hiring""#)
        XCTAssertTrue(row.contains(#"<div class="title">New hires work in</div>"#))
        XCTAssertTrue(row.contains(#"id="hire-folder""#))
        XCTAssertTrue(row.contains(#"id="hire-choose""#))
        XCTAssertTrue(row.contains(#"id="hire-folder-saved" role="status""#))
        XCTAssertTrue(row.contains("The same folder as Start here's, on Easy setup."))
        // Not inside the hire and fire switch's row any more.
        let hiring = try part(page, from: #"<div class="step" id="step-hiring""#, to: #"<button class="switch" id="hiring""#)
        XCTAssertFalse(hiring.contains(#"id="hire-folder""#))
        // Saved, said on a save; Find a setting uses Start here's word.
        XCTAssertTrue(page.contains("said('hire-folder-saved', 'Saved ✓', 'good');"))
        XCTAssertTrue(page.contains(#"data-at="step-hire-folder"><b>Change where hires work</b><small>The folder your hires work in"#))
        XCTAssertFalse(page.contains("Change where hires start"))
    }

    // MARK: B. Floors

    func testAFloorsProjectHasAnAddButtonAndItsWholePath() throws {
        let page = try page()
        let rules = try part(page, from: "function floorRulesRow(", to: "function shortFolder(")
        XCTAssertTrue(rules.contains("textContent: 'Add' }"))
        XCTAssertTrue(rules.contains("add.onchange = keep;"), "Enter still adds")
        XCTAssertTrue(rules.contains("adding.onclick = keep;"))
        XCTAssertTrue(rules.contains("const says = rule.exact ? `${rule.folder}, this folder alone` : `${rule.folder}, and the folders in it`;"))
        XCTAssertTrue(rules.contains("chip.title = says;"))
        XCTAssertTrue(rules.contains("pathLine(document.createElement('span'), shortFolder(rule.folder)"))
        XCTAssertFalse(rules.contains("'/…'"), "no more smolcorp/… chips")
        // Inside the box, and whole on a phone.
        XCTAssertTrue(page.contains(".box > .floor-rules { padding: 0 14px 10px; border-top: 0; }"))
        XCTAssertTrue(page.contains(".field.floor input[type=text], .field.floor select { flex: 1 1 0; min-width: 0; max-width: none; }"))
        XCTAssertTrue(page.contains("<small>Floor names, styles and the projects on each · Floors</small>"))
    }

    func testCloudBotsFoldUnlessOneIsInUseAndNamesEndWithAnEllipsis() throws {
        let page = try page()
        XCTAssertTrue(page.contains(#"<div class="group" id="bots-group" data-folds hidden>"#))
        let bots = try part(page, from: "function renderBots()", to: "/** A bot's mailbox folder")
        XCTAssertTrue(bots.contains("const inUse = list.some((bot) => !bot.discovered || bot.pinned || bot.floor || bot.mailbox);"))
        XCTAssertTrue(bots.contains("if (!$('bots-group').dataset.touched) setFold($('bots-group'), !inUse);"))
        XCTAssertTrue(bots.contains("className: 'bot-name', title: bot.name"))
        XCTAssertTrue(page.contains(".field.floor > .bot-name b { font-weight: inherit; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }"))
    }

    /// What search or Find a setting lands on, in a folded section, is opened first.
    func testGoOpensAFoldedSection() throws {
        let page = try page()
        let go = try part(page, from: "function go(pane, at = null, moveFocus = true) {", to: "at.scrollIntoView(")
        XCTAssertTrue(go.contains("const folded = at && at.closest('[data-folds].folded');"))
        XCTAssertTrue(go.contains("if (folded) { folded.dataset.touched = '1'; setFold(folded, false); }"))
    }

    // MARK: C. Office and look

    func testOfficeAndLookFoldsUnderStyleAndExplainsYours() throws {
        let page = try page()
        let office = try part(page, from: #"<section class="pane layout" id="layout" data-pane="office""#, to: "<!-- The same choices as above, in their older places")
        for section in [#"data-search="Mic" data-keywords="microphone talk sound waves" data-folds"#, "<div class=\"group\" data-folds>\n      <h2>The room</h2>",
                        "<div class=\"group\" data-folds>\n      <h2>Sky and air</h2>", "<div class=\"group\" data-folds>\n      <h2>Life</h2>",
                        #"id="window-group" data-folds hidden"#, #"data-keywords="save switch export import rename profile saved looks" data-folds hidden"#] {
            XCTAssertTrue(office.contains(section), section)
        }
        let style = try XCTUnwrap(office.range(of: #"id="look-style""#))
        let firstFold = try XCTUnwrap(office.range(of: "data-folds"))
        XCTAssertLessThan(style.lowerBound, firstFold.lowerBound, "Style comes first, open")
        XCTAssertFalse(office[style.lowerBound..<office.range(of: "<div class=\"cards easy-look\">")!.lowerBound].contains("data-folds"))
        XCTAssertTrue(office.contains("Yours: as you've arranged it, starting from Classic"))
        XCTAssertTrue(page.contains("$('easy-style-yours').hidden = style !== 'yours';"))
    }

    func testRecentDoesntMoveTheTilesUnderThePointer() throws {
        let page = try page()
        let draw = try part(page, from: "function drawCatalog(c) {", to: "// The shelves: by group")
        XCTAssertTrue(draw.contains("if (!title) { h4.style.visibility = 'hidden'; h4.setAttribute('aria-hidden', 'true'); }"), "its heading's room is kept")
        XCTAssertTrue(draw.contains("if (c.visit !== paneVisit || !c.recentIds) { c.visit = paneVisit; c.recentIds = keptList(recentKey); }"))
        XCTAssertTrue(draw.contains("parts.push(lane(recent.length ? 'Recent' : null, [...pinned, ...recent], 'recent', true));"))
        XCTAssertTrue(page.contains("if (pane !== document.querySelector('.pane.current')?.dataset.pane) paneVisit++;"))
    }

    // MARK: E. Pairing

    func testPairingSaysTheQRCaptionAnyWayRoundAndTheTailscaleAddress() throws {
        let page = try page()
        XCTAssertFalse(page.contains("Pairs with the code beside it"))
        // The QR code holds the key it hands over, not the code (SECWIFI): a typed code works over Tailscale only.
        XCTAssertTrue(page.contains(#"<div class="qr-note">Works once, as the 6 digits do. It hands your phone its key without sending it over the Wi-Fi: keep it to yourself.</div>"#))
        // Over the Wi-Fi the QR code is the only way, so it comes first; the code is typed over Tailscale (FXSEC N8).
        XCTAssertTrue(page.contains(#"<li>Get a code, then scan the QR code with your phone's camera: it opens <span class="address" id="address"></span> and pairs.</li>"#))
        XCTAssertTrue(page.contains(#"<li>Over Tailscale, you can type this code instead<span id="pair-tailnet" hidden>, from anywhere, with Tailscale on your phone: <span class="address" id="pair-tailnet-address"></span></span></li>"#))
        XCTAssertFalse(page.contains("with this code\""), "a QR code carries a key that works once, not the code")
        XCTAssertTrue(page.contains("$('pair-tailnet').hidden = !p.tailnet || !p.address;"))
    }
}
