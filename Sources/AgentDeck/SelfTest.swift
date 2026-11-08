import AppKit

/// The self-test: setup's "Start here" checklist, and what
/// `Smolcorp.app/Contents/MacOS/AgentDeck --setup` prints. Each check only
/// looks; what puts one right is a fix, asked for by its name, as the
/// Windows and Linux app has them (desktop/selftest.go).
struct Check: Equatable {
    enum Status: String { case ok, fix, todo, info }

    let id: String
    var status: Status
    let title: String
    var detail: String
    /// The fix's name for POST /setup/fix, and its button's words, where there's one to press.
    var fix: String?
    var label: String?

    var jsonObject: [String: Any] {
        var object: [String: Any] = ["id": id, "status": status.rawValue, "title": title, "detail": detail]
        if let fix { object["fix"] = fix }
        if let label { object["label"] = label }
        return object
    }
}

enum SelfTest {
    /// The fixes, by name.
    static let repair = "repair", install = "install", accessibility = "accessibility", login = "login", tailscale = "tailscale"

    /// The checklist, in the order it's done in. On the main queue.
    static func checks(deck: Deck) -> [Check] {
        let phones = deck.access.phones.all.filter { $0.id != PairedPhone.officeWindow.id }.count
        return [
            installCheck(),
            hooksCheck(found: Hooks.found.map(\.name), connected: Hooks.found.filter(Hooks.isConnected).map(\.name),
                       scriptThere: FileManager.default.fileExists(atPath: Paths.hookScript.path)),
            accessibilityCheck(Permissions.hasAccessibility),
            networkCheck(tailnet: Tailnet.url),
            phoneCheck(phones),
            loginCheck(LoginItem.isEnabled),
        ]
    }

    /// Whether nothing in `checks` is in the way: a step still to take isn't.
    static func ready(_ checks: [Check]) -> Bool { !checks.contains { $0.status == .fix } }

    static func report(_ checks: [Check]) -> [String: Any] {
        ["checks": checks.map(\.jsonObject), "ready": ready(checks)]
    }

    /// Whether the app is in Applications: not where a download opens it from,
    /// a disk image, or the copy macOS makes of an app it hasn't let out of
    /// Downloads yet (App Translocation), which changes at every launch.
    ///
    /// A translocated launch hides where the app really is (finding out takes
    /// a private macOS call), so the two usual homes are looked at instead:
    /// if the app sits in Applications and still carries the download's
    /// quarantine mark, that may be why it's translocated, and the owner is told
    /// the one command that ends it, beside the move (a launch from Downloads
    /// looks the same). `quarantined` says whether a path has
    /// that mark; it's read from the disk only by default.
    static func installCheck(bundle: URL = Bundle.main.bundleURL, testCopy: Bool = TestCopy.isOn,
                             home: String = NSHomeDirectory(),
                             quarantined: (String) -> Bool = SelfTest.hasQuarantine) -> Check {
        var check = Check(id: "install", status: .ok, title: "Smolcorp is in Applications", detail: "")
        let path = bundle.standardizedFileURL.path
        let translocated = path.contains("/AppTranslocation/")
        if testCopy {
            check.detail = "A test copy, beside the Smolcorp you use: it stays where it was built."
        } else if !translocated,
                  path.hasPrefix("/Applications/") || path.hasPrefix(home + "/Applications/") {
            check.detail = "As \(path)."
        } else if translocated {
            check.status = .fix
            let name = bundle.lastPathComponent
            let original = ["/Applications/\(name)", "\(home)/Applications/\(name)"].first(where: quarantined)
            check.fix = install
            check.label = "Move to Applications"
            if let original {
                check.detail = "macOS is opening a temporary copy of Smolcorp because it was just downloaded. "
                    + "If this is the copy you just downloaded, move it to Applications. "
                    + "If Smolcorp is already there, quit it, run this in Terminal, then open it again: "
                    + "xattr -d com.apple.quarantine '\(original)'. After that, allow Accessibility."
            } else {
                check.detail = "macOS is opening a temporary copy of Smolcorp because it was just downloaded. If it is not in Applications yet, "
                    + "move it there. If it is, quit Smolcorp, run this in Terminal, then open it again: "
                    + "xattr -d com.apple.quarantine '/Applications/\(name)'. After that, allow Accessibility."
            }
        } else {
            check.status = .fix
            check.fix = install
            check.label = "Move to Applications"
            check.detail = "It's running from \(bundle.deletingLastPathComponent().path), where macOS may not keep it. "
                + "In Applications it stays put, and opens at login."
        }
        return check
    }

    /// Whether the file or app at `path` carries the mark macOS puts on downloads.
    static func hasQuarantine(_ path: String) -> Bool {
        getxattr(path, "com.apple.quarantine", nil, 0, 0, XATTR_NOFOLLOW) >= 0
    }

    /// How a test copy hears from its hires: through its own hook, whatever your agents' settings hold. Said by this
    /// check and by the GM preflight's hooks line (GMPreflight.items), both ticked, so Start here, its hire's lines and
    /// General never disagree on it.
    static let testCopyHires = "Your hires report to this test copy through its own hook."

