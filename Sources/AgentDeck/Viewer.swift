import AppKit
import CryptoKit
import Security

/// The portable office (beta): the office, seen on a phone from
/// anywhere, view only. While it's on, a redacted snapshot of the office is
/// locked with a key only this Mac and the link know, and put on the
/// viewer you run on your own Vercel, which never sees the key. Off at every
/// start; nothing is snapshotted or sent until you turn it on.
/// The contract with the viewer page, v1, is in `viewer/README.md`.

// MARK: Settings

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

// MARK: Keychain

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
        guard SecItemCopyMatching(query as CFDictionary, &found) == errSecSuccess, let data = found as? Data,
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
        return SecItemCopyMatching(query as CFDictionary, &found) == errSecSuccess
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
        return SecItemCopyMatching(query as CFDictionary, &found) == errSecItemNotFound
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
        return SecItemAdd(item as CFDictionary, nil) == errSecSuccess
    }

    static func delete(_ account: Account) {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account.rawValue,
        ]
        SecItemDelete(query as CFDictionary)
    }
}

// MARK: Crypto

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

// MARK: Snapshot

/// What the viewer draws, built field by field from what's allowed (spec
/// §3), never by taking the phone's snapshot and removing what isn't. The
/// same shape as `Deck.snapshotJSON()`, so office.js draws it unchanged.
enum ViewerSnapshot {
    struct Desk {
        let pin: String?
        let session: Session?
        let role: Role?
    }

    /// With names hidden, what can still carry your own text goes as a
    /// generic value: an error kind the office doesn't know (it's an agent
    /// hook's raw text), a subagent type that isn't one of Claude Code's own
    /// (custom agents are named by you), and a role you made or renamed.
    static let knownErrorKinds: Set<String> = ["rate_limit", "overloaded", "billing_error", "authentication_failed",
                                               "oauth_org_not_allowed", "account_on_hold", "cloud_credential_error"]
    static let knownSubagentTypes: Set<String> = ["general-purpose", "Explore", "Plan", "statusline-setup",
                                                  "output-style-setup", "claude-code-guide"]

    static func shownErrorKind(_ kind: String) -> String { knownErrorKinds.contains(kind) ? kind : "error" }
    static func shownSubagentType(_ type: String) -> String { knownSubagentTypes.contains(type) ? type : "agent" }

    /// The badge and outfit still draw; the id and name only as shipped.
    static func shownRole(_ role: Role) -> [String: Any] {
        let shipped = Role.shipped.first { $0.id == role.id }
        return ["id": shipped == nil ? "role" : role.id, "name": shipped?.name == role.name ? role.name : "",
                "badge": role.badge, "outfit": role.outfit]
    }

    static func build(desks: [Desk], overflow: Int, theme: [String: String], music: (enabled: Bool, playing: Bool?),
                      layout: Layout, hideNames: Bool, now: Int64, alias: (String) -> String) -> [String: Any] {
        // A desk's name, when names are hidden, by the session's folder (what office.js finds the team by).
        var deskOfName: [String: String] = [:]
        let keys = desks.enumerated().map { index, desk -> [String: Any] in
            let deskName = "Desk \(index + 1)"
            var entry: [String: Any] = [
                "pin": hideNames ? NSNull() : desk.pin.map { ($0 as NSString).lastPathComponent } ?? NSNull(),
            ]
            if let s = desk.session {
                if deskOfName[s.name] == nil { deskOfName[s.name] = hideNames ? deskName : s.name }
                entry["session"] = [
                    "id": alias(s.id),
                    "name": hideNames ? deskName : s.name,
                    "project": hideNames ? deskName : s.projectName,
                    "state": s.state.rawValue,
                    "effort": s.pendingEffort.isEmpty ? s.effort : s.pendingEffort,
                    "pending": !s.pendingEffort.isEmpty,
                    "agent": s.agent.rawValue, "agentName": s.agent.name,
                    "compacting": s.compacting,
                    "turnStartedAt": s.turnStartedAt.map { Int64($0) } ?? NSNull(),
                    "errorKind": s.errorKind.isEmpty ? NSNull() : hideNames ? shownErrorKind(s.errorKind) : s.errorKind,
                    "subagents": s.subagents.map {
                        ["id": alias($0.id), "type": hideNames ? shownSubagentType($0.type) : $0.type, "model": NSNull()] as [String: Any]
                    },
                    "role": desk.role.map { hideNames ? shownRole($0) : $0.forPhone } ?? NSNull(),
                    // Nothing to press
                    "dial": false, "mic": false, "dictates": false, "keys": false, "fire": false, "summary": false,
                ] as [String: Any]
            }
            return entry
        }

        // The room, with only the team members at a desk now, under the desk's name when names are hidden.
        var room = layout.room
        var team: [String: TeamMember] = [:]
        for (name, shown) in deskOfName {
            guard var member = room.team[name] else { continue }
            if hideNames { member.name = "" }
            team[shown] = member
        }
        room.team = team
        // Rules match model names, which never leave
        room.dress.rules = []
        let layoutObject = (layout.jsonObject as? [String: Any]) ?? [:]
        // The room as the Mac's office draws it (its plan of added pieces, the team's own names),
        // with the same team and dress redactions as above. It's the same encoded `Room` as the one
        // above (Layout.jsonObject encodes the struct), so either gives the phone the full room.
        // Of the room, only the team (its projects and nicknames) and the dress rules (parts of model
        // names) hold your own text: every other text in it is one of a fixed list of choices, checked
        // as the room is read. The self-check holds the raw room to that with names hidden.
        var roomObject = (try? JSONSerialization.jsonObject(with: JSONEncoder().encode(room))) as? [String: Any] ?? [:]
        if var raw = layoutObject["room"] as? [String: Any] {
            let rawTeam = raw["team"] as? [String: Any] ?? [:]
            var shownTeam: [String: Any] = [:]
            for (name, shown) in deskOfName {
                guard var member = rawTeam[name] as? [String: Any] else { continue }
                if hideNames { member["name"] = "" }
                shownTeam[shown] = member
            }
            raw["team"] = shownTeam
            if var dress = raw["dress"] as? [String: Any] {
                dress["rules"] = [Any]()
                raw["dress"] = dress
            }
            roomObject = raw
        }
        var arranged: [String: Any] = ["room": roomObject, "rows": [Any]()]
        for field in ["keys", "office", "dial", "stick"] { arranged[field] = layoutObject[field] }

        return [
            "keys": keys,
            "overflow": overflow,
            "now": now,
            "theme": theme,
            "scroll": false,
            "dictation": NSNull(),
            "hiring": ["fire": false, "hire": false, "roles": [Any]()] as [String: Any],
            "music": music.enabled ? ["playing": music.playing.map { $0 as Any } ?? NSNull()] as [String: Any] : NSNull(),
            "layout": arranged,
        ]
    }
}

