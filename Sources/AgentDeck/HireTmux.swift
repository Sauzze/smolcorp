import Foundation

/// A hire's tmux session on a server of its own, as desks started before the sandbox have it: `tmux -L
/// smolcorp-<hire id>` (a test copy's: `smolcorp-<port>-<hire id>`), one session called `hire`. The core starts every
/// desk (desktop/hireengine.go); a terminal window is only a viewer of it, through `smolcorp attach`. Every call names
/// our socket with -L, so the user's own tmux server, or any socket not made by us, is never reached.
///
/// All of it goes through `run`, which takes tmux's arguments and answers as `Tool.run` does, so tests give a stand-in
/// and no real tmux is needed. The real one is `TmuxClient`'s, told "unchecked", as the core tells its own calls on
/// such a socket (desktop/hiretmux.go `tmuxFor`): no server pid was ever checked for a desk from before the sandbox.
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

    /// The server named `socketName`, with the real tmux, when it is one of ours.
    init?(socketName: String, testPort: UInt16? = TestCopy.port, runner: Runner? = nil) {
        guard HireTmux.owns(socketName, testPort: testPort), socketName.count <= 100,
              socketName.allSatisfy({ $0.isASCII && ($0.isLowercase || $0.isNumber || $0 == "-") }) else { return nil }
        if let runner {
            run = runner
        } else {
            guard let path = Tmux.path else { return nil }
            run = { TmuxClient.run(path, server: TmuxClient.unchecked, $0) }
        }
        self.socketName = socketName
    }

    /// Whether `socketPath` (or a bare name) is one of ours: a hire's, not the user's `default`. A desk from before the
    /// sandbox has `smolcorp-<id>` in tmux's own folder; one started in its sandbox, `s` in a folder of its own,
    /// `<id>`, inside `/tmp/smolcorp-<uid>` (desktop/hiretmux.go `deskSocketPath`). A test copy owns only those with
    /// its port first (`smolcorp-<port>-<id>`, `<port>-<id>`), so it never touches a real install's hires.
    static func owns(_ socketPath: String, testPort: UInt16? = TestCopy.port) -> Bool {
        let port = testPort.map { "\($0)-" } ?? ""
        if let folder = sandboxedFolder(socketPath) { return folder.hasPrefix(port) && isDeskID(String(folder.dropFirst(port.count))) }
        return URL(fileURLWithPath: socketPath).lastPathComponent.hasPrefix(prefix + port)
    }

    /// Whether `socketPath` is shaped as any desk's socket, a test copy's or the real app's: a server the app drives only
    /// with what the core recorded of that desk (`TmuxDesk`), never on its own.
    static func isDeskSocket(_ socketPath: String) -> Bool {
        sandboxedFolder(socketPath) != nil || URL(fileURLWithPath: socketPath).lastPathComponent.hasPrefix(prefix)
    }

    /// The folder of a desk's socket in its sandbox, `/tmp/smolcorp-<uid>/<folder>/s` (or /private/tmp's), or nil.
    private static func sandboxedFolder(_ socketPath: String) -> String? {
        let parts = socketPath.split(separator: "/", omittingEmptySubsequences: false).map(String.init)
        let parent = parts.dropLast(2).joined(separator: "/")
        guard socketPath.hasPrefix("/"), parts.count >= 4, parts.last == "s", !parts.contains(".."),
              ["/tmp", "/private/tmp"].contains(where: { parent == $0 + "/smolcorp-\(getuid())" }) else { return nil }
        return parts[parts.count - 2]
    }

    /// Whether `id` can be a desk's id: only a-z, 0-9 and -, as the core makes them (desktop/hireshared.go `plainHireID`).
    static func isDeskID(_ id: String) -> Bool {
        !id.isEmpty && id.count <= 64 && id.allSatisfy { $0.isASCII && ($0.isLowercase || $0.isNumber || $0 == "-") }
    }

    /// tmux's arguments for `arguments` on our server.
    func command(_ arguments: [String]) -> [String] { ["-L", socketName] + arguments }

    private func ran(_ arguments: [String]) -> Bool { run(command(arguments))?.status == 0 }

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
}

