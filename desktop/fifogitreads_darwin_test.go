package desktop

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// RT126SWEEP4 part 3's B1 (B7's class, on the owner's routes): whoever can write the .git of an owner's repo outside the
// folder for hires (a General Manager can) puts a FIFO where the hire's place check reads the shared .git as a file:
// refs/remotes/origin/HEAD (the default branch) or HEAD (the branch the owner's checkout is on). Neither is waited on:
// the place check answers at once, and takes nothing from the FIFO. Stand-ins only: the test's own home folder and
// repo; every FIFO is let go at the end (plantFifo).

// The sweep's probe: the place check of a hire into the repo, with a FIFO at its origin/HEAD, answers at once, and as
// it does with none.
func TestAFIFOAtOriginHEADNeverHoldsThePlaceCheck(t *testing.T) {
	home := isolate(t)
	homeDir = physical(home)
	repo := filepath.Join(homeDir, "code", "x")
	ownersApp(t, repo)
	control := hirePlaceWords(repo, runGit)
	fifo := plantFifo(t, filepath.Join(repo, ".git", "refs", "remotes", "origin", "HEAD"))
	var words string
	took := heldAtMost(t, "the place check", fifo, func() { words = hirePlaceWords(repo, runGit) })
	t.Logf("with a FIFO at origin/HEAD: %q in %v", words, took.Round(time.Millisecond))
	if words != control {
		t.Errorf("with a FIFO at origin/HEAD: %q, want %q as with none", words, control)
	}
}

// A desk's git places with a FIFO at the shared .git's HEAD and at its origin/HEAD, or at its worktrees folder: each
// answers at once, with no branch named from a FIFO. Git is a stand-in here, so the FIFOs are only read as files.
func TestAFIFOAtTheSharedGitsHEADNeverHoldsTheBranchRows(t *testing.T) {
	for _, c := range []struct {
		name  string
		fifos []string
	}{
		{"HEAD and origin/HEAD", []string{"HEAD", "refs/remotes/origin/HEAD"}},
		{"the worktrees folder", []string{"worktrees"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			home := isolate(t)
			homeDir = physical(home)
			repo := filepath.Join(homeDir, "code", "x")
			ownersApp(t, repo)
			common := filepath.Join(repo, ".git")
			tree := filepath.Join(homeDir, "tree")
			git := func(dir string, args ...string) (int, string, bool) {
				return 0, common + "\n" + filepath.Join(common, "worktrees", "tree"), true
			}
			var held []*heldFifo
			for _, rel := range c.fifos {
				held = append(held, plantFifo(t, filepath.Join(common, filepath.FromSlash(rel))))
			}
			var pl sandbox.Places
			var err error
			heldAtMost(t, "the desk's git places", held[0], func() {
				pl = sandbox.Places{Root: tree}
				err = deskGitPlaces(deskAt{ID: "abcdef012345", Root: tree, Checkout: repo, Git: git}, &pl)
			}, held[1:]...)
			t.Logf("%v; checkout %q, default %q", err, pl.CheckoutBranch, pl.DefaultBranchNow)
			if pl.CheckoutBranch != "" || pl.DefaultBranchNow != "" {
				t.Errorf("a branch named from a FIFO: checkout %q, default %q", pl.CheckoutBranch, pl.DefaultBranchNow)
			}
		})
	}
}

// The owner's hire from Setup into the repo with a FIFO at its origin/HEAD ends without waiting on it, refused in
// words, and the owner's next hire, the office's, is answered meanwhile, never "Hired a moment ago": the FIFO is
// still there.
func TestAnOwnersHireIntoARepoWithAFIFOAtOriginHEADHoldsNoOtherHire(t *testing.T) {
	d, _, _ := officeDeck(t)
	x := filepath.Join(homeDir, "code", "x")
	ownersApp(t, x)
	fifo := plantFifo(t, filepath.Join(x, ".git", "refs", "remotes", "origin", "HEAD"))
	type answer struct {
		status int
		why    string
		took   time.Duration
	}
	first := make(chan answer, 1)
	start := time.Now()
	go func() {
		status, _, why := d.hireFromSetupSheet(setupHire{Role: "janitor", Folder: x, Answers: []string{}, Worktree: true})
		first <- answer{status, why, time.Since(start)}
	}()
	var a answer
	done := false
	select {
	case a = <-first:
		done = true
	case <-time.After(10 * time.Second):
		t.Error("the owner's hire into the repo waited on its origin/HEAD")
	}
	spend(d)
	if status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}, "project": {"app"}}); status != http.StatusNoContent {
		t.Errorf("the owner's office hire meanwhile: %d %s", status, said)
	}
	if !done {
		fifo.release()
		a = <-first
	}
	t.Logf("the owner's hire into the repo: %d %q in %v", a.status, a.why, a.took.Round(time.Millisecond))
	// Its start's own look at the shared .git (CheckRefPlaces) then refuses the FIFO, in words, at once.
	if done && (a.status != http.StatusConflict || !strings.Contains(a.why, "isn't a folder or a file")) {
		t.Errorf("the owner's hire into the repo: %d %q", a.status, a.why)
	}
	d.watching.Wait()
}

// heldAtMost runs read, which must be done within 5s (well under one git timeout); past that the row fails, and the
// FIFOs are let go so it comes back. How long it took.
func heldAtMost(t *testing.T, what string, fifo *heldFifo, read func(), more ...*heldFifo) time.Duration {
	t.Helper()
	done := make(chan struct{})
	start := time.Now()
	go func() { defer close(done); read() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Errorf("%s waited on a FIFO: %s", what, strings.Join(fifosOpened(append([]*heldFifo{fifo}, more...)), ", "))
		for _, f := range append([]*heldFifo{fifo}, more...) {
			f.release()
		}
		<-done
	}
	return time.Since(start)
}
