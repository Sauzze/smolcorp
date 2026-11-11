import XCTest
@testable import AgentDeck

/// Open in T3 Code (T792): what's found, the folder from the registry and its refusals, and `t3` run with exactly
/// its arguments, never through a shell. A fake home folder holds a fake `t3` that writes down what it was given;
/// the real `t3` and the real app are never run.
final class T3CodeTests: XCTestCase {
    private var home: URL!
    private var calls: URL { home.appendingPathComponent("calls") }
    private var mode: URL { home.appendingPathComponent("mode") }
    private var threads: URL { home.appendingPathComponent("threads") }
    private var next: URL { home.appendingPathComponent("next") }
    private var opened = 0
    private var clock = Date(timeIntervalSince1970: 1_000_000)

    override func setUpWithError() throws {
        home = FileManager.default.temporaryDirectory.appendingPathComponent("t3-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)
        home = home.resolvingSymlinksInPath()
        opened = 0
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: home)
    }

    /// A fake `t3` in `folder` (in the home folder): each run's arguments one to a line, then "--". As `mode` says:
    /// "ok", "fail" (always, saying why on two lines), "exists" (project add says it's there), "late" (`t3 app` fails
    /// twice), "renderer" (`t3 app` fails twice as T3 Code does while its window isn't up), "slow" (its version takes a
    /// second) or "unsure" (`t3 app` says thread-open-failed, as T3 Code 0.1.8 does even when it opened one). `thread
    /// list` prints the threads file, and fails as T3 Code does when it isn't running without one; a `next` file is
    /// what `t3 app` makes the threads. It says its version as the real one: "t3 v0.1.8".
    private func fakeT3(in folder: String = ".local/bin", mode: String = "ok") throws {
        let bin = home.appendingPathComponent(folder)
        try FileManager.default.createDirectory(at: bin, withIntermediateDirectories: true)
        try mode.write(to: self.mode, atomically: true, encoding: .utf8)
        let script = """
        #!/bin/sh
        for a in "$@"; do printf '%s\\n' "$a" >> '\(calls.path)'; done
        printf -- '--\\n' >> '\(calls.path)'
        mode=$(cat '\(self.mode.path)')
        if [ "$1" = "--version" ]; then
          [ "$mode" = "fail" ] && { echo "  broken install  " >&2; exit 1; }
          [ "$mode" = "slow" ] && sleep 1
          echo "t3 v0.1.8"; exit 0
        fi
        if [ "$1" = "thread" ]; then
          [ -f '\(threads.path)' ] || { echo "No running T3 Code server uses ~/.t3. Start T3 Code, then retry." >&2; exit 1; }
          cat '\(threads.path)'; exit 0
        fi
        [ -f '\(next.path)' ] && mv '\(next.path)' '\(threads.path)'
        apps=$(grep -c '^app$' '\(calls.path)')
        case "$mode" in
          unsure) echo "[01:33:17.503] ERROR (#2): DesktopAppRequestFailedError: T3 Code could not open $2 (thread-open-failed)." >&2
            echo "    at Object.<anonymous> (file:///binCli.mjs:8963:34)" >&2; exit 1 ;;
          fail) printf '  Could not reach the T3 Code desktop app.  \\nmore detail\\n' >&2; exit 1 ;;
          exists) echo "Project already exists: $3" >&2; exit 1 ;;
          late) [ "$apps" -le 2 ] && { echo "not yet" >&2; exit 1; }; exit 0 ;;
          renderer) [ "$apps" -le 2 ] && { echo "T3 Code could not open $2 (renderer-unavailable)." >&2; exit 1; }; exit 0 ;;
        esac
        exit 0
        """
        let program = bin.appendingPathComponent("t3")
        try script.write(to: program, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: program.path)
    }

    /// Each run's arguments, as the fake wrote them down.
    private func runs() -> [[String]] {
        let text = (try? String(contentsOf: calls, encoding: .utf8)) ?? ""
        return text.components(separatedBy: "--\n").dropLast().map { $0.split(separator: "\n", omittingEmptySubsequences: false).map(String.init).filter { !$0.isEmpty } }
    }

