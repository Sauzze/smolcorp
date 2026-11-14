import Foundation
import Security

/// The record key (SANDBOX-PLAN §3.2, SK1): 32 random bytes the Go core seals Smolcorp's records with (phones.json,
/// mac-peers.json, the registry's permissions, config.json's security settings, the guard: desktop/records.go). It
/// is one Keychain item of this app's, in the login keychain opened by its path (`Keychain`), which only this app
/// reads and hands to the core over the bridge. So a program running as you can write those files, but not seal
/// them.
///
/// Any program of yours can overwrite the item's value, or add a second item under its name that trusts this app
/// (probes K7, K11). Neither can be undone without a trace: an overwrite moves the item's modification date off its
/// creation date, and a planted item is a second item. So the item is added once and never changed, and `look`
/// says which of these it finds. To rotate, the core asks `add` (a second item, under a new account), seals every
/// record under it, then asks `retire` for the old one: add, seal, delete. A stop between the two leaves two items,
/// which the core takes for an interrupted update.
///
/// An overwrite in the very second of the add leaves the dates equal (RT126SK1 B1), so every add is read back once
/// that second is over (`Keychain.add`), and its key handed on only if it holds the bytes just made. If it doesn't,
/// the item is left there with one more beside it (`witness`): the key's name then never holds a single item whose
/// value another program chose, and the next start finds two, which counts for nothing until you reset.
struct RecordKeyItem: Equatable {
    let id: String
    let value: Data

    var json: [String: Any] { ["id": id, "value": value.map { String(format: "%02x", $0) }.joined()] }
}

/// What `look` found: one item that checks out, none, more than one, one whose value was overwritten, or items it
/// couldn't read without a prompt.
enum RecordKeyLook: Equatable {
    case one(RecordKeyItem)
    case none
    case many([RecordKeyItem])
    case moved
    case unreadable
}

protocol RecordKeyStore {
    func look() -> RecordKeyLook
    /// The items' key made new where it must be (`ItemSeal.renew`), with the record key's: at the update to 1.26
    /// (`update`) or a reset.
    func renewItems(update: Bool)
    /// Every item under the key's name deleted, a planted one with them, and one new item made.
    func makeNew() -> RecordKeyItem?
    /// A new item beside the one there: the first step of a rotation.
    func add() -> RecordKeyItem?
    /// The item `id` deleted: a rotation's last step.
    func retire(_ id: String) -> Bool
}

extension RecordKeyStore {
    /// A store whose items aren't the Keychain's (a test copy's file) has no items' key.
    func renewItems(update: Bool) {}
}

/// 32 random bytes, and a fresh account for them.
private func freshRecordKey() -> RecordKeyItem? {
    var bytes = [UInt8](repeating: 0, count: 32), name = [UInt8](repeating: 0, count: 8)
    guard SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes) == errSecSuccess,
          SecRandomCopyBytes(kSecRandomDefault, name.count, &name) == errSecSuccess else { return nil }
    return RecordKeyItem(id: "key-" + name.map { String(format: "%02x", $0) }.joined(), value: Data(bytes))
}

/// The record key in the Keychain. `keychain` and `open` are the tests' throwaway keychain file (see `Keychain`).
struct RecordKeychain: RecordKeyStore {
    static let service = SummaryKey.testMode ? "smolcorp-records-test" : "smolcorp-records"
    var service = RecordKeychain.service
    var keychain: SecKeychain? = nil
    var open: Bool? = nil

