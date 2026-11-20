package desktop

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What this version leaves off (dormant.go): spend asks, the night shift, company templates, floors from other Macs
// and photos. Each is off as it ships, every way in is unknown or refused, and nothing an owner reads names it. Each
// feature's own rows turn its switch on (turnOn) and run as before, so the code keeps working for a later version.

// turnOn turns features this version leaves off on for one test, and off again after it.
func turnOn(t *testing.T, switches ...*bool) {
	t.Helper()
	for _, s := range switches {
		was := *s
		*s = true
		t.Cleanup(func() { *s = was })
	}
}

func TestTheDormantFeaturesAreOffInThisVersion(t *testing.T) {
	for name, on := range map[string]bool{"spend asks": spendAsksOn, "the night shift": nightShiftOn,
		"company templates": templatesOn, "floors from other Macs": peersOn, "photos": photoOn} {
		if on {
			t.Errorf("%s is on", name)
		}
	}
}

// Every setup route of a feature that's off is unknown to each caller the route table lets in, says so in plain
// words, and makes nothing. A caller the table keeps out is refused as before.
func TestADormantFeaturesSetupRoutesAreUnknown(t *testing.T) {
	m := newMailDeck(t)
	for _, c := range []struct {
		as           who
		method, path string
	}{
		{asPage, "GET", nightListPath}, {asGM, "GET", nightListPath}, {asTool, "GET", nightListPath},
		{asGM, "POST", nightProposePath}, {asPage, "POST", nightProposePath}, {asPage, "POST", nightSavePath},
		{asPage, "POST", nightApprovePath + "?id=x"}, {asPage, "POST", nightPausePath + "?id=x"},
		{asPage, "POST", nightDeletePath + "?id=x"},
		{asPage, "POST", "/setup/hire/plan"}, {asGM, "POST", "/setup/hire/plan/propose"},
		{asPage, "POST", "/setup/hire/plan/approve"}, {asPage, "POST", "/setup/hire/plan/cancel"},
		{asPage, "GET", "/setup/hire/queue"}, {asGM, "GET", "/setup/hire/queue"}, {asTool, "GET", "/setup/hire/queue"},
		{asPage, "POST", "/setup/hire/queue/resume"}, {asPage, "POST", "/setup/hire/queue/cancel"},
		{asPage, "GET", "/setup/peers"}, {asPage, "POST", "/setup/peers?enabled=1"}, {asPage, "POST", "/setup/peers/code"},
		{asPage, "POST", "/setup/peers/revoke?peer=x"}, {asPage, "POST", "/setup/peers/join"},
		{asPage, "POST", "/setup/grants/answer?id=x&answer=yes"},
	} {
		got := m.call(t, c.as, c.method, c.path, `{"template":"solo-builder","name":"n","kind":"hire"}`)
		if got.Code != http.StatusNotFound || strings.TrimSpace(got.Body.String()) != notInThisVersion {
			t.Errorf("%d %s %s: %d %s", c.as, c.method, c.path, got.Code, got.Body)
		}
	}
	if got := m.call(t, asRev, "POST", nightSavePath, "{}"); got.Code != http.StatusForbidden {
		t.Errorf("a hire on the night shift's save: %d %s", got.Code, got.Body)
	}
	for _, name := range []string{nightFileName, "hire-queue.json"} {
		if _, err := os.Lstat(filepath.Join(supportDir, name)); err == nil {
			t.Errorf("%s was made", name)
		}
	}
	m.d.mu.Lock()
	floors := m.d.config.FloorsFromOtherMacs
	m.d.mu.Unlock()
	if floors {
		t.Error("floors from other Macs were switched on")
	}
}

