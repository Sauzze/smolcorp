import XCTest
@testable import AgentDeck

/// A test copy keeps the GM's token and the Summary key in files of its own,
/// and never asks the Keychain (a stand-in that fails the test if it's asked).
final class TestCopySecretsTests: XCTestCase {
    private var folder: URL!
    private let token = String(repeating: "aB3_-", count: 8) + "xyz"
    private let key = "sk-" + "ant-" + "fake-" + String(repeating: "T3st", count: 6)

    private struct NoKeychainToken: GMTokenStore {
        let fail: (String) -> Void
        func read() -> GMTokenRead { fail("GM read"); return .none }
        func write(_ token: String) -> Bool { fail("GM write"); return false }
        func remove() { fail("GM remove") }
    }

    override func setUp() {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("test-copy-secrets-\(UUID().uuidString)", isDirectory: true)
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o755])
    }

    override func tearDown() { try? FileManager.default.removeItem(at: folder) }

    private func mode(_ url: URL) -> Int {
        ((try? FileManager.default.attributesOfItem(atPath: url.path))?[.posixPermissions] as? Int ?? -1) & 0o777
    }

    func testGMTokenRoundTripsThroughItsFileAndNeverAsksTheKeychain() {
        XCTAssertEqual(token.utf8.count, 43)
        let store = GMIdentity.defaultStore(testCopy: true, support: folder, keychain: {
            XCTFail("a test copy asked for the Keychain store"); return NoKeychainToken(fail: { XCTFail("Keychain: \($0)") })
        })
        XCTAssertEqual(store.read(), GMTokenRead.none)
        XCTAssertTrue(store.write(token))
        XCTAssertEqual(store.read(), .token(token))
        let file = folder.appendingPathComponent(TestCopySecrets.gmTokenName)
        XCTAssertEqual(mode(file), 0o600)
        XCTAssertEqual(mode(folder), 0o700)
        store.remove()
        XCTAssertEqual(store.read(), GMTokenRead.none)
        XCTAssertFalse(FileManager.default.fileExists(atPath: file.path))
    }

    func testGMIdentityInATestCopyMakesAndKeepsItsTokenInTheFile() {
        let store = GMIdentity.defaultStore(testCopy: true, support: folder, keychain: {
            XCTFail("a test copy asked for the Keychain store"); return NoKeychainToken(fail: { XCTFail("Keychain: \($0)") })
        })
        let made = GMIdentity(store: store, tokenFile: folder.appendingPathComponent("gm-token"), fileAllowed: { false }).token()
        XCTAssertNotNil(made)
        // A new run of the copy reads the same token back from the file.
        let again = GMIdentity(store: store, tokenFile: folder.appendingPathComponent("gm-token"), fileAllowed: { false })
        XCTAssertTrue(again.isToken(made))
    }

    func testTheRealAppStillUsesTheKeychainStore() {
        var asked = false
        _ = GMIdentity.defaultStore(testCopy: false, support: folder, underTest: false, keychain: { asked = true; return NoKeychainToken(fail: { _ in }) })
        XCTAssertTrue(asked)
    }

    func testAPIKeyRoundTripsThroughItsFileAndNeverAsksTheKeychain() {
        let fail = { (what: String) in XCTFail("Keychain: \(what)") }
        let keychain = SummaryKey.Store(
            read: { fail("read"); return nil }, hint: { fail("hint"); return nil },
            write: { _, _ in fail("write"); return false }, delete: { fail("delete") })
        let store = SummaryKey.defaultStore(testCopy: true, support: folder, keychain: keychain)
        XCTAssertNil(store.read())
        XCTAssertNil(store.hint())
        XCTAssertTrue(store.write(key, SummaryKey.hint(key)))
        XCTAssertEqual(store.read(), key)
        XCTAssertEqual(store.hint(), SummaryKey.hint(key))
        let file = folder.appendingPathComponent(TestCopySecrets.apiKeyName)
        XCTAssertEqual(mode(file), 0o600)
        XCTAssertEqual(mode(folder), 0o700)
        store.delete()
        XCTAssertNil(store.read())
    }

    func testAKeyAPersonPutsInTheFileByHandIsRead() throws {
        let file = folder.appendingPathComponent(TestCopySecrets.apiKeyName)
        try Data("  \(key)\n".utf8).write(to: file)
        let store = SummaryKey.Store.file(file)
        XCTAssertEqual(store.read(), key)
        XCTAssertEqual(store.hint(), SummaryKey.hint(key))
        // Something that isn't a key is no key.
        try Data("not a key\n".utf8).write(to: file)
        XCTAssertNil(store.read())
        XCTAssertNil(store.hint())
    }

    func testTheRealAppKeepsTheKeychainStoreForTheKey() {
        var reads = 0
        let keychain = SummaryKey.Store(read: { reads += 1; return nil }, hint: { nil }, write: { _, _ in false }, delete: {})
        _ = SummaryKey.defaultStore(testCopy: false, support: folder, keychain: keychain).read()
        XCTAssertEqual(reads, 1)
    }

    func testAFileAndFolderLeftTooOpenAreTightenedBeforeTheyAreRead() throws {
        let file = folder.appendingPathComponent(TestCopySecrets.apiKeyName)
        try Data("\(key)\n".utf8).write(to: file)
        try FileManager.default.setAttributes([.posixPermissions: 0o644], ofItemAtPath: file.path)
        XCTAssertEqual(mode(file), 0o644)
        XCTAssertEqual(mode(folder), 0o755)
        XCTAssertEqual(TestCopySecrets.read(file), key)
        XCTAssertEqual(mode(file), 0o600)
        XCTAssertEqual(mode(folder), 0o700)
    }

    func testAHireOnTheAPIKeyAccountInATestCopyReadsTheKeyFileAndNeverTheKeychain() {
        let file = folder.appendingPathComponent(TestCopySecrets.apiKeyName)
        let lines = HireDesk.scriptLines(ports: "", apiKey: true, copyPort: 7790, copySupport: folder, keyFile: file)
        XCTAssertFalse(lines.contains("security"))
        XCTAssertTrue(lines.contains("cat '\(file.path)'"))
        XCTAssertTrue(lines.contains("exit 1"))
        XCTAssertFalse(lines.contains(key))
        // The real app's hire still reads the Keychain.
        XCTAssertTrue(HireDesk.scriptLines(ports: "", apiKey: true, copyPort: nil, copySupport: nil, keyFile: nil).contains("security find-generic-password"))
    }

    func testTheViewerInATestCopyKeepsItsSecretsInFilesAndNeverAsksTheKeychain() {
        let fail = { (what: String) in XCTFail("Keychain: \(what)") }
        let keychain = ViewerStore(read: { _ in fail("read"); return nil }, absent: { _ in fail("absent"); return false },
                                   write: { _, _ in fail("write"); return false })
        let store = ViewerStore.defaultStore(testCopy: true, support: folder, keychain: keychain)
        XCTAssertTrue(store.absent(.writeSecret))
        XCTAssertNil(store.read(.writeSecret))
        XCTAssertTrue(store.write("secret-1", .writeSecret))
        XCTAssertFalse(store.absent(.writeSecret))
        XCTAssertEqual(store.read(.writeSecret), "secret-1")
        XCTAssertTrue(store.absent(.id))
        XCTAssertEqual(mode(folder.appendingPathComponent("\(TestCopySecrets.viewerName)-write-secret")), 0o600)
        XCTAssertEqual(mode(folder), 0o700)
        // The real app keeps the Keychain.
        var asked = false
        let real = ViewerStore(read: { _ in nil }, absent: { _ in asked = true; return true }, write: { _, _ in false })
        _ = ViewerStore.defaultStore(testCopy: false, support: folder, keychain: real).absent(.id)
        XCTAssertTrue(asked)
    }

    func testAPushKeyIsNeverLookedForInATestCopy() {
        XCTAssertNil(PushKey.account(testCopy: true))
        XCTAssertNil(PushKey.token(keyID: "TESTKEY123", now: Date(), testCopy: true))
    }
}
