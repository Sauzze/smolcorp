import CryptoKit
import Foundation

/// A phone you paired, as phones.json keeps it.
struct PairedPhone: Codable {
    /// The office's own window on this Mac, as the deck sees it: a phone that
    /// isn't paired, saved or listed. See `Deck.windowKey`.
    static let officeWindow = PairedPhone(id: "office-window", name: "This Mac's office window", pairedAt: Date(), lastSeen: Date(),
                                          keyHash: "")

    let id: String
    let name: String
    let pairedAt: Date
    var lastSeen: Date
    /// SHA-256 of the phone's key, in hex. The key itself is never stored.
    let keyHash: String
    /// iPhone app (personal build): where its notifications go, and which it wants. See `Pushes`.
    var push: PushTarget? = nil
}

/// Every paired phone. Each has its own key, so one can be revoked and the
/// others keep working. The phone keeps its key in the deck's URL; this Mac
/// keeps only a hash of it, so the file can't be used to pose as a phone.
final class Phones {
    /// How out of date `lastSeen` may get before a request updates it.
    /// Saving on every request would rewrite the file many times a second.
    private static let seenResolution: TimeInterval = 60

    private(set) var all: [PairedPhone]
    /// phones.json; another file in tests.
    private let file: URL

    init(file: URL = Paths.phones) {
        self.file = file
        all = Phones.load(file)
    }

    /// Adds a phone and returns its key: the only time the key exists here.
    func pair(name: String) -> String {
        let key = randomKey()
        let now = Date()
        all.append(PairedPhone(id: UUID().uuidString, name: name, pairedAt: now, lastSeen: now,
                               keyHash: Phones.hash(key)))
        save()
        return key
    }

    /// The phone holding `key`, if any.
    func phone(withKey key: String?) -> PairedPhone? {
        guard let key else { return nil }
        let hash = Phones.hash(key)
        return all.first { constantTimeEqual($0.keyHash, hash) }
    }

    func markSeen(_ id: String) {
        guard let index = all.firstIndex(where: { $0.id == id }),
              -all[index].lastSeen.timeIntervalSinceNow >= Phones.seenResolution else { return }
        all[index].lastSeen = Date()
        save()
    }

    /// iPhone app (personal build): the phone's device token and the notifications it wants, or nil for none.
    func setPush(_ push: PushTarget?, for id: String) {
        guard let index = all.firstIndex(where: { $0.id == id }), all[index].push != push else { return }
        // One phone, one token: one handed on to a phone paired again is the newest pairing's only.
        if let token = push?.token {
            for other in all.indices where other != index && all[other].push?.token == token { all[other].push = nil }
        }
        all[index].push = push
        save()
    }

    /// A token Apple no longer takes, forgotten: only if it's still the phone's, as a new one may have come since.
    func forgetPush(token: String, for id: String) {
        guard let push = all.first(where: { $0.id == id })?.push, push.token == token else { return }
        setPush(nil, for: id)
    }

    func remove(_ id: String) {
        all.removeAll { $0.id == id }
        save()
    }

    func removeAll() {
        all = []
        save()
    }

    /// What the setup page calls a phone. The User-Agent is all there is to
    /// go on, and a broad name is all it can honestly give.
    static func name(forUserAgent userAgent: String) -> String {
        // Agent Office pairs like a phone. Its app is named with a space, so
        // the system may write it either way.
        if userAgent.contains("AgentOffice") || userAgent.contains("Agent%20Office") { return "Agent Office" }
        // So does the screen saver, from its Options: see ScreenSaver/Pairing.swift.
        if userAgent.contains("SauzzeDeckScreenSaver") { return "Screen saver" }
        // Personal build: the iPhone app, which says so: see ios/.
        if userAgent.hasPrefix(PhoneApp.userAgent) { return "iPhone app" }
        if userAgent.contains("iPhone") { return "iPhone" }
        if userAgent.contains("iPad") { return "iPad" }
        if userAgent.contains("Android") { return "Android phone" }
        return "Phone"
    }

    private static func hash(_ key: String) -> String {
        SHA256.hash(data: Data(key.utf8)).map { String(format: "%02x", $0) }.joined()
    }

    private static func load(_ file: URL) -> [PairedPhone] {
        guard let data = try? Data(contentsOf: file) else { return [] }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return (try? decoder.decode([PairedPhone].self, from: data)) ?? []
    }

    private func save() {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        encoder.dateEncodingStrategy = .iso8601
        guard let data = try? encoder.encode(all) else { return }
        // Whole or not at all: a file cut short would unpair every phone.
        Paths.writePrivately(data, to: file)
    }
}
