//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RT126POLISHB N1: a GM held to one project names a hire in mail, contacts and a watch. The scope finds which hire
// that is, and the route is given that hire by its id, never the name, as the folder routes are given the folder as
// checked. The office's names change under a held GM's own `gm nickname`, so a name that changes meaning between the
// check and the route can't move the message, the pair or the watch to a hire outside the project.

// heldNames is a deck with a GM held to proj and three hires: Ada (Adapter, no nickname) and Cy (a nickname of its
// own) at work in proj, Bea (Beacon, no nickname) in elsewhere, outside it. The office calls proj's desk "Jan".
func heldNames(t *testing.T) (d *Deck, gm Hire, token, proj, elsewhere string) {
	t.Helper()
	d = newTestDeck(t)
	managerOn(d)
	proj, elsewhere = filepath.Join(homeDir, "proj"), filepath.Join(homeDir, "elsewhere")
	for _, dir := range []string{proj, elsewhere} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	proj, elsewhere = physical(proj), physical(elsewhere)
	gm = Hire{ID: "g00001", Name: "General Manager", Nickname: "Boss", Role: gmRole, Folder: keptPath(proj), Scope: proj, State: "working", Started: 1761000000000}
	hires := []Hire{gm,
		{ID: "ad0001", Name: "Adapter", Role: "janitor", Folder: keptPath(proj), State: "idle", Started: 1761000100000},
		{ID: "cy0001", Name: "Cyclist", Nickname: "Cy", Role: "tester", Folder: keptPath(proj), State: "idle", Started: 1761000200000},
		{ID: "be0001", Name: "Beacon", Role: "janitor", Folder: keptPath(elsewhere), State: "idle", Started: 1761000300000},
	}
	data, _ := json.Marshal(map[string]any{"version": 1, "hires": hires})
	if err := writeRegistryFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	officeCalls(d, proj, "Jan")
	token = d.newGMToken()
	if token == "" || gmNow().ID != gm.ID {
		t.Fatalf("the held GM: token %v, gmNow %+v", token != "", gmNow())
	}
	return d, gm, token, proj, elsewhere
}

// officeCalls names folder's desk in the office, as `gm nickname` does; "" takes the name away.
func officeCalls(d *Deck, folder, name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.config.Layout.Room.Team == nil {
		d.config.Layout.Room.Team = map[string]TeamMember{}
	}
	if name == "" {
		delete(d.config.Layout.Room.Team, lastPart(folder))
		return
	}
	d.config.Layout.Room.Team[lastPart(folder)] = TeamMember{Name: name}
}

