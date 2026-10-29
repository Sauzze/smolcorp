import AppKit
import os

// MARK: Cloud bots

/// A bot that works in someone else's cloud, not in a terminal here, as a
/// Grok Bot runs at xAI: it has no hooks and nothing to type into, so its
/// desk shows only whether it has news. A permanent one is always at work
/// (blue); each new report turns it green for a while (see `CloudDesk`).
/// One that comes and goes sits only between its "start" and "stop" events.
/// Set in config.json, through POST /setup/bots.
struct CloudBot: Codable, Equatable {
    /// Its address in /bots/<id>/event: letters, digits and dashes.
    var id = ""
    /// What its desk says.
    var name = ""
    /// The agent whose colours it wears, one of `Agent`'s.
    var agent = Agent.grok.rawValue
    /// What it wears in the office, one of the team's outfits.
    var outfit = "grok"
    /// Something extra on its desk, one of `Role.badges`, or none.
    var badge = ""
    /// Always at its desk, or only between "start" and "stop".
    var permanent = true
    /// Where its desk is: "upstairs", the floor above, by the headless runs,
    /// as it never has a terminal; or "ground", at a free key.
    var floor = "upstairs"
    /// Where its reports are seen, besides the event route.
    var source = Source()

    struct Source: Codable, Equatable {
        /// "grokBotApp", the Grok Bot app's own copy of its chats on this Mac
        /// (see `GrokBotApp`), or "none", the event route alone.
        var kind = "none"
        /// The bot's name in that app: how it's found when it has no
        /// `botId`, as in configs from before ids.
        var bot = ""
        /// The bot's id in that app, which a rename doesn't change: with it,
        /// the bot is found by that alone, and its desk says the name it has
        /// there now. See `GrokBotApp.resolve`.
        var botId = ""
        /// Which of its messages are reports: "unprompted", those in a turn
        /// you didn't start, as its routine's; or "any", replies to you too.
        var counts = "unprompted"

        static let kinds = ["none", "grokBotApp"]
        static let counting = ["unprompted", "any"]

        init(kind: String = "none", bot: String = "", botId: String = "", counts: String = "unprompted") {
            self.kind = kind
            self.bot = bot
            self.botId = botId
            self.counts = counts
        }

        /// Any part left out is the default's.
        init(from decoder: Decoder) throws {
            let c = try decoder.container(keyedBy: CodingKeys.self)
            kind = try c.decodeIfPresent(String.self, forKey: .kind) ?? kind
            bot = try c.decodeIfPresent(String.self, forKey: .bot) ?? bot
            botId = try c.decodeIfPresent(String.self, forKey: .botId) ?? botId
            counts = try c.decodeIfPresent(String.self, forKey: .counts) ?? counts
        }
    }

    init(id: String, name: String, agent: String = Agent.grok.rawValue, outfit: String = "grok", badge: String = "",
         permanent: Bool = true, floor: String = "upstairs", source: Source = Source()) {
        self.id = id
        self.name = name
        self.agent = agent
        self.outfit = outfit
        self.badge = badge
        self.permanent = permanent
        self.floor = floor
        self.source = source
    }

