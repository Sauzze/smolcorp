import Foundation

/// What setup's brief and hire routes answer, apart from the deck: each
/// takes what a request sent and gives what it's answered, so they can be
/// tried without the app. DeckRouter lets in only setup's own requests.
enum BriefRoutes {
    /// A request refused, with what it's answered.
    struct Refusal: Error {
        let response: Response

        static func status(_ code: Int, _ message: String) -> Refusal { Refusal(response: .status(code, message)) }
    }

    /// Whether a request is one of these routes'.
    static func handles(_ method: String, _ path: String) -> Bool {
        switch (method, path) {
        case ("GET", "/setup/briefs"), ("POST", "/setup/briefs/preview"), ("POST", "/setup/hire"), ("GET", "/setup/hire/folders"): return true
        case ("GET", _): return path.hasPrefix("/setup/briefs/")
        default: return false
        }
    }

    /// Answers one of these routes, let in already as setup's own. `hire`
    /// hires as the phone does, once the request is checked.
    static func handle(_ request: Request, config: Config, library: BriefLibrary, home: URL = Paths.home,
                       hire: (HireRequest, @escaping (Response) -> Void) -> Void, respond: @escaping (Response) -> Void) {
        switch (request.method, request.path) {
        case ("GET", "/setup/briefs"):
            respond(list(library))
        case ("POST", "/setup/briefs/preview"):
            respond(preview(request.body, roles: config.roles, library: library, hireFolder: config.hireFolder, pins: config.pins))
        case ("POST", "/setup/hire"):
            switch hireRequest(request.body, roles: config.roles, home: home) {
            case .failure(let refused): respond(refused.response)
            case .success(let checked): hire(checked, respond)
            }
        case ("GET", "/setup/hire/folders"):
            respond(folders(default: config.hireFolder, recent: config.recentHireFolders))
        case ("GET", let path) where path.hasPrefix("/setup/briefs/"):
            respond(markdown(String(path.dropFirst("/setup/briefs/".count)), library))
        default:
            respond(.status(404))
        }
    }

    /// `GET /setup/briefs`: the library, each brief's id, name and summary,
    /// with the questions it suggests for a role of your own.
    static func list(_ library: BriefLibrary) -> Response {
        .json(["briefs": library.entries().map(\.jsonObject)])
    }

    /// `GET /setup/briefs/<id>`: the whole brief, as a picked file would be.
    static func markdown(_ id: String, _ library: BriefLibrary) -> Response {
        guard let markdown = library.template(id) else { return .status(404, "There's no brief \(id).") }
        return .json(["markdown": markdown])
    }

    /// The answers a request sent: nil if it sent none, or a refusal for
    /// anything but a list of text.
    static func answers(_ given: Any?) -> Result<[String]?, Refusal> {
        guard let given, !(given is NSNull) else { return .success(nil) }
        guard let list = given as? [Any], list.count <= Role.questionsMost, let answers = list as? [String] else {
            return .failure(.status(400, "Answers are a list of at most \(Role.questionsMost) texts."))
        }
        return .success(answers)
    }

