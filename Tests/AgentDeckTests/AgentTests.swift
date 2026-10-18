import XCTest
@testable import AgentDeck

/// Telling an agent from what else may run in its tab, by its command line.
final class AgentTests: XCTestCase {
    func testAnAgentIsKnownByItsOwnProgramOrPackage() {
        XCTAssertTrue(Agent.claude.isItself(commandLine: "claude --resume"))
        XCTAssertTrue(Agent.claude.isItself(commandLine: "/opt/homebrew/bin/node /opt/homebrew/lib/node_modules/@anthropic-ai/claude-code/cli.js"))
        XCTAssertTrue(Agent.claude.isItself(commandLine: "node /Users/Jean Dupont/.npm-global/lib/node_modules/@anthropic-ai/claude-code/cli.js -c"),
                      "a home folder with a space")
        XCTAssertTrue(Agent.codex.isItself(commandLine: "node /usr/local/lib/node_modules/@openai/codex/bin/codex.js"))
        XCTAssertTrue(Agent.gemini.isItself(commandLine: "node /opt/homebrew/lib/node_modules/@google/gemini-cli/dist/index.js"))
        XCTAssertTrue(Agent.grok.isItself(commandLine: "bun /Users/me/.bun/install/global/node_modules/@vibe-kit/grok-cli/dist/index.js"))
        XCTAssertTrue(Agent.opencode.isItself(commandLine: "opencode"))
    }

    /// After the agent crashed, what was started in its tab is someone's work.
    func testNodeOrBunRunningSomethingElseIsntTheAgent() {
        for line in ["node /Users/me/claude-app/node_modules/.bin/vite", "node /opt/homebrew/lib/node_modules/npm/bin/npm-cli.js run dev",
                     "node /Users/me/claude-app/node_modules/next/dist/bin/next dev", "node server.js", "bun run dev", "node", "-zsh",
                     "/usr/bin/less README.md"] {
            XCTAssertFalse(Agent.claude.isItself(commandLine: line), line)
            XCTAssertFalse(Agent.gemini.isItself(commandLine: line), line)
        }
    }
}
