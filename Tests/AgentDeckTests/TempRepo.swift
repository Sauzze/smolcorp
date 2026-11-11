import XCTest
@testable import AgentDeck

/// Throwaway repos a test makes in its own temp folder must never ask the owner's signing key for a signature
/// (1Password pops up, or a locked one fails the commit). Call `TempRepo.stopSigning` right after `git init`.
/// It writes that repo's OWN config only: never the global one, never a repo the test didn't make.
enum TempRepo {
    /// Sets commit.gpgsign and tag.gpgsign to false in the config of `repo`, which must be a repo whose git folder sits in the temp folder.
    @discardableResult
    static func stopSigning(_ repo: String, in temp: String = NSTemporaryDirectory(), file: StaticString = #filePath, line: UInt = #line) -> Bool {
        guard let gitDir = ownGitDir(repo, in: temp) else {
            XCTFail("TempRepo.stopSigning is for repos in the temp folder only (and a folder that isn't a repo, a worktree or a path through a link to a repo outside it is refused): \(repo)", file: file, line: line)
            return false
        }
        for key in ["commit.gpgsign", "tag.gpgsign"] {
            // --git-dir names the folder git itself says holds the config, so nothing else can be written.
            let done = Tool.run("/usr/bin/git", ["--git-dir=" + gitDir, "config", "--local", key, "false"], errors: true)
            XCTAssertEqual(done?.status, 0, "git config --local \(key): \(done?.output ?? "")", file: file, line: line)
            if done?.status != 0 { return false }
        }
        return true
    }

    /// Where git keeps the config `repo` really uses (a worktree shares its main repo's), resolved on disk,
    /// or nil unless that folder sits strictly inside the temp folder. git is asked with the path as given
    /// (so `link/..` is followed the way the OS does), and a folder that isn't a repo, or only sits inside
    /// another repo, answers with that repo's folder or nothing: both refused.
    static func ownGitDir(_ repo: String, in temp: String = NSTemporaryDirectory()) -> String? {
        guard let asked = Tool.run("/usr/bin/git", ["-C", repo, "rev-parse", "--path-format=absolute", "--git-common-dir"]), asked.status == 0 else { return nil }
        let dir = HireDesk.physical(asked.output.trimmingCharacters(in: .whitespacesAndNewlines))
        return isStrictlyInside(dir, HireDesk.physical(temp)) ? dir : nil
    }

    /// `path` is under `folder`, by whole path parts (`/a/tmp2/x` is not under `/a/tmp`), and is not `folder` itself.
    static func isStrictlyInside(_ path: String, _ folder: String) -> Bool {
        let inside = URL(fileURLWithPath: path).standardizedFileURL.pathComponents
        let outside = URL(fileURLWithPath: folder).standardizedFileURL.pathComponents
        return inside.count > outside.count && Array(inside.prefix(outside.count)) == outside
    }

    /// What `git config --local <key>` says in `repo` (nil if unset).
    static func local(_ key: String, in repo: String) -> String? {
        guard let got = Tool.run("/usr/bin/git", ["-C", repo, "config", "--local", key]), got.status == 0 else { return nil }
        return got.output.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}
