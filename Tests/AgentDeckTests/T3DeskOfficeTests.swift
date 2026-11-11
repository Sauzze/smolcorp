import XCTest
@testable import AgentDeck

/// A hire in T3 Code is a desk in the office (T3D-a, experimental): made from Smolcorp's own registry alone (engine
/// t3, its thread and worktree), never from a tabless hook session, a name or a folder. Its Claude Code's own hook
/// events make no second desk and never land on a terminal's. Nothing types into it; a tap opens its thread in T3
/// Code; Let go is the lane's; and it sits where it sat after a restart, as the registry kept it.
final class T3DeskOfficeTests: XCTestCase {
    private var home: URL!
    private var registry: URL { Paths.support.appendingPathComponent("registry.json") }
    private var keptRegistry: Data?
    private var keptT3 = false
    private let hireID = "abcdef12-0000-4000-8000-00000000d35c"
    private var opened: [URL] = []

    override func setUpWithError() throws {
        home = URL(fileURLWithPath: HireDesk.physical(NSTemporaryDirectory())).appendingPathComponent("t3office-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)
        // The test support folder's registry, put back after: each test starts with none.
        keptRegistry = try? Data(contentsOf: registry)
        try? FileManager.default.removeItem(at: registry)
        keptT3 = Config.shared.t3CodeEnabled
        opened = []
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: home)
        if let keptRegistry { try keptRegistry.write(to: registry) } else { try? FileManager.default.removeItem(at: registry) }
        Config.shared.t3CodeEnabled = keptT3
    }

    // MARK: Stand-ins

    private var worktree: String { home.appendingPathComponent("repo-hires/rev-abcdef").path }
    private var role: Role { Config.shared.roles.first { $0.id != Deck.gmRole } ?? Role.shipped[0] }

    /// A hire in T3 Code as the lane registers it once its thread runs.
    private func t3Hire(_ id: String? = nil, floor: Int? = nil, desk: Int? = nil, state: FrontDesk.State = .working, thread: String = "mcp:abc-1") -> FrontDesk.Hire {
        let id = id ?? hireID
        var hire = FrontDesk.Hire(id: id, name: role.name, nickname: "Rev", role: role.id, folder: home.appendingPathComponent("repo").path,
                                  branch: "hire/rev-\(id.prefix(6))", worktree: id == hireID ? worktree : home.appendingPathComponent("repo-hires/wt-\(id.prefix(6))").path,
                                  repo: home.appendingPathComponent("repo").path, model: "claude-sonnet-5-5", started: 1_761_000_000_000,
                                  state: state, engine: T3Desk.engine, t3Thread: thread, t3Link: "https://app.t3.codes/threads/env-1/mcp%3Aabc-1")
        (hire.t3Floor, hire.t3Desk) = (floor, desk)
        return hire
    }

    /// A deck that restores, saves and tells nothing, laid out by a fresh config, with the hires registered.
    private func deck(_ hires: [FrontDesk.Hire] = []) -> Deck {
        let deck = Deck(quiet: true)
        deck.layoutConfig = Config()
        // No cloud bot: whatever the suite left in the config, only these desks are in the office.
        deck.cloud.configure([])
        for hire in hires { XCTAssertTrue(deck.frontDesk.register(hire)) }
        return deck
    }

    /// A hook event, as `/hook` gives it to the deck.
    private func hook(_ deck: Deck, _ id: String, _ name: String = "UserPromptSubmit", cwd: String, tab: String = "", startedIn: String = "", at time: Double = 10) {
        deck.ingest(["hook_event_name": name, "session_id": id, "cwd": cwd], reportedAs: "claude", tab: tab, tty: "", startedIn: startedIn, at: time)
    }

    private func snapshot(_ deck: Deck) throws -> [String: Any] {
        try XCTUnwrap(JSONSerialization.jsonObject(with: deck.snapshotJSON()) as? [String: Any])
    }

    /// Every desk in the snapshot, the keys' and every floor's.
    private func desks(_ snapshot: [String: Any]) -> [[String: Any]] {
        let keys = (snapshot["keys"] as? [[String: Any]] ?? []).compactMap { $0["session"] as? [String: Any] }
        let floors = (snapshot["floors"] as? [[String: Any]] ?? []).flatMap { $0["desks"] as? [[String: Any]] ?? [] }
        return keys + floors
    }

