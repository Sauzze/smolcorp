import XCTest
@testable import AgentDeck

final class CloudBotTests: XCTestCase {

    // MARK: The bots as set

    func testBotsAreCheckedAsTheyAreKept() throws {
        let json = #"""
        [{"id": "Smolcorp-Mini", "name": "Smolcorp mini", "source": {"kind": "grokBotApp"}},
         {"id": "smolcorp-mini", "name": "Twice"},
         {"id": "bad id!", "name": "Nope"},
         {"id": "x", "name": "  "},
         {"id": "other", "name": "a/b", "outfit": "nonsense", "badge": "megaphone", "agent": "who", "permanent": false, "floor": "roof",
          "source": {"kind": "mail", "counts": "all"}}]
        """#
        let bots = CloudBot.checked(try JSONDecoder().decode([CloudBot].self, from: Data(json.utf8)))
        XCTAssertEqual(bots.map(\.id), ["smolcorp-mini", "other"])
        XCTAssertEqual(bots[0].name, "Smolcorp mini")
        XCTAssertEqual(bots[0].agent, "grok")
        XCTAssertEqual(bots[0].outfit, "grok")
        XCTAssertTrue(bots[0].permanent)
        XCTAssertEqual(bots[0].floor, "", "where new bots go unless set")
        XCTAssertEqual(bots[0].source, CloudBot.Source(kind: "grokBotApp", bot: "Smolcorp mini", counts: "unprompted"))
        XCTAssertEqual(bots[0].sessionID, "cloud-smolcorp-mini")
        XCTAssertEqual(bots[0].folder, "/cloud/Smolcorp mini")
        XCTAssertEqual(bots[1].name, "a-b")
        XCTAssertEqual(bots[1].outfit, "")
        XCTAssertEqual(bots[1].badge, "megaphone")
        XCTAssertEqual(bots[1].agent, "grok")
        XCTAssertFalse(bots[1].permanent)
        XCTAssertEqual(bots[1].floor, "", "a floor there isn't is where new bots go")
        XCTAssertEqual(CloudBot(id: "g", name: "G", floor: "ground").checked?.floor, "ground")
        XCTAssertEqual(bots[1].source, CloudBot.Source())
    }

    func testItsRoleDressesItWithoutAName() {
        let role = CloudBot(id: "a", name: "A").role
        XCTAssertEqual(role.outfit, "grok")
        XCTAssertEqual(role.name, "", "the desk says only the bot's own name")
        XCTAssertEqual(CloudBot(id: "a", name: "A").hirer, "Grok Bots")
        XCTAssertNil(CloudBot(id: "b", name: "B", agent: "codex").hirer)
    }

    // MARK: The Grok Bot app

    /// The app's file name for `key`: base32, lower case, no padding.
    private func fileName(_ key: String) -> String {
        let alphabet = Array("abcdefghijklmnopqrstuvwxyz234567")
        var bits = 0, value = 0, out = ""
        for byte in Array(key.utf8) {
            value = (value << 8) | Int(byte)
            bits += 8
            while bits >= 5 {
                bits -= 5
                out.append(alphabet[(value >> bits) & 31])
            }
            value &= (1 << bits) - 1
        }
        if bits > 0 { out.append(alphabet[(value << (5 - bits)) & 31]) }
        return out + ".blob"
    }

    func testAFileIsKnownByItsKeyInBase32() {
        // As the app names its roster.
        XCTAssertEqual(GrokBotApp.key(ofFile: "onqw4zbomnwgszlooqxhg3djmnss4y3mnfsw45bnnvsxiyjomfrwg33vnz2c243mn52a.blob"),
                       "sand.client.slice.client-meta.account-slot")
        XCTAssertEqual(GrokBotApp.key(ofFile: fileName("a.roster.last-roster")), "a.roster.last-roster")
        XCTAssertNil(GrokBotApp.key(ofFile: "sand-secrets.json"))
        XCTAssertNil(GrokBotApp.key(ofFile: "not base32!.blob"))
    }

    /// The roster as the app keeps it: a group chat, between two of its bots, among the bots' own chats.
    /// Made up: no real names or ids.
    private let groupRoster = #"""
    {"schemaVersion":4,"value":{"rows":[
     {"name":"Alpha","id":"A1-11","isGroup":false,"memberIds":[]},
     {"name":"Two Bots","id":"C3-33","isGroup":true,"memberIds":["A1-11","B2-22"]},
     {"name":"Beta","id":"B2-22","isGroup":false,"memberIds":[]},
     {"name":"Older Group","id":"D4-44","memberIds":["A1-11","B2-22"]},
     {"name":"Gamma","id":"E5-55"},
     {"name":"Flag Only","id":"F6-66","isGroup":true,"memberIds":[]}]}}
    """#

    func testAGroupChatInTheRosterIsToldFromItsBots() throws {
        let rows = try XCTUnwrap(GrokBotApp.roster(in: Data(groupRoster.utf8)))
        XCTAssertEqual(rows.map(\.id), ["A1-11", "C3-33", "B2-22", "D4-44", "E5-55", "F6-66"])
        XCTAssertEqual(rows.filter(\.isGroup).map(\.id), ["C3-33", "D4-44", "F6-66"], "it says so, or has more than one bot in it")
    }

    func testTheAppsGroupChatNeverGetsADesk() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        try Data(groupRoster.utf8).write(to: folder.appendingPathComponent(fileName("s.roster.last-roster")))
        let found = CloudBot.discover(GrokBotApp.rows(folder: folder), in: [], hidden: [])
        XCTAssertEqual(found.bots.map(\.source.botId), ["A1-11", "B2-22", "E5-55"])
        XCTAssertEqual(found.noRoom, [])
    }

    func testTurnsAreToldFromIDs() {
        XCTAssertEqual(GrokBotApp.turn(of: "t9u"), "t9")
        XCTAssertEqual(GrokBotApp.turn(of: "t12s3"), "t12")
        XCTAssertNil(GrokBotApp.turn(of: "event-abc"))
        XCTAssertNil(GrokBotApp.turn(of: "tu"))
        XCTAssertNil(GrokBotApp.turn(of: "t3s"))
        XCTAssertNil(GrokBotApp.turn(of: "t3x1"))
    }

    private func transcript(_ entries: [(id: String, kind: String, role: String?)]) -> Data {
        let list = entries.enumerated().map { i, e -> [String: Any] in
            var entry: [String: Any] = ["id": e.id, "kind": e.kind, "seq": i + 1, "content": "never read"]
            if let role = e.role { entry["role"] = role }
            return entry
        }
        return try! JSONSerialization.data(withJSONObject: ["schemaVersion": 1, "value": ["entries": list]])
    }

    func testOnlyTheBotsUnpromptedTurnsAreReports() throws {
        let data = transcript([("t1u", "message", "user"), ("t1s0", "send-message", nil), ("event-1", "event", nil),
                               ("t2s0", "send-message", nil), ("t2s1", "send-message", nil), ("r7", "send-message", nil)])
        let entries = try XCTUnwrap(GrokBotApp.entries(in: data))
        XCTAssertEqual(GrokBotApp.botMessages(in: entries).map(\.id), ["t1s0", "t2s0", "t2s1", "r7"])
        // A reply to you is none; the routine's turn is one, however many messages; an id of another shape counts.
        XCTAssertEqual(GrokBotApp.reports(in: entries, new: ["t1s0"], countsAll: false), 0)
        XCTAssertEqual(GrokBotApp.reports(in: entries, new: ["t2s0", "t2s1"], countsAll: false), 1)
        XCTAssertEqual(GrokBotApp.reports(in: entries, new: ["r7"], countsAll: false), 1)
        XCTAssertEqual(GrokBotApp.reports(in: entries, new: ["t1s0", "t2s1"], countsAll: true), 2)
        XCTAssertEqual(GrokBotApp.reports(in: entries, new: [], countsAll: true), 0)
    }

    func testTheSourceLearnsFirstThenSeesNewReports() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let roster = #"{"value":{"rows":[{"name":"Smolcorp mini","id":"ABC-1"}]}}"#
        try Data(roster.utf8).write(to: folder.appendingPathComponent(fileName("sand.client.slice.account.x.roster.last-roster")))
        let file = folder.appendingPathComponent(fileName("sand.client.slice.account.x.transcript.replicas.ABC-1"))
        // Another bot's transcript, never looked at.
        try transcript([("t1s0", "send-message", nil)]).write(to: folder.appendingPathComponent(fileName("sand.client.slice.account.x.transcript.replicas.OTHER")))

        let source = GrokBotApp(bot: "Smolcorp mini", counts: "unprompted", folder: folder)
        XCTAssertEqual(source.newReports(), 0, "no transcript yet")
        try transcript([("t1u", "message", "user"), ("t1s0", "send-message", nil)]).write(to: file)
        XCTAssertEqual(source.newReports(), 0, "the first look only learns")
        try transcript([("t1u", "message", "user"), ("t1s0", "send-message", nil), ("t2u", "message", "user"), ("t2s0", "send-message", nil)])
            .write(to: file)
        try FileManager.default.setAttributes([.modificationDate: Date().addingTimeInterval(10)], ofItemAtPath: file.path)
        XCTAssertEqual(source.newReports(), 0, "a reply to you")
        try transcript([("t1u", "message", "user"), ("t1s0", "send-message", nil), ("t2u", "message", "user"), ("t2s0", "send-message", nil),
                        ("t3s0", "send-message", nil)]).write(to: file)
        try FileManager.default.setAttributes([.modificationDate: Date().addingTimeInterval(20)], ofItemAtPath: file.path)
        XCTAssertEqual(source.newReports(), 1, "the routine's run")
        XCTAssertEqual(source.newReports(), 0, "unchanged, not read again")
    }

    // MARK: Renames

    private func roster(_ rows: [(name: String, id: String)]) -> Data {
        try! JSONSerialization.data(withJSONObject: ["value": ["rows": rows.map { ["name": $0.name, "id": $0.id, "description": "never read"] }]])
    }

    func testABotFoundByItsIDIsStillFollowedAfterARenameAndSaysItsNewName() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let rosterFile = folder.appendingPathComponent(fileName("sand.client.slice.account.x.roster.last-roster"))
        let file = folder.appendingPathComponent(fileName("sand.client.slice.account.x.transcript.replicas.F159-AB"))
        try roster([("Smolcorp Mini", "F159-AB"), ("Xreplier", "6322-CD")]).write(to: rosterFile)
        try transcript([("t1s0", "send-message", nil)]).write(to: file)

        // Set with its old name, and its id in another case.
        let source = GrokBotApp(bot: "Smolcorp Mini", botId: "f159-ab", counts: "unprompted", folder: folder)
        XCTAssertEqual(source.newReports(), 0, "the first look only learns")
        XCTAssertEqual(source.liveName, "Smolcorp Mini")

        try roster([("Smolcorp Mini Manager", "F159-AB"), ("Xreplier", "6322-CD")]).write(to: rosterFile)
        try transcript([("t1s0", "send-message", nil), ("t2s0", "send-message", nil)]).write(to: file)
        try FileManager.default.setAttributes([.modificationDate: Date().addingTimeInterval(10)], ofItemAtPath: file.path)
        XCTAssertEqual(source.newReports(), 1, "renamed, its routine's run is still seen")
        XCTAssertEqual(source.liveName, "Smolcorp Mini Manager", "and its new name with it")

        // Another bot taking its old name changes nothing: it's known by its id.
        try roster([("Smolcorp Mini Manager", "F159-AB"), ("Smolcorp Mini", "6322-CD")]).write(to: rosterFile)
        XCTAssertEqual(source.newReports(), 0)
        XCTAssertEqual(source.liveName, "Smolcorp Mini Manager")

        // By name alone, the old way, it says no live name: its desk keeps the one it was set with.
        let byName = GrokBotApp(bot: "Smolcorp Mini Manager", counts: "unprompted", folder: folder)
        _ = byName.newReports()
        XCTAssertNil(byName.liveName)
    }

    func testANameAloneStillFindsItThenTheOneNameLikeIt() {
        let rows = [GrokBotApp.Row(id: "A1", name: "Smolcorp Mini Manager"), GrokBotApp.Row(id: "B2", name: "Xreplier"),
                    GrokBotApp.Row(id: "C3", name: "Lead Finder")]
        XCTAssertEqual(GrokBotApp.resolve(botId: "", name: "xreplier", in: rows), .init(botId: "B2", name: "Xreplier", by: "name"))
        XCTAssertEqual(GrokBotApp.resolve(botId: "", name: "Smolcorp Mini", in: rows),
                       .init(botId: "A1", name: "Smolcorp Mini Manager", by: "guess"), "renamed: the one name holding it")
        XCTAssertEqual(GrokBotApp.resolve(botId: "", name: "Lead Finder Pro", in: rows)?.botId, "C3", "or held in it")
        let two = rows + [GrokBotApp.Row(id: "D4", name: "Smolcorp Mini Writer")]
        XCTAssertNil(GrokBotApp.resolve(botId: "", name: "Smolcorp Mini", in: two), "two like it: no guess")
        XCTAssertEqual(GrokBotApp.resolve(botId: "", name: "Smolcorp Mini Manager", in: two)?.by, "name", "a name exactly beats a guess")
        XCTAssertEqual(GrokBotApp.resolve(botId: "b2", name: "Lead Finder", in: rows), .init(botId: "B2", name: "Xreplier", by: "id"),
                       "an id beats its name")
    }

    func testABotNotInTheAppIsNoneAndItsSourceSeesNothing() throws {
        let rows = [GrokBotApp.Row(id: "A1", name: "Xreplier")]
        XCTAssertNil(GrokBotApp.resolve(botId: "", name: "Fantasy Coach", in: rows))
        XCTAssertNil(GrokBotApp.resolve(botId: "", name: "  ", in: rows))
        XCTAssertNil(GrokBotApp.resolve(botId: "FFFF", name: "Xreplier", in: rows), "gone by its id: not another by its name")

        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        try roster([("Xreplier", "A1")]).write(to: folder.appendingPathComponent(fileName("s.roster.last-roster")))
        try transcript([("t1s0", "send-message", nil)]).write(to: folder.appendingPathComponent(fileName("s.transcript.replicas.A1")))
        let source = GrokBotApp(bot: "Fantasy Coach", counts: "any", folder: folder)
        XCTAssertEqual(source.newReports(), 0)
        XCTAssertEqual(source.newReports(), 0)
        XCTAssertNil(source.liveName)
        XCTAssertNil(GrokBotApp.resolve(CloudBot.Source(kind: "grokBotApp", bot: "Fantasy Coach"), folder: folder))
        XCTAssertEqual(GrokBotApp.resolve(CloudBot.Source(kind: "grokBotApp", bot: "Xreplier"), folder: folder)?.botId, "A1")
        XCTAssertNil(GrokBotApp.resolve(CloudBot.Source(kind: "none", bot: "Xreplier"), folder: folder), "not from the app")
    }

    func testABotsIDInTheAppIsKeptOnlyWhenSafe() throws {
        let json = #"""
        [{"id": "a", "name": "A", "source": {"kind": "grokBotApp", "botId": "f1593db1-c43e-49a8"}},
         {"id": "b", "name": "B", "source": {"kind": "grokBotApp", "botId": "../secrets"}}]
        """#
        let bots = CloudBot.checked(try JSONDecoder().decode([CloudBot].self, from: Data(json.utf8)))
        XCTAssertEqual(bots[0].source, CloudBot.Source(kind: "grokBotApp", bot: "A", botId: "f1593db1-c43e-49a8"))
        XCTAssertEqual(bots[1].source.botId, "")
        let saved = try JSONSerialization.jsonObject(with: JSONEncoder().encode(bots[0])) as! [String: Any]
        XCTAssertEqual((saved["source"] as? [String: Any])?["botId"] as? String, "f1593db1-c43e-49a8")
    }
}
