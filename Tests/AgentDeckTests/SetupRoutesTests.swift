import XCTest
@testable import AgentDeck

/// Which callers may use which setup route, driven through the real gate, row by row.
final class SetupRoutesTests: XCTestCase {
    private let fileKey = "the-setup-key-file"
    private let gmToken = "the-gm-token"
    private let hireKey = "a-hires-send-key"
    private var sessions: SetupSessions!
    private var cookie = ""

    override func setUpWithError() throws {
        sessions = SetupSessions()
        cookie = try XCTUnwrap(sessions.open(link: sessions.newLink()))
    }

    private func headers(for caller: SetupSessions.Caller) -> [String: String] {
        var headers = [DeckRouter.setupHeader: "1"]
        switch caller {
        case .page: headers["cookie"] = "\(DeckRouter.setupCookie)=\(cookie)"
        case .tool: headers[DeckRouter.setupKeyHeader] = fileKey
        case .gm: headers[GMIdentity.header] = gmToken
        case .hire: headers[FrontDesk.sendKeyHeader] = hireKey
        case .nobody: break
        }
        return headers
    }

    private func gate(_ method: String, _ path: String, _ headers: [String: String]) -> (refusal: Response?, caller: SetupSessions.Caller) {
        let request = Request(method: method, path: path, query: [:], headers: headers, body: Data(), isLocal: true,
                              isAddressedDirectly: true, remoteAddress: "127.0.0.1")
        return DeckRouter.setupGate(request, sessions: sessions, isSetupKey: { $0 == self.fileKey }, isGMToken: { $0 == self.gmToken },
                                    isHireKey: { $0 == self.hireKey })
    }

    /// A path a row stands for: a prefix's gets something after it; a row for any method is tried as a POST.
    private func concrete(_ row: SetupRoutes.Row) -> (String, String) {
        (row.method == "*" ? "POST" : row.method, row.path.hasSuffix("/*") ? String(row.path.dropLast()) + "anything" : row.path)
    }

    /// A request with the setup key and the GM's token: the owner's tool for a read, the GM for an action a tool may not use.
    func testAKeyAndATokenTogetherAreTheGMForAGMActionAndTheToolForARead() {
        let both = [DeckRouter.setupHeader: "1", DeckRouter.setupKeyHeader: fileKey, GMIdentity.header: gmToken]
        let wrongToken = [DeckRouter.setupHeader: "1", DeckRouter.setupKeyHeader: fileKey, GMIdentity.header: "wrong"]
        for path in ["/setup/gm/send", "/setup/gm/hire"] {
            let result = gate("POST", path, both)
            XCTAssertNil(result.refusal, "\(path): the GM's action, not refused")
            XCTAssertEqual(result.caller, .gm, path)
            let keyAlone = gate("POST", path, wrongToken)
            XCTAssertEqual(keyAlone.caller, .tool, "\(path): a wrong token changes nothing")
            XCTAssertEqual(keyAlone.refusal?.status, 403, path)
            XCTAssertEqual(keyAlone.refusal?.body, gate("POST", path, [DeckRouter.setupHeader: "1", DeckRouter.setupKeyHeader: fileKey]).refusal?.body,
                           "\(path): the same refusal as the key alone")
        }
        let read = gate("GET", "/setup/state", both)
        XCTAssertNil(read.refusal)
        XCTAssertEqual(read.caller, .tool)
    }

