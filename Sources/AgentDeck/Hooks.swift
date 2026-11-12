import Foundation

/// Where each coding agent is told to run the hook script.
///
/// User-level, so every session on this Mac reports to the deck, whatever
/// folder it starts in. Only agents with a settings folder are connected; one
/// installed later is picked up by connecting again.
///
/// Claude Code, Codex and Gemini CLI read hooks from a settings file they
/// share with everything else: the deck's entries are added to it, the rest
/// is kept as it was, and the file is backed up before each change (see
/// `write`). The entries are recognised by the path of the hook
/// script, so connecting twice changes nothing and disconnecting takes out
/// only those. Claude Code has one such file in each account's folder (see
/// `ClaudeProfiles`): connecting writes to every one, and refreshing at
/// launch only to those that already have the entries. Grok and OpenCode load whole files from a folder, so the deck
/// has one of its own there.
enum Hooks {
    /// The deck's entries in an agent's hook settings, in the layout Claude
    /// Code set and the others follow: event → matcher groups → handlers.
    private struct Entries {
        let file: URL
        let events: [String]
        let handler: [String: Any]
        /// The file holds nothing but these entries.
        var isOwnFile = false
    }

    /// Quoted, because Application Support has a space in it. Grok's entries
    /// match Claude Code's to the letter: Grok also reads Claude Code's
    /// hooks, and runs a handler it finds twice only once.
    private static func command(for agent: Agent) -> String {
        let script = "\"\(Paths.hookScript.path)\""
        return agent == .claude || agent == .grok ? script : "\(script) \(agent.rawValue)"
    }

    /// Claude Code's events the deck follows. PermissionDenied is the one it
    /// sends when a permission is refused; the event log keeps it.
    static let claudeEvents = [
        "SessionStart", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PermissionDenied", "PostToolUse",
        "PostToolUseFailure", "Notification", "Stop", "StopFailure", "SessionEnd",
        "SubagentStart", "SubagentStop", "PreCompact", "PostCompact",
    ]

    /// Claude Code's entries for the settings file of one account.
    private static func claudeEntries(file: URL) -> Entries {
        Entries(file: file, events: claudeEvents, handler: claudeHandler())
    }

    private static func claudeHandler(script: URL = Paths.hookScript) -> [String: Any] {
        ["type": "command", "command": "\"\(script.path)\"", "async": true]
    }

    /// The hooks a test copy's hire carries in its own settings file, since the account's settings
    /// (which hold the real app's) aren't loaded for it: the same events and handler as the account's
    /// entries, running the copy's own script, which sends to the copy's port.
    static func hireHooks(script: URL = Paths.hookScript) -> [String: Any] {
        Dictionary(uniqueKeysWithValues: claudeEvents.map { ($0, [["hooks": [claudeHandler(script: script)]]] as [[String: Any]]) })
    }

    private static func entries(for agent: Agent) -> Entries? {
        let command = command(for: agent)
        switch agent {
        case .claude:
            return claudeEntries(file: Paths.claudeSettings)
        case .codex:
            return Entries(file: Paths.codexHooks, events: [
                "SessionStart", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PostToolUse",
                "Stop", "Interrupt", "SessionEnd",
            ], handler: ["type": "command", "command": command, "async": true, "timeout": 5])
        case .gemini:
            // Gemini CLI waits for its hooks, and counts their timeout in milliseconds.
            return Entries(file: Paths.geminiSettings, events: [
                "SessionStart", "BeforeAgent", "BeforeTool", "AfterTool", "Notification", "AfterAgent",
                "SessionEnd",
            ], handler: ["type": "command", "command": command, "name": "smolcorp", "timeout": 5000])
        case .grok:
            return Entries(file: Paths.grokHooks, events: [
                "SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PostToolUseFailure",
                "Notification", "Stop", "StopFailure", "StopCancelled", "SessionEnd",
            ], handler: ["type": "command", "command": command], isOwnFile: true)
        case .opencode:
            return nil // a plugin: see `writePlugin()`
        }
    }

