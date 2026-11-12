import XCTest
@testable import AgentDeck

/// ✨ Summary made by a coding agent: what setup's page says about it.
final class SummaryProvidersTests: XCTestCase {
    func testTheSetupPageKeepsTheKeyHiddenAndWarns() throws {
        let page = try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .appendingPathComponent("../../Resources/web/setup.html"), encoding: .utf8)
        XCTAssertTrue(page.contains(#"<input type="password" id="summary-key" spellcheck="false" autocomplete="new-password""#))
        XCTAssertTrue(page.contains("Use a key from a dedicated workspace with a spending limit."))
        XCTAssertTrue(page.contains(#"href="https://console.anthropic.com/settings/limits""#))
        XCTAssertTrue(page.contains("Only Smolcorp's summary call runs on your plan; your terminals and agents keep their own keys and logins."))
        XCTAssertTrue(page.contains("Your chosen provider isn't supported yet; pick Claude Code, OpenCode or the API key."))
    }
}