    /// `T3Code.run`, with time enough for the fake: a shell script that, on a busy Mac, can take seconds just to
    /// start, past the 3 the version and the thread list get, and be stopped before it writes down its run. It
    /// always ends by itself; how a run that takes too long is stopped is tested on `run` itself.
    private static func runFake(_ program: URL, _ arguments: [String], _ timeout: TimeInterval) -> T3Code.Ran {
        T3Code.run(program, arguments, max(timeout, 60))
    }

    /// This machine as the tests have it: the fake home, no other folders, no PATH, the app as `app` says; time
    /// moves only when the open pauses.
    private func machine(app: Bool) -> T3Code.Machine {
        var machine = T3Code.Machine()
        machine.home = home
        machine.system = []
        machine.path = ""
        machine.run = T3CodeTests.runFake
        machine.appInstalled = { app }
        machine.openApp = { [unowned self] in self.opened += 1 }
        machine.pause = { [unowned self] in self.clock.addTimeInterval($0) }
        machine.now = { [unowned self] in self.clock }
        return machine
    }

    private func folder(_ name: String) throws -> String {
        let url = home.appendingPathComponent(name, isDirectory: true)
        try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
        return url.path
    }

    private func hire(folder: String, worktree: String = "", branch: String = "hire/ada-1a2b3c") -> FrontDesk.Hire {
        FrontDesk.Hire(id: "h1", name: "Ada", folder: folder, branch: branch, worktree: worktree, state: .working)
    }

    private func json(_ response: Response) -> NSDictionary? {
        try? JSONSerialization.jsonObject(with: response.body) as? NSDictionary
    }

    private func text(_ response: Response) -> String { String(decoding: response.body, as: UTF8.self) }

    /// The run that lists `folder`'s threads.
    private func list(_ folder: String) -> [String] { ["thread", "list", "--project", folder, "--json"] }

    // MARK: Finding it

    func testEachStateIsFoundAndTheVersionIsAsked() throws {
        XCTAssertEqual(T3Code.detect(machine(app: false)), T3Code.Status(state: .none))
        XCTAssertEqual(T3Code.detect(machine(app: true)), T3Code.Status(state: .appOnly))
        XCTAssertEqual(runs(), [], "no t3: nothing run")
        try fakeT3()
        XCTAssertEqual(T3Code.detect(machine(app: false)), T3Code.Status(state: .cliOnly, version: "v0.1.8"))
        XCTAssertEqual(T3Code.detect(machine(app: true)), T3Code.Status(state: .found, version: "v0.1.8"), "without its \"t3 \"")
        XCTAssertEqual(runs(), [["--version"], ["--version"]], "only --version, to find it")
        try "fail".write(to: mode, atomically: true, encoding: .utf8)
        XCTAssertEqual(T3Code.detect(machine(app: true)), T3Code.Status(state: .found, version: ""), "a t3 that fails is still there")
    }

    func testT3IsLookedForInTheHomeFoldersFirstThenThePath() throws {
        var m = machine(app: false)
        m.system = ["/opt/homebrew/bin", "/usr/local/bin"]
        m.path = "/usr/bin:/bin"
        XCTAssertEqual(T3Code.folders(m), [home.appendingPathComponent(".local/bin").path, home.appendingPathComponent("bin").path,
                                           "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin"])
        XCTAssertEqual(T3Code.Machine().system, ["/opt/homebrew/bin", "/usr/local/bin"])
        try fakeT3(in: "bin")
        XCTAssertEqual(T3Code.program(machine(app: false))?.path, home.appendingPathComponent("bin/t3").path)
        XCTAssertEqual(T3Code.json(T3Code.Status(state: .appOnly)) as NSDictionary,
                       ["state": "appOnly", "version": "", "hint": "In T3 Code, open Settings → About and install the t3 command."])
        XCTAssertEqual(T3Code.json(T3Code.Status(state: .none))["hint"] as? String, "Install T3 Code to open this hire's folder there (t3.codes).")
        XCTAssertEqual(T3Code.json(T3Code.Status(state: .found, version: "0.1.8"))["hint"] as? String, "")
    }

