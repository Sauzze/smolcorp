import Foundation

/// ✨ Summary made by a coding agent you already have, on your own plan,
/// instead of with an API key: Claude Code, Codex, Gemini CLI, Grok Build,
/// Cursor or OpenCode, run once per summary, headless and with no tools.
///
/// Each is run in an empty folder of its own, made for the run and deleted
/// after it, with the same redacted excerpt and prompt the API key gets, on
/// its standard input (never its command line, which `ps` shows), for at most
/// `timeout` and `outputMost` bytes. Its environment is Smolcorp's, less the
/// providers' API keys, so it can only use the plan you signed in with, and
/// with SMOLCORP_SUMMARY=1, which Smolcorp's own hook ignores, so the run
/// never gets a desk. That one process is all this changes: your shell, your
/// terminals, the agents Smolcorp hires and Smolcorp itself keep theirs.

// MARK: The providers

enum SummaryProvider: String, CaseIterable {
    case claude, codex, gemini, grok, cursor, opencode
    case apiKey = "apikey"

    /// The coding agents, in the order the first that works is picked.
    static let agents: [SummaryProvider] = [.claude, .codex, .gemini, .grok, .cursor, .opencode]

    var name: String {
        switch self {
        case .claude: return "Claude Code"
        case .codex: return "Codex"
        case .gemini: return "Gemini CLI"
        case .grok: return "Grok Build"
        case .cursor: return "Cursor"
        case .opencode: return "OpenCode"
        case .apiKey: return "API key"
        }
    }

    /// What it costs, in setup's words.
    var plan: String {
        switch self {
        case .claude: return "uses your Claude plan"
        case .codex: return "uses your ChatGPT plan"
        case .gemini: return "uses your Gemini plan"
        case .grok: return "uses your Grok plan"
        case .cursor: return "uses your Cursor plan"
        case .opencode: return "uses your OpenCode setup (its default model)"
        case .apiKey: return "Anthropic, billed to that key"
        }
    }

    /// Where the summary input goes, for setup's privacy line.
    var destination: String {
        switch self {
        case .apiKey: return "Anthropic's API, with your key"
        default: return "\(name) under your own account"
        }
    }

    /// The program's names, the first found wins.
    var programs: [String] {
        switch self {
        case .claude: return ["claude"]
        case .codex: return ["codex"]
        case .gemini: return ["gemini"]
        case .grok: return ["grok"]
        // Cursor's own installer names it `agent` now, and `cursor-agent` before. Grok Build installs an `agent` too.
        case .cursor: return ["cursor-agent", "agent"]
        case .opencode: return ["opencode"]
        case .apiKey: return []
        }
    }

    /// How to sign in, for setup.
    var signIn: String {
        switch self {
        case .claude: return "run `claude` and log in"
        case .codex: return "run `codex login`"
        case .gemini: return "run `gemini` once and sign in with Google"
        case .grok: return "run `grok login`"
        case .cursor: return "run `cursor-agent login`"
        case .opencode: return "run `opencode auth login`"
        case .apiKey: return ""
        }
    }

    /// Whether every tool it has can be turned off, verified: only these are
    /// run. The others' own docs say a headless run keeps some (Gemini's plan
    /// mode switches to YOLO and keeps web search, Codex's read-only sandbox
    /// still runs a shell that reads your home, Cursor's -p has every tool,
    /// Grok's is a deny list with MCP on and no network block on macOS), and
    /// an excerpt can carry text an agent fetched from a page.
    var isSupported: Bool { self == .claude || self == .opencode || self == .apiKey }

    static let unsupportedReason = "not supported yet: can't turn its tools off safely"

    /// The flags that ask for its help, whose text must still list every flag a summary passes.
    var helpArguments: [String] {
        switch self {
        case .codex: return ["exec", "--help"]
        case .opencode: return ["run", "--help"]
        default: return ["--help"]
        }
    }
}

// MARK: One run

/// Why a coding agent made no summary.
enum SummaryCLIFailure: Error, Equatable {
    case missing, signedOut, quota, timeout, changed, empty, failed, unsupported, usedTool
    /// Its login now pays another way than setup showed (a plan, or a Console account).
    case loginChanged

