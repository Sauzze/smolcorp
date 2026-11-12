//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestAHiresAgentIsLookedForInTheLoginShellItStartsIn(t *testing.T) {
	home := isolate(t)
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("ENV", "") // an interactive sh reads the file ENV names
	// The agent is on Smolcorp's PATH, in a folder of its own, as one installed in a terminal's session.
	agents := t.TempDir()
	agent := filepath.Join(agents, "smolcorp-test-agent")
	if err := os.WriteFile(agent, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", agents+string(os.PathListSeparator)+os.Getenv("PATH"))

	// A login file that sets PATH afresh, as some systems' do: the folder is lost.
	writeFile(t, filepath.Join(home, ".profile"), "PATH=/usr/bin:/bin\necho a line of the profile's own\n")
	if found, known := findInLoginShell("smolcorp-test-agent"); !known || found != "" {
		t.Errorf("with PATH set afresh: found %q (known %v), want not found", found, known)
	}
	message := hireAgentMissing("smolcorp-test-agent")
	if !strings.Contains(message, "(sh)") || !strings.Contains(message, `export PATH="`+agents+`:$PATH"`) {
		t.Errorf("the message is %q, want the shell, and the line that puts the folder Smolcorp found it in on PATH", message)
	}

	// The line added: found, and a hire can start.
	writeFile(t, filepath.Join(home, ".profile"), "PATH=/usr/bin:/bin\nexport PATH=\""+agents+":$PATH\"\n")
	if found, known := findInLoginShell("smolcorp-test-agent"); !known || found != agent {
		t.Errorf("with the folder on PATH: found %q (known %v), want %s", found, known, agent)
	}
	if message := hireAgentMissing("smolcorp-test-agent"); message != "" {
		t.Errorf("a hire whose agent is found is refused: %q", message)
	}
}

