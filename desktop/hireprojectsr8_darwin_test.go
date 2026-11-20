package desktop

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// FXHIREPROJECTSR8 (RT126FIXREVIEW-HIREPROJECTSR7 B1, B2, N1, N2, O2) under the real profile, with stand-ins only.
//   - B1: a link below a desk's place to a folder that holds a repo a desk can change (one level up, or more, or
//     through another link), or into a package manager's folder that holds one, refuses the next start.
//   - N1: the owner's own clones and mirrors in a package manager's folders are held from a desk; a clone it makes
//     during its session is its own until its next start.
//   - B2: a gitlink a hire commits in its own worktree over a repo it made there refuses its next start; a real
//     submodule the project has still starts (O2: the worktree hire's next start runs, its git in its sandbox).
//   - N2: a plain hire's end takes its worktree and branch away, with its recap, by every route its launcher ends.

// r8LinkWords is the words of a start refused for link, which leads to a folder that holds repo.
func r8LinkWords(place, link, repo string) string {
	return "This hire was set to work in " + tilde(place) + ", but " + tilde(link) + " is a link to a folder that holds a git repository a hire can change, " + tilde(repo) + ", and your own git runs that repository's hooks when it works there through the link, so it didn't start. Remove the link " + tilde(link) + ", then start it again."
}

// r8RepoLinkWords is the words of a start refused for link, which leads to repo itself.
func r8RepoLinkWords(place, link, repo string) string {
	return "This hire was set to work in " + tilde(place) + ", but " + tilde(link) + " is a link to a git repository a hire can change, " + tilde(repo) + ", and your own git runs that repository's hooks when it works in " + tilde(link) + ", so it didn't start. Remove the link " + tilde(link) + ", then start it again."
}

