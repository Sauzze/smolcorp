import SwiftUI

/// The office, full screen, live from the Mac; the ⚙︎ key for the app's
/// settings in the room's corner, where the office's styles key is drawn
/// (its panel opens from settings), and the connection's label at the top.
struct DeckView: View {
    let link: DeckLink
    let unpair: () -> Void

    @State private var office: OfficeWeb
    @State private var mic: PhoneMic
    @State private var messages: Messages
    @State private var choices: Choices
    @State private var board: Board
    @State private var info: Info
    @State private var showingSettings = false
    /// The lite office, as the page keeps it: read as settings open.
    @State private var lite = true
    @State private var notifications = Notifications.shared
    @Environment(\.scenePhase) private var phase

    init(link: DeckLink, unpair: @escaping () -> Void) {
        self.link = link
        self.unpair = unpair
        let office = OfficeWeb()
        _office = State(initialValue: office)
        let mic = PhoneMic(link: link, office: office)
        _mic = State(initialValue: mic)
        _messages = State(initialValue: Messages(link: link, flash: { mic.flash($0) }))
        _choices = State(initialValue: Choices(link: link, flash: { mic.flash($0) }))
        _board = State(initialValue: Board(link: link))
        _info = State(initialValue: Info(link: link))
    }

    var body: some View {
        GeometryReader { outer in
            let insets = outer.safeAreaInsets
            let wide = outer.size.width + insets.leading + insets.trailing > outer.size.height + insets.top + insets.bottom
            // From the screen's own corner, as the page measures: the office goes edge to edge.
            ZStack(alignment: .topLeading) {
                OfficeView(office: office)
                gearButton
                    .frame(width: gearSize.width, height: gearSize.height)
                    .offset(gearSpot(wide: wide))
                micButton
                    .frame(height: micHeight)
                    .offset(micSpot(wide: wide))
                routeButton(wide: wide)
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topTrailing)
                    // Upright, beside the Dynamic Island in the strip the office leaves clear of it;
                    // on its side, in the margin it leaves on the right.
                    .padding(.trailing, wide ? max(4, (insets.trailing - 54) / 2) : 30)
                    .padding(.top, wide ? 14 : 16)
            }
            .ignoresSafeArea()
        }
        .background(Color.black)
            .overlay { panel }
            .onAppear {
                wire()
                notifications.start(link)
            }
            .onChange(of: phase, initial: true) { _, phase in
                if phase == .active { link.start() } else if phase == .background {
                    mic.cancelListening()
                    link.stop()
                }
            }
            // The other mic: the office redrawn for it, its FR/EN switch or the Mac's own.
            .onChange(of: mic.usesPhone) { _, _ in
                if let latest = link.latest { office.deliver(DeckView.shown(latest, mic: mic)) }
            }
            .onChange(of: link.status) { _, status in
                if case .connected = status {
                    Task { await mic.learnNames() }
                    Task { await notifications.look() }
                }
            }
            // A notification tapped: the office opens on its desk.
            .onChange(of: notifications.opened, initial: true) { _, _ in openTapped() }
            .onChange(of: lite) { _, on in office.setLite(on) }
    }

    /// What's waiting over the office, in the office's own look: one at a time.
    @ViewBuilder private var panel: some View {
        if showingSettings {
            PixelPanel(dismiss: { showingSettings = false }, tall: true) {
                SettingsView(link: link, mic: mic, notifications: notifications, lite: $lite, styles: openStyles,
                             unpair: unpair, close: { showingSettings = false })
            }
        } else if let heard = mic.heard {
            PixelPanel(dismiss: nil) {
                HeardSheet(heard: heard, deliver: { text, send in
                    Task { await mic.deliver(heard, text: text, send: send) }
                }, cancel: { mic.heard = nil })
            }
            .id(heard.id)
        } else if let writing = messages.writing {
            // Closed by a tap beside it, the draft stays for next time.
            PixelPanel(dismiss: messages.close) {
                SendSheet(title: writing.desk.isEmpty ? "Message" : writing.desk, text: messages.draft(writing.session),
                          blocked: messages.blocked, typing: true,
                          deliver: { send in Task { await messages.deliver(writing, send: send) } },
                          cancel: { messages.cancel(writing) })
            }
            .id(writing.id)
        } else if let asking = choices.asking {
            PixelPanel(dismiss: choices.close) {
                ChoiceSheet(asking: asking, choices: choices,
                            pick: { n in Task { await choices.pick(n, of: asking) } },
                            answer: { text in Task { await choices.answer(text, to: asking) } },
                            cancel: choices.close)
            }
            .id(asking.id)
        } else if let showing = board.showing {
            PixelPanel(dismiss: board.close, tall: true) {
                BoardSheet(board: board, close: board.close)
            }
            .id(showing.id)
        } else if let showing = info.showing {
            PixelPanel(dismiss: info.close) {
                InfoSheet(info: info, close: info.close)
            }
            .id(showing.id)
        }
    }

    /// The ⚙︎ key, drawn over the office's styles key, as big as it is.
    private var gearButton: some View {
        Button(action: openSettings) {
            Image(systemName: "gearshape.fill")
                .font(.system(size: max(13, gearSize.height * 0.6), weight: .bold))
                .foregroundStyle(.white)
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background {
                    ZStack {
                        PixelBox(step: Pixel.unit).fill(Color(hex: 0x5a6070)).offset(y: Pixel.unit)
                        PixelBox(step: Pixel.unit).fill(Color(hex: 0x3a4050))
                    }
                }
        }
        .accessibilityLabel("Settings")
    }

    private var gearSize: CGSize {
        guard let key = styleKey else { return CGSize(width: 30, height: 26) }
        return CGSize(width: max(24, key.width), height: max(22, key.height))
    }

    /// On the styles key; before it's drawn, the top-left corner's strip.
    private func gearSpot(wide: Bool) -> CGSize {
        guard let key = styleKey else { return CGSize(width: wide ? 8 : 30, height: 16) }
        return CGSize(width: key.midX - gearSize.width / 2, height: key.midY - gearSize.height / 2)
    }

    private func openSettings() {
        UIImpactFeedbackGenerator(style: .light).impactOccurred()
        Task {
            lite = await office.liteOn()
            await notifications.look()
        }
        showingSettings = true
    }

    /// Settings' "Office styles…": the office's own panel, as its key opens it.
    private func openStyles() {
        showingSettings = false
        guard let key = styleKey else { return }
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { office.openStyles(at: key) }
    }

    /// The desk a tapped notification is about, picked once the office has it.
    private func openTapped() {
        guard let session = notifications.opened else { return }
        Task {
            guard link.snapshot?.desks.contains(where: { $0.id == session }) == true, await office.select(session) else { return }
            if notifications.opened == session { notifications.opened = nil }
        }
    }

    /// Where the office's styles key is, for the mic's button beside it.
    @State private var styleKey: CGRect?

    /// The mic's switch, beside the office's styles key: a tap for the other
    /// mic, and it says which is in use. Auto is chosen in Settings.
    private var micButton: some View {
        Button {
            mic.switchSource()
            UIImpactFeedbackGenerator(style: .light).impactOccurred()
        } label: {
            HStack(spacing: 4) {
                Image(systemName: mic.usesPhone ? "iphone" : "laptopcomputer")
                Text(mic.usesPhone ? "iPhone mic" : "Mac mic")
                if mic.source == .auto { Text("auto").font(.system(size: 9, weight: .medium)).opacity(0.6) }
            }
            .font(.system(size: 12, weight: .semibold))
            .foregroundStyle(.white.opacity(0.9))
            .padding(.horizontal, 9)
            .frame(maxHeight: .infinity)
            .background(Color(white: 0.12).opacity(0.85), in: Capsule())
            .overlay(Capsule().strokeBorder(.white.opacity(0.15)))
        }
        .accessibilityLabel(mic.usesPhone ? "Microphone: iPhone. Tap for the Mac's." : "Microphone: Mac. Tap for the iPhone's.")
    }

    private var micHeight: CGFloat { min(30, max(24, styleKey?.height ?? 26)) }

    /// Just right of the styles key, centred on it; before it's drawn, the top-left corner's strip.
    private func micSpot(wide: Bool) -> CGSize {
        guard let key = styleKey else { return CGSize(width: wide ? 8 : 30, height: 16) }
        return CGSize(width: key.maxX + 6, height: key.midY - micHeight / 2)
    }

    private func routeButton(wide: Bool) -> some View {
        Button(action: openSettings) {
            Group {
                if wide {
                    VStack(spacing: 3) {
                        Circle().fill(dot).frame(width: 7, height: 7)
                        Text(routeName).font(.system(size: 10, weight: .semibold)).lineLimit(1).minimumScaleFactor(0.7)
                    }
                    .frame(width: 54, height: 38)
                    .background(.white.opacity(0.08), in: RoundedRectangle(cornerRadius: 10))
                } else {
                    HStack(spacing: 5) {
                        Circle().fill(dot).frame(width: 7, height: 7)
                        Text(routeName).font(.system(size: 12, weight: .semibold))
                    }
                    .padding(.horizontal, 10)
                    .frame(height: 26)
                    .background(.white.opacity(0.08), in: Capsule())
                }
            }
            .foregroundStyle(.white.opacity(0.8))
        }
        .accessibilityLabel("Settings, connected over \(routeName)")
    }

    /// How the Mac is reached, where you can see it at a glance.
    private var routeName: String {
        switch link.status {
        case .connected(let route): return route.rawValue
        case .connecting: return "Connecting"
        case .unreachable: return "Offline"
        case .unpaired: return "Unpaired"
        }
    }

    private var dot: Color {
        switch link.status {
        case .connected(.lan): return .green
        case .connected(.tailnet): return .blue
        case .connecting: return .yellow
        case .unreachable, .unpaired: return .red
        }
    }

    private func wire() {
        office.onWantsEvents = { [link, office, mic] in
            if let latest = link.latest { office.deliver(DeckView.shown(latest, mic: mic)) }
        }
        office.perform = { [link, mic, messages, choices, board, info] path, query, body in
            // The mic, held and let go: the phone's own, where the Mac takes its words.
            let asked = URLComponents(string: "x:/" + query)?.queryItems ?? []
            let value = { (name: String) in asked.first { $0.name == name }?.value }
            // The Mac's mic, chosen: the deck's usual action, as below.
            if path == "/act", value("action") == "mic" {
                let down = value("phase") == "down"
                if down ? (mic.usesPhone && link.snapshot?.phoneMicOn == true) : mic.isHolding {
                    if down { mic.press(value("session") ?? "") } else { mic.release() }
                    return (204, "", "")
                }
            }
            // ✎ message: the app's own sheet, never the Mac's button.
            if path == "/act", value("action") == "button", value("button") == Messages.button {
                messages.open(value("session") ?? "")
                return (204, "", "")
            }
            // ▤ conversation: the app's own blackboard.
            if path == "/act", value("action") == "button", value("button") == Board.button {
                info.close()
                if !board.open(value("session") ?? "") { mic.flash("No conversation for this agent") }
                return (204, "", "")
            }
            // ⓘ info: the app's own card, of what the Mac reports.
            if path == "/act", value("action") == "button", value("button") == Info.button {
                board.close()
                if !info.open(value("session") ?? "") { mic.flash("No info for this agent") }
                return (204, "", "")
            }
            // ☰ choices: the app's own sheet too.
            if path == "/act", value("action") == "button", value("button") == Choices.button {
                choices.open(value("session") ?? "")
                return (204, "", "")
            }
            return await link.post(path + DeckView.withoutKey(query), body: body)
        }
        office.onStyleKey = { styleKey = $0 }
        link.onSnapshot = { [office, mic, choices, link] json in
            office.deliver(DeckView.shown(json, mic: mic))
            choices.follow(link.snapshot)
            openTapped()
        }
        link.onPing = { [office] in office.ping() }
        link.onUnreachable = { [office] in office.unreachable() }
        office.load()
    }

    /// The snapshot as the office draws it here: with the ▤ conversation, ⓘ info and ✎ message keys, ☰ choices
    /// while someone asks you to choose, and the phone's mic when it's in use.
    static func shown(_ json: String, mic: PhoneMic) -> String {
        Board.withBoardKey(Info.withInfoKey(Messages.withMessageKey(Choices.withChoicesKey(mic.usesPhone ? PhoneMic.withPhoneMic(json) : json))))
    }

    /// The page's query without its `t=`: the key goes in a header, and an
    /// empty `t` would stand in for it on the Mac.
    static func withoutKey(_ query: String) -> String {
        guard var parts = URLComponents(string: "x:/" + query) else { return query }
        parts.queryItems = parts.queryItems?.filter { $0.name != "t" }
        guard let items = parts.queryItems, !items.isEmpty else { return "" }
        return "?" + (parts.percentEncodedQuery ?? "")
    }
}

