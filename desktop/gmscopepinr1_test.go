//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXGMSCOPEPINR1 (RT126GMSCOPEPIN B1, B2): a scoped GM's night job is held to its scope where its hire is made, and
// once a hire folder was checked, git is never driven by its path again: the hire's worktree is made from the
// project. The rt* helpers and the TestRT* rows are the cold reviewer's (RT126GMSCOPEPIN's scratch rtpin_test.go):
// the swap of the last window is made before the hire's `worktree add` wherever git is asked to make it, and the
// reviewer's hook rows assert, as guards, with an fsmonitor beside the hook.

// rtAt runs swap just before the first engine git whose first arg is at, in folder ("" for any folder).
func rtAt(t *testing.T, d *Deck, folder, at string, swap func()) *bool {
	t.Helper()
	git, done := d.engine.Git, new(bool)
	if git == nil {
		git = runGit
	}
	d.engine.Git = func(dir string, args ...string) (int, string, bool) {
		if !*done && (folder == "" || dir == folder) && len(args) > 0 && args[0] == at {
			*done = true
			swap()
		}
		return git(dir, args...)
	}
	return done
}

// rtInOther is what of a hire's is in other: a branch, a worktree beyond its own, a non-GM row there.
func rtInOther(t *testing.T, other string) string {
	t.Helper()
	branches := strings.TrimSpace(gitOut(t, other, "branch", "--list", "hire/*"))
	trees := gitOut(t, other, "worktree", "list", "--porcelain")
	var rows []string
	for _, h := range registered(t) {
		if h.Role != gmRole && (strings.HasPrefix(h.Repo, other) || strings.HasPrefix(h.Folder, other)) {
			rows = append(rows, h.Repo+" "+h.Worktree)
		}
	}
	if branches == "" && strings.Count(trees, "worktree ") == 1 && len(rows) == 0 {
		return ""
	}
	return "branches " + branches + " | trees " + trees + " | rows " + strings.Join(rows, ",")
}

func rtHire(t *testing.T, d *Deck, token, folder string) (int, string) {
	body, _ := json.Marshal(map[string]any{"role": "janitor", "folder": folder})
	return scopedCall(t, d, token, "POST", "/setup/gm/hire", string(body))
}

func rtMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// rtRuns gives repo a post-checkout hook and an fsmonitor that only touch a marker each in the test's temp folder,
// and says which of them ran.
func rtRuns(t *testing.T, repo string) func() string {
	t.Helper()
	dir := t.TempDir()
	hook, monitor := filepath.Join(dir, "hook-ran"), filepath.Join(dir, "fsmonitor-ran")
	rtMust(t, os.WriteFile(filepath.Join(repo, ".git", "hooks", "post-checkout"), []byte("#!/bin/sh\ntouch '"+hook+"'\n"), 0o755))
	script := filepath.Join(dir, "fsmonitor")
	rtMust(t, os.WriteFile(script, []byte("#!/bin/sh\ntouch '"+monitor+"'\nexit 1\n"), 0o755))
	gitOut(t, repo, "config", "core.fsmonitor", script)
	return func() string {
		var ran []string
		for name, marker := range map[string]string{"post-checkout hook": hook, "core.fsmonitor": monitor} {
			if _, err := os.Stat(marker); err == nil {
				ran = append(ran, name)
			}
		}
		return strings.Join(ran, ", ")
	}
}

// rtBeside is a worktree of the project beside it, on a branch of its own, links followed.
func rtBeside(t *testing.T, project string) string {
	t.Helper()
	w := filepath.Join(homeDir, "beside")
	gitOut(t, project, "worktree", "add", "-q", "-b", "feature", w)
	return physical(w)
}

// rtSwap swaps folder for a link to other.
func rtSwap(t *testing.T, folder, other string) func() {
	return func() {
		rtMust(t, os.Rename(folder, folder+".away"))
		rtMust(t, os.Symlink(other, folder))
	}
}

