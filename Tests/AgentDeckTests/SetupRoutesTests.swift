import XCTest
@testable import AgentDeck

/// Which callers may use which setup route, driven through the real gate, row by row.
final class SetupRoutesTests: XCTestCase {

    private func fixture(_ name: String) throws -> [String: Any] {
        try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: FrontDeskFixtures.url(name))) as? [String: Any])
    }

    func testTheFixtureHasNoRepeatedRowAndNamesOnlyKnownCallers() throws {
        let rows = try XCTUnwrap(try fixture("routes.json")["routes"] as? [[String: Any]])
        var seen = Set<String>()
        for row in rows {
            let key = "\(row["method"] as? String ?? "") \(row["path"] as? String ?? "")"
            XCTAssertTrue(seen.insert(key).inserted, "\(key) twice")
            XCTAssertTrue(Set(row["callers"] as? [String] ?? []).isSubset(of: ["gm", "page", "tool", "hire"]), key)
            XCTAssertTrue((row["callers"] as? [String] ?? []).contains("page"), key)
        }
    }
}
