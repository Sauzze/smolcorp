// Command upgrade is CI's upgrade from a real 1.24 release to this build (UPGRADE1), on Linux and Windows:
//
//	go run ./scripts/upgrade -version v1.24.2 -archive <the release's download> -new <this build's smolcorp> -work <empty folder> [-seed strict]
//
// It checks the download against its SHA-256 in SHA256SUMS beside this file, unpacks it into work/install, and
// starts that 1.24 headless (`smolcorp serve`) with a home of its own under work, on a port of its own. It gives it
// a stand-in office: a team (two hires on a Claude account, who may write to each other, and the General Manager),
// a guard with a group switched off and a pattern added, the office's look, floors and a pin, the security settings
// opened up (every interface, hiring, the manager, a webhook), and a phone, paired through 1.24's own pairing flow.
// With -seed strict, the security settings are at their strictest instead (Tailscale only, hiring, the manager and
// the webhook off, every guard group on), so any change 1.26 makes to them is a loosening.
// Fake keys only. It quits that 1.24 (`smolcorp quit` on Linux, Ctrl+Break on Windows: stop_windows.go), puts this
// build's program in its place, as an owner who downloads 1.26 does,
// starts it headless, and checks:
//
//   - what should carry did: the team, its accounts and contacts, the guard, the office and the settings;
//   - nothing is looser than 1.24 had it, Typing over Wi-Fi included, and every key changed (the phone is let go, the
//     hook key is new);
//   - `smolcorp setup --check` says, in Setup's own words, what was kept and what reset, and names the hires from
//     1.24 to hire again (desktop/upgrade.go; desktop's TestTheUpgradeJobLooksForSetupsOwnWords holds the words
//     here to those).
//
// It never runs on a Mac: the Mac's upgrade is checked in-process (desktop/upgrade_test.go), and nothing installs
// or opens a Smolcorp build on a Mac this way.
package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/Sauzze/smolcorp/internal/appdirs"
)

// Setup's words the check looks for, as desktop/upgrade.go says them.
var setupSays = []string{
	"Updated to Smolcorp 1.26",
	"Kept as they were: your team and its worktrees, who may write to whom, your Claude accounts, your office and your settings.",
	"Every key Smolcorp keeps was changed once, so links and keys from before the update no longer work.",
	"The phone paired before the update was let go: pair it again.",
	"Typing over Wi-Fi is off",
	"Since the update, a paired phone types to agents over Tailscale only, as a new setup has it. Its keys still work over the Wi-Fi. To type over the Wi-Fi too, turn on Typing over Wi-Fi in Setup, under Paired phones: anyone on this Wi-Fi could then read what you type. The Tailscale link stays private.",
	"Re-hire your hires from 1.24 (",
}

// port is where both versions listen here: not 7710, the one in use on a computer of the team's.
const port = "7746"