    func error(_ provider: SummaryProvider) -> SummaryError {
        let name = provider.name
        switch self {
        case .missing: return SummaryError(message: "\(name) isn't on this computer any more. Pick another in setup.", status: 503)
        case .signedOut: return SummaryError(message: "\(name) isn't signed in. Sign in on your computer: \(provider.signIn).", status: 502)
        case .quota: return SummaryError(message: "Your \(name) plan's limit is reached. Try again later.", status: 429)
        case .timeout: return SummaryError(message: "\(name) didn't answer in time. Try again.", status: 504)
        case .changed: return SummaryError(message: "\(name) has changed: summaries with it need an update of Smolcorp.", status: 502)
        case .empty: return SummaryError(message: "\(name) sent back nothing.", status: 502)
        case .unsupported: return SummaryError(message: "\(name) isn't supported yet for summaries: pick another in setup.", status: 503)
        case .usedTool: return SummaryError(message: "\(name) tried to use a tool, so its answer was thrown away.", status: 502)
        case .loginChanged:
            return SummaryError(message: "\(name)'s login has changed since setup showed it: check it in setup, on your computer.", status: 409)
        case .failed: return SummaryError(message: "\(name) couldn't make a summary.", status: 502)
        }
    }

    /// A few words for a summary made with the key instead.
    func short(_ provider: SummaryProvider) -> String {
        let name = provider.name
        switch self {
        case .missing: return "\(name) wasn't found"
        case .signedOut: return "\(name) isn't signed in"
        case .quota: return "your \(name) plan's limit is reached"
        case .timeout: return "\(name) didn't answer in time"
        case .changed: return "\(name) has changed"
        case .unsupported: return "\(name) isn't supported yet"
        case .usedTool: return "\(name) tried to use a tool"
        case .loginChanged: return "\(name)'s login has changed"
        case .empty, .failed: return "\(name) failed"
        }
    }
}

/// What a run needs: the arguments, what goes on its standard input, files
/// written in its folder first, and where its answer is.
struct SummaryInvocation {
    var arguments: [String]
    var input: Data
    var files: [String: Data] = [:]
    /// A file in the folder holding the answer, instead of standard output.
    var outputFile: String?
}

enum SummaryCLI {
    static let timeout: TimeInterval = 45
    /// OpenCode starts a server of its own and may run a model on this computer.
    static let openCodeTimeout: TimeInterval = 90
    static let outputMost = 64 * 1024
    static let errorMost = 8 * 1024
    /// A summary's length, at most, as kept.
    static let linesMost = 5
    static let charactersMost = 600

    /// Variables a summary's process never gets: with them, a CLI would bill
    /// that key rather than use your plan, or talk to somewhere else. Only that
    /// process: nothing else Smolcorp starts loses them.
    static let removed: Set<String> = [
        "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL",
        "OPENAI_API_KEY", "CODEX_API_KEY", "OPENAI_BASE_URL",
        "GEMINI_API_KEY", "GOOGLE_API_KEY", "XAI_API_KEY", "CURSOR_API_KEY",
        // Or another provider's cloud, billed elsewhere.
        "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY", "AWS_BEARER_TOKEN_BEDROCK",
        "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL",
        "ANTHROPIC_CUSTOM_HEADERS", "GOOGLE_GENAI_USE_VERTEXAI", "GOOGLE_CLOUD_PROJECT", "GOOGLE_APPLICATION_CREDENTIALS",
        "AZURE_OPENAI_API_KEY",
        // A run's hook would otherwise take it for a session of a desk's, or a hire.
        "SMOLCORP_SESSION", "SAUZZEDECK_SESSION", "SMOLCORP_HEADLESS", "SMOLCORP_PARENT_FOLDER",
        "TMUX", "TMUX_PANE", "WEZTERM_PANE", "WEZTERM_UNIX_SOCKET", "KITTY_WINDOW_ID", "KITTY_LISTEN_ON",
    ]

    /// The environment of a summary's process, from Smolcorp's.
    static func environment(_ base: [String: String] = ProcessInfo.processInfo.environment) -> [String: String] {
        var environment = base.filter { !removed.contains($0.key.uppercased()) && !$0.key.uppercased().hasPrefix("ANTHROPIC_DEFAULT_") }
        environment["SMOLCORP_SUMMARY"] = "1"
        return environment
    }

    static func timeout(of provider: SummaryProvider) -> TimeInterval {
        provider == .opencode ? openCodeTimeout : timeout
    }

