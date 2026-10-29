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

    /// Why a request for setup is refused, or nil to let it in: it must
    /// come from this Mac, an action must carry the setup header, and
    /// anything but the page itself needs the setup key: the cookie the page
    /// is given as the menu opens it, or the header a tool of your own sends.
    /// The page itself holds nothing.
    static func setupRefusal(_ request: Request, isKey: (String?) -> Bool) -> Response? {
        guard request.isLocal else { return .status(404) }
        guard request.method == "GET" || request.headers[setupHeader] == "1" else { return .status(403) }
        if request.path != "/setup" {
            let key = request.headers[setupKeyHeader] ?? cookie(setupCookie, in: request)
            guard isKey(key) else { return .status(401) }
        }
        return nil
    }

    /// Whom the deck answers past setup and the hook: an IP address or a
    /// local name, as a phone on the network asks; or, with the phone mic on
    /// (personal build), a tailnet name passed on by `tailscale serve`.
    static func admits(_ request: Request, phoneMic: Bool) -> Bool {
        request.isAddressedDirectly || (request.isTailnet && phoneMic)
    }

    /// Personal build: the phone's words are typed, and sent, only from the
    /// deck opened through `tailscale serve`, over HTTPS, where the page uses
    /// the phone mic. Never over the Wi-Fi's plain http, where a key seen on
    /// the network could otherwise type a prompt of its own.
    static func takesDictation(_ request: Request) -> Bool {
        request.isTailnet
    }

    /// Personal build: the peek at an agent's input line shows what its
    /// terminal shows, so it's given only where dictation is taken, through
    /// `tailscale serve`. Never over the Wi-Fi, where a key seen on the
    /// network could otherwise read a terminal.
    static func showsTerminal(_ request: Request) -> Bool {
        request.isTailnet
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
    /// `tailscale serve`, as typed text does, from a phone paired with its own key.
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
    /// nil, for no more notifications, when there's no token that could be one.
    static func pushTarget(_ given: [String: Any]) -> PushTarget? {
        guard let token = given["token"] as? String, PushTarget.isToken(token) else { return nil }
        return PushTarget(token: token, sandbox: given["sandbox"] as? Bool ?? true, finished: given["finished"] as? Bool ?? true,
                          waiting: given["waiting"] as? Bool ?? true, gm: given["gm"] as? Bool ?? true)
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

    /// The setup page's key, as a tool of your own sends it, read from the
    /// setup-key file; or as the page's cookie, from the menu's link.
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
            if let refused = DeckRouter.setupRefusal(request, isKey: deck.access.isSetupKey) { return respond(refused) }
            return setup(request, respond: respond)
        }

        // A cloud bot's news, from a tool of yours: guarded as setup's actions are.
        if request.path.hasPrefix("/bots/") {
            if let refused = DeckRouter.setupRefusal(request, isKey: deck.access.isSetupKey) { return respond(refused) }
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
            let page = DeckRouter.withIcon(Response.file(Paths.web.appendingPathComponent(phone == nil ? "pair.html" : "deck.html")),
                                           icon: request.isTailnet ? Config.shared.logo.tailIcon : Config.shared.logo.icon)
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
            guard DeckRouter.takesDictation(request) else {
                return respond(.status(403, "The phone mic works only over Tailscale's HTTPS address."))
            }
            let text = request.json["text"] as? String ?? ""
            // The confirm box's choice, the deck's or the iPhone app's: Send ⏎ or Type only; an older page's says nothing.
            deck.typePhoneDictation(text, into: session, send: request.json["send"] as? Bool, done: respond)

        // Phone microphone (personal build): the last lines of the session's tab, read-only, to see what was typed.
        case ("POST", "/peek"):
            guard DeckRouter.showsTerminal(request) else {
                return respond(.status(403, "The input line shows only over Tailscale's HTTPS address."))
            }
            deck.peek(session, done: respond)

        // iPhone app (personal build): a typed answer to a menu's "Type something", never over the Wi-Fi.
        case ("POST", "/choose"):
            guard DeckRouter.takesDictation(request) else {
                return respond(.status(403, "Typed answers go only over Tailscale's HTTPS address."))
            }
            guard request.query["option"] == "other" else { return respond(.status(400, "Pick an option with the deck's keys.")) }
            deck.answerOther(request.json["text"] as? String ?? "", of: request.query["choice"], in: session, done: respond)

        // iPhone app (personal build): what a permission prompt is about, its command or file, never over the Wi-Fi.
        case ("GET", "/choice"):
            guard DeckRouter.showsTerminal(request) else {
                return respond(.status(403, "The details show only over Tailscale's HTTPS address."))
            }
            deck.choiceDetail(session, done: respond)

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

    private func setup(_ request: Request, respond: @escaping (Response) -> Void) {
        let config = Config.shared

        switch (request.method, request.path) {
        case ("GET", "/setup"):
            if !config.hasCompletedSetup {
                config.hasCompletedSetup = true
                config.save()
            }
            respond(.file(Paths.web.appendingPathComponent("setup.html")))

        // The menu's link carries the key after #k=. The page hands it over once,
        // already checked above, for a cookie only this page's requests carry.
        case ("POST", "/setup/session"):
            var session = Response.status(204)
            session.headers["Set-Cookie"] = "\(DeckRouter.setupCookie)=\(deck.access.setupKey); Path=/setup; HttpOnly; SameSite=Strict"
            respond(session)

        case ("GET", "/setup/state"):
            // Personal build: Tailscale's address for this Mac, looked up now and then, read-only.
            // With the setting off, to warn if `tailscale serve` still hands the deck on.
            Tailnet.refresh { [deck] in deck.refresh() }
            respond(.json(setupState()))

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
                respond(.json(setupState()))
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
            respond(.json(deck.portable.setupState(withLink: true)))

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
            respond(.json(deck.portable.setupState()))

        case ("POST", "/setup/viewer/secret"):
            guard let secret = deck.portable.makeWriteSecret() else {
                return respond(.status(500, "Couldn't keep the write secret in the Keychain."))
            }
            var state = deck.portable.setupState()
            state["secret"] = secret
            respond(.json(state))

        case ("POST", "/setup/viewer/check"):
            deck.portable.check { [deck] in respond(.json(deck.portable.setupState())) }

        case ("POST", "/setup/viewer/switch"):
            if request.query["on"] == "1" { deck.portable.start() } else { deck.portable.stop() }
            respond(.json(deck.portable.setupState()))

        case ("POST", "/setup/viewer/rotate"):
            deck.portable.rotateKey()
            respond(.json(deck.portable.setupState()))

        case ("POST", "/setup/viewer/wipe"):
            deck.portable.wipe { [deck] in respond(.json(deck.portable.setupState())) }

        case ("GET", "/setup/viewer/qr.png"):
            guard deck.portable.isSetUp, let link = deck.portable.link else { return respond(.status(404)) }
            respond(Response(contentType: "image/png", body: QRCode.png(for: link) ?? Data()))

        // The portable office's viewer (beta). Only this reply carries the link,
        // which holds the key; the write secret is in a reply only once, as it's made.
        case ("GET", "/setup/viewer"):
            respond(.json(deck.portable.setupState(withLink: true)))

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
            respond(.json(deck.portable.setupState()))

        case ("POST", "/setup/viewer/secret"):
            guard let secret = deck.portable.makeWriteSecret() else {
                return respond(.status(500, "Couldn't keep the write secret in the Keychain."))
            }
            var state = deck.portable.setupState()
            state["secret"] = secret
            respond(.json(state))

        case ("POST", "/setup/viewer/check"):
            deck.portable.check { [deck] in respond(.json(deck.portable.setupState())) }

        case ("POST", "/setup/viewer/switch"):
            if request.query["on"] == "1" { deck.portable.start() } else { deck.portable.stop() }
            respond(.json(deck.portable.setupState()))

        case ("POST", "/setup/viewer/rotate"):
            deck.portable.rotateKey()
            respond(.json(deck.portable.setupState()))

        case ("POST", "/setup/viewer/wipe"):
            deck.portable.wipe { [deck] in respond(.json(deck.portable.setupState())) }

        case ("GET", "/setup/viewer/qr.png"):
            guard deck.portable.isSetUp, let link = deck.portable.link else { return respond(.status(404)) }
            respond(Response(contentType: "image/png", body: QRCode.png(for: link) ?? Data()))

        case ("GET", let path) where path.hasPrefix("/setup/logo/"):
            guard let logo = Logo(rawValue: (path as NSString).lastPathComponent) else { return respond(.status(404)) }
            respond(.file(logo.icon))

        case ("POST", "/setup/pair"):
            deck.access.newCode()
            respond(.json(setupState()))

        case ("POST", "/setup/unpair"):
            if request.query["all"] == "1" {
                deck.unpairAll()
            } else {
                guard let id = request.query["phone"], !id.isEmpty else { return respond(.status(400)) }
                deck.unpair(id)
            }
            respond(.json(setupState()))

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
            config.cloudBots = CloudBot.checked(given)
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

        // The six floors' names and styles, all of them, replaced at once and checked as they're kept.
        case ("POST", "/setup/floors"):
            guard let floors = Floor.sent(request.body) else {
                return respond(.status(400, "Send the floors as a list: {\"floors\": [{\"name\": …, \"preset\": …}]}."))
            }
            config.floors = floors
            config.save()
            deck.refresh()
            respond(.json(setupState()))

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
            respond(.json(setupState()))

        case ("POST", "/setup/scroll"):
            config.scrollEnabled = request.query["enabled"] == "1"
            config.save()
            deck.refresh()
            respond(.json(setupState()))

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
            respond(.json(setupState()))

        case ("POST", "/setup/webhook/test"):
            deck.testWebhook { [self] in respond(.json(setupState())) }

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
        // and only with a key; off at any time.
        case ("POST", "/setup/summaries"):
            if request.query["enabled"] == "1" {
                guard request.query["confirmed"] == "1" else {
                    return respond(.status(400, "Turn Summary on from its dialog, which says what it sends where."))
                }
                guard SummaryKey.isPresent else { return respond(.status(400, "Add your Anthropic API key first.")) }
                config.summaryOptIn = true
            } else {
                config.summaryOptIn = false
                deck.summaries.forget()
            }
            config.save()
            deck.refresh()
            respond(.json(setupState()))

        // Phone microphone (personal build): on or off, and whether its text is sent.
        case ("POST", "/setup/phonemic"):
            if let enabled = request.query["enabled"] { config.phoneMicEnabled = enabled == "1" }
            if let send = request.query["autosend"] { config.phoneMicAutoSend = send == "1" }
            config.save()
            deck.refresh()
            respond(.json(setupState()))

        // Whether the office asks before max effort: the phones are told, as it's theirs to ask.
        case ("POST", "/setup/maxconfirm"):
            config.maxConfirm = request.query["enabled"] == "1"
            config.save()
            deck.refresh()
            respond(.json(setupState()))

        // In the Dock and ⌘-Tab, or the menu bar only: at once, and kept.
        case ("POST", "/setup/dock"):
            config.showInDock = request.query["enabled"] == "1"
            config.save()
            Dock.show(config.showInDock)
            respond(.json(setupState()))

        // The key Summary uses: kept in the Keychain, and answered only as its hint.
        case ("POST", "/setup/summary/key"):
            if request.query["remove"] == "1" {
                SummaryKey.remove()
                // With no key left, Summary is off: no key, nothing to send with.
                if !SummaryKey.isPresent { config.summaryOptIn = false; config.save() }
            } else {
                let given = (try? JSONSerialization.jsonObject(with: request.body) as? [String: Any])?["key"] as? String ?? ""
                guard SummaryKey.save(given) else {
                    return respond(.status(400, "That isn't an Anthropic API key. It starts with sk-ant-."))
                }
            }
            deck.summaries.forget()
            deck.refresh()
            respond(.json(setupState()))

        case ("POST", "/setup/music"):
            config.musicEnabled = request.query["enabled"] == "1"
            config.save()
            deck.followMusic()
            deck.refresh()
            respond(.json(setupState()))

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
            if let enabled = request.query["enabled"] { config.hiringEnabled = enabled == "1" }
            if let folder { config.hireFolder = folder }
            if let agent { config.hireAgent = agent }
            if let terminal { config.hireTerminal = terminal }
            if let tmux { config.hireInTmux = tmux == "1" }
            config.save()
            deck.refresh()
            respond(.json(setupState()))

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
            guard config.dictationEnabled else { return respond(.json(setupState())) }
            Dictation.askForMicrophone { [self] _ in
                Dictation.prepare(config.dictationLanguage) { [self] problem in
                    DeckRouter.dictationProblem = problem
                    deck.refresh()
                    respond(.json(setupState()))
                }
            }

        // Personal build: the names dictation's clean-up knows, as the page's lines.
        case ("POST", "/setup/dictation/glossary"):
            guard let lines = try? JSONDecoder().decode([String].self, from: request.body) else {
                return respond(.status(400, "That isn't a list of names."))
            }
            config.dictationGlossary = DictationCleanup.checked(lines)
            config.save()
            respond(.json(setupState()))

        // The roles as the page has them, all of them: checked as they're kept,
        // since their instructions go to agents.
        case ("POST", "/setup/roles"):
            switch BriefRoutes.roles(request.body, saved: config.roles) {
            case .failure(let refused): return respond(refused.response)
            case .success(let roles): config.roles = roles
            }
            config.save()
            deck.refresh()
            respond(.json(setupState()))

        // The briefs, and hiring from setup as from the phone.
        case (_, let path) where BriefRoutes.handles(request.method, path):
            BriefRoutes.handle(request, config: config, library: .bundled, hire: { [deck] hire, done in
                deck.hire(as: hire.role, in: hire.folder, answers: hire.answers, done: done)
            }, respond: respond)

        // The Mac's own folder picker, which a web page can't open.
        case ("POST", "/setup/hiring/choose"):
            deck.chooseHireFolder { [self] in respond(.json(setupState())) }

        case ("POST", "/setup/hooks"):
            do {
                if request.query["remove"] == nil {
                    try Hooks.connect()
                } else {
                    try Hooks.disconnect()
                }
                respond(.json(setupState()))
            } catch {
                respond(.status(500, error.localizedDescription))
            }

        case ("POST", "/setup/accessibility"):
            Permissions.requestAccessibility()
            respond(.status(204))

        case ("POST", "/setup/login"):
            LoginItem.isEnabled = request.query["enabled"] == "1"
            respond(.json(setupState()))

        // The self-test: Start here's checklist, and --setup's.
        case ("GET", "/setup/selftest"):
            // Tailscale's answer first, if the last one is old or there's none,
            // so the network line is right the first time it's asked.
            Tailnet.whenFresh(changed: { [deck] in deck.refresh() }) { [deck] in
                respond(.json(SelfTest.report(SelfTest.checks(deck: deck))))
            }

        // One of its fixes, by name, and the checklist after it.
        case ("POST", "/setup/fix"):
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

        // Start here's Hire your General Manager: the GM role, in the folder for
        // new hires or your home folder, its questions asked in its terminal.
        case ("POST", "/setup/hire/gm"):
            let folder = config.hireFolder.isEmpty ? NSHomeDirectory() : config.hireFolder
            deck.hire(as: "gm", in: folder, answers: nil) { response in
                respond(response.status < 400 ? .status(204) : response)
            }

        default:
            respond(.status(404))
        }
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

    /// ✨ Summary for the setup page: on or off, the key's hint and where it's
    /// from (never the key), and the model, for the dialog.
    static func summaryState(optIn: Bool, environment: [String: String] = ProcessInfo.processInfo.environment) -> [String: Any] {
        let shown = SummaryKey.shown(environment: environment)
        return [
            "enabled": optIn && shown != nil,
            "key": shown.map { ["source": $0.source.rawValue, "hint": $0.hint] as [String: Any] } ?? NSNull(),
            "model": SummaryRequest.modelName,
        ]
    }

    private func setupState() -> [String: Any] {
        let config = Config.shared
        let (keys, _) = deck.layout()
        var keyOf: [String: Int] = [:]
        for (index, key) in keys.enumerated() {
            if let session = key.session { keyOf[session.id] = index }
        }

        var pairing: [String: Any] = ["address": deck.phoneAddress, "codeExpiresIn": 0]
        if let code = deck.access.activeCode {
            pairing["code"] = code
            pairing["codeExpiresIn"] = Int(deck.access.codeExpiry.timeIntervalSinceNow)
        }

        return [
            "checklist": [
                "hooks": Hooks.isConnected,
                "accessibility": Permissions.hasAccessibility,
                "scroll": Config.shared.scrollEnabled,
                "openAtLogin": LoginItem.isEnabled,
            ],
            "agents": Hooks.found.map { ["id": $0.rawValue, "name": $0.name, "connected": Hooks.isConnected($0)] },
            "hiring": [
                "enabled": config.hiringEnabled, "folder": config.hireFolder, "agent": config.hireAgent,
                "canHire": true, "canChoose": true,
                // Where hires open: the terminals on this Mac that can open them, and tmux if it's there.
                "terminal": Openers.isInstalled(config.hireTerminal) ? config.hireTerminal : "terminal",
                "terminals": Openers.choices.filter { Openers.isInstalled($0.id) }.map { ["id": $0.id, "name": $0.name] },
                "tmux": config.hireInTmux && Tmux.path != nil, "hasTmux": Tmux.path != nil,
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
            "dock": ["enabled": config.showInDock],
            "summary": DeckRouter.summaryState(optIn: config.summaryOptIn),
            // Phone microphone (personal build), and this Mac's tailnet address if Tailscale says it.
            "phoneMic": [
                "enabled": config.phoneMicEnabled, "autoSend": config.phoneMicAutoSend,
                "tailnet": (config.phoneMicEnabled ? Tailnet.url : nil).map { $0 as Any } ?? NSNull(),
                "serving": Tailnet.serving.map { $0 as Any } ?? NSNull(),
            ] as [String: Any],
            "webhook": [
                "enabled": config.webhook.enabled, "url": config.webhook.url, "secret": config.webhook.secret,
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
            "presets": Room.presets.filter { $0 != "yours" },
            // Saved looks, besides Default, the one the deck was last switched
            // to, and whether it has changed since.
            "profiles": Config.shared.profiles.map(\.jsonObject),
            "activeProfile": Config.shared.activeProfile,
            "profileEdited": Config.shared.profileEdited,
        ]
    }
}
