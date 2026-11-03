//go:build !windows

package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fire tests hire through the engine on a stand-in tmux, then show the
// hire's window as one of Tests/fixtures/panes (or can't be read at all).

// windowShows makes the stand-in tmux answer capture-pane with capture, or fail.
func windowShows(r *engineRig, capture string, readable bool) {
	engine := r.tmux.run
	r.o.Tmux = func(args []string) (string, int, bool) {
		if args[2] == "capture-pane" {
			r.tmux.asked = append(r.tmux.asked, args)
			if !readable {
				return "", 1, true
			}
			return capture, 0, true
		}
		return engine(args)
	}
}

func hiredForFire(t *testing.T, nickname string) (*Deck, *engineRig, Hire) {
	t.Helper()
	d, r, _ := engineDeck(t)
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if nickname != "" {
		if err := updateRegistry(func(f *registryFile) { f.hires[0].Nickname = nickname }); err != nil {
			t.Fatal(err)
		}
		hire.Nickname = nickname
	}
	return d, r, hire
}

func TestAFireRefusesAtAQuestionDraftBusyOrAnUnreadableWindowAndChangesNothing(t *testing.T) {
	cases := []struct {
		name, capture string
		readable      bool
		want          string
	}{
		{"permission menu", pane(t, "permission-menu"), true, "Rex is waiting for an answer in its window. Answer it there first; nothing was ended."},
		{"trust menu", pane(t, "trust-menu"), true, "Rex is waiting for an answer in its window. Answer it there first; nothing was ended."},
		{"draft", paneBox(paneAuto, "❯ half a thought"), true, "Rex has something typed in its window that isn't sent yet. Send or clear it, or fire with force."},
		{"busy", pane(t, "busy-1"), true, "Rex is working right now. Wait until it is done, or fire with force."},
		{"half drawn", "just some text", true, "Couldn't tell what Rex's window holds, so nothing was ended. Look at it, or fire with force."},
		{"unreadable", "", false, "Couldn't tell what Rex's window holds, so nothing was ended. Look at it first."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, r, hire := hiredForFire(t, "Rex")
			windowShows(r, c.capture, c.readable)
			_, err := fireEngineHire(hire, 0, r.o, false)
			if refused, ok := err.(*hireRefusal); !ok || refused.Status != http.StatusConflict || refused.Message != c.want {
				t.Fatalf("err = %v, want 409 %q", err, c.want)
			}
			if r.tmux.ran("kill-session") || !r.tmux.up {
				t.Error("the session was ended")
			}
			if _, err := os.Stat(hire.Worktree); err != nil || registered(t)[0].State != "starting" {
				t.Errorf("worktree or state changed: %v %+v", err, registered(t))
			}
			if strings.Contains(c.want, "janitor") || strings.Contains(c.want, "Janitor") {
				t.Error("the refusal names the role")
			}
		})
	}
}

func TestAFireRefusalNamesTheHireByNicknameElseNameNeverTheRole(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	windowShows(r, pane(t, "permission-menu"), true)
	_, err := fireEngineHire(hire, 0, r.o, false)
	if err == nil || !strings.HasPrefix(err.Error(), "Janitor is waiting") {
		t.Fatalf("err = %v, want the hire's name", err)
	}
	if calledHire(Hire{Name: "Janitor", Nickname: "Rex"}) != "Rex" {
		t.Error("a nickname isn't preferred")
	}
}

