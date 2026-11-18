import Foundation
import Security

/// The VAPID key the Go core signs every notification with (desktop/pushkey.go): one of the app's own items, which
/// the core reaches only through the bridge (`push-key`), never in a file or the Keychain itself. The core makes it
/// the first time a phone turns notifications on, and keeps it for good: every phone's subscription is made for it.
/// So it's never removed this way, and one that's there but can't be read is never "none" (the core would make a new
/// one over it). Sealed like every item of the app's (`Keychain.add`), so one another program added or changed is
/// never read. A test copy keeps its own, as it does the viewer's.
enum PushKeychain {
    static let service = TestCopy.isOn ? "smolcorp-push-test" : "smolcorp-push"
    static let account = "vapid-key"

    private static var match: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service, kSecAttrAccount as String: account]
    }

    static func read() -> String? {
        var query = match
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var found: CFTypeRef?
        guard Keychain.copyMatching(query, &found) == errSecSuccess, let data = found as? Data,
              let value = String(data: data, encoding: .utf8), !value.isEmpty else { return nil }
        return value
    }

    /// Whether there's surely none: the Keychain said so. Any other answer isn't taken for none.
    static func absent() -> Bool {
        var query = match
        query[kSecReturnAttributes as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var found: CFTypeRef?
        return Keychain.copyMatching(query, &found) == errSecItemNotFound
    }

    /// Readable while the screen is locked, when a desk is most likely to ring, and never synced to other devices.
    static func write(_ value: String) -> Bool {
        Keychain.delete(match)
        var item = match
        item[kSecAttrLabel as String] = "Smolcorp notifications (VAPID key)"
        item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        item[kSecValueData as String] = Data(value.utf8)
        return Keychain.add(item) == errSecSuccess
    }
}

/// Where the push key is kept: the Keychain, or a file in a test copy's own support folder, and the test suite's.
struct PushKeyStore {
    var read: () -> String?
    var absent: () -> Bool
    var write: (String) -> Bool

    static let keychain = PushKeyStore(read: { PushKeychain.read() }, absent: { PushKeychain.absent() },
                                       write: { PushKeychain.write($0) })

    /// The key in a file of a test copy's own: see `TestCopySecrets`. `absent` only where there's no file at all.
    static func file(_ file: URL) -> PushKeyStore {
        PushKeyStore(read: { TestCopySecrets.read(file) }, absent: { !FileManager.default.fileExists(atPath: file.path) },
                     write: { TestCopySecrets.write($0, to: file) })
    }

    /// The Keychain, or in a test copy a file in its own support folder (the Keychain is never asked there).
    static func defaultStore(testCopy: Bool = TestCopy.isOn, support: URL? = TestCopy.support, underTest: Bool = Paths.underTest,
                             keychain: PushKeyStore = .keychain) -> PushKeyStore {
        if testCopy, let support { return .file(support.appendingPathComponent(TestCopySecrets.pushKeyName)) }
        // The test suite never reads or replaces the owner's own item.
        if underTest { return .file(Paths.testSupport.appendingPathComponent(TestCopySecrets.pushKeyName)) }
        return keychain
    }

    /// As the bridge's secret table has it (`HostBridge.Actions`): read, kept, never removed.
    var secret: HostBridge.Secret {
        HostBridge.Secret(read: read, write: write, delete: nil, absent: absent)
    }
}
