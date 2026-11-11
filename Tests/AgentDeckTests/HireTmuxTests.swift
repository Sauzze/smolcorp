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

    func testStartIsDetachedInTheFolderOnItsOwnSocket() {
        answer = (0, "%0 /private/tmp/tmux-501/smolcorp-ab12-cd34")
        let made = desk().start(in: "/work/app", shell: "/bin/zsh", running: "/x/hire.sh")
        XCTAssertEqual(made?.pane, "%0")
        XCTAssertEqual(made?.socket, "/private/tmp/tmux-501/smolcorp-ab12-cd34")
        XCTAssertEqual(asked, [["-L", "smolcorp-ab12-cd34", "new-session", "-d", "-P", "-F", "#{pane_id} #{socket_path}",
                                "-s", "hire", "-c", "/work/app", "/bin/zsh", "-lic", "'/x/hire.sh'"]])
    }

    func testStartFailsWhenTmuxDoes() {
        answer = (1, "")
        XCTAssertNil(desk().start(in: "/w", shell: "/bin/zsh", running: "x"))
        answer = (0, "garbage")
        XCTAssertNil(desk().start(in: "/w", shell: "/bin/zsh", running: "x"))
        answer = nil
        XCTAssertNil(desk().start(in: "/w", shell: "/bin/zsh", running: "x"))
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

    func testPasteStateReadsTheTwoFlags() {
        let want: [String: (inMode: Bool, bracketed: Bool)] = ["0 1": (false, true), "1 0": (true, false), "0 0": (false, false)]
        for (out, expected) in want {
            answer = (0, out)
            let got = desk().pasteState()
            XCTAssertEqual(got?.inMode, expected.inMode, out)
            XCTAssertEqual(got?.bracketed, expected.bracketed, out)
        }
        XCTAssertEqual(asked.first, ["-L", "smolcorp-ab12-cd34", "display-message", "-p", "-t", HireTmux.pane, "#{pane_in_mode} #{bracket_paste_flag}"])
    }

    /// A tmux before 3.7 has no bracket_paste_flag and leaves it empty: not bracketed, not "couldn't say".
    /// `Tool.run` trims the answer, so the Mac's tmux hands over "0" or "1"; the untrimmed forms are the same.
    func testPasteStateOnATmuxBefore37TakesTheEmptyFlagAsNotBracketed() {
        for (out, inMode) in [("0 ", false), ("1 ", true), ("0", false), ("1", true)] {
            answer = (0, out)
            let got = desk().pasteState()
            XCTAssertNotNil(got, out)
            XCTAssertEqual(got?.inMode, inMode, out)
            XCTAssertEqual(got?.bracketed, false, out)
        }
    }

    func testPasteStateRefusesWhatIsMalformed() {
        for bad in ["", " ", " 1", "0  1", "0 1 2", "a b", "0 x"] {
            answer = (0, bad)
            XCTAssertNil(desk().pasteState(), "\"\(bad)\" was read")
        }
        answer = (1, "0 1")
        XCTAssertNil(desk().pasteState(), "a failed call was read")
        answer = nil
        XCTAssertNil(desk().pasteState())
    }

    func testKillEndsOnlyThisServer() {
        XCTAssertTrue(desk().kill())
        XCTAssertEqual(asked, [["-L", "smolcorp-ab12-cd34", "kill-server"]])
    }

    func testEveryCallNamesOurSocket() {
        let hire = desk()
        _ = hire.start(in: "/w", shell: "/bin/sh", running: "x"); _ = hire.isAlive(); _ = hire.capture(lines: 3)
        _ = hire.paste("t"); _ = hire.kill()
        XCTAssertFalse(asked.isEmpty)
        XCTAssertTrue(asked.allSatisfy { $0.prefix(2) == ["-L", "smolcorp-ab12-cd34"] })
    }

    func testTheViewerAttachesToThatSocketOnly() {
        XCTAssertEqual(desk().attachLine(tmuxPath: "/opt/homebrew/bin/tmux"),
                       "exec '/opt/homebrew/bin/tmux' -L 'smolcorp-ab12-cd34' attach-session -t '=hire'")
        XCTAssertEqual(desk(port: 7811).attachLine(tmuxPath: "/Applications/My Tools/tmux"),
                       "exec '/Applications/My Tools/tmux' -L 'smolcorp-7811-ab12-cd34' attach-session -t '=hire'")
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
}
