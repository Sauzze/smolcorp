import Foundation
import Security

/// ✨ Summary: what an agent has been doing, in a few lines written by Claude
/// Haiku from the end of its Claude Code transcript, for a phone that can't
/// see the Terminal. Off until you turn it on in setup, and made with your own
/// Anthropic API key, billed to you.
///
/// Only the tail of the transcript is read, tool output cut down to a line,
/// lines that look like secrets taken out, and at most `TranscriptTail.budget`
/// characters sent, and only when a summary is asked for. The key (see
/// `SummaryKey`) is read as each call is made, held in memory only for it, and
/// never written anywhere else, logged or answered.

// MARK: The transcript's tail

/// The end of a Claude Code transcript as short plain lines, oldest first:
/// what the user asked, what Claude said, each tool it ran in a line, and
/// each tool's result in a line.
enum TranscriptTail {
    /// How many characters are sent at most.
    static let budget = 16_000
    /// How much of the file is read from its end.
    static let reach: UInt64 = 1024 * 1024
    /// How much further back the latest request is looked for, when it's not in `reach`.
    static let lookBack: UInt64 = 16 * 1024 * 1024
    /// Each message's text, a tool's input and a tool's result, at most.
    static let textMost = 1_500
    static let toolMost = 140
    static let resultMost = 160

    /// What comes just before the latest request, at most: enough to tell what a short answer answers.
    static let leadIn = 500
    /// The latest request, when what follows it is too long to send whole.
    static let latestMost = 600
    /// How what the user last asked is marked, so the summary is about it.
    static let latest = "LATEST REQUEST"
    /// How a later message from another session is marked: it may give the agent its task now.
    static let latestPeer = "LATEST PEER MESSAGE"

    struct Extract: Equatable {
        /// The lines, oldest first, already redacted.
        var text: String
        /// "French" or "English", from what the user wrote.
        var language: String
        /// Which messages the latest request and any later peer message are, to tell new ones from them:
        /// their ids in the transcript.
        var latest: String? = nil
    }

    /// A message to the agent: what the user typed, or what another session
    /// sent it, `from` its name: a message, or a report it hands back.
    struct Message: Equatable {
        enum Kind: Equatable { case user, peer, report }
        var kind: Kind
        var text: String
        var id: String
        var from = ""

        var label: String {
            switch kind {
            case .user: return "USER: "
            case .peer: return "PEER \(from): "
            case .report: return "REPORT from \(from): "
            }
        }
        /// How it's marked as the latest of its kind.
        var mark: String { kind == .user ? latest + ": " : latestPeer + " (from \(from)): " }
        /// Its line: redacted before it's cut.
        var line: String { label + clip(Redact.text(text), textMost) }
    }

    /// The latest request, and a peer's message after it, each with where it is in the lines;
    /// nil there when it's from before them.
    private struct Latest {
        var request: (index: Int?, message: Message)?
        var peer: (index: Int?, message: Message)?
    }

    /// The tail of the transcript at `path`, or nil if there's none to read.
    static func read(_ path: String) -> Extract? {
        guard !path.isEmpty, let file = FileHandle(forReadingAtPath: path) else { return nil }
        defer { try? file.close() }
        let size = (try? file.seekToEnd()) ?? 0
        let start = size > reach ? size - reach : 0
        guard (try? file.seek(toOffset: start)) != nil, let data = try? file.read(upToCount: Int(reach)) else { return nil }
        var lines = data.split(separator: 0x0A, omittingEmptySubsequences: false).map { Data($0) }
        // A line cut off where the reading starts is no use here; it's the end of one looked at further back.
        var cut = Data()
        if start > 0, !lines.isEmpty { cut = lines.removeFirst() }
        var (extract, asked) = build(lines)
        // Tool results are kept whole in a transcript, so the latest request is often further back than `reach`.
        if !asked, start > 0 {
            let earlier = lastMessages(in: file, before: start, cut: cut)
            if earlier.request != nil || earlier.peer != nil {
                extract = build(lines, earlier: earlier).extract
            }
        }
        return extract.text.isEmpty ? nil : extract
    }

    /// The last request in `file` before `end`, and the last peer message after it, looked for
    /// backwards a chunk at a time, at most `lookBack`. `cut` is the end of the line `end` falls in.
    static func lastMessages(in file: FileHandle, before end: UInt64, cut: Data) -> (request: Message?, peer: Message?) {
        var end = end, cut = cut
        var peer: Message?
        let stop = end > lookBack ? end - lookBack : 0
        while end > stop {
            let start = end - min(end - stop, reach)
            guard (try? file.seek(toOffset: start)) != nil, let chunk = try? file.read(upToCount: Int(end - start)) else { break }
            var lines = (chunk + cut).split(separator: 0x0A, omittingEmptySubsequences: false)
            cut = start > 0 && !lines.isEmpty ? Data(lines.removeFirst()) : Data()
            for line in lines.reversed() {
                guard let entry = try? JSONSerialization.jsonObject(with: Data(line)) as? [String: Any],
                      let message = message(entry) else { continue }
                if message.kind == .user { return (message, peer) }
                if message.kind == .peer, peer == nil { peer = message }
            }
            end = start
        }
        return (nil, peer)
    }