    /// Any part left out is the default's, as setup or a tool of yours may send only some.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decodeIfPresent(String.self, forKey: .id) ?? id
        name = try c.decodeIfPresent(String.self, forKey: .name) ?? name
        agent = try c.decodeIfPresent(String.self, forKey: .agent) ?? agent
        outfit = try c.decodeIfPresent(String.self, forKey: .outfit) ?? outfit
        badge = try c.decodeIfPresent(String.self, forKey: .badge) ?? badge
        permanent = try c.decodeIfPresent(Bool.self, forKey: .permanent) ?? permanent
        floor = try c.decodeIfPresent(String.self, forKey: .floor) ?? floor
        source = try c.decodeIfPresent(Source.self, forKey: .source) ?? source
    }

    static let floors = ["upstairs", "ground"]
    static let idLimit = 32
    static let most = 9

    /// As kept, or nil for one that can't be: a bad id or no name.
    var checked: CloudBot? {
        let id = id.lowercased()
        guard !id.isEmpty, id.count <= CloudBot.idLimit, id.allSatisfy({ $0.isASCII && ($0.isLetter || $0.isNumber || $0 == "-") }) else { return nil }
        let name = CloudBot.deskName(name)
        guard !name.isEmpty else { return nil }
        var source = source
        if !Source.kinds.contains(source.kind) { source.kind = "none" }
        source.bot = source.bot.oneLine(64)
        if source.kind == "grokBotApp" && source.bot.isEmpty { source.bot = name }
        source.botId = GrokBotApp.safeID(source.botId) ?? ""
        if !Source.counting.contains(source.counts) { source.counts = "unprompted" }
        return CloudBot(id: id, name: name, agent: Agent(rawValue: agent)?.rawValue ?? Agent.grok.rawValue,
                        outfit: TeamMember.choices["outfit"]?.contains(outfit) == true && outfit != "auto" ? outfit : "",
                        badge: Role.badges.contains(badge) ? badge : "", permanent: permanent,
                        floor: CloudBot.floors.contains(floor) ? floor : "upstairs", source: source)
    }

    /// Bots as kept: each checked, each id once, at most `most`.
    static func checked(_ bots: [CloudBot]) -> [CloudBot] {
        var kept: [CloudBot] = []
        for bot in bots.compactMap(\.checked) where !kept.contains(where: { $0.id == bot.id }) && kept.count < most {
            kept.append(bot)
        }
        return kept
    }

    /// What it wears, through the role path office.js already draws: no
    /// name, so its desk says only its own.
    var role: Role { Role(id: "cloud", name: "", agent: agent, model: "", effort: "", instructions: "", badge: badge, outfit: outfit) }

    /// Who its card upstairs says hired it: a Grok Bot is one of the Grok Bots.
    var hirer: String? { agent == Agent.grok.rawValue ? "Grok Bots" : nil }

    /// A name as its desk says it: no slash, as it's its folder's last part.
    static func deskName(_ name: String) -> String { name.replacingOccurrences(of: "/", with: "-").oneLine(Role.nameLimit) }

    var sessionID: String { "cloud-\(id)" }
    var tab: String { "cloud:\(id)" }
    var folder: String { CloudBot.folder(named: name) }
    static func folder(named name: String) -> String { "/cloud/\(name)" }
}

/// What a cloud bot last did, for its card: what, how many, and when.
struct CloudAction: Equatable {
    var text: String
    var count: Int
    var at: Date

    static let report = "Sent a report"
    static let textLimit = 60
    static let most = 99

    init(text: String = CloudAction.report, count: Int = 1, at: Date) {
        // Plain text: markup's tags go, then any bracket left.
        let untagged = text.replacingOccurrences(of: "<[^<>]*>", with: "", options: .regularExpression)
        let text = String(String.UnicodeScalarView(untagged.unicodeScalars.filter { $0 != "<" && $0 != ">" })).oneLine(CloudAction.textLimit)
        self.text = text.isEmpty ? CloudAction.report : text
        self.count = min(max(count, 1), CloudAction.most)
        self.at = at
    }

    /// For the stream: its time in milliseconds, as the office counts.
    var json: [String: Any] { ["text": text, "count": count, "at": Int64(at.timeIntervalSince1970 * 1000)] }

    /// What an event's body says, as `{"type": "report", "text": "Sent a
    /// report", "count": 3, "at": "<ISO 8601>"}`: every part but the type
    /// may be left out. Plain text only, cut short; a time to come is now.
    /// The words why for a part that can't be read.
    static func parse(_ body: [String: Any], now: Date) -> Result<CloudAction, CloudEventError> {
        var text = report
        if let given = body["text"], !(given is NSNull) {
            guard let given = given as? String else { return .failure(.init("text should be a string.")) }
            text = given
        }
        var count = 1
        if let given = body["count"], !(given is NSNull) {
            guard let number = given as? NSNumber, CFGetTypeID(number) != CFBooleanGetTypeID(),
                  number.doubleValue == number.doubleValue.rounded(), number.doubleValue >= 1 else {
                return .failure(.init("count should be a whole number, 1 or more."))
            }
            count = number.doubleValue > Double(most) ? most : number.intValue
        }
        var at = now
        if let given = body["at"], !(given is NSNull) {
            guard let string = given as? String, let date = parseTime(string) else {
                return .failure(.init("at should be an ISO 8601 time, as 2026-10-28T22:20:00Z."))
            }
            at = min(date, now)
        }
        return .success(CloudAction(text: text, count: count, at: at))
    }

