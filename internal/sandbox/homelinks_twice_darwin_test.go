package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// RT126POLISHAR3 N1, ruled (a): a folder reached in one round both by a two-level look and by a whole one is walked
// once, whole, so the shared count isn't spent on it twice. The reviewer's c6 home: ~/.claude's plugins/kit and
// skills/kit both lead to one flat folder of 95,000 files, ~/code/kit, with 15,000 more flat at the top of the
// plugins. Counted once, it fits the shared 200,000 and the start is made, as before FXPOLISHA; counted twice, it was
// refused, "stopped in ~/code/kit". c6b, the same two links to 90,000 files with 9,000 in the paste-cache, starts
// either way. Stand-in homes in the test's own folder, the Policy alone, at the real budgets.
func TestOnAMacAFolderAPluginAndASkillBothLeadToIsCountedOnce(t *testing.T) {
	if maxHomeEntries != 200_000 {
		t.Fatalf("the shared budget is %d: this row's sizes are the reviewer's, for 200,000", maxHomeEntries)
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fill := func(folder string, n int) {
		t.Helper()
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
		for i := range n {
			if err := os.WriteFile(filepath.Join(folder, fmt.Sprintf("f%d", i)), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, c := range []struct {
		name   string
		kit    int
		other  string
		others int
	}{
		{"c6", 95_000, ".claude/plugins", 15_000},
		{"c6b", 90_000, ".claude/paste-cache", 9_000},
	} {
		home := filepath.Join(base, c.name)
		fill(filepath.Join(home, "code", "kit"), c.kit)
		fill(filepath.Join(home, c.other), c.others)
		for _, folder := range []string{"plugins", "skills"} {
			if err := os.MkdirAll(filepath.Join(home, ".claude", folder), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		for _, l := range [][2]string{{".claude/plugins/kit", "../../code/kit"}, {".claude/skills/kit", "../../code/kit"}} {
			if err := os.Symlink(l[1], filepath.Join(home, l[0])); err != nil {
				t.Fatal(err)
			}
		}
		pl := Places{GOOS: "darwin", Home: home, Support: filepath.Join(home, "L", "AD"), OwnID: "h1", Root: filepath.Join(home, "P", "app")}
		p, err := New(pl)
		if err != nil {
			t.Errorf("%s: the start is refused: %v", c.name, err)
			continue
		}
		// What the link leads to is still held where it leads, the folder of a skill.
		if v, r := p.Decide(filepath.Join(home, "code", "kit", "f1"), Write); v != Deny {
			t.Errorf("%s: writing in the kit the skill leads to: %v (%s), want it held", c.name, v, r.Row)
		}
	}
}
