import Foundation

/// Which engine runs the deck, as `Config.engine` says: this app's own (Swift,
/// as always), or the Go core, the same server the Linux and Windows app runs,
/// started by this app as its child (`GoCore`), with this app as its shell:
/// the menu bar, the windows, and what only macOS can do (`HostBridge`).
/// For now only a test copy may run the Go core.
enum Engine: String {
    case swift, go

    /// Said once, as the app starts, when a copy that isn't a test copy is set to "go".
    static let refusal = "The Go engine is only for a test copy of Smolcorp for now, so this Smolcorp runs its own engine."

    /// The engine to run for `setting`, and the sentence to say when "go" is refused.
    static func chosen(_ setting: String?, testCopy: Bool = TestCopy.isOn) -> (engine: Engine, refusal: String?) {
        guard setting == Engine.go.rawValue else { return (.swift, nil) }
        guard testCopy else { return (.swift, refusal) }
        return (.go, nil)
    }
}

/// The Go core, run as this app's child: `bin/smolcorp core`, bundled and
/// signed with the app. It's told its port, the support folder and the
/// bridge's socket on its command line, and the bridge's token as the first
/// line on its standard input: never in its arguments or its environment,
/// which other processes of yours can read (`ps -E`). It says it's ready on
/// its standard output; this app then asks it, a line of JSON each way, for
/// the office window's key and for setup sessions: only this app holds that
/// pipe's other end. The bridge serves this core's pid alone, and the core
/// sends the token only to its parent, this app (see `HostBridge`). When this
/// app quits, or dies, the core's input ends and it stops: `stop` also makes
/// sure, by its pid.
final class GoCore {
    enum Failure: LocalizedError {
        case noProgram, didNotStart(String), notReady(String)

        var errorDescription: String? {
            switch self {
            case .noProgram: return "This build of Smolcorp has no Go engine inside it (bin/smolcorp)."
            case .didNotStart(let why): return "The Go engine didn't start: \(why)"
            case .notReady(let why): return "The Go engine didn't get ready: \(why)"
            }
        }
    }

    let program: URL
    let port: UInt16
    let support: URL
    let bridge: HostBridge
    /// The name the core gives its setup page's session cookie, as it said when ready.
    private(set) var setupCookie = "smolcorp-setup"
    /// Told on the main queue if the core stops by itself, with its exit status.
    var onExit: (Int32) -> Void = { _ in }

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
    /// nothing else. A Smolcorp opened from a terminal would otherwise hand the
    /// core that terminal's whole environment, its tokens too.
    static let inherited = ["PATH", "HOME", "USER", "LANG", "TMPDIR", "SHELL"]

    /// The core's environment: `inherited` from this app's, with the port and folder
    /// a test copy's hires and `smolcorp gm` read, as the Swift engine gives them. Never the token.
    func environment(base: [String: String] = ProcessInfo.processInfo.environment) -> [String: String] {
        var environment = base.filter { GoCore.inherited.contains($0.key) }
        environment["SMOLCORP_PORT"] = String(port)
        environment["SMOLCORP_TEST_SUPPORT_DIR"] = support.path
        return environment
    }

