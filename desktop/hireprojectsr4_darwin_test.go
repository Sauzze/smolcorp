package desktop

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// FXHIREPROJECTSR4 (the owner's ruling of 18 Nov, "a hire writes only its own project folder; one hire per repo") under
// the real profile, with stand-ins only. On a Mac with the sandbox on, no desk's place is the projects folder, or a
// folder that holds it:
//   - the office's hire into the folder for hires asks which project first, hires nobody until one is picked, and goes
//     into that project's own worktree;
//   - a hire asked for there by any other route is refused at its hire, and a desk made so before doesn't start;
//   - the ways a desk at the projects folder still had after R3 (RT126FIXREVIEW-HIREPROJECTSR3 N1, O1–O3 and its list)
//     are refused to every desk there is now: a worktree hire, the office's picked hire and a GM-made hire.
//
// The words are spelled out here, not taken from the code, so these rows run on the tree before as they are.

// projectsWholeAtStart and projectsWholeAtHire are the words a desk at where gets, where says what it was set to work
// in (wholeProjectsWords): at a start, what to do by each way to hire (FXHIREPROJECTSR5, N2); at its hire (Setup's,
// the GM's: the office asks which project first), to give a project's folder.
func projectsWholeAtStart(where string) string {
	return "This hire was set to work in " + where + ", but on this Mac each hire works in one project of its own, so it didn't start. " +
		"Fire it, then hire it again into one of your projects: the office asks which one; in Setup, choose or type that project's own folder; or ask the General Manager to name it."
}

func projectsWholeAtHire(where string) string {
	return "On this Mac, each hire works in one project of its own, not in " + where + ". Give it the folder of one of your projects instead."
}

// A desk whose place is the projects folder (the office's plain hire before R4, a hire with worktrees off in the
// projects folder that is itself a repo) or holds it (the home folder) doesn't start in its sandbox, whatever made its
// row: its start says why and what to do, and nothing of its sandbox is made or kept.
func TestADeskAtTheProjectsFolderOrHomeDoesntStartInItsSandbox(t *testing.T) {
	l := newHireProjectsLayout(t)
	at := map[string]Hire{
		"the projects folder": {ID: "r4p", Name: "Janitor", Role: "janitor", Folder: keptPath(l.projects), State: "working"},
		"the home folder":     {ID: "r4h", Name: "Janitor", Role: "janitor", Folder: keptPath(homeDir), State: "working"},
	}
	want := map[string]string{
		"the projects folder": projectsWholeAtStart("your whole projects folder, " + l.projects),
		"the home folder":     projectsWholeAtStart(homeDir + ", which holds your projects folder, " + l.projects),
	}
	check := func(name string) {
		t.Helper()
		h := at[name]
		writeRegistryHires(t, l.hire, h)
		if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := makeDeskSandbox(deskAtOf(h, deskSocketPath(h.ID, 0), nil, engineOptions{Git: runGit}))
		var words *errSandboxNotMade
		if !errors.As(err, &words) || words.words != want[name] {
			t.Errorf("%s: its start: %v, want %q", name, err, want[name])
		}
		if _, kept := deskSandboxOf(h.ID); kept {
			t.Errorf("%s: its sandbox is kept", name)
		}
	}
	check("the projects folder")
	check("the home folder")
	// The projects folder itself a repo, and a desk at its checkout (worktrees off): the same.
	standInOwnersGit(t, l.projects, "init", "-q", "-b", "main")
	check("the projects folder")
	// Its worktree hire still starts.
	if _, s := l.profile(t, l.hire); s.places.Root != l.hireTree {
		t.Errorf("h1's place: %q", s.places.Root)
	}
}

