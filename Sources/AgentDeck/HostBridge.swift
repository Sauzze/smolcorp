import Foundation
import Security

/// The Go core's way to what only macOS lets this app do (see `Engine`): a
/// Unix socket in the support folder, readable by you alone (the socket 0600,
/// its folder 0700), whose every request carries the token this launch made
/// and handed the core in its environment. A request is a line of JSON, and
/// so is its answer, one per connection.
///
/// Each action is this app's own code, as the Swift engine runs it: the
/// terminal drivers (Accessibility and AppleScript), the openers, the
/// secrets' stores, the login item. Nothing here runs a shell line: a
/// terminal is opened only on a script the core wrote in the hires folder.
/// The core's side is desktop/hostbridge.go.
final class HostBridge {
    let socket: URL
    let token: String
    let actions: Actions
    private var listener: Int32 = -1
    private let queue = DispatchQueue(label: "io.github.sauzze.agentdeck.bridge", attributes: .concurrent)

    /// The longest request read: a typed text or a secret is short.
    static let requestMost = 256 << 10
    /// The longest a socket's path can be on a Mac.
    static let pathMost = 103

    init(socket: URL, token: String = HostBridge.newToken(), actions: Actions = .mac) {
        self.socket = socket
        self.token = token
        self.actions = actions
    }

