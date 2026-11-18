package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// FXGITSTATECFG (RT126RESOLVESWEEP S1): a desk owns the repo it reports as its project, so it can change the repo's
// config between the read's look at it and the git that reads the files. The stand-in desk here changes it at exactly
// that point: once the config was listed and judged clean, it names a program for every kind of key that makes git run
// one during a status or a diff (a filter's clean, smudge and process, a text converter and a diff command, an external
// diff, an fsmonitor, hooks, and an include of a file it wrote), and the attributes in .git/info, which no attribute
// source replaces, give every file those drivers. Each program only leaves a marker in the test's own temp folder.

// flipToHostile writes the hostile config and attributes into repo dir: what a desk would write after the check.
func (h *hostileRepo) flipToHostile(t *testing.T, dir string) {
	t.Helper()
	cfg := func(key, value string) { gitRepoCmd(t, dir, "config", key, value) }
	cfg("filter.z.clean", h.program(t, "clean"))
	cfg("filter.z.smudge", h.program(t, "smudge"))
	cfg("filter.p.process", h.program(t, "process"))
	cfg("diff.t.textconv", h.program(t, "textconv"))
	cfg("diff.t.command", h.program(t, "diffcommand"))
	cfg("diff.external", h.program(t, "diffexternal"))
	cfg("core.fsmonitor", h.program(t, "fsmonitor"))
	cfg("core.pager", h.program(t, "pager"))
	cfg("core.askPass", h.program(t, "askpass"))
	cfg("credential.helper", h.program(t, "credential"))
	hooks := physical(t.TempDir())
	for _, hook := range []string{"post-index-change", "fsmonitor-watchman", "reference-transaction", "post-checkout"} {
		if err := os.WriteFile(filepath.Join(hooks, hook), []byte("#!/bin/sh\ntouch '"+filepath.Join(h.markers, "hook")+"'\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	h.names = append(h.names, "hook")
	cfg("core.hooksPath", hooks)
	included := filepath.Join(dir, ".git", "desk-written.cfg") // a file the desk wrote, in its own repo
	if err := os.WriteFile(included, []byte("[filter \"inc\"]\n\tclean = "+h.program(t, "included")+"\n[core]\n\tfsmonitor = "+h.program(t, "includedmonitor")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg("include.path", included)
	cfg("includeIf.gitdir:"+dir+"/.path", included)
	writeRepoFile(t, dir, ".git/info/attributes", "* filter=z diff=t\nsrc/b.txt filter=p\nc.txt filter=inc\n")
	writeRepoFile(t, dir, ".gitattributes", "* filter=z diff=t\nsrc/b.txt filter=p\nc.txt filter=inc\n")
}

// aRepoADeskFlips is a clean repo, with every tracked file changed in size so a read has to look inside each one.
// Once flipped, its changes are those three files and the new .gitattributes.
func aRepoADeskFlips(t *testing.T) *hostileRepo {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	h := &hostileRepo{dir: aGitRepo(t), markers: physical(t.TempDir())}
	writeRepoFile(t, h.dir, "c.txt", "c\n")
	gitRepoCmd(t, h.dir, "add", "c.txt")
	gitRepoCmd(t, h.dir, "commit", "-q", "-m", "more")
	for _, name := range []string{"a.txt", "src/b.txt", "c.txt"} {
		writeRepoFile(t, h.dir, name, "changed, and longer than before\n")
	}
	return h
}

// The check-then-run gap: the config is clean when it is listed and judged, hostile when status and diff run. No
// program it names may run, on a git with --attr-source and on one without; and the files are still counted.
func TestAConfigChangedAfterItsCheckRunsNothing(t *testing.T) {
	for _, mode := range []struct {
		name string
		on   bool
	}{{"with an attribute source", true}, {"without one", false}} {
		t.Run(mode.name, func(t *testing.T) {
			h := aRepoADeskFlips(t)
			if out, err := exec.Command("git", "version").Output(); err != nil || !gitVersionAtLeast(string(out), 2, 32) || (mode.on && !gitVersionAtLeast(string(out), 2, 40)) {
				t.Skip("this git is too old for this mode")
			}
			withAttrSource(t, mode.on)
			flipped := false
			// The desk's hook point: right after the read's look at the config, before anything reads a file.
			read := func(ctx context.Context, dir string, call gitCall, args ...string) ([]byte, int, error) {
				out, status, err := readGitOut(ctx, dir, call, args...)
				if len(args) > 0 && args[0] == "config" && !flipped {
					flipped = true
					h.flipToHostile(t, h.dir)
				}
				return out, status, err
			}
			state, _ := readGitState(h.dir, read, time.Now())
			if !flipped {
				t.Fatalf("the read never listed the config, so the desk never flipped it: %+v", state)
			}
			if ran := h.ran(); len(ran) != 0 {
				t.Errorf("a config changed after its check ran, as the owner: %v", ran)
			}
			if !state.Repo || state.Skipped || state.Changed != 4 || state.Added != 3 || state.Removed != 5 {
				t.Errorf("the state isn't read as git reads it with no filter: %+v", state)
			}
			// The control: the flipped repo really is hostile to a plain git.
			if out, err := exec.Command("git", "-C", h.dir, "diff", "--numstat", "HEAD", "--").CombinedOutput(); err != nil {
				t.Logf("the control's diff: %v: %s", err, out)
			}
			if ran := h.ran(); !slices.Contains(ran, "clean") || !slices.Contains(ran, "included") {
				t.Fatalf("the control ran no filter (%v), so this test proves nothing", ran)
			}
		})
	}
}

// The hostile config from the start (no flip): what the check refuses or blanks is one layer; underneath it, the
// git that reads the files never reads the repo's config at all. So a key the check didn't know, had it slipped
// through, still runs nothing: here the check itself is skipped by handing readGitState a clean list.
func TestTheFilesAreReadWithOnlyTheConfigSmolcorpWrites(t *testing.T) {
	h := aRepoADeskFlips(t)
	if out, err := exec.Command("git", "version").Output(); err != nil || !gitVersionAtLeast(string(out), 2, 32) {
		t.Skip("git older than 2.32")
	}
	withAttrSource(t, false) // the worktree's .gitattributes are read too
	h.flipToHostile(t, h.dir)
	read := func(ctx context.Context, dir string, call gitCall, args ...string) ([]byte, int, error) {
		if len(args) > 0 && args[0] == "config" {
			return []byte("core.repositoryformatversion\n0\x00core.filemode\ntrue\x00core.bare\nfalse\x00"), 0, nil
		}
		return readGitOut(ctx, dir, call, args...)
	}
	state, _ := readGitState(h.dir, read, time.Now())
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("the repo's own config was read by the git that reads the files: %v", ran)
	}
	if !state.Repo || state.Skipped || state.Changed != 4 || state.Added != 3 || state.Removed != 5 {
		t.Errorf("%+v", state)
	}
}

// A desk names its session's folder itself (its hook's project): the window's /desk/git reads it only in the place
// the registry gives the desk, its worktree or below. A folder anywhere else, a repo of the desk's making with a
// filter in its config, isn't read at all; the owner's own sessions are read wherever they work.
func TestADeskIsReadOnlyInItsOwnWorktree(t *testing.T) {
	d, keys, sockets := keyedOffice(t)
	h := aRepoADeskFlips(t) // elsewhere: not the desk's
	h.flipToHostile(t, h.dir)
	worktree := filepath.Join(filepath.Dir(sockets["aaa111"]), "wt-aaa111")
	gitRepoCmd(t, filepath.Dir(worktree), "init", "-q", "-b", "main", worktree)
	nested := filepath.Join(worktree, "x")
	gitRepoCmd(t, worktree, "init", "-q", "-b", "main", nested)

	for _, c := range []struct {
		session, folder string
		read            bool
	}{{"s-out", h.dir, false}, {"s-top", worktree, true}, {"s-nested", nested, true}} {
		if code := hookAs(t, d, keys["aaa111"], tmuxTabOn(sockets["aaa111"]), "UserPromptSubmit", c.session, c.folder); code != http.StatusNoContent {
			t.Fatalf("%s: the hook: %d", c.session, code)
		}
		code, answer := d.deskGit(c.session)
		state := gitAnswer(t, answer)
		if code != http.StatusOK || state.Repo != c.read || state.Skipped == c.read {
			t.Errorf("%s (%s): %d %+v", c.session, c.folder, code, answer)
		}
		if !c.read && state.Note != "This desk names a folder outside its own worktree, so its git state isn't read." {
			t.Errorf("%s: %q", c.session, state.Note)
		}
	}
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("a folder outside the desk's worktree was read: %v", ran)
	}
	// The owner's own session (the support folder's key, a terminal's tab) is read where it works: the desk's rule
	// isn't its own. (This repo's core.askPass still stops the read, as it always did: a key a read doesn't know.)
	if code := hookAs(t, d, d.access.HookKey, "tty:/dev/ttys009", "UserPromptSubmit", "s-owner", h.dir); code != http.StatusNoContent {
		t.Fatalf("the owner's hook: %d", code)
	}
	if code, answer := d.deskGit("s-owner"); code != http.StatusOK || !gitAnswer(t, answer).Repo || !strings.Contains(gitAnswer(t, answer).Note, "(core.askpass)") {
		t.Errorf("the owner's session: %d %+v", code, answer)
	}
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("reading the owner's session's repo ran: %v", ran)
	}
}

// gitAnswer is the route's answer as the window reads it: the state, whatever goes with it (withPR).
func gitAnswer(t *testing.T, answer any) GitState {
	t.Helper()
	var state GitState
	data, err := json.Marshal(answer)
	if err != nil || json.Unmarshal(data, &state) != nil {
		t.Fatalf("%v: %s", err, data)
	}
	return state
}
