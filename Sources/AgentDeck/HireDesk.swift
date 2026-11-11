import Foundation

/// The hire engine: what happens to a hire before its window opens and after.
/// The office, the phone, setup and (later) the General Manager all hire
/// through `Deck.hire`, which calls this, so the rules below hold for every
/// way in.
///
/// - a git repo gets a worktree and branch of its own for the hire (C01); a
///   folder that isn't a repo is hired into as it is
/// - the worktree is trusted in the hire's Claude account by editing that
///   account's config the way Claude Code keeps it (C02), never by keystrokes;
///   a config in a shape we don't know is refused
/// - a stray `apiKeyHelper` is refused (C04), unless the API-key account was picked (C23)
/// - a free range of ports per hire (M12)
/// - the hire counts as started once its tmux session is alive and a hook
///   from it has arrived (C06); otherwise it is cleaned up (C16)
///
/// Nothing here types into a window. Git and tmux go through injected
/// runners, so tests use temp repos and a stand-in tmux.
enum HireDesk {
    typealias Refusal = FrontDesk.Refusal
    /// `git -C directory arguments`: its status and output (errors included), nil if it couldn't be run.
    typealias Git = (_ directory: String, _ arguments: [String]) -> (status: Int32, output: String)?

    static let gitTool = "/usr/bin/git"
    /// The first port of the first range, and the last one a range may reach.
    static let firstPort = 4100, lastPort = 4999, portsPerHire = 4
    /// How long a hire has to show its first hook before it is called dead.
    static let startWithin: TimeInterval = 45
    /// What setup says beside the API-key choice.
    static let apiKeyNotice = "Billed to your Anthropic API key (the one saved under Summaries), not your Claude subscription. The hire can read this key."
    /// The same in a test copy, where the key is the copy's own file and the Keychain is never asked.
    static let testCopyApiKeyNotice = "Billed to the Anthropic API key in this test copy's \(TestCopySecrets.apiKeyName) file (or the one pasted under Summaries), not your Claude subscription. The hire can read this key."
    static func apiKeyNoticeText(testCopy: Bool = TestCopy.isOn) -> String { testCopy ? testCopyApiKeyNotice : apiKeyNotice }
    static let notARepo = "This folder isn't a git repository, so the hire works in the folder itself."

    /// A checkout of a big repo can take a while; everything else git does here is quick.
    static let worktreeTimeout: TimeInterval = 30

    static let git: Git = { directory, arguments in
        Tool.run(gitTool, ["-C", directory] + arguments, errors: true, timeout: arguments.starts(with: ["worktree", "add"]) ? worktreeTimeout : Tool.timeout)
    }

    /// What a hire can be asked for besides its role and folder (C15). The account and floor
    /// travel as they always did.
    struct HireOptions: Equatable {
        var model = ""
        var nickname = ""
        /// Hire on the API-key account, billed to the key in the Keychain (C23).
        var apiKey = false
        /// Asked for a worktree of its own, with the trust, ports and start check that go with it. Only the GM's
        /// route and setup's explicit choice say so; otherwise a hire is as in 1.23 unless the manager is on.
        var worktree = false
        /// Hired into T3 Code (T797b, experimental): its agent runs in a T3 Code thread, in a worktree of its own.
        var t3 = false

        /// The options as a request carries them; refuses what couldn't be a model or a nickname, or an engine but T3 Code's.
        static func checked(model: String?, nickname: String?, apiKey: Bool?, worktree: Bool? = nil, engine: Any? = nil) -> Result<HireOptions, Refusal> {
            let model = (model ?? "").trimmingCharacters(in: .whitespaces)
            let nickname = (nickname ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
            guard model.count <= 60, model.allSatisfy({ $0.isASCII && ($0.isLetter || $0.isNumber || "-_.:[]".contains($0)) }) else {
                return .failure(Refusal(status: 400, message: "A model is a name such as claude-sonnet-5-5."))
            }
            guard nickname.count <= FrontDesk.longestNickname, !nickname.contains(where: { $0.isNewline }) else {
                return .failure(Refusal(status: 400, message: "A nickname is one line of at most \(FrontDesk.longestNickname) characters."))
            }
            // Where its agent runs: a terminal, as always (left out, or ""), or T3 Code ("t3"), always in a worktree.
            let t3: Bool
            switch engine {
            case nil, is NSNull: t3 = false
            case let word as String where word.isEmpty || word == "terminal": t3 = false
            case let word as String where word == T3Desk.engine: t3 = true
            default: return .failure(Refusal(status: 400, message: "Where it works is \"t3\" (T3 Code), or left out for a terminal."))
            }
            return .success(HireOptions(model: model, nickname: nickname, apiKey: apiKey ?? false, worktree: (worktree ?? false) || t3, t3: t3))
        }
    }

