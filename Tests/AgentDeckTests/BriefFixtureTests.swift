import CryptoKit
import XCTest
@testable import AgentDeck

/// `Tests/fixtures/briefs/briefs.json` is what this side makes of a fixed set of inputs: hires' briefs (their text, files and
/// first message), setup's previews, picked files, a brief's later parts and the guard's state. The Go app's test
/// (desktop/briefs_fixture_test.go) makes the same from the same inputs and must match byte for byte. Paths are written with
/// `~` and each side expands them with its own home; dates are shown in UTC. Set SMOLCORP_WRITE_FIXTURES=1 for a run to
/// write it again.
///
/// One difference is known and kept: a path that goes through `URL` comes out decomposed here (NFD: "e" then the accent),
/// while the folder as given, and everything on the Go side, stays precomposed (NFC). A hire marked `"normalized": "NFC"`
/// holds this side's brief with that undone (`made`), and as it really is (`madeAsIs`); see docs/GO-PARITY.md.
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
                      answers: [String]? = nil, task: String? = nil, managed: String? = nil, nfc: Bool = false) throws -> [String: Any] {
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
        let asIs: [String: String] = ["brief": shown(made.names.brief.path), "status": shown(made.names.status.path),
                                      "report": made.names.report.map { shown($0.path) } ?? "", "markdown": made.markdown, "firstMessage": made.firstMessage]
        input["made"] = asIs
        if nfc {
            let precomposed = asIs.mapValues(\.precomposedStringWithCanonicalMapping)
            // The difference is still there: if this ever fails, the Mac writes NFC too, and the note can go.
            XCTAssertNotEqual(precomposed.values.map { Array($0.unicodeScalars) }, asIs.values.map { Array($0.unicodeScalars) }, name)
            input["made"] = precomposed
            input["madeAsIs"] = asIs
            input["normalized"] = "NFC"
        }
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
            try hire("pinned to a folder with accents, answered", role: tester, folder: "~/Projets/équipe café", pinned: true,
                     answers: ["« tout », s’il te plaît", "la base\u{00A0}de test, déjà prête"], nfc: true),
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
            "{\"groups\":[null]}",
            "{\"groups\":[{\"name\":\"\\n\",\"on\":true}]}",
            "{\"groups\":[{\"name\":\"push\",\"on\":true,\"commands\":[\"a\",1]}]}",
            "{\"groups\":[{\"name\":\"push\",\"on\":0},{\"name\":\"deploy\",\"on\":1.0}]}",
        ]
        var guards: [[String: Any]] = []
        for (i, text) in guardFiles.enumerated() {
            let url = folder.appendingPathComponent("guard-\(i).json")
            if let text { try text.write(to: url, atomically: true, encoding: .utf8) }
            guards.append(["file": text.map { $0 as Any } ?? NSNull(), "state": GuardFile.state(url)])
        }

        // The guard saved from setup (DeckRouter's POST /setup/guard): what it answers, and the file it leaves.
        let kept = "{\"groups\":[{\"name\":\"cloud\",\"on\":false,\"commands\":[\"aws\",\"gcloud\"]},{\"name\":\"extra\",\"on\":true,\"commands\":[\"terraform import\"]},"
            + "{\"name\":\"mine\",\"on\":true,\"commands\":[\"make ship\"]},{\"name\":\"theirs\",\"on\":false}]}"
        let guardSaves: [(file: String?, missingFolder: Bool, body: String)] = [
            (nil, false, "{\"groups\":[{\"name\":\"push\",\"on\":false}]}"),
            (nil, false, "{}"),
            (kept, false, "{\"extra\":[\"terraform import\",\"\\u00a0terraform import\\t\",\"gh  api\",\"git|svn dcommit\",\"make-it.go_v2+ x\"]}"),
            (kept, false, "{\"groups\":[{\"name\":\"cloud\",\"on\":true},{\"name\":\"local\",\"on\":false}]}"),
            (kept, false, "{\"extra\":[]}"),
            ("not json", false, "{\"groups\":[{\"name\":\"deploy\",\"on\":false}]}"),
            (nil, false, "{\"groups\":[{\"name\":\"push\",\"on\":0},{\"name\":\"remote\",\"on\":1}]}"),
            (nil, true, "{\"groups\":[{\"name\":\"push\",\"on\":false}]}"),
            (kept, false, "not json"),
            (nil, false, "[]"),
            (nil, false, "null"),
            (nil, false, "{\"groups\":[],\"more\":1}"),
            (nil, false, "{\"groups\":{\"name\":\"push\",\"on\":false}}"),
            (nil, false, "{\"groups\":null}"),
            (nil, false, "{\"groups\":[null]}"),
            (nil, false, "{\"groups\":[{\"name\":\"push\"}]}"),
            (nil, false, "{\"groups\":[{\"name\":\"push\",\"on\":\"no\"}]}"),
            (nil, false, "{\"groups\":[{\"name\":\"push\",\"on\":2}]}"),
            (kept, false, "{\"groups\":[{\"name\":\"mine\",\"on\":false}]}"),
            (nil, false, "{\"groups\":[{\"name\":\"extra\",\"on\":false}]}"),
            (nil, false, "{\"groups\":[{\"name\":\"push\",\"on\":true},{\"name\":\"push\",\"on\":false}]}"),
            (nil, false, "{\"extra\":\"terraform import\"}"),
            (nil, false, "{\"extra\":[\"ok\",1]}"),
            (nil, false, "{\"extra\":[" + (1...41).map { "\"tool\($0)\"" }.joined(separator: ",") + "]}"),
            (nil, false, "{\"extra\":[\" \\t\"]}"),
            (nil, false, "{\"extra\":[\"*\"]}"),
            (nil, false, "{\"extra\":[\"git|\"]}"),
            (nil, false, "{\"extra\":[\"-rf x\"]}"),
            (nil, false, "{\"extra\":[\"été x\"]}"),
            (nil, false, "{\"extra\":[\"git\\npush\"]}"),
            (nil, false, "{\"extra\":[\"git\\u200bpush\"]}"),
            (nil, false, "{\"extra\":[\"" + String(repeating: "x", count: 201) + "\"]}"),
            (nil, false, "{\"extra\":[\"" + String(repeating: "x", count: 200) + "\"]}"),
        ]
        var saves: [[String: Any]] = []
        for (i, save) in guardSaves.enumerated() {
            let url = save.missingFolder ? folder.appendingPathComponent("missing-\(i)/hire-guard.json") : folder.appendingPathComponent("save-\(i).json")
            if let file = save.file { try file.write(to: url, atomically: true, encoding: .utf8) }
            var row: [String: Any] = ["file": save.file.map { $0 as Any } ?? NSNull(), "body": save.body]
            if save.missingFolder { row["missingFolder"] = true }
            switch GuardFile.save(Data(save.body.utf8), to: url) {
            case .success(let state): row["status"] = 200; row["answer"] = state
            case .failure(.bad(let why)): row["status"] = 400; row["answer"] = why
            case .failure(.notWritten): row["status"] = 500; row["answer"] = "Couldn't write the guard file."
            }
            if let data = try? Data(contentsOf: url) {
                row["after"] = (try? JSONSerialization.jsonObject(with: data)) ?? String(decoding: data, as: UTF8.self)
            } else {
                row["after"] = NSNull()
            }
            saves.append(row)
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
            "guardSaves": saves,
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
