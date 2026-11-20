package desktop

import (
	"cmp"
	"errors"
	"net/http"

	"github.com/zalando/go-keyring"
)

// What the Mac app does with its own windows, panel, login item and Keychain,
// under the Mac app, through the bridge (hostbridge.go), as the
// Swift engine does them (DeckRouter.swift, SelfTest.swift): Open at login
// (LoginItem), the office's own window and keeping it on top (OfficeWindow),
// the Mac's folder panel (Deck.chooseHireFolder), ✨ Summary's key and the
// portable office's keys in the Keychain (SummaryKey, ViewerKeychain), when
// you're back at the Mac (MacPresence), and the self-test's install,
// Accessibility and network lines in the Mac's words. On Windows and Linux
// there is no host, and each does what it did before: autostart_*.go,
// window_*.go, picker*.go, summary_store_other.go, viewer_store_other.go and
// presence_*.go.

// keepOfficeOnTop is setup's switch, and the Mac's menu's Always on Top: the app keeps the office's window on
// top, or not, at once, and then it's kept. A bridge that doesn't answer leaves the setting as it was.
func (d *Deck) keepOfficeOnTop(h hostBridge, on bool) error {
	if err := hostOfficeOnTop(h, on); err != nil {
		return err
	}
	d.mu.Lock()
	d.config.AlwaysOnTop = on
	d.config.save()
	d.mu.Unlock()
	return nil
}

