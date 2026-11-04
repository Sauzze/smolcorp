package desktop

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/gm"
)

// ---- helpers

type watchBench struct {
	t     *testing.T
	d     *Deck
	root  string // Rev's folder, which exists
	gm    string
	clock time.Time
}

// watchDeck is a deck with the manager on, a registry with Rev (at work, in root), Jan (idle, in another folder) and
// Bob (fired), and a clock the test moves.
func watchDeck(t *testing.T) *watchBench {
	t.Helper()
	d := newTestDeck(t)
	managerOn(d)
	// The app's own watches have their own tests (watchers_builtin_test.go); these look at the GM's.
	d.mu.Lock()
	d.config.BuiltinWatches = false
	d.mu.Unlock()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	other, _ := filepath.EvalSymlinks(t.TempDir())
	file := registryFile{hires: []Hire{
		{ID: "a1b2c3", Name: "Reviewer", Nickname: "Rev", Role: "reviewer", Folder: root, State: "working"},
		{ID: "d4e5f6", Name: "Janitor", Role: "janitor", Folder: other, State: "idle"},
		{ID: "9z9z9z", Name: "Zed", Role: "builder", Folder: filepath.Join(root, "zed"), State: "working"},
		{ID: "0a0b0c", Name: "Builder", Nickname: "Bob", Role: "builder", Folder: filepath.Join(root, "old"), State: "fired"},
	}}
	if err := file.save(registryPath()); err != nil {
		t.Fatal(err)
	}
	b := &watchBench{t: t, d: d, root: root, gm: d.newGMToken(), clock: time.UnixMilli(1761000000000)}
	d.watchers.mu.Lock()
	d.watchers.now = func() time.Time { return b.clock }
	d.watchers.mu.Unlock()
	return b
}

func (b *watchBench) call(method, target, body string, asGM bool) *httptest.ResponseRecorder {
	c := call{method: method, target: target, host: "localhost:7710", from: thisComputer, body: body, headers: map[string]string{setupHeader: "1"}}
	if asGM {
		c.withoutSetupKey = true
		c.headers[gmTokenHeader] = b.gm
	}
	return serve(b.t, b.d, c)
}

// watch sets a watch as the GM and returns it as answered.
func (b *watchBench) watch(body string) map[string]any {
	b.t.Helper()
	w := b.call("POST", "/setup/gm/watch", strings.ReplaceAll(body, "$ROOT", b.root), true)
	if w.Code != http.StatusOK {
		b.t.Fatalf("watch %s: %d %s", body, w.Code, w.Body)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		b.t.Fatal(err)
	}
	return got
}

// fired is the watcherFired lines in the event log, oldest first.
func (b *watchBench) fired() []eventEntry {
	var out []eventEntry
	for _, e := range b.d.events.snapshot() {
		if e.Kind == "watcherFired" {
			out = append(out, e)
		}
	}
	return out
}

func (b *watchBench) tick() { b.d.watchers.tick() }

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func putFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

type watchFixture struct {
	Requests      map[string]json.RawMessage
	AnswerToFile  map[string]any                        `json:"answer-to-file"`
	UnwatchAnswer map[string]any                        `json:"unwatch-answer"`
	EventKinds    []string                              `json:"event-kinds"`
	FiredLine     struct{ Kind, Detail, Folder string } `json:"fired-line-in-the-event-log"`
	Refusals      []struct {
		Request json.RawMessage
		Status  int
		Why     string
	}
}

