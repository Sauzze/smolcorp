import XCTest
@testable import AgentDeck

/// "Hire your General Manager" starts the GM through the app: its token goes into its own
/// start script's environment and nowhere else, and only one GM works at a time.
final class GMLaunchTests: XCTestCase {
    private let token = String(repeating: "t", count: 43)

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
