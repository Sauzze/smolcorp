import XCTest
@testable import AgentDeck

/// ⓘ Info (personal build): the price table, a transcript's tokens and model
/// as the card reads them, the account from Claude Code's profile, and the
/// card itself, which leaves out what it has no source for. Offline.
final class SessionInfoTests: XCTestCase {
    private var folder: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: folder)
    }

    // MARK: Sample lines, as Claude Code writes them

    private func json(_ object: [String: Any]) -> String {
        String(data: try! JSONSerialization.data(withJSONObject: object, options: [.sortedKeys]), encoding: .utf8)!
    }

    private func reply(_ id: String, model: String, usage: [String: Any], effort: String? = nil, sidechain: Bool = false,
                       at: String = "2026-10-23T10:00:00.000Z") -> String {
        var entry: [String: Any] = ["type": "assistant", "timestamp": at, "isSidechain": sidechain,
                                    "message": ["id": id, "model": model, "role": "assistant", "usage": usage,
                                                "content": [["type": "text", "text": "ok"]]]]
        if let effort { entry["effort"] = effort }
        return json(entry)
    }

    private func turn(_ ms: Int) -> String {
        json(["type": "system", "subtype": "turn_duration", "durationMs": ms, "timestamp": "2026-10-23T10:05:00.000Z"])
    }

    private func usage(input: Int = 0, output: Int = 0, read: Int = 0, write5m: Int = 0, write1h: Int = 0, fast: Bool = false) -> [String: Any] {
        var usage: [String: Any] = ["input_tokens": input, "output_tokens": output, "cache_read_input_tokens": read,
                                    "cache_creation_input_tokens": write5m + write1h,
                                    "cache_creation": ["ephemeral_5m_input_tokens": write5m, "ephemeral_1h_input_tokens": write1h]]
        if fast { usage["speed"] = "fast" }
        return usage
    }

    private func file(_ lines: [String], name: String = "s.jsonl") throws -> String {
        let url = folder.appendingPathComponent(name)
        try (lines.joined(separator: "\n") + "\n").write(to: url, atomically: true, encoding: .utf8)
        return url.path
    }

    private func append(_ lines: [String], to path: String, ending: Bool = true) throws {
        let handle = try XCTUnwrap(FileHandle(forWritingAtPath: path))
        try handle.seekToEnd()
        try handle.write(contentsOf: Data((lines.joined(separator: "\n") + (ending ? "\n" : "")).utf8))
        try handle.close()
    }

    // MARK: Prices

    /// A finished session's own `cost-state` on this Mac: 82 in, 31,715 out, 3,519,619 read, 166,201 written for an hour,
    /// on Opus 5.5, which Claude Code priced at $2.6681598.
    func testTheTableGivesWhatClaudeCodeChargedForASession() throws {
        let tokens = Tokens(input: 82, output: 31_715, cacheRead: 3_519_619, write1h: 166_201)
        XCTAssertEqual(try XCTUnwrap(Prices.cost(tokens, model: "claude-opus-5-5")), 2.6681598, accuracy: 0.000_01)
        // And Sonnet 5.5's cache reads, at 0.05x: $0.974259 for 90 in, 32,709 out, 2,708,685 read, 94,030 written for an hour.
        let sonnet = Tokens(input: 90, output: 32_709, cacheRead: 2_708_685, write1h: 94_030)
        XCTAssertEqual(try XCTUnwrap(Prices.cost(sonnet, model: "claude-sonnet-5-5")), 0.974259, accuracy: 0.000_01)
    }

    func testCacheWritesFastModeAndLongPrompts() throws {
        // 5-minute writes at 1.25x the input price, hour-long ones at 2x.
        XCTAssertEqual(try XCTUnwrap(Prices.cost(Tokens(write5m: 1_000_000), model: "claude-opus-5")), 6.25, accuracy: 1e-9)
        XCTAssertEqual(try XCTUnwrap(Prices.cost(Tokens(write1h: 1_000_000), model: "claude-opus-5")), 10, accuracy: 1e-9)
        // Fast mode, twice the price, where it has one; none for a model without.
        XCTAssertEqual(try XCTUnwrap(Prices.cost(Tokens(output: 1_000_000), model: "claude-opus-5-5", fast: true)), 40, accuracy: 1e-9)
        XCTAssertNil(Prices.cost(Tokens(output: 1_000_000), model: "claude-sonnet-5-5", fast: true))
        // Haiku 5.5 over 100K prompt tokens is priced higher, cache reads included.
        XCTAssertEqual(try XCTUnwrap(Prices.cost(Tokens(input: 100_000, output: 1_000_000), model: "claude-haiku-5-5")), 0.51, accuracy: 1e-9)
        XCTAssertEqual(try XCTUnwrap(Prices.cost(Tokens(input: 1, output: 1_000_000, cacheRead: 100_000), model: "claude-haiku-5-5")),
                       2.5 + 0.005 + 0.0000005, accuracy: 1e-9)
    }

    func testModelsAreFoundWithoutTheirSuffixesAndUnknownOnesHaveNoPrice() {
        XCTAssertEqual(Prices.id("claude-opus-5-5[1m]"), "claude-opus-5-5")
        XCTAssertEqual(Prices.id("claude-haiku-4-5-20251001"), "claude-haiku-4-5")
        XCTAssertEqual(Prices.rate("claude-fable-5-1")?.cacheRead, 0.25)
        XCTAssertNotEqual(Prices.rate("claude-opus-5"), Prices.rate("claude-opus-5-5"))
        XCTAssertNil(Prices.cost(Tokens(input: 1), model: "gpt-5"))
        XCTAssertNil(Prices.cost(Tokens(input: 1), model: "claude-opus-9"))
    }

    func testModelNames() {
        XCTAssertEqual(SessionInfo.modelName("claude-opus-5-5"), "Opus 5.5")
        XCTAssertEqual(SessionInfo.modelName("claude-opus-5-5[1m]"), "Opus 5.5 · 1M")
        XCTAssertEqual(SessionInfo.modelName("claude-haiku-4-5-20251001"), "Haiku 4.5")
        XCTAssertEqual(SessionInfo.modelName("claude-fable-5-1"), "Fable 5.1")
        XCTAssertEqual(SessionInfo.modelName("gpt-5.5-codex"), "gpt-5.5-codex")
    }

    // MARK: The transcript

    func testATranscriptWithSeveralModelsCountsEachReplyOnceAndShowsTheLatestModel() throws {
        let opus = usage(input: 10, output: 1_000, read: 100_000, write1h: 20_000)
        let haiku = usage(input: 500, output: 50)
        let path = try file([
            json(["type": "file-history-snapshot", "snapshot": [:]]),
            json(["type": "user", "timestamp": "2026-10-23T09:00:00.000Z", "message": ["role": "user", "content": "hi"]]),
            // A reply is written once for each of its blocks, all with the same usage.
            reply("m1", model: "claude-opus-5-5", usage: opus, effort: "high"),
            reply("m1", model: "claude-opus-5-5", usage: opus, effort: "high"),
            reply("m2", model: "claude-haiku-5-5", usage: haiku, sidechain: true),
            // Claude Code's own notice isn't a reply.
            reply("m3", model: "<synthetic>", usage: usage(output: 999)),
            turn(90_000),
            reply("m4", model: "claude-sonnet-5-5", usage: usage(input: 5, output: 200), effort: "medium"),
            turn(30_000),
        ])
        let follower = TranscriptFollower(path: path)
        follower.update()
        let tally = follower.tally
        XCTAssertEqual(tally.tokens, Tokens(input: 515, output: 1_250, cacheRead: 100_000, write1h: 20_000))
        XCTAssertEqual(tally.model, "claude-sonnet-5-5")
        XCTAssertEqual(tally.effort, "medium")
        XCTAssertEqual(tally.activeMs, 120_000)
        XCTAssertEqual(tally.unpriced, 0)
        XCTAssertEqual(try XCTUnwrap(tally.startedAt), try XCTUnwrap(Tally.time("2026-10-23T09:00:00.000Z")))
        let expected = [Prices.cost(Tokens(usage: opus), model: "claude-opus-5-5"), Prices.cost(Tokens(usage: haiku), model: "claude-haiku-5-5"),
                        Prices.cost(Tokens(input: 5, output: 200), model: "claude-sonnet-5-5")].compactMap { $0 }.reduce(0, +)
        XCTAssertEqual(tally.cost, expected, accuracy: 1e-12)
    }

    func testMissingFieldsCountAsNothingAndAnUnpricedModelIsCounted() throws {
        let path = try file([
            json(["type": "assistant", "message": ["id": "a", "model": "claude-opus-5-5"]]),
            json(["type": "assistant", "message": ["id": "b", "model": "claude-opus-5-5", "usage": ["output_tokens": 7,
                                                                                                    "cache_creation_input_tokens": 3]]]),
            json(["type": "assistant", "message": ["id": "c", "model": "claude-opus-9", "usage": ["output_tokens": 1]]]),
            json(["type": "assistant", "message": ["model": "claude-opus-5-5"]]),
            "not json at all",
            json(["type": "system", "subtype": "turn_duration"]),
        ])
        let follower = TranscriptFollower(path: path)
        follower.update()
        // A write with no split counts as 5 minutes'.
        XCTAssertEqual(follower.tally.tokens, Tokens(output: 8, write5m: 3))
        XCTAssertEqual(follower.tally.unpriced, 1)
        XCTAssertEqual(follower.tally.activeMs, 0)
        XCTAssertNil(follower.tally.startedAt)
        XCTAssertEqual(follower.tally.model, "claude-opus-5-5")
    }

    func testOnlyWhatsAddedIsReadAndALineIsTakenOnceItEnds() throws {
        let path = try file([reply("m1", model: "claude-opus-5-5", usage: usage(output: 100))])
        let follower = TranscriptFollower(path: path)
        follower.update()
        XCTAssertEqual(follower.tally.tokens.output, 100)
        // Half a line: not yet.
        let line = reply("m2", model: "claude-opus-5-5", usage: usage(output: 20))
        let half = line.index(line.startIndex, offsetBy: line.count / 2)
        try append([String(line[..<half])], to: path, ending: false)
        follower.update()
        XCTAssertEqual(follower.tally.tokens.output, 100)
        try append([String(line[half...])], to: path)
        follower.update()
        XCTAssertEqual(follower.tally.tokens.output, 120)
        follower.update()
        XCTAssertEqual(follower.tally.tokens.output, 120)
        // A file written again from the start is read again.
        _ = try file([reply("m9", model: "claude-opus-5-5", usage: usage(output: 5))])
        follower.update()
        XCTAssertEqual(follower.tally.tokens.output, 5)
    }

    func testATooLongLineIsSkipped() throws {
        let huge = json(["type": "user", "message": ["role": "user", "content": [["type": "tool_result",
                         "content": String(repeating: "x", count: TranscriptFollower.lineMost + 10)]]]])
        let path = try file([huge, reply("m1", model: "claude-opus-5-5", usage: usage(output: 3))])
        let follower = TranscriptFollower(path: path)
        follower.update()
        XCTAssertEqual(follower.tally.tokens.output, 3)
    }

    // MARK: The account

    private let now = 1_792_400_000_000.0

    private func iso(_ ms: Double) -> String {
        ISO8601DateFormatter().string(from: Date(timeIntervalSince1970: ms / 1000))
    }

    private func profile(fetchedAgo: Double = 120_000, owner: String = "acct-1", fiveResetsIn: Double = 3_600_000) -> [String: Any] {
        ["oauthAccount": ["accountUuid": "acct-1", "emailAddress": "me@example.com", "displayName": "Me",
                          "organizationType": "claude_max", "organizationRateLimitTier": "default_claude_max_20x",
                          "billingType": "stripe_subscription"],
         "cachedUsageUtilization": ["accountUuid": owner, "fetchedAtMs": now - fetchedAgo, "utilization": [
            "five_hour": ["utilization": 23, "resets_at": iso(now + fiveResetsIn)],
            "seven_day": ["utilization": 41, "resets_at": iso(now + 86_400_000)],
         ]],
         "primaryApiKey": "must never be read"]
    }

    func testASubscriptionWithItsUseAsClaudeCodeLastFetchedIt() throws {
        let account = try XCTUnwrap(ClaudeAccount.from(profile(), now: now))
        XCTAssertEqual(account.name, "me@example.com")
        XCTAssertEqual(account.plan, "Max 20x")
        XCTAssertTrue(account.subscription)
        XCTAssertEqual(account.fiveHour?.percent, 23)
        XCTAssertEqual(account.weekly?.percent, 41)
        XCTAssertEqual(try XCTUnwrap(account.fiveHour?.resetsAt), now + 3_600_000, accuracy: 1000)
        XCTAssertEqual(account.usageAt, now - 120_000)
    }

    func testUseThatsStaleResetOrAnotherAccountsIsLeftOut() throws {
        let stale = try XCTUnwrap(ClaudeAccount.from(profile(fetchedAgo: ClaudeAccount.usageFresh + 1), now: now))
        XCTAssertNil(stale.fiveHour)
        XCTAssertNil(stale.weekly)
        XCTAssertNil(stale.usageAt)
        XCTAssertNil(try XCTUnwrap(ClaudeAccount.from(profile(owner: "someone-else"), now: now)).weekly)
        let reset = try XCTUnwrap(ClaudeAccount.from(profile(fiveResetsIn: -1000), now: now))
        XCTAssertNil(reset.fiveHour)
        XCTAssertEqual(reset.weekly?.percent, 41)
        // Read earlier, looked at later: what's reset meanwhile goes.
        let later = SessionInfo.aged(try XCTUnwrap(ClaudeAccount.from(profile(), now: now)), now: now + 3_700_000)
        XCTAssertNil(later.fiveHour)
        XCTAssertNil(later.weekly)
    }

    func testNoLoginNoAccount() {
        XCTAssertNil(ClaudeAccount.from([:], now: now))
        XCTAssertEqual(ClaudeAccount.plan("claude_pro", tier: nil), "Pro")
        XCTAssertEqual(ClaudeAccount.plan("claude_max", tier: "default_claude_max_5x"), "Max 5x")
        XCTAssertNil(ClaudeAccount.plan("api", tier: nil))
    }

    func testTheProfileIsTheOneBesideTheTranscriptsSettings() {
        let home = "/Users/me"
        let standard = ClaudeAccount.configFolder(transcript: "/Users/me/.claude/projects/-Users-me-app/s.jsonl", home: home)
        XCTAssertEqual(standard, "/Users/me/.claude")
        XCTAssertEqual(ClaudeAccount.profilePath(folder: standard, home: home), "/Users/me/.claude.json")
        let own = ClaudeAccount.configFolder(transcript: "/Users/me/.claude-work/projects/-x/s.jsonl", home: home)
        XCTAssertEqual(ClaudeAccount.profilePath(folder: own, home: home), "/Users/me/.claude-work/.claude.json")
        XCTAssertEqual(ClaudeAccount.configFolder(transcript: "", home: home), "/Users/me/.claude")
    }

    func testAKeyHelperIsSeenOnlyByBeingThere() throws {
        let with = folder.appendingPathComponent("with.json")
        try Data(#"{"apiKeyHelper": "~/bin/key.sh"}"#.utf8).write(to: with)
        let without = folder.appendingPathComponent("without.json")
        try Data(#"{"model": "opus"}"#.utf8).write(to: without)
        XCTAssertTrue(SessionInfo.hasKeyHelper(settings: [without.path, with.path]))
        XCTAssertFalse(SessionInfo.hasKeyHelper(settings: [without.path, folder.appendingPathComponent("none.json").path]))
    }

    // MARK: The card

    private func desk(_ agent: Agent = .claude) -> SessionInfo.Desk {
        SessionInfo.Desk(id: "s1", agent: agent, project: "app", projectPath: "/p/app", transcript: "/t.jsonl",
                         startedAt: now - 7_200_000, model: "claude-opus-5", effort: "low", turnStartedAt: nil)
    }

    private func tally() -> Tally {
        var tally = Tally()
        tally.take(["type": "user", "timestamp": iso(now - 3_600_000)])
        tally.take(["type": "assistant", "effort": "high",
                    "message": ["id": "m", "model": "claude-opus-5-5", "usage": ["input_tokens": 10, "output_tokens": 1_000]]])
        tally.take(["type": "system", "subtype": "turn_duration", "durationMs": 60_000])
        return tally
    }

    func testASubscriptionsCardHasItsAccountUseAndWhatTheTokensWouldHaveCost() throws {
        let account = ClaudeAccount.from(profile(), now: now)
        let card = SessionInfo.card(desk(), tally: tally(), account: account, billing: nil, now: now)
        XCTAssertEqual(card["account"] as? [String: String], ["kind": "subscription", "name": "me@example.com", "plan": "Max 20x"])
        let model = try XCTUnwrap(card["model"] as? [String: Any])
        // The latest reply's model and effort, not the hook's.
        XCTAssertEqual(model["id"] as? String, "claude-opus-5-5")
        XCTAssertEqual(model["effort"] as? String, "high")
        let time = try XCTUnwrap(card["time"] as? [String: Double])
        XCTAssertEqual(try XCTUnwrap(time["sessionMs"]), 3_600_000, accuracy: 1000)
        XCTAssertEqual(time["activeMs"], 60_000)
        XCTAssertEqual(try XCTUnwrap(time["deskMs"]), 7_200_000, accuracy: 1)
        let cost = try XCTUnwrap(card["cost"] as? [String: Any])
        XCTAssertEqual(cost["basis"] as? String, "equivalent")
        XCTAssertEqual(try XCTUnwrap(cost["usd"] as? Double), 0.02, accuracy: 0.0001)
        let limits = try XCTUnwrap(card["limits"] as? [String: Any])
        XCTAssertEqual((limits["fiveHour"] as? [String: Any])?["percent"] as? Double, 23)
        XCTAssertEqual(limits["ageMs"] as? Double, 120_000)
        // Nothing of a key, nor the account's id.
        let text = String(decoding: try JSONSerialization.data(withJSONObject: card), as: UTF8.self)
        XCTAssertFalse(text.contains("never be read"))
        XCTAssertFalse(text.contains("acct-1"))
    }

    func testAnAPIKeysCardHasTheCostAndNoSubscription() throws {
        let card = SessionInfo.card(desk(), tally: tally(), account: ClaudeAccount.from(profile(), now: now), billing: .apiKey, now: now)
        XCTAssertEqual(card["account"] as? [String: String], ["kind": "api-key"])
        XCTAssertNil(card["limits"])
        XCTAssertEqual((card["cost"] as? [String: Any])?["basis"] as? String, "api")
        // A cloud provider has its own prices: tokens, but no cost.
        let cloud = SessionInfo.card(desk(), tally: tally(), account: nil, billing: .cloud, now: now)
        XCTAssertNotNil(cloud["tokens"])
        XCTAssertNil(cloud["cost"])
    }

    func testWhatHasNoSourceIsLeftOut() throws {
        // Another agent: no transcript read, so no tokens, cost or account; its model and time, from its hooks.
        let codex = SessionInfo.card(SessionInfo.Desk(id: "c", agent: .codex, project: "x", projectPath: "/x", transcript: "",
                                                      startedAt: now - 60_000, model: "", effort: "", turnStartedAt: nil),
                                     tally: nil, account: nil, billing: nil, now: now)
        XCTAssertEqual(Set(codex.keys), ["agent", "desk", "time"])
        XCTAssertEqual(codex["time"] as? [String: Double], ["sessionMs": 60_000])
        // A reply with no price: the tokens, not a wrong sum.
        var unpriced = tally()
        unpriced.take(["type": "assistant", "message": ["id": "x", "model": "claude-opus-9", "usage": ["output_tokens": 1]]])
        let card = SessionInfo.card(desk(), tally: unpriced, account: nil, billing: nil, now: now)
        XCTAssertNotNil(card["tokens"])
        XCTAssertNil(card["cost"])
        XCTAssertNil(card["account"])
        XCTAssertNil(card["limits"])
        // A session as old as its desk has no separate desk time.
        var fresh = desk()
        fresh.startedAt = now - 3_600_000
        XCTAssertNil((SessionInfo.card(fresh, tally: tally(), account: nil, billing: nil, now: now)["time"] as? [String: Any])?["deskMs"])
    }
}
