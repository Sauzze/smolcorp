import Foundation

/// The office's desks for the hires in T3 Code (T3D-a, experimental). A hire in T3 Code has no terminal tab, and
/// `Sessions` drops every session with none, so its desk is made here, as a cloud bot's is: from Smolcorp's own
/// registry alone (engine t3, with its thread and its worktree), never from a hook, a name, a title or a folder
/// guessed. Its Claude Code's own hook events are never a desk of their own (`Deck.ingest` keeps them off every
/// terminal's desk, `Deck.isT3HiresOwn`).
///
/// The desk's look is what the lane's follow last saw of its thread (`T3Desk.lastSeen`): at work, waiting for an
/// answer in T3 Code, or done. Nothing here types into anything: these sessions aren't `Sessions`', so no key,
/// text, dictation, effort or fire can find them, and the office is told each control is off (`Deck.t3Entry`).
/// A tap opens the hire's own thread in T3 Code; Let go is the lane's (`Deck.letGoOfT3`).
final class T3Desks {
    /// How a T3 desk's session id starts: the rest is its hire's id. No hook's session id looks like this.
    static let prefix = "t3:"

    static func sessionID(of hireID: String) -> String { prefix + hireID }

    /// The desk's session, by its hire's id, kept so it keeps its place and the time it started.
    private var byHire: [String: Session] = [:]
    /// The hires whose desks are kept, as the registry said last.
    private var hires: [String: FrontDesk.Hire] = [:]
    /// The hires whose done was seen (their desk tapped): white till they work again, rather than green.
    private var seenDone: Set<String> = []

    /// Whether `hire` has a desk: the registry says it runs in T3 Code, with a thread and a worktree, and it's still
    /// on the team. Let go (fired) or failed, its desk is freed. Its worktree must be where hires' worktrees are made.
    static func hasDesk(_ hire: FrontDesk.Hire, worktrees: String = Config.shared.hireWorktrees) -> Bool {
        hire.engine == T3Desk.engine && !hire.t3Thread.isEmpty && hire.state != .fired && hire.state != .failed
            && inHiresFolder(hire, worktrees: worktrees)
    }

    /// Whether `hire`'s worktree is one Smolcorp would have made (FXT3Da N3): a folder of its own in `<repo>-hires`
    /// beside its repo, or in the repo's folder inside the folder set for hires' worktrees (`HireDesk.prepare`). A
    /// registry row naming any other folder (`/Users`, say) gets no desk and keeps no session off a desk.
    static func inHiresFolder(_ hire: FrontDesk.Hire, worktrees: String = Config.shared.hireWorktrees) -> Bool {
        guard !hire.repo.isEmpty, !hire.worktree.isEmpty else { return false }
        // Compared as `HireDesk` keeps them: physical, with no `..` left.
        let plain = { (path: String) in HireDesk.physical((path as NSString).standardizingPath) }
        let worktree = (hire.worktree as NSString).standardizingPath
        let name = (worktree as NSString).lastPathComponent
        guard !name.isEmpty, name != "/", name != ".." else { return false }
        let repo = (hire.repo as NSString).standardizingPath
        let repoName = (repo as NSString).lastPathComponent
        var parents = [((repo as NSString).deletingLastPathComponent as NSString).appendingPathComponent(repoName + "-hires")]
        if !worktrees.isEmpty { parents.append((worktrees as NSString).appendingPathComponent(repoName)) }
        let parent = plain((worktree as NSString).deletingLastPathComponent)
        return parents.contains { plain($0) == parent }
    }

    /// The desks, in the registry's order, each brought in line with it and with what the follow last saw.
    func seated(_ registry: [FrontDesk.Hire], seen: (String) -> T3Desk.Seen? = T3Desk.lastSeen) -> [(session: Session, hire: FrontDesk.Hire)] {
        let open = registry.filter { T3Desks.hasDesk($0) }
        let ids = Set(open.map(\.id))
        byHire = byHire.filter { ids.contains($0.key) }
        hires = Dictionary(open.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        seenDone = seenDone.filter { ids.contains($0) }
        return open.map { hire in
            let session = byHire[hire.id] ?? Session(id: T3Desks.sessionID(of: hire.id), agent: .claude, folder: hire.worktree,
                                                      project: hire.worktree, startedAt: hire.started)
            byHire[hire.id] = session
            let look = seen(hire.id)
            if look?.status != .done { seenDone.remove(hire.id) }
            session.folder = hire.worktree
            session.project = hire.worktree
            session.model = hire.model
            session.state = T3Desks.state(look?.status, registry: hire.state, seen: seenDone.contains(hire.id))
            session.toolActivity = T3Desks.doing(session.state)
            return (session, hire)
        }
    }

    /// The hire at desk `id`, while it has one (as `seated` last found it).
    func hire(ofSession id: String) -> FrontDesk.Hire? {
        guard id.hasPrefix(T3Desks.prefix) else { return nil }
        return hires[String(id.dropFirst(T3Desks.prefix.count))]
    }

    /// The desk was tapped: a done hire's news is seen. Its hire, or nil for a desk that isn't a T3 hire's.
    @discardableResult
    func tapped(session id: String) -> FrontDesk.Hire? {
        guard let hire = hire(ofSession: id) else { return nil }
        if byHire[hire.id]?.state == .unread { seenDone.insert(hire.id) }
        return hire
    }

    /// A desk's colour for its thread: at work blue, waiting for an answer orange, done green till seen. Not seen
    /// yet (as after a restart), it's as the registry says; T3 Code not answering, white.
    static func state(_ status: T3Desk.Status?, registry: FrontDesk.State, seen: Bool) -> SessionState {
        switch status {
        case .working: return .thinking
        case .waiting: return .asking
        case .done: return seen ? .idle : .unread
        case .unknown: return .idle
        case .starting, nil: return registry == .idle ? .idle : .thinking
        }
    }

    /// What its card says it's doing, for its state.
    static func doing(_ state: SessionState) -> String {
        switch state {
        case .thinking: return "At work in T3 Code"
        case .asking: return "Waiting for an answer in T3 Code"
        default: return ""
        }
    }

    /// The controls a terminal's desk has, every one off at a T3 desk: there's no tab to type into.
    static let controlsOff = ["keys", "fire", "mic", "dial", "dictates", "summary", "phoneMic", "conversation"]

    /// Why an office control can't reach desk `name`: nothing types into a hire in T3 Code.
    static func noControls(_ name: String) -> String {
        "\(name) runs in T3 Code: Smolcorp doesn't type into it. Open it in T3 Code, or mail it."
    }

    /// The office's word for the T3 desk (its card): the tag, its thread's status, its branch, and whether it has a
    /// thread to open. No path, no thread id: those stay on this Mac.
    static func deskJSON(_ hire: FrontDesk.Hire) -> [String: Any] {
        let desk = T3Desk.deskJSON(hire)
        return ["tag": T3Desk.tag, "experimental": true, "status": desk["status"] ?? "unknown",
                "branch": hire.branch, "canOpen": desk["canOpen"] ?? false]
    }
}