extension ViewerSnapshot {
    /// Who hired a headless run, as the viewer may name them (see
    /// `Deck.hiredBy`): by name with names shown; with names hidden, the
    /// number of the hiring desk, as that desk shows it, or "Downstairs".
    static func hirer(_ parent: (session: Session?, name: String)?, desks: [Session?], hideNames: Bool) -> String? {
        guard let parent else { return nil }
        guard hideNames else { return parent.name }
        return desks.firstIndex { $0 != nil && $0 === parent.session }.map { "Desk \($0 + 1)" } ?? "Downstairs"
    }

    /// The second floor's headless runs, as the viewer may see them: each
    /// desk's name, under the same rule as the desks downstairs, and its
    /// state, and the name of whoever hired it, where `hiredBy` knows,
    /// already under that rule. Nothing else of them leaves this Mac.
    static func upstairs(_ sessions: [Session], overflow: Int, hideNames: Bool,
                         hiredBy: (Session) -> String? = { _ in nil }) -> [String: Any] {
        let desks = sessions.enumerated().map { index, s -> [String: Any] in
            var desk: [String: Any] = ["name": hideNames ? "Upstairs \(index + 1)" : s.name, "state": s.state.rawValue]
            if let name = hiredBy(s) { desk["hiredBy"] = ["name": name] }
            return desk
        }
        return ["desks": desks, "overflow": overflow]
    }
}

// MARK: The portable office

/// Turning it on and off, and everything while it's on: the uploads (at
/// most one every 5 s after a change, one in flight, a heartbeat every 60 s,
/// backing off when the viewer can't be reached), stopping when you're back
/// or after some hours, and the setup page's actions.
final class PortableOffice {
    static let debounce: TimeInterval = 5
    static let heartbeat: TimeInterval = 60
    static let backoff: [TimeInterval] = [5, 10, 30, 60]
    static let timeout: TimeInterval = 10
    /// How long a quit waits for the viewer to forget the office.
    static let quitWait: TimeInterval = 5
    /// Coming back this soon after turning it on doesn't count: launch, then lock.
    static let returnGrace: TimeInterval = 120
    /// How long after a wake to watch for you being at the Mac.
    static let wakeWatch: TimeInterval = 600

    private(set) var isOn = false
    private(set) var since: Date?
    private(set) var lastSent: Date?
    /// What went wrong, in a few plain words.
    private(set) var problem: String?
    /// How the last "Check the viewer" or wipe went, for the setup page.
    private(set) var checked: String?
    /// Changes with the link, so the setup page knows to redraw its QR code.
    private(set) var linkRevision = 0
    /// Called when it turns on or off or an upload goes or fails, for the menu bar.
    var onChange: () -> Void = {}

