import AppKit

/// Puts the hires' windows where their desks are: the ground floor's 3×3
/// desks (the office's keys) become a 3×3 grid on one screen, each hire's
/// viewer window over its desk's cell, with the office's window under them.
///
/// Windows are only ever moved with their terminal's own AppleScript
/// (Terminal and iTerm2: "Automation" permission for that terminal, not
/// Accessibility). A terminal without window scripting gets one window
/// showing a tmux "team" grid instead (`TeamGrid`). Nothing here presses a
/// key or drives another app's interface.
///
/// Coordinates are AppleScript's: from the main screen's top-left corner, y
/// going down. NSScreen's go up from its bottom-left, so they're turned over.
/// The screens are read on every call and nothing is kept.
enum Tiler {
    struct Screen: Equatable {
        /// What the display is called to the system (stable while it stays plugged in); empty if unknown.
        var id: String = ""
        var name: String
        /// As NSScreen gives it, y up from the main screen's bottom.
        var frame: CGRect
        /// The part of it free of the menu bar and the Dock, in the same coordinates.
        var visible: CGRect
        var isMain: Bool
        var isPortrait: Bool { frame.height > frame.width }
    }

    struct Grid: Equatable {
        var screen: Screen
        /// The nine cells, by desk key, top-left first.
        var cells: [CGRect]
        var office: CGRect
    }

    /// A hire with a viewer to place: its id, its desk on the ground floor and its tmux socket.
    struct Seat: Equatable {
        var hireID: String
        var desk: Int
        var socket: String
    }

    // MARK: Geometry

    /// `rect` from NSScreen's coordinates into AppleScript's (and back: it is its own inverse).
    static func flipped(_ rect: CGRect, mainHeight: CGFloat) -> CGRect {
        CGRect(x: rect.minX, y: mainHeight - rect.maxY, width: rect.width, height: rect.height)
    }

    static func mainHeight(of screens: [Screen]) -> CGFloat {
        (screens.first(where: \.isMain) ?? screens.first)?.frame.height ?? 0
    }

    /// The screen that holds the team: the one chosen (`Config.teamScreen`)
    /// if it is still plugged in, else the left-most portrait one, else the main one.
    static func teamScreen(among screens: [Screen], chosen: String? = nil) -> Screen? {
        if let chosen, !chosen.isEmpty, let match = screens.first(where: { $0.id == chosen }) { return match }
        return screens.filter(\.isPortrait).min { $0.frame.minX < $1.frame.minX } ?? screens.first(where: \.isMain) ?? screens.first
    }

    /// The grid on `screen`'s visible part: the office the full width at 16:9,
    /// flush with the bottom; the three rows share what's above it; the
    /// columns a third of the width each. Desk key → cell: column = key % 3,
    /// row = key / 3.
    static func grid(on screen: Screen, mainHeight: CGFloat) -> Grid {
        let area = flipped(screen.visible, mainHeight: mainHeight)
        let officeHeight = (area.width * 9 / 16).rounded()
        let width = (area.width / 3).rounded(.down)
        let height = ((area.height - officeHeight) / 3).rounded(.down)
        let cells = (0..<9).map { key in
            CGRect(x: area.minX + CGFloat(key % 3) * width, y: area.minY + CGFloat(key / 3) * height, width: width, height: height)
        }
        let office = CGRect(x: area.minX, y: area.maxY - officeHeight, width: area.width, height: officeHeight)
        return Grid(screen: screen, cells: cells, office: office)
    }

    /// Whether a window asked to go to `wanted` went there. A terminal rounds
    /// a window's size to whole lines and columns, so the size may be off by
    /// about a character; where it starts may not.
    static func landed(_ actual: CGRect, at wanted: CGRect) -> Bool {
        abs(actual.minX - wanted.minX) <= 2 && abs(actual.minY - wanted.minY) <= 2
            && abs(actual.width - wanted.width) <= 24 && abs(actual.height - wanted.height) <= 24
    }

