import Foundation
import Observation
import SwiftUI
import UserNotifications

/// "Waiting for you": who waits on an answer, from the Mac's `GET /approvals`
/// (through Tailscale only), and the "Ready for review" cards for the lines a
/// hire marked NEEDS-REVIEW. The list only shows: a wait is answered with the
/// sheet the app already has (☰ choices), never by a new path.
///
/// A Mac before 1.24 has no `/approvals`, so the section hides; the same for
/// the review cards, which wait for the Mac to serve `GET /reviews` (the event
/// log's lines whose `agent` is `needs-review`, as `Watchers.reviewMark` marks them).
@MainActor
@Observable
final class Waiting {
    /// One session asking, as the Mac's list gives it: a short label, never the command.
    struct Wait: Identifiable, Equatable {
        let session: String
        let name: String
        let kind: String
        let label: String
        /// When it began asking, in ms since 1970; nil if the Mac doesn't know.
        let since: Double?
        /// Whether the Mac's own push already told this phone.
        let told: Bool

        var id: String { session + "|" + String(Int64(since ?? 0)) }
        var isQuestion: Bool { kind == "question" }
    }

    /// A hire's NEEDS-REVIEW line, as the event log keeps it.
    struct Review: Identifiable, Equatable {
        let id: Int
        let at: Double
        let session: String
        let hire: String
        let file: String
        let text: String
    }

    /// The waits now; nil while the Mac gives no list (older Mac, or no Tailscale), and the section is hidden.
    private(set) var waits: [Wait]?
    /// The review cards not yet dismissed; nil as above.
    private(set) var reviews: [Review]?
    var isOpen = false

    private let link: DeckLink
    private var lastCount = -1
    private var lastReviews = Date.distantPast
    private var following: Task<Void, Never>?
    private var looking = false

    /// How often the list is asked for again while its panel is up.
    static let every: Duration = .seconds(5)
    static let toldKey = "waitingToldKeys"
    static let seenKey = "reviewsSeen"

    init(link: DeckLink) {
        self.link = link
    }

    var count: Int { (waits?.count ?? 0) + (reviews?.count ?? 0) }
    var visible: Bool { count > 0 }

    // MARK: Following the Mac

    /// A new snapshot: the list asked for again when the number asking changed.
    func follow(_ snapshot: Snapshot?, notify: Bool) {
        guard let snapshot else { return }
        if snapshot.waiting != lastCount {
            lastCount = snapshot.waiting
            Task { await look(notify: notify) }
        }
        if Date().timeIntervalSince(lastReviews) > 30 { Task { await lookReviews() } }
    }

    /// The connection came up: both lists, from scratch.
    func connected(notify: Bool) {
        lastCount = -1
        lastReviews = .distantPast
        Task {
            await look(notify: notify)
            await lookReviews()
        }
    }

    func open() {
        isOpen = true
        following?.cancel()
        following = Task { [weak self] in
            while !Task.isCancelled {
                await self?.look(notify: false)
                await self?.lookReviews()
                try? await Task.sleep(for: Waiting.every)
            }
        }
    }

    func close() {
        isOpen = false
        following?.cancel()
        following = nil
    }

    func look(notify: Bool) async {
        guard !looking else { return }
        looking = true
        defer { looking = false }
        let answer = await link.fetch("/approvals", over: .tailnet, timeout: 6)
        guard let answer, answer.status == 200, let found = Waiting.waits(answer.data) else {
            waits = nil
            return
        }
        waits = found
        if found.isEmpty && isOpen && (reviews ?? []).isEmpty { close() }
        if notify { Waiting.tell(found) }
    }

    func lookReviews() async {
        lastReviews = Date()
        let answer = await link.fetch("/reviews", over: .tailnet, timeout: 6)
        guard let answer, answer.status == 200, let found = Waiting.reviews(answer.data) else {
            reviews = nil
            return
        }
        let seen = Waiting.seen()
        reviews = found.filter { !seen.contains($0.id) }
    }

    /// A card put away: it doesn't come back.
    func dismiss(_ review: Review) {
        var seen = Waiting.seen()
        seen.insert(review.id)
        UserDefaults.standard.set(Array(seen.sorted().suffix(200)), forKey: Waiting.seenKey)
        reviews?.removeAll { $0.id == review.id }
    }

    private static func seen() -> Set<Int> {
        Set(UserDefaults.standard.array(forKey: seenKey) as? [Int] ?? [])
    }

    // MARK: Decoding

    /// The list, or nil when the answer has no `approvals` (a Mac before 1.24).
    nonisolated static func waits(_ data: Data) -> [Wait]? {
        guard let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let items = object["approvals"] as? [[String: Any]] else { return nil }
        return items.compactMap { item in
            guard let session = item["session"] as? String, !session.isEmpty else { return nil }
            let nickname = item["nickname"] as? String ?? ""
            let name = nickname.isEmpty ? item["name"] as? String ?? "" : nickname
            return Wait(session: session, name: name.isEmpty ? "An agent" : name, kind: item["kind"] as? String ?? "permission",
                        label: item["label"] as? String ?? item["tool"] as? String ?? "",
                        since: (item["since"] as? NSNumber)?.doubleValue, told: item["notified"] as? Bool ?? false)
        }
    }

