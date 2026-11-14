package desktop

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// RT126SK1R4 follow-ups 2 and 3: while the records' key can't be read, nothing is saved, so what takes something away
// would seem to apply, then come back at the restart after the owner unlocks the keychain. Each is refused instead,
// in its own route's answer, with Setup's reason; and the GM's new token isn't kept, so its token from before goes on
// working. After the unlocked restart everything is as the owner was told: nothing was taken away.

// heldRun is a support folder with something of each kind to take away, under a deck whose records' key can't be
// read now (a restart with the keychain locked): the held deck, the stand-in, and what to check after the unlocked
// restart.
type heldRun struct {
	held, first       *Deck
	mac               *macStandIn
	phoneKey, macKey  string
	hire02Key         string
	phoneID, macID    string
	mailbox, otherBox string
	before            map[string][]byte
}

// heldViewer is the viewer's options and address startHeld saves: names hidden, stopping soon.
var heldViewer = ViewerSettings{BaseURL: "https://office-viewer-fake.vercel.app", HideNames: true, StopOnReturn: true, StopAfterHours: 2}

// heldFolder is a folder of the test's own, as its real path, for a bot's mailbox.
func heldFolder(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func startHeld(t *testing.T) heldRun {
	t.Helper()
	d, mac, phoneKey, _ := macDeck(t)
	withAHire(t)
	hire02Key, err := newSendKey(registryPath(), "hire02")
	if err != nil || hire02Key == "" {
		t.Fatalf("hire02's send key: %v", err)
	}
	d.mu.Lock()
	d.config.KeystrokesAllowed, d.config.FloorsFromOtherMacs, d.config.ManagerEnabled, d.config.HiringEnabled = true, true, true, true
	d.config.GMByHand, d.config.PhoneTextWiFi, d.config.SummaryOptIn = true, true, true
	d.config.Webhook = Webhook{Enabled: true, URL: "http://127.0.0.1:9/news", Secret: "FAKE-hook-secret", Events: []string{"done"}}
	// RT126SK1F B1: what the first table missed. Claude's clean-up, the viewer's options and address, a bot and its
	// mailbox, Summary's provider, and scrolling.
	mailbox, otherBox := heldFolder(t), heldFolder(t)
	d.config.DictationCleanup, d.config.ScrollEnabled, d.config.Viewer = true, true, heldViewer
	d.config.CloudBots = checkedCloudBots([]CloudBot{{ID: "grok-1", Name: "Grokky", Mailbox: mailbox}})
	d.config.SummaryProvider = string(providerAPIKey)
	d.config.save()
	macKey := d.macPeers.pair("Laptop")
	run := heldRun{first: d, mac: mac, phoneKey: phoneKey, macKey: macKey, hire02Key: hire02Key, mailbox: mailbox, otherBox: otherBox,
		phoneID: d.access.Phones.WithKey(phoneKey).ID, macID: d.macPeers.withKey(macKey).ID}
	d.mu.Unlock()
	run.before = sealedCopies(t)
	mac.recordLook = "unreadable"
	run.held = restart(t)
	if !recordsHeldOnly() {
		t.Fatalf("the records' state: %q", recordsState())
	}
	frontDeskOnWhileHeld(t, run.held)
	return run
}

// frontDeskOnWhileHeld switches the front desk on in a run whose records' key can't be read, as the owner would to
// reach the GM's routes: the core holds the safe defaults then, the front desk off, and a switch on is taken (held in
// memory for the run, as anything switched on is).
func frontDeskOnWhileHeld(t *testing.T, d *Deck) {
	t.Helper()
	if code, _, body := fromPage(t, d, http.MethodPost, "/setup/gm/manager?enabled=1", nil); code != http.StatusOK {
		t.Fatalf("the front desk switched on while the key can't be read: %d %s", code, body)
	}
}

// afterTheUnlockedRestart checks that nothing was written while the key couldn't be read, then restarts with it
// readable and checks that everything taken away in the held run is still there.
func (run heldRun) afterTheUnlockedRestart(t *testing.T) {
	t.Helper()
	if after := sealedCopies(t); !maps.EqualFunc(run.before, after, bytes.Equal) {
		t.Errorf("a sealed file was written while the key can't be read: %v", slices.Sorted(maps.Keys(after)))
	}
	run.mac.recordLook = ""
	again := restart(t)
	if state, dropped := recordsState(), recordsDroppedText(); state != recordsOK || dropped != "" {
		t.Fatalf("the unlocked restart: %q, dropped %q", state, dropped)
	}
	f := readRegistry(registryPath(), false)
	hire02 := slices.IndexFunc(f.hires, func(h Hire) bool { return h.ID == "hire02" })
	again.mu.Lock()
	defer again.mu.Unlock()
	c := again.config
	got := map[string]bool{
		"hire02 on the team":         hire02 >= 0 && isOpen(f.hires[hire02]),
		"hire02's key works":         isHireKey(registryPath(), run.hire02Key),
		"the contact":                len(f.contacts) == 1,
		"the phone":                  again.access.Phones.WithKey(run.phoneKey) != nil,
		"the Mac":                    again.macPeers.withKey(run.macKey) != nil,
		"keystrokes":                 c.KeystrokesAllowed,
		"floors from other Macs":     c.FloorsFromOtherMacs,
		"the front desk":             c.ManagerEnabled,
		"hiring":                     c.HiringEnabled,
		"a GM started by hand":       c.GMByHand,
		"typing over Wi-Fi":          c.PhoneTextWiFi,
		"Summary":                    c.SummaryOptIn,
		"the webhook, as it was set": c.Webhook.Enabled && c.Webhook.URL == "http://127.0.0.1:9/news" && c.Webhook.Secret == "FAKE-hook-secret",
		"Claude's clean-up":          c.DictationCleanup,
		"scrolling":                  c.ScrollEnabled,
		"the viewer, as it was set":  c.Viewer == heldViewer,
		"the bot and its mailbox":    len(c.CloudBots) == 1 && c.CloudBots[0].ID == "grok-1" && c.CloudBots[0].Mailbox == run.mailbox,
		"Summary's provider":         c.SummaryProvider == string(providerAPIKey),
	}
	for what, there := range got {
		if !there {
			t.Errorf("after the unlocked restart, %s isn't as it was", what)
		}
	}
}

func TestOnAMacWhileTheKeyIsUnreadableEveryRevocationIsRefused(t *testing.T) {
	rows := []struct {
		name, method, target, body string
		// direct, for a change no route of the page's makes: its status and words.
		direct func(run heldRun) (int, string)
	}{
		{"hire02 let go", http.MethodPost, "/setup/gm/registry", `{"id":"hire02","state":"fired"}`, nil},
		{"a contact removed", http.MethodPost, "/setup/gm/contacts", `{"from":"hire01","to":"hire02","remove":true}`, nil},
		{"the phone unpaired", http.MethodPost, "/setup/unpair?phone=%phone", "", nil},
		{"every phone unpaired", http.MethodPost, "/setup/unpair?all=1", "", nil},
		{"the Mac revoked", http.MethodPost, "/setup/peers/revoke?peer=%mac", "", nil},
		{"floors from other Macs off", http.MethodPost, "/setup/peers?enabled=0", "", nil},
		{"keystrokes off", http.MethodPost, "/setup/keystrokes?enabled=0", "", nil},
		{"the front desk off", http.MethodPost, "/setup/gm/manager?enabled=0", "", nil},
		{"hiring off", http.MethodPost, "/setup/hiring?enabled=0", "", nil},
		{"a GM started by hand off", http.MethodPost, "/setup/gm/by-hand?enabled=0", "", nil},
		{"the webhook off", http.MethodPost, "/setup/webhook", `{"enabled":false}`, nil},
		{"the webhook's address changed", http.MethodPost, "/setup/webhook", `{"url":"http://127.0.0.1:9/other"}`, nil},
		{"typing over Wi-Fi off", http.MethodPost, "/setup/wifityping?on=0", "", nil},
		{"Summary off", http.MethodPost, "/setup/summaries?enabled=0", "", nil},
		// RT126SK1F B1.
		{"Claude's clean-up off, from Setup", http.MethodPost, "/setup/dictation?cleanup=0", "", nil},
		{"Claude's clean-up off, from the mic's switch", "", "", "", func(run heldRun) (int, string) { return run.held.setCleanup("0") }},
		{"the viewer showing names", http.MethodPost, "/setup/viewer", `{"hideNames":false}`, nil},
		{"the viewer stopping later", http.MethodPost, "/setup/viewer", `{"stopOnReturn":false,"stopAfterHours":0}`, nil},
		{"the viewer's address cleared", http.MethodPost, "/setup/viewer", `{"baseURL":""}`, nil},
		{"the viewer's address moved", http.MethodPost, "/setup/viewer", `{"baseURL":"https://office-viewer-other.vercel.app"}`, nil},
		{"a bot removed", http.MethodPost, "/setup/bots", `{"bots":[]}`, nil},
		{"a bot hidden", http.MethodPost, "/setup/bots/override", `{"id":"grok-1","hidden":true}`, nil},
		{"a bot's mailbox cleared", http.MethodPost, "/setup/bots/mailbox", `{"id":"grok-1","folder":""}`, nil},
		{"a bot's mailbox moved", http.MethodPost, "/setup/bots/mailbox", `{"id":"grok-1","folder":"%otherBox"}`, nil},
		{"Summary's provider changed", "", "", "", func(run heldRun) (int, string) {
			// As /setup/summary/provider and /setup/summaries ask, once the provider works (Deck.summaryRefusal).
			run.held.mu.Lock()
			defer run.held.mu.Unlock()
			return http.StatusConflict, run.held.config.heldRefusal(func(c *Config) { c.SummaryProvider = "codex" })
		}},
		{"scrolling off", http.MethodPost, "/setup/scroll?enabled=0", "", nil},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			run := startHeld(t)
			code, said := 0, ""
			if row.direct != nil {
				code, said = row.direct(run)
			} else {
				target := strings.NewReplacer("%phone", run.phoneID, "%mac", run.macID).Replace(row.target)
				body := strings.ReplaceAll(row.body, "%otherBox", run.otherBox)
				w := serve(t, run.held, call{method: row.method, target: target, host: "localhost:7710", from: thisComputer,
					body: body, headers: map[string]string{setupHeader: "1"}})
				code, said = w.Code, w.Body.String()
			}
			if code != http.StatusConflict || !strings.Contains(said, recordsHeldAnswer) {
				t.Errorf("%s while the key can't be read: %d %s", row.name, code, said)
			}
			run.afterTheUnlockedRestart(t)
		})
	}
}

