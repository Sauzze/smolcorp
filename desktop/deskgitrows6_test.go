//go:build !windows

package desktop

import (
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXGITROWS2R6 (RT126FIXREVIEW-GITROWS2R5's O1 and O2): the branch sentence is one rule for every engine's hire on its
// own branch. The General Manager is told to ask the owner; an agent no sandbox holds (the launcher's engine hire, T3
// Code's) is told nothing, since no row holds its git.
func TestTheBranchSentenceTellsTheGMToAskTheOwnerAndAnUnheldAgentNothing(t *testing.T) {
	gm := deskBranchWords("darwin", true, true, "hire/0123456789ab/general-manager")
	if !strings.HasPrefix(gm, "You can commit only to your own branches, hire/0123456789ab/general-manager and any new one under hire/0123456789ab/: ") ||
		!strings.HasSuffix(gm, ", and ask the owner when you need another branch or a tag.") || strings.Contains(gm, "General Manager") {
		t.Errorf("the GM's: %q", gm)
	}
	if hire := deskBranchWords("darwin", true, false, "hire/0123456789ab/janitor"); !strings.HasSuffix(hire, ", and ask the General Manager or the owner when you need another branch or a tag.") {
		t.Errorf("a hire's: %q", hire)
	}
	for _, gm := range []bool{false, true} {
		if got := deskBranchWords("darwin", false, gm, "hire/0123456789ab/janitor"); got != "" {
			t.Errorf("an agent no sandbox holds (gm %v): %q", gm, got)
		}
	}
}

// FXGITROWS2R6 (RT126FIXREVIEW-GITROWS2R5's B1 and N2): a HEAD with a second name, the owner checkout's and another
// worktree's, and a name at the top of the .git that git reads before a branch, each say where it is and what to do.
func TestARefusedHEADOrNameAtTheTopOfTheGitSaysWhatToDo(t *testing.T) {
	for _, c := range []struct {
		name     string
		e        sandbox.RefPlaceError
		say, not []string
	}{
		{"the owner checkout's HEAD, a hard link", sandbox.RefPlaceError{At: "/r/.git/HEAD", Ref: "HEAD", Hard: true},
			[]string{"Your checkout's HEAD (/r/.git/HEAD) has another name elsewhere (a hard link)", "git symbolic-ref HEAD", "a copy of its own", "the desk didn't start"},
			[]string{"before its own branches", "a branch Smolcorp keeps"}},
		{"another worktree's HEAD, a hard link", sandbox.RefPlaceError{At: "/r/.git/worktrees/review/HEAD", Ref: "worktrees/review/HEAD", Hard: true},
			[]string{"The HEAD of the worktree git knows as review (/r/.git/worktrees/review/HEAD) has another name elsewhere (a hard link)", "git worktree list", "a copy of its own", "the desk didn't start"},
			[]string{"Your checkout", "a branch Smolcorp keeps"}},
		{"another worktree's HEAD, by way of a per-worktree ref", sandbox.RefPlaceError{At: "/r/.git/worktrees/review/HEAD", Ref: "worktrees/review/HEAD", Symref: "refs/worktree/w"},
			[]string{"points at refs/worktree/w, which Smolcorp can't follow to a ref no desk can move", "could move that worktree too", "git worktree list"},
			[]string{"would move"}},
		{"a name at the top of the .git", sandbox.RefPlaceError{At: "/r/.git/hire/h2/work", Ref: "hire/h2/work", Symref: "refs/heads/hire/h1/work", Moves: "refs/heads/hire/h1/work"},
			[]string{"hire/h2/work, in the repository's .git folder, isn't a branch: git reads it before any branch called hire/h2/work",
				"which a desk can move", "git log refs/heads/hire/h1/work", "rm '/r/.git/hire/h2/work'", "the desk didn't start"},
			[]string{"update-ref", "isn't a ref of its own"}},
		{"a name at the top of the .git, to a checkout's HEAD", sandbox.RefPlaceError{At: "/r/.git/feature", Ref: "feature", Symref: "HEAD"},
			[]string{"feature, in the repository's .git folder, isn't a branch", "could change what your git takes for feature", "rm '/r/.git/feature'"},
			[]string{"update-ref"}},
	} {
		words := refPlaceRefusal(&c.e)
		for _, s := range c.say {
			if !strings.Contains(words, s) {
				t.Errorf("%s: %q doesn't say %q", c.name, words, s)
			}
		}
		for _, s := range c.not {
			if strings.Contains(words, s) {
				t.Errorf("%s: %q says %q", c.name, words, s)
			}
		}
	}
}
