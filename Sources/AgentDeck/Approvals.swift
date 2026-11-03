import Foundation

/// Who is waiting for you, and on what: every session in `asking`, as a list
/// the GM and the deck can read. The list only shows. A prompt is answered by
/// your tap alone, on a paired phone or the deck, with 1.23's `/act` and
/// `/choose`; nothing here, and no /setup/gm route, answers one.
///
/// Like the event log, the list is thin: the tool, and a short label (for Bash
/// only the command's first word, for a file tool the file's name). Never a
/// whole command, a file's path or what a question says.
enum Approvals {
    /// One session asking, as the deck knows it.
    struct Item: Equatable {
        var session: String
        /// The registry's id for the hire, or "" for a session started by hand.
        var hire = ""
        var name: String
        var nickname = ""
        var agent = ""
        var floor: Int?
        var desk: Int?
        /// "question" for an AskUserQuestion, "permission" for a tool's prompt.
        var kind = "permission"
        var tool = ""
        /// The tool's input as the session keeps it (JSON); only `label` is ever given out.
        var input = ""
        /// When it began asking, in ms; 0 if unknown.
        var since: Double = 0
        /// Whether a note that it waits on you was made for the paired phones.
        var notified = false
    }

    static let labelLimit = 80

    /// The tool and what it works on, in a few words: "Bash: git", "Edit: Deck.swift", or just the tool.
    static func label(tool: String, input: String) -> String {
        let object = (try? JSONSerialization.jsonObject(with: Data(input.utf8))) as? [String: Any] ?? [:]
        let what: String
        if tool == "Bash" {
            what = EventLog.firstWord(of: object["command"] as? String ?? "")
        } else {
            let path = EventLog.fileFields.lazy.compactMap { object[$0] as? String }.first { !$0.isEmpty } ?? ""
            what = (path as NSString).lastPathComponent
        }
        let line = what.isEmpty ? tool : tool.isEmpty ? what : "\(tool): \(what)"
        let one = line.split(whereSeparator: \.isNewline).joined(separator: " ")
        return one.count > labelLimit ? String(one.prefix(labelLimit - 1)) + "…" : one
    }

    /// What each item says, the longest waiting first, those whose time isn't known last.
    static func json(_ items: [Item], now: Double) -> [[String: Any]] {
        items.sorted { ($0.since > 0 ? 0 : 1, $0.since, $0.session) < ($1.since > 0 ? 0 : 1, $1.since, $1.session) }.map { item in
            [
                "session": item.session, "hire": item.hire, "name": item.name, "nickname": item.nickname,
                "agent": item.agent, "floor": item.floor.map { $0 as Any } ?? NSNull(), "desk": item.desk.map { $0 as Any } ?? NSNull(),
                "kind": item.kind, "tool": item.tool, "label": label(tool: item.tool, input: item.input),
                "since": item.since > 0 ? Int64(item.since) as Any : NSNull(),
                "waitingSeconds": item.since > 0 ? Int(max(0, now - item.since) / 1000) as Any : NSNull(),
                "notified": item.notified,
            ]
        }
    }

    /// The answer of `GET /setup/gm/approvals` and of the deck's `/approvals`.
    static func answer(_ items: [Item], now: Date = Date()) -> [String: Any] {
        let at = now.timeIntervalSince1970 * 1000
        return ["at": Int64(at), "approvals": json(items, now: at)]
    }
}

extension Deck {
    /// The sessions asking you now, with the hire the front desk knows each as and where it sits.
    func approvalItems() -> [Approvals.Item] {
        let asking = sessions.all.filter { $0.state == .asking }
        guard !asking.isEmpty else { return [] }
        let live = frontDeskLive()
        let hires = frontDesk.hires
        let phonesHavePush = access.phones.all.contains { $0.push != nil }
        return asking.map { session in
            let place = live.first { $0.id == session.id }
            let hire = place.flatMap { place in hires.first { $0.state != .fired && $0.state != .failed && FrontDesk.works(place, for: $0) } }
            let tool = session.asked.tool.isEmpty ? session.waitingFor : session.asked.tool
            let since = session.askedSince > 0 ? session.askedSince : session.lastEventAt
            let told = pushes.toldWaiting(session.id) != nil
            return Approvals.Item(session: session.id, hire: hire?.id ?? "", name: hire?.name ?? session.projectName,
                                  nickname: hire?.nickname ?? "", agent: session.agent.rawValue, floor: place?.floor, desk: place?.desk,
                                  kind: tool == "AskUserQuestion" ? "question" : "permission", tool: tool, input: session.asked.input,
                                  since: since, notified: phonesHavePush && told)
        }
    }
}
