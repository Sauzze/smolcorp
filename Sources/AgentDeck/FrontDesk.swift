import CryptoKit
import Foundation

/// The front desk: what the app remembers of its team, so the General
/// Manager reads it from the app instead of walking windows. It keeps the
/// registry, one record for every hire in `registry.json` in the support
/// folder, and joins it with the sessions the deck follows into the team
/// that `GET /setup/gm/team` answers. Only used on the main thread.
final class FrontDesk {
    /// Where a hire is in its working life. The app moves it along as the
    /// hire starts and ends; the GM may correct it by hand (see `edit`).
    enum State: String, Codable, CaseIterable {
        case starting, working, idle, fired, failed
    }

    /// What the app knows of one hire. Where it works (folder, branch,
    /// worktree, tmux socket) is set when it is hired and never edited from
    /// outside; only `nickname`, `state` and `notes` can be.
    struct Hire: Codable, Equatable {
        var id: String
        /// What it is called by the role it was hired for, as the office shows it.
        var name: String
        var nickname = ""
        var role = ""
        /// The folder it was hired into.
        var folder: String
        var branch = ""
        /// Its own worktree, when it was given one.
        var worktree = ""
        /// The repo that worktree was made from (its top folder); "" with no worktree.
        var repo = ""
        /// The tmux socket its session runs on, when it runs in tmux.
        var tmuxSocket = ""
        /// The Claude Code account (profile) it was hired on; "" for the only one.
        var account = ""
        var model = ""
        /// The ports it was given, "4100-4103", also in its SMOLCORP_HIRE_PORTS.
        var ports = ""
        /// The full path of its BRIEF file, written when it was hired; "" for a hire from before that was kept.
        var brief = ""
        /// When it was hired: milliseconds since 1970, as the deck's other times.
        var started: Double = 0
        var state = State.starting
        var notes = ""
    }

    /// A session as the deck follows it, with where it sits in the office.
    struct Live: Equatable {
        var id: String
        var agent: String
        var folder: String
        var project: String
        var state: String
        var model = ""
        /// 0 for the ground floor, 1 for 2F up to 5 for 6F; nil with no desk.
        var floor: Int?
        /// Its desk on that floor, counted from 0; nil with none.
        var desk: Int?
        var lastEventAt: Double = 0
    }

    /// Bumped when the file's shape changes in a way an older app can't read.
    static let fileVersion = 1
    static let longestNickname = 40
    static let longestNotes = 2000

    /// One way of writing: `from` may send mail to `to` (both hires' ids). Two hires who may
    /// write to each other have one of these each way. The GM or the page sets them (T403).
    struct Contact: Codable, Equatable {
        var from: String
        var to: String
    }

    let registryFile: URL
    private(set) var hires: [Hire]
    /// Which hires may write to which. Without a pair here, a hire may write only to the GM.
    private(set) var contacts: [Contact]
    /// The hash of each hire's send key (hire id to hash), never the key.
    private var sendKeys: [String: String]

    /// Opens the registry kept in `folder`. A file that can't be read is set
    /// aside as `registry.broken.json`, so the next save doesn't wipe what
    /// might still be mended by hand.
    init(folder: URL) {
        registryFile = folder.appendingPathComponent("registry.json")
        let found = FrontDesk.load(registryFile)
        hires = found.hires
        contacts = found.contacts
        sendKeys = found.keys
    }

    // MARK: Registry

    /// Writes down a new hire, before its session has reported. Give its
    /// record whole; the app calls this once, as it hires. `sendKey` is the key
    /// the hire was started with (see `hire(forSendKey:)`); only its hash is kept.
    @discardableResult
    func register(_ hire: Hire, sendKey: String = "") -> Bool {
        guard !hire.id.isEmpty, !hires.contains(where: { $0.id == hire.id }) else { return false }
        hires.append(hire)
        if !sendKey.isEmpty { sendKeys[hire.id] = FrontDesk.hash(sendKey) }
        save()
        return true
    }

    // MARK: Writing to each other (T403)

    /// The header a hire sends its key in, and the variable its start script puts it in.
    static let sendKeyHeader = "x-smolcorp-hire-key"
    static let sendKeyEnvironment = "SMOLCORP_HIRE_KEY"

    static func hash(_ key: String) -> String {
        SHA256.hash(data: Data(key.utf8)).map { String(format: "%02x", $0) }.joined()
    }

