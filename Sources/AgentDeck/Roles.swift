import Foundation

/// A role the office can hire: its name, the agent it is and how that agent
/// starts (its model, and its effort where it has one), what it asks you
/// before it starts, the instructions it works on once you've answered, and
/// the badge it wears in the office. Plain text you own, kept on this Mac:
/// nothing runs until the office hires one, which hiring has to be on for.
/// All of it is the agent's first message, there to read in its Terminal
/// window, and nothing changes what the agent may do without asking.
struct Role: Codable, Equatable {
    var id = ""
    var name = ""
    var agent = Agent.claude.rawValue
    /// Empty for the agent's own default.
    var model = ""
    /// Empty for the agent's own, or one of `Deck.effortLevels` for an agent
    /// with an effort setting.
    var effort = ""
    /// What it asks you before it starts, after which project to work on.
    var questions: [String] = []
    var instructions = ""
    var badge = ""

    static let nameLimit = 24
    static let modelLimit = 64
    static let instructionsLimit = 4000
    /// The most questions a role asks, and the longest each can be.
    static let questionsMost = 8
    static let questionLimit = 200
    /// The most roles of your own, besides the ones that come with the app.
    static let ownMost = 12
    /// What a role can wear in the office: the shipped roles' own, then some
    /// for yours. office.js draws them.
    static let badges = ["glasses", "clipboard", "magnifier", "quill", "wrench", "star", "bulb", "headset", "flag", "shield",
                         "rocket", "coffee", "heart"]

