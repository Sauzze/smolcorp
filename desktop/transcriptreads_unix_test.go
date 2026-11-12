//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Stand-ins: a FIFO in each place a path is opened. Each must come back at once, with nothing read from it.
func fifoAt(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skip("no FIFO here:", err)
	}
}

func returnsAtOnce(t *testing.T, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s waited on a FIFO", what)
	}
}

// N2: regular files only, so a FIFO can't block anything.
func TestAFifoStandsInForATranscriptAndReturnsAtOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	fifoAt(t, path)
	returnsAtOnce(t, "the conversation", func() {
		if _, ok := readConversation(path, nil, conversationPageMost); ok {
			t.Error("a FIFO gave a page")
		}
	})
}

func TestAFifoStandsInForASubagentTheProfileAndTheSettings(t *testing.T) {
	home := t.TempDir()
	projects := filepath.Join(home, ".claude", "projects", "-x")
	transcript := filepath.Join(projects, "s.jsonl")
	fifoAt(t, transcript)
	fifoAt(t, filepath.Join(projects, "s", "subagents", "a.jsonl"))
	fifoAt(t, filepath.Join(home, ".claude.json"))
	fifoAt(t, filepath.Join(home, ".claude", "settings.json"))
	s := &sessionInfos{followers: map[string]*transcriptFollower{}, profiles: map[string]keptProfile{}, billing: map[string]string{}}
	returnsAtOnce(t, "the card", func() {
		card := s.report(infoDesk{id: "s", agent: Claude, project: "x", projectPath: home, transcript: transcript}, infoNow, home)
		if card["account"] != nil {
			t.Errorf("an account from a FIFO: %v", card["account"])
		}
	})
}
