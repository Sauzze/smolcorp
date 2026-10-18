import AppKit

/// The keys the phone presses, as tmux names them and as the bytes a
/// terminal sends for them: for terminals that take input directly, with no
/// window brought forward and no Accessibility. As the Windows and Linux
/// app's tmuxKeys and keyBytes (desktop/tabs.go).
extension Terminal.Keystroke {
    var tmuxName: String? {
        switch (key, modifiers.contains(.maskShift), modifiers.contains(.maskControl)) {
        case (.returnKey, false, false): return "Enter"
        case (.escape, false, false): return "Escape"
        case (.space, false, false): return "Space"
        case (.up, false, false): return "Up"
        case (.down, false, false): return "Down"
        case (.tab, false, false): return "Tab"
        case (.tab, true, false): return "BTab"
        case (.c, false, true): return "C-c"
        default: return nil
        }
    }

    var bytes: String? {
        switch (key, modifiers.contains(.maskShift), modifiers.contains(.maskControl)) {
        case (.returnKey, false, false): return "\r"
        case (.escape, false, false): return "\u{1b}"
        case (.space, false, false): return " "
        case (.up, false, false): return "\u{1b}[A"
        case (.down, false, false): return "\u{1b}[B"
        case (.tab, false, false): return "\t"
        case (.tab, true, false): return "\u{1b}[Z"
        case (.c, false, true): return "\u{03}"
        default: return nil
        }
    }

    /// The pause between one press and the next of the same key.
    static let gap: TimeInterval = 0.12
}

/// A terminal's command line tool, stopped if it hangs: keys go one at a
/// time, so one that hung would hold up every phone's keys behind it.
enum Tool {
    static let timeout: TimeInterval = 5

    /// Its output and exit status, or nil if it couldn't be run or was stopped for taking too long.
    /// With `errors`, what it says on its standard error comes in its output too.
    static func run(_ path: String, _ arguments: [String], errors: Bool = false) -> (status: Int32, output: String)? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: path)
        process.arguments = arguments
        let output = Pipe()
        process.standardOutput = output
        process.standardError = errors ? output : FileHandle.nullDevice
        do { try process.run() } catch { return nil }
        // Stopped when it overruns, which also ends the reading below.
        let overran = DispatchWorkItem { if process.isRunning { process.terminate() } }
        DispatchQueue.global().asyncAfter(deadline: .now() + timeout, execute: overran)
        let data = output.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        overran.cancel()
        guard process.terminationReason == .exit else { return nil }
        return (process.terminationStatus, String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines))
    }

    /// The first of `paths` that can be run: an app in the menu bar has no
    /// Homebrew on its PATH.
    static func find(_ paths: [String]) -> String? {
        paths.first { FileManager.default.isExecutableFile(atPath: $0) }
    }
}

/// Firing as Terminal.app does, for a tab whose own process, `root`, isn't
/// started under login, as a tmux pane's, a WezTerm pane's or a kitty
/// window's: the agent in front is stopped, then the tab closed; the tab's
/// own process alone in front is closed with it if it's a shell or the agent
/// itself; anything else in front, a job in the background, or a tab that
/// can't be looked at, is left as it is. If, once the agent has stopped,
/// another job is in front or in the background, the tab is left to it.
enum Firing {
    /// What a tab's own process is when it's only a shell.
    static let shells: Set<String> = ["zsh", "bash", "sh", "fish", "dash", "ksh", "tcsh", "csh", "nu", "login"]

    static func fire(tty: String, root: pid_t, agent: Agent, close: () -> Bool) -> Terminal.Fired {
        switch Terminal.foreground(tty: tty, shell: root) {
        case nil:
            return .notStopped
        case .job(let job, _, let commandLines)?:
            guard commandLines.contains(where: agent.isItself), Terminal.stop(job) else { return .notStopped }
            if case .job? = Terminal.foreground(tty: tty, shell: root) { return .fired }
            if Terminal.hasOtherJobs(tty: tty, shell: root) != false { return .fired }
        case .shell?:
            // A job in the background would be hung up with the tab.
            guard Terminal.hasOtherJobs(tty: tty, shell: root) == false else { return .notStopped }
            guard let line = Tool.run("/bin/ps", ["-p", String(root), "-o", "args="])?.output, !line.isEmpty else { return .notStopped }
            let program = (String(line.split(separator: " ")[0]) as NSString).lastPathComponent.lowercased()
                .trimmingCharacters(in: CharacterSet(charactersIn: "-"))
            guard agent.isItself(commandLine: line) || shells.contains(program) else { return .notStopped }
        }
        return close() ? .fired : .notStopped
    }

