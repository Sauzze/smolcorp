package desktop

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

// The GM is a managed hire, so hiring one from setup turns the manager on, as
// the Mac does (DeckRouter.swift): the Go setup page has no switch of its own.

func pageCall(t *testing.T, d *Deck, method, target string) (int, string) {
	t.Helper()
	w := serve(t, d, call{method: method, target: target, host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}})
	return w.Code, strings.TrimSpace(w.Body.String())
}

func TestHiringTheGMTurnsTheManagerOnAndTheGMRoutesAnswer(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	if code, text := pageCall(t, d, "GET", "/setup/gm/team"); code != http.StatusConflict || text != managerOffAnswer {
		t.Fatalf("before the hire: %d %q, want 409 %q", code, text, managerOffAnswer)
	}
	if code, text := pageCall(t, d, "POST", "/setup/hire/gm"); code != http.StatusNoContent || len(h.windows) != 1 {
		t.Fatalf("the GM hire: %d %q, %d windows", code, text, len(h.windows))
	}
	if !loadConfig().ManagerEnabled {
		t.Error("the hire did not keep the manager on")
	}
	if code, text := pageCall(t, d, "GET", "/setup/gm/team"); code == http.StatusConflict && text == managerOffAnswer {
		t.Errorf("after the hire the GM routes still say %q", text)
	}
}

func TestARefusedGMHireLeavesTheManagerOffAndSavesNothing(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	writeRegistryFile(registryPath(), []byte(`{"version":1,"hires":[{"id":"bbb222","name":"GM","folder":"/w","role":"gm","state":"working"}]}`), 0o600)
	os.Remove(configFile)
	w := serve(t, d, call{method: "POST", target: "/setup/hire/gm", host: "localhost:7710", from: thisComputer,
		withoutSetupKey: true, headers: map[string]string{setupHeader: "1", gmTokenHeader: d.newGMToken()}})
	if w.Code != http.StatusConflict || len(h.windows) != 0 {
		t.Fatalf("a second GM: %d %q, %d windows, want 409 and none", w.Code, w.Body, len(h.windows))
	}
	if d.config.ManagerEnabled {
		t.Error("a refused hire turned the manager on")
	}
	if _, err := os.Stat(configFile); err == nil {
		t.Error("a refused hire saved the config")
	}
}

func TestAGMHireThatFailsPutsTheOldSwitchBack(t *testing.T) {
	for _, before := range []bool{false, true} {
		d, h, _ := hiringDeck(t, "")
		d.config.ManagerEnabled = before
		h.missing = "claude isn't installed here."
		if code, _ := pageCall(t, d, "POST", "/setup/hire/gm"); code != http.StatusConflict {
			t.Fatalf("a hire whose agent is missing: %d, want 409", code)
		}
		if d.config.ManagerEnabled != before || loadConfig().ManagerEnabled != before {
			t.Errorf("after a failed hire the switch is %v (saved %v), want %v", d.config.ManagerEnabled, loadConfig().ManagerEnabled, before)
		}
	}
}

func TestHiringAnotherRoleLeavesTheManagerAlone(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	if status, text := d.hireFromSetup("reviewer"); status >= http.StatusBadRequest {
		t.Fatalf("hiring a reviewer: %d %q", status, text)
	}
	if len(h.windows) != 1 || d.config.ManagerEnabled || loadConfig().ManagerEnabled {
		t.Errorf("a reviewer hire: %d windows, manager %v / saved %v, want one window and off", len(h.windows), d.config.ManagerEnabled, loadConfig().ManagerEnabled)
	}
}
