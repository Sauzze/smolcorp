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

    /// Where hires may not work, by a folder's first part from `/`: the system's own folders (and Linux's
    /// and Windows' too, as hirefolder.go has them), wherever the folder isn't inside the home folder.
    static let systemFolders: Set<String> = ["applications", "bin", "boot", "cores", "dev", "etc", "lib", "lib32", "lib64", "libx32",
                                             "library", "private", "proc", "run", "sbin", "snap", "sys", "system", "tmp", "usr", "var",
                                             "windows", "program files", "program files (x86)", "programdata"]
    /// Where other disks show up: one of these, or a disk right inside it (`/Volumes/Work`), is a whole disk.
    static let diskFolders: Set<String> = ["volumes", "mnt", "media"]

    /// `given` checked as the folder for new hires. `~` is `home`. A folder already `saved` is checked
    /// only for what hiring needs (there, a folder, writable), so a folder kept before these checks
    /// stays as it was; a new one may also not be a whole disk, the home folder or a system or app folder.
    static func check(_ given: String, saved: Bool = false, home: URL = Paths.home) -> Check {
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
        if !saved, let refusal = refusal(folder, home: base, example: example) { return problem(refusal) }
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

    /// Why `folder` (worked out by `real`) can't be a new hires folder, if it can't. Names are
    /// compared whatever their letter case, as the Mac's disk does.
    private static func refusal(_ folder: String, home: String, example: String) -> String? {
        if folder == home {
            return "That's your whole home folder. Choose a folder inside it, \(example), so hires keep to their projects."
        }
        let inside = folder.hasPrefix(home + "/")
        let parts = (inside ? folder.dropFirst(home.count + 1) : Substring(folder)).split(separator: "/").map { $0.lowercased() }
        let first = parts.first ?? ""
        if !inside, parts.isEmpty || diskFolders.contains(first) && parts.count <= (first == "media" ? 3 : 2) {
            return "That's a whole disk. Choose a project folder, \(example)."
        }
        let system = inside ? ["library", "appdata"].contains(first) : systemFolders.contains(first)
        if system || parts.contains(where: { $0.hasSuffix(".app") }) {
            return "That folder holds your Mac's or your apps' own files. Choose a project folder, \(example)."
        }
        if let hidden = hiddenRefusal(parts, example: example) { return hidden }
        return nil
    }

    /// Why a path with a hidden (dot) folder in it can't be used, saying so: the folder itself, or one it's
    /// inside. As hirefolder.go's hiddenHireFolderRefusal.
    static func hiddenRefusal(_ parts: [String], example: String) -> String? {
        guard let at = parts.firstIndex(where: { $0.hasPrefix(".") }) else { return nil }
        if at == parts.count - 1 {
            return "A hidden folder (one whose name starts with '.') can't be used. Choose a project folder, \(example)."
        }
        return "A folder inside a hidden folder (one whose name starts with '.') can't be used: choose one outside it."
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
        var shown = check(saved, saved: true, home: home)
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

    /// `/setup/hire-folder` as DeckRouter answers it: GET is `state`; POST keeps `folder=` in `config`
    /// once checked, then `saved()` saves it and refreshes. A refusal answers 400 with the same JSON,
    /// to show with its fix, and the folder set stays as it was.
    static func route(_ request: Request, config: Config, home: URL = Paths.home, saved: () -> Void) -> Response {
        guard request.method == "POST" else {
            return .json(state(saved: config.hireFolder, asked: request.query["check"], home: home))
        }
        switch keep(request.query["folder"] ?? "", create: request.query["create"] == "1", home: home) {
        case .failure(let found):
            return .json(json(found, suggested: "", home: home), status: 400)
        case .success(let folder):
            config.hireFolder = folder
            saved()
            return .json(state(saved: folder, home: home))
        }
    }

    /// `path` with `..` worked out, and the symlinks of its deepest part that's there resolved (with the
    /// letter case the disk has), the rest put back as given: as hirefolder.go's realPath. So a folder
    /// not made yet is checked where it would really be made.
    private static func real(_ path: String) -> String {
        var there = URL(fileURLWithPath: path).standardizedFileURL.path
        var rest: [String] = []
        while there != "/", !FileManager.default.fileExists(atPath: there) {
            rest.insert((there as NSString).lastPathComponent, at: 0)
            there = (there as NSString).deletingLastPathComponent
        }
        var resolved = URL(fileURLWithPath: there).resolvingSymlinksInPath().path
        for part in rest { resolved = (resolved as NSString).appendingPathComponent(part) }
        return resolved.count > 1 && resolved.hasSuffix("/") ? String(resolved.dropLast()) : resolved
    }

    /// `path` with ~ for `home`, as the page shows it.
    static func shown(_ path: String, home: String) -> String {
        path == home ? "~" : path.hasPrefix(home + "/") ? "~" + path.dropFirst(home.count) : path
    }
}
