import Foundation

/// The hire guard's file, `hire-guard.json` in the support folder: the groups
/// of commands every hire is refused, which you switch here. The guard
/// (`internal/guard`, run by each hire's hook) reads the same file at each
/// command, so the format is its: `{"groups": [{"name", "on", "commands"}]}`.
/// Only the page writes it (see `SetupRoutes`): a hire, or the GM, must never
/// loosen its own fence.
enum GuardFile {
    /// The groups the page switches, with what each refuses in plain words.
    /// The commands are the guard's own defaults (`guard.DefaultGroups`); a Go
    /// test checks the two lists agree.
    static let groups: [(name: String, about: String, commands: [String])] = [
        ("push", "Sending code to a remote: git push and its relatives.", [
            "git push|send-pack",
            "git subtree push",
            "git svn dcommit",
            "git lfs push",
        ]),
        ("deploy", "Putting things live: Vercel, Netlify, Docker pushes, kubectl, terraform, EAS builds.", [
            "vercel",
            "netlify",
            "wrangler",
            "firebase deploy|hosting|functions|apphosting|firestore|database|extensions|appdistribution|remoteconfig|dataconnect",
            "fly",
            "railway",
            "heroku",
            "render",
            "eas build|submit|update",
            "supabase db push",
            "supabase functions deploy",
            "kubectl apply|delete",
            "terraform apply|destroy",
            "pulumi up|destroy",
            "docker push",
            "docker image push",
            "docker compose push",
            "docker-compose push",
            "docker manifest push",
            "docker buildx build|bake --push",
            "docker build --push",
            "docker builder build --push",
            "podman push",
            "podman image push",
            "podman manifest push",
            "podman compose push",
        ]),
        ("publish", "Publishing packages and changing things on GitHub: npm publish, pull requests, releases, secrets.", [
            "npm publish|publis|publi|publ|pub|pu",
            "pnpm publish",
            "yarn publish",
            "bun publish",
            "cargo publish",
            "twine upload",
            "gem push",
            "fastlane",
            "xcrun altool|notarytool",
            "gh pr create|merge|close|edit|comment|review|ready|reopen|update-branch|revert|lock|unlock",
            "gh issue create|close|edit|comment|delete|reopen|transfer|lock|unlock|pin|unpin|develop",
            "gh release",
            "gh repo create|delete|edit|fork|rename|archive|unarchive|sync|deploy-key|autolink",
            "gh gist create|edit|delete",
            "gh label create|delete|edit|clone",
            "gh ssh-key add|delete",
            "gh gpg-key add|delete",
            "gh codespace create|delete|ssh",
            "gh extension install",
            "gh cache delete",
            "gh auth token|refresh|login|logout|setup-git",
            "gh auth status -t|--show-token",
            "npm unpublish|deprecate|dist-tag|access|owner|token|login|adduser|team|org",
            "gh workflow run|enable|disable",
            "gh run rerun|cancel|delete",
            "gh secret",
            "gh variable",
            "gh api -X|--method|-f|-F|--field|--raw-field|--input",
        ]),
        ("cloud", "Cloud accounts: aws, gcloud, az and their kin.", [
            "aws",
            "gcloud",
            "az",
            "gsutil",
            "bq",
            "cdk",
        ]),
        ("remote", "Reaching other machines: ssh, scp, sftp, rsync to a host.", [
            "ssh",
            "scp",
            "sftp",
            "rsync @remote",
        ]),
        ("local", "Driving your Mac: tmux keystrokes, the GM's commands, osascript, stored passwords.", [
            "tmux ... send-keys|send-prefix|kill-session|kill-server|kill-window|kill-pane|respawn-pane|respawn-window|pipe-pane|run-shell|if-shell|paste-buffer|source-file|command-prompt",
            "smolcorp gm hire|fire|brief|tile|watch|unwatch|rotate-token|screen",
            "security",
            "osascript",
            "git credential|credential-osxkeychain|credential-store|credential-cache|credential-manager",
            "op read|item|inject|signin",
            "bw",
            "pass",
        ]),
    ]

    /// What hires can do once a group is off, finishing "Hires will be able to …".
    /// The page shows it on a group that is off: off means allowed, for the guard
    /// and for Claude Code's deny rules alike.
    static let allows: [String: String] = [
        "push": "send code to a remote with git push.",
        "deploy": "deploy: put things live with Vercel, Netlify, Docker pushes, kubectl, terraform or EAS builds.",
        "publish": "publish packages and change things on GitHub: npm publish, pull requests, releases and secrets.",
        "cloud": "use your cloud accounts with aws, gcloud, az and the like.",
        "remote": "reach other machines with ssh, scp, sftp and rsync.",
        "local": "drive your Mac: tmux keystrokes, the GM's commands, osascript and your stored passwords.",
    ]

    /// The group the page keeps your own patterns in.
    static let extraName = "extra"
    static let maxExtra = 40
    static let maxPatternLength = 200

    /// One sentence on what the guard can't do, for the page.
    static let limits = "The guard reads the commands a hire types, so it can't stop a script the hire writes and then runs, a command built from variables, or python -c."

    struct Stored: Equatable {
        var on: [String: Bool]
        var commands: [String: [String]]
        var extra: [String]
        /// Groups the file holds that this page doesn't know: kept as they are.
        var others: [[String: Any]]

        static func == (a: Stored, b: Stored) -> Bool {
            a.on == b.on && a.commands == b.commands && a.extra == b.extra
                && NSArray(array: a.others) == NSArray(array: b.others)
        }
    }

