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

/// What the watch reads of a clipboard: its count, and the names of the types on it when the count moves,
/// never what's there (SIERRA, CLIP1DIAG). The one place a test names the Mac's clipboard, for those type
/// names alone; a row of the watch gives a stand-in, read the same way.
struct WatchedClipboard {
    let changeCount: () -> Int
    let typeNames: () -> [String]

    static let mac = WatchedClipboard(changeCount: { MacClipboard.changeCount }, typeNames: { names(on: NSPasteboard.general) })

    static func standIn(_ pasteboard: NSPasteboard) -> WatchedClipboard {
        WatchedClipboard(changeCount: { pasteboard.changeCount }, typeNames: { names(on: pasteboard) })
    }

    /// The names of the types on it, as it lists them: nothing it holds is read.
    private static func names(on pasteboard: NSPasteboard) -> [String] { pasteboard.types?.map(\.rawValue) ?? [] }
}

/// Watches the Mac's own clipboard through the whole run by its count, nothing on it read (SIERRA, CLIP1):
/// counted before and after each test, and from the first test to the last. Anything copied there during
/// the run ends it as failed, with the tests it happened in. It also looks every few hundredths of a second,
/// and at each change keeps the names of the types then on it (CLIP1DIAG), which name the likely source.
final class ClipboardWatch: NSObject, XCTestObservation {
    private static let lock = NSLock()
    private static var watch: ClipboardWatch?

    /// One change seen: the count it moved to, the test running then, and the type names then on it.
    struct Change: Equatable {
        let count: Int
        let during: String
        let types: [String]
    }

    /// The changes kept: the first ones, which name the source.
    static let kept = 20

    let clipboard: WatchedClipboard
    /// The count as the watch started.
    let first: Int
    private let lock = NSLock()
    private var started = false
    private var testsStarted = 0
    private var before: Int?
    private var changed: [String] = []
    private var running: String?
    private var seen: Int
    private var changes: [Change] = []
    private var more = 0
    private var timer: DispatchSourceTimer?

    init(_ clipboard: WatchedClipboard = .mac) {
        self.clipboard = clipboard
        first = clipboard.changeCount()
        seen = first
    }

    /// The run's watch, started once: as XCTest builds the suites, before any test runs (see `RealClipboardTests`).
    @discardableResult
    static func start() -> ClipboardWatch {
        lock.withLock {
            if let watch { return watch }
            let started = ClipboardWatch()
            XCTestObservationCenter.shared.addTestObserver(started)
            started.lookEvery(0.02)
            watch = started
            return started
        }
    }

    /// Whether it watched from the run's start, before its first test: only then does it cover every test.
    var sawTheRunStart: Bool { lock.withLock { started } }
    /// The tests during which the count changed.
    var changedIn: [String] { lock.withLock { changed } }
    /// The changes seen, with the type names then on it.
    var seenChanges: [Change] { lock.withLock { changes } }

    /// Looks at the count; moved since the last look, keeps the type names on it now, with the test running,
    /// and at the same count, the latest names on it. An office paste stays on it for `ClipboardPaste.hold`
    /// (0.4 s) at least before what was there comes back.
    func look() {
        lock.withLock {
            let count = clipboard.changeCount()
            guard count != seen else {
                // Seen emptied for new contents, or partway through writing them (a paste's put empties it, then
                // writes its types one after another, all at one count): at the same count it only gets
                // further, so the names on it now replace the ones kept.
                guard let last = changes.last, last.count == count else { return }
                let types = clipboard.typeNames()
                if types != last.types { changes[changes.count - 1] = Change(count: count, during: last.during, types: types) }
                return
            }
            seen = count
            let types = clipboard.typeNames()
            guard changes.count < Self.kept else { more += 1; return }
            changes.append(Change(count: count, during: running ?? "between tests", types: types))
        }
    }

