import AppKit

/// "Open in T3 Code" (T792): a hire's folder (its worktree, else the folder it
/// was hired into) opened in T3 Code, on this Mac only. T3 Code is a separate
/// app, alpha, with its own agent; Smolcorp only runs what's installed, never
/// npx, an install or a download, and never touches the hire's terminal or agent.
///
/// T3 Code is found by its app (by bundle id) and its `t3` command. With both,
/// the app is opened and `t3 app <folder>` starts a thread there; with `t3`
/// alone, `t3 project add` adds the folder to its projects. `t3` only ever runs
/// with `--version`, `app <folder>` or `project add <folder> --title <title>`,
/// its arguments as a list, never through a shell; and `thread list --project
/// <folder> --json`, before `t3 app` and again after one that says it failed,
/// to see whether it did. The Linux/Windows app's t3code.go answers the same.
enum T3Code {
    /// What's installed: the app and `t3`, one of them, or neither.
    enum State: String { case found, appOnly, cliOnly, none }

    struct Status: Equatable {
        var state: State
        /// What `t3 --version` said, "" without `t3` or an answer.
        var version = ""
    }

    /// T3 Code's app, as its Info.plist names it.
    static let bundleID = "com.t3tools.t3code"
    /// How long `t3 --version` may take, and how long the app may take to answer `t3 app`.
    static let versionTimeout: TimeInterval = 3
    static let openWithin: TimeInterval = 10
    /// How long a check is kept: the page asks again at most this often, or when the button's menu opens. Even
    /// asked fresh, a check newer than `freshFor` is the answer.
    static let keptFor: TimeInterval = 30
    static let freshFor: TimeInterval = 2
    /// How new a thread of the folder's must be to count as the one `t3 app` opened: one that wasn't listed before it.
    static let threadWithin: TimeInterval = 15

    static let appOnlyHint = "In T3 Code, open Settings → About and install the t3 command."
    static let noneHint = "Install T3 Code to open this hire's folder there (t3.codes)."
    static let alreadyOpening = "Already opening T3 Code for this hire."

    /// One run of `t3`: how it ended (nil: it didn't start, or was stopped for taking too long) and what it printed.
    struct Ran {
        var status: Int32?
        var output = ""
        var errors = ""
    }

    /// What the checks and the open use on this Mac. Tests give their own: a home folder with a fake `t3`, and no app.
    struct Machine {
        var home = Paths.home
        /// The folders `t3` may be in besides the home folder's: Homebrew's, then the system's.
        var system = ["/opt/homebrew/bin", "/usr/local/bin"]
        var path = ProcessInfo.processInfo.environment["PATH"] ?? ""
        var appInstalled: () -> Bool = { NSWorkspace.shared.urlForApplication(withBundleIdentifier: T3Code.bundleID) != nil }
        var openApp: () -> Void = {
            guard let app = NSWorkspace.shared.urlForApplication(withBundleIdentifier: T3Code.bundleID) else { return }
            let configuration = NSWorkspace.OpenConfiguration()
            configuration.activates = true
            NSWorkspace.shared.openApplication(at: app, configuration: configuration)
        }
        var run: (URL, [String], TimeInterval) -> Ran = T3Code.run
        /// The same, with `Data` given on its standard input: a message for `t3 thread send <id> -` (T797b).
        var feed: (URL, [String], Data, TimeInterval) -> Ran = T3Code.feed
        /// Opens a link (T3 Code's own `t3code://` one) with NSWorkspace, never through a shell. Says whether it was taken.
        var openURL: (URL) -> Bool = { NSWorkspace.shared.open($0) }
        var pause: (TimeInterval) -> Void = { Thread.sleep(forTimeInterval: $0) }
        var now: () -> Date = Date.init
    }

    // MARK: Finding it

    /// Where `t3` is looked for, in the order T3 Code's own DesktopCliCommand.ts checks: a Mac app doesn't
    /// see your shell's PATH, so the folders T3 Code links it into come first, then the PATH the app has.
    static func folders(_ machine: Machine) -> [String] {
        [machine.home.appendingPathComponent(".local/bin").path, machine.home.appendingPathComponent("bin").path]
            + machine.system + machine.path.split(separator: ":").map(String.init)
    }

    /// The `t3` command, or nil.
    static func program(_ machine: Machine) -> URL? {
        OpenCodeInstall.place(named: "t3", in: folders(machine))
    }

