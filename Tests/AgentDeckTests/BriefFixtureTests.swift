import CryptoKit
import XCTest
@testable import AgentDeck

/// `Tests/fixtures/briefs/briefs.json` is what this side makes of a fixed set of inputs: hires' briefs (their text, files and
/// first message), setup's previews, picked files, a brief's later parts and the guard's state. The Go app's test
/// (desktop/briefs_fixture_test.go) makes the same from the same inputs and must match byte for byte. Paths are written with
/// `~` and each side expands them with its own home; dates are shown in UTC. Set SMOLCORP_WRITE_FIXTURES=1 for a run to
/// write it again.
final class BriefFixtureTests: XCTestCase {
    private let library = BriefLibrary(folder: URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        .deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("Resources/briefs"))
    private let fixture = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
        .appendingPathComponent("fixtures/briefs/briefs.json")
    /// 2027-01-15 08:00 UTC.
    private let date = Date(timeIntervalSince1970: 1_800_000_000)
    private var folder: URL!
    private var savedTZ: String?

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        // Dates in UTC, whatever this Mac's time zone: both sides show the same day and minute.
        savedTZ = ProcessInfo.processInfo.environment["TZ"]
        setenv("TZ", "UTC", 1)
        tzset()
        NSTimeZone.resetSystemTimeZone()
    }

    override func tearDownWithError() throws {
        if let savedTZ { setenv("TZ", savedTZ, 1) } else { unsetenv("TZ") }
        tzset()
        NSTimeZone.resetSystemTimeZone()
        try? FileManager.default.removeItem(at: folder)
    }

    private func expanded(_ path: String) -> String {
        path.hasPrefix("~/") ? NSHomeDirectory() + String(path.dropFirst()) : path
    }
    private func shown(_ path: String) -> String { (path as NSString).abbreviatingWithTildeInPath }
    private func sha256(_ text: String) -> String { SHA256.hash(data: Data(text.utf8)).map { String(format: "%02x", $0) }.joined() }
    private func json(_ role: Role) throws -> Any { try JSONSerialization.jsonObject(with: JSONEncoder().encode(role)) }
    private func shipped(_ id: String) throws -> Role { try XCTUnwrap(Role.shipped.first { $0.id == id }, id) }

    /// A picked file's brief with every rule in it: front matter of its own, comments, sections set and not, nested and
    /// alone on their lines, a report file, a done signal left to its default, and spaces before line ends.
    private let fileBrief = """
        ---
        name: Porter
        summary: Carries things.
        report: CARRY
        heavy: yes
        light: no
        question: What should it carry?
        question: Where to?
        ---
        <!-- A note for whoever edits this. -->
        # {{role}} on {{date}}
        Agent {{agent}}{{#model}} on {{model}}{{/model}}{{#effort}}, effort {{effort}}{{/effort}}.
        {{#heavy}}
        Heavy{{#light}} and light{{/light}}{{^light}} only{{/light}}.
        {{/heavy}}
        {{^heavy}}
        Never shown.
        {{/heavy}}
        Inline <!-- gone --> note.



        ## The job
        {{job}}

        Branch: {{#branch}}{{branch}}{{/branch}}{{^branch}}none{{/branch}}. Read-only: {{read_only}}.
        Brief {{brief_file}}, status {{status_file}}, report {{report_file}}. Say {{done_signal}}.
        {{approval}}
        """

    private func roles() throws -> [String: Role] {
        var porter = Role(id: "porter", name: "Porter", agent: "codex", model: "gpt-5", effort: "high",
                          questions: ["What should it carry?", "Where to?"], instructions: "Carry.", badge: "", outfit: "")
        porter.brief = try Brief.file(fileBrief, named: "porter.md")
        var planner = Role(id: "my-planner", name: "My Planner", agent: "claude", model: "opus", effort: "max",
                           questions: ["What's the plan for?"], instructions: "Plan.", badge: "", outfit: "")
        planner.brief = .library("planner")
        var plain = Role(id: "plain", name: "Plain", agent: "claude", model: "", effort: "",
                         questions: ["First?", "Second?"], instructions: "Just do it.", badge: "", outfit: "")
        plain.brief = .text("Just do it.")
        return ["reviewer": try shipped("reviewer"), "tester": try shipped("tester"), "securityguard": try shipped("securityguard"),
                "gm": try shipped("gm"), "porter": porter, "my-planner": planner, "plain": plain]
    }

    /// One hire's brief, as `BriefBuilder.make` makes it (not written).
    private func hire(_ name: String, role: Role, folder: String = "~/Projects/app", pinned: Bool = false, desk: Int? = nil,
                      answers: [String]? = nil, task: String? = nil, managed: String? = nil) throws -> [String: Any] {
        let kept = "~/Support/briefs"
        var input: [String: Any] = ["name": name, "role": try json(role), "folder": folder, "pinned": pinned, "kept": kept,
                                    "date": date.timeIntervalSince1970]
        if let desk { input["desk"] = desk }
        if let answers { input["answers"] = answers }
        if let task { input["task"] = task }
        if let managed { input["managed"] = managed }
        let made = try BriefBuilder.make(role, library: library, folder: expanded(folder), pinned: pinned, desk: desk, answers: answers,
                                         task: task, managed: managed.map { URL(fileURLWithPath: expanded($0), isDirectory: true) },
                                         date: date, kept: URL(fileURLWithPath: expanded(kept), isDirectory: true), write: false)
        guard let made else { input["made"] = NSNull(); return input }
        input["made"] = ["brief": shown(made.names.brief.path), "status": shown(made.names.status.path),
                         "report": made.names.report.map { shown($0.path) } ?? "", "markdown": made.markdown, "firstMessage": made.firstMessage]
        return input
    }

    private func written() throws -> Data {
        let roles = try roles()
        let reviewer = try XCTUnwrap(roles["reviewer"]), tester = try XCTUnwrap(roles["tester"])
        var hires = [
            try hire("kept, asks its questions", role: reviewer),
            try hire("kept at desk 3, answered", role: reviewer, desk: 3,
                     answers: ["the branch feature-x", "", "bugs\n  and security  \nmost of all\u{7}"]),
            try hire("pinned to its project", role: tester, pinned: true),
            try hire("managed, with a task", role: tester, task: "  Fix the login bug.\n\tStep by step.\u{1b}  ", managed: "~/Support/hires/0123abcd4567"),
            try hire("a picked file, on Codex", role: try XCTUnwrap(roles["porter"]), desk: 0,
                     answers: ["{{role}} and {{#heavy}}braces{{/heavy}}", "there", "one too many"]),
            try hire("a library brief of your own", role: try XCTUnwrap(roles["my-planner"]), managed: "~/Support/hires/feedface0001"),
            try hire("the security guard's", role: try XCTUnwrap(roles["securityguard"]), desk: 0),
            try hire("the general manager", role: try XCTUnwrap(roles["gm"]), managed: "~/Support/hires/9a9a9a9a9a9a"),
            try hire("its own instructions", role: try XCTUnwrap(roles["plain"])),
        ]
        // Every brief of the library, by a role of its own.
        var every: [[String: Any]] = []
        for entry in library.entries() {
            var role = Role(id: entry.id == "security-auditor" ? "auditor" : entry.id, name: entry.name, agent: "claude", model: "", effort: "",
                            questions: entry.questions, instructions: "", badge: "", outfit: "")
            role.brief = .library(entry.id)
            let made = try XCTUnwrap(try BriefBuilder.make(role, library: library, folder: expanded("~/Projects"), pinned: false, desk: 1,
                                                          answers: nil, date: date, kept: URL(fileURLWithPath: expanded("~/Support/briefs")), write: false))
            every.append(["id": entry.id, "role": try json(role), "template": sha256(try XCTUnwrap(library.template(entry.id))),
                          "markdown": sha256(made.markdown), "firstMessage": made.firstMessage])
        }
        hires.sort { ($0["name"] as? String ?? "") < ($1["name"] as? String ?? "") }

        // Setup's previews, as BriefRoutes answers them.
        let saved = [reviewer, try XCTUnwrap(roles["plain"])]
        let previewBodies: [Any] = [
            ["role": "reviewer", "folder": "~/Projects/app", "pinned": false],
            ["role": "reviewer", "folder": "~/Projects/app", "pinned": true, "answers": ["main...feature", "main"]],
            ["role": "plain", "folder": "~/Projects/app", "pinned": false],
            ["role": "plain", "folder": "~/Projects/app", "pinned": true],
            ["role": "plain", "folder": "~/Projects/app", "answers": ["one\nline two"]],
            ["role": ["id": "helper", "name": "Helper", "agent": "claude", "questions": ["Why?"], "instructions": "Help.",
                      "brief": ["kind": "text", "text": "Help a lot."]], "folder": "~/Projects/app", "pinned": false],
            ["role": ["id": "helper", "name": "Helper", "brief": ["kind": "library", "id": "nobody"]], "folder": "~/Projects/app"],
            ["role": ["id": "helper", "name": "Helper", "brief": ["kind": "file", "text": "{{nope}}", "fileName": "a.md"]], "folder": "~/Projects/app"],
            ["role": ["id": "Bad Id", "name": "Helper"], "folder": "~/Projects/app"],
            ["role": "nobody", "folder": "~/Projects/app"],
            ["role": "reviewer", "folder": "~/Projects/app", "answers": "not a list"],
            ["role": "reviewer", "folder": "~/Projects/app", "answers": Array(repeating: "a", count: 9)],
            ["role": NSNull(), "folder": "~/Projects/app"],
            "not json",
        ]
        var previews: [[String: Any]] = []
        for given in previewBodies {
            let body = given is String ? Data((given as! String).utf8) : try JSONSerialization.data(withJSONObject: given, options: [.sortedKeys])
            let answered = BriefRoutes.preview(body, roles: saved, library: library, hireFolder: "", date: date,
                                               kept: URL(fileURLWithPath: expanded("~/Support/briefs")))
            var answer: Any = String(decoding: answered.body, as: UTF8.self)
            if answered.contentType == "application/json" { answer = try JSONSerialization.jsonObject(with: answered.body) }
            previews.append(["body": String(decoding: body, as: UTF8.self), "status": answered.status, "answer": answer])
        }

        // Picked files, as kept or refused.
        let files: [(String, String)] = [
            ("\u{FEFF}# Mine\r\nLine\rEnd\u{7}\twith a tab\n", "notes/dir\\Mine.MD"),
            ("   \n\t", "a.md"),
            ("Hello {{nobody}} and {{#who}}x{{/who}}", "a.md"),
            ("Hello {{> about}}", "a.md"),
            ("Hello {{#role}}open", "a.md"),
            ("Hello", "a.pdf"),
            ("Hello", ".md"),
            ("Hello\u{0}", "a.md"),
            (String(repeating: "x", count: 32 * 1024 + 1), "a.md"),
            ("---\nwho: you\n---\nHi {{who}}", " spaced name.txt "),
        ]
        let picked: [[String: Any]] = files.map { text, name in
            var row: [String: Any] = ["text": text.count > 1000 ? "x*\(text.count)" : text, "name": name]
            do {
                let brief = try Brief.file(text, named: name)
                row["kept"] = ["text": brief.text, "fileName": brief.fileName]
            } catch {
                row["problem"] = error.localizedDescription
            }
            return row
        }

        // A brief's later parts.
        let parts: [[String: Any]] = ["  Also check the login page.\n\n", "Two\nlines"].map { text in
            ["text": text, "date": date.timeIntervalSince1970, "part": BriefMore.part(text, at: date), "note": BriefMore.note("~/BRIEF.md")]
        }

        // The guard's state, for files as they may be.
        let guardFiles: [String?] = [
            nil,
            "{\"groups\":[{\"name\":\"cloud\",\"on\":false,\"commands\":[\"aws\"]},{\"name\":\"extra\",\"on\":true,\"commands\":[\"terraform import\"]},{\"name\":\"mine\",\"on\":true}]}",
            "{\"groups\":[]}",
            "not json",
            "{\"groups\":[{\"name\":\"push\",\"on\":\"yes\"}]}",
            "{\"groups\":[{\"name\":\"push\",\"on\":true,\"commands\":[\"  \"]}]}",
            "{\"groups\":[{\"name\":\"push\",\"on\":true}],\"more\":1}",
        ]
        var guards: [[String: Any]] = []
        for (i, text) in guardFiles.enumerated() {
            let url = folder.appendingPathComponent("guard-\(i).json")
            if let text { try text.write(to: url, atomically: true, encoding: .utf8) }
            guards.append(["file": text.map { $0 as Any } ?? NSNull(), "state": GuardFile.state(url)])
        }

        let object: [String: Any] = [
            "note": "Written by BriefFixtureTests.swift with SMOLCORP_WRITE_FIXTURES=1; read by desktop/briefs_fixture_test.go. "
                + "Paths start with ~ (each side's home), dates are UTC.",
            "entries": library.entries().map(\.jsonObject),
            "hires": hires,
            "library": every,
            "previews": previews,
            "files": picked,
            "parts": parts,
            "guard": guards,
        ]
        return try JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]) + Data("\n".utf8)
    }

    func testTheFixtureIsWhatThisSideWrites() throws {
        let data = try written()
        if ProcessInfo.processInfo.environment["SMOLCORP_WRITE_FIXTURES"] == "1" {
            try FileManager.default.createDirectory(at: fixture.deletingLastPathComponent(), withIntermediateDirectories: true)
            try data.write(to: fixture)
        }
        XCTAssertEqual(try Data(contentsOf: fixture), data, "briefs.json is out of date: run once with SMOLCORP_WRITE_FIXTURES=1")
    }

    /// The fixture's paths are shown with `~`, so they hold nothing of the Mac that wrote them.
    func testTheFixtureNamesNoFolderOfThisMac() throws {
        let text = try String(contentsOf: fixture, encoding: .utf8)
        XCTAssertFalse(text.contains(NSHomeDirectory()))
        XCTAssertFalse(text.contains(folder.deletingLastPathComponent().path))
    }
}
