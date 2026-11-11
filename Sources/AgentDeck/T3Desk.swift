import AppKit

/// The T3 Code desk (T797b, experimental): a hire whose agent runs in T3 Code, one thread for its one task, in a
/// worktree and branch of its own made as every hire's are (`HireDesk`). Smolcorp starts no agent itself here: it
/// uses T3 Code's own `t3` command, its arguments as a list, never through a shell, and only with the argument
/// lists `T3DeskTests` pins.
///
/// The fence is T797a's method A. T3 Code runs Claude Code with its permissions bypassed, and the deny rules and the
/// hook still apply from the worktree's `.claude/settings.local.json`. So that file gets exactly what `HireSettings`
/// gives the hire's own settings file (the deny rules, the guard before every tool call), it's kept out of commits by
/// the repo's info/exclude, and it's on disk BEFORE T3 Code hears of the folder. Then the worktree is added as a T3
/// project of its own and the thread started there, without T3's --worktree. If the fence or the project can't be
/// confirmed, everything made for the hire is taken back; a thread never starts without the fence in place.
///
/// Only Claude Code reads that file, so a hire in T3 Code is always on T3 Code's Claude provider.
enum T3Desk {
    typealias Refusal = FrontDesk.Refusal

    /// The registry's word for a hire whose agent runs in T3 Code.
    static let engine = "t3"
    /// The `t3` versions the fence was proven on (T797a). Any other is refused, by name.
    static let supported: Set<String> = ["v0.1.8"]
    /// T3 Code's Claude provider: the fence is Claude Code's settings file, which no other agent reads.
    static let provider = "claudeAgent"
    /// The model when neither the hire nor its role names one.
    static let defaultModel = "claude-sonnet-5-5"
    /// The fence, in the worktree, as git and Claude Code name it.
    static let fenceName = ".claude/settings.local.json"
    static let tag = "EXPERIMENTAL · runs in T3 Code"
    /// How often a hire's thread is looked at, at most.
    static let followEvery: TimeInterval = 10
    /// How long one `t3` may take.
    static let within: TimeInterval = 30
    /// How long, after its project is removed, T3 Code's agent has to leave the worktree.
    static let leaveWithin: TimeInterval = 5

    static let switchedOff = "T3 Code is off in Settings: turn it on to hire into T3 Code."

    // MARK: The gate

    /// Why a hire into T3 Code isn't offered or taken now, in one sentence; nil when it is. `on` is the "T3 Code" choice
    /// (`T3Code.laneAllowed`), `status` what's installed, with `t3`'s version.
    static func refusal(on: Bool, status: T3Code.Status) -> String? {
        guard on else { return switchedOff }
        switch status.state {
        case .none: return "Install T3 Code and its t3 command to hire into T3 Code (t3.codes)."
        case .appOnly: return "Hiring into T3 Code needs its t3 command: in T3 Code, open Settings → About and install it."
        case .found, .cliOnly: break
        }
        guard supported.contains(status.version) else {
            return status.version.isEmpty ? "Smolcorp couldn't read t3's version, so it won't hire into T3 Code."
                : "t3 \(status.version) isn't a version Smolcorp's fence was proven on (\(supported.sorted().joined(separator: ", "))), so it won't hire into T3 Code."
        }
        return nil
    }

    // MARK: Hiring

    /// Why this hire can't go into T3 Code, whatever's installed: only a Claude Code role (the fence is Claude Code's
    /// file), on T3 Code's own account, never the General Manager, never from a test copy (T3 Code is the real one).
    static func hireRefusal(role: Role?, account: String, apiKey: Bool, testCopy: Bool = TestCopy.isOn) -> String? {
        if testCopy { return "A test copy doesn't hire into T3 Code: that's your real T3 Code." }
        guard let role else { return "Pick a role: a hire in T3 Code starts on its brief." }
        if role.id == Deck.gmRole { return "The General Manager isn't hired into T3 Code." }
        if (Agent(rawValue: role.agent) ?? .claude) != .claude { return "Only a Claude Code role is hired into T3 Code: the fence there is Claude Code's." }
        if apiKey || !account.isEmpty { return "A hire in T3 Code works on T3 Code's own Claude account, so no account is picked for it." }
        return nil
    }

