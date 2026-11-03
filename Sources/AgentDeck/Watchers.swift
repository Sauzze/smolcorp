import Foundation

/// What the GM asks the app to keep an eye on (T405), so a watch outlasts the GM's
/// restarts and the app's: kept in watchers.json in the support folder, readable by
/// you alone. A test copy has its own support folder, so its watches, and the
/// events they raise, never reach the real GM.
///
/// The kinds:
/// - `file`: a new line matching a pattern in a file inside a folder the app
///   manages (a hire's folder or worktree, or the support `hires/` folder),
///   such as a hire's STATUS.md. Never a file outside those, never a link out of them;
/// - `event`: a line of that kind in the event log (session stopped, permission denied...);
/// - `idle`: a hire's folder has been quiet since its turn ended, for N minutes;
/// - `exit`: a hire was fired or failed, or is gone from the registry;
/// - `time`: N minutes pass, once, or every N minutes.
///
/// Only NEW lines fire. A watch notes how far it has read (and which file, by its
/// inode); lines already there when it is set never fire, and a file that was
/// rotated or got shorter is taken again from its end, so old lines don't come back.
/// A file that did not exist yet is read from its first line when it appears.
///
/// A firing is a line in the event log (kind `watcherFired`): the GM's inbox
/// (`smolcorp gm inbox`) reads it, so with no GM at work it waits there. When a
/// way to wake the GM is given (`wake`, one short line, put through the same
/// gate as all mail: see `MailRouter.gate`), it is called too; it never types
/// into a window by itself.
///
/// The app's own watches (T406, `sync`): for each hire the manager has at work, its STATUS.md lines
/// `DONE`, `STOP`, `BLOCKED`, `BUDGET REQUEST` and `NEEDS-REVIEW:`, a usage-limit notice, a refused
/// call, and a permission asked and still waiting after 2 minutes; and, while a GM is at work, an
/// hourly digest line. They are set when the hire starts and taken away when it is fired. Each
/// fires once for each new line or event. A NEEDS-REVIEW firing is marked (`reviewMark`) for the
/// owner's phone and the deck. One setting (`Config.builtinWatches`) switches them all off.
///
/// State is only touched on the main thread; files are read on `queue`.
final class Watchers {
    enum Kind: String, Codable, CaseIterable {
        case file, event, idle, exit, time
    }

    struct Watch: Codable, Equatable {
        var id: String
        var kind: Kind
        /// `file`: the full path. `event`: the event kind. `idle`, `exit`: the hire (its id for `exit`, its folder for `idle`).
        /// `time`: a few words the GM wants back.
        var target = ""
        /// A regular expression: matched against each new line, or an event's detail.
        var pattern = ""
        /// `event`: only for events in this folder or under it.
        var folder = ""
        /// `idle`, `time`: how many minutes.
        var minutes: Double = 0
        var repeats = false
        /// "gm" or "owner".
        var by = ""
        var createdAt: Double = 0
        var firedAt: Double = 0
        var fired = 0
        /// `time`: when it is next due, in ms.
        var due: Double = 0
        /// `file`: how far it has read, and the file's inode (0 while it has not been there).
        var offset = 0
        var inode: UInt64 = 0
        /// `file`: a hash of the (up to 64) bytes just before `offset`, so a file rewritten in place is told from one only added to. 0: not known.
        var tail: UInt64 = 0
        /// `idle`: the event log line it last fired for, so one quiet spell fires once.
        var lastEvent = 0
        /// The app's own watch: what it is for (`needs-review`, `status`, `denied`, `limit`, `wait`, `digest`), else "".
        var builtin = ""
        /// A built-in's hire (its id).
        var hire = ""

        private enum CodingKeys: String, CodingKey {
            // "repeat" in and out, as POST takes it.
            case repeats = "repeat"
            case id, kind, target, pattern, folder, minutes, by, createdAt, firedAt, fired, due, offset, inode, tail, lastEvent, builtin, hire
        }

        init(id: String, kind: Kind) {
            self.id = id
            self.kind = kind
        }

