//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RT126WATCHHELDR2 N1, the reviewer's pT with no hook: the app's own STATUS.md watches are made while the path below
// a hire's place is plain, and a link put down after that sync, before the look, still reads nothing of another
// root's STATUS.md. Each look reads a built-in only while its path still resolves to itself, as a held watch does.
// The same watch reads its own file once that is plain again.
func TestABuiltinWatchReadsNoOtherRootsStatusThroughALinkMadeAfterItsSync(t *testing.T) {
	for _, way := range []string{".team", "STATUS.md"} {
		t.Run(way, func(t *testing.T) {
			d, _, _, proj, elsewhere := heldNames(t)
			notes := filepath.Join(elsewhere, "notes")
			if err := os.MkdirAll(notes, 0o755); err != nil {
				t.Fatal(err)
			}
			appendLine(t, filepath.Join(notes, "STATUS.md"), "DONE: bea's line")
			builtinsOnFor(t, d)
			// The sync sees no proj/.team and no proj/STATUS.md: Ada's and Cy's built-ins are made there.
			d.watchers.syncBuiltins()
			want := filepath.Join(proj, way)
			if way == ".team" {
				want = filepath.Join(proj, ".team", "STATUS.md")
			}
			if got := builtinTargets(d, "ad0001"); !strings.Contains(strings.Join(got, "\n"), "status "+want) {
				t.Fatalf("Ada's built-ins: %q, want one on %s", got, want)
			}
			// Then the link, before the look.
			link, to := filepath.Join(proj, way), notes
			if way == "STATUS.md" {
				to = filepath.Join(notes, "STATUS.md")
			}
			if err := os.Symlink(to, link); err != nil {
				t.Fatal(err)
			}
			fires, _ := lookOnce(d.watchers)
			for _, f := range fires {
				if strings.Contains(f.detail, "bea's line") || f.folder == notes {
					t.Errorf("a built-in read Bea's STATUS.md through %s: %+v", link, f)
				}
			}
			// The link gone and the file plain, the same built-in reads it.
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
			appendLine(t, filepath.Join(proj, "STATUS.md"), "DONE: ada's line")
			if way == ".team" {
				if err := os.MkdirAll(filepath.Join(proj, ".team"), 0o755); err != nil {
					t.Fatal(err)
				}
				appendLine(t, filepath.Join(proj, ".team", "STATUS.md"), "DONE: ada's line")
			}
			fires, _ = lookOnce(d.watchers)
			read := false
			for _, f := range fires {
				read = read || strings.Contains(f.detail, "ada's line")
				if strings.Contains(f.detail, "bea's line") {
					t.Errorf("a built-in read Bea's line: %+v", f)
				}
			}
			if !read {
				t.Errorf("the plain %s: no built-in read it (%+v)", want, fires)
			}
		})
	}
}