    /// The ground floor's hires among the team rows `GET /setup/gm/team`
    /// answers: those with a tmux session of their own and a desk, one to a
    /// desk (the first row keeps it), by desk.
    static func seats(fromTeam rows: [[String: Any]]) -> [Seat] {
        var taken = Set<Int>()
        var seats: [Seat] = []
        for row in rows {
            guard row["registered"] as? Bool == true, let id = row["id"] as? String, let socket = row["tmuxSocket"] as? String, !socket.isEmpty,
                  row["floor"] as? Int == 0, let desk = row["desk"] as? Int, (0..<9).contains(desk), !taken.contains(desk),
                  !["fired", "failed"].contains(row["state"] as? String ?? "") else { continue }
            taken.insert(desk)
            seats.append(Seat(hireID: id, desk: desk, socket: socket))
        }
        return seats.sorted { $0.desk < $1.desk }
    }

    // MARK: This Mac

    static func screens() -> [Screen] {
        let main = NSScreen.screens.first
        return NSScreen.screens.enumerated().map { index, screen in
            let number = (screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber)?.stringValue ?? "screen-\(index)"
            return Screen(id: number, name: screen.localizedName, frame: screen.frame, visible: screen.visibleFrame, isMain: screen == main)
        }
    }

    /// `GET /setup/gm/screens`: the screens, which one holds the team now, and
    /// whether to ask (two or more). `chosen` is `Config.teamScreen`.
    static func screensJSON(_ screens: [Screen], chosen: String?) -> [String: Any] {
        let current = teamScreen(among: screens, chosen: chosen)
        let rows: [[String: Any]] = screens.map { screen in
            ["id": screen.id, "name": screen.name, "main": screen.isMain, "portrait": screen.isPortrait,
             "team": screen == current,
             "frame": ["x": screen.frame.minX, "y": screen.frame.minY, "width": screen.frame.width, "height": screen.frame.height]]
        }
        return ["screens": rows, "chosen": current?.id ?? "", "ask": screens.count > 1]
    }

    // MARK: AppleScript

    /// What an AppleScript came to.
    enum ScriptResult: Equatable {
        case ok(String)
        /// The terminal wouldn't take Apple events from Smolcorp ("Automation" is off for it).
        case notAllowed
        case failed
    }

    typealias Script = (_ source: String, _ arguments: [String]) -> ScriptResult

    /// osascript's arguments for `source`, its `arguments` passed as argv and
    /// never spliced into the source.
    static func osascriptArguments(_ source: String, _ arguments: [String]) -> [String] {
        ["-e", "on run argv\n\(source)\nend run"] + arguments
    }

    /// Whether osascript's complaint is the one for a refused Automation permission (error -1743).
    static func isNotAllowed(_ errorText: String) -> Bool {
        errorText.contains("-1743") || errorText.localizedCaseInsensitiveContains("not authorized")
    }

    /// The real thing: `/usr/bin/osascript`, stopped if it hangs (a permission question left unanswered).
    static let osascript: Script = { source, arguments in
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/osascript")
        process.arguments = osascriptArguments(source, arguments)
        let out = Pipe(), err = Pipe()
        process.standardOutput = out
        process.standardError = err
        do { try process.run() } catch { return .failed }
        let stop = DispatchWorkItem { if process.isRunning { process.terminate() } }
        DispatchQueue.global().asyncAfter(deadline: .now() + 60, execute: stop)
        var complaint = ""
        let reading = DispatchGroup()
        DispatchQueue.global().async(group: reading) {
            complaint = String(decoding: err.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
        }
        let data = out.fileHandleForReading.readDataToEndOfFile()
        reading.wait()
        process.waitUntilExit()
        stop.cancel()
        if process.terminationStatus == 0 { return .ok(String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)) }
        return isNotAllowed(complaint) ? .notAllowed : .failed
    }

    /// The scripting name of a terminal that can place its windows, or nil.
    static func scriptable(_ terminal: String) -> String? {
        switch terminal {
        case "terminal": return "Terminal"
        case "iterm": return "iTerm"
        default: return nil
        }
    }

