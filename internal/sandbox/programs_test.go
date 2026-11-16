//go:build !windows

package sandbox

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// SA-X2's programs: every program Smolcorp runs for itself, resolved once, with its links, its whole install root,
// what it loads, and the places a lookup takes first, all denied to every desk. Each test runs on stand-ins in its own
// temp folder, with the program list put in its place (onlyPrograms), so nothing of this computer's is read but what
// a stand-in names (/bin/sh, /usr/bin/env).

// standins is a test's temp folder, links resolved (so its own place has no link on the way), with a home in it.
type standins struct {
	t          *testing.T
	base, home string
}

func newStandins(t *testing.T) standins {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return standins{t: t, base: base, home: filepath.Join(base, "home")}
}

func (s standins) at(rel string) string { return filepath.Join(s.base, filepath.FromSlash(rel)) }

// program makes an executable file at rel with text.
func (s standins) program(rel, text string) string {
	s.t.Helper()
	at := s.at(rel)
	if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
		s.t.Fatal(err)
	}
	if err := os.WriteFile(at, []byte(text), 0o700); err != nil {
		s.t.Fatal(err)
	}
	return at
}

func (s standins) dir(rel string) string {
	s.t.Helper()
	if err := os.MkdirAll(s.at(rel), 0o700); err != nil {
		s.t.Fatal(err)
	}
	return s.at(rel)
}

// link makes a link at rel to target, as written.
func (s standins) link(rel, target string) string {
	s.t.Helper()
	at := s.at(rel)
	if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
		s.t.Fatal(err)
	}
	if err := os.Symlink(target, at); err != nil {
		s.t.Fatal(err)
	}
	return at
}

// onlyPrograms puts names in the program list's place for the test.
func onlyPrograms(t *testing.T, names ...ProgramName) {
	saved := programNames
	programNames = names
	t.Cleanup(func() { programNames = saved })
}

func mustResolve(t *testing.T, f Finding) *Programs {
	t.Helper()
	ps, err := ResolvePrograms(f)
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

// denied says, for each place, whether InstallRoots holds it (it or a folder above it), and fails for each it doesn't
// say as want.
func denied(t *testing.T, ps *Programs, goos string, want map[string]bool) {
	t.Helper()
	roots := ps.InstallRoots()
	for place, deny := range want {
		got := slices.ContainsFunc(roots, func(r string) bool { return under(key(goos, place), key(goos, r)) })
		if got != deny {
			t.Errorf("%s denied: %v, want %v", place, got, deny)
		}
	}
	if t.Failed() {
		t.Logf("the install roots: %q", roots)
	}
}

func TestAHomebrewProgramIsDeniedWithItsKegEveryLinkAndTheFoldersSearchedFirst(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t, ProgramName{Name: "gh", Why: "a stand-in"})
	s.program("brew/Cellar/gh/2.0/bin/gh", "#!/bin/sh\n")
	// bin/gh → a middle link → the keg: the middle one is denied too, or it could be put back elsewhere.
	s.link("mid/gh", "../brew/Cellar/gh/2.0/bin/gh")
	s.link("brew/bin/gh", "../../mid/gh")
	s.dir("early")
	s.dir("late")
	f := Finding{GOOS: "linux", Home: s.home, Search: []string{filepath.Join(s.home, ".local", "bin"), "relative/bin", s.at("early"), s.at("brew/bin"), s.at("late")}}
	ps := mustResolve(t, f)
	if got := ps.Path("gh"); got != s.at("brew/bin/gh") {
		t.Errorf("gh is %q, want the one the search found first", got)
	}
	denied(t, ps, "linux", map[string]bool{
		s.at("brew/Cellar/gh/2.0/bin/gh"): true, s.at("brew/Cellar/gh/2.0/bin/gh-planted"): true, s.at("brew/Cellar/gh/3.0"): true,
		s.at("brew/bin/gh"): true, s.at("mid/gh"): true, s.at("early/gh"): true, filepath.Join(s.home, ".local", "bin", "gh"): true,
		// The folder after it too: a caller with another order would take it (N1).
		s.at("late/gh"): true,
		// Not the folders, nor other programs: brew install still works.
		s.at("brew/bin"): false, s.at("brew/bin/jq"): false, s.at("early/jq"): false, s.at("late/jq"): false, s.at("mid/other"): false,
		filepath.Join(s.home, ".local", "bin", "jq"): false, s.home: false,
	})

	// Through a desk's Policy: a program planted in the keg, or put first on the search, is denied by the row for
	// programs that run outside; the home folder's missing bin is made before the desk starts (Hold).
	p := mustNew(t, Places{GOOS: "linux", Home: s.home, Support: filepath.Join(s.home, "support"), OwnID: "h1",
		Root: filepath.Join(s.home, "P", "app"), InstallRoots: ps.InstallRoots()})
	for _, place := range []string{s.at("brew/Cellar/gh/2.0/bin/gh-planted"), s.at("early/gh"), s.at("brew/bin/gh"), filepath.Join(s.home, ".local", "bin", "gh")} {
		if v, r := p.Decide(place, Write); v != Deny || r.Row != "programs that run outside" {
			t.Errorf("a desk writing %s: %v by %q", place, v, r.Row)
		}
	}
	if v, _ := p.Decide(s.at("brew/bin/jq"), Write); v != Allow {
		t.Errorf("a desk installing another program: %v", v)
	}
	if !slices.Contains(p.Hold(), filepath.Join(s.home, ".local", "bin")) {
		t.Errorf("the home folder's missing bin isn't held: %q", p.Hold())
	}
}

