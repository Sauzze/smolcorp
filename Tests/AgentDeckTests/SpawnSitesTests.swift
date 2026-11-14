import XCTest
@testable import AgentDeck

/// Every program the app starts holds none of Smolcorp's permissions (SANDBOX-PLAN §3.5, ST1s): it is made by
/// `Disclaimed.process`, or it is on the exempt list below, each entry with why. These read the app's own Swift
/// files (`SwiftSource`), as desktop's TestEveryProgramStartsDisclaimedOnAMac reads the core's Go.
final class SpawnSitesTests: XCTestCase {
    /// A start the app may make without `Disclaimed.process`: in `file`, inside `function`, a start of `kind` whose
    /// program (what `executableURL` is set to there) is exactly `program`.
    struct Exempt {
        let file: String
        let function: String
        let kind: String
        let program: String
        let why: String
    }

    static let osascript = #"URL(fileURLWithPath: "/usr/bin/osascript")"#
    static let ps = #"URL(fileURLWithPath: "/bin/ps")"#
    static let open = #"URL(fileURLWithPath: "/usr/bin/open")"#

    static let exempt: [Exempt] = [
        Exempt(file: "Disclaimed.swift", function: "process", kind: "Process()", program: "place.spawn",
               why: "the factory itself: smolcorp-spawn, or outside an app with none, the program as it is (nothing to be charged to)"),
        Exempt(file: "Engine.swift", function: "start", kind: "Process()", program: "program",
               why: "the core: Smolcorp itself (Paths.bundledCLI)"),
        Exempt(file: "Music.swift", function: "ask", kind: "Process()", program: osascript,
               why: "Apple's osascript, one fixed script asking Spotify or Music whether it plays (Automation as Smolcorp)"),
        Exempt(file: "Tiler.swift", function: "osascript", kind: "Process()", program: osascript,
               why: "Apple's osascript, Tiler's fixed scripts moving Terminal or iTerm2 windows (Automation as Smolcorp)"),
        Exempt(file: "Terminal.swift", function: "run", kind: "Process()", program: "URL(fileURLWithPath: osascript)",
               why: "Apple's osascript (Terminal.osascript, which only tests set to a stand-in), the terminals' fixed scripts"),
        Exempt(file: "Terminal.swift", function: "commands", kind: "Process()", program: ps,
               why: "Apple's ps, reading what runs on a tty"),
        Exempt(file: "Terminal.swift", function: "processes", kind: "Process()", program: ps,
               why: "Apple's ps, reading what runs on a tty"),
        Exempt(file: "SetupCommand.swift", function: "run", kind: "Process()", program: open,
               why: "Apple's open, opening Smolcorp.app itself (`smolcorp setup`)"),
        Exempt(file: "SelfTest.swift", function: "moveToApplications", kind: "Process()", program: open,
               why: "Apple's open, opening Smolcorp.app itself once it's moved to /Applications"),
        // NSWorkspace hands an app or a link to LaunchServices, which starts the app on its own account, as
        // /usr/bin/open does: never charged to Smolcorp.
        Exempt(file: "SetupCommand.swift", function: "run", kind: "NSWorkspace", program: "",
               why: "the setup page in your browser"),
        Exempt(file: "GoShell.swift", function: "showSettings", kind: "NSWorkspace", program: "",
               why: "Settings in your browser before the window is ready"),
        Exempt(file: "GoShell.swift", function: "openInBrowser", kind: "NSWorkspace", program: "",
               why: "the setup link in your browser"),
        Exempt(file: "SettingsWindow.swift", function: "webView", kind: "NSWorkspace", program: "",
               why: "a link from the setup page, in your browser"),
        Exempt(file: "ScreenSaver.swift", function: "install", kind: "NSWorkspace", program: "",
               why: "System Settings, at Screen Saver"),
        Exempt(file: "System.swift", function: "requestAccessibility", kind: "NSWorkspace", program: "",
               why: "System Settings, at Accessibility"),
        Exempt(file: "T3Code.swift", function: "openApp", kind: "NSWorkspace", program: "",
               why: "T3 Code's app, by its bundle id"),
        Exempt(file: "T3Code.swift", function: "openURL", kind: "NSWorkspace", program: "",
               why: "T3 Code's own t3code:// link"),
    ]