    func testEveryRowOfTheTableThroughTheRealGate() {
        XCTAssertGreaterThan(SetupRoutes.table.count, 30)
        for row in SetupRoutes.table {
            let (method, path) = concrete(row)
            for caller in [SetupSessions.Caller.gm, .page, .tool, .hire] {
                let answer = gate(method, path, headers(for: caller))
                if row.callers.contains(caller) {
                    XCTAssertNil(answer.refusal, "\(method) \(path): \(caller.name) is let in")
                    XCTAssertEqual(answer.caller, caller)
                } else {
                    XCTAssertEqual(answer.refusal?.status, 403, "\(method) \(path): \(caller.name) is refused")
                    XCTAssertEqual(answer.caller, caller, "and is still who it is")
                }
            }
            XCTAssertEqual(gate(method, path, headers(for: .nobody)).refusal?.status, 401, "\(method) \(path): nobody")
            XCTAssertTrue(row.callers.contains(.page), "\(path): the page can always do what a tool can")
        }
    }

    /// T404: where a cloud bot's mail goes is chosen by the owner on the page; neither the GM nor a tool of yours may.
    func testABotsMailboxFolderIsPageOnly() {
        XCTAssertNil(gate("POST", "/setup/bots/mailbox", headers(for: .page)).refusal)
        XCTAssertEqual(gate("POST", "/setup/bots/mailbox", headers(for: .gm)).refusal?.status, 403)
        XCTAssertEqual(gate("POST", "/setup/bots/mailbox", headers(for: .tool)).refusal?.status, 403)
    }

    func testTheSetupKeyFileAloneCantDoWhatAHireCouldMisuse() {
        let misusable = ["/setup/hire", "/setup/hire/gm", "/setup/hiring", "/setup/hiring/choose", "/setup/roles", "/setup/hooks",
                         "/setup/unpair", "/setup/notify", "/setup/webhook", "/setup/webhook/test", "/setup/bots", "/setup/bots/override",
                         "/setup/bots/resolve", "/setup/maxconfirm", "/setup/phonemic", "/setup/dictation", "/setup/login",
                         "/setup/layout", "/setup/profiles", "/setup/pin", "/setup/unpin", "/setup/floors",
                         "/setup/gm/hire", "/setup/gm/fire", "/setup/gm/brief", "/setup/gm/tile", "/setup/gm/send", "/setup/gm/contacts", "/setup/gm/registry", "/setup/gm/nickname",
                         "/setup/gm/manager"]
        for path in misusable {
            XCTAssertEqual(gate("POST", path, headers(for: .tool)).refusal?.status, 403, path)
            XCTAssertNil(gate("POST", path, headers(for: .gm)).refusal, "\(path): the GM")
            XCTAssertNil(gate("POST", path, headers(for: .page)).refusal, "\(path): the page")
        }
    }

    func testAHireMaySendAndDoNothingElse() {
        XCTAssertNil(gate("POST", "/setup/gm/send", headers(for: .hire)).refusal)
        XCTAssertEqual(gate("POST", "/setup/gm/send", headers(for: .hire)).caller, .hire)
        for (method, path) in [("POST", "/setup/gm/contacts"), ("POST", "/setup/gm/hire"), ("POST", "/setup/gm/fire"), ("GET", "/setup/gm/team"),
                               ("GET", "/setup/gm/outbox"), ("POST", "/setup/gm/token/rotate"), ("POST", "/setup/hire")] {
            XCTAssertEqual(gate(method, path, headers(for: .hire)).refusal?.status, 403, "\(method) \(path)")
        }
        // The GM's token is not a hire's key, nor the key the GM's token.
        XCTAssertEqual(gate("POST", "/setup/gm/contacts", [DeckRouter.setupHeader: "1", FrontDesk.sendKeyHeader: gmToken]).refusal?.status, 401)
        XCTAssertEqual(gate("POST", "/setup/gm/send", [DeckRouter.setupHeader: "1", FrontDesk.sendKeyHeader: "wrong"]).refusal?.status, 401)
        XCTAssertEqual(gate("POST", "/setup/gm/send", [DeckRouter.setupHeader: "1", GMIdentity.header: hireKey]).refusal?.status, 401)
    }

