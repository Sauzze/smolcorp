import Foundation
import Network

struct Request {
    let method: String
    let path: String
    let query: [String: String]
    let headers: [String: String] // names lowercased
    let body: Data
    /// Sent from this Mac to a local name: see `HTTPConnection.isLocal`.
    let isLocal: Bool
    /// Addressed to an IP address or a local name, as a phone on the network
    /// would. A web page using DNS rebinding arrives under its own domain.
    let isAddressedDirectly: Bool
    /// The address the request came from, to count wrong pairing codes by.
    let remoteAddress: String
    /// Headers that came more than once, by their lowercased names: where
    /// it matters, as for the hook's, the request isn't believed.
    var repeatedHeaders: Set<String> = []
    /// Personal build: addressed to a Tailscale name (`*.ts.net`) and come
    /// from this Mac, or passed on by a proxy on this Mac whatever its Host,
    /// as `tailscale serve` passes on a phone's request from the tailnet.
    /// DeckRouter lets it in only with the phone mic setting on.
    var isTailnet = false

    var json: [String: Any] {
        (try? JSONSerialization.jsonObject(with: body)) as? [String: Any] ?? [:]
    }

    /// A request as the server reads it: who it's for, from its Host and
    /// headers and whether it came from this Mac. Pure, for tests.
    static func addressed(method: String, path: String, query: [String: String], headers: [String: String], body: Data,
                          fromLoopback: Bool, remoteAddress: String, repeatedHeaders: Set<String> = []) -> Request {
        let host = Addressing.host(headers["host"])
        // Passed on by a proxy on this Mac (tailscale serve): from this Mac's
        // loopback, but for someone else, so never "this Mac" for setup or the hook.
        let proxied = Addressing.isProxied(headers)
        // And `tailscale serve` passes on whatever Host the device sent, so
        // such a request is the tailnet's even as `localhost` or an IP: only
        // let in with the phone mic on, its wrong codes counted as its own.
        let fromProxy = fromLoopback && proxied
        return Request(method: method, path: path, query: query, headers: headers, body: body,
                       isLocal: fromLoopback && !proxied && ["localhost", "127.0.0.1", "[::1]"].contains(host),
                       isAddressedDirectly: !fromProxy && Addressing.isDirectAddress(host),
                       remoteAddress: remoteAddress, repeatedHeaders: repeatedHeaders,
                       isTailnet: fromProxy || Addressing.isTailnet(host, fromLoopback: fromLoopback))
    }
}

struct Response {
    var status = 200
    var contentType = "text/plain; charset=utf-8"
    var body = Data()
    /// Who may show a page in a frame. No other site may, where it could lay
    /// something over the page and trick you into pressing a button; the
    /// setup page's preview of the deck is the one page this deck frames.
    var framedBy = Framing.noOne
    /// Any other headers, such as the setup page's cookie.
    var headers: [String: String] = [:]

    enum Framing {
        case noOne, thisDeck
    }

    static func status(_ code: Int, _ message: String = "") -> Response {
        Response(status: code, body: Data(message.utf8))
    }

    static func json(_ object: Any, status: Int = 200) -> Response {
        let data = (try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])) ?? Data("{}".utf8)
        return Response(status: status, contentType: "application/json", body: data)
    }

    static func file(_ url: URL) -> Response {
        guard let data = try? Data(contentsOf: url) else { return .status(404) }
        let type = ["png": "image/png", "js": "text/javascript; charset=utf-8", "m4a": "audio/mp4", "webm": "audio/webm"][url.pathExtension] ?? "text/html; charset=utf-8"
        return Response(contentType: type, body: data)
    }

    fileprivate static let reasons = [
        200: "OK", 204: "No Content", 400: "Bad Request", 401: "Unauthorized", 403: "Forbidden",
        404: "Not Found", 409: "Conflict", 500: "Internal Server Error",
    ]
}

/// What a request's Host header says about who it's for. Pure, for tests.
enum Addressing {
    /// The Host header's name, lowercased, without its port.
    static func host(_ header: String?) -> String {
        (header ?? "").lowercased().replacingOccurrences(of: #":\d+$"#, with: "", options: .regularExpression)
    }

    /// An IP address, localhost, or a Bonjour name — never someone's domain.
    static func isDirectAddress(_ host: String) -> Bool {
        host == "localhost" || host.hasSuffix(".local") || host.hasPrefix("[")
            || host.range(of: #"^\d{1,3}(\.\d{1,3}){3}$"#, options: .regularExpression) != nil
    }

    /// Personal build: a MagicDNS name in a tailnet, such as
    /// `my-mac.tail1234.ts.net`, reaching this server from this Mac's own
    /// loopback: how `tailscale serve` hands on a request from the tailnet.
    /// A web page can't use DNS rebinding with it: Tailscale, not the page,
    /// says where `ts.net` names point. The same name from anywhere else
    /// isn't believed.
    static func isTailnet(_ host: String, fromLoopback: Bool) -> Bool {
        fromLoopback && host.range(of: #"^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.){2}ts\.net$"#, options: .regularExpression) != nil
    }

    /// Whether a proxy passed the request on, as `tailscale serve` does: it
    /// says whom for in these headers, and sets them over any the sender put in.
    static func isProxied(_ headers: [String: String]) -> Bool {
        ["x-forwarded-for", "x-forwarded-host", "x-forwarded-proto", "forwarded", "tailscale-user-login"]
            .contains { headers[$0] != nil }
    }
}
