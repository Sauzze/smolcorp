//go:build !windows

package desktop

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/guard"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// The release gate's item 4b (MSYLCHAIN's Decision 1, SA-L2): a Linux desk started in its view keeps a copy of the
// places no mount holds (sandbox.SaveKept), and its guard is told of it before each call (`guard … --held <copy>`)
// and after each one, failed ones included (`guard --after --settings <file> --sum <seal> --held <copy>`). F4: the
// view knows the desk by the slug its branch is named by, the GM's too.

// guardWordsIn are the guard's words in the settings file at path: before a call, and after one, by event.
func guardWordsIn(t *testing.T, path string) (before []string, after map[string][]string) {
	t.Helper()
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct{ Args []string }
		}
	}
	if err := json.Unmarshal([]byte(fileSettings(path)), &settings); err != nil {
		t.Fatal(err)
	}
	after = map[string][]string{}
	for event, groups := range settings.Hooks {
		for _, group := range groups {
			for _, hook := range group.Hooks {
				switch {
				case event == "PreToolUse" && len(hook.Args) > 0 && hook.Args[0] == "guard":
					before = hook.Args
				case len(hook.Args) > 1 && hook.Args[0] == "guard" && hook.Args[1] == "--after":
					after[event] = hook.Args
				}
			}
		}
	}
	return before, after
}

// A Linux desk in its view: the copy named on both sides of each call, after the seal; nothing else of the words
// changes. A desk that keeps none (a Mac's, one not in its view) names none. Settings asked to name a copy with no
// file to name after a call aren't made: that desk would have no after-call check.
func TestALinuxDesksGuardIsToldItsKeptCopyBeforeAndAfterEachCall(t *testing.T) {
	r := newHireSettingsRig(t)
	kept := deskKeptFolder("linux", true, "abc123")
	if kept != sandbox.KeptFolder(physical(supportDir), "abc123") {
		t.Fatalf("the kept copy %q", kept)
	}
	written, err := r.writeFile("abc123", func(o *hireSettingsOptions) { o.Kept = kept })
	if err != nil {
		t.Fatal(err)
	}
	seal := []string{"--settings", written.Path, "--sum", written.Sum, "--held", kept}
	before, after := guardWordsIn(t, written.Path)
	if len(before) < len(seal) || !slices.Equal(before[len(before)-len(seal):], seal) || slices.Index(before, "--held") != len(before)-2 {
		t.Errorf("the guard's words before a call: %q", before)
	}
	for _, event := range []string{"PostToolUse", "PostToolUseFailure"} {
		if !slices.Equal(after[event], append([]string{"guard", "--after"}, seal...)) {
			t.Errorf("the guard's words after a call (%s): %q", event, after[event])
		}
	}
	// The guard reads these words as they are: --held is checked after the call, and a copy that isn't there stops
	// the desk (fail closed), never "a word it doesn't know".
	var out, stderr bytes.Buffer
	guard.Run(after["PostToolUse"][1:], strings.NewReader("{}"), &out, &stderr)
	if said := out.String(); !strings.Contains(said, "places it may not change couldn't all be checked") || strings.Contains(said, "unknown word") {
		t.Errorf("the guard after a call, with no copy there: %s", said)
	}

	for _, none := range []string{deskKeptFolder("darwin", true, "abc123"), deskKeptFolder("linux", false, "abc123"), deskKeptFolder("windows", true, "abc123")} {
		if none != "" {
			t.Fatalf("a desk that keeps no copy is told of %q", none)
		}
	}
	plain, err := r.writeFile("abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fileSettings(plain.Path), "--held") {
		t.Error("a desk that keeps no copy is told of one")
	}
	if _, err := hireSettingsObject(r.options("abc123", func(o *hireSettingsOptions) { o.Kept = kept })); err == nil {
		t.Error("settings naming a copy, with no file to name after a call, were made")
	}
}

// F4: on Linux the view's own id is the slug its branch is named by (hireBranch), the GM's as well as a hire's: its
// branch folder, refs/heads/hire/<id>, is then the one its view lets it write. An id that isn't its own slug has no
// view there, rather than one whose folders don't match. A Mac keeps the id as it is.
func TestALinuxViewKnowsItsDeskByTheSlugItsBranchIsNamedBy(t *testing.T) {
	for _, id := range []string{"0f3a9c2b7e41", "gm", "a1-b2"} {
		own, err := deskOwnID("linux", id)
		if err != nil || !strings.HasPrefix(hireBranch(id, "janitor-0f3a9c"), sandbox.HireBranch(own, "")) {
			t.Errorf("%q: the view's id %q (%v), its branch %q", id, own, err, hireBranch(id, "janitor-0f3a9c"))
		}
	}
	for _, id := range []string{"AB12", "a_b", "x/y"} {
		if own, err := deskOwnID("linux", id); err == nil {
			t.Errorf("%q has a view on Linux as %q", id, own)
		}
		if own, err := deskOwnID("darwin", id); err != nil || own != id {
			t.Errorf("a Mac's %q: %q, %v", id, own, err)
		}
	}
	if deskKeptFolder("linux", true, "0f3a9c2b7e41") != filepath.Join(physical(supportDir), "hires", "0f3a9c2b7e41", "held") {
		t.Error("the kept copy isn't in the desk's own folder, by its view's id")
	}
}

// keptForThisSystem is what the engine's desks are told of their kept copy here: on Linux, their own; elsewhere none.
func keptForThisSystem(id string) string { return deskKeptFolder(runtime.GOOS, true, id) }
