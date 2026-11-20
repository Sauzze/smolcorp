package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXGMNICK on Windows: a folder typed with another drive letter's case and forward slashes is the session's own
// folder, as Windows sees it, and `gm team` shows the nickname set through it.
func TestOnWindowsANicknameSetThroughAnotherSpellingOfTheFolderShowsInGMTeam(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, _ := withGMAndAHire(t, d)
	project := filepath.Join(t.TempDir(), "shop")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionAt(d, "s1", project, "launcher:L-s1")
	typed := strings.ToLower(project[:1]) + filepath.ToSlash(project[1:])
	if typed == project {
		t.Fatalf("the typed folder %q should be spelled another way", typed)
	}
	body, _ := json.Marshal(map[string]string{"folder": typed, "name": "Ada"})
	if status, text := gmPost(t, d, gm, "/setup/gm/nickname", string(body)); status != http.StatusOK {
		t.Fatalf("%s: %d %q", typed, status, text)
	}
	if got := gmTeamNicknames(t, d, gm)["live:s1"]; got != "Ada" {
		t.Errorf("the owner's session in %s: nickname %q, want Ada", project, got)
	}
}
