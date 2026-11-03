import Foundation

/// Firing a hire (C17 to C21): one path for the office, the phone, setup and
/// the General Manager's `POST /setup/gm/fire`.
///
/// - It refuses, and changes nothing, when the hire's worktree holds changes
///   nobody committed (the files are listed) unless `force`, and when its
///   window shows a question or a menu. A menu is never pressed over: the
///   hire is asked nothing, and nothing is sent.
/// - Otherwise `/exit` is pasted and Return pressed, only when the input box
///   is idle and empty; then it waits. Only that hire's own tmux server is
///   killed, and only if it is still there.
/// - The desk is freed (the registry says `fired`), the worktree is kept
///   unless asked to remove it, and then only if clean, and a short recap is
///   written to `recaps/<hire>.md`.
extension HireDesk {
    struct FireOptions: Equatable {
        /// Fire although the worktree holds changes nobody committed.
        var force = false
        /// Remove the worktree and its branch afterwards, when clean.
        var removeWorktree = false
    }

    /// How a hire's input box looks in its window.
    enum Input: Equatable {
        /// Nothing typed: `/exit` may be pasted.
        case idleAndEmpty
        /// At work: a paste would only queue up behind it.
        case busy
        /// Someone has typed something that isn't sent yet.
        case draft
        /// No input box can be told apart.
        case unknown
    }

    /// How long `/exit` has to end the session before its server is killed.
    static let exitWithin: TimeInterval = 10

    // MARK: What the window shows

    /// The last `count` lines of a capture that have words in them.
    private static func tail(_ capture: String, _ count: Int) -> [String] {
        let lines = capture.split(omittingEmptySubsequences: false, whereSeparator: \.isNewline).map(String.init)
            .filter { !$0.trimmingCharacters(in: .whitespaces).isEmpty }
        return Array(lines.suffix(count))
    }

    /// What Claude Code's window looks like at the bottom, read from its structure and never from its words
    /// (a mail text or old output in the scrollback may say "do you want to proceed?" and mean nothing).
    private struct Bottom {
        /// The lines of the input box between the last two rules, when the window ends in one: its first
        /// line starts with `❯`, and only the footer (mode line, a status line) follows the lower rule.
        var box: [String]?
        /// The footer under the box.
        var footer: [String] = []
        /// The last lines that have words in them, when there is no box.
        var tail: [String] = []
    }

    /// A full-width rule: a long run of `─` and nothing else.
    private static func isRule(_ line: String) -> Bool {
        let text = line.trimmingCharacters(in: .whitespaces)
        return text.count >= 20 && text.allSatisfy { $0 == "─" }
    }

    /// Claude Code's `❯` selector, with what follows it; nil when the line doesn't start with one.
    private static func afterSelector(_ line: String) -> String? {
        let text = line.trimmingCharacters(in: .whitespaces)
        guard text.first == "❯" else { return nil }
        return text.dropFirst().trimmingCharacters(in: .whitespaces)
    }

    private static func bottom(of capture: String) -> Bottom {
        var lines = capture.split(omittingEmptySubsequences: false, whereSeparator: \.isNewline).map(String.init)
        while let last = lines.last, last.trimmingCharacters(in: .whitespaces).isEmpty { lines.removeLast() }
        let rules = lines.indices.filter { isRule(lines[$0]) }
        if rules.count >= 2 {
            let upper = rules[rules.count - 2], lower = rules[rules.count - 1]
            let footer = Array(lines[(lower + 1)...])
            let inside = lines[(upper + 1)..<lower].filter { !$0.trimmingCharacters(in: .whitespaces).isEmpty }
            // The box sits at the bottom: a footer of a line or two under it, not a screenful.
            if footer.count <= 3, let first = inside.first, afterSelector(first) != nil {
                return Bottom(box: Array(inside), footer: footer)
            }
        }
        return Bottom(box: nil, footer: [], tail: Array(lines.filter { !$0.trimmingCharacters(in: .whitespaces).isEmpty }.suffix(15)))
    }

