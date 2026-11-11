import Foundation

/// How the GM's mail reaches a hire's window (T402): the `deliver` step of `MailRouter`.
/// All of it runs on `queue`, never the main queue; what only the main queue may know
/// (the deck's view of the session) is asked for by hopping there for a moment.
///
/// In this order:
/// 1. **Claude Code's inbox socket**, but only when `Config.mailInboxSocket` is on AND the session's own
///    file names a socket AND its Claude Code is at least `inboxSince`. Off by default: the one test of
///    it (Smolcorp Plus's spike, Claude Code 2.1.309) is ours, not Anthropic's documentation, so it is
///    the owner's choice to trust it. A socket that refuses falls through to tmux.
/// 2. **A tmux bracketed paste**, only into an idle, empty input box (`MailRouter.gate`, on a pane capture
///    taken just before, with the deck's own `.asking` state read right after it), then Return, which here is
///    only the send of that message. Multi-line text is pasted as it is only if the agent asked for
///    bracketed paste; otherwise it goes as one line and the outbox says so.
/// 3. Otherwise the router keeps it queued: `waiting` (the hire is busy, asking or has a draft) is looked at again
///    every few seconds and is not an attempt; `trouble` (tmux or the session failed) counts toward the attempts limit.
///
/// Nothing here answers a question or a menu: the box must be empty, and Return is pressed only after
/// the window is looked at again and still shows no question.
enum MailDelivery {
    /// The delivery work: one message at a time, so two never paste into one window together.
    static let queue = DispatchQueue(label: "smolcorp.mail-delivery")
    /// The Claude Code version the inbox socket was seen to work on (Smolcorp Plus's spike, 23 Oct 2026).
    static let inboxSince = "2.1.309"
    /// How often the router is asked to try the messages that are due.
    static let pumpEvery: TimeInterval = 5
    /// Lines of the window looked at.
    static let captureLines = 40
    /// A moment between the paste and Return, so the agent has taken the paste in.
    static let settle: TimeInterval = 0.4

    /// What delivery needs from the Mac, so that tests give stand-ins.
    struct Ways {
        var tmux: (FrontDesk.Hire) -> HireTmux?
        /// Whether the deck itself knows the hire's session is asking a question. Read just then.
        var deckAsking: (FrontDesk.Hire) -> Bool
        /// The hire's Claude Code inbox socket and version, from its session file.
        var inbox: (FrontDesk.Hire) -> (socket: String, version: String)?
        var inboxOn: () -> Bool
        /// Writes a line to a socket: nil once written, else why not.
        var post: (_ path: String, _ line: Data) -> String?
        var pause: (TimeInterval) -> Void
        /// A hire in T3 Code's way in: `t3 thread send <id> -` (T797b).
        var t3: (FrontDesk.Hire, String) -> MailRouter.Delivery = { T3Desk.send($0, $1) }
    }

