import CryptoKit
import Foundation

/// Server-Sent Events as the Mac's deck sends them: `data:` lines, an
/// `event:` name for its pings, a blank line to end each. Pure, for tests.
struct SSEParser {
    struct Event: Equatable {
        var name: String
        var data: String
    }

    private var line = Data()
    private var name = ""
    private var data: [String] = []

    /// The events `chunk` completes, in order.
    mutating func feed(_ chunk: Data) -> [Event] {
        var events: [Event] = []
        for byte in chunk {
            guard byte == 0x0A else {
                line.append(byte)
                continue
            }
            if line.last == 0x0D { line.removeLast() }
            let text = String(decoding: line, as: UTF8.self)
            line.removeAll(keepingCapacity: true)
            if text.isEmpty {
                if !data.isEmpty { events.append(Event(name: name.isEmpty ? "message" : name, data: data.joined(separator: "\n"))) }
                name = ""
                data = []
            } else if text.hasPrefix(":") {
                continue
            } else {
                let field = text.split(separator: ":", maxSplits: 1, omittingEmptySubsequences: false)
                var value = field.count > 1 ? String(field[1]) : ""
                if value.hasPrefix(" ") { value.removeFirst() }
                switch field[0] {
                case "data": data.append(value)
                case "event": name = value
                default: break
                }
            }
        }
        return events
    }
}

/// Trusts the Mac's own certificate, by its SHA-256, where one was pinned at
/// pairing; anything else gets the system's usual checks.
final class PinningDelegate: NSObject, URLSessionDataDelegate {
    /// Lowercase hex SHA-256 of the leaf certificate's DER, by host.
    var pins: [String: String] = [:]

    func urlSession(_ session: URLSession, didReceive challenge: URLAuthenticationChallenge,
                    completionHandler: @escaping (URLSession.AuthChallengeDisposition, URLCredential?) -> Void) {
        handle(challenge, completionHandler)
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didReceive challenge: URLAuthenticationChallenge,
                    completionHandler: @escaping (URLSession.AuthChallengeDisposition, URLCredential?) -> Void) {
        handle(challenge, completionHandler)
    }

    private func handle(_ challenge: URLAuthenticationChallenge,
                        _ completion: @escaping (URLSession.AuthChallengeDisposition, URLCredential?) -> Void) {
        let space = challenge.protectionSpace
        guard space.authenticationMethod == NSURLAuthenticationMethodServerTrust, let trust = space.serverTrust,
              let pin = pins[space.host.lowercased()] else {
            return completion(.performDefaultHandling, nil)
        }
        guard let leaf = (SecTrustCopyCertificateChain(trust) as? [SecCertificate])?.first,
              PinningDelegate.fingerprint(of: leaf) == pin else {
            return completion(.cancelAuthenticationChallenge, nil)
        }
        completion(.useCredential, URLCredential(trust: trust))
    }

    static func fingerprint(of certificate: SecCertificate) -> String {
        SHA256.hash(data: SecCertificateCopyData(certificate) as Data).map { String(format: "%02x", $0) }.joined()
    }

    // MARK: Streams

    /// The open streams' tasks, each with what it's told.
    var streams: [Int: (parser: SSEParser, onEvent: (SSEParser.Event) -> Void, onEnd: (Int?, Error?) -> Void)] = [:]

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive response: URLResponse,
                    completionHandler: @escaping (URLSession.ResponseDisposition) -> Void) {
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        if status != 200, let stream = streams.removeValue(forKey: dataTask.taskIdentifier) {
            stream.onEnd(status, nil)
            return completionHandler(.cancel)
        }
        completionHandler(.allow)
    }

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
        guard var stream = streams[dataTask.taskIdentifier] else { return }
        let events = stream.parser.feed(data)
        streams[dataTask.taskIdentifier] = stream
        events.forEach(stream.onEvent)
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
        guard let stream = streams.removeValue(forKey: task.taskIdentifier) else { return }
        stream.onEnd((task.response as? HTTPURLResponse)?.statusCode, error)
    }
}

/// The app's one URLSession to the Mac: its pins, and its event streams,
/// all called back on the main queue.
final class DeckSession {
    static let shared = DeckSession()

    let delegate = PinningDelegate()
    let session: URLSession

    private init() {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.timeoutIntervalForRequest = 15
        configuration.requestCachePolicy = .reloadIgnoringLocalCacheData
        configuration.httpAdditionalHeaders = ["User-Agent": "Smolcorp iOS"]
        session = URLSession(configuration: configuration, delegate: delegate, delegateQueue: .main)
    }

    /// Opens `/events`: each event to `onEvent`, then `onEnd` once with the
    /// status (401: unpaired) or the error. Cancel the task to hang up.
    func stream(_ request: URLRequest, onEvent: @escaping (SSEParser.Event) -> Void,
                onEnd: @escaping (Int?, Error?) -> Void) -> URLSessionDataTask {
        var request = request
        request.timeoutInterval = 24 * 3600
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        let task = session.dataTask(with: request)
        delegate.streams[task.taskIdentifier] = (SSEParser(), onEvent, onEnd)
        task.resume()
        return task
    }
}
