package desktop

import (
	"bufio"
	"encoding/json"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// A floor's feed (FL1), read as the home Mac reads it: over the network, from the stand-in tailnet of newTwoMacs,
// with the peer key. No real Tailscale, no window, no agent.

// peerFeed is an open GET /peer/events: each event's data as it comes ("ping" for a ping), closed when the stream ends.
type peerFeed struct {
	events chan string
	answer *http.Response
}

// openFeed opens the feed on the home Mac with key, and waits for its answer.
func openFeed(t *testing.T, m *twoMacs, key string) *peerFeed {
	t.Helper()
	r, _ := http.NewRequest("GET", "http://"+m.address+"/peer/events", nil)
	r.Header.Set(peerKeyHeader, key)
	// peerClient's transport, the stand-in tailnet's dial, without its ten-second limit on the whole answer.
	answer, err := (&http.Client{Transport: peerClient.Transport}).Do(r)
	if err != nil {
		t.Fatalf("opening the feed: %v", err)
	}
	t.Cleanup(func() { answer.Body.Close() })
	feed := &peerFeed{events: make(chan string, 64), answer: answer}
	go func() {
		defer close(feed.events)
		lines := bufio.NewScanner(answer.Body)
		lines.Buffer(make([]byte, 1<<20), 1<<20)
		event := ""
		for lines.Scan() {
			line := lines.Text()
			switch {
			case line == "event: ping":
				event = "ping"
			case strings.HasPrefix(line, "data: ") && event == "":
				event = strings.TrimPrefix(line, "data: ")
			case line == "" && event != "":
				feed.events <- event
				event = ""
			}
		}
	}()
	return feed
}

// next is the feed's next event that isn't a ping; it fails the test if none comes, or the stream ends.
func (f *peerFeed) next(t *testing.T) map[string]any {
	t.Helper()
	for {
		select {
		case event, open := <-f.events:
			if !open {
				t.Fatal("the feed ended")
			}
			if event == "ping" {
				continue
			}
			var message map[string]any
			if err := json.Unmarshal([]byte(event), &message); err != nil {
				t.Fatalf("a message that isn't JSON: %q", event)
			}
			return message
		case <-time.After(5 * time.Second):
			t.Fatal("no message came")
		}
	}
}

// ends says whether the feed ends within the time given, pings and messages read past.
func (f *peerFeed) ends(within time.Duration) bool {
	deadline := time.After(within)
	for {
		select {
		case _, open := <-f.events:
			if !open {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// peerStreamsOpen is how many feeds the deck holds open.
func peerStreamsOpen(d *Deck) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.peerStreams)
}

// seatSecrets puts a session at the home's first desk, with every field a phone is told and none the feed may carry
// set to text that says so.
func seatSecrets(d *Deck) {
	d.launcherConnected("L1", Claude, "/Users/someone/app", "", "")
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "desk-1", "cwd": "/Users/someone/app"},
		Claude, "launcher:L1", "/Users/someone/app", 1)
	s := d.sessions.Get("desk-1")
	s.State, s.Effort, s.Tool, s.Model = Thinking, "high", "Bash", "claude-model"
	s.Messaged, s.MessagedAt, s.MaxPressedAt = "secret-message", 5, 6
	s.Transcript = "/Users/someone/.claude/secret-transcript.jsonl"
	s.Subagents = []Subagent{{ID: "sub-1", Type: "Explore", Model: "claude-submodel"}}
	d.changed()
}

// The feed is the phone's state cut down to a short list, never more: the version, the clock, and each desk
// downstairs with its folder's name and, of its session, peerDeskFields alone, each value the phone's own.
func TestAPeersFeedIsNoMoreThanAPhoneSees(t *testing.T) {
	turnOn(t, &peersOn)
	d := newTestDeck(t)
	d.config.PhoneMicEnabled, d.config.MusicEnabled = true, true
	seatSecrets(d)
	d.mu.Lock()
	phoneState := d.snapshot()
	d.mu.Unlock()
	var phone struct {
		Keys []struct {
			Pin     any            `json:"pin"`
			Session map[string]any `json:"session"`
		} `json:"keys"`
		Now float64 `json:"now"`
	}
	var feed struct {
		Version float64          `json:"version"`
		Now     float64          `json:"now"`
		Keys    []map[string]any `json:"keys"`
	}
	data := peerFeedOf(phoneState)
	var whole map[string]any
	if json.Unmarshal(phoneState, &phone) != nil || json.Unmarshal(data, &feed) != nil || json.Unmarshal(data, &whole) != nil {
		t.Fatalf("the feed %s from %s", data, phoneState)
	}
	if got := slices.Sorted(maps.Keys(whole)); !slices.Equal(got, []string{"keys", "now", "version"}) {
		t.Errorf("the feed says %v; want keys, now and version alone", got)
	}
	if feed.Version != 1 || feed.Now != phone.Now || len(feed.Keys) != len(phone.Keys) || len(feed.Keys) == 0 {
		t.Fatalf("version %v, now %v (the phone's %v), %d desks (the phone's %d)", feed.Version, feed.Now, phone.Now, len(feed.Keys), len(phone.Keys))
	}
	seated := 0
	for i, desk := range feed.Keys {
		if got := slices.Sorted(maps.Keys(desk)); !slices.Equal(got, []string{"pin", "session"}) {
			t.Errorf("desk %d says %v", i, got)
		}
		if !reflect.DeepEqual(desk["pin"], phone.Keys[i].Pin) {
			t.Errorf("desk %d's folder %v, the phone's %v", i, desk["pin"], phone.Keys[i].Pin)
		}
		session, _ := desk["session"].(map[string]any)
		if (session == nil) != (phone.Keys[i].Session == nil) {
			t.Errorf("desk %d: a session %v, the phone's %v", i, session, phone.Keys[i].Session)
		}
		if session == nil {
			continue
		}
		seated++
		for field, value := range session {
			if !slices.Contains(peerDeskFields, field) {
				t.Errorf("desk %d's session tells %s", i, field)
			}
			if mine, ok := phone.Keys[i].Session[field]; !ok || !reflect.DeepEqual(value, mine) {
				t.Errorf("desk %d's %s is %v, the phone's %v", i, field, value, mine)
			}
		}
		for _, field := range []string{"id", "name", "project", "state", "effort", "agent", "model", "subagents", "startedAt"} {
			if _, ok := session[field]; !ok {
				t.Errorf("desk %d's session doesn't tell %s", i, field)
			}
		}
	}
	if seated != 1 {
		t.Errorf("%d desks seated, want 1", seated)
	}
	// What the phone is told that no other Mac is: what was typed, the buttons, the settings, hiring, music.
	text := string(data)
	for _, never := range []string{"secret-message", "messaged", "maxPressedAt", "summary", "conversation", "phoneMic",
		"hiring", "music", "layout", "theme", "floors", "upstairs"} {
		if strings.Contains(text, never) {
			t.Errorf("the feed has %q: %s", never, text)
		}
		if !strings.Contains(string(phoneState), never) {
			t.Errorf("the phone's state hasn't %q, so the check proves nothing", never)
		}
	}
}

// A paired Mac reads the feed over Tailscale with its key: the desks at once, then each change as it's made.
func TestAPeerReadsTheFeedAndHearsEachChange(t *testing.T) {
	turnOn(t, &peersOn)
	m := newTwoMacs(t, otherMac, tailHome+":0")
	key := pairedPeer(t, m)
	feed := openFeed(t, m, key)
	if feed.answer.StatusCode != http.StatusOK || feed.answer.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("the feed: %d %s", feed.answer.StatusCode, feed.answer.Header.Get("Content-Type"))
	}
	first := feed.next(t)
	if first["version"] != float64(peerFeedVersion) {
		t.Fatalf("the first message: %v", first)
	}
	seatSecrets(m.home)
	changed := feed.next(t)
	desks, _ := changed["keys"].([]any)
	if len(desks) == 0 || changed["version"] != float64(peerFeedVersion) {
		t.Fatalf("after the change: %v", changed)
	}
	session, _ := desks[0].(map[string]any)["session"].(map[string]any)
	if session["id"] != "desk-1" || session["state"] != string(Thinking) {
		t.Errorf("the change's desk: %v", desks[0])
	}
	// It was marked seen, as a request is.
	m.home.mu.Lock()
	seen := m.home.macPeers.All[0].LastSeen
	m.home.mu.Unlock()
	if seen.IsZero() {
		t.Error("the Mac wasn't marked seen")
	}
}

