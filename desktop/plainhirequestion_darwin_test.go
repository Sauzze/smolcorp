package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
