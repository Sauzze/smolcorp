//go:build !windows

package desktop

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// POST /setup/gm/hire (gmhire.go) on the engine tests' stand-ins (engineDeck), with the shared fixtures hire.json and hire-guard.json.

const asGMCaller, asPageCaller, asHireCaller = "gm", "page", "hire"

// gmHireDeck is the engine's deck, the manager on, its repository (moved into the home folder) the folder for new hires, and a GM token.
func gmHireDeck(t *testing.T) (*Deck, *engineRig, *hiring, string) {
	t.Helper()
	d, r, h := engineDeck(t)
	moved := filepath.Join(homeDir, "proj")
	if err := os.Rename(r.repo, moved); err != nil {
		t.Fatal(err)
	}
	r.repo, d.config.HireFolder = moved, moved
	d.engine.HeardFrom = func(Hire) bool { return true }
	managerOn(d)
	return d, r, h, d.newGMToken()
}

// postGMHire is POST /setup/gm/hire from as, carrying token (the GM's, or a hire's key).
func postGMHire(t *testing.T, d *Deck, as, token, body string) (int, string) {
	t.Helper()
	c := call{method: "POST", target: "/setup/gm/hire", host: "localhost:7710", from: thisComputer, body: body, headers: map[string]string{setupHeader: "1"}}
	if as != asPageCaller {
		c.withoutSetupKey, c.headers[map[string]string{asGMCaller: gmTokenHeader, asHireCaller: hireKeyHeader}[as]] = true, token
	}
	w := serve(t, d, c)
	d.watching.Wait()
	return w.Code, strings.TrimSpace(w.Body.String())
}

// nothingMade fails if a refused hire made a row, a window, a session or a worktree.
func nothingMade(t *testing.T, r *engineRig, h *hiring) {
	t.Helper()
	trees, _ := filepath.Glob(filepath.Join(filepath.Dir(r.repo), "*-hires"))
	if got := registered(t); len(got) != 0 || r.tmux.ran("new-session") || len(h.windows) != 0 || len(h.tmux) != 0 || len(trees) != 0 {
		t.Errorf("a refused hire made something: registry %+v, tmux %q, windows %q, worktrees %q", got, r.tmux.asked, h.windows, trees)
	}
}

// Every refusal in hire.json with its status, and the answer in its shape. The 409 row is refused for want
// of a folder, so it runs with none set up.
func TestTheGMHireAnswersAndRefusesAsTheFixtureDoes(t *testing.T) {
	var f struct {
		Answer struct {
			Status int
			Body   map[string]any
		}
		Refusals []struct {
			Request json.RawMessage
			Status  int
		}
	}
	if err := json.Unmarshal(frontDeskFixture(t, "hire.json"), &f); err != nil || len(f.Refusals) < 5 {
		t.Fatalf("hire.json: %d refusals, %v", len(f.Refusals), err)
	}
	for _, c := range f.Refusals {
		d, r, h, token := gmHireDeck(t)
		if c.Status == http.StatusConflict {
			d.config.HireFolder = ""
		}
		code, text := postGMHire(t, d, asGMCaller, token, string(c.Request))
		if code != c.Status || text == "" || c.Status == http.StatusConflict && text != setupHireNeedsFolder {
			t.Errorf("%s: %d %q, want %d (and setupHireNeedsFolder's words for 409)", c.Request, code, text, c.Status)
		}
		nothingMade(t, r, h)
	}
	d, r, _, token := gmHireDeck(t)
	code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor"}`)
	var got map[string]any
	json.Unmarshal([]byte(text), &got)
	if hires := registered(t); code != f.Answer.Status || len(got) != len(f.Answer.Body) || got["ok"] != true || got["session"] != nil || got["desk"] != 0.0 ||
		len(hires) != 1 || hires[0].Role != "janitor" || hires[0].Worktree == "" || !r.tmux.ran("new-session") {
		t.Errorf("answer %d %q, registry %+v: want %d, ok, desk 0, no session, a worktree hire by the engine", code, text, hires, f.Answer.Status)
	}
}

