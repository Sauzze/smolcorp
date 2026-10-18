import AppKit
import Carbon.HIToolbox
import WebKit

/// The office in a window of its own on this Mac: no title bar and no frame,
/// only the room, rounded, with the window's shadow under it. The menu bar's
/// Show Office, or ⌥⌘O from anywhere, shows and hides it, and Esc hides it
/// once you've clicked it. It's moved by the bar along the middle of its top
/// edge, which shows while the pointer is over the office, or held with ⌘ and
/// dragged anywhere, onto any screen; and resized from its edges and corners,
/// as any window is, keeping the office's shape: drawn taller than wide, it
/// turns upright, and wider than tall, sideways. Each screen remembers where
/// it was.
///
/// It reads the office as a phone does, with a key of its own that lives as
/// long as the app and works only from this Mac: no pairing, and nothing
/// listed among your phones. Showing it doesn't take the keyboard from the
/// window you're typing in.
final class OfficeWindow: NSObject, NSWindowDelegate, WKNavigationDelegate {
    /// The office's shapes, as office.js draws them: sideways and upright.
    static let wide = NSSize(width: 320, height: 180)
    static let tall = NSSize(width: 224, height: 372)
    /// How big it first opens, sideways: twice the office's own size.
    static let firstSize = NSSize(width: 640, height: 360)
    /// From a screen's top-right corner, where it first opens.
    static let firstMargin: CGFloat = 24

    /// The office page, with the window's key.
    private let address: URL
    private var panel: OfficePanel?
    private var monitors: [Any] = []
    private let defaults: UserDefaults

    init(address: URL, defaults: UserDefaults = .standard) {
        self.address = address
        self.defaults = defaults
    }

    var isShown: Bool { panel?.isVisible == true }

    // MARK: Options, kept between launches

    private func option(_ name: String, standard: Bool) -> Bool {
        defaults.object(forKey: "officeWindow.\(name)") as? Bool ?? standard
    }

    /// On top of other windows: on unless you turn it off.
    var alwaysOnTop: Bool {
        get { option("alwaysOnTop", standard: true) }
        set { defaults.set(newValue, forKey: "officeWindow.alwaysOnTop"); applyOptions() }
    }

    /// On every Space, and beside apps in full screen.
    var onAllSpaces: Bool {
        get { option("onAllSpaces", standard: false) }
        set { defaults.set(newValue, forKey: "officeWindow.onAllSpaces"); applyOptions() }
    }

    /// Clicks go through it to whatever is behind: turned off from the menu,
    /// as the office can't be clicked meanwhile.
    var clickThrough: Bool {
        get { option("clickThrough", standard: false) }
        set { defaults.set(newValue, forKey: "officeWindow.clickThrough"); applyOptions() }
    }

    var isUpright: Bool {
        guard let frame = panel?.frame ?? savedFrame(on: screenUnderPointer()) else { return false }
        return frame.height > frame.width
    }

    private func applyOptions() {
        guard let panel else { return }
        panel.level = alwaysOnTop ? .floating : .normal
        panel.collectionBehavior = onAllSpaces ? [.canJoinAllSpaces, .fullScreenAuxiliary] : [.fullScreenAuxiliary]
        panel.ignoresMouseEvents = clickThrough
    }

    // MARK: Showing and hiding

    func toggle() { isShown ? hide() : show() }

    func show() {
        let panel = self.panel ?? makePanel()
        let screen = screenUnderPointer()
        panel.setFrame(savedFrame(on: screen) ?? firstFrame(on: screen), display: false)
        applyOptions()
        // In front, leaving the keyboard where you were typing: it takes Esc
        // once you've clicked it.
        panel.orderFrontRegardless()
        refreshShadow()
    }

    func hide() {
        guard let panel, panel.isVisible else { return }
        saveFrame()
        panel.orderOut(nil)
    }

    /// Sideways or upright, keeping its middle and about its size.
    func setUpright(_ upright: Bool) {
        if !isShown { show() }
        guard let panel else { return }
        let shape = upright ? OfficeWindow.tall : OfficeWindow.wide
        let area = panel.frame.width * panel.frame.height
        let width = (area * shape.width / shape.height).squareRoot()
        let size = OfficeWindow.fit(NSSize(width: width, height: width * shape.height / shape.width), by: .width)
        let middle = NSPoint(x: panel.frame.midX, y: panel.frame.midY)
        panel.setFrame(NSRect(x: middle.x - size.width / 2, y: middle.y - size.height / 2, width: size.width, height: size.height), display: true)
        refreshShadow()
    }

