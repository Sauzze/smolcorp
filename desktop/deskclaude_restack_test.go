//go:build !windows

package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// SA-X8 stacked on SA-X5 (RT126SAX8's restack items, MSYSAX8): a desk with its own Claude folder is read there alone,
// by the key it speaks with; its plans don't outlive it in a folder others share; on Linux its folder holds copies.
// Stand-in accounts and keys only.

// ownClaudeTranscript makes desk id's own Claude folder, as the engine does, with a transcript in it whose model is
// model, and returns the transcript's path.
func ownClaudeTranscript(t *testing.T, id, model string) string {
	t.Helper()
	path := filepath.Join(deskClaudeFolder(supportDir, id), "projects", "-wt-"+id, "s.jsonl")
	writeFile(t, path, reply(model)+"\n")
	return path
}

// hookTranscript posts a Stop hook event from this computer with key, for session id on tab ("" for none), naming
// transcript, and says the transcript and model the session was left with.
func hookTranscript(t *testing.T, d *Deck, key, tab, id, transcript string) (kept, model string) {
	t.Helper()
	body := fmt.Sprintf(`{"hook_event_name":"Stop","session_id":%q,"cwd":"/proj","transcript_path":%q}`, id, transcript)
	r := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	r.Host, r.RemoteAddr = "localhost:7710", thisComputer
	r.Header.Set(hookKeyHeader, key)
	r.Header.Set("X-AgentDeck-Project", "/proj")
	if tab != "" {
		r.Header.Set("X-AgentDeck-Tab", tab)
	}
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("the event for %s wasn't taken: %d %s", id, w.Code, w.Body.String())
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.sessions.Get(id)
	if s == nil {
		t.Fatalf("no session %s", id)
	}
	return s.Transcript, s.Model
}

// RT126SAX8 restack item 5: an X8 desk's transcript is taken for its own key alone. The same path with another desk's
// key, or with the owner's sessions' key, is refused; a transcript under another desk's Claude folder is refused; and
// a desk with its own folder takes no account folder's either. A desk with a key and no folder of its own (a launcher's,
// a plain one) and the owner's sessions take the account folders, as X5 does, and never a desk's folder.
func TestAnX8DesksTranscriptIsTakenForItsOwnKeyAlone(t *testing.T) {
	d, keys, sockets := keyedOffice(t)
	ann, bea, owner := keys["aaa111"], keys["bbb222"], d.access.HookKey
	annTab, beaTab, ownerTab := tmuxTabOn(sockets["aaa111"]), tmuxTabOn(sockets["bbb222"]), "tty:/dev/ttys041"
	annT := ownClaudeTranscript(t, "aaa111", "claude-opus-5-5")
	beaT := ownClaudeTranscript(t, "bbb222", "claude-sonnet-5-5")
	accountT := filepath.Join(projectsFolder(t), "s.jsonl")
	writeFile(t, accountT, reply("claude-haiku-5-5")+"\n")
	// A session on a tab takes the model of the one before it there: a refused transcript shows only if its own model
	// does.
	modelOf := map[string]string{annT: "claude-opus-5-5", beaT: "claude-sonnet-5-5", accountT: "claude-haiku-5-5"}
	refused := func(row, path, kept, model string) {
		t.Helper()
		if kept != "" || model == modelOf[path] {
			t.Errorf("%s: kept as %q (model %q), want refused", row, kept, model)
		}
	}

	rows := []struct {
		row, key, tab, path string
		kept                bool
	}{
		{"Ann's own, with her key", ann, annTab, annT, true},
		{"Bea's own, with her key", bea, beaTab, beaT, true},
		{"Ann's, with Bea's key", bea, beaTab, annT, false},
		{"Ann's, with the owner's sessions' key", owner, ownerTab, annT, false},
		{"Bea's, under another desk's folder, with Ann's key", ann, annTab, beaT, false},
		{"an account's, with Ann's key: not her folder", ann, annTab, accountT, false},
		{"an account's, with the owner's key (the control)", owner, ownerTab, accountT, true},
		{"Ann's, through a headless run of Bea's", bea, "headless:s-bea-run", annT, false},
		{"Ann's, through a headless run of hers", ann, "headless:s-ann-run", annT, true},
	}
	for i, c := range rows {
		id := fmt.Sprintf("s-x8-%d", i)
		if strings.HasPrefix(c.tab, "headless:") {
			id = strings.TrimPrefix(c.tab, "headless:")
		}
		kept, model := hookTranscript(t, d, c.key, c.tab, id, c.path)
		if !c.kept {
			refused(c.row, c.path, kept, model)
		} else if kept != c.path || model != modelOf[c.path] {
			t.Errorf("%s: kept as %q (model %q), want taken", c.row, kept, model)
		}
	}

	// Bea with no folder of her own (a desk the engine didn't start on Claude Code, or from before X8): the account
	// folders, as X5 takes them, and never Ann's folder.
	if err := os.RemoveAll(deskClaudeFolder(supportDir, "bbb222")); err != nil {
		t.Fatal(err)
	}
	if kept, _ := hookTranscript(t, d, bea, beaTab, "s-x8-bea-account", accountT); kept != accountT {
		t.Errorf("a keyed desk with no folder of its own lost an account's transcript: %q", kept)
	}
	kept, model := hookTranscript(t, d, bea, beaTab, "s-x8-bea-ann", annT)
	refused("a keyed desk with no folder of its own, Ann's", annT, kept, model)
	// An account confirmed at a desk's folder is still that desk's: the owner's key never takes it.
	d.mu.Lock()
	d.config.ClaudeProfiles = []claudeProfile{{Name: "odd", Dir: deskClaudeFolder(supportDir, "aaa111")}}
	d.mu.Unlock()
	kept, model = hookTranscript(t, d, owner, "tty:/dev/ttys042", "s-x8-owner-odd", annT)
	refused("an account named at Ann's folder, with the owner's key", annT, kept, model)
	// A place the support folder is: refused for a desk too, as X5 refuses it.
	d.mu.Lock()
	if d.transcriptKnown("aaa111", filepath.Join(supportDir, acpTranscriptFolder, "a.jsonl")) {
		t.Error("an ACP transcript in the support folder was taken for Ann")
	}
	d.mu.Unlock()
}

