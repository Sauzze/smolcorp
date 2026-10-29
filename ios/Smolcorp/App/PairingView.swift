import SwiftUI

/// Pairing with the Mac: where it is, and the code it shows.
struct PairingView: View {
    let paired: (Mac, String) -> Void

    @State private var discovery = Discovery()
    @State private var lookedForMac = false
    @State private var choice = (Mac.lastKnown?.lan ?? Mac.defaultLAN).map(Choice.lan) ?? .typed
    @State private var typed = ""
    @State private var code = ""
    @State private var pairing = false
    @State private var problem: String?
    @FocusState private var codeFocused: Bool

    enum Choice: Hashable {
        case lan(URL), tailnet, typed
    }

    var body: some View {
        NavigationStack {
            Form {
                if !lookedForMac {
                    Section {
                        Text("Smolcorp looks for your Mac on this Wi-Fi. iOS will ask whether it may: tap Allow. Away from home, it goes through Tailscale instead.")
                        Button("Look for my Mac") {
                            lookedForMac = true
                            discovery.start()
                        }
                    }
                } else {
                    Section {
                        ForEach(discovery.found) { mac in
                            row("\(mac.name)", detail: mac.url.host ?? "", .lan(mac.url))
                        }
                        if let known = Mac.lastKnown?.lan ?? Mac.defaultLAN, !discovery.found.contains(where: { $0.url == known }) {
                            row(known.host ?? "Your Mac", detail: "this Mac's local name", .lan(known))
                        }
                        if let tailnet {
                            row("Tailscale", detail: tailnet.host ?? "", .tailnet)
                        }
                        row("Another address", detail: "", .typed)
                        if choice == .typed {
                            TextField("http://192.168.1.20:7710 or https://….ts.net", text: $typed)
                                .keyboardType(.URL)
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled()
                        }
                    } header: {
                        Text("Your Mac")
                    } footer: {
                        if discovery.access == .denied {
                            VStack(alignment: .leading, spacing: 6) {
                                Text("Local Network is off for Smolcorp, so it can't reach your Mac on the Wi-Fi. Tailscale still works.")
                                Button("Open Settings") {
                                    UIApplication.shared.open(URL(string: UIApplication.openSettingsURLString)!)
                                }
                            }
                        }
                    }
                }

                Section {
                    TextField("6-digit code", text: $code)
                        .keyboardType(.numberPad)
                        .textContentType(.oneTimeCode)
                        .font(.title2.monospacedDigit())
                        .focused($codeFocused)
                        .onChange(of: code) { _, new in code = String(new.filter(\.isNumber).prefix(6)) }
                    Button {
                        Task { await pair() }
                    } label: {
                        HStack {
                            Text("Pair")
                            if pairing { Spacer(); ProgressView() }
                        }
                    }
                    .disabled(code.count != 6 || pairing || address == nil)
                } header: {
                    Text("Code")
                } footer: {
                    Text("On your Mac, choose Pair a Phone… in Smolcorp's menu, then type the code it shows. It works for 5 minutes, once.")
                }

                if let problem {
                    Section { Text(problem).foregroundStyle(.red) }
                }
            }
            .navigationTitle("Pair with your Mac")
        }
        .onAppear {
            // Once Local Network has been answered, there's nothing to explain first.
            if Mac.lastKnown != nil {
                lookedForMac = true
                discovery.start()
            }
        }
        .onDisappear { discovery.stop() }
    }

    private func row(_ title: String, detail: String, _ value: Choice) -> some View {
        Button {
            choice = value
        } label: {
            HStack {
                VStack(alignment: .leading) {
                    Text(title).foregroundStyle(.primary)
                    if !detail.isEmpty { Text(detail).font(.caption).foregroundStyle(.secondary) }
                }
                Spacer()
                if choice == value { Image(systemName: "checkmark").foregroundStyle(.tint) }
            }
        }
    }

    /// Where the code goes.
    private var address: URL? {
        switch choice {
        case .lan(let url): return url
        case .tailnet: return tailnet
        case .typed:
            let text = typed.trimmingCharacters(in: .whitespaces)
            guard !text.isEmpty else { return nil }
            var url = URL(string: text.contains("://") ? text : "http://\(text)")
            if url?.port == nil, url?.scheme == "http", let host = url?.host { url = URL(string: "http://\(host):7710") }
            return url
        }
    }

    /// Your Mac through Tailscale's HTTPS, as last paired or as the build was given it.
    private var tailnet: URL? { Mac.lastKnown?.tailnet ?? Mac.defaultTailnet }

    /// Whether `url` is a Tailscale HTTPS name, as `tailscale serve` gives it.
    private static func isTailnet(_ url: URL) -> Bool {
        url.scheme == "https" && (url.host ?? "").hasSuffix(".ts.net")
    }

    private func pair() async {
        guard let address else { return }
        pairing = true
        problem = nil
        defer { pairing = false }
        do {
            let key = try await Pairing.pair(at: address, code: code)
            // A Tailscale address typed in is kept as the way in from away, as the build's would be.
            let throughTailnet = choice == .tailnet || PairingView.isTailnet(address)
            let lan = throughTailnet ? (Mac.lastKnown?.lan ?? Mac.defaultLAN ?? address) : address
            let name = discovery.found.first { $0.url == address }?.name ?? lan.host ?? "Mac"
            let mac = Mac(name: name, lan: lan, tailnet: PairingView.isTailnet(address) ? address : tailnet)
            mac.save(key: key)
            paired(mac, key)
        } catch {
            problem = "\(error)"
            code = ""
        }
    }
}
