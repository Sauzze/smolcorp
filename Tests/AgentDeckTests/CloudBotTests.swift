import XCTest
@testable import AgentDeck

final class CloudBotTests: XCTestCase {
    // MARK: The desk

    func testAPermanentBotIsBlueThenGreenForTenMinutesOnAReport() {
        let start = Date(timeIntervalSince1970: 1_000_000)
        var desk = CloudDesk(permanent: true)
        XCTAssertEqual(desk.state(at: start), .thinking)
        XCTAssertNil(desk.nextChange(after: start))
        desk.apply(.report, at: start, permanent: true)
        XCTAssertEqual(desk.state(at: start), .unread)
        XCTAssertEqual(desk.state(at: start.addingTimeInterval(599)), .unread)
        XCTAssertEqual(desk.nextChange(after: start), start.addingTimeInterval(CloudDesk.greenFor))
        XCTAssertEqual(desk.state(at: start.addingTimeInterval(600)), .thinking)
    }

    func testATapEndsTheGreenAndANewReportStartsItAgain() {
        let start = Date(timeIntervalSince1970: 1_000_000)
        var desk = CloudDesk(permanent: true)
        desk.apply(.report, at: start, permanent: true)
        desk.apply(.seen, at: start.addingTimeInterval(5), permanent: true)
        XCTAssertEqual(desk.state(at: start.addingTimeInterval(6)), .thinking)
        let later = start.addingTimeInterval(3600)
        desk.apply(.report, at: later, permanent: true)
        XCTAssertEqual(desk.state(at: later.addingTimeInterval(300)), .unread)
    }

    func testAPermanentBotNeverLeavesAndAVisitorComesAndGoes() {
        let now = Date()
        var permanent = CloudDesk(permanent: true)
        permanent.apply(.stop, at: now, permanent: true)
        XCTAssertEqual(permanent.state(at: now), .thinking)

        var visitor = CloudDesk(permanent: false)
        XCTAssertNil(visitor.state(at: now), "away until it starts")
        visitor.apply(.start, at: now, permanent: false)
        XCTAssertEqual(visitor.state(at: now), .thinking)
        visitor.apply(.stop, at: now, permanent: false)
        XCTAssertNil(visitor.state(at: now))
        visitor.apply(.report, at: now, permanent: false)
        XCTAssertEqual(visitor.state(at: now), .unread, "a report brings it to its desk")
    }

    // MARK: Its last action

    func testAnOldReportBodyStillWorks() throws {
        let now = Date(timeIntervalSince1970: 2_000_000)
        let action = try CloudAction.parse(["type": "report"], now: now).get()
        XCTAssertEqual(action, CloudAction(text: "Sent a report", count: 1, at: now))
    }

