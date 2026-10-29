import Foundation

/// Phone microphone (personal build): the phone listens with its own speech
/// recognition (the browser's Web Speech API) and sends only the words it
/// heard to `POST /dictate`. No audio ever reaches the Mac, and nothing is
/// kept: the text is typed into the agent's tab and forgotten.
///
/// iOS only lets a page use the microphone over HTTPS, so this works when the
/// deck is opened through `tailscale serve`; over plain http on the Wi-Fi,
/// the mic is the Mac's, as before.
enum PhoneDictation {
    /// The most text one dictation may carry, in characters: well beyond a
    /// minute of speech, short enough that a stray request can't type a book.
    static let longest = 2000
    /// Claude Code's own /voice sends what it heard once it's this many words.
    static let wordsToSend = 3
    /// How long the agent is given to take the text in before Return goes,
    /// on its own. Sent with the text, in the same read, Claude Code takes
    /// the lot for a paste, and a Return in a paste is a new line in it.
    static let settle: TimeInterval = 0.25
    /// The peek at the input line: this many of the tab's last lines, each
    /// cut to this many characters. Enough for Claude Code's input box and
    /// what's under it; never its scrollback.
    static let peekLines = 10
    static let peekWidth = 240
    /// At most one peek a session this often: one look after each dictation, never a live view.
    static let peekGap: TimeInterval = 1

    /// Whether a peek at a session comes too soon after its last, `last`.
    static func peekTooSoon(last: Date?, now: Date) -> Bool {
        last.map { now.timeIntervalSince($0) < peekGap } ?? false
    }

    /// The text as it will be typed: line breaks (Unicode's own line and
    /// paragraph separators too) and other control characters become spaces
    /// (a line break would press Return part way through), runs of spaces
    /// one, trimmed. Nil if nothing's left or it's too long.
    static func prepared(_ text: String) -> String? {
        let breaks = CharacterSet.controlCharacters.union(.newlines)
        let flat = String(text.unicodeScalars.map { breaks.contains($0) ? " " : Character($0) })
        let single = flat.split(separator: " ", omittingEmptySubsequences: true).joined(separator: " ")
        guard !single.isEmpty, single.count <= longest else { return nil }
        return single
    }

    /// Whether the text goes with Return: with the setting on, and once it's
    /// a few words, as Claude Code does. Shorter, it waits for you to send.
    static func sends(_ text: String, autoSend: Bool) -> Bool {
        autoSend && text.split(whereSeparator: \.isWhitespace).count >= wordsToSend
    }

    /// The end of what a tab shows, as the phone's peek gives it: the blank
    /// lines under it dropped, its last `peekLines`, each cut to `peekWidth`,
    /// with escape and other control characters gone, so nothing is drawn
    /// but text, and no blank lines over it.
    static func peek(_ screen: String) -> [String] {
        let blank = { (line: String) in line.trimmingCharacters(in: .whitespaces).isEmpty }
        var lines = screen.components(separatedBy: .newlines).map { line in
            String(String(line.unicodeScalars.filter { !CharacterSet.controlCharacters.contains($0) }).prefix(peekWidth))
        }
        while let last = lines.last, blank(last) { lines.removeLast() }
        return Array(lines.suffix(peekLines).drop(while: blank))
    }
}

/// Personal build: this Mac's address in its tailnet, if Tailscale's command
/// line is here, for setup to show, and whether `tailscale serve` hands the
/// deck on, to warn while the phone mic is off. Only ever reads (`tailscale
/// status --json`, `tailscale serve status --json`), changes nothing, and is
/// never needed: without it setup just shows the steps. Looked up when setup
/// asks, at most once a minute.
enum Tailnet {
    /// Where Tailscale's command line can be: the Mac app's own, then Homebrew's and the open-source one's.
    static let candidates = [
        "/Applications/Tailscale.app/Contents/MacOS/Tailscale",
        "/opt/homebrew/bin/tailscale",
        "/usr/local/bin/tailscale",
    ]
    static let lookAgainAfter: TimeInterval = 60
    static let timeout: TimeInterval = 3

    /// The last answer: the HTTPS address, or nil if none was found.
    private(set) static var url: String?
    /// Whether `tailscale serve` hands this deck on; nil if it can't be told.
    private(set) static var serving: Bool?
    private static var lookedAt: Date?
    private static var looking = false
    /// Who waits for the look under way to end.
    private static var waiting: [() -> Void] = []

    /// How Tailscale is asked: its address and whether it serves the deck,
    /// or nil if it isn't installed. Off the main queue. A test answers here.
    static var ask: () -> (url: String?, serving: Bool?)? = {
        guard let tool = candidates.first(where: { FileManager.default.isExecutableFile(atPath: $0) }) else { return nil }
        return (run(tool, ["status", "--json"]).flatMap(url(fromStatus:)),
                run(tool, ["serve", "status", "--json"]).flatMap(servesDeck(fromServeStatus:)))
    }