    static func parseTime(_ string: String) -> Date? {
        let plain = ISO8601DateFormatter()
        let fractional = ISO8601DateFormatter()
        fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return plain.date(from: string) ?? fractional.date(from: string)
    }
}

struct CloudEventError: Error, Equatable {
    let message: String
    init(_ message: String) { self.message = message }
}

/// One cloud bot's desk, as a state: whether it's at its desk, when its
/// last report came, and what it last did. Green lasts `greenFor`, or until
/// its card is opened.
struct CloudDesk: Equatable {
    var present: Bool
    var reportAt: Date?
    /// Kept once seen, for its card.
    var last: CloudAction?

    /// Ten minutes: long enough to be seen across the room, gone well
    /// before an hourly routine's next run.
    static let greenFor: TimeInterval = 600

    init(permanent: Bool) {
        present = permanent
    }

    enum Event: String, CaseIterable {
        case report, seen, start, stop
    }

    /// What it shows, or nil when it isn't at its desk.
    func state(at now: Date) -> SessionState? {
        guard present else { return nil }
        if let reportAt, now.timeIntervalSince(reportAt) < CloudDesk.greenFor, now >= reportAt { return .unread }
        return .thinking
    }

    /// When it next changes by itself: when its green ends.
    func nextChange(after now: Date) -> Date? {
        guard state(at: now) == .unread, let reportAt else { return nil }
        return reportAt.addingTimeInterval(CloudDesk.greenFor)
    }

    /// A report comes to a bot that's away too: it sits down to give it.
    /// Reports not yet seen add up on its card; it's green from when the
    /// last arrived, whatever time it says it was sent.
    mutating func apply(_ event: Event, at now: Date, permanent: Bool, action: CloudAction? = nil) {
        switch event {
        case .report:
            var action = action ?? CloudAction(at: now)
            if reportAt != nil, let last { action.count = min(last.count + action.count, CloudAction.most) }
            last = action
            present = true
            reportAt = now
        case .seen:
            reportAt = nil
        case .start:
            present = true
        case .stop:
            // A permanent bot never leaves.
            present = permanent
            reportAt = nil
        }
    }
}

// MARK: Sources

/// Somewhere a cloud bot's reports can be seen from this Mac. Looked at
/// every few seconds, off the main thread; says how many reports are new
/// since the last look. The first look only learns what's there.
protocol CloudBotSource: AnyObject {
    func newReports() -> Int
    /// The name the bot has there now, as of the last look, when it's
    /// known by an id a rename doesn't change; else nil.
    var liveName: String? { get }
}

/// The Grok Bot app (xAI, com.anysphere.sand) keeps a copy of each chat
/// on this Mac, as plain JSON, in files named by their key in base32. Two
/// are read: the roster, for each bot's id and name, and that bot's
/// transcript. Of those, only ids, names, kinds, roles and order: never
/// what a message says, and no other file there (its secrets and cookies
/// are in others). Nothing is written. It's kept up while the app is open.
final class GrokBotApp: CloudBotSource {
    static let folder = FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent("Library/Application Support/Grok Bot/sand-client-persistence", isDirectory: true)
    #if DEBUG
    private static let log = Logger(subsystem: "io.github.sauzze.agentdeck", category: "cloudbots")
    #endif

    private let bot: String
    private let botId: String
    private let countsAll: Bool
    private let folder: URL
    /// The bot's messages already seen, by id: nil until the first look.
    private var known: Set<String>?
    /// The transcript's modification date at the last look, so an unchanged one isn't read again.
    private var lastRead: (file: URL, at: Date)?
    /// Who it was found to be at the last look, to warn once when that changes.
    private var lastFound: Resolution?
    private var looked = false
    private(set) var liveName: String?

    init(bot: String, botId: String = "", counts: String, folder: URL = GrokBotApp.folder) {
        self.bot = bot
        self.botId = botId
        countsAll = counts == "any"
        self.folder = folder
    }

