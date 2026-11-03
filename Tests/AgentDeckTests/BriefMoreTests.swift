import XCTest
@testable import AgentDeck

/// A later part of a brief for a working hire: the file it is added to, and the mail that points at it.
/// Everything is in a folder of its own; nothing is typed anywhere.
final class BriefMoreTests: XCTestCase {
    private var root: URL!
    private let date = Date(timeIntervalSince1970: 1_761_000_000)

    override func setUpWithError() throws {
        root = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("brief-more-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws { try? FileManager.default.removeItem(at: root) }

    private func hire(_ id: String = "abcdef123456", folder: String = "/f", worktree: String = "", role: String = "reviewer",
                      brief: String = "", state: FrontDesk.State = .working) -> FrontDesk.Hire {
        FrontDesk.Hire(id: id, name: "Reviewer", nickname: "Rev", role: role, folder: folder, worktree: worktree, brief: brief, state: state)
    }

    private func body(_ id: String?, _ text: String?) -> Data {
        var json: [String: Any] = [:]
        if let id { json["id"] = id }
        if let text { json["text"] = text }
        return try! JSONSerialization.data(withJSONObject: json)
    }

    private func write(_ text: String, to file: URL) throws {
        try FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        try text.write(to: file, atomically: true, encoding: .utf8)
    }

    private func refusal(_ result: Result<(hire: FrontDesk.Hire, added: BriefMore.Added), BriefMore.Refusal>) -> BriefMore.Refusal? {
        if case .failure(let why) = result { return why }
        return nil
    }

    func testAManagedHiresPartIsAddedToTheEndOfItsOwnBriefDatedAndTheRestIsKept() throws {
        let managed = hire(worktree: "/w/tree")
        let file = root.appendingPathComponent("abcdef123456/BRIEF.md")
        try write("# The job\n\nFirst part.\n", to: file)
        guard case .success(let done) = BriefMore.add(body: body("abcdef123456", " Also check the exporter.\n"), hires: [managed], maxBytes: 1000, now: date, folder: root) else {
            return XCTFail("refused")
        }
        XCTAssertEqual(done.added.file.path, file.path)
        let text = try String(contentsOf: file, encoding: .utf8)
        XCTAssertTrue(text.hasPrefix("# The job\n\nFirst part.\n"))
        XCTAssertTrue(text.hasSuffix("\n\n## New part, \(stamp())\n\nAlso check the exporter.\n"), text)
        // A second part goes after the first.
        _ = BriefMore.add(body: body("abcdef123456", "Then the importer."), hires: [managed], maxBytes: 1000, now: date, folder: root)
        let twice = try String(contentsOf: file, encoding: .utf8)
        XCTAssertTrue(twice.contains("Also check the exporter."))
        XCTAssertTrue(twice.hasSuffix("Then the importer.\n"))
    }

    private func stamp() -> String {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "yyyy-MM-dd HH:mm"
        return f.string(from: date)
    }

    func testEachHireInOneFolderGetsThePartInItsOwnBriefAndAKeptBriefIsFound() throws {
        let project = root.appendingPathComponent("project")
        let first = project.appendingPathComponent("BRIEF.md")
        let second = project.appendingPathComponent("BRIEF-dev.md")
        let kept = root.appendingPathComponent("kept/desk-dev-20261101-0900.md")
        try write("owner's and first hire's", to: first)
        try write("second hire's", to: second)
        try write("unpinned hire's", to: kept)
        let hires = [hire("aaaaaaaaaaaa", folder: project.path, brief: first.path),
                     hire("bbbbbbbbbbbb", folder: project.path, brief: second.path),
                     hire("cccccccccccc", folder: project.path, brief: kept.path)]
        for (id, text) in [("bbbbbbbbbbbb", "for B"), ("cccccccccccc", "for C"), ("aaaaaaaaaaaa", "for A")] {
            guard case .success = BriefMore.add(body: body(id, text), hires: hires, maxBytes: 1000, now: date, folder: root) else { return XCTFail("refused \(id)") }
        }
        let read = { (url: URL) in try String(contentsOf: url, encoding: .utf8) }
        XCTAssertTrue(try read(first).contains("for A") && !read(first).contains("for B") && !read(first).contains("for C"))
        XCTAssertTrue(try read(second).contains("for B") && !read(second).contains("for A"))
        XCTAssertTrue(try read(kept).contains("for C") && !read(kept).contains("for A"))
    }

    func testAHireWithNoRecordedBriefIsNeverGuessedAtInAProjectFolder() throws {
        let project = root.appendingPathComponent("project")
        try write("owner's", to: project.appendingPathComponent("BRIEF.md"))
        try write("other", to: project.appendingPathComponent("BRIEF-dev.md"))
        let result = BriefMore.add(body: body("abcdef123456", "x"), hires: [hire(folder: project.path)], maxBytes: 1000, folder: root)
        guard case .noBrief? = refusal(result) else { return XCTFail("\(result)") }
        XCTAssertEqual(try String(contentsOf: project.appendingPathComponent("BRIEF.md"), encoding: .utf8), "owner's")
        XCTAssertEqual(try String(contentsOf: project.appendingPathComponent("BRIEF-dev.md"), encoding: .utf8), "other")
        // A recorded path that is gone is not made again, nor replaced by a guess.
        let gone = BriefMore.add(body: body("abcdef123456", "x"), hires: [hire(folder: project.path, brief: project.appendingPathComponent("BRIEF-lost.md").path)], maxBytes: 1000, folder: root)
        guard case .noBrief? = refusal(gone) else { return XCTFail("\(gone)") }
        XCTAssertFalse(FileManager.default.fileExists(atPath: project.appendingPathComponent("BRIEF-lost.md").path))
    }

    func testAnOldManagedHireWithNoRecordedPathUsesItsOwnFolder() throws {
        let file = root.appendingPathComponent("abcdef123456/BRIEF-dev.md")
        try write("job", to: file)
        XCTAssertEqual(BriefMore.file(of: hire(worktree: "/w"), hires: root)?.path, file.path)
    }

    func testALinkAtTheRecordedPathIsNotFollowedAndNotOpened() throws {
        let elsewhere = root.appendingPathComponent("elsewhere.md")
        try write("keep", to: elsewhere)
        let link = root.appendingPathComponent("project/BRIEF.md")
        try FileManager.default.createDirectory(at: link.deletingLastPathComponent(), withIntermediateDirectories: true)
        try FileManager.default.createSymbolicLink(at: link, withDestinationURL: elsewhere)
        let result = BriefMore.add(body: body("abcdef123456", "x"), hires: [hire(brief: link.path)], maxBytes: 1000, folder: root)
        guard case .noBrief? = refusal(result) else { return XCTFail("\(result)") }
        // Even where the check passed and the link came after: the open itself refuses a link.
        XCTAssertLessThan(open(link.path, O_WRONLY | O_APPEND | O_NOFOLLOW), 0)
        XCTAssertEqual(try String(contentsOf: elsewhere, encoding: .utf8), "keep")
    }

    func testALinkInPlaceOfTheBriefIsNotFollowed() throws {
        let elsewhere = root.appendingPathComponent("elsewhere.md")
        try write("keep", to: elsewhere)
        let folder = root.appendingPathComponent("abcdef123456")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        try FileManager.default.createSymbolicLink(at: folder.appendingPathComponent("BRIEF.md"), withDestinationURL: elsewhere)
        let result = BriefMore.add(body: body("abcdef123456", "x"), hires: [hire(worktree: "/w")], maxBytes: 1000, folder: root)
        guard case .noBrief? = refusal(result) else { return XCTFail("\(result)") }
        XCTAssertEqual(try String(contentsOf: elsewhere, encoding: .utf8), "keep")
    }

    func testRefusalsWriteNothingAndHaveTheirStatus() throws {
        let file = root.appendingPathComponent("abcdef123456/BRIEF.md")
        try write("first", to: file)
        let managed = hire(worktree: "/w")
        func status(_ data: Data, hires: [FrontDesk.Hire]? = nil, max: Int = 10) -> Int? {
            refusal(BriefMore.add(body: data, hires: hires ?? [managed], maxBytes: max, folder: root))?.response.status
        }
        XCTAssertEqual(status(body(nil, "x")), 400)
        XCTAssertEqual(status(body("abcdef123456", nil)), 400)
        XCTAssertEqual(status(Data("nonsense".utf8)), 400)
        XCTAssertEqual(status(body("abcdef123456", " \n\t ")), 400)
        XCTAssertEqual(status(body("nobody", "x")), 404)
        XCTAssertEqual(status(body("abcdef123456", "x"), hires: [hire(worktree: "/w", state: .fired)]), 409)
        XCTAssertEqual(status(body("abcdef123456", "x"), hires: [hire(worktree: "/w", state: .failed)]), 409)
        XCTAssertEqual(status(body("abcdef123456", "x"), hires: [hire(worktree: "/w", role: "gm")]), 409)
        // Over the limit by bytes, not letters.
        XCTAssertEqual(status(body("abcdef123456", String(repeating: "é", count: 6))), 413)
        XCTAssertNil(status(body("abcdef123456", String(repeating: "é", count: 5))))
        // Only that last one wrote.
        let text = try String(contentsOf: file, encoding: .utf8)
        XCTAssertEqual(text.components(separatedBy: "## New part").count, 2)
    }

    func testTheNoteGoesToTheHireThroughTheMailRouter() throws {
        var given: [(String, String)] = []
        let router = MailRouter(folder: root, limits: { MailRouter.Limits(maxBytes: 100, perHour: 10) }, hires: { [self.hire()] }, mayWrite: { _, _ in false },
                                deliver: { hire, text, commit in
            given.append((hire.id, text))
            return commit() ? .delivered : .trouble("not typed")
        })
        let note = BriefMore.note("BRIEF.md")
        let response = router.send(from: .gm, body: try JSONSerialization.data(withJSONObject: ["to": "abcdef123456", "text": note, "id": "brief-1"]))
        XCTAssertEqual(response.status, 200)
        XCTAssertEqual(given.count, 1)
        XCTAssertEqual(given.first?.0, "abcdef123456")
        XCTAssertTrue(given.first?.1.contains("Your brief has a new part: read BRIEF.md") == true)
        // The part's own text is in no mail.
        XCTAssertFalse(given.first?.1.contains("exporter") == true)
    }

    func testTheRouteIsTheManagersAndTheFixtureAgrees() throws {
        XCTAssertEqual(SetupRoutes.row("POST", "/setup/gm/brief")?.callers, [.gm, .page])
        let json = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: FrontDeskFixtures.url("brief.json"))) as? [String: Any])
        XCTAssertEqual(json["route"] as? String, "POST /setup/gm/brief")
        XCTAssertEqual(json["callers"] as? [String], ["gm", "page"])
        XCTAssertFalse(String(decoding: try Data(contentsOf: FrontDeskFixtures.url("brief.json")), as: UTF8.self).contains("none yet"))
        XCTAssertEqual(json["mail-the-hire-gets"] as? String, BriefMore.note("BRIEF.md"))
        // The request in the fixture is accepted.
        let request = try XCTUnwrap(json["request"] as? [String: Any])
        let file = root.appendingPathComponent("abcdef123456/BRIEF.md")
        try write("job", to: file)
        let result = BriefMore.add(body: try JSONSerialization.data(withJSONObject: request), hires: [hire(worktree: "/w", brief: file.path)], maxBytes: 1000, folder: root)
        XCTAssertNil(refusal(result))
    }
}
