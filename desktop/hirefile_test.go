package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// The core's reader (hirefile.go, SA-X5): what a desk writes is read only below the place the core chose, through no
// link, from a plain file of one name, up to a cap. Every row uses a stand-in key, a file of the test's own whose text
// is shaped as what the reader would show (a transcript reply, a STATUS line), so a read that followed a link to it
// would show its words. Each row has a plain-file control, so a refusal is the reader's and not an empty fixture's.

// projectsFolder is a new project folder of the test home's ~/.claude/projects, as Claude Code keeps a project's
// transcripts there; removed after the test.
func projectsFolder(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(homeDir, ".claude", "projects", "-x5-"+randomHex(6))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// standInKey is the stand-in secret: never a real key, only words a test looks for.
const standInKey = "STAND-IN-KEY-sax5-not-a-secret"

// standInKeyFile is a file outside every desk's place holding the stand-in key, as a transcript reply (its model and
// its words) and as the last line of a STATUS.md would hold it.
func standInKeyFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	writeFile(t, path, `{"type":"assistant","message":{"role":"assistant","model":"`+standInKey+`","content":[{"type":"text","text":"`+
		standInKey+`"}]}}`+"\n")
	return path
}

// linkOrSkip makes a symbolic link, or skips where this account can't make one: Windows without the privilege, and
// only that. Any other failure (a folder not there, say) fails the row, so a row never skips unseen (RT126SAX5 N1).
func linkOrSkip(t *testing.T, target, at string) {
	t.Helper()
	err := os.Symlink(target, at)
	if err == nil {
		return
	}
	if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(1314)) { // ERROR_PRIVILEGE_NOT_HELD
		t.Skipf("no link here: %v", err)
	}
	t.Fatalf("the link wasn't made: %v", err)
}

// claudeProjects is the test home's ~/.claude/projects, made: Claude Code's own folder of every project's transcripts.
func claudeProjects(t *testing.T) string {
	t.Helper()
	projects := filepath.Join(homeDir, ".claude", "projects")
	if err := os.MkdirAll(projects, 0o700); err != nil {
		t.Fatal(err)
	}
	return projects
}

func TestTheReaderTakesOnlyAPlainFileOfOneNameBelowItsPlaceThroughNoLink(t *testing.T) {
	place := t.TempDir()
	key := standInKeyFile(t, "key.md")
	plain := filepath.Join(place, "ok", "STATUS.md")
	writeFile(t, plain, "fine\n")
	if data, err := readHireFile(place, plain, 5); err != nil || string(data) != "fine\n" {
		t.Fatalf("the plain file, at its cap: %q %v", data, err)
	}
	refused := map[string][2]string{ // row: place, path
		"no place":          {"", plain},
		"a relative path":   {place, filepath.Join("ok", "STATUS.md")},
		"outside its place": {place, key},
		"up and out":        {place, filepath.Join(place, "..", filepath.Base(filepath.Dir(key)), "key.md")},
		"the place itself":  {place, place},
		"a folder":          {place, filepath.Join(place, "ok")},
		"nothing there":     {place, filepath.Join(place, "gone.md")},
	}
	if err := os.Link(key, filepath.Join(place, "hard.md")); err == nil {
		refused["a hard link to the key"] = [2]string{place, filepath.Join(place, "hard.md")}
	} else {
		t.Logf("no hard link here: %v", err)
	}
	if os.Symlink(key, filepath.Join(place, "last.md")) == nil && os.Symlink(filepath.Dir(key), filepath.Join(place, "through")) == nil &&
		os.Symlink(filepath.Join(place, "through"), filepath.Join(place, "ok", "deeper")) == nil {
		refused["the file a link to the key"] = [2]string{place, filepath.Join(place, "last.md")}
		refused["a folder on the way a link"] = [2]string{place, filepath.Join(place, "through", "key.md")}
		refused["a link two folders down"] = [2]string{place, filepath.Join(place, "ok", "deeper", "key.md")}
		// Links that stay inside the place: os.Root alone would follow them.
		if os.Symlink("ok", filepath.Join(place, "inside")) == nil && os.Symlink("STATUS.md", filepath.Join(place, "ok", "same.md")) == nil {
			refused["a folder linked inside its place"] = [2]string{place, filepath.Join(place, "inside", "STATUS.md")}
			refused["a file linked inside its place"] = [2]string{place, filepath.Join(place, "ok", "same.md")}
		}
	} else {
		t.Log("no links here: their rows are left out")
	}
	for row, c := range refused {
		data, err := readHireFile(c[0], c[1], 1<<20)
		if err == nil || strings.Contains(string(data), standInKey) {
			t.Errorf("%s: read %q (%v)", row, data, err)
		}
		if file, _, err := openHireFile(c[0], c[1]); err == nil {
			file.Close()
			t.Errorf("%s: opened", row)
		}
	}
	if data, err := readHireFile(place, plain, 4); err == nil {
		t.Errorf("past its cap: %q", data)
	}
}

