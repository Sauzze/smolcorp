import Foundation
import Security

/// Where the GM's token is kept between runs of the app.
protocol GMTokenStore {
    func read() -> String?
    func write(_ token: String) -> Bool
    func remove()
}

/// The token in the Mac's Keychain, under the app's own item. A test copy
/// has an item of its own, never the real one's.
struct GMKeychain: GMTokenStore {
    static let service = SummaryKey.testMode ? "smolcorp-gm-test" : "smolcorp-gm"
    static let account = "gm-token"

    private var match: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: GMKeychain.service, kSecAttrAccount as String: GMKeychain.account]
    }

    func read() -> String? {
        var query = match
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var found: CFTypeRef?
        guard SecItemCopyMatching(query as CFDictionary, &found) == errSecSuccess, let data = found as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }

    func write(_ token: String) -> Bool {
        SecItemDelete(match as CFDictionary)
        var item = match
        item[kSecAttrLabel as String] = "Smolcorp General Manager token"
        item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        item[kSecValueData as String] = Data(token.utf8)
        return SecItemAdd(item as CFDictionary, nil) == errSecSuccess
    }

    func remove() { SecItemDelete(match as CFDictionary) }
}

/// Who the General Manager is: whoever sends the GM token, and nobody else.
/// Not a session's name, not a socket: hires run as you, so they could name
/// a session or open a socket of their own.
///
/// The token is made when the GM is hired (or the first time something needs
/// it) and kept in memory and in the Keychain under the app's own item, until
/// it's rotated. It is in no file: the GM's start script puts it in the GM's
/// own environment (`SMOLCORP_GM_TOKEN`) and nothing else gets it. The app
/// never puts it in its own environment, so nothing it starts (tmux, a hire)
/// inherits it.
///
/// Only for people who start their GM by hand, the setting "Allow a GM
/// started by hand" also writes `<support>/gm-token` (only you can read it),
/// and with it off the file is taken away. A hire is kept from reading that
/// file by the deny rules it starts with, not by this class. A hire that reads
/// the GM's process environment (`ps eww`, as you) still gets the token: the
/// wall for that is a separate Mac account. The setup-key file doesn't open
/// the GM either: see `SetupRoutes`.
final class GMIdentity {
    /// The header the GM sends its token in (names are lowercased on arrival).
    static let header = "x-smolcorp-gm-token"
    /// The header the GM also sends the id of its own Claude Code session in, so the app can tell which session it is.
    static let sessionHeader = "x-smolcorp-session"
    /// Where the GM's launch puts the token, in its own environment.
    static let environment = "SMOLCORP_GM_TOKEN"

    let tokenFile: URL
    /// Where the by-hand GM's session id is kept between runs: the id alone, beside the token file.
    let sessionFile: URL
    /// Whether a session with this exact id is running now. The deck sets it; until then nothing is live.
    var isLive: (String) -> Bool = { _ in false }
    private let store: GMTokenStore
    private let fileAllowed: () -> Bool
    private let lock = NSLock()
    private var kept: String?
    private var loaded = false
    private var seen = false
    /// The session a by-hand GM said it runs in, while `seen` holds. In memory only, like `seen`.
    private var seenSession: String?
    /// The session id kept in `sessionFile`: it counts as the GM's only while a live session has exactly that id (see `restoreIfLive`).
    private var remembered: String?

    init(store: GMTokenStore = GMIdentity.defaultStore(), tokenFile: URL = Paths.gmToken, fileAllowed: @escaping () -> Bool = { Config.shared.gmByHand }) {
        self.store = store
        self.tokenFile = tokenFile
        self.sessionFile = tokenFile.deletingLastPathComponent().appendingPathComponent(GMIdentity.sessionFileName)
        self.fileAllowed = fileAllowed
        // Only with by-hand on. Any token made since it was written deleted it (see `make`), so it is the same GM's.
        if fileAllowed(), let text = try? String(contentsOf: sessionFile, encoding: .utf8) {
            remembered = GMIdentity.validSession(text.trimmingCharacters(in: .whitespacesAndNewlines))
        }
    }

    /// The file the by-hand GM's session id is kept in, in the support folder.
    static let sessionFileName = "gm-session"

    /// Where the token is kept: the Keychain, or in a test copy a file in its own support
    /// folder (the Keychain is never asked there: see `TestCopySecrets`).
    static func defaultStore(testCopy: Bool = TestCopy.isOn, support: URL? = TestCopy.support, underTest: Bool = Paths.underTest,
                             keychain: () -> GMTokenStore = { GMKeychain() }) -> GMTokenStore {
        if testCopy, let support { return GMTokenFile(file: support.appendingPathComponent(TestCopySecrets.gmTokenName)) }
        // The test suite never reads or replaces the owner's own item.
        if underTest { return GMTokenFile(file: Paths.testSupport.appendingPathComponent(TestCopySecrets.gmTokenName)) }
        return keychain()
    }

    /// A token as it's kept: 43 base64url characters (32 random bytes), else none.
    private static func valid(_ text: String) -> String? {
        let token = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard token.utf8.count == 43, token.utf8.allSatisfy({ $0 == 45 || $0 == 95 || ($0 >= 48 && $0 <= 57) || ($0 >= 65 && $0 <= 90) || ($0 >= 97 && $0 <= 122) }) else { return nil }
        return token
    }

