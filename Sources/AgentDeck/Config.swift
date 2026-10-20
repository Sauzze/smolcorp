import Foundation

/// What you set on the setup page: which project is pinned to which key, the
/// deck's layout and colours, the profiles they're saved in, its logo,
/// whether two fingers scroll, and whether the office hires and fires. Saved
/// as JSON in Application Support.
final class Config: Codable {
    static let shared = Config.load()

    /// A project folder per key, or nil for a key that takes any session.
    var pins: [String?] = Array(repeating: nil, count: KeyCount.most)
    /// Only the colours that differ from the default theme.
    var colours: [String: String] = [:]
    var layout = Layout.standard
    /// Saved looks, besides Default, and the one the deck was last switched to.
    var profiles: [Profile] = []
    var activeProfile = Profile.defaultName
    var logo = Logo.smolcorp
    /// Whether two fingers on the office scroll Terminal. Off until you turn it on, so a
    /// newly paired phone can do no more than the office's keys do.
    var scrollEnabled = false
    /// Whether the office may hire and fire: open a new Terminal window with
    /// an agent in it, and stop an agent and close its tab. Off until you
    /// turn it on, for the same reason.
    var hiringEnabled = false
    /// The folder new hires start in, and the agent they are.
    var hireFolder = ""
    var hireAgent = Agent.claude.rawValue
    /// The terminal hires open in (see `Openers.choices`), and whether the
    /// agent runs in a tmux session of its own there.
    var hireTerminal = "terminal"
    var hireInTmux = false
    /// Dictation for agents with no voice mode of their own (see `Dictation`):
    /// whether it's on, its language, and whether Claude Code uses it too
    /// rather than its own voice mode.
    var dictationEnabled = false
    var dictationLanguage = Dictation.suggestedLanguage
    var dictationForClaude = false
    /// The roles the office can hire, in the order the phone offers them:
    /// those that come with the app until you change them, and your own.
    var roles = Role.shipped
    /// The ids of the shipped roles this config has been given: a role that
    /// comes with a later version is added to your roles once, at the end,
    /// and one you've deleted isn't given back.
    var rolesOffered = Role.shipped.map(\.id)
    /// Whether a jukebox in the office may play and pause the Mac's music.
    /// Off until you turn it on: it presses a media key.
    var musicEnabled = false
    /// Where the deck's sessions are told to other tools, if anywhere.
    var webhook = Webhook()
    var hasCompletedSetup = false
    /// Some of the file couldn't be read as this version reads it, and is
    /// left out: a copy of the file is kept before it's saved over.
    private(set) var lostOnLoad = false

    /// What the file held that this version doesn't read, as it was: saved
    /// back with the rest, so a setting from another version, older, newer
    /// or a build of your own, isn't lost at the next save.
    private var unknown: [String: Any] = [:]

    private enum CodingKeys: String, CodingKey, CaseIterable {
        case pins, colours, layout, profiles, activeProfile, logo, scrollEnabled, hiringEnabled, hireFolder, hireAgent, roles, rolesOffered
        case hireTerminal, hireInTmux, dictationEnabled, dictationLanguage, dictationForClaude
        case musicEnabled, webhook, hasCompletedSetup
    }

    init() {}