// Removing a Mac, turning the setting off, or resetting the records cuts its feed there and then, not at its next
// request nor the next check (RT126M1 N2): the check here is an hour away.
func TestAPeersFeedIsCutTheMomentItsRemoved(t *testing.T) {
	turnOn(t, &peersOn)
	saved := peerStreamCheck
	t.Cleanup(func() { peerStreamCheck = saved })
	peerStreamCheck = time.Hour
	m := newTwoMacs(t, otherMac, tailHome+":0")
	key := pairedPeer(t, m)
	other := pairedPeer(t, m)
	feed, kept := openFeed(t, m, key), openFeed(t, m, other)
	feed.next(t)
	kept.next(t)
	id := m.home.macPeers.All[0].ID
	if got := peerPage(t, m.home, "POST", "/setup/peers/revoke?peer="+id, ""); got.Code != http.StatusOK {
		t.Fatalf("removing: %d %s", got.Code, got.Body)
	}
	if !feed.ends(5 * time.Second) {
		t.Fatal("the removed Mac's feed goes on")
	}
	// The other Mac's goes on.
	if kept.ends(200 * time.Millisecond) {
		t.Fatal("another Mac's feed was cut with it")
	}
	if feed := openFeed(t, m, key); feed.answer.StatusCode != http.StatusUnauthorized {
		t.Errorf("the removed Mac's feed again: %d", feed.answer.StatusCode)
	}
	// Floors turned off: every feed.
	if got := peerPage(t, m.home, "POST", "/setup/peers?enabled=0", ""); got.Code != http.StatusOK {
		t.Fatalf("turning floors off: %d", got.Code)
	}
	if !kept.ends(5 * time.Second) {
		t.Fatal("a feed goes on with floors off")
	}
	for deadline := time.Now().Add(5 * time.Second); peerStreamsOpen(m.home) != 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%d feeds still held", peerStreamsOpen(m.home))
		}
	}
}

