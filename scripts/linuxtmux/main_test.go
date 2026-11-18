package main

import (
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

const root = "../.."

// The checker and the build agree: the version build-tmux.sh builds, the guard's messages as the patch prints them
// (its C literals joined as the compiler joins them), and the recipe files are there.
func TestTheCheckerMatchesTheBuildAndThePatch(t *testing.T) {
	script := readText(t, filepath.Join(root, "scripts/build-tmux.sh"))
	if !strings.Contains(script, "\nTMUX_V="+tmuxVersion+" ") {
		t.Errorf("build-tmux.sh doesn't build tmux %s (TMUX_V)", tmuxVersion)
	}
	var added []string
	for _, line := range strings.Split(readText(t, filepath.Join(root, "third_party/tmux/guard.patch")), "\n") {
		if strings.HasPrefix(line, "+") {
			added = append(added, strings.TrimSpace(line[1:]))
		}
	}
	joined := regexp.MustCompile(`"\s*"`).ReplaceAllString(strings.Join(added, " "), "")
	for _, message := range guardMessages {
		if !strings.Contains(joined, `"`+message) {
			t.Errorf("the patch doesn't print %q", message)
		}
	}
	if _, err := recipeOf(root); err != nil {
		t.Error(err)
	}
}

func TestOnlyPublicGlibcVersionsUpTo235FromGlibcPass(t *testing.T) {
	for _, row := range []struct {
		versions []string
		newest   string
		problem  string
	}{
		{[]string{"libc.so.6 GLIBC_2.2.5", "libc.so.6 GLIBC_2.34", "libresolv.so.2 GLIBC_2.2.5", "libm.so.6 GLIBC_2.29"}, "2.34", ""},
		{[]string{"libc.so.6 GLIBC_2.35"}, "2.35", ""},
		{[]string{"libc.so.6 GLIBC_2.34", "libc.so.6 GLIBC_2.36"}, "2.36", "newer than glibc 2.35"},
		{[]string{"libc.so.6 GLIBC_3.0"}, "3.0", "newer than glibc 2.35"},
		{[]string{"libc.so.6 GLIBC_PRIVATE"}, "", "isn't a public glibc version"},
		{[]string{"libc.so.6 GLIBC_2.34", "libsystemd.so.0 LIBSYSTEMD_209"}, "2.34", "from libsystemd.so.0, which isn't allowed"},
		{nil, "", "names no glibc versions"},
	} {
		newest, problems := newestNeeded(row.versions, nil)
		got := strings.Join(problems, "; ")
		if newest != row.newest || (row.problem == "") != (got == "") || !strings.Contains(got, row.problem) {
			t.Errorf("%v: newest %q, problems %q; want %q and %q", row.versions, newest, got, row.newest, row.problem)
		}
	}
}

// FXTMUXARM64, on the facts alone (no stand-in built): an arm64 glibc program loads its own loader, from which
// Ubuntu's gcc takes the stack protector's __stack_chk_guard@GLIBC_2.17, as CI's arm64 tmux did. That loader and its
// public versions up to 2.35 pass for arm64 only; any other library, another processor's loader, and the loader's
// private or newer versions are still refused, and amd64 still refuses its own loader.
func TestOnlyArm64LoadsItsOwnLoader(t *testing.T) {
	text := []byte(strings.Join(guardMessages, "\x00"))
	program := func(arch string, libraries, versions []string) facts {
		target := targets[arch]
		return facts{class: elf.ELFCLASS64, machine: target.machine, loader: target.loader, libraries: libraries,
			versions: versions, stripped: true, text: text}
	}
	arm64 := []string{"libc.so.6", "libm.so.6", "libresolv.so.2", "ld-linux-aarch64.so.1"}
	arm64Versions := []string{"libc.so.6 GLIBC_2.17", "libc.so.6 GLIBC_2.34", "libresolv.so.2 GLIBC_2.17", "ld-linux-aarch64.so.1 GLIBC_2.17"}
	amd64 := []string{"libc.so.6", "libm.so.6", "libresolv.so.2"}
	amd64Versions := []string{"libc.so.6 GLIBC_2.2.5", "libc.so.6 GLIBC_2.34"}
	for _, row := range []struct {
		name, arch          string
		libraries, versions []string
		newest              string
		want                []string
	}{
		{"arm64 with its loader", "arm64", arm64, arm64Versions, "2.34", nil},
		{"arm64 without its loader", "arm64", amd64, arm64Versions[:3], "2.34", nil},
		{"arm64 with a library of its own", "arm64", append(arm64[:4:4], "libtinfo.so.6"), append(arm64Versions[:4:4], "libtinfo.so.6 NCURSES6_TINFO_5.0.19991023"), "2.34",
			[]string{"it loads libtinfo.so.6: only libc, libm and libresolv are allowed", "it needs NCURSES6_TINFO_5.0.19991023 from libtinfo.so.6, which isn't allowed"}},
		{"arm64 with amd64's loader", "arm64", append(arm64[:4:4], "ld-linux-x86-64.so.2"), append(arm64Versions[:4:4], "ld-linux-x86-64.so.2 GLIBC_2.3"), "2.34",
			[]string{"it loads ld-linux-x86-64.so.2", "it needs GLIBC_2.3 from ld-linux-x86-64.so.2, which isn't allowed"}},
		{"arm64's loader, a private version", "arm64", arm64, append(arm64Versions[:4:4], "ld-linux-aarch64.so.1 GLIBC_PRIVATE"), "2.34",
			[]string{"it needs GLIBC_PRIVATE from ld-linux-aarch64.so.1, which isn't a public glibc version"}},
		{"arm64's loader, a newer glibc", "arm64", arm64, append(arm64Versions[:4:4], "ld-linux-aarch64.so.1 GLIBC_2.36"), "2.36",
			[]string{"it needs GLIBC_2.36 from ld-linux-aarch64.so.1, newer than glibc 2.35"}},
		{"amd64", "amd64", amd64, amd64Versions, "2.34", nil},
		{"amd64 with its loader", "amd64", append(amd64[:3:3], "ld-linux-x86-64.so.2"), append(amd64Versions[:2:2], "ld-linux-x86-64.so.2 GLIBC_2.3"), "2.34",
			[]string{"it loads ld-linux-x86-64.so.2: only libc, libm and libresolv are allowed", "it needs GLIBC_2.3 from ld-linux-x86-64.so.2, which isn't allowed"}},
		{"amd64 with arm64's loader", "amd64", append(amd64[:3:3], "ld-linux-aarch64.so.1"), append(amd64Versions[:2:2], "ld-linux-aarch64.so.1 GLIBC_2.17"), "2.34",
			[]string{"it loads ld-linux-aarch64.so.1", "it needs GLIBC_2.17 from ld-linux-aarch64.so.1, which isn't allowed"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			problems, newest := judge(program(row.arch, row.libraries, row.versions), row.arch)
			got := strings.Join(problems, "\n")
			if newest != row.newest {
				t.Errorf("newest glibc %q, want %q", newest, row.newest)
			}
			if row.want == nil && got != "" {
				t.Errorf("should pass:\n%s", got)
			}
			for _, want := range row.want {
				if !strings.Contains(got, want) {
					t.Errorf("no %q among:\n%s", want, got)
				}
			}
			if len(problems) != len(row.want) {
				t.Errorf("%d problems, want %d:\n%s", len(problems), len(row.want), got)
			}
		})
	}
	if allowedLibraries["ld-linux-aarch64.so.1"] || len(allowedLibraries) != 3 {
		t.Errorf("arm64's loader leaked into every processor's list: %v", allowedLibraries)
	}
}

// standIn builds a Linux program for arch on this computer, as Go's own linker makes one: dynamic, with the system
// loader and the libraries named, the guard's messages in it unless guard is false, stripped unless strip is false.
// Go's linker writes no version table, so every stand-in also says it names no glibc versions (a real build does:
// TestTheBuiltTmuxPassesTheCheck).
func standIn(t *testing.T, arch string, libraries []string, guard, strip bool) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("SKIPPED, no go here to build the stand-ins")
	}
	src := t.TempDir()
	text := "package main\n\nimport _ \"unsafe\"\n\n"
	for _, lib := range libraries {
		text += "//go:cgo_import_dynamic _ _ \"" + lib + "\"\n"
	}
	messages := []string{"no guard here"}
	if guard {
		messages = guardMessages
	}
	text += "\nvar messages = []string{\"" + strings.Join(messages, "\", \"") + "\"}\n\nfunc main() {\n\tfor _, m := range messages {\n\t\tprintln(m)\n\t}\n}\n"
	writeText(t, filepath.Join(src, "main.go"), text)
	writeText(t, filepath.Join(src, "go.mod"), "module standin\n\ngo 1.26\n")
	out := filepath.Join(t.TempDir(), "tmux")
	flags := "-linkmode=internal"
	if strip {
		flags += " -s -w"
	}
	cmd := exec.Command("go", "build", "-ldflags="+flags, "-o", out, ".")
	cmd.Dir = src
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0", "GOFLAGS=")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, b)
	}
	return out
}

