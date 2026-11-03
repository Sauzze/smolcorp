import XCTest
@testable import AgentDeck

/// The approvals list: who waits for you, read only. And the proof that nothing
/// the GM can use answers a prompt (that is your tap alone).
final class ApprovalsTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private static func source(_ path: String) throws -> String {
        try String(contentsOf: repo.appendingPathComponent(path), encoding: .utf8)
    }

    // MARK: The list

    func testTheLabelIsTheToolAndAShortWord() {
        XCTAssertEqual(Approvals.label(tool: "Bash", input: #"{"command":"git push --force origin main"}"#), "Bash: git")
        XCTAssertEqual(Approvals.label(tool: "Bash", input: #"{"command":"TOKEN=abc curl https://x.test/?k=1"}"#), "Bash: curl", "a NAME=value set before it can hold a key")
        XCTAssertEqual(Approvals.label(tool: "Edit", input: #"{"file_path":"/work/app/Sources/Deck.swift","new_string":"x"}"#), "Edit: Deck.swift")
        XCTAssertEqual(Approvals.label(tool: "NotebookEdit", input: #"{"notebook_path":"/a/b/n.ipynb"}"#), "NotebookEdit: n.ipynb")
        XCTAssertEqual(Approvals.label(tool: "WebFetch", input: #"{"url":"https://example.com/secret"}"#), "WebFetch", "an address isn't given out")
        XCTAssertEqual(Approvals.label(tool: "Bash", input: ""), "Bash")
        XCTAssertEqual(Approvals.label(tool: "", input: ""), "")
        let long = Approvals.label(tool: "Edit", input: #"{"file_path":"/a/\#(String(repeating: "x", count: 300))"}"#)
        XCTAssertEqual(long.count, Approvals.labelLimit)
    }

    func testNoAnswerCarriesACommandAPathOrAQuestion() throws {
        let fixture = try fixture()
        let items = try items(of: fixture)
        let text = String(decoding: try JSONSerialization.data(withJSONObject: Approvals.answer(items)), as: UTF8.self)
        for secret in ["TOKEN=abc", "--force", "origin", "/work/app", "secret text", "Which database"] {
            XCTAssertFalse(text.contains(secret), secret)
        }
    }

    func testTheAnswerIsTheFixture() throws {
        let fixture = try fixture()
        let items = try items(of: fixture)
        let now = Date(timeIntervalSince1970: 1_761_000_600)
        let answer = Approvals.answer(items, now: now) as NSDictionary
        let url = FrontDeskFixtures.url("approvals.json")
        if ProcessInfo.processInfo.environment["SMOLCORP_WRITE_FIXTURES"] == "1" {
            var file = fixture
            file["answer"] = answer
            try JSONSerialization.data(withJSONObject: file, options: [.prettyPrinted, .sortedKeys]).write(to: url)
        }
        XCTAssertEqual(answer, try XCTUnwrap(fixture["answer"] as? NSDictionary), "approvals.json is out of date: run once with SMOLCORP_WRITE_FIXTURES=1")
        let list = try XCTUnwrap(answer["approvals"] as? [[String: Any]])
        XCTAssertEqual(list.map { $0["session"] as? String }, ["s1", "s2", "s3"], "the longest waiting first, the unknown last")
    }

    func testOnlyAPairedPhoneOnTheTailnetReadsTheList() {
        func request(host: String, remote: String, loopback: Bool, headers: [String: String] = [:]) -> Request {
            Request.addressed(method: "GET", path: "/approvals", query: ["t": "k"], headers: ["host": host].merging(headers) { $1 }, body: Data(),
                              fromLoopback: loopback, remoteAddress: remote)
        }
        let phone = PairedPhone(id: "p1", name: "iPhone app", pairedAt: Date(), lastSeen: Date(), keyHash: "h")
        let tailnet = request(host: "my-mac.tail1234.ts.net", remote: "127.0.0.1", loopback: true,
                              headers: ["x-forwarded-for": "100.101.102.103", "x-forwarded-proto": "https", "x-forwarded-host": "my-mac.tail1234.ts.net"])
        XCTAssertTrue(DeckRouter.showsApprovals(tailnet, phone: phone))
        XCTAssertFalse(DeckRouter.showsApprovals(tailnet, phone: .officeWindow), "not the Mac's own window")
        XCTAssertFalse(DeckRouter.showsApprovals(request(host: "192.168.1.4:7710", remote: "192.168.1.9", loopback: false), phone: phone), "not the Wi-Fi")
        XCTAssertFalse(DeckRouter.showsApprovals(request(host: "localhost:7710", remote: "127.0.0.1", loopback: true), phone: phone), "not loopback without the proxy")
    }

    // MARK: G07: nothing the GM can use answers a prompt

    /// What answers a waiting prompt: the paired phone's /act (its keys and options), /choose (a typed answer)
    /// and /dictate (its words, typed into the tab).
    private static let answerRoutes = ["/act", "/choose", "/dictate"]

    func testNoRouteTheGMCanUseIsAnAnswerRoute() {
        for row in SetupRoutes.table where row.callers.contains(.gm) {
            for answer in Self.answerRoutes {
                XCTAssertFalse(row.path == answer || row.path.hasPrefix(answer + "/"), "\(row.method) \(row.path) is an answer route")
                XCTAssertFalse(SetupRoutes.row(row.method == "*" ? "POST" : row.method, answer).map { $0 == row } ?? false)
            }
        }
        // They aren't setup routes at all, so the setup gate (and the GM's token) never reaches them.
        for answer in Self.answerRoutes { XCTAssertFalse(DeckRouter.isSetupPath(answer), answer) }
        // The list itself is a read for the GM, and only a read.
        let row = SetupRoutes.row("GET", "/setup/gm/approvals")
        XCTAssertEqual(row?.kind, "read")
        XCTAssertNil(SetupRoutes.row("POST", "/setup/gm/approvals").flatMap { $0.path == "/setup/gm/approvals" ? $0 : nil }, "no POST of its own")
        XCTAssertEqual(SetupRoutes.row("POST", "/setup/gm/approvals")?.kind, "action", "a POST falls to the catch-all, which no tool may use")
        XCTAssertFalse(SetupRoutes.allows(.tool, "POST", "/setup/gm/approvals"))
    }

    func testTheAnswerRoutesRejectTheGMToken() throws {
        // The GM's token is not a phone's key: the deck's phone lookup finds nobody, so /act, /choose and /dictate answer 401.
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("approvals-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: file, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: file) }
        let phones = Phones(file: file.appendingPathComponent("phones.json"))
        let key = phones.pair(name: "iPhone app")
        let identity = GMIdentity(tokenFile: file.appendingPathComponent("gm-token"))
        let token = try XCTUnwrap(identity.token())
        XCTAssertNotNil(phones.phone(withKey: key), "a paired phone's key is a phone")
        XCTAssertNil(phones.phone(withKey: token), "the GM's token is nobody's phone")
        XCTAssertNil(phones.phone(withKey: nil))
        // The router reads the key from `t` or the hook's token header only: the GM's header is not read at all.
        let router = try Self.source("Sources/AgentDeck/DeckRouter.swift")
        XCTAssertTrue(router.contains(#"let key = request.query["t"] ?? request.headers["x-agentdeck-token"]"#))
        let keyPart = try XCTUnwrap(router.range(of: #"let key = request.query["t"]"#))
        let afterKey = router[keyPart.lowerBound...]
        let phoneGate = try XCTUnwrap(afterKey.range(of: "guard let phone else { return respond(.status(401)) }"))
        XCTAssertFalse(afterKey[..<phoneGate.lowerBound].contains("isGMToken"), "the GM's token doesn't make a phone")
        for route in Self.answerRoutes {
            let at = try XCTUnwrap(afterKey.range(of: #"case ("POST", "\#(route)"):"#), route)
            XCTAssertLessThan(phoneGate.lowerBound, at.lowerBound, "\(route) is answered only past the phone gate")
        }
    }

    func testNoSetupRouteReachesTheCodeThatAnswers() throws {
        // The setup routes (the GM's included) never call what /act, /choose and /dictate call.
        let router = try Self.source("Sources/AgentDeck/DeckRouter.swift")
        let start = try XCTUnwrap(router.range(of: "private func setup(_ request: Request, caller:"))
        let setup = router[start.lowerBound...]
        for call in ["deck.perform(", "deck.answerOther(", "typePhoneDictation(", "deck.choose(", ".press(",
                     // The mail typing helpers (T402) answer a prompt just as well.
                     "MailDelivery.", ".paste(", ".submit(", "pasteState("] {
            XCTAssertFalse(setup.contains(call), "setup calls \(call)")
        }
        // And the list's own code answers nothing.
        let list = try Self.source("Sources/AgentDeck/Approvals.swift")
        for call in ["perform(", "answerOther(", "typePhoneDictation(", "press(", "type("] {
            XCTAssertFalse(list.contains(call), "Approvals calls \(call)")
        }
    }

    // MARK: The deck's "Waiting for you" list

    /// The page lists what waits, answers only with the deck's own /act, and hides itself when the route isn't there.
    func testTheDeckPageListsTheWaitsWithItsOwnAnswerKeys() throws {
        let page = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/deck.html"), encoding: .utf8)
        XCTAssertTrue(page.contains("Waiting for you"))
        let list = try XCTUnwrap(page.range(of: "async function checkWaiting() {"))
        let check = String(page[list.upperBound...].prefix(900))
        XCTAssertTrue(check.contains("get('/approvals')"))
        // Older Mac, Linux/Windows app or the Wi-Fi's address: not ok means hidden, and not asked again.
        XCTAssertTrue(check.contains("if (!res.ok) {"))
        XCTAssertTrue(check.contains("$('waiting').hidden = true;"))
        XCTAssertTrue(check.contains("clearInterval(waitingTimer);"))
        // The answer is 1.23's /act with accept or reject; no new path.
        XCTAssertTrue(page.contains("await send('/act', { session: item.session, action });"))
        XCTAssertTrue(page.contains("[['reject', 'No'], ['accept', 'Yes']]"))
        XCTAssertFalse(page.contains("/setup/gm"), "the deck never goes through the GM's routes")
        // A menu is answered at its desk.
        XCTAssertTrue(page.contains("selected = item.session; render();"))
    }

    /// The office tags a waiting desk, and the viewer's copy of the office is the same file.
    func testTheOfficeTagsAWaitingDeskInBothCopies() throws {
        let web = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/office.js"), encoding: .utf8)
        let viewer = try String(contentsOf: Self.repo.appendingPathComponent("viewer/public/office.js"), encoding: .utf8)
        XCTAssertTrue(web.contains("s.session.state === 'asking' && !DESKS[desk].off"))
        XCTAssertTrue(web.contains("'WAITING'"))
        XCTAssertEqual(web, viewer)
    }

    // MARK: The fixture

    private func fixture() throws -> [String: Any] {
        try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: FrontDeskFixtures.url("approvals.json"))) as? [String: Any])
    }

    private func items(of fixture: [String: Any]) throws -> [Approvals.Item] {
        try XCTUnwrap(fixture["items"] as? [[String: Any]]).map { json in
            Approvals.Item(session: json["session"] as? String ?? "", hire: json["hire"] as? String ?? "", name: json["name"] as? String ?? "",
                           nickname: json["nickname"] as? String ?? "", agent: json["agent"] as? String ?? "", floor: json["floor"] as? Int,
                           desk: json["desk"] as? Int, kind: json["kind"] as? String ?? "permission", tool: json["tool"] as? String ?? "",
                           input: json["input"] as? String ?? "", since: json["since"] as? Double ?? 0, notified: json["notified"] as? Bool ?? false)
        }
    }
}