    private static func settingsFolder(of agent: Agent) -> URL {
        let path = [
            .claude: ".claude", .codex: ".codex", .gemini: ".gemini", .opencode: ".config/opencode", .grok: ".grok",
        ][agent]!
        return Paths.home.appendingPathComponent(path, isDirectory: true)
    }

    /// Agents with a settings folder on this Mac.
    static var found: [Agent] { found(profiles: nil) }

    static func found(profiles: [ClaudeProfile]?) -> [Agent] {
        Agent.allCases.filter { $0 == .claude ? !claudeProfiles(profiles).isEmpty : FileManager.default.fileExists(atPath: settingsFolder(of: $0).path) }
    }

    /// The Claude accounts whose folder is there.
    private static func claudeProfiles(_ profiles: [ClaudeProfile]? = nil) -> [ClaudeProfile] {
        (profiles ?? ClaudeProfiles.current).filter { FileManager.default.fileExists(atPath: $0.dir) }
    }

    static func isConnected(_ agent: Agent) -> Bool {
        if agent == .claude { return claudeProfiles().allSatisfy(isConnected) }
        guard let entries = entries(for: agent) else {
            return [Paths.openCodePlugin, Paths.oldOpenCodePlugin].contains { FileManager.default.fileExists(atPath: $0.path) }
        }
        guard let hooks = (try? read(entries.file))?["hooks"] as? [String: Any] else { return false }
        return entries.events.allSatisfy { event in
            ((hooks[event] as? [[String: Any]]) ?? []).contains { group in
                ((group["hooks"] as? [[String: Any]]) ?? []).contains(where: isOurs)
            }
        }
    }

    /// The deck is in one Claude account's hooks for every event.
    static func isConnected(_ profile: ClaudeProfile) -> Bool {
        let entries = claudeEntries(file: profile.settings)
        guard let hooks = (try? read(entries.file))?["hooks"] as? [String: Any] else { return false }
        return entries.events.allSatisfy { event in
            ((hooks[event] as? [[String: Any]]) ?? []).contains { group in
                ((group["hooks"] as? [[String: Any]]) ?? []).contains(where: isOurs)
            }
        }
    }

    /// Every agent found is connected.
    static var isConnected: Bool {
        let agents = found
        return !agents.isEmpty && agents.allSatisfy(isConnected)
    }

    /// Connects every agent found. One whose settings can't be read is left
    /// alone, and named in the error once the others are done. With
    /// `turningOnVoice`, Claude Code's voice input is turned on in tap mode —
    /// which the mic key needs — unless voice is already set up some other
    /// way; setup's repair connects without it. `notice` is then what to tell
    /// you, set even when connecting fails after it: nil when voice was left as it was.
    static func connect(turningOnVoice: Bool = true, notice: inout String?) throws {
        guard !TestCopy.isOn else { throw HookError.testCopy }
        let agents = found
        guard !agents.isEmpty else { throw HookError.noAgents }
        try updateFiles()
        moveGrokHooks()
        var unreadable: [URL] = []
        for agent in agents {
            do {
                if try add(agent, turningOnVoice: turningOnVoice) { notice = voiceNotice }
            } catch HookError.unreadableSettings(let files) {
                unreadable += files
            }
        }
        if !unreadable.isEmpty { throw HookError.unreadableSettings(unreadable) }
    }

    /// What setup tells you when connecting turned Claude Code's voice on.
    static let voiceNotice = "Claude Code's voice input is now on, in tap mode, for the phone's mic. "
        + "Disconnecting turns it off again; a voice setting of your own is never changed."

    /// The voice entry connecting writes into Claude Code's settings.
    static let smolcorpVoice: [String: Any] = ["enabled": true, "mode": "tap"]

    /// Claude Code's settings with Smolcorp's voice entry, when voice isn't set
    /// at all; nil when it is, as you or an earlier connect left it.
    static func withSmolcorpVoice(_ settings: [String: Any]) -> [String: Any]? {
        guard settings["voice"] == nil else { return nil }
        var settings = settings
        settings["voice"] = smolcorpVoice
        return settings
    }

