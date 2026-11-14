package desktop

import (
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// SK3 (SANDBOX-PLAN §3.3): every Mac build has the hardened runtime, the smolcorp command and tmux included, and
// scripts/check-runtime.sh stops a build without it. These rows run on stand-ins in the test's own folder: copies of
// /usr/bin/true and two tiny C programs, signed ad hoc (no keychain is asked) and run directly. No debugger is
// attached to anything (it can ask for Developer Tools access): the debugger rows (M1, M2) are checked as what the
// kernel decides by, the runtime flag and no get-task-allow entitlement.

// repoRoot is the repository's top folder, where scripts/ is.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func needTools(t *testing.T, tools ...string) {
	t.Helper()
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s isn't here: %v", tool, err)
		}
	}
}

// adHoc signs file ad hoc, with the given codesign options.
func adHoc(t *testing.T, file string, options ...string) {
	t.Helper()
	args := append([]string{"--force", "--sign", "-"}, options...)
	if out, err := exec.Command("codesign", append(args, file)...).CombinedOutput(); err != nil {
		t.Fatalf("codesign %v: %v\n%s", options, err, out)
	}
}

// checkRuntime runs scripts/check-runtime.sh on files: its exit status and what it said.
func checkRuntime(t *testing.T, files ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(filepath.Join(repoRoot(t), "scripts", "check-runtime.sh"), files...)
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), string(out)
	} else if err != nil {
		t.Fatal(err)
	}
	return 0, string(out)
}

func TestABuildWithoutTheHardenedRuntimeFailsTheCheck(t *testing.T) {
	needTools(t, "codesign", "lipo")
	program := filepath.Join(t.TempDir(), "standin")
	data, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(program, data, 0o755); err != nil {
		t.Fatal(err)
	}

	// Unsigned, then signed ad hoc as build-app.sh signed before SK3: refused, for each processor in the file.
	if out, err := exec.Command("codesign", "--remove-signature", program).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if code, out := checkRuntime(t, program); code != 1 || !strings.Contains(out, "signed without the hardened runtime") {
		t.Errorf("unsigned: %d, %q", code, out)
	}
	adHoc(t, program)
	code, out := checkRuntime(t, program)
	archs, _ := exec.Command("lipo", "-archs", program).Output()
	if code != 1 || strings.Count(out, "signed without the hardened runtime") != len(strings.Fields(string(archs))) {
		t.Errorf("ad hoc without the runtime, %s: %d, %q", strings.TrimSpace(string(archs)), code, out)
	}

	// With it, ad hoc (flags=0x10002(adhoc,runtime)): passes.
	adHoc(t, program, "--options", "runtime")
	if code, out := checkRuntime(t, program); code != 0 || out != "" {
		t.Errorf("ad hoc with the runtime: %d, %q", code, out)
	}
	// One file of several failing fails the check, and names that one.
	other := filepath.Join(t.TempDir(), "other")
	if err := os.WriteFile(other, data, 0o755); err != nil {
		t.Fatal(err)
	}
	adHoc(t, other)
	if code, out := checkRuntime(t, program, other); code != 1 || !strings.Contains(out, other) || strings.Contains(out, program+" ") {
		t.Errorf("one of two without it: %d, %q", code, out)
	}
	if code, out := checkRuntime(t, filepath.Join(t.TempDir(), "gone")); code != 1 || !strings.Contains(out, "missing") {
		t.Errorf("a missing file: %d, %q", code, out)
	}
}

