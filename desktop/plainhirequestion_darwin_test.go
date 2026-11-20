package desktop

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXSTACKG G22: one plain-hire question, plainHireSandboxed(cfg) = plainHiresSandboxed && !cfg.SandboxOff, asked by
// the office's picker, its place check and its own worktree (FXHIREPROJECTS), and by the launcher as it starts
// (FXSWITCHPLAIN). With the owner's switch off, a plain hire isn't asked which project and goes into the folder as
// before, outside its sandbox, told so; with it on, it gets its own project and worktree.

// The question is the system's launcher and the owner's switch, nothing else.
func TestThePlainHireQuestionIsTheLauncherAndTheSwitch(t *testing.T) {
	saved := plainHiresSandboxed
	t.Cleanup(func() { plainHiresSandboxed = saved })
	for _, sandboxes := range []bool{false, true} {
		plainHiresSandboxed = sandboxes
		for _, off := range []bool{false, true} {
			if got, want := plainHireSandboxed(Config{SandboxOff: off}), sandboxes && !off; got != want {
				t.Errorf("a launcher that sandboxes %v, the switch off %v: %v, want %v", sandboxes, off, got, want)
			}
		}
	}
}

// The office's picker: asked with the switch on; with it off, no question, and the launcher opens in the folder for
// hires, as before.
func TestOnAMacTheOfficesPlainHireIsAskedWhichProjectOnlyWithTheSwitchOn(t *testing.T) {
	for _, off := range []bool{false, true} {
		t.Run(map[bool]string{false: "switch on", true: "switch off"}[off], func(t *testing.T) {
			d, l, h := r7wDeck(t, true)
			d.mu.Lock()
			d.config.SandboxOff = off
			d.mu.Unlock()
			status, said := d.Perform("hire", "", url.Values{})
			if !off {
				if status != http.StatusOK || !strings.Contains(said, `"pick"`) || len(h.tmux) != 0 {
					t.Errorf("asked: %d %s, tmux %q", status, said, h.tmux)
				}
				return
			}
			if status != http.StatusNoContent {
				t.Fatalf("not asked: %d %s", status, said)
			}
			if folder, desk := r7wPlainHire(t, h); folder != l.projects || desk != "" {
				t.Errorf("its launcher in %q for desk %q, want the folder for hires and no desk", folder, desk)
			}
		})
	}
}

// The place check: a plain hire into the whole projects folder is refused at its hire with the switch on (its
// sandbox couldn't be made there), and goes in as before with it off.
func TestOnAMacAPlainHiresPlaceIsCheckedOnlyWithTheSwitchOn(t *testing.T) {
	for _, off := range []bool{false, true} {
		t.Run(map[bool]string{false: "switch on", true: "switch off"}[off], func(t *testing.T) {
			d, l, h := r7wDeck(t, true)
			d.mu.Lock()
			d.config.SandboxOff = off
			d.mu.Unlock()
			status, said := d.hireWith(hireSpec{fromSetup: true, extra: &setupHire{Folder: l.projects}})
			if !off {
				if status != http.StatusConflict || said != wholeProjectsWords(l.projects, false) || len(h.tmux) != 0 {
					t.Errorf("refused: %d %s, tmux %q", status, said, h.tmux)
				}
				return
			}
			if status != http.StatusNoContent {
				t.Fatalf("as before: %d %s", status, said)
			}
			if folder, _ := r7wPlainHire(t, h); folder != l.projects {
				t.Errorf("its launcher in %q, want %q", folder, l.projects)
			}
		})
	}
}

