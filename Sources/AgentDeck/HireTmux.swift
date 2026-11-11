import Foundation

/// A hire's tmux session on a server of its own: `tmux -L smolcorp-<hire id>`
/// (a test copy's: `smolcorp-<port>-<hire id>`), one session called `hire`,
/// started detached in the hire's folder. A terminal window is only a viewer
/// of it: closing the window leaves the hire running. Every call names our
/// socket with -L, so the user's own tmux server, or any socket not made by
/// us, is never reached.
///
/// All of it goes through `run`, which takes tmux's arguments and answers as
/// `Tool.run` does, so tests give a stand-in and no real tmux is needed.
struct HireTmux {
    typealias Runner = ([String]) -> (status: Int32, output: String)?

    /// The one session on the hire's server.
    static let session = "hire"
    /// Every socket we make starts with this, and `owns` checks for it.
    static let prefix = "smolcorp-"
    /// The exact pane target: `=` matches the name exactly, `:` makes it the session's active pane.
    static let pane = "=" + session + ":"
    /// The paste buffer's name: the server is the hire's alone, so one will do.
    static let buffer = "smolcorp-paste"

    let socketName: String
    private let run: Runner

    /// Nil for an id that couldn't be part of a socket's name (only a-z, 0-9 and - are).
    init?(hireID: String, testPort: UInt16? = TestCopy.port, runner: @escaping Runner) {
        guard !hireID.isEmpty, hireID.count <= 64,
              hireID.allSatisfy({ $0.isASCII && ($0.isLowercase || $0.isNumber || $0 == "-") }) else { return nil }
        socketName = HireTmux.prefix + (testPort.map { "\($0)-" } ?? "") + hireID
        run = runner
    }

    /// With the real tmux on this Mac; nil without one.
    init?(hireID: String) {
        guard let path = Tmux.path else { return nil }
        self.init(hireID: hireID, runner: { Tool.run(path, $0) })
    }

    /// The server named `socketName`, with the real tmux, when it is one of ours.
    init?(socketName: String, testPort: UInt16? = TestCopy.port, runner: Runner? = nil) {
        guard HireTmux.owns(socketName, testPort: testPort), socketName.count <= 100,
              socketName.allSatisfy({ $0.isASCII && ($0.isLowercase || $0.isNumber || $0 == "-") }) else { return nil }
        if let runner {
            run = runner
        } else {
            guard let path = Tmux.path else { return nil }
            run = { Tool.run(path, $0) }
        }
        self.socketName = socketName
    }

    /// Whether `socketPath` (or a bare name) is one of ours: a hire's, not the user's `default`.
    /// A test copy owns only `smolcorp-<port>-` sockets, so it never touches a real install's hires.
    static func owns(_ socketPath: String, testPort: UInt16? = TestCopy.port) -> Bool {
        let name = URL(fileURLWithPath: socketPath).lastPathComponent
        return name.hasPrefix(prefix + (testPort.map { "\($0)-" } ?? ""))
    }

    /// tmux's arguments for `arguments` on our server.
    func command(_ arguments: [String]) -> [String] { ["-L", socketName] + arguments }

    private func ran(_ arguments: [String]) -> Bool { run(command(arguments))?.status == 0 }

    /// Starts the session, `command` run by your login shell in `folder`.
    /// Gives its pane and the socket's path (as the deck addresses a tab:
    /// tmux:<pane>@<socket>), or nil if tmux didn't start it.
    func start(in folder: String, shell: String, running command: String) -> (pane: String, socket: String)? {
        let arguments = self.command(Openers.session(HireTmux.session, in: folder, shell: shell, running: command))
        guard let made = run(arguments), made.status == 0 else { return nil }
        let parts = made.output.split(separator: " ", maxSplits: 1).map(String.init)
        guard parts.count == 2, parts[0].hasPrefix("%"), !parts[1].isEmpty else { return nil }
        return (parts[0], parts[1])
    }

    /// Whether the session is there; nil when tmux couldn't be asked. No
    /// server at all (the hire's ended, or the Mac restarted) is a no.
    func isAlive() -> Bool? {
        guard let answer = run(command(["has-session", "-t", "=" + HireTmux.session])) else { return nil }
        return answer.status == 0
    }

