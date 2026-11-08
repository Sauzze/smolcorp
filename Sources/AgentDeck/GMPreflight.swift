import AppKit
import CoreServices

/// What a first General Manager hire needs, checked before Start here's Hire
/// button: `GET /setup/hire/gm/preflight`, the same JSON as the Linux/Windows
/// app's (desktop/gmpreflight.go). Each line says what's wrong in one
/// sentence and exactly what to do: a command to copy, a fix to press, or
/// where to go on the page. Only looks: the fixes are the page's buttons.
///
/// Signed in is what Claude Code's own `claude auth status` says, as Summary
/// asks it: no credential, key or token is ever read.
enum GMPreflight {
    struct Action: Equatable {
        /// "copy" (a command to paste in Terminal), "fix" (a POST /setup/fix by name) or "folder" (the folder step above).
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

        var jsonObject: [String: Any] {
            ["id": id, "ok": ok, "title": title, "fix": fix, "action": action?.jsonObject ?? NSNull(), "optional": optional]
        }
    }

    enum SignIn: Equatable { case yes, no, unknown }
    enum Automation: Equatable { case allowed, denied, unknown }

    /// What was found, gathered by `facts` and turned into lines by `items`, so tests give their own.
    struct Facts {
        /// The folder for new hires as set ("" when it isn't), and whether it's a folder.
        var folder = ""
        var folderExists = false
        /// Where the app finds Claude Code, nil nowhere; whether your login shell, where hires start, finds it (nil: it didn't say).
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
    }

    /// The fix that opens Privacy & Security's Automation pane.
    static let automationFix = "automation"
    static let automationPane = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_Automation")!
    /// Claude Code's own installer.
    static let installCommand = "curl -fsSL https://claude.ai/install.sh | bash"

