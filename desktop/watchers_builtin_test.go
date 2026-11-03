package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The app's own watches (watchers_builtin.go), against Watchers.swift sync and check.
// The registry is watchDeck's: Rev (at work, in root), Janitor (idle, elsewhere), Zed (at work, in root/zed)
// and Bob (fired).

// builtinDeck is watchDeck with the app's own watches on, and a first look that sets them.
func builtinDeck(t *testing.T) *watchBench {
	t.Helper()
	b := watchDeck(t)
	b.d.mu.Lock()
	b.d.config.BuiltinWatches = true
	b.d.mu.Unlock()
	b.tick()
	return b
}

// builtins are the app's own watches as the watches route lists them, with "hire" read from the kept ones.
func (b *watchBench) builtins(kind string) []map[string]any {
	var out []map[string]any
	for _, x := range b.d.watchers.list() {
		if name, _ := x["builtin"].(string); name != "" && (kind == "" || name == kind) {
			out = append(out, x)
		}
	}
	return out
}

// builtinsOf is how many of the app's own watches a hire has.
func (b *watchBench) builtinsOf(hire string) int {
	b.d.watchers.mu.Lock()
	defer b.d.watchers.mu.Unlock()
	n := 0
	for _, x := range b.d.watchers.all {
		if x.Builtin != "" && x.Hire == hire {
			n++
		}
	}
	return n
}

func (b *watchBench) firedWords() []string {
	var out []string
	for _, e := range b.fired() {
		out = append(out, e.Detail)
	}
	return out
}

func (b *watchBench) registry(change func(*registryFile)) {
	b.t.Helper()
	file := readRegistry(registryPath(), false)
	change(&file)
	if err := file.save(registryPath()); err != nil {
		b.t.Fatal(err)
	}
}

// hook is a hook event from a session of the deck.
func (b *watchBench) hook(event map[string]any) {
	b.d.mu.Lock()
	defer b.d.mu.Unlock()
	b.d.sessions.Apply(event, Claude, "tab-"+event["session_id"].(string), "", float64(b.clock.UnixMilli()))
}

func TestEachOpenHireGetsItsBuiltinsAndTheGMAndAFiredHireGetNone(t *testing.T) {
	b := builtinDeck(t)
	count := map[string]int{}
	for _, x := range b.builtins("") {
		count[x["builtin"].(string)]++
		if x["by"] != "app" || x["repeat"] != true || x["active"] != true || x["fired"] != 0 {
			t.Errorf("a built-in as the Mac sets it: %v", x)
		}
	}
	// Three hires at work, and each has both STATUS files.
	want := map[string]int{"needs-review": 6, "status": 6, "denied": 3, "limit": 3, "wait": 3}
	for name, n := range want {
		if count[name] != n {
			t.Errorf("%s: %d, want %d", name, count[name], n)
		}
	}
	if len(count) != len(want) {
		t.Errorf("kinds: %v", count)
	}
	if b.builtinsOf("0a0b0c") != 0 {
		t.Error("a fired hire has watches")
	}
	for _, x := range b.builtins("") {
		switch x["builtin"] {
		case "needs-review":
			if x["kind"] != "file" || x["pattern"] != `^\s*(?:[-*]\s+)?NEEDS-REVIEW:` {
				t.Errorf("needs-review: %v", x)
			}
		case "status":
			if x["kind"] != "file" || x["pattern"] != `^\s*(?:[-*]\s+)?(?:DONE|STOP|BLOCKED|BUDGET REQUEST)\b` {
				t.Errorf("status: %v", x)
			}
		case "denied":
			if x["kind"] != "event" || x["target"] != "permissionDenied" || x["folder"] == "" {
				t.Errorf("denied: %v", x)
			}
		case "limit":
			if x["kind"] != "event" || x["target"] != "notification" || x["pattern"] != "^usage limit$" || x["folder"] == "" {
				t.Errorf("limit: %v", x)
			}
		case "wait":
			if x["kind"] != "idle" || x["minutes"] != 2.0 && x["minutes"] != 2 {
				t.Errorf("wait: %v", x)
			}
		}
	}
	var files []string
	for _, x := range b.builtins("status") {
		if strings.HasPrefix(x["target"].(string), b.root+string(filepath.Separator)) && !strings.Contains(x["target"].(string), "zed") {
			files = append(files, strings.TrimPrefix(x["target"].(string), b.root+string(filepath.Separator)))
		}
	}
	if len(files) != 2 || files[0] != "STATUS.md" || files[1] != filepath.Join(".team", "STATUS.md") {
		t.Errorf("Rev's status files: %v", files)
	}
	// A look again adds nothing.
	b.tick()
	if len(b.builtins("")) != 21 {
		t.Errorf("after a second look: %d", len(b.builtins("")))
	}
}

