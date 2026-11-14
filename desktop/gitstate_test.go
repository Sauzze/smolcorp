package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// Git state in the core (gitstate.go): the parsers on git's own words, real throwaway repos (signing off in their
// local config only), the cache on a fake clock and a fake git, and the routes.

// gitRepoCmd runs git in dir for a test, with no signing and an identity given on the command line only.
func gitRepoCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	out, err := exec.Command("git", append([]string{"-C", dir, "-c", "commit.gpgsign=false", "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeRepoFile(t *testing.T, dir, name, text string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// aRepo is a repo with one commit, in a temp folder with its links resolved.
func aGitRepo(t *testing.T) string {
	t.Helper()
	dir := physical(t.TempDir())
	gitRepoCmd(t, dir, "init", "-q", "-b", "main")
	gitRepoCmd(t, dir, "config", "commit.gpgsign", "false")
	writeRepoFile(t, dir, "a.txt", "one\ntwo\nthree\n")
	writeRepoFile(t, dir, "src/b.txt", "b\n")
	gitRepoCmd(t, dir, "add", ".")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "first")
	return dir
}

func realGitState(dir string) GitState {
	state, _ := readGitState(dir, readGitOut, time.Now())
	return state
}

// withGitHere stands in for what is asked of the git here: its version and the owner's settings.
func withGitHere(t *testing.T, here gitHereInfo) {
	t.Helper()
	was := gitHere
	gitHere = func() gitHereInfo { return here }
	t.Cleanup(func() { gitHere = was })
}

func TestStatusOfGitIsReadFromItsPorcelainV2(t *testing.T) {
	out := "# branch.oid 0123456789abcdef\x00# branch.head feat/x\x00# branch.upstream origin/feat/x\x00# branch.ab +2 -1\x00" +
		"1 .M N... 100644 100644 100644 aaa bbb dir/a b.txt\x00" +
		"2 R. N... 100644 100644 100644 aaa bbb R100 new name.txt\x00old name.txt\x00" +
		"u UU N... 100644 100644 100644 100644 a b c conflict.txt\x00" +
		"? new/\x00"
	var state GitState
	state.Files = []GitFile{}
	if unborn := parseGitStatus([]byte(out), &state); unborn {
		t.Error("a repo with a commit isn't unborn")
	}
	if state.Branch != "feat/x" || state.Head != "01234567" || state.Upstream != "origin/feat/x" || state.Ahead != 2 || state.Behind != 1 || state.Changed != 4 {
		t.Fatalf("%+v", state)
	}
	want := []GitFile{{Path: filepath.FromSlash("dir/a b.txt"), State: ".M"}, {Path: "new name.txt", State: "R."}, {Path: "conflict.txt", State: "UU"}, {Path: "new" + string(filepath.Separator), State: "??"}}
	for i, f := range want {
		if state.Files[i] != f {
			t.Errorf("file %d: %+v, want %+v", i, state.Files[i], f)
		}
	}
	var fresh, detached GitState
	if unborn := parseGitStatus([]byte("# branch.oid (initial)\x00# branch.head main\x00"), &fresh); !unborn || fresh.Branch != "main" {
		t.Errorf("before the first commit: %v %+v", unborn, fresh)
	}
	parseGitStatus([]byte("# branch.oid abc\x00# branch.head (detached)\x00"), &detached)
	if !detached.Detached || detached.Branch != "" {
		t.Errorf("detached: %+v", detached)
	}
}

func TestNumstatCountsEveryFileAndNamesTheListedOnes(t *testing.T) {
	state := GitState{Files: []GitFile{{Path: "a.txt"}, {Path: filepath.FromSlash("src/b.txt")}}}
	parseGitNumstat([]byte("3\t1\ta.txt\x00-\t-\tpic.png\x002\t0\tsrc/b.txt\x002\t2\tnot listed.txt\x00"), &state)
	if state.Added != 7 || state.Removed != 3 || state.Files[0].Added != 3 || state.Files[0].Removed != 1 || state.Files[1].Added != 2 {
		t.Errorf("%+v", state)
	}
}

func TestARealRepoIsReadWithItsChangesAndIsLeftAsItWas(t *testing.T) {
	dir := aGitRepo(t)
	writeRepoFile(t, dir, "a.txt", "one\nTWO\nthree\nfour\n") // +2 −1
	writeRepoFile(t, dir, "src/b.txt", "")                    // −1, staged below
	gitRepoCmd(t, dir, "add", "src/b.txt")
	writeRepoFile(t, dir, "fresh.txt", "x\n") // untracked: listed, no lines counted
	before := map[string]string{}
	for _, name := range []string{"index", "HEAD"} {
		data, _ := os.ReadFile(filepath.Join(dir, ".git", name))
		before[name] = string(data)
	}
	state := realGitState(filepath.Join(dir, "src"))
	if !state.Repo || state.Top != dir || state.Branch != "main" || state.Detached || state.Upstream != "" || state.Head == "" {
		t.Fatalf("%+v", state)
	}
	if state.Changed != 3 || state.Added != 2 || state.Removed != 2 || state.Skipped || len(state.Files) != 3 {
		t.Fatalf("%+v", state)
	}
	byPath := map[string]GitFile{}
	for _, f := range state.Files {
		byPath[filepath.ToSlash(f.Path)] = f
	}
	if f := byPath["a.txt"]; f.State != ".M" || f.Added != 2 || f.Removed != 1 {
		t.Errorf("a.txt %+v", f)
	}
	if f := byPath["src/b.txt"]; f.State != "M." || f.Removed != 1 {
		t.Errorf("b.txt %+v", f)
	}
	if f := byPath["fresh.txt"]; f.State != "??" || f.Added != 0 {
		t.Errorf("fresh.txt %+v", f)
	}
	for name, was := range before {
		if data, _ := os.ReadFile(filepath.Join(dir, ".git", name)); string(data) != was {
			t.Errorf("%s was written to", name)
		}
	}
}

func TestAFolderInNoRepoAndAnUnbornRepoAreAnswered(t *testing.T) {
	if state := realGitState(physical(t.TempDir())); state.Repo || state.Skipped || state.Files == nil {
		t.Errorf("no repo: %+v", state)
	}
	dir := physical(t.TempDir())
	gitRepoCmd(t, dir, "init", "-q", "-b", "trunk")
	writeRepoFile(t, dir, "x.txt", "x\n")
	if state := realGitState(dir); !state.Repo || state.Branch != "trunk" || state.Changed != 1 || state.Skipped {
		t.Errorf("unborn: %+v", state)
	}
	if state := realGitState(filepath.Join(dir, "gone")); state.Repo {
		t.Errorf("a folder that isn't there: %+v", state)
	}
}

// A hire works in a worktree of its own: the state is the worktree's branch and changes (startedIn's place), not the repo's.
func TestAHireIsReadInItsWorktreeNotTheRepo(t *testing.T) {
	repo := aGitRepo(t)
	worktree := filepath.Join(physical(t.TempDir()), "hire-wt")
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/ana", worktree)
	writeRepoFile(t, worktree, "src/c.txt", "c\n")
	writeRepoFile(t, repo, "only-in-repo.txt", "r\n")
	h := Hire{Folder: filepath.Join(repo, "src"), Repo: repo, Worktree: worktree}
	place := startedIn(h)
	if place != filepath.Join(worktree, "src") {
		t.Fatalf("startedIn: %s", place)
	}
	state := realGitState(place)
	if state.Top != physical(worktree) || state.Branch != "hire/ana" || state.Changed != 1 || state.Files[0].Path != filepath.FromSlash("src/c.txt") {
		t.Errorf("the worktree: %+v", state)
	}
	if main := realGitState(repo); main.Branch != "main" || main.Changed != 1 || main.Files[0].Path != "only-in-repo.txt" {
		t.Errorf("the repo: %+v", main)
	}
}

func TestAnUpstreamGivesAheadAndBehind(t *testing.T) {
	origin := physical(t.TempDir())
	gitRepoCmd(t, origin, "init", "-q", "--bare", "-b", "main")
	repo := aGitRepo(t)
	gitRepoCmd(t, repo, "remote", "add", "origin", origin)
	gitRepoCmd(t, repo, "push", "-q", "-u", "origin", "main")
	writeRepoFile(t, repo, "n.txt", "n\n")
	gitRepoCmd(t, repo, "add", "n.txt")
	gitRepoCmd(t, repo, "commit", "-q", "-m", "second")
	if state := realGitState(repo); state.Upstream != "origin/main" || state.Ahead != 1 || state.Behind != 0 {
		t.Errorf("%+v", state)
	}
}

// withAttrSource forces the answer of the version check, so both paths are run on any git (2.32 or later).
func withAttrSource(t *testing.T, on bool) {
	t.Helper()
	if on {
		withGitHere(t, gitHereInfo{version: "git version 2.40.0"})
	} else {
		withGitHere(t, gitHereInfo{version: "git version 2.39.0"})
	}
}

// A hostile repo: its own config, its attributes (in the working folder and in .git/info), an included config, the
// owner's own config and the environment each name a program that writes a marker. Reading it must run none.
type hostileRepo struct {
	dir     string
	markers string
	names   []string
}

func (h *hostileRepo) program(t *testing.T, name string) string {
	t.Helper()
	script := filepath.Join(physical(t.TempDir()), name+".sh")
	body := "#!/bin/sh\ntouch '" + filepath.Join(h.markers, name) + "'\ncat\n" // a filter passes its input on
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	h.names = append(h.names, name)
	return script
}

// ran says which programs left their marker since the last call, and clears the markers.
func (h *hostileRepo) ran() []string {
	var ran []string
	for _, name := range h.names {
		if err := os.Remove(filepath.Join(h.markers, name)); err == nil {
			ran = append(ran, name)
		}
	}
	return ran
}

func aHostileGitRepo(t *testing.T) *hostileRepo {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	h := &hostileRepo{dir: aGitRepo(t), markers: physical(t.TempDir())}
	dir := h.dir
	for _, name := range []string{"c.txt", "d.txt", "e.txt", "g.txt", "h.txt"} {
		writeRepoFile(t, dir, name, name+"\n")
	}
	gitRepoCmd(t, dir, "add", "c.txt", "d.txt", "e.txt", "g.txt", "h.txt")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "more")

	cfg := func(key, value string) { gitRepoCmd(t, dir, "config", key, value) }
	cfg("filter.clean1.clean", h.program(t, "clean"))
	cfg("filter.clean1.smudge", h.program(t, "smudge"))
	cfg("filter.clean1.process", h.program(t, "process"))
	cfg("diff.conv.textconv", h.program(t, "textconv"))
	cfg("diff.conv.command", h.program(t, "externaldiff"))
	cfg("diff.external", h.program(t, "diffexternal"))
	cfg("core.fsmonitor", h.program(t, "fsmonitor"))
	cfg("core.pager", h.program(t, "pager"))
	cfg("core.sshCommand", h.program(t, "ssh"))
	cfg("credential.helper", h.program(t, "credential"))
	cfg("filter.infofilter.clean", h.program(t, "infoclean"))
	cfg("filter.x=y.clean", h.program(t, "equalsclean")) // `-c filter.x=y.clean=` would set "filter.x" instead
	hooks := physical(t.TempDir())
	for _, hook := range []string{"post-index-change", "fsmonitor-watchman", "reference-transaction"} {
		if err := os.WriteFile(filepath.Join(hooks, hook), []byte("#!/bin/sh\ntouch '"+filepath.Join(h.markers, "hook")+"'\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	h.names = append(h.names, "hook")
	cfg("core.hooksPath", hooks)
	included := filepath.Join(physical(t.TempDir()), "included.cfg")
	if err := os.WriteFile(included, []byte("[filter \"inc\"]\n\tclean = "+h.program(t, "included")+"\n[core]\n\tfsmonitor = "+h.program(t, "includedmonitor")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg("include.path", included)
	cfg("includeIf.gitdir:"+dir+"/.path", included)

	writeRepoFile(t, dir, ".gitattributes", "a.txt filter=clean1 diff=conv\nc.txt filter=inc\nd.txt filter=owners\ne.txt filter=fromenv\nh.txt filter=x=y\n")
	writeRepoFile(t, dir, ".git/info/attributes", "src/b.txt filter=infofilter\ng.txt filter=x=y\n")

	// The owner's own config and the environment, set after the setup above, which ran with both as they were.
	home := physical(t.TempDir())
	writeRepoFile(t, home, ".gitconfig", "[filter \"owners\"]\n\tclean = "+h.program(t, "global")+"\n[core]\n\tfsmonitor = "+h.program(t, "globalmonitor")+"\n")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "filter.fromenv.clean")
	t.Setenv("GIT_CONFIG_VALUE_0", h.program(t, "env"))
	t.Setenv("GIT_EXTERNAL_DIFF", h.program(t, "envdiff"))

	// Every tracked file changed in size after the index was written, so any read has to look inside it.
	for _, name := range []string{"a.txt", "src/b.txt", "c.txt", "d.txt", "e.txt", "g.txt", "h.txt"} {
		writeRepoFile(t, dir, name, "changed, and longer than before\n")
	}
	return h
}

// A repo's own config may name programs (and so may the owner's, and the environment): reading it must run none of
// them, on a git that has --attr-source and on one that has not.
func TestReadingARepoRunsNothingItsConfigNames(t *testing.T) {
	for _, mode := range []struct {
		name string
		on   bool
	}{{"with an attribute source", true}, {"without one", false}} {
		t.Run(mode.name, func(t *testing.T) {
			h := aHostileGitRepo(t)
			if out, err := exec.Command("git", "version").Output(); err != nil || (mode.on && !gitVersionAtLeast(string(out), 2, 40)) {
				t.Skip("this git has no --attr-source")
			}
			// The control: a plain diff of this repo does run its filters, so the repo is really hostile.
			if control, err := exec.Command("git", "-C", h.dir, "diff", "--numstat", "HEAD", "--").CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, control)
			}
			if ran := h.ran(); !slices.Contains(ran, "equalsclean") || !slices.Contains(ran, "infoclean") {
				t.Fatalf("the control repo's filters didn't run (%v), so this test proves nothing", ran)
			}
			withAttrSource(t, mode.on)
			state := realGitState(h.dir)
			if ran := h.ran(); len(ran) != 0 {
				t.Errorf("reading the repo ran: %v", ran)
			}
			if state.Skipped || state.Changed != 8 || state.Added != 7 || state.Removed != 9 {
				t.Errorf("the state is still read: %+v", state)
			}
			// The same through the cache the app reads with.
			if got := newGitStates().get(h.dir, 0); got.Changed != 8 {
				t.Errorf("%+v", got)
			}
			if ran := h.ran(); len(ran) != 0 {
				t.Errorf("the cached read ran: %v", ran)
			}
		})
	}
}

// aLazyRepo is a partial clone missing the blob a diff against HEAD needs, so git would fetch it from its promisor
// remote: that remote's upload-pack and the alternates' refs command (core.alternateRefsCommand) each write a marker,
// and the repo allows the file transport itself (protocol.file.allow), which beats a plain protocol.allow=never.
func aLazyRepo(t *testing.T, h *hostileRepo) string {
	t.Helper()
	dir := aGitRepo(t)
	origin, other := filepath.Join(physical(t.TempDir()), "origin"), filepath.Join(physical(t.TempDir()), "other")
	gitRepoCmd(t, dir, "clone", "-q", "--bare", "--no-local", dir, origin)
	gitRepoCmd(t, dir, "init", "-q", "--bare", other)
	blob := gitRepoCmd(t, dir, "rev-parse", "HEAD:a.txt")
	if err := os.Remove(filepath.Join(dir, ".git", "objects", blob[:2], blob[2:])); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, dir, ".git/objects/info/alternates", filepath.Join(other, "objects")+"\n")
	cfg := func(key, value string) { gitRepoCmd(t, dir, "config", key, value) }
	cfg("core.repositoryformatversion", "1")
	cfg("extensions.partialClone", "origin")
	gitRepoCmd(t, dir, "remote", "add", "origin", origin)
	cfg("remote.origin.promisor", "true")
	cfg("protocol.file.allow", "always")
	up := filepath.Join(physical(t.TempDir()), "up.sh")
	if err := os.WriteFile(up, []byte("#!/bin/sh\ntouch '"+filepath.Join(h.markers, "uploadpack")+"'\nexec git-upload-pack \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	h.names = append(h.names, "uploadpack")
	cfg("remote.origin.uploadpack", up)
	cfg("core.alternateRefsCommand", h.program(t, "alternaterefs"))
	writeRepoFile(t, dir, "a.txt", "one\ntwo\nthree\nfour\n")
	return dir
}

// A missing object in a partial clone makes git fetch it, and the fetch runs what the repo's config names for it.
// A read starts no fetch: no transport is allowed (GIT_ALLOW_PROTOCOL, protocol.allow) and no lazy fetch either
// (GIT_NO_LAZY_FETCH); and the repo isn't read at all, as its config names an upload-pack, which isn't blanked.
func TestALazyFetchStartsNothing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	h := &hostileRepo{markers: physical(t.TempDir())}
	// The control: plain git fetches, and runs both.
	control := aLazyRepo(t, h)
	if out, err := exec.Command("git", "-C", control, "diff", "--numstat", "HEAD", "--").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if ran := h.ran(); !slices.Contains(ran, "uploadpack") || !slices.Contains(ran, "alternaterefs") {
		t.Fatalf("the control fetched nothing (%v), so this test proves nothing", ran)
	}
	dir := aLazyRepo(t, h)
	state := realGitState(dir)
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("reading the repo ran: %v", ran)
	}
	if !state.Repo || !state.Skipped || (!strings.Contains(state.Note, "core.alternaterefscommand") && !strings.Contains(state.Note, "remote.*.uploadpack")) {
		t.Errorf("%+v", state)
	}
	// Below that refusal, the guards alone: the diff a read would run, with what every read is given.
	out, status, err := readGitOut(context.Background(), dir, gitCall{global: gitReadArgs}, "diff", "--numstat", "-z", "HEAD", "--")
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("the guarded diff ran: %v (%d %v %q)", ran, status, err, out)
	}
	if err != nil || status == 0 {
		t.Errorf("the blob was there after all: %d %v %q", status, err, out)
	}
}

// fakeRead is the cache on a fake git of the given version.
func fakeRead(t *testing.T, version string) (*gitStates, *fakeGit) {
	t.Helper()
	withGitHere(t, gitHereInfo{version: version})
	g, f, _ := cachedWith(t)
	return g, f
}

// Before 2.32 git can't be handed config safely, so only where the repo is gets read, and the owner is told.
func TestAnOldGitReadsOnlyWhereTheRepoIs(t *testing.T) {
	g, f := fakeRead(t, "git version 2.31.8")
	state := g.get(t.TempDir(), 0)
	if !state.Repo || state.Top == "" || !state.Skipped || !strings.Contains(state.Note, "older than 2.32") {
		t.Errorf("%+v", state)
	}
	if f.count("config") != 0 || f.count("status") != 0 || f.count("diff") != 0 {
		t.Errorf("an old git was asked more: %v", f.calls)
	}
	g, f = fakeRead(t, "git version 2.32.0")
	if state := g.get(t.TempDir(), 0); state.Skipped || f.count("status") != 1 {
		t.Errorf("2.32: %+v", state)
	}
}

// A key the read doesn't know, and nothing past the config is read: the owner is told which.
func TestAConfigKeyNotUnderstoodStopsTheRead(t *testing.T) {
	g, f := fakeRead(t, "git version 2.55.0")
	f.config = "core.bare\nfalse\x00core.gitproxy\nprog\x00"
	if state := g.get(t.TempDir(), 0); !state.Repo || !state.Skipped || !strings.Contains(state.Note, "core.gitproxy") || f.count("status") != 0 {
		t.Errorf("%+v %v", state, f.calls)
	}
	dir := aGitRepo(t)
	gitRepoCmd(t, dir, "config", "web.browser", "prog")
	if state := realGitState(dir); !state.Skipped || !strings.Contains(state.Note, "web.browser") || !strings.Contains(state.Note, "doesn't know") || state.Changed != 0 {
		t.Errorf("a real repo: %+v", state)
	}
}

// A repo owned by another account is refused by git: it is said so, unless the owner trusts it in their own config.
func TestARepoOfAnotherAccountIsSaidSoUnlessTheOwnerTrustsIt(t *testing.T) {
	dir := aGitRepo(t)
	writeRepoFile(t, dir, "a.txt", "changed\n")
	t.Setenv("GIT_TEST_ASSUME_DIFFERENT_OWNER", "1") // git's own stand-in for another account
	if exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Run() == nil {
		t.Skip("this git has no stand-in for another account")
	}
	withGitHere(t, gitHereInfo{version: "git version 2.40.0"})
	if state := realGitState(dir); !state.Repo || state.Top != dir || !state.Skipped || !strings.Contains(state.Note, "another account") {
		t.Errorf("%+v", state)
	}
	withGitHere(t, gitHereInfo{version: "git version 2.40.0", owner: [][2]string{{"safe.directory", dir}}})
	if state := realGitState(dir); state.Skipped || state.Changed != 1 {
		t.Errorf("trusted: %+v", state)
	}
	t.Setenv("GIT_TEST_ASSUME_DIFFERENT_OWNER", "0")
	if state := realGitState(physical(t.TempDir())); state.Repo || state.Note != "" {
		t.Errorf("no repo: %+v", state)
	}
}

// The owner's own git config is left out of a read, but its ignore file and trusted folders are asked for and kept.
func TestTheOwnersIgnoreFileStillHidesFiles(t *testing.T) {
	home := physical(t.TempDir())
	writeRepoFile(t, home, ".gitconfig", "[core]\n\texcludesfile = ~/ignored\n\tfsmonitor = prog\n[safe]\n\tdirectory = /some/repo\n[filter \"x\"]\n\tclean = prog\n")
	writeRepoFile(t, home, "ignored", "*.swp\n")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	here := askGitHere()
	if !gitVersionAtLeast(here.version, 2, 0) || len(here.owner) != 2 || here.owner[0] != [2]string{"core.excludesfile", "~/ignored"} ||
		here.owner[1] != [2]string{"safe.directory", "/some/repo"} {
		t.Fatalf("%+v", here)
	}
	withGitHere(t, here)
	dir := aGitRepo(t)
	writeRepoFile(t, dir, "notes.swp", "x\n")
	writeRepoFile(t, dir, "kept.txt", "x\n")
	if state := realGitState(dir); state.Skipped || state.Changed != 1 || state.Files[0].Path != "kept.txt" {
		t.Errorf("%+v", state)
	}
}

func TestBothPathsAgreeOnAnOrdinaryRepo(t *testing.T) {
	dir := aGitRepo(t)
	writeRepoFile(t, dir, "a.txt", "one\ntwo changed\n")
	writeRepoFile(t, dir, "new.txt", "n\n")
	withAttrSource(t, true)
	on := realGitState(dir)
	withAttrSource(t, false)
	off := realGitState(dir)
	on.At, off.At = 0, 0
	if on.Skipped || off.Skipped || on.Changed != 2 || on.Added != 1 || on.Removed != 2 {
		t.Fatalf("%+v", on)
	}
	if on.Files[0] != off.Files[0] || on.Files[1] != off.Files[1] || on.Head != off.Head || on.Branch != off.Branch {
		t.Errorf("%+v / %+v", on, off)
	}
}

func TestASha256RepoIsReadWithItsOwnEmptyTree(t *testing.T) {
	if out, err := exec.Command("git", "version").Output(); err != nil || !gitVersionAtLeast(string(out), 2, 40) {
		t.Skip("this git has no --attr-source")
	}
	dir := physical(t.TempDir())
	if out, err := exec.Command("git", "-C", dir, "init", "-q", "-b", "main", "--object-format=sha256").CombinedOutput(); err != nil {
		t.Skipf("no sha256 repos here: %s", out)
	}
	gitRepoCmd(t, dir, "config", "commit.gpgsign", "false")
	writeRepoFile(t, dir, "a.txt", "one\n")
	gitRepoCmd(t, dir, "add", ".")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "first")
	writeRepoFile(t, dir, "a.txt", "one\ntwo\n")
	withAttrSource(t, true)
	if state := realGitState(dir); state.Skipped || state.Changed != 1 || state.Added != 1 {
		t.Errorf("%+v", state)
	}
}

