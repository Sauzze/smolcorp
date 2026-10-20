import AppKit

/// The deck itself: the sessions, how they sit on the six keys, what the
/// phone's controls do to them, and the live feed of all that to phones.
final class Deck {
    static let port: UInt16 = TestCopy.port ?? 7710
    /// The setup page, with its key after #k=, which never leaves this Mac:
    /// the page hands it over once for a cookie, and takes it out of the
    /// address. `then` is what the page does on opening: "pair" or "unpair".
    func setupURL(then: String? = nil) -> URL {
        URL(string: "http://localhost:\(Deck.port)/setup#k=\(access.setupKey)" + (then.map { "&\($0)" } ?? ""))!
    }

    /// How often to look for a green key's tab coming to the front.
    private static let seenCheckInterval: TimeInterval = 1.5
    /// How often to look for tabs that closed without their session ending.
    private static let closedTabCheckInterval: TimeInterval = 10
    /// How often to ping event streams: often enough that an idle one stays
    /// open, as a browser may drop one that's quiet for a minute, and that
    /// the page can tell a stream that died quietly.
    private static let pingInterval: TimeInterval = 40
    /// How often to look whether music is playing, with the jukebox on.
    private static let musicInterval: TimeInterval = 3
    /// How soon after a change the sessions are saved, so a burst of events
    /// writes once.
    private static let saveDelay: TimeInterval = 1
    /// Sessions saved this recently before a restart are shown as they were;
    /// any older, a turn may have ended while nobody was listening.
    private static let freshSave: Double = 120_000
    /// Claude stops listening by itself after two minutes.
    private static let voiceLimit: TimeInterval = 120
    /// Two Returns to one session closer than this are one tap landing twice.
    private static let returnRepeat: TimeInterval = 0.5
    /// Without Accessibility, posted keys vanish without a trace. The phone
    /// says so instead, since every update turns the permission off.
    private static let noKeyboard = "Keyboard access is off. Turn it on in setup, on your Mac."

    let access = Access()
    let sessions = Sessions()
    /// The key the office's own window on this Mac reads the office with:
    /// new at each start, never saved, and taken only from this Mac.
    let windowKey = randomKey()
    /// Called on every change, for the menu bar.
    var onChange: () -> Void = {}
    /// The portable office (beta): the office on a phone from anywhere, view only. Off at every start.
    private(set) lazy var portable = PortableOffice { [unowned self] hideNames, alias in
        self.viewerSnapshot(hideNames: hideNames, alias: alias)
    }

    /// Each open stream, with the phone that opened it, so unpairing a phone
    /// can hang up on it at once.
    private var streams: [ObjectIdentifier: (phone: String, stream: EventStream)] = [:]
    /// The key each session was given, so it keeps it.
    private var keyOf: [String: Int] = [:]
    /// The role each session was hired for, by the role's id.
    private var roleOf: [String: String] = [:]
    /// The tabs hires opened, waiting for their agent to report: the first
    /// session in the tab takes the role it was hired for, if any, and the
    /// desk it was hired at, with a set number of desks.
    private var hiredTabs: [(tab: String, role: String?, desk: Int?, at: Date, folder: String)] = []
    /// The tab recording started in, and when. One Mac, one microphone.
    private var micPress: (tab: String, at: Date)?
    /// When each session was last sent Return from the phone, so a tap that
    /// lands twice sends it once.
    private var lastReturn: [String: Date] = [:]
    private var checkingFront = false
    private var checkingTabs = false
    /// Ttys Terminal reported last time. Only these can be found closed:
    /// sessions in other terminal apps never appear here and are left alone.
    private var terminalTTYs = Set<String>()
    /// Ttys missing from Terminal's last look. Missing again at the next, with
    /// no agent left running on them, their tabs have closed: one look alone
    /// can catch Terminal in the middle of opening or closing a window.
    private var missedOnce = Set<String>()
    /// Tabs of other terminals found gone at the last look: gone again at the
    /// next, their sessions go too.
    private var panesMissedOnce = Set<String>()
    private var checkingPanes = false
    private var savePending = false
    /// Whether music is playing, for the office's jukeboxes: true, or nil when it can't be told.
    private var playing: Bool?
    private var checkingMusic = false
    /// The Mac's output volume and mute, for the jukebox's volume bar: nil
    /// where the output has no volume to set. Kept by CoreAudio's listeners.
    private var volume: (level: Int, muted: Bool)?
    private var watchingVolume = false

    init() {
        sessions.onChange = { [weak self] in self?.changed() }
        restoreSessions()
        // What's running as it starts is where the webhook starts from, so the first change after it is news
        tellWebhook()

        Timer.scheduledTimer(withTimeInterval: Deck.seenCheckInterval, repeats: true) { [weak self] _ in
            self?.checkFrontTab()
            self?.sessions.endHandOns(now: Date().timeIntervalSince1970 * 1000)
        }
        Timer.scheduledTimer(withTimeInterval: Deck.closedTabCheckInterval, repeats: true) { [weak self] _ in
            self?.pruneClosedTabs()
            self?.pruneClosedPanes()
            // Agents that exited unheard, and headless runs long quiet.
            self?.sessions.prune(now: Date().timeIntervalSince1970 * 1000)
        }
        Timer.scheduledTimer(withTimeInterval: Deck.pingInterval, repeats: true) { [weak self] _ in
            self?.streams.values.forEach { $0.stream.ping() }
        }
        Timer.scheduledTimer(withTimeInterval: Deck.musicInterval, repeats: true) { [weak self] _ in
            self?.followMusic()
        }
    }

    // MARK: Addresses

    /// What a phone on the same network opens to pair.
    var phoneAddress: String { "http://\(LocalNetwork.address() ?? "localhost"):\(Deck.port)" }

    /// What the QR code opens: the pairing page with `code` filled in.
    func pairingLink(for code: String) -> String { "\(phoneAddress)/?pair=\(code)" }

    // MARK: Hook events

    /// `reported` is the agent the hook script was set up for; see
    /// `Agent.of(_:reportedAs:)` for why that isn't always the sender.
    /// `tab` is the tab the hook found, as a `TabAddress`; `tty` the tty it
    /// ran on, which a hook from before 1.9 sends alone.
    /// `pid` is the agent's process, which a headless run's hook sends, and
    /// `parent` the folder it was launched from, where its launcher said.
    func ingest(_ event: [String: Any], reportedAs reported: String, tab: String, tty: String, startedIn: String,
                pid: Int32? = nil, parent: String = "", at time: Double) {
        let agent = Agent.of(event, reportedAs: reported)
        guard let event = agent.canonical(event) else { return }
        let found = !tab.isEmpty ? tab : !tty.isEmpty ? TabAddress.terminalApp(tty: tty).text : self.tab(for: event, of: agent)
        sessions.apply(event, agent: agent, tab: found, startedIn: startedIn, pid: pid, parent: parent, at: time)
        learnGhostty(found)
        noteMessage(event, from: agent)
    }

