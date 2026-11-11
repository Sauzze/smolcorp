package desktop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What setup writes for "cloud off, one pattern of your own" is the Mac's file,
// byte for byte (GuardFileTests.swift writes the fixture; the guard reads it).
func TestSavingTheGuardWritesTheMacsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), hireGuardFile)
	if status, why := saveGuard([]byte(`{"groups":[{"name":"cloud","on":false}],"extra":["terraform import"]}`), path); status != 0 {
		t.Fatalf("%d %s", status, why)
	}
	written, _ := os.ReadFile(path)
	fixture, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "frontdesk", "hire-guard.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, fixture) {
		t.Errorf("the file isn't the Mac's:\n%s", written)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", info.Mode().Perm())
	}
	// Left out, a group keeps what it was, and so do your patterns.
	if status, _ := saveGuard([]byte(`{"groups":[{"name":"push","on":false}]}`), path); status != 0 {
		t.Fatal(status)
	}
	on, extra, problem := guardFileState(path)
	if on["cloud"] || on["push"] || !on["deploy"] || len(extra) != 1 || extra[0] != "terraform import" || problem != "" {
		t.Errorf("on %v, extra %v, %q", on, extra, problem)
	}
	// Patterns are trimmed, kept once each, in order; none is none.
	if status, _ := saveGuard([]byte(`{"extra":[" helm ","helm","kubectl drain"]}`), path); status != 0 {
		t.Fatal(status)
	}
	if _, extra, _ := guardFileState(path); strings.Join(extra, "|") != "helm|kubectl drain" {
		t.Errorf("extra %v", extra)
	}
	if status, _ := saveGuard([]byte(`{"extra":[]}`), path); status != 0 {
		t.Fatal(status)
	}
	if written, _ := os.ReadFile(path); !strings.Contains(string(written), "\"commands\" : [\n\n      ],\n      \"name\" : \"extra\"") {
		t.Errorf("no patterns isn't written as the Mac writes it:\n%s", written)
	}
}