// B1: a scoped GM's night job in a worktree of the project, swapped for a link to another project after the night's
// own re-check (nightStartRefusal), as the queue hires it (hireQueueRole, as nightDue's queue calls it).
func TestRTNightJobSwappedAfterItsRecheck(t *testing.T) {
	turnOn(t, &nightShiftOn)
	d, _, _, _, project := scopedGM(t)
	other := placeRepo(t)
	w := rtBeside(t, project)
	gm := gmNow()
	j := keepNightJob(t, d, NightJob{ID: "n1", Role: "janitor", Folder: w, Status: nightApproved, ProposedBy: "gm",
		Scope: gm.Scope, ScopeRoles: gm.ScopeRoles, Account: gm.Account})
	if why := d.nightStartRefusal(j); why != "" {
		t.Fatalf("the night's re-check refused: %q", why)
	}
	rtAt(t, d, w, "rev-parse", rtSwap(t, w, other))
	code, text := d.hireQueueRole(j.nightStep())
	d.watching.Wait()
	if got := rtInOther(t, other); got != "" {
		t.Fatalf("the night job's hire (%d %q) was made in the other project: %s", code, text, got)
	}
	if code/100 == 2 {
		t.Fatalf("the night job's hire was answered %d %q", code, text)
	}
	t.Logf("%d %q", code, text)
}

// B1, the same through the night shift itself: nightDue re-checks, then its queue hires.
func TestRTNightJobSwappedAfterItsRecheckThroughNightDue(t *testing.T) {
	turnOn(t, &nightShiftOn)
	d, _, _, _, project := scopedGM(t)
	other := placeRepo(t)
	w := rtBeside(t, project)
	gm := gmNow()
	j := keepNightJob(t, d, NightJob{ID: "n1", Role: "janitor", Folder: w, Status: nightApproved, ProposedBy: "gm",
		Scope: gm.Scope, ScopeRoles: gm.ScopeRoles, Account: gm.Account})
	rtAt(t, d, w, "rev-parse", rtSwap(t, w, other))
	managerOn(d)
	j, _ = d.night.get("n1")
	d.nightDue(nightDue{Job: j, Actor: j.actor()}, func() {})
	waitNight(t, d)
	d.watching.Wait()
	j, _ = d.night.get("n1")
	if got := rtInOther(t, other); got != "" {
		t.Fatalf("the night job's hire (%+v) was made in the other project: %s", j.Last, got)
	}
	if j.Last == nil || j.Last.Hired {
		t.Fatalf("the night job's run: %+v", j.Last)
	}
	t.Logf("%+v", j.Last)
}

// Kept as a guard (N1): prepareHire's `git worktree add` runs neither the repo's post-checkout hook nor its
// fsmonitor as the core (ownerGit, #95). Harmless stand-ins: each only touches a marker in the test's temp folder.
func TestRTPrepareHireRunsTheReposHook(t *testing.T) {
	repo := placeRepo(t)
	ran := rtRuns(t, repo)
	_, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "janitor", Folder: repo, Worktree: true}, placeOptions(t))
	t.Logf("prepareHire err=%v", err)
	if got := ran(); got != "" {
		t.Fatalf("the repo's %s ran as the core", got)
	}
}

// Kept as a guard (B2): a worktree of the project swapped, just before the hire's `worktree add`, for a link to a repo
// whose post-checkout hook and fsmonitor only touch a marker: neither runs as the core, and nothing is left there.
func TestRTLastWindowRunsTheOtherReposGit(t *testing.T) {
	d, _, _, token, project := scopedGM(t)
	other := placeRepo(t)
	ran := rtRuns(t, other)
	w := rtBeside(t, project)
	swapped := rtAt(t, d, "", "worktree", rtSwap(t, w, other))
	code, text := rtHire(t, d, token, w)
	got := ran()
	t.Logf("hire: %d %q; swapped %v; left in other: %q", code, text, *swapped, rtInOther(t, other))
	if got != "" {
		t.Fatalf("the other repo's %s ran as the core during the scoped GM's hire", got)
	}
}

