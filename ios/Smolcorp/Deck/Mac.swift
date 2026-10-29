import Foundation

/// The Mac this phone is paired with: where it can be reached, and its key,
/// which only the Keychain holds.
struct Mac: Codable, Equatable {
    /// What setup calls it, or its Bonjour name.
    var name: String
    /// On the Wi-Fi, plain http: its local name or address, and the deck's port.
    var lan: URL
    /// Away from home, through Tailscale's HTTPS (`tailscale serve`).
    var tailnet: URL?
    /// SHA-256 of a certificate to trust by pin, by host: the mock deck's in
    /// development. The real routes use the system's trust.
    var pins: [String: String] = [:]

    /// The Mac offered first when pairing, as the build was given it
    /// (SMOLCORP_MAC_LAN, SMOLCORP_MAC_TAILNET): nil unless it was.
    static let defaultLAN = built("SmolcorpMacLAN")
    static let defaultTailnet = built("SmolcorpMacTailnet")

    /// An address the build put in Info.plist, or nil where it's blank.
    private static func built(_ key: String) -> URL? {
        guard let text = Bundle.main.object(forInfoDictionaryKey: key) as? String, text.contains("://") else { return nil }
        return URL(string: text.trimmingCharacters(in: .whitespaces))
    }

    private static let saved = "mac"
    private static let keyAccount = "mac-key"

    static func load() -> (mac: Mac, key: String)? {
        guard let data = UserDefaults.standard.data(forKey: saved), let mac = try? JSONDecoder().decode(Mac.self, from: data),
              let key = Keychain.read(keyAccount).map({ String(decoding: $0, as: UTF8.self) }), !key.isEmpty else { return nil }
        return (mac, key)
    }

    func save(key: String) {
        UserDefaults.standard.set(try? JSONEncoder().encode(self), forKey: Mac.saved)
        Keychain.write(Data(key.utf8), for: Mac.keyAccount)
    }

    /// Unpaired: the key goes; where the Mac was is kept, to pair again.
    static func forgetKey() {
        Keychain.remove(keyAccount)
    }

    static var lastKnown: Mac? {
        UserDefaults.standard.data(forKey: saved).flatMap { try? JSONDecoder().decode(Mac.self, from: $0) }
    }

    /// Development: the mock deck, from launch arguments (see ios/scripts/mock-deck.py).
    ///   -devLAN http://192.168.x.y:7791 -devTailnet https://192.168.x.y:7792 -devPin <sha256>
    static var development: Mac? {
        let defaults = UserDefaults.standard
        guard let lan = defaults.string(forKey: "devLAN").flatMap(URL.init(string:)) else { return nil }
        let tailnet = defaults.string(forKey: "devTailnet").flatMap(URL.init(string:))
        var pins: [String: String] = [:]
        if let pin = defaults.string(forKey: "devPin"), let host = tailnet?.host { pins[host.lowercased()] = pin.lowercased() }
        return Mac(name: "Mock deck", lan: lan, tailnet: tailnet, pins: pins)
    }
}
