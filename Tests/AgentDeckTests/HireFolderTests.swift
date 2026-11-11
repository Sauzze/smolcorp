import XCTest
@testable import AgentDeck

/// Start here's hires folder (T781): the folder suggested when none is set, every
/// check with its fix, keeping one, and the route's JSON, which the Linux/Windows
/// app's hirefolder_test.go checks for the same keys.
final class HireFolderTests: XCTestCase {
    private var home: URL!
    private var base: String!
    private let files = FileManager.default

    override func setUp() {
        home = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("hire-folder-home-\(UUID().uuidString)")
        try? files.createDirectory(at: home, withIntermediateDirectories: true)
        base = home.standardizedFileURL.resolvingSymlinksInPath().path
    }

    override func tearDown() {
        // A folder left unwritable by a test can't be removed until it's writable again.
        for name in ["Locked", "Developer"] {
            _ = try? files.setAttributes([.posixPermissions: 0o755], ofItemAtPath: home.appendingPathComponent(name).path)
        }
        try? files.removeItem(at: home)
    }

    private func make(_ path: String) {
        try? files.createDirectory(at: home.appendingPathComponent(path), withIntermediateDirectories: true)
    }

    func testTheSuggestionIsTheFirstLikelyFolderThere() {
        XCTAssertEqual(HireFolder.suggestion(home: home), base + "/Smolcorp", "none there: ~/Smolcorp, made only once accepted")
        XCTAssertFalse(files.fileExists(atPath: home.appendingPathComponent("Smolcorp").path), "suggesting makes nothing")
        make("src")
        XCTAssertEqual(HireFolder.suggestion(home: home), base + "/src")
        make("Code")
        XCTAssertEqual(HireFolder.suggestion(home: home), base + "/Code")
        make("Developer")
        XCTAssertEqual(HireFolder.suggestion(home: home), base + "/Developer")
        make("Projects")
        XCTAssertEqual(HireFolder.suggestion(home: home), base + "/Projects", "~/Projects first")
    }

    func testALikelyFolderThatFailsAChecksIsPassedOver() throws {
        // ~/Projects is a file here, and ~/Developer can't be written in.
        try "".write(to: home.appendingPathComponent("Projects"), atomically: true, encoding: .utf8)
        make("Code")
        XCTAssertEqual(HireFolder.suggestion(home: home), base + "/Code")
        try files.removeItem(at: home.appendingPathComponent("Code"))
        make("Developer")
        try files.setAttributes([.posixPermissions: 0o555], ofItemAtPath: home.appendingPathComponent("Developer").path)
        XCTAssertEqual(HireFolder.suggestion(home: home), base + "/Smolcorp")
    }

