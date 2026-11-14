//go:build !windows

package desktop

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// A GM held to one project (gmscope.go) proposes a night job's hire only there (scopedGM is a unix rig).
func TestAGMHeldToOneProjectProposesOnlyThere(t *testing.T) {
	d, _, _, _, project := scopedGM(t)
	ask := func(folder string) verdict {
		body := `{"name":"n","kind":"hire","role":"tester","brief":"b","folder":"` + folder + `","when":{"every":"daily","at":"02:00"},"limits":{"wallMinutes":30}}`
		return d.decide(actorGM, action{route: &routeAsk{method: "POST", path: nightProposePath, body: strings.NewReader(body)}})
	}
	if v := ask(t.TempDir()); v.allowed() || !strings.Contains(v.why, "proposes night jobs only in "+project) {
		t.Errorf("outside the project: %+v", v)
	}
	if v := ask(filepath.Join(project, ".")); !v.allowed() || v.body.(nightAsk).Folder != project {
		t.Errorf("in the project: %+v", v)
	}
	task := `{"name":"n","kind":"gm","brief":"b","when":{"every":"daily","at":"02:00"},"limits":{"wallMinutes":30}}`
	if v := d.decide(actorGM, action{route: &routeAsk{method: "POST", path: nightProposePath, body: strings.NewReader(task)}}); !v.allowed() {
		t.Errorf("a task for the GM: %+v", v)
	}
}

// The scope check reads a proposal as the route does, each key spelled once and as nightAsk spells it, so a hire
// can't pass as a task for the GM by spelling "Kind" or saying the kind twice (RT126N1 B2). What it lets through
// is what it read, and the route reads that back the same.
func TestAScopedGMCantSpellItsWayOutOfTheProject(t *testing.T) {
	d, _, _, _, project := scopedGM(t)
	outside := t.TempDir()
	rest := `"role":"tester","brief":"b","folder":"` + outside + `","when":{"every":"daily","at":"02:00"},"limits":{"wallMinutes":30}`
	propose := func(body string) verdict {
		return d.decide(actorGM, action{route: &routeAsk{method: "POST", path: nightProposePath, body: strings.NewReader(body)}})
	}
	for _, kinds := range []string{
		`"Kind":"hire"`,
		`"KIND":"hire"`,
		`"kInd":"hire"`,
		`"kind":"gm","KIND":"hire"`,
		`"kind":"gm","Kind":"hire"`,
		`"kind":"gm","kind":"hire"`,
		`"Kind":"gm","kind":"hire"`,
		`"kind":"hire"`,
	} {
		if v := propose(`{"name":"n",` + kinds + `,` + rest + `}`); v.allowed() {
			t.Errorf("%s with a folder outside the project: let through as %+v", kinds, v.body)
		}
	}
	// Other keys spelled another way, or twice, are refused too, the nested ones included.
	for _, odd := range []string{`"Folder":"` + outside + `"`, `"folder":"` + project + `","folder":"` + outside + `"`, `"when":{"Every":"daily","at":"02:00"}`} {
		body := `{"name":"n","kind":"hire","role":"tester","brief":"b",` + odd + `,"limits":{"wallMinutes":30}}`
		if !strings.Contains(odd, `"when"`) {
			body = strings.Replace(body, `"limits"`, `"when":{"every":"daily","at":"02:00"},"limits"`, 1)
		} else {
			body = strings.Replace(body, `"role"`, `"folder":"`+project+`","role"`, 1)
		}
		if v := propose(body); v.allowed() {
			t.Errorf("%s: let through as %+v", odd, v.body)
		}
	}
	// A task for the GM, spelled right, goes on as read, and the route reads the body it's given the same way.
	task := `{"name":"n","kind":"gm","brief":"b","when":{"every":"daily","at":"02:00"},"limits":{"wallMinutes":30}}`
	v := propose(task)
	if !v.allowed() {
		t.Fatalf("a task for the GM: %+v", v)
	}
	sent, _ := json.Marshal(v.body)
	var read nightAsk
	if !decodedNight(bytes.NewReader(sent), &read) || read.Kind != nightGM || read.Folder != "" {
		t.Fatalf("the route reads %s as %+v", sent, read)
	}
}
