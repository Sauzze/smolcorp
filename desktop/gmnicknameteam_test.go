package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Sauzze/smolcorp/internal/gm"
)

// FXGMNICK: what `gm nickname` sets is what `gm team` shows, and `gm fire` of a session the owner started says what it is.

// gmTeamNicknames asks GET /setup/gm/team with the GM's token: each row's nickname by its id.
func gmTeamNicknames(t *testing.T, d *Deck, gm string) map[string]string {
	t.Helper()
	w := serve(t, d, call{method: "GET", target: "/setup/gm/team", host: "localhost:7710", from: thisComputer,
		withoutSetupKey: true, headers: map[string]string{setupHeader: "1", gmTokenHeader: gm}})
	var reply struct {
		Team []struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		} `json:"team"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &reply) != nil {
		t.Fatalf("the team: %d %q", w.Code, w.Body)
	}
	names := map[string]string{}
	for _, row := range reply.Team {
		names[row.ID] = row.Nickname
	}
	return names
}

func gmPost(t *testing.T, d *Deck, gm, path, body string) (int, string) {
	t.Helper()
	w := serve(t, d, call{method: "POST", target: path, host: "localhost:7710", from: thisComputer,
		withoutSetupKey: true, body: body, headers: map[string]string{setupHeader: "1", gmTokenHeader: gm}})
	return w.Code, w.Body.String()
}

// A session at hook time, as the hooks report it: its cwd as given, on its tab (with none, the deck keeps no session).
func sessionAt(d *Deck, id, cwd, tab string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "session_id": id, "cwd": cwd}, Claude, tab, "", 1761000540000)
}

func TestTheNicknameTheGMSetsIsTheOneGMTeamShows(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, _ := withGMAndAHire(t, d) // Rev (a1b2c3) in /work/app-wt/rev, Janitor (d4e5f6, no nickname) in /work/api, Bob fired
	// A session the owner started on Windows, with its folder as Windows writes it; the hires' own sessions.
	sessionAt(d, "s1", `C:\Users\ada\Projects\shop`, "launcher:L-s1")
	sessionAt(d, "s2", "/work/api", "other:/dev/ttys002")
	sessionAt(d, "s3", "/work/app-wt/rev", "other:/dev/ttys003")

	before := gmTeamNicknames(t, d, gm)
	if before["live:s1"] != "" || before["d4e5f6"] != "" || before["a1b2c3"] != "Rev" || before["0a0b0c"] != "Bob" {
		t.Fatalf("before any nickname: %v", before)
	}
	for folder, name := range map[string]string{`C:\Users\ada\Projects\shop`: "Ada", "/work/api": "Jan", "/work/app-wt/rev": "Revvy"} {
		body, _ := json.Marshal(map[string]string{"folder": folder, "name": name})
		if status, text := gmPost(t, d, gm, "/setup/gm/nickname", string(body)); status != http.StatusOK {
			t.Fatalf("%s: %d %q", folder, status, text)
		}
	}
	got := gmTeamNicknames(t, d, gm)
	// The owner's session and a hire with no nickname show the office's name; a hire with a nickname of its own keeps
	// it (FXGMNICKR1: the registry wins for hires), and so does one the office has no name for.
	want := map[string]string{"live:s1": "Ada", "d4e5f6": "Jan", "a1b2c3": "Rev", "0a0b0c": "Bob"}
	for id, name := range want {
		if got[id] != name {
			t.Errorf("%s: nickname %q, want %q (team %v)", id, got[id], name, got)
		}
	}
	// Taken away again, the row is back to the registry's (none for the owner's session or the hire with none).
	gmPost(t, d, gm, "/setup/gm/nickname", `{"folder":"/work/api","name":""}`)
	gmPost(t, d, gm, "/setup/gm/nickname", `{"folder":"C:\\Users\\ada\\Projects\\shop","name":""}`)
	if got := gmTeamNicknames(t, d, gm); got["d4e5f6"] != "" || got["live:s1"] != "" {
		t.Errorf("taken away: %v", got)
	}
}

// The office's name is keyed as the page keys it, by the project's last folder name, with either separator.
func TestTheOfficesNameIsFoundByTheProjectsLastFolderNameWithEitherSeparator(t *testing.T) {
	team := map[string]TeamMember{"shop": {Name: "Ada"}}
	for _, project := range []string{`C:\Users\ada\Projects\shop`, `C:\Users\ada\Projects\shop\`, "C:/Users/ada/Projects/shop", "/home/ada/shop"} {
		if got := officeName(team, project); got != "Ada" {
			t.Errorf("%s: %q", project, got)
		}
	}
	if got := officeName(team, ""); got != "" {
		t.Errorf("no project: %q", got)
	}
}

func TestAGMFireOfASessionTheOwnerStartedSaysWhatItIsAndHowItStops(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, _ := withGMAndAHire(t, d)
	d.mu.Lock()
	d.config.HiringEnabled = true
	d.launchers["L1"] = &launcherTab{agent: Claude, desk: -1}
	d.mu.Unlock()
	sessionAt(d, "plain", "/work/shop", "other:/dev/ttys004")    // a terminal Smolcorp only watches
	sessionAt(d, "pane", "/work/site", "tmux:%3@/work/ada.sock") // a tmux pane
	sessionAt(d, "window", "/work/blog", "launcher:L1")          // smolcorp run's window
	sessionAt(d, "left", "/work/old", "other:/dev/ttys005")      // where Bob, fired, worked
	gmPost(t, d, gm, "/setup/gm/nickname", `{"folder":"/work/site","name":"Sid"}`)

	const started = " is a session you started yourself, not a hire, so the General Manager can't fire it. To stop it, quit it in its own window"
	const office = ", or hold its desk in the office and fire it."
	officeOnPanes := runtime.GOOS != "windows" // on Windows, firing leaves a pane open (process_windows.go)
	for _, c := range []struct {
		id, says string
		office   bool
	}{
		{"live:plain", "shop" + started, false},
		{"live:pane", "Sid" + started, officeOnPanes},
		{"live:window", "blog" + started, true},
		{"live:left", gmFireNoHire, false},
		{"live:nobody", gmFireNoHire, false},
		{"nobody", gmFireNoHire, false},
	} {
		status, text := gmPost(t, d, gm, "/setup/gm/fire", `{"id":"`+c.id+`"}`)
		if status != http.StatusNotFound || !strings.HasPrefix(text, c.says) || strings.Contains(text, office) != c.office {
			t.Errorf("%s: %d %q, want 404 %q, the office's Fire offered %v", c.id, status, text, c.says, c.office)
		}
		if c.says != gmFireNoHire && !c.office && !strings.HasSuffix(strings.TrimSpace(text), "quit it in its own window.") {
			t.Errorf("%s: %q", c.id, text)
		}
	}
	d.mu.Lock()
	sessions := len(d.sessions.All())
	d.mu.Unlock()
	if sessions != 4 {
		t.Errorf("a refused fire ended a session: %d left", sessions)
	}
}

// FXGMNICKR1 (from the review's probes): a hire's own nickname is what `gm team` shows, whole, whatever name the
// office keeps on its desk.

// A worktree hire hired as "Rev": its start keeps "Rev" on its desk (hiredInto). The GM then renames it in the
// registry (POST /setup/gm/registry, the edit the API documents for a hire's nickname).
func TestARegistryRenameOfAHireShowsInGMTeam(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, _ := withGMAndAHire(t, d)
	h, _ := hireByID("a1b2c3")
	d.mu.Lock()
	d.hiredInto(h)
	d.mu.Unlock()
	if code, text := gmPost(t, d, gm, "/setup/gm/registry", `{"id":"a1b2c3","nickname":"Rex"}`); code != http.StatusOK {
		t.Fatalf("registry edit: %d %q", code, text)
	}
	if got := gmTeamNicknames(t, d, gm)["a1b2c3"]; got != "Rex" {
		t.Errorf("gm team shows %q, the registry's nickname is Rex", got)
	}
}

// longNickname is a hire's nickname over the office's 16 characters (the hire and registry allow 40).
const longNickname = "Pull request reviewer"

// A worktree hire with a nickname over 16 characters: the office keeps its first 16 (hiredInto).
func withALongNicknamedHire(t *testing.T, d *Deck, gm string) {
	t.Helper()
	if code, text := gmPost(t, d, gm, "/setup/gm/registry", `{"id":"a1b2c3","nickname":"`+longNickname+`"}`); code != http.StatusOK {
		t.Fatalf("registry edit: %d %q", code, text)
	}
	h, _ := hireByID("a1b2c3")
	d.mu.Lock()
	d.hiredInto(h) // as its start does with the nickname it was hired with
	d.mu.Unlock()
}

func TestAHiresLongNicknameShowsWholeInGMTeam(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, _ := withGMAndAHire(t, d)
	withALongNicknamedHire(t, d, gm)
	if got := gmTeamNicknames(t, d, gm)["a1b2c3"]; got != longNickname {
		t.Errorf("gm team shows %q, the registry's nickname is %q", got, longNickname)
	}
}

// `smolcorp gm fire` and `gm brief` find a hire by the whole nickname `gm team` shows, and by a registry rename. The
// team comes from the real router; the fire and the brief go to a stand-in that only keeps the id asked, so nothing
// is fired or written.
func TestGMFireAndBriefFindAHireByItsWholeNicknameAndARename(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	token, _ := withGMAndAHire(t, d)
	withALongNicknamedHire(t, d, token)
	var mu sync.Mutex
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && (r.URL.Path == "/setup/gm/fire" || r.URL.Path == "/setup/gm/brief") {
			var body struct{ ID string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			asked = append(asked, r.URL.Path+" "+body.ID)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		(&router{deck: d}).ServeHTTP(w, r)
	}))
	defer server.Close()
	t.Setenv("SMOLCORP_PORT", server.URL[strings.LastIndex(server.URL, ":")+1:])
	t.Setenv("SMOLCORP_GM_TOKEN", token)
	t.Setenv("SMOLCORP_HIRE_KEY", "")
	run := func(args ...string) {
		t.Helper()
		var out, errOut strings.Builder
		if code := gm.Run(args, &out, &errOut); code != 0 {
			t.Errorf("gm %s: %d %q %q", strings.Join(args, " "), code, out.String(), errOut.String())
		}
	}
	run("fire", longNickname)
	run("brief", longNickname, "--text", "Look at the pull requests first.")
	if code, text := gmPost(t, d, token, "/setup/gm/registry", `{"id":"a1b2c3","nickname":"Rex"}`); code != http.StatusOK {
		t.Fatalf("registry edit: %d %q", code, text)
	}
	run("fire", "Rex")
	run("brief", "Rex", "--text", "And the failing build.")
	mu.Lock()
	defer mu.Unlock()
	want := []string{"/setup/gm/fire a1b2c3", "/setup/gm/brief a1b2c3", "/setup/gm/fire a1b2c3", "/setup/gm/brief a1b2c3"}
	if !slices.Equal(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}
}

// O1: under the Mac app, a session in a Terminal, iTerm2 or Ghostty tab is offered the office's fire when the app says
// it can close that tab, as the office shows its fire there (addMacControls), and not when it says it can't.
func TestUnderTheMacAppAGMFireOfASessionTheOwnerStartedOffersTheOfficeWhereTheAppCanClose(t *testing.T) {
	d := newMacTestDeck(t)
	managerOn(d)
	gm, _ := withGMAndAHire(t, d)
	tabs := map[string]string{"term": "tty:/dev/ttys006", "iterm": "iterm:AB12", "ghost": "ghostty:/dev/ttys007", "pane": "tmux:%3@/work/ada.sock"}
	fires := map[string]bool{"term": true, "iterm": true, "ghost": false, "pane": false}
	d.mu.Lock()
	d.config.HiringEnabled = true
	var key []string
	d.mac.abilities = map[string]tabAbilities{}
	for id, tab := range tabs {
		d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "session_id": id, "cwd": "/work/" + id}, Claude, tab, "", 1761000540000)
		d.mac.abilities[tab] = tabAbilities{Keys: true, Text: true, Fire: fires[id]}
		key = append(key, tab)
	}
	slices.Sort(key)
	d.mac.abilitiesFor = strings.Join(key, "\n") // what the app said, so it isn't asked again
	d.mu.Unlock()
	const office = ", or hold its desk in the office and fire it."
	for id := range tabs {
		status, text := gmPost(t, d, gm, "/setup/gm/fire", `{"id":"live:`+id+`"}`)
		if status != http.StatusNotFound || !strings.HasPrefix(text, id+" is a session you started yourself") || strings.Contains(text, office) != fires[id] {
			t.Errorf("live:%s on %s: %d %q, the office's fire offered %v", id, tabs[id], status, text, fires[id])
		}
	}
	d.mu.Lock()
	sessions := len(d.sessions.All())
	d.mu.Unlock()
	if sessions != len(tabs) {
		t.Errorf("a refused fire ended a session: %d left", sessions)
	}
}

// RT126FIXREVIEW-GMNICKR1 O1: a hire with no nickname of its own in the registry is found by the name `gm team` shows
// for it, the office's, everywhere a hire is named: mail, contacts, a watch. A name that found a hire before still
// finds that one: Janitor's desk called "Rev" in the office doesn't take Rev's mail.
func TestAHireWithNoNicknameIsFoundByTheNameGMTeamShows(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, _ := withGMAndAHire(t, d) // Rev (a1b2c3), Janitor (d4e5f6, no nickname) in /work/api, Bob fired
	sessionAt(d, "s2", "/work/api", "other:/dev/ttys002")
	if status, text := gmPost(t, d, gm, "/setup/gm/nickname", `{"folder":"/work/api","name":"Jan"}`); status != http.StatusOK {
		t.Fatalf("nickname: %d %q", status, text)
	}
	if got := gmTeamNicknames(t, d, gm)["d4e5f6"]; got != "Jan" {
		t.Fatalf("gm team shows %q for Janitor, want Jan", got)
	}
	if dest, refused := d.mailTarget("jan"); refused != nil || dest.ID != "d4e5f6" {
		t.Errorf("mail to Jan: %+v %+v", dest, refused)
	}
	if status, text := gmPost(t, d, gm, "/setup/gm/contacts", `{"from":"Jan","to":"Rev","oneWay":true}`); status != http.StatusOK ||
		!strings.Contains(text, `"from":"d4e5f6"`) || !strings.Contains(text, `"to":"a1b2c3"`) {
		t.Errorf("contacts Jan to Rev: %d %q", status, text)
	}
	if status, text := gmPost(t, d, gm, "/setup/gm/watch", `{"kind":"exit","target":"Jan"}`); status != http.StatusOK ||
		!strings.Contains(text, `"target":"d4e5f6"`) {
		t.Errorf("a watch on Jan: %d %q", status, text)
	}
	if h, refused := d.hireNamed("Jan"); refused != nil || h.ID != "d4e5f6" {
		t.Errorf("a preview's hire Jan: %+v %+v", h, refused)
	}
	// The registry's names come first.
	gmPost(t, d, gm, "/setup/gm/nickname", `{"folder":"/work/api","name":"Rev"}`)
	if dest, refused := d.mailTarget("Rev"); refused != nil || dest.ID != "a1b2c3" {
		t.Errorf("mail to Rev, with Janitor's desk called Rev too: %+v %+v", dest, refused)
	}
	if dest, refused := d.mailTarget("Janitor"); refused != nil || dest.ID != "d4e5f6" {
		t.Errorf("mail to Janitor: %+v %+v", dest, refused)
	}
}

// RT126FIXREVIEW-GMNICKR1 O2: `gm nickname` on the folder of a hire with a nickname of its own renames only its desk
// in the office: `gm team` keeps the registry's, so the answer says so, and where a hire is renamed. The registry
// names a hire by its id, in up to 40 characters, and a folder can hold several sessions: the folder can't rename it.
// A folder whose hire has none, as Janitor's, gets no note.
func TestGMNicknameOnANicknamedHiresFolderSaysWhereToRenameIt(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, _ := withGMAndAHire(t, d)
	sessionAt(d, "s2", "/work/api", "other:/dev/ttys002")
	sessionAt(d, "s3", "/work/app-wt/rev", "other:/dev/ttys003")
	var answer map[string]string
	status, text := gmPost(t, d, gm, "/setup/gm/nickname", `{"folder":"/work/app-wt/rev","name":"Revvy"}`)
	if status != http.StatusOK || json.Unmarshal([]byte(text), &answer) != nil {
		t.Fatalf("nickname: %d %q", status, text)
	}
	const note = "Only the office's name for the desk changed: Rev keeps its own nickname from the registry, which gm team, " +
		"mail and the gm commands go by. To rename a hire, change its nickname in the registry (POST /setup/gm/registry " +
		"with its id and nickname)."
	if answer["note"] != note || answer["nickname"] != "Revvy" {
		t.Errorf("the answer: %q\nwant the note %q", answer, note)
	}
	if got := gmTeamNicknames(t, d, gm)["a1b2c3"]; got != "Rev" {
		t.Errorf("gm team shows %q, the registry's nickname is Rev", got)
	}
	answer = nil
	status, text = gmPost(t, d, gm, "/setup/gm/nickname", `{"folder":"/work/api","name":"Jan"}`)
	if status != http.StatusOK || json.Unmarshal([]byte(text), &answer) != nil || answer["note"] != "" {
		t.Errorf("a hire with no nickname of its own: %d %q", status, text)
	}
	if got := nicknameKeptNote([]string{"Rev", "Ada", "Bo"}, false); !strings.Contains(got, ": Rev, Ada and Bo keep their own nicknames from the registry,") {
		t.Errorf("three hires: %q", got)
	}
}
