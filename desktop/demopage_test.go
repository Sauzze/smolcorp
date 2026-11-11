//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The demo's page (demopage.go): the app reads the demo's hire from its folder, opens Setup at it with a one-time
// link the command never sees, hears whether the page is open, and the page's own hire keeps the GM's scope.

// demoPageDeck is a deck on the engine's stand-ins with the manager and hiring off, and a demo folder in home.
func demoPageDeck(t *testing.T) (*Deck, demoFolder) {
	t.Helper()
	d, _, _, _ := gmHireDeck(t)
	d.mu.Lock()
	d.config.ManagerEnabled = false
	d.mu.Unlock()
	f, err := makeDemoFolder(demoTestFolder(t), true, []string{"reviewer", "tester"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		demoPages.Lock()
		delete(demoPages.at, f.root)
		demoPages.Unlock()
	})
	return d, f
}

// demoCall is a request to the demo's routes: as the page (its cookie), or as a tool (the setup key).
func demoPageCall(t *testing.T, d *Deck, asTool bool, method, target, body string) (int, string) {
	t.Helper()
	w := serve(t, d, call{method: method, target: target, host: "localhost", from: thisComputer, body: body, asTool: asTool,
		headers: map[string]string{setupHeader: "1"}})
	d.watching.Wait()
	return w.Code, strings.TrimSpace(w.Body.String())
}

type demoPageAnswer struct {
	GMHired bool     `json:"gmHired"`
	Open    []string `json:"open"`
	Demo    *struct {
		Root, Folder, Scope, Page string
		Roles                     []string
		Hire                      map[string]any
		Switches                  []struct {
			Name, Route, Setting string
			On                   bool
		}
		GM *struct{ ID, State string }
	}
}

func readDemoPage(t *testing.T, d *Deck, asTool bool, root string) demoPageAnswer {
	t.Helper()
	target := "/setup/demo"
	if root != "" {
		target += "?root=" + url.QueryEscape(root)
	}
	code, body := demoPageCall(t, d, asTool, "GET", target, "")
	var answer demoPageAnswer
	if code != http.StatusOK || json.Unmarshal([]byte(body), &answer) != nil {
		t.Fatalf("GET %s: %d %s", target, code, body)
	}
	return answer
}

func TestTheDemosPageReadsItsHireFromTheDemoFolder(t *testing.T) {
	d, f := demoPageDeck(t)
	// The office as the demo needs it, for the page and for the command (a tool).
	if a := readDemoPage(t, d, true, ""); a.GMHired || len(a.Open) != 0 || a.Demo != nil {
		t.Errorf("the office: %+v", a)
	}
	a := readDemoPage(t, d, true, f.root)
	if a.Demo == nil {
		t.Fatal("no demo")
	}
	demo := a.Demo
	if demo.Root != f.root || demo.Folder != f.repo || demo.Scope != f.repo || strings.Join(demo.Roles, ",") != "reviewer,tester" ||
		demo.Page != "waiting" || demo.GM != nil {
		t.Errorf("the demo: %+v", demo)
	}
	task, _ := demo.Hire["task"].(string)
	if demo.Hire["role"] != gmRole || demo.Hire["folder"] != f.repo || demo.Hire["scope"] != f.repo || !strings.Contains(task, "--folder "+f.repo) ||
		!strings.Contains(task, "Roles you may hire: reviewer, tester.") || !strings.Contains(task, "Stop before anything ships") {
		t.Errorf("the hire: %v", demo.Hire)
	}
	if len(demo.Switches) != 2 || demo.Switches[0].Route != "/setup/gm/manager" || demo.Switches[0].On || demo.Switches[1].Route != "/setup/hiring" ||
		!demo.Switches[1].On || demo.Switches[0].Setting != "Let a General Manager run your team" {
		t.Errorf("the switches: %+v", demo.Switches)
	}
	// Only a folder the demo made: not another folder, a demo folder's project, a folder outside home, nor one named so by hand.
	fake := filepath.Join(homeDir, "smolcorp-demo-7")
	os.MkdirAll(filepath.Join(fake, demoRepoName, ".git"), 0o755)
	os.MkdirAll(filepath.Join(fake, demoRepoName, ".claude"), 0o755)
	os.WriteFile(filepath.Join(fake, demoRepoName, ".claude", "settings.json"), []byte(`{"hooks":{}}`), 0o644)
	for _, root := range []string{"", "relative/smolcorp-demo", homeDir, f.repo, filepath.Dir(f.root), t.TempDir(), fake, filepath.Join(f.root, "gone")} {
		for _, target := range []string{"/setup/demo?root=" + url.QueryEscape(root), "/setup/demo/open?root=" + url.QueryEscape(root)} {
			method := map[bool]string{true: "POST", false: "GET"}[strings.Contains(target, "/open")]
			if root == "" && method == "GET" {
				continue // no root is the office alone, above
			}
			if code, body := demoPageCall(t, d, true, method, target, ""); code != http.StatusBadRequest || body == "" {
				t.Errorf("%s %s: %d %q", method, target, code, body)
			}
		}
	}
	for name, want := range map[string]bool{"smolcorp-demo": true, "smolcorp-demo-2": true, "smolcorp-demo-12": true, "smolcorp-demo-1": false,
		"smolcorp-demo-02": false, "smolcorp-demo-x": false, "smolcorp-demos": false} {
		if demoFolderNamed(name) != want {
			t.Errorf("demoFolderNamed(%q) = %v", name, !want)
		}
	}
}

