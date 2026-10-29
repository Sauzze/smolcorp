import XCTest
@testable import AgentDeck

/// iPhone app (personal build): what a session asking you something offers,
/// shown on the phone as buttons and picked by its number, only while that
/// same question is open; a permission's command or file never in the snapshot.
final class ChoiceTests: XCTestCase {
    private func event(_ name: String, _ fields: [String: Any] = [:]) -> [String: Any] {
        ["hook_event_name": name, "session_id": "s1", "cwd": "/work/api"].merging(fields) { $1 }
    }

    private let question: [String: Any] = [
        "questions": [[
            "question": "Which database?", "header": "DB", "multiSelect": false,
            "options": [["label": "Supabase", "description": "The usual"], ["label": "SQLite", "description": "Local only"]],
        ]],
    ]

    private func asked(_ fields: [String: Any], agent: Agent = .claude, tab: String = "tty:/dev/ttys003") -> Sessions {
        let sessions = Sessions()
        sessions.apply(event("UserPromptSubmit"), agent: agent, tab: tab, startedIn: "", at: 10)
        sessions.apply(event("PreToolUse", fields), agent: agent, tab: tab, startedIn: "", at: 20)
        return sessions
    }

    func testAQuestionsOptionsAreKeptWhileItAsks() throws {
        let sessions = asked(["tool_name": "AskUserQuestion", "tool_use_id": "t1", "tool_input": question])
        let choice = try XCTUnwrap(sessions["s1"]?.choice)
        XCTAssertEqual(sessions["s1"]?.state, .asking)
        XCTAssertEqual(choice.kind, .question)
        XCTAssertEqual(choice.question, "Which database?")
        XCTAssertEqual(choice.options, [.init(label: "Supabase", description: "The usual"), .init(label: "SQLite", description: "Local only")])
        XCTAssertTrue(choice.answerable)
        XCTAssertEqual(choice.forPhone(menuSeen: true)["question"] as? String, "Which database?")
        // Its permission, where one is asked for it too, doesn't take its place.
        sessions.apply(event("PermissionRequest", ["tool_name": "AskUserQuestion", "tool_input": question]), agent: .claude,
                       tab: "tty:/dev/ttys003", startedIn: "", at: 25)
        XCTAssertEqual(sessions["s1"]?.choice?.kind, .question)
        // Answered, on the Mac or the phone: gone.
        sessions.apply(event("PostToolUse", ["tool_name": "AskUserQuestion", "tool_use_id": "t1"]), agent: .claude,
                       tab: "tty:/dev/ttys003", startedIn: "", at: 30)
        XCTAssertNil(sessions["s1"]?.choice)
    }

    func testAPermissionShowsItsToolOnlyAndKeepsTheCommandForTailscale() throws {
        let sessions = Sessions()
        let input: [String: Any] = ["command": "rm -rf build/ && make", "description": "Rebuild"]
        sessions.apply(event("UserPromptSubmit"), agent: .claude, tab: "tty:/dev/ttys003", startedIn: "", at: 10)
        sessions.apply(event("PermissionRequest", ["tool_name": "Bash", "tool_input": input, "permission_suggestions": [["type": "addRules"]]]),
                       agent: .claude, tab: "tty:/dev/ttys003", startedIn: "", at: 20)
        let choice = try XCTUnwrap(sessions["s1"]?.choice)
        XCTAssertEqual(choice.question, "Allow Bash?")
        XCTAssertEqual(choice.options.map(\.label), ["Yes", "Yes, and don't ask again", "No"])
        XCTAssertEqual(choice.detail, "rm -rf build/ && make")
        let shown = String(decoding: try JSONSerialization.data(withJSONObject: choice.forPhone(menuSeen: true)), as: UTF8.self)
        XCTAssertFalse(shown.contains("rm -rf"), "the command goes only by GET /choice, through Tailscale")
        XCTAssertNil(choice.forPhone(menuSeen: true)["detail"])
        // With no rules to suggest, Claude Code offers Yes and No.
        let plain = try XCTUnwrap(PendingChoice.permission(tool: "Edit", input: ["file_path": "/work/api/.env"], suggests: false))
        XCTAssertEqual(plain.options.map(\.label), ["Yes", "No"])
        XCTAssertEqual(plain.detail, "/work/api/.env")
        XCTAssertEqual(PendingChoice.detail(of: [:]), "")
        XCTAssertEqual(PendingChoice.detail(of: ["command": String(repeating: "x", count: 900)]).count, PendingChoice.longest.detail)
    }

    func testOnlyClaudeCodeInATerminalOffersChoices() {
        let input: [String: Any] = ["tool_name": "AskUserQuestion", "tool_input": question]
        XCTAssertNil(asked(input, agent: .codex)["s1"]?.choice)
        XCTAssertNil(asked(input, tab: "headless:s1")["s1"]?.choice, "nobody answers a headless run")
        let sessions = Sessions()
        sessions.apply(event("PermissionRequest", ["tool_name": "Bash"]), agent: .gemini, tab: "tty:/dev/ttys003", startedIn: "", at: 20)
        XCTAssertNil(sessions["s1"]?.choice)
    }

    func testSeveralAnswersOrQuestionsAreShownNotAnswered() throws {
        var several = question
        several["questions"] = (question["questions"] as! [[String: Any]]) + [["question": "And the host?", "options": [["label": "Vercel"]]]]
        XCTAssertEqual(PendingChoice.question(several)?.questions, 2)
        XCTAssertFalse(try XCTUnwrap(PendingChoice.question(several)).answerable)
        var multi = (question["questions"] as! [[String: Any]])[0]
        multi["multiSelect"] = true
        XCTAssertFalse(try XCTUnwrap(PendingChoice.question(["questions": [multi]])).answerable)
        XCTAssertNil(PendingChoice.question(["questions": [["question": "Nothing to pick?", "options": []]]]))
        XCTAssertNil(PendingChoice.question([:]))
    }

    /// A tap answers the question it was for, while that one is asked, and no other.
    func testATapAnswersOnlyTheQuestionStillAsked() throws {
        let choice = try XCTUnwrap(PendingChoice.question(question, id: "q1"))
        XCTAssertNotNil(Deck.claimable(choice, state: .asking, id: "q1", lastReturn: nil, now: Date()))
        XCTAssertNil(Deck.claimable(choice, state: .asking, id: "q0", lastReturn: nil, now: Date()), "an earlier question's tap")
        XCTAssertNil(Deck.claimable(choice, state: .asking, id: "", lastReturn: nil, now: Date()))
        XCTAssertNil(Deck.claimable(choice, state: .asking, id: nil, lastReturn: nil, now: Date()))
        XCTAssertNil(Deck.claimable(choice, state: .thinking, id: "q1", lastReturn: nil, now: Date()), "answered meanwhile")
        XCTAssertNil(Deck.claimable(nil, state: .asking, id: "q1", lastReturn: nil, now: Date()), "already picked")
        XCTAssertNotEqual(PendingChoice.question(question)?.id, PendingChoice.question(question)?.id)
        // The route claims it on the main queue before any key goes, and clears it for a second tap.
        let deck = try String(contentsOfFile: Self.root + "Sources/AgentDeck/Deck.swift", encoding: .utf8)
        let body = try XCTUnwrap(deck.range(of: "private func choose(_ option:")).upperBound
        let choose = deck[body...].prefix(3600)
        let claim = try XCTUnwrap(choose.range(of: "Deck.claimable(session.choice, state: session.state, id: choiceID, lastReturn: lastReturn[id], now: Date())")).lowerBound
        let cleared = try XCTUnwrap(choose.range(of: "session.choice = nil")).lowerBound
        let pressed = try XCTUnwrap(choose.range(of: "Deck.press(keys, look: look, with: driver, in: tab)")).lowerBound
        XCTAssertLessThan(claim, cleared)
        XCTAssertLessThan(cleared, pressed)
    }

