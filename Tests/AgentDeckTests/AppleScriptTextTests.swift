import XCTest
@testable import AgentDeck

/// iTerm2 and Ghostty are driven through osascript: here a stand-in that records each call's
/// arguments and what it was given on its standard input, and answers "ok". No real app is
/// ever told anything (no Automation prompt).
final class AppleScriptTextTests: XCTestCase {
    private var folder: URL!
    private var saved = ""
    private let tty = "/dev/ttys077"

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let standIn = folder.appendingPathComponent("osascript")
        // One folder per call: arg-0, arg-1… as given, and stdin when the script reads it.
        try """
            #!/bin/sh
            n=$(ls "\(folder.path)" | grep -c '^call-')
            call="\(folder.path)/call-$n"
            mkdir "$call"
            i=0
            for a in "$@"; do printf '%s' "$a" > "$call/arg-$i"; i=$((i+1)); done
            case "$2" in *fileHandleWithStandardInput*) cat > "$call/stdin" ;; esac
            echo ok
            """.write(to: standIn, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: standIn.path)
        saved = Terminal.osascript
        Terminal.osascript = standIn.path
    }

    override func tearDownWithError() throws {
        Terminal.osascript = saved
        Ghostty.forget(tty: tty)
        try? FileManager.default.removeItem(at: folder)
    }

    /// Each call's arguments and standard input, in order.
    private var calls: [(arguments: [String], stdin: Data?)] {
        let count = ((try? FileManager.default.contentsOfDirectory(atPath: folder.path)) ?? []).filter { $0.hasPrefix("call-") }.count
        return (0..<count).map { n in
            let call = folder.appendingPathComponent("call-\(n)")
            var arguments: [String] = []
            while let data = try? Data(contentsOf: call.appendingPathComponent("arg-\(arguments.count)")) {
                arguments.append(String(decoding: data, as: UTF8.self))
            }
            return (arguments, try? Data(contentsOf: call.appendingPathComponent("stdin")))
        }
    }

    private let text = "Café à l’été, ça va ? « œ » 👩‍💻 $(id) fin;"

    /// As an argument, macOS takes accents apart (é as e and a combining accent): the text
    /// reaches the script on osascript's standard input, byte for byte, never as an argument.
    func testITermGetsTheTextOnStandardInput() throws {
        let session = try XCTUnwrap(TabAddress("iterm:BD2206F6-EE01-476F-A6D9-3061679BD8B0"))
        XCTAssertTrue(ITerm().insert(text, in: session))
        XCTAssertTrue(ITerm().type("/clear", in: session))
        let calls = calls
        XCTAssertEqual(calls.count, 2)
        XCTAssertEqual(calls[0].arguments.count, 3)
        XCTAssertEqual(calls[0].arguments[2], "BD2206F6-EE01-476F-A6D9-3061679BD8B0")
        XCTAssertTrue(calls[0].arguments[1].contains("tell s to write text givenText newline NO"))
        XCTAssertEqual(calls[0].stdin, Data(text.utf8))
        XCTAssertTrue(calls[1].arguments[1].contains("tell s to write text givenText\n"))
        XCTAssertEqual(calls[1].stdin, Data("/clear".utf8))
        for call in calls { XCTAssertFalse(call.arguments.joined().contains("Caf"), "never an argument") }
    }

    /// Ghostty: its paste (`input text`) takes the text from standard input; a key's own bytes stay an argument.
    func testGhosttyGetsTheTextOnStandardInput() throws {
        Ghostty.learn(tty: tty) // the stand-in says "ok": that's the terminal's id here
        let terminal = try XCTUnwrap(TabAddress("ghostty:\(tty)"))
        XCTAssertTrue(Ghostty().insert(text, in: terminal))
        XCTAssertTrue(Ghostty().press(.init(key: .escape), in: terminal))
        let calls = Array(calls.dropFirst()) // the first was learning the terminal
        XCTAssertEqual(calls.count, 2)
        XCTAssertEqual(Array(calls[0].arguments.dropFirst(2)), ["ok", tty])
        XCTAssertTrue(calls[0].arguments[1].contains("input text givenText to t"))
        XCTAssertEqual(calls[0].stdin, Data(text.utf8))
        XCTAssertNil(calls[1].stdin)
        XCTAssertEqual(calls[1].arguments.last, "\\x1b")
        for call in calls { XCTAssertFalse(call.arguments.joined().contains("Caf"), "never an argument") }
    }

    /// The real osascript, told nothing but to give back what it read: the text arrives whole.
    /// No app is addressed.
    func testOsascriptReadsTheTextWhole() throws {
        Terminal.osascript = saved
        guard FileManager.default.isExecutableFile(atPath: saved) else { throw XCTSkip("no osascript") }
        let answer = try XCTUnwrap(Terminal.run("return givenText & \"|\" & (item 1 of argv)", arguments: ["arg"], input: text))
        XCTAssertEqual(Array(answer.unicodeScalars), Array((text + "|arg").unicodeScalars))
        XCTAssertEqual(Terminal.run("return \"plain\""), "plain", "a script reading nothing is as it was")
    }

    /// A test copy started with SMOLCORP_TEST_NO_TERMINAL_APP=1 asks Terminal.app nothing: no
    /// Apple event, so no Automation prompt and no wait on one. Other terminals, and the app
    /// itself, are as ever.
    func testATestCopyCanLeaveTerminalAppAlone() {
        XCTAssertTrue(TestCopy.leavesTerminalAppAlone(testCopy: true, environment: ["SMOLCORP_TEST_NO_TERMINAL_APP": "1"]))
        XCTAssertFalse(TestCopy.leavesTerminalAppAlone(testCopy: false, environment: ["SMOLCORP_TEST_NO_TERMINAL_APP": "1"]), "never the real app")
        XCTAssertFalse(TestCopy.leavesTerminalAppAlone(testCopy: true, environment: [:]))
        XCTAssertFalse(TestCopy.leavesTerminalAppAlone(testCopy: true, environment: ["SMOLCORP_TEST_NO_TERMINAL_APP": "0"]))
        XCTAssertFalse(TestCopy.leavesTerminalAppAlone, "the suite is no test copy")
        let terminalApp = "if application \"Terminal\" is not running then return \"\"\ntell application \"Terminal\" to …"
        XCTAssertTrue(Terminal.leavesAlone(terminalApp, alone: true))
        XCTAssertFalse(Terminal.leavesAlone(terminalApp, alone: false))
        XCTAssertFalse(Terminal.leavesAlone("tell application \"iTerm\" to …", alone: true))
    }
}
