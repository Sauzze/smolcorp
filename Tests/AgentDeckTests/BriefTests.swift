import XCTest
@testable import AgentDeck

/// Roles' briefs: the library, how a brief is filled in on hire, where it's
/// written, what a picked file may hold, and how roles kept before briefs
/// are given one.
final class BriefTests: XCTestCase {
    private let library = BriefLibrary(folder: URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        .deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("Resources/briefs"))
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    private func role(_ id: String) -> Role { Role.shipped.first { $0.id == id }! }

    private func render(_ role: Role, template: String? = nil, answers: [String]? = nil, pinned: Bool = false) throws -> String {
        let template = try template ?? XCTUnwrap(role.briefTemplate(in: library))
        let names = BriefFiles.names(.project(folder, role: role.id), report: Role.reportName(of: template), suffix: 0)
        return try role.renderBrief(template, BriefContext(folder: folder.path, pinned: pinned, answers: answers,
                                                           date: Date(timeIntervalSince1970: 1_792_713_600), briefFile: names.brief.path,
                                                           statusFile: names.status.path, reportFile: names.report?.path))
    }

    // MARK: The template's rules

    func testValuesSectionsAndNotesAreResolved() throws {
        let body = """
            <!-- a note
            on two lines -->
            # {{role}}
            {{#branch}}
            On {{branch}}.
            {{/branch}}
            {{^branch}}
            No branch.
            {{/branch}}
            Inline{{#read_only}} read-only{{/read_only}}{{^read_only}} writable{{/read_only}}.



            End <!-- inline -->
            """
        let known = BriefTemplate.names
        XCTAssertEqual(try BriefTemplate.render(body, values: ["role": "Janitor", "branch": "dev", "read_only": "true"], known: known),
                       "# Janitor\nOn dev.\nInline read-only.\n\nEnd\n")
        XCTAssertEqual(try BriefTemplate.render(body, values: ["role": "Janitor", "read_only": "no"], known: known),
                       "# Janitor\nNo branch.\nInline writable.\n\nEnd\n")
        for unset in ["", "false", "No", "0", "  "] { XCTAssertFalse(BriefTemplate.isSet(unset)) }
        XCTAssertTrue(BriefTemplate.isSet("true"))
    }

    func testSectionsNestAndValuesAreNeverReadForTags() throws {
        let body = "{{#branch}}A{{#read_only}}B{{/read_only}}{{^read_only}}C{{/read_only}}{{/branch}} {{job}}"
        XCTAssertEqual(try BriefTemplate.render(body, values: ["branch": "x", "job": "{{#branch}}{{role}}"], known: BriefTemplate.names),
                       "AC {{#branch}}{{role}}\n")
    }

    func testATagLeftOverIsRefused() {
        XCTAssertThrowsError(try BriefTemplate.render("{{nope}}", values: [:], known: BriefTemplate.names)) {
            XCTAssertEqual($0 as? Brief.Problem, .unknown(["nope"]))
        }
        XCTAssertThrowsError(try BriefTemplate.render("{{#branch}}open", values: [:], known: BriefTemplate.names)) {
            XCTAssertEqual($0 as? Brief.Problem, .sections)
        }
        XCTAssertThrowsError(try BriefTemplate.render("{{ role }}", values: [:], known: BriefTemplate.names)) {
            XCTAssertEqual($0 as? Brief.Problem, .sections)
        }
        XCTAssertThrowsError(try BriefTemplate.render("{{> about}}", values: [:], known: BriefTemplate.names)) {
            XCTAssertEqual($0 as? Brief.Problem, .slots)
        }
    }