    /// Which Ghostty terminal a session is in, where Ghostty can say (1.4 and
    /// later): see `Ghostty`. Its desk gets its controls once known; on Ghostty
    /// 1.3 it stays watched.
    private var learningGhostty: [String: Date] = [:]
    private func learnGhostty(_ tab: String) {
        guard let address = TabAddress(tab), address.kind == "ghostty", Ghostty.id(on: address.id) == nil else { return }
        // At most every few seconds: each try asks Ghostty.
        if let last = learningGhostty[address.id], Date().timeIntervalSince(last) < 5 { return }
        learningGhostty[address.id] = Date()
        Terminal.queue.async {
            Ghostty.learn(tty: address.id)
            if Ghostty.id(on: address.id) != nil { DispatchQueue.main.async { self.changed() } }
        }
    }

    /// A Claude Code session sending another a message, as its SendMessage
    /// tool is about to: the sender is marked with when and to whom, for the
    /// office to walk it over. A recipient that isn't at a desk, or can't be
    /// told apart, is left out. It changes nothing else.
    private func noteMessage(_ event: [String: Any], from agent: Agent) {
        guard agent == .claude, event["hook_event_name"] as? String == "PreToolUse", event["tool_name"] as? String == "SendMessage",
              let id = event["session_id"] as? String, let sender = sessions[id],
              let to = (event["tool_input"] as? [String: Any])?["to"] as? String,
              let recipient = ClaudePeers.sessionID(for: to), recipient != id, sessions[recipient] != nil else { return }
        sender.messagedAt = Date().timeIntervalSince1970 * 1000
        sender.messaged = recipient
        changed()
    }

    /// Codex runs its hooks in a shared background service, so its events
    /// arrive with no tty. Its tab is the one running Codex in the session's
    /// folder, when exactly one does and no other session holds it already.
    /// A session the deck knows keeps the tab it was found in.
    private func tab(for event: [String: Any], of agent: Agent) -> String {
        guard let id = event["session_id"] as? String, sessions[id] == nil,
              let name = agent.processName, let folder = event["cwd"] as? String else { return "" }
        let taken = Set(sessions.all.compactMap { Terminals.ttyInAddress(of: $0.tab) })
        guard let tty = Processes.tty(of: name, in: folder, excluding: taken) else { return "" }
        // A Terminal.app tab only if Terminal listed it at its last look; any
        // other is watched until it does (see `pruneClosedTabs`).
        return terminalTTYs.contains(tty) ? TabAddress.terminalApp(tty: tty).text : "other:\(tty)"
    }

    private func changed() {
        applyQueuedEffort()
        let state = snapshotJSON()
        streams.values.forEach { $0.stream.send(state) }
        onChange()
        scheduleSave()
        tellWebhook()
        portable.changed()
    }

    // MARK: Webhook

    /// What each session was when the webhook was last told, to tell it
    /// what changed: nil until the deck first looks, so what's already
    /// running when it starts isn't news.
    private var told: [String: Told]?
    private typealias Told = (state: SessionState, project: String, agent: Agent, key: Int?, kind: String)
    /// The webhook's messages waiting their turn, sent one after another so
    /// a quick "working" then "done" can't arrive the other way round.
    private var webhookQueue: [(webhook: Webhook, message: [String: Any])] = []
    private var webhookSending = false
    /// How many messages wait at most, if the address stops answering.
    private static let webhookQueueMost = 64
    /// How the last delivery went, for the setup page: what came back, and when.
    private(set) var webhookLast: String?

    /// The webhook's event for a session going from `before` to `after`, if it has one.
    private static func event(from before: SessionState?, to after: SessionState) -> String? {
        switch after {
        case .asking: return before == .asking ? nil : "waiting"
        case .unread: return before == .unread ? nil : "done"
        case .error: return before == .error ? nil : "error"
        case .thinking: return before == .thinking ? nil : "working"
        case .idle: return nil
        }
    }

    /// Tells the webhook what has happened since it was last told: sessions
    /// started, ended, or now waiting on you, done, failed or at work.
    private func tellWebhook() {
        let (keys, _) = layout()
        var keyOf: [String: Int] = [:]
        for (index, key) in keys.enumerated() { if let session = key.session { keyOf[session.id] = index + 1 } }
        let now: [String: Told] = Dictionary(uniqueKeysWithValues: sessions.all.map {
            ($0.id, (state: $0.state, project: $0.projectName, agent: $0.agent, key: keyOf[$0.id], kind: $0.errorKind))
        })
        defer { told = now }
        let webhook = Config.shared.webhook
        guard let before = told, webhook.enabled, !webhook.url.isEmpty, Webhook.problem(with: webhook.url, outside: webhook.outside) == nil else { return }
        var messages: [[String: Any]] = []
        let message = { (event: String, now: Told, previous: SessionState?) -> [String: Any] in
            var session: [String: Any] = [
                "project": now.project, "agent": now.agent.rawValue, "agentName": now.agent.name, "state": now.state.rawValue,
                "previous": previous?.rawValue ?? NSNull(), "key": now.key ?? NSNull(),
            ]
            if now.state == .error, !now.kind.isEmpty { session["errorKind"] = now.kind }
            return ["event": event, "at": ISO8601DateFormatter().string(from: Date()), "session": session]
        }
        // In the order of the desks, then of their ids: one change's messages
        // always go in the same order, and a new session's start before its state
        let inOrder = { (sessions: [String: Told]) in
            sessions.keys.sorted { (sessions[$0]!.key ?? 0, $0) < (sessions[$1]!.key ?? 0, $1) }
        }
        for id in inOrder(now) {
            let session = now[id]!
            if let old = before[id] {
                if let event = Deck.event(from: old.state, to: session.state) { messages.append(message(event, session, old.state)) }
            } else {
                messages.append(message("started", session, nil))
                if let event = Deck.event(from: nil, to: session.state), event != "working" { messages.append(message(event, session, nil)) }
            }
        }
        for id in inOrder(before) where now[id] == nil { messages.append(message("ended", before[id]!, before[id]!.state)) }
        for message in messages where webhook.events.contains(message["event"] as? String ?? "") {
            guard webhookQueue.count < Deck.webhookQueueMost else { break } // the address has stopped answering
            webhookQueue.append((webhook, message))
        }
        sendNextWebhook()
    }

    /// Sends the next message waiting, once the last has gone.
    private func sendNextWebhook() {
        guard !webhookSending, !webhookQueue.isEmpty else { return }
        let (webhook, message) = webhookQueue.removeFirst()
        webhookSending = true
        webhook.send(message) { [weak self] result in
            self?.webhookLast = Deck.described(result)
            self?.webhookSending = false
            self?.sendNextWebhook()
        }
    }

    /// A test message to the webhook as it's set up, saying how it went.
    func testWebhook(done: @escaping () -> Void) {
        let webhook = Config.shared.webhook
        if let problem = Webhook.problem(with: webhook.url, outside: webhook.outside) {
            webhookLast = problem
            return done()
        }
        let message: [String: Any] = ["event": "test", "at": ISO8601DateFormatter().string(from: Date()), "session": NSNull()]
        webhook.send(message) { [weak self] result in
            self?.webhookLast = Deck.described(result)
            done()
        }
    }

    /// How a delivery went, for the setup page: "Sent: 204 at 20:41", or what went wrong.
    private static func described(_ result: String) -> String {
        let time = DateFormatter.localizedString(from: Date(), dateStyle: .none, timeStyle: .short)
        guard let status = Int(result) else { return "\(result) (\(time))" }
        return (200..<300).contains(status) ? "Sent: \(status) at \(time)" : "It answered \(status) at \(time)"
    }