func TestAProgramIsResolvedOnceAndOnePutFirstLaterIsntTaken(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t, ProgramName{Name: "git", Why: "a stand-in"})
	s.program("brew/bin/git", "#!/bin/sh\n")
	s.dir("early")
	ps := mustResolve(t, Finding{GOOS: "linux", Home: s.home, Search: []string{s.at("early"), s.at("brew/bin")}})
	s.program("early/git", "#!/bin/sh\necho planted\n")
	if got := ps.Path("git"); got != s.at("brew/bin/git") {
		t.Errorf("after a git was put first, git is %q", got)
	}
	denied(t, ps, "linux", map[string]bool{s.at("early/git"): true})
	if ps.Path("gh") != "" {
		t.Errorf("a program off the list has a path: %q", ps.Path("gh"))
	}
}

// RT126SAX2 N1's probe: findGH looks on the login shell's PATH before the core's own, Finding.Search the other way
// round. A gh found on the PATH must leave no login-path folder after it (~/.local/bin) open to a planted gh.
func TestAProgramFoundIsHeldInTheFoldersAfterItForACallerThatLooksThereFirst(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t, ProgramName{Name: "gh", Why: "a stand-in"})
	s.program("path/gh", "#!/bin/sh\n")
	s.dir("login")
	local := filepath.Join(s.home, ".local", "bin")
	ps := mustResolve(t, Finding{GOOS: "linux", Home: s.home, Search: []string{s.at("path"), s.at("login"), local}})
	if ps.Path("gh") != s.at("path/gh") {
		t.Errorf("gh is %q, want the one first in Search's order", ps.Path("gh"))
	}
	denied(t, ps, "linux", map[string]bool{s.at("login/gh"): true, filepath.Join(local, "gh"): true, s.at("login/jq"): false, filepath.Join(local, "jq"): false})
	p := mustNew(t, Places{GOOS: "linux", Home: s.home, Support: filepath.Join(s.home, "support"), OwnID: "h1",
		Root: filepath.Join(s.home, "P", "app"), InstallRoots: ps.InstallRoots()})
	if v, r := p.Decide(filepath.Join(local, "gh"), Write); v != Deny || r.Row != "programs that run outside" {
		t.Errorf("a desk planting gh in the login path's ~/.local/bin: %v by %q", v, r.Row)
	}
}

func TestAProgramNotFoundIsHeldInEverySearchFolderAndPlace(t *testing.T) {
	s := newStandins(t)
	s.dir("a")
	s.dir("apps")
	onlyPrograms(t, ProgramName{Name: "kitten", Places: []string{s.at("apps/kitty.app/Contents/MacOS/kitten"), s.at("nope/deeper/bin/kitten")}, Why: "a stand-in"})
	// A search folder through a link: the place is denied where the link leads too, as the kernel checks it.
	s.dir("real")
	s.link("linked", "real")
	ps := mustResolve(t, Finding{GOOS: "darwin", Home: s.home, Search: []string{s.at("a"), s.at("linked"), filepath.Join(s.home, "bin")}})
	if ps.Path("kitten") != "" {
		t.Errorf("kitten was found at %q", ps.Path("kitten"))
	}
	denied(t, ps, "darwin", map[string]bool{
		s.at("a/kitten"): true, s.at("linked/kitten"): true, s.at("real/kitten"): true, filepath.Join(s.home, "bin", "kitten"): true,
		// Outside the home folder, a missing place is denied from the first folder missing on the way: the app, the
		// folder that would hold the rest. Nothing then has to be made for it.
		s.at("apps/kitty.app"): true, s.at("nope"): true, s.at("apps/other.app"): false, s.at("a/other"): false,
	})
	if roots := ps.InstallRoots(); slices.Contains(roots, s.at("apps/kitty.app/Contents/MacOS/kitten")) || slices.Contains(roots, s.at("nope/deeper/bin/kitten")) {
		t.Errorf("a missing place outside the home folder is denied below what a desk could make: %q", roots)
	}
}

