import WebKit
import XCTest
@testable import AgentDeck

/// The settings window: what opens in it, and how it gets into the setup page
/// with no key in any address.
final class SettingsWindowTests: XCTestCase {
    private let port: UInt16 = 7710

    func testItsAddressNeverCarriesTheKey() {
        XCTAssertEqual(SettingsWindow.address(port: port).absoluteString, "http://localhost:7710/setup")
        XCTAssertEqual(SettingsWindow.address(port: port, then: "pair").absoluteString, "http://localhost:7710/setup#pair")
    }

    func testItsCookieIsTheOneTheBrowserIsGiven() throws {
        let cookie = try XCTUnwrap(SettingsWindow.sessionCookie(key: "abc_DEF-123"))
        XCTAssertEqual(cookie.name, DeckRouter.setupCookie)
        XCTAssertEqual(cookie.value, "abc_DEF-123")
        XCTAssertEqual(cookie.domain, "localhost")
        XCTAssertEqual(cookie.path, "/setup")
        XCTAssertTrue(cookie.isHTTPOnly)
        XCTAssertEqual(cookie.sameSitePolicy, .sameSiteStrict)
        XCTAssertTrue(cookie.isSessionOnly, "kept in memory, never saved")
        // As WebKit sends it, the router reads it back
        let sent = Request(method: "GET", path: "/setup/state", query: [:], headers: ["cookie": "\(cookie.name)=\(cookie.value)"],
                           body: Data(), isLocal: true, isAddressedDirectly: true, remoteAddress: "127.0.0.1")
        XCTAssertEqual(DeckRouter.cookie(DeckRouter.setupCookie, in: sent), "abc_DEF-123")
    }

    func testOnlyTheSetupPageOpensInIt() {
        let open = { (address: String?) in SettingsWindow.isSetupPage(address.flatMap(URL.init(string:)), port: self.port) }
        XCTAssertTrue(open("http://localhost:7710/setup"))
        XCTAssertTrue(open("http://localhost:7710/setup#viewer"))
        XCTAssertTrue(open("http://localhost:7710/setup/preview"))
        XCTAssertFalse(open("http://localhost:7710/"), "the deck")
        XCTAssertFalse(open("http://localhost:7710/setupx"))
        XCTAssertFalse(open("http://localhost:7711/setup"), "another port")
        XCTAssertFalse(open("https://localhost:7710/setup"))
        XCTAssertFalse(open("http://127.0.0.1:7710/setup"), "its cookie is localhost's")
        XCTAssertFalse(open("http://localhost.example.com:7710/setup"))
        XCTAssertFalse(open("http://example.com/setup"))
        XCTAssertFalse(open(nil))
    }

    func testALinkElsewhereOpensInTheBrowser() {
        let decide = { (address: String, mainFrame: Bool, clicked: Bool, download: Bool) in
            SettingsWindow.decide(URL(string: address), port: self.port, mainFrame: mainFrame, clicked: clicked, download: download)
        }
        XCTAssertEqual(decide("http://localhost:7710/setup#pair", true, false, false), .allow)
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
        XCTAssertEqual(decide("blob:http://localhost:7710/6f1c2a3e-0000-4000-8000-000000000000"), .download, "a profile's export")
        XCTAssertEqual(decide("http://localhost:7710/setup/qr.png"), .download)
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

    /// The window's own page, in WebKit, against a server standing in for the
    /// deck on a port of its own, never the app's: the setup key reaches it as
    /// the cookie, only under /setup, and no address ever carries it.
    func testThePageReadsSetupWithTheKeyInNoAddress() throws {
        let key = randomKey()
        let (stub, web) = try open(serverKey: key, windowKey: key)
        XCTAssertEqual(web.title, "done 200 mac mac #pair")
        XCTAssertEqual(stub.keyAt["/setup/state"], true, "the key came as the cookie")
        XCTAssertEqual(stub.keyAt["/elsewhere"], false, "not sent outside /setup")
        XCTAssertFalse(stub.addresses.isEmpty)
        XCTAssertFalse(stub.addresses.contains { $0.contains(key) }, "no address carries the key")
        XCTAssertFalse((web.url?.absoluteString ?? "").contains(key))
    }

    func testAWrongKeyIsRefused() throws {
        let (_, web) = try open(serverKey: randomKey(), windowKey: randomKey())
        XCTAssertEqual(web.title, "done 401 mac mac #pair")
    }

    /// The window's page, at #pair, against a stand-in server on a port of its own.
    private func open(serverKey: String, windowKey: String) throws -> (SetupStub, WKWebView) {
        let stub = SetupStub(key: serverKey)
        let port = UInt16.random(in: 47_100...47_900)
        let server = HTTPServer(port: port, router: stub)
        var listening = false
        server.onReady = { listening = true }
        server.start()
        servers.append(server)
        XCTAssertTrue(wait(10) { listening }, "the stand-in server listens")

        let settings = SettingsWindow(port: port, setupKey: windowKey)
        let web = settings.page(frame: NSRect(x: 0, y: 0, width: 900, height: 640), then: "pair")
        XCTAssertTrue(wait(15) { web.title?.hasPrefix("done") == true }, "the page loaded and read setup: \(web.title ?? "nothing")")
        return (stub, web)
    }

    private var servers: [HTTPServer] = []

    /// Runs the main run loop until `done`, for up to `seconds`.
    private func wait(_ seconds: TimeInterval, until done: () -> Bool) -> Bool {
        let end = Date().addingTimeInterval(seconds)
        while !done() && Date() < end { RunLoop.main.run(until: Date().addingTimeInterval(0.05)) }
        return done()
    }
}

/// The deck's setup check, as DeckRouter makes it, and a page that reads
/// through it and puts what it got in its title.
private final class SetupStub: Router {
    let key: String
    private(set) var addresses: [String] = []
    private(set) var keyAt: [String: Bool] = [:]

    init(key: String) { self.key = key }

    static let page = """
        <!doctype html><title>loading</title><script>
        (async () => {
          const state = await fetch('/setup/state', { headers: { 'X-AgentDeck-Setup': '1' } });
          await fetch('/elsewhere');
          const app = window.smolcorpApp ? window.smolcorpApp.platform : 'none';
          document.title = ['done', state.status, app, document.documentElement.dataset.smolcorpApp, location.hash].join(' ');
        })();
        </script>
        """

    func handle(_ request: Request, reply: @escaping (Reply) -> Void) {
        addresses.append(request.path + "?" + request.query.map { "\($0)=\($1)" }.joined(separator: "&"))
        let sent = request.headers[DeckRouter.setupKeyHeader] ?? DeckRouter.cookie(DeckRouter.setupCookie, in: request)
        let given = sent.map { constantTimeEqual($0, key) } ?? false
        keyAt[request.path] = given
        switch request.path {
        case "/setup": reply(.response(Response(contentType: "text/html; charset=utf-8", body: Data(SetupStub.page.utf8))))
        case "/setup/state": reply(.response(given && request.headers[DeckRouter.setupHeader] == "1" ? .json([:]) : .status(401)))
        default: reply(.response(.status(404)))
        }
    }
}
