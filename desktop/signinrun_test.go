package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// ✨ Sign in from Smolcorp (1.26 S2): the login runner, on stand-in CLIs of the
// tests' own that print canned login flows in a pseudo-terminal. No real claude,
// codex or gemini is ever run, and no credential is read.

// A token-shaped string, obviously fake: it must never pass the filter, nor reach an event or the log.
var loginFakeToken = fakeAnthropicToken("oat", "FAKEFAKEFAKEFAKEFAKEFAKE0123456789")

// An AWS access key ID and secret key on one line, obviously fake: AWS's documentation example with
// FAKE in it. It's put together only when the tests run, so neither the public check
// (scripts/check-public.sh) nor GitHub's push protection takes this file for a real key.
var loginFakeAWSKeys = "AKIA" + "FAKEFAKEFAKEFAKE" + " " + "wJalrXUtnFEMI" + "K7MDENGbPxRfiCY" + "FAKEFAKEKEY1"

// Links shaped as the agents print them (the PKCE values are made up).
const (
	pkce          = "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_AbCdE"
	claudeLinkNow = "https://claude.com/cai/oauth/authorize?code=true&client_id=9d1c250a-e61b-44d9-88ed-5944d1962f5e&response_type=code" +
		"&redirect_uri=https%3A%2F%2Fplatform.claude.com%2Foauth%2Fcode%2Fcallback&scope=org%3Acreate_api_key+user%3Aprofile+user%3Ainference" +
		"&code_challenge=" + pkce + "&code_challenge_method=S256&state=" + pkce
	codexLink = "https://auth.openai.com/oauth/authorize?response_type=code&client_id=app_EMoamEEZ73f0CkXaXp7hrann" +
		"&redirect_uri=http%3A%2F%2Flocalhost%3A1455%2Fauth%2Fcallback&scope=openid+profile+email+offline_access" +
		"&code_challenge=" + pkce + "&code_challenge_method=S256&id_token_add_organizations=true&codex_cli_simplified_flow=true" +
		"&state=" + pkce + "&originator=codex_cli_rs"
	codexDeviceLink = "https://auth.openai.com/codex/device"
)

// loginPlace makes stand-ins for the agents named (name: its bash body) and makes them the only
// programs found: no real program is ever run. It gives the stand-ins' folder.
func loginPlace(t *testing.T, bodies map[string]string) string {
	t.Helper()
	isolate(t)
	dir := t.TempDir()
	for name, body := range bodies {
		fakeCLI(t, dir, name, body)
	}
	saved, savedLimit := loginFind, loginTimeLimit
	t.Cleanup(func() { loginFind, loginTimeLimit = saved, savedLimit })
	loginFind = func(name string) string {
		if _, ok := bodies[name]; !ok {
			return ""
		}
		real, _ := filepath.EvalSymlinks(filepath.Join(dir, name))
		return real
	}
	return dir
}

// loginDeck is a test deck whose login, if one is left running, is ended at the test's end.
func loginDeck(t *testing.T) *Deck {
	t.Helper()
	d := newTestDeck(t)
	t.Cleanup(func() {
		if run := d.logins.current(""); run != nil {
			run.end(loginCancelled)
			<-run.done
		}
	})
	return d
}

// page asks as setup's page does.
func page(t *testing.T, d *Deck, method, target, body string) (int, string) {
	t.Helper()
	got := serve(t, d, call{method: method, target: target, host: "localhost:7710", from: thisComputer, body: body,
		headers: map[string]string{setupHeader: "1"}})
	return got.Code, got.Body.String()
}

func startLogin(t *testing.T, d *Deck, agent, way string) string {
	t.Helper()
	status, body := page(t, d, "POST", "/setup/accounts/login", fmt.Sprintf(`{"agent":%q,"way":%q}`, agent, way))
	var started struct{ ID string }
	if status != http.StatusOK || json.Unmarshal([]byte(body), &started) != nil || started.ID == "" {
		t.Fatalf("start %s %s: %d %s", agent, way, status, body)
	}
	return started.ID
}

func loginViewOf(t *testing.T, d *Deck, id string) loginView {
	t.Helper()
	status, body := page(t, d, "GET", "/setup/accounts/login?id="+id, "")
	var view loginView
	if status != http.StatusOK || json.Unmarshal([]byte(body), &view) != nil {
		t.Fatalf("state: %d %s", status, body)
	}
	return view
}

