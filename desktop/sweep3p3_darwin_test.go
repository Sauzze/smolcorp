package desktop

import (
	"testing"
	"time"
)

// FXSWEEP3P3 F-3, the launcher's side: its relay sends with the launcher's own key once the app has it (its
// connection said so), never the owner's; before then, an event waits a little and then gets no key at all.
func TestAPlainHiresRelaySendsWithItsLaunchersOwnKeyOnly(t *testing.T) {
	s := &launcherSandbox{hookKey: "fake-relay-key", connected: make(chan struct{})}
	if s.ownHookKey() != "fake-relay-key" {
		t.Errorf("its connection gives %q", s.ownHookKey())
	}
	started := time.Now()
	if key, err := s.relayKey(); key != "" || err == nil || time.Since(started) < relayKeyWait {
		t.Errorf("before the app has it: %q %v, after %v", key, err, time.Since(started))
	}
	s.noteConnected()
	s.noteConnected() // a reconnection says it again
	if key, err := s.relayKey(); key != "fake-relay-key" || err != nil {
		t.Errorf("once the app has it: %q %v", key, err)
	}
	if relayKeyWait >= hookTimeout {
		t.Errorf("an event waits %v for the key, as long as the hook waits for the relay (%v)", relayKeyWait, hookTimeout)
	}
	var none *launcherSandbox
	if none.ownHookKey() != "" {
		t.Error("a launcher with no sandbox gives a key")
	}
	none.noteConnected()
}
