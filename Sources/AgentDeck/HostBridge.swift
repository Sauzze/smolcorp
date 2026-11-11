import Foundation
import Security

/// The Go core's way to what only macOS lets this app do (see `Engine`): a
/// Unix socket in the support folder, readable by you alone (the socket 0600,
/// its folder 0700), whose every request carries the token this launch made
/// and handed the core as the first line on its input (never its arguments or
/// environment, which other processes of yours can read). A request is a line
/// of JSON, and so is its answer, one per connection.
///
/// The folder and the token aren't enough on their own: another process of
/// yours can reach the folder, and could move the socket aside and listen at
/// its name. So both ends check who is at the other one, by pid, as the
/// system has it (`LOCAL_PEERPID`): this app serves only the core it started
/// (`core`), token or not, and the core sends its token only to its parent,
/// this app (desktop/hostbridge_darwin.go).
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
    /// Connections being served, never more than `connectionsMost`. Guarded by `counting`.
    private var serving = 0
    private let counting = NSLock()
    /// The pid of the core this app started (`GoCore`), the only process served: nil while there's none.
    private var corePid: pid_t?
    private let coreLock = NSLock()

    /// The core's pid: set by `GoCore` as it starts it, before the core has the token, and
    /// cleared as it ends. A connection from any other process is closed unanswered.
    var core: pid_t? {
        get { coreLock.lock(); defer { coreLock.unlock() }; return corePid }
        set { coreLock.lock(); corePid = newValue; coreLock.unlock() }
    }

    /// The longest request read: a typed text or a secret is short.
    static let requestMost = 256 << 10
    /// The most connections served at once: the core asks one thing at a time
    /// per session, so more is someone else's, and is closed at once.
    static let connectionsMost = 8
    /// How long a whole request has to arrive, however it's cut up.
    static let requestWait: TimeInterval = 5
    /// How long a tab action has to start, from the moment its connection is
    /// taken: under the core's 20 s (desktop/hostbridge_darwin.go), so the core
    /// is still waiting when it's answered. One that hasn't started by then is
    /// never run: the core has given up, and a phone may try again.
    static let actionWait: TimeInterval = 18
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
        case pathTooLong, notOurs, inUse, socket(String)

        var errorDescription: String? {
            switch self {
            case .pathTooLong: return "The support folder's path is too long for the engine's socket."
            case .notOurs: return "Something that isn't the engine's socket is where it goes."
            case .inUse: return "Another Smolcorp is already running on this support folder."
            case .socket(let call): return "The engine's socket couldn't be made (\(call): \(String(cString: strerror(errno))))."
            }
        }
    }

    /// Listens on the socket, its folder made yours alone first, and a socket
    /// an earlier launch left removed. A socket that still answers is another
    /// Smolcorp's, running on this folder: it's never taken over. Anything else
    /// there is left alone.
    func start() throws {
        let path = socket.path
        guard path.utf8.count <= HostBridge.pathMost else { throw Failure.pathTooLong }
        let folder = socket.deletingLastPathComponent()
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: folder.path)
        var info = stat()
        var address = HostBridge.address(path)
        if lstat(path, &info) == 0 {
            guard info.st_mode & S_IFMT == S_IFSOCK else { throw Failure.notOurs }
            guard !HostBridge.answers(&address) else { throw Failure.inUse }
            unlink(path)
        }
        let fd = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { throw Failure.socket("socket") }
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

    /// The socket's address for `path`, which `start` has checked is short enough.
    private static func address(_ path: String) -> sockaddr_un {
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        withUnsafeMutableBytes(of: &address.sun_path) { raw in
            raw.copyBytes(from: path.utf8.prefix(raw.count - 1))
        }
        return address
    }

    /// Whether something listens at `address`: a running Smolcorp's bridge, not one a launch left behind.
    private static func answers(_ address: inout sockaddr_un) -> Bool {
        let fd = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { return false }
        defer { close(fd) }
        return withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
        } == 0
    }

    private func acceptLoop(_ fd: Int32) {
        while true {
            let connection = accept(fd, nil, nil)
            if connection < 0 {
                if errno == EINTR { continue }
                return // stopped
            }
            let taken = DispatchTime.now()
            // An answer to a core that has given up mustn't raise SIGPIPE in this app.
            var noSignal: Int32 = 1
            setsockopt(connection, SOL_SOCKET, SO_NOSIGPIPE, &noSignal, socklen_t(MemoryLayout<Int32>.size))
            counting.lock()
            let room = serving < HostBridge.connectionsMost
            if room { serving += 1 }
            counting.unlock()
            guard room else {
                close(connection)
                continue
            }
            queue.async { [weak self] in
                self?.serve(connection, taken: taken)
                close(connection)
                self?.counting.lock()
                self?.serving -= 1
                self?.counting.unlock()
            }
        }
    }

    /// One request on `connection`, from the core this app started only (its
    /// pid, and this user's), read whole within `requestWait` of its being taken.
    private func serve(_ connection: Int32, taken: DispatchTime) {
        var uid = uid_t(0), gid = gid_t(0)
        guard getpeereid(connection, &uid, &gid) == 0, uid == getuid() else { return }
        // Another process of yours, even with the token, is never served.
        guard let core, HostBridge.peerPid(connection) == core else { return }
        let readBy = taken + HostBridge.requestWait
        var request = Data()
        var chunk = [UInt8](repeating: 0, count: 4096)
        while !request.contains(0x0A) {
            // What's left of the whole request's time, for this read.
            let left = Int64(readBy.uptimeNanoseconds) - Int64(DispatchTime.now().uptimeNanoseconds)
            guard left > 0 else { return }
            var wait = timeval(tv_sec: Int(left / 1_000_000_000), tv_usec: Int32(left % 1_000_000_000 / 1000))
            setsockopt(connection, SOL_SOCKET, SO_RCVTIMEO, &wait, socklen_t(MemoryLayout<timeval>.size))
            let count = read(connection, &chunk, chunk.count)
            guard count > 0 else { return }
            request.append(chunk, count: count)
            guard request.count <= HostBridge.requestMost else { return }
        }
        let line = request.prefix(upTo: request.firstIndex(of: 0x0A)!)
        let answer = HostBridge.answer(line, token: token, actions: actions, until: taken + HostBridge.actionWait)
        var data = (try? JSONSerialization.data(withJSONObject: answer)) ?? Data()
        data.append(0x0A)
        data.withUnsafeBytes { raw in
            var sent = 0
            while sent < raw.count {
                let count = write(connection, raw.baseAddress! + sent, raw.count - sent)
                guard count > 0 else { return }
                sent += count
            }
        }
    }

    /// The pid of the process at `connection`'s other end, as the system has it, or nil.
    static func peerPid(_ connection: Int32) -> pid_t? {
        var pid = pid_t(0)
        var size = socklen_t(MemoryLayout<pid_t>.size)
        guard getsockopt(connection, SOL_LOCAL, LOCAL_PEERPID, &pid, &size) == 0, pid > 0 else { return nil }
        return pid
    }

    // MARK: Answers

    /// The answer to one request: refused without the token, then the action's.
    /// A tab action or a window not started by `deadline` isn't run.
    static func answer(_ line: Data, token: String, actions: Actions, until deadline: DispatchTime = .distantFuture) -> [String: Any] {
        guard let request = (try? JSONSerialization.jsonObject(with: line)) as? [String: Any],
              let given = request["token"] as? String, sameToken(given, token) else {
            return ["ok": false, "error": "Not this app's engine."]
        }
        guard let action = request["action"] as? String else { return refused("No action.") }
        switch action {
        case "hello":
            return ["ok": true]
        case "press", "type", "insert", "focus", "scroll", "scroll-bottom", "close":
            return tabAction(action, request, actions, until: deadline)
        case "open":
            return open(request, actions, until: deadline)
        case "secret-read", "secret-write", "secret-delete":
            return secret(action, request, actions)
        case "login-item":
            guard let item = actions.loginItem else { return refused("A test copy has no login item: it's the copy in use's.") }
            if let on = request["on"] as? Bool { item.set(on) }
            return ["ok": true, "on": item.get()]
        default:
            // Notifications and dictation aren't asked of the app yet.
            return refused("This Mac app can't do \(action) for the engine yet.")
        }
    }

    private static func refused(_ why: String) -> [String: Any] { ["ok": false, "error": why] }

    /// A terminal the app can't do this in: the core tells it apart from a tab that's gone.
    private static let unsupported: [String: Any] = ["ok": false, "error": "This terminal can't do that.", "unsupported": true]

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

    private static func tabAction(_ action: String, _ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        guard let text = request["tab"] as? String, let tab = TabAddress(text) else { return refused("Not a tab.") }
        // Checked here; run on the terminals' queue, below, if it's still wanted.
        let work: () -> [String: Any]
        switch action {
        case "press":
            guard let name = request["key"] as? String, var keystroke = keys[name] else { return refused("Not a key the deck presses.") }
            let times = request["times"] as? Int ?? 1
            guard (1...timesMost).contains(times) else { return refused("Too many presses.") }
            keystroke.times = times
            work = { done(actions.press(keystroke, tab)) }
        case "type", "insert":
            guard let text = request["text"] as? String, !text.isEmpty, text.count <= textMost, !text.hasControlCharacters else {
                return refused("Not text to type.")
            }
            work = { done(action == "type" ? actions.type(text, tab) : actions.insert(text, tab)) }
        case "focus":
            work = { done(actions.focus(tab)) }
        case "scroll":
            guard let lines = request["lines"] as? Int, lines != 0 else { return refused("No lines to scroll.") }
            work = { done(actions.scroll(max(-Terminal.maxScroll, min(Terminal.maxScroll, lines)), tab)) }
        case "scroll-bottom":
            work = { done(actions.scrollToBottom(tab)) }
        default: // close
            guard let name = request["agent"] as? String, let agent = Agent(rawValue: name) else { return refused("Not an agent.") }
            work = {
                switch actions.fire(tab, agent) {
                case .fired?: return ["ok": true, "fired": "fired"]
                case .gone?: return ["ok": true, "fired": "gone"]
                case .notStopped?: return ["ok": true, "fired": "not-stopped"]
                case nil: return unsupported
                }
            }
        }
        switch run(on: actions.queue, until: deadline, work) {
        case .done(let answer): return answer
        case .tooLate: return refused("The terminal didn't answer in time.")
        }
    }

    /// What came of something run on a queue with a deadline.
    enum Timed<T> {
        case done(T), tooLate
    }

    /// Runs `body` on `queue`, after what's already there, as the deck runs its
    /// terminals one at a time, and waits for it until `deadline`. One that
    /// hasn't started by then never runs: its caller has given up, and a key
    /// typed late could be typed twice. One already running when the time is up
    /// can't be stopped; it finishes, and its answer goes unheard.
    static func run<T>(on queue: DispatchQueue, until deadline: DispatchTime, _ body: @escaping () -> T) -> Timed<T> {
        let pending = Pending<T>()
        queue.async {
            guard pending.begin() else { return }
            pending.finish(body())
        }
        return pending.wait(until: deadline)
    }

    /// One `run`, between the queue and its caller.
    private final class Pending<T> {
        private let lock = NSLock()
        private let finished = DispatchSemaphore(value: 0)
        private var givenUp = false
        private var value: T?
        private var hasValue = false

        /// Whether to run it: not once its caller has given up.
        func begin() -> Bool {
            lock.lock()
            defer { lock.unlock() }
            return !givenUp
        }

        func finish(_ result: T) {
            lock.lock()
            value = result
            hasValue = true
            lock.unlock()
            finished.signal()
        }

        func wait(until deadline: DispatchTime) -> Timed<T> {
            _ = finished.wait(timeout: deadline)
            lock.lock()
            defer { lock.unlock() }
            if hasValue, let value { return .done(value) }
            givenUp = true
            return .tooLate
        }
    }

    private static func done(_ result: Bool?) -> [String: Any] {
        switch result {
        case true?: return ["ok": true]
        case false?: return refused("The terminal didn't take it.")
        case nil: return unsupported
        }
    }

    /// A window of a terminal of yours, in `folder`, running `script`: a file
    /// of yours alone in the hires folder, which the core wrote. Never a line of text.
    /// Opened on the terminals' queue, as the deck opens a hire's, and as a tab
    /// action: not at all once the core has given up, so a retry never makes two.
    private static func open(_ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        guard let terminal = request["terminal"] as? String, Openers.choices.contains(where: { $0.id == terminal }) else {
            return refused("Not a terminal Smolcorp opens.")
        }
        guard let folder = request["folder"] as? String, folder.hasPrefix("/"), isFolder(folder) else { return refused("Not a folder.") }
        guard let script = request["script"] as? String, isHireScript(script, in: actions.hires) else {
            return refused("Not a script in Smolcorp's hires folder.")
        }
        switch run(on: actions.queue, until: deadline, { actions.open(terminal, folder, script) }) {
        case .done(let tab?): return ["ok": true, "tab": tab]
        case .done(nil): return refused("The window didn't open.")
        case .tooLate: return refused("The terminal didn't answer in time.")
        }
    }

    private static func isFolder(_ path: String) -> Bool {
        var isDirectory: ObjCBool = false
        return FileManager.default.fileExists(atPath: path, isDirectory: &isDirectory) && isDirectory.boolValue
    }

    /// A regular file of yours, nobody else can change, inside `hires`, with no
    /// link anywhere on the way from `hires` to it: not the file, nor a folder.
    static func isHireScript(_ path: String, in hires: URL) -> Bool {
        guard path.hasPrefix("/"), !path.split(separator: "/").contains(".."), path.utf8.count < 1024 else { return false }
        let folder = hires.standardizedFileURL.path
        let standardized = URL(fileURLWithPath: path).standardizedFileURL.path
        guard standardized.hasPrefix(folder + "/") else { return false }
        // The whole path resolved: the hires folder's own, then exactly the rest as given.
        guard let realFolder = realPath(folder), let realScript = realPath(standardized),
              realScript == realFolder + standardized.dropFirst(folder.count) else { return false }
        var info = stat()
        guard lstat(path, &info) == 0, info.st_mode & S_IFMT == S_IFREG, info.st_uid == getuid() else { return false }
        return info.st_mode & 0o022 == 0
    }

    /// `path` with every link resolved, or nil if it isn't there.
    private static func realPath(_ path: String) -> String? {
        guard let resolved = realpath(path, nil) else { return nil }
        defer { free(resolved) }
        return String(cString: resolved)
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
        /// The login item, or nil in a test copy, whose login item is the copy in use's: never read or set.
        var loginItem: (get: () -> Bool, set: (Bool) -> Void)?
        /// Where tab actions and windows opened run, one at a time.
        var queue: DispatchQueue = Terminal.queue

        /// The Swift engine's own: the terminal drivers on `Terminal.queue`, one at a time
        /// as the deck runs them, the openers, the secrets' stores (files in a test copy), the login item.
        static var mac: Actions { onThisMac() }

        /// `mac`, with what a test changes: whether this is a test copy, the login item, and the opener
        /// (`Openers.open`, given the command, the folder and the terminal).
        static func onThisMac(testCopy: Bool = TestCopy.isOn,
                              loginItem: (get: () -> Bool, set: (Bool) -> Void) = (get: { LoginItem.isEnabled }, set: { LoginItem.isEnabled = $0 }),
                              opener: @escaping (_ command: String, _ folder: String, _ terminal: String) -> String? = { command, folder, terminal in
                                  // The tab only, for now; the window Smolcorp opened (T796) goes over the bridge with M4.
                                  Openers.open(command, in: folder, terminal: terminal, inTmux: false, hireID: "")?.tab
                              }) -> Actions {
            // On `queue`, already: see `HostBridge.run`.
            func drive<T>(_ tab: TabAddress, _ body: (TerminalDriver) -> T) -> T? {
                guard let driver = Terminals.driver(for: tab) else { return nil }
                return body(driver)
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
                // The script's path as it is, as the deck's hire gives it (Deck.swift): each terminal's
                // AppleScript quotes it for the shell itself (`quoted form of`).
                open: { terminal, folder, script in opener(script, folder, terminal) },
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
                loginItem: testCopy ? nil : loginItem)
        }
    }
}
