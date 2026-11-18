//go:build !windows

package desktop

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// `smolcorp demo`'s own words, and --no-open, where its page really opens (FXDEMOMACR1): the command runs against a
// real Smolcorp on each system, played in-process with stand-ins (no window and no browser opens). Under the Mac app
// and on Windows with its window, the page opens in Smolcorp's own Settings window (openDemoPage), so the command says
// so, and refuses --no-open, whose link would open in a browser whose Setup only looks there. On Linux and on a
// Windows without WebView2 it opens in the browser, as before, and --no-open prints its link as before.

// demoSystem is a system the demo's command runs on, with where its page opens there.
type demoSystem struct {
	name, goos string
	window     bool // Smolcorp has a Settings window of its own there
}

var demoSystems = []demoSystem{
	{"the Mac app", "darwin", true},
	{"Windows with its window", "windows", true},
	{"Windows without WebView2", "windows", false},
	{"Linux", "linux", false},
}

// demoOpens is where a real Smolcorp on one system opened the demo's page: what the Mac app's bridge, Windows' window
// and the browser were asked to open.
type demoOpens struct {
	mac    *fakeHost
	window *standInSettings
	links  *[]string
}

func (o demoOpens) seen() (macWindow []string, windowRuns int, links []string) {
	if o.mac != nil {
		for _, asked := range o.mac.asked {
			if asked.Action == "settings" {
				macWindow = append(macWindow, bridgeThen(asked))
			}
		}
	}
	if o.window != nil {
		runs, forwards := o.window.seen()
		windowRuns = len(runs) + len(forwards)
	}
	return macWindow, windowRuns, *o.links
}

// demoOnSystem is a real Smolcorp (a Deck, served on an ephemeral port) as it is on s, and the command's team aimed
// at it, holding its tool's key as the demo does once you click Allow.
func demoOnSystem(t *testing.T, s demoSystem) (*Deck, *officeTeam, demoOpens) {
	t.Helper()
	d, _, _, _ := gmHireDeck(t)
	saved := setupSystem
	setupSystem = s.goos
	t.Cleanup(func() { setupSystem = saved })
	var opens demoOpens
	switch {
	case s.goos == "darwin":
		withoutWebView2(t)
		opens.mac = &fakeHost{answer: bridgeAnswer{OK: true}}
		d.mu.Lock()
		d.mac.host = opens.mac
		d.mu.Unlock()
	case s.window:
		opens.window = onWindowsWithWebView2(t)
	default:
		withoutWebView2(t)
	}
	opens.links = browserOpens(t)
	server := httptest.NewServer(&router{deck: d})
	t.Cleanup(server.Close)
	team := &officeTeam{base: server.URL, key: func() ([]byte, error) { return []byte(d.access.SetupKey), nil }, http: server.Client(),
		start: func() error { return errors.New("started the app while it was running") }, every: 5 * time.Millisecond, wait: 200 * time.Millisecond}
	return d, team, opens
}

// runDemoOn runs the real demo with team, your answer to its question being yes, nobody clicking Hire on its page.
func runDemoOn(t *testing.T, team *officeTeam) (int, string, string) {
	t.Helper()
	in := demoTestFolder(t)
	t.Cleanup(func() {
		demoPages.Lock()
		delete(demoPages.at, filepath.Join(in, demoFolderName))
		demoPages.Unlock()
	})
	var out bytes.Buffer
	code := runDemo(context.Background(), demoRun{in: in, team: team, stdin: strings.NewReader("y\n"), out: &out})
	return code, out.String(), in
}

const (
	demoAsksForTheWindow  = "After your yes, the demo makes its folder, and Smolcorp's Settings window\nopens at the demo: check it there, and click Hire"
	demoAsksForTheBrowser = "After your yes, the demo makes its folder, and Setup opens in your browser at\nthe demo: check it there, and click Hire"
	demoOpenInTheWindow   = "Smolcorp's Settings window is open at the demo. Check what it says, then click Hire there"
	demoOpenInTheBrowser  = "Setup is open in your browser at the demo. Check what it says, then click Hire there"
)