    func testSpendingAndSendingElsewhereIsThePagesAlone() {
        for path in ["/setup/summaries", "/setup/summary/provider", "/setup/summary/key",
                     "/setup/viewer", "/setup/viewer/switch", "/setup/viewer/wipe"] {
            XCTAssertTrue(SetupSessions.isPageOnly("POST", path), path)
            XCTAssertEqual(gate("POST", path, headers(for: .tool)).refusal?.status, 403, "\(path): not a tool")
            XCTAssertEqual(gate("POST", path, headers(for: .gm)).refusal?.status, 403, "\(path): not the GM")
            XCTAssertNil(gate("POST", path, headers(for: .page)).refusal, path)
        }
        // Looking at the viewer, and asking whether an address answers, stay open.
        XCTAssertNil(gate("GET", "/setup/viewer", headers(for: .tool)).refusal)
        XCTAssertNil(gate("POST", "/setup/viewer/check", headers(for: .tool)).refusal)
    }

    func testTheRefusalSaysWhoMayDoIt() {
        XCTAssertEqual(gate("POST", "/setup/pair", headers(for: .tool)).refusal.map { String(decoding: $0.body, as: UTF8.self) },
                       "This is for the setup page the menu opens.")
        XCTAssertEqual(gate("POST", "/setup/hire", headers(for: .tool)).refusal.map { String(decoding: $0.body, as: UTF8.self) },
                       "Only the setup page or the GM can do this.")
    }

    func testAToolMayOnlyRepairAndNeverSeesTheWebhookSecret() {
        XCTAssertNil(DeckRouter.fixRefusal(caller: .tool, fix: SelfTest.repair, auto: "1"))
        // A repair without auto=1 would reconnect agents the owner disconnected.
        for auto in [nil, "0", ""] as [String?] {
            XCTAssertEqual(DeckRouter.fixRefusal(caller: .tool, fix: SelfTest.repair, auto: auto)?.status, 403, auto ?? "none")
            XCTAssertNil(DeckRouter.fixRefusal(caller: .page, fix: SelfTest.repair, auto: auto))
        }
        for fix in [SelfTest.install, SelfTest.login, SelfTest.accessibility, SelfTest.tailscale, "", nil] {
            XCTAssertEqual(DeckRouter.fixRefusal(caller: .tool, fix: fix)?.status, 403, fix ?? "none")
            XCTAssertNil(DeckRouter.fixRefusal(caller: .page, fix: fix))
        }
        XCTAssertEqual(DeckRouter.webhookSecret("s3cret", for: .page), "s3cret")
        for caller in [SetupSessions.Caller.tool, .gm, .nobody] { XCTAssertEqual(DeckRouter.webhookSecret("s3cret", for: caller), "") }
    }

    /// Every POST route under /setup/ the router answers is in the table, or on this reviewed list of what a tool
    /// with the setup-key file may still do. A new route that is in neither fails here, so someone decides.
    func testEveryPostRouteIsInTheTableOrReviewedForTools() throws {
        let allowedForTools: Set<String> = [
            "/setup/session",             // hands over a link's key for a cookie
            "/setup/viewer/check",        // only asks the server whether an address answers
            "/setup/summary/check",       // only asks whether a provider answers
            "/setup/theme", "/setup/logo", "/setup/scroll", "/setup/dock", "/setup/ontop", "/setup/music", // looks only
            "/setup/dictation/glossary",  // a list of names, nothing sent
            "/setup/accessibility",       // asks macOS for its prompt
            "/setup/office",              // shows the window
            "/setup/fix",                 // a tool may repair (see fixRefusal)
        ]
        let router = try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("Sources/AgentDeck/DeckRouter.swift"), encoding: .utf8)
        let pattern = try NSRegularExpression(pattern: #"case \("POST", "(/setup/[^"]*)"\)"#)
        let found = Set(pattern.matches(in: router, range: NSRange(router.startIndex..., in: router)).map {
            String(router[Range($0.range(at: 1), in: router)!])
        })
        XCTAssertTrue(found.contains("/setup/fix") && found.contains("/setup/layout"), "the pattern finds the routes")
        for path in found.sorted() {
            let listed = SetupRoutes.row("POST", path) != nil
            XCTAssertTrue(listed != allowedForTools.contains(path), "\(path): in the table, or reviewed for tools, not both or neither")
            if !listed { XCTAssertTrue(SetupRoutes.allows(.tool, "POST", path), path) }
        }
        for path in allowedForTools { XCTAssertNil(SetupRoutes.row("POST", path), "\(path): reviewed for tools, so not in the table") }
    }

