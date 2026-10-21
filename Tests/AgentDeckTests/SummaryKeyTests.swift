import XCTest
@testable import AgentDeck

/// ✨ Summary's key: where it comes from, what setup may show of it, and
/// everywhere it must never turn up. A stand-in store, never the Keychain,
/// and a made-up key, put together here so no file holds one whole.
final class SummaryKeyTests: XCTestCase {
    private final class Memory {
        var key: String?
        var hint: String?
        var reads = 0
    }

    private var memory = Memory()
    private var kept: SummaryKey.Store!
    private let key = "sk-" + "ant-" + "api03-" + String(repeating: "T3stK3y", count: 12) + "-xY9z"

    override func setUp() {
        kept = SummaryKey.store
        let memory = Memory()
        self.memory = memory
        SummaryKey.store = SummaryKey.Store(
            read: { memory.reads += 1; return memory.key },
            hint: { memory.hint },
            write: { key, hint in memory.key = key; memory.hint = hint; return true },
            delete: { memory.key = nil; memory.hint = nil })
        SummaryKey.forgetKnown()
    }

    override func tearDown() {
        SummaryKey.store = kept
        SummaryKey.forgetKnown()
    }

    /// Every run of 12 characters of the key past its common start: none may turn up anywhere.
    private func assertNoPartOfTheKey(_ text: String, _ what: String, file: StaticString = #filePath, line: UInt = #line) {
        let characters = Array(key)
        for start in 7...(characters.count - 12) {
            let piece = String(characters[start..<start + 12])
            if text.contains(piece) { return XCTFail("\(what) holds part of the key", file: file, line: line) }
        }
    }

    private func text(_ object: Any) -> String {
        String(data: try! JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]), encoding: .utf8)!
    }

    // MARK: Where it comes from

    func testOnlyAKeyIsKept() {
        XCTAssertFalse(SummaryKey.save("hello"))
        XCTAssertFalse(SummaryKey.save("sk-" + "ant-short"))
        XCTAssertFalse(SummaryKey.save(key + " and more"))
        XCTAssertNil(memory.key)
        XCTAssertTrue(SummaryKey.save("  \(key)\n"))
        XCTAssertEqual(memory.key, key, "kept trimmed")
        XCTAssertEqual(SummaryKey.shown(environment: [:]), .init(source: .keychain, hint: "sk-ant-…xY9z"))
        SummaryKey.remove()
        XCTAssertNil(memory.key)
        XCTAssertNil(SummaryKey.shown(environment: [:]))
    }

    func testAPastedKeyComesBeforeTheEnvironmentsAndAnInvalidOneIsIgnored() {
        let other = "sk-" + "ant-" + "api03-" + String(repeating: "0th3r", count: 10) + "-AbCd"
        let environment = ["ANTHROPIC_API_KEY": other]
        XCTAssertEqual(SummaryKey.read(environment: environment), other)
        XCTAssertEqual(SummaryKey.shown(environment: environment), .init(source: .environment, hint: "sk-ant-…AbCd"))
        SummaryKey.save(key)
        XCTAssertEqual(SummaryKey.read(environment: environment), key)
        XCTAssertEqual(SummaryKey.shown(environment: environment)?.source, .keychain)
        SummaryKey.remove()
        XCTAssertNil(SummaryKey.read(environment: ["ANTHROPIC_API_KEY": "not a key"]))
        XCTAssertNil(SummaryKey.shown(environment: ["ANTHROPIC_API_KEY": ""]))
    }

    /// Setup asks often: the hint comes from memory, and the key itself is never read for it.
    func testSetupNeverReadsTheKeyItself() {
        SummaryKey.save(key)
        for _ in 0..<5 { _ = DeckRouter.summaryState(optIn: true, environment: [:]) }
        XCTAssertEqual(memory.reads, 0)
    }

    // MARK: Where it never goes

    func testSetupShowsOnlyTheHint() {
        SummaryKey.save(key)
        let state = text(DeckRouter.summaryState(optIn: true, environment: [:]))
        XCTAssertTrue(state.contains("sk-ant-…xY9z"))
        XCTAssertTrue(state.contains("\"enabled\":true"))
        assertNoPartOfTheKey(state, "setup's state")
        SummaryKey.remove()
        let fromEnvironment = text(DeckRouter.summaryState(optIn: true, environment: ["ANTHROPIC_API_KEY": key]))
        assertNoPartOfTheKey(fromEnvironment, "setup's state with the environment's key")
        XCTAssertTrue(text(DeckRouter.summaryState(optIn: true, environment: [:])).contains("\"enabled\":false"), "no key, not on")
    }

    func testNeverInConfigOrSessions() throws {
        SummaryKey.save(key)
        let config = Config()
        config.summaryOptIn = true
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let file = folder.appendingPathComponent("config.json")
        config.save(to: file)
        assertNoPartOfTheKey(try String(contentsOf: file, encoding: .utf8), "config.json")

        let sessions = Sessions()
        sessions.apply(["session_id": "s1", "hook_event_name": "UserPromptSubmit", "cwd": "/work/api",
                        "transcript_path": "/work/t.jsonl"], agent: .claude, tab: "tty:/dev/ttys003", startedIn: "", at: 1)
        let saved = String(data: try JSONEncoder().encode(sessions.saved), encoding: .utf8)!
        assertNoPartOfTheKey(saved, "sessions.json")
    }

    func testNeverInTheViewersSnapshot() {
        SummaryKey.save(key)
        let session = Session(id: "s1", agent: .claude, folder: "/work/api", project: "/work/api", startedAt: 1)
        let office = ViewerSnapshot.build(desks: [.init(pin: nil, session: session, role: nil)], overflow: 0, theme: [:],
                                          music: (false, nil), layout: Layout.standard, hideNames: false, now: 1, alias: { _ in "v1" })
        let plain = String(data: ViewerCrypto.plaintext(office: office, at: 1)!, encoding: .utf8)!
        assertNoPartOfTheKey(plain, "the viewer's snapshot")
        XCTAssertTrue(plain.contains("\"summary\":false"), "nothing to press on the viewer")
    }

    /// A reply that echoes the key, or anything else, never comes through in an error.
    func testErrorsNeverCarryTheKeyOrAReply() {
        let echo = Data(#"{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key \#(key)"}}"#.utf8)
        for status in [400, 401, 403, 404, 413, 429, 500, 529] {
            guard case let .failure(error) = SummaryRequest.answer(echo, status: status, error: nil) else {
                return XCTFail("\(status) answered")
            }
            assertNoPartOfTheKey(error.message, "the error for \(status)")
            XCTAssertFalse(error.message.contains("authentication_error"), "a reply's own words never come through")
        }
        for error in [SummaryError.off, .noKey, .noSession, .noHistory, .tooMany, .timeout, .unreachable, .empty] {
            assertNoPartOfTheKey(text(["error": error.message]), "the phone's reply \(error.message)")
        }
    }

    /// The key goes in its header and nowhere else in the request.
    func testTheKeyIsOnlyInItsHeader() throws {
        let request = SummaryRequest.request(.init(text: "USER: hi", language: "English"), project: "api", key: key)
        XCTAssertEqual(request.value(forHTTPHeaderField: "x-api-key"), key)
        assertNoPartOfTheKey(request.url!.absoluteString, "the URL")
        assertNoPartOfTheKey(String(data: request.httpBody!, encoding: .utf8)!, "the body")
        for (name, value) in request.allHTTPHeaderFields ?? [:] where name.lowercased() != "x-api-key" {
            assertNoPartOfTheKey(value, "the \(name) header")
        }
    }
}