func TestGitIsReadWithNothingToAskAndNoOtherRepoNamed(t *testing.T) {
	app := []string{"PATH=/bin", "git_dir=/elsewhere", "GIT_WORK_TREE=/x", "GIT_TERMINAL_PROMPT=1", "GIT_PAGER=less", "HOME=/h",
		"GIT_CONFIG_COUNT=1", "git_config_key_0=filter.x.clean", "GIT_CONFIG_VALUE_0=prog", "GIT_CONFIG_PARAMETERS='a.b=c'", "GIT_CONFIG_GLOBAL=/g",
		"GIT_EXTERNAL_DIFF=prog", "GIT_SSH_COMMAND=prog", "GIT_ASKPASS=prog", "GIT_ATTR_SOURCE=HEAD", "GIT_ALLOW_PROTOCOL=file:ext",
		"GIT_PROTOCOL_FROM_USER=1", "GIT_NO_LAZY_FETCH=0", "GIT_EXEC_PATH=/elsewhere", "GIT_AUTHOR_NAME=kept", "GIT_TEST_SPLIT_INDEX=1", "git_test_x=1",
		"GIT_TEST_ASSUME_DIFFERENT_OWNER=1", "GIT_LITERAL_PATHSPECS=1", "GIT_GLOB_PATHSPECS=1", "GIT_NOGLOB_PATHSPECS=1", "git_icase_pathspecs=1"}
	want := "PATH=/bin HOME=/h GIT_AUTHOR_NAME=kept GIT_TEST_ASSUME_DIFFERENT_OWNER=1 GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0 GIT_PAGER=cat GIT_ALLOW_PROTOCOL= GIT_NO_LAZY_FETCH=1 " +
		"GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=" + os.DevNull
	if got := strings.Join(gitReadEnv(app, nil), " "); got != want {
		t.Errorf("%s", got)
	}
	// Config is handed over whole, a key with an "=" in its subsection included.
	got := strings.Join(gitReadEnv(app, [][2]string{{"filter.x=y.clean", ""}, {"filter.x=y.required", "false"}}), " ")
	if got != want+" GIT_CONFIG_COUNT=2 GIT_CONFIG_KEY_0=filter.x=y.clean GIT_CONFIG_VALUE_0= GIT_CONFIG_KEY_1=filter.x=y.required GIT_CONFIG_VALUE_1=false" {
		t.Errorf("%s", got)
	}
}

