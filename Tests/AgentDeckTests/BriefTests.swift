import XCTest
@testable import AgentDeck

/// Roles' briefs: what the briefs that come with the app say (the core fills
/// them in), what a picked file may hold, and how roles kept before briefs
/// are given one.
final class BriefTests: XCTestCase {
    private let library = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        .deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("Resources/briefs")
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    private func role(_ id: String) -> Role { Role.shipped.first { $0.id == id }! }

    /// A brief that comes with the app, as written; nil for a name that isn't one of them.
    private func template(_ id: String) -> String? {
        guard Brief.libraryIDs.contains(id) else { return nil }
        return try? String(contentsOf: library.appendingPathComponent(id + ".md"), encoding: .utf8)
    }

    // MARK: The template's rules

    func testATagLeftOverIsRefused() {
        XCTAssertThrowsError(try BriefTemplate.check("{{nope}}")) {
            XCTAssertEqual($0 as? Brief.Problem, .unknown(["nope"]))
        }
        XCTAssertThrowsError(try BriefTemplate.check("{{#branch}}open")) {
            XCTAssertEqual($0 as? Brief.Problem, .sections)
        }
        XCTAssertThrowsError(try BriefTemplate.check("{{ role }}")) {
            XCTAssertEqual($0 as? Brief.Problem, .sections)
        }
        XCTAssertThrowsError(try BriefTemplate.check("{{> about}}")) {
            XCTAssertEqual($0 as? Brief.Problem, .slots)
        }
    }

    func testFrontMatterIsReadAndKeptOutOfTheBody() {
        let parsed = BriefTemplate.frontMatter("---\nname: Janitor\nquestion: One?\nquestion: Two?\nbad key: x\ndone_signal: TIDY\n---\n# Body\n")
        XCTAssertEqual(parsed.values, ["name": "Janitor", "done_signal": "TIDY"])
        XCTAssertEqual(parsed.body, "# Body\n")
        XCTAssertEqual(BriefTemplate.frontMatter("---\n---\nText").body, "Text")
        XCTAssertEqual(BriefTemplate.frontMatter("No front matter").body, "No front matter")
    }

    // MARK: The library

    func testTheLibraryHasEveryBriefAndNothingElse() throws {
        let files = try FileManager.default.contentsOfDirectory(atPath: library.path).filter { $0.hasSuffix(".md") }.sorted()
        XCTAssertEqual(files, (["_base"] + Brief.libraryIDs).map { "\($0).md" }.sorted())
        for id in Brief.libraryIDs {
            let values = BriefTemplate.frontMatter(try XCTUnwrap(template(id), id)).values
            XCTAssertFalse((values["name"] ?? "").isEmpty, id)
            XCTAssertFalse((values["summary"] ?? "").isEmpty, id)
        }
        XCTAssertNil(template("_base"))
        XCTAssertNil(template("../Info"))
        XCTAssertNil(template("securityguard"), "its brief is the security auditor's")
    }

    /// As many roles of your own as you make: there's no most.
    func testEveryRoleOfYourOwnIsKept() {
        let own = (0..<40).map { Role(id: "own-\($0)", name: "Own \($0)", agent: "claude", model: "", effort: "", questions: [],
                                      instructions: "Do \($0).", badge: "star") }
        let kept = Role.checked(own + Role.shipped)
        XCTAssertEqual(kept.count, own.count + Role.shipped.count)
        XCTAssertEqual(Array(kept.prefix(own.count)).map(\.id), own.map(\.id), "in the order given")
    }

    /// The General Manager as it was saved before it ran a team, with its brief, comes up to date; one you changed doesn't.
    func testTheGeneralManagerFromBeforeTeamsIsBroughtUpToDate() throws {
        var saved = try XCTUnwrap(Role.shippedBeforeTeams.first)
        saved.brief = .library("gm")
        let now = try XCTUnwrap(Role.checked([saved]).first)
        XCTAssertEqual(now.questions, Role.shipped.first { $0.id == "gm" }?.questions)
        XCTAssertEqual(now.source, .library("gm"))
        var edited = saved
        edited.instructions = "Plan only."
        XCTAssertEqual(Role.checked([edited]).first?.instructions, "Plan only.")
    }

    /// Every shipped role has its own brief, in their order, with its questions; the Security guard's is the security auditor's.
    func testShippedRolesHaveTheirOwnBriefWithTheirQuestions() throws {
        let shipped = Role.shipped.map(\.id).filter { $0 != "securityguard" }
        XCTAssertEqual(Array(Brief.libraryIDs.prefix(shipped.count)), shipped)
        for id in shipped {
            let role = role(id)
            XCTAssertEqual(role.source, .library(id), id)
            let text = try XCTUnwrap(template(id), id)
            XCTAssertEqual(BriefTemplate.frontMatter(text).values["name"], role.name, id)
            let asked = text.components(separatedBy: "\n").filter { $0.hasPrefix("question:") }
                .map { $0.dropFirst("question:".count).trimmingCharacters(in: .whitespaces) }
            XCTAssertEqual(asked, role.questions, id)
        }
        XCTAssertEqual(role("securityguard").source, .library("security-auditor"))
    }

