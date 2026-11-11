import XCTest
@testable import AgentDeck

/// The route contract (M0 of the one-build plan): what the Mac answers on each route, written as shapes into
/// `Tests/fixtures/contract/`, which the Linux/Windows app's `desktop/contract_test.go` checks its own answers
/// against. Shapes, not values: an answer's status, its kind of body, and for JSON its keys and their types.
/// A run with SMOLCORP_WRITE_FIXTURES=1 writes the files again; a normal run checks the Mac still answers so.
/// `INVENTORY.md` beside them lists every route, with or without a fixture.
///
/// Only routes that stay inside the test's own folder are asked: none that writes your home folder (hooks,
/// login item, the smolcorp link), asks the Keychain, opens a window or a terminal, or starts another program.
/// Those are in the inventory, with their refusals here where a refusal comes before anything is done.
final class ContractTests: XCTestCase {
    // MARK: The routes asked

    /// One request, as one caller: the page (its cookie), a tool (the setup key), a paired phone on the Wi-Fi,
    /// the hook (its key), or nobody. `given` is what's set up first (see `prepare`).
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
            default: return ["phone"]
            }
        }
    }

    private static func ask(_ caller: String = "page", query: [String: String] = [:], body: Any? = nil, given: [String] = [],
                            headers: [String: String] = [:]) -> Ask {
        Ask(caller: caller, query: query, body: body, given: given, headers: headers)
    }

    /// What the setup page's state route answers, and the office's snapshot: the shapes most routes refer to.
    static let state: [Route] = [
        Route("GET", "/setup/state", ask(given: ["sessions", "a pairing code", "a paired phone"]),
              refusals: [(ask("nobody"), "no cookie, no setup key: the locked page")]),
        Route("GET", "/setup/state", ask("tool", given: ["sessions", "a pairing code", "a paired phone"]),
              note: "a tool sees whether pairing is open, never the code, and no webhook secret"),
        Route("GET", "/events", ask("phone", given: ["sessions"]),
              refusals: [(ask("nobody"), "no phone's key")],
              note: "an event stream; its first message is the snapshot (shape snapshot), and each change sends it again"),
    ]

    /// Setup's routes the page uses, besides the GM's. Each asked with a pairing code open, so those that answer
    /// with the setup state answer it whole (the page sees the code).
    static let setup: [Route] = setupRoutes.map { route in
        var route = route
        route.ask?.given.append("a pairing code")
        return route
    }

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
        Route("POST", "/setup/floors", ask(body: ["rules": [Any]()]),
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
        Route("POST", "/setup/bots", ask(body: ["bots": [Any]()]),
              refusals: [(ask(body: "not json"), "not a list of bots")]),
        Route("POST", "/setup/bots/override", ask(body: ["botFloor": 1]),
              refusals: [(ask(body: ["botFloor": "2"]), "botFloor not a whole number"),
                         (ask(body: ["id": "no-such-bot"]), "no such bot")]),
        Route("POST", "/setup/bots/mailbox", nil, refusals: [(ask(body: [String: Any]()), "no id and folder")]),
        Route("POST", "/setup/bots/resolve", ask()),
        Route("GET", "/setup/hire-folder", ask()),
        Route("GET", "/setup/hire/folders", ask()),
        Route("GET", "/setup/briefs", ask()),
        Route("POST", "/setup/hire", nil, refusals: [(ask(body: [String: Any]()), "no role"),
                                                     (ask(body: ["role": "nobody-at-all"]), "no such role")]),
        Route("POST", "/setup/hire/gm", nil, refusals: [(ask(), "no folder for your hires yet")],
              note: "hires the GM in the folder for your hires; asked here only with none set"),
        Route("POST", "/setup/notify", nil, refusals: [(ask(body: [String: Any]()), "no title or body")]),
        Route("POST", "/setup/viewer", nil, refusals: [(ask(body: "not json"), "not a viewer setting")]),
        Route("POST", "/setup/t3code/open", nil, refusals: [(ask(body: ["id": "no-such-hire"]), "no such hire")]),
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
        Route("GET", "/setup/gm/outbox", ask(given: ["manager on"])),
        Route("GET", "/setup/gm/watches", ask(given: ["manager on"])),
        Route("POST", "/setup/gm/watch", nil, refusals: [(ask(body: "not json", given: ["manager on"]), "not a watch")]),
        Route("POST", "/setup/gm/unwatch", nil, refusals: [(ask(given: ["manager on"]), "no id"),
                                                           (ask(query: ["id": "no-such-watch"], given: ["manager on"]), "no such watch")]),
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
    ]

    /// The office's routes (a paired phone's), the hook's, a cloud bot's, and pairing.
    static let office: [Route] = [
        Route("POST", "/hook", ask("hook", body: ContractTests.hooks[0]["body"]!, headers: ContractTests.hooks[0]["headers"] as! [String: String]),
              refusals: [(ask("nobody", body: [String: Any]()), "no hook key")],
              note: "the agent's hook event, with its agent, tab, project, pid and parent headers; it feeds the snapshot"),
        Route("POST", "/pair", nil, refusals: [(ask("nobody", body: ["code": "1"]), "a code that isn't the one asked for")],
              note: "a right code answers {key}; a wrong one 403 {error}"),
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
        Route("POST", "/dictate", nil, refusals: [(ask("phone", body: ["text": "hi"]), "over Tailscale's HTTPS address only")]),
        Route("POST", "/peek", nil, refusals: [(ask("phone"), "over Tailscale's HTTPS address only")]),
        Route("POST", "/choose", nil, refusals: [(ask("phone"), "over Tailscale's HTTPS address only")]),
        Route("GET", "/choice", nil, refusals: [(ask("phone"), "over Tailscale's HTTPS address only")]),
        Route("GET", "/t3code", nil, refusals: [(ask("phone"), "the office's own window only, never a phone")]),
        Route("POST", "/t3code/open", nil, refusals: [(ask("phone"), "the office's own window only, never a phone")]),
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

    /// "sessions": two Claude Code sessions as their hooks report them, one working and one asking you.
    static let hooks: [[String: Any]] = [
        ["headers": ["x-agentdeck-agent": "claude", "x-agentdeck-tab": "tty:/dev/ttys901", "x-agentdeck-project": "/work/alpha"],
         "body": ["hook_event_name": "UserPromptSubmit", "session_id": "contract-a", "cwd": "/work/alpha", "prompt": "Write the tests."]],
        ["headers": ["x-agentdeck-agent": "claude", "x-agentdeck-tab": "tty:/dev/ttys902", "x-agentdeck-project": "/work/beta"],
         "body": ["hook_event_name": "PreToolUse", "session_id": "contract-b", "cwd": "/work/beta", "tool_name": "Bash",
                  "tool_input": ["command": "ls"]]],
        ["headers": ["x-agentdeck-agent": "claude", "x-agentdeck-tab": "tty:/dev/ttys902", "x-agentdeck-project": "/work/beta"],
         "body": ["hook_event_name": "Notification", "session_id": "contract-b", "cwd": "/work/beta",
                  "message": "Claude needs your permission to use Bash"]],
    ]

    // MARK: Shapes

    /// A JSON value's shape: "string", "number", "bool" or "null" for a leaf; an object's keys, each with its shape;
    /// an array's elements merged into one shape, or [] for none.
    static func shape(_ value: Any) -> Any {
        switch value {
        case is NSNull: return "null"
        case is String: return "string"
        case let number as NSNumber: return CFGetTypeID(number) == CFBooleanGetTypeID() ? "bool" : "number"
        case let array as [Any]: return array.isEmpty ? [Any]() : [array.map(shape).reduce(array.map(shape)[0], merge)]
        case let object as [String: Any]: return object.mapValues(shape)
        default: return "any"
        }
    }

    /// Two shapes as one: null gives way to the other, objects keep every key, and two different leaves are "any".
    static func merge(_ a: Any, _ b: Any) -> Any {
        if a as? String == "null" { return b }
        if b as? String == "null" { return a }
        if let a = a as? [String: Any], let b = b as? [String: Any] {
            return a.merging(b) { merge($0, $1) }
        }
        if let a = a as? [Any], let b = b as? [Any] {
            if a.isEmpty { return b }
            if b.isEmpty { return a }
            return [merge(a[0], b[0])]
        }
        if let a = a as? String, let b = b as? String, a == b { return a }
        return "any"
    }

    /// Where `got` doesn't have `want`'s shape, as "where: what" lines (`desktop/contract_test.go` says them the
    /// same way). A null on either side, an empty array or object, or "any" stands for anything: values, not shapes.
    static func differences(_ want: Any, _ got: Any, at path: String = "") -> [String] {
        let here = path.isEmpty ? "." : path
        if let want = want as? String, want == "null" || want == "any" { return [] }
        if let got = got as? String, got == "null" || got == "any" { return [] }
        if let want = want as? [String: Any], let got = got as? [String: Any] {
            // An empty object is a map with nothing in it now, such as the team's nicknames: anything.
            if want.isEmpty || got.isEmpty { return [] }
            var found: [String] = []
            for key in Set(want.keys).union(got.keys).sorted() {
                switch (want[key], got[key]) {
                case let (w?, g?): found += differences(w, g, at: path + "." + key)
                case (_?, nil): found.append("\(path).\(key): missing")
                default: found.append("\(path).\(key): extra")
                }
            }
            return found
        }
        if let want = want as? [Any], let got = got as? [Any] {
            return want.isEmpty || got.isEmpty ? [] : differences(want[0], got[0], at: path + "[]")
        }
        let w = name(want), g = name(got)
        return w == g ? [] : ["\(here): \(g), want \(w)"]
    }

    private static func name(_ shape: Any) -> String {
        if let leaf = shape as? String { return leaf }
        return shape is [Any] ? "array" : "object"
    }

    // MARK: Asking the Mac

    private struct Answer {
        let status: Int
        let kind: String
        let json: Any?
    }

    private func prepare(_ deck: Deck, _ router: DeckRouter, _ given: [String]) throws {
        for step in given {
            switch step {
            case "sessions":
                for hook in Self.hooks {
                    let answer = try send(router, deck, method: "POST", path: "/hook", caller: "hook",
                                          headers: hook["headers"] as? [String: String] ?? [:], body: hook["body"])
                    XCTAssertEqual(answer.status, 204, "a hook for the sessions")
                }
            case "a pairing code": deck.access.newCode()
            case "a paired phone": _ = try phoneKey(deck)
            case "manager on": Config.shared.managerEnabled = true
            default: XCTFail("nothing gives \(step)")
            }
        }
    }

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
        case .eventStream: return Answer(status: 200, kind: "event-stream", json: nil)
        case .response(let response):
            let json = response.contentType == "application/json" ? try? JSONSerialization.jsonObject(with: response.body) : nil
            let kind = json != nil ? "json" : response.body.isEmpty ? "empty"
                : response.contentType.hasPrefix("image/png") ? "png" : response.contentType.hasPrefix("text/html") ? "html" : "text"
            return Answer(status: response.status, kind: kind, json: json)
        }
    }

    /// One request on a deck of its own, with what it's given set up first.
    private func answer(_ route: Route, _ ask: Ask) throws -> Answer {
        Config.shared.managerEnabled = false
        let deck = Deck(quiet: true)
        deck.layoutConfig = Config()
        let router = DeckRouter(deck: deck)
        try prepare(deck, router, ask.given)
        return try send(router, deck, method: route.method, path: route.path, caller: ask.caller, query: ask.query,
                        headers: ask.headers, body: ask.body)
    }

    /// The office's snapshot, as `/events` sends it first, with the sessions in it.
    private func snapshot() throws -> Any {
        Config.shared.managerEnabled = false
        let deck = Deck(quiet: true)
        deck.layoutConfig = Config()
        try prepare(deck, DeckRouter(deck: deck), ["sessions"])
        return try JSONSerialization.jsonObject(with: deck.snapshotJSON())
    }

    // MARK: The files

    static let folder = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
        .appendingPathComponent("fixtures/contract", isDirectory: true)

    /// `contract.json`: the shapes routes refer to by name, and what each `given` is.
    private func shared() throws -> [String: Any] {
        let state = try answer(Self.state[0], try XCTUnwrap(Self.state[0].ask))
        let tool = try answer(Self.state[1], try XCTUnwrap(Self.state[1].ask))
        return [
            "note": "Written by ContractTests.swift from the Mac's answers (SMOLCORP_WRITE_FIXTURES=1). Shapes, not values: "
                + "\"string\", \"number\", \"bool\", \"null\" (any value: it can be null), an object's keys, an array's element "
                + "([] for none: any). Read by desktop/contract_test.go.",
            "shapes": [
                "setup-state": ["route": "GET /setup/state", "as": "page", "shape": Self.shape(try XCTUnwrap(state.json))],
                "setup-state-tool": ["route": "GET /setup/state", "as": "tool", "shape": Self.shape(try XCTUnwrap(tool.json))],
                "snapshot": ["route": "GET /events", "as": "phone", "shape": Self.shape(try snapshot())],
            ],
            "given": [
                "sessions": ["hooks": Self.hooks.map { ["method": "POST", "path": "/hook", "headers": $0["headers"]!, "body": $0["body"]!] }],
                "a pairing code": "the page asked for a code (POST /setup/pair)",
                "a paired phone": "a phone paired with that code (POST /pair); phone requests carry its key as ?t=",
                "manager on": "config.json's managerEnabled is true",
            ],
        ]
    }

    /// A group's file: each route, who may call it, and its answers as the Mac gives them.
    private func group(_ routes: [Route], shapes: [String: Any]) throws -> [String: Any] {
        var written: [[String: Any]] = []
        for route in routes {
            var row: [String: Any] = ["route": route.name, "callers": route.callers]
            if let ask = route.ask {
                let got = try answer(route, ask)
                var answer: [String: Any] = ["status": got.status, "kind": got.kind]
                if route.path == "/events" {
                    answer["shape"] = "snapshot"
                } else if let json = got.json {
                    let shape = Self.shape(json)
                    // An answer that is a shape with a name is written by its name.
                    let named = shapes.filter { _, value in
                        guard let named = (value as? [String: Any])?["shape"] else { return false }
                        return Self.differences(named, shape).isEmpty && Self.differences(shape, named).isEmpty
                    }.keys.sorted()
                    answer["shape"] = ask.caller == "tool" && named.contains("setup-state-tool") ? "setup-state-tool"
                        : named.first(where: { $0 != "setup-state-tool" }) ?? named.first ?? shape
                }
                row["request"] = ask.json
                row["answer"] = answer
            }
            if !route.refusals.isEmpty {
                row["refusals"] = try route.refusals.map { refusal -> [String: Any] in
                    let got = try answer(route, refusal.ask)
                    XCTAssertGreaterThanOrEqual(got.status, 400, "\(route.name): \(refusal.why)")
                    return ["request": refusal.ask.json, "status": got.status, "why": refusal.why]
                }
            }
            if let note = route.note { row["note"] = note }
            written.append(row)
        }
        return ["routes": written]
    }

    private static func data(_ object: Any) throws -> Data {
        try JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted, .sortedKeys])
    }

    // MARK: Tests

    private func keep<T>(_ path: ReferenceWritableKeyPath<Config, T>) {
        let kept = Config.shared[keyPath: path]
        addTeardownBlock {
            Config.shared[keyPath: path] = kept
            Config.shared.save()
        }
    }

    override func setUpWithError() throws {
        // What the routes asked here change, put back after; and a Mac with no folder for hires, as on a first run.
        keep(\.managerEnabled); keep(\.scrollEnabled); keep(\.maxConfirm); keep(\.guideDone); keep(\.phoneMicEnabled)
        keep(\.phoneMicAutoSend); keep(\.officeOnTop); keep(\.logo); keep(\.colours); keep(\.layout); keep(\.pins)
        keep(\.floors); keep(\.floorRules); keep(\.webhook); keep(\.profiles); keep(\.activeProfile); keep(\.hireAgent)
        keep(\.summaryOptIn); keep(\.cloudBots); keep(\.botFloor); keep(\.hireFolder); keep(\.hiringEnabled); keep(\.gmByHand)
        Config.shared.hireFolder = ""
        Config.shared.hiringEnabled = false
        Config.shared.summaryOptIn = false
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

    /// Writes the files with SMOLCORP_WRITE_FIXTURES=1; otherwise the Mac must still answer as they say: the same
    /// routes, requests, callers and statuses, and answers of the same shape.
    func testTheMacAnswersAsTheContractSays() throws {
        let shared = try shared()
        let shapes = try XCTUnwrap(shared["shapes"] as? [String: Any])
        var files: [(String, [String: Any])] = [("contract.json", shared)]
        for (file, routes) in Self.groups { files.append((file, try group(routes, shapes: shapes))) }

        if ProcessInfo.processInfo.environment["SMOLCORP_WRITE_FIXTURES"] == "1" {
            try FileManager.default.createDirectory(at: Self.folder, withIntermediateDirectories: true)
            for (file, object) in files { try Self.data(object).write(to: Self.folder.appendingPathComponent(file)) }
        }
        for (file, now) in files {
            let written = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: Self.folder.appendingPathComponent(file))) as? [String: Any],
                                        "\(file): run once with SMOLCORP_WRITE_FIXTURES=1")
            XCTAssertEqual(Self.withoutShapes(written) as NSDictionary, Self.withoutShapes(now) as NSDictionary,
                           "\(file) is out of date: run once with SMOLCORP_WRITE_FIXTURES=1")
            for (name, found) in Self.shapes(in: now, named: shapes) {
                let want = Self.shapes(in: written, named: written["shapes"] as? [String: Any] ?? shapes)[name] ?? [String: Any]()
                for difference in Self.differences(want, found) + Self.differences(found, want).filter({ $0.hasSuffix(": missing") }) {
                    XCTFail("\(file) \(name) \(difference): run once with SMOLCORP_WRITE_FIXTURES=1")
                }
            }
        }
    }

    /// A file with every shape left out, for comparing all the rest exactly.
    private static func withoutShapes(_ object: [String: Any]) -> [String: Any] {
        var object = object
        if var shapes = object["shapes"] as? [String: Any] {
            for (name, value) in shapes { shapes[name] = (value as? [String: Any])?.filter { $0.key != "shape" } }
            object["shapes"] = shapes
        }
        if let routes = object["routes"] as? [[String: Any]] {
            object["routes"] = routes.map { route -> [String: Any] in
                var route = route
                if var answer = route["answer"] as? [String: Any], !(answer["shape"] is String) {
                    answer["shape"] = "inline"
                    route["answer"] = answer
                }
                return route
            }
        }
        return object
    }

    /// Every shape in a file, by where it is: a named one, or a route's answer (a name resolved).
    private static func shapes(in object: [String: Any], named: [String: Any]) -> [String: Any] {
        var found: [String: Any] = [:]
        for (name, value) in object["shapes"] as? [String: Any] ?? [:] { found[name] = (value as? [String: Any])?["shape"] }
        for (index, route) in (object["routes"] as? [[String: Any]] ?? []).enumerated() {
            guard let shape = (route["answer"] as? [String: Any])?["shape"] else { continue }
            let resolved = (shape as? String).flatMap { (named[$0] as? [String: Any])?["shape"] } ?? shape
            found["\(route["route"] ?? "") #\(index)"] = resolved
        }
        return found
    }

    func testShapesAreKeysAndTypes() throws {
        let json = try JSONSerialization.jsonObject(with: Data(#"{"a": 1, "b": [true, null], "c": [{"d": "x"}, {"e": null}], "f": [], "g": null}"#.utf8))
        let shape = try XCTUnwrap(Self.shape(json) as? [String: Any])
        XCTAssertEqual(shape as NSDictionary, ["a": "number", "b": ["bool"], "c": [["d": "string", "e": "null"]], "f": [], "g": "null"] as NSDictionary)
        XCTAssertEqual(Self.differences(["a": "number", "b": "string"], ["a": "string", "c": "bool"]),
                       [".a: string, want number", ".b: missing", ".c: extra"])
        XCTAssertEqual(Self.differences(["a": ["string"], "g": "number"], ["a": [Any](), "g": "null"]), [], "an empty array or a null stands for anything")
        XCTAssertEqual(Self.differences([["x": "bool"]], [["x": "number"]]), ["[].x: number, want bool"])
    }

    /// Every route the Mac's router answers is a row of INVENTORY.md, so none is left out of the contract.
    func testEveryRouteTheMacAnswersIsInTheInventory() throws {
        let inventory = try String(contentsOf: Self.folder.appendingPathComponent("INVENTORY.md"), encoding: .utf8)
        let listed = Set(inventory.split(separator: "\n").compactMap { line -> String? in
            let cells = line.split(separator: "|").map { $0.trimmingCharacters(in: .whitespaces) }
            guard cells.count > 2, ["GET", "POST", "DELETE", "*"].contains(cells[0]) else { return nil }
            return "\(cells[0]) \(cells[1].replacingOccurrences(of: "`", with: ""))"
        })
        let sources = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Sources/AgentDeck")
        var found = Set<String>()
        for file in ["DeckRouter.swift", "BriefRoutes.swift"] {
            let text = try String(contentsOf: sources.appendingPathComponent(file), encoding: .utf8)
            let pattern = try NSRegularExpression(pattern: #"\("(GET|POST|DELETE)", "(/[^"]*)"\)"#)
            for match in pattern.matches(in: text, range: NSRange(text.startIndex..., in: text)) {
                found.insert("\(text[Range(match.range(at: 1), in: text)!]) \(text[Range(match.range(at: 2), in: text)!])")
            }
        }
        XCTAssertGreaterThan(found.count, 90, "the router's cases were found")
        for route in found.sorted() { XCTAssertTrue(listed.contains(route), "\(route) is not in INVENTORY.md") }
        for (_, routes) in Self.groups {
            for route in routes where !route.path.hasPrefix("/bots/") && !route.path.contains("nothing-here") && !route.path.hasPrefix("/setup/logo/") {
                XCTAssertTrue(listed.contains(route.name), "\(route.name) has a fixture but no row in INVENTORY.md")
            }
        }
    }
}
