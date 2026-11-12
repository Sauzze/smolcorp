package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// Stand-in GM tokens, as kept: 43 base64url characters, and obviously not a real one.
const (
	fakeKeptToken  = "fake-gm-token-from-1-24-0000000000000000000"
	fakeOtherToken = "fake-gm-token-other-00000000000000000000000"
)

// keptItem is the Mac app's gm-token item as the stand-in holds it, and whether there is one.
func (m *macStandIn) keptItem() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, found := m.secrets[gmTokenItem]
	return value, found
}

// withHiredGM puts a GM in the registry, open or fired.
func withHiredGM(t *testing.T, state string) {
	t.Helper()
	file := readRegistry(registryPath(), true)
	file.hires = []Hire{{ID: "gm0001", Name: "GM", Role: gmRole, Folder: homeDir, State: state}}
	if err := file.save(registryPath()); err != nil {
		t.Fatal(err)
	}
}

func worksAsGM(d *Deck, token string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.isGMToken(token)
}

// A hired GM, in a tmux session of its own that outlives the app, isn't refused after a restart: its token goes in
// the Mac app's gm-token item at each change and is read back as the core starts. Fired, the item goes too.
func TestAHiredGMsTokenIsKeptInTheMacsKeychainItemAcrossARestart(t *testing.T) {
	isolate(t)
	mac := withMac(t)
	d := startDeck(t)
	token := d.newGMToken()
	d.gmKept.flush(time.Minute)
	if kept, _ := mac.keptItem(); kept != token {
		t.Fatalf("the hired GM's token isn't the Keychain item's (kept %v)", kept != "")
	}
	withHiredGM(t, "working")
	again := startDeck(t)
	if !worksAsGM(again, token) {
		t.Fatal("after a restart, the hired GM's token is refused")
	}

	// Fired: its token stops working, and isn't kept for the next run.
	again.clearGMTokenIf(token)
	again.gmKept.flush(time.Minute)
	if _, found := mac.keptItem(); found {
		t.Error("a fired GM's token is still in the Keychain item")
	}
	withHiredGM(t, "fired")
	if worksAsGM(startDeck(t), token) {
		t.Error("a fired GM's token works after a restart")
	}
}

// The upgrade from 1.24: the token the Swift engine kept in the same item is read on the first launch, so the GM
// that's running goes on working, and nothing is written over it. With no GM hired (and none started by hand), a
// token left in the item is taken away rather than kept working.
func TestTheTokenOfAGMHiredOn124WorksOnTheFirstLaunch(t *testing.T) {
	isolate(t)
	mac := withMac(t)
	mac.secrets = map[string]string{gmTokenItem: fakeKeptToken}
	withHiredGM(t, "working")
	d := startDeck(t)
	d.gmKept.flush(time.Minute)
	if !worksAsGM(d, fakeKeptToken) {
		t.Fatal("the GM hired on 1.24 is refused on the first launch")
	}
	for _, action := range mac.actions() {
		if action == "secret-write" || action == "secret-delete" {
			t.Errorf("the item was changed at the start: %s", action)
		}
	}

	withHiredGM(t, "fired")
	gone := startDeck(t)
	gone.gmKept.flush(time.Minute)
	if worksAsGM(gone, fakeKeptToken) {
		t.Error("with no GM hired, the old token works")
	}
	if _, found := mac.keptItem(); found {
		t.Error("with no GM hired, the old token is still kept")
	}
}

// A new token from setup is the one kept: after a restart the old one is refused and the new one works.
func TestANewGMTokenIsTheOneKeptForTheNextRun(t *testing.T) {
	isolate(t)
	mac := withMac(t)
	mac.secrets = map[string]string{gmTokenItem: fakeKeptToken}
	withHiredGM(t, "working")
	d := startDeck(t)
	managerOn(d)
	if code, _, _ := fromPage(t, d, http.MethodPost, "/setup/gm/token/rotate", nil); code != http.StatusOK {
		t.Fatalf("rotate: %d", code)
	}
	d.gmKept.flush(time.Minute)
	fresh, _ := mac.keptItem()
	if fresh == "" || fresh == fakeKeptToken || !worksAsGM(d, fresh) {
		t.Fatalf("the Keychain item after a new token: changed %v, works %v", fresh != fakeKeptToken, worksAsGM(d, fresh))
	}
	again := startDeck(t)
	if worksAsGM(again, fakeKeptToken) || !worksAsGM(again, fresh) {
		t.Error("after a restart, the old token works or the new one doesn't")
	}
}

