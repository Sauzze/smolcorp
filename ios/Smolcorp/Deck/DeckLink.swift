import Foundation
import Network
import Observation

/// The live line to the Mac: its event stream, kept open while the app is in
/// front, over the Wi-Fi when the Mac is there and through Tailscale when
/// not; and the page's POSTs, over whichever answered last.
@MainActor
@Observable
final class DeckLink {
    enum Route: String { case lan = "Wi-Fi", tailnet = "Tailscale" }
    enum Status: Equatable {
        case connecting
        case connected(Route)
        /// Neither route answered; trying again.
        case unreachable(String)
        /// The Mac no longer knows this phone.
        case unpaired
    }

    private(set) var status = Status.connecting
    /// The newest snapshot, as the Mac sent it, and what the app reads of it.
    private(set) var latest: String?
    private(set) var snapshot: Snapshot?

    var onSnapshot: (String) -> Void = { _ in }
    var onPing: () -> Void = {}
    var onUnreachable: () -> Void = {}
    var onUnpaired: () -> Void = {}

    let mac: Mac
    private let key: String
    private var stream: URLSessionDataTask?
    private var route: Route?
    private var heard = Date.distantPast
    private var retry: Task<Void, Never>?
    private var watchdog: Timer?
    private var failures = 0
    private var active = false
    private let paths = NWPathMonitor()
    private var onWiFi = true
    /// The network as last seen: its interfaces and whether it's up. Any
    /// change (Wi-Fi to 5G, one Wi-Fi to another) starts the stream again.
    private var network = ""
    private var probing = false

    /// The Mac pings every 40 s; two missed and the stream is taken for dead.
    static let quietFor: TimeInterval = 100
    /// How long the Wi-Fi gets before Tailscale is tried, when the Wi-Fi is up.
    static let lanTimeout: TimeInterval = 4

    init(mac: Mac, key: String) {
        self.mac = mac
        self.key = key
        DeckSession.shared.delegate.pins = mac.pins
        paths.pathUpdateHandler = { [weak self] path in
            let wifi = path.usesInterfaceType(.wifi) || path.usesInterfaceType(.wiredEthernet)
            let network = "\(path.status) " + path.availableInterfaces.map(\.name).joined(separator: ",")
            Task { @MainActor in self?.networkChanged(wifi: wifi, up: path.status == .satisfied, network: network) }
        }
        paths.start(queue: .main)
    }

    // MARK: Lifecycle

    /// In front: open the stream, and keep it open.
    func start() {
        guard !active else { return }
        active = true
        failures = 0
        connect()
        watchdog = Timer.scheduledTimer(withTimeInterval: 10, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated {
                guard let self, self.active else { return }
                if self.stream != nil, Date().timeIntervalSince(self.heard) > DeckLink.quietFor { return self.connect() }
                self.lookForWiFi()
            }
        }
    }

    /// Away: hang up. Reconnecting is cheap, and the Mac keeps nothing for us.
    func stop() {
        active = false
        retry?.cancel()
        watchdog?.invalidate()
        hangUp()
    }

    private func networkChanged(wifi: Bool, up: Bool, network: String) {
        let changed = network != self.network
        self.network = network
        onWiFi = wifi
        // A new network: the old stream may be dead without knowing it, and
        // the best route may be another: the Wi-Fi first, then Tailscale.
        guard active, up, changed || stream == nil else { return }
        failures = 0
        connect()
    }

    /// On a Wi-Fi but through Tailscale, as at home with the Mac asleep a
    /// moment ago: every 10 s, a quick look whether the Mac answers on the
    /// Wi-Fi, and if it does, the stream moves there.
    private func lookForWiFi() {
        guard onWiFi, route == .tailnet, !probing else { return }
        probing = true
        var request = URLRequest(url: mac.lan)
        request.timeoutInterval = 3
        Task {
            let answered = (try? await DeckSession.shared.session.data(for: request))
                .map { ($0.1 as? HTTPURLResponse)?.statusCode == 200 } ?? false
            probing = false
            if answered, active, route == .tailnet { connect() }
        }
    }

    // MARK: The stream

    /// The routes to try, best first: the Wi-Fi, when on one, then Tailscale.
    private var routes: [Route] {
        var routes: [Route] = onWiFi ? [.lan] : []
        if mac.tailnet != nil { routes.append(.tailnet) }
        if routes.isEmpty { routes = [.lan] }
        return routes
    }

    func base(_ route: Route) -> URL? {
        route == .lan ? mac.lan : mac.tailnet
    }

    private func connect() {
        retry?.cancel()
        hangUp()
        if case .connected = status {} else if status != .unpaired { status = .connecting }
        attempt(routes, problems: [])
    }

