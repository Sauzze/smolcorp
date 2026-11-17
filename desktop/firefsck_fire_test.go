//go:build !windows

package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// FXFIREFSCK through the fires that remove a worktree: the engine's (the office's and the GM's) and the launcher's.
// The desk commits on its branch, then makes it look merged by rewriting main's commit in place (firefsck_test.go).
// The fire still ends its session and marks it fired, but keeps its worktree and branch, and says why.

// deskCommitsThenForgesMain commits in h's worktree, then rewrites the repo's HEAD commit so h's branch looks merged.
// It shows the owner's git taking the branch as merged, so the case isn't empty.
func deskCommitsThenForgesMain(t *testing.T, repo string, h Hire) {
	t.Helper()
	writeRepoFile(t, h.Worktree, "desk.txt", "desk\n")
	gitRepoCmd(t, h.Worktree, "add", "desk.txt")
	gitRepoCmd(t, h.Worktree, "commit", "-q", "-m", "D")
	madeMergedByAForgedCommit(t, repo, gitRepoCmd(t, repo, "rev-parse", "HEAD"), gitRepoCmd(t, h.Worktree, "rev-parse", "HEAD"))
	if err := exec.Command("git", "-C", repo, "merge-base", "--is-ancestor", "refs/heads/"+h.Branch, "HEAD").Run(); err != nil {
		t.Fatalf("the owner's git doesn't take the branch as merged, so this case proves nothing: %v", err)
	}
}

// keptForTheStore checks what a fire that found the store changed left: its worktree and branch, its row fired with
// the owner's words, and said (the fire's answer) with them too. They name main's commit, which the hire's branch,
// made from main, reaches too.
func keptForTheStore(t *testing.T, repo string, h Hire, said string) {
	t.Helper()
	if _, err := os.Stat(h.Worktree); err != nil {
		t.Errorf("the worktree went: %v", err)
	}
	if !strings.Contains(branches(t, repo), h.Branch) {
		t.Errorf("the branch went: %q", branches(t, repo))
	}
	head := gitRepoCmd(t, repo, "rev-parse", "HEAD")
	words := "the repo's stored objects were changed under it (the object " + head + " holds other bytes than its name says: it is the commit " + head + ", on main and " + h.Branch + ")"
	for _, text := range []string{said, registered(t)[0].Notes} {
		if !strings.Contains(text, "Smolcorp kept ") || !strings.Contains(text, words) || !strings.Contains(text, "git fsck --full in "+repo) {
			t.Errorf("the owner is told %q, want %q", text, words)
		}
	}
	if registered(t)[0].State != "fired" {
		t.Errorf("row %+v, want fired", registered(t)[0])
	}
}

func TestAFireKeepsTheWorktreeAndBranchOfABranchMadeToLookMerged(t *testing.T) {
	// The control: with the look stood in as sound, the fire's merged look takes the forged history, and the desk's
	// branch is deleted as merged; its commit then lives on in no branch.
	t.Run("without the look", func(t *testing.T) {
		_, r, hire := hiredForFire(t, "Rex")
		windowShows(r, pane(t, "idle-empty"), true)
		deskCommitsThenForgesMain(t, r.repo, hire)
		r.o.StoreLook = func(context.Context, string, []string) (int, string, error) { return 1, "", nil }
		if kept, err := fireEngineHire(hire, 0, r.o, false); err != nil || kept != "" {
			t.Fatalf("kept %q, err %v", kept, err)
		}
		if branches(t, r.repo) != "" {
			t.Fatalf("the branch was kept: %q", branches(t, r.repo))
		}
	})
	t.Run("the look", func(t *testing.T) {
		_, r, hire := hiredForFire(t, "Rex")
		windowShows(r, pane(t, "idle-empty"), true)
		deskCommitsThenForgesMain(t, r.repo, hire)
		kept, said, err := fireEngineHireSaid(hire, 0, r.o, false)
		if err != nil || !strings.HasPrefix(kept, "Smolcorp kept Rex's worktree "+hire.Worktree+" and its branch "+hire.Branch+": ") {
			t.Fatalf("kept %q, err %v", kept, err)
		}
		if !r.tmux.ran("kill-server") || r.tmux.up {
			t.Error("its session wasn't ended")
		}
		keptForTheStore(t, r.repo, hire, said)
	})
}

// The ruling's (2): a look that can't be made keeps them too, and says so; the fire still ends the session.
func TestAFireWhoseLookCantBeMadeKeepsTheWorktreeAndBranch(t *testing.T) {
	_, r, hire := hiredForFire(t, "Rex")
	windowShows(r, pane(t, "idle-empty"), true)
	r.o.StoreLook = func(context.Context, string, []string) (int, string, error) {
		return 0, "", errors.New("it took over 2 minutes")
	}
	kept, said, err := fireEngineHireSaid(hire, 0, r.o, false)
	want := "Smolcorp kept Rex's worktree " + hire.Worktree + " and its branch " + hire.Branch +
		": it couldn't check the repo's stored objects (it took over 2 minutes), so it didn't look at whether the branch is merged." +
		" To look, run git fsck --full in " + r.repo + "."
	if err != nil || kept != want || said != want || registered(t)[0].Notes != want || registered(t)[0].State != "fired" {
		t.Fatalf("kept %q, said %q, err %v, row %+v", kept, said, err, registered(t)[0])
	}
	if _, err := os.Stat(hire.Worktree); err != nil || !strings.Contains(branches(t, r.repo), hire.Branch) {
		t.Errorf("the worktree (%v) or the branch (%q) went", err, branches(t, r.repo))
	}
	if !r.tmux.ran("kill-server") {
		t.Error("its session wasn't ended")
	}
}