// Its own worktree: made with the switch on, on its own branch; with it off, none, no branch, the checkout itself.
func TestOnAMacAPlainHireGetsAWorktreeOfItsOwnOnlyWithTheSwitchOn(t *testing.T) {
	for _, off := range []bool{false, true} {
		t.Run(map[bool]string{false: "switch on", true: "switch off"}[off], func(t *testing.T) {
			d, l, h := r7wDeck(t, true)
			d.mu.Lock()
			d.config.SandboxOff = off
			d.mu.Unlock()
			if status, said := d.hireWith(hireSpec{fromSetup: true, extra: &setupHire{Folder: l.lib}}); status != http.StatusNoContent {
				t.Fatalf("Setup's plain hire into lib: %d %s", status, said)
			}
			folder, desk := r7wPlainHire(t, h)
			_, made := os.Stat(hiresFolder(l.lib, ""))
			branches := strings.TrimSpace(standInOwnersGit(t, l.lib, "branch", "--list", "hire/*"))
			if !off {
				if !plainDeskID(desk) || folder == l.lib || made != nil || !strings.Contains(branches, "hire/"+desk+"/") {
					t.Errorf("its own worktree: in %q for desk %q, branches %q, folder made %v", folder, desk, branches, made == nil)
				}
				return
			}
			if folder != l.lib || desk != "" || made == nil || branches != "" {
				t.Errorf("as before: in %q for desk %q, branches %q, folder made %v", folder, desk, branches, made == nil)
			}
		})
	}
}

// hire.go's plain path: its launcher is told the sandbox is off by the owner's switch alone, whether this system's
// launcher sandboxes or a test's stand-in doesn't.
func TestOnAMacAPlainHireIsToldTheSandboxIsOffByTheSwitchAlone(t *testing.T) {
	for _, sandboxes := range []bool{false, true} {
		for _, off := range []bool{false, true} {
			t.Run(map[bool]string{false: "stand-in launcher", true: "launcher sandboxes"}[sandboxes]+", "+map[bool]string{false: "switch on", true: "switch off"}[off], func(t *testing.T) {
				d, l, h := r7wDeck(t, sandboxes)
				d.mu.Lock()
				d.config.SandboxOff = off
				d.mu.Unlock()
				if status, said := d.hireWith(hireSpec{fromSetup: true, extra: &setupHire{Folder: l.lib}}); status != http.StatusNoContent {
					t.Fatalf("Setup's plain hire into lib: %d %s", status, said)
				}
				if told := slices.Contains(h.tmux[0], "--sandbox-off"); told != off {
					t.Errorf("told off %v", told)
				}
			})
		}
	}
}

// The launcher: its sandbox asks the same question once the core said off (the file's switch, and this system's
// launcher), and a plain hire made with the switch off, with no desk of its own, doesn't start in its sandbox in its
// folder when the switch is on again: it refuses. The GM's place is its own; a hire with its desk, or made with the
// switch on, starts.
func TestALaunchersSandboxAsksThePlainHireQuestion(t *testing.T) {
	isolate(t)
	saved := plainHiresSandboxed
	t.Cleanup(func() { plainHiresSandboxed = saved })
	token := strings.Repeat("AB", 16)
	for _, sandboxes := range []bool{false, true} {
		plainHiresSandboxed = sandboxes
		for _, fileOff := range []bool{false, true} {
			writeFile(t, configFile, map[bool]string{true: `{"sandboxOff": true}`, false: `{}`}[fileOff])
			run := &launcherRun{hire: token, command: []string{"claude"}, sandboxOff: true}
			if got, want := run.sandboxed(), plainHireSandboxed(*savedConfig()); got != want {
				t.Errorf("said off, a launcher that sandboxes %v, the file off %v: sandboxed %v, want %v", sandboxes, fileOff, got, want)
			}
		}
	}
	repo, none := t.TempDir(), t.TempDir()
	standInOwnersGit(t, repo, "init", "-q")
	for _, c := range []struct {
		row       string
		run       launcherRun
		sandboxed bool
		folder    string
		refused   bool
	}{
		{"made off, on again, no desk, in a repo", launcherRun{sandboxOff: true}, true, repo, true},
		{"made off, on again, a role's, in a repo", launcherRun{sandboxOff: true, role: "tester"}, true, repo, true},
		{"made off, on again, no desk, in no repo", launcherRun{sandboxOff: true}, true, none, false},
		{"made off, on again, a folder git can't look in", launcherRun{sandboxOff: true}, true, filepath.Join(none, "gone"), true},
		{"made off, still off", launcherRun{sandboxOff: true}, false, repo, false},
		{"made off, the GM", launcherRun{sandboxOff: true, role: gmRole}, true, repo, false},
		{"made off, its desk", launcherRun{sandboxOff: true, desk: "plain-0a1b2c3d4e5f"}, true, repo, false},
		{"made on, its desk", launcherRun{desk: "plain-0a1b2c3d4e5f"}, true, repo, false},
		{"made on, no desk", launcherRun{}, true, repo, false},
	} {
		if why := c.run.sandboxOnAgainWords(c.sandboxed, c.folder); (why != "") != c.refused {
			t.Errorf("%s: %q, refused want %v", c.row, why, c.refused)
		}
	}
}

