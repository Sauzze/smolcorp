package desktop

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// FXHIREPROJECTSR5 (RT126FIXREVIEW-HIREPROJECTSR4 B1, N1; options (a) + (b)) under the real profile, with stand-ins
// only. On a Mac with the sandbox on:
//   - the office's picker lists repos: each at the top of the projects folder, and each one level down in a plain
//     folder (clients/acme), never the plain folder that holds them, nor a link (O3); a folder that holds no repo is
//     still a project of its own;
//   - a place in no repo that holds one is refused at its hire and at its start, by every route, with a bounded walk
//     that fails closed;
//   - a plain hire from the office (another agent, no role) is asked which project too, and starts there.
//
// The words are spelled out here, not taken from the code, so these rows run on the tree before as they are (with a
// stand-in for the two variables this round adds, gitBelowMost and plainHiresSandboxed).

// gitBelowAtHire and gitBelowAtStart are the words a desk at root, a folder in no repo that holds the repo found,
// gets at its hire and at a start.
// Since FXHIREPROJECTSR6 (O2) they name folders in the home folder as ~/… (tilde).
func gitBelowAtHire(root, found string) string {
	return "On this Mac, each hire works in one project of its own, and " + tilde(root) + " isn't one: it holds a git repository, " + tilde(found) + ". Hire it into that repository itself."
}

func gitBelowAtStart(root, found string) string {
	return "This hire was set to work in " + tilde(root) + ", which holds a git repository, " + tilde(found) + ", but on this Mac each hire works in one project of its own, so it didn't start. Fire it, then hire it again into that repository itself."
}

// tilde is a resolved path in the stand-in home folder as the owner's words spell it.
func tilde(p string) string {
	if rest, ok := strings.CutPrefix(p, physical(homeDir)+"/"); ok {
		return "~/" + rest
	}
	return p
}

// clientsLayout adds to the office's projects folder what the R4 review found (B1): clients, a plain folder holding
// two repos, acme and beta. Besides: notes, a plain folder with folders but no repo in it; deep, a plain folder with
// a repo two levels down; mirror, a bare repo; and two links, to a folder outside the projects folder and to lib (O3).
func clientsLayout(t *testing.T, l gmRootLayout) (clients, acme, beta, notes string) {
	t.Helper()
	clients = filepath.Join(l.projects, "clients")
	acme, beta, notes = filepath.Join(clients, "acme"), filepath.Join(clients, "beta"), filepath.Join(l.projects, "notes")
	for _, repo := range []string{acme, beta, filepath.Join(l.projects, "deep", "2024", "site")} {
		ownerMade(t, repo, "README")
		standInOwnersGit(t, repo, "init", "-q", "-b", "main")
		standInOwnersGit(t, repo, "add", "README")
		standInOwnersGit(t, repo, "commit", "-q", "-m", "first")
	}
	ownerMade(t, notes, "todo.md", "2024/january.md", "drafts/")
	standInOwnersGit(t, l.projects, "init", "-q", "--bare", "mirror")
	ownerMade(t, l.base, "elsewhere/")
	for link, to := range map[string]string{"outside": filepath.Join(l.base, "elsewhere"), "shortcut": l.lib} {
		if err := os.Symlink(to, filepath.Join(l.projects, link)); err != nil {
			t.Fatal(err)
		}
	}
	return clients, acme, beta, notes
}

