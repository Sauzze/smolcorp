import XCTest
@testable import AgentDeck

/// A GM started by hand is known by its session (T747): the GM token plus the id of a live session, with
/// by-hand on. It gets the gm role in memory only, and a hired GM wins.
final class ByHandGMSessionTests: XCTestCase {
    private var wasByHand = false

    private var sessionFile: URL { Paths.support.appendingPathComponent("gm-session") }

    override func setUp() {
        wasByHand = Config.shared.gmByHand
        Config.shared.gmByHand = true
        // The tests share one temp support folder: no mark left by an earlier test.
        try? FileManager.default.removeItem(at: sessionFile)
    }

    override func tearDown() {
        Config.shared.gmByHand = wasByHand
        try? FileManager.default.removeItem(at: sessionFile)
    }

    private func quietDeck() -> Deck {
        let deck = Deck(quiet: true)
        deck.layoutConfig = Config()
        return deck
    }

    private var clock = 10.0
    private func report(_ deck: Deck, _ id: String, tab: String? = nil) {
        clock += 1
        deck.sessions.apply(["hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": "/work/\(id)"],
                            agent: .claude, tab: tab ?? "tty:/dev/\(id)", startedIn: "", at: clock)
    }

    /// The GM's token worked (as the gate saw it), then its request named a session.
    private func asGM(_ deck: Deck, names session: String?) throws {
        let token = try XCTUnwrap(deck.access.gm.token())
        XCTAssertTrue(deck.access.isGMToken(token))
        deck.noteGM(session: session)
    }

    func testAKnownSessionWithTheValidTokenIsTheGM() throws {
        let deck = quietDeck()
        report(deck, "gm1"); report(deck, "dev1")
        try asGM(deck, names: "gm1")
        XCTAssertEqual(deck.roleID(of: "gm1"), "gm")
        XCTAssertNil(deck.roleID(of: "dev1"))
        let state = DeckRouter.managerState(enabled: true, byHand: true, startedByHand: deck.access.gm.startedByHand)
        XCTAssertEqual(state["gmStartedByHand"] as? Bool, true)
        let rows = deck.frontDesk.teamJSON(live: deck.frontDeskLive(), byHandGM: deck.byHandGMSession)["team"] as? [[String: Any]]
        XCTAssertEqual(rows?.first { $0["sessionId"] as? String == "gm1" }?["role"] as? String, "gm")
        XCTAssertEqual(rows?.first { $0["sessionId"] as? String == "dev1" }?["role"] as? String, "")
    }

    func testNoRoleFromAnUnknownIdNoHeaderOrAMalformedOne() throws {
        let deck = quietDeck()
        report(deck, "gm1")
        for header in ["nobody", nil, "", "gm1 ", "gm1\nx"] as [String?] {
            try asGM(deck, names: header)
            XCTAssertNil(deck.roleID(of: "gm1"), header ?? "nil")
            XCTAssertNil(deck.byHandGMSession)
        }
    }

    func testNoRoleWithAWrongTokenOrWithByHandOff() throws {
        let deck = quietDeck()
        report(deck, "gm1")
        // The router names a session only for the GM caller; a wrong token never makes one.
        XCTAssertFalse(deck.access.isGMToken(String(repeating: "z", count: 43)))
        XCTAssertNil(deck.roleID(of: "gm1"))
        // Without a token that worked, naming a session remembers nothing.
        deck.noteGM(session: "gm1")
        XCTAssertNil(deck.roleID(of: "gm1"))
        // By-hand off: the token works but there is no by-hand GM.
        Config.shared.gmByHand = false
        try asGM(deck, names: "gm1")
        XCTAssertNil(deck.roleID(of: "gm1"))
        // And turning it on later doesn't bring back what was refused.
        Config.shared.gmByHand = true
        XCTAssertNil(deck.roleID(of: "gm1"))
    }

    func testRotateByHandOffAndAFireForgetTheRole() throws {
        let deck = quietDeck()
        report(deck, "gm1")
        try asGM(deck, names: "gm1")
        XCTAssertEqual(deck.roleID(of: "gm1"), "gm")
        _ = deck.access.gm.rotate()
        XCTAssertNil(deck.roleID(of: "gm1"))

        try asGM(deck, names: "gm1")
        Config.shared.gmByHand = false
        deck.access.gm.applySetting()
        Config.shared.gmByHand = true
        XCTAssertNil(deck.roleID(of: "gm1"))

        try asGM(deck, names: "gm1")
        deck.access.gm.forgetStartedByHand()
        XCTAssertNil(deck.roleID(of: "gm1"))
    }

