//go:build !windows

package sandbox

import (
	"path/filepath"
	"testing"
)

// The report.sh the Mac app ships runs from the app (SA-X11): read-only to every desk, as it is in Smolcorp's
// install, which ResolvePrograms takes from the core's own program and the Policy denies whole. A desk can read and
// run it, and can't change, replace, move or link it. The copy an earlier version left in the support folder is
// denied with the folder, read as well as write.
func TestTheAppsReportShIsReadOnlyToEveryDesk(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t)
	core := s.program("Applications/Smolcorp.app/Contents/Resources/bin/smolcorp", "#!/bin/sh\n")
	report := s.program("Applications/Smolcorp.app/Contents/Resources/report.sh", "#!/bin/bash\n")
	ps := mustResolve(t, Finding{GOOS: "darwin", Home: s.home, Executable: core, DeveloperDir: s.at("no-developer-tools")})
	for _, c := range setups() {
		pl := c.pl
		pl.InstallRoots = append(ps.InstallRoots(), pl.InstallRoots...)
		p := mustNew(t, pl)
		for _, at := range []string{report, filepath.Dir(report)} {
			for op, want := range map[Op]Verdict{Read: Allow, Write: Deny, Move: Deny, Link: Deny} {
				if got, rule := p.Decide(at, op); got != want {
					t.Errorf("%s: %s, op %d: %v (%+v), want %v", c.name, at, op, got, rule, want)
				}
			}
		}
		for _, op := range []Op{Read, Write, Move, Link} {
			if got, rule := p.Decide(filepath.Join(pl.Support, "report.sh"), op); got != Deny {
				t.Errorf("%s: the support folder's copy, op %d: %v (%+v)", c.name, op, got, rule)
			}
		}
	}
}
