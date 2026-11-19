//go:build !windows

package desktop

import (
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXGITROWS2R7 (RT126FIXREVIEW-GITROWS2R6's B1 and O2): a name at the top of the .git that holds a commit's id, is a
// link (a file's or a folder's) or has a second name says what it is, that git reads it before the branch of that
// name, and how to remove it. A link in refs/ still says to put what it leads to in its place.
func TestANameAtTheTopOfTheGitThatIsALinkOrACommitSaysHowToRemoveIt(t *testing.T) {
	for _, c := range []struct {
		name     string
		e        sandbox.RefPlaceError
		say, not []string
	}{
		{"a commit's id", sandbox.RefPlaceError{At: "/r/.git/feature", Ref: "feature", Commit: true},
			[]string{"feature, in the repository's .git folder, isn't a branch: it holds a commit's id", "before any branch called feature",
				"cat '/r/.git/feature'", "rm '/r/.git/feature'", "the desk didn't start"},
			[]string{"update-ref", "link"}},
		{"a link", sandbox.RefPlaceError{At: "/r/.git/feature", Ref: "feature", Link: true},
			[]string{"/r/.git/feature is a link, where git looks for a branch's name before its own branches", "before a branch called feature, or one whose name starts feature/",
				"ls -l '/r/.git/feature'", "remove the link (rm '/r/.git/feature')", "the desk didn't start"},
			[]string{"in the link's place", "update-ref"}},
		{"a folder linked", sandbox.RefPlaceError{At: "/r/.git/hire", Ref: "hire", Link: true},
			[]string{"/r/.git/hire is a link", "one whose name starts hire/", "check what's in it", "rm '/r/.git/hire'"},
			[]string{"in the link's place"}},
		{"a hard link", sandbox.RefPlaceError{At: "/r/.git/hire/h2/work", Ref: "hire/h2/work", Hard: true},
			[]string{"hire/h2/work, in the repository's .git folder, isn't a branch, and it has another name elsewhere (a hard link)",
				"before any branch called hire/h2/work", "rm '/r/.git/hire/h2/work'", "the desk didn't start"},
			[]string{"a copy of its own"}},
		{"a link in refs/, as before", sandbox.RefPlaceError{At: "/r/.git/refs/heads/topic", Ref: "refs/heads/topic", Link: true},
			[]string{"put it in the link's place"}, []string{"rm '"}},
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
	if got := (&sandbox.RefPlaceError{At: "/r/.git/feature", Ref: "feature", Commit: true}).Error(); got != "/r/.git/feature holds a commit's id, which git takes for the branch feature" {
		t.Errorf("Error(): %q", got)
	}
}
