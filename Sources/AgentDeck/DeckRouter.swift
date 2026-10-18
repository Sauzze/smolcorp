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
    /// The setup page's key, as a tool of your own sends it, read from the
    /// setup-key file; or as the page's cookie, from the menu's link.
    static let setupKeyHeader = "x-smolcorp-setup-key"
    static let setupCookie = "smolcorp-setup"

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

        if request.path == "/setup" || request.path.hasPrefix("/setup/") {
            guard request.isLocal else { return respond(.status(404)) }
            guard request.method == "GET" || request.headers[DeckRouter.setupHeader] == "1" else {
                return respond(.status(403))
            }
            // The page itself holds nothing. What it reads and changes needs the setup key: the
            // cookie the page is given as the menu opens it, or the header a tool of your own sends.
            if request.path != "/setup" {
                let key = request.headers[DeckRouter.setupKeyHeader] ?? DeckRouter.cookie(DeckRouter.setupCookie, in: request)
                guard deck.access.isSetupKey(key) else { return respond(.status(401)) }
            }
            return setup(request, respond: respond)
        }

        if request.path == "/hook" {
            guard request.isLocal, request.method == "POST" else { return respond(.status(404)) }
            guard deck.access.isHookKey(request.headers["x-agentdeck-token"]) else { return respond(.status(401)) }
            guard !DeckRouter.hasSmuggledHookHeader(request) else {
                return respond(.status(400, "A hook header came twice"))
            }
            let time = Double(request.headers["x-agentdeck-at"] ?? "") ?? Date().timeIntervalSince1970 * 1000
            deck.ingest(request.json, reportedAs: request.headers["x-agentdeck-agent"] ?? "",
                        tab: request.headers["x-agentdeck-tab"] ?? "", tty: request.headers["x-agentdeck-tty"] ?? "",
                        startedIn: request.headers["x-agentdeck-project"] ?? "", at: time)
            return respond(.status(204))
        }

        guard request.isAddressedDirectly else { return respond(.status(403)) }

        if request.method == "POST" && request.path == "/pair" {
            let code = (request.json["code"] as? String ?? "").filter(\.isNumber)
            let userAgent = request.headers["user-agent"] ?? ""
            guard let key = deck.access.redeem(code, from: request.remoteAddress, userAgent: userAgent) else {
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

        let phone = deck.access.phones.phone(withKey: request.query["t"] ?? request.headers["x-agentdeck-token"])
        if let phone { deck.access.phones.markSeen(phone.id) }

        // The deck, or the office on a screen of its own: the same page, and
        // the same pairing first.
        if request.method == "GET" && (request.path == "/" || request.path == "/office") {
            return respond(.file(Paths.web.appendingPathComponent(phone == nil ? "pair.html" : "deck.html")))
        }
        guard let phone else { return respond(.status(401)) }

        let session = request.query["session"] ?? ""
        switch (request.method, request.path) {
        case ("GET", "/events"):
            reply(.eventStream { [deck] in deck.attach($0, phone: phone.id) })

        case ("POST", "/focus"):
            deck.focus(session, done: respond)

        case ("POST", "/act"):
            deck.perform(request.query["action"] ?? "", on: session, query: request.query, done: respond)

        default:
            respond(.status(404))
        }
    }

    // MARK: Setup page

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
            respond(Response(contentType: "image/png", body: QRCode.png(for: deck.pairingLink(for: code)) ?? Data()))

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

        // The roles as the page has them, all of them: checked as they're kept,
        // since their instructions go to agents.
        case ("POST", "/setup/roles"):
            guard let roles = try? JSONDecoder().decode([Role].self, from: request.body) else {
                return respond(.status(400, "Those aren't roles this app can use."))
            }
            config.roles = Role.checked(roles)
            config.save()
            deck.refresh()
            respond(.json(setupState()))

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
            // The roles, those that come with the app to go back to, and what a
            // role can be: its agent, whether that agent has an effort and
            // takes a first message, its badge, and how long each part can be.
            "roles": [
                "roles": config.roles.map(\.jsonObject),
                "shipped": Role.shipped.map(\.jsonObject),
                "agents": Agent.allCases.map { ["id": $0.rawValue, "name": $0.name, "effort": $0.hasEffortDial, "starts": $0 != .grok] },
                "efforts": Deck.effortLevels,
                "badges": Role.badges,
                "limits": ["name": Role.nameLimit, "model": Role.modelLimit, "instructions": Role.instructionsLimit, "own": Role.ownMost,
                           "questions": Role.questionsMost, "question": Role.questionLimit],
            ] as [String: Any],
            "music": ["enabled": config.musicEnabled],
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
                ]
            },
            "theme": Theme.editorJSON(),
            "layout": Config.shared.layout.jsonObject,
            // Saved looks, besides Default, the one the deck was last switched
            // to, and whether it has changed since.
            "profiles": Config.shared.profiles.map(\.jsonObject),
            "activeProfile": Config.shared.activeProfile,
            "profileEdited": Config.shared.profileEdited,
        ]
    }
}
