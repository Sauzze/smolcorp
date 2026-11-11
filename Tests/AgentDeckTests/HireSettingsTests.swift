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
