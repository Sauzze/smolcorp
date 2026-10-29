import AppKit
import WebKit

/// The setup page in a window of its own, Smolcorp Settings, from the menu's
/// Settings… (⌘,). One window: asked for again, it comes forward as it is.
/// It's resized as any window, never below `minimumSize`, and opens again
/// where you left it.
///
/// It reads the page as the browser does after the menu's link, with the
/// setup key's cookie, but no link carries the key: the cookie is put
/// straight into the window's own cookie store, which lives in memory only,
/// as long as the app. Nothing on the server changes, and the browser's way
/// in keeps all its checks.
///
/// Only the setup page opens in it. A link elsewhere opens in your browser.
final class SettingsWindow: NSObject, NSWindowDelegate, WKNavigationDelegate, WKUIDelegate, WKDownloadDelegate {
    static let minimumSize = NSSize(width: 900, height: 640)
    static let firstSize = NSSize(width: 1100, height: 780)
    static let frameName = "SmolcorpSettings"

    private let port: UInt16
    private let setupKey: String
    private var window: SettingsPanel?
    private var web: WKWebView?
    /// Cookies and the page's own storage, for as long as the app runs, and never on disk.
    private let store = WKWebsiteDataStore.nonPersistent()
    /// Downloads saved over a file you chose to replace: written beside it
    /// first, and swapped in once whole, so a failed one leaves yours as it was.
    private var replacing: [ObjectIdentifier: (written: URL, chosen: URL)] = [:]
    /// Where each download goes, to show it in the Finder once it's saved.
    private var saving: [ObjectIdentifier: URL] = [:]

    init(port: UInt16 = Deck.port, setupKey: String) {
        self.port = port
        self.setupKey = setupKey
    }

    var isShown: Bool { window?.isVisible == true }

    // MARK: Addresses and what opens where

    /// The setup page, and what to do there (`pair`, `viewer`), as the page
    /// reads it after #. Never the key.
    static func address(port: UInt16, then: String? = nil) -> URL {
        URL(string: "http://localhost:\(port)/setup" + (then.map { "#\($0)" } ?? ""))!
    }

    /// The cookie the page is given in the browser, for the window's store.
    static func sessionCookie(key: String) -> HTTPCookie? {
        HTTPCookie(properties: [
            .name: DeckRouter.setupCookie, .value: key, .domain: "localhost", .path: "/setup",
            .sameSitePolicy: HTTPCookieStringPolicy.sameSiteStrict, HTTPCookiePropertyKey("HttpOnly"): "TRUE",
        ])
    }

    /// The setup page, or what it reads, from this Mac's Smolcorp.
    static func isSetupPage(_ url: URL?, port: UInt16) -> Bool {
        guard let url, url.scheme == "http", url.host == "localhost", url.port == Int(port) else { return false }
        return DeckRouter.isSetupPath(url.path)
    }

    enum Decision: Equatable { case allow, download, openOutside, cancel }

    /// What happens to a page asked for in the window: the setup page opens,
    /// a file it makes is saved, a link clicked to anywhere else opens in the
    /// browser, and nothing else happens.
    static func decide(_ url: URL?, port: UInt16, mainFrame: Bool, clicked: Bool, download: Bool) -> Decision {
        if download {
            // A file the page made, as a profile's export: blob:http://localhost:7710/…
            let made = url?.scheme == "blob" ? url.flatMap { URL(string: String($0.absoluteString.dropFirst("blob:".count))) } : url
            return isOwnOrigin(made, port: port) ? .download : .cancel
        }
        if isSetupPage(url, port: port) { return .allow }
        // The preview's frame, before it loads
        if !mainFrame, let url, url.scheme == "about" { return .allow }
        if clicked, let scheme = url?.scheme, ["http", "https", "mailto"].contains(scheme) { return .openOutside }
        return .cancel
    }

    private static func isOwnOrigin(_ url: URL?, port: UInt16) -> Bool {
        url?.scheme == "http" && url?.host == "localhost" && url?.port == Int(port)
    }

    // MARK: Showing it

    /// In front, with the keyboard. `then`, as the menu's Pair a Phone…
    /// asks, opens the page again at that.
    func show(then: String? = nil) {
        let window = self.window ?? makeWindow()
        if web == nil {
            open(in: window, then: then)
        } else if then != nil {
            web?.load(URLRequest(url: SettingsWindow.address(port: port, then: then)))
        }
        // A menu bar app is never active by itself, and the window would open behind others.
        NSApp.activate(ignoringOtherApps: true)
        window.makeKeyAndOrderFront(nil)
    }

