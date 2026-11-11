package desktop

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// A fake pty: the fixtures as the agent's output would come, in chunks, run
// through the tee the launcher uses.
func teedScreen(t *testing.T, rows, cols int, output string, chunk int) *launcherScreen {
	t.Helper()
	screen := newLauncherScreen(rows, cols, true)
	tee := io.TeeReader(strings.NewReader(output), io.MultiWriter(&cursorMode{}, screen))
	buf := make([]byte, chunk)
	for {
		if _, err := tee.Read(buf); err != nil {
			break
		}
	}
	return screen
}

func TestTheLauncherScreenReadsTheSixFixturesThroughTheTee(t *testing.T) {
	// What ConPTY and Claude Code send first, then the window.
	start := "\x1b[?9001h\x1b[?1004h\x1b(B\x1b[38:2::215:119:87m\x1b[0 q\x1b[?25l\x1b[2J\x1b[H"
	for name, want := range map[string]PaneState{
		"idle-empty": PaneIdle, "after-decline": PaneIdle, "draft": PaneDraft,
		"busy-1": PaneBusy, "permission-menu": PaneMenu, "trust-menu": PaneMenu,
	} {
		for _, chunk := range []int{1, 7, 4096} {
			screen := teedScreen(t, 60, 200, start+ptyOutput(pane(t, name)), chunk)
			if got, unknown := screen.Look(); got != want || unknown {
				t.Errorf("%s (chunks of %d): %v unknown %v, want %v", name, chunk, got, unknown, want)
			}
		}
	}
}

func TestStartUpSequencesAreSkippedAndOthersStillReadUnknown(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	for _, seq := range []string{
		"\x1b[38:2::12:34:56m", "\x1b[48:5:99m", "\x1b[4:3m", "\x1b[0 q", "\x1b[5 q", "\x1b(B", "\x1b)B",
	} {
		s := drawn(t, 60, 200, seq+idle)
		if s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("%q: unknown %v, state %v, want idle", seq, s.Unknown(), s.PaneState())
		}
	}
	// A colour in the middle of a pending wrap changes nothing about it.
	if s := drawn(t, 3, 3, "abc\x1b[38:2::1:2:3mdef"); s.Text() != "abc\ndef\n" || s.Unknown() {
		t.Errorf("colour broke the wrap: %q", s.Text())
	}
	for _, seq := range []string{
		"\x1b[4:3H",           // a colon on anything but a colour
		"\x1b[1:2A",           // same
		"\x1b(0",              // line drawing changes what bytes mean
		"\x1b(A",              // another character set
		"\x1b(\x1b[2J",        // an escape where the set should be
		"\x1b[!p",             // soft reset
		"\x1b[1 x",            // an intermediate on another final byte
		"\x1b[0  q",           // two spaces
		"\x1b[1;2 q",          // not one number
		"\x1b[0$q",            // a different intermediate
		"\x1b[38:2::1:2:3;4H", // a colon in a cursor move
		"\x1b[38:2::1:2:3 m",  // a space before m
	} {
		s := drawn(t, 60, 200, seq+idle)
		if !s.Unknown() || s.PaneState() != PaneUnknown {
			t.Errorf("%q: unknown %v, state %v, want unknown", seq, s.Unknown(), s.PaneState())
		}
	}
}

func TestTheLauncherScreenIsUnknownWithoutASizeAndKeepsStepWithTheConsole(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	unsized := newLauncherScreen(24, 80, false)
	unsized.Write([]byte(idle))
	if state, unknown := unsized.Look(); state != PaneUnknown || !unknown {
		t.Errorf("a console of no known size read %v", state)
	}

	screen := teedScreen(t, 60, 200, idle, 4096)
	screen.Resize(70, 220) // bigger: still known
	if state, unknown := screen.Look(); state != PaneIdle || unknown {
		t.Errorf("after growing: %v unknown %v", state, unknown)
	}
	if got := len(screen.screen.grid); got != 70 || len(screen.screen.grid[0]) != 220 {
		t.Errorf("screen is %dx%d, want 70x220", got, len(screen.screen.grid[0]))
	}
	screen.Resize(70, 190) // narrower: lines may rewrap, so unknown until a full erase
	if state, unknown := screen.Look(); state != PaneUnknown || !unknown {
		t.Errorf("after narrowing: %v unknown %v", state, unknown)
	}
	screen.Write([]byte("\x1b[2J" + idle))
	if state, _ := screen.Look(); state != PaneIdle {
		t.Errorf("after a full erase: %v", state)
	}
}

