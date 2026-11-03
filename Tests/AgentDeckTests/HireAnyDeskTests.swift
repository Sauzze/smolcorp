import XCTest
@testable import AgentDeck

/// A hire sits at the free desk the owner chose, with Auto too (desks have
/// always been Auto since the count left setup), and the first free one only
/// when none was chosen.
final class HireAnyDeskTests: XCTestCase {
    private func session(_ id: String) -> Session {
        let sessions = Sessions()
        sessions.apply(["hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": "/work/\(id)"],
                       agent: .claude, tab: "tty:/dev/\(id)", startedIn: "", at: 10)
        return sessions[id]!
    }

    private func keys(_ count: Int, seated: [Int: Session] = [:], pinned: [Int: String] = [:]) -> [Deck.Key] {
        (0..<count).map { Deck.Key(pin: pinned[$0], session: seated[$0]) }
    }

    func testAChosenFreeDeskInTheMiddleIsHeldWhileEarlierOnesAreFree() throws {
        let one = session("a")
        let desks = keys(4, seated: [0: one])
        let choice = Deck.groundHire(desk: "3", keys: desks)
        XCTAssertNil(choice.refusal)
        XCTAssertEqual(choice.held, 3, "not the first free desk, 1")
    }

    func testADeskAfterTheLastIsHeldToo() {
        XCTAssertEqual(Deck.groundHire(desk: "5", keys: keys(2)).held, 5)
        XCTAssertEqual(Deck.groundHire(desk: "8", keys: keys(2)).held, 8)
    }

    /// The first free desk is where a hire with no desk goes, so it is not held (the Go app agrees):
    /// Auto still packs, and a hire there does not leave a gap when an earlier desk is vacated.
    func testTheFirstFreeDeskIsNotHeld() {
        let one = session("a"), two = session("b")
        XCTAssertNil(Deck.groundHire(desk: "0", keys: keys(3)).held, "all free: the first")
        XCTAssertNil(Deck.groundHire(desk: "1", keys: keys(3, seated: [0: one])).held)
        XCTAssertNil(Deck.groundHire(desk: "2", keys: keys(2, seated: [0: one, 1: two])).held, "the desk after the last, none free")
        XCTAssertNil(Deck.groundHire(desk: "1", keys: keys(3, seated: [0: one], pinned: [0: "/work/app"])).held,
                     "a pinned desk is never the first free one")
        XCTAssertEqual(Deck.groundHire(desk: "2", keys: keys(3, seated: [0: one])).held, 2, "not the first free, 1")
        XCTAssertEqual(Deck.groundHire(desk: "3", keys: keys(2, seated: [0: one, 1: two])).held, 3, "past the first free, 2")
    }

    func testAnOccupiedDeskIsRefusedAndAnUnknownOne() {
        let desks = keys(3, seated: [1: session("a")])
        let taken = Deck.groundHire(desk: "1", keys: desks).refusal
        XCTAssertEqual(taken?.status, 409)
        XCTAssertEqual(taken.map { String(decoding: $0.body, as: UTF8.self) }, "Someone's at that desk already.")
        for bad in ["9", "-1", "x", ""] {
            XCTAssertEqual(Deck.groundHire(desk: bad, keys: desks).refusal?.status, 400, bad)
        }
    }

    func testAPinnedDeskHoldsNoDeskAndGivesItsProject() {
        let desks = keys(3, pinned: [2: "/work/app"])
        let choice = Deck.groundHire(desk: "2", keys: desks)
        XCTAssertNil(choice.refusal)
        XCTAssertNil(choice.held)
        XCTAssertEqual(choice.pin, "/work/app")
    }

    func testTheGridReachesTheChosenDeskAndTheHireSitsThere() {
        let hire = session("hire")
        // Desk 4 chosen with the others free: five keys, the hire on the fifth, the rest left to pack.
        let grid = Deck.autoGrid(keys(9), chosen: [(key: 4, session: hire)], others: 2)
        XCTAssertEqual(grid.keys.count, 5)
        XCTAssertEqual(grid.seated.map(\.0), [4])
        XCTAssertEqual(grid.seated.first?.1.id, hire.id)
        // Without a chosen desk, packed: just a key for each.
        let packed = Deck.autoGrid(keys(9), chosen: [], others: 2)
        XCTAssertEqual(packed.keys.count, 2)
        XCTAssertTrue(packed.seated.isEmpty)
    }

    func testAChosenDeskThatIsPinnedOrTakenIsNotHeld() {
        let a = session("a"), b = session("b")
        let pinned = Deck.autoGrid(keys(9, pinned: [2: "/work/app"]), chosen: [(key: 2, session: a)], others: 1)
        XCTAssertTrue(pinned.seated.isEmpty, "a pinned key is its project's")
        let same = Deck.autoGrid(keys(9), chosen: [(key: 3, session: a), (key: 3, session: b)], others: 2)
        XCTAssertEqual(same.seated.map(\.1.id), [a.id], "two hires can't both hold desk 3")
    }

    // MARK: The wiring, on a deck of its own

    /// A deck that restores, saves and tells nothing, laid out by a fresh config (no pins, no floor rules).
    private func quietDeck() -> Deck {
        let deck = Deck(quiet: true)
        deck.layoutConfig = Config()
        return deck
    }