    // MARK: Across a restart

    /// What's written to keep the deck across a restart: the sessions, their
    /// keys, and when.
    private struct SavedDeck: Codable {
        let savedAt: Double
        let sessions: [Sessions.Saved]
        let keys: [String: Int]
        /// Missing from a file written before roles.
        var roles: [String: String]?
    }

    private func scheduleSave() {
        guard !savePending else { return }
        savePending = true
        DispatchQueue.main.asyncAfter(deadline: .now() + Deck.saveDelay) { [weak self] in
            guard let self else { return }
            self.savePending = false
            let saved = SavedDeck(savedAt: Date().timeIntervalSince1970 * 1000, sessions: self.sessions.saved, keys: self.keyOf,
                                  roles: self.roleOf)
            guard let data = try? JSONEncoder().encode(saved) else { return }
            Paths.writePrivately(data, to: Paths.sessions)
        }
    }

    /// The sessions running when the app last stopped, back on their keys:
    /// those whose agent is still running in its tab, and the headless runs
    /// upstairs whose process is still there. Sessions only report as
    /// something happens, so an idle one would otherwise stay away until it's
    /// next used.
    private func restoreSessions() {
        guard let data = try? Data(contentsOf: Paths.sessions),
              let saved = try? JSONDecoder().decode(SavedDeck.self, from: data), !saved.sessions.isEmpty else { return }
        DispatchQueue.global(qos: .utility).async {
            let running = saved.sessions.filter { Deck.isStillRunning($0) }
            DispatchQueue.main.async {
                guard !running.isEmpty else { return }
                for entry in running where self.keyOf[entry.id] == nil {
                    if let key = saved.keys[entry.id] { self.keyOf[entry.id] = key }
                }
                for entry in running where self.roleOf[entry.id] == nil {
                    if let role = saved.roles?[entry.id] { self.roleOf[entry.id] = role }
                }
                let fresh = Date().timeIntervalSince1970 * 1000 - saved.savedAt < Deck.freshSave
                self.sessions.restore(running, fresh: fresh)
            }
        }
    }

    /// Whether a session saved before a restart is still running: its agent in
    /// its tab, or for a headless run, which has none, its process.
    static func isStillRunning(_ entry: Sessions.Saved, isGone: (Int32) -> Bool = Sessions.processGone) -> Bool {
        if TabAddress.isHeadless(entry.address) { return entry.pid.map { !isGone($0) } ?? false }
        guard let tty = Terminals.tty(of: entry.address), let agent = Agent(rawValue: entry.agent) else { return false }
        return Terminal.isRunning(agent, on: tty) == true
    }

    // MARK: Keys

    struct Key {
        let pin: String?
        var session: Session?
    }

    /// A pinned key shows its project's session when one is running and stays
    /// reserved when none is. Every other session gets a free key. With a fixed
    /// number of keys, a session keeps its key until it ends, so keys don't
    /// shuffle as sessions come and go. With Auto, the keys grow and shrink with
    /// the sessions, and those still running keep their order when one ends.
    /// Sessions beyond the last key have none until one frees up. Headless
    /// runs have no keys: they sit upstairs (see `upstairs()`).
    func layout() -> (keys: [Key], overflow: [Session]) {
        let config = Config.shared
        var remaining = Deck.downstairs(sessions.all)
        let live = Set(remaining.map(\.id))
        // A session that took another's place in its tab, as after /clear, takes its key and role too.
        for (new, old) in sessions.takeReplacements() {
            if keyOf[new] == nil, let key = keyOf[old] { keyOf[new] = key }
            if roleOf[new] == nil, let role = roleOf[old] { roleOf[new] = role }
        }
        // A hire's role and desk go to the first session in the tab it opened.
        // The desk is kept for it below if it's still free; if it was taken in
        // the meantime, the hire sits at the first free one, as anyone would.
        hiredTabs.removeAll { Date().timeIntervalSince($0.at) > Deck.hiredTabWait }
        for session in remaining where !session.tab.isEmpty {
            // A Ghostty hire is known by its terminal, its tty only once its session reports.
            let ghostty = TabAddress(session.tab).flatMap { $0.kind == "ghostty" ? Ghostty.id(on: $0.id) : nil }.map { "ghostty-id:\($0)" }
            guard let i = hiredTabs.firstIndex(where: { $0.tab == session.tab || $0.tab == ghostty }) else { continue }
            let hired = hiredTabs.remove(at: i)
            if let role = hired.role, roleOf[session.id] == nil { roleOf[session.id] = role }
            if let desk = hired.desk { keyOf[session.id] = desk }
        }
        // A session dropped as its tab seemed to close keeps its key and role, should it come back.
        keyOf = keyOf.filter { live.contains($0.key) || sessions.wasDropped($0.key) }
        roleOf = roleOf.filter { live.contains($0.key) || sessions.wasDropped($0.key) }

        var fixed: Int?
        if case .fixed(let count) = config.layout.keys { fixed = count }
        var keys = config.pins.prefix(fixed ?? KeyCount.most).map { Key(pin: $0, session: nil) }

        func place(_ session: Session, at index: Int) {
            keys[index].session = session
            keyOf[session.id] = index
            remaining.removeAll { $0.id == session.id }
        }

        for i in keys.indices {
            guard let pin = keys[i].pin else { continue }
            let candidates = remaining.filter { $0.project == pin }
            if let match = candidates.first(where: { keyOf[$0.id] == i }) ?? candidates.first { place(match, at: i) }
        }
        if fixed != nil {
            for session in remaining {
                if let i = keyOf[session.id], i < keys.count, keys[i].pin == nil, keys[i].session == nil { place(session, at: i) }
            }
        } else {
            // Every pinned key, and a free one for each other session.
            let pinned = keys.filter { $0.pin != nil }.count
            let last = (keys.lastIndex { $0.pin != nil } ?? -1) + 1
            keys = Array(keys.prefix(min(max(1, last, pinned + remaining.count), KeyCount.most)))
            // In the order their keys had, so the grid shrinks without shuffling.
            let previous = { (session: Session) in self.keyOf[session.id] ?? KeyCount.most }
            remaining = remaining.enumerated()
                .sorted { (previous($0.element), $0.offset) < (previous($1.element), $1.offset) }
                .map(\.element)
        }
        for session in remaining {
            guard let free = keys.indices.first(where: { keys[$0].pin == nil && keys[$0].session == nil }) else { break }
            place(session, at: free)
        }
        // A session left without a key forgets the one it had, or it would
        // claim it back from the session that has it now.
        for session in remaining { keyOf[session.id] = nil }
        return (keys, remaining)
    }

