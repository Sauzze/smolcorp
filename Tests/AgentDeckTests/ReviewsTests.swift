import XCTest
@testable import AgentDeck

/// The reviews list, and the push that a NEEDS-REVIEW sends. The fixture lines are in the shape the
/// app's own watch (T406) logs: `agent` is "needs-review", the detail "needs-review: <hire>: <file>: <line>".
final class ReviewsTests: XCTestCase {

    // The list and its push are the personal build's (IPHONE_APP, M6): `swift test -Xswiftc -DIPHONE_APP` runs them.
    #if IPHONE_APP
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

    #endif
}
