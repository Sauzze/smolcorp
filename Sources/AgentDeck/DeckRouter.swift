import AppKit

/// Every URL the deck answers.
///
/// - The phone's pages and API need a paired phone's key, sent as `?t=` (the
///   page's own URL, so a home-screen bookmark keeps it) or as
///   `X-AgentDeck-Token`. They must also be addressed to an IP address or a
///   local name, which keeps out web pages using DNS rebinding.
/// - `/hook` needs the hook's own key, and answers only to this Mac.
/// - Pairing is open, but useless without a code you asked for on this Mac.
/// - The setup page has no key, so it answers only to this Mac, and its
///   actions need a header that other sites cannot send without asking first.
final class DeckRouter: Router {
    /// Sent by the setup page with every action. A custom header makes the
    /// browser ask permission first for any cross-site request, which this
    /// server never grants — so a web page you visit cannot press these buttons.
    static let setupHeader = "x-agentdeck-setup"
    /// Names the app on the 401 of /setup/state, so the locked page tells how to get in.
    static let appHeader = "X-Smolcorp-App"

    /// Remembers in config.json that you disconnected the agents (or connected
    /// them again), so --setup's repair leaves them be. Saved only when it changes.
    static func keepDisconnected(_ disconnected: Bool, in config: Config, to file: URL? = nil) {
        guard config.hooksDisconnected != disconnected else { return }
        config.hooksDisconnected = disconnected
        if let file { config.save(to: file) } else { config.save() }
    }

    /// The agent's pid from a hook's header: digits only, anything else is
    /// ignored.
    static func pid(_ text: String?) -> Int32? {
        guard let text, !text.isEmpty, text.allSatisfy({ $0.isASCII && $0.isNumber }), let pid = Int32(text), pid > 0 else { return nil }
        return pid
    }
    /// Setup's page and everything it reads and changes: this Mac's only.
    static func isSetupPath(_ path: String) -> Bool {
        path == "/setup" || path.hasPrefix("/setup/")
    }

    /// Setup's gate, as `handle` runs it: who is asking, or why they're
    /// refused. It must come from this Mac (404), and an action must carry
    /// the setup header (403). Past the page itself, it needs the page's
    /// session cookie, from the menu's one-time link, or the setup-key
    /// file's key in a tool's header (401); the file's key is never taken as
    /// the cookie. The GM is whoever sends its token in its header. Each
    /// caller is refused (403) what the route table doesn't give it: see
    /// `SetupRoutes`. The page itself holds nothing, and /setup/session is
    /// where a link's key is handed over for a session.
    static func setupGate(_ request: Request, sessions: SetupSessions, isSetupKey: (String?) -> Bool,
                          isGMToken: (String?) -> Bool = { _ in false }, isHireKey: (String?) -> Bool = { _ in false }) -> (refusal: Response?, caller: SetupSessions.Caller) {
        guard request.isLocal else { return (.status(404), .nobody) }
        guard request.method == "GET" || request.headers[setupHeader] == "1" else { return (.status(403), .nobody) }
        var caller = sessions.caller(header: request.headers[setupKeyHeader], cookie: cookie(setupCookie, in: request), isSetupKey: isSetupKey,
                                     gmHeader: request.headers[GMIdentity.header], isGMToken: isGMToken,
                                     hireKey: request.headers[FrontDesk.sendKeyHeader], isHireKey: isHireKey)
        // A request with the setup key and the GM's token is the owner's tool for what a tool may do
        // (a read-only request with both acts as the owner and doesn't mark the GM as started by hand).
        // For a route a tool may not use, the token makes it the GM: its action, signed as the GM.
        if caller == .tool, !SetupRoutes.allows(.tool, request.method, request.path), isGMToken(request.headers[GMIdentity.header]) { caller = .gm }
        let open = request.path == "/setup" || (request.path == "/setup/session" && request.method == "POST")
        if caller == .nobody && !open {
            // The locked page says which app it came from, so its note names this app's own way in.
            var locked = Response.status(401)
            if request.path == "/setup/state" { locked.headers[appHeader] = "mac" }
            return (locked, caller)
        }
        if caller != .nobody && !open && !SetupRoutes.allows(caller, request.method, request.path) {
            return (.status(403, SetupRoutes.refusal(request.method, request.path)), caller)
        }
        return (nil, caller)
    }

    /// POST /setup/guide: the first-run guide finished or skipped (done=1), or to be shown again (done=0), saved.
    static func saveGuide(_ request: Request, config: Config, file: URL = Paths.config) {
        config.guideDone = request.query["done"] == "1"
        config.save(to: file)
    }

    /// Setup's state for the first-run guide: whether it's over (see guide.js).
    static func guideState(_ config: Config) -> [String: Any] { ["done": config.guideDone] }

    /// Start here's GM hire with no folder for your hires: the page points at its folder step (#first-folder).
    static let chooseHireFolderFirst = "Choose the folder for your hires first: it's the first step of Start here."

    /// A tool with the setup-key file may only run `--setup`'s own repair (auto=1),
    /// which leaves agents you disconnected alone: installing the app, the login
    /// and accessibility fixes, and a repair that reconnects are for the setup page (or the GM).
    static func fixRefusal(caller: SetupSessions.Caller, fix: String?, auto: String? = nil) -> Response? {
        caller == .tool && !(fix == SelfTest.repair && auto == "1") ? .status(403, "This is for the setup page the menu opens.") : nil
    }

    /// The webhook's secret goes to the setup page only: with it, anyone could sign as the owner.
    static func webhookSecret(_ secret: String, for caller: SetupSessions.Caller) -> String {
        caller == .page ? secret : ""
    }

    /// Whom the deck answers past setup and the hook: an IP address or a
    /// local name, as a phone on the network asks; or, with the phone mic on
    /// (personal build), a tailnet name passed on by `tailscale serve`.
    static func admits(_ request: Request, phoneMic: Bool) -> Bool {
        request.isAddressedDirectly || (request.isTailnet && phoneMic)
    }

    /// The phone's text (typed, or heard by its own mic) is typed, and sent,
    /// from a paired phone with its key, over the Wi-Fi's plain http as well
    /// as through `tailscale serve` (I34, the owner's call): on the Wi-Fi it
    /// isn't encrypted, which the phone's settings say. Never from the office's
    /// own window, which never typed text. Every request here has passed
    /// `admits` and the phone's key.
    static func takesDictation(phone: PairedPhone) -> Bool {
        phone.id != PairedPhone.officeWindow.id
    }

    static let phonesOnly = "Text is typed from a paired phone."

    /// Personal build: the peek at an agent's input line shows what its
    /// terminal shows, so it's given only through `tailscale serve`, though
    /// text is typed over the Wi-Fi too (I34). Never over the Wi-Fi, where a
    /// key seen on the network could otherwise read a terminal.
    static func showsTerminal(_ request: Request) -> Bool {
        request.isTailnet
    }

    /// The approvals list (who waits for you, and on what, as a short label): to a paired phone through `tailscale serve`
    /// only, never over the Wi-Fi's plain http nor to the office's window. It only shows; answering is `/act` and `/choose`.
    static func showsApprovals(_ request: Request, phone: PairedPhone) -> Bool {
        request.isTailnet && phone.id != PairedPhone.officeWindow.id
    }

    /// iPhone app (personal build): an agent's conversation, read from its
    /// transcript, goes only through `tailscale serve`, to a phone paired with
    /// its own key: never over the Wi-Fi's plain http, nor to the office's window.
    static func showsConversation(_ request: Request, phone: PairedPhone) -> Bool {
        request.isTailnet && phone.id != PairedPhone.officeWindow.id
    }

    /// ⓘ Info and the Mac's ▤ conversation (personal build): this Mac's own
    /// office window, where nothing leaves the Mac, so the conversation is shown
    /// as it was written, without the phone's redaction.
    static func isOfficeWindow(_ request: Request, phone: PairedPhone) -> Bool {
        request.isLocal && !request.isTailnet && phone.id == PairedPhone.officeWindow.id
    }

    /// ⓘ Info (personal build): the session's account, model, time and tokens,
    /// to this Mac's office window, or to a phone paired with its own key
    /// through `tailscale serve`, as the conversation goes: never over the Wi-Fi.
    static func showsInfo(_ request: Request, phone: PairedPhone) -> Bool {
        isOfficeWindow(request, phone: phone) || showsConversation(request, phone: phone)
    }

    /// iPhone app (personal build): a phone's device token comes only through
    /// `tailscale serve`, from a phone paired with its own key.
    static func takesPushToken(_ request: Request, phone: PairedPhone) -> Bool {
        request.isTailnet && phone.id != PairedPhone.officeWindow.id
    }

    /// iPhone app (personal build): what the phone's settings show of its
    /// notifications: whether this Mac has the push key, and which it gets.
    static func notifications(_ push: PushTarget?, hasKey: Bool) -> [String: Any] {
        ["key": hasKey, "registered": push != nil, "finished": push?.finished ?? true, "waiting": push?.waiting ?? true,
         "gm": push?.gm ?? true]
    }

    /// What a phone's POST /notifications asks for: its token and switches, or
    /// nil, for no more notifications, when there's no token that could be one. Its office's name and id are kept
    /// only if they're good ones; a bad one is left out and the token is taken all the same.
    static func pushTarget(_ given: [String: Any]) -> PushTarget? {
        guard let token = given["token"] as? String, PushTarget.isToken(token) else { return nil }
        return PushTarget(token: token, sandbox: given["sandbox"] as? Bool ?? true, finished: given["finished"] as? Bool ?? true,
                          waiting: given["waiting"] as? Bool ?? true, gm: given["gm"] as? Bool ?? true,
                          office: PushTarget.officeLabel(given["office"]), officeID: PushTarget.officeIdentifier(given["officeID"]))
    }

    /// A page's end in the transcript, as an earlier page gave it: digits only.
    static func offset(_ text: String?) -> UInt64?? {
        guard let text else { return .some(nil) }
        guard !text.isEmpty, text.count <= 20, text.allSatisfy({ $0.isASCII && $0.isNumber }), let value = UInt64(text) else { return nil }
        return .some(value)
    }

    /// Whom wrong pairing codes are counted against. Personal build: through
    /// `tailscale serve` every phone comes from this Mac, so they're counted
    /// by the tailnet address the proxy says the phone has (its own, last),
    /// apart from the Wi-Fi's.
    static func pairingAddress(_ request: Request) -> String {
        guard request.isTailnet else { return request.remoteAddress }
        let forwarded = request.headers["x-forwarded-for"]?.split(separator: ",").last
            .map { $0.trimmingCharacters(in: .whitespaces) }.flatMap { $0.isEmpty ? nil : $0 }
        return "tailnet:" + (forwarded ?? request.remoteAddress)
    }

