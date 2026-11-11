import XCTest
@testable import AgentDeck

/// Firing a hire, with temp repos and a stand-in tmux: nothing real is started, nothing is pressed.
final class HireFireTests: XCTestCase {
    private var root: URL!
    private var repo: String!
    private var recaps: URL!
    private var worktree = ""
    private var branch = ""
    private var desk: FrontDesk!
    private var hire: FrontDesk.Hire!

    // The stand-in tmux.
    private var asked: [[String]] = []
    private var alive = true
    private var screen = HireFireTests.idle
    /// What happens to the session when Return is pressed.
    private var returnEnds = true

    static let rule = String(repeating: "─", count: 60)
    static let idle = "some work\n\(rule)\n❯\u{a0}Try \"fix a bug\"\n\(rule)\n  ⏵⏵ auto mode on (shift+tab to cycle)"
    static let menu = "some work\n\(rule)\n Bash command\n rm -rf build\n Do you want to proceed?\n ❯ 1. Yes\n   2. No\n\n Esc to cancel · Tab to amend"

    /// A capture Claude Code 2.1.316 really made, in Tests/fixtures/panes/.
    static func pane(_ name: String) throws -> String {
        let folder = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("fixtures/panes")
        return try String(contentsOf: folder.appendingPathComponent(name + ".txt"), encoding: .utf8)
    }

