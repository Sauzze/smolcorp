import XCTest
@testable import AgentDeck

/// `Tests/fixtures/frontdesk/t3-fence.json` is a hire in T3 Code's fence, `<worktree>/.claude/settings.local.json`, as this
/// side writes it (`T3Desk.writeFence`) for a fixed set of inputs: the guard's groups as it prints them, the hire, the support
/// folder, the Claude accounts, the other hires and what sits in the support folder, auto mode or not. The Go engine's lane
/// (desktop/t3desk_test.go) writes the fence from the same inputs and must match it byte for byte (M5a), so a hire in T3 Code
/// gets the same fence whichever engine hires it. Set SMOLCORP_WRITE_FIXTURES=1 for a run to write it again.
final class T3FenceFixtureTests: XCTestCase {
    private let cli = "/Applications/Smolcorp.app/Contents/Resources/bin/smolcorp"

    private func cases() throws -> [[String: Any]] {
        let defaults = try String(contentsOf: FrontDeskFixtures.url("hire-guard.json"), encoding: .utf8)
        let short = #"{"groups":[{"name":"push","on":true,"commands":["git push|send-pack"]},{"name":"cloud","on":false,"commands":["aws"]}],"file":"x","problem":""}"#
        return [
            ["name": "a hire on a Mac: the default groups, auto mode, two accounts", "hireID": "a1b2c3d4e5f6", "auto": true, "printed": defaults,
             "support": "/Users/fixture/Library/Application Support/AgentDeck",
             "profiles": [["name": "default", "dir": "/Users/fixture/.claude"], ["name": "work", "dir": "/Users/fixture/.claude-work"]],
             "others": ["0f9e8d7c6b5a"], "entries": ["briefs", "config.json", "hires", ".hidden", "recaps"]],
            ["name": "a hire on Linux: no auto mode, the default account, a folder with a quote", "hireID": "c0ffee000001", "auto": false,
             "printed": short, "support": "/home/fixture/.config/smolcorp \"q\"",
             "profiles": [["name": "default", "dir": "/home/fixture/.claude"]], "others": [], "entries": ["hires"]],
        ]
    }

    /// The fence this side writes for one case: the settings `HireSettings.object` makes from these inputs, through `writeFence`.
    private func fence(for given: [String: Any], in folder: URL) throws -> String {
        let support = URL(fileURLWithPath: try XCTUnwrap(given["support"] as? String))
        let rules = support.appendingPathComponent(HireSettings.guardFileName)
        let text = try XCTUnwrap(given["printed"] as? String)
        let groups = try XCTUnwrap(HireSettings.groups(tool: cli, rules: rules, run: { _, _ in (0, text) }))
        let profiles = try XCTUnwrap(given["profiles"] as? [[String: String]]).map { ClaudeProfile(name: $0["name"] ?? "", dir: $0["dir"] ?? "") }
        let deny = HireSettings.denyRules(hireID: try XCTUnwrap(given["hireID"] as? String), groups: groups, support: support, profiles: profiles,
                                          others: try XCTUnwrap(given["others"] as? [String]), entries: try XCTUnwrap(given["entries"] as? [String]))
        let object = HireSettings.settings(deny: deny, guardCommand: HireSettings.guardCommand(tool: cli, rules: rules), auto: try XCTUnwrap(given["auto"] as? Bool))
        let file = folder.appendingPathComponent(T3Desk.fenceName)
        XCTAssertTrue(T3Desk.writeFence(object, to: file))
        return String(decoding: try Data(contentsOf: file), as: UTF8.self)
    }

    func testTheFixtureIsTheFenceThisSideWrites() throws {
        let root = URL(fileURLWithPath: HireDesk.physical(NSTemporaryDirectory())).appendingPathComponent("t3fence-fixture-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: root) }
        var written: [[String: Any]] = []
        for (i, given) in try cases().enumerated() {
            var entry = given
            entry["fence"] = try fence(for: given, in: root.appendingPathComponent("wt\(i)"))
            written.append(entry)
        }
        let data = try JSONSerialization.data(withJSONObject: ["cli": cli, "fenceName": T3Desk.fenceName, "cases": written],
                                              options: [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes])
        let url = FrontDeskFixtures.url("t3-fence.json")
        if ProcessInfo.processInfo.environment["SMOLCORP_WRITE_FIXTURES"] == "1" { try data.write(to: url) }
        XCTAssertEqual(try Data(contentsOf: url), data, "t3-fence.json is out of date: run once with SMOLCORP_WRITE_FIXTURES=1")
    }
}