// A hire asked for in the projects folder, or in a folder that holds it, by a route that names its folder (Setup's,
// a recent one, the GM's) is refused at its hire with words, before anything is made or trusted there; the GM, whose
// workspace is its place, and a hire picked into a project, are made. The sandbox off, it is made as before.
func TestAHireAskedForInTheProjectsFolderIsRefusedAtItsHire(t *testing.T) {
	r := newSandboxRig(t)
	projects := filepath.Dir(r.repo)
	setHireFolder(t, projects)
	trusted, _ := os.ReadFile(filepath.Join(r.home, ".claude.json"))
	for folder, want := range map[string]string{
		projects: projectsWholeAtHire("your whole projects folder, " + projects),
		homeDir:  projectsWholeAtHire("your whole home folder"),
	} {
		request := r.request("janitor")
		request.Folder = folder
		_, err := runManagedHire(request, r.o)
		var refused *hireRefusal
		if !errors.As(err, &refused) || refused.Status != http.StatusConflict || refused.Message != want {
			t.Errorf("a hire into %s: %v, want %q", folder, err, want)
		}
	}
	if len(r.desks) != 0 || len(registered(t)) != 0 {
		t.Fatalf("made: %d sandboxes, registry %+v", len(r.desks), registered(t))
	}
	if now, _ := os.ReadFile(filepath.Join(r.home, ".claude.json")); string(now) != string(trusted) {
		t.Errorf("the account's file was changed: %s", now)
	}
	// The GM there gets its workspace; a hire in the project, its worktree.
	gm := r.request(gmRole)
	gm.Folder = projects
	if _, err := runManagedHire(gm, r.o); err != nil || r.desks[0].places.GMProjects != projects {
		t.Errorf("the GM in the projects folder: %v", err)
	}
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil || hire.Worktree.none() || r.desks[1].places.Root != physical(hire.Worktree.storedOnly()) {
		t.Errorf("a hire in the project: %v, %+v", err, hire)
	}
	// The sandbox off: as before.
	r.o.SandboxOff = true
	request := r.request("janitor")
	request.Folder = projects
	if off, err := runManagedHire(request, r.o); err != nil || off.Folder.storedOnly() != projects || !off.Worktree.none() {
		t.Errorf("the sandbox off: %v, %+v", err, off)
	}
}

// officeDeck is newHireProjectsLayout's office: a deck hiring through the engine, tmux and the desk's start
// stand-ins, each desk's sandbox made for real (makeDeskSandbox) and kept in desks, the projects folder its folder for
// hires.
func officeDeck(t *testing.T) (*Deck, gmRootLayout, *sandboxRig) {
	t.Helper()
	fakeHiring(t, "/usr/bin/tmux")
	r := newSandboxRig(t)
	l := newHireProjectsLayout(t)
	t.Setenv("HOME", homeDir)
	writeConfig(t, homeDir, "{}")
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	d.config.HiringEnabled, d.config.HireInTmux, d.config.HireFolder = true, true, l.projects
	r.o.Worktrees = ""
	d.engine = r.o
	d.engine.Within, d.engine.Every = 20*time.Millisecond, 5*time.Millisecond
	d.engine.HeardFrom = func(Hire) bool { return true }
	return d, l, r
}

// The office's hire with no project picked hires nobody: it answers with the projects to pick from. Picked, it goes
// into that project's own worktree, as any hire; a name that isn't a project there is refused. With the sandbox off
// it hires into the folder for hires as before.
func TestTheOfficesHireAsksWhichProjectFirstThenTakesItsWorktree(t *testing.T) {
	d, l, r := officeDeck(t)
	status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}})
	if status != http.StatusOK || !strings.Contains(said, `"projects":["app","lib"]`) {
		t.Fatalf("the hire with no project: %d %s", status, said)
	}
	if len(r.desks) != 0 || len(registered(t)) != 1 {
		t.Fatalf("made: %d sandboxes, registry %+v", len(r.desks), registered(t))
	}
	if status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}, "project": {"../lib"}}); status != http.StatusConflict ||
		said != "There's no project called ../lib in your projects folder, ~/Projects, now. Hire again to pick another." {
		t.Errorf("a project that isn't there: %d %s", status, said)
	}
	hp := pickedHire(t, d, "lib")
	if len(r.desks) != 1 || r.desks[0].places.Root != hp.Worktree.storedOnly() || r.desks[0].places.Checkout != l.lib {
		t.Errorf("its place: %+v", r.desks[0].places)
	}
	// The sandbox off: no question, the folder for hires as before.
	d.mu.Lock()
	d.config.SandboxOff = true
	d.lastHire, d.gmLastHire = time.Time{}, time.Time{}
	d.mu.Unlock()
	if status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Fatalf("the sandbox off: %d %s", status, said)
	}
	d.watching.Wait()
	if got := registered(t); got[len(got)-1].Folder.storedOnly() != l.projects || !(got[len(got)-1].Worktree).none() {
		t.Errorf("the sandbox off: %+v", got[len(got)-1])
	}
}

// pickedHire is the office's hire of a janitor into project, picked in the folder for hires: its row, which must be a
// worktree of that project's own.
func pickedHire(t *testing.T, d *Deck, project string) Hire {
	t.Helper()
	d.mu.Lock()
	d.lastHire, d.gmLastHire = time.Time{}, time.Time{}
	d.mu.Unlock()
	before := len(registered(t))
	if status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}, "project": {project}}); status != http.StatusNoContent {
		t.Fatalf("the hire into %s: %d %s", project, status, said)
	}
	d.watching.Wait()
	got := registered(t)
	if len(got) != before+1 {
		t.Fatalf("the registry after the hire into %s: %+v", project, got)
	}
	h := got[len(got)-1]
	repo := filepath.Join(homeDir, "Projects", project)
	if h.Folder.storedOnly() != repo || h.Repo != repo || filepath.Dir(h.Worktree.storedOnly()) != hiresFolder(repo, "") {
		t.Fatalf("the hire into %s: folder %q, repo %q, worktree %q", project, h.Folder, h.Repo, h.Worktree)
	}
	return h
}