    /// `POST /setup/briefs/preview`, with `{role, answers?, folder?, pinned?}`:
    /// the role's BRIEF.md as a hire would have it, or for a role on its own
    /// instructions, what its agent is first told. `role` is a role as the
    /// page has it, or the id of one of `roles`. A folder one of `pins` is,
    /// or `pinned`, is a pinned desk's, where its brief would be written.
    static func preview(_ body: Data, roles: [Role], library: BriefLibrary, hireFolder: String, pins: [String?] = [], date: Date = Date(),
                        kept: URL = Paths.hireBriefs) -> Response {
        guard let given = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any] else {
            return .status(400, "Send {role, answers}.")
        }
        var role: Role?
        if let id = given["role"] as? String {
            role = roles.first { $0.id == id }
            if role == nil { return .status(404, "There's no role \(id).") }
        } else if let object = given["role"] as? [String: Any], let data = try? JSONSerialization.data(withJSONObject: object),
                  let decoded = try? JSONDecoder().decode(Role.self, from: data) {
            if let problem = problem(in: decoded) { return .status(400, problem) }
            role = Role.synced([decoded], saved: roles).first?.checked
        }
        guard var role else { return .status(400, "That isn't a role this app can use.") }
        role.brief = role.source
        let answers: [String]?
        switch BriefRoutes.answers(given["answers"]) {
        case .success(let list): answers = list
        case .failure(let refused): return refused.response
        }
        let folder = (given["folder"] as? String).flatMap { $0.isEmpty ? nil : ($0 as NSString).expandingTildeInPath }
            ?? Deck.folder(hireFolder) ?? Paths.home.appendingPathComponent("Projects").path
        let pinned = given["pinned"] as? Bool ?? pins.contains { $0.flatMap(Deck.folder) == Deck.folder(folder) && Deck.folder(folder) != nil }
        do {
            guard let template = try role.briefTemplate(in: library) else {
                let message = answers.map { role.firstMessage(in: folder, answers: $0) }
                    ?? role.firstMessage(in: folder, pinned: pinned)
                return .json(["kind": "text", "markdown": message, "firstMessage": message])
            }
            let place = BriefFiles.place(for: role.id, in: folder, pinned: pinned, desk: nil, at: date, kept: kept)
            let names = BriefFiles.names(place, report: Role.reportName(of: template), suffix: 0)
            let markdown = try role.renderBrief(template, BriefContext(folder: folder, pinned: pinned,
                                                                       answers: answers, date: date, briefFile: names.brief.path,
                                                                       statusFile: names.status.path, reportFile: names.report?.path))
            return .json(["kind": role.source.kind.rawValue, "markdown": markdown,
                          "firstMessage": role.briefMessage(at: names.brief.path, asks: answers == nil)])
        } catch {
            return .status(400, error.localizedDescription)
        }
    }

    /// Why a role's brief, as sent, can't be kept, in words setup shows as
    /// they are; nil when it can.
    static func problem(in role: Role) -> String? {
        guard let brief = role.brief else { return nil }
        let name = role.name.isEmpty ? role.id : role.name
        switch brief.kind {
        case .text: return nil
        case .library: return Brief.libraryIDs.contains(brief.id) ? nil : "\(name): there's no brief \(brief.id)."
        case .file:
            do {
                _ = try Brief.file(brief.text, named: brief.fileName)
                return nil
            } catch {
                return "\(name): \(error.localizedDescription)"
            }
        }
    }

    /// `POST /setup/roles`: the roles as the page has them, all of them,
    /// each with its brief's text kept in step with its instructions, or the
    /// first reason one's brief can't be kept.
    static func roles(_ body: Data, saved: [Role]) -> Result<[Role], Refusal> {
        guard let given = try? JSONDecoder().decode([Role].self, from: body) else {
            return .failure(.status(400, "Those aren't roles this app can use."))
        }
        if let problem = given.lazy.compactMap(problem).first { return .failure(.status(400, problem)) }
        return .success(Role.checked(Role.synced(given, saved: saved)))
    }

    /// Where setup may hire into: a folder that's there, inside your home
    /// folder but not your home folder itself, nor ~/Library or a hidden
    /// folder at its top, such as ~/.ssh. Links are followed first, so none
    /// leads out. The folder as hired into, or why not.
    static func checkedFolder(_ given: String, home: URL = Paths.home) -> Result<String, Refusal> {
        let trimmed = given.trimmingCharacters(in: .whitespacesAndNewlines)
        let expanded = (trimmed as NSString).expandingTildeInPath
        guard expanded.hasPrefix("/") else { return .failure(.status(400, "Give the folder's full path, such as ~/Projects/app.")) }
        let folder = URL(fileURLWithPath: expanded).standardizedFileURL.resolvingSymlinksInPath().path
        var isFolder: ObjCBool = false
        guard FileManager.default.fileExists(atPath: folder, isDirectory: &isFolder) else {
            return .failure(.status(400, "There's no folder at \(trimmed)."))
        }
        guard isFolder.boolValue else { return .failure(.status(400, "\(trimmed) is a file, not a folder.")) }
        let home = home.standardizedFileURL.resolvingSymlinksInPath().path
        guard folder.hasPrefix(home + "/") else { return .failure(.status(400, "Hire into a folder inside your home folder.")) }
        let top = folder.dropFirst(home.count + 1).split(separator: "/").first.map(String.init) ?? ""
        guard top != "Library", !top.hasPrefix(".") else {
            return .failure(.status(400, "That's a folder for your Mac's or your apps' own files. Hire into a project folder."))
        }
        return .success(folder)
    }

    struct HireRequest: Equatable {
        let role, folder: String
        let answers: [String]
    }

    /// `POST /setup/hire`, with `{role, folder, answers}`: what to hire, checked.
    static func hireRequest(_ body: Data, roles: [Role], home: URL = Paths.home) -> Result<HireRequest, Refusal> {
        guard let given = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any] else {
            return .failure(.status(400, "Send {role, folder, answers}."))
        }
        guard let id = given["role"] as? String, !id.isEmpty else { return .failure(.status(400, "Say which role to hire.")) }
        guard roles.contains(where: { $0.id == id }) else { return .failure(.status(404, "That role isn't there any more. Pick another.")) }
        guard let path = given["folder"] as? String, !path.isEmpty else { return .failure(.status(400, "Say which folder to hire into.")) }
        let folder: String
        switch checkedFolder(path, home: home) {
        case .success(let checked): folder = checked
        case .failure(let refused): return .failure(refused)
        }
        switch answers(given["answers"]) {
        case .success(let list): return .success(HireRequest(role: id, folder: folder, answers: list ?? []))
        case .failure(let refused): return .failure(refused)
        }
    }

    /// `GET /setup/hire/folders`: the folder set up for new hires (empty if
    /// none), and the folders hires last started in that are still there.
    static func folders(default folder: String, recent: [String]) -> Response {
        .json(["default": folder, "recent": recent.compactMap(Deck.folder)])
    }
}
