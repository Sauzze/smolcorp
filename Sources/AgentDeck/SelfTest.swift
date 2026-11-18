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

    /// What a test copy says of the login item, which is the Smolcorp in use's: here, and through the bridge.
    static let testCopyLogin = "A test copy leaves the login item to the Smolcorp you use."

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
