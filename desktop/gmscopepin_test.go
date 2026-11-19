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

// FXGMSCOPEPIN (RT126RESOLVESWEEP S5): a scoped GM's hire folder is checked once and used as checked. A folder that
// passes the scope (one the GM made to pass, with a .git file naming the project's .git, or a real worktree of the
// project) is swapped for a link to another project once it has passed. The swap is made at a fixed point, as the
// hire's place is made: just before its first git there (prepareHire's resolve), or just before its worktree is made.

// swapBeforePlace makes the engine's git swap folder for a link to other just before the first git the hire's place
// runs there with args starting so: its resolve ("rev-parse"), or the making of its worktree ("worktree"), wherever git
// is asked to make it (RT126GMSCOPEPINR2's case-3 ruling: since B2 the worktree is made from the project). It counts
// the worktrees git is asked to make once the folder is swapped, the one the swap was made before aside.
func swapBeforePlace(t *testing.T, d *Deck, folder, other, at string) (made *int) {
	t.Helper()
	git, swapped := d.engine.Git, false
	if git == nil {
		git = runGit
	}
	made = new(int)
	d.engine.Git = func(dir string, args ...string) (int, string, bool) {
		if swapped && len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
			*made++
		}
		if !swapped && (dir == folder || at == "worktree") && len(args) > 0 && args[0] == at {
			swapped = true
			if err := os.Rename(folder, folder+".away"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(other, folder); err != nil {
				t.Fatal(err)
			}
		}
		return git(dir, args...)
	}
	return made
}

func TestAScopedGMsHireFolderSwappedForALinkAfterItsCheckIsNeverHiredInto(t *testing.T) {
	// made: the GM's own trick, a folder in the project whose .git is a file naming the project's .git.
	made := func(t *testing.T, project string) string {
		folder := filepath.Join(project, "inner")
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, ".git"), []byte("gitdir: "+filepath.Join(project, ".git")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return physical(folder)
	}
	// worktree: a real worktree of the project, beside it, which the scope holds until it is swapped.
	worktree := func(t *testing.T, project string) string {
		folder := filepath.Join(homeDir, "beside")
		gitOut(t, project, "worktree", "add", "-q", "-b", "feature", folder)
		return physical(folder)
	}
	for _, c := range []struct {
		name   string
		folder func(t *testing.T, project string) string
		at     string
	}{
		{"a folder in the project made to pass as a worktree, swapped as its place is resolved", made, "rev-parse"},
		{"a worktree of the project, swapped as its place is resolved", worktree, "rev-parse"},
		{"a worktree of the project, swapped as its own worktree is made", worktree, "worktree"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, _, _, token, project := scopedGM(t)
			other := placeRepo(t) // the stand-in "other project", outside the scope
			folder := c.folder(t, project)
			made := swapBeforePlace(t, d, folder, other, c.at)
			body, _ := json.Marshal(map[string]any{"role": "janitor", "folder": folder})
			code, text := scopedCall(t, d, token, "POST", "/setup/gm/hire", string(body))
			// Nothing of the hire's is in the other project: no branch, no worktree registered there, no row working there.
			branches := gitOut(t, other, "branch", "--list", "hire/*")
			trees := gitOut(t, other, "worktree", "list", "--porcelain")
			var rows []string
			for _, h := range registered(t) {
				if h.Role != gmRole {
					rows = append(rows, h.Repo+" "+h.Worktree.storedOnly())
				}
			}
			if strings.TrimSpace(branches) != "" || strings.Count(trees, "worktree ") != 1 || len(rows) != 0 {
				t.Fatalf("the hire (%d %q) was made in the other project: branches %q, worktrees %q, rows %q", code, text, branches, trees, rows)
			}
			if code == http.StatusOK || code == http.StatusNoContent {
				t.Fatalf("the hire was answered %d %q", code, text)
			}
			// Swapped as its place is resolved: refused there, before git is asked to make anything anywhere.
			if c.at == "rev-parse" && *made != 0 {
				t.Errorf("git was asked to make %d worktree(s) after the swap", *made)
			}
			t.Logf("refused: %d %q", code, text)
		})
	}
}

// A folder passes the scope as a worktree's top only where the project's git registered a worktree, which no desk can
// write: never one the GM made to pass (its .git a file naming the project's), in the project or anywhere else.
func TestAScopedGMsHireFolderIsHeldOnlyAsAWorktreeTheProjectsGitMade(t *testing.T) {
	_, _, _, _, project := scopedGM(t)
	s := gmScope{root: project, git: scopeGit}
	made := func(folder string) string {
		os.MkdirAll(folder, 0o755)
		os.WriteFile(filepath.Join(folder, ".git"), []byte("gitdir: "+filepath.Join(project, ".git")+"\n"), 0o644)
		return folder
	}
	beside := physical(t.TempDir())
	wt := filepath.Join(beside, "wt")
	gitOut(t, project, "worktree", "add", "-q", "-b", "hire/x-1", wt)
	inside := filepath.Join(project, "hires", "y")
	gitOut(t, project, "worktree", "add", "-q", "-b", "hire/y-1", inside)
	for folder, want := range map[string]bool{
		project:                               true,
		wt:                                    true, // a worktree beside it
		inside:                                true, // a worktree inside it (hires' worktrees set there)
		made(filepath.Join(project, "inner")): false,
		made(filepath.Join(wt, "deeper")):     false,
		made(filepath.Join(beside, "made")):   false,
	} {
		if got := s.holds(folder); got != want {
			t.Errorf("holds(%s) = %v, want %v", folder, got, want)
		}
	}
}

// The nickname and the pin keep the folder as the scope checked it, links followed, never the words sent.
func TestAScopedGMsNicknameAndPinKeepTheFolderAsChecked(t *testing.T) {
	d, _, _, token, project := scopedGM(t)
	link := filepath.Join(homeDir, "via")
	if err := os.Symlink(project, link); err != nil {
		t.Fatal(err)
	}
	if code, text := scopedCall(t, d, token, "POST", "/setup/pin?key=0&folder="+link, ""); code != http.StatusNoContent {
		t.Fatalf("pin: %d %q", code, text)
	}
	d.mu.Lock()
	pin := d.config.pinAt(0, 0)
	d.mu.Unlock()
	if pin == nil || *pin != project {
		kept := "none"
		if pin != nil {
			kept = *pin
		}
		t.Errorf("the pin kept %q, not %q", kept, project)
	}
	status, _, body := d.gmScopeRefusal(gmNow(), http.MethodPost, "/setup/gm/nickname", nil, []byte(`{"folder":"`+link+`","name":"Bo"}`))
	data, _ := json.Marshal(body)
	var given struct{ Folder string }
	json.Unmarshal(data, &given)
	if status != 0 || given.Folder != project {
		t.Errorf("the nickname's folder, as the route is given it: %d %q, want %q", status, given.Folder, project)
	}
}