    private var deskID: String { T3Desks.sessionID(of: hireID) }

    /// A Mac with T3 Code's app and no `t3`: the link opens, nothing runs.
    private func machine() -> T3Code.Machine {
        var machine = T3Code.Machine()
        machine.home = home
        machine.system = []
        machine.path = ""
        machine.appInstalled = { true }
        machine.openApp = {}
        machine.openURL = { [unowned self] url in self.opened.append(url); return true }
        // Older than any kept check, so this machine is asked.
        machine.now = { Date().addingTimeInterval(86_400 * 365) }
        return machine
    }

    /// A fake `t3` in the test's home: `project remove` works, or with `fail` in its folder, refuses.
    private func leaving() throws -> T3Desk.Leaving {
        let bin = home.appendingPathComponent(".local/bin")
        try FileManager.default.createDirectory(at: bin, withIntermediateDirectories: true)
        let script = """
        #!/bin/sh
        [ "$1 $2" = "project remove" ] || exit 9
        [ -f '\(home.appendingPathComponent("fail").path)' ] && { echo "T3 Code isn't running" >&2; exit 1; }
        echo "Removed project p-1"; exit 0
        """
        let t3 = bin.appendingPathComponent("t3")
        try script.write(to: t3, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: t3.path)
        var machine = machine()
        machine.now = Date.init
        return T3Desk.Leaving(machine: machine, working: { _ in [] }, name: { _ in "" }, pause: { _ in })
    }

    /// The deck's answer to `act`, waited for: Let go runs off the main thread and tells the registry on it.
    private func answer(_ act: (@escaping (Response) -> Void) -> Void) -> Response? {
        var got: Response?
        let answered = expectation(description: "answered")
        act { response in
            got = response
            answered.fulfill()
        }
        wait(for: [answered], timeout: 10)
        return got
    }

    private func text(_ response: Response?) -> String { response.map { String(decoding: $0.body, as: UTF8.self) } ?? "" }

    // MARK: The exception: a desk keyed by the registry

    func testARegisteredT3HireIsADeskWithItsRoleNameBranchAndStatus() throws {
        let deck = deck([t3Hire()])
        let shown = try XCTUnwrap(desks(try snapshot(deck)).first { $0["id"] as? String == deskID }, "the hire's desk")
        XCTAssertEqual(shown["name"] as? String, "rev-abcdef", "its worktree's name: the team's nickname is keyed by it")
        XCTAssertEqual((shown["role"] as? [String: Any])?["id"] as? String, role.id, "its role's look")
        XCTAssertEqual(shown["state"] as? String, "thinking", "at work, as the registry says until T3 Code is asked")
        XCTAssertEqual(shown["activity"] as? String, "At work in T3 Code")
        let t3 = try XCTUnwrap(shown["t3"] as? [String: Any])
        XCTAssertEqual(t3["tag"] as? String, T3Desk.tag)
        XCTAssertEqual(t3["experimental"] as? Bool, true)
        XCTAssertEqual(t3["branch"] as? String, "hire/rev-abcdef")
        XCTAssertEqual(t3["canOpen"] as? Bool, true)
        XCTAssertNil(t3["thread"], "no thread id or path leaves the Mac")
        XCTAssertFalse(String(decoding: deck.snapshotJSON(), as: UTF8.self).contains(worktree), "no path in the snapshot")
        // One desk, on the ground floor, at a key.
        XCTAssertEqual(desks(try snapshot(deck)).filter { $0["id"] as? String == deskID }.count, 1)
        XCTAssertTrue(deck.layout().keys.contains { $0.session?.id == deskID })
    }

