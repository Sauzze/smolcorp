import XCTest
@testable import AgentDeck

/// The deck's Home Screen icon, in the page itself: iOS fetches a linked one outside the page, and where that fails
/// the Home Screen shows a letter.
final class HomeScreenIconTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    func testTheHomeScreenIconIsInThePageItself() throws {
        for logo in Logo.allCases {
            let icon = Self.repo.appendingPathComponent("Resources/web/icon-\(logo.rawValue).png")
            let png = try Data(contentsOf: icon)
            for page in ["deck.html", "pair.html"] {
                let original = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/" + page), encoding: .utf8)
                XCTAssertTrue(original.contains(#"<link rel="apple-touch-icon" href="/icon.png">"#), page)
                let inlined = String(decoding: DeckRouter.withIcon(Response(contentType: "text/html", body: Data(original.utf8)), icon: icon).body,
                                     as: UTF8.self)
                XCTAssertTrue(inlined.contains(#"<link rel="apple-touch-icon" sizes="180x180" href="data:image/png;base64,\#(png.base64EncodedString())">"#), page)
                XCTAssertFalse(inlined.contains(#"rel="apple-touch-icon" href="/icon.png""#), "no icon left for iOS to fetch: \(page)")
            }
        }
        let notFound = Response.status(404)
        XCTAssertEqual(DeckRouter.withIcon(notFound, icon: Self.repo.appendingPathComponent("Resources/web/icon-smolcorp.png")).body, notFound.body)
    }
}
