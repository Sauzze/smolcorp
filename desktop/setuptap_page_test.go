package desktop

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/appdirs"
)

// The pages' halves of Setup opened by a tap on the phone (FXSETUPTAP; OLD-TAP1 built the server's, setupask.go).
// setup.html, opened at #tap, asks, shows the code and waits (tapFor); the phone's page shows the ask and answers it
// (deck.html's setupAskSheetFor). These rows run the pages' own functions in node against this package's server: the
// page's on a real listener on 127.0.0.1, where its fetch keeps the session's cookie as a browser does; the phone's on
// what GET /grants answered the phone. Without node they're skipped.

// pageSource is a page of Resources/web.
func pageSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Resources", "web", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// pageFunction is a function of a page's script, whole: from its "function name(" line to its brace at two spaces in.
func pageFunction(t *testing.T, page, name string) string {
	t.Helper()
	at := strings.Index(page, "\n  function "+name+"(")
	if async := strings.Index(page, "\n  async function "+name+"("); at < 0 {
		at = async
	}
	if at < 0 {
		t.Fatalf("the page has no function %s: update this test", name)
	}
	fn := page[at+1:]
	return fn[:strings.Index(fn, "\n  }\n")+4]
}

// pageLine is the line of a page's script that starts with start, whole.
func pageLine(t *testing.T, page, start string) string {
	t.Helper()
	at := strings.Index(page, "\n  "+start)
	if at < 0 {
		t.Fatalf("the page has no line %q: update this test", start)
	}
	line := page[at+1:]
	return line[:strings.Index(line, "\n")+1]
}

// nodeOrSkip is node, or the test is skipped.
func nodeOrSkip(t *testing.T) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: the pages' functions can't be run")
	}
	return node
}

// tapPageScript is setup.html at #tap, in node: its tapFor, with fetch keeping the cookie the answer sets, as a
// browser does for the page. It prints the ask as the page shows it, then how the tap ended and the cookie it holds.
func tapPageScript(t *testing.T) string {
	t.Helper()
	page := pageSource(t, "setup.html")
	return `
const BASE = process.argv[2];
let jar = '';
async function ask(path, init = {}) {
  const { cache, ...rest } = init;
  const res = await fetch(BASE + path, { ...rest, headers: { ...(rest.headers || {}), ...(jar ? { Cookie: jar } : {}) } });
  for (const cookie of res.headers.getSetCookie()) jar = cookie.split(';')[0];
  return res;
}
` + pageFunction(t, page, "setupHeaders") + pageFunction(t, page, "tapFor") + pageFunction(t, page, "tapWaitingWords") +
		pageFunction(t, page, "tapEndWords") + `
(async () => {
  const said = await tapFor(ask, (asked) => console.log(JSON.stringify({ shown: tapWaitingWords(asked), code: asked.code })));
  console.log(JSON.stringify({ state: said.state, end: said.state === 'allowed' ? null : tapEndWords(said), cookie: jar }));
})().catch((e) => { console.error(e); process.exit(1); });
`
}

// pageWait is how long a row waits for the next line a page running in node prints: each comes well within a second
// here, and far longer still under the race detector (raceSlowdown), which slows this package's server, not node.
const pageWait = 15 * time.Second * raceSlowdown

// nodePage is a page's script running in node, and the lines it prints, read to the end before cmd.Wait: Wait closes
// the pipe once node exits, so a line printed while the row was busy elsewhere (the phone's half) must already have
// been read, or it's lost (the Studio's race-desktop run of bd4a5d94, the Wi-Fi case).
type nodePage struct {
	lines  chan string
	done   chan struct{} // closed once node has gone and its stderr is all in
	stderr strings.Builder
	cancel context.CancelFunc
	stdin  io.WriteCloser // what the row answers a page that reads its input (phoneSheetPage); others never read it
}

// runInNode runs node with args, and is ended in a cleanup.
func runInNode(t *testing.T, node string, args ...string) *nodePage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), pageWait+30*time.Second)
	p := &nodePage{lines: make(chan string), done: make(chan struct{}), cancel: cancel}
	cmd := exec.CommandContext(ctx, node, args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if p.stdin, err = cmd.StdinPipe(); err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = &p.stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(p.done)
		lines := bufio.NewScanner(out)
		lines.Buffer(make([]byte, 0, 64<<10), 1<<20)
		for lines.Scan() {
			select {
			case p.lines <- lines.Text():
			case <-ctx.Done(): // the row has stopped reading: go on to the end, so Wait can close the pipe
			}
		}
		close(p.lines)
		_ = cmd.Wait()
	}()
	t.Cleanup(func() {
		cancel()
		<-p.done
	})
	return p
}

// next is the next line the page printed, as JSON into v, waiting up to pageWait for it.
func (p *nodePage) next(t *testing.T, what string, v any) {
	t.Helper()
	select {
	case line, ok := <-p.lines:
		if !ok {
			<-p.done
			t.Fatalf("%s: node ended without printing it: %s", what, p.stderr.String())
		}
		if err := json.Unmarshal([]byte(line), v); err != nil {
			t.Fatalf("%s: the page printed %q", what, line)
		}
	case <-time.After(pageWait):
		p.cancel()
		<-p.done
		t.Fatalf("%s: nothing printed in %v: %s", what, pageWait, p.stderr.String())
	}
}

// tapPage is setup.html opened at #tap, running in node against server: what it showed, once it has asked.
type tapPage struct{ *nodePage }

type tapPageShown struct {
	Shown struct{ Head, Code, Hint string }
	Code  string
}

type tapPageEnd struct {
	State  string
	End    *struct{ Head, Hint string }
	Cookie string
}

