package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// Setup's Phone over HTTPS, on every system (1.26, FR2): where Tailscale stands (not installed, not signed in, serve
// off, serve on and at what address), the one `tailscale serve` line to copy, and the tap that runs it, or stops it,
// after a confirmation that shows the exact command and what it exposes (answer 4). Off the Mac, the hand-on isn't let
// in until its own piece after SN1 (serveHandOnHere): the status says so, and Turn it on is refused there.
//
// The tap is the setup page's own owner action and nothing else: not the GM, a hire, a tool with the setup key,
// MCP, or a phone (a phone's request never reaches setup at all). It runs exactly one of two commands, built here from
// this app's own port, never a funnel, never another port, never anything the request sends. What the request sends
// back is the command it was shown, which has to be the one Smolcorp would run now.

// serveHandOnHere is the one switch for `tailscale serve`'s hand-on (the ts.net Host, or a proxy's headers, through
// serve.sock): let in on a Mac only. Design B's SN1 identifies the peer positively there (peerowner.go), so only the
// real `tailscale serve` counts. Off the Mac there is no serve.sock and nothing identifies it yet
// (identifyServePeerHere says no), and the switch stays off: turning it on there is its own piece after SN1, the peer identified on Linux and Windows and then
// this one line (`return true`), reviewed, with the route dump's diff (TestTheServeHandOnStaysTheMacsUntilSN1).
// macTailscale is runtime.GOOS == "darwin", a variable so a test can say either.
func serveHandOnHere() bool { return macTailscale }

// serveMacOnly is what setup says off the Mac, and why its Turn it on is refused there.
const serveMacOnly = "HTTPS through tailscale serve is Mac only for now; on this computer, use Tailscale's own address."

// The statuses setup shows.
const (
	httpsChecking     = "checking"     // Tailscale hasn't been asked yet
	httpsNotInstalled = "notInstalled" // its command line isn't on this computer
	httpsSignedOut    = "signedOut"    // not running, or not signed in
	httpsOff          = "off"          // signed in, serve doesn't hand this app on
	httpsOn           = "on"           // serve hands this app on
)

// httpsStatus is where Tailscale stands, as setup says it.
func (a tailnetAnswer) httpsStatus() string {
	switch {
	case !a.asked:
		return httpsChecking
	case !a.installed:
		return httpsNotInstalled
	case !a.signedIn:
		return httpsSignedOut
	case a.serving != nil && *a.serving:
		return httpsOn
	}
	return httpsOff
}

// serveOnArgs and serveOffArgs are the only two commands the tap runs, after `tailscale`. On hands this app on to the
// tailnet, as HTTPS; off stops what is handed on at that HTTPS port.
func serveOnArgs() []string { return []string{"serve", "--bg", "--https=443", serveTarget()} }

// serveTarget is what `tailscale serve` hands on to: on a Mac, serve.sock, the only way in that counts as Tailscale's
// (SN1); elsewhere, where the hand-on is refused, this app's own port, as before.
func serveTarget() string {
	if serveHandOnHere() {
		return "unix:" + serveSocketPath()
	}
	return fmt.Sprintf("http://127.0.0.1:%d", Port)
}

func serveOffArgs() []string { return []string{"serve", "--https=443", "off"} }

// serveArgsFor is the command for an action ("on" or "off"), or nil.
func serveArgsFor(action string) []string {
	switch action {
	case "on":
		return serveOnArgs()
	case "off":
		return serveOffArgs()
	}
	return nil
}

// commandLine is a command as it's shown and copied: `tailscale` first, and an argument a shell would split (the
// support folder's "Application Support") in single quotes, so a copy pasted into Terminal runs the same.
func commandLine(args []string) string {
	shown := make([]string, len(args))
	for i, arg := range args {
		shown[i] = arg
		if !shellPlain.MatchString(arg) {
			shown[i] = shellQuoted(arg)
		}
	}
	return "tailscale " + strings.Join(shown, " ")
}

// shellPlain is an argument a shell passes on as it is.
var shellPlain = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// serveOnAgain is what setup says when `tailscale serve` still hands this app on the way it did before this update
// (oldWay): HTTPS through it has to be turned on again, once.
const serveOnAgain = "HTTPS through tailscale serve was on before this update. Smolcorp now takes it only through a private socket, so turn it on again, once: tap Turn it on."

