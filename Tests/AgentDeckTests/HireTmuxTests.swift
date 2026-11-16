import XCTest
@testable import AgentDeck

/// A hire's own tmux server, through a runner that records the arguments, so no real tmux runs.
final class HireTmuxTests: XCTestCase {
    private var asked: [[String]] = []
    private var answer: (status: Int32, output: String)? = (0, "")

    private func desk(_ id: String = "ab12-cd34", port: UInt16? = nil) -> HireTmux {
        HireTmux(hireID: id, testPort: port) { self.asked.append($0); return self.answer }!
    }

    func testTheSocketIsNamedAfterTheHire() {
        XCTAssertEqual(desk().socketName, "smolcorp-ab12-cd34")
        XCTAssertEqual(desk(port: 7811).socketName, "smolcorp-7811-ab12-cd34")
    }

    func testAnIdThatCouldEscapeTheNameIsRefused() {
        for id in ["", "../x", "A B", "a/b", "a;b", "é"] {
            XCTAssertNil(HireTmux(hireID: id, testPort: nil) { _ in nil }, id)
        }
    }

    func testIsAliveIsAskedByExactName() {
        XCTAssertEqual(desk().isAlive(), true)
        answer = (1, "")
        XCTAssertEqual(desk().isAlive(), false)
        answer = nil
        XCTAssertNil(desk().isAlive())
        XCTAssertEqual(asked.first, ["-L", "smolcorp-ab12-cd34", "has-session", "-t", "=hire"])
    }

    func testCaptureReadsTheLastLines() {
        answer = (0, "one\ntwo")
        XCTAssertEqual(desk().capture(lines: 40), "one\ntwo")
        XCTAssertEqual(asked, [["-L", "smolcorp-ab12-cd34", "capture-pane", "-p", "-t", "=hire:", "-S", "-40"]])
        answer = (1, "")
        XCTAssertNil(desk().capture(lines: 5))
        asked = []
        _ = desk().capture(lines: 0)
        _ = desk().capture(lines: 999_999)
        XCTAssertEqual(asked.map { $0.last }, ["-1", "-5000"])
    }

    /// Text is pasted whole, and nothing presses Return.
    func testPasteIsBracketedAndNeverEnter() {
        XCTAssertTrue(desk().paste("ajoute un test; puis lance-le"))
        XCTAssertEqual(asked, [
            ["-L", "smolcorp-ab12-cd34", "set-buffer", "-b", "smolcorp-paste", "--", "ajoute un test; puis lance-le"],
            ["-L", "smolcorp-ab12-cd34", "paste-buffer", "-p", "-r", "-d", "-b", "smolcorp-paste", "-t", "=hire:"],
        ])
        XCTAssertFalse(asked.joined().contains("send-keys"))
    }

