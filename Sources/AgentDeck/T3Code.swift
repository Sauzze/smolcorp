import AppKit

/// T3 Code's app on this Mac, for the Go core (M5c): whether it's installed, found by its
/// bundle id, opening it, and handing it a thread's link, all through NSWorkspace, never a
/// shell. The core runs T3 Code's `t3` command itself (desktop/t3code.go).
enum T3Code {
    /// T3 Code's app, as its Info.plist names it.
    static let bundleID = "com.t3tools.t3code"

    /// What the bridge asks of this Mac for the Go core (`HostBridge`'s `t3-app`, `t3-open-app` and `t3-link`).
    struct Machine {
        var appInstalled: () -> Bool = { NSWorkspace.shared.urlForApplication(withBundleIdentifier: T3Code.bundleID) != nil }
        var openApp: () -> Void = {
            guard let app = NSWorkspace.shared.urlForApplication(withBundleIdentifier: T3Code.bundleID) else { return }
            let configuration = NSWorkspace.OpenConfiguration()
            configuration.activates = true
            NSWorkspace.shared.openApplication(at: app, configuration: configuration)
        }
        /// Opens a link (T3 Code's own `t3code://` one) with NSWorkspace, never through a shell. Says whether it was taken.
        var openURL: (URL) -> Bool = { NSWorkspace.shared.open($0) }
    }
}
