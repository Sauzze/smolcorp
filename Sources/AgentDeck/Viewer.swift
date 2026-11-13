import AppKit
import CryptoKit
import Security

/// The viewer's settings in config.json: its address and the options. No
/// secret is ever kept here: the id, the key and the write secret are in the
/// Keychain.
struct ViewerSettings: Codable, Equatable {
    /// The viewer's address, as Vercel gives it: https://office-viewer-xxxx.vercel.app.
    var baseURL = ""
    /// Desks shown as "Desk 1", "Desk 2"… rather than by their project's name.
    var hideNames = false
    /// Turn it off when you're back at the Mac.
    var stopOnReturn = true
    /// Turn it off after this many hours; 0 is never.
    var stopAfterHours = 8

    static let hourChoices = [2, 4, 8, 12, 0]

    init() {}

    /// Field by field, so a file from another version still loads.
    init(from decoder: Decoder) throws {
        let c = try? decoder.container(keyedBy: CodingKeys.self)
        baseURL = ((try? c?.decodeIfPresent(String.self, forKey: .baseURL)) ?? nil) ?? ""
        hideNames = ((try? c?.decodeIfPresent(Bool.self, forKey: .hideNames)) ?? nil) ?? false
        stopOnReturn = ((try? c?.decodeIfPresent(Bool.self, forKey: .stopOnReturn)) ?? nil) ?? true
        let hours = ((try? c?.decodeIfPresent(Int.self, forKey: .stopAfterHours)) ?? nil) ?? 8
        stopAfterHours = ViewerSettings.hourChoices.contains(hours) ? hours : 8
    }

    /// The viewer's origin, scheme://host[:port], or nil if `text` isn't one
    /// it can send to: https only, but for http://localhost and 127.0.0.1,
    /// for trying it out on this Mac. No path, query, fragment or password.
    static func origin(of text: String) -> String? {
        var trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
        while trimmed.hasSuffix("/") { trimmed.removeLast() }
        guard let parts = URLComponents(string: trimmed), let scheme = parts.scheme?.lowercased(),
              let host = parts.host?.lowercased(), !host.isEmpty,
              parts.user == nil, parts.password == nil, parts.query == nil, parts.fragment == nil, parts.path.isEmpty,
              host.allSatisfy({ $0.isASCII && ($0.isLetter || $0.isNumber || $0 == "." || $0 == "-") }) else { return nil }
        guard scheme == "https" || (scheme == "http" && ["localhost", "127.0.0.1"].contains(host)) else { return nil }
        return "\(scheme)://\(host)" + (parts.port.map { ":\($0)" } ?? "")
    }

    static let addressProblem = "That isn't a viewer address. Paste the https:// address Vercel gives your project."
}

/// The viewer's id, key and write secret: generic passwords under one
/// service. Never in config.json, logs or the snapshot. A test copy keeps
/// its own, so trying one out never reads or replaces the real app's.
enum ViewerKeychain {
    static let service = TestCopy.isOn ? "smolcorp-viewer-test" : "smolcorp-viewer"

    enum Account: String { case id, key, writeSecret = "write-secret" }

    static func read(_ account: Account) -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account.rawValue,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var found: CFTypeRef?
        guard Keychain.copyMatching(query, &found) == errSecSuccess, let data = found as? Data,
              let value = String(data: data, encoding: .utf8), !value.isEmpty else { return nil }
        return value
    }

    /// Whether there is one, without reading it.
    static func has(_ account: Account) -> Bool {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account.rawValue,
            kSecReturnAttributes as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var found: CFTypeRef?
        return Keychain.copyMatching(query, &found) == errSecSuccess
    }

    /// Whether there's surely none: the Keychain said so. Any other answer
    /// (an error, a prompt that was denied) isn't taken for none.
    static func absent(_ account: Account) -> Bool {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account.rawValue,
            kSecReturnAttributes as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var found: CFTypeRef?
        return Keychain.copyMatching(query, &found) == errSecItemNotFound
    }

    /// Readable while the screen is locked, which is when it's used most,
    /// and never synced to other devices.
    @discardableResult
    static func write(_ value: String, for account: Account) -> Bool {
        delete(account)
        let item: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account.rawValue,
            kSecAttrLabel as String: "Smolcorp viewer (\(account.rawValue))",
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
            kSecValueData as String: Data(value.utf8),
        ]
        return Keychain.add(item) == errSecSuccess
    }

    static func delete(_ account: Account) {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account.rawValue,
        ]
        Keychain.delete(query)
    }
}