    /// The run of `provider` for a summary of `extract`, in `folder`.
    static func invocation(_ provider: SummaryProvider, system: String, prompt: String, folder: String) -> SummaryInvocation {
        // For those with no flag for the instructions, they come first on its input.
        let both = Data((system + "\n\n" + prompt).utf8)
        switch provider {
        case .claude:
            // --setting-sources "" also leaves out your hooks, --tools "" every tool, and --bare can't be used:
            // it ignores the login, which is the plan.
            return SummaryInvocation(arguments: [
                "--print", "--model", "haiku", "--tools", "", "--strict-mcp-config", "--setting-sources", "",
                "--disable-slash-commands", "--no-session-persistence", "--permission-mode", "dontAsk",
                "--output-format", "json", "--system-prompt", system,
            ], input: Data(prompt.utf8))
        case .codex:
            // Read-only is exec's own sandbox; --ignore-user-config leaves out your config.toml's MCP servers.
            return SummaryInvocation(arguments: [
                "exec", "--ephemeral", "--skip-git-repo-check", "--sandbox", "read-only", "--ignore-user-config",
                "--ignore-rules", "--cd", folder, "--output-last-message", folder + "/summary.txt", "-",
            ], input: both, outputFile: "summary.txt")
        case .gemini:
            // Plan mode is read-only; no extension and no MCP server ("none" names none of them).
            return SummaryInvocation(arguments: [
                "--prompt", "Write the summary, as the instructions above say.", "--approval-mode", "plan",
                "--extensions", "none", "--allowed-mcp-server-names", "none", "--output-format", "json",
            ], input: both)
        case .grok:
            return SummaryInvocation(arguments: [
                "--prompt-file", folder + "/prompt.txt", "--cwd", folder, "--sandbox", "strict",
                "--disallowed-tools", "run_terminal_cmd,write,search_replace,read_file,grep,list_dir,web_search,web_fetch,task,Agent",
                "--disable-web-search", "--no-subagents", "--max-turns", "1",
                "--system-prompt-override", system, "--output-format", "json",
            ], input: Data(), files: ["prompt.txt": Data(prompt.utf8)])
        case .cursor:
            // Ask mode is read-only; without --force or --approve-mcps, nothing is written and no MCP server runs.
            return SummaryInvocation(arguments: [
                "--print", "--mode", "ask", "--sandbox", "enabled", "--trust", "--workspace", folder,
                "--output-format", "json",
            ], input: both)
        case .opencode:
            // OpenCode 2 ignores a config given for one run, so its own summary agent, which every tool is denied.
            return SummaryInvocation(arguments: ["run", "--standalone", "--agent", "summary", "--format", "json"], input: both)
        case .apiKey:
            return SummaryInvocation(arguments: [], input: Data())
        }
    }

    /// The long flags a run passes, which its help must still list.
    static func flags(_ provider: SummaryProvider) -> [String] {
        invocation(provider, system: "", prompt: "", folder: "/f").arguments.filter { $0.hasPrefix("--") }
    }

    // MARK: Its answer

    /// The summary from what the run printed, or why there's none.
    static func answer(_ provider: SummaryProvider, status: Int32?, timedOut: Bool, output: Data, error: Data,
                       file: Data? = nil) -> Result<String, SummaryCLIFailure> {
        if timedOut { return .failure(.timeout) }
        let printed = String(decoding: output, as: UTF8.self)
        let complaint = String(decoding: error, as: UTF8.self)
        var text: String?
        var failure: String?
        switch provider {
        case .codex:
            text = file.map { String(decoding: $0, as: UTF8.self) }
        case .opencode:
            // A tool it ran, though its summary agent is denied every one: nothing it said is kept.
            if openCodeUsedTool(printed) { return .failure(.usedTool) }
            // Cut short or unreadable, it can't be told whether a tool ran: nothing is kept either.
            if output.count >= outputMost || openCodeUnreadable(printed) { return .failure(.failed) }
            (text, failure) = openCodeText(printed)
        default:
            (text, failure) = jsonText(printed)
        }
        if status != 0 || failure != nil {
            return .failure(classify((failure ?? "") + "\n" + complaint + "\n" + (text ?? "")))
        }
        // Its answer is redacted as the excerpt was, before it's shown or kept.
        let cleaned = clean(Redact.text(text ?? ""))
        return cleaned.isEmpty ? .failure(.empty) : .success(cleaned)
    }