    private func makePanel() -> OfficePanel {
        let panel = OfficePanel(contentRect: NSRect(origin: .zero, size: OfficeWindow.firstSize),
                                styleMask: [.borderless, .nonactivatingPanel, .resizable], backing: .buffered, defer: false)
        panel.office = self
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.hasShadow = true
        panel.hidesOnDeactivate = false
        panel.isFloatingPanel = true
        panel.becomesKeyOnlyIfNeeded = true
        panel.isReleasedWhenClosed = false
        panel.delegate = self

        let content = NSView(frame: NSRect(origin: .zero, size: OfficeWindow.firstSize))
        let configuration = WKWebViewConfiguration()
        // Nothing kept on disk: the page holds no state of its own.
        configuration.websiteDataStore = .nonPersistent()
        let web = OfficeWebView(frame: content.bounds, configuration: configuration)
        web.setValue(false, forKey: "drawsBackground")
        web.autoresizingMask = [.width, .height]
        web.navigationDelegate = self
        content.addSubview(web)
        let bar = GrabBar(in: content.bounds, office: self)
        content.addSubview(bar)
        // The bar shows while the pointer is anywhere over the office, so it's there to be found.
        content.addTrackingArea(NSTrackingArea(rect: .zero, options: [.mouseEnteredAndExited, .activeAlways, .inVisibleRect], owner: bar))
        panel.contentView = content
        web.load(URLRequest(url: address))

        // A click brings it forward, as only the window clicked takes it (⌘
        // held, it's a move: see OfficePanel); Esc hides it once it has the keyboard.
        monitors.append(NSEvent.addLocalMonitorForEvents(matching: .leftMouseDown) { [weak panel] event in
            guard let panel, event.window === panel, !event.modifierFlags.contains(.command) else { return event }
            panel.firstClickOnlyRaises = !panel.isKeyWindow && panel.isPartlyCovered
            panel.orderFrontRegardless()
            return event
        } as Any)
        monitors.append(NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self, weak panel] event in
            guard let panel, panel.isKeyWindow, event.window === panel, event.keyCode == UInt16(kVK_Escape) else { return event }
            self?.hide()
            return nil
        } as Any)
        self.panel = panel
        return panel
    }

    // MARK: Its shape

    enum Drive { case width, height }

    /// `proposed` as the office fits it: upright when taller than wide,
    /// sideways otherwise, keeping the office's own proportions from the
    /// dimension being dragged, and never smaller than the office drawn once.
    static func fit(_ proposed: NSSize, by drive: Drive) -> NSSize {
        let shape = proposed.height > proposed.width ? tall : wide
        let aspect = shape.width / shape.height
        if drive == .width {
            let width = max(shape.width, proposed.width)
            return NSSize(width: width.rounded(), height: (width / aspect).rounded())
        }
        let height = max(shape.height, proposed.height)
        return NSSize(width: (height * aspect).rounded(), height: height.rounded())
    }

    /// A resize under way: the size it began at, and whether its width and
    /// its height are being dragged.
    private var resizing: (start: NSSize, width: Bool, height: Bool)?

    func windowWillStartLiveResize(_ notification: Notification) {
        resizing = panel.map { ($0.frame.size, false, false) }
    }

    /// macOS resizes it, from any edge or corner, as it does any window, a
    /// step at a time, and the office keeps its shape (see `fit`). The shape
    /// is the one the pointer asks for: what's dragged follows it, what isn't
    /// stays as it was when the drag began, since the office's proportions
    /// move it meanwhile. So a bottom edge dragged down far enough turns it
    /// upright.
    func windowWillResize(_ sender: NSWindow, to proposed: NSSize) -> NSSize {
        let now = sender.frame.size
        var drag = resizing ?? (now, false, false)
        drag.width = drag.width || proposed.width != now.width
        drag.height = drag.height || proposed.height != now.height
        resizing = drag
        let wanted = NSSize(width: drag.width ? proposed.width : drag.start.width, height: drag.height ? proposed.height : drag.start.height)
        return OfficeWindow.fit(wanted, by: OfficeWindow.drive(from: drag.start, to: wanted, width: drag.width, height: drag.height))
    }

    /// Which way a resize from `start` to `wanted` goes by: the dimension
    /// dragged, or of both, the one that's changed the more.
    static func drive(from start: NSSize, to wanted: NSSize, width: Bool, height: Bool) -> Drive {
        guard width, height else { return width ? .width : .height }
        return abs(wanted.width - start.width) / start.width >= abs(wanted.height - start.height) / start.height ? .width : .height
    }

    func windowDidEndLiveResize(_ notification: Notification) {
        resizing = nil
        saveFrame()
        refreshShadow()
    }

    // MARK: Moving it

    /// Where the window and the pointer were as a move began. It's moved by
    /// hand: the window server's own drag doesn't take a panel that leaves
    /// Smolcorp behind the app you're in.
    private var moving: (frame: NSRect, pointer: NSPoint)?

    var isMoving: Bool { moving != nil }

    /// From the pointer at `pointer`, on the screen.
    func beginMove(at pointer: NSPoint) {
        guard let panel else { return }
        moving = (panel.frame, pointer)
    }

    func move(to pointer: NSPoint) {
        guard let panel, let moving else { return }
        panel.setFrameOrigin(NSPoint(x: moving.frame.minX + pointer.x - moving.pointer.x, y: moving.frame.minY + pointer.y - moving.pointer.y))
    }

    func endMove() {
        guard moving != nil else { return }
        moving = nil
        saveFrame()
    }

    /// The window's shadow follows what's drawn: it's worked out again once
    /// the page has drawn at its new size.
    private func refreshShadow() {
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.4) { [weak self] in self?.panel?.invalidateShadow() }
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) { refreshShadow() }

    /// Only the office's own pages, from this Mac's Smolcorp: the window's
    /// address carries its key.
    func webView(_ webView: WKWebView, decidePolicyFor action: WKNavigationAction,
                 decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        decisionHandler(OfficeWindow.isOwnPage(action.request.url, of: address) ? .allow : .cancel)
    }

    static func isOwnPage(_ url: URL?, of address: URL) -> Bool {
        guard let url else { return false }
        return url.scheme == "http" && url.host == "127.0.0.1" && url.port == address.port
    }

    func windowDidMove(_ notification: Notification) { saveFrame() }

    // MARK: Where it is, per screen

    private func screenUnderPointer() -> NSScreen? {
        let pointer = NSEvent.mouseLocation
        return NSScreen.screens.first { NSMouseInRect(pointer, $0.frame, false) } ?? NSScreen.main
    }

    private static func id(of screen: NSScreen?) -> String? {
        (screen?.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber)?.stringValue
    }

    private var savedFrames: [String: String] {
        get { defaults.dictionary(forKey: "officeWindow.frames") as? [String: String] ?? [:] }
        set { defaults.set(newValue, forKey: "officeWindow.frames") }
    }

    private func savedFrame(on screen: NSScreen?) -> NSRect? {
        guard let id = OfficeWindow.id(of: screen), let text = savedFrames[id], let screen else { return nil }
        let frame = NSRectFromString(text)
        // Kept only while it's still on that screen, as its screen can change size
        return frame.width > 0 && screen.visibleFrame.intersects(frame) ? frame : nil
    }

    private func firstFrame(on screen: NSScreen?) -> NSRect {
        let visible = screen?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1280, height: 800)
        let size = OfficeWindow.firstSize
        return NSRect(x: visible.maxX - size.width - OfficeWindow.firstMargin, y: visible.maxY - size.height - OfficeWindow.firstMargin,
                      width: size.width, height: size.height)
    }

    private func saveFrame() {
        guard let panel, panel.isVisible, let id = OfficeWindow.id(of: panel.screen) else { return }
        savedFrames[id] = NSStringFromRect(panel.frame)
    }
}