// heldChecked is the request as the held GM's scope let it through, as its route is given it (gates.go).
func heldChecked(t *testing.T, d *Deck, gm Hire, path, body string) map[string]any {
	t.Helper()
	status, why, given := d.gmScopeRefusal(gm, http.MethodPost, path, nil, []byte(body))
	if status != 0 {
		t.Fatalf("%s %s: refused, %d %q", path, body, status, why)
	}
	data, _ := json.Marshal(given)
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// A held GM's send, contacts and watch reach the route as the hire's id, not the name, and the rest as sent.
func TestAHeldGMsMailContactsAndWatchReachTheRouteAsTheHiresID(t *testing.T) {
	d, gm, token, _, _ := heldNames(t)
	for _, c := range []struct{ path, body, want string }{
		{"/setup/gm/send", `{"to":"Jan","text":"hello","id":"m-1"}`, `{"id":"m-1","text":"hello","to":"ad0001"}`},
		{"/setup/gm/send", `{"to":" the GM ","text":"hello"}`, `{"text":"hello","to":" the GM "}`},
		{"/setup/gm/contacts", `{"from":"Jan","to":"cy","oneWay":true}`, `{"from":"ad0001","oneWay":true,"to":"cy0001"}`},
		{"/setup/gm/watch", `{"kind":"exit","target":"Jan"}`, `{"kind":"exit","target":"ad0001"}`},
		{"/setup/gm/watch", `{"kind":"idle","target":" jan","minutes":5}`, `{"kind":"idle","minutes":5,"target":"ad0001"}`},
		{"/setup/gm/watch", `{"kind":"file","hire":"Jan","target":"notes.txt","pattern":"done"}`,
			`{"hire":"ad0001","kind":"file","pattern":"done","target":"notes.txt"}`},
	} {
		got, _ := json.Marshal(heldChecked(t, d, gm, c.path, c.body))
		if string(got) != c.want {
			t.Errorf("%s %s: the route is given %s, want %s", c.path, c.body, got, c.want)
		}
	}
	// Through the gate and the routes, with the GM's token.
	if status, text := gmPost(t, d, token, "/setup/gm/contacts", `{"from":"Jan","to":"Cy","oneWay":true}`); status != http.StatusOK ||
		!strings.Contains(text, `"from":"ad0001"`) || !strings.Contains(text, `"to":"cy0001"`) {
		t.Errorf("contacts Jan to Cy: %d %q", status, text)
	}
	if status, text := gmPost(t, d, token, "/setup/gm/watch", `{"kind":"exit","target":"Jan"}`); status != http.StatusOK ||
		!strings.Contains(text, `"target":"ad0001"`) {
		t.Errorf("a watch on Jan: %d %q", status, text)
	}
	// Outside the project, by its office name or its own, still refused.
	officeCalls(d, filepath.Join(homeDir, "elsewhere"), "Bea")
	for path, body := range map[string]string{
		"/setup/gm/send":     `{"to":"Bea","text":"hello"}`,
		"/setup/gm/contacts": `{"from":"Jan","to":"Beacon"}`,
		"/setup/gm/watch":    `{"kind":"exit","target":"Bea"}`,
	} {
		if status, text := gmPost(t, d, token, path, body); !scopeRefused(status, text) {
			t.Errorf("%s %s: %d %q", path, body, status, text)
		}
	}
}

// The office renames between the check and the route: "Jan" now names Bea's desk, outside the project. What the
// route was given still finds Ada, the hire the scope approved.
func TestANameThatMovesAfterAHeldGMsCheckDoesntMoveTheTarget(t *testing.T) {
	d, gm, _, proj, elsewhere := heldNames(t)
	send := heldChecked(t, d, gm, "/setup/gm/send", `{"to":"Jan","text":"hello"}`)
	contacts := heldChecked(t, d, gm, "/setup/gm/contacts", `{"from":"Jan","to":"Cy","oneWay":true}`)
	watch := heldChecked(t, d, gm, "/setup/gm/watch", `{"kind":"exit","target":"Jan"}`)
	officeCalls(d, proj, "")
	officeCalls(d, elsewhere, "Jan")
	if dest, refused := d.mailTarget("Jan"); refused != nil || dest.ID != "be0001" {
		t.Fatalf("after the rename, Jan is %+v %+v, want Bea", dest, refused)
	}
	// Each as its route resolves it: mail (mailTarget, for the GM), contacts (setContacts), a watch (watchers.add).
	to, _ := send["to"].(string)
	if dest, refused := d.mailTarget(to); refused != nil || dest.ID != "ad0001" {
		t.Errorf("mail as checked went to %+v %+v, want Ada", dest, refused)
	}
	body, _ := json.Marshal(contacts)
	hires := readRegistry(registryPath(), false).hires
	if pairs, refused := setContacts(registryPath(), body, d.shownNames(hires)); refused != nil || len(pairs) != 1 || pairs[0].From != "ad0001" {
		t.Errorf("contacts as checked: %+v %+v, want Ada's", pairs, refused)
	}
	if made, refused := d.watchers.add(watch, "gm", nil); refused != nil || made.Target != "ad0001" {
		t.Errorf("a watch as checked: %+v %+v, want on Ada", made, refused)
	}
}

// RT126POLISHB N2: a bot keeps its name. The office's names are tried only when no bot has the name, so a hire's desk
// the office calls "Jan" doesn't make mail to the bot Jan ambiguous, for the owner, an unheld GM or a held one.
func TestABotKeepsItsNameWhenTheOfficeCallsAHiresDeskTheSame(t *testing.T) {
	d, gm, _, _, _ := heldNames(t) // Ada's desk is "Jan"
	d.mu.Lock()
	d.cloud.bots = append(d.cloud.bots, CloudBot{ID: "jb", Name: "Jan"})
	d.mu.Unlock()
	if _, refused := d.mailTarget("Jan"); refused == nil || refused.Message != "Jan has no mailbox folder. The owner sets one on the setup page." {
		t.Errorf("mail to Jan: %+v, want the bot found (it has no mailbox)", refused)
	}
	if h, refused := findHire("Jan", readRegistry(registryPath(), false).hires, d.shownNames(readRegistry(registryPath(), false).hires)); refused != nil || h.ID != "ad0001" {
		t.Errorf("elsewhere, with no bots, Jan is still Ada: %+v %+v", h, refused)
	}
	// A held GM never writes to a bot: the name is the bot's, so not one of its hires.
	if status, why, _ := d.gmScopeRefusal(gm, http.MethodPost, "/setup/gm/send", nil, []byte(`{"to":"Jan","text":"hi"}`)); !scopeRefused(status, why) {
		t.Errorf("a held GM's mail to Jan: %d %q", status, why)
	}
	// A bot and one of its hires by the registry's name: the route's own answer, as before, never the hire instead.
	d.mu.Lock()
	d.cloud.bots = append(d.cloud.bots, CloudBot{ID: "cb", Name: "Cy"})
	d.mu.Unlock()
	if status, why, _ := d.gmScopeRefusal(gm, http.MethodPost, "/setup/gm/send", nil, []byte(`{"to":"Cy","text":"hi"}`)); status != http.StatusConflict || why != mailMoreThanOne {
		t.Errorf("a held GM's mail to Cy: %d %q", status, why)
	}
	if _, refused := d.mailTarget("Cy"); refused == nil || refused.Status != http.StatusConflict || refused.Message != mailMoreThanOne {
		t.Errorf("mail to Cy: %+v", refused)
	}
}

// RT126POLISHB O1: `gm nickname` on the folder of a hire with a nickname of its own tells a GM held to one project to
// ask the owner, never to use the registry's edit route, which its scope refuses. An unheld GM keeps the route's words
// (TestGMNicknameOnANicknamedHiresFolderSaysWhereToRenameIt).
func TestAHeldGMIsToldToAskTheOwnerToRenameAHire(t *testing.T) {
	d, _, token, proj, _ := heldNames(t) // Cy, with a nickname of its own, works in proj, as the GM (Boss) does
	sessionAt(d, "s1", proj, "other:/dev/ttys002")
	body, _ := json.Marshal(map[string]string{"folder": proj, "name": "Crew"})
	status, text := gmPost(t, d, token, "/setup/gm/nickname", string(body))
	var answer map[string]string
	if status != http.StatusOK || json.Unmarshal([]byte(text), &answer) != nil {
		t.Fatalf("nickname: %d %q", status, text)
	}
	// The GM works there too, with a nickname of its own (keepTheirNicknames lists every open registered row).
	const note = "Only the office's name for the desk changed: Boss and Cy keep their own nicknames from the registry, which " +
		"gm team, mail and the gm commands go by. To rename a hire, ask the owner to change its nickname in the registry."
	if answer["note"] != note {
		t.Errorf("the note: %q\nwant %q", answer["note"], note)
	}
	if status, text := gmPost(t, d, token, "/setup/gm/registry", `{"id":"cy0001","nickname":"Cyd"}`); !scopeRefused(status, text) {
		t.Errorf("the registry's edit, for a held GM: %d %q", status, text)
	}
}

// fired fires the hire id in the registry, as the front desk keeps a fired hire: still there, no longer open.
func fired(t *testing.T, id string) {
	t.Helper()
	hires := readRegistry(registryPath(), false).hires
	for i := range hires {
		if hires[i].ID == id {
			hires[i].State = "fired"
		}
	}
	data, _ := json.Marshal(map[string]any{"version": 1, "hires": hires})
	if err := writeRegistryFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// RT126POLISHBR1 B1: a held GM's send or contacts with a name missing, null, not text or blank is never filled in with a
// hire it didn't name. Each answers the route's own 400, through the gate with the GM's token, and keeps nothing: no
// mail queued, no pair saved. Bea is fired, so Ada is the one open hire with no nickname, the hire a blank name matches.
// A name missing, null or not text reaches the route as sent; a blank one in contacts is the route's 400 at the gate,
// since the route would look it up as a name.
func TestAHeldGMsRequestWithNoNameAnswersTheRoutesOwn400(t *testing.T) {
	d, gm, token, _, _ := heldNames(t)
	fired(t, "be0001")
	if h, refused := findHire("", readRegistry(registryPath(), false).hires, nil); refused != nil || h.ID != "ad0001" {
		t.Fatalf("a blank name, as the routes find it: %+v %+v, want Ada", h, refused)
	}
	const mailShape = "Send {to, text, id}: who it is for, what to say, and an id of your own so that sending twice delivers once."
	for _, c := range []struct {
		path, body, want string
		asSent           bool // the gate lets it through to the route, as sent
	}{
		{"/setup/gm/send", `{"text":"hello","id":"p-1"}`, mailShape, true},
		{"/setup/gm/send", `{"to":"","text":"hello"}`, mailShape, true},
		{"/setup/gm/send", `{"to":5,"text":"hello"}`, mailShape, true},
		{"/setup/gm/send", `{"to":null,"text":"hello"}`, mailShape, true},
		{"/setup/gm/send", `{"to":" \t ","text":"hello"}`, mailShape, true},
		{"/setup/gm/contacts", `{"from":null,"to":"Cy","oneWay":true}`, contactsShape, true},
		{"/setup/gm/contacts", `{"to":"Cy"}`, contactsShape, true},
		{"/setup/gm/contacts", `{"from":7,"to":"Cy"}`, contactsShape, true},
		{"/setup/gm/contacts", `{"from":"","to":"Cy"}`, contactsShape, false},
		{"/setup/gm/contacts", `{"from":"Cy","to":"  "}`, contactsShape, false},
	} {
		status, why, given := d.gmScopeRefusal(gm, http.MethodPost, c.path, nil, []byte(c.body))
		if c.asSent {
			var sent map[string]any
			_ = json.Unmarshal([]byte(c.body), &sent)
			want, _ := json.Marshal(sent)
			got, _ := json.Marshal(given)
			if status != 0 || string(got) != string(want) {
				t.Errorf("%s %s: the gate gives %d %q, and the route %s, want it as sent", c.path, c.body, status, why, got)
			}
		} else if status != http.StatusBadRequest || why != c.want {
			t.Errorf("%s %s: the gate gives %d %q, want the route's 400", c.path, c.body, status, why)
		}
		if status, text := gmPost(t, d, token, c.path, c.body); status != http.StatusBadRequest || text != c.want {
			t.Errorf("%s %s: %d %q, want the route's own 400", c.path, c.body, status, text)
		}
	}
	if messages, _ := d.mail.list(nil, true)["messages"].([]map[string]any); len(messages) != 0 {
		t.Errorf("mail queued: %v", messages)
	}
	if pairs := readRegistry(registryPath(), false).contacts; len(pairs) != 0 {
		t.Errorf("pairs saved: %+v", pairs)
	}
	// A name the GM gave still goes through, as its hire's id.
	if status, text := gmPost(t, d, token, "/setup/gm/contacts", `{"from":"Jan","to":"Cy","oneWay":true}`); status != http.StatusOK ||
		!strings.Contains(text, `"from":"ad0001"`) {
		t.Errorf("contacts Jan to Cy: %d %q", status, text)
	}
}

// RT126POLISHBR1 N1: a held GM's /setup/preview names its hire as "hire", and the route finds it again (hireNamed, with
// the office's names). The route is given the hire as checked, by its id, so a rename in between can't move the
// preview to a desk outside the project.
func TestAHeldGMsPreviewReachesTheRouteAsTheHiresID(t *testing.T) {
	d, gm, token, proj, elsewhere := heldNames(t)
	body := `{"url":"http://localhost:3000","hire":"Jan"}`
	checked := heldChecked(t, d, gm, "/setup/preview", body)
	if got, _ := json.Marshal(checked); string(got) != `{"hire":"ad0001","url":"http://localhost:3000"}` {
		t.Errorf("the route is given %s, want Ada's id and the url as sent", got)
	}
	// Through the gate and the route: Ada has no desk here, and the route says so of Ada, the hire it was given.
	if status, text := gmPost(t, d, token, "/setup/preview", body); status != http.StatusConflict || text != "Adapter has no desk in the office right now." {
		t.Errorf("a preview for Jan: %d %q", status, text)
	}
	officeCalls(d, proj, "")
	officeCalls(d, elsewhere, "Jan")
	if h, refused := d.hireNamed("Jan"); refused != nil || h.ID != "be0001" {
		t.Fatalf("after the rename, Jan is %+v %+v, want Bea", h, refused)
	}
	// As the route resolves it (declarePreview: hireNamed of what it was given).
	hire, _ := checked["hire"].(string)
	if h, refused := d.hireNamed(hire); refused != nil || h.ID != "ad0001" {
		t.Errorf("the preview as checked is for %+v %+v, want Ada", h, refused)
	}
}
