import Foundation

/// A role the office can hire: its name, the agent it is and how that agent
/// starts (its model, and its effort where it has one), the instructions it
/// starts on, and the badge it wears in the office. Plain text you own, kept
/// on this Mac: nothing runs until the office hires one, which hiring has to
/// be on for. The instructions are the agent's first message, there to read
/// in its Terminal window, and nothing changes what the agent may do
/// without asking.
struct Role: Codable, Equatable {
    var id = ""
    var name = ""
    var agent = Agent.claude.rawValue
    /// Empty for the agent's own default.
    var model = ""
    /// Empty for the agent's own, or one of `Deck.effortLevels` for an agent
    /// with an effort setting.
    var effort = ""
    var instructions = ""
    var badge = ""

    static let nameLimit = 24
    static let modelLimit = 64
    static let instructionsLimit = 4000
    /// The most roles of your own, besides the ones that come with the app.
    static let ownMost = 12
    /// What a role can wear in the office: the shipped roles' own, then some
    /// for yours. office.js draws them.
    static let badges = ["glasses", "clipboard", "magnifier", "quill", "wrench", "star", "bulb", "headset", "flag", "shield",
                         "rocket", "coffee", "heart"]

    /// The roles that come with the app. Each reports and asks before it
    /// changes anything.
    static let shipped = [
        Role(id: "reviewer", name: "Reviewer", agent: "claude", model: "opus", effort: "high", instructions: """
            Review the changes on this branch against main. List the problems you find, most serious first, each with \
            its file and line and why it matters. Don't edit any files: report, and wait for me to say which to fix.
            """, badge: "glasses"),
        Role(id: "tester", name: "Tester", agent: "claude", model: "sonnet", effort: "medium", instructions: """
            Run this project's tests. If any fail, find out why, and explain each failure with the fix you'd make. \
            Don't change any code until I say so.
            """, badge: "clipboard"),
        Role(id: "bughunter", name: "Bug hunter", agent: "claude", model: "opus", effort: "high", instructions: """
            Look through this project for bugs: wrong results, crashes, edge cases that aren't handled. Report each \
            one with how to reproduce it and how sure you are. Don't fix anything until I say which.
            """, badge: "magnifier"),
        Role(id: "docs", name: "Docs writer", agent: "claude", model: "sonnet", effort: "medium", instructions: """
            Check the README and the docs against what the code actually does. List what's missing, wrong or out of \
            date, with the changes you'd make. Wait for my OK before editing any file.
            """, badge: "quill"),
        Role(id: "refactorer", name: "Refactorer", agent: "claude", model: "opus", effort: "high", instructions: """
            Find the code in this project that's hardest to follow or change. Propose one refactor at a time: what \
            you'd change, why, and what could break. Wait for my OK before editing, and keep each change small.
            """, badge: "wrench"),
    ]

    init(id: String, name: String, agent: String, model: String, effort: String, instructions: String, badge: String) {
        (self.id, self.name, self.agent, self.model, self.effort, self.instructions, self.badge) =
            (id, name, agent, model, effort, instructions, badge)
    }

    /// Field by field: anything missing or of the wrong kind is empty, and
    /// `checked` decides what's kept.
    init(from decoder: Decoder) throws {
        let c = try? decoder.container(keyedBy: CodingKeys.self)
        func text(_ key: CodingKeys) -> String { ((try? c?.decodeIfPresent(String.self, forKey: key)) ?? nil) ?? "" }
        (id, name, agent, model, effort, instructions, badge) =
            (text(.id), text(.name), text(.agent), text(.model), text(.effort), text(.instructions), text(.badge))
    }