/// What the portable office needs of the Keychain: the real one, or a stand-in in tests.
/// Every call may stop on a dialog: the bridge's secrets make them for the Go core (`HostBridge`).
struct ViewerStore {
    var read: (ViewerKeychain.Account) -> String?
    var absent: (ViewerKeychain.Account) -> Bool
    var write: (String, ViewerKeychain.Account) -> Bool

    static let keychain = ViewerStore(
        read: { ViewerKeychain.read($0) }, absent: { ViewerKeychain.absent($0) }, write: { ViewerKeychain.write($0, for: $1) })
}

/// What the Keychain held when last asked, kept in memory so the main queue never asks.
/// Only whether each is there, never the id, key or write secret themselves: those are read
/// when setup or Start needs them, and dropped again.
struct ViewerKept: Equatable {
    /// There is a write secret, readable or not.
    var secretPresent: Bool
    /// There is an id and a key, readable or not.
    var linkPresent: Bool

    /// Asks the Keychain about each, attributes only (no data, so no dialog); on `keychainQueue` only.
    static func load(_ store: ViewerStore) -> ViewerKept {
        ViewerKept(secretPresent: !store.absent(.writeSecret), linkPresent: !store.absent(.id) && !store.absent(.key))
    }
}

/// "Stop when I'm back at the Mac", for the Go core's portable office (desktop/viewer.go's `macReturn`): what the
/// Swift engine's PortableOffice watched (Viewer.swift at 1da4e935), told through the bridge's `presence`. The
/// screen's unlocks, and the Mac's and its screens' wakes, are counted from the app's start: the core asks every
/// few seconds while the portable office is on, and a count gone up is one since it last asked. Nothing here asks
/// for a permission or shows anything.
final class MacPresence {
    /// What the core is told: the keyboard's and mouse's idle time in seconds, whether the screen is locked, and
    /// the unlocks and wakes counted so far.
    struct Now: Equatable {
        var idle: TimeInterval = 0
        var locked = false
        var unlocks = 0
        var wakes = 0
    }

    /// Idle for longer than this (never used since the Mac started) is said as this: a year.
    static let idleMost: TimeInterval = 365 * 24 * 3600

    static let shared = MacPresence()

    private let distributed: NotificationCenter
    private let workspace: NotificationCenter
    private let idle: () -> TimeInterval
    private let locked: () -> Bool
    /// Guards the counts: they're counted on the main queue and read on the bridge's.
    private let lock = NSLock()
    private var unlocks = 0
    private var wakes = 0
    private var watchers: [(NotificationCenter, NSObjectProtocol)] = []

    /// The Mac's own centres and readings, or a test's stand-ins.
    init(distributed: NotificationCenter = DistributedNotificationCenter.default(),
         workspace: NotificationCenter = NSWorkspace.shared.notificationCenter,
         idle: @escaping () -> TimeInterval = { MacPresence.idleSeconds },
         locked: @escaping () -> Bool = { MacPresence.screenIsLocked }) {
        self.distributed = distributed
        self.workspace = workspace
        self.idle = idle
        self.locked = locked
    }

    deinit { watchers.forEach { $0.0.removeObserver($0.1) } }

    /// Starts counting, once: on an unlock, as the Swift engine came back on it, and on a wake, after which it
    /// watched the keyboard and mouse (the core does that now, with `now`).
    func start() {
        lock.lock()
        defer { lock.unlock() }
        guard watchers.isEmpty else { return }
        let unlocked = distributed.addObserver(forName: Notification.Name("com.apple.screenIsUnlocked"), object: nil, queue: nil) { [weak self] _ in
            self?.count { $0.unlocks += 1 }
        }
        watchers.append((distributed, unlocked))
        for name in [NSWorkspace.didWakeNotification, NSWorkspace.screensDidWakeNotification] {
            let woke = workspace.addObserver(forName: name, object: nil, queue: nil) { [weak self] _ in self?.count { $0.wakes += 1 } }
            watchers.append((workspace, woke))
        }
    }

