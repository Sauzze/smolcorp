import CryptoKit
import Foundation
import Security

/// Smolcorp's Keychain items (the API key, the GM's token, the portable office's) are read by this app alone, and
/// only once it holds its support folder's instance lock (`InstanceLock`). A second Smolcorp.app started on the same
/// folder while yours runs, by a hire say, never takes the lock, so it asks the Keychain nothing. The `smolcorp`
/// program never asks the Keychain at all: the core gets what it needs from this app, over the bridge.
///
/// Every SecItem call in the app goes through `Keychain` below (HostBridgeHardeningTests checks no other file makes
/// one). Until the gate opens, each answers `errSecNotAvailable` without asking: a failed read, never "not found", so
/// nothing takes it for "none" and writes a new item over the real one.
///
/// Every one of them asks the login keychain alone, opened by its fixed path (SANDBOX-PLAN §3.2, RSBXPLANf F1): any
/// program of yours can change your default keychain and your keychains' search list without a prompt, to one whose
/// password it knows. Smolcorp never asks either, so its items never land in, or are read from, a keychain like that.
/// A read never prompts, and an item another program added or overwrote isn't read at all (`Keychain`, `ItemSeal`).
enum KeychainGate {
    private static let lock = NSLock()
    private static var opened = false

    static var isOpen: Bool {
        lock.lock()
        defer { lock.unlock() }
        return opened
    }

    /// Opened by the app once it holds `held`: never before, and never closed, as the lock is held until the app ends.
    static func open(holding held: InstanceLock) {
        guard held.isHeld else { return }
        lock.lock()
        opened = true
        lock.unlock()
        // What was looked for while the gate was shut found nothing: it's looked for again.
        SummaryKey.forgetKnown()
    }

    /// `call`'s answer when `open`; `errSecNotAvailable`, without calling it, when not.
    static func call(when open: Bool, _ call: () -> OSStatus) -> OSStatus {
        open ? call() : errSecNotAvailable
    }

    static func call(_ call: () -> OSStatus) -> OSStatus { self.call(when: isOpen, call) }
}

/// The Keychain as this app asks it: SecItem's calls, each through `KeychainGate`, each in the login keychain alone.
/// `keychain` and `open` are for the tests: a throwaway keychain file of their own, asked with the gate shut for the
/// rest of the app. Without `keychain`, `open` is never more than the gate.
///
/// Any program of yours can put a value of its own in an item of this app's, with no prompt, or add one under a name
/// of the app's that is free (probes K7, K11). So the app trusts an item only as it added it (RT126SK1 B1, B2):
/// - **An add is read back.** The dates are kept to the second, so an overwrite in the second of the add leaves them
///   equal. `add` waits until that second is over, then reads the item back, its attributes and value in one call:
///   it counts only if it is the only one under its name, holds the bytes just added and its dates are still equal.
///   Any later overwrite moves its modification date, which nobody can set back (K10).
/// - **Every item is sealed** (`ItemSeal`), but for the two keys at the root (the record key and the items' own key,
///   `root`): an HMAC of its name and value under a key only this app reads, beside it in the item. An item another
///   program added, or whose value it changed, has no seal that fits, and is never read.
enum Keychain {
    /// The login keychain's file, under the home folder the system has for this user: never `$HOME`, which whoever
    /// starts the app can set.
    static func loginPath() -> String {
        let home = getpwuid(getuid()).flatMap { String(validatingCString: $0.pointee.pw_dir) } ?? NSHomeDirectory()
        return home + "/Library/Keychains/login.keychain-db"
    }

    /// A keychain file opened by its path, or nil: opening it asks nothing.
    static func open(path: String) -> SecKeychain? {
        var keychain: SecKeychain?
        return SecKeychainOpen(path, &keychain) == errSecSuccess ? keychain : nil
    }

    /// The login keychain, opened the first time the gate lets a call through.
    private static let login = open(path: loginPath())

    /// What a read answers for an item another program overwrote, added or sealed: never `errSecItemNotFound`, so
    /// it's never taken for none and written over in silence. And what an add answers when the item it read back
    /// isn't the one it added.
    static let overwritten: OSStatus = errSecInvalidItemRef