    /// A ";" at the end reaches the hire: tmux would take it for the end of set-buffer and drop it.
    func testASemicolonAtTheEndIsPastedToo() {
        XCTAssertTrue(desk().paste("lance les tests;"))
        XCTAssertEqual(asked.first, ["-L", "smolcorp-ab12-cd34", "set-buffer", "-b", "smolcorp-paste", "--", #"lance les tests\;"#])
    }

    /// Escape and Return could press Enter, so those texts never reach tmux.
    func testControlCharactersAreNotPasted() {
        for text in ["a\u{1B}[201~\rb", "a\rb", "a\u{7}b", "a\u{85}b"] {
            XCTAssertFalse(desk().paste(text), text.debugDescription)
        }
        XCTAssertTrue(asked.isEmpty)
        XCTAssertTrue(desk().paste("a\nb\tc"))
        XCTAssertEqual(asked.count, 2)
    }

    func testAFailedBufferIsNotPasted() {
        answer = (1, "")
        XCTAssertFalse(desk().paste("x"))
        XCTAssertEqual(asked.count, 1)
    }

    func testKillEndsOnlyThisServer() {
        XCTAssertTrue(desk().kill())
        XCTAssertEqual(asked, [["-L", "smolcorp-ab12-cd34", "kill-server"]])
    }

    func testEveryCallNamesOurSocket() {
        let hire = desk()
        _ = hire.isAlive(); _ = hire.capture(lines: 3)
        _ = hire.paste("t"); _ = hire.kill()
        XCTAssertFalse(asked.isEmpty)
        XCTAssertTrue(asked.allSatisfy { $0.prefix(2) == ["-L", "smolcorp-ab12-cd34"] })
    }

    func testOwnsOnlyOurSockets() {
        XCTAssertTrue(HireTmux.owns("/private/tmp/tmux-501/smolcorp-ab12", testPort: nil))
        XCTAssertTrue(HireTmux.owns("smolcorp-7811-ab12", testPort: nil))
        XCTAssertFalse(HireTmux.owns("/private/tmp/tmux-501/default", testPort: nil))
        XCTAssertFalse(HireTmux.owns(""))
        // A test copy owns only its own port's sockets, never a real install's.
        XCTAssertTrue(HireTmux.owns("smolcorp-7811-ab12", testPort: 7811))
        XCTAssertFalse(HireTmux.owns("smolcorp-ab12", testPort: 7811))
        XCTAssertFalse(HireTmux.owns("smolcorp-7812-ab12", testPort: 7811))
        XCTAssertNil(HireTmux(socketName: "smolcorp-ab12", testPort: 7811) { _ in nil })
        XCTAssertNil(HireTmux(socketName: "default", testPort: nil) { _ in nil })
        XCTAssertNil(HireTmux(socketName: "smolcorp-../x", testPort: nil) { _ in nil })
        XCTAssertEqual(HireTmux(socketName: "smolcorp-ab12", testPort: nil) { _ in nil }?.socketName, "smolcorp-ab12")
    }

    /// SA-M9: a desk started in its sandbox has `s` in a folder of its own inside /tmp/smolcorp-<uid> (the core's
    /// deskSocketPath), a test copy's with its port first. Both forms are a desk's socket; nothing else is.
    func testADesksSocketInItsSandboxIsOursToo() {
        let parent = "/private/tmp/smolcorp-\(getuid())"
        XCTAssertTrue(HireTmux.owns("\(parent)/ab12/s", testPort: nil))
        XCTAssertTrue(HireTmux.owns("/tmp/smolcorp-\(getuid())/ab12/s", testPort: nil))
        XCTAssertTrue(HireTmux.owns("\(parent)/7811-ab12/s", testPort: 7811))
        XCTAssertFalse(HireTmux.owns("\(parent)/ab12/s", testPort: 7811), "a test copy never owns the real app's desks")
        XCTAssertFalse(HireTmux.owns("\(parent)/7812-ab12/s", testPort: 7811), "nor another copy's")
        for other in ["/private/tmp/smolcorp-\(getuid() + 1)/ab12/s", "\(parent)/ab12/t", "\(parent)/AB12/s", "/private/tmp/x/ab12/s",
                      "\(parent)/../ab12/s", "\(parent)/ab12/s/s", "ab12/s", "/private/tmp/tmux-501/default"] {
            XCTAssertFalse(HireTmux.owns(other, testPort: nil), other)
        }
        // Shaped as a desk's socket but not one of ours: never owned, yet still a desk's shape, so the app drives it
        // only with what the core recorded (it fails closed), never as a tmux of your own.
        XCTAssertTrue(HireTmux.isDeskSocket("\(parent)/AB12/s"))
        for other in ["/private/tmp/smolcorp-\(getuid() + 1)/ab12/s", "\(parent)/ab12/t", "/private/tmp/x/ab12/s",
                      "\(parent)/../ab12/s", "\(parent)/ab12/s/s", "ab12/s", "/private/tmp/tmux-501/default"] {
            XCTAssertFalse(HireTmux.isDeskSocket(other), other)
        }
        for desk in ["\(parent)/ab12/s", "\(parent)/7811-ab12/s", "/private/tmp/tmux-501/smolcorp-ab12", "/private/tmp/tmux-501/smolcorp-7811-ab12"] {
            XCTAssertTrue(HireTmux.isDeskSocket(desk), desk)
        }
    }

    // MARK: TmuxClient, the one way the app runs tmux

    private func standIn() throws -> (program: String, log: URL, folder: URL) {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tc-\(UUID().uuidString.prefix(8))", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: folder) }
        let log = folder.appendingPathComponent("runs"), program = folder.appendingPathComponent("tmux")
        // As the gate's stand-in (desktop/tmuxgate_test.go): the guard, whether stdin was /dev/null, the names in its environment, its words.
        try """
            #!/bin/sh
            names=$(/usr/bin/env | /usr/bin/sed -e 's/=.*//' -e '/^PWD$/d' -e '/^SHLVL$/d' -e '/^_$/d' -e '/^OLDPWD$/d' | /usr/bin/sort | /usr/bin/tr '\\n' ',')
            if [ /dev/stdin -ef /dev/null ]; then stdin=null; else stdin=$(/bin/cat); fi
            printf '%s|%s|%s|%s\\n' "${SMOLCORP_TMUX_SERVER-UNSET}" "$stdin" "$names" "$*" >> '\(log.path)'
            """.write(to: program, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: program.path)
        return (program.path, log, folder)
    }

    private func runs(_ log: URL) -> [String] {
        ((try? String(contentsOf: log, encoding: .utf8)) ?? "").split(separator: "\n").map(String.init)
    }

    /// Told its server, with a fixed PATH, TERM and LANG and nothing else of the app's environment, stdin from
    /// /dev/null unless it's given text.
    func testTheClientIsToldItsServerAndNothingElse() throws {
        let tmux = try standIn()
        XCTAssertEqual(TmuxClient.run(tmux.program, server: "5151", ["-S", "/x/s", "list-clients"])?.status, 0)
        XCTAssertEqual(TmuxClient.run(tmux.program, server: TmuxClient.unchecked, ["load-buffer", "-"], input: Data("hé;".utf8))?.status, 0)
        XCTAssertEqual(runs(tmux.log), ["5151|null|LANG,PATH,SMOLCORP_TMUX_SERVER,TERM,|-S /x/s list-clients",
                                        "unchecked|hé;|LANG,PATH,SMOLCORP_TMUX_SERVER,TERM,|load-buffer -"])
    }

    /// Only a pid above 1, as a plain number, or "unchecked": "off" is the core's start alone. Anything else runs nothing.
    func testTheClientRunsOnlyWithAGuard() throws {
        let tmux = try standIn()
        for server in ["", "off", "OFF", "1", "0", "-5", "05", "5151 ", " 5151", "5151;", "abc", "99999999999", "Unchecked"] {
            XCTAssertNil(TmuxClient.run(tmux.program, server: server, ["ls"]), server)
        }
        XCTAssertNil(TmuxClient.run("", server: "5151", ["ls"]))
        XCTAssertEqual(runs(tmux.log), [], "nothing ran")
        XCTAssertTrue(TmuxClient.isGuard("5151") && TmuxClient.isGuard("unchecked") && TmuxClient.isGuard("2"))
    }

    /// An old desk's own server, from a socket's name: "unchecked", as the core tells its own calls there.
    func testAnOldDesksServerIsUnchecked() throws {
        let tmux = try standIn()
        let saved = Tmux.path
        Tmux.path = tmux.program
        defer { Tmux.path = saved }
        XCTAssertEqual(HireTmux(socketName: "smolcorp-ab12", testPort: nil)?.isAlive(), true)
        XCTAssertEqual(runs(tmux.log), ["unchecked|null|LANG,PATH,SMOLCORP_TMUX_SERVER,TERM,|-L smolcorp-ab12 has-session -t =hire"])
    }

    /// The desk the core gives with an action on its pane: its id, its guard, and a folder by its full path, or none.
    func testADeskIsReadAsTheCoreGivesIt() {
        let given: [String: Any] = ["id": "ab12", "server": "5151", "pane": 4242, "paneStarted": NSNumber(value: Int64(1_794_000_000_000_000)),
                                    "folder": "/Users/example/repo-hires/ab12"]
        XCTAssertEqual(TmuxDesk(given), TmuxDesk(id: "ab12", server: "5151", pane: 4242, paneStarted: 1_794_000_000_000_000,
                                                 folder: "/Users/example/repo-hires/ab12"))
        XCTAssertEqual(TmuxDesk(["id": "cd34", "server": "unchecked"]), TmuxDesk(id: "cd34", server: "unchecked"))
        for (name, change) in [("an id that isn't one", ["id": "../x"]), ("no id", ["id": 3]), ("the start's own guard", ["server": "off"]),
                               ("a pid as a number", ["server": 5151]), ("a folder that isn't a full path", ["folder": "repo"])] as [(String, [String: Any])] {
            XCTAssertNil(TmuxDesk(given.merging(change) { $1 }), name)
        }
        XCTAssertNil(TmuxDesk("ab12"))
        XCTAssertNil(TmuxDesk(nil))
    }
}
