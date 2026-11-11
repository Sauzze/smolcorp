import Foundation

/// Each terminal app's own queue: a desk's keys and text wait only on its own
/// terminal. A tmux desk never waits behind Terminal.app, which can stop
/// answering for a minute at a time (as while macOS asks whether Smolcorp may
/// control it); within one terminal they still go one at a time, in order.
extension Terminals {
    private static let lock = NSLock()
    private static var queues: [String: DispatchQueue] = [:]

    /// The queue `tab`'s terminal is driven on: `Terminal.queue` for
    /// Terminal.app's tabs, and for no tab at all (a window opened).
    static func queue(for tab: TabAddress?) -> DispatchQueue { queue(kind: tab?.kind ?? "tty") }

    static func queue(for tab: String) -> DispatchQueue { queue(for: TabAddress(tab)) }

    /// The queue a window of `terminal` (an `Openers` choice) opens on: that
    /// terminal's own, so an iTerm2 or Ghostty window slow to open holds up
    /// only that terminal's desks, never Terminal.app's.
    static func queue(opening terminal: String) -> DispatchQueue {
        queue(kind: ["iterm": "iterm", "ghostty": "ghostty"][terminal] ?? "tty")
    }

    /// The queue of the terminal whose tabs are addressed as `kind:` (`TabAddress.kind`).
    static func queue(kind: String) -> DispatchQueue {
        guard kind != "tty" else { return Terminal.queue }
        lock.lock()
        defer { lock.unlock() }
        if let queue = queues[kind] { return queue }
        let queue = DispatchQueue(label: "io.github.sauzze.agentdeck.terminal.\(kind)")
        queues[kind] = queue
        return queue
    }

    /// How long a desk's keys or text may wait for their terminal before the
    /// phone is told it isn't answering. Past it, they never go: typed late,
    /// they could land in the middle of what you send next.
    static let inputWait: TimeInterval = 5

    /// What the phone is told when `tab`'s terminal doesn't get to its keys in
    /// time, or, `partWay`, stops answering once some have gone.
    static func notAnswering(_ tab: String, partWay: Bool = false) -> String {
        let names = ["tty": "Terminal", "tmux": "tmux", "iterm": "iTerm2", "ghostty": "Ghostty", "wezterm": "WezTerm", "kitty": "kitty"]
        let name = TabAddress(tab).flatMap { names[$0.kind] } ?? "The terminal"
        return partWay ? "\(name) stopped answering part way. Look at the desk before you try again."
            : "\(name) isn't answering, so nothing was typed. Try again in a moment."
    }
}

/// A look at a terminal, taken on a queue of its own, off the way of the
/// desks' keys and text, one round at a time: a round started while the last
/// is still running, as with Terminal not answering, is skipped rather than
/// piled up behind it.
final class Poller {
    private let queue: DispatchQueue
    private let lock = NSLock()
    private var running = false

    init(_ name: String) {
        queue = DispatchQueue(label: "io.github.sauzze.agentdeck.poll.\(name)")
    }

    /// Runs `look` on the poller's queue, then `then` with its answer on the
    /// main queue. False, and nothing run, while the last round still runs.
    @discardableResult
    func round<T>(_ look: @escaping () -> T, then: @escaping (T) -> Void) -> Bool {
        lock.lock()
        defer { lock.unlock() }
        guard !running else { return false }
        running = true
        queue.async {
            let answer = look()
            self.lock.lock()
            self.running = false
            self.lock.unlock()
            DispatchQueue.main.async { then(answer) }
        }
        return true
    }
}

/// One input on its way to a desk's terminal, answered once. It's typed only
/// if its terminal gets to it within its wait: checked right before the first
/// key. Past that the caller is told the terminal isn't answering, and the
/// input is dropped, never typed later, where it could merge with the next.
/// Once its first key has gone, it's seen through and answered as it went.
final class Delivery {
    private enum Step { case waiting, typing, givenUp }
    private let lock = NSLock()
    private var step = Step.waiting
    private var answered = false
    private let deadline: Date
    private let late: Response
    private let done: (Response) -> Void

    /// `done` is called once, on the main queue; with `late` if the terminal
    /// hasn't got to it within `wait`.
    init(within wait: TimeInterval, late: Response, done: @escaping (Response) -> Void) {
        deadline = Date().addingTimeInterval(wait)
        self.late = late
        self.done = done
        DispatchQueue.main.asyncAfter(deadline: .now() + wait) { [self] in
            lock.lock()
            if step == .waiting { step = .givenUp }
            let gaveUp = step == .givenUp
            lock.unlock()
            if gaveUp { answer(late) }
        }
    }

    /// On the terminal's queue, right before the first key: whether to type
    /// at all. Not once given up; and given up now if the wait is over.
    func begin() -> Bool {
        lock.lock()
        if step == .waiting, Date() < deadline { step = .typing }
        if step == .waiting { step = .givenUp }
        let typing = step == .typing
        lock.unlock()
        if !typing { answer(late) }
        return typing
    }

    /// The caller's answer, from any queue: only the first counts.
    func answer(_ response: Response) {
        lock.lock()
        let first = !answered
        answered = true
        lock.unlock()
        guard first else { return }
        if Thread.isMainThread { done(response) } else { DispatchQueue.main.async { self.done(response) } }
    }
}
