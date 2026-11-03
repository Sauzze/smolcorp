import AppKit
import ApplicationServices

/// Keys pressed in another app for you, only if you allowed it (`Config.keystrokesAllowed`,
/// off until you turn it on in Smolcorp's setup). Nothing calls this yet but the setting and the
/// tests: a feature that needs a key asks here, and here is where the safeguards live.
///
/// Every call, and every single key inside it, is checked first:
/// - the setting is on, and macOS has given Smolcorp Accessibility;
/// - the app in front is exactly the one named. If you click elsewhere halfway, it stops;
/// - no file dialog is open there (an Open or Save panel, or any sheet or dialog we can't
///   tell from one, or anything we can't read). A path is never typed, and nothing is typed
///   into a dialog;
/// - the app is on your list of allowed apps (`Config.keystrokeApps`, empty until you add one);
/// - the app is not a terminal (or Smolcorp itself): see `neverInto`. An agent may be waiting there
///   for an answer, and a key must never be that answer;
/// - you aren't using the Mac: before every key you must have been idle 2 seconds. Our keys go to
///   the app's process alone from a private source, so they don't count as you.
/// - the key would land in a plain text field, never a password field or anything else.
///
/// A file is never typed as a path: it's dropped onto the app, or refused. The text itself is never
/// logged: the event log gets the app, the action and how it ended. The keys go to that app's
/// process alone, so they never reach whatever else is in front.
struct Keystrokes {
    enum Action: String { case type, press, drop }

    /// Why nothing (more) was sent.
    enum Refusal: String, Error, Equatable {
        case off, noAccessibility, blockedApp, notAllowedApp, returnNotAllowed, wrongApp, fileDialog, userActive, postFailed, badText, tooLong, noDrop, notAFile
    }

    /// How a call ended: `sent` keys went out; `stoppedBy` is why it stopped short, or nil when it finished.
    struct Outcome: Equatable {
        var sent = 0
        var stoppedBy: Refusal?
        var finished: Bool { stoppedBy == nil }
    }

    enum Key: String, Equatable {
        case `return`, tab, escape, delete, space, left, right, up, down

        /// macOS's virtual key code.
        var code: CGKeyCode {
            switch self {
            case .return: return 36
            case .tab: return 48
            case .space: return 49
            case .delete: return 51
            case .escape: return 53
            case .left: return 123
            case .right: return 124
            case .down: return 125
            case .up: return 126
            }
        }
    }

    struct Modifiers: OptionSet, Equatable {
        let rawValue: Int
        static let shift = Modifiers(rawValue: 1), control = Modifiers(rawValue: 2)
        static let option = Modifiers(rawValue: 4), command = Modifiers(rawValue: 8)

        var flags: CGEventFlags {
            var flags: CGEventFlags = []
            if contains(.shift) { flags.insert(.maskShift) }
            if contains(.control) { flags.insert(.maskControl) }
            if contains(.option) { flags.insert(.maskAlternate) }
            if contains(.command) { flags.insert(.maskCommand) }
            return flags
        }
    }

    /// What happened to one key.
    enum PostResult: Equatable { case sent, wrongApp, failed }

    /// One key to send.
    enum Stroke: Equatable {
        case character(Character)
        case key(Key, Modifiers)
    }

    /// Everything that touches the Mac, so a test can stand in for it and no real key is ever sent.
    struct Environment {
        var allowed: () -> Bool
        /// The apps you allowed (`Config.keystrokeApps`), read at every key. Empty means none.
        var allowedApps: () -> [String]
        var accessibility: () -> Bool
        /// The bundle id of the app in front, or nil when it can't be told.
        var frontmost: () -> String?
        /// Seconds since you last touched the keyboard, mouse or trackpad. Our own keys don't count: they
        /// are posted straight to one app's process from a private source and never enter that stream.
        var idleSeconds: () -> TimeInterval
        /// A clock in seconds that only goes forward.
        var now: () -> TimeInterval
        /// Whether a file dialog may be open in that app. True when unsure.
        var dialogShowing: (String) -> Bool
        /// Sends one key to that app. Live, it checks once more that the app is still in front.
        var post: (Stroke, String) -> PostResult
        /// Drops a file onto that app, true if it was done.
        var drop: (URL, String) -> Bool
        /// A short wait between keys, so the app keeps up and you can take over.
        var pause: () -> Void
        /// The event log: app, action, how it ended. Never the text.
        var log: (_ app: String, _ action: Action, _ result: String) -> Void
    }

    /// The most one `type` sends, so a mistake can't run on.
    static let textMost = 2000

