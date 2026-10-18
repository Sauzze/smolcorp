import AppKit

/// The menu bar item: the deck's logo, with the number of sessions waiting on
/// you beside it.
final class StatusMenu: NSObject, NSMenuDelegate {
    private let deck: Deck
    private let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private var error: String?

    init(deck: Deck) {
        self.deck = deck
        super.init()
        item.button?.imagePosition = .imageLeading
        item.menu = NSMenu()
        item.menu?.delegate = self
        deck.onChange = { [weak self] in self?.update() }
        update()
    }

    func showPortInUse() {
        error = "Port \(Deck.port) is in use. Is Smolcorp already running? Trying again…"
        item.button?.image = NSImage(systemSymbolName: "exclamationmark.triangle", accessibilityDescription: error)
        item.button?.title = ""
    }

    /// The port is the deck's again: back to the usual icon.
    func showListening() {
        guard error != nil else { return }
        error = nil
        update()
    }

    private func update() {
        guard error == nil else { return }
        item.button?.image = StatusMenu.icon(for: Config.shared.logo)
        let waiting = deck.sessions.all.filter { $0.state == .asking }.count
        item.button?.title = waiting > 0 ? " \(waiting)" : ""
    }

    func menuNeedsUpdate(_ menu: NSMenu) {
        menu.removeAllItems()

        if let error {
            menu.addItem(withTitle: error, action: nil, keyEquivalent: "")
        } else {
            let count = deck.sessions.all.count
            menu.addItem(withTitle: count == 1 ? "1 agent session" : "\(count) agent sessions", action: nil, keyEquivalent: "")
        }
        menu.addItem(.separator())
        add(to: menu, "Open Setup…", #selector(openSetup))
        add(to: menu, "Pair a Phone…", #selector(pairPhone))
        if !deck.access.phones.all.isEmpty {
            add(to: menu, "Unpair All Phones…", #selector(unpairAll))
        }
        menu.addItem(.separator())
        add(to: menu, "Install Screen Saver…", #selector(installScreenSaver))
        let login = add(to: menu, "Open at Login", #selector(toggleLogin))
        login.state = LoginItem.isEnabled ? .on : .off
        menu.addItem(.separator())
        add(to: menu, "Quit Smolcorp", #selector(quit), key: "q")
    }

    @discardableResult
    private func add(to menu: NSMenu, _ title: String, _ action: Selector, key: String = "") -> NSMenuItem {
        let entry = menu.addItem(withTitle: title, action: action, keyEquivalent: key)
        entry.target = self
        return entry
    }

    @objc private func openSetup() { NSWorkspace.shared.open(deck.setupURL()) }

    @objc private func pairPhone() { NSWorkspace.shared.open(deck.setupURL(then: "pair")) }

    @objc private func unpairAll() {
        let alert = NSAlert()
        alert.messageText = "Unpair all phones?"
        alert.informativeText = "They stop working straight away. To use the deck again, pair each phone with a new code."
        alert.addButton(withTitle: "Unpair All")
        alert.addButton(withTitle: "Cancel")
        // A menu bar app is never active by itself, and the alert would open behind other windows.
        NSApp.activate(ignoringOtherApps: true)
        if alert.runModal() == .alertFirstButtonReturn { deck.unpairAll() }
    }

    @objc private func installScreenSaver() {
        do {
            // The installed screen saver is the copy in use's: a test copy would put its own over it.
            guard !TestCopy.isOn else { throw TestCopy.Refused.screenSaver }
            try ScreenSaver.install()
        } catch {
            let alert = NSAlert()
            alert.messageText = "Couldn't install the screen saver"
            alert.informativeText = error.localizedDescription
            NSApp.activate(ignoringOtherApps: true)
            alert.runModal()
        }
    }

    @objc private func toggleLogin() { LoginItem.isEnabled.toggle() }

    @objc private func quit() { NSApp.terminate(nil) }

    /// Both are drawn as templates, so they follow the menu bar's colour.
    private static func icon(for logo: Logo) -> NSImage {
        switch logo {
        case .smolcorp: return smolcorpIcon
        case .keys: return keysIcon
        }
    }

    /// The hire, a little person in a suit and tie, a point to each cell of
    /// its grid, as the logo kit draws it. A template is tinted white in a
    /// dark menu bar, where the light-ground drawing's hollow face would turn
    /// into a helmet, so a dark one gets the drawing made for dark grounds.
    private static let hireOnLight = [
        ".....######.....", "....########....", "....##....##....", "....#......#....",
        "....#.#..#.#....", "....#.#..#.#....", "....#......#....", ".....######.....",
        "................", "...##..##..##...", "...###.##.###...", "...####..####...",
        "...####..####...", "...##########...", "....###..###....", "....###..###....",
    ]
    private static let hireOnDark = [
        ".....######.....", "....########....", "................", "....########....",
        "....##.##.##....", "....##.##.##....", "....########....", ".....######.....",
        "................", "...##.#..#.##...", "...###.##.###...", "...####..####...",
        "...####..####...", "...##########...", "....###..###....", "....###..###....",
    ]
    private static let smolcorpIcon: NSImage = {
        let image = NSImage(size: NSSize(width: 18, height: 18), flipped: true) { _ in
            guard let context = NSGraphicsContext.current?.cgContext else { return false }
            // Whole cells only, or the edges blur
            context.setShouldAntialias(false)
            let dark = NSAppearance.currentDrawing().bestMatch(from: [.aqua, .darkAqua]) == .darkAqua
            for (row, cells) in (dark ? hireOnDark : hireOnLight).enumerated() {
                for (column, cell) in cells.enumerated() where cell == "#" {
                    context.fill(CGRect(x: 1 + column, y: 1 + row, width: 1, height: 1))
                }
            }
            return true
        }
        image.isTemplate = true
        return image
    }()

    /// Six rounded keys, 3 × 2.
    private static let keysIcon: NSImage = {
        let image = NSImage(size: NSSize(width: 18, height: 14), flipped: false) { _ in
            let side: CGFloat = 4.6, gap: CGFloat = 1.6
            let top: CGFloat = (14 - (2 * side + gap)) / 2
            for row in 0..<2 {
                for col in 0..<3 {
                    let origin = NSPoint(x: 1 + CGFloat(col) * (side + gap), y: top + CGFloat(row) * (side + gap))
                    let rect = NSRect(origin: origin, size: NSSize(width: side, height: side))
                    NSBezierPath(roundedRect: rect, xRadius: 1.2, yRadius: 1.2).fill()
                }
            }
            return true
        }
        image.isTemplate = true
        return image
    }()
}
