//go:build unix

package guard

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// L1GUARD1: a Linux desk's guard, given its start's copy (--held), compares the places no mount holds before AND
// after every tool call (PreToolUse and PostToolUse: a long Bash call can write after the check before it), and on a
// change stops the desk ("continue": false) and says what, before and after, and when. It never touches what it found
// (SIERRA, 15 Nov: only the core puts back): it notes what stopped the desk, and the next start puts that back.
// Stand-ins only: a temp desk, a made-up call.

// keptGuardDesk is a temp desk with its start's copy kept, and the folder of that copy.
func keptGuardDesk(t *testing.T) (pl sandbox.Places, held string) {
	t.Helper()
	base, _ := filepath.EvalSymlinks(t.TempDir())
	pl = sandbox.Places{GOOS: "linux", Root: base + "/app-hires/h1", GitDir: base + "/app/.git", OwnWorktree: "h1", OwnID: "h1", Support: base + "/support"}
	reg := pl.GitDir + "/worktrees/h1"
	for at, text := range map[string]string{
		pl.Root + "/.claude/settings.json": "{}\n", pl.Root + "/.git": "gitdir: " + reg + "\n", reg + "/commondir": "../..\n",
		reg + "/gitdir": pl.Root + "/.git\n", reg + "/config.worktree": "", pl.GitDir + "/objects/info/packs": "",
		pl.GitDir + "/refs/heads/hire/h1/work": "0123\n", pl.GitDir + "/logs/refs/heads/hire/h1/work": "",
		reg + "/branches/.keep": "", reg + "/remotes/.keep": "",
	} {
		os.MkdirAll(filepath.Dir(at), 0o700)
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(pl.Support+"/hires/h1", 0o700)
	if err := sandbox.SaveKept(pl); err != nil {
		t.Fatal(err)
	}
	return pl, sandbox.KeptFolder(pl.Support, "h1")
}

// keptAnswer is the guard's answer to a call on event, with --held: whether it stops the desk, and how it says it.
type keptAnswer struct {
	Continue   *bool  `json:"continue"`
	StopReason string `json:"stopReason"`
	Decision   string `json:"decision"`
	Reason     string `json:"reason"`
	Hook       struct {
		Event    string `json:"hookEventName"`
		Decision string `json:"permissionDecision"`
	} `json:"hookSpecificOutput"`
}

func guardWithHeld(t *testing.T, held, event, command string) (keptAnswer, string) {
	t.Helper()
	call, _ := json.Marshal(map[string]any{"hook_event_name": event, "tool_name": "Bash", "tool_input": map[string]any{"command": command}, "cwd": "/"})
	rules := filepath.Join(t.TempDir(), "hire-guard.json")
	os.WriteFile(rules, DefaultFile(), 0o600)
	var out, errs bytes.Buffer
	args := []string{"--rules", rules, "--held", held}
	if event != "PreToolUse" { // the settings' PostToolUse and PostToolUseFailure hooks, which name the desk's sealed
		// settings file too (SETTINGSFILE1): `guard --after --settings FILE --sum HEX --held DIR`
		settings, sum := sealedForTests(t, t.TempDir())
		args = []string{"--after", "--settings", settings, "--sum", sum, "--held", held}
	}
	if code := Run(args, bytes.NewReader(call), &out, &errs); code != exitOK {
		t.Fatalf("%s: exit %d, %s", event, code, errs.String())
	}
	var a keptAnswer
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &a); err != nil {
			t.Fatalf("%s: not JSON: %q", event, out.String())
		}
	}
	return a, out.String()
}

// Both sides: a change made before the call, or during it, stops the desk; the version found stays as it is, noted
// for the next start, which puts the saved content back.
func TestTheGuardStopsADeskWhoseKeptPlaceChangedOnBothSides(t *testing.T) {
	for _, event := range []string{"PreToolUse", "PostToolUse", "PostToolUseFailure"} {
		t.Run(event, func(t *testing.T) {
			pl, held := keptGuardDesk(t)
			if a, out := guardWithHeld(t, held, event, "ls"); out != "" {
				t.Fatalf("nothing changed, yet: %+v", a)
			}
			at := pl.GitDir + "/worktrees/h1/config.worktree"
			desks := "[core]\n\tfsmonitor = ./stand-in\n"
			os.WriteFile(at+".new", []byte(desks), 0o600)
			os.Rename(at+".new", at)
			a, out := guardWithHeld(t, held, event, "ls")
			if a.Continue == nil || *a.Continue || !strings.Contains(a.StopReason, at) || !strings.Contains(a.StopReason, "fsmonitor") ||
				!strings.HasPrefix(a.StopReason, "Smolcorp stopped this desk.") || !strings.Contains(a.StopReason, "Nothing is undone here") {
				t.Fatalf("the desk isn't stopped with where and what: %s", out)
			}
			switch event {
			case "PreToolUse":
				if a.Hook.Event != "PreToolUse" || a.Hook.Decision != "deny" {
					t.Fatalf("the call isn't refused: %s", out)
				}
			default:
				if a.Decision != "block" || a.Reason != a.StopReason || a.Hook.Event != "" {
					t.Fatalf("not PostToolUse's block: %s", out)
				}
			}
			if data, _ := os.ReadFile(at); string(data) != desks {
				t.Fatalf("the guard touched what it found: %q", data)
			}
			// Asked again, it stops again, and notes it once.
			guardWithHeld(t, held, event, "ls")
			if notes, _ := os.ReadFile(filepath.Join(filepath.Dir(held), "kept-stop.json")); strings.Count(string(notes), at) != 1 {
				t.Fatalf("the notes: %q", notes)
			}
			if err := sandbox.SaveKept(pl); !errors.Is(err, sandbox.ErrKeptChanged) || !strings.Contains(err.Error(), "set aside whole") {
				t.Fatalf("the next start: %v", err)
			}
			if data, _ := os.ReadFile(at); len(data) != 0 {
				t.Fatalf("the next start didn't put back the saved content: %q", data)
			}
		})
	}
}