// RT126STACKP3 O1, RT126STACKFIX5 N1 (FXSTACKFIX5R1): a launcher runs with its terminal's environment, and a ceiling
// there (GIT_CEILING_DIRECTORIES at the repo) makes git answer "not a git repository" for a folder inside it. Only the
// launcher's refusal looks past it (gitFindsNoRepo): in the repo's sub, a plain hire made with the switch off is still
// refused when the switch is on again. Every other read keeps the owner's ceiling: under it the launcher finds no repo
// there, and a start there reads no git folder, so the sandbox's rows hold the hidden .git whole
// (TestAGMUnderAGitCeilingAtHomeGetsNoWayIntoTheHiddenGitFolder).
func TestOnlyTheLaunchersRefusalLooksPastATerminalsGitCeiling(t *testing.T) {
	isolate(t)
	repo := t.TempDir()
	standInOwnersGit(t, repo, "init", "-q")
	sub := filepath.Join(repo, "sub")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	run := launcherRun{hire: strings.Repeat("AB", 16), sandboxOff: true}
	for _, ceiling := range []bool{false, true} {
		hires, gitDir := physical(repo)+"-hires", physical(filepath.Join(repo, ".git"))
		if ceiling {
			t.Setenv("GIT_CEILING_DIRECTORIES", repo)
			hires, gitDir = "", ""
			// The ceiling bites: the terminal's own git sees no repo here.
			said, err := exec.Command("git", "-C", sub, "rev-parse", "--show-toplevel").CombinedOutput()
			if err == nil || !strings.Contains(string(said), "not a git repository") {
				t.Fatalf("the terminal's git under the ceiling: %v %s", err, said)
			}
		}
		if why := run.sandboxOnAgainWords(true, sub); why != plainHireSandboxOnAgain {
			t.Errorf("a ceiling %v: the launcher's refusal in the repo's sub: %q", ceiling, why)
		}
		if checkout, got := launcherRepo(sub, ""); checkout != "" || got != hires {
			t.Errorf("a ceiling %v: the launcher's repo: %q %q, want \"\" %q", ceiling, checkout, got, hires)
		}
		pl := sandbox.Places{Root: physical(sub)}
		if err := deskGitPlaces(deskAt{ID: "x", Root: sub, Git: runGit}, &pl); err != nil || pl.GitDir != gitDir {
			t.Errorf("a ceiling %v: the start's git folder %q (%v), want %q", ceiling, pl.GitDir, err, gitDir)
		}
	}
}

