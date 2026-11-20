package desktop

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// GMEDIT1 (gonedesk.go): the GM's or the page's edit never marks a hire fired or failed while its session runs, or
// while that can't be told, whatever kind its session is; the fire is where it's ended.

// noDeskSession is a stand-in look at a hire's session: none runs.
func noDeskSession(Hire) deskLook { return deskLook{known: true} }

// sessionLook is a stand-in look that answers what, running and known for every hire, and counts its looks.
func sessionLook(what string, running, known bool, looks *int) func(Hire) deskLook {
	return func(Hire) deskLook { *looks++; return deskLook{what: what, running: running, known: known} }
}

func TestTheEditNeverMarksAHireGoneWhileItsSessionRunsOrCantBeTold(t *testing.T) {
	isolate(t)
	rows := []struct {
		name, engine, what string
		running, known     bool
		want               string // "" when the edit goes ahead
	}{
		{"tmux session running", "", "tmux session", true, true, "Ada's tmux session is still running, so it wasn't marked STATE: fire Ada, which ends it first."},
		{"launcher window running", "", "window", true, true, "Ada's window is still running, so it wasn't marked STATE: fire Ada, which ends it first."},
		{"T3 Code agent running", t3Engine, "T3 Code agent", true, true, "Ada's T3 Code agent is still running, so it wasn't marked STATE: let Ada go from Setup's team list, which ends it first."},
		{"tmux session unknown", "", "tmux session", false, false, "Smolcorp can't tell whether Ada's tmux session is still running, so it wasn't marked STATE. To stop it, fire Ada."},
		{"T3 Code agent unknown", t3Engine, "T3 Code agent", false, false, "Smolcorp can't tell whether Ada's T3 Code agent is still running, so it wasn't marked STATE. To stop it, let Ada go from Setup's team list."},
		{"tmux session ended", "", "tmux session", false, true, ""},
		{"T3 Code agent ended", t3Engine, "T3 Code agent", false, true, ""},
	}
	for _, row := range rows {
		for _, state := range []string{"fired", "failed"} {
			t.Run(row.name+" "+state, func(t *testing.T) {
				h := Hire{ID: "a1b2c3", Name: "Reviewer", Nickname: "Ada", Role: "reviewer", Folder: keptPath(homeDir), State: "working", Engine: row.engine}
				if err := updateRegistry(func(f *registryFile) { f.hires = []Hire{h} }); err != nil {
					t.Fatal(err)
				}
				looks := 0
				got, refused := editHire(registryPath(), []byte(`{"id":"a1b2c3","state":"`+state+`","notes":"x"}`), sessionLook(row.what, row.running, row.known, &looks))
				if looks != 1 {
					t.Errorf("the session was looked at %d times", looks)
				}
				now := readRegistry(registryPath(), false).hires[0]
				if row.want == "" {
					if refused != nil || got.State != state || now.State != state {
						t.Fatalf("refused %v, got %+v, registry %+v", refused, got, now)
					}
					return
				}
				want := strings.ReplaceAll(row.want, "STATE", state)
				if refused == nil || refused.status != http.StatusConflict || refused.message != want {
					t.Fatalf("refused %+v, want %q", refused, want)
				}
				if now.State != "working" || now.Notes != "" {
					t.Errorf("a refused edit changed the row: %+v", now)
				}
			})
		}
	}
}

// Only an open hire turned fired or failed is looked at: a nickname, notes, an open state, or a row already gone
// edit as they did, with a session running.
func TestTheEditLooksOnlyWhenAnOpenHireWouldBeMarkedGone(t *testing.T) {
	isolate(t)
	h := Hire{ID: "a1b2c3", Name: "Reviewer", Nickname: "Ada", Role: "reviewer", Folder: keptPath(homeDir), State: "working"}
	if err := updateRegistry(func(f *registryFile) { f.hires = []Hire{h} }); err != nil {
		t.Fatal(err)
	}
	looks := 0
	running := sessionLook("tmux session", true, true, &looks)
	for _, body := range []string{`{"id":"a1b2c3","nickname":"Bea"}`, `{"id":"a1b2c3","notes":"later"}`, `{"id":"a1b2c3","state":"working"}`,
		`{"id":"a1b2c3","state":"starting"}`, `{"id":"nobody","state":"fired"}`, `{"id":"a1b2c3","state":"gone"}`} {
		_, _ = editHire(registryPath(), []byte(body), running)
	}
	if err := updateRegistry(func(f *registryFile) { f.hires[0].State = "fired" }); err != nil {
		t.Fatal(err)
	}
	if _, refused := editHire(registryPath(), []byte(`{"id":"a1b2c3","state":"failed"}`), running); refused != nil {
		t.Errorf("a row already gone: %+v", refused)
	}
	if looks != 0 {
		t.Errorf("looked %d times", looks)
	}
}