    func testEveryCheckSaysWhatsWrongWithItsFix() throws {
        make("Projects/app")
        make("Library/Work")
        make(".config/x")
        make("Projects/smolcorp/.team/scratch")
        make("AppData/Roaming")
        make("Applications/Smolcorp.app/Contents")
        make("Locked")
        try files.setAttributes([.posixPermissions: 0o555], ofItemAtPath: home.appendingPathComponent("Locked").path)
        try "".write(to: home.appendingPathComponent("notes.md"), atomically: true, encoding: .utf8)
        try files.createSymbolicLink(atPath: home.appendingPathComponent("out").path, withDestinationPath: "/usr")
        try files.createSymbolicLink(atPath: home.appendingPathComponent("lib").path, withDestinationPath: home.appendingPathComponent("Library").path)
        let system = "That folder holds your Mac's or your apps' own files. Choose a project folder, such as ~/Projects."
        let disk = "That's a whole disk. Choose a project folder, such as ~/Projects."
        // A hidden (dot) folder says so, not "your Mac's own files" (fresh eyes 3, problem 4).
        let insideHidden = "A folder inside a hidden folder (one whose name starts with '.') can't be used: choose one outside it."
        let hidden = "A hidden folder (one whose name starts with '.') can't be used. Choose a project folder, such as ~/Projects."
        let noDisk = "smolcorp-no-such-disk"
        let cases: [(String, String, HireFolder.Fix, Bool)] = [
            ("", "Type the folder's full path, such as ~/Projects.", .choose, false),
            ("Projects", "Give the folder's full path, such as ~/Projects.", .choose, false),
            ("/", disk, .choose, false),
            (home.path, "That's your whole home folder. Choose a folder inside it, such as ~/Projects, so hires keep to their projects.", .choose, false),
            (home.path + "/", "That's your whole home folder. Choose a folder inside it, such as ~/Projects, so hires keep to their projects.", .choose, false),
            ("/usr", system, .choose, false),
            ("/etc/x", system, .choose, false),
            (home.path + "/out", system, .choose, false),
            // Another disk is fine, but not the whole disk.
            ("/Volumes/\(noDisk)", disk, .choose, false),
            ("/Volumes/\(noDisk)/code", "There's no folder at /Volumes/\(noDisk)/code yet.", .create, false),
            ("/mnt/\(noDisk)/code", "There's no folder at /mnt/\(noDisk)/code yet.", .create, false),
            (home.path + "/Library/Work", system, .choose, false),
            // As the disk has them, whatever the letter case; and through a symlink, where it really is.
            (home.path + "/LIBRARY/New", system, .choose, false),
            (home.path + "/lib/New", system, .choose, false),
            (home.path + "/Applications/Smolcorp.APP/Contents", system, .choose, false),
            (home.path + "/.config/x", insideHidden, .choose, false),
            (home.path + "/.config", hidden, .choose, false),
            (home.path + "/Projects/smolcorp/.team/scratch", insideHidden, .choose, false),
            (home.path + "/Projects/smolcorp/.team", hidden, .choose, false),
            ("~/Projects/.Hidden/New", insideHidden, .choose, false),
            // A system or app folder is still said as one, hidden folder or not.
            (home.path + "/Library/.cache", system, .choose, false),
            (home.path + "/AppData/Roaming", system, .choose, false),
            (home.path + "/Applications/Smolcorp.app/Contents", system, .choose, false),
            (home.path + "/Missing/deeper", "There's no folder at ~/Missing/deeper yet.", .create, false),
            (home.path + "/notes.md", "~/notes.md is a file, not a folder.", .choose, true),
            (home.path + "/Locked", "Smolcorp can't write in ~/Locked. Choose a folder of your own, such as ~/Projects.", .choose, true),
            (home.path + "/Projects/app", "", .none, true),
            (" \(home.path)/Projects/app/ ", "", .none, true),
            (home.path + "/Projects/../Projects/app", "", .none, true),
        ]
        for (given, problem, fix, exists) in cases {
            let check = HireFolder.check(given, home: home)
            XCTAssertEqual(check.problem, problem, "\(given)")
            XCTAssertEqual(check.fix, fix, "\(given)")
            XCTAssertEqual(check.exists, exists, "\(given)")
        }
        XCTAssertEqual(HireFolder.check(home.path + "/Projects/../Projects/app/", home: home).folder, base + "/Projects/app")
    }

    func testATildeIsTheHomeFolder() {
        make("Projects")
        XCTAssertEqual(HireFolder.check("~/Projects", home: home), HireFolder.Check(folder: base + "/Projects", exists: true))
        XCTAssertEqual(HireFolder.check("~", home: home).problem,
                       "That's your whole home folder. Choose a folder inside it, such as ~/Projects, so hires keep to their projects.")
    }

    /// A folder set before these checks is judged only by what hiring needs, so an older config stays as it was.
    func testASavedFolderOutsideHomeHasNoProblem() throws {
        let elsewhere = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("hire-folder-elsewhere-\(UUID().uuidString)")
        try files.createDirectory(at: elsewhere, withIntermediateDirectories: true)
        defer { try? files.removeItem(at: elsewhere) }
        XCTAssertNotEqual(HireFolder.check(elsewhere.path, home: home).fix, .none, "as a new folder, it isn't one to choose")
        for saved in [elsewhere.path, home.path] {
            let state = HireFolder.state(saved: saved, home: home)
            XCTAssertEqual(state["folder"] as? String, saved)
            XCTAssertEqual(state["problem"] as? String, "", saved)
            XCTAssertEqual(state["fix"] as? String, "", saved)
            XCTAssertEqual(state["exists"] as? Bool, true, saved)
        }
        let gone = HireFolder.state(saved: elsewhere.path + "/Gone", home: home)
        XCTAssertEqual(gone["fix"] as? String, "create", "gone since: still said, with Create it")
    }

