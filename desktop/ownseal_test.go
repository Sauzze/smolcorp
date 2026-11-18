//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// SANDBOX-PLAN §4 g8: Smolcorp's own programs are looked at at each desk start and each hook event, with a stat;
// the app's seal is checked at the first look and again only when one changed (a stand-in check here). A seal
// that fails stops every new desk, in Setup's words, before anything is made; an event's look checks it in the
// background. Off an app (a build of its own) there's nothing to look at.
func TestSmolcorpsOwnFilesAreCheckedAtEachStartAndEvent(t *testing.T) {
	if got := appHolding("/Applications/Smolcorp.app/Contents/Resources/bin/smolcorp"); got != "/Applications/Smolcorp.app" {
		t.Errorf("the app of its own program: %q", got)
	}
	for _, program := range []string{"/usr/local/bin/smolcorp", "/x/Contents/Resources/bin/smolcorp", "/x/Smolcorp.app/Contents/MacOS/AgentDeck"} {
		if got := appHolding(program); got != "" {
			t.Errorf("%s is in an app: %q", program, got)
		}
	}

	app := filepath.Join(t.TempDir(), "Smolcorp.app")
	bin := filepath.Join(app, "Contents", "Resources", "bin")
	writeFile(t, filepath.Join(bin, "smolcorp"), "core")
	writeFile(t, filepath.Join(bin, "tmux"), "tmux")
	var checks atomic.Int32
	var intact atomic.Bool
	intact.Store(true)
	savedFolder, savedSealed, savedFiles := ownAppFolder, ownAppSealed, ownFiles
	ownAppFolder = func() string { return app }
	ownAppSealed = func(folder, digest string) (string, bool) {
		checks.Add(1)
		if folder != app || checks.Load() > 1 && digest != "digest-1" {
			t.Errorf("checked %q against %q", folder, digest)
		}
		return "digest-1", intact.Load()
	}
	ownFiles = &ownSeal{}
	t.Cleanup(func() { ownAppFolder, ownAppSealed, ownFiles = savedFolder, savedSealed, savedFiles })

	if ownSealRefusal() != nil || ownSealRefusal() != nil || checks.Load() != 1 {
		t.Fatalf("the first looks: checked %d times", checks.Load())
	}
	writeFile(t, filepath.Join(bin, "tmux"), "another tmux")
	if ownSealRefusal() != nil || checks.Load() != 2 {
		t.Fatalf("a changed tmux, still sealed: checked %d times", checks.Load())
	}
	ownFiles.noteEvent()
	ownFiles.mu.Lock() // an event's check, if any, is done
	ownFiles.mu.Unlock()
	if checks.Load() != 2 {
		t.Fatalf("an event with nothing changed checked the seal")
	}
	// A smolcorp-spawn put in, and the seal broken: the event's look finds it, and no desk starts.
	intact.Store(false)
	writeFile(t, filepath.Join(bin, "smolcorp-spawn"), "planted")
	ownFiles.noteEvent()
	refusal := ownSealRefusal()
	if refusal == nil || refusal.Message != "Smolcorp's own files changed. Reinstall Smolcorp. No desk was started." || checks.Load() != 3 {
		t.Fatalf("a broken seal: %v, checked %d times", refusal, checks.Load())
	}
	// For good, without checking again.
	intact.Store(true)
	if ownSealRefusal() == nil || checks.Load() != 3 {
		t.Errorf("a broken seal mended itself: checked %d times", checks.Load())
	}
	r := newEngineRig(t)
	if _, err := runManagedHire(r.request("janitor"), r.o); err == nil || err.Error() != refusal.Message || len(r.tmux.asked) != 0 || len(registered(t)) != 0 {
		t.Errorf("a desk was started: %v, tmux %q", err, r.tmux.asked)
	}

	// Off an app, nothing is looked at.
	ownFiles = &ownSeal{}
	ownAppFolder = func() string { return "" }
	if ownSealRefusal() != nil || checks.Load() != 3 {
		t.Errorf("a build of its own was checked")
	}
	if _, err := os.Stat(app); err != nil {
		t.Fatal(err)
	}
}
