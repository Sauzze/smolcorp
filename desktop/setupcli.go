package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// smolcorp setup: the whole setup from a terminal, for you or for an agent
// you ask to do it. It starts Smolcorp if it isn't running, puts right what
// can be put right without a choice of yours (installing it, connecting
// your agents, on Windows the firewall's rules, which Windows asks you to
// OK once), then prints the checklist setup's Start here shows, and ends
// with 0 when nothing is in the way, 1 when something is.
//
//	smolcorp setup           as above
//	smolcorp setup --check   the checklist only, changing nothing
//	smolcorp setup --open    and then the setup page, to pair a phone

// The fixes setup makes by itself: none is a choice of yours.
var setupFixes = []string{fixInstall, fixRepair, fixFirewall}

// RunSetup is `smolcorp setup`.
func RunSetup(args []string) int {
	checkOnly, open := slices.Contains(args, "--check"), slices.Contains(args, "--open")
	client := setupClient{base: localBase(), http: &http.Client{Timeout: 2 * time.Minute}}
	if !client.answers() {
		if checkOnly {
			fmt.Println("Smolcorp isn't running. Open it, or run smolcorp setup without --check to start it.")
			return 1
		}
		fmt.Println("Starting Smolcorp…")
		if err := startApp(); err != nil {
			fmt.Fprintf(os.Stderr, "Smolcorp couldn't start: %v\n", err)
			return 1
		}
		if !client.waitUntilItAnswers(30 * time.Second) {
			fmt.Fprintln(os.Stderr, "Smolcorp didn't answer on its port within 30 s. Open it from the app menu, then try again.")
			return 1
		}
	}
	report, err := client.selfTest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Setup can't be read: %v\n", err)
		return 1
	}
	if !checkOnly {
		for _, c := range report.Checks {
			if c.Status != checkFix || !slices.Contains(setupFixes, c.Fix) {
				continue
			}
			fmt.Printf("Fixing: %s…\n", c.Title)
			if c.Fix == fixFirewall {
				fmt.Println("  Windows asks for your OK: choose Yes.")
			}
			if err := client.fix(c.Fix); err != nil {
				fmt.Printf("  %v\n", err)
			}
			if c.Fix == fixInstall {
				// The installed copy takes over: what follows is its checklist.
				client.waitUntilItAnswers(30 * time.Second)
			}
		}
		if report, err = client.selfTest(); err != nil {
			fmt.Fprintf(os.Stderr, "Setup can't be read: %v\n", err)
			return 1
		}
	}
	printChecklist(os.Stdout, report.Checks)
	if open {
		openURL(localBase() + "/setup")
	}
	if !report.Ready {
		return 1
	}
	return 0
}

// printChecklist writes the checklist as a terminal shows it: a mark, the
// line's title, and what was found.
func printChecklist(out io.Writer, checks []check) {
	marks := map[string]string{checkOK: "✓", checkFix: "✗", checkTodo: "•", checkInfo: "i"}
	fmt.Fprintln(out)
	for _, c := range checks {
		fmt.Fprintf(out, "%s %s\n    %s\n", marks[c.Status], c.Title, c.Detail)
		if c.Status != checkOK && c.Label != "" && !slices.Contains(setupFixes, c.Fix) {
			fmt.Fprintf(out, "    On the setup page: %s.\n", c.Label)
		}
	}
	if ready(checks) {
		fmt.Fprintln(out, "\nReady. Steps marked • are on the setup page: smolcorp setup --open.")
	} else {
		fmt.Fprintln(out, "\nNot ready: see the lines marked ✗.")
	}
}

// selfTestAnswer is GET /setup/selftest's answer.
type selfTestAnswer struct {
	Checks []check `json:"checks"`
	Ready  bool    `json:"ready"`
}

// setupClient is setup's API, as a tool on this computer reaches it: with
// the setup key the running app wrote for it.
type setupClient struct {
	base string
	http *http.Client
}

func (c setupClient) answers() bool {
	quick := &http.Client{Timeout: 2 * time.Second}
	response, err := quick.Get(c.base + "/pair")
	if err != nil {
		return false
	}
	response.Body.Close()
	return true
}

func (c setupClient) waitUntilItAnswers(wait time.Duration) bool {
	for deadline := time.Now().Add(wait); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		if c.answers() {
			return true
		}
	}
	return false
}

func (c setupClient) send(method, path string) (*http.Response, error) {
	key, err := os.ReadFile(setupKeyFile)
	if err != nil {
		return nil, errors.New("no setup key: is this the computer Smolcorp runs on, as you?")
	}
	request, err := http.NewRequest(method, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set(setupKeyHeader, strings.TrimSpace(string(key)))
	request.Header.Set(setupHeader, "1")
	return c.http.Do(request)
}

func (c setupClient) selfTest() (selfTestAnswer, error) {
	var answer selfTestAnswer
	response, err := c.send(http.MethodGet, "/setup/selftest")
	if err != nil {
		return answer, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return answer, fmt.Errorf("Smolcorp answered %s: is it an older version? Update it, then try again", response.Status)
	}
	err = json.NewDecoder(response.Body).Decode(&answer)
	return answer, err
}

func (c setupClient) fix(name string) error {
	response, err := c.send(http.MethodPost, "/setup/fix?fix="+name)
	if err != nil {
		// The install's own copy took over before answering.
		if name == fixInstall {
			return nil
		}
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		said, _ := io.ReadAll(io.LimitReader(response.Body, 1000))
		return errors.New(strings.TrimSpace(string(said)))
	}
	return nil
}

// startApp starts Smolcorp's app, left running: on Windows the program with
// the tray beside this console one, elsewhere this program with no command.
func startApp() error {
	program, err := executable()
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		if app := filepath.Join(filepath.Dir(program), "Smolcorp.exe"); fileExists(app) {
			program = app
		}
	}
	return startProgram(program)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