    func testKeepingMakesTheFolderOnlyWhenAskedAndOnlyWhereItMay() {
        let missing = home.appendingPathComponent("Smolcorp").path
        guard case .failure(let refused) = HireFolder.keep(missing, create: false, home: home) else { return XCTFail("kept a folder that isn't there") }
        XCTAssertEqual(refused.fix, .create)
        XCTAssertFalse(files.fileExists(atPath: missing))
        XCTAssertEqual(try HireFolder.keep(missing, create: true, home: home).get(), base + "/Smolcorp")
        XCTAssertTrue(files.fileExists(atPath: missing), "made once accepted")
        XCTAssertEqual(try HireFolder.keep("~/Smolcorp", create: false, home: home).get(), base + "/Smolcorp", "there now: kept as it is")
        // ~/lib is the Library by a symlink, and ~/LIBRARY is it too on the Mac's disk: refused before anything is made.
        make("Library")
        try? files.createSymbolicLink(atPath: home.appendingPathComponent("lib").path, withDestinationPath: home.appendingPathComponent("Library").path)
        let places = ["/", home.path, home.path + "/Library/New", home.path + "/.new", "/usr/local/new", home.path + "/lib/New",
                      home.path + "/LIBRARY/New", "~/library/Made/deeper"]
        for refusedPlace in places {
            guard case .failure(let check) = HireFolder.keep(refusedPlace, create: true, home: home) else { return XCTFail("kept \(refusedPlace)") }
            XCTAssertEqual(check.fix, .choose, refusedPlace)
        }
        XCTAssertEqual(try? files.contentsOfDirectory(atPath: home.appendingPathComponent("Library").path), [], "never made where it may not be")
        XCTAssertFalse(files.fileExists(atPath: home.appendingPathComponent(".new").path))
    }

    /// The route's JSON: these keys, on a Mac and on Linux/Windows alike (hirefolder_test.go).
    static let keys: Set<String> = ["exists", "fix", "folder", "home", "problem", "suggested"]

    func testTheRouteSaysWhatsSetOrSuggested() throws {
        // Nothing set: the suggestion, not there yet, and nothing wrong with it.
        var state = HireFolder.state(saved: "", home: home)
        XCTAssertEqual(Set(state.keys), Self.keys)
        XCTAssertEqual(state["folder"] as? String, "")
        XCTAssertEqual(state["suggested"] as? String, base + "/Smolcorp")
        XCTAssertEqual(state["exists"] as? Bool, false)
        XCTAssertEqual(state["problem"] as? String, "")
        XCTAssertEqual(state["fix"] as? String, "")
        XCTAssertEqual(state["home"] as? String, base)
        // Set and there.
        make("Projects")
        state = HireFolder.state(saved: base + "/Projects", home: home)
        XCTAssertEqual(state["folder"] as? String, base + "/Projects")
        XCTAssertEqual(state["suggested"] as? String, "")
        XCTAssertEqual(state["exists"] as? Bool, true)
        XCTAssertEqual(state["problem"] as? String, "")
        // Set, but gone since: its problem, and Create it.
        state = HireFolder.state(saved: base + "/Gone", home: home)
        XCTAssertEqual(state["folder"] as? String, base + "/Gone")
        XCTAssertEqual(state["problem"] as? String, "There's no folder at ~/Gone yet.")
        XCTAssertEqual(state["fix"] as? String, "create")
        // One typed, as it's checked.
        state = HireFolder.state(saved: base + "/Projects", asked: "~/Library", home: home)
        XCTAssertEqual(Set(state.keys), Self.keys)
        XCTAssertEqual(state["folder"] as? String, base + "/Library")
        XCTAssertEqual(state["fix"] as? String, "choose")
        XCTAssertEqual(state["suggested"] as? String, "")
        // As JSON, the sorted keys the Linux/Windows app writes too.
        let body = HireFolder.json(HireFolder.Check(), suggested: "", home: home)
        let data = try JSONSerialization.data(withJSONObject: body, options: [.sortedKeys])
        XCTAssertEqual(String(decoding: data, as: UTF8.self),
                       "{\"exists\":false,\"fix\":\"\",\"folder\":\"\",\"home\":\"\(base.replacingOccurrences(of: "/", with: "\\/"))\",\"problem\":\"\",\"suggested\":\"\"}")
    }