    /// One hire into T3 Code, as `Deck.hire` has checked it.
    struct Hiring {
        var hireID: String
        /// Its worktree's and branch's name: the nickname, else the role.
        var name: String
        /// What T3 Code's project is called by: "<name> · <branch>" (see `T3Code.title`).
        var called: String
        /// The folder it's hired into, inside a git repo.
        var folder: String
        var model: String
        /// The folder set for worktrees, "" for beside the repo; the ports other hires hold.
        var worktrees = ""
        var taken: [String] = []
    }

    /// What a hire into T3 Code does besides `t3` and git, so tests give their own. The app's go to its main queue.
    struct Ways {
        var machine = T3Code.Machine()
        var git: HireDesk.Git = HireDesk.git
        var portIsFree: (Int) -> Bool = HireDesk.portIsFree
        /// The hire's settings, as `HireSettings.object` makes them for it.
        var settings: () throws -> [String: Any]
        /// Writes its brief (as every managed hire's, in its folder in Smolcorp's), and gives its first message and the brief's path.
        var brief: (HireDesk.Ready) throws -> (message: String, path: String)
        var discardBrief: () -> Void
        /// Its registry entry, with engine t3: written once its worktree is made.
        var register: (HireDesk.Ready) -> Void
        /// Its thread, link and brief, once the thread runs in its worktree.
        var started: (_ thread: String, _ link: String, _ brief: String) -> Void
        /// It didn't start: the registry says why.
        var fail: (String) -> Void
    }

    struct Started: Equatable {
        var ready: HireDesk.Ready
        var thread: String
        var link: String
    }

