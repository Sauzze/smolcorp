import XCTest
@testable import AgentDeck

/// The team's screen in config.json, which the Go core keeps, and what the core's tiling asks of this Mac
/// (desktop/gmscreens.go works out the grid): the windows moved with their terminal's own AppleScript, on a
/// stand-in here, and never a key pressed or System Events asked.
final class TilerTests: XCTestCase {

    func testTheChosenScreenIsSavedInTheConfigAndOptional() throws {
        let config = Config()
        XCTAssertNil(config.teamScreen)
        config.teamScreen = "2"
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("tiler-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: file) }
        config.save(to: file)
        XCTAssertEqual(Config.load(from: file).teamScreen, "2")
    }

    /// Only the tiling files: the opt-in keystrokes (T505) have a module of their own and may use Accessibility there.
    func testTheTilingFilesNeverTalkToSystemEventsOrPressKeys() throws {
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        for name in ["Tiler.swift", "OfficeWindow.swift", "TerminalDrivers.swift"] {
            let source = try String(contentsOf: root.appendingPathComponent("Sources/AgentDeck/" + name), encoding: .utf8)
            XCTAssertFalse(source.contains("System " + "Events"), name)
            XCTAssertFalse(source.contains("AXUI" + "Element"), name)
            XCTAssertFalse(source.contains("AXIsProcess" + "Trusted"), name)
            XCTAssertFalse(source.contains("key " + "code"), name)
            XCTAssertFalse(source.contains("keystroke \""), name + " has no AppleScript keystroke")
        }
    }
}

extension TilerTests {
    func testTheScriptsMoveWithTheTerminalsOwnBoundsAndNeverSystemEvents() {
        let terminal = Tiler.moveScript(app: "Terminal"), iterm = Tiler.moveScript(app: "iTerm")
        XCTAssertTrue(terminal.contains("tell application \"Terminal\"") && terminal.contains("set bounds of window id"))
        XCTAssertTrue(iterm.contains("tell application \"iTerm\"") && iterm.contains("set bounds of window id"))
        XCTAssertTrue(Tiler.listScript(app: "iTerm").contains("tty of s"))
        XCTAssertTrue(Tiler.listScript(app: "Terminal").contains("tty of t"))
        XCTAssertEqual(Tiler.moveArguments(window: 12, to: CGRect(x: 480, y: 603, width: 480, height: 573)), ["12", "480", "603", "480", "573"])
        XCTAssertEqual(Tiler.osascriptArguments("return 1", ["a b"]), ["-e", "on run argv\nreturn 1\nend run", "a b"], "arguments are argv, not source")
        XCTAssertEqual(Tiler.scriptable("terminal"), "Terminal")
        XCTAssertNil(Tiler.scriptable("ghostty"), "Ghostty can't place its windows: the core shows the team grid")
    }

    func testParsingWhatTheScriptsSay() {
        let windows = Tiler.parseWindows("101 /dev/ttys003\n102 /dev/ttys004 /dev/ttys005\nbad")
        XCTAssertEqual(windows.map(\.id), [101, 102])
        XCTAssertEqual(windows[1].ttys, ["/dev/ttys004", "/dev/ttys005"])
        XCTAssertEqual(Tiler.parseRect("480 603 470 566"), CGRect(x: 480, y: 603, width: 470, height: 566))
        XCTAssertNil(Tiler.parseRect("x"))
        XCTAssertTrue(Tiler.isNotAllowed("execution error: Not authorized to send Apple events to Terminal. (-1743)"))
        XCTAssertFalse(Tiler.isNotAllowed("syntax error"))
    }

    func testAMoveHasLandedWithinACharacterOfItsSize() {
        let wanted = CGRect(x: 480, y: 603, width: 480, height: 573)
        XCTAssertTrue(Tiler.landed(CGRect(x: 480, y: 603, width: 470, height: 566), at: wanted))
        XCTAssertFalse(Tiler.landed(CGRect(x: 480, y: 30, width: 480, height: 573), at: wanted), "every row on the top one")
        XCTAssertEqual(Tiler.flipped(Tiler.flipped(wanted, mainHeight: 1440), mainHeight: 1440), wanted, "flipping twice is no change")
    }

    /// Each window found by the tty it shows and moved; one not found, one put elsewhere and one that failed are left
    /// out; a refused permission stops the work there; a terminal that can't be asked at all is nil.
    func testTilingMovesTheWindowShowingEachTTYAndStopsAtARefusal() {
        var ran: [[String]] = []
        let place = CGRect(x: 0, y: 30, width: 480, height: 573)
        let places = [("/dev/ttys003", place), ("/dev/ttys004", place), ("/dev/ttys005", place), ("/dev/ttys006", place), ("/dev/ttys007", place)]
        let script: Tiler.Script = { source, arguments in
            ran.append(arguments)
            guard let window = arguments.first else { return .ok("101 /dev/ttys003\n102 /dev/ttys008 /dev/ttys005\n103 /dev/ttys006\n104 /dev/ttys007") }
            return ["101": .ok("0 30 476 566"), "102": .ok("0 900 480 573"), "103": .failed, "104": .notAllowed][window] ?? .failed
        }
        XCTAssertEqual(Tiler.tile(app: "Terminal", places: places, script: script), Tiler.Moved(placed: ["/dev/ttys003"], needsPermission: true))
        XCTAssertEqual(ran.map(\.first), [nil, "101", "102", "103", "104"], "ttys004 has no window; nothing after the refusal")
        XCTAssertEqual(Tiler.tile(app: "iTerm", places: places, script: { _, _ in .notAllowed }), Tiler.Moved(needsPermission: true))
        XCTAssertNil(Tiler.tile(app: "iTerm", places: places, script: { _, _ in .failed }))
    }
}
