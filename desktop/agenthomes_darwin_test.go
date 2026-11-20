package desktop

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXAGENTHOMEMAC: a core started with CODEX_HOME (the Mac app now passes it, GoCore.agentPlaces) holds that folder in
// a desk's real profile, write denied as ~/.codex is. Without the variable, the same folder is the desk's to write:
// the core then can't know it's Codex's, which is how every Mac app's core was before.
func TestOnAMacACoreGivenCodexHomeHoldsItInADesksProfile(t *testing.T) {
	if err := sandbox.CheckSandboxExec(); err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal(err)
		}
		t.Skip(err)
	}
	for _, given := range []bool{false, true} {
		t.Run(map[bool]string{false: "not given", true: "given"}[given], func(t *testing.T) {
			moved := filepath.Join(physical(t.TempDir()), "dotfiles", "codex")
			if err := os.MkdirAll(moved, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CODEX_HOME", moved)
			if !given {
				os.Unsetenv("CODEX_HOME")
			}
			s := standInDesk(t)
			codex := filepath.Join(homeDir, ".codex")
			if err := os.MkdirAll(codex, 0o700); err != nil {
				t.Fatal(err)
			}
			if held := slices.Contains(s.places.AgentHomes, moved); held != given {
				t.Errorf("the desk's places hold %q: %q", moved, s.places.AgentHomes)
			}
			profile := deskProfileFor(t, s, filepath.Join(s.places.OwnSockets, deskSocketFile))
			writes := func(folder string) bool {
				cmd := exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/sh", "-c", `: > "$1/config.toml"`, "sh", folder)
				cmd.Dir = s.places.Root
				return cmd.Run() == nil
			}
			if !writes(s.places.Root) {
				t.Fatal("control: the desk can't write its own place")
			}
			if writes(codex) {
				t.Error("the desk wrote in ~/.codex")
			}
			if wrote := writes(moved); wrote == given {
				t.Errorf("CODEX_HOME given %v: the desk wrote in it %v", given, wrote)
			}
		})
	}
}

// sandboxExecHere skips a row where /usr/bin/sandbox-exec isn't fit to run, or fails it when the run requires it.
func sandboxExecHere(t *testing.T) {
	t.Helper()
	if err := sandbox.CheckSandboxExec(); err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal(err)
		}
		t.Skip(err)
	}
}

// deskDoes is whether a stand-in desk, under profile, at root, runs script (sh -c, its arguments $1…) to the end.
func deskDoes(profile, root, script string, args ...string) bool {
	cmd := exec.Command("/usr/bin/sandbox-exec", append([]string{"-f", profile, "/bin/sh", "-c", script, "sh"}, args...)...)
	cmd.Dir = root
	return cmd.Run() == nil
}