    /// Hires into T3 Code, in T797a's order: the worktree and branch, the registry entry, the fence and its exclude line,
    /// the brief, `t3 project add`, `t3 thread start` without --worktree, then a check that the thread runs in the
    /// worktree. Refused at any step with everything made taken back. Runs git and `t3`: never on the main thread.
    static func hire(_ hiring: Hiring, ways: Ways) -> Result<Started, Refusal> {
        let machine = ways.machine, git = ways.git
        guard let program = T3Code.program(machine) else { return .failure(Refusal(status: 409, message: T3Code.noneHint)) }
        // 1. The worktree and branch, named as every hire's. T3 Code's agent works on T3's own Claude account, so no
        //    account of Smolcorp's is trusted or checked for it.
        let request = HireDesk.Request(hireID: hiring.hireID, name: hiring.name, folder: hiring.folder, claude: false,
                                       account: ClaudeProfiles.defaultProfile(), apiKey: false, worktree: true)
        let ready: HireDesk.Ready
        do { ready = try HireDesk.prepare(request, worktrees: hiring.worktrees, taken: hiring.taken, git: git, portIsFree: ways.portIsFree) } catch {
            return .failure(error as? Refusal ?? Refusal(status: 500, message: error.localizedDescription))
        }
        guard !ready.worktree.isEmpty else {
            return .failure(Refusal(status: 409, message: "A hire in T3 Code works in a worktree of its own: hire into a folder in a git repository."))
        }
        let worktree = ready.worktree
        let fence = URL(fileURLWithPath: worktree).appendingPathComponent(fenceName)
        let claudeFolder = fence.deletingLastPathComponent()
        let madeClaudeFolder = !FileManager.default.fileExists(atPath: claudeFolder.path)
        let title = "\(hiring.called) · \(ready.branch)"
        // Only a fence Smolcorp wrote is ever taken away: never a file the repo had.
        var wroteFence = false
        // 2. The registry entry, with engine t3.
        ways.register(ready)

        /// Takes back what was made, says why in the registry, and refuses. A project T3 Code may still have keeps its fence and worktree.
        func takeBack(_ status: Int, _ why: String, project: Bool = false, brief: Bool = false) -> Result<Started, Refusal> {
            var message = why
            if project {
                let removed = machine.run(program, ["project", "remove", worktree, "--force"], within)
                if removed.status != 0 {
                    message += " T3 Code didn't remove the project it was given (\(said(removed))), so the hire's worktree and fence were kept: remove \"\(title)\" in T3 Code."
                    ways.fail(message)
                    return .failure(Refusal(status: status, message: message))
                }
            }
            if wroteFence { removeFence(fence, folder: madeClaudeFolder ? claudeFolder : nil) }
            if brief { ways.discardBrief() }
            if !HireDesk.removeWorkspace(repo: ready.repo, worktree: worktree, branch: ready.branch, git: git) {
                message += " Its worktree \(worktree) was kept: git wouldn't remove it."
            }
            ways.fail(message)
            return .failure(Refusal(status: status, message: message))
        }

        // 3. The fence, before T3 Code hears of the folder: never in a file the repo already has (it would be committed).
        guard !FileManager.default.fileExists(atPath: fence.path) else {
            return takeBack(409, "This repository keeps its own \(fenceName), so Smolcorp can't put the hire's fence there.")
        }
        let settings: [String: Any]
        do { settings = try ways.settings() } catch {
            return takeBack((error as? Refusal)?.status ?? 500, (error as? Refusal)?.message ?? "The hire's fence couldn't be made, so it wasn't started.")
        }
        wroteFence = true
        guard writeFence(settings, to: fence) else {
            return takeBack(500, "The hire's fence couldn't be written in its worktree, so it wasn't started.")
        }
        // 4. Never committed: the repo's info/exclude, which every worktree of it shares.
        guard exclude(worktree: worktree, git: git) else {
            return takeBack(500, "Smolcorp couldn't keep the hire's fence out of git, so it wasn't started.")
        }
        // 5. The brief, as every managed hire's.
        let brief: (message: String, path: String)
        do { brief = try ways.brief(ready) } catch {
            return takeBack(500, error.localizedDescription, brief: true)
        }
        // 6. The worktree as a T3 project of its own: then the thread can't land in a project around it (T797a 2a).
        let added = machine.run(program, ["project", "add", worktree, "--title", title], within)
        guard added.status == 0 else {
            // "already exists" is someone else's project: it's left alone.
            return takeBack(502, "T3 Code didn't add the hire's worktree as a project: \(said(added))", brief: true)
        }
        guard let project = addedProject(added.output, folder: worktree) else {
            return takeBack(502, "T3 Code didn't say it added the hire's worktree as its own project, so the thread wasn't started.", project: true, brief: true)
        }
        // 7. The thread, in that folder, told to read its brief.
        let start = machine.run(program, ["thread", "start", worktree, brief.message, "--provider", provider, "--model", hiring.model, "--json"], within)
        guard start.status == 0, let thread = object(start.output)?["threadId"] as? String, !thread.isEmpty else {
            return takeBack(502, "T3 Code didn't start the hire's thread: \(said(start))", project: true, brief: true)
        }
        // 8. It runs in the worktree: T3 made no worktree of its own, and the thread is in the project just added.
        guard let shown = show(thread, program: program, machine: machine),
              shown["worktreePath"] == nil || shown["worktreePath"] is NSNull, shown["projectId"] as? String == project else {
            return takeBack(502, "Smolcorp couldn't confirm the thread runs in the hire's worktree, so it was taken back.", project: true, brief: true)
        }
        let link = shown["link"] as? String ?? ""
        ways.started(thread, link, brief.path)
        return .success(Started(ready: ready, thread: thread, link: link))
    }

    /// The fence written whole, private to you, and read back the same.
    static func writeFence(_ settings: [String: Any], to fence: URL) -> Bool {
        guard let data = try? JSONSerialization.data(withJSONObject: settings, options: [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]),
              (try? FileManager.default.createDirectory(at: fence.deletingLastPathComponent(), withIntermediateDirectories: true)) != nil,
              Paths.writePrivately(data, to: fence),
              let back = try? Data(contentsOf: fence), let read = try? JSONSerialization.jsonObject(with: back) else { return false }
        return JSONText.same(read, settings)
    }