// RT126SK1F B1 and nit 2: while the key can't be read, a plain setting changes for the run; a switch turned on in it,
// off in the file, can be turned off again (nothing comes back); and a bot or a role added is taken.
func TestOnAMacWhileTheKeyIsUnreadableWhatTakesNothingAwayIsTaken(t *testing.T) {
	_, mac, _, _ := macDeck(t)
	mac.recordLook = "unreadable"
	held := restart(t)
	for _, step := range []struct{ name, target, body string }{
		{"the front desk on, off in the file", "/setup/gm/manager?enabled=1", ""},
		{"the front desk off again", "/setup/gm/manager?enabled=0", ""},
		{"typing over Wi-Fi on", "/setup/wifityping?on=1", ""},
		{"typing over Wi-Fi off again", "/setup/wifityping?on=0", ""},
		{"the music on", "/setup/music?enabled=1", ""},
		{"the music off", "/setup/music?enabled=0", ""},
		{"a bot added", "/setup/bots", `{"bots":[{"id":"grok-9","name":"Nine"}]}`},
		{"the bot renamed", "/setup/bots", `{"bots":[{"id":"grok-9","name":"Niner"}]}`},
		{"the bot removed again", "/setup/bots", `{"bots":[]}`},
	} {
		w := serve(t, held, call{method: http.MethodPost, target: step.target, host: "localhost:7710", from: thisComputer,
			body: step.body, headers: map[string]string{setupHeader: "1"}})
		if w.Code >= 300 {
			t.Errorf("%s while the key can't be read: %d %s", step.name, w.Code, w.Body)
		}
	}
	// With hiring and the apps keys may go to on in the file, off in memory (their safe defaults): hiring's agent
	// changed alone, and an app added in this run taken off again, take nothing the file holds.
	run := startHeld(t)
	if code, _, body := fromPage(t, run.held, http.MethodPost, "/setup/hiring?agent=codex", nil); code != http.StatusOK {
		t.Errorf("hiring's agent changed while the key can't be read: %d %s", code, body)
	}
	if !heldChangeTakes("keystrokeApps", json.RawMessage(`["com.example.mail"]`), json.RawMessage(`["com.example.notes"]`), json.RawMessage(`[]`)) {
		t.Error("an app added in this run can't be taken off again")
	}
	run.afterTheUnlockedRestart(t)
}

