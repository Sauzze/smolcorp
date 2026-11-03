import XCTest
@testable import AgentDeck

/// The app's own watches for the hires at work, and the hourly digest: temp folders and a fake clock.
final class BuiltinWatchesTests: XCTestCase {
    private var root: URL!
    private var support: URL!
    private var folder: String!
    private var clock = Date(timeIntervalSince1970: 1_761_000_000)
    private var hires: [FrontDesk.Hire] = []
    private var logged: [EventLog.Entry] = []
    private var on = true
    private var asking: [String] = []
    private var counter = 0

    override func setUpWithError() throws {
        root = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("builtin-\(UUID().uuidString)", isDirectory: true)
        support = root.appendingPathComponent("support", isDirectory: true)
        let rev = root.appendingPathComponent("rev", isDirectory: true)
        try FileManager.default.createDirectory(at: support, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: rev, withIntermediateDirectories: true)
        folder = rev.resolvingSymlinksInPath().path
        hires = [FrontDesk.Hire(id: "abcdef123456", name: "Reviewer", nickname: "Rev", role: "reviewer", folder: folder, state: .working),
                 FrontDesk.Hire(id: "gm0000000001", name: "General Manager", role: "gm", folder: "/gm", state: .working)]
    }

    override func tearDownWithError() throws { try? FileManager.default.removeItem(at: root) }

    private func make() -> Watchers {
        Watchers(folder: support, roots: { self.hires.filter { $0.role != "gm" }.map(\.folder) + [self.root.appendingPathComponent("hires").path] },
                 hires: { self.hires }, log: { self.logged.append($0) },
                 now: { self.clock }, newID: { self.counter += 1; return String(format: "b-%04d", self.counter) },
                 builtins: { self.on }, asking: { self.asking })
    }

    private func write(_ text: String, to name: String = "STATUS.md", append: Bool = false) throws {
        let url = URL(fileURLWithPath: folder).appendingPathComponent(name)
        if append, let handle = try? FileHandle(forWritingTo: url) {
            try handle.seekToEnd()
            try handle.write(contentsOf: Data(text.utf8))
            try handle.close()
        } else {
            try text.write(to: url, atomically: false, encoding: .utf8)
        }
    }

    private func event(_ kind: EventLog.Kind, id: Int, detail: String = "") -> EventLog.Entry {
        EventLog.Entry(id: id, at: clock.timeIntervalSince1970 * 1000, session: "s1", folder: folder, agent: "claude", kind: kind, detail: detail)
    }

    private func later(_ seconds: Double) { clock = clock.addingTimeInterval(seconds) }

    // MARK: Set and taken away

    func testEachHireGetsItsOwnWatchesAndTheGMGetsTheDigestOnly() {
        let watchers = make()
        watchers.sync()
        let own = watchers.all.filter { !$0.builtin.isEmpty }
        XCTAssertEqual(Set(own.filter { $0.hire == "abcdef123456" }.map(\.builtin)), ["needs-review", "status", "denied", "limit", "wait"])
        XCTAssertEqual(own.filter { $0.hire == "gm0000000001" }.map(\.builtin), ["digest"])
        let before = watchers.all.count
        watchers.sync()
        XCTAssertEqual(watchers.all.count, before, "syncing again adds nothing")
    }

    func testTheyAreNotSetOffOrFlagOffAndGoWhenTheHireIsFiredOrTheManagerIsSwitchedOff() {
        on = false
        let watchers = make()
        watchers.sync()
        XCTAssertTrue(watchers.all.isEmpty)

        on = true
        watchers.sync()
        XCTAssertFalse(watchers.all.isEmpty)
        hires[0].state = .fired
        watchers.sync()
        XCTAssertEqual(watchers.all.map(\.builtin), ["digest"], "a fired hire's watches go; the GM's digest stays")
        hires[1].state = .fired
        watchers.sync()
        XCTAssertTrue(watchers.all.isEmpty, "the digest goes when the GM is fired")

        hires[0].state = .working
        hires[1].state = .working
        watchers.sync()
        on = false
        watchers.sync()
        XCTAssertTrue(watchers.all.isEmpty, "switched off, all go")
    }

