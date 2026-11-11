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
/// its arguments as a list, never through a shell. The Linux/Windows app's
/// t3code.go answers the same.
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
    /// How long a check is kept: the page asks again at most this often, or when the button's menu opens.
    static let keptFor: TimeInterval = 30

    static let appOnlyHint = "In T3 Code, open Settings → About and install the t3 command."
    static let noneHint = "Install T3 Code to open this hire's folder there (t3.codes)."

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
        let version = ran.status == 0 ? firstLine(ran.output) : ""
        return Status(state: app ? .found : .cliOnly, version: String(version.prefix(40)))
    }

    private static let lock = NSLock()
    private static var kept: (status: Status, at: Date)?

    /// The last check, if it's newer than `keptFor`; otherwise, or `fresh`, a new one. Never on the main thread.
    static func status(fresh: Bool = false, machine: Machine = Machine()) -> Status {
        lock.lock()
        if !fresh, let kept, machine.now().timeIntervalSince(kept.at) < keptFor { lock.unlock(); return kept.status }
        lock.unlock()
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

    /// Opens `hire`'s folder in T3 Code as `status` says it's installed: a 200 with `{ok, opened}`, or a refusal.
    /// Runs `t3` and may wait for the app: never on the main thread.
    static func open(_ hire: FrontDesk.Hire, status: Status, machine: Machine = Machine()) -> Response {
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
        // The app, opened, then `t3 app` once it answers: tried again until it does, for `openWithin` at most.
        machine.openApp()
        let until = machine.now().addingTimeInterval(openWithin)
        var ran = Ran()
        repeat {
            ran = machine.run(program, ["app", folder], max(1, until.timeIntervalSince(machine.now())))
            if ran.status == 0 { return .json(["ok": true, "opened": "thread"]) }
            machine.pause(1)
        } while machine.now() < until
        return failed(ran)
    }

    /// A `t3` that failed: a 502 with the first line it said, and nothing else.
    static func failed(_ ran: Ran) -> Response {
        let line = firstLine(ran.errors.isEmpty ? ran.output : ran.errors)
        return .status(502, line.isEmpty ? (ran.status == nil ? "T3 Code didn't answer." : "T3 Code couldn't open it.") : String(line.prefix(200)))
    }

    static func firstLine(_ text: String) -> String {
        text.split(whereSeparator: \.isNewline).lazy.map { $0.trimmingCharacters(in: .whitespaces) }.first { !$0.isEmpty } ?? ""
    }

    /// Runs `program` with `arguments` as they are, no shell, stopped after `timeout`. Its folder is put first
    /// on the PATH it's given, so a `t3` that's a script finds what sits beside it.
    static func run(_ program: URL, _ arguments: [String], _ timeout: TimeInterval) -> Ran {
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
        process.standardInput = FileHandle.nullDevice
        let ended = DispatchSemaphore(value: 0)
        process.terminationHandler = { _ in ended.signal() }
        guard (try? process.run()) != nil else { return Ran(status: nil) }
        // Both read at once, so neither pipe fills; what an app it started keeps open isn't waited for past the end.
        final class Box { var data = Data() }
        let printed = Box(), said = Box(), reading = DispatchGroup()
        for (pipe, box) in [(output, printed), (errors, said)] {
            reading.enter()
            DispatchQueue.global().async { box.data = pipe.fileHandleForReading.readDataToEndOfFile(); reading.leave() }
        }
        guard ended.wait(timeout: .now() + timeout) == .success else {
            process.terminate()
            return Ran(status: nil)
        }
        _ = reading.wait(timeout: .now() + 1)
        return Ran(status: process.terminationStatus,
                   output: String(decoding: printed.data.prefix(4096), as: UTF8.self), errors: String(decoding: said.data.prefix(4096), as: UTF8.self))
    }
}
