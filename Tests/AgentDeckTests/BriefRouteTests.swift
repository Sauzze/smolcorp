import XCTest
@testable import AgentDeck

/// Setup's brief and hire routes: what each answers, and that only setup's
/// own requests get in, tried on a stand-in server on a spare port (never
/// the app's own).
final class BriefRouteTests: XCTestCase {
    private let library = BriefLibrary(folder: URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        .deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("Resources/briefs"))
    /// Stands in for your home folder.
    private var home: URL!

    override func setUpWithError() throws {
        home = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
            .resolvingSymlinksInPath()
        for folder in ["Projects/app/.team/hire", "Library/Keys", ".ssh"] {
            try FileManager.default.createDirectory(at: home.appendingPathComponent(folder), withIntermediateDirectories: true)
        }
        try "".write(to: home.appendingPathComponent("Projects/notes.md"), atomically: true, encoding: .utf8)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: home)
    }

    private func body(_ object: Any) -> Data { try! JSONSerialization.data(withJSONObject: object) }
    private func json(_ response: Response) -> [String: Any] {
        (try? JSONSerialization.jsonObject(with: response.body)) as? [String: Any] ?? [:]
    }
    private func text(_ response: Response) -> String { String(decoding: response.body, as: UTF8.self) }

    // MARK: Each route

    func testTheLibraryIsListed() throws {
        let briefs = try XCTUnwrap(json(BriefRoutes.list(library))["briefs"] as? [[String: Any]])
        XCTAssertEqual(briefs.map { $0["id"] as? String }, Brief.libraryIDs)
        let janitor = try XCTUnwrap(briefs.first { $0["id"] as? String == "janitor" })
        XCTAssertEqual(janitor["name"] as? String, "Janitor")
        XCTAssertTrue((janitor["summary"] as? String)?.contains("Deletes nothing until you say which") == true)
    }

    func testABriefIsGivenWhole() {
        let found = BriefRoutes.markdown("janitor", library)
        XCTAssertEqual(found.status, 200)
        let markdown = json(found)["markdown"] as? String ?? ""
        XCTAssertTrue(markdown.hasPrefix("---\nname: Janitor\n"))
        XCTAssertTrue(markdown.contains("## Never") && markdown.contains("{{job}}"))
        XCTAssertNoThrow(try Brief.file(markdown, named: "janitor.md"), "a library brief can be picked as a file as it is")
        for id in ["nope", "../Info", "_base", ""] { XCTAssertEqual(BriefRoutes.markdown(id, library).status, 404, id) }
    }

    func testAPreviewIsTheBriefAHireWouldHave() throws {
        let roles = Role.checked(Role.shipped)
        let folder = home.appendingPathComponent("Projects/app").path
        let reviewer = BriefRoutes.preview(body(["role": "reviewer", "answers": ["feature-x", "main", "bugs"], "folder": folder]),
                                           roles: roles, library: library, hireFolder: "", pins: [nil, folder])
        XCTAssertEqual(reviewer.status, 200)
        XCTAssertEqual(json(reviewer)["kind"] as? String, "library")
        let markdown = json(reviewer)["markdown"] as? String ?? ""
        XCTAssertTrue(markdown.contains("** feature-x\n") && markdown.contains("`\(folder)/STATUS.md`"))
        XCTAssertEqual(json(reviewer)["firstMessage"] as? String, roles[0].briefMessage(at: "\(folder)/BRIEF.md", asks: false))
        // Without answers, the questions the agent asks first; at a desk not pinned there, its brief is kept by Smolcorp.
        let kept = home.appendingPathComponent("Support/briefs")
        let asked = json(BriefRoutes.preview(body(["role": "reviewer", "folder": folder]), roles: roles, library: library, hireFolder: "",
                                             kept: kept))
        XCTAssertTrue((asked["markdown"] as? String)?.contains("Before anything else, ask the user") == true)
        XCTAssertTrue((asked["markdown"] as? String)?.contains("`\(kept.path)/desk-reviewer-") == true)
        XCTAssertTrue((asked["firstMessage"] as? String)?.hasPrefix("Read `\(kept.path)/desk-reviewer-") == true)
        // A role as the page has it, not saved yet, with a picked file.
        var own = Role(id: "own-1", name: "Mine", agent: "claude", model: "", effort: "", questions: ["What?"], instructions: "", badge: "star")
        own.brief = Brief(kind: .file, text: "# {{role}}: {{job}}", fileName: "mine.md")
        let picked = BriefRoutes.preview(body(["role": own.jsonObject, "answers": ["this"]]), roles: roles, library: library, hireFolder: folder)
        XCTAssertEqual(json(picked)["kind"] as? String, "file")
        XCTAssertTrue((json(picked)["markdown"] as? String)?.hasPrefix("# Mine: The user answered") == true)
        // A role on its own instructions: what its agent is first told.
        var text = own
        text.brief = .text("Read the licences.")
        let lawyer = json(BriefRoutes.preview(body(["role": text.jsonObject, "answers": ["the licence"]]), roles: roles, library: library,
                                              hireFolder: folder))
        XCTAssertEqual(lawyer["kind"] as? String, "text")
        text.instructions = "Read the licences."
        XCTAssertEqual(lawyer["markdown"] as? String, text.firstMessage(in: folder, answers: ["the licence"]))
    }

    func testAPreviewSaysWhatsWrong() {
        let roles = Role.checked(Role.shipped)
        let preview = { (object: Any) in BriefRoutes.preview(self.body(object), roles: roles, library: self.library, hireFolder: "") }
        XCTAssertEqual(preview(["role": "nope"]).status, 404)
        XCTAssertEqual(preview(["answers": []]).status, 400)
        XCTAssertEqual(preview(["role": "reviewer", "answers": "feature-x"]).status, 400)
        XCTAssertEqual(preview(["role": "reviewer", "answers": [1, 2]]).status, 400)
        XCTAssertEqual(preview(["role": "reviewer", "answers": Array(repeating: "a", count: Role.questionsMost + 1)]).status, 400)
        XCTAssertEqual(BriefRoutes.preview(Data("not json".utf8), roles: roles, library: library, hireFolder: "").status, 400)
        let bad = preview(["role": ["id": "own-1", "name": "Mine", "brief": ["kind": "file", "text": "{{secret}}", "fileName": "a.md"]]])
        XCTAssertEqual(bad.status, 400)
        XCTAssertTrue(text(bad).hasPrefix("Mine: The brief has placeholders Smolcorp can't fill: {{secret}}"))
    }

    func testRolesAreSavedWithTheirBriefsOrRefusedSayingWhy() throws {
        var own = Role(id: "own-1", name: "Mine", agent: "claude", model: "", effort: "", instructions: "", badge: "star")
        own.brief = Brief(kind: .file, text: "# Mine", fileName: "mine.md")
        var library = Role.shipped[0]
        library.brief = .library("security-auditor")
        let saved = try BriefRoutes.roles(body([own.jsonObject, library.jsonObject]), saved: Role.shipped).get()
        XCTAssertEqual(saved.map(\.brief), [Brief(kind: .file, text: "# Mine", fileName: "mine.md"), .library("security-auditor")])
        own.brief = Brief(kind: .file, text: String(repeating: "x", count: Brief.fileLimit + 1), fileName: "mine.md")
        XCTAssertThrowsError(try BriefRoutes.roles(body([own.jsonObject]), saved: []).get()) {
            XCTAssertEqual(($0 as? BriefRoutes.Refusal)?.response.status, 400)
            XCTAssertEqual(($0 as? BriefRoutes.Refusal).map { self.text($0.response) }, "Mine: The brief is over 32 KB. Pick a shorter file.")
        }
        own.brief = .library("nope")
        XCTAssertThrowsError(try BriefRoutes.roles(body([own.jsonObject]), saved: []).get())
        XCTAssertThrowsError(try BriefRoutes.roles(Data("{}".utf8), saved: []).get())
    }

    func testOnlyAFolderInsideYourHomeCanBeHiredInto() throws {
        let checked = { (path: String) in try? BriefRoutes.checkedFolder(path, home: self.home).get() }
        let app = home.appendingPathComponent("Projects/app").path
        XCTAssertEqual(checked(app), app)
        XCTAssertEqual(checked(app + "/"), app)
        XCTAssertEqual(checked("  \(app)/.team/hire "), app + "/.team/hire", "a hidden folder further in is fine")
        XCTAssertEqual(checked(home.appendingPathComponent("Projects/app/../app").path), app)
        try FileManager.default.createSymbolicLink(atPath: home.appendingPathComponent("Projects/out").path, withDestinationPath: "/usr")
        try FileManager.default.createSymbolicLink(atPath: home.appendingPathComponent("Projects/in").path, withDestinationPath: app)
        XCTAssertEqual(checked(home.appendingPathComponent("Projects/in").path), app)
        for refused in [home.path, home.path + "/", home.appendingPathComponent("Projects/out").path, home.appendingPathComponent("Projects/../..").path,
                        home.appendingPathComponent("Library/Keys").path, home.appendingPathComponent(".ssh").path,
                        home.appendingPathComponent("Projects/notes.md").path, home.appendingPathComponent("Projects/missing").path,
                        "/usr", "/etc", "/System", "/", "Projects/app", "", "~"] {
            XCTAssertNil(checked(refused), refused)
        }
    }

    func testAHireIsCheckedBeforeItsHired() throws {
        let roles = Role.checked(Role.shipped)
        let app = home.appendingPathComponent("Projects/app").path
        let request = { (object: Any) in BriefRoutes.hireRequest(self.body(object), roles: roles, home: self.home) }
        XCTAssertEqual(try request(["role": "janitor", "folder": app, "answers": ["the importer", "public APIs"]]).get(),
                       BriefRoutes.HireRequest(role: "janitor", folder: app, answers: ["the importer", "public APIs"]))
        XCTAssertEqual(try request(["role": "janitor", "folder": app]).get().answers, [])
        let refusals: [([String: Any], Int, String)] = [
            (["folder": app], 400, "Say which role to hire."),
            (["role": "nope", "folder": app], 404, "That role isn't there any more. Pick another."),
            (["role": "janitor"], 400, "Say which folder to hire into."),
            (["role": "janitor", "folder": "/etc"], 400, "Hire into a folder inside your home folder."),
            (["role": "janitor", "folder": app, "answers": "x"], 400, "Answers are a list of at most 8 texts."),
        ]
        for (object, status, message) in refusals {
            XCTAssertThrowsError(try request(object).get(), message) {
                XCTAssertEqual(($0 as? BriefRoutes.Refusal)?.response.status, status)
                XCTAssertEqual(($0 as? BriefRoutes.Refusal).map { self.text($0.response) }, message)
            }
        }
    }

    func testTheFoldersToOfferAreThoseStillThere() {
        let app = home.appendingPathComponent("Projects/app").path
        let folders = json(BriefRoutes.folders(default: "~/Projects", recent: [app, home.appendingPathComponent("gone").path]))
        XCTAssertEqual(folders["default"] as? String, "~/Projects")
        XCTAssertEqual(folders["recent"] as? [String], [app])
    }

    func testAHireIsNotedFirstAmongTheRecentFolders() {
        let config = Config()
        for folder in ["/a", "/b", "/a"] + (1...10).map { "/f\($0)" } + ["/b"] { config.noteHire(in: folder) }
        XCTAssertEqual(config.recentHireFolders, ["/b", "/f10", "/f9", "/f8", "/f7", "/f6", "/f5", "/f4"])
    }

    // MARK: On a stand-in server

    /// Setup's gate and these routes, as DeckRouter has them, without the deck.
    private final class StandIn: Router {
        let key = "stand-in-key"
        let config = Config()
        let library: BriefLibrary
        let home: URL
        var hired: [BriefRoutes.HireRequest] = []

        init(library: BriefLibrary, home: URL) {
            (self.library, self.home) = (library, home)
            config.hireFolder = home.appendingPathComponent("Projects").path
        }

        func handle(_ request: Request, reply: @escaping (Reply) -> Void) {
            let respond = { reply(.response($0)) }
            guard DeckRouter.isSetupPath(request.path) else { return respond(.status(404)) }
            if let refused = DeckRouter.setupRefusal(request, isKey: { $0 == self.key }) { return respond(refused) }
            guard BriefRoutes.handles(request.method, request.path) else { return respond(.status(404)) }
            BriefRoutes.handle(request, config: config, library: library, home: home, hire: { hire, done in
                self.hired.append(hire)
                done(.json(["ok": true, "desk": 2, "session": NSNull()]))
            }, respond: respond)
        }
    }

    /// A port nothing listens on, as the system hands one out.
    private func sparePort() throws -> UInt16 {
        let fd = socket(AF_INET, SOCK_STREAM, 0)
        defer { close(fd) }
        var address = sockaddr_in(sin_len: UInt8(MemoryLayout<sockaddr_in>.size), sin_family: sa_family_t(AF_INET), sin_port: 0,
                                  sin_addr: in_addr(s_addr: inet_addr("127.0.0.1")), sin_zero: (0, 0, 0, 0, 0, 0, 0, 0))
        var length = socklen_t(MemoryLayout<sockaddr_in>.size)
        try withUnsafeMutablePointer(to: &address) { pointer in
            try pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                guard Darwin.bind(fd, $0, length) == 0, Darwin.getsockname(fd, $0, &length) == 0 else { throw URLError(.cannotConnectToHost) }
            }
        }
        let port = UInt16(bigEndian: address.sin_port)
        XCTAssertNotEqual(port, 7710)
        return port
    }

    private func send(_ method: String, _ path: String, port: UInt16, key: String?, action: Bool = true, json: Any? = nil) -> (Int, Data) {
        var request = URLRequest(url: URL(string: "http://localhost:\(port)\(path)")!)
        request.httpMethod = method
        if let key { request.setValue(key, forHTTPHeaderField: DeckRouter.setupKeyHeader) }
        if action { request.setValue("1", forHTTPHeaderField: DeckRouter.setupHeader) }
        if let json { request.httpBody = body(json) }
        let done = expectation(description: "\(method) \(path)")
        var result = (0, Data())
        URLSession.shared.dataTask(with: request) { data, response, _ in
            result = ((response as? HTTPURLResponse)?.statusCode ?? 0, data ?? Data())
            done.fulfill()
        }.resume()
        wait(for: [done], timeout: 10)
        return result
    }

    func testTheRoutesAnswerOnlySetupsOwnRequests() throws {
        let port = try sparePort()
        let standIn = StandIn(library: library, home: home)
        let server = HTTPServer(port: port, router: standIn)
        let ready = expectation(description: "listening")
        server.onReady = { ready.fulfill() }
        server.start()
        defer { server.stop() }
        wait(for: [ready], timeout: 10)

        XCTAssertEqual(send("GET", "/setup/briefs", port: port, key: nil).0, 401, "no key")
        XCTAssertEqual(send("GET", "/setup/briefs", port: port, key: "wrong").0, 401)
        XCTAssertEqual(send("POST", "/setup/hire", port: port, key: standIn.key, action: false, json: [:]).0, 403, "no setup header")
        let (status, list) = send("GET", "/setup/briefs", port: port, key: standIn.key, action: false)
        XCTAssertEqual(status, 200)
        XCTAssertEqual(((try JSONSerialization.jsonObject(with: list) as? [String: Any])?["briefs"] as? [Any])?.count, Brief.libraryIDs.count)
        XCTAssertEqual(send("GET", "/setup/briefs/chef", port: port, key: standIn.key).0, 200)
        XCTAssertEqual(send("GET", "/setup/briefs/nope", port: port, key: standIn.key).0, 404)
        XCTAssertEqual(send("POST", "/setup/briefs/preview", port: port, key: standIn.key, json: ["role": "tester", "answers": ["all", "no"]]).0, 200)
        let folders = send("GET", "/setup/hire/folders", port: port, key: standIn.key)
        XCTAssertEqual((try JSONSerialization.jsonObject(with: folders.1) as? [String: Any])?["default"] as? String,
                       home.appendingPathComponent("Projects").path)

        let (refused, reason) = send("POST", "/setup/hire", port: port, key: standIn.key, json: ["role": "janitor", "folder": "/etc"])
        XCTAssertEqual(refused, 400)
        XCTAssertEqual(String(decoding: reason, as: UTF8.self), "Hire into a folder inside your home folder.")
        XCTAssertTrue(standIn.hired.isEmpty)
        let app = home.appendingPathComponent("Projects/app").path
        let (hired, answer) = send("POST", "/setup/hire", port: port, key: standIn.key, json: ["role": "janitor", "folder": app, "answers": ["all"]])
        XCTAssertEqual(hired, 200)
        XCTAssertEqual((try JSONSerialization.jsonObject(with: answer) as? [String: Any])?["ok"] as? Bool, true)
        XCTAssertEqual(standIn.hired, [BriefRoutes.HireRequest(role: "janitor", folder: app, answers: ["all"])])
    }
}
