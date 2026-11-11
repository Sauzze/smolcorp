import AppKit
import XCTest
@testable import AgentDeck

/// Several lines pasted through the clipboard in Terminal.app, iTerm2 and Ghostty (I34, the owner's Q1 b),
/// with a stand-in clipboard: no test touches the real one, and no real terminal is told anything.
final class ClipboardPasteTests: XCTestCase {
    /// A clipboard that keeps what's on it and says what was done to it, in order.
    final class StandInClipboard: Clipboard {
        var items: [[ClipboardEntry]]
        private(set) var changeCount = 1
        private(set) var steps: [String] = []
        /// Something else copied while the paste was on its way, as another app would.
        var copiedMeanwhile: (() -> Void)?

        init(_ items: [[ClipboardEntry]]) { self.items = items }

        func save() -> [[ClipboardEntry]] {
            steps.append("save")
            return items
        }

        func put(_ text: String) -> Int? {
            steps.append("put")
            items = [[ClipboardEntry(type: NSPasteboard.PasteboardType.string.rawValue, data: Data(text.utf8))]]
            changeCount += 1
            return changeCount
        }

        func restore(_ saved: [[ClipboardEntry]]) {
            steps.append("restore")
            items = saved
            changeCount += 1
        }

        func copy(_ text: String) {
            items = [[ClipboardEntry(type: "public.utf8-plain-text", data: Data(text.utf8))]]
            changeCount += 1
        }

        /// The text a paste now would take.
        var text: String? {
            items.first?.first { $0.type == NSPasteboard.PasteboardType.string.rawValue }.map { String(decoding: $0.data, as: UTF8.self) }
        }
    }