    /// Tries each route in turn, the next once one fails or stays silent.
    private func attempt(_ left: [Route], problems: [String]) {
        guard active else { return }
        guard let route = left.first, let base = base(route) else { return giveUp(problems) }
        // Silence is counted from now: a slow route isn't taken for a dead stream.
        heard = Date()
        var request = URLRequest(url: base.appendingPathComponent("events"))
        request.setValue(key, forHTTPHeaderField: "X-AgentDeck-Token")
        var opened = false
        let task = DeckSession.shared.stream(request, onEvent: { [weak self] event in
            guard let self else { return }
            self.heard = Date()
            if !opened {
                opened = true
                self.route = route
                self.failures = 0
                self.status = .connected(route)
            }
            if event.name == "ping" { return self.onPing() }
            self.received(event.data)
        }, onEnd: { [weak self] status, error in
            // Only the current stream ends here: one hung up on purpose is forgotten first.
            guard let self else { return }
            self.stream = nil
            if status == 401 { return self.unpaired() }
            let problem = "\(route.rawValue): " + (status.map { "answered \($0)" } ?? DeckLink.describe(error))
            if opened {
                // It was working and dropped: start over from the best route.
                self.route = nil
                self.onUnreachable()
                self.status = .unreachable(problem)
                self.retryLater()
            } else {
                self.attempt(Array(left.dropFirst()), problems: problems + [problem])
            }
        })
        stream = task
        // The Wi-Fi gets a few seconds: a Mac that isn't there often doesn't answer at all.
        if left.count > 1 {
            let id = task.taskIdentifier
            DispatchQueue.main.asyncAfter(deadline: .now() + DeckLink.lanTimeout) { [weak self] in
                guard let self, !opened, self.stream?.taskIdentifier == id else { return }
                self.hangUp()
                self.attempt(Array(left.dropFirst()), problems: problems + ["\(route.rawValue): no answer"])
            }
        }
    }

    private func hangUp() {
        guard let stream else { return }
        self.stream = nil
        DeckSession.shared.delegate.streams[stream.taskIdentifier] = nil
        stream.cancel()
    }

    private func giveUp(_ problems: [String]) {
        route = nil
        onUnreachable()
        status = .unreachable(problems.joined(separator: " · "))
        retryLater()
    }

    /// 1, 2, 4… up to 30 s between tries.
    private func retryLater() {
        guard active else { return }
        failures += 1
        let delay = min(30, pow(2, Double(failures - 1)))
        retry = Task { [weak self] in
            try? await Task.sleep(for: .seconds(delay))
            guard !Task.isCancelled else { return }
            self?.connect()
        }
    }

    private func received(_ json: String) {
        latest = json
        snapshot = Snapshot(json)
        onSnapshot(json)
    }

    private func unpaired() {
        stop()
        status = .unpaired
        Mac.forgetKey()
        onUnpaired()
    }

    // MARK: Requests

    /// A POST from the page, over the route the stream uses, or the best one.
    /// Nil when the Mac can't be reached.
    func post(_ pathAndQuery: String, body: String?, contentType: String? = nil,
              over only: Route? = nil, timeout: TimeInterval? = nil) async -> (status: Int, body: String, type: String)? {
        let chosen = only ?? route ?? routes.first ?? .lan
        guard let base = base(chosen), let url = URL(string: base.absoluteString.trimmingSuffix("/") + pathAndQuery) else { return nil }
        var request = URLRequest(url: url)
        if let timeout { request.timeoutInterval = timeout }
        request.httpMethod = "POST"
        request.setValue(key, forHTTPHeaderField: "X-AgentDeck-Token")
        if let body {
            request.httpBody = Data(body.utf8)
            request.setValue(contentType ?? "application/json", forHTTPHeaderField: "Content-Type")
        }
        guard let (data, response) = try? await DeckSession.shared.session.data(for: request),
              let http = response as? HTTPURLResponse else { return nil }
        if http.statusCode == 401 { unpaired() }
        return (http.statusCode, String(decoding: data, as: UTF8.self), http.value(forHTTPHeaderField: "Content-Type") ?? "")
    }

    func get(_ path: String, over only: Route? = nil) async -> Data? {
        let chosen = only ?? route ?? routes.first ?? .lan
        guard let base = base(chosen), let url = URL(string: base.absoluteString.trimmingSuffix("/") + path) else { return nil }
        var request = URLRequest(url: url)
        request.setValue(key, forHTTPHeaderField: "X-AgentDeck-Token")
        guard let (data, response) = try? await DeckSession.shared.session.data(for: request),
              (response as? HTTPURLResponse)?.statusCode == 200 else { return nil }
        return data
    }