    /// Whether every agent found is hooked up. Their hooks run report.sh in
    /// Smolcorp's own folder, never the app where it happens to be, so it's
    /// enough that it's there. A test copy's hires report through its own hook: ticked.
    static func hooksCheck(found: [String], connected: [String], scriptThere: Bool, testCopy: Bool = TestCopy.isOn) -> Check {
        var check = Check(id: "hooks", status: .ok, title: "Your agents report to Smolcorp", detail: "")
        if testCopy {
            check.detail = testCopyHires + " Sessions you start yourself report to the Smolcorp you use."
            return check
        }
        guard !found.isEmpty else {
            check.status = .fix
            check.detail = Hooks.HookError.noAgents.localizedDescription
            return check
        }
        let missing = found.filter { !connected.contains($0) }
        if !missing.isEmpty || !scriptThere {
            check.status = .fix
            check.fix = repair
            check.label = "Connect"
            check.detail = missing.isEmpty ? "Their hook script is missing from Smolcorp's folder."
                : missing.joined(separator: ", ") + (missing.count == 1 ? " isn't" : " aren't") + " connected."
            return check
        }
        check.detail = found.joined(separator: ", ") + ": each new session shows up at a desk."
        return check
    }

    /// Accessibility, said the same way here and in General (setup.html draws both from this line).
    static func accessibilityCheck(_ allowed: Bool) -> Check {
        var check = Check(id: "accessibility", status: .ok, title: "Smolcorp can use your keyboard", detail: "")
        if allowed {
            check.detail = "Your phone's keys, the buttons, the mic and scrolling reach your agents."
        } else {
            check.status = .todo
            check.fix = accessibility
            check.label = "Allow"
            check.detail = "Your phone's keys, the buttons, the mic and scrolling need it. Turn Smolcorp on in the list that opens. After an update, turn it on again."
        }
        return check
    }

    static func networkCheck(tailnet: String?) -> Check {
        var check = Check(id: "network", status: .ok, title: "Your phone can reach this Mac", detail: "")
        if let tailnet {
            check.detail = "On this Wi-Fi, and through Tailscale at \(tailnet) from anywhere."
        } else {
            check.status = .info
            check.fix = tailscale
            check.label = "Get Tailscale"
            check.detail = "On this Wi-Fi. With Tailscale, free, on this Mac and your phone, it reaches you from anywhere."
        }
        return check
    }

    static func phoneCheck(_ phones: Int) -> Check {
        var check = Check(id: "phone", status: .ok, title: "A phone is paired", detail: "\(phones) paired. Pair another from Phone and voice.")
        if phones == 0 {
            check.status = .todo
            check.detail = "Open Pair a phone, then scan its QR code with your phone's camera, or type its 6-digit code on your phone."
        }
        return check
    }

    static func loginCheck(_ on: Bool, testCopy: Bool = TestCopy.isOn) -> Check {
        var check = Check(id: "login", status: .ok, title: "Smolcorp opens at login", detail: "It's in the menu bar from the moment you log in.")
        if testCopy {
            check.status = .info
            check.detail = "A test copy leaves the login item to the Smolcorp you use."
        } else if !on {
            check.status = .todo
            check.fix = login
            check.label = "Open at login"
            check.detail = "Otherwise, sessions started before you open Smolcorp aren't at a desk until their next turn."
        }
        return check
    }

    enum FixError: LocalizedError, Equatable {
        case unknown, testCopy, notMoved(String), disconnected
        var errorDescription: String? {
            switch self {
            case .unknown: return "There's no fix by that name."
            case .disconnected:
                return "Your agents are disconnected, as you left them, so setup didn't connect them. "
                    + "To connect them again, press Connect in Smolcorp's Settings."
            case .testCopy: return "A test copy stays where it was built."
            case .notMoved(let why): return "Smolcorp couldn't be moved to Applications: \(why)"
            }
        }
    }

    /// Whether a fix asked for leaves the agents as they are: `--setup`'s own
    /// repair (`auto`), after you disconnected them. Pressing a fix yourself
    /// always runs it.
    static func leavesDisconnected(fix: String, auto: Bool, disconnected: Bool) -> Bool {
        fix == repair && auto && disconnected
    }

    /// Runs the fix named `name`. On the main queue.
    static func fix(_ name: String) throws {
        switch name {
        case repair:
            // Repair connects what was connected; it never turns voice on, so there's nothing to tell
            var notice: String?
            try Hooks.connect(turningOnVoice: false, notice: &notice)
            if Config.shared.hooksDisconnected {
                Config.shared.hooksDisconnected = false
                Config.shared.save()
            }
        case install: try moveToApplications()
        case accessibility: Permissions.requestAccessibility()
        case login: LoginItem.isEnabled = true
        case tailscale: NSWorkspace.shared.open(URL(string: "https://tailscale.com/download")!)
        // The GM preflight's: where you let Smolcorp open your terminal.
        case GMPreflight.automationFix: NSWorkspace.shared.open(GMPreflight.automationPane)
        default: throw FixError.unknown
        }
    }

    /// The app copied into /Applications, any Smolcorp already there put in
    /// the Trash first, and the copy opened in this one's place.
    static func moveToApplications() throws {
        guard !TestCopy.isOn else { throw FixError.testCopy }
        let here = Bundle.main.bundleURL
        let there = URL(fileURLWithPath: "/Applications").appendingPathComponent(here.lastPathComponent)
        guard here.standardizedFileURL != there.standardizedFileURL else { return }
        do {
            if FileManager.default.fileExists(atPath: there.path) {
                try FileManager.default.trashItem(at: there, resultingItemURL: nil)
            }
            try FileManager.default.copyItem(at: here, to: there)
        } catch {
            throw FixError.notMoved(error.localizedDescription)
        }
        let open = Process()
        open.executableURL = URL(fileURLWithPath: "/usr/bin/open")
        open.arguments = ["-n", there.path]
        // The page's answer goes first, then this copy leaves for the new one.
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
            try? open.run()
            NSApp.terminate(nil)
        }
    }
}
