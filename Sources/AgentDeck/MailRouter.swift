import Foundation

/// The GM's mail (T401): a message from the GM, or from you on the setup page, to a
/// hire. The core only decides and remembers; how a message reaches a window is
/// the `deliver` step (`MailDelivery`, T402), so tests give a stand-in.
///
/// - Every message that is accepted is written to `outbox.json` (sender, target,
///   time, id, state, size) before anything is delivered.
/// - The same `id` twice is the same message: it is answered again and delivered once.
/// - The text goes below a line that says who sent it, so a hire knows where it
///   came from. The line is the app's; the sender can't change it.
/// - A message for a hire that is busy, asking or has a draft stays `queued` and is looked at on every pump,
///   without counting as an attempt, until the hire is idle, or `Limits.waitHours` pass and it is `left`.
///   Real failures (tmux, a dead session) count: after `maxAttempts` it is `left` too. It is never
///   sent again by itself. One the delivery step says can never work is `failed`.
/// - Nothing is ever put into the GM's own session (it reads `smolcorp gm inbox`),
///   and nothing into a window that is waiting at a question (see `gate`).
/// - Size limits are settings (`Config.mailMaxBytes`, `Config.mailPerHour`).
/// - A cloud bot can be the target too (T404): its message is written as a file in the bot's mailbox folder
///   (`BotMailbox`), not typed; the bot's replies are read from the folder on every pump (`onBotReply`).
///   A bot's `to` in the outbox is `bot:<id>`.
///
/// A hire sends too (T403), as itself: the route knows which hire by its send key
/// (`FrontDesk.hire(forSendKey:)`), never by what it says. A hire may write to the
/// GM, and to another hire only for a pair the GM or the page allowed
/// (`FrontDesk.mayWrite`). It reaches the GM also through its STATUS and
/// NEEDS-REVIEW lines (the watchers, T405). Only used on the main thread.
final class MailRouter {
    enum State: String, Codable {
        case queued, delivered, left, failed
    }

    /// Who a message is from, and so the words of its first line. A hire's line names the
    /// hire (see `Message.fromLine`), so it can't read as the GM's or the owner's.
    enum Sender: String, Codable {
        case gm, owner, hire

        var line: String {
            switch self {
            case .gm: return "the GM"
            case .owner: return "the owner"
            case .hire: return "a hire"
            }
        }
    }

    struct Message: Codable, Equatable {
        var id: String
        var from: Sender
        /// The hire's id, or "gm" for a hire writing to the GM.
        var to: String
        /// What the hire is called, when it was sent.
        var toName: String
        /// The sending hire's id and name, when `from` is a hire. Left out of older outboxes.
        var fromID: String?
        var fromName: String?
        /// What was written, without the sender line.
        var text: String
        /// Milliseconds since 1970, as the deck's other times.
        var at: Double
        var state: State
        /// The text's size in bytes.
        var size: Int
        /// How many times delivery was tried. Counted and saved before each try, so a crash can't make it twice for free.
        var attempts = 0
        /// When delivery is tried next (milliseconds); 0 once it isn't.
        var nextAt: Double = 0
        /// Why it is `left` or `failed`, in words for the GM.
        var note = ""

        /// Who the first line says it is from. A hire is always "hire <name> [<id>]", so a
        /// nickname of "the GM" can't make its mail look like the GM's.
        var fromLine: String {
            guard from == .hire else { return from.line }
            return "hire \(fromName ?? "?") [\(fromID ?? "?")]"
        }
    }

    /// What one try at delivering a message came to.
    enum Delivery: Equatable {
        case delivered
        /// Delivered, but not quite as written (say, a multi-line text sent as one line): the words go in the outbox's note.
        case deliveredAs(String)
        /// The window isn't ready (a question, a draft, at work). Looked at again every pump and NOT an attempt;
        /// it ends only when the hire is free, or after `Limits.waitHours`. The words say why ("waiting: at work").
        case waiting(String)
        /// A real failure that may pass (tmux didn't answer, the session isn't there): counts toward `maxAttempts`.
        case trouble(String)
        /// It can't be delivered at all.
        case failed(String)
    }

    struct Limits: Equatable {
        var maxBytes: Int
        var perHour: Int
        /// How long a message may wait for a free hire before it is `left`.
        var waitHours: Double = MailRouter.defaultWaitHours
    }