// RT126SAX8 restack item 5, restore: a saved session is mapped to its desk by its tab, and keeps its transcript only
// in that desk's own folder; one naming another desk's, or saved on the owner's tab, is let go.
func TestRestoreKeepsADesksOwnTranscriptAndDropsAnothers(t *testing.T) {
	d, _, sockets := keyedOffice(t)
	annTab := tmuxTabOn(sockets["aaa111"])
	annT := ownClaudeTranscript(t, "aaa111", "claude-opus-5-5")
	beaT := ownClaudeTranscript(t, "bbb222", "claude-sonnet-5-5")
	accountT := filepath.Join(projectsFolder(t), "s.jsonl")
	writeFile(t, accountT, reply("claude-haiku-5-5")+"\n")
	d.mu.Lock()
	d.restore([]savedSession{
		{ID: "r-own", Agent: "claude", Tab: annTab, Transcript: annT},
		{ID: "r-other", Agent: "claude", Tab: annTab, Transcript: beaT},
		{ID: "r-account", Agent: "claude", Tab: annTab, Transcript: accountT},
		{ID: "r-owner-desk", Agent: "claude", Tab: "tty:/dev/ttys071", Transcript: annT},
		{ID: "r-owner-account", Agent: "claude", Tab: "tty:/dev/ttys072", Transcript: accountT},
	}, savedDeck{}, true)
	got := map[string]string{}
	for _, id := range []string{"r-own", "r-other", "r-account", "r-owner-desk", "r-owner-account"} {
		if s := d.sessions.Get(id); s != nil {
			got[id] = s.Transcript
		} else {
			got[id] = "(no session)"
		}
	}
	d.mu.Unlock()
	want := map[string]string{"r-own": annT, "r-other": "", "r-account": "", "r-owner-desk": "", "r-owner-account": accountT}
	for id, path := range want {
		if got[id] != path {
			t.Errorf("%s restored with %q, want %q", id, got[id], path)
		}
	}
}

