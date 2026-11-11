import XCTest
@testable import AgentDeck

/// The route contract (M0 of the one-build plan): what the Mac answers on each route, written as shapes into
/// `Tests/fixtures/contract/`, which the Linux/Windows app's `desktop/contract_test.go` checks its own answers
/// against. Shapes, not values: an answer's status, its kind of body, and for JSON its keys, their types, and
/// for a closed set (a session's state, an agent…) the values it may take. A run with SMOLCORP_WRITE_FIXTURES=1
/// writes the files again; a normal run checks the Mac still answers so. `INVENTORY.md` beside them lists every
/// route, with or without a fixture, and every place a shape doesn't compare all the way down.
///
/// Only routes that write nothing outside the test's own folder are asked: none that writes your home folder
/// (hooks, login item, the smolcorp link), asks the Keychain, opens a window or a terminal, or starts another
/// program. Some do read this Mac (the agents and terminals here, its Claude accounts, its screens): those lists
/// are "array" in the shapes, listed in INVENTORY.md, so a Mac with fewer still passes. Routes not asked are in
/// the inventory with why, with their refusals here where a refusal comes before anything is done.
final class ContractTests: XCTestCase {
    // MARK: The routes asked

    /// One request, as one caller: the page (its cookie), a tool (the setup key), a paired phone on the Wi-Fi,
    /// the hook (its key), or nobody. `given` is what's set up first (see `givens`).
    struct Ask {
        var caller = "page"
        var query: [String: String] = [:]
        var body: Any?
        var given: [String] = []
        var headers: [String: String] = [:]

        var json: [String: Any] {
            var json: [String: Any] = ["as": caller]
            if !query.isEmpty { json["query"] = query }
            if !headers.isEmpty { json["headers"] = headers }
            if let body { json["body"] = body }
            if !given.isEmpty { json["given"] = given }
            return json
        }
    }

    /// A route: the request answered as it should be (none where only refusals can be asked safely), and the
    /// requests refused, each with why.
    struct Route {
        let method: String
        let path: String
        var ask: Ask?
        var refusals: [(ask: Ask, why: String)] = []
        var note: String?

        init(_ method: String, _ path: String, _ ask: Ask? = Ask(), refusals: [(Ask, String)] = [], note: String? = nil) {
            self.method = method
            self.path = path
            self.ask = ask
            self.refusals = refusals.map { (ask: $0.0, why: $0.1) }
            self.note = note
        }

        var name: String { "\(method) \(path)" }

        /// Who may call it: setup's table, or for a route outside setup, what its gate lets in.
        var callers: [String] {
            if DeckRouter.isSetupPath(path) || path.hasPrefix("/bots/") {
                guard let row = SetupRoutes.row(method, path) else { return ["page", "tool"] }
                return row.callers.map(\.name)
            }
            switch path {
            case "/hook": return ["hook"]
            case "/pair": return ["anyone"]
            case "/t3code", "/t3code/open", "/hire/window": return ["window"]
            default: return ["phone"]
            }
        }
    }

    private static func ask(_ caller: String = "page", query: [String: String] = [:], body: Any? = nil, given: [String] = [],
                            headers: [String: String] = [:]) -> Ask {
        Ask(caller: caller, query: query, body: body, given: given, headers: headers)
    }

    /// What the setup state is asked with, and every setup route that answers it: so that each answers it whole,
    /// with something in each of its lists.
    /// The floor rule comes first: it puts contract-b upstairs as it starts.
    static let stateGiven = ["a floor rule", "sessions", "a pin", "a saved profile", "a cloud bot", "a pairing code", "a paired phone"]
    /// What the office's snapshot is asked with: a desk at the pinned key, one at a free key, one upstairs, a
    /// cloud bot, and a hire in T3 Code's desk (T3D-a), first, as its registry is read before anything else.
    static let officeGiven = ["a T3 hire", "a floor rule", "sessions", "a pin", "a session at a free key", "a cloud bot"]

    /// The setup page's state route, and the office's snapshot (the first message of /events): the shapes most
    /// routes refer to, by name.
    static let state: [Route] = [
        Route("GET", "/setup/state", ask(given: stateGiven),
              refusals: [(ask("nobody"), "no cookie, no setup key: the locked page")]),
        Route("GET", "/setup/state", ask("tool", given: stateGiven),
              note: "a tool sees whether pairing is open, never the code, and no webhook secret"),
        Route("GET", "/events", ask("phone", given: officeGiven),
              refusals: [(ask("nobody"), "no phone's key")],
              note: "an event stream; its first message is the snapshot (shape snapshot), and each change sends it again"),
    ]

    /// Setup's routes the page uses, besides the GM's. Each asked with the setup state's own given, so those that
    /// answer with the setup state answer it whole, as `GET /setup/state` does.
    static let setup: [Route] = setupRoutes.map { route in
        var route = route
        if route.ask != nil, route.ask?.given.isEmpty == true { route.ask?.given = stateGiven }
        return route
    }

    /// The roles as the app comes with them: what `POST /setup/roles` is sent, so it answers the state unchanged.
    static let shippedRoles: Any = {
        let data = (try? JSONEncoder().encode(Role.shipped)) ?? Data("[]".utf8)
        return (try? JSONSerialization.jsonObject(with: data)) ?? [Any]()
    }()

    private static let setupRoutes: [Route] = [
        Route("POST", "/setup/session", nil, refusals: [(ask("nobody"), "no menu link's key: no session")],
              note: "the menu's one-time link key, in the setup-key header, for a cookie (204 with Set-Cookie)"),
        Route("POST", "/setup/scroll", ask(query: ["enabled": "1"])),
        Route("POST", "/setup/maxconfirm", ask(query: ["enabled": "1"])),
        Route("POST", "/setup/guide", ask(query: ["done": "1"])),
        Route("POST", "/setup/phonemic", ask(query: ["enabled": "0"])),
        Route("POST", "/setup/logo", ask(query: ["logo": "keys"]),
              refusals: [(ask(query: ["logo": "nope"]), "no such logo")]),
        Route("POST", "/setup/theme", ask(body: [String: Any]())),
        Route("POST", "/setup/layout", ask(body: ["keyCount": 6, "showProject": true]),
              refusals: [(ask(body: "not a layout"), "not a layout")]),
        Route("POST", "/setup/pin", ask(query: ["key": "0", "folder": "/work/alpha"]),
              refusals: [(ask(query: ["key": "99", "folder": "/work/alpha"]), "no such key"),
                         (ask(query: ["key": "0"]), "no folder")]),
        Route("POST", "/setup/unpin", ask(query: ["key": "0"]),
              refusals: [(ask(query: ["key": "x"]), "no such key")]),
        Route("POST", "/setup/floors", ask(body: ["rules": [["folder": "/work/beta", "floor": 2, "exact": false]]]),
              refusals: [(ask(body: [String: Any]()), "neither floors nor rules")]),
        Route("POST", "/setup/webhook", ask(body: ["enabled": false]),
              refusals: [(ask(body: "not json"), "not a webhook"),
                         (ask(body: ["enabled": true, "url": "ftp://example.invalid/x"]), "an address it can't send to")]),
        Route("POST", "/setup/profiles", ask(query: ["action": "save", "name": "Contract"]),
              refusals: [(ask(query: ["action": "nope"]), "unknown action"),
                         (ask(query: ["action": "check"], body: "not a profile"), "check: not a profile")]),
        Route("POST", "/setup/pair", ask()),
        Route("POST", "/setup/unpair", ask(query: ["phone": "no-such-phone"]),
              refusals: [(ask(), "neither phone= nor all=1")]),
        Route("GET", "/setup/qr.png", ask(given: ["a pairing code"]),
              refusals: [(ask(), "no pairing code asked for")]),
        Route("GET", "/setup/logo/nope", nil, refusals: [(ask(), "no such logo")],
              note: "GET /setup/logo/<logo> answers the logo's PNG"),
        Route("POST", "/setup/hiring", ask(query: ["agent": "claude"]),
              refusals: [(ask(query: ["agent": "nope"]), "unknown agent"),
                         (ask(query: ["terminal": "nope"]), "a terminal that isn't here"),
                         (ask(query: ["folder": "/no/such/folder/for/contract"]), "no folder there")]),
        Route("POST", "/setup/hire-folder", nil, refusals: [(ask(query: ["folder": "/no/such/folder/for/contract"]), "no folder there")],
              note: "a folder that's there is saved as the folder for your hires; only the refusal, which comes first, is asked"),
        Route("POST", "/setup/summaries", ask(query: ["enabled": "0"]),
              refusals: [(ask(query: ["enabled": "1"]), "on only from its dialog (confirmed=1)"),
                         (ask(query: ["enabled": "1", "confirmed": "1", "provider": "nope"]), "no such provider")]),
        Route("POST", "/setup/summary/provider", nil, refusals: [(ask(query: ["id": "nope"]), "no such provider")]),
        Route("POST", "/setup/summary/key", nil, refusals: [(ask(body: ["key": "not-a-key"]), "not an Anthropic API key")],
              note: "the key goes to the Keychain (the Mac) or the secret store; only the refusal is asked here"),
        Route("POST", "/setup/dictation", nil, refusals: [(ask(query: ["language": "xx"]), "unknown language")]),
        Route("POST", "/setup/dictation/glossary", nil, refusals: [(ask(body: "not a list"), "not a list of names")]),
        Route("POST", "/setup/keystrokes", nil, refusals: [(ask(), "neither enabled=1 nor enabled=0")]),
        Route("POST", "/setup/keystrokes/apps", nil, refusals: [(ask(), "neither add= nor remove=")]),
        Route("GET", "/setup/guard", ask()),
        Route("POST", "/setup/guard", nil, refusals: [(ask(body: "not json"), "not a guard file")]),
        Route("GET", "/setup/claude-accounts", ask()),
        Route("POST", "/setup/claude-accounts", nil, refusals: [(ask(body: [String: Any]()), "no profiles list")]),
        Route("GET", "/setup/bots", ask()),
        Route("POST", "/setup/bots", ask(body: ["bots": [["id": "contract-bot", "name": "Contract Bot"]]]),
              refusals: [(ask(body: "not json"), "not a list of bots")]),
        Route("POST", "/setup/bots/override", ask(body: ["id": "contract-bot", "floor": "2"]),
              refusals: [(ask(body: ["botFloor": "2"]), "botFloor not a whole number"),
                         (ask(body: ["id": "no-such-bot"]), "no such bot")]),
        Route("POST", "/setup/bots/mailbox", nil, refusals: [(ask(body: [String: Any]()), "no id and folder")]),
        Route("POST", "/setup/bots/resolve", ask()),
        Route("GET", "/setup/hire-folder", ask()),
        Route("GET", "/setup/hire/folders", ask()),
        Route("GET", "/setup/hire/gm/preflight", nil, refusals: [(ask(query: ["account": "no-such-account"]), "no such Claude account")],
              note: "with a known account it checks the GM's tools, which runs them; only the refusal, which comes first, is asked"),
        Route("POST", "/setup/roles", ask(body: shippedRoles),
              refusals: [(ask(body: "not json"), "not a list of roles")],
              note: "the roles, whole: sent as they come with the app, so the state is answered unchanged"),
        Route("GET", "/setup/briefs", ask(given: ["the app's briefs"])),
        Route("POST", "/setup/briefs/preview", nil, refusals: [(ask(body: "not json"), "not a role and folder")],
              note: "reads the brief a hire would get; this test bundle has none of the app's briefs, so only the refusal is asked"),
        Route("POST", "/setup/hire", nil, refusals: [(ask(body: [String: Any]()), "no role"),
                                                     (ask(body: ["role": "nobody-at-all"]), "no such role")]),
        Route("POST", "/setup/hire/gm", nil, refusals: [(ask(), "no folder for your hires yet")],
              note: "hires the GM in the folder for your hires; asked here only with none set"),
        Route("POST", "/setup/notify", nil, refusals: [(ask(body: [String: Any]()), "no title or body")]),
        Route("POST", "/setup/viewer", nil, refusals: [(ask(body: "not json"), "not a viewer setting")]),
        // The two plain choices (T796): asked with nothing switched to the background, which would detach the hires' windows.
        Route("POST", "/setup/choices", ask(query: ["t3": "0"]),
              note: "background=1 is refused (400) where there's no tmux: not asked, as whether there is depends on this computer"),
        Route("POST", "/setup/hire/window", nil, refusals: [(ask(body: ["id": "no-such-hire"]), "no such hire")],
              note: "a hire that is found has a window opened for it; only the refusal, which comes first, is asked"),
        Route("GET", "/setup/t3code", nil, refusals: [(ask(), "T3 Code is off")],
              note: "with T3 Code on it looks for T3 Code's app and runs its command; only the refusal, which comes first, is asked"),
        Route("POST", "/setup/t3code/open", nil, refusals: [(ask(body: ["id": "no-such-hire"]), "T3 Code is off"),
                                                          (ask(body: ["id": "no-such-hire"], given: ["T3 Code on"]), "no such hire")]),
        Route("POST", "/setup/t3code/letgo", nil, refusals: [(ask(body: ["id": "no-such-hire"]), "no such hire")],
              note: "a hire in T3 Code that is found is let go there; only the refusal, which comes first, is asked"),
        Route("POST", "/setup/fix", nil, refusals: [(ask("tool", query: ["fix": "nope"]), "a tool only repairs")]),
        Route("GET", "/setup/nothing-here", nil, refusals: [(ask(), "no such route")]),
    ]