    func newReports() -> Int {
        guard let names = try? FileManager.default.contentsOfDirectory(atPath: folder.path) else { return 0 }
        let keys = GrokBotApp.keys(of: names)
        guard let rows = GrokBotApp.roster(in: folder, keys: keys) else { return 0 }
        let found = GrokBotApp.resolve(botId: botId, name: bot, in: rows)
        note(found)
        liveName = found?.by == "id" ? found?.name : nil
        guard let found, let transcript = keys.first(where: { $0.key.hasSuffix(".transcript.replicas.\(found.botId)") }) else { return 0 }
        let file = folder.appendingPathComponent(transcript.value)
        // Another bot than before: learn its chat afresh.
        if lastRead?.file != file {
            known = nil
            lastRead = nil
        }
        let modified = (try? file.resourceValues(forKeys: [.contentModificationDateKey]))?.contentModificationDate ?? Date()
        if let lastRead, lastRead.at == modified, known != nil { return 0 }
        guard let data = FileManager.default.contents(atPath: file.path), let entries = GrokBotApp.entries(in: data) else { return 0 }
        lastRead = (file, modified)
        let bots = Set(GrokBotApp.botMessages(in: entries).map(\.id))
        guard let before = known else {
            known = bots
            return 0
        }
        known = before.union(bots)
        return GrokBotApp.reports(in: entries, new: bots.subtracting(before), countsAll: countsAll)
    }

    /// A word in the log when it can't be found, or only by a guess, or is
    /// renamed: once each time who it's found to be changes.
    private func note(_ found: Resolution?) {
        guard !looked || found != lastFound else { return }
        #if DEBUG
        let before = lastFound
        #endif
        lastFound = found
        looked = true
        // What it found, in a debug build's log: a release logs nothing (see scripts/check-public.sh).
        #if DEBUG
        guard let found else {
            let which = botId.isEmpty ? "named \"\(bot)\"" : "with id \(botId)"
            return GrokBotApp.log.warning("No Grok Bot \(which, privacy: .public) in the Grok Bot app: its desk sees no reports. See GET /setup/bots.")
        }
        if found.by == "guess" {
            GrokBotApp.log.warning("""
                No Grok Bot named "\(self.bot, privacy: .public)" in the Grok Bot app: following "\(found.name, privacy: .public)" \
                (\(found.botId, privacy: .public)), the one name like it. POST /setup/bots/resolve to keep it by its id.
                """)
        } else if let before, before.botId == found.botId, before.name != found.name {
            GrokBotApp.log.info("Grok Bot \(found.botId, privacy: .public) is now \"\(found.name, privacy: .public)\"")
        }
        #endif
    }

    // The parts below read data and nothing else, for the tests.

    /// Who a bot is in the app, and how that was told: "id", by the id a
    /// rename doesn't change; "name", by the name it's set with; "guess",
    /// by the one name in the roster like it, as after a rename.
    struct Resolution: Equatable {
        let botId: String
        let name: String
        let by: String

        var json: [String: Any] { ["botId": botId, "name": name, "by": by] }
    }

    /// One bot in the roster: its id and the name it has now, and nothing else of it.
    struct Row: Equatable {
        let id: String
        let name: String
    }

    /// Finds a bot: by `botId` alone when it has one (a bot by that id
    /// gone is no bot, not another one by its name); else by name, its case
    /// aside; else by the only name that holds it or that it holds, as
    /// "Smolcorp Mini" for "Smolcorp Mini Manager".
    static func resolve(botId: String, name: String, in rows: [Row]) -> Resolution? {
        if !botId.isEmpty {
            return rows.first { $0.id.lowercased() == botId.lowercased() }.map { Resolution(botId: $0.id, name: $0.name, by: "id") }
        }
        let wanted = name.lowercased().trimmingCharacters(in: .whitespaces)
        guard !wanted.isEmpty else { return nil }
        if let row = rows.first(where: { $0.name.lowercased() == wanted }) { return Resolution(botId: row.id, name: row.name, by: "name") }
        let like = rows.filter {
            let other = $0.name.lowercased().trimmingCharacters(in: .whitespaces)
            return !other.isEmpty && (other.contains(wanted) || wanted.contains(other))
        }
        guard like.count == 1 else { return nil }
        return Resolution(botId: like[0].id, name: like[0].name, by: "guess")
    }

