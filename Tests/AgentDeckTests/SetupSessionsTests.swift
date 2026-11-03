import XCTest
@testable import AgentDeck

/// Who may pair a phone: only the setup page the menu opened, by its session
/// cookie; never a tool with the setup-key file, which a hire or the GM can
/// read (it could pair a fake phone and answer a prompt).
final class SetupSessionsTests: XCTestCase {
    private let fileKey = "the-setup-key-file"
    private func isSetupKey(_ key: String?) -> Bool { key == fileKey }

    func testAHeaderKeyCallerIsAToolAndIsNeverShownTheCode() {
        let sessions = SetupSessions()
        let caller = sessions.caller(header: fileKey, cookie: nil, isSetupKey: isSetupKey)
        XCTAssertEqual(caller, .tool)
        let pairing = SetupSessions.pairing(code: "123456", expiresIn: 280, address: "http://mac.local:7710", for: caller)
        XCTAssertNil(pairing["code"])
        XCTAssertNil(pairing["codeExpiresIn"])
        XCTAssertEqual(pairing["open"] as? Bool, true, "whether pairing is open, and nothing more")
        XCTAssertEqual(SetupSessions.pairing(code: nil, expiresIn: 0, address: "", for: .tool)["open"] as? Bool, false)
    }

    func testTheMenusPageGetsASessionThatIsShownTheCode() throws {
        let sessions = SetupSessions()
        let cookie = try XCTUnwrap(sessions.open(link: sessions.newLink()))
        let caller = sessions.caller(header: nil, cookie: cookie, isSetupKey: isSetupKey)
        XCTAssertEqual(caller, .page)
        let pairing = SetupSessions.pairing(code: "123456", expiresIn: 280, address: "http://mac.local:7710", for: caller)
        XCTAssertEqual(pairing["code"] as? String, "123456")
        XCTAssertEqual(pairing["codeExpiresIn"] as? Int, 280)
        // A page with a session and a tool's header too is still the page.
        XCTAssertEqual(sessions.caller(header: fileKey, cookie: cookie, isSetupKey: isSetupKey), .page)
    }

    func testTheFileKeyNeverMakesAPageSession() {
        let sessions = SetupSessions()
        XCTAssertNil(sessions.open(link: fileKey), "the file's key opens no session")
        XCTAssertEqual(sessions.caller(header: nil, cookie: fileKey, isSetupKey: isSetupKey), .nobody, "nor passes as a cookie")
        XCTAssertEqual(sessions.caller(header: nil, cookie: nil, isSetupKey: isSetupKey), .nobody)
        XCTAssertEqual(sessions.caller(header: "wrong", cookie: "wrong", isSetupKey: isSetupKey), .nobody)
    }

    /// A request is one caller, in a fixed order: a valid setup key wins over the GM's token.
    func testAGoodKeyWithAGoodTokenIsTheToolNotTheGM() {
        let sessions = SetupSessions()
        let isGM: (String?) -> Bool = { $0 == "gm-token" }
        XCTAssertEqual(sessions.caller(header: fileKey, cookie: nil, isSetupKey: isSetupKey, gmHeader: "gm-token", isGMToken: isGM), .tool)
        XCTAssertEqual(sessions.caller(header: fileKey, cookie: nil, isSetupKey: isSetupKey, gmHeader: "wrong", isGMToken: isGM), .tool, "as the key alone")
        XCTAssertEqual(sessions.caller(header: "wrong", cookie: nil, isSetupKey: isSetupKey, gmHeader: "gm-token", isGMToken: isGM), .gm, "as the token alone")
        XCTAssertEqual(sessions.caller(header: nil, cookie: nil, isSetupKey: isSetupKey, gmHeader: "gm-token", isGMToken: isGM), .gm)
    }