func TestAFireOfAnIdleHireEndsItOnItsOwnSocketAndRemovesItsCleanWorktree(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	windowShows(r, pane(t, "idle-empty"), true)
	kept, err := fireEngineHire(hire, 0, r.o, false)
	if err != nil || kept != "" {
		t.Fatalf("kept %q, err %v", kept, err)
	}
	for _, a := range r.tmux.asked {
		if a[0] != "-S" || a[1] != hire.TmuxSocket {
			t.Errorf("tmux was asked on another socket: %q", a)
		}
		if a[2] == "send-keys" || a[2] == "paste-buffer" {
			t.Errorf("something was typed: %q", a)
		}
	}
	if !r.tmux.ran("kill-session") || r.tmux.up {
		t.Error("the session wasn't ended")
	}
	if _, err := os.Stat(hire.Worktree); err == nil || branches(t, r.repo) != "" {
		t.Error("the clean worktree or its branch was left")
	}
	if got := registered(t); got[0].State != "fired" {
		t.Errorf("state = %q", got[0].State)
	}
}

func TestAForcedFireKeepsAWorktreeWithChangesAndSaysWhyInLowerCase(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	os.WriteFile(filepath.Join(hire.Worktree, "new.txt"), []byte("x"), 0o644)
	windowShows(r, pane(t, "idle-empty"), true)
	kept, err := fireEngineHire(hire, 0, r.o, true)
	if err != nil || kept != "it has changes, or git wouldn't remove it" {
		t.Fatalf("kept %q, err %v", kept, err)
	}
	if _, err := os.Stat(hire.Worktree); err != nil {
		t.Error("a worktree with changes was removed")
	}
	got := registered(t)[0]
	if got.State != "fired" || !strings.Contains(got.Notes, "was kept: it has") {
		t.Errorf("hire = %+v", got)
	}
}

func TestAFireNeverReachesASocketThatIsNotTheHiresOwn(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	windowShows(r, pane(t, "idle-empty"), true)
	hire.TmuxSocket = filepath.Join(t.TempDir(), "default")
	r.tmux.asked = nil
	_, err := fireEngineHire(hire, 0, r.o, false)
	if err == nil || err.Error() != "Couldn't reach Janitor's tmux session, so nothing was changed." || len(r.tmux.asked) != 0 {
		t.Fatalf("err = %v, tmux %q", err, r.tmux.asked)
	}
}

func TestAFireOfAHireAlreadyGoneIsRefused(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	setHire(hire.ID, "fired", "")
	windowShows(r, pane(t, "idle-empty"), true)
	if _, err := fireEngineHire(hire, 0, r.o, false); err == nil || err.Error() != "Janitor is already gone." {
		t.Fatalf("err = %v", err)
	}
}

func TestTheActFireRouteFiresAnEngineHireThroughTheRefusals(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	tab := "tmux:%1@" + hire.TmuxSocket
	d.config.HiringEnabled = true
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s1", "cwd": hire.Worktree},
		Claude, tab, "", 200)
	windowShows(r, pane(t, "permission-menu"), true)
	d.engine = r.o
	if status, message := d.Perform("fire", "s1", url.Values{}); status != http.StatusConflict || !strings.HasPrefix(message, "Rex is waiting") {
		t.Fatalf("at a menu: %d %q", status, message)
	}
	if r.tmux.ran("kill-session") {
		t.Error("the session was ended at a menu")
	}
	windowShows(r, pane(t, "idle-empty"), true)
	d.engine = r.o
	if status, message := d.Perform("fire", "s1", url.Values{}); status != http.StatusNoContent {
		t.Fatalf("idle: %d %q", status, message)
	}
	if !r.tmux.ran("kill-session") || registered(t)[0].State != "fired" {
		t.Error("the idle hire wasn't fired")
	}
	if _, _, handled := d.fireTab("tmux:%1@"+filepath.Join(t.TempDir(), "other"), false); handled {
		t.Error("a tab that isn't an engine hire's was taken")
	}
}

// failing makes the stand-in tmux answer verb with a failure that ran.
func failing(r *engineRig, verb string) {
	engine := r.o.Tmux
	r.o.Tmux = func(args []string) (string, int, bool) {
		if args[2] == verb {
			r.tmux.asked = append(r.tmux.asked, args)
			return "", 1, true
		}
		return engine(args)
	}
}