// waitForLogin waits until the login's view passes ok, for a few seconds at most.
func waitForLogin(t *testing.T, d *Deck, id, what string, ok func(loginView) bool) loginView {
	t.Helper()
	var view loginView
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		if view = loginViewOf(t, d, id); ok(view) {
			return view
		}
	}
	t.Fatalf("never %s: %+v", what, view)
	return view
}

func ended(v loginView) bool { return v.State != loginRunning }

// pidGone is whether a process has gone.
func pidGone(pid int) bool {
	process, err := os.FindProcess(pid)
	return err != nil || process.Signal(syscall.Signal(0)) != nil
}

func readPid(t *testing.T, file string) int {
	t.Helper()
	var pid int
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline) && pid == 0; time.Sleep(25 * time.Millisecond) {
		text, _ := os.ReadFile(file)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(text)))
	}
	if pid == 0 {
		t.Fatal("the stand-in never started its child")
	}
	return pid
}

// The Claude stand-in prints the link and asks for the code, as `claude auth login` does; it notes
// what it was given in got, and says it signed in only for the right code.
func claudeStandIn(got string) string {
	return `echo "args: $*" > ` + got + `.args
echo 'Opening browser to sign in…'
echo "If the browser didn't open, visit: ` + claudeLinkNow + `"
printf 'Paste code here if prompted > '
read -r code
echo "$code" >> ` + got + `
if [ "$code" = "GOODCODE#` + pkce + `" ]; then echo 'Login successful.'; exit 0; fi
echo 'Login failed: Invalid code.'; exit 1`
}

func TestEachStandInLoginFlow(t *testing.T) {
	dir := t.TempDir()
	got := filepath.Join(dir, "claude-got")
	ready := filepath.Join(dir, "codex-go")
	loginPlace(t, map[string]string{
		"claude": claudeStandIn(got),
		// Device auth, in colour, as `codex login --device-auth` prints it; done when the test says so.
		"codex": `echo "args: $*" > ` + dir + `/codex-args
if [ "$1 $2" = "login --device-auth" ]; then
  printf '\033[1mFollow these steps to sign in with ChatGPT using device code authorization:\033[0m\n\n'
  printf '1. Open this link in your browser and sign in to your account\n   \033[94m` + codexDeviceLink + `\033[0m\n\n'
  printf '2. Enter this one-time code \033[90m(expires in 15 minutes)\033[0m\n   \033[94mABCD-EFGH2\033[0m\n'
else
  echo 'Starting local login server on http://localhost:1455.'
  echo 'If your browser did not open, navigate to this URL to authenticate:'
  echo
  echo '` + codexLink + `'
fi
while [ ! -f ` + ready + ` ]; do sleep 0.05; done
if [ "$(cat ` + ready + `)" = ok ]; then echo 'Successfully logged in'; exit 0; fi
echo 'Error logging in: the sign-in was refused'; exit 1`,
	})
	d := loginDeck(t)

	// Claude: the link, then the code typed in the page, then success.
	id := startLogin(t, d, "claude", "")
	view := waitForLogin(t, d, id, "showed Claude's link", func(v loginView) bool { return v.URL != "" })
	if view.URL != claudeLinkNow || view.Code != "" || view.State != loginRunning || view.Agent != "claude" || view.Way != "claudeai" {
		t.Fatalf("claude: %+v", view)
	}
	if status, body := page(t, d, "POST", "/setup/accounts/login/code", `{"id":"`+id+`","code":"GOODCODE#`+pkce+`"}`); status != http.StatusNoContent {
		t.Fatalf("the code: %d %s", status, body)
	}
	view = waitForLogin(t, d, id, "ended", ended)
	if view.State != loginSucceeded || view.URL != "" || view.Code != "" {
		t.Errorf("claude, signed in: %+v", view)
	}
	if args, _ := os.ReadFile(got + ".args"); string(args) != "args: auth login --claudeai\n" {
		t.Errorf("claude's arguments: %q", args)
	}

	// Claude, a wrong code: failed.
	id = startLogin(t, d, "claude", "console")
	waitForLogin(t, d, id, "showed the link", func(v loginView) bool { return v.URL != "" })
	page(t, d, "POST", "/setup/accounts/login/code", `{"id":"`+id+`","code":"WRONG"}`)
	if view = waitForLogin(t, d, id, "ended", ended); view.State != loginFailed {
		t.Errorf("claude, a wrong code: %+v", view)
	}
	if args, _ := os.ReadFile(got + ".args"); string(args) != "args: auth login --console\n" {
		t.Errorf("claude's arguments: %q", args)
	}

	// Codex by device code: the link and the code, in colour, come through plain; then success.
	id = startLogin(t, d, "codex", "device")
	view = waitForLogin(t, d, id, "showed the device code", func(v loginView) bool { return v.URL != "" && v.Code != "" })
	if view.URL != codexDeviceLink || view.Code != "ABCD-EFGH2" || view.State != loginRunning {
		t.Fatalf("codex device: %+v", view)
	}
	os.WriteFile(ready, []byte("ok"), 0o600)
	if view = waitForLogin(t, d, id, "ended", ended); view.State != loginSucceeded || view.Code != "" {
		t.Errorf("codex device, signed in: %+v", view)
	}
	os.Remove(ready)

	// Codex in the browser: its link (its localhost line isn't https, so not that one); then a failure.
	id = startLogin(t, d, "codex", "")
	if view = waitForLogin(t, d, id, "showed the link", func(v loginView) bool { return v.URL != "" }); view.URL != codexLink || view.Code != "" {
		t.Fatalf("codex: %+v", view)
	}
	os.WriteFile(ready, []byte("no"), 0o600)
	if view = waitForLogin(t, d, id, "ended", ended); view.State != loginFailed {
		t.Errorf("codex, refused: %+v", view)
	}
	if args, _ := os.ReadFile(filepath.Join(dir, "codex-args")); string(args) != "args: login\n" {
		t.Errorf("codex's arguments: %q", args)
	}
}

