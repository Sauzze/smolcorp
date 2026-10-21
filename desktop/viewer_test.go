package desktop

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTheViewersTestVector(t *testing.T) {
	// The spec's test vector, as the Mac app's self-test and viewer/scripts/check-vector.mjs seal it.
	id := "AAECAwQFBgcICQoLDA0ODw"
	key := viewerKeyOf("AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8")
	nonce := []byte{0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab}
	plain, ok := viewerPadded([]byte(`{"v":1,"at":1792000000000,"office":{"keys":[],"overflow":0}}`))
	if !ok || len(plain) != viewerBlock {
		t.Fatalf("the padded plaintext is %d bytes", len(plain))
	}
	body, err := viewerSeal(plain, key, id, nonce)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	if got := hex.EncodeToString(digest[:]); got != "aeb857fedacfd942ddda9f7b1824d43230bd580bf0f1ea25ea35972a88798327" {
		t.Errorf("the vector's body hashes to %s", got)
	}
	if body[0] != 0x01 || len(body) != 1+12+len(plain)+16 {
		t.Errorf("the body is %d bytes, starting %x", len(body), body[0])
	}
	if _, ok := viewerPadded(make([]byte, viewerMost+1)); ok {
		t.Error("a plaintext over the most was padded")
	}
}

func TestAViewerAddressIsAnOriginToSendTo(t *testing.T) {
	for given, want := range map[string]string{
		"https://office-viewer-x.vercel.app": "https://office-viewer-x.vercel.app",
		"  HTTPS://Office.Example.com//  ":   "https://office.example.com",
		"https://viewer.example:8443":        "https://viewer.example:8443",
		"http://localhost:7771":              "http://localhost:7771",
		"http://127.0.0.1:7771/":             "http://127.0.0.1:7771",
		"http://viewer.example":              "",
		"https://viewer.example/v/abc":       "",
		"https://viewer.example?x=1":         "",
		"https://viewer.example?":            "",
		"https://viewer.example#k=1":         "",
		"https://user:pass@viewer.example":   "",
		"https://vïewer.example":             "",
		"ftp://viewer.example":               "",
		"viewer.example":                     "",
		"":                                   "",
	} {
		got, ok := viewerOrigin(given)
		if ok != (want != "") || got != want {
			t.Errorf("%q is %q, %v; want %q", given, got, ok, want)
		}
	}
}

func TestViewerSettingsLoadFieldByField(t *testing.T) {
	var settings ViewerSettings
	json.Unmarshal([]byte(`{"baseURL": "https://v.example", "stopAfterHours": 5, "hideNames": "yes"}`), &settings)
	if settings != (ViewerSettings{BaseURL: "https://v.example", StopOnReturn: true, StopAfterHours: 8}) {
		t.Errorf("settings load as %+v", settings)
	}
	isolate(t)
	if config := loadConfig(); config.Viewer != standardViewerSettings() {
		t.Errorf("a new config's viewer is %+v", config.Viewer)
	}
}

// memorySecrets stands in for the secret store.
type memorySecrets struct {
	mu     sync.Mutex
	values map[string]string
	broken bool
}

var errMissing = errors.New("not found")

func (m *memorySecrets) store() viewerSecrets {
	m.values = map[string]string{}
	return viewerSecrets{
		get: func(account string) (string, error) {
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.broken {
				return "", errors.New("the store is locked")
			}
			value, ok := m.values[account]
			if !ok {
				return "", errMissing
			}
			return value, nil
		},
		set: func(account, value string) error {
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.broken {
				return errors.New("the store is locked")
			}
			m.values[account] = value
			return nil
		},
		delete:   func(account string) { m.mu.Lock(); delete(m.values, account); m.mu.Unlock() },
		notFound: func(err error) bool { return errors.Is(err, errMissing) },
	}
}

