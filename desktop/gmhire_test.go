//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// POST /setup/gm/hire (gmhire.go): the Mac's request and refusals, read from the
// shared fixtures, and the same fence as a hire from the office. The engine's
// stand-ins are the engine tests' (engineDeck): no real tmux, claude or git config.

// homeRepo is a git repository inside the home folder, where a GM's hire may work.
func homeRepo(t *testing.T) string {
	t.Helper()
	repo := physical(filepath.Join(homeDir, "Projects", "app"))
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	repo = physical(repo)
	os.WriteFile(filepath.Join(repo, "sub", "a.txt"), []byte("a"), 0o644)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "one"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return repo
}

// gmHireDeck is the engine's deck with the manager on, the folder for new hires a repository in the
// home folder, and a GM token.
func gmHireDeck(t *testing.T) (*Deck, *engineRig, *hiring, string) {
	t.Helper()
	d, r, h := engineDeck(t)
	d.config.HireFolder = homeRepo(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	managerOn(d)
	return d, r, h, d.newGMToken()
}

const (
	callAsGM   = "gm"
	callAsPage = "page"
	callAsHire = "hire"
)

// postGMHire is POST /setup/gm/hire from as: the GM with token, the setup page, or a hire with its key.
func postGMHire(t *testing.T, d *Deck, as, token, body string) (int, string) {
	t.Helper()
	c := call{method: "POST", target: "/setup/gm/hire", host: "localhost:7710", from: thisComputer, body: body,
		headers: map[string]string{setupHeader: "1"}}
	switch as {
	case callAsGM:
		c.withoutSetupKey, c.headers[gmTokenHeader] = true, token
	case callAsHire:
		c.withoutSetupKey, c.headers[hireKeyHeader] = true, token
	}
	w := serve(t, d, c)
	d.watching.Wait()
	return w.Code, strings.TrimSpace(w.Body.String())
}

type hireFixtureRefusal struct {
	Request json.RawMessage `json:"request"`
	Status  int             `json:"status"`
	Why     string          `json:"why"`
}

func hireFixture(t *testing.T) (answer struct {
	Status int            `json:"status"`
	Body   map[string]any `json:"body"`
}, refusals []hireFixtureRefusal) {
	t.Helper()
	var f struct {
		Answer struct {
			Status int            `json:"status"`
			Body   map[string]any `json:"body"`
		} `json:"answer"`
		Refusals []hireFixtureRefusal `json:"refusals"`
	}
	if err := json.Unmarshal(frontDeskFixture(t, "hire.json"), &f); err != nil {
		t.Fatal(err)
	}
	return f.Answer, f.Refusals
}

func nothingMade(t *testing.T, r *engineRig, h *hiring) {
	t.Helper()
	if got := registered(t); len(got) != 0 || r.tmux.ran("new-session") || len(h.windows) != 0 || len(h.tmux) != 0 {
		t.Errorf("a refused hire made something: registry %+v, tmux %q, windows %q, old tmux %q", got, r.tmux.asked, h.windows, h.tmux)
	}
}

// Every refusal in hire.json, with its status. The row that is refused for want of a folder is run with none set up.
func TestTheGMHireRefusesWhatTheFixtureRefuses(t *testing.T) {
	_, refusals := hireFixture(t)
	if len(refusals) < 5 {
		t.Fatalf("the fixture has %d refusals", len(refusals))
	}
	for _, c := range refusals {
		t.Run(c.Why, func(t *testing.T) {
			d, r, h, token := gmHireDeck(t)
			if c.Status == http.StatusConflict {
				d.config.HireFolder = ""
			}
			code, text := postGMHire(t, d, callAsGM, token, string(c.Request))
			if code != c.Status || text == "" {
				t.Errorf("%s: %d %q, want %d and a sentence", c.Request, code, text, c.Status)
			}
			if c.Status == http.StatusConflict && text != setupHireNeedsFolder {
				t.Errorf("%s: %q, want setupHireNeedsFolder's words", c.Request, text)
			}
			nothingMade(t, r, h)
		})
	}
}

// The answer is the fixture's: ok, the desk, and no session yet.
func TestTheGMHireAnswersAsTheFixtureDoes(t *testing.T) {
	answer, _ := hireFixture(t)
	d, r, _, token := gmHireDeck(t)
	code, text := postGMHire(t, d, callAsGM, token, `{"role":"janitor"}`)
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil || code != answer.Status {
		t.Fatalf("%d %q: %v, want %d", code, text, err, answer.Status)
	}
	if len(got) != len(answer.Body) || got["ok"] != answer.Body["ok"] || got["session"] != nil {
		t.Errorf("answer %v, want the shape of %v", got, answer.Body)
	}
	if desk, ok := got["desk"].(float64); !ok || desk != 0 {
		t.Errorf("desk %v, want 0, the first free desk", got["desk"])
	}
	if hires := registered(t); len(hires) != 1 || hires[0].Role != "janitor" || hires[0].Worktree == "" || !r.tmux.ran("new-session") {
		t.Errorf("registry %+v, tmux %q: not made by the engine", hires, r.tmux.asked)
	}
}

func TestTheGMHireIsOnlyForTheGMAndThePageAndOnlyWithTheManagerOn(t *testing.T) {
	d, r, h, token := gmHireDeck(t)
	hireKey := func() string {
		writeRegistry(t, Hire{ID: "a1b2c3", Name: "Rev", Folder: "/w", Role: "reviewer", State: "working"})
		key, err := newSendKey(registryPath(), "a1b2c3")
		if err != nil || key == "" {
			t.Fatalf("send key: %q %v", key, err)
		}
		return key
	}()
	if code, _ := postGMHire(t, d, callAsHire, hireKey, `{"role":"janitor"}`); code != http.StatusForbidden {
		t.Errorf("a hire's token: %d, want 403", code)
	}
	if len(registered(t)) != 1 || r.tmux.ran("new-session") || len(h.windows) != 0 {
		t.Errorf("a hire's token made a hire: %+v", registered(t))
	}
	d.mu.Lock()
	d.config.ManagerEnabled = false
	d.mu.Unlock()
	for _, as := range []string{callAsGM, callAsPage} {
		if code, text := postGMHire(t, d, as, token, `{"role":"janitor"}`); code != http.StatusConflict || text != managerOffAnswer {
			t.Errorf("%s with the manager off: %d %q, want 409 %q", as, code, text, managerOffAnswer)
		}
	}
	if len(registered(t)) != 1 || r.tmux.ran("new-session") {
		t.Errorf("a refused hire made something: %+v", registered(t))
	}
}

// One GM per office: the GM's token never hires the gm role, whether or not one is working, and the text says which.
func TestTheGMHireRefusesTheGMRoleToTheGMsToken(t *testing.T) {
	d, r, h, token := gmHireDeck(t)
	if code, text := postGMHire(t, d, callAsGM, token, `{"role":"gm"}`); code != http.StatusConflict || text != onlyOwnerHiresGMRefusal {
		t.Errorf("no GM yet: %d %q, want 409 %q", code, text, onlyOwnerHiresGMRefusal)
	}
	nothingMade(t, r, h)
	writeRegistry(t, Hire{ID: "g1", Name: "Boss", Folder: "/w", Role: "gm", State: "working"})
	if code, text := postGMHire(t, d, callAsGM, token, `{"role":"gm"}`); code != http.StatusConflict || text != secondGMRefusal {
		t.Errorf("a GM is working: %d %q, want 409 %q", code, text, secondGMRefusal)
	}
	if r.tmux.ran("new-session") || len(registered(t)) != 1 {
		t.Errorf("the refusal made something: %+v", registered(t))
	}
}

func TestTheGMHireFolder(t *testing.T) {
	t.Run("the folder for new hires when none is given", func(t *testing.T) {
		d, _, _, token := gmHireDeck(t)
		repo := d.config.HireFolder
		if code, text := postGMHire(t, d, callAsGM, token, `{"role":"janitor"}`); code != http.StatusOK {
			t.Fatalf("%d %q", code, text)
		}
		if hires := registered(t); len(hires) != 1 || hires[0].Repo != repo {
			t.Errorf("registry %+v, want a hire of %s", hires, repo)
		}
	})
	t.Run("a folder inside home is used", func(t *testing.T) {
		d, _, _, token := gmHireDeck(t)
		other := filepath.Join(homeDir, "Work", "other")
		if err := os.MkdirAll(other, 0o755); err != nil {
			t.Fatal(err)
		}
		other = physical(other)
		for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "one"}} {
			if out, err := exec.Command("git", append([]string{"-C", other}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v %s", args, err, out)
			}
		}
		body, _ := json.Marshal(map[string]any{"role": "janitor", "folder": other})
		if code, text := postGMHire(t, d, callAsGM, token, string(body)); code != http.StatusOK {
			t.Fatalf("%d %q", code, text)
		}
		if hires := registered(t); len(hires) != 1 || hires[0].Repo != other {
			t.Errorf("registry %+v, want a hire of %s", hires, other)
		}
	})
	t.Run("none set up is refused in setupHireNeedsFolder's words", func(t *testing.T) {
		for _, folder := range []string{"", filepath.Join(t.TempDir(), "gone")} {
			d, r, h, token := gmHireDeck(t)
			d.config.HireFolder = folder
			if code, text := postGMHire(t, d, callAsGM, token, `{"role":"janitor"}`); code != http.StatusConflict || text != setupHireNeedsFolder {
				t.Errorf("folder %q: %d %q, want 409 %q", folder, code, text, setupHireNeedsFolder)
			}
			nothingMade(t, r, h)
		}
	})
	t.Run("outside home, the home folder itself, or a way out of it is 400", func(t *testing.T) {
		d, r, h, token := gmHireDeck(t)
		outside := physical(t.TempDir())
		link := filepath.Join(homeDir, "Projects", "escape")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatal(err)
		}
		for name, folder := range map[string]string{"outside": outside, "home": homeDir, "home, spelt out": homeDir + "/Projects/..", "a link out": link, "/etc": "/etc"} {
			body, _ := json.Marshal(map[string]any{"role": "janitor", "folder": folder})
			if code, text := postGMHire(t, d, callAsGM, token, string(body)); code != http.StatusBadRequest || text == "" {
				t.Errorf("%s: %d %q, want 400", name, code, text)
			}
		}
		nothingMade(t, r, h)
	})
	t.Run("a default folder outside home is 400 too, and home itself is never the default", func(t *testing.T) {
		d, r, h, token := gmHireDeck(t)
		d.config.HireFolder = physical(t.TempDir())
		if code, _ := postGMHire(t, d, callAsGM, token, `{"role":"janitor"}`); code != http.StatusBadRequest {
			t.Errorf("a default outside home: %d, want 400", code)
		}
		d.config.HireFolder = homeDir
		if code, text := postGMHire(t, d, callAsGM, token, `{"role":"janitor"}`); code != http.StatusBadRequest {
			t.Errorf("home as the default: %d %q, want 400", code, text)
		}
		nothingMade(t, r, h)
	})
}

