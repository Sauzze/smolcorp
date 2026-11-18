// Command linuxtmux checks the tmux that the Linux downloads carry (SANDBOX-PLAN design A, SA-L5): Smolcorp's
// own tmux with the guard in it (third_party/tmux/guard.patch), built by `scripts/build-tmux.sh linux` on a
// Linux computer of each processor (CI's ubuntu-22.04 runners, amd64 and arm64), and put into each download by
// scripts/release-desktop.sh, which runs on a Mac and so can't start it. What the file alone can say, it checks:
//
//   - an ELF program for the processor named, started by the system's own loader, and stripped;
//
//   - it loads only glibc's libc, libm and libresolv (and, on arm64, glibc's own loader), and needs no glibc newer
//     than Ubuntu 22.04's, so it was built without systemd (a tmux built with it starts a server of its own when
//     LISTEN_FDS is set);
//
//   - it carries the guard's messages;
//
//   - BUILDINFO, written beside it by the build, names this checkout's recipe (build-tmux.sh, SHA256SUMS and
//     guard.patch, hashed) and the file's own SHA-256, and THIRD-PARTY-NOTICES.txt has each library's licence.
//
//     go run ./scripts/linuxtmux -arch amd64 -write DIR   checks DIR/tmux and writes DIR/BUILDINFO (the build)
//     go run ./scripts/linuxtmux -arch amd64 DIR          checks DIR/tmux, BUILDINFO and THIRD-PARTY-NOTICES.txt
//     (the build's check, CI, and the release)
//
// Run from the repo's top folder, or name it with -repo. It starts nothing, and reads only DIR and the recipe.
package main

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// tmuxVersion is the tmux build-tmux.sh builds (its TMUX_V); a test holds the two together.
const tmuxVersion = "3.8"

// newestGlibc is the newest glibc the program may need: Ubuntu 22.04's, where CI builds it. Debian 12, Ubuntu
// 22.04 and later, Fedora 36 and later run it.
var newestGlibc = [2]int{2, 35}

// The libraries it may load: glibc's own. libm is asked for by tmux's configure (fmod), libresolv for b64_ntop.
var allowedLibraries = map[string]bool{"libc.so.6": true, "libm.so.6": true, "libresolv.so.2": true}

// Each processor's ELF machine, the loader glibc puts in every program built for it, and whether a program built
// for it loads that loader as a library too. On arm64 it does: Ubuntu's gcc builds with the stack protector on, and
// there its guard value is __stack_chk_guard@GLIBC_2.17, which the loader itself gives (FXTMUXARM64). On amd64 the
// guard is read from the thread, and a tmux that loads the loader there is refused as before.
var targets = map[string]struct {
	machine      elf.Machine
	loader       string
	loaderNeeded bool
}{
	"amd64": {elf.EM_X86_64, "/lib64/ld-linux-x86-64.so.2", false},
	"arm64": {elf.EM_AARCH64, "/lib/ld-linux-aarch64.so.1", true},
}

// librariesFor is the libraries a tmux for arch may load: glibc's own, and that processor's loader where its
// programs load it (targets' loaderNeeded), by the name the program asks for it (ld-linux-aarch64.so.1).
func librariesFor(arch string) map[string]bool {
	target := targets[arch]
	if !target.loaderNeeded {
		return allowedLibraries
	}
	allowed := map[string]bool{path.Base(target.loader): true}
	for lib := range allowedLibraries {
		allowed[lib] = true
	}
	return allowed
}

// guardMessages are what the guard says when it stops a client (the patch's fprintf texts, as the compiler
// joins them): a tmux without them isn't the guarded one.
var guardMessages = []string{
	"smolcorp guard: no server pid given (SMOLCORP_TMUX_SERVER)",
	"smolcorp guard: server pid %ld, not %ld",
	"smolcorp guard: server pid %ld is a desk's, or unknown",
	"smolcorp guard: refused %s (message %u)",
}

// recipeFiles are what the build is made from besides the pinned tarballs, which SHA256SUMS names.
var recipeFiles = []string{"scripts/build-tmux.sh", "third_party/tmux/SHA256SUMS", "third_party/tmux/guard.patch"}

// noticeSections are the headings build-tmux.sh writes into THIRD-PARTY-NOTICES.txt for the Linux build.
var noticeSections = []string{"# tmux " + tmuxVersion + " (ISC licence)", "# libevent ", "# ncurses ", "# utf8proc "}

