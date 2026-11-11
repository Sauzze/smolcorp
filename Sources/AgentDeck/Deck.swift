import AppKit
import os

/// What the Mac app keeps of the deck it ran before 1.25.0, for the Go core that
/// runs it now (`Engine`): its port, and what the bridge does on this Mac with the
/// app's own code (`HostBridge`): the phone's words typed into a terminal, with
/// every gate and sentence they had, the Mac's folder panel, and a few words the
/// core's answers share.
enum Deck {
    static let port: UInt16 = TestCopy.port ?? 7710

    /// Without Accessibility, posted keys vanish without a trace. The phone
    /// says so instead, since every update turns the permission off.
    static let noKeyboard = "Keyboard access is off. Turn it on in setup, on your Mac."

    static let effortLevels = ["low", "medium", "high", "xhigh", "max"]

    /// `path` as a folder that's there, with ~ for home, or nil.
    static func folder(_ path: String) -> String? {
        let expanded = (path.trimmingCharacters(in: .whitespaces) as NSString).expandingTildeInPath
        var isFolder: ObjCBool = false
        guard expanded.hasPrefix("/"), FileManager.default.fileExists(atPath: expanded, isDirectory: &isFolder),
              isFolder.boolValue else { return nil }
        return (expanded as NSString).standardizingPath
    }

    /// The folder panel, while it's open: one for the app, whichever engine asked for it (the Go core asks
    /// through the bridge, HostBridge's choose-folder).
    private static var hirePanel: NSOpenPanel?

    /// What the panel says it's for: the hires folder, or a project's folder for a floor (Floors' Choose…).
    static let hireFolderMessage = "New hires from the office start their work in this folder."
    static let floorFolderMessage = "The agents working in this project's folder sit on the floor you chose it for."

    /// The Mac's own folder panel, opened at `start` (nil: the panel's own choice) and saying `message`:
    /// `done` gets the folder picked, or nil. Asked again while it's open, it brings that panel forward
    /// rather than opening another, and answers nil at once. The Go core keeps what's picked (HostBridge's
    /// choose-folder).
    static func chooseFolder(from start: String?, message: String, then done: @escaping (String?) -> Void) {
        // A test copy opens no panel: nothing is picked, as if cancelled. Its tester types the folder on the page.
        guard TestCopy.mayComeForward("the folder panel") else { return done(nil) }
        if let open = hirePanel {
            Foreground.activate("the folder panel")
            open.makeKeyAndOrderFront(nil)
            return done(nil)
        }
        let panel = NSOpenPanel()
        hirePanel = panel
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        panel.prompt = "Choose"
        panel.message = message
        if let start { panel.directoryURL = URL(fileURLWithPath: start) }
        Foreground.activate("the folder panel")
        panel.begin { response in
            hirePanel = nil
            done(response == .OK ? panel.url?.path : nil)
        }
    }

    /// Closes the folder panel if it's open saying `message`, as if cancelled.
    static func closeFolderPanel(for message: String) {
        guard let open = hirePanel, open.message == message else { return }
        open.cancel(nil)
    }

    static let answerFirst = "It's asking you something: answer it on the deck first."
    static let noLines = "This desk's terminal can't take several lines as one paste, so nothing was typed: send it as one line."
    static let linesInCopyMode = "This desk's tmux pane is scrolled back, so nothing was typed: bring it back to the bottom and try again."
    static func noLines(for agent: Agent) -> String {
        "\(agent.name) isn't known to take several lines as one paste here, so nothing was typed: send it as one line."
    }
    static let lockedLines = "Several lines are pasted with ⌘V in this desk's terminal, which can't be done with the Mac locked, so nothing was typed: send it as one line."
    static let linesWhileWorking = "It's working, and several lines go only while it waits at its prompt, so nothing was typed: send them once it's finished."
    static let linesUnknown = "Smolcorp can't tell what this desk's tab is showing now, so nothing was typed: try again in a moment, or send it as one line."
    static let linesAgentEnded = "That agent has ended, so nothing was typed."
    static let linesBehind = "That agent isn't in front in its terminal, so nothing was typed: bring it back with fg."
    static let linesInEditor = "An editor is open in this desk's tab, so nothing was typed: close it, then send again."
    static let linesInPager = "A pager (less, man) is open in this desk's tab, so nothing was typed: quit it, then send again."
    static let linesInShell = "A shell or a command is running in this desk's tab, so nothing was typed: send again once it's finished."
    /// Names what has the tab's keys when it's known, so the owner can find it (FXI34d); "?" is a nameless one.
    static func linesInTab(_ readers: Set<String>) -> String {
        let names = readers.filter { $0 != "?" }.sorted()
        guard !names.isEmpty else { return "Something else open in this desk's tab has the tab's keys, so nothing was typed: close it, then send again." }
        return "\(names.joined(separator: " and ")) \(names.count == 1 ? "has" : "have") this tab's keys, so nothing was typed: close it, then send again."
    }
    static let linesNotInFront = "This desk's tab couldn't be brought to the front, or another tab or app was in front by the time of the paste, so nothing was typed: try again."
    static let linesMacInUse = "The Mac is in use, and pasting would bring this desk's terminal to the front, so nothing was typed: try again once it's been left alone for 10 seconds, or send it as one line."
    static let linesClipboardKept = "What's on the Mac's clipboard (a file being copied, perhaps) couldn't be put back exactly after the paste, so nothing was typed: send it as one line, or copy something else first."
    static let linesNotRead = "The terminal hadn't taken the paste in time, so no Return was sent: look at the Mac before trying again, as the text may still appear."