// The app opens the page itself: the command gets no token. The link opens Setup at the demo, once.
func TestTheDemosPageOpensWithALinkTheCommandNeverSees(t *testing.T) {
	d, f := demoPageDeck(t)
	saved := openURL
	t.Cleanup(func() { openURL = saved })
	var opened []string
	openURL = func(link string) error { opened = append(opened, link); return nil }
	target := "/setup/demo/open?root=" + url.QueryEscape(f.root)
	code, body := demoPageCall(t, d, true, "POST", target, "")
	if code != http.StatusNoContent || body != "" || len(opened) != 1 {
		t.Fatalf("open: %d %q %v", code, body, opened)
	}
	link, err := url.Parse(opened[0])
	if err != nil || link.Path != "/setup" || link.Query().Get("demo") != f.root || !strings.HasPrefix(link.Fragment, "o=") {
		t.Fatalf("the link: %s", opened[0])
	}
	// The link's token gives the page its session, once.
	session := func() int {
		w := serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", setupTokenHeader: strings.TrimPrefix(link.Fragment, "o=")}})
		return w.Code
	}
	if first, second := session(), session(); first != http.StatusNoContent || second != http.StatusUnauthorized {
		t.Errorf("the token: %d then %d", first, second)
	}
	// Once per demo folder: a second open is refused, and opens nothing.
	if code, body := demoPageCall(t, d, true, "POST", target, ""); code != http.StatusConflict || len(opened) != 1 || !strings.Contains(body, "run smolcorp demo again") {
		t.Errorf("a second open: %d %q %v", code, body, opened)
	}
}

