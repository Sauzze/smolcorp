import Foundation
import Security

/// Who is asking the setup routes: the page the menu opened, a tool of
/// your own with the setup-key file, or the General Manager with its token.
/// A tool can read and change settings; only the page may pair a phone or
/// see a phone's key, so nothing that can read the file (a hire, any program
/// running as you) can pair a fake phone and answer a prompt with it. What a
/// hire could misuse with the file wants the page or the GM's token: see
/// `SetupRoutes`.
///
/// The menu's link carries a key that works once, for a few minutes, kept
/// only in memory: the page hands it over for a session cookie, and the link
/// left in the browser's history opens nothing after that. The setup-key
/// file's key never makes a session, and isn't taken as a cookie.
final class SetupSessions {
    enum Caller: Equatable {
        /// The setup page, by its session cookie.
        case page
        /// A tool, by the setup-key file's key in its header.
        case tool
        /// The General Manager, by its token in its header (`GMIdentity`).
        case gm
        /// A hire the app started, by the send key in its own environment (`FrontDesk.hire(forSendKey:)`).
        /// It may send mail and nothing else.
        case hire
        case nobody
    }

    /// How long a menu link's key works, if it isn't used.
    static let linkLifetime: TimeInterval = 300
    /// The most links not yet opened, and page sessions, kept.
    static let most = 20

    private var links: [String: Date] = [:]
    private var sessions: [String] = []
    private let lock = NSLock()

    /// A key for one menu link.
    func newLink(now: Date = Date()) -> String {
        lock.lock(); defer { lock.unlock() }
        links = links.filter { $0.value > now }
        if links.count >= SetupSessions.most, let oldest = links.min(by: { $0.value < $1.value })?.key { links[oldest] = nil }
        let key = SetupSessions.random()
        links[key] = now.addingTimeInterval(SetupSessions.linkLifetime)
        return key
    }

    /// A session for the page that brings a link's key, once; nil for any other key.
    func open(link: String?, now: Date = Date()) -> String? {
        lock.lock(); defer { lock.unlock() }
        guard let link, !link.isEmpty, let match = links.keys.first(where: { constantTimeEqual($0, link) }) else { return nil }
        let expiry = links.removeValue(forKey: match)
        guard let expiry, expiry > now else { return nil }
        let session = SetupSessions.random()
        sessions.append(session)
        if sessions.count > SetupSessions.most { sessions.removeFirst(sessions.count - SetupSessions.most) }
        return session
    }

    func isSession(_ candidate: String?) -> Bool {
        lock.lock(); defer { lock.unlock() }
        guard let candidate, !candidate.isEmpty else { return false }
        return sessions.contains { constantTimeEqual($0, candidate) }
    }

    /// Who a request is, from its setup-key header, its GM-token header and
    /// its cookie. A key in the header is the file's, never a page's; a
    /// cookie is a page session, never the file's key. The GM's token is only
    /// the GM's: the setup key never passes for it, nor it for the setup key.
    /// A request is one caller, in a fixed order: page, then a hire's key, then a valid
    /// setup key (a tool), then the GM's token. So a request with a
    /// good key and a token is the owner's tool, not the GM; a wrong one of the
    /// two never changes what the other gets alone.
    func caller(header: String?, cookie: String?, isSetupKey: (String?) -> Bool,
                gmHeader: String? = nil, isGMToken: (String?) -> Bool = { _ in false },
                hireKey: String? = nil, isHireKey: (String?) -> Bool = { _ in false }) -> Caller {
        if isSession(cookie) { return .page }
        if isHireKey(hireKey) { return .hire }
        if isSetupKey(header) { return .tool }
        if isGMToken(gmHeader) { return .gm }
        return .nobody
    }

    /// The routes only the page may use: they show or make a phone's code or
    /// key. (A fresh code also lifts the lockout, so a tool asking again and
    /// again could guess one.) Taking a phone away is the page's or the
    /// GM's: see `SetupRoutes`, which holds them all.
    static func isPageOnly(_ method: String, _ path: String) -> Bool {
        SetupRoutes.row(method, path)?.callers == [.page]
    }

    /// The pairing part of the setup state: the code only for the page;
    /// for a tool, whether pairing is open and nothing more.
    static func pairing(code: String?, expiresIn: Int, address: String, for caller: Caller) -> [String: Any] {
        var pairing: [String: Any] = ["address": address, "open": code != nil]
        if caller == .page {
            pairing["codeExpiresIn"] = code == nil ? 0 : expiresIn
            if let code { pairing["code"] = code }
        }
        return pairing
    }

    static func random() -> String {
        var bytes = [UInt8](repeating: 0, count: 32)
        _ = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        return bytes.map { String(format: "%02x", $0) }.joined()
    }
}
