package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// Setup's Phone microphone under the Mac app (engine=go), as the Swift engine
// served it (DeckRouter.swift, PhoneDictation.swift's Tailnet): the switch for
// the phone's text (config.json's phoneMicEnabled, read here since I5) and
// whether an older page's text goes with Return; and with it on, this Mac's
// address in its tailnet, for `tailscale serve` and the QR code that pairs a
// phone there, and whether Tailscale still hands the deck on while it's off.
// Tailscale's command line is only ever read (`tailscale status --json`,
// `tailscale serve status --json`), at most once a minute, never needed:
// without it setup just shows the steps. On Windows and Linux setup has no
// such section: the route is 404 and the state has no phoneMic.

// tailnetCandidates are where Tailscale's command line can be: the Mac app's own, then Homebrew's and the
// open-source one's (Tailnet.candidates).
var tailnetCandidates = []string{
	"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
	"/opt/homebrew/bin/tailscale",
	"/usr/local/bin/tailscale",
}

const (
	// tailnetLookAgainAfter is how old an answer may be before it's looked for again.
	tailnetLookAgainAfter = time.Minute
	// tailnetTimeout is how long Tailscale's command line may take to answer.
	tailnetTimeout = 3 * time.Second
)

// tailnetAnswer is what Tailscale says: this Mac's HTTPS address ("" for none), and whether `tailscale
// serve` hands the deck on (nil when that can't be told).
type tailnetAnswer struct {
	url     string
	serving *bool
}

// askTailnet asks Tailscale's command line, if it's here: false when it isn't installed. A variable, so
// the tests never run it.
var askTailnet = func() (tailnetAnswer, bool) {
	tool := ""
	for _, candidate := range tailnetCandidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			tool = candidate
			break
		}
	}
	if tool == "" {
		return tailnetAnswer{}, false
	}
	var answer tailnetAnswer
	if status, ok := runTailscale(tool, "status", "--json"); ok {
		answer.url = tailnetURL(status)
	}
	if status, ok := runTailscale(tool, "serve", "status", "--json"); ok {
		answer.serving = servesDeck(status, Port)
	}
	return answer, true
}

// runTailscale runs the tool, read-only, and gives what it printed, or false if it failed or took too long.
// The Mac app's binary is its command line only when TERM is set; without it, as from an app opened in
// Finder, it tries to start its window instead (Tailnet.environment).
func runTailscale(tool string, arguments ...string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), tailnetTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, tool, arguments...)
	command.Env = os.Environ()
	if os.Getenv("TERM") == "" {
		command.Env = append(command.Env, "TERM=dumb")
	}
	var out bytes.Buffer
	command.Stdout = &out
	// A child of the tool's left holding its output never keeps the call past the timeout by much (RGAPA N5).
	command.WaitDelay = time.Second
	if err := command.Run(); err != nil {
		return nil, false
	}
	return out.Bytes(), true
}

// tailnetURL is the address `tailscale serve` gives this Mac, from `tailscale status --json`: https:// and
// its MagicDNS name, or "" if it isn't there (Tailnet.url).
func tailnetURL(status []byte) string {
	var said struct {
		Self struct{ DNSName string }
	}
	if json.Unmarshal(status, &said) != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSuffix(said.Self.DNSName, "."))
	if !isTailnetName(host) {
		return ""
	}
	return "https://" + host
}

// tailnetScheme is a target's scheme, taken off before its address is compared.
var tailnetScheme = regexp.MustCompile(`^[a-z+]+://`)

// servesDeck says whether `tailscale serve status --json` hands this deck on: a proxy, or a TCP forward,
// to port on this Mac. Nil if it isn't that JSON (Tailnet.servesDeck).
func servesDeck(status []byte, port int) *bool {
	var said any
	if json.Unmarshal(status, &said) != nil {
		return nil
	}
	if _, ok := said.(map[string]any); !ok {
		return nil
	}
	points := func(target string) bool {
		address, _, _ := strings.Cut(tailnetScheme.ReplaceAllString(strings.ToLower(target), ""), "/")
		return slices.Contains([]string{fmt.Sprintf("127.0.0.1:%d", port), fmt.Sprintf("localhost:%d", port),
			fmt.Sprintf("[::1]:%d", port), fmt.Sprint(port)}, address)
	}
	var walk func(any) bool
	walk = func(value any) bool {
		switch v := value.(type) {
		case map[string]any:
			for key, inner := range v {
				if text, ok := inner.(string); ok && (key == "Proxy" || key == "TCPForward") && points(text) {
					return true
				}
				if walk(inner) {
					return true
				}
			}
		case []any:
			return slices.ContainsFunc(v, walk)
		}
		return false
	}
	serving := walk(said)
	return &serving
}

// tailnetLook is the last answer, and when it was looked for; whether a look is under way.
var tailnetLook struct {
	mu       sync.Mutex
	answer   tailnetAnswer
	lookedAt time.Time
	looking  bool
}

// tailnetNow is the last answer, and a fresh look started in the background when it's too old, which tells
// the office and setup when it's different. Never waits on Tailscale. Without the deck's lock.
func (d *Deck) tailnetNow() tailnetAnswer {
	tailnetLook.mu.Lock()
	defer tailnetLook.mu.Unlock()
	if !tailnetLook.looking && time.Since(tailnetLook.lookedAt) >= tailnetLookAgainAfter {
		tailnetLook.looking = true
		d.rounds.run(func() {
			answer, _ := askTailnet()
			tailnetLook.mu.Lock()
			changed := answer.url != tailnetLook.answer.url || !sameServing(answer.serving, tailnetLook.answer.serving)
			tailnetLook.answer, tailnetLook.lookedAt, tailnetLook.looking = answer, time.Now(), false
			tailnetLook.mu.Unlock()
			if changed {
				d.Refresh()
			}
		})
	}
	return tailnetLook.answer
}

func sameServing(a, b *bool) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// phoneMicState is setup's Phone microphone: on, whether an older page's text is sent, and with it on this
// Mac's tailnet address; whether Tailscale hands the deck on (DeckRouter.setupState's phoneMic). With the lock held.
func (d *Deck) phoneMicState(found tailnetAnswer) map[string]any {
	var url, serving any
	if d.config.PhoneMicEnabled && found.url != "" {
		url = found.url
	}
	if found.serving != nil {
		serving = *found.serving
	}
	return map[string]any{"enabled": d.config.PhoneMicEnabled, "autoSend": d.config.PhoneMicAutoSend, "tailnet": url, "serving": serving}
}

// tailnetPairingLink is the pairing link at this Mac's tailnet address, for the phone mic over HTTPS: "" with
// the phone mic off or no address. Without the deck's lock.
func (d *Deck) tailnetPairingLink(code string) string {
	found := d.tailnetNow()
	d.mu.Lock()
	on := d.config.PhoneMicEnabled
	d.mu.Unlock()
	if !on || found.url == "" {
		return ""
	}
	return found.url + "/?pair=" + code
}
