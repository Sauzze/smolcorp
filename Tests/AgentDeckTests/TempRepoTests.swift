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
        let done = git(repo, ["commit", "-q", "--allow-empty", "-m", "x"])
        XCTAssertNotEqual(done?.status, 0, "the signer is wired in, so this proof means something")
        XCTAssertTrue(done?.output.contains("sign") ?? false, "it failed for the signer, not for another reason: \(done?.output ?? "")")
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

    // MARK: The guard: only a repo whose git folder is really inside the temp folder is written

    private func init_(_ repo: String) {
        XCTAssertEqual(Tool.run("/usr/bin/git", ["init", "-q", "-b", "main", repo], errors: true)?.status, 0)
        XCTAssertEqual(git(repo, ["commit", "-q", "--allow-empty", "--no-gpg-sign", "-m", "x"])?.status, 0)
    }

    /// The pretend temp folder for a guard test: a folder of its own, so a repo "outside" it can sit next to it.
    private func fakeTemp(_ name: String = "tmp") throws -> String {
        let temp = root.appendingPathComponent(name)
        try FileManager.default.createDirectory(at: temp, withIntermediateDirectories: true)
        return temp.path
    }

    /// Refused: nothing written (the guard says no and the outside repo's config is as it was).
    private func assertRefused(_ repo: String, in temp: String, outside: String, _ why: String, file: StaticString = #filePath, line: UInt = #line) {
        let before = TempRepo.local("commit.gpgsign", in: outside)
        XCTAssertNil(TempRepo.ownGitDir(repo, in: temp), why, file: file, line: line)
        XCTExpectFailure("the guard says so") { XCTAssertFalse(TempRepo.stopSigning(repo, in: temp), why, file: file, line: line) }
        XCTAssertEqual(TempRepo.local("commit.gpgsign", in: outside), before, "\(why): the outside repo's config changed", file: file, line: line)
        XCTAssertNil(TempRepo.local("commit.gpgsign", in: outside), "\(why): the outside repo's config changed", file: file, line: line)
    }

    func testStopSigningRefusesTheRepoThisTestFileLivesIn() {
        let real = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().path
        let before = TempRepo.local("commit.gpgsign", in: real)
        XCTExpectFailure("the guard says so") { XCTAssertFalse(TempRepo.stopSigning(real)) }
        XCTAssertEqual(TempRepo.local("commit.gpgsign", in: real), before)
    }

    func testStopSigningRefusesALinkThenDotDotToARepoOutside() throws {
        let temp = try fakeTemp()
        let outside = root.appendingPathComponent("outside").path
        init_(outside)
        try FileManager.default.createDirectory(atPath: outside + "/sub", withIntermediateDirectories: true)
        try FileManager.default.createDirectory(atPath: temp + "/x", withIntermediateDirectories: true)
        try FileManager.default.createSymbolicLink(atPath: temp + "/x/link", withDestinationPath: outside + "/sub")
        // The text says temp/x; the OS goes through the link to outside/sub and then up to outside.
        assertRefused(temp + "/x/link/..", in: temp, outside: outside, "link/..")
        // A link straight to the repo, and a plain `..` out of the temp folder, too.
        try FileManager.default.createSymbolicLink(atPath: temp + "/direct", withDestinationPath: outside)
        assertRefused(temp + "/direct", in: temp, outside: outside, "a link to the repo")
        assertRefused(temp + "/../outside", in: temp, outside: outside, "..")
    }

    func testStopSigningRefusesAWorktreeOfARepoOutside() throws {
        let temp = try fakeTemp()
        let outside = root.appendingPathComponent("outside").path
        init_(outside)
        let tree = temp + "/wt"
        XCTAssertEqual(git(outside, ["worktree", "add", "-q", "-b", "side", tree])?.status, 0)
        // The worktree's own folder is in the temp folder, but its config is the outside repo's.
        assertRefused(tree, in: temp, outside: outside, "a worktree of an outside repo")
    }

    func testStopSigningRefusesAFolderThatIsNotARepo() throws {
        let temp = try fakeTemp()
        let plain = temp + "/plain"
        try FileManager.default.createDirectory(atPath: plain, withIntermediateDirectories: true)
        let outside = root.appendingPathComponent("outside").path
        init_(outside)
        assertRefused(plain, in: temp, outside: outside, "a plain folder, no repo around it")
        // A plain folder in a temp folder that sits inside a repo: git walks up to that repo, which is outside.
        let inner = outside + "/tmp"
        try FileManager.default.createDirectory(atPath: inner + "/plain", withIntermediateDirectories: true)
        assertRefused(inner + "/plain", in: inner, outside: outside, "a plain folder, temp folder inside a repo")
        assertRefused(root.appendingPathComponent("missing").path, in: temp, outside: outside, "a folder that isn't there")
    }

    func testTheTempFolderIsComparedByWholePathParts() {
        XCTAssertFalse(TempRepo.isStrictlyInside("/a/tmp2/repo/.git", "/a/tmp"), "tmp2 is not inside tmp")
        XCTAssertFalse(TempRepo.isStrictlyInside("/a/tmp", "/a/tmp"), "the temp folder itself is not inside")
        XCTAssertFalse(TempRepo.isStrictlyInside("/a", "/a/tmp"))
        XCTAssertTrue(TempRepo.isStrictlyInside("/a/tmp/repo/.git", "/a/tmp"))
        XCTAssertTrue(TempRepo.isStrictlyInside("/a/tmp/repo/.git", "/a/tmp/"))
    }

    func testAWorktreeOfATempRepoIsTakenAndSharesItsConfig() throws {
        let temp = try fakeTemp()
        let repo = temp + "/repo", tree = temp + "/wt"
        init_(repo)
        XCTAssertEqual(git(repo, ["worktree", "add", "-q", "-b", "side", tree])?.status, 0)
        XCTAssertTrue(TempRepo.stopSigning(tree, in: temp))
        XCTAssertEqual(TempRepo.local("commit.gpgsign", in: repo), "false")
    }
}