    /// Whether the last answer is too old to go by, or there's none yet.
    static var isStale: Bool {
        lookedAt.map { Date().timeIntervalSince($0) >= lookAgainAfter } ?? true
    }

    /// The address `tailscale serve` gives this Mac, from `tailscale status
    /// --json`: `https://` and its MagicDNS name. Nil if it isn't there.
    static func url(fromStatus data: Data) -> String? {
        guard let status = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let me = status["Self"] as? [String: Any], let name = me["DNSName"] as? String else { return nil }
        let host = name.hasSuffix(".") ? String(name.dropLast()) : name
        guard Addressing.isTailnet(host.lowercased(), fromLoopback: true) else { return nil }
        return "https://\(host.lowercased())"
    }

    /// Whether `tailscale serve status --json` hands this deck on: a proxy,
    /// or a TCP forward, to port 7710 on this Mac. Nil if it isn't that JSON.
    static func servesDeck(fromServeStatus data: Data) -> Bool? {
        guard let status = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return nil }
        func points(_ target: String) -> Bool {
            let address = target.lowercased().replacingOccurrences(of: #"^[a-z+]+://"#, with: "", options: .regularExpression)
                .split(separator: "/", maxSplits: 1).first.map(String.init) ?? ""
            return ["127.0.0.1:\(Deck.port)", "localhost:\(Deck.port)", "[::1]:\(Deck.port)"].contains(address)
                || address == "\(Deck.port)"
        }
        func walk(_ value: Any) -> Bool {
            if let object = value as? [String: Any] {
                return object.contains { key, inner in
                    (["Proxy", "TCPForward"].contains(key) && (inner as? String).map(points) == true) || walk(inner)
                }
            }
            if let list = value as? [Any] { return list.contains(where: walk) }
            return false
        }
        return walk(status)
    }

    /// Looks again in the background if it's been a while, then calls
    /// `changed` on the main queue if the answer is different, and `then`
    /// once the answer is as fresh as it gets. On the main queue.
    static func refresh(changed: @escaping () -> Void, then: (() -> Void)? = nil) {
        if let then { waiting.append(then) }
        if looking { return }
        guard isStale else { return finish() }
        looking = true
        let ask = self.ask
        DispatchQueue.global(qos: .utility).async {
            let answer = ask()
            DispatchQueue.main.async {
                looking = false
                lookedAt = Date()
                if let answer, answer.url != url || answer.serving != serving {
                    url = answer.url
                    serving = answer.serving
                    changed()
                }
                finish()
            }
        }
    }

    /// Calls `then` on the main queue once the answer is fresh, or after
    /// `wait` with the one there is, whichever comes first: the setup check
    /// is right on its first run, and never waits long.
    static func whenFresh(within wait: TimeInterval = timeout, changed: @escaping () -> Void, _ then: @escaping () -> Void) {
        var answered = false
        let once = {
            guard !answered else { return }
            answered = true
            then()
        }
        refresh(changed: changed, then: once)
        DispatchQueue.main.asyncAfter(deadline: .now() + wait, execute: once)
    }

    private static func finish() {
        let done = waiting
        waiting = []
        done.forEach { $0() }
    }

    /// Back to the start, with nothing known: for tests.
    static func forget() {
        url = nil
        serving = nil
        lookedAt = nil
    }

    /// The environment Tailscale's command line runs in: this app's, with a
    /// TERM. The Mac app's binary is its command line only when TERM is set;
    /// without it, as from an app opened in Finder, it tries to start its
    /// window instead, fails, and prints no status.
    static func environment(_ inherited: [String: String] = ProcessInfo.processInfo.environment) -> [String: String] {
        var environment = inherited
        if environment["TERM", default: ""].isEmpty { environment["TERM"] = "dumb" }
        return environment
    }

    /// Runs the tool, read-only, and gives what it printed, or nil if it
    /// failed or took too long.
    private static func run(_ tool: String, _ arguments: [String]) -> Data? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: tool)
        process.arguments = arguments
        process.environment = environment()
        let out = Pipe()
        process.standardOutput = out
        process.standardError = FileHandle.nullDevice
        do { try process.run() } catch { return nil }
        let reading = DispatchQueue(label: "tailnet.read")
        var data = Data()
        let read = DispatchWorkItem { data = out.fileHandleForReading.readDataToEndOfFile() }
        reading.async(execute: read)
        if read.wait(timeout: .now() + timeout) == .timedOut {
            process.terminate()
            return nil
        }
        process.waitUntilExit()
        return process.terminationStatus == 0 ? data : nil
    }
}
