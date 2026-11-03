import Foundation

/// The office's record of what its sessions did: one JSON line per event in
/// events.jsonl, in the support folder, readable by you alone. The GM reads it
/// through /setup/gm/events instead of walking the windows, and it outlasts
/// the GM's restarts and the app's.
///
/// Only ten kinds of event are kept (see `Kind`; the eighth is a watcher's firing, the ninth a cloud bot's mail, the tenth keys pressed for you). Of each it keeps who,
/// where and when, with a short detail. The log stays on disk for a long time,
/// so the detail is thin: for a permission the tool, and for Bash only the
/// command's first word, for a file tool the file; for a notification its
/// message only when it's Claude Code's own wording; why a session started or
/// ended. Never the prompt, the agent's replies, a tool's output or a command's
/// arguments, headers or content.
///
/// Lines are numbered (`id`) in the order they're added, across restarts. The
/// file is written off the main thread. When it passes `maxBytes` it becomes
/// events.1.jsonl (the one before that is dropped) and a new one starts, so
/// the newest events are always kept. The latest lines are also held in
/// memory, so reading them doesn't touch the disk. Only used on the main
/// thread.
final class EventLog {
    enum Kind: String, Codable, CaseIterable {
        case sessionStart, sessionStop, prompt, permissionAsked, permissionDenied, notification, idle
        /// A watcher fired (see `Watchers`): what the GM reads in its inbox. Not a hook's.
        case watcherFired
        /// A cloud bot's reply, left in its mailbox folder (T404); never from a hook.
        case mail
        /// Keys sent to an app for you (`Keystrokes`), if you allowed it. Raised by the app itself, never by a hook.
        case keystroke

        /// The kind of Claude Code's hook event `name`, or nil for one that isn't kept.
        init?(hook name: String) {
            switch name {
            case "SessionStart": self = .sessionStart
            case "SessionEnd": self = .sessionStop
            case "UserPromptSubmit": self = .prompt
            case "PermissionRequest": self = .permissionAsked
            case "PermissionDenied": self = .permissionDenied
            case "Notification": self = .notification
            case "Stop": self = .idle
            default: return nil
            }
        }
    }

    struct Entry: Codable, Equatable {
        var id = 0
        /// Milliseconds since 1970, when the event was raised.
        var at: Double = 0
        var session = ""
        var folder = ""
        var agent = ""
        var kind = Kind.idle
        var detail = ""
    }

    static let detailLimit = 200
    /// How many of the latest lines are kept in memory.
    static let recentMost = 2000
    /// The most one answer holds.
    static let limitMost = 1000
    /// The longest a request may wait, in seconds.
    static let waitMost: TimeInterval = 30

    let folder: URL
    let maxBytes: Int
    var file: URL { folder.appendingPathComponent("events.jsonl") }
    var earlier: URL { folder.appendingPathComponent("events.1.jsonl") }

    private(set) var lastID = 0
    /// The first line still kept, or the next one when there's none.
    private(set) var oldestID = 1
    /// The first line of the live file (the next one when it's empty), and its size so far.
    /// Kept here so the rotation is decided on the main thread and `oldestID` follows it.
    private var liveFirst = 1
    private var liveBytes = 0
    private var recent: [Entry] = []
    /// Told of each line as it is added (the deck sends a push for a review).
    var onAppend: ((Entry) -> Void)?
    private let disk = DispatchQueue(label: "smolcorp.events")
    private var waiters: [UUID: (since: Int, limit: Int, done: ([Entry]) -> Void)] = [:]

    init(folder: URL, maxBytes: Int = 10 << 20) {
        self.folder = folder
        self.maxBytes = maxBytes
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let before = EventLog.read(earlier), live = EventLog.read(file)
        let kept = before + live
        recent = Array(kept.suffix(EventLog.recentMost))
        lastID = kept.last?.id ?? 0
        oldestID = kept.first?.id ?? lastID + 1
        liveFirst = live.first?.id ?? lastID + 1
        liveBytes = (try? FileManager.default.attributesOfItem(atPath: file.path)[.size] as? NSNumber)?.intValue ?? 0
    }

    // MARK: Writing