func TestALoginThatSaysSuccessButExitsBadlyFailed(t *testing.T) {
	loginPlace(t, map[string]string{"codex": `echo 'Successfully logged in'; exit 3`})
	d := loginDeck(t)
	id := startLogin(t, d, "codex", "browser")
	if view := waitForLogin(t, d, id, "ended", ended); view.State != loginFailed {
		t.Errorf("%+v", view)
	}
}

// On a Mac a login starts through smolcorp-spawn (SANDBOX-PLAN §3.5, ST1; RT126ST1 B1), so a CLI a hire replaced
// holds none of Smolcorp's permissions. In Smolcorp.app's layout, a stand-in smolcorp-spawn notes what it was given
// and becomes the stand-in codex in the login's terminal; with no smolcorp-spawn there, nothing starts and the page
// is told why.
func TestOnAMacALoginStartsThroughSmolcorpSpawnAndNeverWithoutIt(t *testing.T) {
	dir := t.TempDir()
	loginPlace(t, map[string]string{"codex": `echo "argv0=$0 args=$*" > ` + dir + `/ran
[ -t 0 ] && echo 'Successfully logged in'`})
	program := loginFind("codex")
	bin := filepath.Join(t.TempDir(), "Smolcorp.app", "Contents", "Resources", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	where := disclaim.PlaceBeside(filepath.Join(bin, "smolcorp"))
	if !where.InApp {
		t.Fatalf("not an app's layout: %+v", where)
	}
	t.Cleanup(disclaim.StandIn(true, where))
	// One line per start, written at once: another test's leftover git may come through here too.
	fakeCLI(t, bin, disclaim.SpawnName, `line="$(printf '%s|' "$@")"; printf '%s\n' "$line" >> `+dir+`/spawned
exec -a "$2" "$1" "${@:3}"`)
	d := loginDeck(t)

	id := startLogin(t, d, "codex", "browser")
	if view := waitForLogin(t, d, id, "ended", ended); view.State != loginSucceeded {
		t.Errorf("through smolcorp-spawn, in its terminal: %+v", view)
	}
	spawned, _ := os.ReadFile(filepath.Join(dir, "spawned"))
	if want := program + "|" + program + "|login|"; !slices.Contains(strings.Split(string(spawned), "\n"), want) {
		t.Errorf("smolcorp-spawn wasn't given %q: %q", want, spawned)
	}
	if ran, _ := os.ReadFile(filepath.Join(dir, "ran")); string(ran) != "argv0="+program+" args=login\n" {
		t.Errorf("the stand-in ran as %q", ran)
	}

	// No smolcorp-spawn in the app: nothing starts.
	os.Remove(filepath.Join(dir, "ran"))
	if err := os.Remove(where.Spawn); err != nil {
		t.Fatal(err)
	}
	status, body := page(t, d, "POST", "/setup/accounts/login", `{"agent":"codex","way":"browser"}`)
	if status != http.StatusInternalServerError || !strings.Contains(body, "Smolcorp didn't start codex: its smolcorp-spawn is missing") {
		t.Errorf("with no smolcorp-spawn: %d %s", status, body)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "ran")); !os.IsNotExist(err) {
		t.Errorf("the stand-in ran with no smolcorp-spawn: %v", err)
	}
}