    /// What's installed now, and `t3`'s version. Runs `t3`: never on the main thread.
    static func detect(_ machine: Machine = Machine()) -> Status {
        let app = machine.appInstalled()
        guard let program = program(machine) else { return Status(state: app ? .appOnly : .none) }
        let ran = machine.run(program, ["--version"], versionTimeout)
        // It says "t3 v0.1.8": the version is what follows "t3 ".
        var version = ran.status == 0 ? firstLine(ran.output) : ""
        if version.hasPrefix("t3 ") { version = String(version.dropFirst(3)) }
        return Status(state: app ? .found : .cliOnly, version: String(version.prefix(40)))
    }

    private static let lock = NSLock()
    private static var kept: (status: Status, at: Date)?
    /// Held while `t3 --version` runs: one check at a time.
    private static let checking = NSLock()

    /// The last check, if it's newer than `keptFor` (`freshFor`, asked `fresh`); otherwise a new one, one at a
    /// time: a check that ends while this one waits for it is the answer. Never on the main thread.
    static func status(fresh: Bool = false, machine: Machine = Machine()) -> Status {
        func newEnough() -> Status? {
            lock.lock(); defer { lock.unlock() }
            guard let kept else { return nil }
            let age = machine.now().timeIntervalSince(kept.at)
            return age >= 0 && age < (fresh ? freshFor : keptFor) ? kept.status : nil
        }
        if let status = newEnough() { return status }
        checking.lock(); defer { checking.unlock() }
        if let status = newEnough() { return status }
        let status = detect(machine)
        lock.lock(); kept = (status, machine.now()); lock.unlock()
        return status
    }

    /// `GET /setup/t3code` and the office's: what's installed, and the line a greyed button shows under it.
    static func json(_ status: Status) -> [String: Any] {
        ["state": status.state.rawValue, "version": status.version, "hint": hint(status.state)]
    }

    static func hint(_ state: State) -> String {
        switch state {
        case .appOnly: return appOnlyHint
        case .none: return noneHint
        case .found, .cliOnly: return ""
        }
    }

    // MARK: The hire's folder

    /// The folder to open for a hire: its worktree, else its folder. Only a folder that's there, inside the
    /// home folder, not the home folder itself and not hidden (nor inside a hidden one).
    static func folder(of hire: FrontDesk.Hire, home: URL) -> Result<String, Refusal> {
        let given = hire.worktree.isEmpty ? hire.folder : hire.worktree
        guard !given.isEmpty else { return .failure(Refusal(400, "This hire has no folder to open.")) }
        var isFolder: ObjCBool = false
        guard FileManager.default.fileExists(atPath: given, isDirectory: &isFolder), isFolder.boolValue else {
            return .failure(Refusal(409, "This hire's folder isn't there any more."))
        }
        let folder = URL(fileURLWithPath: given).resolvingSymlinksInPath().standardizedFileURL.path
        let base = home.resolvingSymlinksInPath().standardizedFileURL.path
        guard folder != base else { return .failure(Refusal(409, "That's your whole home folder: T3 Code opens a hire's own folder only.")) }
        guard folder.hasPrefix(base + "/") else { return .failure(Refusal(409, "Only a folder inside your home folder opens in T3 Code.")) }
        guard !folder.dropFirst(base.count + 1).split(separator: "/").contains(where: { $0.hasPrefix(".") }) else {
            return .failure(Refusal(409, "A hidden folder doesn't open in T3 Code."))
        }
        return .success(folder)
    }

    /// How a hire's project is named in T3 Code: "<name> · <branch>".
    static func title(of hire: FrontDesk.Hire) -> String {
        let name = hire.nickname.isEmpty ? hire.name : hire.nickname
        return hire.branch.isEmpty ? name : "\(name) · \(hire.branch)"
    }

    struct Refusal: Error, Equatable {
        var status: Int
        var message: String
        init(_ status: Int, _ message: String) { (self.status, self.message) = (status, message) }
    }

    // MARK: Opening

    /// The hires whose folder is being opened now.
    private static var opening = Set<String>()

