import AppKit
import CoreServices

/// What a first General Manager hire needs, checked before Start here's Hire
/// button: `GET /setup/hire/gm/preflight`, the same JSON as the Linux/Windows
/// app's (desktop/gmpreflight.go). Each line says what's wrong in one
/// sentence and exactly what to do: a command to copy, a fix to press, or
/// where to go on the page. Only looks: the fixes are the page's buttons.
///
/// Signed in is what Claude Code's own `claude auth status` says, asked as the
/// hire starts it (your login shell, the account's config folder): no
/// credential, key or token is ever read. Signed out is said, never in the
/// way: the GM's window asks you to sign in.
enum GMPreflight {
    struct Action: Equatable {
        /// "copy" (a command to paste in Terminal), "fix" (a POST /setup/fix by name), "folder" (the folder step above)
        /// or "account" (the account picker beside Hire when it's shown, otherwise General's Claude accounts).
        var kind: String
        var label: String
        var command: String?
        var fix: String?

        var jsonObject: [String: Any] {
            var object: [String: Any] = ["kind": kind, "label": label]
            if let command { object["command"] = command }
            if let fix { object["fix"] = fix }
            return object
        }
    }

    struct Item: Equatable {
        let id: String
        var ok: Bool
        var title: String
        /// What to do, "" when there's nothing to.
        var fix = ""
        var action: Action?
        /// Not in the way of the hire: it starts without it.
        var optional = false
        /// A step not taken yet, not a problem: the page shows it as To do, never in red.
        var todo = false

        var jsonObject: [String: Any] {
            ["id": id, "ok": ok, "title": title, "fix": fix, "action": action?.jsonObject ?? NSNull(), "optional": optional, "todo": todo]
        }
    }

    enum SignIn: Equatable { case yes, no, unknown }
    enum Automation: Equatable { case allowed, denied, unknown }

    /// What was found, gathered by `facts` and turned into lines by `items`, so tests give their own.
    struct Facts {
        /// The folder for new hires as set ("" when it isn't), and whether it's a folder.
        var folder = ""
        var folderExists = false
        /// Where the app finds Claude Code, links left as they are, nil nowhere; whether your login shell,
        /// where hires start, finds it (nil: it didn't say).
        var claudeAt: String?
        var claudeOnLoginPath: Bool?
        var shell = "/bin/zsh"
        var signIn = SignIn.unknown
        /// Why Claude Code doesn't report to Smolcorp, nil when it does.
        var hooks: String?
        /// The terminal hires open in, and whether macOS lets Smolcorp drive it.
        var terminal = "Terminal"
        var automation = Automation.unknown
        var tmuxWanted = false
        var tmux = false
        var home = NSHomeDirectory()
        /// A test copy: its hires report through its own hook, whatever your agents' settings hold.
        var testCopy = false
        /// The Claude account the hire would work on ("default" for ~/.claude's), nil while none is picked among several;
        /// and whether an account besides the default is listed. Only a test copy has a line for them.
        var account: String?
        var otherAccounts = false
    }

    /// The fix that opens Privacy & Security's Automation pane.
    static let automationFix = "automation"
    static let automationPane = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_Automation")!
    /// Claude Code's own installer.
    static let installCommand = "curl -fsSL https://claude.ai/install.sh | bash"

    /// A test copy's General Manager never works on your default Claude account (`Deck.testCopyAccountRefusal`):
    /// said before Hire, with what to do. The page matches these words for the refusal's button.
    static let needsAccount = "Your General Manager needs a Claude account of its own (this is a test copy)."
    static let addAccountFix = "Add one under Claude accounts: sign in to a second account there, then add it."
    static let chooseAccountFix = "Choose one other than default, beside Hire your General Manager below."