    /// Tries to put `text` (the sender line and the message) into the window of `hire`.
    /// `commit` counts the try and writes it down; the step calls it just before the first thing that types
    /// (more than once is fine: it counts once), and types nothing if it answers false.
    typealias Deliver = (_ hire: FrontDesk.Hire, _ text: String, _ commit: () -> Bool) -> Delivery

    static let fileVersion = 1
    static let defaultMaxBytes = 16 * 1024
    static let defaultPerHour = 200
    /// How long a message waits for a busy hire (setting `mailWaitHours`).
    static let defaultWaitHours = 24.0
    /// How often a message with real trouble is tried, and how many tries before it is left.
    static let retryEvery: TimeInterval = 30
    static let maxAttempts = 6
    /// The outbox keeps this many messages, the oldest dropped first.
    static let keepMost = 2000
    static let longestID = 64

    let outboxFile: URL
    private(set) var messages: [Message]
    private let limits: () -> Limits
    private let hires: () -> [FrontDesk.Hire]
    /// Whether one hire may write to another (the registry's contacts).
    private let mayWrite: (_ from: String, _ to: String) -> Bool
    private let bots: () -> [CloudBot]
    /// The folders a bot's mailbox may not be in or hold (the support folder, the hires' worktrees).
    private let barred: () -> [URL]
    private let now: () -> Date
    /// Set when the delivery paths exist (T402). Without it, a message stays queued.
    var deliver: Deliver?
    /// Where `deliver` runs. Nil runs it right there (tests); the app gives a queue of its own, so that
    /// tmux and sockets never hold up the main queue. The answer comes back to the main queue to be recorded.
    var queue: DispatchQueue?
    /// Messages whose delivery is out on `queue`.
    private var inFlight = Set<String>()
    /// Called on the main queue for each reply a bot left in its mailbox, once.
    var onBotReply: (CloudBot, BotMailbox.Reply) -> Void = { _, _ in }
    /// Called once for each new message a hire sends to the GM (never for an answered duplicate, never for
    /// hire-to-hire mail), so the GM's inbox wakes for it. Gets the hire and the message as kept.
    var onHireToGM: (FrontDesk.Hire, Message) -> Void = { _, _ in }
    private var scanning = false

    init(folder: URL, limits: @escaping () -> Limits, hires: @escaping () -> [FrontDesk.Hire],
         mayWrite: @escaping (_ from: String, _ to: String) -> Bool = { _, _ in false },
         bots: @escaping () -> [CloudBot] = { [] }, barred: @escaping () -> [URL] = { [] },
         deliver: Deliver? = nil, now: @escaping () -> Date = Date.init) {
        outboxFile = folder.appendingPathComponent("outbox.json")
        self.limits = limits
        self.hires = hires
        self.mayWrite = mayWrite
        self.bots = bots
        self.barred = barred
        self.deliver = deliver
        self.now = now
        messages = MailRouter.load(outboxFile)
    }

    // MARK: Sending

    /// `POST /setup/gm/send` with `{to, text, id?}`: `to` is a hire's id or its nickname or name.
    /// Without an `id` one is made; give your own so that sending twice delivers once.
    /// `signer` is the hire a message from a hire is from; the caller finds it from the hire's send key.
    func send(from sender: Sender, signer: FrontDesk.Hire? = nil, body: Data) -> Response {
        if sender == .hire && signer == nil { return .status(403, MailRouter.noKey) }
        guard let json = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any],
              let to = json["to"] as? String, !to.trimmingCharacters(in: .whitespaces).isEmpty,
              let text = json["text"] as? String, !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        else { return .status(400, "Send {to, text, id}: who it is for, what to say, and an id of your own so that sending twice delivers once.") }
        var id = ""
        if let given = json["id"] {
            guard let given = given as? String, MailRouter.plain(given) else {
                return .status(400, "The id may use letters, digits, - _ and ., up to \(MailRouter.longestID) of them.")
            }
            id = given
        } else {
            id = "m-" + UUID().uuidString.lowercased().prefix(12)
        }

        // The same id is the same message, whatever became of it: answered again, never sent again.
        if let seen = messages.first(where: { $0.id == id }) {
            // Another sender's id isn't theirs to ask about.
            if sender == .hire && (seen.from != .hire || seen.fromID != signer?.id) { return .status(409, "That id is already used. Pick another.") }
            return .json(MailRouter.answer(seen, duplicate: true))
        }