// M1/M2: a debugger gets into a hardened program only with get-task-allow, and K6's library only with the DYLD or
// library-validation entitlements: each one fails the check, the runtime on or not.
func TestTheEntitlementsThatOpenAHardenedProgramFailTheCheck(t *testing.T) {
	needTools(t, "codesign", "lipo")
	folder := t.TempDir()
	program := filepath.Join(folder, "standin")
	data, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(program, data, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"com.apple.security.get-task-allow", "com.apple.security.cs.allow-dyld-environment-variables",
		"com.apple.security.cs.disable-library-validation"} {
		entitlements := filepath.Join(folder, "e.plist")
		plist := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` +
			`<plist version="1.0"><dict><key>` + key + `</key><true/></dict></plist>`
		if err := os.WriteFile(entitlements, []byte(plist), 0o644); err != nil {
			t.Fatal(err)
		}
		adHoc(t, program, "--options", "runtime", "--entitlements", entitlements)
		if code, out := checkRuntime(t, program); code != 1 || !strings.Contains(out, "carries "+key) {
			t.Errorf("%s: %d, %q", key, code, out)
		}
	}
	// The app's own entitlements open nothing: Apple Events and the mic.
	adHoc(t, program, "--options", "runtime", "--entitlements", filepath.Join(repoRoot(t), "packaging", "macos", "Smolcorp.entitlements"))
	if code, out := checkRuntime(t, program); code != 0 {
		t.Errorf("Smolcorp.entitlements: %d, %q", code, out)
	}
}

// Each processor's slice carries entitlements of its own: an opener on one slice only, the runtime on both, fails the
// check, which names that slice. Each slice in turn, so the one this Mac doesn't run is among them.
func TestAnEntitlementOnOneProcessorOnlyFailsTheCheck(t *testing.T) {
	needTools(t, "codesign", "lipo")
	folder := t.TempDir()
	out, err := exec.Command("lipo", "-archs", "/usr/bin/true").Output()
	archs := strings.Fields(string(out))
	if err != nil || len(archs) < 2 {
		t.Skipf("/usr/bin/true has one processor here (%q, %v): no two slices to sign apart", out, err)
	}
	entitlements := filepath.Join(folder, "e.plist")
	plist := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` +
		`<plist version="1.0"><dict><key>com.apple.security.get-task-allow</key><true/></dict></plist>`
	if err := os.WriteFile(entitlements, []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, opened := range archs[:2] {
		var slices []string
		for _, arch := range archs[:2] {
			slice := filepath.Join(folder, opened+"-"+arch)
			if out, err := exec.Command("lipo", "/usr/bin/true", "-thin", arch, "-output", slice).CombinedOutput(); err != nil {
				t.Fatalf("lipo -thin %s: %v\n%s", arch, err, out)
			}
			if arch == opened {
				adHoc(t, slice, "--options", "runtime", "--entitlements", entitlements)
			} else {
				adHoc(t, slice, "--options", "runtime")
			}
			slices = append(slices, slice)
		}
		program := filepath.Join(folder, "standin-"+opened)
		if out, err := exec.Command("lipo", append([]string{"-create"}, append(slices, "-output", program)...)...).CombinedOutput(); err != nil {
			t.Fatalf("lipo -create: %v\n%s", err, out)
		}
		code, said := checkRuntime(t, program)
		if code != 1 || !strings.Contains(said, "("+opened+"): carries com.apple.security.get-task-allow") || strings.Count(said, "carries") != 1 {
			t.Errorf("get-task-allow on %s only: %d, %q", opened, code, said)
		}
	}
}

// K6 on a stand-in: a library put in with DYLD_INSERT_LIBRARIES runs in a program without the hardened runtime, and
// isn't loaded into the same program with it.
func TestAnInjectedLibraryIsNotLoadedIntoAHardenedProgram(t *testing.T) {
	needTools(t, "codesign", "cc")
	folder := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(folder, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	program, library := filepath.Join(folder, "standin"), filepath.Join(folder, "inject.dylib")
	for _, build := range [][]string{
		{"-o", program, write("standin.c", "#include <stdio.h>\nint main(void) { puts(\"standin ran\"); return 0; }\n")},
		{"-dynamiclib", "-o", library, write("inject.c",
			"#include <unistd.h>\n__attribute__((constructor)) static void in(void) { write(1, \"INJECTED\\n\", 9); }\n")},
	} {
		if out, err := exec.Command("cc", build...).CombinedOutput(); err != nil {
			t.Skipf("cc can't build the stand-ins here: %v\n%s", err, out)
		}
	}
	run := func() string {
		cmd := exec.Command(program)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "DYLD_INSERT_LIBRARIES=" + library}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		return string(out)
	}
	adHoc(t, library)
	adHoc(t, program)
	if out := run(); !strings.Contains(out, "INJECTED") || !strings.Contains(out, "standin ran") {
		t.Fatalf("without the runtime the library should run (the check means nothing otherwise): %q", out)
	}
	adHoc(t, program, "--options", "runtime")
	if out := run(); strings.Contains(out, "INJECTED") || !strings.Contains(out, "standin ran") {
		t.Errorf("with the runtime the library ran: %q", out)
	}
}

// build-app.sh signs the app, the command, tmux and smolcorp-spawn with the runtime, the app with its entitlements,
// and checks all four; the test copies' scripts keep it when they sign again.
func TestEveryMacBuildIsSignedWithTheRuntimeAndChecked(t *testing.T) {
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	build := read("build-app.sh")
	if !strings.Contains(build, "SIGN=(codesign --force --options runtime)") {
		t.Error("build-app.sh's codesign has no --options runtime")
	}
	// Every signature build-app.sh makes goes through SIGN: no bare codesign that signs.
	for _, line := range strings.Split(build, "\n") {
		if strings.Contains(line, "codesign ") && strings.Contains(line, "--sign") && !strings.Contains(line, "SIGN=") {
			t.Errorf("build-app.sh signs without SIGN: %s", strings.TrimSpace(line))
		}
	}
	if strings.Count(build, `"${SIGN[@]}" --sign`) != 8 || strings.Count(build, `--entitlements "$ENTITLEMENTS" "$APP"`) != 2 ||
		strings.Count(build, `--identifier io.github.sauzze.smolcorp-spawn "$APP/Contents/Resources/bin/smolcorp-spawn"`) != 2 {
		t.Error("build-app.sh should sign the command, tmux, smolcorp-spawn and the app (with its entitlements), with an identity and ad hoc")
	}
	if !strings.Contains(build, `scripts/check-runtime.sh "${SIGNED[@]}" || {`) ||
		!regexp.MustCompile(`SIGNED\+=\("\$APP/Contents/Resources/bin/smolcorp"\)`).MatchString(build) ||
		!regexp.MustCompile(`SIGNED\+=\("\$APP/Contents/Resources/bin/tmux"\)`).MatchString(build) ||
		!strings.Contains(build, `SIGNED=("$APP" "$APP/Contents/Resources/bin/smolcorp-spawn")`) {
		t.Error("build-app.sh should check the app, the command, tmux and smolcorp-spawn with check-runtime.sh, and stop when it fails")
	}
	for _, name := range []string{"test-copy.sh", "demo-page-run.sh"} {
		for _, line := range strings.Split(read(name), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "codesign --force") && !strings.Contains(line, "--options runtime") {
				t.Errorf("%s signs a copy without the runtime: %s", name, strings.TrimSpace(line))
			}
		}
	}
	if !strings.Contains(read("test-copy.sh"), `SMOLCORP_SIGN_IDENTITY="$TEST_IDENTITY" scripts/build-app.sh`) {
		t.Error("a test copy must never be signed with the owner's identity")
	}
}

// buildAppCheckSigning runs build-app.sh's dry run with a home of the test's own: it signs, builds, unlocks and
// locks nothing, and with SMOLCORP_SIGN_IDENTITY=- or a signing keychain named asks no keychain anything.
func buildAppCheckSigning(t *testing.T, home string, env ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(filepath.Join(repoRoot(t), "scripts", "build-app.sh"), "--check-signing")
	cmd.Env = append([]string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + home}, env...)
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), string(out)
	} else if err != nil {
		t.Fatal(err)
	}
	return 0, string(out)
}

