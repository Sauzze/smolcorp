import Foundation
import Security

/// What reading the kept token found: the text kept (checked later), no item at all,
/// or an item that couldn't be read (a locked Keychain, an app signed another way).
/// Only `none` means there is no GM yet: a read that failed never makes a new token.
enum GMTokenRead: Equatable {
    case token(String)
    case none
    case failed
}

/// Where the GM's token is kept between runs of the app.
protocol GMTokenStore {
    func read() -> GMTokenRead
    func write(_ token: String) -> Bool
    func remove()
}

/// What the core keeps of the GM's token in the Mac's Keychain, under the app's own item: since 1.26 only its
/// verifier ("v1:" and its SHA-256, then the core's seal of it under the record key), never the token (SANDBOX-PLAN
/// §3.1, desktop/gmtokenstore.go). The item is sealed by this app too (`ItemSeal`): one another program added is
/// never read. A test copy has an item of its own, never the real one's.
struct GMKeychain: GMTokenStore {
    static let service = SummaryKey.testMode ? "smolcorp-gm-test" : "smolcorp-gm"
    static let account = "gm-token"

    private var match: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: GMKeychain.service, kSecAttrAccount as String: GMKeychain.account]
    }

    func read() -> GMTokenRead {
        var query = match
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var found: CFTypeRef?
        let status = Keychain.copyMatching(query, &found)
        return GMKeychain.result(status: status, data: found as? Data)
    }

    /// Only "not found" is no item. Any other failure is an item there that this run can't read;
    /// an item that reads but isn't text is no token, and is replaced like one.
    static func result(status: OSStatus, data: Data?) -> GMTokenRead {
        switch status {
        case errSecSuccess: return .token(data.flatMap { String(data: $0, encoding: .utf8) } ?? "")
        case errSecItemNotFound: return .none
        default: return .failed
        }
    }

    func write(_ token: String) -> Bool {
        Keychain.delete(match)
        var item = match
        item[kSecAttrLabel as String] = "Smolcorp General Manager token"
        item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        item[kSecValueData as String] = Data(token.utf8)
        return Keychain.add(item) == errSecSuccess
    }

    func remove() { Keychain.delete(match) }
}

/// Where the General Manager's token's verifier is kept on this Mac. The Go core makes the
/// token, gives it to the GM alone, and checks who sends it against the verifier; this app only holds the
/// Keychain item for that verifier (the bridge's "gm-token" secret), and reads it only if no other program
/// overwrote it (`Keychain`).
enum GMIdentity {
    /// Where the token is kept: the Keychain, or in a test copy a file in its own support
    /// folder (the Keychain is never asked there: see `TestCopySecrets`).
    static func defaultStore(testCopy: Bool = TestCopy.isOn, support: URL? = TestCopy.support, underTest: Bool = Paths.underTest,
                             keychain: () -> GMTokenStore = { GMKeychain() }) -> GMTokenStore {
        if testCopy, let support { return GMTokenFile(file: support.appendingPathComponent(TestCopySecrets.gmTokenName)) }
        // The test suite never reads or replaces the owner's own item.
        if underTest { return GMTokenFile(file: Paths.testSupport.appendingPathComponent(TestCopySecrets.gmTokenName)) }
        return keychain()
    }
}
