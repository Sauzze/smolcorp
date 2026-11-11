import CoreGraphics
import Foundation

/// The coding agents the deck follows, and what each one's terminal UI
/// answers to.
///
/// Every agent reports through the same hook script, each in its own dialect.
/// `canonical(_:)` translates them all into Claude Code's, so one state
/// machine in `Sessions` serves every agent.
enum Agent: String, CaseIterable {
    case claude, codex, gemini, opencode, grok

    var name: String {
        switch self {
        case .claude: return "Claude Code"
        case .codex: return "Codex"
        case .gemini: return "Gemini CLI"
        case .opencode: return "OpenCode"
        case .grok: return "Grok"
        }
    }

    /// Stops a running turn. Grok cancels on Ctrl+C and ignores Esc;
    /// OpenCode asks for Esc twice, so a single stray press can't stop it.
    var interrupt: Terminal.Keystroke {
        switch self {
        case .grok: return Terminal.Keystroke(key: .c, modifiers: .maskControl)
        case .opencode: return Terminal.Keystroke(key: .escape, times: 2)
        default: return Terminal.Keystroke(key: .escape)
        }
    }

    /// Declines a permission prompt. In Grok, Esc only moves focus off the
    /// prompt; Ctrl+C is what cancels the request.
    var reject: Terminal.Keystroke {
        self == .grok ? Terminal.Keystroke(key: .c, modifiers: .maskControl) : Terminal.Keystroke(key: .escape)
    }

    /// The command that starts a fresh conversation in the same tab.
    var newConversation: String {
        switch self {
        case .claude, .gemini: return "/clear"
        case .codex, .opencode, .grok: return "/new"
        }
    }

    /// Thinking effort set with `/effort`, and push-to-talk through
    /// `/voice`: both are Claude Code's own.
    var hasEffortDial: Bool { self == .claude }
    var hasVoice: Bool { self == .claude }

    /// Whether the agent asks its terminal for bracketed paste while it waits at its prompt, so a paste
    /// of several lines comes to it between markers, its line breaks new lines in the text, never Return.
    /// Terminal.app, iTerm2 and Ghostty can't say whether it did, so several lines are pasted there only
    /// for these (`LinesWay.clipboard`), and only while the desk waits at its prompt with nothing else
    /// reading the tab's keys (`Deck.clipboardRefusal`). Only what was shown is on the list (the owner's
    /// rule: when in doubt, nothing is typed). Read on 11 Nov 2026 in the installed binaries' strings,
    /// none of them started:
    /// - Claude Code 2.1.322: its terminal-mode manager writes `set("bracketedPaste")` (ESC[?2004h) when
    ///   its input's raw mode first turns on, `reset("bracketedPaste")` when raw mode drops to zero, and
    ///   reasserts it on resume. It goes off in two other places only: a terminal `$EDITOR` (Ctrl-G,
    ///   opening a file: its alternate-screen switch suspends every mode), and Ctrl-Z. A window editor
    ///   (`code --wait`, `subl --wait`) goes through a handoff that turns off only the mouse and focus
    ///   modes: bracketed paste stays on while it waits. The deck's own checks cover the two (something
    ///   else with the tab as its input, the agent not in front).
    /// - OpenCode 2.0.32 (Homebrew): has ESC[?2004h among the modes it turns on once the terminal says it
    ///   has them, asks for the mode's state (ESC[?2004$p) and keeps it as a capability that's off unless
    ///   told otherwise. In a terminal that doesn't answer, it may never turn it on: left off (RI34c B1).
    ///   It comes back after a paste of two lines by hand into its prompt in each of Terminal.app, iTerm2
    ///   and Ghostty that doesn't send the first line on its own.
    /// - Codex and Gemini CLI: not installed on that Mac, so nothing could be checked: left off.
    /// - Grok CLI: its binary has the sequences too (crossterm), but when it turns them on wasn't
    ///   checked: left off.
    var asksForBracketedPaste: Bool { self == .claude }

    /// The agent's process in its Terminal tab, for finding the tab when its
    /// hooks run elsewhere: Codex runs them in a shared background service,
    /// whose process tree leads to no tab. Gemini CLI runs as `node`, which
    /// says nothing, but runs its hooks in the tab.
    var processName: String? {
        self == .gemini ? nil : rawValue
    }

