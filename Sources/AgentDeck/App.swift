import AppKit
import Carbon.HIToolbox

@main
enum AgentDeckApp {
    static func main() {
        // The whole setup from a terminal, checked and put right where it can be, without starting a second app.
        if CommandLine.arguments.contains("--setup") {
            exit(SetupCommand.run(arguments: CommandLine.arguments))
        }
        #if DEBUG
        // The portable office's crypto and redaction, checked without starting the app.
        if CommandLine.arguments.contains("--viewer-selftest") {
            exit(ViewerSelfTest.run(arguments: CommandLine.arguments))
        }
        #endif
        let app = NSApplication.shared
        let delegate = AppDelegate()
        app.delegate = delegate
        // In the Dock, or the menu bar only, as set up, from the start.
        app.setActivationPolicy(Dock.policy(showing: Config.shared.showInDock))
        app.run()
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    private let deck = Deck()
    private lazy var menu = StatusMenu(deck: deck)
    private lazy var router = DeckRouter(deck: deck)
    private lazy var server = HTTPServer(port: Deck.port, router: router)
    private lazy var office = OfficeWindow(address: URL(string: "http://127.0.0.1:\(Deck.port)/office?window&t=\(deck.windowKey)")!)
    private var officeKey: HotKey?
    private lazy var settings = SettingsWindow(setupKey: deck.access.setupKey)

    func applicationDidFinishLaunching(_ notification: Notification) {
        // A test copy leaves your agents' hooks and your login item to the copy in use.
        if !TestCopy.isOn {
            try? Hooks.refresh()
            Hooks.watchOpenCode()
            LoginItem.refresh()
        }
        server.onFailure = { [menu] _ in menu.showPortInUse() }
        server.onReady = { [menu] in menu.showListening() }
        server.service = PhoneApp.service
        server.start()
        menu.office = office
        menu.settings = settings
        router.showOffice = { [office] in office.show() }
        NSApp.mainMenu = mainMenu()
        // ⌥⌘O, from anywhere, shows and hides the office's window.
        officeKey = HotKey(keyCode: UInt32(kVK_ANSI_O), modifiers: UInt32(cmdKey | optionKey), id: 1) { [office] in office.toggle() }

        if !Config.shared.hasCompletedSetup {
            settings.show()
        }
    }

    /// The portable office stops on quit, and the viewer is asked to forget it, for up to 5 s.
    func applicationWillTerminate(_ notification: Notification) {
        deck.portable.stop(waitingUpTo: PortableOffice.quitWait)
    }

    /// A click on the Dock icon, or the app opened again from Finder,
    /// Spotlight or Launchpad: the office if it's hidden, and with the
    /// office showing, Settings.
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows: Bool) -> Bool {
        office.isShown ? settings.show() : office.show()
        return false
    }

    /// The Dock icon's own menu.
    func applicationDockMenu(_ sender: NSApplication) -> NSMenu? {
        let menu = NSMenu()
        add(to: menu, "Show Office", #selector(toggleOffice)).state = office.isShown ? .on : .off
        add(to: menu, "Settings…", #selector(openSettings))
        add(to: menu, "Pair a Phone…", #selector(pairPhone))
        return menu
    }

    // MARK: The menu bar's menus, while Smolcorp is in the Dock

    /// Smolcorp's, Edit and Window. In the menu bar only, they aren't shown,
    /// but their keys still work in its windows.
    private func mainMenu() -> NSMenu {
        let bar = NSMenu()
        let app = submenu(of: bar, "Smolcorp")
        add(to: app, "About Smolcorp", #selector(about))
        app.addItem(.separator())
        add(to: app, "Settings…", #selector(openSettings), key: ",")
        app.addItem(.separator())
        add(to: app, "Hide Smolcorp", #selector(NSApplication.hide(_:)), key: "h", target: NSApp)
        add(to: app, "Hide Others", #selector(NSApplication.hideOtherApplications(_:)), key: "h", target: NSApp)
            .keyEquivalentModifierMask = [.command, .option]
        add(to: app, "Show All", #selector(NSApplication.unhideAllApplications(_:)), target: NSApp)
        app.addItem(.separator())
        add(to: app, "Quit Smolcorp", #selector(NSApplication.terminate(_:)), key: "q", target: NSApp)

        // To whatever has the keyboard: the settings page's fields among them.
        let edit = submenu(of: bar, "Edit")
        forward(to: edit, "Undo", Selector(("undo:")), key: "z")
        forward(to: edit, "Redo", Selector(("redo:")), key: "z").keyEquivalentModifierMask = [.command, .shift]
        edit.addItem(.separator())
        forward(to: edit, "Cut", #selector(NSText.cut(_:)), key: "x")
        forward(to: edit, "Copy", #selector(NSText.copy(_:)), key: "c")
        forward(to: edit, "Paste", #selector(NSText.paste(_:)), key: "v")
        forward(to: edit, "Select All", #selector(NSText.selectAll(_:)), key: "a")

        let window = submenu(of: bar, "Window")
        // ⌥⌘O is the office's key from anywhere already, so it isn't given again here.
        add(to: window, "Office", #selector(toggleOffice))
        add(to: window, "Settings", #selector(openSettings))
        window.addItem(.separator())
        forward(to: window, "Minimize", #selector(NSWindow.performMiniaturize(_:)), key: "m")
        forward(to: window, "Close", #selector(NSWindow.performClose(_:)), key: "w")
        NSApp.windowsMenu = window
        return bar
    }

    private func submenu(of bar: NSMenu, _ title: String) -> NSMenu {
        let menu = NSMenu(title: title)
        bar.addItem(withTitle: title, action: nil, keyEquivalent: "").submenu = menu
        return menu
    }

    /// This app's own, or `target`'s.
    @discardableResult
    private func add(to menu: NSMenu, _ title: String, _ action: Selector, key: String = "", target: AnyObject? = nil) -> NSMenuItem {
        let item = menu.addItem(withTitle: title, action: action, keyEquivalent: key)
        item.target = target ?? self
        return item
    }

    /// Sent to whatever has the keyboard.
    @discardableResult
    private func forward(to menu: NSMenu, _ title: String, _ action: Selector, key: String) -> NSMenuItem {
        menu.addItem(withTitle: title, action: action, keyEquivalent: key)
    }

    @objc private func about() {
        NSApp.activate(ignoringOtherApps: true)
        NSApp.orderFrontStandardAboutPanel(nil)
    }

    @objc private func openSettings() { settings.show() }
    @objc private func pairPhone() { settings.show(then: "pair") }
    @objc private func toggleOffice() { office.toggle() }
}

/// Smolcorp in the Dock and ⌘-Tab, as a full app, or in the menu bar only,
/// as setup's Show in Dock says. The menu bar's item is there either way.
enum Dock {
    static func policy(showing: Bool) -> NSApplication.ActivationPolicy { showing ? .regular : .accessory }

    /// At once: from the settings window, it stays in front, with the menus
    /// it now has or hasn't; from the browser, Smolcorp stays behind.
    static func show(_ showing: Bool) {
        let policy = policy(showing: showing)
        guard NSApp.activationPolicy() != policy else { return }
        let wasActive = NSApp.isActive
        NSApp.setActivationPolicy(policy)
        // macOS puts the app's menus in the menu bar, and keeps its windows forward, once it's activated again.
        if wasActive { DispatchQueue.main.async { NSApp.activate(ignoringOtherApps: true) } }
    }
}