func TestFiltersAndConvertersAConfigNamesAreBlankedByName(t *testing.T) {
	list := "core.bare\nfalse\x00filter.lfs.clean\ngit-lfs clean\x00filter.lfs.required\ntrue\x00filter.a.B.process\nprog\x00" +
		"diff.tc.textconv\nprog\x00diff.tc.command\nprog\x00diff.tc.binary\ntrue\x00diff.algorithm\nhistogram\x00filter.x=y.clean\nprog\x00" +
		"filter.lfs.clean\nagain\x00remote.origin.url\n/r\x00protocol.file.allow\nalways\x00alias.st\n!prog\x00core.fsmonitor\nprog\x00" +
		"core.hookspath\n.husky\x00branch.main.vscode-merge-base\norigin/main\x00include.path\n/x\x00lfs.repositoryformatversion\n0\x00"
	blanks, refused := gitRepoConfig([]byte(list))
	var got []string
	for _, kv := range blanks {
		got = append(got, kv[0]+"="+kv[1])
	}
	want := "filter.lfs.clean= filter.lfs.required=false filter.a.B.process= diff.tc.textconv= diff.tc.command= filter.x=y.clean= " +
		"protocol.file.allow=never alias.st="
	if refused != "" || strings.Join(got, " ") != want {
		t.Errorf("%q: %s", refused, strings.Join(got, " "))
	}
	if blanks, refused := gitRepoConfig([]byte(list + "bare-word\x00")); refused != "a key with no section" || blanks != nil {
		t.Errorf("a record that isn't a key: %q %v", refused, blanks)
	}
	// Fail closed: a program the read doesn't blank, or a key it doesn't know, and the repo isn't read.
	for key, rule := range map[string]string{"remote.origin.uploadpack": "remote.*.uploadpack", "core.alternateRefsCommand": "core.alternaterefscommand",
		"core.gitproxy": "core.gitproxy", "remote.Odd=Name.vcs": "remote.*.vcs", "gpg.ssh.defaultkeycommand": "gpg.*.defaultkeycommand",
		"something.new": "something.new", "difftool.x.cmd": "difftool.*.cmd", "core.fsmonitor.x": "core.*.x",
		"core.unsetEnvVars": "core.unsetenvvars", "mergetool.x.cmd": "mergetool.*.cmd", "trailer.x.cmd": "trailer.*.cmd",
		"trailer.x.command": "trailer.*.command", "sendemail.smtpServer": "sendemail.smtpserver", "interactive.diffFilter": "interactive.difffilter"} {
		if blanks, refused := gitRepoConfig([]byte("core.bare\nfalse\x00" + key + "\nprog\x00")); refused != rule || blanks != nil {
			t.Errorf("%s: %q %v", key, refused, blanks)
		}
	}
	if blanks, refused := gitRepoConfig(nil); len(blanks) != 0 || refused != "" {
		t.Error("an empty config names nothing")
	}
}

