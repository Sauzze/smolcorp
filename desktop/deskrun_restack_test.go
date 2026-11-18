//go:build !windows

package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// SA-X6 stacked on SA-M6 (MSYSAX6): once the fire removed a registered desk's tree, git on it gets git's own answer
// (it can't go into the folder: 128, ran), as before deskRun, which SA-M6's stop after a core restart reads as no repo.
// A tree that is there but has no registration is still refused, never run by its .git file (P3).
func TestGitOnARemovedDesksTreeIsGitsOwnAnswerAndAnUnregisteredOneIsRefused(t *testing.T) {
	r := newDeskRunRig(t)
	if err := os.Rename(filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef"), filepath.Join(r.base, "registration")); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := runGit(r.tree, "rev-parse", "--path-format=absolute", "--git-common-dir", "--git-dir"); ok {
		t.Error("a tree with no registration was run")
	}
	if err := os.RemoveAll(r.tree); err != nil {
		t.Fatal(err)
	}
	status, out, ok := runGit(r.tree, "rev-parse", "--path-format=absolute", "--git-common-dir", "--git-dir")
	if !ok || status != 128 || !strings.Contains(out, "cannot change to") {
		t.Errorf("a removed tree: %d %q %v, want git's own 128", status, out, ok)
	}
	pl := sandbox.Places{Root: physical(r.tree)}
	if err := deskGitPlaces(deskAt{ID: r.hire.ID, Root: r.tree, Git: runGit}, &pl); err != nil || pl.GitDir != "" {
		t.Errorf("SA-M6's places of a removed tree: %q %v, want no repo", pl.GitDir, err)
	}
}

// SA-X6 stacked on SA-M6 (MSYSAX6): hireEnvironment's list is one, in hireEnvironmentOf, so what the core runs for a
// desk (deskRun) is never given the bundled tmux's guard variable the core inherited, as a desk's start isn't (SA-M6,
// RT126SAM6 G1).
func TestADesksRunIsNotGivenTheTmuxGuardTheCoreInherited(t *testing.T) {
	r := newDeskRunRig(t)
	t.Setenv("SMOLCORP_TMUX_SERVER", "4242")
	env := func(id string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var out strings.Builder
		if err := deskRun(ctx, id, r.base, os.Environ(), &out, &out, "/usr/bin/env"); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if !strings.Contains(env(""), "SMOLCORP_TMUX_SERVER=4242") {
		t.Fatal("the control: a run for no desk wasn't given the variable, so this row proves nothing")
	}
	if got := env(r.hire.ID); strings.Contains(got, "SMOLCORP_TMUX_SERVER") {
		t.Error("a desk's run was given the tmux guard the core inherited")
	}
	for _, variable := range hireEnvironment() {
		if strings.HasPrefix(strings.ToUpper(variable), "SMOLCORP_TMUX_SERVER=") {
			t.Error("a desk's start is given the tmux guard the core inherited")
		}
	}
}