    // MARK: Preparing

    struct Request {
        var hireID: String
        /// The name its worktree and branch are given: the nickname, else the role.
        var name: String
        var folder: String
        /// Claude Code is the agent: only then are trust, the account and apiKeyHelper in play.
        var claude: Bool
        var account: ClaudeProfile
        /// The API-key account was picked for this hire.
        var apiKey: Bool
        /// A repo gets a worktree for it; false works in the folder as it is (the GM, unless asked).
        var worktree = true
    }

    /// Where the hire works.
    struct Ready: Equatable {
        /// The folder the agent starts in.
        var folder: String
        var branch = ""
        var worktree = ""
        /// The repo's top folder, when there is one.
        var repo = ""
        var ports = ""
        /// Something to tell whoever hired, if any.
        var note = ""
    }

    /// The hire's place: `prepare`d when the hire is managed (the manager is on, or the hire asked for a worktree),
    /// otherwise as in 1.23, the folder itself with nothing made, trusted, checked or numbered.
    static func place(_ request: Request, managed: Bool, worktrees: String = "", taken: [String] = [], home: URL = Paths.home,
                      git: Git = HireDesk.git, portIsFree: (Int) -> Bool = HireDesk.portIsFree,
                      key: () -> String? = { SummaryKey.store.read() }) throws -> Ready {
        guard managed else { return Ready(folder: request.folder) }
        return try prepare(request, worktrees: worktrees, taken: taken, home: home, git: git, portIsFree: portIsFree, key: key)
    }