    /// `commit` is called just before the first thing that types (the socket write, the tmux paste); if it
    /// answers false, nothing is typed. The router counts the try there. Tests may leave it out.
    static func deliver(_ hire: FrontDesk.Hire, _ text: String, ways: Ways, commit: () -> Bool = { true }) -> MailRouter.Delivery {
        // 0. A hire in T3 Code has no window: its thread takes the text on t3's standard input, and queues it while busy.
        if hire.engine == T3Desk.engine {
            guard commit() else { return .trouble("The outbox couldn't be written, so nothing was sent.") }
            return ways.t3(hire, text)
        }
        // 1. The inbox socket, when it's switched on and known to work on this version. The deck's own
        // `.asking` state is looked at first, so that no path writes to a session waiting at a question.
        if ways.inboxOn(), let inbox = ways.inbox(hire), versionAtLeast(inbox.version, inboxSince) {
            if let wait = MailRouter.deckGate(deckAsking: ways.deckAsking(hire)) { return wait }
            guard commit() else { return .trouble("The outbox couldn't be written, so nothing was typed.") }
            if ways.post(inbox.socket, line(for: text)) == nil { return .delivered }
        }

        // 2. tmux.
        guard !hire.tmuxSocket.isEmpty, let tmux = ways.tmux(hire) else {
            return .failed("\(hire.name) doesn't run in tmux here and has no inbox socket to use, so mail can't reach it.")
        }
        switch tmux.isAlive() {
        case nil: return .trouble("tmux couldn't be asked.")
        case false?: return .trouble("\(hire.name)'s tmux session isn't running.")
        case true?: break
        }
        // Looked at now, and the deck asked after it: either says it is waiting at a question, nothing is typed.
        let capture = tmux.capture(lines: captureLines)
        if let wait = MailRouter.gate(capture, deckAsking: ways.deckAsking(hire)) { return wait }

        guard let mode = tmux.pasteState() else { return .trouble("Couldn't tell how its window takes a paste.") }
        if mode.inMode { return .waiting("waiting: its window is scrolled back or in copy mode") }
        var body = text
        var note: String?
        if text.contains("\n"), !mode.bracketed {
            // Without bracketed paste a line feed would be read as Return.
            body = oneLine(text)
            note = "Sent as one line (new lines shown as /): its window wasn't taking pastes as one piece."
        }
        guard HireTmux.canPaste(body) else { return .failed("The message holds a control character that can't be pasted safely.") }
        guard commit() else { return .trouble("The outbox couldn't be written, so nothing was typed.") }
        guard tmux.paste(body) else { return .trouble("tmux didn't take the paste.") }

        ways.pause(settle)
        // Return is the send of this message and nothing else. The box must now hold the message (a draft), with
        // no menu showing (read from the window's structure, so the message's own words count for nothing), and the
        // deck must still not know the session as asking. Otherwise no Return: the text stays in the box.
        let after = tmux.capture(lines: captureLines)
        if let asking = HireDesk.promptRefusal(after) {
            return .failed("The text was put in its box but not sent, because the window began waiting at a question (\(asking)) The text in its box has to be cleared by hand; later mail waits until then.")
        }
        guard HireDesk.inputState(after) == .draft else {
            return .failed("The text was put in its box but not sent, because its window no longer looked like a box holding the message. What is in the box has to be cleared by hand; later mail waits until then.")
        }
        if ways.deckAsking(hire) {
            return .failed("The text was put in its box but not sent, because the session began waiting at a question. The text in its box has to be cleared by hand; later mail waits until then.")
        }
        guard tmux.submit() else { return .failed("The text was put in its box but Return couldn't be pressed.") }
        return note.map { .deliveredAs($0) } ?? .delivered
    }

    /// The lines of `text` joined with " / ", empty lines dropped, tabs made spaces (a Tab would arrive as a key).
    static func oneLine(_ text: String) -> String {
        text.replacingOccurrences(of: "\t", with: " ").split(whereSeparator: \.isNewline).map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }.joined(separator: " / ")
    }

    // MARK: The inbox socket

    /// The line an inbox socket reads: one JSON line, no auth line, newline-terminated.
    static func line(for text: String) -> Data {
        let object: [String: Any] = ["type": "user", "message": ["role": "user", "content": text]]
        var data = (try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys, .withoutEscapingSlashes])) ?? Data()
        data.append(0x0A)
        return data
    }

    /// Whether the dotted `version` is `minimum` or later; false for one that can't be read.
    static func versionAtLeast(_ version: String, _ minimum: String) -> Bool {
        let have = version.split(separator: ".").map { Int($0) }
        let need = minimum.split(separator: ".").map { Int($0) }
        guard !have.isEmpty, !have.contains(where: { $0 == nil }) else { return false }
        for i in 0..<max(have.count, need.count) {
            let a = i < have.count ? have[i]! : 0, b = i < need.count ? need[i]! : 0
            if a != b { return a > b }
        }
        return true
    }

    /// Writes `line` to the Unix socket at `path`: nil once all of it is written, else why not.
    /// Only a socket of this user is written to.
    static func post(_ path: String, _ line: Data) -> String? {
        var info = stat()
        guard lstat(path, &info) == 0 else { return "its socket isn't there" }
        guard info.st_mode & S_IFMT == S_IFSOCK, info.st_uid == getuid() else { return "that isn't a socket of yours" }
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        let bytes = Array(path.utf8)
        guard bytes.count < MemoryLayout.size(ofValue: address.sun_path) else { return "its socket's path is too long" }
        withUnsafeMutableBytes(of: &address.sun_path) { raw in
            raw.copyBytes(from: bytes)
            raw[bytes.count] = 0
        }
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { return "no socket: \(String(cString: strerror(errno)))" }
        defer { close(fd) }
        var timeout = timeval(tv_sec: 3, tv_usec: 0)
        setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &timeout, socklen_t(MemoryLayout<timeval>.size))
        var noSignal: Int32 = 1
        setsockopt(fd, SOL_SOCKET, SO_NOSIGPIPE, &noSignal, socklen_t(MemoryLayout<Int32>.size))
        let connected = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
        }
        guard connected == 0 else { return "refused: \(String(cString: strerror(errno)))" }
        var sent = 0
        return line.withUnsafeBytes { raw in
            while sent < raw.count {
                let n = write(fd, raw.baseAddress!.advanced(by: sent), raw.count - sent)
                if n <= 0 { return "cut off after \(sent) of \(raw.count) bytes: \(String(cString: strerror(errno)))" }
                sent += n
            }
            return nil
        }
    }
}

