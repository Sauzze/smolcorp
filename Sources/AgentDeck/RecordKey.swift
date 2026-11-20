import CryptoKit
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
    /// The update to 1.26 marked under way for `key`, the item just made for it (`RecordKeyUpdate`).
    func markUpdate(for key: RecordKeyItem)
    /// Whether the update's mark for `key` is there, as `markUpdate` left it.
    func updateUnderWay(for key: RecordKeyItem) -> Bool
    /// The update's mark for the key `id` deleted, by its account alone: the core's start sealed the records. Whether
    /// no item under that account is left. Another item under the mark's name (K11), one the app can't delete among
    /// them, neither stops it nor is touched (RT126UPGRADESTART N1).
    func settleUpdate(for id: String) -> Bool
}

extension RecordKeyStore {
    /// A store whose items aren't the Keychain's (a test copy's file) has no items' key.
    func renewItems(update: Bool) {}
    /// A store with no update to mark (the shared fixture's stand-ins).
    func markUpdate(for key: RecordKeyItem) {}
    func updateUnderWay(for key: RecordKeyItem) -> Bool { false }
    func settleUpdate(for id: String) -> Bool { true }
}

/// The update to 1.26 under way (FXUPGRADESTART126R1): the record key made for it, and the core's start not over yet.
/// A start stopped between the two (a Keychain prompt left open, the app quit) leaves records the core hadn't sealed
/// yet, and the next start finishes the update rather than drop them (desktop/records.go's startRecords).
///
/// The mark is an item of its own, beside the record key's: its account is the key's, its value an HMAC of the key's
/// id under the key. A program running as you can add an item under that name (K11), but can't read the key, so it
/// can't make one that fits; it can't delete the app's (K9), nor change its value unseen (K7, K10). The core has it
/// deleted, by the key's account, once its start has sealed the records (`record-key-settled`). A new key (a reset, a
/// rotation) leaves an old mark fitting nothing, and any other item under the name makes the mark count for nothing.
enum RecordKeyUpdate {
    static func mark(for key: RecordKeyItem) -> RecordKeyItem {
        let message = Data("smolcorp-records-update-v1\u{0}\(key.id)".utf8)
        return RecordKeyItem(id: key.id, value: Data(HMAC<SHA256>.authenticationCode(for: message, using: SymmetricKey(data: key.value))))
    }
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
    var label = "Smolcorp records key"

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
        guard clear() else { return nil }
        return add()
    }

    /// Every item under the name deleted, a planted one with them, or only the one under `account`: whether none is left.
    func clear(account: String? = nil) -> Bool {
        var items = match
        if let account { items[kSecAttrAccount as String] = account }
        // A keychain file deletes every match at once; a few more tries for anything left.
        var tries = 0
        while tries < 8, Keychain.delete(items, in: keychain, open: open) == errSecSuccess { tries += 1 }
        var none: CFTypeRef?
        return Keychain.copyMatching(items, &none, in: keychain, open: open) == errSecItemNotFound
    }

    private func entry(_ item: RecordKeyItem) -> [String: Any] {
        var entry = match
        entry[kSecAttrAccount as String] = item.id
        entry[kSecAttrLabel as String] = label
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

    /// The update's mark (`RecordKeyUpdate`), kept as the key is: one item, added once and read back, never changed.
    private var updates: RecordKeychain {
        RecordKeychain(service: service + "-update", keychain: keychain, open: open, label: "Smolcorp records update")
    }

    func markUpdate(for key: RecordKeyItem) {
        let mark = RecordKeyUpdate.mark(for: key)
        _ = Keychain.add(updates.entry(mark), in: keychain, open: open, root: true)
    }

    func updateUnderWay(for key: RecordKeyItem) -> Bool { updates.look() == .one(RecordKeyUpdate.mark(for: key)) }

    func settleUpdate(for id: String) -> Bool { !id.isEmpty && updates.clear(account: id) }

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

    /// The update's mark, in a file beside the key's.
    private var updates: RecordKeyFile { RecordKeyFile(file: URL(fileURLWithPath: file.path + "-update")) }

    func markUpdate(for key: RecordKeyItem) { _ = updates.keep([RecordKeyUpdate.mark(for: key)]) }

    func updateUnderWay(for key: RecordKeyItem) -> Bool { updates.look() == .one(RecordKeyUpdate.mark(for: key)) }

    func settleUpdate(for id: String) -> Bool { !id.isEmpty && updates.keep(updates.items().filter { $0.id != id }) }
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