func TestTheGMsFireSaysWhyTheWorktreeAndBranchWereKept(t *testing.T) {
	g := newGMRig(t, "Rex")
	deskCommitsThenForgesMain(t, g.r.repo, g.hire)
	w := g.fire(t, true, map[string]any{"removeWorktree": true})
	var got map[string]any
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %q", w.Code, w.Body)
	}
	note, _ := got["note"].(string)
	kept, _ := got["kept"].(string)
	if kept == "" || !strings.HasPrefix(note, kept) || got["worktree"] != g.hire.Worktree || got["branch"] != g.hire.Branch {
		t.Errorf("answer %v", got)
	}
	keptForTheStore(t, g.r.repo, g.hire, note)
}

func TestALauncherFireKeepsTheWorktreeAndBranchOfABranchMadeToLookMerged(t *testing.T) {
	r := newLRig(t)
	h, w := r.hire("janitor")
	deskCommitsThenForgesMain(t, r.repo, h)
	kept, err := r.d.fireLauncherHire(h, r.o, false)
	if err != nil || w.stopped() != 1 {
		t.Fatalf("kept %q, err %v, stops %d", kept, err, w.stopped())
	}
	keptForTheStore(t, r.repo, h, kept)
	if len(strings.Split(r.worktrees(), "\n")) != 2 {
		t.Errorf("worktrees: %s", r.worktrees())
	}
	if _, err := os.Stat(filepath.Join(supportDir, "recaps", h.ID+".md")); err != nil {
		t.Errorf("no recap: %v", err)
	}
}

// A hire in a window of its own, its tab closed by the Mac app (a stand-in), fired with its worktree's removal asked:
// the same look, the same words.
func TestAFireOfAHireInAWindowOfItsOwnKeepsTheWorktreeAndBranchOfABranchMadeToLookMerged(t *testing.T) {
	d, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
	deskCommitsThenForgesMain(t, g.r.repo, g.hire)
	kept, _, err := d.fireOwnWindowHire(g.hire, "tty:/dev/ttys009", Claude, false, false)
	if err != nil || len(closedTabs(fake)) != 1 {
		t.Fatalf("kept %q, err %v, closed %+v", kept, err, fake.asked)
	}
	keptForTheStore(t, g.r.repo, g.hire, kept)
}

// aWideRepo is a repo whose main holds 3000 files in one commit: more names than a pipe holds unread.
func aWideRepo(t *testing.T) string {
	t.Helper()
	dir := physical(t.TempDir())
	gitRepoCmd(t, dir, "init", "-q", "-b", "main")
	var stream strings.Builder
	stream.WriteString("commit refs/heads/main\ncommitter t <t@example.com> 1 +0000\ndata 4\nwide\n")
	for i := 0; i < 3000; i++ {
		body := fmt.Sprintf("file %d\n", i)
		fmt.Fprintf(&stream, "M 100644 inline d%02d/f%04d\ndata %d\n%s", i%30, i, len(body), body)
	}
	cmd := exec.Command("git", "-C", dir, "fast-import", "--quiet")
	cmd.Stdin = strings.NewReader(stream.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fast-import: %v %s", err, out)
	}
	return dir
}

// The ruling's (2), for answers real git doesn't give: a git that stops reading names after the first, and one that
// reads them all but answers for the first only. Each is a look that couldn't be made, never a sound store.
func TestTheFiresLookFailsClosedOnAnAnswerShortOfItsList(t *testing.T) {
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git here")
	}
	for _, c := range []struct{ name, takes, want string }{
		{"names cut short", `IFS= read -r first`, "git's list of them was cut short"},
		{"answers short", `IFS= read -r first; cat >/dev/null`, "git answered for 1 of the 3032 objects it listed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo := aWideRepo(t)
			bin := t.TempDir()
			// The stand-in: cat-file takes names as c.takes says, answers for the first alone, and ends.
			script := "#!/bin/sh\nfor a in \"$@\"; do [ \"$a\" = cat-file ] && cat=1; done\n" +
				"if [ -n \"$cat\" ]; then " + c.takes + "; printf '%s\\n' \"$first\" | '" + real + "' \"$@\"; exit $?; fi\n" +
				"exec '" + real + "' \"$@\"\n"
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			objects, cause, err := storeHoldsWhatItNames(context.Background(), repo, []string{"HEAD"})
			if err == nil || err.Error() != c.want || cause != "" || objects != 1 {
				t.Fatalf("objects %d, cause %q, err %v; want the look refused with %q", objects, cause, err, c.want)
			}
		})
	}
}
