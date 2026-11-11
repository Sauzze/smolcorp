import AppKit

/// What tiling the team needs of this Mac, for the Go core (desktop/gmscreens.go), which works out the grid:
/// the screens, read afresh at each ask, and the hires' windows moved, each to the place the core gives for the
/// tty its window shows.
///
/// Windows are only ever moved with their terminal's own AppleScript (Terminal and iTerm2: "Automation"
/// permission for that terminal, not Accessibility). Nothing here presses a key or drives another app's
/// interface. The places are AppleScript's coordinates: from the main screen's top-left corner, y going down.
enum Tiler {
    // MARK: This Mac

    /// The screens as the bridge's `screens` answers them: what the display is called to the system (stable while
    /// it stays plugged in), its name, its frame and the part of it free of the menu bar and the Dock, as NSScreen
    /// gives them (y up from the main screen's bottom), and whether it's the main one. On the main queue.
    static func screens() -> [[String: Any]] {
        let main = NSScreen.screens.first
        return NSScreen.screens.enumerated().map { index, screen in
            let number = (screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber)?.stringValue ?? "screen-\(index)"
            return ["id": number, "name": screen.localizedName, "frame": json(screen.frame), "visible": json(screen.visibleFrame), "main": screen == main]
        }
    }

    static func json(_ rect: CGRect) -> [String: Any] {
        ["x": rect.minX, "y": rect.minY, "width": rect.width, "height": rect.height]
    }

    /// `rect` from NSScreen's coordinates into AppleScript's (and back: it is its own inverse).
    static func flipped(_ rect: CGRect, mainHeight: CGFloat) -> CGRect {
        CGRect(x: rect.minX, y: mainHeight - rect.maxY, width: rect.width, height: rect.height)
    }

    /// Whether a window asked to go to `wanted` went there. A terminal rounds a window's size to whole lines and
    /// columns, so the size may be off by about a character; where it starts may not.
    static func landed(_ actual: CGRect, at wanted: CGRect) -> Bool {
        abs(actual.minX - wanted.minX) <= 2 && abs(actual.minY - wanted.minY) <= 2
            && abs(actual.width - wanted.width) <= 24 && abs(actual.height - wanted.height) <= 24
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

    /// osascript's arguments for `source`, its `arguments` passed as argv and never spliced into the source.
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

    /// The script moving `app`'s window (argv 1) to the left, top, width and height of argv 2 to 5 with its own
    /// `bounds`, and saying where it is after.
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

    /// What moving the windows came to: the ttys whose windows went where they were asked, and whether `app`
    /// refused Smolcorp its Automation permission (the work stops there, and isn't asked again); nil when `app`
    /// couldn't be asked for its windows at all.
    struct Moved: Equatable {
        var placed: [String] = []
        var needsPermission = false
    }

    /// Moves `app`'s window showing each place's tty to its rectangle, with `script` running the AppleScript.
    static func tile(app: String, places: [(tty: String, rect: CGRect)], script: Script) -> Moved? {
        let listed = script(listScript(app: app), [])
        guard case .ok(let output) = listed else { return listed == .notAllowed ? Moved(needsPermission: true) : nil }
        let windows = parseWindows(output)
        var moved = Moved()
        for place in places {
            guard let window = windows.first(where: { $0.ttys.contains(place.tty) }) else { continue }
            switch script(moveScript(app: app), moveArguments(window: window.id, to: place.rect)) {
            case .ok(let answer):
                if let actual = parseRect(answer), landed(actual, at: place.rect) { moved.placed.append(place.tty) }
            case .notAllowed:
                moved.needsPermission = true
                return moved
            case .failed:
                continue
            }
        }
        return moved
    }
}