// A photo from the window or a phone, and an answer to an ask, are unknown: before the refusals that would name them.
// The window's card says no photo, with no words.
func TestADormantFeaturesPhoneRoutesAreUnknown(t *testing.T) {
	saved := tailscaleAddresses
	tailscaleAddresses = func() []string { return []string{"100.64.1.2"} }
	t.Cleanup(func() { tailscaleAddresses = saved })
	d, f := windowDeck(t)
	d.mu.Lock()
	d.config.PhoneMicEnabled = true
	window := d.access.WindowKey
	d.mu.Unlock()
	phone := pair(t, d)
	ask := func(method, path, key, host, from string, body []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set(hookKeyHeader, key)
		r.Header.Set("Content-Type", "image/png")
		r.Host, r.RemoteAddr = host, from
		if host != "localhost:7710" {
			r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, addrOf(host)))
		}
		w := httptest.NewRecorder()
		(&router{deck: d}).ServeHTTP(w, r)
		return w
	}
	picture := testPNG(t)
	for _, c := range []struct{ method, path, key, host, from string }{
		{"POST", "/talk/photo?session=s1", window, "localhost:7710", thisComputer},
		{"POST", "/photo?session=s1", phone, "100.64.1.2:7710", "100.101.102.103:50000"},
		{"POST", "/talk/photo?session=s1", phone, "100.64.1.2:7710", "100.101.102.103:50000"},
		{"POST", "/grants/answer?id=x&answer=yes", phone, "100.64.1.2:7710", "100.101.102.103:50000"},
		{"POST", "/grants/answer?id=x&answer=yes", window, "localhost:7710", thisComputer},
	} {
		if got := ask(c.method, c.path, c.key, c.host, c.from, picture); got.Code != http.StatusNotFound ||
			strings.TrimSpace(got.Body.String()) != notInThisVersion {
			t.Errorf("%s %s from %s: %d %s", c.method, c.path, c.from, got.Code, got.Body)
		}
	}
	if got := offBy(f); len(got) != 0 || len(photosOf(t, d, "s1")) != 0 {
		t.Errorf("typed %q, kept %v", got, photosOf(t, d, "s1"))
	}
	card := talkCardOf(t, d, "s1")
	if photo, _ := card["photo"].(map[string]any); photo["ok"] != false || photo["why"] != "" || photo["note"] != nil {
		t.Errorf("the card's photo: %v", card["photo"])
	}
	turnOn(t, &photoOn)
	if photo, _ := talkCardOf(t, d, "s1")["photo"].(map[string]any); photo["ok"] != true {
		t.Errorf("the card's photo with photos on: %v", photo)
	}
}

// Another Mac's routes, with a key or without, and a peer's key sent anywhere else, are unknown, whatever
// config.json says.
func TestADormantFeaturesPeerRoutesAreUnknown(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	d.config.FloorsFromOtherMacs = true
	d.mu.Unlock()
	for _, c := range []struct{ method, target, key string }{
		{"GET", "/peer/hello", "k"}, {"GET", "/peer/hello", ""}, {"GET", "/peer/events", "k"},
		{"POST", peerPairPath, ""}, {"GET", "/peer/nowhere", ""}, {"GET", "/setup/state", "k"},
	} {
		got := peerAsk(d, c.method, c.target, c.key, otherMac, "", `{"code":"123456"}`)
		if got.Code != http.StatusNotFound || strings.TrimSpace(got.Body.String()) != notInThisVersion {
			t.Errorf("%s %s with key %q: %d %s", c.method, c.target, c.key, got.Code, got.Body)
		}
	}
}

// A spend line is refused as the hook refuses it, for every actor the guard holds, and no ask is kept or listed.
func TestASpendLineIsRefusedAndNeverAsks(t *testing.T) {
	d := newTestDeck(t)
	rules, line := guardFile(t), "stripe --live refunds create --charge ch_1"
	for _, a := range []actor{actorHire, actorGM, mcpActor("k"), nightActor("j")} {
		if v := d.decide(a, action{shell: &shellAsk{line: line, rules: rules}}); v.kind != verdictRefuse || !strings.Contains(v.why, "(spend: ") ||
			strings.Contains(v.why, askOwnerAnswer) {
			t.Errorf("%s: %+v", a, v)
		}
	}
	if waiting := d.grants.waiting(); len(waiting) != 0 {
		t.Errorf("asks kept: %v", waiting)
	}
	if asks, ok := d.approvalsWithAsks(nowMillis())["asks"].([]map[string]any); !ok || len(asks) != 0 {
		t.Errorf("asks listed: %v", asks)
	}
	turnOn(t, &spendAsksOn)
	if v := decideShell(actorHire, shellAsk{line: line, rules: rules}); v.kind != verdictAskOwner {
		t.Errorf("with spend asks on: %+v", v)
	}
}

