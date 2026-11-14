import XCTest
@testable import AgentDeck

/// Who sits on which floor: those put on one, by the office or by their
/// folder's rule, and everyone else where there's room, each keeping their desk.
final class BuildingTests: XCTestCase {

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
}