    /// The fence and, if Smolcorp made it, its .claude folder (only while empty).
    static func removeFence(_ fence: URL, folder: URL?) {
        try? FileManager.default.removeItem(at: fence)
        if let folder, (try? FileManager.default.contentsOfDirectory(atPath: folder.path))?.isEmpty == true {
            try? FileManager.default.removeItem(at: folder)
        }
    }

    /// `.claude/settings.local.json` in the repo's info/exclude (once), then git asked that it's ignored and not listed.
    static func exclude(worktree: String, git: HireDesk.Git) -> Bool {
        guard let common = git(worktree, ["rev-parse", "--git-common-dir"]), common.status == 0 else { return false }
        let named = common.output.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !named.isEmpty else { return false }
        let base = named.hasPrefix("/") ? URL(fileURLWithPath: named) : URL(fileURLWithPath: worktree).appendingPathComponent(named)
        let file = base.appendingPathComponent("info/exclude")
        var text = (try? String(contentsOf: file, encoding: .utf8)) ?? ""
        if !text.split(whereSeparator: \.isNewline).contains(where: { $0.trimmingCharacters(in: .whitespaces) == fenceName }) {
            if !text.isEmpty, !text.hasSuffix("\n") { text += "\n" }
            text += fenceName + "\n"
            guard (try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)) != nil,
                  (try? text.write(to: file, atomically: true, encoding: .utf8)) != nil else { return false }
        }
        guard git(worktree, ["check-ignore", "-q", fenceName])?.status == 0,
              let listed = git(worktree, ["status", "--porcelain", "--untracked-files=all", "--", ".claude"]), listed.status == 0 else { return false }
        return listed.output.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    /// The project's id when `t3 project add` says it added `folder` itself: "Added project <id> (<title>) at <folder>."
    /// (a title or a folder may hold " at ", so the folder is matched whole at the end); nil otherwise.
    static func addedProject(_ output: String, folder: String) -> String? {
        let ends = Set([folder, HireDesk.physical(folder)].map { " at \($0)." })
        guard let line = output.split(whereSeparator: \.isNewline).map({ $0.trimmingCharacters(in: .whitespaces) })
            .first(where: { line in line.hasPrefix("Added project ") && ends.contains { line.hasSuffix($0) } }) else { return nil }
        let id = line.dropFirst("Added project ".count).prefix { $0 != " " }
        return id.isEmpty ? nil : String(id)
    }

    /// The thread as `t3 thread show <id> --json` gives it, or nil.
    static func show(_ thread: String, program: URL, machine: T3Code.Machine) -> [String: Any]? {
        let ran = machine.run(program, ["thread", "show", thread, "--json"], within)
        guard ran.status == 0 else { return nil }
        return object(ran.output)?["thread"] as? [String: Any]
    }

    static func object(_ text: String) -> [String: Any]? {
        (try? JSONSerialization.jsonObject(with: Data(text.utf8))) as? [String: Any]
    }

    /// The first line `t3` said, on its errors first, cut to 200 characters; what's said when it said nothing.
    static func said(_ ran: T3Code.Ran) -> String {
        let line = T3Code.firstLine(ran.errors.isEmpty ? ran.output : ran.errors)
        return line.isEmpty ? (ran.status == nil ? "it didn't answer" : "it gave no reason") : String(line.prefix(200))
    }

    // MARK: Following

    /// A thread as the desk shows it.
    enum Status: String {
        case starting, working, waiting, done, unknown
    }

    /// T3 Code's thread status as the desk says it (T797a §4).
    static func status(of thread: [String: Any]) -> Status {
        let raw = thread["status"] as? String ?? ""
        if raw == "waiting" || (thread["pendingRequestCount"] as? Int ?? 0) > 0 { return .waiting }
        if ["preparing", "queued", "starting", "running"].contains(raw) { return .working }
        if ["completed", "failed", "interrupted", "cancelled", "rolled_back", "idle"].contains(raw) { return .done }
        return .unknown
    }

