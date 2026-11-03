import XCTest
@testable import AgentDeck

/// The hire guard's file and its two routes: the page switches groups and adds patterns, the guard (Go) reads the result.
final class GuardFileTests: XCTestCase {
    private var url: URL!

    override func setUpWithError() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("guard-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        url = dir.appendingPathComponent("hire-guard.json")
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: url.deletingLastPathComponent())
    }

    private func save(_ object: Any) -> Result<[String: Any], GuardFile.Refused> {
        GuardFile.save(try! JSONSerialization.data(withJSONObject: object), to: url)
    }

    private func refusal(_ object: Any, file: StaticString = #filePath, line: UInt = #line) -> String? {
        if case .failure(.bad(let why)) = save(object) { return why }
        XCTFail("was not refused", file: file, line: line)
        return nil
    }

    private func groups(_ state: [String: Any]) -> [[String: Any]] { state["groups"] as? [[String: Any]] ?? [] }

    // MARK: Who may use the routes

    func testAnyoneMayReadTheGuardButOnlyThePageChangesIt() {
        for caller in [SetupSessions.Caller.gm, .page, .tool] {
            XCTAssertTrue(SetupRoutes.allows(caller, "GET", "/setup/guard"), "\(caller.name) reads it")
        }
        XCTAssertTrue(SetupRoutes.allows(.page, "POST", "/setup/guard"))
        XCTAssertFalse(SetupRoutes.allows(.gm, "POST", "/setup/guard"), "the GM never loosens its own fence")
        XCTAssertFalse(SetupRoutes.allows(.tool, "POST", "/setup/guard"), "nor does a hire, with the setup key file")
        XCTAssertTrue(SetupSessions.isPageOnly("POST", "/setup/guard"))
    }

    // MARK: Reading

    func testWithoutAFileEveryGroupIsOnAndNothingIsWrong() {
        let state = GuardFile.state(url)
        XCTAssertEqual(groups(state).map { $0["name"] as? String }, ["push", "deploy", "publish", "cloud", "remote", "local"])
        XCTAssertTrue(groups(state).allSatisfy { $0["on"] as? Bool == true && !($0["about"] as? String ?? "").isEmpty })
        XCTAssertEqual(state["extra"] as? [String], [])
        XCTAssertNil(state["problem"])
        XCTAssertFalse((state["limits"] as? String ?? "").isEmpty)
        XCTAssertFalse(FileManager.default.fileExists(atPath: url.path), "reading writes nothing")
    }

    /// Off means allowed (m6): each group says what hires can do once it is off.
    func testEveryGroupSaysWhatItAllowsOnceOff() throws {
        let state = GuardFile.state(url)
        for group in groups(state) {
            let allows = try XCTUnwrap(group["allows"] as? String)
            XCTAssertFalse(allows.isEmpty)
            XCTAssertTrue(allows.hasSuffix("."))
        }
        XCTAssertEqual(Set(GuardFile.allows.keys), Set(GuardFile.groups.map(\.name)))
        XCTAssertTrue(GuardFile.allows["deploy"]?.hasPrefix("deploy") == true)
    }

    /// A fresh rules file (and a missing one) has every group on, the six the guard defaults to.
    func testAFreshRulesFileHasEveryGroupOn() throws {
        XCTAssertEqual(GuardFile.load(url).stored.on.count, GuardFile.groups.count)
        XCTAssertTrue(GuardFile.load(url).stored.on.values.allSatisfy { $0 })
        let data = GuardFile.data(GuardFile.load(url).stored)
        let list = try XCTUnwrap((JSONSerialization.jsonObject(with: data) as? [String: Any])?["groups"] as? [[String: Any]])
        XCTAssertTrue(list.allSatisfy { $0["on"] as? Bool == true })
    }

    func testAFileThatIsntAGuardFileReadsAsTheDefaultWithAProblem() throws {
        for text in ["", "nope", "{}", #"{"groups": []}"#, #"{"groups": [{"name": "push"}]}"#, #"{"groups": [{"name": "push", "on": true, "extra": 1}]}"#,
                     #"{"groups": [{"name": "push", "on": true, "commands": [" "]}]}"#] {
            try Data(text.utf8).write(to: url)
            let state = GuardFile.state(url)
            XCTAssertNotNil(state["problem"], text)
            XCTAssertTrue(groups(state).allSatisfy { $0["on"] as? Bool == true }, text)
        }
    }

    // MARK: Writing

    func testSwitchingAGroupOffKeepsTheRestAndTheFileIsTheGuardsFormat() throws {
        guard case .success(let state) = save(["groups": [["name": "cloud", "on": false]]]) else { return XCTFail("refused") }
        XCTAssertEqual(groups(state).filter { $0["on"] as? Bool == false }.map { $0["name"] as? String }, ["cloud"])
        let file = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])
        let list = try XCTUnwrap(file["groups"] as? [[String: Any]])
        XCTAssertEqual(list.map { $0["name"] as? String }, ["push", "deploy", "publish", "cloud", "remote", "local", "extra"])
        XCTAssertEqual(Set(list.flatMap { $0.keys }), ["name", "on", "commands"])
        let push = try XCTUnwrap(list.first { $0["name"] as? String == "push" })
        XCTAssertEqual(push["commands"] as? [String], GuardFile.groups[0].commands)
    }

    func testYourPatternsAreAddedTrimmedWithoutRepeatsAndRemoved() throws {
        guard case .success(let added) = save(["extra": ["  terraform import ", "terraform import", "helm"]]) else { return XCTFail("refused") }
        XCTAssertEqual(added["extra"] as? [String], ["terraform import", "helm"])
        guard case .success(let removed) = save(["extra": ["helm"]]) else { return XCTFail("refused") }
        XCTAssertEqual(removed["extra"] as? [String], ["helm"])
        // A change to the groups alone keeps the patterns
        guard case .success(let switched) = save(["groups": [["name": "push", "on": false]]]) else { return XCTFail("refused") }
        XCTAssertEqual(switched["extra"] as? [String], ["helm"])
    }

    func testAnUnknownGroupIsRefusedAndChangesNothing() {
        XCTAssertEqual(refusal(["groups": [["name": "push", "on": false], ["name": "nuclear", "on": false]]]), "There is no group called nuclear.")
        XCTAssertFalse(FileManager.default.fileExists(atPath: url.path))
        XCTAssertNotNil(refusal(["groups": [["name": "extra", "on": false]]]), "your patterns can't be switched off from the groups")
        XCTAssertNotNil(refusal(["groups": [["name": "push", "on": false], ["name": "push", "on": true]]]), "a group twice")
    }

    func testAPatternThatWouldMatchEverythingOrNothingIsRefused() {
        for bad in ["", "   ", "*", "* push", ".", "...", "@remote", "-f", "|", "git|", "|git", "a b\nc", "git\tpush\u{7}", String(repeating: "x", count: 201)] {
            XCTAssertNotNil(refusal(["extra": [bad]]), "\(bad.debugDescription)")
        }
        XCTAssertFalse(FileManager.default.fileExists(atPath: url.path), "nothing was written")
        for good in ["terraform import", "helm|kubectl install", "git push|send-pack", "rsync @remote", "tmux ... kill-session", "gh api -X|--method", "docker-compose up"] {
            XCTAssertNil(GuardFile.problem(withPattern: good), good)
        }
    }

    func testTooManyPatternsAndBadShapesAreRefused() {
        XCTAssertNotNil(refusal(["extra": (0...GuardFile.maxExtra).map { "tool\($0)" }]))
        XCTAssertNotNil(refusal(["extra": "helm"]))
        XCTAssertNotNil(refusal(["extra": [3]]))
        XCTAssertNotNil(refusal(["groups": "push"]))
        XCTAssertNotNil(refusal(["groups": [["name": "push"]]]))
        XCTAssertNotNil(refusal(["flavour": "mint"]))
        if case .failure = GuardFile.save(Data("not json".utf8), to: url) {} else { XCTFail("not json was kept") }
    }

    func testAGroupYouAddedByHandAndYourEditedCommandsAreKept() throws {
        let text = #"{"groups": [{"name": "push", "on": true, "commands": ["git push"]}, {"name": "mine", "on": true, "commands": ["make deploy"]}]}"#
        try Data(text.utf8).write(to: url)
        guard case .success = save(["groups": [["name": "push", "on": false]]]) else { return XCTFail("refused") }
        let list = try XCTUnwrap((JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])?["groups"] as? [[String: Any]])
        XCTAssertEqual(list.first { $0["name"] as? String == "push" }?["commands"] as? [String], ["git push"])
        XCTAssertEqual(list.first { $0["name"] as? String == "push" }?["on"] as? Bool, false)
        XCTAssertEqual(list.first { $0["name"] as? String == "mine" }?["commands"] as? [String], ["make deploy"])
    }

    func testABrokenFileIsReplacedByAGoodOneOnSave() throws {
        try Data("{broken".utf8).write(to: url)
        guard case .success(let state) = save(["groups": [["name": "remote", "on": false]]]) else { return XCTFail("refused") }
        XCTAssertNil(state["problem"])
        XCTAssertNil(GuardFile.load(url).problem)
    }

    func testTheWriteIsWholeAndPrivateAndLeavesNoTemporaryFile() throws {
        guard case .success = save(["extra": ["helm"]]) else { return XCTFail("refused") }
        let left = try FileManager.default.contentsOfDirectory(atPath: url.deletingLastPathComponent().path)
        XCTAssertEqual(left, ["hire-guard.json"])
        let mode = try FileManager.default.attributesOfItem(atPath: url.path)[.posixPermissions] as? Int
        XCTAssertEqual(mode, 0o600)
    }

    // MARK: Shared with the Go guard

    /// `Tests/fixtures/frontdesk/hire-guard.json` is what this side writes for "cloud off, one pattern of your own"; the Go
    /// guard's test reads it. Set SMOLCORP_WRITE_FIXTURES=1 for a run to write it again.
    func testTheFixtureIsWhatThisSideWrites() throws {
        guard case .success = save(["groups": [["name": "cloud", "on": false]], "extra": ["terraform import"]]) else { return XCTFail("refused") }
        let written = try Data(contentsOf: url)
        let fixture = FrontDeskFixtures.url("hire-guard.json")
        if ProcessInfo.processInfo.environment["SMOLCORP_WRITE_FIXTURES"] == "1" { try written.write(to: fixture) }
        XCTAssertEqual(try Data(contentsOf: fixture), written, "hire-guard.json is out of date: run once with SMOLCORP_WRITE_FIXTURES=1")
    }
}