func TestTheLauncherScreenCanBeWrittenResizedAndReadAtOnce(t *testing.T) {
	screen := newLauncherScreen(40, 120, true)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < 200; n++ {
				switch i {
				case 0:
					screen.Write([]byte("hello\r\n"))
				case 1:
					screen.Resize(40+n%3, 120+n%5)
				default:
					screen.Look()
				}
			}
		}(i)
	}
	wg.Wait()
}

// follow hands a look to the launcher and leaves type, press and stop as they were.
func TestFollowAnswersALookAndLeavesTheOtherCommandsAlone(t *testing.T) {
	looked := make(chan string, 2)
	var typed strings.Builder
	input := &lockedWriter{w: &typed}
	stopped := make(chan struct{}, 1)
	commands := `{"secret":"s3"}` + "\n" + `{"look":"7"}` + "\n" + `{"press":{"key":"enter"}}` + "\n" + `{"stop":true}` + "\n"
	follow(strings.NewReader(commands), input, &cursorMode{}, func(proof, n string) { looked <- proof + "/" + n }, func() { stopped <- struct{}{} })
	select {
	case n := <-looked:
		if n != "s3/7" { // the secret from the first line is not a command
			t.Errorf("look %q", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the look wasn't passed on")
	}
	if typed.String() != keyBytes[Enter] {
		t.Errorf("press typed %q", typed.String())
	}
	select {
	case <-stopped:
	default:
		t.Error("stop didn't stop")
	}
	// An old app has no look; a new launcher with no answerer ignores one.
	follow(strings.NewReader(`{"look":"8"}`+"\n"), input, &cursorMode{}, nil, func() {})
}

func newLocalServer(t *testing.T, d *Deck) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(&router{deck: d})
	t.Cleanup(server.Close)
	return server
}

func TestALookRoundTripsThroughTheRoute(t *testing.T) {
	d := newTestDeck(t)
	tab := d.launcherConnected("L1", Claude, "/work/api", "", "")
	server := newLocalServer(t, d)
	for name, want := range map[string]PaneState{"idle-empty": PaneIdle, "draft": PaneDraft, "busy-1": PaneBusy, "permission-menu": PaneMenu} {
		screen := teedScreen(t, 60, 200, ptyOutput(pane(t, name)), 4096)
		done := make(chan struct{})
		go func() {
			defer close(done)
			command := <-tab.commands
			if command.Look == "" || command.Type != "" || command.Press != nil || command.Stop {
				t.Errorf("the deck sent %+v, want only a look", command)
				return
			}
			postScreenAnswer(server.Client(), server.URL, d.access.HookKey, tab.screenSecret, "L1", command.Look, screen)
		}()
		got, err := d.lookAtLauncher("L1", 5*time.Second)
		<-done
		if err != nil || got != want {
			t.Errorf("%s: %v, %v; want %v", name, got, err, want)
		}
	}
	if len(d.looks) != 0 {
		t.Errorf("%d looks left waiting", len(d.looks))
	}
}

func TestAnUnknownScreenAnswersUnknownNeverIdle(t *testing.T) {
	d := newTestDeck(t)
	tab := d.launcherConnected("L1", Claude, "/work/api", "", "")
	server := newLocalServer(t, d)
	// Idle text, but a sequence the screen can't draw.
	screen := teedScreen(t, 60, 200, "\x1bM"+ptyOutput(pane(t, "idle-empty")), 4096)
	go func() {
		command := <-tab.commands
		postScreenAnswer(server.Client(), server.URL, d.access.HookKey, tab.screenSecret, "L1", command.Look, screen)
	}()
	if got, err := d.lookAtLauncher("L1", 5*time.Second); err != nil || got != PaneUnknown {
		t.Errorf("%v, %v; want unknown", got, err)
	}
	// And an answer that says "idle" but "unknown": true is not trusted.
	waiting := make(chan PaneState, 1)
	d.mu.Lock()
	d.looks = map[string]chan PaneState{"L1/99": waiting}
	d.mu.Unlock()
	secret := d.launchers["L1"].screenSecret
	if !d.launcherScreenAnswered("L1", secret, screenAnswer{Look: "99", State: "idle", Unknown: true}) || <-waiting != PaneUnknown {
		t.Error("an answer marked unknown read as a state")
	}
	if d.launcherScreenAnswered("L1", secret, screenAnswer{Look: "98", State: "idle"}) {
		t.Error("an answer nobody waited for was taken")
	}
}

func TestALookThatIsNotAnsweredOrCantBeSentIsUnknown(t *testing.T) {
	d := newTestDeck(t)
	d.launcherConnected("L1", Claude, "/work/api", "", "")
	start := time.Now()
	if got, err := d.lookAtLauncher("L1", 50*time.Millisecond); got != PaneUnknown || err != errLookTimedOut {
		t.Errorf("no answer: %v, %v", got, err)
	}
	if time.Since(start) > 10*time.Second { // far from 50ms, so a busy computer can't fail it
		t.Error("the wait wasn't bounded")
	}
	if got, err := d.lookAtLauncher("nobody", time.Second); got != PaneUnknown || err != errLauncherGone {
		t.Errorf("no such launcher: %v, %v", got, err)
	}
	// A launcher whose line is full isn't waited for.
	full := d.launcherConnected("L2", Claude, "/work/api", "", "")
	for i := 0; i < cap(full.commands); i++ {
		full.commands <- launcherCommand{Type: "x"}
	}
	if got, err := d.lookAtLauncher("L2", time.Second); got != PaneUnknown || err == nil {
		t.Errorf("full line: %v, %v", got, err)
	}
	// It has gone while asked.
	gone := d.launcherConnected("L3", Claude, "/work/api", "", "")
	d.launcherGone("L3", gone)
	if got, err := d.lookAtLauncher("L3", time.Second); got != PaneUnknown || err != errLauncherGone {
		t.Errorf("gone: %v, %v", got, err)
	}
}

func TestTheScreenRouteIsGuardedLikeTheLauncherRoute(t *testing.T) {
	d := newTestDeck(t)
	tab := d.launcherConnected("L1", Claude, "/work/api", "", "")
	body := `{"look":"1","state":"idle","unknown":false}`
	post := func(headers map[string]string, host, from string) int {
		return serve(t, d, call{method: http.MethodPost, target: screenAnswerPath + "?id=L1", host: host, from: from, headers: headers, body: body}).Code
	}
	good := map[string]string{hookKeyHeader: d.access.HookKey, lookSecretHeader: tab.screenSecret}
	// Each of these must leave the look unanswered.
	results := map[string]int{
		"no key":        post(nil, "127.0.0.1:7710", thisComputer),
		"wrong key":     post(map[string]string{hookKeyHeader: "not-the-key", lookSecretHeader: tab.screenSecret}, "127.0.0.1:7710", thisComputer),
		"a phone":       post(good, deckOnWiFi, aPhone),
		"not addressed": post(good, "evil.example", thisComputer),
	}
	if results["no key"] != http.StatusUnauthorized || results["wrong key"] != http.StatusUnauthorized {
		t.Errorf("no key / wrong key: %d, %d", results["no key"], results["wrong key"])
	}
	if results["a phone"] != http.StatusNotFound || results["not addressed"] != http.StatusNotFound {
		t.Errorf("a phone / not addressed: %d, %d", results["a phone"], results["not addressed"])
	}
	if got := serve(t, d, call{method: http.MethodGet, target: screenAnswerPath + "?id=L1", host: "127.0.0.1:7710", from: thisComputer, headers: good}).Code; got != http.StatusNotFound {
		t.Errorf("GET: %d", got)
	}
	// With the key but nobody waiting: not found, and still no state anywhere.
	if got := post(good, "127.0.0.1:7710", thisComputer); got != http.StatusNotFound {
		t.Errorf("nobody waiting: %d", got)
	}
	if got, err := d.lookAtLauncher("L1", 50*time.Millisecond); got != PaneUnknown || err == nil {
		t.Errorf("refused answers reached a look: %v, %v", got, err)
	}
	if got := serve(t, d, call{method: http.MethodPost, target: screenAnswerPath + "?id=L1", host: "127.0.0.1:7710", from: thisComputer, headers: good, body: "not json"}).Code; got != http.StatusBadRequest {
		t.Errorf("bad body: %d", got)
	}
}

func TestTheAnswerCarriesNoScreenText(t *testing.T) {
	// The answer's type has the four fields (the last says only whether bracketed paste is on) and nothing that holds the window's words.
	var seen []string
	capture := &captureTransport{seen: &seen}
	client := &http.Client{Transport: capture}
	screen := teedScreen(t, 60, 200, ptyOutput(pane(t, "draft")), 4096)
	postScreenAnswer(client, "http://127.0.0.1:1", "fake-key", "fake-proof", "L1", "3", screen)
	if len(seen) != 1 || seen[0] != `{"look":"3","state":"draft","unknown":false,"bracketed":false}` {
		t.Errorf("the answer was %q", seen)
	}
}

type captureTransport struct{ seen *[]string }

func (c *captureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	*c.seen = append(*c.seen, string(body))
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: http.Header{}}, nil
}