const noVersions = "names no glibc versions"

// Each thing the file alone can show, on stand-ins: the processor, the loader, the libraries (libsystemd's among
// them), the symbol table and the guard. A stand-in right in all of them has only the version table missing.
func TestTheProgramIsCheckedForItsProcessorLibrariesAndGuard(t *testing.T) {
	good := []string{"libc.so.6", "libresolv.so.2"}
	for _, row := range []struct {
		name, built, checkedAs string
		libraries              []string
		guard, strip           bool
		want                   []string
	}{
		{"right for amd64", "amd64", "amd64", good, true, true, nil},
		{"right for arm64", "arm64", "arm64", good, true, true, nil},
		{"arm64 checked as amd64", "arm64", "amd64", good, true, true, []string{"not linux/amd64", "not glibc's \"/lib64/ld-linux-x86-64.so.2\""}},
		{"amd64 checked as arm64", "amd64", "arm64", good, true, true, []string{"not linux/arm64", "not glibc's \"/lib/ld-linux-aarch64.so.1\""}},
		{"built with systemd", "amd64", "amd64", append(good, "libsystemd.so.0"), true, true, []string{"it loads libsystemd.so.0"}},
		{"a library of its own", "amd64", "amd64", append(good, "libtinfo.so.6"), true, true, []string{"it loads libtinfo.so.6"}},
		{"no libc", "amd64", "amd64", []string{"libm.so.6"}, true, true, []string{"doesn't load glibc's libc.so.6"}},
		{"not stripped", "amd64", "amd64", good, true, false, []string{"isn't stripped"}},
		{"no guard", "amd64", "amd64", good, false, true, []string{"it isn't the guarded tmux"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			f, err := readFacts(standIn(t, row.built, row.libraries, row.guard, row.strip))
			if err != nil {
				t.Fatal(err)
			}
			problems, _ := judge(f, row.checkedAs)
			got := strings.Join(problems, "\n")
			for _, want := range append(row.want, noVersions) {
				if !strings.Contains(got, want) {
					t.Errorf("no %q among:\n%s", want, got)
				}
			}
			if len(problems) == 1 && row.want != nil {
				t.Errorf("only the version table was found wrong: %s", got)
			}
			if row.want == nil && len(problems) != 1 {
				t.Errorf("a right stand-in should have only the version table wrong:\n%s", got)
			}
		})
	}
	dir := t.TempDir()
	writeText(t, filepath.Join(dir, "tmux"), "#!/bin/sh\necho tmux 3.8\n")
	if _, err := readFacts(filepath.Join(dir, "tmux")); err == nil || !strings.Contains(err.Error(), "not an ELF program") {
		t.Errorf("a script: %v", err)
	}
}

