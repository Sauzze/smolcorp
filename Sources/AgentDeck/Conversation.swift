import Foundation

/// iPhone app (personal build): an agent's conversation, for the phone's
/// blackboard, read from the same Claude Code transcript ✨ Summary reads:
/// what the user wrote, what the agent answered, each tool it ran in a line,
/// a result's first line, and the questions it asked with their answers.
///
/// Read from the end, a page at a time. Each message is cut to `textMost`
/// and every line of it goes through Summary's redaction before it leaves,
/// and nothing of a tool's input or output beyond what Summary takes. Given
/// only through `tailscale serve`, to a paired phone (see DeckRouter).
enum Conversation {
    /// Messages in a page, at most.
    static let pageMost = 60
    /// Each message's text, at most, before "… more".
    static let textMost = 2_000
    /// How much of the file one page looks through, at most: tool results
    /// are kept whole in a transcript, and can be large.
    static let reach: UInt64 = 4 * 1024 * 1024
    /// A line longer than this is a tool's whole output: skipped, not read.
    static let lineMost = 2 * 1024 * 1024
    /// How much is read at a time, going back.
    static let chunk: UInt64 = 256 * 1024

    struct Item: Equatable {
        enum Kind: String { case user, peer, agent, tool, result, question, answer, note }
        var kind: Kind
        var text: String
        /// Cut at `textMost`.
        var more = false
        /// Where its line starts in the file, and which of the line's items it is: its id, and its order.
        var offset: UInt64 = 0
        var index = 0
        /// When it was written, in ms, if the transcript says.
        var at: Int64?
        /// Who sent a peer's message, or the tool a line is about.
        var from = ""

        var jsonObject: [String: Any] {
            var object: [String: Any] = ["id": "\(offset).\(index)", "offset": offset, "n": index, "kind": kind.rawValue,
                                         "text": text, "more": more]
            if let at { object["at"] = at }
            if !from.isEmpty { object["from"] = from }
            return object
        }
    }

    struct Page: Equatable {
        /// Oldest first.
        var items: [Item]
        /// Where the next older page ends: nil at the start of the file.
        var before: UInt64?
        /// The file's size as it was read, to tell whether it has grown.
        var size: UInt64

        var jsonObject: [String: Any] {
            ["items": items.map(\.jsonObject), "before": before.map { $0 as Any } ?? NSNull(), "size": size]
        }
    }

    /// The page of the transcript at `path` that ends where `before` is (a
    /// line's start, from an earlier page), or at the end. Nil with no file.
    /// Not `redacted` only for the Mac's own office window: see DeckRouter.
    static func page(_ path: String, before: UInt64? = nil, most: Int = pageMost, redacted: Bool = true) -> Page? {
        guard !path.isEmpty, let file = FileHandle(forReadingAtPath: path) else { return nil }
        defer { try? file.close() }
        let size = (try? file.seekToEnd()) ?? 0
        let end = min(before ?? size, size)
        let stop = end > reach ? end - reach : 0

        // Back from `end`, a chunk at a time, whole lines only, newest first, till the page is full or `reach` is spent.
        var lines: [(offset: UInt64, entry: [String: Any], count: Int)] = []
        var counted = 0
        var position = end          // bytes from here to `end` have been looked at
        var pending = Data()        // the line `position` is in, as far as it's been read
        var tooLong = false         // that line is longer than `lineMost`: skipped
        var earliest = end          // where the earliest line looked at starts
        var none: [String: String] = [:]
        func take(_ bytes: Data, at offset: UInt64) {
            earliest = offset
            guard !bytes.isEmpty, let entry = try? JSONSerialization.jsonObject(with: bytes) as? [String: Any] else { return }
            let count = Conversation.items(entry, offset: offset, uses: &none, redacted: redacted).count
            guard count > 0 else { return }
            lines.append((offset, entry, count))
            counted += count
        }
        scan: while position > 0 {
            let from = position > chunk ? position - chunk : 0
            guard (try? file.seek(toOffset: from)) != nil, var data = try? file.read(upToCount: Int(position - from)) else { break }
            while let newline = data.lastIndex(of: 0x0A) {
                let start = from + UInt64(newline - data.startIndex) + 1
                let length = data.endIndex - newline - 1 + pending.count
                if !tooLong, length <= lineMost { take(Data(data[(newline + 1)...]) + pending, at: start) } else { earliest = start }
                pending = Data()
                tooLong = false
                data = data[..<newline]
                if counted >= most || start <= stop { break scan }
            }
            if !tooLong {
                pending = Data(data) + pending
                if pending.count > lineMost { tooLong = true; pending = Data() }
            }
            position = from
            if from == 0 {
                if !tooLong { take(pending, at: 0) } else { earliest = 0 }
            }
        }

        // Oldest first, and no more than `most`: the oldest lines go first, but never the only one.
        lines.reverse()
        var start = earliest
        while counted > most, lines.count > 1 {
            counted -= lines.removeFirst().count
            start = lines[0].offset
        }
        var uses: [String: String] = [:] // a tool use's id, its tool: for a question's answer
        let items = lines.flatMap { Conversation.items($0.entry, offset: $0.offset, uses: &uses, redacted: redacted) }
        return Page(items: items, before: start == 0 ? nil : start, size: size)
    }