// fakeShell is a program at path that starts as a shell would, running body in place of one.
func fakeShell(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fakeshell")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheLoginShellIsTheOneSHELLNamesElseBashElseSh(t *testing.T) {
	isolate(t)
	t.Setenv("SHELL", "/usr/bin/zsh")
	if got := loginShell(); got != "/usr/bin/zsh" {
		t.Errorf("with SHELL set the login shell is %q, want /usr/bin/zsh", got)
	}

	// SHELL unset (or empty): the bash on PATH, as a terminal's own start would have it; on a Mac, zsh, as
	// Terminal and the Mac's engine (TerminalDrivers.swift) start, so ~/.zprofile and ~/.zshrc are read.
	bin := t.TempDir()
	bash := filepath.Join(bin, "bash")
	if err := os.WriteFile(bash, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", "")
	t.Setenv("PATH", bin)
	withBash, withNone := bash, "/bin/sh"
	if runtime.GOOS == "darwin" {
		withBash, withNone = macShell, macShell
	}
	if got := loginShell(); got != withBash {
		t.Errorf("with SHELL empty the login shell is %q, want %s", got, withBash)
	}
	if got := inLoginShell([]string{"claude"}); got[0] != withBash {
		t.Errorf("a hire's command with SHELL empty: %q", got)
	}

	// No bash on PATH either.
	t.Setenv("PATH", t.TempDir())
	if got := loginShell(); got != withNone {
		t.Errorf("with SHELL empty and no bash the login shell is %q, want %s", got, withNone)
	}
}

func TestAHireIsRunInTheLoginShellWithEveryWordQuoted(t *testing.T) {
	isolate(t)
	t.Setenv("SHELL", "/bin/zsh")
	got := inLoginShell([]string{"claude", "it's; rm -rf ~", "$HOME"})
	want := []string{"/bin/zsh", "-lic", `exec 'claude' 'it'\''s; rm -rf ~' '$HOME'`}
	if !slices.Equal(got, want) {
		t.Errorf("a hire is run as %q, want %q", got, want)
	}
}

func TestAnAgentNotOnAnyPathIsToldAsNotInstalled(t *testing.T) {
	home := isolate(t)
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("ENV", "") // an interactive sh reads the file ENV names
	writeFile(t, filepath.Join(home, ".profile"), "PATH=/usr/bin:/bin\n")

	if found, known := findInLoginShell("smolcorp-test-nowhere"); !known || found != "" {
		t.Errorf("found %q (known %v), want not found, known", found, known)
	}
	want := "Can't hire: smolcorp-test-nowhere isn't installed, or your login shell (sh), where hires start, doesn't find smolcorp-test-nowhere. " +
		"Install smolcorp-test-nowhere, or add the folder it's in to your PATH in ~/.profile, then hire again."
	if got := hireAgentMissing("smolcorp-test-nowhere"); got != want {
		t.Errorf("the message is\n%s\nwant\n%s", got, want)
	}
	// A known agent is called by its name, not its program.
	if got := hireAgentMissing("claude-test-nowhere"); strings.Contains(got, "Claude Code") {
		t.Errorf("an unknown program is called by an agent's name: %s", got)
	}
	if got := agentNotFoundMessage("claude", "/bin/sh", ""); !strings.Contains(got, "Can't hire: Claude Code isn't installed") || !strings.Contains(got, "Install Claude Code,") {
		t.Errorf("the message for claude is %q, want it to name Claude Code", got)
	}
}

func TestAnAgentTheAppFindsButTheLoginShellDoesntIsToldWhereItIs(t *testing.T) {
	home := isolate(t)
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("ENV", "") // an interactive sh reads the file ENV names
	writeFile(t, filepath.Join(home, ".profile"), "PATH=/usr/bin:/bin\n")
	folder := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	// Not on the app's PATH, but in a folder installers use: whereAppFinds looks there too.
	if got := whereAppFinds("smolcorp-test-agent"); got != "" {
		t.Fatalf("found %q before it's installed", got)
	}
	agent := filepath.Join(folder, "smolcorp-test-agent")
	writeFile(t, agent, "#!/bin/sh\n")
	if got := whereAppFinds("smolcorp-test-agent"); got != agent {
		t.Errorf("whereAppFinds is %q, want %s", got, agent)
	}
	want := "Can't hire: your login shell (sh), where hires start, doesn't find smolcorp-test-agent, which is in ~/.local/bin. " +
		`Add this line at the end of ~/.profile, then hire again: export PATH="$HOME/.local/bin:$PATH"`
	if got := hireAgentMissing("smolcorp-test-agent"); got != want {
		t.Errorf("the message is\n%s\nwant\n%s", got, want)
	}

	// On the app's own PATH, that is what's found first.
	onPath := t.TempDir()
	first := filepath.Join(onPath, "smolcorp-test-agent")
	if err := os.WriteFile(first, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", onPath+string(os.PathListSeparator)+os.Getenv("PATH"))
	if got := whereAppFinds("smolcorp-test-agent"); got != first {
		t.Errorf("whereAppFinds is %q, want the one on PATH, %s", got, first)
	}
}

func TestTheLineToAddIsTheOneForTheShellAndItsFile(t *testing.T) {
	home := isolate(t)
	agent := filepath.Join(home, ".npm-global", "bin", "claude")
	for _, c := range []struct{ shell, file, line string }{
		{"/usr/bin/zsh", "~/.zprofile", `export PATH="$HOME/.npm-global/bin:$PATH"`},
		{"/usr/bin/fish", "~/.config/fish/config.fish", "fish_add_path ~/.npm-global/bin"},
		{"/bin/bash", "~/.profile", `export PATH="$HOME/.npm-global/bin:$PATH"`},
		{"/bin/sh", "~/.profile", `export PATH="$HOME/.npm-global/bin:$PATH"`},
	} {
		got := agentNotFoundMessage("claude", c.shell, agent)
		if !strings.Contains(got, "Add this line at the end of "+c.file+", then hire again: "+c.line) || !strings.Contains(got, "which is in ~/.npm-global/bin.") {
			t.Errorf("for %s the message is %q, want the file %s and the line %s", c.shell, got, c.file, c.line)
		}
	}
	// Bash reads the first of its own login files it finds, so that's the one to change.
	writeFile(t, filepath.Join(home, ".bash_login"), "")
	if got := agentNotFoundMessage("claude", "/bin/bash", agent); !strings.Contains(got, "end of ~/.bash_login,") {
		t.Errorf("with a .bash_login the message is %q, want it to name that file", got)
	}
	writeFile(t, filepath.Join(home, ".bash_profile"), "")
	if got := agentNotFoundMessage("claude", "/bin/bash", agent); !strings.Contains(got, "end of ~/.bash_profile,") {
		t.Errorf("with a .bash_profile the message is %q, want it to name that file", got)
	}
}

func TestAShellFilesNoiseDoesntHideWhereTheAgentIs(t *testing.T) {
	home := isolate(t)
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("ENV", "") // an interactive sh reads the file ENV names
	agents := t.TempDir()
	agent := filepath.Join(agents, "smolcorp-test-agent")
	if err := os.WriteFile(agent, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Noise on both streams, one line ending without a newline, two that only look like the answer (a longer marker, an indented one), and a command that fails.
	writeFile(t, filepath.Join(home, ".profile"),
		"PATH=\""+agents+":/usr/bin:/bin\"\n"+
			"echo 'Welcome back'\necho 'a warning' >&2\nfalse\nnot-a-command-smolcorp 2>/dev/null\n"+
			"echo 'smolcorp-found-not:/elsewhere'\necho ' smolcorp-found:/indented/elsewhere'\nprintf 'no newline'\n")
	if found, known := findInLoginShell("smolcorp-test-agent"); !known || found != agent {
		t.Errorf("with a noisy profile: found %q (known %v), want %s", found, known, agent)
	}
	if message := hireAgentMissing("smolcorp-test-agent"); message != "" {
		t.Errorf("a noisy profile refuses a hire: %q", message)
	}
}

func TestAShellFileThatEndsTheShellLeavesItUnknownAndTheHireAllowed(t *testing.T) {
	home := isolate(t)
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("ENV", "") // an interactive sh reads the file ENV names
	// A profile that exits before the shell can say anything.
	writeFile(t, filepath.Join(home, ".profile"), "echo 'bye'\nexit 3\n")
	if found, known := findInLoginShell("smolcorp-test-agent"); known || found != "" {
		t.Errorf("with a profile that exits: found %q (known %v), want unknown", found, known)
	}
	if message := hireAgentMissing("smolcorp-test-agent"); message != "" {
		t.Errorf("a hire is refused when it can't be told: %q", message)
	}
}

func TestASHELLThatIsMissingOrNotAShellLeavesItUnknownAndTheHireAllowed(t *testing.T) {
	for name, shell := range map[string]string{
		"a path that isn't there": filepath.Join(t.TempDir(), "no-such-shell"),
		"a folder":                t.TempDir(),
		"a program that's not a shell and prints nothing": "/bin/true",
		"a fake that prints other lines":                  fakeShell(t, "echo 'not a shell'\necho 'smolcorp-found'\n"),
		"a fake that fails":                               fakeShell(t, "exit 1\n"),
	} {
		t.Run(name, func(t *testing.T) {
			home := isolate(t)
			t.Setenv("HOME", home)
			t.Setenv("ENV", "")
			t.Setenv("SHELL", shell)
			if found, known := findInLoginShell("smolcorp-test-agent"); known || found != "" {
				t.Errorf("found %q (known %v), want unknown", found, known)
			}
			if message := hireAgentMissing("smolcorp-test-agent"); message != "" {
				t.Errorf("a hire is refused when it can't be told: %q", message)
			}
		})
	}
}

func TestAFakeShellThatAnswersIsBelievedAndItsWordsAreKept(t *testing.T) {
	home := isolate(t)
	t.Setenv("HOME", home)
	t.Setenv("ENV", "")
	// Runs the way a shell does for -lic: its third word is the line, run by sh.
	t.Setenv("SHELL", fakeShell(t, "echo 'banner'\nexec /bin/sh -c \"$2\"\n"))
	if found, known := findInLoginShell("sh"); !known || filepath.Base(found) != "sh" {
		t.Errorf("found %q (known %v), want the sh the fake shell's PATH has", found, known)
	}
	if found, known := findInLoginShell("smolcorp-test-nowhere"); !known || found != "" {
		t.Errorf("found %q (known %v), want not found, known", found, known)
	}
	// A program name is a word, never a line: this one is only looked for.
	marker := filepath.Join(home, "ran")
	if found, known := findInLoginShell("x; touch " + marker); !known || found != "" {
		t.Errorf("found %q (known %v), want not found, known", found, known)
	}
	if fileExists(marker) {
		t.Error("a program name was run as a command")
	}
}

func TestAnEmptyProgramIsNeverLookedFor(t *testing.T) {
	home := isolate(t)
	t.Setenv("HOME", home)
	t.Setenv("ENV", "")
	// A shell that would refuse the hire if it were run: it says nothing is found, and leaves a mark.
	ran := filepath.Join(t.TempDir(), "ran")
	t.Setenv("SHELL", fakeShell(t, "touch '"+ran+"'\nexec /bin/sh -c \"$2\"\n"))
	if found, known := findInLoginShell("smolcorp-test-nowhere"); !known || found != "" {
		t.Fatalf("the fake shell isn't one that refuses: found %q (known %v), want not found, known", found, known)
	}
	if err := os.Remove(ran); err != nil {
		t.Fatalf("the fake shell didn't leave its mark: %v", err)
	}
	if got := hireAgentMissing(""); got != "" {
		t.Errorf("an empty program is refused: %q", got)
	}
	if fileExists(ran) {
		t.Error("an empty program started a login shell")
	}
}