    /// What the phone is told of a session at a desk.
    private func sessionEntry(_ s: Session) -> [String: Any] {
        // What its terminal lets the deck do: one Smolcorp can only watch shows the desk, not the controls.
        let can = Terminals.abilities(of: s.tab)
        return [
            "id": s.id, "name": s.name, "project": s.projectName, "state": s.state.rawValue,
            "effort": s.pendingEffort.isEmpty ? s.effort : s.pendingEffort,
            "pending": !s.pendingEffort.isEmpty,
            "agent": s.agent.rawValue, "agentName": s.agent.name,
            "dial": s.agent.hasEffortDial && can.contains(.text),
            // Claude Code's own voice mode through Space, or dictation through the text it types.
            "mic": Deck.dictates(s.agent) ? can.contains(.text) : s.agent.hasVoice && can.contains(.keys),
            "dictates": Deck.dictates(s.agent) && can.contains(.text),
            "keys": can.contains(.keys), "fire": can.contains(.fire),
            "activity": s.activity ?? NSNull(),
            "tool": s.tool.isEmpty ? NSNull() : s.tool,
            "model": s.model.isEmpty ? NSNull() : s.model,
            "subagents": s.subagents.map { subagent -> [String: Any] in
                ["id": subagent.id, "type": subagent.type, "model": subagent.model.isEmpty ? NSNull() : subagent.model]
            },
            "maxPressedAt": s.maxPressedAt.map { Int64($0) } ?? NSNull(),
            "messagedAt": s.messagedAt.map { Int64($0) } ?? NSNull(),
            "messaged": s.messaged.isEmpty ? NSNull() : s.messaged,
            "compacting": s.compacting,
            "turnStartedAt": s.turnStartedAt.map { Int64($0) } ?? NSNull(),
            "errorKind": s.errorKind.isEmpty ? NSNull() : s.errorKind,
            "role": roleOf[s.id].flatMap { id in Config.shared.roles.first { $0.id == id } }?.forPhone ?? NSNull(),
        ]
    }

    /// A headless run's desk upstairs: what it's doing, with every control
    /// off, as there's nothing to type into.
    private func upstairsEntry(_ s: Session) -> [String: Any] {
        var entry = sessionEntry(s)
        for control in ["keys", "fire", "mic", "dial", "dictates"] { entry[control] = false }
        entry["role"] = NSNull()
        entry["headless"] = true
        entry["startedAt"] = Int64(s.startedAt)
        entry["hiredBy"] = Deck.hiredBy(s, among: sessions.all).map { parent -> [String: Any] in
            ["id": parent.session?.id ?? NSNull(), "name": parent.name]
        } ?? NSNull()
        return entry
    }

    /// Who started a headless run, for its card upstairs: the ground-floor
    /// session working in the folder it was launched from, the oldest, under
    /// its project's name; else that folder's name, with no session; else
    /// nil, when its launcher didn't say. Only the name leaves the Mac.
    static func hiredBy(_ s: Session, among all: [Session]) -> (session: Session?, name: String)? {
        let trimmed = { (path: String) in path.hasSuffix("/") ? String(path.dropLast()) : path }
        let parent = trimmed(s.parentFolder)
        guard !parent.isEmpty else { return nil }
        if let hirer = downstairs(all).first(where: { trimmed($0.project) == parent || trimmed($0.folder) == parent }) {
            return (hirer, hirer.projectName)
        }
        return (nil, Session.lastPart(of: parent))
    }

    /// As many desks as the second floor has.
    static let upstairsDesks = 12

    /// The second floor: a desk for each headless run, oldest first, up to
    /// `upstairsDesks`, and how many more there are. They never have keys.
    func upstairs() -> (desks: [Session], overflow: Int) { Deck.upstairs(of: sessions.all) }

    static func upstairs(of all: [Session]) -> (desks: [Session], overflow: Int) {
        let headless = all.filter(\.isHeadless)
        return (Array(headless.prefix(upstairsDesks)), max(0, headless.count - upstairsDesks))
    }

    /// The sessions that may have keys: every one but the headless runs.
    static func downstairs(_ all: [Session]) -> [Session] { all.filter { !$0.isHeadless } }

    /// What the phone draws: the keys, the theme and the layout. No ttys or paths.
    func snapshotJSON() -> Data {
        let (keys, overflow) = layout()
        let upstairs = self.upstairs()
        let canHire = Config.shared.hiringEnabled && Deck.folder(Config.shared.hireFolder) != nil
        let state: [String: Any] = [
            "keys": keys.map { key -> [String: Any] in
                var entry: [String: Any] = ["pin": key.pin.map { ($0 as NSString).lastPathComponent } ?? NSNull()]
                if let s = key.session { entry["session"] = sessionEntry(s) }
                return entry
            },
            "overflow": overflow.count,
            // The second floor: headless runs, at desks of their own.
            "upstairs": ["desks": upstairs.desks.map(upstairsEntry), "overflow": upstairs.overflow] as [String: Any],
            // The Mac's clock, so the office can count turns on the phone's,
            // whichever of the two is right.
            "now": Int64(Date().timeIntervalSince1970 * 1000),
            "theme": Theme.current(),
            "scroll": Config.shared.scrollEnabled,
            // Dictation's language, for the mic's quick switch.
            "dictation": ["on": Dictation.isOn, "language": Config.shared.dictationLanguage] as [String: Any],
            // What the office's held desks offer: firing, and hiring once there's a folder to hire into, by role.
            "hiring": [
                "fire": Config.shared.hiringEnabled,
                "hire": canHire,
                "roles": canHire ? Config.shared.roles.map(\.forPhone) : [],
            ] as [String: Any],
            // What the office's jukeboxes know: whether music is playing, null when that can't be told.
            // And the Mac's volume for the hold menu's bar, null where it can't be set.
            "music": Config.shared.musicEnabled ? [
                "playing": playing.map { $0 as Any } ?? NSNull(),
                "volume": volume.map { $0.level as Any } ?? NSNull(),
                "muted": volume?.muted ?? false,
            ] as [String: Any] : NSNull(),
            "layout": Config.shared.layout.jsonObject,
        ]
        // Keys in order: unsorted, the same layout can come out differently
        // each time, and the phone would take it for a new one.
        return (try? JSONSerialization.data(withJSONObject: state, options: [.sortedKeys])) ?? Data()
    }

    /// What the portable office's viewer draws: built field by field from
    /// what may leave this Mac (see `ViewerSnapshot`), never from
    /// `snapshotJSON()`. No session ids, paths, activity, models or buttons.
    func viewerSnapshot(hideNames: Bool, alias: (String) -> String) -> [String: Any] {
        let (keys, overflow) = layout()
        let config = Config.shared
        let desks = keys.map { key in
            ViewerSnapshot.Desk(pin: key.pin, session: key.session,
                                role: key.session.flatMap { roleOf[$0.id] }.flatMap { id in config.roles.first { $0.id == id } })
        }
        var snapshot = ViewerSnapshot.build(desks: desks, overflow: overflow.count, theme: Theme.current(),
                                            music: (config.musicEnabled, playing), layout: config.layout, hideNames: hideNames,
                                            now: Int64(Date().timeIntervalSince1970 * 1000), alias: alias)
        let upstairs = self.upstairs()
        // Who hired each, by name only: with names hidden, the hiring desk's number, as its desk shows it.
        let all = sessions.all
        snapshot["upstairs"] = ViewerSnapshot.upstairs(upstairs.desks, overflow: upstairs.overflow, hideNames: hideNames) { s in
            ViewerSnapshot.hirer(Deck.hiredBy(s, among: all), desks: keys.map(\.session), hideNames: hideNames)
        }
        return snapshot
    }

    func attach(_ stream: EventStream, phone: String) {
        let id = ObjectIdentifier(stream)
        streams[id] = (phone, stream)
        stream.onClose = { [weak self] in
            guard let self else { return }
            self.streams[id] = nil
            // Its last stream gone, as it sleeps or leaves the Wi-Fi: dictation it started stops.
            if !self.streams.values.contains(where: { $0.phone == phone }) { Dictation.phoneLeft(phone) }
        }
        stream.send(snapshotJSON())
    }