    /// Claude Code's settings without the voice entry, when Smolcorp turned it
    /// on (`marked`) and it's still exactly as written; otherwise as they are.
    static func withoutSmolcorpVoice(_ settings: [String: Any], marked: Bool) -> [String: Any] {
        guard marked, let voice = settings["voice"] as? NSDictionary, voice.isEqual(to: smolcorpVoice) else { return settings }
        var settings = settings
        settings["voice"] = nil
        return settings
    }

    /// Brings agents connected by an earlier version up to this one's list
    /// of events, so an update that follows more of them works without
    /// connecting again. Agents the deck isn't in are left alone, and so is
    /// voice: only connecting turns it on.
    static func refresh(profiles: [ClaudeProfile]? = nil) throws {
        moveGrokHooks()
        let accounts = claudeProfiles(profiles)
        let connected = found(profiles: profiles).filter { $0 == .claude ? accounts.contains(where: hasEntries) : hasEntries($0) }
        guard !connected.isEmpty else { return }
        try updateFiles()
        for agent in connected where agent != .claude && !isConnected(agent) { try? add(agent, turningOnVoice: false) }
        refreshClaude(accounts)
    }

    /// Each Claude account on its own: one you disconnected stays so, and an
    /// account with some of the deck's entries gets the rest.
    static func refreshClaude(_ accounts: [ClaudeProfile]) {
        for profile in accounts where hasEntries(profile) && !isConnected(profile) {
            try? addClaude(to: profile, turningOnVoice: false)
        }
    }

    /// Grok's hooks, in their own file under the name the app had before:
    /// written to the new file and the old one taken out, so each event is
    /// reported once.
    private static func moveGrokHooks() {
        guard FileManager.default.fileExists(atPath: Paths.oldGrokHooks.path) else { return }
        try? FileManager.default.removeItem(at: Paths.oldGrokHooks)
        if !isConnected(.grok) { try? add(.grok, turningOnVoice: false) }
    }

    /// The deck is in `agent`'s hooks for at least one event.
    private static func hasEntries(_ agent: Agent) -> Bool {
        if agent == .claude { return claudeProfiles().contains(where: hasEntries) }
        guard let entries = entries(for: agent) else { return isConnected(agent) }
        return hasEntries(in: entries.file)
    }

    /// The deck is in one Claude account's hooks for at least one event.
    static func hasEntries(_ profile: ClaudeProfile) -> Bool { hasEntries(in: profile.settings) }

    private static func hasEntries(in file: URL) -> Bool {
        guard let hooks = (try? read(file))?["hooks"] as? [String: Any] else { return false }
        return hooks.values.contains { value in
            (value as? [[String: Any]] ?? []).contains { group in
                ((group["hooks"] as? [[String: Any]]) ?? []).contains(where: isOurs)
            }
        }
    }

    /// Adds the deck's entries for `agent`, replacing any it had, and says
    /// whether it turned Claude Code's voice on.
    @discardableResult
    private static func add(_ agent: Agent, turningOnVoice: Bool) throws -> Bool {
        if agent == .claude { return try addClaude(to: claudeProfiles(), turningOnVoice: turningOnVoice) }
        guard let entries = entries(for: agent) else {
            try writePlugin()
            lookAtOpenCode(always: true)
            return false
        }
        return try add(entries, turningOnVoice: false)
    }

    /// Every account's settings, the others done even if one can't be read,
    /// which is named once they are. Voice is only ever turned on in ~/.claude's.
    private static func addClaude(to profiles: [ClaudeProfile], turningOnVoice: Bool) throws -> Bool {
        var voiceOn = false
        var unreadable: [URL] = []
        for profile in profiles {
            do {
                if try addClaude(to: profile, turningOnVoice: turningOnVoice && profile.isDefault) { voiceOn = true }
            } catch HookError.unreadableSettings(let files) {
                unreadable += files
            }
        }
        if !unreadable.isEmpty { throw HookError.unreadableSettings(unreadable) }
        return voiceOn
    }

