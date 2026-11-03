import XCTest
@testable import AgentDeck

/// A manager hire's own settings file: its content, its mode, what stays writable, and the refusals.
/// The guard is a stand-in that prints the groups; nothing real is run.
final class HireSettingsTests: XCTestCase {
    private var root: URL!
    private var support: URL!
    private let cli = URL(fileURLWithPath: "/Applications/Smolcorp.app/Contents/Resources/bin/smolcorp")
    private var work: ClaudeProfile!

    override func setUpWithError() throws {
        root = URL(fileURLWithPath: HireDesk.physical(NSTemporaryDirectory())).appendingPathComponent("hiresettings-\(UUID().uuidString)")
        support = root.appendingPathComponent("support")
        try FileManager.default.createDirectory(at: support.appendingPathComponent("hires/other11"), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: support.appendingPathComponent("events"), withIntermediateDirectories: true)
        work = ClaudeProfile(name: "work", dir: root.appendingPathComponent("claude-work").path)
    }

    override func tearDownWithError() throws { try? FileManager.default.removeItem(at: root) }

    private let printed = """
    {"groups":[{"name":"push","on":true,"commands":["git push|send-pack"]},
               {"name":"deploy","on":true,"commands":["vercel","eas build|submit","fly","docker buildx build|bake --push"]},
               {"name":"cloud","on":false,"commands":["aws"]},
               {"name":"remote","on":true,"commands":["rsync @remote","tmux ... send-keys"]}],
     "file":"x","problem":""}
    """

    private func write(_ id: String = "abc123", auto: Bool = true, printing: String? = nil, executable: Bool = true) throws -> URL {
        let text = printing ?? printed
        return try HireSettings.write(hireID: id, auto: auto, cli: cli, support: support, profiles: [work],
                                      run: { _, _ in (0, text) }, isExecutable: { _ in executable })
    }

