import XCTest
@testable import AgentDeck

/// Your settings survive a file this version can't fully read.
final class ConfigTests: XCTestCase {
    private var file: URL!

    override func setUpWithError() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        file = folder.appendingPathComponent("config.json")
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: file.deletingLastPathComponent())
    }

    func testAFieldOfTheWrongTypeLosesOnlyItself() throws {
        try ##"{"pins": ["/work/a"], "scrollEnabled": "yes", "hireFolder": "/work", "colours": {"key": "#ff0000"}}"##
            .write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        XCTAssertEqual(config.pins.first, "/work/a")
        XCTAssertEqual(config.hireFolder, "/work")
        XCTAssertEqual(config.colours, ["key": "#ff0000"])
        XCTAssertFalse(config.scrollEnabled)
    }

    /// The calm office is a choice of the room's, so it's kept with the layout, in the config and in its profiles.
    func testACalmOfficeIsOffUntilChosenAndKeptInAProfile() throws {
        XCTAssertEqual(Room().calm, "off")
        let room = { (json: String) in try JSONDecoder().decode(Room.self, from: Data(json.utf8)) }
        XCTAssertEqual(try room(#"{"calm": "on"}"#).calm, "on")
        XCTAssertEqual(try room(#"{"calm": "very"}"#).calm, "off")
        XCTAssertEqual(try room(#"{"floor": "tiles"}"#).calm, "off")
        let profile = try Config.checkedProfile(Data(#"{"name": "Quiet", "layout": {"room": {"calm": "on"}}}"#.utf8))
        XCTAssertEqual(profile.layout.room.calm, "on")
        let encoded = try JSONSerialization.jsonObject(with: JSONEncoder().encode(profile.layout.room)) as? [String: Any]
        XCTAssertEqual(encoded?["calm"] as? String, "on")
    }

    /// The sound plays on behind other windows unless it's to be heard only in front, a choice kept with the room.
    func testTheSoundPlaysBehindOtherWindowsUntilKeptToTheFront() throws {
        XCTAssertEqual(Room().soundWhen, "always")
        let room = { (json: String) in try JSONDecoder().decode(Room.self, from: Data(json.utf8)) }
        XCTAssertEqual(try room(#"{"soundWhen": "front"}"#).soundWhen, "front")
        XCTAssertEqual(try room(#"{"soundWhen": "sometimes"}"#).soundWhen, "always")
        XCTAssertEqual(try room(#"{"sound": "quiet"}"#).soundWhen, "always")
        let profile = try Config.checkedProfile(Data(#"{"name": "Front", "layout": {"room": {"soundWhen": "front"}}}"#.utf8))
        XCTAssertEqual(profile.layout.room.soundWhen, "front")
        let encoded = try JSONSerialization.jsonObject(with: JSONEncoder().encode(profile.layout.room)) as? [String: Any]
        XCTAssertEqual(encoded?["soundWhen"] as? String, "front")
        XCTAssertEqual(try room(String(decoding: JSONEncoder().encode(profile.layout.room), as: UTF8.self)).soundWhen, "front")
    }

    /// The door and the elevator are moved along the back wall, and upright, pieces hang on the lounge's wall past its end: kept as saved.
    func testTheDoorTheElevatorAndTheLoungesWallAreKeptAsArranged() throws {
        let json = #"{"plan": {"wide": {"door": {"x": -40, "y": 12, "size": 2, "off": true, "thing": "duck"}, "elevator": {"x": -90.4}},"#
            + #" "tall": {"door": {"x": -12}}, "hung": {"tall": [{"kind": "poster", "x": 300}, {"kind": "cuckoo", "x": 999}]}}}"#
        let room = try JSONDecoder().decode(Room.self, from: Data(json.utf8))
        let door = try XCTUnwrap(room.plan.wide["door"]), elevator = try XCTUnwrap(room.plan.wide["elevator"])
        XCTAssertEqual([door.x, elevator.x, room.plan.tall["door"]?.x], [-40, -90, -12])
        XCTAssertEqual(door.size, 1)
        XCTAssertFalse(door.off)
        XCTAssertEqual(door.thing, "auto")
        XCTAssertEqual(room.plan.hung.tall, [Plan.HungPiece(kind: "poster", x: 300), Plan.HungPiece(kind: "cuckoo", x: Plan.wallLength)])
        let again = try JSONDecoder().decode(Room.self, from: JSONEncoder().encode(room))
        XCTAssertEqual(again.plan, room.plan)
    }

    /// A desk can give whoever sits at it an outfit and its colour; anything else can't, and an outfit the office doesn't have is Auto.
    func testADeskKeepsTheOutfitItGives() throws {
        let json = #"{"plan": {"wide": {"desk2": {"outfit": "pirate", "colour": "teal"}, "desk3": {"outfit": "ballgown", "colour": "plaid"}, "pong": {"outfit": "pirate"}}}}"#
        let room = try JSONDecoder().decode(Room.self, from: Data(json.utf8))
        XCTAssertEqual(room.plan.wide["desk2"]?.outfit, "pirate")
        XCTAssertEqual(room.plan.wide["desk2"]?.colour, "teal")
        XCTAssertEqual(room.plan.wide["desk3"]?.outfit, "auto")
        XCTAssertEqual(room.plan.wide["desk3"]?.colour, "auto")
        XCTAssertEqual(room.plan.wide["pong"]?.outfit, "auto")
        XCTAssertEqual(try JSONDecoder().decode(Room.self, from: JSONEncoder().encode(room)).plan, room.plan)
        // A layout saved before desks had outfits loads as it was, each desk on Auto.
        let old = try JSONDecoder().decode(Room.self, from: Data(#"{"plan": {"wide": {"desk1": {"thing": "duck"}}}}"#.utf8))
        XCTAssertEqual(old.plan.wide["desk1"]?.outfit, "auto")
    }

    func testAConfigThatCantBeReadIsKeptAside() throws {
        let broken = #"{"pins": ["/work/a"], "roles": ["#
        try broken.write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        XCTAssertNil(config.pins.first ?? nil)
        XCTAssertEqual(try String(contentsOf: file.appendingPathExtension("broken"), encoding: .utf8), broken)
    }

    func testAFieldLeftOutIsKeptAsideToo() throws {
        for text in [#"{"pins": ["/work/a", 3], "hireFolder": "/work"}"#, #"{"roles": {"not": "a list"}, "hireFolder": "/work"}"#] {
            try text.write(to: file, atomically: true, encoding: .utf8)
            let config = Config.load(from: file)
            XCTAssertEqual(config.hireFolder, "/work", "for \(text)")
            XCTAssertEqual(try String(contentsOf: file.appendingPathExtension("broken"), encoding: .utf8), text)
            try FileManager.default.removeItem(at: file.appendingPathExtension("broken"))
        }
    }

    /// ✨ Summary's opt-in is saved and read back, and every other setting with it, as it was.
    func testTheSummaryOptInRoundTripsAndLeavesTheRestAsItWas() throws {
        let given: [String: Any] = [
            "pins": (["/work/a", NSNull(), "/work/c"] + Array(repeating: NSNull(), count: KeyCount.most - 3)) as [Any], "colours": ["key": "#ff0000"], "activeProfile": "Default", "logo": "smolcorp",
            "scrollEnabled": true, "hiringEnabled": true, "hireFolder": "/work", "hireAgent": "codex", "hireTerminal": "terminal",
            "hireInTmux": true, "dictationEnabled": true, "dictationLanguage": "en_US", "dictationForClaude": true,
            "dictationCleanup": false, "dictationGlossary": ["Smolcorp", "Acmebill"], "musicEnabled": true,
            "summaryOptIn": true, "hasCompletedSetup": true, "maxConfirm": false,
            "phoneMicEnabled": false, "phoneMicSkipsConfirm": true,
            // A key this version doesn't know is read past, not a reason to keep the file aside.
            "aSettingFromANewerVersion": ["anything": 1],
        ]
        try JSONSerialization.data(withJSONObject: given).write(to: file)
        let config = Config.load(from: file)
        XCTAssertTrue(config.summaryOptIn)
        XCTAssertFalse(FileManager.default.fileExists(atPath: file.appendingPathExtension("broken").path))

        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(config)) as? [String: Any])
        for (key, value) in given where key != "aSettingFromANewerVersion" {
            let savedValue = try XCTUnwrap(saved[key], key)
            XCTAssertEqual(try json(savedValue), try json(value), key)
        }
        // And read back once more, it's the same again.
        let again = try JSONDecoder().decode(Config.self, from: JSONEncoder().encode(config))
        XCTAssertTrue(again.summaryOptIn)
        XCTAssertEqual(try json(JSONSerialization.jsonObject(with: JSONEncoder().encode(again))), try json(saved))
    }

    /// Setup has no logo choice or desk count any more: an older pick of either is read but not kept.
    func testAnOlderLogoOrDeskCountLoadsAsTheRegularOneAndAuto() throws {
        try #"{"logo": "keys", "layout": {"keys": 4}}"#.write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        XCTAssertEqual(config.logo, .smolcorp)
        XCTAssertEqual(config.layout.keys, .auto)
        XCTAssertFalse(FileManager.default.fileExists(atPath: file.appendingPathExtension("broken").path))
    }

    /// A value as canonical JSON, to compare two whatever their types.
    private func json(_ value: Any) throws -> String {
        String(data: try JSONSerialization.data(withJSONObject: value, options: [.sortedKeys, .fragmentsAllowed]), encoding: .utf8)!
    }

    /// The office asks before max effort until you say not to, and an older file, which never said, asks too.
    func testMaxEffortIsAskedAboutUntilTurnedOff() throws {
        XCTAssertTrue(Config().maxConfirm)
        try #"{"hireFolder": "/work"}"#.write(to: file, atomically: true, encoding: .utf8)
        XCTAssertTrue(Config.load(from: file).maxConfirm)
        try #"{"maxConfirm": false}"#.write(to: file, atomically: true, encoding: .utf8)
        let off = Config.load(from: file)
        XCTAssertFalse(off.maxConfirm)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(off)) as? [String: Any])
        XCTAssertEqual(saved["maxConfirm"] as? Bool, false)
        try #"{"maxConfirm": "no"}"#.write(to: file, atomically: true, encoding: .utf8)
        XCTAssertTrue(Config.load(from: file).maxConfirm, "a value it can't read asks, as the default does")
    }

    func testAFileReadInFullIsntCopied() throws {
        try #"{"pins": ["/work/a"], "hireFolder": "/work"}"#.write(to: file, atomically: true, encoding: .utf8)
        _ = Config.load(from: file)
        XCTAssertFalse(FileManager.default.fileExists(atPath: file.appendingPathExtension("broken").path))
    }

    /// A setting another version wrote, older, newer or a build of your own, is saved back as it was.
    func testWhatThisVersionDoesntReadIsSavedBack() throws {
        try #"{"hireFolder": "/work", "someLaterSetting": true, "someLaterList": ["a", "b"], "someLaterObject": {"x": 1}}"#
            .write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        config.hireFolder = "/elsewhere"
        config.save(to: file)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: file)) as? [String: Any])
        XCTAssertEqual(saved["hireFolder"] as? String, "/elsewhere", "this version's own settings win")
        XCTAssertEqual(saved["someLaterSetting"] as? Bool, true)
        XCTAssertEqual(saved["someLaterList"] as? [String], ["a", "b"])
        XCTAssertEqual((saved["someLaterObject"] as? [String: Any])?["x"] as? Int, 1)
        XCTAssertEqual(Config.load(from: file).hireFolder, "/elsewhere")
    }

    func testTheFloorsAre1FTo6FInTheOfficesOwnStyleUntilChanged() throws {
        try #"{"hireFolder": "/work"}"#.write(to: file, atomically: true, encoding: .utf8)
        let floors = Config.load(from: file).floors
        XCTAssertEqual(floors.map(\.name), ["1F", "2F", "3F", "4F", "5F", "6F"])
        XCTAssertEqual(floors.map(\.preset), ["", "", "", "", "", ""])
    }

    /// Six floors, each name one line of 24 at most, each style one there is; the ground floor's always the office's own.
    func testTheFloorsAreCheckedAsTheyLoadAndKeptAsSaved() throws {
        let long = String(repeating: "x", count: 30)
        try ##"""
            {"floors": [{"name": "Lobby", "preset": "space"}, {"name": "  Ops\n\u0007 ", "preset": "server"},
                        {"name": "   ", "preset": "nowhere"}, {"name": "\##(long)", "preset": "yours"}, 42,
                        {"name": "Roof", "preset": "pirates"}, {"name": "Seventh", "preset": "space"}],
             "somethingElse": 1}
            """##.write(to: file, atomically: true, encoding: .utf8)
        let config = Config.load(from: file)
        XCTAssertEqual(config.floors, [
            Floor(name: "Lobby", preset: ""), Floor(name: "Ops", preset: "server"), Floor(name: "3F", preset: ""),
            Floor(name: String(repeating: "x", count: 24), preset: ""), Floor(name: "5F", preset: ""), Floor(name: "Roof", preset: "pirates"),
        ])
        config.save(to: file)
        XCTAssertEqual(Config.load(from: file).floors, config.floors)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: file)) as? [String: Any])
        XCTAssertEqual(saved["somethingElse"] as? Int, 1)
        // A short list gets the rest of the floors as they start.
        XCTAssertEqual(Floor.checked([Floor(name: "Hall", preset: "space")]).map(\.name), ["Hall", "2F", "3F", "4F", "5F", "6F"])
    }

    /// What setup sends: the floors under "floors", or the list alone; anything else isn't floors.
    func testSetupSendsTheFloorsAsAListOrUnderTheirName() {
        let given = Floor.sent(Data(#"{"floors": [{"name": "", "preset": ""}, {"name": "Lab", "preset": "space"}]}"#.utf8))
        XCTAssertEqual(given?[1], Floor(name: "Lab", preset: "space"))
        XCTAssertEqual(given?.count, 6)
        XCTAssertEqual(Floor.sent(Data(#"[{"name": "Lab"}]"#.utf8))?.first?.name, "Lab")
        XCTAssertNil(Floor.sent(Data(#"{"floors": "Lab"}"#.utf8)))
        XCTAssertNil(Floor.sent(Data("nope".utf8)))
    }

    /// It holds the webhook's secret, so only you can read it, and its copy kept aside too.
    func testItIsSavedReadableByYouAlone() throws {
        Config().save(to: file)
        try "{".write(to: file, atomically: true, encoding: .utf8)
        _ = Config.load(from: file)
        Config().save(to: file)
        for saved in [file!, file.appendingPathExtension("broken")] {
            let permissions = try FileManager.default.attributesOfItem(atPath: saved.path)[.posixPermissions] as? Int
            XCTAssertEqual(permissions, 0o600, saved.lastPathComponent)
        }
    }
}

/// Personal build: dictation's clean-up settings.
final class PersonalVoiceConfigTests: XCTestCase {
    private func load(_ json: String) throws -> Config {
        let file = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString + ".json")
        defer { try? FileManager.default.removeItem(at: file) }
        try json.write(to: file, atomically: true, encoding: .utf8)
        return Config.load(from: file)
    }

    /// Off until you turn it on: it sends the dictated text to Anthropic, with your own key.
    func testCleanUpIsOffByDefaultWithTheGlossary() throws {
        let config = Config()
        XCTAssertFalse(config.dictationCleanup)
        XCTAssertFalse(config.dictationForClaude, "Claude Code too keeps today's default")
        XCTAssertEqual(config.dictationGlossary, DictationCleanup.defaultGlossary)
        let older = try load(#"{"hireFolder": "/work"}"#)
        XCTAssertFalse(older.dictationCleanup)
        XCTAssertEqual(older.dictationGlossary, DictationCleanup.defaultGlossary)
    }

    func testClaudeCodeTooIsLeftAsTheUserHadIt() throws {
        // Pass 2: no forced default; whatever the config says is kept, and saved back the same.
        for value in [false, true] {
            let config = try load(#"{"dictationForClaude": \#(value)}"#)
            XCTAssertEqual(config.dictationForClaude, value)
            XCTAssertFalse(config.dictationCleanup, "the clean-up starts off")
            let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(config)) as? [String: Any])
            XCTAssertEqual(saved["dictationForClaude"] as? Bool, value)
        }
        XCTAssertFalse(try load(#"{"hireFolder": "/work"}"#).dictationForClaude)
        // A config the first pass's build wrote: its old flag is ignored and not written again.
        let first = try load(#"{"dictationForClaude": false, "personalVoice": true}"#)
        XCTAssertFalse(first.dictationForClaude)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(first)) as? [String: Any])
        XCTAssertNil(saved["personalVoice"])
    }

    func testItsSettingsAreKept() throws {
        let config = try load(#"{"dictationCleanup": false, "dictationGlossary": ["Orbit", " orbit ", "", "Resend"]}"#)
        XCTAssertFalse(config.dictationCleanup)
        XCTAssertEqual(config.dictationGlossary, ["Orbit", "Resend"])
        let data = try JSONEncoder().encode(config)
        let saved = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(saved["dictationCleanup"] as? Bool, false)
    }
}
