// Command forgedevents stops a release whose forged-event table isn't whole (SANDBOX-PLAN design A, SA-X10 and
// SA-X11): the tests that show a desk can't send an event as another desk, on each of the three systems. Each test of
// the table must be in the desktop package as each system it is for builds it, so that no build tag or rename drops
// it there unseen, and must pass here, on the computer the release is built on. CI runs the same tests on Linux and
// Windows (go test ./... in ci.yml), which a Mac can't.
//
//	go run ./scripts/forgedevents -go <go> -cc <cc> -sdk <sdk> -work <folder> -modules <folder>
//
// (scripts/release-desktop.sh runs it so, from the repo's top folder.)
//
// It says why it stopped and exits 1; 0 when every test is in every build it is for, and passed here.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

var (
	everywhere = []string{"darwin", "linux", "windows"}
	unix       = []string{"darwin", "linux"}
)

// test is one test of the table, by its exact name, and the systems whose build of the package must hold it.
type test struct {
	name    string
	systems []string
}

// table is the forged-event table. A test renamed here is a test renamed in its file too, in the same change.
var table = []test{
	// X10's rows, on the deck's side (desktop/hookkeys_test.go, hookkeys_unix_test.go).
	{"TestADeskCantSendAnEventAsAnotherDesk", everywhere},
	{"TestALaunchersDeskReportsWithItsOwnKeyOnly", everywhere},
	{"TestALauncherFoundByFolderIsCheckedAsItsDesks", everywhere},
	{"TestADesksDroppedSessionComesBackForItsOwnKeyOnly", everywhere},
	{"TestAManagedDeskReportsWithTheHookKeyItsTicketHolds", unix},
	// X11's, through the hooks themselves: `smolcorp hook` everywhere (desktop/forgedevents_test.go), the Mac app's
	// report.sh where bash runs it (desktop/reportscript_test.go).
	{"TestADeskCantSendAnEventAsAnotherDeskThroughItsHook", everywhere},
	{"TestTheAppsReportShSendsItsDesksKeyAndNoOthers", unix},
}

// tools are the programs the gate runs, each named once, at the start: go, the C compiler that builds the cgo parts of
// the package's test binary and links it, and on a Mac the SDK that compiler builds with. Then where go runs: home is
// HOME and tmp TMPDIR; work holds go's build cache, module cache and GOPATH, made fresh for the run (see goFolders);
// and modules is the module download folder go reads offline, as a proxy, checking each zip against go.sum.
type tools struct {
	goProgram, cc, sdk       string
	home, tmp, work, modules string
}

func main() {
	if err := run(os.Args[1:], table, os.Stdout); err != nil {
		fmt.Println("Stopped: " + err.Error())
		os.Exit(1)
	}
	fmt.Println("The forged-event table is in every system's build, and passed here.")
}

// run is the gate, with the command line's arguments: it checks tests, the table, and stops on any error.
func run(args []string, tests []test, out io.Writer) error {
	flags := flag.NewFlagSet("forgedevents", flag.ContinueOnError)
	pkg := flags.String("pkg", "./desktop", "the package the table is in")
	goProgram := flags.String("go", "go", "the go to run the tests with: the one the release pinned")
	cc := flags.String("cc", "", "the C compiler the tests are built with, by its real path: the one the release pinned (found here, as the release finds it, when not given)")
	sdk := flags.String("sdk", "", "on a Mac, the SDK that C compiler builds with: the one the release pinned")
	work := flags.String("work", "", "the folder for go's build cache, module cache and GOPATH: the release's, made fresh at its start (made here, and removed at the end, when not given)")
	modules := flags.String("modules", "", "the module download folder go reads offline: the release's (go's own for this account when not given)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	use := tools{goProgram: *goProgram, cc: *cc, sdk: *sdk, home: home, tmp: os.TempDir(), work: *work, modules: *modules}
	if use.cc == "" {
		if use.cc, use.sdk, err = cCompiler(); err != nil {
			return err
		}
	}
	if use.modules == "" {
		if use.modules, err = goDownloads(use); err != nil {
			return err
		}
	}
	if use.work == "" {
		if use.work, err = os.MkdirTemp("", "smolcorp-forged-"); err != nil {
			return err
		}
		defer removeFolders(use.work, out)
	}
	return check(".", *pkg, use, tests, out)
}

// goDownloads is go's own module download folder for use.home's account, as go names it with none of the caller's
// settings.
func goDownloads(use tools) (string, error) {
	cmd := exec.Command(use.goProgram, "env", "GOMODCACHE")
	cmd.Env = []string{"HOME=" + use.home, "PATH=" + systemPath, "GOENV=off", "GOTOOLCHAIN=local", "LANG=C"}
	out, err := cmd.Output()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return "", fmt.Errorf("go doesn't name its module cache (go env GOMODCACHE: %v)", err)
	}
	return filepath.Join(strings.TrimSpace(string(out)), "cache", "download"), nil
}