    private func json(_ url: URL) throws -> [String: Any] {
        try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])
    }

    func testTheFileHoldsTheDenyRulesTheGuardAndAutoMode() throws {
        let url = try write()
        XCTAssertEqual(url, support.appendingPathComponent("hires/abc123/settings.json"))
        let settings = try json(url)
        let permissions = try XCTUnwrap(settings["permissions"] as? [String: Any])
        XCTAssertEqual(permissions["defaultMode"] as? String, "auto")
        let deny = try XCTUnwrap(permissions["deny"] as? [String])
        // The guard's groups in force, plain: alternatives apart, through npx too; groups off and rules a pattern can't say left out.
        for rule in ["Bash(git push:*)", "Bash(git send-pack:*)", "Bash(vercel:*)", "Bash(npx vercel:*)", "Bash(eas build:*)", "Bash(eas submit:*)", "Bash(fly:*)"] {
            XCTAssertTrue(deny.contains(rule), rule)
        }
        for rule in ["Bash(aws:*)", "Bash(rsync:*)", "Bash(tmux:*)", "Bash(docker buildx build:*)"] { XCTAssertFalse(deny.contains(rule), rule) }
        for rule in ["Read(**/.env*)", "Edit(**/.env*)", "Edit(//**/.env*)", "Edit(~/.claude*/settings*.json)", "Edit(**/.claude/settings*.json)",
                     "Read(~/.claude*/projects/**)", "Edit(//\(work.dir.dropFirst())/settings*.json)", "Read(//\(work.dir.dropFirst())/projects/**)",
                     "Edit(//\(support.path.dropFirst())/*.json)",
                     "Read(//\(support.path.dropFirst())/gm-token)", "Edit(//\(support.path.dropFirst())/gm-token)",
                     "Edit(//\(support.path.dropFirst())/hires/*/*.json)", "Edit(//\(support.path.dropFirst())/hires/other11/**)",
                     "Edit(//\(support.path.dropFirst())/events/**)"] {
            XCTAssertTrue(deny.contains(rule), rule)
        }
        // Edit rules cover Write too; a Write(path) rule never matches and makes Claude Code warn at every hire start.
        XCTAssertFalse(deny.contains { $0.hasPrefix("Write(") }, "no Write(...) rules")
        // The guard runs before every tool, reading the app's hire-guard.json.
        let hooks = try XCTUnwrap((settings["hooks"] as? [String: Any])?["PreToolUse"] as? [[String: Any]])
        XCTAssertEqual(hooks.count, 1)
        XCTAssertEqual(hooks[0]["matcher"] as? String, "*")
        let command = try XCTUnwrap(((hooks[0]["hooks"] as? [[String: Any]])?.first)?["command"] as? String)
        XCTAssertEqual(command, "'\(cli.path)' guard --rules '\(support.path)/hire-guard.json'")
        XCTAssertEqual(settings["disableAllHooks"] as? Bool, false, "hooks stay on, whatever a checkout's own settings say")
    }

    /// Off means allowed (m6): a group turned off gives no deny rules, so the guard and the rules agree.
    func testAGroupTurnedOffDropsItsDenyRulesAndBackOnBringsThemBack() throws {
        func groups(deployOn: Bool) -> [HireSettings.Group] {
            let json = """
            [{"name":"push","on":true,"commands":["git push"]},{"name":"deploy","on":\(deployOn),"commands":["vercel","eas build|submit"]}]
            """
            return try! JSONDecoder().decode([HireSettings.Group].self, from: Data(json.utf8))
        }
        let on = HireSettings.commandRules(groups(deployOn: true))
        XCTAssertTrue(on.contains("Bash(vercel:*)"))
        XCTAssertTrue(on.contains("Bash(npx eas submit:*)"))
        let off = HireSettings.commandRules(groups(deployOn: false))
        XCTAssertEqual(off, ["Bash(git push:*)", "Bash(npx git push:*)"], "only the group still on is denied")
        XCTAssertEqual(HireSettings.commandRules(groups(deployOn: true)), on)
    }

    /// The GM's file has the gm profile in its hook (and asks the guard for the gm groups); a hire's has the hire profile.
    func testTheGMsHookUsesTheGMProfileAndAHiresDoesNot() throws {
        func hook(_ url: URL) throws -> String {
            let hooks = try XCTUnwrap((try json(url)["hooks"] as? [String: Any])?["PreToolUse"] as? [[String: Any]])
            return try XCTUnwrap(((hooks[0]["hooks"] as? [[String: Any]])?.first)?["command"] as? String)
        }
        var asked: [[String]] = []
        let gm = try HireSettings.write(hireID: "gm0001", profile: .gm, cli: cli, support: support, profiles: [work],
                                        run: { _, arguments in asked.append(arguments); return (0, self.printed) }, isExecutable: { _ in true })
        XCTAssertEqual(try hook(gm), "'\(cli.path)' guard --profile gm --rules '\(support.path)/hire-guard.json'")
        XCTAssertEqual(asked.first, ["guard", "--print", "--profile", "gm", "--rules", "\(support.path)/hire-guard.json"])
        let hire = try write("hire01")
        XCTAssertEqual(try hook(hire), "'\(cli.path)' guard --rules '\(support.path)/hire-guard.json'")
        XCTAssertFalse(try hook(hire).contains("--profile"))
    }

    /// The GM keeps the deny rules a hire gets: no editing Claude's settings or the app's state files, no transcripts;
    /// it may read its team's STATUS files (no rule denies reading them).
    func testTheGMKeepsTheDenyRulesAndMayReadTheTeamsStatusFiles() throws {
        let url = try HireSettings.write(hireID: "gm0001", profile: .gm, cli: cli, support: support, profiles: [work],
                                         run: { _, _ in (0, self.printed) }, isExecutable: { _ in true })
        let deny = try XCTUnwrap((try json(url)["permissions"] as? [String: Any])?["deny"] as? [String])
        let root = String(support.path.dropFirst())
        for rule in ["Edit(//\(root)/*.json)", "Edit(~/.claude*/settings*.json)", "Read(~/.claude*/projects/**)",
                     "Read(//\(root)/hire-guard.json)", "Edit(//\(root)/hires/other11/**)", "Edit(//\(root)/hires/*/settings*)",
                     "Bash(git push:*)", "Bash(vercel:*)"] {
            XCTAssertTrue(deny.contains(rule), rule)
        }
        XCTAssertFalse(deny.contains { $0.hasPrefix("Read(") && $0.contains("/hires/") }, "reading the team's STATUS.md files is not denied")
        XCTAssertFalse(deny.contains { $0.contains("STATUS") })
    }

    func testAutoModeIsLeftOutWhenNotWanted() throws {
        let permissions = try XCTUnwrap(try json(write(auto: false))["permissions"] as? [String: Any])
        XCTAssertNil(permissions["defaultMode"])
    }

    func testTheFileIsPrivateAndInAFolderOfItsOwn() throws {
        let url = try write()
        let mode = try FileManager.default.attributesOfItem(atPath: url.path)[.posixPermissions] as? Int
        XCTAssertEqual(mode, 0o600)
        let folder = try FileManager.default.attributesOfItem(atPath: url.deletingLastPathComponent().path)[.posixPermissions] as? Int
        XCTAssertEqual(folder, 0o700)
        // Written again, it is replaced whole.
        _ = try write(auto: false)
        XCTAssertNil((try json(url)["permissions"] as? [String: Any])?["defaultMode"])
    }

    /// A deny rule beats any allow, so no rule may cover the hire's own STATUS.md, while the rest of the app's folder is covered.
    func testTheHiresOwnStatusFileIsNotDeniedButItsSettingsAndTheRestAre() throws {
        let deny = try XCTUnwrap((try json(write())["permissions"] as? [String: Any])?["deny"] as? [String])
        func denied(_ path: String, _ asked: String) -> Bool {
            let tool = asked == "Write" ? "Edit" : asked // Claude Code checks Edit rules for every tool that changes a file
            return deny.contains { rule in
                guard rule.hasPrefix(tool + "("), rule.hasSuffix(")") else { return false }
                let pattern = String(rule.dropFirst(tool.count + 1).dropLast())
                return pattern.hasPrefix("//") && Self.glob(String(pattern.dropFirst()), path)
            }
        }
        let base = support.path
        for tool in ["Edit", "Write"] {
            XCTAssertFalse(denied("\(base)/hires/abc123/STATUS.md", tool), "its own status file must stay writable")
            XCTAssertTrue(denied("\(base)/hires/abc123/settings.json", tool), "its own settings")
            XCTAssertTrue(denied("\(base)/hire-guard.json", tool))
            XCTAssertTrue(denied("\(base)/gm-token", tool))
            XCTAssertTrue(denied("\(base)/gm-session", tool))
            XCTAssertTrue(denied("\(base)/frontdesk.json", tool), "the registry")
            XCTAssertTrue(denied("\(base)/events/outbox.log", tool))
            XCTAssertTrue(denied("\(base)/hires/other11/STATUS.md", tool), "another hire's folder")
            XCTAssertTrue(denied("\(base)/hires/hire-1234.sh", tool))
        }
        for name in ["gm-token", "gm-session", "setup-key", "hook-header", "token", "hire-guard.json", "phones.json", "sessions.json", "config.json", "registry.json"] {
            XCTAssertTrue(denied("\(base)/\(name)", "Read"), "\(name) can't be read")
        }
        XCTAssertTrue(denied("\(work.dir)/projects/x/session.jsonl", "Read"))
        XCTAssertTrue(denied("\(work.dir)/settings.json", "Edit"))
    }

    /// A hire started after this file was written has no folder yet: its settings are still out of reach.
    func testALaterHiresSettingsAreDeniedButItsStatusFileIsNot() throws {
        let deny = try XCTUnwrap((try json(write())["permissions"] as? [String: Any])?["deny"] as? [String])
        func denied(_ path: String, _ asked: String) -> Bool {
            let tool = asked == "Write" ? "Edit" : asked // Claude Code checks Edit rules for every tool that changes a file
            return deny.contains { rule in
                guard rule.hasPrefix(tool + "("), rule.hasSuffix(")") else { return false }
                let pattern = String(rule.dropFirst(tool.count + 1).dropLast())
                return pattern.hasPrefix("//") && Self.glob(String(pattern.dropFirst()), path)
            }
        }
        let base = support.path
        XCTAssertFalse(FileManager.default.fileExists(atPath: support.appendingPathComponent("hires/late99").path))
        for tool in ["Edit", "Write"] {
            XCTAssertTrue(denied("\(base)/hires/late99/settings.json", tool))
            XCTAssertTrue(denied("\(base)/hires/late99/settings.local.json", tool))
            XCTAssertTrue(denied("\(base)/hires/late99/run.sh", tool))
            XCTAssertFalse(denied("\(base)/hires/abc123/STATUS.md", tool), "its own status file stays writable")
        }
    }

    /// Folders the app makes later (none there now) are covered from the fixed list.
    func testKnownFoldersAreDeniedEvenWhenAbsent() throws {
        let deny = try XCTUnwrap((try json(write())["permissions"] as? [String: Any])?["deny"] as? [String])
        for name in HireSettings.knownFolders {
            XCTAssertFalse(FileManager.default.fileExists(atPath: support.appendingPathComponent(name).path))
            XCTAssertTrue(deny.contains("Edit(//\(support.path.dropFirst())/\(name)/**)"), name)
        }
    }

    func testNoSmolcorpCommandMeansNoHire() throws {
        XCTAssertThrowsError(try write(executable: false)) { error in
            let refusal = error as? FrontDesk.Refusal
            XCTAssertEqual(refusal?.message, HireSettings.guardMissing.message)
            XCTAssertTrue(refusal?.message.contains("unguarded") == true)
        }
        XCTAssertFalse(FileManager.default.fileExists(atPath: support.appendingPathComponent("hires/abc123").path), "nothing is left behind")
    }

    func testAGuardThatCantBeRunMeansNoHire() throws {
        for answer in ["not json", #"{"problem":"x"}"#] {
            XCTAssertThrowsError(try write(printing: answer))
        }
        XCTAssertThrowsError(try HireSettings.write(hireID: "abc123", cli: cli, support: support, profiles: [work], run: { _, _ in nil }, isExecutable: { _ in true }))
        XCTAssertThrowsError(try HireSettings.write(hireID: "abc123", cli: cli, support: support, profiles: [work], run: { _, _ in (1, self.printed) }, isExecutable: { _ in true }))
        XCTAssertFalse(FileManager.default.fileExists(atPath: support.appendingPathComponent("hires/abc123/settings.json").path))
    }

    func testTheGuardIsAskedAboutTheAppsOwnRulesFile() throws {
        var asked: [String] = []
        _ = try HireSettings.write(hireID: "abc123", cli: cli, support: support, profiles: [work],
                                   run: { tool, arguments in asked = [tool] + arguments; return (0, self.printed) }, isExecutable: { _ in true })
        XCTAssertEqual(asked, [cli.path, "guard", "--print", "--rules", support.appendingPathComponent("hire-guard.json").path])
    }

    func testOnlyAClaudeHireWithSettingsStartsWithThem() {
        var role = Role.shipped[0]
        role.agent = Agent.claude.rawValue
        let settings = "/s/hires/abc123/settings.json"
        XCTAssertTrue(role.startScript(firstMessage: "go", settings: settings).contains("'claude' '--settings=\(settings)'"))
        XCTAssertFalse(role.startScript(firstMessage: "go").contains("--settings"), "an unmanaged hire gets nothing new")
        role.agent = Agent.codex.rawValue
        XCTAssertFalse(role.startScript(firstMessage: "go", settings: settings).contains("--settings"))
    }

    // MARK: A test copy's hire

    func testANormalHiresStartAndSettingsAreAsTheyWere() throws {
        var role = Role.shipped[0]
        role.agent = Agent.claude.rawValue
        let script = role.startScript(firstMessage: "go", settings: "/s/settings.json", testCopy: false)
        XCTAssertFalse(script.contains("setting-sources"))
        XCTAssertTrue(script.contains("'claude' '--settings=/s/settings.json' "))
        // Settings written without reporting hooks hold the guard alone, as before.
        let object = try json(write())
        let hooks = try XCTUnwrap(object["hooks"] as? [String: Any])
        XCTAssertEqual(Set(hooks.keys), ["PreToolUse"])
        XCTAssertEqual((hooks["PreToolUse"] as? [[String: Any]])?.count, 1)
        let plain = HireSettings.settings(deny: ["x"], guardCommand: "g", auto: true)
        let expected: [String: Any] = ["permissions": ["deny": ["x"], "defaultMode": "auto"] as [String: Any], "disableAllHooks": false,
                                       "hooks": ["PreToolUse": [["matcher": "*", "hooks": [["type": "command", "command": "g", "timeout": 10] as [String: Any]]]]]]
        XCTAssertEqual(try JSONSerialization.data(withJSONObject: plain, options: .sortedKeys),
                       try JSONSerialization.data(withJSONObject: expected, options: .sortedKeys))
    }

    func testATestCopysHireIsStartedWithoutTheAccountsSettings() {
        var role = Role.shipped[0]
        role.agent = Agent.claude.rawValue
        let script = role.startScript(firstMessage: "go", settings: "/s/settings.json", testCopy: true)
        XCTAssertTrue(script.contains("'--settings=/s/settings.json' '--setting-sources=project,local'"))
        role.agent = Agent.codex.rawValue
        XCTAssertFalse(role.startScript(firstMessage: "go", testCopy: true).contains("setting-sources"), "only Claude Code takes it")
    }

    func testATestCopysSettingsReportToTheCopysScriptAndKeepTheGuard() throws {
        let copyScript = support.appendingPathComponent("report.sh")
        let url = try HireSettings.write(hireID: "abc123", cli: cli, support: support, profiles: [work], run: { _, _ in (0, self.printed) },
                                         reporting: Hooks.hireHooks(script: copyScript), isExecutable: { _ in true })
        let object = try json(url)
        let hooks = try XCTUnwrap(object["hooks"] as? [String: Any])
        XCTAssertEqual(Set(hooks.keys), Set(Hooks.claudeEvents))
        for event in Hooks.claudeEvents {
            let text = String(data: try JSONSerialization.data(withJSONObject: hooks[event] as Any, options: .withoutEscapingSlashes), encoding: .utf8) ?? ""
            XCTAssertTrue(text.contains(copyScript.path), event)
        }
        // The guard is still first on PreToolUse, and the deny rules and the rest are as for any hire.
        let pre = try XCTUnwrap(hooks["PreToolUse"] as? [[String: Any]])
        XCTAssertEqual(pre.count, 2)
        XCTAssertEqual(pre[0]["matcher"] as? String, "*")
        let guardHook = try XCTUnwrap((pre[0]["hooks"] as? [[String: Any]])?.first)
        XCTAssertTrue((guardHook["command"] as? String ?? "").contains(" guard --rules "))
        XCTAssertEqual(object["disableAllHooks"] as? Bool, false)
        let permissions = object["permissions"] as? NSDictionary
        XCTAssertEqual(try json(write("abc123"))["permissions"] as? NSDictionary, permissions, "the deny rules are those of a normal hire")
    }

    func testTheReportScriptSendsToTheCopysPortAndNeverTheRealOne() throws {
        let template = root.appendingPathComponent("report-template.sh")
        try "curl http://127.0.0.1:{{PORT}}/hook".write(to: template, atomically: true, encoding: .utf8)
        let target = support.appendingPathComponent("report.sh")
        try Hooks.writeScript(template: template, to: target, port: 7751)
        let text = try String(contentsOf: target, encoding: .utf8)
        XCTAssertTrue(text.contains(":7751/"))
        XCTAssertFalse(text.contains("7710"))
        let hooks = Hooks.hireHooks(script: target)
        XCTAssertFalse(String(data: try JSONSerialization.data(withJSONObject: hooks), encoding: .utf8)!.contains("7710"))
    }

    func testAnUnguardedTestCopyHireGetsTheHooksAlone() throws {
        let hooks = Hooks.hireHooks(script: support.appendingPathComponent("report.sh"))
        let url = try HireSettings.writeReportingOnly(hireID: "plain1", hooks: hooks, support: support)
        let object = try json(url)
        XCTAssertEqual(Set(object.keys), ["hooks"])
        XCTAssertEqual(Set((object["hooks"] as? [String: Any] ?? [:]).keys), Set(Hooks.claudeEvents))
    }

    func testATestCopyHiresClaudeCodeAndNoOtherAgent() {
        XCTAssertNil(Deck.testCopyRefusal(for: .claude, testCopy: true), "the old 409 is gone")
        XCTAssertNotNil(Deck.testCopyRefusal(for: .codex, testCopy: true))
        XCTAssertNotNil(Deck.testCopyRefusal(for: nil, testCopy: true))
        XCTAssertNil(Deck.testCopyRefusal(for: .codex, testCopy: false))
        XCTAssertNil(Deck.testCopyRefusal(for: nil, testCopy: false))
    }

    func testACopysHiresTalkToTheCopyAndItsFolderNotTheRealApp() {
        let folder = URL(fileURLWithPath: "/tmp/copy-support", isDirectory: true)
        let lines = HireDesk.scriptLines(ports: "", apiKey: false, copyPort: 7751, copySupport: folder)
        XCTAssertTrue(lines.contains("export SMOLCORP_PORT='7751'\n"))
        XCTAssertTrue(lines.contains("export SMOLCORP_TEST_SUPPORT_DIR='/tmp/copy-support'\n"))
        // Outside a copy the lines are exactly what they were.
        XCTAssertEqual(HireDesk.scriptLines(ports: "", apiKey: false, copyPort: nil, copySupport: nil), "")
        XCTAssertEqual(HireDesk.scriptLines(ports: "4100-4103", apiKey: false, sendKey: "k-1_a", copyPort: nil, copySupport: nil),
                       "export SMOLCORP_HIRE_PORTS='4100-4103'\nexport \(FrontDesk.sendKeyEnvironment)='k-1_a'\n")
        XCTAssertFalse(HireDesk.scriptLines(ports: "4100-4103", apiKey: true, sendKey: "k", copyPort: nil, copySupport: nil).contains("SMOLCORP_PORT"))
        // A role's start script carries them, so does a role-less hire's command line.
        var role = Role.shipped[0]
        role.agent = Agent.claude.rawValue
        XCTAssertTrue(role.startScript(firstMessage: "go", environment: lines, testCopy: true).contains("export SMOLCORP_PORT='7751'"))
        XCTAssertEqual(HireDesk.copyPrefix(port: 7751, support: folder), "SMOLCORP_PORT='7751' SMOLCORP_TEST_SUPPORT_DIR='/tmp/copy-support' ")
    }

    func testCanHireMeansTheDefaultAgentCanHireHere() {
        func canHire(_ agent: String, testCopy: Bool) -> Bool {
            Deck.testCopyRefusal(for: Deck.hiredAgent(role: nil, hireAgent: agent), testCopy: testCopy) == nil
        }
        XCTAssertTrue(canHire(Agent.claude.rawValue, testCopy: true))
        XCTAssertFalse(canHire(Agent.codex.rawValue, testCopy: true))
        XCTAssertTrue(canHire(Agent.codex.rawValue, testCopy: false), "outside a copy nothing is refused")
    }

    /// Just enough of gitignore's globbing for the rules above: `**` crosses folders, `*` doesn't.
    private static func glob(_ pattern: String, _ path: String) -> Bool {
        var regex = "^"
        var i = pattern.startIndex
        while i < pattern.endIndex {
            let c = pattern[i]
            if c == "*", pattern.index(after: i) < pattern.endIndex, pattern[pattern.index(after: i)] == "*" {
                regex += ".*"
                i = pattern.index(i, offsetBy: 2)
                continue
            }
            regex += c == "*" ? "[^/]*" : NSRegularExpression.escapedPattern(for: String(c))
            i = pattern.index(after: i)
        }
        return path.range(of: regex + "$", options: .regularExpression) != nil
    }
}
