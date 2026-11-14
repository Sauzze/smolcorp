package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
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
// phone mic switch: the route is 404 and the state has no phoneMic. What it
// shares with the Mac, on every system, is Tailscale's side: the look below,
// and setup's Phone over HTTPS status and its tap (phonehttps.go), whose Turn
// it on is refused only where the hand-on isn't let in (serveHandOnHere). On a Mac and on Linux it hands the deck on
// to serve.sock (SN1, OLD-SERVE1), on Windows to the serve port (OLD-SERVE2, peerowner.go).

// tailnetCandidates are where Tailscale's command line can be on this system, before a look along PATH.
var tailnetCandidates = tailnetCandidatesOn(runtime.GOOS)

// tailnetLookPath is the look along PATH for Tailscale's command line. A variable so a test never finds the real one.
var tailnetLookPath = exec.LookPath

// tailnetCandidatesOn is where Tailscale's command line can be on a system, by Go's name for it. On a Mac, the app's
// own, then Homebrew's and the open-source one's (Tailnet.candidates); on Linux the package's, and a snap's; on
// Windows its installer's folder.
func tailnetCandidatesOn(system string) []string {
	switch system {
	case "linux":
		return []string{"/usr/bin/tailscale", "/usr/sbin/tailscale", "/usr/local/bin/tailscale", "/snap/bin/tailscale"}
	case "windows":
		var found []string
		for _, variable := range []string{"ProgramFiles", "ProgramW6432", "ProgramFiles(x86)"} {
			if folder := os.Getenv(variable); folder != "" {
				found = append(found, filepath.Join(folder, "Tailscale", "tailscale.exe"))
			}
		}
		return found
	}
	return []string{
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
		"/opt/homebrew/bin/tailscale",
		"/usr/local/bin/tailscale",
	}
}

// tailscaleTool is Tailscale's command line on this computer, or "" if it isn't here: a place it's usually put,
// then PATH. Windows files have no execute bit, so there it's a file that exists.
func tailscaleTool() string {
	for _, candidate := range tailnetCandidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && (runtime.GOOS == "windows" || info.Mode()&0o111 != 0) {
			return candidate
		}
	}
	if found, err := tailnetLookPath("tailscale"); err == nil {
		return found
	}
	return ""
}

const (
	// tailnetLookAgainAfter is how old an answer may be before it's looked for again.
	tailnetLookAgainAfter = time.Minute
	// tailnetTimeout is how long Tailscale's command line may take to answer.
	tailnetTimeout = 3 * time.Second
)

// tailnetAnswer is what Tailscale says: this computer's HTTPS address ("" for none), and whether `tailscale
// serve` hands the deck on (nil when that can't be told).
type tailnetAnswer struct {
	url     string
	serving *bool
	// Tailscale is running and signed in (its BackendState is Running).
	signedIn bool
	// `tailscale serve` hands something else on, not this deck: turning it on would replace that.
	busy bool
	// oldWay: on a Mac or on Linux, `tailscale serve` hands this deck on as before 1.26's SN1, to its TCP port, where
	// nothing counts as Tailscale's any more: it has to be turned on again once, to serve.sock (busy is true with it).
	oldWay bool
	// Set by the look, never by askTailnet: whether Tailscale's command line is here, and whether it was asked at all.
	installed, asked bool
}

// askTailnet asks Tailscale's command line, if it's here: false when it isn't installed. A variable, so
// the tests never run it.
var askTailnet = func() (tailnetAnswer, bool) {
	tool := tailscaleTool()
	if tool == "" {
		return tailnetAnswer{}, false
	}
	var answer tailnetAnswer
	if status, ok := runTailscale(tool, "status", "--json"); ok {
		answer.url = tailnetURL(status)
		answer.signedIn = tailnetSignedIn(status)
	}
	if status, ok := runTailscale(tool, "serve", "status", "--json"); ok {
		answer.serving, answer.oldWay = servingOf(status)
		answer.busy = answer.serving != nil && !*answer.serving && servesAnything(status)
	}
	return answer, true
}

// runTailscale runs the tool, read-only, and gives what it printed, or false if it failed or took too long.
// The Mac app's binary is its command line only when TERM is set; without it, as from an app opened in
// Finder, it tries to start its window instead (Tailnet.environment).
func runTailscale(tool string, arguments ...string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), tailnetTimeout)
	defer cancel()
	command := disclaim.Cmd(exec.CommandContext(ctx, tool, arguments...))
	hideGitWindow(command) // no console window flashing up on Windows
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

