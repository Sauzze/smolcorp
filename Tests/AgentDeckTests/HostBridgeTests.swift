import XCTest
@testable import AgentDeck

/// The bridge the Go core asks this app through: the token, what each action
/// may be given, and the socket, on stand-ins for the terminals and the stores.
final class HostBridgeTests: XCTestCase {
    private let token = "fake-token-fake-token-fake-token-0"
    private var folder: URL!
    private var hires: URL!
    /// What the stand-ins were asked, in order.
    private var asked: [String] = []

    override func setUpWithError() throws {
        // Short, for a socket's path (103 bytes at most).
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("sb-\(UUID().uuidString.prefix(8))", isDirectory: true)
        hires = folder.appendingPathComponent("hires", isDirectory: true)
        try FileManager.default.createDirectory(at: hires, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        asked = []
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    private var kept: [String: String] = ["summary-key": "sk-ant-fake"]
    private var login = false

    private var actions: HostBridge.Actions {
        HostBridge.Actions(
            press: { [unowned self] key, tab in asked.append("press \(key.key) \(key.modifiers.rawValue) \(key.times) \(tab.text)"); return true },
            type: { [unowned self] text, tab in asked.append("type \(text) \(tab.text)"); return true },
            insert: { [unowned self] text, tab in asked.append("insert \(text) \(tab.text)"); return false },
            focus: { [unowned self] tab in asked.append("focus \(tab.text)"); return tab.kind == "tmux" ? true : nil },
            scroll: { [unowned self] lines, tab in asked.append("scroll \(lines) \(tab.text)"); return true },
            scrollToBottom: { [unowned self] tab in asked.append("bottom \(tab.text)"); return true },
            fire: { [unowned self] tab, agent in
                asked.append("fire \(agent.rawValue) \(tab.text)")
                return ["%1": .fired, "%2": .gone, "%3": .notStopped][tab.id]
            },
            hires: hires,
            open: { [unowned self] terminal, folder, script in asked.append("open \(terminal) \(folder) \(script)"); return "iterm:AB12" },
            secrets: [
                "summary-key": HostBridge.Secret(read: { [unowned self] in kept["summary-key"] },
                                                 write: { [unowned self] in kept["summary-key"] = $0; return true },
                                                 delete: { [unowned self] in kept["summary-key"] = nil }),
                "viewer-key": HostBridge.Secret(read: { nil }, write: { _ in false }, delete: nil),
            ],
            loginItem: { [unowned self] in login },
            setLoginItem: { [unowned self] in login = $0 })
    }

    private func ask(_ request: [String: Any], token given: String? = nil) -> [String: Any] {
        var request = request
        request["token"] = given ?? token
        let line = try! JSONSerialization.data(withJSONObject: request)
        return HostBridge.answer(line, token: token, actions: actions)
    }

    private func isOK(_ answer: [String: Any]) -> Bool { answer["ok"] as? Bool == true }

    func testNothingWithoutTheToken() {
        XCTAssertFalse(isOK(ask(["action": "hello"], token: "wrong-token-wrong-token-wrong-tok")))
        XCTAssertFalse(isOK(ask(["action": "hello"], token: "")))
        XCTAssertFalse(isOK(HostBridge.answer(Data(#"{"action":"hello"}"#.utf8), token: token, actions: actions)))
        XCTAssertFalse(isOK(HostBridge.answer(Data("not json".utf8), token: token, actions: actions)))
        XCTAssertFalse(isOK(ask(["action": "focus", "tab": "tmux:%1@/tmp/s"], token: String(token.dropLast()))))
        XCTAssertTrue(asked.isEmpty, "nothing was done")
        XCTAssertTrue(isOK(ask(["action": "hello"])))
        XCTAssertTrue(HostBridge.sameToken(token, token))
        XCTAssertFalse(HostBridge.sameToken("", ""))
    }

    func testTheKeysAreTheDecksOnly() {
        XCTAssertTrue(isOK(ask(["action": "press", "tab": "tty:/dev/ttys003", "key": "escape", "times": 2])))
        XCTAssertTrue(isOK(ask(["action": "press", "tab": "tty:/dev/ttys003", "key": "space"])))
        XCTAssertTrue(isOK(ask(["action": "press", "tab": "tty:/dev/ttys003", "key": "shift-tab"])))
        XCTAssertEqual(asked, ["press escape 0 2 tty:/dev/ttys003", "press space 0 1 tty:/dev/ttys003",
                               "press tab \(CGEventFlags.maskShift.rawValue) 1 tty:/dev/ttys003"])
        for refused: [String: Any] in [
            ["action": "press", "tab": "tty:/dev/ttys003", "key": "cmd-q"],
            ["action": "press", "tab": "tty:/dev/ttys003", "key": "enter", "times": 10],
            ["action": "press", "tab": "tty:/dev/ttys003", "key": "enter", "times": 0],
            ["action": "press", "tab": "/etc/passwd", "key": "enter"],
            ["action": "press", "tab": "tty:/dev/ttys003\nx", "key": "enter"],
            ["action": "type", "tab": "tty:/dev/ttys003", "text": "rm -rf ~\n"],
            ["action": "type", "tab": "tty:/dev/ttys003", "text": ""],
            ["action": "type", "tab": "tty:/dev/ttys003", "text": String(repeating: "a", count: HostBridge.textMost + 1)],
            ["action": "scroll", "tab": "tty:/dev/ttys003", "lines": 0],
            ["action": "close", "tab": "tmux:%1@/tmp/s", "agent": "bash"],
            ["action": "notify", "text": "hello"],
            ["action": "dictate"],
        ] {
            XCTAssertFalse(isOK(ask(refused)), "\(refused)")
        }
        XCTAssertEqual(asked.count, 3, "none of those reached a terminal")
    }

    func testEachTabActionIsTheDrivers() {
        XCTAssertTrue(isOK(ask(["action": "type", "tab": "iterm:BD22-06F6", "text": "hello there"])))
        XCTAssertFalse(isOK(ask(["action": "insert", "tab": "iterm:BD22-06F6", "text": "hello"])), "the terminal didn't take it")
        XCTAssertTrue(isOK(ask(["action": "focus", "tab": "tmux:%1@/tmp/s"])))
        let cant = ask(["action": "focus", "tab": "ghostty:/dev/ttys004"])
        XCTAssertEqual(cant["error"] as? String, "This terminal can't do that.")
        XCTAssertTrue(isOK(ask(["action": "scroll", "tab": "tmux:%1@/tmp/s", "lines": -5000])))
        XCTAssertTrue(isOK(ask(["action": "scroll-bottom", "tab": "tmux:%1@/tmp/s"])))
        XCTAssertEqual(asked, ["type hello there iterm:BD22-06F6", "insert hello iterm:BD22-06F6", "focus tmux:%1@/tmp/s",
                               "focus ghostty:/dev/ttys004", "scroll -\(Terminal.maxScroll) tmux:%1@/tmp/s", "bottom tmux:%1@/tmp/s"])
    }

    func testCloseSaysWhatTheFiringDid() {
        XCTAssertEqual(ask(["action": "close", "tab": "tmux:%1@/tmp/s", "agent": "claude"])["fired"] as? String, "fired")
        XCTAssertEqual(ask(["action": "close", "tab": "tmux:%2@/tmp/s", "agent": "codex"])["fired"] as? String, "gone")
        XCTAssertEqual(ask(["action": "close", "tab": "tmux:%3@/tmp/s", "agent": "grok"])["fired"] as? String, "not-stopped")
        XCTAssertFalse(isOK(ask(["action": "close", "tab": "tmux:%4@/tmp/s", "agent": "claude"])), "no driver")
    }

    func testATerminalOpensOnlyOnAHireScript() throws {
        let script = hires.appendingPathComponent("abc/hire-1.sh")
        try FileManager.default.createDirectory(at: script.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("#!/bin/sh\n".utf8).write(to: script)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
        let open: [String: Any] = ["action": "open", "terminal": "iterm", "folder": folder.path, "script": script.path]
        let answer = ask(open)
        XCTAssertEqual(answer["tab"] as? String, "iterm:AB12")
        XCTAssertEqual(asked, ["open iterm \(folder.path) \(script.path)"])

        let outside = folder.appendingPathComponent("hire.sh")
        try Data("#!/bin/sh\n".utf8).write(to: outside)
        let link = hires.appendingPathComponent("link.sh")
        try FileManager.default.createSymbolicLink(at: link, withDestinationURL: outside)
        let shared = hires.appendingPathComponent("shared.sh")
        try Data("#!/bin/sh\n".utf8).write(to: shared)
        try FileManager.default.setAttributes([.posixPermissions: 0o772], ofItemAtPath: shared.path)
        for (key, value) in [("script", outside.path), ("script", link.path), ("script", shared.path),
                             ("script", hires.path + "/../hire.sh"), ("script", "echo hi; rm -rf ~"), ("script", "abc/hire-1.sh"),
                             ("terminal", "xterm"), ("folder", "relative"), ("folder", script.path)] {
            var request = open
            request[key] = value
            XCTAssertFalse(isOK(ask(request)), "\(key) \(value)")
        }
        XCTAssertEqual(asked.count, 1, "no other window opened")
    }

    func testTheAppsOwnItemsOnly() {
        XCTAssertEqual(ask(["action": "secret-read", "item": "summary-key"])["value"] as? String, "sk-ant-fake")
        XCTAssertTrue(isOK(ask(["action": "secret-write", "item": "summary-key", "value": "sk-ant-other"])))
        XCTAssertEqual(kept["summary-key"], "sk-ant-other")
        XCTAssertTrue(isOK(ask(["action": "secret-delete", "item": "summary-key"])))
        let none = ask(["action": "secret-read", "item": "summary-key"])
        XCTAssertTrue(isOK(none))
        XCTAssertEqual(none["found"] as? Bool, false)
        XCTAssertFalse(isOK(ask(["action": "secret-read", "item": "push-key"])), "not an item the engine may have")
        XCTAssertFalse(isOK(ask(["action": "secret-write", "item": "viewer-key", "value": "x"])), "not kept")
        XCTAssertFalse(isOK(ask(["action": "secret-write", "item": "summary-key", "value": "a\nb"])))
        XCTAssertFalse(isOK(ask(["action": "secret-delete", "item": "viewer-key"])))
        XCTAssertEqual(Set(HostBridge.Actions.mac.secrets.keys), ["summary-key", "gm-token", "viewer-id", "viewer-key", "viewer-write-secret"],
                       "as desktop/hostbridge.go's bridgeSecrets")
    }

    func testTheLoginItem() {
        XCTAssertEqual(ask(["action": "login-item"])["on"] as? Bool, false)
        XCTAssertEqual(ask(["action": "login-item", "on": true])["on"] as? Bool, true)
        XCTAssertTrue(login)
    }

    // MARK: The socket

    /// One request on the socket, as the core sends it.
    private func send(_ line: String, to path: String) -> String? {
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        defer { close(fd) }
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        withUnsafeMutableBytes(of: &address.sun_path) { $0.copyBytes(from: path.utf8.prefix($0.count - 1)) }
        let connected = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
        }
        guard connected == 0 else { return nil }
        let data = Array((line + "\n").utf8)
        guard write(fd, data, data.count) == data.count else { return nil }
        var answer = Data()
        var chunk = [UInt8](repeating: 0, count: 4096)
        while true {
            let count = read(fd, &chunk, chunk.count)
            if count <= 0 { break }
            answer.append(chunk, count: count)
        }
        return String(data: answer, encoding: .utf8)
    }

    func testTheSocketIsYoursAloneAndAsksForTheToken() throws {
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: folder.path)
        let socket = folder.appendingPathComponent("bridge.sock")
        let bridge = HostBridge(socket: socket, token: token, actions: actions)
        try bridge.start()
        defer { bridge.stop() }
        let mode = { (path: String) in (try FileManager.default.attributesOfItem(atPath: path)[.posixPermissions] as? NSNumber)?.intValue }
        XCTAssertEqual(try mode(socket.path), 0o600)
        XCTAssertEqual(try mode(folder.path), 0o700, "its folder made yours alone")

        let answer = try XCTUnwrap(send(#"{"token":"\#(token)","action":"focus","tab":"tmux:%1@/tmp/s"}"#, to: socket.path))
        XCTAssertEqual(answer, "{\"ok\":true}\n")
        let refused = try XCTUnwrap(send(#"{"token":"nope","action":"focus","tab":"tmux:%1@/tmp/s"}"#, to: socket.path))
        XCTAssertTrue(refused.contains("\"ok\":false"))
        XCTAssertEqual(asked, ["focus tmux:%1@/tmp/s"])

        bridge.stop()
        XCTAssertFalse(FileManager.default.fileExists(atPath: socket.path), "removed as it stops")
        XCTAssertNil(send(#"{"token":"\#(token)","action":"hello"}"#, to: socket.path))
    }

    func testAnEarlierSocketIsReplacedButNothingElse() throws {
        let socket = folder.appendingPathComponent("bridge.sock")
        let first = HostBridge(socket: socket, token: token, actions: actions)
        try first.start()
        let second = HostBridge(socket: socket, token: "fake-token-second-second-second-00", actions: actions)
        try second.start()
        defer { second.stop(); first.stop() }
        XCTAssertNotNil(try XCTUnwrap(send(#"{"token":"fake-token-second-second-second-00","action":"hello"}"#, to: socket.path)).range(of: "true"))

        let file = folder.appendingPathComponent("plain")
        try Data().write(to: file)
        XCTAssertThrowsError(try HostBridge(socket: file, actions: actions).start(), "a file that isn't a socket is left alone")
        XCTAssertTrue(FileManager.default.fileExists(atPath: file.path))
        let long = folder.appendingPathComponent(String(repeating: "x", count: 120))
        XCTAssertThrowsError(try HostBridge(socket: long, actions: actions).start())
    }

    func testEachLaunchHasItsOwnToken() {
        let one = HostBridge.newToken(), two = HostBridge.newToken()
        XCTAssertEqual(one.count, 64)
        XCTAssertNotEqual(one, two)
    }
}
