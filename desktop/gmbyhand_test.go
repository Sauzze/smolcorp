package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// fromPage is a request from the setup page, as the page sends it.
func fromPage(t *testing.T, d *Deck, method, target string, headers map[string]string) (int, map[string]any, string) {
	t.Helper()
	all := map[string]string{setupHeader: "1"}
	for name, value := range headers {
		all[name] = value
	}
	w := serve(t, d, call{method: method, target: target, host: "localhost:7710", from: thisComputer, headers: all})
	var said map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &said)
	return w.Code, said, w.Body.String()
}

// tokenInFile is the gm-token file's token, "" for none, and checks it is yours alone (yoursAlone: 0600, or on
// Windows an access list for you, the system and the administrators only).
func tokenInFile(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(gmTokenPath()); err != nil {
		return ""
	}
	if err := yoursAlone(gmTokenPath()); err != nil {
		t.Errorf("gm-token isn't yours alone: %v", err)
	}
	data, _ := os.ReadFile(gmTokenPath())
	return validGMToken(string(data))
}

// Without the Mac app (Linux, Windows) there is no GM started by hand and no new token: both routes are 404, the
// setup state names no file, and none is written.
func TestWithoutTheMacAppThereIsNoGMStartedByHand(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	for _, target := range []string{"/setup/gm/by-hand?enabled=1", "/setup/gm/token/rotate"} {
		if code, _, _ := fromPage(t, d, http.MethodPost, target, nil); code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", target, code)
		}
	}
	d.mu.Lock()
	manager := d.managerState()
	d.mu.Unlock()
	if manager["gmTokenFile"] != "" || manager["gmByHand"] != false {
		t.Errorf("manager state %v", manager)
	}
	if _, err := os.Stat(gmTokenPath()); err == nil {
		t.Error("a token file was written")
	}
}

// Under the Mac app: "Allow a General Manager started by hand" keeps the token in gm-token (yours alone), with the
// manager off too; a GM that uses it is "started by hand", and the live session it names has the GM's role, in the
// office and on the team. Off takes the file and the GM started by hand away.
func TestUnderTheMacAppAGMCanBeStartedByHand(t *testing.T) {
	isolate(t)
	withMac(t)
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	if code, _, body := fromPage(t, d, http.MethodPost, "/setup/gm/by-hand?enabled=yes", nil); code != http.StatusBadRequest || !strings.Contains(body, "enabled=1 or enabled=0") {
		t.Errorf("enabled=yes: %d %q", code, body)
	}
	code, said, _ := fromPage(t, d, http.MethodPost, "/setup/gm/by-hand?enabled=1", nil)
	if code != http.StatusOK || said["gmByHand"] != true {
		t.Fatalf("by hand on, with the manager off: %d %v", code, said)
	}
	token := tokenInFile(t)
	if token == "" || !loadConfig().GMByHand {
		t.Fatalf("no token kept: %q, saved %v", token, loadConfig().GMByHand)
	}
	d.mu.Lock()
	manager := d.managerState()
	d.mu.Unlock()
	if manager["gmTokenFile"] != gmTokenPath() || manager["gmByHand"] != true || manager["gmStartedByHand"] != false {
		t.Errorf("manager state %v", manager)
	}
	if said, _ := json.Marshal(manager); strings.Contains(string(said), token) {
		t.Error("the setup state carries the token")
	}

	// The GM, started by hand in a session of its own, uses the token.
	managerOn(d)
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "session_id": "gm-1", "cwd": "/work/gm"}, Claude, "tty:/dev/ttys009", "", 1)
	d.mu.Unlock()
	if code, _, _ := fromPage(t, d, http.MethodGet, "/setup/gm/version", map[string]string{gmTokenHeader: token, gmSessionHeader: "gm-1"}); code != http.StatusOK {
		t.Fatalf("the GM's version: %d", code)
	}
	d.mu.Lock()
	started, session := d.startedByHand(), d.byHandGMSession()
	role, hasRole := d.sessionRole(d.sessions.Get("gm-1"))
	d.mu.Unlock()
	if !started || session != "gm-1" || !hasRole || role.ID != gmRole {
		t.Errorf("started by hand %v, session %q, role %v %q", started, session, hasRole, role.ID)
	}
	if saved, _ := os.ReadFile(gmSessionPath()); strings.TrimSpace(string(saved)) != "gm-1" {
		t.Errorf("gm-session holds %q", saved)
	}
	_, team, _ := fromPage(t, d, http.MethodGet, "/setup/gm/team", nil)
	if !teamHasRole(team, "gm-1", gmRole) {
		t.Errorf("the team doesn't show gm-1 as the GM: %v", team["team"])
	}

	// After a restart: the same token, and the session once it is live again.
	d2 := startDeck(t)
	d2.mu.Lock()
	sameToken := d2.isGMToken(token)
	d2.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "session_id": "gm-1", "cwd": "/work/gm"}, Claude, "tty:/dev/ttys009", "", 2)
	again := d2.byHandGMSession()
	d2.mu.Unlock()
	if !sameToken || again != "gm-1" {
		t.Errorf("after a restart: token kept %v, session %q", sameToken, again)
	}

	// Off: no file, no GM started by hand.
	if code, said, _ := fromPage(t, d, http.MethodPost, "/setup/gm/by-hand?enabled=0", nil); code != http.StatusOK || said["gmByHand"] != false {
		t.Fatalf("by hand off: %d %v", code, said)
	}
	for _, file := range []string{gmTokenPath(), gmSessionPath()} {
		if _, err := os.Stat(file); err == nil {
			t.Errorf("%s still there with the setting off", file)
		}
	}
	d.mu.Lock()
	if d.startedByHand() || d.byHandGMSession() != "" {
		t.Error("still started by hand with the setting off")
	}
	d.mu.Unlock()
}