    /// The text of a JSON answer (Claude Code, Grok, Gemini, Cursor): one
    /// object, or lines of them, the last with text winning. Also an error's
    /// message, when it says it's one.
    static func jsonText(_ printed: String) -> (text: String?, failure: String?) {
        var objects: [[String: Any]] = []
        if let object = parse(printed) {
            objects = [object]
        } else {
            objects = printed.split(whereSeparator: \.isNewline).compactMap { parse(String($0)) }
        }
        var text: String?, failure: String?
        for object in objects {
            if object["type"] as? String == "error" || object["is_error"] as? Bool == true {
                failure = string(object["message"]) ?? string(object["result"]) ?? string(object["error"]) ?? "error"
                continue
            }
            if let error = object["error"], !(error is NSNull) {
                failure = string((error as? [String: Any])?["message"]) ?? string(error) ?? "error"
            }
            if let found = string(object["result"]) ?? string(object["response"]) ?? string(object["text"]) {
                text = found
            }
        }
        // Not JSON at all: taken as it is, as a CLI that changed its output might print plain text.
        if objects.isEmpty, !printed.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { text = printed }
        return (text, failure)
    }

    /// OpenCode's answer: lines of events, its text in "text" events' parts.
    static func openCodeText(_ printed: String) -> (text: String?, failure: String?) {
        var parts: [String] = [], failure: String?
        for line in printed.split(whereSeparator: \.isNewline) {
            guard let event = parse(String(line)) else { continue }
            switch event["type"] as? String {
            case "text": if let text = string((event["part"] as? [String: Any])?["text"]) { parts.append(text) }
            case "error": failure = string((event["error"] as? [String: Any])?["message"]) ?? string(event["message"]) ?? "error"
            default: break
            }
        }
        return (parts.isEmpty ? nil : parts.joined(separator: "\n"), failure)
    }

    /// Whether OpenCode's events show a tool being run.
    static func openCodeUsedTool(_ printed: String) -> Bool {
        printed.split(whereSeparator: \.isNewline).contains { line in
            guard let event = parse(String(line)) else { return false }
            let kinds = [event["type"] as? String, (event["part"] as? [String: Any])?["type"] as? String].compactMap { $0 }
            return kinds.contains { $0.lowercased().contains("tool") }
        }
    }

    /// Whether a line of OpenCode's events starts as JSON and doesn't read as it.
    static func openCodeUnreadable(_ printed: String) -> Bool {
        printed.split(whereSeparator: \.isNewline).contains { line in
            line.trimmingCharacters(in: .whitespaces).hasPrefix("{") && parse(String(line)) == nil
        }
    }

    /// Whether OpenCode's agents, as `opencode debug agents` lists them, have
    /// a summary agent denied every tool: a rule denying everything, and no
    /// rule after it allowing or asking anything. Without one, `--agent
    /// summary` could quietly run another agent, with tools.
    static func openCodeSummaryAgentIsLocked(_ listed: Data) -> Bool {
        guard let agents = (try? JSONSerialization.jsonObject(with: listed)) as? [[String: Any]],
              let summary = agents.first(where: { $0["id"] as? String == "summary" }),
              let rules = summary["permissions"] as? [[String: Any]],
              let all = rules.lastIndex(where: { $0["action"] as? String == "*" && $0["resource"] as? String == "*" && $0["effect"] as? String == "deny" })
        else { return false }
        return rules[(all + 1)...].allSatisfy { $0["effect"] as? String == "deny" }
    }

    /// What a Claude Code login pays with, from `claude auth status --json`:
    /// a plan, an API account, or nil when not signed in.
    static func claudeLogin(_ status: Data) -> SummaryProviderState.Kind? {
        guard let object = (try? JSONSerialization.jsonObject(with: status)) as? [String: Any],
              object["loggedIn"] as? Bool == true else { return nil }
        let plan = (object["subscriptionType"] as? String).map { !$0.trimmingCharacters(in: .whitespaces).isEmpty } ?? false
        return plan && object["authMethod"] as? String == "claude.ai" ? .ready : .apiAccount
    }

