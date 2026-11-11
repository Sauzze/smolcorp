import AppKit

/// One type's data in one item on the clipboard, as it was.
struct ClipboardEntry: Equatable {
    let type: String
    let data: Data
}

/// What was on the clipboard before a paste through it, to put back.
enum SavedClipboard: Equatable {
    /// Every item, with every type each had, in order.
    case items([[ClipboardEntry]])
    /// Something marked concealed, as a password manager copies a password: never read, and never put
    /// back (VICTOR's ruling on RI34b S2). The clipboard is left empty instead, so the password manager's
    /// own "clear after a while" isn't defeated by a copy of ours.
    case concealed
}

/// The clipboard, as a paste through it uses it: everything on it saved, the phone's text put in its
/// place, and what was saved put back. Settable for tests, which give a stand-in: no test touches the
/// real clipboard.
protocol Clipboard: AnyObject {
    /// Changes each time anything is copied.
    var changeCount: Int { get }
    /// What's on it, to put back exactly; nil when that can't be done (a file being copied, whose promise
    /// can't be kept, or a type whose data can't be read): nothing is pasted then.
    func save() -> SavedClipboard?
    /// `text` alone, as plain text, handed over only when it's read: `read` is called then. The clipboard's
    /// count once it's there, nil if it couldn't be put.
    func put(_ text: String, read: @escaping () -> Void) -> Int?
    /// What it holds now as plain text, if any.
    var plainText: String? { get }
    /// `saved` back, for this Mac only; a concealed copy leaves it empty.
    func restore(_ saved: SavedClipboard)
}

/// The Mac's own clipboard.
final class SystemClipboard: Clipboard {
    private let pasteboard: NSPasteboard
    /// The phone's text, handed over when it's read (see `put`).
    private var served: Served?

    /// `.general` is the Mac's clipboard; a test gives a pasteboard of its own name, never that one.
    init(_ pasteboard: NSPasteboard = .general) { self.pasteboard = pasteboard }

    var changeCount: Int { pasteboard.changeCount }
    var plainText: String? { pasteboard.string(forType: .string) }

    static let concealedType = NSPasteboard.PasteboardType("org.nspasteboard.ConcealedType")
    static let transientType = NSPasteboard.PasteboardType("org.nspasteboard.TransientType")

    func save() -> SavedClipboard? {
        let items = pasteboard.pasteboardItems ?? []
        // A password isn't even read.
        if items.contains(where: { $0.types.contains(SystemClipboard.concealedType) }) { return .concealed }
        var saved: [[ClipboardEntry]] = []
        for item in items {
            let types = SystemClipboard.worthSaving(item.types.map(\.rawValue))
            // A file being copied is a promise only its app can keep: it can't be put back as it was.
            guard !types.contains(where: SystemClipboard.isPromise) else { return nil }
            var entries: [ClipboardEntry] = []
            for type in types {
                guard let data = item.data(forType: NSPasteboard.PasteboardType(type)) else { return nil }
                entries.append(ClipboardEntry(type: type, data: data))
            }
            saved.append(entries)
        }
        return .items(saved)
    }

    /// The types to read and put back: all but the plain-text copies the Mac makes itself from another
    /// type (UTF-16 from UTF-8 or RTF), which it makes again from what's put back, so they aren't
    /// rendered for nothing. Every type an app put there is read: only that brings it back exactly.
    static func worthSaving(_ types: [String]) -> [String] {
        guard types.contains(NSPasteboard.PasteboardType.string.rawValue) else { return types }
        return types.filter { !["public.utf16-external-plain-text", "public.utf16-plain-text"].contains($0) }
    }

    /// A file promised by the app it's copied from (as Finder's or Photos' copies can be).
    static func isPromise(_ type: String) -> Bool { type.lowercased().contains("promise") }

    /// For this Mac only, not your other devices over Universal Clipboard, and marked so clipboard
    /// managers that follow the nspasteboard.org markers leave it out (as the viewer's link is copied).
    /// The text is handed over only when something reads it, which is how the paste knows the terminal
    /// took it (RI34b S1).
    func put(_ text: String, read: @escaping () -> Void) -> Int? {
        let served = Served(text, read: read)
        pasteboard.prepareForNewContents(with: .currentHostOnly)
        let item = NSPasteboardItem()
        guard item.setDataProvider(served, forTypes: [.string]) else { return nil }
        for marker in [SystemClipboard.transientType, SystemClipboard.concealedType] { item.setString("", forType: marker) }
        guard pasteboard.writeObjects([item]) else { return nil }
        self.served = served
        return pasteboard.changeCount
    }

