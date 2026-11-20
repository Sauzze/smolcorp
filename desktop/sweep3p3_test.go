//go:build !windows

package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// FXSWEEP3P3: the re-sweep's part-3 follow-ups, one row each (F-1 and F-4 are in their own files' rows:
// t3deskdeck_test.go and desksignin_test.go). Throwaway repos, stand-in folders and fake keys only.

// F-3: a sandboxed plain hire's relay sends with its launcher's own hook key, given to the app on the launcher's
// connection: its events land on its own tab alone. The owner's session, named by that relay, is neither moved onto
// the hire's tab nor ended, and the owner's key reaches no tab of the launcher's. Gone with its connection, the key
// opens nothing.
func TestAPlainHiresRelayKeyLandsOnItsOwnTabAlone(t *testing.T) {
	d := newTestDeck(t)
	owner := d.access.HookKey
	if got := hookAs(t, d, owner, "tty:/dev/ttys009", "SessionStart", "s-owner", "/work/mine"); got != http.StatusNoContent {
		t.Fatalf("the owner's session: %d", got)
	}
	server := newLocalServer(t, d)
	const relayKey = "fake-relay-key-p1"
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/launcher?id=P1&agent=claude&folder=/work/p1", nil)
	request.Header.Set(hookKeyHeader, owner)
	request.Header.Set("X-AgentDeck-Relay-Key", relayKey) // launcherRelayKeyHeader, as the launcher sends it
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() }) // before the server's own cleanup, which waits for the stream to end
	var first launcherCommand
	if err := json.NewDecoder(response.Body).Decode(&first); err != nil || first.Secret == "" {
		t.Fatalf("the launcher's first line %+v, %v", first, err)
	}
	tabOf := func(id string) string {
		d.mu.Lock()
		defer d.mu.Unlock()
		if s := d.sessions.Get(id); s != nil {
			return s.Tab
		}
		return ""
	}
	if got := hookAs(t, d, relayKey, "launcher:P1", "SessionStart", "s-p1", "/work/p1"); got != http.StatusNoContent || tabOf("s-p1") != "launcher:P1" {
		t.Fatalf("P1's relay, its own session on its own tab: %d, on %q", got, tabOf("s-p1"))
	}
	for _, name := range []string{"UserPromptSubmit", "SessionEnd"} {
		if got := hookAs(t, d, relayKey, "launcher:P1", name, "s-owner", "/work/mine"); got != http.StatusForbidden || tabOf("s-owner") != "tty:/dev/ttys009" {
			t.Errorf("P1's relay, %s for the owner's session: %d, now on %q", name, got, tabOf("s-owner"))
		}
	}
	if got := hookAs(t, d, relayKey, "tty:/dev/ttys009", "UserPromptSubmit", "s-owner", "/work/mine"); got != http.StatusForbidden {
		t.Errorf("P1's key on the owner's tab: %d", got)
	}
	if got := hookAs(t, d, owner, "launcher:P1", "UserPromptSubmit", "s-p1", "/work/p1"); got != http.StatusForbidden {
		t.Errorf("the owner's key on P1's tab: %d", got)
	}
	response.Body.Close()
	for end := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		d.mu.Lock()
		_, still := d.launchers["P1"]
		d.mu.Unlock()
		if !still {
			break
		}
		if time.Now().After(end) {
			t.Fatal("P1's launcher is still connected")
		}
	}
	if got := hookAs(t, d, relayKey, "launcher:P1", "UserPromptSubmit", "s-p1", "/work/p1"); got != http.StatusUnauthorized {
		t.Errorf("P1's key once it's gone: %d", got)
	}
}