// withVersions has readFacts give the stand-ins the version table Go's linker leaves out, for the rows below that
// are about BUILDINFO and the notices.
func withVersions(t *testing.T) {
	t.Helper()
	readELF = func(path string) (facts, error) {
		f, err := readFacts(path)
		f.versions, f.versionsErr = []string{"libc.so.6 GLIBC_2.2.5", "libc.so.6 GLIBC_2.34"}, nil
		return f, err
	}
	t.Cleanup(func() { readELF = readFacts })
}

// built is a folder as the build leaves it: the stand-in tmux, the notices, and BUILDINFO written by -write.
func built(t *testing.T, repo, arch string) string {
	t.Helper()
	dir := filepath.Dir(standIn(t, arch, []string{"libc.so.6", "libm.so.6", "libresolv.so.2"}, true, true))
	writeText(t, filepath.Join(dir, "THIRD-PARTY-NOTICES.txt"), "# tmux 3.8 (ISC licence): COPYING\n# libevent 2\n# ncurses 6\n# utf8proc 2\n")
	if problems := check(repo, dir, arch, true); problems != nil {
		t.Fatalf("-write: %v", problems)
	}
	return dir
}

// A copy of the recipe files, to change one.
func recipeCopy(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, name := range recipeFiles {
		writeText(t, filepath.Join(repo, name), readText(t, filepath.Join(root, name)))
	}
	return repo
}

