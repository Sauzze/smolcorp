import AppKit
import Carbon.HIToolbox
import WebKit

/// The office in a window of its own on this Mac: no title bar and no frame,
/// only the room, rounded, with the window's shadow under it. The menu bar's
/// Show Office, or ⌥⌘O from anywhere, shows and hides it, and Esc hides it
/// once you've clicked it. It's held with ⌘ and dragged anywhere, or by the bar
/// along the middle of its top edge, and resized from its edges and corners,
/// keeping the office's shape: drawn taller than wide, it turns upright, and
/// wider than tall, sideways. Each screen remembers where it was.
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
        content.addSubview(GrabBar(in: content.bounds, office: self))
        for edges in ResizeHandle.all { content.addSubview(ResizeHandle(edges, in: content.bounds, office: self)) }
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

    /// Resizes from `edges`, `start` being the frame when the drag began and
    /// `moved` how far the pointer has gone since, in screen points.
    func resize(from edges: ResizeHandle.Edges, start: NSRect, moved: NSPoint) {
        guard let panel else { return }
        var proposed = start.size
        if edges.contains(.right) { proposed.width += moved.x }
        if edges.contains(.left) { proposed.width -= moved.x }
        if edges.contains(.top) { proposed.height += moved.y }
        if edges.contains(.bottom) { proposed.height -= moved.y }
        let horizontal = !edges.isDisjoint(with: [.left, .right]), vertical = !edges.isDisjoint(with: [.top, .bottom])
        let drive: Drive = horizontal && !vertical ? .width : !horizontal ? .height
            : abs(proposed.width - start.width) / start.width >= abs(proposed.height - start.height) / start.height ? .width : .height
        let size = OfficeWindow.fit(proposed, by: drive)
        // The edges not dragged stay where they were.
        let x = edges.contains(.left) ? start.maxX - size.width : start.minX
        let y = edges.contains(.bottom) ? start.maxY - size.height : start.minY
        panel.setFrame(NSRect(x: x, y: y, width: size.width, height: size.height), display: true)
    }

    // MARK: Moving it

    /// Where the window and the pointer were as a move began. It's moved by
    /// hand, as it's resized: the window server's own drag doesn't take a
    /// panel that leaves Smolcorp behind the app you're in.
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

    func resized() {
        saveFrame()
        refreshShadow()
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

/// A slim bar along the middle of the office's top edge that shows while the
/// pointer is over it, to drag the window by without ⌘. It keeps clear of the
/// corners, where the office has its style key.
final class GrabBar: NSView {
    static let size = NSSize(width: 96, height: 12)
    private var showing = false { didSet { needsDisplay = true } }
    private weak var office: OfficeWindow?

    init(in bounds: NSRect, office: OfficeWindow) {
        self.office = office
        super.init(frame: NSRect(x: (bounds.width - GrabBar.size.width) / 2, y: bounds.height - GrabBar.size.height,
                                 width: GrabBar.size.width, height: GrabBar.size.height))
        autoresizingMask = [.minXMargin, .maxXMargin, .minYMargin]
    }

    required init?(coder: NSCoder) { nil }

    override func updateTrackingAreas() {
        trackingAreas.forEach(removeTrackingArea)
        addTrackingArea(NSTrackingArea(rect: bounds, options: [.mouseEnteredAndExited, .activeAlways], owner: self))
    }

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
        NSColor(white: 1, alpha: 0.55).setFill()
        NSBezierPath(roundedRect: bounds.insetBy(dx: 24, dy: 4), xRadius: 2, yRadius: 2).fill()
    }
}

/// An edge or corner of the office's window to resize it by.
final class ResizeHandle: NSView {
    struct Edges: OptionSet {
        let rawValue: Int
        static let left = Edges(rawValue: 1), right = Edges(rawValue: 2), top = Edges(rawValue: 4), bottom = Edges(rawValue: 8)
    }

    static let all: [Edges] = [.left, .right, .top, .bottom, [.left, .top], [.right, .top], [.left, .bottom], [.right, .bottom]]
    /// How far in from the edge it can be taken, and along from a corner, in points.
    static let thickness: CGFloat = 5, corner: CGFloat = 14

    private let edges: Edges
    private weak var office: OfficeWindow?
    private var start: (frame: NSRect, pointer: NSPoint)?

    init(_ edges: Edges, in bounds: NSRect, office: OfficeWindow) {
        self.edges = edges
        self.office = office
        let (t, c) = (ResizeHandle.thickness, ResizeHandle.corner)
        let isCorner = edges.rawValue.nonzeroBitCount == 2
        let width = isCorner ? c : edges.isDisjoint(with: [.left, .right]) ? bounds.width - 2 * c : t
        let height = isCorner ? c : edges.isDisjoint(with: [.top, .bottom]) ? bounds.height - 2 * c : t
        let x = edges.contains(.left) ? 0 : edges.contains(.right) ? bounds.width - width : c
        let y = edges.contains(.bottom) ? 0 : edges.contains(.top) ? bounds.height - height : c
        super.init(frame: NSRect(x: x, y: y, width: width, height: height))
        var mask: NSView.AutoresizingMask = []
        mask.insert(edges.contains(.left) ? .maxXMargin : edges.contains(.right) ? .minXMargin : .width)
        mask.insert(edges.contains(.bottom) ? .maxYMargin : edges.contains(.top) ? .minYMargin : .height)
        autoresizingMask = mask
    }

    required init?(coder: NSCoder) { nil }

    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }

    override func resetCursorRects() {
        let cursor: NSCursor = edges == .left || edges == .right ? .resizeLeftRight
            : edges == .top || edges == .bottom ? .resizeUpDown : .crosshair
        addCursorRect(bounds, cursor: cursor)
    }

    override func mouseDown(with event: NSEvent) {
        guard let frame = window?.frame else { return }
        start = (frame, NSEvent.mouseLocation)
    }

    override func mouseDragged(with event: NSEvent) {
        guard let start else { return }
        let pointer = NSEvent.mouseLocation
        office?.resize(from: edges, start: start.frame, moved: NSPoint(x: pointer.x - start.pointer.x, y: pointer.y - start.pointer.y))
    }

    override func mouseUp(with event: NSEvent) {
        start = nil
        office?.resized()
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