    func testItsLookFollowsTheThreadWorkingWaitingDone() {
        let hire = t3Hire()
        let desks = T3Desks()
        let look = { (status: T3Desk.Status) in desks.seated([hire], seen: { _ in T3Desk.Seen(status: status, raw: "") }).first?.session }
        XCTAssertEqual(look(.working)?.state, .thinking)
        XCTAssertEqual(look(.waiting)?.state, .asking)
        XCTAssertEqual(look(.waiting)?.activity, "Waiting for an answer in T3 Code")
        XCTAssertEqual(look(.done)?.state, .unread, "done: green till seen")
        XCTAssertNotNil(desks.tapped(session: deskID))
        XCTAssertEqual(look(.done)?.state, .idle, "seen")
        XCTAssertEqual(look(.working)?.state, .thinking)
        XCTAssertEqual(look(.done)?.state, .unread, "done again after more work: green again")
        XCTAssertEqual(look(.unknown)?.state, .idle, "T3 Code didn't answer: white, never made up")
        XCTAssertEqual(T3Desks.state(nil, registry: .idle, seen: false), .idle)
        XCTAssertEqual(T3Desks.state(nil, registry: .working, seen: false), .thinking)
    }

    func testOnlyAnOpenT3HireWithAThreadAndAWorktreeHasADesk() {
        var noThread = t3Hire("00000000-0000-4000-8000-0000000000a1", thread: "")
        noThread.state = .starting
        var noWorktree = t3Hire("00000000-0000-4000-8000-0000000000a2")
        noWorktree.worktree = ""
        let fired = t3Hire("00000000-0000-4000-8000-0000000000a3", state: .fired)
        let failed = t3Hire("00000000-0000-4000-8000-0000000000a4", state: .failed)
        var terminal = t3Hire("00000000-0000-4000-8000-0000000000a5")
        terminal.engine = ""
        let open = t3Hire()
        XCTAssertEqual(T3Desks().seated([noThread, noWorktree, fired, failed, terminal, open]).map(\.session.id), [deskID])
    }

    func testAnUnregisteredTablessSessionIsStillDropped() throws {
        // A plain hire in the registry (no engine) and a tabless session in its folder: no desk, as before.
        var plain = t3Hire("00000000-0000-4000-8000-0000000000b1")
        plain.engine = ""
        plain.t3Thread = ""
        let deck = deck([plain])
        hook(deck, "stray", "SessionStart", cwd: plain.worktree)
        hook(deck, "elsewhere", "SessionStart", cwd: "/work/elsewhere")
        XCTAssertNil(deck.sessions["stray"])
        XCTAssertNil(deck.sessions["elsewhere"])
        XCTAssertTrue(desks(try snapshot(deck)).isEmpty)
        // A T3 hire's name, title or folder alone makes nothing: only its registry entry does.
        hook(deck, "named", "SessionStart", cwd: "/work/rev-abcdef")
        XCTAssertTrue(desks(try snapshot(deck)).isEmpty)
    }

    func testTheHiresOwnClaudeSessionMakesNoSecondDesk() throws {
        let deck = deck([t3Hire()])
        // Its own Claude Code, through your hooks, with no tab: dropped, its desk is the registry's.
        hook(deck, "t3-claude", "SessionStart", cwd: worktree, startedIn: worktree)
        hook(deck, "t3-claude", "PreToolUse", cwd: worktree + "/src", at: 11)
        XCTAssertNil(deck.sessions["t3-claude"])
        XCTAssertEqual(desks(try snapshot(deck)).map { $0["id"] as? String }, [deskID])
        // Even with a tab named (a pane handed on in its environment), it's no desk of its own.
        hook(deck, "t3-claude-2", "SessionStart", cwd: worktree, tab: "tty:/dev/ttys951", at: 12)
        XCTAssertNil(deck.sessions["t3-claude-2"])
        XCTAssertEqual(desks(try snapshot(deck)).map { $0["id"] as? String }, [deskID])
    }

