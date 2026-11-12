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
    /// `timeout` is how long it has, `Tool.timeout` unless a slow one (a checkout) is given longer.
    /// `input`, if given, is its standard input, then closed: text that must arrive byte for byte goes
    /// there, as Process passes each argument in the file system's form, its accents taken apart (é as
    /// e and a combining accent).
    static func run(_ path: String, _ arguments: [String], errors: Bool = false, timeout limit: TimeInterval = Tool.timeout,
                    input: Data? = nil) -> (status: Int32, output: String)? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: path)
        process.arguments = arguments
        let output = Pipe()
        process.standardOutput = output
        process.standardError = errors ? output : FileHandle.nullDevice
        let given = input.map { _ in Pipe() }
        if let given { process.standardInput = given }
        do { try process.run() } catch { return nil }
        if let given, let input { feed(input, to: given) }
        Terminal.noteStep()
        // Stopped when it overruns, which also ends the reading below.
        let overran = DispatchWorkItem { if process.isRunning { process.terminate() } }
        DispatchQueue.global().asyncAfter(deadline: .now() + limit, execute: overran)
        let data = output.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        overran.cancel()
        guard process.terminationReason == .exit else {
            Terminal.noteTimeout()
            return nil
        }
        return (process.terminationStatus, String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines))
    }

    /// Writes `input` into `pipe`, then closes it, in the background: a tool's standard input, once it runs.
    static func feed(_ input: Data, to pipe: Pipe) {
        let fd = pipe.fileHandleForWriting.fileDescriptor
        // A tool that stops reading early mustn't raise SIGPIPE in Smolcorp.
        _ = fcntl(fd, F_SETNOSIGPIPE, 1)
        DispatchQueue.global().async {
            input.withUnsafeBytes { bytes in
                var sent = 0
                while sent < bytes.count {
                    let n = write(fd, bytes.baseAddress! + sent, bytes.count - sent)
                    if n <= 0 { break }
                    sent += n
                }
            }
            try? pipe.fileHandleForWriting.close()
        }
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

    private func run(_ tab: TabAddress, _ arguments: [String], input: Data? = nil) -> (status: Int32, output: String)? {
        guard let path = Tmux.path else { return nil }
        return Tool.run(path, (tab.socket.isEmpty ? [] : ["-S", tab.socket]) + arguments, input: input)
    }

    private func ran(_ tab: TabAddress, _ arguments: [String], input: Data? = nil) -> Bool { run(tab, arguments, input: input)?.status == 0 }

    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard let name = keystroke.tmuxName else { return false }
        for i in 0..<max(keystroke.times, 1) {
            if i > 0 { Thread.sleep(forTimeInterval: Terminal.Keystroke.gap) }
            guard ran(tab, ["send-keys", "-t", tab.id, name]) else { return false }
        }
        return true
    }

    /// `text` as the last argument of a tmux command, kept whole. tmux reads an
    /// argument ending in ";" as the end of its command and drops that ";"
    /// (a lone ";" leaves no text at all), and takes one ending in "\;" for a
    /// ";": so a ";" at the end goes as "\;". As the Windows and Linux app's
    /// tmuxLiteral (desktop/tabs.go). Text that must arrive byte for byte goes
    /// on tmux's standard input instead (see `pasted`).
    static func literal(_ text: String) -> String {
        text.hasSuffix(";") ? String(text.dropLast()) + #"\;"# : text
    }

    func type(_ text: String, in tab: TabAddress) -> Bool {
        ran(tab, ["send-keys", "-t", tab.id, "-l", "--", Tmux.literal(text)]) && ran(tab, ["send-keys", "-t", tab.id, "Enter"])
    }

    /// As a bracketed paste, so the agent takes it as one piece of text.
    func insert(_ text: String, in tab: TabAddress) -> Bool {
        // Escape could end the bracket early and Return would press Enter: refused, as a hire's paste does.
        guard HireTmux.canPaste(text) else { return false }
        return pasted(text, in: tab)
    }

    /// Several lines as one bracketed paste, only while the pane's agent has asked for bracketed paste:
    /// tmux reads the pane's flag and pastes in one command (`if-shell -F`), so the flag can't change in
    /// between, and `-r` keeps each line feed a line feed, byte for byte. The same command says which way
    /// it went (RI34 S1): "pasted", or "not-taken" and whether the pane is in copy mode (its flag then
    /// reads 0), the buffer deleted. Anything else, as tmux ending for taking too long or a pane that's
    /// gone, is a failure: the buffer goes, and no Return follows. A tmux before 3.7, without the flag,
    /// reads as not asked: nothing typed.
    var linesWay: LinesWay { .paneFlag }
    func insertLines(_ text: String, in tab: TabAddress, ready: () -> String?) -> LinesPasted {
        guard HireTmux.canPaste(text) else { return .failed }
        if let why = ready() { return .refused(why) }
        let buffer = "smolcorp-\(UUID().uuidString.prefix(8))"
        guard ran(tab, ["load-buffer", "-b", buffer, "-"], input: Data(text.utf8)) else { return .failed }
        let answer = run(tab, ["if-shell", "-F", "-t", tab.id, "#{bracket_paste_flag}",
                               "paste-buffer -p -r -d -b \(buffer) -t \(tab.id) ; display-message -p pasted",
                               "delete-buffer -b \(buffer) ; display-message -p -t \(tab.id) 'not-taken #{pane_in_mode}'"])
        switch answer.map({ ($0.status, $0.output) }) {
        case (0, "pasted")?: return .pasted
        case (0, "not-taken 0")?: return .notTaken
        case (0, "not-taken 1")?: return .inMode
        default:
            _ = ran(tab, ["delete-buffer", "-b", buffer])
            return .failed
        }
    }

    /// `text` into the pane through a buffer of its own, loaded from tmux's standard input, byte for
    /// byte: as an argument, its accents would come apart (see `Tool.run`) and a ";" at its end go.
    private func pasted(_ text: String, in tab: TabAddress) -> Bool {
        let buffer = "smolcorp-\(UUID().uuidString.prefix(8))"
        guard ran(tab, ["load-buffer", "-b", buffer, "-"], input: Data(text.utf8)) else { return false }
        if ran(tab, ["paste-buffer", "-p", "-d", "-b", buffer, "-t", tab.id]) { return true }
        _ = ran(tab, ["delete-buffer", "-b", buffer]) // -d only deletes after a paste
        return false
    }

    /// The pane, and the terminal tab tmux is shown in, when that's one the
    /// deck can bring forward. Only tmux is asked here, on tmux's queue; the
    /// tab is brought forward, or a hire's window opened again, afterwards on
    /// that terminal's own queue, so no tmux desk's keys wait on a terminal
    /// app that isn't answering.
    func focus(_ tab: TabAddress) -> Bool {
        // Selecting a pane and asking tmux about it type nothing.
        Terminal.onlyLooking {
            guard ran(tab, ["select-window", "-t", tab.id]), ran(tab, ["select-pane", "-t", tab.id]) else { return false }
            // The tab showing this pane's own session, not the server's first client.
            if let session = run(tab, ["display-message", "-p", "-t", tab.id, "#{session_id}"]), session.status == 0,
               let client = run(tab, ["list-clients", "-t", session.output, "-F", "#{client_tty}"])?.output
                   .split(separator: "\n").first.map(String.init) {
                // Skipped when its terminal gets to it too late, as when Terminal answers again after a
                // hang: it would bring Terminal forward seconds after the tap.
                let tapped = Date()
                let inTime = { Date().timeIntervalSince(tapped) < Terminals.inputWait }
                Terminal.queue.async {
                    guard inTime(), !Terminal.focus(tty: client) else { return }
                    Terminals.queue(kind: "iterm").async { if inTime() { _ = ITerm().focus(tty: client) } }
                }
            } else if HireTmux.owns(tab.socket) {
                // A hire's own server with no window showing it: "open" shows it again.
                let config = Config.shared
                let terminal = Openers.isInstalled(config.hireTerminal) ? config.hireTerminal : "terminal"
                let folder = run(tab, ["display-message", "-p", "-t", tab.id, "#{pane_current_path}"]).flatMap { $0.status == 0 ? $0.output : nil }
                let socket = tab.socket
                // One window, however many taps arrive while it opens.
                guard Tmux.reopening.insert(socket) else { return true }
                Terminals.queue(opening: terminal).async {
                    defer { Tmux.reopening.remove(socket) }
                    if let window = Openers.reattach(socket: socket, in: folder ?? NSHomeDirectory(), terminal: terminal) {
                        Tmux.reopened?(socket, window)
                    }
                }
            }
            return true
        }
    }

    /// The hires' servers whose window `focus` is opening again.
    static let reopening = Openings()

    /// Told of a window `focus` opened again for a hire's own server (its socket, the window as its terminal gave
    /// it), so the deck records it as Smolcorp's for that hire.
    static var reopened: ((_ socket: String, _ window: String) -> Void)?

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
        guard let tty = Terminal.onlyLooking({ run(tab, ["display-message", "-p", "-t", tab.id, "#{pane_tty}"]) }), tty.status == 0,
              !tty.output.isEmpty else { return nil }
        return tty.output
    }

    /// Whether the pane is there: tmux answering that it isn't is the only no.
    func isOpen(_ tab: TabAddress) -> Bool? {
        guard let answer = Terminal.onlyLooking({ run(tab, ["display-message", "-p", "-t", tab.id, "#{pane_id}"]) }) else { return nil }
        return answer.status == 0 && answer.output == tab.id
    }

    /// The pane's visible lines, as text, without its colours.
    func screen(of tab: TabAddress) -> String? {
        guard let answer = Terminal.onlyLooking({ run(tab, ["capture-pane", "-p", "-t", tab.id]) }), answer.status == 0 else { return nil }
        return answer.output
    }
}

