package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXGITROWS2R4 (RT126FIXREVIEW-GITROWS2R3's B1) under the real profile, with newRowsDesk's stand-ins: the hire h1 on
// hire/h1/work, another desk's worktree on hire/h2/work, the owner's checkout on feature, and the owner's own topic/y,
// which no row names until the owner switches to it. These rows name no API of this round's, so they run on the tree
// before it too.

// ownersTopic is the owner's topic/y, a commit of its own, with the owner's checkout back on feature.
func ownersTopic(d *escapeDesk, owner func(string, ...string) string) {
	owner(d.checkout, "switch", "-q", "-c", "topic/y")
	owner(d.checkout, "commit", "-q", "--allow-empty", "-m", "the owner's topic/y")
	owner(d.checkout, "switch", "-q", "feature")
}

// theOwnersHEADStays is the owner's switch to topic/y, then the next start, which is wanted refused; were it not, the
// desk's ordinary commit on its own branch must not move the owner's HEAD.
func theOwnersHEADStays(t *testing.T, d *escapeDesk, git string, owner func(string, ...string) string) {
	t.Helper()
	if out, status := ownerTries(t, d, git, d.checkout, "switch", "-q", "topic/y"); status != 0 {
		t.Logf("the owner's switch to topic/y: %d %s", status, out)
		return
	}
	rowsPlaces(&d.pl, "topic/y")
	if err := d.prepared(t); err == nil {
		t.Errorf("the next start, the owner on topic/y: <nil>, want it refused")
	} else {
		t.Logf("the next start: %v", err)
		return
	}
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "desk-made commit"); r.status != 0 {
		t.Fatalf("the desk's commit on its own branch: %+v", r)
	}
	mine := strings.TrimSpace(owner(d.root, "rev-parse", "HEAD"))
	if got, _ := ownerTries(t, d, git, d.checkout, "rev-parse", "HEAD"); strings.TrimSpace(got) == mine {
		t.Errorf("the owner's HEAD (topic/y): at the desk's commit %s", mine)
	}
}

// SYM, SYMR and a symbolic ref made outside: at start 1 (the owner on feature) the desk points the owner's topic/y at
// its own branch, by git or by a `ref:` file renamed in, alone or in a folder; or one is there already, made outside
// any sandbox (by a desk under an older profile, say). Each of the desk's steps is refused, and topic/y stays the
// owner's; were one to get through, the next start, the owner on topic/y, is refused, and the owner's HEAD stays.
func TestOnAMacADeskCantPointTheOwnersBranchAtItsOwn(t *testing.T) {
	git, python := rowsTools(t)
	const symref = "ref: refs/heads/hire/h1/work\n"
	for _, c := range []struct {
		name string
		// plant is the desk's try at start 1, or the plant made outside: whether topic/y now points at the desk's branch
		plant func(t *testing.T, d *escapeDesk, heads string) bool
	}{
		{"SYM: the desk's git symbolic-ref", func(t *testing.T, d *escapeDesk, heads string) bool {
			r := d.deskGit(t, git, "symbolic-ref", "refs/heads/topic/y", "refs/heads/hire/h1/work")
			if r.status == 0 {
				t.Errorf("the desk's symbolic-ref of the owner's topic/y worked")
			}
			return r.status == 0
		}},
		{"SYMR: a ref: file renamed onto it", func(t *testing.T, d *escapeDesk, heads string) bool {
			mustWrite(t, filepath.Join(d.root, "y"), symref)
			said := d.answers(t, python, [][]string{{"rename", filepath.Join(d.root, "y"), heads + "/topic/y"}})
			if said[0] != "PermissionError" {
				t.Errorf("the desk's rename of a ref: file onto topic/y: %s", said[0])
			}
			return said[0] == "ok"
		}},
		{"SYMR: a ref: file in a folder renamed in", func(t *testing.T, d *escapeDesk, heads string) bool {
			mustWrite(t, filepath.Join(d.root, "d", "y"), symref)
			said := d.answers(t, python, [][]string{{"rename", heads + "/topic", filepath.Join(d.root, "old")},
				{"rename", filepath.Join(d.root, "d"), heads + "/topic"}})
			if said[0] != "PermissionError" || said[1] != "PermissionError" {
				t.Errorf("the desk's renames of refs/heads/topic aside and its own folder in: %q", said)
			}
			return said[1] == "ok"
		}},
		{"a symbolic ref made outside", func(t *testing.T, d *escapeDesk, heads string) bool {
			mustWrite(t, heads+"/topic/y", symref)
			if err := d.prepared(t); err == nil {
				t.Errorf("the start with the owner's topic/y pointing at the desk's branch: <nil>, want it refused")
			}
			return true
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, owner := newRowsDesk(t, git)
			ownersTopic(d, owner)
			topic := owner(d.checkout, "rev-parse", "topic/y")
			heads := filepath.Join(d.git, "refs", "heads")
			if !c.plant(t, d, heads) {
				if got := owner(d.checkout, "rev-parse", "--symbolic-full-name", "topic/y"); got != "refs/heads/topic/y\n" {
					t.Errorf("topic/y now names %q", got)
				}
				if got := owner(d.checkout, "rev-parse", "topic/y"); got != topic {
					t.Errorf("topic/y moved: %s, was %s", got, topic)
				}
				return
			}
			theOwnersHEADStays(t, d, git, owner)
		})
	}
}

