import Foundation

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
    static func run(_ path: String, _ arguments: [String]) -> (status: Int32, output: String)? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: path)
        process.arguments = arguments
        let output = Pipe()
        process.standardOutput = output
        process.standardError = FileHandle.nullDevice
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
        let tty = String(parts[0])
        switch Terminal.foreground(tty: tty, shell: pane) {
        case nil:
            return .notStopped
        case .job(let job, _, let commandLines)?:
            guard commandLines.contains(where: agent.isItself), Terminal.stop(job) else { return .notStopped }
            // The agent has gone, and if another job has taken its place, the pane is that job's now.
            if case .job? = Terminal.foreground(tty: tty, shell: pane) { return .fired }
        case .shell?:
            // The pane's own process is in front: its shell, or the agent when
            // the pane was started with it. Anything else is someone's work.
            guard let line = Tool.run("/bin/ps", ["-p", String(pane), "-o", "args="])?.output, !line.isEmpty else { return .notStopped }
            let program = (String(line.split(separator: " ")[0]) as NSString).lastPathComponent.lowercased()
                .trimmingCharacters(in: CharacterSet(charactersIn: "-"))
            guard agent.isItself(commandLine: line) || Tmux.shells.contains(program) else { return .notStopped }
        }
        return ran(tab, ["kill-pane", "-t", tab.id]) ? .fired : .notStopped
    }

    /// What a pane's own process is when it's only a shell.
    static let shells: Set<String> = ["zsh", "bash", "sh", "fish", "dash", "ksh", "tcsh", "csh", "nu", "login"]

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

/// Ghostty, through its AppleScript (1.3 and later): keys and text go to the
/// terminal without bringing it forward, with the Mac locked or the screen
/// saver on, as the bytes its `text:` action types. Addressed by its tty,
/// as ghostty:/dev/ttys004. Ghostty 1.3 can't say which terminal is on a
/// tty, so each is learned from its session's folder, when exactly one
/// terminal is there, and kept; Ghostty 1.4, which can, is asked directly.
/// Until then its desk is only watched.
struct Ghostty: TerminalDriver {
    var abilities: TerminalAbilities { [.keys, .text, .focus, .fire, .scroll] }
    var keysNeedAccessibility: Bool { false }

    /// Which terminal is on each tty, as learned.
    private static var known: [String: String] = [:]
    private static let lock = NSLock()

    static func id(on tty: String) -> String? {
        lock.lock(); defer { lock.unlock() }
        return known[tty]
    }

    /// Learns which terminal is on `tty`, the session there being in
    /// `folder`: by its tty where Ghostty says it, else the one terminal in
    /// that folder that isn't already another tty's. Off the main thread.
    static func learn(tty: String, folder: String) {
        guard id(on: tty) == nil else { return }
        let answer = Terminal.run("""
            if application "Ghostty" is not running then return ""
            tell application "Ghostty"
                try
                    return id of first terminal whose tty is (item 1 of argv)
                end try
                set found to {}
                repeat with t in terminals
                    if working directory of t is (item 2 of argv) then set end of found to (id of t)
                end repeat
                set AppleScript's text item delimiters to linefeed
                return found as text
            end tell
            """, arguments: [tty, folder]) ?? ""
        lock.lock(); defer { lock.unlock() }
        let taken = Set(known.values)
        let candidates = answer.split(separator: "\n").map(String.init).filter { !taken.contains($0) }
        if candidates.count == 1 { known[tty] = candidates[0] }
    }

    /// Forgets the terminal once its session has gone, as its tty may be another's next.
    static func forget(tty: String) {
        lock.lock(); defer { lock.unlock() }
        known[tty] = nil
    }

    /// Runs `body` with `t` as the terminal, "gone" if it isn't there.
    private func withTerminal(_ tab: TabAddress, _ body: String, _ extra: [String] = []) -> String? {
        guard let id = Ghostty.id(on: tab.id) else { return nil }
        return Terminal.run("""
            if application "Ghostty" is not running then return "gone"
            tell application "Ghostty"
                if not (exists terminal id (item 1 of argv)) then return "gone"
                set t to terminal id (item 1 of argv)
                \(body)
            end tell
            """, arguments: [id] + extra)
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
        withTerminal(tab, "perform action (\"text:\" & (item 2 of argv)) on t\nreturn \"ok\"", [Ghostty.escaped(bytes)]) == "ok"
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

    func focus(_ tab: TabAddress) -> Bool { withTerminal(tab, "focus t\nactivate\nreturn \"ok\"") == "ok" }

    func scroll(lines: Int, in tab: TabAddress) -> Bool {
        withTerminal(tab, "perform action (\"scroll_page_lines:\" & (item 2 of argv)) on t\nreturn \"ok\"", [String(lines)]) == "ok"
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