// Settings that real tools write, and that are only data, are read: submodules, relative worktree paths, scalar, a
// Windows credential manager, SSH signing and the like. A key that names a program (or can) is still refused.
func TestSettingsOrdinaryToolsWriteAreRead(t *testing.T) {
	data := "submodule.active\n.\x00submodule.sub.active\ntrue\x00extensions.relativeworktrees\ntrue\x00worktree.useRelativePaths\ntrue\x00" +
		"am.keepcr\nfalse\x00commitgraph.changedPaths\ntrue\x00commitgraph.generationVersion\n1\x00gui.gcwarning\nfalse\x00receive.autogc\nfalse\x00" +
		"credential.validate\nfalse\x00credential.provider\ngeneric\x00credential.https://dev.azure.com.provider\nazure\x00" +
		"gpg.ssh.allowedSignersFile\n~/allowed\x00gpg.format\nssh\x00mergetool.keepBackup\nfalse\x00stash.showPatch\ntrue\x00help.autocorrect\nprompt\x00" +
		"versionsort.suffix\n-rc\x00format.signOff\ntrue\x00interactive.singleKey\ntrue\x00safe.bareRepository\nexplicit\x00notes.rewriteRef\nrefs/notes/*\x00" +
		"trailer.separators\n:\x00trailer.sign.key\nSigned-off-by\x00trailer.where\nafter\x00core.hooksPath\n.husky/_\x00remote.origin.gh-resolved\nbase\x00" +
		"feature.manyFiles\ntrue\x00core.untrackedCache\ntrue\x00branch.main.remote\norigin\x00lfs.repositoryformatversion\n0\x00"
	if _, refused := gitRepoConfig([]byte(data)); refused != "" {
		t.Errorf("refused %q", refused)
	}
}

