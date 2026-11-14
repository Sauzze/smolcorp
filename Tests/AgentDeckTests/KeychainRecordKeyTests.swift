import XCTest
import Security
@testable import AgentDeck

/// SK1 (SANDBOX-PLAN §3.2), the app's half: the record key, one item added once and never changed, in the login
/// keychain opened by its path. Every row runs on throwaway keychain files of this test's own, made with a password
/// it knows and never put in the search list or made the default; every read is made with the user's interaction
/// off. Nothing here asks the owner's login keychain, or opens the gate for the rest of the app.
///
/// "Another program" is played by this test with SecItem's own calls on the same file: probes K7–K12 showed what
/// another program can and can't do to the app's item (overwrite its value, plant a second item; not set a date
/// back, not read it). These rows check that the app sees each of those.
/// A string shaped as an Anthropic API key, obviously fake, ending as given. Its prefix is put together only when the
/// tests run, so the public check (scripts/check-public.sh) never takes this file for a real key.
private func fakeAnthropicKey(_ ending: String) -> String {
    "sk-" + "ant-" + ending
}

final class KeychainRecordKeyTests: XCTestCase {
    private var folder: URL!
    private var made: [SecKeychain] = []
    private var searchListBefore: [String] = []

    /// The user's keychain search list, by path: read only, to check it never changes.
    private func searchList() -> [String] {
        var list: CFArray?
        guard SecKeychainCopyDomainSearchList(.user, &list) == errSecSuccess, let keychains = list as? [SecKeychain] else { return [] }
        return keychains.compactMap { keychain in
            var length: UInt32 = 1024
            var path = [CChar](repeating: 0, count: 1024)
            return SecKeychainGetPath(keychain, &length, &path) == errSecSuccess ? String(cString: path) : nil
        }
    }

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("sk1-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        searchListBefore = searchList()
    }

    override func tearDownWithError() throws {
        Keychain.afterAdd = nil
        for keychain in made { SecKeychainDelete(keychain) }
        made = []
        try? FileManager.default.removeItem(at: folder)
        XCTAssertEqual(searchList(), searchListBefore, "the user's keychain search list changed")
    }

    /// A throwaway keychain file, unlocked with a password this test knows, never in the search list.
    private func throwaway(_ name: String) throws -> SecKeychain {
        let password = "FAKE-throwaway-keychain"
        var keychain: SecKeychain?
        let status = SecKeychainCreate(folder.appendingPathComponent("\(name).keychain-db").path, UInt32(password.utf8.count), password, false, nil, &keychain)
        let made = try XCTUnwrap(keychain, "SecKeychainCreate: \(status)")
        self.made.append(made)
        XCTAssertFalse(searchList().contains { $0.hasPrefix(folder.path) }, "a throwaway keychain went in the search list")
        return made
    }

    private func store(_ keychain: SecKeychain) -> RecordKeychain {
        RecordKeychain(service: "smolcorp-records-test", keychain: keychain, open: true)
    }

    /// Another program's item under the key's name, with a value of its own (K11).
    private func plant(_ id: String, in keychain: SecKeychain) -> OSStatus {
        SecItemAdd([kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: "smolcorp-records-test",
                    kSecAttrAccount as String: id, kSecValueData as String: Data(repeating: 0x41, count: 32),
                    kSecUseKeychain as String: keychain] as CFDictionary, nil)
    }

    func testTheRecordKeyIsOneItemReadWithoutAPromptAndRotatedAddSealDelete() throws {
        let keychain = try throwaway("one")
        let records = store(keychain)
        XCTAssertEqual(records.look(), .none)
        let first = try XCTUnwrap(records.makeNew())
        XCTAssertEqual(first.value.count, 32)
        XCTAssertEqual(records.look(), .one(first))
        // A rotation: the new item beside the old (two items: what a stop here leaves), then the old one deleted.
        let second = try XCTUnwrap(records.add())
        XCTAssertNotEqual(second.id, first.id)
        XCTAssertNotEqual(second.value, first.value)
        guard case .many(let both) = records.look() else { return XCTFail("after the add: \(records.look())") }
        XCTAssertEqual(Set(both.map(\.id)), [first.id, second.id])
        XCTAssertTrue(records.retire(first.id))
        XCTAssertEqual(records.look(), .one(second))
    }

