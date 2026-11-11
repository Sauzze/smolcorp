import Foundation

/// A manager hire's own Claude Code settings: a file the app writes for that hire alone,
/// `<support>/hires/<id>/settings.json` (private to you), which the hire's Claude Code starts with
/// (`--settings`), merged on top of the account's own settings. It holds
///
/// - deny rules for the plain forms of what the guard refuses (push, deploy, publish…), for reading
///   `.env` files and Claude's session transcripts, and for editing Claude's settings and this app's files;
/// - the guard (`smolcorp guard`) as a PreToolUse hook before every tool call;
/// - auto mode, where the account allows it.
///
/// Deny rules only see the start of a command, so they stop the plain forms; the guard sees the rest.
/// A deny rule always beats an allow, so the hire's own STATUS.md (which it must write) is never
/// covered by a rule: the support folder is denied piece by piece, never as a whole.
///
/// The General Manager's file is the same with its own guard profile (`Profile.gm`). It may still read its team's
/// `hires/*/STATUS.md` (nothing denies reading them), and it can't edit them, its settings, or read session transcripts.
///
/// Hires that aren't managed get none of this. A managed hire is never started without the guard:
/// if the guard can't be run, the hire is refused.
enum HireSettings {
    typealias Refusal = FrontDesk.Refusal
    /// `tool arguments`: its status and output, nil if it couldn't be run.
    typealias Runner = (_ tool: String, _ arguments: [String]) -> (status: Int32, output: String)?

    /// Whose rules a settings file carries: a hire's, or the General Manager's own. The GM's differ in one thing: the guard
    /// lets it use its own `smolcorp gm` actions (hire, fire, brief…); it can't push, deploy, publish or use the cloud either.
    enum Profile: String {
        case hire, gm
        /// The words after `guard` that choose it. A hire's has none: its command line is the one hires always had.
        var guardArguments: String { self == .gm ? "--profile gm " : "" }
    }

    static let guardFileName = "hire-guard.json"
    static let fileName = "settings.json"
    /// The folders the app makes in its support folder, besides `hires` (see `Paths`).
    static let knownFolders = ["briefs", "recaps"]

    /// The groups the guard refuses, as it says them (`smolcorp guard --print`).
    struct Group: Codable, Equatable {
        var name: String
        var on: Bool
        var commands: [String]?
    }

    static let guardMissing = Refusal(status: 409, message: "This copy of Smolcorp has no smolcorp command inside it, so a hire can't be guarded, and Smolcorp won't start one unguarded. Install a build of Smolcorp that includes it.")

    /// The hire's own folder in the support folder, where its settings, brief and status go.
    static func folder(of hireID: String, support: URL = Paths.support) -> URL {
        support.appendingPathComponent("hires", isDirectory: true).appendingPathComponent(hireID, isDirectory: true)
    }

    static func file(of hireID: String, support: URL = Paths.support) -> URL {
        folder(of: hireID, support: support).appendingPathComponent(fileName)
    }

    // MARK: The guard

    static let runTool: Runner = { tool, arguments in Tool.run(tool, arguments, errors: false) }

    /// The groups in force, as the guard itself reads `rules`: the app and the hook never disagree.
    /// Nil if it can't be run or says something we can't read.
    static func groups(tool: String, rules: URL, profile: Profile = .hire, run: Runner = runTool) -> [Group]? {
        let profileWords = profile == .gm ? ["--profile", "gm"] : []
        guard let answer = run(tool, ["guard", "--print"] + profileWords + ["--rules", rules.path]), answer.status == 0,
              let object = try? JSONSerialization.jsonObject(with: Data(answer.output.utf8)) as? [String: Any],
              let list = object["groups"].flatMap({ try? JSONSerialization.data(withJSONObject: $0) }),
              let groups = try? JSONDecoder().decode([Group].self, from: list) else { return nil }
        return groups
    }

    /// The hook's command line: the tool, quoted for the shell, then `guard` and the rules file.
    static func guardCommand(tool: String, rules: URL, profile: Profile = .hire) -> String {
        "\(Openers.quoted(tool)) guard \(profile.guardArguments)--rules \(Openers.quoted(rules.path))"
    }

    /// The deny rules the groups in force give: each command with its words, each alternative (eas build,
    /// eas submit…), plain and through npx. A rule that needs an option, its words anywhere or a remote
    /// host is the guard's alone: a deny rule can't say it.
    static func commandRules(_ groups: [Group]) -> [String] {
        var rules: [String] = []
        for group in groups where group.on {
            for rule in group.commands ?? [] {
                let words = rule.split(separator: " ").map(String.init)
                guard !words.isEmpty, !words.contains(where: { $0 == "..." || $0.hasPrefix("@") || $0.hasPrefix("-") }) else { continue }
                var forms = [""]
                for word in words {
                    let alternatives = word.split(separator: "|").map(String.init)
                    forms = forms.flatMap { start in alternatives.map { start.isEmpty ? $0 : "\(start) \($0)" } }
                }
                for form in forms {
                    for rule in ["Bash(\(form):*)", "Bash(npx \(form):*)"] where !rules.contains(rule) { rules.append(rule) }
                }
            }
        }
        return rules
    }

