import XCTest
import Security
@testable import AgentDeck

/// FL0: the Macs this one joined, keys and all (`PeerKeys`), one item of the app's in the login keychain opened by its
/// path, through SK1's `Keychain`. Every Keychain row runs on throwaway keychain files of this test's own, made with a
/// password it knows and never put in the search list or made the default, as KeychainRecordKeyTests' are. Nothing
/// here asks the owner's login keychain or opens the gate. "Another program" is this test, with SecItem's own calls on
/// the same file (probes K7, K10, K11).
final class PeerKeysTests: XCTestCase {
    private var folder: URL!
    private var made: [SecKeychain] = []
    private let list = #"[{"address":"100.101.102.103:7710","name":"Studio","joinedAt":"2026-11-14T09:00:00Z","key":"fake-peer-key-studio"}]"#
    private let other = #"[{"address":"100.64.0.9:7710","name":"Mini","joinedAt":"2026-11-14T10:00:00Z","key":"fake-peer-key-mini"}]"#

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("fl0-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
    }

    override func tearDownWithError() throws {
        for keychain in made { SecKeychainDelete(keychain) }
        made = []
        try? FileManager.default.removeItem(at: folder)
    }

    /// A throwaway keychain file, unlocked with a password this test knows, never in the search list.
    private func throwaway(_ name: String) throws -> SecKeychain {
        let password = "FAKE-throwaway-keychain"
        var keychain: SecKeychain?
        let status = SecKeychainCreate(folder.appendingPathComponent("\(name).keychain-db").path, UInt32(password.utf8.count), password, false, nil, &keychain)
        let made = try XCTUnwrap(keychain, "SecKeychainCreate: \(status)")
        self.made.append(made)
        return made
    }

    private var item: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: PeerKeys.service, kSecAttrAccount as String: PeerKeys.account]
    }

    /// None is none; the list kept reads back as it was written; removed, it's none again.
    func testTheMacsJoinedAreOneItemReadBack() throws {
        let store = PeerKeys.keychain(try throwaway("one"), open: true)
        XCTAssertNil(store.read())
        XCTAssertTrue(store.absent())
        XCTAssertTrue(store.write(list))
        XCTAssertEqual(store.read(), list)
        XCTAssertFalse(store.absent())
        XCTAssertTrue(store.write(other), "a list written over the app's own")
        XCTAssertEqual(store.read(), other)
        store.remove()
        XCTAssertTrue(store.absent())
        XCTAssertNil(store.read())
    }

    /// The date check (K7, K10): another program puts a list of its own in the app's item, which moves its
    /// modification date off its creation date. That list is never read, and the item is never taken for none, so the
    /// core refuses to join rather than write over it. The app's own write then replaces it.
    func testAListAnotherProgramChangedIsNeverReadNorNone() throws {
        let keychain = try throwaway("moved")
        let store = PeerKeys.keychain(keychain, open: true)
        XCTAssertTrue(store.write(list))
        Thread.sleep(forTimeInterval: 1.2) // the dates are kept to the second
        var match = item
        match[kSecMatchSearchList as String] = [keychain]
        XCTAssertEqual(SecItemUpdate(match as CFDictionary, [kSecValueData as String: Data(other.utf8)] as CFDictionary), errSecSuccess)
        XCTAssertNil(store.read(), "the changed list was read")
        XCTAssertFalse(store.absent(), "the changed list was taken for none")
        XCTAssertTrue(store.write(list))
        XCTAssertEqual(store.read(), list)
    }

    /// K11: a list another program adds under the app's free name carries no seal of the app's: never read, never none.
    func testAListAnotherProgramAddedIsNeverReadNorNone() throws {
        let keychain = try throwaway("planted")
        let store = PeerKeys.keychain(keychain, open: true)
        XCTAssertTrue(store.write(other), "the items' key made, as at a first sealed add")
        store.remove()
        var planted = item
        planted[kSecValueData as String] = Data(list.utf8)
        planted[kSecUseKeychain as String] = keychain
        XCTAssertEqual(SecItemAdd(planted as CFDictionary, nil), errSecSuccess)
        XCTAssertNil(store.read(), "the planted list was read")
        XCTAssertFalse(store.absent(), "the planted list was taken for none")
    }

    /// RSBXPLANf F1: only the pinned keychain is asked. A list planted in another keychain file is never found there,
    /// and the app's list never goes there.
    func testOnlyThePinnedKeychainIsAsked() throws {
        let pinned = try throwaway("pinned"), elsewhere = try throwaway("elsewhere")
        var planted = item
        planted[kSecValueData as String] = Data(other.utf8)
        planted[kSecUseKeychain as String] = elsewhere
        XCTAssertEqual(SecItemAdd(planted as CFDictionary, nil), errSecSuccess)
        let store = PeerKeys.keychain(pinned, open: true)
        XCTAssertTrue(store.absent(), "a list in another keychain was found")
        XCTAssertTrue(store.write(list))
        var there = item
        there[kSecMatchSearchList as String] = [elsewhere]
        there[kSecReturnData as String] = true
        there[kSecUseAuthenticationUI as String] = kSecUseAuthenticationUIFail
        var found: CFTypeRef?
        XCTAssertEqual(SecItemCopyMatching(there as CFDictionary, &found), errSecSuccess)
        XCTAssertEqual((found as? Data).flatMap { String(data: $0, encoding: .utf8) }, other, "the app's list went to another keychain")
    }

    /// The tests' own switch never reaches the owner's login keychain, and a shut gate (the app without its instance
    /// lock, KeychainGate) is a failed read, never "none": no list is written over the Macs joined before.
    func testAShutGateIsNeverNone() {
        XCTAssertFalse(KeychainGate.isOpen)
        let store = PeerKeys.keychain(open: true)
        XCTAssertNil(store.read())
        XCTAssertFalse(store.absent())
        XCTAssertFalse(store.write(list))
    }

    /// A test copy keeps the list in a file of its own, readable by you alone; the test suite in its own folder. The
    /// Keychain is never asked by either.
    func testATestCopyKeepsTheMacsJoinedInAFile() throws {
        let support = folder.appendingPathComponent("copy", isDirectory: true)
        let store = PeerKeys.defaultStore(testCopy: true, support: support, underTest: false)
        let file = support.appendingPathComponent(TestCopySecrets.peerKeysName)
        XCTAssertTrue(store.absent())
        XCTAssertTrue(store.write(list))
        XCTAssertEqual(store.read(), list)
        XCTAssertFalse(store.absent())
        let mode = try XCTUnwrap(FileManager.default.attributesOfItem(atPath: file.path)[.posixPermissions] as? NSNumber)
        XCTAssertEqual(mode.intValue & 0o777, 0o600)
        store.remove()
        XCTAssertTrue(store.absent())
        // The test suite's: in its own support folder, never the owner's item.
        let suite = PeerKeys.defaultStore(testCopy: false, support: nil, underTest: true)
        let suiteFile = Paths.testSupport.appendingPathComponent(TestCopySecrets.peerKeysName)
        defer { try? FileManager.default.removeItem(at: suiteFile) }
        XCTAssertTrue(suite.write(other))
        XCTAssertEqual(TestCopySecrets.read(suiteFile), other)
        XCTAssertEqual(PeerKeys.defaultStore().read(), other, "the suite's store, as the bridge's `mac` has it")
    }
}
