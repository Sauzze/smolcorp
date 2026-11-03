//go:build !windows

package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

const (
	macGMWorking = "A General Manager is already working. Fire it first, then hire another."
	t722Already  = "This office already has a General Manager. The owner can replace it from Setup."
	t722Owner    = "Only the owner hires the General Manager, from Setup."
)

// gmDeck is a deck with a GM hired through the engine (state starting, its tmux session up).
func gmDeck(t *testing.T) (*Deck, *engineRig, Hire) {
	t.Helper()
	d, r, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	gm, err := runManagedHire(r.request("gm"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	return d, r, gm
}

func setGMState(t *testing.T, id, state string) {
	t.Helper()
	if err := updateRegistry(func(f *registryFile) {
		for i := range f.hires {
			if f.hires[i].ID == id {
				f.hires[i].State = state
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSetupRefusesAGMWhileOneIsWorkingAndFiresNothing(t *testing.T) {
	for _, state := range []string{"starting", "working", "idle"} {
		d, r, gm := gmDeck(t)
		setGMState(t, gm.ID, state)
		status, message := d.hireFromSetup("gm")
		if status != http.StatusConflict || message != macGMWorking {
			t.Fatalf("%s: %d %q", state, status, message)
		}
		got := registered(t)
		if len(got) != 1 || got[0].State != state || !r.tmux.up || r.tmux.ran("kill-session") || countVerb(r, "new-session") != 1 {
			t.Errorf("%s: something was fired or made: registry %+v, tmux %q", state, got, r.tmux.asked)
		}
	}
}

func countVerb(r *engineRig, verb string) (n int) {
	for _, a := range r.tmux.asked {
		if a[2] == verb {
			n++
		}
	}
	return
}

func TestSetupHiresAGMWhenTheOldOneIsFiredFailedOrItsSessionIsGone(t *testing.T) {
	for _, c := range []struct {
		name  string
		state string
		up    bool
	}{{"fired", "fired", true}, {"failed", "failed", true}, {"working, session gone", "working", false}, {"idle, session gone", "idle", false}} {
		d, r, gm := gmDeck(t)
		setGMState(t, gm.ID, c.state)
		r.tmux.up = c.up
		if status, message := d.hireFromSetup("gm"); status != http.StatusNoContent {
			t.Fatalf("%s: %d %q", c.name, status, message)
		}
		d.watching.Wait()
		if got := registered(t); len(got) != 2 || got[0].State != c.state || got[1].Role != "gm" {
			t.Errorf("%s: registry %+v", c.name, got)
		}
		if r.tmux.ran("kill-session") {
			t.Errorf("%s: something was fired: %q", c.name, r.tmux.asked)
		}
	}
}

func TestSetupHiresAGMWhenNoneIsThere(t *testing.T) {
	d, r, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	if status, message := d.hireFromSetup("gm"); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, message)
	}
	d.watching.Wait()
	if got := registered(t); len(got) != 1 || got[0].Role != "gm" || r.tmux.ran("kill-session") {
		t.Errorf("registry %+v, tmux %q", got, r.tmux.asked)
	}
}

func TestSetupRefusesAGMWhenItsSessionCantBeAsked(t *testing.T) {
	d, r, gm := gmDeck(t)
	setGMState(t, gm.ID, "working")
	engine := r.o.Tmux
	d.engine.Tmux = func(args []string) (string, int, bool) {
		if args[2] == "has-session" {
			return "", 0, false
		}
		return engine(args)
	}
	if status, message := d.hireFromSetup("gm"); status != http.StatusConflict || message != macGMWorking {
		t.Fatalf("%d %q", status, message)
	}
}

func TestTheTwoT722TextsAreChosenByWhetherAGMIsHired(t *testing.T) {
	for _, c := range []struct {
		name  string
		hires []Hire
		want  string
	}{
		{"working", []Hire{{Role: "gm", State: "working"}}, t722Already},
		{"starting", []Hire{{Role: "gm", State: "starting"}}, t722Already},
		{"idle", []Hire{{Role: "gm", State: "idle"}}, t722Already},
		{"none", nil, t722Owner},
		{"fired and failed", []Hire{{Role: "gm", State: "fired"}, {Role: "gm", State: "failed"}}, t722Owner},
		{"another role", []Hire{{Role: "janitor", State: "working"}}, t722Owner},
	} {
		status, text, refused := gmHireRefusal(gmRole, callerGM, registryFile{hires: c.hires}.gmIsHired())
		if !refused || status != http.StatusConflict || text != c.want {
			t.Errorf("%s: %d %q, want 409 %q", c.name, status, text, c.want)
		}
	}
}

func TestEveryHireRouteRefusesAGMWithTheMacTextWhileOneWorksAndHiresWhenNoneDoes(t *testing.T) {
	routes := map[string]func(d *Deck) (int, string){
		"act":      func(d *Deck) (int, string) { return d.Perform("hire", "", url.Values{"role": {"gm"}}) },
		"upstairs": func(d *Deck) (int, string) { return d.hireUpstairs("1", "0", "gm") },
	}
	for name, hire := range routes {
		d, r, gm := gmDeck(t)
		setGMState(t, gm.ID, "working")
		if status, message := hire(d); status != http.StatusConflict || message != macGMWorking {
			t.Fatalf("%s, a GM works: %d %q", name, status, message)
		}
		if len(registered(t)) != 1 || r.tmux.ran("kill-session") || countVerb(r, "new-session") != 1 {
			t.Errorf("%s: something was fired or made: %+v, %q", name, registered(t), r.tmux.asked)
		}
		setGMState(t, gm.ID, "fired")
		if status, message := hire(d); status != http.StatusNoContent {
			t.Fatalf("%s, none works: %d %q", name, status, message)
		}
		d.watching.Wait()
		if got := registered(t); name == "act" && (len(got) != 2 || got[1].Role != "gm") {
			t.Errorf("%s: registry %+v", name, got)
		}
	}
}

// sessionIn is a live session started in cwd.
func sessionIn(d *Deck, id, cwd string) {
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": cwd}, Claude, "tmux:"+id, "", 100)
}

func TestSetupRefusesAGMWhoseSessionIsGoneWhileASessionWorksInItsWorktree(t *testing.T) {
	d, r, gm := gmDeck(t)
	tree, other := t.TempDir(), t.TempDir()
	if err := updateRegistry(func(f *registryFile) { f.hires[0].State, f.hires[0].Worktree = "working", tree }); err != nil {
		t.Fatal(err)
	}
	r.tmux.up = false
	sessionIn(d, "elsewhere", other)
	if status, message := d.hireFromSetup("gm"); status != http.StatusNoContent {
		t.Fatalf("a session elsewhere blocked: %d %q", status, message)
	}
	d.watching.Wait()
	setGMState(t, gm.ID, "working")
	if err := updateRegistry(func(f *registryFile) { f.hires = f.hires[:1] }); err != nil {
		t.Fatal(err)
	}
	r.tmux.up = false // the first hire made a session of its own
	sessionIn(d, "inside", tree)
	if status, message := d.hireFromSetup("gm"); status != http.StatusConflict || message != macGMWorking {
		t.Fatalf("%d %q", status, message)
	}
}

func TestASessionInASubfolderOrASiblingOfAGMsFolderDoesntBlock(t *testing.T) {
	d, r, _ := gmDeck(t)
	folder := t.TempDir()
	sub, sibling := filepath.Join(folder, "sub"), filepath.Join(filepath.Dir(folder), filepath.Base(folder)+"-sibling")
	for _, dir := range []string{sub, sibling} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := updateRegistry(func(f *registryFile) {
		f.hires[0].State, f.hires[0].Folder, f.hires[0].Worktree = "working", folder, ""
	}); err != nil {
		t.Fatal(err)
	}
	r.tmux.up = false
	sessionIn(d, "sub", sub)
	sessionIn(d, "sibling", sibling)
	if status, message := d.hireFromSetup("gm"); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, message)
	}
	d.watching.Wait()
}

func TestSetupRefusesAGMStillStartingEvenWithNoSessionYet(t *testing.T) {
	d, r, gm := gmDeck(t)
	r.tmux.up = false // no session yet: only the state says it is starting
	setGMState(t, gm.ID, "starting")
	if status, message := d.hireFromSetup("gm"); status != http.StatusConflict || message != macGMWorking {
		t.Fatalf("%d %q", status, message)
	}
	if len(registered(t)) != 1 || countVerb(r, "new-session") != 1 {
		t.Errorf("a second GM was made: %+v", registered(t))
	}
}
