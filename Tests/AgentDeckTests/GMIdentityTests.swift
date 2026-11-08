import XCTest
import Security
@testable import AgentDeck

/// A Keychain that lives in a test: the same item across "restarts" of the app, as the real one would.
final class FakeGMTokenStore: GMTokenStore {
    var item: String?
    var refuses = false
    /// The item is there but can't be read (a locked Keychain, an app signed another way).
    var readFails = false
    /// Every token written, in order.
    private(set) var writes: [String] = []
    func read() -> GMTokenRead {
        if readFails { return .failed }
        return item.map { .token($0) } ?? .none
    }
    func write(_ token: String) -> Bool {
        guard !refuses else { return false }
        item = token
        writes.append(token)
        return true
    }
    func remove() { item = nil }
}

/// The GM is whoever holds the token: made when first needed, kept in memory and the Keychain,
/// in no file unless "Allow a GM started by hand" is on, rotated on request.
final class GMIdentityTests: XCTestCase {
    private var folder: URL!
    private var file: URL { folder.appendingPathComponent("gm-token") }
    private var store = FakeGMTokenStore()
    private var byHand = false

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("gm-identity-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        store = FakeGMTokenStore()
        byHand = false
    }

    override func tearDown() {
        try? FileManager.default.removeItem(at: folder)
    }

    private func identity() -> GMIdentity { GMIdentity(store: store, tokenFile: file, fileAllowed: { self.byHand }) }
    private var fileThere: Bool { FileManager.default.fileExists(atPath: file.path) }

    func testNothingIsMadeUntilItIsNeeded() {
        let gm = identity()
        XCTAssertNil(store.item, "making the object makes no token")
        XCTAssertFalse(gm.isToken("anything"), "with no token, nobody is the GM")
        XCTAssertFalse(gm.isToken(nil))
        XCTAssertFalse(gm.isToken(""))
        XCTAssertNil(store.item, "and asking never makes one")
        XCTAssertFalse(fileThere)
    }

    func testTheTokenIsMadeOnFirstNeedAndKeptInTheKeychainNotInAFile() throws {
        let gm = identity()
        let token = try XCTUnwrap(gm.token())
        XCTAssertEqual(token.count, 43)
        XCTAssertTrue(gm.isToken(token))
        XCTAssertEqual(store.item, token, "kept in the Keychain")
        XCTAssertFalse(fileThere, "and in no file")
        XCTAssertEqual(gm.token(), token, "asking again gives the same one")
    }

    func testTheTokenSurvivesAnAppRestart() throws {
        let token = try XCTUnwrap(identity().token())
        let restarted = identity()
        XCTAssertTrue(restarted.isToken(token), "a new run reads it from the Keychain")
        XCTAssertEqual(restarted.token(), token)
        XCTAssertFalse(fileThere)
    }

    func testOnlyTheTokenIsTheGM() throws {
        let gm = identity()
        let token = try XCTUnwrap(gm.token())
        XCTAssertFalse(gm.isToken(String(token.dropLast())))
        XCTAssertFalse(gm.isToken(token + "x"))
        XCTAssertFalse(gm.isToken(String(token.dropLast()) + (token.hasSuffix("A") ? "B" : "A")))
        XCTAssertFalse(gm.isToken("projects-fc"), "no session name makes a GM")
    }

    func testRotatingEndsTheOldTokenAtOnceAndUpdatesTheKeychain() throws {
        let gm = identity()
        let old = try XCTUnwrap(gm.token())
        let fresh = try XCTUnwrap(gm.rotate())
        XCTAssertNotEqual(old, fresh)
        XCTAssertFalse(gm.isToken(old))
        XCTAssertTrue(gm.isToken(fresh))
        XCTAssertEqual(store.item, fresh)
        XCTAssertFalse(fileThere)
        XCTAssertNotNil(GMIdentity(store: FakeGMTokenStore(), tokenFile: file, fileAllowed: { false }).rotate(), "rotating when there was none makes the first")
    }

    func testATokenThatCannotBeKeptLeavesTheOldOne() throws {
        let gm = identity()
        let old = try XCTUnwrap(gm.token())
        store.refuses = true
        XCTAssertNil(gm.rotate())
        XCTAssertTrue(gm.isToken(old))
    }