    /// Who a bot set with `source` is in the app now, read afresh, or nil
    /// for one not from the app, or not in it.
    static func resolve(_ source: CloudBot.Source, folder: URL = GrokBotApp.folder) -> Resolution? {
        guard source.kind == "grokBotApp", let names = try? FileManager.default.contentsOfDirectory(atPath: folder.path),
              let rows = roster(in: folder, keys: keys(of: names)) else { return nil }
        return resolve(botId: source.botId, name: source.bot, in: rows)
    }

    /// The folder's files by their key.
    static func keys(of names: [String]) -> [String: String] {
        Dictionary(names.compactMap { name in key(ofFile: name).map { ($0, name) } }, uniquingKeysWith: { a, _ in a })
    }

    static func roster(in folder: URL, keys: [String: String]) -> [Row]? {
        guard let roster = keys.first(where: { $0.key.hasSuffix(".roster.last-roster") }),
              let data = FileManager.default.contents(atPath: folder.appendingPathComponent(roster.value).path) else { return nil }
        return GrokBotApp.roster(in: data)
    }

    /// A file's key: its name, less ".blob", read as base32 (RFC 4648, any case, no padding).
    static func key(ofFile name: String) -> String? {
        guard name.hasSuffix(".blob") else { return nil }
        let alphabet = Array("abcdefghijklmnopqrstuvwxyz234567")
        var bits = 0, value = 0
        var bytes: [UInt8] = []
        for c in name.dropLast(5).lowercased() {
            guard let i = alphabet.firstIndex(of: c) else { return nil }
            value = (value << 5) | i
            bits += 5
            if bits >= 8 {
                bits -= 8
                bytes.append(UInt8((value >> bits) & 0xff))
            }
            value &= (1 << bits) - 1
        }
        return String(bytes: bytes, encoding: .utf8)
    }

    /// The roster's bots, each with a safe id and a name.
    static func roster(in data: Data) -> [Row]? {
        guard let file = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
              let rows = (file["value"] as? [String: Any])?["rows"] as? [[String: Any]] else { return nil }
        return rows.compactMap { row in
            guard let id = (row["id"] as? String).flatMap(safeID), let name = row["name"] as? String else { return nil }
            return Row(id: id, name: name)
        }
    }

    /// An id as the app gives it, or nil: it names a file, so nothing that could reach outside the folder.
    static func safeID(_ id: String) -> String? {
        !id.isEmpty && id.count <= 64 && id.allSatisfy({ $0.isASCII && ($0.isHexDigit || $0 == "-") }) ? id : nil
    }

    /// The id of the bot called `name` in the roster, its case aside.
    static func botID(named name: String, inRoster data: Data) -> String? {
        let wanted = name.lowercased()
        return roster(in: data)?.first { $0.name.lowercased() == wanted }?.id
    }

    /// One line of a transcript, as much of it as is read.
    struct Entry: Equatable {
        let id: String
        let kind: String
        let role: String
    }

    static func entries(in data: Data) -> [Entry]? {
        guard let file = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
              let list = (file["value"] as? [String: Any])?["entries"] as? [[String: Any]] else { return nil }
        let ordered = list.enumerated().sorted { a, b in
            ((a.element["seq"] as? Int) ?? Int.max, a.offset) < ((b.element["seq"] as? Int) ?? Int.max, b.offset)
        }
        return ordered.compactMap { _, entry in
            guard let id = entry["id"] as? String else { return nil }
            return Entry(id: id, kind: entry["kind"] as? String ?? "", role: entry["role"] as? String ?? "")
        }
    }

    /// The bot's own messages: what it sent, as against yours and the chat's events.
    static func botMessages(in entries: [Entry]) -> [Entry] {
        entries.filter { $0.kind == "send-message" || ($0.kind == "message" && $0.role == "assistant") }
    }

    /// The turn a message is in, from its id: "t9" for t9u (yours) and
    /// t9s0, t9s1… (the bot's), or nil for an id of another shape.
    static func turn(of id: String) -> String? {
        guard id.hasPrefix("t"), let end = id.dropFirst().firstIndex(where: { !$0.isNumber }), end > id.index(after: id.startIndex) else { return nil }
        let rest = id[end...]
        guard rest == "u" || (rest.hasPrefix("s") && rest.count > 1 && rest.dropFirst().allSatisfy(\.isNumber)) else { return nil }
        return String(id[..<end])
    }