    /// RM5a §7: T3 Code's server started from a terminal hands that terminal's pane to its Claude Code, so the hire's
    /// events come with the terminal's tab. They must not take that terminal's desk (a new session in a tab replaces
    /// the one there).
    func testAT3ServerStartedFromATerminalDoesntStealThatTerminalsDesk() throws {
        let deck = deck([t3Hire()])
        let pane = "tty:/dev/ttys950"
        hook(deck, "terminal", cwd: "/work/server", tab: pane)
        let before = deck.layout().keys.firstIndex { $0.session?.id == "terminal" }
        XCTAssertNotNil(before)
        hook(deck, "t3-claude", "SessionStart", cwd: worktree, tab: pane, startedIn: worktree, at: 20)
        hook(deck, "t3-claude", "UserPromptSubmit", cwd: worktree, tab: pane, at: 21)
        XCTAssertNil(deck.sessions["t3-claude"])
        XCTAssertNotNil(deck.sessions["terminal"], "the terminal's own session is still at its desk")
        XCTAssertEqual(deck.sessions["terminal"]?.tab, pane)
        XCTAssertEqual(deck.layout().keys.firstIndex { $0.session?.id == "terminal" }, before)
        XCTAssertEqual(Set(desks(try snapshot(deck)).compactMap { $0["id"] as? String }), ["terminal", deskID])
        // A session already at a desk that goes into the worktree stays as it was: only a new one is the hire's.
        hook(deck, "terminal", "PreToolUse", cwd: worktree, tab: pane, at: 22)
        XCTAssertEqual(deck.sessions["terminal"]?.folder, worktree)
        // Let go, the worktree is nobody's: a session started there is a desk again, as any.
        XCTAssertTrue(deck.frontDesk.move(hireID, to: .fired))
        hook(deck, "after", "SessionStart", cwd: worktree, tab: "tty:/dev/ttys952", at: 30)
        XCTAssertNotNil(deck.sessions["after"])
    }

    /// FXT3Da N7: a `t3:` id is a T3 desk's alone. A hook event naming one, even with a tab, is no session.
    func testAHookEventCantTakeAT3DesksID() throws {
        let deck = deck([t3Hire()])
        hook(deck, deskID, "SessionStart", cwd: "/work/elsewhere", tab: "tty:/dev/ttys960")
        hook(deck, deskID, "UserPromptSubmit", cwd: "/work/elsewhere", tab: "tty:/dev/ttys960", at: 11)
        XCTAssertNil(deck.sessions[deskID])
        XCTAssertEqual(desks(try snapshot(deck)).filter { $0["id"] as? String == deskID }.count, 1, "the hire's desk, once")
        let sessions = Sessions()
        sessions.apply(["hook_event_name": "SessionStart", "session_id": "t3:anything", "cwd": "/work/x"], agent: .claude,
                       tab: "tty:/dev/ttys961", startedIn: "", at: 1)
        XCTAssertNil(sessions["t3:anything"])
        // Nor is one saved before a restart (RT3Da2 n2).
        sessions.restore([Sessions.Saved(id: "t3:saved", agent: "claude", folder: "/work/x", project: "/work/x", tty: "", tab: "tty:/dev/ttys961",
                                         state: "idle", effort: "", model: "", transcript: "", startedAt: 1, lastEventAt: 1, turnStartedAt: nil)],
                         fresh: true)
        XCTAssertNil(sessions["t3:saved"])
    }

    /// FXT3Da N3: a T3 hire's worktree counts (a desk, and new sessions there kept off every desk) only where Smolcorp
    /// makes hires' worktrees: in `<repo>-hires` beside its repo, or in the repo's folder inside the folder set for them.
    func testOnlyAWorktreeWhereHiresAreMadeCounts() throws {
        let repo = home.appendingPathComponent("repo").path
        var hire = t3Hire()
        let at = { (worktree: String, worktrees: String) -> Bool in
            hire.worktree = worktree
            return T3Desks.inHiresFolder(hire, worktrees: worktrees)
        }
        XCTAssertTrue(at(repo + "-hires/rev-abcdef", ""))
        XCTAssertTrue(at(repo + "-hires/rev-abcdef/", ""))
        XCTAssertTrue(at("/w/hires/repo/rev-abcdef", "/w/hires"), "in the folder set for worktrees")
        XCTAssertTrue(at(repo + "-hires/rev-abcdef", "/w/hires"), "beside the repo still, made before the folder was set")
        XCTAssertFalse(at("/w/hires/repo/rev-abcdef", ""), "no folder set")
        XCTAssertFalse(at("/w/hires/other/rev-abcdef", "/w/hires"), "another repo's")
        XCTAssertFalse(at(repo + "-hires", ""), "the hires' folder itself")
        XCTAssertFalse(at(repo + "-hires/rev-abcdef/src", ""), "a folder inside a worktree")
        XCTAssertFalse(at(repo + "-hires/../repo", ""), "out of it again")
        XCTAssertFalse(at("/Users", ""))
        XCTAssertFalse(at(home.path, ""))
        XCTAssertFalse(at(repo, ""), "the repo itself")
        hire.repo = ""
        XCTAssertFalse(at(repo + "-hires/rev-abcdef", ""), "no repo on record")

        // A registry row naming a broad folder: no desk, and a session you start under it is a desk as any.
        var broad = t3Hire()
        broad.worktree = home.path
        let deck = deck([broad])
        XCTAssertTrue(desks(try snapshot(deck)).isEmpty)
        hook(deck, "mine", "SessionStart", cwd: home.appendingPathComponent("project").path, tab: "tty:/dev/ttys962",
             startedIn: home.appendingPathComponent("project").path)
        XCTAssertNotNil(deck.sessions["mine"])
    }

