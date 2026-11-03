import Foundation

/// A Claude Code account on this Mac: a config folder of its own, as
/// `CLAUDE_CONFIG_DIR` names it, signed in to one account. The one in
/// ~/.claude is always there, called "default"; the others are the ones you
/// confirmed in setup.
struct ClaudeProfile: Codable, Equatable {
    var name: String
    var dir: String

    var sessions: URL { URL(fileURLWithPath: dir, isDirectory: true).appendingPathComponent("sessions", isDirectory: true) }
    var settings: URL { URL(fileURLWithPath: dir, isDirectory: true).appendingPathComponent("settings.json") }
    var isDefault: Bool { name == ClaudeProfiles.defaultName }
}

/// The Claude Code accounts Smolcorp follows. Only folders you confirmed are
/// ever written to; folders that merely look like one are offered in setup
/// (`found`) and nothing more.
enum ClaudeProfiles {
    static let defaultName = "default"
    static let longestName = 30
    /// More than anyone has; keeps the list in setup short.
    static let most = 8

    /// ~/.claude, the account used when none is named.
    static func defaultProfile(home: URL = Paths.home) -> ClaudeProfile {
        ClaudeProfile(name: defaultName, dir: home.appendingPathComponent(".claude", isDirectory: true).path)
    }

    /// The default account, then the confirmed ones that are still folders. A
    /// saved entry that is badly formed, repeats a name or a folder, or
    /// whose folder has gone is left out.
    static func all(saved: [ClaudeProfile], home: URL = Paths.home) -> [ClaudeProfile] {
        let first = defaultProfile(home: home)
        var list = [first]
        for profile in saved {
            guard case .success(let checked) = check(profile, home: home), !checked.isDefault,
                  !list.contains(where: { $0.name == checked.name || $0.dir == checked.dir }),
                  isFolder(checked.dir) else { continue }
            list.append(checked)
        }
        return list
    }

    /// This Mac's accounts now.
    static var current: [ClaudeProfile] { all(saved: Config.shared.claudeProfiles) }