// A clone with its submodules (which sets submodule.active) and a worktree with relative paths are repos like any other.
func TestARecursiveCloneAndARelativeWorktreeAreRead(t *testing.T) {
	sub := aGitRepo(t)
	super := aGitRepo(t)
	gitRepoCmd(t, super, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "sub")
	gitRepoCmd(t, super, "commit", "-q", "-m", "with a submodule")
	parent := physical(t.TempDir())
	gitRepoCmd(t, parent, "-c", "protocol.file.allow=always", "clone", "-q", "--recurse-submodules", super, "clone")
	dir := filepath.Join(parent, "clone")
	if got := gitRepoCmd(t, dir, "config", "--local", "--get", "submodule.active"); got == "" {
		t.Skipf("this git sets no submodule.active on a recursive clone")
	}
	writeRepoFile(t, dir, "a.txt", "one\ntwo\nthree\nfour\n")
	if state := realGitState(dir); state.Skipped || state.Changed != 1 || state.Added != 1 {
		t.Errorf("a recursive clone: %+v", state)
	}
	if state := realGitState(filepath.Join(dir, "sub")); state.Skipped || !state.Repo {
		t.Errorf("inside its submodule: %+v", state)
	}

	if out, err := exec.Command("git", "version").Output(); err != nil || !gitVersionAtLeast(string(out), 2, 48) {
		t.Skip("this git has no relative worktree paths (2.48)")
	}
	main := aGitRepo(t)
	tree := filepath.Join(physical(t.TempDir()), "hire")
	gitRepoCmd(t, main, "worktree", "add", "-q", "--relative-paths", "-b", "hire", tree)
	writeRepoFile(t, tree, "a.txt", "one\ntwo\nthree\nfour\n")
	if state := realGitState(tree); state.Skipped || state.Changed != 1 || state.Added != 1 {
		t.Errorf("a relative worktree: %+v", state)
	}
	if state := realGitState(main); state.Skipped {
		t.Errorf("its main repo: %+v", state)
	}
}

