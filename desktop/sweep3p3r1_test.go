//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// FXSWEEP3P3R1: round 1 on FXSWEEP3P3, the cold review's B1 and B2 (N1's rows are internal/gm's). Throwaway repos,
// stand-in folders and fake keys only.

// B1: the core makes a hire's worktree only through a git folder that leads back to the folder's top. A .git file
// naming another repo's .git, or one of that repo's registrations, is refused in plain words naming the folder, and
// nothing is written into that repo. A plain repo, a real linked worktree, a submodule's git folder and a repo whose
// .git is a link to its real git folder hire as before.
func TestAWorktreeIsMadeOnlyThroughAGitFolderThatLeadsBackToItsTop(t *testing.T) {
	hireInto := func(t *testing.T, r *ownerRig, folder string) (hirePlace, error) {
		t.Helper()
		keepRegistry(t)
		o := hirePlaceOptions{Worktrees: filepath.Join(r.base, "hires"), PortFree: func(int) bool { return true }}
		return prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: folder, Worktree: true}, o)
	}
	registrations := func(repo string) []string {
		var names []string
		entries, _ := os.ReadDir(filepath.Join(repo, ".git", "worktrees"))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return names
	}
	for _, c := range []struct{ name, plant string }{
		{"a .git file naming another repo's .git", "dotgit"},
		{"a .git file naming one of another repo's registrations", "registration"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newOwnerRig(t) // r.repo stands for a repo nobody hired into
			x := filepath.Join(r.base, "x")
			named := filepath.Join(r.repo, ".git")
			if c.plant == "registration" {
				gitRepoCmd(t, r.repo, "worktree", "add", "-q", "-b", "own-wt", filepath.Join(r.base, "own-wt"))
				named = filepath.Join(r.repo, ".git", "worktrees", "own-wt")
			}
			writeRepoFile(t, x, ".git", "gitdir: "+named+"\n")
			// The control: git takes x as the top, and the other repo's .git as its shared git folder.
			if top, common := gitRepoCmd(t, x, "rev-parse", "--show-toplevel"), gitRepoCmd(t, x, "rev-parse", "--path-format=absolute", "--git-common-dir"); physical(top) != physical(x) || physical(common) != physical(filepath.Join(r.repo, ".git")) {
				t.Fatalf("the control: git takes %q as the top and %q as the shared git folder, so this row proves nothing", top, common)
			}
			before := registrations(r.repo)
			status, out, err := ownerGitRun(t.Context(), x, []string{"worktree", "add", "-q", "-b", "hire/ada-abcdef", filepath.Join(r.base, "hires", "x", "ada-abcdef")})
			if err == nil || !strings.Contains(err.Error(), "isn't "+x+"'s own") || !strings.Contains(err.Error(), named) {
				t.Errorf("the core's worktree add: %d %q %v", status, out, err)
			}
			if place, err := hireInto(t, r, x); err == nil {
				t.Errorf("hired into x: %+v", place)
			}
			if got := branches(t, r.repo); got != "" {
				t.Errorf("a branch was written into the other repo: %q", got)
			}
			if got := registrations(r.repo); !slices.Equal(got, before) {
				t.Errorf("a registration was written into the other repo: %v, was %v", got, before)
			}
			if config, _ := os.ReadFile(filepath.Join(r.repo, ".git", "config")); strings.Contains(string(config), "hire/") {
				t.Errorf("the other repo's config has a hire's branch: %s", config)
			}
		})
	}
	// The controls, each hired into as before, its branch in its own repo.
	hires := func(t *testing.T, r *ownerRig, folder, repo string) {
		t.Helper()
		place, err := hireInto(t, r, folder)
		if err != nil || place.Worktree == "" {
			t.Fatalf("hired into %s: %+v %v", folder, place, err)
		}
		if got := branches(t, repo); !strings.Contains(got, "ada-abcdef") {
			t.Errorf("its branch isn't in its repo: %q", got)
		}
	}
	t.Run("a plain repo", func(t *testing.T) {
		r := newOwnerRig(t)
		hires(t, r, r.repo, r.repo)
	})
	t.Run("a real linked worktree", func(t *testing.T) {
		r := newOwnerRig(t)
		linked := filepath.Join(r.base, "linked")
		gitRepoCmd(t, r.repo, "worktree", "add", "-q", "-b", "linked", linked)
		hires(t, r, linked, r.repo)
	})
	t.Run("a submodule's git folder", func(t *testing.T) {
		r := newOwnerRig(t)
		source := filepath.Join(r.base, "subsource")
		writeRepoFile(t, source, "s.txt", "s\n")
		gitRepoCmd(t, source, "init", "-q", "-b", "main")
		stopSigning(t, source)
		gitRepoCmd(t, source, "add", "s.txt")
		gitRepoCmd(t, source, "commit", "-q", "-m", "s")
		gitRepoCmd(t, r.repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", source, "mod")
		mod := filepath.Join(r.repo, "mod")
		if common := gitRepoCmd(t, mod, "rev-parse", "--path-format=absolute", "--git-common-dir"); physical(common) != physical(filepath.Join(r.repo, ".git", "modules", "mod")) {
			t.Fatalf("the control: the submodule's git folder is %q", common)
		}
		hires(t, r, mod, mod)
	})
	t.Run("a repo whose .git is a link to its real git folder", func(t *testing.T) {
		r := newOwnerRig(t)
		real := filepath.Join(r.base, "proj-git")
		if err := os.Rename(filepath.Join(r.repo, ".git"), real); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(real, filepath.Join(r.repo, ".git")); err != nil {
			t.Fatal(err)
		}
		if top := gitRepoCmd(t, r.repo, "rev-parse", "--show-toplevel"); physical(top) != physical(r.repo) {
			t.Skipf("this git doesn't take a .git link: its top is %q", top)
		}
		hires(t, r, r.repo, r.repo)
	})
}

