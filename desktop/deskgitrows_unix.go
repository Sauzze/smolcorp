//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// deskBranches are the branches FXGITROWS2's rows hold for desk d in the repo whose shared .git is gitDir (RT126SWEEP2's
// F1): the one the owner's checkout is on now, for a desk in a worktree of it, from its HEAD in the shared .git, which
// no desk writes, read as a file or as a link git wrote for core.preferSymlinkRefs (FXGITROWS2R4: read through, a link
// gave a commit's id, and the branch wasn't held), "" when detached or a name the Policy doesn't take; the start
// follows that branch on if it is a symbolic ref (CheckRefPlaces); and the desk's own besides hire/<its id>: the
// folder hireBranch names its branches under, and, for a worktree, the one a hire made before branches had a folder
// of their own was on (hire/<the worktree's name>, hireworktree.go).
func deskBranches(d deskAt, gitDir string) (checkout string, own []string) {
	if d.Checkout != "" {
		if name, ok := strings.CutPrefix(headRef(gitDir), "refs/heads/"); ok && sandbox.BranchName(name) {
			checkout = name
		}
	}
	for _, name := range []string{hireSlug(d.ID), filepath.Base(d.Root)} {
		if name != "" && !strings.Contains(name, "/") && sandbox.BranchName("hire/"+name) && !slices.Contains(own, "hire/"+name) {
			own = append(own, "hire/"+name)
		}
		if d.Checkout == "" {
			break
		}
	}
	return checkout, own
}

// headRef is the ref the HEAD of the shared .git gitDir names: "ref: <it>" in a file, or a link's text; "" when HEAD
// is detached or can't be read.
func headRef(gitDir string) string {
	head := filepath.Join(gitDir, "HEAD")
	if to, err := os.Readlink(head); err == nil {
		return to
	}
	data, err := os.ReadFile(head)
	if err != nil {
		return ""
	}
	if name, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "ref: "); ok {
		return name
	}
	return ""
}
