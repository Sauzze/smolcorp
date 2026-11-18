package desktop

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// keyedOffice is a deck with three desks in the registry: Ann (aaa111) and Bea (bbb222), each started with a hook
// key of its own, and Cy (ccc333), a desk from before the update, with none. It gives each key and each tmux socket.
func keyedOffice(t *testing.T) (d *Deck, keys, sockets map[string]string) {
	t.Helper()
	d = newTestDeck(t)
	// Its links resolved, as the core spells a sandboxed desk's socket: a desk's tab through a link (the Mac's
	// /var, here) counts only as the core recorded it (FXTABLINK).
	dir := physical(t.TempDir())
	keys, sockets = map[string]string{}, map[string]string{}
	var hires []Hire
	for _, id := range []string{"aaa111", "bbb222", "ccc333"} {
		sockets[id] = filepath.Join(dir, hireSocketPrefix+id)
		hires = append(hires, Hire{ID: id, Name: "Desk " + id, Role: "janitor", Folder: "/proj", Repo: "/proj",
			Worktree: filepath.Join(dir, "wt-"+id), TmuxSocket: sockets[id], State: "working"})
	}
	if err := updateRegistry(func(f *registryFile) { f.hires = hires }); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"aaa111", "bbb222"} {
		key, err := newHookKey(registryPath(), id)
		if err != nil || key == "" {
			t.Fatalf("%s: %q %v", id, key, err)
		}
		keys[id] = key
	}
	return d, keys, sockets
}

// hookAs posts a hook event from this computer with key, for session id in folder cwd, on tab ("" for none).
func hookAs(t *testing.T, d *Deck, key, tab, name, id, cwd string) int {
	t.Helper()
	body := fmt.Sprintf(`{"hook_event_name":%q,"session_id":%q,"cwd":%q,"prompt":"hi"}`, name, id, cwd)
	r := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	r.Host, r.RemoteAddr = "localhost:7710", thisComputer
	r.Header.Set(hookKeyHeader, key)
	r.Header.Set("X-AgentDeck-Project", cwd)
	if tab != "" {
		r.Header.Set("X-AgentDeck-Tab", tab)
	}
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	if w.Code == http.StatusForbidden && w.Body.String() != hookNotYourDesk {
		t.Errorf("the refusal said %q", w.Body.String())
	}
	return w.Code
}

func tmuxTabOn(socket string) string { return "tmux:%1@" + socket }

// teamSession is the session the team pairs hire id with, "" for none.
func teamSession(d *Deck, id string) string {
	for _, row := range buildTeam(readRegistry(registryPath(), false).hires, d.frontDeskLive(), 0, func(string) string { return "" }) {
		if row["id"] == id {
			s, _ := row["sessionId"].(string)
			return s
		}
	}
	return ""
}