    /// `open`, once at a time for a hire: a second open for the same hire while one runs is a 409. What's
    /// installed is `check`ed once it's this hire's turn. Never on the main thread.
    static func openOnce(_ hire: FrontDesk.Hire, machine: Machine = Machine(),
                         check: (Machine) -> Status = { status(machine: $0) }) -> Response {
        lock.lock()
        let first = opening.insert(hire.id).inserted
        lock.unlock()
        guard first else { return .status(409, alreadyOpening) }
        defer { lock.lock(); opening.remove(hire.id); lock.unlock() }
        return open(hire, status: check(machine), machine: machine)
    }

    /// Opens `hire`'s folder in T3 Code as `status` says it's installed: a 200 with `{ok, opened}`, or a refusal.
    /// Runs `t3` and may wait for the app: never on the main thread.
    static func open(_ hire: FrontDesk.Hire, status: Status, machine: Machine = Machine()) -> Response {
        // A hire that runs in T3 Code (T797b): its own thread there, by T3 Code's link, not a new one.
        if hire.engine == T3Desk.engine { return T3Desk.openThread(hire, status: status, machine: machine) }
        let folder: String
        switch T3Code.folder(of: hire, home: machine.home) {
        case .failure(let refusal): return .status(refusal.status, refusal.message)
        case .success(let found): folder = found
        }
        switch status.state {
        case .none: return .status(409, noneHint)
        case .appOnly: return .status(409, appOnlyHint)
        case .cliOnly, .found: break
        }
        guard let program = program(machine) else { return .status(409, status.state == .found ? appOnlyHint : noneHint) }
        if status.state == .cliOnly {
            let ran = machine.run(program, ["project", "add", folder, "--title", title(of: hire)], openWithin)
            // A folder that's already one of its projects: it's there, as asked.
            if ran.status == 0 || (ran.output + ran.errors).range(of: "already exists", options: .caseInsensitive) != nil {
                return .json(["ok": true, "opened": "project"])
            }
            return failed(ran)
        }
        // The folder's threads before: only one that isn't among them can be the one `t3 app` opens. None when T3 Code
        // isn't running yet or the folder isn't one of its projects.
        let before = Set((threads(program, folder: folder, machine: machine) ?? []).map(\.id))
        // The app, opened, then `t3 app` once it answers: tried again until it does, for `openWithin` at most.
        machine.openApp()
        let until = machine.now().addingTimeInterval(openWithin)
        var ran = Ran()
        repeat {
            ran = machine.run(program, ["app", folder], max(1, until.timeIntervalSince(machine.now())))
            if ran.status == 0 { return .json(["ok": true, "opened": "thread"]) }
            // The app answered, and said it couldn't open a thread: T3 Code 0.1.8 says so even when it did, and its
            // new thread is a draft, which isn't listed until it's used. Not tried again, it would open another. A
            // thread of the folder's that's just appeared says it did; a list without one, that the folder is open
            // as a project and T3 Code got as far as its thread; a list that fails, the error.
            if (ran.output + ran.errors).contains("(thread-open-failed)") {
                guard let after = threads(program, folder: folder, machine: machine) else { return failed(ran) }
                let since = machine.now().addingTimeInterval(-threadWithin)
                let new = after.contains { !before.contains($0.id) && ($0.made.map { $0 >= since } ?? false) }
                return .json(["ok": true, "opened": new ? "thread" : "unsure"])
            }
            machine.pause(1)
        } while machine.now() < until
        return failed(ran)
    }

    /// The threads T3 Code lists for `folder`, by id with when each was made; nil if the list fails or isn't a list.
    static func threads(_ program: URL, folder: String, machine: Machine) -> [(id: String, made: Date?)]? {
        let ran = machine.run(program, ["thread", "list", "--project", folder, "--json"], versionTimeout)
        guard ran.status == 0, let threads = try? JSONSerialization.jsonObject(with: Data(ran.output.utf8)) as? [[String: Any]] else { return nil }
        return threads.compactMap { thread in
            (thread["threadId"] as? String).map { ($0, (thread["createdAt"] as? String).flatMap(date)) }
        }
    }

    /// An ISO 8601 time as T3 Code writes it, with or without its milliseconds.
    static func date(_ text: String) -> Date? {
        let withMilliseconds = ISO8601DateFormatter(), plain = ISO8601DateFormatter()
        withMilliseconds.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return withMilliseconds.date(from: text) ?? plain.date(from: text)
    }