func TestAFailedKillIsNeverAFire(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	windowShows(r, pane(t, "idle-empty"), true)
	failing(r, "kill-session")
	_, err := fireEngineHire(hire, 0, r.o, false)
	if err == nil || err.Error() != "Couldn't end Janitor's tmux session, so nothing was changed." {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(hire.Worktree); err != nil || registered(t)[0].State != "starting" {
		t.Errorf("worktree or state changed: %v %+v", err, registered(t))
	}
	if _, err := os.Stat(filepath.Join(supportDir, "recaps")); err == nil {
		t.Error("a recap was written")
	}
}

func TestAFireStopsAtUncommittedChangesBeforeAnythingIsEndedUnlessForced(t *testing.T) {
	_, r, hire := hiredForFire(t, "Rex")
	os.WriteFile(filepath.Join(hire.Worktree, "new.txt"), []byte("x"), 0o644)
	windowShows(r, pane(t, "idle-empty"), true)
	want := "Rex has changes that aren't committed: ?? new.txt. Commit them, or fire with force."
	if _, err := fireEngineHire(hire, 0, r.o, false); err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if r.tmux.ran("kill-session") || !r.tmux.up || registered(t)[0].State != "starting" {
		t.Error("something was ended or changed")
	}
	// Git unable to say is a refusal too.
	cant := r.o
	cant.Git = func(string, ...string) (int, string, bool) { return 0, "", false }
	want = "Couldn't tell whether Rex's worktree has unsaved work. Fire again with force to go ahead."
	if _, err := fireEngineHire(hire, 0, cant, false); err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if _, err := fireEngineHire(hire, 0, r.o, true); err != nil || !r.tmux.ran("kill-session") {
		t.Fatalf("forced: %v", err)
	}
}

func TestAnEngineHireIsKnownByItsRowWhateverWayItsTabSpellsTheSocket(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	link := filepath.Join(t.TempDir(), "support")
	if err := os.Symlink(supportDir, link); err != nil {
		t.Fatal(err)
	}
	spelt := filepath.Join(link, filepath.Base(hire.TmuxSocket))
	if got, ok := engineHireAt(spelt); !ok || got.ID != hire.ID {
		t.Fatalf("engineHireAt(%q) = %v %v", spelt, got.ID, ok)
	}
	windowShows(r, pane(t, "permission-menu"), true)
	d.engine = r.o
	d.config.HiringEnabled = true
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s1", "cwd": hire.Worktree},
		Claude, "tmux:%1@"+spelt, "", 200)
	if status, message := d.Perform("fire", "s1", url.Values{}); status != http.StatusConflict || !strings.HasPrefix(message, "Rex is waiting") {
		t.Fatalf("fire by a spelling of the socket: %d %q", status, message)
	}
	if _, ok := engineHireAt(filepath.Join(t.TempDir(), "x")); ok {
		t.Error("another socket was taken")
	}
}

func TestAFireOfAHireBeingFiredAlreadyIsRefused(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	windowShows(r, pane(t, "idle-empty"), true)
	firing.Lock()
	firing.ids[hire.ID] = true
	firing.Unlock()
	defer func() { firing.Lock(); delete(firing.ids, hire.ID); firing.Unlock() }()
	if _, err := fireEngineHire(hire, 0, r.o, false); err == nil || err.Error() != "Janitor is being fired already." || r.tmux.ran("kill-session") {
		t.Fatalf("err = %v", err)
	}
}