func TestGeminiAndOthersHaveNoLoginAndNothingIsRun(t *testing.T) {
	asked := 0
	loginPlace(t, map[string]string{"gemini": "exit 0"})
	loginFind = func(string) string { asked++; return "" }
	d := loginDeck(t)
	for _, c := range []struct{ body, says string }{
		{`{"agent":"gemini"}`, "no sign-in command"},
		{`{"agent":"claude","way":"setup-token"}`, "no sign-in for that"},
		{`{"agent":"codex","way":"api-key"}`, "no sign-in for that"},
		{`{"agent":"sh"}`, "no sign-in for that"},
		{`{"agent":"claude"}`, "isn't installed"},
	} {
		if status, body := page(t, d, "POST", "/setup/accounts/login", c.body); status == http.StatusOK || !strings.Contains(body, c.says) {
			t.Errorf("%s: %d %s", c.body, status, body)
		}
	}
	if asked != 1 {
		t.Errorf("looked for a program %d times", asked)
	}
	// No way on the list prints or reads a token.
	for _, w := range loginWays {
		joined := strings.Join(w.args, " ")
		if strings.Contains(joined, "token") || strings.Contains(joined, "key") || w.agent == "gemini" {
			t.Errorf("%s %s: %s", w.agent, w.way, joined)
		}
	}
}

// A stand-in that prints tokens, keys, escapes and look-alike links: none of it passes the
// filter, nor reaches an event, the log or a file of the deck's.
func TestOnlyTheFilteredLinkCodeAndOutcomeLeaveALogin(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	lines := []string{
		loginFakeToken,
		"access_token: " + loginFakeToken,
		`{"accessToken":"` + loginFakeToken + `","refreshToken":"` + fakeAnthropicToken("ort", "FAKEFAKE") + `"}`,
		"OPENAI_API_KEY=sk-proj-FAKEFAKEFAKE",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJGQUtFRkFLRSJ9.FAKEFAKEsignature",
		loginFakeAWSKeys,
		"https://claude.ai.evil.example/oauth/authorize?state=" + pkce,
		"https://evil.example/claude.ai/oauth/authorize",
		"https://claude.ai@evil.example/oauth/authorize",
		"https://claude.ai:8443/oauth/authorize",
		"http://claude.ai/oauth/authorize",
		"https://clаude.ai/oauth/authorize",
		"https://CLAUDE.AI/oauth/authorize",
		"https://claude.ai/oauth/authorize/../../steal",
		"https://claude.ai/oauth/authorize?access_token=" + loginFakeToken,
		"https://claude.ai/oauth/authorize?state=" + loginFakeToken,
		"https://claude.ai/oauth/authorize?code=true#" + loginFakeToken,
		"https://claude.ai/oauth/authorize?client_id=FAKEFAKE%0Aevil",
		"https://claude.ai/oauth/authorize\x1b[0m.evil.example/x",
		"\x1b]8;;https://evil.example/\x1b\\click here\x1b]8;;\x1b\\",
		"\x1b]0;" + loginFakeToken + "\x07",
		"ABCD-EFGH2",
		"Paste code here if prompted > " + loginFakeToken,
		"Login successful" + loginFakeToken,
		// Last, the real link: the first link that passes is kept, so it shows only if all the rest were dropped.
		"If the browser didn't open, visit: \x1b[4m" + claudeLinkNow + "\x1b[0m",
	}
	folder := t.TempDir()
	printed, ready := filepath.Join(folder, "printed"), filepath.Join(folder, "ready")
	if err := os.WriteFile(printed, []byte(strings.Join(lines, "\r\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loginPlace(t, map[string]string{"claude": "cat " + printed + "\necho " + loginFakeToken + " >&2\n" +
		"while [ ! -f " + ready + " ]; do sleep 0.05; done\nexit 1"})
	d := loginDeck(t)
	events := filepath.Join(supportDir, "events.jsonl")

	id := startLogin(t, d, "claude", "")
	running := waitForLogin(t, d, id, "showed a link", func(v loginView) bool { return v.URL != "" })
	if running.URL != claudeLinkNow || running.Code != "" || running.State != loginRunning {
		t.Errorf("while it runs: %+v", running)
	}
	_, whileRunning := page(t, d, "GET", "/setup/accounts/login?id="+id, "")
	os.WriteFile(ready, nil, 0o600)
	// It said "Login successful" only with more after it, and ended badly: failed.
	if view := waitForLogin(t, d, id, "ended", ended); view.State != loginFailed || view.URL != "" || view.Code != "" {
		t.Errorf("%+v", view)
	}
	_, answer := page(t, d, "GET", "/setup/accounts/login?id="+id, "")
	every := []string{whileRunning, answer, logged.String()}
	data, _ := os.ReadFile(events)
	every = append(every, string(data))
	for _, folder := range []string{supportDir, homeDir} {
		filepath.WalkDir(folder, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				text, _ := os.ReadFile(path)
				every = append(every, string(text))
			}
			return nil
		})
	}
	for i, text := range every {
		for _, mark := range []string{"FAKEFAKE", "sk-ant", "sk-proj", "evil", "eyJ"} {
			if strings.Contains(text, mark) {
				t.Errorf("%q reached output %d: %.200s", mark, i, text)
			}
		}
	}
}