    /// Adds `entry` as the next line, and answers anyone waiting for it.
    @discardableResult
    func append(_ entry: Entry) -> Entry {
        var entry = entry
        lastID += 1
        entry.id = lastID
        entry.at = entry.at.rounded() // whole milliseconds, as the answer says
        entry.detail = EventLog.cut(entry.detail)
        recent.append(entry)
        if recent.count > EventLog.recentMost { recent.removeFirst(recent.count - EventLog.recentMost) }
        guard var line = try? EventLog.encoder.encode(entry) else { return entry }
        line.append(0x0A)
        // Past the cap the live file becomes the earlier one (the one before is dropped),
        // so what's kept starts where it did.
        liveBytes += line.count
        let rotate = liveBytes >= maxBytes
        if rotate {
            oldestID = liveFirst
            liveFirst = entry.id + 1
            liveBytes = 0
            // Memory holds no more than the files do.
            recent.removeAll { $0.id < oldestID }
        }
        let (file, earlier) = (self.file, self.earlier)
        disk.async {
            EventLog.write(line, to: file)
            if rotate {
                try? FileManager.default.removeItem(at: earlier)
                try? FileManager.default.moveItem(at: file, to: earlier)
            }
        }
        for (id, waiter) in waiters where waiter.since < entry.id {
            waiters[id] = nil
            waiter.done(entries(since: waiter.since, limit: waiter.limit).entries)
        }
        onAppend?(entry)
        return entry
    }

    /// Waits until everything added so far is on disk: for tests.
    func flush() { disk.sync {} }

    static let encoder: JSONEncoder = {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
        return encoder
    }()

    private static func write(_ line: Data, to file: URL) {
        if !FileManager.default.fileExists(atPath: file.path) {
            FileManager.default.createFile(atPath: file.path, contents: nil, attributes: [.posixPermissions: 0o600])
        }
        guard let handle = try? FileHandle(forWritingTo: file) else { return }
        defer { try? handle.close() }
        _ = try? handle.seekToEnd()
        try? handle.write(contentsOf: line)
    }

    // MARK: Reading

    /// The lines after `since` that memory holds, at most `limit` of them; the
    /// `since` to ask with next; and the first line still kept. A `since` past the
    /// last line (the log was reset) answers with none and `next` = the last line,
    /// to start again from. Lines older than memory come from `wait`, off the main thread.
    func entries(since: Int, limit: Int) -> (entries: [Entry], next: Int, oldest: Int) {
        guard since < lastID else { return ([], lastID, oldestID) }
        let found = Array(recent.drop { $0.id <= since }.prefix(max(1, limit)))
        return (found, found.last?.id ?? max(since, 0), oldestID)
    }

    /// Answers with the lines after `since` as soon as there are any, or with
    /// none after `timeout` seconds (at most `waitMost`). Nothing blocks: the
    /// answer is given when it's ready, from the main queue.
    func wait(since: Int, limit: Int, timeout: TimeInterval, done: @escaping ([Entry]) -> Void) {
        // Past the last line: nothing to wait for, `next` says where to start again.
        guard since <= lastID else { return done([]) }
        if since < lastID, let first = recent.first, since < first.id - 1 {
            return fromFiles(since: since, limit: max(1, limit), done: done)
        }
        let now = entries(since: since, limit: limit).entries
        guard now.isEmpty, timeout > 0 else { return done(now) }
        let id = UUID()
        waiters[id] = (since, limit, done)
        DispatchQueue.main.asyncAfter(deadline: .now() + min(timeout, EventLog.waitMost)) { [weak self] in
            guard let waiter = self?.waiters.removeValue(forKey: id) else { return }
            waiter.done([])
        }
    }

    /// Lines older than memory holds: read on the disk queue (after what's pending is
    /// written), answered on the main queue.
    private func fromFiles(since: Int, limit: Int, done: @escaping ([Entry]) -> Void) {
        let (file, earlier) = (self.file, self.earlier)
        disk.async {
            let found = Array((EventLog.read(earlier) + EventLog.read(file)).filter { $0.id > since }.prefix(limit))
            DispatchQueue.main.async { done(found) }
        }
    }

    private static let decoder = JSONDecoder()

    /// The lines of `file`, skipping any that aren't one.
    static func read(_ file: URL) -> [Entry] {
        guard let data = try? Data(contentsOf: file) else { return [] }
        return data.split(separator: 0x0A).compactMap { try? decoder.decode(Entry.self, from: Data($0)) }
    }