    func testFrontMatterIsReadAndKeptOutOfTheBody() {
        let parsed = BriefTemplate.frontMatter("---\nname: Janitor\nquestion: One?\nquestion: Two?\nbad key: x\ndone_signal: TIDY\n---\n# Body\n")
        XCTAssertEqual(parsed.values, ["name": "Janitor", "done_signal": "TIDY"])
        XCTAssertEqual(parsed.questions, ["One?", "Two?"])
        XCTAssertEqual(parsed.body, "# Body\n")
        XCTAssertEqual(BriefTemplate.frontMatter("---\n---\nText").body, "Text")
        XCTAssertEqual(BriefTemplate.frontMatter("No front matter").body, "No front matter")
    }

    // MARK: The library

    func testTheLibraryHasEveryBriefAndNothingElse() throws {
        let files = try FileManager.default.contentsOfDirectory(atPath: library.folder.path).filter { $0.hasSuffix(".md") }.sorted()
        XCTAssertEqual(files, (["_base"] + Brief.libraryIDs).map { "\($0).md" }.sorted())
        let entries = library.entries()
        XCTAssertEqual(entries.map(\.id), Brief.libraryIDs)
        for entry in entries {
            XCTAssertFalse(entry.name.isEmpty, entry.id)
            XCTAssertFalse(entry.summary.isEmpty, entry.id)
            XCTAssertFalse(entry.questions.isEmpty, entry.id)
        }
        XCTAssertNil(library.template("_base"))
        XCTAssertNil(library.template("../Info"))
        XCTAssertNil(library.template("securityguard"), "its brief is the security auditor's")
    }

    /// Every shipped role has its own brief, in their order, with its questions; the Security guard's is the security auditor's.
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

    func testShippedRolesHaveTheirOwnBriefWithTheirQuestions() throws {
        let shipped = Role.shipped.map(\.id).filter { $0 != "securityguard" }
        XCTAssertEqual(Array(Brief.libraryIDs.prefix(shipped.count)), shipped)
        for id in shipped {
            let role = role(id)
            XCTAssertEqual(role.source, .library(id), id)
            let entry = try XCTUnwrap(library.entries().first { $0.id == id })
            XCTAssertEqual(entry.name, role.name, id)
            XCTAssertEqual(entry.questions, role.questions, id)
        }
        XCTAssertEqual(role("securityguard").source, .library("security-auditor"))
        XCTAssertNotNil(try role("securityguard").briefTemplate(in: library))
    }

