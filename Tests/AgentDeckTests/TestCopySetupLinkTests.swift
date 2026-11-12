import XCTest
@testable import AgentDeck

/// A test copy leaves its tester one setup link in its own folder, readable by its owner only;
/// the real app (no test port) never writes one.
final class TestCopySetupLinkTests: XCTestCase {
    private var folder: URL!
    private let link = URL(string: "http://localhost:7750/setup#k=fake-link-key")!

    override func setUp() {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent("test-copy-link-\(UUID().uuidString)", isDirectory: true)
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    }

    override func tearDown() { try? FileManager.default.removeItem(at: folder) }

    func testATestCopyLeavesItsLinkForItsOwnerOnly() throws {
        let file = folder.appendingPathComponent("setup-link")
        FileManager.default.createFile(atPath: file.path, contents: Data("an older link\n".utf8))
        TestCopy.leaveSetupLink(link, into: folder, when: true)
        XCTAssertEqual(try String(contentsOf: file, encoding: .utf8), link.absoluteString + "\n")
        let mode = (try FileManager.default.attributesOfItem(atPath: file.path)[.posixPermissions] as? Int ?? -1) & 0o777
        XCTAssertEqual(mode, 0o600)
    }

    func testTheRealAppNeverWritesOne() {
        TestCopy.leaveSetupLink(link, into: folder, when: false)
        XCTAssertFalse(FileManager.default.fileExists(atPath: folder.appendingPathComponent("setup-link").path))
        // And in this test run, which is no test copy, the defaults write nothing anywhere.
        XCTAssertFalse(TestCopy.isOn)
        TestCopy.leaveSetupLink(link)
    }
}

/// The real app doesn't even make the link: App.swift asks for one only in a test copy.
final class TestCopySetupLinkLaunchTests: XCTestCase {
    func testOnlyATestCopyMintsTheLink() throws {
        let here = URL(fileURLWithPath: #filePath)
        let app = try String(contentsOf: here.deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Sources/AgentDeck/GoShell.swift"), encoding: .utf8)
        // The core makes a link only when asked; the shell leaves it only for a test copy (TestCopy.leaveSetupLink checks).
        XCTAssertTrue(app.contains("let link = TestCopy.isOn ? core.setupLink() : nil"))
        XCTAssertTrue(app.contains("if let setupLink { TestCopy.leaveSetupLink(setupLink) }"))
        XCTAssertEqual(app.components(separatedBy: "TestCopy.leaveSetupLink(").count - 1, 1)
    }
}
