import AppKit

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