func main() {
	version := flag.String("version", "", "the release upgraded from, as its tag: v1.24.2")
	archive := flag.String("archive", "", "that release's download for this system")
	newProgram := flag.String("new", "", "this build's smolcorp program (smolcorp-cli.exe on Windows)")
	work := flag.String("work", "", "an empty folder for the install and its home")
	seed := flag.String("seed", "open", "the office's security settings: open (opened up) or strict (at their strictest)")
	flag.Parse()
	if runtime.GOOS == "darwin" {
		fail("this runs in CI on Linux and Windows only: the Mac's upgrade is desktop/upgrade_test.go's")
	}
	if *version == "" || *archive == "" || *newProgram == "" || *work == "" || *seed != "open" && *seed != "strict" {
		fail("usage: upgrade -version v1.24.2 -archive FILE -new PROGRAM -work FOLDER [-seed open|strict]")
	}
	strict := *seed == "strict"
	if entries, err := os.ReadDir(*work); err != nil || len(entries) > 0 {
		fail("the work folder must be there and empty: %s", *work)
	}
	sums, err := os.ReadFile(filepath.Join(filepath.Dir(must(os.Executable())), "SHA256SUMS"))
	if err != nil {
		// go run builds the program in a temporary folder: the sums are beside this file, from the module's root.
		sums = must(os.ReadFile(filepath.Join("scripts", "upgrade", "SHA256SUMS")))
	}
	check(checkSum(sums, *version, *archive))
	install := filepath.Join(*work, "install")
	check(unpack(*archive, install))
	o := newOffice(*work, install)

	// 1.24, headless, with its stand-in office.
	project := filepath.Join(o.home, "project")
	account := filepath.Join(o.home, ".claude-work")
	for _, dir := range []string{project, account, o.support} {
		check(os.MkdirAll(dir, 0o700))
	}
	o.seedFiles(project, strict)
	o.start("1.24")
	o.seedRoutes(project, strict)
	o.quit("1.24")
	before := o.snapshot()
	say("1.24 left: %d hires, %d contacts, a phone, its settings", len(list(before.registry["hires"])), len(list(before.registry["contacts"])))

	// This build over it, as an owner installs 1.26: the program replaced in the same folder.
	replace(*newProgram, o.program)
	o.start("1.26")
	state := o.page("1.26").get("/setup/state")
	checklist := o.setupCheck()
	o.quit("1.26")
	after := o.snapshot()

	problems := carried(before, after)
	problems = append(problems, notLooser(before, after)...)
	if phones := list(state["phones"]); len(phones) != 0 {
		problems = append(problems, fmt.Sprintf("1.26 lets in %d phones paired before the update", len(phones)))
	}
	if before.hookKey == "" || after.hookKey == before.hookKey {
		problems = append(problems, "the hook key didn't change")
	}
	problems = append(problems, setupSaysIt(checklist, "Ada", "Bo", "the General Manager")...)
	fmt.Println("smolcorp setup --check, after the update:")
	fmt.Println(checklist)
	if len(problems) > 0 {
		for _, problem := range problems {
			fmt.Println("::error::" + problem)
		}
		os.Exit(1)
	}
	say("PASS: %s (%s office) to this build on %s/%s: carried, nothing looser, every key changed, Setup says it", *version, *seed, runtime.GOOS, runtime.GOARCH)
}

// setupSaysIt is what the checklist doesn't say of Setup's words for the update, and of the hires named to re-hire.
func setupSaysIt(checklist string, hires ...string) []string {
	var problems []string
	for _, words := range setupSays {
		if !strings.Contains(checklist, words) {
			problems = append(problems, fmt.Sprintf("smolcorp setup --check doesn't say %q", words))
		}
	}
	for _, name := range hires {
		if !regexp.MustCompile(`Re-hire your hires from 1\.24 \([^)]*\b` + regexp.QuoteMeta(name) + `\b[^)]*\): they still run`).MatchString(checklist) {
			problems = append(problems, fmt.Sprintf("the re-hire line doesn't name %s", name))
		}
	}
	return problems
}

// office is one install and its home.
type office struct {
	home, support, program, log string
	env                         []string
	cmd                         *exec.Cmd
	done                        chan error
}

func newOffice(work, install string) *office {
	home := filepath.Join(work, "home")
	o := &office{home: home, log: filepath.Join(work, "serve.log")}
	// Everything a Smolcorp reads from the environment, pointed at this home: its support folder, the agents'
	// settings it writes its hooks into, the browser it would open. No display.
	set := map[string]string{"HOME": home, "USERPROFILE": home, "XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"APPDATA": filepath.Join(home, "AppData", "Roaming"), "LOCALAPPDATA": filepath.Join(home, "AppData", "Local"),
		"XDG_DATA_HOME": filepath.Join(home, ".local", "share"), "XDG_STATE_HOME": filepath.Join(home, ".local", "state"),
		"SMOLCORP_PORT": port, "BROWSER": "true"}
	for _, kept := range os.Environ() {
		name, _, _ := strings.Cut(kept, "=")
		if _, ours := set[strings.ToUpper(name)]; !ours && name != "DISPLAY" && name != "WAYLAND_DISPLAY" && !strings.HasPrefix(name, "SMOLCORP_") {
			o.env = append(o.env, kept)
		}
	}
	for name, value := range set {
		o.env = append(o.env, name+"="+value)
	}
	if runtime.GOOS == "windows" {
		o.support = filepath.Join(set["APPDATA"], appdirs.Name("windows"))
		o.program = filepath.Join(install, "smolcorp-cli.exe")
	} else {
		o.support = filepath.Join(set["XDG_CONFIG_HOME"], appdirs.Name(runtime.GOOS))
		o.program = filepath.Join(install, "smolcorp")
	}
	if _, err := os.Stat(o.program); err != nil {
		fail("the release has no %s", filepath.Base(o.program))
	}
	return o
}