    /// The route in process, through setup's gate, as hirefolder_test.go's TestSetupKeepsTheHireFolderFromStartHere:
    /// a refusal answers 400 with the JSON and leaves the folder set as it was; a keep saves it.
    func testSetupKeepsTheHireFolderFromStartHere() throws {
        let config = Config() // never saved: `saved` only counts
        var saves = 0
        make("Library")
        try files.createSymbolicLink(atPath: home.appendingPathComponent("lib").path, withDestinationPath: home.appendingPathComponent("Library").path)
        let key = "test-setup-key", gmToken = "test-gm-token"
        func ask(_ method: String, _ query: [String: String], gm: Bool = true) throws -> (Int, [String: Any]) {
            var headers = [DeckRouter.setupKeyHeader: key]
            if method == "POST" { headers[DeckRouter.setupHeader] = "1" }
            if gm { headers[GMIdentity.header] = gmToken }
            let request = Request(method: method, path: "/setup/hire-folder", query: query, headers: headers, body: Data(), isLocal: true,
                                  isAddressedDirectly: true, remoteAddress: "127.0.0.1")
            let gate = DeckRouter.setupGate(request, sessions: SetupSessions(), isSetupKey: { $0 == key }, isGMToken: { $0 == gmToken })
            let response = gate.refusal ?? HireFolder.route(request, config: config, home: home) { saves += 1 }
            let answer = (try? JSONSerialization.jsonObject(with: response.body)) as? [String: Any] ?? [:]
            return (response.status, answer)
        }
        var (status, answer) = try ask("GET", [:], gm: false)
        XCTAssertEqual(status, 200)
        XCTAssertEqual(Set(answer.keys), Self.keys)
        XCTAssertEqual(answer["suggested"] as? String, base + "/Smolcorp")
        XCTAssertEqual(answer["folder"] as? String, "")
        (status, answer) = try ask("GET", ["check": "~/Library"], gm: false)
        XCTAssertEqual(answer["fix"] as? String, "choose")
        XCTAssertTrue((answer["problem"] as? String ?? "").contains("own files"))
        // A tool with the setup-key file alone can't keep one.
        XCTAssertEqual(try ask("POST", ["folder": "~/Smolcorp", "create": "1"], gm: false).0, 403)
        XCTAssertFalse(files.fileExists(atPath: home.appendingPathComponent("Smolcorp").path))
        // Not there yet: refused with Create it, and nothing kept.
        (status, answer) = try ask("POST", ["folder": "~/Smolcorp"])
        XCTAssertEqual(status, 400)
        XCTAssertEqual(answer["fix"] as? String, "create")
        XCTAssertEqual(Set(answer.keys), Self.keys)
        XCTAssertEqual(config.hireFolder, "")
        XCTAssertEqual(saves, 0)
        (status, answer) = try ask("POST", ["folder": "~/Smolcorp", "create": "1"])
        XCTAssertEqual(status, 200)
        XCTAssertEqual(answer["folder"] as? String, base + "/Smolcorp")
        XCTAssertEqual(answer["problem"] as? String, "")
        XCTAssertEqual(config.hireFolder, base + "/Smolcorp")
        XCTAssertEqual(saves, 1, "saved and refreshed")
        // What can't be a hires folder is refused, and the one set stays.
        for refused in [home.path, "~/lib/New", "/"] {
            (status, answer) = try ask("POST", ["folder": refused, "create": "1"])
            XCTAssertEqual(status, 400, refused)
            XCTAssertEqual(answer["fix"] as? String, "choose", refused)
        }
        XCTAssertEqual(config.hireFolder, base + "/Smolcorp")
        XCTAssertEqual(saves, 1)
        (status, answer) = try ask("GET", [:], gm: false)
        XCTAssertEqual(answer["folder"] as? String, base + "/Smolcorp")
        XCTAssertEqual(answer["suggested"] as? String, "")
        XCTAssertEqual(answer["exists"] as? Bool, true)
    }

    /// The Mac's picker needs the Mac's panel, so this reads the source: Start here's Choose another… only
    /// asks the picker for a folder (pick=1, which keeps nothing) and keeps it through POST /setup/hire-folder.
    func testThePickedFolderIsKeptThroughTheSameChecks() throws {
        let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let router = try String(contentsOf: repo.appendingPathComponent("Sources/AgentDeck/DeckRouter.swift"), encoding: .utf8)
        XCTAssertNotNil(router.range(of: #"deck.chooseHireFolder(keep: false, message: message) { picked in respond(.json(["picked": picked ?? ""])) }"#))
        let page = try String(contentsOf: repo.appendingPathComponent("Resources/web/setup.html"), encoding: .utf8)
        XCTAssertNotNil(page.range(of: "fetch('/setup/hiring/choose?pick=1'"))
        XCTAssertNotNil(page.range(of: "if (answer.picked) return keepFirstFolder(answer.picked, false);"))
        XCTAssertEqual(page.components(separatedBy: "new CustomEvent('smolcorp:hire-folder'").count, 2, "sent once, when a folder is kept")
    }

    func testTheRouteIsTheGMsOrThePagesToChange() {
        XCTAssertTrue(SetupRoutes.allows(.page, "POST", "/setup/hire-folder"))
        XCTAssertTrue(SetupRoutes.allows(.gm, "POST", "/setup/hire-folder"))
        XCTAssertFalse(SetupRoutes.allows(.tool, "POST", "/setup/hire-folder"))
        XCTAssertFalse(SetupRoutes.allows(.hire, "POST", "/setup/hire-folder"))
        XCTAssertTrue(SetupRoutes.allows(.tool, "GET", "/setup/hire-folder"), "reading it is as any setup read")
    }
}
