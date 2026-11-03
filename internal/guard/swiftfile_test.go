package guard

import (
	"path/filepath"
	"reflect"
	"testing"
)

// The Mac's setup page writes hire-guard.json (Tests/AgentDeckTests/GuardFileTests.swift
// keeps this fixture up to date): cloud switched off and one pattern of the
// owner's own. The guard must read what the Swift side wrote, and the
// Swift side's six groups must be the guard's defaults.
func TestTheGuardReadsTheFileTheMacWrote(t *testing.T) {
	path := filepath.Join("..", "..", "Tests", "fixtures", "frontdesk", "hire-guard.json")
	groups, problem := Load(path)
	if problem != "" {
		t.Fatalf("the guard can't read the Mac's file: %s", problem)
	}
	defaults := DefaultGroups()
	if len(groups) != len(defaults)+1 {
		t.Fatalf("%d groups, want the %d defaults and the owner's own", len(groups), len(defaults))
	}
	for i, d := range defaults {
		got := groups[i]
		if got.Name != d.Name || !reflect.DeepEqual(got.Commands, d.Commands) {
			t.Errorf("group %d: the Mac wrote %q %v, the guard's default is %q %v (update GuardFile.groups)", i, got.Name, got.Commands, d.Name, d.Commands)
		}
		if want := d.Name != "cloud"; got.On != want {
			t.Errorf("group %q: on is %v, want %v", got.Name, got.On, want)
		}
	}
	extra := groups[len(groups)-1]
	if extra.Name != "extra" || !extra.On || !reflect.DeepEqual(extra.Commands, []Rule{"terraform import"}) {
		t.Fatalf("the owner's patterns: %+v", extra)
	}
	for line, group := range map[string]string{"git push": "push", "terraform import x": "extra", "vercel deploy": "deploy"} {
		if r := Check(line, groups); r == nil || r.Group != group {
			t.Errorf("%q: refused by %+v, want %s", line, r, group)
		}
	}
	if r := Check("aws s3 ls", groups); r != nil {
		t.Errorf("cloud is off in the file, yet aws was refused: %+v", r)
	}
}
