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

    func testOnlyTerminalAppIsDrivenForNow() {
        XCTAssertEqual(Terminals.abilities(of: "tty:/dev/ttys003"), [.keys, .text, .focus, .fire, .scroll])
        for watched in ["other:/dev/ttys005", "ghostty:/dev/ttys004", "iterm:BD22", "tmux:%3@/tmp/s", ""] {
            XCTAssertEqual(Terminals.abilities(of: watched), .none, watched)
        }
    }

    func testATabsTTYIsKnownWhereItHasOne() {
        XCTAssertEqual(Terminals.tty(of: "tty:/dev/ttys003"), "/dev/ttys003")
        XCTAssertEqual(Terminals.tty(of: "other:/dev/ttys005"), "/dev/ttys005")
        XCTAssertNil(Terminals.tty(of: "iterm:BD2206F6"))
        XCTAssertNil(Terminals.tty(of: "tmux:%3@/tmp/s"))
    }

    func testADeckSavedBefore19StillLoadsItsTabs() throws {
        let old = #"{"id":"s1","agent":"claude","folder":"/w","project":"/w","tty":"/dev/ttys003","state":"idle","effort":"","model":"","transcript":"","startedAt":1,"lastEventAt":1}"#
        let saved = try JSONDecoder().decode(Sessions.Saved.self, from: Data(old.utf8))
        XCTAssertEqual(saved.address, "tty:/dev/ttys003")
        let sessions = Sessions()
        sessions.restore([saved], fresh: true)
        XCTAssertEqual(sessions["s1"]?.tab, "tty:/dev/ttys003")
    }

    func testASessionAndItsReplacementShareATab() {
        let sessions = Sessions()
        sessions.apply(["session_id": "a", "hook_event_name": "SessionStart", "source": "startup", "cwd": "/w"],
                       agent: .claude, tab: "iterm:ABC", startedIn: "/w", at: 1)
        sessions.apply(["session_id": "b", "hook_event_name": "SessionStart", "source": "clear", "cwd": "/w"],
                       agent: .claude, tab: "iterm:ABC", startedIn: "/w", at: 2)
        XCTAssertNil(sessions["a"], "the new session took the old one's place in the tab")
        XCTAssertEqual(sessions["b"]?.tab, "iterm:ABC")
        let saved = sessions.saved.first
        XCTAssertEqual(saved?.tty, "", "a tab that isn't Terminal.app's has no tty to save for older versions")
        XCTAssertEqual(saved?.address, "iterm:ABC")
    }
}