func TestATranscriptIsTakenOnlyFromAClaudeFolderOfTheOwnersByItsPlace(t *testing.T) {
	d := newTestDeck(t)
	work := filepath.Join(homeDir, ".claude-x5work")
	if err := os.MkdirAll(filepath.Join(work, "projects", "-w"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(work) })
	d.mu.Lock()
	d.config.ClaudeProfiles = []claudeProfile{{Name: "x5work", Dir: work}}
	d.mu.Unlock()
	inSupport := filepath.Join(supportDir, "x5-key.jsonl")
	writeFile(t, inSupport, `{"type":"assistant","message":{"model":"`+standInKey+`"}}`+"\n")
	t.Cleanup(func() { os.Remove(inSupport) })
	shaped := filepath.Join(t.TempDir(), ".claude", "projects", "-p", "s.jsonl")
	writeFile(t, shaped, `{"type":"assistant","message":{"model":"`+standInKey+`"}}`+"\n")
	for i, c := range []struct {
		path string
		kept bool
	}{
		{filepath.Join(projectsFolder(t), "s.jsonl"), true},
		{filepath.Join(work, "projects", "-w", "s.jsonl"), true},
		{inSupport, false},
		{filepath.Join(supportDir, acpTranscriptFolder, "a.jsonl"), false},
		{shaped, false}, // Claude Code's shape, but no folder of the owner's
		{filepath.Join(homeDir, ".claude", "s.jsonl"), false},
		{filepath.Join(homeDir, ".claude", "projects", "-p", "s.json"), false},
		{"projects/-p/s.jsonl", false},
	} {
		id := "s-place-" + string(rune('a'+i))
		d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": id, "cwd": "/w", "transcript_path": c.path},
			"claude", "tty:/dev/ttys06"+string(rune('0'+i)), "", 10)
		d.mu.Lock()
		got, model := d.sessions.Get(id).Transcript, d.sessions.Get(id).Model
		d.mu.Unlock()
		if (got == c.path) != c.kept || !c.kept && got != "" || model == standInKey {
			t.Errorf("%s: kept as %q (model %q), want kept %v", c.path, got, model, c.kept)
		}
	}
	// Saved before by place, or by hand: a transcript elsewhere is let go as the deck comes back.
	d.mu.Lock()
	d.restore([]savedSession{{ID: "s-saved", Agent: "claude", Tab: "tty:/dev/ttys070", Transcript: inSupport}}, savedDeck{}, true)
	saved := d.sessions.Get("s-saved")
	d.mu.Unlock()
	if saved == nil || saved.Transcript != "" {
		t.Errorf("a saved transcript outside the owner's Claude folders was kept: %+v", saved)
	}
}

