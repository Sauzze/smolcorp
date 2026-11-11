import XCTest
@testable import AgentDeck

/// The T3 Code desk (T797b): the gate, the hire in T797a's order with its fence, the undo, the follow, the deep link,
/// mail on standard input, letting go, and `t3` with its pinned argument lists only. A scratch home folder holds a
/// fake `t3` that writes down each run (and, at `project add`, what was on disk then); the git repo is a real one in
/// the same scratch folder. The real `t3` and T3 Code are never run here (see `T3DeskRealFenceTests`).
final class T3DeskTests: XCTestCase {
    private var home: URL!
    private var calls: URL { home.appendingPathComponent("calls") }
    private var flags: URL { home.appendingPathComponent("flags") }
    private var notes: URL { home.appendingPathComponent("notes") }
    private var stdin: URL { home.appendingPathComponent("stdin") }
    private var support: URL { home.appendingPathComponent("support") }
    private var opened: [URL] = []
    private var log: [String] = []

    override func setUpWithError() throws {
        home = URL(fileURLWithPath: HireDesk.physical(NSTemporaryDirectory())).appendingPathComponent("t3desk-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: support.appendingPathComponent("hires"), withIntermediateDirectories: true)
        opened = []
        log = []
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: home)
    }

    private func runs() -> [[String]] {
        let text = (try? String(contentsOf: calls, encoding: .utf8)) ?? ""
        return text.components(separatedBy: "--\n").dropLast().map { $0.split(separator: "\n", omittingEmptySubsequences: false).map(String.init).filter { !$0.isEmpty } }
    }

    /// A real repo, one commit, its only remote none.
    private func repo() throws -> String {
        let repo = home.appendingPathComponent("repo").path
        for arguments in [["init", "-q", "-b", "main", repo], ["-C", repo, "commit", "-q", "--allow-empty", "-m", "init"]] {
            let git = Process()
            git.executableURL = URL(fileURLWithPath: "/usr/bin/git")
            git.arguments = ["-c", "user.name=t", "-c", "user.email=t@example.invalid"] + arguments
            try git.run()
            git.waitUntilExit()
            XCTAssertEqual(git.terminationStatus, 0, arguments.joined(separator: " "))
            // After the init, before the first commit; only for a repo that was made.
            if arguments[0] == "init", git.terminationStatus == 0 { TempRepo.stopSigning(repo) }
        }
        return repo
    }

    private func worktree() -> String { HireDesk.physical(home.appendingPathComponent("repo-hires/ada-abcdef").path) }

    /// T797a's real `link`, from `t3 thread show --json`.
    private let realLink = "https://app.t3.codes/threads/4e467189-667c-4dec-b427-6a0819b039cb/mcp%3A32872c1f-1f9c-4bbd-a31c-0a28a6248867"

    func testTheDeepLinkIsBuiltFromARealLink() {
        XCTAssertEqual(T3Desk.deepLink(thread: "mcp:32872c1f-1f9c-4bbd-a31c-0a28a6248867", link: realLink)?.absoluteString,
                       "t3code://threads/4e467189-667c-4dec-b427-6a0819b039cb/mcp%3A32872c1f-1f9c-4bbd-a31c-0a28a6248867")
        XCTAssertNil(T3Desk.deepLink(thread: "", link: realLink))
        XCTAssertNil(T3Desk.deepLink(thread: "mcp:x", link: ""))
        XCTAssertNil(T3Desk.deepLink(thread: "mcp:x", link: "https://app.t3.codes/projects/x"))
        XCTAssertNil(T3Desk.deepLink(thread: "mcp:x", link: "https://app.t3.codes/threads/a%2Fb/mcp%3Ax"), "an env that isn't plain")
        XCTAssertEqual(T3Desk.deepLink(thread: "mcp:a/b?c", link: realLink)?.absoluteString,
                       "t3code://threads/4e467189-667c-4dec-b427-6a0819b039cb/mcp%3Aa%2Fb%3Fc", "the thread id encoded whole")
    }

    func testAProcessInTheWorktreeIsFoundByThePIDList() throws {
        let wt = home.appendingPathComponent("busy wt", isDirectory: true)
        try FileManager.default.createDirectory(at: wt, withIntermediateDirectories: true)
        let sleeper = Process()
        sleeper.executableURL = URL(fileURLWithPath: "/bin/sleep")
        sleeper.arguments = ["30"]
        sleeper.currentDirectoryURL = wt
        try sleeper.run()
        defer { sleeper.terminate(); sleeper.waitUntilExit() }
        XCTAssertTrue(Processes.working(in: wt.path).contains(sleeper.processIdentifier))
        XCTAssertFalse(Processes.working(in: home.appendingPathComponent("elsewhere").path).contains(sleeper.processIdentifier))
        XCTAssertEqual(Processes.arguments(of: sleeper.processIdentifier), ["/bin/sleep", "30"])
    }

