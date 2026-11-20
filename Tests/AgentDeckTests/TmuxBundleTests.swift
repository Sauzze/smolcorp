import XCTest
@testable import AgentDeck

/// The tmux that came with the app is the first looked for (Tmux.places), as the Go core looks beside itself.
final class TmuxBundleTests: XCTestCase {
    private var resources: URL!

    override func setUpWithError() throws {
        resources = FileManager.default.temporaryDirectory.appendingPathComponent("tmux-bundle-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: resources.appendingPathComponent("bin"), withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: resources)
    }

    private var bundled: String { resources.appendingPathComponent("bin/tmux").path }

    private func make(_ mode: Int) throws {
        try "#!/bin/sh\nexit 0\n".write(toFile: bundled, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: mode], ofItemAtPath: bundled)
    }

    func testTheBundlesTmuxIsFirstThenTheUsualPlaces() {
        let places = Tmux.places(resources: resources)
        XCTAssertEqual(places.first, bundled)
        XCTAssertEqual(Array(places.dropFirst()), ["/opt/homebrew/bin/tmux", "/usr/local/bin/tmux", "/opt/local/bin/tmux", "/usr/bin/tmux"])
        XCTAssertEqual(Tmux.places(resources: nil).first, "/opt/homebrew/bin/tmux")
    }

    func testTheBundlesTmuxIsPickedWhenItCanBeRun() throws {
        try make(0o755)
        XCTAssertEqual(Tmux.find(resources: resources), bundled)
    }

    func testABundledFileThatCantBeRunIsSkipped() throws {
        try make(0o644)
        XCTAssertNotEqual(Tmux.find(resources: resources), bundled)
    }

    func testABundledLinkIsSkipped() throws {
        let real = resources.appendingPathComponent("real-tmux").path
        try "#!/bin/sh\nexit 0\n".write(toFile: real, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: real)
        try FileManager.default.createSymbolicLink(atPath: bundled, withDestinationPath: real)
        XCTAssertNotEqual(Tmux.find(resources: resources), bundled)
    }

    func testABundledFileOthersCanWriteIsSkipped() throws {
        try make(0o777)
        XCTAssertNotEqual(Tmux.find(resources: resources), bundled)
        try make(0o775) // group-writable too
        XCTAssertNotEqual(Tmux.find(resources: resources), bundled)
    }

    func testABundledFileInAFolderOthersCanWriteIsSkipped() throws {
        try make(0o755)
        let folder = resources.appendingPathComponent("bin").path
        try FileManager.default.setAttributes([.posixPermissions: 0o777], ofItemAtPath: folder)
        XCTAssertNotEqual(Tmux.find(resources: resources), bundled)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: folder)
        XCTAssertEqual(Tmux.find(resources: resources), bundled)
    }

    func testWithNoBundledFileTheUsualPlacesAreUsed() {
        XCTAssertNotEqual(Tmux.find(resources: resources), bundled)
    }
}
