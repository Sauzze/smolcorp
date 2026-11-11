import AppKit

/// The two plain choices of first setup and Settings (T796): T3 Code on or
/// off, and agent windows (a window for each hire, or hires in the background).
enum Choices {
    /// T3 Code's desktop app, as macOS knows it.
    static let t3BundleID = "com.t3tools.t3code"

    /// The T3 Code gate: everything of T3 Code's (a button, a hint, looking
    /// for it, running `t3`) asks this first, and does nothing when it's no.
    static func t3Code(_ config: Config = .shared) -> Bool { config.t3CodeEnabled }

    /// Where a `t3` command may be, in the order T3 Code's own CLI install
    /// looks: an app on the Mac doesn't see the shell's PATH, so these first.
    static func t3Folders(home: String, path: String?) -> [String] {
        let fixed = [".local/bin", "bin", ".t3/bin"].map { (home as NSString).appendingPathComponent($0) } + ["/opt/homebrew/bin", "/usr/local/bin"]
        let onPath = (path ?? "").split(separator: ":").map(String.init).filter { $0.hasPrefix("/") }
        var seen = Set<String>()
        return (fixed + onPath).filter { seen.insert($0).inserted }
    }

    /// Whether T3 Code looks installed here: its app by bundle id, or a `t3`
    /// file in one of `t3Folders`. Existence only: `t3` is never run.
    static func t3Installed(home: String = NSHomeDirectory(), path: String? = ProcessInfo.processInfo.environment["PATH"],
                            appFound: () -> Bool = { NSWorkspace.shared.urlForApplication(withBundleIdentifier: t3BundleID) != nil },
                            isExecutable: (String) -> Bool = { FileManager.default.isExecutableFile(atPath: $0) }) -> Bool {
        appFound() || t3Folders(home: home, path: path).contains { isExecutable(($0 as NSString).appendingPathComponent("t3")) }
    }

    /// The last look, for 30 s: setup asks for its state often.
    private static var lastLook: (at: Date, installed: Bool)?
    private static let lookLock = NSLock()

    static func t3InstalledRecently() -> Bool {
        lookLock.lock(); defer { lookLock.unlock() }
        if let last = lastLook, Date().timeIntervalSince(last.at) < 30 { return last.installed }
        let installed = t3Installed()
        lastLook = (Date(), installed)
        return installed
    }

    /// Whether hires run in the background now: chosen, and tmux here.
    static func background(_ config: Config = .shared, hasTmux: Bool = Tmux.path != nil) -> Bool {
        config.hiresInBackground && hasTmux
    }

    /// What setup shows of the two. Whether T3 Code is installed is looked for only where it's
    /// needed: first setup, to know whether to ask, and with T3 Code on, for its hint. Off and
    /// asked, nothing is looked for.
    static func json(_ config: Config, hasTmux: Bool, installed: () -> Bool = t3InstalledRecently) -> [String: Any] {
        var answer: [String: Any] = [
            "t3Code": config.t3CodeEnabled, "background": config.hiresInBackground && hasTmux,
            "hasTmux": hasTmux, "asked": config.choicesAsked,
        ]
        if !config.choicesAsked || config.t3CodeEnabled { answer["t3Installed"] = installed() }
        return answer
    }
}

/// Switching to background: each hire's tmux viewers detached, and the
/// windows Smolcorp opened for them closed, by the id it recorded. Never the
/// session nor the server, never by a name, never a window Smolcorp didn't open.
enum HireWindows {
    /// What closing one window came to.
    enum Closed: Equatable {
        case closed
        /// Left open, and why, in a few words.
        case left(String)
    }

    /// What a recorded window's terminal said of it.
    enum Shown: Equatable {
        /// Open, showing this tty.
        case open(String)
        /// Not open any more: its record is forgotten.
        case gone
        /// The terminal couldn't be asked: the record is kept, and nothing is closed.
        case unknown
    }

    /// Closes windows by the id Smolcorp recorded as it opened them.
    protocol Closer {
        /// Whether the recorded window is open, and the tty it shows.
        func look(_ window: String) -> Shown
        /// Closes it, once the tmux client in it has gone, if that's safe.
        func close(_ window: String, tty: String) -> Closed
    }

    /// A hire to switch: its id, the name it's said by, and its tmux socket ("" for none).
    struct Hire: Equatable {
        var id: String
        var name: String
        var socket: String
    }

