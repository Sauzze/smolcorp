import SwiftUI
import UIKit
import WebKit

/// The Mac's deck page and its office, drawn in a web view from the copies in
/// the app, with its network handed to the app (see bridge.js).
@MainActor
final class OfficeWeb: NSObject {
    static let scheme = "smolcorp"
    static let page = URL(string: "smolcorp://deck/")!

    let webView: WKWebView
    /// The page opened its event stream: send it the latest snapshot.
    var onWantsEvents: () -> Void = {}
    /// A POST from the page: path, query (with its `?`) and body, answered
    /// with the status, body and type, or nil when the Mac can't be reached.
    var perform: (String, String, String?) async -> (status: Int, body: String, type: String)? = { _, _, _ in nil }

    private let haptics = UIImpactFeedbackGenerator(style: .light)

    init(extraScripts: [String] = []) {
        let configuration = WKWebViewConfiguration()
        configuration.setURLSchemeHandler(BundledFiles(), forURLScheme: OfficeWeb.scheme)
        configuration.allowsInlineMediaPlayback = true
        configuration.mediaTypesRequiringUserActionForPlayback = []
        configuration.suppressesIncrementalRendering = false
        let content = configuration.userContentController
        let bundled = { (name: String) in
            Bundle.main.url(forResource: name, withExtension: "js").flatMap { try? String(contentsOf: $0, encoding: .utf8) } ?? ""
        }
        for source in [bundled("bridge"), bundled("stylekey")] + extraScripts {
            content.addUserScript(WKUserScript(source: source, injectionTime: .atDocumentStart, forMainFrameOnly: true))
        }
        webView = WKWebView(frame: .zero, configuration: configuration)
        super.init()
        let relay = Relay(self)
        content.add(relay, name: "deck")
        content.add(relay, name: "haptic")
        content.add(relay, name: "spike")
        content.add(relay, name: "styleKey")
        content.addScriptMessageHandler(relay, contentWorld: .page, name: "deckFetch")
        webView.isInspectable = true
        webView.isOpaque = false
        webView.backgroundColor = .black
        webView.scrollView.isScrollEnabled = false
        webView.scrollView.bounces = false
        webView.scrollView.contentInsetAdjustmentBehavior = .never
        webView.allowsLinkPreview = false
    }

    func load() {
        webView.load(URLRequest(url: OfficeWeb.page))
    }

    /// A snapshot, as the Mac's stream sent it.
    func deliver(_ snapshot: String) {
        webView.callAsyncJavaScript("window.__smolcorp && __smolcorp.message(data)", arguments: ["data": snapshot],
                                    in: nil, in: .page) { _ in }
    }

    func ping() {
        webView.evaluateJavaScript("window.__smolcorp && __smolcorp.ping()") { _, _ in }
    }

    func unreachable() {
        webView.evaluateJavaScript("window.__smolcorp && __smolcorp.error()") { _, _ in }
    }

    /// Script in the page, its result back.
    func run(_ script: String, arguments: [String: Any] = [:]) async -> Any? {
        try? await webView.callAsyncJavaScript(script, arguments: arguments, contentWorld: .page)
    }

    /// Picks session `id`'s desk, as a tap on it does: false before the office is up.
    func select(_ id: String) async -> Bool {
        await run("return !!(window.__smolcorp && __smolcorp.select(id))", arguments: ["id": id]) as? Bool ?? false
    }

    /// The lite office, as the page keeps it on this phone.
    func liteOn() async -> Bool {
        await run("return !window.__smolcorp || __smolcorp.liteOn()") as? Bool ?? true
    }

    func setLite(_ on: Bool) {
        Task { _ = await run("window.__smolcorp && __smolcorp.setLite(on); return 1", arguments: ["on": on]) }
    }

    /// The office's own styles' panel, as a tap on its key opens it: `key` is where the key is.
    func openStyles(at key: CGRect) {
        Task { _ = await run("window.__smolcorp && __smolcorp.openStyles(x, y); return 1", arguments: ["x": key.midX, "y": key.midY]) }
    }