// relay stands in for the viewer, keeping what each request was.
type relay struct {
	mu       sync.Mutex
	requests []string
	bodies   map[string][]byte
	auth     []string
	status   int
}

func (r *relay) serve(t *testing.T) *httptest.Server {
	r.bodies = map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.requests = append(r.requests, req.Method+" "+req.URL.Path)
		r.auth = append(r.auth, req.Header.Get("Authorization"))
		if req.Method == http.MethodPut {
			r.bodies[req.URL.Path] = body
		}
		status := r.status
		r.mu.Unlock()
		if status == 0 {
			status = http.StatusNoContent
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server
}

func (r *relay) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.requests)
}

func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); !done(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("waited for %s", what)
		}
	}
}

// opened opens a viewer body with the link's key, as the page does.
func opened(t *testing.T, link string, body []byte) map[string]any {
	t.Helper()
	match := regexp.MustCompile(`/v/([A-Za-z0-9_-]{22})#k=([A-Za-z0-9_-]{43})$`).FindStringSubmatch(link)
	if match == nil {
		t.Fatalf("the link %q isn't a viewer link", link)
	}
	key, _ := base64.RawURLEncoding.DecodeString(match[2])
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	if len(body) < 29 || body[0] != 0x01 {
		t.Fatalf("a body of %d bytes", len(body))
	}
	plain, err := gcm.Open(nil, body[1:13], body[13:], []byte("smolcorp-viewer:v1:"+match[1]))
	if err != nil {
		t.Fatalf("the body doesn't open with the link's key: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(plain, &payload); err != nil || payload["v"] != float64(1) {
		t.Fatalf("the plaintext is %s", plain)
	}
	return payload
}

// portableDeck is a deck whose portable office has a stand-in store, set up
// to send to a stand-in viewer.
func portableDeck(t *testing.T) (*Deck, *relay, *memorySecrets) {
	t.Helper()
	secrets := &memorySecrets{}
	saved := viewerStore
	viewerStore = secrets.store()
	t.Cleanup(func() { viewerStore = saved })
	d := newTestDeck(t)
	r := &relay{}
	server := r.serve(t)
	d.config.Viewer.BaseURL = strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	d.portable.setSettings(d.config.Viewer)
	d.portable.presence = func() (time.Duration, bool, bool) { return 0, false, false }
	t.Cleanup(func() { d.portable.Stop(0) })
	return d, r, secrets
}

func TestThePortableOfficeSendsTheOfficeLocked(t *testing.T) {
	d, r, _ := portableDeck(t)
	if d.portable.IsSetUp() {
		t.Fatal("set up with no write secret")
	}
	d.portable.Start()
	if d.portable.IsOn() {
		t.Fatal("on with no write secret")
	}
	secret, err := d.portable.MakeWriteSecret()
	if err != nil || len(secret) != 43 {
		t.Fatalf("a write secret: %q, %v", secret, err)
	}
	running(d, 1)
	d.portable.Start()
	link := d.portable.Link()
	id := regexp.MustCompile(`/v/([^#]+)#`).FindStringSubmatch(link)[1]
	waitFor(t, "the first upload", func() bool { return slices.Contains(r.seen(), "PUT /api/s/"+id) })
	r.mu.Lock()
	body, auth := r.bodies["/api/s/"+id], r.auth[0]
	r.mu.Unlock()
	if auth != "Bearer "+secret {
		t.Errorf("sent with %q, want the write secret", auth)
	}
	payload := opened(t, link, body)
	office := payload["office"].(map[string]any)
	desk := office["keys"].([]any)[0].(map[string]any)["session"].(map[string]any)
	if desk["id"] != "v1" || desk["name"] != "p0" || desk["keys"] != false {
		t.Errorf("the desk is sent as %v", desk)
	}
	if strings.Contains(string(body), `"name":"p0"`) {
		t.Error("the project's name is readable in what's sent")
	}

	d.portable.Stop(time.Second)
	waitFor(t, "the viewer told to forget it", func() bool { return slices.Contains(r.seen(), "DELETE /api/s/"+id) })
	if d.portable.IsOn() {
		t.Error("still on after Stop")
	}
}

func TestRotatingTheKeyForgetsTheOldLink(t *testing.T) {
	d, r, _ := portableDeck(t)
	d.portable.MakeWriteSecret()
	old := d.portable.Link()
	revision := d.portable.SetupState(false)["revision"]
	d.portable.RotateKey()
	link := d.portable.Link()
	if link == old || link == "" || d.portable.SetupState(false)["revision"] == revision {
		t.Fatalf("after rotating, the link is %q (was %q)", link, old)
	}
	oldID := regexp.MustCompile(`/v/([^#]+)#`).FindStringSubmatch(old)[1]
	waitFor(t, "the old id forgotten", func() bool { return slices.Contains(r.seen(), "DELETE /api/s/"+oldID) })

	d.portable.Wipe()
	if !slices.Contains(r.seen(), "DELETE /api/s") || d.portable.SetupState(false)["checked"] != "Viewer wiped ✓" {
		t.Errorf("wiping: %v, %v", r.seen(), d.portable.SetupState(false)["checked"])
	}
	d.portable.Check()
	if state := d.portable.SetupState(false); state["checked"] != "Viewer ready ✓" {
		t.Errorf("checking: %v", state["checked"])
	}
	r.mu.Lock()
	r.status = http.StatusUnauthorized
	r.mu.Unlock()
	if d.portable.Check(); d.portable.SetupState(false)["checked"] != "The viewer refused the write secret" {
		t.Errorf("a refused check: %v", d.portable.SetupState(false)["checked"])
	}
}

func TestAViewerThatRedirectsIsNotFollowed(t *testing.T) {
	d, _, _ := portableDeck(t)
	elsewhere := &relay{}
	other := elsewhere.serve(t)
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, other.URL+req.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirecting.Close)
	d.config.Viewer.BaseURL = strings.Replace(redirecting.URL, "127.0.0.1", "localhost", 1)
	d.portable.setSettings(d.config.Viewer)
	d.portable.MakeWriteSecret()
	d.portable.Check()
	if got := d.portable.SetupState(false)["checked"]; got != "The viewer said no (307)" || len(elsewhere.seen()) != 0 {
		t.Errorf("a redirect: %v, and the other server was sent %v", got, elsewhere.seen())
	}
}

