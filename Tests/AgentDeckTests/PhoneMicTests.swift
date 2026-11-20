import XCTest
@testable import AgentDeck

/// The phone's microphone and text: the words it sends, how they are checked
/// and where they go. Offline.
final class PhoneMicTests: XCTestCase {
    // MARK: The text

    func testOneLineIsFlattenedAsBefore() {
        XCTAssertEqual(PhoneDictation.prepared("  fix the  login\tbug \r\n "), "fix the login bug")
        XCTAssertEqual(PhoneDictation.prepared("café, déjà vu"), "café, déjà vu")
        // A vertical tab or a form feed is a control character, not a line break.
        XCTAssertEqual(PhoneDictation.prepared("go\u{1b}[A\u{0b}on\u{0c}"), "go [A on")
        XCTAssertFalse(PhoneDictation.isLines("fix the login bug"))
    }

    /// Several lines stay several lines (I34): each break a line feed, for one bracketed paste to carry; the
    /// lines as they were, indentation and tabs kept; an Escape, which could end the paste early, a space.
    func testSeveralLinesAreKeptAsTheyWere() {
        XCTAssertEqual(PhoneDictation.prepared("  fix the\nlogin\tbug \r\n "), "fix the\nlogin\tbug")
        XCTAssertEqual(PhoneDictation.prepared("a\r\nb\rc\u{2028}d\u{2029}e\u{85}f"), "a\nb\nc\nd\ne\nf")
        let code = "Change this:\n\n    def f():\n\t\treturn 1;  \n« fin » 👩‍💻 $(touch x) `y` \\"
        XCTAssertEqual(PhoneDictation.prepared("\n\n" + code + "\n\n"), code, "blank lines at either end go, nothing else")
        XCTAssertEqual(PhoneDictation.prepared("one\u{1b}[201~\ntwo\u{7}"), "one [201~\ntwo")
        XCTAssertTrue(PhoneDictation.isLines("one\ntwo"))
        // The same size limit, the line breaks counted; the "!" rule on the first code point, once trimmed.
        XCTAssertNil(PhoneDictation.prepared(String(repeating: "a\n", count: PhoneDictation.longest / 2) + "a"))
        XCTAssertNotNil(PhoneDictation.prepared(String(repeating: "a\n", count: PhoneDictation.longest / 2 - 1) + "aa"))
        XCTAssertEqual(PhoneDictation.refusal(String(repeating: "a\n", count: PhoneDictation.longest)), PhoneDictation.tooLong)
        XCTAssertTrue(PhoneDictation.runsInShell(PhoneDictation.prepared("\n \n!\u{301}rm -rf x\nmore")!))
    }

    func testNothingOrTooMuchIsRefused() {
        XCTAssertNil(PhoneDictation.prepared(""))
        XCTAssertNil(PhoneDictation.prepared(" \n\t "))
        XCTAssertNil(PhoneDictation.prepared(String(repeating: "a", count: PhoneDictation.longest + 1)))
        XCTAssertNotNil(PhoneDictation.prepared(String(repeating: "a", count: PhoneDictation.longest)))
    }