    private let snapshot: (_ hideNames: Bool, _ alias: (String) -> String) -> [String: Any]
    /// Each session's and subagent's stand-in id, "v1", "v2"…, for this run.
    private var aliases: [String: String] = [:]
    /// Read once as it starts, so the Keychain isn't asked at every upload.
    private var credentials: (base: String, id: String, key: Data, secret: String)?
    /// Changes at each start and stop, so a reply from before is ignored.
    private var run = 0
    private var inFlight = false
    private var pending = false
    private var failures = 0
    private var lastAttempt: Date?
    private var debounceTimer: Timer?
    private var retryTimer: Timer?
    private var heartbeatTimer: Timer?
    private var activityTimer: Timer?
    /// An unlock during `returnGrace`, looked at again once it's over.
    private var graceTimer: Timer?
    private var watchers: [(NotificationCenter, NSObjectProtocol)] = []
    /// A PUT already sent can reach the viewer after a DELETE and bring the
    /// office back. So each id told to forget while PUTs to it are still out
    /// is told again once they've finished. Guarded by `lock`: PUTs finish on
    /// the session's queue, and on quit the main thread is waiting.
    private let lock = NSLock()
    private var putsOut: [String: Int] = [:]
    private var forgotten: [String: (base: String, secret: String)] = [:]
    /// Every PUT and DELETE still out, so a quit can wait for them.
    private let outstanding = DispatchGroup()
    private lazy var secretKnown = ViewerKeychain.has(.writeSecret)

    init(snapshot: @escaping (_ hideNames: Bool, _ alias: (String) -> String) -> [String: Any]) {
        self.snapshot = snapshot
    }

    // MARK: State

    var origin: String? { ViewerSettings.origin(of: Config.shared.viewer.baseURL) }
    var hasWriteSecret: Bool { secretKnown }
    /// A viewer address and a write secret: enough to turn it on.
    var isSetUp: Bool { origin != nil && hasWriteSecret }

    /// The link to open on the phone, with the key after #k=, which browsers never send.
    var link: String? {
        if let c = credentials { return "\(c.base)/v/\(c.id)#k=\(ViewerCrypto.base64URL(c.key))" }
        guard let base = origin, let id = ViewerKeychain.read(.id), ViewerCrypto.isID(id),
              let key = ViewerKeychain.read(.key), ViewerCrypto.data(base64URL: key)?.count == 32 else { return nil }
        return "\(base)/v/\(id)#k=\(key)"
    }

    /// The menu's header, after "Portable office: ON".
    var status: String {
        if let problem { return problem.prefix(1).lowercased() + problem.dropFirst() }
        if let lastSent { return "last sent \(PortableOffice.ago(lastSent))" }
        guard let since else { return "starting" }
        let time = DateFormatter()
        time.dateStyle = .none
        time.timeStyle = .short
        return "since \(time.string(from: since))"
    }

    static func ago(_ date: Date) -> String {
        let seconds = max(0, Int(Date().timeIntervalSince(date)))
        if seconds < 60 { return "\(seconds) s ago" }
        if seconds < 3600 { return "\(seconds / 60) min ago" }
        return "\(seconds / 3600) h ago"
    }

    /// For the setup page. The link (which holds the key) is the only secret
    /// in it, and only in the reply to `GET /setup/viewer` (spec §7): every
    /// other reply says whether there is one, and the page asks for it.
    func setupState(withLink: Bool = false) -> [String: Any] {
        let settings = Config.shared.viewer
        let ms = { (date: Date?) -> Any in date.map { Int64($0.timeIntervalSince1970 * 1000) } ?? NSNull() }
        let current = isSetUp ? link : nil
        return [
            "baseURL": settings.baseURL, "hasSecret": hasWriteSecret, "setUp": isSetUp,
            "on": isOn, "since": ms(since), "lastSent": ms(lastSent),
            "problem": problem ?? NSNull(), "checked": checked ?? NSNull(),
            "hasLink": current != nil, "link": withLink ? current.map { $0 as Any } ?? NSNull() : NSNull(), "revision": linkRevision,
            "hideNames": settings.hideNames, "stopOnReturn": settings.stopOnReturn,
            "stopAfterHours": settings.stopAfterHours, "hourChoices": ViewerSettings.hourChoices,
        ]
    }

    // MARK: On and off

    func start() {
        guard !isOn else { return }
        guard let base = origin, let secret = ViewerKeychain.read(.writeSecret) else {
            problem = "Not set up yet"
            return onChange()
        }
        guard let (id, key) = identity() else {
            // identity() says why when the key is there but unreadable
            if problem != PortableOffice.keychainProblem { problem = "Couldn't keep the viewer's key in the Keychain" }
            return onChange()
        }
        credentials = (base, id, key, secret)
        lock.lock(); forgotten[id] = nil; lock.unlock()
        run += 1
        isOn = true
        since = Date()
        lastSent = nil
        problem = nil
        failures = 0
        inFlight = false
        pending = false
        heartbeatTimer = Timer.scheduledTimer(withTimeInterval: PortableOffice.heartbeat, repeats: true) { [weak self] _ in self?.beat() }
        watchForReturn()
        upload()
        onChange()
    }

