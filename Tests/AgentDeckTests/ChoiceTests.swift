import XCTest
@testable import AgentDeck

/// What the phone's page says when a tap didn't go through.
final class ChoiceTests: XCTestCase {
    /// The digits a menu is picked by (the talk bubble's pick) type themselves in every terminal, never as a paste,
    /// and whatever the keyboard's layout.
    func testDigitsAreTypedAsKeysEverywhere() {
        for n in 1...9 {
            let key = Terminal.Key.digit(n)
            XCTAssertEqual(key?.character, String(n))
            let stroke = Terminal.Keystroke(key: key!)
            XCTAssertEqual(stroke.tmuxName, String(n))
            XCTAssertEqual(stroke.bytes, String(n))
        }
        XCTAssertNil(Terminal.Key.digit(0))
        XCTAssertNil(Terminal.Key.digit(10))
        XCTAssertNil(Terminal.Key.returnKey.character)
        XCTAssertEqual(Set((1...9).compactMap { Terminal.Key.digit($0)?.rawValue }).count, 9)
    }

    /// Why a tap didn't answer stays on the phone long enough to read.
    func testTheReasonStaysLongEnoughToRead() throws {
        let page = try String(contentsOfFile: Self.root + "Resources/web/deck.html", encoding: .utf8)
        let start = try XCTUnwrap(page.range(of: "function flash(text) {")).upperBound
        let body = page[start...].prefix(500)
        let delay = try XCTUnwrap(body.range(of: #"\}, (\d+)\);"#, options: .regularExpression))
        let ms = try XCTUnwrap(Int(body[delay].filter(\.isNumber)))
        XCTAssertGreaterThanOrEqual(ms, 4000)
    }

    private static let root: String = {
        var url = URL(fileURLWithPath: #filePath)
        for _ in 0..<3 { url.deleteLastPathComponent() }
        return url.path + "/"
    }()
}