    /// What the agent is called among the processes in its tab's foreground,
    /// for firing it and nothing else there. `node` is all Gemini CLI says,
    /// which is enough inside the agent's own tab.
    var foregroundName: String { processName ?? "node" }

    /// Whether `commandLine`, as `ps -o args=` gives it, is this agent: its
    /// own program, or Node or Bun running the agent's own package, as Claude
    /// Code from npm runs (node …/node_modules/@anthropic-ai/claude-code/cli.js)
    /// and Grok CLI from Bun. Node or Bun running anything else, as `npm run
    /// dev` or vite started in the tab after the agent crashed, is someone's
    /// work, not the agent.
    func isItself(commandLine: String) -> Bool {
        guard let first = commandLine.split(separator: " ").first else { return false }
        let program = (String(first) as NSString).lastPathComponent.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "-"))
        let name = rawValue
        guard program == "node" || program == "bun" else { return program == name || program == foregroundName.lowercased() }
        let rest = commandLine.dropFirst(first.count)
        // The package the script is in, exactly: what follows the last
        // node_modules, with its scope. claude-code-router, say, isn't Claude Code.
        if let modules = rest.range(of: "node_modules/", options: .backwards) {
            let parts = rest[modules.upperBound...].split(separator: "/", maxSplits: 2).map { $0.lowercased() }
            let package = (parts.first?.hasPrefix("@") == true ? parts.prefix(2) : parts.prefix(1)).joined(separator: "/")
            return packages.contains(package)
        }
        // A script of its own, outside any package: by its own name, exactly.
        guard let script = rest.split(separator: " ").first(where: { !$0.hasPrefix("-") }) else { return false }
        return ((String(script) as NSString).lastPathComponent as NSString).deletingPathExtension.lowercased() == name
    }

    /// The npm packages the agent is installed as, run by Node or Bun.
    var packages: Set<String> {
        switch self {
        case .claude: return ["@anthropic-ai/claude-code"]
        case .codex: return ["@openai/codex"]
        case .gemini: return ["@google/gemini-cli"]
        case .opencode: return ["opencode-ai", "opencode"]
        case .grok: return ["@vibe-kit/grok-cli"]
        }
    }

    /// Which agent sent `event`. The hook script names it, except for Grok:
    /// it also runs Claude Code's hooks, and tells itself apart by its
    /// camelCase envelope.
    static func of(_ event: [String: Any], reportedAs reported: String) -> Agent {
        if event["hookEventName"] != nil { return .grok }
        return Agent(rawValue: reported) ?? .claude
    }

    /// `event` in Claude Code's words, or nil for one the deck should not
    /// act on.
    func canonical(_ event: [String: Any]) -> [String: Any]? {
        switch self {
        case .claude, .codex, .opencode:
            // Codex uses Claude Code's names, and OpenCode's plugin sends them.
            return event
        case .gemini:
            return Agent.renaming(event, events: [
                "BeforeAgent": "UserPromptSubmit", "AfterAgent": "Stop",
                "BeforeTool": "PreToolUse", "AfterTool": "PostToolUse",
                // Gemini CLI notifies only to ask for a tool's permission.
                "Notification": "PermissionRequest",
            ])
        case .grok:
            // A subagent's events are not its session's.
            guard event["subagentType"] == nil else { return nil }
            var claude = event
            for (camel, snake) in ["sessionId": "session_id", "toolName": "tool_name", "notificationType": "notification_type"] {
                claude[snake] = event[camel]
            }
            // Grok also fires Stop as a session ends; only a turn's end counts.
            if claude["hook_event_name"] as? String == "Stop",
               let reason = event["reason"] as? String, reason != "end_turn" { return nil }
            return Agent.renaming(claude, events: ["StopCancelled": "Interrupt"])
        }
    }

    private static func renaming(_ event: [String: Any], events: [String: String]) -> [String: Any] {
        var renamed = event
        if let name = event["hook_event_name"] as? String, let claude = events[name] {
            renamed["hook_event_name"] = claude
        }
        return renamed
    }
}
