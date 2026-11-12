import Foundation
import Network

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