    /// What the phone is told when several lines weren't pasted as they should be: nil for `.pasted`, and for
    /// `.failed`, which is said as any key that didn't go (404, or 503 when its terminal stopped answering).
    static func linesResponse(_ pasted: LinesPasted) -> Response? {
        switch pasted {
        case .pasted, .failed: return nil
        case .notTaken: return .status(409, noLines)
        case .inMode: return .status(409, linesInCopyMode)
        case .refused(let why): return .status(409, why)
        case .notInFront: return .status(409, linesNotInFront)
        case .macInUse: return .status(409, linesMacInUse)
        case .unsaved: return .status(409, linesClipboardKept)
        // ⌘V went: not a refusal, so the page doesn't offer the words again to paste twice.
        case .notRead: return .status(503, linesNotRead)
        }
    }

    /// Where `agent` is in its tab `tab`, and what else is in front there (see `Terminal.front`). Settable
    /// for tests, which give a stand-in: nothing real is looked at.
    static var tabFront: (Agent, String) -> Terminal.TabFront? = { agent, tab in
        Terminals.tty(of: tab).flatMap { Terminal.front(of: agent, on: $0) }
    }

    /// Why several lines can't be pasted through the clipboard now (RI34b B1), nil if they may. The terminal
    /// can't say whether the agent asked for bracketed paste, which an agent on the list does only at its
    /// main prompt (`Agent.asksForBracketedPaste`): so only while the desk waits there (idle, unread or
    /// error: not asking, not working, as a tool's own prompt runs while it works), and only with the agent
    /// in front of its tab, nothing else in its foreground job reading the tab's keys (RI34c S1): no
    /// terminal editor it opened (Claude Code turns bracketed paste off while one has the tab), whatever
    /// it's called, no pager, no shell. The names only pick the sentence. Not known is refused.
    static func clipboardRefusal(state: SessionState?, front: Terminal.TabFront?) -> String? {
        switch state {
        case .asking?: return answerFirst
        case .thinking?: return linesWhileWorking
        case nil: return linesUnknown
        case .idle?, .unread?, .error?: break
        }
        guard let front else { return linesUnknown }
        switch front.place {
        case .gone: return linesAgentEnded
        case .behind: return linesBehind
        case .front: break
        }
        let readers = front.readers
        guard !readers.isEmpty else { return nil }
        if readers.contains(where: { editors.contains($0) || $0.hasPrefix("emacs") }) { return linesInEditor }
        if !readers.isDisjoint(with: pagers) { return linesInPager }
        if !readers.isDisjoint(with: shells) { return linesInShell }
        return linesInTab(readers)
    }

    /// Only to pick the sentence (see `clipboardRefusal`), never what refuses: what `$EDITOR` and `$VISUAL`
    /// are often set to, in a terminal or waiting on a window (`code --wait`); pagers; shells, and what
    /// opens one or asks for a password.
    static let editors: Set<String> = ["vi", "vim", "nvim", "view", "vimdiff", "gvim", "mvim", "nano", "pico", "micro", "hx", "helix",
                                       "kak", "joe", "jed", "ne", "mg", "ed", "ex", "code", "cursor", "windsurf", "subl", "zed",
                                       "mate", "bbedit", "edit", "open"]
    static let pagers: Set<String> = ["less", "more", "most", "pg", "man", "bat", "delta", "w3m", "lynx"]
    static let shells: Set<String> = ["sh", "bash", "zsh", "fish", "dash", "ksh", "mksh", "tcsh", "csh", "nu", "xonsh", "elvish",
                                      "pwsh", "login", "sudo", "su", "ssh", "mosh", "telnet", "script", "tmux", "screen"]

