import XCTest
@testable import AgentDeck

/// The GM's watchers: temp folders and a fake clock; nothing is woken, and nothing is typed.
final class WatchersTests: XCTestCase {
    private var root: URL!
    private var support: URL!
    private var hireFolder: String!
    private var clock = Date(timeIntervalSince1970: 1_761_000_000)
    private var hires: [FrontDesk.Hire] = []
    private var logged: [EventLog.Entry] = []
    private var woken: [String] = []
    private var counter = 0

    override func setUpWithError() throws {
        root = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("watchers-\(UUID().uuidString)", isDirectory: true)
        support = root.appendingPathComponent("support", isDirectory: true)
        let folder = root.appendingPathComponent("rev", isDirectory: true)
        try FileManager.default.createDirectory(at: support, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        // /var is a link to /private/var: use the plain path, as the watchers do.
        hireFolder = folder.resolvingSymlinksInPath().path
        hires = [FrontDesk.Hire(id: "abcdef123456", name: "Reviewer", nickname: "Rev", role: "reviewer", folder: hireFolder, state: .working),
                 FrontDesk.Hire(id: "gm0000000001", name: "General Manager", role: "gm", folder: "/gm", state: .working)]
    }

    override func tearDownWithError() throws { try? FileManager.default.removeItem(at: root) }

    private func make(in folder: URL? = nil, log: ((EventLog.Entry) -> Void)? = nil) -> Watchers {
        Watchers(folder: folder ?? support, roots: { self.hires.filter { $0.role != "gm" }.map(\.folder) + [self.root.appendingPathComponent("hires").path] },
                 hires: { self.hires }, log: log ?? { self.logged.append($0) }, wake: { self.woken.append($0) },
                 now: { self.clock }, newID: { self.counter += 1; return String(format: "w-%04d", self.counter) })
    }

    private func write(_ text: String, to name: String = "STATUS.md", append: Bool = false) throws {
        let url = URL(fileURLWithPath: hireFolder).appendingPathComponent(name)
        if append, let handle = try? FileHandle(forWritingTo: url) {
            try handle.seekToEnd()
            try handle.write(contentsOf: Data(text.utf8))
            try handle.close()
        } else {
            try text.write(to: url, atomically: false, encoding: .utf8)
        }
    }

    private func add(_ watchers: Watchers, _ object: [String: Any], by: String = "gm") throws -> Watchers.Watch {
        try watchers.add(object, by: by)
    }

    private func refusal(_ watchers: Watchers, _ object: [String: Any]) -> Int? {
        do { _ = try watchers.add(object, by: "gm"); return nil } catch let refusal as FrontDesk.Refusal { return refusal.status } catch { return -1 }
    }

    private var path: String { hireFolder + "/STATUS.md" }

    private func event(_ kind: EventLog.Kind, id: Int, folder: String? = nil, detail: String = "", at: Double? = nil) -> EventLog.Entry {
        EventLog.Entry(id: id, at: at ?? clock.timeIntervalSince1970 * 1000, session: "s1", folder: folder ?? hireFolder, agent: "claude", kind: kind, detail: detail)
    }

    // MARK: Files

    func testOnlyNewMatchingLinesFireAndWhatWasThereNeverDoes() throws {
        try write("DONE old one\nworking\n")
        let watchers = make()
        _ = try add(watchers, ["kind": "file", "target": path, "pattern": #"\bDONE\b"#, "repeat": true])
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty, "the line that was there when it was set must not fire")

        try write("nothing here\n", append: true)
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty)

        try write("DONE T405\nDONE again\n", append: true)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
        XCTAssertEqual(logged[0].kind, .watcherFired)
        XCTAssertTrue(logged[0].detail.contains("STATUS.md: DONE T405 (+1 more)"), logged[0].detail)
        XCTAssertEqual(logged[0].folder, hireFolder)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1, "the same lines don't fire twice")
    }