// The page says it's open, or closed; only the page may.
func TestTheDemosPageSaysWhetherItsOpen(t *testing.T) {
	d, f := demoPageDeck(t)
	savedQuiet, savedGrace := demoPageQuiet, demoPageGrace
	t.Cleanup(func() { demoPageQuiet, demoPageGrace = savedQuiet, savedGrace })
	says := func(asTool bool, state string) int {
		code, _ := demoPageCall(t, d, asTool, "POST", "/setup/demo/page?root="+url.QueryEscape(f.root)+"&state="+state, "")
		return code
	}
	page := func() string { return readDemoPage(t, d, true, f.root).Demo.Page }
	if code := says(true, "closed"); code != http.StatusForbidden || page() != "waiting" {
		t.Errorf("a tool saying closed: %d %s", code, page())
	}
	if code := says(false, "gone"); code != http.StatusBadRequest {
		t.Errorf("a state that isn't one: %d", code)
	}
	if code := says(false, "open"); code != http.StatusNoContent || page() != "open" {
		t.Errorf("open: %d %s", code, page())
	}
	// Closed and open again at once (a reload) is open; closed for good is closed.
	says(false, "closed")
	if page() != "open" {
		t.Errorf("just closed: %s", page())
	}
	demoPageGrace = 0
	if page() != "closed" {
		t.Errorf("closed: %s", page())
	}
	says(false, "open")
	demoPageQuiet = 0
	if page() != "closed" {
		t.Errorf("not heard from: %s", page())
	}
	// The GM's token gets none of the demo's routes.
	token := d.newGMToken()
	w := serve(t, d, call{method: "GET", target: "/setup/demo", host: "localhost", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1", gmTokenHeader: token}})
	if w.Code != http.StatusForbidden {
		t.Errorf("the GM: %d", w.Code)
	}
}

// The page's Hire: the switches it needs on (the page may), then its own POST /setup/hire of what the app read,
// with the scope, which the app keeps. The command, a tool, can do neither.
func TestTheDemosPageHireKeepsTheScope(t *testing.T) {
	d, f := demoPageDeck(t)
	demo := readDemoPage(t, d, false, f.root).Demo
	hire, _ := json.Marshal(demo.Hire)
	// The setup key alone (the demo's command) can't turn the manager on, nor hire.
	if code, _ := demoPageCall(t, d, true, "POST", "/setup/gm/manager?enabled=1", ""); code != http.StatusForbidden {
		t.Errorf("a tool's manager switch: %d", code)
	}
	if code, _ := demoPageCall(t, d, true, "POST", "/setup/hire", string(hire)); code != http.StatusForbidden {
		t.Errorf("a tool's hire: %d", code)
	}
	for _, s := range demo.Switches {
		if !s.On {
			if code, body := demoPageCall(t, d, false, "POST", s.Route+"?enabled=1", ""); code >= 300 {
				t.Fatalf("the page's %s: %d %s", s.Route, code, body)
			}
		}
	}
	code, body := demoPageCall(t, d, false, "POST", "/setup/hire", string(hire))
	if code != http.StatusOK {
		t.Fatalf("the page's hire: %d %s", code, body)
	}
	hires := registered(t)
	if len(hires) != 1 || hires[0].Role != gmRole || hires[0].Folder != f.repo || hires[0].Scope != f.repo || gmScopeNow() != f.repo {
		t.Fatalf("registry: %+v", hires)
	}
	// The command sees its GM; the page can't be opened for a demo whose GM is hired.
	if a := readDemoPage(t, d, true, f.root); a.Demo.GM == nil || a.Demo.GM.ID != hires[0].ID || !a.GMHired {
		t.Errorf("after the hire: %+v", a.Demo)
	}
	if code, _ := demoPageCall(t, d, true, "POST", "/setup/demo/open?root="+url.QueryEscape(f.root), ""); code != http.StatusConflict {
		t.Errorf("open after the hire: %d", code)
	}
}

// The page's own code, in node: the demo it was opened at (?demo=), and what its Hire sends.
func TestTheDemosPageParamsInNode(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: the page's script can't be run")
	}
	data, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	parse := regexp.MustCompile(`(?s)\n  function demoFromAddress\(search\) \{.*?\n  \}\n`).FindString(page)
	if parse == "" {
		t.Fatal("no demoFromAddress in setup.html")
	}
	script := parse + `
const assert = (ok, what) => { if (!ok) throw new Error(what); };
assert(demoFromAddress('?demo=' + encodeURIComponent('/home/ada/smolcorp-demo')) === '/home/ada/smolcorp-demo', 'a full path');
assert(demoFromAddress('?app&demo=%2FUsers%2Fada%2Fsmolcorp-demo-2') === '/Users/ada/smolcorp-demo-2', 'beside ?app');
assert(demoFromAddress('?demo=' + encodeURIComponent('/home/ada/my demo/smolcorp-demo')) === '/home/ada/my demo/smolcorp-demo', 'a space');
assert(demoFromAddress('?demo=' + encodeURIComponent('C:\\Users\\ada\\smolcorp-demo')) === 'C:\\Users\\ada\\smolcorp-demo', 'a Windows path');
assert(demoFromAddress('?demo=%20%2Fhome%2Fada%2Fsmolcorp-demo%20') === '/home/ada/smolcorp-demo', 'spaces around');
assert(demoFromAddress('') === '', 'none');
assert(demoFromAddress('?demo=') === '', 'empty');
assert(demoFromAddress('?demo=smolcorp-demo') === '', 'a relative path');
assert(demoFromAddress('?demo=~%2Fsmolcorp-demo') === '', 'a home path the page would have to guess');
assert(demoFromAddress('?pair') === '', 'another page');
console.log('ok');
`
	out, err := exec.Command(node, "-e", script).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("%v\n%s", err, out)
	}
	// Taking the token out of the address keeps the demo in it; Hire sends what the app read, the scope with it.
	for _, want := range []string{
		"history.replaceState(null, '', location.pathname + location.search + (then ? `#${then}` : ''));",
		"fetch('/setup/hire', { method: 'POST', headers: setupHeaders(true),\n      body: JSON.stringify({ ...demo.hire, ...(account ? { account } : {}) }) })",
		"fetch(`${s.route}?enabled=1`, { method: 'POST', headers: setupHeaders(false) })",
		"keepalive: word === 'closed'",
		"}).then(loadDemo).then(loadFirstFolder)",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("setup.html has no %q", want)
		}
	}
}
