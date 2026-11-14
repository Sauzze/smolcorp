import Foundation

/// A desk above the ground floor: its floor, 1 for 2F up to 5 for 6F, and
/// which of that floor's desks it is.
struct Seat: Codable, Equatable, Hashable {
    let floor: Int
    let desk: Int
}

/// A folder whose sessions sit on a floor of their own: "Atlas projects go
/// to the Atlas floor". `floor` is 0 for the ground floor up to 5 for 6F.
struct FloorRule: Codable, Equatable {
    var folder: String
    var floor: Int
    /// The folder alone, not those in it: as "Always put … here" remembers
    /// it from the office, where a folder above a project's would otherwise
    /// take all of them up. A rule added in setup holds the folders in it too.
    var exact = false

    static let most = 64

    init(folder: String, floor: Int, exact: Bool = false) {
        self.folder = folder
        self.floor = floor
        self.exact = exact
    }

    /// Read leniently: a rule from before `exact` holds the folders in it.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        folder = try c.decode(String.self, forKey: .folder)
        floor = try c.decode(Int.self, forKey: .floor)
        exact = (try? c.decodeIfPresent(Bool.self, forKey: .exact)) ?? false
    }

    /// As they're kept: each folder a path, with no slash at its end, on a
    /// floor there is; a folder's rule for itself alone, and its rule for
    /// the folders in it too, one of each, the last given winning.
    static func checked(_ given: [FloorRule]) -> [FloorRule] {
        var rules: [FloorRule] = []
        for rule in given {
            guard let folder = ruleFolder(rule.folder), (0..<Floor.count).contains(rule.floor) else { continue }
            // One of each kind a folder: "Always put … here" for itself alone beside setup's for the folders in it.
            rules.removeAll { $0.folder == folder && $0.exact == rule.exact }
            rules.append(FloorRule(folder: folder, floor: rule.floor, exact: rule.exact))
        }
        return Array(rules.suffix(most))
    }

    /// A rule's folder as it's kept: a whole path, a Mac's (/…), under your
    /// home (~/…), or a Windows computer's (C:\…, \\server\…), kept as given for
    /// a config that goes back there; with no separator at its end. Nil for
    /// one that isn't a whole path.
    static func ruleFolder(_ given: String) -> String? {
        var folder = given.trimmingCharacters(in: .whitespacesAndNewlines)
        if folder == "~" || folder.hasPrefix("~/") { folder = (folder as NSString).expandingTildeInPath }
        let chars = Array(folder)
        let windows = chars.count >= 3 && chars[0].isASCII && chars[0].isLetter && chars[1] == ":" && (chars[2] == "\\" || chars[2] == "/")
        guard folder.hasPrefix("/") || folder.hasPrefix("\\\\") || windows else { return nil }
        while folder.count > 1, let last = folder.last, last == "/" || last == "\\", !(windows && folder.count == 3) { folder.removeLast() }
        return folder
    }

    /// The floor `project` goes to by its rules: the rule for the folder it's
    /// in, or the closest above it. Nil when no rule says.
    static func floor(of project: String, in rules: [FloorRule]) -> Int? {
        guard !project.isEmpty else { return nil }
        // The closest folder; on that folder itself, its own rule before the one for what's in it; then the later.
        return rules.enumerated()
            .filter { _, rule in project == rule.folder || !rule.exact && project.hasPrefix(rule.folder == "/" ? "/" : rule.folder + "/") }
            .max { a, b in (a.element.folder.count, a.element.exact ? 1 : 0, a.offset) < (b.element.folder.count, b.element.exact ? 1 : 0, b.offset) }?
            .element.floor
    }

    /// The folder a hire on `floor` starts in: the floor's one rule's, or
    /// nil when it has none or more than one.
    static func folder(of floor: Int, in rules: [FloorRule]) -> String? {
        let on = Set(rules.filter { $0.floor == floor }.map(\.folder))
        return on.count == 1 ? on.first : nil
    }

    var jsonObject: [String: Any] { ["folder": folder, "floor": floor, "exact": exact] }
}
