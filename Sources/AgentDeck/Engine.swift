import Foundation

/// The engine that runs the deck: since 1.25.0, on every Mac, the Go core, the
/// same server the Linux and Windows app runs, started by this app as its child
/// (`GoCore`), with this app as its shell: the menu bar, the windows, and what
/// only macOS can do (`HostBridge`). The Swift engine this app had before is
/// gone, so config.json's `engine` no longer chooses anything.
enum Engine {
    /// The line to log once, as the app starts, when config.json still names
    /// an engine other than "go" (a test copy's "swift", say): the field is
    /// left in the file as it is, and ignored. Nil when it names none, or "go".
    static func ignored(_ setting: String?) -> String? {
        guard let setting, setting != "go" else { return nil }
        return "config.json's \"engine\": \"\(setting)\" is ignored: Smolcorp runs its Go engine, the only one it has since 1.25.0."
    }
}

/// The Go core, run as this app's child: `bin/smolcorp core`, bundled and
/// signed with the app. It's told its port, the support folder and the
/// bridge's socket on its command line, and the bridge's token as the first
/// line on its standard input: never in its arguments or its environment,
/// which other processes of yours can read (`ps -E`). It says it's ready on
/// its standard output; this app then asks it, a line of JSON each way, for
/// the office window's key and for setup sessions: only this app holds that
/// pipe's other end. It asks what the menus do too (`Op`), and the core sends,
/// unasked, the menu's state whenever it changes (`onMenu`). The bridge serves this core's pid alone, and the core
/// sends the token only to its parent, this app (see `HostBridge`). When this
/// app quits, or dies, the core's input ends and it stops: `stop` also makes
/// sure, by its pid.
final class GoCore {
    enum Failure: LocalizedError {
        case noProgram, didNotStart(String), notReady(String), portInUse(UInt16)

        var errorDescription: String? {
            switch self {
            case .noProgram: return "This build of Smolcorp has no Go engine inside it (bin/smolcorp)."
            case .didNotStart(let why): return "The Go engine didn't start: \(why)"
            case .notReady(let why): return "The Go engine didn't get ready: \(why)"
            case .portInUse(let port): return "Port \(port) is in use. Is Smolcorp already running?"
            }
        }
    }

    /// What this app may ask the core, as Tests/fixtures/shell/ops.json lists them for both sides.
    enum Op: String, CaseIterable {
        case office, setupSession = "setup-session", setupLink = "setup-link"
        // The menu bar menu's (GoShellDelegate).
        case menu, guideAgain = "guide-again", unpairAll = "unpair-all", portableStart = "portable-start"
        case portableStop = "portable-stop", viewerLink = "viewer-link", officeOnTop = "office-ontop"
    }

    /// The status the core ends with when its port is taken (EADDRINUSE as macOS numbers it; `corePortInUse`
    /// in desktop/core_darwin.go): `start` then throws `Failure.portInUse`, for the shell to say so and try again.
    static let portInUseStatus: Int32 = 48

    let program: URL
    let port: UInt16
    let support: URL
    let bridge: HostBridge
    /// The name the core gives its setup page's session cookie, as it said when ready.
    private(set) var setupCookie = "smolcorp-setup"
    /// Told on the main queue if the core stops by itself, with its exit status.
    var onExit: (Int32) -> Void = { _ in }
    /// Told on the main queue the menu's state the core sends unasked, as it changes.
    var onMenu: (ShellMenu) -> Void = { _ in }

    private var process: Process?
    private var input: FileHandle?
    /// Lines from the core, and the waits for them. Guarded by `lock`.
    private var buffer = Data()
    private var lines: [[String: Any]] = []
    private let arrived = DispatchSemaphore(value: 0)
    private let lock = NSLock()
    /// One request at a time, so each answer is the one asked for.
    private let asking = NSLock()
    private var nextID = 1
    private var stopping = false
    /// A `menu` question was written and nothing has come from the core since (no answer, no push). Guarded by
    /// `lock`. The menu, opening on the main queue, doesn't write again while it's set: a core that is alive but
    /// stuck would otherwise leave one more unread request on the pipe at each opening, until it's full and a write blocks.
    private var menuUnanswered = false
    /// Whether this start's core has said it's ready: one that ends before that is `start`'s failure to
    /// throw, not an engine that "stopped by itself".
    private var served = false
    /// The status of a core that ended after saying ready, but before `start` had taken note of it.
    private var endedWhileStarting: Int32?