// RT126SK1F B1: the allow-list itself. Every setting config.json holds is plain, a switch, a list, or else refused
// whenever it would come back at the restart: a new setting is refused until someone names it here. And every
// name the lists give is a setting.
func TestEverySettingIsPlainOrRefusedWhileTheKeyIsUnreadable(t *testing.T) {
	settings := knownConfigKeys()
	for _, list := range []map[string]bool{heldPlainSettings, heldSwitches, heldLists} {
		for name := range list {
			if !settings[name] {
				t.Errorf("%q is listed, but isn't a setting", name)
			}
		}
	}
	for name := range heldListParts {
		if !heldLists[name] {
			t.Errorf("%q has parts, but isn't a list", name)
		}
	}
	fields := configFields(loadConfig())
	for name := range settings {
		lists := 0
		for _, list := range []map[string]bool{heldPlainSettings, heldSwitches, heldLists} {
			if list[name] {
				lists++
			}
		}
		if lists > 1 {
			t.Errorf("%q is in %d lists", name, lists)
		}
		if _, compared := fields[name]; !compared && !strings.Contains(reflectTag(name), "omitempty") {
			t.Errorf("%q isn't compared", name)
		}
		// Taken back to something else, or taken off: refused unless plain.
		for _, change := range [][2]string{{`true`, `false`}, {`"a"`, `"b"`}, {`1`, `0`}, {`["a"]`, `[]`}, {`{"a":1}`, `{"a":2}`}} {
			saved, now := json.RawMessage(change[0]), json.RawMessage(change[1])
			if took := heldChangeTakes(name, saved, saved, now); took != heldPlainSettings[name] {
				t.Errorf("%q from %s to %s while the key can't be read: taken %v", name, change[0], change[1], took)
			}
		}
		// Back as the file has it: nothing comes back, so it's taken.
		if !heldChangeTakes(name, json.RawMessage(`"a"`), json.RawMessage(`"b"`), json.RawMessage(`"a"`)) {
			t.Errorf("%q set as the file has it is refused", name)
		}
	}
}

