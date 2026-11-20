import XCTest
@testable import AgentDeck

/// SK3 (SANDBOX-PLAN §3.3), the app's half: the Keychain only after the instance lock, the bridge by the core's audit
/// token, and the API key only to Anthropic's address. Nothing here opens the Keychain gate (that would let the rest
/// of the suite reach the real Keychain) or asks the Keychain anything: the gate is checked with stand-in calls.
final class HostBridgeHardeningTests: XCTestCase {
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("sk3-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    // MARK: The Keychain only after the instance lock

    /// Shut, the gate answers "not available" without asking: never "not found", which would make a new GM token
    /// and write it over the real one.
    func testTheShutGateAsksTheKeychainNothing() {
        var asked = 0
        let shut = KeychainGate.call(when: false) { asked += 1; return errSecSuccess }
        XCTAssertEqual(shut, errSecNotAvailable)
        XCTAssertEqual(asked, 0)
        XCTAssertNotEqual(shut, errSecItemNotFound)
        XCTAssertEqual(GMKeychain.result(status: shut, data: nil), .failed, "a shut gate is a failed read, never \"no token\"")
        XCTAssertEqual(KeychainGate.call(when: true) { asked += 1; return errSecItemNotFound }, errSecItemNotFound)
        XCTAssertEqual(asked, 1)
    }

    /// The test suite holds no instance lock, so the gate is shut here, and a lock not held opens nothing.
    func testTheGateOpensOnlyForALockHeld() {
        XCTAssertFalse(KeychainGate.isOpen)
        KeychainGate.open(holding: InstanceLock(support: folder))
        XCTAssertFalse(KeychainGate.isOpen, "a lock not taken")
    }

    /// Every SecItem call in the app goes through the gate: no other file makes one.
    func testOnlyTheGateAsksTheKeychain() throws {
        let sources = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("Sources/AgentDeck")
        let call = try NSRegularExpression(pattern: #"\bSec(Item(CopyMatching|Add|Update|Delete)|Keychain\w*)\s*\("#)
        var callers: [String] = []
        for name in try FileManager.default.contentsOfDirectory(atPath: sources.path) where name.hasSuffix(".swift") {
            let text = try String(contentsOf: sources.appendingPathComponent(name), encoding: .utf8)
            if call.firstMatch(in: text, range: NSRange(text.startIndex..., in: text)) != nil { callers.append(name) }
        }
        XCTAssertEqual(callers, ["KeychainGate.swift"])
    }

    /// One holder at a time per folder: a second Smolcorp on it waits, and takes it once the first lets go.
    func testTheInstanceLockHasOneHolder() throws {
        let first = InstanceLock(support: folder), second = InstanceLock(support: folder)
        XCTAssertTrue(first.take())
        XCTAssertTrue(first.isHeld)
        XCTAssertTrue(first.take(), "taken again by its holder: still held")
        XCTAssertFalse(second.take())
        XCTAssertFalse(second.isHeld)
        let mode = try FileManager.default.attributesOfItem(atPath: first.file.path)[.posixPermissions] as? Int
        XCTAssertEqual(mode, 0o600)
        first.release()
        XCTAssertTrue(second.take())
        second.release()

        // A folder not made yet is made, yours alone.
        let fresh = InstanceLock(support: folder.appendingPathComponent("new"))
        XCTAssertTrue(fresh.take())
        fresh.release()
    }

    /// A link at the lock's name is refused, never followed: the file it names is left as it was.
    func testTheInstanceLockFollowsNoLink() throws {
        let elsewhere = folder.appendingPathComponent("elsewhere")
        try Data("left alone".utf8).write(to: elsewhere)
        let lock = InstanceLock(support: folder)
        try FileManager.default.createSymbolicLink(at: lock.file, withDestinationURL: elsewhere)
        XCTAssertFalse(lock.take())
        XCTAssertEqual(try String(contentsOf: elsewhere, encoding: .utf8), "left alone")
    }

    // MARK: The bridge by audit token

    func testAProcessAuditTokenIsItsPidItsVersionAndItsUser() throws {
        let mine = try XCTUnwrap(HostBridge.auditToken(of: getpid()))
        XCTAssertEqual(mine.val.5, UInt32(bitPattern: getpid()))
        XCTAssertEqual(mine.val.1, getuid())
        XCTAssertTrue(HostBridge.sameProcess(mine, mine))

        // The same pid in another version is another process: one that got the core's pid after it ended.
        var later = mine
        later.val.7 &+= 1
        XCTAssertFalse(HostBridge.sameProcess(later, mine))
        var other = mine
        other.val.5 &+= 1
        XCTAssertFalse(HostBridge.sameProcess(other, mine))
        var root = mine
        root.val.1 = 0
        XCTAssertFalse(HostBridge.sameProcess(root, root), "not this user's")

        XCTAssertNil(HostBridge.auditToken(of: 0))
        XCTAssertNil(HostBridge.auditToken(of: -1))
        // A process that has ended has no token.
        let ended = Process()
        ended.executableURL = URL(fileURLWithPath: "/usr/bin/true")
        try ended.run()
        ended.waitUntilExit()
        if kill(ended.processIdentifier, 0) != 0 { XCTAssertNil(HostBridge.auditToken(of: ended.processIdentifier)) }
    }

    /// A connection's other end, as the system set it: this process, when it connects to itself.
    func testTheBridgeReadsTheConnectionsAuditToken() throws {
        let socket = folder.appendingPathComponent("b.sock")
        let bridge = HostBridge(socket: socket, token: "fake-token-sk3", actions: .mac)
        try bridge.start()
        defer { bridge.stop() }
        bridge.core = getpid() // this test stands in for the core
        XCTAssertEqual(bridge.core, getpid())

        let fd = socket.path.withCString { path -> Int32 in
            let fd = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
            var address = sockaddr_un()
            address.sun_family = sa_family_t(AF_UNIX)
            withUnsafeMutableBytes(of: &address.sun_path) { $0.copyBytes(from: UnsafeRawBufferPointer(start: path, count: strlen(path))) }
            let connected = withUnsafePointer(to: &address) {
                $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
            }
            return connected == 0 ? fd : -1
        }
        XCTAssertGreaterThanOrEqual(fd, 0)
        defer { close(fd) }
        let peer = try XCTUnwrap(HostBridge.peerToken(fd), "this end's peer is the bridge, in this process")
        let mine = try XCTUnwrap(HostBridge.auditToken(of: getpid()))
        XCTAssertTrue(HostBridge.sameProcess(peer, mine))
        bridge.core = nil
        XCTAssertNil(bridge.core)
    }

    // MARK: The API key only to Anthropic

    func testTheAPIKeyGoesOnlyToAnthropicsAddress() {
        let request = DictationCleanup.request(text: "fix this text please", glossary: [], key: "sk-ant-fake")
        XCTAssertEqual(request.url, DictationCleanup.endpoint)
        XCTAssertEqual(DictationCleanup.endpoint.host, "api.anthropic.com")
        XCTAssertEqual(DictationCleanup.endpoint.scheme, "https")

        // Anywhere else, nothing is sent: answered at once, as never answered.
        var elsewhere = request
        elsewhere.url = URL(string: "http://127.0.0.1:9/v1/messages")
        var answered: (Data?, Int)?
        DictationCleanup.sendToAnthropic(elsewhere) { answered = ($0, $1) }
        XCTAssertNotNil(answered)
        XCTAssertNil(answered?.0)
        XCTAssertEqual(answered?.1, 0)

        // A redirect isn't followed: it would carry the key's header to wherever it points.
        let redirect = HTTPURLResponse(url: DictationCleanup.endpoint, statusCode: 307, httpVersion: nil,
                                       headerFields: ["Location": "https://example.invalid/"])!
        var followed: URLRequest?? = .none
        let task = URLSession(configuration: .ephemeral).dataTask(with: request)
        DictationCleanup.NoRedirects().urlSession(URLSession.shared, task: task, willPerformHTTPRedirection: redirect,
                                                  newRequest: URLRequest(url: URL(string: "https://example.invalid/")!)) { followed = .some($0) }
        XCTAssertEqual(followed, .some(nil))
    }
}