/// iTerm2, through its AppleScript: `write text` reaches the session
/// without bringing it forward, with the Mac locked or the screen saver on.
/// Addressed by the session's unique ID, which its ITERM_SESSION_ID ends in.
struct ITerm: TerminalDriver {
    var abilities: TerminalAbilities { [.keys, .text, .focus, .fire] }
    var keysNeedAccessibility: Bool { false }

    /// Runs `body` with `s` as the session whose unique ID is the first
    /// argument; "gone" if there's none. `text`, if given, is its `givenText`.
    private func withSession(_ id: String, _ body: String, _ extra: [String] = [], text: String? = nil) -> String? {
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
            """, arguments: [id] + extra, input: text)
    }

    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard let bytes = keystroke.bytes else { return false }
        for i in 0..<max(keystroke.times, 1) {
            if i > 0 { Thread.sleep(forTimeInterval: Terminal.Keystroke.gap) }
            guard withSession(tab.id, "tell s to write text (item 2 of argv) newline NO\nreturn \"ok\"", [bytes]) == "ok" else { return false }
        }
        return true
    }

    /// The text goes on osascript's standard input, never as an argument: see `Terminal.run`.
    func type(_ text: String, in tab: TabAddress) -> Bool {
        withSession(tab.id, "tell s to write text givenText\nreturn \"ok\"", text: text) == "ok"
    }

    func insert(_ text: String, in tab: TabAddress) -> Bool {
        withSession(tab.id, "tell s to write text givenText newline NO\nreturn \"ok\"", text: text) == "ok"
    }

    /// The session's visible lines, as text: its `contents`.
    func screen(of tab: TabAddress) -> String? {
        guard let answer = withSession(tab.id, "return \"ok\" & (contents of s)"), answer.hasPrefix("ok") else { return nil }
        return String(answer.dropFirst(2))
    }

    func focus(_ tab: TabAddress) -> Bool {
        withSession(tab.id, "select w\nselect t\nselect s\nactivate\nreturn \"ok\"") == "ok"
    }

    static let bundleID = "com.googlecode.iterm2"

    /// Several lines through the clipboard and ⌘V, the session brought forward first: iTerm2's own paste,
    /// which it wraps in bracketed-paste markers where the agent asked for them. Posting ⌘V needs Accessibility.
    var linesWay: LinesWay { .clipboard(inFront: true) }
    func insertLines(_ text: String, in tab: TabAddress, ready: () -> String?) -> LinesPasted {
        ClipboardPaste.paste(text, first: { Terminal.macInUse ? .macInUse : nil },
                             forward: { Terminal.macInUse ? .macInUse : forwardForPaste(tab) ? nil : .notInFront }, ready: ready,
                             press: { posting in isInFront(tab) && Terminal.pressPaste(in: ITerm.bundleID, posting: posting) ? .sent : .notInFront })
    }

    /// The session selected and iTerm2 in front, never while the Mac is locked; true once this very
    /// session is the current one of iTerm2's front window, asked again after it came forward. Types nothing.
    private func forwardForPaste(_ tab: TabAddress) -> Bool {
        Terminal.onlyLooking {
            guard !Terminal.isLocked, focus(tab), Terminal.waitUntilFrontmost(ITerm.bundleID) else { return false }
            return isInFront(tab)
        }
    }

    /// Whether this very session is the current one of iTerm2's front window, iTerm2 in front: asked again
    /// right before ⌘V too (RI34b B2), as a click on another pane, tab or window could have changed it.
    private func isInFront(_ tab: TabAddress) -> Bool {
        Terminal.onlyLooking {
            Terminal.run("""
                if application "iTerm" is not running then return "gone"
                tell application "iTerm"
                    if not frontmost then return "behind"
                    if (unique ID of current session of current window) is (item 1 of argv) then return "ok"
                end tell
                return "other"
                """, arguments: [tab.id]) == "ok"
        }
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
    /// `text`, if given, is its `givenText`.
    private func withTerminal(_ tab: TabAddress, _ body: String, _ extra: [String] = [], text: String? = nil) -> String? {
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
            """, arguments: [id, tab.id] + extra, input: text)
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