/// The office's window: it can take Esc while in front, without making
/// Smolcorp the app in front.
final class OfficePanel: NSPanel {
    override var canBecomeKey: Bool { true }

    weak var office: OfficeWindow?

    /// ⌘ held, a drag anywhere moves it, and a move, begun here or on its bar,
    /// follows the pointer until let go: taken here, before the page sees
    /// any of it.
    override func sendEvent(_ event: NSEvent) {
        let pointer = convertPoint(toScreen: event.locationInWindow)
        switch event.type {
        case .leftMouseDown where event.modifierFlags.contains(.command):
            office?.beginMove(at: pointer)
        case .leftMouseDragged where office?.isMoving == true:
            office?.move(to: pointer)
        case .leftMouseUp where office?.isMoving == true:
            office?.endMove()
        default:
            super.sendEvent(event)
        }
    }

    /// Whether the click being taken was only to bring the window forward.
    var firstClickOnlyRaises = false

    /// Whether another window is over part of it: a click on it then is one
    /// to bring it forward. Only windows at its own level or below count,
    /// not the menu bar or the Dock.
    var isPartlyCovered: Bool {
        guard let above = CGWindowListCopyWindowInfo([.optionOnScreenAboveWindow, .excludeDesktopElements],
                                                     CGWindowID(windowNumber)) as? [[String: Any]],
              let primary = NSScreen.screens.first else { return false }
        // Window lists measure from the top of the main screen, downwards.
        let mine = CGRect(x: frame.minX, y: primary.frame.maxY - frame.maxY, width: frame.width, height: frame.height)
        return above.contains { window in
            guard let layer = window[kCGWindowLayer as String] as? Int, (0...level.rawValue).contains(layer),
                  (window[kCGWindowAlpha as String] as? Double ?? 1) > 0,
                  let bounds = (window[kCGWindowBounds as String] as? NSDictionary).flatMap({ CGRect(dictionaryRepresentation: $0 as CFDictionary) })
            else { return false }
            return bounds.intersects(mine)
        }
    }
}