    func testAKeychainThatRefusesNeverFallsBackToTheFile() {
        store.refuses = true
        byHand = true
        let gm = identity()
        XCTAssertNil(gm.token())
        XCTAssertNil(gm.rotate())
        gm.applySetting()
        XCTAssertFalse(fileThere, "even with the file allowed, a token that can't be kept in the Keychain is not written down")
    }

    func testAKeychainItemThatIsNotATokenIsReplacedNotTrusted() throws {
        for bad in ["", "short", "has spaces in it, so it is not a token at all!!!!"] {
            store.item = bad
            let gm = identity()
            XCTAssertFalse(gm.isToken(bad.trimmingCharacters(in: .whitespacesAndNewlines)), "'\(bad)' is no token")
            let made = try XCTUnwrap(gm.token())
            XCTAssertEqual(made.count, 43)
            XCTAssertTrue(gm.isToken(made))
        }
    }

    // MARK: The token file, for a GM started by hand

    func testTheFileExistsOnlyWhileTheSettingIsOn() throws {
        let gm = identity()
        let token = try XCTUnwrap(gm.token())
        XCTAssertFalse(fileThere, "off by default")
        byHand = true
        gm.applySetting()
        XCTAssertEqual(try String(contentsOf: file, encoding: .utf8), token + "\n")
        XCTAssertEqual(try FileManager.default.attributesOfItem(atPath: file.path)[.posixPermissions] as? Int, 0o600, "only you can read it")
        let fresh = try XCTUnwrap(gm.rotate())
        XCTAssertEqual(try String(contentsOf: file, encoding: .utf8), fresh + "\n", "rotating updates the file too")
        byHand = false
        gm.applySetting()
        XCTAssertFalse(fileThere, "off again, the file is taken away")
        XCTAssertTrue(gm.isToken(fresh), "and the token still works")
    }

    func testAnOldBuildsFileIsRemovedAtStart() throws {
        try (String(repeating: "q", count: 43) + "\n").write(to: file, atomically: true, encoding: .utf8)
        identity().applySetting()
        XCTAssertFalse(fileThere)
        XCTAssertNil(store.item, "and no token is made for it")
    }

    func testAFileWrittenBesideTheAppIsNotTheGM() throws {
        let gm = identity()
        let real = try XCTUnwrap(gm.token())
        let planted = String(repeating: "q", count: 43)
        try (planted + "\n").write(to: file, atomically: true, encoding: .utf8)
        XCTAssertFalse(gm.isToken(planted), "the app reads memory and the Keychain, not the file")
        XCTAssertTrue(gm.isToken(real))
    }

    func testAGMStartedByHandIsSeenAfterItsTokenWorks() throws {
        byHand = true
        let gm = identity()
        let token = try XCTUnwrap(gm.token())
        XCTAssertFalse(gm.startedByHand)
        XCTAssertFalse(gm.isToken("not-the-token"))
        XCTAssertFalse(gm.startedByHand)
        XCTAssertTrue(gm.isToken(token))
        XCTAssertTrue(gm.startedByHand)
    }

    func testAGMIsNeverSeenWithTheSettingOff() throws {
        let gm = identity()
        let token = try XCTUnwrap(gm.token())
        XCTAssertTrue(gm.isToken(token))
        XCTAssertFalse(gm.startedByHand)
        byHand = true // turned on afterwards: the earlier use is not counted
        XCTAssertFalse(gm.startedByHand)
    }

    func testRotatingAndTurningTheSettingOffForgetTheGM() throws {
        byHand = true
        let gm = identity()
        let token = try XCTUnwrap(gm.token())
        XCTAssertTrue(gm.isToken(token))
        let fresh = try XCTUnwrap(gm.rotate())
        XCTAssertFalse(gm.startedByHand)
        XCTAssertTrue(gm.isToken(fresh))
        XCTAssertTrue(gm.startedByHand)
        byHand = false
        gm.applySetting()
        byHand = true
        XCTAssertFalse(gm.startedByHand)
    }

    func testFiringTheGMForgetsItWasStartedByHand() throws {
        byHand = true
        let gm = identity()
        let token = try XCTUnwrap(gm.token())
        XCTAssertTrue(gm.isToken(token))
        XCTAssertTrue(gm.startedByHand)
        gm.forgetStartedByHand()
        XCTAssertFalse(gm.startedByHand)
        XCTAssertTrue(gm.isToken(token)) // the token itself still works
    }

