import XCTest
@testable import AgentDeck

/// Several Claude accounts: the list of them, the sessions seen on each, the
/// hooks written into each one's settings, and a hire naming its account.
/// Every test works in a home folder of its own, never yours. The wire
/// formats are in Tests/fixtures/frontdesk/profiles.json.
final class ClaudeAccountsTests: XCTestCase {
    private var home: URL!
    private let fixtures = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        .deletingLastPathComponent().appendingPathComponent("fixtures/frontdesk")

    override func setUpWithError() throws {
        home = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
            .resolvingSymlinksInPath()
        try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: home)
    }

    @discardableResult
    private func folder(_ name: String, files: [String: String] = [:]) throws -> URL {
        let dir = home.appendingPathComponent(name, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        for (file, text) in files {
            try FileManager.default.createDirectory(at: dir.appendingPathComponent(file).deletingLastPathComponent(), withIntermediateDirectories: true)
            try text.write(to: dir.appendingPathComponent(file), atomically: true, encoding: .utf8)
        }
        return dir
    }

    private func profile(_ name: String, _ folderName: String) -> ClaudeProfile {
        ClaudeProfile(name: name, dir: home.appendingPathComponent(folderName).path)
    }

    /// A fixture with {home} filled in.
    private func fixture() throws -> [String: Any] {
        var text = try String(contentsOf: fixtures.appendingPathComponent("profiles.json"), encoding: .utf8)
        text = text.replacingOccurrences(of: "{home}", with: home.path)
        return try XCTUnwrap(JSONSerialization.jsonObject(with: Data(text.utf8)) as? [String: Any])
    }

    private func settings(of profile: ClaudeProfile) throws -> [String: Any] { try Hooks.read(profile.settings) }

    // MARK: The list

    func testTheDefaultIsAlwaysFirstAndOthersNeedAFolder() throws {
        try folder(".claude-work")
        let list = ClaudeProfiles.all(saved: [profile("work", ".claude-work"), profile("gone", ".claude-gone")], home: home)
        XCTAssertEqual(list.map(\.name), ["default", "work"])
        XCTAssertEqual(list[0].dir, home.appendingPathComponent(".claude").path)
        XCTAssertEqual(ClaudeProfiles.all(saved: [], home: home).map(\.name), ["default"])
    }

    func testFoldersThatLookLikeAccountsAreOfferedNotUsed() throws {
        try folder(".claude")
        try folder(".claude-work", files: ["settings.json": "{}"])
        try folder(".claude-spare", files: [".claude.json": "{}"])
        try folder(".claude-empty")
        try folder("Projects", files: ["settings.json": "{}"])
        try "{}".write(to: home.appendingPathComponent(".claude.json"), atomically: true, encoding: .utf8)
        let found = ClaudeProfiles.found(home: home)
        XCTAssertEqual(found.map(\.name), ["spare", "work"])
        // Offered only: a list that nobody confirmed holds nothing.
        XCTAssertEqual(ClaudeProfiles.all(saved: [], home: home).map(\.name), ["default"])
        XCTAssertEqual(ClaudeProfiles.found(home: home, confirmed: [profile("w", ".claude-work")]).map(\.name), ["spare"])
    }

    func testTheListIsCheckedWhole() throws {
        try folder(".claude-work", files: ["settings.json": "{}"])
        try folder(".claude-spare", files: ["sessions/x.json": "{}"])
        try folder(".claude-bare")
        let ok = try ClaudeProfiles.checked([profile("work", ".claude-work"), profile("spare", ".claude-spare")], home: home).get()
        XCTAssertEqual(ok.map(\.name), ["work", "spare"])
        XCTAssertEqual(try ClaudeProfiles.checked([ClaudeProfile(name: "work", dir: "~/.claude-work")], home: home).get().first?.dir,
                       home.appendingPathComponent(".claude-work").path)
        let bad: [[ClaudeProfile]] = [
            [profile("default", ".claude-work")], // the name is taken
            [profile("work", ".claude")], // that folder is the default
            [profile("work", ".claude-work"), profile("work", ".claude-spare")], // a name twice
            [profile("work", ".claude-work"), profile("other", ".claude-work")], // a folder twice
            [profile("work", ".claude-gone")], // not a folder
            [profile("work", ".claude-bare")], // a folder, but not a Claude Code one
            [ClaudeProfile(name: "work", dir: ".claude-work")], // not a full path
            [profile("", ".claude-work")], [profile("a b", ".claude-work")],
            [profile(String(repeating: "x", count: 31), ".claude-work")],
        ]
        for list in bad { XCTAssertThrowsError(try ClaudeProfiles.checked(list, home: home).get(), "\(list)") }
        let many = (0...ClaudeProfiles.most).map { profile("a\($0)", ".claude-work") }
        XCTAssertThrowsError(try ClaudeProfiles.checked(many, home: home).get())
    }

    func testTheAnswerAndTheRefusalsMatchTheFixture() throws {
        try folder(".claude")
        try folder(".claude-work", files: ["settings.json": "{}"])
        try folder(".claude-spare", files: ["settings.json": "{}"])
        let fixture = try fixture()
        let accounts = ClaudeProfiles.all(saved: [profile("work", ".claude-work")], home: home)
        try Hooks.addClaude(to: accounts[0], turningOnVoice: false)
        let answer = ClaudeProfiles.json(accounts, home: home)
        XCTAssertTrue((answer as NSDictionary).isEqual(to: try XCTUnwrap(fixture["answer"] as? [String: Any])), "\(answer)")
        // The page sends the request in the fixture's shape, and it's kept.
        let set = try XCTUnwrap(fixture["set"] as? [String: Any])
        let requested = try XCTUnwrap((set["request"] as? [String: Any])?["profiles"] as? [[String: String]])
        XCTAssertEqual(try ClaudeProfiles.checked(requested.map { ClaudeProfile(name: $0["name"]!, dir: $0["dir"]!) }, home: home).get().count, 1)
        // Each refusal in the fixture is one, whatever the route makes of its words.
        for case let refusal as [String: Any] in set["refusals"] as? [Any] ?? [] {
            let list = ((refusal["request"] as? [String: Any])?["profiles"] as? [[String: String]]) ?? []
            if list.isEmpty { continue } // not a list at all: the route answers 400 before this check
            XCTAssertThrowsError(try ClaudeProfiles.checked(list.map { ClaudeProfile(name: $0["name"]!, dir: $0["dir"]!) }, home: home).get())
        }
    }

    func testConfigKeepsTheListAndDropsEntriesThatAreMalformed() throws {
        let file = home.appendingPathComponent("config.json")
        let config = Config()
        config.claudeProfiles = [profile("work", ".claude-work")]
        config.save(to: file)
        XCTAssertEqual(Config.load(from: file).claudeProfiles, [profile("work", ".claude-work")])
        let text = #"{"claudeProfiles": [{"name": "ok", "dir": "/tmp/x"}, {"name": "default", "dir": "/tmp/y"}, {"name": "rel", "dir": "x"}, {"name": 3}]}"#
        try text.write(to: file, atomically: true, encoding: .utf8)
        XCTAssertEqual(Config.load(from: file).claudeProfiles.map(\.name), ["ok"])
    }

    // MARK: Sessions

    func testSessionsAreSeenOnEveryAccount() throws {
        let one = #"{"pid": 11, "sessionId": "s-one", "name": "Rev", "messagingSocketPath": "/tmp/11.sock", "bridgeKey": "not-a-real-key"}"#
        let two = #"{"pid": 22, "sessionId": "s-two", "name": "Ops", "messagingSocketPath": "/tmp/22.sock"}"#
        let clash = #"{"pid": 33, "sessionId": "s-three", "name": "Rev"}"#
        try folder(".claude", files: ["sessions/11.json": one, "sessions/key.txt": "secret", "sessions/big.json": String(repeating: " ", count: 70_000)])
        try folder(".claude-work", files: ["sessions/22.json": two])
        let accounts = ClaudeProfiles.all(saved: [profile("work", ".claude-work")], home: home)
        let peers = ClaudePeers.all(in: accounts)
        XCTAssertEqual(peers.map(\.sessionId), ["s-one", "s-two"])
        XCTAssertEqual(peers.map(\.profile), ["default", "work"])
        XCTAssertEqual(ClaudePeers.sessionID(for: "Ops", among: peers), "s-two")
        XCTAssertEqual(ClaudePeers.sessionID(for: "uds:/tmp/22.sock", among: peers), "s-two")
        XCTAssertEqual(ClaudePeers.sessionID(for: "Rev [ab12]", among: peers), "s-one")
        XCTAssertNil(ClaudePeers.sessionID(for: "Nobody", among: peers))
        // The same name on two accounts names no one.
        try clash.write(to: home.appendingPathComponent(".claude-work/sessions/33.json"), atomically: true, encoding: .utf8)
        XCTAssertNil(ClaudePeers.sessionID(for: "Rev", among: ClaudePeers.all(in: accounts)))
    }

    // MARK: Hooks

    func testConnectingWritesToEveryAccountAndKeepsWhatIsThere() throws {
        try folder(".claude", files: ["settings.json": #"{"model": "opus"}"#])
        try folder(".claude-work", files: ["settings.json": #"{"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "/usr/bin/mine"}]}]}}"#])
        let accounts = ClaudeProfiles.all(saved: [profile("work", ".claude-work")], home: home)
        for account in accounts { try Hooks.addClaude(to: account, turningOnVoice: false) }
        XCTAssertTrue(accounts.allSatisfy(Hooks.isConnected))
        XCTAssertEqual(try settings(of: accounts[0])["model"] as? String, "opus")
        XCTAssertNil(try settings(of: accounts[0])["voice"], "voice stays as it was")
        let stop = try XCTUnwrap((try settings(of: accounts[1])["hooks"] as? [String: Any])?["Stop"] as? [[String: Any]])
        XCTAssertEqual(stop.count, 2, "your own hook and Smolcorp's")
        // Each file is backed up before its first change.
        XCTAssertTrue(FileManager.default.fileExists(atPath: Hooks.backups(of: accounts[1].settings).first.path))
    }

    func testDisconnectingOneAccountLeavesTheOthersAndYourHooks() throws {
        try folder(".claude", files: ["settings.json": "{}"])
        try folder(".claude-work", files: ["settings.json": #"{"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "/usr/bin/mine"}]}]}}"#])
        let accounts = ClaudeProfiles.all(saved: [profile("work", ".claude-work")], home: home)
        for account in accounts { try Hooks.addClaude(to: account, turningOnVoice: false) }
        XCTAssertTrue(try Hooks.removeClaude(from: accounts[1]))
        XCTAssertFalse(Hooks.hasEntries(accounts[1]))
        XCTAssertTrue(Hooks.isConnected(accounts[0]), "the other account stays connected")
        let left = try XCTUnwrap((try settings(of: accounts[1])["hooks"] as? [String: Any])?["Stop"] as? [[String: Any]])
        XCTAssertEqual((left.first?["hooks"] as? [[String: Any]])?.first?["command"] as? String, "/usr/bin/mine")
        // Nothing to take out is no change, and an account with no settings file is left without one.
        XCTAssertFalse(try Hooks.removeClaude(from: profile("none", ".claude-none")))
        XCTAssertFalse(FileManager.default.fileExists(atPath: home.appendingPathComponent(".claude-none").path))
    }

    func testAnAccountWhoseSettingsCantBeReadIsNotWrittenOver() throws {
        try folder(".claude-work", files: ["settings.json": "{not json"])
        let work = profile("work", ".claude-work")
        XCTAssertThrowsError(try Hooks.addClaude(to: work, turningOnVoice: false))
        XCTAssertEqual(try String(contentsOf: work.settings, encoding: .utf8), "{not json")
        XCTAssertFalse(Hooks.isConnected(work))
    }

    // MARK: Hiring

    func testAHireMustNameItsAccountWhenThereIsMoreThanOne() throws {
        let accounts = [ClaudeProfiles.defaultProfile(home: home), profile("work", ".claude-work")]
        for case let row as [String: Any] in try fixture()["hireRefusals"] as? [Any] ?? [] {
            let refusal = try XCTUnwrap(ClaudeProfiles.hireRefusal(account: row["account"] as? String, among: accounts))
            XCTAssertEqual(refusal.status, row["status"] as? Int)
            XCTAssertEqual(refusal.message, row["message"] as? String)
        }
        XCTAssertNil(ClaudeProfiles.hireRefusal(account: "work", among: accounts))
        XCTAssertNil(ClaudeProfiles.hireRefusal(account: nil, among: [accounts[0]]), "the only account needs no name")
        XCTAssertEqual(ClaudeProfiles.profile(named: "work", among: accounts)?.name, "work")
        XCTAssertNil(ClaudeProfiles.profile(named: nil, among: accounts))
    }

    func testTheHireRouteRefusesWithoutAnAccountAndAsksNoneOfOtherAgents() throws {
        try folder("Projects/app")
        let accounts = [ClaudeProfiles.defaultProfile(home: home), profile("work", ".claude-work")]
        var claude = try XCTUnwrap(Role.shipped.first)
        claude.agent = Agent.claude.rawValue
        var codex = claude
        codex.id = "codexer"
        codex.agent = Agent.codex.rawValue
        let request = { (role: Role, extra: [String: Any]) -> Result<BriefRoutes.HireRequest, BriefRoutes.Refusal> in
            let body = try! JSONSerialization.data(withJSONObject: ["role": role.id, "folder": self.home.appendingPathComponent("Projects/app").path].merging(extra) { $1 })
            return BriefRoutes.hireRequest(body, roles: [claude, codex], home: self.home, accounts: accounts)
        }
        guard case .failure(let refused) = request(claude, [:]) else { return XCTFail("a hire with no account") }
        XCTAssertEqual(refused.response.status, 400)
        XCTAssertTrue(String(decoding: refused.response.body, as: UTF8.self).contains("Say which Claude account"))
        guard case .failure(let unknown) = request(claude, ["account": "nope"]) else { return XCTFail("an account that isn't there") }
        XCTAssertEqual(unknown.response.status, 404)
        guard case .success(let named) = request(claude, ["account": "work"]) else { return XCTFail("a named account") }
        XCTAssertEqual(named.account, "work")
        guard case .success(let other) = request(codex, [:]) else { return XCTFail("another agent needs no account") }
        XCTAssertEqual(other.account, "")
        // The only account is the one a hire is on, named or not.
        let one = BriefRoutes.hireRequest(try JSONSerialization.data(withJSONObject: ["role": claude.id, "folder": home.appendingPathComponent("Projects/app").path]),
                                          roles: [claude], home: home, accounts: [accounts[0]])
        guard case .success(let alone) = one else { return XCTFail("one account") }
        XCTAssertEqual(alone.account, "")
    }

    func testAHiredAgentStartsInItsAccountsFolder() throws {
        var role = try XCTUnwrap(Role.shipped.first)
        role.agent = Agent.claude.rawValue
        let work = profile("work", ".claude-work")
        let script = role.startScript(firstMessage: "hello", account: work)
        XCTAssertTrue(script.contains("export CLAUDE_CONFIG_DIR='\(work.dir)'\nexec "), script)
        XCTAssertTrue(role.startScript(firstMessage: "hello", account: ClaudeProfiles.defaultProfile(home: home)).contains("unset CLAUDE_CONFIG_DIR\nexec "))
        XCTAssertFalse(role.startScript(firstMessage: "hello").contains("CLAUDE_CONFIG_DIR"), "no account named: as it was")
        role.agent = Agent.codex.rawValue
        XCTAssertFalse(role.startScript(firstMessage: "hello", account: work).contains("CLAUDE_CONFIG_DIR"), "other agents have none")
    }

    // MARK: Taking out and refreshing

    func testTakingOutWhatIsntThereLeavesTheFileAsItIs() throws {
        let odd = "{\n\"model\":   \"opus\",\n  \"hooks\" : {},\n\t\"zeta\": 1, \"alpha\": [1,2]\n}\n"
        let dir = try folder(".claude-work", files: ["settings.json": odd])
        let work = profile("work", ".claude-work")
        XCTAssertFalse(try Hooks.removeClaude(from: work))
        XCTAssertEqual(try String(contentsOf: work.settings, encoding: .utf8), odd, "byte for byte")
        let siblings = try FileManager.default.contentsOfDirectory(atPath: dir.path)
        XCTAssertEqual(siblings, ["settings.json"], "no backup made for nothing")
    }

    func testRefreshingKeepsADisconnectedAccountDisconnected() throws {
        try folder(".claude-work", files: ["settings.json": "{}"])
        try folder(".claude-spare", files: ["settings.json": "{}"])
        let work = profile("work", ".claude-work"), spare = profile("spare", ".claude-spare")
        try Hooks.addClaude(to: work, turningOnVoice: false)
        // One event short, as an older version left it.
        var older = try settings(of: work)
        var hooks = try XCTUnwrap(older["hooks"] as? [String: Any])
        hooks["Stop"] = nil
        older["hooks"] = hooks
        try JSONSerialization.data(withJSONObject: older).write(to: work.settings)
        XCTAssertFalse(Hooks.isConnected(work))
        Hooks.refreshClaude([work, spare])
        XCTAssertTrue(Hooks.isConnected(work), "a connected account is brought up to date")
        XCTAssertFalse(Hooks.hasEntries(spare), "a disconnected one stays so")
        XCTAssertEqual(try String(contentsOf: spare.settings, encoding: .utf8), "{}")
    }

    func testAnAddedAccountIsConnectedOnlyWhileConnected() throws {
        try folder(".claude", files: ["settings.json": "{}"])
        try folder(".claude-work", files: ["settings.json": "{}"])
        let work = profile("work", ".claude-work")
        Hooks.connectAdded([work], home: home)
        XCTAssertFalse(Hooks.hasEntries(work), "disconnected: the new account is left alone")
        try Hooks.addClaude(to: ClaudeProfiles.defaultProfile(home: home), turningOnVoice: false)
        Hooks.connectAdded([work], home: home)
        XCTAssertTrue(Hooks.isConnected(work), "connected: the new account joins")
    }

    // MARK: The gate

    func testOnlyThePageChangesTheAccounts() {
        for caller in [SetupSessions.Caller.tool, .gm] {
            XCTAssertFalse(SetupRoutes.allows(caller, "POST", "/setup/claude-accounts"), "\(caller) can't change them")
            XCTAssertTrue(SetupRoutes.allows(caller, "GET", "/setup/claude-accounts"), "\(caller) may read them")
        }
        XCTAssertTrue(SetupRoutes.allows(.page, "POST", "/setup/claude-accounts"))
    }
}