    /// The ttys of the terminal windows attached to the session: where its viewers are. Nil when tmux couldn't be asked.
    func clientTTYs() -> [String]? {
        guard let answer = run(command(["list-clients", "-t", "=" + HireTmux.session, "-F", "#{client_tty}"])), answer.status == 0 else { return nil }
        return answer.output.split(separator: "\n").map(String.init).filter(TabAddress.isTTY)
    }

    /// The ttys of every pane on this server (the team grid's: each a viewer of one hire). Nil when tmux couldn't be asked.
    func paneTTYs() -> [String]? {
        guard let answer = run(command(["list-panes", "-a", "-F", "#{pane_tty}"])), answer.status == 0 else { return nil }
        return answer.output.split(separator: "\n").map(String.init).filter(TabAddress.isTTY)
    }

    /// The last `lines` lines of the pane, as text, without colours; nil if it can't be read.
    func capture(lines: Int) -> String? {
        let count = min(max(lines, 1), 5000)
        guard let answer = run(command(["capture-pane", "-p", "-t", HireTmux.pane, "-S", "-\(count)"])),
              answer.status == 0 else { return nil }
        return answer.output
    }

    /// `text` as one bracketed paste, so the agent takes it as a piece of
    /// text. It is never followed by Return: sending it is a decision of its own.
    /// A control character other than a line feed or tab (Escape could end the
    /// bracket early, Return would press Enter) is refused, and `-r` keeps
    /// tmux from turning line feeds into Return.
    func paste(_ text: String) -> Bool {
        guard HireTmux.canPaste(text) else { return false }
        return ran(["set-buffer", "-b", HireTmux.buffer, "--", Tmux.literal(text)])
            && ran(["paste-buffer", "-p", "-r", "-d", "-b", HireTmux.buffer, "-t", HireTmux.pane])
    }

    /// Whether `text` holds nothing but characters that are safe to paste: no control character other than a
    /// line feed or tab. The older `Tmux.insert` asks this too.
    static func canPaste(_ text: String) -> Bool {
        !text.unicodeScalars.contains(where: { $0.properties.generalCategory == .control && $0 != "\n" && $0 != "\t" })
    }

    /// How the pane takes a paste: whether it is in a mode of tmux's own (copy mode, scrolled back: a paste
    /// would go to the mode, not the agent) and whether the agent asked for bracketed paste. Nil if tmux couldn't say.
    /// A tmux before 3.7 has no bracket_paste_flag and leaves it empty (`"0 "`, which `Tool.run` trims to `"0"`):
    /// the pane is then taken as not asking for bracketed paste, the safe way, so a multi-line message goes as one line.
    func pasteState() -> (inMode: Bool, bracketed: Bool)? {
        guard let answer = run(command(["display-message", "-p", "-t", HireTmux.pane, "#{pane_in_mode} #{bracket_paste_flag}"])),
              answer.status == 0 else { return nil }
        let parts = answer.output.split(separator: " ", omittingEmptySubsequences: false).map(String.init)
        guard parts.count == 1 || parts.count == 2, !parts[0].isEmpty,
              parts.allSatisfy({ $0.allSatisfy(\.isNumber) }) else { return nil }
        return (parts[0] != "0", parts.count == 2 && parts[1] == "1")
    }

    /// Return, once. Only after a paste the caller has put into an idle, empty input box: this is the send
    /// of a message, never an answer to a question. Nothing else presses it (`exit` too).
    func submit() -> Bool { ran(["send-keys", "-t", HireTmux.pane, "Enter"]) }

    /// Ends the session the polite way: `/exit` as one paste, then Return. The caller has looked at the
    /// window first and found the input box idle and empty (`HireDesk.inputState`): nothing is ever
    /// pressed over a question or a menu.
    func exit() -> Bool {
        paste("/exit") && submit()
    }

    /// Detaches the terminal windows attached to the session, on this socket
    /// alone: the hire carries on, with no window (agent windows in the background).
    func detachClients() -> Bool { ran(["detach-client", "-s", "=" + HireTmux.session]) }

    /// Ends the hire's server, and so its session. Only this socket's.
    func kill() -> Bool { ran(["kill-server"]) }

    /// The line a terminal window runs to show the session. `exec`, so the window ends when the viewer detaches.
    func attachLine(tmuxPath: String) -> String {
        "exec \(Openers.quoted(tmuxPath)) -L \(Openers.quoted(socketName)) attach-session -t \(Openers.quoted("=" + HireTmux.session))"
    }
}
