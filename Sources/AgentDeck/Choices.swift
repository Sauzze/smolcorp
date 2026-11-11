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

    /// Closes windows by the id Smolcorp recorded as it opened them.
    protocol Closer {
        /// The tty the recorded window shows, or nil when it's not open any more.
        func tty(of window: String) -> String?
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
    /// `windows` is hire id to the windows Smolcorp opened for it. Gives what
    /// was done, a line per hire that had something to say, and the windows
    /// that are closed or gone, to be forgotten.
    static func toBackground(_ hires: [Hire], windows: [String: [String]], tmux: (String) -> HireTmux?,
                             closer: Closer) -> (said: [String], forget: [String: [String]]) {
        var said: [String] = [], forget: [String: [String]] = [:]
        for hire in hires {
            let recorded = windows[hire.id] ?? []
            guard !hire.socket.isEmpty else {
                if !recorded.isEmpty { said.append("\(hire.name) runs in its window without tmux, so it stays there.") }
                continue
            }
            guard let desk = tmux(URL(fileURLWithPath: hire.socket).lastPathComponent) else {
                said.append("\(hire.name)'s tmux couldn't be reached, so its window is left.")
                continue
            }
            let viewers = desk.clientTTYs()
            // Which tty each recorded window shows, asked before the viewers go. Not open any more: forgotten.
            var shown: [(window: String, tty: String)] = []
            for window in recorded {
                if let tty = closer.tty(of: window) { shown.append((window, tty)) } else { forget[hire.id, default: []].append(window) }
            }
            guard !viewers.isEmpty else { continue }
            guard desk.detachClients() else {
                said.append("\(hire.name)'s window couldn't be detached, so it's left.")
                continue
            }
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
            let unknown = viewers.filter { tty in !shown.contains { $0.tty == tty } }.count
            if unknown > 0 {
                lines.append(unknown == 1 ? "its window wasn't opened by Smolcorp while it ran, so it's left open"
                                          : "\(unknown) windows weren't opened by Smolcorp while it ran, so they're left open")
            }
            if !lines.isEmpty { said.append("\(hire.name): detached; " + lines.joined(separator: "; ") + ".") }
        }
        return (said, forget)
    }

    /// The Mac's terminals, closing a window by the id `Openers` gave for it:
    /// Terminal's tab by its tty, iTerm2's session by its unique ID, Ghostty's
    /// terminal by its id. Only once nothing but its shell runs in it, so the
    /// terminal never asks, and nothing of yours is ended.
    struct Terminals: Closer {
        /// How long the tmux client has to end once it's detached.
        var settle: TimeInterval = 2

        func tty(of window: String) -> String? {
            switch Self.parse(window) {
            case ("terminal", let tty)?:
                return Terminal.openTTYs()?.contains(tty) == true ? tty : nil
            case ("iterm", let id)?:
                return ITerm().tty(of: TabAddress(kind: "iterm", id: id))
            case ("ghostty-id", let id)?:
                let tty = Terminal.run("""
                    if application "Ghostty" is not running then return ""
                    tell application "Ghostty"
                        if not (exists terminal id (item 1 of argv)) then return ""
                        return tty of terminal id (item 1 of argv)
                    end tell
                    """, arguments: [id]) ?? ""
                return TabAddress.isTTY(tty) ? tty : nil
            default:
                return nil
            }
        }

        func close(_ window: String, tty: String) -> Closed {
            guard let (kind, id) = Self.parse(window) else { return .left("Smolcorp can't close that kind of window") }
            guard shellOnly(tty) else { return .left("something still runs in it") }
            let answer: String?
            switch kind {
            case "terminal":
                // Terminal can close a window, not one tab of several: a window with other tabs is left.
                answer = Terminal.run("""
                    if application "Terminal" is not running then return "gone"
                    tell application "Terminal"
                        repeat with w in windows
                            repeat with t in tabs of w
                                if tty of t is (item 1 of argv) then
                                    if (count of tabs of w) is not 1 then return "tabs"
                                    close w
                                    return "ok"
                                end if
                            end repeat
                        end repeat
                    end tell
                    return "gone"
                    """, arguments: [id])
            case "iterm":
                answer = Terminal.run("""
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
                    """, arguments: [id, tty])
            default:
                answer = Terminal.run("""
                    if application "Ghostty" is not running then return "gone"
                    tell application "Ghostty"
                        if not (exists terminal id (item 1 of argv)) then return "gone"
                        set t to terminal id (item 1 of argv)
                        if (tty of t) is not (item 2 of argv) then return "moved"
                        close t
                        return "ok"
                    end tell
                    """, arguments: [id, tty])
            }
            switch answer {
            case "ok", "gone": return .closed
            case "tabs": return .left("its window has other tabs")
            default: return .left("the terminal didn't close it")
            }
        }

        /// Nothing but the tab's shell runs on `tty`, or nothing at all, waited for a moment while the tmux client ends.
        private func shellOnly(_ tty: String) -> Bool {
            let deadline = Date().addingTimeInterval(settle)
            repeat {
                if case .shell? = Terminal.foreground(tty: tty), Terminal.hasOtherJobs(tty: tty) == false { return true }
                Thread.sleep(forTimeInterval: 0.1)
            } while Date() < deadline
            return false
        }

        /// A recorded window as its terminal and its id there.
        static func parse(_ window: String) -> (kind: String, id: String)? {
            if window.hasPrefix("ghostty-id:") {
                let id = String(window.dropFirst("ghostty-id:".count))
                guard !id.isEmpty, id.count <= 100, id.allSatisfy({ $0.isASCII && ($0.isLetter || $0.isNumber || $0 == "-") }) else { return nil }
                return ("ghostty-id", id)
            }
            switch TabAddress(window).map({ ($0.kind, $0.id) }) {
            case ("tty", let tty)?: return ("terminal", tty)
            case ("iterm", let id)?: return ("iterm", id)
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
            let result = HireWindows.toBackground(hires, windows: windows, tmux: { HireTmux(socketName: $0) }, closer: HireWindows.Terminals())
            DispatchQueue.main.async {
                for (id, gone) in result.forget {
                    self.hireWindows[id] = self.hireWindows[id]?.filter { !gone.contains($0) }
                    if self.hireWindows[id]?.isEmpty == true { self.hireWindows[id] = nil }
                }
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
        Terminal.queue.async {
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
    /// The id of the hire working in session `id`, as the team pairs them; nil for a session that's no hire's.
    func teamHireID(session id: String) -> String? {
        guard !id.isEmpty else { return nil }
        let rows = frontDesk.teamJSON(live: frontDeskLive())["team"] as? [[String: Any]] ?? []
        return rows.first { $0["sessionId"] as? String == id && $0["registered"] as? Bool == true }?["id"] as? String
    }
}