    /// The lines of a transcript, as JSONL lines, cut down to `budget`: from the latest request
    /// on, marked, with a short lead-in before it, and a later peer message marked too; or, if
    /// that's too long, those and the end of what came after them. `earlier` is what's latest
    /// when it's before these lines.
    static func extract(_ jsonl: [Data], budget: Int = budget,
                        earlier: (request: Message?, peer: Message?) = (nil, nil)) -> Extract {
        build(jsonl, budget: budget, earlier: earlier).extract
    }

    /// `extract`, and whether the user asked anything in `jsonl`.
    private static func build(_ jsonl: [Data], budget: Int = budget,
                              earlier: (request: Message?, peer: Message?) = (nil, nil)) -> (extract: Extract, asked: Bool) {
        var items: [String] = []
        var messages: [(index: Int, message: Message)] = []
        for line in jsonl {
            guard let entry = try? JSONSerialization.jsonObject(with: line) as? [String: Any] else { continue }
            if let message = message(entry) {
                messages.append((items.count, message))
                items.append(message.line)
                continue
            }
            let kind = entry["type"] as? String
            guard kind == "user" || kind == "assistant", entry["isSidechain"] as? Bool != true,
                  let message = entry["message"] as? [String: Any] else { continue }
            items += render(message, from: kind!)
        }
        items = items.map(prepared)

        var latest = Latest()
        if let request = messages.last(where: { $0.message.kind == .user }) {
            latest.request = (request.index, request.message)
        } else if let request = earlier.request {
            latest.request = (nil, request)
        }
        let after = latest.request?.index ?? -1
        if let peer = messages.last(where: { $0.message.kind == .peer && $0.index > after }) {
            latest.peer = (peer.index, peer.message)
        } else if latest.request?.index == nil, let peer = earlier.peer {
            latest.peer = (nil, peer)
        }

        // The user's language, from their latest words if those lean either way: only what's sent of them.
        let typed = (earlier.request.map { [$0] } ?? []) + messages.map(\.message)
        let words = typed.filter { $0.kind == .user }.map { clip(Redact.text($0.text), textMost) }
        let language = Language.lean(words.suffix(1)) ?? Language.of(words.suffix(8))
        let id = [latest.request?.message.id, latest.peer?.message.id].compactMap { $0 }.joined(separator: " ")
        let extract = Extract(text: excerpt(items, latest, budget).joined(separator: "\n"), language: language,
                              latest: id.isEmpty ? nil : id)
        return (extract, latest.request?.index != nil)
    }

    /// `items` cut down to `budget` around what's `latest`, marked.
    private static func excerpt(_ items: [String], _ latest: Latest, _ budget: Int) -> [String] {
        let marks = [latest.request, latest.peer].compactMap { $0 }
        guard let first = marks.first else { return tail(items[...], budget) }
        var items = items
        for case let (index?, message) in marks { items[index] = marked(items[index], message) }
        // All of it from the first marked line on, if that's among `items` and fits, with a lead-in.
        if let index = first.index, size(items[index...]) <= budget {
            return lead(items[..<index], most: min(leadIn, budget - size(items[index...]))) + items[index...]
        }
        // Or each marked line first, cut short, then the end that fits: a later peer's message
        // stays where it is when the end from it fits.
        var head: [String] = []
        var from = 0
        let most = max(2, min(latestMost, budget / marks.count - 3)) // each with its "…", within `budget`
        for (index, message) in marks {
            if let index, message.kind == .peer, size(items[index...]) <= budget - size(head) { break }
            head += [clip(index.map { items[$0] } ?? marked(prepared(message.line), message), most), "…"]
            from = index.map { $0 + 1 } ?? from
        }
        return head + tail(items[from...], budget - size(head))
    }

    private static func size<C: Collection>(_ lines: C) -> Int where C.Element == String {
        lines.reduce(0) { $0 + $1.count + 1 }
    }

    /// `item` redacted, and with no line of its own starting as a mark does: only the latest messages' may.
    private static func prepared(_ item: String) -> String {
        Redact.text(item).replacingOccurrences(of: "\nLATEST ", with: "\n LATEST ")
    }

    /// A latest message's line, `item`, with its mark for its label.
    private static func marked(_ item: String, _ message: Message) -> String {
        message.mark + (item.hasPrefix(message.label) ? item.dropFirst(message.label.count) : item[...])
    }