    /// The GM's routes, under /setup/gm/, with the manager on (each is 409 while it's off).
    static let gm: [Route] = [
        Route("GET", "/setup/gm/version", ask()),
        Route("POST", "/setup/gm/manager", ask(query: ["enabled": "1"]),
              refusals: [(ask(query: ["enabled": "2"]), "neither enabled=1 nor enabled=0")]),
        Route("GET", "/setup/gm/team", ask(given: ["manager on", "sessions"]),
              refusals: [(ask(), "the manager is off")]),
        Route("GET", "/setup/gm/events", ask(query: ["since": "0", "wait": "0"], given: ["manager on", "sessions"])),
        Route("GET", "/setup/gm/approvals", ask(given: ["manager on", "sessions"])),
        Route("GET", "/setup/gm/outbox", ask(given: ["manager on", "an outbox message"])),
        Route("GET", "/setup/gm/watches", ask(given: ["manager on", "a watch"])),
        Route("POST", "/setup/gm/watch", nil, refusals: [(ask(body: "not json", given: ["manager on"]), "not a watch")]),
        Route("POST", "/setup/gm/unwatch", nil, refusals: [(ask(given: ["manager on"]), "no id"),
                                                           (ask(query: ["id": "no-such-watch"], given: ["manager on"]), "no such watch")]),
        Route("DELETE", "/setup/gm/unwatch", nil, refusals: [(ask(given: ["manager on"]), "no id")]),
        Route("GET", "/setup/gm/screens", ask(given: ["manager on"])),
        Route("POST", "/setup/gm/screen", nil, refusals: [(ask(given: ["manager on"]), "no id"),
                                                          (ask(query: ["id": "no-such-screen"], given: ["manager on"]), "no such screen")]),
        Route("POST", "/setup/gm/hire", nil, refusals: [(ask(body: [String: Any](), given: ["manager on"]), "no role")],
              note: "the request and answer are frontdesk/hire.json's"),
        Route("POST", "/setup/gm/fire", nil, refusals: [(ask(body: [String: Any](), given: ["manager on"]), "no id")],
              note: "the request and answer are frontdesk/fire.json's"),
        Route("POST", "/setup/gm/brief", nil, refusals: [(ask(body: [String: Any](), given: ["manager on"]), "no hire named")],
              note: "the request and answer are frontdesk/brief.json's"),
        Route("POST", "/setup/gm/send", nil, refusals: [(ask(body: [String: Any](), given: ["manager on"]), "no one to send to")],
              note: "the request and answer are frontdesk/send.json's"),
        Route("POST", "/setup/gm/contacts", nil, refusals: [(ask(body: [String: Any](), given: ["manager on"]), "no pair given")],
              note: "the request and answer are frontdesk/contacts.json's"),
        Route("POST", "/setup/gm/registry", nil, refusals: [(ask(body: [String: Any](), given: ["manager on"]), "no hire named")],
              note: "the request and answer are frontdesk/registry-edit.json's"),
        Route("POST", "/setup/gm/nickname", nil, refusals: [(ask(body: [String: Any](), given: ["manager on"]), "no project or name")]),
        Route("GET", "/setup/gm/registry", nil, refusals: [(ask(given: ["manager on"]), "listed for the page, but nothing answers it")]),
        Route("POST", "/setup/gm/by-hand", ask(query: ["enabled": "0"], given: ["manager on"]),
              refusals: [(ask(given: ["manager on"]), "neither enabled=1 nor enabled=0")]),
        Route("POST", "/setup/gm/token/rotate", ask(given: ["manager on"])),
        Route("GET", "/setup/gm/nothing-here", nil, refusals: [(ask(given: ["manager on"]), "no such route"),
                                                               (ask("tool", given: ["manager on"]), "a tool, on a route not listed for it")],
              note: "any path under /setup/gm/ that isn't a route (INVENTORY: * /setup/gm/*)"),
    ]

    /// The office's routes (a paired phone's), the hook's, a cloud bot's, and pairing.
    static let office: [Route] = [
        Route("POST", "/hook", ask("hook", body: hooks[0]["body"]!, headers: hooks[0]["headers"] as! [String: String]),
              refusals: [(ask("nobody", body: [String: Any]()), "no hook key")],
              note: "the agent's hook event, with its agent, tab, project, pid and parent headers; it feeds the snapshot"),
        Route("POST", "/pair", ask("nobody", body: ["code": "$code"], given: ["a pairing code"]),
              refusals: [(ask("nobody", body: ["code": "1"]), "a code that isn't the one asked for")],
              note: "\"$code\" stands for the code the given asked for"),
        Route("POST", "/focus", nil, refusals: [(ask("phone", query: ["session": "no-such-session"]), "no such session")]),
        Route("POST", "/act", nil, refusals: [
            (ask("phone", query: ["action": "accept", "session": "no-such-session"]), "no such session"),
            (ask("phone", query: ["action": "hire", "floor": "9"]), "no such floor"),
            (ask("phone", query: ["action": "settings"]), "settings open on the computer itself, not a phone"),
            (ask("phone", query: ["action": "seen", "session": "no-such-session"]), "no such cloud bot"),
            (ask("phone", query: ["action": "language", "lang": "xx"]), "unknown dictation language"),
            (ask("nobody", query: ["action": "accept"]), "no phone's key"),
        ]),
        Route("POST", "/summary", nil, refusals: [(ask("phone", query: ["session": "no-such-session"]), "summaries are off")],
              note: "{summary} or {error} with the status"),
        Route("GET", "/glossary", ask("phone", given: ["sessions"])),
        Route("GET", "/approvals", nil, refusals: [(ask("phone"), "over Tailscale's HTTPS address only")]),
        Route("GET", "/reviews", nil, refusals: [(ask("phone"), "over Tailscale's HTTPS address only")]),
        Route("GET", "/conversation", nil, refusals: [(ask("phone"), "over Tailscale's HTTPS address only")]),
        Route("GET", "/session/info", nil, refusals: [(ask("phone"), "over Tailscale's HTTPS address only")]),
        Route("GET", "/notifications", ask("phone")),
        Route("POST", "/notifications", nil, refusals: [(ask("phone", body: ["token": "not-a-token"]), "over Tailscale's HTTPS address only")]),
        Route("POST", "/dictate", nil, refusals: [(ask("phone", body: ["text": "!ls"]), "a Wi-Fi phone too (I34): text starting with “!”"),
                                                  (ask("phone", query: ["session": "nope"], body: ["text": "one\ntwo"]), "several lines, to no such session"),
                                                  (ask("phone", body: ["text": "one line"], given: ["the phone mic off"]), "the phone mic off in setup")]),
        Route("POST", "/peek", nil, refusals: [(ask("phone"), "over Tailscale's HTTPS address only")]),
        Route("POST", "/choose", nil, refusals: [(ask("phone"), "a Wi-Fi phone too (I34): no option=other")]),
        Route("GET", "/choice", nil, refusals: [(ask("phone"), "over Tailscale's HTTPS address only")]),
        Route("GET", "/t3code", nil, refusals: [(ask("phone"), "the office's own window only, never a phone")]),
        Route("POST", "/t3code/open", nil, refusals: [(ask("phone"), "the office's own window only, never a phone")]),
        Route("GET", "/hire/window", nil, refusals: [(ask("phone"), "the office's own window only, never a phone")]),
        Route("POST", "/hire/window", nil, refusals: [(ask("phone"), "the office's own window only, never a phone")]),
        Route("POST", "/bots/no-such-bot/event", nil, refusals: [
            (ask("tool", body: ["type": "nope"]), "not an event a bot has"),
            (ask("tool", body: ["type": "seen"]), "no such bot"),
            (ask("nobody", body: ["type": "seen"]), "no setup key"),
        ]),
        Route("GET", "/nothing-here", nil, refusals: [(ask("phone"), "no such route")]),
    ]

