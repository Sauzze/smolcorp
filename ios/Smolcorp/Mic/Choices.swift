import Foundation
import Observation
import SwiftUI

/// The picked agent's ☰ choices key, there while an agent asks you to
/// choose: its question and a button for each option, picked on the Mac by
/// its number, through the deck's usual /act, on the Wi-Fi too, as the ▲ ▼ ✓
/// keys go. Only keys go that way, never text: a typed answer, to the
/// menu's "Type something", goes only through Tailscale, as a permission's
/// command or file comes.
@MainActor
@Observable
final class Choices {
    struct Asking: Identifiable {
        let session: String
        let desk: String
        let choice: Snapshot.Choice
        var id: String { choice.id }
    }

    /// What a permission prompt is about, fetched as the sheet opens.
    enum Detail: Equatable {
        case none, loading, shown(String), needsTailscale
    }

    /// The sheet, open over this desk's question.
    var asking: Asking?
    private(set) var detail = Detail.none
    /// A typed answer being written, for the question open.
    var typed = ""

    private let link: DeckLink
    private let flash: (String) -> Void
    let tailscale: TailscaleWatch

    /// The key, as a button of the layout's, after ✎ message; the Mac never sees it pressed.
    static let button = "smolcorp-choices"

    init(link: DeckLink, flash: @escaping (String) -> Void) {
        self.link = link
        self.flash = flash
        tailscale = TailscaleWatch(link: link)
    }

    /// The key tapped over `session`'s desk.
    func open(_ session: String) {
        guard let desk = link.snapshot?.desks.first(where: { $0.id == session }), let choice = desk.choice else {
            return flash("Nothing to choose here")
        }
        if asking?.id != choice.id { typed = "" }
        asking = Asking(session: session, desk: desk.project, choice: choice)
        detail = choice.isPermission ? .loading : .none
        guard choice.isPermission else { return }
        Task {
            let query = "/choice?session=" + (session.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? "")
            let data = await link.get(query, over: .tailnet)
            let answer = data.flatMap { try? JSONSerialization.jsonObject(with: $0) as? [String: Any] }
            guard asking?.id == choice.id else { return }
            if let answer, answer["id"] as? String == choice.id {
                let text = answer["detail"] as? String ?? ""
                detail = text.isEmpty ? .none : .shown(text)
            } else {
                detail = .needsTailscale
            }
        }
    }

    /// A newer snapshot: a question no longer asked, or another in its place, takes the sheet away.
    func follow(_ snapshot: Snapshot?) {
        guard let asking else { return }
        let now = snapshot?.desks.first { $0.id == asking.session }?.choice
        if now?.id != asking.choice.id { close() }
    }

    func close() {
        asking = nil
        tailscale.stop()
    }

    /// The menu's "Type something", with `text`: through Tailscale only, Return and all.
    func answer(_ text: String, to asking: Asking) async {
        close()
        let words = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !words.isEmpty else { return }
        let encode = { (s: String) in s.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? "" }
        let query = "?session=" + encode(asking.session) + "&option=other&choice=" + encode(asking.choice.id)
        let body = (try? JSONSerialization.data(withJSONObject: ["text": words])).map { String(decoding: $0, as: UTF8.self) }
        guard let answer = await link.post("/choose" + query, body: body, over: .tailnet) else {
            return flash("Not sent: can't reach your Mac through Tailscale")
        }
        guard (200..<300).contains(answer.status) else {
            return flash("Not sent: " + (answer.body.isEmpty ? "the Mac answered \(answer.status)" : answer.body))
        }
        typed = ""
    }

    /// Option `n`, 1 first, tapped.
    func pick(_ n: Int, of asking: Asking) async {
        close()
        let query = "?session=" + (asking.session.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? "")
            + "&action=choose&option=\(n)&choice=" + (asking.choice.id.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? "")
        guard let answer = await link.post("/act" + query, body: nil) else { return flash("Not sent: can't reach your Mac") }
        guard !(200..<300).contains(answer.status) else { return }
        flash("Not sent: " + (answer.body.isEmpty ? "the Mac answered \(answer.status)" : answer.body))
    }