/// The office's page, which takes the click that brings its window forward
/// as a click, as a phone would, unless the window was partly hidden: that
/// click only brings it forward, so it doesn't press whatever it lands on.
final class OfficeWebView: WKWebView {
    override func acceptsFirstMouse(for event: NSEvent?) -> Bool {
        !((window as? OfficePanel)?.firstClickOnlyRaises ?? false)
    }

    /// Clicked, the window takes the keyboard, for Esc, without bringing
    /// Smolcorp forward.
    override var needsPanelToBecomeKey: Bool { true }
}

/// A bar along the middle of the office's top edge to move the window by,
/// shown while the pointer is over the office. It keeps clear of the corners,
/// where the office has its style key.
final class GrabBar: NSView {
    static let size = NSSize(width: 140, height: 18)
    private var showing = false { didSet { needsDisplay = true } }
    private weak var office: OfficeWindow?

    init(in bounds: NSRect, office: OfficeWindow) {
        self.office = office
        super.init(frame: NSRect(x: (bounds.width - GrabBar.size.width) / 2, y: bounds.height - GrabBar.size.height,
                                 width: GrabBar.size.width, height: GrabBar.size.height))
        autoresizingMask = [.minXMargin, .maxXMargin, .minYMargin]
    }

    required init?(coder: NSCoder) { nil }

    /// Over the office, from its tracking area: see `OfficeWindow.makePanel`.
    override func mouseEntered(with event: NSEvent) { showing = true }
    override func mouseExited(with event: NSEvent) { showing = false }
    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }
    /// The move follows the pointer from here: see OfficePanel.
    override func mouseDown(with event: NSEvent) {
        guard let window else { return }
        office?.beginMove(at: window.convertPoint(toScreen: event.locationInWindow))
    }
    override func resetCursorRects() { addCursorRect(bounds, cursor: .openHand) }

    override func draw(_ dirtyRect: NSRect) {
        guard showing else { return }
        let pill = NSBezierPath(roundedRect: bounds.insetBy(dx: 30, dy: 5), xRadius: 4, yRadius: 4)
        NSColor(white: 1, alpha: 0.85).setFill()
        pill.fill()
        NSColor(white: 0, alpha: 0.35).setStroke()
        pill.stroke()
    }
}

/// A key held down anywhere, as ⌥⌘O for the office's window: Carbon's hot
/// keys, which need no permission, as they only ever see their own keys.
final class HotKey {
    private var reference: EventHotKeyRef?
    private static var actions: [UInt32: () -> Void] = [:]
    private static var handler: EventHandlerRef?

    init?(keyCode: UInt32, modifiers: UInt32, id: UInt32, action: @escaping () -> Void) {
        if HotKey.handler == nil {
            var pressed = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))
            InstallEventHandler(GetApplicationEventTarget(), { _, event, _ in
                var key = EventHotKeyID()
                GetEventParameter(event, EventParamName(kEventParamDirectObject), EventParamType(typeEventHotKeyID), nil,
                                  MemoryLayout<EventHotKeyID>.size, nil, &key)
                HotKey.actions[key.id]?()
                return noErr
            }, 1, &pressed, nil, &HotKey.handler)
        }
        // "SMOL", for Smolcorp's own keys
        let key = EventHotKeyID(signature: OSType(0x534D_4F4C), id: id)
        guard RegisterEventHotKey(keyCode, modifiers, key, GetApplicationEventTarget(), 0, &reference) == noErr else { return nil }
        HotKey.actions[id] = action
    }

    deinit {
        if let reference { UnregisterEventHotKey(reference) }
    }
}