    /// For the tests alone: run right after an add, before it's read back, as another program polling for the item
    /// would (the item's name). Nil in the app.
    static var afterAdd: (([String: Any]) -> Void)?

    /// `query` asked of `keychain` alone: added there, or looked for there only, and never with a prompt.
    static func pinned(_ query: [String: Any], in keychain: SecKeychain, adding: Bool = false) -> [String: Any] {
        var pinned = query
        if adding {
            pinned[kSecUseKeychain as String] = keychain
        } else {
            pinned[kSecMatchSearchList as String] = [keychain]
            pinned[kSecUseAuthenticationUI as String] = kSecUseAuthenticationUIFail
        }
        return pinned
    }

    /// Whether an item is as this app wrote it: it only ever adds an item, never changes one, and any program can
    /// put a value of its own in it, but not without moving its modification date off its creation date, which
    /// nobody can set (probes K7, K10), once the second of the add is over (`add`).
    static func sameDates(_ attributes: [String: Any]) -> Bool {
        guard let made = attributes[kSecAttrCreationDate as String] as? Date,
              let changed = attributes[kSecAttrModificationDate as String] as? Date else { return false }
        return made == changed
    }

    /// What names one item: its class, service and account, without what a query asks back.
    private static func name(_ query: [String: Any]) -> [String: Any] {
        query.filter { [kSecClass, kSecAttrService, kSecAttrAccount].map { $0 as String }.contains($0.key) }
    }

    /// The one item `query` names, its value read with its attributes in one call: only if it's the only one, its
    /// dates are equal, and (but for a root item) its seal fits. Its value then, or why not.
    private static func checked(_ query: [String: Any], in target: SecKeychain, keychain: SecKeychain?, open: Bool?,
                                root: Bool) -> (OSStatus, Data?) {
        var list = pinned(name(query), in: target)
        list[kSecReturnAttributes as String] = true
        list[kSecMatchLimit as String] = kSecMatchLimitAll
        var rows: CFTypeRef?
        let status = SecItemCopyMatching(list as CFDictionary, &rows)
        guard status == errSecSuccess else { return (status, nil) }
        guard (rows as? [[String: Any]])?.count == 1 else { return (errSecDuplicateItem, nil) }
        var one = pinned(name(query), in: target)
        one[kSecReturnAttributes as String] = true
        one[kSecReturnData as String] = true
        one[kSecMatchLimit as String] = kSecMatchLimitOne
        var found: CFTypeRef?
        let read = SecItemCopyMatching(one as CFDictionary, &found)
        guard read == errSecSuccess else { return (read, nil) }
        guard let item = found as? [String: Any], let value = item[kSecValueData as String] as? Data, sameDates(item) else {
            return (overwritten, nil)
        }
        if !root, !ItemSeal.fits(item[kSecAttrGeneric as String] as? Data, item, value, in: keychain, open: open) {
            return (overwritten, nil)
        }
        return (errSecSuccess, value)
    }

    private static func call(_ keychain: SecKeychain?, _ open: Bool?, _ body: (SecKeychain) -> OSStatus) -> OSStatus {
        let gate = keychain == nil ? KeychainGate.isOpen : (open ?? KeychainGate.isOpen)
        return KeychainGate.call(when: gate) {
            guard let target = keychain ?? login else { return errSecNoSuchKeychain }
            return body(target)
        }
    }

    /// A query that asks for a value gets it only from an item `checked` vouches for, as the value alone. `root`:
    /// the record key's and the items' key's own items, which carry no seal.
    static func copyMatching(_ query: [String: Any], _ result: UnsafeMutablePointer<CFTypeRef?>?,
                             in keychain: SecKeychain? = nil, open: Bool? = nil, root: Bool = false) -> OSStatus {
        call(keychain, open) { target in
            guard query[kSecReturnData as String] as? Bool == true else {
                return SecItemCopyMatching(pinned(query, in: target) as CFDictionary, result)
            }
            let (status, value) = checked(query, in: target, keychain: keychain, open: open, root: root)
            if status == errSecSuccess { result?.pointee = value as CFTypeRef? }
            return status
        }
    }