    /// From the end of `items`, as many as `budget` takes.
    private static func tail(_ items: ArraySlice<String>, _ budget: Int) -> [String] {
        var kept: [String] = []
        var used = 0
        for item in items.reversed() {
            guard used + item.count + 1 <= budget else { break }
            kept.append(item)
            used += item.count + 1
        }
        return kept.reversed()
    }

    /// The end of what came before the latest request, in at most `most` characters, "…" first
    /// if there was more: whole lines, or the end of the last one alone, where a question is asked.
    private static func lead(_ before: ArraySlice<String>, most: Int) -> [String] {
        let most = most - 2 // for "…"
        var kept: [String] = []
        var used = 0
        for item in before.reversed() {
            if used + item.count + 1 <= most {
                kept.insert(item, at: 0)
                used += item.count + 1
                continue
            }
            // `item` is redacted already; again after the cut, as a line cut mid-word can start to look like a secret.
            if kept.isEmpty, most > 40 { kept = [Redact.text(clipStart(item, most - 1))] }
            break
        }
        if kept.count < before.count, most > 0 { kept.insert("…", at: 0) }
        return kept
    }

    /// The entry as a message to the agent, if it is one: what the user typed, or what another
    /// session sent it. One sent while the agent works is an attachment, queued into its turn.
    static func message(_ entry: [String: Any]) -> Message? {
        // A compaction's summary of the conversation so far is in the user's turn, but no one asked it.
        guard entry["isSidechain"] as? Bool != true, entry["isCompactSummary"] as? Bool != true,
              entry["isVisibleInTranscriptOnly"] as? Bool != true else { return nil }
        let content: Any?, origin: [String: Any]?, meta: Bool
        switch entry["type"] as? String {
        case "user":
            guard let message = entry["message"] as? [String: Any] else { return nil }
            (content, origin, meta) = (message["content"], entry["origin"] as? [String: Any], entry["isMeta"] as? Bool == true)
        case "attachment":
            guard let queued = entry["attachment"] as? [String: Any], queued["type"] as? String == "queued_command",
                  queued["commandMode"] as? String == "prompt" else { return nil }
            (content, origin, meta) = (queued["prompt"], queued["origin"] as? [String: Any], queued["isMeta"] as? Bool == true)
        default:
            return nil
        }
        let text = words(content)
        let id = entry["uuid"] as? String ?? entry["timestamp"] as? String ?? text
        switch origin?["kind"] as? String {
        case "human":
            return text.isEmpty || isNote(text) ? nil : Message(kind: .user, text: text, id: id)
        case "peer":
            // A report handed back is what its body says around it.
            let report = origin?["handback"] as? Bool == true
            let said = (report ? origin?["handbackReportText"] as? String : nil) ?? origin?["body"] as? String ?? text
            let body = said.trimmingCharacters(in: .whitespacesAndNewlines)
            // Its name is redacted too: it's shown as given.
            let name = clip(Redact.text(firstLine(origin?["name"] as? String ?? "")), 60)
            return body.isEmpty ? nil : Message(kind: report ? .report : .peer, text: body, id: id,
                                                from: name.isEmpty ? "another session" : name)
        case nil:
            // Where Claude Code didn't write who a message came from: the user's words, but not its own notes.
            guard entry["type"] as? String == "user", !meta, !text.isEmpty, !text.hasPrefix("<"), !text.hasPrefix("Caveat:"),
                  !isNote(text) else { return nil }
            return Message(kind: .user, text: text, id: id)
        default: // a task's notification, and anything new
            return nil
        }
    }

    /// A command run or an interruption, in the user's turn: shown as what it is, not as a request.
    private static func isNote(_ text: String) -> Bool {
        text.hasPrefix("<command-name>") || text.hasPrefix("[Request interrupted")
    }

    /// What a message says in words: its text, or its text blocks together.
    private static func words(_ content: Any?) -> String {
        (content as? String ?? resultText(content)).trimmingCharacters(in: .whitespacesAndNewlines)
    }

    /// A message's lines, each redacted before it's cut.
    private static func render(_ message: [String: Any], from kind: String) -> [String] {
        if let text = message["content"] as? String {
            return said(text, by: kind)
        }
        guard let blocks = message["content"] as? [[String: Any]] else { return [] }
        return blocks.flatMap { block -> [String] in
            switch block["type"] as? String {
            case "text":
                return said(block["text"] as? String ?? "", by: kind)
            case "tool_use":
                return ["TOOL " + tool(block)]
            case "tool_result":
                let error = block["is_error"] as? Bool == true
                let first = firstLine(resultText(block["content"]))
                return [(error ? "ERROR: " : "RESULT: ") + clip(first.isEmpty ? "(nothing)" : Redact.text(first), resultMost)]
            default: // thinking, images, and anything new
                return []
            }
        }
    }