    func testACheckIsKeptThirtySecondsUnlessAskedFresh() throws {
        try fakeT3()
        let m = machine(app: true)
        _ = T3Code.status(fresh: true, machine: m)
        _ = T3Code.status(machine: m)
        clock.addTimeInterval(1)
        _ = T3Code.status(fresh: true, machine: m)
        XCTAssertEqual(runs().count, 1, "asked fresh within 2 seconds: the last check")
        clock.addTimeInterval(28)
        _ = T3Code.status(machine: m)
        XCTAssertEqual(runs().count, 1, "kept")
        _ = T3Code.status(fresh: true, machine: m)
        XCTAssertEqual(runs().count, 2, "the menu opened: asked again")
        clock.addTimeInterval(31)
        _ = T3Code.status(machine: m)
        XCTAssertEqual(runs().count, 3, "older than 30 seconds: asked again")
    }

    func testOneCheckAtATime() throws {
        try fakeT3(mode: "slow")
        let m = machine(app: true)
        clock.addTimeInterval(3600) // newer than any check an earlier test kept
        DispatchQueue.concurrentPerform(iterations: 3) { _ in _ = T3Code.status(fresh: true, machine: m) }
        XCTAssertEqual(runs(), [["--version"]], "three fresh asks at once: one t3 --version")
    }

    /// `run` reads what's said as it comes: a program that leaves a process holding its output is let go a second
    /// after it ends, with what it said, and nothing is left waiting on a read. The times are far apart, so a busy
    /// Mac that's slow to start a program can't pass for one held up.
    func testARunIsntHeldByWhatItLeavesRunning() throws {
        let program = home.appendingPathComponent("lingers")
        try "#!/bin/sh\necho said\necho also >&2\nsleep 30 &\nexit 3\n".write(to: program, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: program.path)
        let start = Date()
        let ran = T3Code.run(program, [], 20)
        XCTAssertLessThan(Date().timeIntervalSince(start), 10, "not held till what it left ends, nor till its limit")
        XCTAssertEqual(ran.status, 3)
        XCTAssertEqual(ran.output, "said\n")
        XCTAssertEqual(ran.errors, "also\n")
        let slow = home.appendingPathComponent("slow")
        try "#!/bin/sh\necho started\nsleep 30\n".write(to: slow, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: slow.path)
        let stopped = Date()
        XCTAssertNil(T3Code.run(slow, [], 0.5).status, "stopped for taking too long")
        XCTAssertLessThan(Date().timeIntervalSince(stopped), 10, "at its limit, not when it ends")
    }

    // MARK: The folder, from the registry

    func testTheWorktreeComesFirstThenTheFolder() throws {
        let project = try folder("Projects/app"), worktree = try folder("Projects/app-hires/ada-1a2b3c")
        XCTAssertEqual(try T3Code.folder(of: hire(folder: project, worktree: worktree), home: home).get(), worktree)
        XCTAssertEqual(try T3Code.folder(of: hire(folder: project), home: home).get(), project)
        XCTAssertEqual(T3Code.title(of: hire(folder: project)), "Ada · hire/ada-1a2b3c")
        var named = hire(folder: project, branch: "")
        named.nickname = "Lovelace"
        XCTAssertEqual(T3Code.title(of: named), "Lovelace")
    }

