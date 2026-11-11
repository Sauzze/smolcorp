import AppKit
import Carbon.HIToolbox

/// The app as the Go core's shell, when `Config.engine` is "go" in a test copy
/// (see `Engine`): the menu bar item, the office's window and Settings, loaded
/// from the core's port, and the bridge the core asks for what only macOS can do.
/// No Swift deck is made and the Swift server doesn't start, so the core alone
/// reads and writes the support folder. Quitting stops the core.
final class GoShellDelegate: NSObject, NSApplicationDelegate, NSMenuDelegate {
    private let bridge: HostBridge
    private let core: GoCore
    private let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private var office: OfficeWindow?
    private var settings: SettingsWindow?
    private var officeKey: HotKey?
    /// Why the core isn't there, while it isn't.
    private var problem: String? = "The Go engine is starting…"

    init(port: UInt16 = Deck.port, support: URL = Paths.support) {
        bridge = HostBridge(socket: support.appendingPathComponent("bridge.sock"))
        core = GoCore(port: port, support: support, bridge: bridge)
        super.init()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        item.button?.image = StatusMenu.icon(for: Config.shared.logo)
        item.menu = NSMenu()
        item.menu?.delegate = self
        NSApp.mainMenu = mainMenu()
        core.onExit = { [weak self] status in self?.coreStopped(status) }
        do {
            try bridge.start()
        } catch {
            problem = error.localizedDescription
            return
        }
        DispatchQueue.global().async { [self] in
            do {
                try core.start()
            } catch {
                DispatchQueue.main.async { self.problem = error.localizedDescription }
                return
            }
            let address = core.officeAddress()
            let link = core.setupLink()
            DispatchQueue.main.async { self.ready(office: address, setupLink: link) }
        }
    }

    private func ready(office address: URL?, setupLink: URL?) {
        guard let address else {
            problem = "The Go engine didn't give the office's address."
            return
        }
        problem = nil
        office = OfficeWindow(address: address)
        settings = SettingsWindow(port: core.port, cookieName: core.setupCookie) { [core] in
            core.setupSession()?.session
        }
        // A test copy leaves its tester a setup link in its own folder, as with the Swift engine.
        if let setupLink { TestCopy.leaveSetupLink(setupLink) }
        officeKey = HotKey(keyCode: UInt32(kVK_ANSI_O), modifiers: UInt32(cmdKey | optionKey), id: 1) { [weak self] in self?.office?.toggle() }
        if !Config.shared.hasCompletedSetup { settings?.show() }
    }

    /// The core ended by itself: its pages are gone, so their windows close, and
    /// it's said once. It isn't started again: a core that keeps failing would
    /// only fail again (a test copy only, for now).
    private func coreStopped(_ status: Int32) {
        let said = "The Go engine stopped (\(status)). Quit and open Smolcorp again."
        guard problem != said else { return }
        problem = said
        officeKey = nil
        office?.hide()
        office = nil
        settings?.close()
        settings = nil
        let alert = NSAlert()
        alert.messageText = "Smolcorp's engine stopped"
        alert.informativeText = "Its windows are closed. Quit Smolcorp and open it again."
        alert.addButton(withTitle: "Quit Smolcorp")
        alert.addButton(withTitle: "Not Now")
        NSApp.activate(ignoringOtherApps: true)
        if alert.runModal() == .alertFirstButtonReturn { NSApp.terminate(nil) }
    }

    func applicationWillTerminate(_ notification: Notification) {
        // Its input ends, then SIGTERM, then SIGKILL, until its pid is gone.
        core.stop()
        bridge.stop()
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows: Bool) -> Bool {
        office?.bringForward()
        return false
    }

    // MARK: Menus

    func menuNeedsUpdate(_ menu: NSMenu) {
        menu.removeAllItems()
        menu.addItem(withTitle: problem ?? "Smolcorp Test · Go engine on port \(core.port)", action: nil, keyEquivalent: "")
        menu.addItem(.separator())
        if let office {
            let show = add(to: menu, "Show Office", #selector(toggleOffice), key: "o")
            show.keyEquivalentModifierMask = [.command, .option]
            show.state = office.isShown ? .on : .off
            add(to: menu, "Settings…", #selector(openSettings), key: ",")
            add(to: menu, "Open Settings in Browser", #selector(openInBrowser))
            menu.addItem(.separator())
        }
        add(to: menu, "Quit Smolcorp", #selector(quit), key: "q")
    }

    /// Smolcorp's, Edit and Window, so the Settings page's fields take their keys.
    private func mainMenu() -> NSMenu {
        let bar = NSMenu()
        let app = submenu(of: bar, "Smolcorp")
        add(to: app, "Settings…", #selector(openSettings), key: ",")
        app.addItem(.separator())
        add(to: app, "Quit Smolcorp", #selector(quit), key: "q")
        let edit = submenu(of: bar, "Edit")
        edit.addItem(withTitle: "Undo", action: Selector(("undo:")), keyEquivalent: "z")
        edit.addItem(withTitle: "Redo", action: Selector(("redo:")), keyEquivalent: "z").keyEquivalentModifierMask = [.command, .shift]
        edit.addItem(.separator())
        edit.addItem(withTitle: "Cut", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        edit.addItem(withTitle: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        edit.addItem(withTitle: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        edit.addItem(withTitle: "Select All", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")
        let window = submenu(of: bar, "Window")
        window.addItem(withTitle: "Minimize", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
        window.addItem(withTitle: "Close", action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w")
        NSApp.windowsMenu = window
        return bar
    }

    private func submenu(of bar: NSMenu, _ title: String) -> NSMenu {
        let menu = NSMenu(title: title)
        bar.addItem(withTitle: title, action: nil, keyEquivalent: "").submenu = menu
        return menu
    }

    @discardableResult
    private func add(to menu: NSMenu, _ title: String, _ action: Selector, key: String = "") -> NSMenuItem {
        let item = menu.addItem(withTitle: title, action: action, keyEquivalent: key)
        item.target = self
        return item
    }

    @objc private func toggleOffice() { office?.toggle() }
    @objc private func openSettings() { settings?.show() }
    @objc private func openInBrowser() {
        DispatchQueue.global().async { [core] in
            guard let link = core.setupLink() else { return }
            DispatchQueue.main.async { NSWorkspace.shared.open(link) }
        }
    }
    @objc private func quit() { NSApp.terminate(nil) }
}