// A new GM token (the page's alone, with the manager on): the old one stops working at once, the file has the new
// one, the answer never carries it, and the GM started by hand is forgotten. A GM hire's token goes in the file, and
// one taken away (its GM fired) is replaced there, so a GM can always be started by hand while the setting is on.
func TestANewGMTokenStopsTheOldOneAndTheFileFollows(t *testing.T) {
	isolate(t)
	withMac(t)
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	if code, _, _ := fromPage(t, d, http.MethodPost, "/setup/gm/token/rotate", nil); code != http.StatusConflict {
		t.Errorf("a new token with the manager off: %d, want 409", code)
	}
	managerOn(d)
	fromPage(t, d, http.MethodPost, "/setup/gm/by-hand?enabled=1", nil)
	old := tokenInFile(t)
	code, said, body := fromPage(t, d, http.MethodPost, "/setup/gm/token/rotate", nil)
	fresh := tokenInFile(t)
	if code != http.StatusOK || said["rotated"] != true || said["message"] != gmRotated["message"] || fresh == "" || fresh == old || strings.Contains(body, fresh) {
		t.Errorf("rotate: %d %v (old %v, new %v)", code, said, old != "", fresh != "")
	}
	d.mu.Lock()
	if d.isGMToken(old) || !d.isGMToken(fresh) {
		t.Error("the old token still works, or the new one doesn't")
	}
	d.mu.Unlock()
	// The GM can't ask for a new one: the page's alone.
	asGM := serve(t, d, call{method: http.MethodPost, target: "/setup/gm/token/rotate", host: "localhost:7710", from: thisComputer,
		withoutSetupKey: true, headers: map[string]string{setupHeader: "1", gmTokenHeader: fresh}})
	if asGM.Code != http.StatusForbidden || tokenInFile(t) != fresh {
		t.Errorf("the GM's own rotate: %d, token changed %v", asGM.Code, tokenInFile(t) != fresh)
	}

	hired := d.newGMToken()
	if tokenInFile(t) != hired {
		t.Error("the hired GM's token isn't the file's")
	}
	d.clearGMTokenIf(hired)
	left := tokenInFile(t)
	d.mu.Lock()
	works := d.isGMToken(left)
	d.mu.Unlock()
	if left == "" || left == hired || !works {
		t.Errorf("after the GM went: a new token in the file %v, the same as the fired GM's %v, works %v", left != "", left == hired, works)
	}
}

// With a GM started by hand allowed, a fire whose gm-token file write fails (here, its temp name is taken by a
// folder) takes the file away, so the Keychain's replacement is read at the next start and the fired GM's token
// stays dead (RTOK2 N1).
func TestAFiredGMsTokenFileThatCantBeRewrittenIsTakenAway(t *testing.T) {
	isolate(t)
	mac := withMac(t)
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	managerOn(d)
	if code, _, _ := fromPage(t, d, http.MethodPost, "/setup/gm/by-hand?enabled=1", nil); code != http.StatusOK {
		t.Fatalf("by hand on: %d", code)
	}
	fired := d.newGMToken()
	withHiredGM(t, "working")
	d.gmKept.flush(time.Minute)
	if tokenInFile(t) != fired {
		t.Fatal("the hired GM's token isn't the file's")
	}

	if err := os.Mkdir(gmTokenPath()+".tmp", 0o700); err != nil {
		t.Fatal(err)
	}
	d.clearGMTokenIf(fired)
	withHiredGM(t, "fired")
	if !d.gmKept.flush(time.Minute) {
		t.Fatal("the replacement isn't kept in the store")
	}
	replacement, _ := mac.keptItem()
	if replacement == "" || replacement == fired {
		t.Fatalf("the store's replacement: made %v", replacement != "")
	}
	if tokenInFile(t) == fired {
		t.Error("the fired GM's token is still in the file")
	}
	again := startDeck(t)
	if worksAsGM(again, fired) || !worksAsGM(again, replacement) {
		t.Errorf("after a restart, the fired GM's token works %v, the replacement %v", worksAsGM(again, fired), worksAsGM(again, replacement))
	}
}

func teamHasRole(team map[string]any, session, role string) bool {
	rows, _ := team["team"].([]any)
	for _, row := range rows {
		r, _ := row.(map[string]any)
		if r["sessionId"] == session {
			return r["role"] == role
		}
	}
	return false
}