    /// The snapshot with the key in the layout's buttons, after ✎ message,
    /// while some desk asks you to choose: the layout's keys are every desk's.
    static func withChoicesKey(_ json: String) -> String {
        guard var object = (try? JSONSerialization.jsonObject(with: Data(json.utf8))) as? [String: Any] else { return json }
        let keys = object["keys"] as? [[String: Any]] ?? []
        guard keys.contains(where: { ($0["session"] as? [String: Any])?["choice"] is [String: Any] }) else { return json }
        var layout = object["layout"] as? [String: Any] ?? [:]
        let rows = layout["rows"] as? [Any] ?? []
        layout["rows"] = [["buttons": [["id": button, "label": "☰ choices"]]]] + rows
        object["layout"] = layout
        guard let data = try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]) else { return json }
        return String(decoding: data, as: UTF8.self)
    }
}

/// A question's options, one big button each; or, where one tap can't
/// answer it, the same, to read, with how to answer.
struct ChoiceSheet: View {
    let asking: Choices.Asking
    @Bindable var choices: Choices
    let pick: (Int) -> Void
    let answer: (String) -> Void
    let cancel: () -> Void
    /// Writing a typed answer, in the sheet's place.
    @State private var typing = false
    @Environment(\.verticalSizeClass) private var height

    private var detail: Choices.Detail { choices.detail }

    var body: some View {
        if typing {
            SendSheet(title: asking.choice.question, text: $choices.typed, blocked: choices.tailscale.blocked, typing: true,
                      typeOnly: false, deliver: { _ in answer(choices.typed) }, cancel: {
                          choices.tailscale.stop()
                          typing = false
                      })
        } else {
            options
        }
    }

    private var options: some View {
        VStack(alignment: .leading, spacing: height == .compact ? 8 : 12) {
            HStack(alignment: .firstTextBaseline) {
                Text(asking.desk.isEmpty ? "Choose" : asking.desk).font(Pixel.font(15, .bold)).foregroundStyle(Pixel.dim)
                Spacer(minLength: 8)
                Button("Cancel", action: cancel).buttonStyle(PixelKeyStyle(kind: .plain))
            }
            // All at once where they fit; scrolled where they don't, as sideways.
            ViewThatFits(in: .vertical) {
                list
                ScrollView { list }.scrollBounceBehavior(.basedOnSize)
            }
        }
    }

    private var list: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(asking.choice.question)
                .font(Pixel.font(18, .bold)).foregroundStyle(Pixel.ink)
                .fixedSize(horizontal: false, vertical: true)
            detailView
            ForEach(Array(asking.choice.options.enumerated()), id: \.offset) { index, option in
                Button {
                    pick(index + 1)
                } label: {
                    row(number: "\(index + 1)", label: option.label, description: option.description)
                }
                .buttonStyle(PixelKeyStyle(kind: .key))
                .disabled(!asking.choice.answerable)
            }
            if asking.choice.answerable && asking.choice.other {
                Button {
                    choices.tailscale.start()
                    typing = true
                } label: {
                    row(number: "\(asking.choice.options.count + 1)", label: "Type something…", description: "")
                }
                .buttonStyle(PixelKeyStyle(kind: .key))
            }
            if !asking.choice.answerable {
                Text("Answer with ▲ ▼ ✓ or on the Mac").font(Pixel.font(13, .medium)).foregroundStyle(Pixel.dim)
            }
        }
    }

    private func row(number: String, label: String, description: String) -> some View {
        HStack(alignment: .center, spacing: 10) {
            PixelBadge(text: number)
            VStack(alignment: .leading, spacing: 2) {
                Text(label).font(Pixel.font(16, .bold))
                if !description.isEmpty {
                    Text(description).font(Pixel.font(13, .medium)).opacity(0.7)
                }
            }
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
        }
        .padding(.vertical, 6)
    }

    @ViewBuilder private var detailView: some View {
        switch detail {
        case .none: EmptyView()
        case .loading: ProgressView().controlSize(.small).tint(Pixel.ink)
        case .shown(let text):
            Text(text).font(.system(size: 13, design: .monospaced)).foregroundStyle(Pixel.terminalText).textSelection(.enabled)
                .padding(10).frame(maxWidth: .infinity, alignment: .leading)
                .pixelFrame(Pixel.terminal)
        case .needsTailscale:
            Label("Details need Tailscale", systemImage: "network.slash").font(Pixel.font(13, .medium)).foregroundStyle(Pixel.warn)
        }
    }
}
