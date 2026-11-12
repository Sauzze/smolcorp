package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// N1: a hook's billing note never waits on the card's lock, which is held while files are read.
func TestAHooksBillingNoteNeverWaitsOnTheCardsReads(t *testing.T) {
	s := &sessionInfos{followers: map[string]*transcriptFollower{}, profiles: map[string]keptProfile{}, billing: map[string]string{}}
	s.mu.Lock() // as report() holds it through a slow read
	done := make(chan struct{})
	go func() {
		s.noteBilling(billingAPIKey, "s1")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the hook waited on the card's lock")
	}
	s.mu.Unlock()
	s.billingMu.Lock()
	got := s.billing["s1"]
	s.billingMu.Unlock()
	if got != billingAPIKey {
		t.Fatalf("%q", got)
	}
}

// N3: a line with no newline that is longer than the line limit ends the page at the reach, and the next page goes on.
func TestTheReachIsKeptInsideOneOverLongLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	data := convUser("first") + strings.Repeat("x", int(conversationReach)+3*conversationLineMost)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	page := convPage(t, path, nil, conversationPageMost)
	if len(page.Items) != 0 || page.Before == nil || *page.Before <= 0 || *page.Before >= page.Size {
		t.Fatalf("%+v", page)
	}
	if *page.Before < page.Size-conversationReach-conversationLineMost-conversationChunk {
		t.Fatalf("went on past the reach: %d of %d", *page.Before, page.Size)
	}
}

// N4: the session's own transcript is never the one let go, however many subagents it has.
func TestTheSessionsOwnTranscriptIsNotLetGoForItsSubagents(t *testing.T) {
	s := &sessionInfos{followers: map[string]*transcriptFollower{}, profiles: map[string]keptProfile{}, billing: map[string]string{}}
	base := time.Now()
	s.followers["own"] = &transcriptFollower{path: "own", lastUsed: base.Add(-time.Hour)}
	for i := 0; i < followersMost+5; i++ {
		path := filepath.Join("sub", string(rune('a'+i%26)), strings.Repeat("n", i))
		s.followers[path] = &transcriptFollower{path: path, lastUsed: base.Add(time.Duration(i) * time.Second)}
	}
	s.trim("own")
	if s.followers["own"] == nil || len(s.followers) != followersMost {
		t.Fatalf("%v %d", s.followers["own"], len(s.followers))
	}
}

// N5: on Windows a transcript path in other capitals still finds the profile beside ~/.claude.
func TestTheProfilePathAgreesWithTheHomeFolderInAnyCapitalsOnWindows(t *testing.T) {
	home := filepath.Join(string(filepath.Separator)+"Users", "me")
	folder := filepath.Join(string(filepath.Separator)+"USERS", "ME", ".claude")
	want := filepath.Join(folder, ".claude.json")
	if isWindows := os.PathSeparator == '\\'; isWindows {
		want = filepath.Join(home, ".claude.json")
	}
	if got := claudeProfilePath(folder, home); got != want {
		t.Fatalf("%s want %s", got, want)
	}
}
