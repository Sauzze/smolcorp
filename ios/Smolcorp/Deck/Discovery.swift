import Foundation
import Network
import Observation

/// Macs running Smolcorp on this Wi-Fi, by Bonjour (`_smolcorp._tcp`), each
/// resolved to an address. Browsing is also what asks for Local Network
/// access: until it's allowed, iOS refuses every connection to the Wi-Fi at
/// once, so the app browses first, from a screen that says why.
@MainActor
@Observable
final class Discovery {
    struct Found: Identifiable, Equatable {
        var id: String { name }
        let name: String
        let url: URL
    }

    enum Access { case unknown, allowed, denied }

    private(set) var found: [Found] = []
    private(set) var access = Access.unknown
    private var browser: NWBrowser?
    private var resolving: [String: NWConnection] = [:]

    func start() {
        guard browser == nil else { return }
        let browser = NWBrowser(for: .bonjourWithTXTRecord(type: "_smolcorp._tcp", domain: nil), using: .tcp)
        browser.stateUpdateHandler = { [weak self] state in
            Task { @MainActor in self?.changed(state) }
        }
        browser.browseResultsChangedHandler = { [weak self] results, _ in
            Task { @MainActor in self?.resolve(results) }
        }
        browser.start(queue: .main)
        self.browser = browser
    }

    func stop() {
        browser?.cancel()
        browser = nil
        resolving.values.forEach { $0.cancel() }
        resolving = [:]
    }

    private func changed(_ state: NWBrowser.State) {
        switch state {
        case .ready:
            access = .allowed
        case .waiting(let error), .failed(let error):
            if case .dns(let code) = error, code == DNSServiceErrorType(kDNSServiceErr_PolicyDenied) { access = .denied }
        default:
            break
        }
    }

    private func resolve(_ results: Set<NWBrowser.Result>) {
        access = .allowed
        let names = Set(results.compactMap(Discovery.name(of:)))
        found.removeAll { !names.contains($0.name) }
        for result in results {
            guard let name = Discovery.name(of: result), resolving[name] == nil, !found.contains(where: { $0.name == name }) else { continue }
            // Where it is: an IPv4 address, which every part of the deck takes as it is.
            let parameters = NWParameters.tcp
            (parameters.defaultProtocolStack.internetProtocol as? NWProtocolIP.Options)?.version = .v4
            let connection = NWConnection(to: result.endpoint, using: parameters)
            resolving[name] = connection
            connection.stateUpdateHandler = { [weak self] state in
                Task { @MainActor in
                    guard let self else { return }
                    switch state {
                    case .ready:
                        if case let .hostPort(host, port) = connection.currentPath?.remoteEndpoint,
                           let url = URL(string: "http://\(Discovery.text(of: host)):\(port.rawValue)") {
                            self.found.append(Found(name: name, url: url))
                        }
                        fallthrough
                    case .failed, .cancelled:
                        connection.cancel()
                        self.resolving[name] = nil
                    default:
                        break
                    }
                }
            }
            connection.start(queue: .main)
        }
    }

    private static func name(of result: NWBrowser.Result) -> String? {
        guard case let .service(name, _, _, _) = result.endpoint else { return nil }
        return name
    }

    private static func text(of host: NWEndpoint.Host) -> String {
        switch host {
        case .ipv4(let address): return "\(address)".components(separatedBy: "%").first ?? "\(address)"
        case .ipv6(let address): return "[\(address)]"
        case .name(let name, _): return name
        @unknown default: return "\(host)"
        }
    }
}

/// Pairing: the code shown on the Mac, for the key this phone keeps.
enum Pairing {
    enum Failure: Error, CustomStringConvertible {
        case refused(String), unreachable(String)
        var description: String {
            switch self {
            case .refused(let message), .unreachable(let message): return message
            }
        }
    }

    static func pair(at base: URL, code: String) async throws -> String {
        var request = URLRequest(url: base.appendingPathComponent("pair"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: ["code": code])
        let data: Data, response: URLResponse
        do {
            (data, response) = try await DeckSession.shared.session.data(for: request)
        } catch {
            throw Failure.unreachable("Can't reach \(base.host ?? "the Mac"): \(DeckLink.describe(error)).")
        }
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        let answer = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
        if status == 200, let key = answer?["key"] as? String, !key.isEmpty { return key }
        if status == 403, base.scheme == "https", answer == nil {
            throw Failure.refused("The Mac turned Tailscale away: turn the phone mic on in Smolcorp's setup.")
        }
        throw Failure.refused(answer?["error"] as? String ?? "The Mac answered \(status).")
    }
}