func main() {
	arch := flag.String("arch", "", "the processor it's for: amd64 or arm64")
	write := flag.Bool("write", false, "write DIR/BUILDINFO after checking DIR/tmux, instead of checking it")
	repo := flag.String("repo", ".", "the repo's top folder, for the recipe")
	flag.Parse()
	if flag.NArg() != 1 || *arch == "" {
		fmt.Fprintln(os.Stderr, "Usage: go run ./scripts/linuxtmux -arch amd64|arm64 [-write] DIR")
		os.Exit(2)
	}
	dir := flag.Arg(0)
	problems := check(*repo, dir, *arch, *write)
	for _, p := range problems {
		fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Join(dir, "tmux"), p)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
	if *write {
		fmt.Printf("  linux/%s: wrote %s\n", *arch, filepath.Join(dir, "BUILDINFO"))
	} else {
		sum, _ := fileSum(filepath.Join(dir, "tmux"))
		fmt.Printf("  linux/%s: the guarded tmux %s, built from this checkout's recipe, SHA-256 %s\n", *arch, tmuxVersion, sum)
	}
}

// check is every problem with DIR's tmux for arch, or nil. With write, BUILDINFO is written when the program
// passes; without, BUILDINFO and the notices are checked too.
func check(repo, dir, arch string, write bool) []string {
	if _, ok := targets[arch]; !ok {
		return []string{fmt.Sprintf("-arch is %q, not amd64 or arm64", arch)}
	}
	program := filepath.Join(dir, "tmux")
	facts, err := readELF(program)
	if err != nil {
		return []string{err.Error()}
	}
	problems, newest := judge(facts, arch)
	recipe, err := recipeOf(repo)
	if err != nil {
		return append(problems, err.Error())
	}
	sum, err := fileSum(program)
	if err != nil {
		return append(problems, err.Error())
	}
	want := buildInfo(arch, recipe, sum, newest)
	if write {
		if len(problems) == 0 {
			if err := os.WriteFile(filepath.Join(dir, "BUILDINFO"), []byte(want), 0o644); err != nil {
				problems = append(problems, err.Error())
			}
		}
		return problems
	}
	have, err := os.ReadFile(filepath.Join(dir, "BUILDINFO"))
	switch {
	case err != nil:
		problems = append(problems, "no BUILDINFO beside it: "+err.Error())
	case string(have) != want:
		problems = append(problems, fmt.Sprintf("BUILDINFO doesn't match: it says\n%s\nand this file, from this checkout's recipe, is\n%s", have, want))
	}
	notices, err := os.ReadFile(filepath.Join(dir, "THIRD-PARTY-NOTICES.txt"))
	if err != nil {
		return append(problems, "no THIRD-PARTY-NOTICES.txt beside it: "+err.Error())
	}
	for _, heading := range noticeSections {
		if !bytes.Contains(notices, []byte(heading)) {
			problems = append(problems, fmt.Sprintf("THIRD-PARTY-NOTICES.txt has no %q section", heading))
		}
	}
	return problems
}

// buildInfo is the BUILDINFO text for a build: what it is, the recipe it came from, its SHA-256, the newest
// glibc it needs.
func buildInfo(arch, recipe, sum, glibc string) string {
	return fmt.Sprintf("Smolcorp's tmux %s for linux/%s, guarded\nrecipe %s\nsha256 %s\nglibc %s\n", tmuxVersion, arch, recipe, sum, glibc)
}

// facts are what the ELF file says about itself.
type facts struct {
	class       elf.Class
	machine     elf.Machine
	loader      string
	libraries   []string
	versions    []string // every version it needs, as "library version"
	versionsErr error
	stripped    bool
	text        []byte
}

// readELF is readFacts; the tests add the version table that the stand-ins Go's linker makes can't have.
var readELF = readFacts

func readFacts(path string) (facts, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return facts{}, err
	}
	f, err := elf.NewFile(bytes.NewReader(text))
	if err != nil {
		return facts{}, fmt.Errorf("not an ELF program: %v", err)
	}
	defer f.Close()
	out := facts{class: f.Class, machine: f.Machine, text: text, stripped: f.Section(".symtab") == nil}
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			loader, _ := io.ReadAll(p.Open())
			out.loader = strings.TrimRight(string(loader), "\x00")
		}
	}
	if out.libraries, err = f.ImportedLibraries(); err != nil {
		return facts{}, fmt.Errorf("its libraries can't be read: %v", err)
	}
	needs, err := f.DynamicVersionNeeds()
	out.versionsErr = err
	for _, need := range needs {
		for _, dep := range need.Needs {
			out.versions = append(out.versions, need.Name+" "+dep.Dep)
		}
	}
	return out, nil
}