    func testOnlyAGMFireThatWorkedForgetsIt() {
        func hire(_ role: String) -> FrontDesk.Hire { FrontDesk.Hire(id: "h1", name: role, role: role, folder: "/x") }
        XCTAssertTrue(Deck.firedGM(hire("gm"), answer: .status(204)))
        XCTAssertTrue(Deck.firedGM(hire("gm"), answer: .json(["ok": true, "kept": "x"])))
        XCTAssertFalse(Deck.firedGM(hire("gm"), answer: .status(409, "no")))
        XCTAssertFalse(Deck.firedGM(hire("dev"), answer: .status(204)))
        XCTAssertFalse(Deck.firedGM(nil, answer: .status(204)))
    }

    func testTheManagerStateCarriesTheFlagOnlyWithTheSettingOn() {
        XCTAssertEqual(DeckRouter.managerState(enabled: true, byHand: true, startedByHand: true)["gmStartedByHand"] as? Bool, true)
        XCTAssertEqual(DeckRouter.managerState(enabled: true, byHand: false, startedByHand: true)["gmStartedByHand"] as? Bool, false)
        XCTAssertEqual(DeckRouter.managerState(enabled: true, byHand: true, startedByHand: false)["gmStartedByHand"] as? Bool, false)
    }

    /// The page says where "Allow a General Manager started by hand" writes the token, never the token.
    func testTheManagerStateSaysWhereTheTokenFileIs() {
        let file = URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library/Application Support/AgentDeck/gm-token")
        let state = DeckRouter.managerState(enabled: false, byHand: false, startedByHand: false, tokenFile: file)
        XCTAssertEqual(state["gmTokenFile"] as? String, file.path)
        XCTAssertEqual(state["gmTokenFileShown"] as? String, "~/Library/Application Support/AgentDeck/gm-token")
        XCTAssertEqual(Set(state.keys), ["enabled", "gmByHand", "gmStartedByHand", "gmTokenFile", "gmTokenFileShown"])
    }

    // MARK: The by-hand GM's session

    private let sessionID = "0b1f6c1e-52aa-4d0e-9a57-3c1d2f4e8a90"

    func testASessionIsRememberedOnlyAfterTheTokenWorkedWithTheSettingOn() throws {
        byHand = true
        let gm = identity()
        gm.rememberSession(sessionID)
        XCTAssertNil(gm.session, "no token seen yet")
        XCTAssertTrue(gm.isToken(try XCTUnwrap(gm.token())))
        XCTAssertNil(gm.session, "and one named before the token worked isn't kept for later")
        XCTAssertTrue(gm.isToken(try XCTUnwrap(gm.token())))
        gm.rememberSession(sessionID)
        XCTAssertEqual(gm.session, sessionID)
        gm.rememberSession("bad id with spaces")
        XCTAssertEqual(gm.session, sessionID, "a malformed id changes nothing")
        byHand = false
        XCTAssertNil(gm.session, "with the setting off it isn't the GM's")
        gm.rememberSession(sessionID)
        byHand = true
        XCTAssertEqual(gm.session, sessionID, "and off never replaced it")
    }

    func testAWrongTokenNeverSetsTheSession() throws {
        byHand = true
        let gm = identity()
        _ = gm.token()
        XCTAssertFalse(gm.isToken(String(repeating: "x", count: 43)))
        gm.rememberSession(sessionID)
        XCTAssertNil(gm.session)
    }

    func testRotateSettingOffAndAFireForgetTheSession() throws {
        byHand = true
        let gm = identity()
        let token = try XCTUnwrap(gm.token())
        XCTAssertTrue(gm.isToken(token))
        gm.rememberSession(sessionID)
        let fresh = try XCTUnwrap(gm.rotate())
        XCTAssertTrue(gm.isToken(fresh)) // seen again, but the session was forgotten with the old token
        XCTAssertNil(gm.session)
        gm.rememberSession(sessionID)
        byHand = false
        gm.applySetting()
        byHand = true
        XCTAssertTrue(gm.isToken(fresh))
        XCTAssertNil(gm.session, "turning the setting off forgot it")
        gm.rememberSession(sessionID)
        gm.forgetStartedByHand()
        XCTAssertTrue(gm.isToken(fresh))
        XCTAssertNil(gm.session, "a fire forgot it")
    }

