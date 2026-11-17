package goline

import (
	"os"
	"runtime"
	"testing"
)

// The rule, row by row: go.mod's release line, at or past its patch. The Studio's go1.27.4 and a Mac's go1.27.2 pass
// go.mod's 1.27.2; CI's 1.26.0 of 17 Nov, a 1.28 and an earlier patch don't.
func TestAGoIsGoModsLineAtOrPastItsPatch(t *testing.T) {
	const mod = "module example.invalid/m\n\ngo 1.27.2\n\nrequire (\n\tgolang.org/x/sys v0.49.0\n)\n"
	for _, c := range []struct {
		mod, version string
		ok           bool
	}{
		{mod, "go1.27.2", true},
		{mod, "go1.27.4", true},
		{mod, "go1.27.10", true},
		{mod, "go1.27.4 X:nocoverageredesign", true},
		{mod, "go1.27.1", false},
		{mod, "go1.27", false},
		{mod, "go1.26.0", false},
		{mod, "go1.28.0", false},
		{mod, "go1.28rc1", false},
		{mod, "go2.27.2", false},
		{mod, "devel go1.28-abcdef", false},
		{mod, "", false},
		{"module m\n\ngo 1.27\n", "go1.27.0", true},
		{"module m\n\ngo 1.27\n", "go1.27rc2", true},
		{"module m\r\n\r\ngo 1.27.2\r\n", "go1.27.2", true},
		{"module m\n\ntoolchain go1.27.2\n", "go1.27.2", false},
		{"module m\n\n// go 1.27.2\n", "go1.27.2", false},
		{"module m\n", "go1.27.2", false},
	} {
		if err := Check([]byte(c.mod), c.version); (err == nil) != c.ok {
			t.Errorf("go.mod %q, go %q: got %v, want ok %v", c.mod, c.version, err, c.ok)
		}
	}
	err := Check([]byte(mod), "go1.26.0")
	if err == nil || err.Error() != "this go is go1.26.0, and go.mod asks for go 1.27.x from 1.27.2: install a go 1.27 at 1.27.2 or later, then run this again" {
		t.Errorf("the refusal: %v", err)
	}
}

// This repo's go.mod and the go running its tests: the same line, so this row fails on a machine whose go drifted.
func TestThisRepoIsTestedWithGoModsLine(t *testing.T) {
	data, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(data, runtime.Version()); err != nil {
		t.Error(err)
	}
}