func TestTheLinkFilter(t *testing.T) {
	claude, codex := loginWayOf("claude", ""), loginWayOf("codex", "device")
	for _, c := range []struct {
		way  *loginWay
		link string
		ok   bool
	}{
		{claude, claudeLinkNow, true},
		{claude, strings.Replace(claudeLinkNow, "https://claude.com/cai", "https://platform.claude.com", 1), true},
		{claude, strings.Replace(claudeLinkNow, "https://claude.com/cai", "https://claude.ai", 1), true},
		{claude, claudeLinkNow + ".", true}, // a sentence's full stop is left out
		{codex, codexLink, true},
		{codex, codexDeviceLink, true},
		{claude, codexLink, false}, // another agent's host
		{codex, claudeLinkNow, false},
		{claude, strings.Replace(claudeLinkNow, "claude.com", "claude.com.evil.example", 1), false},
		{claude, strings.Replace(claudeLinkNow, "claude.com", "xn--claude-fake.com", 1), false},
		{claude, strings.Replace(claudeLinkNow, "https://", "https://me@", 1), false},
		{claude, strings.Replace(claudeLinkNow, "claude.com", "claude.com:443", 1), false},
		{claude, strings.Replace(claudeLinkNow, "/cai/oauth/authorize", "/cai/oauth/authorize/x", 1), false},
		{claude, strings.Replace(claudeLinkNow, "/cai/", "/cai%2F", 1), false},
		{claude, strings.Replace(claudeLinkNow, "https://", "http://", 1), false},
		{claude, claudeLinkNow + "#" + pkce, false},
		{claude, claudeLinkNow + "&token=abc", false},
		{claude, claudeLinkNow + "&api_key=abc", false},
		{claude, claudeLinkNow + "&x=" + loginFakeToken, false},
		{claude, claudeLinkNow + "&x=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJGQUtF.sig", false},
		{claude, strings.Replace(claudeLinkNow, "state="+pkce, "state="+loginFakeToken, 1), false},
		{claude, strings.Replace(claudeLinkNow, "response_type=code", "response_type=token", 1), false},
		{claude, strings.Replace(claudeLinkNow, "platform.claude.com%2Foauth", "evil.example%3A1%40x%2Foauth%3Fa%3D", 1), false},
		{claude, claudeLinkNow + "&x=%1b%5b31m", false},
		{claude, "https://claude.com/cai/oauth/authorize?" + strings.Repeat("a", loginURLMost), false},
		{claude, "https://claude.com/cai/oauth/authorize?state=" + pkce + "%00", false},
	} {
		if got, ok := c.way.signInURL(c.link); ok != c.ok || (ok && got != strings.TrimRight(c.link, ".")) {
			t.Errorf("%s …%s: %v %.60s", c.way.agent, c.link[max(0, len(c.link)-60):], ok, got)
		}
	}
	// Any PKCE values pass: the second net isn't run over them (one in tens would read as an AWS key).
	random := rand.New(rand.NewSource(7))
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	value := func() string {
		b := make([]byte, 43)
		for i := range b {
			b[i] = letters[random.Intn(len(letters))]
		}
		return string(b)
	}
	for range 2000 {
		link := strings.ReplaceAll(claudeLinkNow, pkce, value())
		if _, ok := claude.signInURL(link); !ok {
			t.Fatalf("dropped: %s", link)
		}
	}
	// A device code is a whole line, and only for a way that prints one.
	for line, want := range map[string]bool{"ABCD-EFGH2": true, "ABCDE-FGHIJ": true, "abcd-efgh": false, "ABCD-EFGH2 x": false,
		"code ABCD-EFGH2": false, "ABCDEFGH": false, "ABC-DEFG": false} {
		run := &loginRun{way: codex}
		run.look([]byte("\x1b[94m" + line + "\x1b[0m"))
		if (run.code != "") != want {
			t.Errorf("%q: %q", line, run.code)
		}
	}
	run := &loginRun{way: claude}
	run.look([]byte("ABCD-EFGH2"))
	if run.code != "" {
		t.Errorf("claude took a device code: %q", run.code)
	}
}