    /// Whether `help` lists `flag` as a flag of its own, not inside another's name.
    static func lists(_ help: String, _ flag: String) -> Bool {
        let pattern = #"(^|[\s,\[|(])"# + NSRegularExpression.escapedPattern(for: flag) + #"($|[\s,=\]|)<])"#
        return help.range(of: pattern, options: .regularExpression) != nil
    }

    /// The session OpenCode kept for the run, to delete after it.
    static func openCodeSession(_ printed: Data) -> String? {
        for line in String(decoding: printed, as: UTF8.self).split(whereSeparator: \.isNewline) {
            if let id = parse(String(line))?["sessionID"] as? String,
               id.range(of: #"^ses_[A-Za-z0-9]+$"#, options: .regularExpression) != nil { return id }
        }
        return nil
    }

    private static func parse(_ text: String) -> [String: Any]? {
        guard let data = text.trimmingCharacters(in: .whitespacesAndNewlines).data(using: .utf8), data.first == UInt8(ascii: "{") else { return nil }
        return (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
    }

    private static func string(_ value: Any?) -> String? {
        guard let text = value as? String, !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return nil }
        return text
    }

    /// What a failed run's words say went wrong. They're only matched, never shown.
    static func classify(_ words: String) -> SummaryCLIFailure {
        let words = words.lowercased()
        func any(_ phrases: [String]) -> Bool { phrases.contains { words.contains($0) } }
        if any(["unknown option", "unexpected argument", "unrecognized", "unknown flag", "invalid option", "unknown argument"]) { return .changed }
        if any(["usage limit", "rate limit", "rate_limit", "quota", "limit reached", "too many requests", "429", "resource_exhausted"]) { return .quota }
        if any(["not signed in", "not logged in", "please log in", "please login", "login required", "sign in", "authenticate",
                "unauthorized", "401", "invalid api key", "no credentials", "/login"]) { return .signedOut }
        return .failed
    }

    /// The summary as kept: no code fences, no "Here's…" line, no thinking,
    /// at most `linesMost` lines and `charactersMost` characters.
    static func clean(_ text: String) -> String {
        var text = text
        while let open = text.range(of: "<thinking>"), let close = text.range(of: "</thinking>", range: open.upperBound..<text.endIndex) {
            text.removeSubrange(open.lowerBound..<close.upperBound)
        }
        var lines = text.split(whereSeparator: \.isNewline).map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty && !$0.hasPrefix("```") }
        if let first = lines.first?.lowercased(), first.hasSuffix(":"),
           ["summary", "here", "sure", "voici", "résumé"].contains(where: first.hasPrefix) {
            lines.removeFirst()
        }
        let kept = lines.prefix(linesMost).joined(separator: "\n")
        return kept.count <= charactersMost ? kept : String(kept.prefix(charactersMost - 1)) + "…"
    }

    // MARK: Running it

    struct Ran {
        var status: Int32?
        var timedOut = false
        var output = Data()
        var error = Data()
        var file: Data?
    }

