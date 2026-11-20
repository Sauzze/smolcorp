package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// FXGITROWS2R2 (RT126FIXREVIEW-GITROWS2R1's B1) under the real profile, with newRowsDesk's stand-ins: the hire h1 on
// hire/h1/work, the owner's checkout on feature, the default branch develop. Each branch, reflog and base branch's
// name the Policy holds is held by its path, which Seatbelt checks where its links lead. These rows name no API of
// this round's, so they run on the tree before it too; its Policy rows are in gitrows2r2_test.go.

// (b): no desk makes a link anywhere in refs/ or logs/: at a folder no row holds yet (a branch's, a reflog's, a tag's,
// a remote's), in its own branch's folder, or a hard link. Its own work there still runs: a commit, a branch of its own
// with a slash (hire/h1/…, since FXGITROWS2R4 its only kind), a fetch.
func TestOnAMacNoDeskMakesALinkInRefsOrLogs(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	refs, logs := filepath.Join(d.git, "refs"), filepath.Join(d.git, "logs", "refs", "heads")
	lt := filepath.Join(d.root, "lt")
	if err := os.Mkdir(lt, 0o700); err != nil {
		t.Fatal(err)
	}
	d.probe(t, python, [][]string{{"symlink", refs + "/heads/topic", lt}, {"symlink", logs + "/topic", lt},
		{"symlink", refs + "/tags/team", lt}, {"symlink", refs + "/remotes/up", lt}, {"symlink", refs + "/heads/hire/h1/alias", lt},
		{"symlink", logs + "/hire/h1/alias", lt}, {"link", filepath.Join(d.root, "README"), refs + "/heads/hard"}}, times(7, "PermissionError"))
	for _, at := range []string{refs + "/heads/topic", logs + "/topic", refs + "/tags/team", refs + "/remotes/up"} {
		if _, err := os.Lstat(at); err == nil {
			t.Errorf("%s is there", at)
		}
	}
	// Its own work.
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "the desk's"); r.status != 0 {
		t.Errorf("the desk's commit: %+v", r)
	}
	if r := d.deskGit(t, git, "branch", "hire/h1/topic/mine"); r.status != 0 {
		t.Errorf("the desk's branch hire/h1/topic/mine: %+v", r)
	}
	if got, want := owner(d.checkout, "rev-parse", "hire/h1/topic/mine"), owner(d.root, "rev-parse", "HEAD"); got != want {
		t.Errorf("hire/h1/topic/mine: %s, want the desk's commit %s", got, want)
	}
	owner(d.checkout, "remote", "add", "origin", d.checkout) // a stand-in remote: the repo itself, on this disk
	if r := d.deskGit(t, git, "fetch", "-q", "origin"); r.status != 0 || strings.Contains(r.stderr, "error") {
		t.Errorf("the desk's fetch: %+v", r)
	}
}

// (a): a link at a folder on the way to a ref or a reflog the next start holds, made before (by a desk before
// rowRefLinks, or by a program outside any sandbox), refuses that start, and nothing is made through it, no keeper
// above all: the owner's later topic/x, its reflog, a default branch team/main the owner sets, and that name beside the
// .git, which the owner's git reads before refs/heads/team/main. Without the refusal, the desk writes each of them by
// its held path, which leads into its worktree.
func TestOnAMacALinkOnTheWayToAHeldRefStopsTheNextStart(t *testing.T) {
	git, python := rowsTools(t)
	onTopic := func(d *escapeDesk, owner func(string, ...string) string) {
		owner(d.checkout, "switch", "-q", "-c", "topic/x")
		rowsPlaces(&d.pl, "topic/x")
	}
	defaultTeam := func(d *escapeDesk, owner func(string, ...string) string) {
		owner(d.checkout, "branch", "team/main")
		owner(d.checkout, "update-ref", "refs/remotes/origin/team/main", "team/main")
		owner(d.checkout, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/team/main")
		d.pl.DefaultBranchNow = DefaultBranch(d.git) // as deskGitPlaces reads it at each start (N1)
		if d.pl.DefaultBranchNow != "team/main" {
			t.Fatalf("the default branch now: %q", d.pl.DefaultBranchNow)
		}
	}
	for _, c := range []struct {
		name, link, held string // the link, to lt in the desk's worktree, and the place the next start holds, below the .git
		then             func(*escapeDesk, func(string, ...string) string)
		written          []string // what the owner's git wrote through the link
	}{
		{"the owner's branch", "refs/heads/topic", "refs/heads/topic/x", onTopic, []string{"x"}},
		{"its reflog", "logs/refs/heads/topic", "logs/refs/heads/topic/x", onTopic, []string{"x"}},
		{"a slash default branch", "refs/heads/team", "refs/heads/team/main", defaultTeam, []string{"main"}},
		{"its name beside the .git", "team", "team/main", defaultTeam, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, owner := newRowsDesk(t, git)
			lt, link := filepath.Join(d.root, "lt"), filepath.Join(d.git, filepath.FromSlash(c.link))
			if err := os.Mkdir(lt, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(lt, link); err != nil { // made outside any sandbox
				t.Fatal(err)
			}
			c.then(d, owner)
			err := d.prepared(t) // the next start
			if err == nil || !strings.Contains(err.Error(), link+" is a link") {
				t.Errorf("the next start: %v, want it refused for the link at %s", err, link)
			}
			if names := namesIn(t, lt); !slices.Equal(names, c.written) {
				t.Errorf("made through the link, in the desk's worktree: %q, want only git's %q", names, c.written)
			}
			if err == nil {
				d.probe(t, python, [][]string{{"write", filepath.Join(d.git, filepath.FromSlash(c.held))}}, []string{"PermissionError"})
			}
		})
	}
}

// prepared is prepare's start, its error an answer: PrepareProfile of the desk's Policy for its Places, with the lists
// as a start would give them.
func (d *escapeDesk) prepared(t *testing.T) error {
	t.Helper()
	svc := Services{OwnSocket: d.ownSocket, DeniedPorts: DeniedPorts(false),
		Sockets: SocketPaths(Sockets("darwin"), d.home, func(string) string { return "" }, false, false)}
	for _, m := range MachServices() {
		svc.Mach = append(svc.Mach, m.Name)
	}
	for _, c := range IOKitClasses() {
		svc.IOKit = append(svc.IOKit, IOKitOpen{Class: c.Class, UserClient: c.UserClient})
	}
	return PrepareProfile(mustNew(t, d.pl), d.profile, svc)
}

// namesIn is the names in folder, sorted.
func namesIn(t *testing.T, folder string) []string {
	t.Helper()
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
