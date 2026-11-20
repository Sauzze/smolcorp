package sandbox

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// SA-X2 on this Mac, with stand-ins only (owner Q5 (a)): the programs resolved for a stand-in hire are written into its
// real profile, and the stand-in, under /usr/bin/sandbox-exec, tries to plant programs where Smolcorp would run them: in
// a Homebrew keg, over its link, in an npm package, first on the search, in Smolcorp's app. Each check is only "ok" or
// the error's name. The "ok" rows show the same desk still installs and works beside them.

func sandboxExecAndPython(t *testing.T) (string, string) {
	sandboxExec, python := "/usr/bin/sandbox-exec", python3(t)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v) or python3 (%q) missing", err, python)
		}
		t.Skip("no sandbox-exec or python3 here")
	}
	return sandboxExec, python
}

func TestOnAMacAProgramPlantedUnderAnInstallRootIsRefused(t *testing.T) {
	sandboxExec, python := sandboxExecAndPython(t)
	s := newStandins(t)
	onlyPrograms(t, ProgramName{Name: "gh", Why: "a stand-in"}, ProgramName{Name: "codex", Why: "a stand-in"})
	s.program("brew/Cellar/gh/2.0/bin/gh", "#!/bin/sh\n")
	s.link("brew/bin/gh", "../Cellar/gh/2.0/bin/gh")
	s.program("brew/lib/node_modules/@openai/codex/bin/codex.js", "#!/usr/bin/env node\n")
	s.link("brew/bin/codex", "../lib/node_modules/@openai/codex/bin/codex.js")
	s.program("brew/Cellar/node/22.1/bin/node", "#!/bin/sh\n")
	s.link("brew/bin/node", "../Cellar/node/22.1/bin/node")
	app := s.program("Applications/Smolcorp.app/Contents/MacOS/smolcorp", "#!/bin/sh\n")
	early, elsewhere := s.dir("early"), s.dir("elsewhere")
	support := filepath.Join(s.home, "L", "AD")
	own := s.dir("home/L/AD/hires/h1")
	root := s.dir("home/P/app")
	ps := mustResolve(t, Finding{GOOS: "darwin", Home: s.home, Search: []string{filepath.Join(s.home, ".local", "bin"), early, s.at("brew/bin")},
		Executable: app, DeveloperDir: s.at("no-developer-tools")})
	p := mustNew(t, Places{GOOS: "darwin", Home: s.home, Support: support, OwnID: "h1", Root: root, InstallRoots: ps.InstallRoots()})
	file := filepath.Join(own, "sandbox.sb")
	if err := PrepareProfile(p, file); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(s.home, ".local", "bin")
	if info, err := os.Lstat(local); err != nil || !info.IsDir() {
		t.Fatalf("the Hold folder %s wasn't made: %v", local, err)
	}

	type step struct{ what, at, to, want string }
	const ok, no = "ok", "PermissionError"
	keg, codex := s.at("brew/Cellar/gh/2.0/bin"), s.at("brew/lib/node_modules/@openai/codex")
	steps := []step{
		// In the keg, over its link, the keg moved away; in the npm package and its interpreter's keg; in the app.
		{"write", keg + "/gh-planted", "", no}, {"write", keg + "/gh", "", no}, {"unlink", s.at("brew/bin/gh"), "", no},
		{"symlink", s.at("brew/bin/gh"), elsewhere, no}, {"rename", s.at("brew/Cellar/gh"), elsewhere + "/gh", no},
		{"write", codex + "/bin/planted.js", "", no}, {"write", codex + "/package.json", "", no},
		{"write", s.at("brew/Cellar/node/22.1/bin/planted.js"), "", no}, {"unlink", s.at("brew/bin/node"), "", no},
		{"mkdir", s.at("Applications/Smolcorp.app/Contents/Resources"), "", no}, {"write", app, "", no},
		// First on the search: the same name, for the program and the interpreter `env` finds.
		{"write", early + "/gh", "", no}, {"write", early + "/node", "", no}, {"write", local + "/codex", "", no},
		// Beside them, installing and working go on.
		{"write", s.at("brew/bin/jq"), "", ok}, {"symlink", s.at("brew/bin/jq2"), elsewhere, ok}, {"write", early + "/jq", "", ok},
		{"write", local + "/jq", "", ok}, {"write", s.at("brew/lib/node_modules/other.js"), "", ok},
		{"mkdir", s.at("Applications/Other.app"), "", ok}, {"write", root + "/main.go", "", ok},
	}
	var args [][]string
	for _, st := range steps {
		arg := []string{st.what, st.at}
		if st.to != "" {
			arg = append(arg, st.to)
		}
		args = append(args, arg)
	}
	encoded, _ := json.Marshal(args)
	cmd := disclaim.Cmd(exec.Command(sandboxExec, "-f", file, python, "-I", "-c", sbProbe, string(encoded)))
	cmd.Dir, cmd.Env = s.base, []string{"HOME=" + s.home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("the stand-in didn't run under the profile: %v %s", err, stderr.Bytes())
	}
	var said []string
	if err := json.Unmarshal(out.Bytes(), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %q (%v)", out.Bytes(), err)
	}
	for i, st := range steps {
		if said[i] != st.want {
			t.Errorf("%s %s %s: %s, want %s", st.what, strings.TrimPrefix(st.at, s.base), strings.TrimPrefix(st.to, s.base), said[i], st.want)
		}
	}
}

