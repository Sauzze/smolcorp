import CryptoKit
import Foundation
import Security

/// iPhone app: notifications on the phone, sent by this Mac
/// itself to Apple's push service, as an agent finishes its turn or needs you,
/// or as the GM session says. Off, and everything else as before, until the
/// push key is in the Keychain.
///
/// What goes through Apple is only a desk's name and "finished" or "needs
/// you", or the GM's own short message, and the session's id for the tap to
/// open: never anything an agent or the user wrote.

/// What a phone asked to be told, and where: its device token, kept beside
/// its pairing in phones.json.
struct PushTarget: Codable, Equatable {
    /// The device token, in hex.
    var token: String
    /// A development build's token, for Apple's sandbox.
    var sandbox = true
    /// (a) an agent finished its turn, (b) one needs you, (c) the GM's messages.
    var finished = true
    var waiting = true
    var gm = true

    func wants(_ event: PushEvent) -> Bool {
        switch event {
        case .finished: return finished
        case .waiting: return waiting
        case .gm: return gm
        }
    }

    /// Hex, as iOS gives it: 32 bytes today, up to 100 allowed.
    static func isToken(_ text: String) -> Bool {
        (64...200).contains(text.count) && text.count % 2 == 0 && text.allSatisfy { $0.isHexDigit && ($0.isNumber || $0.isLowercase) }
    }
}

enum PushEvent: String { case finished, waiting, gm }

/// One notification: what it's about, and its only words.
struct PushNote: Equatable {
    let event: PushEvent
    /// The session a tap opens, if any.
    let session: String?
    let title: String
    let body: String

    static let titleMost = 60
    static let bodyMost = 180

    /// An agent's: its desk's name, and what happened, nothing more.
    static func desk(_ name: String, _ event: PushEvent, session: String) -> PushNote {
        PushNote(event: event, session: session, title: TranscriptTail.clip(name.isEmpty ? "Smolcorp" : name, titleMost),
                 body: event == .waiting ? "needs you" : "finished")
    }

    /// The GM's: its own short words, on one line each, redacted as Summary's are.
    static func gm(title: String, body: String) -> PushNote? {
        let line = { (text: String, most: Int) in
            TranscriptTail.clip(Redact.text(text.split(whereSeparator: \.isNewline).joined(separator: " "))
                .trimmingCharacters(in: .whitespaces), most)
        }
        let title = line(title, titleMost), body = line(body, bodyMost)
        guard !title.isEmpty || !body.isEmpty else { return nil }
        return PushNote(event: .gm, session: nil, title: title.isEmpty ? "Smolcorp" : title, body: body)
    }

    /// The notification as Apple takes it.
    var payload: Data {
        var aps: [String: Any] = ["alert": ["title": title, "body": body], "sound": "default"]
        if let session { aps["thread-id"] = session }
        var object: [String: Any] = ["aps": aps, "event": event.rawValue]
        if let session { object["session"] = session }
        return (try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])) ?? Data()
    }
}

// MARK: The token Apple takes

/// The provider token: a JWT signed with ES256 by the push key, as Apple's
/// token-based connection wants it.
enum APNsJWT {
    /// The key from its .p8 file's text, or that text's base64 body alone.
    static func privateKey(_ data: Data) -> P256.Signing.PrivateKey? {
        guard let text = String(data: data, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) else { return nil }
        if text.contains("-----BEGIN") { return try? P256.Signing.PrivateKey(pemRepresentation: text) }
        guard let der = Data(base64Encoded: text.filter { !$0.isWhitespace }) else { return nil }
        return try? P256.Signing.PrivateKey(derRepresentation: der)
    }

    static func make(key: P256.Signing.PrivateKey, keyID: String, teamID: String, issuedAt: Date) throws -> String {
        let header = try JSONSerialization.data(withJSONObject: ["alg": "ES256", "kid": keyID], options: [.sortedKeys])
        let claims = try JSONSerialization.data(withJSONObject: ["iss": teamID, "iat": Int(issuedAt.timeIntervalSince1970)],
                                                options: [.sortedKeys])
        let signed = base64URL(header) + "." + base64URL(claims)
        // ES256's signature is r and s side by side, 64 bytes: CryptoKit's raw form.
        let signature = try key.signature(for: Data(signed.utf8)).rawRepresentation
        return signed + "." + base64URL(signature)
    }

