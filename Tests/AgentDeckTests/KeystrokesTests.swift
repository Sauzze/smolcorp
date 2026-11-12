import XCTest
@testable import AgentDeck

/// Keys pressed in other apps: off by default, and every key checked first. Nothing here sends a real
/// key: the Mac is stood in for by `Fake`.
final class KeystrokesTests: XCTestCase {
    private let app = "com.example.notes"

    /// A Mac that can be changed between keys, and that records what would have been sent.
    private final class Fake {
        var allowed = true, apps = ["com.example.notes"], accessibility = true, dialog = false
        /// A clock, and when YOU last touched the Mac. Our own keys don't move it: they never enter that stream.
        var time: TimeInterval = 1000, lastEvent: TimeInterval = 940
        var postResult = Keystrokes.PostResult.sent
        func userTouches() { lastEvent = time }
        var front: String? = "com.example.notes"
        var sent: [Keystrokes.Stroke] = []
        var logged: [(String, Keystrokes.Action, String)] = []
        /// Runs 4 ms after a key (the 12 ms pause), with the count of keys sent, to change the Mac between keys.
        var afterKey: ((Int) -> Void)?

        var keystrokes: Keystrokes {
            Keystrokes(env: .init(
                allowed: { self.allowed }, allowedApps: { self.apps }, accessibility: { self.accessibility }, frontmost: { self.front },
                idleSeconds: { self.time - self.lastEvent }, now: { self.time }, dialogShowing: { _ in self.dialog },
                post: { stroke, _ in
                    if self.postResult != .sent { return self.postResult }
                    self.sent.append(stroke); return .sent },
                pause: { self.time += 0.004; self.afterKey?(self.sent.count); self.time += 0.008 }, log: { self.logged.append(($0, $1, $2)) }))
        }
    }

    func testItIsOffByDefault() {
        XCTAssertFalse(Config().keystrokesAllowed)
        let fake = Fake()
        fake.allowed = false
        let outcome = fake.keystrokes.type("hello", into: app)
        XCTAssertEqual(outcome, .init(sent: 0, stoppedBy: .off))
        XCTAssertTrue(fake.sent.isEmpty)
    }