// The records reset (only under the Mac app, which keeps their key): every Mac paired goes, and its feed with it, as
// a phone's stream does.
func TestAPeersFeedIsCutWhenTheRecordsReset(t *testing.T) {
	turnOn(t, &peersOn)
	d, _, _, _ := macDeck(t)
	d.mu.Lock()
	d.config.FloorsFromOtherMacs = true
	key := d.macPeers.pair("Laptop")
	id := d.macPeers.All[0].ID
	d.mu.Unlock()
	stream := d.attachPeer(id, key)
	if stream == nil {
		t.Fatal("no feed for the Mac paired")
	}
	defer d.detachPeer(stream)
	records.mu.Lock()
	records.stale = true
	records.mu.Unlock()
	d.mu.Lock()
	err := d.resetRecords()
	d.mu.Unlock()
	if err != nil {
		t.Fatalf("resetting the records: %v", err)
	}
	select {
	case <-stream.closed:
	default:
		t.Fatal("the feed goes on once the records are reset")
	}
}

// While it streams, the key is asked again every peerStreamCheck, whatever cut it or didn't: a Mac no longer on the
// list, its key changed, or the setting off, is hung up on at the next check; till then, each check pings it.
func TestAPeersKeyIsAskedAgainWhileItStreams(t *testing.T) {
	turnOn(t, &peersOn)
	saved := peerStreamCheck
	t.Cleanup(func() { peerStreamCheck = saved })
	peerStreamCheck = 20 * time.Millisecond
	for _, change := range []struct {
		name string
		make func(d *Deck)
	}{
		{"off the list", func(d *Deck) { d.macPeers.All = nil }},
		{"its key changed", func(d *Deck) { d.macPeers.All[0].KeyHash = hashKey("another-fake-key") }},
		{"floors off", func(d *Deck) { d.config.FloorsFromOtherMacs = false }},
	} {
		t.Run(change.name, func(t *testing.T) {
			m := newTwoMacs(t, otherMac, tailHome+":0")
			key := pairedPeer(t, m)
			feed := openFeed(t, m, key)
			feed.next(t)
			// Pinged while it may listen.
			pinged := false
			for deadline := time.After(5 * time.Second); !pinged; {
				select {
				case event, open := <-feed.events:
					if !open {
						t.Fatal("the feed ended while its key was good")
					}
					pinged = event == "ping"
				case <-deadline:
					t.Fatal("no ping came")
				}
			}
			// Changed behind hangUpPeers' back, so only the check can see it.
			m.home.mu.Lock()
			change.make(m.home)
			m.home.mu.Unlock()
			if !feed.ends(5 * time.Second) {
				t.Fatal("the feed goes on")
			}
		})
	}
}

// Removed between servePeer letting it in and the stream opening: no stream, 401.
func TestAFeedOpensOnlyForTheMacItsKeyIs(t *testing.T) {
	turnOn(t, &peersOn)
	m := newTwoMacs(t, otherMac, tailHome+":0")
	key := pairedPeer(t, m)
	id := m.home.macPeers.All[0].ID
	if stream := m.home.attachPeer("another-mac", key); stream != nil {
		t.Error("a stream for another Mac's id with this one's key")
	}
	if stream := m.home.attachPeer(id, "fake-peer-key"); stream != nil {
		t.Error("a stream with a key that isn't the Mac's")
	}
	stream := m.home.attachPeer(id, key)
	if stream == nil {
		t.Fatal("no stream for the Mac's own key")
	}
	m.home.detachPeer(stream)
	if peerStreamsOpen(m.home) != 0 {
		t.Error("a stream held once let go")
	}
	// A phone's or the window's key is no peer's: 401, no stream.
	m.home.mu.Lock()
	phoneKey, windowKey := m.home.access.Phones.Pair("Probe", true), m.home.access.WindowKey
	m.home.mu.Unlock()
	for _, other := range []string{phoneKey, windowKey} {
		if got := peerAsk(m.home, "GET", "/peer/events", other, otherMac, "", ""); got.Code != http.StatusUnauthorized {
			t.Errorf("the feed with a phone's or the window's key: %d", got.Code)
		}
	}
	if peerStreamsOpen(m.home) != 0 {
		t.Error("a stream held for a key that isn't a Mac's")
	}
}