    func testTheSessionFollowsAClearAndIsNotSavedAnywhere() throws {
        byHand = true
        let gm = identity()
        XCTAssertTrue(gm.isToken(try XCTUnwrap(gm.token())))
        gm.rememberSession("old")
        gm.moveSession(from: "other", to: "x")
        XCTAssertEqual(gm.session, "old")
        gm.moveSession(from: "old", to: "new")
        XCTAssertEqual(gm.session, "new")
        // A new run of the app knows nothing of it, and neither the Keychain nor the token file holds it.
        XCTAssertNil(identity().session)
        XCTAssertFalse(store.item?.contains("new") ?? false)
        XCTAssertFalse((try String(contentsOf: file, encoding: .utf8)).contains("new"))
    }

    // MARK: Restarts (T758)

    private var sessionFile: URL { folder.appendingPathComponent(GMIdentity.sessionFileName) }
    private var sessionFileThere: Bool { FileManager.default.fileExists(atPath: sessionFile.path) }

    /// The app starting again: a new identity over the same store and folder, the GM's session live, the setting applied.
    private func restart(live: String? = nil) -> GMIdentity {
        let gm = identity()
        let live = live ?? sessionID
        gm.isLive = { $0 == live }
        gm.applySetting()
        return gm
    }

    func testARoleTakenUnderAnOlderBuildComesBackOnlyAfterOneGMRequest() throws {
        // An older build: the token in the Keychain and its file, but no gm-session (it never wrote one).
        byHand = true
        let token = String(repeating: "aB3_-", count: 8) + "xyz"
        store.item = token
        try (token + "\n").write(to: file, atomically: true, encoding: .utf8)
        let first = restart()
        XCTAssertNil(first.session, "nothing to restore, and nothing guessed from the live session")
        XCTAssertFalse(first.startedByHand)
        XCTAssertFalse(sessionFileThere)
        XCTAssertEqual(store.item, token, "the same token: no rotation")
        // The GM's first request writes it.
        XCTAssertTrue(first.isToken(token))
        first.rememberSession(sessionID)
        XCTAssertEqual(try String(contentsOf: sessionFile, encoding: .utf8), sessionID + "\n")
        // And the next restart restores it.
        let second = restart()
        XCTAssertEqual(second.session, sessionID)
        XCTAssertTrue(second.startedByHand)
        XCTAssertEqual(store.item, token)
    }

    func testAKeychainReadErrorAtLaunchKeepsTheTokenFromTheFileAndTheRole() throws {
        byHand = true
        let token = try XCTUnwrap(identity().token())
        let gm = identity()
        XCTAssertTrue(gm.isToken(token))
        gm.rememberSession(sessionID)
        XCTAssertTrue(sessionFileThere)
        let before = store.writes.count
        store.readFails = true
        let restarted = restart()
        XCTAssertTrue(restarted.isToken(token), "the same token, from the private file")
        XCTAssertEqual(restarted.token(), token)
        XCTAssertEqual(Array(store.writes.dropFirst(before)), [], "nothing written to the Keychain, and no new token made")
        XCTAssertTrue(sessionFileThere, "gm-session kept")
        XCTAssertEqual(restarted.session, sessionID, "and the role restored")
        XCTAssertTrue(restarted.startedByHand)
        XCTAssertEqual(try String(contentsOf: file, encoding: .utf8), token + "\n")
    }

    func testARealNoItemMakesANewTokenAndDeletesTheSession() throws {
        byHand = true
        let gm = identity()
        let old = try XCTUnwrap(gm.token())
        XCTAssertTrue(gm.isToken(old))
        gm.rememberSession(sessionID)
        store.item = nil // the Keychain says "not found"
        let restarted = restart()
        let fresh = try XCTUnwrap(restarted.token())
        XCTAssertNotEqual(fresh, old)
        XCTAssertFalse(restarted.isToken(old))
        XCTAssertFalse(sessionFileThere, "a new token is a new GM")
        XCTAssertNil(restarted.session)
        XCTAssertEqual(try String(contentsOf: file, encoding: .utf8), fresh + "\n")
    }

