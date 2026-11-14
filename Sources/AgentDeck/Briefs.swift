import Foundation

/// Where a role's brief comes from: its own instructions, as roles have
/// always started (`text`); one of the briefs that come with the app
/// (`library`, by its id); or a Markdown file you picked in setup, copied in
/// whole (`file`, with the file's name kept to show). A library or file
/// brief is filled in on hire and written into the hire's folder as
/// BRIEF.md, for the agent to read first.
struct Brief: Codable, Equatable {
    enum Kind: String, Codable {
        case text, library, file
    }

    var kind: Kind
    var id = ""
    var text = ""
    var fileName = ""

    static func library(_ id: String) -> Brief { Brief(kind: .library, id: id) }
    static func text(_ text: String) -> Brief { Brief(kind: .text, text: text) }

    /// The briefs that come with the app, in the order setup lists them: the
    /// shipped roles' own, by their ids, then those for roles of your own.
    static let libraryIDs = ["reviewer", "tester", "bughunter", "docs", "refactorer", "artdirector", "lawyer", "therapist", "plumber",
                             "gm", "janitor", "firefighter", "pilot", "postman", "accountant", "marketer", "chef", "thinker",
                             "builder", "designer", "minimanager", "security-auditor", "perf-auditor", "pixel-artist", "planner", "fixer", "study",
                             "issue-sorter", "test-writer", "dependency-updater", "translator", "api-checker", "support-writer"]
    /// A shipped role whose brief has an id of its own: the Security guard's is the security auditor's.
    static let libraryOfRole = ["securityguard": "security-auditor"]

    /// The library brief a shipped role comes with, if any.
    static func libraryID(ofRole id: String) -> String? {
        libraryOfRole[id] ?? (libraryIDs.contains(id) ? id : nil)
    }
    /// The most a picked file can hold, in bytes, and how long its name can be.
    static let fileLimit = 32 * 1024
    static let fileNameLimit = 100
    static let fileExtensions = ["md", "markdown", "txt"]

    private enum CodingKeys: String, CodingKey {
        case kind, id, text, fileName
    }

    /// An unknown kind isn't a brief; anything else missing or of the wrong
    /// kind is empty, and `Role.checked` decides what's kept.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        func text(_ key: CodingKeys) -> String { ((try? c.decodeIfPresent(String.self, forKey: key)) ?? nil) ?? "" }
        kind = try c.decode(Kind.self, forKey: .kind)
        (id, self.text, fileName) = (text(.id), text(.text), text(.fileName))
    }

    init(kind: Kind, id: String = "", text: String = "", fileName: String = "") {
        (self.kind, self.id, self.text, self.fileName) = (kind, id, text, fileName)
    }

    /// Only what its kind has.
    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(kind, forKey: .kind)
        switch kind {
        case .library: try c.encode(id, forKey: .id)
        case .text: try c.encode(text, forKey: .text)
        case .file:
            try c.encode(text, forKey: .text)
            try c.encode(fileName, forKey: .fileName)
        }
    }

    var jsonObject: [String: Any] {
        switch kind {
        case .library: return ["kind": kind.rawValue, "id": id]
        case .text: return ["kind": kind.rawValue, "text": text]
        case .file: return ["kind": kind.rawValue, "text": text, "fileName": fileName]
        }
    }

    /// Why a picked file can't be a brief, in words setup shows as they are.
    enum Problem: LocalizedError, Equatable {
        case empty, tooLong, notText, fileName, slots, sections, unknown([String])

        var errorDescription: String? {
            switch self {
            case .empty: return "The brief is empty."
            case .tooLong: return "The brief is over \(Brief.fileLimit / 1024) KB. Pick a shorter file."
            case .notText: return "That file isn't text. Pick a Markdown or text file."
            case .fileName: return "Pick a .md, .markdown or .txt file."
            case .slots: return "{{> …}} works only in the briefs that come with the app."
            case .sections: return "A {{#…}}, {{^…}} or {{/…}} in the brief has no partner."
            case .unknown(let names):
                return "The brief has placeholders Smolcorp can't fill: " + names.map { "{{\($0)}}" }.joined(separator: ", ")
                    + ". It can fill " + BriefTemplate.names.sorted().map { "{{\($0)}}" }.joined(separator: ", ") + "."
            }
        }
    }

    /// A picked file as kept: its text without a byte-order mark, with plain
    /// line ends and no control characters but tabs, and its name without a
    /// folder. Its placeholders and sections must be ones a hire can fill.
    static func file(_ text: String, named name: String) throws -> Brief {
        guard !text.unicodeScalars.contains("\0"), !text.contains("\u{FFFD}") else { throw Problem.notText }
        var text = text.hasPrefix("\u{FEFF}") ? String(text.dropFirst()) : text
        text = text.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\r", with: "\n")
        text = String(String.UnicodeScalarView(text.unicodeScalars.filter {
            $0 == "\n" || $0 == "\t" || !CharacterSet.controlCharacters.contains($0)
        }))
        guard !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { throw Problem.empty }
        guard text.utf8.count <= fileLimit else { throw Problem.tooLong }
        let base = String(name.split(whereSeparator: { $0 == "/" || $0 == "\\" }).last ?? "")
        let fileName = String(String.UnicodeScalarView(base.unicodeScalars.filter { !CharacterSet.controlCharacters.contains($0) }))
            .trimmingCharacters(in: .whitespaces)
        guard (1...fileNameLimit).contains(fileName.count),
              fileExtensions.contains((fileName as NSString).pathExtension.lowercased()) else { throw Problem.fileName }
        try BriefTemplate.check(text)
        return Brief(kind: .file, text: text, fileName: fileName)
    }
}