    /// Runs `provider` for a summary, in a folder made for it and deleted after.
    /// Its login and its summary agent are checked again just before: either
    /// may have changed since setup looked. `expected` is how setup showed a
    /// Claude Code login paying.
    static func run(_ provider: SummaryProvider, program: URL, system: String, prompt: String,
                    expected: SummaryProviderState.Kind = .ready,
                    base: [String: String] = ProcessInfo.processInfo.environment) -> Result<String, SummaryCLIFailure> {
        let folder = FileManager.default.temporaryDirectory
            .appendingPathComponent("smolcorp-summary-" + UUID().uuidString, isDirectory: true)
        guard (try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: false,
                                                        attributes: [.posixPermissions: 0o700])) != nil else { return .failure(.failed) }
        defer { try? FileManager.default.removeItem(at: folder) }
        guard provider.isSupported else { return .failure(.unsupported) }
        let call = invocation(provider, system: system, prompt: prompt, folder: folder.path)
        for (name, data) in call.files {
            guard Paths.writePrivately(data, to: folder.appendingPathComponent(name)) else { return .failure(.failed) }
        }
        let environment = self.environment(base)
        switch provider {
        case .claude:
            let login = claudeLogin(spawn(program, ["auth", "status", "--json"], input: Data(), environment: environment,
                                          folder: folder.path, timeout: 15).output)
            guard let login else { return .failure(.signedOut) }
            guard login == expected else { return .failure(.loginChanged) }
        case .opencode:
            let agents = spawn(program, ["debug", "agents"], input: Data(), environment: environment, folder: folder.path, timeout: 15)
            guard openCodeSummaryAgentIsLocked(agents.output) else { return .failure(.unsupported) }
        default:
            break
        }
        var ran = spawn(program, call.arguments, input: call.input, environment: environment, folder: folder.path,
                        timeout: timeout(of: provider))
        if let file = call.outputFile { ran.file = try? Data(contentsOf: folder.appendingPathComponent(file)) }
        if provider == .opencode, let session = openCodeSession(ran.output) {
            _ = spawn(program, ["session", "delete", session], input: Data(), environment: environment, folder: folder.path, timeout: 15)
        }
        return answer(provider, status: ran.status, timedOut: ran.timedOut, output: ran.output, error: ran.error, file: ran.file)
    }

    /// Starts `program` in a process group of its own, so a timeout ends it
    /// and anything it started; gives it `input` and reads at most
    /// `outputMost` of its output and `errorMost` of its errors, the rest
    /// dropped.
    static func spawn(_ program: URL, _ arguments: [String], input: Data, environment: [String: String], folder: String,
                      timeout: TimeInterval) -> Ran {
        var ran = Ran()
        var inPipe: [Int32] = [0, 0], outPipe: [Int32] = [0, 0], errPipe: [Int32] = [0, 0]
        guard pipe(&inPipe) == 0 else { return ran }
        guard pipe(&outPipe) == 0 else { close(inPipe[0]); close(inPipe[1]); return ran }
        guard pipe(&errPipe) == 0 else { [inPipe, outPipe].forEach { close($0[0]); close($0[1]) }; return ran }
        var actions: posix_spawn_file_actions_t?
        posix_spawn_file_actions_init(&actions)
        defer { posix_spawn_file_actions_destroy(&actions) }
        posix_spawn_file_actions_adddup2(&actions, inPipe[0], 0)
        posix_spawn_file_actions_adddup2(&actions, outPipe[1], 1)
        posix_spawn_file_actions_adddup2(&actions, errPipe[1], 2)
        posix_spawn_file_actions_addchdir_np(&actions, folder)
        var attributes: posix_spawnattr_t?
        posix_spawnattr_init(&attributes)
        defer { posix_spawnattr_destroy(&attributes) }
        // Its own group, and none of Smolcorp's other open files.
        posix_spawnattr_setflags(&attributes, Int16(POSIX_SPAWN_SETPGROUP | POSIX_SPAWN_CLOEXEC_DEFAULT))
        posix_spawnattr_setpgroup(&attributes, 0)
        let argv = ([program.path] + arguments).map { strdup($0) } + [nil]
        let envp = environment.map { strdup("\($0.key)=\($0.value)") } + [nil]
        defer { (argv + envp).forEach { free($0) } }
        var pid: pid_t = 0
        let started = posix_spawn(&pid, program.path, &actions, &attributes, argv, envp)
        close(inPipe[0]); close(outPipe[1]); close(errPipe[1])
        guard started == 0 else {
            close(inPipe[1]); close(outPipe[0]); close(errPipe[0])
            return ran
        }
        let group = DispatchGroup()
        let lock = NSLock()
        // Its input, then closed: a CLI that stops reading early mustn't raise SIGPIPE in Smolcorp.
        _ = fcntl(inPipe[1], F_SETNOSIGPIPE, 1)
        DispatchQueue.global().async(group: group) {
            input.withUnsafeBytes { bytes in
                var sent = 0
                while sent < bytes.count {
                    let n = write(inPipe[1], bytes.baseAddress! + sent, bytes.count - sent)
                    if n <= 0 { break }
                    sent += n
                }
            }
            close(inPipe[1])
        }
        func drain(_ fd: Int32, most: Int, into keep: @escaping (Data) -> Void) {
            DispatchQueue.global().async(group: group) {
                var kept = Data(), buffer = [UInt8](repeating: 0, count: 16 * 1024)
                while true {
                    let n = read(fd, &buffer, buffer.count)
                    if n <= 0 { break }
                    if kept.count < most { kept.append(contentsOf: buffer[0..<min(n, most - kept.count)]) }
                }
                close(fd)
                lock.lock(); keep(kept); lock.unlock()
            }
        }
        drain(outPipe[0], most: outputMost) { ran.output = $0 }
        drain(errPipe[0], most: errorMost) { ran.error = $0 }
        let exited = DispatchSemaphore(value: 0)
        var status: Int32 = 0
        DispatchQueue.global().async {
            while waitpid(pid, &status, 0) == -1 && errno == EINTR {}
            exited.signal()
        }
        if exited.wait(timeout: .now() + timeout) == .timedOut {
            ran.timedOut = true
            killpg(pid, SIGTERM)
            if exited.wait(timeout: .now() + 2) == .timedOut { killpg(pid, SIGKILL); exited.wait() }
        }
        // Anything it left running would hold its output open.
        killpg(pid, SIGKILL)
        group.wait()
        if !ran.timedOut { ran.status = (status & 0x7f) == 0 ? (status >> 8) & 0xff : -1 }
        return ran
    }
}

