package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXGITROWS2R3 (RT126FIXREVIEW-GITROWS2R2's B1) under the real profile, with newRowsDesk's stand-ins: the hire h1 on
// hire/h1/work, the owner's checkout on feature. At start 1 the desk renames into refs/ or logs/ a folder it made in
// its worktree, holding a link at a ref's or a reflog's own file that leads back to a file f in its worktree (a
// folder renamed in isn't a link made, so rowRefLinks doesn't stop it). The owner then switches to that branch, and
// the next start holds its ref and reflog by their paths. That start is refused, naming the link; were it not, the
// desk's write to f would move the owner's checked-out branch, or write its reflog. These rows name no API of this
// round's, so they run on the tree before it too.
func TestOnAMacALinkAtAHeldRefsOwnFileStopsTheNextStart(t *testing.T) {
	git, python := rowsTools(t)
	ownerTopic := func(d *escapeDesk, owner func(string, ...string) string) {
		owner(d.checkout, "switch", "-q", "-c", "topic/y")
		owner(d.checkout, "commit", "-q", "--allow-empty", "-m", "the owner's topic/y")
		owner(d.checkout, "switch", "-q", "feature")
	}
	for _, c := range []struct {
		name, branch string // the branch the owner switches to before the next start
		held         string // its ref or reflog below the .git, where the desk's link ends up
		has          func(*escapeDesk, func(string, ...string) string)
	}{
		{"the owner's branch (D2)", "topic/y", "refs/heads/topic/y", ownerTopic},
		{"the owner's packed branch (D2p)", "topic/y", "refs/heads/topic/y", func(d *escapeDesk, owner func(string, ...string) string) {
			ownerTopic(d, owner)
			owner(d.checkout, "pack-refs", "--all")
		}},
		{"its reflog", "topic/y", "logs/refs/heads/topic/y", ownerTopic},
		{"a branch the desk made (D)", "rev/x", "refs/heads/rev/x", func(*escapeDesk, func(string, ...string) string) {}},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, owner := newRowsDesk(t, git)
			c.has(d, owner)
			held := filepath.Join(d.git, filepath.FromSlash(c.held))
			folder, f := filepath.Dir(held), filepath.Join(d.root, "f")
			// What f holds, in the desk's worktree: what git reads at the held place, which the desk can read (the
			// commit the branch is at, or the reflog); for its own rev/x, its HEAD.
			was := owner(d.root, "rev-parse", "HEAD") // with its newline, as git writes it
			if _, err := os.Lstat(held); err == nil {
				b, err := os.ReadFile(held)
				if err != nil {
					t.Fatal(err)
				}
				was = string(b)
			} else if !strings.HasPrefix(c.held, "logs/") && c.branch == "topic/y" {
				was = owner(d.checkout, "rev-parse", "topic/y") // packed
			}
			if err := os.WriteFile(f, []byte(was), 0o600); err != nil {
				t.Fatal(err)
			}
			// Start 1: the desk moves the folder aside if it's there, then renames in its own.
			made := filepath.Join(d.root, "d")
			steps := [][]string{{"mkdir", made}, {"symlink", filepath.Join(made, filepath.Base(held)), f}, {"rename", made, folder}}
			if _, err := os.Lstat(folder); err == nil {
				steps = append([][]string{{"rename", folder, filepath.Join(d.root, "old")}}, steps...)
			}
			if said := d.answers(t, python, steps); strings.Join(said, " ") != strings.TrimSpace(strings.Repeat("ok ", len(steps))) {
				t.Logf("the desk's steps at start 1 %q: %q: stopped by the profile", steps, said)
				return
			}
			owner(d.checkout, "switch", "-q", c.branch) // git reads the ref through the link: nothing looks wrong
			rowsPlaces(&d.pl, c.branch)
			err := d.prepared(t) // the next start
			if err == nil || !strings.Contains(err.Error(), held+" is a link") {
				t.Errorf("the next start: %v, want it refused for the link at %s", err, held)
			}
			if err != nil {
				return
			}
			// The desk's turn, under the start's profile.
			if strings.HasPrefix(c.held, "logs/") {
				d.probe(t, python, [][]string{{"write", f}}, []string{"PermissionError"})
				if b, _ := os.ReadFile(held); string(b) != was {
					t.Errorf("the owner's reflog of %s: %q, written by the desk", c.branch, b)
				}
				return
			}
			if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "desk-made commit"); r.status != 0 {
				t.Fatalf("the desk's commit: %+v", r)
			}
			ownRef := filepath.Join(d.git, "refs", "heads", "hire", "h1", "work")
			if r := d.inside(t, nil, "/bin/cp", ownRef, f); r.status != 0 {
				t.Logf("the desk's copy of its commit into f: %+v", r)
			}
			mine := strings.TrimSpace(owner(d.root, "rev-parse", "HEAD"))
			if got, _ := ownerTries(t, d, git, d.checkout, "rev-parse", "HEAD"); strings.TrimSpace(got) == mine {
				t.Errorf("the owner's %s, its checked-out branch, and HEAD: at the desk's commit %s", c.branch, mine)
			}
		})
	}
}

// answers is what sbProbe said of each of steps, run as the desk.
func (d *escapeDesk) answers(t *testing.T, python string, steps [][]string) []string {
	t.Helper()
	encoded, _ := json.Marshal(steps)
	r := d.inside(t, []string{"PYTHONDONTWRITEBYTECODE=1"}, python, "-I", "-c", sbProbe, string(encoded))
	var said []string
	if err := json.Unmarshal([]byte(r.stdout), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %q (%v) %s", r.stdout, err, r.stderr)
	}
	return said
}
