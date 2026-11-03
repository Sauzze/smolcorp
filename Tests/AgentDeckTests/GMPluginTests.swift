import XCTest
@testable import AgentDeck

/// The General Manager's plugin: its manifest, that everything it names is inside it,
/// and that the GM's start line loads it.
final class GMPluginTests: XCTestCase {
    private let plugin = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        .deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("Resources/gm/plugin")

    private func json(_ path: String) throws -> [String: Any] {
        let data = try Data(contentsOf: plugin.appendingPathComponent(path))
        return try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
    }

    private func frontMatter(_ path: String) throws -> [String: String] {
        let text = try String(contentsOf: plugin.appendingPathComponent(path), encoding: .utf8)
        let lines = text.components(separatedBy: "\n")
        XCTAssertEqual(lines.first, "---", path)
        let end = try XCTUnwrap(lines.dropFirst().firstIndex(of: "---"), path)
        var values: [String: String] = [:]
        for line in lines[1..<end] {
            guard let colon = line.firstIndex(of: ":") else { continue }
            values[String(line[..<colon])] = line[line.index(after: colon)...].trimmingCharacters(in: .whitespaces)
        }
        return values
    }

    func testTheManifestParsesAndNamesThePlugin() throws {
        let manifest = try json(".claude-plugin/plugin.json")
        XCTAssertEqual(manifest["name"] as? String, GMPlugin.name)
        XCTAssertFalse((manifest["description"] as? String ?? "").isEmpty)
        XCTAssertNotNil(manifest["version"] as? String)
    }

    func testTheToolsAreTheMCPServerRunByTheProgramTheLaunchNames() throws {
        let servers = try XCTUnwrap(json(".mcp.json")["mcpServers"] as? [String: Any])
        let server = try XCTUnwrap(servers["smolcorp"] as? [String: Any])
        XCTAssertEqual(server["command"] as? String, "${\(GMPlugin.cliEnvironment)}")
        XCTAssertEqual(server["args"] as? [String], ["mcp"])
        // No token or key is written in the plugin: the program has them from the GM's environment.
        XCTAssertEqual(servers.count, 1)
        XCTAssertNil(server["env"])
    }

    func testTheAgentAndEverySkillAreThereAndNamed() throws {
        let agent = try frontMatter("agents/\(GMPlugin.agent).md")
        XCTAssertEqual(agent["name"], GMPlugin.agent)
        XCTAssertFalse(agent["description"]?.isEmpty ?? true)
        for skill in ["run-task", "hourly-digest", "budget-request", "fire-and-recap"] {
            let values = try frontMatter("skills/\(skill)/SKILL.md")
            XCTAssertEqual(values["name"], skill)
            XCTAssertFalse(values["description"]?.isEmpty ?? true, skill)
        }
    }

    func testTheRulesKeepTheHardLines() throws {
        let text = try String(contentsOf: plugin.appendingPathComponent("agents/\(GMPlugin.agent).md"), encoding: .utf8)
        for line in ["Hire on Sonnet", "independent reviewer", "Answer a permission prompt", "Push, publish, deploy", "NEEDS-REVIEW", "BUDGET REQUEST",
                     "`PLAYBOOK.md` in your own folder", "never in Smolcorp's support folder"] {
            XCTAssertTrue(text.lowercased().contains(line.lowercased()), line)
        }
        XCTAssertTrue(FileManager.default.fileExists(atPath: plugin.appendingPathComponent("playbook-start.md").path))
    }

    func testEveryFileOfThePluginStaysInsideIt() throws {
        let root = plugin.resolvingSymlinksInPath().path
        let files = try XCTUnwrap(FileManager.default.enumerator(at: plugin, includingPropertiesForKeys: nil))
        var count = 0
        for case let file as URL in files {
            XCTAssertTrue(file.resolvingSymlinksInPath().path.hasPrefix(root + "/"), file.path)
            count += 1
        }
        XCTAssertGreaterThan(count, 8)
        // The app's build copies the whole of Resources/gm.
        let build = try String(contentsOf: plugin.deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("scripts/build-app.sh"), encoding: .utf8)
        XCTAssertTrue(build.contains("cp -R Resources/gm \"$APP/Contents/Resources/\""))
    }

    func testTheBundledLaunchNeedsTheManifestAndTheProgram() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let cli = folder.appendingPathComponent("bin/smolcorp")
        XCTAssertNil(GMPlugin.Launch.bundled(plugin: folder, cli: cli))
        let found = GMPlugin.Launch.bundled(plugin: plugin, cli: cli)
        XCTAssertEqual(found, GMPlugin.Launch(plugin: plugin.path, cli: nil))
        XCTAssertTrue(found?.note.contains("not available") ?? false)
        try FileManager.default.createDirectory(at: cli.deletingLastPathComponent(), withIntermediateDirectories: true)
        try "#!/bin/sh\n".write(to: cli, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cli.path)
        let there = GMPlugin.Launch.bundled(plugin: plugin, cli: cli)
        XCTAssertEqual(there?.cli, cli.path)
        XCTAssertEqual(there?.note, "")
    }

    func testTheGMsStartLineLoadsThePluginAndItsAgent() throws {
        let gm = try XCTUnwrap(Role.shipped.first { $0.id == GMPlugin.role })
        let launch = GMPlugin.Launch(plugin: "/Apps/Smolcorp's.app/Contents/Resources/gm/plugin", cli: "/Apps/Smolcorp's.app/Contents/Resources/bin/smolcorp")
        let script = gm.startScript(firstMessage: "Go.", plugin: launch)
        XCTAssertTrue(script.contains("'--plugin-dir=/Apps/Smolcorp'\\''s.app/Contents/Resources/gm/plugin'"), script)
        XCTAssertTrue(script.contains("'--agent=smolcorp-gm:general-manager'"), script)
        XCTAssertTrue(script.contains("export SMOLCORP_CLI='/Apps/Smolcorp'\\''s.app/Contents/Resources/bin/smolcorp'\n"), script)
        XCTAssertTrue(script.range(of: "'--agent=")!.lowerBound < script.range(of: "'--' 'Go.'")!.lowerBound)
        // Without the program, the variable is cleared so no other one is run in its place.
        let without = gm.startScript(firstMessage: "Go.", plugin: GMPlugin.Launch(plugin: "/p", cli: nil))
        XCTAssertTrue(without.contains("unset SMOLCORP_CLI\n"))
        XCTAssertTrue(without.contains("'--plugin-dir=/p'"))
    }

    func testOtherRolesAndAgentsAreLeftAlone() throws {
        let launch = GMPlugin.Launch(plugin: "/p", cli: "/c")
        var codex = try XCTUnwrap(Role.shipped.first { $0.id == GMPlugin.role })
        codex.agent = "codex"
        XCTAssertFalse(codex.startScript(firstMessage: "Go.", plugin: launch).contains("--plugin-dir"))
        for role in Role.shipped where role.id != GMPlugin.role {
            let script = role.startScript(firstMessage: "Go.")
            XCTAssertFalse(script.contains("--plugin-dir") || script.contains("SMOLCORP_CLI"), role.id)
        }
    }

    func testTheReviewerBriefAsksForAVerdictAndExactFixes() throws {
        let brief = try String(contentsOf: plugin.deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("briefs/reviewer.md"), encoding: .utf8)
        XCTAssertTrue(brief.contains("read_only: true"))
        XCTAssertTrue(brief.contains("`PASS`"))
        XCTAssertTrue(brief.contains("`FAIL`"))
        XCTAssertTrue(brief.contains("**exact fix**"))
    }
}
