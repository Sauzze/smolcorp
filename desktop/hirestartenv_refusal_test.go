package desktop

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// The app's 409 on a launcher's start says which refusal it is (MSYGMEDIT1): the hire's row marked gone before its
// window asked (GMEDIT1), or its settings file not the one Smolcorp sealed (SETTINGSFILE1). The launcher tells the
// owner which, and one it doesn't know still starts nothing, in words that claim neither.
func TestALaunchersStartRefusalSaysWhichOneItIs(t *testing.T) {
	cases := []struct{ body, want string }{
		{startRefusedGone, "this hire was marked gone before its window opened, so nothing was started"},
		{startRefusedSettings, "this hire's settings file isn't the one Smolcorp wrote: something on this computer changed or replaced it. Hire again"},
		{"", "Smolcorp refused this hire's start, so nothing was started. Hire again"},
		{"something else", "Smolcorp refused this hire's start, so nothing was started. Hire again"},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(c.body))
		}))
		env, err := fetchHireStartEnv(srv.Client(), srv.URL, "fake-hook-key", "tok", "abc123")
		srv.Close()
		if err == nil || err.Error() != c.want || env.Settings != "" || len(env.Set) != 0 {
			t.Errorf("body %q: %v, want %q", c.body, err, c.want)
		}
		if err != nil && strings.Contains(err.Error(), "tok") {
			t.Errorf("body %q: the token is in the words: %v", c.body, err)
		}
	}
}

// Through the route itself, each refusal comes with its own reason: a changed settings file behind an open row is
// SETTINGSFILE1's, and a row marked gone is GMEDIT1's, even with the file changed too (the row is read first). Each
// spends the start.
func TestTheLaunchersRouteTellsAGoneRowFromAChangedSettingsFile(t *testing.T) {
	d := newTestDeck(t)
	r := newHireSettingsRig(t)
	claudeRegistry(t, Hire{ID: "abc123", Name: "Ada", Folder: keptPath(r.root), State: "starting", ViaLauncher: true})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) { d.startEnvRoute(w, q.URL.Query()) }))
	t.Cleanup(server.Close)
	for _, c := range []struct {
		gone         bool
		reason, want string
	}{
		{false, startRefusedSettings, "settings file isn't the one Smolcorp wrote"},
		{true, startRefusedGone, "marked gone before its window opened"},
	} {
		if c.gone {
			must0(t, updateRegistry(func(f *registryFile) { f.hires[0].State = "fired" }))
		}
		written, err := r.writeFile("abc123", nil)
		must0(t, err)
		must0(t, os.Link(written.Path, written.Path+".md")) // changed after it was sealed, as SETTINGSFILE1's row does
		token := strings.ToUpper(newID())
		d.mu.Lock()
		d.hires[token] = &pendingHire{at: time.Now(), hireID: "abc123", env: &hireStartEnv{Settings: written.Path, SettingsSum: written.Sum}}
		d.mu.Unlock()
		raw, err := server.Client().Get(server.URL + "?hire=" + token + "&id=abc123")
		must0(t, err)
		body := make([]byte, 64)
		n, _ := raw.Body.Read(body)
		raw.Body.Close()
		if raw.StatusCode != http.StatusConflict || string(body[:n]) != c.reason {
			t.Errorf("gone %v: %d %q, want 409 %q", c.gone, raw.StatusCode, body[:n], c.reason)
		}
		d.mu.Lock()
		spent := d.hires[token] == nil || d.hires[token].env == nil
		d.mu.Unlock()
		if !spent {
			t.Errorf("gone %v: the start wasn't spent", c.gone)
		}
		token = strings.ToUpper(newID())
		d.mu.Lock()
		d.hires[token] = &pendingHire{at: time.Now(), hireID: "abc123", env: &hireStartEnv{Settings: written.Path, SettingsSum: written.Sum}}
		d.mu.Unlock()
		if _, err := fetchHireStartEnv(server.Client(), server.URL, "fake-hook-key", token, "abc123"); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("gone %v: the launcher said %v, want %q", c.gone, err, c.want)
		}
		must0(t, os.Remove(written.Path+".md"))
	}
}
