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

    /// What `tailscale serve` hands this Mac: from its loopback, the
    /// tailnet name kept as Host, and who it's for in forwarding headers.
    private func throughTailscale(_ method: String, _ path: String, query: [String: String] = [:],
                                  host: String = "my-mac.tail1234.ts.net", extra: [String: String] = [:]) -> Request {
        var headers = ["host": host, "x-forwarded-for": "100.101.102.103", "x-forwarded-proto": "https",
                       "x-forwarded-host": "my-mac.tail1234.ts.net", "tailscale-user-login": "someone@example.com"]
        headers.merge(extra) { _, given in given }
        return Request.addressed(method: method, path: path, query: query, headers: headers, body: Data(),
                                 fromLoopback: true, remoteAddress: "127.0.0.1")
    }

    private func fromThisMac(_ method: String, _ path: String, headers: [String: String] = [:]) -> Request {
        Request.addressed(method: method, path: path, query: [:], headers: ["host": "localhost:7710"].merging(headers) { _, b in b },
                          body: Data(), fromLoopback: true, remoteAddress: "127.0.0.1")
    }

    func testTheHookIsntLocalThroughTailscaleNorItsFloorTwoReports() {
        let hook = ["x-agentdeck-token": "the-real-hook-key", "x-agentdeck-pid": "4242", "x-agentdeck-parent": "parent-session",
                    "x-agentdeck-agent": "claude", "x-agentdeck-tab": "tty:/dev/ttys003"]
        XCTAssertFalse(throughTailscale("POST", "/hook", extra: hook).isLocal)
        XCTAssertFalse(throughTailscale("POST", "/hook", host: "localhost", extra: hook).isLocal)
    }

    func testAnyOneForwardingHeaderIsEnough() {
        for header in ["x-forwarded-for", "x-forwarded-host", "x-forwarded-proto", "forwarded", "tailscale-user-login"] {
            XCTAssertFalse(fromThisMac("GET", "/setup/viewer", headers: [header: "x"]).isLocal, header)
        }
    }

    func testThisMacItselfIsStillLocal() {
        XCTAssertTrue(fromThisMac("GET", "/setup/viewer").isLocal)
        XCTAssertTrue(fromThisMac("POST", "/hook").isLocal)
        let fromNetwork = Request.addressed(method: "GET", path: "/setup", query: [:], headers: ["host": "localhost"], body: Data(),
                                            fromLoopback: false, remoteAddress: "192.168.1.9")
        XCTAssertFalse(fromNetwork.isLocal)
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
