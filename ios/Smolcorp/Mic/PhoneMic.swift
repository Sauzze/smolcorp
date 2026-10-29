import Foundation
import Observation
import SwiftUI
import UIKit

/// The office's mic, as the phone's own: held, this iPhone listens and the
/// words show over the desk; let go, they wait in a sheet to be checked,
/// then go to the Mac to be typed, sent or not. Only through Tailscale's
/// HTTPS for now: never in plain http on the Wi-Fi, where the key could be
/// seen and used to type a prompt of its own.
@MainActor
@Observable
final class PhoneMic {
    struct Heard: Identifiable {
        let id = UUID()
        let session: String
        let desk: String
        let text: String
    }

    /// Waiting in the sheet.
    var heard: Heard?

    private let listener = Listener()
    private let link: DeckLink
    private let office: OfficeWeb
    private var session: String?
    private var starting: Task<Void, Never>?
    private var problem: String?
    private var cap: Task<Void, Never>?
    /// The Mac's glossary, once it has said it.
    private(set) var names: [String] = []
    private let haptics = UIImpactFeedbackGenerator(style: .medium)

    /// Held longer, it stops by itself, as the Mac's dictation does.
    static let longest: Duration = .seconds(60)

    /// The names Smolcorp's dictation knows before setup changes them.
    static let builtInNames = ["Smolcorp", "Claude", "Claude Code", "Codex", "Gemini", "OpenCode", "Grok", "GitHub",
                               "Tailscale", "Cloudflare", "Vercel", "Supabase"]

    init(link: DeckLink, office: OfficeWeb) {
        self.link = link
        self.office = office
    }

    /// Which microphone the office's mic uses: this iPhone's, heard here and
    /// checked in the sheet, or the Mac's own (its dictation, or Claude
    /// Code's voice mode), with a headset at the desk say, through the deck's
    /// usual mic action. Auto: the Mac's on the Wi-Fi, where the Mac is near,
    /// the iPhone's through Tailscale. A choice made stays.
    enum Source: String, CaseIterable, Identifiable {
        case auto, phone, mac
        var id: String { rawValue }
        var name: String {
            switch self {
            case .auto: return "Auto: Mac on the Wi-Fi, iPhone away"
            case .phone: return "iPhone mic"
            case .mac: return "Mac mic"
            }
        }
    }

    var source = Source(rawValue: UserDefaults.standard.string(forKey: "micSource") ?? "") ?? .auto {
        didSet { UserDefaults.standard.set(source.rawValue, forKey: "micSource") }
    }

    /// Whether the office's mic is this iPhone's now. Only with Tailscale
    /// to send its words through.
    var usesPhone: Bool {
        guard link.mac.tailnet != nil else { return false }
        switch source {
        case .phone: return true
        case .mac: return false
        case .auto:
            if case .connected(.lan) = link.status { return false }
            return true
        }
    }

    /// A tap on the mic's switch: the other one, chosen for good.
    func switchSource() {
        source = usesPhone ? .mac : .phone
    }

    /// Held now: its release is this mic's, whichever is chosen meanwhile.
    var isHolding: Bool { session != nil }

    /// The Mac's glossary, asked for once connected; setup's built-in names
    /// and the desks' until then, or from a Mac too old to say.
    func learnNames() async {
        guard let data = await link.get("/glossary"),
              let names = (try? JSONSerialization.jsonObject(with: data) as? [String: Any])?["glossary"] as? [String] else { return }
        self.names = names
    }

    private var expected: [String] {
        let desks = link.snapshot?.desks.map(\.project) ?? []
        var seen = Set<String>()
        return ((names.isEmpty ? PhoneMic.builtInNames : names) + desks).filter { !$0.isEmpty && seen.insert($0.lowercased()).inserted }
    }

    // MARK: The office's mic

    /// The office's mic held over `session`'s desk.
    func press(_ session: String) {
        cancelListening()
        self.session = session
        problem = nil
        haptics.impactOccurred()
        showHearing("")
        let language = link.snapshot?.language ?? "en_US"
        starting = Task {
            do {
                try await listener.start(language: language, names: expected) { [weak self] text in
                    self?.showHearing(text)
                }
            } catch {
                problem = "\(error)"
            }
        }
        cap = Task { [weak self] in
            try? await Task.sleep(for: PhoneMic.longest)
            guard !Task.isCancelled else { return }
            self?.release()
        }
    }

    /// Let go: what was heard goes to the sheet.
    func release() {
        guard let session else { return }
        self.session = nil
        cap?.cancel()
        Task {
            await starting?.value
            let text = await listener.finish()
            showHearing(nil)
            if let problem { return flash(problem) }
            guard !text.isEmpty else { return flash("Heard nothing") }
            let desk = link.snapshot?.desks.first { $0.id == session }?.project ?? ""
            heard = Heard(session: session, desk: desk, text: text)
        }
    }

    /// The app went away mid-word: nothing is kept.
    func cancelListening() {
        cap?.cancel()
        starting?.cancel()
        listener.cancel()
        if session != nil { showHearing(nil) }
        session = nil
    }

    // MARK: Sending