    /// The hire a send key was made for, or nil: a key nobody was given, or a hire that is gone.
    /// The key is the hire's identity, so a hire can't say it is another; the GM's token never
    /// passes for one, nor a key for the GM's token.
    func hire(forSendKey key: String?) -> Hire? {
        guard let key, !key.isEmpty else { return nil }
        let given = FrontDesk.hash(key)
        var match: Hire?
        // Every hire is compared, so how long it takes doesn't say where a key was close.
        for hire in hires {
            guard let kept = sendKeys[hire.id], constantTimeEqual(kept, given) else { continue }
            if hire.state != .fired && hire.state != .failed { match = hire }
        }
        return match
    }

    /// Whether a hire may write to another: only a pair the GM or the page allowed.
    func mayWrite(from: String, to: String) -> Bool {
        contacts.contains(Contact(from: from, to: to))
    }

    /// The pairs, as `gm team` and `POST /setup/gm/contacts` answer them.
    var contactsJSON: [[String: Any]] { contacts.map { ["from": $0.from, "to": $0.to] } }

    /// `POST /setup/gm/contacts`, with `{from, to, oneWay?, remove?}`: `from` and `to` are
    /// hires' ids, nicknames or names. By default two hires may write to each other; with
    /// `oneWay`, only `from` to `to`. `remove` takes the pair away (with `oneWay`, only that
    /// direction). Answers the pairs as they are now.
    func setContacts(body: Data) -> Response {
        guard let json = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any],
              let from = json["from"] as? String, let to = json["to"] as? String,
              [json["oneWay"], json["remove"]].allSatisfy({ $0 == nil || $0 is Bool })
        else { return .status(400, "Send {from, to, oneWay?, remove?}: two hires, by id or name.") }
        let removing = json["remove"] as? Bool == true, oneWay = json["oneWay"] as? Bool == true
        let first: Hire, second: Hire
        switch (FrontDesk.find(from, among: hires), FrontDesk.find(to, among: hires)) {
        case (.failure(let refusal), _), (_, .failure(let refusal)): return .status(refusal.status, refusal.message)
        case (.success(let a), .success(let b)): (first, second) = (a, b)
        }
        guard first.id != second.id else { return .status(400, "That is one hire. Name two.") }
        guard first.role != "gm", second.role != "gm" else {
            return .status(400, "The GM isn't one of these pairs: a hire may always write to the GM, and the GM to any hire.")
        }
        if !removing {
            let gone = [first, second].filter { $0.state == .fired || $0.state == .failed }
            if let one = gone.first { return .status(409, "\(one.name) is already gone.") }
        }
        let pairs = oneWay ? [Contact(from: first.id, to: second.id)] : [Contact(from: first.id, to: second.id), Contact(from: second.id, to: first.id)]
        if removing {
            contacts.removeAll { pairs.contains($0) }
        } else {
            for pair in pairs where !contacts.contains(pair) { contacts.append(pair) }
        }
        save()
        return .json(["contacts": contactsJSON])
    }

    /// The hire a name means: its id, else its nickname or name (open hires first, so a
    /// fired hire's name can be used again). Says why not when there is none or two.
    static func find(_ name: String, among all: [Hire]) -> Result<Hire, Refusal> {
        let asked = name.trimmingCharacters(in: .whitespaces).lowercased()
        var found = all.filter { $0.id == name }
        if found.isEmpty {
            let named = all.filter { $0.nickname.lowercased() == asked || $0.name.lowercased() == asked }
            let open = named.filter { $0.state != .fired && $0.state != .failed }
            found = open.isEmpty ? named : open
        }
        guard let hire = found.first else { return .failure(Refusal(status: 404, message: "There's no hire called that. Look at the team for ids and names.")) }
        guard found.count == 1 else { return .failure(Refusal(status: 409, message: "More than one hire is called that. Use the id.")) }
        return .success(hire)
    }

    /// Moves a hire to `state` (the app's own changes: working, idle, failed
    /// or fired), saved at once. Says whether there was such a hire.
    @discardableResult
    func move(_ id: String, to state: State) -> Bool {
        guard let i = hires.firstIndex(where: { $0.id == id }) else { return false }
        guard hires[i].state != state else { return true }
        hires[i].state = state
        save()
        return true
    }

    /// A hire that didn't start: failed, with the reason put first in its notes.
    func fail(_ id: String, reason: String) {
        guard let i = hires.firstIndex(where: { $0.id == id }) else { return }
        hires[i].state = .failed
        hires[i].notes = hires[i].notes.isEmpty ? reason : reason + "\n" + hires[i].notes
        save()
    }

    /// The port ranges of hires still at work, for the next hire to keep clear of.
    var portsInUse: [String] { hires.filter { $0.state != .fired && $0.state != .failed }.map(\.ports).filter { !$0.isEmpty } }

    /// Why a General Manager can't be hired: one is still working (not fired, not failed). Fire it first.
    /// A GM past starting whose session is gone (its tmux died) is not working, so it doesn't block a new one.
    static func gmRefusal(among hires: [Hire], live: [Live]) -> Refusal? {
        let working = hires.contains { hire in
            hire.role == "gm" && hire.state != .fired && hire.state != .failed
                && (hire.state == .starting || live.contains { works($0, for: hire) })
        }
        guard working else { return nil }
        return Refusal(status: 409, message: "A General Manager is already working. Fire it first, then hire another.")
    }

    /// Why an edit was refused, with the status it answers.
    struct Refusal: Error {
        let status: Int
        let message: String
    }

    /// What can be edited by hand, and the only words an edit may carry.
    static let editable: Set<String> = ["id", "nickname", "state", "notes"]

    /// `POST /setup/gm/registry`: changes the nickname, state or notes of the
    /// record `id`, and nothing else. Anything but those, such as the folder,
    /// branch or tmux socket, is refused whole, with nothing changed.
    func edit(_ json: [String: Any]) throws -> Hire {
        let others = json.keys.filter { !FrontDesk.editable.contains($0) }.sorted()
        guard others.isEmpty else {
            throw Refusal(status: 400, message: "Only a hire's nickname, state and notes can be changed here, not \(others.joined(separator: ", ")).")
        }
        guard let id = json["id"] as? String, !id.isEmpty else { throw Refusal(status: 400, message: "Say which hire, by its id.") }
        guard json.count > 1 else { throw Refusal(status: 400, message: "Give a nickname, a state or notes to change.") }
        guard let i = hires.firstIndex(where: { $0.id == id }) else { throw Refusal(status: 404, message: "There's no hire with that id.") }
        var hire = hires[i]
        if let value = json["nickname"] {
            guard let text = value as? String else { throw Refusal(status: 400, message: "A nickname is text.") }
            let nickname = text.trimmingCharacters(in: .whitespacesAndNewlines)
            guard nickname.count <= FrontDesk.longestNickname, !nickname.contains(where: { $0.isNewline }) else {
                throw Refusal(status: 400, message: "A nickname is one line of at most \(FrontDesk.longestNickname) characters.")
            }
            hire.nickname = nickname
        }
        if let value = json["state"] {
            guard let text = value as? String, let state = State(rawValue: text) else {
                throw Refusal(status: 400, message: "State is one of \(State.allCases.map(\.rawValue).joined(separator: ", ")).")
            }
            hire.state = state
        }
        if let value = json["notes"] {
            guard let text = value as? String, text.count <= FrontDesk.longestNotes else {
                throw Refusal(status: 400, message: "Notes are text of at most \(FrontDesk.longestNotes) characters.")
            }
            hire.notes = text
        }
        hires[i] = hire
        save()
        return hire
    }

    // MARK: Files

    /// `contacts` and `keys` are left out of the file while empty, so a registry without
    /// them is read as it was before, and one without them is written as it was.
    private struct File: Codable {
        var version: Int
        var hires: [Hire]
        var contacts: [Contact]?
        var keys: [String: String]?
    }

    private static let encoder: JSONEncoder = {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
        return encoder
    }()

    private var file: File {
        File(version: FrontDesk.fileVersion, hires: hires, contacts: contacts.isEmpty ? nil : contacts, keys: sendKeys.isEmpty ? nil : sendKeys)
    }

    private func save() {
        guard let data = try? FrontDesk.encoder.encode(file) else { return }
        Paths.writePrivately(data, to: registryFile)
    }

    private static func load(_ file: URL) -> (hires: [Hire], contacts: [Contact], keys: [String: String]) {
        // A file that is there but can't be read is set aside like a bad one,
        // so the next save can't overwrite what might still be mended.
        if let data = try? Data(contentsOf: file) {
            if let found = try? JSONDecoder().decode(File.self, from: data), found.version <= fileVersion {
                return (found.hires, found.contacts ?? [], found.keys ?? [:])
            }
        } else if !FileManager.default.fileExists(atPath: file.path) {
            return ([], [], [:])
        }
        let aside = file.deletingLastPathComponent().appendingPathComponent("registry.broken.json")
        try? FileManager.default.removeItem(at: aside)
        try? FileManager.default.moveItem(at: file, to: aside)
        return ([], [], [:])
    }

    // MARK: Team

    /// Whether a live session works where `hire` does: in its worktree, if
    /// it has one, else in its folder, or the project it was started in.
    static func works(_ live: Live, for hire: Hire) -> Bool {
        // A worktree is kept as its physical path (/private/var/...), a session's folder may be written as /var/...: both are compared resolved.
        let places = Set([hire.worktree, hire.folder].filter { !$0.isEmpty }.flatMap { [$0, HireDesk.physical($0)] })
        return [live.folder, live.project].filter { !$0.isEmpty }.contains { places.contains($0) || places.contains(HireDesk.physical($0)) }
    }

    /// The team: one row for every hire in the registry, and one for every
    /// live session no hire matches (a session someone started by hand), in
    /// that order. A hire that is fired or failed claims no session, so one
    /// started later in the same folder is a row of its own. Each live
    /// session is claimed by at most one hire, and a hire by at most one
    /// session, the oldest listed first. `status` gives a folder's last STATUS line.
    static func team(hires: [Hire], live: [Live], now: Double, byHandGM: String? = nil, status: (String) -> String) -> [[String: Any]] {
        var claimed = Set<String>()
        var rows: [[String: Any]] = []
        for hire in hires {
            let open = hire.state != .fired && hire.state != .failed
            let session = open ? live.first { !claimed.contains($0.id) && works($0, for: hire) } : nil
            if let session { claimed.insert(session.id) }
            rows.append(row(hire: hire, live: session, now: now, status: status))
        }
        for session in live where !claimed.contains(session.id) {
            var extra = row(hire: nil, live: session, now: now, status: status)
            // A General Manager started by hand has no hire; the app knows its session.
            if session.id == byHandGM { extra["role"] = Deck.gmRole }
            rows.append(extra)
        }
        return rows
    }

    private static func row(hire: Hire?, live: Live?, now: Double, status: (String) -> String) -> [String: Any] {
        let folder = hire?.folder ?? live?.project ?? ""
        let place = hire.map { $0.worktree.isEmpty ? $0.folder : $0.worktree } ?? live?.folder ?? ""
        var row: [String: Any] = [
            "id": hire?.id ?? "live:" + (live?.id ?? ""),
            "registered": hire != nil, "live": live != nil,
            "name": hire?.name ?? Session.lastPart(of: folder),
            "nickname": hire?.nickname ?? "", "role": hire?.role ?? "",
            "folder": folder, "branch": hire?.branch ?? "", "worktree": hire?.worktree ?? "",
            "tmuxSocket": hire?.tmuxSocket ?? "", "account": hire?.account ?? "",
            "model": [hire?.model, live?.model].compactMap { $0 }.first { !$0.isEmpty } ?? "",
            "started": hire.map { $0.started as Any } ?? NSNull(),
            "state": hire?.state.rawValue ?? "",
            "notes": hire?.notes ?? "",
            "status": status(place),
        ]
        // Where the session is now, or what a hire with none is: gone.
        row["sessionId"] = live?.id ?? ""
        row["agent"] = live?.agent ?? ""
        row["liveState"] = live?.state ?? "gone"
        row["floor"] = live?.floor.map { $0 as Any } ?? NSNull()
        row["desk"] = live?.desk.map { $0 as Any } ?? NSNull()
        if let live, live.lastEventAt > 0 {
            row["lastEventAt"] = Int64(live.lastEventAt.rounded(.down))
            row["idleFor"] = Int(max(0, now - live.lastEventAt) / 1000)
        } else {
            row["lastEventAt"] = NSNull()
            row["idleFor"] = NSNull()
        }
        return row
    }

    /// The team as `GET /setup/gm/team` answers it.
    func teamJSON(live: [Live], now: Date = Date(), byHandGM: String? = nil, status: (String) -> String = FrontDesk.statusLine) -> [String: Any] {
        let at = now.timeIntervalSince1970 * 1000
        return ["at": Int(at), "team": FrontDesk.team(hires: hires, live: live, now: at, byHandGM: byHandGM, status: status), "contacts": contactsJSON]
    }

    /// The registry as the file keeps it, for a repair by hand.
    var registryJSON: [String: Any] {
        guard let data = try? FrontDesk.encoder.encode(file),
              let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return [:] }
        return object
    }

    /// `POST /setup/gm/registry`, with `{id, nickname?, state?, notes?}`:
    /// answers the record as it is now.
    func edit(body: Data) -> Response {
        guard let json = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any] else {
            return .status(400, "Send {id, nickname, state, notes}.")
        }
        do {
            let hire = try edit(json)
            let data = try FrontDesk.encoder.encode(hire)
            return .json(try JSONSerialization.jsonObject(with: data))
        } catch let refusal as Refusal {
            return .status(refusal.status, refusal.message)
        } catch {
            return .status(500, "Couldn't save the change.")
        }
    }

    /// The last line with words in it of the STATUS.md a hire keeps, in its
    /// folder or its .team/ folder, cut to 200 characters; "" without one.
    /// Only the file's last few kilobytes are read.
    static func statusLine(_ folder: String) -> String {
        guard folder.hasPrefix("/") else { return "" }
        for path in ["STATUS.md", ".team/STATUS.md"] {
            let file = URL(fileURLWithPath: folder).appendingPathComponent(path)
            // Only a plain file: opening a FIFO or a stalled mount would hang the app.
            guard (try? file.resolvingSymlinksInPath().resourceValues(forKeys: [.isRegularFileKey]))?.isRegularFile == true,
                  let handle = try? FileHandle(forReadingFrom: file) else { continue }
            defer { try? handle.close() }
            if let size = try? handle.seekToEnd(), size > 4096 { try? handle.seek(toOffset: size - 4096) } else { try? handle.seek(toOffset: 0) }
            guard let data = try? handle.read(upToCount: 4096) else { continue }
            let text = String(decoding: data, as: UTF8.self)
            if let line = text.split(whereSeparator: \.isNewline).last(where: { !$0.trimmingCharacters(in: .whitespaces).isEmpty }) {
                let trimmed = line.trimmingCharacters(in: .whitespaces)
                return trimmed.count > 200 ? String(trimmed.prefix(199)) + "…" : trimmed
            }
        }
        return ""
    }
}