    // MARK: The route's words

    /// What /setup/gm/events was asked for: `since` (the last id seen),
    /// `wait` in seconds and `limit`, each kept to what's allowed.
    static func question(_ query: [String: String]) -> (since: Int, wait: TimeInterval, limit: Int) {
        (max(0, Int(query["since"] ?? "") ?? 0),
         min(waitMost, max(0, Double(query["wait"] ?? "") ?? 0)),
         min(limitMost, max(1, Int(query["limit"] ?? "") ?? 200)))
    }

    /// The answer to /setup/gm/events.
    func answer(_ entries: [Entry], since: Int) -> [String: Any] {
        let objects = entries.compactMap { try? JSONSerialization.jsonObject(with: EventLog.encoder.encode($0)) }
        return ["events": objects, "next": entries.last?.id ?? min(since, lastID), "oldest": oldestID]
    }

    // MARK: From a hook event

    /// The tools that work on a file, and the input field naming it. Their
    /// detail is the file; for any other tool, the tool alone.
    static let fileFields = ["file_path", "notebook_path"]

    /// The line for hook `event` of a session at `folder`, run by `agent`; nil
    /// when it's not one of the kinds kept.
    static func entry(for event: [String: Any], agent: String, folder: String, at: Double) -> Entry? {
        guard let kind = Kind(hook: event["hook_event_name"] as? String ?? "") else { return nil }
        return Entry(at: at, session: event["session_id"] as? String ?? "", folder: folder, agent: agent, kind: kind,
                     detail: detail(of: event, kind: kind))
    }

    /// What's worth knowing about `event` beyond its kind. The log is kept long, so
    /// never its prompt, reply, a tool's output or a call's arguments.
    static func detail(of event: [String: Any], kind: Kind) -> String {
        switch kind {
        case .permissionAsked, .permissionDenied:
            let tool = event["tool_name"] as? String ?? ""
            let input = event["tool_input"] as? [String: Any] ?? [:]
            let what: String
            if tool == "Bash" {
                what = firstWord(of: input["command"] as? String ?? "")
            } else {
                what = fileFields.lazy.compactMap { input[$0] as? String }.first { !$0.isEmpty } ?? ""
            }
            return cut(what.isEmpty ? tool : tool.isEmpty ? what : "\(tool): \(what)")
        case .notification:
            let message = event["message"] as? String ?? ""
            // A usage or rate limit is told by one fixed phrase, never the message (it could carry anything).
            if limitWording(message) { return limitDetail }
            return ownWording(message) ? cut(message) : "notification"
        case .sessionStart:
            return cut(event["source"] as? String ?? "")
        case .sessionStop:
            return cut(event["reason"] as? String ?? "")
        case .prompt, .idle, .watcherFired, .mail, .keystroke:
            return ""
        }
    }

    /// The program a shell command runs: its first word, past any `NAME=value`
    /// set before it (those can hold a key).
    static func firstWord(of command: String) -> String {
        let words = command.split(whereSeparator: \.isWhitespace)
        return String(words.first { !$0.contains("=") } ?? "")
    }

    /// Whether `message` is one of Claude Code's own notifications, which hold no
    /// more than a tool's name; anything else could say anything.
    static func ownWording(_ message: String) -> Bool {
        message == "Claude is waiting for your input"
            || message.range(of: #"^Claude needs your permission to use [A-Za-z0-9_.:\-]+$"#, options: .regularExpression) != nil
    }

    /// What a notification about a usage or rate limit is kept as.
    static let limitDetail = "usage limit"

    /// Whether `message` says the account hit a usage or rate limit.
    static func limitWording(_ message: String) -> Bool {
        message.range(of: #"(usage|rate|session|weekly|5-hour|daily) limit|hit your limit|limit reached"#,
                      options: [.regularExpression, .caseInsensitive]) != nil
    }

    /// On one line, at most `detailLimit` characters.
    static func cut(_ text: String) -> String {
        let line = text.split(whereSeparator: \.isNewline).joined(separator: " ")
        return line.count > detailLimit ? String(line.prefix(detailLimit - 1)) + "…" : line
    }
}