    /// The lines, in the order they're put right.
    static func items(_ f: Facts) -> [Item] {
        var items: [Item] = []
        let folder = tilde(f.folder, home: f.home)
        if f.folder.isEmpty {
            items.append(Item(id: "folder", ok: false, title: "There's no folder for your hires yet.", fix: "Choose it in the first step above.",
                              action: Action(kind: "folder", label: "Show me"), todo: true))
        } else if !f.folderExists {
            items.append(Item(id: "folder", ok: false, title: "The folder for your hires isn't there any more: \(folder).",
                              fix: "Choose it again in the first step above.", action: Action(kind: "folder", label: "Show me")))
        } else {
            items.append(Item(id: "folder", ok: true, title: "Your hires work in \(folder)."))
        }
        if f.claudeAt == nil && f.claudeOnLoginPath != true {
            items.append(Item(id: "claude", ok: false, title: "Claude Code isn't installed on this Mac.",
                              fix: "Paste this in Terminal, wait for it to finish, then press Check again.",
                              action: Action(kind: "copy", label: "Copy", command: installCommand), todo: true))
            return items + accountItems(f) + terminalItems(f)
        }
        // Installed, but where your login shell, which hires start in, doesn't look.
        if f.claudeOnLoginPath == false, let at = f.claudeAt {
            let where_ = (at as NSString).deletingLastPathComponent
            items.append(Item(id: "claude", ok: false, title: "Claude Code is in \(tilde(where_, home: f.home)), where Terminal doesn't look for it.",
                              fix: "Paste this in Terminal, open a new Terminal window, then press Check again.",
                              action: Action(kind: "copy", label: "Copy", command: pathLine(shell: f.shell, folder: where_, home: f.home))))
            return items + accountItems(f) + terminalItems(f)
        }
        items.append(Item(id: "claude", ok: true, title: "Claude Code is installed."))
        switch f.signIn {
        case .yes: items.append(Item(id: "signin", ok: true, title: "Claude Code is signed in."))
        case .no:
            // Said, never in the way: an API key or a cloud provider can work where this can't see it.
            items.append(Item(id: "signin", ok: false, title: "Claude Code isn't signed in.",
                              fix: "Hire anyway: its window asks you to sign in. Or sign in first: paste this in Terminal (type /login if it doesn't ask), then press Check again.",
                              action: Action(kind: "copy", label: "Copy", command: "claude"), optional: true))
        case .unknown:
            items.append(Item(id: "signin", ok: false, title: "Smolcorp couldn't tell if Claude Code is signed in.",
                              fix: "You can hire anyway: if your General Manager asks you to sign in, type /login in its window.", optional: true))
        }
        if f.testCopy {
            // The self-test's hooks line says the same, ticked too (SelfTest.hooksCheck): one answer for a test copy.
            items.append(Item(id: "hooks", ok: true, title: SelfTest.testCopyHires))
        } else if f.hooks != nil {
            items.append(Item(id: "hooks", ok: false, title: "Claude Code doesn't report to Smolcorp yet, so your General Manager would never reach a desk.",
                              fix: "Press Connect, then Check again.", action: Action(kind: "fix", label: "Connect", fix: SelfTest.repair), todo: true))
        } else {
            items.append(Item(id: "hooks", ok: true, title: "Claude Code reports to Smolcorp."))
        }
        return items + accountItems(f) + terminalItems(f)
    }

    /// A test copy's account line: To do while the hire would be on the default account (it's refused there), ticked
    /// with the account's name once another is picked. The real app hires on the default account: no line.
    /// Linux/Windows keeps its line red (!), as there it can't be put right: a test copy there never hires Claude Code.
    private static func accountItems(_ f: Facts) -> [Item] {
        guard f.testCopy else { return [] }
        if let name = f.account, name != ClaudeProfiles.defaultName {
            return [Item(id: "account", ok: true, title: "Your General Manager works on the Claude account \(name).")]
        }
        return [Item(id: "account", ok: false, title: needsAccount, fix: f.otherAccounts ? chooseAccountFix : addAccountFix,
                     action: Action(kind: "account", label: f.otherAccounts ? "Choose an account" : "Add an account"), todo: true)]
    }

    private static func terminalItems(_ f: Facts) -> [Item] {
        var items: [Item] = []
        switch f.automation {
        case .allowed: items.append(Item(id: "terminal", ok: true, title: "Smolcorp may open \(f.terminal) for your hires."))
        case .unknown:
            items.append(Item(id: "terminal", ok: true, title: "Your hires open in \(f.terminal). If your Mac asks to let Smolcorp control it, choose OK."))
        case .denied:
            items.append(Item(id: "terminal", ok: false, title: "Your Mac doesn't let Smolcorp open \(f.terminal), so your General Manager can't start.",
                              fix: "In System Settings, Privacy & Security, Automation: turn on \(f.terminal) under Smolcorp. Then press Check again.",
                              action: Action(kind: "fix", label: "Open System Settings", fix: automationFix)))
        }
        if f.tmuxWanted {
            items.append(f.tmux ? Item(id: "tmux", ok: true, title: "tmux is installed.")
                : Item(id: "tmux", ok: false, title: "tmux isn't installed, so your hires open without it.",
                       fix: "You can hire anyway, in a plain \(f.terminal) window. For tmux: paste this in Terminal (it needs Homebrew), then quit and reopen Smolcorp.",
                       action: Action(kind: "copy", label: "Copy", command: "brew install tmux"), optional: true))
        }
        return items
    }