// RT126STACKFIX5 O1, O2 (FXSTACKFIX5R1): the launcher's refusal counts only git's own "not a git repository" answer
// as no repo; any other failure is a repo, and refused. In a repo git refuses as another account's
// (GIT_TEST_ASSUME_DIFFERENT_OWNER, which reads keep), and in a folder whose .git file names no git folder, it still
// refuses; in a folder in no repo it doesn't, the terminal in another language, or tracing git, too.
func TestTheLaunchersRefusalTakesOnlyGitsNoRepoAnswerForNoRepo(t *testing.T) {
	isolate(t)
	repo, none, broken := t.TempDir(), t.TempDir(), t.TempDir()
	standInOwnersGit(t, repo, "init", "-q")
	sub := filepath.Join(repo, "sub")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, ".git"), []byte("gitdir: "+filepath.Join(none, "gone")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := launcherRun{hire: strings.Repeat("AB", 16), sandboxOff: true}
	for _, c := range []struct {
		row     string
		env     [][2]string
		folder  string
		refused bool
	}{
		{"a repo git takes for another account's", [][2]string{{"GIT_TEST_ASSUME_DIFFERENT_OWNER", "1"}}, repo, true},
		{"a .git file that names no git folder", nil, broken, true},
		{"a repo under the terminal's ceiling", [][2]string{{"GIT_CEILING_DIRECTORIES", repo}}, sub, true},
		{"in no repo, the terminal in French", [][2]string{{"LC_ALL", "fr_FR.UTF-8"}, {"LANGUAGE", "fr"}}, none, false},
		// RT126STACKFIX5R1 O3: git's trace lines come before its no-repo words; the refusal's read gives git no trace.
		{"in no repo, the terminal tracing git", [][2]string{{"GIT_TRACE", "1"}, {"GIT_TRACE_SETUP", "2"}, {"GIT_TRACE2", "1"}}, none, false},
		{"in no repo", nil, none, false},
	} {
		t.Run(c.row, func(t *testing.T) {
			for _, kv := range c.env {
				t.Setenv(kv[0], kv[1])
			}
			// Git itself fails here, or finds no repo (the stand-in home's git, with the case's environment): none of
			// these is a read that just works.
			look := disclaim.Cmd(exec.Command("git", "-C", c.folder, "rev-parse", "--show-toplevel"))
			look.Env = gmRootEnv()
			for _, kv := range c.env {
				look.Env = append(look.Env, kv[0]+"="+kv[1])
			}
			if said, err := look.CombinedOutput(); err == nil {
				t.Fatalf("git: %s, want it to fail", said)
			}
			if why := run.sandboxOnAgainWords(true, c.folder); (why == plainHireSandboxOnAgain) != c.refused || (why != "" && why != plainHireSandboxOnAgain) {
				t.Errorf("%q, refused want %v", why, c.refused)
			}
		})
	}
}

// RT126STACKFIX5 N1 (FXSTACKFIX5R1), the review's probe: the owner keeps the home folder's dotfiles in a repo at home
// and hides it from git with GIT_CEILING_DIRECTORIES=$HOME, and a GM starts at ~/gm. Its start reads no git folder
// there (deskGitPlaces keeps the ceiling), so the sandbox holds that .git whole: no object, no ORIG_HEAD a later
// `git reset --hard ORIG_HEAD` in the home folder would take.
func TestAGMUnderAGitCeilingAtHomeGetsNoWayIntoTheHiddenGitFolder(t *testing.T) {
	isolate(t)
	home := physical(homeDir)
	standInOwnersGit(t, home, "init", "-q")
	gm := filepath.Join(home, "gm")
	if err := os.MkdirAll(gm, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", home)
	pl := sandbox.Places{GOOS: "darwin", Subject: sandbox.GM, Home: home, Support: physical(supportDir), OwnID: "gm1", Root: gm}
	if err := deskGitPlaces(deskAt{ID: "gm1", GM: true, Root: gm, Git: runGit}, &pl); err != nil || pl.GitDir != "" {
		t.Fatalf("the GM's git folder under the ceiling: %q (%v), want none", pl.GitDir, err)
	}
	p, err := sandbox.New(pl)
	if err != nil {
		t.Fatal(err)
	}
	git := filepath.Join(home, ".git")
	for _, at := range []string{filepath.Join(git, "objects", "ab", "cd"), filepath.Join(git, "ORIG_HEAD"),
		filepath.Join(git, "refs", "heads", "main"), filepath.Join(git, "config")} {
		if !deniedWrite(p, at) {
			v, r := p.Decide(at, sandbox.Write)
			t.Errorf("the GM may write %s: %v by %q", at, v, r.Row)
		}
	}
}