// FXAGENTHOMEMACR1 (RT126AGENTHOMEMAC B1): a moved place the owner can't make either, on a volume that isn't there,
// under a folder of root's, or under one of theirs they can't write in, doesn't stop a desk's start. The desk can't
// make it: in a folder the owner can write in again after the start (as a volume mounted later would be), the desk
// still can't make it, link it or rename one there, and the owner can.
func TestOnAMacAMovedPlaceTheOwnerCantMakeDoesntStopAStart(t *testing.T) {
	sandboxExecHere(t)
	if os.Geteuid() == 0 {
		t.Skip("as root, every folder can be made")
	}
	for _, variable := range []string{"CODEX_HOME", "XDG_CONFIG_HOME", "GEMINI_CLI_SYSTEM_SETTINGS_PATH"} {
		for _, where := range []string{"a missing volume", "a folder of root's", "a shut folder"} {
			t.Run(variable+" on "+where, func(t *testing.T) {
				var top, shut string // top: the first folder on the way that isn't there
				switch where {
				case "a missing volume":
					top = "/Volumes/SmolcorpNoSuchDrive" + filepath.Base(t.TempDir())
				case "a folder of root's":
					top = "/private/etc/smolcorp-standin-" + filepath.Base(t.TempDir())
				default:
					shut = filepath.Join(physical(t.TempDir()), "shut")
					if err := os.Mkdir(shut, 0o500); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(shut, 0o700) })
					top = filepath.Join(shut, "dotfiles")
				}
				if _, err := os.Lstat(top); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("%s: %v, want it missing", top, err)
				}
				place := filepath.Join(top, "moved", "x")
				value := place
				if variable == "GEMINI_CLI_SYSTEM_SETTINGS_PATH" {
					value = filepath.Join(place, "settings.json")
				}
				t.Setenv(variable, value)
				s := standInDesk(t)
				profile := filepath.Join(hireFolder(supportDir, s.at.ID), "sandbox.sb")
				if err := s.prepareProfile(profile, deskServices(s.places, filepath.Join(s.places.OwnSockets, deskSocketFile))); err != nil {
					t.Fatalf("the start was refused: %v", err)
				}
				if _, err := os.Lstat(top); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("%s was made: %v", top, err)
				}
				if shut != "" {
					// Now the owner could make it: the desk still can't.
					if err := os.Chmod(shut, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				if !deskDoes(profile, s.places.Root, `: > "$1/notes.md"`, s.places.Root) {
					t.Fatal("control: the desk can't write its own place")
				}
				if deskDoes(profile, s.places.Root, `mkdir -p "$1"`, place) {
					t.Errorf("the desk made %s", place)
				}
				if deskDoes(profile, s.places.Root, `mkdir "$1"`, top) {
					t.Errorf("the desk made %s", top)
				}
				if deskDoes(profile, s.places.Root, `ln -s "$2" "$1"`, top, s.places.Root) {
					t.Errorf("the desk made %s a link", top)
				}
				if deskDoes(profile, s.places.Root, `mkdir -p "$2/moved/x" && mv "$2" "$1"`, top, filepath.Join(s.places.Root, "staged")) {
					t.Errorf("the desk renamed a folder to %s", top)
				}
				if shut != "" {
					if err := os.Mkdir(top, 0o700); err != nil {
						t.Errorf("control: the owner can't make %s now: %v", top, err)
					}
				}
			})
		}
	}
}

// FXAGENTHOMEMACR1 (B1): a value no desk's profile can hold, the top of the disk or a place with a control character
// in it, still refuses the start, but the words name the variable and say to fix or unset it, never to turn the
// sandbox off. FXAGENTHOMEMACR2 (RT126AGENTHOMEMACR1 N1): the variable's own value is checked too, not only the place
// below it an agent reads, so XDG_CONFIG_HOME=/ (the config folder, held whole's links walked) and XDG_DATA_HOME=/
// are refused so as well.
func TestOnAMacAnAgentPlaceNoSandboxCanHoldNamesItsVariable(t *testing.T) {
	for _, c := range []struct{ variable, value string }{
		{"CODEX_HOME", "/"},
		{"CODEX_HOME", "//"},
		{"GROK_AUTH_PATH", "/"},
		{"GEMINI_CLI_HOME", "/tmp/a\nb"},
		{"XDG_CONFIG_HOME", "/"},
		{"XDG_CONFIG_HOME", "//"},
		{"XDG_DATA_HOME", "/"},
	} {
		t.Run(c.variable+"="+strconv.Quote(c.value), func(t *testing.T) {
			t.Setenv(c.variable, c.value)
			wantAgentPlaceWords(t, refusedStart(t), c.variable)
		})
	}
}

// refusedStart is the words a stand-in desk's start is refused with, as makeDeskSandbox gives them; it fails the test
// when the start isn't refused.
func refusedStart(t *testing.T) string {
	t.Helper()
	shortSupport(t)
	homeDir = t.TempDir()
	deskSockets(t)
	if err := os.MkdirAll(hireFolder(supportDir, "ab12"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := makeDeskSandbox(deskAt{ID: "ab12", Root: t.TempDir(), Socket: deskSocketPath("ab12", 0), Git: runGit})
	var refused *errSandboxNotMade
	if !errors.As(err, &refused) {
		t.Fatalf("makeDeskSandbox: %v, want a refusal", err)
	}
	return refused.Error()
}

// wantAgentPlaceWords fails the test unless words are agentPlaceWords' for variable: they name it, say to unset it,
// never to turn the sandbox off, on one line.
func wantAgentPlaceWords(t *testing.T, words, variable string) {
	t.Helper()
	if words != agentPlaceWords(variable) || !strings.Contains(words, variable) || !strings.Contains(words, "unset") ||
		strings.Contains(words, "sandbox off") || strings.Contains(words, "\n") {
		t.Errorf("the words: %q", words)
	}
}

// FXAGENTHOMEMACR2 (RT126AGENTHOMEMACR1 N1): a place one of these variables names that the start can't look all
// through (here a folder in it the owner can't read; `XDG_CONFIG_HOME=/private` meets root's) refuses the start in the
// same words, naming the variable, not in the words that say to turn the sandbox off.
func TestOnAMacAnAgentPlaceTheStartCantLookThroughNamesItsVariable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("as root, every folder can be read")
	}
	for _, c := range []struct{ variable, shut string }{
		{"CODEX_HOME", "shut"},             // held as an agent's own folder: looked at two levels deep
		{"XDG_CONFIG_HOME", "git/hooks/x"}, // its git folder held whole: looked at to the end
	} {
		t.Run(c.variable, func(t *testing.T) {
			moved := filepath.Join(physical(t.TempDir()), "dotfiles", "moved")
			shut := filepath.Join(moved, c.shut)
			if err := os.MkdirAll(shut, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(shut, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(shut, 0o700) })
			t.Setenv(c.variable, moved)
			wantAgentPlaceWords(t, refusedStart(t), c.variable)
		})
	}
}