    /// What T3 Code's log puts before an error: "[01:33:17.503] ERROR (#2): DesktopAppRequestFailedError: ".
    static let logPrefix = try! NSRegularExpression(pattern: #"^\[[^\]]*\] ERROR \(#\d+\): \w+Error: "#)

    /// A `t3` that failed: a 502 with the first line it said, without T3 Code's log prefix, and nothing else.
    static func failed(_ ran: Ran) -> Response {
        var line = firstLine(ran.errors.isEmpty ? ran.output : ran.errors)
        line = logPrefix.stringByReplacingMatches(in: line, range: NSRange(line.startIndex..., in: line), withTemplate: "")
        return .status(502, line.isEmpty ? (ran.status == nil ? "T3 Code didn't answer." : "T3 Code couldn't open it.") : String(line.prefix(200)))
    }

    static func firstLine(_ text: String) -> String {
        text.split(whereSeparator: \.isNewline).lazy.map { $0.trimmingCharacters(in: .whitespaces) }.first { !$0.isEmpty } ?? ""
    }

    /// What a run may print that's kept: a thread list's JSON fits.
    static let keptOutput = 1 << 20

    /// Runs `program` with `arguments` as they are, no shell, stopped after `timeout`. Its folder is put first
    /// on the PATH it's given, so a `t3` that's a script finds what sits beside it.
    static func run(_ program: URL, _ arguments: [String], _ timeout: TimeInterval) -> Ran {
        launch(program, arguments, timeout, input: nil)
    }

    /// `run`, with `input` written to its standard input, then closed: the text never goes on its command line.
    static func feed(_ program: URL, _ arguments: [String], _ input: Data, _ timeout: TimeInterval) -> Ran {
        launch(program, arguments, timeout, input: input)
    }

    private static func launch(_ program: URL, _ arguments: [String], _ timeout: TimeInterval, input: Data?) -> Ran {
        let process = Process()
        process.executableURL = program
        process.arguments = arguments
        var environment = ProcessInfo.processInfo.environment
        environment["PATH"] = ([program.deletingLastPathComponent().path, "/opt/homebrew/bin", "/usr/local/bin"]
            + (environment["PATH"] ?? "/usr/bin:/bin").split(separator: ":").map(String.init)).joined(separator: ":")
        process.environment = environment
        let output = Pipe(), errors = Pipe()
        process.standardOutput = output
        process.standardError = errors
        let given = input.map { _ in Pipe() }
        process.standardInput = given ?? FileHandle.nullDevice
        let ended = DispatchSemaphore(value: 0)
        process.terminationHandler = { _ in ended.signal() }
        // What each pipe says, kept as it comes under a lock, and when it's closed. Nothing waits on a read: what
        // an app it started keeps open is let go a second after the end.
        final class Said {
            private let lock = NSLock()
            private var data = Data()
            let closed = DispatchSemaphore(value: 0)
            func add(_ chunk: Data) { lock.lock(); data.append(chunk.prefix(T3Code.keptOutput - data.count)); lock.unlock() }
            var text: String { lock.lock(); defer { lock.unlock() }; return String(decoding: data, as: UTF8.self) }
        }
        let printed = Said(), said = Said(), pipes = [(output, printed), (errors, said)]
        for (pipe, box) in pipes {
            pipe.fileHandleForReading.readabilityHandler = { handle in
                let chunk = handle.availableData
                if chunk.isEmpty { handle.readabilityHandler = nil; box.closed.signal() } else { box.add(chunk) }
            }
        }
        let letGo = { for (pipe, _) in pipes { pipe.fileHandleForReading.readabilityHandler = nil } }
        guard (try? process.run()) != nil else { letGo(); return Ran(status: nil) }
        if let given, let input {
            // Written off this thread, so a program that doesn't read it can't hold the wait for its end.
            // A program that stops reading early mustn't raise SIGPIPE in Smolcorp (as SummaryProviders does).
            _ = fcntl(given.fileHandleForWriting.fileDescriptor, F_SETNOSIGPIPE, 1)
            DispatchQueue.global(qos: .userInitiated).async {
                try? given.fileHandleForWriting.write(contentsOf: input)
                try? given.fileHandleForWriting.close()
            }
        }
        let inTime = ended.wait(timeout: .now() + timeout) == .success
        if !inTime { process.terminate() }
        let drained = DispatchTime.now() + 1
        for (_, box) in pipes { _ = box.closed.wait(timeout: drained) }
        letGo()
        guard inTime else { return Ran(status: nil) }
        return Ran(status: process.terminationStatus, output: printed.text, errors: said.text)
    }
}