    private func count(_ change: (MacPresence) -> Void) {
        lock.lock()
        change(self)
        lock.unlock()
    }

    /// What's counted, with the idle time and the lock read now. On any queue.
    func now() -> Now {
        lock.lock()
        let counted = (unlocks, wakes)
        lock.unlock()
        return Now(idle: idle(), locked: locked(), unlocks: counted.0, wakes: counted.1)
    }

    /// Seconds since the keyboard or mouse was last used.
    static var idleSeconds: TimeInterval {
        guard let anyInput = CGEventType(rawValue: ~0) else { return .infinity }
        return CGEventSource.secondsSinceLastEventType(.combinedSessionState, eventType: anyInput)
    }

    static var screenIsLocked: Bool {
        guard let session = CGSessionCopyCurrentDictionary() as? [String: Any] else { return false }
        return (session["CGSSessionScreenIsLocked"] as? Bool) ?? false
    }
}

/// The body the viewer stores and the phone opens: [0x01] + nonce + ciphertext
/// + tag, AES-256-GCM over the padded JSON, bound to the viewer's id.
enum ViewerCrypto {
    static let version: UInt8 = 0x01
    /// The plaintext is padded with spaces to a multiple of this, so small changes don't show in its size.
    static let block = 1024
    /// The most a padded plaintext may be.
    static let most = 192 * 1024

    static func aad(for id: String) -> Data { Data("smolcorp-viewer:v1:\(id)".utf8) }

    /// `json` with trailing spaces to the next multiple of `block`, or nil over `most`.
    static func padded(_ json: Data) -> Data? {
        let size = max(block, (json.count + block - 1) / block * block)
        guard size <= most else { return nil }
        return json + Data(repeating: 0x20, count: size - json.count)
    }

    /// The plaintext: `{"at":…,"office":…,"v":1}`, padded; nil when it's too big.
    static func plaintext(office: [String: Any], at: Int64) -> Data? {
        let payload: [String: Any] = ["v": 1, "at": at, "office": office]
        guard let json = try? JSONSerialization.data(withJSONObject: payload, options: [.sortedKeys]) else { return nil }
        return padded(json)
    }

    /// A fresh random nonce each time, unless one is given (the test vector's).
    static func seal(_ plaintext: Data, key: Data, id: String, nonce: AES.GCM.Nonce = AES.GCM.Nonce()) throws -> Data {
        guard key.count == 32 else { throw CryptoKitError.incorrectKeySize }
        let box = try AES.GCM.seal(plaintext, using: SymmetricKey(data: key), nonce: nonce, authenticating: aad(for: id))
        guard let combined = box.combined else { throw CryptoKitError.incorrectParameterSize }
        return Data([version]) + combined
    }

    static func random(_ count: Int) -> Data? {
        var bytes = Data(count: count)
        let status = bytes.withUnsafeMutableBytes { SecRandomCopyBytes(kSecRandomDefault, count, $0.baseAddress!) }
        return status == errSecSuccess ? bytes : nil
    }

    static func base64URL(_ data: Data) -> String {
        data.base64EncodedString().replacingOccurrences(of: "+", with: "-").replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
    }

    static func data(base64URL text: String) -> Data? {
        var plain = text.replacingOccurrences(of: "-", with: "+").replacingOccurrences(of: "_", with: "/")
        while plain.count % 4 != 0 { plain += "=" }
        return Data(base64Encoded: plain)
    }

    /// 22 characters of base64url: 16 bytes.
    static func isID(_ text: String) -> Bool {
        text.count == 22 && text.allSatisfy { $0.isASCII && ($0.isLetter || $0.isNumber || $0 == "-" || $0 == "_") }
    }
}