// A code goes to its own login's terminal, while it runs, and to no other.
func TestACodeGoesOnlyToItsOwnLogin(t *testing.T) {
	dir := t.TempDir()
	got := filepath.Join(dir, "got")
	loginPlace(t, map[string]string{
		"claude": claudeStandIn(got),
		// Codex takes no code: if one reached it, it would note it.
		"codex": `echo '` + codexLink + `'; while read -r -t 5 line; do echo "$line" >> ` + dir + `/codex-got; done; exit 1`,
	})
	d := loginDeck(t)

	first := startLogin(t, d, "claude", "")
	waitForLogin(t, d, first, "showed the link", func(v loginView) bool { return v.URL != "" })
	// Not while another login runs.
	if status, _ := page(t, d, "POST", "/setup/accounts/login", `{"agent":"codex"}`); status != http.StatusConflict {
		t.Errorf("a second login: %d", status)
	}
	// Not to a login that isn't this one, nor without an id, nor a code with a control character or a space in it.
	for _, body := range []string{`{"id":"0123","code":"STRAY"}`, `{"code":"STRAY"}`, `{"id":"` + first + `","code":"A\u001b[2J"}`,
		`{"id":"` + first + `","code":"A\nB"}`, `{"id":"` + first + `","code":"A B"}`, `{"id":"` + first + `","code":""}`} {
		if status, _ := page(t, d, "POST", "/setup/accounts/login/code", body); status == http.StatusNoContent {
			t.Errorf("%s: taken", body)
		}
	}
	if status, _ := page(t, d, "POST", "/setup/accounts/login/code", `{"id":"`+first+`","code":"FIRST"}`); status != http.StatusNoContent {
		t.Fatalf("the code: %d", status)
	}
	waitForLogin(t, d, first, "ended", ended)
	if text, _ := os.ReadFile(got); string(text) != "FIRST\n" {
		t.Errorf("the first login read %q", text)
	}
	// Once it has ended, its code goes nowhere.
	if status, _ := page(t, d, "POST", "/setup/accounts/login/code", `{"id":"`+first+`","code":"LATE"}`); status == http.StatusNoContent {
		t.Error("a code after the end was taken")
	}

	second := startLogin(t, d, "codex", "")
	waitForLogin(t, d, second, "showed the link", func(v loginView) bool { return v.URL != "" })
	// The first login's id reaches nothing now; and codex takes no code.
	if status, _ := page(t, d, "POST", "/setup/accounts/login/code", `{"id":"`+first+`","code":"STRAY"}`); status != http.StatusNotFound {
		t.Errorf("the first login's id: %d", status)
	}
	if status, _ := page(t, d, "POST", "/setup/accounts/login/code", `{"id":"`+second+`","code":"STRAY"}`); status != http.StatusConflict {
		t.Errorf("a code for codex: %d", status)
	}
	page(t, d, "POST", "/setup/accounts/login/cancel", `{"id":"`+second+`"}`)
	if text, err := os.ReadFile(filepath.Join(dir, "codex-got")); err == nil {
		t.Errorf("codex read %q", text)
	}
	if text, _ := os.ReadFile(got); string(text) != "FIRST\n" {
		t.Errorf("the first login read %q", text)
	}
}