// hireAs's own backstop: a fenced hire never starts in the home folder, whoever calls it.
func TestAFencedHireNeverStartsInTheHomeFolder(t *testing.T) {
	d, r, h, _ := gmHireDeck(t)
	d.config.HireFolder = homeDir
	status, text := d.hireWith(hireSpec{roleID: "janitor", fromSetup: true, fenced: true})
	if status != http.StatusConflict || text != setupHireNeedsFolder {
		t.Errorf("%d %q, want 409 %q", status, text, setupHireNeedsFolder)
	}
	nothingMade(t, r, h)
}

// What the Go path can't honour yet is refused with a sentence naming it, never dropped.
func TestTheGMHireRefusesAFieldItCannotHonour(t *testing.T) {
	d, r, h, token := gmHireDeck(t)
	for field, value := range map[string]string{"account": `"work"`, "model": `"opus"`, "nickname": `"Ada"`, "floor": `2`, "apiKey": `true`,
		"worktree": `false`, "task": `"Review it."`, "answers": `["a"]`} {
		body := `{"role":"janitor","` + field + `":` + value + `}`
		code, text := postGMHire(t, d, callAsGM, token, body)
		if code != http.StatusBadRequest || !strings.Contains(text, `"`+field+`"`) {
			t.Errorf("%s: %d %q, want 400 naming it", body, code, text)
		}
	}
	nothingMade(t, r, h)
	// Left out, null, or what's already so: honoured.
	if code, text := postGMHire(t, d, callAsGM, token, `{"role":"janitor","apiKey":false,"worktree":true,"floor":0,"task":"","answers":[],"account":null}`); code != http.StatusOK {
		t.Errorf("what is already so: %d %q", code, text)
	}
}