    func testAHiredGMWinsAndThereAreNeverTwo() {
        let hired = FrontDesk.Hire(id: "h1", name: "GM", role: "gm", folder: "/work/gm", started: 1000, state: .working)
        let fired = FrontDesk.Hire(id: "h2", name: "GM", role: "gm", folder: "/work/gm", started: 1000, state: .fired)
        XCTAssertEqual(Deck.byHandGM(remembered: "s1", hires: [], live: ["s1"]), "s1")
        XCTAssertEqual(Deck.byHandGM(remembered: "s1", hires: [fired], live: ["s1"]), "s1", "a fired hire doesn't count")
        XCTAssertNil(Deck.byHandGM(remembered: "s1", hires: [hired], live: ["s1"]))
        XCTAssertNil(Deck.byHandGM(remembered: "s1", hires: [], live: []), "an ended session")
        XCTAssertNil(Deck.byHandGM(remembered: nil, hires: [], live: ["s1"]))
        // A hired GM's own row keeps its registered role.
        let team = FrontDesk.team(hires: [hired], live: [], now: 0, byHandGM: "s1") { _ in "" }
        XCTAssertEqual(team.filter { $0["role"] as? String == "gm" }.count, 1)
    }

    func testTheRoleIsNeverSavedAndFollowsClear() throws {
        let deck = quietDeck()
        report(deck, "gm1")
        try asGM(deck, names: "gm1")
        _ = deck.layout()
        let saved = try JSONDecoder().decode(Deck.SavedDeck.self, from: JSONEncoder().encode(deck.savedDeck()))
        XCTAssertNil(saved.roles?["gm1"], "not saved as a role")
        XCTAssertFalse(String(decoding: try JSONEncoder().encode(deck.savedDeck()), as: UTF8.self).contains("\"gm\""))
        // /clear: a new session in the same tab takes the place of the old one.
        report(deck, "gm2", tab: "tty:/dev/gm1")
        _ = deck.layout()
        XCTAssertNil(deck.sessions["gm1"])
        XCTAssertEqual(deck.roleID(of: "gm2"), "gm")
        XCTAssertNil(deck.roleID(of: "gm1"))
        // After a restart nothing is known until the GM's next request.
        let again = quietDeck()
        again.putBack(saved, running: saved.sessions)
        XCTAssertNil(again.roleID(of: "gm1"))
    }