// phoneHTTPSState is setup's Phone over HTTPS: where Tailscale stands, the address with serve on, the lines, and,
// where the hand-on isn't let in (off the Mac, until its own piece after SN1), the sentence that says so (null on a Mac).
// onAgain is the sentence that asks for it to be turned on again, once, while serve hands the app on the old way.
func (d *Deck) phoneHTTPSState(found tailnetAnswer) map[string]any {
	var address, notHere, onAgain any
	if found.url != "" {
		address = found.url
	}
	if !serveHandOnHere() {
		notHere = serveMacOnly
	}
	if found.oldWay {
		onAgain = serveOnAgain
	}
	return map[string]any{
		"notHere":    notHere,
		"status":     found.httpsStatus(),
		"address":    address,
		"port":       Port,
		"command":    commandLine(serveOnArgs()),
		"offCommand": commandLine(serveOffArgs()),
		"busy":       found.busy,
		"onAgain":    onAgain,
	}
}

// phoneHTTPSPlan is what the confirmation shows before a tap runs anything: the exact command, and what it does.
func (d *Deck) phoneHTTPSPlan(action string, found tailnetAnswer) map[string]any {
	args := serveArgsFor(action)
	if args == nil {
		return nil
	}
	machine := d.machine()
	address := found.url
	if address == "" {
		address = "this " + machine + "'s ts.net address"
	}
	var exposes []string
	if action == "on" {
		exposes = []string{
			fmt.Sprintf("It hands this app on to your tailnet over HTTPS, at %s, through a private socket in Smolcorp's own folder.", address),
			"Only your own devices signed in to your Tailscale can reach it, and a phone still needs a pairing code.",
			"It is not Funnel: nothing is put on the public internet.",
			"Smolcorp changes nothing else in Tailscale.",
		}
		if found.oldWay {
			exposes = append(exposes, "Tailscale still hands this app on the way it did before this update, which Smolcorp no longer lets in: this replaces it.")
		} else if found.busy {
			exposes = append(exposes, "Tailscale is already handing something else on at HTTPS port 443: this replaces it.")
		}
	} else {
		exposes = []string{
			"It stops Tailscale handing this app on at HTTPS port 443, so phones that open it through your tailnet's name can't reach it any more.",
			"Phones on the Wi-Fi or at this " + machine + "'s Tailscale address aren't affected.",
		}
	}
	return map[string]any{"action": action, "command": commandLine(args), "exposes": exposes}
}

// tailnetTap keeps two taps from running at once.
var tailnetTap sync.Mutex

// tailnetServeTimeout is how long `tailscale serve` may take: the first time on a tailnet it can wait on a page the
// owner has to open, and it is let go of after this.
var tailnetServeTimeout = 20 * time.Second

// runTailscaleServe runs one of the two commands with Tailscale's command line, and gives what it printed. A
// variable, so a test runs a stand-in; it still refuses anything but those two.
var runTailscaleServe = func(tool string, args []string) (string, error) {
	if !slices.Equal(args, serveOnArgs()) && !slices.Equal(args, serveOffArgs()) {
		return "", fmt.Errorf("not a command Smolcorp runs: %q", args)
	}
	ctx, cancel := context.WithTimeout(context.Background(), tailnetServeTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, tool, args...)
	command.Env = os.Environ()
	if os.Getenv("TERM") == "" {
		command.Env = append(command.Env, "TERM=dumb")
	}
	hideGitWindow(command) // keeps a console window from flashing up on Windows, as it does for git
	var out bytes.Buffer
	command.Stdout, command.Stderr = &out, &out
	command.WaitDelay = time.Second
	err := command.Run()
	return out.String(), err
}