    func testAKeychainErrorWithABadOrNoFileGivesNoTokenAndNoRotation() throws {
        let token = String(repeating: "aB3_-", count: 8) + "xyz"
        for bad in [nil, "", "short", "has spaces in it, so it is not a token at all!!!!"] as [String?] {
            for allowed in [true, false] {
                try? FileManager.default.removeItem(at: file)
                if let bad { try bad.write(to: file, atomically: true, encoding: .utf8) }
                try (sessionID + "\n").write(to: sessionFile, atomically: true, encoding: .utf8)
                store = FakeGMTokenStore()
                store.item = token
                store.readFails = true
                byHand = allowed
                let gm = restart()
                let what = "file \(bad.map { "'\($0)'" } ?? "missing"), by-hand \(allowed)"
                XCTAssertNil(gm.token(), what)
                XCTAssertFalse(gm.isToken(token), what)
                XCTAssertTrue(store.writes.isEmpty, "no token made: \(what)")
                XCTAssertEqual(store.item, token, what)
                XCTAssertNil(gm.session, what)
                if allowed {
                    XCTAssertTrue(sessionFileThere, "gm-session kept for a later launch: \(what)")
                    XCTAssertEqual(try? String(contentsOf: file, encoding: .utf8), bad, "the file left as it was: \(what)")
                }
                // Rotating, which the owner asks for, still makes one.
                XCTAssertNotNil(gm.rotate(), what)
                XCTAssertEqual(store.writes.count, 1, what)
            }
        }
    }

    func testOnlyNotFoundIsNoItemInTheKeychain() {
        XCTAssertEqual(GMKeychain.result(status: errSecSuccess, data: Data("abc".utf8)), .token("abc"))
        XCTAssertEqual(GMKeychain.result(status: errSecSuccess, data: Data([0xff, 0xfe])), .token(""), "not text: no token, replaced like one")
        XCTAssertEqual(GMKeychain.result(status: errSecItemNotFound, data: nil), GMTokenRead.none)
        for status in [errSecInteractionNotAllowed, errSecAuthFailed, errSecUserCanceled, errSecNotAvailable, errSecDecode] {
            XCTAssertEqual(GMKeychain.result(status: status, data: nil), .failed, "status \(status)")
        }
    }

    func testTheTokenFileStoreTellsNoFileFromAnUnreadableOne() throws {
        let store = GMTokenFile(file: folder.appendingPathComponent("store"))
        XCTAssertEqual(store.read(), GMTokenRead.none)
        XCTAssertTrue(store.write("abc"))
        XCTAssertEqual(store.read(), .token("abc"))
        try Data().write(to: store.file)
        XCTAssertEqual(store.read(), .token(""), "empty: no token, replaced like one")
        try FileManager.default.removeItem(at: store.file)
        try FileManager.default.createDirectory(at: store.file, withIntermediateDirectories: false)
        XCTAssertEqual(store.read(), .failed, "there, but it can't be read")
    }

    func testSessionIdsAreCheckedLikeTheCommandChecksThem() {
        XCTAssertEqual(GMIdentity.validSession(sessionID), sessionID)
        XCTAssertEqual(GMIdentity.validSession("a_B-9"), "a_B-9")
        for bad in [nil, "", "a b", "a\nb", "séance", String(repeating: "a", count: 65), "a/b"] as [String?] {
            XCTAssertNil(GMIdentity.validSession(bad), bad ?? "nil")
        }
        XCTAssertEqual(GMIdentity.sessionHeader, "x-smolcorp-session")
    }

    func testTheSettingIsOffByDefault() {
        XCTAssertFalse(Config().gmByHand)
    }

    func testTheTokenFileIsInTheSupportFolderAndNamedForTheHeader() {
        XCTAssertEqual(Paths.gmToken.lastPathComponent, "gm-token")
        XCTAssertEqual(Paths.gmToken.deletingLastPathComponent(), Paths.support)
        XCTAssertEqual(GMIdentity.header, "x-smolcorp-gm-token")
        XCTAssertEqual(GMIdentity.environment, "SMOLCORP_GM_TOKEN")
    }

    func testTheFixtureTokenIsFakeAndNotAValidOne() throws {
        let fake = try String(contentsOf: FrontDeskFixtures.url("gm-token.txt"), encoding: .utf8).trimmingCharacters(in: .whitespacesAndNewlines)
        XCTAssertTrue(fake.hasPrefix("fake-"))
        XCTAssertNotEqual(fake.count, 43, "the Go port's tests can't mistake it for a made token")
    }
}

/// Where the shared fixtures for the Linux/Windows port are.
enum FrontDeskFixtures {
    static func url(_ name: String) -> URL {
        URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("fixtures/frontdesk/\(name)")
    }
}
