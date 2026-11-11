//go:build !windows

package desktop

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// listenAt makes a real unix socket at path, kept for the test's length.
func listenAt(t *testing.T, path string) {
	t.Helper()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
}

// cleanupHire makes a real worktree for a hire on a temporary repo, with git's
// global config and the home folder out of reach, and the hire as the registry
// would hold it, on the socket it would have.
func cleanupHire(t *testing.T, id string, testPort int) (Hire, string) {
	hire, repo, _ := cleanupHireIn(t, id, testPort)
	return hire, repo
}

// cleanupHireIn also says the folder the hire's worktree was made under.
func cleanupHireIn(t *testing.T, id string, testPort int) (Hire, string, string) {
	t.Helper()
	shortSupport(t)
	t.Setenv("HOME", homeDir)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(homeDir, "no-gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(homeDir, "no-gitconfig"))
	writeConfig(t, homeDir, "{}")
	repo := placeRepo(t)
	opts := placeOptions(t)
	place, err := prepareHire(hireRequest{HireID: id, Name: "x", Folder: repo, Claude: true, Worktree: true}, opts)
	if err != nil {
		t.Fatal(err)
	}
	socket := hireOwnSocket(id, testPort)
	listenAt(t, socket)
	return Hire{ID: id, Name: "x", Folder: place.Folder, Branch: place.Branch, Worktree: place.Worktree, Repo: repo,
		TmuxSocket: socket}, repo, opts.Worktrees
}

func TestACleanUpRemovesOnlyAHiresOwnCleanWorktree(t *testing.T) {
	hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	remove := removeHireWorktree(nil, 0, trees)
	// The main repo, spelt through a link, is never taken for a hire's worktree.
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	main := hire
	main.Worktree = link
	if remove(main) {
		t.Error("the main repo was taken for a hire's worktree")
	}
	for name, edit := range map[string]func(*Hire){
		"no worktree": func(h *Hire) { h.Worktree = "" },
		"no repo":     func(h *Hire) { h.Repo = "" },
		"no socket":   func(h *Hire) { h.TmuxSocket = "" },
	} {
		other := hire
		edit(&other)
		if remove(other) {
			t.Errorf("%s: a worktree was removed", name)
		}
	}
	if branches(t, repo) == "" {
		t.Fatal("a refused clean-up removed the worktree")
	}
	if !remove(hire) || branches(t, repo) != "" {
		t.Error("a hire's own clean worktree wasn't removed")
	}
}

func TestACleanUpKeepsAWorktreeThatHoldsChanges(t *testing.T) {
	hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	os.WriteFile(filepath.Join(hire.Worktree, "new.txt"), []byte("work"), 0o644)
	if removeHireWorktree(nil, 0, trees)(hire) || branches(t, repo) == "" {
		t.Error("a worktree with changes was removed")
	}
}

// A real install never cleans up a test copy's socket, and a test copy never
// a real install's, though ownsHireSocket counts "smolcorp-7799-<id>" as a hire's
// own name for a real install (7799-<id> is a name an id can have).
func TestACleanUpAcceptsOnlyTheHiresOwnExactSocket(t *testing.T) {
	hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	in := func(name string) string { return filepath.Join(supportDir, name) }
	if !isHireSocket(hire, 0) || hireOwnTmux(hire, 0, newRecorder().run) == nil {
		t.Fatal("a hire's own socket wasn't accepted")
	}
	for name, socket := range map[string]string{
		"a test copy's socket on port 7799":   in("smolcorp-7799-ab12cd34ef56"),
		"a test copy's socket on port 7720":   in("smolcorp-7720-ab12cd34ef56"),
		"another hire's socket":               in("smolcorp-ffffffffffff"),
		"the default tmux socket":             filepath.Join(os.TempDir(), "tmux-1000", "default"),
		"the same name in another folder":     filepath.Join(os.TempDir(), "smolcorp-ab12cd34ef56"),
		"the right path through a longer one": in("x/../smolcorp-ab12cd34ef56x"),
	} {
		other := hire
		other.TmuxSocket = socket
		if isHireSocket(other, 0) || hireOwnTmux(other, 0, newRecorder().run) != nil || removeHireWorktree(nil, 0, trees)(other) {
			t.Errorf("a real install's clean-up accepted %s", name)
		}
	}
	if branches(t, repo) == "" {
		t.Fatal("a refused clean-up removed the worktree")
	}
	// A test copy owns only its own port's socket, and a real install's is not that.
	copyHire := hire
	copyHire.TmuxSocket = in("smolcorp-7799-ab12cd34ef56")
	listenAt(t, copyHire.TmuxSocket)
	if !isHireSocket(copyHire, 7799) || isHireSocket(hire, 7799) || removeHireWorktree(nil, 7799, trees)(hire) {
		t.Error("a test copy reached a real install's hire")
	}
	if got := hireOwnSocket("A B", 0); got != "" {
		t.Errorf("an id that can't name a socket made %q", got)
	}
}

// A test copy's clean-up never takes a real install's hire, even with the right
// worktrees folder, so it is the socket check that refuses.
func TestATestCopyNeverCleansUpARealInstallsHire(t *testing.T) {
	hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	g := &yesGit{head: hire.Branch}
	if removeHireWorktree(g.run, 7799, trees)(hire) || g.removed() {
		t.Error("a test copy's clean-up took a real install's hire")
	}
	if removeHireWorktree(nil, 7799, trees)(hire) || branches(t, repo) == "" {
		t.Error("real git: a test copy's clean-up took a real install's hire")
	}
}

// cleanUpHire through the exact-socket check ends the hire's session on its own
// socket and no other.
func TestCleanUpEndsTheSessionOnlyOnTheHiresOwnSocket(t *testing.T) {
	hire, _, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	r := newRecorder()
	cleanUpHire(hire, "r", hireOwnTmux(hire, 0, r.run), removeHireWorktree(nil, 0, trees), func(string, string) {})
	if len(r.asked) != 2 || r.asked[1][0] != "-S" || r.asked[1][1] != hire.TmuxSocket || r.asked[1][2] != "kill-session" {
		t.Errorf("tmux was asked %q, want kill-session on %q", r.asked, hire.TmuxSocket)
	}
	foreign := hire
	foreign.TmuxSocket = filepath.Join(supportDir, "smolcorp-7799-ab12cd34ef56")
	r = newRecorder()
	cleanUpHire(foreign, "r", hireOwnTmux(foreign, 0, r.run), removeHireWorktree(nil, 0, trees), func(string, string) {})
	if len(r.asked) != 0 {
		t.Errorf("a real install's clean-up reached a test copy's server: %q", r.asked)
	}
}

// A fake git that says yes to everything: only the guard's own checks can refuse.
// It records what it was asked, and names head as the branch checked out.
type yesGit struct {
	asked [][]string
	head  string
}

func (g *yesGit) run(dir string, args ...string) (int, string, bool) {
	g.asked = append(g.asked, append([]string{dir}, args...))
	if len(args) > 0 && args[0] == "symbolic-ref" {
		return 0, g.head, true
	}
	return 0, "", true
}

func (g *yesGit) removed() bool {
	for _, a := range g.asked {
		if len(a) > 2 && a[1] == "worktree" && a[2] == "remove" {
			return true
		}
	}
	return false
}

// The main-repo check is the guard's own, not git's: the repo spelt as ".", as
// a sibling, a subfolder, a link, or its parent is never a hire's worktree, with
// a git that would remove anything. The repo sits directly in the hires folder
// here, so for the repo itself and a link to it only that check stands between.
func TestTheMainRepoCheckIsTheGuardsOwnNotGits(t *testing.T) {
	hire, _ := cleanupHire(t, "ab12cd34ef56", 0)
	trees := physical(t.TempDir())
	repo := filepath.Join(trees, "proj", "proj")
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(trees, "proj", "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct{ worktree, cwd string }{
		"the repo":                    {repo, ""},
		"the repo as .":               {".", repo},
		"the repo as a relative name": {"proj", filepath.Dir(repo)},
		"the repo as ../proj":         {"../proj", filepath.Join(repo, "sub")},
		"a subfolder":                 {filepath.Join(repo, "sub"), ""},
		"a subfolder, relative":       {"sub", repo},
		"the repo's parent":           {filepath.Dir(repo), ""},
		"the repo's parent, relative": {"..", repo},
		"the repo through a link":     {link, ""},
	}
	for name, c := range cases {
		if c.cwd != "" {
			t.Chdir(c.cwd)
		}
		h := hire
		h.Repo, h.Worktree = repo, c.worktree
		h.Branch = "hire/" + filepath.Base(physical(c.worktree))
		git := &yesGit{head: h.Branch}
		if removeHireWorktree(git.run, 0, trees)(h) || git.removed() {
			t.Errorf("%s: taken for a hire's worktree", name)
		}
	}
	// The same fake git does remove a hire's own worktree (it is not a stub that always says no).
	own := filepath.Join(trees, "proj", "x-ab12cd")
	os.MkdirAll(own, 0o755)
	h := hire
	h.Repo, h.Worktree, h.Branch = repo, own, "hire/x-ab12cd"
	git := &yesGit{head: h.Branch}
	if !removeHireWorktree(git.run, 0, trees)(h) || !git.removed() {
		t.Error("a hire's own worktree wasn't taken by the guard with a git that says yes")
	}
}

// A user's own linked worktree of the same repo is never removed, though a
// record names it with the hire's socket and git would say yes.
func TestACleanUpNeverRemovesAUsersOwnWorktree(t *testing.T) {
	hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	git := func(args ...string) {
		t.Helper()
		all := append([]string{"-C", repo}, args...)
		if out, err := exec.Command("git", all...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	elsewhere := filepath.Join(physical(t.TempDir()), "mine")
	git("worktree", "add", "-b", "hire/mine", elsewhere)
	inFolder := filepath.Join(hiresFolder(repo, trees), "theirs-111111")
	git("worktree", "add", "-b", "feature", inFolder)
	for name, c := range map[string]struct{ path, branch string }{
		"outside the hires folder, a hire/ branch": {elsewhere, "hire/mine"},
		"in the hires folder, another branch":      {inFolder, "feature"},
		"the hire's folder, another branch":        {hire.Worktree, "feature"},
	} {
		h := hire
		h.Worktree, h.Branch = c.path, c.branch
		if removeHireWorktree(nil, 0, trees)(h) {
			t.Errorf("%s: removed", name)
		}
		if _, err := os.Stat(c.path); err != nil {
			t.Errorf("%s: the folder is gone", name)
		}
	}
	if out, _ := exec.Command("git", "-C", repo, "branch", "--list", "feature", "hire/mine").Output(); !strings.Contains(string(out), "feature") || !strings.Contains(string(out), "hire/mine") {
		t.Errorf("a user's branch was dropped: %q", out)
	}
}

// What is at the recorded path must be a socket and not a link or another file.
func TestACleanUpRefusesASocketPathThatIsALinkOrNotASocket(t *testing.T) {
	hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	other := filepath.Join(supportDir, "elsewhere")
	listenAt(t, other)
	if err := os.Remove(hire.TmuxSocket); err != nil {
		t.Fatal(err)
	}
	for name, make := range map[string]func() error{
		"a link to a socket": func() error { return os.Symlink(other, hire.TmuxSocket) },
		"a plain file":       func() error { return os.WriteFile(hire.TmuxSocket, nil, 0o600) },
		"nothing there":      func() error { return nil },
	} {
		os.Remove(hire.TmuxSocket)
		if err := make(); err != nil {
			t.Fatal(err)
		}
		if isHireSocket(hire, 0) || hireOwnTmux(hire, 0, newRecorder().run) != nil || removeHireWorktree(nil, 0, trees)(hire) {
			t.Errorf("%s: accepted as the hire's socket", name)
		}
	}
	if branches(t, repo) == "" {
		t.Fatal("a refused clean-up removed the worktree")
	}
}

// A support folder reached through a link (tmux may say /private/var for /var)
// still finds the hire's own socket, spelt either way; the socket stays a file.
func TestACleanUpWorksThroughASymlinkedSupportFolder(t *testing.T) {
	hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	real := supportDir
	dir, err := os.MkdirTemp("", "hl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	link := filepath.Join(dir, "s")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(hire.TmuxSocket)
	for spelling, c := range map[string]struct{ support, recorded string }{
		"the folder is a link, the socket recorded resolved":  {link, filepath.Join(real, name)},
		"the folder is real, the socket recorded by the link": {real, filepath.Join(link, name)},
		"both by the link": {link, filepath.Join(link, name)},
	} {
		supportDir = c.support
		h := hire
		h.TmuxSocket = c.recorded
		if !isHireSocket(h, 0) || hireOwnTmux(h, 0, newRecorder().run) == nil {
			t.Errorf("%s: not accepted", spelling)
		}
	}
	// A different folder with the same file name is still not the hire's.
	supportDir = real
	elsewhere := t.TempDir()
	h := hire
	h.TmuxSocket = filepath.Join(elsewhere, name)
	if isHireSocket(h, 0) {
		t.Error("a socket in another folder was accepted")
	}
	supportDir = link
	if !removeHireWorktree(nil, 0, trees)(hire) || branches(t, repo) != "" {
		t.Error("the hire's own worktree wasn't removed through a linked support folder")
	}
}

// An empty id names no socket, and cleans up nothing, even on the socket that
// the bare prefix would give.
func TestAnEmptyHireIDNamesNoSocketAndCleansUpNothing(t *testing.T) {
	hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	if got := hireOwnSocket("", 0); got != "" {
		t.Errorf("an empty id made %q", got)
	}
	bare := filepath.Join(supportDir, "smolcorp-")
	listenAt(t, bare)
	h := hire
	h.ID, h.TmuxSocket = "", bare
	r := newRecorder()
	if isHireSocket(h, 0) || hireOwnTmux(h, 0, r.run) != nil || removeHireWorktree(nil, 0, trees)(h) {
		t.Error("an empty id was taken for a hire")
	}
	if len(r.asked) != 0 || branches(t, repo) == "" {
		t.Error("something was cleaned up for an empty id")
	}
}

// physical gives an absolute path with links resolved, so a relative spelling
// of the main repo is the same string as the repo.
func TestPhysicalGivesAnAbsoluteResolvedPath(t *testing.T) {
	dir := physical(t.TempDir())
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "a"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(dir, "a", "b"))
	for in, want := range map[string]string{
		".":                        filepath.Join(dir, "a", "b"),
		"..":                       filepath.Join(dir, "a"),
		"../b":                     filepath.Join(dir, "a", "b"),
		filepath.Join(dir, "link"): filepath.Join(dir, "a"),
		"not-there":                filepath.Join(dir, "a", "b", "not-there"),
		"":                         "",
	} {
		if got := physical(in); got != want {
			t.Errorf("physical(%q) = %q, want %q", in, got, want)
		}
	}
}

// gitIn runs git in repo for a test's setup.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	all := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", all...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// Only a DIRECT child of the hires folder is a hire's worktree: not the folder
// itself (also with a trailing slash), nor a worktree nested inside a hire's.
func TestACleanUpTakesOnlyADirectChildOfTheHiresFolder(t *testing.T) {
	hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	folder := hiresFolder(repo, trees)
	nested := filepath.Join(hire.Worktree, "inner")
	gitIn(t, repo, "worktree", "add", "-b", "hire/inner", nested)
	// A clean linked worktree at the hires folder itself is made by moving the
	// hire's own out of the way first.
	cases := map[string]struct{ path, branch string }{
		"the hires folder":                {folder, "hire/" + filepath.Base(folder)},
		"the hires folder, trailing /":    {folder + "/", "hire/" + filepath.Base(folder)},
		"nested inside a hire's worktree": {nested, "hire/inner"},
	}
	for name, c := range cases {
		git := &yesGit{head: c.branch}
		h := hire
		h.Worktree, h.Branch = c.path, c.branch
		if removeHireWorktree(git.run, 0, trees)(h) || git.removed() {
			t.Errorf("%s: taken for a hire's worktree", name)
		}
		if removeHireWorktree(nil, 0, trees)(h) {
			t.Errorf("%s: removed with real git", name)
		}
	}
	if _, err := os.Stat(nested); err != nil {
		t.Error("the nested worktree is gone")
	}
	if _, err := os.Stat(hire.Worktree); err != nil {
		t.Error("the hire's worktree is gone")
	}
}

// The guard and git read one path: git is only ever given the guard's own
// resolved absolute paths, and a relative record is refused.
func TestGitOnlyEverSeesTheGuardsOwnResolvedPaths(t *testing.T) {
	hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	links := t.TempDir()
	treesLink, repoLink := filepath.Join(links, "t"), filepath.Join(links, "r")
	for to, from := range map[string]string{treesLink: trees, repoLink: repo} {
		if err := os.Symlink(from, to); err != nil {
			t.Fatal(err)
		}
	}
	h := hire
	h.Repo = repoLink
	h.Worktree = filepath.Join(treesLink, filepath.Base(repo), filepath.Base(hire.Worktree))
	git := &yesGit{head: h.Branch}
	if !removeHireWorktree(git.run, 0, trees)(h) || !git.removed() {
		t.Fatal("the hire's own worktree, spelt through links, wasn't taken")
	}
	tree := physical(hire.Worktree)
	for _, a := range git.asked {
		if a[0] != physical(repo) && a[0] != tree {
			t.Errorf("git was run in %q: %q", a[0], a)
		}
		for _, arg := range a[1:] {
			if strings.Contains(arg, string(filepath.Separator)) && arg != tree && !strings.HasPrefix(arg, "hire/") {
				t.Errorf("git was given the path %q: %q", arg, a)
			}
		}
	}
	// A relative Repo or Worktree is refused, whatever the folder Smolcorp is in.
	for name, edit := range map[string]func(*Hire){
		"relative worktree": func(h *Hire) { h.Worktree = filepath.Base(hire.Worktree) },
		"relative repo":     func(h *Hire) { h.Repo = filepath.Base(repo) },
	} {
		for _, cwd := range []string{filepath.Dir(hire.Worktree), filepath.Dir(repo), repo} {
			t.Chdir(cwd)
			h := hire
			edit(&h)
			g := &yesGit{head: h.Branch}
			if removeHireWorktree(g.run, 0, trees)(h) || len(g.asked) != 0 {
				t.Errorf("%s in %s: git was asked: %q", name, cwd, g.asked)
			}
		}
	}
}

// A worktree whose branch was renamed, or is detached, is kept with every branch.
func TestACleanUpKeepsAWorktreeWhoseBranchIsNotTheHires(t *testing.T) {
	for name, change := range map[string]func(t *testing.T, hire Hire, repo string){
		"renamed": func(t *testing.T, hire Hire, repo string) {
			gitIn(t, repo, "branch", "-m", hire.Branch, "my-feature")
		},
		"renamed, the old name made again, merged": func(t *testing.T, hire Hire, repo string) {
			gitIn(t, repo, "branch", "-m", hire.Branch, "my-feature")
			gitIn(t, repo, "branch", hire.Branch)
		},
		"detached": func(t *testing.T, hire Hire, repo string) {
			gitIn(t, hire.Worktree, "checkout", "--detach")
		},
	} {
		t.Run(name, func(t *testing.T) {
			hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
			change(t, hire, repo)
			before := gitIn(t, repo, "branch", "--list")
			if removeHireWorktree(nil, 0, trees)(hire) {
				t.Error("removed")
			}
			if _, err := os.Stat(hire.Worktree); err != nil {
				t.Error("the worktree is gone")
			}
			if after := gitIn(t, repo, "branch", "--list"); after != before {
				t.Errorf("branches changed:\n%s\n->\n%s", before, after)
			}
		})
	}
}

// isHireSocket's belt-and-braces refusals: a relative path (Smolcorp's own folder
// being the support folder), one with "..", and an id that names no socket.
func TestIsHireSocketRefusesRelativeDotDotAndNamelessRecords(t *testing.T) {
	hire, _ := cleanupHire(t, "ab12cd34ef56", 0)
	name := filepath.Base(hire.TmuxSocket)
	if !isHireSocket(hire, 0) {
		t.Fatal("the hire's own socket wasn't accepted")
	}
	t.Chdir(supportDir)
	for spelling, edit := range map[string]func(*Hire){
		"relative": func(h *Hire) { h.TmuxSocket = name },
		"dotdot":   func(h *Hire) { h.TmuxSocket = filepath.Join(supportDir, "x") + "/../" + name },
		"no name":  func(h *Hire) { h.ID = "" },
	} {
		h := hire
		edit(&h)
		if isHireSocket(h, 0) {
			t.Errorf("%s: accepted", spelling)
		}
	}
}

// A record whose Worktree is itself a link is never taken, though git would say
// yes and the link's target is a clean worktree on a hire/ branch with the
// hire's own socket: the target is the other hire's (or the record's own, seen
// through a link) and must stay. Only the record's last path part can be a link
// the guard can see, so that is what it looks at.
func TestACleanUpRefusesARecordWhoseWorktreeIsALink(t *testing.T) {
	// Each case has its own repo and hires, so one that goes wrong can't hide the next.
	for name, linkAt := range map[string]func(folder string) string{
		"in the hires folder":      func(folder string) string { return filepath.Join(folder, "link-ab12cd") },
		"outside the hires folder": func(string) string { return filepath.Join(physical(t.TempDir()), "link") },
	} {
		for what, own := range map[string]bool{"another hire's worktree": false, "the record's own worktree": true} {
			t.Run(name+", to "+what, func(t *testing.T) {
				hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
				target, branch := hire.Worktree, hire.Branch
				if !own {
					target, branch = filepath.Join(hiresFolder(repo, trees), "other-111111"), "hire/other-111111"
					gitIn(t, repo, "worktree", "add", "-b", branch, target)
				}
				link := linkAt(hiresFolder(repo, trees))
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
				h := hire
				h.Worktree, h.Branch = link, branch
				git := &yesGit{head: branch}
				if removeHireWorktree(git.run, 0, trees)(h) || git.removed() {
					t.Error("taken, with a git that says yes")
				}
				if removeHireWorktree(nil, 0, trees)(h) {
					t.Error("removed with real git")
				}
				for _, p := range []string{target, link} {
					if _, err := os.Lstat(p); err != nil {
						t.Errorf("%s is gone", p)
					}
				}
			})
		}
	}
	t.Run("the plain record is still taken", func(t *testing.T) {
		hire, _, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
		git := &yesGit{head: hire.Branch}
		if !removeHireWorktree(git.run, 0, trees)(hire) || !git.removed() {
			t.Error("the hire's own plain worktree wasn't taken by a git that says yes")
		}
		if !removeHireWorktree(nil, 0, trees)(hire) {
			t.Error("the hire's own plain worktree wasn't removed with real git")
		}
	})
}

// How a path is spelt must not get round the link check: a record whose
// Worktree or Repo is not already in clean form is refused, so <link>/. , <link>/./
// and <folder in the target>/.. (which all name the link's target through the
// link) take nothing. The fake git is never asked: the refusal comes first.
var unclean = []string{"/", "//", "/.", "/./", "/sub/.."}

func TestACleanUpRefusesAWorktreeSpeltInAnyUncleanWay(t *testing.T) {
	for name, linkAt := range map[string]func(folder string) string{
		"in the hires folder":      func(folder string) string { return filepath.Join(folder, "link-ab12cd") },
		"outside the hires folder": func(string) string { return filepath.Join(physical(t.TempDir()), "link") },
	} {
		for what, own := range map[string]bool{"another hire's worktree": false, "the record's own worktree": true} {
			for _, tail := range append([]string{""}, unclean...) {
				t.Run(name+", to "+what+", spelt <link>"+tail, func(t *testing.T) {
					hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
					target, branch := hire.Worktree, hire.Branch
					if !own {
						target, branch = filepath.Join(hiresFolder(repo, trees), "other-111111"), "hire/other-111111"
						gitIn(t, repo, "worktree", "add", "-b", branch, target)
					}
					if err := os.MkdirAll(filepath.Join(target, "sub"), 0o755); err != nil {
						t.Fatal(err)
					}
					link := linkAt(hiresFolder(repo, trees))
					if err := os.Symlink(target, link); err != nil {
						t.Fatal(err)
					}
					h := hire
					h.Worktree, h.Branch = link+tail, branch
					git := &yesGit{head: branch}
					if removeHireWorktree(git.run, 0, trees)(h) || len(git.asked) != 0 {
						t.Errorf("taken or asked, with a git that says yes: %v", git.asked)
					}
					if removeHireWorktree(nil, 0, trees)(h) {
						t.Error("removed with real git")
					}
					if _, err := os.Stat(target); err != nil {
						t.Error("the target is gone")
					}
				})
			}
		}
	}
}

// The same for the record's Repo: an unclean spelling of the real repo is
// refused, not read as the repo.
func TestACleanUpRefusesARepoSpeltInAnyUncleanWay(t *testing.T) {
	for _, tail := range unclean {
		t.Run("repo"+tail, func(t *testing.T) {
			hire, repo, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
			if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
			h := hire
			h.Repo = repo + tail
			git := &yesGit{head: h.Branch}
			if removeHireWorktree(git.run, 0, trees)(h) || len(git.asked) != 0 {
				t.Errorf("taken or asked, with a git that says yes: %v", git.asked)
			}
			if removeHireWorktree(nil, 0, trees)(h) {
				t.Error("removed with real git")
			}
			if _, err := os.Stat(h.Worktree); err != nil {
				t.Error("the hire's worktree is gone")
			}
		})
	}
}

// A record whose Worktree can't be looked at (its folder is right, the worktree
// is gone) is refused by the link check's own Lstat error, not left to git.
func TestACleanUpRefusesAWorktreeThatCannotBeLookedAt(t *testing.T) {
	hire, _, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	h := hire
	h.Worktree = filepath.Join(filepath.Dir(hire.Worktree), "gone-111111")
	h.Branch = "hire/gone-111111"
	git := &yesGit{head: h.Branch}
	if removeHireWorktree(git.run, 0, trees)(h) || git.removed() {
		t.Error("a worktree that isn't there was taken, with a git that says yes")
	}
}

// The two halves of the main-repo check are each pinned: with the worktrees
// folder the repo's parent, the hires folder IS the repo, so a subfolder of the
// repo is a direct child of it (only "the worktree is inside the repo" refuses
// it), and a repo inside a record's tree (only "the repo is inside the worktree"
// refuses it). A git that says yes removes nothing here.
func TestEachHalfOfTheMainRepoCheckStandsOnItsOwn(t *testing.T) {
	hire, _ := cleanupHire(t, "ab12cd34ef56", 0)
	trees := physical(t.TempDir())
	repo := filepath.Join(trees, "proj")
	sub := filepath.Join(repo, "sub")
	holder := filepath.Join(trees, "proj", "holder")
	inner := filepath.Join(holder, "proj")
	for _, d := range []string{sub, inner} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, c := range map[string]struct{ repo, tree string }{
		"a subfolder of the repo (the hires folder is the repo)": {repo, sub},
		"a repo inside the record's tree":                        {inner, holder},
	} {
		h := hire
		h.Repo, h.Worktree, h.Branch = c.repo, c.tree, "hire/"+filepath.Base(c.tree)
		if filepath.Dir(physical(c.tree)) != physical(hiresFolder(physical(c.repo), trees)) {
			t.Fatalf("%s: the setup is not a direct child of the hires folder", name)
		}
		git := &yesGit{head: h.Branch}
		if removeHireWorktree(git.run, 0, trees)(h) || git.removed() {
			t.Errorf("%s: taken for a hire's worktree", name)
		}
	}
}

// With no support folder at all no record's socket is the hire's (isHireSocket
// needs no separate check for it: a recorded absolute path never resolves to "").
func TestNoSupportFolderMeansNoHireSocket(t *testing.T) {
	hire, _ := cleanupHire(t, "ab12cd34ef56", 0)
	if !isHireSocket(hire, 0) {
		t.Fatal("the hire's own socket wasn't accepted")
	}
	supportDir = ""
	if isHireSocket(hire, 0) {
		t.Error("accepted with no support folder")
	}
}

// Agent windows in the background (T796): detachHireViewers, through a fake tmux, only lists the hire's viewers and
// detaches them, on its own socket and session; it never ends the session or the server. A socket that isn't
// exactly the hire's own runs nothing, and a tmux that can't say who's watching is never asked to detach.
func TestDetachingAHiresViewersNeverEndsIt(t *testing.T) {
	shortSupport(t)
	port := testCopyPort()
	socket := hireOwnSocket("ab12cd34ef56", port)
	listenAt(t, socket)
	hire := Hire{ID: "ab12cd34ef56", Name: "x", TmuxSocket: socket}
	never := func(t *testing.T, r *recorder) {
		t.Helper()
		for _, call := range r.asked {
			if len(call) < 3 || call[0] != "-S" || filepath.Base(call[1]) != filepath.Base(socket) {
				t.Errorf("not on the hire's own socket: %v", call)
			}
			for _, word := range call {
				if word == "kill-session" || word == "kill-server" || word == "kill-pane" {
					t.Errorf("ended something: %v", call)
				}
			}
		}
	}

	two := newRecorder()
	two.output = "/dev/pts/3\n/dev/pts/4"
	if got := detachHireViewers(hire, two.run); got != (detached{own: true, asked: true, viewers: 2, ok: true}) {
		t.Errorf("two viewers: %+v", got)
	}
	never(t, two)
	if len(two.asked) != 2 || !reflect.DeepEqual(two.asked[0][2:], []string{"list-clients", "-t", "=hire", "-F", "#{client_tty}"}) ||
		!reflect.DeepEqual(two.asked[1][2:], []string{"detach-client", "-s", "=hire"}) {
		t.Errorf("calls %v", two.asked)
	}

	none := newRecorder()
	if got := detachHireViewers(hire, none.run); got != (detached{own: true, asked: true, ok: true}) || len(none.asked) != 1 {
		t.Errorf("no viewer: %+v, calls %v", got, none.asked)
	}
	never(t, none)

	failing := newRecorder()
	failing.status = 1
	if got := detachHireViewers(hire, failing.run); got != (detached{own: true}) || len(failing.asked) != 1 {
		t.Errorf("tmux couldn't say: %+v, calls %v", got, failing.asked)
	}
	never(t, failing)

	elsewhere := newRecorder()
	other := hire
	other.TmuxSocket = filepath.Join(filepath.Dir(socket), "default")
	if got := detachHireViewers(other, elsewhere.run); got != (detached{}) || len(elsewhere.asked) != 0 {
		t.Errorf("not its own socket: %+v, calls %v", got, elsewhere.asked)
	}
}