    /// Stops sending and asks the viewer to forget the office. It's off even
    /// if that fails: the viewer forgets it by itself after 10 minutes.
    /// `wait` is how long to wait for the viewer's reply, on quit.
    func stop(waitingUpTo wait: TimeInterval = 0) {
        guard isOn else { return }
        let c = credentials
        isOn = false
        run += 1
        credentials = nil
        since = nil
        problem = nil
        inFlight = false
        pending = false
        [debounceTimer, retryTimer, heartbeatTimer, activityTimer, graceTimer].forEach { $0?.invalidate() }
        debounceTimer = nil; retryTimer = nil; heartbeatTimer = nil; activityTimer = nil; graceTimer = nil
        watchers.forEach { $0.0.removeObserver($0.1) }
        watchers = []
        onChange()
        guard let c else { return }
        let stoppedRun = run
        forget(c.id, at: c.base, secret: c.secret) { [weak self] status in
            guard !PortableOffice.isSuccess(status) else { return }
            DispatchQueue.main.async {
                guard let self, self.run == stoppedRun else { return }
                self.problem = "Stopped, but can't reach the viewer: it forgets the office within 10 minutes"
                self.onChange()
            }
        }
        // On quit, also wait for an upload still out and the DELETE sent after it.
        if wait > 0 { _ = outstanding.wait(timeout: .now() + wait) }
    }

    /// Asks the viewer to forget `id`, and again after each PUT to it that's still out.
    private func forget(_ id: String, at base: String, secret: String, then done: @escaping (Int?) -> Void = { _ in }) {
        lock.lock()
        if putsOut[id, default: 0] > 0 { forgotten[id] = (base, secret) }
        lock.unlock()
        outstanding.enter()
        PortableOffice.send("DELETE", to: "\(base)/api/s/\(id)", secret: secret) { [outstanding] status in
            done(status)
            outstanding.leave()
        }
    }

    /// A PUT to `id` is done (or gave up), on the session's queue.
    private func putFinished(_ id: String) {
        lock.lock()
        let left = putsOut[id, default: 1] - 1
        putsOut[id] = left > 0 ? left : nil
        let again = forgotten[id]
        if left <= 0 { forgotten[id] = nil }
        lock.unlock()
        if let again {
            outstanding.enter()
            PortableOffice.send("DELETE", to: "\(again.base)/api/s/\(id)", secret: again.secret) { [outstanding] _ in
                outstanding.leave()
            }
        }
        outstanding.leave()
    }

    /// The deck changed: send it within 5 s, the newest each time.
    func changed() {
        guard isOn else { return }
        pending = true
        guard debounceTimer == nil, retryTimer == nil, !inFlight else { return }
        scheduleUpload()
    }

    private func scheduleUpload() {
        let since = lastAttempt.map { Date().timeIntervalSince($0) } ?? PortableOffice.debounce
        let delay = max(1, PortableOffice.debounce - since)
        debounceTimer = Timer.scheduledTimer(withTimeInterval: delay, repeats: false) { [weak self] _ in
            self?.debounceTimer = nil
            self?.upload()
        }
    }

    private func beat() {
        guard isOn, !stopIfOverdue() else { return }
        guard !inFlight, retryTimer == nil, debounceTimer == nil else { return }
        if let lastAttempt, Date().timeIntervalSince(lastAttempt) < PortableOffice.heartbeat - 5 { return }
        upload()
    }

    /// After N hours it turns itself off.
    @discardableResult
    private func stopIfOverdue() -> Bool {
        let hours = Config.shared.viewer.stopAfterHours
        guard hours > 0, let since, Date().timeIntervalSince(since) >= TimeInterval(hours) * 3600 else { return false }
        stop()
        return true
    }

    private func alias(_ id: String) -> String {
        if let known = aliases[id] { return known }
        let made = "v\(aliases.count + 1)"
        aliases[id] = made
        return made
    }

    private func upload() {
        debounceTimer?.invalidate()
        debounceTimer = nil
        guard isOn, let c = credentials, !stopIfOverdue() else { return }
        if inFlight { pending = true; return }
        pending = false
        lastAttempt = Date()
        let office = snapshot(Config.shared.viewer.hideNames, alias)
        guard let plain = ViewerCrypto.plaintext(office: office, at: Int64(Date().timeIntervalSince1970 * 1000)) else {
            return failed("The office is too big to send")
        }
        guard let body = try? ViewerCrypto.seal(plain, key: c.key, id: c.id) else { return failed("Couldn't lock the snapshot") }
        inFlight = true
        let thisRun = run
        let id = c.id
        lock.lock(); putsOut[id, default: 0] += 1; lock.unlock()
        outstanding.enter()
        PortableOffice.send("PUT", to: "\(c.base)/api/s/\(id)", secret: c.secret, body: body) { [weak self] status in
            self?.putFinished(id)
            DispatchQueue.main.async {
                guard let self, self.run == thisRun, self.isOn else { return }
                self.inFlight = false
                if PortableOffice.isSuccess(status) {
                    self.lastSent = Date()
                    self.problem = nil
                    self.failures = 0
                    if self.pending { self.scheduleUpload() }
                    self.onChange()
                } else {
                    self.failed(PortableOffice.words(for: status))
                }
            }
        }
    }