func TestAFireWithTmuxThatCantBeAskedChangesNothing(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	r.o.Tmux = func([]string) (string, int, bool) { return "", -1, false }
	if _, err := fireEngineHire(hire, 0, r.o, false); err == nil || err.Error() != "Couldn't reach Janitor's tmux session, so nothing was changed." {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(hire.Worktree); err != nil || registered(t)[0].State != "starting" || !r.tmux.up {
		t.Error("something changed")
	}
}

func TestAFireLooksAgainJustBeforeEndingAndStopsAtAQuestionThatCameUpMeanwhile(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	screen := pane(t, "idle-empty")
	engine := r.o.Tmux
	r.o.Tmux = func(args []string) (string, int, bool) {
		if args[2] == "capture-pane" {
			return screen, 0, true
		}
		return engine(args)
	}
	// The question comes up while git is looking at the worktree.
	r.o.Git = func(dir string, args ...string) (int, string, bool) {
		status, out, ok := runGit(dir, args...)
		screen = pane(t, "permission-menu")
		return status, out, ok
	}
	if _, err := fireEngineHire(hire, 0, r.o, false); err == nil || !strings.HasPrefix(err.Error(), "Janitor is waiting for an answer") {
		t.Fatalf("err = %v", err)
	}
	if r.tmux.ran("kill-session") || !r.tmux.up {
		t.Error("the session was ended over a question")
	}
}

func TestForceSkipsOnlyWhatTheMacsForceSkipsAndNothingIsEverTyped(t *testing.T) {
	for _, c := range []struct {
		name, capture string
		readable      bool
		fires         bool
	}{
		{"busy", pane(t, "busy-1"), true, true},
		{"draft", paneBox(paneAuto, "❯ half a thought"), true, true},
		{"half drawn", "just some text", true, true},
		{"menu", pane(t, "permission-menu"), true, false},
		{"unreadable", "", false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, r, hire := hiredForFire(t, "")
			windowShows(r, c.capture, c.readable)
			_, err := fireEngineHire(hire, 0, r.o, true)
			if (err == nil) != c.fires || r.tmux.ran("kill-session") != c.fires {
				t.Fatalf("err = %v, session ended %v", err, r.tmux.ran("kill-session"))
			}
			if r.tmux.ran("send-keys") || r.tmux.ran("paste-buffer") {
				t.Error("something was typed")
			}
		})
	}
}

func TestAFireWritesTheRecapAndAnUnreadableOneSaysSo(t *testing.T) {
	_, r, hire := hiredForFire(t, "Rex")
	own := hireFolder(supportDir, hire.ID)
	os.WriteFile(filepath.Join(own, "BRIEF.md"), []byte("# Review the login\n\nmore"), 0o644)
	os.WriteFile(filepath.Join(own, "STATUS.md"), []byte("one\ntwo\nthree: tests pass\n"), 0o644)
	windowShows(r, "Total cost: $0.42\n"+pane(t, "idle-empty"), true)
	if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(supportDir, "recaps", hire.ID+".md")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"# Recap: Rex", "Who: Janitor (janitor), called Rex", "Task: Review the login", "Branch: " + hire.Branch,
		"(removed)", "Cost: $0.42", "- three: tests pass"} {
		if !strings.Contains(string(data), part) {
			t.Errorf("recap lacks %q:\n%s", part, data)
		}
	}
	if info, _ := os.Stat(file); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", info.Mode().Perm())
	}
	// A hire with no brief, status or cost.
	_, r, hire = hiredForFire(t, "")
	windowShows(r, pane(t, "idle-empty"), true)
	if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(supportDir, "recaps", hire.ID+".md"))
	for _, part := range []string{"Task: not written down", "Cost: not shown", "Nothing was written to STATUS.md."} {
		if !strings.Contains(string(data), part) {
			t.Errorf("recap lacks %q:\n%s", part, data)
		}
	}
}

