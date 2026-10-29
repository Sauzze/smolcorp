import Foundation

/// ⓘ Info (personal build): what an agent's session has used, for a card on
/// the Mac's office and in the iPhone app. Only what this Mac already has, and
/// only what's there: a fact with no source here is left out of the card, not
/// guessed at.
///
/// - Claude Code's transcript: each reply's model, effort and tokens, and how
///   long each turn took. Read once, then only what's been added since.
/// - The profile Claude Code keeps beside its settings (`.claude.json`): whose
///   subscription it is, and the 5-hour and weekly use Claude Code last fetched.
/// - The hook: whether the agent has an API key, or a cloud provider, in its
///   environment. Only that one is there: never the key.
///
/// Nothing is sent anywhere: the card reads it from this Mac, as the
/// conversation does. No key, token or credential is read, kept or answered.

// MARK: Prices

/// Anthropic's published API prices, per million tokens, from
/// platform.claude.com/docs/en/about-claude/pricing as of 2026-10-23. Cache
/// writes are the input price × 1.25 for 5 minutes and × 2 for an hour, on
/// every model. Claude Code's own cost for a session (its `cost-state`) comes
/// out the same from these.
enum Prices {
    struct Rate: Equatable {
        var input: Double
        var output: Double
        var cacheRead: Double
        /// Fast mode's prices, as a multiple of these: only where fast mode has a price.
        var fast: Double?
        /// Over this many prompt tokens a request is priced at `long` instead (Haiku 5.5).
        var longFrom: Int?
        var longInput = 0.0
        var longOutput = 0.0
        var longCacheRead = 0.0

        init(_ input: Double, _ output: Double, _ cacheRead: Double, fast: Double? = nil) {
            self.input = input
            self.output = output
            self.cacheRead = cacheRead
            self.fast = fast
        }
    }

    static let table: [String: Rate] = {
        var haiku55 = Rate(0.10, 0.50, 0.01)
        haiku55.longFrom = 100_000
        (haiku55.longInput, haiku55.longOutput, haiku55.longCacheRead) = (0.50, 2.50, 0.05)
        return [
            "claude-fable-5-1": Rate(10, 50, 0.25), "claude-mythos-5-1": Rate(10, 50, 0.25),
            "claude-fable-5": Rate(10, 50, 1), "claude-mythos-5": Rate(10, 50, 1),
            "claude-opus-5-5": Rate(4, 20, 0.20, fast: 2), "claude-opus-5": Rate(5, 25, 0.50, fast: 2),
            "claude-opus-4-8": Rate(5, 25, 0.50, fast: 2), "claude-opus-4-7": Rate(5, 25, 0.50),
            "claude-opus-4-6": Rate(5, 25, 0.50), "claude-opus-4-5": Rate(5, 25, 0.50),
            "claude-sonnet-5-5": Rate(2, 10, 0.10), "claude-sonnet-5": Rate(2, 10, 0.20),
            "claude-sonnet-4-6": Rate(3, 15, 0.30), "claude-sonnet-4-5": Rate(3, 15, 0.30),
            "claude-haiku-5-5": haiku55, "claude-haiku-4-5": Rate(1, 5, 0.10),
        ]
    }()

    /// A model as the table names it: without Claude Code's "[1m]" or a snapshot's date.
    static func id(_ model: String) -> String {
        var id = model.lowercased().trimmingCharacters(in: .whitespaces)
        if let bracket = id.firstIndex(of: "[") { id = String(id[..<bracket]) }
        if let dash = id.lastIndex(of: "-"), id.distance(from: dash, to: id.endIndex) == 9,
           id[id.index(after: dash)...].allSatisfy(\.isNumber) {
            id = String(id[..<dash])
        }
        return id
    }

    static func rate(_ model: String) -> Rate? { table[id(model)] }