// macShell answers setup's routes the Mac app does with its own windows, panel
// and login item, under the Mac app, and says whether the route was one.
// Elsewhere it answers none: they're served as on Windows and Linux.
func (rt *router) macShell(w http.ResponseWriter, r *request) bool {
	d := rt.deck
	h := d.macHost()
	if h == nil || r.Method != http.MethodPost {
		return false
	}
	query := r.URL.Query()
	switch r.URL.Path {
	// The office's window kept on top of other windows: at once, and kept.
	case "/setup/ontop":
		if err := d.keepOfficeOnTop(h, query.Get("enabled") == "1"); err != nil {
			bridgeFailed(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The keys that show and hide the office from anywhere: checked on the layout the app says is in use, held
	// by the app at once, then kept. Keys macOS has switched on, or won't let it hold, are the app's refusal
	// (409), and nothing changes.
	case "/setup/officekey":
		var labels map[string]string
		if facts := d.macFactsNow(); facts != nil {
			labels = facts.OfficeKey.labels()
		}
		keys, refusal := officeKeyChoice(query.Get("keys"), labels)
		if refusal != "" {
			writeStatus(w)(http.StatusBadRequest, refusal)
			return true
		}
		if err := hostOfficeKey(h, keys); err != nil {
			bridgeFailed(w, err)
			return true
		}
		d.mu.Lock()
		d.config.OfficeKey = officeKeySaved(keys)
		d.config.save()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// Opening Smolcorp when you log in. A test copy leaves the login item to the Smolcorp you use: the app
	// says no, and setup answers as it is, as the Swift engine's LoginItem does in a test copy.
	case "/setup/login":
		on := query.Get("enabled") == "1"
		if _, err := hostLoginItem(h, &on); err != nil && !isAppRefusal(err) {
			bridgeFailed(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The office's window, from the page: the Mac's own, never the browser.
	case "/setup/office":
		if err := hostShowOffice(h); err != nil {
			bridgeFailed(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)

	// The Mac's own folder panel, which a web page can't open. With pick=1 it only answers the folder
	// picked ("picked", empty if none), for Start here to keep through its checks (POST /setup/hire-folder),
	// or with for=floor, for Floors to put a project on a floor (POST /setup/floors).
	case "/setup/hiring/choose":
		pick := query.Get("pick") == "1"
		purpose := folderForHires
		if pick && query.Get("for") == "floor" {
			purpose = folderForFloor
		}
		d.mu.Lock()
		start, _ := existingFolder(d.config.HireFolder)
		d.mu.Unlock()
		picked, err := hostChooseFolder(h, start, purpose)
		if err != nil {
			bridgeFailed(w, err)
			return true
		}
		if pick {
			writeJSON(w, http.StatusOK, map[string]string{"picked": picked})
			return true
		}
		if folder, ok := existingFolder(picked); picked != "" && ok {
			d.mu.Lock()
			d.config.HireFolder = folder
			d.config.save()
			d.changed()
			d.mu.Unlock()
		}
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	default:
		return false
	}
	return true
}

// isAppRefusal is err the Mac app saying no, rather than a bridge that didn't answer.
func isAppRefusal(err error) bool {
	var refusal appRefusal
	return errors.As(err, &refusal)
}

// closeMacFolderPanel closes the Mac's folder panel if it's open for the hires folder, as if cancelled: a
// folder typed on the page is kept instead (DeckRouter's closeHireFolderPanel). Nothing without the app.
// Without the lock.
func (d *Deck) closeMacFolderPanel() {
	if h := d.macHost(); h != nil {
		_ = hostCloseFolderPanel(h)
	}
}

// openAtLogin is whether Smolcorp opens at login: under the Mac app, its login item (in a test copy, the
// Smolcorp in use's, read and never set); elsewhere this program's own. Without the lock.
func (d *Deck) openAtLogin() bool {
	if h := d.macHost(); h != nil {
		on, err := hostLoginItem(h, nil)
		return err == nil && on
	}
	return isOpenAtLogin()
}

// officeWindowState is setup's officeWindow: under the Mac app its own window, which can be kept on top;
// elsewhere a window of its own only on Windows. With the lock held.
func (d *Deck) officeWindowState() map[string]any {
	if d.mac.host != nil {
		return map[string]any{"available": true, "mac": true, "onTop": d.config.AlwaysOnTop}
	}
	return map[string]any{"available": hasNativeWindow, "mac": false, "onTop": hasNativeWindow && d.config.AlwaysOnTop}
}

// canChooseFolder is whether setup can open a folder picker: the Mac's panel under the Mac app, or this
// system's own. With the lock held.
func (d *Deck) canChooseFolder() bool {
	return d.mac.host != nil || hasFolderPicker()
}

// summaryKeyItem is ✨ Summary's key among the Mac app's own items (bridgeSecrets).
const summaryKeyItem = "summary-key"

// summaryStore is where a pasted Summary key is kept: under the Mac app, its Keychain item, as the Swift
// engine keeps it (in a test copy, the app's own file, never the Keychain); elsewhere this system's
// keyring. Never in this program's files, and never logged.
func summaryStore(h hostBridge) secretStore {
	if h == nil {
		return keyringStore
	}
	return secretStore{
		get: func() (string, error) {
			value, found, err := hostReadSecret(h, summaryKeyItem)
			if err == nil && !found {
				return "", keyring.ErrNotFound
			}
			return value, err
		},
		set:    func(key string) error { return hostWriteSecret(h, summaryKeyItem, key) },
		delete: func() error { return hostWriteSecret(h, summaryKeyItem, "") },
	}
}

// viewerItems are the portable office's id, key and write secret among the Mac app's own items
// (bridgeSecrets), by the account the core names each by.
var viewerItems = map[string]string{viewerIDAccount: "viewer-id", viewerKeyAccount: "viewer-key", viewerSecretAccount: "viewer-write-secret"}

// errViewerItemNone is the Mac app saying it has no such item: surely none, so a first id and key are made.
// One that's there but can't be read is the app's refusal instead, never taken for none.
var errViewerItemNone = errors.New("the Mac app has no such item")

// viewerStoreFor is where the portable office's id, key and write secret are kept: under the Mac app, its
// Keychain items, the Swift engine's own (smolcorp-viewer), so a link made by 1.24 keeps working (in a
// test copy, the app's own files, never the Keychain); elsewhere this system's (viewerStore). The app
// never removes them this way, nor does the portable office: a wipe has the viewer forget, and a new key
// is written over the old one.
func viewerStoreFor(h hostBridge) viewerSecrets {
	if h == nil {
		return viewerStore
	}
	item := func(account string) (string, error) {
		if name, ok := viewerItems[account]; ok {
			return name, nil
		}
		return "", errors.New("not one of the viewer's items")
	}
	return viewerSecrets{
		get: func(account string) (string, error) {
			name, err := item(account)
			if err != nil {
				return "", err
			}
			value, found, err := hostReadSecret(h, name)
			if err == nil && !found {
				return "", errViewerItemNone
			}
			return value, err
		},
		set: func(account, value string) error {
			name, err := item(account)
			if err != nil || value == "" {
				return cmp.Or(err, errors.New("nothing to keep"))
			}
			return hostWriteSecret(h, name, value)
		},
		delete:   func(string) {},
		notFound: func(err error) bool { return errors.Is(err, errViewerItemNone) },
		keychain: true,
	}
}

// macPresenceOf is what the Mac app tells of you being back at the Mac, for the portable office; nil
// without the app.
func macPresenceOf(h hostBridge) func() (macPresence, bool) {
	if h == nil {
		return nil
	}
	return func() (macPresence, bool) {
		now, err := hostPresence(h)
		return now, err == nil
	}
}

// macChecks are the self-test's lines after the hooks under the Mac app, as SelfTest.swift has them:
// Accessibility, then the network, the phone and the login lines in the Mac's words. Without the app's
// facts there's no Accessibility line. Without the lock.
func (d *Deck) macChecks(phones int, waiting string) []check {
	var checks []check
	facts := d.macFactsNow()
	if facts != nil {
		checks = append(checks, accessibilityCheck(facts.Accessibility))
	}
	checks = append(checks, macNetworkCheck(tailscaleAddresses()), phoneCheck(phones, waiting),
		macLoginCheck(d.openAtLogin(), facts != nil && facts.TestCopy))
	if facts != nil && facts.SigningKeyUnlocked != nil && *facts.SigningKeyUnlocked {
		checks = append(checks, signingKeyCheck())
	}
	return checks
}

// signingKeyCheck is the line shown while the key Smolcorp is signed with sits unlocked on this Mac (SANDBOX-PLAN
// §3.7). It's advice, and worded so: the app looks only in the keychains of the search list, which any program of
// yours can change, so its silence proves nothing. There's no fix to press: moving the key is the owner's own step.
func signingKeyCheck() check {
	return check{ID: "signing", Status: checkTodo, Title: "Smolcorp's signing key is locked away",
		Detail: "The key Smolcorp is signed with is unlocked on this Mac, so another program could sign a fake Smolcorp " +
			"that reads Smolcorp's secrets. If you build Smolcorp yourself, keep that key in a keychain of its own that " +
			"locks after each build: see Keys, records and the guard in Smolcorp's docs."}
}

// macNetworkCheck is how a phone reaches this Mac, in the Mac's words, by Tailscale's addresses here.
func macNetworkCheck(tailscale []string) check {
	c := check{ID: "network", Title: "Your phone can reach this Mac"}
	if len(tailscale) > 0 {
		c.Status, c.Detail = checkOK, "On this Wi-Fi, and through Tailscale at "+tailscale[0]+" from anywhere."
		return c
	}
	c.Status, c.Fix, c.Label = checkInfo, fixTailscale, "Get Tailscale"
	c.Detail = "On this Wi-Fi. With Tailscale, free, on this Mac and your phone, it reaches you from anywhere."
	return c
}

// accessibilityCheck is whether macOS lets Smolcorp use the keyboard, as the Mac's General says it.
func accessibilityCheck(allowed bool) check {
	c := check{ID: "accessibility", Status: checkOK, Title: "Smolcorp can use your keyboard"}
	if allowed {
		c.Detail = "Your phone's keys, the buttons, the mic and scrolling reach your agents."
		return c
	}
	c.Status, c.Fix, c.Label = checkTodo, fixAccessibility, "Allow"
	c.Detail = "Your phone's keys, the buttons, the mic and scrolling need it. Turn Smolcorp on in the list that opens. After an update, turn it on again."
	return c
}

// macLoginCheck is whether Smolcorp opens at login, in the Mac's words: a test copy leaves it to the
// Smolcorp in use.
func macLoginCheck(on, testCopy bool) check {
	c := check{ID: "login", Status: checkOK, Title: "Smolcorp opens at login", Detail: "It's in the menu bar from the moment you log in."}
	switch {
	case testCopy:
		c.Status, c.Detail = checkInfo, "A test copy leaves the login item to the Smolcorp you use."
	case !on:
		c.Status, c.Fix, c.Label = checkTodo, fixLogin, "Open at login"
		c.Detail = "Otherwise, sessions started before you open Smolcorp aren't at a desk until their next turn."
	}
	return c
}

// macFix runs a fix that is the Mac app's under it, and says whether name was one: Accessibility asked of
// macOS, the login item (left to the Smolcorp in use by a test copy, as the Swift engine leaves it), and
// Move to Applications.
func (d *Deck) macFix(name string) (status int, message string, done bool) {
	h := d.macHost()
	if h == nil {
		return 0, "", false
	}
	var err error
	switch name {
	case fixAccessibility:
		err = hostAskAccessibility(h)
	case fixLogin:
		on := true
		if _, err = hostLoginItem(h, &on); isAppRefusal(err) {
			err = nil
		}
	// Move to Applications: the app copies itself there and opens the copy. Why not (a test copy stays where it
	// was built) is the Mac's sentence, 409, as the Swift engine's fix route says it.
	case fixInstall:
		if err = hostInstall(h); isAppRefusal(err) {
			return http.StatusConflict, err.Error(), true
		}
	default:
		return 0, "", false
	}
	if err != nil {
		return http.StatusServiceUnavailable, "Smolcorp couldn't ask the Mac app: " + err.Error(), true
	}
	return http.StatusNoContent, "", true
}
