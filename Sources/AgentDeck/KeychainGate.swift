import Foundation
import Security

/// Smolcorp's Keychain items (the API key, the GM's token, the portable office's) are read by this app alone, and
/// only once it holds its support folder's instance lock (`InstanceLock`). A second Smolcorp.app started on the same
/// folder while yours runs, by a hire say, never takes the lock, so it asks the Keychain nothing. The `smolcorp`
/// program never asks the Keychain at all: the core gets what it needs from this app, over the bridge.
///
/// Every SecItem call in the app goes through `Keychain` below (HostBridgeHardeningTests checks no other file makes
/// one). Until the gate opens, each answers `errSecNotAvailable` without asking: a failed read, never "not found", so
/// nothing takes it for "none" and writes a new item over the real one.
enum KeychainGate {
    private static let lock = NSLock()
    private static var opened = false

    static var isOpen: Bool {
        lock.lock()
        defer { lock.unlock() }
        return opened
    }

    /// Opened by the app once it holds `held`: never before, and never closed, as the lock is held until the app ends.
    static func open(holding held: InstanceLock) {
        guard held.isHeld else { return }
        lock.lock()
        opened = true
        lock.unlock()
        // What was looked for while the gate was shut found nothing: it's looked for again.
        SummaryKey.forgetKnown()
    }

    /// `call`'s answer when `open`; `errSecNotAvailable`, without calling it, when not.
    static func call(when open: Bool, _ call: () -> OSStatus) -> OSStatus {
        open ? call() : errSecNotAvailable
    }

    static func call(_ call: () -> OSStatus) -> OSStatus { self.call(when: isOpen, call) }
}

/// The Keychain as this app asks it: SecItem's calls, each through `KeychainGate`.
enum Keychain {
    static func copyMatching(_ query: [String: Any], _ result: UnsafeMutablePointer<CFTypeRef?>?) -> OSStatus {
        KeychainGate.call { SecItemCopyMatching(query as CFDictionary, result) }
    }

    static func add(_ item: [String: Any]) -> OSStatus {
        KeychainGate.call { SecItemAdd(item as CFDictionary, nil) }
    }

    @discardableResult
    static func delete(_ query: [String: Any]) -> OSStatus {
        KeychainGate.call { SecItemDelete(query as CFDictionary) }
    }
}

/// A support folder's instance lock: `instance.lock`, locked (flock, exclusive) for as long as this app runs, as the
/// Go app takes it on Linux and Windows (desktop/instance_unix.go). On a Mac this app holds it, before it starts its
/// core or opens the Keychain, and the core checks it's held before it starts (desktop/core_darwin.go). A link at
/// its name is refused, never followed. The system lets go when the app ends, however it ends.
final class InstanceLock {
    let file: URL
    private var held: Int32 = -1

    init(support: URL) {
        file = support.appendingPathComponent("instance.lock")
    }

    var isHeld: Bool { held >= 0 }

    /// Takes it, unless another process holds it (another Smolcorp on this folder) or it can't be had: false then.
    func take() -> Bool {
        if held >= 0 { return true }
        let folder = file.deletingLastPathComponent()
        try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        let fd = open(file.path, O_CREAT | O_RDWR | O_NOFOLLOW | O_CLOEXEC, 0o600)
        guard fd >= 0 else { return false }
        guard flock(fd, LOCK_EX | LOCK_NB) == 0 else {
            close(fd)
            return false
        }
        held = fd
        return true
    }

    /// Lets go: for the tests. The app keeps it until it ends.
    func release() {
        guard held >= 0 else { return }
        close(held)
        held = -1
    }

    deinit { release() }
}