    func testAnOptionIsPickedByItsDigitOrByTheArrows() throws {
        var choice = try XCTUnwrap(PendingChoice.question(question))
        XCTAssertEqual(choice.keys(for: 2, byArrows: false, reject: .init(key: .escape)).map(\.key), [.two])
        XCTAssertEqual(choice.keys(for: 1, byArrows: true, reject: .init(key: .escape)).map(\.key), [.returnKey])
        let down = choice.keys(for: 2, byArrows: true, reject: .init(key: .escape))
        XCTAssertEqual(down.map(\.key), [.down, .returnKey])
        choice.move(1)
        choice.move(1)
        XCTAssertEqual(choice.cursor, 1, "kept within the menu")
        let up = choice.keys(for: 1, byArrows: true, reject: .init(key: .escape))
        XCTAssertEqual(up.map(\.key), [.up, .returnKey])
        XCTAssertEqual(up.first?.times, 1)
        choice.move(-5)
        XCTAssertEqual(choice.cursor, 0)
    }

    /// The digits type themselves in every terminal, never as a paste, and
    /// whatever the keyboard's layout.
    func testDigitsAreTypedAsKeysEverywhere() {
        for n in 1...9 {
            let key = Terminal.Key.digit(n)
            XCTAssertEqual(key?.character, String(n))
            let stroke = Terminal.Keystroke(key: key!)
            XCTAssertEqual(stroke.tmuxName, String(n))
            XCTAssertEqual(stroke.bytes, String(n))
        }
        XCTAssertNil(Terminal.Key.digit(0))
        XCTAssertNil(Terminal.Key.digit(10))
        XCTAssertNil(Terminal.Key.returnKey.character)
        XCTAssertEqual(Set((1...9).compactMap { Terminal.Key.digit($0)?.rawValue }).count, 9)
    }

    // MARK: The audit's findings (C-1 to C-3)

    /// C-1: a permission's No is the agent's reject key, never a digit that,
    /// on another menu than the one guessed, could be a Yes.
    func testAPermissionsNoIsTheRejectKeyNeverADigit() throws {
        let reject = Terminal.Keystroke(key: .escape)
        for suggests in [true, false] {
            let choice = try XCTUnwrap(PendingChoice.permission(tool: "Bash", input: [:], suggests: suggests))
            let no = choice.options.count
            XCTAssertEqual(choice.options.last?.label, "No")
            XCTAssertEqual(choice.keys(for: no, byArrows: false, reject: reject).map(\.key), [.escape])
            XCTAssertEqual(choice.keys(for: no, byArrows: true, reject: reject).map(\.key), [.escape])
            XCTAssertEqual(choice.keys(for: 1, byArrows: false, reject: reject).map(\.key), [.one])
        }
    }

    /// C-1: only a tool whose menu is known gets buttons; ExitPlanMode's plan
    /// approval none at all.
    func testOnlyKnownMenusAreAnsweredAndPlanApprovalIsLeftAlone() throws {
        for tool in ["Bash", "Edit", "Write", "MultiEdit", "NotebookEdit", "WebFetch", "mcp__github__create_issue"] {
            XCTAssertTrue(try XCTUnwrap(PendingChoice.permission(tool: tool, input: [:], suggests: false)).answerable, tool)
        }
        for tool in ["Task", "SlashCommand", "Skill", ""] {
            let choice = try XCTUnwrap(PendingChoice.permission(tool: tool, input: [:], suggests: true))
            XCTAssertFalse(choice.answerable, tool)
            XCTAssertTrue(choice.options.isEmpty, "\(tool): no guessed buttons")
        }
        XCTAssertNil(PendingChoice.permission(tool: "ExitPlanMode", input: [:], suggests: true))
        let sessions = Sessions()
        sessions.apply(event("UserPromptSubmit"), agent: .claude, tab: "tty:/dev/ttys003", startedIn: "", at: 10)
        sessions.apply(event("PermissionRequest", ["tool_name": "ExitPlanMode", "tool_input": ["plan": "Do it"]]),
                       agent: .claude, tab: "tty:/dev/ttys003", startedIn: "", at: 20)
        XCTAssertEqual(sessions["s1"]?.state, .asking)
        XCTAssertNil(sessions["s1"]?.choice)
    }

    /// C-1: "Yes, and don't ask again", a rule for good, goes only where the
    /// screen shows it at that number; Yes and No need no look.
    func testDontAskAgainIsPressedOnlyWhereTheScreenShowsIt() throws {
        let choice = try XCTUnwrap(PendingChoice.permission(tool: "Bash", input: [:], suggests: true))
        XCTAssertEqual(choice.screenCheck(for: 1)?.number, 1)
        XCTAssertEqual(choice.screenCheck(for: 1)?.labels, ["Yes"])
        XCTAssertEqual(choice.screenCheck(for: 2)?.number, 2)
        XCTAssertEqual(choice.screenCheck(for: 2)?.labels, ["Yes"])
        XCTAssertNil(choice.screenCheck(for: 3)?.number, "No, at whichever number it has")
        XCTAssertEqual(choice.screenCheck(for: 3)?.labels, ["No"])
        XCTAssertNil(choice.screenCheck(for: 4))
        let menu = "Do you want to proceed?\n❯ 1. Yes\n  2. Yes, and don't ask again for make commands in /work/api\n  3. No, and tell Claude what to do differently (esc)"
        XCTAssertTrue(PendingChoice.screen(menu, shows: 2, startingWith: ["Yes"]))
        XCTAssertTrue(PendingChoice.screen("│ ❯ 2. Yes, allow all edits │", shows: 2, startingWith: ["Yes"]))
        XCTAssertFalse(PendingChoice.screen("❯ 1. Yes\n  2. No, and tell Claude what to do differently (esc)", shows: 2, startingWith: ["Yes"]),
                       "a two-option menu: 2 is No")
        XCTAssertFalse(PendingChoice.screen("ran 12. Yes it did", shows: 2, startingWith: ["Yes"]))
        XCTAssertFalse(PendingChoice.screen("", shows: 2, startingWith: ["Yes"]))
        // And the route looks before any key goes: a two-option menu's 2 is No, never pressed for a Yes.
        let allow = try XCTUnwrap(PendingChoice.permission(tool: "Bash", input: ["command": "make"], suggests: true))
        let tab = try XCTUnwrap(TabAddress("tty:/dev/ttys003"))
        let twoOptions = FakeTerminal { _, _ in "❯ 1. Yes\n  2. No, and tell Claude what to do differently (esc)" }
        XCTAssertEqual(Deck.press(allow.keys(for: 2, byArrows: false, reject: .init(key: .escape)), look: allow.screenCheck(for: 2),
                                  with: twoOptions, in: tab), .menuDiffers)
        XCTAssertEqual(twoOptions.pressed, [])
    }

