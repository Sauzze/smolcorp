import XCTest
@testable import AgentDeck

/// Phone microphone (personal build), behind `tailscale serve`: a device on
/// the tailnet reaches no more than a paired phone does, route by route,
/// including those added since the phone mic was first written. Offline.
final class TailnetRouteTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private static func source(_ path: String) throws -> String {
        try String(contentsOf: repo.appendingPathComponent(path), encoding: .utf8)
    }

    func testSmolcorpOnlyEverReadsTailscalesStatus() throws {
        let sources = try FileManager.default.contentsOfDirectory(atPath: Self.repo.appendingPathComponent("Sources/AgentDeck").path)
        for file in sources where file.hasSuffix(".swift") {
            let text = try Self.source("Sources/AgentDeck/" + file).lowercased()
            XCTAssertFalse(text.contains("\"funnel\""), file)
            // `serve` only ever to read its status: never to start, change or stop serving.
            let read = #"["serve", "status", "--json"]"#
            XCTAssertFalse(text.replacingOccurrences(of: read, with: "").contains("\"serve\""), file)
        }
        // Two looks, both read-only, and nothing else.
        let tailnet = try Self.source("Sources/AgentDeck/PhoneDictation.swift")
        XCTAssertEqual(tailnet.components(separatedBy: "run(tool, ").count, 3)
        XCTAssertTrue(tailnet.contains(#"run(tool, ["status", "--json"])"#))
        XCTAssertTrue(tailnet.contains(#"run(tool, ["serve", "status", "--json"])"#))
    }
}