    func testALineWithoutItsNewlineWaitsForIt() throws {
        try write("")
        let watchers = make()
        _ = try add(watchers, ["kind": "file", "target": path, "pattern": "DONE"])
        try write("DO", append: true)
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty)
        try write("NE\n", append: true)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
    }

    func testARotatedOrShortenedFileStartsAgainFromItsEndAndNeverReFiresOldLines() throws {
        try write("one\n")
        let watchers = make()
        _ = try add(watchers, ["kind": "file", "target": path, "pattern": "DONE", "repeat": true])

        // Rotated: moved aside, a new file in its place that already holds a DONE.
        try FileManager.default.moveItem(atPath: path, toPath: path + ".1")
        try write("DONE in the new file\n")
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty, "a new file is taken from its end")
        try write("DONE after the rotation\n", append: true)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
        XCTAssertTrue(logged[0].detail.contains("after the rotation"))

        // Shortened: rewritten smaller, the old DONE lines are not seen again.
        try write("DONE x\nDONE y\nDONE z\n", append: true)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 2)
        try write("DONE y\n")
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 2, "truncated: from its end, nothing old fires")
        try write("DONE fresh\n", append: true)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 3)
    }

    func testAFileRewrittenInPlaceToALongerTextNeverReFiresAndOnlyNewLinesDo() throws {
        try write("DONE a\nDONE b\n")
        let watchers = make()
        _ = try add(watchers, ["kind": "file", "target": path, "pattern": "DONE", "repeat": true])
        // Same file (truncated and written again), longer than before, with DONE lines in it.
        try write("DONE old one\nDONE old two\nDONE old three\n")
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty, "rewritten in place: taken from its end, nothing old fires")
        try write("DONE c\n", append: true)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
        XCTAssertTrue(logged[0].detail.contains("DONE c"))
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
    }

    func testAFileThatComesLaterIsReadFromItsFirstLine() throws {
        let watchers = make()
        _ = try add(watchers, ["kind": "file", "target": path, "pattern": "DONE"])
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty)
        try write("DONE first thing\n")
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
    }

    func testAOnceWatchFiresOnceAndStaysTillTakenAway() throws {
        try write("")
        let watchers = make()
        let watch = try add(watchers, ["kind": "file", "target": path, "pattern": "DONE"])
        try write("DONE\n", append: true)
        watchers.tick(inline: true)
        try write("DONE\n", append: true)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
        XCTAssertEqual(watchers.all.first?.fired, 1)
        XCTAssertEqual(watchers.json.first?["active"] as? Bool, false)
        try watchers.remove(watch.id)
        XCTAssertTrue(watchers.all.isEmpty)
        XCTAssertThrowsError(try watchers.remove(watch.id))
    }

    func testOnlyFoldersTheAppManagesCanBeWatched() throws {
        let watchers = make()
        try write("x\n")
        try write("KEY=1\n", to: ".env")
        XCTAssertEqual(refusal(watchers, ["kind": "file", "target": "/etc/hosts", "pattern": "x"]), 403)
        XCTAssertEqual(refusal(watchers, ["kind": "file", "target": hireFolder + "/../outside.md", "pattern": "x"]), 403)
        XCTAssertEqual(refusal(watchers, ["kind": "file", "target": hireFolder + "/.env", "pattern": "x"]), 403)
        XCTAssertEqual(refusal(watchers, ["kind": "file", "target": support.path + "/gm-token", "pattern": "x"]), 403)
        XCTAssertEqual(refusal(watchers, ["kind": "file", "target": path, "pattern": "("]), 400)
        XCTAssertEqual(refusal(watchers, ["kind": "file", "target": path]), 400)
        // The GM's own folder is no hire's.
        XCTAssertEqual(refusal(watchers, ["kind": "file", "target": "/gm/STATUS.md", "pattern": "x"]), 403)
        XCTAssertNil(refusal(watchers, ["kind": "file", "target": path, "pattern": "x"]))
        // A short path is inside the named hire's folder; one that climbs out is refused.
        XCTAssertNil(refusal(watchers, ["kind": "file", "hire": "Rev", "target": "STATUS.md", "pattern": "x"]))
        XCTAssertEqual(refusal(watchers, ["kind": "file", "hire": "Rev", "target": "../x.md", "pattern": "x"]), 403)
    }

    func testAHomeFolderOrAboveItIsNoRootAndKeyFoldersAreNeverWatched() throws {
        let home = root.appendingPathComponent("home", isDirectory: true)
        let ssh = home.appendingPathComponent(".ssh", isDirectory: true)
        let project = home.appendingPathComponent("Projects/app", isDirectory: true)
        for folder in [ssh, project, home.appendingPathComponent(".config/x", isDirectory: true), home.appendingPathComponent("w/.git", isDirectory: true)] {
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        }
        let house = home.resolvingSymlinksInPath().path
        let key = ssh.resolvingSymlinksInPath().path + "/id_ed25519"
        let notes = project.resolvingSymlinksInPath().path + "/STATUS.md"
        // The home folder, and a folder above it, as a hire's folder: nothing inside is watchable.
        XCTAssertNil(Watchers.inside(key, roots: [house], home: house))
        XCTAssertNil(Watchers.inside(notes, roots: [house], home: house))
        XCTAssertNil(Watchers.inside(notes, roots: [(house as NSString).deletingLastPathComponent], home: house))
        XCTAssertNil(Watchers.inside(notes, roots: ["/"], home: house))
        // A hire's folder inside the home folder still works, but never its key or settings folders.
        XCTAssertEqual(Watchers.inside(notes, roots: [project.resolvingSymlinksInPath().path], home: house), notes)
        for part in [".ssh", ".gnupg", ".aws", ".git", ".config", ".env.local"] {
            XCTAssertNil(Watchers.inside(hireFolder + "/\(part)/f", roots: [hireFolder], home: house), part)
        }
        XCTAssertNil(Watchers.inside(house + "/w/.git/config", roots: [house + "/w"], home: house))
        XCTAssertEqual(Watchers.inside(hireFolder + "/.claude-notes.md", roots: [hireFolder], home: house), hireFolder + "/.claude-notes.md")
    }

    func testALinkOutOfTheFolderIsNotFollowedEvenIfSwappedInLater() throws {
        let secret = root.appendingPathComponent("secret.md")
        try "DONE secret\n".write(to: secret, atomically: true, encoding: .utf8)
        try write("")
        let watchers = make()
        XCTAssertEqual(refusal(watchers, ["kind": "file", "target": hireFolder + "/link.md", "pattern": "DONE"]), nil, "not a link yet")
        try FileManager.default.createSymbolicLink(atPath: hireFolder + "/link2.md", withDestinationPath: secret.path)
        XCTAssertEqual(refusal(watchers, ["kind": "file", "target": hireFolder + "/link2.md", "pattern": "DONE"]), 403)

        // Set on a plain file, then the file is replaced by a link out.
        let watch = try add(watchers, ["kind": "file", "target": path, "pattern": "DONE", "repeat": true])
        try FileManager.default.removeItem(atPath: path)
        try FileManager.default.createSymbolicLink(atPath: path, withDestinationPath: secret.path)
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty, watch.id)
    }

    // MARK: Events, idle, exit, time

    func testAnEventWatchFiresOnItsKindFolderAndPattern() throws {
        let watchers = make()
        _ = try add(watchers, ["kind": "event", "target": "permissionDenied", "folder": hireFolder, "pattern": "^Bash", "repeat": true])
        XCTAssertEqual(refusal(watchers, ["kind": "event", "target": "watcherFired"]), 400)
        XCTAssertEqual(refusal(watchers, ["kind": "event", "target": "nothing"]), 400)

        watchers.observe(event(.idle, id: 1))
        watchers.observe(event(.permissionDenied, id: 2, folder: "/elsewhere", detail: "Bash: rm"))
        watchers.observe(event(.permissionDenied, id: 3, detail: "Edit: /x"))
        XCTAssertTrue(logged.isEmpty)
        watchers.observe(event(.permissionDenied, id: 4, folder: hireFolder + "/sub", detail: "Bash: rm"))
        XCTAssertEqual(logged.count, 1)
        XCTAssertTrue(logged[0].detail.contains("permissionDenied in sub: Bash: rm"), logged[0].detail)
        // What it raised is never seen as an event by another watch (no loop).
        watchers.observe(logged[0])
        XCTAssertEqual(logged.count, 1)
    }

    func testIdleFiresOnceAfterTheQuietTimeAndNotForALaterWork() throws {
        let watchers = make()
        let begin = clock.timeIntervalSince1970 * 1000
        _ = try add(watchers, ["kind": "idle", "target": "Rev", "minutes": 10])
        watchers.observe(event(.idle, id: 5, at: begin))
        clock = clock.addingTimeInterval(9 * 60)
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty, "not quiet long enough")
        clock = clock.addingTimeInterval(61)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
        XCTAssertTrue(logged[0].detail.contains("rev has been idle for 10 min"), logged[0].detail)

        // Repeating: a new quiet spell fires again; one the hire went back to work in, not.
        let again = make(in: root.appendingPathComponent("support2"))
        _ = try add(again, ["kind": "idle", "target": "Rev", "minutes": 1, "repeat": true])
        again.observe(event(.idle, id: 6, at: clock.timeIntervalSince1970 * 1000))
        again.observe(event(.prompt, id: 7, at: clock.timeIntervalSince1970 * 1000))
        clock = clock.addingTimeInterval(120)
        again.tick(inline: true)
        XCTAssertEqual(logged.count, 1, "it went back to work")
        again.observe(event(.idle, id: 8, at: clock.timeIntervalSince1970 * 1000))
        clock = clock.addingTimeInterval(61)
        again.tick(inline: true)
        again.tick(inline: true)
        XCTAssertEqual(logged.count, 2, "once per quiet spell")
    }

    func testIdleStillFiresWhenClaudeCodesNotificationComesAfterTheIdleEvent() throws {
        let watchers = make()
        let begin = clock.timeIntervalSince1970 * 1000
        _ = try add(watchers, ["kind": "idle", "target": "Rev", "minutes": 1])
        watchers.observe(event(.idle, id: 5, at: begin))
        // About a minute later: "Claude is waiting for your input".
        clock = clock.addingTimeInterval(60)
        watchers.observe(event(.notification, id: 6, detail: "Claude is waiting for your input"))
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1, "the notification is not work")
        XCTAssertTrue(logged[0].detail.contains("rev has been idle for 1 min"), logged[0].detail)
    }

    func testIdleDoesNotFireOnceWorkStartsAgain() throws {
        for work in [EventLog.Kind.prompt, .sessionStart, .sessionStop, .permissionAsked, .permissionDenied] {
            let watchers = make(in: root.appendingPathComponent("support-\(work.rawValue)"))
            let before = logged.count
            _ = try add(watchers, ["kind": "idle", "target": "Rev", "minutes": 1])
            watchers.observe(event(.idle, id: 5))
            watchers.observe(event(.notification, id: 6))
            watchers.observe(event(work, id: 7))
            clock = clock.addingTimeInterval(120)
            watchers.tick(inline: true)
            XCTAssertEqual(logged.count, before, "\(work.rawValue) ends the idle time")
        }
    }

    func testIdleFiresOncePerIdleSpellEvenWithNotificationsBetween() throws {
        let watchers = make()
        _ = try add(watchers, ["kind": "idle", "target": "Rev", "minutes": 1, "repeat": true])
        watchers.observe(event(.idle, id: 5))
        watchers.observe(event(.notification, id: 6))
        clock = clock.addingTimeInterval(61)
        for _ in 0..<3 { watchers.tick(inline: true) }
        XCTAssertEqual(logged.count, 1)
        watchers.observe(event(.notification, id: 7))
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1, "the same spell")
        watchers.observe(event(.prompt, id: 8))
        watchers.observe(event(.idle, id: 9))
        clock = clock.addingTimeInterval(61)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 2, "a new spell")
    }

    func testAnIdleWatchOnAGoneHireBecomesInactive() throws {
        let watchers = make()
        let watch = try add(watchers, ["kind": "idle", "target": "Rev", "minutes": 1])
        watchers.tick(inline: true)
        XCTAssertTrue(watchers.all.first { $0.id == watch.id }!.active)
        hires[0].state = .fired
        watchers.tick(inline: true)
        XCTAssertFalse(watchers.all.first { $0.id == watch.id }!.active, "a fired hire's watch is over")
        XCTAssertTrue(logged.isEmpty, "it didn't fire")
        // A repeating one too, and a hire that failed.
        hires[0].state = .working
        let again = try add(watchers, ["kind": "idle", "target": "Rev", "minutes": 1, "repeat": true])
        hires[0].state = .failed
        watchers.tick(inline: true)
        XCTAssertFalse(watchers.all.first { $0.id == again.id }!.active)
    }

    func testAnExitWatchFiresWhenTheHireIsFiredOrGone() throws {
        let watchers = make()
        XCTAssertEqual(refusal(watchers, ["kind": "exit", "target": "nobody"]), 404)
        _ = try add(watchers, ["kind": "exit", "target": "Reviewer"])
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty)
        hires[0].state = .fired
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
        XCTAssertTrue(logged[0].detail.contains("Rev is gone (fired)"), logged[0].detail)
        XCTAssertEqual(refusal(watchers, ["kind": "exit", "target": "Rev"]), 409, "already gone")
    }

    func testAnExitWatchFiresOnceEvenWhenAskedToRepeat() throws {
        let watchers = make()
        _ = try add(watchers, ["kind": "exit", "target": "Reviewer", "repeat": true])
        hires[0].state = .fired
        for _ in 0..<3 { watchers.tick(inline: true) }
        XCTAssertEqual(logged.count, 1)
    }

    func testATimeWatchFiresAfterItsMinutesAndRepeatsFromNow() throws {
        let watchers = make()
        XCTAssertEqual(refusal(watchers, ["kind": "time"]), 400)
        XCTAssertEqual(refusal(watchers, ["kind": "time", "minutes": 0.5, "repeat": true]), 400)
        _ = try add(watchers, ["kind": "time", "target": "look at the build", "minutes": 5])
        _ = try add(watchers, ["kind": "time", "minutes": 2, "repeat": true])
        clock = clock.addingTimeInterval(119)
        watchers.tick(inline: true)
        XCTAssertTrue(logged.isEmpty)
        clock = clock.addingTimeInterval(2)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 1)
        // Asleep for an hour: one more firing, not thirty.
        clock = clock.addingTimeInterval(3600)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 3, "the once watch (5 min) and one repeat")
        watchers.tick(inline: true)
        XCTAssertEqual(logged.count, 3)
        XCTAssertTrue(logged.contains { $0.detail.contains("look at the build") })
    }

    // MARK: Kept and isolated

    func testWatchesSurviveARestartAndOldLinesStillNeverFire() throws {
        try write("DONE before\n")
        let first = make()
        _ = try add(first, ["kind": "file", "target": path, "pattern": "DONE", "repeat": true])
        _ = try add(first, ["kind": "time", "minutes": 60])
        try write("DONE while the app was off\n", append: true)

        let second = make()
        XCTAssertEqual(second.all.map(\.kind), [.file, .time])
        XCTAssertEqual(second.all.map(\.id), ["w-0001", "w-0002"])
        second.tick(inline: true)
        XCTAssertEqual(logged.count, 1, "a line that came while the app was off fires once, the one before never")
        XCTAssertTrue(logged[0].detail.contains("while the app was off"))
        let third = make()
        third.tick(inline: true)
        XCTAssertEqual(logged.count, 1, "and not again after another restart")
        let mode = (try FileManager.default.attributesOfItem(atPath: support.appendingPathComponent("watchers.json").path)[.posixPermissions] as? NSNumber)?.intValue
        XCTAssertEqual(mode, 0o600)
    }

    func testABrokenFileIsSetAsideNotWiped() throws {
        try Data("not json".utf8).write(to: support.appendingPathComponent("watchers.json"))
        let watchers = make()
        XCTAssertTrue(watchers.all.isEmpty)
        XCTAssertTrue(FileManager.default.fileExists(atPath: support.appendingPathComponent("watchers.broken.json").path))
    }

    func testATestCopyKeepsItsWatchesAndFiringsToItself() throws {
        // Two copies of the app: each its own support folder, its own event log, its own GM.
        var realLog: [EventLog.Entry] = [], copyLog: [EventLog.Entry] = []
        let copySupport = root.appendingPathComponent("copy-support", isDirectory: true)
        try FileManager.default.createDirectory(at: copySupport, withIntermediateDirectories: true)
        let real = make(in: support, log: { realLog.append($0) })
        let copy = make(in: copySupport, log: { copyLog.append($0) })
        _ = try add(copy, ["kind": "time", "minutes": 1])
        XCTAssertTrue(real.all.isEmpty, "the copy's watches are not the real one's")
        XCTAssertTrue(FileManager.default.fileExists(atPath: copySupport.appendingPathComponent("watchers.json").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: support.appendingPathComponent("watchers.json").path))
        clock = clock.addingTimeInterval(61)
        real.tick(inline: true)
        copy.tick(inline: true)
        XCTAssertTrue(realLog.isEmpty)
        XCTAssertEqual(copyLog.count, 1)
    }

    func testNothingIsTypedAnywhereAndTheWakeIsOneShortLine() throws {
        let watchers = make()
        _ = try add(watchers, ["kind": "time", "minutes": 1])
        clock = clock.addingTimeInterval(61)
        watchers.tick(inline: true)
        XCTAssertEqual(woken.count, 1)
        XCTAssertFalse(woken[0].contains("\n"))
        XCTAssertTrue(woken[0].contains("gm inbox"))
        // Without the paste paths, the event waits in the log: the log line is all there is.
        let quiet = Watchers(folder: root.appendingPathComponent("quiet"), roots: { [] }, hires: { [] }, log: { self.logged.append($0) }, now: { self.clock })
        _ = try quiet.add(["kind": "time", "minutes": 1], by: "owner")
        clock = clock.addingTimeInterval(61)
        quiet.tick(inline: true)
        XCTAssertEqual(woken.count, 1)
        XCTAssertEqual(logged.count, 2)
    }

    func testTheWatchesAreCappedAndTheEventLogTakesTheirFirings() throws {
        let watchers = make()
        for _ in 0..<Watchers.watchesMost { _ = try add(watchers, ["kind": "time", "minutes": 5]) }
        XCTAssertEqual(refusal(watchers, ["kind": "time", "minutes": 5]), 409)

        let log = EventLog(folder: root.appendingPathComponent("log"))
        let live = Watchers(folder: root.appendingPathComponent("live"), roots: { [] }, hires: { [] }, log: { log.append($0) }, now: { self.clock })
        _ = try live.add(["kind": "time", "target": "ping", "minutes": 1], by: "gm")
        clock = clock.addingTimeInterval(61)
        live.tick(inline: true)
        let read = log.entries(since: 0, limit: 10).entries
        XCTAssertEqual(read.map(\.kind), [.watcherFired])
        XCTAssertEqual(read.first?.detail, "watch \(live.all[0].id) (time): ping")
    }

    // MARK: Fixtures and routes

    private func fixture(_ name: String) throws -> [String: Any] {
        let text = try String(contentsOf: FrontDeskFixtures.url(name)).replacingOccurrences(of: "$ROOT", with: hireFolder)
        return try XCTUnwrap(JSONSerialization.jsonObject(with: Data(text.utf8)) as? [String: Any])
    }

    func testWatchFixtureRequestsAreAcceptedAndRefusedAsItSays() throws {
        let spec = try fixture("watch.json")
        let requests = try XCTUnwrap(spec["requests"] as? [String: [String: Any]])
        let watchers = make()
        try write("x\n")
        for (name, request) in requests.sorted(by: { $0.key < $1.key }) {
            XCTAssertNoThrow(try add(watchers, request), name)
        }
        XCTAssertEqual(watchers.all.count, requests.count)
        for refused in try XCTUnwrap(spec["refusals"] as? [[String: Any]]) {
            XCTAssertEqual(refusal(make(in: root.appendingPathComponent("r\(counter)")), refused["request"] as! [String: Any]),
                           refused["status"] as? Int, "\(refused)")
        }
        let kinds = EventLog.Kind.allCases.filter { $0 != .watcherFired }.map(\.rawValue)
        XCTAssertEqual(spec["event-kinds"] as? [String], kinds)
    }

    func testTheAnswersAreTheFixtures() throws {
        try write("")
        let watchers = make()
        let spec = try fixture("watch.json")
        let watch = try add(watchers, (spec["requests"] as! [String: [String: Any]])["file"]!)
        let shown = try XCTUnwrap(watchers.json.first { $0["id"] as? String == watch.id })
        XCTAssertTrue((shown as NSDictionary).isEqual(to: spec["answer-to-file"] as! [String: Any]), "\(shown)")

        _ = try add(watchers, ["kind": "time", "target": "check the build", "minutes": 30, "repeat": true], by: "owner")
        let list = try fixture("watches.json")["answer"] as! [String: Any]
        XCTAssertTrue((["watches": watchers.json] as NSDictionary).isEqual(to: list), "\(watchers.json)")

        // What a firing leaves in the event log.
        let line = spec["fired-line-in-the-event-log"] as! [String: Any]
        try write("DONE T405\n", append: true)
        watchers.tick(inline: true)
        XCTAssertEqual(logged.first?.kind.rawValue, line["kind"] as? String)
        XCTAssertEqual(logged.first?.detail, line["detail"] as? String)
        XCTAssertEqual(logged.first?.folder, line["folder"] as? String)
    }

    func testTheRoutesAreForTheGMAndThePage() {
        XCTAssertTrue(SetupRoutes.allows(.tool, "GET", "/setup/gm/watches"))
        XCTAssertTrue(SetupRoutes.allows(.gm, "GET", "/setup/gm/watches"))
        for (method, path) in [("POST", "/setup/gm/watch"), ("POST", "/setup/gm/unwatch"), ("DELETE", "/setup/gm/unwatch")] {
            XCTAssertTrue(SetupRoutes.allows(.gm, method, path), path)
            XCTAssertTrue(SetupRoutes.allows(.page, method, path), path)
            XCTAssertFalse(SetupRoutes.allows(.tool, method, path), "a tool of yours (a hire can be one) can't set watches: \(path)")
        }
    }
}