        /// Field by field, so a file kept before a field was added still reads.
        init(from decoder: Decoder) throws {
            let c = try decoder.container(keyedBy: CodingKeys.self)
            id = try c.decode(String.self, forKey: .id)
            kind = try c.decode(Kind.self, forKey: .kind)
            func text(_ key: CodingKeys) -> String { ((try? c.decodeIfPresent(String.self, forKey: key)) ?? nil) ?? "" }
            func number(_ key: CodingKeys) -> Double { ((try? c.decodeIfPresent(Double.self, forKey: key)) ?? nil) ?? 0 }
            (target, pattern, folder, by) = (text(.target), text(.pattern), text(.folder), text(.by))
            (builtin, hire) = (text(.builtin), text(.hire))
            (minutes, createdAt, firedAt, due) = (number(.minutes), number(.createdAt), number(.firedAt), number(.due))
            fired = ((try? c.decodeIfPresent(Int.self, forKey: .fired)) ?? nil) ?? 0
            offset = ((try? c.decodeIfPresent(Int.self, forKey: .offset)) ?? nil) ?? 0
            lastEvent = ((try? c.decodeIfPresent(Int.self, forKey: .lastEvent)) ?? nil) ?? 0
            inode = ((try? c.decodeIfPresent(UInt64.self, forKey: .inode)) ?? nil) ?? 0
            tail = ((try? c.decodeIfPresent(UInt64.self, forKey: .tail)) ?? nil) ?? 0
            repeats = ((try? c.decodeIfPresent(Bool.self, forKey: .repeats)) ?? nil) ?? false
        }

        /// Whether it still watches: fired once, it is done (and kept, fired, till taken away) unless it repeats.
        var active: Bool { repeats || firedAt == 0 }
    }

    /// What a check says happened, before it is written down.
    struct Fire: Equatable {
        var id: String
        var folder = ""
        var detail: String
        /// `reviewMark` for a NEEDS-REVIEW line, else "".
        var mark = ""
        /// A hire's DONE line: whose, for the digest.
        var done = ""
        /// The digest: how many finished hires it told of.
        var told = 0
    }

    /// How a NEEDS-REVIEW firing is marked in the event log (its `agent`), so the deck and the phone can pick it up.
    static let reviewMark = "needs-review"
    /// A built-in's patterns: a line that begins with the word (a list dash may come first).
    static let reviewPattern = #"^\s*(?:[-*]\s+)?NEEDS-REVIEW:"#
    static let statusPattern = #"^\s*(?:[-*]\s+)?(?:DONE|STOP|BLOCKED|BUDGET REQUEST)\b"#
    static let digestMinutes = 60.0
    static let waitMinutes = 2.0
    /// The most firings one built-in file watch makes at a look.
    static let firesMost = 10

    /// Where a file watch stands after a look: the lines that are new.
    struct Look: Equatable {
        var offset: Int
        var inode: UInt64
        var tail: UInt64
        var lines: [String]
    }

    static let queue = DispatchQueue(label: "io.github.sauzze.agentdeck.watchers")
    /// How often files and timers are looked at.
    static let interval: TimeInterval = 5
    /// The most one look reads of a file, and the longest line looked at.
    static let readMost = 1 << 20
    static let lineMost = 4000
    static let textMost = 500
    static let watchesMost = 50
    static let fileVersion = 1

    let file: URL
    private(set) var all: [Watch]
    private let roots: () -> [String]
    private let hires: () -> [FrontDesk.Hire]
    private let log: (EventLog.Entry) -> Void
    private let wake: ((String) -> Void)?
    private let now: () -> Date
    private let newID: () -> String
    /// Whether the built-in watches are wanted (the manager is on, and the setting).
    private let builtinsOn: () -> Bool
    /// The folders of sessions waiting at a question, now.
    private let asking: () -> [String]
    /// When each of those folders was first seen waiting, in ms.
    private var askingSince: [String: Double] = [:]
    /// Hires seen finishing (a DONE line) since the last digest.
    private(set) var doneSince: [String] = []
    /// The latest event of each folder, from the event log, for `idle`.
    private var latest: [String: EventLog.Entry] = [:]
    /// The `idle` event that began each folder's quiet spell, for `idle`. It stays until real work starts
    /// again (see `endsIdle`): a notification Claude Code raises while it waits doesn't end it.
    private var idleSince: [String: EventLog.Entry] = [:]
    private var checking = false
    private var timer: Timer?

    /// `roots` are the folders a file watch may look inside; `hires` the registry;
    /// `log` adds a line to the event log (the GM's inbox); `wake` is the optional
    /// short line to the GM, given when the paste paths exist; `seed` the event
    /// log's latest lines, so `idle` knows where folders stood.
    init(folder: URL, roots: @escaping () -> [String], hires: @escaping () -> [FrontDesk.Hire],
         log: @escaping (EventLog.Entry) -> Void, wake: ((String) -> Void)? = nil, seed: [EventLog.Entry] = [],
         now: @escaping () -> Date = Date.init, newID: @escaping () -> String = { "w" + UUID().uuidString.prefix(8).lowercased() },
         builtins: @escaping () -> Bool = { Config.shared.managerEnabled && Config.shared.builtinWatches },
         asking: @escaping () -> [String] = { [] }) {
        builtinsOn = builtins
        self.asking = asking
        file = folder.appendingPathComponent("watchers.json")
        self.roots = roots
        self.hires = hires
        self.log = log
        self.wake = wake
        self.now = now
        self.newID = newID
        all = Watchers.load(file)
        seed.forEach { note($0) }
    }