func TestBuildInfoTiesTheFileToThisCheckoutsRecipe(t *testing.T) {
	withVersions(t)
	dir := built(t, root, "amd64")
	info := readText(t, filepath.Join(dir, "BUILDINFO"))
	recipe, _ := recipeOf(root)
	sum, _ := fileSum(filepath.Join(dir, "tmux"))
	if info != "Smolcorp's tmux 3.8 for linux/amd64, guarded\nrecipe "+recipe+"\nsha256 "+sum+"\nglibc 2.34\n" {
		t.Errorf("BUILDINFO:\n%s", info)
	}
	if problems := check(root, dir, "amd64", false); problems != nil {
		t.Errorf("as built: %v", problems)
	}

	expect := func(name, repo, dir, arch, want string) {
		t.Helper()
		got := strings.Join(check(repo, dir, arch, false), "\n")
		if !strings.Contains(got, want) {
			t.Errorf("%s: no %q among:\n%s", name, want, got)
		}
	}
	// The guard changed after it was built (one letter of a comment): the recipe isn't this checkout's.
	repo := recipeCopy(t)
	patch := filepath.Join(repo, "third_party/tmux/guard.patch")
	writeText(t, patch, strings.Replace(readText(t, patch), "Smolcorp's guard.", "Smolcorp's guard!", 1))
	expect("another guard", repo, dir, "amd64", "BUILDINFO doesn't match")
	// The same for the build script and the pins.
	repo = recipeCopy(t)
	sums := filepath.Join(repo, "third_party/tmux/SHA256SUMS")
	writeText(t, sums, readText(t, sums)+"\n")
	expect("other pins", repo, dir, "amd64", "BUILDINFO doesn't match")
	// The file replaced after BUILDINFO was written (a byte more).
	other := copyDir(t, dir)
	data, _ := os.ReadFile(filepath.Join(other, "tmux"))
	writeText(t, filepath.Join(other, "tmux"), string(data)+"\x00")
	expect("a changed file", root, other, "amd64", "BUILDINFO doesn't match")
	// Another processor's build, checked as this one's, and its BUILDINFO.
	expect("arm64's BUILDINFO as amd64", root, built(t, root, "arm64"), "amd64", "not linux/amd64")
	// No BUILDINFO, no notices, notices without a library.
	other = copyDir(t, dir)
	os.Remove(filepath.Join(other, "BUILDINFO"))
	expect("no BUILDINFO", root, other, "amd64", "no BUILDINFO beside it")
	other = copyDir(t, dir)
	os.Remove(filepath.Join(other, "THIRD-PARTY-NOTICES.txt"))
	expect("no notices", root, other, "amd64", "no THIRD-PARTY-NOTICES.txt beside it")
	other = copyDir(t, dir)
	writeText(t, filepath.Join(other, "THIRD-PARTY-NOTICES.txt"), "# tmux 3.8 (ISC licence): COPYING\n# libevent 2\n# utf8proc 2\n")
	expect("notices without ncurses", root, other, "amd64", `no "# ncurses " section`)
	expect("an unknown processor", root, dir, "386", "not amd64 or arm64")

	// -write refuses a program that fails, and writes nothing.
	bad := filepath.Dir(standIn(t, "amd64", []string{"libc.so.6", "libsystemd.so.0"}, true, true))
	if problems := check(root, bad, "amd64", true); problems == nil {
		t.Error("-write passed a tmux built with systemd")
	}
	if _, err := os.Stat(filepath.Join(bad, "BUILDINFO")); err == nil {
		t.Error("-write wrote BUILDINFO for a tmux it refused")
	}
}