// judge is every problem the facts show for a tmux for arch, and the newest glibc it needs ("2.34").
func judge(f facts, arch string) ([]string, string) {
	var problems []string
	target := targets[arch]
	if f.class != elf.ELFCLASS64 || f.machine != target.machine {
		problems = append(problems, fmt.Sprintf("it's built for %v %v, not linux/%s (%v)", f.class, f.machine, arch, target.machine))
	}
	if f.loader != target.loader {
		problems = append(problems, fmt.Sprintf("its loader is %q, not glibc's %q", f.loader, target.loader))
	}
	if !f.stripped {
		problems = append(problems, "it isn't stripped (it has a symbol table)")
	}
	allowed := librariesFor(arch)
	libs := map[string]bool{}
	for _, lib := range f.libraries {
		libs[lib] = true
		if !allowed[lib] {
			problems = append(problems, fmt.Sprintf("it loads %s: only libc, libm and libresolv are allowed", lib))
		}
	}
	if !libs["libc.so.6"] {
		problems = append(problems, "it doesn't load glibc's libc.so.6")
	}
	newest, versionProblems := newestNeededFrom(f.versions, f.versionsErr, allowed)
	problems = append(problems, versionProblems...)
	for _, message := range guardMessages {
		if !bytes.Contains(f.text, []byte(message)) {
			problems = append(problems, fmt.Sprintf("it doesn't carry the guard's %q: it isn't the guarded tmux", message))
		}
	}
	return problems, newest
}

// newestNeeded is the newest GLIBC_ version among versions ("lib version" pairs), and the problems with them:
// none listed (a program built against glibc always lists them), one from a library that isn't allowed, glibc's
// private ones, or one newer than newestGlibc.
func newestNeeded(versions []string, readErr error) (string, []string) {
	return newestNeededFrom(versions, readErr, allowedLibraries)
}

// newestNeededFrom is newestNeeded with the libraries a version may come from: a processor's own (librariesFor).
// The loader's versions, where it's allowed, are held to the same rules as glibc's other libraries.
func newestNeededFrom(versions []string, readErr error, allowed map[string]bool) (string, []string) {
	if readErr != nil || len(versions) == 0 {
		return "", []string{fmt.Sprintf("it names no glibc versions (%v): a program built against glibc does", readErr)}
	}
	var problems []string
	best := [2]int{-1, -1}
	sort.Strings(versions)
	for _, v := range versions {
		lib, version, _ := strings.Cut(v, " ")
		if !allowed[lib] {
			problems = append(problems, fmt.Sprintf("it needs %s from %s, which isn't allowed", version, lib))
			continue
		}
		major, minor, ok := glibcVersion(version)
		if !ok {
			problems = append(problems, fmt.Sprintf("it needs %s from %s, which isn't a public glibc version", version, lib))
			continue
		}
		if major > newestGlibc[0] || major == newestGlibc[0] && minor > newestGlibc[1] {
			problems = append(problems, fmt.Sprintf("it needs %s from %s, newer than glibc %d.%d (Ubuntu 22.04)", version, lib, newestGlibc[0], newestGlibc[1]))
		}
		if major > best[0] || major == best[0] && minor > best[1] {
			best = [2]int{major, minor}
		}
	}
	if best[0] < 0 {
		return "", problems
	}
	return fmt.Sprintf("%d.%d", best[0], best[1]), problems
}

// glibcVersion reads "GLIBC_2.34" (or "GLIBC_2.2.5", read as 2.2).
func glibcVersion(v string) (int, int, bool) {
	rest, ok := strings.CutPrefix(v, "GLIBC_")
	if !ok {
		return 0, 0, false
	}
	parts := strings.Split(rest, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	return major, minor, err1 == nil && err2 == nil
}

// recipeOf hashes the recipe files of the repo at root: each one's name, length and bytes.
func recipeOf(root string) (string, error) {
	h := sha256.New()
	for _, name := range recipeFiles {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return "", errors.New("the recipe can't be read: " + err.Error())
		}
		fmt.Fprintf(h, "%s\x00%d\x00", name, len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileSum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