    /// The tab's own process on `tty`: the one whose parent isn't on it.
    static func root(on tty: String) -> pid_t? {
        guard let rows = Tool.run("/bin/ps", ["-t", (tty as NSString).lastPathComponent, "-o", "pid=,ppid="])?.output else { return nil }
        let pairs = rows.split(separator: "\n").compactMap { line -> (pid_t, pid_t)? in
            let fields = line.split(separator: " ")
            guard fields.count == 2, let pid = pid_t(fields[0]), let ppid = pid_t(fields[1]) else { return nil }
            return (pid, ppid)
        }
        let pids = Set(pairs.map(\.0))
        let roots = pairs.filter { !pids.contains($0.1) }.map(\.0)
        return roots.count == 1 ? roots[0] : nil
    }
}

/// tmux, in whichever terminal it runs: its server owns the pane, so keys
/// and text reach the agent without its window, with the Mac locked or the
/// screen saver on. Addressed as tmux:%3@<socket>.
struct Tmux: TerminalDriver {
    var abilities: TerminalAbilities { [.keys, .text, .focus, .fire, .scroll] }
    var keysNeedAccessibility: Bool { false }

    /// Settable for tests, which give a stand-in.
    static var path = Tool.find(["/opt/homebrew/bin/tmux", "/usr/local/bin/tmux", "/opt/local/bin/tmux", "/usr/bin/tmux"])

    private func run(_ tab: TabAddress, _ arguments: [String]) -> (status: Int32, output: String)? {
        guard let path = Tmux.path else { return nil }
        return Tool.run(path, (tab.socket.isEmpty ? [] : ["-S", tab.socket]) + arguments)
    }