    /// The review lines, or nil when the answer has no `reviews`. Each is an event-log line,
    /// "<hire>: <file>: <line>" in its detail, and only those the watch marked `needs-review`.
    nonisolated static func reviews(_ data: Data) -> [Review]? {
        guard let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let items = object["reviews"] as? [[String: Any]] else { return nil }
        return items.compactMap { item in
            guard (item["agent"] as? String ?? "needs-review") == "needs-review", let id = (item["id"] as? NSNumber)?.intValue else { return nil }
            let parts = (item["detail"] as? String ?? "").split(separator: ":", maxSplits: 2, omittingEmptySubsequences: false)
                .map { $0.trimmingCharacters(in: .whitespaces) }
            let hire = parts.count == 3 ? parts[0] : ""
            let file = parts.count == 3 ? parts[1] : ""
            let text = parts.last ?? ""
            return Review(id: id, at: (item["at"] as? NSNumber)?.doubleValue ?? 0, session: item["session"] as? String ?? "",
                          hire: hire.isEmpty ? "A hire" : hire, file: file, text: text)
        }
    }

    // MARK: One notification for each wait

    /// What of `waits` still needs a note: not told by the Mac's push, not noted before.
    nonisolated static func fresh(_ waits: [Wait], noted: Set<String>) -> [Wait] {
        waits.filter { !$0.told && !noted.contains($0.id) }
    }

    /// The lock screen's words: who waits, never what for.
    nonisolated static func body(_ wait: Wait) -> String { "\(wait.name) is waiting for you" }

    /// A local note for each wait not noted yet, once; the ones no longer waiting are forgotten,
    /// so a later wait of the same desk is a new one. It follows the "waiting" switch in settings.
    static func tell(_ waits: [Wait]) {
        let defaults = UserDefaults.standard
        let noted = Set(defaults.stringArray(forKey: toldKey) ?? [])
        let still = Set(waits.map(\.id))
        // The Mac's push told this phone of these already: noted, not shown twice.
        let next = noted.intersection(still).union(waits.filter(\.told).map(\.id))
        let new = fresh(waits, noted: noted)
        defaults.set(Array(next.union(new.map(\.id))), forKey: toldKey)
        // The ids are pruned above whether or not the switch is on; it only gates the note.
        guard !new.isEmpty, defaults.object(forKey: "notifyWaiting") as? Bool ?? true else { return }
        Task {
            let center = UNUserNotificationCenter.current()
            let status = await center.notificationSettings().authorizationStatus
            guard status == .authorized || status == .provisional || status == .ephemeral else { return }
            for wait in new {
                let content = UNMutableNotificationContent()
                content.title = "Smolcorp"
                content.body = body(wait)
                content.sound = .default
                content.threadIdentifier = "waiting"
                content.userInfo = ["session": wait.session]
                try? await center.add(UNNotificationRequest(identifier: "wait-" + wait.id, content: content, trigger: nil))
            }
        }
    }

    /// How long ago, in words.
    static func ago(_ since: Double?, now: Date = Date()) -> String {
        guard let since, since > 0 else { return "" }
        let seconds = max(0, now.timeIntervalSince1970 - since / 1000)
        return seconds < 45 ? "just now" : Info.duration(seconds * 1000) + " ago"
    }
}

/// The panel: the waits, each with a key to answer it in the app's own choices sheet, then the review cards.
struct WaitingSheet: View {
    @Bindable var waiting: Waiting
    let answer: (String) -> Void
    let openDesk: (String) -> Void
    let close: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline) {
                Text("Waiting for you").font(Pixel.font(17, .bold)).foregroundStyle(Pixel.ink)
                Spacer(minLength: 8)
                Button("Close", action: close).buttonStyle(PixelKeyStyle(kind: .plain))
            }
            ScrollView {
                VStack(alignment: .leading, spacing: 10) {
                    ForEach(waiting.waits ?? []) { wait in waitRow(wait) }
                    if let reviews = waiting.reviews, !reviews.isEmpty {
                        Text("Ready for review").font(Pixel.font(14, .bold)).foregroundStyle(Pixel.dim).padding(.top, 4)
                        ForEach(reviews) { review in reviewCard(review) }
                    }
                }
            }
            .scrollBounceBehavior(.basedOnSize)
        }
    }

    private func waitRow(_ wait: Waiting.Wait) -> some View {
        HStack(alignment: .center, spacing: 10) {
            VStack(alignment: .leading, spacing: 2) {
                Text(wait.name).font(Pixel.font(16, .bold)).foregroundStyle(Pixel.ink).lineLimit(1)
                Text([wait.isQuestion ? "Question" : wait.label, Waiting.ago(wait.since)].filter { !$0.isEmpty }.joined(separator: " · "))
                    .font(Pixel.font(13, .medium)).foregroundStyle(Pixel.dim).lineLimit(2)
            }
            Spacer(minLength: 0)
            Button(wait.isQuestion ? "Answer" : "Open") { answer(wait.session) }
                .buttonStyle(PixelKeyStyle(kind: .send))
        }
    }

    private func reviewCard(_ review: Waiting.Review) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(review.hire).font(Pixel.font(16, .bold)).foregroundStyle(Pixel.ink)
            if !review.file.isEmpty { Text(review.file).font(Pixel.font(13, .medium)).foregroundStyle(Pixel.dim) }
            if !review.text.isEmpty {
                Text(review.text).font(Pixel.font(14, .medium)).foregroundStyle(Pixel.ink).fixedSize(horizontal: false, vertical: true)
            }
            HStack {
                if !review.session.isEmpty { Button("Open desk") { openDesk(review.session) }.buttonStyle(PixelKeyStyle(kind: .key)) }
                Button("Done") { waiting.dismiss(review) }.buttonStyle(PixelKeyStyle(kind: .plain))
            }
        }
        .padding(10)
        .pixelFrame(Color.white, step: Pixel.unit)
    }
}
