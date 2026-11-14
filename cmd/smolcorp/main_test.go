package main

import (
	"os"
	"regexp"
	"testing"
)

// The app, `serve` and the guard keep their memory from the user's other programs first thing, before anything
// holds a secret (internal/procmem): the package's rows test the calls, this one that main still makes them.
func TestTheAppServeAndTheGuardProtectTheirMemoryFirst(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for what, wiring := range map[string]string{
		"the app": `if len\(args\) == 0 \{\s*protect\(procmem\.NotDumpable\)\s*os\.Exit\(runApp\(\)\)`,
		"serve":   `case "serve":\s*protect\(procmem\.NotDumpable, procmem\.DenyMemoryAccess\)\s*os\.Exit\(runServer\(\)\)`,
		"guard":   `case "guard":\s*(//[^\n]*\s*)*protect\(procmem\.NotDumpable\)\s*os\.Exit\(guard\.Run\(`,
	} {
		if !regexp.MustCompile(wiring).Match(data) {
			t.Errorf("%s no longer protects its memory before it starts (cmd/smolcorp/main.go)", what)
		}
	}
}
