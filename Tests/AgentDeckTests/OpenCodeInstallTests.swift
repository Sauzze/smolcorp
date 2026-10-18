import XCTest
@testable import AgentDeck

/// Which OpenCode is installed, for the plugin it can load.
final class OpenCodeInstallTests: XCTestCase {
    func testTheVersionIsReadAsOpenCodePrintsIt() {
        XCTAssertEqual(OpenCodeInstall.majorVersion(in: "opencode v2.0.32\n"), 2)
        XCTAssertEqual(OpenCodeInstall.majorVersion(in: "1.14.0"), 1)
        XCTAssertEqual(OpenCodeInstall.majorVersion(in: "v10.2.1"), 10)
        XCTAssertNil(OpenCodeInstall.majorVersion(in: ""))
        XCTAssertNil(OpenCodeInstall.majorVersion(in: "version 2"))
    }

    /// A stand-in opencode on the PATH, saying the version it's given, as an
    /// update changes the program and its version.
    func testOpenCodeIsFoundAndAskedItsVersionOnlyWhenItChanges() throws {
        let bin = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: bin, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: bin) }
        let program = bin.appendingPathComponent("opencode"), version = bin.appendingPathComponent("version")
        try "#!/bin/sh\ncat \"\(version.path)\"\n".write(to: program, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: program.path)
        let path = ProcessInfo.processInfo.environment["PATH"] ?? ""
        setenv("PATH", "\(bin.path):\(path)", 1)
        defer { setenv("PATH", path, 1) }

        try "1.14.0\n".write(to: version, atomically: true, encoding: .utf8)
        XCTAssertEqual(OpenCodeInstall.program()?.path, program.resolvingSymlinksInPath().path)
        XCTAssertEqual(OpenCodeInstall.major(), 1)
        let before = OpenCodeInstall.stamp()
        try "opencode v2.0.32\n".write(to: version, atomically: true, encoding: .utf8)
        XCTAssertEqual(OpenCodeInstall.stamp(), before, "only the program changing is an update")
        try FileManager.default.setAttributes([.modificationDate: Date().addingTimeInterval(60)], ofItemAtPath: program.path)
        XCTAssertNotEqual(OpenCodeInstall.stamp(), before)
        XCTAssertEqual(OpenCodeInstall.major(), 2)
    }
}
