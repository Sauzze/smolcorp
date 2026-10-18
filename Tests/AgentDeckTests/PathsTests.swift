import XCTest
@testable import AgentDeck

final class PathsTests: XCTestCase {
    func testAPrivateWriteReplacesTheFileWholeAndKeepsItYours() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let file = folder.appendingPathComponent("phones.json")
        XCTAssertTrue(Paths.writePrivately(Data("old".utf8), to: file))
        XCTAssertTrue(Paths.writePrivately(Data("new".utf8), to: file))
        XCTAssertEqual(try String(contentsOf: file, encoding: .utf8), "new")
        let mode = try FileManager.default.attributesOfItem(atPath: file.path)[.posixPermissions] as? Int
        XCTAssertEqual(mode, 0o600)
        XCTAssertEqual(try FileManager.default.contentsOfDirectory(atPath: folder.path), ["phones.json"], "nothing left beside it")
    }
}