// The picker lists clients/acme and clients/beta, each its own project, not clients; notes, which holds no repo, as
// a project of its own; not deep (a repo below it), mirror (a bare repo), nor either link, wherever it leads (O3).
// clients, deep and the links, asked for by name, are refused as projects that aren't there.
func TestThePickerListsTheReposInAPlainFolderNotTheFolder(t *testing.T) {
	d, l, r := officeDeck(t)
	clientsLayout(t, l)
	status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}})
	if want := `"projects":["app","clients/acme","clients/beta","lib","notes"]`; status != http.StatusOK || !strings.Contains(said, want) {
		t.Fatalf("the hire with no project: %d %s, want %s", status, said, want)
	}
	for _, name := range []string{"clients", "deep", "mirror", "outside", "shortcut", "clients/acme-hires", "clients/../lib"} {
		d.mu.Lock()
		d.lastHire = time.Time{}
		d.mu.Unlock()
		want := "There's no project called " + name + " in your projects folder, ~/Projects, now. Hire again to pick another."
		if status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}, "project": {name}}); status != http.StatusConflict || said != want {
			t.Errorf("a pick of %s: %d %s", name, status, said)
		}
	}
	if len(r.desks) != 0 {
		t.Errorf("made: %d sandboxes", len(r.desks))
	}
}

// A hand-made hire at clients, a folder in no repo that holds two, is refused at its hire, by any route that names
// its folder (Setup's, the GM's), with nothing made or trusted (the picked hire into clients/acme gets its worktree:
// TestAPickedRepoInAPlainFolderGetsItsWorktreeAndCantWriteItsNeighbours). The sandbox off, it is made as before.
func TestAHireAtAFolderOfReposIsRefusedAtItsHire(t *testing.T) {
	r := newSandboxRig(t)
	projects := filepath.Dir(r.repo)
	setHireFolder(t, projects)
	clients := physical(filepath.Join(projects, "clients"))
	for _, repo := range []string{"acme", "beta"} {
		ownerMade(t, filepath.Join(clients, repo), "README")
		standInOwnersGit(t, filepath.Join(clients, repo), "init", "-q", "-b", "main")
	}
	trusted, _ := os.ReadFile(filepath.Join(r.home, ".claude.json"))
	request := r.request("janitor")
	request.Folder = clients
	_, err := runManagedHire(request, r.o)
	var refused *hireRefusal
	if want := gitBelowAtHire(clients, clients+"/acme"); !errors.As(err, &refused) || refused.Status != http.StatusConflict || refused.Message != want {
		t.Errorf("a hire into clients: %v, want %q", err, want)
	}
	if len(r.desks) != 0 || len(registered(t)) != 0 {
		t.Fatalf("made: %d sandboxes, registry %+v", len(r.desks), registered(t))
	}
	if now, _ := os.ReadFile(filepath.Join(r.home, ".claude.json")); string(now) != string(trusted) {
		t.Errorf("the account's file was changed: %s", now)
	}
	// The sandbox off: as before.
	r.o.SandboxOff = true
	request.Folder = clients
	if off, err := runManagedHire(request, r.o); err != nil || off.Folder != clients {
		t.Errorf("the sandbox off: %v, %+v", err, off)
	}
}

// A desk whose row says clients (made before this round, or by hand) doesn't start, the engine's or a plain hire's
// launcher's, and nothing of its sandbox is kept; nor does one at deep, whose repo is two levels down.
func TestADeskAtAFolderOfReposDoesntStart(t *testing.T) {
	l := newHireProjectsLayout(t)
	clients, acme, _, _ := clientsLayout(t, l)
	deep := filepath.Join(l.projects, "deep")
	for i, c := range []struct{ root, found string }{{clients, acme}, {deep, filepath.Join(deep, "2024", "site")}} {
		h := Hire{ID: "r5c" + string(rune('0'+i)), Name: "Janitor", Role: "janitor", Folder: c.root, State: "working"}
		writeRegistryHires(t, l.hire, h)
		if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := makeDeskSandbox(deskAtOf(h, deskSocketPath(h.ID, 0), nil, engineOptions{Git: runGit}))
		var words *errSandboxNotMade
		if want := gitBelowAtStart(c.root, c.found); !errors.As(err, &words) || words.words != want {
			t.Errorf("a desk at %s: %v, want %q", c.root, err, want)
		}
		if _, kept := deskSandboxOf(h.ID); kept {
			t.Errorf("a desk at %s: its sandbox is kept", c.root)
		}
		program := filepath.Join(l.base, "gemini")
		writeFile(t, program, "#!/bin/sh\n")
		if s, err := newLauncherSandbox(c.root, false, "L1", Gemini, program); s != nil || err == nil || err.Error() != gitBelowAtStart(c.root, c.found) {
			if s != nil {
				s.close()
			}
			t.Errorf("a plain hire at %s: %v", c.root, err)
		}
	}
}