func TestEachKindOfInstallIsDeniedWhole(t *testing.T) {
	for _, c := range []struct {
		goos, home, dev, real, want string
	}{
		{"darwin", "/Users/u", "", "/Applications/WezTerm.app/Contents/MacOS/wezterm", "/Applications/WezTerm.app"},
		{"darwin", "/Users/u", "", "/Applications/Outer.app/Contents/Helpers/Inner.app/Contents/MacOS/x", "/Applications/Outer.app"},
		{"darwin", "/Users/u", "", "/opt/homebrew/Cellar/tmux/3.5a/bin/tmux", "/opt/homebrew/Cellar/tmux"},
		{"darwin", "/Users/u", "", "/opt/homebrew/Caskroom/claude-code/2.1.0/claude", "/opt/homebrew/Caskroom/claude-code"},
		{"darwin", "/Users/u", "", "/opt/homebrew/Cellar/gemini-cli/0.9/libexec/lib/node_modules/@google/gemini-cli/dist/index.js", "/opt/homebrew/Cellar/gemini-cli"},
		{"darwin", "/Users/u", "", "/opt/homebrew/lib/node_modules/@openai/codex/bin/codex.js", "/opt/homebrew/lib/node_modules/@openai/codex"},
		{"linux", "/home/u", "", "/home/u/.npm-global/lib/node_modules/opencode-ai/bin/opencode", "/home/u/.npm-global/lib/node_modules/opencode-ai"},
		{"linux", "/home/u", "", "/home/u/.local/share/pnpm/global/5/node_modules/.pnpm/x@1/node_modules/x/bin/x", "/home/u/.local/share/pnpm/global/5/node_modules/.pnpm"},
		{"darwin", "/Users/u", "", "/Users/u/.local/share/claude/versions/2.1.325", "/Users/u/.local/share/claude"},
		{"darwin", "/Users/u", "/Library/Developer/CommandLineTools", "/Library/Developer/CommandLineTools/usr/libexec/git-core/git", "/Library/Developer/CommandLineTools"},
		{"darwin", "/Users/u", "/Applications/Xcode.app", "/Applications/Xcode.app/Contents/Developer/usr/bin/git", "/Applications/Xcode.app"},
		// Anywhere else, the program itself.
		{"darwin", "/Users/u", "", "/usr/bin/git", "/usr/bin/git"},
		{"linux", "/home/u", "", "/home/u/.nvm/versions/node/v22/bin/node", "/home/u/.nvm/versions/node/v22/bin/node"},
		{"linux", "/home/u", "", "/opt/X.app/bin/x", "/opt/X.app/bin/x"},
		// Never a top folder, nor one that holds the home folder.
		{"darwin", "/Users/u", "", "/X.app/Contents/MacOS/x", "/X.app/Contents/MacOS/x"},
		{"linux", "/opt/homebrew/Cellar/x/1/home", "", "/opt/homebrew/Cellar/x/1/bin/x", "/opt/homebrew/Cellar/x/1/bin/x"},
		{"linux", "/home/u", "", "/node_modules/x/y", "/node_modules/x"},
	} {
		r := &resolver{goos: c.goos, home: c.home, devRoot: c.dev}
		if got := r.rootOf(c.real); got != c.want {
			t.Errorf("%s on %s: %s, want %s", c.real, c.goos, got, c.want)
		}
	}
}

func TestAScriptsInterpreterAndWhatItsLauncherNamesAreDenied(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t, ProgramName{Name: "codex", Why: "a stand-in"}, ProgramName{Name: "opencode", Why: "a stand-in"})
	// npm's global layout: bin/codex is a link to the package's script, run by the node `env` finds.
	s.program("brew/lib/node_modules/@openai/codex/bin/codex.js", "#!/usr/bin/env -S node --no-warnings\nimport './x.js'\n")
	s.link("brew/bin/codex", "../lib/node_modules/@openai/codex/bin/codex.js")
	s.program("brew/Cellar/node/22.1/bin/node", "#!/bin/sh\n")
	s.link("brew/bin/node", "../Cellar/node/22.1/bin/node")
	// pnpm's: a launcher script that runs `$basedir/node` if there is one, else node by name, on the package's file.
	s.program("pnpm/global/5/node_modules/opencode-ai/bin/opencode", "#!/usr/bin/env node\n")
	s.program("pnpm/opencode", `#!/bin/sh
basedir=$(dirname "$(echo "$0" | sed -e 's,\\,/,g')")
if [ -x "$basedir/node" ]; then
  exec "$basedir/node"  "$basedir/global/5/node_modules/opencode-ai/bin/opencode" "$@"
else
  exec node  "$basedir/global/5/node_modules/opencode-ai/bin/opencode" "$@"
fi
`)
	s.dir("early")
	ps := mustResolve(t, Finding{GOOS: "linux", Home: s.home, Search: []string{s.at("early"), s.at("pnpm"), s.at("brew/bin")}})
	denied(t, ps, "linux", map[string]bool{
		s.at("brew/lib/node_modules/@openai/codex/x.js"): true, s.at("brew/bin/codex"): true, s.at("early/codex"): true,
		s.at("brew/bin/node"): true, s.at("brew/Cellar/node/22.1/lib/planted.js"): true, s.at("early/node"): true, s.at("pnpm/node"): true,
		s.at("pnpm/opencode"): true, s.at("pnpm/global/5/node_modules/opencode-ai/lib/planted.js"): true,
		"/usr/bin/env": true, "/bin/sh": true,
		s.at("brew/lib/node_modules/other"): false, s.at("pnpm/global/5/node_modules/other"): false, s.at("brew/bin/npm"): false,
	})
}