    private static func said(_ text: String, by kind: String) -> [String] {
        let text = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return [] }
        guard kind == "user" else { return ["CLAUDE: " + clip(Redact.text(text), textMost)] }
        // What was asked is a `message`; what's left in the user's turn is Claude Code's own: a command, an interruption.
        if text.hasPrefix("<command-name>") {
            let name = text.range(of: "<command-name>").flatMap { open in
                text.range(of: "</command-name>").map { String(text[open.upperBound..<$0.lowerBound]) }
            } ?? ""
            return name.isEmpty ? [] : ["USER ran " + clip(Redact.text(name), 60)]
        }
        if text.hasPrefix("[Request interrupted") { return ["(the user interrupted)"] }
        return []
    }

    /// A tool's call in a line: its name and the one part of its input that says what it did.
    static func tool(_ block: [String: Any]) -> String {
        let name = block["name"] as? String ?? "tool"
        let input = block["input"] as? [String: Any] ?? [:]
        // From its input, only paths, commands and searches, never text it was given to write. What it gave back goes
        // as its first line (see above): for a file it read, that file's first line.
        for field in ["file_path", "notebook_path", "path", "command", "pattern", "url", "query", "description", "subject", "skill"] {
            if let value = input[field] as? String, !value.isEmpty {
                return "\(name): " + clip(Redact.text(firstLine(value)), toolMost)
            }
        }
        return name
    }

    static func resultText(_ content: Any?) -> String {
        if let text = content as? String { return text }
        if let blocks = content as? [[String: Any]] {
            return blocks.compactMap { $0["type"] as? String == "text" ? $0["text"] as? String : nil }.joined(separator: "\n")
        }
        return ""
    }

    static func firstLine(_ text: String) -> String {
        text.split(whereSeparator: \.isNewline).lazy
            .map { $0.trimmingCharacters(in: .whitespaces) }.first { !$0.isEmpty } ?? ""
    }

    static func clip(_ text: String, _ most: Int) -> String {
        text.count <= most ? text : String(text.prefix(most - 1)) + "…"
    }

    /// `text`'s end, in at most `most` characters, keeping its label.
    static func clipStart(_ text: String, _ most: Int) -> String {
        guard text.count > most else { return text }
        var label = ["USER: ", "CLAUDE: ", "RESULT: ", "ERROR: "].first { text.hasPrefix($0) } ?? ""
        if label.isEmpty, text.hasPrefix("PEER ") || text.hasPrefix("REPORT from "), let colon = text.range(of: ": ") {
            label = String(text[..<colon.upperBound])
        }
        return label + "…" + String(text.suffix(max(0, most - label.count - 1)))
    }
}

// MARK: Redaction

/// Lines that look like they hold a secret, taken out before anything is sent.
enum Redact {
    static let placeholder = "[redacted]"

    private static let patterns: [NSRegularExpression] = [
        #"(?<![A-Za-z0-9_-])sk-[A-Za-z0-9]"#,    // Anthropic, OpenAI and others' keys
        #"(?<![A-Za-z0-9_-])re_[A-Za-z0-9]"#,   // Resend
        #"(?i)key['"]?\s*[=:]"#,                  // API_KEY=…, "apiKey": …
        #"(?i)passw(or)?d"#,
        #"(?i)token"#,
        #"(?i)secret"#,
        #"(?i)bearer\s"#,
        #"(?<![A-Za-z0-9])(ghp|gho|github_pat|xox[abpr]|AKIA)[A-Za-z0-9_-]"#,
        #"(?<![A-Za-z0-9_])(sk|rk|pk)_(live|test)_[A-Za-z0-9]"#,  // Stripe
        // A JWT, from a token's start and possessive: as it was, a long run of "eyJ" took minutes.
        #"(?<![A-Za-z0-9_-])eyJ[A-Za-z0-9_-]{8,}+\.[A-Za-z0-9_-]{8,}+\."#,
        // postgres://user:password@… or redis://:password@…, found from its "://": a whole scheme before it took
        // minutes on a long line.
        #"[A-Za-z0-9+.-]://[^/\s:@]*:[^/\s@]+@"#,
        #"-----BEGIN [A-Z ]*PRIVATE KEY"#,
    ].map { try! NSRegularExpression(pattern: $0) }

