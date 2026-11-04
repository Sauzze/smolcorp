import Foundation

/// A test copy's secrets (the GM's token, the Summary API key) live in files
/// in its own support folder, never in the login Keychain: each rebuilt copy
/// has a new ad-hoc signature, so macOS would ask the owner about every item
/// an earlier build made, and the app waits for the answer. The files are
/// readable by you alone, in a folder that is too.
enum TestCopySecrets {
    /// The GM's token, as a test copy keeps it.
    static let gmTokenName = "test-gm-token"
    /// The Summary API key, as a test copy keeps it: the key alone, on one line.
    static let apiKeyName = "test-api-key"

    /// The viewer's id, key and write secret, as a test copy keeps them: one file each, this name and a dash before the account.
    static let viewerName = "test-viewer-key"

    /// Reads the file's one line. A file put there by hand may be too open (a `>` makes it 0644, in a folder
    /// that was made 0755), so it and its folder are tightened to 0600 and 0700 before it is read.
    static func read(_ file: URL) -> String? {
        guard FileManager.default.fileExists(atPath: file.path) else { return nil }
        try? FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: file.path)
        try? FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: file.deletingLastPathComponent().path)
        guard let data = try? Data(contentsOf: file), let text = String(data: data, encoding: .utf8) else { return nil }
        let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    /// Writes the file whole (mode 0600) and puts its folder to 0700. Says whether it was kept.
    static func write(_ text: String, to file: URL) -> Bool {
        let folder = file.deletingLastPathComponent()
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        try? FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: folder.path)
        return Paths.writePrivately(Data("\(text)\n".utf8), to: file)
    }

    static func remove(_ file: URL) { try? FileManager.default.removeItem(at: file) }
}

/// The GM's token in a file of a test copy's own: see `TestCopySecrets`.
struct GMTokenFile: GMTokenStore {
    let file: URL

    /// No file is no token; a file there that can't be read is a failed read, never "none".
    func read() -> GMTokenRead {
        guard FileManager.default.fileExists(atPath: file.path) else { return .none }
        if let text = TestCopySecrets.read(file) { return .token(text) }
        // There but empty reads as an empty token (replaced like any bad one); unreadable is a failure.
        return (try? Data(contentsOf: file)) != nil ? .token("") : .failed
    }
    func write(_ token: String) -> Bool { TestCopySecrets.write(token, to: file) }
    func remove() { TestCopySecrets.remove(file) }
}

extension SummaryKey.Store {
    /// The API key in a file of a test copy's own: see `TestCopySecrets`. Its hint is made from the key.
    static func file(_ file: URL) -> SummaryKey.Store {
        func key() -> String? { TestCopySecrets.read(file).flatMap { SummaryKey.isKey($0) ? $0 : nil } }
        return SummaryKey.Store(
            read: key,
            hint: { key().map(SummaryKey.hint) },
            // The hint is made from the key, so the one given is not used.
            write: { key, _ in TestCopySecrets.write(key, to: file) },
            delete: { TestCopySecrets.remove(file) })
    }
}

extension ViewerStore {
    /// The viewer's id, key and write secret in files of a test copy's own: see `TestCopySecrets`.
    /// `absent` is true only where there is no file at all, as the Keychain says only "not found".
    static func file(in folder: URL) -> ViewerStore {
        func file(_ account: ViewerKeychain.Account) -> URL {
            folder.appendingPathComponent("\(TestCopySecrets.viewerName)-\(account.rawValue)")
        }
        return ViewerStore(
            read: { TestCopySecrets.read(file($0)) },
            absent: { !FileManager.default.fileExists(atPath: file($0).path) },
            write: { TestCopySecrets.write($0, to: file($1)) })
    }

    /// The Keychain, or in a test copy files in its own support folder (the Keychain is never asked there).
    static func defaultStore(testCopy: Bool = TestCopy.isOn, support: URL? = TestCopy.support,
                             keychain: ViewerStore = .keychain) -> ViewerStore {
        if testCopy, let support { return .file(in: support) }
        return keychain
    }
}
