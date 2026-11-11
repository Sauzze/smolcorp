import Foundation

/// Where a session's agent runs: a terminal's tab, as `kind:id`, or
/// `kind:id@socket` for a terminal reached through a socket, the form the
/// Windows and Linux app gives its tabs (desktop/tabs.go):
///
///   tty:/dev/ttys003                    a Terminal.app tab, by its tty
///   tmux:%3@/private/tmp/tmux-501/default
///   iterm:BD2206F6-EE01-476F-A6D9-3061679BD8B0
///   ghostty:/dev/ttys004                Ghostty, by tty until it names its tabs
///   other:/dev/ttys005                  a terminal Smolcorp can't drive: shown, not controlled
///   headless:3f2a9c1e-…                 a headless claude run, by its session ID: no terminal at all
///
/// A headless run is named so that it can't be mistaken for a tab: it has no
/// driver, nothing is watched or pressed there, and it is never matched to a
/// terminal's tab.
/// A bare tty, as a deck saved before 1.9 kept, reads as a Terminal.app tab.
extension TerminalDriver {
    func typeKeys(_ text: String, in tab: TabAddress) -> Bool { insert(text, in: tab) }
    var showsScreen: Bool { true }
    /// Nowhere by default: WezTerm and kitty send text as it is and can't say whether the agent asked for
    /// bracketed paste, so a line break could press Return part way. Refused before anything waits on them.
    var linesWay: LinesWay { .never }
    func insertLines(_ text: String, in tab: TabAddress, ready: () -> String?) -> LinesPasted { .notTaken }
}

/// How a terminal takes text of several lines from the phone, if at all.
enum LinesWay: Equatable {
    /// It can't: refused before anything waits on its queue.
    case never
    /// tmux: it reads whether the pane's agent asked for bracketed paste, and pastes only then.
    case paneFlag
    /// Through the clipboard, as you'd paste it: the terminal wraps it in bracketed-paste markers itself
    /// where the agent asked for them, which it can't say, so only for an agent known to ask
    /// (`Agent.asksForBracketedPaste`). `inFront`: the tab is brought forward and ⌘V pressed, which needs
    /// Accessibility and the Mac unlocked.
    case clipboard(inFront: Bool)
}

/// How text of several lines went. Only `.pasted` typed anything, and only it is followed by Return;
/// `.notRead` and `.failed` may have typed it, or may yet, but never with a Return of ours.
enum LinesPasted: Equatable {
    /// As one paste.
    case pasted
    /// Not at all: the tab doesn't take one now (tmux: the agent didn't ask for bracketed paste).
    case notTaken
    /// Not at all: tmux's pane is in copy mode, where nothing is pasted.
    case inMode
    /// Not at all: `ready` said why, right before the paste (asking, working, an editor in the tab…).
    case refused(String)
    /// Not at all: the tab couldn't be brought to the front, or another tab or app was in front by ⌘V.
    case notInFront
    /// Not at all: Terminal.app or iTerm2 would have to come to the front while the Mac is in use.
    case macInUse
    /// Not at all: what's on the clipboard couldn't be put back exactly afterwards (a file being copied).
    case unsaved
    /// ⌘V went, but the terminal hadn't read the clipboard in time: it may still paste.
    case notRead
    /// The terminal didn't take it, or ended for taking too long.
    case failed
}

struct TabAddress: Hashable {
    let kind: String
    let id: String
    let socket: String

    init(kind: String, id: String, socket: String = "") {
        self.kind = kind
        self.id = id
        self.socket = socket
    }