// MARK: Which are here

/// Whether each coding agent is here and signed in, as setup shows it.
struct SummaryProviderState: Equatable {
    enum Kind: String { case ready, missing, signedOut, changed, unsupported
        /// Signed in with an API account (a Console login): it works, billed to that account.
        case apiAccount
    }
    /// Whether a summary can be made with it.
    var works: Bool { kind == .ready || kind == .apiAccount }
    var kind: Kind
    var reason: String
    var program: URL?

    static func missing(_ provider: SummaryProvider) -> SummaryProviderState {
        SummaryProviderState(kind: .missing, reason: "not installed")
    }
}

/// Finds the coding agents, asks each whether it's signed in, and checks its
/// help still lists every flag a summary passes, off the main queue, at most
/// once a minute unless asked: setup asks every second or so.
final class SummaryProviders {
    static let shared = SummaryProviders()
    static let staleAfter: TimeInterval = 60

    private let queue = DispatchQueue(label: "smolcorp.summary-providers")
    private let lock = NSLock()
    private var known: [SummaryProvider: SummaryProviderState]?
    private var checkedAt = Date.distantPast
    private var checking = false
    /// Each program's help text, by its path and when it changed.
    private var helps: [String: String] = [:]

    /// What's known, or nil before the first check has finished.
    var states: [SummaryProvider: SummaryProviderState]? {
        lock.lock(); defer { lock.unlock() }
        return known
    }

    var isChecking: Bool {
        lock.lock(); defer { lock.unlock() }
        return checking
    }

    /// Checks again, in the background, if it's been a while or `force`.
    func refresh(force: Bool = false) {
        lock.lock()
        guard !checking, force || Date().timeIntervalSince(checkedAt) > Self.staleAfter else { return lock.unlock() }
        checking = true
        lock.unlock()
        queue.async { [self] in
            let found = Dictionary(uniqueKeysWithValues: SummaryProvider.agents.map { ($0, check($0)) })
            lock.lock()
            known = found
            checkedAt = Date()
            checking = false
            lock.unlock()
        }
    }

    /// What's known, checked now if never yet: never on the main queue.
    func statesNow() -> [SummaryProvider: SummaryProviderState] {
        if let states { return states }
        refresh(force: true)
        queue.sync {}
        return states ?? [:]
    }