    /// Looks every `interval`, from now on, while the manager is on.
    func start() {
        guard timer == nil else { return }
        timer = Timer.scheduledTimer(withTimeInterval: Watchers.interval, repeats: true) { [weak self] _ in
            guard Config.shared.managerEnabled else { return self?.sync() ?? () }
            self?.tick()
        }
    }

    // MARK: Setting and taking away

    /// `POST /setup/gm/watch`. `by` is "gm" or "owner".
    func add(_ json: [String: Any], by: String) throws -> Watch {
        func text(_ key: String) -> String { (json[key] as? String ?? "").oneLine(Watchers.textMost) }
        guard let kind = Kind(rawValue: text("kind").lowercased()) else {
            throw FrontDesk.Refusal(status: 400, message: "Say a kind: " + Kind.allCases.map(\.rawValue).joined(separator: ", ") + ".")
        }
        guard all.filter({ $0.builtin.isEmpty }).count < Watchers.watchesMost else {
            throw FrontDesk.Refusal(status: 409, message: "That is \(Watchers.watchesMost) watches, the most. Take one away first.")
        }
        let ms = (now().timeIntervalSince1970 * 1000).rounded()
        var watch = Watch(id: newID(), kind: kind)
        (watch.by, watch.createdAt, watch.pattern, watch.target) = (by, ms, text("pattern"), text("target"))
        watch.repeats = json["repeat"] as? Bool ?? false
        if !watch.pattern.isEmpty, Watchers.regex(watch.pattern) == nil {
            throw FrontDesk.Refusal(status: 400, message: "That pattern isn't a regular expression.")
        }
        let minutes = (json["minutes"] as? NSNumber)?.doubleValue ?? 0

        switch kind {
        case .file:
            guard !watch.pattern.isEmpty else { throw FrontDesk.Refusal(status: 400, message: "Say the pattern to look for.") }
            var path = (watch.target as NSString).expandingTildeInPath
            if !path.hasPrefix("/"), !text("hire").isEmpty {
                // A short path is inside the hire's folder.
                path = try (folder(of: try find(text("hire"))) as NSString).appendingPathComponent(path)
            }
            guard path.hasPrefix("/") else {
                throw FrontDesk.Refusal(status: 400, message: "Give the file as a full path, or a path inside a hire's folder with \"hire\".")
            }
            guard let inside = allowed(path) else {
                throw FrontDesk.Refusal(status: 403, message: "Only files inside a hire's folder or the hires folder can be watched.")
            }
            watch.target = inside
            // What is there now never fires: start from its end.
            if let state = Watchers.fileState(inside) { (watch.offset, watch.inode, watch.tail) = (state.size, state.inode, state.tail) }
        case .event:
            guard let seen = EventLog.Kind(rawValue: watch.target), seen != .watcherFired else {
                throw FrontDesk.Refusal(status: 400, message: "Say which event: "
                    + EventLog.Kind.allCases.filter { $0 != .watcherFired }.map(\.rawValue).joined(separator: ", ") + ".")
            }
            watch.folder = (text("folder") as NSString).standardizingPath
            if text("folder").isEmpty { watch.folder = "" }
        case .idle:
            guard minutes > 0 else { throw FrontDesk.Refusal(status: 400, message: "Say how many \"minutes\" quiet.") }
            watch.minutes = minutes
            watch.target = try folder(of: try find(watch.target))
            // A quiet spell already running was not set by this watch.
            watch.lastEvent = latest[watch.target]?.id ?? 0
        case .exit:
            let hire = try find(watch.target)
            if hire.state == .fired || hire.state == .failed { throw FrontDesk.Refusal(status: 409, message: "\(hire.name) is already gone.") }
            watch.target = hire.id
            watch.repeats = false // a hire leaves once: a repeating exit would fire at every look
        case .time:
            guard minutes > 0 else { throw FrontDesk.Refusal(status: 400, message: "Say how many \"minutes\" from now.") }
            guard !watch.repeats || minutes >= 1 else { throw FrontDesk.Refusal(status: 400, message: "Repeating means every minute at the soonest.") }
            watch.minutes = minutes
            watch.due = ms + (minutes * 60_000).rounded()
        }
        all.append(watch)
        save()
        return watch
    }

