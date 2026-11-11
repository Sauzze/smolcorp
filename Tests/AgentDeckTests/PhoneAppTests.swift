import Network
import XCTest
@testable import AgentDeck

/// The iPhone app (personal build): how setup names it, how the deck says
/// it's on the Wi-Fi, and what `/glossary` gives it. Offline. It's built only
/// with `-Xswiftc -DIPHONE_APP` (M6): `swift test -Xswiftc -DIPHONE_APP` runs
/// its tests; a plain `swift test` checks that a public build has none of it.
final class PhoneAppTests: XCTestCase {
    #if IPHONE_APP
    func testSetupNamesTheAppApartFromSafari() {
        XCTAssertEqual(Phones.name(forUserAgent: "Smolcorp iOS"), "iPhone app")
        XCTAssertEqual(Phones.name(forUserAgent: "Smolcorp iOS CFNetwork/3860 Darwin/27.2.0"), "iPhone app")
        XCTAssertEqual(Phones.name(forUserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 27_2 like Mac OS X)"), "iPhone")
    }

    func testTheDeckIsAdvertisedAsSmolcorpWithItsAPIOnly() {
        let service = PhoneApp.service
        XCTAssertEqual(service.type, "_smolcorp._tcp")
        guard let txt = service.txtRecordObject else { return XCTFail("no TXT record") }
        XCTAssertEqual(txt.dictionary, ["api": String(PhoneApp.api)])
    }

    func testTheGlossaryIsSetupsThenTheDesks() {
        let answer = PhoneApp.glossary(desks: ["moonbase", "Smolcorp", "new-project"])
        let names = answer["glossary"] as? [String] ?? []
        XCTAssertEqual(Array(names.prefix(Config.shared.dictationGlossary.count)), Config.shared.dictationGlossary)
        XCTAssertTrue(names.contains("new-project"))
        // Already there, whatever its case: not twice.
        XCTAssertEqual(names.filter { $0.lowercased() == "smolcorp" }.count, 1)
        XCTAssertEqual(answer["language"] as? String, Config.shared.dictationLanguage)
    }
    #else
    /// A public build has no name of its own for the app: it's a phone like any.
    func testAPublicBuildNamesTheAppAsAnyPhone() {
        XCTAssertEqual(Phones.name(forUserAgent: "Smolcorp iOS CFNetwork/3860 Darwin/27.2.0"), "Phone")
        XCTAssertEqual(Phones.name(forUserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 27_2 like Mac OS X)"), "iPhone")
    }

    /// No web page asks the iPhone app's routes, and a public build answers none of them: 404, as the Linux/Windows
    /// app does. Asked from the office's own window, which the personal build answers 200 on `/glossary` and
    /// `GET /notifications`, and of setup's `/setup/notify` with the page's cookie (400 there, for no title).
    func testAPublicBuildAnswersNoneOfTheIPhoneAppsRoutes() throws {
        let deck = Deck(quiet: true)
        let router = DeckRouter(deck: deck)
        let setup = deck.access.setupSessions
        let cookie = "\(DeckRouter.setupCookie)=\(try XCTUnwrap(setup.open(link: setup.newLink())))"
        for (method, path) in [("GET", "/glossary"), ("GET", "/notifications"), ("POST", "/notifications"), ("GET", "/reviews"),
                               ("POST", "/setup/notify")] {
            let office = !path.hasPrefix("/setup")
            let headers = office ? ["host": "localhost:7710"] : [DeckRouter.setupHeader: "1", "host": "127.0.0.1:7710", "cookie": cookie]
            var got: Response?
            let answered = expectation(description: path)
            router.handle(Request.addressed(method: method, path: path, query: office ? ["t": deck.windowKey] : [:], headers: headers,
                                            body: Data("{}".utf8), fromLoopback: true, remoteAddress: "127.0.0.1")) { reply in
                if case .response(let response) = reply { got = response }
                answered.fulfill()
            }
            wait(for: [answered], timeout: 10)
            XCTAssertEqual(try XCTUnwrap(got, path).status, 404, "\(method) \(path)")
        }
    }
    #endif
}