    /// How many reports `new` (ids of the bot's messages) brings: one per
    /// turn. Unless every message counts, a turn you started, by writing
    /// to it, is a reply and none. A message whose turn can't be told counts.
    static func reports(in entries: [Entry], new: Set<String>, countsAll: Bool) -> Int {
        let yours = Set(entries.filter { $0.role == "user" }.compactMap { turn(of: $0.id) })
        var turns = Set<String>()
        var untold = 0
        for entry in botMessages(in: entries) where new.contains(entry.id) {
            guard let turn = turn(of: entry.id) else {
                untold += 1
                continue
            }
            if countsAll || !yours.contains(turn) { turns.insert(turn) }
        }
        return turns.count + untold
    }
}

// MARK: Desks

/// The cloud bots' desks: their sessions, as the office draws them, kept
/// here rather than in `Sessions`, so nothing that looks for tabs, hooks or
/// processes ever touches them. They sit upstairs, first, before the
/// headless runs, or at free desks on the ground floor as any new session
/// would; they have no controls.
final class CloudDesks {
    #if DEBUG
    private static let log = Logger(subsystem: "io.github.sauzze.agentdeck", category: "cloudbots")
    #endif
    /// How often its sources are looked at.
    static let lookInterval: TimeInterval = 5

    var onChange: () -> Void = {}
    private var bots: [CloudBot] = []
    private var desks: [String: CloudDesk] = [:]
    private var sessionsByID: [String: Session] = [:]
    private var sources: [String: CloudBotSource] = [:]
    /// The name each bot found by its id has in its app now, which its desk
    /// says instead of the one it was set with.
    private var liveNames: [String: String] = [:]
    private let queue = DispatchQueue(label: "smolcorp.cloudbots")
    private var looking = false
    private var expiry: DispatchWorkItem?

    init(_ bots: [CloudBot] = Config.shared.cloudBots, looking: Bool = true) {
        configure(bots)
        guard looking else { return }
        Timer.scheduledTimer(withTimeInterval: CloudDesks.lookInterval, repeats: true) { [weak self] _ in self?.look() }
    }

