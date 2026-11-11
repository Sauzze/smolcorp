package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

// standInTmux makes a folder with an executable named tmux in it (or one that can't run).
func standInTmux(t *testing.T, mode os.FileMode) (folder, path string) {
	t.Helper()
	folder = t.TempDir()
	path = filepath.Join(folder, "tmux")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), mode); err != nil {
		t.Fatal(err)
	}
	return folder, path
}

// A program opened from the Finder or the Dock has launchd's PATH, with no Homebrew on it: tmux is still
// found where the Mac's engine looks (N1 of C3r). With stand-in folders and a stand-in PATH.
func TestTmuxIsFoundBesideThePATHWhereTheMacLooks(t *testing.T) {
	if !hasTmuxHires {
		t.Skip("no tmux hires on this system")
	}
	nothing := t.TempDir() // launchd's PATH: no tmux on it
	t.Setenv("PATH", nothing)
	savedPlaces, savedLogin := tmuxPlaces, tmuxLoginPath
	t.Cleanup(func() { tmuxPlaces, tmuxLoginPath = savedPlaces, savedLogin })
	asked := 0
	tmuxLoginPath = func() []string { asked++; return nil }
	tmuxPlaces = []string{filepath.Join(nothing, "missing"), nothing}

	if got := findTmux(); got != "" {
		t.Errorf("tmux nowhere: %q, want none", got)
	}
	if asked != 1 {
		t.Errorf("the login shell was asked %d times with nothing found, want once", asked)
	}

	// In a usual place (Homebrew's), not on the PATH.
	homebrew, brewTmux := standInTmux(t, 0o755)
	tmuxPlaces = []string{filepath.Join(nothing, "missing"), homebrew}
	asked = 0
	if got := findTmux(); got != brewTmux {
		t.Errorf("tmux in a usual place: %q, want %q", got, brewTmux)
	}
	if asked != 0 {
		t.Errorf("the login shell was asked although a usual place had tmux")
	}
	if got := tmuxProgram(); got != brewTmux {
		t.Errorf("tmuxProgram: %q, want %q", got, brewTmux)
	}

	// Only on the PATH your login shell sets.
	tmuxPlaces = []string{nothing}
	loginFolder, loginTmux := standInTmux(t, 0o755)
	tmuxLoginPath = func() []string { return []string{"", nothing, loginFolder} }
	if got := findTmux(); got != loginTmux {
		t.Errorf("tmux on the login shell's PATH: %q, want %q", got, loginTmux)
	}

	// A file that can't run isn't tmux.
	noRun, _ := standInTmux(t, 0o644)
	tmuxPlaces = []string{noRun}
	tmuxLoginPath = func() []string { return nil }
	if got := findTmux(); got != "" {
		t.Errorf("a tmux that can't run: %q, want none", got)
	}

	// The PATH still comes first.
	onPath, pathTmux := standInTmux(t, 0o755)
	t.Setenv("PATH", onPath)
	tmuxPlaces = []string{homebrew}
	if got := findTmux(); got != pathTmux {
		t.Errorf("tmux on the PATH and in a usual place: %q, want the PATH's %q", got, pathTmux)
	}
}

// Where tmux isn't found, a tab's command keeps the bare name, as before.
func TestATabsTmuxKeepsItsNameWhereNoneIsFound(t *testing.T) {
	nothing := t.TempDir()
	t.Setenv("PATH", nothing)
	savedPlaces, savedLogin := tmuxPlaces, tmuxLoginPath
	t.Cleanup(func() { tmuxPlaces, tmuxLoginPath = savedPlaces, savedLogin })
	tmuxPlaces, tmuxLoginPath = []string{nothing}, func() []string { return nil }
	if got := tmuxProgram(); got != "tmux" {
		t.Errorf("tmuxProgram with none found: %q, want tmux", got)
	}
}