    private func ran(_ tab: TabAddress, _ arguments: [String]) -> Bool { run(tab, arguments)?.status == 0 }

    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard let name = keystroke.tmuxName else { return false }
        for i in 0..<max(keystroke.times, 1) {
            if i > 0 { Thread.sleep(forTimeInterval: Terminal.Keystroke.gap) }
            guard ran(tab, ["send-keys", "-t", tab.id, name]) else { return false }
        }
        return true
    }

    func type(_ text: String, in tab: TabAddress) -> Bool {
        // A ; at the end would be read as tmux's own command separator.
        let literal = text.hasSuffix(";") ? String(text.dropLast()) + #"\;"# : text
        return ran(tab, ["send-keys", "-t", tab.id, "-l", "--", literal]) && ran(tab, ["send-keys", "-t", tab.id, "Enter"])
    }

    /// As a bracketed paste, so the agent takes it as one piece of text.
    func insert(_ text: String, in tab: TabAddress) -> Bool {
        let buffer = "smolcorp-\(UUID().uuidString.prefix(8))"
        return ran(tab, ["set-buffer", "-b", buffer, "--", text]) && ran(tab, ["paste-buffer", "-p", "-d", "-b", buffer, "-t", tab.id])
    }

    /// The pane, and the terminal tab tmux is shown in, when that's one the
    /// deck can bring forward.
    func focus(_ tab: TabAddress) -> Bool {
        guard ran(tab, ["select-window", "-t", tab.id]), ran(tab, ["select-pane", "-t", tab.id]) else { return false }
        // The tab showing this pane's own session, not the server's first client.
        if let session = run(tab, ["display-message", "-p", "-t", tab.id, "#{session_id}"]), session.status == 0,
           let client = run(tab, ["list-clients", "-t", session.output, "-F", "#{client_tty}"])?.output
               .split(separator: "\n").first.map(String.init) {
            _ = Terminal.focus(tty: client) || ITerm().focus(tty: client)
        }
        return true
    }

    func scroll(lines: Int, in tab: TabAddress) -> Bool {
        guard ran(tab, ["copy-mode", "-e", "-t", tab.id]) else { return false }
        return ran(tab, ["send-keys", "-t", tab.id, "-X", "-N", String(abs(lines)), lines < 0 ? "scroll-up" : "scroll-down"])
    }

    func scrollToBottom(in tab: TabAddress) -> Bool {
        _ = ran(tab, ["send-keys", "-t", tab.id, "-X", "cancel"]) // not in copy mode: already at the bottom
        return true
    }

    /// As in Terminal.app: the agent is stopped first, then the pane closed.
    /// A pane that runs anything else in front, as a dev server or an editor
    /// started there after the agent crashed, is left as it is, and so is one
    /// that can't be looked at.
    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired {
        guard let open = isOpen(tab) else { return .notStopped }
        guard open else { return .gone }
        guard let info = run(tab, ["display-message", "-p", "-t", tab.id, "#{pane_tty} #{pane_pid}"]), info.status == 0 else { return .notStopped }
        let parts = info.output.split(separator: " ")
        guard parts.count == 2, let pane = pid_t(parts[1]) else { return .notStopped }
        return Firing.fire(tty: String(parts[0]), root: pane, agent: agent) { ran(tab, ["kill-pane", "-t", tab.id]) }
    }

    func tty(of tab: TabAddress) -> String? {
        guard let tty = run(tab, ["display-message", "-p", "-t", tab.id, "#{pane_tty}"]), tty.status == 0, !tty.output.isEmpty else { return nil }
        return tty.output
    }

    /// Whether the pane is there: tmux answering that it isn't is the only no.
    func isOpen(_ tab: TabAddress) -> Bool? {
        guard let answer = run(tab, ["display-message", "-p", "-t", tab.id, "#{pane_id}"]) else { return nil }
        return answer.status == 0 && answer.output == tab.id
    }
}

/// iTerm2, through its AppleScript: `write text` reaches the session
/// without bringing it forward, with the Mac locked or the screen saver on.
/// Addressed by the session's unique ID, which its ITERM_SESSION_ID ends in.
struct ITerm: TerminalDriver {
    var abilities: TerminalAbilities { [.keys, .text, .focus, .fire] }
    var keysNeedAccessibility: Bool { false }