// Ubuntu's merged /usr (FXSAL1VM1 B1, VM run 1): /bin is a link to usr/bin at the top of the volume, which no desk
// writes, so it isn't a hop denied, and /usr/bin/sh is a link to dash. A script run by /bin/sh has its interpreter
// denied as spelled, and where it leads: /usr/bin/sh and /usr/bin/dash. A temp folder stands in for the top of the
// volume, so this runs on any computer; on either system, and whether dash was met first by its own name.
func TestOnAMergedUsrAnInterpreterIsDeniedAsSpelledAndWhereItLeads(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		for _, first := range []string{"", "usr/bin/dash", "usr/bin/sh"} {
			s := newStandins(t)
			volume := s.dir("volume")
			s.program("volume/usr/bin/dash", "a shell's stand-in\n")
			s.link("volume/usr/bin/sh", "dash")
			s.link("volume/bin", "usr/bin")
			s.dir("volume/usr/bin/other")
			script := s.program("volume/opt/tool/bin/tool", "#!"+filepath.Join(volume, "bin", "sh")+"\n")
			r := &resolver{goos: goos, home: s.home, volume: volume, budget: maxProgramFiles, seen: map[string]bool{}}
			if first != "" {
				if err := r.hold(s.at("volume/" + first)); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.hold(script); err != nil {
				t.Fatal(err)
			}
			met := "only through the script"
			if first != "" {
				met = "first as " + first
			}
			t.Run(goos+", "+met, func(t *testing.T) {
				denied(t, &Programs{roots: outermost(goos, r.denied)}, goos, map[string]bool{
					s.at("volume/bin/sh"): true, s.at("volume/usr/bin/sh"): true, s.at("volume/usr/bin/dash"): true, script: true,
					// The link at the top itself, and what else is in the folder it leads to, are left as they were.
					s.at("volume/bin"): false, s.at("volume/bin/other"): false, s.at("volume/usr/bin/other"): false,
				})
			})
		}
	}
}

func TestEnvsProgramIsTheFirstWordThatIsntAnOption(t *testing.T) {
	for line, want := range map[string]string{"node": "node", "-S node --x": "node", "-u HOME bun": "bun", "A=1 -i python3": "python3", "-": ""} {
		if got := envProgram(strings.Fields(line)); got != want {
			t.Errorf("env %s: %q, want %q", line, got, want)
		}
	}
}

// machO is a thin 64-bit Mach-O with only these load commands: each a library (cmd and name) or an LC_RPATH.
func machO(t *testing.T, loads ...[2]any) []byte {
	var cmds bytes.Buffer
	for _, l := range loads {
		cmd, name := l[0].(uint32), l[1].(string)
		head := 24 // a dylib_command
		if cmd == lcRpath {
			head = 12
		}
		size := (head + len(name) + 1 + 7) &^ 7
		body := make([]byte, size)
		binary.LittleEndian.PutUint32(body[0:], cmd)
		binary.LittleEndian.PutUint32(body[4:], uint32(size))
		binary.LittleEndian.PutUint32(body[8:], uint32(head))
		copy(body[head:], name)
		cmds.Write(body)
	}
	var out bytes.Buffer
	for _, v := range []uint32{0xfeedfacf, 0x0100000c, 0, 2, uint32(len(loads)), uint32(cmds.Len()), 0, 0} {
		binary.Write(&out, binary.LittleEndian, v)
	}
	out.Write(cmds.Bytes())
	return out.Bytes()
}