    func testTheTableHasTheTeamReadAndTheRegistryAction() {
        let team = SetupRoutes.row("GET", "/setup/gm/team")
        XCTAssertEqual(team?.kind, "read")
        XCTAssertEqual(team?.callers, [.gm, .page, .tool])
        let registry = SetupRoutes.row("POST", "/setup/gm/registry")
        XCTAssertEqual(registry?.kind, "action")
        XCTAssertEqual(registry?.callers, [.gm, .page])
    }

    /// The registry file isn't served to the GM (it reads the team, which has the notes and state), so a refusal
    /// of GET /setup/gm/registry must not say the GM may do it.
    func testTheRegistryFileIsThePagesAndNoRefusalPointsTheGMAtIt() {
        XCTAssertEqual(SetupRoutes.row("GET", "/setup/gm/registry")?.callers, [.page])
        for caller in [SetupSessions.Caller.gm, .tool] {
            let refusal = gate("GET", "/setup/gm/registry", headers(for: caller)).refusal
            XCTAssertEqual(refusal?.status, 403, caller.name)
            XCTAssertEqual(refusal.map { String(decoding: $0.body, as: UTF8.self) }, "This is for the setup page the menu opens.", caller.name)
        }
        XCTAssertNil(gate("GET", "/setup/gm/registry", headers(for: .page)).refusal)
        // Editing it stays the GM's and the page's.
        XCTAssertEqual(SetupRoutes.row("POST", "/setup/gm/registry")?.callers, [.gm, .page])
    }

    func testTheReadRoutesAreTheGMsThePagesOrAToolsOfYours() {
        for path in ["team", "events", "version", "outbox", "approvals", "screens"] {
            for caller in [SetupSessions.Caller.gm, .page, .tool] {
                XCTAssertNil(gate("GET", "/setup/gm/\(path)", headers(for: caller)).refusal, "\(path): \(caller.name)")
            }
            // A read route is only for reading.
            XCTAssertEqual(gate("POST", "/setup/gm/\(path)", headers(for: .tool)).refusal?.status, 403, "\(path): a POST isn't a read")
        }
    }

    func testARouteUnderGMNobodyListedStartsClosed() {
        XCTAssertEqual(gate("POST", "/setup/gm/something-new", headers(for: .tool)).refusal?.status, 403)
        XCTAssertEqual(gate("GET", "/setup/gm/something-new", headers(for: .tool)).refusal?.status, 403)
        XCTAssertNil(gate("GET", "/setup/gm/something-new", headers(for: .gm)).refusal)
    }

    func testARouteNotListedKeepsOneTwentyThreesRule() {
        for path in ["/setup/state", "/setup/theme", "/setup/music", "/setup/briefs", "/setup/hire/folders"] {
            let method = path == "/setup/state" || path == "/setup/briefs" || path == "/setup/hire/folders" ? "GET" : "POST"
            XCTAssertNil(gate(method, path, headers(for: .page)).refusal, path)
            XCTAssertNil(gate(method, path, headers(for: .tool)).refusal, path)
            XCTAssertEqual(gate(method, path, headers(for: .gm)).refusal?.status, 403, "\(path): the GM's token opens only what the table gives it")
        }
    }