    /// Apps nothing is ever typed or pressed into, checked before every key: the terminals and the editors
    /// with one built in (VS Code and its forks, Zed, every JetBrains IDE, Android Studio) (an agent there may be waiting on a permission prompt, and a key must never answer it),
    /// and Smolcorp itself; and the apps that hold secrets or ask for permissions: the Claude desktop app (its Claude Code
    /// has permission prompts), Passwords, System Settings, Keychain Access and 1Password.
    static let neverInto: Set<String> = [
        "com.apple.terminal", "com.googlecode.iterm2", "com.mitchellh.ghostty", "com.github.wez.wezterm",
        "net.kovidgoyal.kitty", "org.alacritty", "io.alacritty", "dev.warp.warp-stable", "dev.warp.warp",
        "co.zeit.hyper", "org.tabby", "io.github.sauzze.agentdeck",
        "com.microsoft.vscode", "com.microsoft.vscodeinsiders", "com.vscodium", "com.todesktop.230313mzl4w4u92",
        "com.exafunction.windsurf", "dev.zed.zed", "dev.zed.zed-preview", "com.google.android.studio",
        "com.anthropic.claudefordesktop", "com.apple.passwords", "com.apple.systempreferences", "com.apple.keychainaccess",
        "com.1password.1password", "com.agilebits.onepassword7",
    ]

    /// Whether `bundleID` is on the block-list (any capitals), or is this running app.
    static func isNeverInto(_ bundleID: String) -> Bool {
        let id = bundleID.lowercased()
        // Every JetBrains IDE has a terminal built in, whatever its product id.
        return neverInto.contains(id) || id.hasPrefix("com.jetbrains.") || id == Bundle.main.bundleIdentifier?.lowercased()
    }

    /// The list of apps you allow, with the rules for what may be on it. Two locks: this list, and the
    /// block-list, which no list lifts.
    enum AppList {
        enum Problem: String, Error, Equatable {
            case blocked, badID, alreadyThere
            var reason: String {
                switch self {
                case .blocked: return "Keys are never sent to a terminal, an editor with a terminal, Claude, a password or settings app, or Smolcorp itself."
                case .badID: return "That isn't an app's id."
                case .alreadyThere: return "That app is already on the list."
                }
            }
        }

        static let most = 50

        /// Whether `id` looks like a bundle id: letters, digits, dots, dashes, underscores, up to 200.
        static func isValid(_ id: String) -> Bool {
            !id.isEmpty && id.count <= 200 && id.contains(".")
                && id.unicodeScalars.allSatisfy { $0.isASCII && (CharacterSet.alphanumerics.contains($0) || ".-_".unicodeScalars.contains($0)) }
        }

        /// Whether `list` has `id` (any capitals).
        static func contains(_ list: [String], _ id: String) -> Bool {
            list.contains { $0.caseInsensitiveCompare(id) == .orderedSame }
        }

        static func adding(_ id: String, to list: [String]) -> Result<[String], Problem> {
            guard isValid(id), list.count < most else { return .failure(.badID) }
            guard !Keystrokes.isNeverInto(id) else { return .failure(.blocked) }
            guard !contains(list, id) else { return .failure(.alreadyThere) }
            return .success(list + [id])
        }

        static func removing(_ id: String, from list: [String]) -> [String] {
            list.filter { $0.caseInsensitiveCompare(id) != .orderedSame }
        }

        /// A list read from disk: only valid, allowed, new ids are kept.
        static func checked(_ list: [String]) -> [String] {
            list.reduce(into: [String]()) { kept, id in
                if case .success(let more) = adding(id, to: kept) { kept = more }
            }
        }
    }

    let env: Environment

    // MARK: The three things it does

    /// Types `text` into the app `bundleID`, a key at a time. Text only: no new lines or other control
    /// characters (send Return with `press`, as in a chat it sends the message).
    func type(_ text: String, into bundleID: String) -> Outcome {
        guard text.count <= Keystrokes.textMost else { return finish(.type, bundleID, Outcome(stoppedBy: .tooLong)) }
        guard !text.isEmpty, !text.unicodeScalars.contains(where: { CharacterSet.controlCharacters.contains($0) }) else {
            return finish(.type, bundleID, Outcome(stoppedBy: .badText))
        }
        return send(text.map { Stroke.character($0) }, .type, bundleID)
    }

    /// Presses one key (with these modifiers) in the app `bundleID`. Return sends a message in a chat, so
    /// it is refused unless the caller names this very app in `allowReturnIn`.
    func press(_ key: Key, _ modifiers: Modifiers = [], into bundleID: String, allowReturnIn: String? = nil) -> Outcome {
        if key == .return, allowReturnIn != bundleID { return finish(.press, bundleID, Outcome(stoppedBy: .returnNotAllowed)) }
        return send([.key(key, modifiers)], .press, bundleID)
    }

