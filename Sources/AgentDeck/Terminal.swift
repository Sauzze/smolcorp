import AppKit

/// Terminal.app: finding a session's tab, bringing it forward, typing into it.
///
/// A tab is identified by its tty. Terminal reports one for every tab and the
/// hook reports the one the agent runs on, so they meet on an exact string —
/// window and tab indexes shift as tabs move, a tty does not.
///
/// AppleScript runs through osascript, one call at a time on `queue`, so a slow
/// script never holds up the menu bar or the server. Every script checks that
/// Terminal is running first, because asking a closed Terminal anything
/// launches it.
enum Terminal {
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
        /// iPhone app (personal build): a menu's options, picked by their number.
        case one = 18, two = 19, three = 20, four = 21, five = 23, six = 22, seven = 26, eight = 28, nine = 25

        private static let digits: [Key] = [.one, .two, .three, .four, .five, .six, .seven, .eight, .nine]

        /// The digit 1 to 9, picking option `n` of a menu.
        static func digit(_ n: Int) -> Key? { digits.indices.contains(n - 1) ? digits[n - 1] : nil }

        /// What a digit types, whatever the keyboard's layout: on a French one, its key alone types "&".
        var character: String? { Key.digits.firstIndex(of: self).map { String($0 + 1) } }
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
    static func openTTYs() -> Set<String>? {
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
            """) else { return nil }
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
            """)
        return tty?.isEmpty == false ? tty : nil
    }

    static func focus(tty: String) -> Bool {
        withTab(tty, do: """
            set selected of t to true
            set index of w to 1
            activate
            """)
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
        let output = run("""
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
            """, arguments: [tty])
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
        guard withTab(tty, do: "set selected of t to true\nset index of w to 1"),
              let terminal = NSRunningApplication.runningApplications(withBundleIdentifier: bundleID).first?.processIdentifier else { return false }
        Thread.sleep(forTimeInterval: settle)
        post(key, modifiers: modifiers, times: times, to: terminal)
        return true
    }

    /// Puts `text` in the tab without Return, as typed characters: `do script`
    /// always ends with Return, so the characters go as key events carrying
    /// them, to Terminal itself, as `press` sends keys. Needs Accessibility.
    static func insert(_ text: String, tty: String) -> Bool {
        guard withTab(tty, do: "set selected of t to true\nset index of w to 1"),
              let terminal = NSRunningApplication.runningApplications(withBundleIdentifier: bundleID).first?.processIdentifier else { return false }
        Thread.sleep(forTimeInterval: settle)
        let units = Array(text.utf16)
        // A key event carries a few characters at most: the text goes in pieces.
        for start in stride(from: 0, to: units.count, by: 16) {
            let piece = Array(units[start..<min(start + 16, units.count)])
            for down in [true, false] {
                let event = CGEvent(keyboardEventSource: nil, virtualKey: 0, keyDown: down)
                event?.keyboardSetUnicodeString(stringLength: piece.count, unicodeString: piece)
                event?.postToPid(terminal)
            }
        }
        return true
    }

    /// Brings the tab forward and presses `key` in it, through the keyboard's
    /// own path: for Terminal's menu shortcuts, as scrolling uses. Never while
    /// the Mac is locked, when the app in front is the lock screen.
    private static func pressInFront(_ key: Key, modifiers: CGEventFlags = [], times: Int = 1, tty: String) -> Bool {
        guard !isLocked, focus(tty: tty), waitUntilFrontmost() else { return false }
        post(key, modifiers: modifiers, times: times)
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
    /// tab's tty, or nil if it didn't open. Both go through `quoted form of`,
    /// so each is taken as it is: the command is one of the agents' own
    /// names, or the path of a hire's start-up script.
    static func open(_ command: String, in folder: String) -> String? {
        run("""
            tell application "Terminal"
                activate
                set opened to do script "cd " & quoted form of (item 1 of argv) & " && " & quoted form of (item 2 of argv)
                return "tty:" & (tty of opened)
            end tell
            """, arguments: [folder, command]).flatMap { $0.hasPrefix("tty:") ? String($0.dropFirst(4)) : nil }
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
    /// Terminal. Posted any sooner, they land in the app you were using:
    /// Space in Finder opens Quick Look.
    private static func waitUntilFrontmost() -> Bool {
        let deadline = Date().addingTimeInterval(activationTimeout)
        while !isFrontmost {
            guard Date() < deadline else { return false }
            Thread.sleep(forTimeInterval: 0.01)
        }
        Thread.sleep(forTimeInterval: settle)
        return true
    }

    private static var isFrontmost: Bool {
        NSWorkspace.shared.frontmostApplication?.bundleIdentifier == bundleID
    }

    /// Posts `key` to the app in front, or, given `pid`, to that app alone.
    private static func post(_ key: Key, modifiers: CGEventFlags, times: Int, to pid: pid_t? = nil) {
        for _ in 0..<times {
            for down in [true, false] {
                let event = CGEvent(keyboardEventSource: nil, virtualKey: key.rawValue, keyDown: down)
                if !modifiers.isEmpty { event?.flags = modifiers }
                if let character = key.character {
                    let units = Array(character.utf16)
                    event?.keyboardSetUnicodeString(stringLength: units.count, unicodeString: units)
                }
                if let pid { event?.postToPid(pid) } else { event?.post(tap: .cghidEventTap) }
            }
        }
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

    /// Runs an AppleScript, passing `arguments` as argv rather than splicing
    /// them into the source. Nil if it failed.
    static func run(_ source: String, arguments: [String] = []) -> String? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/osascript")
        process.arguments = ["-e", "on run argv\n\(source)\nend run"] + arguments
        let output = Pipe()
        process.standardOutput = output
        process.standardError = FileHandle.nullDevice
        do { try process.run() } catch { return nil }
        let data = output.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        guard process.terminationStatus == 0 else { return nil }
        return String(decoding: data, as: UTF8.self).trimmingCharacters(in: .newlines)
    }
}