    /// Field by field, so a file from an older or newer version still loads:
    /// a field it can't read loses only itself, and `lostOnLoad` says so.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        var lost = false
        func field<T: Decodable>(_ type: T.Type, _ key: CodingKeys) -> T? {
            do { return try c.decodeIfPresent(type, forKey: key) } catch {
                lost = true
                return nil
            }
        }
        pins = field([String?].self, .pins) ?? pins
        colours = field([String: String].self, .colours) ?? colours
        // Layouts are checked again as they load: their buttons type into sessions.
        layout = (field(Layout.self, .layout) ?? layout).checked
        // One by one, so a profile this version can't read loses only itself.
        let lossy = field([Lossy<Profile>].self, .profiles) ?? []
        if lossy.contains(where: { $0.value == nil }) { lost = true }
        profiles = lossy.compactMap(\.value).map {
            Profile(name: $0.name, layout: $0.layout.checked, colours: $0.colours)
        }
        activeProfile = field(String.self, .activeProfile) ?? activeProfile
        logo = field(String.self, .logo).flatMap(Logo.init(stored:)) ?? logo
        scrollEnabled = field(Bool.self, .scrollEnabled) ?? false
        hiringEnabled = field(Bool.self, .hiringEnabled) ?? false
        hireFolder = field(String.self, .hireFolder) ?? ""
        hireAgent = field(String.self, .hireAgent).flatMap { Agent(rawValue: $0)?.rawValue } ?? hireAgent
        hireTerminal = field(String.self, .hireTerminal).flatMap { id in Openers.choices.contains { $0.id == id } ? id : nil } ?? hireTerminal
        hireInTmux = field(Bool.self, .hireInTmux) ?? false
        dictationEnabled = field(Bool.self, .dictationEnabled) ?? false
        dictationLanguage = field(String.self, .dictationLanguage).flatMap { Dictation.languages.contains($0) ? $0 : nil } ?? dictationLanguage
        dictationForClaude = field(Bool.self, .dictationForClaude) ?? false
        // Checked as they load: their instructions go to agents.
        let savedRoles = field([Role].self, .roles)
        roles = savedRoles.map(Role.checked) ?? roles
        // Shipped roles this file hasn't been given are added once, at the end: a file from before rolesOffered
        // was given the first five. rolesOffered is then every shipped role, as it's saved.
        if savedRoles != nil {
            let offered = field([String].self, .rolesOffered) ?? Role.offeredBefore
            roles = Role.checked(roles + Role.shipped.filter { role in !offered.contains(role.id) && !roles.contains { $0.id == role.id } })
        }
        musicEnabled = field(Bool.self, .musicEnabled) ?? false
        webhook = field(Webhook.self, .webhook) ?? Webhook()
        hasCompletedSetup = field(Bool.self, .hasCompletedSetup) ?? false
        lostOnLoad = lost
    }

    /// A file it can't read at all starts from the defaults, and one it can
    /// read only some of keeps the rest, but either way a copy is kept beside
    /// it first, as config.json.broken, so the next save doesn't take what
    /// was left out with it.
    static func load(from file: URL = Paths.config) -> Config {
        guard let data = try? Data(contentsOf: file) else { return Config() }
        guard let config = try? JSONDecoder().decode(Config.self, from: data) else {
            Paths.writePrivately(data, to: file.appendingPathExtension("broken"))
            return Config()
        }
        if config.lostOnLoad { Paths.writePrivately(data, to: file.appendingPathExtension("broken")) }
        if config.pins.count != KeyCount.most {
            config.pins = Array((config.pins + Array(repeating: nil, count: KeyCount.most)).prefix(KeyCount.most))
        }
        let known = Set(CodingKeys.allCases.map(\.stringValue))
        let object = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] ?? [:]
        config.unknown = object.filter { !known.contains($0.key) }
        return config
    }

    func save(to file: URL = Paths.config) {
        guard let data = encoded() else { return }
        // Readable by you alone: it holds the webhook's secret.
        Paths.writePrivately(data, to: file)
    }

    /// The file as it's saved: this version's settings, and what it didn't read, as it was.
    func encoded() -> Data? {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        guard let data = try? encoder.encode(self) else { return nil }
        guard !unknown.isEmpty, var object = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] else { return data }
        for (key, value) in unknown where object[key] == nil { object[key] = value }
        return try? JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted, .sortedKeys])
    }

    /// Pin `folder` to key `index`, taking it off any other key it was on.
    func pin(_ folder: String, to index: Int) {
        pins = pins.map { $0 == folder ? nil : $0 }
        pins[index] = folder
        save()
    }

    func unpin(_ index: Int) {
        pins[index] = nil
        save()
    }

    /// Puts `first` on key `b` and `second` on key `a`, each taken off any
    /// other key it was on: two desks swapped in the office. A nil project
    /// is an empty desk, whose key is left free.
    func swapPins(_ first: String?, at a: Int, with second: String?, at b: Int) {
        pins = pins.map { $0 != nil && ($0 == first || $0 == second) ? nil : $0 }
        pins[b] = first
        pins[a] = second
        save()
    }
}

/// A value decoded on its own, so one that can't be read is left out rather
/// than taking the rest of a list with it.
private struct Lossy<Value: Decodable>: Decodable {
    let value: Value?

    init(from decoder: Decoder) throws {
        value = try? Value(from: decoder)
    }
}