    /// Runs `body` with `s` as the session whose unique ID is the first
    /// argument; "gone" if there's none.
    private func withSession(_ id: String, _ body: String, _ extra: [String] = []) -> String? {
        Terminal.run("""
            if application "iTerm" is not running then return "gone"
            tell application "iTerm"
                repeat with w in windows
                    repeat with t in tabs of w
                        repeat with s in sessions of t
                            if unique ID of s is (item 1 of argv) then
                                \(body)
                            end if
                        end repeat
                    end repeat
                end repeat
            end tell
            return "gone"
            """, arguments: [id] + extra)
    }

    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard let bytes = keystroke.bytes else { return false }
        for i in 0..<max(keystroke.times, 1) {
            if i > 0 { Thread.sleep(forTimeInterval: Terminal.Keystroke.gap) }
            guard withSession(tab.id, "tell s to write text (item 2 of argv) newline NO\nreturn \"ok\"", [bytes]) == "ok" else { return false }
        }
        return true
    }

    func type(_ text: String, in tab: TabAddress) -> Bool {
        withSession(tab.id, "tell s to write text (item 2 of argv)\nreturn \"ok\"", [text]) == "ok"
    }

    func insert(_ text: String, in tab: TabAddress) -> Bool {
        withSession(tab.id, "tell s to write text (item 2 of argv) newline NO\nreturn \"ok\"", [text]) == "ok"
    }

    func focus(_ tab: TabAddress) -> Bool {
        withSession(tab.id, "select w\nselect t\nselect s\nactivate\nreturn \"ok\"") == "ok"
    }

    /// The session on `tty`, brought forward: for tmux shown in iTerm2.
    func focus(tty: String) -> Bool {
        Terminal.run("""
            if application "iTerm" is not running then return "gone"
            tell application "iTerm"
                repeat with w in windows
                    repeat with t in tabs of w
                        repeat with s in sessions of t
                            if tty of s is (item 1 of argv) then
                                select w
                                select t
                                select s
                                activate
                                return "ok"
                            end if
                        end repeat
                    end repeat
                end repeat
            end tell
            return "gone"
            """, arguments: [tty]) == "ok"
    }

    func scroll(lines: Int, in tab: TabAddress) -> Bool { false }
    func scrollToBottom(in tab: TabAddress) -> Bool { false }

    /// As in Terminal.app: the agent is stopped first, so closing the session
    /// doesn't ask about a job still running, then the session is closed.
    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired {
        guard let tty = tty(of: tab) else { return isOpen(tab) == false ? .gone : .notStopped }
        switch Terminal.foreground(tty: tty) {
        case nil:
            // Unknown is not the same as none: nothing is touched.
            return .notStopped
        case .job(let job, _, let commandLines)?:
            guard commandLines.contains(where: agent.isItself), Terminal.stop(job) else { return .notStopped }
            // The agent has gone, and if another job has taken its place, the session is that job's now.
            if case .job? = Terminal.foreground(tty: tty) { return .fired }
        case .shell?:
            break
        }
        return withSession(tab.id, "close s\nreturn \"ok\"") == "ok" ? .fired : .notStopped
    }

    func tty(of tab: TabAddress) -> String? {
        guard let tty = withSession(tab.id, "return tty of s"), tty != "gone", !tty.isEmpty else { return nil }
        return tty
    }

    func isOpen(_ tab: TabAddress) -> Bool? {
        guard let answer = withSession(tab.id, "return \"open\"") else { return nil }
        return answer == "open"
    }
}

/// Ghostty, through its AppleScript: keys and text go to the terminal without
/// bringing it forward, with the Mac locked or the screen saver on, as the
/// bytes its `text:` action types. Addressed by its tty, as
/// ghostty:/dev/ttys004. Only a Ghostty that says which terminal is on a tty
/// (1.4 and later) is driven, and that tty is asked again before each key,
/// line or close; on Ghostty 1.3, which can't say, its sessions are watched,
/// never controlled: a guess, by folder, could type into your own shell.
struct Ghostty: TerminalDriver {
    var abilities: TerminalAbilities { [.keys, .text, .focus, .fire, .scroll] }
    var keysNeedAccessibility: Bool { false }

    /// Which terminal is on each tty, as Ghostty said.
    private static var known: [String: String] = [:]
    private static let lock = NSLock()

    static func id(on tty: String) -> String? {
        lock.lock(); defer { lock.unlock() }
        return known[tty]
    }

    /// Asks Ghostty which terminal is on `tty`, where it can say (1.4 and
    /// later). Off the main thread.
    static func learn(tty: String) {
        guard id(on: tty) == nil else { return }
        let answer = Terminal.run("""
            if application "Ghostty" is not running then return ""
            tell application "Ghostty"
                try
                    return id of first terminal whose tty is (item 1 of argv)
                end try
            end tell
            return ""
            """, arguments: [tty]) ?? ""
        guard !answer.isEmpty else { return }
        lock.lock(); defer { lock.unlock() }
        if !known.values.contains(answer) { known[tty] = answer }
    }

    /// Forgets the terminal once its session has gone, as its tty may be another's next.
    static func forget(tty: String) {
        lock.lock(); defer { lock.unlock() }
        known[tty] = nil
    }

    /// Runs `body` with `t` as the terminal, "gone" if it isn't there.
    /// …and still on the session's tty, asked each time: if it isn't, nothing is done.
    private func withTerminal(_ tab: TabAddress, _ body: String, _ extra: [String] = []) -> String? {
        guard let id = Ghostty.id(on: tab.id) else { return nil }
        return Terminal.run("""
            if application "Ghostty" is not running then return "gone"
            tell application "Ghostty"
                if not (exists terminal id (item 1 of argv)) then return "gone"
                set t to terminal id (item 1 of argv)
                try
                    if (tty of t) is not (item 2 of argv) then return "gone"
                on error
                    return "gone"
                end try
                \(body)
            end tell
            """, arguments: [id, tab.id] + extra)
    }