// tailnetSignedIn says whether `tailscale status --json` has Tailscale running and signed in: its BackendState is
// Running (NeedsLogin, Stopped and the rest are not).
func tailnetSignedIn(status []byte) bool {
	var said struct{ BackendState string }
	return json.Unmarshal(status, &said) == nil && said.BackendState == "Running"
}

// servesAnything says whether `tailscale serve status --json` hands anything at all on (a web handler or a TCP
// forward), whatever it points at.
func servesAnything(status []byte) bool {
	var said struct {
		TCP map[string]any
		Web map[string]any
	}
	return json.Unmarshal(status, &said) == nil && (len(said.TCP) > 0 || len(said.Web) > 0)
}

// tailnetScheme is a target's scheme, taken off before its address is compared.
var tailnetScheme = regexp.MustCompile(`^[a-z+]+://`)

// servingOf says whether `tailscale serve status --json` hands this deck on the way it's let in here, and whether it
// does so the old way. On a Mac and on Linux the way is serve.sock (SN1, OLD-SERVE1), on Windows the serve port
// (OLD-SERVE2), and the deck's own TCP port is the old way, which nothing counts through any more. Where the hand-on
// is refused, the deck's port is still what it's told by.
func servingOf(status []byte) (serving *bool, oldWay bool) {
	byPort := servesDeck(status, Port)
	switch {
	case !serveHandOnHere():
		return byPort, false
	case windowsTailscale:
		serving = servesDeck(status, servePort())
	default:
		serving = servesSocket(status, serveSocketPath())
	}
	return serving, serving != nil && !*serving && byPort != nil && *byPort
}

// servesDeck says whether `tailscale serve status --json` hands this deck on to its TCP port: a proxy, or a TCP
// forward, to port on this computer. Nil if it isn't that JSON (Tailnet.servesDeck).
func servesDeck(status []byte, port int) *bool {
	return servesTarget(status, func(target string) bool {
		address, _, _ := strings.Cut(tailnetScheme.ReplaceAllString(strings.ToLower(target), ""), "/")
		return slices.Contains([]string{fmt.Sprintf("127.0.0.1:%d", port), fmt.Sprintf("localhost:%d", port),
			fmt.Sprintf("[::1]:%d", port), fmt.Sprint(port)}, address)
	})
}

// servesSocket says whether `tailscale serve status --json` hands this deck on to serve.sock at path: a proxy to
// unix:path, the same path exactly. Nil if it isn't that JSON.
func servesSocket(status []byte, path string) *bool {
	return servesTarget(status, func(target string) bool {
		found, ok := strings.CutPrefix(strings.TrimSpace(target), "unix:")
		return ok && found != "" && filepath.Clean(found) == filepath.Clean(path)
	})
}

// servesTarget says whether `tailscale serve status --json` has a proxy, or a TCP forward, to a target points takes
// for this deck's. Nil if it isn't that JSON.
func servesTarget(status []byte, points func(target string) bool) *bool {
	var said any
	if json.Unmarshal(status, &said) != nil {
		return nil
	}
	if _, ok := said.(map[string]any); !ok {
		return nil
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
		d.rounds.run(func() { d.tailnetLookNow(true) })
	}
	return tailnetLook.answer
}

// tailnetLookNow asks Tailscale's command line now, keeps the answer, and tells the office and setup if it's
// different. Waits on Tailscale (a few seconds at most): off the request that is waiting for nothing else.
// background says it is the one tailnetNow started, so it lets the next look start.
func (d *Deck) tailnetLookNow(background bool) tailnetAnswer {
	answer, installed := askTailnet()
	answer.installed, answer.asked = installed, true
	tailnetLook.mu.Lock()
	changed := !sameAnswer(answer, tailnetLook.answer)
	tailnetLook.answer, tailnetLook.lookedAt = answer, time.Now()
	if background {
		tailnetLook.looking = false
	}
	tailnetLook.mu.Unlock()
	if changed {
		d.Refresh()
	}
	return answer
}

// sameAnswer says whether two looks found the same.
func sameAnswer(a, b tailnetAnswer) bool {
	return a.url == b.url && sameServing(a.serving, b.serving) && a.signedIn == b.signedIn && a.busy == b.busy &&
		a.installed == b.installed && a.asked == b.asked
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
func (d *Deck) tailnetPairingLink(key string) string {
	found := d.tailnetNow()
	d.mu.Lock()
	on := d.config.PhoneMicEnabled
	d.mu.Unlock()
	if !on || found.url == "" {
		return ""
	}
	return found.url + "/#pair=" + key
}
