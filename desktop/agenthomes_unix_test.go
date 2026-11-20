//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// FXPLANTSTERMR3: the core reads where the owner's environment moves an agent's folders, at each desk's start
// (deskPlaces), from its own environment: each place resolved, and on a Mac as spelled too, so a link there is held
// itself as well as where it leads. A value that isn't a whole path is passed over.
func TestADesksPlacesHoldTheAgentHomesTheOwnersEnvironmentNames(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(physical(base), "dotfiles", "codex")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(base, "codex-link")
	if err := os.Symlink(real, linked); err != nil {
		t.Skipf("no links here: %v", err)
	}
	env := map[string]string{"CODEX_HOME": linked, "GEMINI_CLI_HOME": filepath.Join(base, "g"), "XDG_DATA_HOME": filepath.Join(base, "data"),
		"GROK_AUTH_PATH": filepath.Join(base, "grok-auth.json"), "GROK_HOME": "relative/grok", "OPENCODE_CONFIG_DIR": ""}
	under := physical(base) // the temporary folder, resolved (/private/var on a Mac)
	resolved := []string{real, filepath.Join(under, "g", ".gemini"), filepath.Join(under, "g", ".agents"), filepath.Join(under, "data", "opencode")}
	spelled := []string{linked, filepath.Join(base, "g", ".gemini"), filepath.Join(base, "g", ".agents"), filepath.Join(base, "data", "opencode")}
	for _, goos := range []string{"darwin", "linux"} {
		homes, signIns := deskAgentHomes(goos, func(v string) string { return env[v] })
		want := slices.Clone(resolved)
		if goos == "darwin" {
			for _, s := range spelled {
				if !slices.Contains(want, s) {
					want = append(want, s)
				}
			}
		}
		if !slicesEqualAsSets(homes, want) {
			t.Errorf("%s: the agents' places %q, want %q", goos, homes, want)
		}
		wantSignIns := []string{filepath.Join(real, "auth.json"), filepath.Join(real, "secrets"),
			filepath.Join(under, "g", ".gemini", "oauth_creds.json"), filepath.Join(under, "data", "opencode", "auth.json"),
			filepath.Join(under, "data", "opencode", "opencode.db"), filepath.Join(under, "grok-auth.json")}
		if goos == "darwin" {
			wantSignIns = append(wantSignIns, filepath.Join(linked, "auth.json"))
		}
		for _, file := range wantSignIns {
			if !slices.Contains(signIns, file) {
				t.Errorf("%s: %s isn't among the sign-ins %q", goos, file, signIns)
			}
		}
		if strings.Contains(strings.Join(append(homes, signIns...), "\x00"), "relative") {
			t.Errorf("%s: a value that isn't a whole path was taken: %q %q", goos, homes, signIns)
		}
	}
}

// slicesEqualAsSets is whether a and b hold the same strings, in any order.
func slicesEqualAsSets(a, b []string) bool {
	count := map[string]int{}
	for _, s := range a {
		count[s]++
	}
	for _, s := range b {
		count[s]--
	}
	for _, n := range count {
		if n != 0 {
			return false
		}
	}
	return true
}