// Without the Mac app, and in every test, the deck's store reaches no real one: nothing is read, nothing written.
func TestATestNeverReachesARealStoreForTheGMsToken(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("on a Mac the system's store is none at all (gmtoken_store_darwin.go), so this can't fail here: it checks Linux and Windows, in CI")
	}
	isolate(t)
	d := startDeck(t)
	d.newGMToken()
	d.gmKept.flush(time.Minute)
	if token, err := systemGMTokenStore.read(); token != "" || err != nil {
		t.Errorf("the system's store in a test read %v, %v", token != "", err)
	}
	if _, err := os.Stat(keptGMTokenPath()); err == nil {
		t.Error("a test wrote gm-token-kept")
	}
}

// errStoreDown is a stand-in store that is down.
var errStoreDown = errors.New("no secret store")

// standInStore is a secret store that can be down.
type standInStore struct {
	value string
	down  bool
}

func (s *standInStore) store() secretStore {
	return secretStore{
		get: func() (string, error) {
			if s.down {
				return "", errStoreDown
			}
			if s.value == "" {
				return "", keyring.ErrNotFound
			}
			return s.value, nil
		},
		set: func(v string) error {
			if s.down {
				return errStoreDown
			}
			s.value = v
			return nil
		},
		delete: func() error {
			if s.down {
				return errStoreDown
			}
			s.value = ""
			return nil
		},
	}
}

