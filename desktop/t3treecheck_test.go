//go:build !windows

package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// SA-X7's Take over check (t3treecheck.go, C6), on deskRunRig's real repo and hire worktree: each row changes one
// thing a hire could have changed, and the check names it; the untouched worktree passes.

// withHooks gives the rig's repo a hooks folder its config names (.githooks, from the worktree's top), with one hook
// on main, and the hire's branch brought up to main.
func (r *deskRunRig) withHooks(t *testing.T) string {
	t.Helper()
	writeRepoFile(t, r.repo, ".githooks/pre-commit", "#!/bin/sh\nexit 0\n")
	gitRepoCmd(t, r.repo, "add", ".githooks")
	gitRepoCmd(t, r.repo, "commit", "-q", "-m", "hooks")
	gitRepoCmd(t, r.repo, "config", "core.hooksPath", ".githooks")
	gitRepoCmd(t, r.tree, "merge", "-q", "--ff-only", "main")
	return filepath.Join(r.tree, ".githooks")
}

func checkSays(t *testing.T, h Hire, want string) {
	t.Helper()
	got := t3TreeCheck(h)
	if want == "" && got != "" || want != "" && !strings.Contains(got, want) {
		t.Errorf("the check says %q, want %q", got, want)
	}
}

func TestTakeOverChecksTheWorktreesDotGitAndCommondir(t *testing.T) {
	t.Run("untouched", func(t *testing.T) {
		checkSays(t, newDeskRunRig(t).hire, "")
	})
	t.Run(".git names a git folder the hire made", func(t *testing.T) {
		r := newDeskRunRig(t)
		fake := filepath.Join(r.base, "made-by-the-hire")
		if out, err := exec.Command("cp", "-R", filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef"), fake).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		writeRepoFile(t, r.tree, ".git", "gitdir: "+fake+"\n")
		checkSays(t, r.hire, "its .git names another git folder than its registration")
	})
	t.Run("commondir leads elsewhere", func(t *testing.T) {
		r := newDeskRunRig(t)
		other := filepath.Join(r.base, "other")
		gitRepoCmd(t, r.base, "init", "-q", other)
		writeRepoFile(t, filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef"), "commondir", filepath.Join(other, ".git")+"\n")
		checkSays(t, r.hire, "commondir names another git folder")
	})
	t.Run("no commondir", func(t *testing.T) {
		r := newDeskRunRig(t)
		os.Remove(filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef", "commondir"))
		checkSays(t, r.hire, "has no commondir")
	})
}

// Each setting of config.worktree that could run a program for the owner's git refuses; data passes. The file is
// read whether or not the repo turns worktree config on (a hire's git could, later, from a sandbox of its own).
func TestTakeOverChecksTheWorktreesConfig(t *testing.T) {
	for _, row := range []struct{ key, value, said string }{
		{"user.name", "Ada", ""},
		{"core.fsmonitor", "/stand-in", "core.fsmonitor"},
		{"core.hooksPath", "/stand-in", "core.hookspath"},
		{"filter.sneak.clean", "/stand-in", "filter.*.clean"},
		{"alias.st", "!/stand-in", "alias.st"},
		{"lfs.customtransfer.sneak.path", "/stand-in", "lfs.*.path"},
		{"include.path", "/stand-in.cfg", "include.path"},
		{"core.gitProxy", "/stand-in", "core.gitproxy"},
		// RT126SAX7: a program in a data section (B2), where the work tree is (B1), and a merge strategy's program.
		{"gc.recentObjectsHook", "/stand-in", "gc.recentobjectshook"},
		{"core.worktree", "/stand-in", "core.worktree"},
		{"core.bare", "false", "core.bare"},
		{"pull.twohead", "stand-in", "pull.twohead"},
		{"pull.rebase", "true", ""},
		// RT126SAX7R2's F1: a merge strategy, by the hire's own branch.
		{"branch.hire/ada-abcdef.mergeOptions", "-s stand-in", "branch.*.mergeoptions"},
		{"branch.hire/ada-abcdef.remote", "origin", ""},
	} {
		t.Run(row.key, func(t *testing.T) {
			r := newDeskRunRig(t)
			gitRepoCmd(t, r.repo, "config", "--file", filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef", "config.worktree"), row.key, row.value)
			want := ""
			if row.said != "" {
				want = "its config.worktree has " + row.said + ","
			}
			checkSays(t, r.hire, want)
		})
	}
	t.Run("a link", func(t *testing.T) {
		r := newDeskRunRig(t)
		if err := os.Symlink(filepath.Join(r.base, "elsewhere"), filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef", "config.worktree")); err != nil {
			t.Fatal(err)
		}
		checkSays(t, r.hire, "its config.worktree isn't a plain file")
	})
}

// RT126SAX7's B1 probe: with worktree config on (git sparse-checkout turns it on), the owner's git takes
// core.worktree from config.worktree, and with it the top a relative hooks folder is taken from, a folder the hooks
// check never looks at. The control: the owner's git, run from the hire's worktree, works on that folder. Nothing runs
// a hook: rev-parse reads config only.
func TestTakeOverRefusesAWorktreeConfigThatMovesTheWorkTree(t *testing.T) {
	r := newDeskRunRig(t)
	r.withHooks(t)
	gitRepoCmd(t, r.repo, "config", "extensions.worktreeConfig", "true")
	elsewhere := filepath.Join(r.base, "elsewhere")
	writeRepoFile(t, filepath.Join(elsewhere, ".githooks"), "pre-commit", "#!/bin/sh\n/stand-in\n")
	gitRepoCmd(t, r.tree, "config", "--worktree", "core.worktree", elsewhere)
	if got := gitRepoCmd(t, r.tree, "rev-parse", "--show-toplevel"); realPath(got) != realPath(elsewhere) {
		t.Fatalf("the control: the owner's git works on %q, not %q, so this row proves nothing", got, elsewhere)
	}
	checkSays(t, r.hire, "its config.worktree has core.worktree,")
}

// The hooks folder the owner's config names, in the worktree, must be main's, file for file, by what is on the disk.
func TestTakeOverChecksTheHooksFolderAgainstTheBaseBranch(t *testing.T) {
	t.Run("as main has it", func(t *testing.T) {
		r := newDeskRunRig(t)
		r.withHooks(t)
		checkSays(t, r.hire, "")
	})
	t.Run("a hook changed", func(t *testing.T) {
		r := newDeskRunRig(t)
		writeRepoFile(t, r.withHooks(t), "pre-commit", "#!/bin/sh\n/stand-in\n")
		checkSays(t, r.hire, "the hooks folder your config names (.githooks) isn't main's")
	})
	// The index is the hire's to write: one that says the file is unchanged is never believed.
	t.Run("a hook changed, the index told otherwise", func(t *testing.T) {
		r := newDeskRunRig(t)
		writeRepoFile(t, r.withHooks(t), "pre-commit", "#!/bin/sh\n/stand-in\n")
		gitRepoCmd(t, r.tree, "update-index", "--assume-unchanged", ".githooks/pre-commit")
		if got := gitRepoCmd(t, r.tree, "status", "--porcelain"); got != "" {
			t.Fatalf("the control: git still sees the change (%q), so this row proves nothing", got)
		}
		checkSays(t, r.hire, "isn't main's")
	})
	t.Run("a hook changed and committed on the hire's branch", func(t *testing.T) {
		r := newDeskRunRig(t)
		writeRepoFile(t, r.withHooks(t), "pre-commit", "#!/bin/sh\n/stand-in\n")
		gitRepoCmd(t, r.tree, "commit", "-q", "-a", "-m", "mine")
		checkSays(t, r.hire, "isn't main's")
	})
	t.Run("a hook added", func(t *testing.T) {
		r := newDeskRunRig(t)
		writeRepoFile(t, r.withHooks(t), "post-checkout", "#!/bin/sh\n/stand-in\n")
		checkSays(t, r.hire, "isn't main's")
	})
	t.Run("a hook added that git ignores", func(t *testing.T) {
		r := newDeskRunRig(t)
		hooks := r.withHooks(t)
		writeRepoFile(t, filepath.Join(r.repo, ".git", "info"), "exclude", "/.githooks/post-checkout\n")
		writeRepoFile(t, hooks, "post-checkout", "#!/bin/sh\n/stand-in\n")
		if got := gitRepoCmd(t, r.tree, "status", "--porcelain"); got != "" {
			t.Fatalf("the control: git lists the file (%q), so this row proves nothing", got)
		}
		checkSays(t, r.hire, "isn't main's")
	})
	t.Run("a link in it", func(t *testing.T) {
		r := newDeskRunRig(t)
		if err := os.Symlink("/stand-in", filepath.Join(r.withHooks(t), "pre-push")); err != nil {
			t.Fatal(err)
		}
		checkSays(t, r.hire, "isn't main's")
	})
	t.Run("the folder a link", func(t *testing.T) {
		r := newDeskRunRig(t)
		hooks := r.withHooks(t)
		elsewhere := filepath.Join(r.base, "elsewhere")
		writeRepoFile(t, elsewhere, "pre-commit", "#!/bin/sh\nexit 0\n")
		os.RemoveAll(hooks)
		if err := os.Symlink(elsewhere, hooks); err != nil {
			t.Fatal(err)
		}
		checkSays(t, r.hire, "a link is on the way to the hooks folder your config names (.githooks)")
	})
	t.Run("no such folder", func(t *testing.T) {
		r := newDeskRunRig(t)
		os.RemoveAll(r.withHooks(t))
		checkSays(t, r.hire, "")
	})
	// "~/" is your home, as git expands it (RT126SAX7's nit 1): one that leads into the worktree is looked at too.
	t.Run("a hook changed, the folder named from your home", func(t *testing.T) {
		r := newDeskRunRig(t)
		hooks := r.withHooks(t)
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		rel, err := filepath.Rel(realPath(home), hooks)
		if err != nil {
			t.Fatal(err)
		}
		gitRepoCmd(t, r.repo, "config", "core.hooksPath", "~/"+filepath.ToSlash(rel))
		checkSays(t, r.hire, "")
		writeRepoFile(t, hooks, "pre-commit", "#!/bin/sh\n/stand-in\n")
		checkSays(t, r.hire, "isn't main's")
	})
	t.Run("a home git can't find", func(t *testing.T) {
		r := newDeskRunRig(t)
		r.withHooks(t)
		gitRepoCmd(t, r.repo, "config", "core.hooksPath", "~no-such-user-x7/hooks")
		checkSays(t, r.hire, "the hooks folder your config names (~no-such-user-x7/hooks) couldn't be found")
	})
	t.Run("a folder outside the worktree", func(t *testing.T) {
		r := newDeskRunRig(t)
		writeRepoFile(t, filepath.Join(r.base, "hooks"), "pre-commit", "#!/bin/sh\n/stand-in\n")
		gitRepoCmd(t, r.repo, "config", "core.hooksPath", filepath.Join(r.base, "hooks"))
		checkSays(t, r.hire, "")
	})
	t.Run("no base branch", func(t *testing.T) {
		r := newDeskRunRig(t)
		r.withHooks(t)
		gitRepoCmd(t, r.repo, "branch", "-m", "main", "trunk")
		checkSays(t, r.hire, "your repo has no main, master or default branch")
	})
	t.Run("the default branch, as origin's HEAD names it", func(t *testing.T) {
		r := newDeskRunRig(t)
		r.withHooks(t)
		gitRepoCmd(t, r.repo, "branch", "-m", "main", "trunk")
		writeRepoFile(t, filepath.Join(r.repo, ".git", "refs", "remotes", "origin"), "HEAD", "ref: refs/remotes/origin/trunk\n")
		checkSays(t, r.hire, "")
	})
}

// Through Take over: a worktree that can't go is looked at once the hire is stopped, handed back at once, and T3 Code
// never hears of it; so is one that changes while it is being stopped.
func TestT3TakeOverChecksTheWorktreeOnceTheHireIsStopped(t *testing.T) {
	plant := func(k *t3TakeOverRig) {
		k.t.Helper()
		gitRepoCmd(k.t, k.hire.Repo, "config", "--file", filepath.Join(k.hire.Repo, ".git", "worktrees", "ada-abcdef", "config.worktree"), "core.fsmonitor", "/stand-in")
	}
	for _, during := range []bool{false, true} {
		t.Run(map[bool]string{false: "before", true: "while it is stopped"}[during], func(t *testing.T) {
			k := newT3TakeOverRig(t)
			ways := k.ways()
			if during {
				stop := ways.Stop
				ways.Stop = func(h Hire, session string) error {
					plant(k)
					return stop(h, session)
				}
			} else {
				plant(k)
			}
			_, err := t3TakeOver(k.hire, k.session, ways)
			if err == nil || !strings.Contains(err.Error(), "config.worktree has core.fsmonitor") || !strings.Contains(err.Error(), "so it wasn't handed to T3 Code.") {
				t.Fatalf("got %v", err)
			}
			log := strings.Join(k.log, ",")
			if !strings.Contains(log, "stop "+t3TakeOverSession) || !strings.Contains(log, "resume "+t3TakeOverSession) || strings.Contains(log, "call ") {
				t.Errorf("did %q", k.log)
			}
			for _, run := range k.runs() {
				if len(run) > 1 && run[0] == "project" && run[1] == "add" {
					t.Errorf("T3 Code was told of the worktree: %q", run)
				}
			}
			if _, err := os.Lstat(k.fence()); err == nil {
				t.Error("the fence was left on the worktree")
			}
		})
	}
}

// RT126SAX7R2's F3: a repo of its own below the worktree's top, whose config and hooks the owner's git would use from
// inside it, is refused, whatever git ignores; a link isn't walked into. The controls show the owner's git there works
// on that repo (rev-parse reads, runs nothing). With no repo below, the untouched rows above pass.
func TestTakeOverRefusesARepoBelowTheWorktreesTop(t *testing.T) {
	t.Run("git init in a folder", func(t *testing.T) {
		r := newDeskRunRig(t)
		sub := filepath.Join(r.tree, "sub")
		gitRepoCmd(t, r.tree, "init", "-q", sub)
		if got := gitRepoCmd(t, sub, "rev-parse", "--show-toplevel"); realPath(got) != realPath(sub) {
			t.Fatalf("the control: the owner's git works on %q, not %q, so this row proves nothing", got, sub)
		}
		checkSays(t, r.hire, "a repo of its own is in its worktree (sub/.git), and your git there would use its config and hooks")
	})
	t.Run("a .git file naming a git folder, in a folder git ignores", func(t *testing.T) {
		r := newDeskRunRig(t)
		other := filepath.Join(r.base, "other")
		gitRepoCmd(t, r.base, "init", "-q", other)
		writeRepoFile(t, r.tree, ".gitignore", "vendor/\n")
		writeRepoFile(t, filepath.Join(r.tree, "vendor", "pkg"), ".git", "gitdir: "+filepath.Join(other, ".git")+"\n")
		if got := gitRepoCmd(t, filepath.Join(r.tree, "vendor", "pkg"), "rev-parse", "--git-dir"); realPath(got) != realPath(filepath.Join(other, ".git")) {
			t.Fatalf("the control: the owner's git uses %q, so this row proves nothing", got)
		}
		checkSays(t, r.hire, "a repo of its own is in its worktree (vendor/pkg/.git)")
	})
	t.Run("a .GIT folder", func(t *testing.T) {
		r := newDeskRunRig(t)
		if err := os.MkdirAll(filepath.Join(r.tree, "sub", ".GIT"), 0o755); err != nil {
			t.Fatal(err)
		}
		checkSays(t, r.hire, "a repo of its own is in its worktree (sub/.GIT)")
	})
	t.Run("a link named .git", func(t *testing.T) {
		r := newDeskRunRig(t)
		if err := os.MkdirAll(filepath.Join(r.tree, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(r.repo, ".git"), filepath.Join(r.tree, "sub", ".git")); err != nil {
			t.Fatal(err)
		}
		checkSays(t, r.hire, "a repo of its own is in its worktree (sub/.git)")
	})
	// A folder git takes as a bare repo: by git's own default (the control runs with no config of yours), the owner's
	// git in it works on it.
	t.Run("a bare repo's look", func(t *testing.T) {
		r := newDeskRunRig(t)
		bare := filepath.Join(r.tree, "docs", "b")
		writeRepoFile(t, bare, "HEAD", "ref: refs/heads/main\n")
		for _, dir := range []string{"objects", "refs"} {
			if err := os.MkdirAll(filepath.Join(bare, dir), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		control := exec.Command("git", "-C", bare, "rev-parse", "--git-dir")
		control.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := control.Output(); err != nil || strings.TrimSpace(string(out)) != "." {
			t.Fatalf("the control: the owner's git doesn't take %q as a repo (%q, %v), so this row proves nothing", bare, out, err)
		}
		checkSays(t, r.hire, "a repo of its own is in its worktree (docs/b)")
	})
	// RT126SAX7R3's F1′ (its probe a): a HEAD with a commondir beside it, naming a folder with objects, refs and a
	// config but no HEAD. Neither folder has a bare repo's look, yet git takes the first as a repo, with the second's
	// config and hooks (the control, with no config of yours).
	t.Run("a HEAD with a commondir beside it", func(t *testing.T) {
		r := newDeskRunRig(t)
		sub, evil := filepath.Join(r.tree, "sub"), filepath.Join(r.tree, "evil")
		writeRepoFile(t, sub, "HEAD", "ref: refs/heads/main\n")
		writeRepoFile(t, sub, "commondir", "../evil\n")
		for _, dir := range []string{"objects", "refs"} {
			if err := os.MkdirAll(filepath.Join(evil, dir), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		writeRepoFile(t, evil, "config", "[core]\n\trepositoryformatversion = 0\n\tbare = true\n[stand]\n\tin = the-hires\n")
		control := exec.Command("git", "-C", sub, "config", "--get", "stand.in")
		control.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := control.Output(); err != nil || strings.TrimSpace(string(out)) != "the-hires" {
			t.Fatalf("the control: the owner's git in %q doesn't read the folder its commondir names (%q, %v), so this row proves nothing", sub, out, err)
		}
		checkSays(t, r.hire, "a repo of its own is in its worktree (sub), and your git there would use its config and hooks")
	})
	t.Run("a HEAD alone", func(t *testing.T) {
		r := newDeskRunRig(t)
		writeRepoFile(t, filepath.Join(r.tree, "docs"), "HEAD", "a file of that name\n")
		checkSays(t, r.hire, "")
	})
	// RT126SAX7R3's test gap: a bare repo's look needs both objects and refs beside its HEAD; one of them isn't a repo.
	t.Run("a HEAD and objects, no refs", func(t *testing.T) {
		r := newDeskRunRig(t)
		half := filepath.Join(r.tree, "docs", "half")
		writeRepoFile(t, half, "HEAD", "ref: refs/heads/main\n")
		if err := os.MkdirAll(filepath.Join(half, "objects"), 0o755); err != nil {
			t.Fatal(err)
		}
		checkSays(t, r.hire, "")
	})
	t.Run("a link to a repo elsewhere", func(t *testing.T) {
		r := newDeskRunRig(t)
		if err := os.Symlink(r.repo, filepath.Join(r.tree, "linked")); err != nil {
			t.Fatal(err)
		}
		checkSays(t, r.hire, "")
	})
	t.Run("a folder that can't be read", func(t *testing.T) {
		r := newDeskRunRig(t)
		closed := filepath.Join(r.tree, "closed")
		if err := os.Mkdir(closed, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(closed, 0o755) })
		if _, err := os.ReadDir(closed); err == nil {
			t.Skip("this user reads a folder with no permissions")
		}
		checkSays(t, r.hire, "its worktree couldn't be looked through for a repo of its own")
	})
	t.Run("out of time", func(t *testing.T) {
		r := newDeskRunRig(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if got := t3NoRepoBelow(ctx, r.tree); got != "its worktree couldn't be looked through for a repo of its own" {
			t.Errorf("the check says %q", got)
		}
		if got := t3NoRepoBelow(context.Background(), r.tree); got != "" {
			t.Errorf("the check, in time, says %q", got)
		}
	})
}
