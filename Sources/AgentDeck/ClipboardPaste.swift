import AppKit

/// One type's data in one item on the clipboard, as it was.
struct ClipboardEntry: Equatable {
    let type: String
    let data: Data
}

/// The clipboard, as a paste through it uses it: everything on it saved, the phone's text put in its
/// place, and what was saved put back. Settable for tests, which give a stand-in: no test touches the
/// real clipboard.
protocol Clipboard: AnyObject {
    /// Changes each time anything is copied.
    var changeCount: Int { get }
    /// Every item, with every type each has, in order.
    func save() -> [[ClipboardEntry]]
    /// `text` alone, as plain text; the clipboard's count once it's there, nil if it couldn't be put.
    func put(_ text: String) -> Int?
    /// `items` back, exactly: every item and every type; none clears it.
    func restore(_ items: [[ClipboardEntry]])
}

/// The Mac's own clipboard.
final class SystemClipboard: Clipboard {
    private var pasteboard: NSPasteboard { .general }

    var changeCount: Int { pasteboard.changeCount }

    func save() -> [[ClipboardEntry]] {
        (pasteboard.pasteboardItems ?? []).map { item in
            item.types.compactMap { type in item.data(forType: type).map { ClipboardEntry(type: type.rawValue, data: $0) } }
        }
    }

    /// For this Mac only, not your other devices over Universal Clipboard, and marked so clipboard
    /// managers that follow the nspasteboard.org markers leave it out (as the viewer's link is copied).
    func put(_ text: String) -> Int? {
        pasteboard.prepareForNewContents(with: .currentHostOnly)
        let item = NSPasteboardItem()
        guard item.setString(text, forType: .string) else { return nil }
        for marker in ["org.nspasteboard.TransientType", "org.nspasteboard.ConcealedType"] {
            item.setString("", forType: NSPasteboard.PasteboardType(marker))
        }
        return pasteboard.writeObjects([item]) ? pasteboard.changeCount : nil
    }

    func restore(_ items: [[ClipboardEntry]]) {
        pasteboard.clearContents()
        let restored = items.map { entries -> NSPasteboardItem in
            let item = NSPasteboardItem()
            for entry in entries { item.setData(entry.data, forType: NSPasteboard.PasteboardType(entry.type)) }
            return item
        }
        if !restored.isEmpty { pasteboard.writeObjects(restored) }
    }
}

/// Text of several lines from the phone, pasted as you would paste it, for terminals that type text
/// as keys (where a line feed is Return) and can't say whether the agent asked for bracketed paste:
/// Terminal.app, iTerm2 and Ghostty (owner's call, Q1 b). The text goes on the clipboard, the
/// terminal's own paste takes it (⌘V), and the clipboard is put back as it was, every item and type,
/// however the paste went: done, failed or ended for taking too long. The terminal wraps the paste in
/// bracketed-paste markers itself where the agent asked for them, so a line break is a new line in
/// the text, never Return; Return, if sent, goes on its own afterwards.
enum ClipboardPaste {
    /// Settable for tests, which give a stand-in.
    static var clipboard: Clipboard = SystemClipboard()
    /// How long the terminal is given to read the clipboard once ⌘V has gone, before it's put back. The
    /// terminal reads it as it handles the key, within a few hundredths of a second on a Mac in use.
    static var readWait: TimeInterval = 0.4
    /// One paste at a time, whichever terminal: the clipboard is the Mac's, not a terminal's.
    private static let lock = NSLock()

    /// `forward` brings the tab forward and checks it's the one in front, typing nothing; `ready` is
    /// asked right after, right before the paste; `press` pastes. The clipboard is touched only once
    /// both said yes, and put back afterwards in every case, unless something else was copied in the
    /// meantime: that copy is newer than what was saved, and stays.
    static func paste(_ text: String, ready: () -> Bool, forward: () -> Bool, press: () -> Bool) -> LinesPasted {
        // Escape could end the terminal's bracket early, and a return would press Enter: refused, as tmux's paste does.
        guard HireTmux.canPaste(text) else { return .failed }
        lock.lock()
        defer { lock.unlock() }
        guard forward() else { return .failed }
        guard ready() else { return .asking }
        let saved = clipboard.save()
        guard let ours = clipboard.put(text) else {
            clipboard.restore(saved)
            return .failed
        }
        defer {
            if clipboard.changeCount == ours { clipboard.restore(saved) }
        }
        let pressed = press()
        // Even if the press looked failed (a script ended for taking too long), the terminal may still be reading.
        Thread.sleep(forTimeInterval: readWait)
        return pressed ? .pasted : .failed
    }
}
