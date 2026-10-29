import Foundation
import Observation
import SwiftUI

/// The blackboard: the picked agent's conversation as its transcript has it,
/// from the Mac's `GET /conversation`, through Tailscale only, as it reads
/// what the agent's session wrote. The newest page first, then older ones on
/// request; while it's up, the newest page again every few seconds, which
/// costs the Mac nothing while the file hasn't grown.
@MainActor
@Observable
final class Board {
    struct Item: Identifiable, Equatable {
        enum Kind: String { case user, peer, agent, tool, result, question, answer, note }
        let id: String
        let offset: UInt64
        let n: Int
        let kind: Kind
        let text: String
        let more: Bool
        let from: String

        init?(_ object: [String: Any]) {
            guard let id = object["id"] as? String, let kind = Kind(rawValue: object["kind"] as? String ?? ""),
                  let offset = (object["offset"] as? NSNumber)?.uint64Value else { return nil }
            self.id = id
            self.offset = offset
            n = object["n"] as? Int ?? 0
            self.kind = kind
            text = object["text"] as? String ?? ""
            more = object["more"] as? Bool ?? false
            from = object["from"] as? String ?? ""
        }

        var order: (UInt64, Int) { (offset, n) }
    }

    enum Problem: Equatable {
        case needsTailscale, refused, gone(String)
    }

    struct Showing: Identifiable {
        let session: String
        let desk: String
        var id: String { session }
    }

    /// The board, up over this desk.
    private(set) var showing: Showing?
    /// Oldest first.
    private(set) var items: [Item] = []
    /// Where the next older page ends; nil once the start is here.
    private(set) var before: UInt64?
    private(set) var loading = false
    private(set) var loadingOlder = false
    private(set) var problem: Problem?
    /// Bumped when newer messages arrive, for the board to follow them down.
    private(set) var arrived = 0

    private var size: UInt64?
    private var following: Task<Void, Never>?
    private let link: DeckLink

    /// The key, as a button of the layout's, first in the row, right after ✨ summary; the Mac never sees it pressed.
    static let button = "smolcorp-conversation"
    /// How often the newest page is looked at again, while the board is up.
    static let every: Duration = .seconds(3)

    init(link: DeckLink) {
        self.link = link
    }

    /// The key tapped over `session`'s desk, or the board asked for it.
    func open(_ session: String) -> Bool {
        guard let desk = link.snapshot?.desks.first(where: { $0.id == session }), desk.conversation else { return false }
        if showing?.session != session {
            items = []
            before = nil
            size = nil
        }
        showing = Showing(session: session, desk: desk.project)
        problem = nil
        loading = items.isEmpty
        following?.cancel()
        following = Task { [weak self] in
            while !Task.isCancelled {
                await self?.newest()
                try? await Task.sleep(for: Board.every)
            }
        }
        return true
    }

    func close() {
        following?.cancel()
        following = nil
        showing = nil
        loading = false
        loadingOlder = false
    }

    private static func query(_ session: String) -> String {
        "/conversation?session=" + (session.addingPercentEncoding(withAllowedCharacters: .alphanumerics) ?? "")
    }

    /// The newest page, added to what's here: only what's new comes in.
    private func newest() async {
        guard let session = showing?.session else { return }
        var path = Board.query(session)
        if let size, !items.isEmpty { path += "&size=\(size)" }
        let answer = await link.fetch(path, over: .tailnet)
        guard showing?.session == session else { return }
        loading = false
        guard let answer else { return problem = .needsTailscale }
        switch answer.status {
        case 204: problem = nil
        case 200:
            problem = nil
            guard let page = Board.page(answer.data) else { return }
            size = page.size
            let known = Set(items.map(\.id))
            let fresh = page.items.filter { !known.contains($0.id) }
            if items.isEmpty { before = page.before }
            // A first page, or a gap after a long time away: the newest page in place of what was here.
            if let first = page.items.first, let last = items.last, first.order > last.order, page.before != nil,
               !known.contains(first.id) {
                items = page.items
                before = page.before
                arrived += 1
                return
            }
            guard !fresh.isEmpty else { return }
            items = (items + fresh).sorted { $0.order < $1.order }
            arrived += 1
        case 403: problem = .refused
        default: problem = .gone(Board.message(answer.data) ?? "The Mac answered \(answer.status).")
        }
    }