    /// `text` as Ghostty's text: action reads it, its escapes being Zig's.
    static func escaped(_ text: String) -> String {
        var out = ""
        for scalar in text.unicodeScalars {
            switch scalar {
            case "\\": out += "\\\\"
            case "\r": out += "\\r"
            case "\n": out += "\\n"
            case "\t": out += "\\t"
            default:
                if scalar.value < 0x20 || scalar.value == 0x7f { out += String(format: "\\x%02x", scalar.value) } else { out.unicodeScalars.append(scalar) }
            }
        }
        return out
    }

    private func typeBytes(_ bytes: String, in tab: TabAddress) -> Bool {
        withTerminal(tab, "perform action (\"text:\" & (item 3 of argv)) on t\nreturn \"ok\"", [Ghostty.escaped(bytes)]) == "ok"
    }

    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard let bytes = keystroke.bytes else { return false }
        for i in 0..<max(keystroke.times, 1) {
            if i > 0 { Thread.sleep(forTimeInterval: Terminal.Keystroke.gap) }
            guard typeBytes(bytes, in: tab) else { return false }
        }
        return true
    }

    func type(_ text: String, in tab: TabAddress) -> Bool { typeBytes(text + "\r", in: tab) }

    /// As Ghostty pastes, without Return.
    func insert(_ text: String, in tab: TabAddress) -> Bool {
        withTerminal(tab, "input text (item 3 of argv) to t\nreturn \"ok\"", [text]) == "ok"
    }

    func focus(_ tab: TabAddress) -> Bool { withTerminal(tab, "focus t\nactivate\nreturn \"ok\"") == "ok" }

    func scroll(lines: Int, in tab: TabAddress) -> Bool {
        withTerminal(tab, "perform action (\"scroll_page_lines:\" & (item 3 of argv)) on t\nreturn \"ok\"", [String(lines)]) == "ok"
    }

    func scrollToBottom(in tab: TabAddress) -> Bool { withTerminal(tab, "perform action \"scroll_to_bottom\" on t\nreturn \"ok\"") == "ok" }

    /// As in Terminal.app, whose way of starting shells Ghostty shares: the
    /// agent is stopped first, then the terminal closed; anything else in
    /// front is left as it is.
    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired {
        guard let open = isOpen(tab) else { return .notStopped }
        guard open else { return .gone }
        switch Terminal.foreground(tty: tab.id) {
        case nil:
            return .notStopped
        case .job(let job, _, let commandLines)?:
            guard commandLines.contains(where: agent.isItself), Terminal.stop(job) else { return .notStopped }
            if case .job? = Terminal.foreground(tty: tab.id) { return .fired }
        case .shell?:
            break
        }
        return withTerminal(tab, "close t\nreturn \"ok\"") == "ok" ? .fired : .notStopped
    }

    func tty(of tab: TabAddress) -> String? { tab.id }

    func isOpen(_ tab: TabAddress) -> Bool? {
        guard let answer = withTerminal(tab, "return \"open\"") else { return nil }
        return answer == "open"
    }
}

/// Brings a terminal app forward by its bundle id, after its own command
/// line has picked the pane.
private func activate(bundle: String) {
    DispatchQueue.main.async {
        NSRunningApplication.runningApplications(withBundleIdentifier: bundle).first?.activate()
    }
}

/// WezTerm, through `wezterm cli`, which needs no setting: keys and text go
/// to the pane as bytes, without bringing it forward. Addressed as
/// wezterm:<pane>@<socket>, the socket being the GUI's, from the hook.
/// As the Windows and Linux app's (desktop/tabs.go).
struct WezTerm: TerminalDriver {
    var abilities: TerminalAbilities { [.keys, .text, .focus, .fire] }
    var keysNeedAccessibility: Bool { false }

