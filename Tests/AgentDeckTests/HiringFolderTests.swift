import XCTest
@testable import AgentDeck

/// Hiring can't be switched on without a folder to hire into, where hiring can
/// open at all; a free desk says why when an older config has it on anyway.
final class HiringFolderTests: XCTestCase {
    private var folder: String!

    override func setUp() {
        folder = NSTemporaryDirectory() + "hiring-folder-\(UUID().uuidString)"
        try? FileManager.default.createDirectory(atPath: folder, withIntermediateDirectories: true)
    }

    override func tearDown() {
        try? FileManager.default.removeItem(atPath: folder)
    }

    func testSwitchingHiringOnWithNoFolderIsRefused() {
        let config = Config()
        for hireFolder in ["", "/nowhere/at/all"] {
            XCTAssertEqual(Deck.hiringRefusal(enabled: true, hireFolder: hireFolder, config: config, canHireHere: true),
                           "Hiring needs a folder: choose one in setup.", "folder \(hireFolder)")
        }
        XCTAssertNil(Deck.hiringRefusal(enabled: false, hireFolder: "", config: config, canHireHere: true), "turning it off is always allowed")
    }

    func testAFloorsOwnFolderIsEnough() {
        let config = Config()
        config.floorRules = [FloorRule(folder: folder, floor: 2)]
        XCTAssertNil(Deck.hiringRefusal(enabled: true, hireFolder: "", config: config, canHireHere: true))
        XCTAssertNil(Deck.hiringReason(withHiringOn(config), canHireHere: true))
    }

    func testTheGlobalFolderIsEnough() {
        let config = Config()
        XCTAssertNil(Deck.hiringRefusal(enabled: true, hireFolder: folder, config: config, canHireHere: true))
        config.hireFolder = folder
        XCTAssertNil(Deck.hiringReason(withHiringOn(config), canHireHere: true))
    }

    func testAnOlderConfigWithHiringOnAndNoFolderSaysWhy() {
        let config = withHiringOn(Config())
        XCTAssertFalse(Deck.canHire(config))
        XCTAssertEqual(Deck.hiringReason(config, canHireHere: true), "Hiring needs a folder: choose one in setup.")
        config.hiringEnabled = false
        XCTAssertNil(Deck.hiringReason(config, canHireHere: true), "hiring off: nothing to say")
    }

    func testClearingTheFolderWhileHiringIsOnIsRefused() {
        let config = withHiringOn(Config())
        config.hireFolder = folder
        XCTAssertNotNil(Deck.hiringRefusal(enabled: config.hiringEnabled, hireFolder: "", config: config, canHireHere: true))
    }

    func testWhereHiringCantOpenTheSwitchNeedsNoFolder() {
        let config = Config()
        XCTAssertNil(Deck.hiringRefusal(enabled: true, hireFolder: "", config: config, canHireHere: false))
        XCTAssertNil(Deck.hiringReason(withHiringOn(config), canHireHere: false), "no reason blames the folder")
    }

    private func withHiringOn(_ config: Config) -> Config {
        config.hiringEnabled = true
        return config
    }
}
