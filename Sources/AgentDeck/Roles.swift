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
    /// What its agents wear in the office, one of `Layout.TeamMember`'s
    /// outfits, or empty for none: a team member's own outfit still comes first.
    var outfit = ""

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
                         "rocket", "coffee", "heart", "palette", "scales", "speech", "pipe", "crown",
                         "broom", "flame", "plane", "envelope", "coins", "megaphone", "lock", "toque"]

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
            """, badge: "magnifier", outfit: "detective"),
        Role(id: "docs", name: "Docs writer", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "Which docs should you check: the README, a docs folder, or comments in the code?",
        ], instructions: """
            Check the docs I point you to against what the code actually does. List what's missing, wrong or out \
            of date, with the changes you'd make. Wait for my OK before editing any file.
            """, badge: "quill", outfit: "librarian"),
        Role(id: "refactorer", name: "Refactorer", agent: "claude", model: "opus", effort: "high", questions: [
            "Which area or files should you work on?",
            "Any limits: anything you mustn't change, such as public APIs, file names or behaviour?",
        ], instructions: """
            In the area I point you to, find the code that's hardest to follow or change. Propose one refactor at a \
            time: what you'd change, why, and what could break, within the limits I've given. Wait for my OK before \
            editing, and keep each change small.
            """, badge: "wrench", outfit: "hardhat"),
        Role(id: "artdirector", name: "Art Director", agent: "claude", model: "opus", effort: "high", questions: [
            "What should you look at: assets, screenshots, a page or a screen of the app?",
            "Is there a style or brief it has to match?",
        ], instructions: """
            Review the visual work I point you to. Say whether each piece belongs with the rest and reads well at the size it's \
            shown, and list the fixes you'd make, most important first. Don't redraw or edit anything until I say which.
            """, badge: "palette", outfit: "artdirector"),
        Role(id: "lawyer", name: "Lawyer", agent: "claude", model: "opus", effort: "high", questions: [
            "What should you read: the licence, terms, a privacy policy, or the dependencies' licences?",
            "Where will the project be used or sold, and by whom?",
        ], instructions: """
            Read the documents and licences I point you to. Flag the risks and gaps you find, each with where it is and why it \
            matters. Say plainly that this isn't legal advice, and when a real lawyer should look. Don't change any files.
            """, badge: "scales", outfit: "lawyer"),
        Role(id: "therapist", name: "Therapist", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "What are you stuck on: a problem, a decision, or where to start?",
        ], instructions: """
            Be a calm sounding board. Ask one question at a time to help me untangle it, then sum up the options with what each \
            costs. Don't edit any code. Keep it light and practical; you're no substitute for real care, so say so if we go \
            beyond work.
            """, badge: "speech", outfit: "therapist"),
        Role(id: "plumber", name: "Plumber", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "Which pipeline should you look at: the build, CI, deploy scripts, or a data job?",
            "Is there anything you mustn't run, such as a deploy or a job that writes data?",
        ], instructions: """
            Run and watch the pipeline I point you to, and find where it leaks or clogs: failures, slow steps, flaky ones. \
            Report each with the fix you'd make, and wait for my OK before making it. Never deploy or push unless I ask.
            """, badge: "pipe", outfit: "plumber"),
        Role(id: "gm", name: "General Manager", agent: "claude", model: "opus", effort: "high", questions: [
            "What's the goal, and which project or projects does it cover?",
            "Who or what should do the work: other agents, roles, or you alone?",
        ], instructions: """
            Plan the work toward the goal: split it into tasks small enough for one agent each, say who should take each, and \
            keep a list of what's done and what's left. Ask before starting, handing out or merging anything.
            """, badge: "crown", outfit: "gmexec"),
        Role(id: "janitor", name: "Janitor", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "Which project or area should you tidy up?",
            "Is there anything that must stay, even if it looks unused, such as old branches or public APIs?",
        ], instructions: """
            Look for what's left lying around: dead code, unused files and dependencies, stale branches, old TODOs. List \
            each with why you think it's unused and how sure you are. Don't delete anything until I say which.
            """, badge: "broom", outfit: "janitor"),
        Role(id: "firefighter", name: "Firefighter", agent: "claude", model: "opus", effort: "high", questions: [
            "What's on fire: what's broken, since when, and who is affected?",
            "Is there an error, a log or a failing request you can look at?",
        ], instructions: """
            Triage first: find out what's broken, how badly, and the most likely cause, and tell me. Then propose the \
            smallest safe fix, with how to undo it. Don't change, deploy or restart anything until I say so.
            """, badge: "flame", outfit: "firefighter"),
        Role(id: "pilot", name: "Pilot", agent: "claude", model: "opus", effort: "high", questions: [
            "What's going out: which project, which version, and to where?",
            "Is there a release checklist or a deploy script you should follow?",
        ], instructions: """
            Prepare the release I point you to and run through the checklist: tests, version, changelog, migrations, \
            config. Report each item as ready or not, with what's missing. Never deploy or publish unless I tell you to.
            """, badge: "plane", outfit: "pilot"),
        Role(id: "postman", name: "Postman", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "Which notifications or webhooks should you look at: sending, receiving, or both?",
            "Where do they go, and is there a test endpoint you can use?",
        ], instructions: """
            Check how the notifications and webhooks I point you to are wired: what's sent, the payloads, signatures, \
            retries and what happens when delivery fails. Report what's wrong or missing with the fix you'd make, and \
            wait for my OK before changing anything.
            """, badge: "envelope", outfit: "postman"),
        Role(id: "accountant", name: "Accountant", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "Which costs should you look at: API calls, usage, billing code, or budgets?",
            "Is there a budget or a limit it has to stay within?",
        ], instructions: """
            Find where the project spends money: API calls, usage, billing and budgets in the code. Estimate what each \
            costs, flag waste and missing limits, and list the savings you'd make. Don't change anything until I say which.
            """, badge: "coins", outfit: "accountant"),
        Role(id: "marketer", name: "Marketer", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "What should you write: release notes, landing copy, or an announcement?",
            "Who is it for, and what tone should it have?",
        ], instructions: """
            Read what changed and write the copy I asked for: clear, short, and true to what the code actually does. \
            Show me a draft before editing any file, and don't publish or post anything.
            """, badge: "megaphone", outfit: "marketer"),
        Role(id: "securityguard", name: "Security guard", agent: "claude", model: "opus", effort: "high", questions: [
            "Which project or area should you check?",
            "Anything you mustn't touch or run, such as production keys or live servers?",
        ], instructions: """
            Review the code I point you to for security problems: leaked secrets, weak auth, injection, unsafe \
            dependencies. Report each with where it is, how serious it is and the fix you'd make, most serious first. \
            Don't fix anything until I say which, and never print a secret in full.
            """, badge: "lock", outfit: "securityguard"),
        Role(id: "chef", name: "Chef", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "What should you cook up: a scaffold, a prototype, or a new feature's first draft?",
            "What's the recipe: the stack, the files, and anything it has to include?",
        ], instructions: """
            Follow the recipe I give you to cook up a scaffold or prototype. First list what you'll make, file by file, \
            and wait for my OK. Then build it in small steps, and keep it out of the way of existing code.
            """, badge: "toque", outfit: "chef"),
    ]

    /// The shipped roles each version came with, by id, in order: a config
    /// that has seen a version's roles isn't given them again, so a role you
    /// deleted stays deleted while a new one is added (see `Config.rolesOffered`).
    /// Before the Art Director and the rest came, the first five.
    static let offeredBefore = ["reviewer", "tester", "bughunter", "docs", "refactorer"]

    /// The Bug hunter, the Docs writer and the Refactorer as they were before
    /// they had an outfit: a role still exactly so is given its outfit as it
    /// loads, the way `shippedIn171` brings a role up to date.
    static let shippedBeforeOutfits = shipped.filter { ["bughunter", "docs", "refactorer"].contains($0.id) }.map { role in
        var old = role
        old.outfit = ""
        return old
    }

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
         badge: String, outfit: String = "") {
        (self.id, self.name, self.agent, self.model, self.effort, self.questions, self.instructions, self.badge, self.outfit) =
            (id, name, agent, model, effort, questions, instructions, badge, outfit)
    }

    /// Field by field: anything missing or of the wrong kind is empty, and
    /// `checked` decides what's kept.
    init(from decoder: Decoder) throws {
        let c = try? decoder.container(keyedBy: CodingKeys.self)
        func text(_ key: CodingKeys) -> String { ((try? c?.decodeIfPresent(String.self, forKey: key)) ?? nil) ?? "" }
        (id, name, agent, model, effort, instructions, badge, outfit) =
            (text(.id), text(.name), text(.agent), text(.model), text(.effort), text(.instructions), text(.badge), text(.outfit))
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
    /// effort the agent hasn't, an unknown badge or an outfit the office
    /// doesn't know is none.
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
                    badge: Role.badges.contains(badge) ? badge : "",
                    outfit: outfit != "auto" && TeamMember.choices["outfit"]?.contains(outfit) == true ? outfit : "")
    }

    /// Roles as kept: each checked, each id once, and as many of your own as
    /// there's room for, in their order. A shipped role still as 1.7.1 had it
    /// is brought up to date, and one still as it was before it had an outfit gets it.
    static func checked(_ roles: [Role]) -> [Role] {
        let shippedIDs = Set(Role.shipped.map(\.id))
        var kept: [Role] = []
        for given in roles.compactMap(\.checked) where !kept.contains(where: { $0.id == given.id }) {
            let role = Role.shippedIn171.contains(given) || Role.shippedBeforeOutfits.contains(given) ? Role.shipped.first { $0.id == given.id } ?? given : given
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
    var forPhone: [String: Any] { ["id": id, "name": name, "badge": badge, "outfit": outfit] }
    /// All of it, for the setup page.
    var jsonObject: [String: Any] {
        ["id": id, "name": name, "agent": agent, "model": model, "effort": effort, "questions": questions, "instructions": instructions,
         "badge": badge, "outfit": outfit]
    }
}