// Only the GM and the page, the manager on, and never the gm role for the GM's token.
func TestTheGMHireCallersAndTheGMRole(t *testing.T) {
	d, r, h, token := gmHireDeck(t)
	writeRegistry(t, Hire{ID: "a1b2c3", Name: "Rev", Folder: "/w", Role: "reviewer", State: "working"})
	key, err := newSendKey(registryPath(), "a1b2c3")
	if code, _ := postGMHire(t, d, asHireCaller, key, `{"role":"janitor"}`); err != nil || code != http.StatusForbidden {
		t.Errorf("a hire's token: %d (key error %v), want 403", code, err)
	}
	for _, as := range []string{asGMCaller, asPageCaller} {
		d.mu.Lock()
		d.config.ManagerEnabled = false
		d.mu.Unlock()
		if code, text := postGMHire(t, d, as, token, `{"role":"janitor"}`); code != http.StatusConflict || text != managerOffAnswer {
			t.Errorf("%s, manager off: %d %q, want 409 %q", as, code, text, managerOffAnswer)
		}
	}
	managerOn(d)
	if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"gm"}`); code != http.StatusConflict || text != onlyOwnerHiresGMRefusal {
		t.Errorf("no GM yet: %d %q, want %q", code, text, onlyOwnerHiresGMRefusal)
	}
	writeRegistry(t, Hire{ID: "g1", Name: "Boss", Folder: "/w", Role: "gm", State: "working"})
	if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"gm"}`); code != http.StatusConflict || text != secondGMRefusal {
		t.Errorf("a GM is working: %d %q, want %q", code, text, secondGMRefusal)
	}
	if r.tmux.ran("new-session") || len(h.windows) != 0 || len(registered(t)) != 1 {
		t.Errorf("a refusal made something: %+v", registered(t))
	}
}

// The folder: the one for new hires by default, a given one inside home, never the home folder or a way out of it.
func TestTheGMHireFolder(t *testing.T) {
	outside := physical(t.TempDir())
	for _, c := range []struct {
		name, hires, folder string // "~" is the home folder; hires "" is none set up
		status              int
	}{
		{"the folder for new hires", "~/proj", "", 200}, {"a folder given inside home", "~/proj", "~/other", 200},
		{"none set up", "", "", 409}, {"the one set up is gone", "~/gone", "", 409}, {"the one set up is the home folder", "~", "", 400},
		{"the one set up is outside home", outside, "", 400}, {"outside home", "~/proj", outside, 400}, {"/etc", "~/proj", "/etc", 400},
		{"the home folder", "~/proj", "~", 400}, {"home spelt out", "~/proj", "~/proj/../.", 400}, {"a link out of home", "~/proj", "~/escape", 400},
	} {
		d, r, h, token := gmHireDeck(t)
		os.MkdirAll(filepath.Join(homeDir, "other"), 0o755)
		os.Symlink(outside, filepath.Join(homeDir, "escape"))
		at := func(p string) string { return strings.Replace(p, "~", homeDir, 1) }
		d.config.HireFolder = at(c.hires)
		body, _ := json.Marshal(map[string]any{"role": "janitor", "folder": at(c.folder)})
		code, text := postGMHire(t, d, asGMCaller, token, string(body))
		if code != c.status || c.status == 409 && text != setupHireNeedsFolder {
			t.Errorf("%s: %d %q, want %d", c.name, code, text, c.status)
		}
		if hires := registered(t); c.status != 200 {
			nothingMade(t, r, h)
		} else if len(hires) != 1 || (c.folder != "" && hires[0].Folder != at(c.folder)) || hires[0].Folder == homeDir {
			t.Errorf("%s: registry %+v", c.name, hires)
		}
	}
}