// B2: once a hire folder was checked, its worktree is never made in a repo outside the scope, not even for a moment:
// git is asked to make it from the project, whatever the folder leads to by then. A folder swapped meanwhile still
// gets no hire: the worktree is taken away again, from the project, and nothing of the hire's is left anywhere.
func TestAScopedGMsHiresWorktreeIsNeverMadeInAnotherRepo(t *testing.T) {
	d, _, _, token, project := scopedGM(t)
	other := placeRepo(t)
	w := rtBeside(t, project)
	// Swapped just before the hire's worktree is made, wherever git is asked to make it, and looked at as soon as git
	// has made it, before anything can take it away again.
	swapped, seen := false, ""
	git := d.engine.Git
	if git == nil {
		git = runGit
	}
	d.engine.Git = func(dir string, args ...string) (int, string, bool) {
		add := len(args) > 1 && args[0] == "worktree" && args[1] == "add"
		if add && !swapped {
			swapped = true
			rtSwap(t, w, other)()
		}
		status, out, ok := git(dir, args...)
		if add {
			seen = rtInOther(t, other)
		}
		return status, out, ok
	}
	code, text := rtHire(t, d, token, w)
	d.watching.Wait()
	if !swapped {
		t.Fatalf("no worktree was asked for: %d %q", code, text)
	}
	if seen != "" {
		t.Fatalf("git made the hire's worktree in the other repo (%d %q): %s", code, text, seen)
	}
	if got := rtInOther(t, other); got != "" {
		t.Fatalf("the hire (%d %q) left this in the other repo: %s", code, text, got)
	}
	if code != http.StatusForbidden {
		t.Fatalf("the hire of a folder swapped meanwhile was answered %d %q", code, text)
	}
	branches := gitOut(t, project, "branch", "--list", "hire/*")
	if trees := gitOut(t, project, "worktree", "list", "--porcelain"); branches != "" || strings.Count(trees, "worktree ") != 2 {
		t.Fatalf("the refused hire left this in the project: branches %q, worktrees %q", branches, trees)
	}
	t.Logf("%d %q", code, text)
}

// B2's way: a scoped GM's hire from a worktree of the project, made from the project, starts on that worktree's
// commit (as `git -C <worktree> worktree add` did), on its own branch, registered in the project.
func TestAScopedGMsHireFromAWorktreeStartsFromItsCommit(t *testing.T) {
	d, _, _, token, project := scopedGM(t)
	w := rtBeside(t, project)
	gitOut(t, w, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "two")
	want := gitOut(t, w, "rev-parse", "HEAD")
	if gitOut(t, project, "rev-parse", "HEAD") == want {
		t.Fatal("the worktree is on the project's own commit, so this row proves nothing")
	}
	code, text := rtHire(t, d, token, w)
	d.watching.Wait()
	if code != http.StatusOK {
		t.Fatalf("the hire: %d %q", code, text)
	}
	var answer struct{ Branch, WorksIn string }
	rtMust(t, json.Unmarshal([]byte(text), &answer))
	if answer.Branch == "" || answer.WorksIn == "" {
		t.Fatalf("the answer: %s", text)
	}
	if got := gitOut(t, project, "rev-parse", answer.Branch); got != want {
		t.Errorf("the hire's branch starts at %s, want the worktree's %s", got, want)
	}
	if got := gitOut(t, answer.WorksIn, "rev-parse", "--path-format=absolute", "--git-common-dir"); physical(got) != filepath.Join(project, ".git") {
		t.Errorf("the hire's worktree is registered in %s", got)
	}
	if !(gmScope{root: project, git: scopeGit}).holds(answer.WorksIn) {
		t.Errorf("the scope doesn't hold the hire's worktree %s", answer.WorksIn)
	}
}

// The scope reads git's registration as git wrote it, links followed then, and never follows one in it now: a
// registered worktree swapped for a link to a folder made to pass as that worktree (its .git a file naming the
// registration) isn't held, by its own path or the link's.
func TestAScopedGMsWorktreeSwappedForAFolderPassingAsItIsNotHeld(t *testing.T) {
	project := placeRepo(t)
	s := gmScope{root: project, git: scopeGit}
	w := filepath.Join(filepath.Dir(project), "beside")
	gitOut(t, project, "worktree", "add", "-q", "-b", "feature", w)
	w = physical(w)
	if !s.holds(w) {
		t.Fatalf("holds(%s) = false before the swap", w)
	}
	fake := filepath.Join(filepath.Dir(project), "fake")
	rtMust(t, os.MkdirAll(fake, 0o755))
	rtMust(t, os.WriteFile(filepath.Join(fake, ".git"), []byte("gitdir: "+filepath.Join(project, ".git", "worktrees", "beside")+"\n"), 0o644))
	rtSwap(t, w, fake)()
	if top := gitOut(t, fake, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir"); top != fake+"\n"+filepath.Join(project, ".git") {
		t.Fatalf("the made folder doesn't pass git's look, so this row proves nothing: %q", top)
	}
	for _, folder := range []string{w, fake} {
		if s.holds(folder) {
			t.Errorf("holds(%s) = true: the registration was followed through the link", folder)
		}
	}
}
