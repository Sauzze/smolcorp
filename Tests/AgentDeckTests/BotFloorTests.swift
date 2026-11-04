import XCTest
@testable import AgentDeck

/// Bots on any floor (T762): each where you put it, or where a floor rule
/// for its folder says, or where new bots go; a full floor sends it where
/// there's room; and an old config's bots stay where they sat. The Go app
/// loads the same configs the same way (Tests/fixtures/bots/configs.json).
final class BotFloorTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    // MARK: Old configs

    private struct Case: Decodable {
        struct After: Decodable {
            let botFloor: Int
            let bots: [[String]]
            let pinned: [String]?
            let floorNames: [String]?
            let hireFolder: String?
        }
        let about: String
        /// The desks saved across a restart (sessions.json's `seats`), by desk id: where each bot sat.
        let seats: [String: Seat]?
        let after: After
    }

    /// Each config loads, and as the app starts, its bots still "upstairs" go on the floors they sat on, by the desks
    /// saved for them, with the floor new bots go to; saved and started again, nothing changes.
    func testOldConfigsLoadWithTheirBotsWhereTheySitAndLoadAgainUnchanged() throws {
        let data = try Data(contentsOf: Self.repo.appendingPathComponent("Tests/fixtures/bots/configs.json"))
        let cases = try JSONDecoder().decode([Case].self, from: data)
        let befores = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [[String: Any]]).map { $0["before"] ?? [:] }
        XCTAssertEqual(cases.count, 6)
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        for (n, (one, before)) in zip(cases, befores).enumerated() {
            let file = folder.appendingPathComponent("config-\(n).json"), again = folder.appendingPathComponent("again-\(n).json")
            try JSONSerialization.data(withJSONObject: before).write(to: file)
            let config = Config.load(from: file)
            _ = config.settleBots(seats: one.seats ?? [:])
            config.save(to: again)
            let started = Config.load(from: again)
            XCTAssertFalse(started.settleBots(seats: one.seats ?? [:]), "\(one.about): settled once")
            for (when, loaded) in [("as the app starts", config), ("saved and started again", started)] {
                XCTAssertEqual(loaded.cloudBots.map { [$0.id, $0.floor] }, one.after.bots, "\(one.about) (\(when))")
                XCTAssertEqual(loaded.botFloor, one.after.botFloor, "\(one.about) (\(when))")
                if let pinned = one.after.pinned { XCTAssertEqual(loaded.cloudBots.filter(\.pinned).map(\.id), pinned, one.about) }
                if let names = one.after.floorNames { XCTAssertEqual(loaded.floors.map(\.name), names, one.about) }
                if let hireFolder = one.after.hireFolder { XCTAssertEqual(loaded.hireFolder, hireFolder, one.about) }
            }
        }
    }

    // MARK: In the office

    /// A deck that restores, saves and tells nothing, laid out by a fresh config: no rules, new bots on 2F.
    private func quietDeck() -> Deck {
        let deck = Deck(quiet: true)
        deck.layoutConfig = Config()
        return deck
    }

    /// As many desks as a floor above the ground floor has, as the building seats them.
    private let room = Deck.seat(placed: (0..<99).map { Upstairs(person: $0, id: "p\($0)", floor: 1) }, rest: []).floors[1].count

    private func bots(_ count: Int, from first: Int = 0) -> [CloudBot] {
        (first..<first + count).map { CloudBot(id: "b\($0)", name: "B\($0)") }
    }

    /// Who sits on each floor above, by their desks' ids; the ground floor's are at keys.
    private func floors(_ deck: Deck) -> [[String]] {
        deck.building(overflow: deck.layout().overflow).floors.map { $0.map(\.id) }
    }

    private func keys(_ deck: Deck) -> [String?] { deck.layout().keys.map { $0.session?.id } }

    /// Where each sits above the ground floor, by their desks' ids, the building seated once: the floor, 1 for 2F up,
    /// and the desk there.
    private func seats(_ deck: Deck) -> [String: Seat] {
        let building = deck.building(overflow: deck.layout().overflow)
        var seats: [String: Seat] = [:]
        for (floor, people) in building.floors.enumerated() {
            for one in people { seats[one.id] = Seat(floor: floor, desk: building.desks[one.id] ?? -1) }
        }
        return seats
    }

    /// Headless runs in `folder`, one for each of `ids`, started one after another from `first`.
    private func runs(_ deck: Deck, _ ids: [String], in folder: String, from first: Double) {
        for (n, id) in ids.enumerated() {
            deck.sessions.apply(["hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": folder],
                                agent: .claude, tab: "headless:\(id)", startedIn: "", at: first + Double(n))
        }
    }

    /// An agent at work in a window, at a key on the ground floor.
    private func agent(_ deck: Deck, _ id: String) {
        deck.sessions.apply(["hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": "/work/\(id)"],
                            agent: .claude, tab: "tty:/dev/\(id)", startedIn: "", at: 50)
    }

    /// 2F, where new bots go, full of runs its folder's rule puts there.
    private func fullSecondFloor(_ deck: Deck) {
        deck.layoutConfig.floorRules = [FloorRule(folder: "/work/api", floor: 1)]
        runs(deck, (0..<room).map { "h\($0)" }, in: "/work/api", from: 100)
        XCTAssertEqual(floors(deck)[1].count, room, "2F full of runs")
    }

    func testABotWithNoFloorSitsWhereNewBotsGoAndOnAFullFloorWhereThereIsRoom() {
        let deck = quietDeck()
        deck.layoutConfig.botFloor = 3
        deck.cloud.configure(bots(room + 1))
        var seated = floors(deck)
        XCTAssertEqual(seated[3], bots(room).map(\.sessionID), "4F, where new bots go, every desk")
        XCTAssertEqual(seated[1], ["cloud-b\(room)"], "the one more where there's room")
        // Where new bots go chosen again: none was put anywhere, so they all go there, the one already there at its desk.
        // The last, with 2F full, is on a floor not its own wherever it sits, so it keeps the desk it had, on 4F.
        deck.layoutConfig.botFloor = 1
        seated = floors(deck)
        XCTAssertEqual(seated[1].count, room)
        XCTAssertEqual(seated[1].first, "cloud-b\(room)", "the one there already keeps its desk")
        XCTAssertEqual(seated[2], [])
        XCTAssertEqual(seated[3], ["cloud-b\(room - 1)"], "the last of the others at the desk it had")
        XCTAssertEqual(seats(deck)["cloud-b\(room - 1)"], Seat(floor: 3, desk: room - 1))
    }

    /// A bot nobody put on a floor takes nobody's desk: with 2F, where new bots go, full of runs, one more bot sits
    /// where there's room and nobody moves. Put on 2F, it's seated as anyone put there: a run there only for want of
    /// room makes way.
    func testABotNobodyPutOnAFullFloorTakesNobodysDesk() {
        let deck = quietDeck()
        for i in 0..<room {
            deck.sessions.apply(["hook_event_name": "UserPromptSubmit", "session_id": "h\(i)", "cwd": "/work/api"],
                                agent: .claude, tab: "headless:h\(i)", startedIn: "", at: Double(100 + i))
        }
        let runs = floors(deck)[1]
        XCTAssertEqual(runs.count, room, "2F full of runs")
        deck.cloud.configure([CloudBot(id: "new", name: "New")])
        var seated = floors(deck)
        XCTAssertEqual(seated[1], runs, "nobody on 2F moves for it")
        XCTAssertEqual(seated[2], ["cloud-new"], "it sits where there's room")
        deck.cloud.configure([CloudBot(id: "new", name: "New", floor: "2")])
        seated = floors(deck)
        XCTAssertEqual(seated[1].first, "cloud-new", "put on 2F, at a desk there")
        XCTAssertEqual(seated[2], ["h0"], "the run that sat there, where there's room")
    }

    /// A bot nobody put on a floor, sent on from its full floor, keeps the desk it has as others come and go, as anyone
    /// not put on a floor does: one back at a desk it had takes it only if nobody's there.
    func testABotSentOnFromItsFullFloorKeepsItsDeskAsOthersComeAndGo() {
        let deck = quietDeck()
        fullSecondFloor(deck)
        // A part-time one at work first, then another.
        let part = CloudBot(id: "part", name: "Part", permanent: false)
        deck.cloud.configure([part])
        deck.cloud.apply(.start, to: "part")
        _ = floors(deck)
        deck.cloud.configure([part, CloudBot(id: "kept", name: "Kept")])
        XCTAssertEqual(floors(deck)[2], ["cloud-part", "cloud-kept"], "both on 3F, 2F being full")
        deck.cloud.apply(.stop, to: "part")
        XCTAssertEqual(seats(deck)["cloud-kept"], Seat(floor: 2, desk: 1), "the part-time one gone, the other at its desk")
        // A run with no floor of its own sits at the desk the part-time one had, which comes back to another.
        runs(deck, ["x0"], in: "/work/other", from: 200)
        XCTAssertEqual(seats(deck)["x0"], Seat(floor: 2, desk: 0))
        deck.cloud.apply(.start, to: "part")
        let back = seats(deck)
        XCTAssertEqual(back["cloud-part"], Seat(floor: 2, desk: 2), "back, at a free desk")
        XCTAssertEqual(back["x0"], Seat(floor: 2, desk: 0), "the run at its own")
        // An agent put on 3F: at a free desk there, every bot at its own.
        agent(deck, "s0")
        XCTAssertEqual(deck.move("s0", to: "2", remember: false).status, 204)
        XCTAssertEqual(seats(deck)["s0"], Seat(floor: 2, desk: 3))
        XCTAssertEqual(floors(deck)[2], ["x0", "cloud-kept", "cloud-part", "s0"])
    }

    /// A person sent on from a full floor keeps the desk they were given when a bot with no floor of its own
    /// arrives, or a part-time bot before them leaves.
    func testAPersonSentOnFromAFullFloorKeepsTheirDeskAsBotsComeAndGo() {
        var deck = quietDeck()
        fullSecondFloor(deck)
        runs(deck, ["h9"], in: "/work/api", from: 200) // a tenth run for 2F: sent on to 3F
        XCTAssertEqual(seats(deck)["h9"], Seat(floor: 2, desk: 0), "the tenth run on 3F's first desk")
        deck.cloud.configure([CloudBot(id: "new", name: "New")])
        let after = seats(deck)
        XCTAssertEqual(after["h9"], Seat(floor: 2, desk: 0), "a new bot came: the run at its desk")
        XCTAssertEqual(after["cloud-new"], Seat(floor: 2, desk: 1), "and the bot at the next")

        deck = quietDeck()
        fullSecondFloor(deck)
        let part = CloudBot(id: "part", name: "Part", permanent: false)
        deck.cloud.configure([part])
        deck.cloud.apply(.start, to: "part")
        runs(deck, ["h9"], in: "/work/api", from: 200)
        XCTAssertEqual(seats(deck)["cloud-part"], Seat(floor: 2, desk: 0))
        XCTAssertEqual(seats(deck)["h9"], Seat(floor: 2, desk: 1))
        deck.cloud.apply(.stop, to: "part")
        XCTAssertEqual(seats(deck)["h9"], Seat(floor: 2, desk: 1), "the part-time bot gone, the run at its own desk")
    }

    /// A person sent on from a full floor keeps their desk when a bot nobody put anywhere arrives whose own floor is
    /// the floor they were sent to, and when it stops and starts again.
    func testABotWhoseFloorIsWhereAPersonWasSentOnTakesAnotherDesk() {
        let deck = quietDeck()
        fullSecondFloor(deck)
        runs(deck, ["h9"], in: "/work/api", from: 200) // a tenth run for 2F: sent on to 3F
        XCTAssertEqual(seats(deck)["h9"], Seat(floor: 2, desk: 0), "the tenth run on 3F's first desk")
        deck.layoutConfig.botFloor = 2 // new bots go to 3F
        let part = CloudBot(id: "part", name: "Part", permanent: false)
        deck.cloud.configure([part])
        deck.cloud.apply(.start, to: "part")
        XCTAssertEqual(seats(deck)["h9"], Seat(floor: 2, desk: 0), "a bot for 3F came: the run at its desk")
        XCTAssertEqual(seats(deck)["cloud-part"], Seat(floor: 2, desk: 1), "and the bot at the next")
        deck.cloud.apply(.stop, to: "part")
        deck.cloud.apply(.start, to: "part")
        XCTAssertEqual(seats(deck)["h9"], Seat(floor: 2, desk: 0), "the bot stopped and started again: the run at its own desk")
    }

    /// A bot sent on from its full floor goes back when a desk is free there; the other stays at its own.
    func testABotSentOnFromItsFullFloorGoesBackWhenADeskIsFreeThere() {
        let deck = quietDeck()
        fullSecondFloor(deck)
        deck.cloud.configure(bots(2))
        XCTAssertEqual(floors(deck)[2], ["cloud-b0", "cloud-b1"], "both on 3F, 2F being full")
        deck.sessions.apply(["hook_event_name": "SessionEnd", "session_id": "h4"], agent: .claude, tab: "headless:h4", startedIn: "", at: 300)
        let seated = seats(deck)
        XCTAssertEqual(seated["cloud-b0"], Seat(floor: 1, desk: 4), "back on 2F, at the desk the run left")
        XCTAssertEqual(seated["cloud-b1"], Seat(floor: 2, desk: 1), "the other at its own")
    }

    /// After a restart, a bot sent on from its full floor is back at the desk it had, as an agent still running is.
    func testABotSentOnFromItsFullFloorGetsItsDeskBackAfterARestart() {
        let deck = quietDeck()
        fullSecondFloor(deck)
        deck.cloud.configure([CloudBot(id: "kept", name: "Kept")])
        // Seated once as the app starts, at the first free desk, as the Go app does before it reads what was saved.
        XCTAssertEqual(seats(deck)["cloud-kept"], Seat(floor: 2, desk: 0))
        deck.putBackBots(Deck.SavedDeck(savedAt: 0, sessions: [], keys: [:], seats: ["cloud-kept": Seat(floor: 2, desk: 5)]))
        XCTAssertEqual(seats(deck)["cloud-kept"], Seat(floor: 2, desk: 5), "on 3F, at the desk it had")
    }

    /// On a full floor, one put there takes the desk of a bot there only for want of room on its own, as of a run there
    /// for want of room, be it a bot put there or a run its folder's rule puts there: the bot goes where there's room.
    func testOnAFullFloorABotSentOnFromItsOwnMakesWayForAnyonePutThere() {
        let deck = quietDeck()
        fullSecondFloor(deck)
        deck.layoutConfig.floorRules.append(FloorRule(folder: "/work/web", floor: 2))
        runs(deck, (0..<room - 2).map { "w\($0)" }, in: "/work/web", from: 200)
        _ = floors(deck)
        deck.cloud.configure(bots(2))
        var seated = seats(deck)
        XCTAssertEqual(seated["cloud-b0"], Seat(floor: 2, desk: room - 2), "3F's last desks, 2F being full")
        XCTAssertEqual(seated["cloud-b1"], Seat(floor: 2, desk: room - 1))
        deck.cloud.configure([CloudBot(id: "put", name: "Put", floor: "3")] + bots(2))
        seated = seats(deck)
        XCTAssertEqual(seated["cloud-put"], Seat(floor: 2, desk: room - 2), "the bot put on 3F at the first one's desk")
        XCTAssertEqual(seated["cloud-b0"], Seat(floor: 3, desk: 0), "that one where there's room")
        runs(deck, ["w\(room - 2)"], in: "/work/web", from: 300)
        seated = seats(deck)
        XCTAssertEqual(seated["w\(room - 2)"], Seat(floor: 2, desk: room - 1), "a run its folder's rule puts on 3F, at the other's desk")
        XCTAssertEqual(seated["cloud-b1"], Seat(floor: 3, desk: 1), "that one where there's room")
        XCTAssertEqual(seats(deck), seated, "seated again, nobody moves")
    }

    /// A bot sent on from its full floor takes no room there: one put on it has a desk past a run there only for want
    /// of room.
    func testABotSentOnFromItsFullFloorLeavesItsRoomToAnyonePutThere() {
        let deck = quietDeck()
        deck.layoutConfig.floorRules = [FloorRule(folder: "/work/api", floor: 1)]
        runs(deck, (0..<room - 1).map { "h\($0)" }, in: "/work/api", from: 100)
        runs(deck, ["x0"], in: "/work/other", from: 200)
        XCTAssertEqual(floors(deck)[1].last, "x0", "a run with no floor of its own, at 2F's last desk")
        deck.cloud.configure([CloudBot(id: "kept", name: "Kept")])
        XCTAssertEqual(floors(deck)[2], ["cloud-kept"], "on 3F, 2F being full")
        agent(deck, "s0")
        XCTAssertEqual(deck.move("s0", to: "1", remember: false).status, 204, "2F has room for one put there")
        let seated = seats(deck)
        XCTAssertEqual(seated["s0"], Seat(floor: 1, desk: room - 1), "at the run's desk")
        XCTAssertEqual(seated["cloud-kept"], Seat(floor: 2, desk: 0), "the bot at its own")
    }

    /// Bots their folders' rules send elsewhere go there as desks come free: two each on the other's full floor change
    /// places; one going home frees a desk for another, and that one for a third, new there, all as it's seated once.
    func testBotsSentElsewhereGoHomeAsDesksComeFree() {
        let deck = quietDeck()
        let rules = { (x: Int, y: Int) in
            [FloorRule(folder: "/work/api", floor: 1), FloorRule(folder: "/work/web", floor: 2), FloorRule(folder: "/work/five", floor: 4),
             FloorRule(folder: CloudBot.folder(named: "X"), floor: x), FloorRule(folder: CloudBot.folder(named: "Y"), floor: y)]
        }
        deck.layoutConfig.floorRules = rules(1, 2)
        runs(deck, (0..<room - 1).map { "h\($0)" }, in: "/work/api", from: 100)
        runs(deck, (0..<room - 1).map { "w\($0)" }, in: "/work/web", from: 200)
        runs(deck, (0..<room).map { "v\($0)" }, in: "/work/five", from: 300)
        _ = floors(deck)
        let (x, y) = (CloudBot(id: "x", name: "X"), CloudBot(id: "y", name: "Y"))
        deck.cloud.configure([y, x])
        var seated = seats(deck)
        XCTAssertEqual(seated["cloud-x"], Seat(floor: 1, desk: room - 1))
        XCTAssertEqual(seated["cloud-y"], Seat(floor: 2, desk: room - 1))
        // Each on the other's floor, both full: they change places.
        deck.layoutConfig.floorRules = rules(2, 1)
        seated = seats(deck)
        XCTAssertEqual(seated["cloud-x"], Seat(floor: 2, desk: room - 1), "on 3F, its own")
        XCTAssertEqual(seated["cloud-y"], Seat(floor: 1, desk: room - 1), "on 2F, its own")
        // X's floor 5F, full, Y's 3F, where X sits: nobody moves till 5F has a desk. Then X goes, Y to its desk, and a
        // new bot to Y's.
        deck.layoutConfig.floorRules = rules(4, 2)
        XCTAssertEqual(seats(deck), seated, "nobody moves")
        deck.sessions.apply(["hook_event_name": "SessionEnd", "session_id": "v4"], agent: .claude, tab: "headless:v4", startedIn: "", at: 400)
        deck.cloud.configure([y, x, CloudBot(id: "new", name: "New")])
        seated = seats(deck)
        XCTAssertEqual(seated["cloud-x"], Seat(floor: 4, desk: 4), "on 5F, at the desk the run left")
        XCTAssertEqual(seated["cloud-y"], Seat(floor: 2, desk: room - 1), "on 3F, at X's")
        XCTAssertEqual(seated["cloud-new"], Seat(floor: 1, desk: room - 1), "on 2F, where new bots go, at Y's")
        XCTAssertEqual(seats(deck), seated, "seated again, nobody moves")
    }

    func testAFoldersRuleGivesABotItsFloorAndOneYouPutThereStays() {
        let deck = quietDeck()
        deck.layoutConfig.floorRules = [FloorRule(folder: "/cloud", floor: 4), FloorRule(folder: "/cloud/Scout", floor: 0, exact: true)]
        deck.cloud.configure([CloudBot(id: "amber", name: "Amber"), CloudBot(id: "scout", name: "Scout"), CloudBot(id: "put", name: "Put", floor: "3")])
        let seated = floors(deck)
        XCTAssertEqual(seated[4], ["cloud-amber"], "the bots' folder's rule, before where new bots go")
        XCTAssertEqual(keys(deck), ["cloud-scout"], "its own folder's rule: the ground floor, at a key")
        XCTAssertEqual(seated[2], ["cloud-put"], "put on 3F, it stays there, whatever the rules say")
        XCTAssertEqual(seated[1], [], "nobody where new bots go")
    }

    /// RUBY, the owner's Mini Manager, is on the ground floor because it was put there, never by its name.
    func testTheMiniManagerOnTheGroundFloorTakesAKeyAndLeavesTheBotsTheirRoom() {
        let deck = quietDeck()
        deck.layoutConfig.botFloor = 2
        let manager = CloudBot(id: "manager", name: "Mini Manager", floor: "ground")
        deck.cloud.configure([manager] + bots(room))
        XCTAssertEqual(keys(deck), ["cloud-manager"], "a desk downstairs of its own")
        XCTAssertEqual(floors(deck)[2], bots(room).map(\.sessionID), "every desk on the bots' floor is theirs")
        // Found in the Grok Bot app: past the 9 bots there could be, the manager among them, each new one gets a desk.
        let kept = [manager] + bots(8)
        let found = CloudBot.discover([.init(id: "aa11", name: "New One"), .init(id: "bb22", name: "New Two")], in: kept, hidden: [])
        XCTAssertEqual(found.noRoom, [])
        XCTAssertEqual(found.bots.count, 11)
        XCTAssertEqual(found.bots.last?.floor, "", "where new bots go")
    }

    /// A bot never ends, so it comes back to the desk it had, as an agent still running after a restart does.
    func testABotGetsTheDeskItHadBackAfterARestart() {
        let deck = quietDeck()
        deck.layoutConfig.layout.keys = .fixed(6)
        deck.cloud.configure([CloudBot(id: "manager", name: "Mini Manager", floor: "ground"), CloudBot(id: "up", name: "Up", floor: "3")])
        // Seated once as the app starts, at the first free desks, as the Go app does before it reads what was saved.
        XCTAssertEqual(floors(deck)[2], ["cloud-up"])
        deck.putBackBots(Deck.SavedDeck(savedAt: 0, sessions: [], keys: ["cloud-manager": 4, "gone": 1],
                                        seats: ["cloud-up": Seat(floor: 2, desk: 5)]))
        XCTAssertEqual(keys(deck), [nil, nil, nil, nil, "cloud-manager", nil])
        XCTAssertEqual(deck.building(overflow: []).desks["cloud-up"], 5)
    }

    /// The office keeps a bot's floor in the settings, as setup's list does: the test copy's, put back after, with
    /// the floor new bots go to.
    private func movingBots(_ body: () throws -> Void) rethrows {
        let (before, botFloor) = (Config.shared.cloudBots, Config.shared.botFloor)
        defer {
            Config.shared.cloudBots = before
            Config.shared.botFloor = botFloor
            Config.shared.save()
        }
        try body()
    }

    /// A bot still "upstairs", from before bots could sit on any floor, stays as the app starts on the floor of the
    /// desk saved for it, at that desk, and the settings are saved so.
    func testABotThatWasUpstairsStaysOnTheFloorItSatOnAsTheAppStarts() {
        movingBots {
            let deck = quietDeck()
            Config.shared.cloudBots = [CloudBot(id: "up", name: "Up", floor: CloudBot.upstairs)]
            deck.cloud.configure(Config.shared.cloudBots)
            deck.putBackBots(Deck.SavedDeck(savedAt: 0, sessions: [], keys: [:], seats: ["cloud-up": Seat(floor: 2, desk: 4)]))
            XCTAssertEqual(Config.shared.cloudBots.map(\.floor), ["3"])
            XCTAssertEqual(Config.load().cloudBots.map(\.floor), ["3"], "saved")
            XCTAssertEqual(floors(deck)[2], ["cloud-up"], "on 3F, not where new bots go")
            XCTAssertEqual(deck.building(overflow: []).desks["cloud-up"], 4, "at the desk it had")
        }
    }

    /// Put where it already sits, where new bots go, a bot stays there from then on, at the desk it has.
    func testABotPutWhereItSitsStaysThereAtItsDesk() {
        movingBots {
            let deck = quietDeck()
            deck.layoutConfig.botFloor = 2
            let three = bots(3)
            Config.shared.cloudBots = three
            deck.cloud.configure(three)
            _ = floors(deck)
            // The one between them gone: the last keeps its desk, the third, with a free one before it.
            Config.shared.cloudBots = [three[0], three[2]]
            deck.cloud.configure(Config.shared.cloudBots)
            _ = floors(deck)
            XCTAssertEqual(deck.move("cloud-b2", to: "2", remember: false).status, 204)
            XCTAssertEqual(Config.shared.cloudBots.map(\.floor), ["", "3"])
            deck.layoutConfig.botFloor = 4
            XCTAssertEqual(floors(deck)[4], ["cloud-b0"], "the other, where new bots go now")
            XCTAssertEqual(deck.building(overflow: []).desks["cloud-b2"], 2, "the one put on 3F, at the desk it had")
        }
    }

    /// A bot at a key, where new bots go on the ground floor, is put there though every key is taken: by itself.
    func testABotAtAKeyIsPutOnTheGroundFloorThoughEveryKeyIsTaken() {
        movingBots {
            // Whether a key is free is asked of the settings themselves (`hasFreeKey`): three keys there too, put back after.
            let keysBefore = Config.shared.layout.keys
            defer { Config.shared.layout.keys = keysBefore }
            Config.shared.layout.keys = .fixed(3)
            let deck = quietDeck()
            deck.layoutConfig.layout.keys = .fixed(3)
            deck.layoutConfig.botFloor = 0
            for id in ["s0", "s1"] {
                deck.sessions.apply(["hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": "/work/\(id)"],
                                    agent: .claude, tab: "tty:/dev/\(id)", startedIn: "", at: id == "s0" ? 10 : 11)
            }
            Config.shared.cloudBots = [CloudBot(id: "mini", name: "Mini Manager")]
            deck.cloud.configure(Config.shared.cloudBots)
            XCTAssertEqual(keys(deck), ["s0", "s1", "cloud-mini"])
            XCTAssertEqual(deck.move("cloud-mini", to: "0", remember: false).status, 204)
            XCTAssertEqual(Config.shared.cloudBots.map(\.floor), ["ground"])
        }
    }

    func testABotWithNoDeskOnAnyFloorIsNamedForSetup() {
        let deck = quietDeck()
        deck.layoutConfig.layout.keys = .fixed(3)
        // Every desk taken by bots put there: the three keys downstairs, and every floor above.
        let ground = (0..<3).map { CloudBot(id: "g\($0)", name: "G\($0)", floor: "ground") }
        let above = (1..<Floor.count).flatMap { floor in
            (0..<room).map { CloudBot(id: "f\(floor)-\($0)", name: "F\(floor) \($0)", floor: CloudBot.floor(at: floor)) }
        }
        deck.cloud.configure(ground + above)
        XCTAssertEqual(deck.botsWithNoDesk().map(\.id), [])
        deck.cloud.configure(ground + above + [CloudBot(id: "late", name: "Late")])
        XCTAssertEqual(deck.botsWithNoDesk().map(\.id), ["late"])
    }

    // MARK: Setup

    private func sent(_ json: String) throws -> Any? {
        try XCTUnwrap(JSONSerialization.jsonObject(with: Data(json.utf8)) as? [String: Any])["botFloor"]
    }

    func testSetupsFloorForNewBotsIsAFloorAndNothingElse() throws {
        for (json, floor) in [(#"{"botFloor": 0}"#, 0), (#"{"botFloor": 1}"#, 1), (#"{"botFloor": 5}"#, 5), (#"{"botFloor": -0}"#, 0)] {
            let given = DeckRouter.botFloor(try sent(json))
            XCTAssertEqual(given.floor, floor, json)
            XCTAssertFalse(given.refused, json)
        }
        // A whole number that's no floor is left as it was, as on the Windows and Linux app; so is none. A body with
        // nothing else either, {} or {"botFloor": null}, gets the answer it always got here, as this route also takes
        // the Grok Bot app's switches; the Windows and Linux app's refuses it with a 400, as it always did.
        for left in [#"{"botFloor": 6}"#, #"{"botFloor": -1}"#, #"{"botFloor": null}"#, "{}"] {
            let given = DeckRouter.botFloor(try sent(left))
            XCTAssertNil(given.floor, left)
            XCTAssertFalse(given.refused, left)
        }
        // Anything that isn't a whole number is refused, as the Windows and Linux app refuses it.
        for bad in [#"{"botFloor": 1.5}"#, #"{"botFloor": 1.0}"#, #"{"botFloor": 1e0}"#, #"{"botFloor": true}"#, #"{"botFloor": "2"}"#,
                    #"{"botFloor": [1]}"#, #"{"botFloor": 99999999999999999999}"#] {
            let given = DeckRouter.botFloor(try sent(bad))
            XCTAssertTrue(given.refused, bad)
            XCTAssertNil(given.floor, bad)
        }
    }

    func testSetupsBotsCardOffersEveryFloorAndWhereNewBotsGo() throws {
        let page = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/setup.html"), encoding: .utf8)
        XCTAssertTrue(page.contains("textContent: 'New bots go to'"))
        XCTAssertTrue(page.contains("send({ botFloor: Number(newFloor.value) })"))
        XCTAssertTrue(page.contains("[['', `Its folder’s floor, or where new bots go (${floorName(newBots)})`], ['ground', floorName(0)]"))
        XCTAssertTrue(page.contains("every floor is full. Hide a bot, or free or add a desk, to make room."))
        XCTAssertFalse(page.contains("there can be 9 cloud bots"))
        XCTAssertFalse(page.contains("Upstairs, where there’s room"))
    }
}