    /// The deck's entries in one Claude account, replacing any it had.
    @discardableResult
    static func addClaude(to profile: ClaudeProfile, turningOnVoice: Bool) throws -> Bool {
        try add(claudeEntries(file: profile.settings), turningOnVoice: turningOnVoice)
    }

    private static func add(_ entries: Entries, turningOnVoice: Bool) throws -> Bool {
        var settings = entries.isOwnFile ? [:] : try read(entries.file)
        var hooks = withoutOurs(settings["hooks"] as? [String: Any] ?? [:])
        for event in entries.events {
            hooks[event] = (hooks[event] as? [[String: Any]] ?? []) + [["hooks": [entries.handler]]]
        }
        settings["hooks"] = hooks
        var voiceOn = false
        if turningOnVoice {
            if let withVoice = withSmolcorpVoice(settings) {
                settings = withVoice
                voiceOn = true
            } else {
                // A voice setting Smolcorp didn't just write: an old marker can't vouch for it
                try? FileManager.default.removeItem(at: Paths.voiceMarker)
            }
        }
        try write(settings, to: entries.file, backup: !entries.isOwnFile)
        // Settings first, then the marker: a failed marker leaves voice on, never turns off one that was yours
        if voiceOn { _ = Paths.writePrivately(Data("1".utf8), to: Paths.voiceMarker) }
        return voiceOn
    }

    /// Takes the deck out of every agent. One that can't be changed is left
    /// alone and the others go on; if some were disconnected and some weren't,
    /// `partlyDisconnected` names the ones that weren't. If none could be, the
    /// first error is thrown as it is.
    static func disconnect() throws {
        guard !TestCopy.isOn else { throw HookError.testCopy }
        var failed: [(agent: Agent, error: Error)] = []
        var changed = 0
        for agent in Agent.allCases {
            do {
                if try disconnect(agent) { changed += 1 }
            } catch {
                failed.append((agent, error))
            }
        }
        guard let first = failed.first else { return }
        guard changed > 0 else { throw first.error }
        throw HookError.partlyDisconnected(failed.map(\.agent.name))
    }

    /// Takes the deck out of one agent, and says whether anything was changed.
    private static func disconnect(_ agent: Agent) throws -> Bool {
        let fm = FileManager.default
        if agent == .claude {
            var changed = false
            var failure: Error?
            for profile in claudeProfiles() {
                do { changed = try removeClaude(from: profile) || changed } catch { failure = failure ?? error }
            }
            if let failure { throw failure }
            return changed
        }
        guard let entries = entries(for: agent) else {
            let there = [Paths.openCodePlugin, Paths.oldOpenCodePlugin].filter { fm.fileExists(atPath: $0.path) }
            for plugin in there { try? fm.removeItem(at: plugin) }
            return !there.isEmpty
        }
        if entries.isOwnFile {
            let there = [entries.file, Paths.oldGrokHooks].filter { fm.fileExists(atPath: $0.path) }
            for file in there { try? fm.removeItem(at: file) }
            return !there.isEmpty
        }
        return try remove(from: entries.file, voice: false)
    }

    /// Takes the deck out of one Claude account, and says whether anything
    /// was changed. Voice goes off only in ~/.claude's, where it was turned on.
    @discardableResult
    static func removeClaude(from profile: ClaudeProfile) throws -> Bool {
        try remove(from: profile.settings, voice: profile.isDefault)
    }

