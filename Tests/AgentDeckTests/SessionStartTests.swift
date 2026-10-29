import XCTest
@testable import AgentDeck

/// A desk's name tag counts from when its session started: /clear and
/// compaction don't move that, while each prompt starts a turn of its own.
final class SessionStartTests: XCTestCase {
    private let tab = "tty:/dev/ttys003"

    private func apply(_ sessions: Sessions, _ name: String, _ id: String, at: Double, _ fields: [String: Any] = [:]) {
        let event = ["hook_event_name": name, "session_id": id, "cwd": "/work/api"].merging(fields) { $1 }
        sessions.apply(event, agent: .claude, tab: tab, startedIn: "", at: at)
    }

    func testTheStartIsKeptThroughClearAndCompaction() {
        let sessions = Sessions()
        apply(sessions, "SessionStart", "s1", at: 1_000, ["source": "startup"])
        apply(sessions, "UserPromptSubmit", "s1", at: 5_000)
        apply(sessions, "SessionStart", "s1", at: 6_000, ["source": "compact"])
        XCTAssertEqual(sessions["s1"]?.startedAt, 1_000)
        apply(sessions, "SessionStart", "s2", at: 9_000, ["source": "clear"])
        apply(sessions, "UserPromptSubmit", "s2", at: 9_500)
        XCTAssertNil(sessions["s1"])
        XCTAssertEqual(sessions["s2"]?.startedAt, 1_000)
        XCTAssertEqual(sessions["s2"]?.turnStartedAt, 9_500)
    }
}
