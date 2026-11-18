import XCTest
@testable import AgentDeck

/// The Grok Bot app's bots, at desks by themselves: found, gone, pinned and
/// hidden, and never a bot you set yourself taken away.
final class BotDiscoveryTests: XCTestCase {
    private let mini = GrokBotApp.Row(id: "aa11", name: "Smolcorp Mini")
    private let scout = GrokBotApp.Row(id: "bb22", name: "X scout")

    private func found(_ name: String, _ botId: String, pinned: Bool = false) -> CloudBot {
        CloudBot(id: CloudBot.freeID(for: .init(id: botId, name: name), among: []), name: name,
                 source: .init(kind: "grokBotApp", bot: name, botId: botId), discovered: true, pinned: pinned)
    }

    func testEachBotInTheAppGetsADesk() {
        let result = CloudBot.discover([mini, scout], in: [], hidden: [])
        XCTAssertEqual(result.bots.map(\.id), ["smolcorp-mini", "x-scout"])
        XCTAssertEqual(result.bots.map(\.source.botId), ["aa11", "bb22"])
        XCTAssertTrue(result.bots.allSatisfy { $0.discovered && !$0.pinned && $0.floor == "" && $0.source.kind == "grokBotApp" })
    }

    func testABotYouSetIsntFoundTwiceByItsIdOrItsName() {
        let yours = CloudBot(id: "mini", name: "Mini", source: .init(kind: "grokBotApp", bot: "Smolcorp Mini"))
        let byID = CloudBot(id: "scout", name: "Scout", source: .init(kind: "grokBotApp", botId: "BB22"))
        var named = yours
        named.source.botId = "aa11" // given its id there, found by its name
        XCTAssertEqual(CloudBot.discover([mini, scout], in: [yours, byID], hidden: []).bots, [named, byID])
    }

    func testAFoundBotGoneFromTheAppGoesUnlessPinnedAndYoursNeverGoes() {
        let gone = found("Old", "cc33"), kept = found("Kept", "dd44", pinned: true)
        let yours = CloudBot(id: "yours", name: "Yours", source: .init(kind: "grokBotApp", botId: "ee55"))
        let result = CloudBot.discover([mini], in: [gone, kept, yours], hidden: [])
        XCTAssertEqual(result.bots.map(\.id), ["kept", "yours", "smolcorp-mini"])
    }

    func testAFoundBotYouRenamedKeepsYourNameWhileOneYouOnlyPinnedFollowsTheApp() {
        var renamed = found("Smolcorp Mini", "aa11", pinned: true)
        renamed.name = "Boss" // renamed in setup, which pins it
        let onlyPinned = found("X scout", "bb22", pinned: true)
        let app = [GrokBotApp.Row(id: "aa11", name: "Mini Two"), GrokBotApp.Row(id: "bb22", name: "Scout Two")]
        let result = CloudBot.discover(app, in: [renamed, onlyPinned], hidden: [])
        XCTAssertEqual(result.bots.first { $0.source.botId == "aa11" }?.name, "Boss", "your name stays")
        XCTAssertEqual(result.bots.first { $0.source.botId == "aa11" }?.source.bot, "Smolcorp Mini")
        XCTAssertEqual(result.bots.first { $0.source.botId == "bb22" }?.name, "Scout Two", "a pinned bot not renamed follows the app")
        XCTAssertEqual(renamed.setupJSON(in: app)["name"] as? String, "Boss", "setup's list agrees with the config")
    }

    /// A group chat in the app, between its bots: not a bot. Made up.
    private let group = GrokBotApp.Row(id: "cc33", name: "Bot Room", isGroup: true)

    func testAGroupChatInTheAppGetsNoDesk() {
        let result = CloudBot.discover([mini, group, scout], in: [], hidden: [])
        XCTAssertEqual(result.bots.map(\.source.botId), ["aa11", "bb22"])
        // Nor waits for room.
        let full = (0..<CloudBot.most).map { CloudBot(id: "b\($0)", name: "B\($0)") }
        XCTAssertEqual(CloudBot.discover([group], in: full, hidden: []).noRoom, [])
    }