// A look number the test waits on, as lookAtLauncher would have made it.
func waitOnLook(d *Deck, id, number string) chan PaneState {
	waiting := make(chan PaneState, 1)
	d.mu.Lock()
	if d.looks == nil {
		d.looks = map[string]chan PaneState{}
	}
	d.looks[id+"/"+number] = waiting
	d.mu.Unlock()
	return waiting
}

func TestAnAnswerWithoutTheLaunchersSecretIsRefusedAndTheLookStaysUnknown(t *testing.T) {
	d := newTestDeck(t)
	tab := d.launcherConnected("L1", Claude, "/work/api", "", "")
	other := d.launcherConnected("L2", Claude, "/work/web", "", "")
	if tab.screenSecret == "" || tab.screenSecret == other.screenSecret {
		t.Fatalf("secrets %q and %q", tab.screenSecret, other.screenSecret)
	}
	post := func(id, number string, headers map[string]string) int {
		headers[hookKeyHeader] = d.access.HookKey // the key every agent's hooks hold
		return serve(t, d, call{method: http.MethodPost, target: screenAnswerPath + "?id=" + id, host: "127.0.0.1:7710", from: thisComputer, headers: headers,
			body: `{"look":"` + number + `","state":"idle","unknown":false}`}).Code
	}
	// A forger guessing the look number, with the hook key but no or a wrong secret.
	forged := make(chan int, 4)
	go func() {
		for i := 0; i < 4; i++ {
			command := <-tab.commands
			forged <- post("L1", command.Look, map[string]string{})
			forged <- post("L1", command.Look, map[string]string{lookSecretHeader: "wrong"})
			forged <- post("L1", command.Look, map[string]string{lookSecretHeader: other.screenSecret}) // L2's secret on L1
			return
		}
	}()
	got, err := d.lookAtLauncher("L1", 300*time.Millisecond)
	if got != PaneUnknown || err != errLookTimedOut {
		t.Errorf("a forged idle reached the look: %v, %v", got, err)
	}
	for i := 0; i < 3; i++ {
		if code := <-forged; code != http.StatusUnauthorized {
			t.Errorf("forged answer %d got %d, want 401", i, code)
		}
	}
	// The right secret, for the right id, is taken.
	waiting := waitOnLook(d, "L1", "abc")
	if code := post("L1", "abc", map[string]string{lookSecretHeader: tab.screenSecret}); code != http.StatusNoContent || <-waiting != PaneIdle {
		t.Errorf("the launcher's own answer: %d", code)
	}
	// And A's secret does not answer a look at B.
	waiting = waitOnLook(d, "L2", "abc")
	if code := post("L2", "abc", map[string]string{lookSecretHeader: tab.screenSecret}); code != http.StatusUnauthorized || len(waiting) != 0 {
		t.Errorf("L1's secret answered L2: %d", code)
	}
}

