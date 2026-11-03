import Foundation

/// "Ready for review": a hire wrote a NEEDS-REVIEW line in its STATUS.md and the app's own watch
/// (T406) logged it. The deck lists the recent ones at `GET /reviews` for the paired phone, and each
/// new one sends a push that says only who is ready, never the line.
///
/// The event log line is the watch's: `agent` is `needs-review`, and the detail reads
/// "needs-review: <hire>: <file>: <the line, NEEDS-REVIEW: first>". Lines that don't have that shape
/// are left out.
enum Reviews {
    /// What the watch puts in a line's `agent`.
    static let mark = "needs-review"
    /// How many the list holds, the newest.
    static let most = 20
    /// The longest a line is given out.
    static let textLimit = 200

    struct Item: Equatable {
        var id: Int
        var at: Double
        var session: String
        var hire: String
        var file: String
        var text: String
    }

    /// The review in `entry`, or nil when it isn't a NEEDS-REVIEW firing.
    static func item(_ entry: EventLog.Entry) -> Item? {
        guard entry.agent == mark else { return nil }
        var detail = Substring(entry.detail)
        // The watch's own name comes first in what it logs.
        if detail.hasPrefix(mark + ": ") { detail = detail.dropFirst(mark.count + 2) }
        // The file has no colon and the line begins with NEEDS-REVIEW:, so a colon in the hire's name (or the line) can't shift the parts.
        let text = String(detail)
        let shape = #"^(.+?): ([^:\n]+): ((?:[-*]\s+)?NEEDS-REVIEW:[\s\S]*)$"#
        guard let match = text.range(of: shape, options: .regularExpression),
              let groups = try? NSRegularExpression(pattern: shape).firstMatch(in: text, range: NSRange(match, in: text)),
              groups.numberOfRanges == 4,
              let hireRange = Range(groups.range(at: 1), in: text), let fileRange = Range(groups.range(at: 2), in: text),
              let lineRange = Range(groups.range(at: 3), in: text) else { return nil }
        let hire = text[hireRange].trimmingCharacters(in: .whitespaces)
        guard !hire.isEmpty else { return nil }
        return Item(id: entry.id, at: entry.at, session: entry.session, hire: hire, file: text[fileRange].trimmingCharacters(in: .whitespaces),
                    text: words(of: String(text[lineRange])))
    }

    /// What follows "NEEDS-REVIEW:" on the line, on one line, cut to `textLimit`.
    static func words(of line: String) -> String {
        var text = line.trimmingCharacters(in: .whitespaces)
        if let range = text.range(of: #"^(?:[-*]\s+)?NEEDS-REVIEW:\s*"#, options: .regularExpression) { text.removeSubrange(range) }
        let one = text.split(whereSeparator: \.isNewline).joined(separator: " ")
        return one.count > textLimit ? String(one.prefix(textLimit - 1)) + "…" : one
    }

    /// The newest reviews in `entries`, the latest last.
    static func recent(_ entries: [EventLog.Entry]) -> [Item] {
        Array(entries.compactMap(item).suffix(most))
    }

    /// The answer of `GET /reviews`. Each review also carries `detail` ("<hire>: <file>: <text>") as the
    /// phone reads it, with the mark in `agent`.
    static func answer(_ items: [Item], now: Date = Date()) -> [String: Any] {
        ["at": Int64(now.timeIntervalSince1970 * 1000), "reviews": items.map { item in
            ["id": item.id, "at": Int64(item.at.rounded(.down)), "session": item.session, "agent": mark, "hire": item.hire, "file": item.file,
             "text": item.text, "detail": "\(item.hire): \(item.file): \(item.text)"] as [String: Any]
        }]
    }
}

extension PushNote {
    /// A hire is ready for review: its name and nothing of the line.
    static func review(_ hire: String) -> PushNote {
        PushNote(event: .review, session: nil, title: "Smolcorp",
                 body: TranscriptTail.clip("\(hire.isEmpty ? "A hire" : hire) is ready for review", bodyMost))
    }
}

extension Deck {
    /// The newest reviews the log holds.
    func reviewItems() -> [Reviews.Item] {
        Reviews.recent(events.entries(since: max(0, events.lastID - EventLog.recentMost), limit: EventLog.recentMost).entries)
    }

    /// Called for each line added to the event log: a new review tells the paired phones.
    func noteReview(_ entry: EventLog.Entry) {
        guard let item = Reviews.item(entry) else { return }
        push(.review(item.hire))
    }
}
