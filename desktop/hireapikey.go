package desktop

import (
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"unicode"
)

// The API-key account for a hire (the Mac's C23), on every OS: setup's state
// says whether it's offered and what it costs (hiring.apiKey {available,
// notice}), and a hire on it is billed to the Anthropic API key that Summaries
// use, kept where this computer keeps that key:
//
//   - Under the Mac app: the Mac's Keychain (in a test copy, the copy's own
//     test-api-key file), which only the app reads, through the bridge.
//     api-key-offer says whether it's offered without reading the key;
//     api-key-read gives the key, only while it's offered, to this core alone
//     (M1's token and peer pid). It's read once per hire, as the hire is made,
//     and travels from there only in this process's memory: its last 20
//     characters are approved in the account's .claude.json, as the Mac's
//     HireDesk.prepare does, and the key goes into that hire's start script
//     as `export ANTHROPIC_API_KEY='…'`. That script is the only place it's
//     written: <support>/hires/<id>/hire-*.sh, the file 0700 in a 0700
//     folder, read and run by the hire's own shell, whose first line removes
//     it. A hire refused once the script is written, or one that never
//     starts, has the script removed with it (runManagedHire's undo,
//     watchManagedHire). Never an argument (tmux is given the script's path),
//     never the registry (it says "api-key" as the Mac's does), never a log,
//     and the core doesn't keep it after the hire: the request's field is
//     emptied once the hire is made. Go strings can't be wiped, so copies (the
//     request passed by value, the key's closure, the script's text) stay in
//     memory until the garbage collector reuses them, as in any process that
//     holds a secret. A start script left on disk by a core that stopped
//     before the hire's shell ran it is removed as the next core starts
//     (removeLeftStartScripts), and its hire fails.
//     A DIFFERENCE FROM THE MAC: the Mac's own script never writes the key
//     down, it reads the Keychain with /usr/bin/security as the hire starts.
//     The Go core under the Mac app can't ask for the app's item that way
//     without macOS asking the owner, so it writes the key into the start
//     script, for the seconds until the hire's shell starts it. That is the
//     same lifetime the send key and the GM's token already have, and it
//     needs the owner's OK.
//   - Linux: the Secret Service (GNOME Keyring, KWallet), where Summaries keep
//     it (keyringStore, go-keyring). The tail is read through keyringStore;
//     the script reads the key with secret-tool as it runs, so it's never
//     written down. Offered only with a key saved there, never
//     ANTHROPIC_API_KEY alone (the Mac's rule), and secret-tool installed.
//   - Windows: not offered. A role's hire there goes through the launcher,
//     which has no way to give it the key.
//
// Whatever the OS, a hire on it is made only by the hire engine, fenced as any
// engine hire (its settings, the guard, its own tmux server), or not at all:
// never a plain terminal, which would start on the subscription.

const (
	// The Mac's HireDesk.apiKeyNotice, and its refusals for a hire that isn't Claude Code's.
	apiKeyNotice    = "Billed to your Anthropic API key (the one saved under Summaries), not your Claude subscription. The hire can read this key."
	apiKeyNotClaude = "The API-key account is for Claude Code hires."
	// This computer's own words, as the Mac's HireDesk.apiKeyRefusal says them of the Keychain.
	apiKeyNotInStore   = "There's no Anthropic API key in the secret store. Save one under Summaries, or hire on a Claude account."
	apiKeyNoStore      = "There's no secret store on this computer to keep an Anthropic API key in: hire on a Claude account."
	apiKeyNoSecretTool = "Install secret-tool (libsecret-tools) so a hire can read the API key from the secret store, or hire on a Claude account."
	apiKeyNotHere      = "A hire on the API key isn't set up on this computer yet: hire on a Claude account."
	apiKeyMacUnasked   = "Smolcorp couldn't ask the Mac app about the API key: hire on a Claude account."
	// The Mac's HireDesk.keyRefusal, for a key the app didn't give.
	apiKeyMacUnread = "Smolcorp couldn't read your Anthropic API key from the Keychain. Save it again under Summaries, or hire on a Claude account."
	// A hire on it that only a plain terminal could take.
	apiKeyNeedsEngine = "A hire on the API-key account starts only through Smolcorp's own hire engine, in tmux of its own, and this one couldn't: hire on a Claude account."
	// What the registry says of a hire on it, as the Mac's FrontDesk.Hire.account.
	apiKeyAccountName = "api-key"
)