    func testTheSettingIsSavedAndReadBack() throws {
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("keystrokes-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: file) }
        XCTAssertFalse(Config.load(from: file).keystrokesAllowed)
        var config = Config()
        config.keystrokesAllowed = true
        config.save(to: file)
        XCTAssertTrue(Config.load(from: file).keystrokesAllowed)
        try #"{"keystrokesAllowed": "yes"}"#.write(to: file, atomically: true, encoding: .utf8)
        XCTAssertFalse(Config.load(from: file).keystrokesAllowed, "a value it can't read stays off")
    }

    func testItRefusesWithoutAccessibility() {
        let fake = Fake()
        fake.accessibility = false
        XCTAssertEqual(fake.keystrokes.press(.tab, into: app).stoppedBy, .noAccessibility)
        XCTAssertTrue(fake.sent.isEmpty)
    }

    func testItTypesEveryCharacterWhenAllIsWell() {
        let fake = Fake()
        let outcome = fake.keystrokes.type("héy 👋", into: app)
        XCTAssertEqual(outcome, .init(sent: 5, stoppedBy: nil))
        XCTAssertEqual(fake.sent, "héy 👋".map { .character($0) })
    }

    func testItRefusesWhenAnotherAppIsInFront() {
        let fake = Fake()
        fake.front = "com.apple.Terminal"
        XCTAssertEqual(fake.keystrokes.type("rm", into: app).stoppedBy, .wrongApp)
        fake.front = nil
        XCTAssertEqual(fake.keystrokes.press(.tab, into: app).stoppedBy, .wrongApp, "an app it can't tell isn't the one")
        XCTAssertTrue(fake.sent.isEmpty)
    }

    func testItChecksTheFrontAppBeforeEveryKeyAndStopsAtOnce() {
        let fake = Fake()
        fake.afterKey = { if $0 == 2 { fake.front = "com.apple.Safari" } }
        let outcome = fake.keystrokes.type("abcdef", into: app)
        XCTAssertEqual(outcome, .init(sent: 2, stoppedBy: .wrongApp))
        XCTAssertEqual(fake.sent.count, 2, "no key went after you changed app")
    }

    func testItStopsWhenYouUseTheMac() {
        let fake = Fake()
        // The real cadence: you touch the Mac 4 ms after our third key, inside the 12 ms pause.
        fake.afterKey = { if $0 == 3 { fake.userTouches() } }
        let outcome = fake.keystrokes.type("abcdef", into: app)
        XCTAssertEqual(outcome, .init(sent: 3, stoppedBy: .userActive), "stopped at the very next key")
        fake.afterKey = nil
        fake.sent = []; fake.time += 5; fake.userTouches(); fake.time += 1.9
        XCTAssertEqual(fake.keystrokes.press(.tab, into: app).stoppedBy, .userActive, "idle for 1.9 s isn't enough")
        fake.time += 0.1
        XCTAssertTrue(fake.keystrokes.press(.tab, into: app).finished)
    }

    func testItStopsWhenYouTouchTheMacRightAfterTheFirstKey() {
        let fake = Fake()
        fake.afterKey = { if $0 == 1 { fake.userTouches() } }
        XCTAssertEqual(fake.keystrokes.type("abc", into: app), .init(sent: 1, stoppedBy: .userActive))
    }

    func testOurOwnKeysAloneNeverStopARun() {
        let fake = Fake()
        let outcome = fake.keystrokes.type(String(repeating: "a", count: 300), into: app)
        XCTAssertEqual(outcome, .init(sent: 300, stoppedBy: nil), "our keys are not you, however fast they go")
    }

    func testTheRuleIsTheSameBeforeEveryKey() {
        XCTAssertTrue(UserActivity.isActive(idle: 1.9))
        XCTAssertTrue(UserActivity.isActive(idle: 0.004))
        XCTAssertFalse(UserActivity.isActive(idle: 2))
    }

    // MARK: The list of allowed apps

    func testAnEmptyListRefusesEverything() {
        let fake = Fake()
        fake.apps = []
        XCTAssertTrue(Config().keystrokeApps.isEmpty, "empty by default")
        XCTAssertEqual(fake.keystrokes.type("hi", into: app), .init(sent: 0, stoppedBy: .notAllowedApp))
        XCTAssertEqual(fake.keystrokes.press(.tab, into: app).stoppedBy, .notAllowedApp)
        XCTAssertTrue(fake.sent.isEmpty)
    }

    func testAnAllowedAppPassesAndOthersDoNot() {
        let fake = Fake()
        fake.apps = ["COM.Example.Notes"]
        XCTAssertTrue(fake.keystrokes.type("hi", into: app).finished, "any capitals")
        fake.front = "com.example.other"
        XCTAssertEqual(fake.keystrokes.type("hi", into: "com.example.other").stoppedBy, .notAllowedApp)
    }

    func testAnAppTakenOffTheListIsRefusedAtOnce() {
        let fake = Fake()
        fake.afterKey = { if $0 == 2 { fake.apps = [] } }
        XCTAssertEqual(fake.keystrokes.type("abcdef", into: app), .init(sent: 2, stoppedBy: .notAllowedApp))
    }

    func testTheBlockListIsASecondLockEvenOnTheList() {
        let fake = Fake()
        fake.apps = ["com.apple.Terminal", app]
        fake.front = "com.apple.Terminal"
        XCTAssertEqual(fake.keystrokes.type("y", into: "com.apple.Terminal").stoppedBy, .blockedApp)
    }

    func testABlockedAppCannotBeAdded() {
        for id in ["com.apple.Terminal", "COM.JETBRAINS.pycharm", "com.microsoft.VSCode", "io.github.sauzze.agentdeck"] {
            XCTAssertEqual(Keystrokes.AppList.adding(id, to: []), .failure(.blocked), id)
        }
        XCTAssertFalse(Keystrokes.AppList.Problem.blocked.reason.isEmpty)
        XCTAssertEqual(Keystrokes.AppList.adding("not an id", to: []), .failure(.badID))
        XCTAssertEqual(Keystrokes.AppList.adding("", to: []), .failure(.badID))
        XCTAssertEqual(Keystrokes.AppList.adding(app, to: []), .success([app]))
        XCTAssertEqual(Keystrokes.AppList.adding(app.uppercased(), to: [app]), .failure(.alreadyThere))
        XCTAssertEqual(Keystrokes.AppList.removing("COM.example.NOTES", from: [app, "com.example.b"]), ["com.example.b"])
    }

    func testTheListIsSavedAndCheckedWhenRead() throws {
        let file = FileManager.default.temporaryDirectory.appendingPathComponent("keystroke-apps-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: file) }
        var config = Config()
        config.keystrokeApps = [app]
        config.save(to: file)
        XCTAssertEqual(Config.load(from: file).keystrokeApps, [app])
        try #"{"keystrokeApps": ["com.apple.Terminal", "com.example.notes", "COM.EXAMPLE.NOTES", "bad id", 7]}"#
            .write(to: file, atomically: true, encoding: .utf8)
        XCTAssertEqual(Config.load(from: file).keystrokeApps, ["com.example.notes"], "a hand-edited file can't add a terminal")
    }

    func testAChangeOfTheListIsLoggedWithTheIdAlone() {
        XCTAssertEqual(Keystrokes.appListChange(added: true, id: app), "app allowed: com.example.notes")
        XCTAssertEqual(Keystrokes.appListChange(added: false, id: app), "app removed: com.example.notes")
    }

    /// The Claude desktop app and the apps holding passwords or settings are blocked like a terminal: never typed into,
    /// never added, and (the offer is filtered by the same check) never listed among the apps to allow.
    func testClaudeAndPasswordAndSettingsAppsAreBlockedToo() {
        let fake = Fake()
        for id in ["com.anthropic.claudefordesktop", "com.apple.Passwords", "com.apple.systempreferences", "com.apple.keychainaccess",
                   "com.1password.1password", "com.agilebits.onepassword7", "COM.1Password.1Password"] {
            XCTAssertTrue(Keystrokes.isNeverInto(id), id)
            XCTAssertEqual(Keystrokes.AppList.adding(id, to: []), .failure(.blocked), id)
            fake.front = id
            XCTAssertEqual(fake.keystrokes.type("y", into: id).stoppedBy, .blockedApp, id)
        }
        XCTAssertTrue(fake.sent.isEmpty)
        XCTAssertFalse(Keystrokes.isNeverInto(app))
        // Every running app the offer lists passes the same check.
        for entry in Keystrokes.runningApps(except: []) { XCTAssertFalse(Keystrokes.isNeverInto(entry["id"] ?? ""), entry["id"] ?? "") }
    }

    // MARK: Terminals, and Return

    func testItNeverTypesOrPressesIntoATerminalOrItself() {
        let fake = Fake()
        for id in ["com.apple.Terminal", "com.googlecode.iterm2", "com.mitchellh.ghostty", "com.github.wez.wezterm",
                   "net.kovidgoyal.kitty", "org.alacritty", "dev.warp.Warp-Stable", "co.zeit.hyper", "org.tabby",
                   "io.github.sauzze.agentdeck", "com.microsoft.VSCode", "com.todesktop.230313mzl4w4u92", "dev.zed.Zed", "dev.zed.Zed-Preview",
                   "com.jetbrains.intellij", "com.jetbrains.WebStorm", "com.google.android.studio"] {
            fake.front = id
            XCTAssertEqual(fake.keystrokes.type("y", into: id).stoppedBy, .blockedApp, id)
            XCTAssertEqual(fake.keystrokes.press(.tab, into: id).stoppedBy, .blockedApp, id)
            XCTAssertEqual(fake.keystrokes.press(.return, into: id, allowReturnIn: id).stoppedBy, .blockedApp, "an allow doesn't lift the block: \(id)")
        }
        XCTAssertTrue(fake.sent.isEmpty)
        fake.front = app
        XCTAssertTrue(fake.keystrokes.type("y", into: app).finished)
    }

    func testReturnNeedsAnExplicitAllowForThatApp() {
        let fake = Fake()
        XCTAssertEqual(fake.keystrokes.press(.return, into: app).stoppedBy, .returnNotAllowed)
        XCTAssertEqual(fake.keystrokes.press(.return, [.command], into: app, allowReturnIn: "com.example.other").stoppedBy, .returnNotAllowed)
        XCTAssertTrue(fake.sent.isEmpty)
        XCTAssertTrue(fake.keystrokes.press(.tab, into: app).finished, "other keys need no allow")
        XCTAssertTrue(fake.keystrokes.press(.return, into: app, allowReturnIn: app).finished)
        XCTAssertEqual(fake.logged.first?.2, "stopped: returnNotAllowed, 0 sent")
    }

    func testAFailedPostIsNotReportedAsTheWrongApp() {
        let fake = Fake()
        fake.postResult = .failed
        XCTAssertEqual(fake.keystrokes.type("ab", into: app), .init(sent: 0, stoppedBy: .postFailed))
        fake.postResult = .wrongApp
        XCTAssertEqual(fake.keystrokes.type("ab", into: app), .init(sent: 0, stoppedBy: .wrongApp))
        XCTAssertTrue(fake.logged.first?.2.contains("postFailed") == true)
    }

    func testWhereTheKeysMayLand() {
        for role in ["AXTextField", "AXTextArea", "AXComboBox"] {
            XCTAssertFalse(FileDialog.refuses(focusedRole: role, subrole: nil), role)
        }
        XCTAssertTrue(FileDialog.refuses(focusedRole: "AXTextField", subrole: "AXSecureTextField"), "never a password field")
        for role in ["AXButton", "AXList", "AXWebArea", "AXOutline", "AXStaticText"] {
            XCTAssertTrue(FileDialog.refuses(focusedRole: role, subrole: nil), role)
        }
        XCTAssertTrue(FileDialog.refuses(focusedRole: nil, subrole: nil), "unsure means refuse")
    }

    func testItStopsWhenAFileDialogOpens() {
        let fake = Fake()
        fake.dialog = true
        XCTAssertEqual(fake.keystrokes.type("/Users/me/file.txt", into: app).stoppedBy, .fileDialog)
        fake.dialog = false
        fake.afterKey = { if $0 == 1 { fake.dialog = true } }
        XCTAssertEqual(fake.keystrokes.type("abc", into: app), .init(sent: 1, stoppedBy: .fileDialog))
    }

    func testTextIsOnlyTextAndNotTooLong() {
        let fake = Fake()
        for bad in ["", "one\ntwo", "tab\there", "bell\u{7}"] {
            XCTAssertEqual(fake.keystrokes.type(bad, into: app).stoppedBy, .badText, bad)
        }
        XCTAssertEqual(fake.keystrokes.type(String(repeating: "a", count: Keystrokes.textMost + 1), into: app).stoppedBy, .tooLong)
        XCTAssertTrue(fake.sent.isEmpty)
        XCTAssertTrue(fake.keystrokes.type(String(repeating: "a", count: Keystrokes.textMost), into: app).finished)
    }

    func testAKeyCarriesItsModifiers() {
        let fake = Fake()
        XCTAssertTrue(fake.keystrokes.press(.return, [.command, .shift], into: app, allowReturnIn: app).finished)
        XCTAssertEqual(fake.sent, [.key(.return, [.command, .shift])])
        XCTAssertEqual(Keystrokes.Key.return.code, 36)
        XCTAssertEqual(Keystrokes.Modifiers([.command, .shift]).flags, [.maskCommand, .maskShift])
    }

    // MARK: The log

    func testEveryUseIsLoggedWithoutTheText() {
        let fake = Fake()
        _ = fake.keystrokes.type("my secret password", into: app)
        fake.front = "other"
        _ = fake.keystrokes.press(.tab, into: app)
        XCTAssertEqual(fake.logged.count, 2)
        XCTAssertEqual(fake.logged[0].0, app)
        XCTAssertEqual(fake.logged[0].1, .type)
        XCTAssertEqual(fake.logged[0].2, "done, 18 sent")
        XCTAssertEqual(fake.logged[1].2, "stopped: wrongApp, 0 sent")
        XCTAssertFalse(fake.logged.contains { $0.2.contains("secret") })
    }

    // MARK: Dialogs, and who may change the setting

    func testWhatCountsAsAFileDialog() {
        XCTAssertFalse(FileDialog.refuses(role: "AXWindow", subrole: nil))
        XCTAssertFalse(FileDialog.refuses(role: "AXWindow", subrole: "AXStandardWindow"))
        XCTAssertTrue(FileDialog.refuses(role: "AXSheet", subrole: nil), "a sheet may hold a file browser")
        XCTAssertTrue(FileDialog.refuses(role: "AXWindow", subrole: "AXDialog"))
        XCTAssertTrue(FileDialog.refuses(role: "AXWindow", subrole: "AXSystemDialog"))
        XCTAssertTrue(FileDialog.refuses(role: nil, subrole: nil), "unsure means refuse")
        XCTAssertTrue(FileDialog.isPanelService(owner: "Open and Save Panel Service (Notes)"))
        XCTAssertFalse(FileDialog.isPanelService(owner: "Notes"))
    }
}