    /// Where the app starts a program through the factory: each must be seen doing so.
    static let wrapped: [(file: String, function: String)] = [
        ("TerminalDrivers.swift", "run"), // Tool.run: tmux, kitten, wezterm (through /usr/bin/env)
        ("OpenCodeInstall.swift", "loginShellPath"), // your login shell, and what its start-up files run
        ("OpenCodeInstall.swift", "major"), // opencode --version
    ]

    /// Each way Swift code starts a program, as a regex on the code.
    static let starts: [(kind: String, pattern: String)] = [
        ("Process()", #"\bProcess\s*\("#),
        ("Process()", #":\s*(?:Foundation\s*\.\s*)?Process\s*=\s*\.\s*init\b"#),
        ("Process.run", #"\bProcess\s*\.\s*(?:run|launchedProcess|init)\b"#),
        ("Process under another name", #"\btypealias\s+\w+\s*=\s*(?:Foundation\s*\.\s*)?(?:Process|NSTask)\b"#),
        ("NSTask", #"\bNSTask\b"#),
        ("posix_spawn", #"\bposix_spawnp?\b"#),
        ("exec, fork, system or popen", #"(?<![.\w])(?:execv|execve|execvp|execvP|execl|execle|execlp|fork|vfork|system|popen)\s*\("#),
        ("NSUserUnixTask", #"\bNSUser(?:Unix|Script|AppleScript|Automator)Task\b"#),
        ("NSWorkspace", #"\bNSWorkspace\s*\.\s*shared\s*\.\s*(?:open|openApplication|openFile|openURLs|launchApplication)\b"#),
        ("NSWorkspace taken as a value", #"\bNSWorkspace\s*\.\s*shared\b(?!\s*\.)"#),
    ]

    struct Start: Equatable {
        let file: String
        let function: String
        let kind: String
        let program: String?
        let line: Int
    }

    /// Every start in `sources`, with the program a Process start sets in the same function, if it does.
    static func startsIn(_ sources: [SwiftSource]) -> [Start] {
        sources.flatMap { source in
            starts.flatMap { start in
                source.matches(start.pattern).map { match -> Start in
                    let function = source.enclosing(match.range.lowerBound)
                    return Start(file: source.name, function: function, kind: start.kind,
                                 program: start.kind == "Process()" ? program(in: source, after: match.range.lowerBound) : nil,
                                 line: source.line(of: match.range.lowerBound))
                }
            }
        }
    }

    /// What `executableURL` (or `launchPath`) is next set to, in the function holding `offset`.
    static func program(in source: SwiftSource, after offset: Int) -> String? {
        let body = source.declarations.filter { $0.body.contains(offset) }.min { $0.body.count < $1.body.count }?.body
        guard let set = source.matches(#"\b(?:executableURL|launchPath)\s*=\s*"#)
            .first(where: { $0.range.lowerBound > offset && body?.contains($0.range.lowerBound) != false }) else { return nil }
        // The expression: to the line's end, a ";", or a "}" or ")" that closes what holds it.
        var depth = 0, end = set.range.upperBound
        scan: while end < source.code.count {
            switch source.code[end] {
            case UInt8(ascii: "("), UInt8(ascii: "["), UInt8(ascii: "{"): depth += 1
            case UInt8(ascii: ")"), UInt8(ascii: "]"), UInt8(ascii: "}"):
                if depth == 0 { break scan }
                depth -= 1
            case 0x0A, UInt8(ascii: ";") where depth == 0: break scan
            default: break
            }
            end += 1
        }
        return source.text(set.range.upperBound..<end).trimmingCharacters(in: .whitespaces)
    }

    /// What's wrong with the starts in `sources`, one line each: a start outside the factory and the exempt list,
    /// an exempt entry or factory site that no longer matches, a factory call given its own place.
    static func problems(in sources: [SwiftSource], exempt: [Exempt] = exempt,
                         wrapped: [(file: String, function: String)] = wrapped) -> [String] {
        let found = startsIn(sources)
        var problems: [String] = []
        for start in found where !exempt.contains(where: { matches($0, start) }) {
            problems.append("\(start.file):\(start.line): \(start.kind) in \(start.function.isEmpty ? "the top level" : start.function) "
                            + "starts a program charged to Smolcorp\(start.program.map { " (\($0))" } ?? ""): "
                            + "make it with Disclaimed.process, or add it to the exempt list with why")
        }
        for entry in exempt where !found.contains(where: { matches(entry, $0) }) {
            problems.append("the exempt list's \(entry.file) \(entry.function) \(entry.kind) \(entry.program) matches nothing: take it off")
        }
        for site in wrapped {
            let made = sources.first { $0.name == site.file }.map { source in
                source.matches(#"\bDisclaimed\s*\.\s*process\s*\("#).contains { source.enclosing($0.range.lowerBound) == site.function }
            } ?? false
            if !made { problems.append("\(site.file) \(site.function) doesn't start its program with Disclaimed.process") }
        }
        // The place is for tests: a call that names one could turn the app's smolcorp-spawn off.
        for source in sources {
            for call in source.matches(#"\bDisclaimed\s*\.\s*process\s*\("#)
            where source.arguments(at: call.range.upperBound - 1).contains(where: { $0.hasPrefix("place:") }) {
                problems.append("\(source.name):\(source.line(of: call.range.lowerBound)): Disclaimed.process is given a place: only tests may")
            }
        }
        return problems
    }

    private static func matches(_ entry: Exempt, _ start: Start) -> Bool {
        entry.file == start.file && entry.function == start.function && entry.kind == start.kind && entry.program == (start.program ?? "")
    }

    // MARK: The app

    func testEveryProgramTheAppStartsHoldsNoneOfSmolcorpsPermissions() throws {
        let sources = try SwiftSource.app()
        XCTAssertGreaterThan(sources.count, 40, "the app's Swift files were read")
        let problems = Self.problems(in: sources)
        XCTAssertEqual(problems, [], problems.joined(separator: "\n"))
        // What the plan counted (§3.5, revision f), as it is now: PhoneDictation's and SummaryCLI's starts left with M7.
        let found = Self.startsIn(sources)
        XCTAssertEqual(found.filter { $0.kind == "Process()" }.count, 9, found.map { "\($0.file) \($0.function)" }.joined(separator: ", "))
        XCTAssertEqual(found.filter { $0.kind == "posix_spawn" }.count, 0)
    }

    /// Terminal's osascript is Apple's: only tests give a stand-in.
    func testTerminalsOsascriptIsApplesAndOnlyTestsSetAnother() throws {
        let sources = try SwiftSource.app()
        let terminal = try XCTUnwrap(sources.first { $0.name == "Terminal.swift" })
        // The regex runs on the code, where the literal is blanked; the group is the source's own text.
        let declared = terminal.matches(#"static var osascript = ([^\n]*)"#)
        XCTAssertEqual(declared.map { $0.groups[0].trimmingCharacters(in: .whitespaces) }, [#""/usr/bin/osascript""#])
        for source in sources {
            for set in source.matches(#"\bosascript\s*=[^=]"#) where !(source.name == "Terminal.swift" && declared.first?.range.overlaps(set.range) == true) {
                XCTFail("\(source.name):\(source.line(of: set.range.lowerBound)): sets Terminal's osascript: only tests may")
            }
        }
    }

    // MARK: The check itself

    private func problems(_ text: String, exempt: [Exempt] = [], wrapped: [(file: String, function: String)] = []) -> [String] {
        Self.problems(in: [SwiftSource(name: "StandIn.swift", text: text)], exempt: exempt, wrapped: wrapped)
    }

    /// Each way to start a program is caught; the factory, an exempt program by its full path, a comment and a
    /// string aren't.
    func testTheCheckCatchesEachWayToStartAProgram() {
        let caught = [
            "func a() { let p = Process(); p.executableURL = URL(fileURLWithPath: \"/bin/sh\"); try? p.run() }",
            "func a() { let p = Foundation.Process(); p.launchPath = \"/bin/sh\"; p.launch() }",
            "func a() { let p: Process = .init(); p.executableURL = URL(fileURLWithPath: \"/bin/sh\") }",
            "func a() { _ = try? Process.run(URL(fileURLWithPath: \"/bin/sh\"), arguments: []) }",
            "func a() { let make = Process.init; _ = make() }",
            "typealias Task = Process",
            "func a() { let t = NSTask() }",
            "func a() { var pid: pid_t = 0; posix_spawn(&pid, \"/bin/sh\", nil, nil, nil, nil) }",
            "func a() { var pid: pid_t = 0; posix_spawnp(&pid, \"sh\", nil, nil, nil, nil) }",
            "func a() { let f = posix_spawn }",
            "func a() { execv(\"/bin/sh\", nil) }",
            "func a() { _ = system(\"sh\") }",
            "func a() { _ = popen(\"sh\", \"r\") }",
            "func a() { if fork() == 0 {} }",
            "func a() { _ = try? NSUserUnixTask(url: URL(fileURLWithPath: \"/bin/sh\")) }",
            "func a() { NSWorkspace.shared.open(URL(fileURLWithPath: \"/tmp/x.command\")) }",
            "func a() { NSWorkspace.shared.openApplication(at: u, configuration: c) }",
            "func a() { let w = NSWorkspace.shared; w.open(u) }",
            "var b: () -> Void = { NSWorkspace.shared.open(u) }",
            // An exempt program, but by its name: whatever "ps" PATH finds first.
            "func commands() { let p = Process(); p.executableURL = URL(fileURLWithPath: \"ps\") }",
            // An exempt program, in another function.
            "func other() { let p = Process(); p.executableURL = URL(fileURLWithPath: \"/bin/ps\") }",
            // Hidden in an interpolation.
            "func a() { print(\"\\(Process())\") }",
        ]
        let exempt = [Exempt(file: "StandIn.swift", function: "commands", kind: "Process()", program: Self.ps, why: "")]
        for text in caught {
            let found = problems(text, exempt: exempt).filter { !$0.contains("matches nothing") }
            XCTAssertEqual(found.count, 1, "\(text) → \(found)")
        }
        let passed = [
            "func a() { guard let p = try? Disclaimed.process(URL(fileURLWithPath: \"/bin/sh\"), []) else { return } }",
            "func commands() { let p = Process(); p.executableURL = URL(fileURLWithPath: \"/bin/ps\") }",
            "func a() { /* Process() */ } // posix_spawn(",
            "func a() { print(\"Process() and posix_spawn\"); _ = #\"NSTask()\"#; _ = \"\"\"\n  Process()\n  \"\"\" }",
            "func a() { _ = ProcessInfo.processInfo; _ = NSWorkspace.shared.frontmostApplication; NSStatusBar.system.thickness }",
            "func a() { if let saved { NSWorkspace.shared.activateFileViewerSelecting([saved]) } }",
        ]
        for text in passed {
            XCTAssertEqual(problems(text, exempt: exempt).filter { !$0.contains("matches nothing") }, [], text)
        }
    }

    func testTheCheckKeepsItsListsTrue() {
        let exempt = [Exempt(file: "StandIn.swift", function: "gone", kind: "Process()", program: Self.ps, why: "")]
        XCTAssertEqual(problems("func a() {}", exempt: exempt).count, 1, "an exempt entry that matches nothing")
        XCTAssertEqual(problems("func a() { }", wrapped: [("StandIn.swift", "a")]).count, 1, "a factory site that no longer uses it")
        XCTAssertEqual(problems("func a() { _ = try? Disclaimed.process(u, []) }", wrapped: [("StandIn.swift", "a")]), [])
        XCTAssertEqual(problems("func a() { _ = try? Disclaimed.process(u, [], place: .init(spawn: u, inApp: false)) }").count, 1,
                       "the app's code never names a place")
    }

    /// The reader: functions, closure properties, and literals with what they interpolate.
    func testTheReaderKnowsFunctionsAndStrings() {
        let source = SwiftSource(name: "S.swift", text: """
            struct S {
                var openURL: (URL) -> Bool = { u in f(u) }
                static func run(_ a: Int, b: () -> Void = {}) -> Int {
                    let s = "x \\(a + g("y)")) z" // )
                    return 1
                }
                func press(_ k: Int) -> Bool
                var pane: URL {
                    here()
                }
            }
            """)
        let code = source.codeText
        XCTAssertEqual(source.enclosing((code as NSString).range(of: "f(u)").location), "openURL")
        XCTAssertEqual(source.enclosing((code as NSString).range(of: "return 1").location), "run")
        XCTAssertEqual(source.enclosing((code as NSString).range(of: "here()").location), "pane")
        XCTAssertFalse(source.declarations.contains { $0.name == "press" }, "a requirement has no body")
        let literal = source.literals.first { !$0.interpolations.isEmpty }
        XCTAssertEqual(literal.map { $0.interpolations.map(source.text) }, [#"a + g("y)")"#])
        XCTAssertFalse(code.contains("x "), "a literal's text is blanked")
    }
}