    init?(_ text: String) {
        // Nothing that could end a header line or a script's argument.
        guard !text.isEmpty, text.unicodeScalars.allSatisfy({ !CharacterSet.controlCharacters.contains($0) }) else { return nil }
        if text.hasPrefix("/dev/") {
            guard TabAddress.isTTY(text) else { return nil }
            self.init(kind: "tty", id: text)
            return
        }
        guard let colon = text.firstIndex(of: ":") else { return nil }
        let kind = String(text[..<colon]), rest = text[text.index(after: colon)...]
        guard !kind.isEmpty, !rest.isEmpty else { return nil }
        // A tty has no socket, and Ghostty's and other's ids are ttys too:
        // only in a tty's own shape, as /dev/ttys003.
        if ["tty", "ghostty", "other"].contains(kind) {
            guard TabAddress.isTTY(String(rest)) else { return nil }
            self.init(kind: kind, id: String(rest))
        } else if kind == "headless" {
            guard rest.allSatisfy({ $0.isASCII && ($0.isLetter || $0.isNumber || $0 == "-") }) else { return nil }
            self.init(kind: kind, id: String(rest))
        } else if kind == "iterm" {
            guard rest.allSatisfy({ $0.isASCII && ($0.isHexDigit || $0 == "-") }) else { return nil }
            self.init(kind: kind, id: String(rest))
        } else {
            let at = rest.firstIndex(of: "@")
            let id = String(rest[..<(at ?? rest.endIndex)]), socket = at.map { String(rest[rest.index(after: $0)...]) } ?? ""
            // A pane's id in its terminal's own shape: %3 in tmux, a number in WezTerm and kitty.
            let digits = (kind == "tmux" ? id.dropFirst() : id[...])
            guard !digits.isEmpty, digits.allSatisfy(\.isASCII), digits.allSatisfy(\.isNumber),
                  kind != "tmux" || id.hasPrefix("%") else { return nil }
            // tmux's, WezTerm's and kitty's panes are only known with their server's socket.
            guard !["tmux", "wezterm", "kitty"].contains(kind) || !socket.isEmpty else { return nil }
            self.init(kind: kind, id: id, socket: socket)
        }
    }

    /// Whether this is a headless run rather than a terminal's tab.
    var isHeadless: Bool { kind == "headless" }

    /// Whether `tab` names a headless run.
    static func isHeadless(_ tab: String) -> Bool { TabAddress(tab)?.isHeadless ?? false }

    var text: String { socket.isEmpty ? "\(kind):\(id)" : "\(kind):\(id)@\(socket)" }

    /// A terminal device's path, as /dev/ttys003, and nothing else.
    static func isTTY(_ path: String) -> Bool {
        guard path.hasPrefix("/dev/tty") else { return false }
        let name = path.dropFirst("/dev/tty".count)
        return !name.isEmpty && name.allSatisfy { $0.isASCII && ($0.isLetter || $0.isNumber) }
    }

    static func terminalApp(tty: String) -> TabAddress { TabAddress(kind: "tty", id: tty) }
}

/// What a terminal lets the deck do with one of its tabs.
struct TerminalAbilities: OptionSet {
    let rawValue: Int
    /// Press keys: Return, Escape, arrows, Space for the mic.
    static let keys = TerminalAbilities(rawValue: 1 << 0)
    /// Type a line: a made button's text, /clear, /effort.
    static let text = TerminalAbilities(rawValue: 1 << 1)
    /// Bring the tab forward.
    static let focus = TerminalAbilities(rawValue: 1 << 2)
    /// Stop the agent and close its tab.
    static let fire = TerminalAbilities(rawValue: 1 << 3)
    /// Move the view of its scrollback.
    static let scroll = TerminalAbilities(rawValue: 1 << 4)

    static let none: TerminalAbilities = []
}

/// One terminal app, as the deck drives it. Each call is made on its
/// terminal's queue (`Terminals.queue(for:)`), and may take a moment; none is
/// made on the main thread.
protocol TerminalDriver {
    var abilities: TerminalAbilities { get }
    /// Whether pressing keys needs Smolcorp's Accessibility permission.
    var keysNeedAccessibility: Bool { get }
    /// Presses `key` in the tab, as the phone's keys and mic do.
    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool
    /// Types `text` and Return into the tab.
    func type(_ text: String, in tab: TabAddress) -> Bool
    /// Puts `text` in the tab without Return, as dictation does, for you to
    /// read before sending.
    func insert(_ text: String, in tab: TabAddress) -> Bool
    /// How this terminal takes text of several lines (`insertLines`), if at all.
    var linesWay: LinesWay { get }
    /// Text of several lines, without Return, as ONE paste the terminal wraps in bracketed-paste
    /// markers: only where the agent in the tab has asked for bracketed paste, so a line break is a
    /// new line in the text and never presses Return. Anywhere else nothing is typed. `ready` is asked
    /// right before the paste itself, once the tab is in front where it must be: nil to go on, or why not,
    /// and nothing is typed (`.refused`).
    func insertLines(_ text: String, in tab: TabAddress, ready: () -> String?) -> LinesPasted
    /// iPhone app (personal build): `text` typed as keys, never as a paste,
    /// without Return: into a menu's own text field, as Claude Code's "Type
    /// something", which a paste may not reach. `insert` where that already
    /// types keys.
    func typeKeys(_ text: String, in tab: TabAddress) -> Bool
    /// iPhone app (personal build): whether `screen` can say what the tab
    /// shows, for a menu to be seen before a choice's key goes.
    var showsScreen: Bool { get }
    func focus(_ tab: TabAddress) -> Bool
    func scroll(lines: Int, in tab: TabAddress) -> Bool
    func scrollToBottom(in tab: TabAddress) -> Bool
    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired
    /// The tab's tty, for looking at what runs there, if it has one.
    func tty(of tab: TabAddress) -> String?
    /// Whether the tab is still there: nil when that can't be told, as for
    /// Terminal.app, whose closed tabs the deck finds from its list of tabs.
    func isOpen(_ tab: TabAddress) -> Bool?
    /// What the tab shows now, as text, for the phone mic's peek at the
    /// input line; nil where the terminal can't say.
    func screen(of tab: TabAddress) -> String?
}