    /// One transcript line's items, oldest first, each already redacted and cut.
    static func items(_ entry: [String: Any], offset: UInt64, uses: inout [String: String], redacted: Bool = true) -> [Item] {
        guard entry["isSidechain"] as? Bool != true else { return [] }
        let at = (entry["timestamp"] as? String).flatMap(Conversation.time)
        var items: [Item] = []
        func add(_ kind: Item.Kind, _ text: String, from: String = "") {
            let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !trimmed.isEmpty else { return }
            items.append(Conversation.item(kind, trimmed, offset: offset, index: items.count, at: at, from: from, redacted: redacted))
        }

        // What the user wrote, or another session sent: Summary's own reading of it.
        if let message = TranscriptTail.message(entry) {
            switch message.kind {
            case .user: add(.user, message.text)
            case .peer: add(.peer, message.text, from: message.from)
            case .report: add(.peer, message.text, from: message.from)
            }
            return items
        }
        let kind = entry["type"] as? String
        guard kind == "user" || kind == "assistant", let message = entry["message"] as? [String: Any] else { return [] }
        if let text = message["content"] as? String {
            if kind == "assistant" { add(.agent, text) } else if let note = note(text) { add(.note, note) }
            return items
        }
        for block in message["content"] as? [[String: Any]] ?? [] {
            switch block["type"] as? String {
            case "text":
                let text = block["text"] as? String ?? ""
                if kind == "assistant" { add(.agent, text) } else if let note = note(text) { add(.note, note) }
            case "tool_use":
                let name = block["name"] as? String ?? "tool"
                if let id = block["id"] as? String { uses[id] = name }
                if name == "AskUserQuestion", let asked = question(block["input"] as? [String: Any] ?? [:]) {
                    add(.question, asked)
                } else {
                    // Summary's line for it: the tool, and its path, command or search, never what it was given to write.
                    add(.tool, TranscriptTail.tool(block), from: name)
                }
            case "tool_result":
                let first = TranscriptTail.firstLine(TranscriptTail.resultText(block["content"]))
                let tool = (block["tool_use_id"] as? String).flatMap { uses[$0] } ?? ""
                // Redacted whole before any cut, as Summary does: a cut could drop the words that give a secret away.
                if tool == "AskUserQuestion" {
                    add(.answer, answer(redacted ? Redact.text(first) : first))
                } else {
                    let error = block["is_error"] as? Bool == true
                    add(.result, (error ? "Error: " : "") + TranscriptTail.clip(first.isEmpty ? "(nothing)" : redacted ? Redact.text(first) : first,
                                                                                TranscriptTail.resultMost),
                        from: tool)
                }
            default: // thinking, images, and anything new
                break
            }
        }
        return items
    }

    /// An item, redacted line by line before it's cut, and again after; or, not `redacted`, only cut.
    static func item(_ kind: Item.Kind, _ text: String, offset: UInt64, index: Int, at: Int64?, from: String,
                     redacted: Bool = true) -> Item {
        guard redacted else {
            let cut = text.count > textMost
            return Item(kind: kind, text: cut ? String(text.prefix(textMost)) : text, more: cut, offset: offset, index: index,
                        at: at, from: from.isEmpty ? "" : TranscriptTail.clip(TranscriptTail.firstLine(from), 60))
        }
        let redacted = Redact.text(text)
        let cut = redacted.count > textMost
        let shown = Redact.text(cut ? String(redacted.prefix(textMost)) : redacted)
        let name = from.isEmpty ? "" : TranscriptTail.clip(Redact.text(TranscriptTail.firstLine(from)), 60)
        return Item(kind: kind, text: shown, more: cut, offset: offset, index: index, at: at, from: name == Redact.placeholder ? "" : name)
    }

    /// What's left in the user's turn that isn't a message: a command run, or an interruption.
    private static func note(_ text: String) -> String? {
        let text = text.trimmingCharacters(in: .whitespacesAndNewlines)
        if text.hasPrefix("<command-name>"), let open = text.range(of: "<command-name>"),
           let close = text.range(of: "</command-name>"), open.upperBound <= close.lowerBound {
            let name = String(text[open.upperBound..<close.lowerBound])
            return name.isEmpty ? nil : "ran " + TranscriptTail.clip(Redact.text(name), 60)
        }
        if text.hasPrefix("[Request interrupted") { return "interrupted" }
        return nil
    }

    /// AskUserQuestion's questions, each with its options' labels.
    private static func question(_ input: [String: Any]) -> String? {
        let questions = (input["questions"] as? [[String: Any]] ?? []).compactMap { asked -> String? in
            guard let text = asked["question"] as? String, !text.isEmpty else { return nil }
            let options = (asked["options"] as? [[String: Any]] ?? []).compactMap { $0["label"] as? String }
            return options.isEmpty ? text : text + "\n" + options.enumerated().map { "\($0.offset + 1). \($0.element)" }.joined(separator: "\n")
        }
        return questions.isEmpty ? nil : questions.joined(separator: "\n\n")
    }

    /// A question's answer, from its result's first line, already redacted, without Claude Code's words round it.
    private static func answer(_ first: String) -> String {
        var text = first
        for prefix in ["User has answered your questions:", "User has answered your question:"] where text.hasPrefix(prefix) {
            text = String(text.dropFirst(prefix.count))
        }
        if let range = text.range(of: ". You can now continue") { text = String(text[..<range.lowerBound]) }
        return TranscriptTail.clip(text.trimmingCharacters(in: .whitespaces), TranscriptTail.textMost)
    }

    private static func time(_ text: String) -> Int64? {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let date = formatter.date(from: text) ?? ISO8601DateFormatter().date(from: text)
        return date.map { Int64($0.timeIntervalSince1970 * 1000) }
    }
}