// The office's hire with clients/acme picked gets acme's own worktree, and under the real profile it can't write
// beta's hooks, settings or files, nor move beta or make a folder in clients; its own worktree is its own. The owner's
// checkout in beta then runs nothing the desk tried to plant.
func TestAPickedRepoInAPlainFolderGetsItsWorktreeAndCantWriteItsNeighbours(t *testing.T) {
	d, l, _ := officeDeck(t)
	clients, acme, beta, _ := clientsLayout(t, l)
	standInOwnersGit(t, beta, "branch", "develop")
	h := pickedHire(t, d, "clients/acme")
	if h.Folder != acme || h.Repo != acme {
		t.Fatalf("the hire into clients/acme: %+v", h)
	}
	profile, s := l.profile(t, h)
	if s.places.Root != h.Worktree || s.places.Checkout != acme {
		t.Errorf("its places: Root %q, Checkout %q", s.places.Root, s.places.Checkout)
	}
	out := filepath.Join(l.base, "out")
	ownerMade(t, out, "x")
	got := sandboxedSh(t, profile, h.Worktree, `t() { if "$@" 2>/dev/null; then echo ok; else echo refused; fi; }
hook() { printf '#!/bin/sh\necho planted > "%s"\n' "$3" > "$1" && chmod 755 "$1"; }
t hook "$1/beta/.git/hooks/post-checkout" x "$4"
t sh -c 'printf "[core]\n\tfsmonitor = /nowhere\n" >> "$0/beta/.git/config"' "$1"
t sh -c 'echo x > "$0/beta/README"' "$1"
t mv "$1/beta" "$2/beta"
t mkdir "$1/new"
t hook "$1/acme/.git/hooks/post-checkout" x "$4"
t sh -c 'echo own > own.txt'`, clients, out, "", l.planted)
	if want := "refused refused refused refused refused refused ok"; strings.Join(got, " ") != want {
		t.Errorf("its tries: %q, want %q", got, want)
	}
	standInOwnersGit(t, beta, "checkout", "-q", "develop")
	for _, at := range []string{l.planted, beta + "/.git/hooks/post-checkout", out + "/beta", clients + "/new"} {
		if _, err := os.Lstat(at); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s is there", strings.TrimPrefix(at, l.base))
		}
	}
	if config, _ := os.ReadFile(beta + "/.git/config"); strings.Contains(string(config), "fsmonitor") {
		t.Errorf("beta's config is now %q", config)
	}
}

// The control: a plain folder that holds no repo is still a project. Picked, the hire works in it (no worktree: it
// isn't a repo), starts, and writes there, but not in lib.
func TestAPlainFolderWithNoRepoIsStillHireable(t *testing.T) {
	d, l, r := officeDeck(t)
	_, _, _, notes := clientsLayout(t, l)
	if status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}, "project": {"notes"}}); status != http.StatusNoContent {
		t.Fatalf("the hire into notes: %d %s", status, said)
	}
	d.watching.Wait()
	got := registered(t)
	h := got[len(got)-1]
	if h.Folder != notes || h.Repo != "" || h.Worktree != "" || len(r.desks) != 1 || r.desks[0].places.Root != notes {
		t.Fatalf("the hire into notes: %+v", h)
	}
	profile, _ := l.profile(t, h)
	if got := sandboxedSh(t, profile, notes, `t() { if "$@" 2>/dev/null; then echo ok; else echo refused; fi; }
t sh -c 'echo x > "$0/2024/february.md"' "$1"
t mkdir "$1/later"
t sh -c 'echo x > "$0/README"' "$2"`, notes, l.lib); strings.Join(got, " ") != "ok ok refused" {
		t.Errorf("its tries: %q", got)
	}
}

