import XCTest
@testable import AgentDeck

/// A first GM's preflight from given facts: no login shell, no real claude, no Apple Events, nothing on the network.
/// The routes are driven through the real router, in process, with those looks stood in for.
final class GMPreflightTests: XCTestCase {

    /// Start here's account line goes where it says (FX4), and nothing is Ready while a line is in Hire's way.
    func testStartHereWaitsForATestCopysAccountBeforeReadyToWork() throws {
        let page = try source("Resources/web/setup.html")
        XCTAssertTrue(page.contains("if (action.kind === 'account') return chooseAccount().run();"))
        // Any line still in Hire's way is counted as a step left (or to fix): never Ready, never All set.
        XCTAssertTrue(page.contains("todo: [...(hireAhead ? [] : steps), ...hire.filter((i) => i.todo)],"))
        XCTAssertTrue(page.contains("$('start-now').textContent = !left ? ready : !hireAhead && !fix.length ? `Ready to work · ${left}` : left;"))
        let render = try XCTUnwrap(page.range(of: "function renderGmPreflight() {"))
        XCTAssertNotNil(page[render.upperBound...].prefix(400).range(of: "renderStartNow();"), "the state beside Start here follows the line")
    }

    /// Come to the accounts from Start here's account line (FX7): one sentence on which to pick and what happens, the
    /// accounts that will do marked, and an Add goes straight back to Easy setup, where the line looks again.
    func testComingFromTheAccountLineSaysWhichToAddAndComesBack() throws {
        let page = try source("Resources/web/setup.html")
        XCTAssertTrue(page.contains(#"<p class="accounts-for-gm" id="accounts-for-gm" role="status" hidden></p>"#))
        XCTAssertTrue(page.contains("'Pick any account below that isn\\'t the one you use yourself, and click Add: your General Manager will work on it.'"))
        XCTAssertTrue(page.contains("accountsForGm = testCopy();\n      renderAccounts();\n      go('general', $('step-accounts'));"))
        XCTAssertTrue(page.contains("return accountsForGm && testCopy() && !gmHired && !$('start-gm').hidden && !gmAccount();"))
        XCTAssertTrue(page.contains("textContent: 'Your General Manager can work on this one'"))
        XCTAssertTrue(page.contains("textContent: 'The one you use yourself'"))
        XCTAssertTrue(page.contains("if (forGm && gmAccount()) {"))
        XCTAssertTrue(page.contains("return go('easy', $('first-gm-checks'));"), "back on Easy setup, go() looks again")
        // No word of a Connect button the list doesn't have.
        XCTAssertFalse(page.contains("Connect puts Smolcorp's hooks"))
        XCTAssertTrue(page.contains("$('accounts-hooks').hidden = testCopy();"))
    }

    /// Open the office says where it opened (FX7): its own window on the Mac and Windows, the browser on Linux, beside the button.
    func testOpenTheOfficeSaysWhereItOpened() throws {
        let page = try source("Resources/web/setup.html")
        XCTAssertTrue(page.contains("onMac() ? 'The office opened in its own window.' : 'The office is open: in its own window, or in your browser.'"))
        // R791: one fact counts once (the hooks line in both lists), and the live region is written only when it changes.
        XCTAssertTrue(page.contains("const twice = (c) => c.id === 'hooks' && hire.some((i) => i.id === 'hooks');"))
        XCTAssertTrue(page.contains("own.filter((c) => c.status === 'fix' && !twice(c))"))
        XCTAssertTrue(page.contains("if ($('accounts-for-gm').textContent !== gmHint) $('accounts-for-gm').textContent = gmHint;"))
        XCTAssertTrue(page.contains("const said = officeOpened(button);"))
        XCTAssertTrue(page.contains("button.after(said);"))
    }

    /// The accounts list (FX4): one verb, Add, on the instruction and the buttons; the GM's account marked; a way back to
    /// the hire once one is added; how to sign in to a second account; CLAUDE_CONFIG_DIR only in the small print.
    func testTheAccountsListSaysAddMarksTheGMsAccountAndLeadsBack() throws {
        let page = try source("Resources/web/setup.html")
        let start = try XCTUnwrap(page.range(of: #"<div class="step" id="step-accounts" hidden>"#))
        let end = try XCTUnwrap(page.range(of: #"<div class="step" id="step-launcher""#, range: start.upperBound..<page.endIndex))
        let step = page[start.upperBound..<end.lowerBound]
        let lead = try XCTUnwrap(step.range(of: #"<div class="detail">"#))
        let first = step[lead.upperBound...].prefix { $0 != "<" }
        XCTAssertEqual(first, "Each hire works on one of these Claude accounts. To hire on another, add it here.")
        XCTAssertFalse(first.contains("CLAUDE_CONFIG_DIR"))
        XCTAssertTrue(step.contains(#"<p class="accounts-fine">Each account is a folder of its own (CLAUDE_CONFIG_DIR)."#))
        XCTAssertTrue(step.contains("No second account yet? Sign in to one: paste this in Terminal, then add it when it shows up above."))
        XCTAssertTrue(step.contains(#"<button class="btn primary" id="accounts-back">Back to Easy setup → Hire your General Manager</button>"#))
        XCTAssertFalse(step.contains("Choose"), "the list's verb is Add")
        XCTAssertTrue(page.contains("textContent: 'Your General Manager works on this one'"))
        XCTAssertTrue(page.contains("$('account-signin-line').textContent = `CLAUDE_CONFIG_DIR=~/.claude-${testCopy() ? 'test' : 'work'} claude`;"))
        XCTAssertTrue(page.contains("$('accounts-next').hidden = !accountAdded || gmHired || $('start-gm').hidden;"))
        XCTAssertTrue(page.contains("go('easy', $('start-gm'));"))
        // The lines look again once the picker beside Hire has the new list.
        XCTAssertTrue(page.contains("await load();\n      // Added for the General Manager"))
        XCTAssertTrue(page.contains("return go('easy', $('first-gm-checks'));\n      }\n      checkGmPreflight();"))
    }

    /// Buttons that act on the Mac say so in the page (FX4): Change's folder window, with a path field as the other way,
    /// and Open the office, from both its buttons.
    func testButtonsThatActOnTheMacShowASignOfLifeInThePage() throws {
        let page = try source("Resources/web/setup.html")
        XCTAssertTrue(page.contains(#"<p class="ff-what" id="first-folder-picking" role="status" hidden>A folder window opened on your Mac: pick a folder there. Or type its path here and use it: the window then closes by itself.</p>"#))
        let choose = try XCTUnwrap(page.range(of: "async function chooseFirstFolder() {"))
        let body = page[choose.upperBound...].prefix(1800)
        let said = try XCTUnwrap(body.range(of: "firstFolderPicking = firstFolderTyping = true;\n      renderFirstFolder();"))
        let asked = try XCTUnwrap(body.range(of: "fetch('/setup/hiring/choose?pick=1'"))
        XCTAssertLessThan(said.lowerBound, asked.lowerBound, "said before the window's answer, which waits for a pick")
        // Nothing typed: folded back as it was, without the check made as the window opened (R790).
        XCTAssertNotNil(body.range(of: "if (!firstFolderEdited) {\n          firstFolderTyping = firstFolderChanging = false;\n          firstFolderIssue = null;\n        }"),
                        "a typed path stays; otherwise the step folds back")
        let render = try XCTUnwrap(page.range(of: "function renderFirstFolder() {"))
        XCTAssertNotNil(page[render.upperBound...].prefix(600).range(of: "|| !!firstFolderIssue;"), "an issue left behind would keep the step open")
        XCTAssertTrue(page.contains("$('first-folder-picking').hidden = !firstFolderPicking || !firstFolderTyping;"))
        XCTAssertTrue(page.contains("button.textContent = 'Opening the office…';"))
        XCTAssertTrue(page.contains("$('first-gm-office').onclick = $('start-office').onclick;"))
        XCTAssertFalse(page.contains("startSaid('The office is opening.');"), "said on the button pressed, not under Hire")
    }

    /// Only /bin/sh, on a command made here: nothing is written anywhere.
    private func sh(_ command: String, environment: [String: String] = ["PATH": "/usr/bin:/bin"]) throws -> String {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/sh")
        process.arguments = ["-c", command]
        process.environment = environment
        let out = Pipe()
        process.standardOutput = out
        try process.run()
        let data = out.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        return String(decoding: data, as: UTF8.self).trimmingCharacters(in: .newlines)
    }

    private func source(_ path: String) throws -> String {
        let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        return try String(contentsOf: repo.appendingPathComponent(path), encoding: .utf8)
    }

    /// The page: the checks before the button, the default account chosen, and what to say first after the hire.
    func testThePageShowsTheChecksAndWhatToSayFirst() throws {
        let page = try source("Resources/web/setup.html")
        for id in ["first-gm-checks", "start-gm", "first-gm-recheck", "first-gm-why", "first-gm-next", "first-gm-office", "first-gm-says"] {
            XCTAssertTrue(page.contains(#"id="\#(id)""#), id)
        }
        XCTAssertTrue(page.contains("fetch(`/setup/hire/gm/preflight${account}`"))
        XCTAssertTrue(page.contains("$('start-gm').setAttribute('aria-disabled', String(left.length > 0 || gmHireRunning));"))
        XCTAssertTrue(page.contains("pick.value = profiles.find((p) => p.default).name"))
        // For the guide (T780): no data-ok while checking, then whether nothing greys the hire.
        XCTAssertTrue(page.contains("delete $('first-gm-checks').dataset.ok;"))
        XCTAssertTrue(page.contains("if (gmPreflightAsked && !gmPreflightBusy) box.dataset.ok = String(!left.length);"))
        let checks = try XCTUnwrap(page.range(of: #"id="first-gm-checks""#)), button = try XCTUnwrap(page.range(of: #"id="start-gm""#))
        XCTAssertLessThan(checks.lowerBound, button.lowerBound, "the checks come before the button")
    }

    /// No looks while the lines aren't shown (a GM hired or started by hand) or a hire runs; one hire at a time;
    /// the GMs that had already failed are read once the hire is under way, when the team can be read.
    func testThePageLooksOnlyWhileShownAndHiresOnceAtATime() throws {
        let page = try source("Resources/web/setup.html")
        XCTAssertTrue(page.contains("return gmKnown && !$('start-gm').hidden && !gmHired && !gmHireRunning;"))
        XCTAssertTrue(page.contains("gmPreflightTimer = again && gmPreflightWanted() ? setTimeout(() => checkGmPreflight(), 10000) : 0;"))
        XCTAssertTrue(page.contains("if (!(pressed ? gmKnown && !$('start-gm').hidden && !gmHired : gmPreflightWanted())) return;"))
        // Hire waits for a look already under way, rather than reading the old lines.
        XCTAssertTrue(page.contains("if (!pressed) gmPreflightAgain = true;\n      return pressed ? gmPreflightRunning : undefined;"))
        // No 10 s look while the page is hidden; back in view, it looks.
        XCTAssertTrue(page.contains("(!gmPreflight || !gmPreflight.ready) && !document.hidden;"))
        XCTAssertTrue(page.contains("if (document.hidden) clearTimeout(gmPreflightTimer);"))
        XCTAssertTrue(page.contains("if (gmHireRunning) return startSaid('Your General Manager is being hired. Give it a moment.');"))
        let hire = try XCTUnwrap(page.range(of: "async function hireGm() {"))
        let body = page[hire.upperBound...]
        let post = try XCTUnwrap(body.range(of: "method: 'POST'")), failed = try XCTUnwrap(body.range(of: "let failed = await failedGms();"))
        XCTAssertLessThan(post.lowerBound, failed.lowerBound, "the failed GMs are read after the hire's POST")
        XCTAssertNotNil(body.prefix(3000).range(of: "load();"), "the switches the hire turned on are shown")
        // The team unread when the hire began: the old failures are the GMs started before Hire was pressed.
        XCTAssertTrue(page.contains("if (failedNow && !failed) failed = failedNow.filter((r) => !(r.started >= pressedAt));"))
    }

    /// Never stale (FX2): a folder kept or changed anywhere, a fix pressed, or coming back to Easy setup looks again;
    /// a look asked for during one is made after it; a done folder step is never offered back as Show me.
    func testThePageLooksAgainByItselfAndNeverPointsBackToADoneStep() throws {
        let page = try source("Resources/web/setup.html")
        XCTAssertTrue(page.contains("document.addEventListener('smolcorp:hire-folder', (e) => folderMoved(e.detail && e.detail.folder));"))
        XCTAssertTrue(page.contains("folderMoved(h.folder);"), "a folder changed on Hires and roles, or by the GM")
        XCTAssertTrue(page.contains("if (gmPreflightAgain) {\n      gmPreflightAgain = false;\n      return checkGmPreflight();"))
        XCTAssertTrue(page.contains("if (startAgain) {\n      startAgain = false;\n      loadStartHere();"))
        XCTAssertTrue(page.contains("const back = pane === 'easy' && !!document.querySelector('.pane.current:not([data-pane=\"easy\"])');"))
        XCTAssertTrue(page.contains("if (back) lookAgain();"))
        // A fix pressed on Start here, or General's own buttons: both lists look again.
        let fix = try XCTUnwrap(page.range(of: "async function startFix(fix) {"))
        XCTAssertNotNil(page[fix.upperBound...].prefix(1400).range(of: "checkGmPreflight();"))
        XCTAssertTrue(page.contains("$('accessibility').onclick = () => post('/setup/accessibility').then(lookAgain);"))
        XCTAssertTrue(page.contains("} else if (state.notice) alert(state.notice);\n    lookAgain();"))
        XCTAssertTrue(page.contains("const back = action && action.kind === 'folder' && firstFolderDone();"))
        XCTAssertTrue(page.contains("if (action && !back) row.append(startButton("))
    }

    /// No red on a first screen (FX2): a step not taken yet says To do, neutral; only a real problem is marked !.
    func testAStepNotTakenIsToDoNotRed() throws {
        let page = try source("Resources/web/setup.html")
        XCTAssertTrue(page.contains("const status = item.ok ? 'done' : item.optional ? 'info' : item.todo ? 'todo' : 'fix';"))
        XCTAssertTrue(page.contains("if (status === 'todo') toDo(title);"))
        XCTAssertTrue(page.contains("if (c.status === 'todo') toDo(title);"), "Start here's own steps say it the same way")
        XCTAssertTrue(page.contains("textContent: 'To do'"))
        XCTAssertFalse(page.contains("marked ! above is` : `the ${left.length} lines marked ! above are`} put right."))
    }

    /// With the manager off, the state says whether a GM is hired, so an office that has one isn't looked at
    /// every 10 s; a team that can't be read falls back to the same, and still lets the page look.
    func testThePageReadsWhetherAGMIsHiredFromTheState() throws {
        let page = try source("Resources/web/setup.html")
        let manager = try XCTUnwrap(page.range(of: "function renderManager() {"))
        let body = page[manager.upperBound...].prefix(3500)
        XCTAssertNotNil(body.range(of: "if (!m || !m.enabled) {\n      gmHired = !!state.gmHired;\n      gmKnown = true;"))
        XCTAssertNotNil(body.range(of: "if (!team) return teamUnread();"))
        XCTAssertNotNil(body.range(of: "}).catch(teamUnread);"))
        let unread = try XCTUnwrap(body.range(of: "const teamUnread = () => {"))
        XCTAssertTrue(body[unread.upperBound...].prefix(200).contains("gmHired = !!state.gmHired;\n      gmKnown = true;"))
    }
}
