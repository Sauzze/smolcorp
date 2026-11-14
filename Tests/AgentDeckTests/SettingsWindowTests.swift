import WebKit
import XCTest
@testable import AgentDeck

/// The settings window: what opens in it, and how it gets into the setup page
/// with no key in any address.
final class SettingsWindowTests: XCTestCase {
    private let port: UInt16 = 7710

    func testItsAddressNeverCarriesTheKey() {
        XCTAssertEqual(SettingsWindow.address(port: port).absoluteString, "http://127.0.0.1:7710/setup")
        XCTAssertEqual(SettingsWindow.address(port: port, then: "pair").absoluteString, "http://127.0.0.1:7710/setup#pair")
    }

    func testOnlyTheSetupPageOpensInIt() {
        let open = { (address: String?) in SettingsWindow.isSetupPage(address.flatMap(URL.init(string:)), port: self.port) }
        XCTAssertTrue(open("http://127.0.0.1:7710/setup"))
        XCTAssertTrue(open("http://127.0.0.1:7710/setup#viewer"))
        XCTAssertTrue(open("http://127.0.0.1:7710/setup/preview"))
        XCTAssertFalse(open("http://127.0.0.1:7710/"), "the deck")
        XCTAssertFalse(open("http://127.0.0.1:7710/setupx"))
        XCTAssertFalse(open("http://127.0.0.1:7711/setup"), "another port")
        XCTAssertFalse(open("https://127.0.0.1:7710/setup"))
        XCTAssertFalse(open("http://localhost:7710/setup"), "its cookie is 127.0.0.1's, never localhost's, where previews are")
        XCTAssertFalse(open("http://[::1]:7710/setup"))
        XCTAssertFalse(open("http://localhost.example.com:7710/setup"))
        XCTAssertFalse(open("http://example.com/setup"))
        XCTAssertFalse(open(nil))
    }

    func testALinkElsewhereOpensInTheBrowser() {
        let decide = { (address: String, mainFrame: Bool, clicked: Bool, download: Bool) in
            SettingsWindow.decide(URL(string: address), port: self.port, mainFrame: mainFrame, clicked: clicked, download: download)
        }
        XCTAssertEqual(decide("http://127.0.0.1:7710/setup#pair", true, false, false), .allow)
        XCTAssertEqual(decide("http://localhost:7710/setup#pair", true, false, false), .cancel, "Setup at localhost, not clicked")
        XCTAssertEqual(decide("https://vercel.com/new", true, true, false), .openOutside)
        XCTAssertEqual(decide("mailto:someone@example.com", true, true, false), .openOutside)
        XCTAssertEqual(decide("http://localhost:7710/", true, true, false), .openOutside, "the deck, in the browser")
        // Not clicked, as a script would send it: nowhere
        XCTAssertEqual(decide("https://example.com/", true, false, false), .cancel)
        XCTAssertEqual(decide("file:///etc/hosts", true, true, false), .cancel)
        XCTAssertEqual(decide("javascript:alert(1)", true, true, false), .cancel)
        // The preview's frame
        XCTAssertEqual(decide("about:blank", false, false, false), .allow)
        XCTAssertEqual(decide("about:blank", true, false, false), .cancel)
        XCTAssertEqual(decide("https://example.com/", false, false, false), .cancel)
    }

    func testOnlyThePagesOwnFilesAreSaved() {
        let decide = { (address: String) in
            SettingsWindow.decide(URL(string: address), port: self.port, mainFrame: true, clicked: true, download: true)
        }
        XCTAssertEqual(decide("blob:http://127.0.0.1:7710/6f1c2a3e-0000-4000-8000-000000000000"), .download, "a profile's export")
        XCTAssertEqual(decide("http://127.0.0.1:7710/setup/qr.png"), .download)
        XCTAssertEqual(decide("blob:http://localhost:7710/6f1c2a3e"), .cancel, "not Setup's own origin")
        XCTAssertEqual(decide("blob:https://example.com/6f1c2a3e"), .cancel)
        XCTAssertEqual(decide("https://example.com/file.json"), .cancel)
        XCTAssertEqual(decide("blob:http://localhost:7711/6f1c2a3e"), .cancel)
    }

    func testTheEditingKeysAreTaken() {
        XCTAssertEqual(SettingsPanel.editing("v", .command), #selector(NSText.paste(_:)))
        XCTAssertEqual(SettingsPanel.editing("z", [.command, .shift]), Selector(("redo:")))
        XCTAssertNil(SettingsPanel.editing("v", [.command, .option]))
        XCTAssertNil(SettingsPanel.editing("q", .command))
    }

    func testItsCookieIsTheOneTheBrowserIsGiven() throws {
        let cookie = try XCTUnwrap(SettingsWindow.sessionCookie(session: "abc_DEF-123"))
        XCTAssertEqual(cookie.name, SettingsWindow.setupCookie)
        XCTAssertEqual(cookie.value, "abc_DEF-123")
        XCTAssertEqual(cookie.domain, "127.0.0.1")
        XCTAssertEqual(cookie.path, "/setup")
        XCTAssertTrue(cookie.isHTTPOnly)
        XCTAssertEqual(cookie.sameSitePolicy, .sameSiteStrict)
        XCTAssertTrue(cookie.isSessionOnly, "kept in memory, never saved")
    }
}