    /// Drops the file at `url` onto the app `bundleID`, the way you would drag it from Finder.
    /// Refused if it isn't a file that exists, or if the drop can't be done.
    func dropFile(_ url: URL, onto bundleID: String) -> Outcome {
        guard url.isFileURL, FileManager.default.fileExists(atPath: url.path) else {
            return finish(.drop, bundleID, Outcome(stoppedBy: .notAFile))
        }
        if let why = refusal(for: bundleID) { return finish(.drop, bundleID, Outcome(stoppedBy: why)) }
        return finish(.drop, bundleID, env.drop(url, bundleID) ? Outcome(sent: 1) : Outcome(stoppedBy: .noDrop))
    }

    // MARK: The checks

    /// Why a key to `bundleID` may not go now, or nil if it may. The order matters little, but the
    /// cheap, certain ones come first.
    func refusal(for bundleID: String) -> Refusal? {
        guard env.allowed() else { return .off }
        guard env.accessibility() else { return .noAccessibility }
        guard !Keystrokes.isNeverInto(bundleID) else { return .blockedApp }
        guard AppList.contains(env.allowedApps(), bundleID) else { return .notAllowedApp }
        guard env.frontmost() == bundleID else { return .wrongApp }
        guard !UserActivity.isActive(idle: env.idleSeconds()) else { return .userActive }
        guard !env.dialogShowing(bundleID) else { return .fileDialog }
        return nil
    }

    private func send(_ strokes: [Stroke], _ action: Action, _ bundleID: String) -> Outcome {
        var outcome = Outcome()
        for stroke in strokes {
            // Before every key, as the app in front or you may have changed since the last.
            if let why = refusal(for: bundleID) { outcome.stoppedBy = why; break }
            switch env.post(stroke, bundleID) {
            case .sent: break
            case .wrongApp: outcome.stoppedBy = .wrongApp
            case .failed: outcome.stoppedBy = .postFailed
            }
            if outcome.stoppedBy != nil { break }
            outcome.sent += 1
            env.pause()
        }
        return finish(action, bundleID, outcome)
    }

    private func finish(_ action: Action, _ bundleID: String, _ outcome: Outcome) -> Outcome {
        let result = outcome.stoppedBy.map { "stopped: \($0.rawValue), \(outcome.sent) sent" } ?? "done, \(outcome.sent) sent"
        env.log(bundleID, action, result)
        return outcome
    }
}

// MARK: - The live Mac

extension Keystrokes {
    /// The real thing, for `config` (read each time, so turning it off takes effect at the next key).
    static func live(config: @escaping () -> Bool, apps: @escaping () -> [String], log: @escaping (String, Action, String) -> Void) -> Keystrokes {
        Keystrokes(env: Environment(
            allowed: config,
            allowedApps: apps,
            accessibility: { Permissions.hasAccessibility },
            frontmost: { NSWorkspace.shared.frontmostApplication?.bundleIdentifier },
            idleSeconds: Live.userIdleSeconds,
            now: { ProcessInfo.processInfo.systemUptime },
            dialogShowing: Live.fileDialogMayBeOpen,
            post: Live.post,
            drop: { _, _ in false },
            pause: { Thread.sleep(forTimeInterval: 0.012) },
            log: log))
    }

    /// The event log line for a change of the allowed apps: the id alone.
    static func appListChange(added: Bool, id: String) -> String { "\(added ? "app allowed" : "app removed"): \(id)" }

    /// Writes one use to the event log. It's used on the main thread, so this hops there.
    static func logToEvents(_ events: @escaping () -> EventLog) -> (String, Action, String) -> Void {
        { app, action, result in
            DispatchQueue.main.async {
                _ = events().append(EventLog.Entry(at: Date().timeIntervalSince1970 * 1000, kind: .keystroke,
                                                    detail: "\(action.rawValue) \(app): \(result)"))
            }
        }
    }

    /// The apps open now that could be added: name and id, without those already on the list or on the block-list.
    static func runningApps(except listed: [String]) -> [[String: String]] {
        NSWorkspace.shared.runningApplications.compactMap { app in
            guard app.activationPolicy == .regular, let id = app.bundleIdentifier, let name = app.localizedName,
                  !isNeverInto(id), !AppList.contains(listed, id), AppList.isValid(id) else { return nil }
            return ["name": name, "id": id]
        }.sorted { ($0["name"] ?? "").localizedCaseInsensitiveCompare($1["name"] ?? "") == .orderedAscending }
    }

    /// The parts that ask macOS. Not run by any test.
    enum Live {
        /// Seconds since you last used the keyboard or mouse. It reads the hardware state (`.hidSystemState`),
        /// and our keys, posted to one process from a private source, are not part of it.
        static func userIdleSeconds() -> TimeInterval {
            let any = CGEventType(rawValue: ~0)!
            return CGEventSource.secondsSinceLastEventType(.hidSystemState, eventType: any)
        }