func TestAFireMarksAHireFiredWhetherItIsStartingOrWorkingFreesItsPortsAndCantBeRepeated(t *testing.T) {
	for _, from := range []string{"starting", "working"} {
		t.Run(from, func(t *testing.T) {
			_, r, hire := hiredForFire(t, "")
			if from == "working" {
				setHire(hire.ID, "working", "")
			}
			if got := registered(t)[0].State; got != from {
				t.Fatalf("state before = %q", got)
			}
			if len(takenPorts()) != 1 {
				t.Fatalf("ports taken before: %v", takenPorts())
			}
			windowShows(r, pane(t, "idle-empty"), true)
			if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
				t.Fatal(err)
			}
			if got := registered(t)[0].State; got != "fired" {
				t.Errorf("state = %q, want fired", got)
			}
			if len(takenPorts()) != 0 {
				t.Errorf("ports still taken: %v", takenPorts())
			}
			if _, err := fireEngineHire(hire, 0, r.o, false); err == nil || err.Error() != "Janitor is already gone." {
				t.Errorf("second fire: %v", err)
			}
		})
	}
}

func TestAFireKeepsAWorktreeThatIsNotTheHiresOwnAndStillMarksTheHireFired(t *testing.T) {
	for _, c := range []struct {
		name   string
		change func(h *Hire, o *engineOptions)
	}{
		{"on another branch", func(h *Hire, o *engineOptions) { h.Branch = "hire/someone-else" }},
		{"outside the hires folder", func(h *Hire, o *engineOptions) { o.Worktrees = t.TempDir() }},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, r, hire := hiredForFire(t, "")
			setHire(hire.ID, "working", "")
			windowShows(r, pane(t, "idle-empty"), true)
			c.change(&hire, &r.o)
			kept, err := fireEngineHire(hire, 0, r.o, false)
			if err != nil || kept == "" {
				t.Fatalf("kept %q, err %v", kept, err)
			}
			if _, err := os.Stat(hire.Worktree); err != nil {
				t.Error("a worktree that isn't the hire's own was removed")
			}
			if got := registered(t)[0]; got.State != "fired" || !strings.Contains(got.Notes, "was kept") {
				t.Errorf("hire = %+v", got)
			}
		})
	}
}

func TestTheActFireRouteTakesForceOnlyWhenItIsOne(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	tab := "tmux:%1@" + hire.TmuxSocket
	d.config.HiringEnabled = true
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s1", "cwd": hire.Worktree},
		Claude, tab, "", 200)
	windowShows(r, pane(t, "busy-1"), true)
	d.engine = r.o
	if status, message := d.Perform("fire", "s1", url.Values{}); status != http.StatusConflict || !strings.HasPrefix(message, "Rex is working") || r.tmux.ran("kill-session") {
		t.Fatalf("without force: %d %q", status, message)
	}
	if status, message := d.Perform("fire", "s1", url.Values{"force": {"0"}}); status != http.StatusConflict {
		t.Fatalf("force=0: %d %q", status, message)
	}
	if status, message := d.Perform("fire", "s1", url.Values{"force": {"1"}}); status != http.StatusNoContent {
		t.Fatalf("force=1: %d %q", status, message)
	}
	if !r.tmux.ran("kill-session") || registered(t)[0].State != "fired" {
		t.Error("the forced fire didn't end the hire")
	}
}

func TestAFireOfAHireWhoseSessionIsGoneButSocketFileIsThereEndsNothingAndMarksItFired(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	setHire(hire.ID, "working", "")
	r.tmux.up = false
	r.tmux.asked = nil
	if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
		t.Fatal(err)
	}
	if r.tmux.ran("kill-session") || r.tmux.ran("capture-pane") {
		t.Errorf("a session that is gone was looked at or ended: %q", r.tmux.asked)
	}
	if got := registered(t)[0].State; got != "fired" {
		t.Errorf("state = %q", got)
	}
	if _, err := os.Stat(hire.Worktree); err == nil {
		t.Error("the clean worktree was left")
	}
}