    /// A key's bytes, all ASCII, as an argument: reading standard input costs osascript a moment more.
    private func typeBytes(_ bytes: String, in tab: TabAddress) -> Bool {
        withTerminal(tab, "perform action (\"text:\" & (item 3 of argv)) on t\nreturn \"ok\"", [Ghostty.escaped(bytes)]) == "ok"
    }

    /// Text on osascript's standard input, never as an argument: see `Terminal.run`.
    private func typeText(_ text: String, in tab: TabAddress) -> Bool {
        withTerminal(tab, "perform action (\"text:\" & givenText) on t\nreturn \"ok\"", text: Ghostty.escaped(text)) == "ok"
    }

    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard let bytes = keystroke.bytes else { return false }
        for i in 0..<max(keystroke.times, 1) {
            if i > 0 { Thread.sleep(forTimeInterval: Terminal.Keystroke.gap) }
            guard typeBytes(bytes, in: tab) else { return false }
        }
        return true
    }

    func type(_ text: String, in tab: TabAddress) -> Bool { typeText(text + "\r", in: tab) }

    /// As Ghostty pastes, without Return.
    func insert(_ text: String, in tab: TabAddress) -> Bool {
        withTerminal(tab, "input text givenText to t\nreturn \"ok\"", text: text) == "ok"
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

    /// Not read for Ghostty: the phone's peek says it can't show the input line.
    func screen(of tab: TabAddress) -> String? { nil }
    var showsScreen: Bool { false }

    /// Several lines through the clipboard and Ghostty's own paste (`paste_from_clipboard`, what ⌘V does),
    /// performed on this terminal, so nothing is brought forward: Ghostty wraps it in bracketed-paste
    /// markers where the agent asked for them, and asks you first, by default, where it didn't.
    var linesWay: LinesWay { .clipboard(inFront: false) }
    /// Not answered (it ended for taking too long), the paste may still come: the clipboard keeps the
    /// phone's text a while longer for it (see `ClipboardPaste.grace`), and no Return follows.
    func insertLines(_ text: String, in tab: TabAddress, ready: () -> String?) -> LinesPasted {
        ClipboardPaste.paste(text, forward: { Terminal.onlyLooking { isOpen(tab) == true } ? nil : .failed }, ready: ready,
                             press: { withTerminal(tab, "perform action \"paste_from_clipboard\" on t\nreturn \"ok\"") == "ok" ? .sent : .unsure })
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

    private func run(_ tab: TabAddress, _ arguments: [String], input: Data? = nil) -> (status: Int32, output: String)? {
        guard let path = WezTerm.path else { return nil }
        // The GUI's socket, where the hook gave it; otherwise wezterm finds the running GUI itself.
        let socket = tab.socket.isEmpty ? [] : ["WEZTERM_UNIX_SOCKET=\(tab.socket)"]
        return Tool.run("/usr/bin/env", socket + [path, "cli"] + arguments, input: input)
    }

    private func ran(_ tab: TabAddress, _ arguments: [String], input: Data? = nil) -> Bool { run(tab, arguments, input: input)?.status == 0 }

    /// `text` through send-text, which reads it from its standard input when it's given none: as an
    /// argument, its accents would come apart (see `Tool.run`).
    private func send(_ text: String, in tab: TabAddress, paste: Bool) -> Bool {
        ran(tab, ["send-text", "--pane-id", tab.id] + (paste ? [] : ["--no-paste"]), input: Data(text.utf8))
    }

    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard let bytes = keystroke.bytes else { return false }
        for i in 0..<max(keystroke.times, 1) {
            if i > 0 { Thread.sleep(forTimeInterval: Terminal.Keystroke.gap) }
            guard ran(tab, ["send-text", "--pane-id", tab.id, "--no-paste", bytes]) else { return false }
        }
        return true
    }

    func type(_ text: String, in tab: TabAddress) -> Bool {
        send(text, in: tab, paste: false) && ran(tab, ["send-text", "--pane-id", tab.id, "--no-paste", "\r"])
    }

    /// As a paste, which is send-text's own way, without Return.
    func insert(_ text: String, in tab: TabAddress) -> Bool { send(text, in: tab, paste: true) }

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

    /// The pane's visible lines, as text, without its colours.
    func screen(of tab: TabAddress) -> String? {
        guard let answer = run(tab, ["get-text", "--pane-id", tab.id]), answer.status == 0 else { return nil }
        return answer.output
    }
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

    private func run(_ tab: TabAddress, _ arguments: [String], errors: Bool = false, input: Data? = nil) -> (status: Int32, output: String)? {
        guard let path = Kitty.path else { return nil }
        return Tool.run(path, ["@", "--to", tab.socket] + arguments, errors: errors, input: input)
    }

    private func ran(_ tab: TabAddress, _ arguments: [String], input: Data? = nil) -> Bool { run(tab, arguments, input: input)?.status == 0 }

    /// `text` through send-text's standard input, which it takes as it is, with no escapes: as an
    /// argument, its accents would come apart (see `Tool.run`).
    private func send(_ text: String, in tab: TabAddress) -> Bool {
        ran(tab, ["send-text", "--match", "id:\(tab.id)", "--stdin"], input: Data(text.utf8))
    }

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

    func type(_ text: String, in tab: TabAddress) -> Bool { send(text + "\r", in: tab) }

    func insert(_ text: String, in tab: TabAddress) -> Bool { send(text, in: tab) }

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

    /// The window's visible lines, as text, without its colours.
    func screen(of tab: TabAddress) -> String? {
        guard let answer = run(tab, ["get-text", "--match", "id:\(tab.id)"]), answer.status == 0 else { return nil }
        return answer.output
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

    /// A hire as it opened: its tab as the deck will know it, and the window
    /// Smolcorp opened for it, by the id its terminal gave (nil for none, as a
    /// hire in the background).
    struct Opened: Equatable {
        let tab: String
        let window: String?
    }

    /// Opens the hire's window and gives its tab as the deck will know it,
    /// or nil if it didn't open. A Ghostty terminal's tab is given as
    /// ghostty-id:<terminal>, its tty being known only once its session
    /// reports. In tmux without `window` (agent windows in the background),
    /// no window opens: "Open a window" on its desk shows one. Off the main thread.
    static func open(_ command: String, in folder: String, terminal: String, inTmux: Bool, window showing: Bool = true, hireID: String) -> Opened? {
        guard inTmux, let tmux = Tmux.path, let desk = HireTmux(hireID: hireID) else {
            return window(command, in: folder, terminal: terminal).map { Opened(tab: HireWindows.tab(ofRecord: $0), window: $0) }
        }
        // The agent's own tmux server and session, through your login shell, so
        // it finds what your PATH finds; then a window showing it, unless it runs in the background.
        let shell = ProcessInfo.processInfo.environment["SHELL"].flatMap { $0.isEmpty ? nil : $0 } ?? "/bin/zsh"
        guard let made = desk.start(in: folder, shell: shell, running: command) else { return nil }
        var opened: String?
        if showing {
            guard let attach = script(desk.attachLine(tmuxPath: tmux)), let shown = window(attach, in: folder, terminal: terminal) else {
                // Not hired: its server goes, and nothing else's.
                if desk.isAlive() == true { _ = desk.kill() }
                return nil
            }
            opened = shown
        }
        return Opened(tab: "tmux:\(made.pane)@\(made.socket)", window: opened)
    }

    /// Shows a running tmux hire again in a new window of `terminal`, after its
    /// window was closed. The hire was never stopped; this only attaches.
    /// Gives the window it opened, by its terminal's id; nil for a socket that
    /// isn't one of ours, or a hire no longer there.
    @discardableResult
    static func reattach(socket: String, in folder: String, terminal: String) -> String? {
        guard HireTmux.owns(socket), let tmux = Tmux.path, let desk = HireTmux(socketName: URL(fileURLWithPath: socket).lastPathComponent),
              desk.isAlive() == true, let attach = script(desk.attachLine(tmuxPath: tmux)) else { return nil }
        return window(attach, in: folder, terminal: terminal)
    }

    /// Shows `line` (a shell line, as `TeamGrid.attachLine`) in a new window of `terminal`.
    static func show(line: String, terminal: String) -> Bool {
        guard let attach = script(line) else { return false }
        return window(attach, in: NSHomeDirectory(), terminal: terminal) != nil
    }

    /// tmux's arguments for a hire's session, `command` run by your login shell, which prints its pane and socket.
    static func session(_ name: String, in folder: String, shell: String, running command: String) -> [String] {
        ["new-session", "-d", "-P", "-F", "#{pane_id} #{socket_path}", "-s", name, "-c", folder, shell, "-lic", quoted(command)]
    }

    static func window(_ command: String, in folder: String, terminal: String) -> String? {
        switch terminal {
        case "iterm":
            return Terminal.run("""
                tell application "iTerm"
                    activate
                    set w to (create window with default profile)
                    tell current session of w to write text "cd " & quoted form of (item 1 of argv) & " && " & quoted form of (item 2 of argv)
                    return "iterm:" & (unique ID of current session of w)
                end tell
                """, arguments: [folder, command], timeout: Terminal.openTimeout).flatMap { $0.hasPrefix("iterm:") ? $0 : nil }
        case "ghostty":
            return Terminal.run("""
                tell application "Ghostty"
                    activate
                    set w to new window with configuration {initial working directory:(item 1 of argv), initial input:(quoted form of (item 2 of argv)) & linefeed}
                    return "ghostty-id:" & (id of focused terminal of selected tab of w)
                end tell
                """, arguments: [folder, command], timeout: Terminal.openTimeout).flatMap { $0.hasPrefix("ghostty-id:") ? $0 : nil }
        default:
            // The tab's tty and its window's id, both: macOS gives a closed tab's tty to a new one (see `HireWindows`).
            return Terminal.open(command, in: folder).map { HireWindows.terminalRecord(tty: $0.tty, windowID: $0.window) }
        }
    }

    /// A few lines for a hire's window to run once, which remove themselves as they run.
    static func script(_ line: String) -> String? {
        try? FileManager.default.createDirectory(at: Paths.hires, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        let url = Paths.hires.appendingPathComponent("attach-\(UUID().uuidString).sh")
        guard FileManager.default.createFile(atPath: url.path, contents: Data("#!/bin/sh\nrm -f -- \"$0\"\n\(line)\n".utf8),
                                             attributes: [.posixPermissions: 0o700]) else { return nil }
        return url.path
    }

    /// `text` as one word to a shell, whatever it holds: a hire's path has a space, in Application Support.
    static func quoted(_ text: String) -> String { "'" + text.replacingOccurrences(of: "'", with: "'\\''") + "'" }
}

/// Things being opened, by name, from any queue: a second ask while one is
/// on its way is told so (`insert` false), rather than opening another.
final class Openings {
    private let lock = NSLock()
    private var names = Set<String>()

    func insert(_ name: String) -> Bool {
        lock.lock()
        defer { lock.unlock() }
        return names.insert(name).inserted
    }

    func remove(_ name: String) {
        lock.lock()
        names.remove(name)
        lock.unlock()
    }
}