func TestBuiltinsAreListedKeptAndNotCountedInTheFiftyAndCannotBeRemoved(t *testing.T) {
	b := builtinDeck(t)
	// The answer has the fixture's fields, and builtin; not where it has read to, nor the hire.
	allowed := map[string]bool{"id": true, "kind": true, "target": true, "pattern": true, "repeat": true, "by": true, "createdAt": true,
		"fired": true, "active": true, "firedAt": true, "folder": true, "minutes": true, "builtin": true, "due": true}
	for _, x := range b.builtins("") {
		for key := range x {
			if !allowed[key] {
				t.Errorf("the answer has %q: %v", key, x)
			}
		}
	}
	for i := 0; i < 50; i++ {
		b.watch(`{"kind":"time","target":"t","minutes":5}`)
	}
	if w := b.call("POST", "/setup/gm/watch", `{"kind":"time","target":"t","minutes":5}`, true); w.Code != http.StatusConflict {
		t.Errorf("the 51st: %d", w.Code)
	}
	id := b.builtins("denied")[0]["id"].(string)
	w := b.call("DELETE", "/setup/gm/unwatch?id="+id, "", true)
	if w.Code != http.StatusForbidden {
		t.Errorf("removing a built-in: %d %s", w.Code, w.Body)
	}
	// Kept with who it is for.
	data, _ := os.ReadFile(filepath.Join(supportDir, "watchers.json"))
	if !strings.Contains(string(data), `"hire": "a1b2c3"`) || !strings.Contains(string(data), `"builtin": "wait"`) {
		t.Errorf("watchers.json: %s", data)
	}
}

func TestBuiltinsOfAHireGoWhenItIsFiredFailedOrGone(t *testing.T) {
	b := builtinDeck(t)
	b.registry(func(f *registryFile) { f.hires[0].State = "fired" })
	b.tick()
	if b.builtinsOf("a1b2c3") != 0 || b.builtinsOf("d4e5f6") != 7 || b.builtinsOf("9z9z9z") != 7 {
		t.Errorf("a fired hire: %v", b.builtins(""))
	}
	b.registry(func(f *registryFile) { f.hires[1].State = "failed" })
	b.tick()
	if b.builtinsOf("d4e5f6") != 0 || b.builtinsOf("9z9z9z") != 7 {
		t.Errorf("a failed hire: %v", b.builtins(""))
	}
	b.registry(func(f *registryFile) { f.hires = f.hires[:1] })
	b.tick()
	if len(b.builtins("")) != 0 {
		t.Errorf("a hire gone from the registry: %v", b.builtins(""))
	}
	// One hired later gets its own.
	b.registry(func(f *registryFile) {
		f.hires = append(f.hires, Hire{ID: "n3w", Name: "Newbie", Role: "builder", Folder: filepath.Join(b.root, "newbie"), State: "working"})
	})
	b.tick()
	if b.builtinsOf("n3w") != 7 {
		t.Errorf("a new hire: %v", b.builtins(""))
	}
}