    /// The library is product text: nothing of one team's own setup in it.
    /// Only the General Manager's own brief names its role, as its name.
    func testTheLibraryNamesNoOneInParticular() throws {
        let files = try FileManager.default.contentsOfDirectory(atPath: library.path)
        XCTAssertEqual(files.count, Brief.libraryIDs.count + 1)
        for file in files {
            let text = try String(contentsOf: library.appendingPathComponent(file), encoding: .utf8)
                .replacingOccurrences(of: "\nname: General Manager\n", with: "\n")
            XCTAssertNil(text.range(of: #"\bGMs?\b"#, options: .regularExpression), "\(file) names a GM")
            for word in ["General Manager", "/Users/", "~/Projects", "TO-GM", "SendMessage", "ListAgents", "Claude Code", "7710", "Sauzze"] {
                XCTAssertFalse(text.contains(word), "\(file) has \(word)")
            }
        }
    }

    /// Rules that matter most, kept in their briefs.
    func testTheRolesOwnRulesAreThere() throws {
        let brief = { (id: String) in try XCTUnwrap(self.template(id)) }
        XCTAssertTrue(try brief("janitor").contains("is **never** `rm`'d: move it to the Trash (`mv <path> ~/.Trash/`"))
        XCTAssertTrue(try brief("janitor").contains("a tracked file goes with `git rm` in a commit"))
        XCTAssertTrue(try brief("janitor").contains("**You delete nothing, move nothing and change nothing until the user says which items to clear.**"))
        XCTAssertTrue(try brief("reviewer").contains("Judge the code itself, not the author's description of it."))
        XCTAssertTrue(try brief("builder").contains("in light and dark wherever the project has both"))
        XCTAssertTrue(try brief("designer").contains("in light and dark wherever the project has both"))
        XCTAssertTrue(try brief("lawyer").contains("**This isn't legal advice**"))
        for id in ["reviewer", "bughunter", "security-auditor", "perf-auditor", "thinker", "planner", "lawyer", "pilot", "postman", "accountant"] {
            XCTAssertEqual(BriefTemplate.frontMatter(try brief(id)).values["read_only"], "true", id)
        }
    }

    // MARK: A picked file

    func testAPickedFileIsCheckedAndCleaned() throws {
        let brief = try Brief.file("\u{FEFF}# Mine\r\nLine\rTab\tbell\u{7}\n", named: "/Users/x/Briefs/mine.MD")
        XCTAssertEqual(brief, Brief(kind: .file, text: "# Mine\nLine\nTab\tbell\n", fileName: "mine.MD"))
        let refused: [(String, String, Brief.Problem)] = [
            ("  \n", "a.md", .empty),
            (String(repeating: "x", count: Brief.fileLimit + 1), "a.md", .tooLong),
            ("bin\0ary", "a.md", .notText),
            ("bad \u{FFFD} bytes", "a.md", .notText),
            ("# Mine", "a.pdf", .fileName),
            ("# Mine", "", .fileName),
            ("# Mine", String(repeating: "a", count: 98) + ".md", .fileName),
            ("{{> about}}", "a.md", .slots),
            ("{{#branch}}", "a.md", .sections),
            ("{{secret}} and {{role}} and {{#extra}}x{{/extra}}", "a.md", .unknown(["extra", "secret"])),
        ]
        for (text, name, problem) in refused {
            XCTAssertThrowsError(try Brief.file(text, named: name), "\(name): \(text.prefix(20))") { XCTAssertEqual($0 as? Brief.Problem, problem) }
        }
        XCTAssertNoThrow(try Brief.file(String(repeating: "x", count: Brief.fileLimit), named: "a.txt"))
        XCTAssertNoThrow(try Brief.file("---\nextra: yes\n---\n{{#extra}}x{{/extra}}", named: "a.markdown"), "its own front matter's keys")
        XCTAssertTrue(Brief.Problem.unknown(["secret"]).localizedDescription.contains("{{secret}}"))
    }

    // MARK: Roles kept before briefs

    private func loaded(_ roles: String, offered: String = "") throws -> [Role] {
        let file = folder.appendingPathComponent("config.json")
        try #"{"roles": \#(roles)\#(offered)}"#.write(to: file, atomically: true, encoding: .utf8)
        return Config.load(from: file).roles
    }

    private func json(_ role: Role) throws -> String {
        var old = role
        old.brief = nil
        return String(decoding: try JSONEncoder().encode(old), as: UTF8.self)
    }

    func testAnOldConfigsRolesAreGivenBriefs() throws {
        var edited = role("janitor")
        edited.instructions = "Tidy up, my way."
        var renamed = role("chef")
        renamed.name = "Cook"
        let own = Role(id: "own-1", name: "Mine", agent: "claude", model: "", effort: "", questions: ["Where?"], instructions: "Do it.", badge: "star")
        let old171 = Role.shippedIn171.first { $0.id == "reviewer" }!
        let roles = try loaded("[\(try json(role("tester"))), \(try json(edited)), \(try json(renamed)), \(try json(own)), \(try json(old171)), \(try json(role("securityguard")))]",
                               offered: #", "rolesOffered": \#(String(decoding: try JSONEncoder().encode(Role.shipped.map(\.id)), as: UTF8.self))"#)
        let by = { (id: String) in roles.first { $0.id == id }! }
        XCTAssertEqual(by("tester").brief, .library("tester"), "a shipped role as it came")
        XCTAssertEqual(by("janitor").brief, .text("Tidy up, my way."), "your edit stays yours")
        XCTAssertEqual(by("janitor").instructions, "Tidy up, my way.")
        XCTAssertEqual(by("chef").brief, .library("chef"), "a name of yours keeps the shipped brief")
        XCTAssertEqual(by("chef").name, "Cook")
        XCTAssertEqual(by("own-1").brief, .text("Do it."), "your own role, as it was")
        XCTAssertEqual(by("own-1").questions, ["Where?"])
        XCTAssertEqual(by("reviewer"), { var r = role("reviewer"); r.brief = .library("reviewer"); return r }(), "1.7.1's is brought up to date")
        XCTAssertEqual(by("securityguard").brief, .library("security-auditor"), "the security auditor's")
    }

    func testAConfigsBriefsRoundTrip() throws {
        var file = role("reviewer")
        file.brief = try Brief.file("# Mine {{role}}", named: "mine.md")
        var picked = role("tester")
        picked.brief = .library("security-auditor")
        let config = Config()
        config.roles = Role.checked([file, picked, role("docs")])
        let encoded = try JSONEncoder().encode(config)
        let saved = try XCTUnwrap(try JSONSerialization.jsonObject(with: encoded) as? [String: Any])
        let roles = try XCTUnwrap(saved["roles"] as? [[String: Any]])
        XCTAssertEqual(roles[0]["brief"] as? [String: String], ["kind": "file", "text": "# Mine {{role}}", "fileName": "mine.md"])
        XCTAssertEqual(roles[1]["brief"] as? [String: String], ["kind": "library", "id": "security-auditor"])
        XCTAssertEqual(roles[2]["brief"] as? [String: String], ["kind": "library", "id": "docs"])
        let back = try JSONDecoder().decode(Config.self, from: encoded)
        XCTAssertEqual(back.roles, config.roles)
    }

    func testABriefThatCantBeOneFallsBackToTheDefault() throws {
        let tester = try json(role("tester")).dropLast()
        let roles = try loaded("""
            [\(tester), "brief": {"kind": "library", "id": "nope"}},
             {"id": "own-1", "name": "Mine", "instructions": "Do it.", "brief": {"kind": "file", "text": "{{secret}}", "fileName": "a.md"}},
             {"id": "own-2", "name": "Other", "instructions": "Go.", "brief": {"kind": "teleport"}},
             {"id": "own-3", "name": "Third", "instructions": "", "brief": {"kind": "text", "text": "From the brief."}}]
            """)
        XCTAssertEqual(roles.prefix(4).map(\.brief), [.library("tester"), .text("Do it."), .text("Go."), .text("From the brief.")])
        XCTAssertEqual(roles[3].instructions, "From the brief.")
    }

    func testABriefsTextAndTheInstructionsStayTheSame() {
        let saved = Role(id: "own-1", name: "Mine", agent: "claude", model: "", effort: "", instructions: "Old.", badge: "star", brief: .text("Old."))
        var oldPage = saved
        oldPage.instructions = "Edited on the old page."
        XCTAssertEqual(Role.synced([oldPage], saved: [saved]).first?.brief, .text("Edited on the old page."))
        var newPage = saved
        newPage.brief = .text("Edited on the new page.")
        XCTAssertEqual(Role.synced([newPage], saved: [saved]).first?.instructions, "Edited on the new page.")
        var fresh = saved
        fresh.id = "own-2"
        fresh.brief = .text("New role's text.")
        XCTAssertEqual(Role.synced([fresh], saved: [saved]).first?.instructions, "New role's text.")
        var library = saved
        library.brief = .library("builder")
        XCTAssertEqual(Role.synced([library], saved: [saved]).first, library, "a library brief, its instructions as they were")
    }
}