    /// The token in memory, read from the Keychain the first time. Call with the lock held.
    private func current() -> String? {
        if !loaded {
            kept = store.read().flatMap(GMIdentity.valid)
            loaded = true
        }
        return kept
    }

    /// The token, made now if there is none yet. Nil only if it can't be kept.
    func token() -> String? {
        lock.lock(); defer { lock.unlock() }
        if let token = current() {
            syncFile(token)
            return token
        }
        return make()
    }

    /// Whether a header's value is the GM's token. It never makes one: with
    /// no token yet, nobody is the GM.
    func isToken(_ candidate: String?) -> Bool {
        guard let candidate, !candidate.isEmpty else { return false }
        lock.lock(); defer { lock.unlock() }
        guard let token = current() else { return false }
        let isGM = constantTimeEqual(candidate, token)
        // Kept in memory only, and only while a GM by hand is allowed: Settings says so.
        if isGM && fileAllowed() { seen = true }
        return isGM
    }

    /// Whether a GM started by hand has used the token since the app started
    /// (or since the token was last rotated), with the setting on.
    var startedByHand: Bool {
        restoreIfLive()
        lock.lock(); defer { lock.unlock() }
        return seen && fileAllowed()
    }

    /// A session id as it may come in the header: 1 to 64 letters, digits, "-" or "_", else none.
    static func validSession(_ text: String?) -> String? {
        guard let text, !text.isEmpty, text.utf8.count <= 64,
              text.utf8.allSatisfy({ $0 == 45 || $0 == 95 || ($0 >= 48 && $0 <= 57) || ($0 >= 65 && $0 <= 90) || ($0 >= 97 && $0 <= 122) }) else { return nil }
        return text
    }

    /// The by-hand GM's session: remembered (in memory) only after its token worked with by-hand on.
    /// The caller has checked that the id is a live session.
    func rememberSession(_ id: String) {
        lock.lock(); defer { lock.unlock() }
        guard seen, fileAllowed(), GMIdentity.validSession(id) != nil else { return }
        seenSession = id
        remembered = id
        saveRemembered()
    }

    /// After a restart, the remembered session counts as the by-hand GM (as if its token had just worked)
    /// once a live session has exactly that id. Never another session, never a folder.
    /// `isLive` is asked with the lock free: it reads the deck's sessions.
    private func restoreIfLive() {
        lock.lock()
        // With no token there is no GM to restore.
        let id = seenSession == nil && fileAllowed() && current() != nil ? remembered : nil
        lock.unlock()
        guard let id, isLive(id) else { return }
        lock.lock(); defer { lock.unlock() }
        if seenSession == nil, remembered == id, fileAllowed() {
            seen = true
            seenSession = id
        }
    }

    /// Keeps the id in `sessionFile`: the id alone, readable by you alone. Call with the lock held.
    private func saveRemembered() {
        guard let remembered else { return }
        _ = Paths.writePrivately(Data("\(remembered)\n".utf8), to: sessionFile)
    }

    /// The mark is gone, and so is its file. Call with the lock held.
    private func clearRemembered() {
        seenSession = nil
        remembered = nil
        try? FileManager.default.removeItem(at: sessionFile)
    }

    /// The session a GM started by hand runs in, if it has said so; nil once it's forgotten or by-hand is off.
    var session: String? {
        restoreIfLive()
        lock.lock(); defer { lock.unlock() }
        return seen && fileAllowed() ? seenSession : nil
    }

    /// The session took another's place in its tab (after /clear): the GM goes with it.
    func moveSession(from old: String, to new: String) {
        lock.lock(); defer { lock.unlock() }
        if seenSession == old { seenSession = new }
        if remembered == old {
            remembered = new
            saveRemembered()
        }
    }

    /// The GM's hire has ended (it was fired): Settings stops saying it was started by hand.
    func forgetStartedByHand() {
        lock.lock(); defer { lock.unlock() }
        seen = false
        clearRemembered()
    }

    /// A new token in place of the old, which stops working at once: the GM
    /// must be started again with it (hiring a GM does that). Nil if it can't
    /// be kept (the old one is then as it was).
    func rotate() -> String? {
        lock.lock(); defer { lock.unlock() }
        _ = current()
        seen = false
        clearRemembered()
        return make()
    }

    /// Puts the token file as the setting says: written with the token while it's allowed
    /// (making a token if there is none), gone while it isn't. Called when the setting changes
    /// and when the app starts, which takes away the file an earlier build left.
    func applySetting() {
        lock.lock(); defer { lock.unlock() }
        if fileAllowed() {
            if let token = current() ?? make() { syncFile(token) }
        } else {
            seen = false
            clearRemembered()
            try? FileManager.default.removeItem(at: tokenFile)
        }
    }

    /// Call with the lock held.
    private func make() -> String? {
        let fresh = randomKey()
        guard store.write(fresh) else { return nil }
        // A session id kept for the old token is not this token's GM's.
        clearRemembered()
        seen = false
        kept = fresh
        loaded = true
        syncFile(fresh)
        return fresh
    }

    /// The file holds the token only while the setting is on, and is gone otherwise.
    private func syncFile(_ token: String) {
        if fileAllowed() {
            _ = Paths.writePrivately(Data("\(token)\n".utf8), to: tokenFile)
        } else if FileManager.default.fileExists(atPath: tokenFile.path) {
            try? FileManager.default.removeItem(at: tokenFile)
        }
    }
}
