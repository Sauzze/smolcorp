import XCTest
@testable import AgentDeck

/// The setup page's Manager panel (T311): each control is there and calls the route it is for.
final class ManagerPanelPageTests: XCTestCase {
    private func page() throws -> String {
        try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .appendingPathComponent("../../Resources/web/setup.html"), encoding: .utf8)
    }

    /// The locked note: on a Mac it names the menu bar's item as StatusMenu has it and not `setup --open`.
    func testTheLockedNoteNamesTheMacsMenuItem() throws {
        let page = try page()
        let menu = try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .appendingPathComponent("../../Sources/AgentDeck/GoShell.swift"), encoding: .utf8)
        XCTAssertTrue(menu.contains(#""Open Settings in Browser""#))
        XCTAssertTrue(page.contains("This page opens from Smolcorp's menu bar: choose Open Settings in Browser."))
        // Linux and Windows keep theirs.
        XCTAssertTrue(page.contains("With no tray, use the link Smolcorp prints as it starts; "))
        XCTAssertFalse(page.contains("setup --open"))
        let mac = try XCTUnwrap(page.range(of: "choose Open Settings in Browser"))
        let linux = try XCTUnwrap(page.range(of: "Open Setup… in a browser"))
        XCTAssertLessThan(mac.lowerBound, linux.lowerBound, "the Mac's words come in their own branch")
        // Which words: the app's own header decides, never the browser.
        XCTAssertTrue(page.contains("locked(res.headers.get('X-Smolcorp-App'), res.headers.get('X-Smolcorp-Session-Ended'))"))
        XCTAssertTrue(page.contains("app === 'mac'"))
        XCTAssertFalse(page.contains("navigator.userAgent"))
    }

    /// A guard group that is off shows what hires can now do, without hovering; one that is on shows nothing.
    func testAGuardGroupThatIsOffWarnsWhatHiresCanDo() throws {
        let page = try page()
        let branch = try XCTUnwrap(page.range(of: "if (!g.on) {"), "the warning is built only for a group that is off")
        let text = String(page[branch.lowerBound...].prefix(700))
        XCTAssertTrue(text.contains("guard-warning"))
        XCTAssertTrue(text.contains("Off: hires will be able to ${g.allows"))
        XCTAssertTrue(text.contains("Neither the guard nor Claude's deny rules will stop it."))
        XCTAssertTrue(text.contains("text.append(warning)"))
        XCTAssertFalse(text.contains("title=") || text.contains("setAttribute('title'"), "visible without hovering")
        // The row is rebuilt on every change, so switching back on removes the warning with it.
        XCTAssertTrue(page.contains("$('guard-groups').replaceChildren(...guard.groups.map"))
    }

    func testTheGMCanBeFiredRotatedAndAllowedByHand() throws {
        let page = try page()
        XCTAssertTrue(page.contains(#"id="gm-fire""#))
        XCTAssertTrue(page.contains("Fire the General Manager"))
        XCTAssertTrue(page.contains("fetch('/setup/gm/fire'"))
        XCTAssertTrue(page.contains(#"id="gm-rotate""#))
        XCTAssertTrue(page.contains("New GM token"))
        XCTAssertTrue(page.contains("/setup/gm/token/rotate"))
        XCTAssertTrue(page.contains("Fire the General Manager and hire it again to give it the new one."))
        XCTAssertTrue(page.contains(#"id="gm-byhand""#))
        XCTAssertTrue(page.contains("/setup/gm/by-hand?enabled="))
        XCTAssertTrue(page.contains("The token goes to that General Manager alone, and no file holds it."))
        // Refusals are said in place.
        XCTAssertTrue(page.contains(#"id="gm-problem""#))
    }

    func testTheHireSheetOffersWorktreeModelNicknameAndTheKeyOnlyWhenThereIsOne() throws {
        let page = try page()
        for id in ["hs-worktree", "hs-model", "hs-nickname", "hs-apikey-row", "hs-apikey"] {
            XCTAssertTrue(page.contains(#"id="\#(id)""#), id)
        }
        XCTAssertTrue(page.contains("state.hiring && state.hiring.apiKey"))
        XCTAssertTrue(page.contains("key.available"))
        XCTAssertTrue(page.contains("Billed to your API key. The hire can read this key."))
        XCTAssertTrue(page.contains("worktree: $('hs-worktree').checked"))
        XCTAssertTrue(page.contains("{ model: $('hs-model').value.trim() }"))
        XCTAssertTrue(page.contains("{ nickname: $('hs-nickname').value.trim() }"))
        XCTAssertTrue(page.contains("{ apiKey: true }"))
    }

    func testTheTeamListFiresWithForceAndRemovingTheWorktree() throws {
        let page = try page()
        XCTAssertTrue(page.contains(#"id="team-list""#))
        XCTAssertTrue(page.contains("{ force: true }"))
        XCTAssertTrue(page.contains("{ removeWorktree: true }"))
        XCTAssertTrue(page.contains("Whatever it has not saved is lost"))
        XCTAssertTrue(page.contains("It is kept if it holds changes."))
        // Only a hire with a worktree is offered removing it.
        XCTAssertTrue(page.contains("if (r.worktree) {"))
    }

    func testEachBotHasAMailboxFolderFieldThatCallsItsRoute() throws {
        let page = try page()
        XCTAssertTrue(page.contains("function mailboxRow(bot)"))
        XCTAssertTrue(page.contains("fetch('/setup/bots/mailbox'"))
        XCTAssertTrue(page.contains("JSON.stringify({ id: bot.id, folder: input.value.trim() })"))
        XCTAssertTrue(page.contains("rows.push(mailboxRow(bot));"))
    }

    func testThePageNeverUsesAnAlertForTheseControls() throws {
        let page = try page()
        let start = try XCTUnwrap(page.range(of: "function renderManagerTools"))
        let end = try XCTUnwrap(page.range(of: "/** \"Which screen holds the team?\""))
        XCTAssertFalse(page[start.lowerBound..<end.lowerBound].contains("alert("))
        XCTAssertFalse(page[start.lowerBound..<end.lowerBound].contains("confirm("))
    }

    /// The page refreshes every 1.5 s: it must not put an armed "New GM token" back before the second click.
    func testTheRefreshDoesNotDisarmTheNewTokenButton() throws {
        let page = try page()
        let start = try XCTUnwrap(page.range(of: "function renderGmToken"))
        let end = try XCTUnwrap(page.range(of: "function resetRotate"))
        let render = page[start.lowerBound..<end.lowerBound]
        XCTAssertTrue(render.contains("if (!$('gm-rotate').dataset.sure) resetRotate();"))
        XCTAssertFalse(render.replacingOccurrences(of: "if (!$('gm-rotate').dataset.sure) resetRotate();", with: "").contains("resetRotate();"))
    }

    /// A GM started by hand is named as such, and the hired GM's text and Fire button stay.
    func testSettingsNamesAGMStartedByHand() throws {
        let page = try page()
        let start = try XCTUnwrap(page.range(of: "function renderManagerTools"))
        let end = try XCTUnwrap(page.range(of: "function resetRotate"))
        let render = page[start.lowerBound..<end.lowerBound]
        XCTAssertTrue(render.contains("const byHand = !gm && (state.manager.gmStartedByHand || open.some((r) => r.role === 'gm'));"))
        XCTAssertTrue(render.contains("byHand ? 'Your General Manager was started by hand.'"))
        XCTAssertTrue(render.contains("'Your General Manager is hired.'"))
        // The same words as the Manager card's line above it (fresh eyes, D).
        XCTAssertTrue(render.contains("'No General Manager yet: use Hire your General Manager on Easy setup.'"))
        XCTAssertTrue(render.contains("$('gm-fire').hidden = !gm;"))
    }

    /// A GM started by hand: the Manager card says so and does not count it, and Easy setup's hire button gives way to the same line.
    func testManagerCardAndEasySetupRecogniseAGMStartedByHand() throws {
        let page = try page()
        let start = try XCTUnwrap(page.range(of: "function renderManager()"))
        let end = try XCTUnwrap(page.range(of: "function tell("))
        let card = page[start.lowerBound..<end.lowerBound]
        XCTAssertTrue(card.contains("const hired = rows.some((r) => r.role === 'gm' && r.registered);"))
        XCTAssertTrue(card.contains("const byHand = !hired && (rows.some((r) => r.role === 'gm') || !!state.manager.gmStartedByHand);"))
        XCTAssertTrue(card.contains("const others = rows.filter((r) => r.role !== 'gm').length;"))
        XCTAssertTrue(card.contains("hired ? 'Your General Manager is hired.' : byHand ? 'Your General Manager was started by hand.' : 'No General Manager yet: use Hire your General Manager on Easy setup.'"))
        XCTAssertTrue(card.contains("showStartGM(byHand);"))
        XCTAssertTrue(card.contains("$('start-gm').hidden = byHand;"))
        XCTAssertTrue(card.contains("$('start-gm-byhand').hidden = !byHand;"))
        XCTAssertTrue(page.contains(#"<span class="text" id="start-gm-byhand" hidden>Your General Manager was started by hand.</span>"#))
        // The account choice goes with the button.
        XCTAssertTrue(page.contains("|| $('start-gm').hidden;"))
        // No Fire button for a GM that was never hired.
        XCTAssertTrue(page.contains("$('gm-fire').hidden = !gm;"))
    }
}