func loadWatchFixture(t *testing.T) watchFixture {
	var f watchFixture
	if err := json.Unmarshal(frontDeskFixture(t, "watch.json"), &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// ---- the routes, as the fixtures say

func TestWatchAnswersAsWatchJSONSaysForEachKind(t *testing.T) {
	b := watchDeck(t)
	fx := loadWatchFixture(t)
	want := map[string]map[string]any{
		"file":           {"kind": "file", "target": b.root + "/STATUS.md", "pattern": `\bDONE\b`, "repeat": false},
		"file-in-a-hire": {"kind": "file", "target": b.root + "/STATUS.md", "pattern": "NEEDS-REVIEW", "repeat": true},
		"event":          {"kind": "event", "target": "permissionDenied", "pattern": "", "repeat": true, "folder": b.root},
		"idle":           {"kind": "idle", "target": b.root, "pattern": "", "repeat": false, "minutes": 10.0},
		"exit":           {"kind": "exit", "target": "a1b2c3", "pattern": "", "repeat": false},
		"time":           {"kind": "time", "target": "check the build", "pattern": "", "repeat": true, "minutes": 30.0, "due": float64(1761000000000 + 30*60000)},
	}
	for name, raw := range fx.Requests {
		got := b.watch(string(raw))
		if got["id"] == "" || got["id"] == nil || got["by"] != "gm" || got["createdAt"] != float64(1761000000000) || got["fired"] != 0.0 || got["active"] != true {
			t.Errorf("%s: answered %v", name, got)
		}
		for _, key := range []string{"id", "createdAt", "by", "fired", "active"} {
			delete(got, key)
		}
		if !reflect.DeepEqual(got, want[name]) {
			t.Errorf("%s: answered %v, want %v", name, got, want[name])
		}
	}
	// The shape of the answer is the fixture's, to the last key.
	file := b.watch(string(fx.Requests["file"]))
	keys := func(m map[string]any) (out []string) {
		for k := range m {
			out = append(out, k)
		}
		return
	}
	if len(keys(file)) != len(keys(fx.AnswerToFile)) {
		t.Errorf("keys %v, fixture %v", keys(file), keys(fx.AnswerToFile))
	}
	for k := range fx.AnswerToFile {
		if _, ok := file[k]; !ok {
			t.Errorf("the answer has no %q", k)
		}
	}
}

func TestWatchRefusesAsWatchJSONSays(t *testing.T) {
	b := watchDeck(t)
	fx := loadWatchFixture(t)
	for _, r := range fx.Refusals {
		body := strings.ReplaceAll(string(r.Request), "$ROOT", b.root)
		if w := b.call("POST", "/setup/gm/watch", body, true); w.Code != r.Status {
			t.Errorf("%s (%s): %d %s, want %d", body, r.Why, w.Code, w.Body, r.Status)
		}
	}
	if got := b.d.watchers.list(); len(got) != 0 {
		t.Errorf("a refused watch was kept: %v", got)
	}
	// The kinds an event watch may name are the fixture's.
	w := b.call("POST", "/setup/gm/watch", `{"kind":"event","target":"nothing"}`, true)
	if w.Code != 400 || !strings.HasSuffix(w.Body.String(), strings.Join(fx.EventKinds, ", ")+".") {
		t.Errorf("a bad event: %d %s", w.Code, w.Body)
	}
	if w := b.call("POST", "/setup/gm/watch", `{"kind":"event","target":"watcherFired"}`, true); w.Code != 400 {
		t.Errorf("an event watch on firings themselves: %d", w.Code)
	}
	for _, body := range []string{`[]`, `nothing`, `null`, ``} {
		if w := b.call("POST", "/setup/gm/watch", body, true); w.Code != 400 || !strings.HasPrefix(w.Body.String(), "Send {kind, target") {
			t.Errorf("body %q: %d %s", body, w.Code, w.Body)
		}
	}
	for _, body := range []string{`{"kind":"file","target":"STATUS.md","pattern":"x"}`, `{"kind":"idle","minutes":5}`, `{"kind":"time","minutes":0.5,"repeat":true}`, `{"kind":"time"}`, `{"kind":"file","target":"` + b.root + `/a"}`} {
		if w := b.call("POST", "/setup/gm/watch", body, true); w.Code != 400 {
			t.Errorf("%s: %d %s, want 400", body, w.Code, w.Body)
		}
	}
	// A hire that is gone: 409 for an exit; a name nobody has: 404 for a file in a hire.
	if w := b.call("POST", "/setup/gm/watch", `{"kind":"exit","target":"Bob"}`, true); w.Code != 409 {
		t.Errorf("exit of a fired hire: %d", w.Code)
	}
	if w := b.call("POST", "/setup/gm/watch", `{"kind":"file","hire":"Nobody","target":"S.md","pattern":"x"}`, true); w.Code != 404 {
		t.Errorf("a file in an unknown hire: %d", w.Code)
	}
}

func TestWatchesAnswersAsWatchesJSONSaysAndLeavesOutWhereItHasRead(t *testing.T) {
	b := watchDeck(t)
	var fx struct {
		Answer struct{ Watches []map[string]any }
	}
	if err := json.Unmarshal(frontDeskFixture(t, "watches.json"), &fx); err != nil {
		t.Fatal(err)
	}
	// watchers.json as the Mac keeps it: the same fields, and where the file watch has read to.
	kept := []map[string]any{}
	for _, w := range fx.Answer.Watches {
		k := map[string]any{}
		for key, v := range w {
			k[key] = v
		}
		if s, ok := k["target"].(string); ok {
			k["target"] = strings.ReplaceAll(s, "$ROOT", b.root)
		}
		if k["kind"] == "file" {
			k["offset"], k["inode"], k["tail"] = 120, 4242, 99
		}
		kept = append(kept, k)
	}
	data, _ := json.Marshal(map[string]any{"version": 1, "watches": kept})
	putFile(t, filepath.Join(supportDir, "watchers.json"), string(data))
	b.d.watchers.mu.Lock()
	b.d.watchers.all = loadWatchers(b.d.watchers.file)
	b.d.watchers.mu.Unlock()
	for _, asGM := range []bool{true, false} {
		w := b.call("GET", "/setup/gm/watches", "", asGM)
		var got struct{ Watches []map[string]any }
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil {
			t.Fatalf("watches: %d %s", w.Code, w.Body)
		}
		for _, x := range fx.Answer.Watches {
			x["target"] = strings.ReplaceAll(fmt.Sprint(x["target"]), "$ROOT", b.root)
		}
		if !reflect.DeepEqual(got.Watches, fx.Answer.Watches) {
			t.Errorf("gm=%v answered %v, want %v", asGM, got.Watches, fx.Answer.Watches)
		}
	}
	// A tool with the setup key reads them too, but may not set or take away.
	if w := serve(t, b.d, call{method: "GET", target: "/setup/gm/watches", host: "localhost:7710", from: thisComputer, asTool: true}); w.Code != 200 {
		t.Errorf("a tool reads: %d", w.Code)
	}
	for _, c := range []call{{method: "POST", target: "/setup/gm/watch", body: `{"kind":"exit","target":"Rev"}`}, {method: "DELETE", target: "/setup/gm/unwatch?id=w-0001"}, {method: "POST", target: "/setup/gm/unwatch?id=w-0001"}} {
		c.host, c.from, c.asTool, c.headers = "localhost:7710", thisComputer, true, map[string]string{setupHeader: "1"}
		if w := serve(t, b.d, c); w.Code != http.StatusForbidden {
			t.Errorf("a tool %s %s: %d, want 403", c.method, c.target, w.Code)
		}
	}
	if len(b.d.watchers.list()) != 2 {
		t.Error("a refused tool changed the watches")
	}
}

func TestUnwatchTakesAWatchAwayByQueryOrBodyAndRefusesTheRest(t *testing.T) {
	b := watchDeck(t)
	fx := loadWatchFixture(t)
	first := b.watch(`{"kind":"exit","target":"Rev"}`)["id"].(string)
	second := b.watch(`{"kind":"exit","target":"Janitor"}`)["id"].(string)
	third := b.watch(`{"kind":"time","target":"x","minutes":3}`)["id"].(string)
	for _, c := range []struct{ method, target, body, id string }{
		{"POST", "/setup/gm/unwatch?id=" + first, "", first},
		{"DELETE", "/setup/gm/unwatch", `{"id":"` + second + `"}`, second},
		{"DELETE", "/setup/gm/unwatch?id=" + third, "", third},
	} {
		w := b.call(c.method, c.target, c.body, true)
		var got map[string]any
		json.Unmarshal(w.Body.Bytes(), &got)
		if w.Code != 200 || !reflect.DeepEqual(got, map[string]any{"id": c.id, "removed": fx.UnwatchAnswer["removed"]}) {
			t.Errorf("%s %s: %d %s", c.method, c.target, w.Code, w.Body)
		}
	}
	if len(b.d.watchers.list()) != 0 {
		t.Error("the watches are still there")
	}
	for _, c := range []struct {
		method, target, body string
		status               int
	}{
		{"POST", "/setup/gm/unwatch", "", 400}, {"DELETE", "/setup/gm/unwatch", `{"id":""}`, 400}, {"POST", "/setup/gm/unwatch?id=", `{"id":"x"}`, 400},
		{"POST", "/setup/gm/unwatch?id=" + first, "", 404},
	} {
		if w := b.call(c.method, c.target, c.body, true); w.Code != c.status {
			t.Errorf("%s %s %s: %d, want %d", c.method, c.target, c.body, w.Code, c.status)
		}
	}
	// The app's own watch stays (watchers_builtin.go sets them; one kept in the file is enough here).
	b.d.watchers.mu.Lock()
	b.d.watchers.all = append(b.d.watchers.all, watch{ID: "wbuiltin", Kind: "event", Target: "idle", Builtin: "denied", Repeats: true})
	b.d.watchers.mu.Unlock()
	if w := b.call("DELETE", "/setup/gm/unwatch?id=wbuiltin", "", true); w.Code != 403 {
		t.Errorf("a built-in: %d, want 403", w.Code)
	}
}

func TestOnlyFiftyWatchesAndTheAppsOwnDontCount(t *testing.T) {
	b := watchDeck(t)
	b.d.watchers.mu.Lock()
	b.d.watchers.all = append(b.d.watchers.all, watch{ID: "wbuiltin", Kind: "event", Target: "idle", Builtin: "denied", Repeats: true})
	b.d.watchers.mu.Unlock()
	for i := 0; i < 50; i++ {
		b.watch(`{"kind":"time","target":"t","minutes":5}`)
	}
	w := b.call("POST", "/setup/gm/watch", `{"kind":"time","target":"t","minutes":5}`, true)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "50 watches") {
		t.Errorf("the 51st: %d %s", w.Code, w.Body)
	}
	// The limit is looked at before anything else, as on the Mac.
	if w := b.call("POST", "/setup/gm/watch", `{"kind":"nothing"}`, false); w.Code != http.StatusBadRequest {
		t.Errorf("an unknown kind at the limit: %d", w.Code)
	}
	if w := b.call("POST", "/setup/gm/watch", `{"kind":"time","pattern":"(","minutes":5}`, false); w.Code != http.StatusConflict {
		t.Errorf("a bad pattern at the limit: %d", w.Code)
	}
	if len(b.d.watchers.list()) != 51 {
		t.Errorf("%d watches", len(b.d.watchers.list()))
	}
}

func TestWatchIsMarkedByTheGMOrTheOwner(t *testing.T) {
	b := watchDeck(t)
	page := b.call("POST", "/setup/gm/watch", `{"kind":"exit","target":"Rev"}`, false)
	var got map[string]any
	json.Unmarshal(page.Body.Bytes(), &got)
	if page.Code != 200 || got["by"] != "owner" {
		t.Errorf("from the page: %d %s", page.Code, page.Body)
	}
	if b.watch(`{"kind":"exit","target":"Janitor"}`)["by"] != "gm" {
		t.Error("from the GM, by is gm")
	}
}

// ---- where a file watch may look

func TestAFileWatchLooksOnlyInsideAHiresFolderOrTheHiresFolderAndNeverAtSecrets(t *testing.T) {
	b := watchDeck(t)
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	hiresDir := filepath.Join(supportDir, "hires")
	os.MkdirAll(filepath.Join(hiresDir, "rev"), 0o700)
	os.MkdirAll(filepath.Join(b.root, ".git"), 0o700)
	os.MkdirAll(filepath.Join(b.root, ".config"), 0o700)
	putFile(t, filepath.Join(outside, "S.md"), "x\n")
	os.Symlink(outside, filepath.Join(b.root, "link"))
	os.Symlink(filepath.Join(outside, "S.md"), filepath.Join(b.root, "S.md"))
	home := homeDir
	os.WriteFile(filepath.Join(home, "x.md"), nil, 0o600)
	trial := func(path, hire string) int {
		body, _ := json.Marshal(map[string]any{"kind": "file", "target": path, "pattern": "x", "hire": hire})
		return b.call("POST", "/setup/gm/watch", string(body), true).Code
	}
	for path, want := range map[string]int{
		filepath.Join(b.root, "STATUS.md"):               200, // not there yet is fine
		filepath.Join(b.root, "sub", "deep", "S.md"):     200,
		filepath.Join(hiresDir, "rev", "STATUS.md"):      200,
		filepath.Join(b.root, ".env"):                    403,
		filepath.Join(b.root, ".env.local"):              403,
		filepath.Join(b.root, ".git", "config"):          403,
		filepath.Join(b.root, ".config", "x"):            403,
		filepath.Join(b.root, "..", "elsewhere", "S.md"): 403,
		filepath.Join(b.root, "link", "S.md"):            403, // a link out
		filepath.Join(b.root, "S.md"):                    403, // a link to a file outside
		filepath.Join(outside, "S.md"):                   403,
		"/etc/hosts":                                     403,
		filepath.Join(home, "x.md"):                      403,
	} {
		if got := trial(path, ""); got != want {
			t.Errorf("%s: %d, want %d", path, got, want)
		}
	}
	// A short path is inside the hire's folder; a folder that is the home folder, or above it, opens nothing.
	if got := trial("STATUS.md", "Rev"); got != 200 {
		t.Errorf("a short path in Rev: %d", got)
	}
	if got := trial("../S.md", "Rev"); got != 403 {
		t.Errorf("a short path out of Rev: %d", got)
	}
	if _, ok := watchInside(filepath.Join(home, "x.md"), []string{home, filepath.Dir(home), "/"}, home); ok {
		t.Error("the home folder, or above it, was taken for a hire's folder")
	}
	if got, ok := watchInside(filepath.Join(home, "app", "S.md"), []string{filepath.Join(home, "app")}, home); !ok || !strings.HasSuffix(got, "S.md") {
		t.Errorf("a folder under home is a hire's: %q %v", got, ok)
	}
}

func TestAFileWatchReadingAgainRefusesALinkSwappedInSince(t *testing.T) {
	b := watchDeck(t)
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	secret := filepath.Join(outside, "S.md")
	putFile(t, secret, "")
	path := filepath.Join(b.root, "STATUS.md")
	b.watch(`{"kind":"file","target":"$ROOT/STATUS.md","pattern":"DONE","repeat":true}`)
	if err := os.Symlink(secret, path); err != nil {
		t.Fatal(err)
	}
	appendTo(t, secret, "DONE secret\n")
	b.tick()
	if got := b.fired(); len(got) != 0 {
		t.Errorf("a link out was read: %v", got)
	}
}

// ---- checking: only new lines fire

func TestAFileWatchFiresOnNewMatchingLinesOnlyAndAsTheFixtureWords(t *testing.T) {
	b := watchDeck(t)
	fx := loadWatchFixture(t)
	path := filepath.Join(b.root, "STATUS.md")
	putFile(t, path, "DONE before\n") // there when set: never fires
	b.d.watchers.mu.Lock()
	b.d.watchers.newID = func() string { return "w-0001" }
	b.d.watchers.mu.Unlock()
	b.watch(`{"kind":"file","target":"$ROOT/STATUS.md","pattern":"\\bDONE\\b"}`)
	b.tick()
	if len(b.fired()) != 0 {
		t.Fatalf("a line that was there fired: %v", b.fired())
	}
	appendTo(t, path, "working on it\nDONE T405\nDONE again") // the last line has no newline yet
	b.tick()
	got := b.fired()
	if len(got) != 1 || got[0].Detail != fx.FiredLine.Detail || got[0].Kind != fx.FiredLine.Kind || got[0].Folder != b.root {
		t.Fatalf("fired %v, want detail %q kind %q folder %q", got, fx.FiredLine.Detail, fx.FiredLine.Kind, b.root)
	}
	// It does not repeat: done once, it is kept, fired, and no longer active.
	appendTo(t, path, "\nDONE more\n")
	b.tick()
	if len(b.fired()) != 1 {
		t.Errorf("a watch that does not repeat fired again: %v", b.fired())
	}
	list := b.d.watchers.list()
	if len(list) != 1 || list[0]["active"] != false || list[0]["fired"] != 1 || list[0]["firedAt"] != int64(1761000000000) {
		t.Errorf("after firing: %v", list)
	}
}

func TestARepeatingFileWatchFiresOncePerLookAndSaysHowManyMore(t *testing.T) {
	b := watchDeck(t)
	path := filepath.Join(b.root, "STATUS.md")
	b.watch(`{"kind":"file","target":"$ROOT/STATUS.md","pattern":"^NEEDS","repeat":true}`)
	appendTo(t, path, "NEEDS-REVIEW: one\nnot this\nNEEDS-REVIEW: two\nNEEDS-REVIEW: three\n")
	b.tick()
	appendTo(t, path, "NEEDS-REVIEW: four\n")
	b.tick()
	b.tick() // nothing new
	got := b.fired()
	if len(got) != 2 || !strings.HasSuffix(got[0].Detail, "(file): STATUS.md: NEEDS-REVIEW: one (+2 more)") || !strings.HasSuffix(got[1].Detail, "STATUS.md: NEEDS-REVIEW: four") {
		t.Fatalf("fired %v", got)
	}
	if list := b.d.watchers.list(); list[0]["fired"] != 2 || list[0]["active"] != true {
		t.Errorf("after: %v", list)
	}
}

func TestAFileThatIsShortenedOrReplacedStartsOverWithoutFiringOldLines(t *testing.T) {
	b := watchDeck(t)
	path := filepath.Join(b.root, "STATUS.md")
	putFile(t, path, "DONE one\nsome long line to be cut off later\n")
	b.watch(`{"kind":"file","target":"$ROOT/STATUS.md","pattern":"DONE","repeat":true}`)
	// Shorter (truncated): its old lines don't come back, new ones after do.
	putFile(t, path, "DONE old\n")
	b.tick()
	if len(b.fired()) != 0 {
		t.Fatalf("a truncated file fired its old line: %v", b.fired())
	}
	appendTo(t, path, "DONE new1\n")
	b.tick()
	if got := b.fired(); len(got) != 1 || !strings.HasSuffix(got[0].Detail, "DONE new1") {
		t.Fatalf("after truncation: %v", got)
	}
	// Replaced by another file (another inode), longer than before: not read from the start.
	other := filepath.Join(b.root, "other.md")
	putFile(t, other, "DONE r1\nDONE r2\nDONE r3\nDONE r4\n")
	if err := os.Rename(other, path); err != nil {
		t.Fatal(err)
	}
	b.tick()
	if len(b.fired()) != 1 {
		t.Fatalf("a replaced file fired its old lines: %v", b.fired())
	}
	appendTo(t, path, "DONE after\n")
	b.tick()
	if got := b.fired(); len(got) != 2 || !strings.HasSuffix(got[1].Detail, "DONE after") {
		t.Fatalf("after replacing: %v", got)
	}
	// Rewritten in place, as long or longer: the tail hash tells it from an addition.
	info, _ := os.Stat(path)
	size := int(info.Size())
	putFile(t, path, strings.Repeat("DONE zzzzzz\n", size/12+3))
	b.tick()
	if len(b.fired()) != 2 {
		t.Fatalf("a file rewritten in place fired its old lines: %v", b.fired())
	}
	appendTo(t, path, "DONE last\n")
	b.tick()
	if got := b.fired(); len(got) != 3 || !strings.HasSuffix(got[2].Detail, "DONE last") {
		t.Fatalf("after rewriting: %v", got)
	}
}

// A watch kept without the hash of its tail (older, or from an empty file) is still told when its file is shorter or another file.
func TestLookingAtAFileWithoutATailStillTellsAShorterOrAnotherFile(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	path := filepath.Join(dir, "S.md")
	putFile(t, path, "DONE one\nDONE two\n")
	_, inode, _, _ := watchFileState(dir, path)
	if look := lookAtFile(dir, path, 18, inode, 0); look.offset != 18 || len(look.lines) != 0 {
		t.Errorf("nothing new: %+v", look)
	}
	putFile(t, path, "DONE x\n")
	if look := lookAtFile(dir, path, 18, inode, 0); look.offset != 7 || len(look.lines) != 0 {
		t.Errorf("a shorter file: %+v", look)
	}
	other := filepath.Join(dir, "other.md")
	putFile(t, other, "DONE a\nDONE b\nDONE c\nDONE d\n")
	os.Rename(other, path)
	if look := lookAtFile(dir, path, 18, inode, 0); look.offset != 28 || len(look.lines) != 0 {
		t.Errorf("another file as long: %+v", look)
	}
	// An empty file that was there when the watch was set: its replacement is another file too.
	putFile(t, path, "")
	_, empty, _, _ := watchFileState(dir, path)
	putFile(t, other, "DONE r1\n")
	os.Rename(other, path)
	if look := lookAtFile(dir, path, 0, empty, 0); len(look.lines) != 0 || look.offset != 8 {
		t.Errorf("a replaced empty file: %+v", look)
	}
}

func TestAFileThatAppearsLaterIsReadFromItsFirstLine(t *testing.T) {
	b := watchDeck(t)
	b.watch(`{"kind":"file","target":"$ROOT/late.md","pattern":"DONE"}`)
	b.tick()
	putFile(t, filepath.Join(b.root, "late.md"), "DONE first\n")
	b.tick()
	if got := b.fired(); len(got) != 1 || !strings.HasSuffix(got[0].Detail, "late.md: DONE first") {
		t.Fatalf("fired %v", got)
	}
}

func TestWhereAFileWatchHasReadSurvivesARestartAndIsKeptInWatchersJSONPrivately(t *testing.T) {
	b := watchDeck(t)
	path := filepath.Join(b.root, "STATUS.md")
	putFile(t, path, "DONE old\n")
	b.watch(`{"kind":"file","target":"$ROOT/STATUS.md","pattern":"DONE","repeat":true}`)
	appendTo(t, path, "DONE seen\n")
	b.tick()
	file := filepath.Join(supportDir, "watchers.json")
	info, err := os.Stat(file)
	if err != nil || !modeIs(info, 0o600) {
		t.Fatalf("watchers.json: %v %v", err, info)
	}
	var kept struct {
		Version int
		Watches []map[string]any
	}
	data, _ := os.ReadFile(file)
	if json.Unmarshal(data, &kept) != nil || kept.Version != 1 || len(kept.Watches) != 1 {
		t.Fatalf("watchers.json = %s", data)
	}
	for _, key := range []string{"id", "kind", "target", "pattern", "folder", "minutes", "repeat", "by", "createdAt", "firedAt", "fired", "due", "offset", "inode", "tail", "lastEvent", "builtin", "hire"} {
		if _, ok := kept.Watches[0][key]; !ok {
			t.Errorf("watchers.json has no %q", key)
		}
	}
	// A new run of the app reads it and fires nothing for what was seen.
	again := newWatchers(supportDir, homeDir, func() []string { return []string{b.root} }, func() []Hire { return nil }, b.d.events.append, nil)
	again.tick()
	appendTo(t, path, "DONE fresh\n")
	again.tick()
	got := b.fired()
	if len(got) != 2 || !strings.HasSuffix(got[0].Detail, "DONE seen") || !strings.HasSuffix(got[1].Detail, "DONE fresh") {
		t.Fatalf("fired %v", got)
	}
}

func TestAWatchersFileThatCannotBeReadIsSetAsideAndAMacsFileStillReads(t *testing.T) {
	isolate(t)
	file := filepath.Join(supportDir, "watchers.json")
	for _, broken := range []string{"not json", `{"version":1}`, `{"version":1,"watches":null}`, `{"watches":[{"id":"w1","kind":"nothing"}]}`, `{"watches":[{"kind":"time"}]}`} {
		putFile(t, file, broken)
		if got := newWatchers(supportDir, homeDir, nil, nil, nil, nil); len(got.all) != 0 {
			t.Errorf("%q read as %v", broken, got.all)
		}
		if aside, err := os.ReadFile(filepath.Join(supportDir, "watchers.broken.json")); err != nil || string(aside) != broken {
			t.Errorf("%q was not set aside: %v %q", broken, err, aside)
		}
		if _, err := os.Stat(file); err == nil {
			t.Errorf("%q was left where it was", broken)
		}
	}
	// Written before some fields were added, or with one of the wrong type: it reads.
	putFile(t, file, `{"watches":[{"id":"w1","kind":"time","target":"x","minutes":"soon","repeat":true,"due":5.5,"fired":"no"}]}`)
	got := newWatchers(supportDir, homeDir, nil, nil, nil, nil).all
	want := []watch{{ID: "w1", Kind: "time", Target: "x", Repeats: true, Due: 5.5}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("read %+v, want %+v", got, want)
	}
	// Nothing there is no watches, and nothing is set aside.
	os.Remove(file)
	os.Remove(filepath.Join(supportDir, "watchers.broken.json"))
	if got := newWatchers(supportDir, homeDir, nil, nil, nil, nil); len(got.all) != 0 {
		t.Error("an empty folder has watches")
	}
	if _, err := os.Stat(filepath.Join(supportDir, "watchers.broken.json")); err == nil {
		t.Error("a missing file was set aside")
	}
}

func TestATimeWatchFiresWhenDueOnceOrEveryNMinutesWithoutABurst(t *testing.T) {
	b := watchDeck(t)
	b.watch(`{"kind":"time","target":"check the build","minutes":10}`)
	b.watch(`{"kind":"time","target":"","minutes":5,"repeat":true}`)
	b.clock = b.clock.Add(4 * time.Minute)
	b.tick()
	if len(b.fired()) != 0 {
		t.Fatalf("early: %v", b.fired())
	}
	b.clock = b.clock.Add(2 * time.Minute) // 6 min
	b.tick()
	b.clock = b.clock.Add(4 * time.Minute) // 10 min: the first is due, the second is not (due again at 11)
	b.tick()
	b.clock = b.clock.Add(3 * time.Hour) // a long sleep: one firing, not thirty
	b.tick()
	b.tick()
	var details []string
	for _, e := range b.fired() {
		details = append(details, e.Detail[strings.Index(e.Detail, "("):])
	}
	want := []string{"(time): the time came", "(time): check the build", "(time): the time came"}
	if !reflect.DeepEqual(details, want) {
		t.Fatalf("fired %v, want %v", details, want)
	}
	for _, x := range b.d.watchers.list() {
		switch x["target"] {
		case "check the build":
			if x["active"] != false || x["fired"] != 1 || x["due"] != nil {
				t.Errorf("the one-off: %v", x)
			}
		default:
			if x["active"] != true || x["fired"] != 2 || x["due"] != float64(b.clock.UnixMilli()+5*60000) && x["due"] != b.clock.UnixMilli()+5*60000 {
				t.Errorf("the repeating one: %v", x)
			}
		}
	}
}

func TestAnExitWatchFiresWhenTheHireIsFiredFailedOrGone(t *testing.T) {
	b := watchDeck(t)
	rev := b.watch(`{"kind":"exit","target":"Rev","repeat":true}`)
	if rev["repeat"] != false {
		t.Error("a repeating exit would fire at every look")
	}
	b.watch(`{"kind":"exit","target":"Janitor"}`)
	b.tick()
	if len(b.fired()) != 0 {
		t.Fatalf("hires at work: %v", b.fired())
	}
	file := readRegistry(registryPath(), false)
	file.hires[0].State = "fired"
	file.hires[1].State = "failed"
	file.save(registryPath())
	b.tick()
	b.tick()
	got := b.fired()
	if len(got) != 2 || !strings.HasSuffix(got[0].Detail, "(exit): Rev is gone (fired)") || !strings.HasSuffix(got[1].Detail, "(exit): Janitor is gone (failed)") {
		t.Fatalf("fired %v", got)
	}
	// Taken out of the registry altogether.
	b.watch(`{"kind":"exit","target":"Zed"}`)
	file = readRegistry(registryPath(), false)
	file.hires = file.hires[:2]
	file.save(registryPath())
	b.tick()
	if got := b.fired(); len(got) != 3 || !strings.HasSuffix(got[2].Detail, "9z9z9z is gone (not in the registry)") {
		t.Fatalf("fired %v", got)
	}
}

func TestAnEventWatchFiresOnItsKindInItsFolderMatchingItsPattern(t *testing.T) {
	b := watchDeck(t)
	w := b.d.watchers
	b.watch(`{"kind":"event","target":"permissionDenied","folder":"$ROOT","pattern":"^Bash","repeat":true}`)
	b.watch(`{"kind":"event","target":"idle"}`)
	for i, e := range []eventEntry{
		{Kind: "permissionDenied", Folder: filepath.Join(b.root, "sub"), Detail: "Bash: rm"}, // under the folder: fires
		{Kind: "permissionDenied", Folder: b.root + "x", Detail: "Bash: rm"},                 // a folder that only starts alike
		{Kind: "permissionDenied", Folder: b.root, Detail: "Edit: /x"},                       // the pattern is not met
		{Kind: "permissionAsked", Folder: b.root, Detail: "Bash: ls"},                        // another kind
		{Kind: "idle", Folder: "/somewhere/else"},                                            // any folder
		{Kind: "idle", Folder: "/somewhere/else"},                                            // done once, no more
		{Kind: "watcherFired", Folder: b.root, Detail: "Bash: ls"},                           // never
	} {
		e.ID = int64(i + 1)
		w.observe(e)
	}
	var details []string
	for _, e := range b.fired() {
		details = append(details, e.Detail[strings.Index(e.Detail, "("):])
	}
	want := []string{"(event): permissionDenied in sub: Bash: rm", "(event): idle in else"}
	if !reflect.DeepEqual(details, want) {
		t.Fatalf("fired %v, want %v", details, want)
	}
}

func TestAnIdleWatchFiresOnceForAQuietSpellAfterNMinutesAndEndsWhenTheHireLeaves(t *testing.T) {
	b := watchDeck(t)
	w := b.d.watchers
	w.observe(eventEntry{ID: 1, At: b.clock.UnixMilli(), Kind: "idle", Folder: b.root})
	// A quiet spell already running was not set by this watch.
	b.watch(`{"kind":"idle","target":"Rev","minutes":5,"repeat":true}`)
	b.clock = b.clock.Add(10 * time.Minute)
	b.tick()
	if len(b.fired()) != 0 {
		t.Fatalf("a spell that began before the watch: %v", b.fired())
	}
	w.observe(eventEntry{ID: 2, At: b.clock.UnixMilli(), Kind: "prompt", Folder: b.root})
	w.observe(eventEntry{ID: 3, At: b.clock.UnixMilli(), Kind: "idle", Folder: b.root + "/sub"}) // a folder under it counts
	b.clock = b.clock.Add(4 * time.Minute)
	b.tick()
	if len(b.fired()) != 0 {
		t.Fatalf("too soon: %v", b.fired())
	}
	w.observe(eventEntry{ID: 4, At: b.clock.UnixMilli(), Kind: "notification", Folder: b.root}) // not work: the spell goes on
	b.clock = b.clock.Add(2 * time.Minute)
	b.tick()
	b.tick()
	got := b.fired()
	if len(got) != 1 || !strings.HasSuffix(got[0].Detail, "(idle): "+filepath.Base(b.root)+" has been idle for 5 min") {
		t.Fatalf("fired %v", got)
	}
	// The hire leaves: nothing will ever go idle again, so the watch is over.
	file := readRegistry(registryPath(), false)
	file.hires[0].State = "fired"
	file.save(registryPath())
	b.tick()
	if list := w.list(); list[0]["active"] != false {
		t.Errorf("after the hire left: %v", list)
	}
}

// ---- the gm command, against this server

func TestTheGMCommandsWatchUnwatchAndWatchesWorkAgainstTheServer(t *testing.T) {
	b := watchDeck(t)
	server := httptest.NewServer(&router{deck: b.d})
	defer server.Close()
	t.Setenv("SMOLCORP_PORT", server.URL[strings.LastIndex(server.URL, ":")+1:])
	t.Setenv("SMOLCORP_GM_TOKEN", b.gm)
	t.Setenv("SMOLCORP_HIRE_KEY", "")
	run := func(args ...string) (int, string, string) {
		var out, errOut strings.Builder
		code := gm.Run(args, &out, &errOut)
		return code, out.String(), errOut.String()
	}
	code, out, errOut := run("watch", "time", "check the build", "--minutes", "30", "--repeat")
	if code != 0 || !strings.Contains(out, "set: time check the build") {
		t.Fatalf("watch: %d %q %q", code, out, errOut)
	}
	code, out, errOut = run("watch", "file", b.root+"/STATUS.md", "--pattern", `\bDONE\b`)
	if code != 0 {
		t.Fatalf("watch file: %d %q %q", code, out, errOut)
	}
	code, _, errOut = run("watch", "file", "/etc/hosts", "--pattern", "x")
	if code != 1 || !strings.Contains(errOut, "Only files inside a hire's folder") {
		t.Errorf("a refused watch: %d %q", code, errOut)
	}
	code, out, errOut = run("watches")
	if code != 0 || !strings.Contains(out, "check the build") || !strings.Contains(out, `\bDONE\b`) || !strings.Contains(out, "gm") {
		t.Fatalf("watches: %d %q %q", code, out, errOut)
	}
	code, out, _ = run("watches", "--json")
	var listed struct{ Watches []struct{ ID, Kind string } }
	if code != 0 || json.Unmarshal([]byte(out), &listed) != nil || len(listed.Watches) != 2 {
		t.Fatalf("watches --json: %d %q", code, out)
	}
	code, out, errOut = run("unwatch", listed.Watches[0].ID)
	if code != 0 || !strings.Contains(out, "taken away") || len(b.d.watchers.list()) != 1 {
		t.Fatalf("unwatch: %d %q %q", code, out, errOut)
	}
	if code, _, errOut = run("unwatch", listed.Watches[0].ID); code != 1 || !strings.Contains(errOut, "No watch") {
		t.Errorf("unwatch again: %d %q", code, errOut)
	}
}

// ---- fix 1

func TestAHardLinkToAFileOutsideIsNeverWatchedOrRead(t *testing.T) {
	b := watchDeck(t)
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	secret := filepath.Join(outside, "key")
	putFile(t, secret, "")
	// A watch set first, then the file put in its place as a second name of the secret.
	path := filepath.Join(b.root, "STATUS.md")
	b.watch(`{"kind":"file","target":"$ROOT/STATUS.md","pattern":"DONE","repeat":true}`)
	if err := os.Link(secret, path); err != nil {
		t.Skip("no hard links here:", err)
	}
	appendTo(t, secret, "DONE secret\n")
	b.tick()
	if got := b.fired(); len(got) != 0 {
		t.Errorf("a hard link was read: %v", got)
	}
	// And a new watch on it is refused.
	linked := filepath.Join(b.root, "linked.md")
	if err := os.Link(secret, linked); err != nil {
		t.Fatal(err)
	}
	w := b.call("POST", "/setup/gm/watch", `{"kind":"file","target":"`+linked+`","pattern":"."}`, true)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "Only files inside") {
		t.Errorf("a hard link: %d %s", w.Code, w.Body)
	}
}