// The repo's shared objects/info, changed by the owner's gc: the desk stops and is shown, and nothing is noted to put
// back, ever.
func TestTheGuardStopsADeskWhoseRepoInfoChangedAndNotesNothing(t *testing.T) {
	pl, held := keptGuardDesk(t)
	os.WriteFile(pl.GitDir+"/objects/info/commit-graph", []byte("stand-in"), 0o600)
	a, out := guardWithHeld(t, held, "PostToolUse", "ls")
	if a.Continue == nil || *a.Continue || !strings.Contains(a.StopReason, "never put back") {
		t.Fatalf("%s", out)
	}
	if notes, _ := os.ReadFile(filepath.Join(filepath.Dir(held), "kept-stop.json")); len(notes) != 0 {
		t.Fatalf("noted: %q", notes)
	}
	if err := sandbox.SaveKept(pl); err != nil {
		t.Fatalf("the next start: %v", err)
	}
	if data, _ := os.ReadFile(pl.GitDir + "/objects/info/commit-graph"); string(data) != "stand-in" {
		t.Fatalf("the gc's commit-graph is %q", data)
	}
}

// The branch folder the owner's git made again stops the desk with words to restart it, on either side.
func TestTheGuardStopsADeskWhoseBranchFolderWasReplaced(t *testing.T) {
	pl, held := keptGuardDesk(t)
	branch := pl.GitDir + "/refs/heads/hire/h1"
	os.Mkdir(branch+".new", 0o700)
	os.RemoveAll(branch)
	os.Rename(branch+".new", branch)
	for _, event := range []string{"PreToolUse", "PostToolUse"} {
		if a, out := guardWithHeld(t, held, event, "git commit -m x"); a.Continue == nil || *a.Continue ||
			!strings.Contains(a.StopReason, "restart this hire to keep committing") {
			t.Fatalf("%s: %s", event, out)
		}
	}
}

// Fails closed: a copy that isn't there stops the desk; after a call, nothing but the copy is checked, so a command
// already run isn't refused afterwards, and before it the command is still checked as ever.
func TestTheGuardWithACopyFailsClosedAndChecksCommandsBeforeOnly(t *testing.T) {
	_, held := keptGuardDesk(t)
	if a, out := guardWithHeld(t, held, "PostToolUse", "git push"); out != "" {
		t.Fatalf("after the call, its command is checked again: %+v", a)
	}
	if a, out := guardWithHeld(t, held, "PreToolUse", "git push"); a.Hook.Decision != "deny" || a.Continue != nil {
		t.Fatalf("before the call, a push isn't refused as ever: %s", out)
	}
	for _, event := range []string{"PreToolUse", "PostToolUse"} {
		if a, out := guardWithHeld(t, filepath.Join(t.TempDir(), "held"), event, "ls"); a.Continue == nil || *a.Continue {
			t.Fatalf("%s: no copy, and the desk goes on: %s", event, out)
		}
	}
	var out bytes.Buffer
	if Run([]string{"--held"}, strings.NewReader("{}"), &out, &out); !strings.Contains(out.String(), "--held needs") {
		t.Fatalf("--held with no folder: %q", out.String())
	}
	out.Reset()
	if Run([]string{"--after"}, strings.NewReader("{}"), &out, &out); !strings.Contains(out.String(), `"continue":false`) {
		t.Fatalf("--after with nothing to check goes on: %q", out.String())
	}
}