// A group the page doesn't know is kept as it is; a broken file is replaced by the defaults and the change.
func TestSavingTheGuardKeepsWhatItDoesntKnow(t *testing.T) {
	path := filepath.Join(t.TempDir(), hireGuardFile)
	os.WriteFile(path, []byte(`{"groups":[{"name":"push","on":false,"commands":["git push"]},{"name":"mine","on":true,"commands":["make deploy"]}]}`), 0o600)
	if status, why := saveGuard([]byte(`{"groups":[{"name":"remote","on":false}]}`), path); status != 0 {
		t.Fatalf("%d %s", status, why)
	}
	written, _ := os.ReadFile(path)
	var file struct {
		Groups []struct {
			Name     string   `json:"name"`
			On       bool     `json:"on"`
			Commands []string `json:"commands"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(written, &file); err != nil || len(file.Groups) != 8 {
		t.Fatalf("%v: %s", err, written)
	}
	if g := file.Groups[0]; g.Name != "push" || g.On || strings.Join(g.Commands, "|") != "git push" {
		t.Errorf("push: %+v", g)
	}
	if g := file.Groups[4]; g.Name != "remote" || g.On {
		t.Errorf("remote: %+v", g)
	}
	if g := file.Groups[7]; g.Name != "mine" || strings.Join(g.Commands, "|") != "make deploy" {
		t.Errorf("the group it doesn't know: %+v", g)
	}
	os.WriteFile(path, []byte(`{"groups":"no"}`), 0o600)
	if status, _ := saveGuard([]byte(`{"groups":[{"name":"cloud","on":false}]}`), path); status != 0 {
		t.Fatal(status)
	}
	if on, _, problem := guardFileState(path); on["cloud"] || !on["push"] || problem != "" {
		t.Errorf("after a broken file: %v %q", on, problem)
	}
}

// Each refusal in the Mac's words, and the file left as it was.
func TestSavingTheGuardRefusesAsTheMacs(t *testing.T) {
	path := filepath.Join(t.TempDir(), hireGuardFile)
	saveGuard([]byte(`{"extra":["helm"]}`), path)
	before, _ := os.ReadFile(path)
	for body, why := range map[string]string{
		`[]`:                        "Send {groups: [{name, on}], extra: [pattern]}.",
		`{"groups":[],"other":1}`:   "Send {groups: [{name, on}], extra: [pattern]}.",
		`{"groups":{}}`:             "groups is a list of {name, on}.",
		`{"groups":[{"name":"x"}]}`: "Each group is {name, on}.",
		`{"groups":[{"name":"extra","on":false}]}`:                          "There is no group called extra.",
		`{"groups":[{"name":"sky","on":false}]}`:                            "There is no group called sky.",
		`{"groups":[{"name":"push","on":false},{"name":"push","on":true}]}`: "There are two groups called push.",
		`{"extra":"helm"}`:                               "extra is a list of patterns.",
		`{"extra":[1]}`:                                  "extra is a list of patterns.",
		`{"extra":["*"]}`:                                `A pattern starts with the command's name, such as "terraform import"; "*" isn't one.`,
		`{"extra":["a||b x"]}`:                           `A pattern starts with the command's name, such as "terraform import"; "a||b" isn't one.`,
		`{"extra":["   "]}`:                              "A pattern can't be empty.",
		`{"extra":["helm\u0007"]}`:                       "A pattern is one line of plain words.",
		`{"extra":["` + strings.Repeat("a", 201) + `"]}`: "A pattern is at most 200 characters.",
	} {
		if status, got := saveGuard([]byte(body), path); status != http.StatusBadRequest || got != why {
			t.Errorf("%s: %d %q, want %q", body, status, got, why)
		}
	}
	many, _ := json.Marshal(map[string]any{"extra": strings.Split(strings.Repeat("helm,", 41)[:5*41-1], ",")})
	if status, why := saveGuard(many, path); status != http.StatusBadRequest || why != "At most 40 patterns of your own." {
		t.Errorf("41 patterns: %d %q", status, why)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Error("a refused change touched the file")
	}
	if guardPatternProblem("terraform|tofu import") != "" || guardPatternProblem("npm.cmd publish") != "" {
		t.Error("a good pattern was refused")
	}
}

// The route: the page alone switches the groups, and is answered with the guard's state.
func TestThePageAloneSwitchesTheGuard(t *testing.T) {
	d := newTestDeck(t)
	path := filepath.Join(supportDir, hireGuardFile)
	os.Remove(path)
	w := serve(t, d, call{method: "POST", target: "/setup/guard", host: "localhost:7710", from: thisComputer, asTool: true,
		body: `{"groups":[{"name":"cloud","on":false}]}`, headers: map[string]string{setupHeader: "1"}})
	if w.Code == http.StatusOK {
		t.Fatalf("a tool switched the guard: %s", w.Body)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("a tool's refused change wrote the file")
	}
	w = serve(t, d, call{method: "POST", target: "/setup/guard", host: "localhost:7710", from: thisComputer,
		body: `{"groups":[{"name":"cloud","on":false}]}`, headers: map[string]string{setupHeader: "1"}})
	var state struct {
		Groups []struct {
			Name string `json:"name"`
			On   bool   `json:"on"`
		} `json:"groups"`
		Limits string `json:"limits"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &state) != nil || len(state.Groups) != 6 || state.Limits == "" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	for _, g := range state.Groups {
		if g.On == (g.Name == "cloud") {
			t.Errorf("%s is on: %v", g.Name, g.On)
		}
	}
	w = serve(t, d, call{method: "POST", target: "/setup/guard", host: "localhost:7710", from: thisComputer,
		body: `{"groups":[{"name":"sky","on":false}]}`, headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusBadRequest || strings.TrimSpace(w.Body.String()) != "There is no group called sky." {
		t.Errorf("%d %q", w.Code, w.Body)
	}
}