// SRL, with the words R3 gave: a link at the owner's topic/y that git reads as a symbolic ref to the desk's branch,
// made outside any sandbox. Start 2, the owner on topic/y, is refused for the link. The owner then does what R3's
// words said, `git symbolic-ref refs/heads/topic/y refs/heads/hire/h1/work`, which writes the same target back as a
// file: start 3 is refused too, and the owner's HEAD stays.
func TestOnAMacTheOwnersSymbolicRefWrittenBackStillStopsTheStart(t *testing.T) {
	git, _ := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	ownersTopic(d, owner)
	y := filepath.Join(d.git, "refs", "heads", "topic", "y")
	if err := os.Remove(y); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("refs/heads/hire/h1/work", y); err != nil {
		t.Fatal(err)
	}
	if out, status := ownerTries(t, d, git, d.checkout, "switch", "-q", "topic/y"); status != 0 {
		t.Fatalf("the owner's switch to topic/y: %d %s", status, out)
	}
	rowsPlaces(&d.pl, "topic/y")
	if err := d.prepared(t); err == nil || !strings.Contains(err.Error(), y+" is a link") {
		t.Errorf("start 2: %v, want it refused for the link at %s", err, y)
	}
	owner(d.checkout, "symbolic-ref", "refs/heads/topic/y", "refs/heads/hire/h1/work")
	if info, err := os.Lstat(y); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("the owner's symbolic-ref left %v %v, want a file", info, err)
	}
	if err := d.prepared(t); err == nil {
		t.Errorf("start 3, topic/y a ref: file pointing at the desk's branch: <nil>, want it refused")
	} else {
		t.Logf("start 3: %v", err)
		return
	}
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "desk-made commit"); r.status != 0 {
		t.Fatalf("the desk's commit: %+v", r)
	}
	mine := strings.TrimSpace(owner(d.root, "rev-parse", "HEAD"))
	if got, _ := ownerTries(t, d, git, d.checkout, "rev-parse", "HEAD"); strings.TrimSpace(got) == mine {
		t.Errorf("the owner's HEAD (topic/y): at the desk's commit %s", mine)
	}
}

// F1's own rule as an allowlist: a desk makes no branch but its own hire/h1/…, by git (branch, switch -c, checkout -b,
// update-ref, symbolic-ref, a copy or a rename of its own) or by a stand-in (a folder, a file, a lock, a file renamed
// in, a reflog), and no tag, nor moves or removes the owner's (tag -f, tag -d, update-ref). It removes no loose ref of
// the owner's either, which would bring back the packed one's older commit. Its own new branch is still its work.
func TestOnAMacADeskMakesNoBranchButItsOwnAndNoTag(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	ownersTopic(d, owner)
	owner(d.checkout, "tag", "v1", "main")
	owner(d.checkout, "pack-refs", "--all")
	owner(d.checkout, "update-ref", "refs/heads/topic/y", "feature") // loose again, over its packed commit
	v1, topic := owner(d.checkout, "rev-parse", "v1"), owner(d.checkout, "rev-parse", "topic/y")
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "the desk's"); r.status != 0 {
		t.Fatalf("the desk's commit on its own branch: %+v", r)
	}
	for _, args := range [][]string{{"branch", "mine"}, {"switch", "-q", "-c", "topic/new"}, {"checkout", "-q", "-b", "fix"},
		{"update-ref", "refs/heads/new", "HEAD"}, {"symbolic-ref", "refs/heads/alias", "refs/heads/hire/h1/work"},
		{"branch", "-c", "hire/h1/work", "copy"}, {"update-ref", "-d", "refs/heads/topic/y"}, {"branch", "-D", "topic/y"},
		{"tag", "v2"}, {"tag", "-f", "v1", "HEAD"}, {"tag", "-d", "v1"}, {"update-ref", "refs/tags/v3", "HEAD"},
		{"branch", "-m", "hire/h1/work", "topic/renamed"}} {
		if r := d.deskGit(t, git, args...); r.status == 0 {
			t.Errorf("the desk's git %q worked", args)
		}
	}
	heads, logs, tags := filepath.Join(d.git, "refs", "heads"), filepath.Join(d.git, "logs", "refs", "heads"), filepath.Join(d.git, "refs", "tags")
	mustWrite(t, filepath.Join(d.root, "ref"), topic)
	d.probe(t, python, [][]string{{"mkdir", heads + "/new"}, {"write", heads + "/new"}, {"write", heads + "/new.lock"},
		{"rename", filepath.Join(d.root, "ref"), heads + "/new"}, {"write", heads + "/topic/z"}, {"unlink", heads + "/topic/y"},
		{"write", heads + "/topic/y"}, {"rename", heads + "/topic", heads + "/topic-aside"}, {"mkdir", logs + "/new"},
		{"write", logs + "/topic/y"}, {"write", tags + "/v4"}, {"write", tags + "/v1"}, {"mkdir", tags + "/x"}},
		times(13, "PermissionError"))
	if got := owner(d.checkout, "for-each-ref", "--format=%(refname)", "refs/heads", "refs/tags"); got !=
		"refs/heads/develop\nrefs/heads/feature\nrefs/heads/hire/h1/work\nrefs/heads/hire/h2/work\nrefs/heads/main\nrefs/heads/topic/y\nrefs/tags/v1\n" {
		t.Errorf("the branches and tags: %q", got)
	}
	if got := owner(d.checkout, "rev-parse", "v1"); got != v1 {
		t.Errorf("v1 moved: %s, was %s", got, v1)
	}
	if got := owner(d.checkout, "rev-parse", "topic/y"); got != topic {
		t.Errorf("topic/y: %s, was %s", got, topic)
	}
	if r := d.deskGit(t, git, "branch", "hire/h1/side"); r.status != 0 {
		t.Errorf("the desk's branch of its own: %+v", r)
	}
}