    /// Settable for tests, which give a stand-in.
    static var path = Tool.find(["/Applications/WezTerm.app/Contents/MacOS/wezterm", "/opt/homebrew/bin/wezterm", "/usr/local/bin/wezterm"])

    private func run(_ tab: TabAddress, _ arguments: [String]) -> (status: Int32, output: String)? {
        guard let path = WezTerm.path else { return nil }
        // The GUI's socket, where the hook gave it; otherwise wezterm finds the running GUI itself.
        let socket = tab.socket.isEmpty ? [] : ["WEZTERM_UNIX_SOCKET=\(tab.socket)"]
        return Tool.run("/usr/bin/env", socket + [path, "cli"] + arguments)
    }

    private func ran(_ tab: TabAddress, _ arguments: [String]) -> Bool { run(tab, arguments)?.status == 0 }

    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard let bytes = keystroke.bytes else { return false }
        for i in 0..<max(keystroke.times, 1) {
            if i > 0 { Thread.sleep(forTimeInterval: Terminal.Keystroke.gap) }
            guard ran(tab, ["send-text", "--pane-id", tab.id, "--no-paste", bytes]) else { return false }
        }
        return true
    }

    func type(_ text: String, in tab: TabAddress) -> Bool {
        ran(tab, ["send-text", "--pane-id", tab.id, "--no-paste", "--", text]) && ran(tab, ["send-text", "--pane-id", tab.id, "--no-paste", "\r"])
    }

    /// As a paste, which is send-text's own way, without Return.
    func insert(_ text: String, in tab: TabAddress) -> Bool { ran(tab, ["send-text", "--pane-id", tab.id, "--", text]) }

    func focus(_ tab: TabAddress) -> Bool {
        guard ran(tab, ["activate-pane", "--pane-id", tab.id]) else { return false }
        activate(bundle: "com.github.wez.wezterm")
        return true
    }

    func scroll(lines: Int, in tab: TabAddress) -> Bool { false }
    func scrollToBottom(in tab: TabAddress) -> Bool { false }

    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired {
        guard let open = isOpen(tab) else { return .notStopped }
        guard open else { return .gone }
        guard let tty = tty(of: tab), let root = Firing.root(on: tty) else { return .notStopped }
        return Firing.fire(tty: tty, root: root, agent: agent) { ran(tab, ["kill-pane", "--pane-id", tab.id]) }
    }

    /// The pane as `cli list` describes it, or nil if WezTerm couldn't be asked.
    private func pane(_ tab: TabAddress) -> (open: Bool, tty: String?)? {
        guard let answer = run(tab, ["list", "--format", "json"]), answer.status == 0,
              let panes = try? JSONSerialization.jsonObject(with: Data(answer.output.utf8)) as? [[String: Any]] else { return nil }
        guard let found = panes.first(where: { ($0["pane_id"] as? Int).map(String.init) == tab.id }) else { return (false, nil) }
        return (true, found["tty_name"] as? String)
    }

    func tty(of tab: TabAddress) -> String? { pane(tab)?.tty }
    func isOpen(_ tab: TabAddress) -> Bool? { pane(tab)?.open }
}

/// kitty, through its remote control, which needs `allow_remote_control` and
/// `listen_on` in kitty.conf: keys and text go to the window without bringing
/// it forward. Addressed as kitty:<window>@<socket>, the socket from the
/// hook's KITTY_LISTEN_ON. As the Windows and Linux app's (desktop/tabs.go).
struct Kitty: TerminalDriver {
    var abilities: TerminalAbilities { [.keys, .text, .focus, .fire, .scroll] }
    var keysNeedAccessibility: Bool { false }

    /// Settable for tests, which give a stand-in.
    static var path = Tool.find(["/Applications/kitty.app/Contents/MacOS/kitten", "/opt/homebrew/bin/kitten", "/usr/local/bin/kitten"])

    private func run(_ tab: TabAddress, _ arguments: [String], errors: Bool = false) -> (status: Int32, output: String)? {
        guard let path = Kitty.path else { return nil }
        return Tool.run(path, ["@", "--to", tab.socket] + arguments, errors: errors)
    }