// No timer of a feature that's off starts with the deck: the night shift's scheduler stays unbegun.
func TestNoDormantFeaturesTimerStarts(t *testing.T) {
	isolate(t)
	nightShiftStarts = true
	d := startDeck(t)
	d.night.sched.mu.Lock()
	started := d.night.sched.due != nil
	d.night.sched.mu.Unlock()
	if started {
		t.Error("the night shift's scheduler started")
	}
}

// The update names no Mac paired for floors while those are off.
func TestTheUpdateNoteNamesNoPairedMac(t *testing.T) {
	note := updateNote{Phones: 1, Macs: 2}
	if words := updateWords(note, false, nil); strings.Contains(words, "Mac") {
		t.Errorf("the update says: %s", words)
	}
	turnOn(t, &peersOn)
	if words := updateWords(note, false, nil); !strings.Contains(words, "The 2 Macs paired") {
		t.Errorf("with floors on, the update says: %s", words)
	}
}

// What was said of these features is gone from the release notes, Setup's page, the docs that named them and what
// Setup shows of the records and the update, so a later edit doesn't bring it back.
func TestNoDormantFeatureIsNamedWhereTheOwnerReads(t *testing.T) {
	removed := []string{
		// spend asks
		"their real gate is your ok", "where a request reaches it", "asks you instead", "owner's ok", "allow once",
		// the night shift ("Night-shift coffee" is the office's, and stays)
		"night-shift jobs", "night job", "night shift's",
		// company templates
		"company template", "pick your company", "hire plan", "hiring plan",
		// floors from other Macs
		"paired mac", "phone and mac", "phone or mac", "phones and macs", "floors from other", "shared floors", "a phone, a mac",
		// photos
		"send a photo", "a photo to", "pasting a picture", "photo's file",
	}
	texts := map[string]string{}
	for _, file := range []string{"RELEASE-NOTES.md", "Resources/web/setup.html", "README.md", "docs/keys-and-guard.md",
		"docs/upgrade-1.26.md", "docs/manager/HIRING-GUIDE.md", "docs/manager/API.md", "docs/manager/README.md", "docs/phone.md"} {
		data, err := os.ReadFile(filepath.Join("..", file))
		if err != nil {
			t.Fatal(err)
		}
		texts[file] = string(data)
	}
	for _, state := range []string{recordsOK, recordsChanged, recordsInterrupted, recordsUnreadable} {
		for _, stale := range []bool{false, true} {
			texts["records "+state] += recordsText(state, stale) + "\n" + recordsResetConfirm(state, stale)
		}
	}
	texts["the update"] = updateWords(updateNote{Phones: 2, Macs: 2}, true, []string{"Ada"})
	for where, text := range texts {
		lower := strings.ToLower(text)
		for _, phrase := range removed {
			if strings.Contains(lower, phrase) {
				t.Errorf("%s still says %q", where, phrase)
			}
		}
	}
}

// A hire queue's run kept from a version where templates or night jobs ran is let go at the next start, and said
// nowhere: no word to resume or cancel it, nor to pick a company again.
func TestAKeptQueueRunIsLetGoUnsaid(t *testing.T) {
	r := newQueueRig(t)
	turnOn(t, &templatesOn)
	r.answer = func(int, hireStep) (int, string) { return http.StatusConflict, "No." }
	r.q.start(steps("builder", "tester"), "Solo builder", actorPage)
	r.q.wait()
	said := r.lines()
	templatesOn = false // put back by turnOn
	run := r.build().snapshot()["run"].(hireQueueRun)
	if run.State != hqAbandoned || run.Word != "" || r.lines() != said {
		t.Errorf("run %q %q, said %q", run.State, run.Word, strings.TrimPrefix(r.lines(), said))
	}
}