// The controls: a desk's own work beside the allowlist. Commits on its own branch, an amend, a branch of its own made,
// switched to, merged and deleted, a fetch from origin (a stand-in with a tag the repo doesn't have yet:
// DeskGitConfig follows no tags, and none is made), a merge of main and a rebase onto it, a change committed, a reset,
// its reflog's expiry and `gc --auto`. (A stash and its pop were here until FXGITROWS2R5 denied refs/stash.) Then the next start goes on. Without DeskGitConfig the fetch stops at the tag:
// that is why it follows none.
func TestOnAMacADesksOwnBranchWorkRunsBesideTheAllowlist(t *testing.T) {
	git, _ := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	up := filepath.Join(d.home, "up.git")
	owner(d.home, "clone", "-q", "--bare", d.checkout, up)
	owner(up, "tag", "v-up", "main")
	owner(d.checkout, "remote", "add", "origin", up)
	owner(d.checkout, "commit", "-q", "--allow-empty", "-m", "the owner's next")
	owner(d.checkout, "branch", "-f", "main", "feature")
	hireGit := func(args ...string) ran {
		t.Helper()
		r := d.deskGit(t, git, args...)
		if r.status != 0 {
			t.Errorf("the desk's git %q: %+v", args, r)
		}
		return r
	}
	mustWrite(t, filepath.Join(d.root, "work.go"), "package work")
	hireGit("add", "work.go")
	hireGit("commit", "-q", "-m", "the desk's work")
	hireGit("commit", "-q", "--amend", "-m", "the desk's work, amended")
	hireGit("branch", "hire/h1/side")
	hireGit("switch", "-q", "hire/h1/side")
	hireGit("commit", "-q", "--allow-empty", "-m", "on the side")
	hireGit("switch", "-q", "hire/h1/work")
	hireGit("merge", "-q", "--no-ff", "--no-edit", "hire/h1/side")
	hireGit("branch", "-D", "hire/h1/side")
	hireGit("fetch", "-q", "origin")
	if got := owner(d.checkout, "tag", "--list"); got != "" {
		t.Errorf("the desk's fetch made tags %q", got)
	}
	hireGit("merge", "-q", "--no-edit", "main")
	hireGit("rebase", "-q", "main")
	mustWrite(t, filepath.Join(d.root, "work.go"), "package work // changed")
	hireGit("commit", "-q", "-am", "the change") // no stash since FXGITROWS2R5 (refs/stash: gitrows2r5_darwin_test.go)
	hireGit("reset", "-q", "--hard", "HEAD~1")
	hireGit("reflog", "expire", "--expire=now", "refs/heads/hire/h1/work")
	hireGit("gc", "--auto")
	if got := owner(d.checkout, "rev-parse", "main"); got != owner(d.checkout, "rev-parse", "feature") {
		t.Errorf("main moved: %s", got)
	}
	if err := d.prepared(t); err != nil {
		t.Errorf("the next start: %v", err)
	}
	// Without DeskGitConfig: the fetch takes the tag too, which no desk writes.
	r := d.inside(t, gitTestEnv(git), git, "fetch", "-q", "origin")
	if r.status == 0 || !strings.Contains(r.stderr, "refs/tags/v-up") {
		t.Errorf("the desk's fetch with tags followed: %+v, want git to stop at refs/tags/v-up", r)
	}
}