    /// The groups, each written as its own file.
    static let groups: [(file: String, routes: [Route])] = [
        ("state.json", state), ("setup.json", setup), ("gm.json", gm), ("office.json", office),
    ]

    // MARK: What's given

    private static func hook(_ session: String, tab: String, project: String, _ body: [String: Any]) -> [String: Any] {
        ["headers": ["x-agentdeck-agent": "claude", "x-agentdeck-tab": tab, "x-agentdeck-project": project],
         "body": body.merging(["session_id": session, "cwd": project]) { a, _ in a }]
    }

    /// "sessions": two Claude Code sessions as their hooks report them. contract-a, with its model, a subagent and
    /// a role, works; contract-b asks you. contract-a's tab is a launcher's, the tab a hire with a role opens.
    static let hooks: [[String: Any]] = [
        hook("contract-a", tab: "launcher:contract-a", project: "/work/alpha",
             ["hook_event_name": "SessionStart", "source": "startup", "model": "claude-opus-5-5"]),
        hook("contract-a", tab: "launcher:contract-a", project: "/work/alpha",
             ["hook_event_name": "UserPromptSubmit", "prompt": "Write the tests."]),
        hook("contract-a", tab: "launcher:contract-a", project: "/work/alpha",
             ["hook_event_name": "SubagentStart", "agent_id": "contract-sub", "agent_type": "Explore"]),
        hook("contract-a", tab: "launcher:contract-a", project: "/work/alpha",
             ["hook_event_name": "PreToolUse", "tool_name": "Read", "tool_input": ["file_path": "/work/alpha/README.md"]]),
        hook("contract-b", tab: "tty:/dev/ttys902", project: "/work/beta",
             ["hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": ["command": "ls"]]),
        hook("contract-b", tab: "tty:/dev/ttys902", project: "/work/beta",
             ["hook_event_name": "Notification", "message": "Claude needs your permission to use Bash"]),
    ]

    /// "a session at a free key": contract-c, just asked something, with no model, tool or role said yet. It takes
    /// a key no folder is pinned to, so the snapshot has a key with no pin beside the pinned one.
    static let freeKeyHooks: [[String: Any]] = [
        hook("contract-c", tab: "tty:/dev/ttys903", project: "/work/gamma",
             ["hook_event_name": "UserPromptSubmit", "prompt": "Look at the logs."]),
    ]

    /// The role contract-a's tab was opened with.
    static let hiredRole = "reviewer"

    private static func step(_ method: String, _ path: String, query: [String: String] = [:], body: Any? = nil) -> [String: Any] {
        var step: [String: Any] = ["as": "page", "method": method, "path": path]
        if !query.isEmpty { step["query"] = query }
        if let body { step["body"] = body }
        return step
    }

    /// Each `given`: the requests it is (both apps answer them, each under 300), a file put in the support folder
    /// before the app reads it, or what each app does its own way (`says`). Written to contract.json as they are.
    static let givens: [String: [String: Any]] = [
        "manager on": ["says": "config.json's managerEnabled is true"],
        "T3 Code on": ["says": "config.json's t3CodeEnabled is true (the T3 Code choice, T796)"],
        "the phone mic off": ["says": "config.json's phoneMicEnabled is false (setup's Phone microphone, personal build): "
            + "the Go app has no phone mic (M6)"],
        "sessions": [
            "requests": hooks.map { hook -> [String: Any] in
                ["as": "hook", "method": "POST", "path": "/hook", "headers": hook["headers"]!, "body": hook["body"]!]
            },
            "says": "before its hooks, contract-a's tab (launcher:contract-a) is a hire's, opened with the role \(hiredRole): "
                + "the Mac keeps that in Deck.hiredTabs, the Go app in its launcher (launcherConnected)",
        ],
        "a pin": ["requests": [step("POST", "/setup/pin", query: ["key": "0", "folder": "/work/alpha"])]],
        "a session at a free key": ["requests": freeKeyHooks.map { hook -> [String: Any] in
            ["as": "hook", "method": "POST", "path": "/hook", "headers": hook["headers"]!, "body": hook["body"]!]
        }],
        "a saved profile": ["requests": [step("POST", "/setup/profiles", query: ["action": "save", "name": "Contract"])]],
        // contract-b on 2F (floor 1: the ground floor, the keys, is 0), with the cloud bot: upstairs.
        "a floor rule": ["requests": [step("POST", "/setup/floors", body: ["rules": [["folder": "/work/beta", "floor": 1, "exact": false]]])]],
        "a cloud bot": ["requests": [step("POST", "/setup/bots", body: ["bots": [["id": "contract-bot", "name": "Contract Bot"]]])]],
        "a watch": ["requests": [step("POST", "/setup/gm/watch", body: ["kind": "time", "target": "check the build", "minutes": 30])]],
        // A hire in T3 Code (T3D-a): its desk comes from the registry alone, at a free key, with every control off.
        "a T3 hire": ["file": "registry.json", "content": [
            "version": 1,
            "hires": [
                ["id": t3HireID, "name": "Reviewer", "nickname": "Rev", "role": hiredRole,
                 "folder": "/work/delta", "branch": "hire/rev-c0ffee", "worktree": "/work/delta-hires/rev-c0ffee", "repo": "/work/delta",
                 "tmuxSocket": "", "account": "", "model": "claude-sonnet-5-5", "ports": "", "brief": "", "started": 1_761_000_000_000,
                 "state": "working", "notes": "", "engine": "t3", "t3Thread": "mcp:contract-t3",
                 "t3Link": "https://app.t3.codes/threads/env-1/mcp%3Acontract-t3", "t3Floor": 0],
            ],
        ] as [String: Any], "says": "registry.json holds a hire in T3 Code (engine t3, with its thread and worktree): the Mac gives it "
            + "a desk at a free key, from the registry alone, with no control (T3D-a); T3 Code isn't asked"],
        "an outbox message": ["file": "outbox.json", "content": [
            "version": 1,
            "messages": [
                ["id": "gm-001", "from": "gm", "to": "abcdef123456", "toName": "Rev", "at": 1_761_000_000_000, "state": "delivered",
                 "size": 38, "attempts": 0, "nextAt": 0, "note": "", "text": "Please look at the login change first."],
                ["id": "h-001", "from": "hire", "fromID": "abcdef123456", "fromName": "Rev", "to": "gm", "toName": "the GM",
                 "at": 1_761_000_240_000, "state": "delivered", "size": 12, "attempts": 0, "nextAt": 0,
                 "note": "Kept for the GM to read; not typed into its session.", "text": "Review done."],
            ],
        ] as [String: Any]],
        "the app's briefs": ["says": "the library you edit (the support folder's briefs/templates) is the app's own, Resources/briefs: "
            + "the Go app has it built in and copies it there on first use; the Mac's test bundle has none, so its test copies it there"],
        "a pairing code": ["requests": [step("POST", "/setup/pair")]],
        "a paired phone": ["says": "a phone paired with a code, as the pairing page does it (POST /pair); phone requests carry its key "
            + "as ?t=. A code open before is open after"],
    ]

    // MARK: Shapes

    /// What's written where the Mac's value alone can't say what the contract is.
    enum Mark {
        /// This type, or null: "string|null", and why: the given leaves it empty everywhere, so no element beside it
        /// shows its type. INVENTORY.md's "Null allowed" lists each.
        case nullable(String, why: String)
        /// One of a closed set of strings (a Swift enum's cases where there is one), or null too.
        case oneOf([String], nullable: Bool = false)
        /// Compared no further than this (any, map: an object whose keys are data, array, object|null…), and why.
        case loose(String, why: String)
        /// An empty array in every given here, and why.
        case empty(String)
    }

