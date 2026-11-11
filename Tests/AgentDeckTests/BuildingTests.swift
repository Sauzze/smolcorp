import XCTest
@testable import AgentDeck

/// Who sits on which floor: those put on one, by the office or by their
/// folder's rule, and everyone else where there's room, each keeping their desk.
final class BuildingTests: XCTestCase {
    private func up(_ id: String, floor: Int? = nil, had: Seat? = nil) -> Upstairs<String> {
        Upstairs(person: id, id: id, floor: floor, had: had)
    }

    private func names(_ floor: [(person: String, desk: Int)]) -> [String] { floor.map(\.person) }

    func testThosePutOnAFloorSitThereBeforeAnyoneElse() {
        let seated = Deck.seat(placed: [up("nova", floor: 3)], rest: [up("bot"), up("run")])
        XCTAssertEqual(names(seated.floors[3]), ["nova"])
        XCTAssertEqual(names(seated.floors[1]), ["bot", "run"])
        XCTAssertEqual(seated.seats["nova"], Seat(floor: 3, desk: 0))
        XCTAssertEqual(seated.overflow, 0)
    }

    func testEveryoneKeepsTheDeskTheyHadWhenSomeoneLeaves() {
        // b leaves the second floor's middle desk: a and c stay where they were, with a gap between.
        let seated = Deck.seat(placed: [], rest: [up("a", had: Seat(floor: 1, desk: 0)), up("c", had: Seat(floor: 1, desk: 2))])
        XCTAssertEqual(seated.floors[1].map(\.desk), [0, 2])
        // A newcomer takes the gap, not someone's desk.
        let next = Deck.seat(placed: [], rest: [up("a", had: Seat(floor: 1, desk: 0)), up("c", had: Seat(floor: 1, desk: 2)), up("d")])
        XCTAssertEqual(next.seats["d"], Seat(floor: 1, desk: 1))
        XCTAssertEqual(next.seats["c"], Seat(floor: 1, desk: 2))
    }

    func testSomeonePutOnAFloorTakesTheirOwnDeskBackFromAnyoneSeatedThereSince() {
        let seated = Deck.seat(placed: [up("nova", floor: 1, had: Seat(floor: 1, desk: 4))],
                               rest: [up("run", had: Seat(floor: 1, desk: 4))])
        XCTAssertEqual(seated.seats["nova"], Seat(floor: 1, desk: 4))
        XCTAssertEqual(seated.seats["run"], Seat(floor: 1, desk: 0))
    }

    func testSomeoneMovedOntoAFloorTakesAFreeDeskAndBumpsNobody() {
        // A run sits at 3F's first desk; nova is moved there: the run keeps its desk.
        let seated = Deck.seat(placed: [up("nova", floor: 2)], rest: [up("run", had: Seat(floor: 2, desk: 0))])
        XCTAssertEqual(seated.seats["run"], Seat(floor: 2, desk: 0))
        XCTAssertEqual(seated.seats["nova"], Seat(floor: 2, desk: 1))
    }

    func testOnAFullFloorSomeoneThereForWantOfRoomMakesWay() {
        let runs = (0..<Floor.desks).map { up("r\($0)", had: Seat(floor: 2, desk: $0)) }
        let seated = Deck.seat(placed: [up("nova", floor: 2)], rest: runs)
        XCTAssertEqual(seated.seats["nova"], Seat(floor: 2, desk: 0))
        XCTAssertEqual(seated.seats["r0"], Seat(floor: 1, desk: 0), "the run made way, to where there's room")
        XCTAssertEqual(seated.seats["r1"], Seat(floor: 2, desk: 1), "the others stay")
    }

    func testAPutFloorThatIsFullSendsTheRestWhereThereIsRoom() {
        let crowd = (0...Floor.desks).map { up("p\($0)", floor: 2) }
        let seated = Deck.seat(placed: crowd, rest: [])
        XCTAssertEqual(seated.floors[2].count, 9)
        XCTAssertEqual(names(seated.floors[1]), ["p9"])
    }

    func testADeskThatWasOnAFloorThatIsGoneIsForgotten() {
        let seated = Deck.seat(placed: [], rest: [up("a", had: Seat(floor: 9, desk: 40))])
        XCTAssertEqual(seated.seats["a"], Seat(floor: 1, desk: 0))
    }

    func testPastTheTopFloorThereIsNoDesk() {
        let seated = Deck.seat(placed: [], rest: (0..<46).map { up("h\($0)") })
        XCTAssertEqual(seated.overflow, 1, "five floors of nine")
        XCTAssertNil(seated.seats["h45"])
    }

