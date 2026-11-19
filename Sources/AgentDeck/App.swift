import AppKit

@main
enum AgentDeckApp {
    static func main() {
        // The whole setup from a terminal, checked and put right where it can be, without starting a second app.
        if CommandLine.arguments.contains("--setup") {
            exit(SetupCommand.run(arguments: CommandLine.arguments))
        }
        let app = NSApplication.shared
        // The engine: the Go core, with this app as its shell. An engine config.json still names is logged once, and ignored.
        if let ignored = Engine.ignored(Config.shared.engine) { FileHandle.standardError.write(Data((ignored + "\n").utf8)) }
        let delegate = GoShellDelegate()
        app.delegate = delegate
        // In the Dock, or the menu bar only, as set up, from the start.
        app.setActivationPolicy(Dock.policy(showing: Config.shared.showInDock))
        app.run()
    }
}

/// Smolcorp in the Dock and ⌘-Tab, as a full app, or in the menu bar only,
/// as setup's Show in Dock says. The menu bar's item is there either way.
enum Dock {
    /// A test copy is never in the Dock or ⌘-Tab, whatever its settings say (RTCH N6): the Dock icon would be a
    /// second Smolcorp beside the one in use.
    static func policy(showing: Bool, testCopy: Bool = TestCopy.isOn) -> NSApplication.ActivationPolicy {
        showing && !testCopy ? .regular : .accessory
    }

    /// At once: from the settings window, it stays in front, with the menus
    /// it now has or hasn't; from the browser, Smolcorp stays behind.
    static func show(_ showing: Bool) {
        let policy = policy(showing: showing)
        guard NSApp.activationPolicy() != policy else { return }
        let wasActive = NSApp.isActive
        NSApp.setActivationPolicy(policy)
        // macOS puts the app's menus in the menu bar, and keeps its windows forward, once it's activated again.
        if wasActive { DispatchQueue.main.async { Foreground.activate("the Dock's menus") } }
    }
}

/// Smolcorp in front of other apps, with the keyboard, as a menu bar app is
/// never by itself: every activation goes through here, so a test copy only
/// logs it (see `TestCopy.mayComeForward`).
enum Foreground {
    static func activate(_ what: String) {
        guard TestCopy.mayComeForward(what) else { return }
        NSApp.activate(ignoringOtherApps: true)
    }
}