    func testTheGMIsOnlyWhoSendsTheTokenAndNothingPassesForIt() {
        XCTAssertEqual(gate("GET", "/setup/gm/team", [GMIdentity.header: "wrong"]).refusal?.status, 401)
        XCTAssertEqual(gate("GET", "/setup/gm/team", [GMIdentity.header: fileKey]).refusal?.status, 401, "the setup key isn't the token")
        XCTAssertEqual(gate("GET", "/setup/gm/team", [DeckRouter.setupKeyHeader: gmToken]).refusal?.status, 401, "nor the token the setup key")
        XCTAssertEqual(gate("GET", "/setup/gm/team", ["cookie": "\(DeckRouter.setupCookie)=\(gmToken)"]).refusal?.status, 401, "nor a cookie")
        XCTAssertEqual(gate("POST", "/setup/gm/hire", [DeckRouter.setupKeyHeader: fileKey, GMIdentity.header: "wrong"]).refusal?.status, 403,
                       "a wrong token beside the file's key is a tool")
        XCTAssertEqual(gate("POST", "/setup/gm/hire", headers(for: .gm).merging([DeckRouter.setupKeyHeader: fileKey]) { $1 }).caller, .gm,
                       "with both, for an action a tool may not use, the token makes it the GM")
        XCTAssertEqual(gate("POST", "/setup/gm/hire", headers(for: .gm).merging(headers(for: .page)) { $1 }).caller, .page)
        // An action needs the setup header from the GM too.
        XCTAssertEqual(gate("POST", "/setup/gm/hire", [GMIdentity.header: gmToken]).refusal?.status, 403)
        // Without a token made yet nobody is the GM, even sending nothing.
        let request = Request(method: "GET", path: "/setup/gm/team", query: [:], headers: [GMIdentity.header: ""], body: Data(), isLocal: true,
                              isAddressedDirectly: true, remoteAddress: "127.0.0.1")
        XCTAssertEqual(DeckRouter.setupGate(request, sessions: sessions, isSetupKey: { _ in false }, isGMToken: { _ in false }).refusal?.status, 401)
    }

    func testTheGMStillComesFromThisMacOnly() {
        let request = Request(method: "GET", path: "/setup/gm/team", query: [:], headers: [GMIdentity.header: gmToken], body: Data(),
                              isLocal: false, isAddressedDirectly: true, remoteAddress: "192.168.1.9")
        XCTAssertEqual(DeckRouter.setupGate(request, sessions: sessions, isSetupKey: { _ in false }, isGMToken: { $0 == self.gmToken }).refusal?.status, 404)
    }

    func testPhonesCodesAndKeysStayThePagesAlone() {
        for (method, path) in [("POST", "/setup/pair"), ("GET", "/setup/qr.png"), ("GET", "/setup/viewer/qr.png"),
                               ("POST", "/setup/viewer/secret"), ("POST", "/setup/viewer/rotate")] {
            XCTAssertTrue(SetupSessions.isPageOnly(method, path), path)
            XCTAssertEqual(gate(method, path, headers(for: .gm)).refusal?.status, 403, "\(path): not the GM")
            XCTAssertEqual(gate(method, path, headers(for: .tool)).refusal?.status, 403, "\(path): not a tool")
            XCTAssertNil(gate(method, path, headers(for: .page)).refusal)
        }
        XCTAssertFalse(SetupSessions.isPageOnly("POST", "/setup/unpair"), "taking a phone away is the page's or the GM's")
    }

    func testTheGMNeverSeesAPairingCode() {
        let pairing = SetupSessions.pairing(code: "123456", expiresIn: 200, address: "http://mac.local:7710", for: .gm)
        XCTAssertNil(pairing["code"])
        XCTAssertEqual(pairing["open"] as? Bool, true)
    }

    // MARK: Rotating

