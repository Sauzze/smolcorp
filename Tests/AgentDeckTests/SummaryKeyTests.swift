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

    /// Setup says where the key is, and no part of it: not even its last four, on or off (SIERRA 1.23 m-1).
    func testSetupShowsOnlyWhereTheKeyIs() {
        SummaryKey.save(key)
        for optIn in [true, false] {
            let state = text(DeckRouter.summaryState(optIn: optIn, environment: [:]))
            XCTAssertTrue(state.contains("\"source\":\"keychain\""), state)
            XCTAssertFalse(state.contains("xY9z") || state.contains("sk-ant-…") || state.contains("hint"), state)
            assertNoPartOfTheKey(state, "setup's state")
        }
        XCTAssertTrue(text(DeckRouter.summaryState(optIn: true, environment: [:])).contains("\"enabled\":true"))
        SummaryKey.remove()
        let fromEnvironment = text(DeckRouter.summaryState(optIn: false, environment: ["ANTHROPIC_API_KEY": key]))
        XCTAssertTrue(fromEnvironment.contains("\"source\":\"environment\""), fromEnvironment)
        XCTAssertFalse(fromEnvironment.contains(String(key.suffix(4))) || fromEnvironment.contains("hint"), fromEnvironment)
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

    /// A test copy's run uses only a key pasted into it: never the environment's.
    func testATestCopyNeverUsesTheEnvironmentsKey() {
        let environment = ["ANTHROPIC_API_KEY": key]
        XCTAssertNil(SummaryKey.read(environment: environment, testMode: true))
        XCTAssertNil(SummaryKey.shown(environment: environment, testMode: true))
        XCTAssertEqual(SummaryKey.read(environment: environment, testMode: false), key)
    }

    /// Only the test copy's own build, by its bundle id, ever talks to a stand-in: never a release, whatever its variables.
    func testAReleaseIgnoresTheStandInsAddress() {
        let local = ["SMOLCORP_TEST_ANTHROPIC_URL": "http://127.0.0.1:7781/v1/messages"]
        XCTAssertNil(SummaryRequest.testEndpoint(local), "this build isn't the test copy's")
        XCTAssertNotEqual(Bundle.main.bundleIdentifier, SummaryRequest.testCopyBundle)
    }

    /// A redirect is never followed: the key's header would go with it.
    func testARedirectIsRefusedAndTheKeyGoesNowhereElse() throws {
        RedirectingProtocol.reset()
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [RedirectingProtocol.self]
        let transport = Summaries.transport(using: configuration)
        let request = SummaryRequest.request(.init(text: "USER: hi", language: "English"), project: "api", key: key)
        let done = expectation(description: "answered")
        var status: Int?
        transport(request) { _, code, _ in status = code; done.fulfill() }
        wait(for: [done], timeout: 5)
        XCTAssertEqual(status, 307, "the redirect comes back as it is")
        XCTAssertEqual(RedirectingProtocol.hosts, ["api.anthropic.com"], "nothing was sent to where it pointed")
        XCTAssertFalse(RedirectingProtocol.keysElsewhere)
        guard case let .failure(error) = SummaryRequest.answer(nil, status: status, error: nil) else { return XCTFail() }
        assertNoPartOfTheKey(error.message, "the redirect's error")
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

    // MARK: Off the main queue

    /// A Keychain stuck on its dialog (the write waits) doesn't hold up the main queue, where other routes answer.
    func testSavingWhileTheKeychainWaitsDoesNotBlockMain() {
        let release = DispatchSemaphore(value: 0)
        let started = expectation(description: "the write began")
        let memory = self.memory
        SummaryKey.store = SummaryKey.Store(
            read: { memory.key }, hint: { memory.hint },
            write: { key, hint in
                started.fulfill()
                release.wait()
                memory.key = key; memory.hint = hint
                return true
            },
            delete: { memory.key = nil; memory.hint = nil })

        let saved = expectation(description: "saved")
        var kept: Bool?
        SummaryKey.save(key) { kept = $0; saved.fulfill() }
        wait(for: [started], timeout: 10)

        // Another route's reply, on the main queue, while the write still waits.
        let answered = expectation(description: "another route answered")
        DispatchQueue.main.async { answered.fulfill() }
        wait(for: [answered], timeout: 10)
        XCTAssertNil(kept, "the save is still waiting")

        release.signal()
        wait(for: [saved], timeout: 10)
        XCTAssertEqual(kept, true)
        XCTAssertEqual(SummaryKey.shown(environment: [:]), .init(source: .keychain, hint: "sk-ant-…xY9z"))
    }

    /// The first look for the hint isn't on the main queue: shown() answers at once, then the hint is learned.
    func testTheFirstHintReadWaitsOffMain() {
        let release = DispatchSemaphore(value: 0)
        let started = DispatchSemaphore(value: 0)
        let memory = self.memory
        memory.hint = "sk-ant-…xY9z"
        SummaryKey.store = SummaryKey.Store(
            read: { nil }, hint: { started.signal(); release.wait(); return memory.hint }, write: { _, _ in true }, delete: {})
        SummaryKey.forgetKnown()
        let learned = expectation(description: "learned")
        SummaryKey.onLearned = { learned.fulfill() }
        defer { SummaryKey.onLearned = {} }

        XCTAssertNil(SummaryKey.shown(environment: [:]), "nothing known yet, and not waited for")
        XCTAssertEqual(started.wait(timeout: .now() + 10), .success)
        let answered = expectation(description: "main answered")
        DispatchQueue.main.async { answered.fulfill() }
        wait(for: [answered], timeout: 10)

        release.signal()
        wait(for: [learned], timeout: 10)
        XCTAssertEqual(SummaryKey.shown(environment: [:]), .init(source: .keychain, hint: "sk-ant-…xY9z"))
    }

    func testRemovingOffMainDropsTheKeyAndTellsMain() {
        SummaryKey.save(key)
        let removed = expectation(description: "removed")
        SummaryKey.remove {
            XCTAssertTrue(Thread.isMainThread)
            removed.fulfill()
        }
        wait(for: [removed], timeout: 10)
        XCTAssertNil(SummaryKey.shown(environment: [:]))
        XCTAssertNil(memory.key)
    }

    func testAKeyThatIsntOneIsRefusedOffMainToo() {
        let refused = expectation(description: "refused")
        SummaryKey.save("hello") { kept in
            XCTAssertFalse(kept)
            refused.fulfill()
        }
        wait(for: [refused], timeout: 10)
        XCTAssertNil(memory.key)
    }

    // MARK: A test copy's key file is read again

    func testATestCopySeesAKeyFileWrittenAfterTheFirstLook() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("key-file-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: folder) }
        let file = folder.appendingPathComponent(TestCopySecrets.apiKeyName)
        SummaryKey.store = .file(file)
        SummaryKey.forgetKnown()
        XCTAssertNil(SummaryKey.shown(environment: [:], testMode: true), "no file yet: no key")
        XCTAssertTrue(TestCopySecrets.write(key, to: file))
        XCTAssertEqual(SummaryKey.shown(environment: [:], testMode: true), SummaryKey.Shown(source: .keychain, hint: SummaryKey.hint(key)),
                       "written while the copy runs: seen at once")
        TestCopySecrets.remove(file)
        XCTAssertNil(SummaryKey.shown(environment: [:], testMode: true), "a removed file is no key")
    }

    func testTheRealAppKeepsWhatItLearned() throws {
        memory.key = key
        memory.hint = SummaryKey.hint(key)
        let learned = expectation(description: "learned")
        SummaryKey.onLearned = { learned.fulfill() }
        defer { SummaryKey.onLearned = {} }
        _ = SummaryKey.shown(environment: [:], testMode: false)
        wait(for: [learned], timeout: 5)
        memory.key = nil
        memory.hint = nil
        XCTAssertEqual(SummaryKey.shown(environment: [:], testMode: false)?.hint, SummaryKey.hint(key), "the real app's cache is unchanged")
    }

    func testTheTestCopyRefusalNamesTheFileNotTheKeychain() {
        let file = URL(fileURLWithPath: "/tmp/copy/test-api-key")
        let copy = HireDesk.apiKeyRefusal(picked: true, offered: false, testCopy: true, file: file)
        XCTAssertEqual(copy?.status, 409)
        XCTAssertTrue(copy?.message.contains("/tmp/copy/test-api-key") == true)
        XCTAssertTrue(copy?.message.contains("fake key") == true)
        XCTAssertFalse(copy?.message.contains("Keychain") == true)
        XCTAssertFalse(HireDesk.keyRefusalText(testCopy: true, file: file).message.contains("Keychain"))
        XCTAssertFalse(HireDesk.apiKeyNoticeText(testCopy: true).contains("Keychain"))
        XCTAssertTrue(HireDesk.apiKeyNoticeText(testCopy: true).contains("test-api-key"))
        let real = HireDesk.apiKeyRefusal(picked: true, offered: false, testCopy: false)
        XCTAssertEqual(real?.message, "There's no Anthropic API key in your Keychain. Save one under Summaries, or hire on a Claude account.")
        XCTAssertEqual(HireDesk.keyRefusalText(testCopy: false).message, HireDesk.keyRefusal.message)
        XCTAssertEqual(HireDesk.apiKeyNoticeText(testCopy: false), HireDesk.apiKeyNotice)
    }
}

/// Answers Anthropic's address with a redirect elsewhere, and keeps where requests went.
final class RedirectingProtocol: URLProtocol {
    private static let lock = NSLock()
    private static var seen: [String] = []
    private static var keyElsewhere = false

    static func reset() { lock.lock(); seen = []; keyElsewhere = false; lock.unlock() }
    static var hosts: [String] { lock.lock(); defer { lock.unlock() }; return seen }
    static var keysElsewhere: Bool { lock.lock(); defer { lock.unlock() }; return keyElsewhere }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let host = request.url?.host ?? ""
        RedirectingProtocol.lock.lock()
        RedirectingProtocol.seen.append(host)
        if host != "api.anthropic.com" && request.value(forHTTPHeaderField: "x-api-key") != nil { RedirectingProtocol.keyElsewhere = true }
        RedirectingProtocol.lock.unlock()
        if host == "api.anthropic.com" {
            let elsewhere = URL(string: "https://elsewhere.example/steal")!
            let response = HTTPURLResponse(url: request.url!, statusCode: 307, httpVersion: "HTTP/1.1",
                                           headerFields: ["Location": elsewhere.absoluteString])!
            var next = request
            next.url = elsewhere
            client?.urlProtocol(self, wasRedirectedTo: next, redirectResponse: response)
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocolDidFinishLoading(self)
        } else {
            let response = HTTPURLResponse(url: request.url!, statusCode: 200, httpVersion: "HTTP/1.1", headerFields: [:])!
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: Data(#"{"content":[{"type":"text","text":"stolen"}]}"#.utf8))
            client?.urlProtocolDidFinishLoading(self)
        }
    }

    override func stopLoading() {}
}
