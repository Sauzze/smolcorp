package sandbox

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// FXPLANTSTERM (RT126SWEEP2 P2's F1, P1's B4 and P4's F1), on this Mac with stand-ins only: under the real profile, a
// desk changes nothing the owner's terminals read when a window starts (Smolcorp opens Terminal, iTerm2 and Ghostty
// windows itself, outside any sandbox, and drives WezTerm and kitty), nothing in a hired agent's own settings, and
// nothing in the keychains' folder. Each place is a stand-in in the desk's stand-in home; nothing here opens or
// launches an app, and no row writes a real app's preferences. The owner's files are still read, and the desk's own
// work still runs.

// The places, refused; their reads and the desk's own work, not.
func TestOnAMacAHireCantPlantWhatTheOwnersTerminalsAgentsAndKeychainsRun(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	at := func(parts ...string) string { return filepath.Join(append([]string{d.home}, parts...)...) }
	prefs, appSupport, keychains := at("Library", "Preferences"), at("Library", "Application Support"), at("Library", "Keychains")
	iterm, ghostty := filepath.Join(appSupport, "iTerm2"), filepath.Join(appSupport, "com.mitchellh.ghostty")
	owners := map[string]string{
		filepath.Join(prefs, "com.apple.Terminal.plist"):          "the owner's Terminal profiles",
		filepath.Join(iterm, "DynamicProfiles", "owner.json"):     "{}",
		filepath.Join(iterm, "Scripts", "AutoLaunch", "owner.py"): "# the owner's",
		filepath.Join(ghostty, "config"):                          "# the owner's",
		at(".config", "ghostty", "config"):                        "# the owner's",
		at(".config", "iterm2", "notes"):                          "the owner's",
		at(".codex", "config.toml"):                               "# the owner's",
		at(".gemini", "settings.json"):                            "{}",
		at(".config", "opencode", "opencode.json"):                "{}",
		filepath.Join(keychains, "login.keychain-db"):             "a stand-in keychain",
		filepath.Join(d.root, "draft.txt"):                        "the desk's",
	}
	for file, text := range owners {
		mustWrite(t, file, text)
	}
	staged := filepath.Join(d.root, "staged")
	mustWrite(t, filepath.Join(staged, "config"), "a desk's")

	d.check(t, []escapeRow{
		// The positive controls: the desk's own work, the home folder's other places, and the owner's settings read.
		{"control: its worktree writes", attempt{"create", filepath.Join(d.root, "new.go"), ""}, "ok"},
		{"control: a file in ~/Library", attempt{"create", at("Library", "x"), ""}, "ok"},
		{"control: a file in ~/.config", attempt{"create", at(".config", "x"), ""}, "ok"},
		{"control: a neighbour of ~/.wezterm.lua", attempt{"create", at(".wezterm.lua-notes"), ""}, "ok"},
		{"control: Terminal's preferences read", attempt{"read", filepath.Join(prefs, "com.apple.Terminal.plist"), ""}, "ok"},
		{"control: Codex's settings read", attempt{"read", at(".codex", "config.toml"), ""}, "ok"},
		{"control: the login keychain read", attempt{"read", filepath.Join(keychains, "login.keychain-db"), ""}, "ok"},

		// Terminal and iTerm2's preferences, and every app's named preferences, as files.
		{"Terminal's preferences", attempt{"write", filepath.Join(prefs, "com.apple.Terminal.plist"), ""}, "EPERM"},
		{"iTerm2's preferences, made", attempt{"create", filepath.Join(prefs, "com.googlecode.iterm2.plist"), ""}, "EPERM"},
		{"a copy to rename onto Terminal's", attempt{"create", filepath.Join(prefs, "com.apple.Terminal.plist.Xa1b2c"), ""}, "EPERM"},
		{"Terminal's preferences, by a rename", attempt{"rename", filepath.Join(d.root, "draft.txt"), filepath.Join(prefs, "com.apple.Terminal.plist")}, "EPERM"},
		{"kitty's Mac folder", attempt{"mkdir", filepath.Join(prefs, "kitty"), ""}, "EPERM"},
		{"~/Library/Preferences moved aside", attempt{"rename", prefs, at("Library", "old-prefs")}, "EPERM"},
		// iTerm2's dynamic profiles and AutoLaunch scripts, and its link in ~/.config.
		{"an iTerm2 dynamic profile", attempt{"create", filepath.Join(iterm, "DynamicProfiles", "p.json"), ""}, "EPERM"},
		{"an iTerm2 AutoLaunch script", attempt{"create", filepath.Join(iterm, "Scripts", "AutoLaunch", "a.py"), ""}, "EPERM"},
		{"iTerm2's folder moved aside", attempt{"rename", iterm, filepath.Join(appSupport, "old-iterm")}, "EPERM"},
		{"iTerm2's link in ~/.config", attempt{"symlink", at(".config", "iterm2", "AppSupport"), staged}, "EPERM"},
		// Ghostty's two places.
		{"Ghostty's config", attempt{"write", at(".config", "ghostty", "config"), ""}, "EPERM"},
		{"Ghostty's Mac config", attempt{"write", filepath.Join(ghostty, "config"), ""}, "EPERM"},
		{"Ghostty's folder moved aside", attempt{"rename", at(".config", "ghostty"), at(".config", "old-ghostty")}, "EPERM"},
		// WezTerm's and kitty's, missing here: not made, nor brought in whole.
		{"WezTerm's ~/.wezterm.lua", attempt{"create", at(".wezterm.lua"), ""}, "EPERM"},
		{"git's lock beside it", attempt{"create", at(".wezterm.lua.lock"), ""}, "EPERM"},
		{"WezTerm's folder", attempt{"mkdir", at(".config", "wezterm"), ""}, "EPERM"},
		{"kitty's folder, brought in whole", attempt{"rename", staged, at(".config", "kitty")}, "EPERM"},
		// The hired agents' own settings.
		{"Codex's settings", attempt{"write", at(".codex", "config.toml"), ""}, "EPERM"},
		{"a file of Codex's", attempt{"create", at(".codex", "notify.sh"), ""}, "EPERM"},
		{"Gemini CLI's settings", attempt{"write", at(".gemini", "settings.json"), ""}, "EPERM"},
		{"an OpenCode plugin", attempt{"create", at(".config", "opencode", "plugins.js"), ""}, "EPERM"},
		{"Grok's folder", attempt{"mkdir", at(".grok"), ""}, "EPERM"},
		{"Grok's folder, as a link", attempt{"symlink", at(".grok"), staged}, "EPERM"},
		{"~/.codex moved aside", attempt{"rename", at(".codex"), at("old-codex")}, "EPERM"},
		// The keychains (P4's F1): the login keychain changed, replaced or moved, and no keychain made there.
		{"the login keychain", attempt{"write", filepath.Join(keychains, "login.keychain-db"), ""}, "EPERM"},
		{"the login keychain, replaced", attempt{"rename", filepath.Join(d.root, "draft.txt"), filepath.Join(keychains, "login.keychain-db")}, "EPERM"},
		{"a keychain of its own there", attempt{"create", filepath.Join(keychains, "match.keychain-db"), ""}, "EPERM"},
		{"~/Library/Keychains moved aside", attempt{"rename", keychains, at("Library", "old-keychains")}, "EPERM"},
	})
	for file, text := range owners {
		if data, err := os.ReadFile(file); err != nil || string(data) != text {
			t.Errorf("%s changed: %q (%v)", file, data, err)
		}
	}
	for _, missing := range []string{at(".wezterm.lua"), at(".config", "wezterm"), at(".config", "kitty"), at(".grok"),
		filepath.Join(prefs, "kitty"), filepath.Join(keychains, "match.keychain-db")} {
		if _, err := os.Lstat(missing); err == nil {
			t.Errorf("%s was made", missing)
		}
	}
}

