import Foundation
import Observation
import SwiftUI

/// The picked agent's ✎ message key, beside its ✨ summary: words typed here
/// rather than said, in a sheet with the keyboard up, then sent to the Mac
/// as the iPhone mic's are, to `POST /dictate`, so the Mac's guards are the
/// same. Only through Tailscale's HTTPS, even on the Wi-Fi: never typed text
/// in plain http. A draft is kept for each desk till it goes or is cancelled.
@MainActor
@Observable
final class Messages {
    struct Writing: Identifiable {
        let session: String
        let desk: String
        var id: String { session }
    }

    /// The sheet, open over this desk.
    var writing: Writing?

    /// What's being written to each desk, by session: kept through the
    /// sheet closing by accident, and the app closing too.
    private var drafts = UserDefaults.standard.dictionary(forKey: "messageDrafts") as? [String: String] ?? [:] {
        didSet { UserDefaults.standard.set(drafts, forKey: "messageDrafts") }
    }

    private let link: DeckLink
    private let flash: (String) -> Void
    private let tailscale: TailscaleWatch

    /// The key, as a button of the layout's, which the office puts after
    /// ✨ summary; the Mac never sees it pressed (see DeckView.wire).
    static let button = "smolcorp-message"
    /// As much as the Mac types in one go.
    static let longest = 2000

    init(link: DeckLink, flash: @escaping (String) -> Void) {
        self.link = link
        self.flash = flash
        tailscale = TailscaleWatch(link: link)
    }

    /// The key tapped over `session`'s desk.
    func open(_ session: String) {
        guard !session.isEmpty else { return }
        let desk = link.snapshot?.desks.first { $0.id == session }?.project ?? ""
        writing = Writing(session: session, desk: desk)
        tailscale.start()
    }

    func draft(_ session: String) -> Binding<String> {
        Binding(get: { self.drafts[session] ?? "" },
                set: { self.drafts[session] = $0.isEmpty ? nil : String($0.prefix(Messages.longest)) })
    }

    /// Cancel: the draft goes too.
    func cancel(_ writing: Writing) {
        drafts[writing.session] = nil
        close()
    }

    /// The sheet gone, by Cancel, Send or a swipe.
    func close() {
        writing = nil
        tailscale.stop()
    }

    /// Send ⏎ or Type only. The draft stays till the Mac has taken it.
    func deliver(_ writing: Writing, send: Bool) async {
        close()
        let words = (drafts[writing.session] ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        guard !words.isEmpty else { return }
        let body = (try? JSONSerialization.data(withJSONObject: ["text": words, "send": send])).map { String(decoding: $0, as: UTF8.self) }
        let query = "?session=" + (writing.session.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? "")
        guard let answer = await link.post("/dictate" + query, body: body, over: .tailnet) else {
            return flash("Not sent: can't reach your Mac through Tailscale")
        }
        guard (200..<300).contains(answer.status) else {
            return flash("Not sent: " + (answer.body.isEmpty ? "the Mac answered \(answer.status)" : answer.body))
        }
        drafts[writing.session] = nil
    }

    /// Why Send waits, said in the sheet.
    var blocked: String? { tailscale.blocked }

    /// The snapshot with the key in the layout's buttons, first, so it sits
    /// right after ✨ summary in the picked agent's row.
    static func withMessageKey(_ json: String) -> String {
        guard var object = (try? JSONSerialization.jsonObject(with: Data(json.utf8))) as? [String: Any] else { return json }
        var layout = object["layout"] as? [String: Any] ?? [:]
        let rows = layout["rows"] as? [Any] ?? []
        layout["rows"] = [["buttons": [["id": button, "label": "✎ message"]]]] + rows
        object["layout"] = layout
        guard let data = try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]) else { return json }
        return String(decoding: data, as: UTF8.self)
    }
}

/// Whether Tailscale reaches the Mac from here, while a sheet that sends
/// text is open: already streaming through it, or a quick look every few
/// seconds, as Tailscale is turned on or off.
@MainActor
@Observable
final class TailscaleWatch {
    enum Reach: Equatable {
        case checking, ready, off
        /// It answered, and refused: the phone mic is off in the Mac's setup.
        case refused
    }

    private(set) var reach = Reach.checking
    private let link: DeckLink
    private var looking: Task<Void, Never>?

    init(link: DeckLink) {
        self.link = link
    }

    func start() {
        looking?.cancel()
        if case .connected(.tailnet) = link.status { reach = .ready } else { reach = .checking }
        looking = Task { [weak self] in
            while !Task.isCancelled {
                guard let self else { return }
                if case .connected(.tailnet) = self.link.status {
                    self.reach = .ready
                } else {
                    let status = await self.link.tailnetAnswer()
                    guard !Task.isCancelled else { return }
                    self.reach = status == 200 ? .ready : status == nil ? .off : .refused
                }
                try? await Task.sleep(for: .seconds(self.reach == .ready ? 10 : 3))
            }
        }
    }

    func stop() {
        looking?.cancel()
    }

    /// Why Send waits, said in the sheet.
    var blocked: String? {
        switch reach {
        case .checking: return "Checking Tailscale…"
        case .ready: return nil
        case .off: return "Turn on Tailscale to send text"
        case .refused: return "Your Mac takes text only with the phone mic on in its setup"
        }
    }
}