    /// Gets the hire's place ready, or refuses with nothing left behind. `worktrees`
    /// is the folder set for worktrees, "" for beside the repo. `taken` are the port
    /// ranges of hires still working.
    static func prepare(_ request: Request, worktrees: String = "", taken: [String] = [], home: URL = Paths.home,
                        git: Git = HireDesk.git, portIsFree: (Int) -> Bool = HireDesk.portIsFree,
                        key: () -> String? = { SummaryKey.store.read() }) throws -> Ready {
        let top = git(request.folder, ["rev-parse", "--show-toplevel"])
        let repo = top.flatMap { $0.status == 0 && !$0.output.isEmpty ? physical($0.output) : nil }
        let given = physical(request.folder)
        var approvedTail: String?
        // Before anything is made: the settings that would change who pays, and a config we can't edit.
        if request.claude {
            if !request.apiKey, let helper = apiKeyHelper(in: [request.folder, repo, request.account.dir].compactMap { $0 }) {
                throw Refusal(status: 409, message: "\(helper) sets an apiKeyHelper, which would bill an API key instead of the Claude account. "
                              + "Take it out, or hire on the API-key account.")
            }
            let file = trustFile(of: request.account, home: home)
            guard let data = try? Data(contentsOf: file), (try? trusting("/", in: data)) != nil else { throw trustRefusal }
            // The API-key account: Claude Code asks "use this API key?" (default No) unless the key's tail is already approved.
            if request.apiKey {
                guard let tail = keyTail(key()) else { throw keyRefusalText() }
                guard (try? approving(tail, in: data)) != nil else { throw trustRefusal }
                approvedTail = tail
            }
        }
        guard let ports = allocatePorts(taken: taken, isFree: portIsFree) else {
            throw Refusal(status: 409, message: "No free ports for a hire: fire someone, or free ports \(firstPort) to \(lastPort).")
        }
        var ready = Ready(folder: request.folder, ports: ports)
        if let repo, request.worktree {
            let name = "\(slug(request.name))-\(request.hireID.prefix(6))"
            let parent = worktrees.isEmpty
                ? URL(fileURLWithPath: repo).deletingLastPathComponent().appendingPathComponent((repo as NSString).lastPathComponent + "-hires")
                : URL(fileURLWithPath: worktrees).appendingPathComponent((repo as NSString).lastPathComponent)
            let path = parent.appendingPathComponent(name).path
            let branch = "hire/\(name)"
            try? FileManager.default.createDirectory(at: parent, withIntermediateDirectories: true)
            let made = git(repo, ["worktree", "add", "-b", branch, path])
            guard made?.status == 0 else {
                // No answer: git was stopped for taking too long, and may have left a half-made worktree.
                if made == nil { discardHalfMade(repo: repo, worktree: path, branch: branch, git: git) }
                throw Refusal(status: 409, message: "Couldn't make the hire's worktree: \(String((made?.output ?? "git didn't answer").prefix(200)))")
            }
            let inside = given.hasPrefix(repo + "/") ? String(given.dropFirst(repo.count)) : ""
            ready.repo = repo
            ready.branch = branch
            ready.worktree = physical(path)
            ready.folder = ready.worktree + inside
        } else if repo == nil {
            ready.note = notARepo
        }
        if request.claude {
            do { try preTrust(physical(ready.worktree.isEmpty ? ready.folder : ready.worktree), file: trustFile(of: request.account, home: home), approving: approvedTail) } catch {
                if !ready.worktree.isEmpty { _ = removeWorkspace(repo: ready.repo, worktree: ready.worktree, branch: ready.branch, git: git) }
                throw error
            }
        }
        return ready
    }

    /// Clears what a `worktree add` that was stopped may have left: its registration, its folder (new, named
    /// for this hire alone) and its branch.
    static func discardHalfMade(repo: String, worktree: String, branch: String, git: Git = HireDesk.git) {
        _ = git(repo, ["worktree", "remove", "--force", worktree])
        try? FileManager.default.removeItem(atPath: worktree)
        _ = git(repo, ["worktree", "prune"])
        _ = git(repo, ["branch", "-D", branch])
    }

    static let keyRefusal = Refusal(status: 409, message: "Smolcorp couldn't read your Anthropic API key from the Keychain. Save it again under Summaries, or hire on a Claude account.")

    /// How a test copy is told to put a fake key in its own file: the Keychain isn't mentioned, as the copy never asks it.
    static func testCopyKeyHelp(file: URL?) -> String {
        let named = file.map { "\($0.path)" } ?? TestCopySecrets.apiKeyName
        return "Write a fake key (sk-ant-… on one line, mode 600) into \(named), or paste one under Summaries. No restart is needed."
    }

    static func keyRefusalText(testCopy: Bool = TestCopy.isOn, file: URL? = TestCopy.support?.appendingPathComponent(TestCopySecrets.apiKeyName)) -> Refusal {
        guard testCopy else { return keyRefusal }
        return Refusal(status: 409, message: "Smolcorp couldn't read an Anthropic API key from this test copy's \(TestCopySecrets.apiKeyName) file. \(testCopyKeyHelp(file: file))")
    }

    static let trustRefusal = Refusal(status: 409, message: "Trust this folder once in Claude Code, then hire again: Smolcorp doesn't know how this account keeps its trusted folders.")

    /// Removes a worktree made for a hire, and its branch, when it is clean: nothing the hire did is lost.
    /// False when it was left, as it holds changes (or git couldn't say).
    @discardableResult
    static func removeWorkspace(repo: String, worktree: String, branch: String, git: Git = HireDesk.git) -> Bool {
        guard !repo.isEmpty, !worktree.isEmpty, let status = git(worktree, ["status", "--porcelain"]), status.status == 0, status.output.isEmpty,
              git(repo, ["worktree", "remove", worktree])?.status == 0 else { return false }
        _ = git(repo, ["branch", "-d", branch])
        return true
    }

    // MARK: Starting

    enum Start: Equatable {
        case started
        case failed(String)
    }

    /// Waits for the hire to be alive and heard from. Dead at any look is a failure at once;
    /// not heard from by `within` is one too. The clock and the pause are given, so tests don't wait.
    static func waitForStart(alive: () -> Bool?, heardFrom: () -> Bool, within: TimeInterval = startWithin, every: TimeInterval = 1,
                             now: () -> Date = Date.init, pause: (TimeInterval) -> Void = { Thread.sleep(forTimeInterval: $0) }) -> Start {
        let end = now().addingTimeInterval(within)
        while true {
            if alive() == false { return .failed("its tmux session ended before it reported") }
            if heardFrom() { return alive() == false ? .failed("its tmux session ended before it reported") : .started }
            if now() >= end { return .failed("no word from it in \(Int(within)) seconds") }
            pause(every)
        }
    }

    /// The hire still starting that a hook from `folder` (and `tab`) belongs to: by the tmux
    /// socket it runs on, else by its worktree or folder, or a folder inside either.
    static func startingHire(folder: String, tab: String, among hires: [FrontDesk.Hire]) -> FrontDesk.Hire? {
        let starting = hires.filter { $0.state == .starting }
        if let socket = TabAddress(tab)?.socket, !socket.isEmpty, let hire = starting.first(where: { $0.tmuxSocket == socket }) { return hire }
        let here = real(folder)
        return starting.first { hire in
            [hire.worktree, hire.folder].filter { !$0.isEmpty }.map(real).contains { here == $0 || here.hasPrefix($0 + "/") }
        }
    }

    /// Ends a hire that never started: its tmux server, then its worktree when it is new and clean,
    /// then the registry says it failed and why. Says what it left behind, if anything.
    @discardableResult
    static func cleanUp(_ hire: FrontDesk.Hire, reason: String, tmux: HireTmux?, git: Git = HireDesk.git, desk: FrontDesk) -> String {
        if let tmux, tmux.isAlive() != false { _ = tmux.kill() }
        var left = ""
        if !hire.worktree.isEmpty, !removeWorkspace(repo: hire.repo.isEmpty ? hire.folder : hire.repo, worktree: hire.worktree, branch: hire.branch, git: git) {
            left = " Its worktree \(hire.worktree) was kept: it has changes, or git wouldn't remove it."
        }
        desk.fail(hire.id, reason: "Didn't start: \(reason).\(left)")
        return left
    }

    // MARK: Ports

    /// The first range of `size` ports that no taken range touches and that are all free.
    /// Ranges are written "4100-4103".
    static func allocatePorts(taken: [String], size: Int = portsPerHire, first: Int = firstPort, last: Int = lastPort,
                              isFree: (Int) -> Bool) -> String? {
        let used = taken.compactMap { text -> ClosedRange<Int>? in
            let parts = text.split(separator: "-").compactMap { Int($0) }
            return parts.count == 2 && parts[0] <= parts[1] ? parts[0]...parts[1] : nil
        }
        var start = first
        while start + size - 1 <= last {
            let range = start...(start + size - 1)
            if let clash = used.first(where: { $0.overlaps(range) }) { start = clash.upperBound + 1; continue }
            if let busy = range.first(where: { !isFree($0) }) { start = busy + 1; continue }
            return "\(range.lowerBound)-\(range.upperBound)"
        }
        return nil
    }

    /// Whether nothing is listening on `port` on this computer.
    static func portIsFree(_ port: Int) -> Bool {
        let descriptor = socket(AF_INET, SOCK_STREAM, 0)
        guard descriptor >= 0 else { return false }
        defer { close(descriptor) }
        var address = sockaddr_in()
        address.sin_family = sa_family_t(AF_INET)
        address.sin_port = in_port_t(port).bigEndian
        address.sin_addr.s_addr = inet_addr("127.0.0.1")
        return withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { bind(descriptor, $0, socklen_t(MemoryLayout<sockaddr_in>.size)) == 0 }
        }
    }