    /// C-2: ✓, ✗ and Esc answer or put the menu away, and its choice with
    /// it; no tap is taken within a Return's repeat, which may have answered it.
    func testAMenuAnsweredOtherwiseTakesItsChoiceAway() throws {
        let choice = try XCTUnwrap(PendingChoice.question(question, id: "q1"))
        let now = Date()
        XCTAssertNil(Deck.claimable(choice, state: .asking, id: "q1", lastReturn: now, now: now.addingTimeInterval(0.2)))
        XCTAssertNotNil(Deck.claimable(choice, state: .asking, id: "q1", lastReturn: now, now: now.addingTimeInterval(Deck.returnRepeat)))
        let deck = try String(contentsOfFile: Self.root + "Sources/AgentDeck/Deck.swift", encoding: .utf8)
        let accept = try XCTUnwrap(deck.range(of: #"case "accept", "send":"#)).upperBound
        XCTAssertTrue(deck[accept...].prefix(700).contains("session.choice = nil"))
        let reject = try XCTUnwrap(deck.range(of: #"case "reject", "esc":"#)).upperBound
        XCTAssertTrue(deck[reject...].prefix(200).contains("session.choice = nil"))
    }

    /// C-3: the arrows' way stays off unless set by hand, the highlight moves
    /// only once ▲ or ▼ has gone, and arrows that may have gone part way
    /// drop the choice rather than give it back.
    func testTheArrowsWayIsOffAndKeepsNoGuessedHighlight() throws {
        let deck = try String(contentsOfFile: Self.root + "Sources/AgentDeck/Deck.swift", encoding: .utf8)
        XCTAssertTrue(deck.contains(#"let byArrows = UserDefaults.standard.string(forKey: "choiceKeys") == "arrows""#))
        XCTAssertEqual(deck.components(separatedBy: "choiceKeys").count, 3, "read in one place, besides its comment")
        let move = try XCTUnwrap(deck.range(of: #"case "up", "down":"#)).upperBound
        XCTAssertTrue(deck[move...].prefix(500).contains("if response.status < 400 { self?.sessions[id]?.choice?.move("))
        // Arrows that went, or may have gone part way, never give the choice back; digits that reached nothing do.
        for pressed in [Deck.Pressed.notTaken, .failed] {
            XCTAssertFalse(Deck.outcome(of: pressed, response: .status(pressed == .failed ? 404 : 204), byArrows: true).givesBack)
            XCTAssertTrue(Deck.outcome(of: pressed, response: .status(pressed == .failed ? 404 : 204), byArrows: false).givesBack)
        }
    }

    // MARK: The audit's delta (D-1 to D-3)

    /// D-1: Ghostty types a typed answer as keys, as it presses them, not
    /// with `input text`, a paste; and, its screen unseen, answers nothing
    /// from the phone.
    func testGhosttyTypesAnswersAsKeysAndShowsNoMenu() throws {
        let drivers = try String(contentsOfFile: Self.root + "Sources/AgentDeck/TerminalDrivers.swift", encoding: .utf8)
        let ghostty = try XCTUnwrap(drivers.range(of: "struct Ghostty: TerminalDriver {"))
        let body = drivers[ghostty.upperBound...].prefix(9000)
        let end = try XCTUnwrap(body.range(of: "\n}\n")).lowerBound
        let own = body[..<end]
        XCTAssertTrue(own.contains("func typeKeys(_ text: String, in tab: TabAddress) -> Bool { typeBytes(text, in: tab) }"))
        XCTAssertTrue(own.contains("var showsScreen: Bool { false }"))
        let choice = try XCTUnwrap(PendingChoice.question(question))
        XCTAssertEqual(choice.forPhone(menuSeen: false)["answerable"] as? Bool, false)
        XCTAssertEqual(choice.forPhone(menuSeen: false)["other"] as? Bool, false)
        XCTAssertEqual(choice.forPhone(menuSeen: true)["answerable"] as? Bool, true)
    }

    /// D-2: a typed answer starting with "!", which Claude Code's prompt
    /// would run as a shell command, is refused, spaces before it or not.
    func testATypedAnswerNeverStartsWithABang() throws {
        XCTAssertEqual(PhoneDictation.prepared("  !rm -rf build"), "!rm -rf build")
        let deck = try String(contentsOfFile: Self.root + "Sources/AgentDeck/Deck.swift", encoding: .utf8)
        let start = try XCTUnwrap(deck.range(of: "func answerOther(_ text: String")).upperBound
        let body = deck[start...].prefix(900)
        let cleaned = try XCTUnwrap(body.range(of: "guard let words = PhoneDictation.prepared(text) else")).lowerBound
        let bang = try XCTUnwrap(body.range(of: #"guard !words.hasPrefix("!") else { return done(.status(400, Deck.noBang)) }"#)).lowerBound
        XCTAssertLessThan(cleaned, bang, "checked on the text as it would be typed")
    }

    /// D-3: every option's key goes only where the menu on the screen shows
    /// it at its number, in the screen's last lines; a menu answered on the
    /// Mac's keyboard, gone from the screen, takes nothing.
    func testEveryOptionIsSeenOnTheScreenFirst() throws {
        let choice = try XCTUnwrap(PendingChoice.question(question))
        XCTAssertEqual(choice.screenCheck(for: 2)?.number, 2)
        XCTAssertEqual(choice.screenCheck(for: 2)?.labels, ["SQLite"])
        XCTAssertEqual(choice.screenCheck(for: 3)?.number, 3, "Type something, after the options")
        XCTAssertEqual(choice.screenCheck(for: 3)?.labels, ["Type something", "Other"])
        XCTAssertNil(choice.screenCheck(for: 4))
        let menu = "Which database?\n❯ 1. Supabase\n     The usual\n  2. SQLite\n     Local only\n  3. Type something.\n"
        XCTAssertTrue(PendingChoice.screen(menu, shows: 2, startingWith: ["SQLite"]))
        XCTAssertTrue(PendingChoice.screen(menu, shows: 3, startingWith: ["Type something", "Other"]))
        XCTAssertFalse(PendingChoice.screen(menu, shows: 1, startingWith: ["SQLite"]), "another option at that number")
        let answered = "⏺ User answered Claude's questions:\n  ⎿  · Which database? → SQLite\n\n> "
        XCTAssertFalse(PendingChoice.screen(answered, shows: 2, startingWith: ["SQLite"]))
        // Long ago, scrolled up: only the screen's last lines count.
        let old = menu + String(repeating: "\n", count: PendingChoice.menuLines) + "> "
        XCTAssertFalse(PendingChoice.screen(old, shows: 2, startingWith: ["SQLite"]))
        // No at any number, but only as an option's line.
        XCTAssertTrue(PendingChoice.screen("  1. Yes\n  2. No, and tell Claude what to do differently", shows: nil, startingWith: ["No"]))
        XCTAssertFalse(PendingChoice.screen("No, I said", shows: nil, startingWith: ["No"]))
        XCTAssertFalse(PendingChoice.screen("  2.No", shows: nil, startingWith: ["No"]))
        // A long label is looked for by its start, as a narrow tab may cut it.
        let long = try XCTUnwrap(PendingChoice.question(["questions": [["question": "Which?", "options": [["label": "A very long label that wraps round"]]]]]))
        XCTAssertEqual(long.screenCheck(for: 1)?.labels, ["A very long labe"])
        // In the route: always looked for, and a menu not seen drops the choice, never given back.
        let deck = try String(contentsOfFile: Self.root + "Sources/AgentDeck/Deck.swift", encoding: .utf8)
        let start = try XCTUnwrap(deck.range(of: "private func choose(_ option:")).upperBound
        let body = deck[start...].prefix(3600)
        XCTAssertTrue(body.contains("guard let look, let screen = driver.screen(of: tab),"))
        XCTAssertFalse(body.contains("if let label"), "never skipped")
    }

    /// Claude Code's own AskUserQuestion menu, as Terminal.app showed it on
    /// 21 Oct 2026 (choice-test-1a): each option seen at its number, and
    /// "Type something." right after them.
    func testTheRealQuestionMenuIsSeen() throws {
        let screen = [
            "───────────────────────────────────────────────────────────────",
            " ☐ Snack ",
            "",
            "Screen check: which snack?",
            "",
            "❯ 1. Croissant",
            "     Buttery and flaky",
            "  2. Pain au chocolat",
            "     Chocolate inside the pastry",
            "  3. Apple",
            "     Crisp and light",
            "  4. Type something.                       ",
            "───────────────────────────────────────────────────────────────",
            "  5. Chat about this",
            "",
            "Enter to select · ↑/↓ to navigate · Esc to cancel",
        ].joined(separator: "\n")
        let snacks: [String: Any] = ["questions": [["question": "Screen check: which snack?", "header": "Snack", "multiSelect": false, "options": [
            ["label": "Croissant", "description": "Buttery and flaky"],
            ["label": "Pain au chocolat", "description": "Chocolate inside the pastry"],
            ["label": "Apple", "description": "Crisp and light"],
        ]]]]
        let choice = try XCTUnwrap(PendingChoice.question(snacks))
        for n in 1...4 {
            let look = try XCTUnwrap(choice.screenCheck(for: n))
            XCTAssertTrue(PendingChoice.screen(screen, shows: look.number, startingWith: look.labels), "option \(n)")
        }
        XCTAssertNil(choice.screenCheck(for: 5), "Chat about this is never pressed")
        XCTAssertEqual(Terminal.Key.digit(choice.options.count + 1), .four)
    }

    /// The GetCooked art director's menu, as Terminal.app showed it on 22 Oct
    /// 2026 when a tap on the phone didn't answer it: its question in a
    /// "│ " block, under a "☐" header.
    static let downloadsScreen = [
        "───────────────────────────────────────────────────────────────",
        " ☐ Downloads ",
        "",
        "│ Auto mode blocked my curl downloads from images.unsplash.com and images.pexels.com, which I need for the",
        "│ photo-style comparison board. May I download those from images.unsplash.com and images.pexels.com only?",
        "",
        "❯ 1. Yes, download them",
        "     Unsplash/Pexels CDNs only, into the worktree and the scratchpad. Nothing else is fetched.",
        "  2. No, skip photos",
        "     Ship the comparison with Fluent 3D only, plus a note on the photo style.",
        "  3. Type something.                       ",
        "───────────────────────────────────────────────────────────────",
        "  4. Chat about this",
        "",
        "Enter to select · ↑/↓ to navigate · Esc to cancel",
    ].joined(separator: "\n")

    static let downloads: [String: Any] = ["questions": [["question": "Auto mode blocked my curl downloads from images.unsplash.com and images.pexels.com, which I need for the photo-style comparison board. May I download those from images.unsplash.com and images.pexels.com only?", "header": "Downloads", "multiSelect": false, "options": [
        ["label": "Yes, download them", "description": "Unsplash/Pexels CDNs only, into the worktree and the scratchpad. Nothing else is fetched."],
        ["label": "No, skip photos", "description": "Ship the comparison with Fluent 3D only, plus a note on the photo style."],
    ]]]]

    func testTheDownloadsMenuIsSeen() throws {
        let choice = try XCTUnwrap(PendingChoice.question(Self.downloads))
        XCTAssertTrue(choice.answerable)
        for n in 1...3 {
            let look = try XCTUnwrap(choice.screenCheck(for: n))
            XCTAssertTrue(PendingChoice.screen(Self.downloadsScreen, shows: look.number, startingWith: look.labels), "option \(n)")
        }
    }

    /// The tap that didn't answer it: a key posted with the Mac locked
    /// reaches nothing, and the menu stays as it was. That's a 409, never a
    /// 200; an answer, or another menu in its place, is a change.
    func testAKeyThatReachedNothingLeavesTheMenuOpen() throws {
        let choice = try XCTUnwrap(PendingChoice.question(Self.downloads))
        let look = try XCTUnwrap(choice.screenCheck(for: 1))
        let before = Self.downloadsScreen
        XCTAssertTrue(PendingChoice.stillOpen(before: before, after: before, shows: look.number, startingWith: look.labels))
        // A status line ticking under it is still the same menu.
        XCTAssertTrue(PendingChoice.stillOpen(before: before, after: before + "\n  ◑ 03:46", shows: look.number, startingWith: look.labels))
        let answered = "⏺ User answered Claude's questions:\n  ⎿  · Auto mode blocked my curl downloads… → Yes, download them\n\n> "
        XCTAssertFalse(PendingChoice.stillOpen(before: before, after: answered, shows: look.number, startingWith: look.labels))
        let next = "Do you want to proceed?\n❯ 1. Yes, download them\n  2. No"
        XCTAssertFalse(PendingChoice.stillOpen(before: before, after: next, shows: look.number, startingWith: look.labels), "another menu")
        XCTAssertFalse(Deck.menuChanged(from: before, look: look, wait: 0.2) { before })
        XCTAssertTrue(Deck.menuChanged(from: before, look: look, wait: 0.2) { answered })
        XCTAssertTrue(Deck.menuChanged(from: before, look: look, wait: 0.2) { nil }, "unreadable: as before the keys")
        var looks = 0
        XCTAssertTrue(Deck.menuChanged(from: before, look: look, wait: 1) { looks += 1; return looks < 3 ? before : answered }, "answered a moment later")
    }

    /// The route's keys, on a terminal that shows a menu and takes keys or
    /// doesn't: a 204 only where the menu changed after them. A key posted
    /// to a locked Mac's Terminal may reach nothing: that's a 409, and the
    /// question is given back for another tap. Nothing is refused for the
    /// lock itself, the user's call of 18 Oct.
    func testATapIsAnsweredOnlyWhereTheMenuTakesIt() throws {
        let choice = try XCTUnwrap(PendingChoice.question(Self.downloads))
        let look = choice.screenCheck(for: 1)
        let keys = choice.keys(for: 1, byArrows: false, reject: .init(key: .escape))
        let tab = try XCTUnwrap(TabAddress("tty:/dev/ttys002"))
        let menu = Self.downloadsScreen
        let answered = "⏺ User answered Claude's questions:\n  ⎿  · Auto mode blocked my curl downloads… → Yes, download them\n\n> "
        let wait: TimeInterval = 0.3

        let taken = FakeTerminal { pressed, _ in pressed == 0 ? menu : answered }
        XCTAssertEqual(Deck.press(keys, look: look, with: taken, in: tab, wait: wait), .answered)
        XCTAssertEqual(taken.pressed, [.one])
        let ok = Deck.outcome(of: .answered, response: .status(204), byArrows: false)
        XCTAssertEqual(ok.response.status, 204)
        XCTAssertFalse(ok.givesBack)

        let late = FakeTerminal { pressed, read in pressed == 0 || read < 4 ? menu : answered }
        XCTAssertEqual(Deck.press(keys, look: look, with: late, in: tab, wait: 1), .answered, "redrawn a moment later")

        let lost = FakeTerminal { _, _ in menu }
        XCTAssertEqual(Deck.press(keys, look: look, with: lost, in: tab, wait: wait), .notTaken)
        XCTAssertEqual(lost.pressed, [.one], "pressed: nothing refused it beforehand")
        let notTaken = Deck.outcome(of: .notTaken, response: .status(204), byArrows: false)
        XCTAssertEqual(notTaken.response.status, 409)
        XCTAssertEqual(String(decoding: notTaken.response.body, as: UTF8.self), Deck.notAnswered)
        XCTAssertTrue(notTaken.givesBack)
        XCTAssertFalse(Deck.outcome(of: .notTaken, response: .status(204), byArrows: true).givesBack, "the highlight may have moved")

        let refused = FakeTerminal { _, _ in menu }
        refused.takesKeys = false
        XCTAssertEqual(Deck.press(keys, look: look, with: refused, in: tab, wait: wait), .failed)
        let failed = Deck.outcome(of: .failed, response: .status(404, "Terminal tab not found"), byArrows: false)
        XCTAssertEqual(failed.response.status, 404)
        XCTAssertTrue(failed.givesBack)
        XCTAssertTrue(Deck.outcome(of: nil, response: .status(410, "That agent has ended."), byArrows: false).givesBack, "no key tried")
    }

    /// Every option's key goes only where the screen shows that option at
    /// its number: another menu, or a screen that can't be read, takes none,
    /// and the choice goes rather than coming back.
    func testNoKeyGoesWhereTheMenuIsntSeen() throws {
        let choice = try XCTUnwrap(PendingChoice.question(Self.downloads))
        let keys = choice.keys(for: 2, byArrows: false, reject: .init(key: .escape))
        let tab = try XCTUnwrap(TabAddress("tty:/dev/ttys002"))
        let other = FakeTerminal { _, _ in "Do you want to proceed?\n❯ 1. Yes\n  2. No, and tell Claude what to do differently (esc)" }
        XCTAssertEqual(Deck.press(keys, look: choice.screenCheck(for: 2), with: other, in: tab), .menuDiffers)
        let unreadable = FakeTerminal { _, _ in nil }
        XCTAssertEqual(Deck.press(keys, look: choice.screenCheck(for: 2), with: unreadable, in: tab), .menuDiffers)
        let unknown = FakeTerminal { _, _ in Self.downloadsScreen }
        XCTAssertEqual(Deck.press(keys, look: nil, with: unknown, in: tab), .menuDiffers, "no option to look for")
        XCTAssertEqual(other.pressed + unreadable.pressed + unknown.pressed, [])
        let differs = Deck.outcome(of: .menuDiffers, response: .status(204), byArrows: false)
        XCTAssertEqual(differs.response.status, 409)
        XCTAssertEqual(String(decoding: differs.response.body, as: UTF8.self), Deck.menuDiffers)
        XCTAssertFalse(differs.givesBack)
    }

    /// What's logged of an answer: the route, the option's number, the
    /// session, the status and the deck's own reason, never the words typed.
    func testAnAnswerIsLoggedWithoutItsWords() {
        XCTAssertEqual(Deck.logLine("choose", option: "1", in: "s1", .status(204)), "choose option 1 in s1: 204")
        XCTAssertEqual(Deck.logLine("choose", option: "1", in: "s1", .status(409, Deck.notAnswered)), "choose option 1 in s1: 409 " + Deck.notAnswered)
        XCTAssertEqual(Deck.logLine("answerOther", option: "other", in: "unknown", .status(204)), "answerOther option other in unknown: 204")
        XCTAssertEqual(Deck.logLine("choose", option: "my secret words", in: "s1", .status(400, "Unknown option")), "choose option ? in s1: 400 Unknown option")
    }

    /// A subagent's tools carrying on while its session asks you something
    /// don't end the question, nor take its options from the phone: only
    /// the asker's own events do.
    func testASubagentsToolsDontEndTheQuestion() throws {
        let sessions = asked(["tool_name": "AskUserQuestion", "tool_use_id": "t1", "tool_input": Self.downloads])
        let id = try XCTUnwrap(sessions["s1"]?.choice?.id)
        let apply = { (name: String, fields: [String: Any], at: Double) in
            sessions.apply(self.event(name, fields), agent: .claude, tab: "tty:/dev/ttys003", startedIn: "", at: at)
        }
        apply("PreToolUse", ["agent_id": "a1", "tool_name": "Bash", "tool_use_id": "b1", "tool_input": ["command": "ls"]], 21)
        apply("PostToolUse", ["agent_id": "a1", "tool_name": "Bash", "tool_use_id": "b1"], 22)
        apply("PostToolUseFailure", ["agent_id": "a1", "tool_name": "Bash", "tool_use_id": "b2"], 23)
        XCTAssertEqual(sessions["s1"]?.state, .asking)
        XCTAssertEqual(sessions["s1"]?.choice?.id, id, "the same question, for the same tap")
        // Answered: its own tool's end.
        apply("PostToolUse", ["tool_name": "AskUserQuestion", "tool_use_id": "t1"], 30)
        XCTAssertEqual(sessions["s1"]?.state, .thinking)
        XCTAssertNil(sessions["s1"]?.choice)
        // A subagent's own permission prompt still ends with its tool.
        apply("PermissionRequest", ["agent_id": "a2", "tool_name": "Bash", "tool_input": ["command": "make"]], 31)
        XCTAssertEqual(sessions["s1"]?.state, .asking)
        apply("PostToolUse", ["agent_id": "a1", "tool_name": "Read", "tool_use_id": "r1"], 32)
        XCTAssertEqual(sessions["s1"]?.state, .asking, "another subagent's tool")
        apply("PostToolUse", ["agent_id": "a2", "tool_name": "Bash", "tool_use_id": "m1"], 33)
        XCTAssertEqual(sessions["s1"]?.state, .thinking)
    }

    private func applier(_ sessions: Sessions) -> (String, [String: Any], Double) -> Void {
        { name, fields, at in sessions.apply(self.event(name, fields), agent: .claude, tab: "tty:/dev/ttys003", startedIn: "", at: at) }
    }

    /// Hooks go async: a subagent's prompt can come first as a Notification,
    /// and with its PermissionRequest naming no subagent. The asked tool's
    /// own end, whoever sends it, ends the question all the same.
    func testASubagentsPromptHeardWithoutItsIdEndsWithItsTool() throws {
        let sessions = Sessions()
        let apply = applier(sessions)
        apply("UserPromptSubmit", [:], 10)
        apply("PreToolUse", ["tool_name": "Task", "tool_use_id": "task1", "tool_input": ["prompt": "build"]], 11)
        apply("Notification", ["notification_type": "permission_prompt", "message": "Claude needs your permission to use Bash"], 12)
        XCTAssertEqual(sessions["s1"]?.state, .asking)
        apply("PermissionRequest", ["tool_name": "Bash", "tool_input": ["command": "make", "description": "Build"]], 13)
        XCTAssertEqual(sessions["s1"]?.state, .asking)
        XCTAssertEqual(sessions["s1"]?.asked.tool, "Bash")
        apply("PostToolUse", ["agent_id": "b", "tool_name": "Read", "tool_use_id": "r1", "tool_input": ["file_path": "/x"]], 14)
        apply("PostToolUse", ["agent_id": "b", "tool_name": "Bash", "tool_use_id": "b1", "tool_input": ["command": "ls"]], 15)
        XCTAssertEqual(sessions["s1"]?.state, .asking, "another subagent's tools, another Bash among them")
        apply("PostToolUse", ["agent_id": "a", "tool_name": "Bash", "tool_use_id": "m1", "tool_input": ["description": "Build", "command": "make"]], 16)
        XCTAssertEqual(sessions["s1"]?.state, .thinking, "the asked call's own end")
        XCTAssertNil(sessions["s1"]?.choice)
        XCTAssertEqual(sessions["s1"]?.asked, Session.Asked())
        // Failed, or denied with a failure: the same.
        apply("Notification", ["notification_type": "permission_prompt"], 20)
        apply("PermissionRequest", ["tool_name": "Bash", "tool_input": ["command": "rm -rf build"]], 21)
        apply("PostToolUseFailure", ["agent_id": "a", "tool_name": "Bash", "tool_use_id": "m2", "tool_input": ["command": "rm -rf build"]], 22)
        XCTAssertEqual(sessions["s1"]?.state, .thinking)
    }

    /// A denied prompt has no tool end: the subagent that asked stopping
    /// ends it, a later event having named it; another one's stop doesn't.
    func testADeniedSubagentsStopEndsItsQuestion() throws {
        let sessions = Sessions()
        let apply = applier(sessions)
        apply("UserPromptSubmit", [:], 10)
        apply("Notification", ["notification_type": "permission_prompt"], 11)
        XCTAssertEqual(sessions["s1"]?.asked.by, "")
        apply("PermissionRequest", ["agent_id": "a", "tool_name": "Bash", "tool_input": ["command": "make"]], 12)
        XCTAssertEqual(sessions["s1"]?.asked.by, "a", "named later")
        // Denied: no PostToolUse. Another subagent ends meanwhile.
        apply("SubagentStop", ["agent_id": "b"], 13)
        XCTAssertEqual(sessions["s1"]?.state, .asking)
        apply("SubagentStop", ["agent_id": "a"], 14)
        XCTAssertEqual(sessions["s1"]?.state, .thinking)
        XCTAssertNil(sessions["s1"]?.choice)
        // Denied, and the asker carries on with another tool: its own tool ends it too.
        apply("PermissionRequest", ["agent_id": "a", "tool_name": "Bash", "tool_input": ["command": "make"]], 20)
        apply("PreToolUse", ["agent_id": "a", "tool_name": "Read", "tool_use_id": "r2", "tool_input": ["file_path": "/x"]], 21)
        XCTAssertEqual(sessions["s1"]?.state, .thinking)
    }

    /// A prompt's PermissionRequest is raised before its Notification, the
    /// dialog then shown, and the hooks go async: arriving last, too old for
    /// the state, it still says who asks, what, and the phone's options.
    func testALatePermissionRequestStillSaysWhatsAsked() throws {
        let request: [String: Any] = ["agent_id": "a", "tool_name": "Bash", "tool_input": ["command": "make"]]
        let notice: [String: Any] = ["notification_type": "permission_prompt", "message": "Claude needs your permission to use Bash"]
        for reversed in [true, false] {
            let sessions = Sessions()
            let apply = applier(sessions)
            apply("UserPromptSubmit", [:], 10)
            apply("PreToolUse", ["tool_name": "Task", "tool_use_id": "task1", "tool_input": ["prompt": "build"]], 11)
            // Raised: the PermissionRequest at 13, its Notification at 14.
            if reversed {
                apply("Notification", notice, 14)
                apply("PermissionRequest", request, 13)
            } else {
                apply("PermissionRequest", request, 13)
                apply("Notification", notice, 14)
            }
            let session = try XCTUnwrap(sessions["s1"])
            XCTAssertEqual(session.state, .asking, "reversed: \(reversed)")
            XCTAssertEqual(session.asked.by, "a", "reversed: \(reversed)")
            XCTAssertEqual(session.asked.tool, "Bash")
            XCTAssertEqual(session.choice?.question, "Allow Bash?", "the phone's options")
            apply("PostToolUse", ["agent_id": "a", "tool_name": "Bash", "tool_use_id": "m1", "tool_input": ["command": "make"]], 15)
            XCTAssertEqual(session.state, .thinking)
        }
        // Its own AskUserQuestion, likewise.
        let sessions = Sessions()
        let apply = applier(sessions)
        apply("UserPromptSubmit", [:], 10)
        apply("Notification", ["notification_type": "elicitation_dialog"], 13)
        apply("PreToolUse", ["tool_name": "AskUserQuestion", "tool_use_id": "t1", "tool_input": Self.downloads], 12)
        XCTAssertEqual(sessions["s1"]?.state, .asking)
        XCTAssertEqual(sessions["s1"]?.choice?.kind, .question)
        XCTAssertEqual(sessions["s1"]?.asked.toolUseID, "t1")
        apply("PostToolUse", ["tool_name": "AskUserQuestion", "tool_use_id": "t1"], 14)
        XCTAssertEqual(sessions["s1"]?.state, .thinking)
        // One raised before the last question ended belongs to that one: it says nothing of the next.
        apply("Notification", notice, 20)
        apply("PostToolUse", ["tool_name": "Bash", "tool_use_id": "m1", "tool_input": ["command": "make"]], 22)
        XCTAssertEqual(sessions["s1"]?.state, .thinking)
        apply("Notification", notice, 24)
        apply("PermissionRequest", request, 19)
        XCTAssertEqual(sessions["s1"]?.state, .asking)
        XCTAssertEqual(sessions["s1"]?.asked.tool, "", "not the old prompt's")
        XCTAssertNil(sessions["s1"]?.choice)
        // Nor, too old, does it change the state of a session not asking.
        apply("PostToolUse", ["tool_name": "Read", "tool_use_id": "r1"], 30)
        apply("PermissionRequest", request, 29)
        XCTAssertEqual(sessions["s1"]?.state, .thinking)
        XCTAssertNil(sessions["s1"]?.choice)
    }

    /// No event names the asker, and only one subagent runs while the
    /// session waits on it: that one asks, and its tools or its stop end it.
    /// With several, any could: it stays for the session's own next event,
    /// or the screen.
    func testTheOneSubagentRunningIsTheAsker() throws {
        let sessions = Sessions()
        let apply = applier(sessions)
        apply("UserPromptSubmit", [:], 10)
        apply("PreToolUse", ["tool_name": "Task", "tool_use_id": "task1", "tool_input": ["prompt": "build"]], 11)
        apply("SubagentStart", ["agent_id": "a", "agent_type": "general-purpose"], 12)
        apply("Notification", ["notification_type": "agent_needs_input"], 13)
        XCTAssertEqual(sessions["s1"]?.asked.by, "a")
        apply("SubagentStop", ["agent_id": "a"], 14)
        XCTAssertEqual(sessions["s1"]?.state, .thinking, "denied, then stopped")

        let several = Sessions()
        let both = applier(several)
        both("UserPromptSubmit", [:], 10)
        both("PreToolUse", ["tool_name": "Agent", "tool_use_id": "task1", "tool_input": ["prompt": "build"]], 11)
        both("SubagentStart", ["agent_id": "a", "agent_type": "general-purpose"], 12)
        both("SubagentStart", ["agent_id": "b", "agent_type": "general-purpose"], 13)
        both("Notification", ["notification_type": "permission_prompt"], 14)
        XCTAssertEqual(several["s1"]?.asked.by, "")
        both("PostToolUse", ["agent_id": "a", "tool_name": "Read", "tool_use_id": "r1", "tool_input": ["file_path": "/x"]], 15)
        XCTAssertEqual(several["s1"]?.state, .asking)

        // On a tool of its own, the session is the one asking, a subagent running beside it or not.
        let own = Sessions()
        let mine = applier(own)
        mine("UserPromptSubmit", [:], 10)
        mine("SubagentStart", ["agent_id": "a", "agent_type": "general-purpose"], 11)
        mine("PreToolUse", ["tool_name": "Bash", "tool_use_id": "m1", "tool_input": ["command": "make"]], 12)
        mine("PermissionRequest", ["tool_name": "Bash", "tool_input": ["command": "make"]], 13)
        XCTAssertEqual(own["s1"]?.asked.by, "")
        mine("PostToolUse", ["agent_id": "a", "tool_name": "Read", "tool_use_id": "r1", "tool_input": ["file_path": "/x"]], 14)
        XCTAssertEqual(own["s1"]?.state, .asking)
    }

    /// Two subagents making the same call, same tool and input: the asked
    /// one's id comes from its PreToolUse, however late it arrives, so the
    /// other's end doesn't end this question.
    func testTheSameCallFromAnotherSubagentDoesntEndTheQuestion() throws {
        let make: [String: Any] = ["command": "make"]
        for late in [false, true] {
            let sessions = Sessions()
            let apply = applier(sessions)
            apply("UserPromptSubmit", [:], 10)
            apply("PreToolUse", ["tool_name": "Task", "tool_use_id": "task1", "tool_input": ["prompt": "build"]], 11)
            apply("PreToolUse", ["agent_id": "b", "tool_name": "Bash", "tool_use_id": "b1", "tool_input": make], 12)
            if !late { apply("PreToolUse", ["agent_id": "a", "tool_name": "Bash", "tool_use_id": "a1", "tool_input": make], 13) }
            apply("PermissionRequest", ["agent_id": "a", "tool_name": "Bash", "tool_input": make], 14)
            if late { apply("PreToolUse", ["agent_id": "a", "tool_name": "Bash", "tool_use_id": "a1", "tool_input": make], 13) }
            XCTAssertEqual(sessions["s1"]?.asked.toolUseID, "a1", "late: \(late)")
            apply("PostToolUse", ["agent_id": "b", "tool_name": "Bash", "tool_use_id": "b1", "tool_input": make], 15)
            XCTAssertEqual(sessions["s1"]?.state, .asking, "b's own make, late: \(late)")
            apply("PostToolUse", ["agent_id": "a", "tool_name": "Bash", "tool_use_id": "a1", "tool_input": make], 16)
            XCTAssertEqual(sessions["s1"]?.state, .thinking)
        }
    }

    /// The session's own prompt stays its own: another subagent's prompt
    /// meanwhile doesn't make that subagent its asker.
    func testTheSessionsOwnPromptIsntTakenByASubagents() throws {
        let sessions = Sessions()
        let apply = applier(sessions)
        apply("UserPromptSubmit", [:], 10)
        apply("PermissionRequest", ["tool_name": "Bash", "tool_input": ["command": "make"]], 11)
        apply("PermissionRequest", ["agent_id": "b", "tool_name": "Edit", "tool_input": ["file_path": "/x"]], 12)
        XCTAssertEqual(sessions["s1"]?.asked.by, "")
        XCTAssertEqual(sessions["s1"]?.asked.tool, "Bash")
        apply("PostToolUse", ["agent_id": "b", "tool_name": "Edit", "tool_use_id": "e1", "tool_input": ["file_path": "/x"]], 13)
        apply("SubagentStop", ["agent_id": "b"], 14)
        XCTAssertEqual(sessions["s1"]?.state, .asking, "still its own Bash")
        apply("PostToolUse", ["tool_name": "Bash", "tool_use_id": "m1", "tool_input": ["command": "make"]], 15)
        XCTAssertEqual(sessions["s1"]?.state, .thinking)
        // Its own question, likewise.
        let asking = asked(["tool_name": "AskUserQuestion", "tool_use_id": "t1", "tool_input": Self.downloads])
        applier(asking)("Notification", ["notification_type": "permission_prompt"], 21)
        applier(asking)("PermissionRequest", ["agent_id": "b", "tool_name": "Bash", "tool_input": ["command": "ls"]], 22)
        XCTAssertEqual(asking["s1"]?.asked.by, "")
        XCTAssertEqual(asking["s1"]?.asked.toolUseID, "t1")
    }

    /// Why a tap didn't answer stays on the phone long enough to read.
    func testTheReasonStaysLongEnoughToRead() throws {
        let page = try String(contentsOfFile: Self.root + "Resources/web/deck.html", encoding: .utf8)
        let start = try XCTUnwrap(page.range(of: "function flash(text) {")).upperBound
        let body = page[start...].prefix(500)
        let delay = try XCTUnwrap(body.range(of: #"\}, (\d+)\);"#, options: .regularExpression))
        let ms = try XCTUnwrap(Int(body[delay].filter(\.isNumber)))
        XCTAssertGreaterThanOrEqual(ms, 4000)
    }

    /// D-3: a typed answer's Return waits for no new question, and no ✓ goes while it's typed.
    func testATypedAnswersReturnIsForItsOwnQuestion() throws {
        let now = Date()
        XCTAssertFalse(Deck.mayAnswer(state: .asking, newChoice: true, last: nil, now: now), "a new question came in its place")
        XCTAssertTrue(Deck.mayAnswer(state: .asking, newChoice: false, last: nil, now: now))
        let deck = try String(contentsOfFile: Self.root + "Sources/AgentDeck/Deck.swift", encoding: .utf8)
        XCTAssertTrue(deck.contains("newChoice: sessions[id]?.choice != nil"))
        let accept = try XCTUnwrap(deck.range(of: #"case "accept", "send":"#)).upperBound
        XCTAssertTrue(deck[accept...].prefix(500).contains("guard !answering.contains(id) else { return done(.status(409, Deck.stillTyping)) }"))
        let start = try XCTUnwrap(deck.range(of: "func answerOther(_ text: String")).upperBound
        let body = deck[start...].prefix(3200)
        XCTAssertTrue(body.contains("self?.answering.remove(id)"), "released when it's over, however it ends")
    }

    // MARK: A typed answer, to the menu's "Type something"

    func testOnlyAQuestionsMenuTakesATypedAnswer() throws {
        let choice = try XCTUnwrap(PendingChoice.question(question))
        XCTAssertTrue(choice.typesOther)
        XCTAssertEqual(choice.forPhone(menuSeen: true)["other"] as? Bool, true)
        XCTAssertFalse(try XCTUnwrap(PendingChoice.permission(tool: "Bash", input: [:], suggests: true)).typesOther)
        var multi = (question["questions"] as! [[String: Any]])[0]
        multi["multiSelect"] = true
        XCTAssertFalse(try XCTUnwrap(PendingChoice.question(["questions": [multi]])).typesOther)
        // Nine options leave no digit for it.
        let nine = (1...9).map { ["label": "Option \($0)"] }
        XCTAssertFalse(try XCTUnwrap(PendingChoice.question(["questions": [["question": "Which?", "options": nine]]])).typesOther)
    }

    /// Its Return goes once, while the question is still asked, never within
    /// a Return's repeat of another.
    func testATypedAnswersReturnIsClaimedOnce() {
        let now = Date()
        XCTAssertTrue(Deck.mayAnswer(state: .asking, newChoice: false, last: nil, now: now))
        XCTAssertFalse(Deck.mayAnswer(state: .asking, newChoice: false, last: now, now: now.addingTimeInterval(0.1)), "claimed already")
        XCTAssertTrue(Deck.mayAnswer(state: .asking, newChoice: false, last: now, now: now.addingTimeInterval(Deck.returnRepeat)))
        XCTAssertFalse(Deck.mayAnswer(state: .thinking, newChoice: false, last: nil, now: now), "answered on the Mac meanwhile")
        XCTAssertFalse(Deck.mayAnswer(state: nil, newChoice: false, last: nil, now: now), "the session has gone")
    }

    /// Cleaned and capped as dictation is: line breaks become spaces, and
    /// more than 2,000 characters types nothing.
    func testATypedAnswerIsCleanedAsDictationIs() throws {
        XCTAssertEqual(PhoneDictation.prepared("  Use\nPostgres \u{2028} please\t "), "Use Postgres please")
        XCTAssertNil(PhoneDictation.prepared(String(repeating: "a", count: PhoneDictation.longest + 1)))
        XCTAssertNotNil(PhoneDictation.prepared(String(repeating: "a", count: PhoneDictation.longest)))
        XCTAssertNil(PhoneDictation.prepared(" \n "))
    }

    /// In order: the phone mic's switch, the cleaning, the question still
    /// asked by that id, claimed before any key goes, then its digit, the
    /// text as keys, and the Return claimed on the main queue.
    func testATypedAnswerGoesInOrder() throws {
        let deck = try String(contentsOfFile: Self.root + "Sources/AgentDeck/Deck.swift", encoding: .utf8)
        let start = try XCTUnwrap(deck.range(of: "func answerOther(_ text: String")).upperBound
        let body = String(deck[start...].prefix(3400))
        let steps = [
            "guard Config.shared.phoneMicEnabled else",
            "guard let words = PhoneDictation.prepared(text) else",
            "guard !words.hasPrefix(\"!\") else",
            "guard let choice = Deck.claimable(session.choice, state: session.state, id: choiceID, lastReturn: lastReturn[id], now: Date()) else",
            "guard choice.typesOther, let other = Terminal.Key.digit(choice.options.count + 1),",
            "session.choice = nil",
            "answering.insert(id)",
            "PendingChoice.screen(screen, shows: look.number, startingWith: look.labels) else {",
            "guard driver.press(.init(key: other), in: tab) else",
            "guard driver.typeKeys(words, in: tab) else",
            "guard DispatchQueue.main.sync(execute: { self.claimAnswerReturn(id) }) else",
            "guard driver.press(.init(key: .returnKey), in: tab) else",
        ]
        let places = try steps.map { try XCTUnwrap(body.range(of: $0)?.lowerBound, $0) }
        XCTAssertEqual(places, places.sorted())
        XCTAssertFalse(body.contains("driver.insert("), "never a paste into the menu's field")
        XCTAssertFalse(body.contains("driver.type("), "never the text and Return together")
        // The terminals whose `insert` pastes type it as keys here.
        let drivers = try String(contentsOfFile: Self.root + "Sources/AgentDeck/TerminalDrivers.swift", encoding: .utf8)
        XCTAssertTrue(drivers.contains(#"func typeKeys(_ text: String, in tab: TabAddress) -> Bool { ran(tab, ["send-keys", "-t", tab.id, "-l", "--", text]) }"#))
        XCTAssertTrue(drivers.contains(#"func typeKeys(_ text: String, in tab: TabAddress) -> Bool { ran(tab, ["send-text", "--pane-id", tab.id, "--no-paste", "--", text]) }"#))
    }

    private static let root: String = {
        var url = URL(fileURLWithPath: #filePath)
        for _ in 0..<3 { url.deleteLastPathComponent() }
        return url.path + "/"
    }()
}

/// A terminal that shows what `show` says for the keys pressed so far and
/// the screen reads so far, and records the keys.
private final class FakeTerminal: TerminalDriver {
    private(set) var pressed: [Terminal.Key] = []
    private var reads = 0
    var takesKeys = true
    private let show: (_ pressed: Int, _ read: Int) -> String?

    init(_ show: @escaping (_ pressed: Int, _ read: Int) -> String?) { self.show = show }

    var abilities: TerminalAbilities { .keys }
    var keysNeedAccessibility: Bool { false }
    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard takesKeys else { return false }
        pressed.append(keystroke.key)
        return true
    }
    func type(_ text: String, in tab: TabAddress) -> Bool { false }
    func insert(_ text: String, in tab: TabAddress) -> Bool { false }
    func focus(_ tab: TabAddress) -> Bool { false }
    func scroll(lines: Int, in tab: TabAddress) -> Bool { false }
    func scrollToBottom(in tab: TabAddress) -> Bool { false }
    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired { .notStopped }
    func tty(of tab: TabAddress) -> String? { nil }
    func isOpen(_ tab: TabAddress) -> Bool? { nil }
    func screen(of tab: TabAddress) -> String? {
        reads += 1
        return show(pressed.count, reads)
    }
}
