package desktop

import (
	"errors"
	"net/http"
	"os"
	"strings"
)

// The check-only key (SANDBOX-PLAN §3.1, SK2). The setup key lives in the core's memory alone, so a terminal has no
// key to setup's routes in any file. What `smolcorp setup` and the Mac's --setup still need is Start here's
// checklist: the selftest-key file, new at every start and readable by you alone, opens GET /setup/selftest and
// nothing else. Anything that reads it, a hire among them, learns only what the checklist says. Fixing what the
// checklist finds is Settings' (the page's), or a terminal you allow there (terminalask.go).

// newSelftestKey is a new check-only key, written to its file for the terminal's tools.
func newSelftestKey() string {
	key := randomKey()
	_ = writePrivate(selftestKeyFile, []byte(key+"\n"))
	return key
}

// IsSelftestKey is whether candidate is the check-only key.
func (a *Access) IsSelftestKey(candidate string) bool {
	return candidate != "" && a.SelftestKey != "" && constantTimeEqual(candidate, a.SelftestKey)
}

// isSelftestOnly is whether the request is GET /setup/selftest with the check-only key: the one thing it opens.
func (d *Deck) isSelftestOnly(r *http.Request) bool {
	if r.Method != http.MethodGet || r.URL.Path != "/setup/selftest" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.access.IsSelftestKey(r.Header.Get(selftestKeyHeader))
}

// readSelftestKey is the check-only key from its file, for `smolcorp setup`.
func readSelftestKey() (string, error) {
	data, err := os.ReadFile(selftestKeyFile)
	key := strings.TrimSpace(string(data))
	if err != nil || key == "" {
		return "", errors.New("no check key: is this the computer Smolcorp runs on, as you, and is Smolcorp up to date?")
	}
	return key, nil
}
