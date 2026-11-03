import XCTest
@testable import AgentDeck

/// The reviews list, and the push that a NEEDS-REVIEW sends. The fixture lines are in the shape the
/// app's own watch (T406) logs: `agent` is "needs-review", the detail "needs-review: <hire>: <file>: <line>".
final class ReviewsTests: XCTestCase {
    private func line(_ id: Int, hire: String = "Mia", text: String = "NEEDS-REVIEW: the export is ready", agent: String = "needs-review",
                      detail: String? = nil) -> EventLog.Entry {
        EventLog.Entry(id: id, at: 1_761_000_000_000 + Double(id), session: "", folder: "/work/app", agent: agent, kind: .idle,
                       detail: detail ?? "needs-review: \(hire): STATUS.md: \(text)")
    }

    func testAReviewIsTheHireAndTheWordsAfterTheMark() throws {
        let item = try XCTUnwrap(Reviews.item(line(7)))
        XCTAssertEqual(item, Reviews.Item(id: 7, at: 1_761_000_000_007, session: "", hire: "Mia", file: "STATUS.md", text: "the export is ready"))
        XCTAssertEqual(Reviews.item(line(8, text: "- NEEDS-REVIEW:   spaced, with a dash"))?.text, "spaced, with a dash")
        XCTAssertEqual(Reviews.item(line(9, text: "NEEDS-REVIEW: a: colon: inside"))?.text, "a: colon: inside")
    }

    func testOtherLinesAreLeftOut() {
        XCTAssertNil(Reviews.item(line(1, agent: "")), "a watcher line that isn't a review")
        XCTAssertNil(Reviews.item(line(2, detail: "needs-review: no file or line")))
        XCTAssertNil(Reviews.item(line(3, detail: "needs-review: : STATUS.md: NEEDS-REVIEW: nobody")))
    }

    func testTheWordsAreCutToTwoHundred() throws {
        let item = try XCTUnwrap(Reviews.item(line(1, text: "NEEDS-REVIEW: " + String(repeating: "x", count: 500))))
        XCTAssertEqual(item.text.count, Reviews.textLimit)
        XCTAssertEqual(Reviews.words(of: "NEEDS-REVIEW: one\ntwo"), "one two")
    }

    func testTheListIsTheNewestTwentyAndTheAnswerIsWhatThePhoneReads() throws {
        let entries = (1...30).map { line($0) } + [line(31, agent: "")]
        let items = Reviews.recent(entries)
        XCTAssertEqual(items.count, Reviews.most)
        XCTAssertEqual(items.first?.id, 11)
        XCTAssertEqual(items.last?.id, 30)
        let answer = Reviews.answer(Array(items.prefix(1)), now: Date(timeIntervalSince1970: 1_761_000_600))
        let reviews = try XCTUnwrap(answer["reviews"] as? [[String: Any]])
        XCTAssertEqual(reviews.first?["agent"] as? String, "needs-review")
        XCTAssertEqual(reviews.first?["detail"] as? String, "Mia: STATUS.md: the export is ready")
        XCTAssertEqual(reviews.first?["hire"] as? String, "Mia")
        XCTAssertEqual(reviews.first?["at"] as? Int64, 1_761_000_000_011)
    }

    func testThePushSaysWhoAndNeverTheLine() {
        let note = PushNote.review("Mia")
        XCTAssertEqual(note.body, "Mia is ready for review")
        XCTAssertEqual(note.title, "Smolcorp")
        XCTAssertNil(note.session)
        let payload = String(decoding: note.payload, as: UTF8.self)
        XCTAssertFalse(payload.contains("the export"))
        XCTAssertTrue(PushTarget(token: String(repeating: "a", count: 64), finished: true, waiting: false, gm: false).wants(.review))
        XCTAssertFalse(PushTarget(token: String(repeating: "a", count: 64), finished: false, waiting: true, gm: true).wants(.review),
                       "it follows the finished switch")
    }

    func testTheLogTellsTheDeckOfEachNewLine() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("reviews-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: folder) }
        let log = EventLog(folder: folder)
        var told: [Int] = []
        log.onAppend = { told.append($0.id) }
        log.append(line(0))
        log.append(line(0, agent: ""))
        XCTAssertEqual(told, [1, 2])
        log.flush()
    }

    func testTheRouteIsBehindTheApprovalsGate() throws {
        let router = try String(contentsOfFile: URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("Sources/AgentDeck/DeckRouter.swift").path, encoding: .utf8)
        let route = try XCTUnwrap(router.range(of: #"case ("GET", "/reviews"):"#))
        let after = String(router[route.upperBound...].prefix(200))
        XCTAssertTrue(after.contains("DeckRouter.showsApprovals(request, phone: phone)"))
    }
}