// The forged-event rows (X10, AQ3): a desk reports with its own key from its own tab, and can't send an event as
// another desk, by that desk's tab, its session, or its folder; nor with the owner's sessions' key, which reaches no
// desk with a key. A desk from before the update goes on as before.
func TestADeskCantSendAnEventAsAnotherDesk(t *testing.T) {
	d, keys, sockets := keyedOffice(t)
	ann, bea := keys["aaa111"], keys["bbb222"]
	annTab, beaTab, cyTab := tmuxTabOn(sockets["aaa111"]), tmuxTabOn(sockets["bbb222"]), tmuxTabOn(sockets["ccc333"])
	annWork, beaWork := filepath.Join(filepath.Dir(sockets["aaa111"]), "wt-aaa111"), filepath.Join(filepath.Dir(sockets["aaa111"]), "wt-bbb222")
	owner := d.access.HookKey

	// Each desk from its own tab, with its own key.
	if got := hookAs(t, d, ann, annTab, "SessionStart", "s-ann", annWork); got != http.StatusNoContent {
		t.Fatalf("Ann on her own tab: %d", got)
	}
	if teamSession(d, "aaa111") != "s-ann" {
		t.Errorf("Ann's row: %q", teamSession(d, "aaa111"))
	}
	// Ann on Bea's tab: refused, and nothing made.
	if got := hookAs(t, d, ann, beaTab, "SessionStart", "s-forged", beaWork); got != http.StatusForbidden || d.sessions.Get("s-forged") != nil {
		t.Errorf("Ann on Bea's tab: %d", got)
	}
	// Ann, from her own tab, in Bea's folder: hers, never Bea's (Bea's row claims only what's on her own tab).
	if got := hookAs(t, d, ann, annTab, "SessionStart", "s-in-bea", beaWork); got != http.StatusNoContent {
		t.Fatalf("Ann in Bea's folder: %d", got)
	}
	if got := teamSession(d, "bbb222"); got != "" {
		t.Errorf("Bea's row took %q", got)
	}
	// Asking you from there, it is never shown as Bea asking.
	if got := hookAs(t, d, ann, annTab, "PermissionRequest", "s-in-bea", beaWork); got != http.StatusNoContent {
		t.Fatalf("Ann's prompt: %d", got)
	}
	for _, it := range d.approvalItems() {
		if it.session == "s-in-bea" && it.hire == "bbb222" {
			t.Errorf("Ann's prompt is shown as Bea's: %+v", it)
		}
	}
	if got := hookAs(t, d, bea, beaTab, "SessionStart", "s-bea", beaWork); got != http.StatusNoContent {
		t.Fatalf("Bea on her own tab: %d", got)
	}
	if got := teamSession(d, "bbb222"); got != "s-bea" {
		t.Errorf("Bea's row: %q", got)
	}
	// Ann naming Bea's session, from her own tab: refused, and Bea's session is as it was.
	if got := hookAs(t, d, ann, annTab, "UserPromptSubmit", "s-bea", beaWork); got != http.StatusForbidden {
		t.Errorf("Ann naming Bea's session: %d", got)
	}
	if got := hookAs(t, d, ann, annTab, "SessionEnd", "s-bea", beaWork); got != http.StatusForbidden {
		t.Errorf("Ann ending Bea's session: %d", got)
	}
	if s := d.sessions.Get("s-bea"); s == nil || s.Tab != beaTab || s.IsBusy() {
		t.Errorf("Bea's session after Ann's tries: %+v", s)
	}

	// The owner's sessions' key: on Bea's tab, or naming Bea's session, refused.
	if got := hookAs(t, d, owner, beaTab, "SessionStart", "s-owner-on-bea", beaWork); got != http.StatusForbidden {
		t.Errorf("the owner's key on Bea's tab: %d", got)
	}
	if got := hookAs(t, d, owner, "other:/dev/ttys009", "UserPromptSubmit", "s-bea", beaWork); got != http.StatusForbidden {
		t.Errorf("the owner's key naming Bea's session: %d", got)
	}
	if s := d.sessions.Get("s-bea"); s == nil || s.Tab != beaTab || s.IsBusy() {
		t.Errorf("Bea's session after the owner's key: %+v", s)
	}
	// In a tab of the owner's own, in Bea's folder: a session of the owner's, which Bea's row never takes.
	if got := hookAs(t, d, owner, "other:/dev/ttys009", "SessionStart", "s-owner", beaWork); got != http.StatusNoContent {
		t.Errorf("the owner's own session: %d", got)
	}
	if got := teamSession(d, "bbb222"); got != "s-bea" {
		t.Errorf("Bea's row: %q", got)
	}
	// A desk's key doesn't reach the owner's own session either.
	if got := hookAs(t, d, ann, annTab, "UserPromptSubmit", "s-owner", beaWork); got != http.StatusForbidden {
		t.Errorf("Ann naming the owner's session: %d", got)
	}

	// Cy, from before the update, has no key: the owner's key still reports for it, as before.
	if got := hookAs(t, d, owner, cyTab, "SessionStart", "s-cy", "/proj"); got != http.StatusNoContent {
		t.Errorf("an old desk: %d", got)
	}
	// A desk's key with no tab: only for a session already on its own tab (s-in-bea took s-ann's place there).
	if got := hookAs(t, d, ann, "", "UserPromptSubmit", "s-in-bea", annWork); got != http.StatusNoContent {
		t.Errorf("Ann with no tab, her own session: %d", got)
	}
	if got := hookAs(t, d, ann, "", "SessionStart", "s-tabless", annWork); got != http.StatusForbidden {
		t.Errorf("Ann with no tab, a new session: %d", got)
	}
	// A headless run a desk started reports with the desk's key, as its own.
	if got := hookAs(t, d, ann, "headless:s-run", "SessionStart", "s-run", annWork); got != http.StatusNoContent {
		t.Errorf("Ann's headless run: %d", got)
	}
	// A key nobody was given, or a fired desk's, is no key.
	if got := hookAs(t, d, "hook-made-up", annTab, "SessionStart", "s-x", annWork); got != http.StatusUnauthorized {
		t.Errorf("a made-up key: %d", got)
	}
	if err := updateRegistry(func(f *registryFile) { f.hires[0].State = "fired" }); err != nil {
		t.Fatal(err)
	}
	if got := hookAs(t, d, ann, annTab, "UserPromptSubmit", "s-in-bea", annWork); got != http.StatusUnauthorized {
		t.Errorf("a fired desk's key: %d", got)
	}
}