    /// It stays on, and tries again after 5, 10, 30, then every 60 s.
    private func failed(_ words: String) {
        problem = words
        failures += 1
        retryTimer?.invalidate()
        let delay = PortableOffice.backoff[min(failures - 1, PortableOffice.backoff.count - 1)]
        retryTimer = Timer.scheduledTimer(withTimeInterval: delay, repeats: false) { [weak self] _ in
            self?.retryTimer = nil
            self?.upload()
        }
        onChange()
    }

    // MARK: When you're back

    private func watchForReturn() {
        let distributed = DistributedNotificationCenter.default()
        let unlocked = distributed.addObserver(forName: Notification.Name("com.apple.screenIsUnlocked"), object: nil, queue: .main) { [weak self] _ in
            self?.cameBack()
        }
        watchers.append((distributed, unlocked))
        let workspace = NSWorkspace.shared.notificationCenter
        for name in [NSWorkspace.didWakeNotification, NSWorkspace.screensDidWakeNotification] {
            let woke = workspace.addObserver(forName: name, object: nil, queue: .main) { [weak self] _ in self?.watchActivity() }
            watchers.append((workspace, woke))
        }
    }

    private func cameBack() {
        guard isOn, Config.shared.viewer.stopOnReturn, let since else { return }
        let early = PortableOffice.returnGrace - Date().timeIntervalSince(since)
        guard early > 0 else { return stop() }
        // Back this soon may be launch, then lock: looked at again once the grace is
        // over, when you're back if the screen is unlocked and used since this unlock.
        let unlocked = Date()
        graceTimer?.invalidate()
        graceTimer = Timer.scheduledTimer(withTimeInterval: early, repeats: false) { [weak self] _ in
            guard let self, self.isOn, !PortableOffice.screenIsLocked else { return }
            if PortableOffice.idleSeconds < Date().timeIntervalSince(unlocked) { self.stop() }
        }
    }

    /// After a wake, you're back once the keyboard or mouse is used on an
    /// unlocked screen. A locked one waits for its unlock instead.
    private func watchActivity() {
        activityTimer?.invalidate()
        let woke = Date()
        activityTimer = Timer.scheduledTimer(withTimeInterval: 5, repeats: true) { [weak self] timer in
            guard let self, self.isOn, Date().timeIntervalSince(woke) < PortableOffice.wakeWatch else {
                timer.invalidate()
                return
            }
            if PortableOffice.idleSeconds < 5 && !PortableOffice.screenIsLocked { self.cameBack() }
        }
    }

    /// Seconds since the keyboard or mouse was last used.
    private static var idleSeconds: TimeInterval {
        guard let anyInput = CGEventType(rawValue: ~0) else { return .infinity }
        return CGEventSource.secondsSinceLastEventType(.combinedSessionState, eventType: anyInput)
    }

    private static var screenIsLocked: Bool {
        guard let session = CGSessionCopyCurrentDictionary() as? [String: Any] else { return false }
        return (session["CGSSessionScreenIsLocked"] as? Bool) ?? false
    }

    // MARK: Setup

    static let keychainProblem = "Can't read the viewer's key from the Keychain"

    /// The viewer's id and key, made the first time: only when there's
    /// neither. One that's there but can't be read (a denied prompt after
    /// the app is re-signed, a damaged item) is never overwritten, as every
    /// link given out would stop working without a word.
    private func identity() -> (String, Data)? {
        if let id = ViewerKeychain.read(.id), ViewerCrypto.isID(id),
           let text = ViewerKeychain.read(.key), let key = ViewerCrypto.data(base64URL: text), key.count == 32 {
            return (id, key)
        }
        guard ViewerKeychain.absent(.id), ViewerKeychain.absent(.key) else {
            problem = PortableOffice.keychainProblem
            return nil
        }
        return newIdentity()
    }

    private func newIdentity() -> (String, Data)? {
        guard let idBytes = ViewerCrypto.random(16), let key = ViewerCrypto.random(32) else { return nil }
        let id = ViewerCrypto.base64URL(idBytes)
        guard ViewerKeychain.write(id, for: .id), ViewerKeychain.write(ViewerCrypto.base64URL(key), for: .key) else { return nil }
        linkRevision += 1
        return (id, key)
    }