    func testRotatingThroughTheGateEndsTheOldToken() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("rotate-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let identity = GMIdentity(store: FakeGMTokenStore(), tokenFile: folder.appendingPathComponent("gm-token"), fileAllowed: { false })
        let old = try XCTUnwrap(identity.token())
        let ask = { (headers: [String: String]) in
            let request = Request(method: "POST", path: "/setup/gm/token/rotate", query: [:], headers: headers, body: Data(), isLocal: true,
                                  isAddressedDirectly: true, remoteAddress: "127.0.0.1")
            return DeckRouter.setupGate(request, sessions: self.sessions, isSetupKey: { $0 == self.fileKey }, isGMToken: identity.isToken)
        }
        let asGM = [DeckRouter.setupHeader: "1", GMIdentity.header: old]
        XCTAssertEqual(ask(headers(for: .tool)).refusal?.status, 403, "the setup-key file alone can't rotate it")
        XCTAssertEqual(ask(asGM).refusal?.status, 403, "the GM can't rotate its own token")
        XCTAssertNil(ask(headers(for: .page)).refusal)
        let fresh = try XCTUnwrap(identity.rotate())
        XCTAssertEqual(ask(asGM).refusal?.status, 401, "the old token is nobody now")
        XCTAssertEqual(ask([DeckRouter.setupHeader: "1", GMIdentity.header: fresh]).refusal?.status, 403, "and the new one is the GM's, still not allowed")
        XCTAssertNil(ask(headers(for: .page)).refusal)
    }

    func testRotatingIsForThePageOnlyAndNoAnswerCarriesTheToken() throws {
        XCTAssertEqual(SetupRoutes.row("POST", "/setup/gm/token/rotate")?.callers, [.page])
        XCTAssertFalse(SetupRoutes.allows(.gm, "POST", "/setup/gm/token/rotate"))
        XCTAssertFalse(SetupRoutes.allows(.tool, "POST", "/setup/gm/token/rotate"))
        let answer = try fixture("gm-token-rotate-as-page.json")
        XCTAssertEqual(DeckRouter.rotated() as NSDictionary, answer as NSDictionary)
        XCTAssertNil(answer["token"])
        XCTAssertTrue((answer["message"] as? String)?.contains("hire it again") == true)
    }

    // MARK: The table as a fixture, and the router

    private func fixture(_ name: String) throws -> [String: Any] {
        try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: FrontDeskFixtures.url(name))) as? [String: Any])
    }

    /// `routes.json` is the table. Set SMOLCORP_WRITE_FIXTURES=1 for a run to write it again after the table changes.
    func testTheFixtureIsTheTable() throws {
        let url = FrontDeskFixtures.url("routes.json")
        if ProcessInfo.processInfo.environment["SMOLCORP_WRITE_FIXTURES"] == "1" {
            try SetupRoutes.fixture().write(to: url)
        }
        let written = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? NSDictionary)
        let table = try XCTUnwrap(JSONSerialization.jsonObject(with: SetupRoutes.fixture()) as? NSDictionary)
        XCTAssertEqual(written, table, "routes.json is out of date: run once with SMOLCORP_WRITE_FIXTURES=1")
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

    /// Every /setup/gm route the router answers is in the table by name, so none is only on the catch-all.
    func testEveryGMRouteTheRouterAnswersIsInTheTable() throws {
        let router = try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("Sources/AgentDeck/DeckRouter.swift"), encoding: .utf8)
        let pattern = try NSRegularExpression(pattern: #"case \("([A-Z]+)", "(/setup/gm/[^"]*)"\)"#)
        let found = pattern.matches(in: router, range: NSRange(router.startIndex..., in: router)).map {
            (String(router[Range($0.range(at: 1), in: router)!]), String(router[Range($0.range(at: 2), in: router)!]))
        }
        XCTAssertTrue(found.contains { $0.1 == "/setup/gm/token/rotate" })
        for (method, path) in found {
            XCTAssertTrue(SetupRoutes.table.contains { $0.path == path && ($0.method == method || $0.method == "*") }, "\(method) \(path) is not in the table")
        }
    }
}