// F-2: the General Manager's file watch, held or not, never reads in Smolcorp's support folder, where other desks'
// Claude folders and histories are: refused when set, and a GM's watch kept from before reads nothing there. The
// owner's watch keeps the folder.
func TestTheGMsFileWatchNeverReadsInTheSupportFolder(t *testing.T) {
	b := watchDeck(t)
	history := filepath.Join(supportDir, "hires", "other01", "claude", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(history), 0o700); err != nil {
		t.Fatal(err)
	}
	putFile(t, history, "{}\n")
	body, _ := json.Marshal(map[string]any{"kind": "file", "target": history, "pattern": ".", "repeat": true})
	if w := b.call("POST", "/setup/gm/watch", string(body), true); w.Code != http.StatusForbidden ||
		w.Body.String() != "The General Manager can't watch files in Smolcorp's own folder." {
		t.Errorf("the GM's watch: %d %q", w.Code, w.Body)
	}
	// Spelt through ~ and a link to the folder, the same.
	link := filepath.Join(b.root, "to-support")
	if err := os.Symlink(supportDir, link); err != nil {
		t.Fatal(err)
	}
	via, _ := json.Marshal(map[string]any{"kind": "file", "target": filepath.Join(link, "hires", "other01", "claude", "history.jsonl"), "pattern": "."})
	if w := b.call("POST", "/setup/gm/watch", string(via), true); w.Code != http.StatusForbidden {
		t.Errorf("the GM's watch through a link: %d %q", w.Code, w.Body)
	}
	// The owner's two watches on it; one is made the GM's, as a watch an earlier version kept would be.
	for range 2 {
		if w := b.call("POST", "/setup/gm/watch", string(body), false); w.Code != http.StatusOK {
			t.Fatalf("the owner's watch: %d %q", w.Code, w.Body)
		}
	}
	b.d.watchers.mu.Lock()
	b.d.watchers.all[1].By = "gm"
	b.d.watchers.mu.Unlock()
	appendTo(t, history, "{\"display\":\"a line of another desk's\"}\n")
	b.tick()
	if fired := b.fired(); len(fired) != 1 {
		t.Errorf("fired %v: want the owner's watch alone", fired)
	}
}