    /// Looks every `seconds`, off the main thread (tests hold it), until the run ends.
    func lookEvery(_ seconds: TimeInterval) {
        let timer = DispatchSource.makeTimerSource(queue: DispatchQueue(label: "smolcorp.tests.clipboard-watch"))
        timer.schedule(deadline: .now(), repeating: seconds)
        timer.setEventHandler { [weak self] in self?.look() }
        lock.withLock { self.timer = timer }
        timer.resume()
    }

    func stopLooking() {
        lock.withLock { timer }?.cancel()
    }

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
        look()
        let count = clipboard.changeCount()
        lock.withLock {
            testsStarted += 1
            before = count
            running = testCase.name
        }
    }

    func testCaseDidFinish(_ testCase: XCTestCase) {
        look()
        let count = clipboard.changeCount()
        lock.withLock {
            if let before, before != count { changed.append(testCase.name) }
            before = nil
            running = nil
        }
    }

    /// Smolcorp's own paste into a desk puts its text marked both transient and concealed (`SystemClipboard.put`).
    static let officePaste = "Smolcorp's own office pasted into a desk on this Mac during the run"
    static let universalClipboard = "Universal Clipboard from another device"

    /// The likely source of one change, from its type names alone.
    static func source(of types: [String]) -> String {
        if types.contains(SystemClipboard.transientType.rawValue), types.contains(SystemClipboard.concealedType.rawValue) {
            return officePaste
        }
        if types.contains("com.apple.is-remote-clipboard") { return universalClipboard }
        return types.isEmpty ? "something emptied it (no types on it)" : "something put " + types.joined(separator: ", ")
    }

    /// Why the run fails, with the count now at `last`; nil only when it never moved. Whatever the source, it
    /// fails: a paste from outside the tests still leaves a run nobody can vouch for.
    func failure(at last: Int) -> String? {
        guard last != first else { return nil }
        let (changedIn, changes, more) = lock.withLock { (changed, self.changes, self.more) }
        let during = changedIn.isEmpty ? "between tests" : "during " + changedIn.joined(separator: ", ")
        let sources = changes.map { Self.source(of: $0.types) }
        let likely = [Self.officePaste, Self.universalClipboard].first(where: sources.contains)
            ?? sources.first ?? "not seen (it changed back too fast to look)"
        var lines = changes.map { "\($0.count) \($0.during): " + ($0.types.isEmpty ? "no types" : $0.types.joined(separator: ", ")) }
        if more > 0 { lines.append("\(more) more") }
        return """
            the Mac's own clipboard changed during the test run (count \(first) → \(last)), \(during). \
            Likely source: \(likely). Seen (type names only, nothing on it read): \
            \(lines.isEmpty ? "nothing" : lines.joined(separator: "; ")). \
            No test may touch it (CLIP1): give it a stand-in (`standInPasteboard()`, or a stand-in `Clipboard`).
            """
    }

    /// Changed since the first test: the run fails, whatever its tests said.
    func testBundleDidFinish(_ testBundle: Bundle) {
        stopLooking()
        MacClipboard.pasteboard.releaseGlobally()
        look()
        guard let failure = failure(at: clipboard.changeCount()) else { return }
        FileHandle.standardError.write(Data("error: \(failure)\n".utf8))
        exit(1)
    }
}