    private func makeWindow() -> SettingsPanel {
        let window = SettingsPanel(contentRect: NSRect(origin: .zero, size: SettingsWindow.firstSize),
                                   styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false)
        window.title = "Smolcorp Settings"
        window.contentMinSize = SettingsWindow.minimumSize
        window.isReleasedWhenClosed = false
        window.tabbingMode = .disallowed
        window.delegate = self
        window.settings = self
        if !window.setFrameUsingName(SettingsWindow.frameName) { window.center() }
        window.setFrameAutosaveName(SettingsWindow.frameName)
        self.window = window
        return window
    }

    /// A page of its own each time the window opens: closed, it stops reading the deck.
    private func open(in window: NSWindow, then: String?) {
        let web = page(frame: window.contentLayoutRect, then: then)
        window.contentView = web
        window.makeFirstResponder(web)
        self.web = web
    }

    /// The setup page, loading, its cookie put in its store first.
    func page(frame: NSRect, then: String?) -> WKWebView {
        let configuration = WKWebViewConfiguration()
        configuration.websiteDataStore = store
        // For the page, if it wants it: it's in the app's window, not a browser.
        configuration.userContentController.addUserScript(WKUserScript(source: SettingsWindow.inApp, injectionTime: .atDocumentStart, forMainFrameOnly: true))
        let web = WKWebView(frame: frame, configuration: configuration)
        web.autoresizingMask = [.width, .height]
        web.navigationDelegate = self
        web.uiDelegate = self

        let address = SettingsWindow.address(port: port, then: then)
        guard let cookie = SettingsWindow.sessionCookie(key: setupKey) else {
            web.load(URLRequest(url: address))
            return web
        }
        store.httpCookieStore.setCookie(cookie) { [weak web] in web?.load(URLRequest(url: address)) }
        return web
    }

    /// `window.smolcorpApp`, and `data-smolcorp-app` on the page's <html>.
    static let inApp = """
        Object.defineProperty(window, 'smolcorpApp', { value: Object.freeze({ platform: 'mac', window: 'settings' }) });
        document.documentElement.dataset.smolcorpApp = 'mac';
        """

    func reload() { web?.reload() }

    func windowWillClose(_ notification: Notification) {
        web?.stopLoading()
        window?.contentView = nil
        web = nil
    }

    // MARK: Navigation

    func webView(_ webView: WKWebView, decidePolicyFor action: WKNavigationAction,
                 decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        let decision = SettingsWindow.decide(action.request.url, port: port, mainFrame: action.targetFrame?.isMainFrame ?? true,
                                             clicked: action.navigationType == .linkActivated, download: action.shouldPerformDownload)
        switch decision {
        case .allow: decisionHandler(.allow)
        case .download: decisionHandler(.download)
        case .openOutside:
            decisionHandler(.cancel)
            if let url = action.request.url { NSWorkspace.shared.open(url) }
        case .cancel: decisionHandler(.cancel)
        }
    }

    /// What the page can't show is a file to save.
    func webView(_ webView: WKWebView, decidePolicyFor response: WKNavigationResponse,
                 decisionHandler: @escaping (WKNavigationResponsePolicy) -> Void) {
        decisionHandler(response.canShowMIMEType ? .allow : .download)
    }

    /// A link to a new window opens in the browser instead, as the window has only the setup page.
    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
                 for action: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? {
        if let url = action.request.url, ["http", "https", "mailto"].contains(url.scheme ?? ""), !SettingsWindow.isSetupPage(url, port: port) {
            NSWorkspace.shared.open(url)
        }
        return nil
    }

