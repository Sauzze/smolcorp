package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// claudeSessionsDir is Claude Code's own list of the sessions running on
// this computer, one file each, with the name and socket another session's
// SendMessage names it by.
var claudeSessionsDir = filepath.Join(homeDir, ".claude", "sessions")

// Far more than a session's file holds: a bigger one isn't one, and isn't read.
const claudeSessionMostBytes = 64 << 10

// claudePeer is the session a SendMessage `to` names, by its id: a name,
// with its "[ref]" when two share it, or "uds:" and a socket. Empty when
// none, or more than one, fits. Only the .json files are read, and only
// their pid, id, name and socket: never the keys kept beside them.
func claudePeer(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	files, _ := filepath.Glob(filepath.Join(claudeSessionsDir, "*.json"))
	socket, isSocket := strings.CutPrefix(target, "uds:")
	pid, _ := strconv.Atoi(strings.TrimSuffix(filepath.Base(socket), ".sock"))
	name, _, _ := strings.Cut(target, " [")
	match := ""
	for _, file := range files {
		if info, err := os.Stat(file); err != nil || info.Size() > claudeSessionMostBytes {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var peer struct {
			PID                 int    `json:"pid"`
			SessionID           string `json:"sessionId"`
			Name                string `json:"name"`
			MessagingSocketPath string `json:"messagingSocketPath"`
		}
		if json.Unmarshal(data, &peer) != nil || peer.SessionID == "" {
			continue
		}
		fits := peer.Name == name
		if isSocket {
			fits = peer.MessagingSocketPath == socket || pid != 0 && peer.PID == pid
		}
		if fits {
			if match != "" {
				return "" // two fit: a guess could send the wrong one over
			}
			match = peer.SessionID
		}
	}
	return match
}