extension FrontDesk.Hire {
    /// Reads a record with defaults for every key but the three that say who
    /// it is, so a registry written by hand or by the Linux and Windows app
    /// that leaves out the rest still loads.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        name = try c.decode(String.self, forKey: .name)
        folder = try c.decode(String.self, forKey: .folder)
        nickname = try c.decodeIfPresent(String.self, forKey: .nickname) ?? ""
        role = try c.decodeIfPresent(String.self, forKey: .role) ?? ""
        branch = try c.decodeIfPresent(String.self, forKey: .branch) ?? ""
        worktree = try c.decodeIfPresent(String.self, forKey: .worktree) ?? ""
        repo = try c.decodeIfPresent(String.self, forKey: .repo) ?? ""
        tmuxSocket = try c.decodeIfPresent(String.self, forKey: .tmuxSocket) ?? ""
        account = try c.decodeIfPresent(String.self, forKey: .account) ?? ""
        model = try c.decodeIfPresent(String.self, forKey: .model) ?? ""
        ports = try c.decodeIfPresent(String.self, forKey: .ports) ?? ""
        brief = try c.decodeIfPresent(String.self, forKey: .brief) ?? ""
        started = try c.decodeIfPresent(Double.self, forKey: .started) ?? 0
        state = try c.decodeIfPresent(FrontDesk.State.self, forKey: .state) ?? .starting
        notes = try c.decodeIfPresent(String.self, forKey: .notes) ?? ""
    }
}

extension Deck {
    /// The sessions the deck follows, with the floor and desk each sits at:
    /// the ground floor's by its key, the others' by the building.
    func frontDeskLive() -> [FrontDesk.Live] {
        let (keys, overflow) = layout()
        var seat: [String: (floor: Int, desk: Int)] = [:]
        for (index, key) in keys.enumerated() {
            if let session = key.session { seat[session.id] = (0, index) }
        }
        let above = building(overflow: overflow)
        for (floor, people) in above.floors.enumerated() where floor > 0 {
            for person in people { seat[person.id] = (floor, above.desks[person.id] ?? -1) }
        }
        return sessions.all.map { session in
            let place = seat[session.id]
            return FrontDesk.Live(id: session.id, agent: session.agent.rawValue, folder: session.folder, project: session.project,
                                  state: session.state.rawValue, model: session.model, floor: place?.floor,
                                  desk: place.flatMap { $0.desk >= 0 ? $0.desk : nil }, lastEventAt: session.lastEventAt)
        }
    }
}