    /// Each new session starts a moment after the last, as sessions are ordered by their start.
    private var clock = 10.0

    private func report(_ deck: Deck, _ id: String, tab: String? = nil, at time: Double? = nil) {
        clock += 1
        let time = time ?? clock
        deck.sessions.apply(["hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": "/work/\(id)"],
                            agent: .claude, tab: tab ?? "tty:/dev/\(id)", startedIn: "", at: time)
    }

    /// The hire's tab is waiting for its session, with the desk chosen, as `hire(at:)` leaves it.
    private func hireAt(_ deck: Deck, desk: Int, id: String = "hired") {
        deck.hiredTabs.append((tab: "tty:/dev/\(id)", role: nil, desk: desk, at: Date(), folder: "/work/\(id)", floor: nil))
        report(deck, id)
    }

    private func desk(of id: String, in deck: Deck) -> Int? {
        deck.layout().keys.firstIndex { $0.session?.id == id }
    }

    /// Without `chosenDesk.insert` where the hire's tab is matched, or the grid seating the chosen first,
    /// the hire packs to desk 1.
    func testAHireAtAMiddleDeckSitsThereWhenTheLayoutIsRecomputed() {
        let deck = quietDeck()
        report(deck, "s0")
        hireAt(deck, desk: 4)
        for _ in 0..<3 { XCTAssertEqual(desk(of: "hired", in: deck), 4) }
        XCTAssertEqual(deck.layout().keys.count, 5)
        // The others take the first free desks, not the hire's.
        report(deck, "s1")
        report(deck, "s2")
        XCTAssertEqual(desk(of: "s0", in: deck), 0)
        XCTAssertEqual(desk(of: "s1", in: deck), 1)
        XCTAssertEqual(desk(of: "s2", in: deck), 2)
        XCTAssertEqual(desk(of: "hired", in: deck), 4)
        // With no desk chosen it packs, as before.
        let packed = quietDeck()
        report(packed, "s0")
        packed.hiredTabs.append((tab: "tty:/dev/h", role: nil, desk: nil, at: Date(), folder: "/work/h", floor: nil))
        report(packed, "h")
        XCTAssertEqual(desk(of: "h", in: packed), 1)
    }

    /// Without `chosenDesk.insert(new)` for a replacement, the hire after /clear packs to desk 1.
    func testAHireKeepsItsDeskAfterClear() {
        let deck = quietDeck()
        report(deck, "s0")
        hireAt(deck, desk: 4)
        XCTAssertEqual(desk(of: "hired", in: deck), 4)
        // /clear: a new session in the same tab replaces the old one.
        report(deck, "cleared", tab: "tty:/dev/hired", at: 20)
        XCTAssertNil(deck.sessions["hired"])
        XCTAssertEqual(desk(of: "cleared", in: deck), 4)
        XCTAssertEqual(desk(of: "s0", in: deck), 0)
    }

    /// Without the `wasDropped` test where the chosen set is filtered, the desk is forgotten while the tab seems closed.
    func testAHireDroppedAndBackKeepsItsDesk() {
        let deck = quietDeck()
        report(deck, "s0")
        hireAt(deck, desk: 4)
        XCTAssertEqual(desk(of: "hired", in: deck), 4)
        deck.sessions.prune(closedTabs: ["tty:/dev/hired"])
        XCTAssertNil(deck.sessions["hired"])
        _ = deck.layout()
        report(deck, "hired", at: Date().timeIntervalSince1970 * 1000 + 5000) // after the drop, which is stamped now
        XCTAssertEqual(desk(of: "hired", in: deck), 4)
    }

    /// Without `chosen` in the saved deck (or its put back), the hire packs to desk 1 after a restart.
    func testAChosenDeskIsKeptAcrossARestart() throws {
        let deck = quietDeck()
        report(deck, "s0")
        hireAt(deck, desk: 4)
        _ = deck.layout()
        let data = try JSONEncoder().encode(deck.savedDeck())
        let saved = try JSONDecoder().decode(Deck.SavedDeck.self, from: data)
        XCTAssertEqual(saved.chosen, ["hired"])
        XCTAssertEqual(saved.keys["hired"], 4)

        let again = quietDeck()
        again.putBack(saved, running: saved.sessions)
        XCTAssertEqual(desk(of: "hired", in: again), 4)
        XCTAssertEqual(desk(of: "s0", in: again), 0)
        // A deck saved before the desk was kept reads as before: the hire is packed in its order.
        let old = try JSONDecoder().decode(Deck.SavedDeck.self, from: Data(#"{"savedAt":0,"sessions":[],"keys":{}}"#.utf8))
        XCTAssertNil(old.chosen)
    }

    /// The desk goes with the session that held it: the next one to sit there is not held to it.
    func testTheChosenDeskGoesWhenItsSessionEnds() {
        let deck = quietDeck()
        report(deck, "s0")
        hireAt(deck, desk: 4)
        XCTAssertEqual(deck.layout().keys.count, 5)
        deck.sessions.apply(["hook_event_name": "SessionEnd", "session_id": "hired"], agent: .claude, tab: "tty:/dev/hired", startedIn: "", at: 40)
        XCTAssertEqual(deck.layout().keys.count, 1, "the grid packs again")
    }
}