    func testStateAnswersARequestWithBothTheKeyAndTheToken() {
        func gate(key: String?, token: String?) -> Int? {
            var headers: [String: String] = [:]
            if let key { headers[DeckRouter.setupKeyHeader] = key }
            if let token { headers[GMIdentity.header] = token }
            return DeckRouter.setupGate(Request(method: "GET", path: "/setup/state", query: [:], headers: headers, body: Data(), isLocal: true,
                                                isAddressedDirectly: true, remoteAddress: "127.0.0.1"),
                                        sessions: SetupSessions(), isSetupKey: { $0 == self.fileKey }, isGMToken: { $0 == "gm-token" }).refusal?.status
        }
        XCTAssertNil(gate(key: fileKey, token: "gm-token"), "both valid: the tool's read")
        XCTAssertNil(gate(key: fileKey, token: "wrong"), "key alone")
        XCTAssertEqual(gate(key: "wrong", token: "gm-token"), 403, "token alone, as before")
        XCTAssertEqual(gate(key: "wrong", token: "wrong"), 401)
    }

    func testAMenuLinkWorksOnceAndNotForLong() {
        let sessions = SetupSessions()
        let link = sessions.newLink()
        XCTAssertNotNil(sessions.open(link: link))
        XCTAssertNil(sessions.open(link: link), "the link left in the browser's history opens nothing")
        let now = Date()
        let late = sessions.newLink(now: now)
        XCTAssertNil(sessions.open(link: late, now: now.addingTimeInterval(SetupSessions.linkLifetime + 1)))
        XCTAssertNil(sessions.open(link: ""))
        XCTAssertNil(sessions.open(link: nil))
    }

    func testOnlySoManyLinksAndSessionsAreKept() {
        let sessions = SetupSessions()
        let first = sessions.newLink()
        for _ in 0..<SetupSessions.most { _ = sessions.newLink() }
        XCTAssertNil(sessions.open(link: first), "the oldest unopened link goes")
        let cookies = (0...SetupSessions.most).compactMap { _ in sessions.open(link: sessions.newLink()) }
        XCTAssertFalse(sessions.isSession(cookies[0]))
        XCTAssertTrue(sessions.isSession(cookies.last))
    }

    /// POST /setup/session: the cookie is the page's, with a random value
    /// that isn't the setup key; a link works once; the file's key opens nothing.
    func testTheSessionRouteSetsAPageCookieOnce() throws {
        let sessions = SetupSessions()
        let link = sessions.newLink()
        let first = DeckRouter.sessionReply(link: link, sessions: sessions)
        XCTAssertEqual(first.status, 204)
        let cookie = try XCTUnwrap(first.headers["Set-Cookie"])
        XCTAssertTrue(cookie.contains("HttpOnly"))
        XCTAssertTrue(cookie.contains("SameSite=Strict"))
        XCTAssertTrue(cookie.contains("Path=/setup"))
        let value = try XCTUnwrap(cookie.split(separator: ";").first?.split(separator: "=", maxSplits: 1).last.map(String.init))
        XCTAssertNotEqual(value, fileKey)
        XCTAssertNotEqual(value, link)
        XCTAssertTrue(sessions.isSession(value))
        XCTAssertEqual(DeckRouter.sessionReply(link: link, sessions: sessions).status, 401, "the same link twice")
        XCTAssertEqual(DeckRouter.sessionReply(link: fileKey, sessions: sessions).status, 401, "the setup-key file's key")
        XCTAssertEqual(DeckRouter.sessionReply(link: nil, sessions: sessions).status, 401)
    }

    // MARK: The gate the app runs

    private func asking(_ method: String, _ path: String, headers: [String: String] = [:], local: Bool = true) -> Request {
        Request(method: method, path: path, query: [:], headers: headers, body: Data(), isLocal: local, isAddressedDirectly: true,
                remoteAddress: "127.0.0.1")
    }