    /// What you had copied: two items, text with its rich form, and a picture: every type comes back.
    private let yours: [[ClipboardEntry]] = [
        [ClipboardEntry(type: "public.utf8-plain-text", data: Data("mot de passe ?".utf8)),
         ClipboardEntry(type: "public.rtf", data: Data(#"{\rtf1 mot}"#.utf8))],
        [ClipboardEntry(type: "public.png", data: Data([0x89, 0x50, 0x4E, 0x47, 0, 1, 2, 0xFF]))],
    ]

    private var clipboard: StandInClipboard!
    private var savedClipboard: Clipboard!
    private var savedWait: TimeInterval = 0

    override func setUp() {
        clipboard = StandInClipboard(yours)
        savedClipboard = ClipboardPaste.clipboard
        savedWait = ClipboardPaste.readWait
        ClipboardPaste.clipboard = clipboard
        ClipboardPaste.readWait = 0.05
    }

    override func tearDown() {
        ClipboardPaste.clipboard = savedClipboard
        ClipboardPaste.readWait = savedWait
    }

    private let text = "Deux lignes :\n    « ça marche » 👩‍💻 $(touch x) fin;\n\tet un tab"

    /// The text goes byte for byte, line feeds and all, ⌘V once (no Return of ours, mid-text or after),
    /// and what you had copied comes back exactly, every item and type.
    func testThePasteTakesTheTextAndTheClipboardComesBack() {
        var steps: [String] = []
        var pasted: String?
        let result = ClipboardPaste.paste(text, ready: { steps.append("ready"); return true },
                                          forward: { steps.append("forward"); return true },
                                          press: { pasted = self.clipboard.text; steps.append("⌘V"); return true })
        XCTAssertEqual(result, .pasted)
        XCTAssertEqual(pasted.map { Data($0.utf8) }, Data(text.utf8), "byte for byte")
        XCTAssertEqual(steps, ["forward", "ready", "⌘V"], "asked right before the paste, once in front")
        XCTAssertEqual(clipboard.steps, ["save", "put", "restore"])
        XCTAssertEqual(clipboard.items, yours, "every item and type back")
    }

    /// A paste that didn't go, or whose terminal stopped answering, still gives the clipboard back.
    func testTheClipboardComesBackWhenThePasteFailsOrTimesOut() {
        XCTAssertEqual(ClipboardPaste.paste(text, ready: { true }, forward: { true }, press: { false }), .failed)
        XCTAssertEqual(clipboard.items, yours)
        XCTAssertEqual(clipboard.steps, ["save", "put", "restore"])
    }

    /// Ghostty's own paste, through a stand-in osascript that never answers: ended once its time is up,
    /// the paste fails as having timed out, and the clipboard comes back all the same.
    func testTheClipboardComesBackWhenTheTerminalStopsAnswering() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let standIn = folder.appendingPathComponent("osascript"), asked = folder.appendingPathComponent("asked")
        try """
            #!/bin/sh
            case "$2" in
              *paste_from_clipboard*) echo paste >> '\(asked.path)'; exec /bin/sleep 30 ;;
              *"first terminal whose tty"*) echo ghostty-standin-1 ;;
              *'return "open"'*) echo open ;;
              *) echo other >> '\(asked.path)' ;;
            esac
            """.write(to: standIn, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: standIn.path)
        let saved = Terminal.osascript
        Terminal.osascript = standIn.path
        let tty = "/dev/ttys977"
        defer {
            Terminal.osascript = saved
            Ghostty.forget(tty: tty)
        }
        Ghostty.learn(tty: tty)
        XCTAssertNotNil(Ghostty.id(on: tty))
        let tab = try XCTUnwrap(TabAddress("ghostty:\(tty)"))
        let (result, timedOut, partWay) = Terminal.noticingTimeouts { Ghostty().insertLines(text, in: tab, ready: { true }) }
        XCTAssertEqual(result, .failed)
        XCTAssertTrue(timedOut)
        XCTAssertTrue(partWay, "the paste may have gone: the phone is told to look before trying again")
        XCTAssertEqual(try String(contentsOf: asked, encoding: .utf8), "paste\n", "only Ghostty's paste, nothing typed as keys")
        XCTAssertEqual(clipboard.items, yours, "back, though Ghostty never answered")
        XCTAssertEqual(Ghostty().linesWay, .clipboard(inFront: false))
    }

    /// Asking something by the time the tab is in front, or a tab that couldn't be brought forward: the
    /// clipboard isn't touched and nothing is pressed.
    func testNothingIsTouchedWhenTheDeskAsksOrTheTabIsntInFront() {
        var pressed = 0
        XCTAssertEqual(ClipboardPaste.paste(text, ready: { false }, forward: { true }, press: { pressed += 1; return true }), .asking)
        XCTAssertEqual(ClipboardPaste.paste(text, ready: { true }, forward: { false }, press: { pressed += 1; return true }), .failed)
        // An Escape could end the terminal's bracket early, and a return press Enter.
        for text in ["a\u{1B}[201~\nb", "a\rb\nc"] {
            XCTAssertEqual(ClipboardPaste.paste(text, ready: { true }, forward: { true }, press: { pressed += 1; return true }), .failed)
        }
        XCTAssertEqual(pressed, 0)
        XCTAssertEqual(clipboard.steps, [])
        XCTAssertEqual(clipboard.items, yours)
    }

    /// You copied something while the paste was on its way: that copy is newer than the one saved, and stays.
    func testSomethingCopiedMeanwhileStays() {
        XCTAssertEqual(ClipboardPaste.paste(text, ready: { true }, forward: { true }, press: {
            self.clipboard.copy("copié à l'instant")
            return true
        }), .pasted)
        XCTAssertEqual(clipboard.text, "copié à l'instant")
        XCTAssertEqual(clipboard.steps, ["save", "put"], "not put back over it")
    }

    /// Two desks in two terminals pasting at once: one at a time, each clipboard put back before the next.
    func testOnePasteAtATime() {
        let done = expectation(description: "both")
        done.expectedFulfillmentCount = 2
        let lock = NSLock()
        var inside = 0, most = 0
        for _ in 0..<2 {
            DispatchQueue.global().async {
                _ = ClipboardPaste.paste(self.text, ready: { true }, forward: { true }, press: {
                    lock.lock(); inside += 1; most = max(most, inside); lock.unlock()
                    Thread.sleep(forTimeInterval: 0.05)
                    lock.lock(); inside -= 1; lock.unlock()
                    return true
                })
                done.fulfill()
            }
        }
        wait(for: [done], timeout: 5)
        XCTAssertEqual(most, 1)
        XCTAssertEqual(clipboard.items, yours)
        XCTAssertEqual(clipboard.steps, ["save", "put", "restore", "save", "put", "restore"])
    }

    /// ⌘V is one key, V with ⌘, down then up, to the app in front only while it's the terminal's: no Return.
    /// Posted to a stand-in, so no key reaches a real app.
    func testCommandVIsTheOnlyKeyAndGoesOnlyToTheTerminalInFront() throws {
        guard !Terminal.isLocked else { throw XCTSkip("the Mac is locked") }
        let front = try XCTUnwrap(NSWorkspace.shared.frontmostApplication?.bundleIdentifier, "an app in front")
        let saved = Terminal.postEvent
        defer { Terminal.postEvent = saved }
        var posted: [(key: Int64, down: Bool, command: Bool, pid: pid_t?)] = []
        Terminal.postEvent = { event, pid in
            posted.append((event.getIntegerValueField(.keyboardEventKeycode), event.type == .keyDown, event.flags.contains(.maskCommand), pid))
        }
        XCTAssertFalse(Terminal.pressPaste(in: front + ".not-in-front"), "never to another app")
        XCTAssertTrue(posted.isEmpty)
        XCTAssertTrue(Terminal.pressPaste(in: front))
        XCTAssertEqual(posted.map(\.key), [9, 9])
        XCTAssertEqual(posted.map(\.down), [true, false])
        XCTAssertEqual(posted.map(\.command), [true, true])
        XCTAssertEqual(posted.map(\.pid), [nil, nil], "the keyboard's own path, as Terminal's menu shortcuts go")
    }

    /// Terminal.app and iTerm2 bring the tab forward for ⌘V; Ghostty pastes into its terminal where it is.
    func testWhichTerminalsPasteSeveralLinesAndHow() {
        XCTAssertEqual(TerminalApp().linesWay, .clipboard(inFront: true))
        XCTAssertEqual(ITerm().linesWay, .clipboard(inFront: true))
        XCTAssertEqual(Ghostty().linesWay, .clipboard(inFront: false))
        XCTAssertEqual(Tmux().linesWay, .paneFlag)
        XCTAssertEqual(WezTerm().linesWay, .never)
        XCTAssertEqual(Kitty().linesWay, .never)
    }
}