    /// Re-sends the state, for changes the sessions do not see (the setup page's).
    func refresh() { changed() }

    // MARK: Phones

    func isConnected(_ phone: String) -> Bool {
        streams.values.contains { $0.phone == phone }
    }

    /// Forgets a phone and hangs up on it now, rather than at its next request.
    func unpair(_ phone: String) {
        access.phones.remove(phone)
        streams.values.filter { $0.phone == phone }.forEach { $0.stream.close() }
    }

    func unpairAll() {
        access.phones.removeAll()
        streams.values.forEach { $0.stream.close() }
    }

    // MARK: Controls

    static let effortLevels = ["low", "medium", "high", "xhigh", "max"]

    /// Runs one of the phone's controls on a session, through its terminal's
    /// driver: none brings the tab forward but focusing and Terminal.app's
    /// scrolling.
    func perform(_ action: String, on id: String, query: [String: String], phone: String = "", done: @escaping (Response) -> Void) {
        // Releasing the mic stops recording where it started, whatever is
        // selected now — and even if that session has since gone.
        if action == "mic" && query["phase"] != "down" { return releaseMic(done: done) }
        // A new hire has no session yet, and a swap is of two desks.
        if action == "hire" {
            guard !TestCopy.isOn else { return done(.status(409, TestCopy.Refused.hiring.localizedDescription)) }
            return hire(at: query["desk"], as: query["role"], done: done)
        }
        if action == "swap" { return done(swapKeys(query)) }
        if action == "style" { return done(setStyle(query["preset"])) }
        // The mic's quick switch: dictation's language, for every phone.
        if action == "language" {
            guard let language = query["lang"], Dictation.languages.contains(language) else { return done(.status(400, "Unknown language")) }
            Config.shared.dictationLanguage = language
            Config.shared.save()
            changed()
            return done(.status(204))
        }
        if action == "music" {
            if query["do"] == "volume" { return done(setVolume(query["level"])) }
            return done(playMusic(query["do"] ?? "playpause"))
        }
        guard let session = sessions[id], !session.tab.isEmpty else { return done(.status(404, "Session not found")) }
        guard !session.isHeadless else { return done(.status(409, Deck.headless)) }

        // Accept and send are both Return. They are separate controls on the
        // phone because they mean different things, as do reject and esc,
        // which are often both Escape but not in every agent.
        let agent = session.agent
        switch action {
        case "accept", "send":
            // A second Return this soon would answer whatever the agent asks next, before you've seen it.
            let now = Date()
            lastReturn = lastReturn.filter { now.timeIntervalSince($0.value) < Deck.returnRepeat }
            guard lastReturn[id] == nil else { return done(.status(204)) }
            lastReturn[id] = now
            // A Return that didn't go doesn't count, so the next tap goes.
            press(.init(key: .returnKey), to: session) { [weak self] response in
                if response.status >= 400 { self?.lastReturn[id] = nil }
                done(response)
            }
        case "reject": press(agent.reject, to: session, done: done)
        case "esc": press(agent.interrupt, to: session, done: done)
        case "up": press(.init(key: .up), to: session, done: done)
        case "down": press(.init(key: .down), to: session, done: done)
        case "scroll":
            guard Config.shared.scrollEnabled else {
                return done(.status(403, "Scrolling is off. Turn it on in setup, on your Mac."))
            }
            guard let (driver, tab) = terminal(of: session, for: .scroll) else { return done(.status(409, Deck.notHere)) }
            guard !driver.keysNeedAccessibility || Permissions.hasAccessibility else { return done(.status(403, Deck.noKeyboard)) }
            if query["to"] == "bottom" { return inTab(session.tab, done: done) { driver.scrollToBottom(in: tab) } }
            // Kept within what a scroll can be first, as Go keeps it: an amount as large as an Int
            // can hold would overflow below, which stops the app.
            guard let asked = Int(query["lines"] ?? "") else { return done(.status(400, "Unknown amount")) }
            let lines = max(-Terminal.maxScroll, min(Terminal.maxScroll, asked))
            // The phone counts the way natural scrolling does. With it turned
            // off on the Mac, the wheel turns the other way, as everything else does.
            let direction = Scrolling.isNatural ? 1 : -1
            inTab(session.tab, done: done) { driver.scroll(lines: lines * direction, in: tab) }
        case "mic":
            guard Deck.dictates(agent) else {
                guard agent.hasVoice else { return done(.status(400, "\(agent.name) has no voice input")) }
                return pressMic(session, done: done)
            }
            guard let (driver, _) = terminal(of: session, for: .text) else { return done(.status(409, Deck.notHere)) }
            guard !driver.keysNeedAccessibility || Permissions.hasAccessibility else { return done(.status(403, Deck.noKeyboard)) }
            Dictation.start(language: Config.shared.dictationLanguage, for: session.tab, phone: phone,
                            deliver: { [weak self] text, tab in self?.insertDictation(text, into: tab) }) { started in
                done(started ? .status(204) : .status(409, "Dictation couldn't start. Look at it in setup, on your Mac."))
            }
        case "fire":
            guard Config.shared.hiringEnabled else { return done(.status(403, Deck.hiringOff)) }
            guard let (driver, tab) = terminal(of: session, for: .fire) else { return done(.status(409, Deck.notHere)) }
            Terminal.queue.async {
                let fired = driver.fire(tab, agent: agent)
                DispatchQueue.main.async {
                    switch fired {
                    case .fired: done(.status(204))
                    case .notStopped: done(.status(409, "Couldn't stop the agent, so its tab is left open."))
                    case .gone: done(.status(404, "That agent's tab has gone."))
                    }
                }
            }
        case "clear":
            guard !session.isBusy else { return done(.status(409, "Busy — clear it when \(agent.name) is done")) }
            type(agent.newConversation, to: session, done: done)
        case "button":
            guard let button = Config.shared.layout.madeButton(query["button"] ?? "") else {
                return done(.status(404, "That button has gone. Your phone will catch up in a moment."))
            }
            if let agents = button.agents, !agents.contains(agent.rawValue) {
                return done(.status(400, "This button isn't for \(agent.name)"))
            }
            if button.whenFree == true && session.isBusy {
                return done(.status(409, "Busy — try again when \(agent.name) is done"))
            }
            if let key = button.key, let keystroke = Button.keys[key] { return press(keystroke, to: session, done: done) }
            guard let text = button.text, !text.isEmpty else {
                return done(.status(400, "This button types nothing yet. Set it up in setup, on your Mac."))
            }
            type(text, to: session, done: done)
        case "effort":
            guard agent.hasEffortDial else { return done(.status(400, "\(agent.name) has no effort setting")) }
            // Set by typing /effort, which a terminal Smolcorp can only watch can't take.
            guard terminal(of: session, for: .text) != nil else { return done(.status(409, Deck.notHere)) }
            guard let level = query["level"], Deck.effortLevels.contains(level) else { return done(.status(400, "Unknown level")) }
            // ☢ Max says so, where the dial reaching max doesn't: the office
            // only sends its agent to the button for a press.
            if query["source"] == "max" && level == "max" { session.maxPressedAt = Date().timeIntervalSince1970 * 1000 }
            session.pendingEffort = level == session.effort ? "" : level
            let queued = !session.pendingEffort.isEmpty && session.isBusy
            changed()
            done(.status(queued ? 202 : 204))
        default:
            done(.status(400, "Unknown action"))
        }
    }