    /// The sheet's Send (with Return) or Type only.
    func deliver(_ heard: Heard, text: String, send: Bool) async {
        self.heard = nil
        let words = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !words.isEmpty else { return }
        let body = (try? JSONSerialization.data(withJSONObject: ["text": words, "send": send])).map { String(decoding: $0, as: UTF8.self) }
        let query = "?session=" + (heard.session.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? "")
        // Gone, nothing is said: the sheet showed the words. Only what didn't go is.
        guard let answer = await link.post("/dictate" + query, body: body, over: .tailnet) else {
            return flash("Not sent: can't reach your Mac through Tailscale")
        }
        guard !(200..<300).contains(answer.status) else { return }
        flash("Not sent: " + (answer.body.isEmpty ? "the Mac answered \(answer.status)" : answer.body))
    }

    // MARK: The page

    private func showHearing(_ text: String?) {
        guard let session = session ?? (text == nil ? "" : nil) else { return }
        let hearing: Any = text.map { ["id": session, "text": $0] as [String: Any] } ?? NSNull()
        Task { _ = await office.run("if (typeof office !== 'undefined' && office.hearing) office.hearing(h); return 1", arguments: ["h": hearing]) }
    }

    func flash(_ text: String) {
        Task { _ = await office.run("if (typeof flash === 'function') flash(t); return 1", arguments: ["t": text]) }
    }

    /// The snapshot as the page should draw it with the phone's own mic: a mic
    /// on every desk the Mac can type into, its FR/EN switch, and not the Mac's
    /// clean-up switch, which this mic doesn't use. As the deck's page does
    /// over HTTPS.
    static func withPhoneMic(_ json: String) -> String {
        guard var object = (try? JSONSerialization.jsonObject(with: Data(json.utf8))) as? [String: Any],
              (object["phoneMic"] as? [String: Any])?["on"] as? Bool == true else { return json }
        if let keys = object["keys"] as? [[String: Any]] {
            object["keys"] = keys.map { key -> [String: Any] in
                guard var session = key["session"] as? [String: Any], session["phoneMic"] as? Bool == true else { return key }
                session["mic"] = true
                session["dictates"] = true
                var changed = key
                changed["session"] = session
                return changed
            }
        }
        var dictation = object["dictation"] as? [String: Any] ?? [:]
        dictation["on"] = true
        dictation["cleanup"] = nil
        object["dictation"] = dictation
        guard let data = try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]) else { return json }
        return String(decoding: data, as: UTF8.self)
    }
}

/// What was heard, to check before it goes: editable, then sent with Return,
/// typed without it, or dropped.
struct HeardSheet: View {
    let heard: PhoneMic.Heard
    let deliver: (String, Bool) -> Void
    let cancel: () -> Void
    @State private var text: String

    init(heard: PhoneMic.Heard, deliver: @escaping (String, Bool) -> Void, cancel: @escaping () -> Void) {
        self.heard = heard
        self.deliver = deliver
        self.cancel = cancel
        _text = State(initialValue: heard.text)
    }

    var body: some View {
        SendSheet(title: heard.desk.isEmpty ? "Dictation" : heard.desk, text: $text,
                  deliver: { deliver(text, $0) }, cancel: cancel)
    }
}

/// Words for an agent, to check or write before they go: Send ⏎, Type only
/// (no Return) or Cancel. The iPhone mic's, and the ✎ message key's.
struct SendSheet: View {
    let title: String
    @Binding var text: String
    /// Why the words can't go now, said over the buttons, which wait.
    var blocked: String? = nil
    /// The keyboard up from the start, and a word on what the Mac makes of it.
    var typing = false
    /// Type only, without Return: not for a menu's typed answer, which Return sends.
    var typeOnly = true
    let deliver: (Bool) -> Void
    let cancel: () -> Void
    @FocusState private var focused: Bool
    @Environment(\.verticalSizeClass) private var height

    var body: some View {
        let compact = height == .compact
        VStack(alignment: .leading, spacing: compact ? 8 : 12) {
            HStack(alignment: .firstTextBaseline) {
                Text(title).font(Pixel.font(17, .bold)).foregroundStyle(Pixel.ink).lineLimit(2)
                Spacer(minLength: 8)
                Button("Cancel", action: cancel).buttonStyle(PixelKeyStyle(kind: .plain))
            }
            TextEditor(text: $text)
                .font(Pixel.font(16, .regular))
                .foregroundStyle(Pixel.ink)
                .tint(Pixel.send)
                .scrollContentBackground(.hidden)
                .padding(6)
                .frame(minHeight: compact ? 56 : 96, maxHeight: compact ? 96 : 180)
                .pixelFrame(Pixel.field)
                .focused($focused)
            if let blocked {
                Label(blocked, systemImage: "network.slash").font(Pixel.font(13, .medium)).foregroundStyle(Pixel.warn)
            } else if typing {
                Text("Line breaks go as spaces.").font(Pixel.font(13, .medium)).foregroundStyle(Pixel.dim)
            }
            HStack(spacing: 10) {
                if typeOnly {
                    Button {
                        deliver(false)
                    } label: {
                        Text("Type only").frame(maxWidth: .infinity)
                    }
                    .buttonStyle(PixelKeyStyle(kind: .key))
                }
                Button {
                    deliver(true)
                } label: {
                    Text("Send ⏎").frame(maxWidth: .infinity)
                }
                .buttonStyle(PixelKeyStyle(kind: .send))
            }
            .disabled(blocked != nil || text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
        }
        .onAppear { if typing { focused = true } }
    }
}
