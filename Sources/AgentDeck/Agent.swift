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
        // The package the script is in: what follows the last node_modules,
        // with its scope, as @anthropic-ai/claude-code.
        if let modules = rest.range(of: "node_modules/", options: .backwards) {
            let parts = rest[modules.upperBound...].split(separator: "/", maxSplits: 2).map { $0.lowercased() }
            let package = parts.first?.hasPrefix("@") == true ? parts.prefix(2) : parts.prefix(1)
            return package.contains { $0.contains(name) }
        }
        // A script of its own, outside any package: by its own name.
        guard let script = rest.split(separator: " ").first(where: { !$0.hasPrefix("-") }) else { return false }
        return (String(script) as NSString).lastPathComponent.lowercased().contains(name)
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