    /// `POST /setup/gm/unwatch`, `DELETE /setup/gm/unwatch?id=`.
    func remove(_ id: String) throws {
        guard let i = all.firstIndex(where: { $0.id == id }) else { throw FrontDesk.Refusal(status: 404, message: "No watch \(id).") }
        guard all[i].builtin.isEmpty else {
            throw FrontDesk.Refusal(status: 403, message: "That watch is the app's own. Switch the built-in watches off in settings to stop them.")
        }
        all.remove(at: i)
        save()
    }

    /// `GET /setup/gm/watches`: as set, without where it has read to.
    var json: [[String: Any]] {
        all.map { watch in
            var out: [String: Any] = ["id": watch.id, "kind": watch.kind.rawValue, "target": watch.target, "pattern": watch.pattern,
                                      "repeat": watch.repeats, "by": watch.by, "createdAt": Int64(watch.createdAt.rounded(.down)), "fired": watch.fired,
                                      "active": watch.active]
            if watch.firedAt > 0 { out["firedAt"] = Int64(watch.firedAt.rounded(.down)) }
            if !watch.folder.isEmpty { out["folder"] = watch.folder }
            if watch.minutes > 0 { out["minutes"] = watch.minutes }
            if !watch.builtin.isEmpty { out["builtin"] = watch.builtin }
            if watch.kind == .time, watch.active { out["due"] = Int64(watch.due.rounded(.down)) }
            return out
        }
    }

    /// The hire a name means: its id, or its nickname or name (an open one first).
    private func find(_ name: String) throws -> FrontDesk.Hire {
        let asked = name.trimmingCharacters(in: .whitespaces).lowercased()
        guard !asked.isEmpty else { throw FrontDesk.Refusal(status: 400, message: "Say whom, as \"target\".") }
        let all = hires().filter { $0.role != "gm" }
        var found = all.filter { $0.id == name }
        if found.isEmpty {
            let named = all.filter { $0.nickname.lowercased() == asked || $0.name.lowercased() == asked }
            let open = named.filter { $0.state != .fired && $0.state != .failed }
            found = open.isEmpty ? named : open
        }
        guard let hire = found.first else { throw FrontDesk.Refusal(status: 404, message: "There's no hire called \(name).") }
        guard found.count == 1 else { throw FrontDesk.Refusal(status: 409, message: "More than one hire is called that. Use the id.") }
        return hire
    }

    private func folder(of hire: FrontDesk.Hire) throws -> String {
        let folder = hire.worktree.isEmpty ? hire.folder : hire.worktree
        guard !folder.isEmpty else { throw FrontDesk.Refusal(status: 409, message: "\(hire.name) has no folder.") }
        return folder
    }

    /// `path` made plain (links followed) if it is a file inside a folder the app manages, else nil.
    /// Looked at again at every read, so a link swapped in later doesn't lead out.
    func allowed(_ path: String) -> String? {
        Watchers.inside(path, roots: roots())
    }

    /// Folders that hold keys, settings or history: never watched, wherever they sit.
    static let privateFolders: Set<String> = [".ssh", ".gnupg", ".aws", ".git", ".config"]

    /// `home` is the user's home folder: a root that is it, or above it, is too broad to be a hire's folder.
    static func inside(_ path: String, roots: [String], home: String = NSHomeDirectory()) -> String? {
        func plain(_ path: String) -> String { URL(fileURLWithPath: path).resolvingSymlinksInPath().standardized.path }
        let resolved = plain(path)
        // Never a secrets file or a keys folder, whatever folder it is in.
        for part in resolved.split(separator: "/") where part.hasPrefix(".env") || privateFolders.contains(String(part)) { return nil }
        let house = plain(home)
        for root in roots where !root.isEmpty {
            let base = plain(root)
            // A hire registered at ~ or above it would open every client's files.
            if base == "/" || base == house || house.hasPrefix(base + "/") { continue }
            if resolved == base || resolved.hasPrefix(base + "/") { return resolved }
        }
        return nil
    }

    // MARK: The file

    private func save() {
        struct File: Codable { var version: Int; var watches: [Watch] }
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
        guard let data = try? encoder.encode(File(version: Watchers.fileVersion, watches: all)) else { return }
        Paths.writePrivately(data, to: file)
    }