// residualTries are the ways a desk at the projects folder still had after R3, tried by /bin/sh in a desk's sandbox:
// $1 the projects folder, $2 a folder outside it where it may make anything, $3 the marker a planted program writes,
// $4 the owner's checkout of its own repo, $5 another repo's. Each says "ok" or "refused".
const residualTries = `P=$1 O=$2 M=$3 OWN=$4 OTHER=$5
t() { if "$@" 2>/dev/null; then echo ok; else echo refused; fi; }
hook() { printf '#!/bin/sh\necho planted > "%s"\n' "$M" > "$1" && chmod 755 "$1"; }
todo() { printf 'exec sh -c "echo planted > %s"\n' "$M" >> "$1/.git/rebase-merge/git-rebase-todo"; }
t mv "$P/lib/vendor/sub" "$O/sub"
t hook "$P/lib/vendor/sub/.git/hooks/post-checkout"
t mv "$P/clients/acme" "$O/acme"
t hook "$P/clients/acme/.git/hooks/post-checkout"
t hook "$P/mirror/hooks/post-receive"
t sh -c 'printf "[core]\n\tfsmonitor = /nowhere\n" >> "$0/mirror/config"' "$P"
t mv "$P/mirror" "$O/mirror"
t todo "$OWN"
t mkdir "$OWN/.git/sequencer"
t todo "$OTHER"
t mkdir "$OTHER/.git/sequencer"
t sh -c 'echo x > "$0/.git/refs/heads/main.lock"' "$OTHER"
t sh -c 'echo 0000000000000000000000000000000000000000 > "$0/.git/refs/heads/main"' "$OTHER"
t sh -c 'echo 0000000000000000000000000000000000000000 > "$0/.git/refs/heads/main"' "$OWN"`

// residualLayout is what the owner has in the projects folder before a desk starts: a repo nested in lib
// (lib/vendor/sub), a bare repo not named *.git (mirror, lib's push target), and a rebase each left stopped in app's
// and lib's checkouts.
func residualLayout(t *testing.T, l gmRootLayout) {
	t.Helper()
	sub := filepath.Join(l.lib, "vendor", "sub")
	ownerMade(t, sub, "README")
	standInOwnersGit(t, sub, "init", "-q", "-b", "main")
	standInOwnersGit(t, sub, "add", "README")
	standInOwnersGit(t, sub, "commit", "-q", "-m", "first")
	standInOwnersGit(t, l.projects, "init", "-q", "--bare", "mirror")
	for _, repo := range []string{l.app, l.lib} {
		standInOwnersGit(t, repo, "-c", "sequence.editor=printf 'break\\n' >", "rebase", "-q", "-i", "HEAD")
		if _, err := os.Stat(filepath.Join(repo, ".git", "rebase-merge", "git-rebase-todo")); err != nil {
			t.Fatalf("no rebase stopped in %s: %v", repo, err)
		}
	}
}

