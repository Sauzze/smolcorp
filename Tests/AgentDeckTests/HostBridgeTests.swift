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
    /// Where the stand-ins' tab actions run, as the terminals' queue.
    private let terminals = DispatchQueue(label: "test.terminals")

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
            open: { [unowned self] terminal, folder, script in asked.append("open \(terminal) \(folder) \(script)"); return Openers.Opened(tab: "iterm:AB12", window: "iterm:AB12") },
            secrets: [
                "summary-key": HostBridge.Secret(read: { [unowned self] in kept["summary-key"] },
                                                 write: { [unowned self] in kept["summary-key"] = $0; return true },
                                                 delete: { [unowned self] in kept["summary-key"] = nil }),
                "viewer-key": HostBridge.Secret(read: { nil }, write: { _ in false }, delete: nil),
            ],
            loginItem: (get: { [unowned self] in login }, set: { [unowned self] in login = $0 }),
            queue: { [terminals] _ in terminals },
            windowQueue: { [terminals] _ in terminals })
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
        let notTaken = ask(["action": "insert", "tab": "iterm:BD22-06F6", "text": "hello"])
        XCTAssertFalse(isOK(notTaken), "the terminal didn't take it")
        XCTAssertNil(notTaken["unsupported"])
        XCTAssertTrue(isOK(ask(["action": "focus", "tab": "tmux:%1@/tmp/s"])))
        let cant = ask(["action": "focus", "tab": "ghostty:/dev/ttys004"])
        XCTAssertEqual(cant["error"] as? String, "This terminal can't do that.")
        XCTAssertEqual(cant["unsupported"] as? Bool, true, "the core's errUnsupported, not a tab that's gone")
        XCTAssertTrue(isOK(ask(["action": "scroll", "tab": "tmux:%1@/tmp/s", "lines": -5000])))
        XCTAssertTrue(isOK(ask(["action": "scroll-bottom", "tab": "tmux:%1@/tmp/s"])))
        XCTAssertEqual(asked, ["type hello there iterm:BD22-06F6", "insert hello iterm:BD22-06F6", "focus tmux:%1@/tmp/s",
                               "focus ghostty:/dev/ttys004", "scroll -\(Terminal.maxScroll) tmux:%1@/tmp/s", "bottom tmux:%1@/tmp/s"])
    }

    func testCloseSaysWhatTheFiringDid() {
        XCTAssertEqual(ask(["action": "close", "tab": "tmux:%1@/tmp/s", "agent": "claude"])["fired"] as? String, "fired")
        XCTAssertEqual(ask(["action": "close", "tab": "tmux:%2@/tmp/s", "agent": "codex"])["fired"] as? String, "gone")
        XCTAssertEqual(ask(["action": "close", "tab": "tmux:%3@/tmp/s", "agent": "grok"])["fired"] as? String, "not-stopped")
        let none = ask(["action": "close", "tab": "tmux:%4@/tmp/s", "agent": "claude"])
        XCTAssertFalse(isOK(none), "no driver")
        XCTAssertEqual(none["unsupported"] as? Bool, true)
    }

    /// The core gives up at 20 s: an action that hasn't started by the bridge's deadline is never run.
    func testATabActionTheCoreGaveUpOnIsNeverRunLate() {
        // A slow AppleScript ahead of it holds the terminals' queue.
        let release = DispatchSemaphore(value: 0)
        terminals.async { release.wait() }
        let line = try! JSONSerialization.data(withJSONObject: ["token": token, "action": "type", "tab": "tty:/dev/ttys003", "text": "hello"])
        let late = HostBridge.answer(line, token: token, actions: actions, until: .now() + 0.2)
        XCTAssertEqual(late["error"] as? String, "Terminal isn't answering, so nothing was typed. Try again in a moment.")
        release.signal()
        terminals.sync {}
        XCTAssertTrue(asked.isEmpty, "not typed once the queue was free: the core had given up")
        // In time, it runs.
        XCTAssertTrue(isOK(HostBridge.answer(line, token: token, actions: actions, until: .now() + 5)))
        XCTAssertEqual(asked, ["type hello tty:/dev/ttys003"])
        XCTAssertLessThan(HostBridge.actionWait, 20, "under the core's own wait (desktop/hostbridge_darwin.go)")
        // Running, it's waited for; done, its answer is given.
        if case .done(let value) = HostBridge.run(on: terminals, until: .now() + 5, { 42 }) { XCTAssertEqual(value, 42) } else { XCTFail("not run") }
    }

    /// A tab action runs on its own terminal's queue, as the deck's keys do: a tmux tab's never waits behind
    /// Terminal.app, and one its terminal doesn't get to within the deck's input wait is never run.
    func testATabActionWaitsOnlyOnItsOwnTerminal() {
        let tmux = DispatchQueue(label: "test.tmux")
        var actions = self.actions
        actions.queue = { [terminals] tab in tab?.kind == "tmux" ? tmux : terminals }
        let release = DispatchSemaphore(value: 0)
        terminals.async { release.wait() } // Terminal.app not answering
        func line(_ tab: String) -> Data {
            try! JSONSerialization.data(withJSONObject: ["token": token, "action": "type", "tab": tab, "text": "hello"])
        }
        let started = Date()
        XCTAssertTrue(isOK(HostBridge.answer(line("tmux:%1@/tmp/s"), token: token, actions: actions, until: .now() + HostBridge.actionWait)))
        XCTAssertLessThan(Date().timeIntervalSince(started), 1)
        XCTAssertEqual(asked, ["type hello tmux:%1@/tmp/s"])
        // Terminal.app's: given the deck's input wait to start, not the bridge's whole 18 s.
        let late = HostBridge.answer(line("tty:/dev/ttys003"), token: token, actions: actions, until: .now() + HostBridge.actionWait)
        XCTAssertEqual(late["error"] as? String, "Terminal isn't answering, so nothing was typed. Try again in a moment.")
        XCTAssertLessThan(Date().timeIntervalSince(started), Terminals.inputWait + 1.5)
        release.signal()
        terminals.sync {}
        XCTAssertEqual(asked, ["type hello tmux:%1@/tmp/s"], "never typed once Terminal was free")
        XCTAssertTrue(Terminals.inputWait < HostBridge.actionWait)
    }

    /// A tab action is given the deck's input wait to start, not to finish: one
    /// that starts in time is waited for to its end and answered as it went, as
    /// a close that takes longer than that; one that never starts is refused and
    /// never runs. The core is never told "failed" for keys that went.
    func testATabActionStartedInTimeIsAnsweredAsItWent() {
        let tmux = DispatchQueue(label: "test.tmux"), iterm = DispatchQueue(label: "test.iterm")
        let log = NSLock()
        var actions = self.actions
        actions.queue = { [terminals] tab in ["tmux": tmux, "iterm": iterm][tab?.kind ?? ""] ?? terminals }
        actions.type = { [unowned self] text, tab in
            Thread.sleep(forTimeInterval: 1) // its keys take a while
            log.lock(); asked.append("type \(text) \(tab.text)"); log.unlock()
            return true
        }
        actions.fire = { [unowned self] tab, agent in
            Thread.sleep(forTimeInterval: Terminals.inputWait + 1) // stopping the agent, then the tab closed
            log.lock(); asked.append("fire \(agent.rawValue) \(tab.text)"); log.unlock()
            return .fired
        }
        func line(_ request: [String: Any]) -> Data { try! JSONSerialization.data(withJSONObject: request.merging(["token": token]) { $1 }) }
        terminals.async { Thread.sleep(forTimeInterval: Terminals.inputWait - 0.5) } // Terminal.app slow, but in time
        let release = DispatchSemaphore(value: 0)
        tmux.async { release.wait() } // tmux not answering at all
        var answers: [String: [String: Any]] = [:]
        let group = DispatchGroup()
        let started = Date()
        for (name, request) in [
            ("slow", ["action": "type", "tab": "tty:/dev/ttys003", "text": "hello"]),
            ("never", ["action": "type", "tab": "tmux:%1@/tmp/s", "text": "hello"]),
            ("close", ["action": "close", "tab": "iterm:AB12", "agent": "claude"]),
        ] as [(String, [String: Any])] {
            DispatchQueue.global().async(group: group) { [token, actions] in
                let answer = HostBridge.answer(line(request), token: token, actions: actions, until: .now() + HostBridge.actionWait)
                log.lock(); answers[name] = answer; log.unlock()
            }
        }
        group.wait()
        XCTAssertLessThan(Date().timeIntervalSince(started), HostBridge.actionWait - 5)
        XCTAssertTrue(isOK(answers["slow"] ?? [:]), "\(answers["slow"] ?? [:])")
        XCTAssertEqual(answers["close"]?["fired"] as? String, "fired")
        XCTAssertEqual(answers["never"]?["error"] as? String, "tmux isn't answering, so nothing was typed. Try again in a moment.")
        release.signal()
        tmux.sync {}
        XCTAssertEqual(Set(asked), ["type hello tty:/dev/ttys003", "fire claude iterm:AB12"], "the tmux one never ran")

        // Still running at the bridge's own deadline: said to have stopped part way, not "nothing typed".
        if case .unfinished = HostBridge.run(on: terminals, startBy: .now() + 1, until: .now() + 0.2, { Thread.sleep(forTimeInterval: 0.5) }) {} else {
            XCTFail("not unfinished")
        }
        // A script that timed out once something may have gone: the deck's own words.
        actions.type = { _, _ in Tool.run("/bin/sleep", ["5"], timeout: 0.2) != nil }
        let partWay = HostBridge.answer(line(["action": "type", "tab": "iterm:AB12", "text": "hello"]), token: token, actions: actions,
                                        until: .now() + 5)
        XCTAssertEqual(partWay["error"] as? String, "iTerm2 stopped answering part way. Look at the desk before you try again.")
        actions.type = { _, _ in Terminal.onlyLooking { Tool.run("/bin/sleep", ["5"], timeout: 0.2) != nil } }
        let nothing = HostBridge.answer(line(["action": "type", "tab": "iterm:AB12", "text": "hello"]), token: token, actions: actions,
                                        until: .now() + 5)
        XCTAssertEqual(nothing["error"] as? String, "iTerm2 isn't answering, so nothing was typed. Try again in a moment.")
    }

    /// A window, as a tab action: on the terminals' queue, and never once the core has given up,
    /// so a retry doesn't open a second one.
    func testAWindowTheCoreGaveUpOnIsNeverOpened() throws {
        let script = hires.appendingPathComponent("hire-1.sh")
        try Data("#!/bin/sh\n".utf8).write(to: script)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
        let line = try JSONSerialization.data(withJSONObject: ["token": token, "action": "open", "terminal": "iterm",
                                                               "folder": folder.path, "script": script.path])
        let release = DispatchSemaphore(value: 0)
        terminals.async { release.wait() }
        let late = HostBridge.answer(line, token: token, actions: actions, until: .now() + 0.2)
        XCTAssertEqual(late["error"] as? String, "The terminal didn't answer in time.")
        release.signal()
        terminals.sync {}
        XCTAssertTrue(asked.isEmpty, "not opened once the queue was free")
        var onQueue = false
        var actions = self.actions
        actions.open = { [terminals] _, _, _ in
            dispatchPrecondition(condition: .onQueue(terminals))
            onQueue = true
            return Openers.Opened(tab: "iterm:AB12", window: "iterm:AB12")
        }
        XCTAssertEqual(HostBridge.answer(line, token: token, actions: actions, until: .now() + 5)["tab"] as? String, "iterm:AB12")
        XCTAssertTrue(onQueue, "opened on the terminals' queue")
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

        // A folder inside hires that's a link to somewhere else: the script behind it isn't a hire's.
        let elsewhere = folder.appendingPathComponent("elsewhere", isDirectory: true)
        try FileManager.default.createDirectory(at: elsewhere, withIntermediateDirectories: true)
        let behind = elsewhere.appendingPathComponent("hire-2.sh")
        try Data("#!/bin/sh\n".utf8).write(to: behind)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: behind.path)
        try FileManager.default.createSymbolicLink(at: hires.appendingPathComponent("linked"), withDestinationURL: elsewhere)
        XCTAssertFalse(HostBridge.isHireScript(hires.appendingPathComponent("linked/hire-2.sh").path, in: hires))
        XCTAssertTrue(HostBridge.isHireScript(script.path, in: hires))
    }

    /// The opener gets the script's path as it is, as the deck's hire gives it: each
    /// terminal's AppleScript quotes it itself, and a path quoted twice never runs.
    func testTheOpenerIsGivenTheRawPath() throws {
        let spaced = hires.appendingPathComponent("hire dir/it's.sh")
        try FileManager.default.createDirectory(at: spaced.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("#!/bin/sh\n".utf8).write(to: spaced)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: spaced.path)
        var opened: [String] = []
        var actions = HostBridge.Actions.onThisMac(testCopy: true, opener: { command, folder, terminal in
            opened.append("\(terminal)|\(folder)|\(command)")
            return Openers.Opened(tab: "iterm:AB12", window: nil)
        })
        actions.hires = hires
        let line = try JSONSerialization.data(withJSONObject: ["token": token, "action": "open", "terminal": "iterm",
                                                               "folder": folder.path, "script": spaced.path])
        XCTAssertEqual(HostBridge.answer(line, token: token, actions: actions)["tab"] as? String, "iterm:AB12")
        XCTAssertEqual(opened, ["iterm|\(folder.path)|\(spaced.path)"], "not \(Openers.quoted(spaced.path))")
    }

    /// A hire's window, as the deck records it (T796): the tab the deck knows, and the window's own record,
    /// which for Terminal carries the window's id beside the tty. A tab with no window of its own gives none.
    func testAnOpenedWindowIsAnsweredWithItsRecord() throws {
        let script = hires.appendingPathComponent("hire-3.sh")
        try Data("#!/bin/sh\n".utf8).write(to: script)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
        var actions = self.actions
        let line = try JSONSerialization.data(withJSONObject: ["token": token, "action": "open", "terminal": "terminal",
                                                               "folder": folder.path, "script": script.path])
        actions.open = { _, _, _ in Openers.Opened(tab: "tty:/dev/ttys004", window: HireWindows.terminalRecord(tty: "/dev/ttys004", windowID: "1234")) }
        let answer = HostBridge.answer(line, token: token, actions: actions)
        XCTAssertEqual(answer["tab"] as? String, "tty:/dev/ttys004")
        XCTAssertEqual(answer["window"] as? String, "tty:/dev/ttys004#w1234")
        actions.open = { _, _, _ in Openers.Opened(tab: "tty:/dev/ttys005", window: nil) }
        let bare = HostBridge.answer(line, token: token, actions: actions)
        XCTAssertEqual(bare["tab"] as? String, "tty:/dev/ttys005")
        XCTAssertNil(bare["window"], "no window record when the terminal gave none")
    }

    /// Which of the three terminals are here: what the core offers as setup's choices.
    func testTheInstalledTerminalsAreSaid() {
        var actions = self.actions
        actions.terminals = { ["terminal", "ghostty"] }
        let line = try! JSONSerialization.data(withJSONObject: ["token": token, "action": "terminals"])
        let answer = HostBridge.answer(line, token: token, actions: actions)
        XCTAssertTrue(isOK(answer))
        XCTAssertEqual(answer["terminals"] as? [String], ["terminal", "ghostty"])
        XCTAssertEqual(HostBridge.answer(line, token: "other-token-of-32-characters-long!", actions: actions)["ok"] as? Bool, false)
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

    /// A test copy's login item is the copy in use's: the bridge neither reads nor sets it.
    func testATestCopyLeavesTheLoginItemAlone() {
        var touched = false
        let item: (get: () -> Bool, set: (Bool) -> Void) = (get: { touched = true; return true }, set: { _ in touched = true })
        let copy = HostBridge.Actions.onThisMac(testCopy: true, loginItem: item)
        for request: [String: Any] in [["action": "login-item"], ["action": "login-item", "on": false]] {
            var request = request
            request["token"] = token
            let answer = HostBridge.answer(try! JSONSerialization.data(withJSONObject: request), token: token, actions: copy)
            XCTAssertFalse(isOK(answer))
            XCTAssertEqual(answer["error"] as? String, "A test copy has no login item: it's the copy in use's.")
        }
        XCTAssertFalse(touched)
        XCTAssertNotNil(HostBridge.Actions.onThisMac(testCopy: false, loginItem: item).loginItem, "the copy in use's own")
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
        bridge.core = getpid() // this test stands in for the core
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
        // One a launch left behind: nothing listens there any more.
        let left = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        withUnsafeMutableBytes(of: &address.sun_path) { $0.copyBytes(from: socket.path.utf8.prefix($0.count - 1)) }
        let bound = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { Darwin.bind(left, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
        }
        XCTAssertEqual(bound, 0)
        close(left)
        XCTAssertTrue(FileManager.default.fileExists(atPath: socket.path))
        let first = HostBridge(socket: socket, token: token, actions: actions)
        try first.start()
        defer { first.stop() }
        first.core = getpid() // this test stands in for the core
        XCTAssertNotNil(try XCTUnwrap(send(#"{"token":"\#(token)","action":"hello"}"#, to: socket.path)).range(of: "true"))

        // A live one is another Smolcorp's on this folder: never taken over.
        let second = HostBridge(socket: socket, token: "fake-token-second-second-second-00", actions: actions)
        XCTAssertThrowsError(try second.start()) { XCTAssertEqual($0.localizedDescription, HostBridge.Failure.inUse.localizedDescription) }
        XCTAssertNotNil(try XCTUnwrap(send(#"{"token":"\#(token)","action":"hello"}"#, to: socket.path)).range(of: "true"),
                        "the first still has its bridge")

        let file = folder.appendingPathComponent("plain")
        try Data().write(to: file)
        XCTAssertThrowsError(try HostBridge(socket: file, actions: actions).start(), "a file that isn't a socket is left alone")
        XCTAssertTrue(FileManager.default.fileExists(atPath: file.path))
        let long = folder.appendingPathComponent(String(repeating: "x", count: 120))
        XCTAssertThrowsError(try HostBridge(socket: long, actions: actions).start())
    }

    /// A request has `requestWait` in all, however slowly it's sent, and only so many are served at once.
    func testASlowOrCrowdingClientIsCutOff() throws {
        let socket = folder.appendingPathComponent("bridge.sock")
        let bridge = HostBridge(socket: socket, token: token, actions: actions)
        try bridge.start()
        defer { bridge.stop() }
        bridge.core = getpid() // this test stands in for the core
        func connect() -> Int32 {
            let fd = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
            var address = sockaddr_un()
            address.sun_family = sa_family_t(AF_UNIX)
            withUnsafeMutableBytes(of: &address.sun_path) { $0.copyBytes(from: socket.path.utf8.prefix($0.count - 1)) }
            _ = withUnsafePointer(to: &address) {
                $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { Darwin.connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
            }
            return fd
        }
        // As many as are served, each sending nothing yet.
        let held = (0..<HostBridge.connectionsMost).map { _ in connect() }
        defer { held.forEach { close($0) } }
        Thread.sleep(forTimeInterval: 0.3)
        // One more is closed at once.
        let extra = connect()
        var byte: UInt8 = 0
        var wait = timeval(tv_sec: 2, tv_usec: 0)
        setsockopt(extra, SOL_SOCKET, SO_RCVTIMEO, &wait, socklen_t(MemoryLayout<timeval>.size))
        XCTAssertEqual(read(extra, &byte, 1), 0, "closed, not left waiting")
        close(extra)

        // One that sends a byte a second never gets its whole request in.
        let start = Date()
        let slow = held[0]
        setsockopt(slow, SOL_SOCKET, SO_RCVTIMEO, &wait, socklen_t(MemoryLayout<timeval>.size))
        var closed = false
        for character in Array(#"{"token":"\#(token)","action":"hello"}"#.utf8) {
            var one = character
            if write(slow, &one, 1) != 1 { closed = true; break }
            if read(slow, &byte, 1) == 0 { closed = true; break } // waits up to 2 s, the next byte's pace
        }
        XCTAssertTrue(closed)
        XCTAssertLessThan(Date().timeIntervalSince(start), HostBridge.requestWait + 3)
    }

    /// The token alone opens nothing: only the core this app started is served,
    /// at whatever name its socket has been moved to.
    func testOnlyTheCoreIsServedTokenOrNot() throws {
        let socket = folder.appendingPathComponent("bridge.sock")
        let bridge = HostBridge(socket: socket, token: token, actions: actions)
        try bridge.start()
        defer { bridge.stop() }
        let request = #"{"token":"\#(token)","action":"secret-read","item":"summary-key"}"#
        XCTAssertNil(bridge.core)
        XCTAssertEqual(send(request, to: socket.path), "", "no core started yet: closed unanswered")
        bridge.core = getppid()
        XCTAssertEqual(send(request, to: socket.path), "", "not the core's pid")

        // Moved aside, as another process of yours would to listen in its place: still only the core's.
        let kept = folder.appendingPathComponent("kept.sock")
        try FileManager.default.moveItem(at: socket, to: kept)
        bridge.core = getpid()
        // Another process, with the right token.
        let other = Process()
        other.executableURL = URL(fileURLWithPath: "/usr/bin/nc")
        other.arguments = ["-U", kept.path]
        let toOther = Pipe(), fromOther = Pipe()
        other.standardInput = toOther
        other.standardOutput = fromOther
        other.standardError = FileHandle.nullDevice
        try other.run()
        try toOther.fileHandleForWriting.write(contentsOf: Data((request + "\n").utf8))
        let heard = fromOther.fileHandleForReading.readDataToEndOfFile()
        other.waitUntilExit()
        XCTAssertEqual(heard, Data(), "another process is given nothing")
        try? toOther.fileHandleForWriting.close()

        // The core itself, at the new name.
        XCTAssertNotNil(try XCTUnwrap(send(request, to: kept.path)).range(of: "sk-ant-fake"))
        try FileManager.default.moveItem(at: kept, to: socket)
    }

    /// Setup's Mac parts and the desks' abilities, on stand-ins for what asks macOS: each request of the shared
    /// fixture is answered exactly as it says (desktop/hostbridge_test.go checks the core sends and reads the same).
    func testSetupsMacPartsAnswerAsTheSharedFixture() throws {
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("fixtures/bridge/actions.json")
        let fixture = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])
        let cases = try XCTUnwrap(fixture["cases"] as? [[String: Any]])
        let facts = try XCTUnwrap((cases.first?["answer"] as? [String: Any])?["facts"] as? [String: Any])
        var mac = actions
        mac.main = terminals
        mac.facts = { [unowned self] apps in asked.append("facts \(apps.joined(separator: ","))"); return facts }
        mac.dock = { [unowned self] in asked.append("dock \($0)") }
        mac.askAccessibility = { [unowned self] in asked.append("accessibility") }
        mac.keystrokesOn = { [unowned self] in asked.append("keystrokes on") }
        mac.prepareDictation = { [unowned self] language, done in
            asked.append("dictation \(language)")
            done(language == "fr_FR" ? "Apple's model for French couldn't be downloaded." : nil)
        }
        mac.terminalCommand = { [unowned self] add in
            asked.append("terminal command \(add)")
            throw TerminalCommand.Problem.testCopy
        }
        mac.roster = { [GrokBotApp.Row(id: "0a1b-2c3d", name: "Smolcorp Mini Manager")] }
        func text(_ object: Any) -> String {
            String(decoding: (try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])) ?? Data(), as: UTF8.self)
        }
        XCTAssertGreaterThanOrEqual(cases.count, 10)
        for one in cases {
            var request = try XCTUnwrap(one["request"] as? [String: Any])
            request["token"] = token
            let answer = HostBridge.answer(try JSONSerialization.data(withJSONObject: request), token: token, actions: mac)
            XCTAssertEqual(text(answer), text(one["answer"] ?? NSNull()), one["name"] as? String ?? "")
        }
        // Only what each asked for was done; a language it doesn't know and a refused app did nothing.
        XCTAssertEqual(asked, ["facts com.example.notes", "dock false", "accessibility", "keystrokes on", "dictation fr_FR", "terminal command true"])
    }

    func testSetupsMacPartsTakeOnlyWhatTheyShould() {
        var mac = actions
        mac.main = terminals
        mac.facts = { _ in ["accessibility": true] }
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "facts", "apps": "com.example.notes"]), token: token, actions: mac)), "apps not a list")
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "facts", "apps": Array(repeating: "a.b", count: 51)]), token: token, actions: mac)))
        XCTAssertTrue(isOK(HostBridge.answer(line(["action": "facts"]), token: token, actions: mac)), "no apps: none")
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "dock"]), token: token, actions: mac)), "neither on nor off")
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "abilities", "tabs": Array(repeating: "tty:/dev/ttys001", count: 257)]), token: token, actions: mac)))
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "bots-resolve", "bots": [["bot": "No id"]]]), token: token, actions: mac)))
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "bots-resolve", "bots": [["botId": "../0a1b", "bot": "Mini"]]]), token: token, actions: mac)),
                       "an id no bot can have")
        XCTAssertFalse(isOK(HostBridge.answer(line(["action": "keystroke-apps", "app": "com.example.mail"]), token: token, actions: mac)), "neither on nor off")
        // No roster to read: nobody found, as the Swift engine answers.
        let none = HostBridge.answer(line(["action": "bots-resolve", "bots": [["botId": "", "bot": "Mini"]]]), token: token, actions: mac)
        XCTAssertEqual((none["resolved"] as? [Any])?.first is NSNull, true)
        // A tab Smolcorp can only watch, or none at all: nothing offered.
        XCTAssertEqual(HostBridge.abilities(of: "headless:3f2a9c1e"), ["keys": false, "text": false, "fire": false, "screen": false])
        XCTAssertEqual(HostBridge.abilities(of: "not a tab"), ["keys": false, "text": false, "fire": false, "screen": false])
    }

    private func line(_ request: [String: Any]) -> Data {
        var request = request
        request["token"] = token
        return try! JSONSerialization.data(withJSONObject: request)
    }

    func testEachLaunchHasItsOwnToken() {
        let one = HostBridge.newToken(), two = HostBridge.newToken()
        XCTAssertEqual(one.count, 64)
        XCTAssertNotEqual(one, two)
    }
}
