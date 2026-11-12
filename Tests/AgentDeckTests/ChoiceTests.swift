import XCTest
@testable import AgentDeck

/// What the phone's page says when a tap didn't go through.
final class ChoiceTests: XCTestCase {
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
