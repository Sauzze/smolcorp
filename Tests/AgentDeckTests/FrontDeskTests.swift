import XCTest
@testable import AgentDeck

/// The front desk: the registry kept in a file, the team joined from it and
/// the sessions the deck follows, and what may be edited by hand. The wire
/// formats are the shared fixtures in Tests/fixtures/frontdesk/, which the
/// Linux and Windows app tests against too.
final class FrontDeskTests: XCTestCase {
    private var folder: URL!
    private let fixtures = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        .deletingLastPathComponent().appendingPathComponent("fixtures/frontdesk")

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    // MARK: Fixtures

    private func fixture(_ name: String) throws -> [String: Any] {
        let data = try Data(contentsOf: fixtures.appendingPathComponent(name))
        return try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
    }

    /// A front desk that starts from the registry fixture.
    private func deskWithFixtureRegistry() throws -> FrontDesk {
        try FileManager.default.copyItem(at: fixtures.appendingPathComponent("registry.json"),
                                         to: folder.appendingPathComponent("registry.json"))
        return FrontDesk(folder: folder)
    }

    private func live(_ json: [String: Any]) -> FrontDesk.Live {
        FrontDesk.Live(id: json["id"] as! String, agent: json["agent"] as! String, folder: json["folder"] as! String,
                       project: json["project"] as! String, state: json["state"] as! String, model: json["model"] as! String,
                       floor: json["floor"] as? Int, desk: json["desk"] as? Int, lastEventAt: (json["lastEventAt"] as! NSNumber).doubleValue)
    }

    private func hire(_ id: String, folder path: String, worktree: String = "", state: FrontDesk.State = .working) -> FrontDesk.Hire {
        FrontDesk.Hire(id: id, name: "Reviewer", role: "reviewer", folder: path, worktree: worktree, started: 1000, state: state)
    }

    private func session(_ id: String, folder path: String, project: String? = nil) -> FrontDesk.Live {
        FrontDesk.Live(id: id, agent: "claude", folder: path, project: project ?? path, state: "idle", floor: 0, desk: 0)
    }

    private func ids(_ rows: [[String: Any]]) -> [String] { rows.map { $0["id"] as! String } }

    // MARK: Wire formats