        static func post(_ stroke: Stroke, _ bundleID: String) -> PostResult {
            // Once more, right before the key: still the app in front, and the key goes to its process alone.
            guard let app = NSWorkspace.shared.frontmostApplication, app.bundleIdentifier == bundleID else { return .wrongApp }
            let source = CGEventSource(stateID: .privateState)
            let code: CGKeyCode, flags: CGEventFlags, text: [UniChar]?
            switch stroke {
            case .character(let character): code = 0; flags = []; text = Array(String(character).utf16)
            case .key(let key, let modifiers): code = key.code; flags = modifiers.flags; text = nil
            }
            guard let down = CGEvent(keyboardEventSource: source, virtualKey: code, keyDown: true),
                  let up = CGEvent(keyboardEventSource: source, virtualKey: code, keyDown: false) else { return .failed }
            for event in [down, up] {
                event.flags = flags
                if let text { event.keyboardSetUnicodeString(stringLength: text.count, unicodeString: text) }
                event.postToPid(app.processIdentifier)
            }
            return .sent
        }

        /// Whether a file dialog may be open in the app. Conservative: any dialog or sheet counts, and so
        /// does a window that can't be read.
        static func fileDialogMayBeOpen(_ bundleID: String) -> Bool {
            // A panel of our own, or the system's Open and Save service (which sandboxed apps use) on screen.
            if NSApp?.windows.contains(where: { $0 is NSSavePanel }) == true { return true }
            let listed = CGWindowListCopyWindowInfo([.optionOnScreenOnly], kCGNullWindowID) as? [[String: Any]] ?? []
            if listed.contains(where: { FileDialog.isPanelService(owner: $0[kCGWindowOwnerName as String] as? String ?? "") }) { return true }

            guard let app = NSWorkspace.shared.runningApplications.first(where: { $0.bundleIdentifier == bundleID }) else { return true }
            let element = AXUIElementCreateApplication(app.processIdentifier)
            var window: CFTypeRef?
            switch AXUIElementCopyAttributeValue(element, kAXFocusedWindowAttribute as CFString, &window) {
            case .success: break
            case .noValue: return true // no window has focus: nothing to type into
            default: return true // can't read it
            }
            func text(_ attribute: String) -> String? {
                var value: CFTypeRef?
                guard AXUIElementCopyAttributeValue(window as! AXUIElement, attribute as CFString, &value) == .success else { return nil }
                return value as? String
            }
            if FileDialog.refuses(role: text(kAXRoleAttribute), subrole: text(kAXSubroleAttribute)) { return true }

            // The thing that has the keyboard: only a plain text field, never a password field.
            var focused: CFTypeRef?
            guard AXUIElementCopyAttributeValue(element, kAXFocusedUIElementAttribute as CFString, &focused) == .success,
                  let focused else { return true } // none, or can't read it
            func focusedText(_ attribute: String) -> String? {
                var value: CFTypeRef?
                guard AXUIElementCopyAttributeValue(focused as! AXUIElement, attribute as CFString, &value) == .success else { return nil }
                return value as? String
            }
            return FileDialog.refuses(focusedRole: focusedText(kAXRoleAttribute), subrole: focusedText(kAXSubroleAttribute))
        }
    }
}

/// Telling when you are using the Mac. Our own keys never count, so the rule is the same before
/// every key: you must have been idle for 2 seconds.
enum UserActivity {
    static let idleBeforeKey: TimeInterval = 2

    static func isActive(idle: TimeInterval) -> Bool { idle < idleBeforeKey }
}

/// What counts as a file dialog, apart from asking macOS.
enum FileDialog {
    /// A focused window of this role and subrole is treated as a file dialog. We can't tell an Open
    /// panel from other sheets and dialogs by role alone, so every sheet and dialog counts, and so does
    /// a window whose role or subrole can't be read.
    static func refuses(role: String?, subrole: String?) -> Bool {
        guard let role else { return true }
        if role == kAXSheetRole as String { return true }
        guard let subrole else { return false } // an ordinary window has none
        return ["AXDialog", "AXSystemDialog", "AXFloatingWindow", "AXSystemFloatingWindow"].contains(subrole)
    }

    /// Whether the element that has the keyboard is somewhere we must not type. Only a text field, text
    /// area or combo box is fine, and never a password field. A button, a list, a file browser, a web
    /// area (a browser or Electron page, which a later feature must decide on) or an element we can't read refuses.
    static func refuses(focusedRole: String?, subrole: String?) -> Bool {
        guard let focusedRole, ["AXTextField", "AXTextArea", "AXComboBox"].contains(focusedRole) else { return true }
        return subrole == "AXSecureTextField"
    }

    /// The window of the system's Open and Save panel service, by the name it's listed under.
    static func isPanelService(owner: String) -> Bool {
        owner.lowercased().contains("open and save panel")
    }
}