func TestBuildAppsDryRunSaysHowItSignsAndNeverNamesTheLoginKeychain(t *testing.T) {
	home := t.TempDir()
	code, out := buildAppCheckSigning(t, home, "SMOLCORP_SIGN_IDENTITY=-")
	if code != 0 || !strings.Contains(out, "Signing: ad hoc") || !strings.Contains(out, "Hardened runtime: on") {
		t.Errorf("ad hoc: %d, %q", code, out)
	}

	// A keychain of its own: unlocked to sign, locked again at the end.
	keychains := filepath.Join(home, "Library", "Keychains")
	if err := os.MkdirAll(keychains, 0o700); err != nil {
		t.Fatal(err)
	}
	own := filepath.Join(t.TempDir(), "signing.keychain-db")
	if err := os.WriteFile(own, []byte("stand-in"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out = buildAppCheckSigning(t, home, "SMOLCORP_SIGN_IDENTITY=Smolcorp Stand-in", "SMOLCORP_SIGN_KEYCHAIN="+own)
	if code != 0 || !strings.Contains(out, "from "+own) || !strings.Contains(out, "locked again when the build ends") {
		t.Errorf("its own keychain: %d, %q", code, out)
	}

	// The login keychain, by its name or through a link to it: refused, as the build would lock it.
	login := filepath.Join(keychains, "login.keychain-db")
	if err := os.WriteFile(login, []byte("stand-in"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "signing.keychain-db")
	if err := os.Symlink(login, link); err != nil {
		t.Fatal(err)
	}
	for _, named := range []string{login, "login.keychain", link} {
		code, out := buildAppCheckSigning(t, home, "SMOLCORP_SIGN_IDENTITY=Smolcorp Stand-in", "SMOLCORP_SIGN_KEYCHAIN="+named)
		if code != 1 || !strings.Contains(out, "names your login keychain") {
			t.Errorf("%s: %d, %q", named, code, out)
		}
	}
	code, out = buildAppCheckSigning(t, home, "SMOLCORP_SIGN_IDENTITY=Smolcorp Stand-in", "SMOLCORP_SIGN_KEYCHAIN="+filepath.Join(home, "none"))
	if code != 1 || !strings.Contains(out, "there's no keychain") {
		t.Errorf("a keychain that isn't there: %d, %q", code, out)
	}
}

// The core starts only while the app holds the folder's instance lock: found free, it refuses.
func TestTheCoreStartsOnlyWhileTheAppHoldsTheInstanceLock(t *testing.T) {
	isolate(t)
	if held, err := appHoldsInstance(); held || err == nil {
		t.Errorf("no lock file: %v, %v", held, err)
	}
	if err := os.WriteFile(instanceFile(), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if held, err := appHoldsInstance(); held || err == nil {
		t.Errorf("a lock file nobody holds: %v, %v", held, err)
	}
	holdLock(t) // as the app holds it
	if held, err := appHoldsInstance(); !held || err != nil {
		t.Errorf("held: %v, %v", held, err)
	}
}

// K5: the smolcorp program never asks the Keychain: a program of yours could run it, and a trusted program serves
// whoever runs it. Only Smolcorp.app reads Smolcorp's items, and hands the core what it needs over the bridge.
// go-keyring is linked into the Mac build for its ErrNotFound only: its Get, Set and Delete run /usr/bin/security
// there, so none may be in a file built for a Mac (the Linux and Windows stores are //go:build !darwin).
func TestTheSmolcorpProgramNeverAsksTheKeychain(t *testing.T) {
	asks := regexp.MustCompile(`SecItem\w*\(|SecKeychain\w*\(|"find-(generic|internet)-password"|exec\.Command(Context)?\([^)]*"(/usr/bin/)?security"`)
	asksOnAMac := regexp.MustCompile(`keyring\.(Get|Set|Delete|DeleteAll)\(`)
	mac := build.Default
	mac.GOOS, mac.GOARCH, mac.CgoEnabled = "darwin", "arm64", true
	for _, dir := range []string{".", "../cmd/smolcorp", "../internal/guard", "../internal/gm", "../internal/acp", "../internal/appdirs", "../internal/procmem"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if found := asks.Find(data); found != nil {
				t.Errorf("%s asks the Keychain: %s", file, found)
			}
			onAMac, err := mac.MatchFile(filepath.Dir(file), filepath.Base(file))
			if err != nil {
				t.Fatal(err)
			}
			if found := asksOnAMac.Find(data); onAMac && found != nil {
				t.Errorf("%s is built for a Mac and asks the Keychain through go-keyring: %s", file, found)
			}
		}
	}
}
