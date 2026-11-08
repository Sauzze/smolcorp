import Foundation

/// The folder new hires work in, in one move (Start here's first step): a
/// folder to suggest when none is set, the checks a folder must pass, each
/// problem in a plain sentence with its fix, and keeping one. The
/// Linux/Windows app's hirefolder.go answers with the same JSON.
enum HireFolder {
    /// Where projects usually are already, in order: the first one there that
    /// passes the checks is suggested. Only these, never a search.
    static let likely = ["Projects", "projects", "Developer", "Code", "code", "src", "dev", "repos", "source/repos"]
    /// What's suggested when none of those is there: made only once it's accepted.
    static let fallback = "Smolcorp"

    /// What the page offers for a problem: make the folder, or choose another.
    enum Fix: String { case none = "", create, choose }

    /// A folder as checked: its full path, whether it's there, and what's wrong with it, if anything.
    struct Check: Equatable, Error {
        var folder = ""
        var exists = false
        var problem = ""
        var fix = Fix.none
    }

    /// `given` checked as the folder for new hires. `~` is `home`.
    static func check(_ given: String, home: URL = Paths.home) -> Check {
        let trimmed = given.trimmingCharacters(in: .whitespacesAndNewlines)
        let example = "such as ~/Projects"
        guard !trimmed.isEmpty else { return Check(problem: "Type the folder's full path, \(example).", fix: .choose) }
        let expanded = trimmed == "~" || trimmed.hasPrefix("~/") ? home.path + trimmed.dropFirst() : trimmed
        guard expanded.hasPrefix("/") else { return Check(problem: "Give the folder's full path, \(example).", fix: .choose) }
        let folder = real(expanded), base = real(home.path)
        var check = Check(folder: folder)
        func problem(_ text: String, _ fix: Fix = .choose) -> Check {
            check.problem = text
            check.fix = fix
            return check
        }
        if folder == "/" { return problem("That's your Mac's whole disk. Choose a folder inside your home folder, \(example).") }
        if folder == base {
            return problem("That's your whole home folder. Choose a folder inside it, \(example), so hires keep to their projects.")
        }
        guard folder.hasPrefix(base + "/") else { return problem("Choose a folder inside your home folder, \(example).") }
        let parts = folder.dropFirst(base.count + 1).split(separator: "/").map(String.init)
        if ["Library", "AppData"].contains(parts.first ?? "") || parts.contains(where: { $0.hasPrefix(".") || $0.hasSuffix(".app") }) {
            return problem("That folder holds your Mac's or your apps' own files. Choose a project folder, \(example).")
        }
        let shown = shown(folder, home: base)
        var isFolder: ObjCBool = false
        guard FileManager.default.fileExists(atPath: folder, isDirectory: &isFolder) else {
            return problem("There's no folder at \(shown) yet.", .create)
        }
        check.exists = true
        guard isFolder.boolValue else { return problem("\(shown) is a file, not a folder.") }
        guard FileManager.default.isWritableFile(atPath: folder) else {
            return problem("Smolcorp can't write in \(shown). Choose a folder of your own, \(example).")
        }
        return check
    }

    /// The folder to suggest when none is set: the first likely one that's
    /// there and passes the checks, else ~/Smolcorp.
    static func suggestion(home: URL = Paths.home) -> String {
        for name in likely {
            let found = check(home.appendingPathComponent(name).path, home: home)
            if found.exists, found.problem.isEmpty { return found.folder }
        }
        return check(home.appendingPathComponent(fallback).path, home: home).folder
    }

    /// `GET /setup/hire-folder`: the folder set (`saved`), or, with none, the
    /// one suggested; or the one `asked` about (`?check=`, as you type).
    /// `folder` is what's set or asked, `suggested` the suggestion when
    /// neither; `exists`, `problem` and `fix` are about whichever is shown.
    static func state(saved: String, asked: String? = nil, home: URL = Paths.home) -> [String: Any] {
        if let asked { return json(check(asked, home: home), suggested: "", home: home) }
        if saved.isEmpty {
            let suggested = suggestion(home: home)
            var shown = check(suggested, home: home)
            shown.folder = ""
            // Not there yet is no problem for the suggestion: it's made once accepted.
            if shown.fix == .create { (shown.problem, shown.fix) = ("", .none) }
            return json(shown, suggested: suggested, home: home)
        }
        var shown = check(saved, home: home)
        shown.folder = saved
        return json(shown, suggested: "", home: home)
    }

    /// The route's answer, the same on Linux and Windows.
    static func json(_ check: Check, suggested: String, home: URL = Paths.home) -> [String: Any] {
        ["folder": check.folder, "suggested": suggested, "exists": check.exists, "problem": check.problem, "fix": check.fix.rawValue,
         "home": real(home.path)]
    }

    /// `POST /setup/hire-folder?folder=…[&create=1]`: the folder to keep, checked, made
    /// first when it isn't there yet and `create` says so; or why it can't be.
    static func keep(_ given: String, create: Bool, home: URL = Paths.home) -> Result<String, Check> {
        var found = check(given, home: home)
        if create, found.fix == .create {
            do {
                try FileManager.default.createDirectory(atPath: found.folder, withIntermediateDirectories: true)
                found = check(found.folder, home: home)
            } catch {
                found.problem = "Couldn't make \(shown(found.folder, home: real(home.path))). Choose another folder, such as ~/Projects."
                found.fix = .choose
            }
        }
        return found.problem.isEmpty ? .success(found.folder) : .failure(found)
    }

    /// `path` with `..` and symlinks worked out, as the hire sheet's own check does.
    private static func real(_ path: String) -> String {
        let resolved = URL(fileURLWithPath: path).standardizedFileURL.resolvingSymlinksInPath().path
        return resolved.count > 1 && resolved.hasSuffix("/") ? String(resolved.dropLast()) : resolved
    }

    /// `path` with ~ for `home`, as the page shows it.
    static func shown(_ path: String, home: String) -> String {
        path == home ? "~" : path.hasPrefix(home + "/") ? "~" + path.dropFirst(home.count) : path
    }
}
