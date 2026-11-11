import XCTest
@testable import AgentDeck

/// A first GM's preflight from given facts: no login shell, no real claude, no Apple Events, nothing on the network.
/// The routes are driven through the real router, in process, with those looks stood in for.
final class GMPreflightTests: XCTestCase {
    private let home = "/Users/someone"
    private var saved: (folder: String, manager: Bool, hiring: Bool, roles: [Role])!
    private var savedLook: ((inout GMPreflight.Facts, ClaudeProfile?, String?) -> Void)!
    private var scratch: URL!

    override func setUpWithError() throws {
        let config = Config.shared
        saved = (config.hireFolder, config.managerEnabled, config.hiringEnabled, config.roles)
        savedLook = GMPreflight.look
        scratch = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true).resolvingSymlinksInPath()
        try FileManager.default.createDirectory(at: scratch, withIntermediateDirectories: true)
    }

    override func tearDown() {
        let config = Config.shared
        config.hireFolder = saved.folder
        config.managerEnabled = saved.manager
        config.hiringEnabled = saved.hiring
        config.roles = saved.roles
        config.save()
        GMPreflight.look = savedLook
        try? FileManager.default.removeItem(at: scratch)
    }

    /// Everything there: the folder, Claude Code on the login shell's PATH and signed in, reporting, Terminal allowed.
    private func ready() -> GMPreflight.Facts {
        var f = GMPreflight.Facts()
        f.home = home
        f.folder = "\(home)/Projects"
        f.folderExists = true
        f.claudeAt = "\(home)/.local/bin/claude"
        f.claudeOnLoginPath = true
        f.signIn = .yes
        f.automation = .allowed
        return f
    }

    private func line(_ items: [GMPreflight.Item], _ id: String) throws -> GMPreflight.Item {
        try XCTUnwrap(items.first { $0.id == id }, "no \(id) line in \(items.map(\.id))")
    }

    private func isReady(_ items: [GMPreflight.Item]) -> Bool { GMPreflight.report(items)["ready"] as? Bool == true }

    func testEverythingThereIsReady() {
        let items = GMPreflight.items(ready())
        XCTAssertEqual(items.map(\.id), ["folder", "claude", "signin", "hooks", "terminal"])
        XCTAssertTrue(items.allSatisfy { $0.ok && $0.fix.isEmpty && $0.action == nil }, "\(items)")
        XCTAssertEqual(items.first?.title, "Your hires work in ~/Projects.")
        XCTAssertTrue(isReady(items))
    }

    func testNoFolderPointsToTheFolderStep() throws {
        var f = ready()
        f.folder = ""
        f.folderExists = false
        let folder = try line(GMPreflight.items(f), "folder")
        XCTAssertFalse(folder.ok)
        XCTAssertEqual(folder.title, "There's no folder for your hires yet.")
        XCTAssertEqual(folder.action, GMPreflight.Action(kind: "folder", label: "Show me"))
        XCTAssertFalse(isReady(GMPreflight.items(f)))
        f.folder = "\(home)/Gone"
        XCTAssertTrue(try line(GMPreflight.items(f), "folder").title.contains("isn't there any more: ~/Gone"))
    }

    /// A step not taken yet (no folder, Claude Code to install, Connect to press) is To do on the page, never red;
    /// a real problem (a folder gone, Terminal refused) isn't. Linux/Windows says the same (gmpreflight_test.go).
    func testAStepToTakeIsToDoAndAProblemIsNot() throws {
        var f = ready()
        f.folder = ""
        XCTAssertTrue(try line(GMPreflight.items(f), "folder").todo, "no folder yet")
        XCTAssertEqual(try line(GMPreflight.items(f), "folder").jsonObject["todo"] as? Bool, true)
        f.folder = "\(home)/Gone"
        XCTAssertFalse(try line(GMPreflight.items(f), "folder").todo, "a folder that's gone")
        f = ready()
        f.hooks = "Claude Code isn't connected."
        XCTAssertTrue(try line(GMPreflight.items(f), "hooks").todo)
        f.automation = .denied
        XCTAssertFalse(try line(GMPreflight.items(f), "terminal").todo, "Terminal refused")
        f.claudeAt = nil
        f.claudeOnLoginPath = false
        XCTAssertTrue(try line(GMPreflight.items(f), "claude").todo)
        XCTAssertTrue(GMPreflight.items(ready()).allSatisfy { !$0.todo })
    }

    /// A test copy hears only from its own hires' hook: its line says so, as Start here's agents line does, never "connect".
    func testATestCopySaysItsHiresReportThroughItsOwnHook() throws {
        var f = ready()
        f.testCopy = true
        f.hooks = "Claude Code isn't connected."
        let hooks = try line(GMPreflight.items(f), "hooks")
        XCTAssertTrue(hooks.ok)
        XCTAssertEqual(hooks.title, "Your hires report to this test copy through its own hook.")
        XCTAssertNil(hooks.action)
    }

    /// One row, one state (FX7): a test copy's agents line was an empty circle on General and Start here, while the hire's
    /// line under it was ticked, as each decided the test copy apart. Now both say it from one sentence, both ticked,
    /// and General shows the test copy's words with no Disconnect.
    func testATestCopysAgentsLineIsTickedEverywhereFromOneSource() throws {
        var f = ready()
        f.testCopy = true
        let hire = try line(GMPreflight.items(f), "hooks")
        let check = SelfTest.hooksCheck(found: [], connected: [], scriptThere: true, testCopy: true)
        XCTAssertTrue(hire.ok)
        XCTAssertEqual(check.status, .ok)
        XCTAssertEqual(hire.title, SelfTest.testCopyHires)
        XCTAssertTrue(check.detail.hasPrefix(SelfTest.testCopyHires))
        XCTAssertFalse(check.detail.contains("scripts/"), "no file names on a tester's screen")
        XCTAssertTrue(try source("Sources/AgentDeck/GMPreflight.swift").contains("title: SelfTest.testCopyHires"))
        let page = try source("Resources/web/setup.html")
        XCTAssertTrue(page.contains("detail.textContent = id === 'hooks' && ok && detail.dataset.done && !testCopy() ? detail.dataset.done : c.detail;"))
        XCTAssertTrue(page.contains("el.hidden = !ok || (id === 'hooks' && testCopy());"))
    }

    /// A test copy never hires on your default Claude account (FX4): Start here says so before Hire, To do, with how
    /// and a button to it; ticked with the account's name once another is picked. The real app shows no such line.
    func testATestCopySaysItsGMNeedsAnAccountOfItsOwnBeforeHire() throws {
        var f = ready()
        f.account = "default"
        XCTAssertFalse(GMPreflight.items(f).contains { $0.id == "account" }, "the real app hires on the default account")
        f.testCopy = true
        var items = GMPreflight.items(f)
        XCTAssertEqual(items.map(\.id), ["folder", "claude", "signin", "hooks", "account", "terminal"])
        let add = try line(items, "account")
        XCTAssertFalse(add.ok)
        XCTAssertTrue(add.todo, "a step to take, never red")
        XCTAssertEqual(add.title, "Your General Manager needs a Claude account of its own (this is a test copy).")
        XCTAssertEqual(add.fix, "Add one under Claude accounts: sign in to a second account there, then add it.")
        XCTAssertEqual(add.action, GMPreflight.Action(kind: "account", label: "Add an account"))
        XCTAssertFalse(isReady(items), "Hire waits for it")
        // Another account listed, the default picked (or none picked among several): choose it beside Hire.
        f.otherAccounts = true
        for account in ["default", nil] as [String?] {
            f.account = account
            let choose = try line(GMPreflight.items(f), "account")
            XCTAssertEqual(choose.fix, GMPreflight.chooseAccountFix)
            XCTAssertEqual(choose.action, GMPreflight.Action(kind: "account", label: "Choose an account"))
        }
        f.account = "test"
        items = GMPreflight.items(f)
        let ok = try line(items, "account")
        XCTAssertTrue(ok.ok)
        XCTAssertEqual(ok.title, "Your General Manager works on the Claude account test.")
        XCTAssertTrue(isReady(items))
        // Said even with Claude Code still to install.
        f.account = "default"
        f.claudeAt = nil
        f.claudeOnLoginPath = false
        XCTAssertTrue(GMPreflight.items(f).contains { $0.id == "account" && $0.todo })
        // The hire's refusal says the same thing to do, so the line and the refusal agree.
        let home = ClaudeProfiles.defaultProfile(home: URL(fileURLWithPath: self.home))
        XCTAssertTrue(Deck.testCopyAccountRefusal(account: home, otherAccounts: false, testCopy: true)?.message.hasSuffix(add.fix) ?? false)
    }

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

    /// One count (FX7): the header's "N steps left", the To do badges and the line under Hire all come from startLines.
    /// While the GM is to hire, Start here's own steps (keyboard, phone, open at login) say Later and aren't counted;
    /// the line under Hire names what greys it instead of counting lines.
    func testStartHereCountsItsLinesOnceAndNamesWhatGreysHire() throws {
        let page = try source("Resources/web/setup.html")
        XCTAssertEqual(page.components(separatedBy: "startLines()").count - 1, 3, "defined once, read by the header and the line under Hire")
        XCTAssertTrue(page.contains("const hire = hireAhead ? gmPreflightLeft() : [];"))
        XCTAssertTrue(page.contains("later: hireAhead ? steps : [],"))
        XCTAssertTrue(page.contains("row.querySelector('.to-do').textContent = hireAhead ? 'Later' : 'To do';"))
        XCTAssertTrue(page.contains("const ready = hireAhead ? `Ready to hire${later.length ? ` · ${later.length} for later` : ''}` : 'All set';"))
        // The badges are set once the rows are drawn, not before.
        let draw = try XCTUnwrap(page.range(of: "function renderStartHere(report) {"))
        let body = page[draw.upperBound...].prefix(2600)
        let rows = try XCTUnwrap(body.range(of: "$('start-checks').replaceChildren("))
        let count = try XCTUnwrap(body.range(of: "renderStartNow();"))
        XCTAssertLessThan(rows.lowerBound, count.lowerBound)
        // Named, from the lines in Hire's way.
        XCTAssertTrue(page.contains(": left.length ? hireGreyedUntil(startLines().hire) : '';"))
        XCTAssertTrue(page.contains("return `Hire your General Manager is greyed until you've ${list}.`;"))
        for words in ["'picked the hires folder'", "'added a Claude account'", "'chosen a Claude account'", "'installed Claude Code'", "'pressed Connect'"] {
            XCTAssertTrue(page.contains(words), words)
        }
        // The account line's button words, which say which of add and choose it is, are the app's own.
        var f = ready()
        f.testCopy = true
        f.account = ClaudeProfiles.defaultName
        XCTAssertEqual(try line(GMPreflight.items(f), "account").action?.label, "Add an account")
        f.otherAccounts = true
        XCTAssertEqual(try line(GMPreflight.items(f), "account").action?.label, "Choose an account")
        XCTAssertTrue(page.contains("i.action.label === 'Choose an account' ? 'chosen a Claude account' : 'added a Claude account'"))
        // Start here's own steps are none of the hire's: the hire's needs are its own lines.
        for c in [SelfTest.accessibilityCheck(false), SelfTest.phoneCheck(0), SelfTest.loginCheck(false, testCopy: false)] {
            XCTAssertEqual(c.status, .todo, c.id)
        }
        XCTAssertTrue(page.contains(".start-here .step.todo.later .mark"))
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

    /// A test copy's GM needs an account of its own, so a test copy can add one (FX7): kept in its own settings,
    /// with no account's hooks touched (it used to refuse every Add, 409, so the account line's way out was a dead end).
    func testATestCopyAddsAnAccountWithoutTouchingItsHooks() throws {
        let config = Config.shared
        let before = config.claudeProfiles
        defer {
            config.claudeProfiles = before
            config.save()
        }
        let dir = scratch.appendingPathComponent(".claude-fx7test", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let settings = dir.appendingPathComponent("settings.json")
        try "{}".write(to: settings, atomically: true, encoding: .utf8)
        let router = DeckRouter(deck: quietDeck())
        let body = try JSONSerialization.data(withJSONObject: ["profiles": [["name": "fx7test", "dir": dir.path]]])
        let answered = router.setClaudeAccounts(body, testCopy: true)
        XCTAssertEqual(answered.status, 200, text(answered))
        XCTAssertTrue(config.claudeProfiles.contains { $0.name == "fx7test" && $0.dir == dir.path })
        XCTAssertEqual(try String(contentsOf: settings, encoding: .utf8), "{}", "no hooks written into the account")
        // Taken out again: still nothing written.
        XCTAssertEqual(router.setClaudeAccounts(Data(#"{"profiles":[]}"#.utf8), testCopy: true).status, 200)
        XCTAssertFalse(config.claudeProfiles.contains { $0.name == "fx7test" })
        XCTAssertEqual(try String(contentsOf: settings, encoding: .utf8), "{}")
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

    /// Start here and General draw the keyboard, agents and login lines from the same checks: one name, one button.
    func testStartHereAndGeneralShareTheirWords() throws {
        let keys = SelfTest.accessibilityCheck(false)
        XCTAssertEqual(keys.title, "Smolcorp can use your keyboard")
        XCTAssertEqual(keys.label, "Allow")
        XCTAssertEqual(SelfTest.accessibilityCheck(true).title, keys.title)
        XCTAssertEqual(SelfTest.hooksCheck(found: ["Claude Code"], connected: [], scriptThere: true, testCopy: false).label, "Connect")
        let page = try source("Resources/web/setup.html")
        XCTAssertFalse(page.contains("Your phone's keys reach your agents"))
        XCTAssertFalse(page.contains("Allow Smolcorp to use your keyboard"))
        XCTAssertTrue(page.contains(#"<div class="title">Smolcorp can use your keyboard</div>"#))
        XCTAssertTrue(page.contains("const sameAsStartHere = { hooks: 'step-hooks', accessibility: 'step-accessibility', login: 'step-login' };"))
    }

    func testClaudeCodeMissingGivesItsInstaller() throws {
        var f = ready()
        f.claudeAt = nil
        f.claudeOnLoginPath = false
        let items = GMPreflight.items(f)
        let claude = try line(items, "claude")
        XCTAssertFalse(claude.ok)
        XCTAssertEqual(claude.title, "Claude Code isn't installed on this Mac.")
        XCTAssertEqual(claude.action, GMPreflight.Action(kind: "copy", label: "Copy", command: GMPreflight.installCommand))
        XCTAssertFalse(items.contains { $0.id == "signin" || $0.id == "hooks" }, "nothing asked of a Claude Code that isn't there")
        XCTAssertFalse(isReady(items))
    }

    /// Found where the app looks, not where your login shell (and so the hire) does: the line that puts it right.
    func testClaudeCodeOffTheLoginPathGivesTheLineToAdd() throws {
        var f = ready()
        f.claudeOnLoginPath = false
        f.shell = "/bin/zsh"
        let claude = try line(GMPreflight.items(f), "claude")
        XCTAssertFalse(claude.ok)
        XCTAssertTrue(claude.title.contains("~/.local/bin"), claude.title)
        XCTAssertEqual(claude.action?.command, #"echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zprofile"#)
        XCTAssertEqual(GMPreflight.pathLine(shell: "/opt/homebrew/bin/fish", folder: "\(home)/.local/bin", home: home), "fish_add_path ~/.local/bin")
        // A login shell that didn't say isn't taken for one that doesn't find it.
        f.claudeOnLoginPath = nil
        XCTAssertTrue(try line(GMPreflight.items(f), "claude").ok)
    }

    /// The native installer's ~/.local/bin/claude is a link into a versions folder: the line names the link's
    /// folder, which holds a `claude` once it's on the PATH, never the versions folder, which holds none.
    func testClaudeCodeLinkedIntoAVersionsFolderNamesTheLinksFolder() throws {
        let versions = scratch.appendingPathComponent("share/claude/versions"), bin = scratch.appendingPathComponent("bin")
        for folder in [versions, bin] { try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true) }
        let program = versions.appendingPathComponent("2.1.319")
        try "#!/bin/sh\n".write(to: program, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: program.path)
        try FileManager.default.createSymbolicLink(at: bin.appendingPathComponent("claude"), withDestinationURL: program)

        let found = try XCTUnwrap(OpenCodeInstall.place(named: "claude", in: [bin.path]))
        XCTAssertEqual(found.path, bin.appendingPathComponent("claude").path, "where it was found, the link left as it is")
        XCTAssertEqual(found.resolvingSymlinksInPath().path, program.path, "what program(named:) gives, for running it")
        var f = ready()
        f.claudeAt = found.path
        f.claudeOnLoginPath = false
        let claude = try line(GMPreflight.items(f), "claude")
        XCTAssertEqual(claude.title, "Claude Code is in \(bin.path), where Terminal doesn't look for it.")
        XCTAssertEqual(claude.action?.command, "echo 'export PATH=\"\(bin.path):$PATH\"' >> ~/.zprofile")
        XCTAssertFalse(claude.title.contains("versions") || claude.action?.command?.contains("versions") == true)
        // The preflight's own look finds it the same way: where it was found, never program(named:)'s resolved path.
        let preflight = try source("Sources/AgentDeck/GMPreflight.swift")
        let start = try XCTUnwrap(preflight.range(of: "static var look:"))
        let end = try XCTUnwrap(preflight.range(of: "\n    }\n", range: start.upperBound..<preflight.endIndex))
        let look = preflight[start.upperBound..<end.lowerBound]
        XCTAssertTrue(look.contains(#"OpenCodeInstall.place(named: "claude")"#), String(look))
        XCTAssertFalse(look.contains(#"program(named: "claude")"#), String(look))
    }

    /// A folder with a quote, a $, a backquote or a space: the line still writes that folder, whole.
    func testThePathLineQuotesAnOddFolder() throws {
        let odd = "\(home)/Bob's \"apps\"/$bin`x`"
        let command = GMPreflight.pathLine(shell: "/bin/zsh", folder: odd, home: home)
        XCTAssertEqual(command, #"echo 'export PATH="$HOME/Bob'\''s \"apps\"/\$bin\`x\`:$PATH"' >> ~/.zprofile"#)
        // What the echo writes, read back by a shell as a profile would: the folder, then the PATH there was.
        let echo = try XCTUnwrap(command.range(of: " >> ~/.zprofile")).lowerBound
        let written = try sh(String(command[..<echo]))
        XCTAssertEqual(try sh(written + #"; printf %s "$PATH""#, environment: ["HOME": home, "PATH": "/usr/bin:/bin"]), "\(odd):/usr/bin:/bin")
        XCTAssertEqual(GMPreflight.pathLine(shell: "/opt/homebrew/bin/fish", folder: "\(home)/Bob's apps", home: home),
                       #"fish_add_path '/Users/someone/Bob\'s apps'"#)
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

    /// Signed out is said, never in the way: an API key or a cloud provider may work where the check can't see it.
    func testSignedOutIsSaidButNeverInTheWay() throws {
        var f = ready()
        f.signIn = .no
        let signin = try line(GMPreflight.items(f), "signin")
        XCTAssertFalse(signin.ok)
        XCTAssertTrue(signin.optional)
        XCTAssertEqual(signin.action?.command, "claude")
        XCTAssertTrue(signin.fix.hasPrefix("Hire anyway: its window asks you to sign in."), signin.fix)
        XCTAssertTrue(signin.fix.contains("/login"))
        XCTAssertTrue(isReady(GMPreflight.items(f)))
        // Couldn't tell: said, not in the way.
        f.signIn = .unknown
        let unknown = try line(GMPreflight.items(f), "signin")
        XCTAssertTrue(unknown.optional)
        XCTAssertTrue(isReady(GMPreflight.items(f)))
    }

    func testSignInIsWhatClaudeCodeSays() {
        XCTAssertEqual(GMPreflight.signIn(status: Data(#"{"loggedIn":true,"authMethod":"claude.ai"}"#.utf8)), .yes)
        XCTAssertEqual(GMPreflight.signIn(status: Data(#"{"loggedIn":false}"#.utf8)), .no)
        XCTAssertEqual(GMPreflight.signIn(status: Data("{}".utf8)), .unknown)
        XCTAssertEqual(GMPreflight.signIn(status: Data("error: unknown command 'auth'".utf8)), .unknown)
        XCTAssertEqual(GMPreflight.signIn(status: Data()), .unknown)
        // Asked through your login shell, whose files may print first.
        XCTAssertEqual(GMPreflight.signIn(status: Data("Welcome back {you}\n{\"loggedIn\":true}\n".utf8)), .yes)
        XCTAssertEqual(GMPreflight.signIn(status: Data("motd\n{\n  \"loggedIn\": false\n}\n".utf8)), .no)
    }

    /// Asked as the hire starts Claude Code: your login shell, the account's config folder set (or unset for
    /// ~/.claude's), and Smolcorp's environment with keys and cloud providers kept, as Linux/Windows asks it.
    func testSignInIsAskedAsTheHireStartsIt() {
        XCTAssertEqual(GMPreflight.signInArguments(program: "/x/bin/claude", profile: nil),
                       ["-lic", "exec 'env' '-u' 'CLAUDE_CONFIG_DIR' '/x/bin/claude' 'auth' 'status' '--json'"])
        XCTAssertEqual(GMPreflight.signInArguments(program: "/x/bin/claude", profile: ClaudeProfile(name: ClaudeProfiles.defaultName, dir: "\(home)/.claude")),
                       ["-lic", "exec 'env' '-u' 'CLAUDE_CONFIG_DIR' '/x/bin/claude' 'auth' 'status' '--json'"])
        XCTAssertEqual(GMPreflight.signInArguments(program: "/x/bin/claude", profile: ClaudeProfile(name: "work", dir: "\(home)/Bob's .claude")),
                       ["-lic", "exec 'env' 'CLAUDE_CONFIG_DIR=\(home)/Bob'\\''s .claude' '/x/bin/claude' 'auth' 'status' '--json'"])
        let given = ["ANTHROPIC_API_KEY": "k-fake", "CLAUDE_CODE_USE_BEDROCK": "1", "PATH": "/usr/bin", "SMOLCORP_SESSION": "s1", "TMUX": "t", "SMOLCORP_SUMMARY": "1"]
        XCTAssertEqual(GMPreflight.hireEnvironment(given), ["ANTHROPIC_API_KEY": "k-fake", "CLAUDE_CODE_USE_BEDROCK": "1", "PATH": "/usr/bin"])
    }

    func testHooksNotReportingOffersConnect() throws {
        var f = ready()
        f.hooks = "Connect your agents first"
        let hooks = try line(GMPreflight.items(f), "hooks")
        XCTAssertFalse(hooks.ok)
        XCTAssertEqual(hooks.action, GMPreflight.Action(kind: "fix", label: "Connect", fix: SelfTest.repair))
        XCTAssertFalse(isReady(GMPreflight.items(f)))
    }

    func testTerminalNotAllowedOpensAutomation() throws {
        var f = ready()
        f.terminal = "iTerm2"
        f.automation = .denied
        let terminal = try line(GMPreflight.items(f), "terminal")
        XCTAssertFalse(terminal.ok)
        XCTAssertTrue(terminal.fix.contains("Automation: turn on iTerm2 under Smolcorp"), terminal.fix)
        XCTAssertEqual(terminal.action, GMPreflight.Action(kind: "fix", label: "Open System Settings", fix: GMPreflight.automationFix))
        XCTAssertFalse(isReady(GMPreflight.items(f)))
        // Not asked yet, or Terminal not running: macOS asks at the hire.
        f.automation = .unknown
        XCTAssertTrue(try line(GMPreflight.items(f), "terminal").ok)
    }

    func testTmuxMissingIsSaidWithoutStoppingTheHire() throws {
        var f = ready()
        XCTAssertFalse(GMPreflight.items(f).contains { $0.id == "tmux" }, "no tmux line without hires in tmux")
        f.tmuxWanted = true
        let tmux = try line(GMPreflight.items(f), "tmux")
        XCTAssertFalse(tmux.ok)
        XCTAssertTrue(tmux.optional)
        XCTAssertEqual(tmux.action?.command, "brew install tmux")
        XCTAssertTrue(isReady(GMPreflight.items(f)))
        f.tmux = true
        XCTAssertTrue(try line(GMPreflight.items(f), "tmux").ok)
    }

    /// The JSON the Linux/Windows app answers too (desktop/gmpreflight.go): the same keys on every line.
    func testTheReportsJSON() throws {
        var f = ready()
        f.folder = ""
        let report = GMPreflight.report(GMPreflight.items(f))
        XCTAssertEqual(report["ready"] as? Bool, false)
        let items = try XCTUnwrap(report["items"] as? [[String: Any]])
        for item in items { XCTAssertEqual(Set(item.keys), ["id", "ok", "title", "fix", "action", "optional", "todo"]) }
        XCTAssertEqual((items[0]["action"] as? [String: Any])?["kind"] as? String, "folder")
        XCTAssertTrue(items[1]["action"] is NSNull)
        XCTAssertNotNil(try JSONSerialization.data(withJSONObject: report))
    }

    private func source(_ path: String) throws -> String {
        let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        return try String(contentsOf: repo.appendingPathComponent(path), encoding: .utf8)
    }

    // MARK: Through the router, in process

    private func quietDeck() -> Deck {
        let deck = Deck(quiet: true)
        deck.layoutConfig = Config()
        return deck
    }

    /// A request from the setup page (its session cookie), or from a tool with the setup-key file.
    private func request(_ deck: Deck, _ method: String, _ path: String, query: [String: String] = [:], tool: Bool = false) throws -> Request {
        var headers = [DeckRouter.setupHeader: "1", "host": "127.0.0.1:7710"]
        if tool {
            headers[DeckRouter.setupKeyHeader] = deck.access.setupKey
        } else {
            let sessions = deck.access.setupSessions
            headers["cookie"] = "\(DeckRouter.setupCookie)=\(try XCTUnwrap(sessions.open(link: sessions.newLink())))"
        }
        return Request.addressed(method: method, path: path, query: query, headers: headers, body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")
    }

    private func answer(_ router: DeckRouter, _ request: Request) throws -> Response {
        var got: Response?
        let answered = expectation(description: request.path)
        router.handle(request) { reply in
            if case .response(let response) = reply { got = response }
            answered.fulfill()
        }
        wait(for: [answered], timeout: 10)
        return try XCTUnwrap(got)
    }

    private func text(_ response: Response) -> String { String(decoding: response.body, as: UTF8.self) }

    /// The page's alone, open with the manager off (as on a first run), and the Linux/Windows app's JSON.
    func testThePreflightRouteAnswersThePageAloneWithItsJSON() throws {
        let path = "/setup/hire/gm/preflight"
        XCTAssertNil(ManagerSwitch.refusal("GET", path, enabled: false))
        XCTAssertEqual(SetupRoutes.row("GET", path)?.callers, [.page])
        XCTAssertTrue(try source("Sources/AgentDeck/SelfTest.swift").contains("case GMPreflight.automationFix:"))
        GMPreflight.look = { f, _, _ in
            f.claudeAt = "/x/bin/claude"
            f.claudeOnLoginPath = true
            f.signIn = .no
            f.automation = .allowed
        }
        let config = Config.shared
        config.hireFolder = ""
        config.managerEnabled = false
        let deck = quietDeck()
        let router = DeckRouter(deck: deck)

        let answered = try answer(router, try request(deck, "GET", path))
        XCTAssertEqual(answered.status, 200, text(answered))
        let report = try XCTUnwrap(JSONSerialization.jsonObject(with: answered.body) as? [String: Any])
        XCTAssertEqual(report["ready"] as? Bool, false, "no folder for your hires")
        let items = try XCTUnwrap(report["items"] as? [[String: Any]])
        XCTAssertEqual(Array(items.map { $0["id"] as? String }.prefix(3)), ["folder", "claude", "signin"])
        for item in items { XCTAssertEqual(Set(item.keys), ["id", "ok", "title", "fix", "action", "optional", "todo"]) }
        XCTAssertEqual((items[0]["action"] as? [String: Any])?["kind"] as? String, "folder")
        XCTAssertEqual(items[2]["optional"] as? Bool, true, "signed out never greys the hire")
        XCTAssertFalse(config.managerEnabled, "only looked")

        XCTAssertEqual(try answer(router, try request(deck, "GET", path, tool: true)).status, 403, "never a tool's")
        let unknown = try answer(router, try request(deck, "GET", path, query: ["account": "no-such-account-t782"]))
        XCTAssertEqual(unknown.status, 400)
        XCTAssertTrue(text(unknown).hasPrefix("There's no Claude account called no-such-account-t782."), text(unknown))
    }

    /// Whether a GM is hired is in the setup state with the manager off too (the team is closed then), so
    /// Start here doesn't look again every 10 s for an office that has one. The registry's own answer.
    func testTheSetupStateSaysWhetherAGMIsHired() throws {
        let config = Config.shared
        config.managerEnabled = false
        let deck = quietDeck()
        let answered = try answer(DeckRouter(deck: deck), try request(deck, "GET", "/setup/state"))
        XCTAssertEqual(answered.status, 200, text(answered))
        let state = try XCTUnwrap(JSONSerialization.jsonObject(with: answered.body) as? [String: Any])
        XCTAssertEqual(state["gmHired"] as? Bool, BriefRoutes.gmIsHired(among: deck.frontDesk.hires))
        XCTAssertTrue(try source("Sources/AgentDeck/DeckRouter.swift").contains(#""gmHired": BriefRoutes.gmIsHired(among: deck.frontDesk.hires),"#))
    }

    /// With no folder for your hires (or one that's gone), never your home folder: refused, the switches untouched.
    func testTheGMHireWithoutAFolderIsRefusedWithTheSwitchesUntouched() throws {
        let config = Config.shared
        let deck = quietDeck()
        let router = DeckRouter(deck: deck)
        for folder in ["", scratch.appendingPathComponent("gone").path] {
            config.hireFolder = folder
            config.managerEnabled = false
            config.hiringEnabled = false
            let answered = try answer(router, try request(deck, "POST", "/setup/hire/gm"))
            XCTAssertEqual(answered.status, 409, folder)
            XCTAssertEqual(text(answered), DeckRouter.chooseHireFolderFirst)
            XCTAssertFalse(config.managerEnabled, folder)
            XCTAssertFalse(config.hiringEnabled, folder)
        }
        XCTAssertEqual(DeckRouter.chooseHireFolderFirst, "Choose the folder for your hires first: it's the first step of Start here.")
    }

    /// The hire turns the manager and Hire and fire on, and puts both back when it's refused. The gm role is
    /// taken out for this, so the hire is refused before any window could open.
    func testARefusedGMHirePutsBothSwitchesBack() throws {
        let config = Config.shared
        config.hireFolder = scratch.path
        config.managerEnabled = false
        config.hiringEnabled = false
        config.roles = config.roles.filter { $0.id != GMPlugin.role }
        let deck = quietDeck()
        let answered = try answer(DeckRouter(deck: deck), try request(deck, "POST", "/setup/hire/gm"))
        XCTAssertEqual(answered.status, 404, text(answered))
        XCTAssertEqual(text(answered), "That role isn't there any more. Pick another.")
        XCTAssertFalse(config.managerEnabled, "the manager put back")
        XCTAssertFalse(config.hiringEnabled, "Hire and fire put back")
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

    /// A refusal under Hire with its fix elsewhere carries a button there (FX2): the accounts, Connect, the folder step.
    func testARefusalUnderHirePointsAtItsFix() throws {
        let page = try source("Resources/web/setup.html")
        XCTAssertTrue(page.contains(#"<p class="said" id="start-said" role="status" hidden><span id="start-said-text"></span><button class="btn" id="start-said-do" hidden></button></p>"#))
        XCTAssertTrue(page.contains("startSaid(why, true, hireRefusalFix(why));"))
        XCTAssertTrue(page.contains("return { label: 'Add an account', run: () => {"))
        XCTAssertTrue(page.contains("if (why.startsWith('Connect your agents first')) return { label: 'Connect',"))
        // The words the page answers are the apps' own.
        let refused = try XCTUnwrap(Deck.testCopyAccountRefusal(account: ClaudeProfiles.defaultProfile(home: URL(fileURLWithPath: home)), testCopy: true))
        XCTAssertTrue(refused.message.hasSuffix(GMPreflight.addAccountFix))
        XCTAssertTrue(page.contains(#"/Add one under Claude accounts: |Choose one other than default, |"#))
        XCTAssertFalse(refused.message.contains(".claude.json"), "no file name in the sentence")
        XCTAssertTrue(Deck.hooksRefusal(for: .claude, found: [.claude], connected: { _ in false })?.hasPrefix("Connect your agents first") ?? false)
        // In a test copy, the picker under Hire starts on a test account.
        XCTAssertTrue(page.contains("const testOne = testCopy() && profiles.find((p) => !p.default);"))
        XCTAssertTrue(page.contains("if (profiles.some((p) => p.name === kept) && !(testOne && kept === 'default')) pick.value = kept;"),
                      "a test copy moves off the default account once another is added")
        XCTAssertTrue(page.contains("<b>Change where hires work</b>"))
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