    static func base64URL(_ data: Data) -> String {
        data.base64EncodedString().replacingOccurrences(of: "+", with: "-").replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
    }
}

/// The push key, which you put in the login Keychain yourself, for the
/// iPhone app you built: service `smolcorp-apns`, account its Key ID, and as
/// its comment your Apple team ID and that app's bundle ID, a space between:
///
///     security add-generic-password -s smolcorp-apns -a KEYID -j "TEAMID com.example.smolcorp" -w
///
/// Its value is read only in `token`, to sign, and never kept, written,
/// logged or answered.
enum PushKey {
    static let service = "smolcorp-apns"

    /// Whose key it is: its Key ID, the team it signs for, and the app it pushes to.
    struct Account: Equatable {
        var keyID: String
        var teamID: String
        var topic: String
    }

    /// The key's account, from the item's attributes, without reading the key: nil without all three.
    static func account() -> Account? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service,
            kSecReturnAttributes as String: true, kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var found: CFTypeRef?
        guard SecItemCopyMatching(query as CFDictionary, &found) == errSecSuccess,
              let attributes = found as? [String: Any] else { return nil }
        return parsed(keyID: attributes[kSecAttrAccount as String] as? String ?? "",
                      comment: attributes[kSecAttrComment as String] as? String)
    }

    /// The account from a Key ID and a "TEAMID bundle.id" comment, or nil
    /// where either is missing or isn't one: a team ID is 10 capitals and digits.
    static func parsed(keyID: String, comment: String?) -> Account? {
        let parts = (comment ?? "").split(whereSeparator: \.isWhitespace).map(String.init)
        guard !keyID.isEmpty, parts.count == 2, parts[0].count == 10,
              parts[0].allSatisfy({ $0.isASCII && ($0.isUppercase || $0.isNumber) }),
              parts[1].contains("."), parts[1].allSatisfy({ $0.isASCII && ($0.isLetter || $0.isNumber || ".-".contains($0)) })
        else { return nil }
        return Account(keyID: keyID, teamID: parts[0], topic: parts[1])
    }

    /// The Key ID, if the key is there with its team and app.
    static func keyID() -> String? { account()?.keyID }

    /// The app the pushes go to, as the key's comment names it.
    static func topic() -> String? { account()?.topic }

    /// A provider token signed now with the key `keyID`, or nil without it.
    static func token(keyID: String, now: Date) -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service,
            kSecAttrAccount as String: keyID, kSecReturnData as String: true, kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var found: CFTypeRef?
        guard SecItemCopyMatching(query as CFDictionary, &found) == errSecSuccess, let data = found as? Data,
              let key = APNsJWT.privateKey(data), let team = account().flatMap({ $0.keyID == keyID ? $0.teamID : nil }) else { return nil }
        return try? APNsJWT.make(key: key, keyID: keyID, teamID: team, issuedAt: now)
    }
}

// MARK: Sending

/// What to tell the phones as sessions change, and the sending of it. On
/// the main queue, as the deck is; Keychain and network off it.
final class Pushes {
    /// The same news about the same session no sooner than this again.
    static let quiet: TimeInterval = 60
    /// Apple wants a provider token renewed between 20 and 60 minutes.
    static let tokenLife: TimeInterval = 50 * 60
    static let timeout: TimeInterval = 10

    typealias Transport = (URLRequest, @escaping (Data?, Int?, Error?) -> Void) -> Void