    init(program: URL = Paths.bundledCLI, port: UInt16, support: URL, bridge: HostBridge) {
        self.program = program
        self.port = port
        self.support = support
        self.bridge = bridge
    }

    /// The pid of the running core, if any.
    var pid: pid_t? { process.map(\.processIdentifier) }

    /// What the core is started with: never the token, which goes on its input.
    var arguments: [String] {
        ["core", "--port", String(port), "--support", support.path, "--bridge", bridge.socket.path]
    }

    /// What the core keeps of this app's environment: what a program needs to
    /// find yours and its tools (`SHELL` gives your login shell's PATH), and
    /// what a hire's pane gets from the Mac on the Swift engine and needs to
    /// work the same: your ssh agent (`git push` over SSH), `LOGNAME`, the text
    /// encoding and the locale's own variables, and `TMUX_TMPDIR`, so both
    /// engines' hires are in one tmux folder; and `agentPlaces`, so the core
    /// holds an agent's folder you moved. Nothing else. A Smolcorp opened from
    /// a terminal would otherwise hand the core that terminal's whole
    /// environment, its tokens too.
    static let inherited = ["PATH", "HOME", "USER", "LOGNAME", "LANG", "TMPDIR", "SHELL", "SSH_AUTH_SOCK",
                            "__CF_USER_TEXT_ENCODING", "TMUX_TMPDIR",
                            // The locale's categories, a closed list: an open `LC_*` would pass
                            // LC_TERMINAL (iTerm2) and whatever ssh's AcceptEnv carries.
                            "LC_ALL", "LC_COLLATE", "LC_CTYPE", "LC_MESSAGES", "LC_MONETARY", "LC_NUMERIC", "LC_TIME"] + agentPlaces

    /// The variables that move an agent's own folder out of the home folder (`CODEX_HOME`, `GEMINI_CLI_HOME`,
    /// `GROK_HOME`, the `XDG_*_HOME` OpenCode reads, and the like), or name a file it takes settings or a sign-in
    /// from: places, never secrets. The core reads them at each desk's start and holds where they point in the desk's
    /// sandbox, as it holds `~/.codex` (FXAGENTHOMEMAC). Without them here it saw none, and held only the default places.
    /// The core's own list, in its order: agentHomeVariables, then agentSignInVariables (internal/sandbox/policy.go);
    /// a Go row fails if the two differ (TestTheMacAppPassesTheCoreEveryAgentPlaceVariable). Last, `ZDOTDIR`, where
    /// zsh's start files are when you moved them: the core holds them there too (FXSWEEP3P1).
    static let agentPlaces = ["CODEX_HOME", "CODEX_SQLITE_HOME", "GEMINI_CLI_HOME", "GEMINI_CLI_TRUSTED_FOLDERS_PATH",
                              "GEMINI_CLI_SYSTEM_SETTINGS_PATH", "GEMINI_CLI_SYSTEM_DEFAULTS_PATH", "GROK_HOME", "GROK_CONFIG_PATH",
                              "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "OPENCODE_CONFIG_DIR",
                              "OPENCODE_CONFIG",
                              "GROK_AUTH_PATH", "GROK_TRACE_UPLOAD_CREDENTIALS_FILE", "OPENCODE_DB",
                              "ZDOTDIR"]

    /// The core's environment: `inherited` from this app's, with the port and folder
    /// a test copy's hires and `smolcorp gm` read, as the Swift engine gives them, and
    /// SMOLCORP_TEST_LISTEN=lan if the copy was opened with it (the core listens on this
    /// Mac only otherwise, as `TestCopy.listensOn`). Never the token.
    /// With the names of `claudeNames` too, never their values: see there.
    /// An `agentPlaces` variable only when it's a whole path (FXAGENTHOMEMACR2): a desk's sandbox holds
    /// nothing else, and git and the sign-ins would read a relative one in the temp folder, where desks write.
    /// Nor one spelled with a `.` or `..` in it, or a doubled `/` (FXAGENTHOMEMACR3): a sandbox holds the place
    /// as cleaned, while the Mac takes `..` after a link a desk could make. Its name alone goes, in
    /// `agentPlacesDroppedVariable`, so the core refuses every desk's start in words that name it.
    func environment(base: [String: String] = ProcessInfo.processInfo.environment) -> [String: String] {
        var environment = base.filter {
            GoCore.inherited.contains($0.key) && (!GoCore.agentPlaces.contains($0.key) || GoCore.isCleanWholePath($0.value))
        }
        environment["SMOLCORP_PORT"] = String(port)
        environment["SMOLCORP_TEST_SUPPORT_DIR"] = support.path
        if TestCopy.wantsLAN(base) { environment["SMOLCORP_TEST_LISTEN"] = "lan" }
        let names = GoCore.claudeNames(base)
        if !names.isEmpty { environment[GoCore.claudeNamesVariable] = names.joined(separator: ",") }
        let dropped = GoCore.agentPlaces.filter { base[$0].map { $0.utf8.first == GoCore.slash && !GoCore.isCleanWholePath($0) } ?? false }
        if !dropped.isEmpty { environment[GoCore.agentPlacesDroppedVariable] = dropped.joined(separator: ",") }
        return environment
    }