    /// The page before the oldest here.
    func older() async {
        guard let session = showing?.session, let before, !loadingOlder else { return }
        loadingOlder = true
        defer { loadingOlder = false }
        let answer = await link.fetch(Board.query(session) + "&before=\(before)", over: .tailnet)
        guard showing?.session == session else { return }
        guard let answer else { return problem = .needsTailscale }
        guard answer.status == 200, let page = Board.page(answer.data) else {
            return problem = answer.status == 403 ? .refused : .gone(Board.message(answer.data) ?? "The Mac answered \(answer.status).")
        }
        let known = Set(items.map(\.id))
        items = (page.items.filter { !known.contains($0.id) } + items).sorted { $0.order < $1.order }
        self.before = page.before
    }

    static func page(_ data: Data) -> (items: [Item], before: UInt64?, size: UInt64)? {
        guard let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return nil }
        let items = (object["items"] as? [[String: Any]] ?? []).compactMap(Item.init)
        return (items, (object["before"] as? NSNumber)?.uint64Value, (object["size"] as? NSNumber)?.uint64Value ?? 0)
    }

    private static func message(_ data: Data) -> String? {
        let text = String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
        return text.isEmpty || text.count > 200 ? nil : text
    }

    /// The snapshot with the key in the layout's buttons, first, so it sits
    /// right after ✨ summary in the picked agent's row.
    static func withBoardKey(_ json: String) -> String {
        guard var object = (try? JSONSerialization.jsonObject(with: Data(json.utf8))) as? [String: Any] else { return json }
        var layout = object["layout"] as? [String: Any] ?? [:]
        let rows = layout["rows"] as? [Any] ?? []
        layout["rows"] = [["buttons": [["id": button, "label": "▤ conversation"]]]] + rows
        object["layout"] = layout
        guard let data = try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]) else { return json }
        return String(decoding: data, as: UTF8.self)
    }
}

/// The board itself: a slate in the office's frame, chalk on it, the newest
/// at the foot. Your words on paper, the agent's in chalk, each tool in a
/// dim line, a question in yellow chalk and its answer under it.
struct BoardSheet: View {
    @Bindable var board: Board
    let close: () -> Void
    /// Whether the foot of the board is in sight, so new messages are followed only then.
    @State private var atFoot = true
    @Environment(\.verticalSizeClass) private var height
    /// Away from the front, the slate is wiped: iOS keeps a picture of the app for its switcher.
    @Environment(\.scenePhase) private var phase

    static let slate = Color(hex: 0x1f2b26)
    static let slateEdge = Color(hex: 0x6b4a2b)
    static let chalk = Color(hex: 0xeef0ea)
    static let chalkDim = Color(hex: 0x9fb0a6)
    static let chalkYellow = Color(hex: 0xf2d16b)
    static let chalkBlue = Color(hex: 0x9cc7f0)

    var body: some View {
        VStack(alignment: .leading, spacing: height == .compact ? 6 : 10) {
            HStack(alignment: .firstTextBaseline) {
                Text("▤").font(Pixel.font(15, .bold)).foregroundStyle(Pixel.dim)
                Text(board.showing?.desk.isEmpty == false ? board.showing!.desk : "Conversation")
                    .font(Pixel.font(17, .bold)).foregroundStyle(Pixel.ink).lineLimit(1)
                Spacer(minLength: 8)
                Button("Close", action: close).buttonStyle(PixelKeyStyle(kind: .plain))
            }
            if phase == .active {
                slate
            } else {
                BoardSheet.slate.frame(maxWidth: .infinity, maxHeight: .infinity)
                    .clipShape(PixelBox(step: Pixel.unit * 2))
                    .overlay(PixelBox(step: Pixel.unit * 2).strokeBorder(BoardSheet.slateEdge, lineWidth: Pixel.unit * 2))
            }
        }
    }