    /// N6: the process still there is named by the kernel's own word for it.
    func testAProcessIsNamedAsTheKernelHasIt() throws {
        let sleeper = Process()
        sleeper.executableURL = URL(fileURLWithPath: "/bin/sleep")
        sleeper.arguments = ["30"]
        try sleeper.run()
        defer { sleeper.terminate(); sleeper.waitUntilExit() }
        XCTAssertEqual(Processes.name(of: sleeper.processIdentifier), "sleep")
        XCTAssertEqual(Processes.name(of: -1), "")
    }

    private func source(_ path: String) throws -> String {
        try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent().appendingPathComponent("../../\(path)"), encoding: .utf8)
    }

    private func web(_ name: String) throws -> String { try source("Resources/web/\(name)") }

    func testSetupShowsTheDeskTheTagOpenAndLetGosConfirmAndFitsAPhoneWidth() throws {
        let page = try web("setup.html")
        for words in [
            "const T3_TAG = 'EXPERIMENTAL · runs in T3 Code';",
            "const T3_LETGO = \"T3 Code's thread for this hire, and its history in T3 Code, are removed. Its worktree and branch stay.\";",
            "const T3_STOP_THERE = 'Or open it in T3 Code to stop it there: nothing is removed.';",
            "textContent: `Let ${name} go? ${T3_LETGO}`",
            "fetch('/setup/t3code/letgo', { method: 'POST', headers: setupHeaders(true), body: JSON.stringify({ id: r.id }) })",
            "textContent: 'Open in T3 Code instead'",
            "keys.append(inT3 ? letGoMenu(r, () => { if (t3Key) t3Key.button.click(); }) : fire);",
            "if (inT3) t3Desk(r, label);",
            "link: \"T3 Code is open on this hire's own thread.\"",
            // The hire sheet: the engine choice, greyed with its one sentence, sent as engine t3.
            "<option value=\"t3\">In T3 Code (experimental)</option>",
            "$('hs-engine').options[1].disabled = !found.lane.ok;",
            "...(hireIntoT3() ? { engine: 't3' } : {}),",
        ] {
            XCTAssertTrue(page.contains(words), words)
        }
        // 390 px: the tag doesn't push the row wider; the row still wraps its buttons under the name.
        XCTAssertTrue(page.contains("white-space: nowrap; }"))
        XCTAssertTrue(page.contains("head.style.flexWrap = 'wrap';"))
        XCTAssertTrue(page.contains("body.style.cssText = 'display:grid;gap:6px;padding:8px 0 0';\n    const confirm"), "Let go's sentences wrap in its menu, as Fire…'s")
        // The answer of a let go outlives the redraw, as an open's does.
        XCTAssertTrue(page.contains("const t3Leaving = new Map();"))
    }

    func testTheOfficeCardSaysSoForAT3Hire() throws {
        let page = try web("deck.html")
        for words in ["if (found.lane) {", "lane.tag", "Let it go from Setup's team list or from its desk.", "link: \"T3 Code is open on this hire's own thread.\""] {
            XCTAssertTrue(page.contains(words), words)
        }
    }
}

/// The fence, for real (T797b's merge gate): Smolcorp's own hire path (`T3Desk.hire`, then `T3Desk.letGo`) on the real
/// T3 Code and its real `t3`, one Haiku turn. Opt-in, never in CI: `SMOLCORP_T3_REAL=1`, with `SMOLCORP_T3_CLI` the
/// `smolcorp` command whose guard fences it (`go build -o <scratch>/smolcorp ./cmd/smolcorp`). Based on T797a's
/// fence-proof.sh: a scratch repo whose only remote is an empty LOCAL bare repo; the brief tries `git push` and a read
/// the deny rules refuse (a stand-in config.json in the test's own support folder, holding a random word, no secret).
/// It asserts both were blocked and the remote has 0 refs, then lets the hire go and checks T3 Code's project is gone.
/// Only its own scratch project is added and removed; no T3 setting is changed.
final class T3DeskRealFenceTests: XCTestCase {
    private func git(_ arguments: [String]) -> (status: Int32, output: String)? {
        Tool.run("/usr/bin/git", ["-c", "user.name=fence", "-c", "user.email=fence@example.invalid"] + arguments, errors: true)
    }
}
