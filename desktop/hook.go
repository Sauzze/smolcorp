package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// The hook is `smolcorp hook [agent]`, which every agent runs at each step
// of a session: the counterpart of the Mac app's report.sh, built in because
// Windows has no bash or curl. The event JSON arrives on stdin and is passed
// on as it is, with headers for the agent, the time the event was raised,
// the session's tab (see Tab), and the folder the agent was started in, where
// the agent says: the event's cwd follows the agent into subfolders. The key
// is read from its file, so it never appears on a command line.
//
// A headless claude run, one with no terminal at all, reports in only when
// whoever started it sets SMOLCORP_HEADLESS=1: its tab is then
// headless:<session id>, which the app never matches to a terminal tab, and
// the pid of the agent's process goes with it, so the app can tell when it
// has gone without waiting for an event, and the folder it was launched
// from, where the launcher says in SMOLCORP_PARENT_FOLDER: the working folder
// of the employee who hired it.
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
	tab := tabFromEnvironment(Agent(agent), event)
	if strings.HasPrefix(tab, "headless:") {
		request.Header.Set("X-AgentDeck-PID", strconv.Itoa(headlessAgentPID()))
		if parent := os.Getenv("SMOLCORP_PARENT_FOLDER"); parent != "" && strings.IndexFunc(parent, unicode.IsControl) < 0 {
			request.Header.Set("X-AgentDeck-Parent", parent)
		}
	} else if tab == "" && headlessRequested(Agent(agent)) {
		// Asked to report as headless with an ID that can't be believed: as
		// report.sh, nothing is sent rather than a session the app would place
		// by its folder.
		return 0
	}
	request.Header.Set("X-AgentDeck-Tab", tab)
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
// and the terminals set, or "" when there is none to drive. With none, a
// claude run that says it is headless is named by the event's session ID.
func tabFromEnvironment(agent Agent, event []byte) string {
	tab := terminalTabFromEnvironment(agent)
	if tab == "" && headlessRequested(agent) {
		if id := sessionIDOf(event); headlessID.MatchString(id) {
			return "headless:" + id
		}
	}
	return tab
}

// headlessID is the only shape of session ID believed in a headless tab, so
// that one can never carry anything else into a header.
var headlessID = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// headlessRequested is whether the run was started with SMOLCORP_HEADLESS=1.
// Only claude's runs are taken as headless: the others find their tabs
// their own way.
func headlessRequested(agent Agent) bool {
	return agent == Claude && os.Getenv("SMOLCORP_HEADLESS") == "1"
}

// sessionIDOf reads the event's session_id, or "".
func sessionIDOf(event []byte) string {
	var fields struct {
		SessionID string `json:"session_id"`
	}
	json.Unmarshal(event, &fields)
	return fields.SessionID
}

// headlessAgentPID is the pid of the agent the hook reports for: the nearest
// claude up the process tree, where that can be read cheaply; with none, the
// nearest node, running it, as a node further up may be whatever started the
// run, and outlive it; else the process that ran the hook.
func headlessAgentPID() int {
	return nearestAgent(os.Getppid(), processParent)
}

// nearestAgent walks up from pid by parent, as headlessAgentPID says.
func nearestAgent(pid int, parentOf func(int) (int, string, bool)) int {
	start, node := pid, 0
	for steps := 0; pid > 1 && steps < 32; steps++ {
		next, name, ok := parentOf(pid)
		if !ok {
			break
		}
		switch strings.ToLower(name) {
		case "claude":
			return pid
		case "node":
			if node == 0 {
				node = pid
			}
		}
		pid = next
	}
	if node != 0 {
		return node
	}
	return start
}

// terminalTabFromEnvironment is the tab named by the launcher's or a
// terminal's variables, or "".
func terminalTabFromEnvironment(agent Agent) string {
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
