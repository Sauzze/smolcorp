//go:build unix

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// fakePass writes a program that says every test of the stand-in table passed, running nothing, as go test's -exec
// (or a toolchain put in go's place) would run it, and gives its path. It notes each run in ran.
func fakePass(t *testing.T, ran string) string {
	t.Helper()
	var text strings.Builder
	text.WriteString("#!/bin/sh\necho \"$0 $*\" >> '" + ran + "'\n")
	for _, test := range standinTable {
		text.WriteString("echo '=== RUN   " + test.name + "'\necho '--- PASS: " + test.name + " (0.00s)'\n")
	}
	text.WriteString("echo PASS\n")
	path := filepath.Join(t.TempDir(), "fake-pass")
	if err := os.WriteFile(path, []byte(text.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// standinGoroot is a copy of the real go's toolchain, its go program copied and the rest linked, with goEnv as the
// toolchain's own go.env: its go finds it as its GOROOT. It gives the copy's go.
func standinGoroot(t *testing.T, goEnv string) string {
	t.Helper()
	cmd := exec.Command(realGo(t), "env", "GOROOT")
	cmd.Env = append(os.Environ(), "GOROOT=")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	real := strings.TrimSpace(string(out))
	root := t.TempDir()
	entries, err := os.ReadDir(real)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if name := entry.Name(); name != "bin" && name != "go.env" {
			if err := os.Symlink(filepath.Join(real, name), filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	program := filepath.Join(root, "bin", "go")
	if err := os.MkdirAll(filepath.Dir(program), 0o755); err != nil {
		t.Fatal(err)
	}
	from, err := os.Open(filepath.Join(real, "bin", "go"))
	if err != nil {
		t.Fatal(err)
	}
	defer from.Close()
	to, err := os.OpenFile(program, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(to, from); err != nil {
		t.Fatal(err)
	}
	if err := to.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.env"), []byte(goEnv), 0o644); err != nil {
		t.Fatal(err)
	}
	// The copy reads it: with nothing set, its go.env's GOFLAGS is in force.
	check := exec.Command(program, "env", "GOROOT", "GOFLAGS")
	check.Env = append(os.Environ(), "GOROOT=", "GOFLAGS=", "GOENV=off")
	if out, err := check.Output(); err != nil || !strings.HasPrefix(string(out), root+"\n") {
		t.Fatalf("the stand-in toolchain isn't its own GOROOT: %v\n%s", err, out)
	}
	return program
}

// standinTool writes a program that notes its run in ran and fails, as name in a folder of its own, and gives its
// path: a C compiler or linker the caller names, which must never be the one that builds or links the tests.
func standinTool(t *testing.T, ran, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho \"$0 $*\" >> '"+ran+"'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// callerMark is in the name of each folder of stand-in files the caller names (libraries, headers, a changed copy of a
// module), and in the stand-in files themselves: the gate's output holds it only if the build read one of them.
const callerMark = "named-by-the-caller"

// madeUp is a setting nobody has thought of, made up here: the allow-list holds for it too.
const madeUp = "SMOLCORP_MADE_UP_SETTING"

// callerFolder writes files (path → text) in a fresh folder whose name holds callerMark, and gives the folder.
func callerFolder(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), callerMark)
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// standinLibraries are the libraries the C compiler links a test binary with, on a Mac and on Linux, each a line of
// text: read by the linker, it says so with the file's path, and it never runs.
func standinLibraries() map[string]string {
	files := map[string]string{}
	for _, name := range []string{"libSystem", "libresolv", "libpthread", "libc", "libobjc", "libdl", "libm"} {
		for _, kind := range []string{".tbd", ".dylib", ".so", ".a"} {
			files[name+kind] = "stand-in library " + callerMark + "\n"
		}
	}
	return files
}

// standinHeaders are the C headers cgo's own code includes, each an #error line: read by the compiler, it stops it
// with that line.
func standinHeaders() map[string]string {
	files := map[string]string{}
	for _, name := range []string{"stdlib.h", "stdio.h", "string.h", "stdint.h", "stddef.h", "errno.h", "pthread.h",
		"unistd.h", "signal.h"} {
		files[name] = "#error \"stand-in header " + callerMark + "\"\n"
	}
	return files
}

// deskCgo is a file of the stand-in package that uses cgo, as the desktop package does on a Mac: its test binary is
// built with the C compiler and linked by it.
const deskCgo = "package desk\n\n// static int two(void) { return 2; }\nimport \"C\"\n\nfunc two() int { return int(C.two()) }\n"

// A failing table stops the release whatever go settings the caller's environment, a go env file, go.mod or the
// toolchain's own go.env carry (RT126SAX11 B1), whatever C compiler, linker or flags the caller's environment or PATH
// name (RT126SAX11R2 B3), and whatever folders of libraries or headers, caches or other settings the caller's
// environment holds (RT126SAX11R3 B4): none of them can run a stand-in that says the tests passed, another toolchain,
// or a C compiler or linker that writes the test binary, in place of the tests, have a file of the caller's read into
// that binary, or reach go at all.
func TestACallersGoSettingsCantPassAFailingTable(t *testing.T) {
	failing := func() map[string]string {
		files := whole()
		files["a_test.go"] = testFile("", "desk", map[string]string{"TestEverywhere": `t.Fatal("forged event let in")`})
		files["cgo.go"] = deskCgo
		return files
	}
	const stop = "TestEverywhere failed or was skipped"
	for _, c := range []struct {
		name string
		// set sets the case up in dir, the stand-in module, with ran the file each stand-in notes its runs in, and
		// gives the go check runs (the real one, or a stand-in toolchain's).
		set  func(t *testing.T, dir, ran string) string
		want string
	}{
		{"the caller's GOFLAGS runs a stand-in with -exec", func(t *testing.T, dir, ran string) string {
			t.Setenv("GOFLAGS", "-exec="+fakePass(t, ran))
			return realGo(t)
		}, stop},
		{"the caller's GOFLAGS puts a passing test in place of the failing one with -overlay", func(t *testing.T, dir, ran string) string {
			passing := filepath.Join(t.TempDir(), "a_test.go")
			if err := os.WriteFile(passing, []byte(whole()["a_test.go"]), 0o644); err != nil {
				t.Fatal(err)
			}
			overlay := filepath.Join(t.TempDir(), "overlay.json")
			text := `{"Replace": {"` + filepath.Join(dir, "desk", "a_test.go") + `": "` + passing + `"}}`
			if err := os.WriteFile(overlay, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GOFLAGS", "-overlay="+overlay)
			return realGo(t)
		}, stop},
		{"a go env file's GOFLAGS runs a stand-in with -exec", func(t *testing.T, dir, ran string) string {
			env := filepath.Join(t.TempDir(), "go-env")
			if err := os.WriteFile(env, []byte("GOFLAGS=-exec="+fakePass(t, ran)+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GOFLAGS", "")
			t.Setenv("GOENV", env)
			return realGo(t)
		}, stop},
		{"the toolchain's own go.env runs a stand-in with -exec", func(t *testing.T, dir, ran string) string {
			t.Setenv("GOFLAGS", "")
			return standinGoroot(t, "GOFLAGS=-exec="+fakePass(t, ran)+"\n")
		}, stop},
		{"the caller's GOOS builds for another system, run by a stand-in go_<system>_<arch>_exec", func(t *testing.T, dir, ran string) string {
			other := "linux"
			if runtime.GOOS == "linux" {
				other = "darwin"
			}
			bin := t.TempDir()
			if err := os.Rename(fakePass(t, ran), filepath.Join(bin, "go_"+other+"_"+runtime.GOARCH+"_exec")); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GOOS", other)
			t.Setenv("CGO_ENABLED", "0")
			return realGo(t)
		}, stop},
		{"the caller's CC builds and links the test binary", func(t *testing.T, dir, ran string) string {
			t.Setenv("CC", standinTool(t, ran, "cc"))
			return realGo(t)
		}, stop},
		{"the caller's CGO_LDFLAGS name a stand-in linker with -fuse-ld", func(t *testing.T, dir, ran string) string {
			t.Setenv("CGO_LDFLAGS", "-fuse-ld="+standinTool(t, ran, "ld"))
			return realGo(t)
		}, stop},
		{"a cc, clang and gcc put first on PATH after the gate found its C compiler", func(t *testing.T, dir, ran string) string {
			bin := filepath.Dir(standinTool(t, ran, "cc"))
			for _, name := range []string{"clang", "gcc"} {
				if err := os.Symlink(filepath.Join(bin, "cc"), filepath.Join(bin, name)); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			return realGo(t)
		}, stop},
		{"COMPILER_PATH names a folder whose ld the C compiler would link with", func(t *testing.T, dir, ran string) string {
			t.Setenv("COMPILER_PATH", filepath.Dir(standinTool(t, ran, "ld")))
			return realGo(t)
		}, stop},
		{"CCC_OVERRIDE_OPTIONS adds a stand-in linker to clang's options", func(t *testing.T, dir, ran string) string {
			t.Setenv("CCC_OVERRIDE_OPTIONS", "# +-fuse-ld="+standinTool(t, ran, "ld"))
			return realGo(t)
		}, stop},
		// The caller's library and header folders, searched before the SDK's (RT126SAX11R3 B4). Each go the gate runs
		// has a cold build cache (its own, fresh), so the headers would be read.
		{"the caller's LIBRARY_PATH names a folder of stand-in libraries the linker would take first", func(t *testing.T, dir, ran string) string {
			t.Setenv("LIBRARY_PATH", callerFolder(t, standinLibraries()))
			return realGo(t)
		}, stop},
		{"the caller's CPATH and the other include paths name a folder with a stand-in stdlib.h, the build cache cold", func(t *testing.T, dir, ran string) string {
			headers := callerFolder(t, standinHeaders())
			for _, name := range []string{"CPATH", "C_INCLUDE_PATH", "CPLUS_INCLUDE_PATH", "OBJC_INCLUDE_PATH", "OBJCPLUS_INCLUDE_PATH"} {
				t.Setenv(name, headers)
			}
			return realGo(t)
		}, stop},
		// The allow-list holds for settings nobody has thought of: one made up here never reaches the go the gate runs.
		{"a setting nobody has thought of, made up here, never reaches go", func(t *testing.T, dir, ran string) string {
			t.Setenv(madeUp, "the caller's")
			wrapper := filepath.Join(t.TempDir(), "go")
			body := "#!/bin/sh\nif [ -n \"${" + madeUp + "+set}\" ]; then echo \"$0 $*: " + madeUp + " reached go\" >> '" + ran + "'; fi\n" +
				"exec '" + realGo(t) + "' \"$@\"\n"
			if err := os.WriteFile(wrapper, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
			return wrapper
		}, stop},
		// go's caches are the gate's own, fresh ones (SIERRA, RT126SAX11R3): folders the caller prepared are left alone.
		{"the caller's GOCACHE, GOMODCACHE and GOPATH name folders prepared beforehand", func(t *testing.T, dir, ran string) string {
			for _, name := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
				prepared := callerFolder(t, map[string]string{"prepared": name + "\n"})
				t.Setenv(name, prepared)
				t.Cleanup(func() {
					if entries, _ := os.ReadDir(prepared); len(entries) != 1 {
						t.Errorf("go used the caller's %s (%s): it holds %d files", name, prepared, len(entries))
					}
				})
			}
			return realGo(t)
		}, stop},
		{"go.mod's toolchain line and the caller's GOTOOLCHAIN switch to a stand-in toolchain on PATH", func(t *testing.T, dir, ran string) string {
			addToolchainLine(t, dir)
			// It fakes go test's run, and hands anything else to the real go, which stays itself.
			bin := t.TempDir()
			body := "#!/bin/sh\necho \"$0 $*\" >> '" + ran + "'\nif [ \"$1\" = test ]; then exec '" + fakePass(t, ran) + "'; fi\n" +
				"GOTOOLCHAIN=local exec '" + realGo(t) + "' \"$@\"\n"
			if err := os.WriteFile(filepath.Join(bin, "go1.99.0"), []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GOTOOLCHAIN", "path")
			return realGo(t)
		}, stop},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := standinModule(t, failing())
			ran := filepath.Join(t.TempDir(), "ran")
			// The C compiler is found first, as the gate finds it at its start; then the case sets things up.
			use := realTools(t)
			use.goProgram = c.set(t, dir, ran)
			var out bytes.Buffer
			err := check(dir, "./desk", use, standinTable, &out)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("want a stop saying %q, got %v\n%s", c.want, err, out.String())
			}
			if stood, _ := os.ReadFile(ran); len(stood) > 0 {
				t.Errorf("a stand-in ran in place of go or the tests, or the caller's setting reached go:\n%s", stood)
			}
			if strings.Contains(out.String(), callerMark) || err != nil && strings.Contains(err.Error(), callerMark) {
				t.Errorf("the build read a file the caller named:\n%s", out.String())
			}
			// go built with the gate's own cache.
			if cache, _, _ := goFolders(use.work); !exists(filepath.Join(cache, "README")) {
				t.Errorf("go didn't build with the gate's own cache (%s)", cache)
			}
		})
	}

	// go.mod asking for another toolchain, the caller's GOTOOLCHAIN asking to download it, and no download possible:
	// this go runs the table all the same, and a whole one lets the release on.
	t.Run("go.mod's toolchain line with GOTOOLCHAIN=auto: no download, no switch", func(t *testing.T) {
		dir := standinModule(t, whole())
		addToolchainLine(t, dir)
		t.Setenv("GOTOOLCHAIN", "auto")
		var out bytes.Buffer
		if err := check(dir, "./desk", realTools(t), standinTable, &out); err != nil {
			t.Errorf("a whole table, go.mod asking for go1.99.0: %v\n%s", err, out.String())
		}
	})

	// The caller's SDKROOT, DEVELOPER_DIR and TOOLCHAINS naming an empty SDK, an empty developer folder and no
	// toolchain: the C compiler builds with the SDK the gate found, and a whole table lets the release on.
	t.Run("the caller's SDKROOT, DEVELOPER_DIR and TOOLCHAINS: the gate's own C compiler and SDK", func(t *testing.T) {
		files := whole()
		files["cgo.go"] = deskCgo
		dir := standinModule(t, files)
		use := realTools(t)
		t.Setenv("SDKROOT", t.TempDir())
		t.Setenv("DEVELOPER_DIR", t.TempDir())
		t.Setenv("TOOLCHAINS", "example.invalid.none")
		var out bytes.Buffer
		if err := check(dir, "./desk", use, standinTable, &out); err != nil {
			t.Errorf("a whole table, the caller naming an empty SDK: %v\n%s", err, out.String())
		}
	})
}

// addToolchainLine has the stand-in module's go.mod ask for a toolchain that doesn't exist.
func addToolchainLine(t *testing.T, dir string) {
	t.Helper()
	mod := filepath.Join(dir, "go.mod")
	text, err := os.ReadFile(mod)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mod, append(text, "\ntoolchain go1.99.0\n"...), 0o600); err != nil {
		t.Fatal(err)
	}
}

// exists says whether path is there.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Each go the gate runs gets its own settings and nothing else of the caller's (RT126SAX11R3 B4): with the caller's go
// and C settings, library and header folders, caches, proxy, loader settings and one made up here all set, pinned is
// exactly the gate's list, those no row above can show running a stand-in included (C++, the compile flags,
// pkg-config, gcc's GCC_EXEC_PREFIX, the Mac shim's DEVELOPER_DIR and TOOLCHAINS).
func TestTheGatesGoGetsOnlyItsOwnSettings(t *testing.T) {
	for _, name := range []string{"GOROOT", "GOFLAGS", "GOENV", "GOWORK", "GOTOOLCHAIN", "GOOS", "GOARCH", "GOCACHE",
		"GOMODCACHE", "GOPATH", "GOPROXY", "GOSUMDB", "GONOSUMDB", "GOPRIVATE", "GOINSECURE", "GOTMPDIR", "GOEXPERIMENT",
		"CGO_ENABLED", "CC", "CXX", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_FFLAGS", "CGO_LDFLAGS",
		"PKG_CONFIG", "DEVELOPER_DIR", "SDKROOT", "TOOLCHAINS", "COMPILER_PATH", "GCC_EXEC_PREFIX",
		"CCC_OVERRIDE_OPTIONS", "LIBRARY_PATH", "CPATH", "C_INCLUDE_PATH", "CPLUS_INCLUDE_PATH", "OBJC_INCLUDE_PATH",
		"OBJCPLUS_INCLUDE_PATH", "LD_LIBRARY_PATH", "DYLD_LIBRARY_PATH", "TMPDIR", "LANG", "LC_ALL", madeUp} {
		t.Setenv(name, "/example.invalid/caller")
	}
	use := tools{goProgram: "go", cc: "/example.invalid/pinned/cc", sdk: "/example.invalid/pinned/sdk",
		home: "/example.invalid/home", tmp: "/example.invalid/tmp", work: "/example.invalid/work", modules: "/example.invalid/downloads"}
	got := pinned("linux", use)
	want := []string{"HOME=/example.invalid/home", "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "TMPDIR=/example.invalid/tmp",
		"GOCACHE=/example.invalid/work/cache", "GOMODCACHE=/example.invalid/work/modules",
		"GOPATH=/example.invalid/work/gopath", "GOPROXY=file:///example.invalid/downloads", "GOSUMDB=off",
		"GOFLAGS=-mod=readonly", "GOENV=off", "GOWORK=off", "GOTOOLCHAIN=local", "LANG=C",
		"CC=/example.invalid/pinned/cc", "SDKROOT=/example.invalid/pinned/sdk", "GOOS=linux", "GOARCH=" + runtime.GOARCH}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("the gate's go gets:\n  %s\nwant only:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// Run alone, without the release's folders, the gate makes its own for go and removes them when it ends, passed or
// stopped, after saying how large they grew (SIERRA, RT126SAX11R3).
func TestTheGateRemovesTheFoldersItMadeForGo(t *testing.T) {
	failing := whole()
	failing["a_test.go"] = testFile("", "desk", map[string]string{"TestEverywhere": `t.Fatal("forged event let in")`})
	for _, c := range []struct {
		name  string
		files map[string]string
		stops bool
	}{{"a whole table", whole(), false}, {"a failing table", failing, true}} {
		t.Run(c.name, func(t *testing.T) {
			use := realTools(t)
			dir := standinModule(t, c.files)
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			t.Chdir(dir)
			var out bytes.Buffer
			err := run([]string{"-pkg", "./desk", "-go", use.goProgram, "-cc", use.cc, "-sdk", use.sdk, "-modules", use.modules},
				standinTable, &out)
			if (err != nil) != c.stops {
				t.Errorf("want a stop: %v; got %v\n%s", c.stops, err, out.String())
			}
			if left, _ := os.ReadDir(tmp); len(left) > 0 {
				t.Errorf("the gate left %s in %s", left[0].Name(), tmp)
			}
			if !strings.Contains(out.String(), "go's folders for the table ("+tmp) || !strings.Contains(out.String(), " KB, removed.") {
				t.Errorf("the gate didn't say how large its folders grew before it removed them:\n%s", out.String())
			}
		})
	}
}

// depModule writes example.invalid/dep v1.0.0, with code as its one file, as go's download folder holds a module (its
// zip, go.mod and info), in a fresh folder. It gives that folder and the module's go.sum lines.
func depModule(t *testing.T, code string) (string, string) {
	t.Helper()
	const mod = "module example.invalid/dep\n\ngo 1.22\n"
	prefix := "example.invalid/dep@v1.0.0/"
	files := map[string]string{prefix + "go.mod": mod, prefix + "dep.go": code}
	var zipped bytes.Buffer
	w := zip.NewWriter(&zipped)
	for _, name := range []string{prefix + "dep.go", prefix + "go.mod"} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(files[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	downloads := t.TempDir()
	at := filepath.Join(downloads, "example.invalid", "dep", "@v")
	if err := os.MkdirAll(at, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"list": []byte("v1.0.0\n"), "v1.0.0.mod": []byte(mod), "v1.0.0.zip": zipped.Bytes(),
		"v1.0.0.info": []byte(`{"Version":"v1.0.0","Time":"2020-01-01T00:00:00Z"}`)} {
		if err := os.WriteFile(filepath.Join(at, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sum := "example.invalid/dep v1.0.0 " + hash1(files) + "\nexample.invalid/dep v1.0.0/go.mod " + hash1(map[string]string{"go.mod": mod}) + "\n"
	return downloads, sum
}

// hash1 is go.sum's h1: hash of files (name → text).
func hash1(files map[string]string) string {
	var names []string
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	h := sha256.New()
	for _, name := range names {
		fmt.Fprintf(h, "%x  %s\n", sha256.Sum256([]byte(files[name])), name)
	}
	return "h1:" + base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// The gate's modules come from its download folder, offline, each unpacked into its own module cache only once its zip
// matches go.sum (SIERRA, RT126SAX11R3): a whole table whose package needs a module passes with no network, a download
// go.sum doesn't name stops it, and a changed copy the caller's module cache holds is never built.
func TestTheGatesModulesComeFromItsDownloadsCheckedAgainstGoSum(t *testing.T) {
	const two = "package dep\n\nfunc Two() int { return 2 }\n"
	changed := "package dep\n\nfunc Two() int { return \"" + callerMark + "\" }\n"
	downloads, sum := depModule(t, two)
	// module is the stand-in module with a whole table, its package needing example.invalid/dep.
	module := func(t *testing.T) string {
		files := whole()
		files["dep.go"] = "package desk\n\nimport \"example.invalid/dep\"\n\nvar two = dep.Two()\n"
		dir := standinModule(t, files)
		mod := "module example.invalid/standin\n\ngo 1.22\n\nrequire example.invalid/dep v1.0.0\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.sum"), []byte(sum), 0o600); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	t.Run("offline, from the download folder, into the gate's own module cache", func(t *testing.T) {
		use := realTools(t)
		use.modules = downloads
		var out bytes.Buffer
		if err := check(module(t), "./desk", use, standinTable, &out); err != nil {
			t.Errorf("a whole table needing a module in the download folder: %v\n%s", err, out.String())
		}
		if _, modules, _ := goFolders(use.work); !exists(filepath.Join(modules, "example.invalid", "dep@v1.0.0", "dep.go")) {
			t.Errorf("the module isn't in the gate's own module cache (%s)", modules)
		}
	})
	t.Run("a download whose zip isn't the one go.sum names", func(t *testing.T) {
		use := realTools(t)
		use.modules, _ = depModule(t, changed)
		var out bytes.Buffer
		err := check(module(t), "./desk", use, standinTable, &out)
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch") || strings.Contains(err.Error()+out.String(), callerMark) {
			t.Errorf("want a stop on the changed download, never built; got %v\n%s", err, out.String())
		}
	})
	t.Run("the caller's GOMODCACHE holding a changed copy of the module, its hash file saying it is go.sum's", func(t *testing.T) {
		const mod = "module example.invalid/dep\n\ngo 1.22\n"
		zipSum := strings.Fields(sum)[2]
		prepared := callerFolder(t, map[string]string{
			"example.invalid/dep@v1.0.0/go.mod": mod, "example.invalid/dep@v1.0.0/dep.go": changed,
			"cache/download/example.invalid/dep/@v/v1.0.0.mod": mod, "cache/download/example.invalid/dep/@v/v1.0.0.ziphash": zipSum,
			"cache/download/example.invalid/dep/@v/v1.0.0.info": `{"Version":"v1.0.0","Time":"2020-01-01T00:00:00Z"}`,
			"cache/download/example.invalid/dep/@v/list":        "v1.0.0\n"})
		t.Setenv("GOMODCACHE", prepared)
		t.Setenv("GOPATH", filepath.Dir(prepared))
		use := realTools(t)
		use.modules = downloads
		var out bytes.Buffer
		err := check(module(t), "./desk", use, standinTable, &out)
		if err != nil || strings.Contains(out.String(), callerMark) {
			t.Errorf("want the module's own copy built and a whole table passed; got %v\n%s", err, out.String())
		}
	})
}