    override func setUpWithError() throws {
        root = URL(fileURLWithPath: HireDesk.physical(NSTemporaryDirectory())).appendingPathComponent("hirefire-\(UUID().uuidString)")
        recaps = root.appendingPathComponent("recaps")
        repo = root.appendingPathComponent("app").path
        try FileManager.default.createDirectory(atPath: repo, withIntermediateDirectories: true)
        try "x".write(toFile: repo + "/a.txt", atomically: true, encoding: .utf8)
        for arguments in [["init", "-q", "-b", "main"], ["add", "."], ["-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "first"]] {
            let done = HireDesk.git(repo, arguments)
            XCTAssertEqual(done?.status, 0, arguments.joined(separator: " "))
            if arguments[0] == "init", done?.status == 0 { TempRepo.stopSigning(repo) }  // only a repo that was made
        }
        branch = "hire/rev-abcdef"
        worktree = HireDesk.physical(root.path) + "/app-hires/rev-abcdef"
        XCTAssertEqual(HireDesk.git(repo, ["worktree", "add", "-b", branch, worktree])?.status, 0)
        desk = FrontDesk(folder: root)
        hire = FrontDesk.Hire(id: "abcdef123456", name: "Reviewer", nickname: "Rev", role: "reviewer", folder: repo, branch: branch,
                              worktree: worktree, repo: repo, tmuxSocket: "/t/smolcorp-abcdef123456", ports: "4100-4103", started: 1_761_000_000_000)
        desk.register(hire)
    }

    func testTheTempRepoDoesNotSign() {
        XCTAssertEqual(TempRepo.local("commit.gpgsign", in: repo), "false")
        XCTAssertEqual(TempRepo.local("commit.gpgsign", in: worktree), "false", "the hire's worktree shares the repo's config")
    }

    override func tearDownWithError() throws { try? FileManager.default.removeItem(at: root) }

    private func tmux() -> HireTmux {
        HireTmux(hireID: "abcdef123456", testPort: nil) { arguments in
            self.asked.append(arguments)
            switch arguments.dropFirst(2).first {
            case "has-session": return (self.alive ? 0 : 1, "")
            case "capture-pane": return (0, self.screen)
            case "send-keys": if self.returnEnds { self.alive = false }; return (0, "")
            case "kill-server": self.alive = false; return (0, "")
            default: return (0, "")
            }
        }!
    }

    private func fire(_ options: HireDesk.FireOptions = .init()) -> Result<HireDesk.Fired, HireDesk.Refusal> {
        HireDesk.fire(hire, options: options, tmux: tmux(), desk: desk, recaps: recaps, pause: { _ in })
    }

    private func sent(_ word: String) -> Bool { asked.contains { $0.contains(word) } }

    private func refusal(_ result: Result<HireDesk.Fired, HireDesk.Refusal>) -> HireDesk.Refusal? {
        if case .failure(let refusal) = result { return refusal }
        return nil
    }

    // MARK: Reading the window

    func testRealCapturesAreToldApartByTheirStructure() throws {
        let expected: [(String, HireDesk.Input, Bool)] = [
            ("idle-empty", .idleAndEmpty, false), ("after-decline", .idleAndEmpty, false), ("draft", .draft, false),
            ("busy-1", .busy, false), ("permission-menu", .unknown, true), ("trust-menu", .unknown, true),
        ]
        for (name, state, asking) in expected {
            let pane = try HireFireTests.pane(name)
            XCTAssertEqual(HireDesk.inputState(pane), state, name)
            XCTAssertEqual(HireDesk.promptRefusal(pane) != nil, asking, name)
        }
    }

    func testQuestionWordsInTheScrollbackCountForNothing() throws {
        let words = "Do you want to proceed?\n❯ 1. Yes\nEsc to cancel\nEnter to confirm\n(y/n)\n"
        let pane = words + (try HireFireTests.pane("idle-empty"))
        XCTAssertNil(HireDesk.promptRefusal(pane))
        XCTAssertEqual(HireDesk.inputState(pane), .idleAndEmpty)
        // The old box-drawing look of an input box is not one.
        XCTAssertEqual(HireDesk.inputState("│ > │"), .unknown)
    }

    func testSyntheticVariants() {
        let rule = HireFireTests.rule
        func box(_ lines: String..., footer: String = "  ⏵⏵ auto mode on (shift+tab to cycle)") -> String {
            "old output\n\(rule)\n" + lines.joined(separator: "\n") + "\n\(rule)\n" + footer
        }
        // A long draft, a pasted multi-line message, a draft that is itself "Try …" text.
        XCTAssertEqual(HireDesk.inputState(box("❯ " + String(repeating: "word ", count: 80))), .draft)
        XCTAssertEqual(HireDesk.inputState(box("❯ [Pasted text #1 +3 lines]")), .draft)
        XCTAssertEqual(HireDesk.inputState(box("❯ first line", "  second line", "  third line")), .draft)
        XCTAssertEqual(HireDesk.inputState(box("❯", "  and a pasted second line")), .draft)
        // Busy wins over an empty box; an empty box with no footer can't be trusted (cut off): unknown.
        XCTAssertEqual(HireDesk.inputState(box("❯", footer: "  ⏵⏵ auto mode on · esc to interrupt")), .busy)
        XCTAssertEqual(HireDesk.inputState(box("❯ half", footer: "  ⏵⏵ auto mode on · esc to interrupt")), .busy)
        XCTAssertEqual(HireDesk.inputState(box("❯", footer: "")), .unknown)
        // A menu with two options, and a question with a confirm footer, are asking; the box lines aren't.
        let two = "text\n\(rule)\n Overwrite the file?\n ❯ 1. Yes\n   2. No\n"
        XCTAssertNotNil(HireDesk.promptRefusal(two))
        XCTAssertEqual(HireDesk.inputState(two), .unknown)
        XCTAssertNotNil(HireDesk.promptRefusal("Pick a folder\n ❯ Downloads\n   Desktop\n\n Enter to confirm · Esc to cancel"))
        // An old box higher up with a menu under it: the menu is what shows.
        XCTAssertNotNil(HireDesk.promptRefusal(box("❯") + "\n\(rule)\n Do you want to proceed?\n ❯ 1. Yes\n   2. No\n Esc to cancel"))
        XCTAssertEqual(HireDesk.inputState(box("❯") + "\n\(rule)\n Do you want to proceed?\n ❯ 1. Yes\n   2. No\n Esc to cancel"), .unknown)
        // Neither a box nor a menu: unknown, not asking. Nothing to look at: refused.
        XCTAssertEqual(HireDesk.inputState("nothing like a box"), .unknown)
        XCTAssertNil(HireDesk.promptRefusal("nothing like a box"))
        XCTAssertEqual(HireDesk.inputState(nil), .unknown)
        XCTAssertNotNil(HireDesk.promptRefusal(nil))
    }

    func testTheCostIsTakenFromTheWindowWhenItShowsOne() {
        XCTAssertEqual(HireDesk.visibleCost("Total cost: $1.23\nTotal duration: 4m"), "$1.23")
        XCTAssertNil(HireDesk.visibleCost(HireFireTests.idle))
        XCTAssertNil(HireDesk.visibleCost(nil))
    }

    // MARK: Refusals

    func testUncommittedWorkIsRefusedWithItsFilesAndNothingIsSent() throws {
        try "mine".write(toFile: worktree + "/new.txt", atomically: true, encoding: .utf8)
        let no = try XCTUnwrap(refusal(fire()))
        XCTAssertEqual(no.status, 409)
        XCTAssertTrue(no.message.contains("new.txt"), no.message)
        // It names the hire (its nickname), not the role it was hired for.
        XCTAssertEqual(no.message, "Rev has changes that aren't committed: ?? new.txt. Commit them, or fire with force.")
        XCTAssertFalse(sent("set-buffer"))
        XCTAssertFalse(sent("send-keys"))
        XCTAssertFalse(sent("kill-server"))
        XCTAssertEqual(desk.hires[0].state, .starting)
        XCTAssertFalse(FileManager.default.fileExists(atPath: recaps.path))
    }

    func testForceFiresAnywayAndTheFilesAreKept() throws {
        try "mine".write(toFile: worktree + "/new.txt", atomically: true, encoding: .utf8)
        guard case .success(let fired) = fire(.init(force: true, removeWorktree: true)) else { return XCTFail("refused") }
        XCTAssertFalse(fired.kept.isEmpty)
        XCTAssertTrue(FileManager.default.fileExists(atPath: worktree + "/new.txt"))
        XCTAssertEqual(desk.hires[0].state, .fired)
        XCTAssertTrue(desk.hires[0].notes.contains("was kept"))
    }

    /// The refusals of a fire name the hire as the rest of the app does: its nickname, else its name.
    func testEveryFireRefusalNamesTheHireNotTheRole() {
        XCTAssertEqual(HireDesk.called(hire), "Rev")
        var plain = hire!
        plain.nickname = ""
        XCTAssertEqual(HireDesk.called(plain), "Reviewer")
        var gone = hire!
        gone.state = .fired
        XCTAssertEqual(HireDesk.fireRefusal(gone, force: false, capture: nil)?.message, "Rev is already gone.")
        screen = HireFireTests.idle.replacingOccurrences(of: "❯\u{a0}Try \"fix a bug\"", with: "❯ half a thought")
        XCTAssertEqual(refusal(fire(.init(force: false)))?.message, "Rev has something typed in its window that isn't sent yet. Send or clear it, or fire with force.")
    }

    /// A reason that follows a colon starts in lower case; an acronym keeps its capitals.
    func testAReasonAfterAColonStartsInLowerCase() {
        XCTAssertEqual(HireDesk.afterColon("It holds changes nobody committed."), "it holds changes nobody committed.")
        XCTAssertEqual(HireDesk.afterColon("API keys are in it."), "API keys are in it.")
        XCTAssertEqual(HireDesk.afterColon("already lower"), "already lower")
        XCTAssertEqual(HireDesk.afterColon(""), "")
        XCTAssertEqual(HireDesk.afterColon("I"), "I")
        XCTAssertEqual(HireDesk.afterColon("I can't."), "I can't.")
        XCTAssertEqual(HireDesk.afterColon("I'll try."), "I'll try.")
        XCTAssertEqual(HireDesk.afterColon("I\u{2019}ll try."), "I\u{2019}ll try.")
    }

    func testAQuestionIsNeverAnsweredNotEvenWithForce() {
        screen = HireFireTests.menu
        XCTAssertEqual(refusal(fire(.init(force: true)))?.status, 409)
        XCTAssertNil(asked.first { $0.contains("set-buffer") || $0.contains("send-keys") || $0.contains("paste-buffer") || $0.contains("kill-server") })
        XCTAssertEqual(desk.hires[0].state, .starting)
    }

    func testAWindowThatCantBeReadIsLeftAlone() {
        screen = ""
        let blind = HireTmux(hireID: "abcdef123456", testPort: nil) { self.asked.append($0); return $0.contains("capture-pane") ? (1, "") : (0, "") }
        let result = HireDesk.fire(hire, tmux: blind, desk: desk, recaps: recaps, pause: { _ in })
        XCTAssertNotNil(refusal(result))
        XCTAssertFalse(sent("kill-server"))
    }

    func testAHireAlreadyFiredIsRefused() {
        desk.move(hire.id, to: .fired)
        hire.state = .fired
        XCTAssertEqual(refusal(fire())?.status, 409)
    }

    // MARK: Going

    func testAnIdleHireIsToldToExitThenLeftToEnd() throws {
        guard case .success(let fired) = fire() else { return XCTFail("refused") }
        let paste = try XCTUnwrap(asked.first { $0.contains("set-buffer") })
        XCTAssertEqual(paste.last, "/exit")
        XCTAssertTrue(asked.contains(["-L", "smolcorp-abcdef123456", "send-keys", "-t", "=hire:", "Enter"]))
        // It ended by itself, so its server is not killed.
        XCTAssertFalse(sent("kill-server"))
        XCTAssertEqual(desk.hires[0].state, .fired)
        XCTAssertTrue(desk.portsInUse.isEmpty)
        // The worktree stays unless asked for.
        XCTAssertTrue(FileManager.default.fileExists(atPath: worktree))
        XCTAssertEqual(fired.kept, "Kept, as asked.")
    }

    func testAHireThatIgnoresExitHasItsOwnServerKilled() {
        returnEnds = false
        _ = fire()
        XCTAssertTrue(asked.contains(["-L", "smolcorp-abcdef123456", "kill-server"]))
        // Every call names our socket, never the user's own tmux.
        XCTAssertTrue(asked.allSatisfy { $0.starts(with: ["-L", "smolcorp-abcdef123456"]) })
    }

    func testAHireAtWorkIsRefusedUnlessForcedAndNeverTypedInto() throws {
        screen = "✻ Working…\n\(HireFireTests.rule)\n❯\n\(HireFireTests.rule)\n  ⏵⏵ auto mode on · esc to interrupt"
        let no = try XCTUnwrap(refusal(fire()))
        XCTAssertEqual(no.status, 409)
        XCTAssertTrue(no.message.contains("working"), no.message)
        XCTAssertFalse(sent("set-buffer"))
        XCTAssertFalse(sent("send-keys"))
        XCTAssertFalse(sent("kill-server"))
        XCTAssertEqual(desk.hires[0].state, .starting)
        // With force it is ended, still without typing.
        asked = []
        guard case .success = fire(.init(force: true)) else { return XCTFail("refused") }
        XCTAssertFalse(sent("set-buffer"))
        XCTAssertFalse(sent("send-keys"))
        XCTAssertTrue(sent("kill-server"))
    }

    func testAHireWithADraftIsRefusedUnlessForcedAndNeverTypedInto() throws {
        screen = "\(HireFireTests.rule)\n❯ fix the login te\n\(HireFireTests.rule)\n  ⏵⏵ auto mode on"
        let no = try XCTUnwrap(refusal(fire()))
        XCTAssertEqual(no.status, 409)
        XCTAssertTrue(no.message.contains("typed"), no.message)
        XCTAssertFalse(sent("set-buffer"))
        XCTAssertFalse(sent("send-keys"))
        XCTAssertFalse(sent("kill-server"))
        XCTAssertEqual(desk.hires[0].state, .starting)
        asked = []
        guard case .success = fire(.init(force: true)) else { return XCTFail("refused") }
        XCTAssertFalse(sent("set-buffer"))
        XCTAssertFalse(sent("send-keys"))
        XCTAssertTrue(sent("kill-server"))
    }

    func testAWindowWithNoBoxToTellApartIsRefusedUnlessForced() throws {
        screen = "just some text, no prompt anywhere"
        let no = try XCTUnwrap(refusal(fire()))
        XCTAssertEqual(no.status, 409)
        XCTAssertFalse(sent("kill-server"))
        XCTAssertEqual(desk.hires[0].state, .starting)
        guard case .success = fire(.init(force: true)) else { return XCTFail("refused") }
        XCTAssertTrue(sent("kill-server"))
    }

    func testAHireFiredMeanwhileIsNotFiredAgain() {
        // The caller's copy still says starting; the registry knows better.
        desk.move(hire.id, to: .fired)
        XCTAssertEqual(hire.state, .starting)
        let result = fire()
        XCTAssertEqual(refusal(result)?.status, 409)
        XCTAssertFalse(sent("kill-server"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: recaps.appendingPathComponent(HireDesk.recapName(hire)).path))
    }

    func testTwoFiresAtOnceWriteOneRecap() throws {
        returnEnds = false
        // The second fire is asked while the first is waiting for /exit.
        var second: Result<HireDesk.Fired, HireDesk.Refusal>?
        let pausing: (TimeInterval) -> Void = { _ in
            if second == nil { second = self.fire() }
        }
        let first = HireDesk.fire(hire, tmux: tmux(), desk: desk, recaps: recaps, pause: pausing)
        guard case .success = first else { return XCTFail("refused") }
        XCTAssertEqual(refusal(try XCTUnwrap(second))?.status, 409)
        XCTAssertEqual(desk.hires[0].state, .fired)
        XCTAssertEqual(try FileManager.default.contentsOfDirectory(atPath: recaps.path).count, 1)
    }

    func testAQuestionThatAppearsAfterTheFirstLookGetsNothingPasted() {
        // Idle at the first look, a menu at the second (just before typing).
        var looks = 0
        let changing = HireTmux(hireID: "abcdef123456", testPort: nil) { arguments in
            self.asked.append(arguments)
            switch arguments.dropFirst(2).first {
            case "has-session": return (0, "")
            case "capture-pane":
                looks += 1
                return (0, looks == 1 ? HireFireTests.idle : HireFireTests.menu)
            default: return (0, "")
            }
        }!
        let result = HireDesk.fire(hire, tmux: changing, desk: desk, recaps: recaps, pause: { _ in })
        XCTAssertEqual(refusal(result)?.status, 409)
        XCTAssertEqual(looks, 2)
        XCTAssertFalse(sent("set-buffer"))
        XCTAssertFalse(sent("paste-buffer"))
        XCTAssertFalse(sent("send-keys"))
        XCTAssertFalse(sent("kill-server"))
        XCTAssertEqual(desk.hires[0].state, .starting)
    }

    func testASessionAlreadyGoneJustMarksTheHire() {
        alive = false
        guard case .success = fire() else { return XCTFail("refused") }
        XCTAssertFalse(sent("kill-server"))
        XCTAssertFalse(sent("capture-pane"))
        XCTAssertEqual(desk.hires[0].state, .fired)
    }

    func testTheWorktreeIsRemovedWhenAskedAndClean() throws {
        guard case .success(let fired) = fire(.init(removeWorktree: true)) else { return XCTFail("refused") }
        XCTAssertEqual(fired.kept, "")
        XCTAssertFalse(FileManager.default.fileExists(atPath: worktree))
        XCTAssertEqual(HireDesk.git(repo, ["branch", "--list", branch])?.output, "")
    }

    // MARK: Recap

    func testTheRecapSaysWhoTaskBranchStatusAndCost() throws {
        try "# Review the login\n\nmore".write(toFile: worktree + "/BRIEF.md", atomically: true, encoding: .utf8)
        try "one\ntwo\nthree: tests pass\n".write(toFile: worktree + "/STATUS.md", atomically: true, encoding: .utf8)
        // Committed, so the worktree is clean and the fire isn't refused.
        for arguments in [["add", "BRIEF.md", "STATUS.md"], ["-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "notes"]] {
            XCTAssertEqual(HireDesk.git(worktree, arguments)?.status, 0)
        }
        screen = HireFireTests.idle + "\nTotal cost: $0.42"
        guard case .success(let fired) = fire() else { return XCTFail("refused") }
        XCTAssertEqual(fired.recap, recaps.appendingPathComponent("abcdef123456.md").path)
        let text = try String(contentsOfFile: fired.recap, encoding: .utf8)
        for part in ["# Recap: Rev", "Who: Reviewer (reviewer), called Rev", "Task: Review the login", "Branch: \(branch)",
                     "(kept)", "Cost: $0.42", "- three: tests pass"] {
            XCTAssertTrue(text.contains(part), "\(part)\n\(text)")
        }
        let mode = try FileManager.default.attributesOfItem(atPath: fired.recap)[.posixPermissions] as? Int
        XCTAssertEqual(mode, 0o600)
    }

    func testTheRecapReadsAManagedHiresBriefAndStatusFromItsOwnFolder() throws {
        let hires = root.appendingPathComponent("hires")
        let own = BriefBuilder.hireFolder(hire.id, in: hires)
        try FileManager.default.createDirectory(at: own, withIntermediateDirectories: true)
        try "# Review the login\n".write(to: own.appendingPathComponent("BRIEF.md"), atomically: true, encoding: .utf8)
        try "one\ntwo: tests pass\n".write(to: own.appendingPathComponent("STATUS.md"), atomically: true, encoding: .utf8)
        let path = HireDesk.writeRecap(hire, capture: nil, worktreeRemoved: false, kept: "", in: recaps, now: Date(), hires: hires)
        let text = try String(contentsOfFile: path, encoding: .utf8)
        XCTAssertTrue(text.contains("Task: Review the login"), text)
        XCTAssertTrue(text.contains("- two: tests pass"), text)
    }

    func testARecapWithoutABriefOrStatusOrCostSaysSo() throws {
        guard case .success(let fired) = fire() else { return XCTFail("refused") }
        let text = try String(contentsOfFile: fired.recap, encoding: .utf8)
        XCTAssertTrue(text.contains("Task: not written down"))
        XCTAssertTrue(text.contains("Cost: not shown"))
        XCTAssertTrue(text.contains("Nothing was written to STATUS.md."))
    }

    func testTheRecapNameCantEscapeItsFolder() {
        XCTAssertEqual(HireDesk.recapName(.init(id: "../x y", name: "n", folder: "/f")), "x-y.md")
        XCTAssertEqual(HireDesk.recapName(.init(id: "ab12-cd", name: "n", folder: "/f")), "ab12-cd.md")
    }

    // MARK: Hires without a session of their own (1.23)

    func testAHireWithoutTmuxIsRefusedForUnsavedWorkAndSettledOtherwise() throws {
        var plain = hire!
        plain.tmuxSocket = ""
        try "mine".write(toFile: worktree + "/new.txt", atomically: true, encoding: .utf8)
        XCTAssertTrue(HireDesk.fireRefusal(plain, force: false, capture: nil)?.message.contains("new.txt") == true)
        XCTAssertNil(HireDesk.fireRefusal(plain, force: true, capture: nil))
        HireDesk.settle(plain, options: .init(force: true), desk: desk, recaps: recaps)
        XCTAssertEqual(desk.hires[0].state, .fired)
        XCTAssertTrue(FileManager.default.fileExists(atPath: recaps.appendingPathComponent("abcdef123456.md").path))
    }

    func testASessionIsMatchedToItsHireByTabSocketOrFolder() {
        let tab = "tmux:%0@/t/smolcorp-abcdef123456"
        XCTAssertEqual(HireDesk.hire(ofTab: tab, folder: "/elsewhere", project: "", among: [hire])?.id, "abcdef123456")
        // A hire on its own tmux server is matched by its socket only: a session of yours in its folder is not it.
        XCTAssertNil(HireDesk.hire(ofTab: "", folder: worktree, project: "", among: [hire]))
        XCTAssertNil(HireDesk.hire(ofTab: "tmux:%0@/t/other", folder: worktree, project: "", among: [hire]))
        XCTAssertNil(HireDesk.hire(ofTab: "tmux:%0@/t/other", folder: repo, project: "", among: [hire]))
        // A hire without tmux and with a worktree is matched by where it works.
        var plain = hire!
        plain.tmuxSocket = ""
        XCTAssertEqual(HireDesk.hire(ofTab: "", folder: worktree, project: "", among: [plain])?.id, "abcdef123456")
        XCTAssertNil(HireDesk.hire(ofTab: "", folder: "/elsewhere", project: "", among: [plain]))
        // Without a worktree it works in the project folder, which is the user's own too: never matched by folder.
        plain.worktree = ""
        XCTAssertNil(HireDesk.hire(ofTab: "", folder: repo, project: "", among: [plain]))
        var gone = hire!
        gone.state = .fired
        XCTAssertNil(HireDesk.hire(ofTab: tab, folder: worktree, project: "", among: [gone]))
    }

    // MARK: What a fire answers

    func testWhatAFireAnswers() {
        // A tmux hire is ended by HireDesk.fire: its viewer tab's result doesn't change the answer.
        XCTAssertEqual(Deck.fireAnswer(hireHasTmux: true, closed: .notStopped).status, 204)
        XCTAssertEqual(Deck.fireAnswer(hireHasTmux: true, closed: .gone).status, 204)
        XCTAssertEqual(Deck.fireAnswer(hireHasTmux: true, closed: nil).status, 204)
        // Everyone else is answered by the tab, as in 1.23.
        XCTAssertEqual(Deck.fireAnswer(hireHasTmux: false, closed: .notStopped).status, 409)
        XCTAssertEqual(Deck.fireAnswer(hireHasTmux: false, closed: .gone).status, 404)
        XCTAssertEqual(Deck.fireAnswer(hireHasTmux: false, closed: .fired).status, 204)
        XCTAssertEqual(Deck.fireAnswer(hireHasTmux: nil, closed: .notStopped).status, 409)
        XCTAssertEqual(Deck.fireAnswer(hireHasTmux: nil, closed: .gone).status, 404)
        XCTAssertEqual(Deck.fireAnswer(hireHasTmux: nil, closed: .fired).status, 204)
    }

    /// `gm fire --remove-worktree` on a worktree that stayed: the answer says so, with where and why, and is still a success.
    func testTheFireAnswerSaysWhenTheWorktreeWasKept() throws {
        let hire = try XCTUnwrap(desk.hires.first)
        let answer = Deck.fireAnswer(hireHasTmux: true, closed: nil, kept: (why: "It holds changes.", hire: hire))
        XCTAssertEqual(answer.status, 200)
        let json = try XCTUnwrap(JSONSerialization.jsonObject(with: answer.body) as? [String: Any])
        XCTAssertEqual(json["ok"] as? Bool, true)
        XCTAssertEqual(json["kept"] as? String, "It holds changes.")
        XCTAssertEqual(json["worktree"] as? String, hire.worktree)
        XCTAssertEqual(json["branch"] as? String, hire.branch)
        // A tab that wouldn't close is still told, kept or not.
        XCTAssertEqual(Deck.fireAnswer(hireHasTmux: false, closed: .notStopped, kept: (why: "x", hire: hire)).status, 409)
    }

    func testTheFireBodyNeedsAnIdAndReadsItsFlags() throws {
        XCTAssertNil(Deck.fireRequest(Data()))
        XCTAssertNil(Deck.fireRequest(Data("{}".utf8)))
        XCTAssertNil(Deck.fireRequest(Data(#"{"id":""}"#.utf8)))
        XCTAssertNil(Deck.fireRequest(Data(#"{"id":7}"#.utf8)))
        let plain = try XCTUnwrap(Deck.fireRequest(Data(#"{"id":"abc"}"#.utf8)))
        XCTAssertEqual(plain.id, "abc")
        XCTAssertEqual(plain.options, HireDesk.FireOptions(force: false, removeWorktree: false))
        let both = try XCTUnwrap(Deck.fireRequest(Data(#"{"id":"live:1","force":true,"removeWorktree":true}"#.utf8)))
        XCTAssertEqual(both.options, HireDesk.FireOptions(force: true, removeWorktree: true))
        // Only a real boolean counts.
        XCTAssertEqual(try XCTUnwrap(Deck.fireRequest(Data(#"{"id":"a","force":"yes"}"#.utf8))).options.force, false)
    }

    // MARK: The route and its fixture

    func testTheFireRouteIsTheManagersAndTheFixtureAgrees() throws {
        XCTAssertEqual(SetupRoutes.row("POST", "/setup/gm/fire")?.callers, [.gm, .page])
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("fixtures/frontdesk/fire.json")
        let json = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])
        XCTAssertEqual(json["route"] as? String, "POST /setup/gm/fire")
        XCTAssertEqual(json["callers"] as? [String], ["gm", "page"])
        let recap = try XCTUnwrap(json["recap"] as? [String: Any])
        XCTAssertEqual(recap["file"] as? String, "recaps/<hire id>.md")
        // The recap this code writes has every line the fixture lists.
        guard case .success(let fired) = fire() else { return XCTFail("refused") }
        let text = try String(contentsOfFile: fired.recap, encoding: .utf8)
        for line in try XCTUnwrap(recap["lines"] as? [String]) where !line.contains("<") {
            XCTAssertTrue(text.contains(line), line)
        }
    }

    /// A choice list with no numbers, whose footer says "Enter to select", is a question too.
    func testAnUnnumberedChoiceListIsAsking() {
        let list = "text\n\(HireFireTests.rule)\n Pick a model\n ❯ Option one\n   Option two\n Enter to select · ↑/↓ to navigate · Esc to go back\n"
        XCTAssertNotNil(HireDesk.promptRefusal(list))
        XCTAssertNotEqual(HireDesk.inputState(list), .idleAndEmpty)
    }

}