// ---- fix 2

func TestAFileReachedThroughALinkedFolderIsNotOpenedForALook(t *testing.T) {
	b := watchDeck(t)
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	putFile(t, filepath.Join(outside, "S.md"), "DONE\n")
	if err := os.Symlink(outside, filepath.Join(b.root, "dir")); err != nil {
		t.Skip("no links here:", err)
	}
	// As if the folder was swapped for a link after watchInside told the path was plain.
	path := filepath.Join(b.root, "dir", "S.md")
	// The open itself refuses it (the checks after it could be raced): the
	// folder's root is walked one part at a time and a link out of it is refused.
	if f, err := openWatchedIn(b.root, path); err == nil {
		f.Close()
		t.Error("the open went through a linked folder out of the hire's folder")
	}
	if f, _ := openPlainFile(b.root, path); f != nil {
		f.Close()
		t.Error("a file behind a linked folder was opened")
	}
	if look := lookAtFile(b.root, path, 0, 0, 0); len(look.lines) != 0 {
		t.Errorf("a file behind a linked folder was read: %v", look.lines)
	}
	if _, _, _, ok := watchFileState(b.root, path); ok {
		t.Error("a file behind a linked folder has a state")
	}
}

// ---- fix 3

func TestAHugeNumberOfMinutesIsRefusedAndTheFileStaysLoadable(t *testing.T) {
	b := watchDeck(t)
	for _, body := range []string{
		`{"kind":"time","target":"x","minutes":1e305}`,
		`{"kind":"time","target":"x","minutes":525601}`,
		`{"kind":"idle","target":"Rev","minutes":1e305}`,
		`{"kind":"idle","target":"Rev","minutes":525601}`,
	} {
		w := b.call("POST", "/setup/gm/watch", body, true)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "a year at most") {
			t.Errorf("%s: %d %s", body, w.Code, w.Body)
		}
	}
	b.watch(`{"kind":"time","target":"a year","minutes":525600}`)
	b.watch(`{"kind":"time","target":"soon","minutes":5}`)
	if got := loadWatchers(b.d.watchers.file); len(got) != 2 {
		t.Errorf("watchers.json holds %d watches, want 2", len(got))
	}
}