    /// `{ready, items}`: ready when nothing but an optional line is left.
    static func report(_ items: [Item]) -> [String: Any] {
        ["ready": items.allSatisfy { $0.ok || $0.optional }, "items": items.map(\.jsonObject)]
    }

    /// The command that adds `folder` to your login shell's PATH, in the file that shell reads.
    /// A folder with a quote, a $ or a space in its name still gives a line that works (Linux/Windows: the same).
    static func pathLine(shell: String, folder: String, home: String) -> String {
        let name = (shell as NSString).lastPathComponent
        if name == "fish" {
            if folder.allSatisfy({ plain.contains($0) }) { return "fish_add_path \(tilde(folder, home: home))" }
            return "fish_add_path '" + folder.replacingOccurrences(of: "\\", with: "\\\\").replacingOccurrences(of: "'", with: "\\'") + "'"
        }
        let file: String
        switch name {
        case "zsh": file = "~/.zprofile"
        case "bash":
            file = [".bash_profile", ".bash_login"].first { FileManager.default.fileExists(atPath: (home as NSString).appendingPathComponent($0)) }
                .map { "~/" + $0 } ?? "~/.profile"
        default: file = "~/.profile"
        }
        let underHome = folder.hasPrefix(home + "/")
        // In the profile, the folder sits inside "…"; on the command line, that line sits inside '…'.
        var rest = underHome ? String(folder.dropFirst(home.count + 1)) : folder
        for (from, to) in [("\\", "\\\\"), ("\"", "\\\""), ("$", "\\$"), ("`", "\\`")] { rest = rest.replacingOccurrences(of: from, with: to) }
        let line = "export PATH=\"\(underHome ? "$HOME/" : "")\(rest):$PATH\""
        return "echo '" + line.replacingOccurrences(of: "'", with: "'\\''") + "' >> \(file)"
    }

    /// What a folder can have without quoting it in a shell.
    private static let plain = Set("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-+~@%:,")

    static func tilde(_ path: String, home: String) -> String {
        path == home ? "~" : path.hasPrefix(home + "/") ? "~" + path.dropFirst(home.count) : path
    }

    // MARK: Looking

    /// The checks for a hire on `account` ("" the default), answered on the main queue.
    /// The config and the hooks are read here, on the main queue; the shell, Claude Code and macOS are asked off it.
    static func check(config: Config, account: String, done: @escaping ([String: Any]) -> Void) {
        let profiles = ClaudeProfiles.current
        let profile = ClaudeProfiles.profile(named: account, among: profiles)
        var f = Facts()
        f.folder = config.hireFolder.trimmingCharacters(in: .whitespaces)
        f.folderExists = Deck.folder(f.folder) != nil
        f.hooks = Deck.hooksRefusal(for: .claude, found: TestCopy.isOn ? [] : Hooks.found,
                                    connected: { agent in agent == .claude ? profile.map(Hooks.isConnected) ?? Hooks.isConnected(agent) : Hooks.isConnected(agent) })
        let terminal = Openers.isInstalled(config.hireTerminal) ? config.hireTerminal : "terminal"
        let choice = Openers.choices.first { $0.id == terminal }
        f.terminal = choice?.name ?? "Terminal"
        f.tmuxWanted = config.hireInTmux
        f.tmux = Tmux.path != nil
        f.testCopy = TestCopy.isOn
        f.account = profile?.name
        f.otherAccounts = profiles.contains { !$0.isDefault }
        f.shell = ProcessInfo.processInfo.environment["SHELL"].flatMap { $0.isEmpty ? nil : $0 } ?? "/bin/zsh"
        let bundle = choice?.bundle
        DispatchQueue.global(qos: .userInitiated).async {
            look(&f, profile, bundle)
            let answer = report(items(f))
            DispatchQueue.main.async { done(answer) }
        }
    }