    /// One request's price in dollars, or nil for a model, or a fast mode, with no price here.
    static func cost(_ tokens: Tokens, model: String, fast: Bool = false) -> Double? {
        guard let rate = rate(model) else { return nil }
        var multiple = 1.0
        if fast {
            guard let times = rate.fast else { return nil }
            multiple = times
        }
        let long = rate.longFrom.map { tokens.prompt > $0 } ?? false
        let input = long ? rate.longInput : rate.input
        let output = long ? rate.longOutput : rate.output
        let read = long ? rate.longCacheRead : rate.cacheRead
        let sum = Double(tokens.input) * input + Double(tokens.output) * output + Double(tokens.cacheRead) * read
            + Double(tokens.write5m) * input * 1.25 + Double(tokens.write1h) * input * 2
        return sum * multiple / 1_000_000
    }
}

/// Tokens, as a reply's `usage` counts them.
struct Tokens: Equatable {
    var input = 0
    var output = 0
    var cacheRead = 0
    var write5m = 0
    var write1h = 0

    var cacheWrite: Int { write5m + write1h }
    /// What was sent: what a long prompt's price is decided by.
    var prompt: Int { input + cacheRead + cacheWrite }

    init(input: Int = 0, output: Int = 0, cacheRead: Int = 0, write5m: Int = 0, write1h: Int = 0) {
        (self.input, self.output, self.cacheRead, self.write5m, self.write1h) = (input, output, cacheRead, write5m, write1h)
    }

    /// From a reply's `usage`; a write not split by how long it's kept counts as 5 minutes'.
    init(usage: [String: Any]) {
        func count(_ object: [String: Any]?, _ key: String) -> Int { (object?[key] as? NSNumber)?.intValue ?? 0 }
        input = count(usage, "input_tokens")
        output = count(usage, "output_tokens")
        cacheRead = count(usage, "cache_read_input_tokens")
        if let split = usage["cache_creation"] as? [String: Any] {
            write5m = count(split, "ephemeral_5m_input_tokens")
            write1h = count(split, "ephemeral_1h_input_tokens")
        } else {
            write5m = count(usage, "cache_creation_input_tokens")
        }
    }

    static func + (a: Tokens, b: Tokens) -> Tokens {
        Tokens(input: a.input + b.input, output: a.output + b.output, cacheRead: a.cacheRead + b.cacheRead,
               write5m: a.write5m + b.write5m, write1h: a.write1h + b.write1h)
    }

    static func - (a: Tokens, b: Tokens) -> Tokens {
        Tokens(input: a.input - b.input, output: a.output - b.output, cacheRead: a.cacheRead - b.cacheRead,
               write5m: a.write5m - b.write5m, write1h: a.write1h - b.write1h)
    }
}

// MARK: The transcript

/// What a Claude Code transcript says of its session, line by line: its
/// replies' tokens and their price, the latest reply's model and effort, when
/// it started, and how long its turns took.
struct Tally {
    var tokens = Tokens()
    /// The replies' price, in dollars, at `Prices`.
    var cost = 0.0
    /// A reply from a model, or in a mode, with no price here: `cost` leaves it out.
    var unpriced = 0
    /// The latest reply's, outside subagents.
    var model = ""
    var effort = ""
    var fast = false
    /// The first line's time, in ms.
    var startedAt: Double?
    /// The turns' time, as Claude Code measured each, in ms.
    var activeMs = 0.0

    /// Each reply counted, by its message's id: a reply is written once for each of its blocks, all with the
    /// same usage, and counted once.
    private var counted: [String: (tokens: Tokens, cost: Double?)] = [:]

    mutating func take(_ entry: [String: Any]) {
        if startedAt == nil, let time = (entry["timestamp"] as? String).flatMap(Tally.time) { startedAt = time }
        switch entry["type"] as? String {
        case "assistant":
            guard let message = entry["message"] as? [String: Any], let model = message["model"] as? String,
                  !model.isEmpty, model != "<synthetic>" else { return }
            let usage = message["usage"] as? [String: Any] ?? [:]
            let fast = usage["speed"] as? String == "fast"
            if entry["isSidechain"] as? Bool != true {
                self.model = model
                self.fast = fast
                if let effort = entry["effort"] as? String, !effort.isEmpty { self.effort = effort }
            }
            let tokens = Tokens(usage: usage)
            let price = Prices.cost(tokens, model: model, fast: fast)
            let id = message["id"] as? String ?? UUID().uuidString
            if let earlier = counted[id] { add(earlier.tokens, earlier.cost, sign: -1) }
            counted[id] = (tokens, price)
            add(tokens, price, sign: 1)
        case "system":
            if entry["subtype"] as? String == "turn_duration", let ms = (entry["durationMs"] as? NSNumber)?.doubleValue, ms > 0 {
                activeMs += ms
            }
        default:
            break
        }
    }

