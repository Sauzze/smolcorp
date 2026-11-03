import Foundation

/// Mail to and from a cloud bot through a folder (T404). A cloud bot has no window to type into, so the
/// user picks, on the setup page, a folder its side reads and writes (a synced folder, say). In it:
///
/// - `to-bot/<time>-<id>.md`: a message for the bot, the sender line and the text (`MailRouter.wire`).
///   Written whole under a hidden name and renamed, so the bot never reads half of one.
/// - `from-bot/`: files the bot leaves for the GM. Each is read once, moved to `from-bot/read/`, and
///   becomes a `.mail` line in the event log. Only plain `.md` and `.txt` files; a link is never followed.
///
/// The folder is never inside the app's support folder nor a hire's worktree (nor holds one), since the
/// bot's side can write there. Checked when it is chosen and again on every use. Nothing here types anywhere.
enum BotMailbox {
    static let toBot = "to-bot"
    static let fromBot = "from-bot"
    static let done = "read"
    /// The most replies taken in one look; the rest wait for the next.
    static let mostPerLook = 20
    /// A file changed this recently may still be arriving (a sync), so it is left for the next look.
    static let settle: TimeInterval = 3
    static let extensions: Set<String> = ["md", "txt"]

    struct Reply: Equatable {
        /// The file's name, as the bot left it.
        var name: String
        var text: String
    }

    // MARK: The folder

    /// Whether `path` can be a mailbox: nil if so; `.failed` if never; `.trouble` if it may be a folder that is not there yet.
    /// `barred` are the folders it may not be in or hold (the support folder, the hires' worktrees).
    static func refusal(_ path: String, barred: [URL]) -> MailRouter.Delivery? {
        guard path.hasPrefix("/") else { return .failed("The mailbox folder has to be a full path.") }
        let url = URL(fileURLWithPath: path)
        var isFolder: ObjCBool = false
        guard FileManager.default.fileExists(atPath: url.path, isDirectory: &isFolder) else {
            return .trouble("The mailbox folder isn't there (is it synced?): \(path)")
        }
        guard isFolder.boolValue else { return .failed("The mailbox isn't a folder: \(path)") }
        let real = canonical(url)
        for folder in barred where overlap(real, canonical(folder)) {
            return .failed("The mailbox can't be in or hold the app's own folder or a hire's worktree: \(folder.path)")
        }
        return nil
    }

    /// The path to keep for a folder the user chose, or why not (a folder that isn't there is refused too).
    static func chosen(_ path: String, barred: [URL]) -> Result<String, MailError> {
        let full = (path.trimmingCharacters(in: .whitespacesAndNewlines) as NSString).expandingTildeInPath
        switch refusal(full, barred: barred) {
        case .failed(let why)?, .trouble(let why)?: return .failure(MailError(why))
        default: return .success(canonical(URL(fileURLWithPath: full)))
        }
    }

    struct MailError: Error, Equatable {
        let message: String
        init(_ message: String) { self.message = message }
    }

    private static func canonical(_ url: URL) -> String { url.resolvingSymlinksInPath().standardized.path }

    private static func overlap(_ a: String, _ b: String) -> Bool {
        func inside(_ child: String, _ parent: String) -> Bool { child == parent || child.hasPrefix(parent.hasSuffix("/") ? parent : parent + "/") }
        return inside(a, b) || inside(b, a)
    }

    /// A folder that is really one, not a link to somewhere else; created if `make` and absent.
    private static func realFolder(_ url: URL, make: Bool) -> Bool {
        var info = stat()
        if lstat(url.path, &info) != 0 {
            guard make, (try? FileManager.default.createDirectory(at: url, withIntermediateDirectories: false)) != nil else { return false }
            return lstat(url.path, &info) == 0 && (info.st_mode & S_IFMT) == S_IFDIR
        }
        return (info.st_mode & S_IFMT) == S_IFDIR
    }

    // MARK: To the bot

    /// The file's name for message `id` sent at `date`: sorts by time, and ends in the id so it is found again.
    static func fileName(id: String, at date: Date) -> String {
        let format = DateFormatter()
        format.locale = Locale(identifier: "en_US_POSIX")
        format.timeZone = TimeZone(identifier: "UTC")
        format.dateFormat = "yyyyMMdd'T'HHmmss'Z'"
        return "\(format.string(from: date))-\(id).md"
    }

