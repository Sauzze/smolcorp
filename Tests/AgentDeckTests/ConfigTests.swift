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