    /// Why several lines can't go to a desk in a terminal that takes them `way`, for `agent`, known before
    /// its terminal is asked anything: nil if they may. A terminal that can't take them gets `noLines`; one
    /// pasting through the clipboard, for an agent not known to ask for bracketed paste (where each line
    /// break would press Return), `noLines(for:)`. ⌘V needs Accessibility and the Mac unlocked.
    static func linesRefusal(_ way: LinesWay, agent: Agent, accessibility: () -> Bool = { Permissions.hasAccessibility },
                             locked: () -> Bool = { Terminal.isLocked }) -> Response? {
        switch way {
        case .never: return .status(409, noLines)
        case .paneFlag: return nil
        case .clipboard(let inFront):
            guard agent.asksForBracketedPaste else { return .status(409, noLines(for: agent)) }
            guard inFront else { return nil }
            if !accessibility() { return .status(403, noKeyboard) }
            return locked() ? .status(409, lockedLines) : nil
        }
    }
    /// Whether this app may post keys, and whether the Mac is locked: settable for tests, which give stand-ins
    /// so nothing real is looked at.
    static var keyboardAllowed: () -> Bool = { Permissions.hasAccessibility }
    static var macLocked: () -> Bool = { Terminal.isLocked }

    /// How the phone's words went into a tab (`typePhoneWords`).
    enum PhoneTyped {
        /// In, without Return.
        case typed
        /// Nothing typed, and what the phone is told (a 409).
        case refused(Response)
        /// ⌘V went, but the terminal hadn't read the paste in time: it may still appear (503 `linesNotRead`).
        case notRead(Response)
        /// The terminal didn't take it, or ended for taking too long: it may have typed some of it.
        case failed
    }

    /// The phone's words to a desk whose terminal is `driver` (its `way` for several lines, whether its keys
    /// need Accessibility), for `agent`, refused before anything waits on the terminal's queue: nil if they
    /// may go. The bridge's `phone-text` asks it for the Go core (`HostBridge.phoneAction`, IGL): posted keys
    /// need Accessibility, and
    /// several lines `linesRefusal` (a terminal that can't take them, an agent not known to ask for bracketed
    /// paste, a locked Mac).
    static func phoneWordsRefusal(_ words: String, way: LinesWay, keysNeedAccessibility: Bool, agent: Agent) -> Response? {
        guard !keysNeedAccessibility || keyboardAllowed() else { return .status(403, noKeyboard) }
        guard PhoneDictation.isLines(words) else { return nil }
        return linesRefusal(way, agent: agent, accessibility: keyboardAllowed, locked: macLocked)
    }

    /// The phone's words typed into `tab` by its `driver`, without Return: the one way both engines type them
    /// (IGL), on the terminal's queue, once `phoneWordsRefusal` let them and the agent is known to be in front
    /// there, or that can't be told. `state` is the desk's state as its engine has it right then (the Swift
    /// engine's sessions, or the Go core's, asked over the bridge), nil for a desk it no longer has.
    ///
    /// One line goes as typed text, unless the desk is asking you something by then. Several go as one paste
    /// the terminal wraps in bracketed-paste markers, where the agent asked for them: a line break is then a
    /// new line in the text, never Return. Where it can't be, nothing is typed: never flattened, never a
    /// Return per line (I34). Right before the paste itself, after the tab was brought forward where it must
    /// be, `ready` asks again: through the clipboard, where the terminal can't say whether the agent asked
    /// for bracketed paste, only while the desk waits at its prompt with nothing else in front of it in its
    /// tab (`clipboardRefusal`, RI34b B1); in tmux, which reads the pane's own flag, while it asks nothing.
    static func typePhoneWords(_ words: String, driver: TerminalDriver, tab: TabAddress, agent: Agent,
                               state: @escaping () -> SessionState?) -> PhoneTyped {
        guard PhoneDictation.isLines(words) else {
            guard !isAsking(state()) else { return .refused(.status(409, answerFirst)) }
            return driver.insert(words, in: tab) ? .typed : .failed
        }
        let address = tab.text
        let ready: () -> String?
        if case .clipboard = driver.linesWay {
            ready = { clipboardRefusal(state: state(), front: tabFront(agent, address)) }
        } else {
            ready = { isAsking(state()) ? answerFirst : nil }
        }
        let pasted = driver.insertLines(words, in: tab, ready: ready)
        switch (pasted, linesResponse(pasted)) {
        case (.pasted, _): return .typed
        case (.notRead, let said?): return .notRead(said)
        case (_, let said?): return .refused(said)
        case (_, nil): return .failed
        }
    }

    /// Whether a session's agent is waiting on you: a permission prompt or a question, whose menu keys would answer.
    static func isAsking(_ state: SessionState?) -> Bool { state == .asking }
}