// P4 and N2's row: a transcript in its place, but linked to a stand-in key (the file, its project folder, or a hard
// link). The model, the summary's tail, the conversation, the session card and the talk card read nothing of it.
func TestATranscriptLinkedToAStandInKeyIsNeverRead(t *testing.T) {
	key := standInKeyFile(t, "s.jsonl")
	rows := map[string]func(t *testing.T) string{
		"a plain copy (the control)": func(t *testing.T) string {
			path := filepath.Join(projectsFolder(t), "s.jsonl")
			data, _ := os.ReadFile(key)
			writeFile(t, path, string(data))
			return path
		},
		"the file a link": func(t *testing.T) string {
			path := filepath.Join(projectsFolder(t), "s.jsonl")
			linkOrSkip(t, key, path)
			return path
		},
		"its project folder a link": func(t *testing.T) string {
			folder := filepath.Join(claudeProjects(t), "-x5link-"+randomHex(4))
			linkOrSkip(t, filepath.Dir(key), folder)
			t.Cleanup(func() { os.Remove(folder) })
			return filepath.Join(folder, "s.jsonl")
		},
		// Inside the Claude folder, which os.Root alone would let through: a project folder linked back up to it, or a
		// transcript linked to a file of it (Claude Code's history.jsonl holds every session's prompts).
		"its project folder a link up its own Claude folder": func(t *testing.T) string {
			history := filepath.Join(homeDir, ".claude", "x5-history.jsonl")
			data, _ := os.ReadFile(key)
			writeFile(t, history, string(data))
			t.Cleanup(func() { os.Remove(history) })
			folder := filepath.Join(claudeProjects(t), "-x5up-"+randomHex(4))
			linkOrSkip(t, "..", folder)
			t.Cleanup(func() { os.Remove(folder) })
			return filepath.Join(folder, "x5-history.jsonl")
		},
		"the file a link to a file of its Claude folder": func(t *testing.T) string {
			history := filepath.Join(homeDir, ".claude", "x5-history2.jsonl")
			data, _ := os.ReadFile(key)
			writeFile(t, history, string(data))
			t.Cleanup(func() { os.Remove(history) })
			path := filepath.Join(projectsFolder(t), "s.jsonl")
			linkOrSkip(t, filepath.Join("..", "..", "x5-history2.jsonl"), path)
			return path
		},
		"a hard link": func(t *testing.T) string {
			path := filepath.Join(projectsFolder(t), "s.jsonl")
			if err := os.Link(key, path); err != nil {
				t.Skipf("no hard link here: %v", err)
			}
			return path
		},
	}
	for row, build := range rows {
		t.Run(row, func(t *testing.T) {
			control := strings.HasPrefix(row, "a plain copy")
			d := newTestDeck(t)
			path := build(t)
			d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/w", "transcript_path": path},
				"claude", "tty:/dev/ttys003", "", 10)
			d.mu.Lock()
			transcript, model := d.sessions.Get("s1").Transcript, d.sessions.Get("s1").Model
			d.mu.Unlock()
			if transcript != path {
				t.Fatalf("a transcript in its place wasn't taken: %q", transcript)
			}
			_, tail := readTranscriptTail(path)
			_, page := readConversation(path, nil, conversationPageMost)
			followedSome := !reflect.DeepEqual(followed(path), tally{})
			status, _ := d.conversation("s1", nil, nil)
			// RT126SAX5's nit: the size the window already has is the key's. Asked again with it, the route says
			// "nothing new" (204) only for the file the reader takes, and the talk card gives only that one's size.
			keySize := int64(len(must(os.ReadFile(key))))
			unchanged, _ := d.conversation("s1", nil, &keySize)
			card := talkAsk(t, d, "/talk?session=s1").Body.String()
			var sized struct{ Size int64 }
			if err := json.Unmarshal([]byte(card), &sized); err != nil {
				t.Fatalf("the talk card: %v %s", err, card)
			}
			seen := map[string]bool{"the model": model == standInKey, "the summary's tail": tail, "the conversation": page,
				"the session card": followedSome, "the conversation route": status == http.StatusOK,
				"the talk card": strings.Contains(card, standInKey), "the conversation route's size": unchanged == http.StatusNoContent,
				"the talk card's size": sized.Size == keySize}
			for what, read := range seen {
				if read != control {
					t.Errorf("%s: read %v", what, read)
				}
			}
		})
	}
}

