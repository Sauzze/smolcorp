//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A fifo where a desk's file should be never holds the core: it is refused at once, not waited on.
func TestTheReaderNeverWaitsOnAFifo(t *testing.T) {
	place := t.TempDir()
	fifo := filepath.Join(place, "STATUS.md")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := readHireFile(place, fifo, 1<<20)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a fifo was read")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the reader waited on a fifo")
	}
	if line := statusLine(place); line != "" {
		t.Errorf("the desk's line from a fifo: %q", line)
	}
}

// The inbox socket a Claude Code session file names is written to only when the process listening on it is the
// desk's own (mail_inbox.go): a session file naming another program's socket gets nothing, not a byte.
func TestTheInboxIsWrittenOnlyWhenItsListenerIsTheDesksOwnProcess(t *testing.T) {
	path, got := inboxSocket(t)
	if why := mailInboxPost(path, []byte("one\n"), os.Getpid()+1); why != "that socket isn't the desk's own" {
		t.Errorf("another process's socket: %q", why)
	}
	if line := <-got; line != "" {
		t.Errorf("the other process's socket read %q", line)
	}

	// The whole way: the hook's pid isn't the listener's, so nothing goes, and a hire in a window of its own can't be
	// mailed (one on its own tmux would be typed into instead).
	path, got = inboxSocket(t)
	d, h := inboxHire(t, true, "2.1.309", path)
	d.sessions.Get("s-rex").PID = os.Getpid() + 1
	result := d.mailTypeIntoHire(h, "hello", func() bool { t.Error("counted a try"); return true })
	if result.Outcome != mailCannot || result.Typed {
		t.Errorf("result %+v", result)
	}
	select {
	case line := <-got:
		if line != "" {
			t.Errorf("the socket read %q", line)
		}
	case <-time.After(200 * time.Millisecond):
	}

	// Which pid is the desk's: its pane's, on its own tmux server (whatever its hook said); else its hook's.
	pane := 4321
	if pid := mailDeskPID(paneTmux(&pane), 7); pid != pane {
		t.Errorf("on its own tmux: %d, want the pane's %d", pid, pane)
	}
	pane = 0
	if pid := mailDeskPID(paneTmux(&pane), 7); pid != 0 {
		t.Errorf("a pane that can't be told: %d, want none", pid)
	}
	if pid := mailDeskPID(nil, 7); pid != 7 {
		t.Errorf("in a window of its own: %d, want the hook's 7", pid)
	}
}

// A session file that is a link (to a stand-in session naming a socket) isn't read: the session's inbox isn't there.
func TestAnInboxSessionFileThatIsALinkIsNotRead(t *testing.T) {
	path, got := inboxSocket(t)
	d, h := inboxHire(t, true, "2.1.309", path)
	file := filepath.Join(homeDir, ".claude", "sessions", "4242.json")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	standIn := filepath.Join(t.TempDir(), "4242.json")
	writeFile(t, standIn, string(data))
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(standIn, file); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(file) })
	result := d.mailTypeIntoHire(h, "hello", func() bool { t.Error("counted a try"); return true })
	if result.Outcome != mailCannot {
		t.Errorf("result %+v", result)
	}
	select {
	case line := <-got:
		if line != "" {
			t.Errorf("the socket read %q", line)
		}
	case <-time.After(200 * time.Millisecond):
	}
}

// Checkpoints keep links as links (§A3.5.4): a link in a desk's folder to a stand-in key, a file or a folder, is
// recorded as the link it is (mode 120000, its target's path as text), with git's links on or off, and the key's
// words are in no object of the repo's.
func TestACheckpointKeepsALinkToAStandInKeyAsALink(t *testing.T) {
	for _, symlinks := range []string{"true", "false"} {
		t.Run("core.symlinks="+symlinks, func(t *testing.T) {
			cpScratch(t)
			dir := aGitRepo(t)
			gitRepoCmd(t, dir, "config", "core.symlinks", symlinks)
			key := standInKeyFile(t, "key.txt")
			if err := os.Symlink(key, filepath.Join(dir, "notes.txt")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Dir(key), filepath.Join(dir, "docs")); err != nil {
				t.Fatal(err)
			}
			cp := newSnap(t, newCheckpoints(), cpDesk(dir), "turn")
			tree := gitRepoCmd(t, dir, "ls-tree", "-r", cp.ID)
			for name, target := range map[string]string{"notes.txt": key, "docs": filepath.Dir(key)} {
				if !strings.Contains(tree, "120000 blob ") || !strings.Contains(tree, "\t"+name) {
					t.Errorf("%s isn't kept as a link:\n%s", name, tree)
				}
				if text := gitRepoCmd(t, dir, "show", cp.ID+":"+name); text != target {
					t.Errorf("%s holds %q, want the link's own text %q", name, text, target)
				}
			}
			if strings.Contains(tree, "docs/") {
				t.Errorf("the linked folder was followed:\n%s", tree)
			}
			if all := gitRepoCmd(t, dir, "cat-file", "--batch-all-objects", "--batch"); strings.Contains(all, standInKey) {
				t.Error("the stand-in key's words are in the repo's objects")
			}
		})
	}
}
