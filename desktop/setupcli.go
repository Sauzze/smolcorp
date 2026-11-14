package desktop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"
)

// smolcorp setup: setup's checklist from a terminal, for you or for an agent
// you ask to run it. It starts Smolcorp if it isn't running, then prints the
// checklist setup's Start here shows, and ends with 0 when nothing is in the
// way, 1 when something is. It changes nothing: it holds only the check-only
// key (selftestkey.go), which opens the checklist and nothing else, so what
// the checklist finds is put right in Setup (SANDBOX-PLAN §3.1, SK2).
//
//	smolcorp setup           as above
//	smolcorp setup --check   the same, without starting Smolcorp (on Linux and Windows)
//	smolcorp setup --open    and then where to get a link to the setup page, to pair a phone (it makes none)
//
// On a Mac it opens Smolcorp.app first if it is closed (only when setup is
// aimed at the app's own port).

// setupSystem is the system setup runs on; the tests change it.
var setupSystem = runtime.GOOS

// How long setup waits for the app it started to answer; the tests shorten it.
var setupStartWait = 30 * time.Second

// macNotRunning is what a Mac says when the app can't be opened or doesn't answer.
const macNotRunning = "Smolcorp isn't running. Open Smolcorp from Applications or Spotlight."

// setupAimedAtDefaultPort is true when SMOLCORP_PORT isn't set to another port.
func setupAimedAtDefaultPort() bool { return Port == portFromEnvironment("") }

// setupMayOpenApp says whether setup may open the installed Mac app. On
// another port it is a test copy, and opening the installed app would not
// help it; the tests change this.
var setupMayOpenApp = setupAimedAtDefaultPort

// RunSetup is `smolcorp setup`.
func RunSetup(args []string) int {
	checkOnly, open := slices.Contains(args, "--check"), slices.Contains(args, "--open")
	if !checkOnly {
		// What the checklist finds is put right in Setup: this only looks (and starts Smolcorp if it's closed).
		if setupSystem == "darwin" {
			fmt.Println("On a Mac, setup is in Smolcorp's Settings. This only checks, and changes nothing.")
		} else {
			fmt.Println("Setup is on Smolcorp's setup page. This only checks, and changes nothing.")
		}
	}
	client := setupClient{base: setupBase(), http: &http.Client{Timeout: 2 * time.Minute}}
	if !client.answers() {
		mac := setupSystem == "darwin"
		if checkOnly && !mac {
			fmt.Println("Smolcorp isn't running. Open it, or run smolcorp setup without --check to start it.")
			return 1
		}
		if mac && !setupMayOpenApp() {
			fmt.Println(macNotRunning)
			return 1
		}
		fmt.Println("Starting Smolcorp…")
		if err := startApp(); err != nil {
			if mac {
				fmt.Fprintf(os.Stderr, "Smolcorp couldn't be opened: %v\n", err)
				fmt.Println(macNotRunning)
			} else {
				fmt.Fprintf(os.Stderr, "Smolcorp couldn't start: %v\n", err)
			}
			return 1
		}
		if !client.waitUntilItAnswers(setupStartWait) {
			if mac {
				fmt.Println(macNotRunning)
			} else {
				fmt.Fprintln(os.Stderr, "Smolcorp didn't answer on its port within 30 s. Open it from the app menu, then try again.")
			}
			return 1
		}
	}
	report, err := client.selfTest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Setup can't be read: %v\n", err)
		return 1
	}
	printChecklist(os.Stdout, report.Checks)
	if open {
		client.openSetup(os.Stdout)
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
		if c.Status != checkOK && c.Label != "" {
			fmt.Fprintf(out, "    On the setup page: %s.\n", c.Label)
		}
	}
	if ready(checks) {
		fmt.Fprintf(out, "\nReady. Steps marked • are on the setup page: %s.\n", setupMenuWords())
	} else {
		fmt.Fprintln(out, "\nNot ready: see the lines marked ✗.")
	}
}