// P4's row for STATUS.md (frontdesk.go's line, the recap, the watchers): linked to a stand-in key, its .team folder a
// link, or a hard link. The plain copy is the control.
func TestAStatusLinkedToAStandInKeyIsNeverRead(t *testing.T) {
	keyFolder := t.TempDir()
	key := filepath.Join(keyFolder, "STATUS.md")
	writeFile(t, key, "working\n"+standInKey+"\n")
	for row, c := range map[string]struct {
		make    func(t *testing.T, place string)
		name    string
		control bool
	}{
		"a plain copy (the control)": {func(t *testing.T, place string) {
			writeFile(t, filepath.Join(place, "STATUS.md"), "working\n"+standInKey+"\n")
		}, "STATUS.md", true},
		"STATUS.md a link": {func(t *testing.T, place string) { linkOrSkip(t, key, filepath.Join(place, "STATUS.md")) }, "STATUS.md", false},
		".team a link":     {func(t *testing.T, place string) { linkOrSkip(t, keyFolder, filepath.Join(place, ".team")) }, filepath.Join(".team", "STATUS.md"), false},
		"a hard link": {func(t *testing.T, place string) {
			if err := os.Link(key, filepath.Join(place, "STATUS.md")); err != nil {
				t.Skipf("no hard link here: %v", err)
			}
		}, "STATUS.md", false},
	} {
		t.Run(row, func(t *testing.T) {
			place := physical(t.TempDir()) // the watchers look only at a path with no link in it, /var's included
			c.make(t, place)
			line := statusLine(place)
			recap := strings.Join(lastWords(5, place, "STATUS.md", filepath.Join(".team", "STATUS.md")), "\n")
			watched, _ := openPlainFile(place, filepath.Join(place, c.name))
			if watched != nil {
				watched.Close()
			}
			seen := map[string]bool{"the desk's line": line == standInKey, "the recap": strings.Contains(recap, standInKey),
				"the watchers": watched != nil}
			for what, read := range seen {
				if read != c.control {
					t.Errorf("%s: read %v (line %q, recap %q)", what, read, line, recap)
				}
			}
		})
	}
}

// A transcript's subagent is read through its session's place: one whose id climbs out is never read.
func TestASubagentsTranscriptIsReadOnlyBelowItsSessionsPlace(t *testing.T) {
	folder := projectsFolder(t)
	main := filepath.Join(folder, "s1.jsonl")
	writeFile(t, main, reply("claude-opus-5-5")+"\n")
	writeFile(t, filepath.Join(folder, "s1", "subagents", "agent-a1.jsonl"), reply("claude-haiku-5-5")+"\n")
	if got := firstModel(transcriptPlace(main), filepath.Join(folder, "s1", "subagents", "agent-a1.jsonl")); got != "claude-haiku-5-5" {
		t.Fatalf("its own subagent: %q", got)
	}
	// The id a hook names: "../../../../../x5-out/agent" climbs from the subagents folder to the home folder.
	out := filepath.Join(homeDir, "x5-out", "agent.jsonl")
	writeFile(t, out, reply(standInKey)+"\n")
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(out)) })
	climb := filepath.Join(folder, "s1", "subagents", "..", "..", "..", "..", "..", "x5-out", "agent.jsonl")
	if climb != out {
		t.Fatalf("the climb is %s", climb)
	}
	if got := firstModel(transcriptPlace(main), climb); got != "" {
		t.Errorf("a subagent id that climbs out read %q", got)
	}
	// RT126SAX5's nit: an id that walks sideways inside the place, to another project's transcript, is no subagent's
	// name, so it is never read, though the reader alone would take that file.
	other := filepath.Join(filepath.Dir(folder), "-x5other-"+randomHex(4), "s2.jsonl")
	writeFile(t, other, reply(standInKey)+"\n")
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(other)) })
	sideways := "x/../../../../" + filepath.Base(filepath.Dir(other)) + "/s2"
	if walked := filepath.Join(folder, "s1", "subagents", "agent-"+sideways+".jsonl"); walked != other {
		t.Fatalf("the walk is %s", walked)
	}
	if got := firstModel(transcriptPlace(main), other); got != standInKey {
		t.Fatalf("the reader alone doesn't take the other transcript: %q", got)
	}
	s := NewSessions()
	s.Apply(event("UserPromptSubmit", "transcript_path", main), Claude, tab, "", 100)
	for id, want := range map[string]string{"a1": "claude-haiku-5-5", sideways: "", `x\..\s2`: "", "x..y": ""} {
		s.Apply(event("SubagentStart", "agent_id", id, "agent_type", "Explore"), Claude, tab, "", 200)
		s.Apply(event("PreToolUse", "agent_id", id, "tool_name", "Read"), Claude, tab, "", 300)
		i := slices.IndexFunc(s.Get("s1").Subagents, func(a Subagent) bool { return a.ID == id })
		if i < 0 || s.Get("s1").Subagents[i].Model != want {
			t.Errorf("subagent %q: %+v, want model %q", id, s.Get("s1").Subagents, want)
		}
	}
}