    /// A name made of letters, digits, dashes and underscores, the folder an
    /// absolute path (with ~ for home) that isn't ~/.claude itself.
    static func check(_ profile: ClaudeProfile, home: URL = Paths.home) -> Result<ClaudeProfile, Refusal> {
        let name = profile.name.trimmingCharacters(in: .whitespacesAndNewlines)
        let allowed = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "-_"))
        guard !name.isEmpty, name.count <= longestName, name.unicodeScalars.allSatisfy(allowed.contains) else {
            return .failure(Refusal(message: "A name is up to \(longestName) letters, digits, dashes or underscores."))
        }
        var path = profile.dir.trimmingCharacters(in: .whitespacesAndNewlines)
        if path == "~" || path.hasPrefix("~/") { path = home.path + path.dropFirst() }
        guard path.hasPrefix("/"), !path.contains("\0") else {
            return .failure(Refusal(message: "Give the account's folder as a full path, such as ~/.claude-work."))
        }
        path = (path as NSString).standardizingPath
        if name == defaultName || path == defaultProfile(home: home).dir {
            return .failure(Refusal(message: "\"default\" is ~/.claude, which is always there."))
        }
        return .success(ClaudeProfile(name: name, dir: path))
    }

    struct Refusal: Error, Equatable {
        let message: String
    }

    /// The list as setup sends it, checked whole: each entry well formed, a
    /// folder that is there, no name or folder twice. Nothing is kept if one fails.
    static func checked(_ given: [ClaudeProfile], home: URL = Paths.home) -> Result<[ClaudeProfile], Refusal> {
        guard given.count <= most else { return .failure(Refusal(message: "That's the most accounts there can be: \(most).")) }
        var list: [ClaudeProfile] = []
        for profile in given {
            switch check(profile, home: home) {
            case .failure(let refused): return .failure(refused)
            case .success(let ok):
                guard isFolder(ok.dir) else { return .failure(Refusal(message: "\(ok.dir) isn't a folder on this Mac.")) }
                guard looksLikeAccount(ok.dir) else {
                    return .failure(Refusal(message: "\(ok.dir) doesn't look like a Claude Code folder: it has no settings.json, .claude.json or sessions."))
                }
                guard !list.contains(where: { $0.name == ok.name }) else {
                    return .failure(Refusal(message: "Two accounts are called \(ok.name). Give each its own name."))
                }
                guard !list.contains(where: { $0.dir == ok.dir }) else {
                    return .failure(Refusal(message: "\(ok.dir) is listed twice."))
                }
                list.append(ok)
            }
        }
        return .success(list)
    }

    /// Folders under `home` that look like a Claude Code config folder, ~/.claude-work say: a
    /// settings.json or .claude.json inside. Only offered, never used until
    /// confirmed. Not ~/.claude, and not what is already confirmed.
    static func found(home: URL = Paths.home, confirmed: [ClaudeProfile] = []) -> [ClaudeProfile] {
        let names = (try? FileManager.default.contentsOfDirectory(atPath: home.path)) ?? []
        return names.filter { $0.hasPrefix(".claude") && $0 != ".claude" }.sorted().compactMap { folder -> ClaudeProfile? in
            let dir = home.appendingPathComponent(folder, isDirectory: true)
            guard isFolder(dir.path),
                  ["settings.json", ".claude.json"].contains(where: { FileManager.default.fileExists(atPath: dir.appendingPathComponent($0).path) }),
                  !confirmed.contains(where: { $0.dir == dir.path })
            else { return nil }
            return ClaudeProfile(name: suggestedName(folder), dir: dir.path)
        }
    }

    /// "work" for .claude-work; a name of letters, digits and dashes.
    static func suggestedName(_ folder: String) -> String {
        let rest = folder.dropFirst(".claude".count).drop { $0 == "-" || $0 == "_" || $0 == "." }
        let name = String(rest.map { $0.isLetter || $0.isNumber || $0 == "-" || $0 == "_" ? $0 : "-" }.prefix(longestName))
        return name.isEmpty || name == defaultName ? "account" : name
    }

    /// The account named `name`; none named is the default, when it is the only one.
    static func profile(named name: String?, among profiles: [ClaudeProfile]) -> ClaudeProfile? {
        guard let name = name?.trimmingCharacters(in: .whitespaces), !name.isEmpty else {
            return profiles.count == 1 ? profiles.first : nil
        }
        return profiles.first { $0.name == name }
    }

    /// Why a hire can't go ahead on `account`: with more than one account, it
    /// must name one that is there. Nil when it can.
    static func hireRefusal(account: String?, among profiles: [ClaudeProfile]) -> (status: Int, message: String)? {
        let names = profiles.map(\.name).joined(separator: ", ")
        if let name = account?.trimmingCharacters(in: .whitespaces), !name.isEmpty {
            return profiles.contains { $0.name == name } ? nil : (404, "There's no Claude account called \(name). Pick one of: \(names).")
        }
        return profiles.count > 1 ? (400, "Say which Claude account to hire on: \(names).") : nil
    }

    /// A Claude Code config folder holds a settings.json, a .claude.json or a sessions folder.
    private static func looksLikeAccount(_ dir: String) -> Bool {
        ["settings.json", ".claude.json", "sessions"].contains { FileManager.default.fileExists(atPath: (dir as NSString).appendingPathComponent($0)) }
    }

    private static func isFolder(_ path: String) -> Bool {
        var isFolder: ObjCBool = false
        return FileManager.default.fileExists(atPath: path, isDirectory: &isFolder) && isFolder.boolValue
    }

    /// What setup and the tools are told: each account, whether its folder is
    /// there, and whether Smolcorp's hooks are in it.
    static func json(_ profiles: [ClaudeProfile], home: URL = Paths.home) -> [String: Any] {
        [
            "profiles": profiles.map { ["name": $0.name, "dir": $0.dir, "default": $0.isDefault, "connected": Hooks.isConnected($0)] as [String: Any] },
            "found": found(home: home, confirmed: profiles).map { ["name": $0.name, "dir": $0.dir] },
            "most": most,
        ]
    }
}