    /// A subagent's replies: their tokens and price, not its model or turns.
    mutating func takeSubagent(_ other: Tally) {
        tokens = tokens + other.tokens
        cost += other.cost
        unpriced += other.unpriced
    }

    private mutating func add(_ tokens: Tokens, _ price: Double?, sign: Int) {
        self.tokens = sign > 0 ? self.tokens + tokens : self.tokens - tokens
        if let price { cost += Double(sign) * price } else { unpriced += sign }
    }

    static func time(_ text: String) -> Double? {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let date = formatter.date(from: text) ?? ISO8601DateFormatter().date(from: text)
        return date.map { $0.timeIntervalSince1970 * 1000 }
    }
}

/// A transcript, followed as it grows: each look reads only what's been
/// added, whole lines only, and only the lines a `Tally` takes are parsed.
final class TranscriptFollower {
    let path: String
    private(set) var tally = Tally()
    private var offset: UInt64 = 0
    private var pending = Data()
    var lastUsed = Date()

    /// Read at a time.
    static let chunk = 4 * 1024 * 1024
    /// A line longer than this is a tool's whole output: skipped, not parsed.
    static let lineMost = 2 * 1024 * 1024
    /// What a line must hold to be parsed.
    private static let wanted = ["\"assistant\"", "\"turn_duration\""].map { Data($0.utf8) }
    private static let timestamp = Data("\"timestamp\"".utf8)

    init(path: String) {
        self.path = path
    }

    /// Reads what's new. A file that got shorter is read again from its start.
    func update() {
        lastUsed = Date()
        guard let file = FileHandle(forReadingAtPath: path) else { return }
        defer { try? file.close() }
        let size = (try? file.seekToEnd()) ?? 0
        if size < offset {
            tally = Tally()
            offset = 0
            pending = Data()
        }
        guard size > offset, (try? file.seek(toOffset: offset)) != nil else { return }
        while let data = try? file.read(upToCount: TranscriptFollower.chunk), !data.isEmpty {
            offset += UInt64(data.count)
            var rest = data[...]
            while let newline = rest.firstIndex(of: 0x0A) {
                let line = rest[..<newline]
                if pending.isEmpty { take(line) } else if pending.count + line.count <= TranscriptFollower.lineMost {
                    take(pending + line)
                }
                pending = Data()
                rest = rest[rest.index(after: newline)...]
            }
            // The line not yet ended: kept for the next read, unless it's already too long to parse.
            if pending.count + rest.count <= TranscriptFollower.lineMost { pending += rest } else { pending = Data(count: TranscriptFollower.lineMost + 1) }
        }
    }

    private func take<D: DataProtocol>(_ line: D) {
        guard line.count <= TranscriptFollower.lineMost else { return }
        let bytes = Data(line)
        let wanted = TranscriptFollower.wanted.contains { bytes.range(of: $0) != nil }
            || (tally.startedAt == nil && bytes.range(of: TranscriptFollower.timestamp) != nil)
        guard wanted, let entry = try? JSONSerialization.jsonObject(with: bytes) as? [String: Any] else { return }
        tally.take(entry)
    }
}

// MARK: The account

/// Whose Claude Code it is, from the profile it keeps beside its settings:
/// never its keys, which it keeps in the Keychain anyway.
struct ClaudeAccount: Equatable {
    /// Its email, or its name where there's no email.
    var name: String?
    /// "Max 20x", "Pro"…, where the profile says.
    var plan: String?
    var subscription: Bool
    var fiveHour: Window?
    var weekly: Window?
    /// When Claude Code fetched that use, in ms.
    var usageAt: Double?