    /// The marks, by the shape or route they're in and where. The setup state's are the tool's too.
    static let marks: [String: [String: Mark]] = {
        let agents = Agent.allCases.map(\.rawValue)
        // A session's desk, at a key or upstairs.
        let desk: [String: Mark] = [
            ".state": .oneOf(SessionState.allCases.map(\.rawValue)), ".agent": .oneOf(agents),
            // "" until the agent says.
            ".effort": .oneOf(Deck.effortLevels + [""]),
            ".voiceNote": .loose("object|null", why: "how the last dictation went, for a moment: none in the given (personal build)"),
            ".choice": .loose("object|null", why: "a menu's options, read from the terminal: none in the given"),
            ".last": .loose("object|null", why: "a cloud bot's last report: none in the given"),
            // A hire in T3 Code's desk (T3D-a): its thread's status, as the lane's follow last saw it.
            ".t3.status": .oneOf(T3Desk.Status.allCases.map(\.rawValue)),
        ]
        let layout: [String: Mark] = [
            ".layout.room.floorPlans": .loose("map", why: "a floor's plan by its number: keys are data"),
            ".layout.room.plan.hung": .loose("map", why: "what's hung where: keys are data"),
            ".layout.room.plan.tall": .loose("map", why: "a tall item's place by its id: keys are data"),
            ".layout.room.plan.wide": .loose("map", why: "a wide item's place by its id: keys are data"),
            ".layout.room.team": .loose("map", why: "the team's looks by folder name: keys are data"),
            ".layout.room.dress.rules": .empty("no dress rule in the default layout"),
            ".layout.room.pet": .empty("no pet in the default layout"),
            ".layout.room.plan.added.tall": .empty("nothing added to the default plan"),
            ".layout.room.plan.added.wide": .empty("nothing added to the default plan"),
        ]
        // What no desk in the given has, so no desk beside it says its type. Each other field of a desk is typed
        // by its value, or, where one desk has none, by another's (a model at the pinned key, none at the free one).
        let unsaid: [String: Mark] = [
            ".messaged": .nullable("string", why: "the last text sent to it from the office: none in the given"),
            ".messagedAt": .nullable("number", why: "when text was last sent to it from the office: never in the given"),
            ".errorKind": .nullable("string", why: "why its last turn failed: no turn fails in the given"),
            ".maxPressedAt": .nullable("number", why: "when max effort was last asked for from the office: never in the given"),
        ]
        var snapshot = layout
        for prefix in [".keys[].session", ".floors[].desks[]", ".upstairs.desks[]"] {
            for (path, mark) in desk.merging(unsaid, uniquingKeysWith: { a, _ in a }) { snapshot[prefix + path] = mark }
        }
        snapshot[".keys[].session.subagents[].model"] = .nullable("string", why: "a subagent's model: the given's SubagentStart says none")
        for prefix in [".floors[].desks[]", ".upstairs.desks[]"] {
            snapshot[prefix + ".subagents"] = .empty("only contract-a, at a key, has a subagent in the given")
            snapshot[prefix + ".model"] = .nullable("string", why: "upstairs, contract-b's agent hasn't said its model and a cloud bot has none")
            snapshot[prefix + ".turnStartedAt"] = .nullable("number", why: "upstairs, contract-b was asked nothing (it asks you) and a cloud bot has no turns")
            snapshot[prefix + ".hiredBy.id"] = .nullable("string", why: "the desk of who started it: a cloud bot's hirer has none")
        }
        snapshot[".hiring.roles"] = .empty("no folder for hires in the given, so no role to hire as")
        snapshot[".music"] = .loose("object|null", why: "what's playing: null with music off, as in the given")
        // The saved looks, each with a layout of its own and the colours changed from the theme.
        var setupState = layout
        for (path, mark) in layout { setupState[".profiles[]" + path] = mark }
        setupState.merge([
            ".profiles[].colours": .loose("map", why: "the theme's colours changed in a look, by name: none in the given"),
            ".platform": .oneOf(["darwin", "linux", "windows"]),
            ".logo": .oneOf(Logo.allCases.map(\.rawValue)),
            ".agents": .loose("array", why: "the coding agents found in this computer's home folder"),
            ".hiring.agent": .oneOf(agents),
            ".hiring.terminals": .loose("array", why: "the terminals installed on this computer"),
            ".claudeAccounts.found": .loose("array", why: "the Claude accounts found in this Mac's home folder"),
            ".claudeAccounts.profiles": .loose("array", why: "the Claude accounts found in this Mac's home folder"),
            ".keystrokes.running": .loose("array", why: "the apps running on this Mac"),
            ".keystrokes.apps": .empty("no app allowed keys by default"),
            ".dictation.problem": .nullable("string", why: "what stops dictation: null when nothing does, as this Mac's speech recognition says"),
            ".dictation.cleanupProblem": .nullable("string", why: "what stops Claude cleaning dictation up: null when nothing does, as in the given"),
            ".phoneMic.tailnet": .nullable("string", why: "this Mac's Tailscale name: none, as Tailscale isn't asked (personal build)"),
            ".phoneMic.serving": .nullable("string", why: "the HTTPS address the phone mic is served on: none without Tailscale (personal build)"),
            ".summary.provider": .loose("object|null", why: "the provider picked: none with summaries off, as in the given"),
            ".summary.key": .loose("object|null", why: "where the API key is from: none in the given (no Keychain is asked)"),
            ".summary.fallback": .nullable("string", why: "the provider used when the one picked can't answer: none with summaries off, as in the given"),
            ".summary.providers[].id": .oneOf(SummaryProvider.allCases.map(\.rawValue)),
            ".summary.providers[].state": .oneOf(SummaryProviderState.Kind.allCases.map(\.rawValue) + ["checking"]),
            ".webhook.last": .loose("object|null", why: "the last webhook sent: none in the given"),
            ".webhook.events[]": .oneOf(Webhook.allEvents),
            ".sessions[].state": .oneOf(SessionState.allCases.map(\.rawValue)),
            ".sessions[].agent": .oneOf(agents),
            ".cloudBots.bots[].agent": .oneOf(agents),
            ".cloudBots.hidden": .empty("no bot hidden in the given"),
            ".cloudBots.noRoom": .empty("every bot has a desk in the given"),
            ".roles.agents[].id": .oneOf(agents),
            ".roles.roles[].agent": .oneOf(agents),
            ".roles.shipped[].agent": .oneOf(agents),
        ]) { a, _ in a }
        return [
            "setup-state": setupState,
            "snapshot": snapshot,
            "GET /setup/claude-accounts": [
                ".found": .loose("array", why: "the Claude accounts found in this Mac's home folder"),
                ".profiles": .loose("array", why: "the Claude accounts found in this Mac's home folder"),
            ],
            "GET /setup/guard": [".extra": .empty("no pattern of your own in the given")],
            "GET /setup/bots": [
                ".bots[].agent": .oneOf(agents),
                ".bots[].resolved": .loose("object|null", why: "only a bot from the Grok Bot app is resolved: none in the given"),
            ],
            "POST /setup/bots": [
                ".bots[].agent": .oneOf(agents),
                ".bots[].resolved": .loose("object|null", why: "only a bot from the Grok Bot app is resolved: none in the given"),
            ],
            "POST /setup/bots/resolve": [
                ".bots[].agent": .oneOf(agents),
                ".bots[].resolved": .loose("object|null", why: "only a bot from the Grok Bot app is resolved: none in the given"),
                ".filled": .empty("only a bot from the Grok Bot app is filled in: none in the given"),
                ".unresolved": .empty("only a bot from the Grok Bot app can be unresolved: none in the given"),
            ],
            "GET /setup/hire/folders": [".recent": .empty("no folder hired into in the given")],
            "GET /setup/gm/team": [
                ".contacts": .empty("no pair of hires allowed to write to each other in the given"),
                ".team[].started": .nullable("number", why: "when its hire was started: null for a session no hire matches, as both in the given"),
                ".team[].agent": .oneOf(agents),
                ".team[].liveState": .oneOf(SessionState.allCases.map(\.rawValue)),
                // "" for a session that isn't a hire.
                ".team[].state": .oneOf(FrontDesk.State.allCases.map(\.rawValue) + [""]),
            ],
            "GET /setup/gm/events": [".events[].kind": .oneOf(EventLog.Kind.allCases.map(\.rawValue))],
            "GET /setup/gm/approvals": [".approvals[].agent": .oneOf(agents)],
            "GET /setup/gm/outbox": [
                ".messages[].state": .oneOf(MailRouter.State.allCases.map(\.rawValue)),
                ".messages[].from": .oneOf(MailRouter.Sender.allCases.map(\.rawValue)),
            ],
            "GET /setup/gm/watches": [".watches[].kind": .oneOf(Watchers.Kind.allCases.map(\.rawValue))],
            "GET /setup/gm/screens": [".screens": .loose("array", why: "this Mac's screens")],
        ]
    }()

    /// The mark at `path` in `subject`: the setup state's for the tool's too.
    static func mark(_ subject: String, _ path: String) -> Mark? {
        marks[subject == "setup-state-tool" ? "setup-state" : subject]?[path]
    }

    /// A spot a shape doesn't compare all the way down ("Not compared"), or where it allows a null ("Null allowed"):
    /// the INVENTORY.md table it's listed in, in what, where, what's written, and why.
    struct Loose: Hashable { let table, subject, path, shape, why: String }

    /// The shape written for a value of `subject` (a shape's name or a route) at `path`:
    /// - "string", "number", "bool", "null" for a leaf; 'x' for that string; alternatives as "string|null";
    ///   "any", "object" (any object), "map" (an object whose keys are data) and "array" (any array) compare no further;
    /// - an object's keys, each with its shape: no more, no fewer;
    /// - an array's elements, as the different shapes they have: [] for none. Each element must be like one of
    ///   them, and each like some element.
    /// A null, an empty array or an empty object needs a mark (or, in an array's element, another element with
    /// something there), so none stands for anything by accident: `problems` says where one has none.
    static func shape(_ value: Any, in subject: String, at path: String = "", loose: inout Set<Loose>, problems: inout [String]) -> Any {
        if let mark = mark(subject, path) {
            let written: Any
            switch mark {
            case .nullable(let type, let why):
                let spec = "\(type)|null"
                written = spec
                loose.insert(Loose(table: "Null allowed", subject: subject == "setup-state-tool" ? "setup-state" : subject, path: path,
                                   shape: spec, why: why))
            case .oneOf(let values, let nullable): written = values.map { "'\($0)'" }.joined(separator: "|") + (nullable ? "|null" : "")
            case .loose(let spec, let why):
                written = spec
                loose.insert(Loose(table: "Not compared", subject: subject == "setup-state-tool" ? "setup-state" : subject, path: path, shape: spec, why: why))
            case .empty(let why):
                written = [Any]()
                loose.insert(Loose(table: "Not compared", subject: subject == "setup-state-tool" ? "setup-state" : subject, path: path, shape: "[]", why: why))
            }
            for difference in differences(written, value, at: path) { problems.append("\(subject) \(difference): the Mac's own value, against its mark") }
            return written
        }
        switch value {
        case is NSNull: return "null"
        case is String: return "string"
        case let number as NSNumber: return CFGetTypeID(number) == CFBooleanGetTypeID() ? "bool" : "number"
        case let array as [Any]:
            var variants: [Any] = []
            for element in array {
                let one = shape(element, in: subject, at: path + "[]", loose: &loose, problems: &problems)
                if !variants.contains(where: { same($0, one) }) { variants.append(one) }
            }
            return variants
        case let object as [String: Any]:
            if object.isEmpty { problems.append("\(subject) \(path.isEmpty ? "." : path): an empty object with no mark (a map? empty in every given?)") }
            return object.reduce(into: [String: Any]()) { shape, item in
                shape[item.key] = Self.shape(item.value, in: subject, at: path + "." + item.key, loose: &loose, problems: &problems)
            }
        default: return "any"
        }
    }

