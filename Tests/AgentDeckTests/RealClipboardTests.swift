import AppKit
import XCTest
@testable import AgentDeck

extension XCTestCase {
    /// A pasteboard of the test's own name, let go when the test ends: the one way a test makes a pasteboard
    /// (the source check below holds every other file to it), never the Mac's own clipboard.
    func standInPasteboard() -> NSPasteboard {
        let pasteboard = NSPasteboard(name: .init("smolcorp-test-\(UUID().uuidString)"))
        addTeardownBlock { pasteboard.releaseGlobally() }
        return pasteboard
    }
}

/// Watches the Mac's own clipboard through the whole run by its count alone, nothing on it read (SIERRA,
/// CLIP1): counted before and after each test, and from the first test to the last. Anything copied there
/// during the run ends it as failed, with the tests it happened in.
final class ClipboardWatch: NSObject, XCTestObservation {
    private static let lock = NSLock()
    private static var watch: ClipboardWatch?

    /// The count as the watch started.
    let first = MacClipboard.changeCount
    private let lock = NSLock()
    private var started = false
    private var testsStarted = 0
    private var before: Int?
    private var changed: [String] = []

    /// The run's watch, started once: as XCTest builds the suites, before any test runs (see `RealClipboardTests`).
    @discardableResult
    static func start() -> ClipboardWatch {
        lock.withLock {
            if let watch { return watch }
            let started = ClipboardWatch()
            XCTestObservationCenter.shared.addTestObserver(started)
            watch = started
            return started
        }
    }

    /// Whether it watched from the run's start, before its first test: only then does it cover every test.
    var sawTheRunStart: Bool { lock.withLock { started } }
    /// The tests during which the count changed.
    var changedIn: [String] { lock.withLock { changed } }

    func testBundleWillStart(_ testBundle: Bundle) {
        lock.withLock { started = true }
    }

    /// The suite of the whole run (or of the tests picked) holds suites; a class's suite holds only its tests.
    func testSuiteWillStart(_ testSuite: XCTestSuite) {
        lock.withLock {
            if testsStarted == 0, testSuite.tests.contains(where: { $0 is XCTestSuite }) { started = true }
        }
    }

    func testCaseWillStart(_ testCase: XCTestCase) {
        let count = MacClipboard.changeCount
        lock.withLock {
            testsStarted += 1
            before = count
        }
    }

    func testCaseDidFinish(_ testCase: XCTestCase) {
        let count = MacClipboard.changeCount
        lock.withLock {
            if let before, before != count { changed.append(testCase.name) }
            before = nil
        }
    }

    /// Changed since the first test: the run fails, whatever its tests said.
    func testBundleDidFinish(_ testBundle: Bundle) {
        MacClipboard.pasteboard.releaseGlobally()
        let last = MacClipboard.changeCount
        guard last != first else { return }
        let during = changedIn.isEmpty ? "between tests" : "during " + changedIn.joined(separator: ", ")
        FileHandle.standardError.write(Data("""
            error: the Mac's own clipboard changed during the test run (count \(first) → \(last)), \(during). \
            No test may touch it (CLIP1): give it a stand-in (`standInPasteboard()`, or a stand-in `Clipboard`).

            """.utf8))
        exit(1)
    }
}

/// No test reads or overwrites what the owner copies (SIERRA, CLIP1): the Mac's clipboard is named in one
/// place only (`MacClipboard`), every default under tests is a stand-in, and the watch above covers the run.
final class RealClipboardTests: XCTestCase {
    /// XCTest asks every class for its suite before any test runs: the watch starts then.
    override class var defaultTestSuite: XCTestSuite {
        ClipboardWatch.start()
        return super.defaultTestSuite
    }

    private static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    func testTheMacsClipboardHasntChangedSinceTheRunStarted() {
        let watch = ClipboardWatch.start()
        XCTAssertTrue(watch.sawTheRunStart, "the watch started late: tests before it weren't watched")
        XCTAssertEqual(MacClipboard.changeCount, watch.first, "changed during: \(watch.changedIn)")
    }