    static let hiringOff = "Hiring and firing are off. Turn them on in setup, on your Mac."
    /// For a session in a terminal Smolcorp can only watch, or can't do this in.
    static let notHere = "Smolcorp can't do that in this terminal yet."
    /// For a headless run's desk, which has no terminal at all.
    static let headless = "Nothing to type into: it runs headless."

    /// `session`'s tab and the driver for its terminal, if that terminal lets
    /// the deck do `what` there.
    private func terminal(of session: Session, for what: TerminalAbilities) -> (TerminalDriver, TabAddress)? {
        guard let tab = TabAddress(session.tab), let driver = Terminals.driver(for: tab), driver.abilities.contains(what) else { return nil }
        return (driver, tab)
    }

    /// Swaps two keys' projects, for a person carried from one desk to
    /// another in the office: each is pinned where the other was, so the
    /// order holds the next time too, as pins made in setup do. It only
    /// orders the keys, so any paired phone may.
    private func swapKeys(_ query: [String: String]) -> Response {
        let (keys, _) = layout()
        guard let a = Int(query["from"] ?? ""), let b = Int(query["to"] ?? ""), a != b,
              keys.indices.contains(a), keys.indices.contains(b) else { return .status(400, "Unknown desks") }
        let project = { (i: Int) in keys[i].session?.project ?? keys[i].pin }
        let (first, second) = (project(a), project(b))
        guard first != nil || second != nil else { return .status(204) }
        guard first != second else { return .status(409, "Both desks are the same project, so they stay as they are.") }
        Config.shared.swapPins(first, at: a, with: second, at: b)
        changed()
        return .status(204)
    }

    /// The office in one of its styles, from the phone, on every screen it's
    /// on. Only the style's name is kept: what you furnished stays under it.
    private func setStyle(_ preset: String?) -> Response {
        guard let preset, Room.presets.contains(preset) else { return .status(400, "Unknown style") }
        let config = Config.shared
        guard config.layout.room.preset != preset else { return .status(204) }
        config.layout.room.preset = preset
        config.save()
        changed()
        return .status(204)
    }

    static let musicOff = "The jukebox can't play your music: turn it on in setup, on your Mac."