    private static func remove(from file: URL, voice: Bool) throws -> Bool {
        let fm = FileManager.default
        guard fm.fileExists(atPath: file.path) else { return false }
        var settings = try read(file)
        // Voice goes off only if Smolcorp turned it on and it's still as written
        let marked = voice && fm.fileExists(atPath: Paths.voiceMarker.path)
        // Nothing of Smolcorp's in it: the file is left exactly as it is, not even tidied or backed up.
        guard hasEntries(in: file) || marked else { return false }
        let hooks = withoutOurs(settings["hooks"] as? [String: Any] ?? [:])
        settings["hooks"] = hooks.isEmpty ? nil : hooks
        settings = withoutSmolcorpVoice(settings, marked: marked)
        try write(settings, to: file, backup: true)
        // Only once the settings are written: a failed write keeps the marker
        if marked { try? fm.removeItem(at: Paths.voiceMarker) }
        return true
    }

    /// The hook script, copied out of the app with the deck's port filled in,
    /// to a path that stays put when the app is moved or updated, and
    /// OpenCode's plugin, which runs it. Rewritten at every launch, so an
    /// updated app brings them up to date too.
    static func updateFiles() throws {
        try writeScript()
        // Under the name the app had before, it moves to the new one
        if isConnected(.opencode) {
            // Under the old name, it's written under the new one first, so it stays connected
            if FileManager.default.fileExists(atPath: Paths.oldOpenCodePlugin.path) {
                try writePlugin()
                try? FileManager.default.removeItem(at: Paths.oldOpenCodePlugin)
            }
            // For the OpenCode found: what's there now stays until it's known which
            lookAtOpenCode(always: true)
        }
    }

    /// The hook script alone, with this deck's port filled in, in the support folder.
    static func writeScript(template: URL = Paths.bundledHookScript, to target: URL = Paths.hookScript, port: UInt16 = Deck.port) throws {
        let text = try String(contentsOf: template, encoding: .utf8)
        try text.replacingOccurrences(of: "{{PORT}}", with: String(port)).write(to: target, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: target.path)
    }

    /// The OpenCode the plugin was last written for, by where it is and when
    /// it last changed, and its major version: see `lookAtOpenCode()`. Only
    /// touched on the main thread.
    private static var openCodeStamp: String?
    private static var openCodeMajor: Int?

    /// OpenCode's plugin, which runs the hook: OpenCode has no command hooks.
    /// It's written for the OpenCode last found, OpenCode 1's own for 1, the
    /// default for 2 or one whose version can't be told, and only when it
    /// changes, as OpenCode loads it again each time it does. Written at once,
    /// so connecting says if it can't be; `lookAtOpenCode(always:)` then
    /// finds which OpenCode it's for.
    private static func writePlugin() throws {
        let bundled = openCodeMajor == 1 ? Paths.bundledOpenCodePluginV1 : Paths.bundledOpenCodePlugin
        let template = try String(contentsOf: bundled, encoding: .utf8)
        let command = try JSONSerialization.data(withJSONObject: [Paths.hookScript.path, Agent.opencode.rawValue],
                                                 options: .withoutEscapingSlashes)
        let plugin = template.replacingOccurrences(of: #"["{{COMMAND}}"]"#, with: String(decoding: command, as: UTF8.self))
        guard (try? String(contentsOf: Paths.openCodePlugin, encoding: .utf8)) != plugin else { return }
        try FileManager.default.createDirectory(at: Paths.openCodePlugin.deletingLastPathComponent(),
                                                withIntermediateDirectories: true)
        try plugin.write(to: Paths.openCodePlugin, atomically: true, encoding: .utf8)
    }

    /// Finds which OpenCode is installed and writes its plugin again if it's
    /// changed: its version is asked off the main thread, so the deck's
    /// server never waits on it, and the plugin written on it. `always` asks
    /// even when the program looks unchanged, as when connecting.
    static func lookAtOpenCode(always: Bool = false) {
        DispatchQueue.global(qos: .utility).async {
            let stamp = OpenCodeInstall.stamp()
            DispatchQueue.main.async {
                guard isConnected(.opencode), always || stamp != openCodeStamp else { return }
                DispatchQueue.global(qos: .utility).async {
                    let major = OpenCodeInstall.major()
                    DispatchQueue.main.async {
                        guard isConnected(.opencode) else { return }
                        (openCodeStamp, openCodeMajor) = (stamp, major)
                        try? writePlugin()
                    }
                }
            }
        }
    }

    /// Looks about once a minute whether OpenCode was updated, as it updates
    /// itself, and if so writes its plugin again for the version it is now.
    static func watchOpenCode() {
        Timer.scheduledTimer(withTimeInterval: 60, repeats: true) { _ in lookAtOpenCode() }
    }

    private static func isOurs(_ hook: [String: Any]) -> Bool {
        (hook["command"] as? String)?.contains(Paths.hookScript.path) ?? false
    }

    private static func withoutOurs(_ hooks: [String: Any]) -> [String: Any] {
        var result: [String: Any] = [:]
        for (event, value) in hooks {
            let groups = (value as? [[String: Any]] ?? []).compactMap { group -> [String: Any]? in
                var group = group
                let kept = (group["hooks"] as? [[String: Any]] ?? []).filter { !isOurs($0) }
                guard !kept.isEmpty else { return nil }
                group["hooks"] = kept
                return group
            }
            if !groups.isEmpty { result[event] = groups }
        }
        return result
    }

    /// Only a file that isn't there at all is empty. One that is there but
    /// can't be read, a link to nothing, or one that isn't JSON is named in
    /// the error, so it's never written over with the deck's entries alone.
    static func read(_ file: URL) throws -> [String: Any] {
        let data: Data
        do {
            data = try Data(contentsOf: file)
        } catch CocoaError.fileReadNoSuchFile
                    where (try? FileManager.default.destinationOfSymbolicLink(atPath: file.path)) == nil {
            return [:]
        } catch {
            throw HookError.unreadableSettings([file])
        }
        guard let settings = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] else {
            throw HookError.unreadableSettings([file])
        }
        return settings
    }