func openTapPage(t *testing.T, node, base string) (*tapPage, tapPageShown) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "tap.js")
	if err := os.WriteFile(file, []byte(tapPageScript(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	page := &tapPage{runInNode(t, node, file, base)}
	var shown tapPageShown
	if page.next(t, "the page's ask", &shown); shown.Code == "" {
		t.Fatalf("the page showed no ask: %+v", shown)
	}
	return page, shown
}

// end is how the page's tap ended.
func (p *tapPage) end(t *testing.T) tapPageEnd {
	t.Helper()
	var end tapPageEnd
	p.next(t, "how the page's tap ended", &end)
	return end
}

// phoneSheet is deck.html's sheet for what GET /grants answered the phone: its words, and the ask it answers.
type phoneSheet struct {
	Head, Code, Why, More string
	Ask                   *struct{ ID, Code, Computer string }
}

func phoneSheetFor(t *testing.T, node string, status int, body string) *phoneSheet {
	t.Helper()
	page := pageSource(t, "deck.html")
	dir := t.TempDir()
	script, answer := filepath.Join(dir, "sheet.js"), filepath.Join(dir, "answer")
	source := pageLine(t, page, "const SETUP_ASK_NOT_HERE = ") + pageFunction(t, page, "setupAskSheetFor") + `
console.log(JSON.stringify(setupAskSheetFor(Number(process.argv[2]), require('fs').readFileSync(process.argv[3], 'utf8'))));
`
	if os.WriteFile(script, []byte(source), 0o600) != nil || os.WriteFile(answer, []byte(body), 0o600) != nil {
		t.Fatal("can't write the phone's script")
	}
	out, err := exec.Command(node, script, strconv.Itoa(status), answer).CombinedOutput()
	if err != nil {
		t.Fatalf("the phone's sheet: %v\n%s", err, out)
	}
	var sheet *phoneSheet
	if err := json.Unmarshal(out, &sheet); err != nil {
		t.Fatalf("the phone's sheet: %q", out)
	}
	return sheet
}

// The whole tap, both pages and the server: the page at #tap asks and shows "open Smolcorp on iPhone", with its code;
// the phone over Tailscale shows "Open Setup on studio-pc?" with the same code; the owner's Allow opens Setup for that
// page alone, which reads and changes it. A Deny, an ask that runs out, a tap with another code, and the owner's
// phone over the Wi-Fi or a phone that isn't paired each leave the page without a session, saying why, and how to
// ask again. On the Wi-Fi the phone's sheet says why it can't answer there. Fail-first: the tree before FXSETUPTAP has
// no tapFor in setup.html and no sheet in deck.html.
func TestSetupAtTapAsksThePhoneAndOpensOnlyOnItsAllow(t *testing.T) {
	node := nodeOrSkip(t)
	for _, c := range []struct {
		name   string
		answer func(t *testing.T, d *Deck, phoneKey string, sheet *phoneSheet) // the phone's side, once the page shows its ask
		state  string
		head   string
	}{
		{"the owner's Allow", func(t *testing.T, d *Deck, phoneKey string, sheet *phoneSheet) {
			if status, said := tapLive(t, d, phoneKey, sheet.Ask.ID, sheet.Ask.Code, "allow", callerNamed(t, "phone-wireguard")); status != http.StatusOK {
				t.Fatalf("the owner's Allow: %d %s", status, said)
			}
		}, "allowed", ""},
		{"the owner's Deny", func(t *testing.T, d *Deck, phoneKey string, sheet *phoneSheet) {
			if status, said := tapLive(t, d, phoneKey, sheet.Ask.ID, sheet.Ask.Code, "deny", callerNamed(t, "phone-wireguard")); status != http.StatusOK {
				t.Fatalf("the owner's Deny: %d %s", status, said)
			}
		}, "denied", "You tapped Deny on your phone, so Setup stayed closed."},
		{"no answer", func(t *testing.T, d *Deck, _ string, _ *phoneSheet) { runOutSetupAsks(d) }, "ranOut",
			"The ask ran out or Smolcorp restarted, so Setup stayed closed."},
		{"another code", func(t *testing.T, d *Deck, phoneKey string, sheet *phoneSheet) {
			other := "1-1-1-1"
			if sheet.Ask.Code == other {
				other = "2-2-2-2"
			}
			if status, _ := tapLive(t, d, phoneKey, sheet.Ask.ID, other, "allow", callerNamed(t, "phone-wireguard")); status != http.StatusNotFound {
				t.Errorf("an Allow with another code: %d", status)
			}
			runOutSetupAsks(d)
		}, "ranOut", "The ask ran out or Smolcorp restarted, so Setup stayed closed."},
		{"the phone over the Wi-Fi", func(t *testing.T, d *Deck, phoneKey string, sheet *phoneSheet) {
			wifi := callerNamed(t, "phone-wifi")
			listed := probeAnswerAsSet(t, d, "GET", "/grants?", "", phoneKey, wifi)
			here := phoneSheetFor(t, node, listed.Code, listed.Body.String())
			t.Logf("the Wi-Fi's phone: GET /grants %d, its sheet %+v", listed.Code, here)
			if listed.Code != http.StatusForbidden || here == nil || here.Ask != nil || !strings.Contains(here.Why, "studio-pc asks to open Setup") ||
				!strings.Contains(here.Why, "never over the Wi-Fi") {
				t.Errorf("the Wi-Fi's phone isn't told why it can't answer: %d %+v", listed.Code, here)
			}
			// The refusal names this computer and nothing of the ask (SETUPTAPR3's O1): its code or id would reach a
			// sheet with nothing to tap, over the Wi-Fi.
			for _, secret := range []string{sheet.Ask.Code, sheet.Ask.ID} {
				if (here != nil && strings.Contains(here.Why, secret)) || strings.Contains(listed.Body.String(), secret) {
					t.Errorf("the Wi-Fi's refusal carries the ask's %q: %s", secret, listed.Body)
				}
			}
			if status, _ := tapLive(t, d, phoneKey, sheet.Ask.ID, sheet.Ask.Code, "allow", wifi); status != http.StatusForbidden {
				t.Errorf("an Allow over the Wi-Fi: %d", status)
			}
			runOutSetupAsks(d)
			if after := probeAnswerAsSet(t, d, "GET", "/grants?", "", phoneKey, wifi); phoneSheetFor(t, node, after.Code, after.Body.String()) != nil {
				t.Errorf("with no ask waiting, the Wi-Fi's phone still shows a sheet: %d %s", after.Code, after.Body)
			}
		}, "ranOut", "The ask ran out or Smolcorp restarted, so Setup stayed closed."},
		{"a phone that isn't paired", func(t *testing.T, d *Deck, _ string, sheet *phoneSheet) {
			if status, _ := tapLive(t, d, "fake-phone-key", sheet.Ask.ID, sheet.Ask.Code, "allow", callerNamed(t, "phone-wireguard")); status != http.StatusUnauthorized {
				t.Errorf("an Allow from a phone that isn't paired: %d", status)
			}
			runOutSetupAsks(d)
		}, "ranOut", "The ask ran out or Smolcorp restarted, so Setup stayed closed."},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, phoneKey := tapDeck(t)
			withoutWebView2(t)
			askWait = 200 * time.Millisecond // the page's waits, each short, as many as the test takes
			saved := setupComputerName
			setupComputerName = func() string { return "studio-pc" }
			t.Cleanup(func() { setupComputerName = saved })
			// The router reads the phone's system (askFromSystemOf) on every request, the page's too, so it's set here,
			// before the page's server starts, and never while it runs: the phone's half asks through probeAnswerAsSet.
			// probeDeck puts it back in a cleanup, after the deferred server.Close has waited for the server's requests.
			wireguard, wifi := callerNamed(t, "phone-wireguard"), callerNamed(t, "phone-wifi")
			if wireguard.mac != wifi.mac || wireguard.linux != wifi.linux || wireguard.off != wifi.off || wireguard.tailscale != wifi.tailscale {
				t.Fatal("the phone's two callers ask on different systems, and only one can be set before the server starts")
			}
			askFromSystemOf(wireguard)
			server := httptest.NewServer(&router{deck: d})
			defer server.Close()

			page, shown := openTapPage(t, node, server.URL)
			listed := probeAnswerAsSet(t, d, "GET", "/grants", "", phoneKey, callerNamed(t, "phone-wireguard"))
			sheet := phoneSheetFor(t, node, listed.Code, listed.Body.String())
			t.Logf("the page: %q %q; the phone: %+v", shown.Shown.Head, shown.Shown.Code, sheet)
			if shown.Shown.Head != "Approve on your phone: open Smolcorp on iPhone and tap Allow." || shown.Shown.Code != "Code: "+shown.Code ||
				!strings.Contains(shown.Shown.Hint, "never over the Wi-Fi") {
				t.Errorf("the page shows %+v", shown.Shown)
			}
			if sheet == nil || sheet.Ask == nil || sheet.Head != "Open Setup on studio-pc?" || sheet.Code != "Code "+shown.Code || sheet.Ask.Code != shown.Code {
				t.Fatalf("the phone over Tailscale shows %+v for the page's code %s", sheet, shown.Code)
			}

			c.answer(t, d, phoneKey, sheet)
			end := page.end(t)
			session := strings.TrimPrefix(end.Cookie, setupCookie()+"=")
			reads := openWithCookie(t, d, session)
			t.Logf("%s: the page %q, %+v; with its cookie GET /setup/state %d", c.name, end.State, end.End, reads)
			if end.State != c.state {
				t.Fatalf("the page ended %q, want %q", end.State, c.state)
			}
			if c.state != "allowed" {
				if end.Cookie != "" || reads == http.StatusOK || end.End == nil || end.End.Head != c.head || !strings.Contains(end.End.Hint, "Open Setup…") {
					t.Errorf("no Allow, yet the page has cookie %v (Setup %d), and says %+v", end.Cookie != "", reads, end.End)
				}
				return
			}
			if reads != http.StatusOK {
				t.Fatalf("the tapped page reads Setup with %d", reads)
			}
			if changes := withSession(t, d, session, "POST", "/setup/pair", "").Code; changes != http.StatusOK {
				t.Errorf("the tapped page changes Setup with %d", changes)
			}
		})
	}
}