    /// A new write secret, kept in the Keychain and returned this once, to paste into Vercel.
    func makeWriteSecret() -> String? {
        guard let bytes = ViewerCrypto.random(32) else { return nil }
        let secret = ViewerCrypto.base64URL(bytes)
        guard ViewerKeychain.write(secret, for: .writeSecret) else { return nil }
        secretKnown = true
        _ = identity()
        if let c = credentials { credentials = (c.base, c.id, c.key, secret) }
        checked = nil
        return secret
    }

    /// A new id and key, so every link given out stops working; the old id is forgotten on the viewer.
    func rotateKey() {
        let old = credentials.map { ($0.base, $0.id, $0.secret) }
            ?? origin.flatMap { base in ViewerKeychain.read(.id).flatMap { id in ViewerKeychain.read(.writeSecret).map { (base, id, $0) } } }
        guard let (id, key) = newIdentity() else {
            problem = "Couldn't make a new key in the Keychain"
            return onChange()
        }
        checked = nil
        if let (base, oldID, secret) = old, ViewerCrypto.isID(oldID) {
            let rotatedAt = linkRevision
            forget(oldID, at: base, secret: secret) { [weak self] status in
                guard !PortableOffice.isSuccess(status) else { return }
                DispatchQueue.main.async {
                    // Unless it has been rotated again since
                    guard let self, self.linkRevision == rotatedAt else { return }
                    let words = "Rotated, but the viewer couldn't be reached: old links show the last frame for up to 10 minutes"
                    self.problem = words
                    self.checked = words
                    self.onChange()
                }
            }
        }
        if let c = credentials {
            credentials = (c.base, id, key, c.secret)
            run += 1
            inFlight = false
            upload()
        }
        onChange()
    }

    /// The viewer forgets every office it holds. This one is turned off first.
    func wipe(done: @escaping () -> Void) {
        stop()
        guard let base = origin, let secret = ViewerKeychain.read(.writeSecret) else {
            checked = "Not set up yet"
            return done()
        }
        PortableOffice.send("DELETE", to: "\(base)/api/s", secret: secret) { [weak self] status in
            DispatchQueue.main.async {
                self?.checked = PortableOffice.isSuccess(status) ? "Viewer wiped ✓" : PortableOffice.words(for: status)
                done()
            }
        }
    }

    /// Asks the viewer whether it's ready: its settings, its storage and the write secret.
    func check(done: @escaping () -> Void) {
        guard let base = origin, let secret = ViewerKeychain.read(.writeSecret) else {
            checked = "Not set up yet"
            return done()
        }
        PortableOffice.send("GET", to: "\(base)/api/health", secret: secret) { [weak self] status in
            DispatchQueue.main.async {
                self?.checked = PortableOffice.isSuccess(status) ? "Viewer ready ✓" : PortableOffice.words(for: status)
                done()
            }
        }
    }

    // MARK: Network

    /// A redirect isn't followed: it could take the write secret elsewhere.
    private static let session: URLSession = {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.timeoutIntervalForRequest = timeout
        configuration.timeoutIntervalForResource = timeout
        configuration.urlCache = nil
        configuration.httpCookieStorage = nil
        return URLSession(configuration: configuration, delegate: NoRedirects(), delegateQueue: nil)
    }()

    private final class NoRedirects: NSObject, URLSessionTaskDelegate {
        func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse,
                        newRequest request: URLRequest, completionHandler: @escaping (URLRequest?) -> Void) {
            completionHandler(nil)
        }
    }

    /// The reply's status, or nil if there was none, on the session's own queue. The body is never read.
    private static func send(_ method: String, to address: String, secret: String, body: Data? = nil,
                             done: @escaping (Int?) -> Void) {
        guard let url = URL(string: address) else { return done(nil) }
        var request = URLRequest(url: url, timeoutInterval: timeout)
        request.httpMethod = method
        request.setValue("Bearer \(secret)", forHTTPHeaderField: "Authorization")
        request.setValue("Smolcorp", forHTTPHeaderField: "User-Agent")
        if let body {
            request.httpBody = body
            request.setValue("application/octet-stream", forHTTPHeaderField: "Content-Type")
        }
        session.dataTask(with: request) { _, response, _ in
            done((response as? HTTPURLResponse)?.statusCode)
        }.resume()
    }

    private static func isSuccess(_ status: Int?) -> Bool { status.map { (200..<300).contains($0) } ?? false }

    /// What a failed reply means, in a few plain words: never its body or the address.
    static func words(for status: Int?) -> String {
        switch status {
        case nil: return "Can't reach the viewer"
        case 401?: return "The viewer refused the write secret"
        case 503?: return "The viewer isn't set up on Vercel yet"
        case 404?, 405?: return "No viewer at that address"
        case 413?: return "The office is too big to send"
        case 429?: return "The viewer is busy, trying again"
        case let code?: return "The viewer said no (\(code))"
        }
    }
}