// FXAGENTHOMEMACR1 (O2): the ignore file the owner's git config names (core.excludesFile), which every core read is
// handed (askGitHere), is held, and so is one a moved config folder's config names: a desk that wrote one could hide
// its files from every read.
func TestOnAMacADeskCantWriteTheIgnoreFileTheOwnersGitConfigNames(t *testing.T) {
	sandboxExecHere(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	home, elsewhere := physical(t.TempDir()), physical(t.TempDir())
	moved := filepath.Join(elsewhere, "config")
	writeRepoFile(t, home, ".gitconfig", "[core]\n\texcludesfile = ~/.gitignore_global\n")
	writeRepoFile(t, moved, "git/config", "[core]\n\texcludesfile = "+filepath.Join(elsewhere, "dotfiles", "ignore")+"\n")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", moved)
	s := standInDesk(t)
	profile := deskProfileFor(t, s, filepath.Join(s.places.OwnSockets, deskSocketFile))
	if !deskDoes(profile, s.places.Root, `: > "$1/other.txt"`, home) {
		t.Fatal("control: the desk can't write beside the ignore file")
	}
	for _, file := range []string{filepath.Join(home, ".gitignore_global"), filepath.Join(elsewhere, "dotfiles", "ignore")} {
		if deskDoes(profile, s.places.Root, `mkdir -p "$(dirname "$1")" && : > "$1"`, file) {
			t.Errorf("the desk wrote %s", file)
		}
	}
}

// FXAGENTHOMEMACR1 (O3): under a moved config folder (XDG_CONFIG_HOME), the places the owner's programs read there
// are held as ~/.config's are: git's, fish's, tmux's, autostart, systemd's user units, OpenCode's, and the terminals'
// (Ghostty, kitty, WezTerm, iTerm2's link). The rest of the folder is still the desk's to write.
func TestOnAMacAMovedConfigFoldersNeighboursAreHeldAsTheHomeFolders(t *testing.T) {
	sandboxExecHere(t)
	moved := filepath.Join(physical(t.TempDir()), "dotfiles", "config")
	if err := os.MkdirAll(filepath.Join(moved, "git"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", moved)
	s := standInDesk(t)
	profile := deskProfileFor(t, s, filepath.Join(s.places.OwnSockets, deskSocketFile))
	if !deskDoes(profile, s.places.Root, `mkdir -p "$1/other" && : > "$1/other/x"`, moved) {
		t.Fatal("control: the desk can't write elsewhere in the moved folder")
	}
	for _, file := range []string{"git/ignore", "git/attributes", "fish/config.fish", "tmux/tmux.conf", "autostart/x.desktop",
		"systemd/user/x.service", "opencode/opencode.json", "ghostty/config", "kitty/kitty.conf", "wezterm/wezterm.lua", "iterm2/x"} {
		at := filepath.Join(moved, file)
		if deskDoes(profile, s.places.Root, `mkdir -p "$(dirname "$1")" && : > "$1"`, at) {
			t.Errorf("the desk wrote %s", at)
		}
	}
}

// FXAGENTHOMEMACR2 (RT126AGENTHOMEMACR1 B1): at the core's start, a variable that moves an agent's place is unset
// unless it's a whole path (unsetAgentPlacesNotWhole). A relative XDG_CONFIG_HOME is read by git in the folder it runs
// in, the temp folder, which desks write in: the core's read of the owner's config (askGitHere) took the ignore file
// and the trusted folders of a config a desk wrote at $TMPDIR/rel/git/config, and carried them into every read.
func TestOnAMacARelativeConfigFolderGivesTheCoresReadsNothing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	temp, home := physical(t.TempDir()), physical(t.TempDir())
	t.Setenv("TMPDIR", temp)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "rel")
	writeRepoFile(t, temp, "rel/git/config", "[core]\n\texcludesfile = /planted/by/a/desk\n[safe]\n\tdirectory = *\n")
	carries := func() bool {
		return slices.ContainsFunc(askGitHere().owner, func(kv [2]string) bool {
			return kv[1] == "/planted/by/a/desk" || kv[0] == "safe.directory" && kv[1] == "*"
		})
	}
	if !carries() {
		t.Fatal("control: with XDG_CONFIG_HOME=rel, git didn't read the config in the temp folder")
	}
	unsetAgentPlacesNotWhole()
	if _, set := os.LookupEnv("XDG_CONFIG_HOME"); set {
		t.Error("XDG_CONFIG_HOME=rel is still set")
	}
	if carries() {
		t.Errorf("the core's reads carry what a desk wrote: %q", askGitHere().owner)
	}
}

// FXAGENTHOMEMACR2 (B1): neither does a sign-in, which runs in the temp folder too, get a relative or empty one; a whole
// path is kept as it was.
func TestOnAMacASignInIsGivenNoAgentPlaceButAWholePath(t *testing.T) {
	t.Setenv("CODEX_HOME", "rel/codex")
	t.Setenv("GROK_HOME", "")
	t.Setenv("OPENCODE_DB", "./opencode.db")
	t.Setenv("CODEX_SQLITE_HOME", "/Users/ada/codex-db")
	has := func(name string) bool {
		return slices.ContainsFunc(loginEnvironment(os.Environ(), ""), func(kv string) bool { return strings.HasPrefix(kv, name+"=") })
	}
	if !has("CODEX_HOME") {
		t.Fatal("control: a sign-in is handed CODEX_HOME before the core's start sees to it")
	}
	unsetAgentPlacesNotWhole()
	for _, name := range []string{"CODEX_HOME", "GROK_HOME", "OPENCODE_DB"} {
		if has(name) {
			t.Errorf("a sign-in is handed %s", name)
		}
	}
	if !slices.Contains(loginEnvironment(os.Environ(), ""), "CODEX_SQLITE_HOME=/Users/ada/codex-db") {
		t.Error("a whole path was taken away")
	}
}

// FXAGENTHOMEMACR2 (RT126AGENTHOMEMACR1 N2): the core asks for the owner's ignore file once (gitHere) and carries it
// into its reads for its life; a desk's start reads the config again. When the owner changes core.excludesFile while
// Smolcorp runs, a desk started after holds the one the core carries as well as the new one: it can write neither.
func TestOnAMacADeskHoldsTheIgnoreFileTheCoreCarriesAfterTheOwnerChangesIt(t *testing.T) {
	sandboxExecHere(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	home := physical(t.TempDir())
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	os.Unsetenv("XDG_CONFIG_HOME")
	writeRepoFile(t, home, ".gitconfig", "[core]\n\texcludesfile = ~/ignore-then\n")
	asked := askGitHere()
	if !slices.Contains(asked.owner, [2]string{"core.excludesfile", "~/ignore-then"}) {
		t.Fatalf("control: the core's ask: %q", asked.owner)
	}
	saved := gitHere
	gitHere = func() gitHereInfo { return asked }
	t.Cleanup(func() { gitHere = saved })
	writeRepoFile(t, home, ".gitconfig", "[core]\n\texcludesfile = ~/ignore-now\n")
	s := standInDesk(t)
	profile := deskProfileFor(t, s, filepath.Join(s.places.OwnSockets, deskSocketFile))
	if !deskDoes(profile, s.places.Root, `: > "$1/other.txt"`, home) {
		t.Fatal("control: the desk can't write beside the ignore files")
	}
	for _, name := range []string{"ignore-then", "ignore-now"} {
		if deskDoes(profile, s.places.Root, `: > "$1"`, filepath.Join(home, name)) {
			t.Errorf("the desk wrote ~/%s", name)
		}
	}
}

// FXAGENTHOMEMACR2 (RT126AGENTHOMEMACR1 O1): an ignore file spelled from another account's home folder or from git's
// own still refuses the start, but the words name core.excludesFile and its value, say what form it has, and how to
// spell it instead.
func TestOnAMacAnIgnoreFileNoStartCanTellSaysHowToSpellIt(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	for _, c := range []struct{ value, form string }{
		{"~ada/.gitignore", "another account's home folder"},
		{"%(prefix)/etc/gitignore", "the folder git is installed in"},
	} {
		t.Run(c.value, func(t *testing.T) {
			home := physical(t.TempDir())
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", "")
			os.Unsetenv("XDG_CONFIG_HOME")
			writeRepoFile(t, home, ".gitconfig", "[core]\n\texcludesfile = "+c.value+"\n")
			nothingCarried(t)
			words := refusedStart(t)
			for _, want := range []string{"core.excludesFile", strconv.Quote(c.value), c.form, "whole path", "~/", "quit and reopen Smolcorp"} {
				if !strings.Contains(words, want) {
					t.Errorf("the words don't say %q: %q", want, words)
				}
			}
			if words != excludesFileWords(c.value) || strings.Contains(words, "sandbox off") {
				t.Errorf("the words: %q", words)
			}
		})
	}
}

// FXAGENTHOMEMACR3 (RT126AGENTHOMEMACR2 N1): the core asks for the ignore file once for its life (gitHere), so once the
// owner has fixed their config as the words say, the value the core carries is still refused: the words say to quit
// and reopen Smolcorp, which asks again.
func TestOnAMacAnIgnoreFileTheCoreCarriesSaysToReopenSmolcorp(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	home := physical(t.TempDir())
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	os.Unsetenv("XDG_CONFIG_HOME")
	writeRepoFile(t, home, ".gitconfig", "[core]\n\texcludesfile = ~/.gitignore\n")
	saved := gitHere
	gitHere = func() gitHereInfo { return gitHereInfo{owner: [][2]string{{"core.excludesfile", "~ada/.gitignore"}}} }
	t.Cleanup(func() { gitHere = saved })
	words := refusedStart(t)
	if words != excludesFileWords("~ada/.gitignore") || !strings.Contains(words, strconv.Quote("~ada/.gitignore")) ||
		!strings.Contains(words, "quit and reopen Smolcorp") {
		t.Errorf("the words: %q", words)
	}
}

// FXAGENTHOMEMACR3 (RT126AGENTHOMEMACR2 B1): a profile holds an agent's place as cleaned, $HOME/dots/../cfg as
// $HOME/cfg, while the kernel takes the `..` after the link: with ~/dots a link a desk made into its own worktree,
// XDG_CONFIG_HOME=$HOME/dots/../cfg had the core's read of the owner's git config (askGitHere) carry an ignore file and
// trusted folders the desk wrote. The core's start unsets such a value and keeps its name, its reads carry nothing of
// it, and every desk's start is refused in the variable's words.
func TestOnAMacAConfigFolderWithADotDotThroughADesksLinkGivesTheCoreNothingAndRefusesTheStart(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	t.Cleanup(func() { agentPlacesDropped = nil })
	home := physical(t.TempDir())
	t.Setenv("HOME", home)
	worktree := filepath.Join(home, "desk-worktree")
	writeRepoFile(t, worktree, "cfg/git/config", "[core]\n\texcludesfile = /planted/by/a/desk\n[safe]\n\tdirectory = *\n")
	if err := os.MkdirAll(filepath.Join(worktree, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(worktree, "sub"), filepath.Join(home, "dots")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", home+"/dots/../cfg")
	carries := func() bool {
		return slices.ContainsFunc(askGitHere().owner, func(kv [2]string) bool {
			return kv[1] == "/planted/by/a/desk" || kv[0] == "safe.directory" && kv[1] == "*"
		})
	}
	if !carries() {
		t.Fatal("control: git didn't read the config the desk wrote through ~/dots")
	}
	unsetAgentPlacesNotWhole()
	if _, set := os.LookupEnv("XDG_CONFIG_HOME"); set {
		t.Error("XDG_CONFIG_HOME is still set")
	}
	if carries() {
		t.Errorf("the core's reads carry what a desk wrote: %q", askGitHere().owner)
	}
	wantAgentPlaceWords(t, refusedStart(t), "XDG_CONFIG_HOME")
}

// FXAGENTHOMEMACR3 (B1): so is any value spelled with a `.` or a doubled `/` in it, and `<a link to /private>/..`,
// which the kernel takes as the top of the disk (RT126AGENTHOMEMACR2 N1's gap): as the core is given it, and after its
// start has unset it, or the Mac app has passed only its name. A `/` at the end is no such spelling.
func TestOnAMacAnAgentPlaceSpelledWithADotOrADoubledSlashIsRefused(t *testing.T) {
	t.Cleanup(func() { agentPlacesDropped = nil })
	home := physical(t.TempDir())
	t.Setenv("HOME", home)
	links := physical(t.TempDir())
	if err := os.Symlink("/private", filepath.Join(links, "private")); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ variable, value string }{
		{"CODEX_HOME", home + "/a/./b"},
		{"CODEX_HOME", home + "//x"},
		{"XDG_DATA_HOME", filepath.Join(links, "private") + "/.."},
		{"GROK_AUTH_PATH", home + "/a/../auth.json"},
	} {
		t.Run(c.variable+"="+strings.TrimPrefix(c.value, home), func(t *testing.T) {
			t.Setenv(c.variable, c.value)
			agentPlacesDropped = nil
			t.Run("as given", func(t *testing.T) { wantAgentPlaceWords(t, refusedStart(t), c.variable) })
			t.Run("after the core's start", func(t *testing.T) {
				unsetAgentPlacesNotWhole()
				if _, set := os.LookupEnv(c.variable); set {
					t.Errorf("%s is still set", c.variable)
				}
				wantAgentPlaceWords(t, refusedStart(t), c.variable)
			})
		})
	}
	t.Run("named by the Mac app", func(t *testing.T) {
		t.Setenv(agentPlacesDroppedVariable, "OPENCODE_DB,NOT_ONE_OF_THEM")
		unsetAgentPlacesNotWhole()
		if !slices.Equal(agentPlacesDropped, []string{"OPENCODE_DB"}) {
			t.Errorf("kept: %q", agentPlacesDropped)
		}
		wantAgentPlaceWords(t, refusedStart(t), "OPENCODE_DB")
	})
	agentPlacesDropped = nil
	for _, value := range []string{home + "/a/b/", home + "/a/b"} {
		if sandbox.AgentPlaceUnclean(value) {
			t.Errorf("%q is taken as spelled with a . or a doubled /", value)
		}
	}
}

// FXAGENTHOMEMACR3 (RT126AGENTHOMEMACR2 O1): a start whose link check stops in a place one of these variables names,
// too many files there to look at (OPENCODE_DB=/Users did), is refused in words that name the variable.
func TestOnAMacAPlaceWithTooManyFilesToLookAtNamesItsVariable(t *testing.T) {
	t.Setenv("HOME", physical(t.TempDir()))
	t.Setenv("XDG_CONFIG_HOME", "")
	os.Unsetenv("XDG_CONFIG_HOME")
	nothingCarried(t)
	most := linkCheckMost
	t.Cleanup(func() { linkCheckMost = most })
	// A stand-in for 100,000 entries: 2,000, more than the rest of the start looks at, and fewer than the folder holds.
	linkCheckMost = 2000
	big := filepath.Join(physical(t.TempDir()), "opencode-db")
	if err := os.MkdirAll(big, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_DB", big)
	shortSupport(t)
	homeDir = t.TempDir()
	deskSockets(t)
	if err := os.MkdirAll(hireFolder(supportDir, "ab12"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := makeDeskSandbox(deskAt{ID: "ab12", Root: t.TempDir(), Socket: deskSocketPath("ab12", 0), Git: runGit}); err != nil {
		t.Fatalf("control: with the folder empty, the start: %v", err)
	}
	for i := range 2100 {
		writeRepoFile(t, big, filepath.Join("d"+strconv.Itoa(i%20), "f"+strconv.Itoa(i)), "x")
	}
	wantAgentPlaceWords(t, refusedStart(t), "OPENCODE_DB")
}

// nothingCarried stands in for gitHere with this git's version and no ignore file or trusted folder carried, for the
// test's life. gitHere asks once for the process: a start that asked it in a stand-in home would carry that home's
// ignore file into every later row's start.
func nothingCarried(t *testing.T) {
	t.Helper()
	here := askGitHere()
	here.owner = nil
	saved := gitHere
	gitHere = func() gitHereInfo { return here }
	t.Cleanup(func() { gitHere = saved })
}