    /// Under tests, the defaults are stand-ins: the paste's clipboard refuses everything, and what would be
    /// the Mac's clipboard is a pasteboard of the run's own.
    func testEveryDefaultIsAStandInUnderTests() throws {
        XCTAssertTrue(Paths.underTest)
        let own = "smolcorp-tests-\(ProcessInfo.processInfo.processIdentifier)-"
        XCTAssertTrue(MacClipboard.pasteboard.name.rawValue.hasPrefix(own), MacClipboard.pasteboard.name.rawValue)
        XCTAssertTrue(SystemClipboard().pasteboard === MacClipboard.pasteboard)
        XCTAssertTrue(ClipboardPaste.clipboard is NoClipboard)

        // A paste through the default refuses before anything is pressed.
        XCTAssertEqual(ClipboardPaste.paste("one\ntwo", forward: { XCTFail("not brought forward"); return nil }, ready: { nil },
                                            press: { XCTFail("not pressed"); return .sent }), .unsaved)

        // The viewer's link, copied by default, lands on the run's own pasteboard (checked to be that first).
        try XCTSkipUnless(MacClipboard.pasteboard.name.rawValue.hasPrefix(own))
        let count = MacClipboard.changeCount
        GoShellDelegate.copyForThisMacOnly("https://viewer.example/v/abc#k=fake")
        XCTAssertEqual(MacClipboard.pasteboard.string(forType: .string), "https://viewer.example/v/abc#k=fake")
        XCTAssertEqual(MacClipboard.changeCount, count, "the Mac's own clipboard untouched")
    }

    /// No test names the Mac's clipboard: no general pasteboard, no clipboard command-line tools, and pasteboards
    /// of their own made only through `standInPasteboard()`. In the app, only `MacClipboard` names it.
    func testOnlyTheSeamNamesTheMacsClipboard() throws {
        // Put together, so this file doesn't name what it looks for. A whole word: `.generalCategory` (a
        // character's kind) isn't the clipboard.
        let general = try NSRegularExpression(pattern: "\\.gen" + "eral\\b|gen" + "eralPasteboard")
        func named(in text: Substring) -> Int {
            let text = String(text)
            return general.numberOfMatches(in: text, range: NSRange(text.startIndex..., in: text))
        }
        let made = "NSPasteboard(" + "name:", tools = ["pb" + "copy", "pb" + "paste"]
        func swiftFiles(_ folder: String) throws -> [URL] {
            let root = Self.repo.appendingPathComponent(folder)
            let found = FileManager.default.enumerator(at: root, includingPropertiesForKeys: nil)?.compactMap { $0 as? URL } ?? []
            return found.filter { $0.pathExtension == "swift" }
        }
        let tests = try swiftFiles("Tests")
        XCTAssertTrue(tests.contains { $0.lastPathComponent == "ClipboardPasteTests.swift" }, "the Tests folder was read")
        for file in tests {
            let source = try String(contentsOf: file, encoding: .utf8)
            XCTAssertEqual(named(in: source[...]), 0, "\(file.lastPathComponent) names the general pasteboard")
            for tool in tools { XCTAssertFalse(source.contains(tool), "\(file.lastPathComponent) runs \(tool)") }
            if file.lastPathComponent != "RealClipboardTests.swift" {
                XCTAssertFalse(source.contains(made), "\(file.lastPathComponent): use standInPasteboard()")
            }
        }

        var outside: [String] = []
        var inSeam = 0
        for file in try swiftFiles("Sources") {
            let source = try String(contentsOf: file, encoding: .utf8)
            var all = named(in: source[...])
            if file.lastPathComponent == "ClipboardPaste.swift" {
                let start = try XCTUnwrap(source.range(of: "enum MacClipboard {"))
                let end = try XCTUnwrap(source.range(of: "\n}\n", range: start.upperBound..<source.endIndex))
                inSeam = named(in: source[start.lowerBound..<end.upperBound])
                all -= inSeam
            }
            if all > 0 { outside.append(file.lastPathComponent) }
        }
        XCTAssertEqual(outside, [], "the Mac's clipboard named outside MacClipboard")
        XCTAssertEqual(inSeam, 2, "MacClipboard: the app's pasteboard and the count the watch reads")
    }
}
