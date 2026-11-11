import XCTest
@testable import AgentDeck

final class TerminalCommandTests: XCTestCase {
    private var folder: URL!
    private var command: URL!
    private var link: URL!

    override func setUpWithError() throws {
        folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        let bin = folder.appendingPathComponent("Smolcorp.app/Contents/Resources/bin", isDirectory: true)
        try FileManager.default.createDirectory(at: bin, withIntermediateDirectories: true)
        command = bin.appendingPathComponent("smolcorp")
        FileManager.default.createFile(atPath: command.path, contents: Data("#!/bin/sh\n".utf8), attributes: [.posixPermissions: 0o755])
        link = folder.appendingPathComponent("home/.local/bin/smolcorp")
    }

    override func tearDownWithError() throws { try? FileManager.default.removeItem(at: folder) }

    func testAddingMakesTheFolderAndTheLinkAndRemovingTakesOnlyTheLink() throws {
        XCTAssertEqual(TerminalCommand.state(link: link), .absent)
        try TerminalCommand.add(link: link, to: command)
        XCTAssertEqual(TerminalCommand.state(link: link), .linked)
        XCTAssertEqual(try FileManager.default.destinationOfSymbolicLink(atPath: link.path), command.path)
        try TerminalCommand.remove(link: link)
        XCTAssertEqual(TerminalCommand.state(link: link), .absent)
        XCTAssertTrue(FileManager.default.fileExists(atPath: command.path), "the app's command stays")
    }

    func testAddingTwiceIsFine() throws {
        try TerminalCommand.add(link: link, to: command)
        try TerminalCommand.add(link: link, to: command)
        XCTAssertEqual(TerminalCommand.state(link: link), .linked)
    }

    func testALinkToWhereTheAppWasIsOursAndIsMendedOrRemoved() throws {
        try FileManager.default.createDirectory(at: link.deletingLastPathComponent(), withIntermediateDirectories: true)
        try FileManager.default.createSymbolicLink(atPath: link.path, withDestinationPath: "/Applications/Gone.app/Contents/Resources/bin/smolcorp")
        XCTAssertEqual(TerminalCommand.state(link: link), .linked)
        try TerminalCommand.add(link: link, to: command)
        XCTAssertEqual(try FileManager.default.destinationOfSymbolicLink(atPath: link.path), command.path)
        try TerminalCommand.remove(link: link)
        XCTAssertEqual(TerminalCommand.state(link: link), .absent)
    }