        let limit = limits()
        let size = text.utf8.count
        if size > limit.maxBytes {
            return .status(413, "That message is \(size) bytes. The limit is \(limit.maxBytes) bytes per message (setting mailMaxBytes).")
        }
        let nowMs = now().timeIntervalSince1970 * 1000
        // A hire's limit is its own, not the hires' together.
        let lastHour = messages.filter { $0.from == sender && $0.fromID == signer?.id && $0.at > nowMs - 3_600_000 }.count
        if lastHour >= limit.perHour {
            let who = signer.map { "hire \($0.nickname.isEmpty ? $0.name : $0.nickname)" } ?? sender.line
            return .status(429, "That is \(limit.perHour) messages from \(who) in the last hour, the limit (setting mailPerHour). Try again later.")
        }

        let target: Target
        if let signer {
            // A hire writes to hires only, never to a bot.
            switch toward(signer, to) {
            case .failure(let refusal): return .status(refusal.status, refusal.message)
            case .success(nil): return toTheGM(signer, id: id, text: text, size: size, at: nowMs)
            case .success(let found?): target = .hire(found)
            }
        } else {
            switch resolve(to) {
            case .failure(let refusal): return .status(refusal.status, refusal.message)
            case .success(let found): target = found
            }
        }
        if case .bot(let bot) = target {
            // A mailbox that is set but not usable (never one that is only not synced yet) is told now, not left to fail.
            if bot.mailbox.isEmpty { return .status(409, "\(bot.name) has no mailbox folder. The owner sets one on the setup page.") }
            if case .failed(let why)? = BotMailbox.refusal(bot.mailbox, barred: barred()) { return .status(409, why) }
        }

