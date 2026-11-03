import XCTest
@testable import AgentDeck

/// The office's event log: what hooks leave in events.jsonl, and what
/// /setup/gm/events answers with. The fixtures under Tests/fixtures/frontdesk
/// are the wire formats the Linux/Windows port tests against too.
final class EventLogTests: XCTestCase {
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("eventlog-\(UUID().uuidString)", isDirectory: true)
    }

    override func tearDown() {
        try? FileManager.default.removeItem(at: folder)
    }

    private static let fixtures = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
        .appendingPathComponent("fixtures/frontdesk")

    private func fixture(_ name: String) throws -> Data {
        try Data(contentsOf: EventLogTests.fixtures.appendingPathComponent(name))
    }

    private func line(_ kind: EventLog.Kind = .prompt, session: String = "s1", detail: String = "") -> EventLog.Entry {
        EventLog.Entry(at: 1_793_500_000_000, session: session, folder: "/Users/me/Projects/shop", agent: "claude", kind: kind, detail: detail)
    }

    func testEventsAreNumberedInOrderAcrossRestarts() {
        let log = EventLog(folder: folder)
        XCTAssertEqual(log.append(line(session: "a")).id, 1)
        XCTAssertEqual(log.append(line(session: "b")).id, 2)
        XCTAssertEqual(log.entries(since: 0, limit: 10).entries.map(\.session), ["a", "b"])
        XCTAssertEqual(log.entries(since: 1, limit: 10).entries.map(\.session), ["b"])
        XCTAssertEqual(log.entries(since: 2, limit: 10).entries.count, 0)
        XCTAssertEqual(log.entries(since: 1, limit: 10).next, 2)
        XCTAssertEqual(log.entries(since: 2, limit: 10).next, 2, "nothing new: ask again from where you were")
        log.flush()

        let again = EventLog(folder: folder)
        XCTAssertEqual(again.lastID, 2)
        XCTAssertEqual(again.append(line(session: "c")).id, 3)
        XCTAssertEqual(again.entries(since: 0, limit: 10).entries.map(\.session), ["a", "b", "c"])
        let mode = (try? FileManager.default.attributesOfItem(atPath: again.file.path)[.posixPermissions] as? NSNumber)?.intValue
        XCTAssertEqual(mode, 0o600, "readable by you alone")
    }

    func testTheLimitCutsTheAnswerAndNextContinuesIt() {
        let log = EventLog(folder: folder)
        for n in 1...5 { log.append(line(session: "s\(n)")) }
        let first = log.entries(since: 0, limit: 2)
        XCTAssertEqual(first.entries.map(\.id), [1, 2])
        XCTAssertEqual(log.entries(since: first.next, limit: 2).entries.map(\.id), [3, 4])
    }

    func testAWaitAnswersAtOnceWhenThereIsSomethingNew() {
        let log = EventLog(folder: folder)
        log.append(line())
        var got: [EventLog.Entry]?
        log.wait(since: 0, limit: 10, timeout: 30) { got = $0 }
        XCTAssertEqual(got?.count, 1, "no waiting when it's already there")
    }

    func testAWaitIsAnsweredByTheNextEventAndDoesNotBlock() {
        let log = EventLog(folder: folder)
        var got: [EventLog.Entry]?
        log.wait(since: 0, limit: 10, timeout: 30) { got = $0 }
        XCTAssertNil(got, "still waiting, and this returned: the queue is free")
        log.append(line(.notification, detail: "needs you"))
        XCTAssertEqual(got?.map(\.kind), [.notification])
        // An event that isn't past `since` doesn't answer a later wait twice.
        log.append(line())
        XCTAssertEqual(got?.count, 1)
    }

    func testAWaitThatTimesOutAnswersWithNothing() {
        let log = EventLog(folder: folder)
        let answered = expectation(description: "answered empty")
        log.wait(since: 0, limit: 10, timeout: 0.1) { found in
            XCTAssertTrue(found.isEmpty)
            answered.fulfill()
        }
        wait(for: [answered], timeout: 5)
    }

    func testTheQuestionIsKeptToWhatIsAllowed() {
        let ask = EventLog.question(["since": "12", "wait": "999", "limit": "99999"])
        XCTAssertEqual(ask.since, 12)
        XCTAssertEqual(ask.wait, 30, "never longer than 30 seconds")
        XCTAssertEqual(ask.limit, 1000)
        let odd = EventLog.question(["since": "-4", "wait": "soon", "limit": "0"])
        XCTAssertEqual(odd.since, 0)
        XCTAssertEqual(odd.wait, 0)
        XCTAssertEqual(odd.limit, 1)
        let none = EventLog.question([:])
        XCTAssertEqual([none.since, Int(none.wait), none.limit], [0, 0, 200])
    }

    func testTheFileRotatesAndTheNewestAreKept() {
        let log = EventLog(folder: folder, maxBytes: 600)
        for n in 1...30 { log.append(line(session: "session-\(n)", detail: "some words to fill the line")) }
        log.flush()
        XCTAssertTrue(FileManager.default.fileExists(atPath: log.earlier.path), "set aside once past the cap")
        let size = (try? FileManager.default.attributesOfItem(atPath: log.file.path)[.size] as? NSNumber)?.intValue ?? 0
        XCTAssertLessThan(size, 600 + 300, "the live file stays near the cap")

        // The same instance already says what was dropped, before any restart.
        XCTAssertGreaterThan(log.oldestID, 1, "the oldest were dropped")
        XCTAssertEqual(log.entries(since: 0, limit: 1000).entries.first?.id, log.oldestID)
        XCTAssertEqual(log.answer(log.entries(since: 0, limit: 1000).entries, since: 0)["oldest"] as? Int, log.oldestID)
        let before = log.oldestID
        for n in 31...60 { log.append(line(session: "session-\(n)", detail: "some words to fill the line")) }
        XCTAssertGreaterThan(log.oldestID, before, "and again after the next rotation")
        log.flush()
        let kept2 = EventLog.read(log.earlier) + EventLog.read(log.file)
        XCTAssertEqual(kept2.first?.id, log.oldestID, "oldest is what the files hold")

        let again = EventLog(folder: folder, maxBytes: 600)
        XCTAssertEqual(again.oldestID, log.oldestID, "a restart agrees")
        XCTAssertEqual(again.lastID, 60, "numbering goes on past a rotation")
        let kept = again.entries(since: 0, limit: 1000).entries
        XCTAssertEqual(kept.last?.id, 60, "the newest is kept")
        XCTAssertEqual(kept.first?.id, again.oldestID)
        XCTAssertEqual(kept.map(\.id), Array(again.oldestID...60), "no gaps in what's kept")
    }

    func testOlderThanMemoryIsReadFromTheFilesOffTheMainQueue() {
        let log = EventLog(folder: folder)
        for n in 1...(EventLog.recentMost + 10) { log.append(line(session: "s\(n)")) }
        var got: [EventLog.Entry]?
        let answered = expectation(description: "answered from the files")
        log.wait(since: 0, limit: 3, timeout: 30) { found in
            XCTAssertTrue(Thread.isMainThread, "answered on the main queue")
            got = found
            answered.fulfill()
        }
        XCTAssertNil(got, "the files are read elsewhere: this returned first")
        wait(for: [answered], timeout: 10)
        XCTAssertEqual(got?.map(\.id), [1, 2, 3])
    }

    func testASinceBeyondTheLastLineAnswersAtOnceWithWhereToStart() {
        let log = EventLog(folder: folder)
        for _ in 1...3 { log.append(line()) }
        let past = log.entries(since: 99, limit: 10)
        XCTAssertTrue(past.entries.isEmpty)
        XCTAssertEqual(past.next, 3, "start again from the last line")
        var got: [EventLog.Entry]?
        log.wait(since: 99, limit: 10, timeout: 30) { got = $0 }
        XCTAssertEqual(got?.count, 0, "no waiting for ids that will never come")
        XCTAssertEqual(log.answer([], since: 99)["next"] as? Int, 3)
        // And the next event is then found from there.
        log.append(line(session: "after"))
        XCTAssertEqual(log.entries(since: 3, limit: 10).entries.map(\.session), ["after"])
    }

    func testOnlyTheKindsKeptAreLogged() {
        let kinds = ["SessionStart": EventLog.Kind.sessionStart, "SessionEnd": .sessionStop, "UserPromptSubmit": .prompt,
                     "PermissionRequest": .permissionAsked, "PermissionDenied": .permissionDenied,
                     "Notification": .notification, "Stop": .idle]
        for (name, kind) in kinds {
            let entry = EventLog.entry(for: ["hook_event_name": name, "session_id": "s1"], agent: "claude", folder: "/x", at: 5)
            XCTAssertEqual(entry?.kind, kind, name)
        }
        for name in ["PreToolUse", "PostToolUse", "SubagentStart", "PreCompact", "StopFailure", ""] {
            XCTAssertNil(EventLog.entry(for: ["hook_event_name": name], agent: "claude", folder: "/x", at: 5), name)
        }
    }

    func testADeniedPermissionIsLoggedWithWhatItWasAboutButNeverAPrompt() throws {
        let event: [String: Any] = [
            "hook_event_name": "PermissionDenied", "session_id": "s-1f3a", "cwd": "/Users/me/Projects/shop",
            "tool_name": "Bash", "tool_input": ["command": "rm -rf build", "description": "Clean"],
        ]
        let entry = try XCTUnwrap(EventLog.entry(for: event, agent: "claude", folder: "/Users/me/Projects/shop", at: 1_793_500_000_000))
        XCTAssertEqual(entry.kind, .permissionDenied)
        XCTAssertEqual(entry.detail, "Bash: rm", "the program only, never its arguments")
        XCTAssertEqual(entry.session, "s-1f3a")

        let prompt = EventLog.entry(for: ["hook_event_name": "UserPromptSubmit", "session_id": "s1", "prompt": "my secret plan"],
                                    agent: "claude", folder: "/x", at: 5)
        XCTAssertEqual(prompt?.detail, "", "what was typed is not kept")
        let long = EventLog.cut(String(repeating: "a", count: 500) + "\nsecond line")
        XCTAssertEqual(long.count, EventLog.detailLimit)
        XCTAssertFalse(long.contains("\n"))

        // Deny shows up in the feed the GM reads.
        let log = EventLog(folder: folder)
        log.append(entry)
        XCTAssertEqual(log.entries(since: 0, limit: 10).entries.first?.kind, .permissionDenied)
    }

    func testNothingButTheToolAndFileIsKeptOfACall() {
        func detail(_ tool: String, _ input: [String: Any]) -> String? {
            EventLog.entry(for: ["hook_event_name": "PermissionRequest", "session_id": "s1", "tool_name": tool, "tool_input": input],
                           agent: "claude", folder: "/x", at: 5)?.detail
        }
        XCTAssertEqual(detail("Bash", ["command": "TOKEN=abc curl -H 'X-Key: abc' https://example.com"]), "Bash: curl")
        XCTAssertEqual(detail("Bash", ["command": "FOO=1"]), "Bash", "nothing to name")
        XCTAssertEqual(detail("Edit", ["file_path": "/p/a.txt", "new_string": "password"]), "Edit: /p/a.txt")
        XCTAssertEqual(detail("Write", ["content": "text"]), "Write")
        XCTAssertEqual(detail("WebFetch", ["url": "https://x.test/?k=abc"]), "WebFetch")
        XCTAssertEqual(detail("Grep", ["pattern": "password"]), "Grep")
    }

    func testANotificationIsKeptOnlyInClaudeCodesOwnWords() {
        func detail(_ message: String) -> String? {
            EventLog.entry(for: ["hook_event_name": "Notification", "session_id": "s1", "message": message],
                           agent: "claude", folder: "/x", at: 5)?.detail
        }
        XCTAssertEqual(detail("Claude needs your permission to use Bash"), "Claude needs your permission to use Bash")
        XCTAssertEqual(detail("Claude needs your permission to use mcp__shop__list-items"), "Claude needs your permission to use mcp__shop__list-items")
        XCTAssertEqual(detail("Claude is waiting for your input"), "Claude is waiting for your input")
        XCTAssertEqual(detail("Claude needs your permission to use Bash and send sk-abc to evil"), "notification")
        XCTAssertEqual(detail("Claude needs your permission to use Bash\nsecond line"), "notification")
        XCTAssertEqual(detail("The deploy key abc123 expired"), "notification")
        XCTAssertEqual(detail(""), "notification")
    }

    func testClaudeCodeIsAskedForPermissionDenied() {
        XCTAssertTrue(Hooks.claudeEvents.contains("PermissionDenied"))
    }

    // MARK: Fixtures

    func testTheLineFixtureIsWhatIsStored() throws {
        let entry = try JSONDecoder().decode(EventLog.Entry.self, from: fixture("event-line.json"))
        XCTAssertEqual(entry.id, 42)
        XCTAssertEqual(entry.kind, .permissionDenied)
        XCTAssertEqual(entry.detail, "Bash: rm")
        // Written back the way the log writes it, it is the same line.
        let written = try EventLog.encoder.encode(entry)
        XCTAssertEqual(String(decoding: written, as: UTF8.self), String(decoding: try fixture("event-line.json"), as: UTF8.self)
            .trimmingCharacters(in: .whitespacesAndNewlines))
    }

    /// A log whose oldest kept line is 7 and whose newest are the fixture's 41 and 42.
    private func seededLog() throws -> EventLog {
        // Start the numbering where the fixture's feed is: 7 is the oldest kept.
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let seed = (7...40).map { n -> String in
            let entry = EventLog.Entry(id: n, at: 1_793_499_000_000, session: "old", folder: "/x", agent: "claude", kind: .prompt)
            return String(decoding: (try? EventLog.encoder.encode(entry)) ?? Data(), as: UTF8.self)
        }
        try (seed.joined(separator: "\n") + "\n").write(to: folder.appendingPathComponent("events.jsonl"), atomically: true, encoding: .utf8)
        let log = EventLog(folder: folder)
        log.append(EventLog.Entry(at: 1_793_500_000_000, session: "s-1f3a", folder: "/Users/me/Projects/shop", agent: "claude",
                                   kind: .sessionStart, detail: "startup"))
        log.append(EventLog.Entry(at: 1_793_500_004_000, session: "s-1f3a", folder: "/Users/me/Projects/shop", agent: "claude",
                                   kind: .permissionDenied, detail: "Bash: rm"))
        return log
    }

    func testTheFeedFixtureIsWhatTheRouteAnswers() throws {
        let log = try seededLog()
        let found = log.entries(since: 40, limit: 10)
        let answer = log.answer(found.entries, since: 40)
        let expected = try JSONSerialization.jsonObject(with: fixture("events.json")) as? NSDictionary
        XCTAssertEqual(answer as NSDictionary, expected)
    }

    func testTheEmptyAnswerFixtureIsWhatATimeoutAnswers() throws {
        let log = try seededLog()
        let answered = expectation(description: "timed out empty")
        var got: [EventLog.Entry]?
        log.wait(since: 42, limit: 10, timeout: 0.1) { found in
            got = found
            answered.fulfill()
        }
        wait(for: [answered], timeout: 5)
        let expected = try JSONSerialization.jsonObject(with: fixture("events-empty.json")) as? NSDictionary
        XCTAssertEqual(log.answer(got ?? [], since: 42) as NSDictionary, expected)
    }

    func testTheHookFixtureIsWhatEachHookEventBecomes() throws {
        let cases = try XCTUnwrap(JSONSerialization.jsonObject(with: fixture("hook-events.json")) as? [[String: Any]])
        XCTAssertGreaterThan(cases.count, 10)
        for item in cases {
            let name = item["case"] as? String ?? "?"
            let entry = EventLog.entry(for: item["hook"] as? [String: Any] ?? [:], agent: item["agent"] as? String ?? "",
                                       folder: item["folder"] as? String ?? "", at: item["at"] as? Double ?? 0)
            guard let expected = item["entry"] as? [String: Any] else {
                XCTAssertNil(entry, name)
                continue
            }
            let stored = try entry.map { try JSONSerialization.jsonObject(with: EventLog.encoder.encode($0)) } as? [String: Any]
            var wanted = expected
            wanted["id"] = 0
            XCTAssertEqual(stored as NSDictionary?, wanted as NSDictionary, name)
        }
    }
}