    private static let pemEnd = try! NSRegularExpression(pattern: #"-----END [A-Z ]*PRIVATE KEY"#)
    private static let token40 = try! NSRegularExpression(pattern: #"(?<![A-Za-z0-9/+=])[A-Za-z0-9/+=]{40}(?![A-Za-z0-9/+=])"#)

    static func isSecret(_ line: String) -> Bool {
        let range = NSRange(line.startIndex..., in: line)
        return patterns.contains { $0.firstMatch(in: line, range: range) != nil } || hasAWSSecret(line)
    }

    /// A bare AWS secret key: 40 characters with capitals, small letters and
    /// digits, which a git commit's 40 lowercase hex never has.
    static func hasAWSSecret(_ line: String) -> Bool {
        token40.matches(in: line, range: NSRange(line.startIndex..., in: line)).contains { match in
            guard let range = Range(match.range, in: line) else { return false }
            let token = line[range]
            return token.contains(where: \.isUppercase) && token.contains(where: \.isLowercase) && token.contains(where: \.isNumber)
        }
    }

    /// `text` with each line that looks like a secret replaced, keeping its
    /// label, and a private key's every line, from its BEGIN through its END.
    static func text(_ text: String) -> String {
        var inKey = false
        return text.split(separator: "\n", omittingEmptySubsequences: false).map { line -> String in
            let line = String(line)
            let range = NSRange(line.startIndex..., in: line)
            if inKey {
                if pemEnd.firstMatch(in: line, range: range) != nil { inKey = false }
                return placeholder
            }
            guard isSecret(line) else { return line }
            if line.contains("-----BEGIN"), line.contains("PRIVATE KEY"), pemEnd.firstMatch(in: line, range: range) == nil { inKey = true }
            // "TOOL Bash: export X=…" keeps "TOOL Bash:", so the summary still knows a command ran.
            for label in ["USER: ", "CLAUDE: ", "RESULT: ", "ERROR: "] where line.hasPrefix(label) {
                return label + placeholder
            }
            if line.hasPrefix("TOOL "), let colon = line.firstIndex(of: ":") {
                return line[...colon] + " " + placeholder
            }
            return placeholder
        }.joined(separator: "\n")
    }
}

// MARK: Language

/// French or English, from the words the user wrote.
enum Language {
    private static let french: Set = [
        "le", "la", "les", "un", "une", "des", "est", "et", "je", "tu", "il", "on", "nous", "vous", "pas", "que", "qui",
        "pour", "dans", "sur", "avec", "ça", "c'est", "mais", "ou", "du", "au", "aux", "ce", "cette", "fais", "peux",
        "merci", "oui", "non", "bien", "faut", "veux", "moi", "mon", "ma", "mes", "ton", "ta", "tes", "comme", "aussi",
    ]
    private static let english: Set = [
        "the", "a", "an", "is", "and", "i", "you", "it", "we", "not", "that", "who", "for", "in", "on", "with", "this",
        "but", "or", "of", "to", "do", "can", "please", "thanks", "yes", "no", "my", "your", "what", "should", "make",
    ]

    static func of<S: Sequence>(_ texts: S) -> String where S.Element == String {
        lean(texts) ?? "English"
    }

    /// French or English, or nil if `texts` lean neither way.
    static func lean<S: Sequence>(_ texts: S) -> String? where S.Element == String {
        var fr = 0, en = 0
        for text in texts {
            let words = text.lowercased().split { !($0.isLetter || $0 == "'") }
            for word in words {
                if french.contains(String(word)) { fr += 1 }
                if english.contains(String(word)) { en += 1 }
            }
            if text.range(of: "[éèêàùçôî]", options: .regularExpression) != nil { fr += 1 }
        }
        return fr > en ? "French" : en > fr ? "English" : nil
    }
}

// MARK: Asking Claude

/// Why a summary couldn't be had, in the phone's words: fixed text, never a
/// reply's body, the request or the key.
struct SummaryError: Error, Equatable {
    let message: String
    let status: Int

    static let off = SummaryError(message: "Summaries are off: turn them on in setup, on your computer.", status: 403)
    static let noKey = SummaryError(message: "No Anthropic API key: add yours in setup, on your computer.", status: 403)
    static let noSession = SummaryError(message: "That agent has gone.", status: 404)
    static let noHistory = SummaryError(message: "No history for this agent.", status: 404)
    static let tooMany = SummaryError(message: "Too many summaries just now: try again in a few minutes.", status: 429)
    static let timeout = SummaryError(message: "Anthropic didn't answer in time. Try again.", status: 504)
    static let unreachable = SummaryError(message: "Couldn't reach Anthropic from your computer.", status: 502)
    static let badKey = SummaryError(message: "Anthropic refused the key. Check it in setup, on your computer.", status: 502)
    static let busy = SummaryError(message: "Anthropic is busy, or your key's rate limit is reached. Try again in a minute.", status: 502)
    static func refused(_ code: Int) -> SummaryError {
        switch code {
        case 401, 403: return badKey
        case 429, 529: return busy
        default: return SummaryError(message: "Anthropic said no (\(code)).", status: 502)
        }
    }
    static let empty = SummaryError(message: "Anthropic sent back nothing.", status: 502)
}

/// The Anthropic API key Summary uses: one you paste in setup, kept only in
/// the Keychain (never synced), or else `ANTHROPIC_API_KEY` from the app's
/// own environment. Never a project's `.env` or any other file. Whether
/// there's one, and its hint ("sk-ant-…ab12"), are read without the key: the
/// hint is kept as the item's comment, so drawing setup never asks the
/// Keychain for the secret, and only a summary being made does.
enum SummaryKey {
    /// Whether this is a test copy's run (scripts/test-copy.sh's variables):
    /// then the key is its own, never the real store's or the environment's.
    static let testMode = TestCopy.isOn
    /// A test copy keeps its own, as it does the viewer's.
    static let service = testMode ? "smolcorp-summary-test" : "smolcorp-summary"
    static let account = "anthropic-api-key"
    static let environmentName = "ANTHROPIC_API_KEY"

    enum Source: String { case keychain, environment }

    /// What setup may show of it: where it comes from and its hint, never the key.
    struct Shown: Equatable {
        let source: Source
        let hint: String
    }

    /// A key as Anthropic makes them: sk-ant-…, one word, nothing else.
    static func isKey(_ text: String) -> Bool {
        text.hasPrefix("sk-ant-") && (20...512).contains(text.count)
            && text.allSatisfy { $0.isASCII && ($0.isLetter || $0.isNumber || $0 == "-" || $0 == "_") }
    }

    /// "sk-ant-…ab12": enough to tell which key, nothing to use.
    static func hint(_ key: String) -> String { "sk-ant-…" + key.suffix(4) }

    /// The key a summary is made with, read as it's made.
    static func read(environment: [String: String] = ProcessInfo.processInfo.environment, testMode: Bool = testMode) -> String? {
        if let stored = store.read() { return stored }
        // A test copy uses only a key pasted into it, never the one in the environment.
        guard !testMode else { return nil }
        return environment[environmentName].map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }.flatMap { isKey($0) ? $0 : nil }
    }

    /// What setup shows, without reading the key from the Keychain.
    static func shown(environment: [String: String] = ProcessInfo.processInfo.environment, testMode: Bool = testMode) -> Shown? {
        if knownHint == nil { knownHint = .some(store.hint()) }
        if let hint = knownHint ?? nil { return Shown(source: .keychain, hint: hint) }
        guard !testMode else { return nil }
        guard let key = environment[environmentName]?.trimmingCharacters(in: .whitespacesAndNewlines), isKey(key) else { return nil }
        return Shown(source: .environment, hint: hint(key))
    }

    static var isPresent: Bool { shown() != nil }

    /// Keeps a pasted key, replacing any before it. False if it isn't a key or couldn't be kept.
    @discardableResult
    static func save(_ text: String) -> Bool {
        let key = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard isKey(key), store.write(key, hint(key)) else { return false }
        knownHint = .some(hint(key))
        return true
    }

    static func remove() {
        store.delete()
        knownHint = .some(nil)
    }

    /// Where a pasted key is kept: the Keychain, or a stand-in in tests.
    struct Store {
        var read: () -> String?
        var hint: () -> String?
        var write: (_ key: String, _ hint: String) -> Bool
        var delete: () -> Void
    }

    static var store = Store.keychain

    /// The hint, from memory once known (nil inside: known to be none), so
    /// setup, asking every second or so, doesn't go to the Keychain each time.
    private static var knownHint: String??

    /// Asks the store again next time, as after the store is swapped in a test.
    static func forgetKnown() { knownHint = nil }
}

extension SummaryKey.Store {
    static let keychain = SummaryKey.Store(
        read: {
            let query: [String: Any] = [
                kSecClass as String: kSecClassGenericPassword,
                kSecAttrService as String: SummaryKey.service,
                kSecAttrAccount as String: SummaryKey.account,
                kSecReturnData as String: true,
                kSecMatchLimit as String: kSecMatchLimitOne,
            ]
            var found: CFTypeRef?
            guard SecItemCopyMatching(query as CFDictionary, &found) == errSecSuccess, let data = found as? Data,
                  let key = String(data: data, encoding: .utf8), SummaryKey.isKey(key) else { return nil }
            return key
        },
        hint: {
            // The item's comment, an attribute: reading it doesn't touch the key.
            let query: [String: Any] = [
                kSecClass as String: kSecClassGenericPassword,
                kSecAttrService as String: SummaryKey.service,
                kSecAttrAccount as String: SummaryKey.account,
                kSecReturnAttributes as String: true,
                kSecMatchLimit as String: kSecMatchLimitOne,
            ]
            var found: CFTypeRef?
            guard SecItemCopyMatching(query as CFDictionary, &found) == errSecSuccess,
                  let attributes = found as? [String: Any] else { return nil }
            return attributes[kSecAttrComment as String] as? String ?? "sk-ant-…"
        },
        write: { key, hint in
            let match: [String: Any] = [
                kSecClass as String: kSecClassGenericPassword,
                kSecAttrService as String: SummaryKey.service,
                kSecAttrAccount as String: SummaryKey.account,
            ]
            SecItemDelete(match as CFDictionary)
            var item = match
            item[kSecAttrLabel as String] = "Smolcorp Summary (Anthropic API key)"
            item[kSecAttrComment as String] = hint
            item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
            item[kSecValueData as String] = Data(key.utf8)
            return SecItemAdd(item as CFDictionary, nil) == errSecSuccess
        },
        delete: {
            let match: [String: Any] = [
                kSecClass as String: kSecClassGenericPassword,
                kSecAttrService as String: SummaryKey.service,
                kSecAttrAccount as String: SummaryKey.account,
            ]
            SecItemDelete(match as CFDictionary)
        })
}

/// One call to Claude's Messages API, with what it needs and nothing more.
enum SummaryRequest {
    static let endpoint = testEndpoint() ?? URL(string: "https://api.anthropic.com/v1/messages")!
    static let model = "claude-haiku-5-5"
    /// What setup's dialog names.
    static let modelName = "Claude Haiku 5.5"

