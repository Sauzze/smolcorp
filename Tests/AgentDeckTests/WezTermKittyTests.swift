import XCTest
@testable import AgentDeck

/// WezTerm and kitty through stand-ins that record what they're asked, as the Go tests do.
final class WezTermKittyTests: XCTestCase {
    private var folder: URL!
    private var saved: (String?, String?)
    private var log: URL { folder.appendingPathComponent("asked.txt") }
    /// One line per call; split by bytes, as "\r\n" would otherwise be one character.
    private var asked: [String] {
        ((try? String(contentsOf: log, encoding: .utf8)) ?? "").components(separatedBy: "\n").filter { !$0.isEmpty }
    }

    private func standIn(_ name: String, _ body: String) throws -> String {
        let url = folder.appendingPathComponent(name)
        try "#!/bin/sh\nprintf '%s\\n' \"${WEZTERM_UNIX_SOCKET:+socket=$WEZTERM_UNIX_SOCKET }$*\" >> \"\(log.path)\"\n\(body)\n"
            .write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url.path
    }

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        saved = (WezTerm.path, Kitty.path)
        WezTerm.path = try standIn("wezterm", """
            case "$*" in *list*) echo '[{"window_id":0,"tab_id":0,"pane_id":5,"tty_name":"/dev/ttys042"}]' ;; esac
            """)
        Kitty.path = try standIn("kitten", """
            case "$*" in *"ls --match id:9"*) echo "No matching windows for expression: id:9" >&2; exit 1 ;;
              *"ls --match id:8"*) echo "Failed to connect to unix:/tmp/kitty-99" >&2; exit 1 ;;
              *ls*) echo '[{"tabs":[{"windows":[{"id":3,"pid":1}]}]}]' ;; esac
            """)
    }

    override func tearDownWithError() throws {
        (WezTerm.path, Kitty.path) = saved
        try? FileManager.default.removeItem(at: folder)
    }

    private let pane = TabAddress("wezterm:5@/tmp/wezterm-gui-sock")!
    private let window = TabAddress("kitty:3@unix:/tmp/kitty-99")!

    func testWezTermKeysAndTextGoAsBytesToThePane() {
        XCTAssertTrue(WezTerm().press(.init(key: .tab, modifiers: .maskShift), in: pane))
        XCTAssertTrue(WezTerm().type("-n run it", in: pane))
        XCTAssertEqual(asked, [
            "socket=/tmp/wezterm-gui-sock cli send-text --pane-id 5 --no-paste \u{1b}[Z",
            "socket=/tmp/wezterm-gui-sock cli send-text --pane-id 5 --no-paste -- -n run it",
            "socket=/tmp/wezterm-gui-sock cli send-text --pane-id 5 --no-paste \r",
        ])
    }

    func testWezTermKnowsThePaneFromItsList() {
        XCTAssertEqual(WezTerm().isOpen(pane), true)
        XCTAssertEqual(WezTerm().tty(of: pane), "/dev/ttys042")
        XCTAssertEqual(WezTerm().isOpen(TabAddress("wezterm:7@/tmp/wezterm-gui-sock")!), false)
    }

    func testKittyKeysAndTextAreEscapedAsSendTextReadsThem() {
        XCTAssertTrue(Kitty().press(.init(key: .escape), in: window))
        XCTAssertTrue(Kitty().type(#"a\b"#, in: window))
        XCTAssertEqual(asked, [
            "@ --to unix:/tmp/kitty-99 send-text --match id:3 \\x1b",
            #"@ --to unix:/tmp/kitty-99 send-text --match id:3 -- a\\b\x0d"#,
        ])
    }

    func testKittyScrollsByLines() {
        XCTAssertTrue(Kitty().scroll(lines: -4, in: window))
        XCTAssertTrue(Kitty().scrollToBottom(in: window))
        XCTAssertEqual(asked, ["@ --to unix:/tmp/kitty-99 scroll-window --match id:3 4l-", "@ --to unix:/tmp/kitty-99 scroll-window --match id:3 end"])
    }

    func testKittyKnowsAWindowThatIsntThere() {
        XCTAssertEqual(Kitty().isOpen(window), true)
        XCTAssertEqual(Kitty().isOpen(TabAddress("kitty:9@unix:/tmp/kitty-99")!), false)
        XCTAssertNil(Kitty().isOpen(TabAddress("kitty:8@unix:/tmp/kitty-99")!), "kitty not answering is unknown, not closed")
        XCTAssertEqual(Kitty().fire(TabAddress("kitty:9@unix:/tmp/kitty-99")!, agent: .claude), .gone)
    }

    func testNeitherNeedsAccessibility() {
        XCTAssertFalse(WezTerm().keysNeedAccessibility)
        XCTAssertFalse(Kitty().keysNeedAccessibility)
    }
}
