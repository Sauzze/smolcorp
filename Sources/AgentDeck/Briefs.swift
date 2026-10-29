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
                             "builder", "security-auditor", "perf-auditor", "designer", "pixel-artist", "planner", "fixer", "study"]
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

    /// The branch checked out where `folder` is, or nil outside git or with
    /// none checked out. Read from git's own files, without running git.
    static func branch(in folder: String) -> String? {
        let manager = FileManager.default
        var dir = URL(fileURLWithPath: folder).standardizedFileURL
        while true {
            let dotGit = dir.appendingPathComponent(".git")
            var isFolder: ObjCBool = false
            if manager.fileExists(atPath: dotGit.path, isDirectory: &isFolder) {
                var gitDir = dotGit
                // A worktree's .git is a file naming its folder.
                if !isFolder.boolValue {
                    guard let line = try? String(contentsOf: dotGit, encoding: .utf8), line.hasPrefix("gitdir:") else { return nil }
                    let path = line.dropFirst("gitdir:".count).trimmingCharacters(in: .whitespacesAndNewlines)
                    gitDir = path.hasPrefix("/") ? URL(fileURLWithPath: path) : dir.appendingPathComponent(path)
                }
                guard let head = try? String(contentsOf: gitDir.appendingPathComponent("HEAD"), encoding: .utf8) else { return nil }
                let ref = head.trimmingCharacters(in: .whitespacesAndNewlines)
                guard ref.hasPrefix("ref: refs/heads/") else { return nil }
                let name = String(ref.dropFirst("ref: refs/heads/".count))
                return name.isEmpty || name.unicodeScalars.contains(where: { CharacterSet.controlCharacters.contains($0) }) ? nil : name
            }
            let parent = dir.deletingLastPathComponent()
            if parent.path == dir.path { return nil }
            dir = parent
        }
    }
}

/// The few rules a brief is written in: `{{name}}` for a value,
/// `{{#name}}…{{/name}}` for text kept only when the value is set (not empty,
/// false, no or 0) and `{{^name}}…{{/name}}` only when it isn't, `<!-- … -->`
/// for notes to whoever edits it, and front matter between `---` lines on top.
/// A section tag alone on its line takes the line with it. The library's
/// skeleton also has `{{> slot}}`, for each brief's own sections.
enum BriefTemplate {
    /// Every value a hire fills, besides the front matter's own keys.
    static let names: Set<String> = ["role", "date", "project_folder", "branch", "agent", "model", "effort", "job", "approval",
                                     "status_file", "brief_file", "report_file", "done_signal", "read_only"]
    /// Front matter keys that aren't values, and `question`, of which there can be several.
    static let metaKeys: Set<String> = ["name", "summary", "question", "report"]

    struct FrontMatter {
        var values: [String: String] = [:]
        var questions: [String] = []
        /// The front matter as written, with its `---` lines, or empty.
        var head = ""
        var body: String
    }

