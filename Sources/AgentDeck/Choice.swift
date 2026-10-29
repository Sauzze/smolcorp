import Foundation

/// iPhone app (personal build): what a Claude Code session asking you
/// something offers, for the phone to show as a button each: an
/// AskUserQuestion's options, or a permission prompt's Yes and No. The
/// phone's tap picks one by its number, as a digit typed at the menu does,
/// and only while that same question is still open (see Deck's "choose").
struct PendingChoice {
    enum Kind: String { case question, permission }
    struct Option: Equatable {
        let label: String
        let description: String
        /// A permission's No: the agent's reject key, never a digit, which on
        /// a menu other than the one guessed could be a Yes.
        var rejects = false
    }

    /// New for each question, so a tap meant for one never answers the next.
    let id: String
    let kind: Kind
    /// The question, or "Allow Bash?": a permission prompt's command or file
    /// is never in it, only in `detail`.
    let question: String
    let options: [Option]
    let multiSelect: Bool
    /// How many questions the menu asks in turn.
    let questions: Int
    /// What a permission prompt is about, the command or the file, cut short:
    /// given only through Tailscale, by GET /choice, never in the snapshot.
    let detail: String
    /// Where the menu's highlight is, 0 first, as ▲ and ▼ have moved it: for
    /// picking by the arrow keys, where digits don't work.
    var cursor = 0

    /// Whether the phone can pick an option with one tap. Several answers,
    /// or several questions, need Space and Tab, and a permission for a tool
    /// whose menu isn't known may list other options: those are answered
    /// with ▲ ▼ ✓ or on the Mac.
    var answerable: Bool { !multiSelect && questions == 1 && (1...9).contains(options.count) }

    /// The tools whose permission menu is Yes, maybe "Yes, and don't ask
    /// again", then No. Another tool's, as ExitPlanMode's plan approval, can
    /// list other options under the same numbers.
    static let knownMenus: Set = ["Bash", "Edit", "Write", "MultiEdit", "NotebookEdit", "WebFetch"]

    static func menuIsKnown(_ tool: String) -> Bool { knownMenus.contains(tool) || tool.hasPrefix("mcp__") }

    /// Whether the menu takes a typed answer: AskUserQuestion's own "Type
    /// something", the entry after its options, picked by its digit too.
    var typesOther: Bool { kind == .question && answerable && options.count < 9 }

    /// The longest question, label, description and detail sent, in characters.
    static let longest = (question: 300, label: 80, description: 160, detail: 600)

    /// AskUserQuestion's menu, from its tool call: its first question, with
    /// its options as the menu lists them. Nil if there's none to show.
    static func question(_ input: [String: Any], id: String = UUID().uuidString) -> PendingChoice? {
        let questions = input["questions"] as? [[String: Any]] ?? []
        guard let first = questions.first, let asked = first["question"] as? String else { return nil }
        let options = (first["options"] as? [[String: Any]] ?? []).compactMap { option -> Option? in
            guard let label = option["label"] as? String, !label.isEmpty else { return nil }
            return Option(label: cut(label, longest.label), description: cut(option["description"] as? String ?? "", longest.description))
        }
        guard !options.isEmpty else { return nil }
        return PendingChoice(id: id, kind: .question, question: cut(asked, longest.question), options: options,
                             multiSelect: first["multiSelect"] as? Bool ?? false, questions: questions.count, detail: "")
    }

    /// A permission prompt's menu: Yes, "Yes, and don't ask again" where
    /// Claude Code offers it (it does when it has rules to suggest), and No.
    /// Its own words for the second change with the tool and the version,
    /// so they're not copied, and it's pressed only once the menu on the
    /// screen shows it (see `screenCheck`). For a tool whose menu isn't
    /// known, the question alone, to answer on the Mac; none for
    /// ExitPlanMode, whose plan approval isn't a permission.
    static func permission(tool: String, input: [String: Any], suggests: Bool, id: String = UUID().uuidString) -> PendingChoice? {
        guard tool != "ExitPlanMode" else { return nil }
        let options = !menuIsKnown(tool) ? [] : [Option(label: "Yes", description: "")]
            + (suggests ? [Option(label: "Yes, and don't ask again", description: "As Claude Code offers it on the Mac")] : [])
            + [Option(label: "No", description: "Claude Code asks what to do instead", rejects: true)]
        return PendingChoice(id: id, kind: .permission, question: "Allow \(tool.isEmpty ? "this" : cut(tool, longest.label))?", options: options,
                             multiSelect: false, questions: 1, detail: detail(of: input))
    }

    /// The command, the file or the address a tool call is about, or its input in brief.
    static func detail(of input: [String: Any]) -> String {
        for key in ["command", "file_path", "notebook_path", "path", "url", "pattern"] {
            if let text = input[key] as? String, !text.isEmpty { return cut(text, longest.detail) }
        }
        guard !input.isEmpty, let data = try? JSONSerialization.data(withJSONObject: input, options: [.sortedKeys]) else { return "" }
        return cut(String(decoding: data, as: UTF8.self), longest.detail)
    }