    /// The library is product text: nothing of one team's own setup in it.
    /// Only the General Manager's own brief names its role, as its name.
    func testTheLibraryNamesNoOneInParticular() throws {
        let files = try FileManager.default.contentsOfDirectory(atPath: library.folder.path)
        XCTAssertEqual(files.count, Brief.libraryIDs.count + 1)
        for file in files {
            let text = try String(contentsOf: library.folder.appendingPathComponent(file), encoding: .utf8)
                .replacingOccurrences(of: "\nname: General Manager\n", with: "\n")
            XCTAssertNil(text.range(of: #"\bGMs?\b"#, options: .regularExpression), "\(file) names a GM")
            for word in ["General Manager", "/Users/", "~/Projects", "TO-GM", "SendMessage", "ListAgents", "Claude Code", "7710", "Sauzze"] {
                XCTAssertFalse(text.contains(word), "\(file) has \(word)")
            }
        }
    }

    /// Rules that matter most, kept in their briefs.
    func testTheRolesOwnRulesAreThere() throws {
        let brief = { (id: String) in try XCTUnwrap(self.library.template(id)) }
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

    func testEveryLibraryBriefHasEverySection() throws {
        for id in Brief.libraryIDs {
            let text = try XCTUnwrap(library.template(id), id)
            XCTAssertFalse(text.contains("{{>"), id)
            for heading in ["## Who you are", "## The job", "## Deliver", "## Verify", "## Report", "## Never", "## Clean up"] {
                XCTAssertTrue(text.contains(heading), "\(id) lacks \(heading)")
            }
        }
    }

    // MARK: Rendering

    func testEveryLibraryBriefRendersWithNothingLeftRaw() throws {
        try FileManager.default.createDirectory(at: folder.appendingPathComponent(".git"), withIntermediateDirectories: true)
        for id in Brief.libraryIDs {
            let shipped = Role.shipped.first { $0.id == id }
            let role = shipped ?? Role(id: "own-1", name: "My \(id)", agent: "codex", model: "gpt-5", effort: "high",
                                       questions: library.entries().first { $0.id == id }!.questions, instructions: "", badge: "star",
                                       brief: .library(id))
            for (answers, git) in [(nil, false), (role.questions.map { "Answer to: \($0)" }, true), ([], false)] as [([String]?, Bool)] {
                let head = folder.appendingPathComponent(".git/HEAD")
                if git { try "ref: refs/heads/fix-1\n".write(to: head, atomically: true, encoding: .utf8) } else { try? FileManager.default.removeItem(at: head) }
                for pinned in [false, true] {
                    let text = try render(role, answers: answers, pinned: pinned)
                    let context = "\(id), answers \(answers.map { "\($0.count)" } ?? "asked"), git \(git), pinned \(pinned)"
                    XCTAssertFalse(text.contains("{{") || text.contains("}}") || text.contains("<!--"), context)
                    XCTAssertFalse(text.contains("\n\n\n"), context)
                    XCTAssertTrue(text.hasPrefix("# Brief: \(role.name)\n"), context)
                    XCTAssertTrue(text.contains(BriefContext.approval), context)
                    XCTAssertTrue(text.contains("on 2026-10-23"), context)
                    XCTAssertTrue(text.contains(folder.path), context)
                    XCTAssertTrue(text.contains(folder.appendingPathComponent("STATUS.md").path), context)
                    XCTAssertTrue(text.contains(git ? "`fix-1` is checked out" : "No git here"), context)
                    let done = try XCTUnwrap(BriefTemplate.frontMatter(library.template(id)!).values["done_signal"])
                    XCTAssertTrue(text.contains("`\(done): <one-line summary>`"), context)
                    if let answers {
                        XCTAssertFalse(text.contains("Before anything else, ask"), context)
                        for (i, question) in role.questions.enumerated() {
                            XCTAssertTrue(text.contains("- **\(question)** \(i < answers.count ? answers[i] : "*No answer")"), context)
                        }
                    } else {
                        XCTAssertTrue(text.contains("Before anything else, ask the user these questions"), context)
                        XCTAssertTrue(text.contains(pinned ? "this desk's project. Is that the one" : "Which project should you work on:"), context)
                        for question in role.questions { XCTAssertTrue(text.contains(question), context) }
                    }
                    if shipped == nil {
                        XCTAssertTrue(text.contains("**You run as** Codex, model `gpt-5`."), context)
                    }
                    let lines = text.split(separator: "\n", omittingEmptySubsequences: false).count
                    XCTAssertLessThan(lines, 130, context)
                }
            }
        }
    }

    func testHowTheRoleRunsAndItsReportAreFilledIn() throws {
        let reviewer = try render(role("reviewer"), answers: ["feature-x", "main", "security"])
        XCTAssertTrue(reviewer.contains("**You run as** Claude Code, model `opus`, effort high."))
        XCTAssertTrue(reviewer.contains("`\(folder.appendingPathComponent("REVIEW.md").path)`, next to this brief"))
        XCTAssertTrue(reviewer.contains("You're **read-only**"))
        XCTAssertTrue(reviewer.contains("- **Which project should you work on?** `\(folder.path)`"))
        var plain = role("reviewer")
        plain.model = ""
        plain.effort = ""
        XCTAssertTrue(try render(plain).contains("**You run as** Claude Code. Don't switch"))
        let docs = try render(role("docs"))
        XCTAssertFalse(docs.contains("read-only"))
        XCTAssertTrue(docs.contains("DOCS DONE"))
    }

    func testAnAnswerStaysInItsItem() throws {
        let text = try render(role("bughunter"), answers: ["the importer\n# Not a heading\n\u{7}and the parser"])
        XCTAssertTrue(text.contains("** the importer\n  # Not a heading\n  and the parser\n"))
    }

    func testAPickedFileIsFilledInToo() throws {
        let brief = try Brief.file("---\ndone_signal: SHIP IT\nreport: NOTES\n---\n# {{role}} in {{project_folder}}\n\n{{job}}\n\nEnd with `{{done_signal}}`, notes in {{report_file}}.{{#branch}} On {{branch}}.{{/branch}}\n",
                                   named: "ship.md")
        var role = Role(id: "own-1", name: "Shipper", agent: "claude", model: "", effort: "", questions: ["What ships?"],
                        instructions: "", badge: "star", brief: brief)
        role.brief = role.source
        let text = try render(role, template: role.briefTemplate(in: library), answers: ["v2"])
        XCTAssertEqual(text, "# Shipper in \(folder.path)\n\nThe user answered these questions as they hired you:\n\n"
                       + "- **Which project should you work on?** `\(folder.path)`\n- **What ships?** v2\n\n"
                       + "End with `SHIP IT`, notes in \(folder.appendingPathComponent("NOTES.md").path).\n")
    }

    func testATextRoleHasNoBriefAndStartsAsBefore() throws {
        var lawyer = role("lawyer")
        lawyer.instructions = "Read the licences I point you to."
        XCTAssertEqual(lawyer.source, .text(lawyer.instructions), "edited, it's on its instructions")
        XCTAssertNil(try lawyer.briefTemplate(in: library))
        XCTAssertTrue(lawyer.firstMessage(in: folder.path, pinned: false).contains("as my Lawyer: Read the licences"))
        let answered = lawyer.firstMessage(in: folder.path, answers: ["the licence"])
        XCTAssertTrue(answered.contains("- What should you read: the licence, terms, a privacy policy, or the dependencies' licences? the licence"))
        XCTAssertTrue(answered.contains("(No answer: use your judgement.)"))
        XCTAssertTrue(answered.hasSuffix("As my Lawyer: \(lawyer.instructions)"))
    }

    func testTheFirstMessageSendsTheAgentToItsBrief() {
        let janitor = role("janitor")
        let asks = janitor.briefMessage(at: folder.appendingPathComponent("BRIEF.md").path, asks: true)
        XCTAssertTrue(asks.hasPrefix("Read `\(folder.path)/BRIEF.md` and follow it: it's your brief, as my Janitor."))
        XCTAssertTrue(asks.contains("your first reply must only ask me its questions"))
        XCTAssertTrue(janitor.briefMessage(at: "/x/Application Support/1-janitor.md", asks: false).hasPrefix("Read `/x/Application Support/1-janitor.md` and start"))
        XCTAssertTrue(janitor.startScript(firstMessage: asks).contains("'--' 'Read "))
    }

    // MARK: Where it's written

    func testABriefNeverWritesOverAFile() throws {
        let write = { (role: String) in
            try BriefFiles.write(.project(self.folder, role: role), report: "REVIEW") { "brief for \($0.status.lastPathComponent)" }
        }
        let first = try write("reviewer")
        XCTAssertEqual(first, BriefFiles.Names(brief: folder.appendingPathComponent("BRIEF.md"), status: folder.appendingPathComponent("STATUS.md"),
                                               report: folder.appendingPathComponent("REVIEW.md")))
        XCTAssertEqual(try String(contentsOf: first.brief, encoding: .utf8), "brief for STATUS.md")
        XCTAssertEqual(try write("reviewer").brief.lastPathComponent, "BRIEF-reviewer.md")
        XCTAssertEqual(try write("reviewer").brief.lastPathComponent, "BRIEF-reviewer-2.md")
        XCTAssertEqual(try write("tester").brief.lastPathComponent, "BRIEF-tester.md")
        XCTAssertEqual(try String(contentsOf: first.brief, encoding: .utf8), "brief for STATUS.md", "left as it was")
    }

    func testAStatusOrReportFileOfYourOwnIsLeftAloneToo() throws {
        try "mine".write(to: folder.appendingPathComponent("STATUS.md"), atomically: true, encoding: .utf8)
        try "mine".write(to: folder.appendingPathComponent("REVIEW-reviewer.md"), atomically: true, encoding: .utf8)
        // A link where the brief would go, even to nowhere, is there too.
        try FileManager.default.createSymbolicLink(atPath: folder.appendingPathComponent("BRIEF-reviewer-3.md").path, withDestinationPath: "/nowhere")
        let names = try (1...2).map { _ in try BriefFiles.write(.project(folder, role: "reviewer"), report: "REVIEW") { _ in "brief" } }
        XCTAssertEqual(names.map(\.brief.lastPathComponent), ["BRIEF-reviewer-2.md", "BRIEF-reviewer-4.md"])
        XCTAssertEqual(names[0].status.lastPathComponent, "STATUS-reviewer-2.md")
        XCTAssertEqual(try String(contentsOf: folder.appendingPathComponent("STATUS.md"), encoding: .utf8), "mine")
        XCTAssertFalse(FileManager.default.fileExists(atPath: folder.appendingPathComponent("BRIEF.md").path))
    }

    func testAFolderThatCantBeWrittenIsSaidSo() {
        XCTAssertThrowsError(try BriefFiles.write(.project(folder.appendingPathComponent("missing"), role: "reviewer"), report: nil) { _ in "brief" }) {
            XCTAssertTrue($0.localizedDescription.hasPrefix("Couldn't write the brief into"))
        }
    }

    /// Only a desk pinned to its own project gets its brief there; any other
    /// hire's goes to Smolcorp's own folder, so ~/Projects stays clean.
    func testAnUnpinnedHiresBriefIsKeptOutOfTheSharedFolder() throws {
        let projects = folder.appendingPathComponent("Projects", isDirectory: true)
        let kept = folder.appendingPathComponent("Support/briefs", isDirectory: true)
        try FileManager.default.createDirectory(at: projects, withIntermediateDirectories: true)
        let date = Date(timeIntervalSince1970: 1_792_766_520) // 2026-10-23 14:42 UTC
        let stamp = { () -> String in
            let format = DateFormatter()
            format.dateFormat = "yyyyMMdd-HHmm"
            return format.string(from: date)
        }()
        XCTAssertEqual(BriefFiles.place(for: "janitor", in: projects.path, pinned: true, desk: 2, at: date, kept: kept),
                       .project(projects, role: "janitor"))
        let place = BriefFiles.place(for: "janitor", in: projects.path, pinned: false, desk: 3, at: date, kept: kept)
        XCTAssertEqual(place, .kept(kept, stem: "3-janitor-\(stamp)"))
        XCTAssertEqual(BriefFiles.place(for: "janitor", in: projects.path, pinned: false, desk: nil, at: date, kept: kept),
                       .kept(kept, stem: "desk-janitor-\(stamp)"))
        let first = try BriefFiles.write(place, report: "CLEANUP") { _ in "brief" }
        XCTAssertEqual(first, BriefFiles.Names(brief: kept.appendingPathComponent("3-janitor-\(stamp).md"),
                                               status: kept.appendingPathComponent("3-janitor-\(stamp)-STATUS.md"),
                                               report: kept.appendingPathComponent("3-janitor-\(stamp)-CLEANUP.md")))
        let attributes = try FileManager.default.attributesOfItem(atPath: kept.path)
        XCTAssertEqual((attributes[.posixPermissions] as? NSNumber)?.intValue, 0o700, "made for you alone")
        XCTAssertEqual(try BriefFiles.write(place, report: "CLEANUP") { _ in "brief" }.brief.lastPathComponent, "3-janitor-\(stamp)-2.md")
        XCTAssertEqual(try FileManager.default.contentsOfDirectory(atPath: projects.path), [], "nothing in the shared folder")
        // The brief names the folder the hire works in, and its logbook beside the brief.
        let text = try role("janitor").renderBrief(try XCTUnwrap(role("janitor").briefTemplate(in: library)),
                                                   BriefContext(folder: projects.path, pinned: false, answers: nil, briefFile: first.brief.path,
                                                                statusFile: first.status.path, reportFile: first.report?.path))
        XCTAssertTrue(text.contains("- **Folder:** `\(projects.path)`."))
        XCTAssertTrue(text.contains("**`\(first.status.path)`** is your logbook"))
        XCTAssertTrue(text.contains("`\(first.report!.path)`, next to this brief"))
    }

    func testTheBranchIsReadFromGitsOwnFiles() throws {
        let project = folder.appendingPathComponent("project/sub", isDirectory: true)
        try FileManager.default.createDirectory(at: project, withIntermediateDirectories: true)
        XCTAssertNil(Brief.branch(in: project.path))
        let git = folder.appendingPathComponent("project/.git", isDirectory: true)
        try FileManager.default.createDirectory(at: git, withIntermediateDirectories: true)
        try "ref: refs/heads/feature/briefs\n".write(to: git.appendingPathComponent("HEAD"), atomically: true, encoding: .utf8)
        XCTAssertEqual(Brief.branch(in: project.path), "feature/briefs", "from a folder inside it")
        try "0123456789abcdef0123456789abcdef01234567\n".write(to: git.appendingPathComponent("HEAD"), atomically: true, encoding: .utf8)
        XCTAssertNil(Brief.branch(in: project.path), "detached")
        // A worktree: .git is a file naming its folder.
        let worktree = folder.appendingPathComponent("wt", isDirectory: true)
        let gitDir = git.appendingPathComponent("worktrees/wt", isDirectory: true)
        try FileManager.default.createDirectory(at: worktree, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: gitDir, withIntermediateDirectories: true)
        try "ref: refs/heads/role-briefs\n".write(to: gitDir.appendingPathComponent("HEAD"), atomically: true, encoding: .utf8)
        try "gitdir: \(gitDir.path)\n".write(to: worktree.appendingPathComponent(".git"), atomically: true, encoding: .utf8)
        XCTAssertEqual(Brief.branch(in: worktree.path), "role-briefs")
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

    /// An edit of a library role's instructions, from any page, puts it on them.
    func testEditingALibraryRolesInstructionsPutsItOnThem() throws {
        let saved = Role.checked(Role.shipped)
        var janitor = try XCTUnwrap(saved.first { $0.id == "janitor" })
        XCTAssertEqual(janitor.brief, .library("janitor"))
        janitor.instructions = "Tidy only the docs folder."
        let synced = Role.synced([janitor], saved: saved)
        XCTAssertEqual(synced.first?.brief, .text("Tidy only the docs folder."))
        // Through the route, as the page from before briefs sends it, its brief as the state gave it.
        var object = janitor.jsonObject
        object["brief"] = ["kind": "library", "id": "janitor"]
        let body = try JSONSerialization.data(withJSONObject: [object])
        XCTAssertEqual(try BriefRoutes.roles(body, saved: saved).get().first?.source, .text("Tidy only the docs folder."))
        // Picking a library brief, instructions unchanged, keeps it; so does a new role.
        var picked = try XCTUnwrap(saved.first { $0.id == "lawyer" })
        picked.brief = .library("thinker")
        XCTAssertEqual(Role.synced([picked], saved: saved).first?.brief, .library("thinker"))
        var fresh = picked
        fresh.id = "own-9"
        fresh.instructions = "Something else."
        XCTAssertEqual(Role.synced([fresh], saved: saved).first?.brief, .library("thinker"))
    }
}