// MARK: Self-check (debug builds only)

#if DEBUG
/// `swift run AgentDeck --viewer-selftest [vector.bin]`: seals the spec's
/// test vector and compares it with the file, then checks a sample deck's
/// redaction. Touches no Keychain, no network and no port.
enum ViewerSelfTest {
    static func run(arguments: [String]) -> Int32 {
        var ok = true
        let path = arguments.first { $0.hasSuffix(".bin") } ?? ""
        let id = "AAECAwQFBgcICQoLDA0ODw"
        let key = ViewerCrypto.data(base64URL: "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8")!
        let nonce = try! AES.GCM.Nonce(data: Data((0xa0...0xab).map { UInt8($0) }))
        let plain = ViewerCrypto.padded(Data(#"{"v":1,"at":1792000000000,"office":{"keys":[],"overflow":0}}"#.utf8))!
        let body = try! ViewerCrypto.seal(plain, key: key, id: id, nonce: nonce)
        let hex = { (data: Data) in data.map { String(format: "%02x", $0) }.joined() }
        let digest = hex(Data(SHA256.hash(data: body)))
        print("vector: plaintext \(plain.count) bytes, body \(body.count) bytes")
        print("vector: first 32 \(hex(body.prefix(32)))")
        print("vector: tag      \(hex(body.suffix(16)))")
        print("vector: sha256   \(digest)")
        let expected = "aeb857fedacfd942ddda9f7b1824d43230bd580bf0f1ea25ea35972a88798327"
        if let file = FileManager.default.contents(atPath: path) {
            let match = file == body && digest == expected
            print("vector match: \(match ? "yes" : "no") (against \(path))")
            ok = ok && match
        } else {
            print("vector match: \(digest == expected ? "yes" : "no") (against the spec's SHA-256)")
            ok = ok && digest == expected
        }

        for hide in [false, true] {
            let problems = redactionProblems(hideNames: hide)
            print("redaction (hide names \(hide ? "on" : "off")): \(problems.isEmpty ? "yes" : "no")")
            problems.forEach { print("  - \($0)") }
            ok = ok && problems.isEmpty
        }
        return ok ? 0 : 1
    }

    /// A deck with something sensitive in every field that must not leave.
    static func redactionProblems(hideNames: Bool) -> [String] {
        let session = Session(id: "real-session-id-1234", agent: .claude, folder: "/Users/someone/secret-folder",
                              project: "/Users/someone/secret-project", startedAt: 1)
        session.state = .asking
        session.effort = "high"
        session.tool = "Bash rm -rf /tmp/secret-tool"
        session.toolActivity = "secret-activity"
        session.waitingFor = "secret-waiting"
        session.model = "claude-secret-model"
        session.messaged = "secret-message"
        session.messagedAt = 5
        session.maxPressedAt = 6
        session.transcript = "/Users/someone/.claude/secret-transcript.jsonl"
        session.errorKind = "secret-error-kind"
        session.subagents = [Session.Subagent(id: "real-subagent-id", type: "Explore", model: "claude-secret-submodel"),
                             Session.Subagent(id: "real-subagent-id-2", type: "secret-agent-type", model: "")]
        var role = Role.shipped[0]
        role.instructions = "secret-instructions"
        role.questions = ["secret-question"]
        // A role you made: its id and name are your own text.
        let ownRole = Role(id: "secret-role-id", name: "secret-role-name", agent: "claude", model: "", effort: "",
                           instructions: "secret-instructions", badge: role.badge)
        let other = Session(id: "real-session-id-5678", agent: .claude, folder: "/Users/someone/second-secret-folder",
                            project: "/Users/someone/second-secret-project", startedAt: 1)
        var layout = Layout.standard
        // The room as the setup page keeps it, with every part that holds your text filled: the team's
        // nicknames and projects, a dress rule (part of a model's name), and a plan with added pieces.
        layout.room = try! JSONDecoder().decode(Room.self, from: Data(#"""
            {"team": {"secret-folder": {"name": "secret-nickname", "outfit": "suit"},
                      "second-secret-folder": {"name": "other-nickname"}, "other-secret-project": {}},
             "dress": {"rules": [{"contains": "secret-rule", "size": "top"}]},
             "plan": {"wide": {"desk1": {"x": 4, "thing": "duck"}}, "added": {"wide": [{"kind": "piano", "x": 3}]},
                      "hung": {"tall": [{"kind": "map", "x": 40}]}}}
            """#.utf8))
        layout.rows = [ButtonRow(tall: false, buttons: [Button(id: "secret-button")])]
        var aliases: [String: String] = [:]
        var office = ViewerSnapshot.build(
            desks: [.init(pin: "/Users/someone/pinned-secret", session: nil, role: nil),
                    .init(pin: nil, session: session, role: role),
                    .init(pin: nil, session: other, role: ownRole)],
            overflow: 0, theme: ["idle": "#f2f2f2"], music: (true, true), layout: layout, hideNames: hideNames, now: 1,
            alias: { id in aliases[id] ?? { aliases[id] = "v\(aliases.count + 1)"; return aliases[id]! }() })
        let hiredHere = Session(id: "real-session-id-up-1", agent: .claude, folder: "/Users/someone/up-folder",
                                project: "/Users/someone/up-folder", startedAt: 1)
        hiredHere.parentFolder = session.project
        let hiredElsewhere = Session(id: "real-session-id-up-2", agent: .claude, folder: "/Users/someone/up-folder-2",
                                     project: "/Users/someone/up-folder-2", startedAt: 1)
        hiredElsewhere.parentFolder = "/Users/someone/secret-hirer"
        let all = [session, other, hiredHere, hiredElsewhere]
        office["upstairs"] = ViewerSnapshot.upstairs([hiredHere, hiredElsewhere], overflow: 1, hideNames: hideNames) { s in
            ViewerSnapshot.hirer(Deck.hiredBy(s, among: all), desks: [nil, session, other], hideNames: hideNames)
        }
        guard let plain = ViewerCrypto.plaintext(office: office, at: 1), let text = String(data: plain, encoding: .utf8) else {
            return ["couldn't serialise the snapshot"]
        }
        var problems: [String] = []
        let forbiddenKeys = ["activity", "tool", "messaged", "messagedAt", "maxPressedAt", "voiceNote", "instructions",
                             "questions", "transcript", "folder", "tab", "tty", "secret", "token", "phone", "summaryText"]
        for key in forbiddenKeys where text.contains("\"\(key)\":") { problems.append("key \(key)") }
        var forbiddenText = ["/Users", "real-session-id", "real-subagent-id", "secret-tool", "secret-activity", "secret-waiting",
                             "secret-model", "secret-submodel", "secret-message", "secret-transcript", "secret-instructions",
                             "secret-question", "secret-button", "other-secret-project", "secret-rule"]
        if hideNames {
            forbiddenText += ["secret-folder", "secret-project", "secret-nickname", "other-nickname", "pinned-secret", "secret-hirer", "up-folder",
                              "secret-error-kind", "secret-agent-type", "secret-role-id", "secret-role-name"]
        }
        for needle in forbiddenText where text.contains(needle) { problems.append("text \(needle)") }
        // The session's own model key must be gone; subagents keep theirs as null.
        if let keys = office["keys"] as? [[String: Any]], let s = keys.last?["session"] as? [String: Any] {
            if s["model"] != nil { problems.append("session.model present") }
            for control in ["dial", "mic", "dictates", "keys", "fire", "summary"] where s[control] as? Bool != false {
                problems.append("control \(control) not false")
            }
        } else {
            problems.append("no session in the snapshot")
        }
        if ((office["layout"] as? [String: Any])?["rows"] as? [Any])?.isEmpty != true { problems.append("layout.rows not empty") }
        // Who hired each run upstairs: by name, or with names hidden, the hiring desk's number or "Downstairs".
        let hirers = ((office["upstairs"] as? [String: Any])?["desks"] as? [[String: Any]] ?? []).map { ($0["hiredBy"] as? [String: String])?["name"] }
        let wanted: [String?] = hideNames ? ["Desk 2", "Downstairs"] : ["secret-project", "secret-hirer"]
        if hirers != wanted { problems.append("upstairs hiredBy \(hirers), not \(wanted)") }
        // The raw room: with names hidden, every text in it is one of the room's own choices, and its
        // team is under the desks' names. Whatever is added to Room later and holds your text fails here.
        guard let room = (office["layout"] as? [String: Any])?["room"] as? [String: Any] else {
            return problems + ["no layout.room in the snapshot"]
        }
        if (room["plan"] as? [String: Any])?["added"] == nil { problems.append("room.plan.added missing") }
        if hideNames {
            for key in (room["team"] as? [String: Any] ?? [:]).keys where key.range(of: #"^Desk \d+$"#, options: .regularExpression) == nil {
                problems.append("room.team key \(key)")
            }
            var known: Set<String> = ["", "auto", "none", "bare"]
            Room.choices.values.forEach { known.formUnion($0) }
            TeamMember.choices.values.forEach { known.formUnion($0) }
            known.formUnion(Room.wallChoices + Room.pets + Role.badges)
            func texts(_ value: Any, at path: String) {
                if let text = value as? String, !known.contains(text) { problems.append("room text at \(path)") }
                if let list = value as? [Any] { list.enumerated().forEach { texts($1, at: "\(path)[\($0)]") } }
                if let object = value as? [String: Any] { object.forEach { texts($1, at: "\(path).\($0)") } }
            }
            texts(room, at: "room")
        }
        return problems
    }
}
#endif