// On Linux and Windows the token is in the secret store, else in a file readable by you alone, which is read first
// while it's there: it holds the last token written, and a store that came back may hold an older one.
func TestWhereTheSecretStoreCantKeepItTheTokenIsInAFileOfYoursAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), keptGMTokenName)
	secrets := &standInStore{}
	kept := storeOrFile(secrets.store(), func() string { return path })
	read := func() string {
		t.Helper()
		token, err := kept.read()
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	if err := kept.write(fakeKeptToken); err != nil || secrets.value != fakeKeptToken || read() != fakeKeptToken {
		t.Fatalf("in the store: %v, kept %v", err, secrets.value == fakeKeptToken)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a file beside a store that kept it")
	}

	secrets.down = true
	if err := kept.write(fakeOtherToken); err != nil {
		t.Fatal(err)
	}
	if err := yoursAlone(path); err != nil || read() != fakeOtherToken {
		t.Fatalf("no store: the file %v, read back %v", err, read() == fakeOtherToken)
	}
	secrets.down = false
	if read() != fakeOtherToken {
		t.Error("the store's older token was read over the file's")
	}

	if err := kept.write(fakeKeptToken); err != nil || secrets.value != fakeKeptToken {
		t.Fatalf("the store back: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("the file stays once the store keeps the token")
	}
	if err := kept.write(""); err != nil || secrets.value != "" || read() != "" {
		t.Errorf("taken away: %v, still kept %v", err, secrets.value != "")
	}
}

// A hire's fence keeps it from that file and the one it is written through on Linux and Windows, whether or not
// they're there when the hire's settings are written: a GM hired after the hire makes the file (RTOK B2). A Mac
// never keeps the token there, and its fence is the Mac's, byte for byte.
func TestAHireIsFencedFromTheKeptTokensFile(t *testing.T) {
	support := t.TempDir()
	root := absoluteRulePath(support)
	for _, system := range []string{"linux", "windows"} {
		for _, entries := range [][]string{{"briefs"}, {"briefs", keptGMTokenName, keptGMTokenTemp}} {
			rules := hireDenyRules("abc123", nil, support, nil, nil, entries, system)
			for _, name := range []string{keptGMTokenName, keptGMTokenTemp} {
				for _, want := range []string{"Read(" + root + "/" + name + ")", "Edit(" + root + "/" + name + ")"} {
					if !slices.Contains(rules, want) {
						t.Errorf("%s, with %v in the folder: no %s", system, entries, want)
					}
				}
			}
			if slices.ContainsFunc(rules, func(rule string) bool {
				return strings.Contains(rule, keptGMTokenName+"/") || strings.Contains(rule, keptGMTokenTemp+"/")
			}) {
				t.Errorf("%s: the file is fenced as a folder", system)
			}
		}
	}
	if mac := hireDenyRules("abc123", nil, support, nil, nil, []string{"briefs"}, "darwin"); slices.ContainsFunc(mac, func(rule string) bool {
		return strings.Contains(rule, keptGMTokenName)
	}) {
		t.Error("the Mac's fence has rules for a file it never has")
	}
}

// refuseWrites has the stand-in Mac app refuse to change its items, as a locked Keychain would, or, with why "",
// change them again.
func (m *macStandIn) refuseWrites(why string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if why == "" {
		m.refuse = nil
		return
	}
	m.refuse = map[string]string{"secret-write": why, "secret-delete": why}
}

// A new token the Keychain refused is not given: the page is told (500), the old token goes on working, and it is
// still the one kept after a restart, so it is never said to be dead while it can come back (RTOK B1). Once the
// Keychain takes it, the new token is kept before the page is told, and the old one never works again.
func TestANewGMTokenTheKeychainRefusedIsNotGiven(t *testing.T) {
	isolate(t)
	mac := withMac(t)
	mac.secrets = map[string]string{gmTokenItem: fakeKeptToken}
	withHiredGM(t, "working")
	d := startDeck(t)
	managerOn(d)
	mac.refuseWrites("The Keychain is locked.")
	code, said, body := fromPage(t, d, http.MethodPost, "/setup/gm/token/rotate", nil)
	if code != http.StatusInternalServerError || said["rotated"] == true || !strings.Contains(body, "the old one still works") {
		t.Errorf("a new token the Keychain refused: %d %q", code, body)
	}
	if kept, _ := mac.keptItem(); !worksAsGM(d, fakeKeptToken) || kept != fakeKeptToken {
		t.Fatalf("after the refusal, the old token works %v, is still kept %v", worksAsGM(d, fakeKeptToken), kept == fakeKeptToken)
	}
	if !worksAsGM(startDeck(t), fakeKeptToken) {
		t.Error("after the refusal and a restart, the running GM's token is refused")
	}

	mac.refuseWrites("")
	if code, _, _ := fromPage(t, d, http.MethodPost, "/setup/gm/token/rotate", nil); code != http.StatusOK {
		t.Fatalf("rotate with the Keychain back: %d", code)
	}
	// No flush: the store was written before the page was answered.
	fresh, _ := mac.keptItem()
	if fresh == fakeKeptToken || !worksAsGM(d, fresh) {
		t.Fatalf("the Keychain item after the rotate: changed %v, works %v", fresh != fakeKeptToken, worksAsGM(d, fresh))
	}
	again := startDeck(t)
	if worksAsGM(again, fakeKeptToken) || !worksAsGM(again, fresh) {
		t.Errorf("after a restart, the old token works %v, the new one %v", worksAsGM(again, fakeKeptToken), worksAsGM(again, fresh))
	}
}

// With a GM started by hand allowed, a GM fired while the Keychain refuses every change never comes back with its
// token after a restart: the gm-token file, written at once, holds its replacement, and wins over the Keychain's
// item, which may lag (RTOK B1). The item follows once the Keychain takes changes again.
func TestAGMFiredWhileTheKeychainRefusesNeverComesBackWithItsToken(t *testing.T) {
	isolate(t)
	mac := withMac(t)
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	managerOn(d)
	if code, _, _ := fromPage(t, d, http.MethodPost, "/setup/gm/by-hand?enabled=1", nil); code != http.StatusOK {
		t.Fatalf("by hand on: %d", code)
	}
	fired := d.newGMToken()
	withHiredGM(t, "working")
	d.gmKept.flush(time.Minute)
	if kept, _ := mac.keptItem(); kept != fired || tokenInFile(t) != fired {
		t.Fatalf("the hired GM's token is the Keychain item's %v, the file's %v", kept == fired, tokenInFile(t) == fired)
	}

	mac.refuseWrites("The Keychain is locked.")
	d.clearGMTokenIf(fired)
	withHiredGM(t, "fired")
	replacement := tokenInFile(t)
	if replacement == "" || replacement == fired {
		t.Fatalf("by hand, after the fire: a replacement in the file %v", replacement != "")
	}
	if d.gmKept.flush(10 * time.Millisecond) {
		t.Error("the keeper says the replacement is kept while the Keychain refuses it")
	}
	again := startDeck(t)
	if worksAsGM(again, fired) {
		t.Error("after a restart, the fired GM's token works")
	}
	if !worksAsGM(again, replacement) || tokenInFile(t) != replacement {
		t.Errorf("after a restart, the file's replacement works %v, is still the file's %v", worksAsGM(again, replacement), tokenInFile(t) == replacement)
	}

	mac.refuseWrites("")
	if !again.gmKept.flush(time.Minute) || !d.gmKept.flush(time.Minute) {
		t.Fatal("the Keychain is back, and the token still isn't kept")
	}
	if kept, _ := mac.keptItem(); kept != replacement {
		t.Error("the Keychain is back, and its item isn't the replacement")
	}
}

// A GM fired and replaced while the Keychain refuses every change is never brought back as the old one after a
// restart (RTOK2 B1): a new token is kept before it is given, so the hire that wanted it is refused, and the fired
// GM's token, still the one kept, finds no open GM to be restored for.
func TestAGMReplacedWhileTheKeychainRefusesIsRefusedAndTheOldTokenStaysDead(t *testing.T) {
	isolate(t)
	mac := withMac(t)
	d := startDeck(t)
	old := d.newGMToken()
	withHiredGM(t, "working")
	d.gmKept.flush(time.Minute)
	if kept, _ := mac.keptItem(); old == "" || kept != old {
		t.Fatalf("the first GM's token isn't the Keychain item's (made %v)", old != "")
	}

	mac.refuseWrites("The Keychain is locked.")
	d.clearGMTokenIf(old)
	withHiredGM(t, "fired")
	fresh := d.newGMToken()
	if fresh != "" {
		// The hire went ahead on a token the Keychain didn't take.
		withHiredGM(t, "working")
	}
	if fresh != "" || worksAsGM(d, old) {
		t.Errorf("a new GM's token while the Keychain refuses: given %v, the fired GM's token works %v", fresh != "", worksAsGM(d, old))
	}
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.Role == gmRole && h.State != "fired" {
			t.Errorf("a new GM is open (%s)", h.State)
		}
	}

	d.gmKept.flush(10 * time.Millisecond)
	again := startDeck(t)
	if worksAsGM(again, old) {
		t.Error("after a restart, the fired GM's token works")
	}
	if fresh != "" && worksAsGM(again, fresh) {
		t.Error("after a restart, the new GM's token works though the Keychain refused it")
	}

	// With the Keychain back, a GM can be hired again, and its token is kept before it is given.
	mac.refuseWrites("")
	next := d.newGMToken()
	if kept, _ := mac.keptItem(); next == "" || kept != next || !worksAsGM(d, next) {
		t.Errorf("with the Keychain back: a token given %v, kept %v", next != "", kept == next)
	}
}