func TestBuiltinsGoWithTheManagerAndWithTheSettingAndComeBack(t *testing.T) {
	b := builtinDeck(t)
	if len(b.builtins("")) != 21 {
		t.Fatalf("to start with: %d", len(b.builtins("")))
	}
	b.watch(`{"kind":"time","target":"mine","minutes":5}`)
	// The manager off: the look that runs then (syncOnly) takes them away, the GM's own stay.
	b.d.mu.Lock()
	b.d.config.ManagerEnabled = false
	b.d.mu.Unlock()
	b.d.watchers.syncOnly()
	if len(b.builtins("")) != 0 || len(b.d.watchers.list()) != 1 {
		t.Fatalf("manager off: %v", b.d.watchers.list())
	}
	managerOn(b.d)
	b.tick()
	if len(b.builtins("")) != 21 {
		t.Fatalf("manager on again: %d", len(b.builtins("")))
	}
	// The setting off, with the manager on.
	b.d.mu.Lock()
	b.d.config.BuiltinWatches = false
	b.d.mu.Unlock()
	b.tick()
	if len(b.builtins("")) != 0 || len(b.d.watchers.list()) != 1 {
		t.Fatalf("setting off: %v", b.d.watchers.list())
	}
	// The saved file agrees.
	data, _ := os.ReadFile(filepath.Join(supportDir, "watchers.json"))
	if strings.Contains(string(data), `"builtin": "wait"`) {
		t.Errorf("watchers.json still has them: %s", data)
	}
}