// MSYLCHAIN: the settings file's seal (SETTINGSFILE1) and the kept places (L1GUARD1) guard the same step, each on its
// own. With both named, a settings file changed stops the desk though no kept place changed, and a kept place changed
// stops it though the settings file is as sealed; after a call, --held without the settings file stops it too.
func TestTheSettingsSealAndTheKeptPlacesEachStopTheDeskOnTheirOwn(t *testing.T) {
	call := func(args []string) keptAnswer {
		t.Helper()
		var out bytes.Buffer
		if code := Run(args, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), &out, &out); code != exitOK {
			t.Fatalf("%q: exit %d", args, code)
		}
		var a keptAnswer
		if out.Len() > 0 {
			if err := json.Unmarshal(out.Bytes(), &a); err != nil {
				t.Fatalf("%q: not JSON: %q", args, out.String())
			}
		}
		return a
	}
	rules := filepath.Join(t.TempDir(), "hire-guard.json")
	os.WriteFile(rules, DefaultFile(), 0o600)
	for _, after := range []bool{false, true} {
		words := func(settings, sum, held string) []string {
			if after {
				return []string{"--after", "--settings", settings, "--sum", sum, "--held", held}
			}
			return []string{"--rules", rules, "--settings", settings, "--sum", sum, "--held", held}
		}
		pl, held := keptGuardDesk(t)
		settings, sum := sealedForTests(t, t.TempDir())
		if a := call(words(settings, sum, held)); a.Continue != nil {
			t.Fatalf("after %v: nothing changed, yet: %+v", after, a)
		}
		// The settings file changed, the kept places as kept.
		must(t, os.Chmod(settings, 0o600))
		must(t, appendTo(settings, " "))
		if a := call(words(settings, sum, held)); a.Continue == nil || *a.Continue ||
			!strings.HasPrefix(a.StopReason, "Stopped by Smolcorp's hire guard: this desk's settings file isn't the one") {
			t.Fatalf("after %v: a changed settings file, and the desk goes on: %+v", after, a)
		}
		// The settings file as sealed, a kept place changed.
		settings, sum = sealedForTests(t, t.TempDir())
		os.WriteFile(pl.GitDir+"/worktrees/h1/config.worktree", []byte("[core]\n\tfsmonitor = ./stand-in\n"), 0o600)
		if a := call(words(settings, sum, held)); a.Continue == nil || *a.Continue || !strings.HasPrefix(a.StopReason, "Smolcorp stopped this desk.") {
			t.Fatalf("after %v: a changed kept place, and the desk goes on: %+v", after, a)
		}
	}
	_, held := keptGuardDesk(t)
	if a := call([]string{"--after", "--held", held}); a.Continue == nil || *a.Continue ||
		!strings.HasPrefix(a.StopReason, "Stopped by Smolcorp's hire guard: this desk's settings file isn't the one") {
		t.Fatalf("after a call, --held without the settings file goes on: %+v", a)
	}
}

// RMSYLCHAIN B2: one call after which the settings file fails its seal AND a kept place changed. The desk stops with
// both reasons, and the kept change is still noted, so the core's put back at the fire (RestoreKept) puts it back, as
// it does when the settings file is as sealed. Un-noted, the change would be Kept and taken into the next start's copy.
func TestASettingsStopStillNotesAKeptChangeForTheFireToPutBack(t *testing.T) {
	rules := filepath.Join(t.TempDir(), "hire-guard.json")
	os.WriteFile(rules, DefaultFile(), 0o600)
	for _, after := range []bool{false, true} {
		for _, tampered := range []bool{false, true} {
			pl, held := keptGuardDesk(t)
			settings, sum := sealedForTests(t, t.TempDir())
			if tampered {
				must(t, os.Chmod(settings, 0o600))
				must(t, appendTo(settings, " "))
			}
			at := pl.GitDir + "/worktrees/h1/config.worktree"
			os.WriteFile(at, []byte("[core]\n\tfsmonitor = ./stand-in\n"), 0o600)
			args := []string{"--rules", rules, "--settings", settings, "--sum", sum, "--held", held}
			if after {
				args = []string{"--after", "--settings", settings, "--sum", sum, "--held", held}
			}
			var out bytes.Buffer
			if code := Run(args, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), &out, &out); code != exitOK {
				t.Fatalf("after %v, tampered %v: exit %d", after, tampered, code)
			}
			var a keptAnswer
			if err := json.Unmarshal(out.Bytes(), &a); err != nil {
				t.Fatalf("after %v, tampered %v: not JSON: %q", after, tampered, out.String())
			}
			settingsSays := strings.Contains(a.StopReason, "Stopped by Smolcorp's hire guard: this desk's settings file isn't the one")
			if a.Continue == nil || *a.Continue || !strings.Contains(a.StopReason, "Smolcorp stopped this desk.") ||
				!strings.Contains(a.StopReason, at) || settingsSays != tampered {
				t.Fatalf("after %v, tampered %v: not stopped with every reason: %s", after, tampered, out.String())
			}
			if notes, _ := os.ReadFile(filepath.Join(filepath.Dir(held), "kept-stop.json")); strings.Count(string(notes), at) != 1 {
				t.Fatalf("after %v, tampered %v: the kept change isn't noted: %q", after, tampered, notes)
			}
			// The fire's look (deskKept → RestoreKept) puts it back.
			changes, err := sandbox.RestoreKept(held)
			back := false
			for _, ch := range changes {
				back = back || ch.Back
			}
			if data, _ := os.ReadFile(at); err != nil || !back || len(data) != 0 {
				t.Fatalf("after %v, tampered %v: the fire didn't put the change back: err %v, back %v, %q", after, tampered, err, back, data)
			}
		}
	}
}
