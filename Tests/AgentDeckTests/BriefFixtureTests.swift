import CryptoKit
import XCTest
@testable import AgentDeck

/// `Tests/fixtures/briefs/briefs.json` is what the Mac's own brief code made of a fixed set of inputs: hires' briefs (their text,
/// files and first message), setup's previews, picked files, a brief's later parts and the guard's state. The Go app's test
/// (desktop/briefs_fixture_test.go) makes the same from the same inputs and must match byte for byte. Paths are written with
/// `~` and each side expands them with its own home; dates are shown in UTC. Since the Go core alone makes briefs, nothing
/// writes this file any more: it's kept as it was, and only read.
///
/// One difference is known and kept: a path that went through `URL` came out decomposed there (NFD: "e" then the accent),
/// while the folder as given, and everything on the Go side, stays precomposed (NFC). A hire marked `"normalized": "NFC"`
/// holds the Mac's brief with that undone (`made`), and as it really was (`madeAsIs`); see docs/GO-PARITY.md.
final class BriefFixtureTests: XCTestCase {
    private let fixture = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
        .appendingPathComponent("fixtures/briefs/briefs.json")
    private var folder: URL!
    private var savedTZ: String?

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        // Dates in UTC, whatever this Mac's time zone: both sides show the same day and minute.
        savedTZ = ProcessInfo.processInfo.environment["TZ"]
        setenv("TZ", "UTC", 1)
        tzset()
        NSTimeZone.resetSystemTimeZone()
    }

    override func tearDownWithError() throws {
        if let savedTZ { setenv("TZ", savedTZ, 1) } else { unsetenv("TZ") }
        tzset()
        NSTimeZone.resetSystemTimeZone()
        try? FileManager.default.removeItem(at: folder)
    }

    private func json(_ role: Role) throws -> Any { try JSONSerialization.jsonObject(with: JSONEncoder().encode(role)) }
    private func shipped(_ id: String) throws -> Role { try XCTUnwrap(Role.shipped.first { $0.id == id }, id) }

    /// The fixture's paths are shown with `~`, so they hold nothing of the Mac that wrote them.
    func testTheFixtureNamesNoFolderOfThisMac() throws {
        let text = try String(contentsOf: fixture, encoding: .utf8)
        XCTAssertFalse(text.contains(NSHomeDirectory()))
        XCTAssertFalse(text.contains(folder.deletingLastPathComponent().path))
    }
}