    /// Each hire's viewers detached, on its own socket and session; then the
    /// windows recorded for it closed, those that were among its viewers.
    /// `windows` is hire id to the windows Smolcorp opened for it; `grid`, the
    /// ttys of the team grid's panes, each a read-only viewer of one hire. Gives
    /// what was done, a line per hire that had something to say, and the
    /// windows that are closed or gone, to be forgotten.
    static func toBackground(_ hires: [Hire], windows: [String: [String]], tmux: (String) -> HireTmux?,
                             closer: Closer, grid: Set<String> = []) -> (said: [String], forget: [String: [String]]) {
        var said: [String] = [], forget: [String: [String]] = [:], gridEnded = false
        for hire in hires {
            let recorded = windows[hire.id] ?? []
            guard !hire.socket.isEmpty else {
                said.append("\(hire.name) runs in its window without tmux, so it stays there.")
                continue
            }
            guard let desk = tmux(URL(fileURLWithPath: hire.socket).lastPathComponent) else {
                said.append("\(hire.name)'s tmux couldn't be reached, so its window is left.")
                continue
            }
            guard let viewers = desk.clientTTYs() else {
                said.append("\(hire.name)'s tmux couldn't be asked who's watching it, so its window is left.")
                continue
            }
            // Which tty each recorded window shows, asked before the viewers go. Not open any more: forgotten.
            // Couldn't ask: kept, and left.
            var shown: [(window: String, tty: String)] = [], unasked: [String] = []
            for window in recorded {
                switch closer.look(window) {
                case .open(let tty): shown.append((window, tty))
                case .gone: forget[hire.id, default: []].append(window)
                case .unknown: unasked.append(window)
                }
            }
            guard !viewers.isEmpty else { continue }
            guard desk.detachClients() else {
                said.append("\(hire.name)'s window couldn't be detached, so it's left.")
                continue
            }
            if viewers.contains(where: grid.contains) { gridEnded = true }
            var lines: [String] = []
            for (window, tty) in shown {
                guard viewers.contains(tty) else {
                    lines.append("a window Smolcorp opened for it shows something else now, so it's left")
                    continue
                }
                switch closer.close(window, tty: tty) {
                case .closed: forget[hire.id, default: []].append(window)
                case .left(let why): lines.append("its window is left open: \(why)")
                }
            }
            for app in Set(unasked.map(terminalName)).sorted() {
                lines.append("Smolcorp couldn't ask \(app) about the window it opened for it, so it's left")
            }
            // A window Smolcorp couldn't ask about may be one of these, so it isn't said to be someone else's.
            let others = viewers.filter { tty in !grid.contains(tty) && !shown.contains { $0.tty == tty } }.count - unasked.count
            if others > 0 {
                lines.append(others == 1 ? "its window wasn't opened by Smolcorp while it ran, so it's left open"
                                         : "\(others) windows weren't opened by Smolcorp while it ran, so they're left open")
            }
            if !lines.isEmpty { said.append("\(hire.name): detached; " + lines.joined(separator: "; ") + ".") }
        }
        if gridEnded { said.append("The team grid's view of your hires ended.") }
        return (said, forget)
    }

    /// The terminal a recorded window is in, as it's said.
    static func terminalName(_ window: String) -> String {
        switch Terminals.parse(window)?.kind {
        case "iterm": return "iTerm2"
        case "ghostty-id": return "Ghostty"
        default: return "Terminal"
        }
    }

    /// The team grid's panes, each a read-only viewer of one hire: their ttys, on its own server. None when it's not there.
    static func gridTTYs(_ grid: HireTmux?) -> Set<String> { Set(grid?.paneTTYs() ?? []) }

    /// A Terminal window as it's recorded: its tab's tty and the window's id, `tty:/dev/ttys004#w1234`.
    /// Both must match for it to be closed, as macOS gives a closed tab's tty to a new one.
    static func terminalRecord(tty: String, windowID: String?) -> String {
        TabAddress.terminalApp(tty: tty).text + (windowID.map { "#w" + $0 } ?? "")
    }

    /// The tab of a window's record, as the deck knows it: a Terminal record without its window's id.
    static func tab(ofRecord record: String) -> String {
        record.hasPrefix("tty:") ? String(record.prefix { $0 != "#" }) : record
    }

    /// The Mac's terminals, closing a window by the id `Openers` gave for it:
    /// Terminal's window by its id and its tab's tty, iTerm2's session by its
    /// unique ID, Ghostty's terminal by its id. Only once nothing but its shell
    /// runs in it, so the terminal never asks, and nothing of yours is ended.
    struct Terminals: Closer {
        /// How long the tmux client has to end once it's detached.
        var settle: TimeInterval = 2