    /// The role as kept, or nil without an id or a name. An unknown agent is
    /// Claude Code; a model with anything but a model name's characters, an
    /// effort the agent hasn't, or an unknown badge is none.
    var checked: Role? {
        let id = id.trimmingCharacters(in: .whitespaces)
        let name = String(name.split(whereSeparator: \.isNewline).joined(separator: " ")
            .trimmingCharacters(in: .whitespaces).prefix(Role.nameLimit)).trimmingCharacters(in: .whitespaces)
        guard (1...32).contains(id.count), id.allSatisfy({ ("a"..."z").contains($0) || ("0"..."9").contains($0) || $0 == "-" }),
              !name.isEmpty else { return nil }
        let agent = Agent(rawValue: agent) ?? .claude
        let model = model.trimmingCharacters(in: .whitespaces)
        let modelCharacters = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "-._/:[]"))
        let keptModel = model.count <= Role.modelLimit && model.unicodeScalars.allSatisfy { $0.isASCII && modelCharacters.contains($0) }
            ? model : ""
        // Lines and tabs stay; other control characters go.
        let instructions = String(String.UnicodeScalarView(instructions.unicodeScalars.filter {
            $0 == "\n" || $0 == "\t" || !CharacterSet.controlCharacters.contains($0)
        })).trimmingCharacters(in: .whitespacesAndNewlines)
        return Role(id: id, name: name, agent: agent.rawValue, model: keptModel,
                    effort: agent.hasEffortDial && Deck.effortLevels.contains(effort) ? effort : "",
                    instructions: String(instructions.prefix(Role.instructionsLimit)),
                    badge: Role.badges.contains(badge) ? badge : "")
    }

    /// Roles as kept: each checked, each id once, and as many of your own as
    /// there's room for, in their order.
    static func checked(_ roles: [Role]) -> [Role] {
        let shippedIDs = Set(Role.shipped.map(\.id))
        var kept: [Role] = []
        for role in roles.compactMap(\.checked) where !kept.contains(where: { $0.id == role.id }) {
            if !shippedIDs.contains(role.id) && kept.filter({ !shippedIDs.contains($0.id) }).count == Role.ownMost { continue }
            kept.append(role)
        }
        return kept
    }

    /// What the agent is first told: to settle which folder it works in, then
    /// the role's instructions. A hire mostly starts in the folder set up for
    /// new hires, such as ~/Projects, rather than in a project; at a desk
    /// pinned to a project, it starts in that project's.
    func firstMessage(in folder: String, pinned: Bool) -> String {
        let shown = (folder as NSString).abbreviatingWithTildeInPath
        let settle = pinned
            ? "You've started in \(shown), this desk's project. Before anything else, tell me that's where you'll work and wait for me to confirm it."
            : "You've started in \(shown). Before anything else, ask me which project or folder to work in, confirm it with me, and work only there."
        return instructions.isEmpty ? settle : "\(settle)\n\nThen, as my \(name): \(instructions)"
    }

    /// How the role's agent starts: its model, its effort, and `message` as
    /// its first. Each option is written whole, with its value after `=`, and
    /// the message after `--` where it stands alone, so text starting with a
    /// dash is never read as an option. Grok starts as it is: how it takes
    /// these hasn't been checked.
    func arguments(firstMessage message: String) -> [String] {
        let agent = Agent(rawValue: agent) ?? .claude
        switch agent {
        case .claude:
            return (model.isEmpty ? [] : ["--model=\(model)"]) + (effort.isEmpty ? [] : ["--effort=\(effort)"]) + ["--", message]
        case .codex:
            return (model.isEmpty ? [] : ["--model=\(model)"]) + ["--", message]
        case .gemini:
            return (model.isEmpty ? [] : ["--model=\(model)"]) + ["--prompt-interactive=\(message)"]
        case .opencode:
            return (model.isEmpty ? [] : ["--model=\(model)"]) + ["--prompt=\(message)"]
        case .grok:
            return []
        }
    }

    /// A few lines that start the role's agent from a hire's Terminal window:
    /// they remove themselves, then become the agent. Each of the agent's
    /// words is quoted for the shell, so the instructions are only ever the
    /// agent's message, never a command.
    func startScript(in folder: String, pinned: Bool) -> String {
        let words = [(Agent(rawValue: agent) ?? .claude).rawValue] + arguments(firstMessage: firstMessage(in: folder, pinned: pinned))
        let quoted = words.map { "'" + $0.replacingOccurrences(of: "'", with: "'\\''") + "'" }.joined(separator: " ")
        return "#!/bin/sh\n# A hire from Smolcorp's office, as a \(name): runs once.\nrm -f \"$0\"\nexec \(quoted)\n"
    }

    /// What the phone is told of a role: never its instructions.
    var forPhone: [String: Any] { ["id": id, "name": name, "badge": badge] }
    /// All of it, for the setup page.
    var jsonObject: [String: Any] {
        ["id": id, "name": name, "agent": agent, "model": model, "effort": effort, "instructions": instructions, "badge": badge]
    }
}