    private static func load(_ file: URL) -> [Watch] {
        struct File: Codable { var version: Int?; var watches: [Watch] }
        guard let data = try? Data(contentsOf: file) else { return [] }
        guard let found = try? JSONDecoder().decode(File.self, from: data) else {
            // Set aside, as the registry's is, so the next save doesn't wipe what might be mended.
            try? FileManager.default.moveItem(at: file, to: file.deletingLastPathComponent().appendingPathComponent("watchers.broken.json"))
            return []
        }
        return found.watches
    }

    // MARK: Events

    private func note(_ entry: EventLog.Entry) {
        guard entry.kind != .watcherFired, !entry.folder.isEmpty else { return }
        if let before = latest[entry.folder], before.id > entry.id { return }
        latest[entry.folder] = entry
        if entry.kind == .idle {
            idleSince[entry.folder] = entry
        } else if Watchers.endsIdle(entry.kind) {
            idleSince[entry.folder] = nil
        }
    }

    /// Whether a hire doing this has stopped being idle: it was given a prompt, a session began or ended,
    /// or it asked for (or was refused) a permission, which only happens while it works. A notification
    /// ("Claude is waiting for your input") comes about a minute after it went idle, and is not work.
    /// Claude Code's tool calls aren't kept as events: they are always preceded by a prompt.
    static func endsIdle(_ kind: EventLog.Kind) -> Bool {
        switch kind {
        case .prompt, .sessionStart, .sessionStop, .permissionAsked, .permissionDenied: return true
        default: return false
        }
    }

    /// A line just added to the event log, for the watches on events.
    func observe(_ entry: EventLog.Entry) {
        guard entry.kind != .watcherFired else { return }
        note(entry)
        var fires: [Fire] = []
        for watch in all where watch.active && watch.kind == .event && watch.target == entry.kind.rawValue {
            if !watch.folder.isEmpty, entry.folder != watch.folder, !entry.folder.hasPrefix(watch.folder + "/") { continue }
            if !watch.pattern.isEmpty, !Watchers.matches(watch.pattern, entry.detail) { continue }
            let name = (entry.folder as NSString).lastPathComponent
            fires.append(Fire(id: watch.id, folder: entry.folder,
                              detail: "\(entry.kind.rawValue)\(name.isEmpty ? "" : " in " + name)\(entry.detail.isEmpty ? "" : ": " + entry.detail)"))
        }
        apply(fires, updated: [])
    }

    // MARK: Looking

    /// Looks at files, timers, quiet folders and hires, files off the main thread, and fires what is due.
    /// `inline` looks on the calling thread (for tests); `done` is called on the main queue after.
    func tick(inline: Bool = false, done: @escaping () -> Void = {}) {
        guard !checking else { return done() }
        checking = true
        sync()
        let (roots, states, ms, latest, spells) = (self.roots(), hires(), now().timeIntervalSince1970 * 1000, self.latest, idleSince)
        // When each folder was first seen waiting; one that stopped waiting is forgotten.
        let waiting = Set(asking())
        askingSince = askingSince.filter { waiting.contains($0.key) }
        for folder in waiting where askingSince[folder] == nil { askingSince[folder] = ms }
        let (since, done0) = (askingSince, doneSince)
        let watches = all.filter(\.active)
        let work = {
            let (updated, fires) = Watchers.check(watches, roots: roots, hires: states, latest: latest, idleSince: spells, now: ms, waiting: since, done: done0)
            let finish = { [self] in
                checking = false
                apply(fires, updated: updated)
                done()
            }
            if inline { finish() } else { DispatchQueue.main.async(execute: finish) }
        }
        if inline { work() } else { Watchers.queue.async(execute: work) }
    }