// tapLive is tapSetup while the page's server runs: the caller's system is set already (askFromSystemOf), before the
// server started, so nothing here sets a package variable the server reads.
func tapLive(t *testing.T, d *Deck, phoneKey, id, code, answer string, c routeCaller) (int, string) {
	t.Helper()
	w := probeAnswerAsSet(t, d, "POST", tapSetupTarget(id, code, answer), "", phoneKey, c)
	return w.Code, strings.TrimSpace(w.Body.String())
}

// runOutSetupAsks makes every setup ask run out, as three minutes would.
func runOutSetupAsks(d *Deck) {
	d.setupAsks.mu.Lock()
	defer d.setupAsks.mu.Unlock()
	for _, ask := range d.setupAsks.all {
		ask.asked = ask.asked.Add(-askLifetime - time.Second)
	}
}

// Where no phone can open Setup, the page says what to do instead (SIERRA's interim rule): the safe way back in names
// Smolcorp's real folder on Linux and on Windows (appdirs), and phones.json; it never says to read a key. The locked
// note off a Mac offers the tap, and keeps the Mac's words as they were. docs/windows-linux.md says the way back in word
// for word.
func TestTheWayBackInWithoutAPhoneNamesTheRealFolderAndNoKey(t *testing.T) {
	page := pageSource(t, "setup.html")
	way := pageLine(t, page, "const OFF_MAC_WAY_OUT = ")
	way = page[strings.Index(page, way):]
	way = way[:strings.Index(way, ";\n")]
	for _, want := range []string{"~/.config/" + appdirs.Linux, `%AppData%\\` + appdirs.Windows, "rename phones.json", "Quit Smolcorp from its tray icon",
		"run smolcorp quit", "smolcorp-cli quit on Windows", "That unpairs every phone."} {
		if !strings.Contains(way, want) {
			t.Errorf("the way back in lacks %q:\n%s", want, way)
		}
	}
	if strings.Contains(strings.ToLower(way), "key") {
		t.Errorf("the way back in mentions a key:\n%s", way)
	}
	// The docs say it word for word (TAPASKFINAL's O4): the page's words, and docs/windows-linux.md's paragraph
	// without its marks.
	var said strings.Builder
	unquote := strings.NewReplacer(`\\`, `\`, `\'`, `'`, `\"`, `"`)
	for _, piece := range regexp.MustCompile(`"((?:[^"\\]|\\.)*)"|'((?:[^'\\]|\\.)*)'`).FindAllStringSubmatch(way, -1) {
		said.WriteString(unquote.Replace(piece[1] + piece[2]))
	}
	doc, err := os.ReadFile(filepath.Join("..", "docs", "windows-linux.md"))
	if err != nil {
		t.Fatal(err)
	}
	var documented string
	for _, paragraph := range strings.Split(string(doc), "\n\n") {
		if strings.HasPrefix(paragraph, "Phone lost, ") {
			documented = strings.NewReplacer("**", "", "`", "").Replace(strings.Join(strings.Fields(paragraph), " "))
		}
	}
	if documented != said.String() {
		t.Errorf("docs/windows-linux.md's way back in isn't the page's:\n the docs: %q\n the page: %q", documented, said.String())
	}
	locked := pageFunction(t, page, "locked")
	for _, want := range []string{"Once a phone is paired, Setup opens by a tap on it instead: use Ask my phone below, then tap Allow on it.", "askAgain('Ask my phone'), offMacWayOut()",
		"\"This page opens from Smolcorp's menu bar: choose Open Settings in Browser. \"\n        + 'A link from before Smolcorp last started, as for the home Wi-Fi, no longer works.'"} {
		if !strings.Contains(locked, want) {
			t.Errorf("the locked note lacks %q", want)
		}
	}
	if !strings.Contains(pageFunction(t, page, "handOverKey"), "if (!key && !token && opened.has('tap')) return askThePhone(opened);") {
		t.Error("the page doesn't ask the phone at #tap")
	}
	if !strings.Contains(pageFunction(t, page, "load"), "if (tapping) return;") {
		t.Error("the page's refresh doesn't wait for the tap: it would lock the page while it waits")
	}
}

// setupStartScript is setup.html's own start, in node, opened at hash: the first two steps of the page's start line,
// handOverKey().then(load), with everything they call cut out of the page as it is (handOverKey through
// offMacWayOut, load and what it keeps, setupHeaders), and stand-ins for the DOM. Its fetch keeps the cookie the
// answers set, as a browser does for the page. It prints a line each time the page replaces what it shows, then how
// it ended: whether the start returned or the page waits on its note, `tapping`, every request with its status,
// whether load drew the state, and the cookie.
func setupStartScript(t *testing.T, hash string) string {
	t.Helper()
	page := pageSource(t, "setup.html")
	// The page's start: if it no longer begins with these two steps, this row must follow it.
	pageLine(t, page, "handOverKey().then(load).then(")
	from := strings.Index(page, "\n  async function handOverKey(")
	to := strings.Index(page, "\n  function usedFirst(")
	if from < 0 || to < from {
		t.Fatal("can't cut handOverKey and what it calls out of setup.html: update this test")
	}
	where, _ := json.Marshal(hash)
	return `
const HASH = ` + string(where) + `;
const BASE = process.argv[2];
const say = (line) => console.log(JSON.stringify(line));
let jar = '', rendered = false, lockedWith = null, noted = null;
const asked = [], replaced = [];
const realFetch = globalThis.fetch;
globalThis.fetch = async (path, init = {}) => {
  const { cache, signal, ...rest } = init;
  const res = await realFetch(BASE + path, { ...rest, signal, headers: { ...(rest.headers || {}), ...(jar ? { Cookie: jar } : {}) } });
  for (const cookie of res.headers.getSetCookie()) jar = cookie.split(';')[0];
  asked.push((rest.method || 'GET') + ' ' + path.split('?')[0] + ' ' + res.status);
  return res;
};
const theNote = new Promise((done) => { noted = done; });
const pageMain = {
  childNodes: [{ textContent: 'Setup' }],
  replaceChildren(...nodes) {
    this.childNodes = nodes;
    const shown = nodes.map((node) => node.textContent || '');
    say({ shown });
    if (shown.includes('Ask my phone again')) noted();
  },
};
globalThis.window = {};
globalThis.location = { hash: HASH, pathname: '/setup', search: '', reload() {} };
globalThis.history = { replaceState(_, __, to) { replaced.push(to); } };
globalThis.document = { querySelector: () => pageMain, createElement: (tag) => ({ tag, setAttribute() {} }), body: { dataset: {} } };
function render() { rendered = true; }
function syncGuide() {}
function locked(app) { lockedWith = app || 'locked'; }
function usedFirst() {}
` + pageLine(t, page, "let state = null;") + pageLine(t, page, "let loading = false, loadAgain = false;") +
		pageLine(t, page, "let asks = 0, guideAsked = 0;") + pageFunction(t, page, "load") + pageFunction(t, page, "setupHeaders") +
		page[from:to] + `
(async () => {
  const pause = (ms) => new Promise((done) => setTimeout(done, ms));
  const ended = await Promise.race([
    handOverKey().then(load).then(() => 'returned'),
    // On its note the page waits for good: given a moment, anything that wrongly follows would show.
    theNote.then(() => pause(500)).then(() => 'waiting'),
    pause(120000).then(() => 'stuck'),
  ]);
  say({ ended, tapping, asked, rendered, locked: lockedWith, replaced, cookie: jar });
  process.exit(0);
})().catch((e) => { console.error(e); process.exit(1); });
`
}