func TestTheGitVersionDecidesTheAttributeSource(t *testing.T) {
	for text, want := range map[string]bool{
		"git version 2.55.0\n": true, "git version 2.40.0": true, "git version 2.39.5 (Apple Git-154)": false,
		"git version 2.43.0.windows.1": true, "git version 3.0.1": true, "git version 1.9.0": false, "": false, "git version x.y": false,
	} {
		if got := gitVersionAtLeast(text, 2, 40); got != want {
			t.Errorf("%q: %v", text, got)
		}
	}
}

func TestAnAnswerOverTheCapStopsTheCopy(t *testing.T) {
	var out gitOutput
	chunk := make([]byte, gitOutputCap/2+1)
	if _, err := out.Write(chunk); err != nil {
		t.Fatal(err)
	}
	if _, err := out.Write(chunk); !errors.Is(err, errGitTooBig) || !out.over {
		t.Errorf("%v %v", err, out.over)
	}
}

// fakeGit answers like git would for a repo whose git folder is dir, counting its commands, and may block.
type fakeGit struct {
	mu      sync.Mutex
	dir     string
	calls   map[string]int
	changes string // the status's entries
	config  string // the config's list
	fail    map[string]error
	gate    chan struct{}
}

func (f *fakeGit) read(_ context.Context, _ string, _ gitCall, args ...string) ([]byte, int, error) {
	f.mu.Lock()
	f.calls[args[0]]++
	err, gate := f.fail[args[0]], f.gate
	f.mu.Unlock()
	if gate != nil && args[0] == "status" {
		<-gate
	}
	if err != nil {
		return nil, 0, err
	}
	switch args[0] {
	case "rev-parse":
		return []byte("/repo\n" + f.dir + "\n"), 0, nil
	case "config":
		return []byte(f.config), 0, nil
	case "status":
		return []byte("# branch.oid abc\x00# branch.head main\x00" + f.changes), 0, nil
	}
	return []byte("1\t1\ta.txt\x00"), 0, nil
}