func TestAnEngineHireIsFoundByAHireSocketNameWhenItsSocketFileHasGoneButNotByAnyOtherName(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	if err := os.Remove(hire.TmuxSocket); err != nil {
		t.Fatal(err)
	}
	if got, ok := engineHireAt(hire.TmuxSocket); !ok || got.ID != hire.ID {
		t.Fatalf("a gone socket named like a hire's: %v %v", got.ID, ok)
	}
	d.config.HiringEnabled = true
	d.engine = r.o
	status, message, handled := d.fireTab("tmux:%1@"+hire.TmuxSocket, false)
	if !handled || status != http.StatusConflict || message != "Couldn't reach Rex's tmux session, so nothing was changed." {
		t.Fatalf("fireTab: %d %q %v", status, message, handled)
	}
	if r.tmux.ran("kill-session") {
		t.Error("something was ended")
	}
	// A row whose socket is neither the hire's own nor named like one isn't taken.
	other := filepath.Join(supportDir, "default")
	if err := updateRegistry(func(f *registryFile) { f.hires[0].TmuxSocket = other }); err != nil {
		t.Fatal(err)
	}
	if _, ok := engineHireAt(other); ok {
		t.Error("a row with some other socket was taken for an engine hire")
	}
}

func TestAFireThroughTheRoutePrunesTheHiresSessionsAndKeepsOthers(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	tab := "tmux:%1@" + hire.TmuxSocket
	d.config.HiringEnabled = true
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s1", "cwd": hire.Worktree}, Claude, tab, "", 200)
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s2", "cwd": hire.Folder}, Claude, "tmux:%9@/x/other", "", 200)
	windowShows(r, pane(t, "idle-empty"), true)
	d.engine = r.o
	if status, message := d.Perform("fire", "s1", url.Values{}); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, message)
	}
	var left []string
	for _, s := range d.sessions.All() {
		left = append(left, s.ID)
	}
	if len(left) != 1 || left[0] != "s2" {
		t.Errorf("sessions left: %v", left)
	}
}

