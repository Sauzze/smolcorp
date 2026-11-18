package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
)

// What a page may show in a frame (TV2): the office's own window may show a desk's preview, at localhost only, for
// the TV; a phone, the viewer and every other page show none. Decided as the page is served, by the caller's class.

// cspDirective is one directive's sources in a Content-Security-Policy, and whether it's there.
func cspDirective(csp, name string) (string, bool) {
	for _, directive := range strings.Split(csp, ";") {
		fields := strings.Fields(directive)
		if len(fields) > 0 && strings.EqualFold(fields[0], name) {
			return strings.Join(fields[1:], " "), true
		}
	}
	return "", false
}

func TestOnlyTheOfficeWindowsPageFramesAPreview(t *testing.T) {
	d, phoneKey := probeDeck(t)
	// Every caller of the routes, and the office in a browser on this computer, and the window under the Mac app.
	callers := append(slices.Clone(routeCallers),
		routeCaller{name: "browser", host: "localhost:7710", from: thisComputer, key: "browser"},
		routeCaller{name: "window-mac-app", host: "localhost:7710", from: thisComputer, key: "window", macApp: true})
	var framing []string
	var servedTo []string
	for _, c := range callers {
		// The class that reads a desk's preview address is the one whose page may show it.
		readsPreviews := letIn(probeAnswer(t, d, "GET", "/desk/preview", phoneKey, c).Code)
		for _, path := range []string{"/", "/office"} {
			page := probeAnswer(t, d, "GET", path, phoneKey, c)
			csp := page.Header().Get("Content-Security-Policy")
			// Turned away before any page (a proxy's headers where only `tailscale serve` may send them): no page,
			// so nothing to frame.
			if page.Code != http.StatusOK {
				if strings.Contains(page.Body.String(), "<html") || readsPreviews {
					t.Errorf("%s, GET %s: %d with a page, or the preview's address", c.name, path, page.Code)
				}
				continue
			}
			if path == "/" {
				servedTo = append(servedTo, c.name)
			}
			if page.Header().Get("X-Frame-Options") != "DENY" {
				t.Errorf("%s, GET %s: framed by %q", c.name, path, page.Header().Get("X-Frame-Options"))
			}
			if ancestors, _ := cspDirective(csp, "frame-ancestors"); ancestors != "'none'" {
				t.Errorf("%s, GET %s: the page may be framed by %q, want no one", c.name, path, ancestors)
			}
			// Setup and its cookie live on 127.0.0.1 alone (FXV1R1): no page may frame it.
			if strings.Contains(csp, "127.0.0.1") {
				t.Errorf("%s, GET %s: the page's CSP names Setup's host: %q", c.name, path, csp)
			}
			frames, said := cspDirective(csp, "frame-src")
			want := "'none'"
			if readsPreviews {
				want = "http://localhost:*"
			}
			if !said || frames != want {
				t.Errorf("%s, GET %s: frame-src %q, want %q (CSP %q)", c.name, path, frames, want, csp)
			}
			if path == "/" && frames != "'none'" {
				framing = append(framing, c.name)
			}
		}
	}
	// The window's key on this computer, as the Mac app's window or a browser here: never a phone, the Windows
	// window's paired device, a proxy's made-up headers, or the window's key from the Wi-Fi.
	if want := []string{"window", "browser", "window-mac-app"}; !slices.Equal(framing, want) {
		t.Errorf("the pages that may frame a preview are %v's, want %v's", framing, want)
	}
	// Those checked above include a stranger, the phones, the window's key from the Wi-Fi and the Windows window's
	// device, all of which got a page.
	for _, name := range []string{"stranger", "phone-wifi", "phone-wireguard", "phone-serve-mac", "phone-here", "window-key-on-wifi", "window-device"} {
		if !slices.Contains(servedTo, name) {
			t.Errorf("%s got no page, so its frames went unchecked (served: %v)", name, servedTo)
		}
	}
	// The page alone (?deck), with no key, as a phone's Home Screen app opens it.
	alone := probeAnswer(t, d, "GET", "/?deck", "", routeCallers[0])
	if !strings.Contains(alone.Body.String(), `id="office-canvas"`) {
		t.Fatal("the page alone wasn't the deck")
	}
	if frames, _ := cspDirective(alone.Header().Get("Content-Security-Policy"), "frame-src"); frames != "'none'" {
		t.Errorf("the page alone may frame %q, want nothing", frames)
	}
}

func TestEveryOtherPageFramesNothing(t *testing.T) {
	d := newTestDeck(t)
	for _, target := range []string{"/setup", "/setup/preview"} {
		page := serve(t, d, call{method: "GET", target: target, host: "127.0.0.1:7710", from: thisComputer})
		if page.Code != http.StatusOK {
			t.Fatalf("GET %s: %d", target, page.Code)
		}
		if frames, _ := cspDirective(page.Header().Get("Content-Security-Policy"), "frame-src"); frames != "'none'" {
			t.Errorf("GET %s may frame %q, want nothing", target, frames)
		}
	}
}

// framesNothing is whether a CSP lets its page show no frame: frame-src, or else child-src, or else default-src, is
// 'none'.
func framesNothing(csp string) bool {
	for _, name := range []string{"frame-src", "child-src", "default-src"} {
		if sources, said := cspDirective(csp, name); said {
			return sources == "'none'"
		}
	}
	return false
}

func TestTheViewerFramesNothing(t *testing.T) {
	// A stand-in first, so the check can fail.
	if framesNothing("default-src 'none'; frame-src http://localhost:*") || framesNothing("script-src 'self'") ||
		!framesNothing("default-src 'self'; frame-src 'none'") {
		t.Fatal("framesNothing reads a CSP wrong")
	}
	data, err := os.ReadFile("../viewer/vercel.json")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Headers []struct {
			Source  string `json:"source"`
			Headers []struct{ Key, Value string }
		} `json:"headers"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	policies := 0
	for _, rule := range config.Headers {
		for _, header := range rule.Headers {
			if strings.EqualFold(header.Key, "Content-Security-Policy") {
				policies++
				if !framesNothing(header.Value) {
					t.Errorf("the viewer's pages at %s may show a frame: %q", rule.Source, header.Value)
				}
			}
		}
	}
	if policies == 0 {
		t.Error("the viewer sends no Content-Security-Policy")
	}
}