// RT126SAX5 N2: a hook's transcript_path is never resolved through a desk's links. A folder of a desk's linked to the
// owner's ~/.claude, or to a confirmed account's folder, is neither, though the file it leads to is one of theirs: that
// path isn't taken. The same file named in its own folder is (the control).
func TestATranscriptNamedThroughADesksLinkToAClaudeFolderIsNotTaken(t *testing.T) {
	for _, account := range []string{".claude", ".claude-x5work"} {
		t.Run(account, func(t *testing.T) {
			d := newTestDeck(t)
			folder := filepath.Join(homeDir, account)
			real := filepath.Join(folder, "projects", "-x5n2", "s.jsonl")
			writeFile(t, real, reply("claude-opus-5-5")+"\n")
			if account != ".claude" {
				d.mu.Lock()
				d.config.ClaudeProfiles = []claudeProfile{{Name: "x5work", Dir: folder}}
				d.mu.Unlock()
			}
			link := filepath.Join(t.TempDir(), "desk-claude")
			linkOrSkip(t, folder, link)
			through := filepath.Join(link, "projects", "-x5n2", "s.jsonl")
			if data, err := os.ReadFile(through); err != nil || !strings.Contains(string(data), "claude-opus-5-5") {
				t.Fatalf("the link doesn't lead to the transcript: %q %v", data, err)
			}
			for i, c := range []struct {
				path string
				kept bool
			}{{real, true}, {through, false}} {
				id := "s-n2-" + string(rune('a'+i))
				d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": id, "cwd": "/w", "transcript_path": c.path},
					"claude", "tty:/dev/ttys08"+string(rune('0'+i)), "", 10)
				d.mu.Lock()
				got, model := d.sessions.Get(id).Transcript, d.sessions.Get(id).Model
				d.mu.Unlock()
				if (got == c.path) != c.kept || !c.kept && (got != "" || model != "") {
					t.Errorf("%s: kept as %q (model %q), want kept %v", c.path, got, model, c.kept)
				}
			}
		})
	}
}

