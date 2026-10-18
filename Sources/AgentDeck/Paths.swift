import Foundation

/// Where AgentDeck reads and writes. Its own state lives in
/// ~/Library/Application Support/AgentDeck, so the app can be moved or
/// replaced without losing the paired phones or the deck's layout.
/// A test copy of the app, for trying a build beside the one in use: started
/// with SMOLCORP_TEST_PORT and SMOLCORP_TEST_SUPPORT_DIR (scripts/test-copy.sh),
/// it listens on its own port, keeps its own settings and keys, and changes
/// nothing outside its folder: no hooks in your agents' settings, no login
/// item. Without both, the app is itself.
enum TestCopy {
    static let port: UInt16? = {
        guard support != nil, let text = ProcessInfo.processInfo.environment["SMOLCORP_TEST_PORT"],
              let port = UInt16(text), port != 7710 else { return nil }
        return port
    }()
    static let support: URL? = ProcessInfo.processInfo.environment["SMOLCORP_TEST_SUPPORT_DIR"]
        .flatMap { $0.isEmpty ? nil : URL(fileURLWithPath: $0, isDirectory: true) }
    static var isOn: Bool { port != nil }

    /// What a test copy leaves to the copy in use.
    enum Refused: LocalizedError {
        case screenSaver, hiring

        var errorDescription: String? {
            switch self {
            case .screenSaver: return "This is a test copy of Smolcorp: the screen saver installed is the real one's."
            case .hiring: return "This is a test copy of Smolcorp: a hire's agent would report to the real one, so it doesn't hire."
            }
        }
    }
}

enum Paths {
    /// Writes `data` to `url` readable by you alone, whole or not at all: to
    /// a file beside it, then put in its place, so a crash or a full disk
    /// midway leaves the old one as it was. Says whether it was written.
    @discardableResult
    static func writePrivately(_ data: Data, to url: URL) -> Bool {
        let temporary = url.deletingLastPathComponent().appendingPathComponent(".\(url.lastPathComponent).\(UUID().uuidString)")
        guard FileManager.default.createFile(atPath: temporary.path, contents: data, attributes: [.posixPermissions: 0o600]) else { return false }
        guard rename(temporary.path, url.path) == 0 else {
            try? FileManager.default.removeItem(at: temporary)
            return false
        }
        return true
    }

    static let support: URL = {
        if TestCopy.isOn, let dir = TestCopy.support {
            try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
            return dir
        }
        let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        let dir = base.appendingPathComponent("AgentDeck", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }()

    static let phones = support.appendingPathComponent("phones.json")
    static let config = support.appendingPathComponent("config.json")
    /// The sessions running, and their keys, kept across a restart.
    static let sessions = support.appendingPathComponent("sessions.json")
    static let hookScript = support.appendingPathComponent("report.sh")
    /// Where a hire with a role leaves the few lines that start its agent,
    /// for its Terminal window to run once. Each removes itself as it runs.
    static let hires = support.appendingPathComponent("hires", isDirectory: true)
    /// The hook's key, as the header curl sends: see `Access`.
    static let hookHeader = support.appendingPathComponent("hook-header")
    /// The setup page's key, new at every launch, for tools of your own on
    /// this Mac: see `DeckRouter`.
    static let setupKey = support.appendingPathComponent("setup-key")
    /// 1.0's single key, shared by the phone and the hook. Deleted at launch.
    static let legacyToken = support.appendingPathComponent("token")

    static let home = FileManager.default.homeDirectoryForCurrentUser
    static let claudeSettings = home.appendingPathComponent(".claude/settings.json")
    static let codexHooks = home.appendingPathComponent(".codex/hooks.json")
    static let geminiSettings = home.appendingPathComponent(".gemini/settings.json")
    static let grokHooks = home.appendingPathComponent(".grok/hooks/smolcorp.json")
    static let openCodePlugin = home.appendingPathComponent(".config/opencode/plugins/smolcorp.js")
    /// Grok's and OpenCode's files under the name the app had before, moved to the new ones.
    static let oldGrokHooks = home.appendingPathComponent(".grok/hooks/sauzzedeck-pro.json")
    static let oldOpenCodePlugin = home.appendingPathComponent(".config/opencode/plugins/sauzzedeck-pro.js")
    static let launchAgent = home.appendingPathComponent("Library/LaunchAgents/\(LoginItem.label).plist")
    /// Where macOS finds screen savers installed for you alone.
    static let installedScreenSaver = home.appendingPathComponent("Library/Screen Savers/Smolcorp Office.saver")
    /// The screen saver as it was called before, taken out when the new one goes in.
    static let oldScreenSaver = home.appendingPathComponent("Library/Screen Savers/SauzzeDeck Office.saver")

    /// The phone and setup pages, bundled inside the app.
    static let web = Bundle.main.resourceURL!.appendingPathComponent("web", isDirectory: true)
    static let bundledHookScript = Bundle.main.resourceURL!.appendingPathComponent("report.sh")
    static let bundledOpenCodePlugin = Bundle.main.resourceURL!.appendingPathComponent("opencode-plugin.js")
    /// 1.7.2's plugin, the one OpenCode 1 loads: see `OpenCodeInstall`.
    static let bundledOpenCodePluginV1 = Bundle.main.resourceURL!.appendingPathComponent("opencode-plugin-v1.js")
    static let bundledScreenSaver = Bundle.main.resourceURL!.appendingPathComponent("Smolcorp Office.saver")
}
