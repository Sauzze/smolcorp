import AppKit
import Carbon.HIToolbox

/// The app as the Go core's shell (see `Engine`): the menu bar item, the
/// office's window and Settings, loaded from the core's port, and the bridge
/// the core asks for what only macOS can do.
/// No Swift deck is made and the Swift server doesn't start, so the core alone
/// reads and writes the support folder. Quitting stops the core.
/// Its menus are the Swift app's (StatusMenu and AppDelegate before 1.25.0):
/// what they show of the deck, the core sends as it changes (`ShellMenu`), and
/// what they do to it, they ask the core (`GoCore.Op`).
final class GoShellDelegate: NSObject, NSApplicationDelegate, NSMenuDelegate {
    private let bridge: HostBridge
    private let core: GoCore
    /// The support folder's, held from before the Keychain is asked anything or the core starts (`KeychainGate`).
    private let instance: InstanceLock
    private let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private var office: OfficeWindow? {
        didSet { officeLink.window = office }
    }
    /// The office's window as the bridge reaches it: made only once the core is ready (see `ready`).
    private let officeLink = OfficeLink()
    private var settings: SettingsWindow?
    /// Why the core isn't there, while it isn't.
    private var problem: String? = "The Go engine is starting…"
    /// What the menu shows of the deck, as the core last said.
    private var state: ShellMenu?

    init(port: UInt16 = Deck.port, support: URL = Paths.support) {
        bridge = HostBridge(socket: support.appendingPathComponent("bridge.sock"), actions: officeLink.actions(.mac))
        core = GoCore(port: port, support: support, bridge: bridge)
        instance = InstanceLock(support: support)
        super.init()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        item.button?.imagePosition = .imageLeading
        showState()
        item.menu = NSMenu()
        item.menu?.delegate = self
        NSApp.mainMenu = mainMenu()
        // A test copy leaves your login item to the copy in use (see LoginItem.refresh).
        LoginItem.refresh()
        core.onExit = { [weak self] status in self?.coreStopped(status) }
        core.onMenu = { [weak self] state in
            self?.state = state
            self?.showState()
        }
        // The office's window on top or not, settled once as the Swift engine settles it, before the core reads the file.
        OfficeWindow.settleForCore(Config.shared)
        // The unlocks and wakes the core's portable office asks about, counted from now (`MacPresence`).
        MacPresence.shared.start()
        DispatchQueue.global().async { [self] in
            // The folder's instance lock first, and only then the Keychain, the bridge and the core: a second Smolcorp
            // on this folder (one a hire started, say) waits here, and reads none of your Smolcorp's secrets.
            while !instance.take() {
                DispatchQueue.main.async { self.showWaiting(GoShellDelegate.anotherSmolcorpLine) }
                Thread.sleep(forTimeInterval: GoShellDelegate.retryDelay)
            }
            KeychainGate.open(holding: instance)
            do {
                try bridge.start()
            } catch {
                DispatchQueue.main.async { self.problem = error.localizedDescription }
                return
            }
            while true {
                do {
                    try core.start()
                    break
                } catch GoCore.Failure.portInUse(let port) {
                    // Said in the menu, and tried again until the port is free, as the Swift engine's server did.
                    DispatchQueue.main.async { self.showWaiting(GoShellDelegate.portInUseLine(port: port)) }
                    Thread.sleep(forTimeInterval: GoShellDelegate.retryDelay)
                } catch {
                    DispatchQueue.main.async { self.problem = error.localizedDescription }
                    return
                }
            }
            let address = core.officeAddress()
            // Only a test copy leaves a setup link: the real app mints no link nobody asked for.
            let link = TestCopy.isOn ? core.setupLink() : nil
            DispatchQueue.main.async { self.ready(office: address, setupLink: link) }
        }
    }

    /// How long the port is left before it's tried again.
    static let retryDelay: TimeInterval = 2

    /// The menu's line while the port is taken: another Smolcorp, or another app.
    static func portInUseLine(port: UInt16) -> String {
        "Port \(port) is in use. Is Smolcorp already running? Trying again…"
    }