    // MARK: The rules

    /// A path as Claude Code's rules write an absolute one: `//` then the path.
    private static func absolute(_ path: String) -> String { "//" + (path.hasPrefix("/") ? String(path.dropFirst()) : path) }

    /// Every deny rule for hire `hireID`. `others` are the names of the other hires' folders under
    /// `<support>/hires`, `entries` what sits in the support folder now (a folder named here is denied whole,
    /// except `hires`, which is denied hire by hire so this hire's own STATUS.md stays writable;
    /// `knownFolders` are listed too when they are absent).
    static func denyRules(hireID: String, groups: [Group], support: URL, profiles: [ClaudeProfile], others: [String] = [], entries: [String] = []) -> [String] {
        var rules = commandRules(groups)
        // Claude Code's Edit rules cover every tool that changes a file (Write too); a Write(path) rule never matches and warns at every start.
        func both(_ pattern: String) { rules.append("Edit(\(pattern))") }

        // The hire's secrets and Claude's own settings, wherever they are.
        rules += ["Read(**/.env*)", "Read(//**/.env*)"]
        both("**/.env*"); both("//**/.env*")
        both("~/.claude*/settings*.json"); both("~/.claude*.json")
        both("**/.claude/settings*.json"); both("//**/.claude/settings*.json")
        rules += ["Read(~/.claude*/projects/**)"]
        for profile in profiles {
            let dir = (profile.dir as NSString).standardizingPath
            guard dir.hasPrefix("/") else { continue }
            both("\(absolute(dir))/settings*.json")
            rules.append("Read(\(absolute(dir))/projects/**)")
        }

        // This app's folder, piece by piece: never a rule that would cover the hire's own STATUS.md.
        // The secrets of both engines a Mac may run (this app's gm-session and hook-header, the Go engine's
        // hook-key and window-key), as the Go engine lists them: one file whichever engine writes it.
        let root = absolute(support.path)
        for pattern in ["*.json", "*.sh", "*.md", "*.log", "*.jsonl", "gm-token", "gm-session", "setup-key", "hook-header", "hook-key", "window-key", "token"] {
            both("\(root)/\(pattern)")
        }
        for entry in entries where entry != "hires" && !entry.hasPrefix(".") && !entry.contains("/") { both("\(root)/\(entry)/**") }
        // The folders the app is known to make, listed even when absent now, so one made later is covered too.
        for name in knownFolders where !entries.contains(name) { both("\(root)/\(name)/**") }
        both("\(root)/hires/*.sh")
        // Every hire's folder, by file type and not by name: a hire started after this file was written is covered too.
        // STATUS.md is `.md`, so it stays writable.
        for pattern in ["*.json", "*.sh", "*.jsonl", "*.log", "settings*"] { both("\(root)/hires/*/\(pattern)") }
        for other in others where other != hireID && !other.contains("/") && !other.hasPrefix(".") { both("\(root)/hires/\(other)/**") }
        // Nothing secret or app-state can be read either.
        for name in ["gm-token", "gm-session", "setup-key", "hook-header", "hook-key", "window-key", "token", guardFileName, "phones.json", "sessions.json", "config.json", "registry.json"] {
            rules.append("Read(\(root)/\(name))")
        }
        return rules
    }

    /// The settings file's JSON. Auto mode goes in as the default mode, where Claude Code lets the account have it.
    /// `reporting` is only for a test copy's hire: the hooks that make it report to the copy (`Hooks.hireHooks`), added after the guard.
    static func settings(deny: [String], guardCommand: String, auto: Bool, reporting: [String: Any]? = nil) -> [String: Any] {
        var permissions: [String: Any] = ["deny": deny]
        if auto { permissions["defaultMode"] = "auto" }
        var hooks: [String: Any] = ["PreToolUse": [["matcher": "*", "hooks": [["type": "command", "command": guardCommand, "timeout": 10]]]]]
        for (event, groups) in reporting ?? [:] {
            hooks[event] = (hooks[event] as? [[String: Any]] ?? []) + (groups as? [[String: Any]] ?? [])
        }
        // Hooks stay on: this file sits above the project's and local settings, so a checkout can't turn the guard off.
        return ["permissions": permissions, "disableAllHooks": false, "hooks": hooks]
    }

