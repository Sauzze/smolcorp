import XCTest
@testable import AgentDeck

/// A test's throwaway repo commits without asking any signing key, even where the owner's global git config signs
/// every commit through 1Password (here a stand-in global config whose signer always fails, as a locked vault does).
final class TempRepoTests: XCTestCase {
    private var root: URL!
    private var savedGlobal: String?

    override func setUpWithError() throws {
        root = URL(fileURLWithPath: HireDesk.physical(NSTemporaryDirectory())).appendingPathComponent("temprepo-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        let signing = root.appendingPathComponent("signing.gitconfig")
        try "[commit]\n\tgpgsign = true\n[tag]\n\tgpgsign = true\n[gpg]\n\tprogram = /usr/bin/false\n".write(to: signing, atomically: true, encoding: .utf8)
        savedGlobal = ProcessInfo.processInfo.environment["GIT_CONFIG_GLOBAL"]
        setenv("GIT_CONFIG_GLOBAL", signing.path, 1)
    }

    override func tearDownWithError() throws {
        if let savedGlobal { setenv("GIT_CONFIG_GLOBAL", savedGlobal, 1) } else { unsetenv("GIT_CONFIG_GLOBAL") }
        try? FileManager.default.removeItem(at: root)
    }

    private func git(_ repo: String, _ arguments: [String]) -> (status: Int32, output: String)? {
        Tool.run("/usr/bin/git", ["-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.invalid"] + arguments, errors: true)
    }

    func testAStandInGlobalConfigReallyBreaksSignedCommits() {
        let repo = root.appendingPathComponent("plain").path
        XCTAssertEqual(Tool.run("/usr/bin/git", ["init", "-q", "-b", "main", repo], errors: true)?.status, 0)
        XCTAssertNotEqual(git(repo, ["commit", "-q", "--allow-empty", "-m", "x"])?.status, 0, "the signer is wired in, so this proof means something")
    }

    func testStopSigningLetsATempRepoCommitAndSetsOnlyItsOwnConfig() {
        let repo = root.appendingPathComponent("repo").path
        XCTAssertEqual(Tool.run("/usr/bin/git", ["init", "-q", "-b", "main", repo], errors: true)?.status, 0)
        XCTAssertTrue(TempRepo.stopSigning(repo))
        XCTAssertEqual(TempRepo.local("commit.gpgsign", in: repo), "false")
        XCTAssertEqual(TempRepo.local("tag.gpgsign", in: repo), "false")
        XCTAssertEqual(git(repo, ["commit", "-q", "--allow-empty", "-m", "x"])?.status, 0)
        XCTAssertEqual(git(repo, ["tag", "-a", "-m", "t", "v1"])?.status, 0)
        // The global config (the stand-in) still says sign: only this repo was changed.
        XCTAssertEqual(Tool.run("/usr/bin/git", ["config", "--global", "commit.gpgsign"])?.output.trimmingCharacters(in: .whitespacesAndNewlines), "true")
    }

    func testStopSigningRefusesARepoOutsideTheTempFolder() {
        // The repo this test file lives in is a real one: it must be left alone.
        let real = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().path
        let before = TempRepo.local("commit.gpgsign", in: real)
        XCTExpectFailure("the guard says so") { XCTAssertFalse(TempRepo.stopSigning(real)) }
        XCTAssertEqual(TempRepo.local("commit.gpgsign", in: real), before)
    }
}
