import Foundation
import Security

/// The Macs this one joined for floors from other Macs, with the key each gave (desktop/peers_mac_join.go's
/// `peer-keys`): what the core keeps, as one line of its own, in the bridge's item of that name. On the Mac it is one
/// Keychain item of this app's, in the login keychain opened by its path, added sealed and read back, and read only
/// while its dates and its seal show this app added it as it is (`Keychain`, SK1). Only this app reads it: the core
/// gets it over the bridge, never from a file or the Keychain itself. A test copy and the test suite keep it in a file
/// of their own (`TestCopySecrets`), and never ask the Keychain.
struct PeerKeys {
    static let service = SummaryKey.testMode ? "smolcorp-peers-test" : "smolcorp-peers"
    static let account = "peer-keys"

    var read: () -> String?
    /// Whether there's surely none: the Keychain said "not found", or a test copy has no file. An item there that
    /// can't be read now, or that another program changed or added (its dates moved, or no seal of the app's), is
    /// never none, so the core never writes a list with only the next Mac over the Macs joined before.
    var absent: () -> Bool
    var write: (String) -> Bool
    var remove: () -> Void

    /// The bridge's `peer-keys` item: removed this way too, which is how a list that fails its check can be let go.
    var secret: HostBridge.Secret {
        HostBridge.Secret(read: read, write: write, delete: remove, absent: absent)
    }

    /// The login keychain's item, or the item in `keychain`, a test's own throwaway file (`open` then stands for the gate).
    static func keychain(_ keychain: SecKeychain? = nil, open: Bool? = nil) -> PeerKeys {
        let match: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service,
                                    kSecAttrAccount as String: account]
        return PeerKeys(
            read: {
                var query = match
                query[kSecReturnData as String] = true
                query[kSecMatchLimit as String] = kSecMatchLimitOne
                var found: CFTypeRef?
                guard Keychain.copyMatching(query, &found, in: keychain, open: open) == errSecSuccess, let data = found as? Data,
                      let value = String(data: data, encoding: .utf8), !value.isEmpty else { return nil }
                return value
            },
            absent: {
                // Its attributes alone: no value is read, so nothing is asked and no seal is needed to know it's there.
                var query = match
                query[kSecReturnAttributes as String] = true
                query[kSecMatchLimit as String] = kSecMatchLimitOne
                var found: CFTypeRef?
                return Keychain.copyMatching(query, &found, in: keychain, open: open) == errSecItemNotFound
            },
            write: { value in
                // The app only ever adds its items, never changes one: an item changed since has dates that differ.
                Keychain.delete(match, in: keychain, open: open)
                var item = match
                item[kSecAttrLabel as String] = "Smolcorp floors from other Macs"
                item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
                item[kSecValueData as String] = Data(value.utf8)
                return Keychain.add(item, in: keychain, open: open) == errSecSuccess
            },
            remove: { Keychain.delete(match, in: keychain, open: open) })
    }

    /// In a file of a test copy's own: see `TestCopySecrets`. No file is none; one there that can't be read isn't.
    static func file(_ file: URL) -> PeerKeys {
        PeerKeys(read: { TestCopySecrets.read(file) }, absent: { !FileManager.default.fileExists(atPath: file.path) },
                 write: { TestCopySecrets.write($0, to: file) }, remove: { TestCopySecrets.remove(file) })
    }

    /// The Keychain, or in a test copy a file in its own support folder (the Keychain is never asked there).
    static func defaultStore(testCopy: Bool = TestCopy.isOn, support: URL? = TestCopy.support, underTest: Bool = Paths.underTest) -> PeerKeys {
        if testCopy, let support { return .file(support.appendingPathComponent(TestCopySecrets.peerKeysName)) }
        // The test suite never reads or replaces the owner's own item.
        if underTest { return .file(Paths.testSupport.appendingPathComponent(TestCopySecrets.peerKeysName)) }
        return .keychain()
    }
}