// seedFiles is what the owner's 1.24 office holds that its routes can't make here: hires need an agent and a
// terminal, so the team is written as 1.24's registry.json has it (desktop/frontdesk.go at v1.24.2), as is the guard,
// with deploy switched off and a pattern added, and the settings no page of 1.24's Linux and Windows app changes.
// strict: every guard group on, Tailscale only, hiring and the manager off.
func (o *office) seedFiles(project string, strict bool) {
	hire := func(id, name, nickname, role string) map[string]any {
		return map[string]any{"id": id, "name": name, "nickname": nickname, "role": role, "folder": project,
			"branch": "hire/" + id, "worktree": filepath.Join(project, ".worktrees", id), "repo": project,
			"tmuxSocket": filepath.Join(o.home, "tmux-"+id), "account": "work", "model": "", "ports": "", "brief": "",
			"started": float64(time.Now().Add(-time.Hour).UnixMilli()), "state": "working", "notes": ""}
	}
	o.write("registry.json", map[string]any{"version": 1, "hires": []any{
		hire("h1", "Builder", "Ada", "builder"), hire("h2", "Reviewer", "Bo", "reviewer"), hire("gm1", "General Manager", "", "gm"),
	}, "contacts": []map[string]string{{"from": "h1", "to": "h2"}, {"from": "h2", "to": "h1"}}})
	o.write("hire-guard.json", map[string]any{"groups": []map[string]any{
		{"name": "deploy", "on": strict, "commands": []string{}}, {"name": "extra", "on": true, "commands": []string{"terraform apply"}},
	}})
	listen := "all"
	if strict {
		listen = "tailscale" // this computer and Tailscale only: the routes below come from this computer
	}
	o.write("config.json", map[string]any{"listen": listen, "hiringEnabled": !strict, "managerEnabled": !strict, "hasCompletedSetup": true})
}

// seedRoutes sets the rest through 1.24's own Setup, as its page does, and pairs a phone through its pairing flow.
// strict: the webhook kept off.
func (o *office) seedRoutes(project string, strict bool) {
	page := o.page("1.24")
	page.post("/setup/theme", map[string]any{"body": "#102030"})
	page.post("/setup/floors", map[string]any{"floors": []map[string]string{{"name": "Lobby", "preset": ""}, {"name": "Lab", "preset": "zen"}}})
	page.post("/setup/pin?key=0&folder="+urlQuery(project), nil)
	page.post("/setup/webhook", map[string]any{"enabled": !strict, "url": "http://127.0.0.1:9/hook", "secret": "fake-ci-webhook", "outside": false})
	pairing, _ := page.post("/setup/pair", nil)["pairing"].(map[string]any)
	code, _ := pairing["code"].(string)
	if code == "" {
		fail("1.24 gave no pairing code")
	}
	answer := o.request(http.DefaultClient, "POST", "/pair", map[string]any{"code": code}, nil)
	if key, _ := answer["key"].(string); key == "" {
		fail("1.24 didn't pair the stand-in phone")
	}
	if phones := list(page.get("/setup/state")["phones"]); len(phones) != 1 {
		fail("1.24 lists %d phones after pairing one", len(phones))
	}
}

// snapshot is the office's files as the version that just quit left them.
type snapshot struct {
	registry, config map[string]any
	guard            []byte
	hookKey          string
}