    /// The script listing `app`'s windows: one line each, the window's id and then the tty of each of its tabs or sessions.
    static func listScript(app: String) -> String {
        let inner = app == "iTerm"
            ? "repeat with t in tabs of w\nrepeat with s in sessions of t\nset found to found & \" \" & (tty of s)\nend repeat\nend repeat"
            : "repeat with t in tabs of w\nset found to found & \" \" & (tty of t)\nend repeat"
        return """
            if application "\(app)" is not running then return ""
            set out to {}
            tell application "\(app)"
            repeat with w in windows
            set found to (id of w as text)
            \(inner)
            set end of out to found
            end repeat
            end tell
            set AppleScript's text item delimiters to linefeed
            return out as text
            """
    }

    /// The script moving `app`'s window (argv 1) to the left, top, width and
    /// height of argv 2 to 5 with its own `bounds`, and saying where it is after.
    static func moveScript(app: String) -> String {
        """
        set {x, y, w, h} to {(item 2 of argv) as integer, (item 3 of argv) as integer, (item 4 of argv) as integer, (item 5 of argv) as integer}
        tell application "\(app)"
        set bounds of window id ((item 1 of argv) as integer) to {x, y, x + w, y + h}
        set b to bounds of window id ((item 1 of argv) as integer)
        end tell
        return ((item 1 of b) as text) & " " & (item 2 of b) & " " & ((item 3 of b) - (item 1 of b)) & " " & ((item 4 of b) - (item 2 of b))
        """
    }

    static func moveArguments(window id: Int, to rect: CGRect) -> [String] {
        [String(id)] + [rect.minX, rect.minY, rect.width, rect.height].map { String(Int($0)) }
    }

    /// The windows `listScript` answered: id and ttys.
    static func parseWindows(_ output: String) -> [(id: Int, ttys: [String])] {
        output.split(separator: "\n").compactMap { line in
            let words = line.split(separator: " ").map(String.init)
            guard let id = words.first.flatMap(Int.init) else { return nil }
            return (id, Array(words.dropFirst()))
        }
    }

    static func parseRect(_ output: String) -> CGRect? {
        let parts = output.split(separator: " ").compactMap { Double($0) }
        return parts.count == 4 ? CGRect(x: parts[0], y: parts[1], width: parts[2], height: parts[3]) : nil
    }

    // MARK: Tiling

    /// How the windows were placed.
    enum Method: String {
        /// Each hire's own window, moved with its terminal's scripting.
        case windows
        /// One window showing a tmux grid of all the hires.
        case teamGrid = "team-grid"
        case none
    }

    struct Report: Equatable {
        var method = Method.none
        var placed: [String] = []
        var missed: [String] = []
        var needsPermission = false
        var message = ""
        /// Where the office goes (AppleScript's coordinates), when the screen was read.
        var office: CGRect?

        var json: [String: Any] {
            var out: [String: Any] = ["method": method.rawValue, "placed": placed, "missed": missed,
                                      "needsPermission": needsPermission, "message": message]
            if let office { out["office"] = [Int(office.minX), Int(office.minY), Int(office.width), Int(office.height)] }
            return out
        }
    }

