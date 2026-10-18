import AppKit

@main
enum AgentDeckApp {
    static func main() {
        let app = NSApplication.shared
        let delegate = AppDelegate()
        app.delegate = delegate
        app.setActivationPolicy(.accessory) // menu bar only, no Dock icon
        app.run()
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    private let deck = Deck()
    private lazy var menu = StatusMenu(deck: deck)
    private lazy var server = HTTPServer(port: Deck.port, router: DeckRouter(deck: deck))

    func applicationDidFinishLaunching(_ notification: Notification) {
        // A test copy leaves your agents' hooks and your login item to the copy in use.
        if !TestCopy.isOn {
            try? Hooks.refresh()
            Hooks.watchOpenCode()
            LoginItem.refresh()
        }
        server.onFailure = { [menu] _ in menu.showPortInUse() }
        server.onReady = { [menu] in menu.showListening() }
        server.start()

        if !Config.shared.hasCompletedSetup {
            NSWorkspace.shared.open(deck.setupURL())
        }
    }

    /// Opening the app while it runs — from Finder, Spotlight or Launchpad —
    /// would otherwise do nothing visible, since it has no windows.
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows: Bool) -> Bool {
        NSWorkspace.shared.open(deck.setupURL())
        return false
    }
}
