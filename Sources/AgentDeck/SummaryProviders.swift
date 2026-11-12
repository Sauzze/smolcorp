import Foundation

/// Who makes the summaries, as a saved settings file names it: a coding agent's
/// id, or "apikey" for the Anthropic API. The Go core runs them; this keeps
/// only the ids a settings file may hold (see `Config`).
enum SummaryProvider: String, CaseIterable {
    case claude, codex, gemini, grok, cursor, opencode
    case apiKey = "apikey"
}
