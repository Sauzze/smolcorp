package desktop

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
	d.gmKept.flush()
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
	again.gmKept.flush()
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
	d.gmKept.flush()
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
	gone.gmKept.flush()
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
	d.gmKept.flush()
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
	isolate(t)
	d := startDeck(t)
	d.newGMToken()
	d.gmKept.flush()
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
	info, err := os.Stat(path)
	if err != nil || !modeIs(info, 0o600) || read() != fakeOtherToken {
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

// A hire's fence keeps it from that file when it's there (only where a computer has no secret store).
func TestAHireIsFencedFromTheKeptTokensFile(t *testing.T) {
	support := t.TempDir()
	root := absoluteRulePath(support)
	rules := hireDenyRules("abc123", nil, support, nil, nil, []string{"briefs", keptGMTokenName})
	for _, want := range []string{"Read(" + root + "/" + keptGMTokenName + ")", "Edit(" + root + "/" + keptGMTokenName + ")"} {
		if !slices.Contains(rules, want) {
			t.Errorf("no %s in %v", want, rules)
		}
	}
	if slices.ContainsFunc(rules, func(rule string) bool { return strings.Contains(rule, keptGMTokenName+"/") }) {
		t.Error("the file is fenced as a folder")
	}
	if without := hireDenyRules("abc123", nil, support, nil, nil, []string{"briefs"}); slices.ContainsFunc(without, func(rule string) bool {
		return strings.Contains(rule, keptGMTokenName)
	}) {
		t.Error("fenced from a file that isn't there")
	}
}
