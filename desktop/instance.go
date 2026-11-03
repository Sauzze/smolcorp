package desktop

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
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

// showOffice is what /raise does; a test swaps it so no window opens. It says
// if the browser could not be started, and the second instance is told.
var showOffice = func(d *Deck) error { return d.openOffice() }

// instanceRunning is the message the second instance prints (and, with no
// console to print on, shows).
const instanceRunning = "Smolcorp is already running: look for its icon by the clock, or among the hidden icons. This second copy is closing."

// instanceRaised is what it says when the first one has just been asked to show
// the office.
const instanceRaised = "Smolcorp is already running: its office is coming to the front."

// instanceNotRaised is what it says when the first one was asked but no browser
// could open the office (see notRaisedMessage).
const instanceNotRaised = "Smolcorp is already running, but no browser could open its office"

// notRaisedMessage says why no browser opened the office and gives its address
// on a line of its own, so it copies cleanly. The address carries no token, so
// a browser that isn't paired yet is asked to pair first.
func notRaisedMessage(failure string) string {
	return instanceNotRaised + " (" + failure + ").\n" + officeByHand(localBase())
}

// officeByHand is the office's plain address on a line of its own, for a
// computer with no browser to open it: no token or key in it.
func officeByHand(base string) string {
	return "To open it by hand, go to: " + base + "/office\nIt may ask you to pair this browser first."
}

// stopHint says, on Linux, how to stop the running copy (see quit.go).
var stopHint = func() string {
	if runtime.GOOS == "linux" {
		return " To stop the running copy, run: smolcorp quit"
	}
	return ""
}()

// handOverToRunning tells the user, and asks the running Smolcorp to show the
// office. Whatever happens it returns the exit code the second instance ends
// with: 0 when the office is coming, 1 when no browser could open it (the
// message and the plain address are printed all the same, and Smolcorp is
// running either way).
func handOverToRunning(quiet bool) int {
	if answered, failure := askRunningToShowOffice(Port); answered {
		if failure == "" {
			fmt.Fprintln(os.Stderr, instanceRaised+stopHint)
			return 0
		}
		message := notRaisedMessage(failure)
		fmt.Fprintln(os.Stderr, message+stopHint)
		if !quiet {
			notifyUser(message)
		}
		return 1
	}
	fmt.Fprintln(os.Stderr, instanceRunning+stopHint)
	if !quiet {
		notifyUser(instanceRunning)
	}
	return 0
}

// askRunningToShowOffice reports whether the running Smolcorp answered, and, if
// it couldn't open the office, why. A Smolcorp from before this route existed
// answers 404 or 401, which is no answer.
func askRunningToShowOffice(port int) (answered bool, failure string) {
	key, err := os.ReadFile(hookKeyFile)
	if err != nil {
		return false, ""
	}
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d%s", port, raisePath), nil)
	if err != nil {
		return false, ""
	}
	request.Header.Set(hookKeyHeader, strings.TrimSpace(string(key)))
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return false, ""
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusNoContent:
		return true, ""
	case http.StatusBadGateway:
		text, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		failure = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(text)), "The browser could not be started: "))
		if failure == "" {
			failure = "the browser could not be started"
		}
		return true, failure
	}
	return false, ""
}