// setupStartLine is a line setupStartScript prints: what the page shows, or, with ended set, how its start ended.
type setupStartLine struct {
	Shown    []string
	Ended    string
	Tapping  bool
	Asked    []string
	Rendered bool
	Locked   *string
	Replaced []string
	Cookie   string
}

// setupStarted is setup.html's start running in node.
type setupStarted struct{ *nodePage }

// startSetupAt runs setup.html's start in node, opened at hash, against a live server of d's router on 127.0.0.1,
// as Linux, or Windows without WebView2. d must be tapDeck's: this sets the paired phone's system over Tailscale
// (askFromSystemOf) before the server starts, and probeDeck's cleanup puts it back after the server is closed. While
// it runs, nothing sets a package variable the router reads: the phone asks through probeAnswerAsSet.
func startSetupAt(t *testing.T, node string, d *Deck, hash string) *setupStarted {
	t.Helper()
	withoutWebView2(t)
	askWait = 200 * time.Millisecond // tapDeck's shortAskWait puts it back
	askFromSystemOf(callerNamed(t, "phone-wireguard"))
	server := httptest.NewServer(&router{deck: d})
	t.Cleanup(server.Close)
	file := filepath.Join(t.TempDir(), "start.js")
	if err := os.WriteFile(file, []byte(setupStartScript(t, hash)), 0o600); err != nil {
		t.Fatal(err)
	}
	// Its cleanup is registered after the server's Close, so it runs first: node is gone before the server closes.
	return &setupStarted{runInNode(t, node, file, server.URL)}
}

// line is the next line the page's start printed.
func (s *setupStarted) line(t *testing.T) setupStartLine {
	t.Helper()
	var line setupStartLine
	s.next(t, "the page's start", &line)
	return line
}

// end is how the page's start ended, and the last thing it showed before.
func (s *setupStarted) end(t *testing.T) (setupStartLine, []string) {
	t.Helper()
	var last []string
	for {
		line := s.line(t)
		if line.Ended != "" {
			return line, last
		}
		last = line.Shown
	}
}

// tapAtSetupsStart runs setup.html's own start at hash, as the menu opens it, with the phone's answer once the page
// shows its ask: "allow" or "deny" from the paired phone over Tailscale, with the code its sheet shows, or "ranOut".
// It says what the page showed while it waited (nil if it never asked), how its start ended, and what it showed last.
func tapAtSetupsStart(t *testing.T, node string, d *Deck, phoneKey, hash, answer string) ([]string, setupStartLine, []string) {
	t.Helper()
	started := startSetupAt(t, node, d, hash)
	first := started.line(t)
	if first.Ended != "" {
		return nil, first, nil
	}
	listed := probeAnswerAsSet(t, d, "GET", "/grants", "", phoneKey, callerNamed(t, "phone-wireguard"))
	sheet := phoneSheetFor(t, node, listed.Code, listed.Body.String())
	if sheet == nil || sheet.Ask == nil || len(first.Shown) < 2 || first.Shown[1] != "Code: "+sheet.Ask.Code {
		t.Fatalf("the page shows %q; the phone over Tailscale, %+v", first.Shown, sheet)
	}
	if answer == "ranOut" {
		runOutSetupAsks(d)
	} else if status, said := tapLive(t, d, phoneKey, sheet.Ask.ID, sheet.Ask.Code, answer, callerNamed(t, "phone-wireguard")); status != http.StatusOK {
		t.Fatalf("the phone's %s: %d %s", answer, status, said)
	}
	end, last := started.end(t)
	return first.Shown, end, last
}

// askedAfter is what the page asked after its first request that starts with from: nil if it never asked that.
func askedAfter(asked []string, from string) []string {
	for i, call := range asked {
		if strings.HasPrefix(call, from) {
			return asked[i+1:]
		}
	}
	return nil
}

