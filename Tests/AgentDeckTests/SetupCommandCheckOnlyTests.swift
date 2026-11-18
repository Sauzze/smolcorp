import XCTest
@testable import AgentDeck

/// `--setup` holds only the check-only key (1.26, SK2): the setup key lives in the engine's memory alone, so the
/// terminal reads `selftest-key`, which opens Setup's checklist and nothing else, and it fixes nothing: what the
/// checklist finds is put right in Settings. Read from the source, as nothing here may start the app.
final class SetupCommandCheckOnlyTests: XCTestCase {
    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private func file(_ path: String) throws -> String {
        try String(contentsOf: Self.repo.appendingPathComponent(path), encoding: .utf8)
    }

    func testTheCheckOnlyKeyIsInTheSupportFolderUnderItsName() {
        XCTAssertEqual(Paths.selftestKey.lastPathComponent, "selftest-key")
        XCTAssertEqual(Paths.selftestKey.deletingLastPathComponent().standardizedFileURL, Paths.support.standardizedFileURL)
    }

    func testSetupReadsTheChecklistWithTheCheckOnlyKeyAndFixesNothing() throws {
        let command = try file("Sources/AgentDeck/SetupCommand.swift")
        XCTAssertTrue(command.contains("String(contentsOf: Paths.selftestKey, encoding: .utf8)"))
        XCTAssertTrue(command.contains(#"static let selftestKeyHeader = "x-smolcorp-selftest-key""#))
        XCTAssertTrue(command.contains(#"send(base, "GET", "/setup/selftest")"#))
        // No fix, no setup key, no other route of setup's.
        XCTAssertFalse(command.contains("/setup/fix"))
        XCTAssertFalse(command.contains("x-smolcorp-setup-key"))
        XCTAssertFalse(command.contains("Paths.setupKey"))
        XCTAssertFalse(command.contains(#""POST""#))
        // The engine reads the same header.
        let server = try file("desktop/server.go")
        XCTAssertTrue(server.contains(#"selftestKeyHeader = "X-Smolcorp-Selftest-Key""#))
    }
}