    private var slate: some View {
        ScrollViewReader { scroller in
            ScrollView {
                // Not lazy: every row's height known, so the foot is where it's scrolled to. A page is 60 at most.
                VStack(alignment: .leading, spacing: 8) {
                    top(scroller)
                    ForEach(board.items) { item in
                        row(item).id(item.id)
                    }
                    Color.clear.frame(height: 1).id("foot")
                }
                .padding(12)
            }
            .defaultScrollAnchor(.bottom)
            .onScrollGeometryChange(for: Bool.self) { geometry in
                geometry.contentOffset.y + geometry.containerSize.height >= geometry.contentSize.height - 40
            } action: { _, foot in
                atFoot = foot
            }
            .onChange(of: board.arrived) { before, _ in
                guard atFoot else { return }
                // The first page straight to its foot; newer messages after it, gently.
                if before == 0 {
                    DispatchQueue.main.async { scroller.scrollTo("foot", anchor: .bottom) }
                } else {
                    withAnimation(.easeOut(duration: 0.2)) { scroller.scrollTo("foot", anchor: .bottom) }
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .background(BoardSheet.slate)
            .clipShape(PixelBox(step: Pixel.unit * 2))
            .overlay(PixelBox(step: Pixel.unit * 2).strokeBorder(BoardSheet.slateEdge, lineWidth: Pixel.unit * 2))
            .overlay(alignment: .bottom) { problemLine }
        }
    }

    @ViewBuilder private func top(_ scroller: ScrollViewProxy) -> some View {
        if board.loading && board.items.isEmpty {
            HStack { Spacer(); ProgressView().tint(BoardSheet.chalk); Spacer() }.padding(.vertical, 20)
        } else if board.before != nil {
            HStack {
                Spacer()
                Button(board.loadingOlder ? "Loading…" : "Older messages") {
                    // Where you were stays in sight as the older ones come in above it.
                    let first = board.items.first?.id
                    Task {
                        await board.older()
                        // Once they're laid out.
                        try? await Task.sleep(for: .milliseconds(60))
                        if let first { scroller.scrollTo(first, anchor: .top) }
                    }
                }
                    .buttonStyle(PixelKeyStyle(kind: .key))
                    .disabled(board.loadingOlder)
                Spacer()
            }
        } else if !board.items.isEmpty {
            Text("The start of the conversation").font(Pixel.font(12, .medium)).foregroundStyle(BoardSheet.chalkDim)
                .frame(maxWidth: .infinity)
        } else if board.problem == nil {
            Text("Nothing written yet").font(Pixel.font(14, .medium)).foregroundStyle(BoardSheet.chalkDim)
                .frame(maxWidth: .infinity).padding(.vertical, 20)
        }
    }

    @ViewBuilder private var problemLine: some View {
        if let problem = board.problem {
            let text: String = switch problem {
            case .needsTailscale: "Turn on Tailscale to read the conversation"
            case .refused: "Your Mac shows it only with the phone mic on in its setup"
            case .gone(let why): why
            }
            Label(text, systemImage: problem == .needsTailscale ? "network.slash" : "exclamationmark.triangle")
                .font(Pixel.font(13, .semibold)).foregroundStyle(Pixel.ink)
                .padding(.horizontal, 10).padding(.vertical, 6)
                .pixelFrame(Pixel.paper, step: Pixel.unit)
                .padding(8)
        }
    }

    @ViewBuilder private func row(_ item: Board.Item) -> some View {
        switch item.kind {
        case .user:
            HStack {
                Spacer(minLength: 40)
                words(item, colour: Pixel.ink)
                    .padding(.horizontal, 10).padding(.vertical, 7)
                    .pixelFrame(Pixel.paper, step: Pixel.unit)
            }
        case .peer:
            VStack(alignment: .leading, spacing: 2) {
                Text(item.from.isEmpty ? "Another session" : item.from).font(Pixel.font(12, .bold)).foregroundStyle(BoardSheet.chalkBlue)
                words(item, colour: BoardSheet.chalkBlue)
            }
        case .agent:
            words(item, colour: BoardSheet.chalk)
        case .tool:
            line("⚙︎ " + item.text, colour: BoardSheet.chalkDim)
        case .result:
            line("   ↳ " + item.text, colour: BoardSheet.chalkDim.opacity(0.75))
        case .question:
            VStack(alignment: .leading, spacing: 2) {
                Text("Asked you").font(Pixel.font(12, .bold)).foregroundStyle(BoardSheet.chalkYellow.opacity(0.8))
                words(item, colour: BoardSheet.chalkYellow)
            }
        case .answer:
            HStack {
                Spacer(minLength: 40)
                words(item, colour: Pixel.ink, prefix: "→ ")
                    .padding(.horizontal, 10).padding(.vertical, 7)
                    .pixelFrame(BoardSheet.chalkYellow, step: Pixel.unit)
            }
        case .note:
            Text(item.text).font(Pixel.font(12, .medium)).italic().foregroundStyle(BoardSheet.chalkDim)
                .frame(maxWidth: .infinity)
        }
    }

    private func words(_ item: Board.Item, colour: Color, prefix: String = "") -> some View {
        let more = Text(item.more ? " … more" : "").foregroundStyle(colour.opacity(0.55))
        return Text("\(Text(prefix + item.text).foregroundStyle(colour))\(more)")
            .font(Pixel.font(15, .regular))
            .fixedSize(horizontal: false, vertical: true)
            .textSelection(.enabled)
    }

    private func line(_ text: String, colour: Color) -> some View {
        Text(text).font(.system(size: 12, design: .monospaced)).foregroundStyle(colour).lineLimit(1).truncationMode(.middle)
    }
}