    /// 32 random bytes, as hex: new at every launch.
    static func newToken() -> String {
        var bytes = [UInt8](repeating: 0, count: 32)
        guard SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes) == errSecSuccess else {
            return (0..<4).map { _ in UUID().uuidString.replacingOccurrences(of: "-", with: "") }.joined()
        }
        return bytes.map { String(format: "%02x", $0) }.joined()
    }

    enum Failure: LocalizedError {
        case pathTooLong, notOurs, socket(String)

        var errorDescription: String? {
            switch self {
            case .pathTooLong: return "The support folder's path is too long for the engine's socket."
            case .notOurs: return "Something that isn't the engine's socket is where it goes."
            case .socket(let call): return "The engine's socket couldn't be made (\(call): \(String(cString: strerror(errno))))."
            }
        }
    }

    /// Listens on the socket, its folder made yours alone first, and a socket
    /// an earlier launch left removed. Anything else there is left alone.
    func start() throws {
        let path = socket.path
        guard path.utf8.count <= HostBridge.pathMost else { throw Failure.pathTooLong }
        let folder = socket.deletingLastPathComponent()
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: folder.path)
        var info = stat()
        if lstat(path, &info) == 0 {
            guard info.st_mode & S_IFMT == S_IFSOCK else { throw Failure.notOurs }
            unlink(path)
        }
        let fd = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { throw Failure.socket("socket") }
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        withUnsafeMutableBytes(of: &address.sun_path) { raw in
            raw.copyBytes(from: path.utf8.prefix(raw.count - 1))
        }
        // Made 0600 as it's bound, not after: never a moment others could connect.
        let mask = umask(0o177)
        let bound = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { bind(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
        }
        umask(mask)
        guard bound == 0 else {
            close(fd)
            throw Failure.socket("bind")
        }
        chmod(path, 0o600)
        guard listen(fd, 16) == 0 else {
            close(fd)
            unlink(path)
            throw Failure.socket("listen")
        }
        listener = fd
        let thread = Thread { [weak self] in self?.acceptLoop(fd) }
        thread.name = "Smolcorp bridge"
        thread.start()
    }

    /// No more requests, and the socket removed.
    func stop() {
        guard listener >= 0 else { return }
        let fd = listener
        listener = -1
        shutdown(fd, SHUT_RDWR)
        close(fd)
        unlink(socket.path)
    }

    private func acceptLoop(_ fd: Int32) {
        while true {
            let connection = accept(fd, nil, nil)
            if connection < 0 {
                if errno == EINTR { continue }
                return // stopped
            }
            queue.async { [weak self] in
                self?.serve(connection)
                close(connection)
            }
        }
    }

    /// One request on `connection`, from a process of this user only.
    private func serve(_ connection: Int32) {
        var uid = uid_t(0), gid = gid_t(0)
        guard getpeereid(connection, &uid, &gid) == 0, uid == getuid() else { return }
        var wait = timeval(tv_sec: 5, tv_usec: 0)
        setsockopt(connection, SOL_SOCKET, SO_RCVTIMEO, &wait, socklen_t(MemoryLayout<timeval>.size))
        var request = Data()
        var chunk = [UInt8](repeating: 0, count: 4096)
        while !request.contains(0x0A) {
            let count = read(connection, &chunk, chunk.count)
            guard count > 0 else { return }
            request.append(chunk, count: count)
            guard request.count <= HostBridge.requestMost else { return }
        }
        let line = request.prefix(upTo: request.firstIndex(of: 0x0A)!)
        var answer = (try? JSONSerialization.data(withJSONObject: HostBridge.answer(line, token: token, actions: actions))) ?? Data()
        answer.append(0x0A)
        answer.withUnsafeBytes { raw in
            var sent = 0
            while sent < raw.count {
                let count = write(connection, raw.baseAddress! + sent, raw.count - sent)
                guard count > 0 else { return }
                sent += count
            }
        }
    }

    // MARK: Answers

    /// The answer to one request: refused without the token, then the action's.
    static func answer(_ line: Data, token: String, actions: Actions) -> [String: Any] {
        guard let request = (try? JSONSerialization.jsonObject(with: line)) as? [String: Any],
              let given = request["token"] as? String, sameToken(given, token) else {
            return ["ok": false, "error": "Not this app's engine."]
        }
        guard let action = request["action"] as? String else { return refused("No action.") }
        switch action {
        case "hello":
            return ["ok": true]
        case "press", "type", "insert", "focus", "scroll", "scroll-bottom", "close":
            return tabAction(action, request, actions)
        case "open":
            return open(request, actions)
        case "secret-read", "secret-write", "secret-delete":
            return secret(action, request, actions)
        case "login-item":
            if let on = request["on"] as? Bool { actions.setLoginItem(on) }
            return ["ok": true, "on": actions.loginItem()]
        default:
            // Notifications and dictation aren't asked of the app yet.
            return refused("This Mac app can't do \(action) for the engine yet.")
        }
    }

    private static func refused(_ why: String) -> [String: Any] { ["ok": false, "error": why] }

    /// Compared whole, in the same time however much of it is right.
    static func sameToken(_ given: String, _ token: String) -> Bool {
        let a = Array(given.utf8), b = Array(token.utf8)
        guard a.count == b.count, !b.isEmpty else { return false }
        return zip(a, b).reduce(0) { $0 | ($1.0 ^ $1.1) } == 0
    }

    /// The keys the core may press, by the names the deck gives them (desktop/agent.go):
    /// a made button's keys, and Space.
    static let keys: [String: Terminal.Keystroke] = Button.keys.merging(["space": .init(key: .space)]) { kept, _ in kept }

    /// The most text typed at once, and the most presses of one key.
    static let textMost = 4000
    static let timesMost = 9

    private static func tabAction(_ action: String, _ request: [String: Any], _ actions: Actions) -> [String: Any] {
        guard let text = request["tab"] as? String, let tab = TabAddress(text) else { return refused("Not a tab.") }
        switch action {
        case "press":
            guard let name = request["key"] as? String, var keystroke = keys[name] else { return refused("Not a key the deck presses.") }
            let times = request["times"] as? Int ?? 1
            guard (1...timesMost).contains(times) else { return refused("Too many presses.") }
            keystroke.times = times
            return done(actions.press(keystroke, tab))
        case "type", "insert":
            guard let text = request["text"] as? String, !text.isEmpty, text.count <= textMost, !text.hasControlCharacters else {
                return refused("Not text to type.")
            }
            return done(action == "type" ? actions.type(text, tab) : actions.insert(text, tab))
        case "focus":
            return done(actions.focus(tab))
        case "scroll":
            guard let lines = request["lines"] as? Int, lines != 0 else { return refused("No lines to scroll.") }
            return done(actions.scroll(max(-Terminal.maxScroll, min(Terminal.maxScroll, lines)), tab))
        case "scroll-bottom":
            return done(actions.scrollToBottom(tab))
        default: // close
            guard let name = request["agent"] as? String, let agent = Agent(rawValue: name) else { return refused("Not an agent.") }
            guard let fired = actions.fire(tab, agent) else { return refused("This terminal can't do that.") }
            switch fired {
            case .fired: return ["ok": true, "fired": "fired"]
            case .gone: return ["ok": true, "fired": "gone"]
            case .notStopped: return ["ok": true, "fired": "not-stopped"]
            }
        }
    }

    private static func done(_ result: Bool?) -> [String: Any] {
        switch result {
        case true?: return ["ok": true]
        case false?: return refused("The terminal didn't take it.")
        case nil: return refused("This terminal can't do that.")
        }
    }

    /// A window of a terminal of yours, in `folder`, running `script`: a file
    /// of yours alone in the hires folder, which the core wrote. Never a line of text.
    private static func open(_ request: [String: Any], _ actions: Actions) -> [String: Any] {
        guard let terminal = request["terminal"] as? String, Openers.choices.contains(where: { $0.id == terminal }) else {
            return refused("Not a terminal Smolcorp opens.")
        }
        guard let folder = request["folder"] as? String, folder.hasPrefix("/"), isFolder(folder) else { return refused("Not a folder.") }
        guard let script = request["script"] as? String, isHireScript(script, in: actions.hires) else {
            return refused("Not a script in Smolcorp's hires folder.")
        }
        guard let tab = actions.open(terminal, folder, script) else { return refused("The window didn't open.") }
        return ["ok": true, "tab": tab]
    }

    private static func isFolder(_ path: String) -> Bool {
        var isDirectory: ObjCBool = false
        return FileManager.default.fileExists(atPath: path, isDirectory: &isDirectory) && isDirectory.boolValue
    }

    /// A regular file of yours, not a link, nobody else can change, inside `hires`.
    static func isHireScript(_ path: String, in hires: URL) -> Bool {
        guard path.hasPrefix("/"), !path.split(separator: "/").contains(".."), path.utf8.count < 1024 else { return false }
        let folder = hires.standardizedFileURL.path + "/"
        guard URL(fileURLWithPath: path).standardizedFileURL.path.hasPrefix(folder) else { return false }
        var info = stat()
        guard lstat(path, &info) == 0, info.st_mode & S_IFMT == S_IFREG, info.st_uid == getuid() else { return false }
        return info.st_mode & 0o022 == 0
    }

    private static func secret(_ action: String, _ request: [String: Any], _ actions: Actions) -> [String: Any] {
        guard let item = request["item"] as? String, let store = actions.secrets[item] else { return refused("Not an item of Smolcorp's.") }
        switch action {
        case "secret-read":
            guard let value = store.read() else { return ["ok": true, "found": false] }
            return ["ok": true, "found": true, "value": value]
        case "secret-write":
            guard let value = request["value"] as? String, !value.isEmpty, value.count <= 4096, !value.hasControlCharacters else {
                return refused("Not a value to keep.")
            }
            return store.write(value) ? ["ok": true] : refused("It wasn't kept.")
        default:
            guard let delete = store.delete else { return refused("That item isn't removed this way.") }
            delete()
            return ["ok": true]
        }
    }

    // MARK: What each action does

    /// One of the app's own items: where it's kept, as the Swift engine keeps it.
    struct Secret {
        var read: () -> String?
        var write: (String) -> Bool
        var delete: (() -> Void)?
    }

    /// The app's code each action runs: `mac` in the app, stand-ins in the tests.
    /// A tab action is nil for a terminal the app can't drive.
    struct Actions {
        var press: (Terminal.Keystroke, TabAddress) -> Bool?
        var type: (String, TabAddress) -> Bool?
        var insert: (String, TabAddress) -> Bool?
        var focus: (TabAddress) -> Bool?
        var scroll: (Int, TabAddress) -> Bool?
        var scrollToBottom: (TabAddress) -> Bool?
        var fire: (TabAddress, Agent) -> Terminal.Fired?
        var hires: URL
        var open: (_ terminal: String, _ folder: String, _ script: String) -> String?
        var secrets: [String: Secret]
        var loginItem: () -> Bool
        var setLoginItem: (Bool) -> Void

        /// The Swift engine's own: the terminal drivers on `Terminal.queue`, one at a time
        /// as the deck runs them, the openers, the secrets' stores (files in a test copy), the login item.
        static var mac: Actions {
            func drive<T>(_ tab: TabAddress, _ body: @escaping (TerminalDriver) -> T) -> T? {
                guard let driver = Terminals.driver(for: tab) else { return nil }
                return Terminal.queue.sync { body(driver) }
            }
            let viewer = ViewerStore.defaultStore()
            func viewerItem(_ account: ViewerKeychain.Account) -> Secret {
                Secret(read: { viewer.read(account) }, write: { viewer.write($0, account) }, delete: nil)
            }
            let gm = GMIdentity.defaultStore()
            return Actions(
                press: { keystroke, tab in drive(tab) { $0.press(keystroke, in: tab) } },
                type: { text, tab in drive(tab) { $0.type(text, in: tab) } },
                insert: { text, tab in drive(tab) { $0.insert(text, in: tab) } },
                focus: { tab in drive(tab) { $0.focus(tab) } },
                scroll: { lines, tab in drive(tab) { $0.scroll(lines: lines, in: tab) } },
                scrollToBottom: { tab in drive(tab) { $0.scrollToBottom(in: tab) } },
                fire: { tab, agent in drive(tab) { $0.fire(tab, agent: agent) } },
                hires: Paths.hires,
                open: { terminal, folder, script in
                    Openers.open(Openers.quoted(script), in: folder, terminal: terminal, inTmux: false, hireID: "")
                },
                secrets: [
                    "summary-key": Secret(read: { SummaryKey.store.read() }, write: { SummaryKey.save($0) }, delete: { SummaryKey.remove() }),
                    "gm-token": Secret(read: {
                        if case .token(let token) = gm.read(), !token.isEmpty { return token }
                        return nil
                    }, write: { gm.write($0) }, delete: { gm.remove() }),
                    "viewer-id": viewerItem(.id),
                    "viewer-key": viewerItem(.key),
                    "viewer-write-secret": viewerItem(.writeSecret),
                ],
                loginItem: { LoginItem.isEnabled },
                setLoginItem: { LoginItem.isEnabled = $0 })
        }
    }
}