    /// What each session was at the last look: nil before the first, so what's running at the start isn't news.
    private var told: [String: SessionState]?
    /// When each session's news was last sent, by "<session> <event>".
    private var sent: [String: Date] = [:]
    /// The provider token, one for every push, signed at most once in `tokenLife`: kept and used on `queue` only, so
    /// pushes sent together share it. Apple refuses a new one more often than every 20 minutes.
    private var provider: (token: String, keyID: String, at: Date)?
    private let transport: Transport
    private let keyID: () -> String?
    private let topic: () -> String?
    private let sign: (String, Date) -> String?
    private let now: () -> Date
    private let queue = DispatchQueue(label: "smolcorp.push")

    /// Whether the key was there at the last look, nil before the first: kept on the main queue, looked up on
    /// `queue`. A locked or asking Keychain only holds up `queue`, never the deck and its phones.
    private var keyFound: Bool?
    /// When it was last looked up, so it's looked up again now and then: the key is put in by hand.
    private var keyLookedAt: Date?
    /// How long the last look holds before another is asked for.
    static let keyLookEvery: TimeInterval = 60

    init(transport: @escaping Transport = Pushes.urlSession, keyID: @escaping () -> String? = PushKey.keyID,
         topic: @escaping () -> String? = PushKey.topic,
         sign: @escaping (String, Date) -> String? = PushKey.token, now: @escaping () -> Date = Date.init) {
        self.transport = transport
        self.keyID = keyID
        self.topic = topic
        self.sign = sign
        self.now = now
        lookForKey()
    }

    /// Looks the key up again on `queue`, and keeps the answer on the main queue.
    func lookForKey() {
        keyLookedAt = now()
        queue.async { [weak self] in
            guard let self else { return }
            let found = keyID() != nil
            DispatchQueue.main.async { self.keyFound = found }
        }
    }

    /// One session for every push, so its HTTP/2 connection to Apple stays open between them, as Apple asks.
    private static let session: URLSession = {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.timeoutIntervalForRequest = timeout
        configuration.urlCache = nil
        return URLSession(configuration: configuration)
    }()

    static let urlSession: Transport = { request, done in
        // HTTP/2, as Apple's push service needs: URLSession speaks it to any server that offers it.
        session.dataTask(with: request) { data, response, error in
            done(data, (response as? HTTPURLResponse)?.statusCode, error)
        }.resume()
    }

    /// Whether the push key is in the Keychain, as last looked up: never the Keychain itself, which can wait on
    /// a prompt or a lock with the whole deck waiting behind it. On the main queue. An old look asks for a new one.
    var hasKey: Bool {
        if let at = keyLookedAt, now().timeIntervalSince(at) >= Pushes.keyLookEvery { lookForKey() }
        return keyFound ?? false
    }

    /// The news since the last look, from each desk's state now: an agent
    /// newly waiting on you, or newly done. `desks` are the ground floor's
    /// sessions with their names. The same news about one session goes once a minute at most.
    func news(_ desks: [(id: String, name: String, state: SessionState)]) -> [PushNote] {
        let now = now()
        defer { told = Dictionary(desks.map { ($0.id, $0.state) }) { a, _ in a } }
        guard let before = told else { return [] }
        var notes: [PushNote] = []
        for desk in desks {
            let event: PushEvent
            switch desk.state {
            case .asking where before[desk.id] != .asking: event = .waiting
            case .unread where before[desk.id] != .unread: event = .finished
            default: continue
            }
            let id = "\(desk.id) \(event.rawValue)"
            if let last = sent[id], now.timeIntervalSince(last) < Pushes.quiet { continue }
            sent[id] = now
            notes.append(.desk(desk.name, event, session: desk.id))
        }
        sent = sent.filter { now.timeIntervalSince($0.value) < Pushes.quiet }
        return notes
    }