    // MARK: The API-key account (C23)

    /// Whether the choice is offered: only with a key in the Keychain (not just the environment).
    static func apiKeyOffered() -> Bool { SummaryKey.shown()?.source == .keychain }

    /// Why the API-key account can't be used for a hire, or nil.
    static func apiKeyRefusal(picked: Bool, offered: Bool, testCopy: Bool = TestCopy.isOn,
                              file: URL? = TestCopy.support?.appendingPathComponent(TestCopySecrets.apiKeyName)) -> Refusal? {
        guard picked && !offered else { return nil }
        if testCopy {
            return Refusal(status: 409, message: "There's no Anthropic API key in this test copy's \(TestCopySecrets.apiKeyName) file. \(testCopyKeyHelp(file: file))")
        }
        return Refusal(status: 409, message: "There's no Anthropic API key in your Keychain. Save one under Summaries, or hire on a Claude account.")
    }

    /// The start script's lines for a hire: its ports, and, on the API-key account, the key read from the
    /// Keychain as the script runs (never written down; in a test copy, from its own key file). If the key can't be read it stops, and never
    /// falls back to the subscription.
    static func scriptLines(ports: String, apiKey: Bool, gmToken: String? = nil, sendKey: String = "",
                            copyPort: UInt16? = TestCopy.port, copySupport: URL? = TestCopy.support,
                            keyFile: URL? = TestCopy.support?.appendingPathComponent(TestCopySecrets.apiKeyName)) -> String {
        var lines = ports.isEmpty ? "" : "export SMOLCORP_HIRE_PORTS=\(shellQuoted(ports))\n"
        // In a test copy the `smolcorp` command talks to the copy, and reads the copy's folder, never the real app's.
        if let copyPort { lines += copyEnvironment(port: copyPort, support: copySupport) }
        // The General Manager's token: its own environment (and its children's), in a script that removes itself as it starts.
        if let gmToken { lines += "export \(GMIdentity.environment)=\(shellQuoted(gmToken))\n" }
        if !sendKey.isEmpty { lines += "export \(FrontDesk.sendKeyEnvironment)=\(shellQuoted(sendKey))\n" }
        if apiKey, copyPort != nil {
            // A test copy never asks the Keychain: the key is in the copy's own file (the script gets its path, not the key).
            let file = keyFile.map { shellQuoted($0.path) } ?? "''"
            lines += "ANTHROPIC_API_KEY=\"$(cat \(file) 2>/dev/null)\"\n"
                + "[ -n \"$ANTHROPIC_API_KEY\" ] || { echo \"Smolcorp couldn't read the API key from the test copy's \(TestCopySecrets.apiKeyName) file.\"; exit 1; }\n"
                + "export ANTHROPIC_API_KEY\n"
        } else if apiKey {
            lines += "ANTHROPIC_API_KEY=\"$(/usr/bin/security find-generic-password -s \(shellQuoted(SummaryKey.service)) "
                + "-a \(shellQuoted(SummaryKey.account)) -w)\" || { echo \"Smolcorp couldn't read the API key from the Keychain.\"; exit 1; }\n"
                + "export ANTHROPIC_API_KEY\n"
        }
        return lines
    }