    /// What's asked off the main queue (your login shell, Claude Code, macOS) for `profile` and the terminal's
    /// `bundle`, filled into the facts: a variable, so tests drive the route without any of them.
    static var look: (inout Facts, ClaudeProfile?, String?) -> Void = { f, profile, bundle in
        let login = OpenCodeInstall.loginShellPath()
        // Where it was found, not where its link leads: that folder is the one to add to a PATH.
        let found = OpenCodeInstall.place(named: "claude")
        f.claudeAt = found?.path
        f.claudeOnLoginPath = login.isEmpty ? nil : login.contains { FileManager.default.isExecutableFile(atPath: ($0 as NSString).appendingPathComponent("claude")) }
        if let found, f.claudeOnLoginPath != false { f.signIn = signIn(found, profile: profile, shell: f.shell) }
        f.automation = bundle.map { automation(bundle: $0) } ?? .unknown
    }

    /// What `claude auth status --json` says, asked as the hire starts Claude Code: in your login shell
    /// (so with what its files set: an API key, a cloud provider), on `profile`'s config folder.
    /// Linux/Windows asks the same way (gmpreflight.go's claudeSignIn).
    static func signIn(_ program: URL, profile: ClaudeProfile?, shell: String) -> SignIn {
        let ran = SummaryCLI.spawn(URL(fileURLWithPath: shell), signInArguments(program: program.path, profile: profile),
                                   input: Data(), environment: hireEnvironment(), folder: FileManager.default.temporaryDirectory.path, timeout: 10)
        return signIn(status: ran.output)
    }

    /// The login shell's arguments: the account's config folder set, or unset for ~/.claude's, as a hire's start script does (Role.startScript).
    static func signInArguments(program: String, profile: ClaudeProfile?) -> [String] {
        let account = profile.map { $0.isDefault ? ["-u", "CLAUDE_CONFIG_DIR"] : ["CLAUDE_CONFIG_DIR=\($0.dir)"] } ?? ["-u", "CLAUDE_CONFIG_DIR"]
        let words = ["env"] + account + [program, "auth", "status", "--json"]
        return ["-lic", "exec " + words.map { "'" + $0.replacingOccurrences(of: "'", with: "'\\''") + "'" }.joined(separator: " ")]
    }

    /// Smolcorp's environment less what says where it was started, as a hire's terminal never has it
    /// (Linux/Windows: hireEnvironment). Keys and cloud providers stay: the hire has them too.
    static func hireEnvironment(_ base: [String: String] = ProcessInfo.processInfo.environment) -> [String: String] {
        let left: Set<String> = ["SMOLCORP_SESSION", "SAUZZEDECK_SESSION", "SMOLCORP_HEADLESS", "SMOLCORP_PARENT_FOLDER", "SMOLCORP_GM_TOKEN",
                                 "SMOLCORP_SUMMARY", "TMUX", "TMUX_PANE", "WEZTERM_PANE", "WEZTERM_UNIX_SOCKET", "KITTY_WINDOW_ID", "KITTY_LISTEN_ON"]
        return base.filter { !left.contains($0.key.uppercased()) }
    }

    /// What `claude auth status --json` printed, after whatever your shell's files print as it starts.
    static func signIn(status: Data) -> SignIn {
        let text = String(decoding: status, as: UTF8.self)
        var starts = text.hasPrefix("{") ? [text.startIndex] : []
        var from = text.startIndex
        while let found = text.range(of: "\n{", range: from..<text.endIndex) {
            starts.append(text.index(after: found.lowerBound))
            from = found.upperBound
        }
        for start in starts {
            if let object = (try? JSONSerialization.jsonObject(with: Data(text[start...].utf8))) as? [String: Any],
               let loggedIn = object["loggedIn"] as? Bool { return loggedIn ? .yes : .no }
        }
        return .unknown
    }

    /// Whether macOS lets this app send Apple Events to the app `bundle`, never asking: an app not running can't say.
    static func automation(bundle: String) -> Automation {
        guard let target = NSAppleEventDescriptor(bundleIdentifier: bundle).aeDesc else { return .unknown }
        switch AEDeterminePermissionToAutomateTarget(target, AEEventClass(typeWildCard), AEEventID(typeWildCard), false) {
        case noErr: return .allowed
        case OSStatus(errAEEventNotPermitted): return .denied
        default: return .unknown
        }
    }
}