// A test copy never hires Claude on the default account, on this route too (T721): before anything is made.
func TestTheGMHireRefusesTheDefaultAccountInATestCopy(t *testing.T) {
	d, r, h, token := gmHireDeck(t)
	saved := Port
	t.Cleanup(func() { Port = saved })
	Port = 7755
	if testCopyPort() != 7755 {
		t.Fatalf("not a test copy: %d", testCopyPort())
	}
	code, text := postGMHire(t, d, callAsGM, token, `{"role":"janitor"}`)
	if code != http.StatusConflict || !strings.Contains(text, "test copy of Smolcorp") {
		t.Errorf("%d %q, want 409 and the test copy's refusal", code, text)
	}
	nothingMade(t, r, h)
}

// Only the two fenced ways: what would open a plain terminal is refused in one sentence, with nothing opened or typed.
func TestTheGMHireNeverOpensAPlainTerminal(t *testing.T) {
	t.Run("no tmux", func(t *testing.T) {
		d, r, h, token := gmHireDeck(t)
		tmuxPath = func() string { return "" }
		code, text := postGMHire(t, d, callAsGM, token, `{"role":"janitor"}`)
		if code != http.StatusConflict || !strings.HasPrefix(text, "Install tmux: ") || strings.Contains(text, "\n") {
			t.Errorf("%d %q, want 409 and a sentence naming tmux", code, text)
		}
		nothingMade(t, r, h)
		// The same desk's hire from the office keeps its behaviour: it opens a window as before.
		d.mu.Lock()
		d.lastHire = d.lastHire.Add(-hireCooldown)
		d.mu.Unlock()
		if status, message := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusNoContent || len(h.windows) != 1 {
			t.Errorf("a hire from the office: %d %q, %d windows", status, message, len(h.windows))
		}
	})
	t.Run("another agent", func(t *testing.T) {
		d, r, h, token := gmHireDeck(t)
		d.config.Roles = append(d.config.Roles, Role{ID: "scribe", Name: "Scribe", Agent: "codex"})
		code, text := postGMHire(t, d, callAsGM, token, `{"role":"scribe"}`)
		if code != http.StatusConflict || !strings.Contains(text, "only Claude roles") {
			t.Errorf("%d %q, want 409 naming Claude", code, text)
		}
		nothingMade(t, r, h)
	})
	t.Run("tmux is there but the window setting says no tmux: still the engine", func(t *testing.T) {
		d, r, _, token := gmHireDeck(t)
		d.config.HireInTmux = false
		if code, text := postGMHire(t, d, callAsGM, token, `{"role":"janitor"}`); code != http.StatusOK || !r.tmux.ran("new-session") {
			t.Errorf("%d %q, tmux %q", code, text, r.tmux.asked)
		}
	})
}