    /// If the page's process stops, it starts again rather than leaving the window blank.
    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) { webView.reload() }

    // MARK: The page's alert, confirm and prompt, as sheets

    func webView(_ webView: WKWebView, runJavaScriptAlertPanelWithMessage message: String,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping () -> Void) {
        let alert = NSAlert()
        alert.messageText = message
        sheet(alert) { _ in completionHandler() }
    }

    func webView(_ webView: WKWebView, runJavaScriptConfirmPanelWithMessage message: String,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (Bool) -> Void) {
        let alert = NSAlert()
        alert.messageText = message
        alert.addButton(withTitle: "OK")
        alert.addButton(withTitle: "Cancel")
        sheet(alert) { completionHandler($0 == .alertFirstButtonReturn) }
    }

    func webView(_ webView: WKWebView, runJavaScriptTextInputPanelWithPrompt prompt: String, defaultText: String?,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (String?) -> Void) {
        let alert = NSAlert()
        alert.messageText = prompt
        alert.addButton(withTitle: "OK")
        alert.addButton(withTitle: "Cancel")
        let field = NSTextField(string: defaultText ?? "")
        field.frame = NSRect(x: 0, y: 0, width: 280, height: 24)
        alert.accessoryView = field
        alert.window.initialFirstResponder = field
        sheet(alert) { completionHandler($0 == .alertFirstButtonReturn ? field.stringValue : nil) }
    }

    /// On the window, or on its own if the window has gone meanwhile.
    private func sheet(_ alert: NSAlert, done: @escaping (NSApplication.ModalResponse) -> Void) {
        guard let window, window.isVisible else { return done(alert.runModal()) }
        alert.beginSheetModal(for: window, completionHandler: done)
    }

    // MARK: Files in and out

    /// A profile's file to import, as the page's file input asks.
    func webView(_ webView: WKWebView, runOpenPanelWith parameters: WKOpenPanelParameters,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping ([URL]?) -> Void) {
        let panel = NSOpenPanel()
        panel.canChooseFiles = true
        panel.canChooseDirectories = parameters.allowsDirectories
        panel.allowsMultipleSelection = parameters.allowsMultipleSelection
        guard let window else { return completionHandler(panel.runModal() == .OK ? panel.urls : nil) }
        panel.beginSheetModal(for: window) { completionHandler($0 == .OK ? panel.urls : nil) }
    }

    func webView(_ webView: WKWebView, navigationAction: WKNavigationAction, didBecome download: WKDownload) {
        download.delegate = self
    }

    func webView(_ webView: WKWebView, navigationResponse: WKNavigationResponse, didBecome download: WKDownload) {
        download.delegate = self
    }

    /// Saved where you say (Downloads to start with), under the name the page gave it.
    func download(_ download: WKDownload, decideDestinationUsing response: URLResponse, suggestedFilename: String,
                  completionHandler: @escaping (URL?) -> Void) {
        let panel = NSSavePanel()
        panel.nameFieldStringValue = suggestedFilename
        panel.directoryURL = FileManager.default.urls(for: .downloadsDirectory, in: .userDomainMask).first
        let chosen = { [weak self] (response: NSApplication.ModalResponse) in
            guard response == .OK, let url = panel.url else { return completionHandler(nil) }
            self?.saving[ObjectIdentifier(download)] = url
            // The panel has already asked whether to replace it; WebKit won't write over a file.
            guard FileManager.default.fileExists(atPath: url.path) else { return completionHandler(url) }
            let written = url.deletingLastPathComponent().appendingPathComponent(".\(UUID().uuidString).download")
            self?.replacing[ObjectIdentifier(download)] = (written, url)
            completionHandler(written)
        }
        guard let window else { return chosen(panel.runModal()) }
        panel.beginSheetModal(for: window, completionHandler: chosen)
    }

    /// Once saved, the file is shown in the Finder, as a browser would.
    func downloadDidFinish(_ download: WKDownload) {
        let saved = saving.removeValue(forKey: ObjectIdentifier(download))
        if let replaced = replacing.removeValue(forKey: ObjectIdentifier(download)) {
            do {
                _ = try FileManager.default.replaceItemAt(replaced.chosen, withItemAt: replaced.written)
            } catch {
                try? FileManager.default.removeItem(at: replaced.written)
                return failed(error)
            }
        }
        if let saved { NSWorkspace.shared.activateFileViewerSelecting([saved]) }
    }

    func download(_ download: WKDownload, didFailWithError error: Error, resumeData: Data?) {
        saving.removeValue(forKey: ObjectIdentifier(download))
        if let replaced = replacing.removeValue(forKey: ObjectIdentifier(download)) {
            try? FileManager.default.removeItem(at: replaced.written)
        }
        failed(error)
    }

    private func failed(_ error: Error) {
        let alert = NSAlert()
        alert.messageText = "Couldn't save the file"
        alert.informativeText = error.localizedDescription
        sheet(alert) { _ in }
    }
}

/// The settings window, which takes its keys itself: a menu bar app has no
/// menus for them. ⌘W closes it and ⌘R reloads the page; the editing keys
/// go to the page first, as a browser's would, so its own ⌘Z still works.
final class SettingsPanel: NSWindow {
    weak var settings: SettingsWindow?

    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        let modifiers = event.modifierFlags.intersection(.deviceIndependentFlagsMask)
        let key = event.charactersIgnoringModifiers?.lowercased() ?? ""
        if modifiers == .command && key == "w" { performClose(nil); return true }
        if modifiers == .command && key == "r" { settings?.reload(); return true }
        // The page has it first. What it leaves comes back here, sent again by WebKit.
        if super.performKeyEquivalent(with: event) { return true }
        guard let action = SettingsPanel.editing(key, modifiers) else { return false }
        return NSApp.sendAction(action, to: nil, from: self)
    }

    static func editing(_ key: String, _ modifiers: NSEvent.ModifierFlags) -> Selector? {
        switch (key, modifiers) {
        case ("x", .command): return #selector(NSText.cut(_:))
        case ("c", .command): return #selector(NSText.copy(_:))
        case ("v", .command): return #selector(NSText.paste(_:))
        case ("a", .command): return #selector(NSText.selectAll(_:))
        case ("z", .command): return Selector(("undo:"))
        case ("z", [.command, .shift]): return Selector(("redo:"))
        default: return nil
        }
    }
}