// The walk's bound, and a folder it can't read: either refuses the hire and the start with words that say what to
// do, and the picker doesn't list the folder. Under the bound, the same folder is hireable.
func TestAFolderTheWalkCantLookAllThroughIsRefusedWithWords(t *testing.T) {
	d, l, r := officeDeck(t)
	big := filepath.Join(l.projects, "big")
	ownerMade(t, big, "a/b/c/d/e/f/", "g/h/", "x.md")
	saved := gitBelowMost
	t.Cleanup(func() { gitBelowMost = saved })
	gitBelowMost = 5
	words := func(started bool, why string) string {
		// A folder it can't read has its own remedy (FXHIREPROJECTSR7, RT126FIXREVIEW-HIREPROJECTSR6 N2).
		if unread, ok := strings.CutPrefix(why, "Smolcorp can't read "); ok && started {
			return "This hire was set to work in " + tilde(big) + ", but Smolcorp can't read " + unread + ", so it couldn't make sure " + tilde(big) + " holds no git repository, and it didn't start. Make " + unread + " readable, or move it out of " + tilde(big) + ", then start it again."
		} else if ok {
			return "On this Mac, each hire works in one project of its own, and Smolcorp couldn't make sure " + tilde(big) + " holds no git repository: it can't read " + unread + ". Make " + unread + " readable, or move it out of " + tilde(big) + ", then hire it again."
		}
		if started {
			return "This hire was set to work in " + tilde(big) + ", but Smolcorp couldn't make sure it holds no git repository (" + why + "), and on this Mac each hire works in one project of its own, so it didn't start. Fire it, then hire it again into the repository you want it to work on, or into a smaller folder that holds none."
		}
		return "On this Mac, each hire works in one project of its own, and Smolcorp couldn't make sure " + tilde(big) + " holds no git repository: " + why + ". Hire it into the repository you want it to work on, or into a smaller folder that holds none."
	}
	unread := filepath.Join(big, "g", "h")
	for _, c := range []struct {
		name, why string
		before    func()
	}{
		{"past the bound", tilde(big) + " has more than 5 folders, more than Smolcorp looks through", func() {}},
		{"a folder it can't read", "Smolcorp can't read " + tilde(unread), func() {
			gitBelowMost = saved
			if err := os.Chmod(unread, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(unread, 0o700) })
		}},
	} {
		c.before()
		if _, said := d.Perform("hire", "", url.Values{"role": {"janitor"}}); strings.Contains(said, `"big"`) {
			t.Errorf("%s: the picker lists big: %s", c.name, said)
		}
		request := r.request("janitor")
		request.Folder = big
		_, err := runManagedHire(request, r.o)
		var refused *hireRefusal
		if want := words(false, c.why); !errors.As(err, &refused) || refused.Message != want {
			t.Errorf("%s: its hire: %v, want %q", c.name, err, want)
		}
		h := Hire{ID: "r5b", Name: "Janitor", Role: "janitor", Folder: big, State: "working"}
		writeRegistryHires(t, h)
		if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err = makeDeskSandbox(deskAtOf(h, deskSocketPath(h.ID, 0), nil, engineOptions{Git: runGit}))
		var notMade *errSandboxNotMade
		if want := words(true, c.why); !errors.As(err, &notMade) || notMade.words != want {
			t.Errorf("%s: its start: %v, want %q", c.name, err, want)
		}
	}
	// Readable again, and under the bound: a project of its own.
	if err := os.Chmod(unread, 0o700); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	d.lastHire = time.Time{}
	d.mu.Unlock()
	if status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}, "project": {"big"}}); status != http.StatusNoContent {
		t.Errorf("big, under the bound: %d %s", status, said)
	}
}