    /// Why nothing may be sent to a window: it shows a question or a menu, or can't be seen. Nil when it shows neither.
    /// A menu or a question is told by its shape: no input box at the bottom, and either a numbered list with the
    /// `❯ N.` selector or a confirm/cancel footer. Words anywhere else in the window count for nothing.
    static func promptRefusal(_ capture: String?) -> String? {
        guard let capture else { return "Couldn't look at its window, so nothing was sent. Try again in a moment." }
        let bottom = bottom(of: capture)
        guard bottom.box == nil else { return nil }
        let numbered = bottom.tail.contains { line in
            afterSelector(line)?.range(of: #"^\d+[.)]\s"#, options: .regularExpression) != nil
        }
        let footer = (bottom.tail.last ?? "").lowercased()
        if numbered || footer.contains("enter to confirm") || footer.contains("esc to cancel")
            || footer.contains("enter to select") || footer.contains("to navigate") {
            return "It is waiting for an answer in its window. Answer it there first; nothing was sent."
        }
        return nil
    }

    /// The state of the input box in a capture: the `❯` line between the last two rules, and the footer under it.
    /// A box that shows only the grey placeholder is empty: a capture has no colours, so any text that is exactly
    /// `Try "…"` on the box's one line counts as the placeholder.
    static func inputState(_ capture: String?) -> Input {
        guard let capture else { return .unknown }
        let bottom = bottom(of: capture)
        guard let box = bottom.box, let first = box.first, let typed = afterSelector(first) else { return .unknown }
        let footer = bottom.footer
        // Every real window has a footer under its box: with none, the screen was cut off or half drawn, and
        // "esc to interrupt" may be in the part that's missing. In doubt, nothing is typed.
        if footer.allSatisfy({ $0.trimmingCharacters(in: .whitespaces).isEmpty }) { return .unknown }
        if footer.contains(where: { $0.lowercased().contains("esc to interrupt") }) { return .busy }
        if box.count > 1 { return .draft }
        return typed.isEmpty || typed.range(of: #"^Try ".*"$"#, options: .regularExpression) != nil ? .idleAndEmpty : .draft
    }

    /// The cost the window shows, such as "Total cost: $1.23", or nil.
    static func visibleCost(_ capture: String?) -> String? {
        guard let capture, let found = capture.range(of: #"(?i)(total )?cost[: ]+\$\d+(\.\d+)?"#, options: [.regularExpression, .backwards]) else { return nil }
        return capture[found].range(of: #"\$\d+(\.\d+)?"#, options: .regularExpression).map { String(capture[found][$0]) }
    }

    // MARK: Refusals

    /// The uncommitted files of the hire's worktree, as git lists them; nil when git couldn't say.
    static func uncommittedFiles(_ worktree: String, git: Git = HireDesk.git) -> [String]? {
        guard !worktree.isEmpty, let answer = git(worktree, ["status", "--porcelain"]), answer.status == 0 else { return nil }
        return answer.output.split(whereSeparator: \.isNewline).map(String.init)
    }

    /// What a fire's answers call a hire: its nickname, else its name.
    static func called(_ hire: FrontDesk.Hire) -> String { hire.nickname.isEmpty ? hire.name : hire.nickname }

    /// A reason as it reads after "…: ": its first letter lower case ("It holds…" becomes "it holds…").
    /// The pronoun stays: "I can't…", "I'm…" and "I’ll…" are kept as written.
    static func afterColon(_ reason: String) -> String {
        let firstWord = reason.prefix { $0 != " " }
        if firstWord == "I" || firstWord.hasPrefix("I'") || firstWord.hasPrefix("I\u{2019}") { return reason }
        guard let first = reason.first, first.isUppercase, reason.dropFirst().first.map({ !$0.isUppercase }) ?? true else { return reason }
        return first.lowercased() + reason.dropFirst()
    }

    /// Why this hire may not be fired now, or nil. Reads only: nothing is sent or changed.
    static func fireRefusal(_ hire: FrontDesk.Hire, force: Bool, capture: String?, git: Git = HireDesk.git) -> Refusal? {
        if hire.state == .fired || hire.state == .failed {
            return Refusal(status: 409, message: "\(called(hire)) is already gone.")
        }
        if !hire.worktree.isEmpty, !force {
            guard let files = uncommittedFiles(hire.worktree, git: git) else {
                return Refusal(status: 409, message: "Couldn't tell whether \(called(hire))'s worktree has unsaved work. Fire again with force to go ahead.")
            }
            if !files.isEmpty {
                let shown = files.prefix(10).joined(separator: ", ") + (files.count > 10 ? ", and \(files.count - 10) more" : "")
                return Refusal(status: 409, message: "\(called(hire)) has changes that aren't committed: \(shown). Commit them, or fire with force.")
            }
        }
        if let capture, let asking = promptRefusal(capture) { return Refusal(status: 409, message: asking) }
        // A hire at work is never ended silently: what it is doing would be lost.
        if let capture, !force, inputState(capture) == .busy {
            return Refusal(status: 409, message: "\(called(hire)) is working right now. Wait until it is done, or fire with force.")
        }
        // Ending the session loses what is typed in its box, and a box that can't be told apart may hold anything.
        if let capture, !force {
            switch inputState(capture) {
            case .draft:
                return Refusal(status: 409, message: "\(called(hire)) has something typed in its window that isn't sent yet. Send or clear it, or fire with force.")
            case .unknown:
                return Refusal(status: 409, message: "Couldn't tell what \(called(hire))'s window holds, so nothing was ended. Look at it, or fire with force.")
            case .idleAndEmpty, .busy:
                break
            }
        }
        return nil
    }

    /// Hires being fired right now, so two fires of one hire at once can't both go ahead.
    private static let firing = NSLock()
    nonisolated(unsafe) private static var firingNow = Set<String>()

    private static func startFiring(_ id: String) -> Bool {
        firing.lock(); defer { firing.unlock() }
        return firingNow.insert(id).inserted
    }

    private static func endFiring(_ id: String) {
        firing.lock(); defer { firing.unlock() }
        firingNow.remove(id)
    }

    // MARK: Firing

    struct Fired: Equatable {
        /// The recap's file, "" if it couldn't be written.
        var recap = ""
        /// The worktree was left where it is, with why.
        var kept = ""
    }

    /// Fires `hire`. `tmux` is its own server's, when it runs on one. Everything that waits or runs git
    /// happens here and may be called off the main thread; the registry is only touched on the main one.
    @discardableResult
    static func fire(_ hire: FrontDesk.Hire, options: FireOptions = .init(), tmux: HireTmux?, desk: FrontDesk, recaps: URL = Paths.recaps,
                     git: Git = HireDesk.git, now: () -> Date = Date.init, pause: (TimeInterval) -> Void = { Thread.sleep(forTimeInterval: $0) }) -> Result<Fired, Refusal> {
        // One fire at a time per hire: the lock first, then the registry, which is the truth, not the copy the
        // caller holds: a fire that finished meanwhile (it marks the hire fired before it lets go) is not done again.
        guard startFiring(hire.id) else {
            return .failure(Refusal(status: 409, message: "\(called(hire)) is being fired already."))
        }
        defer { endFiring(hire.id) }
        var live: FrontDesk.State?
        onMain { live = desk.hires.first(where: { $0.id == hire.id })?.state }
        if live == .fired || live == .failed { return .failure(Refusal(status: 409, message: "\(called(hire)) is already gone.")) }
        let alive = tmux?.isAlive()
        if tmux != nil, alive == nil {
            return .failure(Refusal(status: 409, message: "Couldn't reach \(called(hire))'s tmux session, so nothing was changed."))
        }
        // What is in the window is read once, before anything is sent, and kept for the recap.
        let capture = alive == true ? tmux?.capture(lines: 200) : nil
        // A window that is there but can't be looked at is not typed into, nor ended: nobody knows what it shows.
        if alive == true, capture == nil { return .failure(Refusal(status: 409, message: promptRefusal(nil) ?? "")) }
        if let refusal = fireRefusal(hire, force: options.force, capture: capture, git: git) { return .failure(refusal) }

        if let tmux, alive == true {
            // Only an idle, empty box is typed into; at work (with force) or with a draft in it, the session is simply ended.
            if inputState(capture) == .idleAndEmpty {
                // Git may have taken a while: look again just before typing, so nothing is pasted over a question
                // that came up meanwhile. If the window can't be seen now, or asks, nothing is sent.
                let fresh = tmux.capture(lines: 200)
                if let fresh, promptRefusal(fresh) == nil, inputState(fresh) == .idleAndEmpty {
                    if tmux.exit() { waitForEnd(tmux, now: now, pause: pause) }
                } else {
                    return .failure(Refusal(status: 409, message: promptRefusal(fresh) ?? "Its window changed while it was being looked at, so nothing was sent. Try again."))
                }
            }
            if tmux.isAlive() != false { _ = tmux.kill() }
        }

        return .success(finish(hire, options: options, capture: capture, desk: desk, recaps: recaps, git: git, now: now))
    }

    private static func finish(_ hire: FrontDesk.Hire, options: FireOptions, capture: String?, desk: FrontDesk, recaps: URL,
                               git: Git, now: () -> Date) -> Fired {
        var fired = Fired()
        if !hire.worktree.isEmpty {
            if options.removeWorktree {
                if !removeWorkspace(repo: hire.repo.isEmpty ? hire.folder : hire.repo, worktree: hire.worktree, branch: hire.branch, git: git) {
                    fired.kept = "It holds changes nobody committed (new files count too), or git wouldn't remove it."
                }
            } else {
                fired.kept = "Kept, as asked."
            }
        }
        let removed = !hire.worktree.isEmpty && fired.kept.isEmpty
        fired.recap = writeRecap(hire, capture: capture, worktreeRemoved: removed, kept: fired.kept, in: recaps, now: now())
        onMain {
            desk.move(hire.id, to: .fired)
            if options.removeWorktree, !fired.kept.isEmpty, let now = desk.hires.first(where: { $0.id == hire.id }) {
                let note = "Its worktree \(hire.worktree) was kept: \(afterColon(fired.kept))"
                _ = try? desk.edit(["id": hire.id, "notes": String((now.notes.isEmpty ? note : note + "\n" + now.notes).prefix(FrontDesk.longestNotes))])
            }
        }
        return fired
    }

    /// A hire without a tmux session of its own, whose tab has been closed: the worktree, the recap and the registry,
    /// as `fire` does them for the rest.
    @discardableResult
    static func settle(_ hire: FrontDesk.Hire, options: FireOptions, desk: FrontDesk, recaps: URL = Paths.recaps,
                       git: Git = HireDesk.git, now: () -> Date = Date.init) -> Fired {
        finish(hire, options: options, capture: nil, desk: desk, recaps: recaps, git: git, now: now)
    }

    private static func waitForEnd(_ tmux: HireTmux, now: () -> Date, pause: (TimeInterval) -> Void) {
        let end = now().addingTimeInterval(exitWithin)
        while tmux.isAlive() == true, now() < end { pause(0.5) }
    }

    private static func onMain(_ work: () -> Void) {
        if Thread.isMainThread { work() } else { DispatchQueue.main.sync(execute: work) }
    }

    // MARK: Recap (C21)

    /// The recap's file name: the hire's id when it is plain, else made so.
    static func recapName(_ hire: FrontDesk.Hire) -> String {
        let plain = !hire.id.isEmpty && hire.id.count <= 64 && hire.id.allSatisfy { $0.isASCII && ($0.isLowercase || $0.isNumber || $0 == "-") }
        return (plain ? hire.id : slug(hire.id)) + ".md"
    }

    /// The last `count` lines with words in the STATUS file of `place`.
    static func lastStatus(in place: String, count: Int = 5) -> [String] {
        for path in ["STATUS.md", ".team/STATUS.md"] {
            let file = URL(fileURLWithPath: place).appendingPathComponent(path)
            guard (try? file.resolvingSymlinksInPath().resourceValues(forKeys: [.isRegularFileKey]))?.isRegularFile == true,
                  let handle = try? FileHandle(forReadingFrom: file) else { continue }
            defer { try? handle.close() }
            if let size = try? handle.seekToEnd(), size > 4096 { try? handle.seek(toOffset: size - 4096) } else { try? handle.seek(toOffset: 0) }
            guard let data = try? handle.read(upToCount: 4096) else { continue }
            return tail(String(decoding: data, as: UTF8.self), count).map { String($0.trimmingCharacters(in: .whitespaces).prefix(200)) }
        }
        return []
    }

    /// What the hire was asked to do: the first line with words in its BRIEF file, without its heading marks.
    static func task(in place: String) -> String {
        let names = ((try? FileManager.default.contentsOfDirectory(atPath: place)) ?? []).filter { $0.hasPrefix("BRIEF") && $0.hasSuffix(".md") }.sorted()
        for name in names {
            let file = URL(fileURLWithPath: place).appendingPathComponent(name)
            guard let handle = try? FileHandle(forReadingFrom: file), let data = try? handle.read(upToCount: 2048) else { continue }
            try? handle.close()
            if let line = String(decoding: data, as: UTF8.self).split(whereSeparator: \.isNewline).first(where: { !$0.trimmingCharacters(in: .whitespaces).isEmpty }) {
                return String(line.trimmingCharacters(in: CharacterSet(charactersIn: "# ")).prefix(200))
            }
        }
        return ""
    }

    /// Writes `<recaps>/<hire>.md`: who, task, branch, the last STATUS lines and the cost if the window showed one.
    /// Gives the file's path, "" if it couldn't be written.
    @discardableResult
    static func writeRecap(_ hire: FrontDesk.Hire, capture: String?, worktreeRemoved: Bool, kept: String, in folder: URL, now: Date,
                           hires: URL = Paths.hires) -> String {
        // A removed worktree is gone with its files, so the hire's own folder is looked in then.
        let place = hire.worktree.isEmpty || worktreeRemoved ? hire.folder : hire.worktree
        // A managed hire's brief and STATUS are kept outside its worktree, in its folder in Smolcorp's (T304); look there first.
        let own = hire.worktree.isEmpty ? place : BriefBuilder.hireFolder(hire.id, in: hires).path
        let status = [lastStatus(in: own), lastStatus(in: place)].first { !$0.isEmpty } ?? []
        let task = [task(in: own), task(in: place)].first { !$0.isEmpty } ?? ""
        let stamp = ISO8601DateFormatter()
        var lines = ["# Recap: \(hire.nickname.isEmpty ? hire.name : hire.nickname)", ""]
        lines.append("- Who: \(hire.name)\(hire.role.isEmpty ? "" : " (\(hire.role))")\(hire.nickname.isEmpty ? "" : ", called \(hire.nickname)")")
        lines.append("- Task: \(task.isEmpty ? "not written down" : task)")
        lines.append("- Folder: \(hire.folder)")
        if !hire.branch.isEmpty { lines.append("- Branch: \(hire.branch)") }
        if !hire.worktree.isEmpty {
            lines.append("- Worktree: \(hire.worktree) (\(worktreeRemoved ? "removed" : "kept"))")
        }
        if hire.started > 0 { lines.append("- Hired: \(stamp.string(from: Date(timeIntervalSince1970: hire.started / 1000)))") }
        lines.append("- Fired: \(stamp.string(from: now))")
        lines.append("- Cost: \(visibleCost(capture) ?? "not shown")")
        lines += ["", "## Last status", ""]
        lines += status.isEmpty ? ["Nothing was written to STATUS.md."] : status.map { "- \($0)" }
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        let file = folder.appendingPathComponent(recapName(hire))
        return Paths.writePrivately(Data((lines.joined(separator: "\n") + "\n").utf8), to: file) ? file.path : ""
    }

    // MARK: Which hire

    /// The open hire a session is: by the tmux socket its tab is on, else (for hires without a tmux server) by where it works.
    static func hire(ofTab tab: String, folder: String, project: String, among hires: [FrontDesk.Hire]) -> FrontDesk.Hire? {
        let open = hires.filter { $0.state != .fired && $0.state != .failed }
        if let socket = TabAddress(tab)?.socket, !socket.isEmpty, let hire = open.first(where: { $0.tmuxSocket == socket }) { return hire }
        // Only a hire without a tmux server of its own, and with a worktree of its own, is matched by where it works:
        // the user's own session in the same folder is not a hire.
        return open.first { $0.tmuxSocket.isEmpty && !$0.worktree.isEmpty
            && FrontDesk.works(FrontDesk.Live(id: "", agent: "", folder: folder, project: project, state: ""), for: $0) }
    }
}