    /// Whether value is a whole path with no `.` or `..` in it and no doubled `/` (one or more at its end aside),
    /// as the core's `sandbox.AgentPlaceUnclean` tells it. Read by its UTF-8 bytes, as the core and the Mac read a
    /// path: a String's Characters would join a `/` or `.` with a combining mark after it, so `/x/../\u{301}b` would
    /// pass here while the Mac takes its `..` (RT126AGENTHOMEMACR3 N1).
    static func isCleanWholePath(_ value: String) -> Bool {
        var bytes = Array(value.utf8)
        guard bytes.first == slash else { return false }
        while bytes.count > 1 && bytes.last == slash { bytes.removeLast() }
        if bytes == [slash] { return true }
        return !bytes.dropFirst().split(separator: slash, omittingEmptySubsequences: false)
            .contains { $0.isEmpty || $0.elementsEqual([dot]) || $0.elementsEqual([dot, dot]) }
    }

    /// `/` and `.` as UTF-8 bytes, for isCleanWholePath and environment.
    private static let slash = UInt8(ascii: "/"), dot = UInt8(ascii: ".")

    /// The core's `agentPlacesDroppedVariable` (desktop/desksandbox.go).
    static let agentPlacesDroppedVariable = "SMOLCORP_AGENT_PLACES_DROPPED"

    /// The names of the CLAUDECODE and CLAUDE_CODE_* variables this app was opened with, sorted: a Smolcorp
    /// started from a terminal inside a Claude Code session has that session's. The core never gets them, as
    /// nothing outside `inherited`; it's told their names alone, in `claudeNamesVariable`, and decides which
    /// are a session's (internal/claudeenv), so Setup can say why its desks ignore them (desktop/claudesession.go).
    static func claudeNames(_ base: [String: String]) -> [String] {
        base.keys.filter { $0.uppercased() == "CLAUDECODE" || $0.uppercased().hasPrefix("CLAUDE_CODE_") }.sorted()
    }

    /// The core's `claudeSessionVariable` (desktop/claudesession.go).
    static let claudeNamesVariable = "SMOLCORP_CLAUDE_NAMES"

    /// The longest Keychain time `start` doesn't count: a prompt nobody answers, or a Keychain that never does,
    /// still ends in the menu's line.
    static let keychainWaitMost: TimeInterval = 600