// The page's own way in at #tap (RT126FIXREVIEW-SETUPTAPR1's N1): setup.html's start, as the menu opens it off a
// Mac once a phone is paired, runs in node from handOverKey on. It sends POST /setup/tap and shows the code; on the
// phone's Allow, `tapping` is false and load goes on to GET /setup/state, 200, and draws Setup. On Deny, or when the
// ask runs out, it shows the plain words, how to ask again and the way back in without a phone, and asks nothing
// more: no session, no state. Fail-first: the base's page never asks at #tap (M2), and one that keeps `tapping` after
// the Allow (M1) never loads.
func TestSetupsOwnStartAtTapAsksThePhoneAndLoadsOnlyOnItsAllow(t *testing.T) {
	node := nodeOrSkip(t)
	for _, c := range []struct{ answer, head string }{
		{"allow", ""},
		{"deny", "You tapped Deny on your phone, so Setup stayed closed."},
		{"ranOut", "The ask ran out or Smolcorp restarted, so Setup stayed closed."},
	} {
		t.Run(c.answer, func(t *testing.T) {
			d, phoneKey := tapDeck(t)
			waited, end, last := tapAtSetupsStart(t, node, d, phoneKey, "#"+setupTapFragment, c.answer)
			t.Logf("%s: the page showed %q; then its start %s, tapping %v, asked %v, drew Setup %v, locked %v, showed %q",
				c.answer, waited, end.Ended, end.Tapping, end.Asked, end.Rendered, end.Locked, last)
			if waited == nil {
				t.Fatalf("the page at #tap never asked the phone: its start %s after %v", end.Ended, end.Asked)
			}
			if askedAfter(end.Asked, "POST /setup/tap ") == nil || !strings.HasPrefix(waited[0], "Approve on your phone: open Smolcorp on iPhone") {
				t.Errorf("the page at #tap didn't send POST /setup/tap and show its ask: %v %q", end.Asked, waited)
			}
			if end.Locked != nil {
				t.Errorf("the page was locked (%s)", *end.Locked)
			}
			loaded := askedAfter(askedAfter(end.Asked, "POST /setup/tap "), "GET /setup/state ")
			if c.answer == "allow" {
				if end.Ended != "returned" || end.Tapping || loaded == nil || !end.Rendered || end.Cookie == "" {
					t.Fatalf("after the Allow: the start %s, tapping %v, asked %v, drew Setup %v, cookie %v; want returned, false, "+
						"GET /setup/state 200, true, a cookie", end.Ended, end.Tapping, end.Asked, end.Rendered, end.Cookie != "")
				}
				tapped := askedAfter(end.Asked, "POST /setup/tap ")
				if state := slices.IndexFunc(tapped, func(call string) bool { return strings.HasPrefix(call, "GET /setup/state ") }); tapped[state] != "GET /setup/state 200" {
					t.Errorf("after the Allow, load asked %q, want GET /setup/state 200", tapped[state])
				}
				if !slices.Equal(last, []string{"Setup"}) || !slices.Contains(end.Replaced, "/setup") {
					t.Errorf("after the Allow, the page shows %q and its address went %v", last, end.Replaced)
				}
				return
			}
			if end.Ended != "waiting" || !end.Tapping || loaded != nil || end.Rendered || end.Cookie != "" {
				t.Errorf("after %s: the start %s, tapping %v, asked %v, drew Setup %v, cookie %v; want waiting on its note, no "+
					"state asked, no cookie", c.answer, end.Ended, end.Tapping, end.Asked, end.Rendered, end.Cookie != "")
			}
			if len(last) != 4 || last[0] != c.head || !strings.Contains(last[1], "Open Setup…") || last[2] != "Ask my phone again" ||
				!strings.HasPrefix(last[3], "Phone lost, or it can't reach this computer over Tailscale?") {
				t.Errorf("after %s, the page shows %q", c.answer, last)
			}
		})
	}
}

// The phone's page looks for asks to open Setup as it starts, and every SETUP_ASK_EVERY after (RT126FIXREVIEW-
// SETUPTAPR1's N1: without it no sheet ever shows, M3). Its look stops for good where no ask can ever be (O2): a Mac's
// GET /grants has no setupAsks, while a computer off a Mac lists them, even with none waiting. Stopped, it doesn't
// look again as the page comes back into view (the R2 review's O1), on a Mac or after a 404. With two asks waiting,
// the sheet shows the oldest and says another waits too (O1).
func TestThePhonesPageLooksForSetupAsksFromItsStartAndOnlyWhereThereCanBeOne(t *testing.T) {
	node := nodeOrSkip(t)
	page := pageSource(t, "deck.html")
	at := strings.Index(page, "\n  if (whole) document.title")
	if at < 0 {
		t.Fatal("deck.html's start moved: update this test")
	}
	var start []string
	for _, line := range strings.Split(page[at:], "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "*") {
			start = append(start, trimmed)
		}
	}
	for _, want := range []string{"checkSetupAsks();", "setupAskTimer = setInterval(checkSetupAsks, SETUP_ASK_EVERY);"} {
		if !slices.Contains(start, want) {
			t.Errorf("deck.html's start never runs %q", want)
		}
	}

	// The page's return to view looks again (O1 of the R2 review: not once it has stopped for good, below).
	if !slices.Contains(start, "document.addEventListener('visibilitychange', () => { if (!document.hidden) checkSetupAsks(); });") {
		t.Error("deck.html's start no longer looks for setup asks as the page comes back into view: update this test")
	}

	// checkSetupAsks itself, in node, on what each computer's GET /grants answered the phone over Tailscale: as the
	// page starts, then again as it comes back into view. It says how often it asked.
	look := func(status int, body string) (stopped bool, sheet string, asked int) {
		dir := t.TempDir()
		script, answer := filepath.Join(dir, "look.js"), filepath.Join(dir, "answer")
		source := `
let stopped = false, shown = null, asked = 0;
const preview = false, document = { hidden: false };
const clearInterval = () => { stopped = true; };
const showSetupAsk = (sheet) => { shown = sheet; };
const get = async () => { asked++; return { status: Number(process.argv[2]), text: async () => require('fs').readFileSync(process.argv[3], 'utf8') }; };
` + pageLine(t, page, "const SETUP_ASK_NOT_HERE = ") + pageLine(t, page, "let setupAskTimer = null;") + pageLine(t, page, "let setupAskOn = null;") +
			pageLine(t, page, "let setupAskClosed = '';") + pageLine(t, page, "let setupAsksOff = false;") + pageFunction(t, page, "setupAskSheetFor") +
			pageFunction(t, page, "setupAsksHere") + pageFunction(t, page, "checkSetupAsks") + `
checkSetupAsks().then(checkSetupAsks).then(() => console.log(JSON.stringify({ stopped, shown, asked })));
`
		if os.WriteFile(script, []byte(source), 0o600) != nil || os.WriteFile(answer, []byte(body), 0o600) != nil {
			t.Fatal("can't write the phone's script")
		}
		out, err := exec.Command(node, script, strconv.Itoa(status), answer).CombinedOutput()
		var said struct {
			Stopped bool
			Shown   json.RawMessage
			Asked   int
		}
		if err != nil || json.Unmarshal(out, &said) != nil {
			t.Fatalf("the phone's look: %v %q", err, out)
		}
		return said.Stopped, string(said.Shown), said.Asked
	}

	t.Run("a Mac", func(t *testing.T) {
		d, phoneKey := tapDeck(t)
		hostedByTheMacApp(d)
		w := probeAnswerWith(t, d, "GET", "/grants", "", phoneKey, callerNamed(t, "phone-wireguard"))
		stopped, sheet, asked := look(w.Code, w.Body.String())
		t.Logf("a Mac: GET /grants %d %s; the phone stopped looking %v, shows %s, asked %d times", w.Code, w.Body, stopped, sheet, asked)
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "setupAsks") || !stopped || sheet != "null" || asked != 1 {
			t.Errorf("on a Mac the phone keeps looking for asks to open Setup: %d %s, stopped %v, %s, asked %d times (once, and "+
				"never again on a return to the page)", w.Code, w.Body, stopped, sheet, asked)
		}
	})
	t.Run("a computer without the route", func(t *testing.T) {
		if stopped, sheet, asked := look(http.StatusNotFound, "404 page not found"); !stopped || sheet != "null" || asked != 1 {
			t.Errorf("after a 404 the phone stopped %v, shows %s, asked %d times (once, and never again on a return to the page)", stopped, sheet, asked)
		}
	})
	t.Run("off a Mac", func(t *testing.T) {
		d, phoneKey := tapDeck(t)
		withoutWebView2(t)
		saved := setupComputerName
		setupComputerName = func() string { return "studio-pc" }
		t.Cleanup(func() { setupComputerName = saved })
		w := probeAnswerWith(t, d, "GET", "/grants", "", phoneKey, callerNamed(t, "phone-wireguard"))
		if stopped, sheet, asked := look(w.Code, w.Body.String()); w.Code != http.StatusOK || stopped || sheet != "null" || asked != 2 {
			t.Errorf("off a Mac with no ask waiting: %d %s, stopped %v, shows %s, asked %d times (want 2: it keeps looking)", w.Code, w.Body, stopped, sheet, asked)
		}
		for range 2 {
			if code, said := askToOpenSetup(t, d); code != http.StatusOK {
				t.Fatalf("an ask: %d %v", code, said)
			}
		}
		w = probeAnswerWith(t, d, "GET", "/grants", "", phoneKey, callerNamed(t, "phone-wireguard"))
		stopped, _, _ := look(w.Code, w.Body.String())
		sheet := phoneSheetFor(t, node, w.Code, w.Body.String())
		asks, _ := phoneSetupAsks(t, d, phoneKey)
		t.Logf("off a Mac, two asks: stopped %v, the sheet %+v", stopped, sheet)
		if stopped || sheet == nil || sheet.Ask == nil || len(asks) != 2 || sheet.Ask.ID != asks[0]["id"] ||
			sheet.More != "Another ask is waiting too, so check the code before you tap Allow." {
			t.Errorf("with two asks waiting, the phone stopped %v and shows %+v", stopped, sheet)
		}
		runOutSetupAsks(d)
		if code, _ := askToOpenSetup(t, d); code != http.StatusOK {
			t.Fatal("a third ask was refused")
		}
		w = probeAnswerWith(t, d, "GET", "/grants", "", phoneKey, callerNamed(t, "phone-wireguard"))
		if sheet := phoneSheetFor(t, node, w.Code, w.Body.String()); sheet == nil || sheet.More != "" {
			t.Errorf("with one ask waiting, the sheet says %+v", sheet)
		}
	})
}

