import XCTest
@testable import AgentDeck

/// A manager hire's own settings file: its content, its mode, what stays writable, and the refusals.
/// The guard is a stand-in that prints the groups; nothing real is run.
final class HireSettingsTests: XCTestCase {
    private var root: URL!
    private var support: URL!
    private var work: ClaudeProfile!

    override func setUpWithError() throws {
        root = URL(fileURLWithPath: HireDesk.physical(NSTemporaryDirectory())).appendingPathComponent("hiresettings-\(UUID().uuidString)")
        support = root.appendingPathComponent("support")
        try FileManager.default.createDirectory(at: support.appendingPathComponent("hires/other11"), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: support.appendingPathComponent("events"), withIntermediateDirectories: true)
        work = ClaudeProfile(name: "work", dir: root.appendingPathComponent("claude-work").path)
    }

    override func tearDownWithError() throws { try? FileManager.default.removeItem(at: root) }

    private func json(_ url: URL) throws -> [String: Any] {
        try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])
    }

    func testOnlyAClaudeHireWithSettingsStartsWithThem() {
        var role = Role.shipped[0]
        role.agent = Agent.claude.rawValue
        let settings = "/s/hires/abc123/settings.json"
        XCTAssertTrue(role.startScript(firstMessage: "go", settings: settings).contains("'claude' '--settings=\(settings)'"))
        XCTAssertFalse(role.startScript(firstMessage: "go").contains("--settings"), "an unmanaged hire gets nothing new")
        role.agent = Agent.codex.rawValue
        XCTAssertFalse(role.startScript(firstMessage: "go", settings: settings).contains("--settings"))
    }

    func testATestCopysHireIsStartedWithoutTheAccountsSettings() {
        var role = Role.shipped[0]
        role.agent = Agent.claude.rawValue
        let script = role.startScript(firstMessage: "go", settings: "/s/settings.json", testCopy: true)
        XCTAssertTrue(script.contains("'--settings=/s/settings.json' '--setting-sources=project,local'"))
        role.agent = Agent.codex.rawValue
        XCTAssertFalse(role.startScript(firstMessage: "go", testCopy: true).contains("setting-sources"), "only Claude Code takes it")
    }

    func testTheReportScriptSendsToTheCopysPortAndNeverTheRealOne() throws {
        let template = root.appendingPathComponent("report-template.sh")
        try "curl http://127.0.0.1:{{PORT}}/hook".write(to: template, atomically: true, encoding: .utf8)
        let target = support.appendingPathComponent("report.sh")
        try Hooks.writeScript(template: template, to: target, port: 7751)
        let text = try String(contentsOf: target, encoding: .utf8)
        XCTAssertTrue(text.contains(":7751/"))
        XCTAssertFalse(text.contains("7710"))
        let hooks = Hooks.hireHooks(script: target)
        XCTAssertFalse(String(data: try JSONSerialization.data(withJSONObject: hooks), encoding: .utf8)!.contains("7710"))
    }
}