// A desk in a launcher's window is known by its launcher: only its own key reports there.
func TestALaunchersDeskReportsWithItsOwnKeyOnly(t *testing.T) {
	d, keys, _ := keyedOffice(t)
	d.mu.Lock()
	d.launchers["L1"] = &launcherTab{agent: Claude, folder: "/proj", hireID: "aaa111"}
	d.mu.Unlock()
	if got := hookAs(t, d, keys["bbb222"], "launcher:L1", "SessionStart", "s-1", "/proj"); got != http.StatusForbidden {
		t.Errorf("Bea in Ann's launcher: %d", got)
	}
	if got := hookAs(t, d, d.access.HookKey, "launcher:L1", "SessionStart", "s-1", "/proj"); got != http.StatusForbidden {
		t.Errorf("the owner's key in Ann's launcher: %d", got)
	}
	if got := hookAs(t, d, keys["aaa111"], "launcher:L1", "SessionStart", "s-1", "/proj"); got != http.StatusNoContent {
		t.Errorf("Ann in her own launcher: %d", got)
	}
}

// A launcher found by folder, for an event that names no tab, is checked as the tab it is: the owner's key is never
// placed on a desk's launcher by folder (RT126SAX10 B2). A launcher of a desk from before the update still takes it.
func TestALauncherFoundByFolderIsCheckedAsItsDesks(t *testing.T) {
	d, _, _ := keyedOffice(t)
	d.mu.Lock()
	d.launchers["L1"] = &launcherTab{agent: Claude, folder: "/proj-ann", hireID: "aaa111"}
	d.launchers["L2"] = &launcherTab{agent: Claude, folder: "/proj-cy", hireID: "ccc333"}
	d.mu.Unlock()
	owner := d.access.HookKey
	if got := hookAs(t, d, owner, "", "SessionStart", "s-by-folder", "/proj-ann"); got != http.StatusForbidden {
		t.Errorf("the owner's key, no tab, in Ann's launcher's folder: %d", got)
	}
	if s := d.sessions.Get("s-by-folder"); s != nil {
		t.Errorf("it landed on %q", s.Tab)
	}
	// The same way to Cy's launcher, whose desk has no key: found by its folder, as before.
	if got := hookAs(t, d, owner, "", "SessionStart", "s-cy", "/proj-cy"); got != http.StatusNoContent {
		t.Errorf("the owner's key, no tab, in Cy's launcher's folder: %d", got)
	}
	if s := d.sessions.Get("s-cy"); s == nil || s.Tab != "launcher:L2" {
		t.Errorf("Cy's session: %+v", s)
	}
}