/// No test reads or overwrites what the owner copies (SIERRA, CLIP1): the Mac's clipboard is named in one
/// place in the app (`MacClipboard`) and one in the tests (`WatchedClipboard`, its type names alone), every
/// default under tests is a stand-in, and the watch above covers the run.
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
        watch.look()
        XCTAssertNil(watch.failure(at: MacClipboard.changeCount))
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
    /// of their own made only through `standInPasteboard()`. In the app, only `MacClipboard` names it; in the
    /// tests, only `WatchedClipboard`, for the type names the watch reads.
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
        var watched = 0
        for file in tests {
            let source = try String(contentsOf: file, encoding: .utf8)
            var all = named(in: source[...])
            if file.lastPathComponent == "RealClipboardTests.swift" {
                let seam = try Self.watchedClipboard(source)
                watched = named(in: seam)
                all -= watched
            }
            XCTAssertEqual(all, 0, "\(file.lastPathComponent) names the general pasteboard")
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
        XCTAssertEqual(watched, 1, "WatchedClipboard: the type names the watch reads")
    }

    /// The source of `WatchedClipboard`, the one place the tests name the Mac's clipboard.
    private static func watchedClipboard(_ source: String) throws -> Substring {
        let start = try XCTUnwrap(source.range(of: "struct Watched" + "Clipboard {"))
        let end = try XCTUnwrap(source.range(of: "\n}\n", range: start.upperBound..<source.endIndex))
        return source[start.lowerBound..<end.upperBound]
    }

    /// The watch reads a clipboard's count and type names, nothing it holds (CLIP1DIAG): in `WatchedClipboard`
    /// the only things asked of a pasteboard are its count and its types (and, of the Mac's class, which one),
    /// and the watch asks a pasteboard nothing itself but to let go of the run's own at the end.
    func testTheWatchReadsOnlyTheCountAndTheTypeNames() throws {
        let source = try String(contentsOf: Self.repo.appendingPathComponent("Tests/AgentDeckTests/RealClipboardTests.swift"), encoding: .utf8)
        let asked = try NSRegularExpression(pattern: "pasteboard\\??\\.(\\w+)", options: .caseInsensitive)
        func members(_ text: String) -> Set<String> {
            Set(asked.matches(in: text, range: NSRange(text.startIndex..., in: text)).compactMap {
                Range($0.range(at: 1), in: text).map { String(text[$0]) }
            })
        }
        let reads = ["forType", "pasteboardItems", "readObjects", "propertyList", "string(", "data(", "canReadItem", "availableType"]

        let seam = String(try Self.watchedClipboard(source))
        XCTAssertEqual(members(seam), ["changeCount", "types", "general"])
        for read in reads { XCTAssertFalse(seam.contains(read), "WatchedClipboard reads \(read)") }

        let start = try XCTUnwrap(source.range(of: "final class Clipboard" + "Watch"))
        let end = try XCTUnwrap(source.range(of: "\n}\n", range: start.upperBound..<source.endIndex))
        let watch = String(source[start.lowerBound..<end.upperBound])
        XCTAssertEqual(members(watch), ["releaseGlobally"])
        for read in reads { XCTAssertFalse(watch.contains(read), "ClipboardWatch reads \(read)") }
    }

    // MARK: What the watch says of a change, on stand-in pasteboards (never the Mac's own)

    private static let markers = ["org.nspasteboard.TransientType", "org.nspasteboard.ConcealedType"]

    /// Smolcorp's own office pasting into a desk, as `SystemClipboard` does it: the text put marked transient
    /// and concealed, then what was there put back (the Studio's 25 → 27). Named so, still a failure, and the
    /// text never read by the watch.
    func testTheWatchNamesSmolcorpsOwnOfficePaste() throws {
        let pasteboard = standInPasteboard()
        let office = SystemClipboard(pasteboard)
        let watch = ClipboardWatch(.standIn(pasteboard))
        watch.testCaseWillStart(self)
        var reads = 0
        let put = try XCTUnwrap(office.put("fake-note-7f3", read: { reads += 1 }))
        watch.look()
        XCTAssertEqual(reads, 0, "the watch read the text")
        XCTAssertEqual(pasteboard.string(forType: .string), "fake-note-7f3")
        XCTAssertEqual(reads, 1, "a read of the text is seen (control)")
        office.restore(.items([[ClipboardEntry(type: NSPasteboard.PasteboardType.string.rawValue, data: Data("fake-yours-7f3".utf8))]]))
        watch.look()
        watch.testCaseDidFinish(self)

        let last = pasteboard.changeCount
        XCTAssertEqual(watch.seenChanges.map(\.count), [put, last])
        XCTAssertEqual(watch.seenChanges.map(\.during), [name, name])
        XCTAssertTrue(Set(watch.seenChanges[0].types).isSuperset(of: Self.markers), "\(watch.seenChanges)")
        XCTAssertTrue(watch.seenChanges[1].types.contains(NSPasteboard.PasteboardType.string.rawValue), "\(watch.seenChanges)")
        XCTAssertTrue(Set(watch.seenChanges[1].types).isDisjoint(with: Self.markers), "\(watch.seenChanges)")
        let failure = try XCTUnwrap(watch.failure(at: last), "it still fails")
        XCTAssertTrue(failure.contains("Likely source: Smolcorp's own office pasted into a desk on this Mac during the run."), failure)
        XCTAssertTrue(failure.contains("(count \(watch.first) → \(last)), during \(name)."), failure)
        XCTAssertTrue(failure.contains("org.nspasteboard.ConcealedType"), failure)
        XCTAssertFalse(failure.contains("fake-"), "nothing on it in the message: \(failure)")
        XCTAssertEqual(reads, 1, "nothing read after the control")
    }

    /// Seen anywhere in the run, the office's paste is the likely source, ahead of Universal Clipboard and of
    /// anything else, whichever came first.
    func testTheOfficesPasteIsTheLikelySourceWhereverItCame() throws {
        let pasteboard = standInPasteboard()
        let watch = ClipboardWatch(.standIn(pasteboard))
        watch.testCaseWillStart(self)
        pasteboard.declareTypes([.init("com.example.fake-type")], owner: nil)
        watch.look()
        pasteboard.declareTypes([.string, .init("com.apple.is-remote-clipboard")], owner: nil)
        watch.look()
        XCTAssertNotNil(SystemClipboard(pasteboard).put("fake-note-7f3", read: {}))
        watch.testCaseDidFinish(self)

        XCTAssertEqual(watch.seenChanges.count, 3, "\(watch.seenChanges)")
        let failure = try XCTUnwrap(watch.failure(at: pasteboard.changeCount))
        XCTAssertTrue(failure.contains("Likely source: " + ClipboardWatch.officePaste + "."), failure)
    }

    /// A copy from another device over Universal Clipboard: named so, still a failure.
    func testTheWatchNamesUniversalClipboard() throws {
        let pasteboard = standInPasteboard()
        let watch = ClipboardWatch(.standIn(pasteboard))
        watch.testCaseWillStart(self)
        pasteboard.declareTypes([.string, .init("com.apple.is-remote-clipboard")], owner: nil)
        watch.testCaseDidFinish(self)

        let failure = try XCTUnwrap(watch.failure(at: pasteboard.changeCount), "it still fails")
        XCTAssertTrue(failure.contains("Likely source: Universal Clipboard from another device."), failure)
        XCTAssertFalse(failure.contains(ClipboardWatch.officePaste), failure)
    }

    /// Anything else: the type names alone, still a failure. Both markers make the office's paste, not one.
    func testTheWatchGivesTheTypeNamesOfAnythingElse() throws {
        let pasteboard = standInPasteboard()
        let watch = ClipboardWatch(.standIn(pasteboard))
        watch.testCaseWillStart(self)
        pasteboard.declareTypes([.init("com.example.fake-type"), .init(Self.markers[1])], owner: nil)
        watch.look()
        pasteboard.clearContents()
        watch.testCaseDidFinish(self)

        let failure = try XCTUnwrap(watch.failure(at: pasteboard.changeCount), "it still fails")
        XCTAssertTrue(failure.contains("Likely source: something put "), failure)
        XCTAssertTrue(failure.contains("com.example.fake-type") && failure.contains(Self.markers[1]), failure)
        XCTAssertTrue(failure.contains(": no types"), failure)
        XCTAssertEqual(ClipboardWatch.source(of: []), "something emptied it (no types on it)")
        for named in [ClipboardWatch.officePaste, ClipboardWatch.universalClipboard] { XCTAssertFalse(failure.contains(named), failure) }
    }

    /// A change before any test is put to no test; one the watch never saw still fails, saying so; and the
    /// count back where it started is the one way it doesn't fail.
    func testTheWatchFailsWheneverTheCountMoved() throws {
        let pasteboard = standInPasteboard()
        let watch = ClipboardWatch(.standIn(pasteboard))
        XCTAssertNil(watch.failure(at: watch.first), "unchanged: no failure")
        XCTAssertNotNil(watch.failure(at: watch.first + 1), "not seen: still a failure")
        XCTAssertTrue(watch.failure(at: watch.first + 1)?.contains("not seen") == true)

        pasteboard.declareTypes([.init("com.example.fake-type")], owner: nil)
        watch.testCaseWillStart(self)
        watch.testCaseDidFinish(self)
        XCTAssertEqual(watch.seenChanges.map(\.during), ["between tests"])
        let failure = try XCTUnwrap(watch.failure(at: pasteboard.changeCount))
        XCTAssertTrue(failure.contains("), between tests."), failure)
        XCTAssertEqual(watch.changedIn, [])
    }

    /// Looking on its own, off the main thread, with nobody calling `look()`, the watch sees each move of the
    /// count during a test: the office's put and its put-back, held apart as the office holds a paste
    /// (`ClipboardPaste.hold`). Which type names a look finds depends on where it lands in the put's write, so
    /// they're the scripted row's below, not this one's.
    func testTheWatchLooksOnItsOwnBetweenTheStartAndTheEndOfATest() throws {
        let pasteboard = standInPasteboard()
        let office = SystemClipboard(pasteboard)
        let watch = ClipboardWatch(.standIn(pasteboard))
        defer { watch.stopLooking() }
        watch.testCaseWillStart(self)
        watch.lookEvery(0.02)
        func waitUntil(_ done: () -> Bool) {
            let until = Date().addingTimeInterval(5)
            while !done(), Date() < until { Thread.sleep(forTimeInterval: 0.01) }
        }
        let put = try XCTUnwrap(office.put("fake-note-7f3", read: {}))
        // Held until the watch has seen the count move, so the put-back is a move of its own.
        waitUntil { watch.seenChanges.count >= 1 }
        office.restore(.items([]))
        waitUntil { watch.seenChanges.count >= 2 }
        watch.stopLooking()

        XCTAssertEqual(watch.seenChanges.map(\.count), [put, pasteboard.changeCount], "\(watch.seenChanges)")
        XCTAssertEqual(watch.seenChanges.map(\.during), [name, name])
    }

    /// A put writes its types one after another at one count (the Studio's 19:50 run caught it with the text's
    /// types and no markers yet): the watch keeps the finished list, not the first it saw. Scripted, no timer:
    /// each step is what the clipboard holds next, and the watch looks once after each.
    func testTheWatchKeepsTheFinishedTypesAtACount() throws {
        let text = [NSPasteboard.PasteboardType.string.rawValue, "NSStringPboardType"]
        let steps: [(count: Int, types: [String])] = [
            (0, ["com.example.fake-type"]),
            (1, []),
            (1, text),
            (1, text + Self.markers),
            (2, []),
        ]
        var step = 0
        let watch = ClipboardWatch(WatchedClipboard(changeCount: { steps[step].count }, typeNames: { steps[step].types }))
        watch.testCaseWillStart(self)
        while step < steps.count - 1 {
            step += 1
            watch.look()
        }
        watch.testCaseDidFinish(self)

        XCTAssertEqual(watch.seenChanges.map(\.count), [1, 2], "\(watch.seenChanges)")
        XCTAssertTrue(Set(watch.seenChanges.first?.types ?? []).isSuperset(of: Self.markers), "\(watch.seenChanges)")
        XCTAssertEqual(watch.seenChanges.last?.types, [])
        let failure = try XCTUnwrap(watch.failure(at: 2))
        XCTAssertTrue(failure.contains("Likely source: " + ClipboardWatch.officePaste + "."), failure)
    }
}
