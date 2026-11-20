//go:build !windows

package desktop

import (
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXGITROWS2R8 (RT126FIXREVIEW-GITROWS2R7's B1 and O3): a link at one of git's own names at the top of the .git that
// leads where a desk can write, or can't be followed, says what it is, where it leads, why it stops the start, and how
// to fix it: hooks by naming the folder with core.hooksPath, the repository's own files by a copy in the link's place,
// git's state by removing the link. A commit's id says "at most" an "ambiguous" warning.
func TestALinkAtGitsOwnNameSaysWhereItLeadsAndHowToFixIt(t *testing.T) {
	why := "so a desk could change what your git runs or applies there, and the desk didn't start"
	for _, c := range []struct {
		name     string
		e        sandbox.RefPlaceError
		say, not []string
	}{
		{"rr-cache", sandbox.RefPlaceError{At: "/r/.git/rr-cache", Ref: "rr-cache", Link: true, Leads: "/w/rr"},
			[]string{"/r/.git/rr-cache, in the repository's .git folder, is a link to /w/rr, where a desk can write",
				"the conflict resolutions it reuses in a merge (rerere)", why, "ls -l '/r/.git/rr-cache'", "rm '/r/.git/rr-cache'",
				"git makes its own rr-cache when it needs one", "start the desk again"},
			[]string{"core.hooksPath", "cp -R"}},
		{"MERGE_AUTOSTASH", sandbox.RefPlaceError{At: "/r/.git/MERGE_AUTOSTASH", Ref: "MERGE_AUTOSTASH", Link: true, Leads: "/w/as"},
			[]string{"is a link to /w/as", "When a merge ends, git puts the changes it keeps there back into your files", why, "rm '/r/.git/MERGE_AUTOSTASH'"}, nil},
		{"FETCH_HEAD", sandbox.RefPlaceError{At: "/r/.git/FETCH_HEAD", Ref: "FETCH_HEAD", Link: true, Leads: "/w/fh"},
			[]string{"is a link to /w/fh", "Git reads the result of your last fetch there, and git merge FETCH_HEAD merges it", why, "rm '/r/.git/FETCH_HEAD'"}, nil},
		{"hooks", sandbox.RefPlaceError{At: "/r/.git/hooks", Ref: "hooks", Link: true, Leads: "/r/.githooks"},
			// "Git runs the hooks it finds there" since FXGITROWS2R9 (RT126FIXREVIEW-GITROWS2R8's O1).
			[]string{"is a link to /r/.githooks, where a desk can write", "Git runs the hooks it finds there", why, "rm '/r/.git/hooks'",
				"git config core.hooksPath", "Smolcorp keeps every desk out of the folder your git config names for hooks"},
			[]string{"git makes its own", "cp -R"}},
		{"config", sandbox.RefPlaceError{At: "/r/.git/config", Ref: "config", Link: true, Leads: "/u/dotfiles/app-config"},
			[]string{"is a link to /u/dotfiles/app-config", "Git reads this repository's settings there, and they can name programs for it to run", why,
				"put a copy of it in the link's place (rm '/r/.git/config', then cp -R '/u/dotfiles/app-config' '/r/.git/config')"},
			[]string{"git makes its own", "core.hooksPath"}},
		{"a link that leads nowhere", sandbox.RefPlaceError{At: "/r/.git/rr-cache", Ref: "rr-cache", Link: true, Leads: "/gone", Unfollowed: true},
			[]string{"is a link to /gone, which isn't there or can't be read", "Smolcorp can't tell whether a desk could write there",
				why, "ls -l '/r/.git/rr-cache'", "rm '/r/.git/rr-cache'", "put back what it leads to"},
			[]string{"where a desk can write"}},
		{"O3: a commit's id", sandbox.RefPlaceError{At: "/r/.git/feature", Ref: "feature", Commit: true},
			[]string{"with at most an \"ambiguous\" warning"}, []string{"with only"}},
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
	for _, c := range []struct {
		e    sandbox.RefPlaceError
		want string
	}{
		{sandbox.RefPlaceError{At: "/r/.git/hooks", Ref: "hooks", Link: true, Leads: "/r/.githooks"}, "/r/.git/hooks is a link to /r/.githooks, where a desk can write"},
		{sandbox.RefPlaceError{At: "/r/.git/hooks", Ref: "hooks", Link: true, Leads: "/gone", Unfollowed: true}, "/r/.git/hooks is a link to /gone, which can't be followed to a place no desk can write"},
	} {
		if got := c.e.Error(); got != c.want {
			t.Errorf("Error(): %q, want %q", got, c.want)
		}
	}
}