    // MARK: Nothing types into it

    func testItsControlsAreAbsentAndRefusedNotBroken() throws {
        let deck = deck([t3Hire()])
        let shown = try XCTUnwrap(desks(try snapshot(deck)).first { $0["id"] as? String == deskID })
        for control in T3Desks.controlsOff { XCTAssertEqual(shown[control] as? Bool, false, control) }
        XCTAssertTrue(shown["choice"] is NSNull)
        for action in ["accept", "send", "reject", "esc", "up", "down", "clear", "effort", "button", "scroll", "choose", "fire", "summary", "max"] {
            let got = answer { deck.perform(action, on: self.deskID, query: ["level": "max", "phase": "down"], done: $0) }
            XCTAssertEqual(got?.status, 409, action)
            XCTAssertEqual(text(got), T3Desks.noControls("Rev"), action)
        }
        // The mic pressed: refused too (its release stops nothing that started).
        XCTAssertEqual(answer { deck.perform("mic", on: self.deskID, query: ["phase": "down"], done: $0) }?.status, 409)
        // The phone mic's words, where it's on.
        let phoneMic = Config.shared.phoneMicEnabled
        defer { Config.shared.phoneMicEnabled = phoneMic }
        Config.shared.phoneMicEnabled = true
        XCTAssertEqual(answer { deck.typePhoneDictation("hello", into: self.deskID, done: $0) }?.status, 409)
        // It isn't the sessions' to find: no key, text or dictation can reach it.
        XCTAssertNil(deck.sessions[deskID])
        // Its desk isn't carried to another key, which would pin its worktree.
        hook(deck, "terminal", cwd: "/work/other", tab: "tty:/dev/ttys953")
        let keys = deck.layout().keys
        let key = try XCTUnwrap(keys.firstIndex { $0.session?.id == deskID })
        let other = try XCTUnwrap(keys.firstIndex { $0.session?.id == "terminal" })
        let swapped = answer { deck.perform("swap", on: "", query: ["from": String(other), "to": String(key)], done: $0) }
        XCTAssertEqual(swapped?.status, 409)
        XCTAssertEqual(text(swapped), "Rev runs in T3 Code: its desk stays where it is.")
    }