// reflectTag is the json tag of the Config field config.json calls name.
func reflectTag(name string) string {
	kind := reflect.TypeOf(Config{})
	for i := 0; i < kind.NumField(); i++ {
		if tag := kind.Field(i).Tag.Get("json"); strings.Split(tag, ",")[0] == name {
			return tag
		}
	}
	return ""
}

// Setup says what's refused, the GM's token among it, and that unlocking and restarting loses nothing.
func TestSetupSaysWhatsRefusedWhileTheKeyIsUnreadable(t *testing.T) {
	said := recordsText(recordsUnreadable, false)
	for _, words := range []string{"refuses what would take something away", "switching off or changing any other setting",
		"A fire, with hiring on, ends the hire's session and stops its key until the restart", "unless you hired it meanwhile: that one is fired for good",
		"give the General Manager a new token", "its token from before works again once you unlock the keychain and restart",
		"restart Smolcorp: that loses nothing you saved before"} {
		if !strings.Contains(said, words) {
			t.Errorf("Setup's line doesn't say %q: %s", words, said)
		}
	}
}

// Follow-up 3: the GM's token rotated, the GM hired again, or its token taken away (a fire, a failed hire) while the
// key can't be read writes nothing over its sealed verifier: the rotation and the hire are refused with Setup's
// reason, and the token from before works after the unlocked restart.
func TestOnAMacWhileTheKeyIsUnreadableTheGMsTokenFromBeforeKeepsWorking(t *testing.T) {
	d, mac, _, _ := macDeck(t)
	withHiredGM(t, "working")
	token := d.newGMToken()
	if token == "" || !d.gmKept.flush(3*time.Second) {
		t.Fatal("no token for the GM")
	}
	item, _ := mac.sealedItem()
	mac.recordLook = "unreadable"
	held := restart(t)
	frontDeskOnWhileHeld(t, held)
	if code, _, body := fromPage(t, held, http.MethodPost, "/setup/gm/token/rotate", nil); code != http.StatusConflict ||
		!strings.Contains(body, recordsHeldAnswer) {
		t.Errorf("the rotation while the key can't be read: %d %s", code, body)
	}
	if fresh := held.newGMToken(); fresh != "" {
		t.Error("the GM was given a new token while the key can't be read")
	}
	held.clearGMToken()
	held.gmKept.flush(3 * time.Second)
	if now, _ := mac.sealedItem(); now != item {
		t.Errorf("the GM's verifier was written while the key can't be read: %q", now)
	}
	mac.recordLook = ""
	again := restart(t)
	again.mu.Lock()
	works := again.isGMToken(token)
	again.mu.Unlock()
	if recordsState() != recordsOK || !works {
		t.Errorf("after the unlocked restart %q, the GM's token from before works: %v", recordsState(), works)
	}
}

