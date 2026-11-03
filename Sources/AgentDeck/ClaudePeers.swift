import Foundation

/// Claude Code's own list of the sessions running on this Mac, one file each
/// in the sessions folder of every Claude account (see `ClaudeProfiles`),
/// with the name and socket another session's SendMessage names it by. Only
/// the .json files are read, and only their pid, id, name and socket: never
/// the keys kept beside them.
enum ClaudePeers {
    /// Far more than a session's file holds: a bigger one isn't one, and isn't read.
    private static let mostBytes = 64 * 1024

    /// A session as its file says, and the account it was found under.
    struct Peer: Decodable, Equatable {
        let pid: Int?
        let sessionId: String?
        let name: String?
        let messagingSocketPath: String?
        /// Claude Code's version, as its session file says.
        let version: String?
        /// The account's name; set as it's found, not read from the file.
        var profile: String?

        private enum CodingKeys: String, CodingKey {
            case pid, sessionId, name, messagingSocketPath, version
        }
    }

    /// Every session in `profiles`' sessions folders, each with its account.
    static func all(in profiles: [ClaudeProfile] = ClaudeProfiles.current) -> [Peer] {
        profiles.flatMap { profile -> [Peer] in
            guard let files = try? FileManager.default.contentsOfDirectory(at: profile.sessions, includingPropertiesForKeys: nil)
            else { return [] }
            return files.filter { $0.pathExtension == "json" }.sorted { $0.lastPathComponent < $1.lastPathComponent }
                .compactMap { file -> Peer? in
                    guard let size = (try? file.resourceValues(forKeys: [.fileSizeKey]))?.fileSize, size <= mostBytes,
                          let data = try? Data(contentsOf: file),
                          var peer = try? JSONDecoder().decode(Peer.self, from: data) else { return nil }
                    peer.profile = profile.name
                    return peer
                }
        }
    }

    /// The session a SendMessage `to` names, by its id: a name, with its
    /// "[ref]" when two share it, or "uds:" and a socket. Nil when none, or
    /// more than one, fits, across every account.
    static func sessionID(for target: String, among peers: [Peer]? = nil) -> String? {
        let target = target.trimmingCharacters(in: .whitespaces)
        guard !target.isEmpty else { return nil }
        let peers = peers ?? all()
        let matches: [Peer]
        if target.hasPrefix("uds:") {
            let socket = String(target.dropFirst(4))
            let pid = Int((socket as NSString).lastPathComponent.replacingOccurrences(of: ".sock", with: ""))
            matches = peers.filter { $0.messagingSocketPath == socket || (pid != nil && $0.pid == pid) }
        } else {
            // "name [ref]": the name says who, unless two share it.
            let name = target.split(separator: " [", maxSplits: 1).first.map(String.init) ?? target
            matches = peers.filter { $0.name == name }
        }
        return matches.count == 1 ? matches[0].sessionId : nil
    }
}
