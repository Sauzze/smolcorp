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
    /// Agent windows (T796): off, a window for each hire, as before; on, hires
    /// run in tmux only, with no window, and a desk's "Open a window" shows one.
    /// Only with tmux on this Mac (see `Choices.background`).
    var hiresInBackground = false
    /// T3 Code (T796): whether its features show, in the office and Setup, and
    /// whether Smolcorp looks for it at all. Off until you turn it on; the T3
    /// Code code asks `Choices.t3Code` first.
    var t3CodeEnabled = false
    /// Whether first setup has asked the two choices above. A new Mac's file:
    /// not yet. A file from before them: as asked, so an existing user keeps
    /// what they had and isn't asked.
    var choicesAsked = false
    /// Where hires' git worktrees go; empty puts them beside the repo (see `HireDesk`).
    var hireWorktrees = ""
    /// Dictation for agents with no voice mode of their own (see `Dictation`):
    /// whether it's on, its language, and whether Claude Code uses it too
    /// rather than its own voice mode.
    var dictationEnabled = false
    var dictationLanguage = Dictation.suggestedLanguage
    /// Claude Code desks dictate through Smolcorp rather than their own
    /// `/voice`, always: your choice, kept as you left it.
    var dictationForClaude = false
    /// A test: whether dictation's text is cleaned up by
    /// Claude before it's typed (see `DictationCleanup`), and the names it
    /// knows. While it's on, Claude Code desks dictate through Smolcorp too, so
    /// their text can be cleaned (see `Deck.dictates`); off, everything is as
    /// before. On in this build, to be tried first.
    var dictationCleanup = false
    var dictationGlossary = DictationCleanup.defaultGlossary
    /// The roles the office can hire, in the order the phone offers them:
    /// those that come with the app until you change them, and your own.
    var roles = Role.checked(Role.shipped)
    /// The ids of the shipped roles this config has been given: a role that
    /// comes with a later version is added to your roles once, at the end,
    /// and one you've deleted isn't given back.
    var rolesOffered = Role.shipped.map(\.id)
    /// The folders hires last started in, latest first, for setup to offer.
    var recentHireFolders: [String] = []
    static let recentHireFoldersMost = 8
    /// Whether a jukebox in the office may play and pause the Mac's music.
    /// Off until you turn it on: it presses a media key.
    var musicEnabled = false
    /// Whether the office asks before max effort: the red button and a
    /// chip's max bring up a question first. On until you turn it off, as max
    /// uses much more of your usage; off, one tap sets it, as it always did.
    var maxConfirm = true
    /// Where the deck's sessions are told to other tools, if anywhere.
    var webhook = Webhook()
    /// ✨ Summary: whether a held desk offers a summary of its agent's
    /// transcript, written by Claude with your own API key. Off until you turn
    /// it on in setup and confirm what it sends where; and only with a key.
    /// Saved under its own name: an older "summariesEnabled" never
    /// turns it on.
    var summaryOptIn = false
    /// What makes summaries: a coding agent's id ("claude", "codex", …) or
    /// "apikey"; empty until one is picked or summaries are turned on (see
    /// `SummaryChoice`).
    var summaryProvider = ""
    /// The portable office's viewer: its address and options. Never its id,
    /// key or write secret, which are in the Keychain.
    var viewer = ViewerSettings()
    /// Phone microphone: on a deck opened over HTTPS (through
    /// `tailscale serve`), the mic listens on the phone with its own speech
    /// recognition and sends the text here to be typed. On by default; off,
    /// the mic is the Mac's again and Tailscale names are refused.
    var phoneMicEnabled = true
    /// Whether phone dictation skips the phone's confirm box and goes as it
    /// was heard: sent with Return once it's 3 words or more, as Claude
    /// Code's own /voice sends it, typed for you to send if shorter. Off by
    /// default: the box shows what was heard, to send, type or cancel.
    var phoneMicAutoSend = false
    var hasCompletedSetup = false
    /// Whether the first-run guide, the arrows in setup that point at the next thing to do, is over:
    /// finished or skipped. "Show Me Around Again" in the Help menu turns it back off. A file from
    /// before it, of someone who had set up already (setup opened, a hires folder or a GM, by hand too), counts as done.
    var guideDone = false
    /// You disconnected the agents on purpose, so `--setup` leaves them be.
    /// Connecting, in Settings or Start here, clears it.
    var hooksDisconnected = false
    /// The Claude Code accounts you confirmed in setup besides ~/.claude (the
    /// "default"): see `ClaudeProfiles`. A folder is only ever written to once it's here.
    var claudeProfiles: [ClaudeProfile] = []
    /// Whether the manager (the GM's front desk) is on. Off until you hire a General
    /// Manager or switch it on in setup, so 1.23's behaviour is what an existing user
    /// keeps. Off, the GM's routes say so; events are still logged. Turning it off never fires anyone.
    var managerEnabled = false
    /// "Allow a GM started by hand": also keep the GM's token in a file (the core's) for a
    /// General Manager you start yourself. Off, the token is in memory and the Keychain only, and
    /// reaches the GM Smolcorp starts through its environment.
    var gmByHand = false
    /// The GM's mail (`MailRouter`): the longest message in bytes, and the most one sender may send in an hour.
    var mailMaxBytes = MailRouter.defaultMaxBytes
    var mailPerHour = MailRouter.defaultPerHour
    /// Hours a message waits for a busy hire before it is left.
    var mailWaitHours = MailRouter.defaultWaitHours
    /// Whether mail may go to a hire's Claude Code inbox socket first. Off: only tmux (see `MailDelivery`).
    var mailInboxSocket = false
    /// The app's own watches for every hire (DONE, NEEDS-REVIEW, STOP, BLOCKED, BUDGET REQUEST, a limit, a refusal,
    /// a long wait) and the hourly digest for the GM: see `Watchers.sync`. On with the manager; this switches them off alone.
    var builtinWatches = true
    /// The screen that holds the team (`Tiler.Screen.id`), where you chose one;
    /// none means the Mac's own pick. Only asked about when there are two or more.
    var teamScreen: String?
    /// Whether features may press keys in other apps for you (`Keystrokes`). Off until you turn it on
    /// in setup, which is the only place it changes. macOS also asks for Accessibility.
    var keystrokesAllowed = false
    /// The apps (bundle ids) keystrokes may go to: empty until you add one in setup, which is the only
    /// place it changes. Nothing is pressed in an app that isn't here, switch or no switch.
    var keystrokeApps: [String] = []
    /// Smolcorp in the Dock and ⌘-Tab, as a full app, with its menus; off,
    /// in the menu bar only, as before. On unless you turn it off.
    var showInDock = true
    /// The office's window kept on top of other windows, on every Space and
    /// beside apps in full screen (see `OfficeWindow.alwaysOnTop`). Unset
    /// until it's settled once at launch: off on a new Mac, and on one that
    /// had it before as it was.
    var officeOnTop: Bool?
    /// The keys that show and hide the office's window from anywhere, as the
    /// setup page records them (see `OfficeKey`): none for ⌥⌘O, "off" for none.
    var officeKey: String?
    /// Bots that work in a cloud, not in a terminal here, each at a desk of
    /// its own (see `CloudBot`). None until you set some.
    var cloudBots: [CloudBot] = []
    /// Whether the Grok Bot app's bots get desks by themselves (see
    /// `Deck.discoverBots`); on until you turn it off. And the app's ids of
    /// those you hid, never given a desk again until you show them.
    var botsAutoDetect = true
    var hiddenBots: [String] = []
    /// The floor new bots go to, 0 for the ground floor up to 5 for 6F, when
    /// no floor rule names one for them (see `CloudBot.floor`); chosen in
    /// setup. Until it is, the floor your bots are on (`CloudBot.newBotsFloor`).
    var botFloor = 1
    /// Whether the file had `botFloor`: one from before it has it worked out
    /// again once the bots that were upstairs are settled (see `settleBots`).
    private(set) var botFloorKept = false
    /// The building's six floors, the ground floor first: each one's name,
    /// and the style of those above it (see `Floor`).
    var floors = Floor.standard
    /// The folders whose sessions sit on a floor of their own (see `FloorRule`).
    var floorRules: [FloorRule] = []
    /// The engine an earlier version ran ("swift" or "go"): ignored since
    /// 1.25.0, which runs the Go core alone (`Engine`), and kept in the file
    /// as written. Never saved unless it was set.
    var engine: String?
    /// Some of the file couldn't be read as this version reads it, and is
    /// left out: a copy of the file is kept before it's saved over.
    private(set) var lostOnLoad = false

    /// What the file held that this version doesn't read, as it was: saved
    /// back with the rest, so a setting from another version, older, newer
    /// or a build of your own, isn't lost at the next save.
    private var unknown: [String: Any] = [:]

    private enum CodingKeys: String, CodingKey, CaseIterable {
        case pins, colours, layout, profiles, activeProfile, logo, scrollEnabled, hiringEnabled, hireFolder, hireAgent, roles, rolesOffered
        case hireTerminal, hireInTmux, hiresInBackground, t3CodeEnabled, choicesAsked, hireWorktrees, recentHireFolders, dictationEnabled, dictationLanguage, dictationForClaude
        case dictationCleanup, dictationGlossary
        case musicEnabled, maxConfirm, webhook, summaryOptIn, summaryProvider, viewer, hasCompletedSetup, hooksDisconnected, claudeProfiles, managerEnabled, gmByHand
        case guideDone
        case mailMaxBytes, mailPerHour, mailWaitHours, mailInboxSocket, builtinWatches, teamScreen, keystrokesAllowed, keystrokeApps
        case phoneMicEnabled, showInDock, cloudBots, floors, floorRules, botsAutoDetect, hiddenBots, botFloor
        case engine, officeKey
        // Under a name of its own since the confirm box: the old one, saved as
        // true by every earlier build, meant no box had been offered yet.
        case phoneMicAutoSend = "phoneMicSkipsConfirm"
        // As the Windows app names it, for the one switch in setup.
        case officeOnTop = "alwaysOnTop"
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
        // Only the regular logo since setup lost the choice: an older pick is read but not kept.
        logo = .smolcorp
        scrollEnabled = field(Bool.self, .scrollEnabled) ?? false
        hiringEnabled = field(Bool.self, .hiringEnabled) ?? false
        hireFolder = field(String.self, .hireFolder) ?? ""
        hireAgent = field(String.self, .hireAgent).flatMap { Agent(rawValue: $0)?.rawValue } ?? hireAgent
        hireTerminal = field(String.self, .hireTerminal).flatMap { id in Openers.choices.contains { $0.id == id } ? id : nil } ?? hireTerminal
        hireInTmux = field(Bool.self, .hireInTmux) ?? false
        hiresInBackground = field(Bool.self, .hiresInBackground) ?? false
        t3CodeEnabled = field(Bool.self, .t3CodeEnabled) ?? false
        choicesAsked = field(Bool.self, .choicesAsked) ?? true
        hireWorktrees = field(String.self, .hireWorktrees).flatMap { $0.hasPrefix("/") ? $0 : nil } ?? ""
        recentHireFolders = Array((field([String].self, .recentHireFolders) ?? []).filter { $0.hasPrefix("/") }.prefix(Config.recentHireFoldersMost))
        dictationEnabled = field(Bool.self, .dictationEnabled) ?? false
        dictationLanguage = field(String.self, .dictationLanguage).flatMap { Dictation.languages.contains($0) ? $0 : nil } ?? dictationLanguage
        dictationForClaude = field(Bool.self, .dictationForClaude) ?? false
        dictationCleanup = field(Bool.self, .dictationCleanup) ?? false
        dictationGlossary = field([String].self, .dictationGlossary).map(DictationCleanup.checked) ?? dictationGlossary
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
        maxConfirm = field(Bool.self, .maxConfirm) ?? true
        webhook = field(Webhook.self, .webhook) ?? Webhook()
        summaryOptIn = field(Bool.self, .summaryOptIn) ?? false
        summaryProvider = field(String.self, .summaryProvider).flatMap { SummaryProvider(rawValue: $0)?.rawValue } ?? ""
        viewer = field(ViewerSettings.self, .viewer) ?? ViewerSettings()
        hasCompletedSetup = field(Bool.self, .hasCompletedSetup) ?? false
        hooksDisconnected = field(Bool.self, .hooksDisconnected) ?? false
        // Checked as they load: their folders get Smolcorp's hooks written into them.
        claudeProfiles = (field([Lossy<ClaudeProfile>].self, .claudeProfiles)?.compactMap(\.value) ?? []).compactMap {
            try? ClaudeProfiles.check($0).get()
        }
        managerEnabled = field(Bool.self, .managerEnabled) ?? false
        gmByHand = field(Bool.self, .gmByHand) ?? false
        // Someone who had set up before the guide came never sees it unless they ask.
        guideDone = field(Bool.self, .guideDone) ?? (hasCompletedSetup || !hireFolder.isEmpty || managerEnabled || gmByHand)
        // At least 1, so a bad value can't shut the mail off without saying so.
        mailMaxBytes = max(1, field(Int.self, .mailMaxBytes) ?? MailRouter.defaultMaxBytes)
        mailPerHour = max(1, field(Int.self, .mailPerHour) ?? MailRouter.defaultPerHour)
        mailWaitHours = max(1, field(Double.self, .mailWaitHours) ?? MailRouter.defaultWaitHours)
        mailInboxSocket = field(Bool.self, .mailInboxSocket) ?? false
        builtinWatches = field(Bool.self, .builtinWatches) ?? true
        teamScreen = field(String.self, .teamScreen)
        keystrokesAllowed = field(Bool.self, .keystrokesAllowed) ?? false
        // Checked as they load, so a hand-edited file can't put a terminal or an odd id on the list.
        keystrokeApps = Keystrokes.AppList.checked(field([Lossy<String>].self, .keystrokeApps)?.compactMap(\.value) ?? [])
        phoneMicEnabled = field(Bool.self, .phoneMicEnabled) ?? true
        phoneMicAutoSend = field(Bool.self, .phoneMicAutoSend) ?? false
        showInDock = field(Bool.self, .showInDock) ?? true
        officeOnTop = field(Bool.self, .officeOnTop)
        officeKey = field(String.self, .officeKey)
        cloudBots = CloudBot.checked(field([Lossy<CloudBot>].self, .cloudBots)?.compactMap(\.value) ?? [])
        // A file from before it: where the bots are, and again once those that were upstairs are settled.
        let keptFloor = field(Int.self, .botFloor).flatMap { (0..<Floor.count).contains($0) ? $0 : nil }
        botFloorKept = keptFloor != nil
        botFloor = keptFloor ?? CloudBot.newBotsFloor(for: cloudBots)
        botsAutoDetect = field(Bool.self, .botsAutoDetect) ?? true
        hiddenBots = (field([String].self, .hiddenBots) ?? []).compactMap(GrokBotApp.safeID)
        // Checked as they load: their names are drawn in the office.
        floors = Floor.checked(field([Lossy<Floor>].self, .floors)?.map(\.value) ?? [])
        floorRules = FloorRule.checked(field([Lossy<FloorRule>].self, .floorRules)?.compactMap(\.value) ?? [])
        // Kept as written, even a name this version doesn't know: the app logs it, and runs the Go engine.
        engine = field(String.self, .engine)
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

    /// The bots still "upstairs", from before bots could sit on any floor,
    /// each on the floor it sits on, by `seats`, the desks saved across a
    /// restart (see `CloudBot.settled`); and with a file from before
    /// `botFloor`, new bots then go where the bots are. Whether anything
    /// changed, to be saved. Once settled, there's nothing to do again.
    func settleBots(seats: [String: Seat]) -> Bool {
        guard cloudBots.contains(where: { $0.floor == CloudBot.upstairs }) else { return false }
        cloudBots = CloudBot.settled(cloudBots, seats: seats)
        if !botFloorKept { botFloor = CloudBot.newBotsFloor(for: cloudBots) }
        return true
    }

    /// `folder` first among the recent hire folders, once.
    func noteHire(in folder: String) {
        recentHireFolders = Array(([folder] + recentHireFolders.filter { $0 != folder }).prefix(Config.recentHireFoldersMost))
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

// MARK: Floors

/// A floor of the building: the name its sign says, and its style. The
/// ground floor is the office itself, with its desks and keys; the five above
/// hold those who have no desk there (see `Deck.floors`).
struct Floor: Codable, Equatable {
    var name: String
    /// "" for the same style as the office downstairs, "server" for the
    /// server room floor 2 always had, or one of the office's styles.
    var preset: String

    static let count = 6
    /// As many desks as each floor has: three rows of three, as the office
    /// draws them on every floor. The one number every floor's room is
    /// counted from: seating, the moves between floors, a hire upstairs, the
    /// bots' room. The Windows and Linux app's `desksPerFloor` is the same.
    static let desks = 9
    static let nameLimit = 24
    static let server = "server"

    /// 1F for the ground floor, 2F for the one above, and so on.
    static func defaultName(_ index: Int) -> String { "\(index + 1)F" }

    static let standard = (0..<count).map { Floor(name: defaultName($0), preset: "") }

    init(name: String, preset: String) {
        self.name = name
        self.preset = preset
    }

    /// Read leniently: a name or style that's missing or isn't text loses only itself.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        name = ((try? c.decodeIfPresent(String.self, forKey: .name)) ?? nil) ?? ""
        preset = ((try? c.decodeIfPresent(String.self, forKey: .preset)) ?? nil) ?? ""
    }

    /// Exactly six, as they're kept: each name one line of 24 characters at
    /// most, or its default when empty; each style one there is, or the
    /// office's own. The ground floor's look is always the office's own, and
    /// Yours, the office as you furnished it, is the office's own too.
    /// Missing floors are the defaults; any past the sixth are left out.
    static func checked(_ given: [Floor?]) -> [Floor] {
        (0..<count).map { index in
            let floor = index < given.count ? given[index] : nil
            let name = (floor?.name ?? "").oneLine(nameLimit)
            var preset = floor?.preset ?? ""
            if index == 0 || preset == "yours" || !(preset == server || Room.presets.contains(preset)) { preset = "" }
            return Floor(name: name.isEmpty ? defaultName(index) : name, preset: preset)
        }
    }

    /// The floors' rules setup sends, as `{"rules": [...]}`, checked; nil
    /// when there's no list of them, as from a setup page from before rules.
    static func rules(_ body: Data) -> [FloorRule]? {
        guard let list = ((try? JSONSerialization.jsonObject(with: body)) as? [String: Any])?["rules"] as? [Any],
              let data = try? JSONSerialization.data(withJSONObject: list),
              let rules = try? JSONDecoder().decode([Lossy<FloorRule>].self, from: data) else { return nil }
        return FloorRule.checked(rules.compactMap(\.value))
    }

    /// The floors setup sends, as `{"floors": [...]}` or the list alone,
    /// checked; nil unless it's a list.
    static func sent(_ body: Data) -> [Floor]? {
        let object = try? JSONSerialization.jsonObject(with: body)
        guard let list = ((object as? [String: Any])?["floors"] ?? object) as? [Any],
              let data = try? JSONSerialization.data(withJSONObject: list),
              let floors = try? JSONDecoder().decode([Lossy<Floor>].self, from: data) else { return nil }
        return checked(floors.map(\.value))
    }

    var jsonObject: [String: Any] { ["name": name, "preset": preset] }
}

// MARK: Profiles

/// Why a profile couldn't be saved, renamed or added, in the setup page's words.
struct ProfileError: Error {
    let message: String

    static let name = ProfileError(message: "Give the profile a name of its own, up to 24 characters.")
    static let reserved = ProfileError(message: "“Default” is Smolcorp's own profile and can't be overwritten. Give the backup another name.")
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
        if Profile.isDefaultName(name) { throw ProfileError.reserved }
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
        if Profile.isDefaultName(newName) { throw ProfileError.reserved }
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
}