    func testAGroupChatThatHadADeskLosesItPinnedOrNotAndTheBotsKeepTheirs() {
        var bots = [found("Smolcorp Mini", "aa11"), found("Bot Room", "cc33", pinned: true), found("X scout", "bb22")]
        for i in bots.indices { bots[i].floor = "3" }
        var unpinned = bots[1]
        unpinned.pinned = false
        let result = CloudBot.discover([mini, group, scout], in: bots, hidden: [])
        XCTAssertEqual(result.bots, [bots[0], bots[2]], "the bots as they were, on their floor")
        XCTAssertEqual(CloudBot.discover([mini, group, scout], in: [unpinned], hidden: []).bots.map(\.source.botId), ["aa11", "bb22"])
        // One you set yourself on it stays yours, as any bot you set does.
        let yours = CloudBot(id: "room", name: "Room", source: .init(kind: "grokBotApp", botId: "cc33"))
        XCTAssertEqual(CloudBot.discover([group], in: [yours], hidden: []).bots, [yours])
    }

    func testAGroupChatNeverCountsTowardTheMostAndItsPlaceGoesToABot() {
        // Full, one of them the group chat: it goes, and a bot waiting takes its place.
        let eight = (0..<CloudBot.most - 1).map { CloudBot(id: "b\($0)", name: "B\($0)") }
        let result = CloudBot.discover([group, mini], in: eight + [found("Bot Room", "cc33")], hidden: [])
        XCTAssertEqual(result.bots.count, CloudBot.most)
        XCTAssertEqual(result.bots.last?.source.botId, "aa11")
        XCTAssertEqual(result.noRoom, [], "setup says no bot is waiting")
    }

    func testABotRenamedInTheAppIsKeptByItsNameThereNow() {
        // Found as "Lima", now "Onyx" in the app: kept as "Onyx", with the same id, address and floor.
        var lima = found("Lima", "aa11")
        lima.floor = "4"
        let yours = CloudBot(id: "scout", name: "Scout", source: .init(kind: "grokBotApp", bot: "Scout", botId: "bb22"))
        let renamed = CloudBot.discover([.init(id: "aa11", name: "Onyx"), scout], in: [lima, yours], hidden: [])
        XCTAssertEqual(renamed.bots.map(\.name), ["Onyx", "Scout"], "a bot you named keeps your name in your config")
        XCTAssertEqual(renamed.bots[0].id, "lima")
        XCTAssertEqual(renamed.bots[0].floor, "4")
        XCTAssertEqual(renamed.bots[0].source.bot, "Onyx")
        XCTAssertEqual(renamed.bots[0].source.botId, "aa11")
        // Setup's list says each one's name in the app now, by its id; with no roster, the one kept.
        let rows = [GrokBotApp.Row(id: "AA11", name: "Onyx"), scout]
        XCTAssertEqual([lima, yours].map { $0.setupJSON(in: rows)["name"] as? String }, ["Onyx", "X scout"])
        XCTAssertEqual([lima, yours].map { $0.setupJSON(in: [])["name"] as? String }, ["Lima", "Scout"])
        XCTAssertEqual(CloudBot(id: "x", name: "X").setupJSON(in: rows)["name"] as? String, "X", "not from the app: its own")
    }

    func testAnAppThatCantBeReadChangesNothing() {
        let bots = [found("Old", "cc33")]
        XCTAssertEqual(CloudBot.discover(nil, in: bots, hidden: []).bots, bots)
        // An empty roster is read: the app has no bots.
        XCTAssertEqual(CloudBot.discover([], in: bots, hidden: []).bots, [])
    }

    func testAHiddenBotIsNeitherFoundNorKept() {
        let result = CloudBot.discover([mini, scout], in: [found("Smolcorp Mini", "aa11")], hidden: ["AA11"])
        XCTAssertEqual(result.bots.map(\.source.botId), ["bb22"])
    }

    func testPastTheMostThereIsNoRoomAndSetupIsTold() {
        let full = (0..<CloudBot.most).map { CloudBot(id: "b\($0)", name: "B\($0)") }
        let result = CloudBot.discover([mini], in: full, hidden: [])
        XCTAssertEqual(result.bots.count, CloudBot.most)
        XCTAssertEqual(result.noRoom, [mini])
    }