extension Deck {
    /// Gives the mail its delivery step and its timer. Delivery runs on `MailDelivery.queue`; the router's
    /// own state is touched on the main queue only, which is where the timer fires.
    func startMail() {
        mail.queue = MailDelivery.queue
        let ways = MailDelivery.Ways(
            tmux: { hire in HireTmux(socketName: URL(fileURLWithPath: hire.tmuxSocket).lastPathComponent) },
            deckAsking: { [weak self] hire in
                Deck.onMain { self?.hireIsAsking(hire) ?? false }
            },
            inbox: { [weak self] hire in
                let sessionIDs = Deck.onMain { self?.liveSessions(of: hire).map(\.id) ?? [] }
                guard sessionIDs.count == 1,
                      let peer = ClaudePeers.all().first(where: { $0.sessionId == sessionIDs[0] }),
                      let socket = peer.messagingSocketPath, !socket.isEmpty, let version = peer.version else { return nil }
                return (socket, version)
            },
            inboxOn: { Deck.onMain { Config.shared.mailInboxSocket } },
            post: MailDelivery.post,
            pause: { Thread.sleep(forTimeInterval: $0) })
        mail.deliver = { hire, text, commit in MailDelivery.deliver(hire, text, ways: ways, commit: commit) }
        // A bot's reply in its mailbox becomes a line in the GM's inbox (the event log), from that bot.
        mail.onBotReply = { [weak self] bot, reply in
            self?.events.append(EventLog.Entry(at: Date().timeIntervalSince1970 * 1000, session: bot.sessionID, folder: bot.folder,
                                               agent: bot.agent, kind: .mail, detail: "\(bot.name) wrote \(String(reply.name.unicodeScalars.map { CharacterSet.controlCharacters.contains($0) ? " " : Character($0) })): \(reply.text)"))
        }
        // A hire's message to the GM is a line in the same inbox, from that hire: its id and name, never its key.
        mail.onHireToGM = { [weak self] hire, message in self?.events.append(MailRouter.inboxEntry(from: hire, message)) }
        Timer.scheduledTimer(withTimeInterval: MailDelivery.pumpEvery, repeats: true) { [weak self] _ in self?.mail.pump() }
    }

    /// Runs `body` on the main queue and answers, from any queue.
    private static func onMain<T>(_ body: () -> T) -> T {
        Thread.isMainThread ? body() : DispatchQueue.main.sync(execute: body)
    }

    /// The deck's sessions that are `hire`'s.
    private func liveSessions(of hire: FrontDesk.Hire) -> [Session] {
        sessions.all.filter {
            FrontDesk.works(FrontDesk.Live(id: $0.id, agent: $0.agent.rawValue, folder: $0.folder, project: $0.project, state: $0.state.rawValue), for: hire)
        }
    }

    /// Whether any of the hire's sessions is asking a question, as the deck knows it.
    private func hireIsAsking(_ hire: FrontDesk.Hire) -> Bool {
        liveSessions(of: hire).contains { Deck.isAsking($0.state) }
    }
}