/// The few rules a brief is written in: `{{name}}` for a value,
/// `{{#name}}…{{/name}}` for text kept only when the value is set (not empty,
/// false, no or 0) and `{{^name}}…{{/name}}` only when it isn't, `<!-- … -->`
/// for notes to whoever edits it, and front matter between `---` lines on top.
/// A section tag alone on its line takes the line with it. Only checked here,
/// for a brief you picked: the briefs themselves are filled in by the core.
enum BriefTemplate {
    /// Every value a hire fills, besides the front matter's own keys.
    static let names: Set<String> = ["role", "date", "project_folder", "branch", "agent", "model", "effort", "job", "approval",
                                     "status_file", "brief_file", "report_file", "done_signal", "read_only"]

    struct FrontMatter {
        var values: [String: String] = [:]
        var body: String
    }

    /// The front matter, `key: value` lines (`question` lines aside, of which there can be several), and the text after it.
    static func frontMatter(_ text: String) -> FrontMatter {
        guard text.hasPrefix("---\n"), let end = text.range(of: "\n---\n", range: text.index(text.startIndex, offsetBy: 3)..<text.endIndex)
        else { return FrontMatter(body: text) }
        var parsed = FrontMatter(body: String(text[end.upperBound...]))
        let start = text.index(text.startIndex, offsetBy: 4)
        for line in (start < end.lowerBound ? text[start..<end.lowerBound] : "").split(separator: "\n") {
            guard let colon = line.firstIndex(of: ":") else { continue }
            let key = line[..<colon].trimmingCharacters(in: .whitespaces)
            let value = line[line.index(after: colon)...].trimmingCharacters(in: .whitespaces)
            guard key.range(of: "^[A-Za-z0-9_]+$", options: .regularExpression) != nil else { continue }
            if key != "question" { parsed.values[key] = value }
        }
        return parsed
    }

    static func isSet(_ value: String?) -> Bool {
        guard let value = value?.trimmingCharacters(in: .whitespacesAndNewlines), !value.isEmpty else { return false }
        return !["false", "no", "0"].contains(value.lowercased())
    }

    private static func regex(_ pattern: String) -> NSRegularExpression {
        // Only this file's own patterns.
        try! NSRegularExpression(pattern: pattern)
    }
    private static let comment = regex(#"(?m)^[ \t]*<!--[\s\S]*?-->[ \t]*\n|<!--[\s\S]*?-->"#)
    private static let standaloneTag = regex(#"(?m)^[ \t]*(\{\{[#^/][A-Za-z0-9_]+\}\})[ \t]*\n"#)
    private static let section = regex(#"\{\{([#^])([A-Za-z0-9_]+)\}\}((?:(?!\{\{[#^])[\s\S])*?)\{\{/\2\}\}"#)
    private static let value = regex(#"\{\{([A-Za-z0-9_]+)\}\}"#)
    private static let sectionName = regex(#"\{\{[#^/]([A-Za-z0-9_]+)\}\}"#)

    private static func replacing(_ pattern: NSRegularExpression, in text: String, with make: ([String]) -> String) -> String {
        let ns = text as NSString
        var result = ""
        var last = 0
        for match in pattern.matches(in: text, range: NSRange(location: 0, length: ns.length)) {
            result += ns.substring(with: NSRange(location: last, length: match.range.location - last))
            result += make((0..<match.numberOfRanges).map { i in
                let range = match.range(at: i)
                return range.location == NSNotFound ? "" : ns.substring(with: range)
            })
            last = match.range.location + match.range.length
        }
        return result + ns.substring(from: last)
    }

    /// Whether `body` (front matter and all) is a brief a hire can fill:
    /// only known values, sections that pair up, and no slots.
    static func check(_ text: String) throws {
        let parsed = frontMatter(text)
        let known = names.union(parsed.values.keys)
        _ = try resolve(parsed.body, known: known, values: [:])
    }

    private static func resolve(_ body: String, known: Set<String>, values: [String: String]) throws -> String {
        var text = replacing(comment, in: body) { _ in "" }
        if text.contains("{{>") { throw Brief.Problem.slots }
        let unknown = Set((sectionName.matches(in: text, range: NSRange(text.startIndex..., in: text)).map { (text as NSString).substring(with: $0.range(at: 1)) }
            + value.matches(in: text, range: NSRange(text.startIndex..., in: text)).map { (text as NSString).substring(with: $0.range(at: 1)) })
            .filter { !known.contains($0) })
        if !unknown.isEmpty { throw Brief.Problem.unknown(unknown.sorted()) }
        text = replacing(standaloneTag, in: text) { $0[1] }
        // Innermost first, until none is left.
        while section.firstMatch(in: text, range: NSRange(text.startIndex..., in: text)) != nil {
            text = replacing(section, in: text) { groups in
                (groups[1] == "#") == isSet(values[groups[2]]) ? groups[3] : ""
            }
        }
        // Checked before the values go in, which may hold braces of their own.
        if replacing(value, in: text, with: { _ in "" }).contains("{{") { throw Brief.Problem.sections }
        text = replacing(value, in: text) { values[$0[1]] ?? "" }
        text = text.replacingOccurrences(of: #"[ \t]+\n"#, with: "\n", options: .regularExpression)
            .replacingOccurrences(of: #"\n{3,}"#, with: "\n\n", options: .regularExpression)
        return text.trimmingCharacters(in: .whitespacesAndNewlines) + "\n"
    }
}
