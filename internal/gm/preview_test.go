package gm

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// `smolcorp gm preview`: a hire sets its own desk's, signed with its key alone; the GM names a hire; the app's
// refusal comes back in its words.
func TestPreviewDeclaresADesksAddress(t *testing.T) {
	var bodies []map[string]any
	answer := `{"session":"s1","url":"http://localhost:3000"}`
	status := 200
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		bodies = append(bodies, body)
		w.WriteHeader(status)
		io.WriteString(w, answer)
	})

	// Usage mistakes never reach the app.
	for _, args := range [][]string{{"preview"}, {"preview", "a", "b"}, {"preview", "--clear", "http://localhost:3000"}, {"preview", "http://localhost:3000"}} {
		if code, _, _ := runGM(app.env(t, withToken()), args...); code != exitUsage {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	if len(app.reqs) != 0 {
		t.Fatalf("a usage mistake reached the app: %+v", app.reqs)
	}

	// A hire: its key alone, no hire named, its own desk.
	code, out, errOut := runGM(app.env(t, map[string]string{hireKeyEnv: fakeHireKey, tokenEnv: fakeToken}), "preview", "http://localhost:3000")
	if code != 0 || out != "Your desk's preview is http://localhost:3000.\n" {
		t.Fatalf("hire: %d %q %q", code, out, errOut)
	}
	h := app.reqs[0]
	if h.Method != "POST" || h.Path != "/setup/preview" || h.Header.Get("x-smolcorp-hire-key") != fakeHireKey || h.Header.Get("x-smolcorp-gm-token") != "" {
		t.Errorf("hire's request: %+v", h)
	}
	if _, named := bodies[0]["hire"]; named || bodies[0]["url"] != "http://localhost:3000" {
		t.Errorf("hire's body: %v", bodies[0])
	}

	// The GM, for a hire by name; and --clear.
	if code, out, _ := runGM(app.env(t, withToken()), "preview", "--hire", "Rev", "http://localhost:3000"); code != 0 || out != "Rev's desk's preview is http://localhost:3000.\n" {
		t.Errorf("gm: %d %q", code, out)
	}
	if bodies[1]["hire"] != "Rev" || app.reqs[1].Header.Get("x-smolcorp-gm-token") != fakeToken {
		t.Errorf("gm's request: %v %v", bodies[1], app.reqs[1].Header)
	}
	answer = `{"session":"s1","url":""}`
	if code, out, _ := runGM(app.env(t, withToken()), "preview", "--clear", "--hire", "Rev"); code != 0 || !strings.Contains(out, "taken away") || bodies[2]["url"] != "" {
		t.Errorf("clear: %d %q %v", code, out, bodies[2])
	}

	// Refused: the app's words.
	status, answer = 403, "A hire may set only its own desk's preview."
	code, _, errOut = runGM(app.env(t, map[string]string{hireKeyEnv: fakeHireKey}), "preview", "--hire", "Janitor", "http://localhost:3000")
	if code != exitApp || !strings.Contains(errOut, "A hire may set only its own desk's preview.") {
		t.Errorf("refused: %d %q", code, errOut)
	}
}
