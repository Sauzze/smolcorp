//go:build !windows

package desktop

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestMailVersionAtLeastReadsDottedVersions(t *testing.T) {
	for version, want := range map[string]bool{
		"2.1.309": true, "2.1.310": true, "2.2": true, "3.0.0": true, "2.1.309.1": true,
		"2.1.308": false, "2.1": false, "1.9.999": false, "": false, "2.1.x": false, "2.1.309-beta": false,
	} {
		if got := mailVersionAtLeast(version, mailInboxSince); got != want {
			t.Errorf("%q: %v, want %v", version, got, want)
		}
	}
}

// The Mac's MailDelivery.line: sorted keys, slashes and <, > and & as written, one line feed at the end.
func TestTheInboxLineIsTheMacsJSONLine(t *testing.T) {
	got := string(mailInboxLine("look at a/b <now> & \"then\"\nreply"))
	want := `{"message":{"content":"look at a/b <now> & \"then\"\nreply","role":"user"},"type":"user"}` + "\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// inboxSocket listens on a short socket path and hands back what one connection wrote.
func inboxSocket(t *testing.T) (string, <-chan string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "ib")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "in.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	got := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		data, _ := io.ReadAll(conn)
		got <- string(data)
	}()
	return path, got
}

func TestTheInboxIsWrittenOnlyThroughAUsersOwnSocket(t *testing.T) {
	path, got := inboxSocket(t)
	if why := mailInboxPost(path, []byte("one\n")); why != "" {
		t.Fatalf("post: %s", why)
	}
	if line := <-got; line != "one\n" {
		t.Errorf("the socket read %q", line)
	}
	file := filepath.Join(t.TempDir(), "plain")
	os.WriteFile(file, nil, 0o600)
	if why := mailInboxPost(file, []byte("one\n")); why != "that isn't a socket of yours" {
		t.Errorf("a plain file: %q", why)
	}
	if why := mailInboxPost(file+"-gone", []byte("one\n")); why != "its socket isn't there" {
		t.Errorf("nothing there: %q", why)
	}
}

// A hire in a window of its own, on a deck with its one session in the hire's folder, whose session file names socket.
func inboxHire(t *testing.T, on bool, version, socket string) (*Deck, Hire) {
	t.Helper()
	d := newTestDeck(t)
	folder := t.TempDir()
	h := Hire{ID: "rex-1234", Name: "Janitor", Nickname: "Rex", Folder: folder, State: "working"}
	writeRegistry(t, h)
	d.mu.Lock()
	d.config.MailInboxSocket = on
	d.mu.Unlock()
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s-rex", "cwd": folder},
		Claude, "tty:/dev/ttys009", "", nowMillis())
	sessions := filepath.Join(homeDir, ".claude", "sessions")
	os.MkdirAll(sessions, 0o700)
	data, _ := json.Marshal(map[string]any{"pid": 4242, "sessionId": "s-rex", "name": "rex", "messagingSocketPath": socket, "version": version})
	os.WriteFile(filepath.Join(sessions, strconv.Itoa(4242)+".json"), data, 0o600)
	return d, h
}

func TestMailReachesAHireNotInTmuxThroughItsInboxWhenThatIsOn(t *testing.T) {
	path, got := inboxSocket(t)
	d, h := inboxHire(t, true, "2.1.309", path)
	commits := 0
	result := d.mailTypeIntoHire(h, "Please look at the login change.", func() bool { commits++; return true })
	if result.Outcome != mailSent || !result.Typed || commits != 1 {
		t.Fatalf("result %+v, commits %d", result, commits)
	}
	if line := <-got; line != string(mailInboxLine("Please look at the login change.")) {
		t.Errorf("the socket read %q", line)
	}
}

func TestWithoutItsInboxAHireNotInTmuxCantBeMailedAndSaysSo(t *testing.T) {
	path, _ := inboxSocket(t)
	cannot := "Rex doesn't run in tmux here and has no inbox socket to use, so mail can't reach it."
	for _, c := range []struct {
		name, version string
		on            bool
	}{{"off", "2.1.309", false}, {"too old", "2.1.308", true}} {
		t.Run(c.name, func(t *testing.T) {
			d, h := inboxHire(t, c.on, c.version, path)
			result := d.mailTypeIntoHire(h, "hello", func() bool { t.Error("counted a try"); return true })
			if result.Outcome != mailCannot || result.Note != cannot || result.Typed {
				t.Errorf("result %+v", result)
			}
		})
	}
}

func TestInboxMailWaitsWhileTheDeckKnowsTheHireIsAsking(t *testing.T) {
	path, _ := inboxSocket(t)
	d, h := inboxHire(t, true, "2.1.309", path)
	d.sessions.Apply(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s-rex", "tool_name": "Bash"},
		Claude, "tty:/dev/ttys009", "", nowMillis())
	result := d.mailTypeIntoHire(h, "hello", func() bool { t.Error("counted a try"); return true })
	if result.Outcome != mailWaiting || result.Note != "waiting: at a question" {
		t.Errorf("result %+v", result)
	}
}

// RHIRE N4: a worktree hire's inbox is its worktree's session only, never the one session left in its folder; and
// the GM's own session is never taken for a hire's, even alone in the hire's folder.
func TestTheInboxIsNeverASessionInTheHiresFolderButNotItsWorktreeNorTheGMs(t *testing.T) {
	cannot := "Rex doesn't run in tmux here and has no inbox socket to use, so mail can't reach it."
	t.Run("worktree", func(t *testing.T) {
		path, got := inboxSocket(t)
		d, h := inboxHire(t, true, "2.1.309", path)
		h.Worktree = t.TempDir()
		writeRegistry(t, h)
		if result := d.mailTypeIntoHire(h, "hello", func() bool { t.Error("counted a try"); return true }); result.Outcome != mailCannot || result.Note != cannot {
			t.Fatalf("the owner's session in the hire's folder was taken for its own: %+v", result)
		}
		d.sessions.Apply(map[string]any{"hook_event_name": "SessionEnd", "session_id": "s-rex"}, Claude, "tty:/dev/ttys009", "", nowMillis())
		d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s-rex", "cwd": h.Worktree},
			Claude, "tty:/dev/ttys010", "", nowMillis()+1)
		if result := d.mailTypeIntoHire(h, "hello", func() bool { return true }); result.Outcome != mailSent {
			t.Fatalf("its session in its worktree: %+v", result)
		}
		if line := <-got; line != string(mailInboxLine("hello")) {
			t.Errorf("the socket read %q", line)
		}
	})
	t.Run("the GM's", func(t *testing.T) {
		path, _ := inboxSocket(t)
		d, h := inboxHire(t, true, "2.1.309", path)
		writeRegistry(t, h, Hire{ID: "gm-1", Name: "General Manager", Role: gmRole, Folder: h.Folder, State: "working"})
		if result := d.mailTypeIntoHire(h, "hello", func() bool { t.Error("counted a try"); return true }); result.Outcome != mailCannot || result.Note != cannot {
			t.Errorf("the GM's own session was taken for the hire's: %+v", result)
		}
	})
}
