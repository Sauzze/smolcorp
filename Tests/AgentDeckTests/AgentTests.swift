import XCTest
@testable import AgentDeck

/// Telling an agent still runs in its tab from what `ps -o comm=` lists there.
final class AgentTests: XCTestCase {
    func testAnAgentRunningOnNodeOrBunIsStillThere() {
        // Claude Code from npm shows as node in its tab, Grok CLI as bun.
        XCTAssertTrue(Terminal.runs(any: Agent.claude.runningNames, among: "-zsh\n/usr/local/bin/node\n"))
        XCTAssertTrue(Terminal.runs(any: Agent.grok.runningNames, among: "-zsh\n/Users/someone/.bun/bin/bun\n"))
        XCTAssertTrue(Terminal.runs(any: Agent.claude.runningNames, among: "-zsh\nclaude\n"))
    }

    func testOnlyTheShellLeftIsGone() {
        XCTAssertFalse(Terminal.runs(any: Agent.claude.runningNames, among: "-zsh\n"))
        XCTAssertFalse(Terminal.runs(any: Agent.codex.runningNames, among: "login\n-zsh\n/usr/bin/less\n"))
    }
}