    /// The menu's line while another Smolcorp holds this support folder's instance lock.
    static let anotherSmolcorpLine = "Another Smolcorp is using this support folder. Trying again…"

    /// Said in the menu, with a warning for its icon, until the core is ready.
    private func showWaiting(_ line: String) {
        problem = line
        item.button?.image = NSImage(systemSymbolName: "exclamationmark.triangle", accessibilityDescription: line)
    }

    private func ready(office address: URL?, setupLink: URL?) {
        // The port is the app's again: back to the usual icon (the menu's, with its logo and the portable office's dot).
        item.button?.image = GoShellDelegate.icon(for: state)
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
        // The office's keys, as config.json has them; the bridge's office-key changes them at once (OfficeLink).
        officeLink.keys.hold(OfficeKey.chosen(Config.shared, labels: OfficeKey.currentLabels()))
        if !Config.shared.hasCompletedSetup, TestCopy.mayComeForward("first-run Settings") { settings?.show() }
    }

    /// The core ended by itself: its pages are gone, so their windows close, and
    /// it's said once. It isn't started again: a core that keeps failing would
    /// only fail again.
    private func coreStopped(_ status: Int32) {
        let said = "The Go engine stopped (\(status)). Quit and open Smolcorp again."
        guard problem != said else { return }
        problem = said
        state = nil
        showState()
        officeLink.keys.hold(nil)
        office?.hide()
        office = nil
        settings?.close()
        settings = nil
        // A test copy only logs it: the menu says it too, and its tester quits it by its pid.
        guard TestCopy.mayComeForward("the alert “\(said)”") else { return }
        let alert = NSAlert()
        alert.messageText = "Smolcorp's engine stopped"
        alert.informativeText = "Its windows are closed. Quit Smolcorp and open it again."
        alert.addButton(withTitle: "Quit Smolcorp")
        alert.addButton(withTitle: "Not Now")
        Foreground.activate("the alert “Smolcorp's engine stopped”")
        if alert.runModal() == .alertFirstButtonReturn { NSApp.terminate(nil) }
    }

    func applicationWillTerminate(_ notification: Notification) {
        // Its input ends, then SIGTERM, then SIGKILL, until its pid is gone. The core stops the portable office
        // as its input ends, and asks the viewer to forget it.
        core.stop()
        bridge.stop()
    }