    /// Adds `item`, sealed unless `root`, and reads it back once the second of the add is over: `overwritten` if
    /// what's there then isn't what was added. That item is deleted, but for a root item, which is left for the
    /// record key's own store to deal with (RecordKey.swift).
    static func add(_ item: [String: Any], in keychain: SecKeychain? = nil, open: Bool? = nil, root: Bool = false) -> OSStatus {
        call(keychain, open) { target in
            guard let value = item[kSecValueData as String] as? Data else { return errSecParam }
            var entry = item
            if !root {
                guard let seal = ItemSeal.seal(item, value, in: keychain, open: open) else { return errSecNotAvailable }
                entry[kSecAttrGeneric as String] = seal
            }
            let status = SecItemAdd(pinned(entry, in: target, adding: true) as CFDictionary, nil)
            guard status == errSecSuccess else { return status }
            let added = Date().timeIntervalSince1970
            afterAdd?(name(item))
            let wait = added.rounded(.down) + 1.05 - Date().timeIntervalSince1970
            if wait > 0 { Thread.sleep(forTimeInterval: wait) }
            let (read, back) = checked(item, in: target, keychain: keychain, open: open, root: root)
            guard read == errSecSuccess, back == value else {
                if !root { SecItemDelete(pinned(name(item), in: target) as CFDictionary) }
                return overwritten
            }
            return errSecSuccess
        }
    }

    @discardableResult
    static func delete(_ query: [String: Any], in keychain: SecKeychain? = nil, open: Bool? = nil) -> OSStatus {
        call(keychain, open) { SecItemDelete(pinned(query, in: $0) as CFDictionary) }
    }
}

/// The seal on each of this app's items (RT126SK1 B2): an HMAC-SHA-256 of the item's service, account and value
/// under the items' key, 32 random bytes in an item of their own (`service`), kept like the record key: one item,
/// added once and read back, never changed (RecordKey.swift). A program running as you can't read that key, so an
/// item it adds under a free name (the API key, the GM's, the portable office's), or a value it puts in one, carries
/// no seal that fits, and is never read: the summaries never go to its API account, and its GM verifier never counts.
///
/// The key isn't the record key, so a rotation of the records leaves every item as it is. It's made new only when
/// it doesn't check out: at the update to 1.26 (with the items from before sealed as found, as the records are) and
/// at Setup's reset, when it's then planted, overwritten or unreadable (`renew`). Otherwise the first sealed add makes
/// it, deleting any item under its name first.
enum ItemSeal {
    static let service = SummaryKey.testMode ? "smolcorp-items-test" : "smolcorp-items"
    private static let lock = NSLock()
    /// The login keychain's items' key, once it checked out: it's never changed while it does.
    private static var known: Data?

    private static func store(_ keychain: SecKeychain?, _ open: Bool?) -> RecordKeychain {
        RecordKeychain(service: service, keychain: keychain, open: open)
    }

    private static func remember(_ key: Data?, for keychain: SecKeychain?) {
        guard keychain == nil else { return }
        lock.lock()
        known = key
        lock.unlock()
    }

    /// The items' key, if its item checks out; with `making`, one made when there's none.
    static func key(in keychain: SecKeychain?, open: Bool?, making: Bool) -> Data? {
        if keychain == nil {
            lock.lock()
            defer { lock.unlock() }
            if let known { return known }
        }
        var key: Data?
        switch store(keychain, open).look() {
        case .one(let item): key = item.value
        case .none where making: key = store(keychain, open).makeNew()?.value
        default: key = nil
        }
        if key != nil { remember(key, for: keychain) }
        return key
    }

    private static func message(_ item: [String: Any], _ value: Data) -> Data {
        let service = item[kSecAttrService as String] as? String ?? "", account = item[kSecAttrAccount as String] as? String ?? ""
        return Data("smolcorp-item-v1\u{0}\(service)\u{0}\(account)\u{0}".utf8) + value
    }