    /// The roles that come with the app. Each asks what to work on before it
    /// starts, then reports and asks before it changes anything.
    static let shipped = [
        Role(id: "reviewer", name: "Reviewer", agent: "claude", model: "opus", effort: "high", questions: [
            "Which changes should you review: a branch, a pull request, or a range of commits?",
            "What should they be compared with, for example main, or the commit before?",
            "What matters most this time: bugs, security, performance, style, or all of them?",
        ], instructions: """
            Review the changes I point you to. List the problems you find, most serious first, each with its file \
            and line and why it matters. Don't edit any files: report, and wait for me to say which to fix.
            """, badge: "glasses"),
        Role(id: "tester", name: "Tester", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "Which tests should you run: all of them, a folder, or a single file?",
            "Is there anything to set up or start first, such as a database, a server or environment variables?",
        ], instructions: """
            Run the tests I point you to. If any fail, find out why, and explain each failure with the fix you'd \
            make. Don't change any code until I say so.
            """, badge: "clipboard"),
        Role(id: "bughunter", name: "Bug hunter", agent: "claude", model: "opus", effort: "high", questions: [
            "Which area or feature should you focus on, or should you look everywhere?",
        ], instructions: """
            Look for bugs where I point you to: wrong results, crashes, edge cases that aren't handled. Report each \
            one with how to reproduce it and how sure you are. Don't fix anything until I say which.
            """, badge: "magnifier"),
        Role(id: "docs", name: "Docs writer", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "Which docs should you check: the README, a docs folder, or comments in the code?",
        ], instructions: """
            Check the docs I point you to against what the code actually does. List what's missing, wrong or out \
            of date, with the changes you'd make. Wait for my OK before editing any file.
            """, badge: "quill"),
        Role(id: "refactorer", name: "Refactorer", agent: "claude", model: "opus", effort: "high", questions: [
            "Which area or files should you work on?",
            "Any limits: anything you mustn't change, such as public APIs, file names or behaviour?",
        ], instructions: """
            In the area I point you to, find the code that's hardest to follow or change. Propose one refactor at a \
            time: what you'd change, why, and what could break, within the limits I've given. Wait for my OK before \
            editing, and keep each change small.
            """, badge: "wrench"),
    ]

    /// The shipped roles as 1.7.1 had them, which assumed what to work on. A
    /// role still exactly so is brought up to date as it loads; one you've
    /// changed is left as you have it.
    static let shippedIn171 = [
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

    init(id: String, name: String, agent: String, model: String, effort: String, questions: [String] = [], instructions: String,
         badge: String) {
        (self.id, self.name, self.agent, self.model, self.effort, self.questions, self.instructions, self.badge) =
            (id, name, agent, model, effort, questions, instructions, badge)
    }

    /// Field by field: anything missing or of the wrong kind is empty, and
    /// `checked` decides what's kept.
    init(from decoder: Decoder) throws {
        let c = try? decoder.container(keyedBy: CodingKeys.self)
        func text(_ key: CodingKeys) -> String { ((try? c?.decodeIfPresent(String.self, forKey: key)) ?? nil) ?? "" }
        (id, name, agent, model, effort, instructions, badge) =
            (text(.id), text(.name), text(.agent), text(.model), text(.effort), text(.instructions), text(.badge))
        // Missing from a role kept before roles asked questions; text among them is kept.
        questions = (((try? c?.decodeIfPresent([Loose].self, forKey: .questions)) ?? nil) ?? []).compactMap(\.text)
    }

    /// One of the questions as given: text, or nothing for any other kind of value.
    private struct Loose: Decodable {
        let text: String?
        init(from decoder: Decoder) throws { text = try? decoder.singleValueContainer().decode(String.self) }
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
        // Each question one line, without control characters; empty ones go.
        let questions = questions.map { question in
            String(String(String.UnicodeScalarView(question.unicodeScalars.filter { !CharacterSet.controlCharacters.contains($0) }))
                .trimmingCharacters(in: .whitespaces).prefix(Role.questionLimit)).trimmingCharacters(in: .whitespaces)
        }.filter { !$0.isEmpty }
        return Role(id: id, name: name, agent: agent.rawValue, model: keptModel,
                    effort: agent.hasEffortDial && Deck.effortLevels.contains(effort) ? effort : "",
                    questions: Array(questions.prefix(Role.questionsMost)),
                    instructions: String(instructions.prefix(Role.instructionsLimit)),
                    badge: Role.badges.contains(badge) ? badge : "")
    }

    /// Roles as kept: each checked, each id once, and as many of your own as
    /// there's room for, in their order. A shipped role still as 1.7.1 had it
    /// is brought up to date.
    static func checked(_ roles: [Role]) -> [Role] {
        let shippedIDs = Set(Role.shipped.map(\.id))
        var kept: [Role] = []
        for given in roles.compactMap(\.checked) where !kept.contains(where: { $0.id == given.id }) {
            let role = Role.shippedIn171.contains(given) ? Role.shipped.first { $0.id == given.id } ?? given : given
            if !shippedIDs.contains(role.id) && kept.filter({ !shippedIDs.contains($0.id) }).count == Role.ownMost { continue }
            kept.append(role)
        }
        return kept
    }

    /// What the agent is first told: to only ask, in its first reply, which
    /// project to work on and the role's own questions, and to wait for the
    /// answers; then the role's instructions. A hire mostly starts in the
    /// folder set up for new hires, such as ~/Projects, rather than in a
    /// project; at a desk pinned to a project, or started where you chose, it
    /// asks you to confirm that folder instead.
    func firstMessage(in folder: String, pinned: Bool) -> String {
        let shown = (folder as NSString).abbreviatingWithTildeInPath
        let project = pinned
            ? "You've started in \(shown), this desk's project. Is that the one to work on?"
            : "Which project should you work on: a folder on this computer, or a GitHub repository?"
        let asked = ([project] + questions).enumerated().map { "\($0.offset + 1). \($0.element)" }.joined(separator: "\n")
        let then = instructions.isEmpty ? "Once I've answered, ask me what to do." : "Once I've answered, as my \(name): \(instructions)"
        return (pinned ? "" : "You've started in \(shown), a general folder, not a project. ")
            + "Your first reply must only ask me the questions below, all together. Don't read code, run commands or start any "
            + "work until I've answered them.\n\n\(asked)\n\n\(then)"
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
        ["id": id, "name": name, "agent": agent, "model": model, "effort": effort, "questions": questions, "instructions": instructions,
         "badge": badge]
    }
}
