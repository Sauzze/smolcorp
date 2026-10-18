import Foundation

/// A webhook for other tools: what the deck's sessions do, sent as it
/// happens to an address you give, such as Home Assistant's, so a lamp can
/// turn orange when an agent waits on you. Only addresses on your own
/// network, unless you allow others. Each message says what happened, to
/// which project and agent: never folders, prompts or what the agent wrote.
struct Webhook: Codable, Equatable {
    var enabled = false
    var url = ""
    /// Sent with each message, for the tool to check it's yours.
    var secret = ""
    /// Whether addresses outside your network are allowed.
    var outside = false
    var events = Webhook.standardEvents

    /// What can be sent: a session waiting on you, done, failed, getting to
    /// work, starting and ending.
    static let allEvents = ["waiting", "done", "error", "working", "started", "ended"]
    static let standardEvents = ["waiting", "done", "error"]
    /// How long a delivery may take, in seconds.
    static let timeout: TimeInterval = 3

    init() {}

    /// Field by field, so a file from another version still loads.
    init(from decoder: Decoder) throws {
        let c = try? decoder.container(keyedBy: CodingKeys.self)
        func text(_ key: CodingKeys) -> String { ((try? c?.decodeIfPresent(String.self, forKey: key)) ?? nil) ?? "" }
        enabled = ((try? c?.decodeIfPresent(Bool.self, forKey: .enabled)) ?? nil) ?? false
        url = text(.url)
        secret = text(.secret)
        outside = ((try? c?.decodeIfPresent(Bool.self, forKey: .outside)) ?? nil) ?? false
        let given = ((try? c?.decodeIfPresent([String].self, forKey: .events)) ?? nil) ?? Webhook.standardEvents
        events = Webhook.allEvents.filter { given.contains($0) }
    }

    /// Why `url` can't be sent to, or nil if it can: a web address, and one
    /// on your own network unless `outside`.
    static func problem(with url: String, outside: Bool) -> String? {
        guard let parsed = URL(string: url), let scheme = parsed.scheme?.lowercased(), ["http", "https"].contains(scheme),
              let host = parsed.host, !host.isEmpty else {
            return "That isn't a web address. It starts with http:// or https://."
        }
        if !outside && !isLocal(host) {
            return "\(host) isn't on your network. Tick \"Allow addresses outside your network\" to send there."
        }
        return nil
    }

    /// Whether `host` is on your own network: this computer, a name without
    /// dots or ending in .local, or a private address.
    static func isLocal(_ host: String) -> Bool {
        let name = host.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "[]"))
        if name == "localhost" || name.hasSuffix(".local") || name.hasSuffix(".localhost") || name.hasSuffix(".lan") || name.hasSuffix(".home.arpa") { return true }
        if !name.contains(".") && !name.contains(":") { return true }
        let parts = name.split(separator: ".").compactMap { Int($0) }
        if parts.count == 4 && name.split(separator: ".").count == 4 {
            switch (parts[0], parts[1]) {
            case (10, _), (127, _), (192, 168), (169, 254): return true
            case (172, 16...31): return true
            default: return false
            }
        }
        if name.contains(":") {
            return name == "::1" || name.hasPrefix("fc") || name.hasPrefix("fd") || name.hasPrefix("fe80")
        }
        return false
    }

    /// A redirect isn't followed: it could lead off your network, where only
    /// the address you gave was checked, and take the secret with it.
    private static let session = URLSession(configuration: .ephemeral, delegate: NoRedirects(), delegateQueue: nil)

    private final class NoRedirects: NSObject, URLSessionTaskDelegate {
        func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse,
                        newRequest request: URLRequest, completionHandler: @escaping (URLRequest?) -> Void) {
            completionHandler(nil)
        }
    }

    /// Sends `message`, and says how it went, on the main queue: "204", or what went wrong.
    func send(_ message: [String: Any], done: @escaping (String) -> Void) {
        guard let address = URL(string: url), let body = try? JSONSerialization.data(withJSONObject: message, options: [.sortedKeys]) else {
            return done("Nothing sent: the address isn't one it can send to.")
        }
        var request = URLRequest(url: address, timeoutInterval: Webhook.timeout)
        request.httpMethod = "POST"
        request.httpBody = body
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("Smolcorp", forHTTPHeaderField: "User-Agent")
        if !secret.isEmpty { request.setValue(secret, forHTTPHeaderField: "X-Smolcorp-Secret") }
        Webhook.session.dataTask(with: request) { _, response, error in
            let result: String
            if let status = (response as? HTTPURLResponse)?.statusCode {
                result = "\(status)"
            } else {
                result = error.map { "Couldn't reach it: \($0.localizedDescription)" } ?? "Couldn't reach it."
            }
            DispatchQueue.main.async { done(result) }
        }.resume()
    }
}
