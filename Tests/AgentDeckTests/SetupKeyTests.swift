import XCTest
@testable import AgentDeck

/// The setup page's cookie, read from what the browser sends.
final class SetupKeyTests: XCTestCase {
    private func request(cookie: String?) -> Request {
        Request(method: "GET", path: "/setup/state", query: [:], headers: cookie.map { ["cookie": $0] } ?? [:], body: Data(),
                isLocal: true, isAddressedDirectly: true, remoteAddress: "127.0.0.1")
    }

    func testTheSetupCookieIsFoundAmongOthers() {
        let sent = request(cookie: "theme=dark; smolcorp-setup=abc_DEF-123; other=1")
        XCTAssertEqual(DeckRouter.cookie(DeckRouter.setupCookie, in: sent), "abc_DEF-123")
    }

    func testACookieThatOnlyStartsTheSameIsntIt() {
        XCTAssertNil(DeckRouter.cookie(DeckRouter.setupCookie, in: request(cookie: "smolcorp-setup-old=abc")))
        XCTAssertNil(DeckRouter.cookie(DeckRouter.setupCookie, in: request(cookie: nil)))
    }

    /// Opening /setup counts as setup only with a page session or the setup key (SIERRA 1.22.1 m1): a page
    /// anyone on this Mac opens without either leaves setup as it was. The gate says who is asking.
    func testOnlyARequestWithASessionOrTheSetupKeyCarriesIt() throws {
        let sessions = SetupSessions()
        let session = try XCTUnwrap(sessions.open(link: sessions.newLink()))
        let isKey = { (key: String?) in key == "right" }
        func get(_ headers: [String: String]) -> SetupSessions.Caller {
            let request = Request(method: "GET", path: "/setup", query: [:], headers: headers, body: Data(),
                                  isLocal: true, isAddressedDirectly: true, remoteAddress: "127.0.0.1")
            return DeckRouter.setupGate(request, sessions: sessions, isSetupKey: isKey).caller
        }
        XCTAssertEqual(get([:]), .nobody)
        XCTAssertEqual(get([DeckRouter.setupKeyHeader: "wrong"]), .nobody)
        XCTAssertEqual(get(["cookie": "\(DeckRouter.setupCookie)=wrong"]), .nobody)
        XCTAssertEqual(get(["cookie": "\(DeckRouter.setupCookie)=right"]), .nobody, "the file's key is no cookie")
        XCTAssertEqual(get([DeckRouter.setupKeyHeader: "right"]), .tool)
        XCTAssertEqual(get(["cookie": "theme=dark; \(DeckRouter.setupCookie)=\(session)"]), .page)
    }

    /// Setup says which Smolcorp this is (SIERRA 1.22.1 m4): the app's version, and "dev" for a build without one.
    func testSetupShowsTheAppsVersion() throws {
        func bundle(_ info: [String: String]?) throws -> Bundle {
            let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString + ".bundle")
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
            addTeardownBlock { try? FileManager.default.removeItem(at: folder) }
            if let info {
                let plist = try PropertyListSerialization.data(fromPropertyList: info, format: .xml, options: 0)
                try plist.write(to: folder.appendingPathComponent("Info.plist"))
            }
            return Bundle(url: folder)!
        }
        XCTAssertEqual(DeckRouter.version(of: try bundle(nil)), "dev")
        XCTAssertEqual(DeckRouter.version(of: try bundle(["CFBundleShortVersionString": "9.8.7"])), "9.8.7")
    }
}