// What the Go path can't honour is refused naming the field; what's already so is honoured; a floor is the hire's, on its first free desk.
func TestTheGMHireFieldsAndFloor(t *testing.T) {
	d, r, h, token := gmHireDeck(t)
	for field, value := range map[string]string{"account": `"work"`, "model": `"opus"`, "nickname": `"Ada"`, "apiKey": `true`,
		"worktree": `false`, "task": `"Review it."`, "answers": `["a"]`, "floor": `6`, "floor ": `"2"`} {
		code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor","`+strings.TrimSpace(field)+`":`+value+`}`)
		if code != http.StatusBadRequest || !strings.Contains(text, field) && field != "floor" || strings.Contains(text, "\n") {
			t.Errorf("%s: %d %q, want 400 naming it", field, code, text)
		}
	}
	nothingMade(t, r, h)
	if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor","apiKey":false,"worktree":true,"task":"","answers":[],"account":null,"floor":2}`); code != http.StatusOK || !strings.Contains(text, `"desk":0`) {
		t.Fatalf("what is already so, on floor 2: %d %q", code, text)
	}
	d.mu.Lock()
	seat := d.engineSeats[registered(t)[0].TmuxSocket]
	d.mu.Unlock()
	if seat.floor != 2 || seat.desk != 0 {
		t.Errorf("the hire's seat is %+v, want floor 2, desk 0", seat)
	}
}