    /// Only control characters go: the invisible joiner that makes 👩‍💻 one
    /// emoji is a format character, kept, as are accents and French quotes.
    func testEmojiAndAccentsArriveWhole() {
        let french = "Café à l’été, ça va ? « Œuvre » é è à ç ô œ 🎉👍🏽 👩‍💻 ❤️ 👨‍👩‍👧"
        XCTAssertEqual(PhoneDictation.prepared(french), french)
        XCTAssertEqual(PhoneDictation.prepared("a\u{200D}b\u{00AD}c"), "a\u{200D}b\u{00AD}c")
        XCTAssertEqual(PhoneDictation.prepared(#"He said "hi" $HOME `x` $(y) ; a && b \"#), #"He said "hi" $HOME `x` $(y) ; a && b \"#)
    }

    /// Too long is said as such: "Nothing to type" for 2,001 characters sent the owner looking for nothing.
    func testTooLongSaysSo() {
        XCTAssertEqual(PhoneDictation.refusal(String(repeating: "a", count: PhoneDictation.longest + 1)), "Too long: at most 2000 characters at a time.")
        XCTAssertEqual(PhoneDictation.refusal(" \n "), "Nothing to type")
        XCTAssertEqual(PhoneDictation.refusal(""), "Nothing to type")
        // One character can carry any number of combining marks: the bytes are capped too.
        let heavy = "e" + String(repeating: "\u{301}", count: 20_000)
        XCTAssertEqual(heavy.count, 1)
        XCTAssertNil(PhoneDictation.prepared(heavy))
        XCTAssertEqual(PhoneDictation.refusal(heavy), PhoneDictation.tooLong)
        // 2,000 emoji of several pieces still go.
        XCTAssertNotNil(PhoneDictation.prepared(String(repeating: "👍🏽", count: PhoneDictation.longest)))
    }

    // MARK: On a deck of its own, with a stand-in terminal

    /// What's known before the terminal is asked: a terminal that can't, an agent not known to ask for
    /// bracketed paste, ⌘V without Accessibility or with the Mac locked.
    func testWhatRefusesSeveralLinesUpFront() {
        let yes = { true }, no = { false }
        XCTAssertNil(Deck.linesRefusal(.paneFlag, agent: .grok, accessibility: no, locked: yes), "tmux reads the pane itself")
        XCTAssertEqual(Deck.linesRefusal(.never, agent: .claude)?.status, 409)
        XCTAssertEqual(Deck.linesRefusal(.clipboard(inFront: false), agent: .grok)?.status, 409)
        XCTAssertNil(Deck.linesRefusal(.clipboard(inFront: false), agent: .claude, accessibility: no, locked: yes), "Ghostty needs neither")
        XCTAssertEqual(Deck.linesRefusal(.clipboard(inFront: true), agent: .claude, accessibility: no, locked: no)?.status, 403)
        let locked = Deck.linesRefusal(.clipboard(inFront: true), agent: .claude, accessibility: yes, locked: yes)
        XCTAssertEqual(locked?.status, 409)
        XCTAssertEqual(locked.map { String(decoding: $0.body, as: UTF8.self) }, Deck.lockedLines)
        XCTAssertNil(Deck.linesRefusal(.clipboard(inFront: true), agent: .claude, accessibility: yes, locked: no))
        // Claude Code only (RI34c B1): OpenCode's binary may turn bracketed paste on only where the terminal
        // answers its query; Codex and Gemini CLI weren't there to read; Grok CLI's wasn't checked. Each is
        // refused in one sentence, in Terminal, iTerm2 and Ghostty alike; tmux still goes by its pane flag.
        XCTAssertTrue(Agent.claude.asksForBracketedPaste)
        for agent in [Agent.opencode, .codex, .gemini, .grok] {
            XCTAssertFalse(agent.asksForBracketedPaste, agent.name)
            for inFront in [false, true] {
                let refused = Deck.linesRefusal(.clipboard(inFront: inFront), agent: agent, accessibility: yes, locked: no)
                XCTAssertEqual(refused?.status, 409, agent.name)
                XCTAssertEqual(refused.map { String(decoding: $0.body, as: UTF8.self) }, Deck.noLines(for: agent), agent.name)
            }
            XCTAssertNil(Deck.linesRefusal(.paneFlag, agent: agent), agent.name)
        }
    }

    /// Several lines through the clipboard only while the desk waits at its main prompt, the agent alone in
    /// front of its tab (RI34b B1): each case where bracketed paste may be off, or a Return could land
    /// elsewhere, is refused in its own sentence. Not known is refused too.
    func testTheClipboardPasteGoesOnlyToAnAgentWaitingAtItsPrompt() {
        let alone = Terminal.TabFront(place: .front, readers: [])
        func with(_ readers: Set<String>) -> Terminal.TabFront { Terminal.TabFront(place: .front, readers: readers) }
        for state in [SessionState.idle, .unread, .error] { XCTAssertNil(Deck.clipboardRefusal(state: state, front: alone), "\(state)") }
        // Claude Code hands the tab to a terminal $EDITOR (Ctrl-G, opening a file), bracketed paste off: vim, nano,
        // an app's --wait run as a script.
        for editor: Set<String> in [["vim"], ["nvim"], ["nano"], ["emacs"], ["emacs-29.4"], ["code", "bash"], ["hx"]] {
            XCTAssertEqual(Deck.clipboardRefusal(state: .idle, front: with(editor)), Deck.linesInEditor, "\(editor)")
        }
        XCTAssertEqual(Deck.clipboardRefusal(state: .idle, front: with(["less"])), Deck.linesInPager, "a pager")
        XCTAssertEqual(Deck.clipboardRefusal(state: .idle, front: with(["man"])), Deck.linesInPager, "a manual page")
        XCTAssertEqual(Deck.clipboardRefusal(state: .idle, front: with(["zsh"])), Deck.linesInShell, "a ! shell")
        XCTAssertEqual(Deck.clipboardRefusal(state: .idle, front: with(["bash", "sleep"])), Deck.linesInShell)
        // Whatever it's called, what reads the tab's keys is refused (RI34c S1): the names only pick the sentence.
        for unlisted: Set<String> in [["vis"], ["nvi"], ["mcedit"], ["?"]] {
            XCTAssertEqual(Deck.clipboardRefusal(state: .idle, front: with(unlisted)), Deck.linesInTab(unlisted), "\(unlisted)")
        }
        XCTAssertEqual(Deck.clipboardRefusal(state: .asking, front: alone), Deck.answerFirst, "a menu: a question or a permission")
        XCTAssertEqual(Deck.clipboardRefusal(state: .thinking, front: alone), Deck.linesWhileWorking,
                       "a sub-prompt: a tool's own prompt runs while the turn does")
        XCTAssertEqual(Deck.clipboardRefusal(state: nil, front: alone), Deck.linesUnknown, "unknown state")
        XCTAssertEqual(Deck.clipboardRefusal(state: .idle, front: nil), Deck.linesUnknown, "a tab that couldn't be looked at")
        XCTAssertEqual(Deck.clipboardRefusal(state: .idle, front: Terminal.TabFront(place: .behind, readers: ["vim"])), Deck.linesBehind)
        XCTAssertEqual(Deck.clipboardRefusal(state: .idle, front: Terminal.TabFront(place: .gone, readers: [])), Deck.linesAgentEnded)
    }

    /// What reads a tab's keys, from a stand-in process table and stand-in standard inputs (RI34c S1): every
    /// process of the foreground job but the agent itself with the tab's tty as its fd 0. Nothing real is looked at.
    func testWhatReadsTheTabsKeysIsWhatHasItsTTYAsInput() {
        let tty = "/dev/ttys041"
        typealias Input = Processes.StandardInput
        func readers(_ job: [(pid: pid_t, args: String, input: Input, name: String?)], agent: Agent = .claude,
                     paths: [pid_t: String] = [:]) -> Set<String>? {
            Terminal.readers(of: agent, in: job.map { ($0.pid, $0.args) }, tty: tty,
                             input: { pid in job.first { $0.pid == pid }?.input ?? .other },
                             name: { pid in job.first { $0.pid == pid }?.name },
                             path: { paths[$0] })
        }
        let claude = (pid: pid_t(100), args: "claude --resume", input: Input.file(tty), name: String?("2.1.322"))
        // Claude Code's own job, as on a real tab: caffeinate and the MCP servers have /dev/null or a pipe.
        let helpers: [(pid: pid_t, args: String, input: Input, name: String?)] = [
            (101, "caffeinate -i -t 300", .file("/dev/null"), "caffeinate"),
            (102, "npm exec @modelcontextprotocol/server-filesystem /tmp", .other, "node"),
            (103, "uv run mcp-server", .other, "uv"),
            (104, "/usr/bin/python3 -m server", .other, "python3"),
        ]
        XCTAssertEqual(readers([claude] + helpers), [], "the agent alone reads the keys")
        // A hook's or the status line's sh -c reads a pipe: not refused, as the name list did.
        XCTAssertEqual(readers([claude, (110, "/bin/sh -c ~/.claude/statusline.sh", .other, "sh")]), [])
        // An unlisted terminal editor with the tty: refused, by its own name.
        XCTAssertEqual(readers([claude, (120, "vis notes.md", .file(tty), "vis")] + helpers), ["vis"])
        // An editor whose path has a space: the program's own name, whole, as well as the command line's first word.
        XCTAssertEqual(readers([claude, (130, "/Users/me/Editor Tools/nvim notes.md", .file(tty), "nvim")]), ["nvim", "editor"])
        XCTAssertEqual(Deck.clipboardRefusal(state: .idle, front: Terminal.TabFront(place: .front, readers: ["nvim", "editor"])),
                       Deck.linesInEditor)
        // A window editor run as a script (code --wait): bash runs it, and its first word says code.
        XCTAssertEqual(readers([claude, (140, "/bin/bash /usr/local/bin/code --wait x", .file(tty), "bash")]), ["bash"])
        XCTAssertEqual(readers([claude, (141, "code --wait x", .file(tty), "bash")]), ["code", "bash"])
        // Nameless, it's still refused.
        XCTAssertEqual(readers([claude, (150, "", .file(tty), nil)]), ["?"])
        // Another tty or a file isn't this tab's keys; a process that couldn't be looked at (sudo's) isn't known.
        XCTAssertEqual(readers([claude, (160, "tail -f log", .file("/dev/ttys042"), "tail"), (161, "cat", .file("/tmp/x"), "cat")]), [])
        XCTAssertNil(readers([claude, (170, "sudo ls", .unknown, nil)]))
        // The tty given without /dev/, as ps names it.
        XCTAssertEqual(Terminal.readers(of: .claude, in: [(120, "vis")], tty: "ttys041", input: { _ in .file(tty) }, name: { _ in nil }, path: { _ in nil }), ["vis"])
        // `caffeinate -di claude` (FXI34d): the system's caffeinate has the tty as its input and never reads it.
        // Only its whole path says so: a name alone, or another path, exempts nothing.
        let wrapper = (pid: pid_t(105), args: "caffeinate -di claude --resume x", input: Input.file(tty), name: String?("caffeinate"))
        XCTAssertEqual(readers([wrapper, claude], paths: [105: "/usr/bin/caffeinate"]), [], "the system's caffeinate")
        XCTAssertEqual(readers([wrapper, claude], paths: [105: "/tmp/tools/caffeinate"]), ["caffeinate"], "another file called caffeinate")
        XCTAssertEqual(readers([wrapper, claude], paths: [105: "/usr/bin/caffeinate-fake"]), ["caffeinate"], "not exactly that path")
        XCTAssertEqual(readers([wrapper, claude]), ["caffeinate"], "no path known: not exempt")
        // It exempts only itself: an editor next to it still refuses.
        XCTAssertEqual(readers([wrapper, claude, (121, "vis notes.md", .file(tty), "vis")], paths: [105: "/usr/bin/caffeinate"]), ["vis"])
        // The refusal says who has the keys, so the owner can find it.
        XCTAssertTrue(Deck.linesInTab(["caffeinate"]).hasPrefix("caffeinate has this tab's keys"), Deck.linesInTab(["caffeinate"]))
        XCTAssertTrue(Deck.linesInTab(["vis", "wrap"]).hasPrefix("vis and wrap have this tab's keys"))
        XCTAssertTrue(Deck.linesInTab(["?"]).hasPrefix("Something else"), "nameless")
        XCTAssertEqual(Deck.clipboardRefusal(state: .idle, front: Terminal.TabFront(place: .front, readers: ["caffeinate"])),
                       Deck.linesInTab(["caffeinate"]))
        // The agent itself is left out by its command line, whatever its program's file is called.
        XCTAssertEqual(readers([(200, "node /opt/homebrew/lib/node_modules/@anthropic-ai/claude-code/cli.js", .file(tty), "node")]), [])
    }

    /// The real lookups, on processes of the test's own: /dev/null and a pipe aren't a tty, and a process that
    /// has ended has nothing there. Only the kernel is asked, about the test's own children.
    func testTheRealStandardInputLookup() throws {
        let null = Process()
        null.executableURL = URL(fileURLWithPath: "/bin/sleep")
        null.arguments = ["5"]
        null.standardInput = FileHandle.nullDevice
        let piped = Process()
        piped.executableURL = URL(fileURLWithPath: "/bin/sleep")
        piped.arguments = ["5"]
        piped.standardInput = Pipe()
        try null.run()
        try piped.run()
        defer { null.terminate(); piped.terminate() }
        XCTAssertEqual(Processes.standardInput(of: null.processIdentifier), .file("/dev/null"))
        XCTAssertEqual(Processes.standardInput(of: piped.processIdentifier), .other)
        XCTAssertEqual(Processes.programName(of: piped.processIdentifier), "sleep")
        XCTAssertEqual(Processes.programPath(of: piped.processIdentifier), "/bin/sleep")
        null.terminate()
        null.waitUntilExit()
        XCTAssertEqual(Processes.standardInput(of: null.processIdentifier), .other, "ended")
        XCTAssertEqual(Processes.standardInput(of: 1), .unknown, "launchd is root's: not known")
    }

    /// Each way several lines didn't go is said in its own sentence; ⌘V that went but wasn't read in time is
    /// a 503, so the page doesn't offer the words to send twice.
    func testEachWayTheClipboardPasteDidntGoIsSaid() {
        func said(_ pasted: LinesPasted) -> (Int, String)? {
            Deck.linesResponse(pasted).map { ($0.status, String(decoding: $0.body, as: UTF8.self)) }
        }
        XCTAssertNil(said(.pasted))
        XCTAssertNil(said(.failed), "said as any key that didn't go")
        XCTAssertEqual(said(.notInFront)?.1, Deck.linesNotInFront, "not 404 'Terminal tab not found' (RI34 N2)")
        XCTAssertEqual(said(.macInUse)?.1, Deck.linesMacInUse)
        XCTAssertEqual(said(.unsaved)?.1, Deck.linesClipboardKept)
        XCTAssertEqual(said(.refused("x"))?.1, "x")
        for pasted in [LinesPasted.notTaken, .inMode, .notInFront, .macInUse, .unsaved, .refused("x")] { XCTAssertEqual(said(pasted)?.0, 409) }
        XCTAssertEqual(said(.notRead)?.0, 503)
        XCTAssertEqual(said(.notRead)?.1, Deck.linesNotRead)
    }

    func testItSendsFromThreeWordsWithTheSettingOn() {
        XCTAssertFalse(PhoneDictation.sends("yes", autoSend: true))
        XCTAssertFalse(PhoneDictation.sends("yes please", autoSend: true))
        XCTAssertTrue(PhoneDictation.sends("yes please go", autoSend: true))
        XCTAssertFalse(PhoneDictation.sends("yes please go", autoSend: false))
    }

    // MARK: An agent asking you something

    /// The page holds the box's Send ⏎ and Type only while that agent asks, and says why.
    func testTheBoxWaitsWhileTheAgentAsks() throws {
        let page = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/deck.html"), encoding: .utf8)
        let check = try XCTUnwrap(page.range(of: "function confirmable() {"))
        let rule = page[check.upperBound...].prefix(400)
        XCTAssertTrue(rule.contains("const asking = !!s && s.state === 'asking';"))
        XCTAssertTrue(rule.contains("$('confirm-send').disabled = asking;"))
        XCTAssertTrue(rule.contains("$('confirm-type').disabled = asking;"))
        XCTAssertTrue(rule.contains("$('confirm-why').hidden = !asking;"))
        XCTAssertTrue(page.contains("if (!session || $('confirm-send').disabled) return;"), "a tap on a held button sends nothing")
        // As the computer's word on the state comes in, and when the box opens.
        let receive = try XCTUnwrap(page.range(of: "function receive(data) {"))
        XCTAssertTrue(page[receive.upperBound...].prefix(1800).contains("confirmable();"))
        XCTAssertTrue(page.contains("$('confirm').hidden = false;\n    confirmable();"))
        XCTAssertTrue(page.contains("It's asking you something: answer it on the deck first."))
    }

    /// The page (I34): the box keeps several lines; Settings opens it to type where the phone's mic can't
    /// listen, as over the Wi-Fi, and says in one line that the Wi-Fi isn't encrypted; the box stays open
    /// over plain http; the peek is asked for only over HTTPS; refused words go back in the box.
    func testThePageTypesOverTheWiFiAndKeepsTheLines() throws {
        let page = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/deck.html"), encoding: .utf8)
        XCTAssertTrue(page.contains("const text = $('confirm-text').value.replace(/\\r\\n?/g, '\\n').trim();"))
        XCTAssertFalse(page.contains("$('confirm-text').value.replace(/\\s+/g, ' ')"), "never flattened")
        XCTAssertTrue(page.contains("row(`Type to ${picked.project || picked.name}`, button('Type…', null, () => {"))
        XCTAssertTrue(page.contains("if (picked && picked.phoneMic) {"), "only where the computer types into that desk")
        XCTAssertTrue(page.contains("textContent: \"Over the Wi-Fi, what you type isn't encrypted. Tailscale is the private option.\""))
        XCTAssertTrue(page.contains("if (!preview && !whole && phoneMic.on && location.protocol !== 'https:') {"))
        XCTAssertTrue(page.contains("if (!phoneMicHere()) dropHearing();\n    // The box goes with the phone's text"))
        XCTAssertTrue(page.contains("if (location.protocol === 'https:') setTimeout(() => peek(session), PEEK_AFTER);"))
        XCTAssertTrue(page.contains("if (send !== undefined && res.status < 500) confirmDictation(session, text);"))
        // A clipboard manager may keep a copy of several lines pasted through the clipboard (Q1 b), said in the
        // phone's ⚙︎, in setup's Phone microphone text with the Wi-Fi's line (RI34 N5), and in docs/phone.md.
        let clipboard = "Several lines typed to Claude Code in Terminal, iTerm2 or Ghostty go through the clipboard: a clipboard manager, or iTerm2's paste history, may keep a copy, and a password you'd copied is cleared, not put back."
        XCTAssertTrue(page.contains("textContent: \"\(clipboard)\""))
        let setup = try String(contentsOf: Self.repo.appendingPathComponent("Resources/web/setup.html"), encoding: .utf8)
        XCTAssertTrue(setup.contains("over the Wi-Fi too, where what you type isn't encrypted: Tailscale is the private option. \(clipboard)\"></div>"))
        // The doc's safety notes, whatever its line wrapping: lines joined with single spaces.
        let docs = try String(contentsOf: Self.repo.appendingPathComponent("docs/phone.md"), encoding: .utf8)
            .split(whereSeparator: \.isWhitespace).joined(separator: " ")
        XCTAssertTrue(docs.contains("A clipboard manager, or iTerm2's paste history, may keep a copy of text sent this way."), "RI34c N3")
        XCTAssertTrue(docs.contains("and ⌘V, for Claude Code only,"), "RI34c B1")
        XCTAssertFalse(docs.contains("OpenCode waiting"), "RI34c B1")
        XCTAssertTrue(docs.contains("idle, so Smolcorp still reads it as waiting: several lines go to that mode or"), "RI34c's residual note")
        XCTAssertTrue(docs.contains("password you'd copied, which is cleared, not put back"), "VICTOR's ruling on RI34b S2")
        XCTAssertFalse(page.contains("aria-label=\"What the phone heard\" hidden"), "the box is for typed text too (RI34 N1)")
    }

    // MARK: The peek

    func testThePeekIsTheLastLinesAsText() {
        let screen = (1...30).map { "line \($0)" }.joined(separator: "\n") + "\n\n   \n"
        XCTAssertEqual(PhoneDictation.peek(screen), (21...30).map { "line \($0)" })
        XCTAssertEqual(PhoneDictation.peek("> fix the login bug\u{1b}[0m\u{7}"), ["> fix the login bug[0m"])
        XCTAssertEqual(PhoneDictation.peek(String(repeating: "─", count: 500)).first?.count, PhoneDictation.peekWidth)
        XCTAssertEqual(PhoneDictation.peek(""), [])
        XCTAssertEqual(PhoneDictation.peek("\n\n"), [])
        XCTAssertEqual(PhoneDictation.peek(String(repeating: "\n", count: 12) + "──\n❯ hi\n──\n"), ["──", "❯ hi", "──"])
    }

    static let repo = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    /// Deck.swift from `start` up to `end`.
    static func body(of start: String, until end: String) throws -> String {
        let deck = try String(contentsOf: repo.appendingPathComponent("Sources/AgentDeck/Deck.swift"), encoding: .utf8)
        let from = try XCTUnwrap(deck.range(of: start), start)
        let to = try XCTUnwrap(deck.range(of: end, range: from.upperBound..<deck.endIndex), end)
        return String(deck[from.upperBound..<to.lowerBound])
    }

    /// What's inside the braces `opening` opens, matched brace for brace.
    static func block(after opening: String, in text: String) -> String? {
        guard let start = text.range(of: opening) else { return nil }
        var depth = 1
        var index = start.upperBound
        while index < text.endIndex {
            switch text[index] {
            case "{": depth += 1
            case "}":
                depth -= 1
                if depth == 0 { return String(text[start.upperBound..<index]) }
            default: break
            }
            index = text.index(after: index)
        }
        return nil
    }

    // MARK: Settings

    func testThePhoneMicIsOnByDefaultAndKeepsItsSettings() throws {
        let fresh = try JSONDecoder().decode(Config.self, from: Data("{}".utf8))
        XCTAssertTrue(fresh.phoneMicEnabled)
        XCTAssertFalse(fresh.phoneMicAutoSend, "the confirm box is the default")
        XCTAssertFalse(Config().phoneMicAutoSend)
        let off = try JSONDecoder().decode(Config.self, from: Data(#"{"phoneMicEnabled":false,"phoneMicSkipsConfirm":false}"#.utf8))
        XCTAssertFalse(off.phoneMicEnabled)
        XCTAssertFalse(off.phoneMicAutoSend)
        let again = try JSONDecoder().decode(Config.self, from: JSONEncoder().encode(off))
        XCTAssertFalse(again.phoneMicEnabled)
        XCTAssertFalse(again.phoneMicAutoSend)
        let on = try JSONDecoder().decode(Config.self, from: Data(#"{"phoneMicSkipsConfirm":true}"#.utf8))
        XCTAssertTrue(try JSONDecoder().decode(Config.self, from: JSONEncoder().encode(on)).phoneMicAutoSend)
    }

    /// Every earlier build saved "Send automatically" as true without asking:
    /// that file starts with the confirm box, and drops the old name.
    func testAnEarlierBuildsAutoSendGivesWayToTheBox() throws {
        let earlier = try JSONDecoder().decode(Config.self, from: Data(#"{"phoneMicEnabled":true,"phoneMicAutoSend":true}"#.utf8))
        XCTAssertTrue(earlier.phoneMicEnabled)
        XCTAssertFalse(earlier.phoneMicAutoSend)
        XCTAssertFalse(earlier.lostOnLoad)
        let saved = String(decoding: try JSONEncoder().encode(earlier), as: UTF8.self)
        XCTAssertTrue(saved.contains(#""phoneMicSkipsConfirm":false"#))
        XCTAssertFalse(saved.contains(#""phoneMicAutoSend""#))
    }

    /// The phone's text, one table (Tests/fixtures/bridge/phone-text.json, IGL): each case as the Go core asks
    /// it, through the bridge's phone-text action, with the desk's state the core would say, typed through
    /// `Deck.typePhoneWords`: its refusals in its sentences, and the bridge's answer the one the Go core reads
    /// (desktop/phonetext_test.go runs the same table through it). The table's "swift" answers were the Swift
    /// engine's, gone in 1.25.0. Nothing real is looked at or told anything.
    func testThePhonesTextTableThroughTheBridge() throws {
        guard Config.shared.phoneMicEnabled else { return XCTFail("the phone mic is on by default") }
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("fixtures/bridge/phone-text.json")
        let fixture = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])
        let cases = try XCTUnwrap(fixture["cases"] as? [[String: Any]])
        XCTAssertGreaterThanOrEqual(cases.count, 6)
        let gates = (Deck.keyboardAllowed, Deck.macLocked, Deck.tabFront)
        addTeardownBlock { (Deck.keyboardAllowed, Deck.macLocked, Deck.tabFront) = gates }
        Deck.keyboardAllowed = { true }
        Deck.macLocked = { false }
        func json(_ object: Any) -> String {
            String(decoding: (try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])) ?? Data(), as: UTF8.self)
        }
        for one in cases {
            let name = try XCTUnwrap(one["name"] as? String)
            let agent = try XCTUnwrap(Agent(rawValue: one["agent"] as? String ?? ""))
            let desk = try XCTUnwrap(SessionState(rawValue: one["desk"] as? String ?? ""))
            let front = try XCTUnwrap(one["front"] as? [String: Any])
            let place: Terminal.Place = ["front": .front, "behind": .behind, "gone": .gone][front["place"] as? String ?? ""] ?? .gone
            Deck.tabFront = { _, _ in Terminal.TabFront(place: place, readers: Set(front["readers"] as? [String] ?? [])) }
            let pastes: LinesPasted = ["pasted": .pasted, "macInUse": .macInUse, "notRead": .notRead][one["terminal"] as? String ?? ""] ?? .failed
            var request = try XCTUnwrap(one["request"] as? [String: Any])
            let text = try XCTUnwrap(request["text"] as? String)
            let typed = one["typed"] as? Bool == true, returns = one["returns"] as? Bool == true

            // The bridge, as the Go core asks it: the same driver, the desk's state as the core has it.
            let bridged = RecordingTerminal()
            bridged.linesWay = .clipboard(inFront: true)
            bridged.lines = pastes
            var mac = HostBridge.Actions(press: { _, _ in false }, type: { _, _ in false }, insert: { _, _ in false }, focus: { _ in false },
                                         scroll: { _, _ in false }, scrollToBottom: { _ in false }, fire: { _, _ in .notStopped },
                                         hires: FileManager.default.temporaryDirectory, open: { _, _, _ in nil }, secrets: [:], loginItem: nil)
            mac.driver = { _ in bridged }
            mac.place = { _, _ in nil }
            mac.screen = { _ in nil }
            let queue = DispatchQueue(label: "test.phone-text")
            mac.queue = { _ in queue }
            var asked: [String] = []
            request["token"] = "fake-token"
            let answer = HostBridge.answer(try JSONSerialization.data(withJSONObject: request), token: "fake-token", actions: mac,
                                           until: .now() + HostBridge.actionWait, deskState: { asked.append(desk.rawValue); return desk })
            XCTAssertEqual(json(answer), json(one["answer"] ?? NSNull()), name)
            XCTAssertEqual(asked, one["asked"] as? [String], "\(name): the core asked")
            XCTAssertEqual(!(bridged.inserted + bridged.insertedLines).isEmpty, typed, "\(name): typed through the bridge")
            XCTAssertEqual(bridged.pressed, [], "\(name): the bridge never presses Return: the core does, after")
        }
    }
}

/// A terminal that records the text and keys it's given, and can run something as it's asked where the agent is.
private final class RecordingTerminal: TerminalDriver {
    private(set) var inserted: [String] = []
    private(set) var insertedLines: [String] = []
    private(set) var pressed: [Terminal.Key] = []
    var takesKeys = true
    /// How text of several lines goes: not taken, as by a tmux pane whose agent didn't ask, unless a test says so.
    var lines = LinesPasted.notTaken
    var linesWay = LinesWay.paneFlag
    var whenLookedUp: (() -> Void)?
    /// Run as the terminal gets ready to paste, before it asks whether the desk is still free: as the tab
    /// is brought forward for ⌘V.
    var beforePaste: (() -> Void)?
    private(set) var askedToPasteLines = 0

    var abilities: TerminalAbilities { [.keys, .text] }
    var keysNeedAccessibility: Bool { false }
    func press(_ keystroke: Terminal.Keystroke, in tab: TabAddress) -> Bool {
        guard takesKeys else { return false }
        pressed.append(keystroke.key)
        return true
    }
    func type(_ text: String, in tab: TabAddress) -> Bool { false }
    func insert(_ text: String, in tab: TabAddress) -> Bool {
        inserted.append(text)
        return true
    }
    func insertLines(_ text: String, in tab: TabAddress, ready: () -> String?) -> LinesPasted {
        askedToPasteLines += 1
        beforePaste?()
        if let why = ready() { return .refused(why) }
        if lines == .pasted { insertedLines.append(text) }
        return lines
    }
    func focus(_ tab: TabAddress) -> Bool { false }
    func scroll(lines: Int, in tab: TabAddress) -> Bool { false }
    func scrollToBottom(in tab: TabAddress) -> Bool { false }
    func fire(_ tab: TabAddress, agent: Agent) -> Terminal.Fired { .notStopped }
    /// Unknown: the keys go as they always did (see `Deck.toAgent`), and nothing real is asked.
    func tty(of tab: TabAddress) -> String? {
        whenLookedUp?()
        return nil
    }
    func isOpen(_ tab: TabAddress) -> Bool? { nil }
    func screen(of tab: TabAddress) -> String? { nil }
}
