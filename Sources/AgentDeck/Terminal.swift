import AppKit

/// Terminal.app: finding a session's tab, bringing it forward, typing into it.
///
/// A tab is identified by its tty. Terminal reports one for every tab and the
/// hook reports the one the agent runs on, so they meet on an exact string —
/// window and tab indexes shift as tabs move, a tty does not.
///
/// AppleScript runs through osascript, one call at a time on `queue`, so a slow
/// script never holds up the menu bar or the server; the looks at Terminal's
/// tabs run on pollers of their own (`Poller`), and each script has a time
/// limit. Every script checks that Terminal is running first, because asking
/// a closed Terminal anything launches it.
enum Terminal {
    /// Terminal.app's tabs' queue: their keys, text, windows opened. Other
    /// terminals each have their own (`Terminals.queue(for:)`).
    static let queue = DispatchQueue(label: "io.github.sauzze.agentdeck.terminal")
    private static let bundleID = "com.apple.Terminal"

    enum Key: CGKeyCode {
        case returnKey = 36
        case escape = 53
        case space = 49
        case up = 126
        case down = 125
        case pageUp = 116
        case pageDown = 121
        case end = 119
        case c = 8
        case tab = 48
    }

    /// A key, with its modifiers, pressed `times` times in a row.
    struct Keystroke {
        let key: Key
        var modifiers: CGEventFlags = []
        var times = 1
    }

    /// The most lines one request may scroll. A notch of the phone's wheel
    /// is a few; anything beyond is clamped, not obeyed.
    static let maxScroll = 200

    /// How long Terminal may take to come to the front. It usually needs about
    /// 200 ms, more on a busy Mac; past this, the press is given up.
    private static let activationTimeout: TimeInterval = 1.5
    /// A moment more once it is in front, for its window to become key.
    private static let settle: TimeInterval = 0.05
    /// Notches arriving within this long of the last scroll of the same tab
    /// belong to one turn of the wheel, and the tab is still in front.
    private static let scrollGesture: TimeInterval = 1
    /// The tab the last scroll went to, and when. Only touched on `queue`.
    private static var lastScroll: (tty: String, at: Date)?

    /// The ttys of every open tab: none if Terminal isn't running, nil if it
    /// could not be asked. Asked for all at once: windows walked one by one
    /// shift under the walk as one opens, closes or comes to the front, and a
    /// window skipped would look closed.
    static func openTTYs(timeout: TimeInterval = pollTimeout) -> Set<String>? {
        guard let output = run("""
            if application "Terminal" is not running then return ""
            tell application "Terminal" to set byWindow to tty of every tab of every window
            set ttys to {}
            repeat with tabsOfWindow in byWindow
                repeat with t in tabsOfWindow
                    set end of ttys to contents of t
                end repeat
            end repeat
            set AppleScript's text item delimiters to linefeed
            return ttys as text
            """, timeout: timeout) else { return nil }
        return Set(output.split(separator: "\n").map(String.init))
    }

    /// Whether a process called `name` is running on `tty`, in any terminal
    /// app: nil if that couldn't be looked at. A tab that has closed takes its
    /// processes with it.
    static func isRunning(_ name: String, on tty: String) -> Bool? {
        commands(on: tty, "comm=").map { runs(any: [name], among: $0) }
    }

    /// Whether `agent` itself runs on `tty`, judged by each process's command
    /// line: see `Agent.isItself(commandLine:)`. Nil if that couldn't be looked at.
    static func isRunning(_ agent: Agent, on tty: String) -> Bool? {
        commands(on: tty, "args=").map { $0.split(separator: "\n").contains { agent.isItself(commandLine: String($0)) } }
    }

    /// Where an agent is in its tab, for its keys and text.
    enum Place {
        /// In front, where they land.
        case front
        /// Stopped with Ctrl-Z or sent to the background: they'd go to the
        /// shell, and run as a command, or to the job in front.
        case behind
        /// Not running there any more.
        case gone
    }

