package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/gm"
)

// GMTOKENWIN1 (SIERRA, 15 Nov: a user's Windows 1.24.2): a General Manager hired from Setup the plain way (Windows
// without managed windows, a computer without tmux) started with no $SMOLCORP_GM_TOKEN, so every `smolcorp gm` said
// there was no GM token. Its token now reaches its launcher alone, over loopback, as an engine GM's does, and the
// launcher sets it on the agent's process alone. No row prints a token: each says only whether one was there.

// plainGMDeck is a deck hiring the plain way (no tmux, managed windows off) with a GM role and a hire's role.
func plainGMDeck(t *testing.T) (*Deck, *hiring) {
	t.Helper()
	d, h, _ := plainHireDeck(t)
	d.config.Roles = []Role{{ID: gmRole, Name: "General Manager", Agent: "claude"}, {ID: "janitor", Name: "Janitor", Agent: "claude"}}
	managerOn(d) // the GM's routes answer only with the manager on
	return d, h
}

// plainRoleHire hires roleID the plain way, as Setup's sheet does, and gives the launcher's arguments (after `run`)
// and its one-time token.
func plainRoleHire(t *testing.T, d *Deck, h *hiring, roleID string) (args []string, token string) {
	t.Helper()
	d.lastHire = time.Time{}
	opened := len(h.windows)
	if status, message := d.hireWith(hireSpec{roleID: roleID, fromSetup: true, extra: &setupHire{Role: roleID}}); status != http.StatusNoContent {
		t.Fatalf("Setup's hire of %s: %d %s", roleID, status, message)
	}
	if len(h.windows) != opened+1 {
		t.Fatalf("Setup's hire of %s opened %d windows", roleID, len(h.windows)-opened)
	}
	command, token := hiredWith(t, h.windows[opened])
	if slices.Contains(command, "--hire-id") || after(command, "--role") != roleID {
		t.Fatalf("Setup's hire of %s runs %q, want a plain hire of the role", roleID, command)
	}
	return command[2:], token
}

// heldAnywhere is whether secret is in any file under folder.
func heldAnywhere(t *testing.T, folder, secret string) bool {
	t.Helper()
	found := false
	_ = filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), secret) {
				found = true
			}
		}
		return nil
	})
	return found
}

// The GM hired the plain way gets a new token, kept for its launcher alone: not on its command line, not in a file,
// asked for once, with no id. A plain hire of any other role has none to give, and an engine hire's start is never
// given to a launcher that names no id.
func TestAPlainGMHireKeepsItsNewTokenForItsLauncherAlone(t *testing.T) {
	d, h := plainGMDeck(t)
	args, token := plainRoleHire(t, d, h, gmRole)
	d.mu.Lock()
	gmToken, pending := d.gmToken, d.hires[token]
	d.mu.Unlock()
	if gmToken == "" || !d.isGMTokenLocked(gmToken) {
		t.Fatal("the plain GM's hire made no GM token")
	}
	if pending == nil || !pending.plainGM || pending.env == nil || len(pending.env.Set) != 1 || pending.env.Set[gmTokenEnvironment] != gmToken ||
		len(pending.env.Unset) != 0 || pending.env.Settings != "" || pending.hireID != "" {
		t.Fatalf("the plain GM's start isn't its token alone (plainGM %v, env given %v)", pending != nil && pending.plainGM, pending != nil && pending.env != nil)
	}
	if slices.ContainsFunc(h.windows[len(h.windows)-1], func(arg string) bool { return strings.Contains(arg, gmToken) }) {
		t.Error("the GM's token is on its window's command line")
	}
	if heldAnywhere(t, supportDir, gmToken) || heldAnywhere(t, homeDir, gmToken) {
		t.Error("a file holds the GM's token (SK2: none may)")
	}
	run, err := parseRun(args)
	if err != nil || run.role != gmRole || run.hire != token || run.hireID != "" {
		t.Fatalf("the launcher reads %q as %+v, %v", args, run, err)
	}

	// The app's route, through the launcher's own client: with an id, refused; with none, its token, once.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { d.startEnvRoute(w, r.URL.Query()) }))
	defer server.Close()
	ask := func(token, id string) (hireStartEnv, error) {
		return fetchHireStartEnv(server.Client(), server.URL, "fake-hook-key", token, id)
	}
	if _, err := ask(token, "h1"); err == nil {
		t.Error("the plain GM's start was given to a launcher naming a hire's id")
	}
	got, err := ask(token, "")
	if err != nil || len(got.Set) != 1 || got.Set[gmTokenEnvironment] != gmToken || got.Settings != "" {
		t.Fatalf("the plain GM's launcher wasn't given its token alone: %v (given %d variables)", err, len(got.Set))
	}
	if _, err := ask(token, ""); err == nil {
		t.Error("the plain GM's start was given twice")
	}

	// Any other role's plain hire: nothing to give.
	_, janitor := plainRoleHire(t, d, h, "janitor")
	d.mu.Lock()
	other := d.hires[janitor]
	d.mu.Unlock()
	if other == nil || other.plainGM || other.env != nil {
		t.Errorf("a plain hire of another role keeps a start: %+v", other)
	}
	if _, err := ask(janitor, ""); err == nil {
		t.Error("a plain hire of another role was given a start")
	}
	// Refused as no start of its own, not as one already used.
	if env, status := d.takeStartEnv(janitor, ""); env != nil || status != http.StatusForbidden {
		t.Errorf("a plain hire of another role, asked for its start: %d, want %d", status, http.StatusForbidden)
	}
	// An engine hire's start, asked for with no id.
	engine := strings.Repeat("C", 32)
	d.mu.Lock()
	d.hires[engine] = &pendingHire{at: time.Now(), hireID: "abc123def456", env: &hireStartEnv{Set: map[string]string{gmTokenEnvironment: "fake-engine-token"}}}
	d.mu.Unlock()
	if _, err := ask(engine, ""); err == nil {
		t.Error("an engine hire's start was given to a launcher naming no id")
	}
}