    /// The request for `note` to the device `target`.
    static func request(_ note: PushNote, to target: PushTarget, provider: String, topic: String) -> URLRequest? {
        let host = target.sandbox ? "api.sandbox.push.apple.com" : "api.push.apple.com"
        guard PushTarget.isToken(target.token), let url = URL(string: "https://\(host)/3/device/\(target.token)") else { return nil }
        var request = URLRequest(url: url, timeoutInterval: timeout)
        request.httpMethod = "POST"
        request.httpBody = note.payload
        request.setValue("bearer \(provider)", forHTTPHeaderField: "authorization")
        request.setValue(topic, forHTTPHeaderField: "apns-topic")
        request.setValue("alert", forHTTPHeaderField: "apns-push-type")
        request.setValue("10", forHTTPHeaderField: "apns-priority")
        // Old news is no news: not delivered after an hour away.
        request.setValue(String(Int(Date().timeIntervalSince1970) + 3600), forHTTPHeaderField: "apns-expiration")
        if let session = note.session {
            // A newer one about the same desk replaces it on the phone. Apple allows 64 bytes.
            request.setValue(String("\(note.event.rawValue)-\(session)".prefix(64)), forHTTPHeaderField: "apns-collapse-id")
        }
        request.setValue("application/json", forHTTPHeaderField: "content-type")
        return request
    }

    /// Whether Apple's answer says the token is no good any more, to forget it.
    static func isDeadToken(status: Int?, body: Data?) -> Bool {
        if status == 410 { return true }
        guard status == 400, let body, let reply = try? JSONSerialization.jsonObject(with: body) as? [String: Any] else { return false }
        return ["BadDeviceToken", "DeviceTokenNotForTopic", "Unregistered"].contains(reply["reason"] as? String ?? "")
    }

    /// Whether Apple's answer says the provider token has expired, so a new one is due.
    static func isExpiredProvider(status: Int?, body: Data?) -> Bool {
        guard status == 403, let body, let reply = try? JSONSerialization.jsonObject(with: body) as? [String: Any] else { return false }
        return reply["reason"] as? String == "ExpiredProviderToken"
    }

    /// The provider token for `id`, the one kept while it's young enough, or a new one signed. On `queue`.
    private func providerToken(_ id: String, now: Date) -> String? {
        if let kept = provider, kept.keyID == id, now.timeIntervalSince(kept.at) < Pushes.tokenLife { return kept.token }
        guard let token = sign(id, now) else { return nil }
        provider = (token, id, now)
        return token
    }

    /// Sends `note` to each phone that wants it. `forget` is told, on the
    /// main queue, of a phone and its token that Apple no longer takes.
    /// `done` says how many were sent to, on the main queue.
    func send(_ note: PushNote, to phones: [PairedPhone], forget: @escaping (_ phone: String, _ token: String) -> Void = { _, _ in },
              done: @escaping (Int) -> Void = { _ in }) {
        let targets = phones.compactMap { phone in phone.push.flatMap { $0.wants(note.event) ? (phone.id, $0) : nil } }
        guard !targets.isEmpty else { return done(0) }
        let now = now()
        let transport = transport
        queue.async { [self] in
            // Here, off the main queue, a locked Keychain or its prompt holds up the pushes only.
            let id = keyID()
            DispatchQueue.main.async { self.keyFound = id != nil }
            guard let id, let topic = topic(), let token = providerToken(id, now: now) else {
                #if DEBUG
                NSLog("Smolcorp: no push sent, the push key couldn't be read (missing, or the Keychain is locked)")
                #endif
                return DispatchQueue.main.async { done(0) }
            }
            let group = DispatchGroup()
            var delivered = 0
            let lock = NSLock()
            for (phone, target) in targets {
                guard let request = Pushes.request(note, to: target, provider: token, topic: topic) else { continue }
                group.enter()
                transport(request) { [weak self] data, status, _ in
                    if status == 200 { lock.lock(); delivered += 1; lock.unlock() }
                    if Pushes.isDeadToken(status: status, body: data) { DispatchQueue.main.async { forget(phone, target.token) } }
                    // Only an expired provider token is signed again, and only if it's still the one kept.
                    if Pushes.isExpiredProvider(status: status, body: data) {
                        self?.queue.async { if self?.provider?.token == token { self?.provider = nil } }
                    }
                    group.leave()
                }
            }
            group.notify(queue: .main) { done(delivered) }
        }
    }
}