    // MARK: Rules

    // MARK: Manager hires sit by whole rows

    func testAManagersHiresFillARowLeftToRightThenStartTheNext() {
        XCTAssertEqual(Deck.rowDesk(taken: []), 0)
        XCTAssertEqual(Deck.rowDesk(taken: [0]), 1)
        XCTAssertEqual(Deck.rowDesk(taken: [0, 1, 2]), 3)
        XCTAssertEqual(Deck.rowDesk(taken: [0, 1, 2, 3]), 4)
        XCTAssertEqual(Deck.rowDesk(taken: [0, 1, 2, 3, 4, 5]), 6)
    }

    func testARowIsNotRefilledBehindTheTeamAndTheLastRowIsFull() {
        // Desk 1 was left empty by someone who went; the team is in the second row now, so the hire joins it.
        XCTAssertEqual(Deck.rowDesk(taken: [0, 2, 3, 4]), 5)
        // With the top floor's last row full, the first free desk anywhere.
        XCTAssertEqual(Deck.rowDesk(taken: Set(4..<9)), 0)
        XCTAssertNil(Deck.rowDesk(taken: Set(0..<9)))
    }

    func testManagerHiresSeatByRowWhileOthersTakeTheFirstFreeDesk() {
        var a = up("a", floor: 2, had: Seat(floor: 2, desk: 0))
        a.byRow = true
        var hire = up("hire", floor: 2)
        hire.byRow = true
        let seated = Deck.seat(placed: [a, hire, up("old", floor: 2, had: Seat(floor: 2, desk: 2))], rest: [])
        // The 1.23 session keeps desk 2; the hire joins the last row anyone sits in, which is the first.
        XCTAssertEqual(seated.seats["hire"], Seat(floor: 2, desk: 1))
        // With the team already in the second row, a gap in the first stays empty for a manager's hire, not for anyone else.
        let team = up("x", floor: 2, had: Seat(floor: 2, desk: 4))
        let gap = Deck.seat(placed: [team, hire], rest: [])
        XCTAssertEqual(gap.seats["hire"], Seat(floor: 2, desk: 3))
        var plain = hire
        plain.byRow = false
        let first = Deck.seat(placed: [team, plain], rest: [])
        XCTAssertEqual(first.seats["hire"], Seat(floor: 2, desk: 0))
    }

    func testNobodyMovesWhenAHireLeavesOrAnotherComes() {
        var team = (0..<5).map { i -> Upstairs<String> in
            var one = up("h\(i)", floor: 1, had: i == 2 ? nil : Seat(floor: 1, desk: i))
            one.byRow = true
            return one
        }
        team.remove(at: 3)  // h3 left
        let seated = Deck.seat(placed: team, rest: [])
        XCTAssertEqual(seated.seats["h0"], Seat(floor: 1, desk: 0))
        XCTAssertEqual(seated.seats["h1"], Seat(floor: 1, desk: 1))
        XCTAssertEqual(seated.seats["h4"], Seat(floor: 1, desk: 4))
        // h2 is new: the last row anyone sits in is the second (desks 3 to 5), so it takes its first free desk, 3.
        XCTAssertEqual(seated.seats["h2"], Seat(floor: 1, desk: 3))
    }

    func testAManagerHireWithNoFloorSetFindsARowOnTheFirstFloorWithRoom() {
        var hire = up("hire")
        hire.byRow = true
        let full = (0..<Floor.desks).map { up("f\($0)", had: Seat(floor: 1, desk: $0)) }
        let seated = Deck.seat(placed: [], rest: full + [hire])
        XCTAssertEqual(seated.seats["hire"], Seat(floor: 2, desk: 0))
    }

    func testANicknameGoesOnTheHiresDeskAndKeepsHowItWasDressed() {
        var dressed = TeamMember()
        dressed.outfit = "hoodie"
        var team: [String: TeamMember] = ["wt-nova": dressed]
        team = Deck.nicknamed(team, folder: "/tmp/work/wt-nova", nickname: "  Nova \n")
        XCTAssertEqual(team["wt-nova"]?.name, "Nova")
        XCTAssertEqual(team["wt-nova"]?.outfit, "hoodie")
        XCTAssertEqual(Deck.nicknamed([:], folder: "/tmp/x/run", nickname: "")["run"]?.name, nil)
        XCTAssertEqual(Deck.nicknamed([:], folder: "/tmp/x/run", nickname: String(repeating: "n", count: 40))["run"]?.name.count, TeamMember.nicknameLimit)
    }