        func look(_ window: String) -> Shown {
            switch Self.parse(window) {
            case ("terminal", let tty, nil)?:
                // Recorded without its window's id: it can't be closed, so only whether it's open matters.
                guard let open = Terminal.openTTYs() else { return .unknown }
                return open.contains(tty) ? .open(tty) : .gone
            case ("terminal", let tty, let id?)?:
                switch Terminal.run(Self.terminalLook, arguments: [id, tty]) {
                case "open"?: return .open(tty)
                case "gone"?: return .gone
                default: return .unknown
                }
            case ("iterm", let id, _)?:
                let address = TabAddress(kind: "iterm", id: id)
                switch ITerm().isOpen(address) {
                case false?: return .gone
                case true?: return ITerm().tty(of: address).map { .open($0) } ?? .unknown
                case nil: return .unknown
                }
            case ("ghostty-id", let id, _)?:
                switch Terminal.run(Self.ghosttyLook, arguments: [id]) {
                case "gone"?: return .gone
                case let answer? where answer.hasPrefix("tty:") && TabAddress.isTTY(String(answer.dropFirst(4))):
                    return .open(String(answer.dropFirst(4)))
                default: return .unknown
                }
            default:
                return .unknown
            }
        }

        func close(_ window: String, tty: String) -> Closed {
            guard let (kind, id, windowID) = Self.parse(window) else { return .left("Smolcorp can't close that kind of window") }
            if kind == "terminal" && windowID == nil { return .left("Smolcorp didn't get its window's id from Terminal") }
            guard shellOnly(tty) else { return .left("something still runs in it") }
            let answer: String?
            switch kind {
            case "terminal": answer = Terminal.run(Self.terminalClose, arguments: [windowID ?? "", id])
            case "iterm": answer = Terminal.run(Self.itermClose, arguments: [id, tty])
            default: answer = Terminal.run(Self.ghosttyClose, arguments: [id, tty])
            }
            switch answer {
            case "ok", "gone": return .closed
            case "tabs": return .left("its window has other tabs")
            case "moved": return .left("it shows something else now")
            default: return .left("the terminal didn't close it")
            }
        }

        // The scripts, by id alone: a window's id and its tab's tty, a session's unique ID, a terminal's id.

        /// Whether Terminal's window `item 1` is open with a tab on tty `item 2`.
        static let terminalLook = """
            if application "Terminal" is not running then return "gone"
            tell application "Terminal"
                set wid to (item 1 of argv) as integer
                if not (exists window id wid) then return "gone"
                repeat with t in tabs of window id wid
                    if tty of t is (item 2 of argv) then return "open"
                end repeat
            end tell
            return "gone"
            """

        /// Terminal can close a window, not one tab of several: a window with other tabs is left.
        static let terminalClose = """
            if application "Terminal" is not running then return "gone"
            tell application "Terminal"
                set wid to (item 1 of argv) as integer
                if not (exists window id wid) then return "gone"
                set w to window id wid
                if (count of tabs of w) is not 1 then return "tabs"
                if tty of tab 1 of w is not (item 2 of argv) then return "moved"
                close w
                return "ok"
            end tell
            """

        static let itermClose = """
            if application "iTerm" is not running then return "gone"
            tell application "iTerm"
                repeat with w in windows
                    repeat with t in tabs of w
                        repeat with s in sessions of t
                            if unique ID of s is (item 1 of argv) then
                                if tty of s is not (item 2 of argv) then return "moved"
                                close s
                                return "ok"
                            end if
                        end repeat
                    end repeat
                end repeat
            end tell
            return "gone"
            """

        /// Ghostty 1.3 has no tty: asking errors, and that's "couldn't ask", never "not open".
        static let ghosttyLook = """
            if application "Ghostty" is not running then return "gone"
            tell application "Ghostty"
                if not (exists terminal id (item 1 of argv)) then return "gone"
                try
                    return "tty:" & (tty of terminal id (item 1 of argv))
                on error
                    return "?"
                end try
            end tell
            """

        static let ghosttyClose = """
            if application "Ghostty" is not running then return "gone"
            tell application "Ghostty"
                if not (exists terminal id (item 1 of argv)) then return "gone"
                set t to terminal id (item 1 of argv)
                try
                    if (tty of t) is not (item 2 of argv) then return "moved"
                on error
                    return "moved"
                end try
                close t
                return "ok"
            end tell
            """

        /// Nothing but the tab's shell runs on `tty`, or nothing at all, waited for a moment while the tmux client ends.
        private func shellOnly(_ tty: String) -> Bool {
            let deadline = Date().addingTimeInterval(settle)
            repeat {
                if case .shell? = Terminal.foreground(tty: tty), Terminal.hasOtherJobs(tty: tty) == false { return true }
                Thread.sleep(forTimeInterval: 0.1)
            } while Date() < deadline
            return false
        }