// The six ways a desk at the projects folder still had after R3 are refused to every desk there is now: a worktree
// hire, the office's hire with its project picked, and a GM-made hire (the GM open, with its workspace). Each try is
// refused; then the owner, outside any sandbox, continues both rebases, pushes into mirror and makes a repo while the
// desk runs, and nothing the desk tried to plant runs, nor has any branch of the owner's moved.
func TestNoDeskTakesTheWaysADeskAtTheProjectsFolderHad(t *testing.T) {
	for _, c := range []struct {
		name string
		desk func(t *testing.T) (gmRootLayout, Hire, string, string)
	}{
		{"a worktree hire", func(t *testing.T) (gmRootLayout, Hire, string, string) {
			l := newHireProjectsLayout(t)
			residualLayout(t, l)
			return l, l.hire, l.app, l.lib
		}},
		{"the office's hire, its project picked", func(t *testing.T) (gmRootLayout, Hire, string, string) {
			d, l, _ := officeDeck(t)
			residualLayout(t, l)
			return l, pickedHire(t, d, "lib"), l.lib, l.app
		}},
		{"a GM-made hire", func(t *testing.T) (gmRootLayout, Hire, string, string) {
			l := newGMRootLayout(t)
			setHireFolder(t, l.projects)
			residualLayout(t, l)
			return l, l.hire, l.app, l.lib
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			l, h, own, other := c.desk(t)
			profile, s := l.profile(t, h)
			if c.name == "a GM-made hire" && s.places.GMStart != l.projects {
				t.Fatalf("no GM open: the GM's start folder is %q", s.places.GMStart)
			}
			// A repo the owner makes while the desk runs, below the top.
			acme := filepath.Join(l.projects, "clients", "acme")
			ownerMade(t, acme, "README")
			standInOwnersGit(t, acme, "init", "-q", "-b", "main")
			heads := map[string]string{}
			for _, repo := range []string{l.app, l.lib} {
				heads[repo] = standInOwnersGit(t, repo, "rev-parse", "main")
			}
			out := filepath.Join(l.base, "out")
			ownerMade(t, out, "x")
			got := sandboxedSh(t, profile, h.Worktree.storedOnly(), residualTries, l.projects, out, l.planted, own, other)
			if want := strings.TrimSpace(strings.Repeat("refused ", 14)); strings.Join(got, " ") != want {
				t.Errorf("%s's tries: %q, want each refused", h.ID, got)
			}
			// The owner, outside any sandbox.
			for _, repo := range []string{l.app, l.lib} {
				standInOwnersGit(t, repo, "rebase", "--continue")
				if now := standInOwnersGit(t, repo, "rev-parse", "main"); now != heads[repo] {
					t.Errorf("%s's main moved: %s", repo, now)
				}
			}
			standInOwnersGit(t, l.lib, "push", "-q", filepath.Join(l.projects, "mirror"), "main")
			for _, at := range []string{l.planted, l.lib + "/vendor/sub/.git/hooks/post-checkout", acme + "/.git/hooks/post-checkout",
				l.projects + "/mirror/hooks/post-receive", out + "/sub", out + "/acme", out + "/mirror"} {
				if _, err := os.Lstat(at); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("%s is there", strings.TrimPrefix(at, l.base))
				}
			}
			if config, _ := os.ReadFile(l.projects + "/mirror/config"); strings.Contains(string(config), "fsmonitor") {
				t.Errorf("mirror's config is now %q", config)
			}
		})
	}
}

// The fifth way: the projects folder itself a repo, the owner's own. A worktree hire, and the office's picked hire,
// can't move lib out of it, nor write its git folder's hooks or config.
func TestNoDeskTakesTheProjectsFoldersOwnRepo(t *testing.T) {
	d, l, _ := officeDeck(t)
	standInOwnersGit(t, l.projects, "init", "-q", "-b", "main")
	for _, h := range []Hire{l.hire, pickedHire(t, d, "lib")} {
		profile, _ := l.profile(t, h)
		got := sandboxedSh(t, profile, h.Worktree.storedOnly(), `t() { if "$@" 2>/dev/null; then echo ok; else echo refused; fi; }
t mv "$1/lib" "$2/lib"
t mv "$1/app" "$2/app"
t sh -c 'echo x > "$0/.git/hooks/post-checkout"' "$1"
t sh -c 'printf "[core]\n\tfsmonitor = /nowhere\n" >> "$0/.git/config"' "$1"`, l.projects, l.base)
		if strings.Join(got, " ") != "refused refused refused refused" {
			t.Errorf("%s's tries at the projects folder's repo: %q, want each refused", h.ID, got)
		}
	}
}

// The control: the office's picked hire works in its worktree as any hire does: a commit on its branch, a fetch from
// another repo there, a rebase on the owner's main.
func TestTheOfficesPickedHireCommitsFetchesAndRebasesInItsWorktree(t *testing.T) {
	d, l, _ := officeDeck(t)
	ownerMade(t, l.lib, "CHANGES")
	standInOwnersGit(t, l.lib, "add", "CHANGES")
	standInOwnersGit(t, l.lib, "commit", "-q", "-m", "owner's")
	h := pickedHire(t, d, "lib")
	standInOwnersGit(t, h.Worktree.storedOnly(), "reset", "-q", "--hard", "HEAD~1")
	profile, _ := l.profile(t, h)
	script := `set -e
git=/usr/bin/git
c="-c user.name=Hire -c user.email=hire@example.invalid -c commit.gpgsign=false"
echo own > own.txt
$git $c add own.txt
$git $c commit -q -m own
$git $c fetch -q "$1" main
$git $c rebase -q main
$git log --format=%s -3`
	got := sandboxedSh(t, profile, h.Worktree.storedOnly(), script, l.app)
	if len(got) < 2 || got[0] != "own" || got[1] != "owner's" {
		t.Errorf("the picked hire's branch after its rebase: %q", got)
	}
	if got := strings.TrimSpace(standInOwnersGit(t, l.lib, "log", "--format=%s", "-1", h.Branch)); got != "own" {
		t.Errorf("the owner's git reads its branch as %q", got)
	}
}