    /// Tiles `seats` on `grid` as `terminal` allows. `tmux` gives a hire's
    /// server for a socket; `script` runs AppleScript; `teamGrid` shows the
    /// grid for the terminals that can't place windows. A refused
    /// permission ends the work at once and is not asked again.
    static func tile(seats: [Seat], terminal: String, grid: Grid, tmux: (String) -> HireTmux?, script: Script,
                     teamGrid: ([HireTmux]) -> Bool) -> Report {
        var report = Report(office: grid.office)
        guard !seats.isEmpty else {
            report.message = "No hire is on the ground floor in its own tmux session, so there is nothing to tile."
            return report
        }
        guard let app = scriptable(terminal) else {
            let desks = seats.compactMap { tmux($0.socket) }
            guard !desks.isEmpty, teamGrid(desks) else {
                report.message = "The team grid couldn't be made: tmux wasn't found or wouldn't start it."
                return report
            }
            report.method = .teamGrid
            report.placed = seats.map(\.hireID)
            report.message = "This terminal can't place its windows, so the team is in one window, a pane to a hire, read-only."
            return report
        }
        report.method = .windows
        let listed = script(listScript(app: app), [])
        guard case .ok(let output) = listed else {
            report.needsPermission = listed == .notAllowed
            report.missed = seats.map(\.hireID)
            report.message = listed == .notAllowed ? notAllowedMessage(app) : "\(app) couldn't be asked for its windows."
            return report
        }
        let windows = parseWindows(output)
        for seat in seats {
            guard let desk = tmux(seat.socket), let tty = desk.clientTTYs()?.first,
                  let window = windows.first(where: { $0.ttys.contains(tty) }) else { report.missed.append(seat.hireID); continue }
            let wanted = grid.cells[seat.desk]
            switch script(moveScript(app: app), moveArguments(window: window.id, to: wanted)) {
            case .ok(let answer):
                if let actual = parseRect(answer), landed(actual, at: wanted) { report.placed.append(seat.hireID) } else { report.missed.append(seat.hireID) }
            case .notAllowed:
                report.needsPermission = true
                report.missed = seats.map(\.hireID).filter { !report.placed.contains($0) }
                report.message = notAllowedMessage(app)
                return report
            case .failed:
                report.missed.append(seat.hireID)
            }
        }
        if report.message.isEmpty {
            report.message = report.missed.isEmpty ? "The team is tiled."
                : "\(report.placed.count) placed; \(report.missed.count) not (no open window found for it, or \(app) put it elsewhere: it can do that on a screen other than the main one)."
        }
        return report
    }

    static func notAllowedMessage(_ app: String) -> String {
        "\(app) didn't let Smolcorp move its windows. Allow Smolcorp under \(app) in System Settings → Privacy & Security → Automation, then tile again."
    }
}

/// One window showing every hire, for terminals that can't place windows: a
/// tmux session on our own `-L smolcorp-team` server, a pane to a hire. Each
/// pane runs `tmux -L smolcorp-<hire> attach -r`, a read-only attach, so
/// typing in the grid can't reach a hire.
enum TeamGrid {
    static let session = "team"

    /// The server's socket name (a test copy's: `smolcorp-<port>-team`).
    static func socketName(testPort: UInt16? = TestCopy.port) -> String {
        HireTmux.prefix + (testPort.map { "\($0)-" } ?? "") + "team"
    }

    /// What a pane runs: a read-only attach to one hire's session.
    static func paneCommand(tmuxPath: String, hireSocket: String) -> String {
        "exec \(Openers.quoted(tmuxPath)) -L \(Openers.quoted(hireSocket)) attach -r -t \(Openers.quoted("=" + HireTmux.session))"
    }

    /// tmux's arguments, one list per call, making the session with one pane
    /// for each of `panes` (their commands), tiled.
    static func commands(socket: String, panes: [String]) -> [[String]] {
        guard let first = panes.first else { return [] }
        let target = "=" + session + ":"
        var calls: [[String]] = [["new-session", "-d", "-s", session, "-x", "240", "-y", "60", first],
                                 ["set-option", "-t", target, "status", "off"]]
        for command in panes.dropFirst() {
            calls.append(["split-window", "-t", target, command])
            calls.append(["select-layout", "-t", target, "tiled"])
        }
        calls.append(["select-layout", "-t", target, "tiled"])
        return calls.map { ["-L", socket] + $0 }
    }

    /// The line a terminal window runs to show the grid.
    static func attachLine(tmuxPath: String, socket: String) -> String {
        "exec \(Openers.quoted(tmuxPath)) -L \(Openers.quoted(socket)) attach-session -r -t \(Openers.quoted("=" + session))"
    }

    /// Makes the grid afresh on our server (an old one is ended first: only
    /// this socket's) and shows it in a window of `terminal` unless one is
    /// showing it. False if any step failed.
    static func show(hires: [HireTmux], terminal: String, tmuxPath: String, run: ([String]) -> (status: Int32, output: String)?,
                     open: (String) -> Bool) -> Bool {
        let socket = socketName()
        _ = run(["-L", socket, "kill-server"])
        let panes = hires.map { paneCommand(tmuxPath: tmuxPath, hireSocket: $0.socketName) }
        for call in commands(socket: socket, panes: panes) where run(call)?.status != 0 { return false }
        return open(attachLine(tmuxPath: tmuxPath, socket: socket))
    }
}