// fenceCase is a hire made the two ways, with what each started with.
type fenceCase struct {
	settings string   // the hire's settings file, its own names made plain
	guard    []string // what the guard was asked, likewise
}

// fenceHire makes a hire of roleID through hire (the desk's, or the route's) with the guard saying
// groups, on a deck of its own, and reads what it started with.
func fenceHire(t *testing.T, roleID string, groups []map[string]any, hire func(d *Deck, token string)) fenceCase {
	t.Helper()
	d, r, _, token := gmHireDeck(t)
	printed, _ := json.Marshal(map[string]any{"groups": groups, "file": "x", "problem": ""})
	var asked []string
	d.engine.Guard = func(tool string, args []string) (int, string, bool) {
		asked = append(asked, strings.Join(args, " "))
		return 0, string(printed), true
	}
	hire(d, token)
	d.watching.Wait()
	hires := registered(t)
	if len(hires) != 1 || hires[0].Role != roleID || !r.tmux.ran("new-session") {
		t.Fatalf("hire of %s: registry %+v, tmux %q", roleID, hires, r.tmux.asked)
	}
	data, err := os.ReadFile(filepath.Join(hireFolder(supportDir, hires[0].ID), hireSettingsFile))
	if err != nil {
		t.Fatal(err)
	}
	// Each deck has folders of its own: the names that differ are made plain.
	names := []string{hires[0].ID, "<hire>", hires[0].Worktree, "<worktree>", d.config.HireFolder, "<repo>", supportDir, "<support>", homeDir, "<home>"}
	plain := strings.NewReplacer(names...)
	for i := range asked {
		asked[i] = plain.Replace(asked[i])
	}
	return fenceCase{settings: plain.Replace(string(data)), guard: asked}
}