    /// For a test copy only (scripts/test-copy.sh): a stand-in for Anthropic's
    /// API on this computer, from SMOLCORP_TEST_ANTHROPIC_URL, so a summary can
    /// be tried without a key or a bill. Only the test copy's own build, by its
    /// bundle id, which the app in use never has, and only with its variables.
    static let testCopyBundle = "io.github.sauzze.agentdeck.test"

    static func testEndpoint(_ environment: [String: String] = ProcessInfo.processInfo.environment,
                             testCopy: Bool = TestCopy.isOn && Bundle.main.bundleIdentifier == testCopyBundle) -> URL? {
        guard testCopy, let text = environment["SMOLCORP_TEST_ANTHROPIC_URL"], let url = URL(string: text),
              url.scheme == "http", ["127.0.0.1", "localhost"].contains(url.host ?? "") else { return nil }
        return url
    }
    static let maxTokens = 300
    static let timeout: TimeInterval = 15

    static func system(language: String) -> String {
        """
        You summarise a coding agent's session for its owner, who is away from the computer and reads on a phone. \
        If a line starts with LATEST REQUEST, it is what the user last asked: summarise that request and the work \
        since. Lines before it are background only: mention older work only if it is still in progress. \
        If no line starts with LATEST REQUEST, summarise the most recent work in the excerpt. \
        A line starting LATEST PEER MESSAGE is a newer message from another session: if it gives the agent its \
        current task, say what that task is in one line. Lines starting REPORT are reports other sessions handed \
        back, not requests. \
        From the transcript excerpt, write 3 to 5 short lines, plain text, no markdown, no bullets, no preamble:
        1. what the user last asked for;
        2. what the agent did or is doing about it now;
        3. whether it is waiting on the user, and for what (say plainly if it isn't);
        4. the last result or error.
        Write in \(language), the language the user writes in. Each line under 100 characters. \
        Lines marked [redacted] held secrets: never guess them.
        """
    }