    /// The backups beside a settings file: `<file>.agentdeck.bak` is the file
    /// as it was before the deck first changed it, kept from then on, and
    /// `<file>.agentdeck-last.bak` as it was before the latest change.
    static func backups(of file: URL) -> (first: URL, last: URL) {
        (file.appendingPathExtension("agentdeck.bak"), URL(fileURLWithPath: file.path + ".agentdeck-last.bak"))
    }

    /// A link is followed, so the file it points to, in a dotfiles folder
    /// say, is the one that changes, and the link stays. The backups are made
    /// first, or nothing is written.
    static func write(_ settings: [String: Any], to file: URL, backup: Bool) throws {
        let fm = FileManager.default
        let file = file.resolvingSymlinksInPath()
        try fm.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        if backup && fm.fileExists(atPath: file.path) {
            let (first, last) = backups(of: file)
            if (try? first.checkResourceIsReachable()) != true {
                try fm.copyItem(at: file, to: first)
            }
            try? fm.removeItem(at: last)
            try fm.copyItem(at: file, to: last)
        }
        let options: JSONSerialization.WritingOptions = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
        let data = try JSONSerialization.data(withJSONObject: settings, options: options)
        try data.write(to: file, options: .atomic)
    }

    enum HookError: LocalizedError {
        case noAgents
        case unreadableSettings([URL])
        /// A test copy of the app leaves your agents' settings alone.
        case testCopy
        /// Some agents were disconnected and these (by name) couldn't be.
        case partlyDisconnected([String])

        var errorDescription: String? {
            switch self {
            case .noAgents:
                return "No coding agent found on this Mac. Install one, open it once, then connect again."
            case .unreadableSettings(let files):
                let names = files.map { $0.path.replacingOccurrences(of: Paths.home.path, with: "~") }
                return "\(names.joined(separator: " and ")) can't be read or isn't valid JSON. Fix it, then try again."
            case .partlyDisconnected(let names):
                return "Disconnected, except \(names.joined(separator: " and ")): its settings couldn't be changed. Fix them, then disconnect again."
            case .testCopy:
                return "This is a test copy of Smolcorp: it doesn't change your agents' settings. Report to it with a hook of its own."
            }
        }
    }
}
