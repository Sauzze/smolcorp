import XCTest
@testable import AgentDeck

/// Headless claude runs, started with SMOLCORP_HEADLESS=1: named by their
/// session ID, never taken for a terminal's tab, and given desks upstairs.
final class HeadlessTests: XCTestCase {
    private func event(_ name: String, _ id: String = "abc-123", _ fields: [String: Any] = [:]) -> [String: Any] {
        ["hook_event_name": name, "session_id": id, "cwd": "/work/api"].merging(fields) { $1 }
    }

    /// A session store with one headless run in it, `pid` its agent's process.
    private func running(_ id: String = "abc-123", pid: Int32? = nil, at time: Double = 100) -> Sessions {
        let sessions = Sessions()
        sessions.apply(event("UserPromptSubmit", id), agent: .claude, tab: "headless:\(id)", startedIn: "", pid: pid, at: time)
        return sessions
    }

    func testAHeadlessRunSaysWhoHiredIt() {
        let sessions = Sessions()
        sessions.apply(event("UserPromptSubmit", "s1", ["cwd": "/work/api/src"]), agent: .claude, tab: "tty:/dev/ttys003",
                       startedIn: "/work/api", at: 10)
        for (id, parent) in [("h0", "/work/api/"), ("h1", "/elsewhere/tool"), ("h2", "")] {
            sessions.apply(event("UserPromptSubmit", id), agent: .claude, tab: "headless:\(id)", startedIn: "", pid: 42, parent: parent, at: 20)
        }
        let all = sessions.all
        let hired = Deck.hiredBy(sessions["h0"]!, among: all)
        XCTAssertTrue(hired?.session === sessions["s1"])
        XCTAssertEqual(hired?.name, "api")
        let elsewhere = Deck.hiredBy(sessions["h1"]!, among: all)
        XCTAssertNil(elsewhere?.session)
        XCTAssertEqual(elsewhere?.name, "tool")
        XCTAssertNil(Deck.hiredBy(sessions["h2"]!, among: all))
        // Kept over a restart.
        let restored = Sessions()
        restored.restore(sessions.saved, fresh: true)
        XCTAssertEqual(restored["h1"]?.parentFolder, "/elsewhere/tool")
    }

    func testAHeadlessAddressIsReadInItsOwnShapeOnly() {
        let address = TabAddress("headless:abc-123")
        XCTAssertEqual(address?.kind, "headless")
        XCTAssertEqual(address?.id, "abc-123")
        XCTAssertEqual(address?.isHeadless, true)
        XCTAssertEqual(address?.text, "headless:abc-123")
        for bad in ["headless:", "headless:abc 123", "headless:abc@sock", "headless:../x", "headless:abc_123",
                    "headless:abc\r\nX-Other: 1", "headless:é"] {
            XCTAssertNil(TabAddress(bad), bad)
        }
        XCTAssertFalse(TabAddress.isHeadless("tty:/dev/ttys003"))
    }

    func testAHeadlessRunHasNothingToDriveOrWatch() {
        let address = TabAddress("headless:abc-123")!
        XCTAssertNil(Terminals.driver(for: address))
        XCTAssertEqual(Terminals.abilities(of: address.text), .none)
        XCTAssertNil(Terminals.ttyInAddress(of: address.text))
        XCTAssertNil(Terminals.tty(of: address.text))
    }

    func testAHeadlessRunIsNeverTakenForAnotherSessionsTab() {
        let sessions = running("abc-123")
        // Another headless run, and an interactive session, each keep their own place.
        sessions.apply(event("UserPromptSubmit", "def-456"), agent: .claude, tab: "headless:def-456", startedIn: "", at: 200)
        sessions.apply(event("UserPromptSubmit", "s-tty"), agent: .claude, tab: "tty:/dev/ttys003", startedIn: "", at: 300)
        XCTAssertNotNil(sessions["abc-123"])
        XCTAssertNotNil(sessions["def-456"])
        XCTAssertEqual(sessions["s-tty"]?.isHeadless, false)
        XCTAssertTrue(sessions.takeReplacements().isEmpty)
    }

    func testAHeadlessRunIsNeverPrunedForAClosedTab() {
        let sessions = running()
        sessions.prune(closedTabs: ["headless:abc-123"])
        XCTAssertNotNil(sessions["abc-123"])
    }

    func testARunWhoseAgentIsAliveIsKept() {
        let now = Date().timeIntervalSince1970 * 1000
        let sessions = running(pid: getpid(), at: now)
        XCTAssertEqual(sessions["abc-123"]?.pid, getpid())
        sessions.prune(now: now)
        XCTAssertNotNil(sessions["abc-123"])
    }