    /// The keys that pick option `n`, 1 first: its digit, or the arrows from
    /// where the highlight is, then Return; a permission's No, the agent's
    /// reject key, whichever.
    func keys(for n: Int, byArrows: Bool, reject: Terminal.Keystroke) -> [Terminal.Keystroke] {
        if options.indices.contains(n - 1), options[n - 1].rejects { return [reject] }
        guard byArrows else { return Terminal.Key.digit(n).map { [.init(key: $0)] } ?? [] }
        let moves = (n - 1) - cursor
        let arrows: [Terminal.Keystroke] = moves == 0 ? [] : [.init(key: moves > 0 ? .down : .up, times: abs(moves))]
        return arrows + [.init(key: .returnKey)]
    }

    /// What the menu on the screen must show before option `n`'s key goes:
    /// a line "n. <label>". So the menu is still there as it was asked, not
    /// answered on the Mac's keyboard meanwhile, and a guessed list never
    /// presses a Yes: a permission's Yes and "Yes, and don't ask again",
    /// which adds a rule for good, at their number; its No, the reject key,
    /// at whichever number it has; a question's option by the start of its
    /// label; "Type something", as Claude Code may name it. Nil for no
    /// such option.
    func screenCheck(for n: Int) -> (number: Int?, labels: [String])? {
        if typesOther && n == options.count + 1 { return (n, ["Type something", "Other"]) }
        guard options.indices.contains(n - 1) else { return nil }
        let option = options[n - 1]
        switch kind {
        case .permission: return option.rejects ? (nil, ["No"]) : (n, ["Yes"])
        case .question: return (n, [String(option.label.prefix(PendingChoice.labelSeen))])
        }
    }

    /// As much of a label as is looked for on the screen: a long one may be cut there.
    static let labelSeen = 16
    /// The screen's last lines, where the menu is.
    static let menuLines = 30

    /// Whether one of `screen`'s last lines is the menu's option `n` (any
    /// number, for nil), starting with one of `labels`: "❯ 2. Yes, and don't
    /// ask again for…", or without the ❯.
    static func screen(_ screen: String, shows n: Int?, startingWith labels: [String]) -> Bool {
        lastLines(of: screen).contains { line in
            guard let (number, label) = option(line), n == nil || number == n else { return false }
            return labels.contains { !$0.isEmpty && label.hasPrefix($0) }
        }
    }

    /// Whether the menu is still there after its key went, unanswered: the
    /// option still at its number, and the menu's lines, from its first
    /// option to its last, as they were. A key that reached nothing, as on
    /// a locked Mac, leaves them so; an answer takes them away, or brings
    /// another menu.
    static func stillOpen(before: String, after: String, shows n: Int?, startingWith labels: [String]) -> Bool {
        screen(after, shows: n, startingWith: labels) && menu(on: after) == menu(on: before)
    }

    /// The screen's last lines, from the first that's an option to the last.
    static func menu(on screen: String) -> [Substring] {
        let lines = Array(lastLines(of: screen))
        guard let first = lines.firstIndex(where: { option($0) != nil }),
              let last = lines.lastIndex(where: { option($0) != nil }) else { return [] }
        return Array(lines[first...last])
    }

    private static func lastLines(of screen: String) -> ArraySlice<Substring> {
        screen.split(omittingEmptySubsequences: false, whereSeparator: \.isNewline).suffix(menuLines)
    }

    /// A line's option number and label: "❯ 2. Yes, and don't ask again", or without the ❯.
    private static func option(_ line: Substring) -> (Int, Substring)? {
        let text = line.drop { $0 == " " || $0 == "│" || $0 == "❯" || $0 == ">" }
        guard let dot = text.firstIndex(of: "."), let number = Int(text[..<dot]) else { return nil }
        let rest = text[text.index(after: dot)...]
        return rest.hasPrefix(" ") ? (number, rest.dropFirst()) : nil
    }

    /// The highlight moved by ▲ (-1) or ▼ (+1), kept within the menu, as
    /// for a menu that doesn't wrap round: to be checked on the Mac before
    /// the arrows' way is turned on.
    mutating func move(_ by: Int) {
        cursor = max(0, min(options.count - 1, cursor + by))
    }

    /// What the phone is told, in the snapshot: never `detail`. Where the
    /// terminal can't show its menu, nothing is answered from the phone
    /// (see `screenCheck`): the options are there to read.
    func forPhone(menuSeen: Bool) -> [String: Any] {
        ["id": id, "kind": kind.rawValue, "question": question, "multiSelect": multiSelect, "questions": questions,
         "answerable": answerable && menuSeen, "other": typesOther && menuSeen, "options": options.map { ["label": $0.label, "description": $0.description] }]
    }

    private static func cut(_ text: String, _ length: Int) -> String {
        let line = text.trimmingCharacters(in: .whitespacesAndNewlines)
        return line.count <= length ? line : String(line.prefix(length - 1)) + "…"
    }
}