// cfprefsd (option A of the brief): `defaults write com.apple.Terminal …` goes through cfprefsd's user side, which
// writes the plist in the owner's real ~/Library/Preferences, whatever HOME says and past any Seatbelt file rule (seen
// on this Mac with a made-up domain: FXPLANTSTERM's report). So that service is off a desk's Mach list, and a named
// domain's write fails at the lookup, before any file is written. cfprefsd's system side, root's daemon, is off it too
// (FXPLANTSTERMR4): no desk needs it, and it writes for the desk as root.
//
// No named domain is ever written here (the 02:5x rule): the row proves the refusal at the lookup itself, with a probe
// that looks the service up and sends it nothing. It checks the lists first, then that the probe sees a lookup that
// works (under a copy of the profile that allows the service), then that the desk's own lookup is refused: if either
// the lists or the lookup let the service in, the row stops there, before anything below it runs. Then, with path
// domains in the stand-in home only, that a domain's file is read by the desk itself, under its file rules, and that
// Terminal's plist there isn't made. The end-to-end proof, a named domain made up for the run, written from the desk and found
// nowhere, runs only on the Mac Studio, with SMOLCORP_STUDIO_PREFS_PROOF=1, and takes the domain out again if it was
// made. With both services off, a program's CFPreferences writes the plist itself, as the desk, at the real path, which
// the Policy's preferences row denies on the owner's home. This test's Policy is for a stand-in home, where the real
// path is open, so the Studio's step runs under a copy of the profile with that row's deny of the real path added, as a
// desk's Policy for the owner's home has it.
func TestOnAMacAHireCantSetAnAppsPreferencesThroughCfprefsd(t *testing.T) {
	_, python := sandboxExecAndPython(t)
	const agent, daemon = "com.apple.cfprefsd.agent", "com.apple.cfprefsd.daemon"
	for _, service := range []string{agent, daemon} {
		if slices.ContainsFunc(MachServices(), func(m MachService) bool { return m.Name == service }) {
			t.Fatalf("%s is on a desk's Mach list: a named domain's write from a desk lands where the desk can't write", service)
		}
		if !slices.ContainsFunc(NeverMachServices(), func(m MachService) bool { return m.Name == service && m.Why != "" }) {
			t.Errorf("%s isn't on the never list, with its reason", service)
		}
	}
	d := newEscapeDesk(t)
	lookup := func(profile, service string) string {
		r := d.work(t, profile, d.root, 20*time.Second, []string{pathOf(python)}, python, "-I", "-c", machLookupProbe, service)
		if r.status != 0 {
			t.Fatalf("the lookup probe: %+v", r)
		}
		return strings.TrimSpace(r.stdout)
	}
	for _, service := range []string{agent, daemon} {
		allowed := d.appended(t, "with-"+service+".sb", `(allow mach-lookup (global-name "`+service+`")) ; the probe's control`)
		if got := lookup(allowed, service); got != "ok" {
			t.Fatalf("control: under a profile that allows %s, the probe's lookup gives %q, not ok", service, got)
		}
		if got := lookup(d.profile, service); got == "ok" {
			t.Fatalf("%s is looked up from a desk: a named domain's write would land where the desk can't write (nothing was written)", service)
		} else {
			t.Logf("%s looked up from a desk: %s", service, got)
		}
	}

	const defaults = "/usr/bin/defaults"
	standIn := filepath.Join(d.root, "standin.plist")
	mustWrite(t, standIn, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>k</key><string>standin-value</string></dict></plist>
`)
	if got := strings.TrimSpace(outside(t, d.root, []string{"HOME=" + d.home, pathOf()}, defaults, "read", standIn, "k")); got != "standin-value" {
		t.Fatalf("control: the stand-in path domain read outside gives %q", got)
	}
	// With neither service, CFPreferences reads a domain's file itself, as the desk: a file the desk may read is read,
	// and one only root could read (mode 000) isn't, so no daemon of root's reads it for the desk.
	if r := d.inside(t, nil, defaults, "read", standIn, "k"); r.status != 0 || !strings.Contains(r.stdout, "standin-value") {
		t.Errorf("a path domain the desk may read, read inside: %+v", r)
	}
	if err := os.Chmod(standIn, 0); err != nil {
		t.Fatal(err)
	}
	r := d.inside(t, nil, defaults, "read", standIn, "k")
	if err := os.Chmod(standIn, 0o600); err != nil {
		t.Fatal(err)
	}
	if r.status == 0 || strings.Contains(r.stdout, "standin-value") {
		t.Fatalf("a path domain only root could read, read inside: %+v (cfprefsd read it for the desk)", r)
	}
	terminal := filepath.Join(d.home, "Library", "Preferences", "com.apple.Terminal")
	if r := d.inside(t, nil, defaults, "write", terminal, "Startup Window Settings", "-string", "FXPLANTSTERM-standin"); r.status == 0 {
		t.Errorf("Terminal's preferences in the stand-in home, as a path domain: %+v", r)
	}
	if _, err := os.Lstat(terminal + ".plist"); err == nil {
		t.Errorf("%s.plist was made", terminal)
	}

	// The Studio's step below holds its write with a deny added to a copy of the profile: such a deny holds a desk's
	// CFPreferences write, shown with a path domain in a stand-in folder the desk may otherwise write.
	elsewhere := filepath.Join(d.home, "elsewhere-prefs")
	if err := os.MkdirAll(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	held := d.appended(t, "with-elsewhere-held.sb", `(deny file-write* (subpath "`+elsewhere+`")) ; the stand-in folder, held`)
	for _, c := range []struct {
		profile, name string
		wrote         bool
	}{{d.profile, "a", true}, {held, "b", false}} {
		at := filepath.Join(elsewhere, c.name)
		r := d.work(t, c.profile, d.root, 20*time.Second, []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin"}, defaults, "write", at, "k", "-string", "standin")
		if _, err := os.Lstat(at + ".plist"); (err == nil) != c.wrote || (r.status == 0) != c.wrote {
			t.Fatalf("a path domain's write in a stand-in folder, under %s: %+v (the plist made: %v, want %v)", filepath.Base(c.profile), r, err == nil, c.wrote)
		}
	}

	if os.Getenv(studioPrefsProof) != "1" {
		t.Logf("a named domain's write from the desk, end to end: not run, it would go through the owner's real preferences "+
			"if the refusal above ever broke; only on the Mac Studio, with %s=1", studioPrefsProof)
		return
	}
	owner, err := user.Current()
	if err != nil || owner.HomeDir == "" {
		t.Fatalf("the owner's home folder: %v", err)
	}
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}
	domain := "com.smolcorp.plantsterm." + hex.EncodeToString(random)
	ownerPlist := filepath.Join(owner.HomeDir, "Library", "Preferences", domain+".plist")
	// Whether the write said it went through: cfprefsd may write the domain's file only after the cleanup looks for it.
	wrote := false
	t.Cleanup(func() {
		// Only if the write went through, which the row then says: the made-up domain is taken out again, through
		// cfprefsd as well as on disk, whether or not its file is there yet.
		if _, err := os.Lstat(ownerPlist); wrote || err == nil {
			runOutside(t, defaults, "delete", domain)
			os.Remove(ownerPlist)
		}
	})
	// The desk's preferences row on the owner's home, as its Policy for that home has it (above).
	asTheOwners := d.appended(t, "with-the-owners-preferences.sb", `(deny file-write* (subpath "`+
		filepath.Join(owner.HomeDir, "Library", "Preferences")+`")) ; the Policy's row on the owner's own home`)
	r = d.work(t, asTheOwners, d.root, 20*time.Second, []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin"}, defaults, "write", domain, "k",
		"-string", "FXPLANTSTERM-standin")
	wrote = r.status == 0
	if r.status == 0 || !strings.Contains(r.stderr, "Could not write domain "+domain) {
		t.Errorf("a named domain's write inside: %+v", r)
	}
	time.Sleep(200 * time.Millisecond) // cfprefsd writes a domain's file a moment after it answers
	if _, err := os.Lstat(ownerPlist); err == nil {
		t.Errorf("the named domain's write reached the owner's preferences: %s", ownerPlist)
	}
	t.Logf("a named domain's write inside: status %d, %q", r.status, strings.TrimSpace(r.stderr))
}

// Claude Code's sign-in in a desk, under the keychains' row (FXPLANTSTERMR1 N2): a desk's Claude Code never writes
// the login keychain, so the row costs it nothing. Claude Code (2.1.303, its secure storage and OAuth code) saves a
// credential only under a lock it makes first in the folder CLAUDE_SECURESTORAGE_CONFIG_DIR names, the account's
// (".storage-write", for every save), and refreshes a token only under another one there (".oauth_refresh.lock",
// taken before the refresh is even asked for): proper-lockfile's mkdir of each, which the account folders' row (NB6)
// refuses a desk. So the refresh stops at its lock ("lock_error"), the old refresh token is never spent, and no
// `security add-generic-password` runs; the desk reads the token the owner's own Claude Code keeps fresh. What it
// reads still works without cfprefsd's user side: `security` finds the keychains to search in the plist it reads
// itself, at $HOME/Library/Preferences/com.apple.security.plist (a control: that file unreadable, the list is gone).
// Nothing here opens a keychain or asks for one.
func TestOnAMacADesksClaudeSavesNoSignInAndStillFindsItsKeychains(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	account := filepath.Join(d.home, ".claude") // CLAUDE_SECURESTORAGE_CONFIG_DIR="" is ~/.claude's account
	d.check(t, []escapeRow{
		{"Claude Code's lock before a refresh", attempt{"mkdir", filepath.Join(account, ".oauth_refresh.lock"), ""}, "EPERM"},
		{"Claude Code's lock before a save", attempt{"mkdir", filepath.Join(account, ".storage-write.lock"), ""}, "EPERM"},
		{"Claude Code's refresh lock's owner record", attempt{"create", filepath.Join(account, ".oauth_refresh.lock.owner"), ""}, "EPERM"},
	})

	keychain := filepath.Join(d.home, "Library", "Keychains", "standin.keychain-db")
	mustWrite(t, keychain, "a stand-in, never opened")
	searchList := filepath.Join(d.home, "Library", "Preferences", "com.apple.security.plist")
	mustWrite(t, searchList, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>DLDBSearchList</key><array><dict>
<key>DbName</key><string>`+keychain+`</string>
<key>GUID</key><string>{87191ca2-0fc9-11d4-849a-000502b52122}</string>
<key>SubserviceType</key><integer>6</integer></dict></array></dict></plist>
`)
	list := func(profile string) ran {
		return d.work(t, profile, d.root, 20*time.Second, []string{pathOf()}, "/usr/bin/security", "list-keychains", "-d", "user")
	}
	if r := list(""); r.status != 0 || !strings.Contains(r.stdout, keychain) {
		t.Errorf("a desk's keychain search list, without cfprefsd's user side: %+v, want %s", r, keychain)
	}
	unreadable := d.appended(t, "no-search-list.sb", "(deny file-read* (literal "+sbString(searchList)+")) ; the control")
	if r := list(unreadable); strings.Contains(r.stdout, keychain) {
		t.Errorf("control: with its plist unreadable, the search list still names the stand-in: %+v", r)
	}
}

// studioPrefsProof is the variable only the Mac Studio sets, for the one row step that writes a named preferences
// domain (made up for the run): never on another Mac, where it would go through the owner's real preferences.
const studioPrefsProof = "SMOLCORP_STUDIO_PREFS_PROOF"

// machLookupProbe looks up the Mach service its argument names and prints "ok", or the kern_return_t it got. It holds
// the right it was given and sends nothing.
const machLookupProbe = `import ctypes, sys
libc = ctypes.CDLL(None)
port = ctypes.c_uint(0)
kr = libc.bootstrap_look_up(ctypes.c_uint.in_dll(libc, "bootstrap_port"), sys.argv[1].encode(), ctypes.byref(port))
print("ok" if kr == 0 else "kr%d" % kr)
`

// The keychains, with the Security server: a desk still reads an item `security` made, as the owner's programs do
// (§A4.4, TestOnAMacAHireReadsTheItemsSecurityMadeAsTheOwnerDoes), from a keychain in ~/Library/Keychains, but can't
// change it: a file keychain is written by the program that changes it, not by the Security server, so the
// keychains' row refuses it (RT126SWEEP2 P4's F1). It makes a keychain, so it runs only where SMOLCORP_TEST_KEYCHAIN=1:
// CI's Mac sandbox job, on a throwaway runner.
func TestOnAMacAHireReadsButCantChangeAKeychainInTheOwnersKeychains(t *testing.T) {
	sandboxExecOrSkip(t)
	if os.Getenv("SMOLCORP_TEST_KEYCHAIN") != "1" {
		t.Skip("SKIPPED, it makes a keychain: only on a throwaway runner, with SMOLCORP_TEST_KEYCHAIN=1 (CI's Mac sandbox job)")
	}
	d := newEscapeDesk(t)
	keychain := filepath.Join(d.home, "Library", "Keychains", "standin.keychain-db")
	if err := os.MkdirAll(filepath.Dir(keychain), 0o700); err != nil {
		t.Fatal(err)
	}
	security := "/usr/bin/security"
	env := []string{"HOME=" + d.home, pathOf()}
	outside(t, d.base, env, security, "create-keychain", "-p", "standin", keychain)
	t.Cleanup(func() { runOutside(t, security, "delete-keychain", keychain) })
	outside(t, d.base, env, security, "unlock-keychain", "-p", "standin", keychain)
	outside(t, d.base, env, security, "add-generic-password", "-a", "standin", "-s", "smolcorp-standin", "-w", escapeKey, "-T", security, keychain)
	read := func() ran {
		return d.work(t, "", d.root, 30*time.Second, []string{pathOf()}, security, "find-generic-password", "-a", "standin", "-s", "smolcorp-standin", "-w", keychain)
	}
	if r := read(); r.status != 0 || strings.TrimSpace(r.stdout) != escapeKey {
		t.Errorf("a desk reading an item `security` made, in ~/Library/Keychains: %+v (it still can)", r)
	}
	if r := d.work(t, "", d.root, 30*time.Second, []string{pathOf()}, security, "add-generic-password", "-U", "-a", "standin", "-s", "smolcorp-standin",
		"-w", "CHANGED", "-T", security, keychain); r.status == 0 {
		t.Errorf("a desk changing an item there: %+v", r)
	}
	if got := strings.TrimSpace(outside(t, d.base, env, security, "find-generic-password", "-a", "standin", "-s", "smolcorp-standin", "-w", keychain)); got != escapeKey {
		t.Errorf("the item, read outside afterwards: %q", got)
	}
}

// The hired agents' own folders in a desk (FXPLANTSTERMR2, after R1's review: B1–B3, N1): each is the owner's whole,
// its own state included. R1 opened what each agent writes to start (agentState), and each opening reached the owner's
// own sessions: Gemini's project memory, loaded into every one of them, and its old rg; a folder renamed in with Grok's
// grants or a checkpoints' git; Codex's databases, which feed its memories; a link at installation_id, which the
// owner's Codex rewrites and makes world-readable through. A stand-in agent does, in the stand-in home, the writes each
// agent's source makes before its first prompt (FXPLANTSTERMR1's report: Codex 0.169, Gemini CLI 0.68, grok-build,
// OpenCode 2.0; OpenCode 1.19.6's again in FXPLANTSTERMR3), the way it makes them, and reads the settings Smolcorp's
// hooks are in. So Codex, Gemini CLI, OpenCode and Grok hires are refused on a Mac in 1.26.0, before a window opens
// (the owner's rulings; desktop's notonamac.go).
const standInAgent = "agent"

func init() { moreStandIns[standInAgent] = agentStandIn }

// agentStep is one write a stand-in agent makes at its start. Op is one of: read; mkdirall (os.MkdirAll, as Rust's
// create_dir_all and Node's recursive mkdir: a folder that is there is fine); readwrite (read and write, made if missing:
// SQLite's files, Codex's installation_id); append (made if missing: a rollout, a chat, a log); mark (made only if
// missing: Gemini's .project_root); replace (To written, then renamed onto At: projects.json); lock (proper-lockfile:
// a folder made, its time set, then removed).
type agentStep struct{ Op, At, To string }

func agentStandIn(args []string) int {
	var steps []agentStep
	if len(args) != 1 || json.Unmarshal([]byte(args[0]), &steps) != nil {
		fmt.Fprintln(os.Stderr, "agent: one argument, the steps as JSON")
		return 2
	}
	said, status := make([]string, len(steps)), 0
	for i, s := range steps {
		if said[i] = errnoName(s.do()); said[i] != "ok" {
			status = 1
		}
	}
	if json.NewEncoder(os.Stdout).Encode(said) != nil {
		return 3
	}
	return status
}

func (s agentStep) do() error {
	file := func(flags int) error {
		f, err := os.OpenFile(s.At, flags, 0o600)
		if err == nil {
			f.Close()
		}
		return err
	}
	switch s.Op {
	case "read":
		_, err := os.ReadFile(s.At)
		return err
	case "mkdirall":
		return os.MkdirAll(s.At, 0o700)
	case "readwrite":
		return file(os.O_RDWR | os.O_CREATE)
	case "append":
		return file(os.O_WRONLY | os.O_APPEND | os.O_CREATE)
	case "mark":
		if err := file(os.O_WRONLY | os.O_CREATE | os.O_EXCL); !errors.Is(err, fs.ErrExist) {
			return err
		}
		return nil
	case "replace":
		if err := os.WriteFile(s.To, []byte("{}"), 0o600); err != nil {
			return err
		}
		return os.Rename(s.To, s.At)
	case "lock":
		if err := os.Mkdir(s.At, 0o700); err != nil {
			return err
		}
		if err := os.Chtimes(s.At, time.Now(), time.Now()); err != nil {
			return err
		}
		return os.Remove(s.At)
	}
	return fmt.Errorf("no such step %q", s.Op)
}

// A stand-in agent's start under the real profile: each agent's writes to its own state are refused (none is opened
// for them), and their reads still work. OpenCode's state, its data, cache and state folders, is the owner's too since
// FXPLANTSTERMR3: at start it makes its data, settings, state, log, bin and repos folders, with no error caught (its
// global.ts), then appends to its log and opens its database in its data folder.
func TestOnAMacAHiredAgentsOwnStateStaysTheOwners(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	at := func(parts ...string) string { return filepath.Join(append([]string{d.home}, parts...)...) }
	codex, gemini, grok, opencode := at(".codex"), at(".gemini"), at(".grok"), at(".config", "opencode")
	project := strings.ReplaceAll(d.root, "/", "%2F") // Grok's folder per project, as it spells it
	for file, text := range map[string]string{
		filepath.Join(codex, "hooks.json"): "{}", filepath.Join(codex, "installation_id"): "0f8e6c1a",
		filepath.Join(gemini, "settings.json"): "{}", filepath.Join(gemini, "projects.json"): "{}",
		filepath.Join(grok, "hooks", "smolcorp.json"): "{}", filepath.Join(opencode, "plugins", "smolcorp.js"): "// the deck's",
		at(".local", "share", "opencode", "log", "opencode.log"): "the owner's", at(".local", "share", "opencode", "opencode.db"): "the owner's",
	} {
		mustWrite(t, file, text)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Each step and what it must give: a folder that is there, made again, is fine (Rust's create_dir_all, Node's
	// recursive mkdir); every write to the agent's own state is refused.
	type step struct {
		agentStep
		want string
	}
	starts := []struct {
		agent string
		steps []step
	}{
		{"Codex", []step{
			{agentStep{"read", filepath.Join(codex, "hooks.json"), ""}, "ok"}, {agentStep{"mkdirall", codex, ""}, "ok"},
			{agentStep{"readwrite", filepath.Join(codex, "state_5.sqlite"), ""}, "EPERM"},
			{agentStep{"readwrite", filepath.Join(codex, "installation_id"), ""}, "EPERM"},
			{agentStep{"mkdirall", filepath.Join(codex, "sessions", "2026", "11", "18"), ""}, "EPERM"},
		}},
		{"Gemini CLI", []step{
			{agentStep{"read", filepath.Join(gemini, "settings.json"), ""}, "ok"}, {agentStep{"mkdirall", gemini, ""}, "ok"},
			{agentStep{"lock", filepath.Join(gemini, "projects.json.lock"), ""}, "EPERM"},
			{agentStep{"replace", filepath.Join(gemini, "projects.json"), filepath.Join(gemini, "projects.json.0f8e6c1a.tmp")}, "EPERM"},
			{agentStep{"mkdirall", filepath.Join(gemini, "tmp", "h1", "chats"), ""}, "EPERM"},
			{agentStep{"mkdirall", filepath.Join(gemini, "history", "h1"), ""}, "EPERM"},
		}},
		{"Grok", []step{
			{agentStep{"read", filepath.Join(grok, "hooks", "smolcorp.json"), ""}, "ok"},
			{agentStep{"mkdirall", filepath.Join(grok, "sessions", project, "s1"), ""}, "EPERM"},
		}},
		{"OpenCode", []step{
			{agentStep{"read", filepath.Join(opencode, "plugins", "smolcorp.js"), ""}, "ok"}, {agentStep{"mkdirall", opencode, ""}, "ok"},
			{agentStep{"mkdirall", at(".local", "share", "opencode", "log"), ""}, "ok"}, // the owner's, there
			{agentStep{"mkdirall", at(".local", "state", "opencode"), ""}, "EPERM"},
			{agentStep{"mkdirall", at(".cache", "opencode", "bin"), ""}, "EPERM"},
			{agentStep{"mkdirall", at(".local", "share", "opencode", "repos"), ""}, "EPERM"},
			{agentStep{"append", at(".local", "share", "opencode", "log", "opencode.log"), ""}, "EPERM"},
			{agentStep{"readwrite", at(".local", "share", "opencode", "opencode.db"), ""}, "EPERM"},
		}},
	}
	for _, start := range starts {
		steps := make([]agentStep, len(start.steps))
		for i, s := range start.steps {
			steps[i] = s.agentStep
		}
		data, err := json.Marshal(steps)
		if err != nil {
			t.Fatal(err)
		}
		r := d.inside(t, nil, self, standInAgent, string(data))
		var said []string
		if json.Unmarshal([]byte(r.stdout), &said) != nil || len(said) != len(steps) {
			t.Fatalf("%s's stand-in didn't run: %+v", start.agent, r)
		}
		for i, got := range said {
			if got != start.steps[i].want {
				t.Errorf("%s in a desk: %s %s gave %s, not %s", start.agent, steps[i].Op, steps[i].At, got, start.steps[i].want)
			}
		}
	}
}

// R1's review's shapes (B1, B2, N1), and every plant R1's row named, under the real profile: refused, and nothing of
// them is there afterwards.
func TestOnAMacAHireCantPlantInAHiredAgentsOwnState(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	at := func(parts ...string) string { return filepath.Join(append([]string{d.home}, parts...)...) }
	codex, gemini, grok, opencode := at(".codex"), at(".gemini"), at(".grok"), at(".config", "opencode")
	project := strings.ReplaceAll(d.root, "/", "%2F")
	for file, text := range map[string]string{
		filepath.Join(codex, "config.toml"):                                     "# the owner's",
		filepath.Join(codex, "state_5.sqlite"):                                  "the owner's",
		filepath.Join(codex, "sessions", "2026", "11", "17", "rollout-1.jsonl"): "the owner's",
		filepath.Join(gemini, "settings.json"):                                  "{}",
		filepath.Join(gemini, "projects.json"):                                  "{}",
		filepath.Join(gemini, "tmp", "app", ".project_root"):                    "the owner's",
		filepath.Join(gemini, "history", "app", ".project_root"):                "the owner's",
		filepath.Join(grok, "sessions", "%2Fowner", "permission.toml"):          "# the owner's",
		filepath.Join(grok, "sessions", "%2Fowner", "s1", "updates.jsonl"):      "the owner's",
		filepath.Join(opencode, "plugins", "smolcorp.js"):                       "// the deck's",
		// Made by the desk in its worktree, to bring in whole: a project's grants, a session's grants, a checkpoints' git.
		filepath.Join(d.root, "stA", "permission.toml"):     "a desk's",
		filepath.Join(d.root, "stB", "sandbox_grants.toml"): "a desk's",
		filepath.Join(d.root, "stC", ".git", "config"):      "[core]\n\tfsmonitor = stand-in\n",
		filepath.Join(d.root, "stC", ".project_root"):       "a desk's",
		filepath.Join(d.root, "staged", "permission.toml"):  "a desk's",
	} {
		mustWrite(t, file, text)
	}
	staged := filepath.Join(d.root, "staged")
	d.check(t, []escapeRow{
		// Controls: the desk reads the owner's settings, and writes its own worktree.
		{"control: Gemini's settings read", attempt{"read", filepath.Join(gemini, "settings.json"), ""}, "ok"},
		{"control: its worktree writes", attempt{"create", filepath.Join(d.root, "agents-control"), ""}, "ok"},
		// B1: Gemini's project memory, loaded into every owner session, and the rg Gemini ≤ 0.39 runs.
		{"B1: Gemini's tmp/bin", attempt{"mkdir", filepath.Join(gemini, "tmp", "bin"), ""}, "EPERM"},
		{"B1: a project's memory folder", attempt{"mkdir", filepath.Join(gemini, "tmp", "app", "memory"), ""}, "EPERM"},
		{"B1: a project's chats", attempt{"mkdir", filepath.Join(gemini, "tmp", "app", "chats"), ""}, "EPERM"},
		// B2: projects.json, and whole folders renamed in, existing projects too.
		{"B2: Gemini's projects.json", attempt{"write", filepath.Join(gemini, "projects.json"), ""}, "EPERM"},
		{"B2: Gemini's project marks", attempt{"write", filepath.Join(gemini, "tmp", "app", ".project_root"), ""}, "EPERM"},
		{"B2: a project's grants, brought in whole", attempt{"rename", filepath.Join(d.root, "stA"), filepath.Join(grok, "sessions", "%2Fnewproj")}, "EPERM"},
		{"B2: a session's grants, into an existing project", attempt{"rename", filepath.Join(d.root, "stB"), filepath.Join(grok, "sessions", "%2Fowner", "s9")}, "EPERM"},
		{"B2: a checkpoints' git, brought in whole", attempt{"rename", filepath.Join(d.root, "stC"), filepath.Join(gemini, "history", "newid")}, "EPERM"},
		// N1: Codex's databases, which feed its memories, and its rollouts, read back on resume.
		{"N1: Codex's state database", attempt{"write", filepath.Join(codex, "state_5.sqlite"), ""}, "EPERM"},
		{"N1: a Codex database made", attempt{"create", filepath.Join(codex, "memories_1.sqlite"), ""}, "EPERM"},
		{"N1: an owner's rollout", attempt{"write", filepath.Join(codex, "sessions", "2026", "11", "17", "rollout-1.jsonl"), ""}, "EPERM"},
		{"N1: Grok's owner session", attempt{"write", filepath.Join(grok, "sessions", "%2Fowner", "s1", "updates.jsonl"), ""}, "EPERM"},
		// R1's row's plants, still refused.
		{"Codex's settings", attempt{"write", filepath.Join(codex, "config.toml"), ""}, "EPERM"},
		{"Codex's hooks", attempt{"create", filepath.Join(codex, "hooks.json"), ""}, "EPERM"},
		{"Codex's rules", attempt{"mkdir", filepath.Join(codex, "rules"), ""}, "EPERM"},
		{"Codex's .env", attempt{"create", filepath.Join(codex, ".env"), ""}, "EPERM"},
		{"Codex's shell snapshots", attempt{"mkdir", filepath.Join(codex, "shell_snapshots"), ""}, "EPERM"},
		{"Codex's PATH folder", attempt{"mkdir", filepath.Join(codex, "tmp"), ""}, "EPERM"},
		{"Codex's skills", attempt{"mkdir", filepath.Join(codex, "skills"), ""}, "EPERM"},
		{"Codex's sessions moved aside", attempt{"rename", filepath.Join(codex, "sessions"), at("old-sessions")}, "EPERM"},
		{"Gemini's settings", attempt{"write", filepath.Join(gemini, "settings.json"), ""}, "EPERM"},
		{"Gemini's trusted folders", attempt{"create", filepath.Join(gemini, "trustedFolders.json"), ""}, "EPERM"},
		{"Gemini's policies", attempt{"mkdir", filepath.Join(gemini, "policies"), ""}, "EPERM"},
		{"Gemini's checkpoints' git", attempt{"mkdir", filepath.Join(gemini, "history", "app", ".git"), ""}, "EPERM"},
		{"Gemini's history moved aside", attempt{"rename", filepath.Join(gemini, "history"), at("old-history")}, "EPERM"},
		{"Grok's settings", attempt{"create", filepath.Join(grok, "config.toml"), ""}, "EPERM"},
		{"Grok's hooks", attempt{"mkdir", filepath.Join(grok, "hooks"), ""}, "EPERM"},
		{"Grok's sign-in's lock", attempt{"create", filepath.Join(grok, "auth.json.lock"), ""}, "EPERM"},
		{"Grok's grants", attempt{"create", filepath.Join(grok, "sandbox_grants.toml"), ""}, "EPERM"},
		{"a project's grants", attempt{"write", filepath.Join(grok, "sessions", "%2Fowner", "permission.toml"), ""}, "EPERM"},
		{"a project's grants, made", attempt{"mkdir", filepath.Join(grok, "sessions", project), ""}, "EPERM"},
		{"a project's grants, by a rename", attempt{"rename", filepath.Join(staged, "permission.toml"), filepath.Join(grok, "sessions", "%2Fowner", "permission_2.toml")}, "EPERM"},
		{"Grok's sessions moved aside", attempt{"rename", filepath.Join(grok, "sessions"), at("old-grok-sessions")}, "EPERM"},
		{"an OpenCode plugin", attempt{"create", filepath.Join(opencode, "plugins", "desk.js"), ""}, "EPERM"},
	})
	for _, planted := range []string{filepath.Join(gemini, "tmp", "bin"), filepath.Join(gemini, "tmp", "app", "memory"),
		filepath.Join(grok, "sessions", "%2Fnewproj"), filepath.Join(grok, "sessions", "%2Fowner", "s9"), filepath.Join(gemini, "history", "newid"),
		filepath.Join(codex, "memories_1.sqlite"), filepath.Join(grok, "sessions", "%2Fowner", "permission_2.toml")} {
		if _, err := os.Lstat(planted); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s is there after the desk ran (%v)", planted, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(grok, "sessions", "%2Fowner", "permission.toml")); err != nil || string(data) != "# the owner's" {
		t.Errorf("the owner's grants changed: %q (%v)", data, err)
	}
}

// B3's class (FXPLANTSTERMR2): no link made in any hired agent's folder, symbolic or hard, nor one renamed in. The
// owner's Codex opens installation_id at every start following links, sets its mode to 0644 and rewrites it unless it
// holds a UUID, so a link there would have it change any file the owner can write. The links row holds every link by
// itself: a mutant without the folders' write deny still refuses each one, and only lets installation_id be removed,
// which is the write deny's to refuse (FXPLANTSTERMR2's report).
func TestOnAMacNoLinkIsMadeInAHiredAgentsFolder(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	target := filepath.Join(d.home, ".zshrc") // the owner's file the desk can't write itself
	mustWrite(t, target, "# the owner's")
	mustWrite(t, filepath.Join(d.root, "mine"), "a desk's")
	rows := []escapeRow{
		{"control: the owner's file isn't the desk's", attempt{"write", target, ""}, "EPERM"},
		{"control: a link in its worktree", attempt{"symlink", filepath.Join(d.root, "link-control"), target}, "ok"},
	}
	// In each folder, names FXPLANTSTERMR1 had opened to a desk where it had any (Codex's databases, Gemini's project
	// list and its copies, Grok's sessions), and the file the owner's agent rewrites through a link (Codex's
	// installation_id), or a stand-in.
	type names struct{ symlink, hard, replaced string }
	in := map[string]names{
		".codex":           {"made.sqlite", "linked.sqlite-wal", "installation_id"},
		".gemini":          {"projects.json.made", "projects.json.linked", "projects.json"},
		".grok":            {"sessions/made", "sessions/linked", "sessions/standin"},
		".config/opencode": {"made", "linked", "standin"},
		// FXPLANTSTERMR3: Gemini CLI's folder in its own sandbox, OpenCode's data, cache and state, the owner's skills.
		".cache/.gemini":        {"made", "linked", "standin"},
		".local/share/opencode": {"made", "linked", "opencode.db"},
		".cache/opencode":       {"bin/made", "bin/linked", "bin/rg"},
		".local/state/opencode": {"made", "linked", "kv.json"},
		".opencode":             {"plugins/made", "plugins/linked", "plugins/p.js"},
		".agents":               {"skills/made", "skills/linked", "skills/s/SKILL.md"},
	}
	for i, folder := range agentFolders {
		place := filepath.Join(d.home, filepath.FromSlash(folder))
		n := in[folder]
		mustWrite(t, filepath.Join(place, filepath.FromSlash(n.replaced)), "0f8e6c1a")
		ready := filepath.Join(d.root, fmt.Sprintf("ready-%d", i))
		rows = append(rows,
			escapeRow{folder + ": a symlink", attempt{"symlink", filepath.Join(place, filepath.FromSlash(n.symlink)), target}, "EPERM"},
			escapeRow{folder + ": a hard link", attempt{"link", filepath.Join(d.root, "mine"), filepath.Join(place, filepath.FromSlash(n.hard))}, "EPERM"},
			escapeRow{folder + ": a link made in the worktree", attempt{"symlink", ready, target}, "ok"},
			escapeRow{folder + ": that link renamed onto " + n.replaced, attempt{"rename", ready, filepath.Join(place, filepath.FromSlash(n.replaced))}, "EPERM"},
		)
	}
	// R1's review's B3 as it was found: installation_id removed, to be made a link.
	rows = append(rows, escapeRow{"B3: Codex's installation_id removed", attempt{"remove", filepath.Join(d.home, ".codex", "installation_id"), ""}, "EPERM"})
	d.check(t, rows)
	for _, folder := range agentFolders {
		at := filepath.Join(d.home, filepath.FromSlash(folder), filepath.FromSlash(in[folder].replaced))
		if info, err := os.Lstat(at); err != nil || !info.Mode().IsRegular() {
			t.Errorf("%s isn't the owner's file any more (%v)", at, err)
		}
		for _, made := range []string{in[folder].symlink, in[folder].hard} {
			if _, err := os.Lstat(filepath.Join(d.home, filepath.FromSlash(folder), filepath.FromSlash(made))); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("%s/%s is there after the desk ran (%v)", folder, made, err)
			}
		}
	}
}