func TestALoginIsEndedWithAllItStartedOnCancelAndOnTimeout(t *testing.T) {
	dir := t.TempDir()
	loginPlace(t, map[string]string{
		"claude": `echo $$ > ` + dir + `/self; sleep 60 & echo $! > ` + dir + `/child; wait`,
	})
	d := loginDeck(t)

	for _, how := range []loginState{loginCancelled, loginTimedOut} {
		os.Remove(filepath.Join(dir, "self"))
		os.Remove(filepath.Join(dir, "child"))
		if how == loginTimedOut {
			loginTimeLimit = 500 * time.Millisecond
		}
		id := startLogin(t, d, "claude", "")
		self, child := readPid(t, filepath.Join(dir, "self")), readPid(t, filepath.Join(dir, "child"))
		if how == loginCancelled {
			if status, _ := page(t, d, "POST", "/setup/accounts/login/cancel", `{"id":"`+id+`"}`); status != http.StatusNoContent {
				t.Fatalf("cancel: %d", status)
			}
		}
		if view := waitForLogin(t, d, id, "ended", ended); view.State != how {
			t.Errorf("%s: %+v", how, view)
		}
		eventually(t, string(how)+": the stand-in stopped", func() bool { return pidGone(self) })
		eventually(t, string(how)+": what it started stopped", func() bool { return pidGone(child) })
		// Ending it again does nothing.
		if status, _ := page(t, d, "POST", "/setup/accounts/login/cancel", `{"id":"`+id+`"}`); status != http.StatusNoContent {
			t.Errorf("a second cancel: %d", status)
		}
		if view := loginViewOf(t, d, id); view.State != how {
			t.Errorf("after a second cancel: %+v", view)
		}
	}
}

