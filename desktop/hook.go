package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// The hook is `smolcorp hook [agent]`, which every agent runs at each step
// of a session: the counterpart of the Mac app's report.sh, built in because
// Windows has no bash or curl. The event JSON arrives on stdin and is passed
// on as it is, with headers for the agent, the time the event was raised,
// the session's tab (see Tab), and the folder the agent was started in, where
// the agent says: the event's cwd follows the agent into subfolders. The key
// is read from its file, so it never appears on a command line.
//
// It never blocks or fails the session. If the app isn't running, the event
// is simply dropped.

const (
	hookKeyHeader = "X-AgentDeck-Token"
	hookTimeout   = 2 * time.Second
	// Far more than any event; a runaway stdin is cut off here.
	maxEventSize = 64 << 20
	// Under the deck's limit on a request, 1 MiB. A bigger event, such as a
	// tool's whole output, is trimmed to fit rather than lost: see trimmedEvent.
	mostSent    = 900 << 10
	longestText = 4096
)

// RunHook always returns 0: a hook that fails would hold up the agent.
func RunHook(args []string) int {
	agent := string(Claude)
	if len(args) > 0 {
		agent = args[0]
	}
	event, _ := io.ReadAll(io.LimitReader(os.Stdin, maxEventSize))
	event = trimmedEvent(event)
	key, err := os.ReadFile(hookKeyFile)
	if err != nil {
		return 0
	}
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/hook", Port), bytes.NewReader(event))
	if err != nil {
		return 0
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(hookKeyHeader, strings.TrimSpace(string(key)))
	request.Header.Set("X-AgentDeck-Agent", agent)
	request.Header.Set("X-AgentDeck-At", strconv.FormatInt(time.Now().UnixMilli(), 10))
	request.Header.Set("X-AgentDeck-Tab", tabFromEnvironment(Agent(agent)))
	request.Header.Set("X-AgentDeck-Project", startFolderFromEnvironment())
	client := http.Client{Timeout: hookTimeout}
	if response, err := client.Do(request); err == nil {
		response.Body.Close()
	}
	return 0
}

// startFolderFromEnvironment is the folder the agent was started in, from the
// variable the agent sets for its hooks, or "".
func startFolderFromEnvironment() string {
	if folder := os.Getenv("CLAUDE_PROJECT_DIR"); folder != "" {
		return folder
	}
	return os.Getenv("GEMINI_PROJECT_DIR")
}

// tabFromEnvironment names the session's tab from the variables the launcher
// and the terminals set, or "" when there is none to drive.
func tabFromEnvironment(agent Agent) string {
	// Codex runs its hooks in a shared background service, which keeps the
	// variables of whichever terminal started it: they name the wrong tab.
	// The app finds Codex's launcher by folder instead.
	if agent == Codex {
		return ""
	}
	// Agents started before the app was Smolcorp carry the old name
	for _, name := range []string{"SMOLCORP_SESSION", "SAUZZEDECK_SESSION"} {
		if id := os.Getenv(name); id != "" {
			return "launcher:" + id
		}
	}
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		socket, _, _ := strings.Cut(os.Getenv("TMUX"), ",")
		return "tmux:" + pane + "@" + socket
	}
	if pane := os.Getenv("WEZTERM_PANE"); pane != "" {
		return "wezterm:" + pane + "@" + os.Getenv("WEZTERM_UNIX_SOCKET")
	}
	if window := os.Getenv("KITTY_WINDOW_ID"); window != "" {
		if socket := os.Getenv("KITTY_LISTEN_ON"); socket != "" {
			return "kitty:" + window + "@" + socket
		}
	}
	return ""
}

// trimmedEvent is an event small enough for the deck to take. One over mostSent
// loses the tool's output, which the deck never shows, and has each text
// longer than longestText shortened: what the office shows, as the file a
// tool reads or the command it runs, is in the first of it.
func trimmedEvent(event []byte) []byte {
	if len(event) <= mostSent {
		return event
	}
	decoder := json.NewDecoder(bytes.NewReader(event))
	decoder.UseNumber()
	var fields map[string]any
	if decoder.Decode(&fields) != nil {
		return event
	}
	delete(fields, "tool_response")
	out, err := json.Marshal(shortened(fields))
	if err != nil {
		return event
	}
	return out
}

func shortened(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, inner := range v {
			v[key] = shortened(inner)
		}
	case []any:
		for i, inner := range v {
			v[i] = shortened(inner)
		}
	case string:
		if runes := []rune(v); len(runes) > longestText {
			return string(runes[:longestText]) + "..."
		}
	}
	return value
}