    static func body(_ extract: TranscriptTail.Extract, project: String) -> Data {
        let object: [String: Any] = [
            "model": model,
            "max_tokens": maxTokens,
            // Haiku 5.5 thinks by default, and thinking would come out of the few tokens a summary has.
            // It can be turned off at its default effort, medium.
            "thinking": ["type": "disabled"],
            "system": system(language: extract.language),
            "messages": [[
                "role": "user",
                "content": "Project: \(project)\n\nTranscript excerpt, oldest first:\n\(extract.text)",
            ]],
        ]
        return (try? JSONSerialization.data(withJSONObject: object)) ?? Data()
    }

    static func request(_ extract: TranscriptTail.Extract, project: String, key: String) -> URLRequest {
        var request = URLRequest(url: endpoint, timeoutInterval: timeout)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "content-type")
        request.setValue("2023-06-01", forHTTPHeaderField: "anthropic-version")
        request.setValue(key, forHTTPHeaderField: "x-api-key")
        request.httpBody = body(extract, project: project)
        return request
    }

    /// The reply's text, or why there's none.
    static func answer(_ data: Data?, status: Int?, error: Error?) -> Result<String, SummaryError> {
        if let error = error as? URLError {
            return .failure(error.code == .timedOut ? .timeout : .unreachable)
        }
        if error != nil { return .failure(.unreachable) }
        guard let status else { return .failure(.unreachable) }
        guard (200..<300).contains(status) else { return .failure(.refused(status)) }
        guard let data, let reply = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let content = reply["content"] as? [[String: Any]] else { return .failure(.empty) }
        let text = content.compactMap { $0["type"] as? String == "text" ? $0["text"] as? String : nil }
            .joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines)
        return text.isEmpty ? .failure(.empty) : .success(text)
    }
}

