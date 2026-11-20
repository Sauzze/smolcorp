import Foundation

/// A role the office can hire: its name, the agent it is and how that agent
/// starts (its model, and its effort where it has one), what it asks you
/// before it starts, the instructions it works on once you've answered, and
/// the badge it wears in the office. Plain text you own, kept on this Mac:
/// nothing runs until the office hires one, which hiring has to be on for.
/// All of it is the agent's first message, there to read in its Terminal
/// window, or with a brief from the library or a file, in the BRIEF.md it's
/// sent to; and nothing changes what the agent may do without asking.
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
    /// Where its brief comes from, or nil as kept before roles had briefs:
    /// then `source` is what it's hired on.
    var brief: Brief?

    static let nameLimit = 24
    static let modelLimit = 64
    static let instructionsLimit = 4000
    /// The most questions a role asks, and the longest each can be.
    static let questionsMost = 8
    static let questionLimit = 200
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
            "What's the main thing someone must be able to do, for you to try by hand too?",
            "What must be set up or started first, such as a database or a server? Say what's needed; don't send keys or passwords.",
        ], instructions: """
            Run the tests I point you to. If any fail, find out why, and explain each failure with the fix you'd \
            make. Don't change any code until I say so.
            """, badge: "clipboard"),
        Role(id: "bughunter", name: "Bug hunter", agent: "claude", model: "opus", effort: "high", questions: [
            "Which area or feature should you focus on, or should you look everywhere?",
            "Which devices, systems or browsers matter, and which of them can run on this computer?",
            "Which known bugs should you skip, and how serious must a bug be to report it?",
        ], instructions: """
            Look for bugs where I point you to: wrong results, crashes, edge cases that aren't handled. Report each \
            one with how to reproduce it and how sure you are. Don't fix anything until I say which.
            """, badge: "magnifier", outfit: "detective"),
        Role(id: "docs", name: "Docs writer", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "Which docs should you check: the README, a docs folder, or comments in the code?",
            "Who reads them (newcomers, developers, or both), and in which languages?",
            "Should you only list what's wrong, or fix it too? Anything that must stay as it is?",
        ], instructions: """
            Check the docs I point you to against what the code actually does. List what's missing, wrong or out \
            of date, with the changes you'd make. Wait for my OK before editing any file.
            """, badge: "quill", outfit: "librarian"),
        Role(id: "refactorer", name: "Refactorer", agent: "claude", model: "opus", effort: "high", questions: [
            "Which area or files should you work on?",
            "What mustn't change, such as public APIs, file names or behaviour?",
            "Which command runs the tests and checks that prove nothing changed?",
        ], instructions: """
            In the area I point you to, find the code that's hardest to follow or change. Propose one refactor at a \
            time: what you'd change, why, and what could break, within the limits I've given. Wait for my OK before \
            editing, and keep each change small.
            """, badge: "wrench", outfit: "hardhat"),
        Role(id: "artdirector", name: "Art Director", agent: "claude", model: "opus", effort: "high", questions: [
            "What should you look at (assets, a page, a screen of the app), and where is each piece shown?",
            "What must it match: a style guide, a brief, or the pieces already accepted?",
            "How can you see it in place: a page or command that runs here, and in which modes (day and night, light and dark)?",
        ], instructions: """
            Review the visual work I point you to. Say whether each piece belongs with the rest and reads well at the size it's \
            shown, and list the fixes you'd make, most important first. Don't redraw or edit anything until I say which.
            """, badge: "palette", outfit: "artdirector"),
        Role(id: "lawyer", name: "Lawyer", agent: "claude", model: "opus", effort: "high", questions: [
            "What should you read: the licence, terms, a privacy policy, or the dependencies' licences?",
            "How is the project used: open source, sold, or only inside a company, and in which countries?",
            "What is this read for: a release, a sale, a new dependency, or a check-up?",
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
            "Which steps mustn't you run, such as a deploy or a job that writes data?",
            "Can you see CI's recent logs here, and if so, where?",
        ], instructions: """
            Run and watch the pipeline I point you to, and find where it leaks or clogs: failures, slow steps, flaky ones. \
            Report each with the fix you'd make, and wait for my OK before making it. Never deploy or push unless I ask.
            """, badge: "pipe", outfit: "plumber"),
        Role(id: "gm", name: "General Manager", agent: "claude", model: "opus", effort: "high", questions: [
            "What's the goal, and which project or projects does it cover?",
            "What may the team spend (plans, API keys, cloud time), and which agents and accounts may it use?",
            "What do you want to approve yourself, beyond push, publish, deploy, spending and deleting?",
        ], instructions: """
            Run a team of agents toward the goal. First plan it: tasks one agent can finish and prove in one sitting, and \
            wait for my OK on the plan. Then hire one agent per task, each in its own worktree or folder, branch and port \
            range, with a brief that says what done means and what proof to show. Check that each hire really started, check \
            their proof yourself, and have a second agent review any change before it merges. Decide the team and process \
            calls yourself; ask me before anything is pushed, merged to main, published, deployed, released, deleted or paid \
            for, or reaches a real person. Never type into another window, never ask an employee to do what your own checks \
            refused, and when an employee is done, end its session, make sure it's gone, and close its window.
            """, badge: "crown", outfit: "gmexec"),
        Role(id: "janitor", name: "Janitor", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "Which project or area should you tidy up?",
            "What must stay even if it looks unused, such as old branches, public APIs or files other apps load?",
            "Do other projects, scripts or apps call into this one, and where are they?",
        ], instructions: """
            Look for what's left lying around: dead code, unused files and dependencies, stale branches, old TODOs. List \
            each with why you think it's unused and how sure you are. Don't delete anything until I say which.
            """, badge: "broom", outfit: "janitor"),
        Role(id: "firefighter", name: "Firefighter", agent: "claude", model: "opus", effort: "high", questions: [
            "What's broken, since when, and who is affected?",
            "Which error, log or failing request can you look at?",
            "What will do for now (stop the harm, undo a recent change, or only find the cause), and what mustn't you restart or touch?",
        ], instructions: """
            Triage first: find out what's broken, how badly, and the most likely cause, and tell me. Then propose the \
            smallest safe fix, with how to undo it. Don't change, deploy or restart anything until I say so.
            """, badge: "flame", outfit: "firefighter"),
        Role(id: "pilot", name: "Pilot", agent: "claude", model: "opus", effort: "high", questions: [
            "What's going out: which project, which version, and to where?",
            "Is there a release checklist or a deploy script to follow, and what was the last release's tag?",
            "Which commands build and test it here, and is there anything you mustn't run?",
        ], instructions: """
            Prepare the release I point you to and run through the checklist: tests, version, changelog, migrations, \
            config. Report each item as ready or not, with what's missing, and wait for my OK before changing anything. \
            Never deploy or publish unless I tell you to.
            """, badge: "plane", outfit: "pilot"),
        Role(id: "postman", name: "Postman", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "Which notifications or webhooks should you look at: sending, receiving, or both?",
            "Where do they go, and is there a test endpoint you may send to?",
        ], instructions: """
            Check how the notifications and webhooks I point you to are wired: what's sent, the payloads, signatures, \
            retries and what happens when delivery fails. Report what's wrong or missing with the fix you'd make, and \
            wait for my OK before changing anything.
            """, badge: "envelope", outfit: "postman"),
        Role(id: "accountant", name: "Accountant", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "Which costs should you cover: paid APIs and models, hosting and storage, billing code, or all of them?",
            "How much is it used today (calls, users or a monthly bill), and is there a budget it must stay under?",
        ], instructions: """
            Find where the project spends money: API calls, usage, billing and budgets in the code. Estimate what each \
            costs, flag waste and missing limits, and list the savings you'd make. Don't change anything until I say which.
            """, badge: "coins", outfit: "accountant"),
        Role(id: "marketer", name: "Marketer", agent: "claude", model: "sonnet", effort: "medium", questions: [
            "What should you write (release notes, landing copy, or an announcement), where will it appear, and how long may it be?",
            "Who is it for, in which language, and in what tone?",
            "Which changes does it cover: a version, a range of commits, or a feature?",
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
            "What should you make (a scaffold, a prototype, or a feature's first draft), and where in the project should it live?",
            "What's the spec: the stack, the files, and what it must include?",
            "What may stay a stub for now, and what must really work?",
        ], instructions: """
            Follow the recipe I give you to cook up a scaffold or prototype. First list what you'll make, file by file, \
            and wait for my OK. Then build it in small steps, and keep it out of the way of existing code.
            """, badge: "toque", outfit: "chef"),
        Role(id: "thinker", name: "Thinker", agent: "claude", model: "opus", effort: "max", questions: [
            "What should you think about: a question about this project, or a broader one?",
            "What should your answer help me decide or do?",
            "Anything to read first, any limits, or ideas already ruled out?",
        ], instructions: """
            Think the question I give you through before doing anything else. Read what matters in this project, and \
            on the web if it helps. Write your thinking to a file: the options, their trade-offs, what you recommend \
            and why, the open questions, and your sources. Don't change any code: report, and wait for my OK on any \
            next step.
            """, badge: "bulb", outfit: "professor"),
        Role(id: "builder", name: "Builder", agent: "claude", model: "sonnet", effort: "high", questions: [
            "What should you build, and who will use it?",
            "What must it do or include? List the things that make it \"done\".",
            "What should you build from (a design, a study, an existing page), and what mustn't you touch, such as payments, sign-in or stored data?",
        ], instructions: """
            Build what I describe, on a branch of its own. First list what you'll make or change, file by file, and wait \
            for my OK. Then build it in small steps, and check it in the real thing, not only in tests. Show me what \
            you did and how you checked it. Never push, merge, publish or deploy unless I tell you to.
            """, badge: "rocket"),
        Role(id: "designer", name: "Designer", agent: "claude", model: "opus", effort: "high", questions: [
            "What should you design: a page, a screen, a site, or a whole look?",
            "Who is it for, and what should it feel like?",
            "Is there a current version to start from, and anything that must stay (name, logo, contact details) or that you should avoid?",
        ], instructions: """
            Design what I point you to: one strong direction, proven with real HTML/CSS mockups built from real \
            content, beside screenshots of what's there today. First tell me the direction in a few lines and wait for my \
            OK. Keep the mockups next to your brief and leave the project's own code alone. Never push, merge, publish or \
            deploy unless I tell you to.
            """, badge: "star"),
        Role(id: "minimanager", name: "Mini Manager", agent: "claude", model: "opus", effort: "high", questions: [
            "What's the goal for this run, and what's out of scope?",
            "Who already works on it (people, agents, folders, branches), and who hires the people it needs?",
            "What do you want to approve yourself, beyond push, publish, deploy, spending and deleting? What counts as proof that a piece is done?",
        ], instructions: """
            Run one project for me. First settle the goal with me and plan it in pieces one person can finish and \
            prove, then show me the plan and wait for my OK. You don't write the project's code and you don't hire anyone: \
            write each piece's brief and ask the General Manager, or me, for each hire. Check each piece's proof yourself \
            and have a second agent review it before you call it done. Keep a board of the team, the proof and what needs \
            me. Never push, merge, publish or deploy, and never pass an approval on as if it came from me.
            """, badge: "flag"),
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
        old.questions = questionsBefore126[role.id] ?? questionsBeforeOutfits[role.id] ?? role.questions
        return old
    }

    /// What the Refactorer asked before it had an outfit, and until its brief
    /// asked for the command that proves nothing changed.
    static let questionsBeforeOutfits = [
        "refactorer": [
            "Which area or files should you work on?",
            "Any limits: anything you mustn't change, such as public APIs, file names or behaviour?",
        ],
    ]

    /// What the Bug hunter, the Tester, the Docs writer and the Firefighter
    /// asked before 1.26 gave each a sharper brief and more to ask.
    static let questionsBefore126 = [
        "bughunter": ["Which area or feature should you focus on, or should you look everywhere?"],
        "tester": [
            "Which tests should you run: all of them, a folder, or a single file?",
            "Is there anything to set up or start first, such as a database, a server or environment variables?",
        ],
        "docs": ["Which docs should you check: the README, a docs folder, or comments in the code?"],
        "firefighter": [
            "What's on fire: what's broken, since when, and who is affected?",
            "Is there an error, a log or a failing request you can look at?",
        ],
    ]

    /// Those four roles as they came before 1.26: one still exactly so, on
    /// its own brief from the library or none, is brought up to date as it
    /// loads; one you've changed is left as you have it.
    static let shippedBefore126 = shipped.compactMap { role -> Role? in
        guard let questions = questionsBefore126[role.id] else { return nil }
        var old = role
        old.questions = questions
        return old
    }

    /// The General Manager as it came before it ran a team, when it only
    /// planned. Still exactly so, its brief aside, it's brought up to date as
    /// it loads; one you've changed is left as you have it.
    static let shippedBeforeTeams = [
        Role(id: "gm", name: "General Manager", agent: "claude", model: "opus", effort: "high", questions: [
            "What's the goal, and which project or projects does it cover?",
            "Who or what should do the work: other agents, roles, or you alone?",
        ], instructions: """
            Plan the work toward the goal: split it into tasks small enough for one agent each, say who should take each, and \
            keep a list of what's done and what's left. Ask before starting, handing out or merging anything.
            """, badge: "crown", outfit: "gmexec"),
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
         badge: String, outfit: String = "", brief: Brief? = nil) {
        (self.id, self.name, self.agent, self.model, self.effort, self.questions, self.instructions, self.badge, self.outfit) =
            (id, name, agent, model, effort, questions, instructions, badge, outfit)
        self.brief = brief
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
        // Missing from a role kept before roles had briefs, which `Role.checked` gives one.
        brief = (try? c?.decodeIfPresent(Brief.self, forKey: .brief)) ?? nil
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
        // Each question one line, without control characters; empty ones go.
        let questions = questions.map { question in
            String(String(String.UnicodeScalarView(question.unicodeScalars.filter { !CharacterSet.controlCharacters.contains($0) }))
                .trimmingCharacters(in: .whitespaces).prefix(Role.questionLimit)).trimmingCharacters(in: .whitespaces)
        }.filter { !$0.isEmpty }
        // Lines and tabs stay; other control characters go.
        func cleaned(_ text: String) -> String {
            String(String(String.UnicodeScalarView(text.unicodeScalars.filter {
                $0 == "\n" || $0 == "\t" || !CharacterSet.controlCharacters.contains($0)
            })).trimmingCharacters(in: .whitespacesAndNewlines).prefix(Role.instructionsLimit))
        }
        var kept = cleaned(instructions)
        // A brief that can't be one is none. One on its own instructions has them as its text, or gives them its own.
        var brief = brief
        switch brief?.kind {
        case .library: if !Brief.libraryIDs.contains(brief!.id) { brief = nil }
        case .file: brief = try? Brief.file(brief!.text, named: brief!.fileName)
        case .text:
            if kept.isEmpty { kept = cleaned(brief!.text) }
            brief = .text(kept)
        case nil: break
        }
        return Role(id: id, name: name, agent: agent.rawValue, model: keptModel,
                    effort: agent.hasEffortDial && Deck.effortLevels.contains(effort) ? effort : "",
                    questions: Array(questions.prefix(Role.questionsMost)),
                    instructions: kept,
                    badge: Role.badges.contains(badge) ? badge : "",
                    outfit: outfit != "auto" && TeamMember.choices["outfit"]?.contains(outfit) == true ? outfit : "", brief: brief)
    }

    /// The brief a role kept without one is hired on: a shipped role still
    /// on its shipped instructions, its own from the library; any other, its
    /// instructions, as it has always started, so an edit of yours stays yours.
    var defaultBrief: Brief {
        guard let library = Brief.libraryID(ofRole: id), Role.shipped.contains(where: { $0.id == id && $0.instructions == instructions })
        else { return .text(instructions) }
        return .library(library)
    }

    /// What it's hired on.
    var source: Brief { brief ?? defaultBrief }

    /// Roles as kept: each checked, each id once, and every one of your own,
    /// in their order. A shipped role still as 1.7.1 had it
    /// is brought up to date, and one still as it was before it had an outfit
    /// gets it. Each has a brief: one kept without is given its `defaultBrief`.
    static func checked(_ roles: [Role]) -> [Role] {
        let shippedIDs = Set(Role.shipped.map(\.id))
        var kept: [Role] = []
        for given in roles.compactMap(\.checked) where !kept.contains(where: { $0.id == given.id }) {
            var plain = given
            plain.brief = nil
            // One from before 1.26 sharpened it, on its own library brief or none: another brief is yours, and stays.
            let before126 = Role.shippedBefore126.contains(plain) && (given.brief == nil || given.brief == .library(given.id))
            let old = Role.shippedIn171.contains(given) || Role.shippedBeforeOutfits.contains(given) || Role.shippedBeforeTeams.contains(plain)
                || before126
            var role = old ? Role.shipped.first { $0.id == given.id } ?? given : given
            role.brief = role.source
            kept.append(role)
        }
        return kept
    }

    /// What the phone is told of a role: never its instructions.
    var forPhone: [String: Any] { ["id": id, "name": name, "badge": badge, "outfit": outfit] }
    /// All of it, for the setup page.
    var jsonObject: [String: Any] {
        ["id": id, "name": name, "agent": agent, "model": model, "effort": effort, "questions": questions, "instructions": instructions,
         "badge": badge, "outfit": outfit, "brief": source.jsonObject]
    }

    /// Roles as setup sends them, with a brief on their own instructions
    /// kept the same text as those: whichever of the two changed from
    /// `saved` wins, the brief's text if both did. (A page from before
    /// briefs sends the brief's text as it was, and changes the instructions.)
    /// A library role whose instructions changed is on them from then on,
    /// so an edit is never ignored, whatever page made it.
    static func synced(_ given: [Role], saved: [Role]) -> [Role] {
        given.map { role in
            let before = saved.first { $0.id == role.id }?.instructions
            var role = role
            if role.brief?.kind == .library, let before, role.instructions != before {
                role.brief = .text(role.instructions)
                return role
            }
            guard var brief = role.brief, brief.kind == .text else { return role }
            if !brief.text.isEmpty && brief.text != (before ?? role.instructions) {
                role.instructions = brief.text
            }
            brief.text = role.instructions
            role.brief = brief
            return role
        }
    }
}