func TestAKeyThatCantBeReadIsNeverReplaced(t *testing.T) {
	d, _, secrets := portableDeck(t)
	d.portable.MakeWriteSecret()
	link := d.portable.Link()
	secrets.mu.Lock()
	secrets.broken = true
	secrets.mu.Unlock()
	d.portable.Start()
	if d.portable.IsOn() {
		t.Fatal("on with a store that can't be read")
	}
	secrets.mu.Lock()
	secrets.broken = false
	secrets.mu.Unlock()
	if d.portable.Link() != link {
		t.Error("a key that couldn't be read was replaced, and every link given out with it")
	}
}

func TestSetupKeepsTheViewersSettings(t *testing.T) {
	d, _, _ := portableDeck(t)
	post := func(target, body string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: target, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}, body: body})
	}
	if response := post("/setup/viewer", `{"baseURL": "http://viewer.example"}`); response.Code != 400 {
		t.Errorf("an http address: %d", response.Code)
	}
	if response := post("/setup/viewer", `{"baseURL": "https://viewer.example/", "hideNames": true, "stopAfterHours": 12}`); response.Code != 200 ||
		d.config.Viewer != (ViewerSettings{BaseURL: "https://viewer.example", HideNames: true, StopOnReturn: true, StopAfterHours: 12}) {
		t.Errorf("saving: %d, %+v", response.Code, d.config.Viewer)
	}
	if post("/setup/viewer", `{"stopAfterHours": 7}`); d.config.Viewer.StopAfterHours != 12 {
		t.Errorf("an hour that isn't a choice was kept: %d", d.config.Viewer.StopAfterHours)
	}
	made := post("/setup/viewer/secret", "")
	var reply map[string]any
	json.Unmarshal(made.Body.Bytes(), &reply)
	if made.Code != 200 || reply["secret"] == nil || reply["link"] != nil || reply["hasLink"] != true {
		t.Errorf("making a secret: %d %v", made.Code, reply)
	}
	// Only the GET carries the link, and never the secret.
	got := serve(t, d, call{method: "GET", target: "/setup/viewer", host: "localhost:7710", from: thisComputer})
	json.Unmarshal(got.Body.Bytes(), &reply)
	if link, _ := reply["link"].(string); !strings.HasPrefix(link, "https://viewer.example/v/") || strings.Contains(got.Body.String(), reply["secret"].(string)) {
		t.Errorf("the viewer's state: %s", got.Body)
	}
	if qr := serve(t, d, call{method: "GET", target: "/setup/viewer/qr.png", host: "localhost:7710", from: thisComputer}); qr.Code != 200 ||
		qr.Header().Get("Content-Type") != "image/png" {
		t.Errorf("the link's QR code: %d", qr.Code)
	}
	if data, _ := os.ReadFile(configFile); strings.Contains(string(data), reply["secret"].(string)) || !strings.Contains(string(data), `"viewer"`) {
		t.Errorf("the config holds the secret, or not the settings:\n%s", data)
	}
}

