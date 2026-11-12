import XCTest
import Security
@testable import AgentDeck

/// Where the General Manager's token is kept: the Keychain item, or a file in a test copy's own folder.
final class GMIdentityTests: XCTestCase {
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("gm-identity-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
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

    func testTheSettingIsOffByDefault() {
        XCTAssertFalse(Config().gmByHand)
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
