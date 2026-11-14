import XCTest
@testable import AgentDeck

/// osascript runs as Smolcorp (it needs Automation and Accessibility for Music, Terminal and Tiler), so it may only
/// run the app's own fixed scripts (SANDBOX-PLAN §3.5, ST1s): no text a hire can give (a tty, a session's id, a
/// folder, a command, a line to type) is ever part of a script's source. It reaches the script as `argv`, or on
/// osascript's standard input as `givenText`, and the script treats it as data.
///
/// Read from the app's own Swift files (`SwiftSource`), and run against a stand-in osascript that records what it's
/// given. No real app is told anything (no Automation prompt).
final class AppleScriptFixedTests: XCTestCase {
    /// Each interpolation an AppleScript source in the app may hold, in which function, and why no hire's text can
    /// be in it.
    static let interpolations: [(file: String, function: String, code: String, why: String)] = [
        ("Terminal.swift", "script", "source", "wraps a source in `on run argv`; Terminal.run's callers pass fixed scripts (checked)"),
        ("Terminal.swift", "withTab", "body", "its callers pass literals (checked)"),
        ("TerminalDrivers.swift", "withSession", "body", "ITerm's: its callers pass literals (checked)"),
        ("TerminalDrivers.swift", "withTerminal", "body", "Ghostty's: its callers pass literals (checked)"),
        ("Tiler.swift", "osascriptArguments", "source", "wraps a source; tile passes only listScript or moveScript (checked)"),
        ("Tiler.swift", "listScript", "app", "Tiler.scriptable's answer, \"Terminal\" or \"iTerm\" (checked)"),
        ("Tiler.swift", "listScript", "inner", "one of two literals just above it"),
        ("Tiler.swift", "moveScript", "app", "Tiler.scriptable's answer, \"Terminal\" or \"iTerm\" (checked)"),
        ("Music.swift", "isPlaying", "app", "\"Spotify\" or \"Music\", from a literal list (checked)"),
    ]

    static func allowed(_ code: String, in source: SwiftSource, at offset: Int) -> Bool {
        let function = source.enclosing(offset)
        return interpolations.contains { $0.file == source.name && $0.function == function && $0.code == code }
    }

    /// Whether a literal is an AppleScript source.
    static func isAppleScript(_ text: String) -> Bool {
        ["tell application", "of argv", "givenText", "on run", "end tell"].contains(where: text.contains)
    }