// phoneSheetScript is deck.html's own sheet for asks to open Setup, in node: its look (checkSetupAsks), the sheet
// it draws (showSetupAsk), its answer (answerSetupAsk) and its buttons, as the page wires them. The DOM, flash and
// tick are stand-ins. Its get and post print each request, and wait for the row to send it as the paired phone over
// Tailscale and to answer with what the computer said. It prints the sheet as drawn, taps the button named by its
// argument, and prints the sheet and the words flashed once the answer and the look after it are done.
func phoneSheetScript(t *testing.T) string {
	t.Helper()
	return phoneSheetPage(t) + `
const BUTTON = process.argv[2];
(async () => {
  await checkSetupAsks();
  say({ sheet: drawn() });
  const button = $('setup-ask-' + BUTTON);
  if (!button.hidden && typeof button.onclick === 'function') await button.onclick();
  // answerSetupAsk looks again as it ends, without waiting for that look: it's let finish too.
  await settled();
  say({ done: { sheet: drawn(), flashed, ticks } });
  process.exit(0);
})().catch((e) => { console.error(e); process.exit(1); });
`
}

// phoneSheetPage is phoneSheetScript's page, without what it does: deck.html's own sheet and its stand-ins, say,
// drawn (the sheet as drawn) and settled (every request answered, and the page's work after them done).
func phoneSheetPage(t *testing.T) string {
	t.Helper()
	page := pageSource(t, "deck.html")
	return `
const say = (line) => console.log(JSON.stringify(line));
const answers = [], pending = new Set();
require('readline').createInterface({ input: process.stdin }).on('line', (line) => answers.shift()(JSON.parse(line)));
function relay(method, path, params = {}) {
  const answered = new Promise((done) => answers.push(done))
    .then(({ status, body }) => ({ ok: status >= 200 && status < 300, status, text: async () => body }));
  pending.add(answered);
  answered.then(() => pending.delete(answered));
  say({ ask: { method, path, params } });
  return answered;
}
const preview = false, document = { hidden: false };
const elements = {};
const $ = (id) => elements[id] || (elements[id] = { id, textContent: '', hidden: id === 'setup-ask' || id === 'setup-ask-more', onclick: null });
const flashed = [];
let ticks = 0;
function flash(text) { flashed.push(text); }
function tick() { ticks++; }
const clearInterval = () => {};
const get = (path, params) => relay('GET', path, params);
const post = (path, params) => relay('POST', path, params);
` + pageLine(t, page, "const SETUP_ASK_NOT_HERE = ") + pageLine(t, page, "let setupAskTimer = null;") + pageLine(t, page, "let setupAskOn = null;") +
		pageLine(t, page, "let setupAskClosed = '';") + pageLine(t, page, "let setupAsksOff = false;") +
		pageLine(t, page, "const SETUP_ASK_SETTLE = ") + pageLine(t, page, "let setupAskSwappedAt = ") +
		pageLine(t, page, "let setupAskLastID = ") + pageFunction(t, page, "setupAskSheetFor") +
		pageFunction(t, page, "setupAsksHere") + pageFunction(t, page, "showSetupAsk") + pageFunction(t, page, "checkSetupAsks") +
		pageFunction(t, page, "answerSetupAsk") + pageFunction(t, page, "problemText") + pageLine(t, page, "$('setup-ask-allow').onclick = ") +
		pageLine(t, page, "$('setup-ask-deny').onclick = ") + pageLine(t, page, "$('setup-ask-close').onclick = ") + `
const drawn = () => ({ shown: !$('setup-ask').hidden, head: $('setup-ask-head').textContent, code: $('setup-ask-code').textContent,
  codeShown: !$('setup-ask-code').hidden, why: $('setup-ask-why').textContent, allow: !$('setup-ask-allow').hidden, deny: !$('setup-ask-deny').hidden });
async function settled() {
  while (pending.size) {
    await Promise.all([...pending]);
    await new Promise((done) => setTimeout(done, 0));
  }
}
`
}

// phoneSheetDrawn is the phone's sheet as showSetupAsk drew it.
type phoneSheetDrawn struct {
	Shown           bool
	Head, Code, Why string
	CodeShown       bool
	Allow, Deny     bool
}

// phoneRequest is a request the phone's page sent, and what the computer answered it.
type phoneRequest struct {
	Method, Path string
	Params       map[string]string
	Status       int
	Body         string
}

// phoneTapped is the phone's sheet before the tap, the requests the page sent, and the sheet, the words flashed and
// the ticks after it.
type phoneTapped struct {
	Before, After phoneSheetDrawn
	Sent          []phoneRequest
	Flashed       []string
	Ticks         int
}