        let toID: String, toName: String
        switch target {
        case .hire(let hire): (toID, toName) = (hire.id, hire.nickname.isEmpty ? hire.name : hire.nickname)
        case .bot(let bot): (toID, toName) = (MailRouter.botTarget + bot.id, bot.name)
        }
        var message = Message(id: id, from: sender, to: toID, toName: toName,
                              fromID: signer?.id, fromName: signer.map { $0.nickname.isEmpty ? $0.name : $0.nickname },
                              text: text, at: nowMs, state: .queued, size: size)
        // Written down before anything is delivered; if it can't be, nothing is sent.
        messages.append(message)
        guard save() else {
            messages.removeLast()
            return .status(500, "The outbox couldn't be written, so nothing was sent.")
        }
        switch target {
        case .hire(let hire): attempt(message, hire: hire)
        case .bot(let bot): attempt(message, bot: bot)
        }
        return .json(MailRouter.answer(messages.first(where: { $0.id == id }) ?? message, duplicate: false))
    }

    /// Where a message from `signer` may go: nil for the GM, else the hire, if the pair is allowed.
    /// An unknown name and a name not allowed get the same refusal, so a hire can't use this to
    /// find out who is on the team.
    private func toward(_ signer: FrontDesk.Hire, _ name: String) -> Result<FrontDesk.Hire?, FrontDesk.Refusal> {
        let all = hires()
        let asked = name.trimmingCharacters(in: .whitespaces).lowercased()
        let refusal = Result<FrontDesk.Hire?, FrontDesk.Refusal>.failure(.init(status: 403, message: MailRouter.hireRule))
        if asked == "gm" || asked == "the gm" { return .success(nil) }
        guard case .success(let found) = FrontDesk.find(name, among: all) else { return refusal }
        if found.role == "gm" { return .success(nil) }
        guard found.id != signer.id, found.state != .fired, found.state != .failed, mayWrite(signer.id, found.id) else { return refusal }
        return .success(found)
    }

    /// A hire's message to the GM is kept in the outbox for the GM to read (`gm outbox`); it is never
    /// put into the GM's session (E05), so there is nothing to deliver. It also makes one `mail` event
    /// (see `onHireToGM`), so a GM waiting on `gm inbox --wait` wakes.
    private func toTheGM(_ signer: FrontDesk.Hire, id: String, text: String, size: Int, at: Double) -> Response {
        let name = signer.nickname.isEmpty ? signer.name : signer.nickname
        let message = Message(id: id, from: .hire, to: "gm", toName: "the GM", fromID: signer.id, fromName: name,
                              text: text, at: at, state: .delivered, size: size, note: "Kept for the GM to read; not typed into its session.")
        messages.append(message)
        guard save() else {
            messages.removeLast()
            return .status(500, "The outbox couldn't be written, so nothing was sent.")
        }
        // Kept first, then told: the GM's inbox learns of it as it does of a bot's reply.
        onHireToGM(signer, message)
        return .json(MailRouter.answer(message, duplicate: false))
    }

    /// The GM's inbox line for a hire's message to the GM: who wrote it (its id and name, never its key) and what.
    static func inboxEntry(from hire: FrontDesk.Hire, _ message: Message) -> EventLog.Entry {
        let clean = { (s: String) in String(s.unicodeScalars.map { CharacterSet.controlCharacters.contains($0) ? " " : Character($0) }) }
        let name = clean(message.fromName ?? (hire.nickname.isEmpty ? hire.name : hire.nickname))
        return EventLog.Entry(at: message.at, session: "", folder: hire.folder, agent: "", kind: .mail,
                              detail: "\(name) (hire \(hire.id)) wrote to you: \(clean(message.text))")
    }

    /// What a message is for: a hire's window, or a cloud bot's mailbox.
    private enum Target {
        case hire(FrontDesk.Hire)
        case bot(CloudBot)
    }

    /// The start of a bot's `to` in the outbox.
    static let botTarget = "bot:"

    /// Why a message can't go to `name`, or what it is for.
    private func resolve(_ name: String) -> Result<Target, FrontDesk.Refusal> {
        let asked = name.trimmingCharacters(in: .whitespaces).lowercased()
        if asked == "gm" || asked == "the gm" {
            return .failure(.init(status: 403, message: MailRouter.notTheGM))
        }
        // A bot by its id or its desk's name, or `bot:<id>`; a hire of the same name is a clash.
        let bare = asked.hasPrefix(MailRouter.botTarget) ? String(asked.dropFirst(MailRouter.botTarget.count)) : asked
        let botsNamed = bots().filter { $0.id == bare || (!asked.hasPrefix(MailRouter.botTarget) && $0.name.lowercased() == asked) }
        switch FrontDesk.find(name, among: hires()) {
        case .failure(let refusal):
            if refusal.status == 404 {
                if botsNamed.count == 1 { return .success(.bot(botsNamed[0])) }
                if botsNamed.isEmpty { return .failure(.init(status: 404, message: "There's no hire or bot called that. Look at the team for ids and names.")) }
                return .failure(.init(status: 409, message: "More than one hire or bot is called that. Use the id (bot:<id> for a bot)."))
            }
            return .failure(refusal)
        case .success(let hire):
            if !botsNamed.isEmpty { return .failure(.init(status: 409, message: "More than one hire or bot is called that. Use the id (bot:<id> for a bot).")) }
            if hire.role == "gm" { return .failure(.init(status: 403, message: MailRouter.notTheGM)) }
            if hire.state == .fired || hire.state == .failed {
                return .failure(.init(status: 409, message: "\(hire.name) is already gone."))
            }
            return .success(.hire(hire))
        }
    }

    static let hireRule = "A hire may write to the GM, and to another hire only when the GM or the owner has allowed that pair. "
        + "Ask the GM to allow it (smolcorp gm contacts)."
    static let noKey = "This message isn't from a hire Smolcorp started: a hire sends with the key in its own environment."
    static let notTheGM = "Mail is never put into the GM's own session. The GM reads its mail with smolcorp gm inbox."

    // MARK: Delivering

    /// The text a hire is given: the sender line, a blank line, the message.
    static func wire(_ message: Message) -> String {
        "— from \(message.fromLine) (Smolcorp), message \(message.id)\n\n\(message.text)"
    }

    /// The deck's half of the gate alone, for a path that has no window to look at (the inbox socket).
    static func deckGate(deckAsking: Bool) -> Delivery? {
        deckAsking ? .waiting("waiting: at a question") : nil
    }

    /// Whether a window can be written into now, from what it shows: nil when it can, else why it waits.
    /// Only an idle, empty input box is written into; a question or a menu never is. For the delivery
    /// paths (T402), which look again just before typing, and pass whether the deck itself knows the
    /// session is asking (its `.asking` state, read just then): either one keeps the message waiting.
    static func gate(_ capture: String?, deckAsking: Bool) -> Delivery? {
        if let wait = deckGate(deckAsking: deckAsking) { return wait }
        if HireDesk.promptRefusal(capture) != nil { return .waiting("waiting: at a question") }
        switch HireDesk.inputState(capture) {
        case .idleAndEmpty: return nil
        case .busy: return .waiting("waiting: at work")
        case .draft: return .waiting("waiting: a draft is in its box")
        case .unknown: return .waiting("waiting: its input box couldn't be told apart")
        }
    }

    /// Whether a try has been counted yet. Set on the main queue only, by `commit`.
    private final class Counted { var done = false }

    /// One try, if the delivery step is there. The try is counted and written down by `commit`, just before
    /// the first thing is typed, so that a crash between the paste and the save can't paste the same message
    /// again for free. A try that comes to waiting never typed, so it is never counted or written.
    private func attempt(_ message: Message, hire: FrontDesk.Hire) {
        guard let deliver else { return }
        run(message) { text, commit in deliver(hire, text, commit) }
    }

    /// The same for a bot: the message becomes a file in its mailbox folder, whatever `deliver` is.
    private func attempt(_ message: Message, bot: CloudBot) {
        let (mailbox, folders, sentAt) = (bot.mailbox, barred(), Date(timeIntervalSince1970: message.at / 1000))
        run(message) { text, commit in
            mailbox.isEmpty ? .failed("\(bot.name) has no mailbox folder any more.")
                : BotMailbox.drop(text, id: message.id, at: sentAt, mailbox: mailbox, barred: folders, commit: commit)
        }
    }

    private func run(_ message: Message, _ deliver: @escaping (_ text: String, _ commit: () -> Bool) -> Delivery) {
        guard message.state == .queued, !inFlight.contains(message.id) else { return }
        let id = message.id
        let text = MailRouter.wire(message)
        let counted = Counted()
        // If the count can't be written, answer false: nothing is typed, as it would not be safe to.
        let commit: () -> Bool = { [weak self] in
            let count = {
                guard let self, var current = self.messages.first(where: { $0.id == id }) else { return false }
                if counted.done { return true }
                current.attempts += 1
                current.nextAt = self.now().timeIntervalSince1970 * 1000 + MailRouter.retryEvery * 1000
                guard self.update(current) else { return false }
                counted.done = true
                return true
            }
            return Thread.isMainThread ? count() : DispatchQueue.main.sync(execute: count)
        }
        guard let queue else {
            finish(id, deliver(text, commit), counted: counted)
            return
        }
        inFlight.insert(id)
        queue.async { [weak self] in
            let result = deliver(text, commit)
            DispatchQueue.main.async { self?.finish(id, result, counted: counted) }
        }
    }

    /// What a try came to, recorded. On the main queue.
    private func finish(_ id: String, _ result: Delivery, counted: Counted) {
        inFlight.remove(id)
        guard var message = messages.first(where: { $0.id == id }) else { return }
        MailRouter.settle(&message, result, at: now().timeIntervalSince1970 * 1000, waitHours: limits().waitHours, counted: counted.done)
        update(message)
    }

    static func settle(_ message: inout Message, _ result: Delivery, at current: Double, waitHours: Double = defaultWaitHours, counted: Bool = true) {
        switch result {
        case .delivered:
            message.state = .delivered
            message.nextAt = 0
            message.note = ""
        case .deliveredAs(let how):
            message.state = .delivered
            message.nextAt = 0
            message.note = how
        case .failed(let why):
            message.state = .failed
            message.nextAt = 0
            message.note = why
        case .waiting(let why):
            // Nothing was typed and nothing counted: waiting is not an attempt.
            if current - message.at >= waitHours * 3_600_000 {
                message.state = .left
                message.nextAt = 0
                message.note = "Left after waiting \(String(format: "%g", waitHours)) hours for \(message.toName) to be free, not sent again: \(why)"
            } else {
                message.nextAt = 0      // looked at again on the next pump
                message.note = why
            }
        case .trouble(let why):
            // A failure before anything was typed was not counted by `commit`; it counts now.
            if !counted { message.attempts += 1 }
            if message.attempts >= MailRouter.maxAttempts {
                message.state = .left
                message.nextAt = 0
                message.note = "Left after \(message.attempts) tries, not sent again: \(why)"
            } else {
                message.nextAt = current + MailRouter.retryEvery * 1000
                message.note = why
            }
        }
    }

    /// Tries the queued messages that are due (a waiting one on every call). Called every few seconds by the deck's timer (T402).
    func pump() {
        let current = now().timeIntervalSince1970 * 1000
        let open = hires()
        for message in messages where message.state == .queued && message.nextAt <= current {
            if message.to.hasPrefix(MailRouter.botTarget) {
                if let bot = bots().first(where: { MailRouter.botTarget + $0.id == message.to }) {
                    attempt(message, bot: bot)
                } else if !inFlight.contains(message.id) {
                    var gone = message
                    gone.state = .failed
                    gone.nextAt = 0
                    gone.note = "\(message.toName) is gone."
                    update(gone)
                }
            } else if let hire = open.first(where: { $0.id == message.to && $0.state != .fired && $0.state != .failed }) {
                attempt(message, hire: hire)      // saves only what changed, in `commit` and `finish`
            } else if inFlight.contains(message.id) {
                continue        // its delivery is out; what it comes to is recorded then
            } else {
                var gone = message
                gone.state = .failed
                gone.nextAt = 0
                gone.note = "\(message.toName) is gone."
                update(gone)
            }
        }
        collectReplies()
    }

    /// Looks in each bot's `from-bot` folder for replies, and hands each new one to `onBotReply`. Called by `pump`.
    /// On `queue` when there is one (a synced folder can be slow); a look still out is not started again.
    private func collectReplies() {
        guard !scanning else { return }
        let withMailbox = bots().filter { !$0.mailbox.isEmpty }
        guard !withMailbox.isEmpty else { return }
        let (folders, most, current) = (barred(), limits().maxBytes, now())
        let look = { withMailbox.map { ($0, BotMailbox.collect(mailbox: $0.mailbox, barred: folders, maxBytes: most, now: current)) } }
        guard let queue else {
            for (bot, replies) in look() { replies.forEach { onBotReply(bot, $0) } }
            return
        }
        scanning = true
        queue.async { [weak self] in
            let found = look()
            DispatchQueue.main.async {
                guard let self else { return }
                self.scanning = false
                for (bot, replies) in found { replies.forEach { self.onBotReply(bot, $0) } }
            }
        }
    }

    @discardableResult
    private func update(_ message: Message) -> Bool {
        guard let i = messages.firstIndex(where: { $0.id == message.id }) else { return false }
        guard messages[i] != message else { return true }      // nothing changed: nothing written
        messages[i] = message
        return save()
    }

    // MARK: Answers

    private static func plain(_ id: String) -> Bool {
        !id.isEmpty && id.count <= longestID && id.allSatisfy { $0.isASCII && ($0.isLetter || $0.isNumber || "-_.".contains($0)) }
    }

    /// What `POST /setup/gm/send` answers.
    static func answer(_ message: Message, duplicate: Bool) -> [String: Any] {
        var out: [String: Any] = ["id": message.id, "state": message.state.rawValue, "to": message.to, "size": message.size]
        if duplicate { out["duplicate"] = true }
        if !message.note.isEmpty { out["note"] = message.note }
        return out
    }

    /// `GET /setup/gm/outbox`, the latest `limit` messages, oldest first. A message's text is kept
    /// for the GM and the page; a tool of yours (that can read the setup-key file, as a hire can) isn't given it.
    func outboxJSON(limit: Int = 100, includeText: Bool) -> [String: Any] {
        let shown = messages.suffix(max(1, min(limit, MailRouter.keepMost))).map { message -> [String: Any] in
            var out: [String: Any] = ["id": message.id, "from": message.from.rawValue, "to": message.to, "toName": message.toName,
                                      "at": Int64(message.at.rounded(.down)), "state": message.state.rawValue, "size": message.size, "attempts": message.attempts]
            if !message.note.isEmpty { out["note"] = message.note }
            if let id = message.fromID { out["fromID"] = id }
            if let name = message.fromName { out["fromName"] = name }
            if includeText { out["text"] = message.text }
            return out
        }
        return ["messages": shown]
    }

    // MARK: The file

    @discardableResult
    private func save() -> Bool {
        if messages.count > MailRouter.keepMost { messages.removeFirst(messages.count - MailRouter.keepMost) }
        struct File: Codable { var version: Int; var messages: [Message] }
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        guard let data = try? encoder.encode(File(version: MailRouter.fileVersion, messages: messages)) else { return false }
        return Paths.writePrivately(data, to: outboxFile)
    }

    private static func load(_ file: URL) -> [Message] {
        struct File: Codable { var version: Int?; var messages: [Message] }
        guard let data = try? Data(contentsOf: file) else { return [] }
        guard let found = try? JSONDecoder().decode(File.self, from: data) else {
            // Set aside, as the registry's is, so the next save doesn't wipe what might be mended.
            try? FileManager.default.moveItem(at: file, to: file.deletingLastPathComponent().appendingPathComponent("outbox.broken.json"))
            return []
        }
        return found.messages
    }
}
