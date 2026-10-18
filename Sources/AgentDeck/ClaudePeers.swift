import Foundation

/// Claude Code's own list of the sessions running on this Mac, one file each
/// in ~/.claude/sessions, with the name and socket another session's
/// SendMessage names it by. Only the .json files are read, and only their
/// pid, id, name and socket: never the keys kept beside them.
enum ClaudePeers {
    private static let folder = Paths.home.appendingPathComponent(".claude/sessions", isDirectory: true)
    /// Far more than a session's file holds: a bigger one isn't one, and isn't read.
    private static let mostBytes = 64 * 1024

    private struct Peer: Decodable {
        let pid: Int?
        let sessionId: String?
        let name: String?
        let messagingSocketPath: String?
    }

    /// The session a SendMessage `to` names, by its id: a name, with its
    /// "[ref]" when two share it, or "uds:" and a socket. Nil when none, or
    /// more than one, fits.
    static func sessionID(for target: String) -> String? {
        let target = target.trimmingCharacters(in: .whitespaces)
        guard !target.isEmpty, let files = try? FileManager.default.contentsOfDirectory(at: folder, includingPropertiesForKeys: nil)
        else { return nil }
        let peers = files.filter { $0.pathExtension == "json" }.compactMap { file -> Peer? in
            guard let size = (try? file.resourceValues(forKeys: [.fileSizeKey]))?.fileSize, size <= mostBytes,
                  let data = try? Data(contentsOf: file) else { return nil }
            return try? JSONDecoder().decode(Peer.self, from: data)
        }
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