func (f *fakeGit) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[name]
}

func cachedWith(t *testing.T) (*gitStates, *fakeGit, *time.Time) {
	t.Helper()
	f := &fakeGit{dir: t.TempDir(), calls: map[string]int{}, fail: map[string]error{}, changes: "1 .M N... 1 1 1 a b a.txt\x00"}
	writeRepoFile(t, f.dir, "index", "i")
	writeRepoFile(t, f.dir, "HEAD", "ref: refs/heads/main\n")
	clock := time.Unix(1_000_000, 0)
	return &gitStates{byDir: map[string]*gitEntry{}, read: f.read, now: func() time.Time { return clock }}, f, &clock
}

func TestTheStateIsKeptFiveSecondsThenOnlyReadWhenSomethingMoved(t *testing.T) {
	g, f, clock := cachedWith(t)
	folder := t.TempDir()
	g.get(folder, 1)
	*clock = clock.Add(4 * time.Second)
	g.get(folder, 99) // inside the 5 seconds: kept, whatever moved
	if f.count("status") != 1 {
		t.Fatalf("inside the window: %d reads", f.count("status"))
	}
	*clock = clock.Add(10 * time.Second)
	g.get(folder, 1)
	if f.count("status") != 1 {
		t.Errorf("nothing moved: %d reads", f.count("status"))
	}
	g.get(folder, 2) // the session's last event moved
	if f.count("status") != 2 {
		t.Errorf("an event moved: %d reads", f.count("status"))
	}
	*clock = clock.Add(10 * time.Second)
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(f.dir, "index"), later, later); err != nil {
		t.Fatal(err)
	}
	g.get(folder, 2) // the index moved
	if f.count("status") != 3 {
		t.Errorf("the index moved: %d reads", f.count("status"))
	}
	*clock = clock.Add(10 * time.Second)
	g.get(folder, 2)
	*clock = clock.Add(gitStateMaxAge)
	g.get(folder, 2) // nothing seems to have moved, but it is a minute old
	if f.count("status") != 4 {
		t.Errorf("a minute old: %d reads", f.count("status"))
	}
}

