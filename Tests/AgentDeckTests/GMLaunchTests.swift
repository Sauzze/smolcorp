import XCTest
@testable import AgentDeck

/// "Hire your General Manager" starts the GM through the app: its token goes into its own
/// start script's environment and nowhere else, and only one GM works at a time.
final class GMLaunchTests: XCTestCase {
    private let token = String(repeating: "t", count: 43)

    private func gmScript(apiKey: Bool = false) -> String {
        var role = Role.shipped.first { $0.id == Deck.gmRole } ?? Role.shipped[0]
        role.agent = Agent.claude.rawValue
        return role.startScript(firstMessage: "go", environment: HireDesk.scriptLines(ports: "4100-4103", apiKey: apiKey, gmToken: token))
    }

    func testTheStartScriptExportsTheTokenForTheGMAlone() {
        let script = gmScript()
        XCTAssertTrue(script.contains("export SMOLCORP_GM_TOKEN='\(token)'\n"))
        XCTAssertTrue(script.contains("rm -f \"$0\""), "the script removes itself before it starts anything")
        XCTAssertLessThan(script.range(of: "rm -f")!.lowerBound, script.range(of: "SMOLCORP_GM_TOKEN")!.lowerBound)
        XCTAssertEqual(script.components(separatedBy: token).count, 2, "the token appears once")
        let exported = script.split(separator: "\n").filter { $0.hasPrefix("export ") }.map { $0.split(separator: "=")[0] }
        XCTAssertEqual(Set(exported), ["export SMOLCORP_HIRE_PORTS", "export SMOLCORP_GM_TOKEN"], "nothing else secret is exported")
        XCTAssertFalse(script.contains("ANTHROPIC_API_KEY"))
        XCTAssertFalse(script.contains("security find-generic-password"))
    }

    func testTheGMGetsNoSendKeyAndOtherRolesDo() {
        let gm = Role.shipped.first { $0.id == Deck.gmRole }!
        let other = Role.shipped.first { $0.id != Deck.gmRole }!
        let gmKey = Deck.sendKey(for: gm)
        XCTAssertEqual(gmKey, "", "the GM signs with its token, so it has no hire key")
        let gmLines = HireDesk.scriptLines(ports: "4100-4103", apiKey: false, gmToken: token, sendKey: gmKey)
        XCTAssertTrue(gmLines.contains("SMOLCORP_GM_TOKEN"))
        XCTAssertFalse(gmLines.contains(FrontDesk.sendKeyEnvironment))
        let key = Deck.sendKey(for: other)
        XCTAssertFalse(key.isEmpty)
        let lines = HireDesk.scriptLines(ports: "4100-4103", apiKey: false, sendKey: key)
        XCTAssertTrue(lines.contains(FrontDesk.sendKeyEnvironment))
        XCTAssertFalse(lines.contains("SMOLCORP_GM_TOKEN"))
        XCTAssertEqual(Deck.sendKey(for: nil), "", "no role, no start script, no key")
    }

    func testNoOtherHireIsGivenTheToken() {
        XCTAssertFalse(HireDesk.scriptLines(ports: "4100-4103", apiKey: false).contains("SMOLCORP_GM_TOKEN"))
        XCTAssertFalse(HireDesk.scriptLines(ports: "", apiKey: true).contains("SMOLCORP_GM_TOKEN"))
    }

    func testTheTokenIsNotOnAnyCommandLineOrInTheFirstMessage() {
        let message = Deck.gmIntroduction(folder: "/Users/me/Projects/app")
        XCTAssertFalse(message.contains(token))
        XCTAssertTrue(message.contains("/Users/me/Projects/app"), "it is told its folder")
        XCTAssertTrue(message.contains("General Manager"), "and its role")
        XCTAssertTrue(message.contains("`smolcorp gm`"), "and that smolcorp gm is its tool")
        XCTAssertFalse(message.contains("SMOLCORP_GM_TOKEN"), "no launch line or variable for the user to deal with")
    }