    func testAReportSaysWhatHowManyAndWhen() throws {
        let now = Date(timeIntervalSince1970: 1_800_000_000)
        let body = try JSONSerialization.jsonObject(with: Data(#"""
        {"type": "report", "text": "  Drafted <b>3</b> replies\n\u0007 ", "count": 3, "at": "2026-10-28T22:20:00Z"}
        """#.utf8)) as! [String: Any]
        let action = try CloudAction.parse(body, now: now).get()
        XCTAssertEqual(action.text, "Drafted 3 replies", "plain text: no markup, control characters or new lines")
        XCTAssertEqual(CloudAction(text: "3 > 2, <script", at: now).text, "3  2, script")
        XCTAssertEqual(action.count, 3)
        XCTAssertEqual(action.at, CloudAction.parseTime("2026-10-28T22:20:00Z"))
        XCTAssertNotNil(CloudAction.parseTime("2026-10-28T22:20:00.250+01:00"), "fractional seconds and offsets too")

        let long = try CloudAction.parse(["text": String(repeating: "x", count: 500), "count": 5000], now: now).get()
        XCTAssertEqual(long.text.count, CloudAction.textLimit)
        XCTAssertEqual(long.count, CloudAction.most)
        XCTAssertEqual(try CloudAction.parse(["text": "   "], now: now).get().text, "Sent a report")
        XCTAssertEqual(try CloudAction.parse(["at": "2099-01-01T00:00:00Z"], now: now).get().at, now, "a time to come is now")
        XCTAssertEqual(try CloudAction.parse(["text": NSNull(), "count": NSNull(), "at": NSNull()], now: now).get(),
                       CloudAction(at: now))
    }

    func testABadPartOfAReportIsRefused() {
        let now = Date()
        for body: [String: Any] in [["text": 3], ["count": 0], ["count": 2.5], ["count": "3"], ["count": true],
                                    ["at": "yesterday"], ["at": 1_793_000_000]] {
            guard case .failure = CloudAction.parse(body, now: now) else { return XCTFail("took \(body)") }
        }
    }

    func testReportsAddUpTillSeenAndTheCardKeepsTheLast() {
        let start = Date(timeIntervalSince1970: 1_000_000)
        var desk = CloudDesk(permanent: true)
        XCTAssertNil(desk.last)
        desk.apply(.report, at: start, permanent: true)
        XCTAssertEqual(desk.last, CloudAction(at: start), "the old body: one report, now")
        let sent = start.addingTimeInterval(-60)
        desk.apply(.report, at: start.addingTimeInterval(10), permanent: true, action: CloudAction(text: "Drafted replies", count: 2, at: sent))
        XCTAssertEqual(desk.last, CloudAction(text: "Drafted replies", count: 3, at: sent))
        XCTAssertEqual(desk.state(at: start.addingTimeInterval(10)), .unread)
        XCTAssertEqual(desk.nextChange(after: start.addingTimeInterval(10)), start.addingTimeInterval(10 + CloudDesk.greenFor),
                       "green from when it arrived, not the time it says")

        // Past its green, still unseen: they keep adding up.
        let late = start.addingTimeInterval(3600)
        desk.apply(.report, at: late, permanent: true)
        XCTAssertEqual(desk.last?.count, 4)

        desk.apply(.seen, at: late.addingTimeInterval(1), permanent: true)
        XCTAssertEqual(desk.state(at: late.addingTimeInterval(1)), .thinking)
        XCTAssertEqual(desk.last?.count, 4, "seen, the card still says what it last did")
        desk.apply(.report, at: late.addingTimeInterval(60), permanent: true)
        XCTAssertEqual(desk.last, CloudAction(at: late.addingTimeInterval(60)), "after seen, counting starts again")
    }

    func testOpeningACloudCardMarksItSeenAndNothingElse() {
        let desks = CloudDesks([CloudBot(id: "smolcorp-mini", name: "Smolcorp mini")], looking: false)
        var changes = 0
        desks.onChange = { changes += 1 }
        let id = "cloud-smolcorp-mini"
        XCTAssertNil(desks.last(ofSession: id))
        desks.apply(.report, to: "smolcorp-mini", action: CloudAction(text: "Sent a report", count: 2, at: Date()))
        XCTAssertEqual(desks.seated(on: "upstairs").first?.state, .unread)
        desks.apply(.report, to: "smolcorp-mini")
        XCTAssertEqual(changes, 2, "a second report, still green, changes the card")
        XCTAssertEqual(desks.last(ofSession: id)?.count, 3)

        XCTAssertTrue(desks.seen(session: id))
        XCTAssertEqual(desks.seated(on: "upstairs").first?.state, .thinking, "blue at once")
        XCTAssertEqual(desks.last(ofSession: id)?.count, 3)
        XCTAssertFalse(desks.seen(session: "some-terminal-session"), "only a cloud bot's desk")
        XCTAssertFalse(desks.seen(session: "smolcorp-mini"), "by its desk's session id")
    }

    func testThePortableViewerSeesACloudDeskAndItsLastActionNotItsWordsWithNamesHidden() {
        let bot = Session(id: "cloud-smolcorp-mini", agent: .grok, folder: "/cloud/Smolcorp mini", project: "/cloud/Smolcorp mini", startedAt: 0)
        let run = Session(id: "run", agent: .claude, folder: "/tmp/run", project: "/tmp/run", startedAt: 0)
        let at = Date(timeIntervalSince1970: 1_800_000_000)
        let last = CloudAction(text: "Replied to @someone", count: 2, at: at)
        for hide in [false, true] {
            let snapshot = ViewerSnapshot.upstairs([bot, run], overflow: 0, hideNames: hide,
                                                   isCloud: { $0.id == bot.id }, last: { $0.id == bot.id ? last : nil })
            let desks = snapshot["desks"] as! [[String: Any]]
            XCTAssertEqual(desks[0]["cloud"] as? Bool, true)
            let shown = desks[0]["last"] as! [String: Any]
            XCTAssertEqual(shown["text"] as? String, hide ? "Sent a report" : "Replied to @someone")
            XCTAssertEqual(shown["count"] as? Int, 2)
            XCTAssertEqual(shown["at"] as? Int64, 1_800_000_000_000)
            XCTAssertNil(desks[1]["cloud"], "a headless run says nothing more than before")
            XCTAssertNil(desks[1]["last"])
        }
    }

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
        XCTAssertEqual(bots[0].floor, "upstairs", "upstairs unless set")
        XCTAssertEqual(bots[0].source, CloudBot.Source(kind: "grokBotApp", bot: "Smolcorp mini", counts: "unprompted"))
        XCTAssertEqual(bots[0].sessionID, "cloud-smolcorp-mini")
        XCTAssertEqual(bots[0].folder, "/cloud/Smolcorp mini")
        XCTAssertEqual(bots[1].name, "a-b")
        XCTAssertEqual(bots[1].outfit, "")
        XCTAssertEqual(bots[1].badge, "megaphone")
        XCTAssertEqual(bots[1].agent, "grok")
        XCTAssertFalse(bots[1].permanent)
        XCTAssertEqual(bots[1].floor, "upstairs", "a floor there isn't is upstairs")
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

    func testTheBotIsFoundByNameAndOnlyASafeIDIsTaken() {
        let roster = #"{"schemaVersion":4,"value":{"rows":[{"name":"Other","id":"AAAA"},{"name":"Smolcorp Mini","id":"6322898C-71DA-4BB8"}]}}"#
        XCTAssertEqual(GrokBotApp.botID(named: "smolcorp mini", inRoster: Data(roster.utf8)), "6322898C-71DA-4BB8")
        XCTAssertNil(GrokBotApp.botID(named: "nobody", inRoster: Data(roster.utf8)))
        let sneaky = #"{"value":{"rows":[{"name":"x","id":"../../secrets"}]}}"#
        XCTAssertNil(GrokBotApp.botID(named: "x", inRoster: Data(sneaky.utf8)))
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

    func testTheDeskSaysItsLiveNameAndKeepsItsSeatStateAndCard() {
        let desks = CloudDesks([CloudBot(id: "smolcorp-mini", name: "Smolcorp mini"), CloudBot(id: "xreplier", name: "Xreplier")], looking: false)
        var changes = 0
        desks.onChange = { changes += 1 }
        desks.apply(.report, to: "smolcorp-mini", action: CloudAction(text: "Sent a report", count: 2, at: Date()))
        changes = 0

        desks.rename("smolcorp-mini", to: "Smolcorp Mini Manager")
        let seated = desks.seated(on: "upstairs")
        XCTAssertEqual(seated.map(\.id), ["cloud-smolcorp-mini", "cloud-xreplier"], "the same desk, at the same seat")
        XCTAssertEqual(seated[0].projectName, "Smolcorp Mini Manager")
        XCTAssertEqual(seated[0].state, .unread, "still green")
        XCTAssertEqual(desks.last(ofSession: "cloud-smolcorp-mini")?.count, 2, "its card kept")
        XCTAssertEqual(changes, 1)
        desks.rename("smolcorp-mini", to: "Smolcorp Mini Manager")
        XCTAssertEqual(changes, 1, "no change, no redraw")

        desks.rename("smolcorp-mini", to: "Mini/Manager")
        XCTAssertEqual(desks.seated(on: "upstairs")[0].projectName, "Mini-Manager", "no slash, as a name set")
        desks.rename("smolcorp-mini", to: nil)
        XCTAssertEqual(desks.seated(on: "upstairs")[0].projectName, "Smolcorp mini", "no live name: the one it was set with")
        desks.rename("nobody", to: "X")

        // Set again, unchanged: it keeps the live name.
        desks.rename("smolcorp-mini", to: "Smolcorp Mini Manager")
        desks.configure([CloudBot(id: "smolcorp-mini", name: "Smolcorp mini"), CloudBot(id: "xreplier", name: "Xreplier")])
        XCTAssertEqual(desks.seated(on: "upstairs")[0].projectName, "Smolcorp Mini Manager")
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