enum Terminals {
    /// Settable for tests, which give a stand-in terminal for every tab.
    static var standIn: TerminalDriver?

    /// The driver for `tab`'s terminal, or nil for one the deck can only watch.
    static func driver(for tab: TabAddress) -> TerminalDriver? {
        if let standIn { return standIn }
        switch tab.kind {
        case "tty": return TerminalApp()
        case "tmux": return Tmux.path == nil ? nil : Tmux()
        case "iterm": return ITerm()
        // Ghostty's terminal on this tty, once learned: until then, only watched.
        case "ghostty": return Ghostty.id(on: tab.id) == nil ? nil : Ghostty()
        case "wezterm": return WezTerm.path == nil ? nil : WezTerm()
        case "kitty": return Kitty.path == nil ? nil : Kitty()
        // A headless run has nothing to drive.
        case "headless": return nil
        default: return nil
        }
    }

    static func abilities(of tab: String) -> TerminalAbilities {
        TabAddress(tab).flatMap(driver(for:))?.abilities ?? .none
    }

    /// The tab's tty, when its address says it, asking nothing: for the main
    /// thread.
    static func ttyInAddress(of tab: String) -> String? {
        guard let address = TabAddress(tab) else { return nil }
        return address.id.hasPrefix("/dev/") ? address.id : nil
    }

    /// The tab's tty, whatever its terminal, when it has one: for `ps` to say
    /// what runs there. It may ask the terminal, so never on the main thread.
    static func tty(of tab: String) -> String? {
        guard let address = TabAddress(tab) else { return nil }
        if let driver = driver(for: address) { return driver.tty(of: address) }
        return address.id.hasPrefix("/dev/") ? address.id : nil
    }
}

/// Terminal.app, through AppleScript and posted keys: see `Terminal`.
struct TerminalApp: TerminalDriver {
    var abilities: TerminalAbilities { [.keys, .text, .focus, .fire, .scroll] }
    var keysNeedAccessibility: Bool { true }

    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        Terminal.press(keystroke.key, modifiers: keystroke.modifiers, times: keystroke.times, tty: tab.id)
    }

    func type(_ text: String, in tab: TabAddress) -> Bool { Terminal.type(text, tty: tab.id) }
    func insert(_ text: String, in tab: TabAddress) -> Bool { Terminal.insert(text, tty: tab.id) }
    func focus(_ tab: TabAddress) -> Bool { Terminal.focus(tty: tab.id) }
    func scroll(lines: Int, in tab: TabAddress) -> Bool { Terminal.scroll(lines: lines, tty: tab.id) }
    func scrollToBottom(in tab: TabAddress) -> Bool { Terminal.scrollToBottom(tty: tab.id) }
    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired { Terminal.fire(tty: tab.id, agent: agent) }
    func tty(of tab: TabAddress) -> String? { tab.id }
    func isOpen(_ tab: TabAddress) -> Bool? { nil }
    func screen(of tab: TabAddress) -> String? { Terminal.contents(tty: tab.id) }

    /// Several lines through the clipboard and ⌘V, the tab brought forward first: Terminal's own paste,
    /// which it wraps in bracketed-paste markers where the agent asked for them.
    var linesWay: LinesWay { .clipboard(inFront: true) }
    func insertLines(_ text: String, in tab: TabAddress, ready: () -> String?) -> LinesPasted {
        ClipboardPaste.paste(text, first: { Terminal.macInUse ? .macInUse : nil },
                             forward: { Terminal.macInUse ? .macInUse : Terminal.forwardForPaste(tty: tab.id) ? nil : .notInFront },
                             ready: ready, press: { posting in Terminal.pressPaste(tty: tab.id, posting: posting) })
    }
}
