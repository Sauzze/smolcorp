package gm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// `smolcorp gm login` (1.26, SK2): a General Manager you start yourself in a terminal gets its token only when you
// click Allow in Smolcorp's own window, after checking that the code printed here is the one the window shows (the
// owner's B3). No file holds the token: the app hands it to this process once, and this process starts the GM with it
// in the GM's environment alone ($SMOLCORP_GM_TOKEN), where every `smolcorp gm` the GM runs finds it. The token is
// never printed. Each login gets a new token, so one an earlier start held stops working.
//
//	smolcorp gm login                  ask, then start `claude` here as the GM
//	smolcorp gm login -- <command…>    ask, then start that command instead
//
// AskForKey is the asking itself, which `smolcorp demo` uses too (desktop/demooffice.go).

// terminalAskPath is the app's route for a terminal's ask (desktop/terminalask.go).
const terminalAskPath = "/terminal/ask"

// The kinds of key a terminal asks for: the GM's token, or a tool's key (smolcorp demo, status and desks).
const (
	AskGM   = "gm"
	AskTool = "tool"
)

// gmLoginCommand is what login starts when it's given no command.
var gmLoginCommand = []string{"claude"}

// askPatience is how long a terminal waits for your answer: a little longer than the app keeps an ask.
var askPatience = 4 * time.Minute

// askPollWait is how long one wait for the answer lasts at most, the app's own wait and some.
var askPollWait = 30 * time.Second

// startGM replaces this process with the GM's command, its environment given: a variable, for the tests.
var startGM = execGM

// CheckProgram is why the program found for a command's first word mustn't start, or nil: the check every agent
// Smolcorp starts goes through (desktop/agentcheck.go, SG2), which cmd/smolcorp puts here, as this package can't
// import desktop. A hire can write the folders PATH looks in, so the claude found there may be whatever it left
// (RMSY126k nit 6). Unset, login starts nothing.
var CheckProgram func(word, program string) error

// checkLoginProgram is why login mustn't start program, found for word, in plain words, or nil.
func checkLoginProgram(word, program string) error {
	if CheckProgram == nil {
		return &Error{Code: exitApp, Msg: "this smolcorp can't check the program it would start, so your General Manager wasn't started."}
	}
	if err := CheckProgram(word, program); err != nil {
		return &Error{Code: exitApp, Msg: err.Error() + " Your General Manager wasn't started."}
	}
	return nil
}

// AskForKey asks the Smolcorp on this computer for a key of kind (AskGM or AskTool), says in out the code to check and
// where, and waits for your answer. The key is returned, never printed.
func AskForKey(env Env, kind, folder string, out io.Writer) (string, error) {
	if strings.TrimSpace(env.Getenv(hireKeyEnv)) != "" {
		return "", &Error{Code: exitApp, Msg: "a hire can't ask for a key: only you can, in your own terminal."}
	}
	port, err := env.port()
	if err != nil {
		return "", &Error{Code: exitUsage, Msg: err.Error()}
	}
	c := newClient(fmt.Sprintf("http://127.0.0.1:%d", port), "")
	data, err := c.Post(terminalAskPath, url.Values{"kind": {kind}, "folder": {folder}}, nil)
	if err != nil {
		var e *Error
		if errors.As(err, &e) && e.Status == 404 {
			return "", &Error{Code: exitApp, Msg: appName + " here doesn't take a terminal's ask yet: update it."}
		}
		if errors.As(err, &e) && e.Reason != "" {
			return "", &Error{Code: exitApp, Status: e.Status, Msg: e.Reason}
		}
		return "", err
	}
	var asked struct {
		Ticket string `json:"ticket"`
		Code   string `json:"code"`
		Where  string `json:"where"`
	}
	if json.Unmarshal(data, &asked) != nil || asked.Ticket == "" || asked.Code == "" {
		return "", &Error{Code: exitApp, Msg: appName + "'s answer to the ask can't be read."}
	}
	fmt.Fprintf(out, "%s asks you in %s.\nCheck that it shows this code: %s\nThen click Allow there. Waiting…\n", appName, asked.Where, asked.Code)
	deadline := time.Now().Add(askPatience)
	for time.Now().Before(deadline) {
		data, err := c.Get(terminalAskPath, url.Values{"ticket": {asked.Ticket}}, askPollWait)
		if err != nil {
			var e *Error
			if errors.As(err, &e) && e.Status == 404 {
				return "", &Error{Code: exitApp, Msg: "the ask ran out before an answer. Run this again when you're at Smolcorp's window."}
			}
			return "", err
		}
		var answer struct {
			State string `json:"state"`
			Key   string `json:"key"`
		}
		if json.Unmarshal(data, &answer) != nil {
			return "", &Error{Code: exitApp, Msg: appName + "'s answer can't be read."}
		}
		switch answer.State {
		case "allowed":
			if answer.Key == "" {
				return "", &Error{Code: exitApp, Msg: appName + " allowed it, but sent no key."}
			}
			return answer.Key, nil
		case "denied":
			return "", &Error{Code: exitApp, Msg: "you said no in " + appName + "'s window: nothing was given."}
		}
	}
	return "", &Error{Code: exitApp, Msg: "no answer in time. Run this again when you're at Smolcorp's window."}
}

// runLogin is `smolcorp gm login [-- command…]`.
func runLogin(args []string, env Env, stdout, stderr io.Writer) int {
	command := gmLoginCommand
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) > 0 {
		if strings.HasPrefix(args[0], "-") {
			return usageError(stderr, fmt.Sprintf("login takes no option %q: put the command to start after --", args[0]))
		}
		command = args
	}
	if strings.TrimSpace(env.Getenv(tokenEnv)) != "" {
		return finish(&Error{Code: exitUsage, Msg: "$" + tokenEnv + " is set already: this is a GM's environment. Start a new terminal for login."}, stderr)
	}
	program, err := exec.LookPath(command[0])
	if err != nil {
		return finish(&Error{Code: exitUsage, Msg: fmt.Sprintf("can't find %s to start as the GM.", command[0])}, stderr)
	}
	// Checked before the ask, so you aren't asked for a token nothing will use, and again just before the start: the
	// program may have changed while you answered.
	if err := checkLoginProgram(command[0], program); err != nil {
		return finish(err, stderr)
	}
	folder, _ := os.Getwd()
	token, err := AskForKey(env, AskGM, folder, stdout)
	if err != nil {
		return finish(err, stderr)
	}
	token, err = validToken(token)
	if err != nil {
		return finish(err, stderr)
	}
	if err := checkLoginProgram(command[0], program); err != nil {
		return finish(err, stderr)
	}
	fmt.Fprintf(stdout, "Allowed. Starting %s as your General Manager.\n", strings.Join(command, " "))
	environment := append(withoutVar(os.Environ(), tokenEnv), tokenEnv+"="+token)
	if err := startGM(program, command, environment); err != nil {
		return finish(&Error{Code: exitApp, Msg: fmt.Sprintf("couldn't start %s: %v", command[0], err)}, stderr)
	}
	return exitOK
}

// withoutVar is the environment without name.
func withoutVar(environment []string, name string) []string {
	kept := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, name+"=") {
			kept = append(kept, entry)
		}
	}
	return kept
}
