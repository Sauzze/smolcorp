import AppKit
import XCTest
@testable import AgentDeck

/// Several lines pasted through the clipboard in Terminal.app, iTerm2 and Ghostty (I34, the owner's Q1 b),
/// with a stand-in clipboard: no test touches the real one, and no real terminal is told anything. The Mac's
/// own clipboard code runs on a pasteboard of the test's own name, never the general one.
final class ClipboardPasteTests: XCTestCase {
    /// A clipboard that keeps what's on it and says what was done to it, in order, into `log`.
    final class StandInClipboard: Clipboard {
        var saved: SavedClipboard?
        private(set) var items: [[ClipboardEntry]]
        private(set) var changeCount = 1
        private(set) var steps: [String] = []
        /// Every step, the clipboard's and the paste's, as they came.
        var log: (String) -> Void = { _ in }
        private var served: (() -> Void)?
        private let lock = NSLock()

        init(_ items: [[ClipboardEntry]]) {
            self.items = items
            saved = .items(items)
        }

        private func note(_ step: String) {
            lock.withLock { steps.append(step) }
            log(step)
        }

        func save() -> SavedClipboard? {
            note("save")
            return saved
        }

        func put(_ text: String, read: @escaping () -> Void) -> Int? {
            note("put")
            lock.withLock {
                items = [[ClipboardEntry(type: NSPasteboard.PasteboardType.string.rawValue, data: Data(text.utf8))]]
                served = read
                changeCount += 1
            }
            return changeCount
        }

        var plainText: String? {
            lock.withLock { items.first?.first { $0.type == NSPasteboard.PasteboardType.string.rawValue } }
                .map { String(decoding: $0.data, as: UTF8.self) }
        }

        func restore(_ saved: SavedClipboard) {
            note("restore")
            lock.withLock {
                served = nil
                if case .items(let back) = saved { items = back } else { items = [] }
                changeCount += 1
            }
        }

        /// What a terminal's paste takes now, as it reads the clipboard: the phone's text says it was read.
        func read() -> String? {
            note("read")
            let served = lock.withLock { self.served }
            served?()
            return plainText
        }

        func copy(_ text: String) {
            lock.withLock {
                items = [[ClipboardEntry(type: "public.utf8-plain-text", data: Data(text.utf8))]]
                changeCount += 1
            }
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
    private var savedTimes: (TimeInterval, TimeInterval, TimeInterval) = (0, 0, 0)

    override func setUp() {
        clipboard = StandInClipboard(yours)
        savedClipboard = ClipboardPaste.clipboard
        savedTimes = (ClipboardPaste.readWait, ClipboardPaste.hold, ClipboardPaste.grace)
        ClipboardPaste.clipboard = clipboard
        ClipboardPaste.readWait = 0.3
        ClipboardPaste.hold = 0.02
        ClipboardPaste.grace = 0.8
    }

    override func tearDown() {
        // A paste still holding the phone's text for its grace is let finish first, so the next test starts clean.
        _ = ClipboardPaste.paste("x", forward: { .failed }, ready: { nil }, press: { .sent })
        ClipboardPaste.clipboard = savedClipboard
        (ClipboardPaste.readWait, ClipboardPaste.hold, ClipboardPaste.grace) = savedTimes
    }

    private let text = "Deux lignes :\n    « ça marche » 👩‍💻 $(touch x) fin;\n\tet un tab"

    /// The text goes byte for byte, line feeds and all, ⌘V once (no Return of ours, mid-text or after), and
    /// what you had copied comes back exactly, every item and type, once the terminal has read it. In
    /// RI34b B2's order, with RI34c N1's check first: ready, then the clipboard, then forward → ready again →
    /// the tab checked again → ⌘V.
    func testThePasteTakesTheTextAndTheClipboardComesBack() {
        var steps: [String] = []
        clipboard.log = { steps.append($0) }
        var pasted: String?
        let result = ClipboardPaste.paste(text, forward: { steps.append("forward"); return nil }, ready: { steps.append("ready"); return nil },
                                          press: {
                                              steps.append("tab checked again")
                                              steps.append("⌘V")
                                              pasted = self.clipboard.read()
                                              return .sent
                                          })
        XCTAssertEqual(result, .pasted)
        XCTAssertEqual(pasted.map { Data($0.utf8) }, Data(text.utf8), "byte for byte")
        XCTAssertEqual(steps, ["ready", "save", "put", "forward", "ready", "tab checked again", "⌘V", "read", "restore"])
        XCTAssertEqual(clipboard.items, yours, "every item and type back")
    }

    /// Another tab or app in front by ⌘V (checked again inside the press): nothing pressed, nothing read, and
    /// what you had copied is back at once.
    func testNothingIsPastedWhenTheTabIsntInFrontAnyMore() {
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { nil }, ready: { nil }, press: { .notInFront }), .notInFront)
        XCTAssertEqual(clipboard.steps, ["save", "put", "restore"])
        XCTAssertEqual(clipboard.items, yours)
    }