// removeFolders removes the folders the gate made for go, after it says how large they grew: a build cache can be.
// go makes the modules it unpacks read-only, so each folder is made writable first.
func removeFolders(work string, out io.Writer) {
	var size int64
	filepath.WalkDir(work, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			os.Chmod(path, 0o700)
		} else if info, err := entry.Info(); err == nil {
			size += info.Size()
		}
		return nil
	})
	fmt.Fprintf(out, "go's folders for the table (%s): %d KB, removed.\n", work, size/1024)
	os.RemoveAll(work)
}

// passed is go test -v's line for a test that ran and passed (a skipped one says SKIP).
var passed = regexp.MustCompile(`(?m)^--- PASS: (Test\w*) \(`)

// systemPath is the PATH of each go the gate runs: the system's own folders, nothing a caller put first. go and the C
// compiler are named by their real paths, found before.
const systemPath = "/usr/bin:/bin:/usr/sbin:/sbin"

// goFolders are go's build cache, module cache and GOPATH in work, by name: the ones release-desktop.sh's go_env
// names in the same folder.
func goFolders(work string) (cache, modules, gopath string) {
	return filepath.Join(work, "cache"), filepath.Join(work, "modules"), filepath.Join(work, "gopath")
}

// pinned is the whole environment of each go the gate runs, for system's build (RT126SAX11R3 B4): these settings,
// each set here, and nothing of the caller's, a setting nobody has thought of included. So no GOFLAGS but this one (a
// set GOFLAGS replaces a go env file's and the toolchain's own go.env's, and their -exec, -toolexec or -overlay), no
// go env file, no go.work, no other toolchain than this go's own whatever go.mod's toolchain line asks, the
// toolchain's own GOROOT, system for this machine's processor (a test built for another system would run through a
// go_<system>_<arch>_exec), use's C compiler and SDK, the system's own PATH, and none of the folders of libraries or
// headers, the flags or the linker the C compiler would take from the environment. go's caches are use's fresh ones,
// and its modules come from use's download folder, offline, each checked against go.sum.
func pinned(system string, use tools) []string {
	cache, modules, gopath := goFolders(use.work)
	env := []string{"HOME=" + use.home, "PATH=" + systemPath, "TMPDIR=" + use.tmp, "GOCACHE=" + cache,
		"GOMODCACHE=" + modules, "GOPATH=" + gopath, "GOPROXY=file://" + use.modules, "GOSUMDB=off",
		"GOFLAGS=-mod=readonly", "GOENV=off", "GOWORK=off", "GOTOOLCHAIN=local", "LANG=C", "CC=" + use.cc,
		"GOOS=" + system, "GOARCH=" + runtime.GOARCH}
	if use.sdk != "" {
		env = append(env, "SDKROOT="+use.sdk)
	}
	return env
}

// cCompiler finds the C compiler by its real path, and on a Mac the SDK it builds with, as release-desktop.sh does at
// its start: on a Mac, the active developer folder's clang and SDK, asked of xcrun by its own path with no lookup
// cache and none of the caller's settings (DEVELOPER_DIR, SDKROOT, TOOLCHAINS); elsewhere, the cc first on PATH.
func cCompiler() (cc, sdk string, err error) {
	if runtime.GOOS == "darwin" {
		xcrun := func(args ...string) (string, error) {
			cmd := exec.Command("/usr/bin/xcrun", append([]string{"--no-cache"}, args...)...)
			cmd.Env = []string{"HOME=" + os.Getenv("HOME"), "PATH=" + systemPath, "LANG=C"}
			out, err := cmd.Output()
			return strings.TrimSpace(string(out)), err
		}
		if cc, err = xcrun("--find", "clang"); err != nil || cc == "" {
			return "", "", fmt.Errorf("no C compiler found (xcrun --find clang: %v)", err)
		}
		if sdk, err = xcrun("--show-sdk-path"); err != nil || sdk == "" {
			return "", "", fmt.Errorf("no Mac SDK found (xcrun --show-sdk-path: %v)", err)
		}
		if sdk, err = filepath.EvalSymlinks(sdk); err != nil {
			return "", "", err
		}
	} else if cc, err = exec.LookPath("cc"); err != nil {
		return "", "", fmt.Errorf("no C compiler found (cc on PATH): %w", err)
	}
	if cc, err = filepath.EvalSymlinks(cc); err != nil {
		return "", "", err
	}
	return cc, sdk, nil
}