// firedWith fires a hire shown as idle and reads its recap.
func firedWith(t *testing.T, r *engineRig, hire Hire, capture string, force bool) string {
	t.Helper()
	windowShows(r, capture, true)
	if _, err := fireEngineHire(hire, 0, r.o, force); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(supportDir, "recaps", hire.ID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestTheRecapTakesTheLastCostTheLastFiveStatusLinesFromTeamStatusAndTheHiredLine(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	team := filepath.Join(hireFolder(supportDir, hire.ID), ".team")
	os.MkdirAll(team, 0o755)
	os.WriteFile(filepath.Join(team, "STATUS.md"), []byte("l1\nl2\nl3\nl4\nl5\nl6\nl7\n"), 0o644)
	recap := firedWith(t, r, hire, "Total cost: $0.10\nmore\nTotal cost: $2.50\n"+pane(t, "idle-empty"), false)
	for _, part := range []string{"Cost: $2.50", "- l3", "- l7", "- Hired: "} {
		if !strings.Contains(recap, part) {
			t.Errorf("recap lacks %q:\n%s", part, recap)
		}
	}
	for _, part := range []string{"$0.10", "- l2", "- l1"} {
		if strings.Contains(recap, part) {
			t.Errorf("recap holds %q:\n%s", part, recap)
		}
	}
}

func TestTheRecapOfAKeptWorktreeSaysKept(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	os.WriteFile(filepath.Join(hire.Worktree, "new.txt"), []byte("x"), 0o644)
	if recap := firedWith(t, r, hire, pane(t, "idle-empty"), true); !strings.Contains(recap, "(kept)") || strings.Contains(recap, "(removed)") {
		t.Errorf("recap:\n%s", recap)
	}
}

func TestAnUncommittedRefusalListsTenFilesAndSaysHowManyMore(t *testing.T) {
	_, r, hire := hiredForFire(t, "Rex")
	for i := 0; i < 12; i++ {
		os.WriteFile(filepath.Join(hire.Worktree, "f"+strings.Repeat("x", i)+".txt"), []byte("x"), 0o644)
	}
	windowShows(r, pane(t, "idle-empty"), true)
	_, err := fireEngineHire(hire, 0, r.o, false)
	if err == nil || !strings.Contains(err.Error(), ", and 2 more. Commit them") {
		t.Fatalf("err = %v", err)
	}
	if strings.Count(err.Error(), ".txt") != 10 {
		t.Errorf("files shown: %v", err)
	}
}

func TestARecapOfAHireWithANonPlainIdIsNamedBySlugOfItsId(t *testing.T) {
	shortSupport(t)
	folder := t.TempDir()
	h := Hire{ID: "Odd ID/..x", Name: "Odd", Folder: folder}
	file := writeRecap(h, "", false, time.Unix(0, 0))
	if want := filepath.Join(supportDir, "recaps", hireSlug(h.ID)+".md"); file != want || hireSlug(h.ID) == h.ID {
		t.Fatalf("file %q, want %q", file, want)
	}
	if _, err := os.Stat(file); err != nil {
		t.Error(err)
	}
}

// setNotes writes notes into the hire's registry row, as the owner editing them.
func setNotes(t *testing.T, notes string) {
	t.Helper()
	if err := updateRegistry(func(f *registryFile) { f.hires[0].Notes = notes }); err != nil {
		t.Fatal(err)
	}
}

// editingNotesOnKill makes the stand-in tmux rewrite the hire's notes when the
// session is ended, so they change after the fire began.
func editingNotesOnKill(t *testing.T, r *engineRig, notes string) {
	t.Helper()
	engine := r.o.Tmux
	r.o.Tmux = func(args []string) (string, int, bool) {
		if args[2] == "kill-session" {
			setNotes(t, notes)
		}
		return engine(args)
	}
}

func TestACleanFireLeavesTheHiresNotesAsTheyAreInTheRegistry(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	setNotes(t, "owner wrote this")
	windowShows(r, pane(t, "idle-empty"), true)
	// hire is the copy read before the owner wrote: a fire must not write it back.
	if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
		t.Fatal(err)
	}
	if got := registered(t)[0]; got.State != "fired" || got.Notes != "owner wrote this" {
		t.Errorf("hire = %+v", got)
	}
}

func TestAKeptFirePutsTheKeptLineBeforeTheNotesAndNeverReplacesThem(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	setNotes(t, "owner wrote this")
	os.WriteFile(filepath.Join(hire.Worktree, "new.txt"), []byte("x"), 0o644)
	windowShows(r, pane(t, "idle-empty"), true)
	if _, err := fireEngineHire(hire, 0, r.o, true); err != nil {
		t.Fatal(err)
	}
	want := "Its worktree " + hire.Worktree + " was kept: it has changes, or git wouldn't remove it.\nowner wrote this"
	if got := registered(t)[0]; got.State != "fired" || got.Notes != want {
		t.Errorf("notes = %q, want %q", got.Notes, want)
	}
}

func TestNotesEditedAfterTheFireBeganAreKeptCleanOrKept(t *testing.T) {
	for _, kept := range []bool{false, true} {
		name := map[bool]string{false: "clean", true: "kept"}[kept]
		t.Run(name, func(t *testing.T) {
			_, r, hire := hiredForFire(t, "")
			setNotes(t, "before")
			if kept {
				os.WriteFile(filepath.Join(hire.Worktree, "new.txt"), []byte("x"), 0o644)
			}
			windowShows(r, pane(t, "idle-empty"), true)
			editingNotesOnKill(t, r, "edited meanwhile")
			hire.Notes = "before"
			if _, err := fireEngineHire(hire, 0, r.o, kept); err != nil {
				t.Fatal(err)
			}
			got := registered(t)[0].Notes
			if !strings.HasSuffix(got, "edited meanwhile") || strings.Contains(got, "before") || (!kept && got != "edited meanwhile") {
				t.Errorf("notes = %q", got)
			}
		})
	}
}

func TestAKeptFireWithNoNotesLeavesJustTheKeptLine(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	os.WriteFile(filepath.Join(hire.Worktree, "new.txt"), []byte("x"), 0o644)
	windowShows(r, pane(t, "idle-empty"), true)
	if _, err := fireEngineHire(hire, 0, r.o, true); err != nil {
		t.Fatal(err)
	}
	want := "Its worktree " + hire.Worktree + " was kept: it has changes, or git wouldn't remove it."
	if got := registered(t)[0].Notes; got != want {
		t.Errorf("notes = %q, want %q", got, want)
	}
}

func TestAFireWithTmuxThatCantBeAskedAfterTheKillRefusesAndChangesNothing(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	windowShows(r, pane(t, "idle-empty"), true)
	engine := r.o.Tmux
	killed := false
	r.o.Tmux = func(args []string) (string, int, bool) {
		if args[2] == "has-session" && killed {
			return "", -1, false
		}
		if args[2] == "kill-session" {
			killed = true
		}
		return engine(args)
	}
	_, err := fireEngineHire(hire, 0, r.o, false)
	if err == nil || err.Error() != "Couldn't end Janitor's tmux session, so nothing was changed." {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(hire.Worktree); err != nil || registered(t)[0].State != "starting" {
		t.Errorf("worktree or state changed: %v %+v", err, registered(t))
	}
	if _, err := os.Stat(filepath.Join(supportDir, "recaps")); err == nil {
		t.Error("a recap was written")
	}
}

func TestAGitStatusThatFailsWithNoOutputRefusesTheFire(t *testing.T) {
	_, r, hire := hiredForFire(t, "Rex")
	windowShows(r, pane(t, "idle-empty"), true)
	cant := r.o
	cant.Git = func(string, ...string) (int, string, bool) { return 128, "", true }
	want := "Couldn't tell whether Rex's worktree has unsaved work. Fire again with force to go ahead."
	if _, err := fireEngineHire(hire, 0, cant, false); err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if r.tmux.ran("kill-session") || registered(t)[0].State != "starting" {
		t.Error("something was ended or changed")
	}
}

func TestAFailedHireIsAnsweredAsAlreadyGoneLikeAFiredOne(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	setHire(hire.ID, "failed", "it never started")
	windowShows(r, pane(t, "idle-empty"), true)
	if _, err := fireEngineHire(hire, 0, r.o, false); err == nil || err.Error() != "Janitor is already gone." {
		t.Fatalf("err = %v", err)
	}
	if r.tmux.ran("kill-session") || registered(t)[0].State != "failed" {
		t.Error("something was ended or changed")
	}
}

func TestTheRecapReadsStatusFromAKeptWorktreeButFromTheFolderWhenItIsRemoved(t *testing.T) {
	shortSupport(t)
	folder, worktree := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(folder, "STATUS.md"), []byte("folder line\n"), 0o644)
	os.WriteFile(filepath.Join(worktree, "STATUS.md"), []byte("kept line\n"), 0o644)
	h := Hire{ID: "abc123", Name: "Janitor", Folder: folder, Worktree: worktree}
	read := func(removed bool) string {
		data, err := os.ReadFile(writeRecap(h, "", removed, time.Unix(0, 0)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if kept := read(false); !strings.Contains(kept, "- kept line") || strings.Contains(kept, "folder line") {
		t.Errorf("kept recap:\n%s", kept)
	}
	if gone := read(true); !strings.Contains(gone, "- folder line") || strings.Contains(gone, "kept line") {
		t.Errorf("removed recap:\n%s", gone)
	}
}

func TestATabThatIsNotTmuxIsNeverFiredAsAnEngineHireWhateverItsSocket(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	d.config.HiringEnabled = true
	d.engine = r.o
	windowShows(r, pane(t, "idle-empty"), true)
	if status, message, handled := d.fireTab("kitty:1@"+hire.TmuxSocket, false); handled {
		t.Fatalf("handled: %d %q", status, message)
	}
	if r.tmux.ran("kill-session") || registered(t)[0].State != "starting" {
		t.Error("something was ended or changed")
	}
}