    /// The lines (as `export`s) that point the `smolcorp` command at a test copy: its port, and its support folder.
    static func copyEnvironment(port: UInt16, support: URL?) -> String {
        var lines = "export SMOLCORP_PORT=\(shellQuoted(String(port)))\n"
        if let support { lines += "export SMOLCORP_TEST_SUPPORT_DIR=\(shellQuoted(support.path))\n" }
        return lines
    }

    /// The same as a prefix for one command line (a hire with no start script): `NAME='x' NAME='y' `.
    static func copyPrefix(port: UInt16, support: URL?) -> String {
        var words = "SMOLCORP_PORT=\(shellQuoted(String(port))) "
        if let support { words += "SMOLCORP_TEST_SUPPORT_DIR=\(shellQuoted(support.path)) " }
        return words
    }

    private static func shellQuoted(_ word: String) -> String { "'" + word.replacingOccurrences(of: "'", with: "'\\''") + "'" }

    // MARK: Names and paths

    /// Lower case letters, digits and dashes: a folder or branch name's part. "hire" for nothing usable.
    static func slug(_ text: String) -> String {
        let lowered = text.lowercased().map { $0.isASCII && ($0.isLetter || $0.isNumber) ? $0 : "-" }
        let joined = String(lowered).split(separator: "-").joined(separator: "-")
        return joined.isEmpty ? "hire" : String(joined.prefix(30))
    }

    /// `path` with symlinks resolved, /tmp as /private/tmp: how Claude Code knows a folder.
    static func physical(_ path: String) -> String {
        guard let resolved = realpath(path, nil) else { return path }
        defer { free(resolved) }
        return String(cString: resolved)
    }

    private static func real(_ path: String) -> String { (physical(path) as NSString).standardizingPath }

    // MARK: Trust (C02)

