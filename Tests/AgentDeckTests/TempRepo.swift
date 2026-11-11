import XCTest
@testable import AgentDeck

/// Throwaway repos a test makes in its own temp folder must never ask the owner's signing key for a signature
/// (1Password pops up, or a locked one fails the commit). Call `TempRepo.stopSigning` right after `git init`.
/// It writes that repo's OWN config only: never the global one, never a repo the test didn't make.
enum TempRepo {
    /// Sets commit.gpgsign and tag.gpgsign to false in the local config of `repo`, which must sit in the temp folder.
    @discardableResult
    static func stopSigning(_ repo: String, file: StaticString = #filePath, line: UInt = #line) -> Bool {
        let temp = HireDesk.physical(NSTemporaryDirectory())
        guard HireDesk.physical(repo).hasPrefix(temp.hasSuffix("/") ? temp : temp + "/") else {
            XCTFail("TempRepo.stopSigning is for repos in the temp folder only: \(repo)", file: file, line: line)
            return false
        }
        for key in ["commit.gpgsign", "tag.gpgsign"] {
            let done = Tool.run("/usr/bin/git", ["-C", repo, "config", "--local", key, "false"], errors: true)
            XCTAssertEqual(done?.status, 0, "git config --local \(key): \(done?.output ?? "")", file: file, line: line)
            if done?.status != 0 { return false }
        }
        return true
    }

    /// What `git config --local <key>` says in `repo` (nil if unset).
    static func local(_ key: String, in repo: String) -> String? {
        guard let got = Tool.run("/usr/bin/git", ["-C", repo, "config", "--local", key]), got.status == 0 else { return nil }
        return got.output.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}