    /// Takes the bots as set: a bot kept keeps its desk and state; a new one
    /// starts now, at work if permanent.
    func configure(_ given: [CloudBot]) {
        let now = Date()
        let before = Dictionary(bots.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        bots = given
        desks = desks.filter { id, _ in given.contains { $0.id == id } }
        sessionsByID = sessionsByID.filter { id, _ in given.contains { $0.sessionID == id } }
        for bot in given {
            if desks[bot.id] == nil || before[bot.id]?.permanent != bot.permanent {
                var desk = desks[bot.id] ?? CloudDesk(permanent: bot.permanent)
                if bot.permanent { desk.present = true }
                desks[bot.id] = desk
            }
            if before[bot.id]?.source != bot.source || sources[bot.id] == nil {
                liveNames[bot.id] = nil
                switch bot.source.kind {
                case "grokBotApp": sources[bot.id] = GrokBotApp(bot: bot.source.bot, botId: bot.source.botId, counts: bot.source.counts)
                default: sources[bot.id] = nil
                }
            }
            // A new name, agent or source starts it afresh.
            if before[bot.id] != bot || sessionsByID[bot.sessionID] == nil {
                let folder = CloudBot.folder(named: liveNames[bot.id] ?? bot.name)
                let session = Session(id: bot.sessionID, agent: Agent(rawValue: bot.agent) ?? .grok, folder: folder,
                                      project: folder, startedAt: sessionsByID[bot.sessionID]?.startedAt ?? now.timeIntervalSince1970 * 1000)
                session.tab = bot.tab
                sessionsByID[bot.sessionID] = session
            }
        }
        sources = sources.filter { id, _ in given.contains { $0.id == id } }
        liveNames = liveNames.filter { id, _ in given.contains { $0.id == id } }
        update()
    }

    /// The desks to seat on `floor`, in the order the bots were set.
    func seated(on floor: String) -> [Session] {
        bots.compactMap { bot in
            guard bot.floor == floor, let session = sessionsByID[bot.sessionID], session.state != .idle else { return nil }
            return session
        }
    }

    func bot(ofSession id: String) -> CloudBot? { bots.first { $0.sessionID == id } }

    /// An event for bot `id`, from the event route or a source. False for a bot there isn't.
    @discardableResult
    func apply(_ event: CloudDesk.Event, to id: String, action: CloudAction? = nil) -> Bool {
        guard let bot = bots.first(where: { $0.id == id }) else { return false }
        let before = desks[id]
        desks[id, default: CloudDesk(permanent: bot.permanent)].apply(event, at: Date(), permanent: bot.permanent, action: action)
        #if DEBUG
        CloudDesks.log.info("\(bot.id, privacy: .public): \(event.rawValue, privacy: .public)")
        #endif
        // Its card may change with no change of colour, as a second report adds up.
        update(changed: desks[id] != before)
        return true
    }

    /// Its app says bot `id` is now called `name`: its desk says so too,
    /// at the same seat, in the same state, with the same card.
    func rename(_ id: String, to name: String?) {
        guard let bot = bots.first(where: { $0.id == id }), let session = sessionsByID[bot.sessionID] else { return }
        let shown = name.map(CloudBot.deskName).flatMap { $0.isEmpty ? nil : $0 }
        liveNames[id] = shown
        let folder = CloudBot.folder(named: shown ?? bot.name)
        guard session.folder != folder else { return }
        #if DEBUG
        CloudDesks.log.info("\(bot.id, privacy: .public): its desk now says \"\(shown ?? bot.name, privacy: .public)\"")
        #endif
        session.folder = folder
        session.project = folder
        onChange()
    }

    /// What the bot at desk `id` last did, for its card.
    func last(ofSession id: String) -> CloudAction? { bot(ofSession: id).flatMap { desks[$0.id]?.last } }

    /// Its card was opened, on the Mac or a phone: its news is seen, and
    /// nothing else happens. False for a desk that isn't a cloud bot's.
    func seen(session id: String) -> Bool {
        guard let bot = bot(ofSession: id) else { return false }
        return apply(.seen, to: bot.id)
    }

    /// The desk was tapped: its news is seen, and the app it works in comes forward.
    func tapped(session id: String) -> Bool {
        guard let bot = bot(ofSession: id) else { return false }
        apply(.seen, to: bot.id)
        if bot.source.kind == "grokBotApp", let app = NSWorkspace.shared.urlForApplication(withBundleIdentifier: "com.anysphere.sand") {
            NSWorkspace.shared.openApplication(at: app, configuration: NSWorkspace.OpenConfiguration())
        }
        return true
    }

    /// What each desk shows now, its sessions brought in line, and the next
    /// change by itself planned.
    private func update(changed: Bool = false) {
        let now = Date()
        var changed = changed
        for bot in bots {
            guard let session = sessionsByID[bot.sessionID] else { continue }
            let state = desks[bot.id]?.state(at: now) ?? .idle
            if session.state != state {
                session.state = state
                changed = true
            }
        }
        expiry?.cancel()
        if let next = desks.values.compactMap({ $0.nextChange(after: now) }).min() {
            let work = DispatchWorkItem { [weak self] in self?.update() }
            expiry = work
            DispatchQueue.main.asyncAfter(deadline: .now() + max(0.1, next.timeIntervalSince(now) + 0.1), execute: work)
        }
        if changed { onChange() }
    }

    /// Asks each source what's new, off the main thread, one look at a time.
    private func look() {
        guard !looking, !sources.isEmpty else { return }
        looking = true
        let sources = self.sources
        queue.async {
            let news = sources.mapValues { ($0.newReports(), $0.liveName) }
            DispatchQueue.main.async {
                self.looking = false
                // Only from a source still set for its bot.
                for (id, (_, name)) in news where self.sources[id] === sources[id] { self.rename(id, to: name) }
                for (id, (count, _)) in news where count > 0 && self.sources[id] === sources[id] {
                    #if DEBUG
                    CloudDesks.log.info("\(id, privacy: .public): \(count) new report(s) in its source")
                    #endif
                    self.apply(.report, to: id, action: CloudAction(count: count, at: Date()))
                }
            }
        }
    }
}
