import Foundation

/// Which callers may use which setup route: one table, the gate's only source
/// (`DeckRouter.setupGate`). It is also written as
/// `Tests/fixtures/frontdesk/routes.json`, which the Linux/Windows app's tests
/// read too.
///
/// A route that isn't listed keeps 1.23's rule: the page or a tool with the
/// setup-key file. Everything a hire could misuse with the setup-key file (it
/// can read it) is listed, and wants the GM's token or the page. The GM is
/// whoever sends the GM token (`GMIdentity`); it never passes for a tool, nor
/// a tool for it. Under /setup/gm/, a route not listed is the GM's or the
/// page's, so a new one starts closed.
enum SetupRoutes {
    typealias Caller = SetupSessions.Caller

    struct Row: Equatable {
        let method: String
        /// A path, or a prefix ending in `/*`.
        let path: String
        let callers: [Caller]
        /// "read", "action" or "page": what it does, for the reader of the table.
        let kind: String
        let why: String
    }

    private static let all: [Caller] = [.gm, .page, .tool]
    private static let manager: [Caller] = [.gm, .page]
    private static let pageOnly: [Caller] = [.page]

    private static func read(_ path: String, _ why: String) -> Row { Row(method: "GET", path: path, callers: all, kind: "read", why: why) }
    private static func action(_ path: String, _ why: String) -> Row { Row(method: "POST", path: path, callers: manager, kind: "action", why: why) }
    private static func page(_ method: String, _ path: String, _ why: String) -> Row {
        Row(method: method, path: path, callers: pageOnly, kind: "page", why: why)
    }

    static let table: [Row] = [
        // The front desk, for the GM. Reads: the GM, the page or a tool of yours.
        read("/setup/gm/team", "the team: registry merged with live sessions"),
        read("/setup/gm/events", "the event log, since a time, long-polling"),
        read("/setup/gm/version", "the app's version and features"),
        read("/setup/gm/outbox", "the GM's messages and their states"),
        read("/setup/gm/watches", "what the GM asked the app to watch"),
        read("/setup/gm/approvals", "the waits for you"),
        read("/setup/gm/screens", "the screens the team can be tiled on"),
        // Its actions: only the GM or the page.
        action("/setup/gm/hire", "hire an employee"),
        action("/setup/gm/fire", "fire one"),
        action("/setup/gm/brief", "change an employee's brief"),
        action("/setup/gm/tile", "tile the team's windows"),
        action("/setup/gm/screen", "choose the screen that holds the team"),
        Row(method: "POST", path: "/setup/gm/send", callers: [.gm, .page, .hire], kind: "action",
            why: "send a message: signed as the GM, the owner, or (with its own send key) the hire; a hire only to the GM or an allowed contact"),
        action("/setup/gm/contacts", "allow or remove a pair of hires who may write to each other"),
        action("/setup/gm/registry", "edit a registry entry's notes and state"),
        action("/setup/gm/nickname", "name one person in the office (a nickname)"),
        // The GM reads notes and state in the team; the file itself also holds the hires' send-key hashes.
        page("GET", "/setup/gm/registry", "the registry file as kept; the GM reads the team instead"),
        action("/setup/gm/watch", "set a watch"),
        action("/setup/gm/unwatch", "take a watch away"),
        Row(method: "DELETE", path: "/setup/gm/unwatch", callers: manager, kind: "action", why: "take a watch away"),
        action("/setup/gm/manager", "the front desk (manager) switch"),
        page("POST", "/setup/gm/token/rotate", "a new GM token; the old one stops working"),
        page("POST", "/setup/gm/by-hand", "whether the GM's token is also kept in a file, for a GM started by hand"),
        Row(method: "*", path: "/setup/gm/*", callers: manager, kind: "action", why: "a /setup/gm route not listed above starts closed"),

        // 1.23's routes a hire could misuse with the setup-key file.
        action("/setup/hire", "a hire hiring more hires"),
        action("/setup/hire/gm", "starting a GM"),
        action("/setup/hiring", "how and where hires start"),
        action("/setup/hiring/choose", "the folder for new hires"),
        action("/setup/roles", "the roles, and so the briefs future hires get"),
        action("/setup/hooks", "connecting or disconnecting the agents' hooks"),
        action("/setup/unpair", "cutting off your phone"),
        action("/setup/notify", "a push to your phone, signed as the GM"),
        action("/setup/webhook", "where events are sent"),
        action("/setup/webhook/test", "sending to that address"),
        action("/setup/bots", "cloud bots' addresses and keys"),
        action("/setup/bots/override", "a cloud bot's settings"),
        action("/setup/bots/resolve", "a cloud bot's settings"),
        page("POST", "/setup/bots/mailbox", "the folder a cloud bot's mail goes through: only you choose it"),
        action("/setup/maxconfirm", "whether the office asks before max effort"),
        action("/setup/phonemic", "the phone microphone, and whether its text is sent"),
        action("/setup/dictation", "dictation, its language, and Claude's clean-up"),
        action("/setup/login", "opening Smolcorp when you log in"),
        // The desks: tiling and moving them is the GM's job.
        action("/setup/layout", "where the keys sit"),
        action("/setup/profiles", "switching, importing or deleting a Claude profile"),
        read("/setup/claude-accounts", "the Claude accounts, and whether Smolcorp's hooks are in each"),
        read("/setup/guard", "which commands every hire is refused, and your own patterns"),
        action("/setup/pin", "pinning a desk"),
        action("/setup/unpin", "unpinning a desk"),
        action("/setup/floors", "the floors the desks sit on"),

        // Spending and sending elsewhere: the setup page's dialogs alone.
        page("POST", "/setup/summaries", "Summary on or off, only from its dialog"),
        page("POST", "/setup/summary/provider", "where summaries are sent"),
        page("POST", "/setup/summary/key", "the summaries' API key"),
        page("POST", "/setup/viewer", "where the viewer is, and what it hides"),
        page("POST", "/setup/viewer/switch", "the viewer on or off"),
        page("POST", "/setup/viewer/wipe", "forget the viewer"),
        page("POST", "/setup/briefs/restore", "the briefs you edited, put back to the app's own"),
        page("POST", "/setup/claude-accounts", "the Claude accounts Smolcorp's hooks are written into"),
        page("POST", "/setup/terminal-command", "the smolcorp command linked into ~/.local/bin, or taken out"),
        page("POST", "/setup/guard", "the hire guard's groups: a hire or the GM must never loosen its own fence"),
        page("POST", "/setup/keystrokes", "letting features press keys in other apps, only from its switch"),
        page("POST", "/setup/keystrokes/apps", "the apps keys may go to: added or removed, only from the page"),

        // Phones' codes and keys: the page alone (1.23.1).
        page("POST", "/setup/pair", "a pairing code"),
        page("GET", "/setup/qr.png", "the code as a picture"),
        page("GET", "/setup/viewer/qr.png", "the viewer's key as a picture"),
        page("POST", "/setup/viewer/secret", "the viewer's key"),
        page("POST", "/setup/viewer/rotate", "a new viewer key"),

        // A cloud bot's news keeps its tool path.
        Row(method: "*", path: "/bots/*", callers: [.page, .tool], kind: "action", why: "a cloud bot's news, from a tool of yours"),
    ]