/// The one way the app runs tmux (SA-M9, SANDBOX-PLAN §A3.1), as the core's own (desktop/tmuxclient.go `tmuxClient`):
/// told the server it may talk to, in `SMOLCORP_TMUX_SERVER`, the bundled tmux's guard (third_party/tmux/guard.patch):
/// a desk's server pid as the core checked it at its start and gave it over the bridge (`TmuxDesk`), or "unchecked"
/// (a desk from before the sandbox, or a tmux of your own; a stock tmux ignores the variable). Never "off", which only
/// the core's start of a server gives. Nothing of the app's own environment reaches it, nor through it a server: a
/// fixed PATH, TERM and LANG alone, and stdin from /dev/null unless `input` is given (`load-buffer -`). Any other
/// guard, and nothing runs. Started by `Tool.run`, so on its own account (`Disclaimed`).
enum TmuxClient {
    static let variable = "SMOLCORP_TMUX_SERVER"
    static let unchecked = "unchecked"
    static let environment = ["PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "TERM": "xterm-256color", "LANG": "en_US.UTF-8"]

    /// Whether `server` is a guard the app may give: "unchecked", or a pid above 1 written as a plain number.
    static func isGuard(_ server: String) -> Bool {
        server == unchecked || Int32(server).map { $0 > 1 && String($0) == server } == true
    }

    static func run(_ program: String, server: String, _ arguments: [String], input: Data? = nil) -> (status: Int32, output: String)? {
        guard !program.isEmpty, isGuard(server) else { return nil }
        return Tool.run(program, arguments, input: input, environment: environment.merging([variable: server]) { $1 })
    }
}

/// A desk's tmux as the core recorded it, given over the bridge with an action on its pane (SA-M9, desktop/hostbridge.go
/// `bridgeDesk`): its id, the guard its calls are told (`TmuxClient`), the pid of the process its start made in its
/// pane and when that process started (µs since 1970, as the core's processStartedAt; 0 for none recorded), and its
/// folder. The `Tmux` driver drives a desk's pane only with these: what a desk's server says of its own pane
/// (#{pane_pid}, #{pane_current_path}) is never taken (RSBXPLANc N1).
struct TmuxDesk: Equatable {
    /// The start of a plain hire's own server's id (desktop/hire_unix.go `plainHireTmuxID`, SA-M10): a server the core
    /// started for a hire with no registry row, which the core gives from its own record of the sockets it made, told
    /// "unchecked" (desktop/hostbridge.go `deskOfPane`).
    static let plain = "plain-"

    let id: String
    let server: String
    var pane: pid_t = 0
    var paneStarted: Int64 = 0
    var folder = ""

    /// From the bridge's `desk`; nil when it isn't one.
    init?(_ object: Any?) {
        guard let desk = object as? [String: Any], let id = desk["id"] as? String, HireTmux.isDeskID(id),
              let server = desk["server"] as? String, TmuxClient.isGuard(server) else { return nil }
        let folder = desk["folder"] as? String ?? ""
        guard folder.isEmpty || folder.hasPrefix("/") && !folder.utf8.contains(0) else { return nil }
        self.init(id: id, server: server, pane: (desk["pane"] as? NSNumber).flatMap { pid_t(exactly: $0.int64Value) } ?? 0,
                  paneStarted: (desk["paneStarted"] as? NSNumber)?.int64Value ?? 0, folder: folder)
    }

    init(id: String, server: String, pane: pid_t = 0, paneStarted: Int64 = 0, folder: String = "") {
        (self.id, self.server, self.pane, self.paneStarted, self.folder) = (id, server, pane, paneStarted, folder)
    }
}
