// Package goline says whether a go is the one go.mod names: the same release line (major.minor) as go.mod's go
// line, at or past its patch. Every machine that checks or builds Smolcorp (CI, which installs exactly go.mod's, the
// Mac Studio, a Mac) runs one release line, so gofmt and the language read each file alike there: a go of another
// line laid desktop/preview_test.go out differently from CI's, and CI failed on it (FXCITRIAGE). A later patch of
// the same line is fine (the Studio's go1.27.4 beside go.mod's 1.27.2); an earlier one, another minor or another
// major isn't.
//
// scripts/goversion applies it for scripts/build-app.sh, and TestThisRepoIsTestedWithGoModsLine for every go test
// of ./... (CI's, the Studio's). scripts/release-gate.sh checks the same rule in its own shell, in its pins gate.
package goline

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	// goModLine is go.mod's go line: go 1.27 or go 1.27.2, alone on its line.
	goModLine = regexp.MustCompile(`(?m)^go[ \t]+([0-9]+)\.([0-9]+)(?:\.([0-9]+))?[ \t]*\r?$`)
	// goVersion is the version a go gives (runtime.Version, go env GOVERSION): go1.27.4, go1.28rc1 or
	// go1.27.4 X:something, read as far as its numbers go.
	goVersion = regexp.MustCompile(`^go([0-9]+)\.([0-9]+)(?:\.([0-9]+))?`)
)

// Version is a go release: major, minor and patch.
type Version struct{ Major, Minor, Patch int }

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// parts reads a match's three numbers, the patch 0 when it isn't there.
func parts(match []string) Version {
	var n [3]int
	for i, s := range match[1:4] {
		if s != "" {
			n[i], _ = strconv.Atoi(s)
		}
	}
	return Version{n[0], n[1], n[2]}
}

// FromGoMod is go.mod's go line.
func FromGoMod(goMod []byte) (Version, error) {
	match := goModLine.FindSubmatch(goMod)
	if match == nil {
		return Version{}, fmt.Errorf("go.mod has no go line (go 1.27.2)")
	}
	text := make([]string, len(match))
	for i, m := range match {
		text[i] = string(m)
	}
	return parts(text), nil
}

// FromGo is the release a go gives as its version.
func FromGo(version string) (Version, error) {
	match := goVersion.FindStringSubmatch(version)
	if match == nil {
		return Version{}, fmt.Errorf("%q isn't a go release's version", version)
	}
	return parts(match), nil
}

// Check says why the go whose version this is isn't go.mod's line, or nil when it is.
func Check(goMod []byte, version string) error {
	want, err := FromGoMod(goMod)
	if err != nil {
		return err
	}
	have, err := FromGo(version)
	if err != nil {
		return err
	}
	if have.Major != want.Major || have.Minor != want.Minor || have.Patch < want.Patch {
		return fmt.Errorf("this go is %s, and go.mod asks for go %d.%d.x from %s: install a go %d.%d at %s or later, then run this again",
			version, want.Major, want.Minor, want, want.Major, want.Minor, want)
	}
	return nil
}
