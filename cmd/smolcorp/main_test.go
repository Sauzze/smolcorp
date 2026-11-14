package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/gm"
)

// The guard keeps its memory from the user's other programs first thing, before it reads anything
// (internal/procmem): the package's rows test the calls, this one that main still makes them. The app and `serve`
// do it as their own first step and fail closed (desktop/memory_test.go): main hands over to them with nothing
// in between.
func TestTheAppServeAndTheGuardProtectTheirMemoryFirst(t *testing.T) {
	for file, wirings := range map[string]map[string]string{
		"main.go": {
			"the app": `if len\(args\) == 0 \{\s*(//[^\n]*\s*)*os\.Exit\(runApp\(\)\)`,
			"serve":   `case "serve":\s*os\.Exit\(runServer\(\)\)`,
			"guard":   `case "guard":\s*(//[^\n]*\s*)*protect\(procmem\.NotDumpable\)\s*os\.Exit\(guard\.Run\(`,
		},
		"start_other.go": {
			"the app": `func runApp\(\) int\s*\{ return desktop\.RunApp\(\) \}`,
			"serve":   `func runServer\(\) int \{ return desktop\.RunServer\(\) \}`,
		},
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for what, wiring := range wirings {
			if !regexp.MustCompile(wiring).Match(data) {
				t.Errorf("%s no longer protects its memory before it starts (cmd/smolcorp/%s)", what, file)
			}
		}
	}
}

// RMSY126k nit 6: `smolcorp gm login` checks the program it would start as the GM with desktop's own check (SG2), as
// smolcorp run does. On a Mac, a stand-in named claude that Anthropic didn't sign is refused; it is only checked
// (Apple's codesign reads it), never run.
func TestGMLoginChecksTheProgramAsEveryAgentStartIsChecked(t *testing.T) {
	if gm.CheckProgram == nil {
		t.Fatal("gm login has no check: it would start nothing")
	}
	standIn := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(standIn, []byte("#!/bin/sh\nexit 9\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := gm.CheckProgram("codex", standIn); err != nil {
		t.Errorf("another agent, which nobody signs, is refused: %v", err)
	}
	if runtime.GOOS != "darwin" {
		return // off a Mac, nothing is checked (desktop/agentcheck_other.go)
	}
	for _, word := range []string{"claude", standIn} {
		if err := gm.CheckProgram(word, standIn); err == nil || !strings.Contains(err.Error(), "isn't signed by Anthropic") {
			t.Errorf("%s: an unsigned claude passes: %v", word, err)
		}
	}
}
