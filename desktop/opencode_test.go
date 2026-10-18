package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeOpenCode puts an opencode on the PATH that prints the version the test
// gives it, and says how to update it: the program file changes, as an
// update changes it.
func fakeOpenCode(t *testing.T) (setVersion func(string), update func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in opencode is a shell script")
	}
	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	version := filepath.Join(bin, "version")
	program := filepath.Join(bin, "opencode")
	writeFile(t, program, "#!/bin/sh\ncat \""+version+"\"\n")
	if err := os.Chmod(program, 0o755); err != nil {
		t.Fatal(err)
	}
	changes := 0
	return func(v string) { writeFile(t, version, v+"\n") }, func() {
		changes++
		later := time.Now().Add(time.Duration(changes) * time.Minute)
		if err := os.Chtimes(program, later, later); err != nil {
			t.Fatal(err)
		}
	}
}

// writtenFor says which OpenCode the plugin on disk is for.
func writtenFor(t *testing.T) string {
	t.Helper()
	plugin, err := os.ReadFile(openCodePlugin())
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case strings.Contains(string(plugin), "export const Smolcorp"):
		return "OpenCode 1"
	case strings.Contains(string(plugin), "export default {"):
		return "OpenCode 2"
	}
	return "neither"
}

func TestOpenCodesPluginIsWrittenForTheVersionInstalled(t *testing.T) {
	isolate(t)
	setVersion, update := fakeOpenCode(t)
	setVersion("1.14.0")
	if err := writePlugin(); err != nil {
		t.Fatal(err)
	}
	if got := writtenFor(t); got != "OpenCode 1" {
		t.Fatalf("with OpenCode 1.14.0 the plugin is for %s", got)
	}
	// The version alone changing, the program not, isn't looked into.
	setVersion("opencode v2.0.32")
	lookAtOpenCode()
	if got := writtenFor(t); got != "OpenCode 1" {
		t.Fatalf("with the program unchanged the plugin became for %s", got)
	}
	// OpenCode updated: the next look writes the plugin for 2.
	update()
	lookAtOpenCode()
	if got := writtenFor(t); got != "OpenCode 2" {
		t.Fatalf("after updating to 2.0.32 the plugin is for %s", got)
	}
	// A version that can't be read gets the default, which OpenCode 2, and 1.3 and later, load.
	setVersion("something else")
	update()
	lookAtOpenCode()
	if got := writtenFor(t); got != "OpenCode 2" {
		t.Fatalf("with no version to read the plugin is for %s", got)
	}
}

func TestTheVersionIsReadAsOpenCodePrintsIt(t *testing.T) {
	for text, want := range map[string]int{"opencode v2.0.32\n": 2, "1.14.0": 1, "v10.2.1": 10} {
		if got, ok := majorVersionIn(text); !ok || got != want {
			t.Errorf("%q reads as %d, %v; want %d", text, got, ok, want)
		}
	}
	for _, text := range []string{"", "opencode", "version 2"} {
		if _, ok := majorVersionIn(text); ok {
			t.Errorf("%q reads as a version", text)
		}
	}
}

func TestDisconnectingOpenCodeStaysDisconnected(t *testing.T) {
	isolate(t)
	setVersion, update := fakeOpenCode(t)
	setVersion("opencode v2.0.32")
	if err := writePlugin(); err != nil {
		t.Fatal(err)
	}
	forgetOpenCode()
	// The look once a minute, after OpenCode updated, doesn't bring the plugin back.
	update()
	lookAtOpenCode()
	if _, err := os.Stat(openCodePlugin()); !os.IsNotExist(err) {
		t.Fatal("the plugin came back after disconnecting")
	}
	// Connecting again writes it afresh, whole: no part-written file is left beside it.
	if err := writePlugin(); err != nil {
		t.Fatal(err)
	}
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(openCodePlugin()), ".smolcorp-*"))
	if writtenFor(t) != "OpenCode 2" || len(left) != 0 {
		t.Errorf("connecting again wrote it for %s, and left %v", writtenFor(t), left)
	}
}