    /// The seal for `value` in the item `item` names: nil without an items' key.
    static func seal(_ item: [String: Any], _ value: Data, in keychain: SecKeychain?, open: Bool?) -> Data? {
        guard let key = key(in: keychain, open: open, making: true) else { return nil }
        return Data(HMAC<SHA256>.authenticationCode(for: message(item, value), using: SymmetricKey(data: key)))
    }

    /// Whether `seal` is this app's for `value` in the item `item` describes.
    static func fits(_ seal: Data?, _ item: [String: Any], _ value: Data, in keychain: SecKeychain?, open: Bool?) -> Bool {
        guard let seal, let key = key(in: keychain, open: open, making: false) else { return false }
        return HMAC<SHA256>.isValidAuthenticationCode(seal, authenticating: message(item, value), using: SymmetricKey(data: key))
    }

    /// The app's items that a 1.25 Mac may hold: sealed as found at the update to 1.26.
    static var appItems: [[String: Any]] {
        let generic = kSecClassGenericPassword
        return [[kSecClass as String: generic, kSecAttrService as String: SummaryKey.service, kSecAttrAccount as String: SummaryKey.account],
                [kSecClass as String: generic, kSecAttrService as String: GMKeychain.service, kSecAttrAccount as String: GMKeychain.account]]
            + [ViewerKeychain.Account.id, .key, .writeSecret].map {
                [kSecClass as String: generic, kSecAttrService as String: ViewerKeychain.service, kSecAttrAccount as String: $0.rawValue]
            }
    }

    /// At the update to 1.26 (`update`) or Setup's reset of the records: a new items' key, unless at a reset the one
    /// there checks out. At the update, each item of `adopting` there, unsealed, untouched and alone, is sealed as found.
    static func renew(update: Bool, adopting: [[String: Any]] = appItems, in keychain: SecKeychain? = nil, open: Bool? = nil) {
        if !update, case .one(let item) = store(keychain, open).look() {
            remember(item.value, for: keychain)
            return
        }
        remember(nil, for: keychain)
        guard let made = store(keychain, open).makeNew() else { return }
        remember(made.value, for: keychain)
        guard update else { return }
        for match in adopting {
            var query = match
            query[kSecReturnData as String] = true
            var found: CFTypeRef?
            guard Keychain.copyMatching(query, &found, in: keychain, open: open, root: true) == errSecSuccess, let value = found as? Data else { continue }
            var attributes = match
            attributes[kSecReturnAttributes as String] = true
            var about: CFTypeRef?
            _ = Keychain.copyMatching(attributes, &about, in: keychain, open: open)
            var item = match
            for field in [kSecAttrLabel, kSecAttrComment] {
                if let text = (about as? [String: Any])?[field as String] { item[field as String] = text }
            }
            item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
            item[kSecValueData as String] = value
            Keychain.delete(match, in: keychain, open: open)
            _ = Keychain.add(item, in: keychain, open: open)
        }
    }
}

/// A support folder's instance lock: `instance.lock`, locked (flock, exclusive) for as long as this app runs, as the
/// Go app takes it on Linux and Windows (desktop/instance_unix.go). On a Mac this app holds it, before it starts its
/// core or opens the Keychain, and the core checks it's held before it starts (desktop/core_darwin.go). A link at
/// its name is refused, never followed. The system lets go when the app ends, however it ends.
final class InstanceLock {
    let file: URL
    private var held: Int32 = -1

    init(support: URL) {
        file = support.appendingPathComponent("instance.lock")
    }

    var isHeld: Bool { held >= 0 }

    /// Takes it, unless another process holds it (another Smolcorp on this folder) or it can't be had: false then.
    func take() -> Bool {
        if held >= 0 { return true }
        let folder = file.deletingLastPathComponent()
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        let fd = open(file.path, O_CREAT | O_RDWR | O_NOFOLLOW | O_CLOEXEC, 0o600)
        guard fd >= 0 else { return false }
        guard flock(fd, LOCK_EX | LOCK_NB) == 0 else {
            close(fd)
            return false
        }
        held = fd
        return true
    }