// The real build, in CI's tmux-linux job (SMOLCORP_TEST_TMUX, on each processor's runner): it passes as it is, and
// fails once its guard's text is gone or its BUILDINFO is another build's.
func TestTheBuiltTmuxPassesTheCheck(t *testing.T) {
	program := os.Getenv("SMOLCORP_TEST_TMUX")
	if program == "" || runtime.GOOS != "linux" {
		if runtime.GOOS == "linux" && os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal("SMOLCORP_SANDBOX_REQUIRED=1 and no SMOLCORP_TEST_TMUX: run scripts/build-tmux.sh linux first")
		}
		t.Skip("SKIPPED, no Linux build here: CI's tmux-linux job runs this with SMOLCORP_TEST_TMUX")
	}
	dir := filepath.Dir(program)
	if problems := check(root, dir, runtime.GOARCH, false); problems != nil {
		t.Fatalf("the built tmux: %v", problems)
	}
	other := copyDir(t, dir)
	data, _ := os.ReadFile(filepath.Join(other, "tmux"))
	writeText(t, filepath.Join(other, "tmux"), strings.ReplaceAll(string(data), "smolcorp guard: ", "smolcorp GUARD: "))
	if got := strings.Join(check(root, other, runtime.GOARCH, false), "\n"); !strings.Contains(got, "it isn't the guarded tmux") {
		t.Errorf("the guard's text taken out: %s", got)
	}
	other = copyDir(t, dir)
	info := filepath.Join(other, "BUILDINFO")
	writeText(t, info, strings.Replace(readText(t, info), "glibc ", "glibc 9", 1))
	if got := strings.Join(check(root, other, runtime.GOARCH, false), "\n"); !strings.Contains(got, "BUILDINFO doesn't match") {
		t.Errorf("another BUILDINFO: %s", got)
	}
}

// The release checks each Linux tmux before it builds anything, puts it in that download with its notices, and
// reads it back from the archive; install.sh puts it where the core looks for Smolcorp's own tmux on Linux.
func TestTheReleaseAndTheInstallerCarryTheCheckedTmux(t *testing.T) {
	release := readText(t, filepath.Join(root, "scripts/release-desktop.sh"))
	checks := strings.Index(release, `go run ./scripts/linuxtmux -arch "$arch" "build/tmux-linux/$arch"`)
	firstBuild := strings.Index(release, "\n  build windows ")
	if checks < 0 || firstBuild < 0 || checks > firstBuild {
		t.Errorf("release-desktop.sh doesn't check build/tmux-linux before it builds (check at %d, first build at %d)", checks, firstBuild)
	}
	for _, want := range []string{
		`cp "build/tmux-linux/$arch/tmux" "$dir/tmux"`,
		`cp "build/tmux-linux/$arch/THIRD-PARTY-NOTICES.txt" "$dir/THIRD-PARTY-NOTICES-tmux.txt"`,
		`tar -xOzf "build/Smolcorp-Linux-$name.tar.gz" "Smolcorp-Linux-$name/tmux" | shasum -a 256`,
	} {
		if !strings.Contains(release, want) {
			t.Errorf("release-desktop.sh has no %s", want)
		}
	}
	hire := readText(t, filepath.Join(root, "desktop/hire.go"))
	if !strings.Contains(hire, `trustedExecutableIn(filepath.Join(home, ".local", "lib", "smolcorp"), "tmux")`) {
		t.Error(`desktop/hire.go no longer looks for Smolcorp's tmux in ~/.local/lib/smolcorp: install.sh puts it there`)
	}
}

