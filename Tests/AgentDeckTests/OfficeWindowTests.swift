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

    /// The TV's big preview (TV3): a frame in the office may show a desk's own
    /// http://localhost:<port>; the window itself stays the office's.
    func testOnlyAFrameShowsADesksPage() {
        let address = URL(string: "http://127.0.0.1:7710/office?window&t=key")!
        let decide = { (text: String, mainFrame: Bool) in OfficeWindow.decide(URL(string: text), of: address, mainFrame: mainFrame) }
        // The office's own pages, in the window and in a frame, as before
        XCTAssertTrue(decide("http://127.0.0.1:7710/office?window&t=key", true))
        XCTAssertTrue(decide("http://127.0.0.1:7710/events?t=key", false))
        // A desk's page: in a frame, with its own paths, queries and redirects
        XCTAssertTrue(decide("http://localhost:3000/", false))
        XCTAssertTrue(decide("http://localhost:5173/app/login?next=%2F", false))
        XCTAssertTrue(decide("http://localhost:1/", false))
        XCTAssertTrue(decide("http://localhost:65535", false))
        XCTAssertTrue(decide("about:blank", false), "the frame before it loads")
        // Never the window itself
        XCTAssertFalse(decide("http://localhost:3000/", true), "the window taken by a desk's page")
        XCTAssertFalse(decide("about:blank", true))
        // Never Smolcorp's own ports, nor anything but http://localhost:<port>
        for text in ["http://localhost:7710/office", "http://localhost:7720/", "http://localhost:7730/",
                     "http://127.0.0.1:3000/", "http://127.0.0.1:7711/office", "https://localhost:3000/",
                     "http://localhost/", "http://localhost:0/", "http://localhost:65536/", "http://localhost.example.com:3000/",
                     "http://user:pass@localhost:3000/", "http://me@localhost:3000/", "ws://localhost:3000/", "file:///etc/hosts", "about:srcdoc",
                     "data:text/html,hi", "javascript:alert(1)", "https://example.com/"] {
            XCTAssertFalse(decide(text, false), text)
        }
        XCTAssertFalse(OfficeWindow.decide(nil, of: address, mainFrame: false))
        // The window's own port, when it isn't 7710 (a test copy's)
        let copy = URL(string: "http://127.0.0.1:47123/office?window&t=key")!
        XCTAssertFalse(OfficeWindow.decide(URL(string: "http://localhost:47123/office"), of: copy, mainFrame: false))
        XCTAssertFalse(OfficeWindow.decide(URL(string: "http://localhost:7710/"), of: copy, mainFrame: false), "7710, from a copy too")
        XCTAssertTrue(OfficeWindow.decide(URL(string: "http://localhost:3000/"), of: copy, mainFrame: false))
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