// isGMTokenLocked is isGMToken with the lock taken.
func (d *Deck) isGMTokenLocked(token string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.isGMToken(token)
}

// A plain GM whose window didn't open: its token stops working and its start goes, so nothing can take it.
func TestAPlainGMHireWhoseWindowDidntOpenTakesItsTokenAway(t *testing.T) {
	d, h := plainGMDeck(t)
	h.fail = errors.New("no such terminal")
	if status, _ := d.hireWith(hireSpec{roleID: gmRole, fromSetup: true, extra: &setupHire{Role: gmRole}}); status != http.StatusInternalServerError {
		t.Fatalf("a GM hire whose window didn't open: %d", status)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.gmToken != "" {
		t.Error("the GM's token still works after its window didn't open")
	}
	if len(d.hires) != 0 {
		t.Errorf("%d hire starts kept after the window didn't open", len(d.hires))
	}
	if !d.lastHire.IsZero() {
		t.Error("the next hire must wait, though nobody was hired")
	}
}

// The launcher's environment for a plain hire: the GM's token on the GM alone, replacing any it inherited; any other
// plain hire has whatever GM token it inherited taken out. Names compared as Windows does, and as other systems do.
func TestThePlainGMsTokenIsSetOnTheGMAloneAndTakenFromAnyOtherHire(t *testing.T) {
	for _, caseless := range []bool{false, true} {
		inherited := gmTokenEnvironment
		if caseless {
			inherited = "Smolcorp_Gm_Token"
		}
		base := []string{"PATH=/bin", inherited + "=fake-inherited-token", "SMOLCORP_SESSION=s1"}
		for _, c := range []struct {
			row, token string
		}{{"the GM", "fake-gm-token"}, {"another hire", ""}} {
			env := plainGMEnv(base, c.token, caseless)
			var found []string
			for _, entry := range env {
				if name, value, _ := strings.Cut(entry, "="); strings.EqualFold(name, gmTokenEnvironment) {
					found = append(found, name+"="+value)
				}
			}
			switch {
			case c.token != "" && !slices.Equal(found, []string{gmTokenEnvironment + "=" + c.token}):
				t.Errorf("%s (caseless %v): %d GM token variables, want its own alone", c.row, caseless, len(found))
			case c.token == "" && caseless && len(found) != 0:
				t.Errorf("%s (caseless %v): a GM token was left on it", c.row, caseless)
			case c.token == "" && !caseless && slices.Contains(found, gmTokenEnvironment+"=fake-inherited-token"):
				t.Errorf("%s: the inherited GM token was left on it", c.row)
			}
			if _, path := environ(env, "PATH", false); !path {
				t.Errorf("%s (caseless %v): the rest of the environment went", c.row, caseless)
			}
		}
	}
}

// standInClaudeVariable makes this test program a stand-in Claude Code (TestStandInClaudeForTheGMToken): it writes,
// in the file the variable names, whether it was started with a GM token (never the token) and how `smolcorp gm team`
// ended, run in its own environment as a GM's Claude Code runs it.
const standInClaudeVariable = "SMOLCORP_STANDIN_CLAUDE_VERDICT"

func TestStandInClaudeForTheGMToken(t *testing.T) {
	verdict := os.Getenv(standInClaudeVariable)
	if verdict == "" {
		return
	}
	token := "unset"
	if os.Getenv(gmTokenEnvironment) != "" {
		token = "set"
	}
	var out, errOut strings.Builder
	code := gm.Run([]string{"team"}, &out, &errOut)
	why, _, _ := strings.Cut(errOut.String(), "\n")
	_ = os.WriteFile(verdict, []byte(fmt.Sprintf("token=%s team=%d\n%s", token, code, why)), 0o600)
	os.Exit(0)
}

// standInClaude puts a stand-in claude in a folder of its own: this test program as TestStandInClaudeForTheGMToken.
// On Windows it is a .cmd script, as npm installs Claude Code, so it starts through cmd.exe as the user's did.
func standInClaude(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	if runtime.GOOS == "windows" {
		writeFile(t, filepath.Join(bin, "claude.cmd"), "@echo off\r\nset "+launcherStandInVariable+"=\r\n"+
			"\"%SMOLCORP_TEST_BINARY%\" -test.run=TestStandInClaudeForTheGMToken -- %*\r\n")
		return bin
	}
	writeFile(t, filepath.Join(bin, "claude"), "#!/bin/sh\nunset "+launcherStandInVariable+"\n"+
		"exec \"$SMOLCORP_TEST_BINARY\" -test.run=TestStandInClaudeForTheGMToken -- \"$@\"\n")
	if err := os.Chmod(filepath.Join(bin, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// The whole way, every OS (the Windows CI job runs it as the user's machine had it: the launcher, then cmd.exe, then
// claude): Setup hires the GM the plain way; the real launcher, in a process of its own whose window inherited
// another GM token, asks the app; the stand-in claude it starts has the GM's token and `smolcorp gm team` works
// against the app. A hire of another role, started the same way, has no GM token and can't see the team. A start
// asked for again starts nothing.
func TestAGMHiredFromSetupThePlainWayStartsWithItsTokenAndSeesTheTeam(t *testing.T) {
	d, h := plainGMDeck(t)
	gmArgs, _ := plainRoleHire(t, d, h, gmRole)
	hireArgs, _ := plainRoleHire(t, d, h, "janitor")

	// The app, on a port of its own, and the office's files the launcher reads as a hire's reads the app's.
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	port := server.URL[strings.LastIndex(server.URL, ":")+1:]
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	swap(t, &Port, n)
	if key, err := os.ReadFile(hookKeyFile); err != nil || len(strings.TrimSpace(string(key))) == 0 {
		t.Fatalf("the app has no hook key file: %v", err)
	}
	config := filepath.Join(t.TempDir(), "config.json")
	writeFile(t, config, string(must(json.Marshal(map[string]any{"roles": d.config.Roles}))))
	office := string(must(json.Marshal(map[string]string{"config": config, "hookKey": hookKeyFile})))
	bin, folder := standInClaude(t), t.TempDir()

	start := func(args []string) (verdict string, output string, ran bool) {
		t.Helper()
		file := filepath.Join(t.TempDir(), "verdict")
		cmd := exec.Command(os.Args[0], "-test.run=^$")
		cmd.Env = append(os.Environ(), launcherStandInVariable+"="+string(must(json.Marshal(args))), launcherStandInOfficeVariable+"="+office,
			"SMOLCORP_PORT="+port, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "SMOLCORP_TEST_BINARY="+os.Args[0],
			standInClaudeVariable+"="+file, "SMOLCORP_HIRE_KEY=",
			// What the window might have inherited: never the GM's.
			gmTokenEnvironment+"=fake-inherited-gm-token")
		cmd.Dir = folder
		out, _ := cmd.CombinedOutput()
		got, err := os.ReadFile(file)
		return strings.TrimSpace(string(got)), string(out), err == nil
	}

	verdict, output, ran := start(gmArgs)
	if !ran || !strings.HasPrefix(verdict, "token=set team=0") {
		t.Errorf("the GM hired from Setup: %q (ran %v), want its token set and the team seen\n%s", verdict, ran, output)
	}
	verdict, output, ran = start(hireArgs)
	if !ran || !strings.HasPrefix(verdict, "token=unset team=") || strings.HasPrefix(verdict, "token=unset team=0") {
		t.Errorf("a hire of another role: %q (ran %v), want no GM token and no team\n%s", verdict, ran, output)
	}
	// The GM's start, asked for again (its window reopened, say): nothing starts, and the window says why.
	verdict, output, ran = start(gmArgs)
	if ran || !strings.Contains(output, "couldn't get the General Manager's token, so nothing was started") {
		t.Errorf("the GM's start asked for again ran claude (%q) or didn't say why\n%s", verdict, output)
	}
}
