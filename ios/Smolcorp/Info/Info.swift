import Foundation
import Observation
import SwiftUI

/// ⓘ info: what the picked agent runs on and what it has used, from the Mac's
/// `GET /session/info`, through Tailscale only, as the blackboard is: who
/// pays, the model actually running, the session's time, its tokens and what
/// they cost at the API's prices, and a subscription's 5-hour and weekly use
/// where Claude Code has fetched it. Only what the Mac reported: anything it
/// has no source for isn't on the card. While it's up, again every few seconds.
@MainActor
@Observable
final class Info {
    struct Facts: Equatable {
        struct Window: Equatable {
            var percent: Double
            var resetsInMs: Double?
        }

        /// "Claude Max 20x · me@example.com", "API key"…
        var account: String?
        /// "Opus 5.5 · high effort"
        var model: String?
        var sessionMs: Double?
        var activeMs: Double?
        var deskMs: Double?
        var tokensIn: Int?
        var tokensOut: Int?
        var cacheRead: Int?
        var cacheWrite: Int?
        var cost: Double?
        /// The bill, with an API key; otherwise what the tokens would have cost at the API's prices.
        var costIsBill = false
        var fiveHour: Window?
        var weekly: Window?
        var usageAgeMs: Double?

        init?(_ data: Data) {
            guard let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return nil }
            let number = { (object: [String: Any]?, key: String) in (object?[key] as? NSNumber)?.doubleValue }
            if let account = object["account"] as? [String: Any] {
                switch account["kind"] as? String {
                case "api-key": self.account = "API key"
                case "cloud": self.account = "Cloud provider"
                default:
                    let plan = (account["plan"] as? String).map { "Claude " + $0 } ?? "Subscription"
                    self.account = [plan, account["name"] as? String].compactMap { $0 }.joined(separator: " · ")
                }
            }
            if let model = object["model"] as? [String: Any], let name = model["name"] as? String {
                self.model = [name, (model["effort"] as? String).map { "\($0) effort" }, model["fast"] as? Bool == true ? "fast" : nil]
                    .compactMap { $0 }.joined(separator: " · ")
            }
            let time = object["time"] as? [String: Any]
            sessionMs = number(time, "sessionMs")
            activeMs = number(time, "activeMs")
            deskMs = number(time, "deskMs")
            if let tokens = object["tokens"] as? [String: Any] {
                let count = { (key: String) in (tokens[key] as? NSNumber)?.intValue ?? 0 }
                cacheRead = count("cacheRead")
                cacheWrite = count("cacheWrite")
                tokensIn = count("input") + count("cacheRead") + count("cacheWrite")
                tokensOut = count("output")
            }
            if let cost = object["cost"] as? [String: Any] {
                self.cost = number(cost, "usd")
                costIsBill = cost["basis"] as? String == "api"
            }
            if let limits = object["limits"] as? [String: Any] {
                func window(_ key: String) -> Window? {
                    guard let window = limits[key] as? [String: Any], let percent = number(window, "percent") else { return nil }
                    return Window(percent: percent, resetsInMs: number(window, "resetsInMs"))
                }
                fiveHour = window("fiveHour")
                weekly = window("weekly")
                usageAgeMs = number(limits, "ageMs")
            }
        }
    }

    struct Showing: Identifiable {
        let session: String
        let desk: String
        var id: String { session }
    }

    /// The card, up over this desk.
    private(set) var showing: Showing?
    private(set) var facts: Facts?
    private(set) var problem: Board.Problem?

    private var following: Task<Void, Never>?
    private let link: DeckLink

    /// The key, as a button of the layout's, right after ▤ conversation; the Mac never sees it pressed.
    static let button = "smolcorp-info"
    /// How often the card is asked for again, while it's up.
    static let every: Duration = .seconds(5)

    init(link: DeckLink) {
        self.link = link
    }

    /// The key tapped over `session`'s desk.
    func open(_ session: String) -> Bool {
        guard let desk = link.snapshot?.desks.first(where: { $0.id == session }) else { return false }
        if showing?.session != session { facts = nil }
        showing = Showing(session: session, desk: desk.project)
        problem = nil
        following?.cancel()
        following = Task { [weak self] in
            while !Task.isCancelled {
                await self?.look()
                try? await Task.sleep(for: Info.every)
            }
        }
        return true
    }

    func close() {
        following?.cancel()
        following = nil
        showing = nil
    }

    private func look() async {
        guard let session = showing?.session else { return }
        let answer = await link.fetch("/session/info?id=" + (session.addingPercentEncoding(withAllowedCharacters: .alphanumerics) ?? ""),
                                      over: .tailnet)
        guard showing?.session == session else { return }
        guard let answer else { return problem = .needsTailscale }
        switch answer.status {
        case 200:
            problem = nil
            if let facts = Facts(answer.data) { self.facts = facts }
        case 403: problem = .refused
        default:
            let text = String(decoding: answer.data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
            problem = .gone(text.isEmpty || text.count > 200 ? "The Mac answered \(answer.status)." : text)
        }
    }

    // MARK: How it reads

    static func duration(_ ms: Double) -> String {
        let s = max(0, Int((ms / 1000).rounded()))
        if s < 60 { return "\(s) s" }
        let m = s / 60
        if m < 60 { return "\(m) min" }
        let h = m / 60
        if h < 24 { return String(format: "%d h %02d", h, m % 60) }
        return "\(h / 24) d \(h % 24) h"
    }

    static func count(_ n: Int) -> String {
        if n < 1000 { return "\(n)" }
        if n < 1_000_000 { return String(format: n < 10_000 ? "%.1fk" : "%.0fk", Double(n) / 1000) }
        return String(format: n < 10_000_000 ? "%.2fM" : "%.1fM", Double(n) / 1_000_000)
    }

    static func money(_ usd: Double) -> String {
        usd < 0.01 ? "under $0.01" : String(format: "$%.2f", usd)
    }

    /// The snapshot with the key in the layout's buttons, first; the blackboard's goes in front of it.
    static func withInfoKey(_ json: String) -> String {
        guard var object = (try? JSONSerialization.jsonObject(with: Data(json.utf8))) as? [String: Any] else { return json }
        var layout = object["layout"] as? [String: Any] ?? [:]
        let rows = layout["rows"] as? [Any] ?? []
        layout["rows"] = [["buttons": [["id": button, "label": "ⓘ info"]]]] + rows
        object["layout"] = layout
        guard let data = try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]) else { return json }
        return String(decoding: data, as: UTF8.self)
    }
}

