package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// FXAGENTHOMEMAC: the Mac app now gives the core XDG_CONFIG_HOME. On a Mac, git's own ignore file in a moved config
// folder hides nothing from a read; ~/.config/git's, which a desk's sandbox holds, still does. Elsewhere, where no
// desk's sandbox runs yet, a read takes the moved folder's, as the owner's git does and the core always did
// (FXAGENTHOMEMACR1, O1). The owner's own config there is still read for their ignore file and trusted folders, as
// their git reads it.
func TestAMovedConfigFoldersIgnoreFileHidesNothingFromARead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	dir := aGitRepo(t) // made before the stand-in owner's files, which would hide its own
	home := physical(t.TempDir())
	moved := filepath.Join(home, "dotfiles", "config")
	writeRepoFile(t, home, ".config/git/ignore", "*.swp\n")
	writeRepoFile(t, moved, "git/ignore", "*.txt\n") // as a desk could write it
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", moved)
	withGitHere(t, gitHereInfo{version: "git version 2.40.0"})
	writeRepoFile(t, dir, "notes.swp", "x\n")
	writeRepoFile(t, dir, "kept.txt", "x\n")
	shown := "kept.txt"
	if runtime.GOOS != "darwin" {
		shown = "notes.swp"
	}
	if state := realGitState(dir); state.Skipped || state.Changed != 1 || state.Files[0].Path != shown {
		t.Errorf("%+v", state)
	}
	// The owner's settings, from their config in the moved folder (and none in ~/.config/git's).
	writeRepoFile(t, moved, "git/config", "[core]\n\texcludesfile = ~/ignored\n[safe]\n\tdirectory = /some/repo\n")
	writeRepoFile(t, home, ".config/git/config", "[core]\n\texcludesfile = ~/wrong\n")
	if here := askGitHere(); len(here.owner) != 2 || here.owner[0] != [2]string{"core.excludesfile", "~/ignored"} ||
		here.owner[1] != [2]string{"safe.directory", "/some/repo"} {
		t.Errorf("the owner's settings: %+v", here)
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
		"GIT_TEST_ASSUME_DIFFERENT_OWNER=1", "GIT_LITERAL_PATHSPECS=1", "GIT_GLOB_PATHSPECS=1", "GIT_NOGLOB_PATHSPECS=1", "git_icase_pathspecs=1",
		"XDG_CONFIG_HOME=/moved"}
	want := "PATH=/bin HOME=/h GIT_AUTHOR_NAME=kept GIT_TEST_ASSUME_DIFFERENT_OWNER=1 "
	if runtime.GOOS != "darwin" {
		want += "XDG_CONFIG_HOME=/moved " // dropped only where a desk's sandbox runs (FXAGENTHOMEMACR1, O1)
	}
	want += "GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0 GIT_PAGER=cat GIT_ALLOW_PROTOCOL= GIT_NO_LAZY_FETCH=1 " +
		"GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=" + os.DevNull + " GIT_NO_REPLACE_OBJECTS=1 GIT_GRAFT_FILE="
	// FXREPLACEREFS: gitReadAround, last. FXWINWORKTREER3: on Windows, the grafts file is the null device, and git's
	// hint about it is off, after gitReadAround.
	keys := []string{"core.commitGraph", "core.multiPackIndex"}
	counts := [2]string{" GIT_CONFIG_COUNT=2 ", " GIT_CONFIG_COUNT=4 "}
	if runtime.GOOS == "windows" {
		want += os.DevNull
		keys = append(keys, "advice.graftFileDeprecated")
		counts = [2]string{" GIT_CONFIG_COUNT=3 ", " GIT_CONFIG_COUNT=5 "}
	}
	around := func(n int) string { // keys, each false, numbered from n
		var each []string
		for i, key := range keys {
			each = append(each, fmt.Sprintf("GIT_CONFIG_KEY_%d=%s GIT_CONFIG_VALUE_%d=false", n+i, key, n+i))
		}
		return strings.Join(each, " ")
	}
	if got := strings.Join(gitReadEnv(app, nil), " "); got != want+counts[0]+around(0) {
		t.Errorf("%s", got)
	}
	// Config is handed over whole, a key with an "=" in its subsection included.
	got := strings.Join(gitReadEnv(app, [][2]string{{"filter.x=y.clean", ""}, {"filter.x=y.required", "false"}}), " ")
	if got != want+counts[1]+"GIT_CONFIG_KEY_0=filter.x=y.clean GIT_CONFIG_VALUE_0= GIT_CONFIG_KEY_1=filter.x=y.required GIT_CONFIG_VALUE_1=false "+around(2) {
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

// git-lfs reads the lfs section, and the owner's own git runs git-lfs (their global filter.lfs): each lfs key that
// names a program is blanked by name, whatever its subsection's case, dots or "=", and the repo is still read.
func TestLfsKeysThatNameAProgramAreBlanked(t *testing.T) {
	for _, key := range []string{
		"lfs.customtransfer.standin.path", "lfs.customTransfer.StandIn.PATH", "lfs.customtransfer.standin.args", // a custom transfer
		"lfs.standalonetransferagent", "lfs.StandaloneTransferAgent", // the one used without asking the server
		"lfs.https://lfs.example.invalid/a.b/.standalonetransferagent", "lfs.file:///x=y.standalonetransferagent", // per URL
		"lfs.extension.standin.clean", "lfs.extension.standin.smudge", "lfs.extension.Odd=Name.v2.Clean", // an extension
	} {
		blanks, refused := gitRepoConfig([]byte("core.bare\nfalse\x00lfs.url\nhttps://lfs.example.invalid/\x00" + key + "\nstandin-program --x\x00"))
		if refused != "" || len(blanks) != 1 || blanks[0] != [2]string{key, ""} {
			t.Errorf("%s: %q %v", key, refused, blanks)
		}
	}
	// git-lfs's other program, core.askpass, is outside lfs and still refused.
	if blanks, refused := gitRepoConfig([]byte("core.askPass\nstandin-askpass\x00")); refused != "core.askpass" || blanks != nil {
		t.Errorf("core.askpass: %q %v", refused, blanks)
	}
}

// The rest of git-lfs-config(5) (and what git-lfs writes itself) is data: read, and nothing blanked.
func TestLfsDataKeysAreStillRead(t *testing.T) {
	data := "lfs.url\nhttps://lfs.example.invalid/\x00lfs.pushurl\nhttps://lfs.example.invalid/\x00lfs.repositoryformatversion\n0\x00" +
		"lfs.https://lfs.example.invalid/.access\nbasic\x00lfs.https://lfs.example.invalid/.locksverify\ntrue\x00" +
		"lfs.https://lfs.example.invalid/.contenttype\nfalse\x00lfs.https://lfs.example.invalid/.activitytimeout\n5\x00" +
		"lfs.customtransfer.standin.concurrent\nfalse\x00lfs.customtransfer.standin.direction\ndownload\x00" +
		"lfs.extension.standin.priority\n0\x00lfs.fetchinclude\na/*\x00lfs.fetchexclude\nb/*\x00lfs.transfer.maxretries\n3\x00" +
		"lfs.transfer.https://lfs.example.invalid/.httpdownloadencoding\nzstd\x00lfs.storage\nlfs2\x00lfs.sshtransfer\nnever\x00" +
		"lfs.ssh.automultiplex\nfalse\x00lfs.locksverify\nfalse\x00lfs.setlockablereadonly\nfalse\x00lfs.concurrenttransfers\n4\x00" +
		"lfs.basictransfersonly\ntrue\x00lfs.remote.searchall\ntrue\x00lfs.pathfiltercachesize\nnone\x00lfs.defaulttokenttl\n1h\x00"
	if blanks, refused := gitRepoConfig([]byte(data)); refused != "" || len(blanks) != 0 {
		t.Errorf("%q %v", refused, blanks)
	}
}

// Through a real git: the keys as git lists them (subsections kept as written) are blanked on every read after the
// config, and the repo is read.
func TestARepoWithLfsProgramsIsReadWithThemBlanked(t *testing.T) {
	dir := aGitRepo(t)
	keys := []string{"lfs.customTransfer.StandIn.path", "lfs.customTransfer.StandIn.args", "lfs.standalonetransferagent",
		"lfs.https://lfs.example.invalid/a.b/.standalonetransferagent", "lfs.extension.standin.clean", "lfs.extension.standin.smudge"}
	for _, key := range keys {
		gitRepoCmd(t, dir, "config", key, "standin-program")
	}
	gitRepoCmd(t, dir, "config", "lfs.url", "https://lfs.example.invalid/")
	writeRepoFile(t, dir, "a.txt", "one\n")
	given := map[string][][2]string{}
	read := func(ctx context.Context, dir string, call gitCall, args ...string) ([]byte, int, error) {
		given[args[0]] = call.config
		return readGitOut(ctx, dir, call, args...)
	}
	state, _ := readGitState(dir, read, time.Now())
	if !state.Repo || state.Skipped || state.Changed != 1 {
		t.Fatalf("%+v", state)
	}
	for _, command := range []string{"status", "diff"} {
		for _, key := range keys {
			if !slices.Contains(given[command], [2]string{key, ""}) {
				t.Errorf("%s: %s isn't blanked: %v", command, key, given[command])
			}
		}
		// Data is handed over as the repo had it (gitstatefolder.go), never blanked.
		if slices.ContainsFunc(given[command], func(kv [2]string) bool { return kv[0] == "lfs.url" && kv[1] != "https://lfs.example.invalid/" }) {
			t.Errorf("%s: lfs.url is data, not blanked", command)
		}
	}
}

// A remote's address, or an insteadOf's, that makes the owner's git run a program (ext::, fd::, any helper's
// "<name>::", a scheme git doesn't serve itself) refuses the read, in each key form and either case; plain
// addresses are still read. Stand-in values only.
func TestAnAddressThatRunsAProgramIsRefused(t *testing.T) {
	refusedRows := map[string]string{
		"remote.origin.url\next::standin-program %S": "remote.*.url", "remote.origin.url\nEXT::standin-program": "remote.*.url",
		"remote.Odd=Name.v2.url\nfd::3": "remote.*.url", "remote.origin.url\nstandin::x": "remote.*.url",
		"remote.origin.url\nstandin://x": "remote.*.url", "remote.origin.url\nSSH://standin.invalid/x": "remote.*.url",
		"remote.origin.url\n9z::x": "remote.*.url", "remote.origin.URL\nExT::standin-program": "remote.*.url",
		"remote.origin.pushurl\next::standin-program": "remote.*.pushurl", "remote.origin.pushURL\nEXT::standin-program": "remote.*.pushurl",
		"remote.origin.pushurl\na+b.c-d://x":           "remote.*.pushurl",
		"url.ext::standin-program.insteadof\nhttps://": "url.*.insteadof", "url.EXT::standin-program.insteadOf\nhttps://": "url.*.insteadof",
		"url.ext::standin-program.pushinsteadof\nhttps://": "url.*.pushinsteadof", "url.EXT::standin-program.pushInsteadOf\ngh:": "url.*.pushinsteadof",
		// a base that becomes one with the rest of the address after it ("ex" + "t::…")
		"url..insteadof\n/": "url.*.insteadof", "url.ex.insteadof\n/": "url.*.insteadof", "url.ext:.insteadof\n/": "url.*.insteadof",
		"url.stand:/.pushinsteadof\n/": "url.*.pushinsteadof", "url.HTTPS://h/.insteadof\n/": "url.*.insteadof",
		// ":" for "/" in "/:x" is "::x", and for "https" in "https://h/x" ":://h/x": the helper named "" (git-remote-)
		"url.:.insteadof\n/": "url.*.insteadof", "url.:.pushinsteadof\nhttps": "url.*.pushinsteadof",
		// a submodule's address, the remote a branch fetches from or pushes to and the one pushes go to (SIERRA's ruling)
		"submodule.s.url\next::standin-program": "submodule.*.url", "submodule.Odd=Name.v2.URL\nstandin://x": "submodule.*.url",
		"branch.main.remote\nstandin::x": "branch.*.remote", "branch.Main.Remote\nEXT::standin-program": "branch.*.remote",
		"branch.main.pushremote\nfd::3": "branch.*.pushremote", "branch.main.pushRemote\nSSH://standin.invalid/x": "branch.*.pushremote",
		"remote.pushdefault\nstandin::x": "remote.pushdefault", "remote.pushDefault\nExT::standin-program": "remote.pushdefault",
		// an insteadOf's value that is one itself
		"url.https://standin.invalid/.insteadof\next::standin-program":     "url.*.insteadof",
		"url.https://standin.invalid/.pushinsteadof\nEXT::standin-program": "url.*.pushinsteadof",
	}
	for row, rule := range refusedRows {
		if blanks, refused := gitRepoConfig([]byte("core.bare\nfalse\x00" + row + "\x00")); refused != rule+gitRefusedAddress || blanks != nil {
			t.Errorf("%q: %q %v", row, refused, blanks)
		}
	}
	// Plain addresses, in each key form, are data: read, nothing blanked.
	data := "remote.origin.url\nhttps://standin.invalid/a/b.git\x00remote.origin.url\nssh://git@standin.invalid/a/b.git\x00" +
		"remote.origin.url\ngit@standin.invalid:a/b.git\x00remote.origin.url\n/a/b\x00remote.origin.url\n../b\x00remote.origin.url\nfile:///a/b\x00" +
		"remote.origin.url\nssh://[::1]/a\x00remote.origin.url\ngit://standin.invalid/a\x00remote.origin.url\ngit+ssh://h/a\x00" +
		"remote.origin.url\nhttp://h/a\x00remote.origin.url\nftps://h/a\x00remote.origin.url\nC:\\a\\b\x00remote.origin.url\nhost:x::y\x00" +
		"remote.origin.pushurl\nhttps://standin.invalid/a/b.git\x00remote.origin.pushurl\ngit@standin.invalid:a/b.git\x00" +
		"url.https://github.com/.insteadof\ngh:\x00url.git@github.com:.insteadof\nhttps://github.com/\x00url.ssh://git@h/.insteadOf\nhttps://h/\x00" +
		"url.https://github.com/.pushinsteadof\ngit@github.com:\x00url./srv/git/.pushInsteadOf\nsrv:\x00url.git@h:x.insteadof\next\x00" +
		"submodule.s.url\nhttps://standin.invalid/s.git\x00submodule.s.url\n../s.git\x00submodule.s.url\ngit@standin.invalid:s\x00" +
		"branch.main.remote\norigin\x00branch.main.remote\n.\x00branch.main.pushremote\norigin\x00branch.main.remote\nhttps://standin.invalid/x\x00" +
		"remote.pushdefault\norigin\x00remote.pushdefault\nstandin-name\x00remote.pushDefault\ngit@standin.invalid:x\x00"
	if blanks, refused := gitRepoConfig([]byte(data)); refused != "" || len(blanks) != 0 {
		t.Errorf("plain addresses: %q %v", refused, blanks)
	}
	// Only these forms are addresses: the same values elsewhere are what they were.
	for _, row := range []string{"remote.origin.fetch\next::x", "url.ext::x.other\ny", "lfs.url\next::x", "submodule.s.path\next::x",
		"remote.url\nx", "branch.main.merge\next::x", "branch.main.description\next::x", "submodule.s.branch\next::x"} {
		_, wasRefused := gitRepoConfig([]byte(row + "\x00"))
		if strings.HasSuffix(wasRefused, gitRefusedAddress) {
			t.Errorf("%q: %q", row, wasRefused)
		}
	}
}

// gitAddressRuns reads an address as git does: for each stand-in address, git (refused every transport, so it runs
// nothing and reaches no host) names the transport it picked, and the address counts exactly when that transport
// is a program's (anything but git's own). Then insteadOf bases put before the rest of an address, both forms.
func TestAnAddressIsReadAsGitPicksItsTransport(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	own := map[string]bool{"file": true, "git": true, "ssh": true, "http": true, "https": true, "ftp": true, "ftps": true}
	picked := func(dir string, args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		// Only a transport no address names is allowed. (Empty, as gitReadEnv gives it, lets a transport named "" by
		// "::x" through: git then looks for a git-remote- program.)
		cmd.Env = append(gitReadEnv(os.Environ(), nil), "GIT_ALLOW_PROTOCOL=standin-none")
		out, _ := cmd.CombinedOutput()
		_, after, found := strings.Cut(string(out), "transport '")
		name, _, closed := strings.Cut(after, "' not allowed")
		if !found || !closed {
			t.Fatalf("git %v named no transport: %s", args, out)
		}
		return name
	}
	dir := aGitRepo(t)
	for _, address := range []string{"standin::x", "STANDIN::x", "9z::x", "a.b::x", "a+b-c::x", "fd::3", "::x", "standin://x",
		"SSH://standin.invalid/x", "HTTPS://standin.invalid/x", "a+b://x", "https://standin.invalid/x", "http://standin.invalid/x",
		"ssh://[::1]/x", "ssh://standin.invalid/x", "git+ssh://standin.invalid/x", "ssh+git://standin.invalid/x",
		"git://standin.invalid/x", "ftp://standin.invalid/x", "ftps://standin.invalid/x", "git@standin.invalid:x", "standin.invalid:x::y",
		"file:///standin/none", "/standin/none", "gh:x",
		// git trims nothing: a space or tab first is a host's name for ssh, and one after "::" is the helper's to read
		" standin::x", "\tstandin::x", "standin:: x", "standin::", "C::x", "::", "standin:x", "file:/x", "file:"} {
		name := picked(dir, "ls-remote", address)
		if got := gitAddressRuns(address, false); got != !own[name] {
			t.Errorf("%q: git picks %q, counted %v", address, name, got)
		}
	}
	// The base an insteadOf puts in place of "/": "stand" + "in::x" is a helper's address, though "stand" alone isn't.
	for base, address := range map[string]string{"stand": "/in::x", "standin": "/::x", "standin:": "/:x", "standin:/": "//x", "": "/standin::x",
		":": "/:x", "https://standin.invalid/": "/x", "git@standin.invalid:": "/x", "/standin/": "/in::x"} {
		name := picked(dir, "-c", "url."+base+".insteadOf=/", "ls-remote", address)
		if got := gitSettingAddressRuns("url."+base+".insteadOf", "/"); got != !own[name] {
			t.Errorf("url.%s.insteadOf before %q: git picks %q, counted %v", base, address, name, got)
		}
		name = picked(dir, "-c", "url."+base+".pushInsteadOf=/", "push", address, "main")
		if got := gitSettingAddressRuns("url."+base+".pushInsteadOf", "/"); got != !own[name] {
			t.Errorf("url.%s.pushInsteadOf before %q: git push picks %q, counted %v", base, address, name, got)
		}
	}
	// The owner's own remote, a plain https one, reached through a repo's insteadOf of ":" (fetch and push), and a
	// remote's address set with a space first in config: what git picks for each, against what the read counts.
	for _, row := range []struct {
		setting, key, value string
		args                []string
		program             bool
	}{
		{"", "remote.standin.url", "https://standin.invalid/x", []string{"fetch", "standin"}, false},
		{"url.:.insteadOf=https", "url.:.insteadOf", "https", []string{"fetch", "standin"}, true},
		{"url.:.pushInsteadOf=https", "url.:.pushInsteadOf", "https", []string{"push", "standin", "main"}, true},
		{"remote.spaced.url= standin::x", "remote.spaced.url", " standin::x", []string{"ls-remote", "spaced"}, false},
		// a branch's remote, its push remote and remote.pushDefault: an address git uses as it is, or a remote's name
		{"branch.main.remote=standin::x", "branch.main.remote", "standin::x", []string{"fetch"}, true},
		{"branch.main.remote=standin::x", "branch.main.remote", "standin::x", []string{"pull"}, true},
		{"branch.main.remote=standin::x", "branch.main.remote", "standin::x", []string{"ls-remote"}, true},
		{"branch.main.remote=standin", "branch.main.remote", "standin", []string{"fetch"}, false},
		{"branch.main.pushRemote=standin::x", "branch.main.pushRemote", "standin::x", []string{"push"}, true},
		{"branch.main.pushRemote=standin", "branch.main.pushRemote", "standin", []string{"push"}, false},
		{"remote.pushDefault=standin::x", "remote.pushDefault", "standin::x", []string{"push"}, true},
		{"remote.pushDefault=standin", "remote.pushDefault", "standin", []string{"push"}, false},
	} {
		args := []string{"-c", "remote.standin.url=https://standin.invalid/x", "-c", "push.default=current"}
		if row.setting != "" {
			args = append(args, "-c", row.setting)
		}
		name := picked(dir, append(args, row.args...)...)
		if counted := gitSettingAddressRuns(row.key, row.value); !own[name] != row.program || counted != row.program {
			t.Errorf("%q, git %v: git picks %q, counted %v", row.setting, row.args, name, counted)
		}
	}
	// A submodule's address, as `git submodule update` clones it (git refuses the transport, so nothing is cloned).
	sub := aGitRepo(t)
	gitRepoCmd(t, sub, "update-index", "--add", "--cacheinfo", "160000,"+gitRepoCmd(t, sub, "rev-parse", "HEAD")+",s")
	writeRepoFile(t, sub, ".gitmodules", "[submodule \"s\"]\n\tpath = s\n\turl = https://standin.invalid/s\n")
	for value, program := range map[string]bool{"standin::x": true, "EXT::standin-program": true, "https://standin.invalid/s": false} {
		name := picked(sub, "-c", "submodule.s.url="+value, "submodule", "update")
		if counted := gitSettingAddressRuns("submodule.s.url", value); !own[name] != program || counted != program {
			t.Errorf("submodule.s.url=%s: git picks %q, counted %v", value, name, counted)
		}
	}
}

// Through a real git: a repo whose remote runs a program isn't read past its config, and the owner is told why; the
// same repo with a plain address is read.
func TestARepoWithAnAddressThatRunsAProgramIsNotRead(t *testing.T) {
	dir := aGitRepo(t)
	writeRepoFile(t, dir, "a.txt", "one\n")
	for key, value := range map[string]string{"remote.origin.url": "EXT::standin-program", "remote.origin.pushurl": "ext::standin-program",
		"url.ext::standin-program.insteadOf": "https://standin.invalid/", "url.ex.pushInsteadOf": "/", "url.:.insteadOf": "https",
		"submodule.s.url": "standin::x", "branch.main.remote": "EXT::standin-program", "branch.main.pushRemote": "standin::x",
		"remote.pushDefault": "fd::3"} {
		gitRepoCmd(t, dir, "config", key, value)
		rule := key[:strings.Index(key, ".")] + ".*." + strings.ToLower(key[strings.LastIndex(key, ".")+1:])
		if strings.Count(key, ".") == 1 {
			rule = strings.ToLower(key)
		}
		if state := realGitState(dir); !state.Repo || !state.Skipped || state.Changed != 0 ||
			state.Note != "The changes aren't read: this repo's git config gives an address that runs a program ("+rule+")." {
			t.Errorf("%s: %+v", key, state)
		}
		gitRepoCmd(t, dir, "config", "--unset", key)
	}
	gitRepoCmd(t, dir, "config", "remote.origin.url", "https://standin.invalid/a/b.git")
	gitRepoCmd(t, dir, "config", "url.https://standin.invalid/.insteadOf", "gh:")
	if state := realGitState(dir); state.Skipped || state.Changed != 1 {
		t.Errorf("a plain address: %+v", state)
	}
	g, f := fakeRead(t, "git version 2.55.0")
	f.config = "core.bare\nfalse\x00remote.origin.url\next::standin-program\x00"
	if state := g.get(t.TempDir(), 0); !state.Skipped || f.count("status") != 0 || f.count("diff") != 0 {
		t.Errorf("past the config: %+v %v", state, f.calls)
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

// FXWINWORKTREER4: a listing read as it comes (gitRecords) gives each whole record once, however git's writes cut
// it, past gitOutputCap; a record past gitRecordMost, a listing past its bound, a last record cut short, or its
// reader's error stops it, and every write after.
func TestAListingReadAsItComesGivesEachRecordWhole(t *testing.T) {
	var got []string
	taken := func(record []byte) error { got = append(got, string(record)); return nil }
	whole := &gitRecords{sep: 0, each: taken}
	past := strings.Repeat(strings.Repeat("x", 999)+"\x00", gitOutputCap/1000+1) // past the cap, in records of 1,000 bytes
	for _, chunk := range []string{"one\x00tw", "", "o\x00three", "\x00", past} {
		if n, err := whole.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatalf("%d %v", n, err)
		}
	}
	if err := whole.end(); err != nil || len(got) != 3+gitOutputCap/1000+1 || got[0] != "one" || got[1] != "two" || got[2] != "three" {
		t.Errorf("%v %d records %q", err, len(got), got[:min(len(got), 3)])
	}

	long := &gitRecords{sep: 0, each: taken}
	if _, err := long.Write([]byte(strings.Repeat("x", gitRecordMost))); err != nil {
		t.Fatal(err)
	}
	if _, err := long.Write([]byte("a\x00")); !errors.Is(err, errGitTooBig) {
		t.Errorf("a record past gitRecordMost: %v", err)
	}
	bounded := &gitRecords{sep: '\n', most: 10, each: taken}
	if _, err := bounded.Write([]byte("12345\n12345\n")); !errors.Is(err, errGitTooBig) || !errors.Is(bounded.end(), errGitTooBig) {
		t.Errorf("a listing past its bound: %v", err)
	}
	cut := &gitRecords{sep: '\n', each: taken}
	if _, err := cut.Write([]byte("whole\nhalf")); err != nil || cut.end() == nil {
		t.Errorf("a last record cut short: %v %v", err, cut.end())
	}
	no := errors.New("no")
	refused := &gitRecords{sep: '\n', each: func([]byte) error { return no }}
	if _, err := refused.Write([]byte("a\n")); !errors.Is(err, no) {
		t.Errorf("the reader's error: %v", err)
	}
	if _, err := refused.Write([]byte("b\n")); !errors.Is(err, no) || !errors.Is(refused.end(), no) {
		t.Errorf("a write after the stop: %v", err)
	}
}

// FXWINWORKTREER4: the fire's reader of the index (deskGitlinks) keeps a gitlink's path alone, and stops at a record
// git never gives or at a gitlink past deskGitlinksMost, so the tree is kept.
func TestTheFiresIndexReaderKeepsGitlinksAndStopsAtWhatIsntARecord(t *testing.T) {
	id := strings.Repeat("a", 40)
	links := &deskGitlinks{}
	for _, record := range []string{"100644 " + id + " 0\tsub/a.txt", "160000 " + id + " 0\tsub/mod", "120000 " + id + " 0\tlink"} {
		if err := links.take([]byte(record)); err != nil {
			t.Fatal(err)
		}
	}
	if links.records != 3 || len(links.paths) != 1 || links.paths[0] != "sub/mod" {
		t.Errorf("%d records, gitlinks %q", links.records, links.paths)
	}
	for _, record := range []string{"", "a.txt", "100644 " + id + " 0", "100644 " + id + "\ta.txt", "10064 " + id + " 0\ta.txt",
		"10064x " + id + " 0\ta.txt", "100644 " + id + " 0\t"} {
		if err := (&deskGitlinks{}).take([]byte(record)); err == nil {
			t.Errorf("%q was taken as a record", record)
		}
	}
	many := &deskGitlinks{}
	for i := range deskGitlinksMost {
		if err := many.take([]byte(fmt.Sprintf("160000 %s 0\tm/%d", id, i))); err != nil {
			t.Fatal(err)
		}
	}
	if err := many.take([]byte("160000 " + id + " 0\tm/last")); err == nil {
		t.Errorf("gitlink %d was taken", deskGitlinksMost+1)
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
	if slices.Contains(args, "--git-common-dir") { // the read's own git folder (gitstatefolder.go), counted apart
		args = append([]string{"common-dir"}, args...)
	}
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
	case "common-dir":
		return []byte(f.dir + "\nsha1\n"), 0, nil
	case "symbolic-ref":
		return []byte("refs/heads/main\n"), 0, nil
	case "for-each-ref":
		return []byte("abc refs/heads/main\n"), 0, nil
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

// FXAGENTHOMEMACR1 (O4): a read of the owner's own config takes XDG_CONFIG_HOME from the environment it's handed,
// never from the core's own, and has it once.
func TestAReadOfTheOwnersConfigTakesTheConfigFolderItsHanded(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/the-core-s")
	env := gitOwnerConfigEnv([]string{"PATH=/bin", "XDG_CONFIG_HOME=/handed", "GIT_CONFIG_GLOBAL=/g"})
	var xdg []string
	for _, kv := range env {
		if value, ok := strings.CutPrefix(kv, "XDG_CONFIG_HOME="); ok {
			xdg = append(xdg, value)
		}
		if strings.HasPrefix(kv, "GIT_CONFIG_GLOBAL=") {
			t.Errorf("the owner's global config is still off: %s", kv)
		}
	}
	if !slices.Equal(xdg, []string{"/handed"}) {
		t.Errorf("XDG_CONFIG_HOME: %q, want only /handed", xdg)
	}
}