func TestTheSecretIsOnlyOnItsOwnStream(t *testing.T) {
	d := newTestDeck(t)
	server := newLocalServer(t, d)
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/launcher?id=L1&agent=claude&folder=/work/api", nil)
	request.Header.Set(hookKeyHeader, d.access.HookKey)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var first launcherCommand
	if err := json.NewDecoder(response.Body).Decode(&first); err != nil || len(first.Secret) != 32 {
		t.Fatalf("first line %+v, %v; want a 32-character secret", first, err)
	}
	d.mu.Lock()
	want := d.launchers["L1"].screenSecret
	d.mu.Unlock()
	if first.Secret != want {
		t.Errorf("the stream sent %q, the tab holds %q", first.Secret, want)
	}
	// Nowhere else: not /state, not the events, not a look's answer body.
	if strings.Contains(string(d.snapshot()), want) {
		t.Error("the secret is in /state")
	}
	var seen []string
	postScreenAnswer(&http.Client{Transport: &captureTransport{seen: &seen}}, "http://127.0.0.1:1", "fake-key", want, "L1", "3", newLauncherScreen(24, 80, true))
	if len(seen) != 1 || strings.Contains(seen[0], want) {
		t.Errorf("the answer body carries the secret: %q", seen)
	}
}

func TestASecondLiveStreamForAnIdIsRefusedAndAReconnectAfterTheFirstEndsIsNot(t *testing.T) {
	d := newTestDeck(t)
	server := newLocalServer(t, d)
	open := func(ctx context.Context) (*http.Response, error) {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/launcher?id=L1&agent=claude&folder=/work/api", nil)
		request.Header.Set(hookKeyHeader, d.access.HookKey)
		return server.Client().Do(request)
	}
	firstCtx, hangUp := context.WithCancel(context.Background())
	first, err := open(firstCtx)
	if err != nil || first.StatusCode != http.StatusOK {
		t.Fatalf("first stream: %v", err)
	}
	var firstLine launcherCommand
	json.NewDecoder(first.Body).Decode(&firstLine)
	d.mu.Lock()
	tab := d.launchers["L1"]
	d.mu.Unlock()

	// A second one, live at the same time: refused; the tab and secret stay.
	second, err := open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Errorf("second live stream: %d, want 409", second.StatusCode)
	}
	d.mu.Lock()
	same := d.launchers["L1"] == tab && tab.screenSecret == firstLine.Secret
	d.mu.Unlock()
	if !same {
		t.Error("the second connection replaced the tab")
	}

	// The first ends; the id is free; a reconnect gets a new secret.
	hangUp()
	first.Body.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		third, err := open(context.Background())
		if err == nil && third.StatusCode == http.StatusOK {
			var line launcherCommand
			json.NewDecoder(third.Body).Decode(&line)
			third.Body.Close()
			if len(line.Secret) != 32 || line.Secret == firstLine.Secret {
				t.Errorf("reconnect secret %q, first %q", line.Secret, firstLine.Secret)
			}
			return
		}
		if err == nil {
			third.Body.Close()
		}
		if time.Now().After(deadline) {
			t.Fatal("the id never became free again")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestALookAtALauncherThatDropsReturnsAtOnce(t *testing.T) {
	d := newTestDeck(t)
	tab := d.launcherConnected("L1", Claude, "/work/api", "", "")
	go func() {
		<-tab.commands
		d.launcherGone("L1", tab)
	}()
	start := time.Now()
	got, err := d.lookAtLauncher("L1", 10*time.Second)
	if got != PaneUnknown || err != errLauncherGone || time.Since(start) > 3*time.Second {
		t.Errorf("%v, %v after %v", got, err, time.Since(start))
	}
}
