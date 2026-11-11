import Foundation
import Security

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

/// Why a summary couldn't be had, in the phone's words: fixed text, never a
/// reply's body, the request or the key.
struct SummaryError: Error, Equatable {
    let message: String
    let status: Int

    static let off = SummaryError(message: "Summaries are off: turn them on in setup, on your computer.", status: 403)
    static let noKey = SummaryError(message: "No Anthropic API key: add yours in setup, on your computer.", status: 403)
    static let noProvider = SummaryError(message: "Nothing to make summaries with: pick a coding agent or add a key in setup, on your computer.", status: 403)
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
        // Not known yet: the Keychain is asked off the main queue (see `learnHint`), and until it answers
        // this goes by the environment alone. Whoever must have the answer first uses `whenKnown`.
        // A test copy's key is a file, which can be written or removed while the copy runs: it is read again each time
        // (a few bytes, and no Keychain to wait on), rather than trusted by its modification time, which a quick
        // rewrite within the same tick can leave unchanged. The real app keeps what it learned.
        if testMode { knownHint = .some(store.hint()) }
        let known = knownHint
        if known == nil { learnHint() }
        if let hint = known ?? nil { return Shown(source: .keychain, hint: hint) }
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

    /// `save` off the main queue: `done` is told on it, once the Keychain has answered.
    static func save(_ text: String, done: @escaping (Bool) -> Void) {
        queue.async {
            let kept = save(text)
            DispatchQueue.main.async { done(kept) }
        }
    }

    /// `remove` off the main queue: `done` is told on it, once the Keychain has finished.
    static func remove(done: @escaping () -> Void) {
        queue.async {
            remove()
            DispatchQueue.main.async(execute: done)
        }
    }

    /// Where a pasted key is kept: the Keychain, or a stand-in in tests.
    struct Store {
        var read: () -> String?
        var hint: () -> String?
        var write: (_ key: String, _ hint: String) -> Bool
        var delete: () -> Void
    }

    static var store = defaultStore()

    /// The Keychain, or in a test copy a file in its own support folder (the Keychain is never asked there).
    static func defaultStore(testCopy: Bool = TestCopy.isOn, support: URL? = TestCopy.support,
                             keychain: Store = .keychain) -> Store {
        if testCopy, let support { return .file(support.appendingPathComponent(TestCopySecrets.apiKeyName)) }
        return keychain
    }

    /// The hint, from memory once known (nil inside: known to be none), so
    /// setup, asking every second or so, doesn't go to the Keychain each time.
    /// Guarded by `lock`: `save` and `remove` can finish on `queue` while the main queue asks.
    private static var knownHintValue: String??
    private static let lock = NSLock()
    private static var knownHint: String?? {
        get { lock.lock(); defer { lock.unlock() }; return knownHintValue }
        set { lock.lock(); defer { lock.unlock() }; knownHintValue = newValue }
    }

    private static var learning = false
    /// Told on the main queue when the hint has been learned from the Keychain, so what shows it can redraw.
    static var onLearned: () -> Void = {}

    /// Asks the Keychain for the hint on `queue`, once at a time, and keeps it unless a save or remove got there first.
    static func learnHint() {
        lock.lock()
        let begin = !learning && knownHintValue == nil
        if begin { learning = true }
        lock.unlock()
        guard begin else { return }
        queue.async {
            let hint = store.hint()
            DispatchQueue.main.async {
                lock.lock()
                if knownHintValue == nil { knownHintValue = .some(hint) }
                learning = false
                lock.unlock()
                onLearned()
            }
        }
    }

    /// Runs `then` on the main queue once the hint is known (at once if it is).
    static func whenKnown(_ then: @escaping () -> Void) {
        if knownHint != nil { return then() }
        queue.async {
            let hint = store.hint()
            DispatchQueue.main.async {
                lock.lock()
                if knownHintValue == nil { knownHintValue = .some(hint) }
                lock.unlock()
                then()
            }
        }
    }

    /// Where the Keychain is asked to keep or drop the key. macOS may show a
    /// dialog and wait for an answer, so this is never the main queue, which
    /// answers setup, the office, phones and hooks.
    static let queue = DispatchQueue(label: "smolcorp.summary-key")

    /// Asks the store again next time, as after the store is swapped in a test.
    static func forgetKnown() { lock.lock(); knownHintValue = nil; learning = false; lock.unlock() }
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