    /// The desk not ready, the tab not brought forward, or the Mac in use: nothing pressed, the clipboard back.
    /// What's known before the tab is brought forward (the Mac in use, the desk not free) is refused before
    /// the clipboard is even read (RI34c N1): a password you'd copied stays, and the phone's text never goes on.
    func testNothingIsPressedWhenTheDeskOrTheTabSaysNo() {
        var pressed = 0
        let press = { () -> PastePress in pressed += 1; return .sent }
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { nil }, ready: { Deck.linesInEditor }, press: press), .refused(Deck.linesInEditor))
        XCTAssertEqual(ClipboardPaste.paste(text, first: { .macInUse }, forward: { nil }, ready: { nil }, press: press), .macInUse)
        XCTAssertEqual(clipboard.steps, [], "not touched")
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { .notInFront }, ready: { nil }, press: press), .notInFront)
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { .macInUse }, ready: { nil }, press: press), .macInUse)
        // Free when first asked, not by the time the tab is in front: the clipboard back.
        var asked = 0
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { nil }, ready: {
            asked += 1
            return asked == 1 ? nil : Deck.answerFirst
        }, press: press), .refused(Deck.answerFirst))
        XCTAssertEqual(pressed, 0)
        XCTAssertEqual(clipboard.items, yours)
        XCTAssertEqual(clipboard.steps, Array(repeating: ["save", "put", "restore"], count: 3).flatMap { $0 })
        // An Escape could end the terminal's bracket early, and a return press Enter: refused before anything.
        for text in ["a\u{1B}[201~\nb", "a\rb\nc"] {
            XCTAssertEqual(ClipboardPaste.paste(text, forward: { nil }, ready: { nil }, press: press), .failed)
        }
        XCTAssertEqual(clipboard.steps.count, 9)
    }

    /// Only a read after ⌘V goes is the paste's (RI34c N2): one while the press checks the tab again, before
    /// posting, isn't. The text it took is gone then, so the paste is said not read: a 503, and no Return.
    func testOnlyAReadAfterCommandVIsThePastes() {
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { nil }, ready: { nil }, press: { posting in
            posting()
            _ = self.clipboard.read()
            return .sent
        }), .pasted)
        XCTAssertEqual(clipboard.items, yours)
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { nil }, ready: { nil }, press: { posting in
            _ = self.clipboard.read()
            posting()
            return .sent
        }), .notRead, "read while the tab was checked again, before ⌘V")
        XCTAssertEqual(clipboard.plainText, text, "the phone's text stays for the grace, never yours")
        // Nothing posted, nothing counted, even a read during the press.
        let reads = ClipboardPaste.Reads()
        reads.note()
        reads.mark()
        XCTAssertFalse(reads.wait(until: .now() + 0.05), "a read before the mark isn't counted, though it signalled")
        XCTAssertFalse(reads.none)
        reads.note()
        XCTAssertTrue(reads.wait(until: .now() + 0.05))
    }

    /// What can't be put back exactly (a file being copied) isn't taken off the clipboard: nothing is put,
    /// pressed or pasted.
    func testAClipboardThatCantBeSavedIsLeftAndNothingIsPasted() {
        clipboard.saved = nil
        var pressed = false
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { nil }, ready: { nil }, press: { pressed = true; return .sent }), .unsaved)
        XCTAssertFalse(pressed)
        XCTAssertEqual(clipboard.steps, ["save"])
        XCTAssertEqual(clipboard.items, yours)
    }

    /// ⌘V went but the terminal hadn't read the clipboard in time (RI34b S1): said as not read, and the
    /// phone's text stays on, so the paste that comes late takes it, never what you had copied. Put back
    /// right after that late read.
    func testALateReadStillTakesThePhonesTextNeverYours() {
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { nil }, ready: { nil }, press: { .sent }), .notRead)
        XCTAssertEqual(clipboard.plainText, text, "still the phone's text, past the read wait")
        Thread.sleep(forTimeInterval: 0.2)
        XCTAssertEqual(clipboard.read(), text, "the late paste")
        let back = expectation(description: "put back")
        DispatchQueue.global().async {
            while self.clipboard.items != self.yours { Thread.sleep(forTimeInterval: 0.01) }
            back.fulfill()
        }
        wait(for: [back], timeout: 2)
    }

    /// Never read at all: put back once the grace is over, and the next paste waits its turn till then.
    func testNeverReadComesBackAfterTheGraceAndTheNextPasteWaits() {
        let started = Date()
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { nil }, ready: { nil }, press: { .sent }), .notRead)
        var seenAtNext: String?
        XCTAssertEqual(ClipboardPaste.paste(text, forward: {
            seenAtNext = self.clipboard.plainText
            return .notInFront
        }, ready: { nil }, press: { .sent }), .notInFront)
        XCTAssertGreaterThanOrEqual(Date().timeIntervalSince(started), ClipboardPaste.grace - 0.05, "waited for the grace")
        XCTAssertEqual(seenAtNext, text, "the next paste's own text: what you had copied was put back before it saved")
        XCTAssertEqual(clipboard.items, yours)
        XCTAssertEqual(clipboard.steps, ["save", "put", "restore", "save", "put", "restore"])
    }

    /// Ghostty's own paste, through a stand-in osascript that never answers: ended once its time is up, the
    /// paste fails as having timed out, with no Return. The phone's text stays on for the grace (RI34b S1):
    /// Ghostty catching up later pastes it, not what you had copied; then that comes back.
    func testWhenTheTerminalStopsAnsweringALatePasteTakesThePhonesText() throws {
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
        let saved = (Terminal.osascript, Terminal.secondsSinceInput)
        Terminal.osascript = standIn.path
        // Someone at the Mac: Ghostty pastes where it is, so that doesn't stop it.
        Terminal.secondsSinceInput = { 0 }
        let tty = "/dev/ttys977"
        defer {
            (Terminal.osascript, Terminal.secondsSinceInput) = saved
            Ghostty.forget(tty: tty)
        }
        ClipboardPaste.grace = 6
        Ghostty.learn(tty: tty)
        XCTAssertNotNil(Ghostty.id(on: tty))
        let tab = try XCTUnwrap(TabAddress("ghostty:\(tty)"))
        let (result, timedOut, partWay) = Terminal.noticingTimeouts { Ghostty().insertLines(text, in: tab, ready: { nil }) }
        XCTAssertEqual(result, .failed)
        XCTAssertTrue(timedOut)
        XCTAssertTrue(partWay, "the paste may have gone: the phone is told to look before trying again")
        XCTAssertEqual(try String(contentsOf: asked, encoding: .utf8), "paste\n", "only Ghostty's paste, nothing typed as keys")
        XCTAssertEqual(clipboard.read(), text, "Ghostty catching up pastes the phone's text, never yours")
        let back = expectation(description: "put back")
        DispatchQueue.global().async {
            while self.clipboard.items != self.yours { Thread.sleep(forTimeInterval: 0.01) }
            back.fulfill()
        }
        wait(for: [back], timeout: 3)
        XCTAssertEqual(Ghostty().linesWay, .clipboard(inFront: false))
    }

    /// Read before ⌘V (a clipboard manager that doesn't skip what's marked): the text is put again right
    /// before it, so the read that's waited for is the terminal's.
    func testAReadBeforeThePressDoesntCountAsThePaste() {
        var asked = 0
        let result = ClipboardPaste.paste(text, forward: { nil }, ready: {
            // Asked first before the clipboard is touched (RI34c N1), then right before ⌘V, with the phone's text on.
            asked += 1
            if asked == 2 { _ = self.clipboard.read() }
            return nil
        }, press: { .sent })
        XCTAssertEqual(result, .notRead, "only the manager read it")
        XCTAssertEqual(Array(clipboard.steps.prefix(4)), ["save", "put", "read", "put"])
        XCTAssertEqual(clipboard.plainText, text)
    }

    /// You copied something while the paste was on its way: that copy is newer than the one saved, and stays.
    /// A clipboard tool that only rewrote the phone's text (as one stripping formatting) doesn't count: yours
    /// comes back (RI34b N2).
    func testSomethingCopiedMeanwhileStaysButARewriteOfOursDoesnt() {
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { nil }, ready: { nil }, press: {
            _ = self.clipboard.read()
            self.clipboard.copy("copié à l'instant")
            return .sent
        }), .pasted)
        XCTAssertEqual(clipboard.plainText, "copié à l'instant")
        XCTAssertFalse(clipboard.steps.contains("restore"), "not put back over it")

        clipboard.restore(.items(yours))
        XCTAssertEqual(ClipboardPaste.paste(text, forward: { nil }, ready: { nil }, press: {
            let read = self.clipboard.read() ?? ""
            self.clipboard.copy(read)
            return .sent
        }), .pasted)
        XCTAssertEqual(clipboard.items, yours)
    }

    /// Two desks in two terminals pasting at once: one at a time, each clipboard put back before the next.
    func testOnePasteAtATime() {
        let done = expectation(description: "both")
        done.expectedFulfillmentCount = 2
        let lock = NSLock()
        var inside = 0, most = 0
        for _ in 0..<2 {
            DispatchQueue.global().async {
                _ = ClipboardPaste.paste(self.text, forward: { nil }, ready: { nil }, press: {
                    lock.lock(); inside += 1; most = max(most, inside); lock.unlock()
                    Thread.sleep(forTimeInterval: 0.05)
                    _ = self.clipboard.read()
                    lock.lock(); inside -= 1; lock.unlock()
                    return .sent
                })
                done.fulfill()
            }
        }
        wait(for: [done], timeout: 5)
        XCTAssertEqual(most, 1)
        XCTAssertEqual(clipboard.items, yours)
        XCTAssertEqual(clipboard.steps, ["save", "put", "read", "restore", "save", "put", "read", "restore"])
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
        // Told right before the key goes, once every check has passed (RI34c N2); not at all when it doesn't go.
        var order: [String] = []
        Terminal.postEvent = { _, _ in order.append("key") }
        XCTAssertFalse(Terminal.pressPaste(in: front + ".not-in-front", posting: { order.append("posting") }))
        XCTAssertTrue(Terminal.pressPaste(in: front, posting: { order.append("posting") }))
        XCTAssertEqual(order, ["posting", "key", "key"])
    }

    /// Terminal.app's ⌘V asks again, right before, which tab is in front (RI34b B2): another one, and
    /// nothing is pressed. Through a stand-in osascript and a stand-in for posting: nothing real is told.
    func testTerminalsPressChecksTheTabAgainFirst() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder) }
        let standIn = folder.appendingPathComponent("osascript")
        try "#!/bin/sh\necho /dev/ttys978\n".write(to: standIn, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: standIn.path)
        let saved = (Terminal.osascript, Terminal.postEvent)
        defer { (Terminal.osascript, Terminal.postEvent) = saved }
        Terminal.osascript = standIn.path
        var posted = 0
        Terminal.postEvent = { _, _ in posted += 1 }
        XCTAssertEqual(Terminal.pressPaste(tty: "/dev/ttys979"), .notInFront, "another tab is in front")
        XCTAssertEqual(posted, 0)
    }

    /// Terminal.app and iTerm2 come to the front for ⌘V: not while you're using the Mac (VICTOR's ruling on
    /// RI34b S3), on a stand-in clock. Nothing is asked of Terminal, nothing pressed, the clipboard back.
    func testTerminalAndITermDontTakeTheFocusWhileTheMacIsInUse() throws {
        let saved = (Terminal.secondsSinceInput, Terminal.osascript, Terminal.lastOwnKeys)
        defer { (Terminal.secondsSinceInput, Terminal.osascript, Terminal.lastOwnKeys) = saved }
        Terminal.osascript = "/nonexistent/osascript-never-run"
        Terminal.lastOwnKeys = nil
        Terminal.secondsSinceInput = { 3 }
        XCTAssertTrue(Terminal.macInUse)
        let ready = { () -> String? in XCTFail("not asked: the Mac is in use"); return nil }
        XCTAssertEqual(TerminalApp().insertLines(text, in: .terminalApp(tty: "/dev/ttys979"), ready: ready), .macInUse)
        XCTAssertEqual(ITerm().insertLines(text, in: TabAddress(kind: "iterm", id: "BD2206F6"), ready: ready), .macInUse)
        XCTAssertEqual(clipboard.items, yours)
        XCTAssertEqual(clipboard.steps, [], "the clipboard not even read (RI34c N1)")
        Terminal.secondsSinceInput = { 12 }
        XCTAssertFalse(Terminal.macInUse, "left alone for 10 seconds")
        // The rule itself: within 10 seconds of input, unless that input was Smolcorp's own keys.
        XCTAssertTrue(Terminal.inUse(sinceInput: 3, sinceOwnKeys: nil))
        XCTAssertTrue(Terminal.inUse(sinceInput: 9.9, sinceOwnKeys: nil))
        XCTAssertFalse(Terminal.inUse(sinceInput: 10, sinceOwnKeys: nil))
        XCTAssertFalse(Terminal.inUse(sinceInput: 60, sinceOwnKeys: 1))
        XCTAssertFalse(Terminal.inUse(sinceInput: 2, sinceOwnKeys: 2.1), "the Mac counted Smolcorp's own keys")
        XCTAssertTrue(Terminal.inUse(sinceInput: 1, sinceOwnKeys: 5), "touched after Smolcorp's keys")
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

    // MARK: The Mac's own clipboard code, on a pasteboard of the test's own name (never the general one)

    private func ownPasteboard() -> NSPasteboard {
        let pasteboard = NSPasteboard(name: .init("smolcorp-test-\(UUID().uuidString)"))
        addTeardownBlock { pasteboard.releaseGlobally() }
        return pasteboard
    }

    /// The phone's text is handed over only when read, which is what says the terminal took it; marked so
    /// clipboard managers leave it out. What was saved comes back, every item and type.
    func testTheTextIsHandedOverWhenReadAndYoursComesBack() throws {
        let pasteboard = ownPasteboard()
        let system = SystemClipboard(pasteboard)
        system.restore(.items(yours))
        let saved = try XCTUnwrap(system.save())
        XCTAssertEqual(saved, .items(yours))
        var reads = 0
        let count = try XCTUnwrap(system.put(text) { reads += 1 })
        XCTAssertEqual(count, pasteboard.changeCount)
        XCTAssertEqual(reads, 0, "not read yet")
        XCTAssertEqual(Set(pasteboard.types?.map(\.rawValue) ?? []).isSuperset(of: ["org.nspasteboard.TransientType", "org.nspasteboard.ConcealedType"]), true)
        XCTAssertEqual(pasteboard.string(forType: .string), text)
        XCTAssertEqual(reads, 1)
        system.restore(saved)
        XCTAssertEqual(system.save(), .items(yours))
    }

    /// A password (marked concealed) isn't even read, and isn't put back: the clipboard is left empty
    /// (VICTOR's ruling on RI34b S2).
    func testAConcealedCopyIsntReadNorPutBack() throws {
        final class Counting: NSObject, NSPasteboardItemDataProvider {
            var asked = 0
            func pasteboard(_ pasteboard: NSPasteboard?, item: NSPasteboardItem, provideDataForType type: NSPasteboard.PasteboardType) {
                asked += 1
                item.setString("hunter2", forType: type)
            }
        }
        let pasteboard = ownPasteboard()
        let provider = Counting()
        pasteboard.prepareForNewContents(with: .currentHostOnly)
        let item = NSPasteboardItem()
        XCTAssertTrue(item.setDataProvider(provider, forTypes: [.string]))
        item.setString("", forType: SystemClipboard.concealedType)
        XCTAssertTrue(pasteboard.writeObjects([item]))
        let system = SystemClipboard(pasteboard)
        XCTAssertEqual(system.save(), .concealed)
        XCTAssertEqual(provider.asked, 0, "the password wasn't read")
        _ = system.put(text) {}
        system.restore(.concealed)
        XCTAssertEqual(pasteboard.pasteboardItems?.count ?? 0, 0, "left empty")
        XCTAssertNil(pasteboard.string(forType: .string))
    }

    /// A file being copied (a promise) or a type whose data can't be read can't be put back exactly: not
    /// saved, so nothing is pasted (RI34b N1). The plain-text copies the Mac makes itself aren't read.
    func testWhatCantBePutBackExactlyIsntSavedAndCopiesArentRendered() throws {
        final class Empty: NSObject, NSPasteboardItemDataProvider {
            func pasteboard(_ pasteboard: NSPasteboard?, item: NSPasteboardItem, provideDataForType type: NSPasteboard.PasteboardType) {}
        }
        let pasteboard = ownPasteboard()
        let system = SystemClipboard(pasteboard)
        pasteboard.prepareForNewContents(with: .currentHostOnly)
        let promise = NSPasteboardItem()
        promise.setString("file:///x", forType: .init("com.apple.pasteboard.promised-file-url"))
        pasteboard.writeObjects([promise])
        XCTAssertNil(system.save(), "a file promise")
        let provider = Empty()
        pasteboard.prepareForNewContents(with: .currentHostOnly)
        let unreadable = NSPasteboardItem()
        XCTAssertTrue(unreadable.setDataProvider(provider, forTypes: [.init("com.example.lazy")]))
        pasteboard.writeObjects([unreadable])
        XCTAssertNil(system.save(), "a type whose data never comes")
        XCTAssertEqual(SystemClipboard.worthSaving(["public.rtf", "public.utf16-external-plain-text", "public.utf8-plain-text"]),
                       ["public.rtf", "public.utf8-plain-text"])
        XCTAssertEqual(SystemClipboard.worthSaving(["public.utf16-plain-text"]), ["public.utf16-plain-text"], "the only text there")
    }

    /// Put back for this Mac only, never by clearing it for every device (RI34b S2).
    func testItsPutBackForThisMacOnly() throws {
        let source = try String(contentsOfFile: #filePath.replacingOccurrences(of: "Tests/AgentDeckTests/ClipboardPasteTests.swift",
                                                                                with: "Sources/AgentDeck/ClipboardPaste.swift"), encoding: .utf8)
        let restore = try XCTUnwrap(source.range(of: "func restore(_ saved: SavedClipboard) {").map { String(source[$0.upperBound...].prefix(200)) })
        XCTAssertTrue(restore.contains("pasteboard.prepareForNewContents(with: .currentHostOnly)"))
        XCTAssertFalse(source.contains("clearContents()"))
    }
}