func TestOnAMacUsrBinsToolsAreDeniedWithTheDeveloperFolderTheyHandOnTo(t *testing.T) {
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("no /usr/bin/git here")
	}
	s := newStandins(t)
	onlyPrograms(t, ProgramName{Name: "git", Places: []string{"/usr/bin/git"}, Why: "a stand-in"})
	s.program("Xcode.app/Contents/Developer/usr/bin/git", "#!/bin/sh\n")
	temp := s.dir("T")
	ps := mustResolve(t, Finding{GOOS: "darwin", Home: s.home, DeveloperDir: s.at("Xcode.app/Contents/Developer"), UserTemp: temp})
	if ps.Path("git") != "/usr/bin/git" {
		t.Errorf("git is %q", ps.Path("git"))
	}
	// /usr/bin/git itself is sealed (SIP): no desk writes it, so no row is spent on it. xcrun's cache, which it reads
	// the developer folder's git from, isn't (RT126SAX2 B1).
	denied(t, ps, "darwin", map[string]bool{s.at("Xcode.app/Contents/Developer/usr/libexec/git-core/planted"): true, "/usr/bin/git": false,
		s.at("Other.app"): false, filepath.Join(temp, "xcrun_db"): true, filepath.Join(temp, "other"): false})
	if _, err := ResolvePrograms(Finding{GOOS: "darwin", Home: s.home, DeveloperDir: s.at("Xcode.app/Contents/Developer")}); err == nil {
		t.Error("the real /usr/bin/git resolved with no place for xcrun's cache")
	}
	// The developer folder named through a link (an Xcode.app that leads to Xcode-16.app, as the Xcodes app keeps
	// them): the link is denied as well as where it leads, so no desk points it at a folder of its own
	// (RT126RESOLVESWEEP S2c).
	onlyPrograms(t, ProgramName{Name: "git", Places: []string{"/usr/bin/git"}, Why: "a stand-in"})
	s.program("Xcode-16.app/Contents/Developer/usr/bin/git", "#!/bin/sh\n")
	if err := os.Symlink("Xcode-16.app", s.at("Linked.app")); err != nil {
		t.Fatal(err)
	}
	ps = mustResolve(t, Finding{GOOS: "darwin", Home: s.home, DeveloperDir: s.at("Linked.app/Contents/Developer"), UserTemp: temp})
	denied(t, ps, "darwin", map[string]bool{s.at("Linked.app"): true, s.at("Xcode-16.app/Contents/Developer/usr/bin/git"): true})
	if root, hops, err := developerPlace(s.at("Linked.app/Contents/Developer")); err != nil || root != s.at("Xcode-16.app") || len(hops) != 1 {
		t.Errorf("the developer folder through a link: %q %q %v", root, hops, err)
	}
	// /usr/bin/env is no developer tool: it needs no cache.
	onlyPrograms(t, ProgramName{Name: "env", Places: []string{"/usr/bin/env"}, Why: "a stand-in"})
	mustResolve(t, Finding{GOOS: "darwin", Home: s.home, DeveloperDir: s.at("Xcode.app/Contents/Developer")})

	// With no DEVELOPER_DIR, xcode-select's choice, read from its link: Xcode's app whole, or the tools' folder.
	if root := developerRoot(""); !filepath.IsAbs(root) || strings.Contains(strings.ToLower(root), ".app/") {
		t.Errorf("the developer folder's root: %q", root)
	}
}

func TestOnAMacUserLookupNeedsItsMachServiceAndTheListIsAProfilesOwn(t *testing.T) {
	sandboxExec, _ := sandboxExecAndPython(t)
	profile := func(leave string) string {
		var s strings.Builder
		s.WriteString("(version 1)\n(allow default)\n(deny mach-lookup)\n(allow mach-lookup")
		for _, m := range MachServices() {
			if m.Name != leave {
				s.WriteString("\n  (global-name " + sbString(m.Name) + ")")
			}
		}
		s.WriteString(")\n(deny iokit-open)\n(allow iokit-open")
		for _, c := range IOKitClasses() {
			kind := "iokit-registry-entry-class"
			if c.UserClient {
				kind = "iokit-user-client-class"
			}
			s.WriteString("\n  (" + kind + " " + sbString(c.Class) + ")")
		}
		s.WriteString(")\n")
		return s.String()
	}
	whoami := func(p string) string {
		cmd := disclaim.Cmd(exec.Command(sandboxExec, "-p", p, "/usr/bin/whoami"))
		cmd.Env = []string{"PATH=/usr/bin:/bin"}
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("whoami under the lists: %v", err)
		}
		return strings.TrimSpace(string(out))
	}
	uid := strconv.Itoa(os.Getuid())
	if got := whoami(profile("")); got == uid || got == "" {
		t.Errorf("with the whole list, whoami is %q", got)
	}
	// S4: without user lookup, whoami gives the bare uid.
	if got := whoami(profile("com.apple.system.opendirectoryd.libinfo")); got != uid {
		t.Errorf("without user lookup, whoami is %q, want %s", got, uid)
	}
}