    /// Writes `text` to the bot's `to-bot` folder. `commit` counts the try just before the write, as for a window.
    /// A message already there under its id is delivered, so a crash between the write and the save sends nothing twice.
    static func drop(_ text: String, id: String, at date: Date, mailbox: String, barred: [URL], commit: () -> Bool) -> MailRouter.Delivery {
        if let refused = refusal(mailbox, barred: barred) { return refused }
        let out = URL(fileURLWithPath: mailbox).appendingPathComponent(toBot)
        guard realFolder(out, make: true) else { return .failed("\(toBot) in the mailbox isn't a folder, or couldn't be made.") }
        let names = (try? FileManager.default.contentsOfDirectory(atPath: out.path)) ?? []
        if names.contains(where: { $0.hasSuffix("-\(id).md") }) { return .delivered }
        guard commit() else { return .trouble("The outbox couldn't be written, so nothing was put in the mailbox.") }
        guard Paths.writePrivately(Data((text + "\n").utf8), to: out.appendingPathComponent(fileName(id: id, at: date))) else {
            return .trouble("The mailbox folder couldn't be written to.")
        }
        return .delivered
    }

    // MARK: From the bot

    /// The new replies in `from-bot`, oldest first, each moved to `from-bot/read` before it is handed back,
    /// so it is never read twice (one that can't be moved is left alone and not handed back).
    /// A reply longer than `maxBytes` is cut there.
    static func collect(mailbox: String, barred: [URL], maxBytes: Int, now: Date = Date()) -> [Reply] {
        guard refusal(mailbox, barred: barred) == nil else { return [] }
        let inbox = URL(fileURLWithPath: mailbox).appendingPathComponent(fromBot)
        guard realFolder(inbox, make: false) else { return [] }
        let keys: [URLResourceKey] = [.isRegularFileKey, .isSymbolicLinkKey, .contentModificationDateKey]
        let found = ((try? FileManager.default.contentsOfDirectory(at: inbox, includingPropertiesForKeys: keys, options: [.skipsHiddenFiles])) ?? [])
            .compactMap { url -> (url: URL, at: Date)? in
                guard let values = try? url.resourceValues(forKeys: Set(keys)),
                      values.isRegularFile == true, values.isSymbolicLink != true,
                      extensions.contains(url.pathExtension.lowercased()),
                      let at = values.contentModificationDate, now.timeIntervalSince(at) >= settle else { return nil }
                return (url, at)
            }
            .sorted { ($0.at, $0.url.lastPathComponent) < ($1.at, $1.url.lastPathComponent) }
            .prefix(mostPerLook)
        guard !found.isEmpty else { return [] }
        let finished = inbox.appendingPathComponent(done)
        guard realFolder(finished, make: true) else { return [] }

        var replies: [Reply] = []
        for (url, _) in found {
            // Read first, then moved; only a moved file is handed back, so a failed move can't make it twice.
            guard let handle = try? FileHandle(forReadingFrom: url) else { continue }
            let data = (try? handle.read(upToCount: maxBytes + 1)) ?? Data()
            try? handle.close()
            let target = free(finished, named: url.lastPathComponent, at: now)
            guard (try? FileManager.default.moveItem(at: url, to: target)) != nil else { continue }
            var text = String(decoding: data.prefix(maxBytes), as: UTF8.self)
            if data.count > maxBytes { text += "\n[cut at \(maxBytes) bytes]" }
            text = text.trimmingCharacters(in: .whitespacesAndNewlines)
            if !text.isEmpty { replies.append(Reply(name: url.lastPathComponent, text: text)) }
        }
        return replies
    }

    /// `name` in `folder`, or with the time before it when there already is one.
    private static func free(_ folder: URL, named name: String, at date: Date) -> URL {
        let plain = folder.appendingPathComponent(name)
        guard FileManager.default.fileExists(atPath: plain.path) else { return plain }
        return folder.appendingPathComponent("\(Int(date.timeIntervalSince1970))-\(UUID().uuidString.prefix(6))-\(name)")
    }
}