// N1: a plain hire from the office (no role) on a Mac is asked which project first, as the engine's
// hire is, hires nobody until one is picked, and its launcher's window opens in the project picked, where its
// sandbox is made: in the folder for hires itself it couldn't be. Since ruling (W) (FXHIREPROJECTSR7), in its own
// worktree of that project; and a plain hire on a Mac is one with no role here (Gemini CLI, which this row hired
// before, is refused on a Mac by FXPLANTSTERM). The owner's switch off, it isn't asked: it starts outside its
// sandbox in the folder for hires, as before (FXSWITCHPLAIN; one plainHireSandboxed, FXSTACKG G22). A plain hire pinned
// to clients (a floor's folder) is refused at its hire with the walk's words.
func TestTheOfficesPlainHireAsksWhichProjectLandsInItAndStarts(t *testing.T) {
	d, l, _ := officeDeck(t)
	clients, acme, _, _ := clientsLayout(t, l)
	h := fakeHiring(t, "/usr/bin/tmux")
	saved := plainHiresSandboxed
	t.Cleanup(func() { plainHiresSandboxed = saved })
	plainHiresSandboxed = true
	again := func() {
		d.mu.Lock()
		d.lastHire = time.Time{}
		d.mu.Unlock()
	}
	status, said := d.Perform("hire", "", url.Values{})
	if want := `"projects":["app","clients/acme","clients/beta","lib","notes"]`; status != http.StatusOK || !strings.Contains(said, want) {
		t.Fatalf("the plain hire with no project: %d %s, want %s", status, said, want)
	}
	if len(h.tmux) != 0 || len(h.windows) != 0 {
		t.Fatalf("opened before a project was picked: %q %q", h.tmux, h.windows)
	}
	again()
	if status, said := d.Perform("hire", "", url.Values{"project": {"clients/acme"}}); status != http.StatusNoContent {
		t.Fatalf("the plain hire into clients/acme: %d %s", status, said)
	}
	worktree, desk := r7wPlainHire(t, h)
	if filepath.Dir(worktree) != hiresFolder(acme, "") {
		t.Fatalf("tmux was asked %q, want a session in a worktree of %s", h.tmux, acme)
	}
	// Its launcher's sandbox, made where its window opened, as `smolcorp run --hire … --desk …` makes it there.
	program := filepath.Join(l.base, "claude")
	writeFile(t, program, "#!/bin/sh\n")
	s, err := newLauncherSandboxAt(desk, worktree, false, "L1", Claude, program)
	if err != nil {
		t.Fatalf("its sandbox in its worktree of clients/acme: %v", err)
	}
	if s.desk.places.Root != worktree || s.desk.places.Checkout != acme {
		t.Errorf("its sandbox's place: %q, checkout %q", s.desk.places.Root, s.desk.places.Checkout)
	}
	s.close()
	if s, err := newLauncherSandbox(l.projects, false, "L1", Claude, program); err == nil {
		s.close()
		t.Errorf("its sandbox in the folder for hires was made")
	}
	// The switch off: not asked; its launcher opens in the folder for hires, told the sandbox is off, with no desk.
	again()
	d.mu.Lock()
	d.config.SandboxOff = true
	d.mu.Unlock()
	if status, said := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent {
		t.Errorf("the switch off: %d %s", status, said)
	}
	if len(h.tmux) != 2 || !slices.Contains(h.tmux[1], "--sandbox-off") || slices.Contains(h.tmux[1], "--desk") ||
		!slices.Contains(h.tmux[1], l.projects) {
		t.Errorf("the switch off: tmux was asked %q", h.tmux)
	}
	// Pinned to clients (2F's folder): refused at its hire, nothing opened.
	again()
	d.mu.Lock()
	d.config.SandboxOff = false
	d.config.FloorRules = checkedFloorRules([]FloorRule{{clients, 1, false}})
	d.mu.Unlock()
	if status, said := d.Perform("hire", "", url.Values{"floor": {"1"}, "desk": {"0"}}); status != http.StatusConflict || said != gitBelowAtHire(clients, acme) {
		t.Errorf("a plain hire pinned to clients: %d %s", status, said)
	}
	if len(h.tmux) != 2 {
		t.Errorf("opened: %q", h.tmux)
	}
}