// F-5: the fire's object check opens a pack's index, and a stored copy, only as a regular file: a named pipe in the
// object folder, held open by a desk, refuses at once in plain words, rather than holding up the fire.
func TestTheFiresLookRefusesANamedPipeInTheObjectStore(t *testing.T) {
	objects := filepath.Join(t.TempDir(), "objects")
	pack := filepath.Join(objects, "pack")
	if err := os.MkdirAll(pack, 0o700); err != nil {
		t.Fatal(err)
	}
	pipe := func(name string) string {
		t.Helper()
		path := filepath.Join(pack, name)
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
		// Held open as a desk would hold it, writing nothing; let go when the test ends, so nothing stays stuck.
		holder, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { holder.Close() })
		return path
	}
	soon := func(what string, look func() error) error {
		t.Helper()
		done := make(chan error, 1)
		go func() { done <- look() }()
		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: still waiting after 5 s, held up by a named pipe", what)
		}
		return nil
	}
	hex := strings.Repeat("ab", 20)
	idx := pipe("pack-" + hex + ".idx")
	if err := os.WriteFile(filepath.Join(pack, "pack-"+hex+".pack"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := soon("the pack index", func() error { _, err := storeSecondCopies(context.Background(), []string{objects}, 20); return err })
	if err == nil || !strings.Contains(err.Error(), idx+" isn't a plain file") {
		t.Errorf("the pack index: %v", err)
	}
	other := pipe("pack-" + strings.Repeat("cd", 20) + ".pack")
	name := make([]byte, 20)
	for _, c := range []struct {
		what string
		copy storeCopy
	}{{"a loose copy", storeCopy{file: other}}, {"a packed copy", storeCopy{file: other, pack: &storePack{file: other}, at: 12}}} {
		err := soon(c.what, func() error { _, err := c.copy.holds(context.Background(), name); return err })
		if err == nil || !strings.Contains(err.Error(), other+" isn't a plain file") {
			t.Errorf("%s: %v", c.what, err)
		}
	}
}

// F-6: Take over refuses a worktree whose registration holds a rebase, merge, cherry-pick or revert under way, which
// the hire writes and the owner's git (or T3 Code's agent) would carry on, a rebase's exec line included.
func TestTakeOverRefusesARebaseOrMergeUnderWayInTheRegistration(t *testing.T) {
	for _, row := range []struct {
		name   string
		folder bool
	}{{"rebase-merge", true}, {"rebase-apply", true}, {"sequencer", true}, {"MERGE_HEAD", false}, {"CHERRY_PICK_HEAD", false}, {"REVERT_HEAD", false}} {
		t.Run(row.name, func(t *testing.T) {
			r := newDeskRunRig(t)
			registration := filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef")
			// The control: git, in the hire's worktree, looks for it in the registration.
			if got := gitRepoCmd(t, r.tree, "rev-parse", "--path-format=absolute", "--git-path", row.name); realPath(got) != realPath(filepath.Join(registration, row.name)) {
				t.Fatalf("git looks for %s at %q, so this row proves nothing", row.name, got)
			}
			if row.folder {
				writeRepoFile(t, filepath.Join(registration, row.name), "git-rebase-todo", "exec /stand-in\n")
				writeRepoFile(t, filepath.Join(registration, row.name), "interactive", "")
			} else {
				writeRepoFile(t, registration, row.name, gitRepoCmd(t, r.repo, "rev-parse", "main")+"\n")
			}
			checkSays(t, r.hire, "its git folder has "+row.name+",")
		})
	}
}

// F-7: a desk's git-state read stops at rev-parse when the top git gives is outside the desk's place: a bare repo it
// made in a place that isn't a repo, with core.worktree naming a folder of its choosing; a repo of its own in its
// worktree with the same; its worktree's own core.worktree. No status or diff runs over that folder, for the desk's
// window or the GM's ?hire= read. The control, each time: with no place, git works on that folder.
func TestADesksBareRepoOrWorkTreeOutsideItsPlaceIsntRead(t *testing.T) {
	d, keys, sockets := keyedOffice(t)
	managerOn(d)
	outside := physical(t.TempDir())
	writeRepoFile(t, outside, "a-name-the-desk-cant-read.txt", "x\n")
	worktree := filepath.Join(filepath.Dir(sockets["aaa111"]), "wt-aaa111")
	gitRepoCmd(t, filepath.Dir(worktree), "init", "-q", "-b", "main", worktree)
	nested := filepath.Join(worktree, "x")
	gitRepoCmd(t, worktree, "init", "-q", "-b", "main", nested)
	gitRepoCmd(t, nested, "config", "core.worktree", outside)
	place := filepath.Join(filepath.Dir(sockets["bbb222"]), "wt-bbb222") // a place that isn't a repo
	bare := filepath.Join(place, "made.git")
	gitRepoCmd(t, filepath.Dir(place), "init", "-q", "--bare", bare)
	gitRepoCmd(t, bare, "config", "core.bare", "false")
	gitRepoCmd(t, bare, "config", "core.worktree", outside)
	var mu sync.Mutex
	var ran []string
	saved := gits
	gits = newGitStates()
	gits.read = func(ctx context.Context, dir string, call gitCall, args ...string) ([]byte, int, error) {
		mu.Lock()
		ran = append(ran, args[0])
		mu.Unlock()
		return readGitOut(ctx, dir, call, args...)
	}
	t.Cleanup(func() { gits = saved })
	read := func() []string {
		mu.Lock()
		defer mu.Unlock()
		was := ran
		ran = nil
		return was
	}
	onlyRevParse := func(got []string) bool {
		return !slices.ContainsFunc(got, func(a string) bool { return a != "rev-parse" })
	}
	for _, c := range []struct{ name, desk, folder string }{{"a bare repo", "bbb222", bare}, {"a repo of its own", "aaa111", nested}} {
		if state, _ := readGitState(c.folder, gits.read, time.Now()); physical(state.Top) != outside || !slices.Contains(read(), "status") {
			t.Fatalf("%s, the control: git took %q as the top, so this row proves nothing", c.name, state.Top)
		}
		if code := hookAs(t, d, keys[c.desk], tmuxTabOn(sockets[c.desk]), "UserPromptSubmit", "s-"+c.desk, c.folder); code != http.StatusNoContent {
			t.Fatalf("%s: the hook: %d", c.name, code)
		}
		code, answer := d.deskGit("s-" + c.desk)
		if state := gitAnswer(t, answer); code != http.StatusOK || !state.Skipped || len(state.Files) != 0 {
			t.Errorf("%s, the desk's window: %d %+v", c.name, code, answer)
		}
		if got := read(); !onlyRevParse(got) {
			t.Errorf("%s: the desk's window ran git %q over a folder outside its place", c.name, got)
		}
	}
	// Its worktree's own core.worktree, read by the GM by the hire's id.
	gitRepoCmd(t, worktree, "config", "core.worktree", outside)
	if state, _ := readGitState(worktree, gits.read, time.Now()); physical(state.Top) != outside || !slices.Contains(read(), "status") {
		t.Fatalf("the worktree's own, the control: git took %q as the top, so this row proves nothing", state.Top)
	}
	got := serve(t, d, call{method: "GET", target: "/setup/gm/git?hire=aaa111", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	var state GitState
	if json.Unmarshal(got.Body.Bytes(), &state) != nil || got.Code != http.StatusOK || !state.Skipped || len(state.Files) != 0 {
		t.Errorf("the GM's read: %d %s", got.Code, got.Body)
	}
	if got := read(); !onlyRevParse(got) {
		t.Errorf("the GM's read ran git %q over a folder outside the hire's place", got)
	}
}

// F-8: a worktree made through the core's folder refuses a repo whose git folder names, by its commondir, a shared
// git folder that isn't its own: no branch, registration or upstream is written into that other repo. The control:
// git takes the other repo's .git as the folder's.
func TestAWorktreeIsNeverMadeThroughACommondirThatIsntTheReposOwn(t *testing.T) {
	r := newOwnerRig(t) // r.repo stands for a repo nobody hired into
	x, fake := filepath.Join(r.base, "x"), filepath.Join(r.base, "x", "fake")
	writeRepoFile(t, fake, "HEAD", "ref: refs/heads/main\n")
	writeRepoFile(t, fake, "commondir", filepath.Join(r.repo, ".git")+"\n")
	writeRepoFile(t, x, ".git", "gitdir: "+fake+"\n")
	if got := gitRepoCmd(t, x, "rev-parse", "--path-format=absolute", "--git-common-dir"); physical(got) != physical(filepath.Join(r.repo, ".git")) {
		t.Fatalf("the control: git takes %q as x's shared git folder, so this row proves nothing", got)
	}
	status, out, err := ownerGitRun(context.Background(), x, []string{"worktree", "add", "-q", "-b", "hire/ada-abcdef", filepath.Join(r.base, "hires", "x", "ada-abcdef")})
	if err == nil || !strings.Contains(err.Error(), "commondir") {
		t.Errorf("the core's worktree add: %d %q %v", status, out, err)
	}
	keepRegistry(t)
	o := hirePlaceOptions{Worktrees: filepath.Join(r.base, "hires"), PortFree: func(int) bool { return true }}
	if place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: x, Worktree: true}, o); err == nil {
		t.Errorf("hired into x: %+v", place)
	}
	if got := branches(t, r.repo); got != "" {
		t.Errorf("a branch was written into the other repo: %q", got)
	}
	if entries, _ := os.ReadDir(filepath.Join(r.repo, ".git", "worktrees")); len(entries) != 0 {
		t.Errorf("a registration was written into the other repo: %v", entries)
	}
	if config, _ := os.ReadFile(filepath.Join(r.repo, ".git", "config")); strings.Contains(string(config), "hire/") {
		t.Errorf("the other repo's config has a hire's branch: %s", config)
	}
}

// F-9: Setup's folder check refuses ~/Library in every spelling the disk takes for it: a Mac's disk folds case, so
// ~/library is the same folder. A project folder of that name further down is still a project.
func TestSetupsFolderCheckRefusesLibraryInAnySpelling(t *testing.T) {
	home := physical(t.TempDir())
	if err := os.MkdirAll(filepath.Join(home, "Library", "Application Support", "SomeApp"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, spelt := range []string{"Library", "library", "LIBRARY", "lIbRaRy"} {
		given := filepath.Join(home, spelt, "Application Support", "SomeApp")
		if _, err := os.Stat(given); err != nil {
			if spelt == "Library" {
				t.Fatal(err)
			}
			t.Logf("%s: this disk tells names apart by case, so it's another folder here", spelt)
			continue
		}
		if _, status, why := checkedSetupFolder(given, home); status != http.StatusBadRequest || !strings.Contains(why, "apps' own files") {
			t.Errorf("%s: %d %q", spelt, status, why)
		}
	}
	project := filepath.Join(home, "Projects", "library")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if folder, status, why := checkedSetupFolder(project, home); status != 0 || folder != project {
		t.Errorf("a project named library: %q %d %q", folder, status, why)
	}
}