    /// The lines, in the order they're put right.
    static func items(_ f: Facts) -> [Item] {
        var items: [Item] = []
        let folder = tilde(f.folder, home: f.home)
        if f.folder.isEmpty {
            items.append(Item(id: "folder", ok: false, title: "There's no folder for your hires yet.", fix: "Choose it in the first step above.",
                              action: Action(kind: "folder", label: "Show me")))
        } else if !f.folderExists {
            items.append(Item(id: "folder", ok: false, title: "The folder for your hires isn't there any more: \(folder).",
                              fix: "Choose it again in the first step above.", action: Action(kind: "folder", label: "Show me")))
        } else {
            items.append(Item(id: "folder", ok: true, title: "Your hires work in \(folder)."))
        }
        if f.claudeAt == nil && f.claudeOnLoginPath != true {
            items.append(Item(id: "claude", ok: false, title: "Claude Code isn't installed on this Mac.",
                              fix: "Paste this in Terminal, wait for it to finish, then press Check again.",
                              action: Action(kind: "copy", label: "Copy", command: installCommand)))
            return items + terminalItems(f)
        }
        // Installed, but where your login shell, which hires start in, doesn't look.
        if f.claudeOnLoginPath == false, let at = f.claudeAt {
            let where_ = (at as NSString).deletingLastPathComponent
            items.append(Item(id: "claude", ok: false, title: "Claude Code is in \(tilde(where_, home: f.home)), where Terminal doesn't look for it.",
                              fix: "Paste this in Terminal, open a new Terminal window, then press Check again.",
                              action: Action(kind: "copy", label: "Copy", command: pathLine(shell: f.shell, folder: where_, home: f.home))))
            return items + terminalItems(f)
        }
        items.append(Item(id: "claude", ok: true, title: "Claude Code is installed."))
        switch f.signIn {
        case .yes: items.append(Item(id: "signin", ok: true, title: "Claude Code is signed in."))
        case .no:
            items.append(Item(id: "signin", ok: false, title: "Claude Code isn't signed in.",
                              fix: "Paste this in Terminal and sign in in the browser it opens (if it doesn't ask, type /login). Then press Check again.",
                              action: Action(kind: "copy", label: "Copy", command: "claude")))
        case .unknown:
            items.append(Item(id: "signin", ok: false, title: "Smolcorp couldn't tell if Claude Code is signed in.",
                              fix: "You can hire anyway: if your General Manager asks you to sign in, type /login in its window.", optional: true))
        }
        if f.hooks != nil {
            items.append(Item(id: "hooks", ok: false, title: "Claude Code doesn't report to Smolcorp yet, so your General Manager would never reach a desk.",
                              fix: "Press Connect, then Check again.", action: Action(kind: "fix", label: "Connect", fix: SelfTest.repair)))
        } else {
            items.append(Item(id: "hooks", ok: true, title: "Claude Code reports to Smolcorp."))
        }
        return items + terminalItems(f)
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
    static func pathLine(shell: String, folder: String, home: String) -> String {
        let name = (shell as NSString).lastPathComponent
        if name == "fish" { return "fish_add_path \(tilde(folder, home: home))" }
        let file: String
        switch name {
        case "zsh": file = "~/.zprofile"
        case "bash":
            file = [".bash_profile", ".bash_login"].first { FileManager.default.fileExists(atPath: (home as NSString).appendingPathComponent($0)) }
                .map { "~/" + $0 } ?? "~/.profile"
        default: file = "~/.profile"
        }
        let path = folder.hasPrefix(home + "/") ? "$HOME" + folder.dropFirst(home.count) : folder
        return "echo 'export PATH=\"\(path):$PATH\"' >> \(file)"
    }

    static func tilde(_ path: String, home: String) -> String {
        path == home ? "~" : path.hasPrefix(home + "/") ? "~" + path.dropFirst(home.count) : path
    }

    // MARK: Looking

    /// The checks for a hire on `account` ("" the default), answered on the main queue.
    /// The config and the hooks are read here, on the main queue; the shell, Claude Code and macOS are asked off it.
    static func check(config: Config, account: String, done: @escaping ([String: Any]) -> Void) {
        let profile = ClaudeProfiles.profile(named: account, among: ClaudeProfiles.current)
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
        f.shell = ProcessInfo.processInfo.environment["SHELL"].flatMap { $0.isEmpty ? nil : $0 } ?? "/bin/zsh"
        DispatchQueue.global(qos: .userInitiated).async {
            let login = OpenCodeInstall.loginShellPath()
            let found = OpenCodeInstall.program(named: "claude")
            f.claudeAt = found?.path
            f.claudeOnLoginPath = login.isEmpty ? nil : login.contains { FileManager.default.isExecutableFile(atPath: ($0 as NSString).appendingPathComponent("claude")) }
            if let found, f.claudeOnLoginPath != false { f.signIn = signIn(found, profile: profile) }
            f.automation = choice.map { automation(bundle: $0.bundle) } ?? .unknown
            let answer = report(items(f))
            DispatchQueue.main.async { done(answer) }
        }
    }

    /// What `claude auth status --json` says, on `profile`'s config folder.
    static func signIn(_ program: URL, profile: ClaudeProfile?) -> SignIn {
        var environment = SummaryCLI.environment()
        environment.removeValue(forKey: "SMOLCORP_SUMMARY")
        if let profile, !profile.isDefault { environment["CLAUDE_CONFIG_DIR"] = profile.dir } else { environment.removeValue(forKey: "CLAUDE_CONFIG_DIR") }
        let ran = SummaryCLI.spawn(program, ["auth", "status", "--json"], input: Data(), environment: environment,
                                   folder: FileManager.default.temporaryDirectory.path, timeout: 10)
        return signIn(status: ran.output)
    }

    static func signIn(status: Data) -> SignIn {
        guard let object = (try? JSONSerialization.jsonObject(with: status)) as? [String: Any], let loggedIn = object["loggedIn"] as? Bool else { return .unknown }
        return loggedIn ? .yes : .no
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