    func testTheSetupGate() throws {
        let sessions = SetupSessions()
        let session = try XCTUnwrap(sessions.open(link: sessions.newLink()))
        let gate = { (request: Request) in DeckRouter.setupGate(request, sessions: sessions, isSetupKey: { $0 == self.fileKey }) }
        let action = [DeckRouter.setupHeader: "1"]
        let asPage = action.merging(["cookie": "\(DeckRouter.setupCookie)=\(session)"]) { $1 }
        let asTool = action.merging([DeckRouter.setupKeyHeader: fileKey]) { $1 }
        XCTAssertEqual(gate(asking("GET", "/setup/state", local: false)).refusal?.status, 404, "this Mac only")
        XCTAssertEqual(gate(asking("POST", "/setup/pair", headers: [DeckRouter.setupKeyHeader: fileKey])).refusal?.status, 403, "an action needs the header")
        XCTAssertEqual(gate(asking("GET", "/setup/state")).refusal?.status, 401)
        XCTAssertEqual(gate(asking("GET", "/setup/state", headers: ["cookie": "\(DeckRouter.setupCookie)=\(fileKey)"])).refusal?.status, 401,
                       "the file's key is never the cookie")
        XCTAssertEqual(gate(asking("POST", "/setup/pair", headers: action.merging(["cookie": "\(DeckRouter.setupCookie)=\(fileKey)"]) { $1 })).refusal?.status, 401)
        XCTAssertEqual(gate(asking("GET", "/setup/state", headers: asTool)).caller, .tool)
        XCTAssertNil(gate(asking("GET", "/setup/state", headers: asTool)).refusal, "a tool reads setup")
        XCTAssertNil(gate(asking("POST", "/setup/theme", headers: asTool)).refusal, "and changes it")
        XCTAssertNil(gate(asking("POST", "/bots/one/event", headers: asTool)).refusal, "/bots/ keeps its tool path")
        XCTAssertEqual(gate(asking("POST", "/bots/one/event", headers: action)).refusal?.status, 401)
        XCTAssertEqual(gate(asking("POST", "/setup/pair", headers: asPage)).caller, .page)
        XCTAssertNil(gate(asking("POST", "/setup/pair", headers: asPage)).refusal)
        XCTAssertNil(gate(asking("GET", "/setup")).refusal, "the page itself holds nothing")
        XCTAssertNil(gate(asking("POST", "/setup/session", headers: action)).refusal, "where a link's key is handed over")
    }

    /// Every route that shows or makes a phone's code or key: the page's, never a tool's. (Taking a phone away
    /// is the page's or the GM's: see SetupRoutesTests.)
    func testPairingRoutesArePageOnly() throws {
        let sessions = SetupSessions()
        let session = try XCTUnwrap(sessions.open(link: sessions.newLink()))
        let action = [DeckRouter.setupHeader: "1"]
        let asPage = action.merging(["cookie": "\(DeckRouter.setupCookie)=\(session)"]) { $1 }
        let asTool = action.merging([DeckRouter.setupKeyHeader: fileKey]) { $1 }
        let routes = [("POST", "/setup/pair"), ("GET", "/setup/qr.png"),
                      ("GET", "/setup/viewer/qr.png"), ("POST", "/setup/viewer/secret"), ("POST", "/setup/viewer/rotate")]
        for (method, path) in routes {
            XCTAssertTrue(SetupSessions.isPageOnly(method, path), path)
            let gate = { (headers: [String: String]) in
                DeckRouter.setupGate(self.asking(method, path, headers: headers), sessions: sessions, isSetupKey: { $0 == self.fileKey })
            }
            XCTAssertEqual(gate(asTool).refusal?.status, 403, "\(path): not a tool")
            XCTAssertNil(gate(asPage).refusal, "\(path): the page")
            XCTAssertEqual(gate(action).refusal?.status, 401, "\(path): nobody")
        }
        for (method, path) in [("GET", "/setup/state"), ("GET", "/setup/viewer"), ("POST", "/setup/layout"),
                               ("POST", "/setup/hooks"), ("POST", "/setup/hiring"), ("POST", "/setup/unpair")] {
            XCTAssertFalse(SetupSessions.isPageOnly(method, path), path)
        }
    }
}
