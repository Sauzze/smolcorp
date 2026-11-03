import XCTest
@testable import AgentDeck

final class TilerTests: XCTestCase {
    private let portrait = Tiler.Screen(name: "PA278CV", frame: CGRect(x: 0, y: 0, width: 1440, height: 2560),
                                        visible: CGRect(x: 0, y: 0, width: 1440, height: 2530), isMain: true)

    func testTheChosenScreenHoldsTheTeamElseTheDefault() {
        let main = Tiler.Screen(id: "1", name: "Built-in", frame: CGRect(x: 0, y: 0, width: 1512, height: 982), visible: CGRect(x: 0, y: 0, width: 1512, height: 950), isMain: true)
        let tall = Tiler.Screen(id: "2", name: "PA278CV", frame: CGRect(x: -1440, y: 0, width: 1440, height: 2560), visible: CGRect(x: -1440, y: 0, width: 1440, height: 2530), isMain: false)
        XCTAssertEqual(Tiler.teamScreen(among: [main, tall], chosen: "1"), main, "the choice beats the portrait default")
        XCTAssertEqual(Tiler.teamScreen(among: [main, tall], chosen: nil), tall)
        XCTAssertEqual(Tiler.teamScreen(among: [main, tall], chosen: "9"), tall, "a screen that was unplugged: back to the default")
    }

    func testTheScreensAnswerAsksOnlyWithTwoOrMore() {
        let main = Tiler.Screen(id: "1", name: "Built-in", frame: CGRect(x: 0, y: 0, width: 1512, height: 982), visible: CGRect(x: 0, y: 0, width: 1512, height: 950), isMain: true)
        let tall = Tiler.Screen(id: "2", name: "PA278CV", frame: CGRect(x: -1440, y: 0, width: 1440, height: 2560), visible: CGRect(x: -1440, y: 0, width: 1440, height: 2530), isMain: false)
        let one = Tiler.screensJSON([main], chosen: nil)
        XCTAssertEqual(one["ask"] as? Bool, false)
        XCTAssertEqual(one["chosen"] as? String, "1")
        let two = Tiler.screensJSON([main, tall], chosen: "1")
        XCTAssertEqual(two["ask"] as? Bool, true)
        XCTAssertEqual(two["chosen"] as? String, "1")
        let rows = two["screens"] as? [[String: Any]] ?? []
        XCTAssertEqual(rows.map { $0["name"] as? String }, ["Built-in", "PA278CV"])
        XCTAssertEqual(rows.map { $0["team"] as? Bool }, [true, false])
        XCTAssertEqual(rows[1]["portrait"] as? Bool, true)
        XCTAssertEqual((rows[1]["frame"] as? [String: Any])?["width"] as? CGFloat, 1440)
    }

