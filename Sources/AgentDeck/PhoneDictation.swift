import Foundation

/// Phone microphone: the phone listens with its own speech
/// recognition (the browser's Web Speech API) and sends only the words it
/// heard to `POST /dictate`. No audio ever reaches the Mac, and nothing is
/// kept: the text is typed into the agent's tab and forgotten.
///
/// iOS only lets a page use the microphone over HTTPS, so this works when the
/// deck is opened through `tailscale serve`; over plain http on the Wi-Fi,
/// the mic is the Mac's, as before. The same route carries text typed on the
/// phone, over the Wi-Fi as well (I34), with the same checks.
enum PhoneDictation {
    /// The most text one dictation may carry, in characters: well beyond a
    /// minute of speech, short enough that a stray request can't type a book.
    static let longest = 2000
    /// …and in bytes: one character can hide any number of combining marks,
    /// each counted as part of it. Room for 2,000 emoji of several pieces.
    static let longestBytes = 8 * longest
    /// Claude Code's own /voice sends what it heard once it's this many words.
    static let wordsToSend = 3
    /// How long the agent is given to take the text in before Return goes,
    /// on its own. Sent with the text, in the same read, Claude Code takes
    /// the lot for a paste, and a Return in a paste is a new line in it.
    static let settle: TimeInterval = 0.25
    /// The peek at the input line: this many of the tab's last lines, each
    /// cut to this many characters. Enough for Claude Code's input box and
    /// what's under it; never its scrollback.
    static let peekLines = 10
    static let peekWidth = 240
    /// The most lines the core asks for when a pick looks for Claude Code's menu on the screen (desktop/talkpick.go).
    static let menuLines = 60
    /// At most one peek a session this often: one look after each dictation, never a live view.
    static let peekGap: TimeInterval = 1

    /// Whether a peek at a session comes too soon after its last, `last`.
    static func peekTooSoon(last: Date?, now: Date) -> Bool {
        last.map { now.timeIntervalSince($0) < peekGap } ?? false
    }

    /// The text as it will be typed. On one line: control characters become
    /// spaces, runs of spaces one, trimmed. Of several lines (I34): the line
    /// breaks (Unicode's own line and paragraph separators too) kept, each a
    /// line feed, for a bracketed paste to carry (see `Deck.typePhoneDictation`);
    /// the lines themselves as they were, a tab kept, other control characters
    /// spaces, blank lines at either end and the spaces round the whole trimmed.
    /// Nil if nothing's left or it's too long (`refusal` says which).
    /// Only control characters (Cc): the invisible format ones (Cf) stay, as the
    /// joiner in 👩‍💻, which CharacterSet.controlCharacters counts too.
    static func prepared(_ text: String) -> String? {
        let single = flattened(text)
        guard !single.isEmpty, single.count <= longest, single.utf8.count <= longestBytes else { return nil }
        return single
    }

    /// Whether the text, as `prepared` gave it, is of several lines.
    static func isLines(_ words: String) -> Bool { words.contains("\n") }

    /// What ends a line: a line feed, a return (or both, as Windows writes them), and Unicode's next-line,
    /// line and paragraph separators. A vertical tab or form feed is a control character like any other.
    private static let lineBreaks: Set<Unicode.Scalar> = ["\n", "\r", "\u{85}", "\u{2028}", "\u{2029}"]

    private static func flattened(_ text: String) -> String {
        let scalars = text.replacingOccurrences(of: "\r\n", with: "\n").unicodeScalars.map { scalar -> Unicode.Scalar in
            if lineBreaks.contains(scalar) { return "\n" }
            return scalar.properties.generalCategory == .control && scalar != "\t" ? " " : scalar
        }
        let lines = String(String.UnicodeScalarView(scalars)).trimmingCharacters(in: .whitespacesAndNewlines)
        guard !lines.contains("\n") else { return lines }
        // One line, as before: a tab a space too (typed, it would be a key), runs of spaces one.
        return lines.replacingOccurrences(of: "\t", with: " ").split(separator: " ", omittingEmptySubsequences: true).joined(separator: " ")
    }

    /// Why `prepared` gave nothing, for the phone to say: too long, or nothing to type.
    static func refusal(_ text: String) -> String {
        let single = flattened(text)
        return single.count > longest || single.utf8.count > longestBytes ? tooLong : "Nothing to type"
    }
    static let tooLong = "Too long: at most \(longest) characters at a time."

    /// Whether the agent would take the text for a shell command: Claude Code,
    /// Codex, Gemini CLI and OpenCode all run a prompt starting with "!" in a shell.
    /// By its first code point, as they look: "!" with an accent, a joiner or an
    /// emoji's variation selector on it is one character to Swift, but still "!" to them.
    static func runsInShell(_ words: String) -> Bool { words.unicodeScalars.first == "!" }

    /// Whether the text goes with Return: with the setting on, and once it's
    /// a few words, as Claude Code does. Shorter, it waits for you to send.
    static func sends(_ text: String, autoSend: Bool) -> Bool {
        autoSend && text.split(whereSeparator: \.isWhitespace).count >= wordsToSend
    }

    /// The end of what a tab shows, as the phone's peek gives it: the blank
    /// lines under it dropped, its last `lines` (`peekLines`), each cut to `peekWidth`,
    /// with escape and other control characters gone, so nothing is drawn
    /// but text, and no blank lines over it.
    static func peek(_ screen: String, lines count: Int = peekLines) -> [String] {
        let blank = { (line: String) in line.trimmingCharacters(in: .whitespaces).isEmpty }
        var lines = screen.components(separatedBy: .newlines).map { line in
            String(String(line.unicodeScalars.filter { !CharacterSet.controlCharacters.contains($0) }).prefix(peekWidth))
        }
        while let last = lines.last, blank(last) { lines.removeLast() }
        return Array(lines.suffix(count).drop(while: blank))
    }
}