// install.sh, run for real in a home folder of the test's own with stand-ins for the programs: Smolcorp's tmux goes
// to ~/.local/lib/smolcorp/tmux (755, the folder 755, the notices beside it); one that doesn't run there is taken out
// again and the installer says so.
func TestTheInstallerPutsTheTmuxWhereTheCoreLooks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is Linux's")
	}
	for _, runs := range []bool{true, false} {
		home, folder := t.TempDir(), t.TempDir()
		for _, name := range []string{"install.sh", "smolcorp.png"} {
			writeText(t, filepath.Join(folder, name), readText(t, filepath.Join(root, "packaging/linux", name)))
		}
		writeText(t, filepath.Join(folder, "THIRD-PARTY-NOTICES-tmux.txt"), "notices\n")
		writeExec(t, filepath.Join(folder, "smolcorp"), "#!/bin/sh\nexit 0\n")
		tmux := "#!/bin/sh\necho 'tmux 3.8'\n"
		if !runs {
			tmux = "#!/bin/sh\necho 'version GLIBC_2.38 not found' >&2\nexit 1\n"
		}
		writeExec(t, filepath.Join(folder, "tmux"), tmux)
		cmd := exec.Command("/bin/sh", "install.sh")
		cmd.Dir = folder
		cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "SHELL=/bin/sh", "XDG_DATA_HOME=" + filepath.Join(home, "data")}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("install.sh: %v\n%s", err, out)
		}
		lib := filepath.Join(home, ".local", "lib", "smolcorp")
		info, err := os.Lstat(filepath.Join(lib, "tmux"))
		said := strings.Contains(string(out), "Smolcorp's own tmux doesn't run on this computer")
		if runs {
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o755 || said {
				t.Errorf("a tmux that runs: %v %v, said %v\n%s", info, err, said, out)
			}
			if dir, err := os.Stat(lib); err != nil || dir.Mode().Perm() != 0o755 {
				t.Errorf("its folder: %v %v", dir, err)
			}
			if readText(t, filepath.Join(lib, "THIRD-PARTY-NOTICES-tmux.txt")) != "notices\n" {
				t.Error("no notices beside it")
			}
		} else if err == nil || !said {
			t.Errorf("a tmux that doesn't run should be taken out and said: %v, said %v\n%s", err, said, out)
		}
		if _, err := os.Stat(filepath.Join(lib, "tmux.new")); err == nil {
			t.Error("tmux.new was left behind")
		}
	}
}