    func testTheChosenScreenIsSavedInTheConfigAndOptional() throws {
        let config = Config()
        XCTAssertNil(config.teamScreen)
        config.teamScreen = "2"
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("tiler-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: file) }
        config.save(to: file)
        XCTAssertEqual(Config.load(from: file).teamScreen, "2")
    }

    func testTheGroundFloorIsThreeByThreeOverTheOffice() {
        let grid = Tiler.grid(on: portrait, mainHeight: 2560)
        XCTAssertEqual(grid.cells.map(\.minX), [0, 480, 960, 0, 480, 960, 0, 480, 960])
        XCTAssertEqual(grid.cells.map(\.minY), [30, 30, 30, 603, 603, 603, 1176, 1176, 1176])
        XCTAssertTrue(grid.cells.allSatisfy { $0.size == CGSize(width: 480, height: 573) })
        XCTAssertEqual(grid.office, CGRect(x: 0, y: 1750, width: 1440, height: 810), "flush with the bottom, 16:9")
        XCTAssertLessThanOrEqual(grid.cells[8].maxY, grid.office.minY)
    }

    func testTheGridStaysInTheVisibleFrameWhereTheDockTakesTheBottom() {
        var docked = portrait
        docked.visible = CGRect(x: 0, y: 100, width: 1440, height: 2430)
        let grid = Tiler.grid(on: docked, mainHeight: 2560)
        XCTAssertEqual(grid.office.maxY, 2460, "the office stops above the Dock")
        XCTAssertEqual(grid.cells[0].minY, 30)
    }

    func testDeskKeyToCellIsColumnKeyMod3RowKeyDiv3() {
        let grid = Tiler.grid(on: portrait, mainHeight: 2560)
        XCTAssertEqual(grid.cells[4].origin, CGPoint(x: 480, y: 603))
        XCTAssertEqual(grid.cells[6].origin, CGPoint(x: 0, y: 1176))
    }

    func testAScreenAboveTheMainOneGivesNegativeTops() {
        let main = Tiler.Screen(name: "Built-in", frame: CGRect(x: 0, y: 0, width: 2560, height: 1440),
                                visible: CGRect(x: 0, y: 0, width: 2560, height: 1410), isMain: true)
        let tall = Tiler.Screen(name: "PA278CV", frame: CGRect(x: -2952, y: 0, width: 1440, height: 2560),
                                visible: CGRect(x: -2952, y: 0, width: 1440, height: 2530), isMain: false)
        XCTAssertEqual(Tiler.teamScreen(among: [main, tall]), tall, "the left portrait screen")
        let grid = Tiler.grid(on: tall, mainHeight: Tiler.mainHeight(of: [main, tall]))
        XCTAssertEqual(grid.cells[0].origin, CGPoint(x: -2952, y: -1090))
        XCTAssertEqual(Tiler.flipped(Tiler.flipped(grid.office, mainHeight: 1440), mainHeight: 1440), grid.office, "flipping twice is no change")
    }

    func testNoPortraitScreenFallsBackToTheMainOne() {
        let main = Tiler.Screen(name: "A", frame: CGRect(x: 0, y: 0, width: 1920, height: 1080), visible: CGRect(x: 0, y: 0, width: 1920, height: 1050), isMain: true)
        let other = Tiler.Screen(name: "B", frame: CGRect(x: -1920, y: 0, width: 1920, height: 1080), visible: CGRect(x: -1920, y: 0, width: 1920, height: 1080), isMain: false)
        XCTAssertEqual(Tiler.teamScreen(among: [other, main]), main)
        XCTAssertNil(Tiler.teamScreen(among: []))
    }

    func testAMoveHasLandedWithinACharacterOfItsSize() {
        let wanted = CGRect(x: 480, y: 603, width: 480, height: 573)
        XCTAssertTrue(Tiler.landed(CGRect(x: 480, y: 603, width: 470, height: 566), at: wanted))
        XCTAssertFalse(Tiler.landed(CGRect(x: 480, y: 30, width: 480, height: 573), at: wanted), "every row on the top one")
    }

    private func row(_ id: String, desk: Int?, floor: Int? = 0, socket: String = "/tmp/tmux-501/smolcorp-x", state: String = "working", registered: Bool = true) -> [String: Any] {
        ["id": id, "registered": registered, "tmuxSocket": socket, "floor": floor.map { $0 as Any } ?? NSNull(),
         "desk": desk.map { $0 as Any } ?? NSNull(), "state": state]
    }

    func testSeatsAreTheGroundFloorHiresOnTheirOwnTmux() {
        let rows = [row("b", desk: 4), row("a", desk: 0), row("up", desk: 1, floor: 2), row("none", desk: nil), row("plain", desk: 2, socket: ""),
                    row("gone", desk: 3, state: "fired"), row("hand", desk: 5, registered: false), row("twin", desk: 4), row("big", desk: 9)]
        XCTAssertEqual(Tiler.seats(fromTeam: rows).map(\.hireID), ["a", "b"], "by desk, one to a desk")
    }

    // MARK: The scripts

    func testTheScriptsMoveWithTheTerminalsOwnBoundsAndNeverSystemEvents() {
        let terminal = Tiler.moveScript(app: "Terminal"), iterm = Tiler.moveScript(app: "iTerm")
        XCTAssertTrue(terminal.contains("tell application \"Terminal\"") && terminal.contains("set bounds of window id"))
        XCTAssertTrue(iterm.contains("tell application \"iTerm\"") && iterm.contains("set bounds of window id"))
        XCTAssertTrue(Tiler.listScript(app: "iTerm").contains("tty of s"))
        XCTAssertTrue(Tiler.listScript(app: "Terminal").contains("tty of t"))
        XCTAssertEqual(Tiler.moveArguments(window: 12, to: CGRect(x: 480, y: 603, width: 480, height: 573)), ["12", "480", "603", "480", "573"])
        XCTAssertEqual(Tiler.osascriptArguments("return 1", ["a b"]), ["-e", "on run argv\nreturn 1\nend run", "a b"], "arguments are argv, not source")
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
        let tiler = try String(contentsOf: root.appendingPathComponent("Sources/AgentDeck/Tiler.swift"), encoding: .utf8)
        XCTAssertFalse(tiler.contains("keystroke"))
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

    // MARK: Tiling with a fake terminal

    private var scripts: [(String, [String])] = []
    private var tmuxCalls: [[String]] = []
    private let grid = Tiler.grid(on: Tiler.Screen(name: "S", frame: CGRect(x: 0, y: 0, width: 1440, height: 2560),
                                                   visible: CGRect(x: 0, y: 0, width: 1440, height: 2530), isMain: true), mainHeight: 2560)
    private let seats = [Tiler.Seat(hireID: "h1", desk: 0, socket: "/t/smolcorp-aaa"), Tiler.Seat(hireID: "h2", desk: 4, socket: "/t/smolcorp-bbb")]

    private func tmux(_ socket: String) -> HireTmux? {
        let tty = socket.hasSuffix("aaa") ? "/dev/ttys003" : "/dev/ttys004"
        return HireTmux(socketName: URL(fileURLWithPath: socket).lastPathComponent, testPort: nil) { self.tmuxCalls.append($0); return (0, tty) }
    }

    func testTerminalWindowsAreFoundByTheViewersTTYAndMovedToTheirCells() {
        let report = Tiler.tile(seats: seats, terminal: "terminal", grid: grid, tmux: tmux, script: { source, args in
            self.scripts.append((source, args))
            if args.isEmpty { return .ok("101 /dev/ttys003\n102 /dev/ttys004") }
            return .ok("\(args[1]) \(args[2]) \(args[3]) \(args[4])")
        }, teamGrid: { _ in XCTFail("windows are placed"); return false })
        XCTAssertEqual(report.method, .windows)
        XCTAssertEqual(report.placed, ["h1", "h2"])
        XCTAssertEqual(scripts.dropFirst().map(\.1), [["101", "0", "30", "480", "573"], ["102", "480", "603", "480", "573"]])
        XCTAssertEqual(tmuxCalls.first, ["-L", "smolcorp-aaa", "list-clients", "-t", "=hire", "-F", "#{client_tty}"])
        XCTAssertEqual(report.office, grid.office)
    }

    func testARefusedPermissionIsSaidPlainlyAndNotAskedAgain() {
        var asked = 0
        let report = Tiler.tile(seats: seats, terminal: "iterm", grid: grid, tmux: tmux, script: { _, _ in asked += 1; return .notAllowed },
                                teamGrid: { _ in false })
        XCTAssertEqual(asked, 1)
        XCTAssertTrue(report.needsPermission)
        XCTAssertTrue(report.message.contains("Automation") && report.message.contains("iTerm"))
        XCTAssertEqual(report.missed, ["h1", "h2"])
        XCTAssertTrue(report.placed.isEmpty)
    }

    func testARefusalMidwayStopsTheRest() {
        var moves = 0
        let report = Tiler.tile(seats: seats, terminal: "terminal", grid: grid, tmux: tmux, script: { _, args in
            if args.isEmpty { return .ok("101 /dev/ttys003\n102 /dev/ttys004") }
            moves += 1
            return .notAllowed
        }, teamGrid: { _ in false })
        XCTAssertEqual(moves, 1)
        XCTAssertTrue(report.needsPermission)
        XCTAssertEqual(report.missed, ["h1", "h2"])
    }

    func testAWindowThatLandedElsewhereCountsAsMissed() {
        let report = Tiler.tile(seats: seats, terminal: "terminal", grid: grid, tmux: tmux, script: { _, args in
            if args.isEmpty { return .ok("101 /dev/ttys003\n102 /dev/ttys004") }
            // The first window goes where it is told; the second answers a place far from its cell.
            return args[0] == "101" ? .ok("\(args[1]) \(args[2]) \(args[3]) \(args[4])") : .ok("900 900 480 573")
        }, teamGrid: { _ in false })
        XCTAssertEqual(report.placed, ["h1"])
        XCTAssertEqual(report.missed, ["h2"], "it answered another place than its cell")
    }

    func testAHireWithNoWindowOnItsViewersTTYCountsAsMissed() {
        let report = Tiler.tile(seats: seats, terminal: "terminal", grid: grid, tmux: tmux, script: { _, args in
            args.isEmpty ? .ok("101 /dev/ttys003") : .ok("\(args[1]) \(args[2]) \(args[3]) \(args[4])")
        }, teamGrid: { _ in false })
        XCTAssertEqual(report.placed, ["h1"])
        XCTAssertEqual(report.missed, ["h2"], "no window was found on its viewer's tty")
    }

    func testAnUnscriptableTerminalGetsTheTeamGrid() {
        var shown: [String] = []
        let report = Tiler.tile(seats: seats, terminal: "ghostty", grid: grid, tmux: tmux, script: { _, _ in XCTFail("no scripting"); return .failed },
                                teamGrid: { shown = $0.map(\.socketName); return true })
        XCTAssertEqual(report.method, .teamGrid)
        XCTAssertEqual(shown, ["smolcorp-aaa", "smolcorp-bbb"])
        XCTAssertEqual(report.placed, ["h1", "h2"])
    }

    func testNothingToTileSaysSo() {
        let report = Tiler.tile(seats: [], terminal: "terminal", grid: grid, tmux: tmux, script: { _, _ in .failed }, teamGrid: { _ in false })
        XCTAssertEqual(report.method, .none)
        XCTAssertFalse(report.message.isEmpty)
    }
}

final class TeamGridTests: XCTestCase {
    func testTheGridHasItsOwnSocket() {
        XCTAssertEqual(TeamGrid.socketName(testPort: nil), "smolcorp-team")
        XCTAssertEqual(TeamGrid.socketName(testPort: 7811), "smolcorp-7811-team")
    }

    func testEachPaneAttachesReadOnlyToItsHire() {
        XCTAssertEqual(TeamGrid.paneCommand(tmuxPath: "/opt/homebrew/bin/tmux", hireSocket: "smolcorp-ab12"),
                       "exec '/opt/homebrew/bin/tmux' -L 'smolcorp-ab12' attach -r -t '=hire'")
    }

    func testTheCommandsMakeOnePanePerHireOnOurServerOnly() {
        let calls = TeamGrid.commands(socket: "smolcorp-team", panes: ["one", "two", "three"])
        XCTAssertTrue(calls.allSatisfy { $0.prefix(2) == ["-L", "smolcorp-team"] })
        XCTAssertEqual(calls[0], ["-L", "smolcorp-team", "new-session", "-d", "-s", "team", "-x", "240", "-y", "60", "one"])
        XCTAssertEqual(calls.filter { $0[2] == "split-window" }.map { $0.last }, ["two", "three"])
        XCTAssertEqual(calls.last, ["-L", "smolcorp-team", "select-layout", "-t", "=team:", "tiled"])
        XCTAssertTrue(TeamGrid.commands(socket: "s", panes: []).isEmpty)
    }

    func testShowEndsOnlyItsOwnOldServerThenBuildsAndOpensTheWindow() {
        var ran: [[String]] = [], opened: [String] = []
        let hire = HireTmux(socketName: "smolcorp-ab12", testPort: nil) { _ in (0, "") }!
        let ok = TeamGrid.show(hires: [hire], terminal: "ghostty", tmuxPath: "/bin/tmux", run: { ran.append($0); return (0, "") },
                               open: { opened.append($0); return true })
        XCTAssertTrue(ok)
        XCTAssertEqual(ran.first, ["-L", TeamGrid.socketName(), "kill-server"])
        XCTAssertTrue(ran.allSatisfy { $0.prefix(2) == ["-L", TeamGrid.socketName()] })
        XCTAssertEqual(opened, ["exec '/bin/tmux' -L '\(TeamGrid.socketName())' attach-session -r -t '=team'"])
    }

    func testAFailedStepStopsBeforeOpeningAWindow() {
        let hire = HireTmux(socketName: "smolcorp-ab12", testPort: nil) { _ in (0, "") }!
        var opened = false
        XCTAssertFalse(TeamGrid.show(hires: [hire], terminal: "ghostty", tmuxPath: "/bin/tmux", run: { $0.contains("new-session") ? (1, "") : (0, "") },
                                     open: { _ in opened = true; return true }))
        XCTAssertFalse(opened)
    }
}