    /// A shape settled: a null leaf where another of an array's elements has a leaf type there becomes that type
    /// or null; one where another has an object or an array stays that element's null. A null or an empty array
    /// left with nothing beside it is a problem: it needs a mark. Then the elements are sorted, for a stable file.
    static func settled(_ shape: Any, in subject: String, problems: inout [String]) -> Any {
        var seen: [String: Set<String>] = [:]   // where: the kinds found there ("leaf:string", "array", "object", "null", "[]")
        func look(_ shape: Any, _ path: String) {
            switch shape {
            case let leaf as String: seen[path, default: []].insert(leaf == "null" ? "null" : "leaf:" + leaf)
            case let object as [String: Any]: seen[path, default: []].insert("object"); for (key, value) in object { look(value, path + "." + key) }
            case let array as [Any]:
                seen[path, default: []].insert(array.isEmpty ? "[]" : "array")
                for element in array { look(element, path + "[]") }
            default: break
            }
        }
        look(shape, "")
        func settle(_ shape: Any, _ path: String) -> Any {
            switch shape {
            case let leaf as String where leaf == "null":
                let types = (seen[path] ?? []).filter { $0.hasPrefix("leaf:") }.map { String($0.dropFirst(5)) }.sorted()
                if types.count == 1, !types[0].contains("null") { return types[0] + "|null" }
                if seen[path]?.contains("object") == true || seen[path]?.contains("array") == true { return "null" }
                problems.append("\(subject) \(path.isEmpty ? "." : path): a null with no mark (its type, or null always?)")
                return leaf
            case let object as [String: Any]:
                return object.reduce(into: [String: Any]()) { $0[$1.key] = settle($1.value, path + "." + $1.key) }
            case let array as [Any]:
                if array.isEmpty, mark(subject, path) == nil, seen[path]?.contains("array") != true {
                    problems.append("\(subject) \(path.isEmpty ? "." : path): an empty array with no mark (empty in every given?)")
                }
                var variants: [Any] = []
                for element in array.map({ settle($0, path + "[]") }) where !variants.contains(where: { same($0, element) }) {
                    variants.append(element)
                }
                return variants.sorted { text($0) < text($1) }
            default: return shape
            }
        }
        return settle(shape, "")
    }

    /// The lists whose elements are the given's sessions, in the named shapes: their ids are written beside the
    /// shape, and compared too. A shape alone passes a list that drops a session another element looks like.
    static let idLists: [String: [String]] = [
        "setup-state": [".sessions[].id"],
        "setup-state-tool": [".sessions[].id"],
        "snapshot": [".keys[].session.id", ".floors[].desks[].id", ".upstairs.desks[].id"],
    ]

    /// The given's hire in T3 Code (`givens["a T3 hire"]`), whose desk is checked by value too (FXT3Da N1).
    static let t3HireID = "c0ffee12-0000-4000-8000-0000000000d3"

    /// A hire in T3 Code's desk, checked by value beside the snapshot's shape: a shape alone passes a desk with no
    /// `t3`, or with its controls on, as like a plain desk's. Written into contract.json, where `desktop/contract_test.go`
    /// checks every `t3:` desk the Go core answers by it, the same way.
    static let t3DeskRule: [String: Any] = [
        "id": T3Desks.sessionID(of: t3HireID),
        "false": T3Desks.controlsOff.sorted(),
        "null": ["choice"],
        "t3": ["branch", "canOpen", "experimental", "status", "tag"],
        "noSlash": ["name", "project"],
    ]

    /// Where a `t3:` desk in `snapshot` (at a key, on a floor, upstairs) isn't as `rule` says, as "where: what" lines:
    /// each `false` field false, each `null` field null, its `t3` with each of those fields, no `/` in a `noSlash`
    /// field. None where there's no such desk: the ids say whether there should be one.
    static func t3DeskDifferences(_ rule: [String: Any], _ snapshot: Any) -> [String] {
        let lists = [".keys[].session", ".floors[].desks[]", ".upstairs.desks[]"]
        var found: [String] = []
        for list in lists {
            for desk in Self.objects(at: list, in: snapshot) {
                guard let id = desk["id"] as? String, id.hasPrefix(T3Desks.prefix) else { continue }
                let here = "\(list)[\(id)]"
                let isFalse = { (value: Any?) in (value as? NSNumber).map { CFGetTypeID($0) == CFBooleanGetTypeID() && !$0.boolValue } ?? false }
                for field in rule["false"] as? [String] ?? [] where !isFalse(desk[field]) {
                    found.append("\(here).\(field): \(desk[field].map { "\($0)" } ?? "missing"), want false")
                }
                for field in rule["null"] as? [String] ?? [] where !(desk[field] is NSNull) {
                    found.append("\(here).\(field): \(desk[field].map { "\($0)" } ?? "missing"), want null")
                }
                let t3 = desk["t3"] as? [String: Any]
                for field in rule["t3"] as? [String] ?? [] where t3?[field] == nil || t3?[field] is NSNull {
                    found.append("\(here).t3.\(field): missing, want a value")
                }
                for field in rule["noSlash"] as? [String] ?? [] where (desk[field] as? String)?.contains("/") != false {
                    found.append("\(here).\(field): \(desk[field].map { "\($0)" } ?? "missing"), want a name with no /")
                }
            }
        }
        return found
    }

    /// The objects at `path` (".a[].b"), every element's.
    static func objects(at path: String, in json: Any) -> [[String: Any]] {
        var found = [json]
        var rest = Substring(path)
        while !rest.isEmpty {
            if rest.hasPrefix("[]") {
                found = found.flatMap { $0 as? [Any] ?? [] }
                rest = rest.dropFirst(2)
                continue
            }
            rest = rest.dropFirst()
            let key = rest.prefix { $0 != "." && $0 != "[" }
            rest = rest.dropFirst(key.count)
            found = found.compactMap { ($0 as? [String: Any])?[String(key)] }
        }
        return found.compactMap { $0 as? [String: Any] }
    }

    /// The values at `path` (".a[].b"), every element's, as text, sorted.
    static func values(at path: String, in json: Any) -> [String] {
        var found = [json]
        var rest = Substring(path)
        while !rest.isEmpty {
            if rest.hasPrefix("[]") {
                found = found.flatMap { $0 as? [Any] ?? [] }
                rest = rest.dropFirst(2)
                continue
            }
            rest = rest.dropFirst()
            let key = rest.prefix { $0 != "." && $0 != "[" }
            rest = rest.dropFirst(key.count)
            found = found.compactMap { ($0 as? [String: Any])?[String(key)] }
        }
        return found.map { "\($0)" }.sorted()
    }

    /// The shape written for an answer: made, then settled, with every problem found.
    static func written(_ value: Any, in subject: String, loose: inout Set<Loose>, problems: inout [String]) -> Any {
        settled(shape(value, in: subject, loose: &loose, problems: &problems), in: subject, problems: &problems)
    }

    static func text(_ shape: Any) -> String {
        guard JSONSerialization.isValidJSONObject([shape]),
              let data = try? JSONSerialization.data(withJSONObject: [shape], options: [.sortedKeys]) else { return "\(shape)" }
        return String(decoding: data, as: UTF8.self)
    }

    static func same(_ a: Any, _ b: Any) -> Bool { text(a) == text(b) }

    // MARK: Comparing