// A write the store refused is tried again until one succeeds; until then the keeper doesn't take the store to hold
// the token, and a quit's flush says it isn't kept. A change asked meanwhile is the one written.
func TestAWriteTheStoreRefusedIsTriedAgainUntilItIsKept(t *testing.T) {
	var mu sync.Mutex
	down, held, writes := true, "", 0
	k := newGMTokenKeeper(gmTokenStore{
		read: func() (string, error) { return "", nil },
		write: func(token string) error {
			mu.Lock()
			defer mu.Unlock()
			writes++
			if down {
				return errStoreDown
			}
			held = token
			return nil
		},
	})
	k.retry = time.Millisecond
	k.restore(true)
	k.keep(fakeKeptToken)
	if k.flush(20 * time.Millisecond) {
		t.Error("flushed while the store refuses every write")
	}
	k.keep(fakeOtherToken)
	k.mu.Lock()
	known := k.known
	k.mu.Unlock()
	mu.Lock()
	tried := writes
	down = false
	mu.Unlock()
	if known || tried < 2 {
		t.Errorf("after refused writes: taken as kept %v, tried %d times", known, tried)
	}
	if !k.flush(time.Minute) {
		t.Fatal("the store is back, and the token isn't kept")
	}
	mu.Lock()
	defer mu.Unlock()
	if held != fakeOtherToken {
		t.Errorf("the store holds the latest change %v", held == fakeOtherToken)
	}
}