    func testAFileOfYoursIsNeverReplacedOrRemoved() throws {
        try FileManager.default.createDirectory(at: link.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("mine".utf8).write(to: link)
        XCTAssertEqual(TerminalCommand.state(link: link), .occupied)
        XCTAssertThrowsError(try TerminalCommand.add(link: link, to: command)) { XCTAssertEqual($0 as? TerminalCommand.Problem, .occupied) }
        XCTAssertThrowsError(try TerminalCommand.remove(link: link)) { XCTAssertEqual($0 as? TerminalCommand.Problem, .occupied) }
        XCTAssertEqual(try String(contentsOf: link, encoding: .utf8), "mine")
    }

    func testALinkOfYoursToSomethingElseIsLeftAlone() throws {
        try FileManager.default.createDirectory(at: link.deletingLastPathComponent(), withIntermediateDirectories: true)
        try FileManager.default.createSymbolicLink(atPath: link.path, withDestinationPath: "/usr/local/bin/smolcorp")
        XCTAssertEqual(TerminalCommand.state(link: link), .occupied)
        XCTAssertThrowsError(try TerminalCommand.add(link: link, to: command))
        XCTAssertEqual(try FileManager.default.destinationOfSymbolicLink(atPath: link.path), "/usr/local/bin/smolcorp")
    }

    func testNoCommandInTheAppMeansNoLink() throws {
        let missing = folder.appendingPathComponent("nothing")
        XCTAssertThrowsError(try TerminalCommand.add(link: link, to: missing)) { XCTAssertEqual($0 as? TerminalCommand.Problem, .noCommand) }
        XCTAssertEqual(TerminalCommand.state(link: link), .absent)
        XCTAssertFalse(FileManager.default.fileExists(atPath: link.deletingLastPathComponent().path), "no folder made for nothing")
    }

    func testRemovingWhenThereIsNothingIsFine() throws {
        XCTAssertNoThrow(try TerminalCommand.remove(link: link))
    }

    func testOnPathReadsTheFolderWithTildeAndTrailingSlash() {
        let home = URL(fileURLWithPath: "/Users/someone")
        let bin = home.appendingPathComponent(".local/bin")
        XCTAssertTrue(TerminalCommand.isOnPath(bin, path: "/usr/bin:/Users/someone/.local/bin:/bin", home: home))
        XCTAssertTrue(TerminalCommand.isOnPath(bin, path: "/usr/bin:~/.local/bin/", home: home))
        XCTAssertFalse(TerminalCommand.isOnPath(bin, path: "/usr/bin:/bin", home: home))
        XCTAssertFalse(TerminalCommand.isOnPath(bin, path: nil, home: home))
    }

    func testThePathLineSuitsTheLoginShell() {
        let export = "export PATH=\"$HOME/.local/bin:$PATH\""
        XCTAssertEqual(TerminalCommand.pathLine(shell: "/bin/zsh").command, "echo '\(export)' >> ~/.zprofile")
        XCTAssertEqual(TerminalCommand.pathLine(shell: "/bin/zsh").file, "~/.zprofile")
        XCTAssertEqual(TerminalCommand.pathLine(shell: "/opt/homebrew/bin/bash").command, "echo '\(export)' >> ~/.bash_profile")
        for other in ["/usr/local/bin/fish", "", nil] as [String?] {
            XCTAssertEqual(TerminalCommand.pathLine(shell: other).command, export, other ?? "nil")
            XCTAssertNil(TerminalCommand.pathLine(shell: other).file)
        }
        XCTAssertEqual(TerminalCommand.loginShell(environment: ["SHELL": "/bin/bash"]), "/bin/bash")
    }

    /// The page shows the line (and a Copy button) only inside the note that appears when the folder isn't on PATH.
    func testThePageShowsThePathLineOnlyWhileTheFolderIsOffPath() throws {
        let page = try String(contentsOf: URL(fileURLWithPath: #filePath)
            .appendingPathComponent("../../../Resources/web/setup.html").standardized, encoding: .utf8)
        let note = try XCTUnwrap(page.range(of: "<div class=\"detail\" id=\"terminal-path\" hidden>"))
        let end = try XCTUnwrap(page.range(of: "</div>\n", range: page.range(of: "id=\"terminal-path-copy\"")!.upperBound..<page.endIndex))
        XCTAssertTrue(page[note.lowerBound..<end.upperBound].contains("id=\"terminal-path-line\""))
        XCTAssertTrue(page.contains("$('terminal-path').hidden = t.state !== 'linked' || t.onPath;"))
        XCTAssertTrue(page.contains("t.pathLine"))
        XCTAssertTrue(page.contains("copyText($('terminal-path-line').textContent"))
    }

    func testTheSetupAnswerSaysWhatThePageNeeds() throws {
        let before = TerminalCommand.json(link: link, command: command, path: "/usr/bin", shell: "/bin/zsh")
        XCTAssertEqual(before["pathFile"] as? String, "~/.zprofile")
        XCTAssertEqual(before["pathLine"] as? String, TerminalCommand.pathLine(shell: "/bin/zsh").command)
        XCTAssertEqual(before["state"] as? String, "absent")
        XCTAssertEqual(before["available"] as? Bool, true)
        XCTAssertEqual(before["onPath"] as? Bool, false)
        try TerminalCommand.add(link: link, to: command)
        XCTAssertEqual(TerminalCommand.json(link: link, command: command, path: link.deletingLastPathComponent().path)["onPath"] as? Bool, true)
        XCTAssertEqual(TerminalCommand.json(link: link, command: folder.appendingPathComponent("no"), path: "")["available"] as? Bool, false)
    }

    func testATestCopyNeverTakesALivePort() {
        XCTAssertEqual(TestCopy.livePorts, [7710, 7720, 7730])
        let script = try? String(contentsOf: URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("scripts/test-copy.sh"), encoding: .utf8)
        XCTAssertTrue(script?.contains("7710|7720|7730)") == true)
        XCTAssertTrue(script?.contains("\"PermissionDenied\"") == true)
    }

    /// 07710 or +7710 is still the live port: the script stops before it builds anything.
    func testTheTestCopyScriptRefusesALivePortWrittenAnotherWay() throws {
        let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        for port in ["07710", "+7710", "7720", "abc"] {
            let run = Process()
            run.executableURL = URL(fileURLWithPath: "/bin/bash")
            run.arguments = [repo.appendingPathComponent("scripts/test-copy.sh").path, port]
            run.standardOutput = FileHandle.nullDevice
            run.standardError = FileHandle.nullDevice
            try run.run()
            run.waitUntilExit()
            XCTAssertEqual(run.terminationStatus, 1, port)
        }
    }
}
