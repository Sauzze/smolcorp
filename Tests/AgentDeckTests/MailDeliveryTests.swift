import XCTest
@testable import AgentDeck

/// Mail delivery with a stand-in tmux and socket: nothing is typed anywhere.
final class MailDeliveryTests: XCTestCase {

    func testTmuxInsertRefusesControlCharactersToo() {
        XCTAssertFalse(HireTmux.canPaste("a\u{1B}[201~b"))
        XCTAssertFalse(HireTmux.canPaste("a\rb"))
        XCTAssertTrue(HireTmux.canPaste("a\nb\tc — é"))
        let before = Tmux.path
        defer { Tmux.path = before }
        Tmux.path = "/usr/bin/false"
        XCTAssertFalse(Tmux().insert("x\u{1B}[201~\r", in: TabAddress(kind: "tmux", id: "%1")))
    }
}