// tailnetRoute serves /setup/tailnet/*: the status, the confirmation and the tap. The setup page alone: a tool with
// the setup key, the GM, a hire, or no one is told no here too, whatever the table says, so a row that goes missing
// can't open it. Says whether the route was its.
func (d *Deck) tailnetRoute(w http.ResponseWriter, r *request) bool {
	if !strings.HasPrefix(r.URL.Path, "/setup/tailnet/") {
		return false
	}
	if r.caller != callerPage {
		writeStatus(w)(http.StatusForbidden, routeRefusal(r.Method, r.URL.Path))
		return true
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/setup/tailnet/status":
		writeJSON(w, http.StatusOK, d.phoneHTTPSState(d.tailnetNow()))
	case r.Method == http.MethodGet && r.URL.Path == "/setup/tailnet/serve":
		action := r.URL.Query().Get("action")
		if action == "on" && !serveHandOnHere() {
			writeStatus(w)(http.StatusForbidden, serveMacOnly)
			return true
		}
		plan := d.phoneHTTPSPlan(action, d.tailnetNow())
		if plan == nil {
			writeStatus(w)(http.StatusBadRequest, "That isn't something to turn on or off.")
			return true
		}
		writeJSON(w, http.StatusOK, plan)
	case r.Method == http.MethodPost && r.URL.Path == "/setup/tailnet/serve":
		d.tailnetServe(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
	return true
}

// tailnetServe is the tap: it runs the command it was shown, and only that. The request says the action and sends
// back the exact command the confirmation showed.
func (d *Deck) tailnetServe(w http.ResponseWriter, r *request) {
	var given struct {
		Action  string `json:"action"`
		Command string `json:"command"`
	}
	data, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
	if json.Unmarshal(data, &given) != nil {
		writeStatus(w)(http.StatusBadRequest, "That isn't a tap.")
		return
	}
	args := serveArgsFor(given.Action)
	if args == nil {
		writeStatus(w)(http.StatusBadRequest, "That isn't something to turn on or off.")
		return
	}
	// Off the Mac, the hand-on is refused until its own piece after SN1, so turning it on would hand on nothing that's
	// let in. Turning it off stays: it only stops what is handed on.
	if given.Action == "on" && !serveHandOnHere() {
		writeStatus(w)(http.StatusForbidden, serveMacOnly)
		return
	}
	// Nothing runs before the owner has been shown the command: a tap with no command sent back isn't one.
	if given.Command == "" {
		writeStatus(w)(http.StatusPreconditionRequired, "Check the command first: "+commandLine(args))
		return
	}
	if given.Command != commandLine(args) {
		writeStatus(w)(http.StatusConflict, "That isn't the command Smolcorp would run now. Look at it again: "+commandLine(args))
		return
	}
	if !tailnetTap.TryLock() {
		writeStatus(w)(http.StatusConflict, "Tailscale is being changed already. Wait a moment.")
		return
	}
	defer tailnetTap.Unlock()
	// Looked at again now, not by the last minute's answer: it may be different.
	found := d.tailnetLookNow(false)
	if problem := tapRefusal(given.Action, found, d.machine()); problem != "" {
		writeStatus(w)(http.StatusConflict, problem)
		return
	}
	tool := tailscaleTool()
	if tool == "" {
		writeStatus(w)(http.StatusConflict, tapRefusal(given.Action, tailnetAnswer{asked: true}, d.machine()))
		return
	}
	if out, err := runTailscaleServe(tool, args); err != nil {
		said := strings.TrimSpace(out)
		if len(said) > 400 {
			said = said[:400] + "…"
		}
		if said == "" {
			said = err.Error()
		}
		writeStatus(w)(http.StatusBadGateway, "Tailscale didn't do it: "+said)
		return
	}
	writeJSON(w, http.StatusOK, d.phoneHTTPSState(d.tailnetLookNow(false)))
}

// tapRefusal is why a tap can't go ahead where Tailscale stands, or "" if it can: on only where it's off, off only
// where it's this app Tailscale hands on, so what else is handed on is never stopped from here.
func tapRefusal(action string, found tailnetAnswer, machine string) string {
	switch status := found.httpsStatus(); {
	case status == httpsNotInstalled:
		return "Tailscale isn't installed on this " + machine + "."
	case status == httpsSignedOut:
		return "Tailscale isn't signed in on this " + machine + ". Open it and sign in."
	case action == "on" && status == httpsOn:
		return "Tailscale already hands this app on."
	case action == "off" && status != httpsOn:
		return "Tailscale isn't handing this app on."
	}
	return ""
}