    /// K7, K10: any program can put a value of its own in the app's item, with no prompt, but the item's modification
    /// date then moves off its creation date, and nobody can set it back. The app takes that item for overwritten:
    /// the record key's look says so, and no Smolcorp item is read once overwritten.
    func testAnOverwrittenItemIsNeverRead() throws {
        let keychain = try throwaway("moved")
        let records = store(keychain)
        let item = try XCTUnwrap(records.makeNew())
        Thread.sleep(forTimeInterval: 1.2) // the dates are kept to the second
        let match: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: "smolcorp-records-test",
                                    kSecAttrAccount as String: item.id, kSecMatchSearchList as String: [keychain]]
        XCTAssertEqual(SecItemUpdate(match as CFDictionary, [kSecValueData as String: Data(repeating: 0x42, count: 32)] as CFDictionary), errSecSuccess)
        XCTAssertEqual(records.look(), .moved)
        var read = match
        read.removeValue(forKey: kSecMatchSearchList as String)
        read[kSecReturnData as String] = true
        var found: CFTypeRef?
        XCTAssertEqual(Keychain.copyMatching(read, &found, in: keychain, open: true), Keychain.overwritten)
        XCTAssertNil(found, "the overwritten value was read")
        // The owner's reset deletes it, and makes one that checks out.
        let fresh = try XCTUnwrap(records.makeNew())
        XCTAssertEqual(records.look(), .one(fresh))
    }

    /// K11, K12: a second item planted under the key's name is seen (never taken for the key), and a new key deletes
    /// every item under the name, the planted one with them.
    func testAPlantedItemIsASecondItemAndANewKeyDeletesIt() throws {
        let keychain = try throwaway("planted")
        let records = store(keychain)
        let mine = try XCTUnwrap(records.makeNew())
        XCTAssertEqual(plant("key-planted", in: keychain), errSecSuccess)
        guard case .many(let items) = records.look() else { return XCTFail("with a planted item: \(records.look())") }
        XCTAssertTrue(items.contains(mine))
        // Before 1.26's first start, an item planted beforehand is all there is: makeNew deletes it.
        let fresh = try XCTUnwrap(records.makeNew())
        XCTAssertEqual(records.look(), .one(fresh))
        XCTAssertNotEqual(fresh.id, "key-planted")
    }

    /// Waits for the start of a second, so what a row does next happens within it.
    private func startOfASecond() {
        let now = Date().timeIntervalSince1970
        Thread.sleep(forTimeInterval: now.rounded(.down) + 1.01 - now)
    }

    /// Another program's value put in the item `name` names, as it polls for the item and finds it (K7).
    private func overwrite(_ name: [String: Any], in keychain: SecKeychain, with byte: UInt8) -> OSStatus {
        var match = name
        match[kSecMatchSearchList as String] = [keychain]
        return SecItemUpdate(match as CFDictionary, [kSecValueData as String: Data(repeating: byte, count: 32)] as CFDictionary)
    }

    /// RT126SK1 B1: another program overwrites a new item in the very second the app added it, which leaves its dates
    /// equal. The app reads every add back once that second is over: the record key is never handed on, and the key's
    /// name is left with two items, so the next start counts nothing; any other item of the app's is deleted.
    func testAnOverwriteInTheSecondOfTheAddIsCaught() throws {
        let keychain = try throwaway("same-second")
        let records = store(keychain)
        var overwritten: [String: Any]?
        var target = "smolcorp-records-test"
        Keychain.afterAdd = { name in
            guard overwritten == nil, name[kSecAttrService as String] as? String == target else { return }
            overwritten = name
            XCTAssertEqual(self.overwrite(name, in: keychain, with: 0x43), errSecSuccess)
        }
        startOfASecond()
        XCTAssertNil(records.makeNew(), "a record key another program overwrote as it was added was handed on")
        let name = try XCTUnwrap(overwritten)
        var attributes = name
        attributes[kSecReturnAttributes as String] = true
        var found: CFTypeRef?
        XCTAssertEqual(Keychain.copyMatching(attributes, &found, in: keychain, open: true), errSecSuccess)
        XCTAssertTrue(Keychain.sameDates(try XCTUnwrap(found as? [String: Any])), "the overwrite moved the dates: the row didn't play B1")
        guard case .many(let items) = records.look() else { return XCTFail("after the overwritten add: \(records.look())") }
        XCTAssertEqual(items.count, 2)
        // Any other item of the app's: refused, and taken away.
        XCTAssertNotNil(ItemSeal.key(in: keychain, open: true, making: true))
        overwritten = nil
        target = "smolcorp-gm-test"
        let item: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: "smolcorp-gm-test",
                                   kSecAttrAccount as String: "gm-token", kSecValueData as String: Data("v1:FAKE".utf8)]
        startOfASecond()
        XCTAssertEqual(Keychain.add(item, in: keychain, open: true), Keychain.overwritten)
        var gone: CFTypeRef?
        XCTAssertEqual(Keychain.copyMatching(attributes.merging([kSecAttrService as String: "smolcorp-gm-test",
                                                                 kSecAttrAccount as String: "gm-token"]) { $1 }, &gone, in: keychain, open: true),
                       errSecItemNotFound)
        // Without another program, the add holds.
        Keychain.afterAdd = nil
        let fresh = try XCTUnwrap(records.makeNew())
        XCTAssertEqual(records.look(), .one(fresh))
    }

    /// RT126SK1 B2: an item another program adds under a free name of the app's (the GM's, the API key's), K11's way,
    /// carries no seal of the app's, so it's never read; nor is one whose seal it copied from another item, nor one
    /// sealed under an items' key it planted. The app's own add reads.
    func testAnItemTheAppDidntAddIsNeverRead() throws {
        let keychain = try throwaway("items")
        let gm: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: "smolcorp-gm-test",
                                 kSecAttrAccount as String: "gm-token"]
        let api: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: "smolcorp-summary-test",
                                  kSecAttrAccount as String: "api-key"]
        func read(_ name: [String: Any]) -> (OSStatus, String?) {
            var query = name
            query[kSecReturnData as String] = true
            var found: CFTypeRef?
            let status = Keychain.copyMatching(query, &found, in: keychain, open: true)
            return (status, (found as? Data).flatMap { String(data: $0, encoding: .utf8) })
        }
        XCTAssertEqual(Keychain.add(api.merging([kSecValueData as String: Data(fakeAnthropicKey("FAKE-mine").utf8)]) { $1 }, in: keychain, open: true), errSecSuccess)
        XCTAssertEqual(read(api).1, fakeAnthropicKey("FAKE-mine"))
        // Planted with no seal, then with the seal of the app's own item beside it.
        XCTAssertEqual(SecItemAdd(gm.merging([kSecValueData as String: Data("v1:FAKE-planted".utf8), kSecUseKeychain as String: keychain]) { $1 } as CFDictionary, nil), errSecSuccess)
        XCTAssertEqual(read(gm).0, Keychain.overwritten)
        var attributes = api
        attributes[kSecReturnAttributes as String] = true
        var found: CFTypeRef?
        XCTAssertEqual(Keychain.copyMatching(attributes, &found, in: keychain, open: true), errSecSuccess)
        let copied = try XCTUnwrap((found as? [String: Any])?[kSecAttrGeneric as String] as? Data)
        var name = gm
        name[kSecMatchSearchList as String] = [keychain]
        XCTAssertEqual(SecItemUpdate(name as CFDictionary, [kSecAttrGeneric as String: copied] as CFDictionary), errSecSuccess)
        XCTAssertEqual(read(gm).0, Keychain.overwritten)
        XCTAssertNil(read(gm).1)
        // A second items' key planted beside the app's: no item counts until a reset makes the key new.
        XCTAssertEqual(SecItemAdd([kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: ItemSeal.service,
                                   kSecAttrAccount as String: "key-planted", kSecValueData as String: Data(repeating: 0x41, count: 32),
                                   kSecUseKeychain as String: keychain] as CFDictionary, nil), errSecSuccess)
        XCTAssertEqual(read(api).0, Keychain.overwritten)
        ItemSeal.renew(update: false, in: keychain, open: true)
        XCTAssertEqual(read(api).0, Keychain.overwritten, "an item sealed under the key before the reset still read")
        XCTAssertEqual(Keychain.add(gm.merging([kSecValueData as String: Data("v1:FAKE-mine".utf8)]) { $1 }, in: keychain, open: true),
                       errSecDuplicateItem, "the planted GM item is the app's to delete before it writes")
        Keychain.delete(gm, in: keychain, open: true)
        XCTAssertEqual(Keychain.add(gm.merging([kSecValueData as String: Data("v1:FAKE-mine".utf8)]) { $1 }, in: keychain, open: true), errSecSuccess)
        XCTAssertEqual(read(gm).1, "v1:FAKE-mine")
    }

    /// The update to 1.26 seals the items from before as found, as the records are; a reset whose items' key checks
    /// out keeps it, and every item with it.
    func testTheUpdateSealsTheItemsFromBeforeAsFoundAndAResetKeepsThem() throws {
        let keychain = try throwaway("update")
        let api: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: "smolcorp-summary-test",
                                  kSecAttrAccount as String: "api-key"]
        XCTAssertEqual(SecItemAdd(api.merging([kSecValueData as String: Data(fakeAnthropicKey("FAKE-125").utf8), kSecAttrComment as String: fakeAnthropicKey("…125"),
                                               kSecUseKeychain as String: keychain]) { $1 } as CFDictionary, nil), errSecSuccess)
        var query = api
        query[kSecReturnData as String] = true
        var found: CFTypeRef?
        XCTAssertEqual(Keychain.copyMatching(query, &found, in: keychain, open: true), Keychain.overwritten)
        ItemSeal.renew(update: true, adopting: [api], in: keychain, open: true)
        found = nil
        XCTAssertEqual(Keychain.copyMatching(query, &found, in: keychain, open: true), errSecSuccess)
        XCTAssertEqual((found as? Data).flatMap { String(data: $0, encoding: .utf8) }, fakeAnthropicKey("FAKE-125"))
        var attributes = api
        attributes[kSecReturnAttributes as String] = true
        var about: CFTypeRef?
        _ = Keychain.copyMatching(attributes, &about, in: keychain, open: true)
        XCTAssertEqual((about as? [String: Any])?[kSecAttrComment as String] as? String, fakeAnthropicKey("…125"))
        ItemSeal.renew(update: false, in: keychain, open: true)
        found = nil
        XCTAssertEqual(Keychain.copyMatching(query, &found, in: keychain, open: true), errSecSuccess, "a reset dropped an item it could keep")
    }

    /// RSBXPLANf F1: the app asks one keychain, by its path, never the search list or the default keychain. A key
    /// planted in another keychain file (one a program made, whose password it knows, and might have put first in
    /// the search list) is never found, and a new key never goes there. The plan's row sets that keychain as the search
    /// list and the default; both are settings of the whole user, never of one program, so this row plays it with two
    /// pinned files instead and leaves the owner's settings alone (RT126SK1 nit 8).
    func testOnlyThePinnedKeychainIsAsked() throws {
        let pinned = try throwaway("pinned"), other = try throwaway("other")
        XCTAssertEqual(plant("key-elsewhere", in: other), errSecSuccess)
        XCTAssertEqual(store(pinned).look(), .none, "an item in another keychain was found")
        let item = try XCTUnwrap(store(pinned).makeNew())
        XCTAssertEqual(store(pinned).look(), .one(item))
        guard case .one(let elsewhere) = store(other).look() else { return XCTFail("the other keychain: \(store(other).look())") }
        XCTAssertEqual(elsewhere.id, "key-elsewhere", "the new key went in another keychain, or the planted one went")
    }

    /// The login keychain is found by the system's home folder for this user, never by $HOME, which whoever starts
    /// the app sets.
    func testTheLoginKeychainIsFoundByItsFixedPath() throws {
        let saved = ProcessInfo.processInfo.environment["HOME"]
        setenv("HOME", folder.path, 1)
        defer { if let saved { setenv("HOME", saved, 1) } else { unsetenv("HOME") } }
        let path = Keychain.loginPath()
        XCTAssertTrue(path.hasSuffix("/Library/Keychains/login.keychain-db"))
        XCTAssertFalse(path.hasPrefix(folder.path), "the login keychain was looked for under $HOME")
    }

    /// The tests' own switch never reaches the owner's login keychain: without a keychain of their own, a call while
    /// the gate is shut asks nothing, whatever `open` says.
    func testWithoutAKeychainOfItsOwnATestReachesNoKeychain() {
        XCTAssertFalse(KeychainGate.isOpen)
        var found: CFTypeRef?
        let query: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: "smolcorp-records-test",
                                    kSecReturnAttributes as String: true]
        XCTAssertEqual(Keychain.copyMatching(query, &found, open: true), errSecNotAvailable)
        XCTAssertEqual(Keychain.add(query, open: true), errSecNotAvailable)
        XCTAssertEqual(Keychain.delete(query, open: true), errSecNotAvailable)
        XCTAssertEqual(RecordKeychain(service: "smolcorp-records-test", open: true).look(), .unreadable)
    }

    /// A test copy and the test suite keep the record key in a file of their own; the Keychain is never asked.
    func testATestCopyKeepsItsRecordKeyInAFile() throws {
        let support = folder.appendingPathComponent("copy", isDirectory: true)
        let store = RecordKeys.defaultStore(testCopy: true, support: support, underTest: false)
        XCTAssertTrue(store is RecordKeyFile)
        let item = try XCTUnwrap(store.makeNew())
        XCTAssertEqual(store.look(), .one(item))
        let next = try XCTUnwrap(store.add())
        XCTAssertTrue(store.retire(item.id))
        XCTAssertEqual(store.look(), .one(next))
        XCTAssertTrue(RecordKeys.defaultStore(testCopy: false, support: nil, underTest: true) is RecordKeyFile)
    }
}