func TestAskersTogetherShareOneRead(t *testing.T) {
	g, f, _ := cachedWith(t)
	f.gate = make(chan struct{})
	folder := t.TempDir()
	var wg sync.WaitGroup
	answers := make([]GitState, 5)
	for i := range answers {
		wg.Add(1)
		go func() { defer wg.Done(); answers[i] = g.get(folder, 0) }()
	}
	time.Sleep(50 * time.Millisecond)
	close(f.gate)
	wg.Wait()
	if f.count("status") != 1 || f.count("rev-parse") != 1 {
		t.Errorf("reads: %v", f.calls)
	}
	for _, a := range answers {
		if a.Changed != 1 || a.Added != 1 {
			t.Errorf("%+v", a)
		}
	}
}

func TestWhatGitCannotAnswerIsSaidNotHidden(t *testing.T) {
	for _, c := range []struct {
		command string
		err     error
		note    string
	}{
		{"status", errGitTooBig, "too many changes to list"},
		{"rev-parse", exec.ErrNotFound, "isn't installed"},
		{"status", context.DeadlineExceeded, "too long"},
		{"status", errors.New("boom"), "couldn't be read"},
	} {
		g, f, _ := cachedWith(t)
		f.fail[c.command] = c.err
		state := g.get(t.TempDir(), 0)
		if !state.Skipped || !strings.Contains(state.Note, c.note) || state.Files == nil {
			t.Errorf("%v: %+v", c.err, state)
		}
	}
	g, f, _ := cachedWith(t)
	f.fail["diff"] = errGitTooBig
	if state := g.get(t.TempDir(), 0); !state.Skipped || state.Changed != 1 || state.Added != 0 || !strings.Contains(state.Note, "too many changes") {
		t.Errorf("the count skipped: %+v", state)
	}
}

func TestOnlyTheNewestFoldersAreKept(t *testing.T) {
	g, _, clock := cachedWith(t)
	for i := 0; i < gitStatesKept+10; i++ {
		*clock = clock.Add(time.Second)
		g.get(t.TempDir(), 0)
	}
	if len(g.byDir) > gitStatesKept {
		t.Errorf("%d kept", len(g.byDir))
	}
}

func TestTheDeskRouteIsTheWindowsAndTheGMReadsAHireInItsWorktree(t *testing.T) {
	repo := aGitRepo(t)
	worktree := filepath.Join(physical(t.TempDir()), "wt")
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/bo", worktree)
	writeRepoFile(t, worktree, "w.txt", "w\n")
	writeRepoFile(t, repo, "r.txt", "r\n")

	m := newMailDeck(t)
	d := m.d
	var file map[string]any
	current, err := os.ReadFile(registryPath()) // with the hires' keys newMailDeck stored in it
	if err != nil || json.Unmarshal(current, &file) != nil {
		t.Fatal("the registry", err)
	}
	file["hires"] = append(file["hires"].([]any),
		map[string]any{"id": "h1", "name": "Bo", "role": "builder", "state": "working", "folder": repo, "repo": repo, "worktree": worktree})
	data, _ := json.Marshal(file)
	if err := writeRegistryFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	phone := pair(t, d)
	hook := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(jsonLine(map[string]any{"hook_event_name": "UserPromptSubmit",
		"session_id": "s1", "cwd": repo})))
	hook.Host, hook.RemoteAddr = "127.0.0.1:7710", thisComputer
	hook.Header.Set(hookKeyHeader, d.access.HookKey)
	hook.Header.Set("X-AgentDeck-Agent", "claude")
	hook.Header.Set("X-AgentDeck-Tab", "tty:/dev/ttys003")
	if w := httptest.NewRecorder(); func() int { (&router{deck: d}).ServeHTTP(w, hook); return w.Code }() != http.StatusNoContent {
		t.Fatal("the hook")
	}
	window := func(target string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "GET", target: target + "&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
	}
	if got := window("/desk/git?session=s1"); got.Code != http.StatusOK || decoded(t, got)["branch"] != "main" || decoded(t, got)["top"] != repo {
		t.Errorf("window: %d %s", got.Code, got.Body)
	}
	if got := window("/desk/git?session=gone"); got.Code != http.StatusNotFound {
		t.Errorf("gone: %d", got.Code)
	}
	if refused := serve(t, d, call{method: "GET", target: "/desk/git?session=s1&t=" + phone, host: deckOnWiFi, from: aPhone}); refused.Code != http.StatusForbidden ||
		strings.TrimSpace(refused.Body.String()) != "The git state shows only in the office's window on your computer." {
		t.Errorf("phone: %d %s", refused.Code, refused.Body)
	}
	gm := m.call(t, asGM, "GET", "/setup/gm/git?hire=h1", "")
	if gm.Code != http.StatusOK || decoded(t, gm)["branch"] != "hire/bo" || decoded(t, gm)["top"] != worktree {
		t.Errorf("gm: %d %s", gm.Code, gm.Body)
	}
	for target, code := range map[string]int{"/setup/gm/git?hire=nobody": 404, "/setup/gm/git": 400} {
		if got := m.call(t, asGM, "GET", target, ""); got.Code != code {
			t.Errorf("%s: %d", target, got.Code)
		}
	}
	if got := m.call(t, asGM, "POST", "/setup/gm/git?hire=h1", ""); got.Code == http.StatusOK {
		t.Errorf("a POST was answered: %d", got.Code)
	}
	if got := m.call(t, asRev, "GET", "/setup/gm/git?hire=h1", ""); got.Code != http.StatusForbidden {
		t.Errorf("a hire's key: %d", got.Code)
	}
}