    func testTheGMMayPinAHiresDeskAndTheLayoutIsKept() {
        XCTAssertTrue(SetupRoutes.allows(.gm, "POST", "/setup/pin"))
        XCTAssertTrue(SetupRoutes.allows(.gm, "POST", "/setup/unpin"))
        XCTAssertFalse(SetupRoutes.allows(.tool, "POST", "/setup/pin"))
    }

    func testARuleSendsAFolderAndWhatIsInItToItsFloor() {
        let rules = [FloorRule(folder: "/work/atlas", floor: 2), FloorRule(folder: "/work", floor: 4)]
        XCTAssertEqual(FloorRule.floor(of: "/work/atlas", in: rules), 2)
        XCTAssertEqual(FloorRule.floor(of: "/work/atlas/app", in: rules), 2, "the closest rule above it")
        XCTAssertEqual(FloorRule.floor(of: "/work/other", in: rules), 4)
        XCTAssertNil(FloorRule.floor(of: "/work/atlasx", in: [rules[0]]), "a folder named alike isn't in it")
        XCTAssertNil(FloorRule.floor(of: "", in: rules))
    }

    func testARememberedRuleIsItsFolderAloneWhileSetupsHoldsWhatsInIt() throws {
        let rules = [FloorRule(folder: "/Users/me/Projects", floor: 3, exact: true), FloorRule(folder: "/work", floor: 2)]
        XCTAssertEqual(FloorRule.floor(of: "/Users/me/Projects", in: rules), 3)
        XCTAssertNil(FloorRule.floor(of: "/Users/me/Projects/app", in: rules), "the GM's folder takes none of the projects in it")
        XCTAssertEqual(FloorRule.floor(of: "/work/app", in: rules), 2)
        // A rule from before `exact` holds what's in it; kept as given through a save.
        let old = try JSONDecoder().decode(FloorRule.self, from: Data(#"{"folder": "/a", "floor": 1}"#.utf8))
        XCTAssertFalse(old.exact)
        XCTAssertEqual(FloorRule.checked([FloorRule(folder: "/a", floor: 1, exact: true)]).first?.exact, true)
    }

    func testARememberedRuleBesideSetupsOnTheSameFolderLeavesItsFoldersWhereTheyWere() {
        let rules = FloorRule.checked([FloorRule(folder: "/work", floor: 2), FloorRule(folder: "/work", floor: 4, exact: true)])
        XCTAssertEqual(rules.count, 2, "one of each kind")
        XCTAssertEqual(FloorRule.floor(of: "/work", in: rules), 4)
        XCTAssertEqual(FloorRule.floor(of: "/work/app", in: rules), 2)
        XCTAssertEqual(FloorRule.folder(of: 2, in: rules), "/work")
    }

    func testWhereASessionWasPutGoesBeforeItsRule() {
        let rules = [FloorRule(folder: "/work/atlas", floor: 2)]
        XCTAssertEqual(Deck.floor(of: "/work/atlas", put: nil, rules: rules), 2)
        XCTAssertEqual(Deck.floor(of: "/work/atlas", put: 0, rules: rules), 0, "brought down to the ground floor")
        XCTAssertEqual(Deck.floor(of: "/work/atlas", put: 5, rules: rules), 5)
        XCTAssertNil(Deck.floor(of: "/elsewhere", put: nil, rules: rules))
    }

    func testRulesAreCheckedAsTheyAreKept() {
        let rules = FloorRule.checked([
            FloorRule(folder: " /work/atlas/ ", floor: 2), FloorRule(folder: "relative", floor: 1),
            FloorRule(folder: "/work/a", floor: 9), FloorRule(folder: "/work/atlas", floor: 3),
        ])
        XCTAssertEqual(rules, [FloorRule(folder: "/work/atlas", floor: 3)])
    }

    func testAWindowsComputersRulesAreKeptForItButNeverMatchHere() {
        let rules = FloorRule.checked([FloorRule(folder: #"C:\Users\me\proj\"#, floor: 2), FloorRule(folder: #"\\server\share"#, floor: 3)])
        XCTAssertEqual(rules.map(\.folder), [#"C:\Users\me\proj"#, #"\\server\share"#])
        XCTAssertNil(FloorRule.floor(of: "/Users/me/proj", in: rules))
    }

    func testSetupSendsRulesBesideTheFloorsAndAnOlderPageSendsNone() {
        let sent = Floor.rules(Data(#"{"rules": [{"folder": "/work/atlas", "floor": 2}, {"folder": 3}]}"#.utf8))
        XCTAssertEqual(sent, [FloorRule(folder: "/work/atlas", floor: 2)])
        XCTAssertNil(Floor.rules(Data(#"{"floors": []}"#.utf8)))
    }

    func testAHireUpstairsStartsInItsFloorsOneFolder() {
        let rules = [FloorRule(folder: "/a", floor: 2), FloorRule(folder: "/b", floor: 2), FloorRule(folder: "/c", floor: 3)]
        XCTAssertNil(FloorRule.folder(of: 2, in: rules), "two rules: the folder for new hires")
        XCTAssertEqual(FloorRule.folder(of: 3, in: rules), "/c")
        XCTAssertNil(FloorRule.folder(of: 4, in: rules))
    }

    // MARK: Cloud bots

    func testACloudBotCanBePutOnAnyFloor() {
        var bot = CloudBot(id: "mini", name: "smolcorp mini", floor: "4")
        XCTAssertEqual(bot.checked?.floor, "4")
        XCTAssertEqual(bot.floorIndex, 3)
        bot.floor = "ground"
        XCTAssertEqual(bot.floorIndex, 0)
        bot.floor = ""
        XCTAssertNil(bot.floorIndex, "where new bots go")
        bot.floor = "7"
        XCTAssertEqual(bot.checked?.floor, "")
        XCTAssertEqual(CloudBot.floor(at: 3), "4")
        XCTAssertEqual(CloudBot.floor(at: 0), "ground")
    }

    /// A floor with a folder of its own can be hired onto with no folder for
    /// new hires (SIERRA 1.23 m-6); the ground floor still needs that one.
    func testAFloorWithItsOwnFolderOffersHiringWithNoFolderForNewHires() throws {
        let atlas = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: atlas, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: atlas) }
        let config = Config()
        config.hiringEnabled = true
        config.hireFolder = ""
        XCTAssertFalse(Deck.canHire(config), "no folder anywhere")
        config.floorRules = [FloorRule(folder: atlas.path + "-gone", floor: 2)]
        XCTAssertFalse(Deck.canHire(config), "a floor folder that's gone")
        config.floorRules = [FloorRule(folder: atlas.path, floor: 0)]
        XCTAssertFalse(Deck.canHire(config), "a rule for the ground floor isn't a floor upstairs")
        config.floorRules = [FloorRule(folder: atlas.path, floor: 2)]
        XCTAssertTrue(Deck.canHire(config), "a floor with a folder of its own")
        config.hiringEnabled = false
        XCTAssertFalse(Deck.canHire(config), "hiring off")
        config.hiringEnabled = true
        config.floorRules = []
        config.hireFolder = atlas.path
        XCTAssertTrue(Deck.canHire(config), "the folder for new hires")
    }

    /// A hire whose agent's hooks don't report here is refused, as it would
    /// never reach a desk (SIERRA 1.23 M-C); another agent's hooks don't matter.
    func testAHireWithoutItsAgentsHooksIsRefused() {
        let connected: Set<Agent> = [.claude]
        let refusal = { (agent: Agent?) in Deck.hooksRefusal(for: agent, found: [.claude, .codex], connected: connected.contains) }
        XCTAssertNil(refusal(.claude), "Claude hooked, Codex not")
        let codex = refusal(.codex)
        XCTAssertTrue(codex?.hasPrefix("Connect your agents first: Codex") == true, codex ?? "nil")
        XCTAssertTrue(codex?.contains("Press Connect in Smolcorp's Settings") == true)
        XCTAssertNil(refusal(.gemini), "not on this Mac: its terminal says so")
        XCTAssertNil(refusal(nil))
    }

    /// A role whose agent isn't known launches as Claude Code, so its hooks are checked as Claude Code's (T107).
    func testARoleWithAnUnknownAgentIsCheckedAsClaudeCode() {
        func role(_ agent: String) -> Role {
            Role(id: "x", name: "X", agent: agent, model: "", effort: "", instructions: "Do it.", badge: "X")
        }
        XCTAssertEqual(Deck.hiredAgent(role: role("aider"), hireAgent: "codex"), .claude)
        XCTAssertEqual(Deck.hiredAgent(role: role("codex"), hireAgent: "claude"), .codex)
        XCTAssertEqual(Deck.hiredAgent(role: nil, hireAgent: "codex"), .codex)
        XCTAssertNil(Deck.hiredAgent(role: nil, hireAgent: "aider"), "a command of your own has no hooks to check")
        let refusal = Deck.hooksRefusal(for: Deck.hiredAgent(role: role("aider"), hireAgent: "codex"), found: [.claude], connected: { _ in false })
        XCTAssertTrue(refusal?.hasPrefix("Connect your agents first: Claude Code") == true, refusal ?? "nil")
    }

    /// The office says which hires won't get the fence: every agent but Claude Code, and a Claude Code hire
    /// with the manager off, unless it's the General Manager (D1, as `open` writes the fence for `managed && claudeHire`).
    func testTheOfficeSaysWhichHiresGetNoFence() {
        func role(_ id: String, _ agent: String) -> Role {
            Role(id: id, name: id, agent: agent, model: "", effort: "", instructions: "Do it.", badge: "X")
        }
        XCTAssertEqual(Deck.unfencedNote(role: role("rev", "codex"), hireAgent: "claude", managerOn: true),
                       "Codex hires don't get Smolcorp's fence: they run with their own settings.")
        XCTAssertEqual(Deck.unfencedNote(role: nil, hireAgent: "opencode", managerOn: true),
                       "OpenCode hires don't get Smolcorp's fence: they run with their own settings.")
        XCTAssertNotNil(Deck.unfencedNote(role: nil, hireAgent: "aider", managerOn: true), "a command of your own isn't fenced")
        XCTAssertNil(Deck.unfencedNote(role: role("dev", "claude"), hireAgent: "codex", managerOn: true))
        XCTAssertNil(Deck.unfencedNote(role: nil, hireAgent: "claude", managerOn: true))
        XCTAssertNil(Deck.unfencedNote(role: role(Deck.gmRole, "claude"), hireAgent: "claude", managerOn: false), "the GM is always managed")
        XCTAssertNil(Deck.unfencedNote(role: role("dev", "aider"), hireAgent: "codex", managerOn: true), "an unknown agent launches as Claude Code")
        XCTAssertEqual(Deck.unfencedNote(role: role("dev", "claude"), hireAgent: "claude", managerOn: false),
                       "With the manager off, this Claude Code hire doesn't get Smolcorp's fence: it runs with its own settings.")
        XCTAssertNotNil(Deck.unfencedNote(role: nil, hireAgent: "claude", managerOn: false))
    }

    /// The office's ⚙︎ says so when the browser won't open, with the way in that works (SIERRA 1.23 gear).
    func testTheCogSaysWhenTheBrowserWouldNotOpen() {
        XCTAssertEqual(Deck.settingsAnswer(opened: true).status, 204)
        let failed = Deck.settingsAnswer(opened: false)
        XCTAssertEqual(failed.status, 502)
        XCTAssertTrue(String(decoding: failed.body, as: UTF8.self).contains("Settings… from Smolcorp's menu bar icon"))
    }

    /// Going home, a bot that yields never takes a desk kept for someone put on a floor and sent on from it, though
    /// the first one placed is a bot that yields for the floor it sits on; seated again as they now sit, nobody moves.
    /// The same case as the Go app's TestGoingHomeABotTakesNoDeskKeptForSomeoneSentOn: both apps seat alike.
    func testGoingHomeABotTakesNoDeskKeptForSomeoneSentOn() {
        var placed: [Upstairs<String>] = [
            Upstairs(person: "A", id: "A", floor: 2, had: Seat(floor: 2, desk: 0), yields: true),  // a bot for 3F, at 3F's first desk
            Upstairs(person: "P", id: "P", floor: 1, had: Seat(floor: 2, desk: 1), yields: true),  // a bot for 2F, at 3F's second
            Upstairs(person: "H", id: "H", floor: 4, had: Seat(floor: 1, desk: 8)),               // put on 5F, sent on to 2F's last desk
        ]
        placed += (0..<Floor.desks).map { Upstairs(person: "f\($0)", id: "f\($0)", floor: 4, had: Seat(floor: 4, desk: $0)) }  // 5F full
        var rest = (0..<8).map { Upstairs(person: "r\($0)", id: "r\($0)", had: Seat(floor: 1, desk: $0)) }
        let seats = Deck.seat(placed: placed, rest: rest).seats
        XCTAssertEqual(seats["H"], Seat(floor: 1, desk: 8), "H at 2F's last desk")
        XCTAssertEqual(seats["P"], Seat(floor: 2, desk: 1), "P at 3F's second")
        for i in placed.indices { placed[i].had = seats[placed[i].id] }
        for i in rest.indices { rest[i].had = seats[rest[i].id] }
        XCTAssertEqual(Deck.seat(placed: placed, rest: rest).seats, seats, "seated again, nobody moves")
    }
}