    func testTheIntroductionAddsThePluginNoteWhenTheToolsAreMissing() {
        let without = GMPlugin.Launch(plugin: "/p", cli: nil)
        XCTAssertTrue(Deck.gmIntroduction(folder: "/f", plugin: without).contains("not available in this copy of Smolcorp"))
        let with = GMPlugin.Launch(plugin: "/p", cli: "/bin/smolcorp")
        XCTAssertEqual(Deck.gmIntroduction(folder: "/f", plugin: with), Deck.gmIntroduction(folder: "/f", plugin: nil))
    }

    func testTheGMRoleIsTheOneSetupHires() {
        XCTAssertTrue(Role.shipped.contains { $0.id == Deck.gmRole })
    }

    // MARK: Only one at a time

    private func hire(role: String, _ state: FrontDesk.State) -> FrontDesk.Hire {
        FrontDesk.Hire(id: UUID().uuidString, name: role, role: role, folder: "/x", state: state)
    }

    private func session(in folder: String) -> FrontDesk.Live {
        FrontDesk.Live(id: UUID().uuidString, agent: "claude", folder: folder, project: folder, state: "working")
    }

    func testASecondGMIsRefusedWhileOneIsWorking() {
        for state in [FrontDesk.State.starting, .working, .idle] {
            let gm = hire(role: "gm", state)
            let refusal = FrontDesk.gmRefusal(among: [gm], live: [session(in: gm.folder)])
            XCTAssertEqual(refusal?.status, 409, "\(state)")
            XCTAssertTrue(refusal?.message.contains("Fire it first") == true)
        }
    }

    func testAFiredOrFailedGMOrAnotherRoleDoesNotBlockTheNextGM() {
        XCTAssertNil(FrontDesk.gmRefusal(among: [], live: []))
        XCTAssertNil(FrontDesk.gmRefusal(among: [hire(role: "gm", .fired), hire(role: "gm", .failed)], live: []))
        XCTAssertNil(FrontDesk.gmRefusal(among: [hire(role: "builder", .working)], live: []))
    }

    func testAGMWhoseSessionDiedDoesNotBlockANewOne() {
        for state in [FrontDesk.State.working, .idle] {
            XCTAssertNil(FrontDesk.gmRefusal(among: [hire(role: "gm", state)], live: []), "\(state), no session")
            XCTAssertNil(FrontDesk.gmRefusal(among: [hire(role: "gm", state)], live: [session(in: "/elsewhere")]), "\(state), another folder's session")
        }
        XCTAssertEqual(FrontDesk.gmRefusal(among: [hire(role: "gm", .starting)], live: [])?.status, 409, "one still starting has no session yet")
    }

    // MARK: The routes

    func testTheTokenFileSettingIsPageOnlyAndStaysOpenWithTheManagerOff() {
        XCTAssertEqual(SetupRoutes.row("POST", "/setup/gm/by-hand")?.callers, [.page])
        XCTAssertFalse(SetupRoutes.allows(.gm, "POST", "/setup/gm/by-hand"))
        XCTAssertFalse(SetupRoutes.allows(.tool, "POST", "/setup/gm/by-hand"))
        XCTAssertNil(ManagerSwitch.refusal("POST", "/setup/gm/by-hand", enabled: false))
    }

    func testNoTokenFileExistsUnlessTheSettingIsOn() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("gm-launch-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let file = folder.appendingPathComponent("gm-token")
        var allowed = false
        let gm = GMIdentity(store: FakeGMTokenStore(), tokenFile: file, fileAllowed: { allowed })
        // Hiring: a new token, as the hire does.
        XCTAssertNotNil(gm.rotate())
        XCTAssertNotNil(gm.token())
        gm.applySetting()
        XCTAssertEqual(try FileManager.default.contentsOfDirectory(atPath: folder.path), [], "the GM was hired and no file was made")
        allowed = true
        gm.applySetting()
        XCTAssertEqual(try FileManager.default.contentsOfDirectory(atPath: folder.path), ["gm-token"])
    }
}
