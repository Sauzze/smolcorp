package desktop

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// One Smolcorp per session. A second one would find the port taken, and used
// to stay alive with no icon and no server. Now it asks the first one to show
// the office, says so, and leaves.
//
// The lock itself is the platform's (instance_windows.go: a named mutex of the
// Windows session; instance_unix.go: a lock on a file next to the keys).

// raisePath is what the second instance asks the first, on this computer's own
// port, with the hook's key, as /hook is asked: the first opens the office
// window, or brings the one that is open to the front.
const raisePath = "/raise"

// showOffice is what /raise does; a test swaps it so no window opens.
var showOffice = func(d *Deck) { d.openOffice() }

// instanceRunning is the message the second instance prints (and, with no
// console to print on, shows).
const instanceRunning = "Smolcorp is already running: look for its icon by the clock, or among the hidden icons. This second copy is closing."

// instanceRaised is what it says when the first one has just been asked to show
// the office.
const instanceRaised = "Smolcorp is already running: its office is coming to the front."

// handOverToRunning tells the user, and asks the running Smolcorp to show the
// office. Whatever happens it returns the exit code the second instance ends
// with: 0, since nothing failed.
func handOverToRunning(quiet bool) int {
	if askRunningToShowOffice(Port) {
		fmt.Fprintln(os.Stderr, instanceRaised)
		return 0
	}
	fmt.Fprintln(os.Stderr, instanceRunning)
	if !quiet {
		notifyUser(instanceRunning)
	}
	return 0
}

// askRunningToShowOffice reports whether the running Smolcorp answered. A
// Smolcorp from before this route existed answers 404 or 401, which is false.
func askRunningToShowOffice(port int) bool {
	key, err := os.ReadFile(hookKeyFile)
	if err != nil {
		return false
	}
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d%s", port, raisePath), nil)
	if err != nil {
		return false
	}
	request.Header.Set(hookKeyHeader, strings.TrimSpace(string(key)))
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusNoContent
}