    /// Starts the core and waits until it serves, or fails. Off the main queue.
    func start(waitingUpTo wait: TimeInterval = 20) throws {
        guard FileManager.default.isExecutableFile(atPath: program.path) else { throw Failure.noProgram }
        let process = Process()
        process.executableURL = program
        process.arguments = arguments
        process.environment = environment()
        process.currentDirectoryURL = support
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
            let expected = self.stopping
            self.lock.unlock()
            // Its pid may be another process's soon: the bridge serves no one now.
            if self.bridge.core == ended.processIdentifier { self.bridge.core = nil }
            self.arrived.signal()
            if !expected { DispatchQueue.main.async { self.onExit(ended.terminationStatus) } }
        }
        do { try process.run() } catch { throw Failure.didNotStart(error.localizedDescription) }
        self.process = process
        // The only process the bridge serves, from before the core has its token.
        bridge.core = process.processIdentifier
        input = toCore.fileHandleForWriting
        // A core that has ended mustn't raise SIGPIPE in this app as it's written to.
        _ = fcntl(toCore.fileHandleForWriting.fileDescriptor, F_SETNOSIGPIPE, 1)
        // The token first, before anything else is said on that pipe: the core reads it before its `hello`.
        // Not written only when the core has already ended, which the wait below says.
        try? toCore.fileHandleForWriting.write(contentsOf: Data((bridge.token + "\n").utf8))
        guard let ready = nextLine(until: Date().addingTimeInterval(wait)), ready["ready"] as? Bool == true else {
            let why = process.isRunning ? "no answer in \(Int(wait)) s" : "it ended (\(process.terminationStatus))"
            _ = stop()
            throw Failure.notReady(why)
        }
        if let cookie = ready["cookie"] as? String, !cookie.isEmpty { setupCookie = cookie }
    }

    /// Asks the core `op` and waits a moment for its answer: nil without one.
    func ask(_ op: String, then: String? = nil, waitingUpTo wait: TimeInterval = 3) -> [String: Any]? {
        asking.lock()
        defer { asking.unlock() }
        guard let input, process?.isRunning == true else { return nil }
        let id = nextID
        nextID += 1
        var request: [String: Any] = ["id": id, "op": op]
        if let then { request["then"] = then }
        guard var line = try? JSONSerialization.data(withJSONObject: request) else { return nil }
        line.append(0x0A)
        do { try input.write(contentsOf: line) } catch { return nil }
        let deadline = Date().addingTimeInterval(wait)
        // An answer to an earlier request that came too late is passed over.
        while let answer = nextLine(until: deadline) {
            if answer["id"] as? Int == id { return answer["error"] == nil ? answer : nil }
        }
        return nil
    }

    /// A new setup session for the Settings window, and its cookie's name.
    func setupSession() -> (session: String, cookie: String)? {
        guard let answer = ask("setup-session"), let session = answer["session"] as? String,
              let cookie = answer["cookie"] as? String, !session.isEmpty, !cookie.isEmpty else { return nil }
        return (session, cookie)
    }

    /// The office's window, with the office's own key on this Mac, as the Swift engine's.
    func officeAddress() -> URL? {
        guard let key = ask("office")?["key"] as? String, !key.isEmpty,
              let encoded = key.addingPercentEncoding(withAllowedCharacters: .alphanumerics) else { return nil }
        return URL(string: "http://127.0.0.1:\(port)/office?window&t=\(encoded)")
    }

    /// A link to setup that works once, for the browser.
    func setupLink(then: String? = nil) -> URL? {
        (ask("setup-link", then: then ?? "")?["link"] as? String).flatMap(URL.init(string:))
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
        while let newline = buffer.firstIndex(of: 0x0A) {
            let line = buffer[buffer.startIndex..<newline]
            buffer.removeSubrange(buffer.startIndex...newline)
            if let object = (try? JSONSerialization.jsonObject(with: line)) as? [String: Any] {
                lines.append(object)
                count += 1
            }
        }
        // Nothing sane is this long: dropped, rather than kept growing.
        if buffer.count > 1 << 20 { buffer.removeAll() }
        lock.unlock()
        for _ in 0..<count { arrived.signal() }
    }

    private func nextLine(until deadline: Date) -> [String: Any]? {
        while true {
            lock.lock()
            if !lines.isEmpty {
                let line = lines.removeFirst()
                lock.unlock()
                return line
            }
            let running = process?.isRunning ?? false
            lock.unlock()
            guard running, Date() < deadline else { return nil }
            _ = arrived.wait(timeout: .now() + min(0.2, max(0, deadline.timeIntervalSinceNow)))
        }
    }
}
