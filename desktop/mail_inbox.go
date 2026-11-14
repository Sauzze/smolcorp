//go:build !windows

package desktop

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Mail through Claude Code's inbox socket, the Mac's MailDelivery.deliver step 1: tried before tmux, and only when
// config.json's mailInboxSocket is on (off by default; no page setting, the Mac had none either), the hire's one
// live session names a socket in its session file, and that Claude Code is mailInboxSince or later. The one test of
// it was ours, not Anthropic's documentation, so trusting it is the owner's choice. The deck's own word that the
// session is at a question keeps the message waiting; a socket that refuses falls through to tmux. It is the only
// way mail reaches a hire in a window of its own (hirefire_window.go), which has no tmux to paste into.

// mailInboxSince is the Claude Code version the inbox socket was seen to work on (MailDelivery.inboxSince).
const mailInboxSince = "2.1.309"

// mailIntoInbox is one try through the inbox socket; tried is false when it wasn't used, so tmux is next.
func (d *Deck) mailIntoInbox(h Hire, text string, commit func() bool) (result mailResult, tried bool) {
	gms := []Hire{}
	for _, other := range readRegistry(registryPath(), false).hires {
		if gmHired(other) && other.ID != h.ID {
			gms = append(gms, other)
		}
	}
	d.mu.Lock()
	on, saved := d.config.MailInboxSocket, d.config.ClaudeProfiles
	ids, heard := []string{}, 0
	for _, s := range d.sessions.All() {
		if inboxSession(s, h, gms) {
			ids, heard = append(ids, s.ID), s.PID
		}
	}
	d.mu.Unlock()
	if !on || len(ids) != 1 {
		return mailResult{}, false
	}
	var dirs []string
	for _, p := range claudeAccountList(saved) {
		dirs = append(dirs, p.Dir)
	}
	socket, version := claudeInbox(ids[0], dirs)
	if socket == "" || !mailVersionAtLeast(version, mailInboxSince) {
		return mailResult{}, false
	}
	// The socket a session file names is the desk's only if the desk's own process listens on it (SA-X5).
	// Checked before the try is counted: a socket that isn't the desk's is as if there were none.
	desk := mailDeskPID(mailHireTmux(h), heard)
	if desk <= 1 {
		return mailResult{}, false
	}
	conn, why := mailInboxDial(socket, desk)
	if why != "" {
		return mailResult{}, false
	}
	defer conn.Close()
	if d.mailDeckAsking(h) {
		return mailResult{Outcome: mailWaiting, Note: "waiting: at a question"}, true
	}
	if !commit() {
		return mailResult{Outcome: mailTrouble, Note: "The outbox couldn't be written, so nothing was typed."}, true
	}
	if mailInboxWrite(conn, mailInboxLine(text)) == "" {
		return mailResult{Outcome: mailSent, Typed: true}, true
	}
	return mailResult{}, false
}

// inboxSession is whether s may be h's for its inbox (RHIRE N4): for a hire with a worktree, a session working in
// that worktree only, not anywhere in its folder, as its fire goes by; and never a session the GM works in.
func inboxSession(s *Session, h Hire, gms []Hire) bool {
	place := liveSession{Folder: s.Folder, Project: s.Project}
	if h.Worktree != "" && !works(place, Hire{Worktree: h.Worktree}) {
		return false
	}
	return works(place, h) && !slices.ContainsFunc(gms, func(gm Hire) bool { return works(place, gm) })
}

// mailDeskPID is the pid of the desk's own process, as the core knows it, 0 when it doesn't: for a desk on its own
// tmux server, the process its pane runs (the one SG2's start ticket was redeemed by, which execs the agent: its pid
// is the agent's); for a hire in a window of its own, which the sandbox never covers, the pid its hook said.
func mailDeskPID(server *hireTmux, heard int) int {
	if server == nil {
		return heard
	}
	if pid, ok := server.panePID(); ok {
		return pid
	}
	return 0
}

// claudeInbox is the inbox socket and Claude Code version the session file of session id names, in the sessions
// folder of the first of the Claude folders dirs that has one; "" without. Only the .json files are read, through
// the core's reader (hirefile.go: no link, one name, a size cap), and only those fields: never the keys beside them.
func claudeInbox(id string, dirs []string) (socket, version string) {
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "sessions", "*.json"))
		for _, file := range files {
			data, err := readHireFile(dir, file, claudeSessionMostBytes)
			if err != nil {
				continue
			}
			var peer struct {
				SessionID           string `json:"sessionId"`
				MessagingSocketPath string `json:"messagingSocketPath"`
				Version             string `json:"version"`
			}
			if json.Unmarshal(data, &peer) == nil && peer.SessionID == id && peer.MessagingSocketPath != "" {
				return peer.MessagingSocketPath, peer.Version
			}
		}
	}
	return "", ""
}

// mailVersionAtLeast says whether the dotted version is minimum or later; false for one that can't be read.
func mailVersionAtLeast(version, minimum string) bool {
	number := func(text string) ([]int, bool) {
		var parts []int
		for _, part := range strings.Split(text, ".") {
			n, err := strconv.Atoi(part)
			if err != nil || n < 0 {
				return nil, false
			}
			parts = append(parts, n)
		}
		return parts, true
	}
	have, ok := number(version)
	need, _ := number(minimum)
	if !ok {
		return false
	}
	for i := range max(len(have), len(need)) {
		a, b := 0, 0
		if i < len(have) {
			a = have[i]
		}
		if i < len(need) {
			b = need[i]
		}
		if a != b {
			return a > b
		}
	}
	return true
}

// mailInboxLine is the line an inbox socket reads: one JSON line, keys sorted, no auth line, ending in a line feed.
func mailInboxLine(text string) []byte {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text}})
	return out.Bytes()
}

// mailInboxPost writes line to the Unix socket at path: "" once all of it is written, else why not (mailInboxDial).
func mailInboxPost(path string, line []byte, desk int) string {
	conn, why := mailInboxDial(path, desk)
	if why != "" {
		return why
	}
	defer conn.Close()
	return mailInboxWrite(conn, line)
}

// mailInboxDial connects to the Unix socket at path, or says why not. Only a socket of this user's is connected to,
// and kept only when the process at its other end is desk, by the pid the kernel gives for it (LOCAL_PEERPID,
// SO_PEERCRED: the process that listened), before a byte is sent. The caller closes it.
func mailInboxDial(path string, desk int) (net.Conn, string) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "its socket isn't there"
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if info.Mode()&os.ModeSocket == 0 || !ok || int(stat.Uid) != os.Getuid() {
		return nil, "that isn't a socket of yours"
	}
	conn, err := net.DialTimeout("unix", path, 3*time.Second)
	if err != nil {
		return nil, "refused: " + err.Error()
	}
	if pid, err := peerPid(conn); err != nil || pid != desk {
		conn.Close()
		return nil, "that socket isn't the desk's own"
	}
	return conn, ""
}

// mailInboxWrite writes line to conn: "" once all of it is written, else how far it got.
func mailInboxWrite(conn net.Conn, line []byte) string {
	_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if n, err := conn.Write(line); err != nil || n != len(line) {
		return "cut off after " + strconv.Itoa(n) + " of " + strconv.Itoa(len(line)) + " bytes"
	}
	return ""
}