    /// The page with its Home Screen icon in it, as a data: URL, from `icon`, the logo's 180 px PNG. iOS fetches a
    /// linked touch icon itself, outside the page and without its browser, and where that fetch fails the Home Screen
    /// shows a letter instead of the logo.
    static func withIcon(_ page: Response, icon: URL) -> Response {
        guard page.status == 200, let html = String(data: page.body, encoding: .utf8),
              let png = try? Data(contentsOf: icon) else { return page }
        var inlined = page
        inlined.body = Data(html.replacingOccurrences(of: #"<link rel="apple-touch-icon" href="/icon.png">"#,
            with: #"<link rel="apple-touch-icon" sizes="180x180" href="data:image/png;base64,\#(png.base64EncodedString())">"#).utf8)
        return inlined
    }

    /// The pairing page as the Mac says it: "Mac" and the menu bar, without the
    /// tray and the no-tray hint (`smolcorp quit`, Ctrl+C), which the Mac has no use for.
    /// The same file serves Linux and Windows, which keep their own words.
    static func macPairing(_ page: Response) -> Response {
        guard page.status == 200, let html = String(data: page.body, encoding: .utf8) else { return page }
        var worded = page
        worded.body = Data(html
            .replacingOccurrences(of: "Pair with your computer", with: "Pair with your Mac")
            .replacingOccurrences(of: "On your computer, click the Smolcorp icon in the menu bar or tray and choose <b>Pair a Phone…</b>. With no tray, open the setup link Smolcorp printed as it started (for a new one, run <code>smolcorp quit</code> or press Ctrl+C in its terminal, then start Smolcorp again) and choose <b>Pair a phone</b>. Enter",
                                  with: "On your Mac, click the Smolcorp icon in the menu bar and choose <b>Pair a Phone…</b>. Enter")
            .replacingOccurrences(of: "Could not reach your computer.", with: "Could not reach your Mac.").utf8)
        return worded
    }

    /// Personal build: the deck and pairing pages as the Tailscale address
    /// serves them, told apart from the Wi-Fi's at a glance and on the Home
    /// Screen: "Smolcorp Tail", and the deck's TAIL mark shown.
    static func markedForTailnet(_ page: Response) -> Response {
        guard page.status == 200, let html = String(data: page.body, encoding: .utf8) else { return page }
        var marked = page
        marked.body = Data(html
            .replacingOccurrences(of: #"<meta name="apple-mobile-web-app-title" content="Smolcorp">"#,
                                  with: #"<meta name="apple-mobile-web-app-title" content="Smolcorp Tail">"#)
            .replacingOccurrences(of: "Smolcorp</title>", with: "Smolcorp Tail</title>")
            .replacingOccurrences(of: "<b>Add Smolcorp to your Home Screen.</b>", with: "<b>Add Smolcorp Tail to your Home Screen.</b>")
            .replacingOccurrences(of: #"<html lang="en">"#, with: #"<html lang="en" data-tail>"#).utf8)
        return marked
    }

    /// POST /setup/session: a menu link's key, once, for the page's session cookie.
    /// The setup-key file's key, or a link already used, opens nothing.
    static func sessionReply(link: String?, sessions: SetupSessions) -> Response {
        guard let session = sessions.open(link: link) else {
            return .status(401, "Only the menu's link opens a setup page.")
        }
        var answer = Response.status(204)
        answer.headers["Set-Cookie"] = "\(setupCookie)=\(session); Path=/setup; HttpOnly; SameSite=Strict"
        return answer
    }

    /// The setup-key file's key, as a tool of your own sends it; and, on the
    /// page's one call to /setup/session, the menu's one-time link key.
    static let setupKeyHeader = "x-smolcorp-setup-key"
    /// A browser sends localhost's cookies to every port, so a test copy's has
    /// a name of its own: each app's setup page would otherwise sign the other out.
    static let setupCookie = Deck.port == 7710 ? "smolcorp-setup" : "smolcorp-setup-\(Deck.port)"
    /// The office's recordings, in web/sounds, served as office.js is.
    static let sounds: Set = ["/sounds/keys.m4a", "/sounds/keys.webm", "/sounds/talk.m4a", "/sounds/talk.webm"]

    /// What went wrong getting dictation's language ready, if anything, for setup to say.
    static var dictationProblem: String?

    /// A hook header twice is one smuggled in after another, as a line break
    /// in a variable the hook was given would make it.
    static func hasSmuggledHookHeader(_ request: Request) -> Bool {
        request.repeatedHeaders.contains { $0.hasPrefix("x-agentdeck-") }
    }

    static func cookie(_ name: String, in request: Request) -> String? {
        (request.headers["cookie"] ?? "").split(separator: ";").lazy
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .first { $0.hasPrefix("\(name)=") }
            .map { String($0.dropFirst(name.count + 1)) }
    }

    private let deck: Deck
    /// Shows the office's window, for setup's Open the office.
    var showOffice: () -> Void = {}
    /// The office's window takes its options again, as setup changed one.
    var officeOptionsChanged: () -> Void = {}
    /// The office's window goes to a rect (AppleScript's coordinates, from the main screen's top-left) when it is shown, as tiling places it.
    var placeOffice: (CGRect, CGFloat) -> Void = { _, _ in }

    init(deck: Deck) {
        self.deck = deck
    }

    func handle(_ request: Request, reply: @escaping (Reply) -> Void) {
        let respond = { reply(.response($0)) }

        // Fetched by iOS itself for the home-screen icon, with no key to send.
        if request.path == "/icon.png" {
            return respond(.file(Config.shared.logo.icon))
        }
        // A browser tab's icon, drawn for its size: the hire's head and shoulders, or the six keys
        if request.path == "/favicon-16.png" || request.path == "/favicon-32.png" {
            let logo = Config.shared.logo
            return respond(.file(logo == .smolcorp ? Paths.web.appendingPathComponent(String(request.path.dropFirst())) : logo.icon))
        }

        if DeckRouter.isSetupPath(request.path) {
            let gate = DeckRouter.setupGate(request, sessions: deck.access.setupSessions, isSetupKey: deck.access.isSetupKey, isGMToken: deck.access.isGMToken,
                                         isHireKey: { deck.frontDesk.hire(forSendKey: $0) != nil })
            if let refusal = gate.refusal { return respond(refusal) }
            // The GM's token, with the id of the session it runs in: that session is a GM started by hand.
            if gate.caller == .gm { deck.noteGM(session: request.headers[GMIdentity.sessionHeader]) }
            if request.path == "/setup/session", request.method == "POST" {
                let answer = DeckRouter.sessionReply(link: request.headers[DeckRouter.setupKeyHeader], sessions: deck.access.setupSessions)
                if answer.status == 204 { completeSetup() }
                return respond(answer)
            }
            return setup(request, caller: gate.caller, respond: respond)
        }

        // A cloud bot's news, from a tool of yours: guarded as setup's actions are.
        if request.path.hasPrefix("/bots/") {
            let gate = DeckRouter.setupGate(request, sessions: deck.access.setupSessions, isSetupKey: deck.access.isSetupKey, isGMToken: deck.access.isGMToken,
                                         isHireKey: { deck.frontDesk.hire(forSendKey: $0) != nil })
            if let refusal = gate.refusal { return respond(refusal) }
            return respond(botEvent(request))
        }

        if request.path == "/hook" {
            guard request.isLocal, request.method == "POST" else { return respond(.status(404)) }
            guard deck.access.isHookKey(request.headers["x-agentdeck-token"]) else { return respond(.status(401)) }
            guard !DeckRouter.hasSmuggledHookHeader(request) else {
                return respond(.status(400, "A hook header came twice"))
            }
            let time = Double(request.headers["x-agentdeck-at"] ?? "") ?? Date().timeIntervalSince1970 * 1000
            // ⓘ Info: whether the agent's environment has an API key or a cloud provider, never the key.
            SessionInfo.shared.note(billing: request.headers["x-agentdeck-billing"], for: request.json["session_id"] as? String ?? "")
            deck.ingest(request.json, reportedAs: request.headers["x-agentdeck-agent"] ?? "",
                        tab: request.headers["x-agentdeck-tab"] ?? "", tty: request.headers["x-agentdeck-tty"] ?? "",
                        startedIn: request.headers["x-agentdeck-project"] ?? "",
                        pid: DeckRouter.pid(request.headers["x-agentdeck-pid"]),
                        parent: request.headers["x-agentdeck-parent"] ?? "", at: time)
            return respond(.status(204))
        }

        // Personal build: or through `tailscale serve`, for the phone mic over HTTPS. Off, as before.
        guard DeckRouter.admits(request, phoneMic: Config.shared.phoneMicEnabled) else {
            return respond(.status(403))
        }

        if request.method == "POST" && request.path == "/pair" {
            let code = (request.json["code"] as? String ?? "").filter(\.isNumber)
            let userAgent = request.headers["user-agent"] ?? ""
            // Personal build: through tailscale serve every phone comes from this Mac, so wrong codes are
            // counted by the tailnet address it says the phone has, apart from the Wi-Fi's.
            guard let key = deck.access.redeem(code, from: DeckRouter.pairingAddress(request), userAgent: userAgent) else {
                let message = "That code isn't valid. Get a new one on your Mac."
                return respond(.json(["error": message], status: 403))
            }
            return respond(.json(["key": key]))
        }

        // The office's drawing, shared by the deck and the pages that show it.
        // It holds nothing of yours, so it needs no key.
        if request.method == "GET" && request.path == "/office.js" {
            return respond(.file(Paths.web.appendingPathComponent("office.js")))
        }
        // The first-run guide's arrows, for the setup page: nothing of yours either.
        if request.method == "GET" && request.path == "/guide.js" {
            return respond(.file(Paths.web.appendingPathComponent("guide.js")))
        }
        // Its recordings, the keyboards and the talk, in either kind a browser plays.
        if request.method == "GET" && DeckRouter.sounds.contains(request.path) {
            return respond(.file(Paths.web.appendingPathComponent(String(request.path.dropFirst()))))
        }

        let key = request.query["t"] ?? request.headers["x-agentdeck-token"]
        // The office's own window reads it as a phone does, from this Mac only.
        let window = request.isLocal && key.map { constantTimeEqual($0, deck.windowKey) } == true
        let phone = window ? PairedPhone.officeWindow : deck.access.phones.phone(withKey: key)
        if let phone { deck.access.phones.markSeen(phone.id) }

        // The deck, or the office on a screen of its own: the same page, and
        // the same pairing first.
        if request.method == "GET" && (request.path == "/" || request.path == "/office") {
            var file = Response.file(Paths.web.appendingPathComponent(phone == nil ? "pair.html" : "deck.html"))
            if phone == nil { file = DeckRouter.macPairing(file) }
            let page = DeckRouter.withIcon(file, icon: request.isTailnet ? Config.shared.logo.tailIcon : Config.shared.logo.icon)
            return respond(request.isTailnet ? DeckRouter.markedForTailnet(page) : page)
        }
        guard let phone else { return respond(.status(401)) }

        let session = request.query["session"] ?? ""
        switch (request.method, request.path) {
        case ("GET", "/events"):
            reply(.eventStream { [deck] in deck.attach($0, phone: phone.id) })

        case ("POST", "/focus"):
            deck.focus(session, done: respond)

        case ("POST", "/act"):
            deck.perform(request.query["action"] ?? "", on: session, query: request.query, phone: phone.id, done: respond)

        // Phone microphone (personal build): the words the phone heard, typed into the session's tab.
        case ("POST", "/dictate"):
            guard DeckRouter.takesDictation(phone: phone) else { return respond(.status(403, DeckRouter.phonesOnly)) }
            let text = request.json["text"] as? String ?? ""
            // The confirm box's choice, the deck's or the iPhone app's: Send ⏎ or Type only; an older page's says nothing.
            deck.typePhoneDictation(text, into: session, send: request.json["send"] as? Bool, done: respond)

        // Phone microphone (personal build): the last lines of the session's tab, read-only, to see what was typed.
        case ("POST", "/peek"):
            guard DeckRouter.showsTerminal(request) else {
                return respond(.status(403, "The input line shows only over Tailscale's HTTPS address."))
            }
            deck.peek(session, done: respond)

        // iPhone app (personal build): a typed answer to a menu's "Type something", from a paired phone, over the Wi-Fi too.
        case ("POST", "/choose"):
            guard DeckRouter.takesDictation(phone: phone) else { return respond(.status(403, DeckRouter.phonesOnly)) }
            guard request.query["option"] == "other" else { return respond(.status(400, "Pick an option with the deck's keys.")) }
            deck.answerOther(request.json["text"] as? String ?? "", of: request.query["choice"], in: session, done: respond)

        // iPhone app (personal build): what a permission prompt is about, its command or file, never over the Wi-Fi.
        case ("GET", "/choice"):
            guard DeckRouter.showsTerminal(request) else {
                return respond(.status(403, "The details show only over Tailscale's HTTPS address."))
            }
            deck.choiceDetail(session, done: respond)

        // "Open a window" on a hire's desk (T796), from the office's own window on this Mac, never a phone:
        // whether hires run in the background, then a window for the hire in the picked session.
        case ("GET", "/hire/window"):
            guard DeckRouter.isOfficeWindow(request, phone: phone) else { return respond(.status(403, "Hires' windows open on your Mac.")) }
            respond(.json(["background": Choices.background(Config.shared)]))

        case ("POST", "/hire/window"):
            guard DeckRouter.isOfficeWindow(request, phone: phone) else { return respond(.status(403, "Hires' windows open on your Mac.")) }
            guard let id = deck.teamHireID(session: session) else { return respond(.status(404, "This agent wasn't hired by Smolcorp, so it has no window to open.")) }
            deck.openHireWindow(id) { problem in respond(problem.map { .status(409, $0) } ?? .status(204)) }

        // Open in T3 Code (T792), from the office's own window on this Mac: never a phone. What's installed, and
        // whether the picked agent is a hire; then its folder opened, found by its session, never a path that's sent.
        case ("GET", "/t3code"):
            guard DeckRouter.isOfficeWindow(request, phone: phone) else { return respond(.status(403, "T3 Code opens on your Mac.")) }
            guard Choices.t3Code() else { return respond(.status(404, DeckRouter.t3Off)) }
            let hire = deck.hire(inSession: session)
            DeckRouter.t3Status(fresh: request.query["fresh"] == "1") { status in
                var answer = T3Code.json(status)
                answer["hire"] = hire.map { T3Code.title(of: $0) } ?? NSNull()
                // A hire in T3 Code: its desk (the tag, its status) and branch, for the card.
                if let hire, hire.engine == T3Desk.engine { answer["lane"] = T3Desk.deskJSON(hire).merging(["branch": hire.branch]) { $1 } }
                respond(.json(answer))
            }

        case ("POST", "/t3code/open"):
            guard DeckRouter.isOfficeWindow(request, phone: phone) else { return respond(.status(403, "T3 Code opens on your Mac.")) }
            guard Choices.t3Code() else { return respond(.status(404, DeckRouter.t3Off)) }
            DeckRouter.openInT3Code(deck.hire(inSession: session), respond: respond)

        // The approvals list: who waits for you, read only. Answering is the paired phone's /act and /choose.
        case ("GET", "/approvals"):
            guard DeckRouter.showsApprovals(request, phone: phone) else {
                return respond(.status(403, "The approvals show only over Tailscale's HTTPS address."))
            }
            respond(.json(Approvals.answer(deck.approvalItems())))

        // The reviews list: hires that wrote NEEDS-REVIEW, the newest, read only, tailnet only like the approvals.
        case ("GET", "/reviews"):
            guard DeckRouter.showsApprovals(request, phone: phone) else {
                return respond(.status(403, "The reviews show only over Tailscale's HTTPS address."))
            }
            respond(.json(Reviews.answer(deck.reviewItems())))

        // iPhone app (personal build): the names its speech recognition listens for, and in which language.
        case ("GET", "/glossary"):
            respond(.json(PhoneApp.glossary(desks: deck.sessions.all.map(\.projectName))))

        // iPhone app (personal build): the session's conversation, a page at a time, never over the Wi-Fi.
        // The Mac's office window (personal build): the same, as it was written, never leaving the Mac.
        case ("GET", "/conversation"):
            let here = DeckRouter.isOfficeWindow(request, phone: phone)
            guard here || DeckRouter.showsConversation(request, phone: phone) else {
                return respond(.status(403, "The conversation shows only over Tailscale's HTTPS address."))
            }
            guard let before = DeckRouter.offset(request.query["before"]), let known = DeckRouter.offset(request.query["size"]) else {
                return respond(.status(400, "That isn't a place in the conversation."))
            }
            deck.conversation(session, before: before, known: known, redacted: !here, done: respond)

        // ⓘ Info (personal build): what the session runs on, and what it has used.
        case ("GET", "/session/info"):
            guard DeckRouter.showsInfo(request, phone: phone) else {
                return respond(.status(403, "The info shows only over Tailscale's HTTPS address."))
            }
            deck.info(request.query["id"] ?? session, done: respond)

        // iPhone app (personal build): whether this Mac sends notifications, and which this phone gets.
        case ("GET", "/notifications"):
            respond(.json(DeckRouter.notifications(phone.push, hasKey: deck.pushes.hasKey)))

        // iPhone app (personal build): the phone's device token and its choices, never over the Wi-Fi.
        case ("POST", "/notifications"):
            guard DeckRouter.takesPushToken(request, phone: phone) else {
                return respond(.status(403, "Notifications are set up only over Tailscale's HTTPS address."))
            }
            // No token, as the phone sends when it unpairs: nothing more goes to it.
            let push = DeckRouter.pushTarget(request.json)
            deck.access.phones.setPush(push, for: phone.id)
            respond(.json(DeckRouter.notifications(push, hasKey: deck.pushes.hasKey)))

        // ✨ Summary: a few lines on what the session has been doing, or why there are none.
        case ("POST", "/summary"):
            deck.summarize(session) { result in
                switch result {
                case let .success(text): respond(.json(["summary": text]))
                case let .failure(error): respond(.json(["error": error.message], status: error.status))
                }
            }

        default:
            respond(.status(404))
        }
    }

    // MARK: Setup page

    /// POST /bots/<id>/event, with {"type": "report"}, "seen", "start" or
    /// "stop" (or ?type=): what a cloud bot's desk is told by hand. A report
    /// may say what it was, how many and when, for the desk's card:
    /// {"type": "report", "text": "Sent a report", "count": 3, "at": "<ISO 8601>"}.
    /// T3 Code is off in Settings (T796): none of its routes answers, and `t3` is never run.
    static let t3Off = "T3 Code is off: turn it on in Settings."

    /// T3 Code's state, checked off the main thread (it runs `t3 --version`), answered on it.
    static func t3Status(fresh: Bool, done: @escaping (T3Code.Status) -> Void) {
        DispatchQueue.global(qos: .userInitiated).async {
            let status = T3Code.status(fresh: fresh)
            DispatchQueue.main.async { done(status) }
        }
    }

    /// A hire's folder opened in T3 Code, off the main thread: a hire still on the team, or a 404; and once at a
    /// time for a hire, a 409 while it's being opened.
    static func openInT3Code(_ hire: FrontDesk.Hire?, respond: @escaping (Response) -> Void) {
        guard let hire, hire.state != .fired, hire.state != .failed else { return respond(.status(404, "That hire isn't on the team.")) }
        DispatchQueue.global(qos: .userInitiated).async {
            let answer = T3Code.openOnce(hire)
            DispatchQueue.main.async { respond(answer) }
        }
    }

    private func botEvent(_ request: Request) -> Response {
        let parts = request.path.split(separator: "/", omittingEmptySubsequences: false)
        guard request.method == "POST", parts.count == 4, parts[3] == "event" else { return .status(404) }
        let type = request.json["type"] as? String ?? request.query["type"] ?? ""
        guard let event = CloudDesk.Event(rawValue: type) else {
            return .status(400, "Say what happened: \(CloudDesk.Event.allCases.map(\.rawValue).joined(separator: ", ")).")
        }
        var action: CloudAction?
        if event == .report {
            switch CloudAction.parse(request.json, now: Date()) {
            case .success(let given): action = given
            case .failure(let error): return .status(400, error.message)
            }
        }
        guard deck.cloud.apply(event, to: String(parts[2]), action: action) else { return .status(404, "There's no cloud bot by that id.") }
        return .status(204)
    }

    /// The app's version, as its Info.plist says: "dev" for a build without one.
    static func version(of bundle: Bundle = .main) -> String {
        bundle.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "dev"
    }

    /// Setup is marked complete once a page that got in with the setup key opened it.
    private func completeSetup() {
        let config = Config.shared
        guard !config.hasCompletedSetup else { return }
        config.hasCompletedSetup = true
        config.save()
    }

    private func setup(_ request: Request, caller: SetupSessions.Caller, respond: @escaping (Response) -> Void) {
        let config = Config.shared

        // With the manager off, the GM's routes say so (the version and the switch itself stay open).
        if let refusal = ManagerSwitch.refusal(request.method, request.path, enabled: config.managerEnabled) { return respond(refusal) }

        switch (request.method, request.path) {
        case ("GET", "/setup"):
            // Only a visit that carries the setup key counts as setup: the page
            // opened from the menu hands its key over at /setup/session.
            if caller == .page || caller == .tool { completeSetup() }
            respond(.file(Paths.web.appendingPathComponent("setup.html")))

        case ("GET", "/setup/state"):
            // The Grok Bot app's bots looked for again as setup opens, so a bot made there a moment ago is there.
            deck.discoverBots(fromSetup: true)
            // Personal build: Tailscale's address for this Mac, looked up now and then, read-only.
            // With the setting off, to warn if `tailscale serve` still hands the deck on.
            Tailnet.refresh { [deck] in deck.refresh() }
            respond(.json(setupState(caller)))

        // The deck, for the preview beside the layout editor: the one page a
        // frame may show, and only one of this deck's own.
        case ("GET", "/setup/preview"):
            var preview = Response.file(Paths.web.appendingPathComponent("deck.html"))
            preview.framedBy = .thisDeck
            respond(preview)

        case ("POST", "/setup/layout"):
            guard let layout = Layout.parse(request.body) else {
                return respond(.status(400, "That isn't a layout this deck can use."))
            }
            config.layout = layout
            config.save()
            deck.refresh()
            respond(.status(204))

        // What a file's profile would be, without adding it.
        case ("POST", "/setup/profiles") where request.query["action"] == "check":
            do {
                // No room is said up front, not after the warning.
                guard config.profiles.count < Profile.most else { throw ProfileError.tooMany }
                respond(.json(try Config.checkedProfile(request.body).jsonObject))
            } catch let error as ProfileError {
                respond(.status(400, error.message))
            } catch {
                respond(.status(400, "That isn't a profile this deck can add."))
            }

        case ("POST", "/setup/profiles"):
            do {
                try profiles(request)
                deck.refresh()
                respond(.json(setupState(caller)))
            } catch let error as ProfileError {
                respond(.status(400, error.message))
            } catch {
                respond(.status(400, "Unknown action."))
            }

        case ("GET", "/setup/qr.png"):
            guard let code = deck.access.activeCode else { return respond(.status(404)) }
            // Personal build: ?via=tailnet, the same code at this Mac's tailnet address, for the phone mic over HTTPS.
            var link = deck.pairingLink(for: code)
            if request.query["via"] == "tailnet" {
                guard config.phoneMicEnabled, let tailnet = Tailnet.url else { return respond(.status(404)) }
                link = "\(tailnet)/?pair=\(code)"
            }
            respond(Response(contentType: "image/png", body: QRCode.png(for: link) ?? Data()))

        // The portable office's viewer (private build). Only this reply carries the link,
        // which holds the key; the write secret is in a reply only once, as it's made.
        case ("GET", "/setup/viewer"):
            deck.portable.setupState(withLink: caller == .page) { respond(.json($0)) }

        case ("POST", "/setup/viewer"):
            guard let given = try? JSONSerialization.jsonObject(with: request.body) as? [String: Any] else {
                return respond(.status(400, "That isn't a viewer setting."))
            }
            var viewer = config.viewer
            if let address = given["baseURL"] as? String {
                let trimmed = address.trimmingCharacters(in: .whitespacesAndNewlines)
                if trimmed.isEmpty {
                    viewer.baseURL = ""
                } else {
                    guard let origin = ViewerSettings.origin(of: trimmed) else { return respond(.status(400, ViewerSettings.addressProblem)) }
                    viewer.baseURL = origin
                }
            }
            if let hide = given["hideNames"] as? Bool { viewer.hideNames = hide }
            if let stop = given["stopOnReturn"] as? Bool { viewer.stopOnReturn = stop }
            if let hours = given["stopAfterHours"] as? Int, ViewerSettings.hourChoices.contains(hours) { viewer.stopAfterHours = hours }
            let moved = viewer.baseURL != config.viewer.baseURL
            config.viewer = viewer
            config.save()
            // A new address while it's on: the old viewer is told to forget it, and nothing more goes there.
            if moved { deck.portable.stop() }
            deck.portable.changed()
            deck.portable.whenKeychainKnown { [deck] in respond(.json(deck.portable.setupState())) }

        case ("POST", "/setup/viewer/secret"):
            deck.portable.makeWriteSecret { [deck] secret in
                guard let secret else {
                    return respond(.status(500, "Couldn't keep the write secret in the Keychain."))
                }
                var state = deck.portable.setupState()
                state["secret"] = secret
                respond(.json(state))
            }

        case ("POST", "/setup/viewer/check"):
            deck.portable.check { [deck] in respond(.json(deck.portable.setupState())) }

        case ("POST", "/setup/viewer/switch"):
            if request.query["on"] == "1" {
                deck.portable.start { [deck] in respond(.json(deck.portable.setupState())) }
            } else {
                deck.portable.stop()
                deck.portable.whenKeychainKnown { [deck] in respond(.json(deck.portable.setupState())) }
            }

        case ("POST", "/setup/viewer/rotate"):
            deck.portable.rotateKey { [deck] in respond(.json(deck.portable.setupState())) }

        case ("POST", "/setup/viewer/wipe"):
            deck.portable.wipe { [deck] in respond(.json(deck.portable.setupState())) }

        case ("GET", "/setup/viewer/qr.png"):
            deck.portable.currentLink { link in
                guard let link else { return respond(.status(404)) }
                respond(Response(contentType: "image/png", body: QRCode.png(for: link) ?? Data()))
            }

        // The portable office's viewer (beta). Only this reply carries the link,
        // which holds the key; the write secret is in a reply only once, as it's made.
        case ("GET", "/setup/viewer"):
            deck.portable.setupState(withLink: caller == .page) { respond(.json($0)) }

        case ("POST", "/setup/viewer"):
            guard let given = try? JSONSerialization.jsonObject(with: request.body) as? [String: Any] else {
                return respond(.status(400, "That isn't a viewer setting."))
            }
            var viewer = config.viewer
            if let address = given["baseURL"] as? String {
                let trimmed = address.trimmingCharacters(in: .whitespacesAndNewlines)
                if trimmed.isEmpty {
                    viewer.baseURL = ""
                } else {
                    guard let origin = ViewerSettings.origin(of: trimmed) else { return respond(.status(400, ViewerSettings.addressProblem)) }
                    viewer.baseURL = origin
                }
            }
            if let hide = given["hideNames"] as? Bool { viewer.hideNames = hide }
            if let stop = given["stopOnReturn"] as? Bool { viewer.stopOnReturn = stop }
            if let hours = given["stopAfterHours"] as? Int, ViewerSettings.hourChoices.contains(hours) { viewer.stopAfterHours = hours }
            let moved = viewer.baseURL != config.viewer.baseURL
            config.viewer = viewer
            config.save()
            // A new address while it's on: the old viewer is told to forget it, and nothing more goes there.
            if moved { deck.portable.stop() }
            deck.portable.changed()
            deck.portable.whenKeychainKnown { [deck] in respond(.json(deck.portable.setupState())) }

        case ("POST", "/setup/viewer/secret"):
            deck.portable.makeWriteSecret { [deck] secret in
                guard let secret else {
                    return respond(.status(500, "Couldn't keep the write secret in the Keychain."))
                }
                var state = deck.portable.setupState()
                state["secret"] = secret
                respond(.json(state))
            }

        case ("POST", "/setup/viewer/check"):
            deck.portable.check { [deck] in respond(.json(deck.portable.setupState())) }

        case ("POST", "/setup/viewer/switch"):
            if request.query["on"] == "1" {
                deck.portable.start { [deck] in respond(.json(deck.portable.setupState())) }
            } else {
                deck.portable.stop()
                deck.portable.whenKeychainKnown { [deck] in respond(.json(deck.portable.setupState())) }
            }

        case ("POST", "/setup/viewer/rotate"):
            deck.portable.rotateKey { [deck] in respond(.json(deck.portable.setupState())) }

        case ("POST", "/setup/viewer/wipe"):
            deck.portable.wipe { [deck] in respond(.json(deck.portable.setupState())) }

        case ("GET", "/setup/viewer/qr.png"):
            deck.portable.currentLink { link in
                guard let link else { return respond(.status(404)) }
                respond(Response(contentType: "image/png", body: QRCode.png(for: link) ?? Data()))
            }

        case ("GET", let path) where path.hasPrefix("/setup/logo/"):
            guard let logo = Logo(rawValue: (path as NSString).lastPathComponent) else { return respond(.status(404)) }
            respond(.file(logo.icon))

        case ("POST", "/setup/pair"):
            deck.access.newCode()
            respond(.json(setupState(caller)))

        case ("POST", "/setup/unpair"):
            if request.query["all"] == "1" {
                deck.unpairAll()
            } else {
                guard let id = request.query["phone"], !id.isEmpty else { return respond(.status(400)) }
                deck.unpair(id)
            }
            respond(.json(setupState(caller)))

        // The cloud bots, each at a desk of its own: all of them, as a list, replaced at once.
        case ("GET", "/setup/bots"):
            respond(.json(["bots": cloudBotsJSON()]))

        case ("POST", "/setup/bots"):
            let body = try? JSONSerialization.jsonObject(with: request.body)
            let list = (body as? [String: Any])?["bots"] ?? body
            guard let list = list as? [Any], let data = try? JSONSerialization.data(withJSONObject: list),
                  let given = try? JSONDecoder().decode([CloudBot].self, from: data) else {
                return respond(.status(400, "Send the bots as a list: {\"bots\": [{\"id\": …, \"name\": …}]}."))
            }
            // A bot set or changed here is yours: kept whatever the Grok Bot app says.
            let before = config.cloudBots
            config.cloudBots = CloudBot.checked(given).map { bot in
                // The mailbox is the page's to choose (/setup/bots/mailbox): what is sent here never sets or changes it.
                var bot = bot
                bot.mailbox = before.first(where: { $0.id == bot.id })?.mailbox ?? ""
                guard let old = before.first(where: { $0.id == bot.id }) else { return bot }
                if !bot.pinned && !old.sameAsSet(bot) { bot.pinned = true }
                return bot
            }
            config.save()
            deck.cloud.configure(config.cloudBots)
            deck.refresh()
            respond(.json(["bots": cloudBotsJSON()]))

        // One bot pinned or not, put on a floor, or hidden, from setup's list; or a hidden one shown again, by its
        // app id; or the Grok Bot app's bots found by themselves, or not; or the floor new bots go to.
        case ("POST", "/setup/bots/override"):
            let given = (try? JSONSerialization.jsonObject(with: request.body)) as? [String: Any] ?? [:]
            // A floor for new bots that isn't a whole number changes nothing, as on the Windows and Linux app.
            let newBots = DeckRouter.botFloor(given["botFloor"])
            guard !newBots.refused else {
                return respond(.status(400, "Send {\"botFloor\": …} as a whole number: 0 for the ground floor up to 5 for 6F."))
            }
            if let on = given["autoDetect"] as? Bool {
                config.botsAutoDetect = on
            }
            if let floor = newBots.floor {
                config.botFloor = floor
            }
            if let shown = (given["show"] as? String).flatMap(GrokBotApp.safeID) {
                config.hiddenBots.removeAll { $0.lowercased() == shown.lowercased() }
            }
            if let id = given["id"] as? String {
                guard let bot = config.cloudBots.first(where: { $0.id == id }) else { return respond(.status(404, "That bot isn't there any more.")) }
                if given["hidden"] as? Bool == true {
                    // By its app id, found by its name for one set before ids, or the app would give it back at the next look.
                    if let app = bot.appID(in: GrokBotApp.rows() ?? deck.lastRoster), !config.hiddenBots.contains(where: { $0.lowercased() == app.lowercased() }) {
                        config.hiddenBots.append(app)
                    }
                    config.cloudBots.removeAll { $0.id == id }
                }
                config.cloudBots = config.cloudBots.map { b in
                    guard b.id == id else { return b }
                    var b = b
                    if let pinned = given["pinned"] as? Bool { b.pinned = pinned }
                    if let floor = given["floor"] as? String, CloudBot.floors.contains(floor) { b.floor = floor }
                    return b
                }
            }
            config.save()
            deck.cloud.configure(config.cloudBots)
            deck.discoverBots()
            deck.refresh()
            respond(.json(setupState(caller)))

        // A bot's mailbox folder (T404), page only: {id, folder}; an empty folder takes it away.
        case ("POST", "/setup/bots/mailbox"):
            let given = (try? JSONSerialization.jsonObject(with: request.body)) as? [String: Any] ?? [:]
            guard let id = given["id"] as? String, config.cloudBots.contains(where: { $0.id == id }), let folder = given["folder"] as? String else {
                return respond(.status(400, "Send {id, folder}: the bot, and the folder its mail goes through (empty for none)."))
            }
            var kept = ""
            if !folder.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                switch BotMailbox.chosen(folder, barred: deck.mailboxBarred) {
                case .failure(let why): return respond(.status(400, why.message))
                case .success(let path): kept = path
                }
            }
            config.cloudBots = config.cloudBots.map { bot in
                guard bot.id == id else { return bot }
                var bot = bot
                bot.mailbox = kept
                return bot
            }
            config.save()
            deck.cloud.configure(config.cloudBots)
            deck.refresh()
            respond(.json(["bots": cloudBotsJSON()]))

        // Each Grok Bot without its id given it from the Grok Bot app, by its
        // name or the one like it, so a rename there no longer loses it.
        case ("POST", "/setup/bots/resolve"):
            var filled: [String] = [], unresolved: [String] = []
            config.cloudBots = config.cloudBots.map { bot in
                guard bot.source.kind == "grokBotApp", bot.source.botId.isEmpty else { return bot }
                guard let found = GrokBotApp.resolve(bot.source) else {
                    unresolved.append(bot.id)
                    return bot
                }
                var bot = bot
                bot.source.botId = found.botId
                bot.source.bot = found.name
                filled.append(bot.id)
                return bot
            }
            if !filled.isEmpty {
                config.save()
                deck.cloud.configure(config.cloudBots)
                deck.refresh()
            }
            respond(.json(["bots": cloudBotsJSON(), "filled": filled, "unresolved": unresolved]))

        // The six floors' names and styles, all of them, replaced at once and checked as they're kept;
        // and the folders whose sessions sit on each, with "rules", which a setup page from before them leaves out.
        case ("POST", "/setup/floors"):
            let rules = Floor.rules(request.body)
            let floors = Floor.sent(request.body)
            guard floors != nil || rules != nil else {
                return respond(.status(400, "Send the floors as a list: {\"floors\": [{\"name\": …, \"preset\": …}]}."))
            }
            if let floors { config.floors = floors }
            if let rules { config.floorRules = rules }
            config.save()
            deck.refresh()
            respond(.json(setupState(caller)))

        case ("POST", "/setup/pin"):
            guard let index = keyIndex(request), let folder = request.query["folder"], !folder.isEmpty else {
                return respond(.status(400))
            }
            config.pin(folder, to: index)
            deck.refresh()
            respond(.status(204))

        case ("POST", "/setup/unpin"):
            guard let index = keyIndex(request) else { return respond(.status(400)) }
            config.unpin(index)
            deck.refresh()
            respond(.status(204))

        case ("POST", "/setup/theme"):
            config.colours = Theme.customisations(from: request.json)
            config.save()
            deck.refresh()
            respond(.status(204))

        case ("POST", "/setup/logo"):
            guard let logo = Logo(rawValue: request.query["logo"] ?? "") else { return respond(.status(400)) }
            config.logo = logo
            config.save()
            deck.refresh()
            respond(.json(setupState(caller)))

        case ("POST", "/setup/scroll"):
            config.scrollEnabled = request.query["enabled"] == "1"
            config.save()
            deck.refresh()
            respond(.json(setupState(caller)))

        // Every part is checked before any is kept, so a wrong one changes nothing.
        case ("POST", "/setup/webhook"):
            guard let given = try? JSONSerialization.jsonObject(with: request.body) as? [String: Any] else {
                return respond(.status(400, "That isn't a webhook this deck can use."))
            }
            var webhook = config.webhook
            if let enabled = given["enabled"] as? Bool { webhook.enabled = enabled }
            if let url = given["url"] as? String { webhook.url = url.trimmingCharacters(in: .whitespaces) }
            if let secret = given["secret"] as? String { webhook.secret = secret }
            if let outside = given["outside"] as? Bool { webhook.outside = outside }
            if let events = given["events"] as? [String] { webhook.events = Webhook.allEvents.filter { events.contains($0) } }
            if webhook.enabled, !webhook.url.isEmpty, let problem = Webhook.problem(with: webhook.url, outside: webhook.outside) {
                return respond(.status(400, problem))
            }
            config.webhook = webhook
            config.save()
            respond(.json(setupState(caller)))

        case ("POST", "/setup/webhook/test"):
            deck.testWebhook { [self] in respond(.json(setupState(caller))) }

        // iPhone app (personal build): the GM session's own short message, to the phones that want it.
        // Only its title and body go, on a line each; nothing is sent without the push key.
        case ("POST", "/setup/notify"):
            guard let note = PushNote.gm(title: request.json["title"] as? String ?? "", body: request.json["body"] as? String ?? "") else {
                return respond(.status(400, "Give it a title or a body."))
            }
            guard deck.pushes.hasKey else {
                return respond(.status(503, "Notifications need a push key in the Keychain (\(PushKey.service)), with your team and app as its comment: see the README."))
            }
            deck.push(note) { sent in respond(.json(["sent": sent])) }

        // ✨ Summary on only once the page's dialog has said what it sends where,
        // and only with what it names working; off at any time.
        case ("POST", "/setup/summaries"):
            if request.query["enabled"] == "1" {
                guard request.query["confirmed"] == "1" else {
                    return respond(.status(400, "Turn Summary on from its dialog, which says what it sends where."))
                }
                guard let provider = request.query["provider"].flatMap(SummaryProvider.init(rawValue:)) else {
                    return respond(.status(400, "Pick what makes summaries first."))
                }
                if let refusal = DeckRouter.summaryRefusal(provider) { return respond(.status(400, refusal)) }
                config.summaryProvider = provider.rawValue
                config.summaryOptIn = true
                deck.summaries.forget()
            } else {
                config.summaryOptIn = false
                deck.summaries.forget()
            }
            config.save()
            deck.refresh()
            respond(.json(setupState(caller)))

        // The GM's feed: the events after `since`, at once if there are any, or as
        // soon as one comes, or none after `wait` seconds (30 at most). The page
        // and a tool may both read it for now.
        case ("GET", "/setup/gm/events"):
            let ask = EventLog.question(request.query)
            let log = deck.events
            log.wait(since: ask.since, limit: ask.limit, timeout: ask.wait) { found in
                respond(.json(log.answer(found, since: ask.since)))
            }

        // Phone microphone (personal build): on or off, and whether its text is sent.
        case ("POST", "/setup/phonemic"):
            if let enabled = request.query["enabled"] { config.phoneMicEnabled = enabled == "1" }
            if let send = request.query["autosend"] { config.phoneMicAutoSend = send == "1" }
            config.save()
            deck.refresh()
            respond(.json(setupState(caller)))

        // Whether the office asks before max effort: the phones are told, as it's theirs to ask.
        case ("POST", "/setup/maxconfirm"):
            config.maxConfirm = request.query["enabled"] == "1"
            config.save()
            deck.refresh()
            respond(.json(setupState(caller)))

        // In the Dock and ⌘-Tab, or the menu bar only: at once, and kept.
        case ("POST", "/setup/dock"):
            config.showInDock = request.query["enabled"] == "1"
            config.save()
            Dock.show(config.showInDock)
            respond(.json(setupState(caller)))

        // The office's window kept on top of other windows: at once, and kept.
        case ("POST", "/setup/ontop"):
            config.officeOnTop = request.query["enabled"] == "1"
            config.save()
            officeOptionsChanged()
            respond(.json(setupState(caller)))

        // What makes summaries: picked while they're on, only from the dialog,
        // as it changes where they're sent; and only one that works.
        case ("POST", "/setup/summary/provider"):
            guard let provider = request.query["id"].flatMap(SummaryProvider.init(rawValue:)) else {
                return respond(.status(400, "No such way to make summaries."))
            }
            if config.summaryOptIn, request.query["confirmed"] != "1" {
                return respond(.status(400, "Change it from its dialog, which says where summaries are sent."))
            }
            // Whether there's a key is asked of the Keychain off the main queue first.
            SummaryKey.whenKnown { [self, deck] in
                if let refusal = DeckRouter.summaryRefusal(provider) { return respond(.status(400, refusal)) }
                config.summaryProvider = provider.rawValue
                config.save()
                deck.summaries.forget()
                deck.refresh()
                respond(.json(setupState(caller)))
            }

        // The coding agents looked for again, as setup's Check again asks.
        case ("POST", "/setup/summary/check"):
            SummaryProviders.shared.refresh(force: true)
            respond(.json(setupState(caller)))

        // The key Summary uses: kept in the Keychain, and answered only as where it is.
        case ("POST", "/setup/summary/key"):
            // The Keychain may stop to ask, so it's asked off the main queue and the reply waits for it;
            // everything else keeps answering meanwhile.
            let finish = { [self, deck] in
                deck.summaries.forget()
                deck.refresh()
                respond(.json(setupState(caller)))
            }
            if request.query["remove"] == "1" {
                SummaryKey.remove {
                    // With no key left, summaries made with it are off: nothing to send with, and never sent elsewhere instead.
                    if !SummaryKey.isPresent, ["", SummaryProvider.apiKey.rawValue].contains(config.summaryProvider) {
                        config.summaryOptIn = false
                        config.save()
                    }
                    finish()
                }
            } else {
                let given = (try? JSONSerialization.jsonObject(with: request.body) as? [String: Any])?["key"] as? String ?? ""
                SummaryKey.save(given) { saved in
                    guard saved else {
                        return respond(.status(400, "That isn't an Anthropic API key. It starts with sk-ant-."))
                    }
                    finish()
                }
            }

        // The first-run guide, finished or skipped (done=1), or to be shown again (done=0).
        case ("POST", "/setup/guide"):
            DeckRouter.saveGuide(request, config: config)
            respond(.json(setupState(caller)))

        case ("POST", "/setup/music"):
            config.musicEnabled = request.query["enabled"] == "1"
            config.save()
            deck.followMusic()
            deck.refresh()
            respond(.json(setupState(caller)))

        // Every part is checked before any is kept, so a wrong one changes
        // nothing. An empty folder clears it.
        case ("POST", "/setup/hiring"):
            var folder = request.query["folder"]
            if let given = folder {
                if given.trimmingCharacters(in: .whitespaces).isEmpty {
                    folder = ""
                } else {
                    guard let checked = Deck.folder(given) else { return respond(.status(400, "There's no folder at \(given).")) }
                    folder = checked
                }
            }
            let agent = request.query["agent"]
            if let agent, Agent(rawValue: agent) == nil { return respond(.status(400, "Unknown agent")) }
            let terminal = request.query["terminal"]
            if let terminal, !Openers.isInstalled(terminal) { return respond(.status(400, "That terminal isn't on this Mac")) }
            let tmux = request.query["tmux"]
            if tmux == "1", Tmux.path == nil { return respond(.status(400, "tmux isn't on this Mac")) }
            // Turning hiring on, or changing the folder while it's on, needs a folder to hire into.
            if request.query["enabled"] != nil || folder != nil {
                let enabled = request.query["enabled"].map { $0 == "1" } ?? config.hiringEnabled
                if let refusal = Deck.hiringRefusal(enabled: enabled, hireFolder: folder ?? config.hireFolder, config: config,
                                                    canHireHere: Deck.canHireHere()) {
                    return respond(.status(400, refusal))
                }
            }
            if let enabled = request.query["enabled"] { config.hiringEnabled = enabled == "1" }
            if let folder { config.hireFolder = folder }
            if let agent { config.hireAgent = agent }
            if let terminal { config.hireTerminal = terminal }
            if let tmux { config.hireInTmux = tmux == "1" }
            config.save()
            deck.refresh()
            // A folder typed while the Mac's folder window is open closes that window, as on Start here.
            if folder?.isEmpty == false { deck.closeHireFolderPanel() }
            respond(.json(setupState(caller)))

        // The two plain choices (T796), from first setup or Settings: T3 Code on or off, agent windows
        // (background=1 runs hires in tmux with no window), and asked=1 once first setup has asked them.
        // Switching to the background detaches each hire's windows and closes those Smolcorp opened.
        case ("POST", "/setup/choices"):
            let background = request.query["background"]
            if background == "1", Tmux.path == nil { return respond(.status(400, "Running hires in the background needs tmux, which isn't on this Mac.")) }
            let wasBackground = Choices.background(config)
            if let t3 = request.query["t3"] { config.t3CodeEnabled = t3 == "1" }
            if let background { config.hiresInBackground = background == "1" }
            if request.query["asked"] == "1" { config.choicesAsked = true }
            config.save()
            deck.refresh()
            guard !wasBackground, Choices.background(config) else { return respond(.json(setupState(caller))) }
            deck.hiresToBackground { [self] said in
                var answer = setupState(caller)
                answer["windowsSaid"] = said
                respond(.json(answer))
            }

        // "Open a window" on a hire's desk in Setup's team list (T796): by the hire's id, from the registry.
        case ("POST", "/setup/hire/window"):
            deck.openHireWindow(request.json["id"] as? String ?? "") { problem in
                respond(problem.map { .status(409, $0) } ?? .status(204))
            }

        // Dictation: turned on, its language, and whether Claude Code uses it. Turning it on
        // asks for the microphone, here on the Mac, and gets the language's model ready.
        case ("POST", "/setup/dictation"):
            if let language = request.query["language"] {
                guard Dictation.languages.contains(language) else { return respond(.status(400, "Unknown language")) }
                config.dictationLanguage = language
            }
            if let claude = request.query["claude"] { config.dictationForClaude = claude == "1" }
            // Personal build: Claude's clean-up of the text, before it's typed.
            if let cleanup = request.query["cleanup"] { Deck.setCleanup(cleanup == "1") }
            if let enabled = request.query["enabled"] { config.dictationEnabled = enabled == "1" && Dictation.isAvailable }
            config.save()
            deck.refresh()
            guard config.dictationEnabled else { return respond(.json(setupState(caller))) }
            Dictation.askForMicrophone { [self] _ in
                Dictation.prepare(config.dictationLanguage) { [self] problem in
                    DeckRouter.dictationProblem = problem
                    deck.refresh()
                    respond(.json(setupState(caller)))
                }
            }

        // Personal build: the names dictation's clean-up knows, as the page's lines.
        case ("POST", "/setup/dictation/glossary"):
            guard let lines = try? JSONDecoder().decode([String].self, from: request.body) else {
                return respond(.status(400, "That isn't a list of names."))
            }
            config.dictationGlossary = DictationCleanup.checked(lines)
            config.save()
            respond(.json(setupState(caller)))

        // The roles as the page has them, all of them: checked as they're kept,
        // since their instructions go to agents.
        case ("POST", "/setup/roles"):
            switch BriefRoutes.roles(request.body, saved: config.roles) {
            case .failure(let refused): return respond(refused.response)
            case .success(let roles): config.roles = roles
            }
            config.save()
            deck.refresh()
            respond(.json(setupState(caller)))

        // The briefs, and hiring from setup as from the phone.
        case (_, let path) where BriefRoutes.handles(request.method, path):
            BriefRoutes.handle(request, config: config, library: .editable(), caller: caller,
                                gmHired: BriefRoutes.gmIsHired(among: deck.frontDesk.hires), hire: { [deck] hire, done in
                deck.hire(as: hire.role, in: hire.folder, answers: hire.answers, floor: hire.floor, account: hire.account, options: hire.options,
                           task: hire.task, done: done)
            }, respond: respond)

        // The front desk: the team, and a hire's nickname, state and notes. The page or a tool may ask (T204 narrows it).
        case ("GET", "/setup/gm/team"):
            respond(.json(T3Desk.decorate(deck.frontDesk.teamJSON(live: deck.frontDeskLive(), byHandGM: deck.byHandGMSession), hires: deck.frontDesk.hires)))

        // Who waits for you, and on what. Read only: a prompt is answered by your tap alone (the phone's /act and /choose).
        case ("GET", "/setup/gm/approvals"):
            respond(.json(Approvals.answer(deck.approvalItems())))

        case ("GET", "/setup/gm/version"):
            respond(.json(ManagerSwitch.versionJSON(version: DeckRouter.version(), enabled: config.managerEnabled)))

        // The manager's switch, from the GM or the page. Off never fires anyone.
        case ("POST", "/setup/gm/manager"):
            guard let enabled = request.query["enabled"], enabled == "1" || enabled == "0" else {
                return respond(.status(400, "Say enabled=1 or enabled=0."))
            }
            config.managerEnabled = enabled == "1"
            config.save()
            respond(.json(ManagerSwitch.stateJSON(enabled: config.managerEnabled)))

        // Fire: refuses unsaved work or an open question, as the office's fire does (see `HireDesk.fire`).
        case ("POST", "/setup/gm/fire"):
            deck.fire(body: request.body, done: respond)

        // A later part of a brief for a hire already working: added to its BRIEF file, and the hire is mailed to read it.
        case ("POST", "/setup/gm/brief"):
            respond(deck.briefMore(body: request.body, from: caller == .gm ? .gm : .owner))

        // Mail: from the GM or the page, to a hire. The outbox's texts go to those two only.
        case ("POST", "/setup/gm/send"):
            // A hire is whoever its send key was made for, found again here, never by anything it says.
            let signer = caller == .hire ? deck.frontDesk.hire(forSendKey: request.headers[FrontDesk.sendKeyHeader]) : nil
            respond(deck.mail.send(from: caller == .gm ? .gm : caller == .hire ? .hire : .owner, signer: signer, body: request.body))

        // Which hires may write to which: the GM or the page sets it.
        case ("POST", "/setup/gm/contacts"):
            respond(deck.frontDesk.setContacts(body: request.body))

        case ("GET", "/setup/gm/outbox"):
            respond(.json(deck.mail.outboxJSON(limit: Int(request.query["limit"] ?? "") ?? 100, includeText: caller != .tool)))

        // Watches: what the GM asks the app to keep an eye on. Firings come to its inbox (the event log).
        case ("GET", "/setup/gm/watches"):
            respond(.json(["watches": deck.watchers.json]))

        case ("POST", "/setup/gm/watch"):
            guard let json = (try? JSONSerialization.jsonObject(with: request.body)) as? [String: Any] else {
                return respond(.status(400, "Send {kind, target, pattern?, minutes?, repeat?}. Kinds: file, event, idle, exit, time."))
            }
            do {
                let watch = try deck.watchers.add(json, by: caller == .gm ? "gm" : "owner")
                respond(.json(deck.watchers.json.first { $0["id"] as? String == watch.id } ?? [:]))
            } catch let refusal as FrontDesk.Refusal {
                respond(.status(refusal.status, refusal.message))
            } catch {
                respond(.status(500, "That watch couldn't be set."))
            }

        case ("POST", "/setup/gm/unwatch"), ("DELETE", "/setup/gm/unwatch"):
            let given = (try? JSONSerialization.jsonObject(with: request.body)) as? [String: Any]
            guard let id = request.query["id"] ?? given?["id"] as? String, !id.isEmpty else {
                return respond(.status(400, "Say which watch, as id."))
            }
            do {
                try deck.watchers.remove(id)
                respond(.json(["id": id, "removed": true]))
            } catch let refusal as FrontDesk.Refusal {
                respond(.status(refusal.status, refusal.message))
            } catch {
                respond(.status(500, "That watch couldn't be taken away."))
            }

        // The screens, and which one holds the team (asked about only where there are two or more).
        case ("GET", "/setup/gm/screens"):
            respond(.json(Tiler.screensJSON(Tiler.screens(), chosen: config.teamScreen)))

        case ("POST", "/setup/gm/screen"):
            guard let id = request.query["id"], !id.isEmpty else { return respond(.status(400, "Say which screen: id=<id> from /setup/gm/screens.")) }
            let screens = Tiler.screens()
            guard screens.contains(where: { $0.id == id }) else { return respond(.status(409, "No screen has that id; read /setup/gm/screens.")) }
            config.teamScreen = id
            config.save()
            respond(.json(Tiler.screensJSON(screens, chosen: id)))

        // Tiling: each hire's viewer window over its desk, or a read-only tmux grid where a terminal can't place windows.
        case ("POST", "/setup/gm/tile"):
            let seats = Tiler.seats(fromTeam: deck.frontDesk.teamJSON(live: deck.frontDeskLive())["team"] as? [[String: Any]] ?? [])
            let screens = Tiler.screens()
            guard let screen = Tiler.teamScreen(among: screens, chosen: config.teamScreen) else { return respond(.status(409, "No screen was found to tile on.")) }
            let mainHeight = Tiler.mainHeight(of: screens)
            let grid = Tiler.grid(on: screen, mainHeight: mainHeight)
            let terminal = config.hireTerminal
            // Still on Terminal's queue, while iTerm2 and Ghostty hires now open their windows on their own
            // terminal's queue: a tile asked right after such a hire can run before its window exists. For the
            // follow-up that moves Tiler and `hiresToBackground` off Terminal's queue.
            Terminal.queue.async { [placeOffice] in
                let tmuxPath = Tmux.path
                let report = Tiler.tile(
                    seats: seats, terminal: terminal, grid: grid, tmux: { HireTmux(socketName: URL(fileURLWithPath: $0).lastPathComponent) },
                    script: Tiler.osascript,
                    teamGrid: { hires in
                        guard let tmuxPath else { return false }
                        return TeamGrid.show(hires: hires, terminal: terminal, tmuxPath: tmuxPath, run: { Tool.run(tmuxPath, $0) },
                                             open: { Openers.show(line: $0, terminal: terminal) })
                    })
                DispatchQueue.main.async {
                    if let office = report.office, !report.placed.isEmpty { placeOffice(office, mainHeight) }
                    respond(.json(report.json))
                }
            }

        case ("POST", "/setup/gm/registry"):
            respond(deck.frontDesk.edit(body: request.body))

        // The GM names one person in the office: only that project's nickname in the layout changes.
        case ("POST", "/setup/gm/nickname"):
            var layout = config.layout
            let answer = GMNickname.apply(body: request.body, layout: &layout,
                                          places: deck.sessions.all.map { GMNickname.Place(project: $0.project, folder: $0.folder) },
                                          pins: config.pins)
            if answer.status == 200 {
                config.layout = layout
                config.save()
                deck.refresh()
            }
            respond(answer)

        // The Claude accounts Smolcorp follows: a tool may read them, only the page changes them (see `SetupRoutes`).
        case ("GET", "/setup/claude-accounts"):
            respond(.json(ClaudeProfiles.json(ClaudeProfiles.current)))

        case ("POST", "/setup/claude-accounts"):
            respond(setClaudeAccounts(request.body))

        // "Add smolcorp to Terminal": the link to the app's command, made or removed (the page alone).
        case ("POST", "/setup/terminal-command"):
            guard !TestCopy.isOn else { return respond(.status(409, TerminalCommand.Problem.testCopy.localizedDescription)) }
            do {
                if request.query["remove"] != nil { try TerminalCommand.remove() } else { try TerminalCommand.add() }
                respond(.json(setupState(caller)))
            } catch {
                respond(.status(409, error.localizedDescription))
            }

        // Open in T3 Code (T792): what's installed (again now, as its menu opens: fresh=1), and a hire's folder
        // opened there, by the hire's id: the folder comes from the registry, never from the request. The page alone.
        case ("GET", "/setup/t3code"):
            guard Choices.t3Code() else { return respond(.status(404, DeckRouter.t3Off)) }
            let on = T3Code.laneAllowed(config)
            DeckRouter.t3Status(fresh: request.query["fresh"] == "1") { status in
                var answer = T3Code.json(status)
                // Whether a hire may go into T3 Code (T797b), or the one sentence saying why not.
                let why = T3Desk.refusal(on: on, status: status)
                answer["lane"] = ["ok": why == nil, "why": why ?? ""]
                respond(.json(answer))
            }

        case ("POST", "/setup/t3code/open"):
            guard Choices.t3Code() else { return respond(.status(404, DeckRouter.t3Off)) }
            let id = request.json["id"] as? String ?? ""
            DeckRouter.openInT3Code(deck.frontDesk.hires.first { $0.id == id }, respond: respond)

        // Let go of a hire in T3 Code (T797b), by its id after the page's confirm: its T3 project removed, its worktree kept.
        case ("POST", "/setup/t3code/letgo"):
            deck.letGoOfT3(id: request.json["id"] as? String ?? "", done: respond)

        // The hire guard's groups and your own patterns: a tool may read them, only the page changes them (see `SetupRoutes`).
        case ("GET", "/setup/guard"):
            respond(.json(GuardFile.state(Paths.hireGuard)))

        case ("POST", "/setup/guard"):
            switch GuardFile.save(request.body, to: Paths.hireGuard) {
            case .success(let state): respond(.json(state))
            case .failure(.bad(let why)): respond(.status(400, why))
            case .failure(.notWritten): respond(.status(500, "Couldn't write the guard file."))
            }

        // Pressing keys in other apps: on or off from the page alone, and on asks macOS for Accessibility.
        case ("POST", "/setup/keystrokes"):
            guard let enabled = request.query["enabled"], enabled == "1" || enabled == "0" else {
                return respond(.status(400, "Say enabled=1 or enabled=0."))
            }
            let changed = config.keystrokesAllowed != (enabled == "1")
            config.keystrokesAllowed = enabled == "1"
            config.save()
            let on = config.keystrokesAllowed
            // On the main thread: the event log and the Accessibility prompt both belong there.
            DispatchQueue.main.async { [self] in
                if changed {
                    _ = deck.events.append(EventLog.Entry(at: Date().timeIntervalSince1970 * 1000, kind: .keystroke,
                                                          detail: on ? "switched on" : "switched off"))
                }
                if on, !TestCopy.isOn, !Permissions.hasAccessibility { Permissions.requestAccessibility() }
            }
            respond(.json(setupState(caller)))

        // The apps keys may go to: one added (?add=id) or taken off (?remove=id), from the page alone.
        case ("POST", "/setup/keystrokes/apps"):
            let adding = request.query["add"], removing = request.query["remove"]
            guard (adding == nil) != (removing == nil), let id = adding ?? removing else {
                return respond(.status(400, "Say add=<app id> or remove=<app id>."))
            }
            let before = config.keystrokeApps
            if adding != nil {
                switch Keystrokes.AppList.adding(id, to: before) {
                case .success(let list): config.keystrokeApps = list
                case .failure(let problem): return respond(.status(409, problem.reason))
                }
            } else {
                config.keystrokeApps = Keystrokes.AppList.removing(id, from: before)
            }
            if config.keystrokeApps != before {
                config.save()
                let line = Keystrokes.appListChange(added: adding != nil, id: id)
                DispatchQueue.main.async { [self] in
                    _ = deck.events.append(EventLog.Entry(at: Date().timeIntervalSince1970 * 1000, kind: .keystroke, detail: line))
                }
            }
            respond(.json(setupState(caller)))

        // The Mac's own folder picker, which a web page can't open. With `pick=1` it only answers the folder
        // picked (`picked`, empty if none), for Start here to keep through its checks (POST /setup/hire-folder),
        // or with `for=floor`, for Floors to put a project on a floor (POST /setup/floors).
        case ("POST", "/setup/hiring/choose"):
            if request.query["pick"] == "1" {
                let message = request.query["for"] == "floor" ? Deck.floorFolderMessage : Deck.hireFolderMessage
                deck.chooseHireFolder(keep: false, message: message) { picked in respond(.json(["picked": picked ?? ""])) }
            } else {
                deck.chooseHireFolder { [self] _ in respond(.json(setupState(caller))) }
            }

        // Start here's hires folder: the one set, or with none, one suggested; `check=` checks one as it's typed.
        case ("GET", "/setup/hire-folder"):
            respond(HireFolder.route(request, config: config) {})

        // Keeps it, once checked; `create=1` makes it first if it isn't there yet. A problem answers 400 with the same JSON, to show with its fix.
        // A folder typed on the page while the Mac's folder window is open closes that window: it's chosen now.
        case ("POST", "/setup/hire-folder"):
            respond(HireFolder.route(request, config: config) { [self] in
                config.save()
                deck.refresh()
                deck.closeHireFolderPanel()
            })

        case ("POST", "/setup/hooks"):
            var notice: String?
            let disconnecting = request.query["remove"] != nil
            do {
                if disconnecting {
                    try Hooks.disconnect()
                } else {
                    try Hooks.connect(notice: &notice)
                }
                DeckRouter.keepDisconnected(disconnecting, in: config)
                var state = setupState(caller)
                // Connecting turned Claude Code's voice on: the page says so.
                if let notice { state["notice"] = notice }
                respond(.json(state))
            } catch {
                // Some agents were disconnected: that is what was asked, so --setup leaves them be
                if case Hooks.HookError.partlyDisconnected = error { DeckRouter.keepDisconnected(true, in: config) }
                // A test copy refuses, like a hire does; the page shows it inline
                if case Hooks.HookError.testCopy = error { return respond(.status(409, error.localizedDescription)) }
                // Connecting partly failed after turning voice on: the notice still reaches the page
                respond(.status(500, error.localizedDescription + (notice.map { "\n\n" + $0 } ?? "")))
            }

        case ("POST", "/setup/accessibility"):
            Permissions.requestAccessibility()
            respond(.status(204))

        case ("POST", "/setup/login"):
            LoginItem.isEnabled = request.query["enabled"] == "1"
            respond(.json(setupState(caller)))

        // The self-test: Start here's checklist, and --setup's.
        case ("GET", "/setup/selftest"):
            // Tailscale's answer first, if the last one is old or there's none,
            // so the network line is right the first time it's asked.
            Tailnet.whenFresh(changed: { [deck] in deck.refresh() }) { [deck] in
                respond(.json(SelfTest.report(SelfTest.checks(deck: deck))))
            }

        // One of its fixes, by name, and the checklist after it.
        case ("POST", "/setup/fix"):
            // A tool (the --setup command) only repairs; the rest is the page's.
            if let refusal = DeckRouter.fixRefusal(caller: caller, fix: request.query["fix"], auto: request.query["auto"]) { return respond(refusal) }
            // --setup asks with auto=1: its repair leaves agents you disconnected.
            if SelfTest.leavesDisconnected(fix: request.query["fix"] ?? "", auto: request.query["auto"] == "1",
                                           disconnected: config.hooksDisconnected) {
                return respond(.status(409, SelfTest.FixError.disconnected.localizedDescription))
            }
            do {
                try SelfTest.fix(request.query["fix"] ?? "")
                respond(.json(SelfTest.report(SelfTest.checks(deck: deck))))
            } catch SelfTest.FixError.unknown {
                respond(.status(400, SelfTest.FixError.unknown.localizedDescription))
            } catch {
                respond(.status(409, error.localizedDescription))
            }

        // The office's window, from the page.
        case ("POST", "/setup/office"):
            showOffice()
            respond(.status(204))

        // Start here's Hire your General Manager: the GM role, in the folder for your hires (never your home
        // folder: with none set, it's refused and the page points at its folder step), started by the app with
        // its token in its own environment. `worktree=1` gives it a worktree. The manager, and Hire and fire
        // from the office (the GM hires its team), are switched on before the hire and back if it fails.
        // `account=` names the Claude account, as a role card's hire does; with one account it is left out.
        case ("POST", "/setup/hire/gm"):
            // One GM per office: the GM's token never hires the gm role, whatever the registry holds. Checked before anything is saved.
            if let refused = BriefRoutes.gmHireRefusal(role: GMPlugin.role, caller: caller, gmHired: BriefRoutes.gmIsHired(among: deck.frontDesk.hires)) { return respond(refused.response) }
            guard let folder = Deck.folder(config.hireFolder) else { return respond(.status(409, DeckRouter.chooseHireFolderFirst)) }
            if let refusal = deck.gmRefusal() { return respond(.status(refusal.status, refusal.message)) }
            let before = config.managerEnabled, hiring = config.hiringEnabled
            config.managerEnabled = true
            config.hiringEnabled = true
            config.save()
            deck.hire(as: "gm", in: folder, answers: nil, account: request.query["account"] ?? "", options: HireDesk.HireOptions(worktree: request.query["worktree"] == "1")) { response in
                if response.status >= 400 { config.managerEnabled = before; config.hiringEnabled = hiring; config.save() }
                respond(response.status < 400 ? .status(204) : response)
            }

        // What a first General Manager hire needs (GMPreflight), Start here's lines above its Hire button.
        // Outside /setup/gm/, which is closed while the manager is off, as it is on a first run. The page's
        // alone (SetupRoutes): it starts your login shell and Claude Code. An account that isn't there is
        // said, as the hire would say it, rather than checked as the default.
        case ("GET", "/setup/hire/gm/preflight"):
            let account = request.query["account"] ?? ""
            if !account.trimmingCharacters(in: .whitespaces).isEmpty, let why = ClaudeProfiles.hireRefusal(account: account, among: ClaudeProfiles.current) {
                return respond(.status(400, why.message))
            }
            GMPreflight.check(config: config, account: account) { respond(.json($0)) }

        // Whether the GM's token is also kept in a file, for a GM started by hand: the page alone, off by default.
        case ("POST", "/setup/gm/by-hand"):
            guard let enabled = request.query["enabled"], enabled == "1" || enabled == "0" else {
                return respond(.status(400, "Say enabled=1 or enabled=0."))
            }
            config.gmByHand = enabled == "1"
            config.save()
            deck.access.gm.applySetting()
            respond(.json(["gmByHand": config.gmByHand]))

        // A new GM token, in place of the old: the page only. No answer carries the token: it is
        // given to a GM hired afterwards, in its environment, and a running GM can't be given another.
        case ("POST", "/setup/gm/token/rotate"):
            guard deck.access.gm.rotate() != nil else { return respond(.status(500, "The new token couldn't be saved; the old one still works.")) }
            respond(.json(DeckRouter.rotated()))

        default:
            respond(.status(404))
        }
    }

    /// `POST /setup/claude-accounts` with `{profiles: [{name, dir}]}`: the
    /// accounts besides ~/.claude, whole, checked before anything is kept. One
    /// added while Smolcorp is connected is connected too; one taken out has
    /// Smolcorp's hooks taken out of it, and nothing else. A test copy keeps the
    /// list in its own settings (its General Manager needs an account of its own)
    /// and never touches an account's hooks: its hires report through its own.
    func setClaudeAccounts(_ body: Data, testCopy: Bool = TestCopy.isOn) -> Response {
        guard let given = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any], let list = given["profiles"] as? [[String: Any]]
        else { return .status(400, "Send {profiles: [{name, dir}]}.") }
        let asked = list.map { ClaudeProfile(name: $0["name"] as? String ?? "", dir: $0["dir"] as? String ?? "") }
        switch ClaudeProfiles.checked(asked) {
        case .failure(let refused): return .status(400, refused.message)
        case .success(let profiles):
            let config = Config.shared
            let before = config.claudeProfiles
            config.claudeProfiles = profiles
            config.save()
            if testCopy { return .json(ClaudeProfiles.json(ClaudeProfiles.current)) }
            if !config.hooksDisconnected { Hooks.connectAdded(profiles.filter { !before.contains($0) }) }
            for gone in before where !profiles.contains(where: { $0.dir == gone.dir }) { _ = try? Hooks.removeClaude(from: gone) } // writes nothing if none of ours is in it
            return .json(ClaudeProfiles.json(ClaudeProfiles.current))
        }
    }

    /// What `POST /setup/gm/token/rotate` answers: that it was made, never the token, and what to do next.
    static func rotated() -> [String: Any] {
        ["rotated": true, "message": "The old token no longer works. Fire the General Manager and hire it again to give it the new one."]
    }

    /// Switches to, saves, renames, deletes or imports a profile.
    private func profiles(_ request: Request) throws {
        let config = Config.shared
        let name = request.query["name"] ?? ""
        switch request.query["action"] {
        case "switch": try config.switchProfile(to: name)
        case "save": try config.saveProfile(as: name)
        case "rename": try config.renameProfile(name, to: request.query["to"] ?? "")
        case "delete": config.deleteProfile(name)
        case "import": try config.importProfile(request.body)
        default: throw ProfileError(message: "Unknown action.")
        }
    }

    private func keyIndex(_ request: Request) -> Int? {
        guard let index = Int(request.query["key"] ?? ""), Config.shared.pins.indices.contains(index) else { return nil }
        return index
    }

    /// The floor new bots go to as setup sends it: `floor`, 0 for the ground
    /// floor up to 5 for 6F; nil when none is sent, or for a whole number
    /// that's no floor, such as 6, left as it was. Anything else, a true, a
    /// "2" or a 1.5 among them, even a 1.0, is `refused`, as the Windows and
    /// Linux app refuses what it can't read as a whole number.
    static func botFloor(_ given: Any?) -> (floor: Int?, refused: Bool) {
        guard let given, !(given is NSNull) else { return (nil, false) }
        guard let number = given as? NSNumber, CFGetTypeID(number) != CFBooleanGetTypeID(), !CFNumberIsFloatType(number) else {
            return (nil, true)
        }
        return ((0..<Floor.count).contains(number.intValue) ? number.intValue : nil, false)
    }

    /// The bots as kept, each with who it is in its app now, read afresh:
    /// `resolved` is `{botId, name, by}`, or null when it isn't found there.
    private func cloudBotsJSON() -> Any {
        let bots = Config.shared.cloudBots
        let data = (try? JSONEncoder().encode(bots)) ?? Data("[]".utf8)
        let list = (try? JSONSerialization.jsonObject(with: data)) as? [[String: Any]] ?? []
        return zip(bots, list).map { bot, json -> [String: Any] in
            var json = json
            json["resolved"] = bot.source.kind == "grokBotApp" ? GrokBotApp.resolve(bot.source)?.json ?? NSNull() : NSNull()
            return json
        }
    }

    /// Why `provider` can't make summaries now, or nil when it can.
    static func summaryRefusal(_ provider: SummaryProvider) -> String? {
        if provider == .apiKey { return SummaryKey.isPresent ? nil : "Add your Anthropic API key first." }
        guard let state = SummaryProviders.shared.states?[provider] else { return "Still looking for \(provider.name). Try again in a moment." }
        return provider.isSupported && state.works ? nil : "\(provider.name): \(state.reason)."
    }

    /// ✨ Summary for the setup page: on or off, what makes summaries and
    /// each way there is, with why it can't when it can't; where the key is
    /// from (never any part of it), and the model, for the dialog.
    static func summaryState(optIn: Bool, saved: String = "", environment: [String: String] = ProcessInfo.processInfo.environment,
                             states: [SummaryProvider: SummaryProviderState] = [:], checking: Bool = false,
                             fallback: String? = nil) -> [String: Any] {
        let shown = SummaryKey.shown(environment: environment)
        let provider = SummaryChoice.chosen(saved: saved, optIn: optIn, states: states, hasKey: shown != nil)
        let providers: [[String: Any]] = SummaryProvider.agents.map { agent in
            let state = states[agent]
            return ["id": agent.rawValue, "name": agent.name,
                    "state": state.map { $0.works && agent.isSupported ? "ready" : $0.kind.rawValue } ?? "checking",
                    "reason": state?.reason ?? "checking…"]
        } + [["id": SummaryProvider.apiKey.rawValue, "name": SummaryProvider.apiKey.name,
              "state": shown != nil ? "ready" : "missing",
              "reason": shown != nil ? SummaryProvider.apiKey.plan : "add a key below"]]
        return [
            "enabled": optIn && provider != nil,
            "provider": provider.map { ["id": $0.rawValue, "name": $0.name, "plan": states[$0]?.reason ?? $0.plan, "destination": $0.destination,
                                        "works": SummaryChoice.works($0, states: states, hasKey: shown != nil)] as [String: Any] } ?? NSNull(),
            "providers": providers,
            "checking": checking,
            "fallback": fallback ?? NSNull(),
            // Where the key is from, never any part of it, with Summaries on or off.
            "key": shown.map { ["source": $0.source.rawValue] as [String: Any] } ?? NSNull(),
            // Only a key pasted here is a fallback: never one from Smolcorp's environment.
            "savedKey": shown?.source == .keychain,
            // What was picked, for saying when it's one that isn't supported any more.
            "saved": saved,
            "model": SummaryRequest.modelName,
        ]
    }

    /// The manager's part of the setup state. `gmStartedByHand` is true only with the
    /// setting on and a GM that has used the token (see `GMIdentity.startedByHand`).
    /// `gmTokenFile` is where "Allow a General Manager started by hand" writes the token,
    /// for the page to say where (`gmTokenFileShown`, with ~) and copy the path: never the token.
    static func managerState(enabled: Bool, byHand: Bool, startedByHand: Bool, tokenFile: URL = Paths.gmToken) -> [String: Any] {
        ["enabled": enabled, "gmByHand": byHand, "gmStartedByHand": byHand && startedByHand,
         "gmTokenFile": tokenFile.path, "gmTokenFileShown": (tokenFile.path as NSString).abbreviatingWithTildeInPath]
    }

    private func setupState(_ caller: SetupSessions.Caller) -> [String: Any] {
        let config = Config.shared
        let (keys, _) = deck.layout()
        var keyOf: [String: Int] = [:]
        for (index, key) in keys.enumerated() {
            if let session = key.session { keyOf[session.id] = index }
        }

        // Tailscale reaches this Mac only over HTTPS, with the phone mic on (see Tailnet): only then is it said.
        let pairing = SetupSessions.pairing(code: deck.access.activeCode, expiresIn: Int(deck.access.codeExpiry.timeIntervalSinceNow),
                                            address: deck.phoneAddress, tailnet: config.phoneMicEnabled ? Tailnet.url : nil, for: caller)

        return [
            // Which Smolcorp this is, shown at the foot of setup's first pane.
            "version": DeckRouter.version(),
            // Which computer this is, so the page shows its own kind of folder path.
            "platform": "darwin",
            "checklist": [
                "hooks": Hooks.isConnected,
                "accessibility": Permissions.hasAccessibility,
                "scroll": Config.shared.scrollEnabled,
                "openAtLogin": LoginItem.isEnabled,
            ],
            "claudeAccounts": ClaudeProfiles.json(ClaudeProfiles.current),
            "terminalCommand": TerminalCommand.json(),
            "agents": Hooks.found.map { ["id": $0.rawValue, "name": $0.name, "connected": Hooks.isConnected($0)] },
            "hiring": [
                "enabled": config.hiringEnabled, "folder": config.hireFolder, "agent": config.hireAgent,
                // "The default agent can hire here". A role with its own agent can still be refused in a copy (another agent than Claude Code).
                "canHire": Deck.testCopyRefusal(for: Deck.hiredAgent(role: nil, hireAgent: config.hireAgent)) == nil, "canChoose": true,
                // Where hires open: the terminals on this Mac that can open them, and tmux if it's there.
                "terminal": Openers.isInstalled(config.hireTerminal) ? config.hireTerminal : "terminal",
                "terminals": Openers.choices.filter { Openers.isInstalled($0.id) }.map { ["id": $0.id, "name": $0.name] },
                "tmux": config.hireInTmux && Tmux.path != nil, "hasTmux": Tmux.path != nil,
                // The API-key account for a hire: offered only with a key in the Keychain, and picked per hire.
                "apiKey": ["available": HireDesk.apiKeyOffered(), "notice": HireDesk.apiKeyNoticeText()],
            ],
            "dictation": [
                "available": Dictation.isAvailable, "enabled": config.dictationEnabled, "allowed": Dictation.microphoneAllowed,
                "language": config.dictationLanguage, "claude": config.dictationForClaude,
                "cleanup": config.dictationCleanup, "glossary": config.dictationGlossary,
                "cleanupProblem": config.dictationCleanup ? DictationCleanup.problem ?? NSNull() : NSNull(),
                "problem": DeckRouter.dictationProblem ?? NSNull(),
            ] as [String: Any],
            // The roles, those that come with the app to go back to, and what a
            // role can be: its agent, whether that agent has an effort and
            // takes a first message, its badge, and how long each part can be.
            "roles": [
                "roles": config.roles.map(\.jsonObject),
                "shipped": Role.shipped.map(\.jsonObject),
                "agents": Agent.allCases.map { ["id": $0.rawValue, "name": $0.name, "effort": $0.hasEffortDial, "starts": $0 != .grok] },
                "efforts": Deck.effortLevels,
                "badges": Role.badges,
                "limits": ["name": Role.nameLimit, "model": Role.modelLimit, "instructions": Role.instructionsLimit,
                           "questions": Role.questionsMost, "question": Role.questionLimit, "briefFile": Brief.fileLimit,
                           "briefFileName": Brief.fileNameLimit, "answer": BriefContext.answerLimit],
            ] as [String: Any],
            "music": ["enabled": config.musicEnabled],
            "maxConfirm": ["enabled": config.maxConfirm],
            // The first-run guide's arrows: shown until it's finished or skipped (see guide.js).
            "guide": DeckRouter.guideState(config),
            "manager": DeckRouter.managerState(enabled: config.managerEnabled, byHand: config.gmByHand, startedByHand: deck.access.gm.startedByHand),
            // Whether a General Manager is hired, whatever the manager switch says: Start here looks no further
            // then. Top level, as Linux/Windows has no manager object to put it in.
            "gmHired": BriefRoutes.gmIsHired(among: deck.frontDesk.hires),
            "keystrokes": ["enabled": config.keystrokesAllowed, "accessibility": Permissions.hasAccessibility,
                           "apps": config.keystrokeApps, "running": Keystrokes.runningApps(except: config.keystrokeApps)] as [String: Any],
            "dock": ["enabled": config.showInDock],
            // The two plain choices (T796), and the T3 Code gate as T3 Code's own code reads it.
            "choices": Choices.json(config, hasTmux: Tmux.path != nil),
            "t3CodeEnabled": Choices.t3Code(config),
            "officeWindow": ["available": true, "mac": true, "onTop": config.officeOnTop ?? false],
            "summary": { () -> [String: Any] in
                SummaryProviders.shared.refresh()
                return DeckRouter.summaryState(optIn: config.summaryOptIn, saved: config.summaryProvider,
                                               states: SummaryProviders.shared.states ?? [:], checking: SummaryProviders.shared.isChecking,
                                               fallback: deck.summaries.lastFallback)
            }(),
            // Phone microphone (personal build), and this Mac's tailnet address if Tailscale says it.
            "phoneMic": [
                "enabled": config.phoneMicEnabled, "autoSend": config.phoneMicAutoSend,
                "tailnet": (config.phoneMicEnabled ? Tailnet.url : nil).map { $0 as Any } ?? NSNull(),
                "serving": Tailnet.serving.map { $0 as Any } ?? NSNull(),
            ] as [String: Any],
            "webhook": [
                "enabled": config.webhook.enabled, "url": config.webhook.url, "secret": DeckRouter.webhookSecret(config.webhook.secret, for: caller),
                "outside": config.webhook.outside, "events": config.webhook.events, "last": deck.webhookLast ?? NSNull(),
            ],
            "pairing": pairing,
            // Times in milliseconds, as the page's Date counts them.
            "phones": deck.access.phones.all.map { phone -> [String: Any] in
                [
                    "id": phone.id, "name": phone.name,
                    "pairedAt": Int(phone.pairedAt.timeIntervalSince1970 * 1000),
                    "lastSeen": Int(phone.lastSeen.timeIntervalSince1970 * 1000),
                    "connected": deck.isConnected(phone.id),
                ]
            },
            "logo": Config.shared.logo.rawValue,
            // Every key a project can be pinned to, even those the phone
            // doesn't show: with Auto, a pin there adds keys up to it; with a
            // fixed number, a pin left beyond them can still be undone.
            "pins": Config.shared.pins.prefix(KeyCount.most).map { pin -> Any in
                guard let pin else { return NSNull() }
                return ["folder": pin, "name": (pin as NSString).lastPathComponent]
            },
            // How many keys the phone shows now.
            "phoneKeys": keys.count,
            "sessions": deck.sessions.all.map { session -> [String: Any] in
                [
                    "id": session.id, "folder": session.project, "name": session.projectName, "agentName": session.agent.name,
                    "state": session.state.rawValue, "key": keyOf[session.id] ?? NSNull(),
                    // What the setup page's outfit maker dresses them by: their agent, model and role
                    "agent": session.agent.rawValue, "model": session.model.isEmpty ? NSNull() : session.model,
                    "role": deck.roleID(of: session.id) ?? NSNull(),
                ]
            },
            "theme": Theme.editorJSON(),
            "layout": Config.shared.layout.jsonObject,
            // The building's floors, the ground floor first, and the office's styles a floor above it can have,
            // besides the office's own ("") and the server room ("server").
            "floors": Config.shared.floors.map(\.jsonObject),
            // The folders whose sessions sit on a floor of their own.
            "floorRules": Config.shared.floorRules.map(\.jsonObject),
            // The cloud bots, for setup's list: each pinned or found in the Grok Bot app, which this Mac looks in;
            // those you hid, by name where the app still has them; those with no room for them, found or at work;
            // and the floor new bots go to.
            "cloudBots": [
                "detects": true,
                "autoDetect": Config.shared.botsAutoDetect,
                "botFloor": Config.shared.botFloor,
                "bots": Config.shared.cloudBots.map { $0.setupJSON(in: deck.lastRoster) },
                "hidden": Config.shared.hiddenBots.map { id in
                    ["botId": id, "name": deck.lastRoster.first { $0.id.lowercased() == id.lowercased() }?.name ?? NSNull()] as [String: Any]
                },
                "noRoom": deck.botsWithNoRoom.map(\.name) + deck.botsWithNoDesk().map(\.name),
            ] as [String: Any],
            "presets": Room.presets.filter { $0 != "yours" },
            // Saved looks, besides Default, the one the deck was last switched
            // to, and whether it has changed since.
            "profiles": Config.shared.profiles.map(\.jsonObject),
            "activeProfile": Config.shared.activeProfile,
            "profileEdited": Config.shared.profileEdited,
        ]
    }
}
