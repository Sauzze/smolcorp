import Foundation

/// The General Manager's plugin for Claude Code: its rules (an agent), its skills and its tools
/// (the `smolcorp mcp` server), bundled in the app at `Resources/gm/plugin` and loaded by the GM's
/// start line with `--plugin-dir`. Its playbook is a file the GM keeps in its own folder, never
/// in Smolcorp's support folder.
enum GMPlugin {
    /// The role that gets it.
    static let role = "gm"
    /// The plugin's name in its manifest, and the agent that is the GM's rules.
    static let name = "smolcorp-gm"
    static let agent = "general-manager"
    /// Where the plugin's tools find the command line program: set by the launch, only when the program is there.
    static let cliEnvironment = "SMOLCORP_CLI"

    /// How the GM's start line loads the plugin, and what it sets for it.
    struct Launch: Equatable {
        /// The plugin's folder.
        var plugin: String
        /// The command line program the plugin's tools run, or nil if the app has none: the tools are then left
        /// without it (Claude Code reports the server as failed and goes on), and the GM is told.
        var cli: String?

        /// The agent's own words: the plugin folder, and its agent as the session's.
        var arguments: [String] { ["--plugin-dir=\(plugin)", "--agent=\(GMPlugin.name):\(GMPlugin.agent)"] }

        /// Shell lines for the start script: the program's path (and nothing, if there is none, even from the terminal's).
        var environment: String {
            guard let cli else { return "unset \(GMPlugin.cliEnvironment)\n" }
            return "export \(GMPlugin.cliEnvironment)=\(Self.quoted(cli))\n"
        }

        /// What to tell the GM about its tools, or "" when they are there.
        var note: String {
            cli == nil ? " The `smolcorp` tools are not available in this copy of Smolcorp: say so to the owner, and use the command `smolcorp gm` only if it is on your PATH." : ""
        }

        /// The plugin and program bundled in the app; nil if the plugin isn't there (a build run from the source folder).
        static func bundled(fileManager: FileManager = .default, plugin: URL = Paths.gmPlugin, cli: URL = Paths.bundledCLI) -> Launch? {
            guard fileManager.fileExists(atPath: plugin.appendingPathComponent(".claude-plugin/plugin.json").path) else { return nil }
            return Launch(plugin: plugin.path, cli: fileManager.isExecutableFile(atPath: cli.path) ? cli.path : nil)
        }

        private static func quoted(_ word: String) -> String { "'" + word.replacingOccurrences(of: "'", with: "'\\''") + "'" }
    }
}