func TestBackAtTheComputerStopsIt(t *testing.T) {
	d, _, _ := portableDeck(t)
	d.portable.MakeWriteSecret()
	d.portable.Start()
	d.portable.mu.Lock()
	started := d.portable.since
	d.portable.mu.Unlock()
	// Back within the grace, as after launch then lock: looked at again once it's over.
	d.portable.cameBack(started, time.Now())
	if !d.portable.IsOn() {
		t.Fatal("stopped as soon as it was launched")
	}
	// Long after it was launched, you're back.
	d.portable.mu.Lock()
	d.portable.since = time.Now().Add(-time.Hour)
	d.portable.mu.Unlock()
	d.portable.cameBack(started, time.Now())
	if !d.portable.IsOn() {
		t.Error("an old start's watcher stopped a newer start")
	}
	d.portable.mu.Lock()
	started = d.portable.since
	d.portable.settings.StopOnReturn = false
	d.portable.mu.Unlock()
	if d.portable.cameBack(started, time.Now()); !d.portable.IsOn() {
		t.Error("stopped when back, with Stop when I'm back off")
	}
	d.portable.mu.Lock()
	d.portable.settings.StopOnReturn = true
	d.portable.mu.Unlock()
	if d.portable.cameBack(started, time.Now()); d.portable.IsOn() {
		t.Error("still on with you back")
	}
}

func TestLogindSaysWhetherYoureAway(t *testing.T) {
	now := time.UnixMicro(1_792_000_000_000_000)
	idle, locked, ok := parseLogindPresence("LockedHint=yes\nIdleHint=yes\nIdleSinceHint=1791999400000000\n", now)
	if !ok || !locked || idle != 10*time.Minute {
		t.Errorf("locked and idle: %v, %v, %v", idle, locked, ok)
	}
	if idle, locked, ok := parseLogindPresence("LockedHint=no\nIdleHint=no\nIdleSinceHint=0\n", now); !ok || locked || idle != 0 {
		t.Errorf("at work: %v, %v, %v", idle, locked, ok)
	}
	if _, _, ok := parseLogindPresence("", now); ok {
		t.Error("an empty answer was taken for one")
	}
}

