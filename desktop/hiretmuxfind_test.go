package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
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

// freshLoginPath has the login shell's PATH not asked yet, as at start, with ask for the shell; as it was
// afterwards. An ask under way is waited for first (loginPathMu).
func freshLoginPath(t *testing.T, ask func() []string) {
	t.Helper()
	loginPathMu.Lock()
	savedPath, savedKnown, savedAsking, savedAsk := loginPath, loginPathKnown.Load(), loginPathAsking.Load(), askShellPath
	loginPath, askShellPath = nil, ask
	loginPathKnown.Store(false)
	loginPathAsking.Store(false)
	loginPathMu.Unlock()
	t.Cleanup(func() {
		loginPathMu.Lock()
		defer loginPathMu.Unlock()
		loginPath, askShellPath = savedPath, savedAsk
		loginPathKnown.Store(savedKnown)
		loginPathAsking.Store(savedAsking)
	})
}

// Finding tmux never waits for your login shell, as the deck's lock may be held (Nit 2 of RFXC3b): the
// shell is asked in the background, and its PATH counts once it has said.
func TestFindingTmuxNeverWaitsForTheLoginShell(t *testing.T) {
	if !hasTmuxHires {
		t.Skip("no tmux hires on this system")
	}
	nothing := t.TempDir()
	t.Setenv("PATH", nothing)
	saved := tmuxPlaces
	t.Cleanup(func() { tmuxPlaces = saved })
	tmuxPlaces = []string{nothing}
	loginFolder, loginTmux := standInTmux(t, 0o755)
	release := make(chan struct{})
	var released sync.Once
	let := func() { released.Do(func() { close(release) }) }
	asked := 0
	freshLoginPath(t, func() []string { asked++; <-release; return []string{loginFolder} })
	t.Cleanup(let) // before freshLoginPath's, which waits for the ask

	found := make(chan string, 1)
	go func() { found <- findTmux() }()
	select {
	case got := <-found:
		if got != "" {
			t.Errorf("tmux before the login shell said: %q, want none yet", got)
		}
	case <-time.After(2 * time.Second):
		let()
		t.Fatal("finding tmux waited for the login shell")
	}
	findTmux() // asked again meanwhile: still one shell
	let()
	for deadline := time.Now().Add(5 * time.Second); !loginPathKnown.Load(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the login shell's PATH never arrived")
		}
	}
	if got := findTmux(); got != loginTmux {
		t.Errorf("tmux once the login shell said: %q, want %q", got, loginTmux)
	}
	if asked != 1 {
		t.Errorf("the login shell was asked %d times, want once", asked)
	}
}

// Only full paths count from the login shell's PATH (Nit 3 of RFXC3b): a relative one would be looked for
// wherever this program runs. A stand-in shell says the PATH; no real start-up file is read.
func TestTheLoginShellsPATHKeepsOnlyFullPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no login shell, and the stand-in is a #!/bin/sh script")
	}
	shell := filepath.Join(t.TempDir(), "shell")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nprintf 'hello\\n<path>%s' 'bin:/opt/one::./two:/opt/three'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", shell)
	if got := loginShellPath(); !slices.Equal(got, []string{"/opt/one", "/opt/three"}) {
		t.Errorf("login shell's PATH: %q, want only the full paths", got)
	}
}

// With no SHELL, a Mac's login shell is zsh, as Terminal's and the Mac engine's (TerminalDrivers.swift):
// a zsh user's ~/.zprofile and ~/.zshrc still give the PATH their agents are on.
func TestWithNoShellTheMacsLoginShellIsZsh(t *testing.T) {
	t.Setenv("SHELL", "")
	want := "/bin/sh"
	if runtime.GOOS == "darwin" {
		want = macShell
	}
	if got := pathShell(); got != want {
		t.Errorf("the shell asked for its PATH with no SHELL: %q, want %q", got, want)
	}
}