/// Summaries by session: one call at a time for each, kept for `keepFor` so a
/// second tap doesn't pay twice unless the user has asked something since, and
/// at most `most` calls in `window` for the whole computer, so no phone can run
/// up your bill. Never retried: a tap is one call at most. Runs on the main
/// queue, as the deck does.
final class Summaries {
    static let keepFor: TimeInterval = 30
    static let most = 20
    static let window: TimeInterval = 10 * 60

    typealias Done = (Result<String, SummaryError>) -> Void
    /// Sends a request and hands back what came, on any queue. Swapped in tests.
    typealias Transport = (URLRequest, @escaping (Data?, Int?, Error?) -> Void) -> Void

    private var kept: [String: (text: String, latest: String?, at: Date)] = [:]
    private var waiting: [String: [Done]] = [:]
    /// When each call to Anthropic went, within `window`.
    private var calls: [Date] = []
    private let transport: Transport
    private let key: () -> String?
    private let now: () -> Date

    init(transport: @escaping Transport = Summaries.urlSession, key: @escaping () -> String? = { SummaryKey.read() },
         now: @escaping () -> Date = Date.init) {
        self.transport = transport
        self.key = key
        self.now = now
    }

    static let urlSession: Transport = transport(using: .ephemeral)

    /// Sends through a session of its own that never follows a redirect: one
    /// could take the key's header elsewhere. A redirect comes back as it is,
    /// and is answered as a refusal.
    static func transport(using base: URLSessionConfiguration) -> Transport {
        { request, done in
            let configuration = base.copy() as! URLSessionConfiguration
            configuration.httpCookieStorage = nil
            configuration.timeoutIntervalForRequest = SummaryRequest.timeout
            configuration.timeoutIntervalForResource = SummaryRequest.timeout
            configuration.urlCache = nil
            let session = URLSession(configuration: configuration, delegate: NoRedirects(), delegateQueue: nil)
            session.dataTask(with: request) { data, response, error in
                done(data, (response as? HTTPURLResponse)?.statusCode, error)
                session.finishTasksAndInvalidate()
            }.resume()
        }
    }

    final class NoRedirects: NSObject, URLSessionTaskDelegate {
        func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse,
                        newRequest request: URLRequest, completionHandler: @escaping (URLRequest?) -> Void) {
            completionHandler(nil)
        }
    }

    /// The summary of session `id`, from its transcript at `transcript`.
    func summarize(_ id: String, transcript: String, project: String, done: @escaping Done) {
        // A summary kept is served only while the user's latest request is still the one it covered,
        // which only the file can tell; it's read off the main queue, and a call made only if it isn't.
        if let hit = kept[id], now().timeIntervalSince(hit.at) < Summaries.keepFor {
            DispatchQueue.global(qos: .userInitiated).async {
                let latest = TranscriptTail.read(transcript)?.latest
                DispatchQueue.main.async { [weak self] in
                    if latest == hit.latest { return done(.success(hit.text)) }
                    self?.call(id, transcript: transcript, project: project, done: done)
                }
            }
            return
        }
        call(id, transcript: transcript, project: project, done: done)
    }

    /// A call to Anthropic for session `id`, or a wait on the one already going.
    private func call(_ id: String, transcript: String, project: String, done: @escaping Done) {
        if waiting[id] != nil { waiting[id]!.append(done); return }
        calls.removeAll { now().timeIntervalSince($0) >= Summaries.window }
        guard calls.count < Summaries.most else { return done(.failure(.tooMany)) }
        calls.append(now())
        waiting[id] = [done]
        let finish = { [weak self] (result: Result<String, SummaryError>, latest: String?) in
            DispatchQueue.main.async {
                guard let self else { return }
                if case let .success(text) = result { self.kept[id] = (text, latest, self.now()) }
                let callers = self.waiting.removeValue(forKey: id) ?? []
                callers.forEach { $0(result) }
            }
        }
        // The file and the Keychain are read off the main queue, so the deck never waits on them.
        let key = key, transport = transport
        DispatchQueue.global(qos: .userInitiated).async {
            guard let extract = TranscriptTail.read(transcript) else { return finish(.failure(.noHistory), nil) }
            guard let secret = key() else { return finish(.failure(.noKey), nil) }
            let request = SummaryRequest.request(extract, project: project, key: secret)
            transport(request) { data, status, error in
                finish(SummaryRequest.answer(data, status: status, error: error), extract.latest)
            }
        }
    }

    /// Forgets every summary kept, as the setting goes off.
    func forget() { kept = [:] }
}