    /// For this Mac only, as the phone's text was put (RI34b S2): a copy made for this Mac alone never
    /// goes to your iPhone over Universal Clipboard by being put back.
    func restore(_ saved: SavedClipboard) {
        pasteboard.prepareForNewContents(with: .currentHostOnly)
        served = nil
        guard case .items(let items) = saved, !items.isEmpty else { return }
        pasteboard.writeObjects(items.map { entries -> NSPasteboardItem in
            let item = NSPasteboardItem()
            for entry in entries { item.setData(entry.data, forType: NSPasteboard.PasteboardType(entry.type)) }
            return item
        })
    }

    /// The phone's text, given to whatever reads the clipboard, and saying so.
    private final class Served: NSObject, NSPasteboardItemDataProvider {
        private let text: String
        private let read: () -> Void

        init(_ text: String, read: @escaping () -> Void) {
            self.text = text
            self.read = read
        }

        func pasteboard(_ pasteboard: NSPasteboard?, item: NSPasteboardItem, provideDataForType type: NSPasteboard.PasteboardType) {
            item.setString(text, forType: type)
            read()
        }
    }
}

/// How ⌘V went, for a paste through the clipboard.
enum PastePress: Equatable {
    /// It went, to the tab checked again right before.
    case sent
    /// Not pressed: another tab or app was in front by then.
    case notInFront
    /// It may or may not have gone, and may yet: Ghostty's paste ended for taking too long.
    case unsure
}

/// Text of several lines from the phone, pasted as you would paste it, for terminals that type text
/// as keys (where a line feed is Return) and can't say whether the agent asked for bracketed paste:
/// Terminal.app, iTerm2 and Ghostty (owner's call, Q1 b). The text goes on the clipboard, the
/// terminal's own paste takes it (⌘V), and the clipboard is put back as it was once the terminal has
/// read it. The terminal wraps the paste in bracketed-paste markers itself where the agent asked for
/// them, so a line break is a new line in the text, never Return; Return, if sent, goes on its own
/// afterwards. A Return is never pressed mid-text: when in doubt, nothing is pasted.
enum ClipboardPaste {
    /// Settable for tests, which give a stand-in.
    static var clipboard: Clipboard = SystemClipboard()
    /// How long the terminal is given to read the clipboard once ⌘V has gone. It's put back as soon as
    /// it's read (and `hold` has passed); not read by then, the paste is said to have failed.
    static var readWait: TimeInterval = 2
    /// Read, the phone's text still stays this long after ⌘V: a clipboard manager that doesn't skip what's
    /// marked may be the one that read it, with the terminal's read still on its way.
    static var hold: TimeInterval = 0.4
    /// Not read in time, the phone's text stays on the clipboard this long after ⌘V before what you had
    /// copied comes back (RI34b S1): a terminal that gets to the paste late then pastes the phone's text,
    /// never what you had copied, which could be a password.
    static var grace: TimeInterval = 10
    /// One paste at a time, whichever terminal: the clipboard is the Mac's, not a terminal's. Held until
    /// the clipboard is put back, even after the paste has answered.
    private static let turn = DispatchSemaphore(value: 1)
    private static let later = DispatchQueue(label: "smolcorp.clipboard-paste.restore")

    static let busy = "Another paste is still on the Mac's clipboard, so nothing was typed: try again in a few seconds."