    /// Each watch as a look left it (where it has read to, its next time), and what fired.
    static func check(_ watches: [Watch], roots: [String], hires: [FrontDesk.Hire], latest: [String: EventLog.Entry],
                      idleSince: [String: EventLog.Entry] = [:], now: Double, waiting: [String: Double] = [:], done: [String] = []) -> (watches: [Watch], fires: [Fire]) {
        var updated: [Watch] = [], fires: [Fire] = []
        for var watch in watches {
            let before = watch
            switch watch.kind {
            case .file:
                // The path is checked again each time: a link swapped in since doesn't lead out.
                guard let path = inside(watch.target, roots: roots) else { break }
                let look = Watchers.look(path, offset: watch.offset, inode: watch.inode, tail: watch.tail)
                (watch.offset, watch.inode, watch.tail) = (look.offset, look.inode, look.tail)
                let hits = look.lines.filter { matches(watch.pattern, $0) }
                if !watch.builtin.isEmpty {
                    // The app's own: one firing for each new line, so none is lost in a burst.
                    let name = hireName(of: watch.hire, in: hires)
                    for line in hits.prefix(firesMost) {
                        fires.append(Fire(id: watch.id, folder: (path as NSString).deletingLastPathComponent,
                                          detail: "\(name): \((path as NSString).lastPathComponent): \(EventLog.cut(line))",
                                          mark: watch.builtin == "needs-review" ? reviewMark : "",
                                          done: Watchers.matches(#"^\s*(?:[-*]\s+)?DONE\b"#, line) ? name : ""))
                    }
                } else if let first = hits.first {
                    let extra = hits.count > 1 ? " (+\(hits.count - 1) more)" : ""
                    fires.append(Fire(id: watch.id, folder: (path as NSString).deletingLastPathComponent,
                                      detail: "\((path as NSString).lastPathComponent): \(EventLog.cut(first))\(extra)"))
                }
            case .time:
                if watch.due > 0, now >= watch.due {
                    if watch.builtin == "digest" {
                        fires.append(Fire(id: watch.id, detail: digest(hires: hires, latest: latest, waiting: waiting, done: done), told: done.count))
                    } else {
                        fires.append(Fire(id: watch.id, detail: watch.target.isEmpty ? "the time came" : watch.target))
                    }
                    // Next one counted from now, so a long sleep doesn't fire in a burst.
                    if watch.repeats { watch.due = now + (watch.minutes * 60_000).rounded() }
                }
            case .idle where watch.builtin == "wait":
                // A question asked and still waiting: once for each wait.
                let name = hireName(of: watch.hire, in: hires)
                let started = waiting.filter { $0.key == watch.target || $0.key.hasPrefix(watch.target + "/") }.values.min()
                if let started, Int(started) > watch.lastEvent, now - started >= watch.minutes * 60_000 {
                    watch.lastEvent = Int(started)
                    fires.append(Fire(id: watch.id, folder: watch.target, detail: "\(name) has waited \(Int(watch.minutes)) min at a question"))
                }
            case .idle:
                // The hire is gone: nothing will ever go idle again, so the watch is over, as a finished one is.
                if !hires.contains(where: { $0.state != .fired && $0.state != .failed && ($0.worktree.isEmpty ? $0.folder : $0.worktree) == watch.target }) {
                    (watch.repeats, watch.firedAt) = (false, now)
                    break
                }
                if let event = quietest(under: watch.target, in: idleSince), event.id > watch.lastEvent,
                   now - event.at >= watch.minutes * 60_000 {
                    watch.lastEvent = event.id
                    fires.append(Fire(id: watch.id, folder: watch.target,
                                      detail: "\((watch.target as NSString).lastPathComponent) has been idle for \(Int(watch.minutes)) min"))
                }
            case .exit:
                let hire = hires.first { $0.id == watch.target }
                if hire == nil || hire?.state == .fired || hire?.state == .failed {
                    let name = hire.map { $0.nickname.isEmpty ? $0.name : $0.nickname } ?? watch.target
                    fires.append(Fire(id: watch.id, folder: hire?.folder ?? "",
                                      detail: "\(name) is gone (\(hire?.state.rawValue ?? "not in the registry"))"))
                }
            case .event:
                break
            }
            if watch != before { updated.append(watch) }
        }
        return (updated, fires)
    }

    /// What a hire is called in a firing: its nickname, else its name, else its id.
    static func hireName(of id: String, in hires: [FrontDesk.Hire]) -> String {
        guard let hire = hires.first(where: { $0.id == id }) else { return id }
        return hire.nickname.isEmpty ? hire.name : hire.nickname
    }

    /// One line for the GM: who is working, idle, waiting, and who finished since the last digest.
    static func digest(hires: [FrontDesk.Hire], latest: [String: EventLog.Entry], waiting: [String: Double], done: [String]) -> String {
        var groups: [String: [String]] = ["working": [], "idle": [], "waiting": []]
        for hire in hires where hire.role != "gm" && hire.state != .fired && hire.state != .failed {
            let place = hire.worktree.isEmpty ? hire.folder : hire.worktree
            let who = hire.nickname.isEmpty ? hire.name : hire.nickname
            if waiting.keys.contains(where: { $0 == place || $0.hasPrefix(place + "/") }) {
                groups["waiting"]!.append(who)
            } else if hire.state == .idle || quietest(under: place, in: latest)?.kind == .idle {
                groups["idle"]!.append(who)
            } else {
                groups["working"]!.append(who)
            }
        }
        func list(_ names: [String]) -> String {
            names.isEmpty ? "none" : names.prefix(6).joined(separator: ", ") + (names.count > 6 ? " +\(names.count - 6)" : "")
        }
        return "Hourly digest. Working: \(list(groups["working"]!)). Idle: \(list(groups["idle"]!)). Waiting: \(list(groups["waiting"]!)). Done since last: \(list(done))."
    }

    /// The latest event among the folders at or under `folder`.
    static func quietest(under folder: String, in latest: [String: EventLog.Entry]) -> EventLog.Entry? {
        latest.filter { $0.key == folder || $0.key.hasPrefix(folder + "/") }.values.max { $0.id < $1.id }
    }

    /// A file's size, inode and tail hash at its end, if it is a plain file.
    static func fileState(_ path: String) -> (size: Int, inode: UInt64, tail: UInt64)? {
        let fd = open(path, O_RDONLY | O_NONBLOCK | O_NOFOLLOW)
        guard fd >= 0 else { return nil }
        defer { close(fd) }
        var info = stat()
        guard fstat(fd, &info) == 0, (info.st_mode & S_IFMT) == S_IFREG else { return nil }
        return (Int(info.st_size), UInt64(info.st_ino), tailHash(fd, before: Int(info.st_size)))
    }

    static let tailBytes = 64

    /// A hash (FNV-1a) of the up to `tailBytes` bytes just before `offset`; 0 when there are none or they can't be read.
    static func tailHash(_ fd: Int32, before offset: Int) -> UInt64 {
        let count = min(tailBytes, offset)
        guard count > 0 else { return 0 }
        var buffer = [UInt8](repeating: 0, count: count)
        guard pread(fd, &buffer, count, off_t(offset - count)) == count else { return 0 }
        var hash: UInt64 = 0xcbf29ce484222325
        for byte in buffer { hash = (hash ^ UInt64(byte)) &* 0x100000001b3 }
        return hash == 0 ? 1 : hash
    }

    /// The new complete lines of `path` since `offset` of the file `inode`, and where the next look starts.
    /// A different file, or a shorter one, is taken again from its end: its old lines never fire.
    /// A file that was not there (`inode` 0) is read from its first line when it comes.
    /// A file rewritten in place (same inode, and as long or longer) is told by `tail`, the hash of the bytes
    /// before `offset` when it was last read: if they changed, it is taken again from its end.
    /// A last line without its newline waits for it.
    static func look(_ path: String, offset: Int, inode: UInt64, tail: UInt64 = 0) -> Look {
        // O_NONBLOCK so a pipe put in its place can't hold the look; only plain files are read.
        let fd = open(path, O_RDONLY | O_NONBLOCK | O_NOFOLLOW)
        guard fd >= 0 else { return Look(offset: offset, inode: inode, tail: tail, lines: []) }
        defer { close(fd) }
        var info = stat()
        guard fstat(fd, &info) == 0, (info.st_mode & S_IFMT) == S_IFREG else { return Look(offset: offset, inode: inode, tail: tail, lines: []) }
        let (size, now) = (Int(info.st_size), UInt64(info.st_ino))
        var start = offset
        if inode == 0 {
            start = 0
        } else if now != inode || size < offset || (tail != 0 && tailHash(fd, before: offset) != tail) {
            return Look(offset: size, inode: now, tail: tailHash(fd, before: size), lines: [])
        }
        guard size > start else { return Look(offset: start, inode: now, tail: tail, lines: []) }
        let count = min(size - start, readMost)
        var buffer = [UInt8](repeating: 0, count: count)
        let got = pread(fd, &buffer, count, off_t(start))
        guard got > 0 else { return Look(offset: start, inode: now, tail: tail, lines: []) }
        buffer.removeLast(count - got)
        guard let last = buffer.lastIndex(of: 0x0A) else {
            // No whole line yet; a line longer than a look reads is let go.
            let next = got >= readMost ? start + got : start
            return Look(offset: next, inode: now, tail: next == start ? tail : tailHash(fd, before: next), lines: [])
        }
        let lines = buffer[...last].split(separator: 0x0A, omittingEmptySubsequences: true).map {
            String(decoding: $0.prefix(lineMost), as: UTF8.self).trimmingCharacters(in: CharacterSet(charactersIn: "\r"))
        }
        return Look(offset: start + last + 1, inode: now, tail: tailHash(fd, before: start + last + 1), lines: lines.filter { !$0.isEmpty })
    }

    // MARK: Firing

    /// Keeps what each watch now stands at, then, for each that fired, a line in the
    /// event log (the GM's inbox), and the wake line if there is a way to send one.
    func apply(_ fires: [Fire], updated: [Watch]) {
        guard !fires.isEmpty || !updated.isEmpty else { return }
        for watch in updated {
            // Taken away meanwhile, it stays away.
            if let i = all.firstIndex(where: { $0.id == watch.id }) { all[i] = watch }
        }
        let ms = (now().timeIntervalSince1970 * 1000).rounded()
        for fire in fires {
            guard let i = all.firstIndex(where: { $0.id == fire.id }), all[i].active else { continue }
            all[i].firedAt = ms
            all[i].fired += 1
            let detail = all[i].builtin.isEmpty ? "watch \(fire.id) (\(all[i].kind.rawValue)): \(fire.detail)" : "\(all[i].builtin): \(fire.detail)"
            if !fire.done.isEmpty { doneSince.append(fire.done) }
            if all[i].builtin == "digest" { doneSince.removeFirst(min(fire.told, doneSince.count)) }
            // The mark rides in `agent`, which a watcher's line has no other use for.
            log(EventLog.Entry(at: ms, session: "", folder: fire.folder, agent: fire.mark, kind: .watcherFired, detail: detail))
            wake?("Smolcorp: watch \(fire.id) fired. Read it with smolcorp gm inbox.")
        }
        save()
    }

    // MARK: The app's own watches

    /// Sets the built-in watches for the hires at work, and takes away those of hires that left,
    /// and all of them when they are switched off. What a new file watch finds there never fires.
    func sync() {
        let live = hires().filter { $0.state != .fired && $0.state != .failed }
        var wanted: [Watch] = []
        if builtinsOn() {
            func make(_ kind: Kind, _ builtin: String, _ hire: FrontDesk.Hire, target: String) -> Watch {
                var watch = Watch(id: "", kind: kind)
                (watch.builtin, watch.hire, watch.target, watch.repeats, watch.by) = (builtin, hire.id, target, true, "app")
                return watch
            }
            for hire in live where hire.role != "gm" {
                let place = hire.worktree.isEmpty ? hire.folder : hire.worktree
                guard !place.isEmpty else { continue }
                for name in ["STATUS.md", ".team/STATUS.md"] {
                    guard let path = allowed((place as NSString).appendingPathComponent(name)) else { continue }
                    var review = make(.file, "needs-review", hire, target: path)
                    review.pattern = Watchers.reviewPattern
                    var status = make(.file, "status", hire, target: path)
                    status.pattern = Watchers.statusPattern
                    wanted += [review, status]
                }
                var denied = make(.event, "denied", hire, target: EventLog.Kind.permissionDenied.rawValue)
                var limit = make(.event, "limit", hire, target: EventLog.Kind.notification.rawValue)
                var wait = make(.idle, "wait", hire, target: place)
                (denied.folder, limit.folder, limit.pattern, wait.minutes) = (place, place, "^" + EventLog.limitDetail + "$", Watchers.waitMinutes)
                wanted += [denied, limit, wait]
            }
            // The digest is the GM's, so it stops when the GM is fired.
            if let gm = live.first(where: { $0.role == "gm" }) {
                var digest = make(.time, "digest", gm, target: "")
                digest.minutes = Watchers.digestMinutes
                wanted.append(digest)
            }
        }
        func key(_ watch: Watch) -> String { [watch.builtin, watch.hire, watch.target].joined(separator: "|") }
        let wantedKeys = Set(wanted.map(key))
        let before = all
        all.removeAll { !$0.builtin.isEmpty && !wantedKeys.contains(key($0)) }
        let have = Set(all.filter { !$0.builtin.isEmpty }.map(key))
        let ms = (now().timeIntervalSince1970 * 1000).rounded()
        for var watch in wanted where !have.contains(key(watch)) {
            (watch.id, watch.createdAt) = (newID(), ms)
            if watch.kind == .file, let state = Watchers.fileState(watch.target) { (watch.offset, watch.inode, watch.tail) = (state.size, state.inode, state.tail) }
            if watch.kind == .time { watch.due = ms + (watch.minutes * 60_000).rounded() }
            all.append(watch)
        }
        if all != before { save() }
    }

    // MARK: Helpers

    static func regex(_ pattern: String) -> NSRegularExpression? {
        guard !pattern.isEmpty else { return nil }
        return try? NSRegularExpression(pattern: pattern)
    }

    static func matches(_ pattern: String, _ text: String) -> Bool {
        guard let regex = regex(pattern) else { return false }
        let line = String(text.prefix(lineMost))
        return regex.firstMatch(in: line, range: NSRange(line.startIndex..., in: line)) != nil
    }
}