    /// One provider, checked now.
    func check(_ provider: SummaryProvider) -> SummaryProviderState {
        guard provider.isSupported else { return SummaryProviderState(kind: .unsupported, reason: SummaryProvider.unsupportedReason) }
        guard let program = Self.program(provider) else { return .missing(provider) }
        let environment = SummaryCLI.environment()
        let home = Paths.home
        // Its help: each flag a summary passes must still be in it.
        let help = helpText(provider, program, environment)
        let missing = SummaryCLI.flags(provider).filter { !SummaryCLI.lists(help, $0) }
        if !missing.isEmpty {
            return SummaryProviderState(kind: .changed, reason: "updated, and no longer takes what Smolcorp passes: summaries with it need an update of Smolcorp",
                                        program: program)
        }
        let signedIn: Bool
        switch provider {
        case .claude:
            switch SummaryCLI.claudeLogin(quick(program, ["auth", "status", "--json"], environment).output) {
            case .ready?: return SummaryProviderState(kind: .ready, reason: provider.plan, program: program)
            case .apiAccount?:
                return SummaryProviderState(kind: .apiAccount, reason: "signed in with a Console account: bills your API account", program: program)
            default: signedIn = false
            }
        case .codex:
            signedIn = quick(program, ["login", "status"], environment).status == 0
        case .gemini:
            signedIn = FileManager.default.fileExists(atPath: home.appendingPathComponent(".gemini/oauth_creds.json").path)
        case .grok:
            signedIn = FileManager.default.fileExists(atPath: home.appendingPathComponent(".grok/auth.json").path)
        case .cursor:
            let ran = quick(program, ["status"], environment)
            let said = String(decoding: ran.output + ran.error, as: UTF8.self).lowercased()
            signedIn = ran.status == 0 && !said.contains("not logged in") && !said.contains("not authenticated")
        case .opencode:
            // Any model it can use: an account it's connected, or one on this computer.
            let ran = quick(program, ["models"], environment)
            signedIn = ran.status == 0 && !String(decoding: ran.output, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            // Its summary agent must exist and be denied every tool, or it isn't run at all.
            if signedIn, !SummaryCLI.openCodeSummaryAgentIsLocked(quick(program, ["debug", "agents"], environment).output) {
                return SummaryProviderState(kind: .unsupported,
                                            reason: "its summary agent isn't there, or may use tools: not supported with this version", program: program)
            }
        case .apiKey:
            signedIn = false
        }
        guard signedIn else { return SummaryProviderState(kind: .signedOut, reason: "installed, not signed in: \(provider.signIn)", program: program) }
        return SummaryProviderState(kind: .ready, reason: provider.plan, program: program)
    }

    private func helpText(_ provider: SummaryProvider, _ program: URL, _ environment: [String: String]) -> String {
        let changed = (try? FileManager.default.attributesOfItem(atPath: program.path))?[.modificationDate] as? Date
        let stamp = "\(provider.rawValue) \(program.path) \(changed?.timeIntervalSince1970 ?? 0)"
        if let help = helps[stamp] { return help }
        let ran = quick(program, provider.helpArguments, environment)
        let help = String(decoding: ran.output + ran.error, as: UTF8.self)
        helps[stamp] = help
        return help
    }

    private func quick(_ program: URL, _ arguments: [String], _ environment: [String: String]) -> SummaryCLI.Ran {
        SummaryCLI.spawn(program, arguments, input: Data(), environment: environment, folder: FileManager.default.temporaryDirectory.path,
                         timeout: 10)
    }

    /// Where `provider`'s program is, or nil. An `agent` that is Grok's isn't Cursor's.
    static func program(_ provider: SummaryProvider) -> URL? {
        for name in provider.programs {
            guard let program = OpenCodeInstall.program(named: name) else { continue }
            if provider == .cursor, name == "agent" {
                let ran = SummaryCLI.spawn(program, ["--version"], input: Data(), environment: SummaryCLI.environment(),
                                           folder: FileManager.default.temporaryDirectory.path, timeout: 10)
                if String(decoding: ran.output + ran.error, as: UTF8.self).lowercased().contains("grok") { continue }
            }
            return program
        }
        return nil
    }

    /// For tests: what's known, set.
    func set(_ states: [SummaryProvider: SummaryProviderState]?) {
        lock.lock(); known = states; checkedAt = states == nil ? .distantPast : Date(); lock.unlock()
    }
}

// MARK: The choice

enum SummaryChoice {
    /// The provider summaries use: the one saved, even when it stops working
    /// (it then says why, and never quietly sends elsewhere); else, for one who
    /// turned summaries on with a key before there was a choice, the key, as
    /// it was; else the first coding agent that works, then the key.
    static func chosen(saved: String, optIn: Bool, states: [SummaryProvider: SummaryProviderState], hasKey: Bool) -> SummaryProvider? {
        if let provider = SummaryProvider(rawValue: saved) {
            guard provider.isSupported else { return nil }
            return provider == .apiKey && !hasKey ? nil : provider
        }
        if optIn && hasKey { return .apiKey }
        if let first = SummaryProvider.agents.first(where: { $0.isSupported && states[$0]?.kind == .ready }) { return first }
        return hasKey ? .apiKey : nil
    }

    /// Whether `provider` can make a summary now.
    static func works(_ provider: SummaryProvider, states: [SummaryProvider: SummaryProviderState], hasKey: Bool) -> Bool {
        provider == .apiKey ? hasKey : provider.isSupported && states[provider]?.works == true
    }
}