    /// Plays or pauses the Mac's music, or skips a song either way, from a
    /// jukebox in the office: `key` is "playpause", "previous" or "next".
    private func playMusic(_ key: String) -> Response {
        guard Music.keys[key] != nil else { return .status(400, "A jukebox has no \(key) key.") }
        guard Config.shared.musicEnabled else { return .status(403, Deck.musicOff) }
        guard Permissions.hasAccessibility else { return .status(403, Deck.noKeyboard) }
        Music.press(key)
        // The player takes a moment to say it has.
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.6) { [weak self] in self?.followMusic() }
        return .status(204)
    }

    /// Sets the Mac's output volume from the jukebox's bar: `level` is 0 to
    /// 100. Unlike the keys it needs no Accessibility.
    private func setVolume(_ level: String?) -> Response {
        guard let level = Music.volumeLevel(level) else { return .status(400, "The volume goes from 0 to 100.") }
        guard Config.shared.musicEnabled else { return .status(403, Deck.musicOff) }
        Music.setVolume(level)
        followVolume()
        return .status(204)
    }

    /// Reads the volume and mute, and tells the phones when either changed:
    /// from the bar, the keyboard's volume keys, or another output device.
    private func followVolume() {
        let now = Music.volume()
        guard now?.level != volume?.level || now?.muted != volume?.muted else { return }
        volume = now
        changed()
    }

    /// Looks whether music is playing, with the jukebox on, and tells the phones when that changes.
    func followMusic() {
        // The volume needs no looking: CoreAudio says when it changes.
        if !watchingVolume {
            watchingVolume = true
            Music.watchVolume { [weak self] in self?.followVolume() }
            followVolume()
        }
        guard !checkingMusic else { return }
        guard Config.shared.musicEnabled else {
            if playing != nil {
                playing = nil
                changed()
            }
            return
        }
        checkingMusic = true
        DispatchQueue.global(qos: .utility).async {
            let now = Music.isPlaying()
            DispatchQueue.main.async {
                self.checkingMusic = false
                guard now != self.playing else { return }
                self.playing = now
                self.changed()
            }
        }
    }

    /// How long after a hire the next is refused. A second tap, or a second
    /// phone, would otherwise open a second window before the first new hire
    /// has reached the deck.
    private static let hireCooldown: TimeInterval = 5
    /// How long a hire's tab waits for its agent to report, to take its role
    /// and desk: long enough for a trust prompt left waiting a while. Its tab
    /// closing ends the wait sooner, so a tty used again later can't take them.
    private static let hiredTabWait: TimeInterval = 15 * 60
    /// Long enough for Terminal to list a tab a hire has just opened: a hire
    /// whose tab isn't among Terminal's after this has closed, even if no
    /// look ever saw it open.
    private static let hiredTabListed: TimeInterval = 15
    private var lastHire: Date?

    /// A new Terminal window with the agent set up for new hires at work in
    /// it, or the role's agent, started on the role. Hired at a desk whose key
    /// is pinned to a project, it starts in that project's folder, so it sits
    /// down at that desk; otherwise in the folder set up for new hires. Its
    /// hooks bring it to the deck, and the office, as for any session.
    private func hire(at desk: String?, as roleID: String?, done: @escaping (Response) -> Void) {
        let config = Config.shared
        guard config.hiringEnabled else { return done(.status(403, Deck.hiringOff)) }
        var pinned: String?
        // With a set number of desks, any free one can be hired at, and the
        // hire sits there; with Auto they stay packed, so it takes the first free.
        var held: Int?
        if let desk {
            let (keys, _) = layout()
            // With Auto, the office also offers the desk after the last: a hire adds it.
            let next = config.layout.keys == .auto && keys.count < KeyCount.most ? keys.count : nil
            guard let index = Int(desk), keys.indices.contains(index) || index == next else { return done(.status(400, "Unknown desk")) }
            pinned = keys.indices.contains(index) ? keys[index].pin.flatMap(Deck.folder) : nil
            if case .fixed = config.layout.keys, keys[index].pin == nil {
                guard keys[index].session == nil else { return done(.status(409, "Someone's at that desk already.")) }
                held = index
            }
        }
        guard let folder = pinned ?? Deck.folder(config.hireFolder) else {
            return done(.status(409, "Choose a folder for new hires in setup, on your Mac."))
        }
        var role: Role?
        if let roleID, !roleID.isEmpty {
            guard let found = config.roles.first(where: { $0.id == roleID }) else {
                return done(.status(404, "That role isn't there any more. Pick another."))
            }
            role = found
        }
        if let last = lastHire, Date().timeIntervalSince(last) < Deck.hireCooldown {
            return done(.status(429, "Hired a moment ago. Try again in a few seconds."))
        }
        lastHire = Date()
        var command = config.hireAgent
        if let role {
            guard let script = Deck.startScript(for: role, in: folder, pinned: pinned != nil) else {
                lastHire = nil
                return done(.status(500, "Couldn't get the role ready to start"))
            }
            command = script.path
        }
        let terminal = Openers.isInstalled(config.hireTerminal) ? config.hireTerminal : "terminal"
        let inTmux = config.hireInTmux && Tmux.path != nil
        let name = Openers.choices.first { $0.id == terminal }?.name ?? "Terminal"
        Terminal.queue.async {
            let tab = Openers.open(command, in: folder, terminal: terminal, inTmux: inTmux)
            DispatchQueue.main.async {
                // Nobody was hired, so the next try needn't wait.
                guard let tab else {
                    self.lastHire = nil
                    if role != nil { try? FileManager.default.removeItem(atPath: command) }
                    return done(.status(500, "\(name) didn't open"))
                }
                // Kept for the role and desk, and for a Ghostty hire, to know its terminal by.
                if role != nil || held != nil || tab.hasPrefix("ghostty-id:") {
                    self.hiredTabs.append((tab, role?.id, held, Date(), folder))
                }
                done(.status(204))
            }
        }
    }

    /// `role`'s start-up script, written for a hire's Terminal window to run
    /// once. Only you can read or run it.
    private static func startScript(for role: Role, in folder: String, pinned: Bool) -> URL? {
        let script = role.startScript(in: folder, pinned: pinned)
        let manager = FileManager.default
        try? manager.createDirectory(at: Paths.hires, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        let url = Paths.hires.appendingPathComponent("hire-\(UUID().uuidString).sh")
        guard manager.createFile(atPath: url.path, contents: Data(script.utf8), attributes: [.posixPermissions: 0o700]) else { return nil }
        return url
    }

    /// `path` as a folder that's there, with ~ for home, or nil.
    static func folder(_ path: String) -> String? {
        let expanded = (path.trimmingCharacters(in: .whitespaces) as NSString).expandingTildeInPath
        var isFolder: ObjCBool = false
        guard expanded.hasPrefix("/"), FileManager.default.fileExists(atPath: expanded, isDirectory: &isFolder),
              isFolder.boolValue else { return nil }
        return (expanded as NSString).standardizingPath
    }

    /// The folder panel, while it's open.
    private var hirePanel: NSOpenPanel?

    /// Asks for the new hires' folder with the Mac's own panel, from the setup
    /// page. Asked again while it's open, it brings that panel forward rather
    /// than opening another, and answers at once.
    func chooseHireFolder(then done: @escaping () -> Void) {
        if let open = hirePanel {
            NSApp.activate(ignoringOtherApps: true)
            open.makeKeyAndOrderFront(nil)
            return done()
        }
        let panel = NSOpenPanel()
        hirePanel = panel
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        panel.prompt = "Choose"
        panel.message = "New hires from the office start their work in this folder."
        if let current = Deck.folder(Config.shared.hireFolder) { panel.directoryURL = URL(fileURLWithPath: current) }
        NSApp.activate(ignoringOtherApps: true)
        panel.begin { response in
            self.hirePanel = nil
            if response == .OK, let url = panel.url {
                Config.shared.hireFolder = url.path
                Config.shared.save()
                self.refresh()
            }
            done()
        }
    }

    /// Push to talk through Claude's own /voice, in tap mode: Space starts
    /// listening, Space again stops and sends. Hold mode cannot be used — it
    /// detects a held key by its key-repeat, and a posted key never repeats.
    ///
    /// Every start is matched by exactly one stop. A second press while
    /// recording, or a release with nothing recording, is ignored: an extra
    /// Space would flip Claude's state and leave the two out of step.
    private func pressMic(_ session: Session, done: @escaping (Response) -> Void) {
        if let current = micPress, Date().timeIntervalSince(current.at) < Deck.voiceLimit {
            return done(.status(204))
        }
        micPress = (session.tab, Date())
        press(.init(key: .space), tab: session.tab) { [weak self] response in
            // Nothing started, so the release must not send a Space either.
            if response.status >= 400 { self?.micPress = nil }
            done(response)
        }
    }

    /// Dictation's text, into the tab it was for, without Return: only while
    /// that tab's agent still runs there, as for keys (see `toAgent`).
    private func insertDictation(_ text: String, into tab: String) {
        guard !text.isEmpty, let address = TabAddress(tab), let driver = Terminals.driver(for: address),
              driver.abilities.contains(.text) else { return }
        let agent = sessions.all.first { $0.tab == tab }?.agent
        Terminal.queue.async {
            // Only where the agent is in front: behind the shell, the text would wait there to run as a command.
            if let agent, let tty = Terminals.tty(of: tab), let place = Terminal.place(of: agent, on: tty), place != .front { return }
            _ = driver.insert(text, in: address)
        }
    }

    /// Whether the mic dictates for `agent`, rather than pressing Space for its
    /// own voice mode: for every agent without one, and for Claude Code too if
    /// setup says so.
    static func dictates(_ agent: Agent) -> Bool {
        Dictation.isOn && (!agent.hasVoice || Config.shared.dictationForClaude)
    }

    private func releaseMic(done: @escaping (Response) -> Void) {
        // Dictating, or getting ready to: the text goes into the tab it was for, without Return.
        if Dictation.isActive {
            Dictation.stop { [weak self] text, tab in self?.insertDictation(text, into: tab) }
            return done(.status(204))
        }
        defer { micPress = nil }
        // After the limit Claude has stopped by itself, and a Space would
        // start it again.
        guard let started = micPress, Date().timeIntervalSince(started.at) < Deck.voiceLimit else {
            return done(.status(204))
        }
        press(.init(key: .space), tab: started.tab, done: done)
    }

    private func press(_ keystroke: Terminal.Keystroke, tab: String, done: @escaping (Response) -> Void) {
        guard let address = TabAddress(tab), let driver = Terminals.driver(for: address), driver.abilities.contains(.keys) else {
            return done(.status(409, Deck.notHere))
        }
        guard !driver.keysNeedAccessibility || Permissions.hasAccessibility else { return done(.status(403, Deck.noKeyboard)) }
        inTab(tab, done: done) { driver.press(keystroke, in: address) }
    }

    /// Runs `work`, which may bring `tab` forward. A tab you have been shown
    /// counts as seen.
    private func inTab(_ tab: String, done: @escaping (Response) -> Void, _ work: @escaping () -> Bool) {
        onTerminal(done) {
            let reached = work()
            if reached { DispatchQueue.main.async { self.sessions.markSeen(tab: tab) } }
            return reached
        }
    }

    /// Presses `keystroke` in `session`'s tab, once its agent is known to be there still.
    private func press(_ keystroke: Terminal.Keystroke, to session: Session, done: @escaping (Response) -> Void) {
        guard let (driver, tab) = terminal(of: session, for: .keys) else { return done(.status(409, Deck.notHere)) }
        guard !driver.keysNeedAccessibility || Permissions.hasAccessibility else { return done(.status(403, Deck.noKeyboard)) }
        let address = session.tab
        toAgent(session, done: done) {
            let reached = driver.press(keystroke, in: tab)
            if reached { DispatchQueue.main.async { self.sessions.markSeen(tab: address) } }
            return reached
        }
    }

    /// Types `text` and Return into `session`'s tab, once its agent is known to be there still.
    private func type(_ text: String, to session: Session, done: @escaping (Response) -> Void) {
        guard let (driver, tab) = terminal(of: session, for: .text) else { return done(.status(409, Deck.notHere)) }
        toAgent(session, done: done) { driver.type(text, in: tab) }
    }

    /// Runs `work` on the tab once its agent is known to be in front there
    /// still. An agent that ended without saying so, crashed or killed, leaves
    /// its tab to the shell, where its keys would land and a button's text
    /// would run as a command, or to whatever runs there next: its session
    /// goes instead. One stopped with Ctrl-Z or in the background stays, and
    /// gets nothing until it's back in front.
    private func toAgent(_ session: Session, done: @escaping (Response) -> Void, _ work: @escaping () -> Bool) {
        let tab = session.tab, agent = session.agent
        Terminal.queue.async {
            // Unknown is not the same as gone: the keys go as they always did.
            switch Terminals.tty(of: tab).flatMap({ Terminal.place(of: agent, on: $0) }) {
            case .gone?:
                return DispatchQueue.main.async {
                    self.sessions.prune(closedTabs: [tab])
                    done(.status(410, "That agent has ended."))
                }
            case .behind?:
                return DispatchQueue.main.async { done(.status(409, "That agent isn't in front in its terminal: bring it back with fg.")) }
            case .front?, nil:
                break
            }
            let ok = work()
            DispatchQueue.main.async { done(ok ? .status(204) : .status(404, "Terminal tab not found")) }
        }
    }

    private func onTerminal(_ done: @escaping (Response) -> Void, _ work: @escaping () -> Bool) {
        Terminal.queue.async {
            let ok = work()
            DispatchQueue.main.async { done(ok ? .status(204) : .status(404, "Terminal tab not found")) }
        }
    }

    func focus(_ id: String, done: @escaping (Response) -> Void) {
        guard let session = sessions[id], !session.tab.isEmpty else { return done(.status(404)) }
        // A terminal Smolcorp can only watch has nothing to bring forward: the tap only picks the desk.
        guard let (driver, tab) = terminal(of: session, for: .focus) else { return done(.status(204)) }
        inTab(session.tab, done: done) { driver.focus(tab) }
    }

    /// Typed into a running turn, /effort would be queued as a message, so it
    /// waits for the session to be free. No hook reports a slash command, so
    /// the new level is assumed; the next event's effort field corrects it.
    private func applyQueuedEffort() {
        for session in sessions.all where !session.pendingEffort.isEmpty && !session.isBusy {
            let level = session.pendingEffort
            session.pendingEffort = ""
            guard let (driver, tab) = terminal(of: session, for: .text) else { continue }
            let address = session.tab, agent = session.agent
            Terminal.queue.async {
                let tty = Terminals.tty(of: address)
                // Only to the agent in front: see `toAgent`.
                guard tty.flatMap({ Terminal.place(of: agent, on: $0) }).map({ $0 == .front }) != false, driver.type("/effort \(level)", in: tab) else { return }
                DispatchQueue.main.async {
                    session.effort = level
                    self.changed()
                }
            }
        }
    }

    // MARK: Housekeeping

    /// Green means "finished, not looked at". While anything is green, watch
    /// for its tab coming to the front.
    private func checkFrontTab() {
        guard sessions.contains(.unread), !checkingFront else { return }
        checkingFront = true
        Terminal.queue.async {
            let tty = Terminal.frontTTY()
            DispatchQueue.main.async {
                self.checkingFront = false
                if let tty { self.sessions.markSeen(tab: TabAddress.terminalApp(tty: tty).text) }
            }
        }
    }

    private func pruneClosedTabs() {
        // Hires waiting for their agent are looked at too, sessions or not.
        guard !sessions.all.isEmpty || !hiredTabs.isEmpty, !checkingTabs else { return }
        checkingTabs = true
        Terminal.queue.async {
            let open = Terminal.openTTYs()
            DispatchQueue.main.async {
                guard let open else { // unknown is not the same as none
                    self.checkingTabs = false
                    return
                }
                let missing = self.terminalTTYs.subtracting(open)
                let closed = missing.intersection(self.missedOnce)
                self.missedOnce = missing.subtracting(closed)
                self.terminalTTYs = open.union(self.missedOnce)
                // A session found by its folder before Terminal had listed its tab is on Terminal's tab after all.
                for tty in open { self.sessions.retag("other:\(tty)", as: TabAddress.terminalApp(tty: tty).text) }
                // Only Terminal.app's tabs are looked at here: its tab list is what says one has closed.
                let terminalTTY = { (tab: String) -> String? in TabAddress(tab).flatMap { $0.kind == "tty" ? $0.id : nil } }
                // A hire's tab closed, seen open before or not: no tty used again later takes its desk or role.
                self.hiredTabs.removeAll { hired in
                    guard let tty = terminalTTY(hired.tab) else { return false }
                    return closed.contains(tty) || !open.contains(tty) && Date().timeIntervalSince(hired.at) > Deck.hiredTabListed
                }
                let suspects = self.sessions.all.compactMap { session -> (tab: String, tty: String, agent: Agent)? in
                    guard let tty = terminalTTY(session.tab), closed.contains(tty) else { return nil }
                    return (session.tab, tty, session.agent)
                }
                guard !suspects.isEmpty else {
                    self.checkingTabs = false
                    return
                }
                // The last word is the tab's own: its agent still running there means it's open.
                DispatchQueue.global(qos: .utility).async {
                    let gone = Set(suspects.filter { Terminal.isRunning($0.agent, on: $0.tty) == false }.map(\.tab))
                    DispatchQueue.main.async {
                        self.checkingTabs = false
                        self.sessions.prune(closedTabs: gone)
                    }
                }
            }
        }
    }

    /// Sessions in tabs of other terminals: tmux panes and iTerm2 sessions,
    /// closed or with their agent gone from them, and those Smolcorp can only
    /// watch, whose agent no longer running on their tty means it has ended. Gone at two looks in a row, they go. Unknown is not the
    /// same as gone.
    private func pruneClosedPanes() {
        let tabs = sessions.all.compactMap { session -> (tab: String, isGone: () -> Bool)? in
            guard let address = TabAddress(session.tab), address.kind != "tty" else { return nil }
            let agent = session.agent
            if let driver = Terminals.driver(for: address) {
                // Closed, or open with its agent gone from it, as after a crash.
                return (session.tab, {
                    driver.isOpen(address) == false || driver.tty(of: address).flatMap { Terminal.isRunning(agent, on: $0) } == false
                })
            }
            guard let tty = Terminals.ttyInAddress(of: session.tab) else { return nil }
            return (session.tab, { Terminal.isRunning(agent, on: tty) == false })
        }
        guard !tabs.isEmpty, !checkingPanes else { return }
        checkingPanes = true
        Terminal.queue.async {
            let gone = Set(tabs.filter { $0.isGone() }.map(\.tab))
            DispatchQueue.main.async {
                self.checkingPanes = false
                let closed = gone.intersection(self.panesMissedOnce)
                self.panesMissedOnce = gone.subtracting(closed)
                // A Ghostty terminal's tty may be another's next.
                for tab in closed { if let address = TabAddress(tab), address.kind == "ghostty" { Ghostty.forget(tty: address.id) } }
                if !closed.isEmpty { self.sessions.prune(closedTabs: closed) }
            }
        }
    }
}