    func testARunWhoseAgentExitedIsPruned() throws {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/true")
        try process.run()
        process.waitUntilExit()
        let sessions = running(pid: process.processIdentifier)
        XCTAssertTrue(Sessions.processGone(process.processIdentifier))
        sessions.prune(now: Date().timeIntervalSince1970 * 1000)
        XCTAssertNil(sessions["abc-123"])
    }

    func testAQuietRunWithoutAPIDIsPrunedAfterTenMinutes() {
        let sessions = running(at: 1000)
        sessions.apply(event("UserPromptSubmit", "s-tty"), agent: .claude, tab: "tty:/dev/ttys003", startedIn: "", at: 1000)
        sessions.prune(now: 1000 + Sessions.headlessQuiet, isGone: { _ in true })
        XCTAssertNotNil(sessions["abc-123"], "pruned before ten quiet minutes")
        sessions.prune(now: 1001 + Sessions.headlessQuiet, isGone: { _ in true })
        XCTAssertNil(sessions["abc-123"])
        XCTAssertNotNil(sessions["s-tty"], "a session in a tab was pruned for being quiet")
    }

    /// A process that's still there may be something else's, which outlives the run: an hour's quiet ends it too.
    func testAQuietRunWithAPIDStillThereIsPrunedAfterAnHour() {
        let sessions = running(pid: 4242, at: 1000)
        sessions.prune(now: 1000 + Sessions.headlessQuiet + 1, isGone: { _ in false })
        XCTAssertNotNil(sessions["abc-123"], "pruned after ten minutes though its process is there")
        sessions.prune(now: 1000 + Sessions.headlessQuietWithPID, isGone: { _ in false })
        XCTAssertNotNil(sessions["abc-123"], "pruned before an hour's quiet")
        sessions.prune(now: 1001 + Sessions.headlessQuietWithPID, isGone: { _ in false })
        XCTAssertNil(sessions["abc-123"])
    }

    func testSessionEndEndsAHeadlessRun() {
        let sessions = running()
        sessions.apply(event("SessionEnd", "abc-123", ["reason": "other"]), agent: .claude, tab: "headless:abc-123",
                       startedIn: "", at: 200)
        XCTAssertNil(sessions["abc-123"])
    }

    func testHeadlessRunsSitUpstairsTwelveAtMost() {
        let sessions = Sessions()
        for i in 0..<14 {
            sessions.apply(event("UserPromptSubmit", "h\(i)"), agent: .claude, tab: "headless:h\(i)", startedIn: "",
                           at: Double(100 + i))
        }
        sessions.apply(event("UserPromptSubmit", "s-tty"), agent: .claude, tab: "tty:/dev/ttys003", startedIn: "", at: 50)
        XCTAssertEqual(Deck.downstairs(sessions.all).map(\.id), ["s-tty"])
        let upstairs = Deck.upstairs(of: sessions.all)
        XCTAssertEqual(upstairs.desks.map(\.id), (0..<12).map { "h\($0)" })
        XCTAssertEqual(upstairs.overflow, 2)
        XCTAssertEqual(Deck.upstairs(of: []).overflow, 0)
    }

    func testTheViewerSeesOnlyAName() {
        let sessions = running()
        let shown = ViewerSnapshot.upstairs(sessions.all, overflow: 3, hideNames: false)
        let hidden = ViewerSnapshot.upstairs(sessions.all, overflow: 3, hideNames: true)
        XCTAssertEqual(shown["overflow"] as? Int, 3)
        XCTAssertEqual(shown["desks"] as? [[String: String]], [["name": "api", "state": "thinking"]])
        XCTAssertEqual(hidden["desks"] as? [[String: String]], [["name": "Upstairs 1", "state": "thinking"]])
        // And who hired it, by name only, as the deck hands it over.
        let hired = ViewerSnapshot.upstairs(sessions.all, overflow: 0, hideNames: true) { _ in "Desk 2" }
        XCTAssertEqual((hired["desks"] as? [[String: Any]])?.first?["hiredBy"] as? [String: String], ["name": "Desk 2"])
        // The viewer's own check, which now looks upstairs too, finds nothing that mustn't leave.
        XCTAssertEqual(ViewerSelfTest.redactionProblems(hideNames: true), [])
        XCTAssertEqual(ViewerSelfTest.redactionProblems(hideNames: false), [])
    }

    func testAHooksPIDIsDigitsOnly() {
        XCTAssertEqual(DeckRouter.pid("4242"), 4242)
        for bad in [nil, "", "42a", "-5", "0", " 42", "99999999999", "٤٢"] { XCTAssertNil(DeckRouter.pid(bad), bad ?? "nil") }
    }
}