    /// Where `got` (an answer, as JSON) isn't as `want` (a shape) says, as "where: what" lines. Nothing stands for
    /// anything but "any", "object", "map" and "array": a null, an empty array or object where the shape has
    /// something else is a difference. `desktop/contract_test.go` compares the same way, line for line.
    static func differences(_ want: Any, _ got: Any, at path: String = "") -> [String] {
        let here = path.isEmpty ? "." : path
        switch want {
        case let spec as String:
            let alternatives = spec.split(separator: "|").map(String.init)
            if alternatives.contains(where: { matches($0, got) }) { return [] }
            return ["\(here): \(describe(got, literal: alternatives.contains { $0.hasPrefix("'") })), want \(spec)"]
        case let want as [String: Any]:
            guard let got = got as? [String: Any] else { return ["\(here): \(describe(got, literal: false)), want object"] }
            var found: [String] = []
            for key in Set(want.keys).union(got.keys).sorted() {
                switch (want[key], got[key]) {
                case let (w?, g?): found += differences(w, g, at: path + "." + key)
                case (_?, nil): found.append("\(path).\(key): missing")
                default: found.append("\(path).\(key): extra")
                }
            }
            return found
        case let variants as [Any]:
            guard let elements = got as? [Any] else { return ["\(here): \(describe(got, literal: false)), want array"] }
            if variants.isEmpty { return elements.isEmpty ? [] : ["\(here): some, want none"] }
            if elements.isEmpty { return ["\(here): none, want some"] }
            var found: [String] = []
            let add = { (lines: [String]) in for line in lines where !found.contains(line) { found.append(line) } }
            // Each element like one of the Mac's; then, if each is, each of the Mac's like one of them. Where none
            // is, the closest says how: an array empty on one side only counts as far.
            let far = { (lines: [String]) in
                lines.reduce(0) { $0 + ($1.hasSuffix(": some, want none") || $1.hasSuffix(": none, want some") ? 10 : 1) }
            }
            for element in elements {
                let each = variants.map { differences($0, element, at: path + "[]") }
                if !each.contains(where: \.isEmpty) { add(each.min { far($0) < far($1) }!) }
            }
            if found.isEmpty {
                for variant in variants {
                    let each = elements.map { differences(variant, $0, at: path + "[]") }
                    if !each.contains(where: \.isEmpty) { add(each.min { far($0) < far($1) }!) }
                }
            }
            return found
        default:
            return ["\(here): a shape this test doesn't know"]
        }
    }

    private static func matches(_ alternative: String, _ got: Any) -> Bool {
        switch alternative {
        case "any": return true
        case "null": return got is NSNull
        case "string": return got is String
        case "bool": return (got as? NSNumber).map { CFGetTypeID($0) == CFBooleanGetTypeID() } ?? false
        case "number": return (got as? NSNumber).map { CFGetTypeID($0) != CFBooleanGetTypeID() } ?? false
        case "object", "map": return got is [String: Any]
        case "array": return got is [Any]
        default:
            guard alternative.count >= 2, alternative.hasPrefix("'"), alternative.hasSuffix("'") else { return false }
            return got as? String == String(alternative.dropFirst().dropLast())
        }
    }

    /// What a value is, said in a difference: its type, the string itself among strings of a closed set, {} and [] when empty.
    private static func describe(_ got: Any, literal: Bool) -> String {
        switch got {
        case is NSNull: return "null"
        case let text as String: return literal ? "'\(text)'" : "string"
        case let number as NSNumber: return CFGetTypeID(number) == CFBooleanGetTypeID() ? "bool" : "number"
        case let object as [String: Any]: return object.isEmpty ? "{}" : "object"
        case let array as [Any]: return array.isEmpty ? "[]" : "array"
        default: return "something else"
        }
    }

    // MARK: Asking the Mac

    private struct Answer {
        let status: Int
        let kind: String
        let json: Any?
    }

    /// Sets up what an ask is given, in order.
    private func prepare(_ deck: Deck, _ router: DeckRouter, _ given: [String]) throws {
        for name in given {
            let given = try XCTUnwrap(Self.givens[name], "nothing gives \(name)")
            switch name {
            case "manager on": Config.shared.managerEnabled = true
            case "T3 Code on": Config.shared.t3CodeEnabled = true
            case "the phone mic off": Config.shared.phoneMicEnabled = false
            case "a paired phone": _ = try phoneKey(deck)
            case "the app's briefs": try copyAppBriefs()
            case "sessions": deck.hiredTabs.append((tab: "launcher:contract-a", role: Self.hiredRole, desk: nil, at: Date(),
                                                     folder: "/work/alpha", floor: nil))
            default: break
            }
            if let file = given["file"] as? String, let content = given["content"] {
                let url = Paths.support.appendingPathComponent(file)
                let kept = try? Data(contentsOf: url)
                try Self.data(content).write(to: url)
                putBack.append { if let kept { try? kept.write(to: url) } else { try? FileManager.default.removeItem(at: url) } }
            }
            for step in given["requests"] as? [[String: Any]] ?? [] {
                let answer = try send(router, deck, method: step["method"] as! String, path: step["path"] as! String,
                                      caller: step["as"] as! String, query: step["query"] as? [String: String] ?? [:],
                                      headers: step["headers"] as? [String: String] ?? [:], body: step["body"])
                XCTAssertLessThan(answer.status, 300, "\(name): \(step["method"]!) \(step["path"]!)")
            }
        }
    }

    /// The app's own briefs, from this repo, in the support folder's library you edit: the test bundle has none.
    private func copyAppBriefs() throws {
        let source = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("Resources/briefs")
        try FileManager.default.createDirectory(at: Paths.briefTemplates, withIntermediateDirectories: true)
        for name in BriefLibrary.fileNames {
            let url = Paths.briefTemplates.appendingPathComponent(name)
            let kept = try? Data(contentsOf: url)
            try Data(contentsOf: source.appendingPathComponent(name)).write(to: url)
            putBack.append { if let kept { try? kept.write(to: url) } else { try? FileManager.default.removeItem(at: url) } }
        }
    }

    /// What a given put in the support folder, put back after each ask.
    private var putBack: [() -> Void] = []
    private var phoneKeys: [ObjectIdentifier: String] = [:]

    /// A phone paired with this deck, as the pairing page does it.
    private func phoneKey(_ deck: Deck) throws -> String {
        if let key = phoneKeys[ObjectIdentifier(deck)] { return key }
        let code = deck.access.activeCode
        let key = try XCTUnwrap(deck.access.redeem(deck.access.newCode(), from: "192.168.1.20", userAgent: "Contract iPhone"))
        if code != nil { deck.access.newCode() }
        phoneKeys[ObjectIdentifier(deck)] = key
        return key
    }

    private func send(_ router: DeckRouter, _ deck: Deck, method: String, path: String, caller: String, query: [String: String] = [:],
                      headers given: [String: String] = [:], body: Any? = nil) throws -> Answer {
        var headers = given
        var local = true
        headers["host"] = "127.0.0.1:7710"
        if method != "GET" { headers[DeckRouter.setupHeader] = "1" }
        var query = query
        switch caller {
        case "page":
            let sessions = deck.access.setupSessions
            headers["cookie"] = "\(DeckRouter.setupCookie)=\(try XCTUnwrap(sessions.open(link: sessions.newLink())))"
        case "tool": headers[DeckRouter.setupKeyHeader] = deck.access.setupKey
        case "hook": headers["x-agentdeck-token"] = deck.access.hookKey
        case "phone":
            headers["host"] = "192.168.1.10:7710"
            local = false
            query["t"] = try phoneKey(deck)
        case "nobody":
            if !DeckRouter.isSetupPath(path) && !path.hasPrefix("/bots/") && path != "/hook" {
                headers["host"] = "192.168.1.10:7710"
                local = false
            }
        default: XCTFail("no caller \(caller)")
        }
        var body = body
        // "$code" is the pairing code asked for.
        if var object = body as? [String: Any], object["code"] as? String == "$code" {
            object["code"] = try XCTUnwrap(deck.access.activeCode)
            body = object
        }
        let data: Data
        switch body {
        case nil: data = Data()
        case let text as String: data = Data(text.utf8)
        case let some?: data = try JSONSerialization.data(withJSONObject: some)
        }
        let request = Request.addressed(method: method, path: path, query: query, headers: headers, body: data,
                                        fromLoopback: local, remoteAddress: local ? "127.0.0.1" : "192.168.1.20")
        var got: Reply?
        let answered = expectation(description: "\(method) \(path)")
        router.handle(request) { reply in
            got = reply
            answered.fulfill()
        }
        wait(for: [answered], timeout: 10)
        switch try XCTUnwrap(got) {
        case .eventStream(let attach):
            // The stream's first message, as the phone reads it: "data: " and the snapshot.
            var first: String?
            let stream = EventStream { text in if first == nil { first = text } }
            attach(stream)
            stream.close()
            let line = try XCTUnwrap(first, "the event stream sent nothing").split(separator: "\n").first.map(String.init) ?? ""
            let json = line.hasPrefix("data: ") ? try JSONSerialization.jsonObject(with: Data(line.dropFirst(6).utf8)) : nil
            return Answer(status: 200, kind: "event-stream", json: json)
        case .response(let response):
            let json = response.contentType == "application/json" ? try? JSONSerialization.jsonObject(with: response.body) : nil
            let kind = json != nil ? "json" : response.body.isEmpty ? "empty"
                : response.contentType.hasPrefix("image/png") ? "png" : response.contentType.hasPrefix("text/html") ? "html" : "text"
            return Answer(status: response.status, kind: kind, json: json)
        }
    }

    /// One request on a deck of its own, from the config as it comes, with what it's given set up first.
    private func answer(_ route: Route, _ ask: Ask) throws -> Answer {
        for reset in resets { reset() }
        // Its layout reads Config.shared, as the app's does: the pin, the floor rule and the bot given are in it.
        let deck = Deck(quiet: true)
        let router = DeckRouter(deck: deck)
        defer {
            for back in putBack { back() }
            putBack = []
        }
        try prepare(deck, router, ask.given)
        return try send(router, deck, method: route.method, path: route.path, caller: ask.caller, query: ask.query,
                        headers: ask.headers, body: ask.body)
    }

    // MARK: The files

    static let folder = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
        .appendingPathComponent("fixtures/contract", isDirectory: true)

    /// Every file, as the Mac answers now: contract.json (the named shapes, what each given is) and each group's.
    /// `loose` gets every spot that isn't compared all the way down; `problems`, every null or empty with no mark.
    private func files(loose: inout Set<Loose>, problems: inout [String]) throws -> [(String, [String: Any])] {
        var named: [String: [String: Any]] = [:]
        for (name, route) in [("setup-state", Self.state[0]), ("setup-state-tool", Self.state[1]), ("snapshot", Self.state[2])] {
            let got = try answer(route, try XCTUnwrap(route.ask))
            let json = try XCTUnwrap(got.json, name)
            named[name] = ["route": route.name, "as": route.ask!.caller,
                           "shape": Self.written(json, in: name, loose: &loose, problems: &problems)]
            if let paths = Self.idLists[name] {
                named[name]?["ids"] = paths.reduce(into: [String: Any]()) { $0[$1] = Self.values(at: $1, in: json) }
            }
            if name == "snapshot" {
                XCTAssertEqual(Self.t3DeskDifferences(Self.t3DeskRule, json), [], "the Mac's own T3 desk")
                named[name]?["t3Desk"] = Self.t3DeskRule
            }
        }
        let shared: [String: Any] = [
            "note": "Written by ContractTests.swift from the Mac's answers (SMOLCORP_WRITE_FIXTURES=1), read by desktop/contract_test.go. "
                + "Shapes, not values: \"string\", \"number\", \"bool\", \"null\"; 'x' for that string; alternatives joined by | "
                + "(\"string|null\", a closed set 'a'|'b'); an object's keys, no more, no fewer; an array's elements as the "
                + "different shapes they have, each element like one of them and each like some element ([] for none). "
                + "\"any\", \"object\", \"map\" (keys are data) and \"array\" compare no further: INVENTORY.md lists each, with why, "
                + "and each null allowed. Beside a shape, ids: the given's sessions in each of its lists, sorted, compared as they are; "
                + "and t3Desk: what every t3: desk must hold by value (each 'false' field false, each 'null' field null, its t3 with "
                + "each of those fields, no / in a 'noSlash' field). "
                + "A refusal answered in JSON has its shape too.",
            "shapes": named,
            "given": Self.givens,
        ]
        var files: [(String, [String: Any])] = [("contract.json", shared)]
        for (file, routes) in Self.groups {
            files.append((file, try group(routes, named: named, loose: &loose, problems: &problems)))
        }
        return files
    }

    /// A group's file: each route, who may call it, and its answers as the Mac gives them.
    private func group(_ routes: [Route], named: [String: [String: Any]], loose: inout Set<Loose>, problems: inout [String]) throws -> [String: Any] {
        var written: [[String: Any]] = []
        for route in routes {
            var row: [String: Any] = ["route": route.name, "callers": route.callers]
            if let ask = route.ask {
                let got = try answer(route, ask)
                var answer: [String: Any] = ["status": got.status, "kind": got.kind]
                if let json = got.json {
                    // The setup state, and the snapshot, are written by name: each must be the named one whole.
                    let isState = (json as? [String: Any]).map { $0["pairing"] != nil && $0["version"] != nil } ?? false
                    let name = route.path == "/events" ? "snapshot" : isState ? (ask.caller == "tool" ? "setup-state-tool" : "setup-state") : nil
                    let shape = Self.written(json, in: name ?? route.name, loose: &loose, problems: &problems)
                    if let name {
                        let want = try XCTUnwrap(named[name]?["shape"])
                        if !Self.same(want, shape) {
                            problems.append("\(route.name) answers \(name), but not as \(named[name]?["route"] ?? "") does: "
                                + (Self.differences(want, json) + Self.differences(shape, json)).joined(separator: "; "))
                        }
                        answer["shape"] = name
                    } else {
                        answer["shape"] = shape
                    }
                }
                row["request"] = ask.json
                row["answer"] = answer
            }
            let refusals = route.refusals + [Self.strangerRefusal(route)].compactMap { $0 }
            if !refusals.isEmpty {
                row["refusals"] = try refusals.map { refusal -> [String: Any] in
                    let got = try answer(route, refusal.ask)
                    XCTAssertGreaterThanOrEqual(got.status, 400, "\(route.name): \(refusal.why)")
                    var written: [String: Any] = ["request": refusal.ask.json, "status": got.status, "kind": got.kind, "why": refusal.why]
                    // A refusal in JSON is a shape too: what the office reads of it ({error}, {message}…).
                    if let json = got.json {
                        written["shape"] = Self.written(json, in: "\(route.name) refusal", loose: &loose, problems: &problems)
                    }
                    return written
                }
            }
            if let note = route.note { row["note"] = note }
            written.append(row)
        }
        return ["routes": written]
    }

    /// A route outside setup asked by one who isn't its caller, as well: nobody, or a phone where only the office's
    /// own window may call. (Setup's callers are frontdesk/routes.json's, which the Go app's routeTable copies.)
    static func strangerRefusal(_ route: Route) -> (ask: Ask, why: String)? {
        let callers = route.callers
        guard !DeckRouter.isSetupPath(route.path), !route.path.hasPrefix("/bots/"), !callers.contains("anyone") else { return nil }
        let stranger = callers == ["window"] ? "phone" : "nobody"
        guard !route.refusals.contains(where: { $0.ask.caller == stranger }) else { return nil }
        var ask = route.ask ?? route.refusals.first?.ask ?? Ask()
        ask.caller = stranger
        ask.given = []
        return (ask, "not one of its callers")
    }

    private static func data(_ object: Any) throws -> Data {
        try JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted, .sortedKeys])
    }

    // MARK: Tests

    /// What each ask starts from: the config as it comes, whatever the tests before left. Put back after.
    private var resets: [() -> Void] = []

    private func reset<T>(_ path: ReferenceWritableKeyPath<Config, T>) {
        let kept = Config.shared[keyPath: path]
        let fresh = Config()[keyPath: path]
        resets.append { Config.shared[keyPath: path] = fresh }
        addTeardownBlock {
            Config.shared[keyPath: path] = kept
            Config.shared.save()
        }
    }

    override func setUpWithError() throws {
        reset(\.managerEnabled); reset(\.scrollEnabled); reset(\.maxConfirm); reset(\.guideDone); reset(\.phoneMicEnabled)
        reset(\.phoneMicAutoSend); reset(\.officeOnTop); reset(\.logo); reset(\.colours); reset(\.layout); reset(\.pins)
        reset(\.floors); reset(\.floorRules); reset(\.webhook); reset(\.profiles); reset(\.activeProfile); reset(\.hireAgent)
        reset(\.summaryOptIn); reset(\.summaryProvider); reset(\.cloudBots); reset(\.botFloor); reset(\.hireFolder)
        reset(\.hiringEnabled); reset(\.gmByHand); reset(\.roles); reset(\.rolesOffered); reset(\.recentHireFolders)
        reset(\.keystrokesAllowed); reset(\.keystrokeApps); reset(\.showInDock); reset(\.botsAutoDetect); reset(\.hiddenBots)
        reset(\.claudeProfiles); reset(\.dictationEnabled); reset(\.dictationForClaude); reset(\.dictationCleanup)
        reset(\.dictationGlossary); reset(\.hireTerminal); reset(\.hireInTmux); reset(\.musicEnabled); reset(\.teamScreen)
        reset(\.t3CodeEnabled); reset(\.hiresInBackground); reset(\.choicesAsked)
        // Nothing is asked of the Keychain, Tailscale or the coding agents' own commands.
        let key = SummaryKey.store
        SummaryKey.store = .init(read: { nil }, hint: { nil }, write: { _, _ in false }, delete: {})
        SummaryKey.forgetKnown()
        let tailnet = Tailnet.ask
        Tailnet.ask = { nil }
        let providers = SummaryProviders.shared.preset([:])
        addTeardownBlock {
            providers()
            SummaryKey.store = key
            SummaryKey.forgetKnown()
            Tailnet.ask = tailnet
            Tailnet.forget()
        }
    }

    /// Writes the files with SMOLCORP_WRITE_FIXTURES=1; otherwise the Mac must still answer as they say: the
    /// same routes, requests, callers, statuses, kinds and shapes, every null and empty with its mark.
    func testTheMacAnswersAsTheContractSays() throws {
        var loose = Set<Loose>(), problems: [String] = []
        let files = try files(loose: &loose, problems: &problems)
        for problem in problems { XCTFail(problem) }

        if ProcessInfo.processInfo.environment["SMOLCORP_WRITE_FIXTURES"] == "1" {
            try FileManager.default.createDirectory(at: Self.folder, withIntermediateDirectories: true)
            for (file, object) in files { try Self.data(object).write(to: Self.folder.appendingPathComponent(file)) }
        }
        for (file, now) in files {
            let written = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: Self.folder.appendingPathComponent(file))) as? [String: Any],
                                        "\(file): run once with SMOLCORP_WRITE_FIXTURES=1")
            XCTAssertEqual(Self.text(written), Self.text(now), "\(file) is out of date: run once with SMOLCORP_WRITE_FIXTURES=1")
        }
    }

    /// The Mac's own answers pass the comparison the Go app's are put through, and a {} or a null where the
    /// shape has something doesn't.
    func testTheMacsOwnAnswersPassAndEmptyOnesDont() throws {
        for (name, route) in [("setup-state", Self.state[0]), ("snapshot", Self.state[2])] {
            var loose = Set<Loose>(), problems: [String] = []
            let got = try XCTUnwrap(try answer(route, try XCTUnwrap(route.ask)).json)
            let shape = Self.written(got, in: name, loose: &loose, problems: &problems)
            XCTAssertEqual(Self.differences(shape, got), [], name)
            XCTAssertFalse(Self.differences(shape, [String: Any]()).isEmpty, "\(name): {} must not pass")
            XCTAssertFalse(Self.differences(shape, NSNull()).isEmpty, "\(name): null must not pass")
            var nulled = try XCTUnwrap(got as? [String: Any])
            for key in nulled.keys { nulled[key] = NSNull() }
            XCTAssertEqual(Self.differences(shape, nulled).count, nulled.count - Self.nullableTopKeys(shape),
                           "\(name): each field null is a difference, but for those marked so")
        }
    }

    /// The given's T3 desk is held to its values, not only its shape (FXT3Da N1): a `t3:` desk with a control on or
    /// gone, a choice, a `t3` with only its tag, or a path for its project is a difference; the Mac's own has none.
    func testAT3DeskIsCheckedByValueToo() throws {
        let route = Self.state[2]
        let got = try XCTUnwrap(try answer(route, try XCTUnwrap(route.ask)).json as? [String: Any])
        XCTAssertEqual(Self.t3DeskDifferences(Self.t3DeskRule, got), [])
        let id = try XCTUnwrap(Self.t3DeskRule["id"] as? String)
        var keys = try XCTUnwrap(got["keys"] as? [[String: Any]])
        let at = try XCTUnwrap(keys.firstIndex { ($0["session"] as? [String: Any])?["id"] as? String == id }, "the given's T3 desk at a key")
        var desk = try XCTUnwrap(keys[at]["session"] as? [String: Any])
        desk["keys"] = true
        desk.removeValue(forKey: "fire")
        desk["choice"] = ["options": ["Yes"]]
        desk["t3"] = ["tag": T3Desk.tag]
        desk["project"] = "/work/delta-hires/rev-c0ffee"
        keys[at]["session"] = desk
        var doctored = got
        doctored["keys"] = keys
        let wheres = Self.t3DeskDifferences(Self.t3DeskRule, doctored).map { $0.components(separatedBy: ": ")[0] }
        XCTAssertEqual(wheres.sorted(), ["keys", "fire", "choice", "t3.branch", "t3.canOpen", "t3.experimental", "t3.status", "project"]
            .map { ".keys[].session[\(id)].\($0)" }.sorted())
        // Only a `t3:` desk: another with its controls on is a desk like any.
        XCTAssertEqual(Self.t3DeskDifferences(Self.t3DeskRule, ["keys": [["session": ["id": "contract-x", "keys": true, "project": "/a/b"]]]]), [])
    }

    /// A desk's fields are typed by what the given fills: contract-a, at the pinned key, has a model, a tool and an
    /// activity, so a null in any is a difference, though contract-c, at the free key, has no model. And the free
    /// key's pin is null: anything but a string or a null there is a difference too.
    func testWhatTheGivenFillsAtADeskMustBeFilled() throws {
        let route = Self.state[2]
        var loose = Set<Loose>(), problems: [String] = []
        let got = try XCTUnwrap(try answer(route, try XCTUnwrap(route.ask)).json as? [String: Any])
        let shape = Self.written(got, in: "snapshot", loose: &loose, problems: &problems)
        XCTAssertEqual(Self.differences(shape, got), [])
        let keys = try XCTUnwrap(got["keys"] as? [[String: Any]])
        let pinned = try XCTUnwrap(keys.firstIndex { $0["pin"] is String }, "a pinned key in the given")
        // contract-c's: the T3 desk (T3D-a) at another free key has its hire's model.
        let free = try XCTUnwrap(keys.firstIndex { $0["pin"] is NSNull && ($0["session"] as? [String: Any])?["id"] as? String == "contract-c" },
                                 "a session at a free key in the given")
        let freeSession = try XCTUnwrap(keys[free]["session"] as? [String: Any])
        XCTAssertTrue(freeSession["model"] is NSNull, "the free key's desk has no model")
        func changed(_ index: Int, _ change: (inout [String: Any]) -> Void) -> [String: Any] {
            var keys = keys, answer = got
            change(&keys[index])
            answer["keys"] = keys
            return answer
        }
        for field in ["model", "tool", "activity"] {
            let nulled = changed(pinned) { key in
                var session = key["session"] as! [String: Any]
                XCTAssertTrue(session[field] is String, "contract-a has a \(field) in the given")
                session[field] = NSNull()
                key["session"] = session
            }
            XCTAssertFalse(Self.differences(shape, nulled).isEmpty, "a null \(field) at the pinned key must not pass")
        }
        XCTAssertFalse(Self.differences(shape, changed(free) { $0["pin"] = false }).isEmpty, "a pin of false at the free key must not pass")
        XCTAssertFalse(Self.differences(shape, changed(pinned) { $0["pin"] = NSNull() }).isEmpty, "a null pin at the pinned key must not pass")
    }

    private static func nullableTopKeys(_ shape: Any) -> Int {
        (shape as? [String: Any])?.values.filter { ($0 as? String)?.split(separator: "|").contains("null") == true }.count ?? 0
    }

    func testShapesAreKeysTypesAndClosedSets() throws {
        var loose = Set<Loose>(), problems: [String] = []
        let json = try JSONSerialization.jsonObject(with: Data(#"{"a": 1, "b": [true, false], "c": [{"d": "x"}, {"e": 2}], "f": "y"}"#.utf8))
        let shape = Self.written(json, in: "test", loose: &loose, problems: &problems)
        XCTAssertEqual(Self.text(shape), Self.text(["a": "number", "b": ["bool"], "c": [["d": "string"], ["e": "number"]], "f": "string"]))
        XCTAssertEqual(problems, [])
        // A null, an empty array or an empty object needs a mark.
        _ = Self.written(["n": NSNull(), "e": [Any](), "o": [String: Any]()], in: "test", loose: &loose, problems: &problems)
        XCTAssertEqual(problems.count, 3, "\(problems)")
        // Keys, types and closed sets.
        XCTAssertEqual(Self.differences(["a": "number", "b": "string"], ["a": "x", "c": true]), [".a: string, want number", ".b: missing", ".c: extra"])
        XCTAssertEqual(Self.differences(["s": "'idle'|'asking'"], ["s": "not-a-state"]), [".s: 'not-a-state', want 'idle'|'asking'"])
        XCTAssertEqual(Self.differences(["s": "string|null"], ["s": NSNull()]), [])
        // Nothing empty stands for anything.
        XCTAssertEqual(Self.differences(["a": ["string"], "g": "number"], ["a": [Any](), "g": NSNull()]), [".a: none, want some", ".g: null, want number"])
        XCTAssertEqual(Self.differences(["o": ["k": "bool"]], ["o": [String: Any]()]), [".o.k: missing"])
        XCTAssertEqual(Self.differences(["o": ["k": "bool"]], ["o": NSNull()]), [".o: null, want object"])
        XCTAssertEqual(Self.differences([Any](), ["x"]), [".: some, want none"])
        // Each element like one of the Mac's, and each of the Mac's like one of them.
        XCTAssertEqual(Self.differences([["x": "bool"]], [["x": 1]]), ["[].x: number, want bool"])
        XCTAssertEqual(Self.differences([["x": "bool"], "null"], [["x": true]]), ["[]: object, want null"])
        XCTAssertEqual(Self.differences(["map", "any"], [[String: Any](), 3]), [])
    }

    /// INVENTORY.md's "Not compared" table is every spot the shapes don't compare all the way down, and its "Null
    /// allowed" every null marked as allowed, each with why (alternatives said with "or", as a table's cells can't
    /// hold a |).
    func testEveryLooseSpotIsInTheInventory() throws {
        var found = Set<Loose>(), problems: [String] = []
        _ = try files(loose: &found, problems: &problems)
        let loose = Set(found.map {
            Loose(table: $0.table, subject: $0.subject, path: $0.path, shape: $0.shape.replacingOccurrences(of: "|", with: " or "), why: $0.why)
        })
        let inventory = try String(contentsOf: Self.folder.appendingPathComponent("INVENTORY.md"), encoding: .utf8)
        var section = "", listed = Set<Loose>()
        for line in inventory.split(separator: "\n", omittingEmptySubsequences: false) {
            if line.hasPrefix("## ") { section = String(line.dropFirst(3)) }
            let cells = line.split(separator: "|", omittingEmptySubsequences: false).dropFirst().dropLast()
                .map { $0.trimmingCharacters(in: .whitespaces).replacingOccurrences(of: "`", with: "") }
            guard section == "Not compared" || section == "Null allowed", cells.count == 4, cells[0] != "In", !cells[0].hasPrefix("---") else { continue }
            listed.insert(Loose(table: section, subject: cells[0], path: cells[1], shape: cells[2], why: cells[3]))
        }
        for spot in loose.subtracting(listed).sorted(by: { ($0.subject, $0.path) < ($1.subject, $1.path) }) {
            XCTFail("not in INVENTORY.md's \(spot.table): | `\(spot.subject)` | `\(spot.path)` | `\(spot.shape)` | \(spot.why) |")
        }
        for spot in listed.subtracting(loose) { XCTFail("INVENTORY.md's \(spot.table) has a row no shape has: \(spot)") }
    }

    /// Every route the Mac's router answers is a row of INVENTORY.md, so none is left out of the contract: the
    /// (method, path) cases, the paths matched as `request.path == ` and by `hasPrefix`, and the sounds.
    func testEveryRouteTheMacAnswersIsInTheInventory() throws {
        let inventory = try String(contentsOf: Self.folder.appendingPathComponent("INVENTORY.md"), encoding: .utf8)
        var listed = Set<String>(), paths = Set<String>()
        for line in inventory.split(separator: "\n") {
            let cells = line.split(separator: "|").map { $0.trimmingCharacters(in: .whitespaces) }
            guard cells.count > 2, ["GET", "POST", "DELETE", "*"].contains(cells[0]) else { continue }
            let path = cells[1].replacingOccurrences(of: "`", with: "")
            listed.insert("\(cells[0]) \(path)")
            paths.insert(path)
        }
        let sources = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Sources/AgentDeck")
        var found = Set<String>(), matched = Set<String>()
        for file in ["DeckRouter.swift", "BriefRoutes.swift"] {
            let text = try String(contentsOf: sources.appendingPathComponent(file), encoding: .utf8)
            let range = NSRange(text.startIndex..., in: text)
            let cases = try NSRegularExpression(pattern: #"\("(GET|POST|DELETE)", "(/[^"]*)"\)"#)
            for match in cases.matches(in: text, range: range) {
                found.insert("\(text[Range(match.range(at: 1), in: text)!]) \(text[Range(match.range(at: 2), in: text)!])")
            }
            // A path matched another way: `path == "/x"` or `hasPrefix("/x/")`. A prefix is listed as /x/<…> or /x/*.
            let others = try NSRegularExpression(pattern: #"(?:path == |hasPrefix\()"(/[^"]*)""#)
            for match in others.matches(in: text, range: range) { matched.insert(String(text[Range(match.range(at: 1), in: text)!])) }
        }
        XCTAssertGreaterThan(found.count, 90, "the router's cases were found")
        XCTAssertGreaterThan(matched.count, 10, "the router's other paths were found")
        for route in found.sorted() { XCTAssertTrue(listed.contains(route), "\(route) is not in INVENTORY.md") }
        for path in matched.sorted() {
            let prefix = path.hasSuffix("/")
            XCTAssertTrue(prefix ? paths.contains { $0.hasPrefix(path) && $0.count > path.count } : paths.contains(path),
                          "\(path) (matched by path) is not in INVENTORY.md")
        }
        for sound in DeckRouter.sounds {
            XCTAssertTrue(paths.contains("/sounds/*") && sound.hasPrefix("/sounds/"), "\(sound) is not in INVENTORY.md")
        }
        for (_, routes) in Self.groups {
            for route in routes where !route.path.hasPrefix("/bots/") && !route.path.contains("nothing-here") && !route.path.hasPrefix("/setup/logo/") {
                XCTAssertTrue(listed.contains(route.name), "\(route.name) has a fixture but no row in INVENTORY.md")
            }
        }
    }
}
