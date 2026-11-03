package desktop

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// A launcher's screen, and the way back to the app. The launcher tees its
// agent's output into a text screen (launcherscreen.go); when the app sends
// {"look":"<n>"} down the command stream, the launcher reads the screen's
// state and POSTs it to /launcher/screen. The app asks "is this window an idle,
// empty box, right now?" with Deck.lookAtLauncher, as mail on Linux asks tmux.
// Only the state goes back, never the screen's text.

// launcherScreen is a textScreen with a lock: the copy goroutine writes the
// agent's output into it, the size follower resizes it, and the reporter reads.
type launcherScreen struct {
	mu     sync.Mutex
	screen *textScreen
}

// newLauncherScreen starts a screen the size of the console. A size nobody
// could read is not a size to trust, so that screen reads unknown for good.
func newLauncherScreen(rows, cols int, known bool) *launcherScreen {
	l := &launcherScreen{screen: newTextScreen(rows, cols)}
	l.screen.broken = !known
	return l
}

func (l *launcherScreen) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.screen.Write(p)
}

func (l *launcherScreen) Resize(rows, cols int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.screen.Resize(rows, cols)
}

// Look is the screen's state, and whether the screen is unknown. An unknown
// screen always reads PaneUnknown.
func (l *launcherScreen) Look() (PaneState, bool) {
	state, unknown, _ := l.lookPaste()
	return state, unknown
}

// lookPaste is Look and whether the agent turned bracketed paste on, read together.
// An unknown screen never says bracketed.
func (l *launcherScreen) lookPaste() (PaneState, bool, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	unknown := l.screen.Unknown()
	return l.screen.PaneState(), unknown, !unknown && l.screen.Bracketed()
}

// Bracketed is whether the agent has bracketed paste on, now.
func (l *launcherScreen) Bracketed() bool {
	_, _, bracketed := l.lookPaste()
	return bracketed
}

// screenAnswer is what the launcher posts back to a look.
type screenAnswer struct {
	Look    string `json:"look"`
	State   string `json:"state"`
	Unknown bool   `json:"unknown"`
	// Bracketed: the agent turned bracketed paste on. Mail pastes multi-line text only then.
	Bracketed bool `json:"bracketed"`
}

// lookSecretHeader carries the launcher's secret back with an answer. The
// secret is made when the launcher connects and goes down its own stream
// only: never into its environment, a log, an event, /state or an answer.
const lookSecretHeader = "X-Smolcorp-Look"

// randomHex is n random bytes as hex.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // no randomness, no safe secret
	}
	return hex.EncodeToString(b)
}

// isLauncherSecret says whether proof is the secret of the launcher with
// that id, compared in constant time. Nothing connected, or nothing sent, is no.
func (d *Deck) isLauncherSecret(id, proof string) bool {
	d.mu.Lock()
	tab, ok := d.launchers[id]
	d.mu.Unlock()
	if !ok || proof == "" || tab.screenSecret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(proof), []byte(tab.screenSecret)) == 1
}

// screenAnswerPath is where a launcher posts the answer to a look.
const screenAnswerPath = "/launcher/screen"

// postScreenAnswer reads the screen and tells the app. base is the app's
// address without a path. A failure is dropped: the app's wait times out,
// which it reads as unknown.
func postScreenAnswer(client *http.Client, base, key, proof, id, look string, screen *launcherScreen) {
	state, unknown, bracketed := screen.lookPaste()
	body, _ := json.Marshal(screenAnswer{Look: look, State: state.String(), Unknown: unknown, Bracketed: bracketed})
	request, err := http.NewRequest(http.MethodPost, base+screenAnswerPath+"?"+url.Values{"id": {id}}.Encode(), bytes.NewReader(body))
	if err != nil {
		return
	}
	request.Header.Set(hookKeyHeader, key)
	request.Header.Set(lookSecretHeader, proof)
	request.Header.Set("Content-Type", "application/json")
	if response, err := client.Do(request); err == nil {
		response.Body.Close()
	}
}

var (
	errLauncherGone = errors.New("that launcher isn't connected")
	errLookTimedOut = errors.New("the launcher didn't answer in time")
)

// lookAtLauncher asks the launcher with the id given for its window's state
// and waits for the answer. A launcher that's gone, doesn't answer (an old
// one ignores look) or can't read its screen gives PaneUnknown, never idle.
func (d *Deck) lookAtLauncher(id string, wait time.Duration) (PaneState, error) {
	state, _, err := d.lookAtLauncherPaste(id, wait)
	return state, err
}

// lookAtLauncherPaste is lookAtLauncher and whether the agent has bracketed
// paste on. It says false on any doubt.
func (d *Deck) lookAtLauncherPaste(id string, wait time.Duration) (PaneState, bool, error) {
	d.mu.Lock()
	tab, ok := d.launchers[id]
	if !ok {
		d.mu.Unlock()
		return PaneUnknown, false, errLauncherGone
	}
	if d.looks == nil {
		d.looks = map[string]chan PaneState{}
	}
	if d.lookBracketed == nil {
		d.lookBracketed = map[string]bool{}
	}
	number := randomHex(8) // not a count anyone could guess
	key := id + "/" + number
	answer := make(chan PaneState, 1)
	d.looks[key] = answer
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.looks, key)
		delete(d.lookBracketed, key)
		d.mu.Unlock()
	}()

	if err := tab.send(launcherCommand{Look: number}); err != nil {
		return PaneUnknown, false, err
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case state := <-answer:
		d.mu.Lock()
		bracketed := d.lookBracketed[key]
		d.mu.Unlock()
		return state, bracketed, nil
	case <-tab.gone:
		return PaneUnknown, false, errLauncherGone
	case <-timer.C:
		return PaneUnknown, false, errLookTimedOut
	}
}

// launcherScreenAnswered hands a launcher's answer to the look waiting for it,
// and says whether one was waiting. A wrong secret for that id is nobody's. Only a state the launcher was sure of
// counts: an unknown screen, or a name that isn't a state, is PaneUnknown.
func (d *Deck) launcherScreenAnswered(id, proof string, answer screenAnswer) bool {
	if !d.isLauncherSecret(id, proof) {
		return false
	}
	state := PaneUnknown
	if !answer.Unknown {
		for _, s := range []PaneState{PaneIdle, PaneDraft, PaneBusy, PaneMenu} {
			if s.String() == answer.State {
				state = s
			}
		}
	}
	d.mu.Lock()
	key := id + "/" + answer.Look
	waiting, ok := d.looks[key]
	if ok && d.lookBracketed != nil {
		// Only a screen that was known can say bracketed paste is on.
		d.lookBracketed[key] = answer.Bracketed && state != PaneUnknown
	}
	d.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case waiting <- state:
	default: // answered already
	}
	return true
}
