import XCTest
@testable import AgentDeck

/// A hire in T3 Code is a desk in the office (T3D-a, experimental): made from Smolcorp's own registry alone (engine
/// t3, its thread and worktree), never from a tabless hook session, a name or a folder. Its Claude Code's own hook
/// events make no second desk and never land on a terminal's. Nothing types into it; a tap opens its thread in T3
/// Code; Let go is the lane's; and it sits where it sat after a restart, as the registry kept it.
final class T3DeskOfficeTests: XCTestCase {
    private var home: URL!
    private var registry: URL { Paths.support.appendingPathComponent("registry.json") }
    private var keptRegistry: Data?
    private var keptT3 = false
    private var opened: [URL] = []

    override func setUpWithError() throws {
        home = URL(fileURLWithPath: HireDesk.physical(NSTemporaryDirectory())).appendingPathComponent("t3office-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)
        // The test support folder's registry, put back after: each test starts with none.
        keptRegistry = try? Data(contentsOf: registry)
        try? FileManager.default.removeItem(at: registry)
        keptT3 = Config.shared.t3CodeEnabled
        opened = []
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: home)
        if let keptRegistry { try keptRegistry.write(to: registry) } else { try? FileManager.default.removeItem(at: registry) }
        Config.shared.t3CodeEnabled = keptT3
    }

    /// Every desk in the snapshot, the keys' and every floor's.
    private func desks(_ snapshot: [String: Any]) -> [[String: Any]] {
        let keys = (snapshot["keys"] as? [[String: Any]] ?? []).compactMap { $0["session"] as? [String: Any] }
        let floors = (snapshot["floors"] as? [[String: Any]] ?? []).flatMap { $0["desks"] as? [[String: Any]] ?? [] }
        return keys + floors
    }

    func testTheDeskHasNoKeysOrFireInTheOffice() throws {
        let office = try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("Resources/web/office/main.js"), encoding: .utf8)
        // The office already hides a desk's keys and fire where the snapshot says they're off: the T3 desk relies on it.
        XCTAssertTrue(office.contains("if (selectedSession().keys === false) return null;"))
        XCTAssertTrue(office.contains("there.seats[desk].session.fire !== false ? deskButton(desk, 'fire') : null"))
        XCTAssertTrue(office.contains("s.session.t3 && DESKS[desk] && !DESKS[desk].off) items.push([DESKS[desk].y + 5.1, () => drawT3Badge(desk)])"))
    }

    /// FXT3Da N2, N4: the card says the lane's Let go refusal in full, not "Your computer answered 502."; and with
    /// nobody waiting, the desk picked first is one with controls rather than a T3 desk.
    func testTheCardSaysTheLanesRefusalAndAT3DeskIsntPickedFirst() throws {
        let page = try String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("Resources/web/deck.html"), encoding: .utf8)
        XCTAssertTrue(page.contains("async function problemText(res, longest = 200) {"))
        XCTAssertTrue(page.contains("t3Say(res ? await problemText(res, 700) : \"Couldn't reach your computer.\");"))
        XCTAssertTrue(page.contains("keyed.find((s) => s.state === 'asking') || keyed.find((s) => !s.t3) || keyed[0]"))
    }
}