// B2: an engine or launcher hire writes its status in its own folder in the support folder, as its brief says. The
// app's own watch there tells the GM's inbox of a NEEDS-REVIEW line, and the team row shows its status, while the GM's
// own watch there is still refused (FXSWEEP3P3 F-2). A hire with a STATUS.md in its own place keeps working as before,
// and the built-in reads nothing in the support folder but that one file.
func TestAHiresStatusInItsOwnSupportFolderReachesTheGMsInboxAndTeam(t *testing.T) {
	b := builtinDeck(t)
	own := hireFolder(supportDir, "a1b2c3") // Rev's
	managed := filepath.Join(own, "STATUS.md")
	if err := os.MkdirAll(own, 0o700); err != nil {
		t.Fatal(err)
	}
	putFile(t, managed, "working\n")
	b.tick()
	appendTo(t, managed, "NEEDS-REVIEW: ready\n")
	b.tick()
	want := "needs-review: Rev: STATUS.md: NEEDS-REVIEW: ready"
	if got := b.firedWords(); !slices.Contains(got, want) {
		t.Errorf("fired %v, want %q", got, want)
	}
	if w := b.call("GET", "/setup/gm/events?since=0", "", true); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), want) {
		t.Errorf("the GM's inbox: %d %.400s", w.Code, w.Body)
	}
	// The team row reads the same file first.
	zed := filepath.Join(b.root, "zed")
	if err := os.MkdirAll(zed, 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(zed, "STATUS.md"), "Zed's own line\n")
	putFile(t, filepath.Join(b.root, "STATUS.md"), "a line in Rev's place\n")
	team := b.call("GET", "/setup/gm/team", "", true)
	var rows struct {
		Team []map[string]any `json:"team"`
	}
	if err := json.Unmarshal(team.Body.Bytes(), &rows); err != nil || team.Code != http.StatusOK {
		t.Fatalf("team: %d %.300s", team.Code, team.Body)
	}
	status := map[string]any{}
	for _, row := range rows.Team {
		status[row["id"].(string)] = row["status"]
	}
	if status["a1b2c3"] != "NEEDS-REVIEW: ready" || status["9z9z9z"] != "Zed's own line" {
		t.Errorf("team status: %v", status)
	}
	// The GM's own watch there: still refused.
	body, _ := json.Marshal(map[string]any{"kind": "file", "target": managed, "pattern": "NEEDS-REVIEW"})
	if w := b.call("POST", "/setup/gm/watch", string(body), true); w.Code != http.StatusForbidden {
		t.Errorf("the GM's own watch on the hire's STATUS.md: %d %q", w.Code, w.Body)
	}
	// A STATUS.md in the hire's place still fires.
	appendTo(t, filepath.Join(b.root, "STATUS.md"), "NEEDS-REVIEW: in the place\n")
	b.tick()
	if got := b.firedWords(); !slices.Contains(got, "needs-review: Rev: STATUS.md: NEEDS-REVIEW: in the place") {
		t.Errorf("the place's STATUS.md: fired %v", got)
	}
	// Made a link to another file in the support folder, the hire's STATUS.md is no longer read.
	secret := filepath.Join(supportDir, "hires", "other01", "STATUS.md")
	if err := os.MkdirAll(filepath.Dir(secret), 0o700); err != nil {
		t.Fatal(err)
	}
	putFile(t, secret, "NEEDS-REVIEW: another desk's line\n")
	if err := os.Remove(managed); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, managed); err != nil {
		t.Fatal(err)
	}
	b.tick()
	b.tick()
	for _, line := range b.firedWords() {
		if strings.Contains(line, "another desk's") {
			t.Errorf("read through a link: %q", line)
		}
	}
}
