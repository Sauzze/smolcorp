import Foundation
import os

/// Where AgentDeck reads and writes. Its own state lives in
/// ~/Library/Application Support/AgentDeck, so the app can be moved or
/// replaced without losing the paired phones or the deck's layout.
/// A test copy of the app, for trying a build beside the one in use: started
/// with SMOLCORP_TEST_PORT and SMOLCORP_TEST_SUPPORT_DIR (scripts/test-copy.sh),
/// it listens on its own port, on this Mac only, keeps its own settings and
/// keys, and changes nothing outside its folder: no hooks in your agents'
/// settings, no login item, no window or alert by itself. Without both, the
/// app is itself.
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

    /// A test copy runs beside the owner's own work, hidden (scripts/test-copy.sh opens it with
    /// `open -g -j`): it never comes in front of other apps, takes the keyboard, or puts up a window
    /// or an alert by itself (first-run Settings, the engine-stopped alert, a page's Open the office
    /// or folder panel). In a debug build each is written to its log instead (subsystem io.github.sauzze.agentdeck,
    /// category test-copy, readable: `log show --predicate 'category == "test-copy"'`), and this
    /// says no (a debug build also writes it to its log, below). The real app: yes.
    static func mayComeForward(_ what: String, testCopy: Bool = isOn,
                               log: (String) -> Void = quietLogLine) -> Bool {
        guard testCopy else { return true }
        log("Smolcorp Test: \(what) not shown, a test copy never comes forward by itself")
        return false
    }

    /// A test copy's line in the system log, written only by a debug build: the Mac app logs nothing in a release
    /// build (scripts/check-public.sh). A test copy from scripts/test-copy.sh is a release build, so it says no
    /// without a line; a debug run of the app writes it.
    private static func quietLogLine(_ line: String) {
        #if DEBUG
        Logger(subsystem: "io.github.sauzze.agentdeck", category: "test-copy").notice("\(line, privacy: .public)")
        #endif
    }

    /// Whether SMOLCORP_TEST_LISTEN says "lan", as the Go engine reads it: spaces around it and the
    /// letter case don't matter (RTCH N3), so both engines and scripts/test-copy.sh agree.
    static func wantsLAN(_ environment: [String: String]) -> Bool {
        environment["SMOLCORP_TEST_LISTEN"]?.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() == "lan"
    }

    /// What a test copy never does to another app (RTCH N1): a bridge action that would bring
    /// a terminal app, T3 Code, the Grok app, System Settings or a system prompt in front of the owner's
    /// work, or touch the owner's music or microphone (RWTC N2), answers with the sentence below instead.
    /// The real app: nothing is refused.
    enum OtherApp: CaseIterable {
        case terminal, tiler, grok, t3Code, accessibility, fix, music, microphone

        var sentence: String {
            switch self {
            case .terminal: return "This is a test copy of Smolcorp: it never brings a terminal app forward, opens a window in one or touches its tabs."
            case .tiler: return "This is a test copy of Smolcorp: it never moves or tiles the windows of other apps."
            case .grok: return "This is a test copy of Smolcorp: it never opens the Grok app."
            case .t3Code: return "This is a test copy of Smolcorp: it never opens T3 Code."
            case .accessibility: return "This is a test copy of Smolcorp: it never asks macOS for Accessibility or opens System Settings."
            case .fix: return "This is a test copy of Smolcorp: it never opens another app, a download page or System Settings to fix something."
            case .music: return "This is a test copy of Smolcorp: it never plays or skips your music, or changes the volume."
            case .microphone: return "This is a test copy of Smolcorp: it never asks for the microphone or turns dictation on."
            }
        }
    }

    /// The sentence a test copy answers when asked to bring `app` forward, or nil for the real app.
    static func refusal(for app: OtherApp, testCopy: Bool = isOn) -> String? {
        testCopy ? app.sentence : nil
    }

    /// The sentence a test copy answers when asked to do anything in `tab` (RWTC N1): keys, text, a paste, a
    /// scroll, a look at its screen, a close. Only a tmux pane on one of the copy's own hire sockets is its own;
    /// any other tab is in one of the owner's terminal apps, or on a tmux server of theirs. Nil for the real app.
    static func refusal(forTab tab: String, testCopy: Bool = isOn, testPort: UInt16? = port) -> String? {
        guard testCopy else { return nil }
        if let address = TabAddress(tab), address.kind == "tmux", !address.socket.isEmpty,
           HireTmux.owns(address.socket, testPort: testPort) { return nil }
        return OtherApp.terminal.sentence
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

    static let bundledHookScript = Bundle.main.resourceURL!.appendingPathComponent("report.sh")
    static let bundledOpenCodePlugin = Bundle.main.resourceURL!.appendingPathComponent("opencode-plugin.js")
    /// 1.7.2's plugin, the one OpenCode 1 loads: see `OpenCodeInstall`.
    static let bundledOpenCodePluginV1 = Bundle.main.resourceURL!.appendingPathComponent("opencode-plugin-v1.js")
    /// The `smolcorp` command-line tool inside the app (universal): see `TerminalCommand`.
    static let bundledCLI = Bundle.main.resourceURL!.appendingPathComponent("bin/smolcorp")
    static let bundledScreenSaver = Bundle.main.resourceURL!.appendingPathComponent("Smolcorp Office.saver")
}