// R7-1: the desk makes a repo in a folder outside its place and links its place to that folder, not to the repo: its
// tries pass, its next start is refused. So is a link two folders up, and one through another link. A link to a folder
// that holds an owner's project no desk writes, and to one that holds no repo, start (the controls), as does the place
// once the link is gone.
func TestALinkToAFolderThatHoldsARepoStopsItsStart(t *testing.T) {
	l := newHireProjectsLayout(t)
	stage := filepath.Join(homeDir, "stage")
	ownerMade(t, homeDir, "stage/box/", "stage/far/a/", "stage/hop/", "stage/plain/notes.md")
	r7Repo(t, filepath.Join(l.projects, "group", "p"))
	h := Hire{ID: "r8l", Name: "Janitor", Role: "janitor", Folder: keptPath(l.lib), State: "working"}
	profile := r6Desk(t, l, h.ID, l.lib)
	got := sandboxedSh(t, profile, l.lib, r6Tries+`t sh -c 'mkdir "$0/z" && cd "$0/z" && /usr/bin/git init -q' "$2/box"
t hook "$2/box/z/.git/hooks/post-checkout" "$3"
t ln -s "$2/box" "$1/box"`, l.lib, stage, l.planted)
	if strings.Join(got, " ") != "ok ok ok" {
		t.Fatalf("the desk's tries: %q", got)
	}
	box, z := filepath.Join(l.lib, "box"), filepath.Join(stage, "box", "z")
	if got, want := r7Start(t, h), r8LinkWords(l.lib, box, z); got != want {
		t.Errorf("its next start:\n got %q\nwant %q", got, want)
	}
	if err := os.Remove(box); err != nil {
		t.Fatal(err)
	}
	r7Repo(t, filepath.Join(stage, "far", "a", "b", "y"))
	if err := os.Symlink(filepath.Join(stage, "hop"), filepath.Join(stage, "hop-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(stage, "far"), filepath.Join(stage, "hop", "in")); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, link, to, repo string }{
		{"two folders up", "far", filepath.Join(stage, "far"), filepath.Join(stage, "far", "a", "b", "y")},
		{"through another link", "hop", filepath.Join(stage, "hop-link"), filepath.Join(stage, "far", "a", "b", "y")},
		{"a folder that holds an owner's project no desk writes", "group", filepath.Join(l.projects, "group"), ""},
		{"a folder that holds no repo", "plain", filepath.Join(stage, "plain"), ""},
	} {
		link := filepath.Join(l.lib, c.link)
		if err := os.Symlink(c.to, link); err != nil {
			t.Fatal(err)
		}
		want := ""
		if c.repo != "" {
			want = r8LinkWords(l.lib, link, c.repo)
		}
		if got := r7Start(t, h); got != want {
			t.Errorf("%s: its start:\n got %q\nwant %q", c.name, got, want)
		}
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
	}
	if words := r7Start(t, h); words != "" {
		t.Errorf("its start with the links gone: %s", words)
	}
}

// R7-2: the desk makes a repo in a package manager's folder in its place, which the .git rule leaves it, and links to
// it from its place: the next start is refused, and so it is for a link to the package manager's folder that holds it.
// With the link gone it starts, and that repo is held from then on.
func TestALinkIntoAPackageManagersFolderStopsItsStart(t *testing.T) {
	l := newHireProjectsLayout(t)
	h := Hire{ID: "r8p", Name: "Janitor", Role: "janitor", Folder: keptPath(l.lib), State: "working"}
	profile := r6Desk(t, l, h.ID, l.lib)
	got := sandboxedSh(t, profile, l.lib, r6Tries+`t mkdir -p "$1/.build/checkouts/evil"
t sh -c 'cd "$0/.build/checkouts/evil" && /usr/bin/git init -q' "$1"
t hook "$1/.build/checkouts/evil/.git/hooks/post-checkout" "$2"
t ln -s "$1/.build/checkouts/evil" "$1/tools"`, l.lib, l.planted)
	if strings.Join(got, " ") != "ok ok ok ok" {
		t.Fatalf("the desk's tries: %q", got)
	}
	tools, evil := filepath.Join(l.lib, "tools"), filepath.Join(l.lib, ".build", "checkouts", "evil")
	if got, want := r7Start(t, h), r8RepoLinkWords(l.lib, tools, evil); got != want {
		t.Errorf("its next start, a link to the repo:\n got %q\nwant %q", got, want)
	}
	if err := os.Remove(tools); err != nil {
		t.Fatal(err)
	}
	deps := filepath.Join(l.lib, "deps")
	if err := os.Symlink(filepath.Join(l.lib, ".build", "checkouts"), deps); err != nil {
		t.Fatal(err)
	}
	if got, want := r7Start(t, h), r8LinkWords(l.lib, deps, evil); got != want {
		t.Errorf("its next start, a link to the folder that holds it:\n got %q\nwant %q", got, want)
	}
	if err := os.Remove(deps); err != nil {
		t.Fatal(err)
	}
	if words := r7Start(t, h); words != "" {
		t.Errorf("its start with the link gone: %s", words)
	}
	profile, _ = l.profile(t, h)
	if got := sandboxedSh(t, profile, l.lib, r6Tries+`t hook "$1/.build/checkouts/evil/.git/hooks/pre-commit" "$2"`, l.lib, l.planted); strings.Join(got, " ") != "refused" {
		t.Errorf("its hook once held: %q", got)
	}
}

// R7-3: the owner's own SwiftPM mirror and checkout in lib (a built Swift package) are held from a desk at lib: their
// hooks and config, and the folders on the way, by name. The owner's checkout there, as SwiftPM's update does, runs
// nothing. A clone the desk makes during its session (its `swift build` fetching a new dependency) is its own, and
// held from its next start on.
func TestTheOwnersPackageManagerClonesAreHeldFromADesk(t *testing.T) {
	l := newHireProjectsLayout(t)
	dep := filepath.Join(l.base, "dep")
	r7Repo(t, dep)
	ownerMade(t, l.lib, ".build/repositories/", ".build/checkouts/")
	mirror, checkout := filepath.Join(l.lib, ".build", "repositories", "dep-1a2b3c4d"), filepath.Join(l.lib, ".build", "checkouts", "dep")
	standInOwnersGit(t, l.lib, "clone", "-q", "--no-hardlinks", "--mirror", dep, mirror)
	standInOwnersGit(t, l.lib, "clone", "-q", "--shared", "--no-checkout", mirror, checkout)
	standInOwnersGit(t, checkout, "checkout", "-q", "main")
	h := Hire{ID: "r8o", Name: "Janitor", Role: "janitor", Folder: keptPath(l.lib), State: "working"}
	profile := r6Desk(t, l, h.ID, l.lib)
	got := sandboxedSh(t, profile, l.lib, r6Tries+`t hook "$1/.build/checkouts/dep/.git/hooks/post-checkout" "$2"
t sh -c 'echo "[core]" >> "$0/.build/checkouts/dep/.git/config"' "$1"
t hook "$1/.build/repositories/dep-1a2b3c4d/hooks/reference-transaction" "$2"
t mv "$1/.build/checkouts/dep" "$1/.build/checkouts/away"
t mv "$1/.build/checkouts" "$1/.build/elsewhere"
t mv "$1/.build" "$1/build-away"
t /usr/bin/git clone -q --no-hardlinks "$3" "$1/.build/checkouts/new"
t hook "$1/.build/checkouts/new/.git/hooks/post-checkout" "$2"`, l.lib, l.planted, dep)
	if want := "refused refused refused refused refused refused ok ok"; strings.Join(got, " ") != want {
		t.Errorf("the desk's tries: %q, want %q (the owner's checkout's hook and config, the mirror's hook, the folders on the way; its own new clone and its hook)", got, want)
	}
	standInOwnersGit(t, checkout, "checkout", "-q", "develop")
	if _, err := os.Lstat(l.planted); err == nil {
		t.Errorf("the owner's checkout in .build/checkouts/dep ran a hook the desk planted")
	}
	if words := r7Start(t, h); words != "" {
		t.Errorf("its next start: %s", words)
	}
	profile, _ = l.profile(t, h)
	if got := sandboxedSh(t, profile, l.lib, r6Tries+`t hook "$1/.build/checkouts/new/.git/hooks/pre-commit" "$2"`, l.lib, l.planted); strings.Join(got, " ") != "refused" {
		t.Errorf("its own clone at its next start: %q", got)
	}
}

// R7-4 and O2: a worktree hire makes a repo in a package manager's folder in its worktree, gives it a core.fsmonitor,
// stages it as a submodule and commits: its next start, run as the app runs it (its git in its sandbox), is refused.
// With the folder deleted it starts; a gitlink only staged is refused the same way. A real submodule the project has on
// its own branch, checked out in the worktree, starts (the control).
func TestAGitlinkOverARepoInAHiresWorktreeStopsItsNextStart(t *testing.T) {
	t.Setenv(deskStandInVariable, "1") // `smolcorp sandbox enter` for its git in its sandbox at its next start (O2)
	l := newHireProjectsLayout(t)
	sub := filepath.Join(l.base, "subsrc")
	r7Repo(t, sub)
	standInOwnersGit(t, l.app, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "sub")
	standInOwnersGit(t, l.app, "commit", "-q", "-m", "a submodule")
	standInOwnersGit(t, l.hireTree, "merge", "-q", "--ff-only", "main")
	standInOwnersGit(t, l.hireTree, "-c", "protocol.file.allow=always", "submodule", "update", "-q", "--init")
	profile, _ := l.profile(t, l.hire) // its first start, with the project's own submodule checked out: it starts
	if words := r7Start(t, l.hire); words != "" {
		t.Fatalf("its start again, with the project's submodule: %s", words)
	}
	fsm := filepath.Join(l.base, "fsm.sh")
	got := sandboxedSh(t, profile, l.hireTree, r6Tries+`c="-c user.name=Hire -c user.email=hire@example.invalid -c commit.gpgsign=false"
t mkdir -p .build/checkouts/evil
t sh -c 'cd .build/checkouts/evil && /usr/bin/git init -q'
t sh -c 'printf "#!/bin/sh\necho planted > %s\n" "$1" > "$0" && chmod 755 "$0"' "$1" "$2"
t /usr/bin/git -C .build/checkouts/evil config core.fsmonitor "$1"
t sh -c '/usr/bin/git update-index --add --cacheinfo 160000,$(/usr/bin/git rev-parse HEAD),.build/checkouts/evil'
t sh -c '/usr/bin/git '"$c"' commit -q -m evil'`, fsm, l.planted)
	if want := "ok ok ok ok ok ok"; strings.Join(got, " ") != want {
		t.Fatalf("the hire's tries: %q, want %q", got, want)
	}
	tree, evil := tilde(l.hireTree), tilde(filepath.Join(l.hireTree, ".build", "checkouts", "evil"))
	words := func(did, then string) string {
		return "This hire was set to work in " + tree + ", but it " + did + ", and " + evil + " holds a git repository your project doesn't have: your own git, run in " + tree + ", would run in that repository too, with its settings. So it didn't start. Delete the folder " + evil + ", then start it again. " + then
	}
	if got, want := r7Start(t, l.hire), words("committed "+evil+" on its branch as a submodule", "To take the submodule off its branch as well, run `git rm --cached .build/checkouts/evil` in "+tree+" afterwards, and commit."); got != want {
		t.Errorf("its next start:\n got %q\nwant %q", got, want)
	}
	if err := os.RemoveAll(filepath.Join(l.hireTree, ".build", "checkouts", "evil")); err != nil {
		t.Fatal(err)
	}
	if words := r7Start(t, l.hire); words != "" {
		t.Errorf("its start with the folder deleted: %s", words)
	}
	profile, _ = l.profile(t, l.hire)
	got = sandboxedSh(t, profile, l.hireTree, r6Tries+`t sh -c 'mkdir -p .build/checkouts/evil2 && cd .build/checkouts/evil2 && /usr/bin/git init -q'
t sh -c '/usr/bin/git update-index --add --cacheinfo 160000,$(/usr/bin/git rev-parse HEAD),.build/checkouts/evil2'`)
	if strings.Join(got, " ") != "ok ok" {
		t.Fatalf("the hire's staged gitlink: %q", got)
	}
	evil = tilde(filepath.Join(l.hireTree, ".build", "checkouts", "evil2"))
	if got, want := r7Start(t, l.hire), words("staged "+evil+" as a submodule", "To unstage the submodule as well, run `git rm --cached .build/checkouts/evil2` in "+tree+" afterwards."); got != want {
		t.Errorf("its next start, staged only:\n got %q\nwant %q", got, want)
	}
	if _, err := os.Lstat(l.planted); err == nil {
		t.Errorf("the core's git ran the hire's fsmonitor")
	}
}

// pnpm's links, with its store in the project (node_modules/.pnpm) and with a global virtual store outside it, start:
// the first are within the place, the second are walked, bounded.
func TestPnpmsLinksStillStart(t *testing.T) {
	l := newHireProjectsLayout(t)
	h := Hire{ID: "r8n", Name: "Janitor", Role: "janitor", Folder: keptPath(l.lib), State: "working"}
	r6Desk(t, l, h.ID, l.lib)
	store := filepath.Join(homeDir, "Library", "pnpm", "store", "v10", "links")
	const packages = 400
	for i := range packages {
		name := fmt.Sprintf("pkg%d", i)
		inside := filepath.Join(l.lib, "node_modules", ".pnpm", name+"@1.0.0", "node_modules", name)
		outside := filepath.Join(store, name, "1.0.0", "h", "node_modules", name)
		ownerMade(t, inside, "index.js", "lib/a.js", "dist/b.js")
		ownerMade(t, outside, "index.js", "lib/a.js", "dist/b.js")
		dep := fmt.Sprintf("pkg%d", (i+1)%packages)
		for _, link := range [][2]string{
			{filepath.Join(l.lib, "node_modules", name), filepath.Join(".pnpm", name+"@1.0.0", "node_modules", name)},
			{filepath.Join(filepath.Dir(inside), dep), filepath.Join("..", "..", dep+"@1.0.0", "node_modules", dep)},
			{filepath.Join(l.lib, "web", "node_modules", name), outside},
			{filepath.Join(filepath.Dir(outside), dep), filepath.Join(store, dep, "1.0.0", "h", "node_modules", dep)},
		} {
			if err := os.MkdirAll(filepath.Dir(link[0]), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(link[1], link[0]); err != nil {
				t.Fatal(err)
			}
		}
	}
	began := time.Now()
	if words := r7Start(t, h); words != "" {
		t.Errorf("its start: %s", words)
	}
	t.Logf("a start with %d packages, %d links: %v", packages, 4*packages, time.Since(began))
}

// r8PlainHire is the office's plain hire into lib, with its launcher connected by its token: the hire, its tab.
func r8PlainHire(t *testing.T, d *Deck, h *hiring, launcher string) (Hire, *launcherTab) {
	t.Helper()
	d.mu.Lock()
	d.lastHire, d.gmLastHire = time.Time{}, time.Time{}
	d.mu.Unlock()
	if status, said := d.Perform("hire", "", url.Values{"project": {"lib"}}); status != http.StatusNoContent {
		t.Fatalf("the plain hire into lib: %d %s", status, said)
	}
	args := h.tmux[len(h.tmux)-1]
	folder, desk, token := "", "", ""
	for i, a := range args {
		switch {
		case a == "-c" && i+1 < len(args):
			folder = args[i+1]
		case a == "--desk" && i+1 < len(args):
			desk = args[i+1]
		case a == "--hire" && i+1 < len(args):
			token = args[i+1]
		}
	}
	record, ok := plainHireRecord(desk)
	if !ok || record.Worktree.storedOnly() != folder || !strings.HasPrefix(record.Branch, "hire/"+desk+"/") {
		t.Fatalf("its record: %+v %v (window in %q)", record, ok, folder)
	}
	tab := d.launcherConnected(launcher, Claude, folder, "", token)
	if tab == nil || tab.plainDesk != desk {
		t.Fatalf("its launcher's tab: %+v", tab)
	}
	return record, tab
}

// r8Ended waits for plain hire id to be off the record, as its end leaves it.
func r8Ended(t *testing.T, id string) {
	t.Helper()
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if _, ok := plainHireRecord(id); !ok {
			return
		}
	}
	t.Fatalf("plain hire %s wasn't ended", id)
}

// N2: a plain hire's fire from the office closes its window; once its launcher is gone its worktree and branch are
// taken away and its recap says so. One with work left in its worktree keeps both, and its recap says where. One whose
// launcher went while Smolcorp was stopped is ended at its start; one whose launcher connected again without its token
// is ended once that one goes.
func TestAPlainHiresEndTakesItsWorktreeAndBranchAway(t *testing.T) {
	saved := plainHireGrace
	plainHireGrace = 10 * time.Millisecond
	t.Cleanup(func() { plainHireGrace = saved })
	d, l, h := r7wDeck(t, true)
	d.config.HiringEnabled = true
	gone := func(hire Hire) {
		t.Helper()
		if _, err := os.Lstat(hire.Worktree.storedOnly()); err == nil {
			t.Errorf("%s's worktree is still there", hire.ID)
		}
		if branches := standInOwnersGit(t, l.lib, "branch", "--list", hire.Branch); strings.TrimSpace(branches) != "" {
			t.Errorf("%s's branch is still there: %q", hire.ID, branches)
		}
		recap, err := os.ReadFile(filepath.Join(supportDir, "recaps", hire.ID+".md"))
		if err != nil || !strings.Contains(string(recap), "- Worktree: "+hire.Worktree.storedOnly()+" (removed)") || !strings.Contains(string(recap), "- Branch: "+hire.Branch) {
			t.Errorf("%s's recap: %q %v", hire.ID, recap, err)
		}
	}

	// The fire: the office's fire of its tab tells its launcher to stop; its launcher goes.
	fired, tab := r8PlainHire(t, d, h, "L1")
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "p1", "cwd": fired.Worktree.storedOnly()},
		Claude, "launcher:L1", "", 200)
	if status, said := d.Perform("fire", "p1", url.Values{}); status != http.StatusNoContent || !sent(t, tab).Stop {
		t.Fatalf("its fire: %d %s, and its launcher wasn't told to stop", status, said)
	}
	d.launcherGone("L1", tab)
	r8Ended(t, fired.ID)
	gone(fired)

	// Its window closed with work left in its worktree: both stay, and its recap says where.
	kept, tab := r8PlainHire(t, d, h, "L2")
	writeFile(t, filepath.Join(kept.Worktree.storedOnly(), "work.txt"), "the hire's work\n")
	d.launcherGone("L2", tab)
	r8Ended(t, kept.ID)
	if _, err := os.Stat(filepath.Join(kept.Worktree.storedOnly(), "work.txt")); err != nil {
		t.Errorf("its work: %v", err)
	}
	if branches := standInOwnersGit(t, l.lib, "branch", "--list", kept.Branch); strings.TrimSpace(branches) == "" {
		t.Errorf("its branch was deleted")
	}
	if recap, err := os.ReadFile(filepath.Join(supportDir, "recaps", kept.ID+".md")); err != nil || !strings.Contains(string(recap), "- Worktree: "+kept.Worktree.storedOnly()+" (kept)") {
		t.Errorf("its recap: %q %v", recap, err)
	}

	// Connected again without its token, in its worktree: ended once that launcher goes, not before.
	again, tab := r8PlainHire(t, d, h, "L3")
	d.launcherGone("L3", tab)
	back := d.launcherConnected("L3", Claude, again.Worktree.storedOnly(), "", "")
	if back == nil || back.plainDesk != again.ID {
		t.Fatalf("its launcher connected again: %+v", back)
	}
	time.Sleep(10 * plainHireGrace)
	if _, ok := plainHireRecord(again.ID); !ok {
		t.Fatalf("ended while its launcher was back")
	}
	d.launcherGone("L3", back)
	r8Ended(t, again.ID)
	gone(again)

	// Its launcher gone while Smolcorp was stopped: a record with no launcher, ended by the next start's sweep.
	stopped, tab := r8PlainHire(t, d, h, "L4")
	d.mu.Lock()
	delete(d.launchers, "L4")
	d.mu.Unlock()
	close(tab.gone)
	d.sweepPlainHires()
	r8Ended(t, stopped.ID)
	gone(stopped)
	if hires := plainHireRecords(); len(hires) != 0 {
		t.Errorf("records left: %+v", hires)
	}
	if left := standInOwnersGit(t, l.lib, "worktree", "list", "--porcelain"); strings.Count(left, "worktree ") != 2 {
		t.Errorf("worktrees left: %s, want lib's and the one with work in it", left)
	}
}