    struct Window: Equatable {
        var percent: Double
        /// In ms.
        var resetsAt: Double?
    }

    /// Use fetched longer ago than this isn't shown.
    static let usageFresh: Double = 60 * 60 * 1000

    /// The account in `profile` (a `.claude.json`), with the use Claude Code
    /// last fetched for it: only that account's, only fresh, and only windows
    /// that haven't reset since. Nil with no subscription login.
    static func from(_ profile: [String: Any], now: Double) -> ClaudeAccount? {
        guard let oauth = profile["oauthAccount"] as? [String: Any] else { return nil }
        let text = { (key: String) -> String? in
            (oauth[key] as? String).flatMap { $0.trimmingCharacters(in: .whitespacesAndNewlines) }.flatMap { $0.isEmpty ? nil : $0 }
        }
        var account = ClaudeAccount(name: text("emailAddress") ?? text("displayName"), plan: plan(text("organizationType"),
                                    tier: text("organizationRateLimitTier") ?? text("userRateLimitTier")),
                                    subscription: false)
        account.subscription = account.plan != nil || (text("billingType") ?? "").contains("subscription")

        // Claude Code's own cache of what /usage shows, for this account only.
        if account.subscription, let cache = profile["cachedUsageUtilization"] as? [String: Any],
           let owner = cache["accountUuid"] as? String, owner == text("accountUuid"),
           let fetched = (cache["fetchedAtMs"] as? NSNumber)?.doubleValue, now - fetched <= usageFresh, fetched <= now + 60_000,
           let windows = cache["utilization"] as? [String: Any] {
            func window(_ key: String) -> Window? {
                guard let object = windows[key] as? [String: Any], let percent = (object["utilization"] as? NSNumber)?.doubleValue,
                      percent >= 0 else { return nil }
                let resets = (object["resets_at"] as? String).flatMap(Tally.time)
                // Reset since it was fetched: what's been used since isn't known.
                if let resets, resets <= now { return nil }
                return Window(percent: min(percent, 100), resetsAt: resets)
            }
            account.fiveHour = window("five_hour")
            account.weekly = window("seven_day")
            if account.fiveHour != nil || account.weekly != nil { account.usageAt = fetched }
        }
        return account
    }

    /// "claude_max" with "default_claude_max_20x": "Max 20x".
    static func plan(_ type: String?, tier: String?) -> String? {
        let names = ["claude_max": "Max", "claude_pro": "Pro", "claude_team": "Team", "claude_enterprise": "Enterprise"]
        guard let type, let name = names[type] else { return nil }
        if let tier, let last = tier.split(separator: "_").last, last.hasSuffix("x"), last.dropLast().allSatisfy(\.isNumber),
           !last.dropLast().isEmpty {
            return "\(name) \(last)"
        }
        return name
    }

    /// The folder of settings a transcript's Claude Code reads: `~/.claude`, or a `CLAUDE_CONFIG_DIR` of its own.
    static func configFolder(transcript: String, home: String = Paths.home.path) -> String {
        guard let range = transcript.range(of: "/projects/", options: .backwards) else { return home + "/.claude" }
        return String(transcript[..<range.lowerBound])
    }

    /// The profile Claude Code keeps for that folder: `~/.claude.json` for `~/.claude`, and inside any other.
    static func profilePath(folder: String, home: String = Paths.home.path) -> String {
        folder == home + "/.claude" ? home + "/.claude.json" : folder + "/.claude.json"
    }
}

// MARK: The card

/// Every desk's facts, kept up to date as the card asks for them, on a queue
/// of their own.
final class SessionInfo {
    static let shared = SessionInfo()

    /// How the agent pays, from its environment, as its hook says: never more than which.
    enum Billing: String {
        case apiKey = "api-key"
        /// Bedrock, Vertex or Foundry: their own prices.
        case cloud
    }

    /// What the card needs of a session, taken on the main queue.
    struct Desk {
        var id: String
        var agent: Agent
        var project: String
        var projectPath: String
        var transcript: String
        var startedAt: Double
        var model: String
        var effort: String
        var turnStartedAt: Double?
    }

