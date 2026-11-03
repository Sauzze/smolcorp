import Foundation

/// The manager's switch (`Config.managerEnabled`): what the GM's routes do while it is
/// off, and what `GET /setup/gm/version` says. Off is 1.23's behaviour; events are
/// logged either way, so turning it on later shows the recent history.
enum ManagerSwitch {
    /// What the GM's tools can count on in this app. Each 1.24 task that adds a route adds its word.
    static let features = ["events", "team", "registry", "token", "mail", "screens", "tile"]

    /// The routes that work with the manager off: the version (a tool asks it first) and the
    /// switch itself, as turning it on has to be possible, and the token-file setting.
    static func staysOpen(_ method: String, _ path: String) -> Bool {
        (method == "GET" && path == "/setup/gm/version") || (method == "POST" && (path == "/setup/gm/manager" || path == "/setup/gm/by-hand"))
    }

    /// What a /setup/gm route is told while the manager is off, or nil if it goes ahead.
    static func refusal(_ method: String, _ path: String, enabled: Bool) -> Response? {
        guard !enabled, path.hasPrefix("/setup/gm/"), !staysOpen(method, path) else { return nil }
        return .status(409, "The manager is off. Turn it on in Smolcorp's setup.")
    }

    /// `GET /setup/gm/version`.
    static func versionJSON(version: String, enabled: Bool) -> [String: Any] {
        ["version": version, "managerEnabled": enabled, "features": features]
    }

    /// `POST /setup/gm/manager?enabled=1` (or 0): answers how the switch stands.
    static func stateJSON(enabled: Bool) -> [String: Any] { ["managerEnabled": enabled] }
}