    /// What the last look at a hire's thread found.
    struct Seen: Equatable {
        var status: Status
        /// T3 Code's own word for it ("running"), or why it couldn't be seen.
        var raw: String
        var link = ""
    }

    private static let lock = NSLock()
    nonisolated(unsafe) private static var seen: [String: Seen] = [:]
    nonisolated(unsafe) private static var looking = false
    static let queue = DispatchQueue(label: "smolcorp.t3-desk")

    static func lastSeen(_ hireID: String) -> Seen? {
        lock.lock(); defer { lock.unlock() }
        return seen[hireID]
    }

    /// Looks at each hire's thread, one at a time, unless a look is still running: then nothing (so each hire is looked
    /// at no more than every `followEvery`). Gives what was seen. Runs `t3`: never on the main thread.
    static func follow(_ hires: [FrontDesk.Hire], machine: T3Code.Machine = T3Code.Machine()) -> [(id: String, seen: Seen)]? {
        lock.lock()
        if looking { lock.unlock(); return nil }
        looking = true
        lock.unlock()
        defer { lock.lock(); looking = false; lock.unlock() }
        let program = T3Code.program(machine)
        var found: [(id: String, seen: Seen)] = []
        for hire in hires where hire.engine == engine && !hire.t3Thread.isEmpty && hire.state != .fired && hire.state != .failed {
            let look: Seen
            if let program, let thread = show(hire.t3Thread, program: program, machine: machine) {
                look = Seen(status: status(of: thread), raw: thread["status"] as? String ?? "", link: thread["link"] as? String ?? "")
            } else {
                look = Seen(status: .unknown, raw: program == nil ? "t3 isn't there" : "T3 Code didn't answer")
            }
            lock.lock(); seen[hire.id] = look; lock.unlock()
            found.append((hire.id, look))
        }
        return found
    }

    /// The registry's state for what was seen: at work or waiting is working, done is idle; unknown changes nothing.
    static func state(for status: Status) -> FrontDesk.State? {
        switch status {
        case .working, .waiting: return .working
        case .done: return .idle
        case .starting, .unknown: return nil
        }
    }

    // MARK: The desk

    /// What a T3 hire's desk shows besides its name and branch: the tag, its status, and whether it has a thread to open.
    static func deskJSON(_ hire: FrontDesk.Hire) -> [String: Any] {
        let look = lastSeen(hire.id)
        return ["tag": tag, "status": (look?.status ?? (hire.t3Thread.isEmpty ? .starting : .unknown)).rawValue,
                "raw": look?.raw ?? "", "thread": hire.t3Thread, "canOpen": deepLink(thread: hire.t3Thread, link: hire.t3Link) != nil]
    }

    /// The team as the page gets it, each T3 hire's row with its desk (`t3`).
    static func decorate(_ team: [String: Any], hires: [FrontDesk.Hire]) -> [String: Any] {
        guard var rows = team["team"] as? [[String: Any]] else { return team }
        for i in rows.indices {
            guard rows[i]["engine"] as? String == engine, let hire = hires.first(where: { $0.id == rows[i]["id"] as? String }) else { continue }
            rows[i]["t3"] = deskJSON(hire)
        }
        var out = team
        out["team"] = rows
        return out
    }

    /// T3 Code's own link to a thread: `t3code://threads/<env>/<thread id, URL-encoded>`, the env being the first part
    /// of `link`'s path after /threads/ (https://app.t3.codes/threads/<env>/<thread>). Nil without both.
    static func deepLink(thread: String, link: String) -> URL? {
        guard !thread.isEmpty, let url = URL(string: link) else { return nil }
        let parts = url.pathComponents
        guard let at = parts.firstIndex(of: "threads"), at + 1 < parts.count else { return nil }
        let env = parts[at + 1]
        guard !env.isEmpty, env.allSatisfy({ $0.isASCII && ($0.isLetter || $0.isNumber || $0 == "-") }) else { return nil }
        var allowed = CharacterSet.alphanumerics.intersection(CharacterSet(charactersIn: Unicode.Scalar(0)...Unicode.Scalar(127)))
        allowed.insert(charactersIn: "-._~")
        guard let encoded = thread.addingPercentEncoding(withAllowedCharacters: allowed) else { return nil }
        return URL(string: "t3code://threads/\(env)/\(encoded)")
    }