    private let queue = DispatchQueue(label: "smolcorp.session-info")
    private var followers: [String: TranscriptFollower] = [:]
    private var profiles: [String: (modified: Date, account: ClaudeAccount?)] = [:]
    private var billing: [String: Billing] = [:]
    /// Transcripts followed at most: the least recently asked for go.
    static let followersMost = 48

    /// From each hook event: the session's `X-AgentDeck-Billing`, or nothing.
    func note(billing header: String?, for session: String) {
        guard !session.isEmpty else { return }
        let kind = header.flatMap(Billing.init(rawValue:))
        queue.async { self.billing[session] = kind }
    }

    /// The card's facts for `desk`, on the main queue.
    func report(_ desk: Desk, done: @escaping ([String: Any]) -> Void) {
        queue.async {
            let report = self.report(desk, now: Date().timeIntervalSince1970 * 1000)
            DispatchQueue.main.async { done(report) }
        }
    }

    private func report(_ desk: Desk, now: Double) -> [String: Any] {
        var tally: Tally?
        var account: ClaudeAccount?
        var apiKey = false
        if desk.agent == .claude, !desk.transcript.isEmpty {
            tally = follow(desk.transcript)
            let subagents = (desk.transcript as NSString).deletingPathExtension + "/subagents"
            for name in (try? FileManager.default.contentsOfDirectory(atPath: subagents)) ?? [] where name.hasSuffix(".jsonl") {
                tally?.takeSubagent(follow(subagents + "/" + name))
            }
            let folder = ClaudeAccount.configFolder(transcript: desk.transcript)
            account = self.account(ClaudeAccount.profilePath(folder: folder), now: now)
            apiKey = SessionInfo.hasKeyHelper(settings: [folder + "/settings.json", desk.projectPath + "/.claude/settings.json",
                                                         desk.projectPath + "/.claude/settings.local.json"])
        }
        trim()
        return SessionInfo.card(desk, tally: tally, account: account, billing: apiKey ? .apiKey : billing[desk.id], now: now)
    }

    /// The card's facts, from what's been gathered: a fact with no source is left out.
    static func card(_ desk: Desk, tally: Tally?, account: ClaudeAccount?, billing: Billing?, now: Double) -> [String: Any] {
        var card: [String: Any] = ["agent": desk.agent.name, "desk": desk.project]

        // Who pays: an API key or a cloud provider in its environment wins over the profile's login.
        switch billing {
        case .apiKey?: card["account"] = ["kind": "api-key"]
        case .cloud?: card["account"] = ["kind": "cloud"]
        case nil:
            if let account, account.subscription {
                var shown: [String: Any] = ["kind": "subscription"]
                if let name = account.name { shown["name"] = name }
                if let plan = account.plan { shown["plan"] = plan }
                card["account"] = shown
                var limits: [String: Any] = [:]
                func window(_ window: ClaudeAccount.Window?) -> [String: Any]? {
                    guard let window else { return nil }
                    var shown: [String: Any] = ["percent": window.percent]
                    if let resets = window.resetsAt { shown["resetsInMs"] = max(0, resets - now) }
                    return shown
                }
                if let five = window(account.fiveHour) { limits["fiveHour"] = five }
                if let week = window(account.weekly) { limits["weekly"] = week }
                if !limits.isEmpty, let at = account.usageAt {
                    limits["ageMs"] = max(0, now - at)
                    card["limits"] = limits
                }
            }
        }

        // The model actually running: the latest reply's, before the one the hook named.
        let model = tally.map(\.model).flatMap { $0.isEmpty ? nil : $0 } ?? (desk.model.isEmpty ? nil : desk.model)
        if let model {
            var shown: [String: Any] = ["id": model, "name": SessionInfo.modelName(model)]
            let effort = tally.map(\.effort).flatMap { $0.isEmpty ? nil : $0 } ?? (desk.effort.isEmpty ? nil : desk.effort)
            if let effort { shown["effort"] = effort }
            if tally?.fast == true { shown["fast"] = true }
            card["model"] = shown
        }

        // Time: since the session started; its turns, from the transcript; and on this desk, through /clear.
        var time: [String: Any] = [:]
        let started = tally?.startedAt ?? desk.startedAt
        if started > 0, started <= now { time["sessionMs"] = now - started }
        if let tally {
            let running = desk.turnStartedAt.map { max(0, now - $0) } ?? 0
            if tally.activeMs > 0 || running > 0 { time["activeMs"] = tally.activeMs + running }
            if desk.startedAt > 0, started - desk.startedAt > 60_000 { time["deskMs"] = now - desk.startedAt }
        }
        if !time.isEmpty { card["time"] = time }

        // Tokens, and their price: the bill with an API key, what they'd have cost with a subscription.
        if let tally, tally.tokens != Tokens() {
            let tokens = tally.tokens
            card["tokens"] = ["input": tokens.input, "output": tokens.output, "cacheRead": tokens.cacheRead,
                              "cacheWrite": tokens.cacheWrite]
            // A cloud provider has prices of its own, and a reply with no price here would make the sum wrong.
            if billing != .cloud, tally.unpriced == 0 {
                // Billed as such only where an API key is seen; otherwise what it would cost at the API's prices.
                card["cost"] = ["usd": (tally.cost * 10_000).rounded() / 10_000, "basis": billing == .apiKey ? "api" : "equivalent"]
            }
        }
        return card
    }