// RT126GMEDIT1 B1 (a), every kind: while a hire's start is under way (marked from its row until its session can be
// looked at; a T3 hire's while it is hired), the edit refuses to mark it gone, and goes ahead once the start is over.
func TestTheEditRefusesWhileAStartIsUnderWayForEveryKind(t *testing.T) {
	d := newTestDeck(t)
	rows := []struct {
		name string
		hire Hire
		mark func(id string) func()
		how  string
	}{
		{"tmux", Hire{TmuxSocket: filepath.Join(t.TempDir(), "not-its-own")}, deskStarting, "fire Ada"},
		{"launcher", Hire{ViaLauncher: true}, deskStarting, "fire Ada"},
		{"T3", Hire{Engine: t3Engine}, func(id string) func() {
			t3InFlight.Lock()
			t3InFlight.ids[id] = true
			t3InFlight.Unlock()
			return func() { t3InFlight.Lock(); delete(t3InFlight.ids, id); t3InFlight.Unlock() }
		}, "let Ada go"},
	}
	for _, row := range rows {
		for _, state := range []string{"fired", "failed"} {
			t.Run(row.name+" "+state, func(t *testing.T) {
				h := row.hire
				h.ID, h.Name, h.Nickname, h.Role, h.Folder, h.State = "a1b2c3", "Reviewer", "Ada", "reviewer", keptPath(homeDir), "starting"
				if err := updateRegistry(func(f *registryFile) { f.hires = []Hire{h} }); err != nil {
					t.Fatal(err)
				}
				done := row.mark(h.ID)
				_, refused := editHire(registryPath(), []byte(`{"id":"a1b2c3","state":"`+state+`"}`), d.deskSession)
				done()
				want := "Ada is still starting, so it wasn't marked " + state + ": try again in a moment, or " + row.how + " once it's up."
				if refused == nil || refused.status != http.StatusConflict || refused.message != want {
					t.Fatalf("refused %+v, want %q", refused, want)
				}
				if now := readRegistry(registryPath(), false).hires[0]; now.State != "starting" {
					t.Fatalf("a refused edit changed the row: %+v", now)
				}
				// The start over (with nothing running), the edit goes ahead.
				if got, refused := editHire(registryPath(), []byte(`{"id":"a1b2c3","state":"`+state+`"}`), d.deskSession); refused != nil || got.State != state {
					t.Errorf("after the start: %+v", refused)
				}
			})
		}
	}
}

// The edit's look and its save aren't one step: a start under way by the save (as a start beginning between them) is
// refused again under the registry's lock, with the row unchanged.
func TestTheEditLooksAgainUnderTheRegistrysLockForAStartUnderWay(t *testing.T) {
	isolate(t)
	h := Hire{ID: "a1b2c3", Name: "Reviewer", Nickname: "Ada", Role: "reviewer", Folder: keptPath(homeDir), State: "working"}
	if err := updateRegistry(func(f *registryFile) { f.hires = []Hire{h} }); err != nil {
		t.Fatal(err)
	}
	var done func()
	startsMeanwhile := func(Hire) deskLook { done = deskStarting(h.ID); return deskLook{what: "tmux session", known: true} }
	_, refused := editHire(registryPath(), []byte(`{"id":"a1b2c3","state":"fired","notes":"x"}`), startsMeanwhile)
	done()
	if want := "Ada is still starting, so it wasn't marked fired: try again in a moment, or fire Ada once it's up."; refused == nil || refused.message != want {
		t.Fatalf("refused %+v", refused)
	}
	if now := readRegistry(registryPath(), false).hires[0]; now.State != "working" || now.Notes != "" {
		t.Errorf("a refused edit changed the row: %+v", now)
	}
}

// RT126GMEDIT1 C1 (VICTOR's call): a hire in a window of its own can't be told from the owner's own session in its
// worktree, so the edit never marks it gone: it says to close the window there, then fire it with force.
func TestTheEditNeverMarksAHireInAWindowOfItsOwnGone(t *testing.T) {
	d := newTestDeck(t)
	h := Hire{ID: "a1b2c3", Name: "Reviewer", Nickname: "Ada", Role: "reviewer", Folder: keptPath(homeDir), Worktree: keptPath(t.TempDir()), State: "working"}
	if err := updateRegistry(func(f *registryFile) { f.hires = []Hire{h} }); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"fired", "failed"} {
		_, refused := editHire(registryPath(), []byte(`{"id":"a1b2c3","state":"`+state+`"}`), d.deskSession)
		want := "Smolcorp can't tell whether Ada's window is still running, so it wasn't marked " + state + ". Close it there, then fire Ada with force."
		if refused == nil || refused.status != http.StatusConflict || refused.message != want {
			t.Errorf("%s: %+v", state, refused)
		}
	}
	if now := readRegistry(registryPath(), false).hires[0]; !isOpen(now) {
		t.Errorf("%+v", now)
	}
}