    func testAHireKeyOrTheSetupKeyNeverMarksAnyone() throws {
        let deck = quietDeck()
        report(deck, "dev1")
        // The router calls noteGM only for the GM caller (a token); the gate gives .gm to nothing else.
        let request = Request.addressed(method: "GET", path: "/setup/gm/team", query: [:],
                                        headers: [FrontDesk.sendKeyHeader: "fake-hire-key", GMIdentity.sessionHeader: "dev1"],
                                        body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1", repeatedHeaders: [])
        let gate = DeckRouter.setupGate(request, sessions: deck.access.setupSessions, isSetupKey: deck.access.isSetupKey,
                                        isGMToken: deck.access.isGMToken, isHireKey: { $0 == "fake-hire-key" })
        XCTAssertNotEqual(gate.caller, .gm)
        XCTAssertNil(deck.roleID(of: "dev1"))
    }

    /// A request that came before the session was live, or before the token worked, leaves nothing to come true later.
    func testNothingRefusedComesBackLater() throws {
        let deck = quietDeck()
        let token = try XCTUnwrap(deck.access.gm.token())
        deck.noteGM(session: "late") // the token has not worked yet, and the session is not there
        XCTAssertTrue(deck.access.isGMToken(token))
        report(deck, "late")
        XCTAssertNil(deck.roleID(of: "late"), "remembered before the token worked")

        let other = quietDeck()
        let key = try XCTUnwrap(other.access.gm.token())
        XCTAssertTrue(other.access.isGMToken(key))
        other.noteGM(session: "ahead") // not a live session
        report(other, "ahead")
        XCTAssertNil(other.roleID(of: "ahead"), "named before it was there")
    }

    // MARK: Through the router

    private func ask(_ router: DeckRouter, _ headers: [String: String], path: String = "/setup/gm/team") -> [String: Any]? {
        let request = Request.addressed(method: "GET", path: path, query: [:], headers: headers.merging(["host": "127.0.0.1:7710"]) { a, _ in a },
                                        body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")
        var answer: [String: Any]?
        router.handle(request) { reply in
            if case .response(let response) = reply, let json = try? JSONSerialization.jsonObject(with: response.body) { answer = json as? [String: Any] }
        }
        return answer
    }

    private func role(_ team: [String: Any]?, of session: String) -> String? {
        (team?["team"] as? [[String: Any]])?.first { $0["sessionId"] as? String == session }?["role"] as? String
    }

    func testTheGMsRequestsMarkItsSessionAndNothingElseDoes() throws {
        let wasManager = Config.shared.managerEnabled
        Config.shared.managerEnabled = true
        defer { Config.shared.managerEnabled = wasManager }
        let deck = quietDeck()
        report(deck, "gm1"); report(deck, "dev1")
        let router = DeckRouter(deck: deck)
        let token = try XCTUnwrap(deck.access.gm.token())
        let tokenHeader = GMIdentity.header, sessionHeader = GMIdentity.sessionHeader

        // A hire's key, the setup key and a wrong token name a session and change nothing.
        XCTAssertTrue(deck.access.isGMToken(token)) // the token has worked before (by hand is on), so only who is asking now matters
        let owner = ask(router, [sessionHeader: "gm1", DeckRouter.setupKeyHeader: deck.access.setupKey])
        XCTAssertEqual(role(owner, of: "gm1"), "", "the setup key's request was answered, and named no GM")
        XCTAssertNil(deck.roleID(of: "gm1"))
        _ = ask(router, [tokenHeader: String(repeating: "q", count: 43), sessionHeader: "gm1"])
        XCTAssertNil(deck.roleID(of: "gm1"))
        // The token with no header, or one naming no live session.
        _ = ask(router, [tokenHeader: token])
        _ = ask(router, [tokenHeader: token, sessionHeader: "ghost"])
        XCTAssertNil(deck.roleID(of: "gm1"))
        // The token with a live session's id.
        let team = ask(router, [tokenHeader: token, sessionHeader: "gm1"])
        XCTAssertEqual(deck.roleID(of: "gm1"), "gm")
        XCTAssertNil(deck.roleID(of: "dev1"))
        XCTAssertEqual(role(team, of: "gm1"), "gm", "the team the same request reads already says so")
        XCTAssertEqual(role(ask(router, [tokenHeader: token]), of: "dev1"), "")
    }

    // MARK: After a restart (T748)

    /// A new deck on the same support folder, as after a restart: the token is the same, the launch applies the setting.
    private func restarted() -> Deck {
        let deck = quietDeck()
        deck.access.gm.applySetting()
        return deck
    }

    func testTheRoleComesBackAfterARestartWithNoGMRequest() throws {
        let first = quietDeck()
        report(first, "gm1")
        try asGM(first, names: "gm1")

        let deck = restarted()
        XCTAssertNil(deck.roleID(of: "gm1"), "not live yet: nothing is shown")
        XCTAssertFalse(deck.access.gm.startedByHand)
        report(deck, "gm1")
        XCTAssertEqual(deck.roleID(of: "gm1"), "gm")
        XCTAssertTrue(deck.access.gm.startedByHand)
        XCTAssertEqual(deck.byHandGMSession, "gm1")
    }

    func testTheRoleIsThereAtOnceWhenTheSessionIsLiveAtLaunch() throws {
        let first = quietDeck()
        report(first, "gm1")
        try asGM(first, names: "gm1")
        let deck = quietDeck()
        report(deck, "gm1")
        deck.access.gm.applySetting()
        XCTAssertTrue(deck.access.gm.startedByHand)
        XCTAssertEqual(deck.roleID(of: "gm1"), "gm")
    }

    func testAnotherSessionInTheSameFolderNeverGetsTheRoleAfterARestart() throws {
        let first = quietDeck()
        report(first, "gm1")
        try asGM(first, names: "gm1")

        let deck = restarted()
        let sameFolder = ["hook_event_name": "UserPromptSubmit", "session_id": "dev1", "cwd": "/work/gm1"]
        deck.sessions.apply(sameFolder, agent: .claude, tab: "tty:/dev/dev1", startedIn: "", at: 50)
        XCTAssertNil(deck.roleID(of: "dev1"))
        XCTAssertNil(deck.byHandGMSession)
        XCTAssertFalse(deck.access.gm.startedByHand)
    }

    func testASessionThatEndedKeepsTheFileAndComesBack() throws {
        let first = quietDeck()
        report(first, "gm1")
        try asGM(first, names: "gm1")
        let deck = restarted()
        XCTAssertNil(deck.byHandGMSession)
        XCTAssertTrue(FileManager.default.fileExists(atPath: sessionFile.path), "kept: a session can come back")
        report(deck, "gm1")
        XCTAssertEqual(deck.byHandGMSession, "gm1")
    }

    func testNothingIsRestoredWithByHandOffAtLaunch() throws {
        let first = quietDeck()
        report(first, "gm1")
        try asGM(first, names: "gm1")
        Config.shared.gmByHand = false
        let deck = quietDeck()
        report(deck, "gm1")
        XCTAssertNil(deck.roleID(of: "gm1"))
        XCTAssertFalse(deck.access.gm.startedByHand)
        deck.access.gm.applySetting()
        XCTAssertFalse(FileManager.default.fileExists(atPath: sessionFile.path), "and the launch takes the file away")
        Config.shared.gmByHand = true
        XCTAssertNil(restarted().roleID(of: "gm1"))
    }

    func testRotateByHandOffAndAFireDeleteTheFile() throws {
        func marked() throws -> Deck {
            let deck = quietDeck()
            report(deck, "gm1")
            try asGM(deck, names: "gm1")
            XCTAssertTrue(FileManager.default.fileExists(atPath: sessionFile.path))
            return deck
        }
        func afterRestart() -> Deck {
            let deck = restarted()
            report(deck, "gm1")
            return deck
        }
        var deck = try marked()
        XCTAssertNotNil(deck.access.gm.rotate())
        XCTAssertFalse(FileManager.default.fileExists(atPath: sessionFile.path), "rotate")
        XCTAssertNil(afterRestart().roleID(of: "gm1"))

        deck = try marked()
        Config.shared.gmByHand = false
        deck.access.gm.applySetting()
        Config.shared.gmByHand = true
        XCTAssertFalse(FileManager.default.fileExists(atPath: sessionFile.path), "by-hand off")
        XCTAssertNil(afterRestart().roleID(of: "gm1"))

        deck = try marked()
        deck.access.gm.forgetStartedByHand()
        XCTAssertFalse(FileManager.default.fileExists(atPath: sessionFile.path), "a GM fire")
        XCTAssertNil(afterRestart().roleID(of: "gm1"))
    }

    func testAClearBeforeTheRestartRestoresTheNewId() throws {
        let first = quietDeck()
        report(first, "gm1")
        try asGM(first, names: "gm1")
        first.access.gm.moveSession(from: "gm1", to: "gm2")

        let deck = restarted()
        report(deck, "gm1"); report(deck, "gm2")
        XCTAssertNil(deck.roleID(of: "gm1"))
        XCTAssertEqual(deck.roleID(of: "gm2"), "gm")
    }

    func testAChangedTokenRestoresNothing() throws {
        let first = quietDeck()
        report(first, "gm1")
        try asGM(first, names: "gm1")
        // The Keychain item is gone, so the launch makes a new token: the old GM can't be this one.
        try FileManager.default.removeItem(at: Paths.testSupport.appendingPathComponent(TestCopySecrets.gmTokenName))
        let deck = restarted()
        report(deck, "gm1")
        XCTAssertNil(deck.roleID(of: "gm1"))
        XCTAssertFalse(deck.access.gm.startedByHand)
        XCTAssertFalse(FileManager.default.fileExists(atPath: sessionFile.path))
    }

    func testAHiredGMWinsOverARestoredMark() throws {
        let first = quietDeck()
        report(first, "gm1")
        try asGM(first, names: "gm1")
        let deck = restarted()
        report(deck, "gm1")
        let remembered = try XCTUnwrap(deck.access.gm.session)
        let hired = FrontDesk.Hire(id: "h1", name: "GM", role: "gm", folder: "/work/gm", started: 1000, state: .working)
        XCTAssertNil(Deck.byHandGM(remembered: remembered, hires: [hired], live: ["gm1"]))
    }

    func testTheFileHoldsTheIdOnlyAndOnlyYouCanRead() throws {
        let deck = quietDeck()
        report(deck, "gm1")
        try asGM(deck, names: "gm1")
        let text = try String(contentsOf: sessionFile, encoding: .utf8)
        XCTAssertEqual(text.trimmingCharacters(in: .whitespacesAndNewlines), "gm1")
        let token = try XCTUnwrap(deck.access.gm.token())
        XCTAssertFalse(text.contains(token))
        XCTAssertFalse(text.contains("/work"), "no folder")
        let mode = try FileManager.default.attributesOfItem(atPath: sessionFile.path)[.posixPermissions] as? Int
        XCTAssertEqual(mode, 0o600)
    }
}
