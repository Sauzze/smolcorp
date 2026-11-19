import XCTest
@testable import AgentDeck

/// The Mac's support folder, found as the Go side finds it (appdirs.ResolveIn): Application Support/Smolcorp only
/// once it is a real folder holding Smolcorp's state, otherwise Application Support/AgentDeck. It never makes either.
final class SupportResolverTests: XCTestCase {
    private var base: URL!

    override func setUpWithError() throws {
        base = Paths.testSupport.appendingPathComponent("resolver-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: base, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: base)
    }

    private var old: URL { base.appendingPathComponent("AgentDeck", isDirectory: true) }
    private var new: URL { base.appendingPathComponent("Smolcorp", isDirectory: true) }

    private func folder(_ url: URL, with file: String? = nil) throws {
        try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
        if let file { try Data("{}".utf8).write(to: url.appendingPathComponent(file)) }
    }

    private func listing() -> [String] {
        (FileManager.default.enumerator(atPath: base.path)?.allObjects as? [String] ?? []).sorted()
    }

    private func check(_ want: URL, _ what: String, file: StaticString = #filePath, line: UInt = #line) {
        let before = listing()
        XCTAssertEqual(Paths.resolveSupport(in: base).path, want.path, what, file: file, line: line)
        XCTAssertEqual(listing(), before, "\(what): the resolver changed the folder", file: file, line: line)
    }

    func testNothingThereIsTheOldName() {
        check(old, "nothing there")
    }

    func testAnEmptySmolcorpFolderIsNotUsed() throws {
        try folder(old, with: "registry.json")
        try folder(new)
        check(old, "an empty Smolcorp folder")
    }

    func testASmolcorpFolderWithoutTheTeamIsNotUsed() throws {
        try folder(new, with: "config.json")
        check(old, "a Smolcorp folder with no registry.json")
    }

    func testASmolcorpFolderWithTheTeamIsUsed() throws {
        try folder(old, with: "registry.json")
        try folder(new, with: "registry.json")
        check(new, "a Smolcorp folder with registry.json")
    }

    func testALinkIsNotARealFolder() throws {
        try folder(old, with: "registry.json")
        try FileManager.default.createSymbolicLink(at: new, withDestinationURL: old)
        check(old, "Smolcorp is a link to the old folder")
    }

    func testARegistryThatIsALinkOrAFolderIsNotState() throws {
        try folder(old, with: "registry.json")
        try folder(new)
        try FileManager.default.createSymbolicLink(at: new.appendingPathComponent("registry.json"),
                                                   withDestinationURL: old.appendingPathComponent("registry.json"))
        check(old, "registry.json is a link")
        try FileManager.default.removeItem(at: new.appendingPathComponent("registry.json"))
        try folder(new.appendingPathComponent("registry.json", isDirectory: true))
        check(old, "registry.json is a folder")
    }
}