    func testAGMsOwnWatchesAreKeptAlongsideAndBuiltInsAreNotTakenAwayByHand() throws {
        let watchers = make()
        let mine = try watchers.add(["kind": "time", "target": "x", "minutes": 5], by: "gm")
        watchers.sync()
        let own = try XCTUnwrap(watchers.all.first { !$0.builtin.isEmpty })
        XCTAssertThrowsError(try watchers.remove(own.id)) { XCTAssertEqual(($0 as? FrontDesk.Refusal)?.status, 403) }
        XCTAssertNoThrow(try watchers.remove(mine.id))
        watchers.sync()
        XCTAssertTrue(watchers.all.contains { $0.id == own.id }, "sync keeps what is already set")
    }

    func testTheyOutlastARestartWithoutDoubling() throws {
        let first = make()
        first.sync()
        try write("")
        let count = first.all.count
        let second = make()
        second.sync()
        XCTAssertEqual(second.all.count, count)
        XCTAssertEqual(Set(second.all.map(\.id)), Set(first.all.map(\.id)))
    }

    // MARK: STATUS lines

    func testANeedsReviewLineFiresOnceAndIsMarkedForThePhone() throws {
        try write("NEEDS-REVIEW: old one\nworking\n")
        let watchers = make()
        watchers.sync()
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty, "what was there when it was set never fires")