// RT126SAX8 restack item 3: the card reads a desk's own .claude.json and settings.json through the core's reader, so
// a link the desk puts at either is never followed; an account's are read as before (a dotfiles link is the owner's).
func TestTheCardReadsADesksOwnProfileAndSettingsThroughTheReader(t *testing.T) {
	isolate(t)
	place := deskClaudeFolder(supportDir, "aaa111")
	key := filepath.Join(t.TempDir(), "elsewhere.json")
	writeFile(t, key, `{"oauthAccount":{"emailAddress":"stand-in@example.com"},"apiKeyHelper":"echo stand-in"}`)
	now := float64(time.Now().UnixMilli())
	profile, settings := filepath.Join(place, ".claude.json"), filepath.Join(place, "settings.json")

	// Plain files in the desk's folder (the control): read.
	writeFile(t, profile, `{"oauthAccount":{"emailAddress":"desk@example.com"}}`)
	writeFile(t, settings, `{"apiKeyHelper":"echo desk"}`)
	s := &sessionInfos{followers: map[string]*transcriptFollower{}, profiles: map[string]keptProfile{}, billing: map[string]string{}}
	if account, ok := s.account(place, profile, now); !ok || account.name != "desk@example.com" {
		t.Errorf("the desk's own profile: %+v %v", account, ok)
	}
	if !hasKeyHelper(place, []string{settings}) {
		t.Error("the desk's own settings weren't read")
	}

	// Links the desk put there: not followed.
	for _, at := range []string{profile, settings} {
		if err := os.Remove(at); err != nil {
			t.Fatal(err)
		}
		linkOrSkip(t, key, at)
	}
	s = &sessionInfos{followers: map[string]*transcriptFollower{}, profiles: map[string]keptProfile{}, billing: map[string]string{}}
	if account, ok := s.account(place, profile, now); ok || account.name != "" {
		t.Errorf("a linked profile in the desk's folder was read: %+v", account)
	}
	if hasKeyHelper(place, []string{settings}) {
		t.Error("a linked settings file in the desk's folder was read")
	}
	// A desk's huge file: past the cap, not read.
	if err := os.Remove(settings); err != nil {
		t.Fatal(err)
	}
	writeFile(t, settings, `{"apiKeyHelper":"echo desk","pad":"`+strings.Repeat("x", deskClaudeFileMostBytes)+`"}`)
	if hasKeyHelper(place, []string{settings}) {
		t.Error("a settings file past the cap was read")
	}
	// An account's folder (place ""): an owner's dotfiles link is followed, as before.
	account := filepath.Join(homeDir, ".claude-dot")
	if err := os.MkdirAll(account, 0o700); err != nil {
		t.Fatal(err)
	}
	linkOrSkip(t, key, filepath.Join(account, "settings.json"))
	if !hasKeyHelper("", []string{filepath.Join(account, "settings.json")}) {
		t.Error("an account's dotfiles-linked settings weren't read")
	}
}

// RT126SAX8 restack item 4: a desk with its own Claude folder finds its inbox in that folder alone, never in an
// account's, where a session file of the same id isn't its own; any other desk looks in the account folders.
func TestADesksInboxIsInItsOwnClaudeFolderAlone(t *testing.T) {
	isolate(t)
	work := filepath.Join(homeDir, ".claude-work")
	saved := []claudeProfile{{Name: "work", Dir: work}}
	session := func(dir, socket string) {
		writeFile(t, filepath.Join(dir, "sessions", "1.json"),
			`{"sessionId":"s-in","messagingSocketPath":"`+socket+`","version":"2.1.325"}`)
	}
	session(work, "/account.sock")
	h := Hire{ID: "aaa111"}
	if dirs := inboxFolders(h, saved); !slices.Contains(dirs, work) {
		t.Fatalf("a desk without its own folder looks in %q", dirs)
	}
	if socket, _ := claudeInbox("s-in", inboxFolders(h, saved)); socket != "/account.sock" {
		t.Errorf("a desk without its own folder: socket %q", socket)
	}
	own := deskClaudeFolder(supportDir, "aaa111")
	if err := os.MkdirAll(own, 0o700); err != nil {
		t.Fatal(err)
	}
	if dirs := inboxFolders(h, saved); !slices.Equal(dirs, []string{own}) {
		t.Errorf("a desk with its own folder looks in %q, want its folder alone", dirs)
	}
	if socket, _ := claudeInbox("s-in", inboxFolders(h, saved)); socket != "" {
		t.Errorf("a desk with its own folder took an account's session file: %q", socket)
	}
	session(own, "/desk.sock")
	if socket, _ := claudeInbox("s-in", inboxFolders(h, saved)); socket != "/desk.sock" {
		t.Errorf("a desk's own session file: %q", socket)
	}
}