    /// A GET's status and body, whatever the status, or nil for no answer.
    func fetch(_ path: String, over only: Route? = nil, timeout: TimeInterval = 10) async -> (status: Int, data: Data)? {
        let chosen = only ?? route ?? routes.first ?? .lan
        guard let base = base(chosen), let url = URL(string: base.absoluteString.trimmingSuffix("/") + path) else { return nil }
        var request = URLRequest(url: url)
        request.timeoutInterval = timeout
        request.setValue(key, forHTTPHeaderField: "X-AgentDeck-Token")
        guard let (data, response) = try? await DeckSession.shared.session.data(for: request),
              let http = response as? HTTPURLResponse else { return nil }
        if http.statusCode == 401 { unpaired() }
        return (http.statusCode, data)
    }

    /// Whether Tailscale reaches the Mac now, whatever the stream uses: what
    /// it answered a key-checked GET, quickly, or nil for no answer.
    func tailnetAnswer() async -> Int? {
        guard let base = mac.tailnet, let url = URL(string: base.absoluteString.trimmingSuffix("/") + "/glossary") else { return nil }
        var request = URLRequest(url: url)
        request.timeoutInterval = 4
        request.setValue(key, forHTTPHeaderField: "X-AgentDeck-Token")
        guard let (_, response) = try? await DeckSession.shared.session.data(for: request) else { return nil }
        return (response as? HTTPURLResponse)?.statusCode
    }

    nonisolated static func describe(_ error: Error?) -> String {
        guard let error = error as? URLError else { return error.map { "\($0.localizedDescription)" } ?? "closed" }
        switch error.code {
        case .notConnectedToInternet:
            // Also what iOS says with Local Network access off.
            return "no network, or Local Network is off for Smolcorp"
        case .timedOut: return "no answer"
        case .cannotFindHost, .dnsLookupFailed: return "not found"
        case .cannotConnectToHost: return "refused"
        case .networkConnectionLost: return "dropped"
        case .cancelled: return "refused (certificate)"
        case .secureConnectionFailed, .serverCertificateUntrusted: return "TLS failed"
        default: return error.localizedDescription
        }
    }
}

extension String {
    func trimmingSuffix(_ suffix: String) -> String {
        hasSuffix(suffix) ? String(dropLast(suffix.count)) : self
    }
}

/// What the app itself reads of a snapshot: who waits on you, the language.
struct Snapshot {
    struct Desk {
        let id: String
        let project: String
        let state: String
        let agentName: String
        let errorKind: String?
        let phoneMic: Bool
        /// Whether its conversation can be read (through Tailscale).
        let conversation: Bool
        /// What it asks you to choose, while it asks.
        let choice: Choice?
    }

    /// A question's options, or a permission's Yes and No, as the Mac sends
    /// them: a permission's command or file only comes from GET /choice.
    struct Choice: Equatable {
        struct Option: Equatable {
            let label: String
            let description: String
        }

        let id: String
        let isPermission: Bool
        let question: String
        let options: [Option]
        /// One tap picks an option: not with several answers or questions.
        let answerable: Bool
        /// The menu takes a typed answer, its "Type something".
        let other: Bool

        init?(_ object: Any?) {
            guard let object = object as? [String: Any], let id = object["id"] as? String, !id.isEmpty else { return nil }
            self.id = id
            isPermission = object["kind"] as? String == "permission"
            question = object["question"] as? String ?? ""
            options = (object["options"] as? [[String: Any]] ?? []).map {
                Option(label: $0["label"] as? String ?? "", description: $0["description"] as? String ?? "")
            }
            answerable = object["answerable"] as? Bool ?? false
            other = object["other"] as? Bool ?? false
        }
    }

    let desks: [Desk]
    let language: String
    let phoneMicOn: Bool

    init?(_ json: String) {
        guard let object = try? JSONSerialization.jsonObject(with: Data(json.utf8)) as? [String: Any] else { return nil }
        let keys = object["keys"] as? [[String: Any]] ?? []
        let upstairs = (object["upstairs"] as? [String: Any])?["desks"] as? [[String: Any]] ?? []
        desks = (keys.compactMap { $0["session"] as? [String: Any] } + upstairs).map { s in
            Desk(id: s["id"] as? String ?? "", project: s["project"] as? String ?? "", state: s["state"] as? String ?? "",
                 agentName: s["agentName"] as? String ?? "", errorKind: s["errorKind"] as? String,
                 phoneMic: s["phoneMic"] as? Bool ?? false, conversation: s["conversation"] as? Bool ?? false,
                 choice: Choice(s["choice"]))
        }
        language = (object["dictation"] as? [String: Any])?["language"] as? String ?? "en_US"
        phoneMicOn = (object["phoneMic"] as? [String: Any])?["on"] as? Bool ?? false
    }

    var waiting: Int { desks.filter { $0.state == "asking" }.count }
}