    /// The front matter, `key: value` lines, and the text after it.
    static func frontMatter(_ text: String) -> FrontMatter {
        guard text.hasPrefix("---\n"), let end = text.range(of: "\n---\n", range: text.index(text.startIndex, offsetBy: 3)..<text.endIndex)
        else { return FrontMatter(body: text) }
        var parsed = FrontMatter(head: String(text[..<end.upperBound]), body: String(text[end.upperBound...]))
        let start = text.index(text.startIndex, offsetBy: 4)
        for line in (start < end.lowerBound ? text[start..<end.lowerBound] : "").split(separator: "\n") {
            guard let colon = line.firstIndex(of: ":") else { continue }
            let key = line[..<colon].trimmingCharacters(in: .whitespaces)
            let value = line[line.index(after: colon)...].trimmingCharacters(in: .whitespaces)
            guard key.range(of: "^[A-Za-z0-9_]+$", options: .regularExpression) != nil else { continue }
            if key == "question" {
                if !value.isEmpty { parsed.questions.append(value) }
            } else {
                parsed.values[key] = value
            }
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

    /// `body` with its sections resolved and its values filled: each value
    /// as it is, never read again for tags of its own. Refused, saying why,
    /// if a tag would be left in it.
    static func render(_ body: String, values: [String: String], known: Set<String>) throws -> String {
        try resolve(body, known: known, values: values)
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

/// The briefs that come with the app: `_base.md`, the skeleton they share,
/// and each brief's own front matter and sections, poured into it.
struct BriefLibrary {
    let folder: URL

    static let bundled = BriefLibrary(folder: Paths.briefs)

    struct Entry {
        let id, name, summary: String
        let questions: [String]

        var jsonObject: [String: Any] { ["id": id, "name": name, "summary": summary, "questions": questions] }
    }

    /// Those that are there, in `Brief.libraryIDs`'s order.
    func entries() -> [Entry] {
        Brief.libraryIDs.compactMap { id in
            guard let text = source(id) else { return nil }
            let parsed = BriefTemplate.frontMatter(text)
            return Entry(id: id, name: parsed.values["name"] ?? id, summary: parsed.values["summary"] ?? "", questions: parsed.questions)
        }
    }

    private func source(_ id: String) -> String? {
        guard Brief.libraryIDs.contains(id) else { return nil }
        return try? String(contentsOf: folder.appendingPathComponent("\(id).md"), encoding: .utf8)
    }

    /// The whole brief, as a picked file would be: its front matter, then
    /// the skeleton with its sections in, placeholders left to fill. Nil for
    /// an id that isn't one of the library's.
    func template(_ id: String) -> String? {
        guard let text = source(id), let base = try? String(contentsOf: folder.appendingPathComponent("_base.md"), encoding: .utf8)
        else { return nil }
        let parsed = BriefTemplate.frontMatter(text)
        // Its sections, each from its `<!-- slot: name -->` line to the next.
        var slots: [String: String] = [:]
        var name: String?
        var lines: [Substring] = []
        func keep() { if let name { slots[name] = lines.joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines) } }
        for line in parsed.body.split(separator: "\n", omittingEmptySubsequences: false) {
            if let found = line.wholeMatch(of: #/<!-- slot: ([a-z_]+) -->/#) {
                keep()
                (name, lines) = (String(found.1), [])
            } else {
                lines.append(line)
            }
        }
        keep()
        let poured = base.split(separator: "\n", omittingEmptySubsequences: false).compactMap { line -> String? in
            guard let found = line.wholeMatch(of: #/\{\{> ([a-z_]+)\}\}/#) else {
                // A slot inside a line.
                return String(line).replacing(#/\{\{> ([a-z_]+)\}\}/#) { slots[String($0.1)] ?? "" }
            }
            let slot = slots[String(found.1)] ?? ""
            return slot.isEmpty ? nil : slot
        }.joined(separator: "\n")
        return parsed.head + poured
    }
}

/// What a hire's brief is filled in with, besides its role.
struct BriefContext {
    /// Where the hire starts, and where its brief is written.
    var folder: String
    /// Started in a desk's own project, so it asks you to confirm that one
    /// rather than which project to work on.
    var pinned: Bool
    /// The answers to the role's questions, in their order; nil when the
    /// agent asks them itself, first.
    var answers: [String]?
    var date = Date()
    var briefFile: String
    var statusFile: String
    var reportFile: String?

    static let answerLimit = 2000
    static let approval = "The user hired you for this; don't wait for plan approval unless this brief says to ask."
}

extension Role {
    /// The brief's text before it's filled in, or nil for a role started on
    /// its own instructions. Throws if its library brief isn't there.
    func briefTemplate(in library: BriefLibrary) throws -> String? {
        switch source.kind {
        case .text: return nil
        case .file: return source.text
        case .library:
            guard let template = library.template(source.id) else { throw BriefError.missing(source.id) }
            return template
        }
    }

    /// The report file a brief names in its front matter, if any, as its
    /// base name: REVIEW for REVIEW.md.
    static func reportName(of template: String) -> String? {
        let name = BriefTemplate.frontMatter(template).values["report"] ?? ""
        return name.range(of: "^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$", options: .regularExpression) != nil ? name : nil
    }

    /// The brief filled in for a hire: its folder and branch, the answers
    /// to its questions (or the questions to ask first), how it runs, the
    /// pre-approval, and where it reports.
    func renderBrief(_ template: String, _ context: BriefContext) throws -> String {
        let parsed = BriefTemplate.frontMatter(template)
        let agent = Agent(rawValue: agent) ?? .claude
        let shown = { (path: String) in (path as NSString).abbreviatingWithTildeInPath }
        let date = DateFormatter()
        date.locale = Locale(identifier: "en_US_POSIX")
        date.dateFormat = "yyyy-MM-dd"
        var values = parsed.values
        values["role"] = name
        values["date"] = date.string(from: context.date)
        values["project_folder"] = shown(context.folder)
        values["branch"] = Brief.branch(in: context.folder) ?? ""
        values["agent"] = agent.name
        values["model"] = model
        values["effort"] = agent.hasEffortDial ? effort : ""
        values["approval"] = BriefContext.approval
        values["brief_file"] = shown(context.briefFile)
        values["status_file"] = shown(context.statusFile)
        values["report_file"] = context.reportFile.map(shown) ?? ""
        values["done_signal"] = BriefTemplate.isSet(parsed.values["done_signal"]) ? parsed.values["done_signal"] : "JOB DONE"
        values["read_only"] = parsed.values["read_only"] ?? "false"
        values["job"] = job(context, statusFile: shown(context.statusFile))
        return try BriefTemplate.render(parsed.body, values: values, known: BriefTemplate.names.union(parsed.values.keys))
    }

    /// "The job": the role's questions with your answers, or, with none
    /// given, the questions the agent asks you first, as a hire always has.
    private func job(_ context: BriefContext, statusFile: String) -> String {
        let shown = (context.folder as NSString).abbreviatingWithTildeInPath
        guard let answers = context.answers else {
            let project = context.pinned
                ? "You've started in \(shown), this desk's project. Is that the one to work on?"
                : "Which project should you work on: a folder on this computer, or a GitHub repository?"
            let asked = ([project] + questions).enumerated().map { "\($0.offset + 1). \($0.element)" }.joined(separator: "\n")
            return "Before anything else, ask the user these questions in your first reply, all together, and wait for "
                + "their answers. Don't read code, run commands or start work until they've answered.\n\n\(asked)\n\n"
                + "Once they've answered, write each question with its answer at the top of `\(statusFile)`, then start."
        }
        let answered = [("Which project should you work on?", "`\(shown)`")] + questions.enumerated().map { i, question in
            let answer = i < answers.count ? BriefContext.answer(answers[i]) : ""
            return (question, answer.isEmpty ? "*No answer: use your judgement, and say what you chose in `\(statusFile)`.*" : answer)
        }
        return "The user answered these questions as they hired you:\n\n"
            + answered.map { "- **\($0.0)** \($0.1)" }.joined(separator: "\n")
    }

    /// What the agent is first told when its brief is written: to read it,
    /// then to ask its questions first or, answered already, to start.
    func briefMessage(at path: String, asks: Bool) -> String {
        let shown = "`\((path as NSString).abbreviatingWithTildeInPath)`"
        return asks
            ? "Read \(shown) and follow it: it's your brief, as my \(name). As it says, your first reply must only ask me its "
                + "questions, all together; wait for my answers before you read code or run commands."
            : "Read \(shown) and start: it's your brief, as my \(name), with my answers to its questions."
    }
}

extension BriefContext {
    /// An answer as it goes into the brief: without control characters
    /// but line ends, trimmed, and each further line indented to stay in its
    /// item.
    static func answer(_ text: String) -> String {
        let clean = String(String.UnicodeScalarView(text.unicodeScalars.filter {
            $0 == "\n" || !CharacterSet.controlCharacters.contains($0)
        })).trimmingCharacters(in: .whitespacesAndNewlines)
        return String(clean.prefix(answerLimit)).split(separator: "\n", omittingEmptySubsequences: false)
            .map { $0.trimmingCharacters(in: .whitespaces) }.joined(separator: "\n  ")
    }
}

enum BriefError: LocalizedError {
    case missing(String), notWritten(String)

    var errorDescription: String? {
        switch self {
        case .missing(let id): return "The brief \(id) isn't in this copy of Smolcorp."
        case .notWritten(let folder): return "Couldn't write the brief into \((folder as NSString).abbreviatingWithTildeInPath)."
        }
    }
}

/// Where a hire's brief goes. At a desk pinned to its own project, into
/// that folder: BRIEF.md, with STATUS.md beside it (and its report, if it
/// has one), or with any of them taken, BRIEF-<role>.md and the rest, then
/// BRIEF-<role>-2.md and on. Anywhere else, so a shared folder such as
/// ~/Projects stays clean, into Smolcorp's own briefs folder:
/// <desk>-<role>-<yyyyMMdd-HHmm>.md, with <…>-STATUS.md beside it, then
/// <…>-2.md and on. A file that's there is never written over, nor named to
/// the agent as its own.
enum BriefFiles {
    struct Names: Equatable {
        let brief, status: URL
        let report: URL?
    }

    /// Where, and under what names.
    enum Place: Equatable {
        case project(URL, role: String)
        case kept(URL, stem: String)

        var folder: URL {
            switch self {
            case .project(let folder, _), .kept(let folder, _): return folder
            }
        }
    }

    static let most = 99

    /// Where a hire's brief goes: into `folder` at a desk pinned to it,
    /// otherwise into `kept`, named for the desk (as setup counts them, from
    /// 0), the role and the minute.
    static func place(for role: String, in folder: String, pinned: Bool, desk: Int?, at date: Date = Date(),
                      kept: URL = Paths.hireBriefs) -> Place {
        guard !pinned else { return .project(URL(fileURLWithPath: folder), role: role) }
        let stamp = DateFormatter()
        stamp.locale = Locale(identifier: "en_US_POSIX")
        stamp.dateFormat = "yyyyMMdd-HHmm"
        return .kept(kept, stem: "\(desk.map(String.init) ?? "desk")-\(role)-\(stamp.string(from: date))")
    }

    static func names(_ place: Place, report: String?, suffix n: Int) -> Names {
        switch place {
        case .project(let folder, let role):
            let tail = n == 0 ? "" : n == 1 ? "-\(role)" : "-\(role)-\(n)"
            let file = { (base: String) in folder.appendingPathComponent("\(base)\(tail).md") }
            return Names(brief: file("BRIEF"), status: file("STATUS"), report: report.map(file))
        case .kept(let folder, let stem):
            let name = n == 0 ? stem : "\(stem)-\(n + 1)"
            let file = { (part: String?) in folder.appendingPathComponent(part.map { "\(name)-\($0).md" } ?? "\(name).md") }
            return Names(brief: file(nil), status: file("STATUS"), report: report.map(file))
        }
    }

    /// Writes the brief `render` makes for the names it's given, under the
    /// first names none of whose files is there. The brief is created only if
    /// it isn't there, so even a file that turns up meanwhile is left as it is.
    /// Smolcorp's own briefs folder is made if it isn't there, for you alone;
    /// a project's folder never is.
    static func write(_ place: Place, report: String?, render: (Names) throws -> String) throws -> Names {
        let manager = FileManager.default
        if case .kept(let folder, _) = place {
            try? manager.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        }
        let there = { (url: URL) in (try? manager.attributesOfItem(atPath: url.path)) != nil }
        for n in 0...most {
            let names = names(place, report: report, suffix: n)
            guard ![names.brief, names.status].contains(where: there), !(names.report.map(there) ?? false) else { continue }
            let text = try render(names)
            let fd = open(names.brief.path, O_WRONLY | O_CREAT | O_EXCL | O_NOFOLLOW, 0o644)
            if fd < 0 {
                if errno == EEXIST { continue }
                throw BriefError.notWritten(place.folder.path)
            }
            let data = Data(text.utf8)
            let written = data.withUnsafeBytes { Darwin.write(fd, $0.baseAddress, $0.count) }
            close(fd)
            guard written == data.count else { throw BriefError.notWritten(place.folder.path) }
            return names
        }
        throw BriefError.notWritten(place.folder.path)
    }
}