    private func ran(_ tab: TabAddress, _ arguments: [String]) -> Bool { run(tab, arguments)?.status == 0 }

    /// Text as kitty's send-text reads it: Python escapes, so a backslash itself is doubled.
    static func escaped(_ text: String) -> String {
        var out = ""
        for scalar in text.unicodeScalars {
            if scalar == "\\" { out += "\\\\" } else if scalar.value < 0x20 { out += String(format: "\\x%02x", scalar.value) } else { out.unicodeScalars.append(scalar) }
        }
        return out
    }

    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard let bytes = keystroke.bytes else { return false }
        for i in 0..<max(keystroke.times, 1) {
            if i > 0 { Thread.sleep(forTimeInterval: Terminal.Keystroke.gap) }
            guard ran(tab, ["send-text", "--match", "id:\(tab.id)", Kitty.escaped(bytes)]) else { return false }
        }
        return true
    }

    func type(_ text: String, in tab: TabAddress) -> Bool {
        ran(tab, ["send-text", "--match", "id:\(tab.id)", "--", Kitty.escaped(text + "\r")])
    }

    func insert(_ text: String, in tab: TabAddress) -> Bool {
        ran(tab, ["send-text", "--match", "id:\(tab.id)", "--", Kitty.escaped(text)])
    }

    func focus(_ tab: TabAddress) -> Bool {
        guard ran(tab, ["focus-window", "--match", "id:\(tab.id)"]) else { return false }
        activate(bundle: "net.kovidgoyal.kitty")
        return true
    }

    func scroll(lines: Int, in tab: TabAddress) -> Bool {
        ran(tab, ["scroll-window", "--match", "id:\(tab.id)", "\(abs(lines))l" + (lines < 0 ? "-" : "")])
    }

    func scrollToBottom(in tab: TabAddress) -> Bool { ran(tab, ["scroll-window", "--match", "id:\(tab.id)", "end"]) }

    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired {
        guard let open = isOpen(tab) else { return .notStopped }
        guard open else { return .gone }
        guard let (root, tty) = window(tab), let root, let tty else { return .notStopped }
        return Firing.fire(tty: tty, root: root, agent: agent) { ran(tab, ["close-window", "--match", "id:\(tab.id)"]) }
    }

    /// The window's own process and its tty, from `ls`.
    private func window(_ tab: TabAddress) -> (root: pid_t?, tty: String?)? {
        guard let answer = run(tab, ["ls", "--match", "id:\(tab.id)"]), answer.status == 0,
              let windows = try? JSONSerialization.jsonObject(with: Data(answer.output.utf8)) as? [[String: Any]] else { return nil }
        for os in windows {
            for t in os["tabs"] as? [[String: Any]] ?? [] {
                for w in t["windows"] as? [[String: Any]] ?? [] where (w["id"] as? Int).map(String.init) == tab.id {
                    let pid = (w["pid"] as? Int).map { pid_t($0) }
                    let tty = pid.flatMap { Tool.run("/bin/ps", ["-p", String($0), "-o", "tty="])?.output }
                        .flatMap { $0.isEmpty || $0 == "??" ? nil : "/dev/\($0)" }
                    return (pid, tty)
                }
            }
        }
        return (nil, nil)
    }

    func tty(of tab: TabAddress) -> String? { window(tab)?.tty }

    /// Closed only when kitty says there's no such window: any other failure,
    /// as kitty not answering on its socket, is unknown.
    func isOpen(_ tab: TabAddress) -> Bool? {
        guard let answer = run(tab, ["ls", "--match", "id:\(tab.id)"], errors: true) else { return nil }
        if answer.status == 0 { return true }
        return answer.output.contains("No matching windows") ? false : nil
    }
}

/// Where hires open: a new window of the terminal chosen in setup, in the
/// hire folder, running the agent or its role's start script. With "Start
/// hires inside tmux", the agent runs in a tmux session of its own, which
/// that window shows, so its keys and mic work whatever the terminal.
enum Openers {
    struct Choice {
        let id: String, name: String, bundle: String
    }