    /// Starts the core and waits until it serves, or fails. Off the main queue. The wait doesn't run while the
    /// bridge answers one of the core's Keychain requests (`HostBridge.keychain`), up to `keychainWaitMost`: the
    /// core waits for that answer before it says it's ready, a prompt waits for you, and the update to 1.26 does
    /// seconds of Keychain work in one (FXUPGRADESTART126).
    func start(waitingUpTo wait: TimeInterval = 20) throws {
        guard FileManager.default.isExecutableFile(atPath: program.path) else { throw Failure.noProgram }
        // Started again after a port that was taken: nothing of the last try is kept.
        lock.lock()
        stopping = false
        served = false
        endedWhileStarting = nil
        menuUnanswered = false
        buffer.removeAll()
        lines.removeAll()
        lock.unlock()
        let process = Process()
        process.executableURL = program
        process.arguments = arguments
        process.environment = environment()
        process.currentDirectoryURL = support
        // A Keychain request of a core started before this one may still be open: its time from now is counted too.
        let keychainBefore = bridge.keychain.seconds
        let launched = Date()
        let toCore = Pipe(), fromCore = Pipe()
        process.standardInput = toCore
        process.standardOutput = fromCore
        // Its messages go where this app's go.
        process.standardError = FileHandle.standardError
        fromCore.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            if data.isEmpty { handle.readabilityHandler = nil }
            self?.received(data)
        }
        process.terminationHandler = { [weak self] ended in
            guard let self else { return }
            self.lock.lock()
            // A core this app stopped, one a newer start has taken the place of, and one that ended before it
            // was ready (`start` throws that) aren't "stopped by itself".
            let current = !self.stopping && self.process === ended
            let expected = !current || !self.served
            if current && !self.served { self.endedWhileStarting = ended.terminationStatus }
            self.lock.unlock()
            // Its pid may be another process's soon: the bridge serves no one now.
            if self.bridge.core == ended.processIdentifier { self.bridge.core = nil }
            self.arrived.signal()
            if !expected { DispatchQueue.main.async { self.onExit(ended.terminationStatus) } }
        }
        // Set before it runs, under the lock its end handler takes: it can't end unseen.
        lock.lock()
        self.process = process
        lock.unlock()
        do { try process.run() } catch {
            lock.lock()
            self.process = nil
            lock.unlock()
            throw Failure.didNotStart(error.localizedDescription)
        }
        // The only process the bridge serves, from before the core has its token.
        bridge.core = process.processIdentifier
        input = toCore.fileHandleForWriting
        // A core that has ended mustn't raise SIGPIPE in this app as it's written to.
        _ = fcntl(toCore.fileHandleForWriting.fileDescriptor, F_SETNOSIGPIPE, 1)
        // The token first, before anything else is said on that pipe: the core reads it before its `hello`.
        // Not written only when the core has already ended, which the wait below says.
        try? toCore.fileHandleForWriting.write(contentsOf: Data((bridge.token + "\n").utf8))
        let deadline = { [bridge] in
            launched.addingTimeInterval(wait + min(GoCore.keychainWaitMost, bridge.keychain.seconds - keychainBefore))
        }
        guard let ready = nextLine(until: deadline), ready["ready"] as? Bool == true else {
            // The Keychain's time included, as it was waited for.
            let why = process.isRunning ? "no answer in \(Int(deadline().timeIntervalSince(launched))) s" : "it ended (\(process.terminationStatus))"
            let ended = process.isRunning ? nil : process.terminationStatus
            _ = stop()
            if ended == GoCore.portInUseStatus { throw Failure.portInUse(port) }
            throw Failure.notReady(why)
        }
        lock.lock()
        served = true
        let early = endedWhileStarting
        lock.unlock()
        // It said ready and then ended, in the moment before this: told as any other stop is.
        if let early { DispatchQueue.main.async { self.onExit(early) } }
        if let cookie = ready["cookie"] as? String, !cookie.isEmpty { setupCookie = cookie }
    }

    /// Asks the core `op` and waits a moment for its answer: nil without one. `queueing` is how long it waits
    /// for an earlier question to be answered first: the menu, opening, waits only a moment.
    func ask(_ op: Op, then: String? = nil, on: Bool? = nil, waitingUpTo wait: TimeInterval = 3,
             queueing: TimeInterval = 60) -> [String: Any]? {
        guard asking.lock(before: Date().addingTimeInterval(queueing)) else { return nil }
        defer { asking.unlock() }
        guard let input, process?.isRunning == true else { return nil }
        let id = nextID
        nextID += 1
        var request: [String: Any] = ["id": id, "op": op.rawValue]
        if let then { request["then"] = then }
        if let on { request["on"] = on }
        guard var line = try? JSONSerialization.data(withJSONObject: request) else { return nil }
        line.append(0x0A)
        if op == .menu { setMenuUnanswered(true) }
        do { try input.write(contentsOf: line) } catch {
            if op == .menu { setMenuUnanswered(false) }
            return nil
        }
        let deadline = Date().addingTimeInterval(wait)
        // An answer to an earlier request that came too late is passed over.
        while let answer = nextLine(until: { deadline }) {
            if answer["id"] as? Int == id { return answer["error"] == nil ? answer : nil }
        }
        return nil
    }

    /// `op` off the main queue, as the menus ask, then `done` on the main queue with the answer, or nil.
    func tell(_ op: Op, on: Bool? = nil, then done: @escaping ([String: Any]?) -> Void = { _ in }) {
        DispatchQueue.global().async { [self] in
            let answer = ask(op, on: on)
            DispatchQueue.main.async { done(answer) }
        }
    }

    /// The menu's state now, for the menu as it opens: nil if the core doesn't answer at once.
    /// Not asked while an earlier question of the menu's is unanswered (see `menuUnanswered`): the last push stands.
    func menu(waitingUpTo wait: TimeInterval) -> ShellMenu? {
        guard !isMenuUnanswered else { return nil }
        return ShellMenu(ask(.menu, waitingUpTo: wait, queueing: wait)?["menu"])
    }

    var isMenuUnanswered: Bool {
        lock.lock(); defer { lock.unlock() }
        return menuUnanswered
    }

    private func setMenuUnanswered(_ value: Bool) {
        lock.lock(); menuUnanswered = value; lock.unlock()
    }

    /// A new setup session for the Settings window, and its cookie's name.
    func setupSession() -> (session: String, cookie: String)? {
        guard let answer = ask(.setupSession), let session = answer["session"] as? String,
              let cookie = answer["cookie"] as? String, !session.isEmpty, !cookie.isEmpty else { return nil }
        return (session, cookie)
    }

    /// The office's window, with the office's own key on this Mac, as the Swift engine's.
    func officeAddress() -> URL? {
        guard let key = ask(.office)?["key"] as? String, !key.isEmpty,
              let encoded = key.addingPercentEncoding(withAllowedCharacters: .alphanumerics) else { return nil }
        return URL(string: "http://127.0.0.1:\(port)/office?window&t=\(encoded)")
    }

    /// A link to setup that works once, for the browser.
    func setupLink(then: String? = nil) -> URL? {
        (ask(.setupLink, then: then ?? "")?["link"] as? String).flatMap(URL.init(string:))
    }

    /// Stops the core: its input ends, which it takes as this app quitting;
    /// then SIGTERM, then SIGKILL, each given a moment. True once its pid is
    /// checked gone. On quit, so it waits.
    @discardableResult
    func stop(grace: TimeInterval = 3) -> Bool {
        lock.lock()
        stopping = true
        lock.unlock()
        guard let process else { return true }
        let pid = process.processIdentifier
        try? input?.close()
        input = nil
        if !GoCore.waitForExit(process, upTo: grace) {
            kill(pid, SIGTERM)
            if !GoCore.waitForExit(process, upTo: grace) {
                kill(pid, SIGKILL)
                _ = GoCore.waitForExit(process, upTo: 1)
            }
        }
        if bridge.core == pid { bridge.core = nil }
        return GoCore.isGone(pid)
    }

    /// Whether `pid` is no process at all: ended and reaped.
    static func isGone(_ pid: pid_t) -> Bool { kill(pid, 0) == -1 && errno == ESRCH }

    private static func waitForExit(_ process: Process, upTo wait: TimeInterval) -> Bool {
        let deadline = Date().addingTimeInterval(wait)
        while process.isRunning, Date() < deadline { Thread.sleep(forTimeInterval: 0.05) }
        guard !process.isRunning else { return false }
        // Reaped once Process has seen it end.
        process.waitUntilExit()
        return true
    }

    // MARK: Lines from the core

    private func received(_ data: Data) {
        lock.lock()
        buffer.append(data)
        var count = 0
        var menu: ShellMenu?
        while let newline = buffer.firstIndex(of: 0x0A) {
            let line = buffer[buffer.startIndex..<newline]
            buffer.removeSubrange(buffer.startIndex...newline)
            guard let object = (try? JSONSerialization.jsonObject(with: line)) as? [String: Any] else { continue }
            // Anything from the core says it's reading and writing: the menu may ask again.
            menuUnanswered = false
            // Neither an answer nor ready: the menu's state, sent unasked, never waited for.
            if object["id"] == nil, object["ready"] == nil {
                menu = ShellMenu(object["menu"]) ?? menu
                continue
            }
            lines.append(object)
            count += 1
        }
        // Nothing sane is this long: dropped, rather than kept growing.
        if buffer.count > 1 << 20 { buffer.removeAll() }
        lock.unlock()
        for _ in 0..<count { arrived.signal() }
        if let menu { DispatchQueue.main.async { self.onMenu(menu) } }
    }

    /// The next line, or nil once the core has ended or `deadline` (asked again as it waits) has passed.
    private func nextLine(until deadline: () -> Date) -> [String: Any]? {
        while true {
            lock.lock()
            if !lines.isEmpty {
                let line = lines.removeFirst()
                lock.unlock()
                return line
            }
            let running = process?.isRunning ?? false
            lock.unlock()
            let until = deadline()
            guard running, Date() < until else { return nil }
            _ = arrived.wait(timeout: .now() + min(0.2, max(0, until.timeIntervalSinceNow)))
        }
    }
}
