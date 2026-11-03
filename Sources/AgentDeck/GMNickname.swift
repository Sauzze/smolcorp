import Foundation

/// `POST /setup/gm/nickname` with `{folder, name}`: the General Manager names one person in the office, the
/// way the setup page's outfit maker does: only that project's member name in the saved layout changes.
/// The name is trimmed, one line of at most `TeamMember.nicknameLimit` characters; an empty one takes the
/// nickname away. (The GM can already change any desk through /setup/layout, so this gives it no new power.)
enum GMNickname {
    /// What the route needs from the deck: where people sit now and what is pinned.
    struct Place {
        /// A session's project, and the folder it is in now.
        var project: String
        var folder: String
    }

    /// Names the person at `folder` in `layout`, or says why not. The person is whoever a session runs for in
    /// that folder (its project or where it is now) or the project pinned to a desk; the layout names
    /// them by the project's last folder name, as the setup page does.
    static func apply(body: Data, layout: inout Layout, places: [Place], pins: [String?]) -> Response {
        guard let json = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any],
              let folder = (json["folder"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines), !folder.isEmpty,
              let given = json["name"] as? String else {
            return .status(400, "Send {folder, name}: the project's folder, and the nickname (empty for none).")
        }
        let name = given.trimmingCharacters(in: .whitespacesAndNewlines)
        guard name.count <= TeamMember.nicknameLimit, !name.contains(where: { $0.isNewline }) else {
            return .status(400, "A nickname is one line of at most \(TeamMember.nicknameLimit) characters.")
        }
        guard let project = project(for: folder, places: places, pins: pins) else {
            return .status(404, "Nobody works in that folder and nothing is pinned to it, so there is no one to name.")
        }
        let key = Session.lastPart(of: project)
        var member = layout.room.team[key] ?? TeamMember()
        member.name = name
        if member == TeamMember() {
            layout.room.team[key] = nil
        } else {
            guard layout.room.team[key] != nil || layout.room.team.count < Room.teamLimit else {
                return .status(409, "The office remembers \(Room.teamLimit) people already; that is the most.")
            }
            layout.room.team[key] = member
        }
        return .json(["project": project, "nickname": name])
    }

    /// The project a typed folder stands for: a session's project or its folder (compared as the real path, so
    /// /var and /private/var agree), else a pinned project.
    private static func project(for folder: String, places: [Place], pins: [String?]) -> String? {
        func same(_ a: String, _ b: String) -> Bool {
            !a.isEmpty && !b.isEmpty && (a == b || HireDesk.physical(a) == HireDesk.physical(b))
        }
        if let place = places.first(where: { same($0.project, folder) }) ?? places.first(where: { same($0.folder, folder) }) {
            return place.project
        }
        return pins.compactMap { $0 }.first(where: { same($0, folder) })
    }
}