// FXSAX5R1 B2: every Claude config folder an account runs on (CLAUDE_CONFIG_DIR), not ~/.claude alone. A transcript is
// read by its layout, <folder>/projects/<project>/<name>.jsonl, through that folder. A hook's is taken only when the
// folder is ~/.claude or a confirmed account's (claudeAccountList), as written or as the account's own folder resolves
// (an owner's dotfiles link). Under a folder of Claude's layout that no confirmed account names, it is refused, as
// intended: every hire Smolcorp starts runs on ~/.claude or a confirmed account (hirestart.go, hirelauncher.go), so it
// is no account the deck follows. A path outside every config folder's layout has no place, so nothing reads it.
func TestATranscriptIsReadFromAnyClaudeFolderAndTakenFromAConfirmedOnesOnly(t *testing.T) {
	const words = "x5-b2-the-reply"
	rows := []struct {
		row         string
		build       func(t *testing.T) (accounts []claudeProfile, path string)
		taken, read bool
	}{
		{"~/.claude (the control)", func(t *testing.T) ([]claudeProfile, string) {
			return nil, filepath.Join(claudeProjects(t), "-b2", "s1.jsonl")
		}, true, true},
		{"a confirmed account's folder", func(t *testing.T) ([]claudeProfile, string) {
			work := filepath.Join(homeDir, ".claude-work")
			return []claudeProfile{{Name: "work", Dir: work}}, filepath.Join(work, "projects", "-b2", "s1.jsonl")
		}, true, true},
		{"a confirmed account's folder, a link, named as written", func(t *testing.T) ([]claudeProfile, string) {
			linked := dotfilesAccount(t)
			return []claudeProfile{{Name: "linked", Dir: linked}}, filepath.Join(linked, "projects", "-b2", "s1.jsonl")
		}, true, true},
		{"a confirmed account's folder, a link, named as it resolves", func(t *testing.T) ([]claudeProfile, string) {
			linked := dotfilesAccount(t)
			return []claudeProfile{{Name: "linked", Dir: linked}}, filepath.Join(physical(linked), "projects", "-b2", "s1.jsonl")
		}, true, true},
		{"an unconfirmed folder of Claude's layout", func(t *testing.T) ([]claudeProfile, string) {
			work := filepath.Join(homeDir, ".claude-work")
			return []claudeProfile{{Name: "work", Dir: work}}, filepath.Join(homeDir, ".claude-other", "projects", "-b2", "s1.jsonl")
		}, false, true},
		{"a saved folder outside the home folder (no account a hire takes)", func(t *testing.T) ([]claudeProfile, string) {
			out := filepath.Join(t.TempDir(), ".claude-out")
			return []claudeProfile{{Name: "out", Dir: out}}, filepath.Join(out, "projects", "-b2", "s1.jsonl")
		}, false, true},
		{"outside every config folder", func(t *testing.T) ([]claudeProfile, string) {
			return nil, filepath.Join(t.TempDir(), "s1.jsonl")
		}, false, false},
	}
	for _, c := range rows {
		t.Run(c.row, func(t *testing.T) {
			d := newTestDeck(t)
			accounts, path := c.build(t)
			writeFile(t, path, `{"type":"user","message":{"role":"user","content":"hi"}}`+"\n"+
				`{"type":"assistant","message":{"role":"assistant","model":"claude-opus-5-5","content":[{"type":"text","text":"`+words+`"}]}}`+"\n")
			d.mu.Lock()
			d.config.ClaudeProfiles = accounts
			d.mu.Unlock()
			d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/w", "transcript_path": path},
				"claude", "tty:/dev/ttys003", "", 10)
			d.mu.Lock()
			transcript, model := d.sessions.Get("s1").Transcript, d.sessions.Get("s1").Model
			d.mu.Unlock()
			if taken := transcript == path && model == "claude-opus-5-5"; taken != c.taken || !c.taken && (transcript != "" || model != "") {
				t.Errorf("taken as %q (model %q), want taken %v", transcript, model, c.taken)
			}
			page, ok := readConversation(path, nil, conversationPageMost)
			if read := ok && strings.Contains(string(must(json.Marshal(page))), words); read != c.read {
				t.Errorf("the conversation read %v: %+v", read, page)
			}
			if read := !reflect.DeepEqual(followed(path), tally{}); read != c.read {
				t.Errorf("the session card read %v", read)
			}
			status, served := d.conversation("s1", nil, nil)
			if served := status == http.StatusOK && strings.Contains(string(must(json.Marshal(served))), words); served != c.taken {
				t.Errorf("the conversation route served it: %v (%d)", served, status)
			}
		})
	}
}

// dotfilesAccount is a Claude account's folder in the test home that is a link, as an owner's dotfiles make one:
// ~/.claude-linked, leading to ~/dotfiles/claude-linked.
func dotfilesAccount(t *testing.T) string {
	t.Helper()
	real := filepath.Join(homeDir, "dotfiles", "claude-linked")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(homeDir, ".claude-linked")
	linkOrSkip(t, real, linked)
	return linked
}