// SIERRA's edge on X8's plans (MSYSAX8): a hire without a worktree and the GM keep their plans in a folder others
// share, each in a folder of its own; at its fire its own go, and another desk's stay.
func TestADeskWithoutAWorktreeLosesItsPlansAtItsFireAndNoOneElses(t *testing.T) {
	d, r, _ := engineDeck(t)
	_ = d
	request := r.request("janitor")
	request.NoWorktree = true
	hire, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	gm := r.request("gm")
	gm.NoWorktree = true
	gmHire, err := runManagedHire(gm, r.o)
	if err != nil {
		t.Fatal(err)
	}
	if hire.Worktree != "" || gmHire.Worktree != "" {
		t.Fatalf("worktrees made: %q %q", hire.Worktree, gmHire.Worktree)
	}
	var settings struct {
		Plans string `json:"plansDirectory"`
	}
	if err := json.Unmarshal([]byte(startedSettings(t, hire.ID)), &settings); err != nil || settings.Plans != deskPlansFor(hire.ID) {
		t.Fatalf("plansDirectory %q (%v), want %q", settings.Plans, err, deskPlansFor(hire.ID))
	}
	plan := func(h Hire) string {
		return filepath.Join(startedIn(h), filepath.FromSlash(deskPlansFor(h.ID)), "calm-river.md")
	}
	for _, h := range []Hire{hire, gmHire} {
		if err := os.WriteFile(plan(h), []byte("# Plan\n"), 0o600); err != nil {
			t.Fatalf("%s's plans folder wasn't made where it starts: %v", h.ID, err)
		}
	}
	windowShows(r, pane(t, "idle-empty"), true)
	exitingPane(r, "0", true)
	if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Dir(plan(hire))); err == nil {
		t.Error("the fired hire's plans are still in the shared folder")
	}
	if _, err := os.Stat(plan(gmHire)); err != nil {
		t.Errorf("the GM's plans went with the hire's fire: %v", err)
	}
	exitingPane(r, "0", true)
	if _, err := fireEngineHire(gmHire, 0, r.o, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Dir(plan(gmHire))); err == nil {
		t.Error("the fired GM's plans are still in its folder")
	}
	if info, err := os.Lstat(filepath.Join(startedIn(hire), ".smolcorp", "plans")); err != nil || !info.IsDir() {
		t.Errorf("the shared plans folder itself went: %v", err)
	}
}

