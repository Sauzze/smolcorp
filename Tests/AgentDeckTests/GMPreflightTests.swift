import XCTest
@testable import AgentDeck

/// A first GM's preflight from given facts: no shell, no real claude, no Apple Events, nothing on the network.
final class GMPreflightTests: XCTestCase {
    private let home = "/Users/someone"

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

    func testSignedOutSaysHowToSignIn() throws {
        var f = ready()
        f.signIn = .no
        let signin = try line(GMPreflight.items(f), "signin")
        XCTAssertFalse(signin.ok)
        XCTAssertFalse(signin.optional)
        XCTAssertEqual(signin.action?.command, "claude")
        XCTAssertTrue(signin.fix.contains("/login"))
        XCTAssertFalse(isReady(GMPreflight.items(f)))
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
        for item in items { XCTAssertEqual(Set(item.keys), ["id", "ok", "title", "fix", "action", "optional"]) }
        XCTAssertEqual((items[0]["action"] as? [String: Any])?["kind"] as? String, "folder")
        XCTAssertTrue(items[1]["action"] is NSNull)
        XCTAssertNotNil(try JSONSerialization.data(withJSONObject: report))
    }

    private func source(_ path: String) throws -> String {
        let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        return try String(contentsOf: repo.appendingPathComponent(path), encoding: .utf8)
    }

    /// Outside /setup/gm/, which is closed while the manager is off, as on a first run: the page reads it.
    func testTheRouteAnswersWithTheManagerOff() throws {
        let path = "/setup/hire/gm/preflight"
        XCTAssertNil(ManagerSwitch.refusal("GET", path, enabled: false))
        XCTAssertTrue(SetupRoutes.allows(.page, "GET", path))
        XCTAssertTrue(try source("Sources/AgentDeck/DeckRouter.swift").contains(#"case ("GET", "/setup/hire/gm/preflight"):"#))
        XCTAssertTrue(try source("Sources/AgentDeck/SelfTest.swift").contains("case GMPreflight.automationFix:"))
    }

    /// One click: in the folder for your hires (never home: refused, pointing at the folder step), Hire and fire
    /// turned on with the manager and both put back if the hire fails.
    func testTheGMHireNeedsTheFolderAndTurnsHiringOn() throws {
        let router = try source("Sources/AgentDeck/DeckRouter.swift")
        let route = try XCTUnwrap(router.range(of: #"case ("POST", "/setup/hire/gm"):"#))
        let body = router[route.upperBound...].prefix(1200)
        XCTAssertNotNil(body.range(of: "guard let folder = Deck.folder(config.hireFolder) else { return respond(.status(409, DeckRouter.chooseHireFolderFirst)) }"))
        XCTAssertNil(body.range(of: "NSHomeDirectory()"), "never the home folder")
        XCTAssertNotNil(body.range(of: "config.hiringEnabled = true"))
        XCTAssertNotNil(body.range(of: "config.managerEnabled = before; config.hiringEnabled = hiring"))
        XCTAssertEqual(DeckRouter.chooseHireFolderFirst, "Choose the folder for your hires first: it's the first step of Start here.")
    }

    /// The page: the checks before the button, the default account chosen, and what to say first after the hire.
    func testThePageShowsTheChecksAndWhatToSayFirst() throws {
        let page = try source("Resources/web/setup.html")
        for id in ["first-gm-checks", "start-gm", "first-gm-recheck", "first-gm-why", "first-gm-next", "first-gm-office", "first-gm-says"] {
            XCTAssertTrue(page.contains(#"id="\#(id)""#), id)
        }
        XCTAssertTrue(page.contains("fetch(`/setup/hire/gm/preflight${account}`"))
        XCTAssertTrue(page.contains("$('start-gm').setAttribute('aria-disabled', String(left.length > 0));"))
        XCTAssertTrue(page.contains("pick.value = profiles.find((p) => p.default).name"))
        // For the guide (T780): no data-ok while checking, then whether nothing greys the hire.
        XCTAssertTrue(page.contains("delete $('first-gm-checks').dataset.ok;"))
        XCTAssertTrue(page.contains("if (gmPreflightAsked && !gmPreflightBusy) box.dataset.ok = String(!left.length);"))
        let checks = try XCTUnwrap(page.range(of: #"id="first-gm-checks""#)), button = try XCTUnwrap(page.range(of: #"id="start-gm""#))
        XCTAssertLessThan(checks.lowerBound, button.lowerBound, "the checks come before the button")
    }
}