// RT126SK1F nit 5, as SK2 leaves it: a start while the key can't be read loses nothing of the GM's token. No file
// holds it any more (SK2): a gm-token file an earlier version wrote goes, unread, at every start, held or not, and
// the token kept from before works again after the unlocked restart. A GM started by hand asks again after any
// restart (smolcorp gm login), held or not.
func TestOnAMacTheGMsTokenOutlastsAStartWhileTheKeyIsUnreadable(t *testing.T) {
	d, mac, _, _ := macDeck(t)
	withHiredGM(t, "working")
	token := d.newGMToken()
	d.mu.Lock()
	d.config.GMByHand, d.config.ManagerEnabled = true, true
	d.config.save()
	_ = d.applyByHand()
	d.mu.Unlock()
	if !d.gmKept.flush(3 * time.Second) {
		t.Fatal("the token isn't kept")
	}
	// The file an earlier version wrote for a GM started by hand, with that token in it.
	if err := os.WriteFile(gmTokenPath(), []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mac.recordLook = "unreadable"
	held := restart(t)
	held.gmKept.flush(3 * time.Second)
	if _, err := os.Stat(gmTokenPath()); err == nil {
		t.Error("a start while the key can't be read left the token file")
	}
	mac.recordLook = ""
	again := restart(t)
	if !worksAsGM(again, token) {
		t.Errorf("after the unlocked restart %q, the GM's token from before doesn't work", recordsState())
	}
}

// RT126SK1R4 follow-up 3 on SK2's login: while the records' key can't be read, the owner's Allow for a GM started by
// hand (smolcorp gm login) gives no token, with Setup's reason, as a rotation in Setup is refused; the GM's token from
// before works again after the unlocked restart.
func TestOnAMacAGMLoginWhileTheKeyIsUnreadableIsRefusedWithSetupsReason(t *testing.T) {
	d, mac, _, _ := macDeck(t)
	withHiredGM(t, "working")
	token := d.newGMToken()
	if !d.gmKept.flush(3 * time.Second) {
		t.Fatal("the token isn't kept")
	}
	mac.recordLook = "unreadable"
	held := restart(t)
	shortAskWait(t)
	// Switched on, it lasts until the restart (heldSwitches): a terminal may ask then.
	if code, _, _ := fromPage(t, held, http.MethodPost, "/setup/gm/by-hand?enabled=1", nil); code != http.StatusOK {
		t.Fatalf("by hand on while the key can't be read: %d", code)
	}
	code, asked := askFromTerminal(t, held, askGM, "/work/gm")
	if code != http.StatusOK {
		t.Fatalf("the GM's ask: %d", code)
	}
	shown := shownAsks(t, held)
	if len(shown) != 1 {
		t.Fatalf("shown %v", shown)
	}
	held.mu.Lock()
	window := held.access.NewSetupSession()
	held.mu.Unlock()
	w := answerAsk(t, held, window, shown[0]["id"].(string), asked["code"].(string), "allow")
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), recordsHeldAnswer) {
		t.Errorf("the Allow while the key can't be read: %d %q", w.Code, w.Body)
	}
	if given := tokenNow(t, held); given != "" && worksAsGM(held, given) && given != token {
		t.Error("while the key can't be read, a new GM token was made for the login")
	}
	mac.recordLook = ""
	again := restart(t)
	if !worksAsGM(again, token) {
		t.Errorf("after the unlocked restart %q, the GM's token from before doesn't work", recordsState())
	}
}