    /// "claude-opus-5-5[1m]": "Opus 5.5 · 1M". Anything else, as it is.
    static func modelName(_ model: String) -> String {
        let long = model.lowercased().hasSuffix("[1m]")
        let id = Prices.id(model)
        guard id.hasPrefix("claude-") else { return model }
        let parts = id.dropFirst("claude-".count).split(separator: "-")
        guard let family = parts.first, parts.count >= 2, parts.dropFirst().allSatisfy({ $0.allSatisfy(\.isNumber) }) else { return model }
        let name = family.prefix(1).uppercased() + family.dropFirst() + " " + parts.dropFirst().joined(separator: ".")
        return long ? name + " · 1M" : name
    }

    /// Whether any of these settings files names an `apiKeyHelper`: only whether it's there, never what it runs.
    static func hasKeyHelper(settings files: [String]) -> Bool {
        files.contains { path in
            guard let data = FileManager.default.contents(atPath: path),
                  let settings = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                  let helper = settings["apiKeyHelper"] as? String else { return false }
            return !helper.trimmingCharacters(in: .whitespaces).isEmpty
        }
    }

    private func follow(_ path: String) -> Tally {
        let follower = followers[path] ?? TranscriptFollower(path: path)
        followers[path] = follower
        follower.update()
        return follower.tally
    }

    private func account(_ path: String, now: Double) -> ClaudeAccount? {
        guard let modified = (try? FileManager.default.attributesOfItem(atPath: path))?[.modificationDate] as? Date else { return nil }
        if let kept = profiles[path], kept.modified == modified {
            // The same file, a little later: a window may have reset meanwhile.
            return kept.account.map { SessionInfo.aged($0, now: now) }
        }
        guard let data = FileManager.default.contents(atPath: path),
              let profile = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return nil }
        let account = ClaudeAccount.from(profile, now: now)
        profiles[path] = (modified, account)
        return account
    }

    /// An account read earlier, as it stands now: use that's gone stale, or a window that's reset, goes.
    static func aged(_ account: ClaudeAccount, now: Double) -> ClaudeAccount {
        var account = account
        if let at = account.usageAt, now - at > ClaudeAccount.usageFresh {
            account.fiveHour = nil
            account.weekly = nil
        }
        if let resets = account.fiveHour?.resetsAt, resets <= now { account.fiveHour = nil }
        if let resets = account.weekly?.resetsAt, resets <= now { account.weekly = nil }
        if account.fiveHour == nil && account.weekly == nil { account.usageAt = nil }
        return account
    }

    private func trim() {
        guard followers.count > SessionInfo.followersMost else { return }
        for (path, _) in followers.sorted(by: { $0.value.lastUsed < $1.value.lastUsed }).prefix(followers.count - SessionInfo.followersMost) {
            followers[path] = nil
        }
    }
}