// selfTestAnswer is GET /setup/selftest's answer.
type selfTestAnswer struct {
	Checks []check `json:"checks"`
	Ready  bool    `json:"ready"`
}

// setupClient is setup's checklist, as a terminal on this computer reaches it:
// with the check-only key the running app wrote for it.
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
	key, err := readSelftestKey()
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(method, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set(selftestKeyHeader, key)
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

// openSetup says where to get a link to the setup page. It makes none: a
// link that opens setup, and so pairing, must come from the app itself (the
// tray's Open Setup…), never from a program that only holds the check-only key.
func (c setupClient) openSetup(out io.Writer) {
	if setupSystem == "darwin" {
		fmt.Fprintln(out, "\nTo open setup, choose Open Settings in Browser from Smolcorp's icon in the menu bar.")
		return
	}
	if setupSystem == "windows" { // the tray is always there
		fmt.Fprintln(out, "\nTo open setup, choose Open Setup… from the Smolcorp icon in the tray, or use the link smolcorp prints as it starts.")
		return
	}
	fmt.Fprintln(out, "\nTo open setup, choose Open Setup… from the Smolcorp icon in the tray. With no tray, use the link smolcorp prints as it starts; for a new one, "+noTrayNewLink+".")
	if !setupHasDesktop() { // no browser to open anything: the office's address is what's left
		fmt.Fprintln(out, "\nThis computer has no desktop to open a browser on. "+officeByHand(c.base))
	}
}

// setupHasDesktop says if there is a desktop to open a browser on: a variable, for the tests.
var setupHasDesktop = hasDesktop

// setupMenuWords is the menu item that opens the setup page, in the words of this system: the Mac's menu bar
// icon says Open Settings in Browser, the tray on Linux and Windows says Open Setup….
func setupMenuWords() string {
	if setupSystem == "darwin" {
		return "choose Open Settings in Browser from the menu bar icon"
	}
	if setupSystem == "windows" {
		return "choose Open Setup… from the tray icon"
	}
	return "choose Open Setup… from the tray icon; with no tray, use the link Smolcorp prints as it starts (for a new one, " + noTrayNewLink + ")"
}

// startApp starts Smolcorp's app, left running: on a Mac it opens Smolcorp.app
// (this program is only the command inside it), on Windows the program with
// the tray beside this console one, elsewhere this program with no command.
func startApp() error {
	program, err := executable()
	if err != nil {
		return err
	}
	if setupSystem == "darwin" {
		return openMacApp(program)
	}
	if runtime.GOOS == "windows" {
		if app := filepath.Join(filepath.Dir(program), "Smolcorp.exe"); fileExists(app) {
			program = app
		}
	}
	return startProgram(program)
}

// macBundleID is Smolcorp.app's identifier, used when the bundle can't be read.
const macBundleID = "io.github.sauzze.agentdeck"

var bundleIDPattern = regexp.MustCompile(`<key>CFBundleIdentifier</key>\s*<string>([^<]+)</string>`)

// openMacApp opens the app this program sits in, by its bundle identifier.
var openMacApp = func(program string) error {
	return startProgramWith("/usr/bin/open", []string{"-b", macBundleIdentifier(program)})
}

// macBundleIdentifier reads the identifier from the .app that holds program,
// and falls back to the known one.
func macBundleIdentifier(program string) string {
	if resolved, err := filepath.EvalSymlinks(program); err == nil {
		program = resolved
	}
	for dir := filepath.Dir(program); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if !strings.HasSuffix(dir, ".app") {
			continue
		}
		if plist, err := os.ReadFile(filepath.Join(dir, "Contents", "Info.plist")); err == nil {
			if m := bundleIDPattern.FindSubmatch(plist); m != nil {
				return strings.TrimSpace(string(m[1]))
			}
		}
		break
	}
	return macBundleID
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