func TestTheLoginRoutesAreThePagesAlone(t *testing.T) {
	paths := []routeRow{{"POST", "/setup/accounts/login", nil}, {"GET", "/setup/accounts/login", nil},
		{"POST", "/setup/accounts/login/code", nil}, {"POST", "/setup/accounts/login/cancel", nil}}
	for _, p := range paths {
		row := routeFor(p.method, p.path)
		if row == nil || !reflect.DeepEqual(row.callers, []string{"page"}) {
			t.Errorf("%s %s: %+v", p.method, p.path, row)
		}
		for _, who := range []caller{callerTool, callerGM, callerHire, callerNobody} {
			if routeAllows(who, p.method, p.path) {
				t.Errorf("%s may %s %s", who.name(), p.method, p.path)
			}
		}
	}

	started := 0
	loginPlace(t, map[string]string{"claude": "sleep 5"})
	find := loginFind
	loginFind = func(name string) string { started++; return find(name) }
	d := loginDeck(t)
	// The GM's powers (an MCP key's, a night job's) aren't the page's.
	for _, a := range []actor{actorGM, mcpActor("claude-desktop"), nightActor("nightly"), actorOf(callerHire), actorOf(callerTool)} {
		for _, p := range paths {
			if d.decide(a, action{route: &routeAsk{method: p.method, path: p.path}}).allowed() {
				t.Errorf("%s may %s %s", a, p.method, p.path)
			}
		}
	}
	d.mu.Lock()
	d.access.WindowKey = "fake-window-key"
	d.mu.Unlock()
	writeRegistryFile(registryPath(), []byte(`{"version":1,"hires":[{"id":"aaa111","name":"Rev","folder":"/w","role":"reviewer","state":"working"}]}`), 0o600)
	hireKey, err := newSendKey(registryPath(), "aaa111")
	if err != nil {
		t.Fatal(err)
	}
	start := `{"agent":"claude"}`
	setup := map[string]string{setupHeader: "1"}
	with := func(extra map[string]string) map[string]string {
		all := map[string]string{setupHeader: "1"}
		for k, v := range extra {
			all[k] = v
		}
		return all
	}
	for name, c := range map[string]call{
		"the setup key (a tool)": {method: "POST", target: "/setup/accounts/login", host: "localhost:7710", from: thisComputer, asTool: true, headers: setup, body: start},
		"the GM's token": {method: "POST", target: "/setup/accounts/login", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: with(map[string]string{gmTokenHeader: d.newGMToken()}), body: start},
		"a hire's key": {method: "POST", target: "/setup/accounts/login", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: with(map[string]string{hireKeyHeader: hireKey}), body: start},
		"the window key": {method: "POST", target: "/setup/accounts/login?t=fake-window-key", host: "localhost:7710", from: thisComputer,
			withoutSetupKey: true, headers: setup, body: start},
		"nobody":                   {method: "POST", target: "/setup/accounts/login", host: "localhost:7710", from: thisComputer, withoutSetupKey: true, headers: setup, body: start},
		"a phone":                  {method: "POST", target: "/setup/accounts/login", host: deckOnWiFi, from: aPhone, headers: setup, body: start},
		"the page, without header": {method: "POST", target: "/setup/accounts/login", host: "localhost:7710", from: thisComputer, body: start},
		"the office's way":         {method: "POST", target: "/accounts/login?t=fake-window-key", host: "localhost:7710", from: thisComputer, body: start},
		"a phone's way":            {method: "POST", target: "/api/accounts/login", host: deckOnWiFi, from: aPhone, body: start},
	} {
		if got := serve(t, d, c); got.Code == http.StatusOK || strings.Contains(got.Body.String(), `"id"`) {
			t.Errorf("%s: %d %s", name, got.Code, got.Body)
		}
	}
	if started != 0 {
		t.Fatalf("a login was started %d times", started)
	}
	// The page may.
	id := startLogin(t, d, "claude", "")
	for name, c := range map[string]call{
		"read, by the setup key": {method: "GET", target: "/setup/accounts/login?id=" + id, host: "localhost:7710", from: thisComputer, asTool: true},
		"read, by the GM": {method: "GET", target: "/setup/accounts/login?id=" + id, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{gmTokenHeader: d.newGMToken()}},
		"a code, by a hire": {method: "POST", target: "/setup/accounts/login/code", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: with(map[string]string{hireKeyHeader: hireKey}), body: `{"id":"` + id + `","code":"X"}`},
		"cancel, by the GM": {method: "POST", target: "/setup/accounts/login/cancel", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: with(map[string]string{gmTokenHeader: d.newGMToken()}), body: `{"id":"` + id + `"}`},
		"a path under it": {method: "GET", target: "/setup/accounts/login/other", host: "localhost:7710", from: thisComputer, asTool: true},
	} {
		if got := serve(t, d, c); got.Code < 400 {
			t.Errorf("%s: %d %s", name, got.Code, got.Body)
		}
	}
	if view := loginViewOf(t, d, id); view.State != loginRunning {
		t.Errorf("after the refusals: %+v", view)
	}
}

func TestALoginsEnvironmentIsCleaned(t *testing.T) {
	env := loginEnvironment([]string{"PATH=/bin", "ANTHROPIC_API_KEY=" + loginFakeToken, "OPENAI_API_KEY=x", "SMOLCORP_SESSION=abc", "HOME=/h"})
	joined := strings.Join(env, "\n")
	for _, gone := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "SMOLCORP_SESSION"} {
		if strings.Contains(joined, gone) {
			t.Errorf("%s is left: %v", gone, env)
		}
	}
	for _, kept := range []string{"PATH=/bin", "HOME=/h", "NO_COLOR=1"} {
		if !strings.Contains(joined, kept) {
			t.Errorf("%s isn't there: %v", kept, env)
		}
	}
	// A path as found, and its arguments from the list alone.
	program, args := loginCommand("/opt/claude", loginWayOf("claude", "").args)
	if program != "/opt/claude" || strings.Join(args, " ") != "auth login --claudeai" {
		t.Errorf("%s %v", program, args)
	}
}
