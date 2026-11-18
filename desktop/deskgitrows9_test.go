//go:build !windows

package desktop

import (
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXGITROWS2R9 (RT126FIXREVIEW-GITROWS2R8's B1, B2 and O1): a link at one of git's own names whose way goes through a
// link a desk can change says which link that is and that where it leads is then the desk's to choose; one of git's
// own files with a second name says so and how to fix it, removing git's state, giving the owner's own files a copy
// of their own; and what git reads through each name is a plain sentence, hooks and lfs by name, the rest a fallback.
func TestALinkByWayOfADesksLinkAndAHardLinkAtGitsOwnFileSayWhatToDo(t *testing.T) {
	why := "so a desk could change what your git runs or applies there"
	for _, c := range []struct {
		name     string
		e        sandbox.RefPlaceError
		say, not []string
	}{
		{"B1: by way of a link in the desk's worktree", sandbox.RefPlaceError{At: "/r/.git/rr-cache", Ref: "rr-cache", Link: true, Leads: "/u/gm", Via: "/w/lnk"},
			[]string{"/r/.git/rr-cache, in the repository's .git folder, is a link whose way goes through another link, /w/lnk, which a desk can change",
				"where it leads is the desk's to choose, wherever it leads now", "Git reads the conflict resolutions it reuses in a merge (rerere) there",
				why, "ls -l '/r/.git/rr-cache'", "rm '/r/.git/rr-cache'", "git makes its own rr-cache when it needs one", "start the desk again"},
			[]string{"where a desk can write", "can't be read"}},
		{"B1: the link itself is the desk's", sandbox.RefPlaceError{At: "/r/.git/FETCH_HEAD", Ref: "FETCH_HEAD", Link: true, Leads: "/u/fh", Via: "/r/.git/FETCH_HEAD"},
			[]string{"is a link a desk can change or remove", "Git reads there what your last fetch brought", why, "Git doesn't make links there", "rm '/r/.git/FETCH_HEAD'"},
			[]string{"goes through another link"}},
		{"B1: by way of a desk's link, a config that can't be followed", sandbox.RefPlaceError{At: "/r/.git/config", Ref: "config", Link: true, Via: "/w/lnk"},
			[]string{"goes through another link, /w/lnk", "put a copy of what it leads to in the link's place"}, []string{"cp -R", "a place Smolcorp can't read"}},
		{"O1: hooks", sandbox.RefPlaceError{At: "/r/.git/hooks", Ref: "hooks", Link: true, Leads: "/r/.githooks"},
			[]string{"Git runs the hooks it finds there, " + why}, []string{"reads the hooks it runs through it"}},
		{"O1: lfs", sandbox.RefPlaceError{At: "/r/.git/lfs", Ref: "lfs", Link: true, Leads: "/w/lfs"},
			[]string{"Git reads the large files git-lfs keeps there, " + why, "rm '/r/.git/lfs'", "git config lfs.storage"}, []string{"its own lfs"}},
		{"O1: a name with no sentence of its own", sandbox.RefPlaceError{At: "/r/.git/svn", Ref: "svn", Link: true, Leads: "/w/svn"},
			[]string{"Git reads its own files there, " + why}, []string{"reads its own svn"}},
		{"B2: FETCH_HEAD", sandbox.RefPlaceError{At: "/r/.git/FETCH_HEAD", Ref: "FETCH_HEAD", Hard: true, Own: true},
			[]string{"/r/.git/FETCH_HEAD, in the repository's .git folder, has another name elsewhere (a hard link)",
				"Git reads there what your last fetch brought", "by writing that other name", "Git doesn't make hard links there",
				"cat '/r/.git/FETCH_HEAD'", "rm '/r/.git/FETCH_HEAD'", "start the desk again"},
			[]string{"isn't a branch", "cp "}},
		{"B2: MERGE_AUTOSTASH", sandbox.RefPlaceError{At: "/r/.git/MERGE_AUTOSTASH", Ref: "MERGE_AUTOSTASH", Hard: true, Own: true},
			[]string{"has another name elsewhere (a hard link)", "the changes it puts back into your files when a merge ends", "rm '/r/.git/MERGE_AUTOSTASH'"},
			[]string{"isn't a branch"}},
		{"B2: packed-refs keeps the owner's branches", sandbox.RefPlaceError{At: "/r/.git/packed-refs", Ref: "packed-refs", Hard: true, Own: true},
			[]string{"has another name elsewhere (a hard link)", "Git reads it as one of its own files",
				"cp '/r/.git/packed-refs' '/r/.git/packed-refs.own', then mv '/r/.git/packed-refs.own' '/r/.git/packed-refs'"},
			[]string{"rm '", "isn't a branch"}},
		{"a name that isn't git's own, a hard link: as before", sandbox.RefPlaceError{At: "/r/.git/feature", Ref: "feature", Hard: true},
			[]string{"isn't a branch", "git reads it before any branch called feature", "rm '/r/.git/feature'"}, nil},
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
		{sandbox.RefPlaceError{At: "/r/.git/rr-cache", Ref: "rr-cache", Link: true, Leads: "/u/gm", Via: "/w/lnk"}, "/r/.git/rr-cache is a link by way of /w/lnk, a link a desk can change"},
		{sandbox.RefPlaceError{At: "/r/.git/FETCH_HEAD", Ref: "FETCH_HEAD", Link: true, Via: "/r/.git/FETCH_HEAD"}, "/r/.git/FETCH_HEAD is a link a desk can change"},
		{sandbox.RefPlaceError{At: "/r/.git/FETCH_HEAD", Ref: "FETCH_HEAD", Hard: true, Own: true}, "/r/.git/FETCH_HEAD has another name (a hard link), where the sandbox holds refs by their paths"},
	} {
		if got := c.e.Error(); got != c.want {
			t.Errorf("Error(): %q, want %q", got, c.want)
		}
	}
}