// RT126SAX2 B1 on this Mac: /usr/bin/git reads the program it hands on to from xcrun's cache, at the whole path the
// environment's xcrun_db names (the reviewer's probe: xcrun_verbose=1 says where it looked, and what it found). A
// stand-in hire under the profile made from a stand-in temp folder can't write, replace, remove or move that cache,
// or the folder it is in; it writes beside it, and its own /usr/bin/git still runs, reading the cache.
func TestOnAMacTheXcrunCacheUsrBinGitReadsIsRefusedToADesk(t *testing.T) {
	sandboxExec, python := sandboxExecAndPython(t)
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("no /usr/bin/git here")
	}
	s := newStandins(t)
	temp := s.dir("T")
	cache := filepath.Join(temp, "xcrun_db")
	git := func(profile string) (string, string, error) {
		args := []string{"/usr/bin/git", "--version"}
		if profile != "" {
			args = append([]string{sandboxExec, "-f", profile}, args...)
		}
		cmd := disclaim.Cmd(exec.Command(args[0], args[1:]...))
		cmd.Dir, cmd.Env = s.base, []string{"HOME=" + s.home, "PATH=/usr/bin:/bin", "xcrun_db=" + cache, "xcrun_verbose=1"}
		var out, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &stderr
		err := cmd.Run()
		return out.String(), stderr.String(), err
	}
	out, said, err := git("")
	if err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("/usr/bin/git: %v %s", err, said)
		}
		t.Skipf("/usr/bin/git doesn't run here (no developer tools?): %v", err)
	}
	if !strings.HasPrefix(out, "git version") || !strings.Contains(said, "xcrun_db = '"+cache+"'") {
		t.Fatalf("/usr/bin/git didn't take its cache from xcrun_db: %q %q", out, said)
	}
	written, err := os.ReadFile(cache)
	if err != nil {
		t.Fatalf("xcrun kept no cache at %s: %v", cache, err)
	}

	onlyPrograms(t, ProgramName{Name: "git", Places: []string{"/usr/bin/git"}, Why: "a stand-in"})
	ps := mustResolve(t, Finding{GOOS: "darwin", Home: s.home, UserTemp: temp})
	own := s.dir("home/L/AD/hires/h1")
	p := mustNew(t, Places{GOOS: "darwin", Home: s.home, Support: filepath.Join(s.home, "L", "AD"), OwnID: "h1", Root: s.dir("home/P/app"), InstallRoots: ps.InstallRoots()})
	profile := filepath.Join(own, "sandbox.sb")
	if err := PrepareProfile(p, profile); err != nil {
		t.Fatal(err)
	}
	const ok, no = "ok", "PermissionError"
	steps := [][]string{
		{"write", cache, no}, {"unlink", cache, no}, {"write", temp + "/xcrun_db-standin", ok}, {"rename", temp + "/xcrun_db-standin", cache, no},
		{"rename", temp, s.at("moved"), no}, {"write", temp + "/other", ok}, {"unlink", temp + "/other", ok},
	}
	var args [][]string
	for _, st := range steps {
		args = append(args, st[:len(st)-1])
	}
	encoded, _ := json.Marshal(args)
	cmd := disclaim.Cmd(exec.Command(sandboxExec, "-f", profile, python, "-I", "-c", sbProbe, string(encoded)))
	cmd.Dir, cmd.Env = s.base, []string{"HOME=" + s.home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
	var probe, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &probe, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("the stand-in didn't run under the profile: %v %s", err, stderr.Bytes())
	}
	var got []string
	if err := json.Unmarshal(probe.Bytes(), &got); err != nil || len(got) != len(steps) {
		t.Fatalf("the stand-in said %q (%v)", probe.Bytes(), err)
	}
	for i, st := range steps {
		if got[i] != st[len(st)-1] {
			t.Errorf("%s %s: %s, want %s", st[0], strings.TrimPrefix(st[1], s.base), got[i], st[len(st)-1])
		}
	}
	// The desk's own git still runs, from the cache it can only read.
	if out, said, err := git(profile); err != nil || !strings.HasPrefix(out, "git version") || !strings.Contains(said, "lookup resolved in '"+cache+"'") {
		t.Errorf("/usr/bin/git under the profile: %v %q %q", err, out, said)
	}
	if now, err := os.ReadFile(cache); err != nil || !bytes.Equal(now, written) {
		t.Errorf("the cache changed under the profile: %v", err)
	}
}
