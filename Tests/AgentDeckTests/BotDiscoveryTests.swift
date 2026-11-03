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
        XCTAssertTrue(result.bots.allSatisfy { $0.discovered && !$0.pinned && $0.floor == "upstairs" && $0.source.kind == "grokBotApp" })
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
        let nine = (0..<CloudBot.most).map { CloudBot(id: "b\($0)", name: "B\($0)") }
        let result = CloudBot.discover([mini], in: nine, hidden: [])
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
