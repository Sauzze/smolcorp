package gm

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// `smolcorp gm project new <name> [--clone <address>]`: the GM's token, POST /setup/gm/project with the name and the
// address as given (the app checks both), the folder made said back, and the app's refusal in its words.
func TestProjectNewAsksTheAppForAFolder(t *testing.T) {
	var bodies []map[string]any
	answer, status := `{"ok":true,"folder":"/Users/ada/Projects/site"}`, 200
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		bodies = append(bodies, body)
		w.WriteHeader(status)
		io.WriteString(w, answer)
	})
	for _, args := range [][]string{{"project"}, {"project", "make", "site"}, {"project", "new"}, {"project", "new", "a", "b"}, {"project", "new", "site", "--clone"}} {
		if code, _, _ := runGM(app.env(t, withToken()), args...); code != exitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	if len(app.reqs) != 0 {
		t.Fatalf("a usage mistake reached the app: %+v", app.reqs)
	}
	if code, out, errOut := runGM(app.env(t, withToken()), "project", "new", "site"); code != 0 || out != "Made the project folder /Users/ada/Projects/site.\n" {
		t.Fatalf("new: %d %q %q", code, out, errOut)
	}
	if r := app.reqs[0]; r.Method != "POST" || r.Path != "/setup/gm/project" || r.Header.Get("x-smolcorp-gm-token") != fakeToken || r.Header.Get("X-AgentDeck-Setup") != "1" {
		t.Errorf("the request: %+v", r)
	}
	if len(bodies[0]) != 1 || bodies[0]["name"] != "site" {
		t.Errorf("the body: %v", bodies[0])
	}
	answer = `{"ok":true,"folder":"/Users/ada/Projects/site","cloned":"https://example.invalid/site.git"}`
	code, out, _ := runGM(app.env(t, withToken()), "project", "new", "site", "--clone", "https://example.invalid/site.git")
	if code != 0 || out != "Made the project folder /Users/ada/Projects/site, a clone of https://example.invalid/site.git.\n" ||
		bodies[1]["clone"] != "https://example.invalid/site.git" || bodies[1]["name"] != "site" {
		t.Errorf("clone: %d %q %v", code, out, bodies[1])
	}
	status, answer = 409, "There's already something called site in /Users/ada/Projects: pick another name."
	if code, _, errOut := runGM(app.env(t, withToken()), "project", "new", "site"); code != exitApp || !strings.Contains(errOut, answer) {
		t.Errorf("refused: %d %q", code, errOut)
	}
}