// The same fence: each case of hire-guard.json (the file as it is, then each group turned the other way),
// through a desk hire and through /setup/gm/hire, starts with the same guard and the same deny settings.
// This is the point of the series: the GM's hire is no looser than the office's.
func TestTheGMHireHasTheSameFenceAsAHireFromTheOffice(t *testing.T) {
	var file struct {
		Groups []map[string]any `json:"groups"`
	}
	if err := json.Unmarshal(frontDeskFixture(t, "hire-guard.json"), &file); err != nil || len(file.Groups) == 0 {
		t.Fatalf("hire-guard.json: %d groups: %v", len(file.Groups), err)
	}
	type guardCase struct {
		name   string
		groups []map[string]any
		turned string // the group turned the other way, "" for none
	}
	cases := []guardCase{{name: "as the file has them", groups: file.Groups}}
	for i, g := range file.Groups {
		turned := slices.Clone(file.Groups)
		flipped := cloneGroup(g)
		flipped["on"] = !(g["on"] == true)
		turned[i] = flipped
		cases = append(cases, guardCase{name: g["name"].(string) + " turned the other way", groups: turned, turned: g["name"].(string)})
	}
	for _, role := range []string{"janitor", "gm"} {
		for _, c := range cases {
			t.Run(role+", "+c.name, func(t *testing.T) {
				desk := fenceHire(t, role, c.groups, func(d *Deck, _ string) {
					if status, message := d.Perform("hire", "", url.Values{"role": {role}}); status != http.StatusNoContent {
						t.Fatalf("the desk's hire: %d %s", status, message)
					}
				})
				route := fenceHire(t, role, c.groups, func(d *Deck, token string) {
					// The GM's token hires every role but the gm one, which only the page hires.
					as := callAsGM
					if role == "gm" {
						as = callAsPage
					}
					if code, text := postGMHire(t, d, as, token, `{"role":"`+role+`"}`); code != http.StatusOK {
						t.Fatalf("the route's hire: %d %s", code, text)
					}
				})
				if desk.settings != route.settings {
					t.Errorf("settings differ:\ndesk:  %s\nroute: %s", desk.settings, route.settings)
				}
				if !reflect.DeepEqual(desk.guard, route.guard) {
					t.Errorf("the guard was asked differently:\ndesk:  %q\nroute: %q", desk.guard, route.guard)
				}
				if len(route.guard) == 0 || !strings.Contains(route.settings, "PreToolUse") || !strings.Contains(route.settings, "hire-guard.json") {
					t.Errorf("no guard in the route's hire: %q %s", route.guard, route.settings)
				}
				if (role == "gm") != strings.Contains(route.settings, "--profile gm") {
					t.Errorf("the %s has the wrong profile: %s", role, route.settings)
				}
				// Not vacuous: the deny rules follow the groups.
				if c.turned != "" {
					base := fenceHire(t, role, file.Groups, func(d *Deck, token string) {
						as := callAsGM
						if role == "gm" {
							as = callAsPage
						}
						postGMHire(t, d, as, token, `{"role":"`+role+`"}`)
					})
					if base.settings == route.settings {
						t.Errorf("turning %s the other way changed nothing in the deny settings", c.turned)
					}
				}
			})
		}
	}
}

func cloneGroup(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