        /// A recorded window as its terminal, its id there (a Terminal tab's tty) and, for Terminal, its window's id.
        static func parse(_ window: String) -> (kind: String, id: String, window: String?)? {
            if window.hasPrefix("ghostty-id:") {
                let id = String(window.dropFirst("ghostty-id:".count))
                guard !id.isEmpty, id.count <= 100, id.allSatisfy({ $0.isASCII && ($0.isLetter || $0.isNumber || $0 == "-") }) else { return nil }
                return ("ghostty-id", id, nil)
            }
            var tab = window, windowID: String?
            if window.hasPrefix("tty:"), let mark = window.range(of: "#w") {
                let id = String(window[mark.upperBound...])
                guard !id.isEmpty, id.count <= 12, id.allSatisfy({ $0.isASCII && $0.isNumber }) else { return nil }
                tab = String(window[..<mark.lowerBound])
                windowID = id
            }
            switch TabAddress(tab).map({ ($0.kind, $0.id) }) {
            case ("tty", let tty)?: return ("terminal", tty, windowID)
            case ("iterm", let id)? where windowID == nil: return ("iterm", id, nil)
            default: return nil
            }
        }
    }
}

extension Deck {
    /// The hires still on the team, as switching to the background takes them.
    var hiresToSwitch: [HireWindows.Hire] {
        frontDesk.hires.filter { $0.state != .fired && $0.state != .failed }
            .map { HireWindows.Hire(id: $0.id, name: $0.nickname.isEmpty ? $0.name : $0.nickname, socket: $0.tmuxSocket) }
    }

    /// Agent windows switched to the background: each hire's viewers detached
    /// and the windows Smolcorp opened for them closed (see `HireWindows`), off
    /// the main thread. `done` gets what there is to say, on the main thread.
    func hiresToBackground(done: @escaping ([String]) -> Void) {
        let hires = hiresToSwitch, windows = hireWindows
        Terminal.queue.async {
            let grid = HireWindows.gridTTYs(HireTmux(socketName: TeamGrid.socketName()))
            let result = HireWindows.toBackground(hires, windows: windows, tmux: { HireTmux(socketName: $0) }, closer: HireWindows.Terminals(),
                                                  grid: grid)
            DispatchQueue.main.async {
                for (id, gone) in result.forget { self.forgetHireWindows(of: id) { gone.contains($0) } }
                done(result.said)
            }
        }
    }

    /// "Open a window" on a hire's desk: one window of the hires' terminal,
    /// attached to the hire's own tmux session, recorded as Smolcorp's. Nil
    /// when it opened, else why not.
    func openHireWindow(_ id: String, done: @escaping (String?) -> Void) {
        guard let hire = frontDesk.hires.first(where: { $0.id == id && $0.state != .fired && $0.state != .failed }) else {
            return done("That hire isn't on the team.")
        }
        guard !hire.tmuxSocket.isEmpty else { return done("This hire runs in a window of its own, not in tmux.") }
        let config = Config.shared
        let terminal = Openers.isInstalled(config.hireTerminal) ? config.hireTerminal : "terminal"
        let folder = hire.worktree.isEmpty ? hire.folder : hire.worktree
        Terminals.queue(opening: terminal).async {
            let window = Openers.reattach(socket: hire.tmuxSocket, in: folder, terminal: terminal)
            DispatchQueue.main.async {
                guard let window else { return done("Its window didn't open: the hire may have ended.") }
                self.hireWindows[id, default: []].append(window)
                done(nil)
            }
        }
    }
}

extension Deck {
    /// The records of windows Smolcorp opened that `isGone` says are gone, of hire `id` or of every hire.
    func forgetHireWindows(of id: String? = nil, _ isGone: (String) -> Bool) {
        for hire in id.map({ [$0] }) ?? Array(hireWindows.keys) {
            let kept = (hireWindows[hire] ?? []).filter { !isGone($0) }
            hireWindows[hire] = kept.isEmpty ? nil : kept
        }
    }

    /// A window `Tmux.focus` opened again on a hire's own server, recorded for the hire on that socket.
    func recordReopenedWindow(socket: String, window: String) {
        guard let id = Deck.hireID(onSocket: socket, among: frontDesk.hires) else { return }
        hireWindows[id, default: []].append(window)
    }

    /// The hire still on the team whose own tmux server is `socket`, by the socket's name; nil for one that's no hire's.
    static func hireID(onSocket socket: String, among hires: [FrontDesk.Hire], testPort: UInt16? = TestCopy.port) -> String? {
        let name = URL(fileURLWithPath: socket).lastPathComponent
        guard HireTmux.owns(socket, testPort: testPort) else { return nil }
        return hires.first {
            $0.state != .fired && $0.state != .failed && !$0.tmuxSocket.isEmpty && URL(fileURLWithPath: $0.tmuxSocket).lastPathComponent == name
        }?.id
    }

    /// The id of the hire working in session `id`, as the team pairs them; nil for a session that's no hire's.
    func teamHireID(session id: String) -> String? {
        guard !id.isEmpty else { return nil }
        let rows = frontDesk.teamJSON(live: frontDeskLive())["team"] as? [[String: Any]] ?? []
        return rows.first { $0["sessionId"] as? String == id && $0["registered"] as? Bool == true }?["id"] as? String
    }
}
