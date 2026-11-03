import Foundation

/// `POST /setup/gm/brief` with `{id, text}`: a later part of a brief for a hire that is already working.
/// The text is added, dated, to the end of the hire's BRIEF file (never written over), and the hire is
/// mailed a short note to read it. The note goes through the mail gate, so it is never typed over a
/// question (T401/T402); the text itself is never typed anywhere.
enum BriefMore {
    /// The note the hire gets, with `file` as it should read it.
    static func note(_ file: String) -> String { "Your brief has a new part: read \(file)" }

    /// What the hire is told, once the part is in the file.
    struct Added {
        let file: URL
    }

    enum Refusal: Error, Equatable {
        case bad(String)
        case none(String)
        case gone(String)
        case tooBig(Int, Int)
        case noBrief(String)
        case notWritten(String)

        var response: Response {
            switch self {
            case .bad(let why): return .status(400, why)
            case .none(let why): return .status(404, why)
            case .gone(let why): return .status(409, why)
            case .tooBig(let size, let most):
                return .status(413, "That brief is \(size) bytes. The limit is \(most) bytes (setting mailMaxBytes).")
            case .noBrief(let why): return .status(409, why)
            case .notWritten(let why): return .status(500, why)
            }
        }
    }

    /// The brief file a hire reads: the path written down when it was hired, and only that. A hire from before
    /// that was kept is found only if it is managed, since its folder in Smolcorp's is its alone; any other is
    /// not guessed at, as a project folder can hold several hires' briefs and the owner's own.
    static func file(of hire: FrontDesk.Hire, hires: URL = Paths.hires) -> URL? {
        if !hire.brief.isEmpty {
            let url = URL(fileURLWithPath: hire.brief)
            return regular(url) ? url : nil
        }
        guard !hire.worktree.isEmpty else { return nil }
        let folder = BriefBuilder.hireFolder(hire.id, in: hires)
        let names = ((try? FileManager.default.contentsOfDirectory(atPath: folder.path)) ?? [])
            .filter { $0.hasPrefix("BRIEF") && $0.hasSuffix(".md") }.sorted()
        return names.map { folder.appendingPathComponent($0) }.first(where: regular)
    }

    /// A real file, not a link to somewhere else.
    private static func regular(_ url: URL) -> Bool {
        (try? url.resourceValues(forKeys: [.isRegularFileKey, .isSymbolicLinkKey]))
            .map { $0.isRegularFile == true && $0.isSymbolicLink != true } ?? false
    }

    /// What the part looks like in the file: a heading with the date, then the text.
    static func part(_ text: String, at date: Date) -> String {
        let stamp = DateFormatter()
        stamp.locale = Locale(identifier: "en_US_POSIX")
        stamp.dateFormat = "yyyy-MM-dd HH:mm"
        return "\n\n## New part, \(stamp.string(from: date))\n\n\(text.trimmingCharacters(in: .whitespacesAndNewlines))\n"
    }

    /// Checks the request and adds the part to the file. Nothing is written for a refusal.
    static func add(body: Data, hires: [FrontDesk.Hire], maxBytes: Int, now: Date = Date(),
                    folder: URL = Paths.hires) -> Result<(hire: FrontDesk.Hire, added: Added), Refusal> {
        guard let json = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any],
              let id = json["id"] as? String, !id.trimmingCharacters(in: .whitespaces).isEmpty,
              let text = json["text"] as? String else { return .failure(.bad("Send {id, text}: which hire, and the new part of its brief.")) }
        guard !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return .failure(.bad("The brief is empty.")) }
        guard let hire = hires.first(where: { $0.id == id }) else { return .failure(.none("There's no hire with that id.")) }
        if hire.role == "gm" { return .failure(.gone("The GM has no brief to add to; it is the one who writes them.")) }
        if hire.state == .fired || hire.state == .failed {
            return .failure(.gone("\(hire.nickname.isEmpty ? hire.name : hire.nickname) is already gone."))
        }
        let size = text.utf8.count
        if size > maxBytes { return .failure(.tooBig(size, maxBytes)) }
        guard let file = file(of: hire, hires: folder) else {
            return .failure(.noBrief("\(hire.nickname.isEmpty ? hire.name : hire.nickname) has no BRIEF file to add to."))
        }
        // Added at the end, as one write. Opened, never made, and a link put in the file's place since the check is not followed.
        let fd = open(file.path, O_WRONLY | O_APPEND | O_NOFOLLOW)
        guard fd >= 0 else { return .failure(.notWritten("Couldn't open the brief file.")) }
        defer { close(fd) }
        let data = Data(part(text, at: now).utf8)
        let written = data.withUnsafeBytes { Darwin.write(fd, $0.baseAddress, $0.count) }
        guard written == data.count else { return .failure(.notWritten("Couldn't add to the brief file.")) }
        return .success((hire, Added(file: file)))
    }
}
