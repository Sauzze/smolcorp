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
        var timeouts = SummaryCLI.Ran()
        let started = Date()
        timeouts = SummaryCLI.spawn(slow, [], input: Data(), environment: SummaryCLI.environment(), folder: folder.path, timeout: 1)
        XCTAssertTrue(timeouts.timedOut)
        XCTAssertLessThan(Date().timeIntervalSince(started), 6)
        let child = try XCTUnwrap(Int32(String(contentsOf: pidFile, encoding: .utf8).trimmingCharacters(in: .whitespacesAndNewlines)))
        usleep(200_000)
        XCTAssertNotEqual(kill(child, 0), 0, "what it started is still running")
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

    func testTheFirstThatWorksIsPickedClaudeCodeFirst() {
        XCTAssertEqual(SummaryChoice.chosen(saved: "", optIn: false, states: [.opencode: ready, .claude: ready], hasKey: true), .claude)
        XCTAssertEqual(SummaryChoice.chosen(saved: "", optIn: false, states: [.claude: out, .opencode: ready], hasKey: false), .opencode)
        XCTAssertEqual(SummaryChoice.chosen(saved: "", optIn: false, states: [.claude: out], hasKey: true), .apiKey)
        XCTAssertNil(SummaryChoice.chosen(saved: "", optIn: false, states: [.claude: out], hasKey: false))
    }

    func testOneWhoUsedTheKeyBeforeKeepsIt() {
        XCTAssertEqual(SummaryChoice.chosen(saved: "", optIn: true, states: [.claude: ready], hasKey: true), .apiKey)
    }

    func testOneSavedIsNeverSwappedForAnother() {
        XCTAssertEqual(SummaryChoice.chosen(saved: "opencode", optIn: true, states: [.claude: ready, .opencode: out], hasKey: true), .opencode)
        XCTAssertFalse(SummaryChoice.works(.opencode, states: [.opencode: out], hasKey: true))
        XCTAssertNil(SummaryChoice.chosen(saved: "apikey", optIn: true, states: [.claude: ready], hasKey: false))
    }

    func testGroksAgentIsNeverTakenForCursor() throws {
        let path = ProcessInfo.processInfo.environment["PATH"] ?? ""
        defer { setenv("PATH", path, 1) }
        setenv("PATH", folder.path, 1)
        let grok = try fake("agent", "echo 'grok 1.0.70 (f50904eaee6c)'")
        XCTAssertNotEqual(SummaryProviders.program(.cursor)?.path, grok.resolvingSymlinksInPath().path)
        try FileManager.default.removeItem(at: grok)
        let cursor = try fake("agent", "echo '2025.10.28-0a1b2c3'")
        XCTAssertEqual(SummaryProviders.program(.cursor)?.path, cursor.resolvingSymlinksInPath().path)
    }

    // MARK: Made with the key when it fails, and saying so

    private func transcript() throws -> String {
        let path = folder.appendingPathComponent("t.jsonl")
        try #"{"type":"user","message":{"role":"user","content":"add a dark mode toggle please"}}"#.write(to: path, atomically: true, encoding: .utf8)
        return path.path
    }

    private func wait(_ summaries: Summaries, _ path: String, _ provider: SummaryProvider) -> Result<String, SummaryError> {
        let done = expectation(description: "summary")
        var got: Result<String, SummaryError>!
        summaries.summarize("s", transcript: path, project: "p", provider: provider) { got = $0; done.fulfill() }
        wait(for: [done], timeout: 5)
        return got
    }

    private let anthropic: Summaries.Transport = { _, done in
        done(Data(#"{"content":[{"type":"text","text":"From the key."}]}"#.utf8), 200, nil)
    }

    func testAFailedAgentFallsBackToTheKeyAndSaysSo() throws {
        let summaries = Summaries(transport: anthropic, key: { "sk-ant-test" }, savedKey: { "sk-ant-test" },
                                  agent: { _, _, _ in .failure(.signedOut) })
        XCTAssertEqual(wait(summaries, try transcript(), .claude),
                       .success("From the key.\n(Claude Code isn't signed in, so this was made with your API key.)"))
        XCTAssertEqual(summaries.lastFallback, "(Claude Code isn't signed in, so this was made with your API key.)")
    }

    func testAFailedAgentWithNoKeySaysWhy() throws {
        var sent = false
        let summaries = Summaries(transport: { _, _ in sent = true }, key: { nil }, savedKey: { nil }, agent: { _, _, _ in .failure(.quota) })
        XCTAssertEqual(wait(summaries, try transcript(), .codex), .failure(SummaryCLIFailure.quota.error(.codex)))
        XCTAssertFalse(sent)
    }

    func testAnAgentThatWorksSendsNothingToAnthropic() throws {
        var sent = false, given = ""
        let summaries = Summaries(transport: { _, _ in sent = true }, key: { "sk-ant-test" }, savedKey: { "sk-ant-test" },
                                  agent: { _, system, prompt in given = system + prompt; return .success("From the agent.") })
        XCTAssertEqual(wait(summaries, try transcript(), .opencode), .success("From the agent."))
        XCTAssertFalse(sent)
        XCTAssertTrue(given.contains("LATEST REQUEST"), "the agent gets today's excerpt")
        XCTAssertNil(summaries.lastFallback)
    }

    func testOnlyAKeyPastedInSetupIsAFallbackNeverTheEnvironments() throws {
        var sent = false
        // ANTHROPIC_API_KEY in Smolcorp's environment (key), nothing pasted (savedKey).
        let summaries = Summaries(transport: { _, _ in sent = true }, key: { "sk-ant-fromenv" }, savedKey: { nil },
                                  agent: { _, _, _ in .failure(.signedOut) })
        XCTAssertEqual(wait(summaries, try transcript(), .claude), .failure(SummaryCLIFailure.signedOut.error(.claude)))
        XCTAssertFalse(sent)
    }

    func testTheFallbackKeepsWithinTheLimit() throws {
        var sent = 0
        let summaries = Summaries(transport: { _, done in sent += 1; self.anthropic(URLRequest(url: URL(string: "http://x")!), done) },
                                  savedKey: { "sk-ant-test" }, agent: { _, _, _ in .failure(.quota) })
        let path = try transcript()
        // Each tap: the agent's call and the key's, until the window is full.
        for i in 0..<Summaries.most {
            let done = expectation(description: "\(i)")
            summaries.summarize("s\(i)", transcript: path, project: "p", provider: .claude) { _ in done.fulfill() }
            wait(for: [done], timeout: 5)
        }
        XCTAssertEqual(sent, Summaries.most / 2, "the fallback went past the limit")
    }

    // MARK: Only what can't use tools

    func testOnlyClaudeCodeAndOpenCodeAreRun() {
        XCTAssertEqual(SummaryProvider.agents.filter(\.isSupported), [.claude, .opencode])
        for provider in [SummaryProvider.codex, .gemini, .cursor, .grok] {
            let state = SummaryProviders.shared.check(provider)
            XCTAssertEqual(state.kind, .unsupported, "\(provider)")
            XCTAssertEqual(state.reason, "not supported yet: can't turn its tools off safely")
            XCTAssertNil(SummaryChoice.chosen(saved: provider.rawValue, optIn: true, states: [provider: ready], hasKey: false))
            XCTAssertFalse(SummaryChoice.works(provider, states: [provider: ready], hasKey: false))
            XCTAssertEqual(SummaryCLI.run(provider, program: URL(fileURLWithPath: "/usr/bin/true"), system: "", prompt: ""), .failure(.unsupported))
        }
        XCTAssertEqual(SummaryChoice.chosen(saved: "", optIn: false, states: [.grok: ready, .codex: ready], hasKey: false), nil)
    }

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

    func testAConsoleLoginSaysItBillsTheAPIAccount() {
        XCTAssertEqual(SummaryCLI.claudeLogin(Data(#"{"loggedIn":true,"authMethod":"claude.ai","subscriptionType":"max"}"#.utf8)), .ready)
        XCTAssertEqual(SummaryCLI.claudeLogin(Data(#"{"loggedIn":true,"authMethod":"console","subscriptionType":null}"#.utf8)), .apiAccount)
        XCTAssertEqual(SummaryCLI.claudeLogin(Data(#"{"loggedIn":true,"authMethod":"claude.ai","subscriptionType":""}"#.utf8)), .apiAccount)
        XCTAssertNil(SummaryCLI.claudeLogin(Data(#"{"loggedIn":false}"#.utf8)))
        let console = SummaryProviderState(kind: .apiAccount, reason: "signed in with a Console account: bills your API account")
        XCTAssertTrue(SummaryChoice.works(.claude, states: [.claude: console], hasKey: false))
        // Never picked by itself, as it isn't a plan: only when chosen, and then it says what it bills.
        XCTAssertNil(SummaryChoice.chosen(saved: "", optIn: false, states: [.claude: console], hasKey: false))
        let state = DeckRouter.summaryState(optIn: false, saved: "claude", environment: [:], states: [.claude: console])
        XCTAssertEqual((state["provider"] as? [String: Any])?["plan"] as? String, "signed in with a Console account: bills your API account")
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

    func testAnUnsupportedPickSaysSo() {
        let state = DeckRouter.summaryState(optIn: true, saved: "gemini", environment: [:],
                                            states: [.gemini: SummaryProviderState(kind: .unsupported, reason: SummaryProvider.unsupportedReason)])
        XCTAssertEqual(state["saved"] as? String, "gemini")
        XCTAssertTrue(state["provider"] is NSNull)
        XCTAssertEqual(state["enabled"] as? Bool, false)
    }

    // MARK: Setup

    func testSetupSaysWhyEachCantAndWhatTheChosenCosts() {
        SummaryKey.store = .init(read: { nil }, hint: { nil }, write: { _, _ in true }, delete: {})
        SummaryKey.forgetKnown()
        defer { SummaryKey.store = .keychain; SummaryKey.forgetKnown() }
        let state = DeckRouter.summaryState(optIn: false, saved: "", environment: [:],
                                            states: [.claude: ready, .grok: out, .codex: .missing(.codex)])
        let providers = state["providers"] as! [[String: Any]]
        XCTAssertEqual(providers.map { $0["id"] as! String }, ["claude", "codex", "gemini", "grok", "cursor", "opencode", "apikey"])
        XCTAssertEqual(providers[1]["reason"] as? String, "not installed")
        XCTAssertEqual(providers[2]["state"] as? String, "checking")
        XCTAssertEqual((state["provider"] as? [String: Any])?["id"] as? String, "claude")
        XCTAssertEqual(state["enabled"] as? Bool, false)
    }

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