// Only the two fenced ways: what would open a plain terminal is refused in one sentence, with nothing opened;
// a hire from the office keeps its behaviour; the window setting doesn't move the GM's hire out of the engine.
func TestTheGMHireNeverOpensAPlainTerminal(t *testing.T) {
	for _, c := range []struct{ name, want string }{{"no tmux", "Install tmux: "}, {"another agent", "only Claude roles"}} {
		d, r, h, token := gmHireDeck(t)
		role := "janitor"
		if c.name == "no tmux" {
			tmuxPath = func() string { return "" }
		} else {
			role = "scribe"
			d.config.Roles = append(d.config.Roles, Role{ID: role, Name: "Scribe", Agent: "codex"})
		}
		if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"`+role+`"}`); code != http.StatusConflict || !strings.Contains(text, c.want) || strings.Contains(text, "\n") {
			t.Errorf("%s: %d %q, want 409 with %q", c.name, code, text, c.want)
		}
		nothingMade(t, r, h)
		if status, message := d.Perform("hire", "", url.Values{"role": {role}}); status != http.StatusNoContent || len(h.windows) != 1 && c.name == "no tmux" {
			t.Errorf("%s, from the office: %d %q, %d windows, want what it did before", c.name, status, message, len(h.windows))
		}
	}
	d, r, _, token := gmHireDeck(t)
	d.config.HireInTmux = false
	if code, _ := postGMHire(t, d, asGMCaller, token, `{"role":"janitor"}`); code != http.StatusOK || !r.tmux.ran("new-session") {
		t.Errorf("tmux not chosen for hires: %d, tmux %q", code, r.tmux.asked)
	}
}

// The same fence: each case of hire-guard.json (as it is, then each group turned the other way) through a desk hire
// and through /setup/gm/hire starts with the same guard and deny settings.
func TestTheGMHireHasTheSameFenceAsAHireFromTheOffice(t *testing.T) {
	var file struct{ Groups []map[string]any }
	if err := json.Unmarshal(frontDeskFixture(t, "hire-guard.json"), &file); err != nil || len(file.Groups) == 0 {
		t.Fatalf("hire-guard.json: %d groups, %v", len(file.Groups), err)
	}
	cases := [][]map[string]any{file.Groups}
	for i, g := range file.Groups {
		turned := slices.Clone(file.Groups)
		turned[i] = map[string]any{"name": g["name"], "commands": g["commands"], "on": g["on"] != true}
		cases = append(cases, turned)
	}
	// hire makes a hire of role with the guard saying groups, and says what it started with, its own names made plain.
	hire := func(role string, groups []map[string]any, route bool) (settings string, guard []string) {
		d, r, _, token := gmHireDeck(t)
		printed, _ := json.Marshal(map[string]any{"groups": groups})
		d.engine.Guard = func(_ string, args []string) (int, string, bool) {
			guard = append(guard, strings.ReplaceAll(strings.Join(args, " "), supportDir, "<support>"))
			return 0, string(printed), true
		}
		as := asGMCaller
		if role == "gm" {
			as = asPageCaller // only the page hires the gm role
		}
		status, text := 0, ""
		if route {
			status, text = postGMHire(t, d, as, token, `{"role":"`+role+`"}`)
		} else {
			status, text = d.Perform("hire", "", url.Values{"role": {role}})
		}
		d.watching.Wait()
		hires := registered(t)
		if status/100 != 2 || len(hires) != 1 || !r.tmux.ran("new-session") {
			t.Fatalf("%s hire of %s: %d %q, registry %+v", map[bool]string{false: "desk", true: "route"}[route], role, status, text, hires)
		}
		data, _ := os.ReadFile(filepath.Join(hireFolder(supportDir, hires[0].ID), hireSettingsFile))
		plain := strings.NewReplacer(hires[0].ID, "<hire>", hires[0].Worktree, "<worktree>", d.config.HireFolder, "<repo>", supportDir, "<support>", homeDir, "<home>")
		return plain.Replace(string(data)), guard
	}
	for _, role := range []string{"janitor", "gm"} {
		var first string
		for i, groups := range cases {
			deskSettings, deskGuard := hire(role, groups, false)
			routeSettings, routeGuard := hire(role, groups, true)
			if deskSettings != routeSettings || !slices.Equal(deskGuard, routeGuard) || len(routeGuard) == 0 {
				t.Errorf("%s, case %d: settings or guard differ:\ndesk:  %s %q\nroute: %s %q", role, i, deskSettings, deskGuard, routeSettings, routeGuard)
			}
			if !strings.Contains(routeSettings, "PreToolUse") || (role == "gm") != strings.Contains(routeSettings, "--profile gm") || i > 0 && routeSettings == first {
				t.Errorf("%s, case %d: no guard hook, the wrong profile, or a group turned the other way changed nothing: %s", role, i, routeSettings)
			}
			if i == 0 {
				first = routeSettings
			}
		}
	}
}

// End to end on a real Deck on a test port (7750 to 7799), which is a test copy: T721 refuses Claude on the default
// account before anything is made, and no socket, worktree, row or session is left.
func TestARealDeckOnATestPortRefusesTheGMHireAsATestCopy(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux isn't installed, and this test runs a Deck whose hires use it")
	}
	d, r, h, token := gmHireDeck(t)
	var listener net.Listener
	was := Port
	t.Cleanup(func() { Port = was })
	for port := 7750; listener == nil && port <= 7799; port++ {
		if l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port)); err == nil {
			listener, Port = l, port
		}
	}
	if listener == nil {
		t.Skip("no port from 7750 to 7799 is free")
	}
	server := &http.Server{Handler: &router{deck: d}}
	go server.Serve(listener)
	defer server.Close()
	d.engine.Tmux = nil // a real tmux, on the hire's own socket in the short support folder
	req, _ := http.NewRequest("POST", "http://"+listener.Addr().String()+"/setup/gm/hire", strings.NewReader(`{"role":"janitor"}`))
	req.Header.Set(setupHeader, "1")
	req.Header.Set(gmTokenHeader, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	text, _ := io.ReadAll(resp.Body)
	sockets, _ := filepath.Glob(filepath.Join(supportDir, "smolcorp-*"))
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(text), "test copy of Smolcorp") || len(sockets) != 0 {
		t.Errorf("%d %q, sockets %q, want 409 and the test copy's refusal", resp.StatusCode, text, sockets)
	}
	nothingMade(t, r, h)
}