// tapOnThePhonesSheet runs phoneSheetScript and taps button ("allow" or "deny"). Each request the page sends goes
// to d as the paired phone over Tailscale, through probeAnswerAsSet, which sets no package variable: a live server
// of d's router may be running meanwhile (startSetupAt).
func tapOnThePhonesSheet(t *testing.T, node string, d *Deck, phoneKey, button string) phoneTapped {
	t.Helper()
	file := filepath.Join(t.TempDir(), "sheet.js")
	if err := os.WriteFile(file, []byte(phoneSheetScript(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	page := runInNode(t, node, file, button)
	var tapped phoneTapped
	for {
		var line struct {
			Ask   *phoneRequest
			Sheet *phoneSheetDrawn
			Done  *struct {
				Sheet   phoneSheetDrawn
				Flashed []string
				Ticks   int
			}
		}
		page.next(t, "the phone's sheet", &line)
		switch {
		case line.Ask != nil:
			tapped.Sent = append(tapped.Sent, answerThePhonesPage(t, d, phoneKey, page, *line.Ask))
		case line.Sheet != nil:
			tapped.Before = *line.Sheet
		case line.Done != nil:
			tapped.After, tapped.Flashed, tapped.Ticks = line.Done.Sheet, line.Done.Flashed, line.Done.Ticks
			return tapped
		}
	}
}

// answerThePhonesPage sends a request the phone's page printed to d, as the paired phone over Tailscale, through
// probeAnswerAsSet, and writes the computer's status and body back to the page's input. It returns the request with
// its answer.
func answerThePhonesPage(t *testing.T, d *Deck, phoneKey string, page *nodePage, sent phoneRequest) phoneRequest {
	t.Helper()
	query := url.Values{}
	for name, value := range sent.Params {
		query.Set(name, value)
	}
	w := probeAnswerAsSet(t, d, sent.Method, sent.Path+"?"+query.Encode(), "", phoneKey, callerNamed(t, "phone-wireguard"))
	sent.Status, sent.Body = w.Code, w.Body.String()
	answer, _ := json.Marshal(map[string]any{"status": w.Code, "body": w.Body.String()})
	if _, err := page.stdin.Write(append(answer, '\n')); err != nil {
		t.Fatalf("can't answer the phone's page: %v", err)
	}
	return sent
}

// The phone's own half of the tap (RT126FIXREVIEW-SETUPTAPR2's N1), both pages' own code, end to end: setup.html's
// start at #tap asks and shows its code; deck.html's own look, sheet and buttons, on what GET /grants answered the
// paired phone over Tailscale, draw that same code, and the button tapped sends POST /grants/setup with the sheet's
// id and code and its own answer. The Allow then opens Setup for that page, which draws it with its cookie; the Deny
// leaves it waiting on its note, with no session. Fail-first: a sheet that never draws the code (M5), or a Deny
// button that sends allow (M4), each fail here.
func TestThePhonesOwnSheetShowsThePagesCodeAndSendsItsOwnAnswer(t *testing.T) {
	node := nodeOrSkip(t)
	for _, c := range []struct{ button, flashed, head string }{
		{"allow", "Setup opens on studio-pc.", ""},
		{"deny", "Setup stays closed.", "You tapped Deny on your phone, so Setup stayed closed."},
	} {
		t.Run(c.button, func(t *testing.T) {
			d, phoneKey := tapDeck(t)
			// Read by the server: set before startSetupAt starts it, and put back after it's closed.
			saved := setupComputerName
			setupComputerName = func() string { return "studio-pc" }
			t.Cleanup(func() { setupComputerName = saved })
			started := startSetupAt(t, node, d, "#"+setupTapFragment)
			first := started.line(t)
			if first.Ended != "" || len(first.Shown) < 2 || !strings.HasPrefix(first.Shown[1], "Code: ") {
				t.Fatalf("the page at #tap shows no code: %q, its start %s after %v", first.Shown, first.Ended, first.Asked)
			}
			code := strings.TrimPrefix(first.Shown[1], "Code: ")

			tapped := tapOnThePhonesSheet(t, node, d, phoneKey, c.button)
			t.Logf("the page's code %s; the phone drew %+v, sent %+v, then drew %+v, flashed %q", code, tapped.Before, tapped.Sent, tapped.After, tapped.Flashed)
			if tapped.Before.Code != "Code "+code || !tapped.Before.CodeShown {
				t.Errorf("the phone's sheet draws the code %q (shown %v), but the page shows %s", tapped.Before.Code, tapped.Before.CodeShown, code)
			}
			if !tapped.Before.Shown || tapped.Before.Head != "Open Setup on studio-pc?" || !tapped.Before.Allow || !tapped.Before.Deny {
				t.Errorf("the phone's sheet: %+v", tapped.Before)
			}
			if len(tapped.Sent) == 0 || tapped.Sent[0].Method != "GET" || tapped.Sent[0].Path != "/grants" || tapped.Sent[0].Status != http.StatusOK {
				t.Fatalf("the phone didn't look with GET /grants first: %+v", tapped.Sent)
			}
			var listed struct {
				SetupAsks []struct{ ID, Code string } `json:"setupAsks"`
			}
			if json.Unmarshal([]byte(tapped.Sent[0].Body), &listed) != nil || len(listed.SetupAsks) != 1 || listed.SetupAsks[0].Code != code {
				t.Fatalf("GET /grants answered the phone %s, for the page's code %s", tapped.Sent[0].Body, code)
			}
			var answers []phoneRequest
			for _, sent := range tapped.Sent {
				if sent.Method == "POST" {
					answers = append(answers, sent)
				}
			}
			want := map[string]string{"id": listed.SetupAsks[0].ID, "code": code, "answer": c.button}
			if len(answers) != 1 || answers[0].Path != "/grants/setup" || !maps.Equal(answers[0].Params, want) || answers[0].Status != http.StatusOK {
				t.Fatalf("the phone's %s sent %+v, want one POST /grants/setup with %v, answered 200", c.button, answers, want)
			}
			if tapped.After.Shown || !slices.Equal(tapped.Flashed, []string{c.flashed}) || tapped.Ticks != 1 {
				t.Errorf("after the %s the phone shows the sheet %v, flashed %q, ticked %d times", c.button, tapped.After.Shown, tapped.Flashed, tapped.Ticks)
			}

			end, last := started.end(t)
			t.Logf("the page then: its start %s, tapping %v, asked %v, drew Setup %v, showed %q", end.Ended, end.Tapping, end.Asked, end.Rendered, last)
			loaded := askedAfter(askedAfter(end.Asked, "POST /setup/tap "), "GET /setup/state ")
			if c.button == "allow" {
				if end.Ended != "returned" || end.Tapping || loaded == nil || !end.Rendered || end.Cookie == "" ||
					openWithCookie(t, d, strings.TrimPrefix(end.Cookie, setupCookie()+"=")) != http.StatusOK {
					t.Errorf("after the phone's Allow the page's start %s, tapping %v, asked %v, drew Setup %v, cookie %v: Setup didn't open",
						end.Ended, end.Tapping, end.Asked, end.Rendered, end.Cookie != "")
				}
				return
			}
			if end.Ended != "waiting" || !end.Tapping || loaded != nil || end.Rendered || end.Cookie != "" || len(last) == 0 || last[0] != c.head {
				t.Errorf("after the phone's Deny the page's start %s, tapping %v, asked %v, drew Setup %v, cookie %v, says %q",
					end.Ended, end.Tapping, end.Asked, end.Rendered, end.Cookie != "", last)
			}
		})
	}
}

// The phone's sheet lets go of a tap right after it drew another ask than the one it last showed (RT126FIXREVIEW-
// SETUPTAPR3's O2, and TAPASKFINAL's N1): the code changed under the owner's thumb, so that tap was meant for the ask
// before. Two asks wait, and the sheet shows the older. Then the sheet draws the newer, with its own code:
//   - a look swaps it, as the older was answered elsewhere just before;
//   - the older was answered elsewhere just before the owner's Allow, which then fails (404), and the tap's own look
//     draws the newer: a second tap then, as the first "didn't work", would allow a code nobody compared;
//   - the owner's Allow answers the older, and the tap's own look draws the newer: a double tap.
//
// A tap at once sends nothing; a tap once SETUP_ASK_SETTLE has passed answers the newer, with its own id and code. A
// tap on a fresh sheet's first ask still goes through. Fail-first: a guard that looks only at a swap of a shown ask
// (afe004f5's) lets the second tap of both tap cases through; one with no guard at all lets all three through.
func TestThePhonesSheetLetsGoOfATapRightAfterItSwapsTheAsk(t *testing.T) {
	node := nodeOrSkip(t)
	// The guard's clock is the page's own: the phone's time can step back, and a guard on it would then hold every tap.
	page := pageSource(t, "deck.html")
	for _, name := range []string{"showSetupAsk", "answerSetupAsk"} {
		if strings.Contains(pageFunction(t, page, name), "Date.now(") {
			t.Errorf("deck.html's %s times the guard with the phone's time, which can step back: use performance.now()", name)
		}
	}
	saved := setupComputerName
	setupComputerName = func() string { return "studio-pc" }
	t.Cleanup(func() { setupComputerName = saved })

	// sheetDriver draws the first look's sheet, then step (which leaves the sheet on the newer ask), then taps Allow at
	// once, then again once the guard has passed. Without step it taps the first sheet at once, and that's all.
	sheetDriver := func(step string) string {
		if step == "" {
			return `
(async () => {
  await checkSetupAsks();
  say({ sheet: drawn() });
  await $('setup-ask-allow').onclick();
  say({ tapped: 'at once' });
  await settled();
  say({ done: { sheet: drawn(), flashed, ticks } });
  process.exit(0);
})().catch((e) => { console.error(e); process.exit(1); });
`
		}
		return `
(async () => {
  await checkSetupAsks();
  say({ sheet: drawn() });
  ` + step + `
  await settled();
  say({ sheet: drawn() });
  await $('setup-ask-allow').onclick();
  say({ tapped: 'at once' });
  await settled();
  await new Promise((done) => setTimeout(done, SETUP_ASK_SETTLE + 250));
  await $('setup-ask-allow').onclick();
  await settled();
  say({ done: { sheet: drawn(), flashed, ticks } });
  process.exit(0);
})().catch((e) => { console.error(e); process.exit(1); });
`
	}
	const (
		byALook = "await checkSetupAsks();"
		byATap  = "await $('setup-ask-allow').onclick();"
	)
	for _, c := range []struct {
		name, step string
		// elsewhere is the page's request before which the older ask is denied elsewhere: "GET 2" is the page's second
		// look, "POST 1" its first answer. Empty: nothing is answered elsewhere.
		elsewhere string
		// first is the status of the answer sent before the tap at once, if any (0: none).
		first int
	}{
		{"a look swaps the ask", byALook, "GET 2", 0},
		{"the shown ask is gone when it's tapped", byATap, "POST 1", http.StatusNotFound},
		{"a double tap after an Allow while another waits", byATap, "", http.StatusOK},
		{"a fresh sheet's first tap", "", "", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, phoneKey := tapDeck(t)
			withoutWebView2(t)
			for range 2 {
				if code, said := askToOpenSetup(t, d); code != http.StatusOK {
					t.Fatalf("an ask: %d %v", code, said)
				}
			}
			asks, body := phoneSetupAsks(t, d, phoneKey)
			if len(asks) != 2 {
				t.Fatalf("two asks should wait: %s", body)
			}
			older, newer := asks[0], asks[1]
			olderID, _ := older["id"].(string)
			olderCode, _ := older["code"].(string)
			newerID, _ := newer["id"].(string)
			newerCode, _ := newer["code"].(string)

			file := filepath.Join(t.TempDir(), "swap.js")
			if err := os.WriteFile(file, []byte(phoneSheetPage(t)+sheetDriver(c.step)), 0o600); err != nil {
				t.Fatal(err)
			}
			page := runInNode(t, node, file)
			var sheets []phoneSheetDrawn
			var posts []phoneRequest
			sent, postsAtOnce := map[string]int{}, -1
			var flashed []string
			for done := false; !done; {
				var line struct {
					Ask    *phoneRequest
					Sheet  *phoneSheetDrawn
					Tapped string
					Done   *struct {
						Sheet   phoneSheetDrawn
						Flashed []string
					}
				}
				page.next(t, "the phone's sheet", &line)
				switch {
				case line.Ask != nil:
					sent[line.Ask.Method]++
					if c.elsewhere == fmt.Sprintf("%s %d", line.Ask.Method, sent[line.Ask.Method]) {
						if status, said := tapLive(t, d, phoneKey, olderID, olderCode, "deny", callerNamed(t, "phone-wireguard")); status != http.StatusOK {
							t.Fatalf("the older ask's Deny elsewhere: %d %s", status, said)
						}
					}
					if answered := answerThePhonesPage(t, d, phoneKey, page, *line.Ask); answered.Method == "POST" {
						posts = append(posts, answered)
					}
				case line.Sheet != nil:
					sheets = append(sheets, *line.Sheet)
				case line.Tapped != "":
					postsAtOnce = len(posts)
				case line.Done != nil:
					flashed, done = line.Done.Flashed, true
				}
			}
			t.Logf("the sheets %+v; sent before the tap at once %d; in all %+v; flashed %q", sheets, postsAtOnce, posts, flashed)

			opens := "Setup opens on studio-pc."
			if c.step == "" {
				want := map[string]string{"id": olderID, "code": olderCode, "answer": "allow"}
				if len(sheets) != 1 || sheets[0].Code != "Code "+olderCode || !sheets[0].Allow {
					t.Fatalf("the sheet should show %s first: %+v", olderCode, sheets)
				}
				if len(posts) != 1 || posts[0].Path != "/grants/setup" || !maps.Equal(posts[0].Params, want) || posts[0].Status != http.StatusOK {
					t.Errorf("a tap at once on a fresh sheet should send one POST /grants/setup with %v, answered 200: %+v", want, posts)
				}
				if !slices.Equal(flashed, []string{opens}) {
					t.Errorf("after the tap the phone flashed %q", flashed)
				}
				return
			}
			if len(sheets) != 2 || sheets[0].Code != "Code "+olderCode || sheets[1].Code != "Code "+newerCode || !sheets[1].Allow {
				t.Fatalf("the sheet should show %s, then swap to %s: %+v", olderCode, newerCode, sheets)
			}
			before := 0
			if c.first != 0 {
				before = 1
				want := map[string]string{"id": olderID, "code": olderCode, "answer": "allow"}
				if len(posts) == 0 || !maps.Equal(posts[0].Params, want) || posts[0].Status != c.first {
					t.Fatalf("the first tap should send %v, answered %d: %+v", want, c.first, posts)
				}
			}
			if postsAtOnce != before {
				t.Errorf("a tap at once after the sheet swapped to another ask sent %d answer(s), want %d: %+v", postsAtOnce, before, posts)
			}
			want := map[string]string{"id": newerID, "code": newerCode, "answer": "allow"}
			if len(posts) != before+1 || posts[before].Path != "/grants/setup" || !maps.Equal(posts[before].Params, want) || posts[before].Status != http.StatusOK {
				t.Errorf("a tap after the guard should send one POST /grants/setup with %v, answered 200: %+v", want, posts)
			}
			if len(flashed) != before+1 || flashed[before] != opens {
				t.Errorf("after the later tap the phone flashed %q", flashed)
			}
		})
	}
}
