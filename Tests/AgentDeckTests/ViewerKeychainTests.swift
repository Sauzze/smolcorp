import XCTest
@testable import AgentDeck

/// The portable office's Keychain calls: none on the main queue. A stand-in store that waits,
/// as the Keychain does while its dialog is up, never the real Keychain.
final class ViewerKeychainTests: XCTestCase {
    private final class Memory {
        let lock = NSLock()
        var items: [String: String] = [:]
        /// While set, the next call of that kind waits for `release`.
        var holdWrites = false
        var holdReads = false
        /// While set, a present/absent question waits for `release`, as at launch.
        var holdAsks = false
        /// How many times the data (not just present/absent) was read.
        var dataReads = 0
        let started = DispatchSemaphore(value: 0)
        let release = DispatchSemaphore(value: 0)

        func get(_ account: ViewerKeychain.Account) -> String? { lock.lock(); defer { lock.unlock() }; return items[account.rawValue] }
        func set(_ value: String, _ account: ViewerKeychain.Account) { lock.lock(); items[account.rawValue] = value; lock.unlock() }

        var store: ViewerStore {
            ViewerStore(
                read: { [self] account in
                    lock.lock(); dataReads += 1; lock.unlock()
                    if holdReads { started.signal(); release.wait() }
                    return get(account)
                },
                absent: { [self] account in
                    if holdAsks { started.signal(); release.wait() }
                    return get(account) == nil
                },
                write: { [self] value, account in
                    if holdWrites { started.signal(); release.wait() }
                    set(value, account)
                    return true
                })
        }
    }

    private func office(_ memory: Memory) -> PortableOffice {
        PortableOffice(store: memory.store) { _, _ in [:] }
    }

    /// Waits until the office has asked the Keychain once.
    private func known(_ office: PortableOffice) {
        let ready = expectation(description: "known")
        office.whenKeychainKnown { ready.fulfill() }
        wait(for: [ready], timeout: 10)
    }

    private func mainStillAnswers() {
        let answered = expectation(description: "main answered")
        DispatchQueue.main.async { answered.fulfill() }
        wait(for: [answered], timeout: 10)
    }

    func testMakingTheWriteSecretWhileTheKeychainWaitsDoesNotBlockMain() {
        let memory = Memory()
        let office = office(memory)
        known(office)
        memory.holdWrites = true

        let made = expectation(description: "secret made")
        var secret: String?
        office.makeWriteSecret { secret = $0; made.fulfill() }
        XCTAssertEqual(memory.started.wait(timeout: .now() + 10), .success)

        // Another route's reply on the main queue, and the state it would show, while the write waits.
        mainStillAnswers()
        XCTAssertNil(secret)
        XCTAssertFalse(office.hasWriteSecret)

        memory.holdWrites = false
        memory.release.signal()
        wait(for: [made], timeout: 10)
        XCTAssertNotNil(secret)
        XCTAssertEqual(memory.get(.writeSecret), secret)
        XCTAssertTrue(office.hasWriteSecret)
        XCTAssertNotNil(memory.get(.id), "the id and key are made with the first secret")
        XCTAssertNotNil(memory.get(.key))
    }

    func testRotatingWhileTheKeychainWaitsDoesNotBlockMain() {
        let memory = Memory()
        memory.set("AAAAAAAAAAAAAAAAAAAAAA", .id)
        memory.set("old", .key)
        let office = office(memory)
        known(office)
        let before = office.setupState()["revision"] as? Int
        memory.holdWrites = true

        let rotated = expectation(description: "rotated")
        office.rotateKey { rotated.fulfill() }
        XCTAssertEqual(memory.started.wait(timeout: .now() + 10), .success)
        mainStillAnswers()
        XCTAssertEqual(office.setupState()["revision"] as? Int, before, "nothing changes until the Keychain has answered")

        memory.holdWrites = false
        memory.release.signal()
        memory.release.signal()
        wait(for: [rotated], timeout: 10)
        XCTAssertEqual(office.setupState()["revision"] as? Int, (before ?? 0) + 1)
        XCTAssertNotEqual(memory.get(.key), "old")
    }

    func testTheFirstQuestionIsNotOnTheMainQueue() {
        let memory = Memory()
        memory.set("secret-for-test", .writeSecret)
        memory.holdAsks = true
        let office = office(memory)
        XCTAssertEqual(memory.started.wait(timeout: .now() + 10), .success, "asked at once, off the main queue")

        // Waiting on the Keychain: the main queue answers, and the state says there's nothing yet.
        mainStillAnswers()
        XCTAssertFalse(office.hasWriteSecret)
        XCTAssertNil(office.link)

        let ready = expectation(description: "known")
        office.whenKeychainKnown { ready.fulfill() }
        memory.holdAsks = false
        for _ in 0..<3 { memory.release.signal() }
        wait(for: [ready], timeout: 10)
        XCTAssertTrue(office.hasWriteSecret)
    }

    func testLaunchReadsOnlyWhetherTheyAreThereNotTheirData() {
        let memory = Memory()
        memory.set("secret-for-test", .writeSecret)
        memory.set("AAAAAAAAAAAAAAAAAAAAAA", .id)
        memory.set("key-for-test", .key)
        let config = Config.shared
        let before = config.viewer
        defer { config.viewer = before }
        config.viewer.baseURL = "https://viewer.example.com"
        let office = office(memory)
        known(office)
        XCTAssertEqual(memory.dataReads, 0, "no id, key or secret read at launch")
        XCTAssertTrue(office.hasWriteSecret)
        let state = office.setupState()
        XCTAssertEqual(state["hasLink"] as? Bool, true, "an id and key are there")
        XCTAssertTrue(state["link"] is NSNull, "the reply doesn't carry it unless asked")
        XCTAssertEqual(state["hasSecret"] as? Bool, true)
        XCTAssertNil(office.link, "off: the key isn't in memory")
    }

    func testTheKeyIsReadWhenTheLinkIsAskedFor() {
        let memory = Memory()
        let key = ViewerCrypto.base64URL(Data(repeating: 7, count: 32))
        memory.set("secret-for-test", .writeSecret)
        memory.set("AAAAAAAAAAAAAAAAAAAAAA", .id)
        memory.set(key, .key)
        let config = Config.shared
        let before = config.viewer
        defer { config.viewer = before }
        config.viewer.baseURL = "https://viewer.example.com"
        let office = office(memory)
        known(office)
        XCTAssertEqual(memory.dataReads, 0)

        let got = expectation(description: "link")
        var link: String?
        office.currentLink { link = $0; got.fulfill() }
        wait(for: [got], timeout: 10)
        XCTAssertEqual(link, "https://viewer.example.com/v/AAAAAAAAAAAAAAAAAAAAAA#k=\(key)")
        XCTAssertNil(office.link, "read for the reply, not kept")
    }

    func testStopWhileStartWaitsOnTheKeychainKeepsItOff() {
        let memory = Memory()
        memory.set("secret-for-test", .writeSecret)
        let config = Config.shared
        let before = config.viewer
        defer { config.viewer = before }
        config.viewer.baseURL = "https://viewer.example.com"
        let office = office(memory)
        known(office)

        memory.holdReads = true
        let told = expectation(description: "start told")
        office.start { told.fulfill() }
        XCTAssertEqual(memory.started.wait(timeout: .now() + 10), .success, "Start is waiting on the Keychain")
        mainStillAnswers()

        office.stop()
        memory.holdReads = false
        for _ in 0..<4 { memory.release.signal() }
        wait(for: [told], timeout: 10)
        XCTAssertFalse(office.isOn, "the Stop cancelled the Start")
    }
}