    /// "Open in T3 Code" for a hire in T3 Code: its thread, by T3 Code's own link, opened with NSWorkspace. Never a shell.
    static func openThread(_ hire: FrontDesk.Hire, status: T3Code.Status, machine: T3Code.Machine) -> Response {
        guard status.state == .found || status.state == .appOnly else {
            return .status(409, "T3 Code's app isn't on this Mac, so the hire's thread can't be shown there.")
        }
        guard let url = deepLink(thread: hire.t3Thread, link: lastSeen(hire.id)?.link.nonEmpty ?? hire.t3Link) else {
            return .status(409, "T3 Code hasn't given this hire's thread a link yet.")
        }
        return machine.openURL(url) ? .json(["ok": true, "opened": "link"]) : .status(502, "T3 Code didn't take the link to the hire's thread.")
    }

    // MARK: Mail

    /// The GM's mail to a hire in T3 Code: `t3 thread send <id> -`, the text on its standard input, never typed anywhere.
    /// A busy thread takes it as its next turn: delivered, and the outbox says it's queued there.
    static func send(_ hire: FrontDesk.Hire, _ text: String, machine: T3Code.Machine = T3Code.Machine()) -> MailRouter.Delivery {
        let name = HireDesk.called(hire)
        guard !hire.t3Thread.isEmpty else { return .failed("\(name) has no T3 Code thread, so mail can't reach it.") }
        guard let program = T3Code.program(machine) else { return .trouble("T3 Code's t3 command isn't there, so mail can't reach \(name) now.") }
        let ran = machine.feed(program, ["thread", "send", hire.t3Thread, "-"], Data(text.utf8), within)
        // No answer: it may have been taken, so it isn't sent again.
        guard let status = ran.status else { return .failed("T3 Code didn't answer, so it isn't known whether \(name) got it; it won't be sent again.") }
        guard status == 0 else { return .trouble("T3 Code didn't take it: \(said(ran))") }
        return T3Code.firstLine(ran.output).hasPrefix("Queued") ? .deliveredAs("queued: \(name)'s thread in T3 Code was busy, so it's its next turn.") : .delivered
    }

    // MARK: Letting go

    static let letGoConfirm = "T3 Code's thread for this hire, and its history in T3 Code, are removed. Its worktree and branch stay."

    /// What letting go uses besides `t3`: the processes now (read from the kernel, by PID), and the clock.
    struct Leaving {
        var machine = T3Code.Machine()
        var working: (String) -> [pid_t] = Processes.working(in:)
        var pause: (TimeInterval) -> Void = { Thread.sleep(forTimeInterval: $0) }
        var now: () -> Date = Date.init
    }