// MARK: Profiles

/// Why a profile couldn't be saved, renamed or added, in the setup page's words.
struct ProfileError: Error {
    let message: String

    static let name = ProfileError(message: "Give the profile a name of its own, up to 24 characters.")
    static let missing = ProfileError(message: "There's no profile by that name.")
    static let tooMany = ProfileError(message: "That's the most profiles there can be. Delete one first.")
}

extension Config {
    func profile(named name: String) -> Profile? {
        name == Profile.defaultName ? .standard : profiles.first { $0.name == name }
    }

    /// The deck no longer matches the profile it was switched to.
    var profileEdited: Bool {
        guard let active = profile(named: activeProfile) else { return true }
        return active.layout != layout || active.colours != colours
    }

    /// Puts a saved profile's layout and colours on the deck.
    func switchProfile(to name: String) throws {
        guard let chosen = profile(named: name) else { throw ProfileError.missing }
        layout = chosen.layout
        colours = chosen.colours
        activeProfile = name
        save()
    }

    /// Keeps the deck as it is under `name`, replacing a profile of that name.
    func saveProfile(as name: String) throws {
        guard let name = Profile.checkedName(name) else { throw ProfileError.name }
        let kept = Profile(name: name, layout: layout, colours: colours)
        if let i = profiles.firstIndex(where: { $0.name == name }) {
            profiles[i] = kept
        } else {
            guard profiles.count < Profile.most else { throw ProfileError.tooMany }
            profiles.append(kept)
        }
        activeProfile = name
        save()
    }

    func renameProfile(_ name: String, to newName: String) throws {
        guard let i = profiles.firstIndex(where: { $0.name == name }) else { throw ProfileError.missing }
        guard let newName = Profile.checkedName(newName) else { throw ProfileError.name }
        if newName != name && profiles.contains(where: { $0.name == newName }) {
            throw ProfileError(message: "Another profile already has that name.")
        }
        profiles[i].name = newName
        if activeProfile == name { activeProfile = newName }
        save()
    }

    /// The deck stays as it is; it just no longer matches a saved profile.
    func deleteProfile(_ name: String) {
        profiles.removeAll { $0.name == name }
        if activeProfile == name { activeProfile = Profile.defaultName }
        save()
    }

    /// A profile from a file as this deck would keep it, checked as a layout
    /// and colours made here are. The setup page shows its buttons from this,
    /// before it's added, so what you're asked about is what's kept.
    static func checkedProfile(_ data: Data) throws -> Profile {
        let file = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] ?? [:]
        guard let layoutObject = file["layout"] as? [String: Any],
              let layoutData = try? JSONSerialization.data(withJSONObject: layoutObject),
              let layout = Layout.parse(layoutData) else {
            throw ProfileError(message: "That isn't a profile this deck can add.")
        }
        let name = Profile.checkedName(file["name"] as? String ?? "") ?? "Imported"
        return Profile(name: name, layout: layout, colours: Theme.customisations(from: file["colours"] as? [String: Any] ?? [:]))
    }

    /// Adds a profile from a file, under a name of its own.
    func importProfile(_ data: Data) throws {
        var profile = try Config.checkedProfile(data)
        guard profiles.count < Profile.most else { throw ProfileError.tooMany }
        profile.name = unusedName(profile.name)
        profiles.append(profile)
        save()
    }

    /// `name`, or name (2), (3)… when a profile already has it.
    private func unusedName(_ name: String) -> String {
        var candidate = name
        var n = 2
        while candidate == Profile.defaultName || profiles.contains(where: { $0.name == candidate }) {
            let suffix = " (\(n))"
            candidate = name.oneLine(Profile.nameLimit - suffix.count) + suffix
            n += 1
        }
        return candidate
    }
}

/// The deck's logo: the hire, a little person in a suit, or the six keys it
/// started out with. It shows on the phone's Home Screen, on the setup page
/// and in the menu bar.
enum Logo: String, Codable, CaseIterable {
    case smolcorp, keys

    /// As saved: the mark the app had before it was Smolcorp is now the hire.
    init?(stored: String) {
        self.init(rawValue: stored == "sauzze" ? "smolcorp" : stored)
    }

    var icon: URL { Paths.web.appendingPathComponent("icon-\(rawValue).png") }
}