    /// The settings of a test copy's hire that isn't guarded (no manager, no worktree): its reporting hooks and nothing else.
    @discardableResult
    static func writeReportingOnly(hireID: String, hooks: [String: Any], support: URL = Paths.support) throws -> URL {
        let target = file(of: hireID, support: support)
        guard let data = try? JSONSerialization.data(withJSONObject: ["hooks": hooks], options: [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]),
              (try? FileManager.default.createDirectory(at: folder(of: hireID, support: support), withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])) != nil,
              Paths.writePrivately(data, to: target) else {
            throw Refusal(status: 500, message: "Couldn't write the hire's settings.")
        }
        return target
    }

    // MARK: Writing

    /// What hire `hireID`'s settings file holds (deny rules, the guard as a PreToolUse hook, auto mode), as `write` writes it:
    /// refused if the guard can't be run. A hire in T3 Code gets the same, in its worktree's local settings (`T3Desk`).
    static func object(hireID: String, auto: Bool = true, profile: Profile = .hire, cli: URL = Paths.bundledCLI, support: URL = Paths.support,
                       profiles: [ClaudeProfile] = ClaudeProfiles.current, run: Runner = runTool, reporting: [String: Any]? = nil,
                       isExecutable: (String) -> Bool = { FileManager.default.isExecutableFile(atPath: $0) }) throws -> [String: Any] {
        guard isExecutable(cli.path) else { throw guardMissing }
        let rules = support.appendingPathComponent(guardFileName)
        guard let groups = groups(tool: cli.path, rules: rules, profile: profile, run: run) else {
            throw Refusal(status: 500, message: "The guard couldn't say which commands a hire is refused, so Smolcorp won't start one unguarded.")
        }
        let manager = FileManager.default
        let hires = support.appendingPathComponent("hires", isDirectory: true)
        let others = ((try? manager.contentsOfDirectory(atPath: hires.path)) ?? []).filter { $0 != hireID }
        let entries = (try? manager.contentsOfDirectory(atPath: support.path)) ?? []
        let deny = denyRules(hireID: hireID, groups: groups, support: support, profiles: profiles, others: others, entries: entries)
        return settings(deny: deny, guardCommand: guardCommand(tool: cli.path, rules: rules, profile: profile), auto: auto, reporting: reporting)
    }

    /// What a hire of `role` is given: the General Manager's profile for the GM, a hire's for anyone else, and `reporting` for a
    /// test copy's hire. The one place this is chosen, for a hire Smolcorp starts (`save`d as its settings file) and for a hire
    /// in T3 Code (written as its worktree's fence, `T3Desk`) alike, so the two never drift apart.
    static func forHire(_ hireID: String, role: Role?, reporting: [String: Any]? = nil) throws -> [String: Any] {
        try object(hireID: hireID, profile: role?.id == Deck.gmRole ? .gm : .hire, reporting: reporting)
    }

    /// Writes hire `hireID`'s settings and says where: refused if the guard can't be run. The file is private
    /// to you and replaced whole. `cli` is the smolcorp command inside the app.
    @discardableResult
    static func write(hireID: String, auto: Bool = true, profile: Profile = .hire, cli: URL = Paths.bundledCLI, support: URL = Paths.support,
                      profiles: [ClaudeProfile] = ClaudeProfiles.current, run: Runner = runTool, reporting: [String: Any]? = nil,
                      isExecutable: (String) -> Bool = { FileManager.default.isExecutableFile(atPath: $0) }) throws -> URL {
        try save(HireSettings.object(hireID: hireID, auto: auto, profile: profile, cli: cli, support: support, profiles: profiles, run: run,
                                     reporting: reporting, isExecutable: isExecutable), hireID: hireID, support: support)
    }

    /// A settings file's bytes, as `save` writes them. The Go engine writes the same bytes for the same rules
    /// (desktop/swiftjson.go), which `Tests/fixtures/frontdesk/hire-settings.json` checks.
    static func encoded(_ object: [String: Any]) -> Data? {
        try? JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes])
    }

    /// Writes `object` as hire `hireID`'s settings file, private to you and replaced whole, and says where.
    @discardableResult
    static func save(_ object: [String: Any], hireID: String, support: URL = Paths.support) throws -> URL {
        let manager = FileManager.default
        let place = folder(of: hireID, support: support)
        let target = file(of: hireID, support: support)
        guard let data = encoded(object),
              (try? manager.createDirectory(at: place, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])) != nil,
              Paths.writePrivately(data, to: target) else {
            throw Refusal(status: 500, message: "Couldn't write the hire's settings, so Smolcorp won't start it unguarded.")
        }
        return target
    }
}