    /// Lets a hire in T3 Code go: T3 has no stop, so its project is removed with --force (which ends T3's agent for it,
    /// T797a), then no process may still work in its worktree; only then is the fence taken away, the recap written and
    /// the registry told. Any step that fails says so and stops there. Never on the main thread. Gives the recap's path.
    static func letGo(_ hire: FrontDesk.Hire, leaving: Leaving = Leaving(), recaps: URL = Paths.recaps,
                      letGone: (String) -> Void) -> Result<String, Refusal> {
        let name = HireDesk.called(hire)
        guard hire.engine == engine else { return .failure(Refusal(status: 409, message: "\(name) doesn't run in T3 Code.")) }
        guard hire.state != .fired, hire.state != .failed else { return .failure(Refusal(status: 409, message: "\(name) is already gone.")) }
        guard !hire.worktree.isEmpty else { return .failure(Refusal(status: 409, message: "\(name) has no worktree on record, so nothing was changed.")) }
        guard let program = T3Code.program(leaving.machine) else {
            return .failure(Refusal(status: 409, message: "T3 Code's t3 command isn't there, so nothing was changed: \(name) may still be running in T3 Code."))
        }
        let removed = leaving.machine.run(program, ["project", "remove", hire.worktree, "--force"], within)
        // A project already removed in T3 Code is as good as removed now.
        guard removed.status == 0 || (removed.output + removed.errors).contains("No T3 Code project matches") else {
            return .failure(Refusal(status: 502, message: "T3 Code didn't remove \(name)'s project (\(said(removed))), so nothing else was changed: it may still be running there."))
        }
        let end = leaving.now().addingTimeInterval(leaveWithin)
        var still = leaving.working(hire.worktree)
        while !still.isEmpty, leaving.now() < end {
            leaving.pause(0.5)
            still = leaving.working(hire.worktree)
        }
        guard still.isEmpty else {
            let pids = still.prefix(5).map(String.init).joined(separator: ", ")
            return .failure(Refusal(status: 409, message: "T3 Code removed the project, but a process still works in \(name)'s worktree (PID \(pids)), so its fence was kept and it isn't let go yet."))
        }
        let fence = URL(fileURLWithPath: hire.worktree).appendingPathComponent(fenceName)
        removeFence(fence, folder: fence.deletingLastPathComponent())
        let recap = HireDesk.writeRecap(hire, capture: nil, worktreeRemoved: false, kept: "", in: recaps, now: leaving.now())
        letGone(recap)
        return .success(recap)
    }
}

extension T3Code {
    /// The "T3 Code" choice in Settings (T796's `t3CodeEnabled`), the one place the T3 Code desk reads it. Config has
    /// no such setting before T796 is merged; until then the desk is allowed, and the rest of the gate still applies.
    static func laneAllowed(_ config: Config) -> Bool {
        Mirror(reflecting: config).children.first { $0.label == "t3CodeEnabled" }?.value as? Bool ?? true
    }
}

extension Processes {
    /// The processes working in `folder` (or a folder inside it), or given it with `--add-dir`, as T3 Code's agent is
    /// (T797a): read from the kernel by PID, nothing run.
    static func working(in folder: String) -> [pid_t] {
        let wanted = HireDesk.physical(folder)
        guard wanted.hasPrefix("/") else { return [] }
        return allPIDs().filter { pid in
            if let here = workingDirectory(of: pid), here == wanted || here.hasPrefix(wanted + "/") { return true }
            let words = arguments(of: pid)
            return zip(words, words.dropFirst() + [""]).contains { word, next in
                word == "--add-dir=\(wanted)" || word == "--add-dir=\(folder)" || (word == "--add-dir" && (next == wanted || next == folder))
            }
        }
    }

    /// A process's arguments, from the kernel (KERN_PROCARGS2); empty when they can't be read.
    static func arguments(of pid: pid_t) -> [String] {
        var mib: [Int32] = [CTL_KERN, KERN_PROCARGS2, pid]
        var size = 0
        guard sysctl(&mib, 3, nil, &size, nil, 0) == 0, size > MemoryLayout<Int32>.size else { return [] }
        var buffer = [UInt8](repeating: 0, count: size)
        guard sysctl(&mib, 3, &buffer, &size, nil, 0) == 0, size > MemoryLayout<Int32>.size else { return [] }
        let count = buffer.withUnsafeBytes { $0.loadUnaligned(as: Int32.self) }
        var i = MemoryLayout<Int32>.size
        // The program's path, then the NULs that pad it, then the arguments.
        while i < size, buffer[i] != 0 { i += 1 }
        while i < size, buffer[i] == 0 { i += 1 }
        var words: [String] = []
        while words.count < Int(count), i < size {
            let start = i
            while i < size, buffer[i] != 0 { i += 1 }
            words.append(String(decoding: buffer[start..<i], as: UTF8.self))
            i += 1
        }
        return words
    }
}

private extension String {
    var nonEmpty: String? { isEmpty ? nil : self }
}
