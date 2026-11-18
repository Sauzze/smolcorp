import XCTest
@testable import AgentDeck

/// Tabs by address, whatever their terminal, and what the deck may do in each.
final class TerminalDriverTests: XCTestCase {
    func testAddressesReadAsTheHookWritesThem() {
        let cases: [(String, String, String, String)] = [
            ("tty:/dev/ttys003", "tty", "/dev/ttys003", ""),
            ("tmux:%3@/private/tmp/tmux-501/default", "tmux", "%3", "/private/tmp/tmux-501/default"),
            ("iterm:BD2206F6-EE01-476F-A6D9-3061679BD8B0", "iterm", "BD2206F6-EE01-476F-A6D9-3061679BD8B0", ""),
            ("ghostty:/dev/ttys004", "ghostty", "/dev/ttys004", ""),
            ("wezterm:5@/Users/x/.local/share/wezterm/gui-sock-1", "wezterm", "5", "/Users/x/.local/share/wezterm/gui-sock-1"),
            ("kitty:3@unix:/tmp/kitty-99", "kitty", "3", "unix:/tmp/kitty-99"),
            ("other:/dev/ttys005", "other", "/dev/ttys005", ""),
        ]
        for (text, kind, id, socket) in cases {
            let address = TabAddress(text)
            XCTAssertEqual(address?.kind, kind, text)
            XCTAssertEqual(address?.id, id, text)
            XCTAssertEqual(address?.socket, socket, text)
            XCTAssertEqual(address?.text, text, "written back as read")
        }
        XCTAssertNil(TabAddress(""))
        XCTAssertNil(TabAddress("nothing"))
    }

    func testABareTTYIsATerminalTab() {
        XCTAssertEqual(TabAddress("/dev/ttys003"), TabAddress(kind: "tty", id: "/dev/ttys003"))
    }

    func testWhatEachTerminalLetsTheDeckDo() {
        XCTAssertEqual(Terminals.abilities(of: "tty:/dev/ttys003"), [.keys, .text, .focus, .fire, .scroll])
        XCTAssertEqual(Terminals.abilities(of: "iterm:BD22"), [.keys, .text, .focus, .fire], "no scrolling through iTerm2's AppleScript")
        if Tmux.path != nil { XCTAssertEqual(Terminals.abilities(of: "tmux:%3@/tmp/s"), [.keys, .text, .focus, .fire, .scroll]) }
        // Ghostty: only watched until its terminal on that tty has been learned.
        for watched in ["other:/dev/ttys005", "ghostty:/dev/ttys004", ""] {
            XCTAssertEqual(Terminals.abilities(of: watched), .none, watched)
        }
    }

    func testOnlyTerminalAppsKeysNeedAccessibility() {
        XCTAssertTrue(TerminalApp().keysNeedAccessibility)
        XCTAssertFalse(Tmux().keysNeedAccessibility)
        XCTAssertFalse(ITerm().keysNeedAccessibility)
    }

    func testATabsTTYIsKnownWhereItHasOne() {
        XCTAssertEqual(Terminals.tty(of: "tty:/dev/ttys003"), "/dev/ttys003")
        XCTAssertEqual(Terminals.tty(of: "other:/dev/ttys005"), "/dev/ttys005")
        XCTAssertNil(Terminals.ttyInAddress(of: "iterm:BD2206F6"), "asked of iTerm2, not read from the address")
        XCTAssertNil(Terminals.ttyInAddress(of: "tmux:%3@/tmp/s"))
    }

    /// Nothing smuggled in: an address carrying a line break, or not in its
    /// terminal's own shape, isn't one.
    func testOnlyAddressesInTheirOwnShape() {
        for bad in ["tty:/dev/ttys003\r\nX-AgentDeck-Tab: tty:/dev/ttys004", "tty:/etc/passwd", "tty:/dev/../ttys1", "/dev/ttys00 3",
                    "tmux:3@/tmp/s", "tmux:%3x@/tmp/s", "tmux:%3@/tmp/s\n", "kitty:3a@unix:/tmp/k", "wezterm:@/tmp/w",
                    "iterm:BD22\")", "ghostty:/dev/../x", "other:ttys005",
                    "tmux:%3", "tmux:%3@", "kitty:3", "wezterm:5", "wezterm:5@", "iterm:\u{FF11}\u{FF12}"] {
            XCTAssertNil(TabAddress(bad), bad)
        }
    }

    func testGhosttysTextIsTypedAsItsTextActionReadsIt() {
        XCTAssertEqual(Ghostty.escaped("/clear\r"), "/clear\\r")
        XCTAssertEqual(Ghostty.escaped("\u{1b}[A"), "\\x1b[A")
        XCTAssertEqual(Ghostty.escaped("a\\b"), "a\\\\b", "a backslash typed as one")
        XCTAssertEqual(Ghostty.escaped("\u{03}"), "\\x03")
        XCTAssertEqual(Ghostty.escaped("café ✓"), "café ✓")
        XCTAssertFalse(Ghostty().keysNeedAccessibility)
    }
}