// apiKeyOffer is the API-key account for a hire, now: whether it's offered,
// the notice beside it, and the refusal a hire on it gets when it isn't.
type apiKeyOffer struct {
	Available bool
	Notice    string
	Refusal   string
}

// state is setup's hiring.apiKey, as the Mac's.
func (o apiKeyOffer) state() map[string]any {
	return map[string]any{"available": o.Available, "notice": o.Notice}
}

// apiKeyOffer asks where the key lives: the Mac app, or this computer's
// secret store. Without the lock.
func (d *Deck) apiKeyOffer() apiKeyOffer {
	if h := d.macHost(); h != nil {
		return macAPIKeyOffer(h)
	}
	shown, noStore := "", false
	if d.summaryKeys != nil {
		shown, noStore = d.summaryKeys.Shown(), d.summaryKeys.NoStore()
	}
	return localAPIKeyOffer(runtime.GOOS, shown, noStore, secretToolFound)
}

// macAPIKeyOffer is the Mac app's answer, in its words; an app that can't be
// asked offers nothing.
func macAPIKeyOffer(h hostBridge) apiKeyOffer {
	available, notice, refusal, err := hostAPIKeyOffer(h)
	if notice == "" {
		notice = apiKeyNotice
	}
	switch {
	case err != nil:
		return apiKeyOffer{Notice: notice, Refusal: apiKeyMacUnasked}
	case !available && refusal == "":
		refusal = apiKeyMacUnread
	}
	return apiKeyOffer{Available: available, Notice: notice, Refusal: refusal}
}

// localAPIKeyOffer is the offer on Linux and Windows: shown is where
// Summaries' key is (summaryKeys.Shown), noStore that there's no secret store to
// keep one in (summaryKeys.NoStore), secretTool whether the script can read it.
func localAPIKeyOffer(goos, shown string, noStore bool, secretTool func() bool) apiKeyOffer {
	offer := apiKeyOffer{Notice: apiKeyNotice}
	switch {
	case goos == "windows":
		offer.Refusal = apiKeyNotHere
	case noStore:
		offer.Refusal = apiKeyNoStore
	case shown != "store":
		offer.Refusal = apiKeyNotInStore
	case !secretTool():
		offer.Refusal = apiKeyNoSecretTool
	default:
		offer.Available = true
	}
	return offer
}

// secretToolFound is whether a hire's start script can read the key from the
// Secret Service (hireStart.environmentLines).
func secretToolFound() bool {
	_, err := exec.LookPath("secret-tool")
	return err == nil
}

// hireAPIKeyText is the key a hire on the API-key account is given in its
// start script: under the Mac app, read through the bridge now, for this
// hire alone; "" elsewhere, where the script reads the secret store as it
// runs. A key the app won't give is the app's refusal, in its words. Without the lock.
func (d *Deck) hireAPIKeyText() (string, error) {
	h := d.macHost()
	if h == nil {
		return "", nil
	}
	key, err := hostAPIKeyRead(h)
	if refusal, ok := err.(appRefusal); ok {
		return "", &hireRefusal{Status: http.StatusConflict, Message: string(refusal)}
	}
	if err != nil || !isHireKeyText(key) {
		return "", &hireRefusal{Status: http.StatusConflict, Message: apiKeyMacUnread}
	}
	return key, nil
}

// onAPIKey is whether setup's hire extra is on the API-key account (not into T3 Code, which refuses it).
func onAPIKey(extra *setupHire) bool { return extra != nil && extra.APIKey && !extra.T3 }

// isHireKeyText is a key that can go in a start script as one word: the Mac
// app checks it's Anthropic's (SummaryKey.isKey); this only that it's one
// short line of letters, digits, - and _.
func isHireKeyText(key string) bool {
	return key != "" && len(key) <= 512 && strings.IndexFunc(key, func(r rune) bool {
		return r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_')
	}) < 0
}