    private var match: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service]
    }

    func look() -> RecordKeyLook {
        var query = match
        query[kSecReturnAttributes as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitAll
        var found: CFTypeRef?
        switch Keychain.copyMatching(query, &found, in: keychain, open: open) {
        case errSecSuccess: break
        case errSecItemNotFound: return .none
        default: return .unreadable
        }
        guard let rows = found as? [[String: Any]], !rows.isEmpty else { return .none }
        if rows.contains(where: { !Keychain.sameDates($0) }) { return .moved }
        var items: [RecordKeyItem] = []
        for row in rows {
            guard let id = row[kSecAttrAccount as String] as? String, let value = read(id), value.count == 32 else { return .unreadable }
            items.append(RecordKeyItem(id: id, value: value))
        }
        return items.count == 1 ? .one(items[0]) : .many(items)
    }

    private func read(_ id: String) -> Data? {
        var query = match
        query[kSecAttrAccount as String] = id
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var found: CFTypeRef?
        guard Keychain.copyMatching(query, &found, in: keychain, open: open, root: true) == errSecSuccess else { return nil }
        return found as? Data
    }

    func makeNew() -> RecordKeyItem? {
        // A keychain file deletes every match at once; a few more tries for anything left.
        var tries = 0
        while tries < 8, Keychain.delete(match, in: keychain, open: open) == errSecSuccess { tries += 1 }
        var none: CFTypeRef?
        guard Keychain.copyMatching(match, &none, in: keychain, open: open) == errSecItemNotFound else { return nil }
        return add()
    }

    private func entry(_ item: RecordKeyItem) -> [String: Any] {
        var entry = match
        entry[kSecAttrAccount as String] = item.id
        entry[kSecAttrLabel as String] = "Smolcorp records key"
        entry[kSecValueData as String] = item.value
        return entry
    }

    func add() -> RecordKeyItem? {
        guard let item = freshRecordKey() else { return nil }
        let status = Keychain.add(entry(item), in: keychain, open: open, root: true)
        if status == Keychain.overwritten, let witness = freshRecordKey() {
            // Another program changed it as it was added: left there, with one more beside it (see above).
            _ = Keychain.add(entry(witness), in: keychain, open: open, root: true)
        }
        return status == errSecSuccess ? item : nil
    }

    func renewItems(update: Bool) { ItemSeal.renew(update: update, in: keychain, open: open) }

    func retire(_ id: String) -> Bool {
        var entry = match
        entry[kSecAttrAccount as String] = id
        let status = Keychain.delete(entry, in: keychain, open: open)
        return status == errSecSuccess || status == errSecItemNotFound
    }
}

/// The record key as a test copy and the test suite keep it: a file in their own support folder, the Keychain never
/// asked (`TestCopySecrets`). Nothing there is the owner's.
struct RecordKeyFile: RecordKeyStore {
    let file: URL

    private func items() -> [RecordKeyItem] {
        guard let text = TestCopySecrets.read(file), let rows = (try? JSONSerialization.jsonObject(with: Data(text.utf8))) as? [[String: String]] else {
            return []
        }
        return rows.compactMap { row in
            guard let id = row["id"], let hex = row["value"], hex.count == 64 else { return nil }
            var bytes: [UInt8] = []
            var at = hex.startIndex
            while at < hex.endIndex, let byte = UInt8(hex[at..<hex.index(at, offsetBy: 2)], radix: 16) {
                bytes.append(byte)
                at = hex.index(at, offsetBy: 2)
            }
            return bytes.count == 32 ? RecordKeyItem(id: id, value: Data(bytes)) : nil
        }
    }

    private func keep(_ items: [RecordKeyItem]) -> Bool {
        guard let data = try? JSONSerialization.data(withJSONObject: items.map(\.json)), let text = String(data: data, encoding: .utf8) else { return false }
        return TestCopySecrets.write(text, to: file)
    }

    func look() -> RecordKeyLook {
        let all = items()
        return all.isEmpty ? .none : all.count == 1 ? .one(all[0]) : .many(all)
    }

    func makeNew() -> RecordKeyItem? {
        guard let item = freshRecordKey(), keep([item]) else { return nil }
        return item
    }

    func add() -> RecordKeyItem? {
        guard let item = freshRecordKey(), keep(items() + [item]) else { return nil }
        return item
    }

    func retire(_ id: String) -> Bool { keep(items().filter { $0.id != id }) }
}

enum RecordKeys {
    /// The test copy's file name for the record key.
    static let fileName = "test-record-key"

    /// Where the record key is kept: the Keychain, or a file in a test copy's support folder and the test suite's.
    static func defaultStore(testCopy: Bool = TestCopy.isOn, support: URL? = TestCopy.support, underTest: Bool = Paths.underTest) -> RecordKeyStore {
        if testCopy, let support { return RecordKeyFile(file: support.appendingPathComponent(fileName)) }
        if underTest { return RecordKeyFile(file: Paths.testSupport.appendingPathComponent(fileName)) }
        return RecordKeychain()
    }
}