    func testEachRefusal() throws {
        let refusal = { (h: FrontDesk.Hire) -> T3Code.Refusal? in
            if case .failure(let why) = T3Code.folder(of: h, home: self.home) { return why }
            return nil
        }
        XCTAssertEqual(refusal(hire(folder: home.appendingPathComponent("gone").path))?.message, "This hire's folder isn't there any more.")
        XCTAssertEqual(refusal(hire(folder: ""))?.status, 400)
        XCTAssertEqual(refusal(hire(folder: home.path))?.message, "That's your whole home folder: T3 Code opens a hire's own folder only.")
        XCTAssertEqual(refusal(hire(folder: home.path + "/"))?.status, 409, "the home folder, however it's written")
        XCTAssertEqual(refusal(hire(folder: try folder(".hidden")))?.message, "A hidden folder doesn't open in T3 Code.")
        XCTAssertEqual(refusal(hire(folder: try folder("Projects/.cache/app")))?.message, "A hidden folder doesn't open in T3 Code.", "inside one too")
        let outside = FileManager.default.temporaryDirectory.resolvingSymlinksInPath().path
        XCTAssertEqual(refusal(hire(folder: outside))?.message, "Only a folder inside your home folder opens in T3 Code.")
        // A link inside the home folder to a folder outside it is outside.
        let link = home.appendingPathComponent("Projects/away")
        try FileManager.default.createDirectory(at: link.deletingLastPathComponent(), withIntermediateDirectories: true)
        try FileManager.default.createSymbolicLink(atPath: link.path, withDestinationPath: outside)
        XCTAssertEqual(refusal(hire(folder: link.path))?.message, "Only a folder inside your home folder opens in T3 Code.")
        // A refused folder runs nothing, whatever is installed.
        try fakeT3()
        let answer = T3Code.open(hire(folder: home.path), status: T3Code.Status(state: .found), machine: machine(app: true))
        XCTAssertEqual(answer.status, 409)
        XCTAssertEqual(runs(), [])
        XCTAssertEqual(opened, 0)
    }

    // MARK: Opening, by state

    func testFoundOpensTheAppThenAThreadNeverThroughAShell() throws {
        try fakeT3()
        let tricky = try folder("Projects/My App; touch pwned")
        let answer = T3Code.open(hire(folder: tricky), status: T3Code.Status(state: .found), machine: machine(app: true))
        XCTAssertEqual(answer.status, 200)
        XCTAssertEqual(json(answer), ["ok": true, "opened": "thread"])
        XCTAssertEqual(opened, 1)
        XCTAssertEqual(runs(), [list(tricky), ["app", tricky]], "its threads before, then one argument for the folder, as it is")
        XCTAssertFalse(FileManager.default.fileExists(atPath: home.appendingPathComponent("Projects/pwned").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: "pwned"))
    }

    func testFoundWaitsForTheAppToAnswer() throws {
        try fakeT3(mode: "late")
        let project = try folder("Projects/app")
        let answer = T3Code.open(hire(folder: project), status: T3Code.Status(state: .found), machine: machine(app: true))
        XCTAssertEqual(json(answer), ["ok": true, "opened": "thread"])
        XCTAssertEqual(runs(), [list(project), ["app", project], ["app", project], ["app", project]])
    }

    /// T3 Code says renderer-unavailable while its window isn't up: tried again, as any failure but thread-open-failed,
    /// which is matched with its parentheses so a folder's name can't pass for it.
    func testRendererUnavailableIsTriedAgain() throws {
        try fakeT3(mode: "renderer")
        let project = try folder("Projects/thread-open-failed")
        let answer = T3Code.open(hire(folder: project), status: T3Code.Status(state: .found), machine: machine(app: true))
        XCTAssertEqual(answer.status, 200)
        XCTAssertEqual(json(answer), ["ok": true, "opened": "thread"])
        XCTAssertEqual(runs(), [list(project), ["app", project], ["app", project], ["app", project]], "three t3 app, no list after")
    }

    func testAT3ErrorIsA502WithItsFirstLineAloneAndTheWaitEnds() throws {
        try fakeT3(mode: "fail")
        let project = try folder("Projects/app")
        let start = clock
        let answer = T3Code.open(hire(folder: project), status: T3Code.Status(state: .found), machine: machine(app: true))
        XCTAssertEqual(answer.status, 502)
        XCTAssertEqual(text(answer), "Could not reach the T3 Code desktop app.")
        XCTAssertLessThanOrEqual(clock.timeIntervalSince(start), T3Code.openWithin + 1, "10 seconds at most")
        XCTAssertGreaterThan(runs().count, 1, "tried again while it waited")
        XCTAssertEqual(runs().first, list(project))
        XCTAssertTrue(runs().dropFirst().allSatisfy { $0 == ["app", project] })
        XCTAssertEqual(opened, 1)
    }