// check stops unless every test of tests is in pkg (run from dir) as each of its systems builds it, and passes here,
// run by use's go, C compiler and SDK.
func check(dir, pkg string, use tools, tests []test, out io.Writer) error {
	var missing []string
	for _, system := range everywhere {
		found, err := testsIn(dir, pkg, use, system)
		if err != nil {
			return fmt.Errorf("can't read %s as %s builds it: %w", pkg, system, err)
		}
		for _, t := range tests {
			if slices.Contains(t.systems, system) && !found[t.name] {
				missing = append(missing, fmt.Sprintf("%s (%s)", t.name, system))
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("the forged-event table isn't whole: no %s in %s. Put each back under its name "+
			"(scripts/forgedevents names them), then run this again", strings.Join(missing, ", no "), pkg)
	}
	var here []string
	for _, t := range tests {
		if slices.Contains(t.systems, runtime.GOOS) {
			here = append(here, t.name)
		}
	}
	if len(here) == 0 {
		return fmt.Errorf("no test of the forged-event table runs on %s", runtime.GOOS)
	}
	run := exec.Command(use.goProgram, "test", "-count=1", "-v", "-run", "^("+strings.Join(here, "|")+")$", pkg)
	run.Dir, run.Env = dir, pinned(runtime.GOOS, use)
	output, err := run.CombinedOutput()
	ran := map[string]bool{}
	for _, m := range passed.FindAllSubmatch(output, -1) {
		ran[string(m[1])] = true
	}
	var failed []string
	for _, name := range here {
		if !ran[name] {
			failed = append(failed, name)
		}
	}
	if err != nil || len(failed) > 0 {
		out.Write(lastLines(output, 40))
		if len(failed) == 0 {
			return fmt.Errorf("go test of the forged-event table failed (%v): above", err)
		}
		return fmt.Errorf("the forged-event table didn't pass here: %s failed or was skipped (above)", strings.Join(failed, ", "))
	}
	return nil
}

// testsIn names the tests declared in pkg's test files (its own and its _test package's) as system builds them: each
// file parsed as Go, so a name in a comment, a string or a /* */ block isn't a test.
func testsIn(dir, pkg string, use tools, system string) (map[string]bool, error) {
	list := exec.Command(use.goProgram, "list", "-f", `{{.Dir}}{{range .TestGoFiles}}{{"\n"}}{{.}}{{end}}{{range .XTestGoFiles}}{{"\n"}}{{.}}{{end}}`, pkg)
	list.Dir = dir
	list.Env = pinned(system, use)
	if system != runtime.GOOS {
		// As the other system's CI builds it, for its files; cgo can't cross from here.
		list.Env = append(list.Env, "CGO_ENABLED=0")
	}
	var stderr bytes.Buffer
	list.Stderr = &stderr
	output, err := list.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, errors.New("go list named no folder")
	}
	found := map[string]bool{}
	files := token.NewFileSet()
	for _, file := range lines[1:] {
		parsed, err := parser.ParseFile(files, filepath.Join(lines[0], file), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range parsed.Decls {
			if f, ok := decl.(*ast.FuncDecl); ok && f.Recv == nil && f.Body != nil && strings.HasPrefix(f.Name.Name, "Test") && takesT(f) {
				found[f.Name.Name] = true
			}
		}
	}
	return found, nil
}

// takesT says whether f takes a *testing.T and nothing else, as a test does.
func takesT(f *ast.FuncDecl) bool {
	params := f.Type.Params.List
	if len(params) != 1 || len(params[0].Names) > 1 {
		return false
	}
	star, ok := params[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "T" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "testing"
}

// lastLines is the last n lines of text.
func lastLines(text []byte, n int) []byte {
	lines := bytes.SplitAfter(text, []byte("\n"))
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return bytes.Join(lines, nil)
}