    /// The file an account keeps its trusted folders in: ~/.claude.json for ~/.claude, else the account's own.
    static func trustFile(of profile: ClaudeProfile, home: URL = Paths.home) -> URL {
        profile.isDefault ? home.appendingPathComponent(".claude.json") : URL(fileURLWithPath: profile.dir).appendingPathComponent(".claude.json")
    }

    /// `data`, an account's .claude.json, with `folder` trusted: only projects[folder].hasTrustDialogAccepted
    /// set, by editing the text in that one place, so every other byte stays as Claude Code wrote it.
    /// Throws for a shape that isn't the one Claude Code writes.
    static func trusting(_ folder: String, in data: Data) throws -> Data {
        guard let text = String(data: data, encoding: .utf8), let root = JSONText.value(in: text, at: text.startIndex),
              text[root.start] == "{" else { throw trustRefusal }
        let key = JSONText.quoted(folder)
        let edited: String
        if let projects = JSONText.member("projects", of: root, in: text) {
            guard text[projects.start] == "{" else { throw trustRefusal }
            if let project = JSONText.member(folder, of: projects, in: text) {
                guard text[project.start] == "{" else { throw trustRefusal }
                if let trust = JSONText.member("hasTrustDialogAccepted", of: project, in: text) {
                    edited = text.replacingCharacters(in: trust.start..<trust.end, with: "true")
                } else {
                    edited = JSONText.insert("\"hasTrustDialogAccepted\": true", into: project, in: text)
                }
            } else {
                edited = JSONText.insert("\(key): {\"hasTrustDialogAccepted\": true}", into: projects, in: text)
            }
        } else {
            edited = JSONText.insert("\"projects\": {\(key): {\"hasTrustDialogAccepted\": true}}", into: root, in: text)
        }
        let out = Data(edited.utf8)
        guard onlyTrustChanged(before: data, after: out, folder: folder) else { throw trustRefusal }
        return out
    }

    /// Whether `after` is `before` with only `folder`'s trust set.
    static func onlyTrustChanged(before: Data, after: Data, folder: String) -> Bool {
        guard let old = try? JSONSerialization.jsonObject(with: before) as? [String: Any],
              var new = try? JSONSerialization.jsonObject(with: after) as? [String: Any],
              var projects = new["projects"] as? [String: Any],
              var project = projects[folder] as? [String: Any], project["hasTrustDialogAccepted"] as? Bool == true else { return false }
        let oldProject = (old["projects"] as? [String: Any])?[folder] as? [String: Any]
        project["hasTrustDialogAccepted"] = oldProject?["hasTrustDialogAccepted"]
        projects[folder] = oldProject == nil ? nil : project
        new["projects"] = old["projects"] == nil && projects.isEmpty ? nil : projects
        return JSONText.same(old, new)
    }

    // MARK: The API key's answer

    /// The part of a key Claude Code records its answer under: the last 20 characters (it trims first). Nil for no key.
    static func keyTail(_ key: String?) -> String? {
        guard let trimmed = key?.trimmingCharacters(in: .whitespacesAndNewlines), !trimmed.isEmpty else { return nil }
        return String(trimmed.suffix(20))
    }

    /// `data`, an account's .claude.json, with `tail` among `customApiKeyResponses.approved` (Claude Code's "yes" to
    /// "use this API key?"): by editing the text in that one place, so every other byte stays as Claude Code wrote it.
    /// Already there, it is left as it is. Throws for a shape that isn't the one Claude Code writes.
    static func approving(_ tail: String, in data: Data) throws -> Data {
        guard let text = String(data: data, encoding: .utf8), let root = JSONText.value(in: text, at: text.startIndex),
              text[root.start] == "{" else { throw trustRefusal }
        let quoted = JSONText.quoted(tail)
        let edited: String
        if let responses = JSONText.member("customApiKeyResponses", of: root, in: text) {
            guard text[responses.start] == "{" else { throw trustRefusal }
            if let approved = JSONText.member("approved", of: responses, in: text) {
                guard text[approved.start] == "[" else { throw trustRefusal }
                if let list = try? JSONSerialization.jsonObject(with: Data(text[approved.start..<approved.end].utf8)) as? [Any],
                   list.contains(where: { ($0 as? String) == tail }) { return data }
                edited = JSONText.insert(quoted, into: approved, in: text)
            } else {
                edited = JSONText.insert("\"approved\": [\(quoted)]", into: responses, in: text)
            }
        } else {
            edited = JSONText.insert("\"customApiKeyResponses\": {\"approved\": [\(quoted)]}", into: root, in: text)
        }
        let out = Data(edited.utf8)
        guard onlyApprovalChanged(before: data, after: out, tail: tail) else { throw trustRefusal }
        return out
    }