    /// Lets go: for the tests. The app keeps it until it ends.
    func release() {
        guard held >= 0 else { return }
        close(held)
        held = -1
    }

    deinit { release() }
}

/// Setup's advice on the key Smolcorp is signed with (SANDBOX-PLAN §3.7, SD1): whether an identity for this app's own
/// signing certificate sits in an unlocked keychain of the search list. While it does, any program of yours can sign
/// a changed Smolcorp that meets this app's requirement, and so reads its items. Only certificates are compared: no
/// private key is used, and nothing is asked with a prompt. The search list is any program's to change, so a no proves
/// nothing, and Setup words the line as advice. Through the gate like every Keychain call: shut, it says nothing.
enum SigningKey {
    /// This app's leaf signing certificate, or nil for an ad hoc or unsigned build.
    static func ownCertificate() -> Data? {
        var code: SecCode?
        guard SecCodeCopySelf([], &code) == errSecSuccess, let code else { return nil }
        var staticCode: SecStaticCode?
        guard SecCodeCopyStaticCode(code, [], &staticCode) == errSecSuccess, let staticCode else { return nil }
        var info: CFDictionary?
        guard SecCodeCopySigningInformation(staticCode, SecCSFlags(rawValue: kSecCSSigningInformation), &info) == errSecSuccess,
              let certificates = (info as? [String: Any])?[kSecCodeInfoCertificates as String] as? [SecCertificate],
              let leaf = certificates.first else { return nil }
        return SecCertificateCopyData(leaf) as Data
    }

    /// Whether an identity whose certificate is `certificate` is in one of `keychains` that is unlocked (the search
    /// list's, without `keychains`). Nil when the gate is shut or the search list can't be read: not said, rather than no.
    static func unlocked(_ certificate: Data, in keychains: [SecKeychain]? = nil, open: Bool? = nil) -> Bool? {
        var found: Bool?
        _ = KeychainGate.call(when: open ?? KeychainGate.isOpen) {
            var list = keychains
            if list == nil {
                var searched: CFArray?
                guard SecKeychainCopySearchList(&searched) == errSecSuccess else { return errSecNoSuchKeychain }
                list = searched as? [SecKeychain]
            }
            let unlocked = (list ?? []).filter { keychain in
                var status: SecKeychainStatus = 0
                return SecKeychainGetStatus(keychain, &status) == errSecSuccess && status & UInt32(kSecUnlockStateStatus) != 0
            }
            found = false
            guard !unlocked.isEmpty else { return errSecSuccess }
            let query: [String: Any] = [
                kSecClass as String: kSecClassIdentity, kSecMatchSearchList as String: unlocked,
                kSecReturnRef as String: true, kSecMatchLimit as String: kSecMatchLimitAll,
                kSecUseAuthenticationUI as String: kSecUseAuthenticationUIFail,
            ]
            var rows: CFTypeRef?
            let status = SecItemCopyMatching(query as CFDictionary, &rows)
            guard status == errSecSuccess, let identities = rows as? [SecIdentity] else { return status }
            found = identities.contains { identity in
                var leaf: SecCertificate?
                guard SecIdentityCopyCertificate(identity, &leaf) == errSecSuccess, let leaf else { return false }
                return SecCertificateCopyData(leaf) as Data == certificate
            }
            return errSecSuccess
        }
        return found
    }

    private static let cacheLock = NSLock()
    private static var cached: (at: Date, unlocked: Bool?)?

    /// Setup's fact `signingKeyUnlocked`, asked every few seconds: looked at again at most once a minute. Null for an
    /// ad hoc build, and always for a test copy or under the tests, which leave the Keychain alone.
    static func fact(testCopy: Bool) -> Any {
        guard !testCopy, !Paths.underTest else { return NSNull() }
        cacheLock.lock()
        defer { cacheLock.unlock() }
        if let cached, Date().timeIntervalSince(cached.at) < 60 { return cached.unlocked.map { $0 as Any } ?? NSNull() }
        let unlocked = ownCertificate().flatMap { self.unlocked($0) }
        cached = (Date(), unlocked)
        return unlocked.map { $0 as Any } ?? NSNull()
    }
}