    /// The row for a request: an exact path first, then the longest prefix.
    static func row(_ method: String, _ path: String) -> Row? {
        func fits(_ row: Row) -> Bool { row.method == "*" || row.method == method }
        if let exact = table.first(where: { fits($0) && $0.path == path }) { return exact }
        return table.filter { fits($0) && $0.path.hasSuffix("/*") && path.hasPrefix(String($0.path.dropLast())) }
            .max { $0.path.count < $1.path.count }
    }

    /// Whether a caller may use a route. Not listed: the page or a tool.
    static func allows(_ caller: Caller, _ method: String, _ path: String) -> Bool {
        guard let row = row(method, path) else { return caller == .page || caller == .tool }
        return row.callers.contains(caller)
    }

    /// What a refused caller is told.
    static func refusal(_ method: String, _ path: String) -> String {
        row(method, path)?.callers.contains(.gm) == true
            ? "Only the setup page or the GM can do this."
            : "This is for the setup page the menu opens."
    }

    /// The table as `Tests/fixtures/frontdesk/routes.json` holds it.
    static func fixture() -> Data {
        let rows: [[String: Any]] = table.map {
            ["method": $0.method, "path": $0.path, "callers": $0.callers.map(\.name), "kind": $0.kind, "why": $0.why]
        }
        let object: [String: Any] = [
            "unlisted": ["callers": ["page", "tool"], "note": "a /setup route not in the table keeps 1.23's rule"],
            "routes": rows,
        ]
        return (try? JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted, .sortedKeys])) ?? Data()
    }
}

extension SetupSessions.Caller {
    /// As the table and the fixture spell it.
    var name: String {
        switch self {
        case .gm: return "gm"
        case .hire: return "hire"
        case .page: return "page"
        case .tool: return "tool"
        case .nobody: return "nobody"
        }
    }
}