    /// Whether `after` is `before` with only `tail` added, first, to `customApiKeyResponses.approved`.
    static func onlyApprovalChanged(before: Data, after: Data, tail: String) -> Bool {
        guard let old = try? JSONSerialization.jsonObject(with: before) as? [String: Any],
              var new = try? JSONSerialization.jsonObject(with: after) as? [String: Any],
              var responses = new["customApiKeyResponses"] as? [String: Any],
              let approved = responses["approved"] as? [Any], approved.first as? String == tail else { return false }
        if let oldResponses = old["customApiKeyResponses"] as? [String: Any] {
            let oldApproved = oldResponses["approved"] as? [Any]
            responses["approved"] = oldApproved == nil ? nil : Array(approved.dropFirst())
            new["customApiKeyResponses"] = responses
        } else {
            new["customApiKeyResponses"] = nil
        }
        return JSONText.same(old, new)
    }

    /// Whether `tail` is among the approved keys in `file`.
    static func isApproved(_ tail: String, file: URL) -> Bool {
        guard let data = try? Data(contentsOf: file), let root = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return false }
        return ((root["customApiKeyResponses"] as? [String: Any])?["approved"] as? [Any])?.contains { ($0 as? String) == tail } == true
    }

    /// Trusts `folder` in `file` (and, for an API-key hire, approves its key's tail), written whole in one go and read back. Claude Code sessions rewrite this
    /// file all the time, so if one wrote over the change it is made once more from the file as it is then;
    /// no copy is ever put back over theirs. Nothing in the file is logged or said back.
    static func preTrust(_ folder: String, file link: URL, approving tail: String? = nil) throws {
        // A config that is a symlink (a dotfiles manager's) is written through to its target, not replaced.
        let file = link.resolvingSymlinksInPath()
        guard var before = try? Data(contentsOf: file) else { throw trustRefusal }
        for _ in 0..<2 {
            var written = try trusting(folder, in: before)
            if let tail { written = try approving(tail, in: written) }
            guard Paths.writePrivately(written, to: file) else { throw Refusal(status: 500, message: "Couldn't write \(file.path).") }
            if isTrusted(folder, file: file), tail.map({ isApproved($0, file: file) }) ?? true { return }
            guard let now = try? Data(contentsOf: file) else { break }
            before = now
        }
        throw Refusal(status: 500, message: "\(file.lastPathComponent) lost the change twice as it was written: another Claude Code session keeps rewriting it. Hire again in a moment.")
    }

    static func isTrusted(_ folder: String, file: URL) -> Bool {
        guard let data = try? Data(contentsOf: file), let root = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return false }
        return ((root["projects"] as? [String: Any])?[folder] as? [String: Any])?["hasTrustDialogAccepted"] as? Bool == true
    }

    // MARK: apiKeyHelper (C04)

    /// The first settings file among those of `places` (folders with a .claude/ inside, or an account's
    /// own folder) that sets an apiKeyHelper, which would bill a key rather than the account; nil if none.
    static func apiKeyHelper(in places: [String]) -> String? {
        for place in places {
            for name in [".claude/settings.json", ".claude/settings.local.json", "settings.json", "settings.local.json"] {
                let file = URL(fileURLWithPath: place).appendingPathComponent(name)
                if let data = try? Data(contentsOf: file), let root = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                   root["apiKeyHelper"] != nil { return file.path }
            }
        }
        return nil
    }
}

/// Just enough of a JSON reader to find where a value sits in the text, so one value can be
/// changed and every other byte kept, for the trust write into an account's .claude.json.
enum JSONText {
    struct Span {
        var start: String.Index
        var end: String.Index
    }

    static func skipSpace(_ text: String, _ i: String.Index) -> String.Index {
        var i = i
        while i < text.endIndex, " \n\r\t".contains(text[i]) { i = text.index(after: i) }
        return i
    }