    /// In this order (RI34b B2): `first` (the Mac in use) and `ready` (the desk free for it) asked once
    /// before the clipboard is touched, so a refusal leaves it as it was (RI34c N1); what's on it saved and
    /// the phone's text put in its place, so nothing slow comes between the tab's checks and ⌘V; `forward`
    /// brings the tab forward and checks it's the one in front, typing nothing (nil, or why not); `ready`
    /// again, right before; `press` checks the tab once more and presses ⌘V, calling `posting` right before
    /// it posts the key: only a read after that is the paste's (RI34c N2). What was saved comes back in every
    /// case, for this Mac only, unless something else was copied in the meantime: that copy is newer, and
    /// stays.
    static func paste(_ text: String, first: () -> LinesPasted? = { nil }, forward: () -> LinesPasted?, ready: () -> String?,
                      press: (_ posting: () -> Void) -> PastePress) -> LinesPasted {
        // Escape could end the terminal's bracket early, and a return would press Enter: refused, as tmux's paste does.
        guard HireTmux.canPaste(text) else { return .failed }
        guard turn.wait(timeout: .now() + grace + readWait) == .success else { return .refused(busy) }
        var handedOn = false
        defer { if !handedOn { turn.signal() } }
        // What's known already is refused before the clipboard is touched: a password you'd copied stays.
        if let why = first() { return why }
        if let why = ready() { return .refused(why) }
        // What can't be put back exactly isn't taken off: a file being copied stays, and nothing is pasted.
        guard let saved = clipboard.save() else { return .unsaved }
        var reads = Reads()
        guard var ours = clipboard.put(text, read: reads.note) else {
            clipboard.restore(saved)
            return .failed
        }
        let putBack = { (ours: Int, reads: Reads) in
            // Rewritten by a clipboard tool (as one that strips formatting does), it's still ours (RI34b N2): such
            // a tool reads it first, so what's there is only looked at once the phone's text was read.
            if clipboard.changeCount == ours || (!reads.none && clipboard.plainText == text) { clipboard.restore(saved) }
        }
        if let why = forward() {
            putBack(ours, reads)
            return why
        }
        if let why = ready() {
            putBack(ours, reads)
            return .refused(why)
        }
        // Read already, by something else than the terminal: put again, so the read that's heard is the paste's.
        if !reads.none {
            reads = Reads()
            guard let again = clipboard.put(text, read: reads.note) else {
                clipboard.restore(saved)
                return .failed
            }
            ours = again
        }
        // From the press on, unless the press says later, right before it posts ⌘V: a read while it checks the tab
        // again isn't the paste's. The text it took is then gone, and the paste is said not read: no Return.
        reads.mark()
        let pressing = press { reads.mark() }
        let pressed = DispatchTime.now()
        if pressing == .notInFront {
            putBack(ours, reads)
            return .notInFront
        }
        if reads.wait(until: pressed + readWait) {
            let since = Double(DispatchTime.now().uptimeNanoseconds - pressed.uptimeNanoseconds) / 1e9
            if since < hold { Thread.sleep(forTimeInterval: hold - since) }
            putBack(ours, reads)
            return pressing == .sent ? .pasted : .failed
        }
        // Not read yet: said as failed, no Return follows, and the phone's text stays until it's read or the grace ends.
        handedOn = true
        let (stillOurs, stillReads) = (ours, reads)
        later.async {
            _ = stillReads.wait(until: pressed + grace)
            putBack(stillOurs, stillReads)
            turn.signal()
        }
        return pressing == .sent ? .notRead : .failed
    }

    /// As above, for a press with no check of its own between being called and posting ⌘V (Ghostty's,
    /// a stand-in's): a read from the press on is the paste's.
    static func paste(_ text: String, first: () -> LinesPasted? = { nil }, forward: () -> LinesPasted?, ready: () -> String?,
                      press: () -> PastePress) -> LinesPasted {
        paste(text, first: first, forward: forward, ready: ready, press: { _ in press() })
    }

    /// Whether the phone's text has been read, at all and since the mark (⌘V), and waiting until it is.
    final class Reads {
        private let lock = NSLock()
        private var count = 0
        private var sinceMark: Int?
        private let signal = DispatchSemaphore(value: 0)

        func note() {
            lock.lock()
            count += 1
            sinceMark = sinceMark.map { $0 + 1 }
            lock.unlock()
            signal.signal()
        }

        /// Reads from now on are the paste's; any before aren't.
        func mark() {
            lock.lock()
            sinceMark = 0
            lock.unlock()
        }

        /// Not read at all, mark or not.
        var none: Bool {
            lock.lock()
            defer { lock.unlock() }
            return count == 0
        }

        private var readSinceMark: Bool {
            lock.lock()
            defer { lock.unlock() }
            return (sinceMark ?? 0) > 0
        }

        /// True once read since the mark, waiting no later than `deadline`.
        func wait(until deadline: DispatchTime) -> Bool {
            while !readSinceMark {
                // A read before the mark signalled too: only one since counts.
                guard signal.wait(timeout: deadline) == .success else { return readSinceMark }
            }
            return true
        }
    }
}
