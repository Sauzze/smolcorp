import XCTest
@testable import AgentDeck

/// `Tests/fixtures/frontdesk/hire-settings.json` is a hire's settings file, the fence, as this side writes it for a fixed
/// set of inputs: the guard's groups as it prints them, the hire, its profile (a hire's or the GM's), auto mode, the
/// support folder, the Claude accounts, the other hires and what sits in the support folder, and a test copy's
/// reporting hooks. The Go engine's test (desktop/hiresettings_fixture_test.go) makes the file from the same inputs
/// and must match it byte for byte, so a Mac hire gets the same fence whichever engine starts it.
/// Set SMOLCORP_WRITE_FIXTURES=1 for a run to write it again.
final class HireSettingsFixtureTests: XCTestCase {
    private let cli = "/Applications/Smolcorp.app/Contents/Resources/bin/smolcorp"

    /// The groups as `smolcorp guard --print` gives them: the defaults with one off and a pattern of your own
    /// (the guard file's own fixture), and a short set with rules a deny rule can't say.
    private func printed() throws -> [String] {
        let defaults = try String(contentsOf: FrontDeskFixtures.url("hire-guard.json"), encoding: .utf8)
        let short = """
            {"groups":[{"name":"push","on":true,"commands":["git push|send-pack"]},
                       {"name":"deploy","on":true,"commands":["vercel","eas build|submit","fly","docker buildx build|bake --push"]},
                       {"name":"cloud","on":false,"commands":["aws"]},
                       {"name":"remote","on":true,"commands":["rsync @remote","tmux ... send-keys"]}],
             "file":"x","problem":""}
            """
        return [defaults, short]
    }

    private func cases() throws -> [[String: Any]] {
        let groups = try printed()
        let reporting: [String: Any] = [
            "SessionStart": [["hooks": [["type": "command", "command": "'/Users/fixture/Library/Application Support/AgentDeck Test/report.sh' 7779", "async": true]]]],
            "PreToolUse": [["matcher": "*", "hooks": [["type": "command", "command": "'/x/report.sh' 7779", "async": true]]]],
        ]
        return [
            ["name": "a hire, the default groups", "hireID": "a1b2c3d4e5f6", "profile": "hire", "auto": true, "printed": groups[0],
             "support": "/Users/fixture/Library/Application Support/AgentDeck",
             "profiles": [["name": "default", "dir": "/Users/fixture/.claude"], ["name": "work", "dir": "/Users/fixture/.claude-work"]],
             "others": ["0f9e8d7c6b5a", "other11"], "entries": ["briefs", "config.json", "events", "hires", ".hidden", "recaps", "hook-key"]],
            ["name": "the General Manager", "hireID": "9a8b7c6d5e4f", "profile": "gm", "auto": true, "printed": groups[0],
             "support": "/Users/fixture/Library/Application Support/AgentDeck",
             "profiles": [["name": "default", "dir": "/Users/fixture/.claude"]], "others": [], "entries": ["hires"]],
            ["name": "no auto mode, a short set, a test copy's folder and reporting", "hireID": "c0ffee000001", "profile": "hire", "auto": false,
             "printed": groups[1], "support": "/Users/fixture/Library/Application Support/AgentDeck Test \"q\"",
             "profiles": [], "others": ["c0ffee000001"], "entries": [], "reporting": reporting],
        ]
    }

    /// The file this side writes for one case, as `HireSettings.save` encodes it.
    private func file(for given: [String: Any]) throws -> String {
        let support = URL(fileURLWithPath: try XCTUnwrap(given["support"] as? String))
        let profile = HireSettings.Profile(rawValue: try XCTUnwrap(given["profile"] as? String)) ?? .hire
        let rules = support.appendingPathComponent(HireSettings.guardFileName)
        let text = try XCTUnwrap(given["printed"] as? String)
        let groups = try XCTUnwrap(HireSettings.groups(tool: cli, rules: rules, profile: profile, run: { _, _ in (0, text) }))
        let profiles = try XCTUnwrap(given["profiles"] as? [[String: String]]).map { ClaudeProfile(name: $0["name"] ?? "", dir: $0["dir"] ?? "") }
        let deny = HireSettings.denyRules(hireID: try XCTUnwrap(given["hireID"] as? String), groups: groups, support: support, profiles: profiles,
                                          others: try XCTUnwrap(given["others"] as? [String]), entries: try XCTUnwrap(given["entries"] as? [String]))
        let object = HireSettings.settings(deny: deny, guardCommand: HireSettings.guardCommand(tool: cli, rules: rules, profile: profile),
                                           auto: try XCTUnwrap(given["auto"] as? Bool), reporting: given["reporting"] as? [String: Any])
        return String(decoding: try XCTUnwrap(HireSettings.encoded(object)), as: UTF8.self)
    }

    func testTheFixtureIsTheFenceThisSideWrites() throws {
        var written: [[String: Any]] = []
        for given in try cases() {
            var entry = given
            entry["file"] = try file(for: given)
            written.append(entry)
        }
        let data = try JSONSerialization.data(withJSONObject: ["cli": cli, "cases": written], options: [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes])
        let url = FrontDeskFixtures.url("hire-settings.json")
        if ProcessInfo.processInfo.environment["SMOLCORP_WRITE_FIXTURES"] == "1" { try data.write(to: url) }
        XCTAssertEqual(try Data(contentsOf: url), data, "hire-settings.json is out of date: run once with SMOLCORP_WRITE_FIXTURES=1")
    }

    /// The fixture is what a hire really gets: `write` makes the same file `encoded` gives, and the rules hold the secrets of both engines.
    func testTheFixturesEncodingIsTheSavedFile() throws {
        let root = URL(fileURLWithPath: HireDesk.physical(NSTemporaryDirectory())).appendingPathComponent("hiresettings-fixture-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: root) }
        let support = root.appendingPathComponent("support")
        try FileManager.default.createDirectory(at: support.appendingPathComponent("hires"), withIntermediateDirectories: true)
        let text = try printed()[1]
        let saved = try HireSettings.write(hireID: "abc123", cli: URL(fileURLWithPath: cli), support: support, profiles: [],
                                           run: { _, _ in (0, text) }, isExecutable: { _ in true })
        let object = try HireSettings.object(hireID: "abc123", cli: URL(fileURLWithPath: cli), support: support, profiles: [],
                                             run: { _, _ in (0, text) }, isExecutable: { _ in true })
        XCTAssertEqual(try Data(contentsOf: saved), HireSettings.encoded(object))
        let deny = try XCTUnwrap((object["permissions"] as? [String: Any])?["deny"] as? [String])
        let root2 = "//" + support.path.dropFirst()
        for name in ["gm-session", "hook-header", "hook-key", "window-key"] {
            XCTAssertTrue(deny.contains("Read(\(root2)/\(name))"), name)
            XCTAssertTrue(deny.contains("Edit(\(root2)/\(name))"), name)
        }
    }
}