// The Mac app has an item it couldn't read (a locked Keychain): with a GM started by hand allowed and no file to fall
// back on, no token is made in its place and nothing is written over it, as the Swift engine did (RTOK N2). With the
// file, the file's token is used, still with nothing written. A GM hired then has its token kept as usual.
func TestAKeychainItemThatCouldntBeReadIsNeverWrittenOver(t *testing.T) {
	var answer bridgeAnswer
	if err := json.Unmarshal([]byte(`{"ok": true, "found": false, "failed": true}`), &answer); err != nil || !answer.Failed {
		t.Fatalf("the app's answer for an item it couldn't read: %v %+v", err, answer)
	}
	isolate(t)
	mac := withMac(t)
	mac.secrets = map[string]string{gmTokenItem: fakeKeptToken}
	mac.lookFailed = map[string]bool{gmTokenItem: true}
	config := loadConfig()
	config.GMByHand = true
	config.save()
	withHiredGM(t, "working")
	changed := func() bool {
		return slices.ContainsFunc(mac.actions(), func(action string) bool { return action == "secret-write" || action == "secret-delete" })
	}

	d := startDeck(t)
	d.gmKept.flush(time.Minute)
	d.mu.Lock()
	token := d.gmToken
	d.mu.Unlock()
	if token != "" || tokenInFile(t) != "" || changed() {
		t.Fatalf("an item that couldn't be read: a token made %v, a file %v, the item changed %v", token != "", tokenInFile(t) != "", changed())
	}

	if err := os.WriteFile(gmTokenPath(), []byte(fakeOtherToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withFile := startDeck(t)
	withFile.gmKept.flush(time.Minute)
	if !worksAsGM(withFile, fakeOtherToken) || changed() {
		t.Fatalf("with the file: its token works %v, the item changed %v", worksAsGM(withFile, fakeOtherToken), changed())
	}

	hired := withFile.newGMToken()
	withFile.gmKept.flush(time.Minute)
	if kept, _ := mac.keptItem(); kept != hired {
		t.Error("a GM hired after the item couldn't be read has its token not kept")
	}
}

// The kept token's file is written through a file made for it alone: one left behind with a wider mode, or a link
// put in its place, changes nothing (RTOK N1).
func TestTheKeptTokensFileIsNeverWrittenThroughAnotherOne(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, keptGMTokenName)
	if err := os.WriteFile(path+".tmp", []byte("left behind\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeKeptToken(path, []byte(fakeKeptToken+"\n")); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); yoursAlone(path) != nil || validGMToken(string(data)) != fakeKeptToken {
		t.Fatalf("over a temp file left behind: %v", yoursAlone(path))
	}
	if _, err := os.Lstat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Error("the temp file is left")
	}
	if runtime.GOOS == "windows" {
		return
	}
	elsewhere := filepath.Join(dir, "elsewhere")
	if err := os.WriteFile(elsewhere, []byte("untouched\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, path+".tmp"); err != nil {
		t.Fatal(err)
	}
	if err := writeKeptToken(path, []byte(fakeOtherToken+"\n")); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(elsewhere); string(data) != "untouched\n" {
		t.Error("the token was written through a link at the temp file")
	}
	if data, _ := os.ReadFile(path); validGMToken(string(data)) != fakeOtherToken {
		t.Error("the kept file doesn't hold the new token")
	}
}