func (o *office) snapshot() snapshot {
	read := func(name string) map[string]any {
		data, err := os.ReadFile(filepath.Join(o.support, name))
		if err != nil {
			fail("%s: %v", name, err)
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil {
			fail("%s isn't JSON: %v", name, err)
		}
		return value
	}
	guard, _ := os.ReadFile(filepath.Join(o.support, "hire-guard.json"))
	key, _ := os.ReadFile(filepath.Join(o.support, "hook-key"))
	return snapshot{registry: read("registry.json"), config: read("config.json"), guard: guard, hookKey: strings.TrimSpace(string(key))}
}

// carried is what didn't carry from before to after: the team as 1.24 left it, field by field, its contacts, the
// guard, and the office's look, floors, pin and settings.
func carried(before, after snapshot) []string {
	var problems []string
	hires := map[string]map[string]any{}
	for _, h := range list(after.registry["hires"]) {
		row, _ := h.(map[string]any)
		hires[fmt.Sprint(row["id"])] = row
	}
	for _, h := range list(before.registry["hires"]) {
		was, _ := h.(map[string]any)
		now := hires[fmt.Sprint(was["id"])]
		for _, field := range []string{"name", "nickname", "role", "folder", "branch", "worktree", "repo", "account", "state"} {
			if now == nil || fmt.Sprint(now[field]) != fmt.Sprint(was[field]) {
				problems = append(problems, fmt.Sprintf("hire %v's %s: 1.24 had %v, 1.26 has %v", was["id"], field, was[field], now[field]))
			}
		}
	}
	if !sameJSON(before.registry["contacts"], after.registry["contacts"]) {
		problems = append(problems, fmt.Sprintf("who may write to whom: 1.24 had %v, 1.26 has %v", before.registry["contacts"], after.registry["contacts"]))
	}
	if !bytes.Equal(before.guard, after.guard) {
		problems = append(problems, "the guard file changed")
	}
	// What 1.26 adds to a setting (a layout's new choices, the roles shipped since) is allowed; what 1.24 had must be
	// there as it was.
	for _, name := range []string{"colours", "floors", "pins", "layout", "listen", "hiringEnabled", "managerEnabled", "webhook"} {
		if !within(before.config[name], after.config[name]) {
			problems = append(problems, fmt.Sprintf("config.json's %s: 1.24 had %v, 1.26 has %v", name, before.config[name], after.config[name]))
		}
	}
	ids := map[string]bool{}
	for _, role := range list(after.config["roles"]) {
		row, _ := role.(map[string]any)
		ids[fmt.Sprint(row["id"])] = true
	}
	for _, role := range list(before.config["roles"]) {
		if row, _ := role.(map[string]any); !ids[fmt.Sprint(row["id"])] {
			problems = append(problems, fmt.Sprintf("role %v is gone", row["id"]))
		}
	}
	return problems
}

// within is whether all of was is in now: each of its object's fields, each of its list's items (by place, but for
// an empty one), each value equal.
func within(was, now any) bool {
	switch was := was.(type) {
	case map[string]any:
		now, ok := now.(map[string]any)
		if !ok {
			return false
		}
		for name, value := range was {
			if !within(value, now[name]) {
				return false
			}
		}
		return true
	case []any:
		now, ok := now.([]any)
		if !ok {
			return len(was) == 0 && now == nil
		}
		for i, value := range was {
			if value != nil && (i >= len(now) || !within(value, now[i])) {
				return false
			}
		}
		return true
	}
	return sameJSON(was, now)
}

// notLooser is what 1.26 holds looser than 1.24 had it: a security setting on that was off (or that 1.24 didn't
// have: Typing over Wi-Fi, which 1.24 had no switch for, among them), a wider listen, a webhook that goes further, a
// guard group off that was on.
func notLooser(before, after snapshot) []string {
	var problems []string
	for _, name := range []string{"hiringEnabled", "managerEnabled", "gmByHand", "keystrokesAllowed", "mailInboxSocket",
		"t3CodeEnabled", "floorsFromOtherMacs", "phoneMicSkipsConfirm", "phoneTextOverWiFi"} {
		if after.config[name] == true && before.config[name] != true {
			problems = append(problems, fmt.Sprintf("%s is on in 1.26, off in 1.24", name))
		}
	}
	if apps := list(after.config["keystrokeApps"]); len(apps) > len(list(before.config["keystrokeApps"])) {
		problems = append(problems, fmt.Sprintf("keys may go to more apps in 1.26: %v", apps))
	}
	if after.config["listen"] == "all" && before.config["listen"] != "all" {
		problems = append(problems, "1.26 listens on every interface, 1.24 didn't")
	}
	was, _ := before.config["webhook"].(map[string]any)
	now, _ := after.config["webhook"].(map[string]any)
	if now["enabled"] == true && (was["enabled"] != true || now["url"] != was["url"]) || now["outside"] == true && was["outside"] != true {
		problems = append(problems, fmt.Sprintf("the webhook goes further in 1.26: %v", now["url"]))
	}
	groupsOn := func(data []byte) map[string]bool {
		var file struct {
			Groups []struct {
				Name string `json:"name"`
				On   bool   `json:"on"`
			} `json:"groups"`
		}
		_ = json.Unmarshal(data, &file)
		on := map[string]bool{}
		for _, g := range file.Groups {
			on[g.Name] = g.On
		}
		return on
	}
	wasOn, nowOn := groupsOn(before.guard), groupsOn(after.guard)
	for name, on := range wasOn {
		if on && !nowOn[name] {
			problems = append(problems, fmt.Sprintf("guard group %s is off in 1.26, on in 1.24", name))
		}
	}
	return problems
}

// start starts the install's program headless, and waits until it answers.
func (o *office) start(which string) {
	out, err := os.OpenFile(o.log, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	check(err)
	o.cmd = exec.Command(o.program, "serve")
	o.cmd.Env, o.cmd.Stdout, o.cmd.Stderr, o.cmd.Dir = o.env, out, out, o.home
	inItsOwnGroup(o.cmd)
	check(o.cmd.Start())
	o.done = make(chan error, 1)
	go func() { o.done <- o.cmd.Wait(); out.Close() }()
	for deadline := time.Now().Add(90 * time.Second); ; time.Sleep(300 * time.Millisecond) {
		select {
		case err := <-o.done:
			o.showLog()
			fail("%s stopped as it started: %v", which, err)
		default:
		}
		if response, err := http.Get("http://127.0.0.1:" + port + "/setup/state"); err == nil {
			response.Body.Close()
			if o.setupToken() != "" {
				say("%s is serving on %s", which, port)
				return
			}
		}
		if time.Now().After(deadline) {
			o.stop()
			o.showLog()
			fail("%s didn't answer within 90 s", which)
		}
	}
}

// quit stops it as an owner without a tray does, and waits for it to end: `smolcorp quit` on Linux, Ctrl+Break on
// Windows (askToQuit).
func (o *office) quit(which string) {
	how, output, err := o.askToQuit()
	select {
	case <-o.done:
	case <-time.After(30 * time.Second):
		o.stop()
		fail("%s didn't stop after %s (%v): %s", which, how, err, output)
	}
	say("%s quit", which)
}

func (o *office) stop() {
	_ = o.cmd.Process.Kill()
	<-o.done
}

func (o *office) showLog() {
	data, _ := os.ReadFile(o.log)
	fmt.Printf("its output:\n%s\n", data)
}

// setupToken is the one-time token of the setup link the running version printed as it started, "" until it has.
func (o *office) setupToken() string {
	data, _ := os.ReadFile(o.log)
	return linkToken(data)
}

// linkToken is the token of the last setup link in a version's output, "" for none.
func linkToken(output []byte) string {
	found := regexp.MustCompile(`/setup#o=([A-Za-z0-9_-]+)`).FindAllSubmatch(output, -1)
	if len(found) == 0 {
		return ""
	}
	return string(found[len(found)-1][1])
}

// setupCheck is what `smolcorp setup --check` prints: its checklist, whatever is left to do in it.
func (o *office) setupCheck() string {
	command := exec.Command(o.program, "setup", "--check")
	command.Env, command.Dir = o.env, o.home
	output, _ := command.CombinedOutput()
	if !bytes.Contains(output, []byte("Ready.")) && !bytes.Contains(output, []byte("Not ready")) {
		fail("smolcorp setup --check printed no checklist:\n%s", output)
	}
	return string(output)
}

// setupPage is the setup page's session, opened with the setup link's token as the page itself does.
type setupPage struct {
	o      *office
	client *http.Client
}

func (o *office) page(which string) setupPage {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: time.Minute}
	request, _ := http.NewRequest("POST", "http://127.0.0.1:"+port+"/setup/session", nil)
	request.Header.Set("X-AgentDeck-Setup", "1")
	request.Header.Set("X-Smolcorp-Setup-Once", o.setupToken())
	response, err := client.Do(request)
	if err != nil || response.StatusCode != http.StatusNoContent {
		fail("%s's setup link didn't open a session: %v %v", which, err, status(response))
	}
	response.Body.Close()
	return setupPage{o, client}
}

func (p setupPage) get(path string) map[string]any {
	return p.o.request(p.client, "GET", path, nil, nil)
}

func (p setupPage) post(path string, body any) map[string]any {
	return p.o.request(p.client, "POST", path, body, map[string]string{"X-AgentDeck-Setup": "1"})
}

// request asks the running version, and is its JSON answer (nil for none); anything but a 2xx stops the run.
func (o *office) request(client *http.Client, method, path string, body any, headers map[string]string) map[string]any {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(must(json.Marshal(body)))
	}
	request, _ := http.NewRequest(method, "http://127.0.0.1:"+port+path, reader)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := client.Do(request)
	if err != nil {
		fail("%s %s: %v", method, path, err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if response.StatusCode/100 != 2 {
		fail("%s %s: %d %s", method, path, response.StatusCode, firstLine(data))
	}
	var answer map[string]any
	_ = json.Unmarshal(data, &answer)
	return answer
}

func (o *office) write(name string, value any) {
	check(os.WriteFile(filepath.Join(o.support, name), must(json.Marshal(value)), 0o600))
}

// checkSum is nil when archive's SHA-256 is the one SHA256SUMS keeps for version and its name.
func checkSum(sums []byte, version, archive string) error {
	want := ""
	scanner := bufio.NewScanner(bytes.NewReader(sums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == version+"/"+filepath.Base(archive) {
			want = fields[0]
		}
	}
	if want == "" {
		return fmt.Errorf("SHA256SUMS has no line for %s/%s", version, filepath.Base(archive))
	}
	data, err := os.ReadFile(archive)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("%s is %s, not the %s SHA256SUMS keeps", archive, got, want)
	}
	say("%s/%s: SHA-256 %s, as kept", version, filepath.Base(archive), want)
	return nil
}

// unpack puts the release's files in folder, without the release's own top folder (Smolcorp-Linux-x86_64/…). A name
// that would land outside it is an error.
func unpack(archive, folder string) error {
	place := func(name string, mode os.FileMode, from io.Reader) error {
		_, rest, _ := strings.Cut(filepath.ToSlash(name), "/")
		if rest == "" || strings.HasSuffix(rest, "/") {
			return nil
		}
		if !filepath.IsLocal(filepath.FromSlash(rest)) {
			return fmt.Errorf("%s would land outside the install", name)
		}
		path := filepath.Join(folder, filepath.FromSlash(rest))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0o600)
		if err != nil {
			return err
		}
		_, err = io.Copy(file, from)
		return errors.Join(err, file.Close())
	}
	if strings.HasSuffix(archive, ".zip") {
		zipped, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer zipped.Close()
		for _, entry := range zipped.File {
			if entry.FileInfo().IsDir() {
				continue
			}
			from, err := entry.Open()
			if err != nil {
				return err
			}
			err = place(entry.Name, entry.Mode().Perm(), from)
			from.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	tarred := tar.NewReader(gz)
	for {
		entry, err := tarred.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.Typeflag == tar.TypeReg {
			if err := place(entry.Name, os.FileMode(entry.Mode).Perm(), tarred); err != nil {
				return err
			}
		}
	}
}

// replace puts the program from in to's place, as a new download unpacked over the old one does.
func replace(from, to string) {
	data := must(os.ReadFile(from))
	check(os.WriteFile(to+".new", data, 0o700))
	check(os.Rename(to+".new", to))
}

func list(value any) []any {
	items, _ := value.([]any)
	return items
}

func sameJSON(a, b any) bool { return bytes.Equal(must(json.Marshal(a)), must(json.Marshal(b))) }

func urlQuery(text string) string {
	return strings.NewReplacer("%", "%25", "&", "%26", "+", "%2B", " ", "%20", "#", "%23", "?", "%3F", "=", "%3D").Replace(text)
}

func status(response *http.Response) string {
	if response == nil {
		return ""
	}
	return response.Status
}

func firstLine(data []byte) string {
	line, _, _ := strings.Cut(string(data), "\n")
	return line
}

func say(format string, args ...any) { fmt.Printf("upgrade: "+format+"\n", args...) }

func fail(format string, args ...any) {
	fmt.Printf("::error::upgrade: "+format+"\n", args...)
	os.Exit(1)
}

func check(err error) {
	if err != nil {
		fail("%v", err)
	}
}

func must[T any](value T, err error) T {
	check(err)
	return value
}
