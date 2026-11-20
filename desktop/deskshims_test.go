//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SA-T2: the app's dev-tool shims (Resources/shims) go first on a Mac desk's PATH, before the app's own folder, and
// only when they are the app's own: a folder in the .app the core runs from, which the Policy denies to every desk.
// The folders here are only made, never run: nothing is exec'd inside a folder named .app (_rules.md).

// standInApp makes <temp>/name/Contents/Resources/bin, and shims beside it when withShims; it returns bin.
func standInApp(t *testing.T, name string, withShims bool) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(base, name, "Contents", "Resources", "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if withShims {
		if err := os.Mkdir(filepath.Join(filepath.Dir(bin), "shims"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return bin
}

func TestOnAMacTheAppsShimsAreFirstOnADesksPath(t *testing.T) {
	bin := standInApp(t, "Smolcorp.app", true)
	shims := filepath.Join(filepath.Dir(bin), "shims")
	env, err := deskEnvironment([]string{"PATH=/usr/bin:/bin", "HOME=/h"}, deskPlan{PathFirst: bin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "PATH=" + bin + ":/usr/bin:/bin"
	if deskSandboxed {
		want = "PATH=" + shims + ":" + bin + ":/usr/bin:/bin"
	}
	if !strings.Contains(strings.Join(env, "\n")+"\n", want+"\n") {
		t.Errorf("the desk's PATH: %q, want %q", env, want)
	}
	// No PathFirst, no shims: the plan names no app.
	env, _ = deskEnvironment([]string{"PATH=/usr/bin"}, deskPlan{}, nil)
	if env[0] != "PATH=/usr/bin" {
		t.Errorf("with no app folder: %q", env)
	}
}

func TestOnlyTheAppsOwnShimsFolderIsTaken(t *testing.T) {
	bin := standInApp(t, "Smolcorp.app", true)
	if got, want := shimsBeside(bin), filepath.Join(filepath.Dir(bin), "shims"); got != want {
		t.Errorf("the app's shims: %q, want %q", got, want)
	}
	if got := shimsBeside(standInApp(t, "Smolcorp.APP", true)); got == "" {
		t.Error("an app whose name says .APP: no shims")
	}

	// A smolcorp that isn't in an app, or not in its Contents/Resources/bin: its desks have their brief's lines alone.
	for _, name := range []string{"Smolcorp", "Smolcorp.app.d", "app"} {
		if got := shimsBeside(standInApp(t, name, true)); got != "" {
			t.Errorf("%s isn't an app, yet its shims were taken: %q", name, got)
		}
	}
	other := standInApp(t, "Smolcorp.app", true)
	for _, dir := range []string{filepath.Dir(other), filepath.Join(filepath.Dir(other), "MacOS"), "relative/Smolcorp.app/Contents/Resources/bin", ""} {
		if got := shimsBeside(dir); got != "" {
			t.Errorf("%q: %q", dir, got)
		}
	}

	// An app with no shims (one built before them), a file in their place, or a link: none taken.
	if got := shimsBeside(standInApp(t, "Old.app", false)); got != "" {
		t.Errorf("an app with no shims: %q", got)
	}
	asFile := standInApp(t, "File.app", false)
	if err := os.WriteFile(filepath.Join(filepath.Dir(asFile), "shims"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := shimsBeside(asFile); got != "" {
		t.Errorf("a file named shims: %q", got)
	}
	asLink := standInApp(t, "Link.app", false)
	if err := os.Symlink(t.TempDir(), filepath.Join(filepath.Dir(asLink), "shims")); err != nil {
		t.Fatal(err)
	}
	if got := shimsBeside(asLink); got != "" {
		t.Errorf("a link named shims, to a folder outside the app: %q", got)
	}
}
