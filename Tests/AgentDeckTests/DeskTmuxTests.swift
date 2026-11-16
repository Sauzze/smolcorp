import XCTest
@testable import AgentDeck

/// SA-M9: the app's tmux on a desk's own server, through stand-ins (no real tmux, no terminal, no window). A desk's
/// pane is driven only with the desk the core gave (`TmuxDesk`): a checked server by the bundled tmux alone, told its
/// pid; a desk from before the sandbox "unchecked"; a desk's socket with no desk, nothing at all. A tmux of your own is
/// told "unchecked". A window shown again runs `smolcorp attach` with `exec`, and closes once that has ended.
final class DeskTmuxTests: XCTestCase {
    private var folder: URL!
    private var saved: (path: String?, bundled: String?)
    private var bundledLog: URL { folder.appendingPathComponent("bundled") }
    private var otherLog: URL { folder.appendingPathComponent("other") }
    private let parent = "/private/tmp/smolcorp-\(getuid())"
    private var deskPane: TabAddress { TabAddress("tmux:%3@\(parent)/ab12/s")! }

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("dt-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        // Two stand-ins, the bundled tmux and the usual places' (Homebrew's): each says its guard and its words.
        for (name, log) in [("bundled-tmux", bundledLog), ("other-tmux", otherLog)] {
            let program = folder.appendingPathComponent(name)
            try """
                #!/bin/sh
                printf '%s|%s\\n' "${SMOLCORP_TMUX_SERVER-UNSET}" "$*" >> '\(log.path)'
                case "$*" in *pane_id*) echo %3 ;; esac
                """.write(to: program, atomically: true, encoding: .utf8)
            try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: program.path)
        }
        saved = (Tmux.path, Tmux.bundled)
        Tmux.bundled = folder.appendingPathComponent("bundled-tmux").path
        Tmux.path = folder.appendingPathComponent("other-tmux").path
    }

    override func tearDownWithError() throws {
        (Tmux.path, Tmux.bundled) = saved
        try? FileManager.default.removeItem(at: folder)
    }

    private func lines(_ log: URL) -> [String] {
        ((try? String(contentsOf: log, encoding: .utf8)) ?? "").split(separator: "\n").map(String.init)
    }

    /// With no desk from the core, a desk's socket, either form, gets nothing run: no key, no look, no fire, no focus.
    func testADesksSocketGetsNothingWithoutItsDesk() {
        for tab in [deskPane, TabAddress("tmux:%3@/private/tmp/tmux-501/smolcorp-ab12")!] {
            XCTAssertFalse(Tmux().press(.init(key: .returnKey), in: tab))
            XCTAssertNil(Tmux().screen(of: tab))
            XCTAssertNil(Tmux().isOpen(tab))
            XCTAssertEqual(Tmux().fire(tab, agent: .claude), .notStopped)
            XCTAssertFalse(Tmux().focus(tab))
        }
        XCTAssertEqual(lines(bundledLog) + lines(otherLog), [], "nothing ran")
    }

    /// A desk whose server the core checked: the bundled tmux alone, told that pid, with no start file.
    func testACheckedDeskIsDrivenByTheBundledTmuxToldItsServer() {
        let tmux = Tmux(desk: TmuxDesk(id: "ab12", server: "5151"))
        XCTAssertTrue(tmux.press(.init(key: .returnKey), in: deskPane))
        XCTAssertEqual(tmux.isOpen(deskPane), true)
        XCTAssertEqual(lines(bundledLog), ["5151|-f /dev/null -S \(parent)/ab12/s send-keys -t %3 Enter",
                                           "5151|-f /dev/null -S \(parent)/ab12/s display-message -p -t %3 #{pane_id}"])
        XCTAssertEqual(lines(otherLog), [], "never Homebrew's")
        // No bundled tmux (a build without one): nothing at all, never another tmux.
        Tmux.bundled = nil
        XCTAssertFalse(tmux.press(.init(key: .returnKey), in: deskPane))
        XCTAssertEqual(lines(otherLog), [])
    }

    /// A desk from before the sandbox: today's tmux, "unchecked", as the core tells its own calls there.
    func testAnOldDeskIsTodaysTmuxUnchecked() {
        let old = TabAddress("tmux:%3@/private/tmp/tmux-501/smolcorp-ab12")!
        XCTAssertTrue(Tmux(desk: TmuxDesk(id: "ab12", server: TmuxClient.unchecked)).press(.init(key: .returnKey), in: old))
        XCTAssertEqual(lines(otherLog), ["unchecked|-f /dev/null -S /private/tmp/tmux-501/smolcorp-ab12 send-keys -t %3 Enter"])
    }

    /// A plain hire's own server (SA-M10, `<tmux folder>/smolcorp-plain-<12>`) has no registry row: the core sends its own
    /// record of it, "unchecked" (MSYSTACK4, RT126SAM9's condition), so its keys, focus and close reach it through today's
    /// tmux, as a tmux of your own's do. Without that desk, its socket is named as a hire's, so nothing runs.
    func testAPlainHiresOwnServerIsDrivenWithTheDeskTheCoreSent() throws {
        let plain = TabAddress("tmux:%3@/private/tmp/tmux-501/smolcorp-plain-0123456789ab")!
        XCTAssertFalse(Tmux().press(.init(key: .returnKey), in: plain))
        XCTAssertFalse(Tmux().focus(plain))
        XCTAssertEqual(lines(bundledLog) + lines(otherLog), [], "nothing without its desk")
        let sent: [String: Any] = ["id": "plain-0123456789ab", "server": "unchecked", "pane": 4242, "paneStarted": 1794000000000000, "folder": "/w"]
        let desk = try XCTUnwrap(TmuxDesk(sent))
        XCTAssertEqual(desk, TmuxDesk(id: "plain-0123456789ab", server: TmuxClient.unchecked, pane: 4242, paneStarted: 1794000000000000, folder: "/w"))
        XCTAssertTrue(Tmux(desk: desk).press(.init(key: .returnKey), in: plain))
        XCTAssertEqual(Tmux(desk: desk).isOpen(plain), true)
        XCTAssertEqual(lines(otherLog), ["unchecked|-f /dev/null -S /private/tmp/tmux-501/smolcorp-plain-0123456789ab send-keys -t %3 Enter",
                                         "unchecked|-f /dev/null -S /private/tmp/tmux-501/smolcorp-plain-0123456789ab display-message -p -t %3 #{pane_id}"])
        XCTAssertEqual(lines(bundledLog), [], "today's tmux, as the core's own calls there")
        // Its window shown again: by its session and socket, as the core's own window for it (no row for its id).
        XCTAssertEqual(Openers.attachLine(desk.id, socket: plain.socket, smolcorp: "/b/smolcorp", testPort: nil, testSupport: "/x"),
                       "exec '/b/smolcorp' attach --session 'hire' --socket '/private/tmp/tmux-501/smolcorp-plain-0123456789ab'")
    }

    /// A tmux of your own: told "unchecked" (a stock tmux ignores it), its words as ever.
    func testYourOwnTmuxIsToldUnchecked() {
        XCTAssertTrue(Tmux().press(.init(key: .returnKey), in: TabAddress("tmux:%3@/private/tmp/tmux-501/default")!))
        XCTAssertEqual(lines(otherLog), ["unchecked|-S /private/tmp/tmux-501/default send-keys -t %3 Enter"])
    }

    /// A desk's fire never asks its server what runs in its pane: with none recorded, it's left, and only #{pane_id} was asked.
    func testADesksFireNeverAsksItsServerWhatRunsInItsPane() {
        XCTAssertEqual(Tmux(desk: TmuxDesk(id: "ab12", server: "5151")).fire(deskPane, agent: .claude), .notStopped)
        XCTAssertEqual(lines(bundledLog), ["5151|-f /dev/null -S \(parent)/ab12/s display-message -p -t %3 #{pane_id}"])
        XCTAssertFalse(lines(bundledLog).contains { $0.contains("pane_pid") || $0.contains("pane_tty") || $0.contains("kill-pane") })
    }

    /// A pid used again: the recorded start doesn't match, so the kernel's tty isn't given.
    func testAProcessIsKnownOnlyByItsPidAndItsStart() throws {
        let sleeper = Process()
        sleeper.executableURL = URL(fileURLWithPath: "/bin/sleep")
        sleeper.arguments = ["30"]
        try sleeper.run()
        defer { sleeper.terminate() }
        XCTAssertNil(Processes.terminal(of: sleeper.processIdentifier, startedAt: 1), "another start")
        XCTAssertNil(Processes.terminal(of: 1, startedAt: 0))
        XCTAssertNil(Processes.terminal(of: 0, startedAt: 0))
    }

    // MARK: The window shown again

    /// `smolcorp attach` with `exec`, each word quoted; a test copy's names its port and folder first, as the core's
    /// window scripts do.
    func testAWindowShownAgainRunsSmolcorpAttachWithExec() {
        XCTAssertEqual(Openers.attachLine("ab12", smolcorp: "/Applications/Smol corp.app/Contents/Resources/bin/smolcorp", testPort: nil, testSupport: "/x"),
                       "exec '/Applications/Smol corp.app/Contents/Resources/bin/smolcorp' attach 'ab12'")
        XCTAssertEqual(Openers.attachLine("ab12", smolcorp: "/b/smolcorp", testPort: 7811, testSupport: "/Users/me/test's copy"),
                       "export SMOLCORP_PORT='7811'\nexport SMOLCORP_TEST_SUPPORT_DIR='/Users/me/test'\\''s copy'\nexec '/b/smolcorp' attach 'ab12'")
    }

    /// Each terminal runs the script with `exec`, so no shell is left in the tab once it ends (RT126SAM8 N3).
    func testEachTerminalExecsItsScript() throws {
        let sources = try SwiftSource.app()
        let terminal = try XCTUnwrap(sources.first { $0.name == "Terminal.swift" }.map { $0.text(0..<$0.bytes.count) })
        let drivers = try XCTUnwrap(sources.first { $0.name == "TerminalDrivers.swift" }.map { $0.text(0..<$0.bytes.count) })
        XCTAssertTrue(terminal.contains(#"do script "cd " & quoted form of (item 1 of argv) & " && exec " & quoted form of (item 2 of argv)"#))
        XCTAssertTrue(drivers.contains(#"write text "cd " & quoted form of (item 1 of argv) & " && exec " & quoted form of (item 2 of argv)"#))
        XCTAssertTrue(drivers.contains(#"initial input:"exec " & (quoted form of (item 2 of argv)) & linefeed"#))
    }

    /// A stand-in closer: what it was asked to close, and what its look says.
    private final class Closer: HireWindows.Closer {
        var closed: [String] = []
        var looked: [String] = []
        var shows: HireWindows.Shown = .unknown
        let lock = NSLock()
        func look(_ window: String) -> HireWindows.Shown { lock.lock(); defer { lock.unlock() }; looked.append(window); return shows }
        func close(_ window: String, tty: String) -> HireWindows.Closed {
            lock.lock(); defer { lock.unlock() }
            closed.append("\(window) \(tty)")
            return .closed
        }
        var all: [String] { lock.lock(); defer { lock.unlock() }; return closed }
    }

    private func process(_ seconds: String) throws -> Process {
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/bin/sleep")
        p.arguments = [seconds]
        try p.run()
        addTeardownBlock { if p.isRunning { p.terminate() } }
        return p
    }

    /// The window closes once its tab's own process has ended, on its terminal's queue, by its record and tty.
    func testTheWindowClosesOnceItsProcessHasEnded() throws {
        let tab = try process("1"), closer = Closer(), queue = DispatchQueue(label: "test.closing")
        Closing.whenDone("tty:/dev/ttys042#w7", queue: queue, closer: closer, shortest: 0,
                         root: { $0 == "/dev/ttys042" ? tab.processIdentifier : nil })
        XCTAssertTrue(eventually(within: 10) { closer.all == ["tty:/dev/ttys042#w7 /dev/ttys042"] }, "\(closer.all)")
        // iTerm2's session, its tty from iTerm2 itself.
        let other = try process("1"), iterm = Closer()
        iterm.shows = .open("/dev/ttys043")
        Closing.whenDone("iterm:AB12", queue: queue, closer: iterm, shortest: 0, root: { $0 == "/dev/ttys043" ? other.processIdentifier : nil })
        XCTAssertTrue(eventually(within: 10) { iterm.all == ["iterm:AB12 /dev/ttys043"] }, "\(iterm.all)")
    }

    /// Ended at once (a refusal): left, so what it said can be read. Still running: left. Ghostty, which can't say its
    /// tty: never watched.
    func testAWindowWhoseProcessEndedAtOnceOrStillRunsIsLeft() throws {
        let quick = try process("0"), running = try process("30"), closer = Closer(), queue = DispatchQueue(label: "test.closing")
        Closing.whenDone("tty:/dev/ttys042#w7", queue: queue, closer: closer, shortest: 60, root: { _ in quick.processIdentifier })
        Closing.whenDone("tty:/dev/ttys044#w8", queue: queue, closer: closer, shortest: 0, root: { _ in running.processIdentifier })
        var rooted = false
        Closing.whenDone("ghostty-id:9F", queue: queue, closer: closer, shortest: 0, root: { _ in rooted = true; return nil })
        quick.waitUntilExit()
        Thread.sleep(forTimeInterval: 1.5)
        queue.sync {}
        XCTAssertEqual(closer.all, [])
        XCTAssertFalse(rooted)
    }
}