    /// A click on the Dock icon, or the app opened again from Finder,
    /// Spotlight or Launchpad: the office, shown if it's hidden and brought
    /// forward if it isn't, never a second one. Settings opens from ⚙︎, the
    /// menus and ⌘, only.
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows: Bool) -> Bool {
        office?.bringForward()
        return false
    }

    /// The Dock icon's own menu, once the core is ready.
    func applicationDockMenu(_ sender: NSApplication) -> NSMenu? {
        office.map { GoShellDelegate.dockMenu(officeShown: $0.isShown, target: self) }
    }

    // MARK: The menu bar item

    /// The logo, with a small dot while the portable office is on, so it's seen at a glance, and the number of
    /// sessions waiting on you beside it.
    private func showState() {
        item.button?.image = GoShellDelegate.icon(for: state)
        item.button?.title = GoShellDelegate.beside(state)
    }

    static func icon(for state: ShellMenu?) -> NSImage {
        let logo = state?.logo ?? Config.shared.logo
        return state?.portable.on == true ? StatusMenu.dotted(logo) : StatusMenu.icon(for: logo)
    }

    static func beside(_ state: ShellMenu?) -> String {
        guard let waiting = state?.waiting, waiting > 0 else { return "" }
        return " \(waiting)"
    }

    func menuNeedsUpdate(_ menu: NSMenu) {
        // As it is now, if the core answers at once ("last sent 4 s ago"); else as it last said.
        if office != nil, let now = core.menu(waitingUpTo: 0.5) {
            state = now
            showState()
        }
        let headline = GoShellDelegate.headline(port: core.port, problem: problem, sessions: state?.sessions)
        let window = office.map { office -> OfficeMenu in
            let labels = OfficeKey.currentLabels()
            let keys = OfficeKey.chosen(Config.shared, labels: labels)?.menuKey(labels)
            return OfficeMenu(shown: office.isShown, onTop: office.alwaysOnTop, allSpaces: office.onAllSpaces,
                              clickThrough: office.clickThrough, upright: office.isUpright,
                              key: keys?.key ?? "", modifiers: keys?.modifiers ?? [])
        }
        GoShellDelegate.fill(menu, headline: headline, state: state, office: window, openAtLogin: LoginItem.isEnabled, target: self)
    }

    /// The menu's first lines: why the core isn't there, while it isn't; a test copy says it's one, and its port;
    /// then the sessions, once the core has said.
    static func headline(port: UInt16, problem: String?, sessions: Int?, testCopy: Bool = TestCopy.isOn) -> [String] {
        if let problem { return [problem] }
        var lines = testCopy ? ["Smolcorp Test · port \(port)"] : []
        if let sessions { lines.append(sessions == 1 ? "1 agent session" : "\(sessions) agent sessions") }
        return lines.isEmpty ? ["Smolcorp is running"] : lines
    }

    /// The menu bar item's menu, as the Swift app's (StatusMenu): what needs the core only once it's ready
    /// (`office`), and what depends on the deck once it has said (`state`).
    static func fill(_ menu: NSMenu, headline: [String], state: ShellMenu?, office: OfficeMenu?, openAtLogin: Bool,
                     target: AnyObject?) {
        menu.removeAllItems()
        for line in headline { menu.addItem(withTitle: line, action: nil, keyEquivalent: "") }
        menu.addItem(.separator())
        func add(_ title: String, _ action: Selector?, to menu: NSMenu = menu, key: String = "") -> NSMenuItem {
            let item = menu.addItem(withTitle: title, action: action, keyEquivalent: key)
            item.target = target
            return item
        }
        if let office {
            _ = add("Settings…", #selector(openSettings), key: ",")
            _ = add("Open Settings in Browser", #selector(openInBrowser))
            _ = add("Pair a Phone…", #selector(pairPhone))
            _ = add("Show Me Around Again", #selector(showMeAround))
            if let state, state.phones > 0 { _ = add("Unpair All Phones…", #selector(unpairAll)) }
            menu.addItem(.separator())
            // The portable office (beta): the office on a phone from anywhere, view only.
            if let portable = state?.portable {
                if portable.on {
                    _ = add("Portable office: ON · \(portable.status)", nil)
                    _ = add("Stop portable office", #selector(stopPortable))
                    _ = add("Copy viewer link", #selector(copyViewerLink))
                } else if !portable.setUp {
                    _ = add("Going away: launch portable office", nil)
                    add("Set up the viewer on the setup page", #selector(setUpViewer)).indentationLevel = 1
                } else {
                    _ = add("Going away: launch portable office", #selector(startPortable))
                    if !portable.problem.isEmpty { _ = add(portable.problem, nil) }
                }
                menu.addItem(.separator())
            }
            // The keys chosen in Settings (⌥⌘O unless changed), as the layout in use prints them; none when off.
            let show = add("Show Office", #selector(toggleOffice), key: office.key)
            show.keyEquivalentModifierMask = office.modifiers
            show.state = office.shown ? .on : .off
            let options = NSMenu()
            menu.addItem(withTitle: "Office Window", action: nil, keyEquivalent: "").submenu = options
            add("Always on Top", #selector(toggleOnTop), to: options).state = office.onTop ? .on : .off
            add("Show on All Spaces", #selector(toggleAllSpaces), to: options).state = office.allSpaces ? .on : .off
            add("Click-Through", #selector(toggleClickThrough), to: options).state = office.clickThrough ? .on : .off
            options.addItem(.separator())
            add("Sideways", #selector(officeSideways), to: options).state = office.upright ? .off : .on
            add("Upright", #selector(officeUpright), to: options).state = office.upright ? .on : .off
        }
        _ = add("Install Screen Saver…", #selector(installScreenSaver))
        add("Open at Login", #selector(toggleLogin)).state = openAtLogin ? .on : .off
        menu.addItem(.separator())
        _ = add("Quit Smolcorp", #selector(quit), key: "q")
    }

    /// The Dock icon's menu, as the Swift app's.
    static func dockMenu(officeShown: Bool, target: AnyObject?) -> NSMenu {
        let menu = NSMenu()
        for (title, action) in [("Show Office", #selector(toggleOffice)), ("Settings…", #selector(openSettings)),
                                ("Pair a Phone…", #selector(pairPhone))] {
            menu.addItem(withTitle: title, action: action, keyEquivalent: "").target = target
        }
        menu.items.first?.state = officeShown ? .on : .off
        return menu
    }

    // MARK: The menu bar's menus, while Smolcorp is in the Dock

    /// Smolcorp's, Edit, Window and Help, as the Swift app's. In the menu bar only, they aren't shown, but their
    /// keys still work in its windows: the Settings page's fields take theirs.
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
        add(to: app, "Quit Smolcorp", #selector(quit), key: "q")
        // To whatever has the keyboard: the settings page's fields among them.
        let edit = submenu(of: bar, "Edit")
        edit.addItem(withTitle: "Undo", action: Selector(("undo:")), keyEquivalent: "z")
        edit.addItem(withTitle: "Redo", action: Selector(("redo:")), keyEquivalent: "z").keyEquivalentModifierMask = [.command, .shift]
        edit.addItem(.separator())
        edit.addItem(withTitle: "Cut", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        edit.addItem(withTitle: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        edit.addItem(withTitle: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        edit.addItem(withTitle: "Select All", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")
        let window = submenu(of: bar, "Window")
        // The office's keys work from anywhere already (⌥⌘O, or those chosen in Settings), so they aren't given again here.
        add(to: window, "Office", #selector(toggleOffice))
        add(to: window, "Settings", #selector(openSettings))
        window.addItem(.separator())
        window.addItem(withTitle: "Minimize", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
        window.addItem(withTitle: "Close", action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w")
        NSApp.windowsMenu = window
        // The first-run guide's arrows again, from the start, in Settings.
        let help = submenu(of: bar, "Help")
        add(to: help, "Show Me Around Again", #selector(showMeAround))
        NSApp.helpMenu = help
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

    // MARK: What the menus do

    /// Settings, at `then` if given, as the page arrives there (#pair, #guide, #viewer). Never a test copy's
    /// window: its tester reaches setup by its own link.
    /// Before the core is ready, or once it's gone, there's no window: the browser, as the Swift app did.
    private func showSettings(then: String? = nil) {
        guard TestCopy.mayComeForward("Settings") else { return }
        if let url = GoShellDelegate.browserFallback(windowReady: settings != nil, port: core.port, then: then) {
            NSWorkspace.shared.open(url)
            return
        }
        settings?.show(then: then)
    }

    /// The setup page's address for the browser while there's no Settings window; nil once there is one.
    static func browserFallback(windowReady: Bool, port: UInt16, then: String?) -> URL? {
        windowReady ? nil : SettingsWindow.address(port: port, then: then)
    }

    @objc private func openSettings() { showSettings() }
    @objc private func openInBrowser() {
        guard TestCopy.mayComeForward("Settings in the browser") else { return }
        DispatchQueue.global().async { [core] in
            guard let link = core.setupLink() else { return }
            DispatchQueue.main.async { NSWorkspace.shared.open(link) }
        }
    }
    /// The setup page has a code ready when it opens on #pair.
    @objc private func pairPhone() { showSettings(then: "pair") }
    /// The first-run guide's arrows again, from the start, in Settings: the core turns the guide on and keeps it.
    @objc private func showMeAround() {
        core.tell(.guideAgain) { [weak self] _ in self?.showSettings(then: SettingsWindow.guide) }
    }

    @objc private func unpairAll() {
        guard TestCopy.mayComeForward("the alert “Unpair all phones?”") else { return }
        let alert = NSAlert()
        alert.messageText = "Unpair all phones?"
        alert.informativeText = "They stop working straight away. To use the deck again, pair each phone with a new code."
        alert.addButton(withTitle: "Unpair All")
        alert.addButton(withTitle: "Cancel")
        // A menu bar app is never active by itself, and the alert would open behind other windows.
        Foreground.activate("the alert “Unpair all phones?”")
        guard alert.runModal() == .alertFirstButtonReturn else { return }
        // The core may refuse it, in Setup's words (while the key that seals its records can't be read): said, not lost.
        core.tell(.unpairAll) { answer in
            guard let refused = GoShellDelegate.refusal(answer),
                  TestCopy.mayComeForward("the alert “Couldn't unpair the phones”") else { return }
            let said = NSAlert()
            said.messageText = "Couldn't unpair the phones"
            said.informativeText = refused
            Foreground.activate("the alert “Couldn't unpair the phones”")
            said.runModal()
        }
    }

    /// What the core's answer says it refused, in Setup's words: nil when it was done, or when it didn't answer.
    static func refusal(_ answer: [String: Any]?) -> String? {
        guard let refused = answer?["refused"] as? String, !refused.isEmpty else { return nil }
        return refused
    }

    @objc private func startPortable() { core.tell(.portableStart) }
    @objc private func stopPortable() { core.tell(.portableStop) }
    @objc private func setUpViewer() { showSettings(then: "viewer") }
    @objc private func copyViewerLink() {
        core.tell(.viewerLink) { answer in
            if let link = answer?["link"] as? String { GoShellDelegate.copyForThisMacOnly(link) }
        }
    }

    /// The link holds the key: copied for this Mac only, not to your other devices
    /// over Universal Clipboard, and marked so clipboard managers leave it out.
    static func copyForThisMacOnly(_ link: String, to pasteboard: NSPasteboard = MacClipboard.pasteboard) {
        pasteboard.prepareForNewContents(with: .currentHostOnly)
        let item = NSPasteboardItem()
        item.setString(link, forType: .string)
        for marker in ["org.nspasteboard.ConcealedType", "org.nspasteboard.TransientType"] {
            item.setString("", forType: NSPasteboard.PasteboardType(marker))
        }
        pasteboard.writeObjects([item])
    }

    @objc private func toggleOffice() { office?.toggle() }
    /// As setup's switch: the core has this app keep it on top at once (the bridge's office-ontop), then keeps it.
    @objc private func toggleOnTop() {
        guard let office else { return }
        core.tell(.officeOnTop, on: !office.alwaysOnTop)
    }
    @objc private func toggleAllSpaces() { office?.onAllSpaces.toggle() }
    @objc private func toggleClickThrough() { office?.clickThrough.toggle() }
    @objc private func officeSideways() { office?.setUpright(false) }
    @objc private func officeUpright() { office?.setUpright(true) }

    @objc private func installScreenSaver() {
        do {
            // The installed screen saver is the copy in use's: a test copy would put its own over it.
            guard !TestCopy.isOn else { throw TestCopy.Refused.screenSaver }
            try ScreenSaver.install()
        } catch {
            guard TestCopy.mayComeForward("the alert “Couldn't install the screen saver”: \(error.localizedDescription)") else { return }
            let alert = NSAlert()
            alert.messageText = "Couldn't install the screen saver"
            alert.informativeText = error.localizedDescription
            Foreground.activate("the alert “Couldn't install the screen saver”")
            alert.runModal()
        }
    }

    /// A test copy leaves the login item to the copy in use (LoginItem).
    @objc private func toggleLogin() { LoginItem.isEnabled.toggle() }

    @objc private func about() {
        guard TestCopy.mayComeForward("About Smolcorp") else { return }
        Foreground.activate("About Smolcorp")
        NSApp.orderFrontStandardAboutPanel(nil)
    }

    @objc private func quit() { NSApp.terminate(nil) }
}

/// The office's window as the menu bar menu shows it: shown or not, its options, and its keys.
struct OfficeMenu {
    var shown, onTop, allSpaces, clickThrough, upright: Bool
    var key = ""
    var modifiers: NSEvent.ModifierFlags = []
}

/// What the menu bar menu shows of the deck, as the core sends it (desktop/menustate.go): the sessions and how
/// many wait on you, the phones paired, the logo, and the portable office.
struct ShellMenu: Equatable {
    struct Portable: Equatable {
        var on = false, setUp = false
        /// While it's on, how it's going; while it's off, what's wrong, if anything.
        var status = "", problem = ""
    }

    var sessions = 0, waiting = 0, phones = 0
    var logo: Logo?
    var portable = Portable()

    init?(_ object: Any?) {
        guard let object = object as? [String: Any] else { return nil }
        sessions = object["sessions"] as? Int ?? 0
        waiting = object["waiting"] as? Int ?? 0
        phones = object["phones"] as? Int ?? 0
        logo = (object["logo"] as? String).flatMap(Logo.init(stored:))
        let portable = object["portable"] as? [String: Any] ?? [:]
        self.portable = Portable(on: portable["on"] as? Bool ?? false, setUp: portable["setUp"] as? Bool ?? false,
                                 status: portable["status"] as? String ?? "", problem: portable["problem"] as? String ?? "")
    }
}

/// The Go shell's office window, for the bridge's `office`, `office-ontop` and `office-key` (setup's Open the
/// office, its switch to keep it on top, and the keys that show and hide it from anywhere), on the main queue as
/// the bridge runs them. The core keeps the settings in config.json, which this app read as it started: they're
/// changed here in memory only, never saved from here. Setup's facts carry the keys held now and the layout in
/// use, for the core to show them as printed and check them as typed.
final class OfficeLink {
    weak var window: OfficeWindow?
    /// The office's keys, held while the core is ready.
    lazy var keys = OfficeKeyHolder(labels: labels, testCopy: testCopy) { [weak self] in self?.window?.toggle() }
    private let testCopy: Bool
    /// Each key's name on the layout in use: a stand-in in the tests.
    private let labels: () -> OfficeKey.Labels

    init(keys register: OfficeKeyHolder.Register? = nil, labels: @escaping () -> OfficeKey.Labels = OfficeKey.currentLabels,
         testCopy: Bool = TestCopy.isOn) {
        self.labels = labels
        self.testCopy = testCopy
        if let register {
            keys = OfficeKeyHolder(register: register, labels: labels, testCopy: testCopy) { [weak self] in self?.window?.toggle() }
        }
    }

    /// `actions`, with the office's window this shell shows.
    func actions(_ actions: HostBridge.Actions, config: Config = .shared) -> HostBridge.Actions {
        var actions = actions
        actions.showOffice = { [weak self] in
            // Setup's Open the office: the page's own ask, never a test copy's window.
            if TestCopy.mayComeForward("the office (from setup)") { self?.window?.show() }
        }
        actions.officeOnTop = { [weak self] on in
            config.officeOnTop = on
            self?.window?.applyOptions()
        }
        // The GM's tiling: the office's window under the team, while it's shown.
        actions.placeOffice = { [weak self] rect, mainHeight in self?.window?.place(atTile: rect, mainHeight: mainHeight) }
        // Held at once; kept in memory for the menu once they are, as the core keeps them once told.
        actions.officeKey = { [weak self] key in
            guard let self else { return "The office isn't open." }
            if let problem = keys.hold(key) { return problem }
            config.officeKey = OfficeKey.saved(key)
            return nil
        }
        actions.officeKeyLabels = labels
        let facts = actions.facts
        actions.facts = { [weak self, labels] apps in
            var said = facts(apps)
            said["officeKey"] = ["held": self?.keys.current?.text ?? OfficeKey.off, "labels": labels()] as [String: Any]
            return said
        }
        return actions
    }
}