// redactionProblems is the Mac app's self-test, ViewerSelfTest.redactionProblems:
// a deck with something sensitive in every field that must not leave.
func redactionProblems(t *testing.T, hideNames bool) []string {
	t.Helper()
	d := newTestDeck(t)
	role := shippedRoles[0]
	role.Instructions, role.Questions = "secret-instructions", []string{"secret-question"}
	// A role you made: its id and name are your own text.
	own := Role{ID: "secret-role-id", Name: "secret-role-name", Agent: "claude", Instructions: "secret-instructions", Badge: role.Badge}
	d.config.Roles = []Role{role, own}
	d.config.Pin("/Users/someone/pinned-secret", 0)
	var room Room
	json.Unmarshal([]byte(`{"team": {"secret-folder": {"name": "secret-nickname", "outfit": "suit"},
		"second-secret-folder": {"name": "other-nickname"}, "other-secret-project": {}},
		"dress": {"rules": [{"contains": "secret-rule", "size": "top"}]},
		"plan": {"wide": {"desk1": {"x": 4, "thing": "duck"}}, "added": {"wide": [{"kind": "piano", "x": 3}]},
		"hung": {"tall": [{"kind": "map", "x": 40}]}}}`), &room)
	d.config.Layout.Room = room
	d.config.Layout.Rows = []ButtonRow{{Buttons: []Button{{ID: "secret-button", Text: "secret-button-text"}}}}

	d.launcherConnected("L1", Claude, "/Users/someone/secret-folder", role.ID, "")
	d.launcherConnected("L2", Claude, "/Users/someone/second-secret-folder", own.ID, "")
	// Each a moment after the last, so they sit in the order they start.
	started := 0.0
	start := func(id, tab, folder, project string) *Session {
		started++
		d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": id, "cwd": folder},
			Claude, tab, project, started)
		return d.sessions.Get(id)
	}
	s := start("real-session-id-1234", "launcher:L1", "/Users/someone/secret-folder", "/Users/someone/secret-project")
	s.State, s.Effort, s.Tool, s.ToolActivity, s.WaitingFor = Asking, "high", "Bash rm -rf /tmp/secret-tool", "secret-activity", "secret-waiting"
	s.Model, s.Messaged, s.MessagedAt, s.MaxPressedAt = "claude-secret-model", "secret-message", 5, 6
	s.Transcript, s.ErrorKind = "/Users/someone/.claude/secret-transcript.jsonl", "secret-error-kind"
	s.Subagents = []Subagent{{ID: "real-subagent-id", Type: "Explore", Model: "claude-secret-submodel"}, {ID: "real-subagent-id-2", Type: "secret-agent-type"}}
	start("real-session-id-5678", "launcher:L2", "/Users/someone/second-secret-folder", "/Users/someone/second-secret-project")
	up1 := start("real-session-id-up-1", "headless:real-session-id-up-1", "/Users/someone/up-folder", "/Users/someone/up-folder")
	up1.ParentFolder = s.Project
	up2 := start("real-session-id-up-2", "headless:real-session-id-up-2", "/Users/someone/up-folder-2", "/Users/someone/up-folder-2")
	up2.ParentFolder = "/Users/someone/secret-hirer"

	aliases := map[string]string{}
	d.mu.Lock()
	office := d.viewerSnapshot(hideNames, func(id string) string {
		if _, ok := aliases[id]; !ok {
			aliases[id] = "v" + string(rune('1'+len(aliases)))
		}
		return aliases[id]
	})
	d.mu.Unlock()
	plain, ok := viewerPlaintext(office, 1)
	if !ok {
		return []string{"couldn't serialise the snapshot"}
	}
	text := string(plain)
	var problems []string
	for _, key := range []string{"activity", "tool", "messaged", "messagedAt", "maxPressedAt", "voiceNote", "instructions",
		"questions", "transcript", "folder", "tab", "tty", "secret", "token", "phone", "summaryText"} {
		if strings.Contains(text, `"`+key+`":`) {
			problems = append(problems, "key "+key)
		}
	}
	forbidden := []string{"/Users", "real-session-id", "real-subagent-id", "secret-tool", "secret-activity", "secret-waiting",
		"secret-model", "secret-submodel", "secret-message", "secret-transcript", "secret-instructions", "secret-question",
		"secret-button", "other-secret-project", "secret-rule"}
	if hideNames {
		forbidden = append(forbidden, "secret-folder", "secret-project", "secret-nickname", "other-nickname", "pinned-secret",
			"secret-hirer", "up-folder", "secret-error-kind", "secret-agent-type", "secret-role-id", "secret-role-name")
	}
	for _, needle := range forbidden {
		if strings.Contains(text, needle) {
			problems = append(problems, "text "+needle)
		}
	}
	// Read back as the page reads it.
	var payload struct {
		Office struct {
			Keys []struct {
				Session map[string]any `json:"session"`
			} `json:"keys"`
			Layout   map[string]any `json:"layout"`
			Upstairs struct {
				Desks []struct {
					HiredBy *struct{ Name string } `json:"hiredBy"`
				} `json:"desks"`
			} `json:"upstairs"`
		} `json:"office"`
	}
	json.Unmarshal(plain, &payload)
	keys := payload.Office.Keys
	if len(keys) != 3 || keys[2].Session == nil {
		return append(problems, "no sessions at the desks")
	}
	if _, has := keys[1].Session["model"]; has {
		problems = append(problems, "session.model present")
	}
	for _, control := range []string{"dial", "mic", "dictates", "keys", "fire", "summary"} {
		if keys[2].Session[control] != false {
			problems = append(problems, "control "+control+" not false")
		}
	}
	if rows, _ := payload.Office.Layout["rows"].([]any); rows == nil || len(rows) != 0 {
		problems = append(problems, "layout.rows not empty")
	}
	// Who hired each run upstairs: by name, or with names hidden, the hiring desk's number or "Downstairs".
	hirers := []string{}
	for _, desk := range payload.Office.Upstairs.Desks {
		if desk.HiredBy != nil {
			hirers = append(hirers, desk.HiredBy.Name)
		}
	}
	want := []string{"secret-project", "secret-hirer"}
	if hideNames {
		want = []string{"Desk 2", "Downstairs"}
	}
	if !slices.Equal(hirers, want) {
		problems = append(problems, "upstairs hiredBy "+strings.Join(hirers, ", "))
	}
	roomObject, _ := payload.Office.Layout["room"].(map[string]any)
	if plan, _ := roomObject["plan"].(map[string]any); plan["added"] == nil {
		problems = append(problems, "room.plan.added missing")
	}
	if hideNames {
		team, _ := roomObject["team"].(map[string]any)
		for key := range team {
			if !regexp.MustCompile(`^Desk \d+$`).MatchString(key) {
				problems = append(problems, "room.team key "+key)
			}
		}
		// The raw room: with names hidden, every text in it is one of the room's own choices.
		known := []string{"", "auto", "none", "bare"}
		for _, choices := range roomChoices {
			known = append(known, choices...)
		}
		for _, choices := range teamChoices {
			known = append(known, choices...)
		}
		known = append(append(append(known, wallChoices...), pets...), roleBadges...)
		var texts func(value any, path string)
		texts = func(value any, path string) {
			switch v := value.(type) {
			case string:
				if !slices.Contains(known, v) {
					problems = append(problems, "room text at "+path)
				}
			case []any:
				for i, item := range v {
					texts(item, path+"["+string(rune('0'+i))+"]")
				}
			case map[string]any:
				for name, item := range v {
					texts(item, path+"."+name)
				}
			}
		}
		texts(roomObject, "room")
	}
	return problems
}

func TestTheViewerIsSentNothingThatMustNotLeave(t *testing.T) {
	for _, hide := range []bool{false, true} {
		if problems := redactionProblems(t, hide); len(problems) > 0 {
			t.Errorf("with names hidden %v: %s", hide, strings.Join(problems, "; "))
		}
	}
}