        try write("NEEDS-REVIEW: T406 ready\n", append: true)
        watchers.tick(inline: true)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
        XCTAssertEqual(logged[0].kind, .watcherFired)
        XCTAssertEqual(logged[0].agent, Watchers.reviewMark)
        XCTAssertTrue(logged[0].detail.contains("Rev") && logged[0].detail.contains("NEEDS-REVIEW: T406 ready"), logged[0].detail)
    }

    /// T406's firing is what T411's list reads: the same line, whose hire may have a colon in its name.
    func testARealNeedsReviewFiringShowsInTheReviewsList() throws {
        hires[0].nickname = "Rev: two"
        try write("start\n")
        let watchers = make()
        watchers.sync()
        try write("- NEEDS-REVIEW: T406 ready: see PR\n", append: true)
        watchers.tick(inline: true)
        let item = try XCTUnwrap(logged.first.flatMap(Reviews.item))
        XCTAssertEqual(item.hire, "Rev: two")
        XCTAssertEqual(item.file, "STATUS.md")
        XCTAssertEqual(item.text, "T406 ready: see PR")
        XCTAssertEqual(Reviews.recent(logged).count, 1)
    }

    func testDoneStopBlockedAndBudgetRequestEachFireOncePerNewLineWithNoMark() throws {
        try write("start\n")
        let watchers = make()
        watchers.sync()
        try write("DONE a\nSTOP need a call\nnotes\nBLOCKED on keys\nBUDGET REQUEST $5\nhalf-done\nDONEISH\n", append: true)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 4, "one for each new line, none lost in a burst: \(logged.map(\.detail))")
        XCTAssertTrue(logged.allSatisfy { $0.agent.isEmpty })
        XCTAssertTrue(logged[0].detail.hasPrefix("status: Rev"), logged[0].detail)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 4)
        try write("- DONE b\n", append: true)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 5, "a list dash may come first")
    }

    func testAStatusFileThatAppearsLaterIsReadFromItsFirstLine() throws {
        let watchers = make()
        watchers.sync()
        watchers.tick(inline: true)
        try FileManager.default.createDirectory(atPath: folder + "/.team", withIntermediateDirectories: true)
        try write("NEEDS-REVIEW: first\n", to: ".team/STATUS.md")
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
    }

    // MARK: Events

    func testARefusedCallFiresForEachOneInTheHiresFolderOnly() {
        let watchers = make()
        watchers.sync()
        watchers.observe(event(.permissionDenied, id: 1, detail: "Bash"))
        watchers.observe(event(.permissionDenied, id: 2, detail: "Edit: a.txt"))
        var other = event(.permissionDenied, id: 3, detail: "Bash")
        other.folder = "/somewhere/else"
        watchers.observe(other)
        XCTAssertEqual(logged.count, 2)
        XCTAssertTrue(logged[0].detail.hasPrefix("denied:"), logged[0].detail)
    }

    func testAUsageLimitNoticeFiresAndAnyOtherNotificationDoesNot() {
        XCTAssertEqual(EventLog.detail(of: ["message": "You've hit your limit · resets 3pm", "tool_name": "x"], kind: .notification), EventLog.limitDetail)
        XCTAssertEqual(EventLog.detail(of: ["message": "Claude usage limit reached. Your limit will reset at 5pm"], kind: .notification), EventLog.limitDetail)
        XCTAssertEqual(EventLog.detail(of: ["message": "secret thing"], kind: .notification), "notification")
        XCTAssertEqual(EventLog.detail(of: ["message": "Claude is waiting for your input"], kind: .notification), "Claude is waiting for your input")

        let watchers = make()
        watchers.sync()
        watchers.observe(event(.notification, id: 1, detail: "notification"))
        XCTAssertTrue(logged.isEmpty)
        watchers.observe(event(.notification, id: 2, detail: EventLog.limitDetail))
        watchers.observe(event(.notification, id: 3, detail: EventLog.limitDetail))
        XCTAssertEqual(logged.count, 2, "each notice, once")
        XCTAssertTrue(logged[0].detail.hasPrefix("limit:"), logged[0].detail)
    }

    func testAQuestionStillWaitingAfterTwoMinutesFiresOncePerWait() {
        let watchers = make()
        watchers.sync()
        asking = [folder]
        watchers.tick(inline: true)
        later(119)
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty)
        later(2)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
        XCTAssertTrue(logged[0].detail.hasPrefix("wait: Rev has waited 2 min"), logged[0].detail)
        later(600)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1, "the same wait does not fire again")

        asking = []
        watchers.tick(inline: true)
        asking = [folder]
        watchers.tick(inline: true)
        later(130)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 2, "a new wait fires again")
    }

    func testAQuestionAnsweredInTimeNeverFires() {
        let watchers = make()
        watchers.sync()
        asking = [folder]
        watchers.tick(inline: true)
        later(90)
        asking = []
        watchers.tick(inline: true)
        later(300)
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty)
    }

    // MARK: The digest

    func testTheDigestComesEveryHourWithWhoDoesWhatAndWhoFinished() throws {
        hires.append(FrontDesk.Hire(id: "bbbbbb111111", name: "Builder", nickname: "Bo", role: "builder", folder: root.appendingPathComponent("bo").path, state: .idle))
        try write("")
        let watchers = make()
        watchers.sync()
        later(59 * 60)
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty)

        try write("- DONE T406\n", append: true) // a list dash still counts as done (the pattern allows it)
        watchers.tick(inline: true)
        asking = [root.appendingPathComponent("bo").path]
        later(2 * 60)
        watchers.tick(inline: true)
        let digests = logged.filter { $0.detail.hasPrefix("digest:") }
        XCTAssertEqual(digests.count, 1)
        XCTAssertEqual(digests[0].detail, "digest: Hourly digest. Working: Rev. Idle: none. Waiting: Bo. Done since last: Rev.")

        later(60 * 60)
        asking = []
        watchers.tick(inline: true)
        let second = logged.filter { $0.detail.hasPrefix("digest:") }
        XCTAssertEqual(second.count, 2, "every hour")
        XCTAssertEqual(second[1].detail, "digest: Hourly digest. Working: Rev. Idle: Bo. Waiting: none. Done since last: none.")
    }

    func testSettingsDefaultToOnWithTheManager() throws {
        XCTAssertTrue(Config().builtinWatches)
        let file = root.appendingPathComponent("config.json")
        try Data(#"{"builtinWatches": false}"#.utf8).write(to: file)
        XCTAssertFalse(Config.load(from: file).builtinWatches)
    }
}
