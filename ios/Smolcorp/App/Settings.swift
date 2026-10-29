import SwiftUI
import UIKit

/// The app's settings, in one panel over the office, in its look: the Mac
/// and how it's reached, the mic, the office's styles and its lite switch,
/// the notifications, help, and the app's version. Opened by the ⚙︎ key in
/// the room's corner, where the office's own styles key was, or by the
/// connection's label at the top.
struct SettingsView: View {
    let link: DeckLink
    @Bindable var mic: PhoneMic
    @Bindable var notifications: Notifications
    /// The lite office, as the page keeps it, and its switch.
    @Binding var lite: Bool
    let styles: () -> Void
    let unpair: () -> Void
    let close: () -> Void
    @State private var confirmingUnpair = false
    @State private var showingNames = false

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline) {
                Text("⚙︎ Settings").font(Pixel.font(18, .bold)).foregroundStyle(Pixel.ink)
                Spacer(minLength: 8)
                Button("Done", action: close).buttonStyle(PixelKeyStyle(kind: .plain))
            }
            ScrollView {
                VStack(alignment: .leading, spacing: 18) {
                    macSection
                    micSection
                    officeSection
                    notificationsSection
                    helpSection
                    aboutSection
                }
                .padding(.vertical, 4)
            }
            .scrollBounceBehavior(.basedOnSize)
        }
        .foregroundStyle(Pixel.ink)
    }

    // MARK: Sections

    private var macSection: some View {
        section("Mac") {
            HStack(spacing: 8) {
                Circle().fill(dot).frame(width: 9, height: 9)
                Text(status).font(Pixel.font(15, .semibold))
            }
            row("Name", link.mac.name)
            row("Wi-Fi", link.mac.lan.host ?? link.mac.lan.absoluteString)
            row("Tailscale", link.mac.tailnet?.host ?? "none")
        }
    }

    private var micSection: some View {
        section("Mic", footer: "The iPhone mic hears you here and lets you check the words before they go, through Tailscale only. The Mac mic is the Mac's own dictation, or Claude Code's voice mode. Auto: the Mac's on the Wi-Fi, the iPhone's away. The key beside ⚙︎ switches too.") {
            HStack(spacing: 8) {
                ForEach(PhoneMic.Source.allCases) { source in
                    Button {
                        mic.source = source
                    } label: {
                        Text(Settings.short(source)).frame(maxWidth: .infinity)
                    }
                    .buttonStyle(PixelKeyStyle(kind: mic.source == source ? .send : .key))
                }
            }
            row("Names it listens for", mic.names.isEmpty ? "built in" : "\(mic.names.count), from the Mac")
        }
    }

    private var officeSection: some View {
        section("Office", footer: "Lite goes as a calm office does, at 30 frames a second, to spare the battery: on this iPhone only. A style is the room's, on every screen it's on.") {
            Toggle("Lite office on this iPhone", isOn: $lite).toggleStyle(PixelSwitch())
            Button {
                styles()
            } label: {
                Label("Office styles…", systemImage: "paintbrush.pointed").frame(maxWidth: .infinity)
            }
            .buttonStyle(PixelKeyStyle(kind: .key))
        }
    }

    private var notificationsSection: some View {
        section("Notifications", footer: "They say only the desk's name and \"finished\" or \"needs you\", or the GM's own few words: they pass through Apple. A tap opens the office on that desk.") {
            if notifications.mac == .noKey {
                warning("Notifications need a push key", detail: "On the Mac: the Keychain item smolcorp-apns, its account the key's ID.")
            }
            if notifications.allowed == .denied {
                warning("Notifications are off for Smolcorp", detail: "Turn them on in the iPhone's Settings › Smolcorp.")
            }
            Toggle("An agent finished", isOn: $notifications.finished).toggleStyle(PixelSwitch())
            Toggle("An agent needs you", isOn: $notifications.waiting).toggleStyle(PixelSwitch())
            Toggle("Messages from the GM", isOn: $notifications.gm).toggleStyle(PixelSwitch())
            if let problem = notifications.problem {
                Label(problem, systemImage: "network.slash").font(Pixel.font(13, .medium)).foregroundStyle(Pixel.warn)
            }
        }
        .disabled(notifications.mac == .noKey)
    }

    private var helpSection: some View {
        section("Help") {
            help("Desks", "Tap a desk to pick its agent. Hold it for the effort chips, and to fire it or hire into an empty one. Drag an agent to swap desks.")
            help("Keys", "Along the foot of the room, for the picked agent: esc, ⏎ send, /clear, ▲ ▼, then ✨ summary, ▤ conversation, ✎ message, and ☰ choices while one asks you to choose.")
            help("▤ Conversation", "The agent's own conversation: your words, its replies, each tool in a line, its questions and your answers. Newest at the foot; older ones on request. Through Tailscale only.")
            help("Mic", "Hold the office's mic and speak. The key beside ⚙︎ picks the iPhone's mic or the Mac's.")
            help("Two fingers", "Up and down the room, they scroll the agent's Terminal; a tap with two goes to its end.")
            help("Elevator", "It rides to the second floor, where runs without a terminal work.")
            Button {
                showingNames.toggle()
            } label: {
                HStack {
                    Text(showingNames ? "Hide the names it listens for" : "Names it listens for")
                    Spacer()
                    Image(systemName: showingNames ? "chevron.up" : "chevron.down")
                }
                .font(Pixel.font(14, .semibold))
            }
            .buttonStyle(.plain)
            if showingNames {
                Text((mic.names.isEmpty ? PhoneMic.builtInNames : mic.names).joined(separator: " · "))
                    .font(Pixel.font(13, .regular)).foregroundStyle(Pixel.dim)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    private var aboutSection: some View {
        section("This iPhone") {
            row("Version", Settings.version)
            Button(role: .destructive) {
                confirmingUnpair = true
            } label: {
                Text("Unpair this iPhone").frame(maxWidth: .infinity)
            }
            .buttonStyle(PixelKeyStyle(kind: .key))
            .confirmationDialog("Unpair this iPhone?", isPresented: $confirmingUnpair, titleVisibility: .visible) {
                Button("Unpair", role: .destructive, action: unpair)
            } message: {
                Text("The key is forgotten here, and your Mac is told to send no more notifications, through Tailscale. If Tailscale is off, notifications stop only once you unpair the phone in Smolcorp's setup, which also revokes its key there.")
            }
        }
    }

    // MARK: Pieces

    private func section<Content: View>(_ title: String, footer: String? = nil, @ViewBuilder _ content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title.uppercased()).font(Pixel.font(12, .bold)).foregroundStyle(Pixel.dim).kerning(0.6)
            VStack(alignment: .leading, spacing: 10) { content() }
                .padding(12)
                .frame(maxWidth: .infinity, alignment: .leading)
                .pixelFrame(Color.white.opacity(0.55), border: Color(hex: 0xd9d3c5), step: Pixel.unit * 2)
            if let footer {
                Text(footer).font(Pixel.font(12, .regular)).foregroundStyle(Pixel.dim).fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    private func row(_ name: String, _ value: String) -> some View {
        HStack(alignment: .firstTextBaseline) {
            Text(name).font(Pixel.font(14, .medium)).foregroundStyle(Pixel.dim)
            Spacer(minLength: 12)
            Text(value).font(Pixel.font(14, .semibold)).multilineTextAlignment(.trailing).lineLimit(2)
        }
    }

    private func help(_ name: String, _ text: String) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(name).font(Pixel.font(14, .bold))
            Text(text).font(Pixel.font(13, .regular)).foregroundStyle(Pixel.dim).fixedSize(horizontal: false, vertical: true)
        }
    }

    private func warning(_ title: String, detail: String) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Label(title, systemImage: "exclamationmark.triangle.fill").font(Pixel.font(14, .bold)).foregroundStyle(Pixel.warn)
            Text(detail).font(Pixel.font(12, .regular)).foregroundStyle(Pixel.dim).fixedSize(horizontal: false, vertical: true)
        }
    }

    private var status: String {
        switch link.status {
        case .connecting: return "Connecting…"
        case .connected(let route): return "Connected over \(route.rawValue)"
        case .unreachable(let why): return "Can't reach it (\(why))"
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
}

enum Settings {
    /// "0.3 (3)", from the app's own Info.plist.
    static var version: String {
        let info = Bundle.main.infoDictionary ?? [:]
        let short = info["CFBundleShortVersionString"] as? String ?? "?"
        let build = info["CFBundleVersion"] as? String ?? "?"
        return "\(short) (\(build))"
    }

    static func short(_ source: PhoneMic.Source) -> String {
        switch source {
        case .auto: return "Auto"
        case .phone: return "📱 iPhone"
        case .mac: return "💻 Mac"
        }
    }
}

/// A switch as the office draws its lite switch: a dark slot, its knob
/// white, lit in the deck's blue while it's on.
struct PixelSwitch: ToggleStyle {
    @Environment(\.isEnabled) private var enabled

    func makeBody(configuration: Configuration) -> some View {
        Button {
            configuration.isOn.toggle()
        } label: {
            HStack {
                configuration.label.font(Pixel.font(15, .semibold)).foregroundStyle(Pixel.ink)
                Spacer(minLength: 12)
                ZStack(alignment: configuration.isOn ? .trailing : .leading) {
                    PixelBox(step: Pixel.unit).fill(configuration.isOn ? Pixel.send : Pixel.key)
                    PixelBox(step: Pixel.unit).fill(Color.white).frame(width: 16, height: 18).padding(4)
                }
                .frame(width: 50, height: 26)
                .overlay(PixelBox(step: Pixel.unit).strokeBorder(Pixel.ink, lineWidth: Pixel.unit))
                .animation(.easeOut(duration: 0.12), value: configuration.isOn)
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .opacity(enabled ? 1 : 0.45)
        .accessibilityValue(configuration.isOn ? "On" : "Off")
    }
}