    func testTheRegistryFixtureLoadsAndIsWrittenBackAsItIs() throws {
        let desk = try deskWithFixtureRegistry()
        XCTAssertEqual(desk.hires.map(\.id), ["a1b2c3", "d4e5f6", "0a0b0c"])
        XCTAssertEqual(desk.hires[0].tmuxSocket, "smolcorp-rev")
        XCTAssertEqual(desk.hires[2].state, .fired)
        // A change saves the whole file in the fixture's shape.
        XCTAssertTrue(desk.move("d4e5f6", to: .idle))
        XCTAssertTrue((desk.registryJSON as NSDictionary).isEqual(to: try fixture("registry.json")))
        XCTAssertTrue(desk.move("d4e5f6", to: .working))
        let written = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: folder.appendingPathComponent("registry.json"))) as? [String: Any])
        XCTAssertEqual(written["version"] as? Int, 1)
        XCTAssertEqual((written["hires"] as? [[String: Any]])?[1]["state"] as? String, "working")
    }

    // MARK: Contacts and send keys (T403)

    private func contactsBody(_ object: [String: Any]) -> Data { try! JSONSerialization.data(withJSONObject: object) }

    func testContactsAreSetBothWaysOrOneWayAndRemovedAndKept() throws {
        let desk = try deskWithFixtureRegistry()
        let fixture = try self.fixture("contacts.json")
        for step in try XCTUnwrap(fixture["steps"] as? [[String: Any]]) {
            let response = desk.setContacts(body: contactsBody(try XCTUnwrap(step["request"] as? [String: Any])))
            XCTAssertEqual(response.status, step["status"] as? Int, "\(step["request"] ?? "")")
            if let want = step["answer"] as? NSDictionary {
                XCTAssertEqual(try JSONSerialization.jsonObject(with: response.body) as? NSDictionary, want, "\(step["request"] ?? "")")
            }
        }
        // Kept in the registry file, and read again; the hires are as they were.
        let again = FrontDesk(folder: folder)
        XCTAssertEqual(again.contacts, desk.contacts)
        XCTAssertEqual(again.hires, desk.hires)
        XCTAssertTrue(again.mayWrite(from: "a1b2c3", to: "d4e5f6"))
        XCTAssertFalse(again.mayWrite(from: "d4e5f6", to: "a1b2c3"), "the fixture ends with a one-way pair")
        XCTAssertEqual(try XCTUnwrap(desk.teamJSON(live: [])["contacts"] as? [[String: String]]), [["from": "a1b2c3", "to": "d4e5f6"]])
    }

    func testARegistryWithoutContactsIsWrittenWithoutThem() throws {
        let desk = try deskWithFixtureRegistry()
        XCTAssertTrue(desk.contacts.isEmpty)
        XCTAssertNil(desk.registryJSON["contacts"])
        XCTAssertNil(desk.registryJSON["keys"])
    }

    func testASendKeyIsKnownAsItsHireAndKeptOnlyAsAHash() throws {
        let desk = FrontDesk(folder: folder)
        XCTAssertTrue(desk.register(hire("h1", folder: "/a"), sendKey: "key-one"))
        XCTAssertTrue(desk.register(hire("h2", folder: "/b"), sendKey: "key-two"))
        XCTAssertTrue(desk.register(hire("h3", folder: "/c")))
        XCTAssertEqual(desk.hire(forSendKey: "key-one")?.id, "h1")
        XCTAssertEqual(desk.hire(forSendKey: "key-two")?.id, "h2")
        for wrong in [nil, "", "key", "key-one ", "KEY-ONE"] { XCTAssertNil(desk.hire(forSendKey: wrong), wrong ?? "nil") }
        let text = try String(contentsOf: folder.appendingPathComponent("registry.json"), encoding: .utf8)
        XCTAssertFalse(text.contains("key-one"), "only the hash is written")
        XCTAssertTrue(text.contains(FrontDesk.hash("key-one")))
        // After a restart it still works; once fired or failed, it doesn't.
        let again = FrontDesk(folder: folder)
        XCTAssertEqual(again.hire(forSendKey: "key-two")?.id, "h2")
        again.move("h2", to: .fired)
        again.fail("h1", reason: "x")
        XCTAssertNil(again.hire(forSendKey: "key-two"))
        XCTAssertNil(again.hire(forSendKey: "key-one"))
    }

    func testTheStartScriptCarriesTheKeyAndTheRegistryDoesNot() {
        let lines = HireDesk.scriptLines(ports: "", apiKey: false, sendKey: "k-1_a")
        XCTAssertEqual(lines, "export \(FrontDesk.sendKeyEnvironment)='k-1_a'\n")
        XCTAssertEqual(FrontDesk.sendKeyEnvironment, "SMOLCORP_HIRE_KEY")
        XCTAssertEqual(HireDesk.scriptLines(ports: "", apiKey: false), "")
    }

    func testTheTeamIsTheFixturesAnswer() throws {
        let desk = try deskWithFixtureRegistry()
        let team = try fixture("team.json")
        let input = try XCTUnwrap(team["input"] as? [String: Any])
        let status = try XCTUnwrap(input["status"] as? [String: String])
        let now = Date(timeIntervalSince1970: (input["now"] as! NSNumber).doubleValue / 1000)
        let answer = desk.teamJSON(live: (input["live"] as! [[String: Any]]).map(live), now: now) { status[$0] ?? "" }
        // Through JSON, as a client reads it.
        let sent = Response.json(answer)
        let received = try XCTUnwrap(JSONSerialization.jsonObject(with: sent.body) as? NSDictionary)
        XCTAssertEqual(received, try XCTUnwrap(team["answer"] as? NSDictionary))
    }

    func testAnEditIsTheFixturesAnswerAndItsRefusals() throws {
        let desk = try deskWithFixtureRegistry()
        let edit = try fixture("registry-edit.json")
        let answered = desk.edit(body: try JSONSerialization.data(withJSONObject: try XCTUnwrap(edit["request"])))
        XCTAssertEqual(answered.status, 200)
        let received = try XCTUnwrap(JSONSerialization.jsonObject(with: answered.body) as? NSDictionary)
        XCTAssertEqual(received, try XCTUnwrap(edit["answer"] as? NSDictionary))
        for refusal in try XCTUnwrap(edit["refusals"] as? [[String: Any]]) {
            let request = try XCTUnwrap(refusal["request"])
            let response = desk.edit(body: try JSONSerialization.data(withJSONObject: request))
            XCTAssertEqual(response.status, refusal["status"] as? Int, "\(request)")
        }
    }

    func testTheRouteTableNamesTheRoutesThatExist() throws {
        let routes = try XCTUnwrap(try fixture("routes.json")["routes"] as? [[String: Any]])
        // routes.json is the one route table (SetupRoutes); the front desk's two routes are in it.
        func callers(_ method: String, _ path: String) -> [String]? {
            routes.first { $0["method"] as? String == method && $0["path"] as? String == path }?["callers"] as? [String]
        }
        XCTAssertEqual(callers("GET", "/setup/gm/team"), ["gm", "page", "tool"])
        XCTAssertEqual(callers("POST", "/setup/gm/registry"), ["gm", "page"])
    }

    // MARK: The team's merge

    func testARegisteredHireWithItsLiveSessionIsOneRow() {
        let hired = hire("h1", folder: "/work/app", worktree: "/work/app-wt/rev")
        let rows = FrontDesk.team(hires: [hired], live: [session("s1", folder: "/work/app-wt/rev", project: "/work/app")], now: 0) { _ in "" }
        XCTAssertEqual(ids(rows), ["h1"])
        XCTAssertEqual(rows[0]["registered"] as? Bool, true)
        XCTAssertEqual(rows[0]["live"] as? Bool, true)
        XCTAssertEqual(rows[0]["sessionId"] as? String, "s1")
        XCTAssertEqual(rows[0]["floor"] as? Int, 0)
    }

    func testALiveSessionNoOneHiredStillAppearsWithItsDeskAndFloor() {
        var upstairs = session("s9", folder: "/work/tools")
        upstairs.floor = 2
        upstairs.desk = 3
        let rows = FrontDesk.team(hires: [hire("h1", folder: "/work/app")], live: [upstairs], now: 0) { _ in "" }
        XCTAssertEqual(ids(rows), ["h1", "live:s9"])
        XCTAssertEqual(rows[1]["registered"] as? Bool, false)
        XCTAssertEqual(rows[1]["live"] as? Bool, true)
        XCTAssertEqual(rows[1]["name"] as? String, "tools")
        XCTAssertEqual(rows[1]["floor"] as? Int, 2)
        XCTAssertEqual(rows[1]["desk"] as? Int, 3)
        XCTAssertEqual(rows[1]["state"] as? String, "", "it has no state of the registry's")
    }

    func testAHireWhoseSessionIsGoneStaysInTheTeamAsGone() {
        let rows = FrontDesk.team(hires: [hire("h1", folder: "/work/app")], live: [], now: 0) { _ in "" }
        XCTAssertEqual(ids(rows), ["h1"])
        XCTAssertEqual(rows[0]["live"] as? Bool, false)
        XCTAssertEqual(rows[0]["liveState"] as? String, "gone")
        XCTAssertEqual(rows[0]["state"] as? String, "working", "the registry's word stays")
        XCTAssertTrue(rows[0]["floor"] is NSNull)
        XCTAssertTrue(rows[0]["desk"] is NSNull)
    }

    func testAFiredOrFailedHireClaimsNoSession() {
        for state in [FrontDesk.State.fired, .failed] {
            let rows = FrontDesk.team(hires: [hire("h1", folder: "/work/app", state: state)], live: [session("s1", folder: "/work/app")], now: 0) { _ in "" }
            XCTAssertEqual(ids(rows), ["h1", "live:s1"], state.rawValue)
            XCTAssertEqual(rows[0]["live"] as? Bool, false)
        }
    }

    func testEachSessionAndEachHireIsClaimedOnce() {
        // Two sessions in one hire's folder: the first is its, the other is someone else's.
        let rows = FrontDesk.team(hires: [hire("h1", folder: "/work/app")],
                                  live: [session("s1", folder: "/work/app"), session("s2", folder: "/work/app")], now: 0) { _ in "" }
        XCTAssertEqual(ids(rows), ["h1", "live:s2"])
        XCTAssertEqual(rows[0]["sessionId"] as? String, "s1")
        // Two hires in one folder: each takes its own session.
        let two = FrontDesk.team(hires: [hire("h1", folder: "/work/app"), hire("h2", folder: "/work/app")],
                                 live: [session("s1", folder: "/work/app"), session("s2", folder: "/work/app")], now: 0) { _ in "" }
        XCTAssertEqual(two.map { $0["sessionId"] as? String }, ["s1", "s2"])
    }

    func testASessionMatchesByWorktreeFolderOrProjectAndNotByANeighbourFolder() {
        let hired = hire("h1", folder: "/work/app", worktree: "/work/app-wt/rev")
        for place in [("/work/app-wt/rev", "/x"), ("/work/app", "/x"), ("/x", "/work/app")] {
            let rows = FrontDesk.team(hires: [hired], live: [session("s1", folder: place.0, project: place.1)], now: 0) { _ in "" }
            XCTAssertEqual(ids(rows), ["h1"], "\(place)")
        }
        let apart = FrontDesk.team(hires: [hired], live: [session("s1", folder: "/work/app2")], now: 0) { _ in "" }
        XCTAssertEqual(ids(apart), ["h1", "live:s1"])
    }

    func testAnIdleTimeIsKnownOnlyOfASessionThatHasReported() {
        var heard = session("s1", folder: "/work/app")
        heard.lastEventAt = 100_000
        let rows = FrontDesk.team(hires: [], live: [heard, session("s2", folder: "/work/api")], now: 160_000) { _ in "" }
        XCTAssertEqual(rows[0]["idleFor"] as? Int, 60)
        XCTAssertTrue(rows[1]["idleFor"] is NSNull)
    }

    func testATimeIsAWholeNumberOfMilliseconds() throws {
        var heard = session("s1", folder: "/work/app")
        heard.lastEventAt = 1_793_524_230_038.1179
        let rows = FrontDesk.team(hires: [], live: [heard], now: 1_793_524_290_000) { _ in "" }
        let data = try JSONSerialization.data(withJSONObject: rows)
        XCTAssertTrue(rows[0]["lastEventAt"] is Int64)
        XCTAssertEqual(rows[0]["lastEventAt"] as? Int64, 1_793_524_230_038)
        XCTAssertFalse(String(decoding: data, as: UTF8.self).contains("."))
    }

    func testTheLastStatusLineIsReadFromTheFolder() throws {
        try "# Status\n\nWorking on login\n\n".write(to: folder.appendingPathComponent("STATUS.md"), atomically: true, encoding: .utf8)
        XCTAssertEqual(FrontDesk.statusLine(folder.path), "Working on login")
        XCTAssertEqual(FrontDesk.statusLine(folder.appendingPathComponent("nothing").path), "")
        XCTAssertEqual(FrontDesk.statusLine("relative"), "")
    }

    func testAStatusFileThatIsNotAPlainFileIsNotOpened() throws {
        let fifo = folder.appendingPathComponent("STATUS.md").path
        XCTAssertEqual(mkfifo(fifo, 0o600), 0)
        // Would hang on open if it were tried.
        XCTAssertEqual(FrontDesk.statusLine(folder.path), "")
    }

    func testOnlyTheEndOfABigStatusFileIsRead() throws {
        let big = String(repeating: "old line\n", count: 5000) + "Last words\n"
        try big.write(to: folder.appendingPathComponent("STATUS.md"), atomically: true, encoding: .utf8)
        XCTAssertEqual(FrontDesk.statusLine(folder.path), "Last words")
    }

    // MARK: Restart

    func testTheRegistrySurvivesARestart() throws {
        let first = FrontDesk(folder: folder)
        XCTAssertTrue(first.register(hire("h1", folder: "/work/app")))
        XCTAssertFalse(first.register(hire("h1", folder: "/work/other")), "an id is used once")
        XCTAssertTrue(first.move("h1", to: .idle))
        _ = try first.edit(["id": "h1", "nickname": "Rev", "notes": "Back after lunch."])

        let second = FrontDesk(folder: folder)
        XCTAssertEqual(second.hires, first.hires)
        XCTAssertEqual(second.hires.first?.nickname, "Rev")
        XCTAssertEqual(second.hires.first?.state, .idle)
        // Readable by you alone.
        let mode = try FileManager.default.attributesOfItem(atPath: first.registryFile.path)[.posixPermissions] as? Int
        XCTAssertEqual(mode, 0o600)
    }

    func testAFileThatCantBeReadIsSetAsideNotWipedAndTheDeskStartsEmpty() throws {
        try "{ not json".write(to: folder.appendingPathComponent("registry.json"), atomically: true, encoding: .utf8)
        let desk = FrontDesk(folder: folder)
        XCTAssertTrue(desk.hires.isEmpty)
        XCTAssertEqual(try String(contentsOf: folder.appendingPathComponent("registry.broken.json")), "{ not json")
        XCTAssertTrue(desk.register(hire("h1", folder: "/work/app")))
        XCTAssertEqual(FrontDesk(folder: folder).hires.count, 1)
    }

    func testAFileThatExistsButCantBeReadIsSetAsideNotOverwritten() throws {
        let file = folder.appendingPathComponent("registry.json")
        try #"{"version": 1, "hires": []}"#.write(to: file, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0], ofItemAtPath: file.path)
        let desk = FrontDesk(folder: folder)
        XCTAssertTrue(desk.hires.isEmpty)
        XCTAssertTrue(FileManager.default.fileExists(atPath: folder.appendingPathComponent("registry.broken.json").path))
        XCTAssertTrue(desk.register(hire("h1", folder: "/work/app")))
        XCTAssertEqual(FrontDesk(folder: folder).hires.count, 1)
    }

    func testARegistryThatLeavesOutTheOptionalKeysStillLoads() throws {
        try #"{"version": 1, "hires": [{"id": "h1", "name": "Reviewer", "folder": "/work/app"}]}"#
            .write(to: folder.appendingPathComponent("registry.json"), atomically: true, encoding: .utf8)
        let hires = FrontDesk(folder: folder).hires
        XCTAssertEqual(hires, [FrontDesk.Hire(id: "h1", name: "Reviewer", folder: "/work/app")])
        XCTAssertFalse(FileManager.default.fileExists(atPath: folder.appendingPathComponent("registry.broken.json").path))
    }

    func testAFileFromANewerAppIsNotOverwritten() throws {
        try #"{"version": 99, "hires": []}"#.write(to: folder.appendingPathComponent("registry.json"), atomically: true, encoding: .utf8)
        XCTAssertTrue(FrontDesk(folder: folder).hires.isEmpty)
        XCTAssertTrue(FileManager.default.fileExists(atPath: folder.appendingPathComponent("registry.broken.json").path))
    }

    // MARK: Edit

    func testOnlyTheNicknameStateAndNotesChange() throws {
        let desk = try deskWithFixtureRegistry()
        let before = desk.hires[0]
        let changed = try desk.edit(["id": "a1b2c3", "nickname": "  Ada ", "state": "idle", "notes": "Paused."])
        XCTAssertEqual(changed.nickname, "Ada", "trimmed")
        XCTAssertEqual(changed.state, .idle)
        XCTAssertEqual(changed.notes, "Paused.")
        var same = changed
        same.nickname = before.nickname
        same.state = before.state
        same.notes = before.notes
        XCTAssertEqual(same, before, "nothing else moved")
        XCTAssertEqual(FrontDesk(folder: folder).hires[0], changed)
        // A field alone changes just that field, and an empty nickname clears it.
        XCTAssertEqual(try desk.edit(["id": "a1b2c3", "nickname": ""]).notes, "Paused.")
        XCTAssertEqual(desk.hires[0].nickname, "")
    }

    func testAnEditToWhereAHireWorksIsRefusedWhole() throws {
        let desk = try deskWithFixtureRegistry()
        let before = desk.hires
        for field in ["folder", "branch", "worktree", "tmuxSocket", "account", "role", "name", "started", "model", "ports", "repo", "unknown"] {
            XCTAssertThrowsError(try desk.edit(["id": "a1b2c3", "notes": "sneaky", field: "x"]), field) { error in
                XCTAssertEqual((error as? FrontDesk.Refusal)?.status, 400, field)
            }
        }
        XCTAssertEqual(desk.hires, before, "even the allowed part of a refused edit is not applied")
        XCTAssertEqual(FrontDesk(folder: folder).hires, before)
    }

    func testEditRefusalsSayWhyAndChangeNothing() throws {
        let desk = try deskWithFixtureRegistry()
        let before = desk.hires
        let cases: [([String: Any], Int)] = [
            (["nickname": "x"], 400), (["id": ""], 400), (["id": "a1b2c3"], 400), (["id": 5, "notes": "x"], 400),
            (["id": "a1b2c3", "state": "retired"], 400), (["id": "a1b2c3", "state": 3], 400),
            (["id": "a1b2c3", "nickname": 7], 400), (["id": "a1b2c3", "nickname": String(repeating: "n", count: 41)], 400),
            (["id": "a1b2c3", "nickname": "two\nlines"], 400), (["id": "a1b2c3", "notes": String(repeating: "n", count: 2001)], 400),
            (["id": "a1b2c3", "notes": ["a"]], 400), (["id": "nobody", "notes": "x"], 404),
        ]
        for (json, status) in cases {
            XCTAssertThrowsError(try desk.edit(json), "\(json)") { error in
                XCTAssertEqual((error as? FrontDesk.Refusal)?.status, status, "\(json)")
            }
        }
        XCTAssertEqual(desk.edit(body: Data("[1]".utf8)).status, 400, "not an object")
        XCTAssertEqual(desk.edit(body: Data()).status, 400)
        XCTAssertEqual(desk.hires, before)
    }

    func testEveryStateCanBeSetByHand() throws {
        let desk = try deskWithFixtureRegistry()
        for state in FrontDesk.State.allCases {
            XCTAssertEqual(try desk.edit(["id": "a1b2c3", "state": state.rawValue]).state, state)
        }
    }

    // MARK: The gate

    /// The team is read by the GM, the page or a tool; an edit is the GM's or the page's (SetupRoutes).
    func testTheLockedStateAnswerNamesTheMacApp() {
        func gate(_ path: String) -> Response? {
            DeckRouter.setupGate(Request(method: "GET", path: path, query: [:], headers: [:], body: Data(), isLocal: true,
                                         isAddressedDirectly: true, remoteAddress: "127.0.0.1"),
                                 sessions: SetupSessions(), isSetupKey: { _ in false }).refusal
        }
        let state = gate("/setup/state")
        XCTAssertEqual(state?.status, 401)
        XCTAssertEqual(state?.headers[DeckRouter.appHeader], "mac")
        XCTAssertEqual(state?.body, Data(), "the body is as before")
        XCTAssertNil(gate("/setup/team")?.headers[DeckRouter.appHeader], "only the state names the app")
    }

    func testAToolMayReadTheTeamButNotEditTheRegistryAndNobodyMayDoEither() {
        let sessions = SetupSessions()
        let key = "the-setup-key-file"
        for (method, path, toolRefusal) in [("GET", "/setup/gm/team", nil), ("POST", "/setup/gm/registry", 403)] as [(String, String, Int?)] {
            XCTAssertFalse(SetupSessions.isPageOnly(method, path), "the GM may use \(path)")
            func request(_ headers: [String: String]) -> Request {
                Request(method: method, path: path, query: [:], headers: headers, body: Data(), isLocal: true,
                        isAddressedDirectly: true, remoteAddress: "127.0.0.1")
            }
            let asNobody = request([DeckRouter.setupHeader: "1"])
            XCTAssertEqual(DeckRouter.setupGate(asNobody, sessions: sessions, isSetupKey: { $0 == key }).refusal?.status, 401, "nobody")
            let gate = DeckRouter.setupGate(request([DeckRouter.setupHeader: "1", DeckRouter.setupKeyHeader: key]),
                                            sessions: sessions, isSetupKey: { $0 == key })
            XCTAssertEqual(gate.refusal?.status, toolRefusal, "a tool at \(path)")
            XCTAssertEqual(gate.caller, .tool)
        }
    }
}