// The installer's sandbox step (L1APPARMOR1, RT126L1APPARMOR1 F2), on a stand-in smolcorp that logs each call: it
// runs exactly the text `smolcorp sandbox command` prints (here a harmless marker writer, no sudo), and only when
// `sandbox check` says so (exit 1, its words printed); it checks again after; a command that fails, or a check that
// still fails, says hires won't start; a missing bubblewrap package (exit 3) gets its words and no command; and the
// install goes on to exit 0 in every case.
func TestTheInstallerRunsSmolcorpsSandboxCommandOnlyWhenTheCheckSays(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is Linux's")
	}
	for _, row := range []struct {
		name   string
		checks string // the check's exit codes, one per call, the last repeated
		fails  bool   // the command fails
		calls  string
		ran    bool
		say    []string
		unsaid []string
	}{
		{"set up by the command", "1 0", false, "sandbox check|sandbox command|sandbox check|setup --open", true,
			[]string{"STAND-IN WHY AND COMMAND", "The installer runs it for you now."}, []string{"hires won't start"}},
		{"the command declined", "1", true, "sandbox check|sandbox command|setup --open", true,
			[]string{"STAND-IN WHY AND COMMAND", "hires won't start until it does: run the line above"}, nil},
		{"still refused after", "1", false, "sandbox check|sandbox command|sandbox check|setup --open", true,
			[]string{"hires won't start until it does"}, nil},
		{"no bubblewrap package", "3", false, "sandbox check|setup --open", false,
			[]string{"STAND-IN INSTALL BUBBLEWRAP Then run ./install.sh again."}, []string{"runs it for you", "hires won't start"}},
		{"already in place", "0", false, "sandbox check|setup --open", false, nil,
			[]string{"STAND-IN", "runs it for you", "hires won't start"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			home, folder, state := t.TempDir(), t.TempDir(), t.TempDir()
			for _, name := range []string{"install.sh", "smolcorp.png"} {
				writeText(t, filepath.Join(folder, name), readText(t, filepath.Join(root, "packaging/linux", name)))
			}
			writeText(t, filepath.Join(folder, "THIRD-PARTY-NOTICES-tmux.txt"), "notices\n")
			writeExec(t, filepath.Join(folder, "tmux"), "#!/bin/sh\necho 'tmux 3.8'\n")
			fail := ""
			if row.fails {
				fail = "; exit 1"
			}
			// The check's nth call exits with the nth code (the last one after); the command prints a line that
			// writes the marker, with quotes and spaces the installer must pass on as they are.
			writeExec(t, filepath.Join(folder, "smolcorp"), `#!/bin/sh
state='`+state+`'
echo "$*" >> "$state/calls"
case "$1 $2" in
"sandbox check")
  n=$(grep -c '^sandbox check' "$state/calls")
  code=$(echo '`+row.checks+`' | tr ' ' '\n' | sed -n "${n}p")
  [ -n "$code" ] || code=$(echo '`+row.checks+`' | tr ' ' '\n' | tail -n 1)
  [ "$code" = 1 ] && echo "STAND-IN WHY AND COMMAND"
  [ "$code" = 3 ] && echo "STAND-IN INSTALL BUBBLEWRAP"
  exit "$code" ;;
"sandbox command")
  printf '%s\n' "printf '%s\\n' \"ran, exactly\" >> '$state/marker'`+fail+`" ;;
esac
exit 0
`)
			cmd := exec.Command("/bin/sh", "install.sh")
			cmd.Dir = folder
			cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "SHELL=/bin/sh", "XDG_DATA_HOME=" + filepath.Join(home, "data")}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("install.sh: %v\n%s", err, out)
			}
			calls := strings.Join(strings.Fields(strings.ReplaceAll(readText(t, filepath.Join(state, "calls")), " ", "_")), "|")
			if want := strings.ReplaceAll(row.calls, " ", "_"); calls != want {
				t.Errorf("the installer called %s, want %s\n%s", calls, want, out)
			}
			marker, _ := os.ReadFile(filepath.Join(state, "marker"))
			if row.ran && string(marker) != "ran, exactly\n" || !row.ran && marker != nil {
				t.Errorf("the command's text ran: %q, want it run once, as printed: %v", marker, row.ran)
			}
			for _, want := range row.say {
				if !strings.Contains(string(out), want) {
					t.Errorf("no %q in:\n%s", want, out)
				}
			}
			for _, unwanted := range row.unsaid {
				if strings.Contains(string(out), unwanted) {
					t.Errorf("%q in:\n%s", unwanted, out)
				}
			}
		})
	}
}

// RT126L1APPARMOR1R2 B3, for the installer (moved here from internal/sandbox, whose test runs where the repo isn't:
// FXL1APPARMOR1VM2 B2): its comments say a program of the owner's can load its own code into Smolcorp's bubblewrap
// (LD_PRELOAD) and use its capabilities, not round 2's "a program that runs the copy gets no capabilities".
func TestTheInstallerSaysWhatTheSandboxGrants(t *testing.T) {
	var comments []string
	for _, line := range strings.Split(readText(t, filepath.Join(root, "packaging/linux/install.sh")), "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "#") {
			comments = append(comments, strings.TrimPrefix(line, "#"))
		}
	}
	text := strings.Join(strings.Fields(strings.Join(comments, " ")), " ")
	if strings.Contains(text, "a program that runs the copy gets a namespace with no capabilities") ||
		!strings.Contains(text, "LD_PRELOAD") {
		t.Error("install.sh doesn't say a program of the owner's can use the copy's own capabilities")
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeExec(t *testing.T, path, text string) {
	t.Helper()
	writeText(t, path, text)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func copyDir(t *testing.T, dir string) string {
	t.Helper()
	other := t.TempDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		writeText(t, filepath.Join(other, entry.Name()), readText(t, filepath.Join(dir, entry.Name())))
	}
	return other
}
