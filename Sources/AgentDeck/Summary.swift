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
    /// Each message's text, a tool's input and a tool's result, at most.
    static let textMost = 1_500
    static let toolMost = 140
    static let resultMost = 160

    struct Extract: Equatable {
        /// The lines, oldest first, already redacted.
        var text: String
        /// "French" or "English", from what the user wrote.
        var language: String
    }

    /// The tail of the transcript at `path`, or nil if there's none to read.
    static func read(_ path: String) -> Extract? {
        guard !path.isEmpty, let file = FileHandle(forReadingAtPath: path) else { return nil }
        defer { try? file.close() }
        let size = (try? file.seekToEnd()) ?? 0
        let start = size > reach ? size - reach : 0
        guard (try? file.seek(toOffset: start)) != nil, let data = try? file.read(upToCount: Int(reach)) else { return nil }
        var lines = data.split(separator: 0x0A).map { Data($0) }
        // A line cut off where the reading starts is no use.
        if start > 0, !lines.isEmpty { lines.removeFirst() }
        let extract = extract(lines)
        return extract.text.isEmpty ? nil : extract
    }

    /// The lines of a transcript, as JSONL lines, cut down to `budget`.
    static func extract(_ jsonl: [Data], budget: Int = budget) -> Extract {
        var items: [String] = []
        var asked: [String] = [] // what the user typed, for the language
        var lastAsk: (index: Int, line: String)?
        for line in jsonl {
            guard let entry = try? JSONSerialization.jsonObject(with: line) as? [String: Any] else { continue }
            let kind = entry["type"] as? String
            guard kind == "user" || kind == "assistant", entry["isSidechain"] as? Bool != true,
                  let message = entry["message"] as? [String: Any] else { continue }
            let meta = entry["isMeta"] as? Bool == true
            for rendered in render(message, from: kind!) {
                switch rendered {
                case let .asked(text):
                    guard !meta else { continue }
                    asked.append(text)
                    let line = "USER: " + clip(text, textMost)
                    lastAsk = (items.count, line)
                    items.append(line)
                case let .line(text):
                    items.append(text)
                }
            }
        }
        items = items.map(Redact.text)

        // From the end, as many lines as the budget takes.
        var kept: [String] = []
        var used = 0
        var firstKept = items.count
        for (i, item) in items.enumerated().reversed() {
            guard used + item.count + 1 <= budget else { break }
            kept.append(item)
            used += item.count + 1
            firstKept = i
        }
        kept.reverse()
        // What the user last asked, even from before the tail, so it's known what the agent is on.
        if let ask = lastAsk, ask.index < firstKept {
            let line = Redact.text(clip(ask.line, 600))
            kept.insert(line + "\n…", at: 0)
            while kept.count > 1 && kept.reduce(0, { $0 + $1.count + 1 }) > budget { kept.remove(at: 1) }
        }
        return Extract(text: kept.joined(separator: "\n"), language: Language.of(asked.suffix(8)))
    }

    private enum Rendered {
        case asked(String)
        case line(String)
    }

    private static func render(_ message: [String: Any], from kind: String) -> [Rendered] {
        if let text = message["content"] as? String {
            return said(text, by: kind)
        }
        guard let blocks = message["content"] as? [[String: Any]] else { return [] }
        return blocks.flatMap { block -> [Rendered] in
            switch block["type"] as? String {
            case "text":
                return said(block["text"] as? String ?? "", by: kind)
            case "tool_use":
                return [.line("TOOL " + tool(block))]
            case "tool_result":
                let error = block["is_error"] as? Bool == true
                let first = firstLine(resultText(block["content"]))
                return [.line((error ? "ERROR: " : "RESULT: ") + clip(first.isEmpty ? "(nothing)" : first, resultMost))]
            default: // thinking, images, and anything new
                return []
            }
        }
    }

    private static func said(_ text: String, by kind: String) -> [Rendered] {
        let text = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return [] }
        guard kind == "user" else { return [.line("CLAUDE: " + clip(text, textMost))] }
        // Claude Code's own notes in the user's turn: a command's output, a reminder, an interruption.
        if text.hasPrefix("<command-name>") {
            let name = text.range(of: "<command-name>").flatMap { open in
                text.range(of: "</command-name>").map { String(text[open.upperBound..<$0.lowerBound]) }
            } ?? ""
            return name.isEmpty ? [] : [.line("USER ran " + clip(name, 60))]
        }
        if text.hasPrefix("<") || text.hasPrefix("Caveat:") { return [] }
        if text.hasPrefix("[Request interrupted") { return [.line("(the user interrupted)")] }
        return [.asked(text)]
    }

    /// A tool's call in a line: its name and the one part of its input that says what it did.
    private static func tool(_ block: [String: Any]) -> String {
        let name = block["name"] as? String ?? "tool"
        let input = block["input"] as? [String: Any] ?? [:]
        // From its input, only paths, commands and searches, never text it was given to write. What it gave back goes
        // as its first line (see above): for a file it read, that file's first line.
        for field in ["file_path", "notebook_path", "path", "command", "pattern", "url", "query", "description", "subject", "skill"] {
            if let value = input[field] as? String, !value.isEmpty {
                return "\(name): " + clip(firstLine(value), toolMost)
            }
        }
        return name
    }

    private static func resultText(_ content: Any?) -> String {
        if let text = content as? String { return text }
        if let blocks = content as? [[String: Any]] {
            return blocks.compactMap { $0["type"] as? String == "text" ? $0["text"] as? String : nil }.joined(separator: "\n")
        }
        return ""
    }

    private static func firstLine(_ text: String) -> String {
        text.split(whereSeparator: \.isNewline).lazy
            .map { $0.trimmingCharacters(in: .whitespaces) }.first { !$0.isEmpty } ?? ""
    }

    static func clip(_ text: String, _ most: Int) -> String {
        text.count <= most ? text : String(text.prefix(most - 1)) + "…"
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
        #"eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\."#,              // a JWT
        #"[A-Za-z][A-Za-z0-9+.-]*://[^/\s:@]+:[^/\s@]+@"#,          // postgres://user:password@…
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
        var fr = 0, en = 0
        for text in texts {
            let words = text.lowercased().split { !($0.isLetter || $0 == "'") }
            for word in words {
                if french.contains(String(word)) { fr += 1 }
                if english.contains(String(word)) { en += 1 }
            }
            if text.range(of: "[éèêàùçôî]", options: .regularExpression) != nil { fr += 1 }
        }
        return fr > en ? "French" : "English"
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
        From the transcript excerpt, write 3 to 5 short lines, plain text, no markdown, no bullets, no preamble:
        1. what the user asked for;
        2. what the agent did or is doing now;
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
/// second tap doesn't pay twice, and at most `most` calls in `window` for the
/// whole computer, so no phone can run up your bill. Never retried: a tap is
/// one call at most. Runs on the main queue, as the deck does.
final class Summaries {
    static let keepFor: TimeInterval = 30
    static let most = 20
    static let window: TimeInterval = 10 * 60

    typealias Done = (Result<String, SummaryError>) -> Void
    /// Sends a request and hands back what came, on any queue. Swapped in tests.
    typealias Transport = (URLRequest, @escaping (Data?, Int?, Error?) -> Void) -> Void

    private var kept: [String: (text: String, at: Date)] = [:]
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
        if let hit = kept[id], now().timeIntervalSince(hit.at) < Summaries.keepFor { return done(.success(hit.text)) }
        if waiting[id] != nil { waiting[id]!.append(done); return }
        calls.removeAll { now().timeIntervalSince($0) >= Summaries.window }
        guard calls.count < Summaries.most else { return done(.failure(.tooMany)) }
        calls.append(now())
        waiting[id] = [done]
        let finish = { [weak self] (result: Result<String, SummaryError>) in
            DispatchQueue.main.async {
                guard let self else { return }
                if case let .success(text) = result { self.kept[id] = (text, self.now()) }
                let callers = self.waiting.removeValue(forKey: id) ?? []
                callers.forEach { $0(result) }
            }
        }
        // The file and the Keychain are read off the main queue, so the deck never waits on them.
        let key = key, transport = transport
        DispatchQueue.global(qos: .userInitiated).async {
            guard let extract = TranscriptTail.read(transcript) else { return finish(.failure(.noHistory)) }
            guard let secret = key() else { return finish(.failure(.noKey)) }
            let request = SummaryRequest.request(extract, project: project, key: secret)
            transport(request) { data, status, error in
                finish(SummaryRequest.answer(data, status: status, error: error))
            }
        }
    }

    /// Forgets every summary kept, as the setting goes off.
    func forget() { kept = [:] }
}
