import XCTest
@testable import AgentDeck

/// T745: the GM names one person in the office, with its own token, and nothing else changes.
final class GMNicknameTests: XCTestCase {
    private let places = [GMNickname.Place(project: "/work/app", folder: "/work/app/sub")]
    private let pins: [String?] = [nil, "/work/pinned"]

    private func apply(_ body: String, _ layout: inout Layout) -> Response {
        GMNickname.apply(body: Data(body.utf8), layout: &layout, places: places, pins: pins)
    }

    private func layoutWithTeam() -> Layout {
        var layout = Layout.standard
        var other = TeamMember()
        other.outfit = "suit"
        other.name = "Boss"
        layout.room.team["other"] = other
        return layout
    }

    func testItNamesOnlyThatProjectsMember() throws {
        var layout = layoutWithTeam()
        let before = layout
        let answer = apply(#"{"folder":"/work/app","name":"  Ada  "}"#, &layout)
        XCTAssertEqual(answer.status, 200)
        XCTAssertEqual(layout.room.team["app"]?.name, "Ada", "trimmed")
        XCTAssertEqual(layout.room.team["other"], before.room.team["other"], "another person is untouched")
        var expected = before
        expected.room.team["app"] = { var m = TeamMember(); m.name = "Ada"; return m }()
        XCTAssertEqual(layout, expected, "nothing else in the layout moves")
    }

    func testItKeepsTheLooksAndAnEmptyNameTakesTheNicknameAway() {
        var layout = layoutWithTeam()
        var dressed = TeamMember()
        dressed.hair = "blond"
        dressed.name = "Ada"
        layout.room.team["app"] = dressed
        XCTAssertEqual(apply(#"{"folder":"/work/app","name":""}"#, &layout).status, 200)
        XCTAssertEqual(layout.room.team["app"]?.hair, "blond")
        XCTAssertEqual(layout.room.team["app"]?.name, "")
        // A member with nothing set at all is dropped, as the page does.
        var plain = layoutWithTeam()
        _ = apply(#"{"folder":"/work/app","name":"Ada"}"#, &plain)
        XCTAssertEqual(apply(#"{"folder":"/work/app","name":""}"#, &plain).status, 200)
        XCTAssertNil(plain.room.team["app"])
    }

    func testTheFolderIsAProjectOrWhereASessionIsOrAPinAndNothingElse() {
        var layout = layoutWithTeam()
        XCTAssertEqual(apply(#"{"folder":"/work/app/sub","name":"Ada"}"#, &layout).status, 200)
        XCTAssertEqual(layout.room.team["app"]?.name, "Ada", "named by its project, not the subfolder")
        XCTAssertEqual(apply(#"{"folder":"/work/pinned","name":"Pin"}"#, &layout).status, 200)
        XCTAssertEqual(layout.room.team["pinned"]?.name, "Pin")
        let before = layout
        let refused = apply(#"{"folder":"/work/nobody","name":"X"}"#, &layout)
        XCTAssertEqual(refused.status, 404)
        XCTAssertTrue(String(decoding: refused.body, as: UTF8.self).contains("Nobody works in that folder"))
        XCTAssertEqual(layout, before)
    }

    func testTheLimitsAndBadRequests() {
        var layout = layoutWithTeam()
        let before = layout
        XCTAssertEqual(apply(#"{"folder":"/work/app","name":"\#(String(repeating: "x", count: 16))"}"#, &layout).status, 200)
        layout = before
        XCTAssertEqual(apply(#"{"folder":"/work/app","name":"\#(String(repeating: "x", count: 17))"}"#, &layout).status, 400)
        XCTAssertEqual(apply(#"{"folder":"/work/app","name":"two\nlines"}"#, &layout).status, 400)
        XCTAssertEqual(apply(#"{"folder":"/work/app"}"#, &layout).status, 400, "no name")
        XCTAssertEqual(apply(#"{"folder":"","name":"A"}"#, &layout).status, 400, "no folder")
        XCTAssertEqual(apply(#"{"folder":"/work/app","name":5}"#, &layout).status, 400)
        XCTAssertEqual(apply("nonsense", &layout).status, 400)
        XCTAssertEqual(layout, before, "a refusal changes nothing")
    }

    func testTheOfficeRemembersAtMostFiftyPeople() {
        var layout = Layout.standard
        for i in 0..<Room.teamLimit { var m = TeamMember(); m.name = "n\(i)"; layout.room.team["p\(i)"] = m }
        XCTAssertEqual(apply(#"{"folder":"/work/app","name":"Ada"}"#, &layout).status, 409)
        XCTAssertNil(layout.room.team["app"])
    }

    func testOnlyTheGMAndThePageMayNameAnyone() throws {
        let row = try XCTUnwrap(SetupRoutes.row("POST", "/setup/gm/nickname"))
        XCTAssertEqual(row.callers, [.gm, .page], "a tool (the setup-key file) and a hire with a send key are refused")
    }
}