func TestTheSettingIsOnUnlessTheFileSaysOff(t *testing.T) {
	isolate(t)
	if !loadConfig().BuiltinWatches {
		t.Error("not on by default")
	}
	if err := os.WriteFile(configFile, []byte(`{"builtinWatches": false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if loadConfig().BuiltinWatches {
		t.Error("the file said off")
	}
}

func TestAStatusFileFiresForNewLinesOnlyAndOnlyTheRightBuiltin(t *testing.T) {
	b := builtinDeck(t)
	status := filepath.Join(b.root, "STATUS.md")
	// A file that was there before the watches were set never fires for its old lines: set them again from a fresh start.
	b2 := watchDeck(t)
	old := filepath.Join(b2.root, "STATUS.md")
	putFile(t, old, "DONE long ago\nNEEDS-REVIEW: long ago\n")
	b2.d.mu.Lock()
	b2.d.config.BuiltinWatches = true
	b2.d.mu.Unlock()
	b2.tick()
	b2.tick()
	if len(b2.fired()) != 0 {
		t.Fatalf("old lines fired: %v", b2.firedWords())
	}
	appendTo(t, old, "BLOCKED: no key\n")
	b2.tick()
	if got := b2.firedWords(); len(got) != 1 || got[0] != "status: Rev: STATUS.md: BLOCKED: no key" {
		t.Fatalf("a new line: %v", got)
	}

	// A file that appears after the watches is read from its first line.
	putFile(t, status, "DONE first\n")
	b.tick()
	if got := b.firedWords(); len(got) != 1 || got[0] != "status: Rev: STATUS.md: DONE first" {
		t.Fatalf("a file that came later: %v", got)
	}
	if b.fired()[0].Agent != "" {
		t.Errorf("a status line has a mark: %q", b.fired()[0].Agent)
	}
	appendTo(t, status, "- NEEDS-REVIEW: the plan\n* STOP now\nplain text\nnote NEEDS-REVIEW: no\n NEEDS-REVIEW without colon\nDONEZO\nBUDGET REQUEST: 3 more\n")
	b.tick()
	got := b.fired()
	words := b.firedWords()[1:]
	want := []string{"needs-review: Rev: STATUS.md: - NEEDS-REVIEW: the plan", "status: Rev: STATUS.md: * STOP now", "status: Rev: STATUS.md: BUDGET REQUEST: 3 more"}
	if len(words) != 3 {
		t.Fatalf("fired %v", words)
	}
	// Both watches look at the same file: the order across them is the registry's, so compare as a set.
	for _, w := range want {
		found := false
		for _, g := range words {
			found = found || g == w
		}
		if !found {
			t.Errorf("%q not in %v", w, words)
		}
	}
	for _, e := range got[1:] {
		mark := strings.HasPrefix(e.Detail, "needs-review: ")
		if mark != (e.Agent == "needs-review") {
			t.Errorf("mark %q on %q", e.Agent, e.Detail)
		}
		if e.Kind != "watcherFired" || e.Folder != b.root {
			t.Errorf("the line: %+v", e)
		}
	}
	// The same lines don't fire again.
	b.tick()
	if len(b.fired()) != 4 {
		t.Errorf("fired again: %v", b.firedWords())
	}
	// And the team folder's file is watched too.
	if err := os.MkdirAll(filepath.Join(b.root, ".team"), 0o700); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(b.root, ".team", "STATUS.md"), "NEEDS-REVIEW: in the team folder\n")
	b.tick()
	last := b.fired()[len(b.fired())-1]
	if last.Detail != "needs-review: Rev: STATUS.md: NEEDS-REVIEW: in the team folder" || last.Agent != "needs-review" || last.Folder != filepath.Join(b.root, ".team") {
		t.Errorf("the team file: %+v", last)
	}
}

func TestABurstOfStatusLinesFiresTenAtALookAndNeverLosesTheRest(t *testing.T) {
	b := builtinDeck(t)
	status := filepath.Join(b.root, "STATUS.md")
	var lines strings.Builder
	for i := 0; i < 12; i++ {
		lines.WriteString("DONE part " + string(rune('a'+i)) + "\n")
	}
	putFile(t, status, lines.String())
	b.tick()
	if got := b.firedWords(); len(got) != 10 || got[0] != "status: Rev: STATUS.md: DONE part a" {
		t.Fatalf("a burst: %d %v", len(got), got)
	}
}

func TestADeniedPermissionAndAUsageLimitFireOnlyInTheHiresFolder(t *testing.T) {
	b := builtinDeck(t)
	w := b.d.watchers
	other := readRegistry(registryPath(), false).hires[1].Folder
	w.observe(eventEntry{ID: 1, At: b.clock.UnixMilli(), Kind: "permissionDenied", Folder: b.root, Detail: "Bash: rm -rf"})
	w.observe(eventEntry{ID: 2, At: b.clock.UnixMilli(), Kind: "permissionDenied", Folder: other, Detail: "Edit: x"})
	w.observe(eventEntry{ID: 3, At: b.clock.UnixMilli(), Kind: "permissionDenied", Folder: t.TempDir(), Detail: "nobody's"})
	w.observe(eventEntry{ID: 4, At: b.clock.UnixMilli(), Kind: "permissionAsked", Folder: b.root, Detail: "Bash"})
	got := b.firedWords()
	if len(got) != 2 || got[0] != "denied: permissionDenied in "+filepath.Base(b.root)+": Bash: rm -rf" || got[1] != "denied: permissionDenied in "+filepath.Base(other)+": Edit: x" {
		t.Fatalf("denied: %v", got)
	}
	if b.fired()[0].Folder != b.root {
		t.Errorf("folder %q", b.fired()[0].Folder)
	}
	// A repeating built-in goes on firing.
	w.observe(eventEntry{ID: 5, At: b.clock.UnixMilli(), Kind: "permissionDenied", Folder: b.root, Detail: "again"})
	if len(b.fired()) != 3 {
		t.Errorf("the second denial: %v", b.firedWords())
	}

	w.observe(eventEntry{ID: 6, At: b.clock.UnixMilli(), Kind: "notification", Folder: b.root, Detail: "Claude is waiting for your input"})
	w.observe(eventEntry{ID: 7, At: b.clock.UnixMilli(), Kind: "notification", Folder: b.root, Detail: "usage limit reached"})
	w.observe(eventEntry{ID: 8, At: b.clock.UnixMilli(), Kind: "idle", Folder: b.root, Detail: "usage limit"})
	if len(b.fired()) != 3 {
		t.Fatalf("not a limit: %v", b.firedWords())
	}
	w.observe(eventEntry{ID: 9, At: b.clock.UnixMilli(), Kind: "notification", Folder: b.root, Detail: "usage limit"})
	got = b.firedWords()
	if len(got) != 4 || got[3] != "limit: notification in "+filepath.Base(b.root)+": usage limit" {
		t.Fatalf("limit: %v", got)
	}
}

func TestAHireWaitingTwoMinutesAtAQuestionFiresOncePerWait(t *testing.T) {
	b := builtinDeck(t)
	b.hook(map[string]any{"hook_event_name": "SessionStart", "session_id": "s1", "cwd": b.root, "source": "startup"})
	b.tick()
	b.clock = b.clock.Add(3 * time.Minute)
	b.tick()
	if len(b.fired()) != 0 {
		t.Fatalf("not asking: %v", b.firedWords())
	}
	ask := func() {
		b.hook(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": b.root, "tool_name": "Edit",
			"tool_input": map[string]any{"file_path": filepath.Join(b.root, "a.go")}})
	}
	ask()
	b.tick() // first seen asking
	b.clock = b.clock.Add(time.Minute)
	b.tick()
	if len(b.fired()) != 0 {
		t.Fatalf("one minute: %v", b.firedWords())
	}
	b.clock = b.clock.Add(time.Minute)
	b.tick()
	b.tick()
	if got := b.firedWords(); len(got) != 1 || got[0] != "wait: Rev has waited 2 min at a question" || b.fired()[0].Folder != b.root {
		t.Fatalf("two minutes: %v", got)
	}
	b.clock = b.clock.Add(10 * time.Minute)
	b.tick()
	if len(b.fired()) != 1 {
		t.Fatalf("the same wait again: %v", b.firedWords())
	}
	// Answered, then a new question: a new wait.
	b.hook(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": b.root, "prompt": "go"})
	b.tick()
	ask()
	b.tick()
	b.clock = b.clock.Add(2 * time.Minute)
	b.tick()
	if len(b.fired()) != 2 {
		t.Fatalf("the next wait: %v", b.firedWords())
	}
}

func TestTheDigestIsOnlyWhileAGMIsAtWorkAndTellsWhoDidWhat(t *testing.T) {
	b := builtinDeck(t)
	if len(b.builtins("digest")) != 0 {
		t.Fatal("a digest with no GM")
	}
	b.registry(func(f *registryFile) {
		f.hires = append(f.hires, Hire{ID: "g1", Name: "General Manager", Role: gmRole, Folder: filepath.Join(b.root, "gm"), State: "working"})
	})
	b.tick()
	digest := b.builtins("digest")
	if len(digest) != 1 || digest[0]["kind"] != "time" || digest[0]["minutes"] != 60.0 && digest[0]["minutes"] != 60 ||
		digest[0]["due"] != b.clock.UnixMilli()+60*60000 && digest[0]["due"] != float64(b.clock.UnixMilli()+60*60000) {
		t.Fatalf("the digest: %v", digest)
	}
	if b.builtinsOf("g1") != 1 {
		t.Errorf("the GM has more than the digest: %d", b.builtinsOf("g1"))
	}
	// A hire finishes, another is quiet.
	putFile(t, filepath.Join(b.root, "STATUS.md"), "DONE the thing\n")
	b.d.watchers.observe(eventEntry{ID: 1, At: b.clock.UnixMilli(), Kind: "idle", Folder: readRegistry(registryPath(), false).hires[1].Folder})
	b.tick()
	b.clock = b.clock.Add(59 * time.Minute)
	b.tick()
	if len(b.firedWords()) != 1 {
		t.Fatalf("too soon: %v", b.firedWords())
	}
	b.clock = b.clock.Add(2 * time.Minute)
	b.tick()
	got := b.firedWords()
	want := "digest: Hourly digest. Working: Rev, Zed. Idle: Janitor. Waiting: none. Done since last: Rev."
	if len(got) != 2 || got[1] != want {
		t.Fatalf("the digest: %v", got)
	}
	// The finished are told once; the next hour starts from the digest.
	b.clock = b.clock.Add(61 * time.Minute)
	b.tick()
	if got := b.firedWords(); len(got) != 3 || !strings.HasSuffix(got[2], "Done since last: none.") {
		t.Fatalf("the next digest: %v", got)
	}
	// The GM leaves: no more digests.
	b.registry(func(f *registryFile) { f.hires[len(f.hires)-1].State = "fired" })
	b.tick()
	if len(b.builtins("digest")) != 0 {
		t.Errorf("the GM is fired: %v", b.builtins("digest"))
	}
}

func TestADigestNamesWhoIsWaitingAndHowManyAreLeftOut(t *testing.T) {
	hires := []Hire{
		{ID: "1", Name: "A", Folder: "/w/a", State: "working"}, {ID: "2", Name: "B", Nickname: "Bee", Folder: "/w/b", State: "working"},
		{ID: "3", Name: "C", Folder: "/w/c", State: "working"}, {ID: "4", Name: "D", Folder: "/w/d", State: "working"},
		{ID: "5", Name: "E", Folder: "/w/e", State: "working"}, {ID: "6", Name: "F", Folder: "/w/f", State: "working"},
		{ID: "7", Name: "G", Folder: "/w/g", State: "working"}, {ID: "8", Name: "H", Folder: "/w/h", State: "fired"},
		{ID: "9", Name: "GM", Role: gmRole, Folder: "/w/gm", State: "working"}, {ID: "10", Name: "I", Folder: "/w/i", State: "working"},
	}
	got := digestText(hires, map[string]eventEntry{"/w/i/sub": {ID: 4, Kind: "idle"}}, map[string]float64{"/w/a/deep": 1}, []string{"X", "Y"})
	want := "Hourly digest. Working: Bee, C, D, E, F, G. Idle: I. Waiting: A. Done since last: X, Y."
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
	more := append(hires, Hire{ID: "11", Name: "J", Folder: "/w/j", State: "working"})
	if got := digestText(more, nil, nil, nil); !strings.Contains(got, "Working: A, Bee, C, D, E, F +3.") {
		t.Errorf("more than six: %s", got)
	}
}

func TestBuiltinsSurviveARestartWithoutFiringAgainOrDoubling(t *testing.T) {
	b := builtinDeck(t)
	status := filepath.Join(b.root, "STATUS.md")
	putFile(t, status, "DONE one\n")
	b.tick()
	if len(b.fired()) != 1 {
		t.Fatalf("before: %v", b.firedWords())
	}
	registry := registryPath()
	again := newWatchers(supportDir, homeDir,
		func() []string {
			return watchRoots(readRegistry(registry, false).hires, filepath.Join(supportDir, "hires"))
		},
		func() []Hire { return readRegistry(registry, false).hires }, b.d.events.append, nil)
	again.setBuiltins(func() bool { return true }, nil)
	again.now = func() time.Time { return b.clock }
	if len(again.all) != 21 {
		t.Fatalf("read back: %d", len(again.all))
	}
	again.tick()
	again.tick()
	if len(b.fired()) != 1 || len(again.all) != 21 {
		t.Fatalf("after a restart: %v, %d watches", b.firedWords(), len(again.all))
	}
	appendTo(t, status, "DONE two\n")
	again.tick()
	if got := b.firedWords(); len(got) != 2 || got[1] != "status: Rev: STATUS.md: DONE two" {
		t.Fatalf("a new line after it: %v", got)
	}
}

func TestBuiltinsOnlyReadFilesInsideTheHiresFolder(t *testing.T) {
	b := watchDeck(t)
	secret := filepath.Join(t.TempDir(), "secret.md")
	putFile(t, secret, "")
	// STATUS.md is a link out of the folder: it is never read, and the look doesn't stop.
	if err := os.Symlink(secret, filepath.Join(b.root, "STATUS.md")); err != nil {
		t.Skip("no links here")
	}
	b.d.mu.Lock()
	b.d.config.BuiltinWatches = true
	b.d.mu.Unlock()
	b.tick()
	appendTo(t, secret, "DONE leaked\n")
	b.tick()
	b.tick()
	if len(b.fired()) != 0 {
		t.Fatalf("read through a link: %v", b.firedWords())
	}
}
