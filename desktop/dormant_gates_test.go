package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// FXPOLISHA (RT126FIXREVIEW-DORMANTOFF's O1): the three gates no row pinned (its mutants M10, M11 and M12), each with
// its switch on as the control.

// M10: no photo sweep starts with the deck while photos are off: a photo in a gone desk's folder stays. With them on,
// the sweep takes it.
func TestNoPhotoSweepStartsWhilePhotosAreOff(t *testing.T) {
	was := photoSweepEvery
	photoSweepEvery = 10 * time.Millisecond
	t.Cleanup(func() { photoSweepEvery = was })
	for _, on := range []bool{false, true} {
		name := "photos off"
		if on {
			name = "photos on"
		}
		t.Run(name, func(t *testing.T) {
			isolate(t)
			if on {
				turnOn(t, &photoOn)
			}
			d := startDeck(t)
			folder := strings.Repeat("ab", 16)
			photo := filepath.Join(supportDir, photosFolder, folder, photoPrefix+"1.png")
			if err := os.MkdirAll(filepath.Dir(photo), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(photo, []byte("standin"), 0o600); err != nil {
				t.Fatal(err)
			}
			d.mu.Lock()
			d.photoDesks = map[string]string{"a-gone-desk": folder}
			d.mu.Unlock()
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Lstat(photo); err != nil {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if _, err := os.Lstat(photo); (err == nil) == on {
				t.Errorf("with photos on %v, the gone desk's photo is there: %v", on, err == nil)
			}
		})
	}
}

// M11: the hire queue never takes a night job's approval as the owner's while the night shift is off, though the job
// is an approved hire's and the step is its own. With it on, it does.
func TestTheHireQueueTakesNoNightApprovalWhileTheNightShiftIsOff(t *testing.T) {
	d := newTestDeck(t)
	j := keepNightJob(t, d, NightJob{ID: "nightly", Role: "tester", Folder: realPath(t.TempDir()), Status: nightApproved, ProposedBy: "page"})
	q := d.newHireQueue()
	steps := []hireStep{j.nightStep()}
	if q.nightApproves("night:nightly", steps) {
		t.Error("a night job's approval was taken with the night shift off")
	}
	turnOn(t, &nightShiftOn)
	if !q.nightApproves("night:nightly", steps) {
		t.Error("the control: with the night shift on, the approved job's own step isn't approved")
	}
}

// M12 and O2: GET /grants's refusal, while spend asks are off, says who reads the list, truly on a Mac too, where the
// office's window reads it as well; it names no ask for a command.
func TestTheGrantsListsRefusalSaysWhoReadsItWhileSpendAsksAreOff(t *testing.T) {
	d, phoneKey := probeDeck(t)
	for caller, want := range map[string]string{
		"browser-mac-app": "Only your paired phone, over Tailscale's HTTPS address, and the office's window on your Mac read this.",
		"browser":         "Only your paired phone, over Tailscale's HTTPS address, reads this.",
	} {
		status, body := probeRoute(t, d, "GET", "/grants", phoneKey, ownerCall(caller))
		if status != http.StatusForbidden || !strings.Contains(body, want) || strings.Contains(body, "asks") {
			t.Errorf("GET /grants as %s: %d %q, want 403 and %q", caller, status, body, want)
		}
	}
}
