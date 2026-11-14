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
/// its name. So both ends check who is at the other one, as the system has it:
/// this app serves only the core it started (`core`), token or not, by its
/// audit token (`LOCAL_PEERTOKEN`: its pid, that pid's version, its user), so a
/// process given the core's pid after it ended is someone else; and the core
/// sends its token only to its parent, this app, by pid
/// (desktop/hostbridge_darwin.go).
///
/// Each action is this app's own code, as the Swift engine runs it: the
/// terminal drivers (Accessibility and AppleScript; the phone's text and the
/// peek at an input line among them, as `Deck` types and reads them), the openers, the
/// secrets' stores, the login item; setup's Mac parts (Accessibility, keys
/// in other apps, In the Dock, dictation, Add smolcorp to Terminal, the Grok
/// Bot app), the office's window and the folder panel; and what each desk's
/// terminal allows, from the drivers' tables, and whether each desk's agent has
/// left its tab, read only; and which processes work in a
/// folder or name a session (`Processes.holding`, what the Mac's Let go looks
/// at), read only: nothing is ever signalled; and T3 Code's app, found and
/// opened by its bundle id, and a hire's thread opened by T3 Code's own link
/// (`T3Code.Machine`), for Open in T3 Code; and the jukebox (`Music`): its
/// keys only with Accessibility, the volume, and what's playing; when
/// you're back at the Mac, for the portable office (`MacPresence`); and the
/// office's mic dictating (`Dictation`, `DictationCleanup`): this Mac's
/// microphone recording for a desk, and what it heard, cleaned up as asked.
/// Nothing here runs a shell line: a terminal is opened only on a script the
/// core wrote in the hires folder.
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
    /// Its audit token, as the system had it when `core` was set: nil while there's none, or if it couldn't be had,
    /// and then nobody is served.
    private var coreToken: audit_token_t?
    private let coreLock = NSLock()

    /// The core's pid: set by `GoCore` as it starts it, before the core has the token, and
    /// cleared as it ends. A connection from any other process is closed unanswered: one with
    /// another pid, and one with this pid that isn't this process (`coreToken`).
    var core: pid_t? {
        get { coreLock.lock(); defer { coreLock.unlock() }; return corePid }
        set {
            let token = newValue.flatMap(HostBridge.auditToken(of:))
            coreLock.lock()
            corePid = newValue
            coreToken = token
            coreLock.unlock()
        }
    }

    private var coreAudit: audit_token_t? {
        coreLock.lock()
        defer { coreLock.unlock() }
        return coreToken
    }

    /// The longest request read: a typed text or a secret is short.
    static let requestMost = 256 << 10
    /// The most connections served at once: the core asks one thing at a time
    /// per session, so more is someone else's, and is closed at once.
    static let connectionsMost = 8
    /// How long a whole request has to arrive, however it's cut up.
    static let requestWait: TimeInterval = 5
    /// How long an action is waited for, from the moment its connection is
    /// taken: under the core's 20 s (desktop/hostbridge_darwin.go), so the
    /// core is still waiting when it's answered. A window has this long to
    /// start opening; one that hasn't by then is never run: the core has given
    /// up, and a phone may try again. A tab action has less to start,
    /// `Terminals.inputWait`, as the deck's own keys have; once started, it's
    /// waited for until this, and answered as it went.
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
        // Another process of yours, even with the token, is never served: only the core, by its audit token.
        guard let expected = coreAudit, let peer = HostBridge.peerToken(connection),
              HostBridge.sameProcess(peer, expected) else { return }
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
        // Anything sent after the request came before any question: it answers none (see CoreQuestion).
        let question = CoreQuestion(connection, sentEarly: request.count > line.count + 1, until: taken + HostBridge.actionWait)
        let answer = HostBridge.answer(line, token: token, actions: actions, until: taken + HostBridge.actionWait, deskState: question.ask)
        question.close()
        var data = (try? JSONSerialization.data(withJSONObject: answer)) ?? Data()
        data.append(0x0A)
        _ = HostBridge.write(data, to: connection)
    }

    /// Writes all of `data` to `connection`: false if it couldn't.
    static func write(_ data: Data, to connection: Int32) -> Bool {
        data.withUnsafeBytes { raw in
            var sent = 0
            while sent < raw.count {
                let count = Darwin.write(connection, raw.baseAddress! + sent, raw.count - sent)
                guard count > 0 else { return false }
                sent += count
            }
            return true
        }
    }

    /// The one thing the bridge asks the core back, on a request's own connection: the desk's state as the core
    /// has it now, for the phone's text (desktop/hostbridge_darwin.go answers it at once, from its sessions).
    /// Asked from the terminal's queue, by `Deck.typePhoneWords`, where the Swift engine looks at its own
    /// sessions: right before one line, and before and right before a paste of several (`ready`). So a prompt
    /// that came while the words waited for the queue holds them, and so does a turn that started. At most
    /// `most` times, and never once the request is answered: the connection may be closed by then, and its
    /// number another's. No answer in time, or anything but a state, is asking: nothing goes.
    final class CoreQuestion {
        /// How long the core has to answer each time: it only looks at its sessions.
        static let wait: TimeInterval = 2
        /// The longest answer read: `{"state":"thinking"}`.
        static let answerMost = 64
        /// The most times one request asks: `Deck.typePhoneWords` asks twice at most (`ClipboardPaste.paste`).
        static let most = 3
        private let lock = NSLock()
        private let connection: Int32
        private let deadline: DispatchTime
        private var left: Int

        init(_ connection: Int32, sentEarly: Bool, until deadline: DispatchTime) {
            self.connection = connection
            self.deadline = deadline
            left = sentEarly ? 0 : CoreQuestion.most
        }

        /// The desk's state, as the core said it: nil for a desk it no longer has.
        func ask() -> SessionState? {
            lock.lock()
            defer { lock.unlock() }
            guard left > 0 else { return .asking }
            left -= 1
            guard HostBridge.write(Data("{\"ready\":true}\n".utf8), to: connection) else {
                left = 0
                return .asking
            }
            let by = min(deadline, .now() + CoreQuestion.wait)
            var reply = Data()
            var chunk = [UInt8](repeating: 0, count: CoreQuestion.answerMost)
            while !reply.contains(0x0A) {
                let time = Int64(by.uptimeNanoseconds) - Int64(DispatchTime.now().uptimeNanoseconds)
                guard time > 0, reply.count <= CoreQuestion.answerMost else { return unheard() }
                var wait = timeval(tv_sec: Int(time / 1_000_000_000), tv_usec: Int32(time % 1_000_000_000 / 1000))
                setsockopt(connection, SOL_SOCKET, SO_RCVTIMEO, &wait, socklen_t(MemoryLayout<timeval>.size))
                let count = read(connection, &chunk, chunk.count)
                guard count > 0 else { return unheard() }
                reply.append(chunk, count: count)
            }
            // One line, and nothing after it: the core says nothing more until it has the answer.
            let line = reply.prefix(upTo: reply.firstIndex(of: 0x0A)!)
            guard reply.count == line.count + 1,
                  let said = (try? JSONSerialization.jsonObject(with: line)) as? [String: Any],
                  let state = said["state"] as? String else { return unheard() }
            // "": the core has no such desk now, as the Swift engine's own sessions would say nil.
            guard !state.isEmpty else { return nil }
            return SessionState(rawValue: state) ?? .asking
        }

        /// Not heard in time, or not as asked: asking, and the connection is asked nothing more, as what it
        /// says next could be the late answer to this.
        private func unheard() -> SessionState? {
            left = 0
            return .asking
        }

        /// No question once the request is answered.
        func close() {
            lock.lock()
            left = 0
            lock.unlock()
        }
    }

    /// The audit token of the process at `connection`'s other end, as the system set it when that end connected.
    static func peerToken(_ connection: Int32) -> audit_token_t? {
        var token = audit_token_t()
        var size = socklen_t(MemoryLayout<audit_token_t>.size)
        guard getsockopt(connection, SOL_LOCAL, LOCAL_PEERTOKEN, &token, &size) == 0,
              size == socklen_t(MemoryLayout<audit_token_t>.size) else { return nil }
        return token
    }

    /// The audit token of the running process `pid`, from its task's name port, which tells about a process of
    /// yours and controls nothing: nil if there's no such process or it isn't yours.
    static func auditToken(of pid: pid_t) -> audit_token_t? {
        var name = mach_port_name_t(MACH_PORT_NULL)
        guard pid > 0, task_name_for_pid(mach_task_self_, pid, &name) == KERN_SUCCESS else { return nil }
        defer { mach_port_deallocate(mach_task_self_, name) }
        var token = audit_token_t()
        var count = mach_msg_type_number_t(MemoryLayout<audit_token_t>.size / MemoryLayout<natural_t>.size)
        let status = withUnsafeMutablePointer(to: &token) {
            $0.withMemoryRebound(to: integer_t.self, capacity: Int(count)) {
                task_info(name, task_flavor_t(TASK_AUDIT_TOKEN), $0, &count)
            }
        }
        guard status == KERN_SUCCESS, token.val.5 == UInt32(bitPattern: pid) else { return nil }
        return token
    }

    /// Whether two audit tokens are the same process of `user`: the same pid, the same version of that pid (a
    /// process that gets the pid later has another), and both run as `user`. The fields are libbsm's
    /// (`audit_token_to_euid`, `_pid`, `_pidversion`: val[1], val[5], val[7]).
    static func sameProcess(_ peer: audit_token_t, _ expected: audit_token_t, user: uid_t = getuid()) -> Bool {
        peer.val.5 == expected.val.5 && peer.val.7 == expected.val.7 && peer.val.1 == user && expected.val.1 == user
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
    /// A tab action or a window not started by `deadline` isn't run. `deskState` asks the core the desk's state,
    /// for the phone's text (`CoreQuestion`): with none to ask, it's asking, and nothing goes.
    static func answer(_ line: Data, token: String, actions: Actions, until deadline: DispatchTime = .distantFuture,
                       deskState: @escaping () -> SessionState? = { .asking }) -> [String: Any] {
        guard let request = (try? JSONSerialization.jsonObject(with: line)) as? [String: Any],
              let given = request["token"] as? String, sameToken(given, token) else {
            return ["ok": false, "error": "Not this app's engine."]
        }
        guard let action = request["action"] as? String else { return refused("No action.") }
        switch action {
        case "hello":
            return ["ok": true]
        case "press", "type", "insert", "focus", "scroll", "scroll-bottom", "close":
            // A test copy brings no terminal forward (RTCH N1), its own hires' panes included, and does nothing at all in
            // a tab that isn't one of those panes (RWTC N1), as the Swift engine's deck.
            if action == "focus", let why = TestCopy.refusal(for: .terminal, testCopy: actions.testCopy) { return refused(why) }
            if let why = TestCopy.refusal(forTab: request["tab"] as? String ?? "", testCopy: actions.testCopy) { return refused(why) }
            return tabAction(action, request, actions, until: deadline)
        case "phone-text", "peek":
            if let why = TestCopy.refusal(forTab: request["tab"] as? String ?? "", testCopy: actions.testCopy) { return refused(why) }
            return phoneAction(action, request, actions, until: deadline, deskState: deskState)
        case "open":
            if let why = TestCopy.refusal(for: .terminal, testCopy: actions.testCopy) { return refused(why) }
            return open(request, actions, until: deadline)
        case "terminals":
            // The terminals of the three Smolcorp opens that are installed, in setup's order.
            return ["ok": true, "terminals": actions.terminals()]
        case "window-look", "window-close":
            // A test copy opens no hire's window (`open` above), so it has none to look at or close: either would ask
            // the owner's terminal app about one of its windows (RMSY4 nit 5).
            if let why = TestCopy.refusal(for: .terminal, testCopy: actions.testCopy) { return refused(why) }
            return window(action, request, actions, until: deadline)
        case "secret-read", "secret-write", "secret-delete":
            return secret(action, request, actions)
        case "record-key-look", "record-key-new", "record-key-add", "record-key-retire":
            return recordKey(action, request, actions.records)
        case "api-key-offer", "api-key-read":
            return hireKey(action, actions.hireKey)
        case "login-item":
            guard let item = actions.loginItem else { return refused("This app has no login item.") }
            if let on = request["on"] as? Bool {
                // A test copy's login item is the Smolcorp in use's: read, as the Swift engine's setup reads it, never set.
                guard !actions.testCopy else { return refused(SelfTest.testCopyLogin) }
                item.set(on)
            }
            return ["ok": true, "on": item.get()]
        case "office", "office-ontop", "office-key", "choose-folder", "folder-panel-close":
            return shell(action, request, actions, until: deadline)
        case "facts", "install-check", "install", "dock", "accessibility", "keystrokes-on", "keystroke-apps", "glossary", "dictation", "terminal-command":
            if action == "accessibility", let why = TestCopy.refusal(for: .accessibility, testCopy: actions.testCopy) { return refused(why) }
            // Getting dictation ready asks for the microphone: a system prompt a test copy never raises (RWTC N2).
            if action == "dictation", let why = TestCopy.refusal(for: .microphone, testCopy: actions.testCopy) { return refused(why) }
            return setup(action, request, actions, until: deadline)
        case "dictation-start", "dictation-stop", "dictation-clean", "dictation-cleanup-off":
            return dictate(action, request, actions, until: deadline)
        case "abilities":
            guard let tabs = strings(request["tabs"], most: tabsMost) else { return refused("Not a list of tabs.") }
            switch run(on: actions.main, until: deadline, { tabs.map(actions.abilities) }) {
            case .done(let abilities): return ["ok": true, "abilities": abilities]
            case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
            }
        case "agents-ended":
            return agentsEnded(request, actions, until: deadline)
        case "bots-resolve":
            return resolveBots(request, actions)
        case "processes":
            return processes(request, actions, until: deadline)
        case "t3-app", "t3-open-app", "t3-link":
            // Looking is fine; opening T3 Code's app or a thread in it is not (RTCH N1).
            if action != "t3-app", let why = TestCopy.refusal(for: .t3Code, testCopy: actions.testCopy) { return refused(why) }
            return t3(action, request, actions, until: deadline)
        case "bots-roster":
            // The Grok Bot app's bots now, read afresh, for the core to find them as `CloudBot.discover` does: null
            // when the roster can't be read, which isn't an app with no bots.
            let rows = actions.roster().map { $0.map { ["id": $0.id, "name": $0.name, "isGroup": $0.isGroup] as [String: Any] } }
            return ["ok": true, "rows": rows ?? NSNull()]
        case "music", "music-state", "volume":
            // The owner's music and volume, and Music or Spotify asked over Apple Events: never from a test copy (RWTC N2).
            if let why = TestCopy.refusal(for: .music, testCopy: actions.testCopy) { return refused(why) }
            return music(action, request, actions, until: deadline)
        case "time-format":
            // How this Mac writes a short time, alone: a webhook delivery's time asks it, not the whole facts (RWTC N7).
            return ["ok": true, "timeFormat": actions.timeFormat()]
        case "presence":
            // When you're back at the Mac, for the portable office (`MacPresence`): asked every few seconds while
            // it's on. Never used since the Mac started is said as a year.
            let now = actions.presence()
            return ["ok": true, "idle": min(max(now.idle, 0), MacPresence.idleMost), "locked": now.locked,
                    "unlocks": now.unlocks, "wakes": now.wakes]
        case "screens", "tile", "office-place":
            return tiling(action, request, actions, until: deadline)
        default:
            // Notifications aren't asked of the app yet.
            return refused("This Mac app can't do \(action) for the engine yet.")
        }
    }

    /// How this Mac writes a short time (`DateFormatter`'s .short time style, as `Deck.described` says a webhook's
    /// delivery): its ICU pattern and the words for AM and PM, so the Go core writes the same one, not a guess from
    /// the region (RFXC1a N5).
    static func shortTimeFormat(locale: Locale = .current) -> [String: String] {
        let formatter = DateFormatter()
        formatter.locale = locale
        formatter.dateStyle = .none
        formatter.timeStyle = .short
        return ["pattern": formatter.dateFormat, "am": formatter.amSymbol, "pm": formatter.pmSymbol]
    }

    /// The most tabs asked about at once, and the most bots looked for.
    static let tabsMost = 256
    static let botsMost = 128

    /// `value` as a list of strings, none longer than 1024: none for nothing, nil for anything else or more than `most`.
    static func strings(_ value: Any?, most: Int) -> [String]? {
        guard let value else { return [] }
        guard let list = value as? [String], list.count <= most, list.allSatisfy({ $0.utf8.count <= 1024 }) else { return nil }
        return list
    }

    /// Setup's Mac parts, as the Swift engine's routes do them (DeckRouter.swift), each with its code: the
    /// settings themselves are the core's to keep.
    private static func setup(_ action: String, _ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        switch action {
        case "facts":
            guard let apps = strings(request["apps"], most: Keystrokes.AppList.most) else { return refused("Not a list of apps.") }
            switch run(on: actions.main, until: deadline, { actions.facts(apps) }) {
            case .done(let facts): return ["ok": true, "facts": facts]
            case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
            }
        case "install-check":
            // The checklist's install line, as this app says it: where it is, and Move to Applications if it
            // should be (the core can't tell where the app's bundle is, nor that macOS translocated it).
            switch run(on: actions.main, until: deadline, { actions.installCheck().jsonObject }) {
            case .done(let check): return ["ok": true, "check": check]
            case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
            }
        case "install":
            // The fix of that line, as the Swift engine's: the app copied into Applications and opened there, or why not.
            switch run(on: actions.main, until: deadline, { () -> String? in
                do { try actions.moveToApplications() } catch { return error.localizedDescription }
                return nil
            }) {
            case .done(nil): return ["ok": true]
            case .done(let problem?): return refused(problem)
            case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
            }
        case "dock":
            guard let on = request["on"] as? Bool else { return refused("Say on or off.") }
            return onMain(actions, until: deadline) { actions.dock(on) }
        case "accessibility":
            return onMain(actions, until: deadline) { actions.askAccessibility() }
        case "keystrokes-on":
            return onMain(actions, until: deadline) { actions.keystrokesOn() }
        case "keystroke-apps":
            guard let list = strings(request["apps"], most: Keystrokes.AppList.most), let app = request["app"] as? String,
                  let add = request["on"] as? Bool else { return refused("Say the app, the list, and on or off.") }
            guard add else { return ["ok": true, "apps": Keystrokes.AppList.removing(app, from: list)] }
            switch Keystrokes.AppList.adding(app, to: list) {
            case .success(let apps): return ["ok": true, "apps": apps]
            case .failure(let problem): return refused(problem.reason)
            }
        case "glossary":
            guard let names = strings(request["names"], most: 10_000) else { return refused("That isn't a list of names.") }
            return ["ok": true, "names": DictationCleanup.checked(names)]
        case "dictation":
            guard let language = request["language"] as? String, Dictation.languages.contains(language) else {
                return refused("Not a language dictation knows.")
            }
            // The microphone asked for, then the language's model got ready: downloaded the first time, which
            // can take longer than the core waits. It goes on getting ready, and setup says so meanwhile.
            let pending = Pending<String?>(startBy: deadline)
            actions.main.async { actions.prepareDictation(language) { pending.finish($0) } }
            switch pending.wait(until: deadline) {
            case .done(let problem): return ["ok": true, "problem": problem ?? NSNull()]
            case .notStarted, .unfinished: return ["ok": true, "problem": stillGettingReady]
            }
        default: // terminal-command
            guard let on = request["on"] as? Bool else { return refused("Say on or off.") }
            do {
                try actions.terminalCommand(on)
                return ["ok": true]
            } catch {
                return refused(error.localizedDescription)
            }
        }
    }

    /// The office's mic dictating (desktop/hostdictation.go): this Mac's microphone recording for a desk's tab, as
    /// the Swift engine's mic started it (`Deck.perform`'s mic: only where the tab's terminal types text, with
    /// Accessibility where its keys need it; `Dictation.start`), and stopped, what was heard given back; then that
    /// text cleaned up by Claude when the core says so, with the core's glossary and desks (`Deck.insertDictation`,
    /// `DictationCleanup.clean`). The core types it into the tab and keeps the settings and the desk's note.
    private static func dictate(_ action: String, _ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        if action == "dictation-start" {
            // A test copy never records from this Mac's microphone (RGAPA B1), whatever its settings say.
            if let why = TestCopy.refusal(for: .microphone, testCopy: actions.testCopy) { return refused(why) }
            guard let language = request["language"] as? String, Dictation.languages.contains(language) else {
                return refused("Not a language dictation knows.")
            }
            guard let text = request["tab"] as? String, let tab = TabAddress(text) else { return refused("Not a tab.") }
            guard let driver = actions.driver(tab), driver.abilities.contains(.text) else { return unsupported }
            guard !driver.keysNeedAccessibility || Deck.keyboardAllowed() else { return refused(Deck.noKeyboard) }
            // The core is recording already: checked as any start, then `Dictation.start`'s no, nothing recorded
            // (RGAPA N2: the Swift engine's order).
            if request["busy"] as? Bool == true { return ["ok": true, "started": false] }
            let pending = Pending<Bool>(startBy: deadline)
            actions.main.async { actions.startDictation(language, tab.text) { pending.finish($0) } }
            switch pending.wait(until: deadline) {
            case .done(let started): return ["ok": true, "started": started]
            case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
            }
        }
        if action == "dictation-cleanup-off" {
            // Claude's clean-up switched off: its last problem goes with it, as `Deck.setCleanup` (RGAPA N4).
            return onMain(actions, until: deadline) { actions.clearCleanupProblem() }
        }
        let pending = Pending<[String: Any]>(startBy: deadline)
        if action == "dictation-stop" {
            // Stopped, and what was heard given back at once: the recording is over before the clean-up, so the
            // next one can start meanwhile, as the Swift engine's.
            actions.main.async { actions.stopDictation { pending.finish(["ok": true, "text": $0]) } }
        } else {
            // dictation-clean: the glossary as setup has it, or none set and the app's own.
            guard let text = request["text"] as? String, !text.isEmpty, text.count <= textMost else { return refused("Not text to clean up.") }
            guard let desks = strings(request["desks"], most: 10_000),
                  let glossary = request["glossary"] == nil ? DictationCleanup.defaultGlossary : strings(request["glossary"], most: 10_000) else {
                return refused("That isn't a list of names.")
            }
            let cleanup = request["on"] as? Bool ?? false
            actions.main.async {
                actions.cleanDictation(text, desks, cleanup, glossary) { cleaned, outcome in
                    pending.finish(["ok": true, "text": cleaned, "note": outcome.note ?? NSNull(), "cleaned": outcome.isCleaned])
                }
            }
        }
        switch pending.wait(until: deadline) {
        case .done(let answer): return answer
        case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
        }
    }

    /// The office's mic for the core: `Dictation` itself, with what a recording heard when it stopped by itself, at
    /// its cap, kept for the core's stop, which comes for it. A new recording drops what an old one left. Main thread.
    enum CoreDictation {
        private static var leftover = ""

        static func start(_ language: String, _ tab: String, _ done: @escaping (Bool) -> Void) {
            leftover = ""
            Dictation.start(language: language, for: tab, phone: "core", deliver: { text, _ in leftover = text }, done: done)
        }

        static func stop(_ done: @escaping (String) -> Void) {
            guard Dictation.isActive else {
                let text = leftover
                leftover = ""
                return done(text)
            }
            Dictation.stop { text, _ in done(text) }
        }
    }

    /// How long the folder panel waits for you before it's closed, as if cancelled, and nothing is picked:
    /// under the core's own wait for it (desktop/hostbridge.go's folderPanelWait and folderPanelMargin).
    static var panelWait: TimeInterval = 600

    /// What the Swift engine does with this app's own windows, as DeckRouter does it: the office's window
    /// shown or kept on top, and its keys from anywhere (the settings are the core's to keep), and the folder
    /// panel, which waits for you.
    private static func shell(_ action: String, _ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        switch action {
        case "office":
            return onMain(actions, until: deadline) { actions.showOffice() }
        case "office-ontop":
            guard let on = request["on"] as? Bool else { return refused("Say on or off.") }
            return onMain(actions, until: deadline) { actions.officeOnTop(on) }
        case "office-key":
            // Checked again here, on the layout in use (read on main): the core checks them too, but only keys the
            // page could choose are ever held.
            guard let keys = request["keys"] as? String, keys != "default" else { return refused(OfficeKey.shapeRefusal) }
            switch run(on: actions.main, until: deadline, { () -> String? in
                switch OfficeKey.choice(keys, labels: actions.officeKeyLabels()) {
                case .failure(let refusal): return refusal.message
                case .success(let key): return actions.officeKey(key)
                }
            }) {
            case .done(nil): return ["ok": true]
            case .done(let problem?): return refused(problem)
            case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
            }
        case "folder-panel-close":
            return onMain(actions, until: deadline) { actions.closeFolderPanel(Deck.hireFolderMessage) }
        default: // choose-folder
            let messages = ["hires": Deck.hireFolderMessage, "floor": Deck.floorFolderMessage]
            guard let purpose = request["for"] as? String, let message = messages[purpose] else { return refused("Not a folder panel Smolcorp opens.") }
            var start: String?
            if let folder = request["folder"] as? String, !folder.isEmpty {
                guard folder.hasPrefix("/"), folder.utf8.count <= 1024, !folder.hasControlCharacters else { return refused("Not a folder.") }
                start = folder
            }
            // Opened within the bridge's own time, as a window is, then waited for as long as you take, up to `panelWait`.
            let until = DispatchTime.now() + panelWait
            let pending = Pending<String?>(startBy: min(deadline, until))
            actions.main.async {
                guard pending.begin() else { return }
                actions.chooseFolder(start, message) { pending.finish($0) }
            }
            switch pending.wait(until: until) {
            case .done(let picked?): return ["ok": true, "folder": picked]
            case .done(nil): return ["ok": true]
            case .notStarted: return refused("The Mac didn't answer in time.")
            case .unfinished:
                // Left open: closed now, as if cancelled, so a later choice isn't kept by nobody.
                actions.main.async { actions.closeFolderPanel(message) }
                return ["ok": true]
            }
        }
    }

    /// How long a tile may take, each window moved with an AppleScript of its own and the first one perhaps waiting
    /// for your answer to macOS's Automation question: under the core's own wait for it (desktop/gmscreens.go's
    /// tileWait). And the most windows moved at once: the ground floor's nine desks.
    static var tileWait: TimeInterval = 90
    static let tilePlacesMost = 9

    /// The GM's tiling, for the core (desktop/gmscreens.go), as the Swift engine's `POST /setup/gm/tile` did it with
    /// `Tiler`: the Mac's screens, read afresh on the main queue; the hires' windows moved with their terminal's own
    /// AppleScript on its queue, each to the place the core gives for the tty it shows; the office's window put
    /// under them. The grid and which windows are the core's to work out.
    private static func tiling(_ action: String, _ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        // A test copy never moves another app's windows, nor puts its own office under them: the Swift engine's refusal
        // of `POST /setup/gm/tile`, now where the bridge's two moving actions are. Reading the screens is harmless.
        if action != "screens", let why = TestCopy.refusal(for: .tiler, testCopy: actions.testCopy) { return refused(why) }
        switch action {
        case "screens":
            switch run(on: actions.main, until: deadline, { actions.screens() }) {
            case .done(let screens): return ["ok": true, "screens": screens]
            case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
            }
        case "office-place":
            guard let rect = rect(request["office"]), let mainHeight = (request["mainHeight"] as? NSNumber)?.doubleValue else {
                return refused("Say where the office goes.")
            }
            return onMain(actions, until: deadline) { actions.placeOffice(rect, CGFloat(mainHeight)) }
        default: // tile
            guard let terminal = request["terminal"] as? String, let app = Tiler.scriptable(terminal) else {
                return refused("Not a terminal Smolcorp can place the windows of.")
            }
            guard let given = request["places"] as? [[String: Any]], given.count <= tilePlacesMost else { return refused("Not a list of places.") }
            var places: [(tty: String, rect: CGRect)] = []
            for place in given {
                guard let tty = place["tty"] as? String, TabAddress.isTTY(tty), let rect = rect(place["rect"]) else { return refused("Not a list of places.") }
                places.append((tty, rect))
            }
            // Started within the bridge's own time, then waited for as long as the AppleScripts take, up to `tileWait`.
            let until = DispatchTime.now() + tileWait
            let pending = Pending<Tiler.Moved?>(startBy: min(deadline, until))
            actions.windowQueue(terminal).async {
                guard pending.begin() else { return }
                pending.finish(Tiler.tile(app: app, places: places, script: actions.tileScript))
            }
            switch pending.wait(until: until) {
            case .done(let moved?):
                var answer: [String: Any] = ["ok": true, "placed": moved.placed]
                if moved.needsPermission { answer["needsPermission"] = true }
                return answer
            case .done(nil): return refused("\(app) couldn't be asked for its windows.")
            case .notStarted, .unfinished: return refused("\(app) didn't answer in time.")
            }
        }
    }

    /// `{x, y, width, height}` as the core sends a place, whole and finite, else nil.
    private static func rect(_ value: Any?) -> CGRect? {
        guard let given = value as? [String: Any] else { return nil }
        let numbers = ["x", "y", "width", "height"].compactMap { (given[$0] as? NSNumber)?.doubleValue }
        guard numbers.count == 4, numbers.allSatisfy(\.isFinite), numbers[2] >= 0, numbers[3] >= 0 else { return nil }
        return CGRect(x: numbers[0], y: numbers[1], width: numbers[2], height: numbers[3])
    }

    /// What setup says while Apple's model for a language is still on its way.
    static let stillGettingReady = "Dictation is still getting the language ready on this Mac. Look again in a moment."

    /// `body` run on the app's main queue, as the Swift engine's routes run it, waited for until `deadline`.
    private static func onMain(_ actions: Actions, until deadline: DispatchTime, _ body: @escaping () -> Void) -> [String: Any] {
        switch run(on: actions.main, until: deadline, body) {
        case .done: return ["ok": true]
        case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
        }
    }

    /// Who each bot is in the Grok Bot app now, read afresh, in the order asked: null for one not there.
    private static func resolveBots(_ request: [String: Any], _ actions: Actions) -> [String: Any] {
        guard let bots = request["bots"] as? [[String: Any]], bots.count <= botsMost else { return refused("Not a list of bots.") }
        var wanted: [(botId: String, name: String)] = []
        for bot in bots {
            guard let id = bot["botId"] as? String, let name = bot["bot"] as? String, id.count <= 64, name.count <= 200 else {
                return refused("Not a list of bots.")
            }
            // An id no bot can have is refused, not looked for by its name instead: a bot without one sends "".
            guard id.isEmpty || GrokBotApp.safeID(id) != nil else { return refused("Not a list of bots.") }
            wanted.append((id, name))
        }
        let rows = actions.roster()
        return ["ok": true, "resolved": wanted.map { bot -> Any in
            rows.flatMap { GrokBotApp.resolve(botId: bot.botId, name: bot.name, in: $0) }?.json ?? NSNull()
        }]
    }

    /// The most processes answered: more than one is enough for the core to wait or refuse.
    static let processesMost = 64

    /// The processes working in a folder (or one inside it), or given it with `--add-dir`, and, with a session, those
    /// whose arguments name it: Let go's check and Take over's on the Go core (desktop/t3process_darwin.go), by the
    /// code the Mac's own Let go uses (`Processes.holding`). Each with its pid, its name and why; the first
    /// `processesMost`. Read from the kernel only: no process is signalled, nothing is run. A folder is a full path
    /// with no `..` and isn't `/`, and must be the worktree of a hire still open in the core's registry (RM5c S2:
    /// no more than Let go, Take over and Hand back look at); a session is a Claude Code session's id, its UUID
    /// shape exactly (desktop/t3takeover.go's t3SessionShape), or none. A Mac that can't look says so, and the core
    /// then changes nothing.
    private static func processes(_ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        guard let folder = request["folder"] as? String, folder.hasPrefix("/"), folder != "/", folder.utf8.count <= 1024,
              !folder.hasControlCharacters, !folder.split(separator: "/").contains("..") else { return refused("Not a folder.") }
        let session = request["session"] as? String ?? ""
        guard request["session"] == nil || request["session"] is String, session.isEmpty || isSessionID(session) else {
            return refused("Not a session.")
        }
        guard let worktrees = actions.openWorktrees() else { return refused("This Mac couldn't read the team's registry.") }
        let wanted = HireDesk.physical(folder)
        guard worktrees.contains(where: { $0 == folder || HireDesk.physical($0) == wanted }) else {
            return refused("Not the worktree of a hire on the team.")
        }
        switch run(on: actions.looking, until: deadline, { actions.processes(folder, session) }) {
        case .done(let found?):
            return ["ok": true, "processes": found.prefix(processesMost).map { ["pid": Int($0.pid), "name": $0.name, "why": $0.why] }]
        case .done(nil): return refused("This Mac couldn't list its processes.")
        case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
        }
    }

    /// Whether each tab's agent has left it, for the core to drop the desks of tabs closed in the terminals only this
    /// app drives (FXHIRE B1 b, as the Swift engine's `Deck.pruneClosedTabs` and `pruneClosedPanes`): `tabs` and
    /// `agents` side by side, answered in order, true only when it's known. Read only (`openTTYs`, `ended`): nothing is
    /// signalled, nothing closed.
    private static func agentsEnded(_ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        guard let tabs = strings(request["tabs"], most: tabsMost), let names = strings(request["agents"], most: tabsMost),
              tabs.count == names.count else { return refused("Not a list of tabs and their agents.") }
        let asked = zip(tabs, names).map { (TabAddress($0), Agent(rawValue: $1)) }
        switch run(on: actions.looking, until: deadline, { () -> [Bool] in
            // A Terminal tab is asked of `ps` only once it has left Terminal's own tab list, as
            // `Deck.pruneClosedTabs` did: Terminal asked once for all. Unknown is not gone, and a test copy never
            // asks Terminal (an Automation prompt on the owner's screen), so its Terminal tabs never go this way.
            let open = !actions.testCopy && asked.contains { $0.0?.kind == "tty" } ? actions.openTTYs() : nil
            return asked.map { tab, agent in
                guard let tab, let agent, tab.kind != "tty" || open.map({ !$0.contains(tab.id) }) == true else { return false }
                return actions.ended(tab, agent, actions.testCopy)
            }
        }) {
        case .done(let ended): return ["ok": true, "ended": ended]
        case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
        }
    }

    /// Whether `agent` has left `tab`, as the Swift engine's `Deck.pruneClosedTabs` and `pruneClosedPanes` judge
    /// it: a Terminal tab (already off Terminal's tab list: `agentsEnded`) by `ps`, its agent no longer on its tty;
    /// another app's tab closed, or open with its agent gone from its tty; a tab Smolcorp can only watch, its address
    /// a tty (`other:<tty>`), by `ps`. Unknown is not gone. A test copy never asks iTerm2 or Ghostty (an Automation
    /// prompt on the owner's screen): `ps` only.
    static func agentEnded(_ tab: TabAddress, _ agent: Agent, testCopy: Bool) -> Bool {
        if tab.kind == "tty" { return Terminal.isRunning(agent, on: tab.id) == false }
        if !testCopy, let driver = Terminals.driver(for: tab) {
            return driver.isOpen(tab) == false || driver.tty(of: tab).flatMap { Terminal.isRunning(agent, on: $0) } == false
        }
        guard tab.kind == "other", let tty = Terminals.ttyInAddress(of: tab.text) else { return false }
        return Terminal.isRunning(agent, on: tty) == false
    }

    /// A Claude Code session's id as T3 Code resumes one: 8-4-4-4-12 hex digits, either case.
    static func isSessionID(_ text: String) -> Bool {
        let groups = text.split(separator: "-", omittingEmptySubsequences: false)
        return groups.map(\.count) == [8, 4, 4, 4, 12] && groups.allSatisfy { $0.allSatisfy { $0.isASCII && $0.isHexDigit } }
    }

    /// The worktrees of the hires still open (neither fired nor failed) in the registry at `file`, the one the core
    /// keeps: read as it is, never written, never set aside (`FrontDesk.load` would move a file it can't read). None
    /// when there's no registry yet; nil when it can't be read.
    static func openWorktrees(in file: URL) -> [String]? {
        guard let data = try? Data(contentsOf: file) else { return FileManager.default.fileExists(atPath: file.path) ? nil : [] }
        guard let registry = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
              let hires = registry["hires"] as? [[String: Any]] else { return nil }
        return hires.compactMap { hire in
            guard let worktree = hire["worktree"] as? String, !worktree.isEmpty,
                  !["fired", "failed"].contains(hire["state"] as? String ?? "") else { return nil }
            return worktree
        }
    }

    /// T3 Code's app on this Mac, as the Swift engine's `T3Code.Machine` has it, for the Go core's Open in T3 Code
    /// (C4 A1): whether it's installed, by its bundle id (`t3-app`); the app opened (`t3-open-app`), as before
    /// `t3 app <folder>`, which the core runs itself; and a hire's thread opened by T3 Code's own link (`t3-link`),
    /// `t3code://threads/<env>/<thread>` exactly as `T3Desk.deepLink` makes it, with NSWorkspace. Never a shell,
    /// never `t3`, never any other link.
    private static func t3(_ action: String, _ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        switch action {
        case "t3-app":
            switch run(on: actions.looking, until: deadline, { actions.t3App() }) {
            case .done(let found): return ["ok": true, "found": found]
            case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
            }
        case "t3-open-app":
            return onMain(actions, until: deadline) { actions.t3OpenApp() }
        default: // t3-link
            guard let text = request["link"] as? String, let link = t3ThreadLink(text) else { return refused("Not a T3 Code thread's link.") }
            switch run(on: actions.main, until: deadline, { actions.t3OpenLink(link) }) {
            case .done(true): return ["ok": true]
            case .done(false): return refused("T3 Code didn't take the link to the hire's thread.")
            case .notStarted, .unfinished: return refused("The Mac didn't answer in time.")
            }
        }
    }

    /// `text` if it's T3 Code's own link to a thread, `t3code://threads/<env>/<thread id, URL-encoded>`, exactly as
    /// `T3Desk.deepLink` makes it (nothing more: no query, no other part); nil for anything else.
    static func t3ThreadLink(_ text: String) -> URL? {
        guard text.utf8.count <= 1024, let parts = URLComponents(string: text), parts.scheme == "t3code", parts.host == "threads",
              parts.user == nil, parts.password == nil, parts.port == nil, parts.query == nil, parts.fragment == nil else { return nil }
        let path = parts.percentEncodedPath.split(separator: "/", omittingEmptySubsequences: false)
        guard path.count == 3, path[0].isEmpty, let thread = path[2].removingPercentEncoding,
              let made = T3Desk.deepLink(thread: thread, link: "https://app.t3.codes/threads/\(path[1])/"),
              made.absoluteString == text else { return nil }
        return made
    }

    /// The jukebox, as the Swift engine's (`Deck.playMusic`, `Deck.setVolume`, `Deck.followMusic`): a key pressed, only
    /// with Accessibility; the volume set; and whether music is playing, and the volume, read afresh.
    private static func music(_ action: String, _ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        switch action {
        case "music":
            guard let key = request["music"] as? String, Music.keys[key] != nil else { return refused("A jukebox has no such key.") }
            guard actions.keyboard() else { return refused(Deck.noKeyboard) }
            return onMain(actions, until: deadline) { actions.pressMusic(key) }
        case "volume":
            guard let level = request["level"] as? Int, (0...100).contains(level) else { return refused("The volume goes from 0 to 100.") }
            return onMain(actions, until: deadline) { actions.setVolume(level) }
        default: // music-state
            let now = actions.musicState()
            return ["ok": true, "playing": now.playing.map { $0 as Any } ?? NSNull(),
                    "volume": now.volume.map { ["level": $0.level, "muted": $0.muted] as Any } ?? NSNull()]
        }
    }

    /// What `tab`'s terminal lets the office do, as its driver's table says (`Terminals.abilities`), and
    /// whether it can say what the tab shows: no terminal is asked.
    static func abilities(of tab: String) -> [String: Bool] {
        let driver = TabAddress(tab).flatMap(Terminals.driver(for:))
        let can = driver?.abilities ?? .none
        return ["keys": can.contains(.keys), "text": can.contains(.text), "fire": can.contains(.fire), "screen": driver?.showsScreen ?? false]
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
    /// a made button's keys, Space, and the digits 1 to 9, a menu's choice picked by its number (desktop/talkpick.go).
    static let keys: [String: Terminal.Keystroke] = Button.keys.merging(["space": .init(key: .space)]) { kept, _ in kept }
        .merging((1...9).compactMap { n in Terminal.Key.digit(n).map { (String(n), Terminal.Keystroke(key: $0)) } }) { kept, _ in kept }

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
            let agent = (request["agent"] as? String).flatMap(Agent.init(rawValue:))
            work = {
                // Dictation's text (desktop/hostdictation.go): only where its agent is in front, as `Deck.typeDictation`
                // looked; unknown is not the same as gone.
                if action == "insert", let agent {
                    switch actions.place(tab, agent) {
                    case .gone?: return ["ok": false, "error": "That agent has ended.", "place": "gone"]
                    case .behind?: return ["ok": false, "error": "That agent isn't in front in its terminal: bring it back with fg.", "place": "behind"]
                    case .front?, nil: break
                    }
                }
                return done(action == "type" ? actions.type(text, tab) : actions.insert(text, tab))
            }
        case "focus":
            work = { done(actions.focus(tab)) }
        case "scroll":
            guard let lines = request["lines"] as? Int, lines != 0 else { return refused("No lines to scroll.") }
            work = { done(actions.scroll(max(-Terminal.maxScroll, min(Terminal.maxScroll, lines)), tab)) }
        case "scroll-bottom":
            work = { done(actions.scrollToBottom(tab)) }
        default: // close
            // In tmux each step has `Tool.timeout`, so a tmux that answers some steps and hangs on others could,
            // in theory, take a close past `deadline`: then it's answered "stopped answering part way", while the
            // close may still finish. It takes several separate hangs; nothing more is done about it.
            guard let name = request["agent"] as? String, let agent = Agent(rawValue: name) else { return refused("Not an agent.") }
            // Strict (FXHIRE B1, a hire in a window of its own, known only by its folder): the tab is closed only
            // with the agent in front there. With only a shell left, the tab may be someone else's on a tty used
            // again, so nothing is touched and it's answered gone; with the agent behind, or unknown, not stopped.
            let strict = request["strict"] as? Bool == true
            work = {
                if strict {
                    switch actions.place(tab, agent) {
                    case .front?: break
                    case .gone?: return ["ok": true, "fired": "gone"]
                    case .behind?, nil: return ["ok": true, "fired": "not-stopped"]
                    }
                }
                switch actions.fire(tab, agent) {
                case .fired?: return ["ok": true, "fired": "fired"]
                case .gone?: return ["ok": true, "fired": "gone"]
                case .notStopped?: return ["ok": true, "fired": "not-stopped"]
                case nil: return unsupported
                }
            }
        }
        return onTabQueue(tab, actions, until: deadline, work)
    }

    /// The phone's text (desktop/phonetext.go, I5): what a paired phone typed or heard, into a Terminal, iTerm2
    /// or Ghostty tab, without Return (the core presses it after the settle, as the Swift engine does), by the
    /// Swift engine's own code (IGL): `Deck.phoneWordsRefusal` before anything waits on the terminal's queue,
    /// then on that queue, once the agent is known to be in front there or that can't be told (as `Deck.toAgent`
    /// looks), `Deck.typePhoneWords`, with the desk's state as the core has it (`deskState`, asked right when the
    /// Swift engine looks at its own sessions). Every no comes with the Swift engine's sentence (`Deck.linesResponse`):
    /// nothing was typed. What may have typed says so (`mayHaveTyped`): ⌘V that the terminal hadn't read in time,
    /// a terminal that didn't take it, or stopped answering part way; the core then sends no Return and never
    /// offers the words again. And the peek: the end of what the tab shows (`PhoneDictation.peek`), only while the
    /// agent is in front, as `Deck.peek` looks, its last `lines` if asked (a pick, up to `PhoneDictation.menuLines`). The words are checked again: as `PhoneDictation.prepared` gives
    /// them, so 2,000 characters at most; no "!" first; no control character but a line feed and a tab.
    private static func phoneAction(_ action: String, _ request: [String: Any], _ actions: Actions, until deadline: DispatchTime,
                                    deskState: @escaping () -> SessionState?) -> [String: Any] {
        guard let text = request["tab"] as? String, let tab = TabAddress(text) else { return refused("Not a tab.") }
        guard let name = request["agent"] as? String, let agent = Agent(rawValue: name) else { return refused("Not an agent.") }
        let work: () -> [String: Any]
        if action == "phone-text" {
            guard let words = request["text"] as? String, PhoneDictation.prepared(words) == words, !PhoneDictation.runsInShell(words),
                  HireTmux.canPaste(words) else { return refused("Not text to type.") }
            guard let driver = actions.driver(tab), driver.abilities.contains(.text) else { return unsupported }
            if let refusal = Deck.phoneWordsRefusal(words, way: driver.linesWay, keysNeedAccessibility: driver.keysNeedAccessibility,
                                                    agent: agent) { return refused(said(refusal)) }
            work = {
                // Unknown is not the same as gone: the words go, as the deck's keys do.
                switch actions.place(tab, agent) {
                case .gone?: return ["ok": false, "error": "That agent has ended.", "place": "gone"]
                case .behind?: return ["ok": false, "error": "That agent isn't in front in its terminal: bring it back with fg.", "place": "behind"]
                case .front?, nil: break
                }
                switch Deck.typePhoneWords(words, driver: driver, tab: tab, agent: agent, state: deskState) {
                case .typed: return ["ok": true, "pasted": "pasted"]
                case .refused(let why): return refused(said(why))
                case .notRead(let why): return ["ok": false, "error": said(why), "mayHaveTyped": true]
                case .failed: return ["ok": false, "error": "The terminal didn't take it.", "mayHaveTyped": true]
                }
            }
        } else {
            work = {
                // Only where the agent is known to be in front: a shell's screen is never read.
                guard actions.place(tab, agent) == .front else {
                    return ["ok": false, "error": "That agent isn't in front in its terminal.", "place": "behind"]
                }
                switch actions.screen(tab) {
                case nil: return unsupported
                case .some(nil): return refused("This terminal can't show its input line.")
                case .some(let screen?):
                    // The phone's peek, or more of it for a pick looking for its menu (desktop/talkpick.go).
                    let lines = min(max(request["lines"] as? Int ?? PhoneDictation.peekLines, 1), PhoneDictation.menuLines)
                    return ["ok": true, "peek": PhoneDictation.peek(screen, lines: lines)]
                }
            }
        }
        return onTabQueue(tab, actions, until: deadline, typing: action == "phone-text", work)
    }

    /// What a response says, as the phone reads it.
    private static func said(_ response: Response) -> String { String(decoding: response.body, as: UTF8.self) }

    /// `work` on `tab`'s own terminal's queue, as the deck's keys go: given no longer to start than they are
    /// (`Terminals.inputWait`), then waited for to its end, within the bridge's own deadline. `typing` the
    /// phone's text, a terminal that stopped answering part way may have typed it (`mayHaveTyped`).
    private static func onTabQueue(_ tab: TabAddress, _ actions: Actions, until deadline: DispatchTime, typing: Bool = false,
                                   _ work: @escaping () -> [String: Any]) -> [String: Any] {
        let tabText = tab.text
        func partWay(_ said: String) -> [String: Any] {
            typing ? ["ok": false, "error": said, "mayHaveTyped": true] : refused(said)
        }
        switch run(on: actions.queue(tab), startBy: min(deadline, .now() + Terminals.inputWait), until: deadline,
                   { Terminal.noticingTimeouts(work) }) {
        case .done(let answer) where answer.timedOut && answer.value["ok"] as? Bool != true:
            let said = Terminals.notAnswering(tabText, partWay: answer.partWay)
            return answer.partWay || answer.value["mayHaveTyped"] as? Bool == true ? partWay(said) : refused(said)
        case .done(let answer): return answer.value
        case .notStarted: return refused(Terminals.notAnswering(tabText))
        case .unfinished: return partWay(Terminals.notAnswering(tabText, partWay: true))
        }
    }

    /// What came of something run on a queue with a deadline.
    enum Timed<T> {
        /// Run, with its answer.
        case done(T)
        /// Not started in time, and never run.
        case notStarted
        /// Started, but still running when the time was up: it's seen through, unheard.
        case unfinished
    }

    /// Runs `body` on `queue`, after what's already there, as the deck runs its
    /// terminals one at a time. One that hasn't started by `startBy` never
    /// runs: its caller is told at once, and a key typed late could be typed
    /// twice. One that has is waited for until `deadline`, so what it did is
    /// what's answered; still running then, it can't be stopped: it finishes,
    /// and its answer goes unheard.
    static func run<T>(on queue: DispatchQueue, startBy: DispatchTime? = nil, until deadline: DispatchTime,
                       _ body: @escaping () -> T) -> Timed<T> {
        let pending = Pending<T>(startBy: min(startBy ?? deadline, deadline))
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
        private let startBy: DispatchTime
        private var started = false
        private var givenUp = false
        private var value: T?
        private var hasValue = false

        init(startBy: DispatchTime) { self.startBy = startBy }

        /// Whether to run it, right before it would: not once its caller has
        /// given up, nor past its time to start.
        func begin() -> Bool {
            lock.lock()
            defer { lock.unlock() }
            guard !givenUp, DispatchTime.now() < startBy else { return false }
            started = true
            return true
        }

        func finish(_ result: T) {
            lock.lock()
            value = result
            hasValue = true
            lock.unlock()
            finished.signal()
        }

        func wait(until deadline: DispatchTime) -> Timed<T> {
            _ = finished.wait(timeout: startBy)
            lock.lock()
            if hasValue, let value {
                lock.unlock()
                return .done(value)
            }
            guard started else {
                givenUp = true
                lock.unlock()
                return .notStarted
            }
            lock.unlock()
            // Started in time: its answer is what it did, however long its keys take.
            _ = finished.wait(timeout: deadline)
            lock.lock()
            defer { lock.unlock() }
            if hasValue, let value { return .done(value) }
            return .unfinished
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
    /// Opened on that terminal's queue, as the deck opens a hire's, and as a tab
    /// action: not at all once the core has given up, so a retry never makes two.
    private static func open(_ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        guard let terminal = request["terminal"] as? String, Openers.choices.contains(where: { $0.id == terminal }) else {
            return refused("Not a terminal Smolcorp opens.")
        }
        guard let folder = request["folder"] as? String, folder.hasPrefix("/"), isFolder(folder) else { return refused("Not a folder.") }
        guard let script = request["script"] as? String, isHireScript(script, in: actions.hires) else {
            return refused("Not a script in Smolcorp's hires folder.")
        }
        switch run(on: actions.windowQueue(terminal), until: deadline, { actions.open(terminal, folder, script) }) {
        case .done(let opened?):
            // The window's own record, as the Swift engine keeps it for a hire (`Deck.hireWindows`): what
            // switching hires to the background closes, by its terminal's id. A tab with no window: none.
            var answer: [String: Any] = ["ok": true, "tab": opened.tab]
            if let window = opened.window {
                answer["window"] = window
                actions.opened.add(window)
            }
            return answer
        case .done(nil): return refused("The window didn't open.")
        case .notStarted, .unfinished: return refused("The terminal didn't answer in time.")
        }
    }

    /// Hires switched to the background under the core (desktop/hirewindows_close.go, as the Swift engine's
    /// `HireWindows.toBackground`): a window this app opened for the core, by the record its `open` answered, looked
    /// at (open on a tty, gone, or it couldn't be asked) and closed by `actions.closer`, as the Swift engine closes
    /// its own: only once nothing but the tab's shell runs on that tty. Never a window it didn't open for the core,
    /// nor one by a name. Each on its terminal's queue, as its window was opened.
    private static func window(_ action: String, _ request: [String: Any], _ actions: Actions, until deadline: DispatchTime) -> [String: Any] {
        guard let window = request["window"] as? String, let parsed = HireWindows.Terminals.parse(window) else {
            return refused("Not a window's record.")
        }
        guard actions.opened.contains(window) else { return refused("Not a window Smolcorp opened for the engine.") }
        let terminal = parsed.kind == "ghostty-id" ? "ghostty" : parsed.kind
        if action == "window-look" {
            switch run(on: actions.windowQueue(terminal), until: deadline, { actions.closer.look(window) }) {
            case .done(.open(let tty)): return ["ok": true, "shown": "open", "tty": tty]
            case .done(.gone):
                actions.opened.remove(window)
                return ["ok": true, "shown": "gone"]
            case .done(.unknown), .notStarted, .unfinished: return ["ok": true, "shown": "unknown"]
            }
        }
        guard let tty = request["tty"] as? String, TabAddress.isTTY(tty) else { return refused("Not a tty.") }
        switch run(on: actions.windowQueue(terminal), until: deadline, { actions.closer.close(window, tty: tty) }) {
        case .done(.closed):
            actions.opened.remove(window)
            return ["ok": true, "closed": true]
        case .done(.left(let why)): return ["ok": true, "closed": false, "left": why]
        case .notStarted, .unfinished: return refused("The terminal didn't answer in time.")
        }
    }

    /// The windows this app opened for the core, by their records: the only ones it looks at or closes for it.
    final class OpenedWindows {
        private let lock = NSLock()
        private var records: Set<String> = []

        func add(_ record: String) { lock.lock(); records.insert(record); lock.unlock() }
        func remove(_ record: String) { lock.lock(); records.remove(record); lock.unlock() }
        func contains(_ record: String) -> Bool { lock.lock(); defer { lock.unlock() }; return records.contains(record) }
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
            if let look = store.look {
                switch look() {
                case .token(let value) where !value.isEmpty: return ["ok": true, "found": true, "value": value]
                case .failed: return ["ok": true, "found": false, "failed": true]
                default: return ["ok": true, "found": false]
                }
            }
            guard let value = store.read() else {
                // There but unreadable (a denied prompt, a damaged item) is never "none": the core would make
                // a new one over it.
                if let absent = store.absent, !absent() { return refused("It couldn't be read.") }
                return ["ok": true, "found": false]
            }
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

    /// The record key (RecordKey.swift), for the core it started alone: what the item is now (`look`, with its value
    /// only when there is exactly one untouched item, or two for the core to tell an interrupted update from a planted
    /// item), a new one in place of every item (`new`, "update" at the update to 1.26), one beside it (`add`, read
    /// back before it's handed on) and the old one deleted (`retire`).
    private static func recordKey(_ action: String, _ request: [String: Any], _ store: RecordKeyStore) -> [String: Any] {
        switch action {
        case "record-key-new", "record-key-add":
            guard let item = action == "record-key-new" ? store.makeNew() : store.add() else { return refused("The Keychain made no key.") }
            // A new record key at the update to 1.26 or a reset: the items' key too, where it must be (ItemSeal).
            if action == "record-key-new" { store.renewItems(update: request["item"] as? String == "update") }
            return ["ok": true, "recordKeys": [item.json]]
        case "record-key-retire":
            guard let id = request["item"] as? String, !id.isEmpty, store.retire(id) else { return refused("That key wasn't deleted.") }
            return ["ok": true]
        default:
            switch store.look() {
            case .one(let item): return ["ok": true, "records": "one", "recordKeys": [item.json]]
            case .many(let items): return ["ok": true, "records": "many", "recordKeys": items.map(\.json)]
            case .none: return ["ok": true, "records": "none"]
            case .moved: return ["ok": true, "records": "moved"]
            case .unreadable: return ["ok": true, "records": "unreadable"]
            }
        }
    }

    /// The API-key account for a hire (C23), as the Swift engine's `Deck.hire` has it, for setup's state and for one
    /// hire on it. `api-key-offer` never reads the key: whether it's offered, the notice beside the choice, and,
    /// when it isn't, the refusal a hire on it gets, all in this app's words (a test copy's own among them).
    /// `api-key-read` is the key itself, only while it's offered, for the hire being made: the core puts it in that
    /// hire's start script alone (desktop/hireapikey.go). It's the item `secret-read` gives too, to the same core.
    private static func hireKey(_ action: String, _ key: HireKey) -> [String: Any] {
        let offered = key.offered()
        let refusal = HireDesk.apiKeyRefusal(picked: true, offered: offered, testCopy: key.testCopy)
        guard action == "api-key-read" else {
            var answer: [String: Any] = ["ok": true, "available": offered, "notice": HireDesk.apiKeyNoticeText(testCopy: key.testCopy)]
            if let refusal { answer["refusal"] = refusal.message }
            return answer
        }
        if let refusal { return refused(refusal.message) }
        guard let value = key.read() else { return refused(HireDesk.keyRefusalText(testCopy: key.testCopy).message) }
        return ["ok": true, "found": true, "value": value]
    }

    // MARK: What each action does

    /// The API-key account a hire may work on: the Summary key, offered only from the Keychain (a test copy's own
    /// file there), never the environment's (`HireDesk.apiKeyOffered`).
    struct HireKey {
        var offered: () -> Bool = { HireDesk.apiKeyOffered() }
        var testCopy = TestCopy.isOn
        var read: () -> String? = { SummaryKey.store.read() }
    }

    /// One of the app's own items: where it's kept, as the Swift engine keeps it. `failed`, for the GM's token
    /// alone, says whether the item is there but couldn't be read (a locked Keychain), which the core must not take
    /// for "none": it would make a new token and write it over the item (`desktop/gmtokenstore.go`).
    struct Secret {
        var read: () -> String?
        var write: (String) -> Bool
        var delete: (() -> Void)?
        /// Whether there's surely none, asked when `read` gives nothing: nil where that's always none.
        var absent: (() -> Bool)? = nil
        var look: (() -> GMTokenRead)? = nil
    }

    /// The app's code each action runs: `mac` in the app, stand-ins in the tests.
    /// A tab action is nil for a terminal the app can't drive.
    struct Actions {
        var press: (Terminal.Keystroke, TabAddress) -> Bool?
        var type: (String, TabAddress) -> Bool?
        var insert: (String, TabAddress) -> Bool?
        /// The driver the phone's text goes through (`Deck.typePhoneWords`): nil for a terminal the app can't drive.
        var driver: (TabAddress) -> TerminalDriver? = { Terminals.driver(for: $0) }
        /// What the tab shows: nil for a terminal the app can't drive, `.some(nil)` where it can't say.
        var screen: (TabAddress) -> String?? = { tab in Terminals.driver(for: tab).map { $0.screen(of: tab) } }
        /// Where `agent` is in the tab, as the deck looks before its keys: nil if that couldn't be looked at.
        var place: (TabAddress, Agent) -> Terminal.Place? = { tab, agent in
            Terminals.tty(of: tab.text).flatMap { Terminal.place(of: agent, on: $0) }
        }
        var focus: (TabAddress) -> Bool?
        var scroll: (Int, TabAddress) -> Bool?
        var scrollToBottom: (TabAddress) -> Bool?
        var fire: (TabAddress, Agent) -> Terminal.Fired?
        /// Whether the agent has left its tab, known for sure (`agentEnded`), given whether this is a test copy.
        var ended: (TabAddress, Agent, _ testCopy: Bool) -> Bool = HostBridge.agentEnded
        /// The ttys of Terminal's open tabs (`Terminal.openTTYs`), nil if it couldn't be asked: for `agents-ended`.
        var openTTYs: () -> Set<String>? = { Terminal.openTTYs() }
        var hires: URL
        var open: (_ terminal: String, _ folder: String, _ script: String) -> Openers.Opened?
        /// The ids of `Openers.choices` installed on this Mac.
        var terminals: () -> [String] = { Openers.choices.filter { Openers.isInstalled($0.id) }.map(\.id) }
        /// The windows opened for the core (`open`), and what looks at and closes them, as the Swift engine's own.
        var opened = OpenedWindows()
        var closer: HireWindows.Closer = HireWindows.Terminals()
        var secrets: [String: Secret]
        /// The API-key account for a hire, as the Swift engine has it: stand-ins in the tests.
        var hireKey = HireKey()
        /// The login item, or nil for an app without one.
        var loginItem: (get: () -> Bool, set: (Bool) -> Void)?
        /// A test copy: its login item is the copy in use's, read and never set.
        var testCopy = false
        /// Where a tab's actions run, one at a time: its terminal's queue, as the deck's keys go.
        var queue: (TabAddress?) -> DispatchQueue = { Terminals.queue(for: $0) }
        /// Where a window of a terminal (an `Openers` choice) opens: that terminal's queue, as the deck opens a hire's.
        var windowQueue: (String) -> DispatchQueue = { Terminals.queue(opening: $0) }
        /// Where setup's Mac parts run, as the Swift engine's routes do: the main queue.
        var main: DispatchQueue = .main
        /// Setup's facts of this Mac (see desktop/hostbridge.go's macFacts), given the apps keys may go to.
        var facts: (_ apps: [String]) -> [String: Any] = { _ in [:] }
        /// The checklist's install line (`SelfTest.installCheck`), and its fix: the app moved to Applications.
        var installCheck: () -> Check = { SelfTest.installCheck() }
        /// Refuses unless `onThisMac` supplies the real one, so no `Actions` built by hand can move a bundle.
        var moveToApplications: () throws -> Void = { throw SelfTest.FixError.testCopy }
        var dock: (Bool) -> Void = { _ in }
        var askAccessibility: () -> Void = {}
        /// Keys in other apps switched on: Accessibility asked for if it isn't given, but never by a test copy.
        var keystrokesOn: () -> Void = {}
        /// The microphone asked for, and the language's model got ready: why not, or nil.
        var prepareDictation: (_ language: String, _ done: @escaping (String?) -> Void) -> Void = { _, done in done(nil) }
        /// The office's mic (desktop/hostdictation.go), on main: recording started in a language for a tab, and whether
        /// it did (`Dictation.start`); stopped, with what was heard, "" for nothing; and that text cleaned up, given
        /// the desks, whether to, and the glossary (`DictationCleanup.clean`).
        var startDictation: (_ language: String, _ tab: String, _ done: @escaping (Bool) -> Void) -> Void = { _, _, done in done(false) }
        var stopDictation: (_ done: @escaping (String) -> Void) -> Void = { done in done("") }
        var cleanDictation: (_ text: String, _ desks: [String], _ cleanup: Bool, _ glossary: [String],
                             _ done: @escaping (String, DictationCleanup.Outcome) -> Void) -> Void = { text, _, _, _, done in done(text, .off) }
        /// Claude's clean-up switched off, on main: its last problem forgotten.
        var clearCleanupProblem: () -> Void = {}
        /// Add smolcorp to Terminal's link made (true) or removed: why not, thrown.
        var terminalCommand: (Bool) throws -> Void = { _ in }
        var abilities: (String) -> [String: Bool] = HostBridge.abilities(of:)
        /// The Grok Bot app's bots now, read afresh: nil when its roster can't be read.
        var roster: () -> [GrokBotApp.Row]? = { nil }
        /// The office's window shown, and kept on top or not: the Go shell's own (GoShellDelegate), nothing elsewhere.
        var showOffice: () -> Void = {}
        var officeOnTop: (Bool) -> Void = { _ in }
        /// The office's keys held instead, or none (nil): why not, or nil once they're held (`OfficeKeyHolder`).
        var officeKey: (OfficeKey?) -> String? = { _ in nil }
        /// Each key's name on the layout in use (`OfficeKey.currentLabels`), on main: the Go shell's own.
        var officeKeyLabels: () -> OfficeKey.Labels = { [:] }
        /// The folder panel opened at a folder (nil: its own choice), saying a message, and what was picked, or nil.
        var chooseFolder: (_ start: String?, _ message: String, _ done: @escaping (String?) -> Void) -> Void = { _, _, done in done(nil) }
        /// The folder panel closed if it's open saying a message, as if cancelled.
        var closeFolderPanel: (_ message: String) -> Void = { _ in }
        /// The processes working in a folder or naming a session, with their names and why (`Processes.holding`):
        /// nil when the Mac can't look. Read from the kernel, nothing signalled.
        var processes: (_ folder: String, _ session: String) -> [(pid: pid_t, name: String, why: String)]? = { folder, session in
            Processes.holding(folder, session: session)?.map { ($0.pid, Processes.name(of: $0.pid), $0.why.rawValue) }
        }
        /// The worktrees of the hires open in the core's registry (`HostBridge.openWorktrees`): the only folders
        /// `processes` looks at. Nil when it can't be read.
        var openWorktrees: () -> [String]? = { HostBridge.openWorktrees(in: Paths.support.appendingPathComponent("registry.json")) }
        /// Where processes and T3 Code's app are looked at: off the main queue and the terminals'.
        var looking: DispatchQueue = .global(qos: .utility)
        /// T3 Code's app, as the Swift engine's Open in T3 Code finds it, opens it and hands it a thread's link.
        var t3App: () -> Bool = { T3Code.Machine().appInstalled() }
        var t3OpenApp: () -> Void = { T3Code.Machine().openApp() }
        var t3OpenLink: (URL) -> Bool = { T3Code.Machine().openURL($0) }
        /// The jukebox (`Music`): whether its keys may be pressed (Accessibility), a key pressed, whether music is
        /// playing and the volume, and the volume set.
        var keyboard: () -> Bool = { false }
        var pressMusic: (String) -> Void = { _ in }
        var musicState: () -> (playing: Bool?, volume: (level: Int, muted: Bool)?) = { (nil, nil) }
        var setVolume: (Int) -> Void = { _ in }
        /// How this Mac writes a short time (`shortTimeFormat`).
        var timeFormat: () -> [String: String] = { HostBridge.shortTimeFormat() }
        /// When you're back at the Mac (`MacPresence`): the keyboard's and mouse's idle time, whether the screen is
        /// locked, and the unlocks and wakes counted.
        var presence: () -> MacPresence.Now = { MacPresence.Now() }
        /// The GM's tiling (`tiling`): the Mac's screens, the AppleScript the windows are moved with (a stand-in in the
        /// tests: nothing there ever drives a terminal), and the office's window put at a place in AppleScript's
        /// coordinates, turned over by the main screen's height (the Go shell's, OfficeLink).
        var screens: () -> [[String: Any]] = { Tiler.screens() }
        var tileScript: Tiler.Script = Tiler.osascript
        var placeOffice: (_ rect: CGRect, _ mainHeight: CGFloat) -> Void = { _, _ in }
        /// The record key's item (RecordKey.swift): the Keychain, or a test copy's and the test suite's own file.
        var records: RecordKeyStore = RecordKeys.defaultStore()

        /// The Swift engine's own: the terminal drivers on their terminals' queues, one at a time
        /// as the deck runs them, the openers, the secrets' stores (files in a test copy), the login item.
        static var mac: Actions { onThisMac() }

        /// `mac`, with what a test changes: whether this is a test copy, the login item, and the opener
        /// (`Openers.open`, given the command, the folder and the terminal).
        static func onThisMac(testCopy: Bool = TestCopy.isOn,
                              loginItem: (get: () -> Bool, set: (Bool) -> Void) = (get: { LoginItem.isEnabled }, set: { LoginItem.isEnabled = $0 }),
                              opener: @escaping (_ command: String, _ folder: String, _ terminal: String) -> Openers.Opened? = { command, folder, terminal in
                                  Openers.open(command, in: folder, terminal: terminal, inTmux: false, hireID: "")
                              }) -> Actions {
            // On `queue`, already: see `HostBridge.run`.
            func drive<T>(_ tab: TabAddress, _ body: (TerminalDriver) -> T) -> T? {
                guard let driver = Terminals.driver(for: tab) else { return nil }
                return body(driver)
            }
            let viewer = ViewerStore.defaultStore()
            // The portable office's, as the Swift engine kept them: never removed, and one there but unreadable
            // is the app's no, never "none" (PortableOffice.identity at 1da4e935).
            func viewerItem(_ account: ViewerKeychain.Account) -> Secret {
                Secret(read: { viewer.read(account) }, write: { viewer.write($0, account) }, delete: nil,
                       absent: { viewer.absent(account) })
            }
            let gm = GMIdentity.defaultStore()
            let push = PushKeyStore.defaultStore()
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
                    }, write: { gm.write($0) }, delete: { gm.remove() }, look: { gm.read() }),
                    "viewer-id": viewerItem(.id),
                    "viewer-key": viewerItem(.key),
                    "viewer-write-secret": viewerItem(.writeSecret),
                    // The Macs this one joined, keys and all (desktop/peers_mac_join.go): one item, read only as the
                    // app added it, and none only when the Keychain says so (`PeerKeys`).
                    "peer-keys": PeerKeys.defaultStore().secret,
                    // The VAPID key notifications are signed with (desktop/pushkey.go): never removed this way.
                    "push-key": push.secret,
                ],
                loginItem: loginItem,
                testCopy: testCopy,
                facts: { apps in
                    [
                        "accessibility": Permissions.hasAccessibility,
                        "testCopy": testCopy,
                        "dictation": [
                            "available": Dictation.isAvailable, "allowed": Dictation.microphoneAllowed,
                            "languages": Dictation.languages, "suggested": Dictation.suggestedLanguage,
                            "glossary": DictationCleanup.defaultGlossary, "cleanupProblem": DictationCleanup.problem ?? NSNull(),
                        ] as [String: Any],
                        "running": Keystrokes.runningApps(except: apps),
                        "terminalCommand": TerminalCommand.json(),
                        "signingKeyUnlocked": SigningKey.fact(testCopy: testCopy),
                    ]
                },
                installCheck: { SelfTest.installCheck(testCopy: testCopy) },
                // A test copy never moves its bundle, whatever the global switch says (so a test of `onThisMac(testCopy: true)` can't reach /Applications).
                // Only the Mac's own app moves its bundle: never a test copy, nor the test suite's bundle.
                moveToApplications: { if testCopy || Paths.underTest { throw SelfTest.FixError.testCopy }; try SelfTest.moveToApplications() },
                dock: { Dock.show($0) },
                askAccessibility: { Permissions.requestAccessibility() },
                keystrokesOn: { if !testCopy, !Permissions.hasAccessibility { Permissions.requestAccessibility() } },
                prepareDictation: { language, done in
                    Dictation.askForMicrophone { _ in Dictation.prepare(language, done: done) }
                },
                startDictation: { language, tab, done in CoreDictation.start(language, tab, done) },
                stopDictation: { CoreDictation.stop($0) },
                cleanDictation: { text, desks, cleanup, glossary, done in
                    DictationCleanup.clean(text, desks: desks, enabled: cleanup, configured: glossary, done: done)
                },
                clearCleanupProblem: { DictationCleanup.problem = nil },
                terminalCommand: { add in
                    guard !testCopy else { throw TerminalCommand.Problem.testCopy }
                    if add { try TerminalCommand.add() } else { try TerminalCommand.remove() }
                },
                roster: { GrokBotApp.rows() },
                chooseFolder: { start, message, done in Deck.chooseFolder(from: start.flatMap(Deck.folder), message: message, then: done) },
                closeFolderPanel: { Deck.closeFolderPanel(for: $0) },
                keyboard: { Permissions.hasAccessibility },
                pressMusic: { Music.press($0) },
                musicState: { (Music.isPlaying(), Music.volume()) },
                setVolume: { Music.setVolume($0) },
                presence: { MacPresence.shared.now() })
        }
    }
}
