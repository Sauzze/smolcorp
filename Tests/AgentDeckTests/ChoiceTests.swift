import XCTest
@testable import AgentDeck

/// iPhone app (personal build): what a session asking you something offers,
/// shown on the phone as buttons and picked by its number, only while that
/// same question is open; a permission's command or file never in the snapshot.
final class ChoiceTests: XCTestCase {

    private let question: [String: Any] = [
        "questions": [[
            "question": "Which database?", "header": "DB", "multiSelect": false,
            "options": [["label": "Supabase", "description": "The usual"], ["label": "SQLite", "description": "Local only"]],
        ]],
    ]

    /// The digits type themselves in every terminal, never as a paste, and
    /// whatever the keyboard's layout.
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

    /// The GetCooked art director's menu, as Terminal.app showed it on 22 Oct
    /// 2026 when a tap on the phone didn't answer it: its question in a
    /// "│ " block, under a "☐" header.
    static let downloadsScreen = [
        "───────────────────────────────────────────────────────────────",
        " ☐ Downloads ",
        "",
        "│ Auto mode blocked my curl downloads from images.unsplash.com and images.pexels.com, which I need for the",
        "│ photo-style comparison board. May I download those from images.unsplash.com and images.pexels.com only?",
        "",
        "❯ 1. Yes, download them",
        "     Unsplash/Pexels CDNs only, into the worktree and the scratchpad. Nothing else is fetched.",
        "  2. No, skip photos",
        "     Ship the comparison with Fluent 3D only, plus a note on the photo style.",
        "  3. Type something.                       ",
        "───────────────────────────────────────────────────────────────",
        "  4. Chat about this",
        "",
        "Enter to select · ↑/↓ to navigate · Esc to cancel",
    ].joined(separator: "\n")

    static let downloads: [String: Any] = ["questions": [["question": "Auto mode blocked my curl downloads from images.unsplash.com and images.pexels.com, which I need for the photo-style comparison board. May I download those from images.unsplash.com and images.pexels.com only?", "header": "Downloads", "multiSelect": false, "options": [
        ["label": "Yes, download them", "description": "Unsplash/Pexels CDNs only, into the worktree and the scratchpad. Nothing else is fetched."],
        ["label": "No, skip photos", "description": "Ship the comparison with Fluent 3D only, plus a note on the photo style."],
    ]]]]

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