/// The card: a line for each fact the Mac reported, in the office's paper and ink.
struct InfoSheet: View {
    @Bindable var info: Info
    let close: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline) {
                Text("ⓘ").font(Pixel.font(15, .bold)).foregroundStyle(Pixel.dim)
                Text(info.showing?.desk.isEmpty == false ? info.showing!.desk : "Info")
                    .font(Pixel.font(17, .bold)).foregroundStyle(Pixel.ink).lineLimit(1)
                Spacer(minLength: 8)
                Button("Close", action: close).buttonStyle(PixelKeyStyle(kind: .plain))
            }
            if let facts = info.facts {
                rows(facts)
            } else if info.problem == nil {
                HStack { Spacer(); ProgressView(); Spacer() }.padding(.vertical, 12)
            }
            if let problem = info.problem {
                let text: String = switch problem {
                case .needsTailscale: "Turn on Tailscale to see the info"
                case .refused: "Your Mac shows it only with the phone mic on in its setup"
                case .gone(let why): why
                }
                Label(text, systemImage: problem == .needsTailscale ? "network.slash" : "exclamationmark.triangle")
                    .font(Pixel.font(13, .semibold)).foregroundStyle(Pixel.warn)
            }
        }
    }

    private func rows(_ facts: Info.Facts) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Grid(alignment: .leadingFirstTextBaseline, horizontalSpacing: 14, verticalSpacing: 6) {
                if let account = facts.account { row("Account", account) }
                if let model = facts.model { row("Model", model) }
                if let ms = facts.sessionMs { row("Session", Info.duration(ms)) }
                if let ms = facts.activeMs { row("Working", Info.duration(ms)) }
                if let ms = facts.deskMs { row("On this desk", Info.duration(ms)) }
                if let tokensIn = facts.tokensIn, let tokensOut = facts.tokensOut {
                    row("Tokens", "\(Info.count(tokensIn)) in · \(Info.count(tokensOut)) out")
                }
                if let read = facts.cacheRead, let write = facts.cacheWrite, read + write > 0 {
                    row("Cache", "\(Info.count(read)) read · \(Info.count(write)) written")
                }
                if let cost = facts.cost { row(facts.costIsBill ? "Cost" : "At API prices", "\(Info.money(cost)) (estimate)") }
                if let window = facts.fiveHour { meterRow("5-hour limit", window) }
                if let window = facts.weekly { meterRow("Weekly limit", window) }
            }
            if let age = facts.usageAgeMs, facts.fiveHour != nil || facts.weekly != nil {
                Text("Use as Claude Code last fetched it, \(age < 60_000 ? "just now" : Info.duration(age) + " ago").")
                    .font(Pixel.font(12, .medium)).foregroundStyle(Pixel.dim)
            }
        }
    }

    private func row(_ name: String, _ value: String) -> some View {
        GridRow {
            Text(name).font(Pixel.font(14, .medium)).foregroundStyle(Pixel.dim)
            Text(value).font(Pixel.font(15, .semibold)).foregroundStyle(Pixel.ink)
                .fixedSize(horizontal: false, vertical: true).textSelection(.enabled)
        }
    }

    private func meterRow(_ name: String, _ window: Info.Facts.Window) -> some View {
        GridRow {
            Text(name).font(Pixel.font(14, .medium)).foregroundStyle(Pixel.dim)
            HStack(spacing: 8) {
                GeometryReader { space in
                    ZStack(alignment: .leading) {
                        Rectangle().fill(Color.white)
                        Rectangle().fill(window.percent >= 80 ? Color(hex: 0xe0483e) : Pixel.send)
                            .frame(width: space.size.width * min(1, max(0, window.percent / 100)))
                    }
                }
                .frame(width: 64, height: 8)
                .overlay(Rectangle().strokeBorder(Pixel.ink, lineWidth: 1.5))
                Text("\(Int(window.percent.rounded()))%" + (window.resetsInMs.map { " · resets in " + Info.duration($0) } ?? ""))
                    .font(Pixel.font(15, .semibold)).foregroundStyle(Pixel.ink)
            }
        }
    }
}
