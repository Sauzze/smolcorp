package desktop

import (
	"os"
	"path/filepath"
	"strings"
)

// The checks every hire's worktree must pass before it is taken away, on any
// OS (hirecleanup.go adds the tmux socket's, hirelauncher.go the launcher's).

// within says whether path is b or below it, both absolute and resolved.
func within(b, path string) bool {
	rel, err := filepath.Rel(b, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// removeOwnWorktree removes a hire's worktree only when that is the hire's
// own: the record's paths are absolute and clean,
// the worktree is a direct child of the folder hires' worktrees are made in
// (worktrees "" for the usual one), checked out on the
// branch prepareHire made for it, never the main repo, a folder inside it or one
// that holds it. Who the hire is (its tmux socket, or its launcher being gone)
// is the caller's to check first. A user's own
// worktree is never taken. The record's paths are resolved once here and git gets exactly those, so the guard and git read the
// same place.
func removeOwnWorktree(git gitRunner, worktrees string, h Hire) bool {
	if git == nil {
		git = runGit
	}
	// Absolute and already clean: real records hold resolved paths, and a spelling
	// like <link>/. or <folder>/.. would hide a link from the check below.
	for _, p := range []string{h.Repo, h.Worktree} {
		if !filepath.IsAbs(p) || p != filepath.Clean(p) {
			return false
		}
	}
	repo, tree := physical(h.Repo), physical(h.Worktree)
	if within(repo, tree) || within(tree, repo) {
		return false
	}
	// The record's own last path part must not be a link: look at it before it is
	// resolved (tree is resolved, so a link can't show there).
	own := filepath.Join(physical(filepath.Dir(h.Worktree)), filepath.Base(h.Worktree))
	if info, err := os.Lstat(own); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	if filepath.Dir(tree) != physical(hiresFolder(repo, worktrees)) || h.Branch != "hire/"+filepath.Base(tree) {
		return false
	}
	// What the worktree really has checked out must be the hire's branch (not
	// renamed, detached or another), or neither it nor any branch is touched.
	if status, out, ok := git(tree, "symbolic-ref", "--short", "-q", "HEAD"); !ok || status != 0 || out != h.Branch {
		return false
	}
	return removeWorkspace(repo, tree, h.Branch, git)
}