    /// Where `agent` is on `tty`, whatever its terminal: nil if that couldn't be looked at.
    static func place(of agent: Agent, on tty: String) -> Place? {
        guard let running = isRunning(agent, on: tty) else { return nil }
        guard running else { return .gone }
        // A tab's shell is started under `login`, a pane's isn't: its own process is the one to go by.
        let underLogin = foreground(tty: tty)
        let root = underLogin == nil ? Firing.root(on: tty) : nil
        switch underLogin ?? root.flatMap({ foreground(tty: tty, shell: $0) }) {
        case nil:
            return nil
        case .job(_, _, let commandLines)?:
            return commandLines.contains(where: agent.isItself) ? .front : .behind
        case .shell?:
            // A pane started with the agent itself has it as its own process.
            guard let root, let line = Tool.run("/bin/ps", ["-p", String(root), "-o", "args="])?.output else { return .behind }
            return agent.isItself(commandLine: line) ? .front : .behind
        }
    }

    /// What `ps` says of each process on `tty`, one per line, in `format`.
    private static func commands(on tty: String, _ format: String) -> String? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/ps")
        process.arguments = ["-t", (tty as NSString).lastPathComponent, "-o", format]
        let output = Pipe()
        process.standardOutput = output
        process.standardError = FileHandle.nullDevice
        do { try process.run() } catch { return nil }
        let data = output.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        // No such tty any more is an answer too: nothing runs on it.
        return String(decoding: data, as: UTF8.self)
    }

    /// Whether `ps -o comm=`'s lines, `commands`, name any of `names`.
    static func runs(any names: Set<String>, among commands: String) -> Bool {
        let wanted = Set(names.map { $0.lowercased() })
        return commands.split(separator: "\n").contains {
            wanted.contains(($0.trimmingCharacters(in: .whitespaces) as NSString).lastPathComponent.lowercased())
        }
    }

    /// The tab you are looking at, if Terminal is the active app.
    static func frontTTY() -> String? {
        let tty = run("""
            if application "Terminal" is not running then return ""
            tell application "Terminal"
                if not frontmost or (count of windows) is 0 then return ""
                return tty of selected tab of front window
            end tell
            """, timeout: pollTimeout)
        return tty?.isEmpty == false ? tty : nil
    }

    static func focus(tty: String) -> Bool {
        onlyLooking {
            withTab(tty, do: """
                set selected of t to true
                set index of w to 1
                activate
                """)
        }
    }

    /// Types `text` and Return into the tab without bringing it forward.
    /// `do script` writes to the tab's input as characters, so the keyboard
    /// layout does not matter, and ends with the byte Enter sends.
    static func type(_ text: String, tty: String) -> Bool {
        withTab(tty, do: "do script (item 2 of argv) in t", extra: [text])
    }

    /// What the tab shows, as text (its `contents`: the window's lines, not
    /// the scrollback), without bringing it forward. Nil if it isn't there.
    static func contents(tty: String) -> String? {
        let output = onlyLooking { run("""
            set target to item 1 of argv
            if application "Terminal" is not running then return "no"
            tell application "Terminal"
                repeat with w in windows
                    repeat with t in tabs of w
                        if tty of t is target then return "yes" & (contents of t)
                    end repeat
                end repeat
            end tell
            return "no"
            """, arguments: [tty]) }
        guard let output, output.hasPrefix("yes") else { return nil }
        return String(output.dropFirst(3))
    }

    /// Presses `key`, `times` times, in the tab without bringing Terminal
    /// forward: the tab is selected and its window put first among
    /// Terminal's, which makes it the one Terminal gives keys to, and the keys
    /// are posted to Terminal itself. So they reach the agent with Terminal
    /// behind other apps, under the screen saver and with the Mac locked, and
    /// never the app in front or the lock screen. Needs Accessibility.
    static func press(_ key: Key, modifiers: CGEventFlags = [], times: Int = 1, tty: String) -> Bool {
        // Selecting the tab types nothing: a Terminal that didn't answer there has had no key.
        guard onlyLooking({ withTab(tty, do: "set selected of t to true\nset index of w to 1") }),
              let terminal = runningPID() else { return false }
        Thread.sleep(forTimeInterval: settle)
        post(key, modifiers: modifiers, times: times, to: terminal)
        // The keys have gone: a later step that stops answering is "part way", not "nothing typed".
        noteStep()
        return true
    }

    /// Puts `text` in the tab without Return, as typed characters: `do script`
    /// always ends with Return, so the characters go as key events carrying
    /// them, to Terminal itself, as `press` sends keys. Needs Accessibility.
    static func insert(_ text: String, tty: String) -> Bool {
        // Selecting the tab types nothing: a Terminal that didn't answer there has had no key.
        guard onlyLooking({ withTab(tty, do: "set selected of t to true\nset index of w to 1") }),
              let terminal = runningPID() else { return false }
        Thread.sleep(forTimeInterval: settle)
        let units = Array(text.utf16)
        // A key event carries a few characters at most: the text goes in pieces.
        for start in stride(from: 0, to: units.count, by: 16) {
            let piece = Array(units[start..<min(start + 16, units.count)])
            for down in [true, false] {
                let event = CGEvent(keyboardEventSource: nil, virtualKey: 0, keyDown: down)
                event?.keyboardSetUnicodeString(stringLength: piece.count, unicodeString: piece)
                if let event { postEvent(event, terminal) }
            }
        }
        // The text has gone: a later step that stops answering is "part way", not "nothing typed".
        noteStep()
        return true
    }

    /// Brings the tab forward and presses `key` in it, through the keyboard's
    /// own path: for Terminal's menu shortcuts, as scrolling uses. Never while
    /// the Mac is locked, when the app in front is the lock screen.
    private static func pressInFront(_ key: Key, modifiers: CGEventFlags = [], times: Int = 1, tty: String) -> Bool {
        guard !isLocked, focus(tty: tty), waitUntilFrontmost() else { return false }
        post(key, modifiers: modifiers, times: times)
        noteStep()
        return true
    }

    /// Whether the Mac is locked, as under a screen saver that asks for the password.
    static var isLocked: Bool {
        (CGSessionCopyCurrentDictionary() as? [String: Any])?["CGSSessionScreenIsLocked"] != nil
    }

    /// Scrolls the tab's window by `lines`, up when negative, with the View
    /// menu's Line Up and Line Down (⌥⌘Page Up, ⌥⌘Page Down). They move
    /// Terminal's view of the scrollback, so the agent itself receives nothing.
    ///
    /// Only a turn's first notch brings the tab forward. Doing it for every
    /// notch would make the wheel lag; a tab you switch to in the meantime
    /// by hand gets the rest of that turn.
    static func scroll(lines: Int, tty: String) -> Bool {
        let count = min(abs(lines), maxScroll)
        guard count > 0 else { return true }
        let key: Key = lines < 0 ? .pageUp : .pageDown
        let modifiers: CGEventFlags = [.maskCommand, .maskAlternate]
        if let last = lastScroll, last.tty == tty, Date().timeIntervalSince(last.at) < scrollGesture, isFrontmost, !isLocked {
            post(key, modifiers: modifiers, times: count)
        } else {
            guard pressInFront(key, modifiers: modifiers, times: count, tty: tty) else { return false }
        }
        lastScroll = (tty, Date())
        return true
    }

    /// The View menu's Scroll to Bottom (⌘End): back to where the agent is.
    static func scrollToBottom(tty: String) -> Bool {
        pressInFront(.end, modifiers: .maskCommand, tty: tty)
    }

    /// Opens a new window in `folder`, running `command`, and gives its
    /// tab's tty and the window's id, or nil if it didn't open. Both go
    /// through `quoted form of`, so each is taken as it is: the command is
    /// one of the agents' own names, or the path of a hire's start-up script.
    /// The id is only asked for: without it, the window still opened.
    static func open(_ command: String, in folder: String) -> (tty: String, window: String?)? {
        let opened = run("""
            tell application "Terminal"
                activate
                set opened to do script "cd " & quoted form of (item 1 of argv) & " && " & quoted form of (item 2 of argv)
                set found to tty of opened
                try
                    repeat with w in windows
                        repeat with t in tabs of w
                            if tty of t is found then return "tty:" & found & linefeed & (id of w)
                        end repeat
                    end repeat
                end try
                return "tty:" & found
            end tell
            """, arguments: [folder, command], timeout: openTimeout)?.split(separator: "\n").map(String.init) ?? []
        guard let first = opened.first, first.hasPrefix("tty:") else { return nil }
        let window = opened.count == 2 && !opened[1].isEmpty && opened[1].allSatisfy(\.isNumber) ? opened[1] : nil
        return (String(first.dropFirst(4)), window)
    }

    /// What firing came to: see `fire(tty:agent:)`.
    enum Fired {
        /// The agent has stopped, and its tab is closed, or left as the
        /// terminal keeps it.
        case fired
        /// Something else is in the tab's foreground, or the agent would not
        /// stop: the tab is left as it was.
        case notStopped
        /// No Terminal tab is on that tty.
        case gone
    }

    /// Stops the agent in the tab on `tty`, then closes the tab. Only the
    /// agent is stopped: a tab whose foreground job is something else, such as
    /// a server started after the agent ended, is left alone. With only the
    /// shell left, the agent has ended already and the tab just closes.
    ///
    /// The agent's job is asked to end, then made to, and the tab is closed
    /// only once none of it is left, so Terminal closes it without asking.
    static func fire(tty: String, agent: Agent) -> Fired {
        // Unknown is not the same as none: nothing is touched.
        guard let open = openTTYs() else { return .notStopped }
        // A tab in another terminal app can't be closed from here, so its
        // agent is not stopped either.
        guard open.contains(tty) else { return .gone }
        switch foreground(tty: tty) {
        case nil:
            return .notStopped
        case .job(let job, _, let commandLines)?:
            guard commandLines.contains(where: agent.isItself), stop(job) else { return .notStopped }
            // The agent has gone, and if another job has taken its place, or
            // one runs in the background, the tab is that job's now.
            if case .job? = foreground(tty: tty) { return .fired }
            if hasOtherJobs(tty: tty) != false { return .fired }
        case .shell?:
            // A job in the background would be hung up with its tab.
            guard hasOtherJobs(tty: tty) == false else { return .notStopped }
        }
        closeTab(tty: tty)
        return .fired
    }

    /// Closes the tab on `tty`, which only its shell is left in. Terminal
    /// counts each tab of a window as a window of its own, which `close`
    /// closes alone; it can't close a tab. A tab it counts among others is
    /// sent `exit 0` instead: the shell ends, cleanly whatever the agent
    /// ended with, and the profile's "When the shell exits" closes the tab
    /// or keeps it as [Process completed].
    private static func closeTab(tty: String) {
        _ = withTab(tty, do: """
            if (count of tabs of w) is 1 then
                close w
            else
                do script "exit 0" in t
            end if
            """)
    }

    /// How long a job is given to end when asked, then when made to.
    private static let stopGrace: [(signal: Int32, wait: TimeInterval)] = [(SIGTERM, 3), (SIGKILL, 1)]

    /// Stops the process group `job`, and whether none of it is left.
    static func stop(_ job: pid_t) -> Bool {
        let isGone = { kill(-job, 0) == -1 && errno == ESRCH }
        for (signal, wait) in stopGrace {
            kill(-job, signal)
            let deadline = Date().addingTimeInterval(wait)
            while Date() < deadline {
                if isGone() { return true }
                Thread.sleep(forTimeInterval: 0.1)
            }
        }
        return isGone()
    }

    /// What runs in the foreground of a tab.
    enum Foreground {
        /// Nothing but the shell, or not even that once the shell has ended.
        case shell
        /// A job: its process group, and the names and command lines of the processes in it.
        case job(pid_t, names: Set<String>, commandLines: [String] = [])
    }

    /// The foreground of the tab on `tty`, or nil if `ps` couldn't be asked.
    /// Terminal starts each tab's shell under `login`, and a job is any
    /// process group in front of that shell.
    static func foreground(tty: String) -> Foreground? {
        foreground(tty: tty, shell: nil)
    }

    /// As above, for a tab whose shell isn't started under `login`, as a tmux
    /// pane's: `shell` is the process the pane was started with.
    static func foreground(tty: String, shell given: pid_t?) -> Foreground? {
        guard let rows = processes(on: tty) else { return nil }
        // Nothing left at all: the shell has ended too.
        guard let tpgid = rows.first?.tpgid else { return .shell }
        guard let shell = shell(among: rows, given: given) else { return nil }
        guard tpgid > 0, tpgid != shell.pgid else { return .shell }
        let job = rows.filter { $0.pgid == tpgid }
        return .job(tpgid, names: Set(job.map(\.name)), commandLines: job.map(\.args))
    }

    /// The processes in the foreground job of `tty`, each with its command
    /// line: empty when nothing is in front, nil if `ps` couldn't be asked.
    static func foregroundJob(tty: String) -> [(pid: pid_t, args: String)]? {
        guard let rows = processes(on: tty) else { return nil }
        guard let tpgid = rows.first?.tpgid, tpgid > 0 else { return [] }
        return rows.filter { $0.pgid == tpgid }.map { ($0.pid, $0.args) }
    }

    /// What's in front in a tab, for a paste of several lines (RI34b B1):
    /// where `agent` is, and what else in its foreground job reads the tab's
    /// keys (RI34c S1).
    struct TabFront: Equatable {
        let place: Place
        /// The names of the processes in the foreground job, other than the
        /// agent itself, that have the tab's tty as their standard input: a
        /// terminal editor or a pager the agent opened, a shell. Whichever it
        /// is, the paste would go to it, not to the agent. Only for saying
        /// which: having the tty is what counts, whatever the name.
        let readers: Set<String>
    }

    /// `agent`'s tab on `tty` as `TabFront` says it; nil if any of it couldn't be looked at.
    static func front(of agent: Agent, on tty: String) -> TabFront? {
        guard let place = place(of: agent, on: tty), let job = foregroundJob(tty: tty),
              let readers = readers(of: agent, in: job, tty: tty) else { return nil }
        return TabFront(place: place, readers: readers)
    }

    /// Which processes of `job`, other than `agent` itself, have `tty` as
    /// their standard input (RI34c S1), by name: the program's own (`name`,
    /// the path whole, spaces and all) and its command line's first word, as
    /// `code`, a script, runs as bash. Nil if one couldn't be looked at: not
    /// known is refused. A hook's or the status line's `sh -c`, an MCP server
    /// or `caffeinate` has a pipe or /dev/null there, and reads none of the
    /// tab's keys. The one exception is the system's `caffeinate` (FXI34d), which owners run in front of
    /// the agent (`caffeinate -di claude`): it has the tab as its input but never reads it. It's known by
    /// its program's whole path, `/usr/bin/caffeinate`, so a name alone exempts nothing. Settable lookups
    /// for tests, which give stand-ins.
    static func readers(of agent: Agent, in job: [(pid: pid_t, args: String)], tty: String,
                        input: (pid_t) -> Processes.StandardInput = Processes.standardInput(of:),
                        name: (pid_t) -> String? = Processes.programName(of:),
                        path: (pid_t) -> String? = Processes.programPath(of:)) -> Set<String>? {
        let device = tty.hasPrefix("/dev/") ? tty : "/dev/" + tty
        let named = { (word: String) in (word as NSString).lastPathComponent.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "-")) }
        var readers: Set<String> = []
        for process in job where !agent.isItself(commandLine: process.args) {
            switch input(process.pid) {
            case .unknown: return nil
            case .other: continue
            case .file(let inputPath):
                guard inputPath == device else { continue }
                if path(process.pid) == neverReads { continue }
                let first = process.args.split(separator: " ").first.map(String.init)
                let names = Set([name(process.pid), first].compactMap { $0.map(named) }.filter { !$0.isEmpty })
                // Unnamed, it still reads the keys.
                readers.formUnion(names.isEmpty ? ["?"] : names)
            }
        }
        return readers
    }

    /// The one program that has the tab's tty as its input and never reads it (see `readers`).
    static let neverReads = "/usr/bin/caffeinate"

    /// Whether a job other than the shell runs on `tty`, as one sent to the
    /// background with `&` or stopped with Ctrl-Z: closing the tab would hang
    /// it up. `shell` as for `foreground`. Nil if that couldn't be looked at.
    static func hasOtherJobs(tty: String, shell given: pid_t? = nil) -> Bool? {
        guard let rows = processes(on: tty) else { return nil }
        guard !rows.isEmpty else { return false }
        guard let shell = shell(among: rows, given: given) else { return nil }
        return rows.contains { $0.pgid != shell.pgid && $0.name != "login" }
    }

    /// A process on a tty, as `ps` gives it.
    private typealias Row = (pid: Int32, ppid: Int32, pgid: Int32, tpgid: Int32, name: String, args: String)

    /// The tab's shell among its processes: `given`, or the one Terminal
    /// started under `login`. A tab with no shell of Terminal's own, such as
    /// a profile running a command of its own, can't be told apart from what
    /// runs in it.
    private static func shell(among rows: [Row], given: pid_t?) -> Row? {
        let logins = Set(rows.filter { $0.name == "login" }.map(\.pid))
        return given.map { pid in rows.first { $0.pid == pid } } ?? rows.first { logins.contains($0.ppid) }
    }

    /// The processes on `tty`, or nil if `ps` couldn't be asked.
    private static func processes(on tty: String) -> [Row]? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/ps")
        process.arguments = ["-t", (tty as NSString).lastPathComponent, "-o", "pid=,ppid=,pgid=,tpgid=,args="]
        let output = Pipe()
        process.standardOutput = output
        process.standardError = FileHandle.nullDevice
        do { try process.run() } catch { return nil }
        let data = output.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        return String(decoding: data, as: UTF8.self).split(separator: "\n").compactMap {
            line -> Row? in
            let fields = line.split(separator: " ", maxSplits: 4, omittingEmptySubsequences: true)
            guard fields.count == 5, let pid = Int32(fields[0]), let ppid = Int32(fields[1]), let pgid = Int32(fields[2]),
                  let tpgid = Int32(fields[3]) else { return nil }
            // The program's name is the first word of its command line.
            let program = fields[4].split(separator: " ").first.map(String.init) ?? ""
            return (pid, ppid, pgid, tpgid, (program as NSString).lastPathComponent, String(fields[4]))
        }
    }

    /// Keys go to whichever app is in front, so they wait until that is
    /// Terminal (or `bundle`'s app). Posted any sooner, they land in the app
    /// you were using: Space in Finder opens Quick Look.
    static func waitUntilFrontmost(_ bundle: String = bundleID) -> Bool {
        let deadline = Date().addingTimeInterval(activationTimeout)
        while !isFrontmost(bundle) {
            guard Date() < deadline else { return false }
            Thread.sleep(forTimeInterval: 0.01)
        }
        Thread.sleep(forTimeInterval: settle)
        return true
    }

    private static var isFrontmost: Bool { isFrontmost(bundleID) }

    private static func isFrontmost(_ bundle: String) -> Bool {
        NSWorkspace.shared.frontmostApplication?.bundleIdentifier == bundle
    }

    // MARK: Several lines through the clipboard (I34, owner's Q1 b)

    /// The tab brought forward for ⌘V (see `ClipboardPaste`): never while the
    /// Mac is locked, when the app in front is the lock screen. True once
    /// Terminal is in front with this very tab selected in its front window,
    /// asked again after it came forward. Types nothing.
    static func forwardForPaste(tty: String) -> Bool {
        guard !isLocked, focus(tty: tty), waitUntilFrontmost() else { return false }
        return onlyLooking { frontTTY() } == tty
    }

    /// ⌘V in this very tab (RI34b B2): asked again right before, whether it's
    /// still the selected tab of Terminal's front window, as a click on
    /// another tab or window could have changed since it was brought forward.
    /// Not, and nothing is pressed. `posting` as for `pressPaste(in:)`.
    static func pressPaste(tty: String, posting: () -> Void = {}) -> PastePress {
        guard onlyLooking({ frontTTY() }) == tty else { return .notInFront }
        return pressPaste(in: bundleID, posting: posting) ? .sent : .notInFront
    }

    /// ⌘V through the keyboard's own path, as Terminal's menu shortcuts go
    /// (see `pressInFront`): to the app in front, only while that is still
    /// `bundle`'s, so it never lands in the app you switched to. `posting` is
    /// called right before the key goes, once every check has passed: the
    /// clipboard's reads count from then on (RI34c N2).
    static func pressPaste(in bundle: String, posting: () -> Void = {}) -> Bool {
        guard !isLocked, isFrontmost(bundle) else { return false }
        posting()
        for down in [true, false] {
            let event = CGEvent(keyboardEventSource: nil, virtualKey: pasteKey, keyDown: down)
            event?.flags = .maskCommand
            // "v", whatever the keyboard's layout says that key types.
            event?.keyboardSetUnicodeString(stringLength: 1, unicodeString: [0x76])
            if let event { postEvent(event, nil) }
        }
        noteStep()
        return true
    }

    /// Whether you're using the Mac: keyboard or mouse input in the last
    /// `inUseWindow` seconds (VICTOR's ruling on RI34b S3). Terminal.app and
    /// iTerm2 come to the front for ⌘V, which would take your focus, and your
    /// keys could land in the agent's tab: refused then. Ghostty pastes where
    /// it is, so it never asks.
    static var macInUse: Bool { inUse(sinceInput: secondsSinceInput(), sinceOwnKeys: lastOwnKeys.map { -$0.timeIntervalSinceNow }) }

    static let inUseWindow: TimeInterval = 10

    /// In use, given how long ago the Mac last had input and how long ago
    /// Smolcorp last posted keys of its own (nil: not since it started): the
    /// Mac may count those as input too, and they aren't you.
    static func inUse(sinceInput: TimeInterval, sinceOwnKeys: TimeInterval?) -> Bool {
        guard sinceInput < inUseWindow else { return false }
        guard let sinceOwnKeys else { return true }
        return sinceInput < sinceOwnKeys - 0.25
    }

    /// Seconds since the Mac's keyboard or mouse was last used. Settable for
    /// tests: the stand-in clock.
    static var secondsSinceInput: () -> TimeInterval = {
        CGEventType(rawValue: ~0).map { CGEventSource.secondsSinceLastEventType(.hidSystemState, eventType: $0) } ?? 0
    }

    /// When Smolcorp last posted keys through the keyboard's path.
    static var lastOwnKeys: Date? {
        get { ownKeysLock.withLock { ownKeys } }
        set { ownKeysLock.withLock { ownKeys = newValue } }
    }
    private static var ownKeys: Date?
    private static let ownKeysLock = NSLock()

    /// V's key, which pastes with ⌘.
    private static let pasteKey: CGKeyCode = 9

    /// Posts `key` to the app in front, or, given `pid`, to that app alone.
    private static func post(_ key: Key, modifiers: CGEventFlags, times: Int, to pid: pid_t? = nil) {
        for _ in 0..<times {
            for down in [true, false] {
                let event = CGEvent(keyboardEventSource: nil, virtualKey: key.rawValue, keyDown: down)
                if !modifiers.isEmpty { event?.flags = modifiers }
                if let event { postEvent(event, pid) }
            }
        }
    }

    /// Sends a key event to `pid`, or with none to the app in front. Settable
    /// for tests, which give one that posts nothing: no key of theirs ever
    /// reaches a real app.
    static var postEvent: (CGEvent, pid_t?) -> Void = { event, pid in
        if let pid { event.postToPid(pid) } else {
            lastOwnKeys = Date()
            event.post(tap: .cghidEventTap)
        }
    }

    /// Terminal's pid while it runs. Settable for tests, with `postEvent`.
    static var runningPID: () -> pid_t? = {
        NSRunningApplication.runningApplications(withBundleIdentifier: bundleID).first?.processIdentifier
    }

    /// Runs `body` with `w` and `t` bound to the window and tab on `tty`.
    private static func withTab(_ tty: String, do body: String, extra: [String] = []) -> Bool {
        let output = run("""
            set target to item 1 of argv
            if application "Terminal" is not running then return "no"
            tell application "Terminal"
                repeat with w in windows
                    repeat with t in tabs of w
                        if tty of t is target then
                            \(body)
                            return "yes"
                        end if
                    end repeat
                end repeat
            end tell
            return "no"
            """, arguments: [tty] + extra)
        return output == "yes"
    }

    /// How long a look at Terminal's tabs may take. It usually answers in a
    /// fraction of a second; one that doesn't answer, as while macOS asks
    /// whether Smolcorp may control it, is ended and asked again next round.
    static let pollTimeout: TimeInterval = 3
    /// How long any other script may take: a tab found, a key's tab selected, a line typed.
    static let scriptTimeout: TimeInterval = 4
    /// How long opening a window may take: Terminal may have to start first,
    /// or macOS ask you whether Smolcorp may control it.
    static let openTimeout: TimeInterval = 30

    /// osascript's path. Settable for tests, which give a stand-in: they never ask a real terminal anything.
    static var osascript = "/usr/bin/osascript"

    /// Runs an AppleScript, passing `arguments` as argv rather than splicing
    /// them into the source. Nil if it failed, or took longer than `timeout`:
    /// then osascript is ended by its pid, so a terminal that doesn't answer
    /// never holds up what waits behind it for long (see `noticingTimeouts`).
    /// `input`, if given, is the script's `givenText`, read from osascript's standard input:
    /// as an argument, its accents would come apart (see `Tool.run`).
    static func run(_ source: String, arguments: [String] = [], input: String? = nil, timeout: TimeInterval = scriptTimeout) -> String? {
        if leavesAlone(source) { return nil }
        let process = Process()
        process.executableURL = URL(fileURLWithPath: osascript)
        process.arguments = ["-e", script(source, reading: input != nil)] + arguments
        let output = Pipe()
        process.standardOutput = output
        process.standardError = FileHandle.nullDevice
        let given = input.map { _ in Pipe() }
        if let given { process.standardInput = given }
        do { try process.run() } catch { return nil }
        if let given, let input { Tool.feed(Data(input.utf8), to: given) }
        noteStep()
        let pid = process.processIdentifier
        // Ended when it overruns, which also ends the reading below.
        let overran = DispatchWorkItem { if process.isRunning { kill(pid, SIGKILL) } }
        DispatchQueue.global().asyncAfter(deadline: .now() + timeout, execute: overran)
        let data = output.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        overran.cancel()
        guard process.terminationReason == .exit else {
            noteTimeout()
            return nil
        }
        guard process.terminationStatus == 0 else { return nil }
        return String(decoding: data, as: UTF8.self).trimmingCharacters(in: .newlines)
    }

    /// `source` as osascript runs it. Reading `input`, its first line reads the
    /// standard input whole, as UTF-8, before any app is told anything.
    static func script(_ source: String, reading input: Bool) -> String {
        guard input else { return "on run argv\n\(source)\nend run" }
        return """
            use framework "Foundation"
            use scripting additions
            on run argv
            set givenText to (current application's NSString's alloc()'s initWithData:(current application's NSFileHandle's \
            fileHandleWithStandardInput()'s readDataToEndOfFile()) encoding:(current application's NSUTF8StringEncoding)) as text
            \(source)
            end run
            """
    }

    /// Whether `source` is left unrun because it asks Terminal.app something and this is a test
    /// copy started with SMOLCORP_TEST_NO_TERMINAL_APP=1: unknown, as when Terminal can't be asked.
    static func leavesAlone(_ source: String, alone: Bool = TestCopy.leavesTerminalAppAlone) -> Bool {
        alone && source.contains(#"application "Terminal""#)
    }

    private static let timedOutKey = "io.github.sauzze.agentdeck.timedOut"
    private static let mayHaveTypedKey = "io.github.sauzze.agentdeck.mayHaveTyped"
    private static let lookingKey = "io.github.sauzze.agentdeck.looking"

    /// `body`'s value, whether a script or a tool it ran on this thread was
    /// ended for taking too long, and if so whether anything may have reached
    /// the tab by then (`partWay`): a key that didn't go because its terminal
    /// didn't answer is told apart from a tab that's gone, and "nothing was
    /// typed" from "part of it may have been". Only a step that just looks, or
    /// selects (`onlyLooking`), is known to type nothing.
    static func noticingTimeouts<T>(_ body: () -> T) -> (value: T, timedOut: Bool, partWay: Bool) {
        let thread = Thread.current.threadDictionary
        let outer = (thread[timedOutKey], thread[mayHaveTypedKey])
        thread[timedOutKey] = false
        thread[mayHaveTypedKey] = false
        let value = body()
        let timedOut = thread[timedOutKey] as? Bool == true
        let typed = thread[mayHaveTypedKey] as? Bool == true
        thread[timedOutKey] = outer.0.map { ($0 as? Bool == true) || timedOut }
        thread[mayHaveTypedKey] = outer.1.map { ($0 as? Bool == true) || typed }
        return (value, timedOut, timedOut && typed)
    }

    /// A script or a tool was ended for taking too long, on this thread.
    static func noteTimeout() {
        let thread = Thread.current.threadDictionary
        if thread[timedOutKey] != nil { thread[timedOutKey] = true }
    }

    /// A script or a tool has started on this thread, or Terminal's keys have
    /// been posted: unless it only looks, what it does may reach the tab even
    /// if it's ended later.
    static func noteStep() {
        let thread = Thread.current.threadDictionary
        if thread[mayHaveTypedKey] != nil, thread[lookingKey] as? Bool != true { thread[mayHaveTypedKey] = true }
    }

    /// Runs `body`, whose scripts and tools only look at a tab or select it:
    /// one of them ended for taking too long has typed nothing.
    static func onlyLooking<T>(_ body: () -> T) -> T {
        let thread = Thread.current.threadDictionary
        let outer = thread[lookingKey]
        thread[lookingKey] = true
        defer { thread[lookingKey] = outer }
        return body()
    }
}