    func testAFoundBotsIdIsItsNameMadeUnique() {
        XCTAssertEqual(CloudBot.freeID(for: .init(id: "aa11", name: "Smolcorp Mini!"), among: ["smolcorp-mini"]), "smolcorp-mini-2")
        XCTAssertEqual(CloudBot.freeID(for: .init(id: "abcdef0123", name: "✨✨"), among: []), "grok-abcdef01")
    }

    func testSetupSendingABotBackUnchangedDoesntPinIt() {
        let bot = found("Smolcorp Mini", "aa11")
        var edited = bot
        XCTAssertTrue(bot.sameAsSet(edited))
        edited.floor = "4"
        XCTAssertFalse(bot.sameAsSet(edited))
    }

    func testOlderConfigsBotsAreYoursAndAutoDetectStartsOn() throws {
        let bot = try JSONDecoder().decode(CloudBot.self, from: Data(#"{"id": "mini", "name": "Mini"}"#.utf8))
        XCTAssertFalse(bot.discovered)
        XCTAssertFalse(bot.pinned)
        XCTAssertTrue(Config().botsAutoDetect)
    }

    func testABotSetBeforeIdsIsHiddenByItsIdInTheAppFoundByName() {
        let byName = CloudBot(id: "mini", name: "Mini", source: .init(kind: "grokBotApp", bot: "Smolcorp Mini"))
        XCTAssertEqual(byName.appID(in: [mini, scout]), "aa11")
        let noSource = CloudBot(id: "scout", name: "X scout")
        XCTAssertEqual(noSource.appID(in: [mini, scout]), "bb22")
        XCTAssertNil(CloudBot(id: "other", name: "Other").appID(in: [mini, scout]))
        // Hidden by that id, discovery doesn't give it back under another.
        XCTAssertEqual(CloudBot.discover([mini, scout], in: [], hidden: ["aa11", "bb22"]).bots, [])
    }

    func testABotSetByNameGetsItsIdSoARenameThereDoesntGiveItASecondDesk() {
        let byName = CloudBot(id: "mini", name: "Mini", source: .init(kind: "grokBotApp", bot: "Smolcorp Mini"))
        let first = CloudBot.discover([mini], in: [byName], hidden: [])
        XCTAssertEqual(first.bots.count, 1)
        XCTAssertEqual(first.bots[0].source.botId, "aa11")
        // Renamed in the app: still the one bot.
        let renamed = CloudBot.discover([.init(id: "aa11", name: "Mini Manager")], in: first.bots, hidden: [])
        XCTAssertEqual(renamed.bots.map(\.id), ["mini"])
    }

    func testABotWithNoSourceIsTheAppsBotOfItsName() {
        let yours = CloudBot(id: "scout", name: "X scout")
        XCTAssertEqual(CloudBot.discover([scout], in: [yours], hidden: []).bots, [yours])
    }

    func testTwoBotsWithTheSameNameGetADeskEach() {
        let result = CloudBot.discover([.init(id: "aa11", name: "Scout"), .init(id: "bb22", name: "Scout")], in: [], hidden: [])
        XCTAssertEqual(result.bots.map(\.source.botId), ["aa11", "bb22"])
        XCTAssertEqual(Set(result.bots.map(\.id)).count, 2)
    }

    func testANewBotWithARenamedOnesOldNameGetsADesk() {
        let first = CloudBot.discover([.init(id: "aa11", name: "Mini")], in: [], hidden: [])
        let later = CloudBot.discover([.init(id: "aa11", name: "Mini Manager"), .init(id: "cc33", name: "Mini")], in: first.bots, hidden: [])
        XCTAssertEqual(later.bots.map(\.source.botId), ["aa11", "cc33"])
    }

    func testAGuessByALikeNameIsntKept() {
        let guessed = CloudBot(id: "mini", name: "Mini", source: .init(kind: "grokBotApp", bot: "Smolcorp Mini"))
        let result = CloudBot.discover([.init(id: "aa11", name: "Smolcorp Mini Manager")], in: [guessed], hidden: [])
        XCTAssertEqual(result.bots.first?.source.botId, "", "found by a guess, it's looked at again next time")
        XCTAssertEqual(result.bots.count, 1, "and it's still the one desk")
    }
}
