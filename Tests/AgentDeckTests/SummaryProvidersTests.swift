import XCTest
@testable import AgentDeck

/// ✨ Summary made by a coding agent: the runs' arguments and environment,
/// their answers, and the run itself, against fake CLIs of the tests' own
/// (shell scripts). No real provider is ever called.
final class SummaryProvidersTests: XCTestCase {
    private var folder: URL!
    private let marker = "PROMPT-MARKER-7f3a"

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
        unsetenv("ANTHROPIC_API_KEY")
    }

    /// A fake CLI: `body` run by bash.
    private func fake(_ name: String, _ body: String) throws -> URL {
        let url = folder.appendingPathComponent(name)
        try ("#!/bin/bash\n" + body + "\n").write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }

    // MARK: The runs' arguments

    func testEachRunIsHeadlessWithNoToolsAndThePromptNeverOnItsCommandLine() {
        for provider in SummaryProvider.agents {
            let call = SummaryCLI.invocation(provider, system: "SYSTEM", prompt: "Project: p\n" + marker, folder: "/tmp/f")
            XCTAssertFalse(call.arguments.contains { $0.contains(marker) }, "\(provider): the excerpt is on the command line")
            let given = String(decoding: call.input, as: UTF8.self) + call.files.values.map { String(decoding: $0, as: UTF8.self) }.joined()
            XCTAssertTrue(given.contains(marker), "\(provider): the excerpt isn't given")
            XCTAssertFalse(call.arguments.contains { ["--yolo", "--force", "--always-approve", "--dangerously-skip-permissions",
                                                      "--approve-mcps", "--auto"].contains($0) }, "\(provider) may run tools")
        }
    }

    func testTheVerifiedFlagsExactly() {
        XCTAssertEqual(SummaryCLI.invocation(.claude, system: "S", prompt: "P", folder: "/f").arguments, [
            "--print", "--model", "haiku", "--tools", "", "--strict-mcp-config", "--setting-sources", "",
            "--disable-slash-commands", "--no-session-persistence", "--permission-mode", "dontAsk",
            "--output-format", "json", "--system-prompt", "S",
        ])
        XCTAssertEqual(SummaryCLI.invocation(.opencode, system: "S", prompt: "P", folder: "/f").arguments,
                       ["run", "--standalone", "--agent", "summary", "--format", "json"])
        let grok = SummaryCLI.invocation(.grok, system: "S", prompt: "P", folder: "/f")
        XCTAssertEqual(Array(grok.arguments.prefix(6)), ["--prompt-file", "/f/prompt.txt", "--cwd", "/f", "--sandbox", "strict"])
        XCTAssertTrue(grok.arguments.contains("--disable-web-search"))
        XCTAssertTrue(grok.arguments[grok.arguments.firstIndex(of: "--disallowed-tools")! + 1].contains("run_terminal_cmd"))
        XCTAssertEqual(SummaryCLI.invocation(.codex, system: "S", prompt: "P", folder: "/f").arguments,
                       ["exec", "--ephemeral", "--skip-git-repo-check", "--sandbox", "read-only", "--ignore-user-config",
                        "--ignore-rules", "--cd", "/f", "--output-last-message", "/f/summary.txt", "-"])
        XCTAssertTrue(SummaryCLI.invocation(.cursor, system: "S", prompt: "P", folder: "/f").arguments.contains("ask"))
        XCTAssertTrue(SummaryCLI.invocation(.gemini, system: "S", prompt: "P", folder: "/f").arguments.contains("plan"))
        XCTAssertEqual(SummaryCLI.flags(.opencode), ["--standalone", "--agent", "--format"])
    }

    // MARK: Its environment, and only its

    func testOnlyTheSummaryRunLosesTheProvidersKeys() {
        let base = ["ANTHROPIC_API_KEY": "sk-ant-test0", "openai_api_key": "x", "XAI_API_KEY": "x", "GEMINI_API_KEY": "x",
                    "PATH": "/usr/bin:/bin", "HOME": "/Users/me", "SMOLCORP_SESSION": "abc"]
        let environment = SummaryCLI.environment(base)
        for name in ["ANTHROPIC_API_KEY", "openai_api_key", "XAI_API_KEY", "GEMINI_API_KEY", "SMOLCORP_SESSION"] {
            XCTAssertNil(environment[name], name)
        }
        XCTAssertEqual(environment["PATH"], "/usr/bin:/bin")
        XCTAssertEqual(environment["HOME"], "/Users/me")
        XCTAssertEqual(environment["SMOLCORP_SUMMARY"], "1")
        XCTAssertEqual(base["ANTHROPIC_API_KEY"], "sk-ant-test0", "the environment given is left as it was")
    }

    /// The owner runs some terminals on an API key and others on a plan: a
    /// hire (through tmux, `Tool.run`, which inherits Smolcorp's environment)
    /// keeps ANTHROPIC_API_KEY, Smolcorp's own environment keeps it, and only
    /// the summary's process goes without.
    func testAHireKeepsANTHROPIC_API_KEYWhileTheSummaryRunHasNone() throws {
        setenv("ANTHROPIC_API_KEY", "sk-ant-hire0", 1)
        let hire = try XCTUnwrap(Tool.run("/usr/bin/env", []))
        XCTAssertTrue(hire.output.contains("ANTHROPIC_API_KEY=sk-ant-hire0"), "a hire's process lost the key")
        XCTAssertFalse(Openers.session("n", in: "/f", shell: "/bin/zsh", running: "claude").joined(separator: " ")
            .contains("ANTHROPIC_API_KEY"), "a hire's tmux session is told to drop the key")
        let summary = SummaryCLI.spawn(URL(fileURLWithPath: "/usr/bin/env"), [], input: Data(),
                                       environment: SummaryCLI.environment(), folder: folder.path, timeout: 10)
        let printed = String(decoding: summary.output, as: UTF8.self)
        XCTAssertFalse(printed.contains("ANTHROPIC_API_KEY"), "the summary's process has the key")
        XCTAssertTrue(printed.contains("SMOLCORP_SUMMARY=1"))
        XCTAssertEqual(ProcessInfo.processInfo.environment["ANTHROPIC_API_KEY"], "sk-ant-hire0", "Smolcorp's own lost it")
    }

    // MARK: Its answer

    func testClaudeCodesAnswer() {
        let ok = Data(#"{"type":"result","subtype":"success","is_error":false,"result":"Asked: dark mode.\nWaiting on you."}"#.utf8)
        XCTAssertEqual(SummaryCLI.answer(.claude, status: 0, timedOut: false, output: ok, error: Data()), .success("Asked: dark mode.\nWaiting on you."))
        let limit = Data(#"{"type":"result","is_error":true,"result":"Claude AI usage limit reached|1793400000"}"#.utf8)
        XCTAssertEqual(SummaryCLI.answer(.claude, status: 1, timedOut: false, output: limit, error: Data()), .failure(.quota))
        let out = Data(#"{"type":"result","is_error":true,"result":"Not logged in · Please run /login"}"#.utf8)
        XCTAssertEqual(SummaryCLI.answer(.claude, status: 1, timedOut: false, output: out, error: Data()), .failure(.signedOut))
    }

    func testGrokNotSignedInAsItSaysIt() {
        // As grok 1.0.70 printed it, with no login on this computer.
        let printed = Data(#"{"type":"error","message":"Not signed in. To authenticate without a browser, run:\n  grok login --device-code"}"#.utf8)
        XCTAssertEqual(SummaryCLI.answer(.grok, status: 1, timedOut: false, output: printed, error: Data()), .failure(.signedOut))
    }

    func testOpenCodesEventsAndItsSession() {
        let printed = """
        {"type":"step_start","sessionID":"ses_abc123","part":{"type":"step-start"}}
        {"type":"text","sessionID":"ses_abc123","part":{"type":"text","text":"Asked: dark mode.\\nWaiting on you."}}
        """
        XCTAssertEqual(SummaryCLI.answer(.opencode, status: 0, timedOut: false, output: Data(printed.utf8), error: Data()),
                       .success("Asked: dark mode.\nWaiting on you."))
        XCTAssertEqual(SummaryCLI.openCodeSession(Data(printed.utf8)), "ses_abc123")
        XCTAssertNil(SummaryCLI.openCodeSession(Data(#"{"sessionID":"ses_1; rm -rf ~"}"#.utf8)))
    }

    func testCodexsAnswerIsItsFile() {
        XCTAssertEqual(SummaryCLI.answer(.codex, status: 0, timedOut: false, output: Data("progress…".utf8), error: Data(),
                                         file: Data("Asked: tests.\n".utf8)), .success("Asked: tests."))
        XCTAssertEqual(SummaryCLI.answer(.codex, status: 2, timedOut: false, output: Data(),
                                         error: Data("error: unexpected argument '--ignore-rules' found".utf8)), .failure(.changed))
    }

    func testGeminisAndCursorsJSON() {
        XCTAssertEqual(SummaryCLI.answer(.gemini, status: 0, timedOut: false,
                                         output: Data(#"{"response":"Asked: x.","stats":{}}"#.utf8), error: Data()), .success("Asked: x."))
        XCTAssertEqual(SummaryCLI.answer(.gemini, status: 1, timedOut: false,
                                         output: Data(#"{"error":{"message":"Quota exceeded for quota metric"}}"#.utf8), error: Data()),
                       .failure(.quota))
        XCTAssertEqual(SummaryCLI.answer(.cursor, status: 0, timedOut: false,
                                         output: Data(#"{"type":"result","result":"Asked: y."}"#.utf8), error: Data()), .success("Asked: y."))
    }

    func testTheSummaryIsCleaned() {
        XCTAssertEqual(SummaryCLI.clean("Here's the summary:\n```\nOne\n\nTwo\n```"), "One\nTwo")
        XCTAssertEqual(SummaryCLI.clean("<thinking>hmm</thinking>A\nB\nC\nD\nE\nF\nG"), "A\nB\nC\nD\nE")
        XCTAssertEqual(SummaryCLI.clean(String(repeating: "x", count: 900)).count, SummaryCLI.charactersMost)
        XCTAssertEqual(SummaryCLI.answer(.claude, status: 0, timedOut: false, output: Data(#"{"result":"  "}"#.utf8), error: Data()),
                       .failure(.empty))
        XCTAssertEqual(SummaryCLI.answer(.claude, status: nil, timedOut: true, output: Data(), error: Data()), .failure(.timeout))
    }

    // MARK: Running a fake CLI

    func testARunGetsTheExcerptOnItsInputInAnEmptyFolderItsOwnDeletedAfter() throws {
        let seen = folder.appendingPathComponent("seen")
        let claude = try fake("claude", """
        if [ "$1" = auth ]; then echo '{"loggedIn":true,"authMethod":"claude.ai","subscriptionType":"max"}'; exit; fi
        { echo "ARGS: $*"; echo "DIR: $PWD"; ls -A; echo "KEY: ${ANTHROPIC_API_KEY:-none}"; echo "INPUT:"; cat; } > \(seen.path)
        echo '{"type":"result","is_error":false,"result":"Asked: dark mode."}'
        """)
        setenv("ANTHROPIC_API_KEY", "sk-ant-run0", 1)
        let result = SummaryCLI.run(.claude, program: claude, system: "SYSTEM", prompt: "Project: p\n" + marker)
        XCTAssertEqual(result, .success("Asked: dark mode."))
        let report = try String(contentsOf: seen, encoding: .utf8)
        XCTAssertTrue(report.contains("INPUT:\nProject: p\n" + marker))
        XCTAssertFalse(report.components(separatedBy: "\n").first!.contains(marker))
        XCTAssertTrue(report.contains("KEY: none"))
        let dir = try XCTUnwrap(report.components(separatedBy: "\n").first { $0.hasPrefix("DIR: ") }).dropFirst(5)
        XCTAssertTrue(dir.contains("smolcorp-summary-"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: String(dir)), "its folder is left behind")
        XCTAssertTrue(report.contains("DIR: \(dir)\nKEY:"), "its folder wasn't empty")
    }

    func testARunThatHangsIsEndedWithAnythingItStarted() throws {
        let pidFile = folder.appendingPathComponent("child")
        let slow = try fake("claude", "sleep 30 & echo $! > \(pidFile.path); wait")
        let started = { (try? String(contentsOf: pidFile, encoding: .utf8)).flatMap { Int32($0.trimmingCharacters(in: .whitespacesAndNewlines)) } }
        // It's stopped only once it has started its child: on a busy Mac, a second may not be enough
        // to get that far, so it's run again with longer.
        for timeout: TimeInterval in [1, 4, 16] where started() == nil {
            let began = Date()
            let ran = SummaryCLI.spawn(slow, [], input: Data(), environment: SummaryCLI.environment(), folder: folder.path, timeout: timeout)
            XCTAssertTrue(ran.timedOut)
            XCTAssertLessThan(Date().timeIntervalSince(began), timeout + 5)
        }
        let child = try XCTUnwrap(started(), "it started its child")
        // Killed, it's gone once its new parent has reaped it: a moment, longer on a busy Mac.
        XCTAssertTrue(eventually { kill(child, 0) != 0 }, "what it started is still running")
    }

    func testARunsOutputIsCut() throws {
        let loud = try fake("claude", "head -c 2000000 /dev/zero | tr '\\0' 'x'; head -c 100000 /dev/zero >&2")
        let ran = SummaryCLI.spawn(loud, [], input: Data(), environment: [:], folder: folder.path, timeout: 10)
        XCTAssertEqual(ran.output.count, SummaryCLI.outputMost)
        XCTAssertEqual(ran.error.count, SummaryCLI.errorMost)
        XCTAssertEqual(ran.status, 0)
    }

    func testARunThatStopsReadingDoesntHurtSmolcorp() throws {
        let deaf = try fake("claude", "exit 3")
        let ran = SummaryCLI.spawn(deaf, [], input: Data(repeating: 65, count: 4_000_000), environment: [:], folder: folder.path, timeout: 10)
        XCTAssertEqual(ran.status, 3)
    }

    // MARK: Which one

    private let ready = SummaryProviderState(kind: .ready, reason: "uses your plan")
    private let out = SummaryProviderState(kind: .signedOut, reason: "installed, not signed in")

    // MARK: Only what can't use tools

    func testOpenCodesSummaryAgentMustDenyEveryTool() {
        // As OpenCode 2.0.32's `debug agents` lists it.
        func agents(_ rules: [(String, String, String)]) -> Data {
            try! JSONSerialization.data(withJSONObject: [
                ["id": "build", "permissions": [["action": "*", "resource": "*", "effect": "allow"]]],
                ["id": "summary", "permissions": rules.map { ["action": $0.0, "resource": $0.1, "effect": $0.2] }],
            ])
        }
        XCTAssertTrue(SummaryCLI.openCodeSummaryAgentIsLocked(agents([("*", "*", "allow"), ("read", "*.env", "ask"),
                                                                      ("*", "*", "deny"), ("browser", "*", "deny")])))
        XCTAssertFalse(SummaryCLI.openCodeSummaryAgentIsLocked(agents([("*", "*", "deny"), ("shell", "*", "allow")])))
        XCTAssertFalse(SummaryCLI.openCodeSummaryAgentIsLocked(agents([("*", "*", "allow")])))
        XCTAssertFalse(SummaryCLI.openCodeSummaryAgentIsLocked(Data(#"[{"id":"build","permissions":[]}]"#.utf8)), "no summary agent")
        XCTAssertFalse(SummaryCLI.openCodeSummaryAgentIsLocked(Data("not json".utf8)))
    }

    func testAnOpenCodeRunThatUsedAToolIsThrownAway() {
        let printed = """
        {"type":"tool_use","sessionID":"ses_a","part":{"type":"tool","tool":"bash"}}
        {"type":"text","sessionID":"ses_a","part":{"type":"text","text":"Asked: x."}}
        """
        XCTAssertEqual(SummaryCLI.answer(.opencode, status: 0, timedOut: false, output: Data(printed.utf8), error: Data()), .failure(.usedTool))
    }

    func testOtherCloudsSettingsDontReachTheRun() {
        let environment = SummaryCLI.environment(["CLAUDE_CODE_USE_BEDROCK": "1", "AWS_BEARER_TOKEN_BEDROCK": "x",
                                                  "ANTHROPIC_DEFAULT_SONNET_MODEL": "x", "ANTHROPIC_DEFAULT_NEW_MODEL": "x",
                                                  "ANTHROPIC_CUSTOM_HEADERS": "x", "GOOGLE_APPLICATION_CREDENTIALS": "x",
                                                  "AZURE_OPENAI_API_KEY": "x", "CLAUDE_CODE_USE_VERTEX": "1", "LANG": "fr_FR.UTF-8"])
        XCTAssertEqual(environment, ["LANG": "fr_FR.UTF-8", "SMOLCORP_SUMMARY": "1"])
    }

    func testItsAnswerIsRedactedLikeTheExcerpt() {
        let printed = Data(#"{"result":"Asked: deploy.\nRan: export API_KEY=sk-live-123\nNot waiting on you."}"#.utf8)
        XCTAssertEqual(SummaryCLI.answer(.claude, status: 0, timedOut: false, output: printed, error: Data()),
                       .success("Asked: deploy.\n[redacted]\nNot waiting on you."))
    }

    func testTheHelpMustListEachFlagExactly() {
        XCTAssertTrue(SummaryCLI.lists("  --tools <tools...>  Specify", "--tools"))
        XCTAssertTrue(SummaryCLI.lists("--format choice", "--format"))
        XCTAssertTrue(SummaryCLI.lists("  -p, --print   Print", "--print"))
        XCTAssertFalse(SummaryCLI.lists("  --allowed-tools <t>", "--tools"))
        XCTAssertFalse(SummaryCLI.lists("  --output-format <f>", "--format"))
        XCTAssertFalse(SummaryCLI.lists("  --toolset <t>", "--tools"))
    }

    func testEachRunChecksAgainThatOpenCodesSummaryAgentDeniesEveryTool() throws {
        let ran = folder.appendingPathComponent("ran")
        // Its summary agent given a tool since setup looked.
        let opencode = try fake("opencode", """
        if [ "$1" = debug ]; then echo '[{"id":"summary","permissions":[{"action":"*","resource":"*","effect":"deny"},{"action":"shell","resource":"*","effect":"allow"}]}]'; exit; fi
        touch \(ran.path)
        """)
        XCTAssertEqual(SummaryCLI.run(.opencode, program: opencode, system: "S", prompt: "P"), .failure(.unsupported))
        XCTAssertFalse(FileManager.default.fileExists(atPath: ran.path), "it ran")
    }

    func testEachRunChecksAgainHowClaudeCodesLoginPays() throws {
        let console = try fake("claude", """
        if [ "$1" = auth ]; then echo '{"loggedIn":true,"authMethod":"console","subscriptionType":null}'; exit; fi
        echo '{"result":"Asked: x."}'
        """)
        // Setup showed a plan: a Console login now is refused, not billed.
        XCTAssertEqual(SummaryCLI.run(.claude, program: console, system: "S", prompt: "P"), .failure(.loginChanged))
        // Setup showed the Console account: it's used, as said.
        XCTAssertEqual(SummaryCLI.run(.claude, program: console, system: "S", prompt: "P", expected: .apiAccount), .success("Asked: x."))
        let out = try fake("claude", """
        if [ "$1" = auth ]; then echo '{"loggedIn":false}'; exit; fi
        echo '{"result":"Asked: x."}'
        """)
        XCTAssertEqual(SummaryCLI.run(.claude, program: out, system: "S", prompt: "P"), .failure(.signedOut))
    }

    func testOpenCodeOutputCutShortOrUnreadableIsThrownAway() {
        let cut = Data(repeating: UInt8(ascii: "x"), count: SummaryCLI.outputMost)
        XCTAssertEqual(SummaryCLI.answer(.opencode, status: 0, timedOut: false, output: cut, error: Data()), .failure(.failed))
        let broken = #"{"type":"tool_use","part":{"type":"to"# + "\n" + #"{"type":"text","part":{"type":"text","text":"Asked: x."}}"#
        XCTAssertEqual(SummaryCLI.answer(.opencode, status: 0, timedOut: false, output: Data(broken.utf8), error: Data()), .failure(.failed))
    }

    func testASecretIsRedactedBeforeTheSummaryIsCut() {
        let text = String(repeating: "a", count: 590) + "\nthe password is hunter2"
        let printed = try! JSONSerialization.data(withJSONObject: ["result": text])
        guard case let .success(kept) = SummaryCLI.answer(.claude, status: 0, timedOut: false, output: printed, error: Data()) else {
            return XCTFail("no summary")
        }
        XCTAssertFalse(kept.contains("passw") || kept.contains("hunter"), kept)
    }

    // MARK: Setup

    func testTheSetupPageKeepsTheKeyHiddenAndWarns() throws {
        let page = try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .appendingPathComponent("../../Resources/web/setup.html"), encoding: .utf8)
        XCTAssertTrue(page.contains(#"<input type="password" id="summary-key" spellcheck="false" autocomplete="new-password""#))
        XCTAssertTrue(page.contains("Use a key from a dedicated workspace with a spending limit."))
        XCTAssertTrue(page.contains(#"href="https://console.anthropic.com/settings/limits""#))
        XCTAssertTrue(page.contains("Only Smolcorp's summary call runs on your plan; your terminals and agents keep their own keys and logins."))
        XCTAssertTrue(page.contains("Your chosen provider isn't supported yet; pick Claude Code, OpenCode or the API key."))
    }
}