    func testTheDeskHasNoKeysOrFireInTheOffice() throws {
        let office = try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("Resources/web/office/main.js"), encoding: .utf8)
        // The office already hides a desk's keys and fire where the snapshot says they're off: the T3 desk relies on it.
        XCTAssertTrue(office.contains("if (selectedSession().keys === false) return null;"))
        XCTAssertTrue(office.contains("there.seats[desk].session.fire !== false ? deskButton(desk, 'fire') : null"))
        XCTAssertTrue(office.contains("s.session.t3 && DESKS[desk] && !DESKS[desk].off) items.push([DESKS[desk].y + 5.1, () => drawT3Badge(desk)])"))
    }

    /// FXT3Da N2, N4: the card says the lane's Let go refusal in full, not "Your computer answered 502."; and with
    /// nobody waiting, the desk picked first is one with controls rather than a T3 desk.
    func testTheCardSaysTheLanesRefusalAndAT3DeskIsntPickedFirst() throws {
        let page = try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("Resources/web/deck.html"), encoding: .utf8)
        XCTAssertTrue(page.contains("async function problemText(res, longest = 200) {"))
        XCTAssertTrue(page.contains("t3Say(res ? await problemText(res, 700) : \"Couldn't reach your computer.\");"))
        XCTAssertTrue(page.contains("keyed.find((s) => s.state === 'asking') || keyed.find((s) => !s.t3) || keyed[0]"))
    }

    // MARK: Open and Let go

    func testATapOpensItsOwnThreadInT3Code() throws {
        let deck = deck([t3Hire()])
        deck.t3Machine = machine()
        Config.shared.t3CodeEnabled = false
        let off = answer { deck.focus(self.deskID, done: $0) }
        XCTAssertEqual(off?.status, 409)
        XCTAssertEqual(text(off), DeckRouter.t3Off)
        XCTAssertTrue(opened.isEmpty)
        Config.shared.t3CodeEnabled = true
        XCTAssertEqual(answer { deck.focus(self.deskID, done: $0) }?.status, 204)
        XCTAssertEqual(opened.map(\.absoluteString), ["t3code://threads/env-1/mcp%3Aabc-1"])
    }

    func testLetGoFromTheDeskFreesItAndKeepsTheLanesRefusals() throws {
        let deck = deck([t3Hire()])
        deck.t3Leaving = try leaving()
        // T3 Code won't remove the project: refused, and the desk stays.
        try "".write(to: home.appendingPathComponent("fail"), atomically: true, encoding: .utf8)
        let refused = answer { deck.perform("letgo", on: self.deskID, query: [:], done: $0) }
        XCTAssertEqual(refused?.status, 502)
        XCTAssertTrue(text(refused).hasPrefix("T3 Code didn't remove Rev's project (T3 Code isn't running)"), text(refused))
        XCTAssertTrue(deck.layout().keys.contains { $0.session?.id == deskID })
        try FileManager.default.removeItem(at: home.appendingPathComponent("fail"))
        // Then it goes: fired in the registry, its desk freed.
        let gone = answer { deck.perform("letgo", on: self.deskID, query: [:], done: $0) }
        XCTAssertEqual(gone?.status, 200, text(gone))
        XCTAssertEqual(deck.frontDesk.hires.first?.state, .fired)
        XCTAssertFalse(deck.layout().keys.contains { $0.session?.id == deskID })
        XCTAssertTrue(desks(try snapshot(deck)).isEmpty)
        if let recap = (try? JSONSerialization.jsonObject(with: gone?.body ?? Data()) as? [String: Any])?["recap"] as? String {
            try? FileManager.default.removeItem(atPath: recap)
        }
        // Again: the desk is gone, so there's nothing to let go.
        XCTAssertEqual(answer { deck.perform("letgo", on: self.deskID, query: [:], done: $0) }?.status, 404)
    }

    // MARK: Placement

    func testItSitsAtTheDeskTheRegistryKeptAfterARestart() throws {
        let first = deck([t3Hire(floor: 0, desk: 3)])
        XCTAssertEqual(first.layout().keys.firstIndex { $0.session?.id == deskID }, 3, "the desk held at hire")
        // A restart: a new deck reads the same registry.
        let again = Deck(quiet: true)
        again.layoutConfig = Config()
        again.cloud.configure([])
        XCTAssertEqual(again.layout().keys.firstIndex { $0.session?.id == deskID }, 3)
        // Upstairs: on its floor, at its desk.
        try? FileManager.default.removeItem(at: registry)
        let up = deck([t3Hire(floor: 2, desk: 4)])
        let building = up.building(overflow: up.layout().overflow)
        XCTAssertTrue(building.floors[2].contains { $0.id == deskID })
        XCTAssertEqual(building.desks[deskID], 4)
        XCTAssertEqual((try snapshot(up)["floors"] as? [[String: Any]])?[2]["desks"].flatMap { $0 as? [[String: Any]] }?
            .first { $0["id"] as? String == deskID }?["desk"] as? Int, 4)
    }

    func testWhereItSitsIsKeptInTheRegistryAndAMoveToo() throws {
        let deck = deck([t3Hire()])
        deck.keepT3Seats()
        let key = try XCTUnwrap(deck.layout().keys.firstIndex { $0.session?.id == deskID })
        XCTAssertEqual(deck.frontDesk.hires.first?.t3Floor, 0)
        XCTAssertEqual(deck.frontDesk.hires.first?.t3Desk, key)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: registry)) as? [String: Any])
        let row = try XCTUnwrap((saved["hires"] as? [[String: Any]])?.first)
        XCTAssertEqual(row["t3Desk"] as? Int, key)
        // Put on 3F from its card: there it stays, kept in the registry.
        XCTAssertEqual(deck.move(deskID, to: "2", remember: false).status, 204)
        deck.keepT3Seats()
        XCTAssertEqual(deck.frontDesk.hires.first?.t3Floor, 2)
        XCTAssertTrue(deck.building(overflow: deck.layout().overflow).floors[2].contains { $0.id == deskID })
        XCTAssertFalse(deck.layout().keys.contains { $0.session?.id == deskID })
        // The team pairs the desk with its hire, by the registry's worktree.
        let rows = deck.frontDesk.teamJSON(live: deck.frontDeskLive())["team"] as? [[String: Any]] ?? []
        let mine = try XCTUnwrap(rows.first { $0["id"] as? String == hireID })
        XCTAssertEqual(mine["sessionId"] as? String, deskID)
        XCTAssertEqual(mine["floor"] as? Int, 2)
        XCTAssertEqual(rows.count, 1, "no row of its own for the desk")
        XCTAssertEqual(deck.hire(inSession: deskID)?.id, hireID)
    }

    /// FXT3Da B1 (RT3Da's P1): you already work in the folder the hire was made from. The hire's row is its own desk,
    /// not your session; your session is a row of its own; the desk has no second row.
    func testAnOlderSessionInTheFolderItWasHiredFromIsntItsDesk() throws {
        // A plain hire made in the T3 hire's worktree, listed first, doesn't take the T3 desk either (RT3Da2 n1).
        let plain = FrontDesk.Hire(id: "00000000-0000-4000-8000-0000000000c1", name: role.name, nickname: "", role: role.id, folder: worktree,
                                   branch: "", worktree: "", repo: "", model: "", started: 1_761_000_000_000)
        let deck = deck([plain, t3Hire()])
        let original = home.appendingPathComponent("repo").path
        hook(deck, "owner", "SessionStart", cwd: original, tab: "tty:/dev/ttys963", startedIn: original, at: 5)
        XCTAssertNotNil(deck.sessions["owner"])
        let rows = deck.frontDesk.teamJSON(live: deck.frontDeskLive())["team"] as? [[String: Any]] ?? []
        let mine = try XCTUnwrap(rows.first { $0["id"] as? String == hireID })
        XCTAssertEqual(mine["sessionId"] as? String, deskID)
        XCTAssertEqual(rows.filter { $0["sessionId"] as? String == deskID }.count, 1, "one row for the desk")
        XCTAssertNil(rows.first { $0["id"] as? String == "live:" + deskID }, "no hand-started row for it")
        XCTAssertEqual(rows.first { $0["id"] as? String == plain.id }?["sessionId"] as? String, "", "the plain hire claims no T3 desk")
        let yours = try XCTUnwrap(rows.first { $0["sessionId"] as? String == "owner" })
        XCTAssertEqual(yours["registered"] as? Bool, false)
        XCTAssertEqual(rows.count, 3)
        XCTAssertEqual(deck.hire(inSession: deskID)?.id, hireID)
        XCTAssertNil(deck.hire(inSession: "owner"))
    }

    func testOnlyAT3HiresRecordSaysWhereItSits() throws {
        var plain = t3Hire()
        plain.engine = ""
        plain.t3Thread = ""
        plain.t3Link = ""
        let written = String(decoding: try JSONEncoder().encode(plain), as: UTF8.self)
        XCTAssertFalse(written.contains("t3Floor"))
        XCTAssertFalse(written.contains("t3Desk"))
        let t3 = t3Hire(floor: 1, desk: 2)
        let back = try JSONDecoder().decode(FrontDesk.Hire.self, from: JSONEncoder().encode(t3))
        XCTAssertEqual(back.t3Floor, 1)
        XCTAssertEqual(back.t3Desk, 2)
        XCTAssertEqual(back, t3)
    }
}