    /// Where the office's styles key is, in the view's points, as stylekey.js works it out; nil before it's drawn.
    var onStyleKey: (CGRect?) -> Void = { _ in }

    /// Messages the spike listens for.
    var onSpike: (Any) -> Void = { _ in }

    fileprivate func received(_ message: WKScriptMessage) {
        switch message.name {
        case "deck": onWantsEvents()
        case "haptic": haptics.impactOccurred()
        case "spike": onSpike(message.body)
        case "styleKey":
            let key = message.body as? [String: Any]
            let number = { (name: String) in (key?[name] as? NSNumber).map { CGFloat(truncating: $0) } }
            if let x = number("x"), let y = number("y"), let w = number("w"), let h = number("h"), w > 0, h > 0 {
                onStyleKey(CGRect(x: x, y: y, width: w, height: h))
            } else {
                onStyleKey(nil)
            }
        default: break
        }
    }

    fileprivate func fetch(_ body: Any) async -> Any? {
        guard let ask = body as? [String: Any], let path = ask["path"] as? String else { return nil }
        guard let answer = await perform(path, ask["query"] as? String ?? "", ask["body"] as? String) else { return nil }
        return ["status": answer.status, "body": answer.body, "type": answer.type]
    }
}

/// Holds the page's handlers without the content controller keeping the
/// office alive.
private final class Relay: NSObject, WKScriptMessageHandler, WKScriptMessageHandlerWithReply {
    weak var office: OfficeWeb?

    init(_ office: OfficeWeb) { self.office = office }

    func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage) {
        MainActor.assumeIsolated { office?.received(message) }
    }

    func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage,
                               replyHandler: @escaping (Any?, String?) -> Void) {
        let body = message.body
        Task { @MainActor [weak office] in
            replyHandler(await office?.fetch(body) ?? NSNull(), nil)
        }
    }
}

/// The page's own files, from the copy of the Mac's `web` folder in the app:
/// `/` is the deck, `/office.js` the office, `/sounds/…` its recordings.
private final class BundledFiles: NSObject, WKURLSchemeHandler {
    static let types = ["html": "text/html", "js": "text/javascript", "png": "image/png", "m4a": "audio/mp4",
                        "webm": "audio/webm", "css": "text/css"]

    func webView(_ webView: WKWebView, start task: WKURLSchemeTask) {
        guard let url = task.request.url, let web = Bundle.main.url(forResource: "web", withExtension: nil) else {
            return task.didFailWithError(URLError(.fileDoesNotExist))
        }
        let path = url.path.isEmpty || url.path == "/" ? "deck.html" : String(url.path.dropFirst())
        let file = web.appendingPathComponent(path).standardizedFileURL
        guard file.path.hasPrefix(web.standardizedFileURL.path + "/"), let data = try? Data(contentsOf: file) else {
            let missing = HTTPURLResponse(url: url, statusCode: 404, httpVersion: "HTTP/1.1", headerFields: [:])!
            task.didReceive(missing)
            task.didReceive(Data())
            return task.didFinish()
        }
        let type = BundledFiles.types[file.pathExtension] ?? "application/octet-stream"
        let response = HTTPURLResponse(url: url, statusCode: 200, httpVersion: "HTTP/1.1", headerFields: [
            "Content-Type": type.hasPrefix("text/") ? "\(type); charset=utf-8" : type,
            "Content-Length": String(data.count),
            "Cache-Control": "no-store",
        ])!
        task.didReceive(response)
        task.didReceive(data)
        task.didFinish()
    }

    func webView(_ webView: WKWebView, stop task: WKURLSchemeTask) {}
}

/// The office, filling the screen.
struct OfficeView: UIViewRepresentable {
    let office: OfficeWeb

    func makeUIView(context: Context) -> WKWebView { office.webView }
    func updateUIView(_ view: WKWebView, context: Context) {}
}