func TestAMachOFilesLibrariesAreDeniedThroughEveryLoadCommandAndRpath(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t, ProgramName{Name: "tool", Why: "a stand-in"})
	s.dir("rp1")
	tool := machO(t,
		[2]any{uint32(lcLoadDylib), "/usr/lib/libSystem.B.dylib"},
		[2]any{uint32(lcLoadDylib), s.at("brew/opt/dep/lib/libdep.dylib")},
		[2]any{uint32(lcLoadWeakDylib), "@rpath/libweak.dylib"},
		[2]any{uint32(lcReexport), "@loader_path/../Frameworks/libre.dylib"},
		[2]any{uint32(lcRpath), "relative/rp"},
		[2]any{uint32(lcRpath), s.at("rp1")},
		[2]any{uint32(lcRpath), s.at("rp2")},
	)
	if err := os.MkdirAll(s.at("brew/Cellar/tool/1/bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.at("brew/Cellar/tool/1/bin/tool"), tool, 0o700); err != nil {
		t.Fatal(err)
	}
	// The library comes through Homebrew's opt link, and itself loads one from a folder that isn't there.
	if err := os.MkdirAll(s.at("brew/Cellar/dep/2/lib"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.at("brew/Cellar/dep/2/lib/libdep.dylib"), machO(t, [2]any{uint32(lcLoadDylib), s.at("deep/lib/libdeep.dylib")}), 0o600); err != nil {
		t.Fatal(err)
	}
	s.link("brew/opt/dep", "../Cellar/dep/2")
	s.program("rp2/libweak.dylib", "not loaded here")
	ps := mustResolve(t, Finding{GOOS: "darwin", Home: s.home, Search: []string{s.at("brew/bin")}, DeveloperDir: s.at("no-developer-tools")})
	if ps.Path("tool") != "" {
		t.Errorf("tool was found on the search: %q", ps.Path("tool"))
	}
	ps = mustResolve(t, Finding{GOOS: "darwin", Home: s.home, Also: []Program{{Name: "tool", Path: s.at("brew/Cellar/tool/1/bin/tool")}}, DeveloperDir: s.at("no-developer-tools")})
	denied(t, ps, "darwin", map[string]bool{
		s.at("brew/Cellar/tool/1/Frameworks/libre.dylib"): true, s.at("brew/opt/dep"): true, s.at("brew/Cellar/dep/2/lib/libdep.dylib"): true,
		// @rpath: the first folder that has it is taken, and every one before it held.
		s.at("rp1/libweak.dylib"): true, s.at("rp2/libweak.dylib"): true, s.at("deep"): true,
		"/usr/lib/libSystem.B.dylib": false, s.at("rp1/other.dylib"): false, s.at("rp2/other.dylib"): false, s.at("brew/opt/other"): false,
		// One that is nowhere is looked for by dyld's fallback, by its name in /usr/local/lib (N5).
		"/usr/local/lib/libdeep.dylib": true,
	})
	if info, err := os.Stat("/usr/local/lib"); err == nil && info.IsDir() {
		denied(t, ps, "darwin", map[string]bool{"/usr/local/lib/libweak.dylib": false, "/usr/local/lib/libdep.dylib": false})
	}
}

// RT126SAX2 N5's probe: a library two programs load, which loads one by @executable_path, is walked for each of them:
// each program's own folder is the one dyld fills in.
func TestALibraryTwoProgramsLoadIsWalkedWithEachOnesExecutablePath(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t)
	write := func(rel string, data []byte) string {
		if err := os.MkdirAll(filepath.Dir(s.at(rel)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(s.at(rel), data, 0o700); err != nil {
			t.Fatal(err)
		}
		return s.at(rel)
	}
	write("shared/libshared.dylib", machO(t, [2]any{uint32(lcLoadDylib), "@executable_path/../lib/libx.dylib"}))
	a := write("a/bin/a", machO(t, [2]any{uint32(lcLoadDylib), s.at("shared/libshared.dylib")}))
	b := write("b/bin/b", machO(t, [2]any{uint32(lcLoadDylib), s.at("shared/libshared.dylib")}))
	ps := mustResolve(t, Finding{GOOS: "darwin", Home: s.home, Also: []Program{{Name: "a", Path: a}, {Name: "b", Path: b}}, DeveloperDir: s.at("no-developer-tools")})
	denied(t, ps, "darwin", map[string]bool{s.at("a/lib/libx.dylib"): true, s.at("b/lib/libx.dylib"): true, s.at("shared/libshared.dylib"): true,
		s.at("shared/lib/libx.dylib"): false, s.at("c/lib/libx.dylib"): false})
}

// RT126SAX2 B1, on stand-ins: a program that loads libxcselect (/usr/bin's developer tools, which hand on through
// xcrun) has xcrun's cache denied where the core's xcrun keeps it: xcrun_db in its temp folder, or the whole path its
// environment's xcrun_db names. A relative xcrun_db is ignored, as libxcrun ignores it.
func TestAnXcrunToolsCacheIsDeniedWhereTheCoresXcrunKeepsIt(t *testing.T) {
	s := newStandins(t)
	shim := machO(t, [2]any{uint32(lcLoadDylib), "/usr/lib/libxcselect.dylib"}, [2]any{uint32(lcLoadDylib), "/usr/lib/libSystem.B.dylib"})
	plain := machO(t, [2]any{uint32(lcLoadDylib), "/usr/lib/libSystem.B.dylib"})
	for rel, data := range map[string][]byte{"xc/git": shim, "xc/env": plain} {
		if err := os.MkdirAll(s.at("xc"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(s.at(rel), data, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	temp := s.dir("T")
	// A cache a desk made a link before the core's start: the link, and where it leads, are both denied.
	s.dir("elsewhere")
	s.link("T2/xcrun_db", s.at("elsewhere/db"))
	git := ProgramName{Name: "git", Places: []string{s.at("xc/git")}, Why: "a stand-in"}
	onlyPrograms(t, git)
	f := Finding{GOOS: "darwin", Home: s.home, UserTemp: temp, DeveloperDir: s.at("no-developer-tools")}
	ps := mustResolve(t, f)
	denied(t, ps, "darwin", map[string]bool{filepath.Join(temp, "xcrun_db"): true, filepath.Join(temp, "xcrun_db-tmp"): false, temp: false, s.at("T/other"): false})

	f.XcrunDB = s.dir("own") + "/xcrun_db"
	denied(t, mustResolve(t, f), "darwin", map[string]bool{s.at("own/xcrun_db"): true, filepath.Join(temp, "xcrun_db"): true, s.at("own/other"): false})
	f.XcrunDB, f.UserTemp = "relative/xcrun_db", s.at("T2")
	denied(t, mustResolve(t, f), "darwin", map[string]bool{s.at("T2/xcrun_db"): true, s.at("elsewhere/db"): true, s.at("elsewhere/other"): false})

	// With no place for it, the start fails rather than leave it open.
	f.XcrunDB, f.UserTemp = "relative/xcrun_db", ""
	if _, err := ResolvePrograms(f); !errors.Is(err, ErrPrograms) {
		t.Errorf("an xcrun tool with no whole temp folder: %v", err)
	}
	// A program that isn't one needs none, and denies none; off a Mac, nothing hands on through xcrun.
	onlyPrograms(t, ProgramName{Name: "env", Places: []string{s.at("xc/env")}, Why: "a stand-in"})
	f.UserTemp = temp
	denied(t, mustResolve(t, f), "darwin", map[string]bool{filepath.Join(temp, "xcrun_db"): false})
	onlyPrograms(t, git)
	denied(t, mustResolve(t, Finding{GOOS: "linux", Home: s.home}), "linux", map[string]bool{filepath.Join(temp, "xcrun_db"): false})

	// The reviewer's probe, through a desk's Policy: the cache is denied by the row for programs that run outside.
	onlyPrograms(t, git)
	f.XcrunDB = ""
	p := mustNew(t, Places{GOOS: "darwin", Home: s.home, Support: filepath.Join(s.home, "L", "AD"), OwnID: "h1",
		Root: filepath.Join(s.home, "P", "app"), InstallRoots: mustResolve(t, f).InstallRoots()})
	if v, r := p.Decide(filepath.Join(temp, "xcrun_db"), Write); v != Deny || r.Row != "programs that run outside" {
		t.Errorf("a desk writing the xcrun cache: %v by %q", v, r.Row)
	}
	if v, _ := p.Decide(filepath.Join(temp, "other"), Write); v != Allow {
		t.Errorf("a desk writing beside it: %v", v)
	}
}

// elfFile is a little-endian 64-bit ELF with a loader and a dynamic section that needs libs through runpath.
func elfFile(t *testing.T, interp string, needed []string, runpath string) []byte {
	strs := []byte{0}
	var dyns []elf.Dyn64
	for _, lib := range needed {
		dyns = append(dyns, elf.Dyn64{Tag: int64(elf.DT_NEEDED), Val: uint64(len(strs))})
		strs = append(append(strs, lib...), 0)
	}
	dyns = append(dyns, elf.Dyn64{Tag: int64(elf.DT_RUNPATH), Val: uint64(len(strs))}, elf.Dyn64{})
	strs = append(append(strs, runpath...), 0)
	shstr := []byte("\x00.dynstr\x00.dynamic\x00.shstrtab\x00")
	var dyn bytes.Buffer
	binary.Write(&dyn, binary.LittleEndian, dyns)
	interpAt := uint64(64 + 56)
	strsAt := interpAt + uint64(len(interp)+1)
	dynAt := strsAt + uint64(len(strs))
	shstrAt := dynAt + uint64(dyn.Len())
	shAt := shstrAt + uint64(len(shstr))
	var out bytes.Buffer
	h := elf.Header64{Type: uint16(elf.ET_DYN), Machine: uint16(elf.EM_X86_64), Version: 1, Phoff: 64, Shoff: shAt, Ehsize: 64,
		Phentsize: 56, Phnum: 1, Shentsize: 64, Shnum: 4, Shstrndx: 3}
	copy(h.Ident[:], []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	binary.Write(&out, binary.LittleEndian, h)
	binary.Write(&out, binary.LittleEndian, elf.Prog64{Type: uint32(elf.PT_INTERP), Flags: 4, Off: interpAt, Filesz: uint64(len(interp) + 1), Memsz: uint64(len(interp) + 1), Align: 1})
	out.WriteString(interp + "\x00")
	out.Write(strs)
	out.Write(dyn.Bytes())
	out.Write(shstr)
	binary.Write(&out, binary.LittleEndian, []elf.Section64{{},
		{Name: 1, Type: uint32(elf.SHT_STRTAB), Off: strsAt, Size: uint64(len(strs)), Addralign: 1},
		{Name: 9, Type: uint32(elf.SHT_DYNAMIC), Off: dynAt, Size: uint64(dyn.Len()), Link: 1, Entsize: 16, Addralign: 8},
		{Name: 18, Type: uint32(elf.SHT_STRTAB), Off: shstrAt, Size: uint64(len(shstr)), Addralign: 1}})
	return out.Bytes()
}

func TestAnELFFilesLoaderAndLibrariesAreDeniedThroughItsRunpath(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t, ProgramName{Name: "t", Why: "a stand-in"})
	s.program("lb/lib/ld.so", "a loader")
	s.program("lb/lib/libz.so.1", "a library")
	s.dir("first")
	file := elfFile(t, s.at("lb/lib/ld.so"), []string{"libz.so.1", "libc.so.6"}, s.at("first")+":$ORIGIN/../../../../lib")
	if err := os.MkdirAll(s.at("lb/Cellar/t/1/bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.at("lb/Cellar/t/1/bin/t"), file, 0o700); err != nil {
		t.Fatal(err)
	}
	s.link("lb/bin/t", "../Cellar/t/1/bin/t")
	ps := mustResolve(t, Finding{GOOS: "linux", Home: s.home, Search: []string{s.at("lb/bin")}})
	denied(t, ps, "linux", map[string]bool{
		s.at("lb/lib/ld.so"): true, s.at("lb/lib/libz.so.1"): true, s.at("first/libz.so.1"): true,
		// One the runpath doesn't have is the system's, from its own folders, after it: held in each runpath folder.
		s.at("first/libc.so.6"): true, s.at("lb/lib/libc.so.6"): true,
		s.at("lb/lib/other.so"): false, s.at("first/other.so"): false,
	})
}

func TestSmolcorpsInstallIsDeniedAndItsTmuxTakenFirst(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t, ProgramName{Name: "tmux", Places: nil, Why: "a stand-in"})
	app := s.program("Applications/Smolcorp.app/Contents/MacOS/smolcorp", "#!/bin/sh\n")
	s.program("Applications/Smolcorp.app/Contents/MacOS/tmux", "#!/bin/sh\n")
	s.program("brew/bin/tmux", "#!/bin/sh\n")
	ps := mustResolve(t, Finding{GOOS: "darwin", Home: s.home, Executable: app, Search: []string{s.at("brew/bin")}, DeveloperDir: s.at("no-developer-tools")})
	if ps.Path("tmux") != s.at("Applications/Smolcorp.app/Contents/MacOS/tmux") || ps.Path("smolcorp") != app {
		t.Errorf("tmux is %q, Smolcorp %q", ps.Path("tmux"), ps.Path("smolcorp"))
	}
	denied(t, ps, "darwin", map[string]bool{s.at("Applications/Smolcorp.app/Contents/Resources/planted"): true, s.at("brew/bin/tmux"): true, s.at("Applications/Other.app"): false})

	// On Linux: the program, linked into ~/.local/bin, and the folder its tmux comes in, there or not.
	l := newStandins(t)
	real := l.program("dist/smolcorp", "#!/bin/sh\n")
	l.link("home/.local/bin/smolcorp", real)
	ps = mustResolve(t, Finding{GOOS: "linux", Home: l.home, Executable: filepath.Join(l.home, ".local", "bin", "smolcorp")})
	denied(t, ps, "linux", map[string]bool{real: true, filepath.Join(l.home, ".local", "bin", "smolcorp"): true,
		filepath.Join(l.home, ".local", "lib", "smolcorp", "tmux"): true, filepath.Join(l.home, ".local", "bin", "other"): false, l.at("dist/other"): false})
	if ps.Path("tmux") != "" {
		t.Errorf("a tmux that isn't there: %q", ps.Path("tmux"))
	}
}

func TestNoDeniedPlaceIsATopFolderOrHoldsTheHomeFolder(t *testing.T) {
	// The temp folder unresolved: on a Mac it is under /var, a link at the top.
	base := t.TempDir()
	home := filepath.Join(base, "home")
	onlyPrograms(t, ProgramName{Name: "gh", Places: []string{"/nowhere-at-all/gh"}, Why: "a stand-in"})
	if err := os.MkdirAll(filepath.Join(home, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	ps := mustResolve(t, Finding{GOOS: runtime.GOOS, Home: home, Search: []string{"/", base, home, filepath.Join(home, "bin")}})
	for _, root := range ps.InstallRoots() {
		if topFolder(root) || under(key(runtime.GOOS, home), key(runtime.GOOS, root)) {
			t.Errorf("%s is denied", root)
		}
	}
	if !slices.Contains(ps.InstallRoots(), filepath.Join(home, "bin", "gh")) {
		t.Errorf("the home folder's own place isn't: %q", ps.InstallRoots())
	}
}

func TestAProgramThatCantBeToldWholeFailsTheStart(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t, ProgramName{Name: "gh", Why: "a stand-in"})
	s.link("bin/gh", "gh2")
	s.link("bin/gh2", "gh")
	if _, err := ResolvePrograms(Finding{GOOS: "linux", Home: s.home, Search: []string{s.at("bin")}}); !errors.Is(err, ErrPrograms) {
		t.Errorf("a link loop: %v", err)
	}
	s.program("broken/gh", "\x7fELF not really")
	// A universal Mach-O that can't be read fails too; a Java class, with the same first four bytes, loads nothing (N5).
	s.program("fat/gh", "\xca\xfe\xba\xbe\x00\x00\x00\x02 cut short")
	if _, err := ResolvePrograms(Finding{GOOS: "darwin", Home: s.home, Search: []string{s.at("fat")}, DeveloperDir: s.at("none")}); !errors.Is(err, ErrPrograms) {
		t.Errorf("a universal Mach-O that can't be read: %v", err)
	}
	s.program("java/gh", "\xca\xfe\xba\xbe\x00\x00\x00\x41 a class file")
	if ps, err := ResolvePrograms(Finding{GOOS: "darwin", Home: s.home, Search: []string{s.at("java")}, DeveloperDir: s.at("none")}); err != nil {
		t.Errorf("a Java class: %v", err)
	} else {
		denied(t, ps, "darwin", map[string]bool{s.at("java/gh"): true})
	}
	if _, err := ResolvePrograms(Finding{GOOS: "linux", Home: s.home, Search: []string{s.at("broken")}}); !errors.Is(err, ErrPrograms) {
		t.Errorf("an ELF file that can't be read: %v", err)
	}
	if _, err := ResolvePrograms(Finding{GOOS: "linux", Home: "home"}); !errors.Is(err, ErrNotWhole) {
		t.Errorf("a home that isn't whole: %v", err)
	}
	if _, err := ResolvePrograms(Finding{GOOS: "linux", Home: s.home, Also: []Program{{Name: "tmux", Path: "tmux"}}}); !errors.Is(err, ErrNotWhole) {
		t.Errorf("a recorded tmux that isn't whole: %v", err)
	}
}

func TestEveryProgramOfTheRowIsOnTheListOncePerSystem(t *testing.T) {
	for goos, want := range map[string][]string{
		"darwin":  {"git", "gh", "tmux", "claude", "codex", "gemini", "opencode", "grok", "cursor-agent", "agent", "kitten", "wezterm", "swift", "xcodebuild"},
		"linux":   {"git", "gh", "tmux", "pactl", "wpctl", "xdg-open", "claude", "codex", "gemini", "opencode", "grok", "cursor-agent", "agent"},
		"windows": {"git", "gh", "claude", "codex", "gemini", "opencode", "grok", "cursor-agent", "agent"},
	} {
		var got []string
		for _, n := range ProgramNames(goos) {
			if n.Why == "" {
				t.Errorf("%s on %s says nothing of who runs it", n.Name, goos)
			}
			for _, place := range n.Places {
				if !filepath.IsAbs(place) || filepath.Base(place) != n.Name {
					t.Errorf("%s on %s: the place %s", n.Name, goos, place)
				}
			}
			got = append(got, n.Name)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", goos, got, want)
		}
	}
	// RT126SAX2 N3: the places the Go finders look in (findGH's ghPlaces; findTmux's tmuxPlaces, on Linux too).
	gh := []string{"/opt/homebrew/bin", "/usr/local/bin", "/opt/local/bin", "/usr/bin", "/home/linuxbrew/.linuxbrew/bin", "/snap/bin"}
	tmux := gh[:4]
	for _, goos := range []string{"darwin", "linux"} {
		for _, n := range ProgramNames(goos) {
			var folders []string
			for _, place := range n.Places {
				folders = append(folders, filepath.Dir(place))
			}
			switch {
			case n.Name == "gh" && !slices.Equal(folders, gh):
				t.Errorf("gh on %s: %q, want findGH's %q", goos, folders, gh)
			case n.Name == "tmux" && !slices.Equal(folders[:min(len(folders), 4)], tmux):
				t.Errorf("tmux on %s: %q, want findTmux's %q first", goos, folders, tmux)
			}
		}
	}
}

func TestARecordedTmuxIsDeniedAndOneThatsGoneIsHeld(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t)
	old := s.program("brew/Cellar/tmux/3.4/bin/tmux", "#!/bin/sh\n")
	ps := mustResolve(t, Finding{GOOS: "linux", Home: s.home, Also: []Program{{Name: "tmux", Path: old}, {Name: "tmux", Path: s.at("gone/bin/tmux")}}})
	denied(t, ps, "linux", map[string]bool{s.at("brew/Cellar/tmux/3.5/bin/tmux"): true, s.at("gone"): true, s.at("brew/Cellar/other"): false})
}

func TestAPlaceAMacSealsIsLeftOutAndNoOtherIs(t *testing.T) {
	r := &resolver{goos: "darwin", home: "/Users/u"}
	for _, p := range []string{"/usr/bin/codex", "/USR/BIN/codex", "/System/Cryptexes/App/usr/bin/codex", "/bin/codex", "/sbin/x", "/Library/Apple/usr/bin/x"} {
		r.deny(p)
	}
	kept := []string{"/usr/local/bin/codex", "/System/Volumes/Data/opt/homebrew/bin/x", "/Library/Developer/CommandLineTools", "/opt/homebrew/bin/x", "/usr-local/x"}
	for _, p := range kept {
		r.deny(p)
	}
	if !slices.Equal(r.denied, kept) {
		t.Errorf("denied %q, want %q", r.denied, kept)
	}
	// Off a Mac, nothing is sealed.
	l := &resolver{goos: "linux", home: "/home/u"}
	l.deny("/usr/bin/codex")
	if len(l.denied) != 1 {
		t.Errorf("on Linux: %q", l.denied)
	}
}