// A desk's session dropped a while ago (its tab looked gone, after a restart or Hand back) would come back as it was,
// on the desk's own tab, when an event names it with no tab: it is checked as that desk's session, so neither the
// owner's key nor another desk's brings it back or asks you in its name (RT126SAX10 B1). Its own desk's key does.
func TestADesksDroppedSessionComesBackForItsOwnKeyOnly(t *testing.T) {
	d, keys, sockets := keyedOffice(t)
	ann, bea, owner := keys["aaa111"], keys["bbb222"], d.access.HookKey
	annTab, beaTab := tmuxTabOn(sockets["aaa111"]), tmuxTabOn(sockets["bbb222"])
	beaWork := filepath.Join(filepath.Dir(sockets["bbb222"]), "wt-bbb222")
	drop := func(id string) {
		t.Helper()
		d.mu.Lock()
		defer d.mu.Unlock()
		s := d.sessions.Get(id)
		if s == nil {
			t.Fatalf("no %s to drop", id)
		}
		d.sessions.drop(s, nowMillis()-1000)
	}
	if got := hookAs(t, d, bea, beaTab, "SessionStart", "s-bea", beaWork); got != http.StatusNoContent {
		t.Fatalf("Bea on her own tab: %d", got)
	}
	drop("s-bea")
	for _, try := range []struct{ who, key, tab string }{
		{"the owner's key, no tab", owner, ""},
		{"the owner's key, a tab of the owner's", owner, "other:/dev/ttys009"},
		{"Ann's key, no tab", ann, ""},
		{"Ann's key, her own tab", ann, annTab},
	} {
		if got := hookAs(t, d, try.key, try.tab, "PermissionRequest", "s-bea", beaWork); got != http.StatusForbidden {
			t.Errorf("%s, Bea's dropped session: %d", try.who, got)
		}
		d.mu.Lock()
		back, waiting := d.sessions.Get("s-bea"), d.sessions.WasDropped("s-bea")
		d.mu.Unlock()
		if back != nil || !waiting {
			t.Errorf("after %s: back=%+v still dropped=%v", try.who, back, waiting)
		}
		if got := teamSession(d, "bbb222"); got != "" {
			t.Errorf("after %s, Bea's row: %q", try.who, got)
		}
		for _, it := range d.approvalItems() {
			if it.session == "s-bea" {
				t.Errorf("after %s, a prompt in Bea's name: %+v", try.who, it)
			}
		}
	}
	// Bea's own key, with no tab, brings it back on her tab, as before.
	if got := hookAs(t, d, bea, "", "UserPromptSubmit", "s-bea", beaWork); got != http.StatusNoContent {
		t.Errorf("Bea's key, her dropped session: %d", got)
	}
	if s := d.sessions.Get("s-bea"); s == nil || s.Tab != beaTab {
		t.Errorf("Bea's session back: %+v", s)
	}
	if got := teamSession(d, "bbb222"); got != "s-bea" {
		t.Errorf("Bea's row: %q", got)
	}
	// The owner's own dropped session comes back for the owner's key with no tab, and for no desk's key.
	if got := hookAs(t, d, owner, "other:/dev/ttys009", "SessionStart", "s-owner", "/proj"); got != http.StatusNoContent {
		t.Fatalf("the owner's own session: %d", got)
	}
	drop("s-owner")
	if got := hookAs(t, d, ann, annTab, "UserPromptSubmit", "s-owner", "/proj"); got != http.StatusForbidden {
		t.Errorf("Ann's key, the owner's dropped session: %d", got)
	}
	if got := hookAs(t, d, owner, "", "UserPromptSubmit", "s-owner", "/proj"); got != http.StatusNoContent {
		t.Errorf("the owner's key, the owner's dropped session: %d", got)
	}
	if s := d.sessions.Get("s-owner"); s == nil || s.Tab != "other:/dev/ttys009" {
		t.Errorf("the owner's session back: %+v", s)
	}
}

// A desk's key is for its hook alone: the routes the owner's sessions' key opens stay shut to it.
func TestADesksHookKeyOpensNothingButTheHook(t *testing.T) {
	d, keys, _ := keyedOffice(t)
	for _, path := range []string{"/launcher", hireStartPath} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Host, r.RemoteAddr = "localhost:7710", thisComputer
		r.Header.Set(hookKeyHeader, keys["aaa111"])
		w := httptest.NewRecorder()
		(&router{deck: d}).ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s with a desk's key: %d", path, w.Code)
		}
	}
}