    private static func stringEnd(_ text: String, _ i: String.Index) -> String.Index? {
        var i = text.index(after: i)
        while i < text.endIndex {
            if text[i] == "\\" {
                i = text.index(after: i)
                guard i < text.endIndex else { return nil }
            } else if text[i] == "\"" {
                return text.index(after: i)
            }
            i = text.index(after: i)
        }
        return nil
    }

    /// The value starting at `i`, after any space; nil if it isn't whole.
    static func value(in text: String, at i: String.Index) -> Span? {
        let start = skipSpace(text, i)
        guard start < text.endIndex else { return nil }
        switch text[start] {
        case "\"":
            return stringEnd(text, start).map { Span(start: start, end: $0) }
        case "{", "[":
            let close: Character = text[start] == "{" ? "}" : "]"
            var i = skipSpace(text, text.index(after: start))
            if i < text.endIndex, text[i] == close { return Span(start: start, end: text.index(after: i)) }
            while i < text.endIndex {
                if close == "}" {
                    guard text[i] == "\"", let keyEnd = stringEnd(text, i) else { return nil }
                    i = skipSpace(text, keyEnd)
                    guard i < text.endIndex, text[i] == ":" else { return nil }
                    i = text.index(after: i)
                }
                guard let member = value(in: text, at: i) else { return nil }
                i = skipSpace(text, member.end)
                guard i < text.endIndex else { return nil }
                if text[i] == close { return Span(start: start, end: text.index(after: i)) }
                guard text[i] == "," else { return nil }
                i = skipSpace(text, text.index(after: i))
            }
            return nil
        default:
            var i = start
            while i < text.endIndex, !",}] \n\r\t".contains(text[i]) { i = text.index(after: i) }
            return i == start ? nil : Span(start: start, end: i)
        }
    }

    /// The value of the member `name` directly in the object at `object`.
    static func member(_ name: String, of object: Span, in text: String) -> Span? {
        var i = skipSpace(text, text.index(after: object.start))
        while i < object.end, text[i] == "\"" {
            guard let keyEnd = stringEnd(text, i) else { return nil }
            let key = try? JSONSerialization.jsonObject(with: Data(text[i..<keyEnd].utf8), options: .fragmentsAllowed) as? String
            var j = skipSpace(text, keyEnd)
            guard j < text.endIndex, text[j] == ":" else { return nil }
            j = text.index(after: j)
            guard let found = value(in: text, at: j) else { return nil }
            if key == name { return found }
            i = skipSpace(text, found.end)
            guard i < object.end, text[i] == "," else { return nil }
            i = skipSpace(text, text.index(after: i))
        }
        return nil
    }

    /// The text with `member` ("key": value) first in the object at `object`, laid out as its own first member is.
    static func insert(_ member: String, into object: Span, in text: String) -> String {
        let afterBrace = text.index(after: object.start)
        let first = skipSpace(text, afterBrace)
        let space = String(text[afterBrace..<first])
        if text[first] == "}" || text[first] == "]" { return text.replacingCharacters(in: afterBrace..<afterBrace, with: member) }
        return text.replacingCharacters(in: afterBrace..<first, with: space + member + "," + space)
    }

    /// `text` as a JSON string, with its slashes as they are.
    static func quoted(_ text: String) -> String {
        let data = (try? JSONSerialization.data(withJSONObject: text, options: [.fragmentsAllowed, .withoutEscapingSlashes])) ?? Data()
        return String(decoding: data, as: UTF8.self)
    }

    /// Whether two parsed JSON values are the same, numbers by value.
    static func same(_ a: Any, _ b: Any) -> Bool {
        switch (a, b) {
        case let (a as [String: Any], b as [String: Any]):
            return a.count == b.count && a.allSatisfy { key, value in b[key].map { same(value, $0) } ?? false }
        case let (a as [Any], b as [Any]):
            return a.count == b.count && zip(a, b).allSatisfy { same($0, $1) }
        case let (a as NSNumber, b as NSNumber):
            let aBool = CFGetTypeID(a) == CFBooleanGetTypeID(), bBool = CFGetTypeID(b) == CFBooleanGetTypeID()
            return aBool == bBool && (aBool ? a.boolValue == b.boolValue : a.doubleValue == b.doubleValue)
        case let (a as String, b as String):
            return a == b
        case (is NSNull, is NSNull):
            return true
        default:
            return false
        }
    }
}