// removeDeskPlans takes the desk's own plans folder only, never through a link at .smolcorp or plans, and links
// inside it go as links.
func TestRemovingADesksPlansNeverFollowsALink(t *testing.T) {
	// A link at .smolcorp, or at .smolcorp/plans: what it leads to keeps its h1 folder.
	for _, at := range []string{".smolcorp", ".smolcorp/plans"} {
		place, away := t.TempDir(), t.TempDir()
		kept := filepath.Join(away, "plans", "h1", "keep.md")
		target := away
		if at == ".smolcorp/plans" {
			kept, target = filepath.Join(away, "h1", "keep.md"), away
			if err := os.Mkdir(filepath.Join(place, ".smolcorp"), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		writeFile(t, kept, "kept")
		linkOrSkip(t, target, filepath.Join(place, filepath.FromSlash(at)))
		_ = removeDeskPlans(place, "h1")
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("a link at %s was followed: %v", at, err)
		}
	}
	away := t.TempDir()
	writeFile(t, filepath.Join(away, "h1", "keep.md"), "kept")
	// Its own, with a link inside: the link goes, what it led to stays; a neighbour's stays.
	place := t.TempDir()
	if err := makeDeskPlans(place, "h1"); err != nil {
		t.Fatal(err)
	}
	if err := makeDeskPlans(place, "h2"); err != nil {
		t.Fatal(err)
	}
	linkOrSkip(t, away, filepath.Join(place, ".smolcorp", "plans", "h1", "out"))
	writeFile(t, filepath.Join(place, ".smolcorp", "plans", "h2", "p.md"), "h2's")
	if err := removeDeskPlans(place, "h1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(place, ".smolcorp", "plans", "h1")); err == nil {
		t.Error("its plans are still there")
	}
	if _, err := os.Stat(filepath.Join(away, "h1", "keep.md")); err != nil {
		t.Errorf("a link inside its plans was followed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(place, ".smolcorp", "plans", "h2", "p.md")); err != nil {
		t.Errorf("a neighbour's plans went: %v", err)
	}
	for _, id := range []string{"", "..", "../h2", "h1/x"} {
		if err := removeDeskPlans(place, id); err == nil {
			t.Errorf("id %q was taken", id)
		}
	}
}

// FXSAL1R1 (MSYSAX8 item 3): on Linux a desk's folder holds copies of what a Mac desk links, since under L1's view a
// link into the account's folder leads nowhere: plain, read-only files of the same bytes; never a link, a sign-in, a
// .claude.json, nor anything below the account in a place the Policy keeps from desks; plugins stays a link. A copy is
// refreshed at each start (the folder is new then). Too much to copy refuses the start, with nothing left.
func TestOnLinuxADesksFolderHoldsCopiesNeverASignIn(t *testing.T) {
	isolate(t)
	saved := deskClaudeCopies
	deskClaudeCopies = true
	t.Cleanup(func() { deskClaudeCopies = saved })
	account, _ := deskAccount(t, filepath.Join(homeDir, "accounts", "work"))
	key := standInKeyFile(t, "key.txt")
	dotfiles := filepath.Join(t.TempDir(), "dotfiles-notes.md")
	writeFile(t, dotfiles, "the owner's notes, kept in dotfiles")
	if err := os.Remove(filepath.Join(account, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	linkOrSkip(t, dotfiles, filepath.Join(account, "CLAUDE.md")) // an owner's dotfiles link: followed, as the Mac's link is
	for path, text := range map[string]string{
		"skills/s/references/x.md": "a skill's own file", "commands/c.md": "a command", "agents/a.md": "an agent",
		"output-styles/o.md": "a style", "skills/s/.credentials.json": `{"fake":"x8-copy"}`, "skills/s/.claude.json": "{}",
		".credentials.json": `{"fake":"x8-top"}`, "projects/-p/s.jsonl": "{}", "history.jsonl": "{}",
	} {
		writeFile(t, filepath.Join(account, filepath.FromSlash(path)), text)
	}
	linkOrSkip(t, key, filepath.Join(account, "skills", "s", "linked.md"))                          // a link inside: never copied
	linkOrSkip(t, filepath.Join(account, "projects"), filepath.Join(account, "agents", "projects")) // into a denied place
	folder := deskClaudeFolder(supportDir, "cccc00000001")
	if err := makeDeskClaude(folder, deskClaude{Account: account, Trusted: "/w"}); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"CLAUDE.md": "the owner's notes, kept in dotfiles", "skills/s/SKILL.md": "a skill",
		"skills/s/references/x.md": "a skill's own file", "commands/c.md": "a command", "agents/a.md": "an agent",
		"output-styles/o.md": "a style"}
	got := map[string]string{}
	err := filepath.WalkDir(folder, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(folder, path)
		rel = filepath.ToSlash(rel)
		info, _ := os.Lstat(path)
		switch {
		case rel == "plugins":
			if info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("plugins isn't a link: %v", info.Mode())
			}
		case info.Mode()&os.ModeSymlink != 0:
			t.Errorf("a link in the desk's folder: %s", rel)
		case info.Mode().IsRegular() && rel != ".claude.json" && rel != "settings.json":
			data, _ := os.ReadFile(path)
			got[rel] = string(data)
			if info.Mode().Perm() != 0o400 {
				t.Errorf("%s is %v, want read-only", rel, info.Mode().Perm())
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(got, want) {
		t.Errorf("the desk's folder holds %q, want %q", got, want)
	}
	if text, _ := os.ReadFile(filepath.Join(folder, ".claude.json")); strings.Contains(string(text), "x8-") || strings.Contains(string(text), standInKey) {
		t.Errorf("a sign-in reached the desk's .claude.json: %s", text)
	}

	// Refreshed at the next start: a skill the owner changed meanwhile.
	writeFile(t, filepath.Join(account, "skills", "s", "SKILL.md"), "a skill, changed")
	if err := removeDeskClaude(supportDir, "cccc00000001"); err != nil {
		t.Fatal(err)
	}
	if err := makeDeskClaude(folder, deskClaude{Account: account, Trusted: "/w"}); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(folder, "skills", "s", "SKILL.md")); string(data) != "a skill, changed" {
		t.Errorf("the copy wasn't refreshed: %q", data)
	}

	// Too much to copy: refused whole, nothing left.
	most := deskClaudeCopyMost
	deskClaudeCopyMost = 3
	t.Cleanup(func() { deskClaudeCopyMost = most })
	other := deskClaudeFolder(supportDir, "cccc00000002")
	if err := makeDeskClaude(other, deskClaude{Account: account, Trusted: "/w"}); !errors.Is(err, errDeskClaudeCopy) {
		t.Errorf("too much to copy: %v", err)
	}
	if _, err := os.Lstat(other); err == nil {
		t.Error("a refused copy left the desk's folder")
	}
}

// CLAUDEPLANT1 on SA-X8 (MSYCLAUDEPLANT1): at a fire, the desk's place is listed while it is still as the desk left
// it, and only then do its plans go from the folder others share. A .claude it made in its plans folder is seen and
// held before that folder goes, what it left beside is held after, and Smolcorp's own .smolcorp/plans is never a
// plant. Stand-ins only: `{"hooks":"standin"}`.
func TestAFireListsADesksPlaceBeforeItsPlansGo(t *testing.T) {
	_, r, _ := engineDeck(t)
	freshClaudeHeld(t)
	request := r.request("janitor")
	request.NoWorktree = true
	hire, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	place := startedIn(hire)
	plans := filepath.Join(place, filepath.FromSlash(deskPlansFor(hire.ID)))
	for _, at := range []string{filepath.Join(plans, ".claude", "settings.json"), filepath.Join(place, "x", ".mcp.json")} {
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(`{"hooks":"standin"}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	windowShows(r, pane(t, "idle-empty"), true)
	exitingPane(r, "0", true)
	if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(plans); err == nil {
		t.Error("the fired hire's plans are still in the shared folder")
	}
	claudeHeld.Lock()
	held := slices.Collect(maps.Values(claudeHeld.plants))
	claudeHeld.Unlock()
	seen := func(want func(string) bool) bool {
		return slices.ContainsFunc(held, func(p claudePlant) bool { return p.Desk == hire.ID && want(p.File) })
	}
	if !seen(func(file string) bool { return inside(plans, file) }) {
		t.Errorf("what it made in its plans folder wasn't listed before the folder went: %+v", held)
	}
	if !seen(func(file string) bool { return file == filepath.Join(place, "x", ".mcp.json") }) {
		t.Errorf("what it left in its place isn't held: %+v", held)
	}
	for _, p := range held {
		if p.File == filepath.Join(place, ".smolcorp") || p.File == filepath.Join(place, ".smolcorp", "plans") || p.File == plans {
			t.Errorf("Smolcorp's own plans folder listed as a plant: %+v", p)
		}
	}
	if !claudeListed(hire.ID) {
		t.Error("fired, its session seen gone: not on record")
	}
	if why := claudeHeldRefusal(place); !strings.Contains(why, ".mcp.json") {
		t.Errorf("your Claude Code in its place: %q, want a refusal naming .mcp.json", why)
	}
}
