//go:build !windows

package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests/fixtures/frontdesk/hire-settings.json is a hire's settings file, the
// fence, as the Mac's HireSettings writes it for fixed inputs
// (HireSettingsFixtureTests.swift writes it). This engine must write the same
// bytes from the same inputs, so a Mac hire gets one fence whichever engine
// starts it. (Windows writes its paths as //c/…, so the Mac's paths aren't its.)
func TestTheFenceIsTheMacsByteForByte(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "frontdesk", "hire-settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		CLI   string `json:"cli"`
		Cases []struct {
			Name      string              `json:"name"`
			HireID    string              `json:"hireID"`
			Profile   string              `json:"profile"`
			Auto      bool                `json:"auto"`
			Printed   string              `json:"printed"`
			Support   string              `json:"support"`
			Profiles  []map[string]string `json:"profiles"`
			Others    []string            `json:"others"`
			Entries   []string            `json:"entries"`
			Reporting map[string]any      `json:"reporting"`
			File      string              `json:"file"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) < 3 {
		t.Fatalf("%d cases in the fixture", len(fixture.Cases))
	}
	for _, c := range fixture.Cases {
		profile := guardProfile(c.Profile)
		rules := filepath.Join(c.Support, hireGuardFile)
		groups := guardGroups(fixture.CLI, rules, profile, func(string, []string) (int, string, bool) { return 0, c.Printed, true })
		if groups == nil {
			t.Fatalf("%s: the groups weren't read", c.Name)
		}
		var profiles []claudeProfile
		for _, p := range c.Profiles {
			profiles = append(profiles, claudeProfile{Name: p["name"], Dir: p["dir"]})
		}
		deny := hireDenyRules(c.HireID, groups, c.Support, profiles, c.Others, c.Entries)
		got := string(swiftJSON(hireSettingsValue(deny, guardHookCommand(fixture.CLI, rules, profile), c.Auto, c.Reporting)))
		if got != c.File {
			gotLines, wantLines := strings.Split(got, "\n"), strings.Split(c.File, "\n")
			for i := range min(len(gotLines), len(wantLines)) {
				if gotLines[i] != wantLines[i] {
					t.Errorf("%s: line %d is\n  %s\nthe Mac's is\n  %s", c.Name, i+1, gotLines[i], wantLines[i])
					break
				}
			}
			t.Errorf("%s: %d lines, the Mac's %d", c.Name, len(gotLines), len(wantLines))
		}
	}
}

// What writeHireSettings writes is that encoding, whole, with no newline after it.
func TestTheSettingsFileIsWrittenInTheMacsEncoding(t *testing.T) {
	r := newHireSettingsRig(t)
	path, err := r.write("abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(written, &object); err != nil {
		t.Fatal(err)
	}
	if string(swiftJSON(object)) != string(written) {
		t.Error("the file isn't in the Mac's encoding")
	}
	if !strings.Contains(string(written), `"disableAllHooks" : false`) || strings.HasSuffix(string(written), "\n") {
		t.Errorf("the file:\n%s", written)
	}
	deny := object["permissions"].(map[string]any)["deny"].([]any)
	has := func(rule string) bool {
		for _, d := range deny {
			if d == rule {
				return true
			}
		}
		return false
	}
	root := absoluteRulePath(r.support)
	for _, name := range []string{"gm-session", "hook-header", "hook-key", "window-key"} {
		if !has("Read("+root+"/"+name+")") || !has("Edit("+root+"/"+name+")") {
			t.Errorf("%s isn't denied", name)
		}
	}
}

// The writer, against what JSONSerialization does with the shapes a file can hold.
func TestSwiftJSONWritesAsJSONSerialization(t *testing.T) {
	got := string(swiftJSON(map[string]any{"b": []any{}, "a": map[string]any{}, "c": []string{"x/y", "é\t\"\\\u0001"}, "d": 10, "e": nil, "f": 1.5}))
	want := "{\n  \"a\" : {\n\n  },\n  \"b\" : [\n\n  ],\n  \"c\" : [\n    \"x/y\",\n    \"é\\t\\\"\\\\\\u0001\"\n  ],\n  \"d\" : 10,\n  \"e\" : null,\n  \"f\" : 1.5\n}"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