    static let choices = [
        Choice(id: "terminal", name: "Terminal", bundle: "com.apple.Terminal"),
        Choice(id: "iterm", name: "iTerm2", bundle: "com.googlecode.iterm2"),
        Choice(id: "ghostty", name: "Ghostty", bundle: "com.mitchellh.ghostty"),
    ]

    static func isInstalled(_ id: String) -> Bool {
        guard let choice = choices.first(where: { $0.id == id }) else { return false }
        return NSWorkspace.shared.urlForApplication(withBundleIdentifier: choice.bundle) != nil
    }

    /// Opens the hire's window and gives its tab as the deck will know it,
    /// or nil if it didn't open. A Ghostty terminal's tab is given as
    /// ghostty-id:<terminal>, its tty being known only once its session
    /// reports. Off the main thread.
    static func open(_ command: String, in folder: String, terminal: String, inTmux: Bool) -> String? {
        guard inTmux, let tmux = Tmux.path else { return window(command, in: folder, terminal: terminal) }
        // The agent's own tmux session, through your login shell, so it finds
        // what your PATH finds; then a window showing it.
        let name = "smolcorp-\(UUID().uuidString.prefix(8).lowercased())"
        let shell = ProcessInfo.processInfo.environment["SHELL"].flatMap { $0.isEmpty ? nil : $0 } ?? "/bin/zsh"
        guard let made = Tool.run(tmux, session(name, in: folder, shell: shell, running: command)), made.status == 0 else { return nil }
        let parts = made.output.split(separator: " ", maxSplits: 1).map(String.init)
        guard parts.count == 2, let attach = script("exec \(quoted(tmux)) -S \(quoted(parts[1])) attach -t \(quoted(name))"),
              window(attach, in: folder, terminal: terminal) != nil else {
            _ = Tool.run(tmux, ["kill-session", "-t", name])
            return nil
        }
        return "tmux:\(parts[0])@\(parts[1])"
    }

    /// tmux's arguments for a hire's session, `command` run by your login shell, which prints its pane and socket.
    static func session(_ name: String, in folder: String, shell: String, running command: String) -> [String] {
        ["new-session", "-d", "-P", "-F", "#{pane_id} #{socket_path}", "-s", name, "-c", folder, shell, "-lic", quoted(command)]
    }

    private static func window(_ command: String, in folder: String, terminal: String) -> String? {
        switch terminal {
        case "iterm":
            return Terminal.run("""
                tell application "iTerm"
                    activate
                    set w to (create window with default profile)
                    tell current session of w to write text "cd " & quoted form of (item 1 of argv) & " && " & quoted form of (item 2 of argv)
                    return "iterm:" & (unique ID of current session of w)
                end tell
                """, arguments: [folder, command]).flatMap { $0.hasPrefix("iterm:") ? $0 : nil }
        case "ghostty":
            return Terminal.run("""
                tell application "Ghostty"
                    activate
                    set w to new window with configuration {initial working directory:(item 1 of argv), initial input:(quoted form of (item 2 of argv)) & linefeed}
                    return "ghostty-id:" & (id of focused terminal of selected tab of w)
                end tell
                """, arguments: [folder, command]).flatMap { $0.hasPrefix("ghostty-id:") ? $0 : nil }
        default:
            return Terminal.open(command, in: folder).map { TabAddress.terminalApp(tty: $0).text }
        }
    }

    /// A few lines for a hire's window to run once, which remove themselves as they run.
    private static func script(_ line: String) -> String? {
        try? FileManager.default.createDirectory(at: Paths.hires, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        let url = Paths.hires.appendingPathComponent("attach-\(UUID().uuidString).sh")
        guard FileManager.default.createFile(atPath: url.path, contents: Data("#!/bin/sh\nrm -f -- \"$0\"\n\(line)\n".utf8),
                                             attributes: [.posixPermissions: 0o700]) else { return nil }
        return url.path
    }

    /// `text` as one word to a shell, whatever it holds: a hire's path has a space, in Application Support.
    static func quoted(_ text: String) -> String { "'" + text.replacingOccurrences(of: "'", with: "'\\''") + "'" }
}