    /// What the file holds, or the defaults and what is wrong with it. A file
    /// that isn't there is not a problem.
    static func load(_ url: URL) -> (stored: Stored, problem: String?) {
        var fresh = Stored(on: [:], commands: [:], extra: [], others: [])
        for g in groups { fresh.on[g.name] = true; fresh.commands[g.name] = g.commands }
        guard FileManager.default.fileExists(atPath: url.path) else { return (fresh, nil) }
        guard let data = try? Data(contentsOf: url) else { return (fresh, "can't read it") }
        guard let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              Set(object.keys) == ["groups"], let list = object["groups"] as? [[String: Any]], !list.isEmpty
        else { return (fresh, "it isn't a guard file") }
        var stored = fresh
        var seen = Set<String>()
        for entry in list {
            guard let name = entry["name"] as? String, !name.trimmingCharacters(in: .whitespaces).isEmpty,
                  Set(entry.keys).isSubset(of: ["name", "on", "commands"]),
                  let on = entry["on"] as? Bool, seen.insert(name).inserted
            else { return (fresh, "a group in it is wrong") }
            let commands = entry["commands"] as? [String] ?? []
            guard commands.allSatisfy({ !$0.split(whereSeparator: \.isWhitespace).isEmpty }) else { return (fresh, "a group in it has an empty command") }
            if name == extraName {
                stored.extra = commands
            } else if fresh.on[name] != nil {
                stored.on[name] = on
                stored.commands[name] = commands
            } else {
                stored.others.append(entry)
            }
        }
        return (stored, nil)
    }

    /// The file as it is written: the six groups in the guard's order, then
    /// your patterns, then any group this page doesn't know.
    static func data(_ stored: Stored) -> Data {
        var list: [[String: Any]] = groups.map {
            ["name": $0.name, "on": stored.on[$0.name] ?? true, "commands": stored.commands[$0.name] ?? $0.commands]
        }
        list.append(["name": extraName, "on": true, "commands": stored.extra])
        list += stored.others
        let object: [String: Any] = ["groups": list]
        let data = (try? JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes])) ?? Data()
        return data + Data("\n".utf8)
    }

    /// What \`GET /setup/guard\` answers.
    static func state(_ url: URL) -> [String: Any] {
        let (stored, problem) = load(url)
        var state: [String: Any] = [
            "groups": groups.map { ["name": $0.name, "on": stored.on[$0.name] ?? true, "about": $0.about, "allows": allows[$0.name] ?? "do what this group refuses."] as [String: Any] },
            "extra": stored.extra,
            "limits": limits,
        ]
        if let problem { state["problem"] = "The guard file \(problem), so every group is on until you save here." }
        return state
    }

    /// Why a pattern can't be kept, or nil. A pattern is a command name (or
    /// names with |) and then the words that must follow it, as in the
    /// default groups. One that doesn't start with a command name would
    /// match nothing, or, written with * or an empty alternative, look as if
    /// it matched everything: both are refused.
    static func problem(withPattern pattern: String) -> String? {
        if pattern.count > maxPatternLength { return "A pattern is at most \(maxPatternLength) characters." }
        if pattern.unicodeScalars.contains(where: { CharacterSet.controlCharacters.contains($0) }) { return "A pattern is one line of plain words." }
        let words = pattern.split(whereSeparator: \.isWhitespace)
        guard let first = words.first else { return "A pattern can't be empty." }
        for name in first.split(separator: "|", omittingEmptySubsequences: false) {
            let ok = name.first.map { $0.isLetter || $0.isNumber } ?? false
            if !ok || !name.allSatisfy({ ($0.isASCII && ($0.isLetter || $0.isNumber)) || "._+-".contains($0) }) {
                return "A pattern starts with the command's name, such as \"terraform import\"; \"\(first)\" isn't one."
            }
        }
        return nil
    }

    enum Refused: Error { case bad(String), notWritten }

    /// Checks a request, then writes it whole or not at all. The body is
    /// \`{"groups": [{"name", "on"}], "extra": ["pattern"]}\`; a group left out
    /// keeps what it was, and so does \`extra\` if it is left out.
    static func save(_ body: Data, to url: URL) -> Result<[String: Any], Refused> {
        guard let given = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any],
              Set(given.keys).isSubset(of: ["groups", "extra"])
        else { return .failure(.bad("Send {groups: [{name, on}], extra: [pattern]}.")) }
        var stored = load(url).stored
        if let list = given["groups"] {
            guard let list = list as? [[String: Any]] else { return .failure(.bad("groups is a list of {name, on}.")) }
            var seen = Set<String>()
            for entry in list {
                guard let name = entry["name"] as? String, let on = entry["on"] as? Bool else { return .failure(.bad("Each group is {name, on}.")) }
                guard stored.on[name] != nil else { return .failure(.bad("There is no group called \(name).")) }
                guard seen.insert(name).inserted else { return .failure(.bad("There are two groups called \(name).")) }
                stored.on[name] = on
            }
        }
        if let extra = given["extra"] {
            guard let extra = extra as? [String] else { return .failure(.bad("extra is a list of patterns.")) }
            let patterns = extra.map { $0.trimmingCharacters(in: .whitespaces) }
            guard patterns.count <= maxExtra else { return .failure(.bad("At most \(maxExtra) patterns of your own.")) }
            for pattern in patterns { if let why = problem(withPattern: pattern) { return .failure(.bad(why)) } }
            var unique: [String] = []
            for pattern in patterns where !unique.contains(pattern) { unique.append(pattern) }
            stored.extra = unique
        }
        guard Paths.writePrivately(data(stored), to: url) else { return .failure(.notWritten) }
        return .success(state(url))
    }
}