    /// A literal, or a `Self.name` constant that is one, holding only what `interpolations` lists for its file
    /// (a wrapper's own `\(body)`, Music's `\(app)`).
    static func isFixed(_ argument: String, in source: SwiftSource, at offset: Int) -> Bool {
        var text = argument
        if let label = text.range(of: #"^\w+:\s*"#, options: .regularExpression) { text.removeSubrange(label) }
        if text.hasPrefix("Self.") {
            // Its value is the literal right after the "=" (in the code a literal is blank: found in the source).
            guard let declared = source.matches(#"\bstatic let \#(text.dropFirst(5))\s*="#).first,
                  let literal = source.literals.first(where: { $0.range.lowerBound >= declared.range.upperBound }),
                  source.text(declared.range.upperBound..<literal.range.lowerBound).allSatisfy(\.isWhitespace) else { return false }
            text = source.text(literal.range)
        }
        guard text.hasPrefix("\""), text.hasSuffix("\"") else { return false }
        let interpolated = try! NSRegularExpression(pattern: #"\\\(([^)]*)\)"#)
        return interpolated.matches(in: text, range: NSRange(text.startIndex..., in: text)).allSatisfy { match in
            let code = (text as NSString).substring(with: match.range(at: 1)).trimmingCharacters(in: .whitespaces)
            return allowed(code, in: source, at: offset)
        }
    }

    /// The calls of `name` in `source`, declarations left out: each one's offset and arguments.
    static func calls(of name: String, in source: SwiftSource, qualified: Bool = false) -> [(offset: Int, arguments: [String])] {
        let escaped = NSRegularExpression.escapedPattern(for: name)
        let pattern = qualified ? #"\b\#(escaped)\s*\("# : #"(?<![.\w])\#(escaped)\s*\("#
        return source.matches(pattern).compactMap { match in
            let before = source.codeText.utf8.prefix(match.range.lowerBound)
            if String(decoding: before.suffix(5), as: UTF8.self) == "func " { return nil }
            return (match.range.lowerBound, source.arguments(at: match.range.upperBound - 1))
        }
    }

    func testOnlyFixedTextIsInAnAppleScriptsSource() throws {
        let sources = try SwiftSource.app()
        var found: [(file: String, function: String, code: String)] = []
        for source in sources {
            for literal in source.literals where Self.isAppleScript(source.text(literal.range)) {
                for code in literal.interpolations {
                    let text = source.text(code).trimmingCharacters(in: .whitespaces)
                    found.append((source.name, source.enclosing(code.lowerBound), text))
                    XCTAssertTrue(Self.allowed(text, in: source, at: code.lowerBound),
                                  "\(source.name):\(source.line(of: code.lowerBound)): \\(\(text)) is in an AppleScript's source: pass it as argv or givenText")
                }
            }
        }
        for allowed in Self.interpolations
        where !found.contains(where: { $0.file == allowed.file && $0.function == allowed.function && $0.code == allowed.code }) {
            XCTFail("\(allowed.file) \(allowed.function)'s \\(\(allowed.code)) isn't in any AppleScript any more: take it off the list")
        }
    }

    /// What reaches those interpolations is fixed text at every call.
    func testEveryScriptTheAppRunsIsFixedAtItsCall() throws {
        let sources = try SwiftSource.app()
        var checked = 0
        for source in sources {
            var scripts = Self.calls(of: "Terminal.run", in: source, qualified: true).map { ($0, 0) }
            if source.name == "Terminal.swift" { scripts += Self.calls(of: "run", in: source).map { ($0, 0) } }
            scripts += Self.calls(of: "withTab", in: source).map { call in (call, call.arguments.firstIndex { $0.hasPrefix("do:") } ?? -1) }
            scripts += Self.calls(of: "withSession", in: source).map { ($0, 1) }
            scripts += Self.calls(of: "withTerminal", in: source).map { ($0, 1) }
            scripts += Self.calls(of: "ask", in: source).filter { _ in source.name == "Music.swift" }.map { ($0, 0) }
            for (call, index) in scripts {
                checked += 1
                let script = call.arguments.indices.contains(index) ? call.arguments[index] : "(none)"
                XCTAssertTrue(Self.isFixed(script, in: source, at: call.offset), "\(source.name):\(source.line(of: call.offset)): the script \(script.prefix(60)) isn't fixed text")
            }
        }
        XCTAssertGreaterThanOrEqual(checked, 40, "the scripts' calls were found")

        let music = try XCTUnwrap(sources.first { $0.name == "Music.swift" })
        XCTAssertEqual(music.matches(#"for app in (\[[^\]]*\])"#).map { $0.groups[0] }, [#"["Spotify", "Music"]"#])
        let tiler = try XCTUnwrap(sources.first { $0.name == "Tiler.swift" })
        XCTAssertEqual(Set(Self.calls(of: "script", in: tiler).map { $0.arguments.first ?? "" }),
                       ["listScript(app: app)", "moveScript(app: app)"])
        let bridge = try XCTUnwrap(sources.first { $0.name == "HostBridge.swift" })
        XCTAssertEqual(bridge.matches(#"let app = Tiler\.scriptable\((\w+)\)"#).count, 1)
        XCTAssertEqual(Self.calls(of: "Tiler.tile", in: bridge, qualified: true).map { $0.arguments.first ?? "" }, ["app: app"])
        for terminal in ["terminal", "iterm", "Terminal", "ghostty", "Terminal\" to do shell script \"id", ""] {
            XCTAssertTrue([nil, "Terminal", "iTerm"].contains(Tiler.scriptable(terminal)), terminal)
        }
    }

    // MARK: Run, against a stand-in osascript

    private var folder: URL!
    private var saved = ""
    /// What a hire could put in a tty, a session's id, a folder or a line: AppleScript of its own.
    private let hostile = #"x" & (do shell script "id") & "\(y) end tell é"#

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let standIn = folder.appendingPathComponent("osascript")
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
        Ghostty.forget(tty: hostile)
        try? FileManager.default.removeItem(at: folder)
    }

    private var calls: [(arguments: [String], stdin: String?)] {
        let count = ((try? FileManager.default.contentsOfDirectory(atPath: folder.path)) ?? []).filter { $0.hasPrefix("call-") }.count
        return (0..<count).map { n in
            let call = folder.appendingPathComponent("call-\(n)")
            var arguments: [String] = []
            while let data = try? Data(contentsOf: call.appendingPathComponent("arg-\(arguments.count)")) {
                arguments.append(String(decoding: data, as: UTF8.self))
            }
            return (arguments, (try? Data(contentsOf: call.appendingPathComponent("stdin"))).map { String(decoding: $0, as: UTF8.self) })
        }
    }

    func testAHiresTextReachesTheTerminalsScriptsOnlyAsData() {
        _ = Terminal.type(hostile, tty: hostile)
        _ = Terminal.open(hostile, in: hostile)
        _ = Terminal.contents(tty: hostile)
        _ = Terminal.focus(tty: hostile)
        let session = TabAddress(kind: "iterm", id: hostile)
        _ = ITerm().type(hostile, in: session)
        _ = ITerm().insert(hostile, in: session)
        _ = ITerm().press(.init(key: .escape), in: session)
        _ = ITerm().screen(of: session)
        _ = ITerm().focus(tty: hostile)
        Ghostty.learn(tty: hostile)
        _ = Ghostty().insert(hostile, in: TabAddress(kind: "ghostty", id: hostile))

        let calls = calls
        XCTAssertGreaterThanOrEqual(calls.count, 10, "Ghostty's insert runs only once it has learnt the terminal")
        var asData = 0
        for call in calls {
            XCTAssertEqual(call.arguments.first, "-e")
            let script = call.arguments.count > 1 ? call.arguments[1] : ""
            XCTAssertFalse(script.contains("do shell script"), "a hire's text in a script's source: \(script)")
            asData += call.arguments.dropFirst(2).filter { $0 == hostile }.count + (call.stdin == hostile ? 1 : 0)
        }
        XCTAssertGreaterThanOrEqual(asData, 12, "the text arrived whole, as argv or givenText")
    }

    func testAHiresTtyNeverReachesTilersScripts() {
        let tty = #"/dev/ttys9"&(do•shell•script•"id")&""#
        var recorded: [(source: String, arguments: [String])] = []
        let script: Tiler.Script = { source, arguments in
            recorded.append((source, arguments))
            return recorded.count == 1 ? .ok("7 \(tty)") : .ok("10 20 300 400")
        }
        let moved = Tiler.tile(app: "Terminal", places: [(tty: tty, rect: CGRect(x: 10, y: 20, width: 300, height: 400))], script: script)
        XCTAssertEqual(moved?.placed, [tty], "the window was found by its tty, in Swift")
        XCTAssertEqual(recorded.count, 2)
        for call in recorded {
            XCTAssertFalse(call.source.contains("do•shell"), call.source)
            XCTAssertFalse(call.arguments.contains { $0.contains("do•shell") }, "\(call.arguments)")
        }
        XCTAssertEqual(recorded.last?.arguments, ["7", "10", "20", "300", "400"])
    }
}