// The registry keeps only each key's hash, sealed with the send keys: the key still counts after the core restarts,
// a new start's key replaces the old, and a registry whose seal doesn't fit grants no key.
func TestADesksHookKeyIsKeptAsItsHashAndRenewedAtEachStart(t *testing.T) {
	d, keys, sockets := keyedOffice(t)
	data, err := os.ReadFile(registryPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), keys["aaa111"]) || !strings.Contains(string(data), hashKey(keys["aaa111"])) {
		t.Errorf("the registry:\n%s", data)
	}
	forgetHeldRegistries() // a new start of the core
	annTab, annWork := tmuxTabOn(sockets["aaa111"]), filepath.Join(filepath.Dir(sockets["aaa111"]), "wt-aaa111")
	if got := hookAs(t, d, keys["aaa111"], annTab, "SessionStart", "s-ann", annWork); got != http.StatusNoContent {
		t.Errorf("after a restart: %d", got)
	}
	old := readRegistry(registryPath(), false)
	renewed, err := newHookKey(registryPath(), "aaa111")
	if err != nil || renewed == keys["aaa111"] {
		t.Fatalf("%q %v", renewed, err)
	}
	if !registryTightens(old, readRegistry(registryPath(), false)) {
		t.Error("a renewed hook key doesn't count as taking authority away")
	}
	if got := hookAs(t, d, keys["aaa111"], annTab, "UserPromptSubmit", "s-ann", annWork); got != http.StatusUnauthorized {
		t.Errorf("the old key: %d", got)
	}
	if got := hookAs(t, d, renewed, annTab, "UserPromptSubmit", "s-ann", annWork); got != http.StatusNoContent {
		t.Errorf("the new key: %d", got)
	}
	if left := withoutGrants(readRegistry(registryPath(), false)); len(left.hookKeys) != 0 {
		t.Errorf("a registry that grants nothing still holds %v", left.hookKeys)
	}
	if _, err := newHookKey(registryPath(), "nobody"); err == nil {
		t.Error("a key for a hire that isn't there")
	}
	forgetHire("bbb222")
	if _, ok := readRegistry(registryPath(), false).hookKeys["bbb222"]; ok {
		t.Error("a forgotten hire's hook key stays")
	}
}

// A desk's start gives its own hook key in its environment, and no desk inherits another's from the core's.
func TestADesksStartHoldsItsOwnHookKey(t *testing.T) {
	isolate(t)
	plan, err := hireStart{Role: testHireRole(), Message: "go", HookKey: "hook-fake-own"}.plan()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(plan.Changes, envChange{Name: hookKeyEnvironment, Value: "hook-fake-own"}) {
		t.Errorf("its changes: %+v", plan.Changes)
	}
	t.Setenv(hookKeyEnvironment, "hook-fake-someone-elses")
	if slices.ContainsFunc(hireEnvironment(), func(v string) bool { return strings.HasPrefix(v, hookKeyEnvironment+"=") }) {
		t.Error("a desk's environment carries the core's hook key")
	}
}

// The hook sends its desk's key, and then never reads the file; without one, the file's, as before.
func TestTheHookSendsItsDesksKeyAndThenNeverReadsTheFile(t *testing.T) {
	isolate(t)
	env := map[string]string{hookKeyEnvironment: " hook-fake-desk\n"}
	key, err := hookKeyFor(func(name string) string { return env[name] }, func(path string) ([]byte, error) {
		t.Errorf("read %s", path)
		return nil, errors.New("no")
	})
	if err != nil || key != "hook-fake-desk" {
		t.Errorf("%q %v", key, err)
	}
	delete(env, hookKeyEnvironment)
	read := ""
	key, err = hookKeyFor(func(name string) string { return env[name] }, func(path string) ([]byte, error) {
		read = path
		return []byte("hook-fake-owner\n"), nil
	})
	if err != nil || key != "hook-fake-owner" || read != hookKeyFile {
		t.Errorf("%q %v %q", key, err, read)
	}
}

// On a Mac the seal covers each desk's hook key as it does the send keys: one another program plants in
// registry.json counts for nothing at the next start, nor does a desk's old key in a copy from before its new start.
func TestOnAMacAPlantedOrOldHookKeyDoesntCount(t *testing.T) {
	keyed := func(t *testing.T) string {
		t.Helper()
		macDeck(t)
		withAHire(t)
		key, err := newHookKey(registryPath(), "hire01")
		if err != nil {
			t.Fatal(err)
		}
		restart(t)
		if currentHookDesks().deskForKey(key) != "hire01" {
			t.Fatal("an untouched hook key stopped counting across a restart")
		}
		return key
	}
	t.Run("planted", func(t *testing.T) {
		keyed(t)
		planted := "hook-fake-planted"
		rewrite(t, registryName, func(f map[string]any) {
			f["hookKeys"].(map[string]any)["hire02"] = hashKey(planted)
		})
		restart(t)
		if currentHookDesks().deskForKey(planted) != "" {
			t.Error("a planted hook key counts")
		}
	})
	t.Run("from before", func(t *testing.T) {
		old := keyed(t)
		before := sealedCopies(t)
		if _, err := newHookKey(registryPath(), "hire01"); err != nil {
			t.Fatal(err)
		}
		putBack(t, before)
		restart(t)
		if currentHookDesks().deskForKey(old) != "" {
			t.Error("the hook key from before the desk's new start works")
		}
	})
}