// The command says where Setup opens at the demo, before your yes and once it's open: Smolcorp's Settings window under
// the Mac app and on Windows with its window, the browser on Linux and on a Windows without WebView2. On the tree
// before (4e410d0f) it said "your browser" everywhere, though the Mac app and Windows' window opened the page there.
func TestTheDemoSaysWhereItsPageOpens(t *testing.T) {
	for _, s := range demoSystems {
		t.Run(s.name, func(t *testing.T) {
			_, team, opens := demoOnSystem(t, s)
			code, out, _ := runDemoOn(t, team)
			macWindow, windowRuns, links := opens.seen()
			t.Logf("%s: exit %d; opened in the Mac's window at %q, Windows' window %d times, the browser at %q\n%s", s.name, code, macWindow, windowRuns, links, out)
			opened := len(macWindow) + windowRuns + len(links)
			inWindow := len(macWindow)+windowRuns == 1
			if opened != 1 || inWindow != s.window {
				t.Fatalf("%s: the page opened %d times (in a window: %v), want once, in a window: %v", s.name, opened, inWindow, s.window)
			}
			asks, said := demoAsksForTheBrowser, demoOpenInTheBrowser
			if s.window {
				asks, said = demoAsksForTheWindow, demoOpenInTheWindow
			}
			if !strings.Contains(out, asks+" to hire the General Manager.\n\nGo on? [y/N] ") || !strings.Contains(out, said) {
				t.Errorf("%s: doesn't say where its page opens (%q, then %q):\n%s", s.name, asks, said, out)
			}
			if s.window && strings.Contains(out, "browser") {
				t.Errorf("%s: says browser, where the page opens in Smolcorp's Settings window:\n%s", s.name, out)
			}
		})
	}
}

// --no-open where the page opens in Smolcorp's own Settings window (the Mac app, Windows with its window) is refused
// at once, before the question, and nothing is made, told or opened: its link would open Setup in a browser, whose
// Hire is refused there. Elsewhere it is as before: the question, the folder, the link printed, nothing opened. On
// the tree before (4e410d0f) the Mac app and Windows' window printed the link too.
func TestTheDemoRefusesNoOpenWhereItsPageOpensInSmolcorpsWindow(t *testing.T) {
	const refused = "The demo's page opens in Smolcorp's own Settings window here, so run smolcorp demo without --no-open."
	for _, s := range demoSystems {
		t.Run(s.name, func(t *testing.T) {
			_, team, opens := demoOnSystem(t, s)
			team.noOpen = true
			code, out, in := runDemoOn(t, team)
			macWindow, windowRuns, links := opens.seen()
			entries, _ := os.ReadDir(in)
			root := filepath.Join(in, demoFolderName)
			demoPages.Lock()
			told := demoPages.at[root] != nil
			demoPages.Unlock()
			t.Logf("%s --no-open: exit %d; made %d, told %v; opened in the Mac's window at %q, Windows' window %d times, the browser at %q\n%s",
				s.name, code, len(entries), told, macWindow, windowRuns, links, out)
			if len(macWindow)+windowRuns+len(links) != 0 {
				t.Errorf("%s --no-open: the page was opened", s.name)
			}
			if s.window {
				if code != 1 || !strings.Contains(out, refused) || strings.Contains(out, "Go on?") || len(entries) != 0 || told || strings.Contains(out, "/setup?demo=") {
					t.Errorf("%s --no-open: exit %d, made %d, told %v; want refused before the question (%q), nothing made:\n%s",
						s.name, code, len(entries), told, refused, out)
				}
				return
			}
			if !strings.Contains(out, "Go on?") || len(entries) != 1 || !told || strings.Contains(out, refused) ||
				!strings.Contains(out, "Open the demo's page in a browser on this computer, and click Hire there:\n\n  "+setupBase()+"/setup?demo="+url.QueryEscape(root)+"\n") {
				t.Errorf("%s --no-open: exit %d, made %d, told %v; want the question, the folder and its link printed:\n%s", s.name, code, len(entries), told, out)
			}
		})
	}
}
