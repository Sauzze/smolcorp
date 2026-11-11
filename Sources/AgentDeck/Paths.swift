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
              let port = UInt16(text), !livePorts.contains(port) else { return nil }
        return port
    }()
    /// The ports of the real app (7710), the General Manager's overseer (7720)
    /// and the one beside it (7730): a test copy never takes one.
    static let livePorts: Set<UInt16> = [7710, 7720, 7730]
    static let support: URL? = ProcessInfo.processInfo.environment["SMOLCORP_TEST_SUPPORT_DIR"]
        .flatMap { $0.isEmpty ? nil : URL(fileURLWithPath: $0, isDirectory: true) }
    static var isOn: Bool { port != nil }
    /// A test copy started with SMOLCORP_TEST_NO_TERMINAL_APP=1 asks Terminal.app nothing (see
    /// `Terminal.leavesAlone`), so it can run while Terminal is open without an Apple event to it:
    /// no Automation prompt, no wait on one. Its tmux desks need nothing from Terminal.
    static let leavesTerminalAppAlone = leavesTerminalAppAlone(testCopy: isOn, environment: ProcessInfo.processInfo.environment)
    static func leavesTerminalAppAlone(testCopy: Bool, environment: [String: String]) -> Bool {
        testCopy && environment["SMOLCORP_TEST_NO_TERMINAL_APP"] == "1"
    }

    /// A test copy, as it opens, writes a setup link to `setup-link` in its own folder (0600), so a
    /// tester's browser can open the same page the Settings window shows. The link works once, as
    /// the menu's do. The real app never writes one.
    static func leaveSetupLink(_ url: URL, into folder: URL? = support, when on: Bool = isOn) {
        guard on, let folder else { return }
        let file = folder.appendingPathComponent("setup-link")
        try? FileManager.default.removeItem(at: file)
        guard FileManager.default.createFile(atPath: file.path, contents: Data((url.absoluteString + "\n").utf8),
                                             attributes: [.posixPermissions: 0o600]) else { return }
    }

    /// What a test copy leaves to the copy in use.
    enum Refused: LocalizedError {
        case screenSaver, hiring

        var errorDescription: String? {
            switch self {
            case .screenSaver: return "This is a test copy of Smolcorp: the screen saver installed is the real one's."
            case .hiring: return "This is a test copy of Smolcorp: only Claude Code hires here, because another agent would report to the real one."
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

    /// Whether this is the test suite, not the app: XCTest is loaded only there.
    static let underTest = NSClassFromString("XCTestCase") != nil

    /// The test suite's own support folder, new for each run: a test never
    /// reads or writes the real one (the setup key, sessions, the event log,
    /// the config), even through a whole `Deck`.
    static let testSupport = FileManager.default.temporaryDirectory
        .appendingPathComponent("smolcorp-tests-\(ProcessInfo.processInfo.processIdentifier)-\(UUID().uuidString)", isDirectory: true)

    /// The real app's folder, as the app finds it: tests compare against it and never use it.
    static func realSupport() -> URL {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("AgentDeck", isDirectory: true)
    }

    static let support: URL = {
        if underTest {
            try? FileManager.default.createDirectory(at: testSupport, withIntermediateDirectories: true,
                                                     attributes: [.posixPermissions: 0o700])
            return testSupport
        }
        if TestCopy.isOn, let dir = TestCopy.support {
            try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
            return dir
        }
        let dir = realSupport()
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }()

    static let phones = support.appendingPathComponent("phones.json")
    static let config = support.appendingPathComponent("config.json")
    /// The groups of commands every hire is refused: see `GuardFile`.
    static let hireGuard = support.appendingPathComponent("hire-guard.json")
    /// The sessions running, and their keys, kept across a restart.
    static let sessions = support.appendingPathComponent("sessions.json")
    static let hookScript = support.appendingPathComponent("report.sh")
    /// Says Smolcorp turned Claude Code's voice on, so disconnecting knows it may turn it off.
    static let voiceMarker = support.appendingPathComponent("voice-set-by-smolcorp")
    /// Where a hire with a role leaves the few lines that start its agent,
    /// for its Terminal window to run once. Each removes itself as it runs.
    static let hires = support.appendingPathComponent("hires", isDirectory: true)
    /// A short recap of each hire fired, `<hire>.md`: see `HireDesk.fire`.
    static let recaps = support.appendingPathComponent("recaps", isDirectory: true)
    /// Where a hire's brief is written when its desk isn't pinned to its own
    /// project, so a shared folder such as ~/Projects stays clean: see `BriefFiles`.
    static let hireBriefs = support.appendingPathComponent("briefs", isDirectory: true)
    /// The briefs you can edit: the app's own, copied here on first use, and
    /// put back by "Restore defaults": see `BriefLibrary.editable`. Inside the
    /// folder hires' briefs are kept in, in a folder of its own.
    static let briefTemplates = hireBriefs.appendingPathComponent("templates", isDirectory: true)
    /// The hook's key, as the header curl sends: see `Access`.
    static let hookHeader = support.appendingPathComponent("hook-header")
    /// The setup page's key, new at every launch, for tools of your own on
    /// this Mac: see `DeckRouter`.
    static let setupKey = support.appendingPathComponent("setup-key")
    /// The General Manager's token, made when something first needs it: see `GMIdentity`.
    static let gmToken = support.appendingPathComponent("gm-token")
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
    /// The briefs that come with the app: see `BriefLibrary`.
    static let briefs = Bundle.main.resourceURL!.appendingPathComponent("briefs", isDirectory: true)
    /// The General Manager's plugin, bundled inside the app: see `GMPlugin`.
    static let gmPlugin = Bundle.main.resourceURL!.appendingPathComponent("gm/plugin", isDirectory: true)
    static let bundledHookScript = Bundle.main.resourceURL!.appendingPathComponent("report.sh")
    static let bundledOpenCodePlugin = Bundle.main.resourceURL!.appendingPathComponent("opencode-plugin.js")
    /// 1.7.2's plugin, the one OpenCode 1 loads: see `OpenCodeInstall`.
    static let bundledOpenCodePluginV1 = Bundle.main.resourceURL!.appendingPathComponent("opencode-plugin-v1.js")
    /// The `smolcorp` command-line tool inside the app (universal): see `TerminalCommand`.
    static let bundledCLI = Bundle.main.resourceURL!.appendingPathComponent("bin/smolcorp")
    static let bundledScreenSaver = Bundle.main.resourceURL!.appendingPathComponent("Smolcorp Office.saver")
}
