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
    /// A key starts so; written in two pieces, as no key-shaped text is committed.
    private let prefix = "sk-" + "ant-"
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
        XCTAssertEqual(SummaryKey.shown(environment: [:]), .init(source: .keychain, hint: prefix + "…xY9z"))
        SummaryKey.remove()
        XCTAssertNil(memory.key)
        XCTAssertNil(SummaryKey.shown(environment: [:]))
    }

    func testAPastedKeyComesBeforeTheEnvironmentsAndAnInvalidOneIsIgnored() {
        let other = "sk-" + "ant-" + "api03-" + String(repeating: "0th3r", count: 10) + "-AbCd"
        let environment = ["ANTHROPIC_API_KEY": other]
        XCTAssertEqual(SummaryKey.read(environment: environment), other)
        XCTAssertEqual(SummaryKey.shown(environment: environment), .init(source: .environment, hint: prefix + "…AbCd"))
        SummaryKey.save(key)
        XCTAssertEqual(SummaryKey.read(environment: environment), key)
        XCTAssertEqual(SummaryKey.shown(environment: environment)?.source, .keychain)
        SummaryKey.remove()
        XCTAssertNil(SummaryKey.read(environment: ["ANTHROPIC_API_KEY": "not a key"]))
        XCTAssertNil(SummaryKey.shown(environment: ["ANTHROPIC_API_KEY": ""]))
    }

    // MARK: Where it never goes

    /// A test copy's run uses only a key pasted into it: never the environment's.
    func testATestCopyNeverUsesTheEnvironmentsKey() {
        let environment = ["ANTHROPIC_API_KEY": key]
        XCTAssertNil(SummaryKey.read(environment: environment, testMode: true))
        XCTAssertNil(SummaryKey.shown(environment: environment, testMode: true))
        XCTAssertEqual(SummaryKey.read(environment: environment, testMode: false), key)
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
        XCTAssertEqual(SummaryKey.shown(environment: [:]), .init(source: .keychain, hint: prefix + "…xY9z"))
    }

    /// The first look for the hint isn't on the main queue: shown() answers at once, then the hint is learned.
    func testTheFirstHintReadWaitsOffMain() {
        let release = DispatchSemaphore(value: 0)
        let started = DispatchSemaphore(value: 0)
        let memory = self.memory
        memory.hint = prefix + "…xY9z"
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
        XCTAssertEqual(SummaryKey.shown(environment: [:]), .init(source: .keychain, hint: prefix + "…xY9z"))
    }

    /// A stand-in whose first look waits, and finds nothing (the Keychain's gate still shut), while the gate opens
    /// (`forgetKnown`, as `KeychainGate.open` calls it); every look after finds the hint.
    private func gateOpensDuringTheFirstLook() -> (started: DispatchSemaphore, release: DispatchSemaphore) {
        final class First { var look = true }
        let first = First(), started = DispatchSemaphore(value: 0), release = DispatchSemaphore(value: 0)
        let memory = self.memory
        memory.hint = "sk-ant-…xY9z"
        SummaryKey.store = SummaryKey.Store(read: { nil }, hint: {
            guard first.look else { return memory.hint }
            first.look = false
            started.signal()
            release.wait()
            return nil
        }, write: { _, _ in true }, delete: {})
        SummaryKey.forgetKnown()
        return (started, release)
    }

    /// The first look's "none" isn't kept once the gate has opened: the hint is looked for again, and found.
    func testALookBegunBeforeTheGateOpenedIsntKept() {
        let (started, release) = gateOpensDuringTheFirstLook()
        let learned = expectation(description: "learned")
        SummaryKey.onLearned = { learned.fulfill() }
        defer { SummaryKey.onLearned = {} }
        XCTAssertNil(SummaryKey.shown(environment: [:]))
        XCTAssertEqual(started.wait(timeout: .now() + 10), .success)
        SummaryKey.forgetKnown()
        release.signal()
        // The first look's answer reaches the main queue, and is dropped there.
        SummaryKey.queue.sync {}
        let drained = expectation(description: "main drained")
        DispatchQueue.main.async { drained.fulfill() }
        wait(for: [drained], timeout: 10)

        XCTAssertNil(SummaryKey.shown(environment: [:]), "not known: looked for again")
        wait(for: [learned], timeout: 10)
        XCTAssertEqual(SummaryKey.shown(environment: [:]), .init(source: .keychain, hint: "sk-ant-…xY9z"))
    }

    /// whenKnown begun before the gate opened looks again before it answers.
    func testWhenKnownBegunBeforeTheGateOpenedLooksAgain() {
        let (started, release) = gateOpensDuringTheFirstLook()
        let answered = expectation(description: "answered")
        SummaryKey.whenKnown {
            XCTAssertEqual(SummaryKey.shown(environment: [:]), .init(source: .keychain, hint: "sk-ant-…xY9z"))
            answered.fulfill()
        }
        XCTAssertEqual(started.wait(timeout: .now() + 10), .success)
        SummaryKey.forgetKnown()
        release.signal()
        wait(for: [answered], timeout: 10)
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