    /// T3 Code 0.1.8's `t3 app` says thread-open-failed even when it opened one, and that thread is a draft, which
    /// isn't listed. The folder's threads are listed before `t3 app` and once after, and nothing is tried again: a
    /// thread that's new since, made in the last 15 seconds, is "thread"; a list without one, "unsure"; a list that
    /// fails, T3 Code's error without its log prefix.
    func testAFalseFailureIsCheckedInTheThreadList() throws {
        try fakeT3(mode: "unsure")
        let project = try folder("Projects/My App; touch pwned")
        let iso = ISO8601DateFormatter()
        iso.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        /// A thread list: each thread's id, made that many seconds ago.
        func listed(_ threads: KeyValuePairs<String, TimeInterval>) -> String {
            "[" + threads.map { #"{"threadId":"\#($0.key)","createdAt":"\#(iso.string(from: clock.addingTimeInterval(-$0.value)))"}"# }
                .joined(separator: ",") + "]"
        }
        func open(before: String?, after: String?) throws -> Response {
            try? FileManager.default.removeItem(at: calls)
            for (file, list) in [(threads, before), (next, after)] {
                try? FileManager.default.removeItem(at: file)
                try list?.write(to: file, atomically: true, encoding: .utf8)
            }
            return T3Code.open(hire(folder: project), status: T3Code.Status(state: .found), machine: machine(app: true))
        }
        let want = [list(project), ["app", project], list(project)]
        for (before, after, why) in [(listed(["t-old": 3600]), listed(["t-old": 3600, "t1": 4]), "a new thread from 4 seconds ago"),
                                     (nil, listed(["t-old": 3600, "t1": 4]), "no list before: one from 4 seconds ago")] {
            XCTAssertEqual(json(try open(before: before, after: after)), ["ok": true, "opened": "thread"], why)
            XCTAssertEqual(runs(), want, "the list before, t3 app once, the list after once: \(why)")
        }
        for (before, after, why) in [("[]", "[]", "the draft, not listed"), (listed(["t1": 4]), listed(["t1": 4]), "a thread that was there before"),
                                     (listed(["t-old": 3600]), listed(["t-old": 3600, "t2": 20]), "a new one, but from 20 seconds ago")] {
            XCTAssertEqual(json(try open(before: before, after: after)), ["ok": true, "opened": "unsure"], why)
            XCTAssertEqual(runs(), want, why)
        }
        let failed = try open(before: nil, after: nil)
        XCTAssertEqual(failed.status, 502, "the list exits 1")
        XCTAssertEqual(text(failed), String("T3 Code could not open \(project) (thread-open-failed).".prefix(200)), "without T3 Code's log prefix")
        XCTAssertEqual(runs(), want, "not tried again")
        XCTAssertEqual(try open(before: "not json", after: nil).status, 502, "a list that isn't a list")
        XCTAssertEqual(T3Code.date("2026-11-11T01:33:17Z"), Date(timeIntervalSince1970: 1_794_360_797), "without milliseconds too")
        XCTAssertFalse(FileManager.default.fileExists(atPath: home.appendingPathComponent("Projects/pwned").path))
    }

    /// Every t3 error is said without T3 Code's log prefix; an error without one, as it is.
    func testT3CodesLogPrefixIsLeftOut() {
        let logged = T3Code.Ran(status: 1, errors: "[01:33:17.503] ERROR (#12): DesktopAppRequestFailedError: T3 Code could not open /a (renderer-unavailable).\n  at x")
        XCTAssertEqual(String(decoding: T3Code.failed(logged).body, as: UTF8.self), "T3 Code could not open /a (renderer-unavailable).")
        let plain = T3Code.Ran(status: 1, errors: "[note] Could not reach the T3 Code desktop app.")
        XCTAssertEqual(String(decoding: T3Code.failed(plain).body, as: UTF8.self), "[note] Could not reach the T3 Code desktop app.")
    }

    /// A second open for a hire while one runs is a 409 and runs nothing; another hire's isn't held up.
    func testOneOpenAtATimeForAHire() throws {
        try fakeT3()
        let project = try folder("Projects/app")
        let started = DispatchSemaphore(value: 0), release = DispatchSemaphore(value: 0)
        var m = machine(app: true)
        m.openApp = {}
        m.now = { Date() }
        let first = NSLock()
        var apps = 0
        m.run = { program, arguments, timeout in
            if arguments.first == "app" {
                first.lock(); apps += 1; let wait = apps == 1; first.unlock()
                if wait { started.signal(); release.wait() }
            }
            return T3CodeTests.runFake(program, arguments, timeout)
        }
        let found: (T3Code.Machine) -> T3Code.Status = { _ in T3Code.Status(state: .found) }
        let done = DispatchSemaphore(value: 0)
        var answer: Response?
        DispatchQueue.global().async { answer = T3Code.openOnce(self.hire(folder: project), machine: m, check: found); done.signal() }
        started.wait()
        let second = T3Code.openOnce(hire(folder: project), machine: m, check: found)
        XCTAssertEqual(second.status, 409)
        XCTAssertEqual(text(second), "Already opening T3 Code for this hire.")
        var other = hire(folder: project)
        other.id = "h2"
        XCTAssertEqual(T3Code.openOnce(other, machine: m, check: found).status, 200, "another hire's")
        release.signal()
        done.wait()
        XCTAssertEqual(answer?.status, 200)
        XCTAssertEqual(T3Code.openOnce(hire(folder: project), machine: m, check: found).status, 200, "once it's done, it opens again")
        XCTAssertEqual(runs().count, 6, "the refused open ran nothing: three opens, each a list and t3 app")
    }

    func testCliOnlyAddsAProjectWithItsTitle() throws {
        try fakeT3()
        let worktree = try folder("Projects/app-hires/ada 1a2b3c")
        let answer = T3Code.open(hire(folder: try folder("Projects/app"), worktree: worktree), status: T3Code.Status(state: .cliOnly), machine: machine(app: false))
        XCTAssertEqual(json(answer), ["ok": true, "opened": "project"])
        XCTAssertEqual(runs(), [["project", "add", worktree, "--title", "Ada · hire/ada-1a2b3c"]])
        XCTAssertEqual(opened, 0, "no app to open")
        // Already one of its projects: as asked.
        try "exists".write(to: mode, atomically: true, encoding: .utf8)
        XCTAssertEqual(json(T3Code.open(hire(folder: worktree), status: T3Code.Status(state: .cliOnly), machine: machine(app: false))), ["ok": true, "opened": "project"])
        // Any other error: its first line.
        try "fail".write(to: mode, atomically: true, encoding: .utf8)
        let failed = T3Code.open(hire(folder: worktree), status: T3Code.Status(state: .cliOnly), machine: machine(app: false))
        XCTAssertEqual(failed.status, 502)
        XCTAssertEqual(text(failed), "Could not reach the T3 Code desktop app.")
    }

    func testAppOnlyAndNoneSayWhatToInstallAndRunNothing() throws {
        let project = try folder("Projects/app")
        let appOnly = T3Code.open(hire(folder: project), status: T3Code.Status(state: .appOnly), machine: machine(app: true))
        XCTAssertEqual(appOnly.status, 409)
        XCTAssertEqual(text(appOnly), "In T3 Code, open Settings → About and install the t3 command.")
        let none = T3Code.open(hire(folder: project), status: T3Code.Status(state: .none), machine: machine(app: false))
        XCTAssertEqual(none.status, 409)
        XCTAssertEqual(text(none), "Install T3 Code to open this hire's folder there (t3.codes).")
        XCTAssertEqual(opened, 0)
        XCTAssertEqual(runs(), [])
    }

    /// `t3` runs with `--version`, `app`, `project add` or `thread list` only: no other call is in the source.
    func testT3RunsWithItsFourArgumentListsOnly() throws {
        let source = try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .appendingPathComponent("../../Sources/AgentDeck/T3Code.swift"), encoding: .utf8)
        let calls = source.components(separatedBy: "machine.run(program, ").dropFirst().map { String($0.prefix(while: { $0 != "]" })) + "]" }
        XCTAssertEqual(calls, [#"["--version"]"#, #"["project", "add", folder, "--title", title(of: hire)]"#, #"["app", folder]"#,
                               #"["thread", "list", "--project", folder, "--json"]"#])
        XCTAssertEqual(source.components(separatedBy: "machine.run(").count - 1, calls.count, "no other run")
        // The code, its comments left out (they say what's never done).
        let code = source.split(separator: "\n").filter { !$0.trimmingCharacters(in: .whitespaces).hasPrefix("//") }.joined(separator: "\n")
        for never in ["npx", "/bin/sh", "/bin/zsh", "/bin/bash", "\"-c\"", "\"-lc\"", "brew ", "npm", "curl"] {
            XCTAssertFalse(code.contains(never), never)
        }
    }

    // MARK: Routes and pages

    func testOnlyThePageOnThisComputerMayAskOrOpen() {
        for (method, path) in [("GET", "/setup/t3code"), ("POST", "/setup/t3code/open")] {
            XCTAssertEqual(SetupRoutes.row(method, path)?.callers, [.page], path)
            for caller in [SetupSessions.Caller.gm, .tool, .hire] { XCTAssertFalse(SetupRoutes.allows(caller, method, path), "\(path): \(caller.name)") }
        }
        // The office asks through its own window on this Mac, never a phone (`isOfficeWindow`, tested with the conversation).
        let phone = PairedPhone(id: "p1", name: "Phone", pairedAt: Date(), lastSeen: Date(), keyHash: "fake")
        let local = Request.addressed(method: "GET", path: "/t3code", query: [:], headers: ["host": "localhost:7710"], body: Data(),
                                      fromLoopback: true, remoteAddress: "127.0.0.1")
        XCTAssertTrue(DeckRouter.isOfficeWindow(local, phone: .officeWindow))
        XCTAssertFalse(DeckRouter.isOfficeWindow(local, phone: phone))
    }

    func testTheRouterGuardsTheOfficesRoutesAndTakesTheFolderFromTheRegistry() throws {
        let router = try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .appendingPathComponent("../../Sources/AgentDeck/DeckRouter.swift"), encoding: .utf8)
        for route in [#"case ("GET", "/t3code"):"#, #"case ("POST", "/t3code/open"):"#] {
            let at = try XCTUnwrap(router.range(of: route), route)
            XCTAssertTrue(router[at.upperBound...].prefix(200).contains("guard DeckRouter.isOfficeWindow(request, phone: phone) else"), route)
        }
        let open = try XCTUnwrap(router.range(of: #"case ("POST", "/setup/t3code/open"):"#))
        let body = String(router[open.upperBound...].prefix(250))
        XCTAssertTrue(body.contains("deck.frontDesk.hires.first { $0.id == id }"), "by id, from the registry")
        XCTAssertFalse(body.contains("folder") || body.contains("path"), "never a path from the request")
    }

    private func web(_ name: String) throws -> String {
        try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .appendingPathComponent("../../Resources/web/\(name)"), encoding: .utf8)
    }

    func testSetupsTeamListHasTheButtonItsHintsAndFitsAPhoneWidth() throws {
        let page = try web("setup.html")
        let row = try XCTUnwrap(page.range(of: "function teamRow(r) {"))
        let teamRow = String(page[row.lowerBound...].prefix(4500))
        XCTAssertTrue(teamRow.contains("const t3Key = state.t3CodeEnabled ? t3Button(r) : null;"), "T3 Code on (T796)")
        XCTAssertTrue(teamRow.contains("head.style.flexWrap = 'wrap';"), "390 px: the buttons go under the name")
        XCTAssertTrue(teamRow.contains("row.append(head, ...(t3Key ? [t3Key.line] : []), ...(why ? [why] : []));"), "the hint on its own line under the button")
        for words in ["textContent: 'Open in T3 Code', disabled: true", "fetch('/setup/t3code/open'", "JSON.stringify({ id: r.id })",
                      "'Opening T3 Code…'", "'T3 Code opens its own agent there, separate from this hire.'",
                      "Added to T3 Code's projects: open T3 Code to see it.", "t3.state === 'found' ? T3_NOTE : ''", ": t3.hint;",
                      #"unsure: "T3 Code has this hire's folder open as a project; it didn't confirm a new thread, so look in T3 Code.""#,
                      "const T3_EVERY = 30000", "`/setup/t3code${fresh ? '?fresh=1' : ''}`", "if (t3Key) row.addEventListener('pointerenter', () => t3State(true));"] {
            XCTAssertTrue(page.contains(words), words)
        }
        // Greyed for appOnly and none: only found and cliOnly may click.
        // (A hire in T3 Code, T797b, opens its own thread by T3 Code's link: that takes the app, and is greyed otherwise.)
        XCTAssertTrue(page.contains("      : t3.state === 'found' || t3.state === 'cliOnly');"))
        XCTAssertTrue(page.contains("button.disabled = open && open.busy ? true : !usable;"))
        // Fire… opened, the buttons' span shrinks, so its sentences wrap in the row (measured at 390 and 1280 px).
        XCTAssertTrue(teamRow.contains("keys.style.cssText = 'flex:0 1 auto;min-width:0;"))
        XCTAssertFalse(teamRow.contains("flex:none"))
        // The open outlives Setup's redraw every 1.5 s: kept by hire id outside the row, and restored as it's drawn.
        let button = try XCTUnwrap(page.range(of: "function t3Button(r) {"))
        let t3Button = String(page[button.lowerBound...].prefix(1600))
        for words in ["const t3Opening = new Map();"] { XCTAssertTrue(page.contains(words), words) }
        for words in ["paintT3Row(button, line);\n    button.onclick", "if ((t3Opening.get(r.id) || {}).busy) return;",
                      "t3Opening.set(r.id, { busy: true, text: 'Opening T3 Code…', at: Date.now() });",
                      "t3Opening.set(r.id, { busy: false, text, at: Date.now() });\n      paintT3();"] {
            XCTAssertTrue(t3Button.contains(words), words)
        }
        XCTAssertFalse(t3Button.contains("line.textContent ="), "the answer goes to the hire's open, never to the row it was clicked in")
        XCTAssertTrue(page.contains("const open = t3Opening.get(button.dataset.t3);"))
        XCTAssertFalse(teamRow.contains("worktree:") && teamRow.contains("/setup/t3code/open', { method: 'POST', headers: setupHeaders(true), body: JSON.stringify({ id: r.id, "),
                       "the id alone is sent")
    }

    func testTheOfficeOffersItOnlyInItsWindowOnThisComputerNeverOnAPhone() throws {
        let page = try web("deck.html")
        // The key exists only once the computer has answered /t3code, which it does to its own window alone; a phone never asks.
        XCTAssertTrue(page.contains("if (whole && !preview && !saver && !matchMedia('(pointer: coarse)').matches) {"))
        XCTAssertTrue(page.contains("if (!res || !res.ok) return;\n      t3Here = true;"))
        XCTAssertTrue(page.contains("...(t3Here ? [{ id: T3_BUTTON, label: 'T3 Code' }] : [])"))
        XCTAssertTrue(page.contains("if (t3Here && action === `button:${T3_BUTTON}`) return openT3(selected);"))
        XCTAssertEqual(page.components(separatedBy: "t3Here = true").count, 2, "set in one place")
        // The card: asked again as it opens, greyed with the hint, the line about T3 Code's agent, and the answers.
        for words in ["get('/t3code', { session: id, fresh: '1' })", "post('/t3code/open', { session: id })", "$('t3-open').disabled = !usable;",
                      "t3Say(usable ? (found.state === 'found' ? T3_NOTE : '') : found.hint);", "t3Say('Opening T3 Code…');",
                      "'T3 Code opens its own agent there, separate from this hire.'", "Added to T3 Code's projects: open T3 Code to see it.",
                      #"unsure: "T3 Code has this hire's folder open as a project; it didn't confirm a new thread, so look in T3 Code.""#,
                      #"<button class="t3-open" id="t3-open" disabled>Open in T3 Code</button>"#] {
            XCTAssertTrue(page.contains(words), words)
        }
        // 390 px: the card is a summary card, as wide as the screen allows.
        XCTAssertTrue(page.contains(#"<div class="summary" id="t3""#))
        XCTAssertTrue(page.contains("width: min(92vw, 420px)"))
    }
}