func TestSavingAWatchThatCannotBeEncodedSaysSo(t *testing.T) {
	b := watchDeck(t)
	var logged strings.Builder
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)
	w := b.d.watchers
	w.mu.Lock()
	w.all = append(w.all, watch{ID: "wbad", Kind: "time", Due: math.Inf(1)})
	w.save()
	w.mu.Unlock()
	if !strings.Contains(logged.String(), "can't save") {
		t.Errorf("a failed save was silent: %q", logged.String())
	}
}

// ---- fix 5

func TestStoppingTheWatchersEndsBothLoops(t *testing.T) {
	b := watchDeck(t)
	b.watch(`{"kind":"event","target":"idle","repeat":true}`)
	b.d.events.append(eventEntry{Kind: "idle", Folder: b.root})
	deadline := time.Now().Add(5 * time.Second)
	for len(b.fired()) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if len(b.fired()) == 0 {
		t.Fatal("the loop never saw the event")
	}
	done := make(chan struct{})
	go func() { b.d.stopWatchers(); b.d.stopWatchers(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the watchers did not stop")
	}
	before := len(b.fired())
	b.d.events.append(eventEntry{Kind: "idle", Folder: b.root})
	time.Sleep(300 * time.Millisecond)
	if len(b.fired()) != before {
		t.Error("an event was watched after the stop")
	}
}

// ---- fix 6

func TestSubfoldersAreToldByTheSystemsOwnSeparator(t *testing.T) {
	b := watchDeck(t)
	b.watch(`{"kind":"event","target":"idle","folder":"$ROOT"}`)
	b.d.watchers.observe(eventEntry{ID: 1, Kind: "idle", Folder: filepath.Join(b.root, "sub", "deeper")})
	if len(b.fired()) != 1 {
		t.Errorf("an event in a subfolder did not fire: %v", b.fired())
	}
	latest := map[string]eventEntry{filepath.Join(b.root, "sub"): {ID: 4}, b.root + "x": {ID: 9}}
	if got, ok := quietest(b.root, latest); !ok || got.ID != 4 {
		t.Errorf("quietest = %v %v, want the subfolder's event", got, ok)
	}
}

// ---- fix 4 (through the command)

func TestWatchesListsAFractionOfAMinute(t *testing.T) {
	b := watchDeck(t)
	b.watch(`{"kind":"time","target":"half","minutes":0.5}`)
	server := httptest.NewServer(&router{deck: b.d})
	defer server.Close()
	t.Setenv("SMOLCORP_PORT", server.URL[strings.LastIndex(server.URL, ":")+1:])
	t.Setenv("SMOLCORP_GM_TOKEN", b.gm)
	t.Setenv("SMOLCORP_HIRE_KEY", "")
	var out, errOut strings.Builder
	if code := gm.Run([]string{"watches", "--all"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "half") {
		t.Fatalf("watches: %d %q %q", code, out.String(), errOut.String())
	}
}
