import XCTest
@testable import AgentDeck

/// The hire engine, with temp repos and homes and a stand-in tmux: nothing real is started.
final class HireDeskTests: XCTestCase {
    private var root: URL!
    private var home: URL!
    private var repo: String!
    private var account: ClaudeProfile!

    override func setUpWithError() throws {
        root = URL(fileURLWithPath: HireDesk.physical(NSTemporaryDirectory())).appendingPathComponent("hiredesk-\(UUID().uuidString)")
        home = root.appendingPathComponent("home")
        try FileManager.default.createDirectory(at: home.appendingPathComponent(".claude"), withIntermediateDirectories: true)
        try #"{"numStartups": 7, "cost": 1.50, "projects": {"/old": {"hasTrustDialogAccepted": true}}}"#
            .write(to: home.appendingPathComponent(".claude.json"), atomically: true, encoding: .utf8)
        account = ClaudeProfiles.defaultProfile(home: home)
        repo = root.appendingPathComponent("app").path
        try FileManager.default.createDirectory(atPath: repo + "/src", withIntermediateDirectories: true)
        try "x".write(toFile: repo + "/src/a.txt", atomically: true, encoding: .utf8)
        for arguments in [["init", "-q", "-b", "main"], ["add", "."], ["-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "first"]] {
            let done = HireDesk.git(repo, arguments)
            XCTAssertEqual(done?.status, 0, arguments.joined(separator: " "))
            if arguments[0] == "init", done?.status == 0 { TempRepo.stopSigning(repo) }  // only a repo that was made
        }
    }

    override func tearDownWithError() throws { try? FileManager.default.removeItem(at: root) }

    func testTheTempRepoDoesNotSign() {
        XCTAssertEqual(TempRepo.local("commit.gpgsign", in: repo), "false")
    }

    private func request(_ folder: String? = nil, apiKey: Bool = false, claude: Bool = true) -> HireDesk.Request {
        HireDesk.Request(hireID: "abcdef123456", name: "Rev iew", folder: folder ?? repo, claude: claude, account: account, apiKey: apiKey)
    }

    private func prepare(_ r: HireDesk.Request, taken: [String] = []) throws -> HireDesk.Ready {
        try HireDesk.prepare(r, taken: taken, home: home, portIsFree: { _ in true }, key: { Self.fakeKey })
    }

    /// Short and obviously fake; Claude Code records the last 20 characters.
    private static let fakeKey = "fake-key-0123456789ABCDEFGHIJ"
    private static let fakeTail = String(fakeKey.suffix(20))

    private static var role: Role {
        var role = Role.shipped[0]
        role.agent = Agent.claude.rawValue
        return role
    }

    private var trustFile: URL { home.appendingPathComponent(".claude.json") }

    // MARK: Worktree and trust

    func testARepoHireGetsItsOwnWorktreeBranchPortsAndTrust() throws {
        let ready = try prepare(request())
        XCTAssertEqual(ready.branch, "hire/rev-iew-abcdef")
        XCTAssertEqual(ready.worktree, root.path + "/app-hires/rev-iew-abcdef")
        XCTAssertEqual(ready.folder, ready.worktree)
        XCTAssertEqual(ready.ports, "4100-4103")
        XCTAssertTrue(FileManager.default.fileExists(atPath: ready.worktree + "/src/a.txt"))
        XCTAssertEqual(HireDesk.git(repo, ["branch", "--list", ready.branch])?.output.contains("hire/rev-iew-abcdef"), true)
        XCTAssertTrue(HireDesk.isTrusted(ready.worktree, file: trustFile))
        let after = try String(contentsOf: trustFile, encoding: .utf8)
        XCTAssertTrue(after.contains(#""cost": 1.50"#), "the rest of the file is as Claude Code wrote it")
        XCTAssertTrue(HireDesk.isTrusted("/old", file: trustFile))
    }

    func testAHireThatAsksForNoWorktreeWorksInTheFolderAsItIsButIsTrustedAndGivenPorts() throws {
        var asked = request()
        asked.worktree = false
        let ready = try prepare(asked)
        XCTAssertEqual(ready.folder, repo)
        XCTAssertEqual(ready.worktree, "")
        XCTAssertEqual(ready.branch, "")
        XCTAssertEqual(ready.ports, "4100-4103")
        XCTAssertEqual(ready.note, "", "it is a repo, so no \"not a repository\" note")
        XCTAssertFalse(FileManager.default.fileExists(atPath: root.path + "/app-hires"), "no worktree made")
        XCTAssertTrue(HireDesk.isTrusted(HireDesk.physical(repo), file: trustFile))
    }

    func testAFolderInsideTheRepoIsHiredIntoTheSameFolderOfTheWorktree() throws {
        let ready = try prepare(request(repo + "/src"))
        XCTAssertEqual(ready.folder, ready.worktree + "/src")
        XCTAssertEqual(ready.repo, HireDesk.physical(repo))
    }

    func testAConfiguredFolderHoldsTheWorktrees() throws {
        let ready = try HireDesk.prepare(request(), worktrees: root.path + "/trees", home: home, portIsFree: { _ in true })
        XCTAssertEqual(ready.worktree, root.path + "/trees/app/rev-iew-abcdef")
    }

    func testAFolderThatIsNotARepoIsHiredIntoAsItIsAndSaysSo() throws {
        let plain = root.appendingPathComponent("plain").path
        try FileManager.default.createDirectory(atPath: plain, withIntermediateDirectories: true)
        let ready = try prepare(request(plain))
        XCTAssertEqual(ready.folder, plain)
        XCTAssertEqual(ready.branch, "")
        XCTAssertEqual(ready.worktree, "")
        XCTAssertEqual(ready.note, HireDesk.notARepo)
        XCTAssertTrue(HireDesk.isTrusted(HireDesk.physical(plain), file: trustFile))
    }

    func testAnotherAgentIsGivenAWorktreeButNoTrustWrite() throws {
        let before = try Data(contentsOf: trustFile)
        let ready = try prepare(request(claude: false))
        XCTAssertFalse(ready.worktree.isEmpty)
        XCTAssertEqual(try Data(contentsOf: trustFile), before)
    }

    func testAConfigInAShapeWeDontKnowIsRefusedWithNothingMade() throws {
        for text in ["[]", "not json", #"{"projects": []}"#] {
            try text.write(to: trustFile, atomically: true, encoding: .utf8)
            XCTAssertThrowsError(try prepare(request()), text) { error in
                XCTAssertEqual((error as? FrontDesk.Refusal)?.message, HireDesk.trustRefusal.message)
            }
            XCTAssertEqual(try String(contentsOf: trustFile, encoding: .utf8), text, "the file is left as it was")
        }
        try FileManager.default.removeItem(at: trustFile)
        XCTAssertThrowsError(try prepare(request()))
        XCTAssertFalse(FileManager.default.fileExists(atPath: root.path + "/app-hires"), "no worktree litter")
    }

    func testTrustingChangesOnlyTheOneValue() throws {
        let text = """
        {
          "a": 1.0e2,
          "projects": {
            "/x": {"hasTrustDialogAccepted": false, "other": [1, 2]},
            "/y": {}
          }
        }
        """
        let result = String(decoding: try HireDesk.trusting("/x", in: Data(text.utf8)), as: UTF8.self)
        XCTAssertEqual(result, text.replacingOccurrences(of: #""hasTrustDialogAccepted": false"#, with: #""hasTrustDialogAccepted": true"#))
        let inserted = String(decoding: try HireDesk.trusting("/y", in: Data(text.utf8)), as: UTF8.self)
        XCTAssertTrue(inserted.contains("1.0e2"))
        let none = String(decoding: try HireDesk.trusting("/z", in: Data(#"{"a": 1}"#.utf8)), as: UTF8.self)
        XCTAssertEqual(none, #"{"projects": {"/z": {"hasTrustDialogAccepted": true}},"a": 1}"#)
    }

    func testAnotherAccountIsTrustedInItsOwnFile() throws {
        let work = root.appendingPathComponent("claude-work")
        try FileManager.default.createDirectory(at: work, withIntermediateDirectories: true)
        try "{}".write(to: work.appendingPathComponent(".claude.json"), atomically: true, encoding: .utf8)
        account = ClaudeProfile(name: "work", dir: work.path)
        let ready = try prepare(request())
        XCTAssertTrue(HireDesk.isTrusted(ready.worktree, file: work.appendingPathComponent(".claude.json")))
        XCTAssertFalse(HireDesk.isTrusted(ready.worktree, file: trustFile))
    }

    // MARK: apiKeyHelper

    func testAStrayApiKeyHelperIsRefusedAndNothingIsMade() throws {
        try FileManager.default.createDirectory(atPath: repo + "/.claude", withIntermediateDirectories: true)
        try #"{"apiKeyHelper": "echo sk-fake"}"#.write(toFile: repo + "/.claude/settings.local.json", atomically: true, encoding: .utf8)
        XCTAssertThrowsError(try prepare(request())) { error in
            XCTAssertEqual((error as? FrontDesk.Refusal)?.status, 409)
            XCTAssertTrue((error as? FrontDesk.Refusal)?.message.contains("apiKeyHelper") == true)
        }
        XCTAssertFalse(FileManager.default.fileExists(atPath: root.path + "/app-hires"))
        // The API-key account was picked: that is what it is for.
        XCTAssertNoThrow(try prepare(request(apiKey: true)))
    }

    func testAnAccountsOwnApiKeyHelperIsRefusedToo() throws {
        try #"{"apiKeyHelper": "x"}"#.write(to: home.appendingPathComponent(".claude/settings.json"), atomically: true, encoding: .utf8)
        XCTAssertThrowsError(try prepare(request()))
        XCTAssertNil(HireDesk.apiKeyHelper(in: [repo]))
    }

    // MARK: Ports

    func testPortsAreAFreeRangeThatNoOtherHireHolds() {
        XCTAssertEqual(HireDesk.allocatePorts(taken: [], isFree: { _ in true }), "4100-4103")
        XCTAssertEqual(HireDesk.allocatePorts(taken: ["4100-4103"], isFree: { _ in true }), "4104-4107")
        XCTAssertEqual(HireDesk.allocatePorts(taken: ["4102-4105", "junk"], isFree: { _ in true }), "4106-4109")
        XCTAssertEqual(HireDesk.allocatePorts(taken: [], isFree: { $0 != 4101 }), "4102-4105")
        XCTAssertNil(HireDesk.allocatePorts(taken: [], isFree: { _ in false }))
        XCTAssertNil(HireDesk.allocatePorts(taken: ["4100-4999"], isFree: { _ in true }))
    }

    func testThePortsAreInTheStartScriptAndTheKeyNeverIs() {
        XCTAssertEqual(HireDesk.scriptLines(ports: "", apiKey: false), "", "an unmanaged hire has no ports line")
        XCTAssertFalse(HireDesk.scriptLines(ports: "", apiKey: true).contains("SMOLCORP_HIRE_PORTS"))
        XCTAssertTrue(HireDesk.scriptLines(ports: "", apiKey: true).contains("ANTHROPIC_API_KEY"))
        let plain = HireDesk.scriptLines(ports: "4100-4103", apiKey: false)
        XCTAssertEqual(plain, "export SMOLCORP_HIRE_PORTS='4100-4103'\n")
        let keyed = HireDesk.scriptLines(ports: "4100-4103", apiKey: true)
        XCTAssertTrue(keyed.contains("security find-generic-password -s '\(SummaryKey.service)'"))
        XCTAssertTrue(keyed.contains("|| {"), "no key, no hire: it stops instead of using the subscription")
        XCTAssertFalse(keyed.contains("sk-ant"))
        let script = Self.role.startScript(firstMessage: "go", environment: plain)
        XCTAssertTrue(script.contains("SMOLCORP_HIRE_PORTS") && script.contains("exec "))
    }

    func testTheApiKeyAccountNeedsAKeyInTheKeychain() {
        XCTAssertEqual(HireDesk.apiKeyRefusal(picked: true, offered: false)?.status, 409)
        XCTAssertNil(HireDesk.apiKeyRefusal(picked: true, offered: true))
        XCTAssertNil(HireDesk.apiKeyRefusal(picked: false, offered: false))
    }

    func testOptionsAreChecked() throws {
        XCTAssertEqual(try HireDesk.HireOptions.checked(model: " claude-sonnet-5-5 ", nickname: "Rev", apiKey: true).get(),
                       HireDesk.HireOptions(model: "claude-sonnet-5-5", nickname: "Rev", apiKey: true))
        XCTAssertEqual(try HireDesk.HireOptions.checked(model: nil, nickname: nil, apiKey: nil).get(), HireDesk.HireOptions())
        for bad in ["a b", "x;rm", "--x y", String(repeating: "a", count: 61)] {
            XCTAssertThrowsError(try HireDesk.HireOptions.checked(model: bad, nickname: "", apiKey: false).get(), bad)
        }
        XCTAssertThrowsError(try HireDesk.HireOptions.checked(model: "", nickname: "a\nb", apiKey: false).get())
    }

    func testTheSetupRouteTakesTheOptionsAndTheApiKeyAccountNeedsNoName() throws {
        let role = Self.role
        let two = [ClaudeProfile(name: "default", dir: home.path + "/.claude"), ClaudeProfile(name: "work", dir: "/w")]
        let folder = home.appendingPathComponent("Projects/app")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        func ask(_ extra: [String: Any]) throws -> Result<BriefRoutes.HireRequest, BriefRoutes.Refusal> {
            let body = try JSONSerialization.data(withJSONObject: ["role": role.id, "folder": folder.path].merging(extra) { $1 })
            return BriefRoutes.hireRequest(body, roles: [role], home: home, accounts: two)
        }
        if case .success = try ask([:]) { XCTFail("several accounts and none named") }
        guard case .success(let keyed) = try ask(["apiKey": true, "model": "claude-opus-5-5", "nickname": "Ann"]) else { return XCTFail("api key") }
        XCTAssertEqual(keyed.options, HireDesk.HireOptions(model: "claude-opus-5-5", nickname: "Ann", apiKey: true))
        XCTAssertEqual(keyed.account, "")
        if case .success = try ask(["model": "a b", "account": "work"]) { XCTFail("bad model") }
    }

    // MARK: The API key's answer

    func testAnApiKeyHireHasItsKeysTailApprovedAndNothingElseChanged() throws {
        let before = try String(contentsOf: trustFile, encoding: .utf8)
        let ready = try prepare(request(apiKey: true))
        XCTAssertTrue(HireDesk.isApproved(Self.fakeTail, file: trustFile))
        XCTAssertEqual(Self.fakeTail.count, 20)
        let after = try String(contentsOf: trustFile, encoding: .utf8)
        XCTAssertFalse(after.contains(Self.fakeKey), "never the whole key")
        // Taking out the two edits leaves the file as it was.
        let old = try JSONSerialization.jsonObject(with: Data(before.utf8)) as? [String: Any] ?? [:]
        var new = try JSONSerialization.jsonObject(with: Data(after.utf8)) as? [String: Any] ?? [:]
        XCTAssertEqual((new["customApiKeyResponses"] as? [String: Any])?["approved"] as? [String], [Self.fakeTail])
        new["customApiKeyResponses"] = nil
        var projects = new["projects"] as? [String: Any] ?? [:]
        projects[ready.worktree] = nil
        new["projects"] = projects
        XCTAssertTrue(JSONText.same(old, new))
        XCTAssertTrue(after.contains(#""cost": 1.50"#))
    }

    func testAHireOnAClaudeAccountGetsNoKeyEntry() throws {
        var asked = false
        _ = try HireDesk.prepare(request(), home: home, portIsFree: { _ in true }, key: { asked = true; return Self.fakeKey })
        XCTAssertFalse(asked, "the Keychain isn't asked")
        XCTAssertFalse(try String(contentsOf: trustFile, encoding: .utf8).contains("customApiKeyResponses"))
        XCTAssertFalse(HireDesk.isApproved(Self.fakeTail, file: trustFile))
    }

    func testApprovingAddsToWhatIsThereAndKeepsTheRest() throws {
        func approve(_ text: String) throws -> String { String(decoding: try HireDesk.approving("T", in: Data(text.utf8)), as: UTF8.self) }
        XCTAssertEqual(try approve(#"{"a": 1, "customApiKeyResponses": {"approved": ["X"], "rejected": ["R"]}}"#),
                       #"{"a": 1, "customApiKeyResponses": {"approved": ["T","X"], "rejected": ["R"]}}"#)
        XCTAssertEqual(try approve(#"{"customApiKeyResponses": {"approved": [], "rejected": []}}"#),
                       #"{"customApiKeyResponses": {"approved": ["T"], "rejected": []}}"#)
        XCTAssertEqual(try approve(#"{"customApiKeyResponses": {"rejected": []}}"#),
                       #"{"customApiKeyResponses": {"approved": ["T"],"rejected": []}}"#)
        XCTAssertEqual(try approve(#"{"a": 1}"#), #"{"customApiKeyResponses": {"approved": ["T"]},"a": 1}"#)
        let there = #"{"customApiKeyResponses": {"approved": ["T"]}}"#
        XCTAssertEqual(try approve(there), there, "already approved: not touched")
    }

    func testAConfigWhoseApprovalsHaveAShapeWeDontKnowIsRefusedAndLeftAlone() throws {
        for text in [#"{"customApiKeyResponses": []}"#, #"{"customApiKeyResponses": {"approved": "x"}}"#, #"{"customApiKeyResponses": {"approved": {}}}"#] {
            try text.write(to: trustFile, atomically: true, encoding: .utf8)
            XCTAssertThrowsError(try prepare(request(apiKey: true)), text) { error in
                XCTAssertEqual((error as? FrontDesk.Refusal)?.message, HireDesk.trustRefusal.message)
            }
            XCTAssertEqual(try String(contentsOf: trustFile, encoding: .utf8), text, "the file is left as it was")
            XCTAssertFalse(FileManager.default.fileExists(atPath: root.path + "/app-hires"), "no worktree litter")
        }
    }

    func testAnApiKeyHireWithoutAReadableKeyIsRefusedWithNothingMade() throws {
        let before = try Data(contentsOf: trustFile)
        XCTAssertThrowsError(try HireDesk.prepare(request(apiKey: true), home: home, portIsFree: { _ in true }, key: { nil })) { error in
            XCTAssertEqual((error as? FrontDesk.Refusal)?.message, HireDesk.keyRefusal.message)
        }
        XCTAssertEqual(try Data(contentsOf: trustFile), before)
        XCTAssertFalse(FileManager.default.fileExists(atPath: root.path + "/app-hires"))
    }

    func testTheKeyApprovalIsWrittenThroughASymlinkedConfig() throws {
        let target = root.appendingPathComponent("dotfiles-claude.json")
        try FileManager.default.moveItem(at: trustFile, to: target)
        try FileManager.default.createSymbolicLink(at: trustFile, withDestinationURL: target)
        _ = try prepare(request(apiKey: true))
        XCTAssertEqual(try? FileManager.default.destinationOfSymbolicLink(atPath: trustFile.path), target.path, "still a link")
        XCTAssertTrue(HireDesk.isApproved(Self.fakeTail, file: target))
    }

    func testKeyTailIsTheLast20CharactersOfTheTrimmedKey() {
        XCTAssertEqual(HireDesk.keyTail("  fake-key-0123456789ABCDEFGHIJ\n"), "0123456789ABCDEFGHIJ")
        XCTAssertNil(HireDesk.keyTail(nil))
        XCTAssertNil(HireDesk.keyTail("  \n"))
    }

    // MARK: Unmanaged hires are as in 1.23

    func testWithoutTheManagerAHireMakesNothingAndTouchesNothing() throws {
        // No config to trust in, and a stray apiKeyHelper: both would refuse a managed hire.
        try FileManager.default.removeItem(at: trustFile)
        try FileManager.default.createDirectory(atPath: repo + "/.claude", withIntermediateDirectories: true)
        try #"{"apiKeyHelper": "/x"}"#.write(toFile: repo + "/.claude/settings.json", atomically: true, encoding: .utf8)
        XCTAssertThrowsError(try HireDesk.place(request(), managed: true, home: home, portIsFree: { _ in true }))
        let ready = try HireDesk.place(request(), managed: false, home: home, portIsFree: { _ in true })
        XCTAssertEqual(ready, HireDesk.Ready(folder: repo))
        XCTAssertFalse(FileManager.default.fileExists(atPath: root.path + "/app-hires"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: trustFile.path), "no .claude.json made or edited")
        XCTAssertEqual(HireDesk.git(repo, ["branch", "--list", "hire/*"])?.output, "")
    }

    func testAnUnmanagedHireAtAPinnedDeskStaysInThePinnedProject() throws {
        let pinned = repo + "/src"
        let ready = try HireDesk.place(request(pinned), managed: false, home: home, portIsFree: { _ in true })
        XCTAssertEqual(ready.folder, pinned)
        XCTAssertEqual(ready.worktree, "")
        XCTAssertEqual(ready.ports, "")
        XCTAssertEqual(HireDesk.git(repo, ["worktree", "list"])?.output.split(separator: "\n").count, 1)
    }

    func testAManagedHireIsPrepared() throws {
        let ready = try HireDesk.place(request(), managed: true, home: home, portIsFree: { _ in true })
        XCTAssertFalse(ready.worktree.isEmpty)
        XCTAssertEqual(ready.ports, "4100-4103")
    }

    func testTheWorktreeOptionIsReadFromARequest() throws {
        XCTAssertFalse(try HireDesk.HireOptions.checked(model: nil, nickname: nil, apiKey: nil).get().worktree)
        XCTAssertTrue(try HireDesk.HireOptions.checked(model: nil, nickname: nil, apiKey: nil, worktree: true).get().worktree)
    }

    func testTheApiKeyNoticeSaysTheHireCanReadTheKey() {
        XCTAssertTrue(HireDesk.apiKeyNotice.contains("The hire can read this key."))
    }

    // MARK: Brief files, a slow git, a symlinked config

    func testPreparingAWorktreeLeavesTheSharedExcludeListAlone() throws {
        let exclude = repo + "/.git/info/exclude"
        let before = try? String(contentsOfFile: exclude, encoding: .utf8)
        let ready = try prepare(request())
        _ = try prepare(HireDesk.Request(hireID: "fedcba654321", name: "Two", folder: repo, claude: true, account: account, apiKey: false))
        XCTAssertEqual(try? String(contentsOfFile: exclude, encoding: .utf8), before, "the user's checkout shares this file")
        XCTAssertEqual(HireDesk.git(ready.worktree, ["status", "--porcelain"])?.output, "")
    }

    func testAWorktreeThatGitStoppedHalfWayIsClearedAndTheHireRefused() throws {
        var calls: [[String]] = []
        // A stand-in git that makes the folder and the branch, then "times out" (no answer).
        let slow: HireDesk.Git = { directory, arguments in
            calls.append(arguments)
            if arguments.starts(with: ["worktree", "add"]) {
                try? FileManager.default.createDirectory(atPath: arguments[4], withIntermediateDirectories: true)
                _ = HireDesk.git(directory, ["branch", arguments[3]])
                return nil
            }
            return HireDesk.git(directory, arguments)
        }
        XCTAssertThrowsError(try HireDesk.prepare(request(), home: home, git: slow, portIsFree: { _ in true })) {
            XCTAssertEqual(($0 as? FrontDesk.Refusal)?.status, 409)
        }
        XCTAssertFalse(FileManager.default.fileExists(atPath: root.path + "/app-hires/rev-iew-abcdef"))
        XCTAssertEqual(HireDesk.git(repo, ["branch", "--list", "hire/*"])?.output, "")
        XCTAssertTrue(calls.contains { $0.starts(with: ["worktree", "prune"]) })
        XCTAssertGreaterThan(HireDesk.worktreeTimeout, Tool.timeout)
    }

    func testASymlinkedConfigIsWrittenThroughToItsTarget() throws {
        let target = root.appendingPathComponent("dotfiles/claude.json")
        try FileManager.default.createDirectory(at: target.deletingLastPathComponent(), withIntermediateDirectories: true)
        try FileManager.default.moveItem(at: trustFile, to: target)
        try FileManager.default.createSymbolicLink(at: trustFile, withDestinationURL: target)
        _ = try prepare(request())
        XCTAssertEqual(try? FileManager.default.destinationOfSymbolicLink(atPath: trustFile.path), target.path, "still a link")
        XCTAssertTrue(try String(contentsOf: target, encoding: .utf8).contains("hasTrustDialogAccepted"))
        XCTAssertTrue(try String(contentsOf: target, encoding: .utf8).contains(#""cost": 1.50"#))
    }

    func testTheRepoIsKeptOnTheRecordAndUsedToCleanUp() throws {
        let ready = try prepare(request(repo + "/src"))
        XCTAssertEqual(ready.repo, HireDesk.physical(repo))
        let desk = FrontDesk(folder: root)
        let hire = FrontDesk.Hire(id: "h", name: "R", folder: repo + "/src", branch: ready.branch, worktree: ready.worktree, repo: ready.repo)
        desk.register(hire)
        XCTAssertEqual(FrontDesk(folder: root).hires[0].repo, ready.repo)
        XCTAssertEqual(HireDesk.cleanUp(hire, reason: "x", tmux: nil, desk: desk), "")
        XCTAssertFalse(FileManager.default.fileExists(atPath: ready.worktree))
    }

    func testATeamRowMatchesASessionWhoseFolderIsWrittenWithoutThePrivatePrefix() throws {
        let physical = root.path + "/wt"
        try FileManager.default.createDirectory(atPath: physical, withIntermediateDirectories: true)
        let hire = FrontDesk.Hire(id: "h", name: "R", folder: "/elsewhere", worktree: HireDesk.physical(physical))
        let live = FrontDesk.Live(id: "s", agent: "claude", folder: physical, project: physical, state: "idle")
        let rows = FrontDesk.team(hires: [hire], live: [live], now: 0, status: { _ in "" })
        XCTAssertEqual(rows.count, 1)
        XCTAssertEqual(rows[0]["live"] as? Bool, true)
    }

    // MARK: Started or cleaned up

    private final class Clock {
        var now = Date(timeIntervalSince1970: 1_000)
    }

    func testAHireIsStartedWhenAliveAndHeardFrom() {
        let clock = Clock()
        var looks = 0
        let result = HireDesk.waitForStart(alive: { true }, heardFrom: { looks += 1; return looks == 3 }, within: 45, every: 1,
                                           now: { clock.now }, pause: { clock.now.addTimeInterval($0) })
        XCTAssertEqual(result, .started)
        XCTAssertEqual(clock.now.timeIntervalSince1970, 1_002)
    }

    func testAHireThatNeverReportsFailsAfterTheWait() {
        let clock = Clock()
        let result = HireDesk.waitForStart(alive: { true }, heardFrom: { false }, within: 45, every: 5, now: { clock.now }, pause: { clock.now.addTimeInterval($0) })
        XCTAssertEqual(result, .failed("no word from it in 45 seconds"))
        XCTAssertEqual(clock.now.timeIntervalSince1970, 1_045)
    }

    func testAHireWhoseSessionDiedFailsAtOnceEvenIfItWasHeardFrom() {
        let clock = Clock()
        XCTAssertEqual(HireDesk.waitForStart(alive: { false }, heardFrom: { true }, now: { clock.now }, pause: { _ in XCTFail("no waiting") }),
                       .failed("its tmux session ended before it reported"))
        // Not being able to ask tmux isn't a death.
        XCTAssertEqual(HireDesk.waitForStart(alive: { nil }, heardFrom: { true }), .started)
    }

    func testTheFirstHookMovesTheRightHire() {
        let hires = [
            FrontDesk.Hire(id: "a", name: "A", folder: "/p", worktree: "/p-hires/a", tmuxSocket: "/tmp/tmux-1/smolcorp-a"),
            FrontDesk.Hire(id: "b", name: "B", folder: "/p", worktree: "/p-hires/b", tmuxSocket: "/tmp/tmux-1/smolcorp-b"),
            FrontDesk.Hire(id: "c", name: "C", folder: "/q", state: .working),
        ]
        XCTAssertEqual(HireDesk.startingHire(folder: "/nowhere", tab: "tmux:%0@/tmp/tmux-1/smolcorp-b", among: hires)?.id, "b")
        XCTAssertEqual(HireDesk.startingHire(folder: "/p-hires/a/src", tab: "", among: hires)?.id, "a")
        XCTAssertNil(HireDesk.startingHire(folder: "/q", tab: "", among: hires), "already working")
        XCTAssertNil(HireDesk.startingHire(folder: "/p-hires/ab", tab: "", among: hires), "a folder beside, not inside")
    }

    func testAFailedHireIsCleanedUp() throws {
        let ready = try prepare(request())
        let desk = FrontDesk(folder: root)
        let hire = FrontDesk.Hire(id: "abcdef123456", name: "R", folder: repo, branch: ready.branch, worktree: ready.worktree, tmuxSocket: "/t/smolcorp-abcdef123456")
        desk.register(hire)
        var tmux: [[String]] = []
        let fake = HireTmux(hireID: "abcdef123456", testPort: nil) { tmux.append($0); return (0, "") }
        let left = HireDesk.cleanUp(hire, reason: "no word from it in 45 seconds", tmux: fake, desk: desk)
        XCTAssertEqual(left, "")
        XCTAssertTrue(tmux.contains(["-L", "smolcorp-abcdef123456", "kill-server"]))
        XCTAssertFalse(FileManager.default.fileExists(atPath: ready.worktree))
        XCTAssertEqual(HireDesk.git(repo, ["branch", "--list", ready.branch])?.output, "")
        XCTAssertEqual(desk.hires[0].state, .failed)
        XCTAssertEqual(desk.hires[0].notes, "Didn't start: no word from it in 45 seconds.")
        XCTAssertTrue(desk.portsInUse.isEmpty)
    }

    func testAWorktreeWithChangesIsKeptAndTheRegistrySaysSo() throws {
        let ready = try prepare(request())
        try "mine".write(toFile: ready.worktree + "/new.txt", atomically: true, encoding: .utf8)
        let desk = FrontDesk(folder: root)
        let hire = FrontDesk.Hire(id: "h", name: "R", folder: repo, branch: ready.branch, worktree: ready.worktree)
        desk.register(hire)
        let left = HireDesk.cleanUp(hire, reason: "x", tmux: nil, desk: desk)
        XCTAssertTrue(left.contains("was kept"))
        XCTAssertTrue(FileManager.default.fileExists(atPath: ready.worktree + "/new.txt"))
        XCTAssertTrue(desk.hires[0].notes.contains("was kept"))
    }

    func testPortsInUseLeaveOutFiredAndFailedHires() {
        let desk = FrontDesk(folder: root)
        desk.register(FrontDesk.Hire(id: "1", name: "A", folder: "/a", ports: "4100-4103"))
        desk.register(FrontDesk.Hire(id: "2", name: "B", folder: "/b", ports: "4104-4107"))
        desk.register(FrontDesk.Hire(id: "3", name: "C", folder: "/c", ports: "4108-4111"))
        desk.move("2", to: .fired)
        desk.fail("3", reason: "x")
        XCTAssertEqual(desk.portsInUse, ["4100-4103"])
        XCTAssertEqual(FrontDesk(folder: root).hires[0].ports, "4100-4103")
    }
}
