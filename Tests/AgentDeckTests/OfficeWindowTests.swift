import XCTest
@testable import AgentDeck

/// The office's own window keeps the office's shape as it's resized.
final class OfficeWindowTests: XCTestCase {
    func testSidewaysStaysSidewaysInItsProportions() {
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 800, height: 300), by: .width), NSSize(width: 800, height: 450))
    }

    func testDrawnTallerThanWideItTurnsUpright() {
        // Dragged down from 640 × 360 to 640 × 700: upright, its height kept
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 640, height: 700), by: .height), NSSize(width: 422, height: 700))
        // And back: dragged wider than tall, sideways again
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 800, height: 700), by: .width), NSSize(width: 800, height: 450))
    }

    func testNeverSmallerThanTheOfficeDrawnOnce() {
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 100, height: 50), by: .width), OfficeWindow.wide)
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 100, height: 200), by: .height), OfficeWindow.tall)
    }

    func testOnlyTheOfficesOwnPagesOpen() {
        let address = URL(string: "http://127.0.0.1:7710/office?window&t=key")!
        XCTAssertTrue(OfficeWindow.isOwnPage(URL(string: "http://127.0.0.1:7710/events?t=key"), of: address))
        XCTAssertFalse(OfficeWindow.isOwnPage(URL(string: "http://127.0.0.1:7711/office"), of: address), "another port")
        XCTAssertFalse(OfficeWindow.isOwnPage(URL(string: "http://localhost:7710/office"), of: address))
        XCTAssertFalse(OfficeWindow.isOwnPage(URL(string: "https://example.com/?t=key"), of: address))
        XCTAssertFalse(OfficeWindow.isOwnPage(URL(string: "http://127.0.0.1.example.com:7710/"), of: address))
        XCTAssertFalse(OfficeWindow.isOwnPage(nil, of: address))
    }

    func testAResizeGoesByWhatsDragged() {
        let start = NSSize(width: 640, height: 360)
        // A bottom edge dragged: the height, whatever the width does meanwhile
        XCTAssertEqual(OfficeWindow.drive(from: start, to: NSSize(width: 640, height: 900), width: false, height: true), .height)
        XCTAssertEqual(OfficeWindow.fit(NSSize(width: 640, height: 900), by: .height), NSSize(width: 542, height: 900), "dragged down past its width, it's upright")
        XCTAssertEqual(OfficeWindow.drive(from: start, to: NSSize(width: 1000, height: 360), width: true, height: false), .width)
        // A corner: the one changed the more
        XCTAssertEqual(OfficeWindow.drive(from: start, to: NSSize(width: 700, height: 600), width: true, height: true), .height)
    }
}

/// The app opened again, or its Dock icon clicked: the one office, in front.
final class OfficeReopenTests: XCTestCase {
    private var defaults: UserDefaults!
    private let suite = "OfficeReopenTests-\(UUID().uuidString)"

    override func setUp() {
        defaults = UserDefaults(suiteName: suite)
    }

    override func tearDown() {
        defaults.removePersistentDomain(forName: suite)
    }

    private var panels: [NSWindow] { NSApp.windows.filter { $0 is OfficePanel && $0.isVisible } }

    func testHiddenItShowsAndShownItComesForwardWhereItIs() throws {
        _ = NSApplication.shared
        let office = OfficeWindow(address: URL(string: "http://127.0.0.1:9/office?window&t=test")!, defaults: defaults)
        let before = panels.count
        office.bringForward()
        XCTAssertTrue(office.isShown)
        let panel = try XCTUnwrap(panels.last)
        // Moved by hand, then the app opened again: in front, where it was, and still the one window.
        let moved = panel.frame.offsetBy(dx: -50, dy: -40)
        panel.setFrame(moved, display: false)
        office.bringForward()
        office.bringForward()
        XCTAssertEqual(panels.count, before + 1)
        XCTAssertEqual(panel.frame, moved)
        office.hide()
        XCTAssertFalse(office.isShown)
        office.bringForward()
        XCTAssertTrue(office.isShown)
        XCTAssertEqual(panels.count, before + 1)
        office.hide()
    }
}

/// Keep the office on top of other windows: setup's switch and the menu's
/// Always on Top, one setting, off on a new Mac.
final class OfficeOnTopTests: XCTestCase {
    private var defaults: UserDefaults!
    private let suite = "OfficeOnTopTests-\(UUID().uuidString)"
    private var file: URL!

    override func setUpWithError() throws {
        defaults = UserDefaults(suiteName: suite)
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        file = folder.appendingPathComponent("config.json")
    }

    override func tearDownWithError() throws {
        defaults.removePersistentDomain(forName: suite)
        try? FileManager.default.removeItem(at: file.deletingLastPathComponent())
    }

    func testANewMacStartsWithItOff() {
        XCTAssertEqual(OfficeWindow.settled(Config(), defaults: defaults), false)
    }

    func testAMacThatHadTheWindowKeepsWhatItHad() {
        // Never touched in the menu, where it was on by default: on.
        let set = Config()
        set.hasCompletedSetup = true
        XCTAssertEqual(OfficeWindow.settled(set, defaults: defaults), true)
        // Turned off in the menu: off.
        defaults.set(false, forKey: "officeWindow.alwaysOnTop")
        let off = Config()
        off.hasCompletedSetup = true
        XCTAssertEqual(OfficeWindow.settled(off, defaults: defaults), false)
    }

    func testSettledOnceItsLeftAsSet() {
        defaults.set(true, forKey: "officeWindow.alwaysOnTop")
        let config = Config()
        config.officeOnTop = false
        XCTAssertNil(OfficeWindow.settled(config, defaults: defaults))
    }

    func testItsTheWindowsAppsKeyInTheFile() throws {
        try #"{"alwaysOnTop": true}"#.write(to: file, atomically: true, encoding: .utf8)
        XCTAssertEqual(Config.load(from: file).officeOnTop, true)
        let config = Config()
        config.officeOnTop = false
        config.save(to: file)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: file)) as? [String: Any])
        XCTAssertEqual(saved["alwaysOnTop"] as? Bool, false)
        // Unset, it isn't in the file, to be settled at launch.
        Config().save(to: file)
        XCTAssertNil(Config.load(from: file).officeOnTop)
    }

    func testOnTopItFloatsOnEverySpaceAndBesideFullScreen() {
        let top = OfficeWindow.placement(onTop: true, onAllSpaces: false)
        XCTAssertEqual(top.level, .floating)
        XCTAssertEqual(top.spaces, [.canJoinAllSpaces, .fullScreenAuxiliary])
        let off = OfficeWindow.placement(onTop: false, onAllSpaces: false)
        XCTAssertEqual(off.level, .normal)
        XCTAssertEqual(off.spaces, [.fullScreenAuxiliary])
        // On every Space on its own, not on top.
        XCTAssertEqual(OfficeWindow.placement(onTop: false, onAllSpaces: true).level, .normal)
        XCTAssertEqual(OfficeWindow.placement(onTop: false, onAllSpaces: true).spaces, [.canJoinAllSpaces, .fullScreenAuxiliary])
    }

    func testTheWindowTakesItAtOnce() {
        let config = Config()
        config.officeOnTop = true
        let office = OfficeWindow(address: URL(string: "http://127.0.0.1:9/office?window&t=test")!, defaults: defaults, config: config)
        office.show()
        let panel = NSApp.windows.last { $0 is OfficePanel && $0.isVisible }
        XCTAssertEqual(panel?.level, .floating)
        config.officeOnTop = false
        office.applyOptions()
        XCTAssertEqual(panel?.level, .normal)
        office.hide()
    }
}
