//go:build !windows

package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// SA-X9: a desk's sign-in as its own Claude folder sees it, S2's folder, and Take over and Hand back of a desk with its
// own Claude folder. Every claude here is a stand-in that prints what `claude auth status --json` or `claude auth
// login` prints and notes what it was given: no real claude runs, and no credential is read. Keys are fake.

// signInStandIn makes the claude deskSignIn asks a stand-in that answers said, and notes in the file it gives back its
// arguments, its two folders' variables and the .claude.json in the folder it was given, one line each.
func signInStandIn(t *testing.T, said string) string {
	t.Helper()
	dir := t.TempDir()
	got := filepath.Join(dir, "got")
	program := fakeCLI(t, dir, "claude", `{ echo "args: $*"; echo "config: ${CLAUDE_CONFIG_DIR-unset}"
  echo "secure: ${CLAUDE_SECURESTORAGE_CONFIG_DIR-unset}"; tr -d '\n' < "$CLAUDE_CONFIG_DIR/.claude.json"; echo; } >> `+got+`
echo '`+said+`'`)
	saved := deskSignInFind
	t.Cleanup(func() { deskSignInFind = saved })
	deskSignInFind = func() string { return program }
	return got
}

// askedLines are the lines a stand-in noted, "" when it never ran.
func askedLines(t *testing.T, got string) []string {
	t.Helper()
	text, err := os.ReadFile(got)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
}

// A desk Claude Code says isn't signed in isn't started, and nothing of it is left; it was asked with its own folder,
// seeded with its account's sign-in profile and nothing secret, and its account's sign-in. One that's signed in
// starts, and so does one Claude Code can't say about.
func TestADeskThatIsntSignedInIsRefusedAtItsStartWithNothingLeft(t *testing.T) {
	r := newEngineRig(t)
	deskAccount(t, "")
	got := signInStandIn(t, `{"loggedIn": false, "authMethod": "none"}`)
	_, err := runManagedHire(r.request("janitor"), r.o)
	var refused *hireRefusal
	if !errors.As(err, &refused) || refused.Status != http.StatusConflict || refused.Message != signInYourHires {
		t.Fatalf("err = %v, want %q", err, signInYourHires)
	}
	if r.tmux.ran("new-session") || len(registered(t)) != 0 {
		t.Errorf("a desk refused for its sign-in started or stayed: tmux %q, registry %+v", r.tmux.asked, registered(t))
	}
	if left, _ := os.ReadDir(filepath.Join(supportDir, "hires")); len(left) != 0 {
		t.Errorf("the hires folder holds %v", left)
	}
	lines := askedLines(t, got)
	if len(lines) != 4 || lines[0] != "args: auth status --json" || lines[2] != "secure: " ||
		!strings.HasPrefix(lines[1], "config: "+filepath.Join(supportDir, "hires")+"/") || !strings.HasSuffix(lines[1], "/"+deskClaudeName) {
		t.Fatalf("asked with %q", lines)
	}
	if !strings.Contains(lines[3], "acct-x8") || strings.Contains(lines[3], "sk-fake-x8-primary") || strings.Contains(lines[3], "fake-ticked-key") {
		t.Errorf("the desk's .claude.json, as it was asked: %s", lines[3])
	}

	for _, said := range []string{`{"loggedIn": true, "authMethod": "claude.ai"}`, "nothing it can read"} {
		got = signInStandIn(t, said)
		hire, err := runManagedHire(r.request("janitor"), r.o)
		if err != nil {
			t.Fatalf("%s: %v", said, err)
		}
		if lines := askedLines(t, got); len(lines) != 4 || lines[1] != "config: "+deskClaudeFolder(supportDir, hire.ID) {
			t.Errorf("%s: asked with %q", said, lines)
		}
	}
}

// A desk on a named account is asked with that account's sign-in; an API-key hire isn't asked at all.
func TestADesksSignInIsItsAccountsAndAnAPIKeyHireIsntAsked(t *testing.T) {
	r := newEngineRig(t)
	macDeskLinks(t)
	work, _ := deskAccount(t, filepath.Join(r.home, "accounts", "work"))
	got := signInStandIn(t, `{"loggedIn": false}`)
	request := r.request("janitor")
	request.ConfigDir, request.DefaultAccount = work, false
	if _, err := runManagedHire(request, r.o); err == nil || err.Error() != signInYourHires {
		t.Fatalf("err = %v", err)
	}
	if lines := askedLines(t, got); len(lines) != 4 || lines[2] != "secure: "+work {
		t.Errorf("asked with %q", lines)
	}
	_ = os.Remove(got)
	if why := deskSignInRefusal(t.TempDir(), work, true); why != "" || askedLines(t, got) != nil {
		t.Errorf("an API-key hire: %q, asked %q", why, askedLines(t, got))
	}
	if why := deskSignInRefusal(t.TempDir(), work, false); why != signInYourHires {
		t.Errorf("a subscription hire: %q", why)
	}
}

// Setup's question: asked in a folder made for it as a desk's is, and taken away after; nothing asked for an account
// with no .claude.json to seed it from.
func TestHiresSignInIsAskedInAFolderOfTheirKindAndLeavesNothing(t *testing.T) {
	isolate(t)
	homeDir = t.TempDir()
	work, _ := deskAccount(t, filepath.Join(homeDir, ".claude-work"))
	got := signInStandIn(t, `{"loggedIn": true}`)
	if state := hiresSignIn(work); state != signedIn {
		t.Fatalf("state %v", state)
	}
	lines := askedLines(t, got)
	if len(lines) != 4 || lines[2] != "secure: "+work || !strings.Contains(lines[3], "acct-x8") || strings.Contains(lines[3], "sk-fake-x8-primary") {
		t.Fatalf("asked with %q", lines)
	}
	// In the support folder, which every desk's sandbox denies, never the user's temp folder (FXSWEEP3P3 F-4).
	folder := strings.TrimPrefix(lines[1], "config: ")
	if folder == work || filepath.Dir(folder) != supportDir || !strings.HasPrefix(filepath.Base(folder), "signin-") {
		t.Errorf("asked in %s, not in the support folder %s", folder, supportDir)
	}
	if _, err := os.Lstat(folder); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the folder it was asked in is still there: %v", err)
	}
	signInStandIn(t, `{"loggedIn": false}`)
	if state := hiresSignIn(work); state != signedOut {
		t.Errorf("signed out: %v", state)
	}
	got = signInStandIn(t, `{"loggedIn": true}`)
	if state := hiresSignIn(filepath.Join(homeDir, ".claude-none")); state != signInUnknown || askedLines(t, got) != nil {
		t.Errorf("an account with no .claude.json: %v, asked %q", state, askedLines(t, got))
	}
	// The GM's preflight asks the same way, with the program it found: in a folder of a desk's kind, never the account's.
	state := claudeSignIn(deskSignInFind(), work)
	if lines := askedLines(t, got); state != signedIn || len(lines) != 4 || lines[2] != "secure: "+work ||
		filepath.Dir(strings.TrimPrefix(lines[1], "config: ")) != supportDir {
		t.Errorf("the preflight's: %v, asked with %q", state, lines)
	}
}

// RT126SAX9 S1: Setup's question and the preflight's are asked with the account's settings.json beside its .claude.json,
// read-only, as a desk's own folder has it (fill): an env or an apiKeyHelper there is how some owners sign in. The
// stand-in is signed in only through such a settings.json, one it can't write.
func TestHiresSignInIsAskedWithTheAccountsSettingsAsADesksFolderHasThem(t *testing.T) {
	isolate(t)
	homeDir = t.TempDir()
	work, _ := deskAccount(t, filepath.Join(homeDir, ".claude-work"))
	deskAccount(t, "")
	for _, account := range []string{work, filepath.Join(homeDir, ".claude")} {
		if err := os.WriteFile(filepath.Join(account, "settings.json"), []byte(`{"env":{"ANTHROPIC_API_KEY":"sk-fake-s1"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	program := fakeCLI(t, t.TempDir(), "claude", `s="$CLAUDE_CONFIG_DIR/settings.json"
if [ -f "$s" ] && [ ! -L "$s" ] && [ ! -w "$s" ] && grep -q sk-fake-s1 "$s"; then echo '{"loggedIn": true}'; else echo '{"loggedIn": false}'; fi`)
	saved := deskSignInFind
	t.Cleanup(func() { deskSignInFind = saved })
	deskSignInFind = func() string { return program }
	for _, account := range []string{work, ""} {
		if state := hiresSignIn(account); state != signedIn {
			t.Errorf("%q: Setup's: %v", account, state)
		}
		if state := claudeSignIn(program, account); state != signedIn {
			t.Errorf("%q: the preflight's: %v", account, state)
		}
	}
	// A desk's own folder has it the same way, and the start's own question says the same.
	desk := filepath.Join(t.TempDir(), deskClaudeName)
	if err := makeDeskClaude(desk, deskClaude{Account: work, Trusted: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if state := deskSignIn(desk, work); state != signedIn {
		t.Errorf("the desk's own folder: %v", state)
	}
	// An account with no settings.json: asked all the same, with none.
	if err := os.Remove(filepath.Join(work, "settings.json")); err != nil {
		t.Fatal(err)
	}
	if state := hiresSignIn(work); state != signedOut {
		t.Errorf("no settings.json: %v", state)
	}
}

// Setup's status names every account's hires' sign-in, kept as the agents' rows are and asked again on a refresh.
func TestSetupSaysWhetherEachAccountsHiresAreSignedIn(t *testing.T) {
	d := newTestDeck(t)
	work, _ := deskAccount(t, filepath.Join(homeDir, ".claude-work"))
	d.config.ClaudeProfiles = []claudeProfile{{Name: "work", Dir: work}}
	d.signInSeen.probe = func() []agentSignIn { return []agentSignIn{} }
	now := time.Unix(1_000_000, 0)
	d.hiresSeen.now = func() time.Time { return now }
	var mu sync.Mutex
	var asked []string
	d.hiresSeen.signIn = func(account string) signIn {
		mu.Lock()
		defer mu.Unlock()
		asked = append(asked, account)
		if account == work {
			return signedOut
		}
		return signedIn
	}
	status := func(target string) []hiresSignInRow {
		t.Helper()
		code, body := page(t, d, "GET", target, "")
		var answer struct{ Hires []hiresSignInRow }
		if code != http.StatusOK || json.Unmarshal([]byte(body), &answer) != nil {
			t.Fatalf("%d %s", code, body)
		}
		return answer.Hires
	}
	want := []hiresSignInRow{{"default", accountSignedIn}, {"work", accountSignedOut}}
	if rows := status("/setup/accounts/status"); !slices.Equal(rows, want) {
		t.Errorf("rows %+v", rows)
	}
	slices.Sort(asked)
	if !slices.Equal(asked, []string{"", work}) {
		t.Errorf("asked %q: ~/.claude's as \"\", work's by its folder", asked)
	}
	status("/setup/accounts/status?refresh=1")
	if len(asked) != 2 {
		t.Errorf("asked again within %v: %q", signInLeastApart, asked)
	}
	now = now.Add(signInLeastApart)
	status("/setup/accounts/status?refresh=1")
	if len(asked) != 4 {
		t.Errorf("a refresh didn't ask again: %q", asked)
	}
	// Another list of accounts is asked at once.
	d.config.ClaudeProfiles = nil
	if rows := status("/setup/accounts/status"); !slices.Equal(rows, want[:1]) || len(asked) != 5 {
		t.Errorf("the default alone: %+v, asked %q", rows, asked)
	}
}

// S2's runner signs in the folder asked for, only a confirmed account's or the hires' own, whatever Smolcorp's own
// environment names; any other folder is refused before anything runs.
func TestASignInIsForAConfirmedAccountsFolderAlone(t *testing.T) {
	dir := t.TempDir()
	got := filepath.Join(dir, "got")
	loginPlace(t, map[string]string{"claude": `echo "config: ${CLAUDE_CONFIG_DIR-unset} secure: ${CLAUDE_SECURESTORAGE_CONFIG_DIR-unset} args: $*" > ` + got + `
echo 'Login successful.'`, "codex": "exit 0"})
	d := loginDeck(t)
	work, _ := deskAccount(t, filepath.Join(homeDir, ".claude-work"))
	other, _ := deskAccount(t, filepath.Join(homeDir, ".claude-other"))
	hires := filepath.Join(dir, "hires-claude")
	saved := hiresSignInFolders
	t.Cleanup(func() { hiresSignInFolders = saved })
	hiresSignInFolders = func() []string { return []string{hires} }
	d.config.ClaudeProfiles = []claudeProfile{{Name: "work", Dir: work}}
	t.Setenv("CLAUDE_CONFIG_DIR", other)
	t.Setenv(secureStorageEnvironment, other)
	for _, c := range []struct{ agent, folder string }{
		{"claude", other}, {"claude", "~/.claude-other"}, {"claude", "/etc"}, {"claude", "relative"}, {"claude", supportDir},
		{"claude", work + "/.."}, {"claude", "//server/share"}, {"codex", work},
	} {
		body, _ := json.Marshal(map[string]string{"agent": c.agent, "way": "", "folder": c.folder})
		if code, said := page(t, d, "POST", "/setup/accounts/login", string(body)); code != http.StatusBadRequest ||
			said != "Smolcorp signs in only the Claude folders of the accounts you confirmed in Claude accounts." {
			t.Errorf("%s %s: %d %s", c.agent, c.folder, code, said)
		}
	}
	if d.logins.current("") != nil || exists(got) {
		t.Fatal("a refused folder started a sign-in")
	}
	for _, c := range []struct{ folder, config string }{
		{work, work}, {"~/.claude-work", work}, {hires, hires}, {"", "unset"}, {filepath.Join(homeDir, ".claude"), "unset"}, {"~/.claude", "unset"},
	} {
		body, _ := json.Marshal(map[string]string{"agent": "claude", "way": "claudeai", "folder": c.folder})
		code, said := page(t, d, "POST", "/setup/accounts/login", string(body))
		var started struct{ ID string }
		if code != http.StatusOK || json.Unmarshal([]byte(said), &started) != nil {
			t.Fatalf("%s: %d %s", c.folder, code, said)
		}
		if view := waitForLogin(t, d, started.ID, "ended", ended); view.State != loginSucceeded {
			t.Errorf("%s: %+v", c.folder, view)
		}
		if text, _ := os.ReadFile(got); string(text) != "config: "+c.config+" secure: unset args: auth login --claudeai\n" {
			t.Errorf("%s: the login was given %q", c.folder, text)
		}
	}
}

func TestALoginsFolderIsItsOwnAlone(t *testing.T) {
	base := []string{"PATH=/bin", "claude_config_dir=/x", "CLAUDE_SECURESTORAGE_CONFIG_DIR=/y", "HOME=/h"}
	if env := loginEnvironment(base, "/w"); slices.ContainsFunc(env, func(v string) bool {
		return strings.Contains(v, "/x") || strings.Contains(v, "/y")
	}) || !slices.Contains(env, "CLAUDE_CONFIG_DIR=/w") || !slices.Contains(env, "HOME=/h") {
		t.Errorf("%q", env)
	}
	if env := loginEnvironment(base, ""); slices.ContainsFunc(env, func(v string) bool { return strings.Contains(strings.ToUpper(v), "CONFIG_DIR") }) {
		t.Errorf("~/.claude's: %q", env)
	}
}

// MARK: Take over and Hand back of a desk with its own Claude folder

// takenOverFromOwnClaude puts the rig's session in its hire's own Claude folder (SA-X8), as Claude Code lays it out there.
func takenOverFromOwnClaude(t *testing.T, k *t3TakeOverRig) string {
	t.Helper()
	own := deskClaudeFolder(supportDir, k.hire.ID)
	transcript := filepath.Join(own, "projects", "-repo-hires-ada-abcdef", t3TakeOverSession+".jsonl")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, []byte("{\"desk\":1}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	k.session.Transcript = transcript
	return transcript
}

// Take over finds a desk's transcript in its own folder, on the default account only, and never another desk's.
func TestT3TakeOverFindsADesksTranscriptInItsOwnFolder(t *testing.T) {
	k := newT3TakeOverRig(t)
	transcript := takenOverFromOwnClaude(t, k)
	home := k.home
	if why := t3TakeOverRefusal(k.hire, k.session, home, false); why != "" {
		t.Errorf("its own folder: %q", why)
	}
	other := "Ada works on another Claude account, and T3 Code's agent works on your default one, so it can't take this session over."
	for _, account := range []string{"", claudeDefaultName} {
		h := k.hire
		h.Account = account
		if why := t3TakeOverRefusal(h, k.session, home, false); why != "" {
			t.Errorf("account %q: %q", account, why)
		}
	}
	h := k.hire
	h.Account = "work"
	if why := t3TakeOverRefusal(h, k.session, home, false); why != other {
		t.Errorf("on work: %q", why)
	}
	// Another desk's folder, or a folder of this desk's that isn't its own Claude folder.
	h = k.hire
	h.ID = "fedcba654321"
	if err := os.MkdirAll(deskClaudeFolder(supportDir, h.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	h.TmuxSocket = filepath.Join(supportDir, hireSocketPrefix+h.ID)
	if why := t3TakeOverRefusal(h, k.session, home, false); why != other {
		t.Errorf("another desk's: %q", why)
	}
	elsewhere := filepath.Join(hireFolder(supportDir, k.hire.ID), "projects", "-x", t3TakeOverSession+".jsonl")
	if why := t3TakeOverRefusal(k.hire, t3Session{ID: t3TakeOverSession, Transcript: elsewhere}, home, false); why != other {
		t.Errorf("its hire folder: %q", why)
	}
	// Gone at its fire: no folder of its own, nothing found there.
	if err := os.RemoveAll(deskClaudeFolder(supportDir, k.hire.ID)); err != nil {
		t.Fatal(err)
	}
	if why := t3TakeOverRefusal(k.hire, t3Session{ID: t3TakeOverSession, Transcript: transcript}, home, false); why != other {
		t.Errorf("a fired desk's: %q", why)
	}
}

// Take over copies the desk's transcript where T3 Code's import looks, and keeps that copy on record; Hand back
// brings what T3 Code wrote back to the desk's own folder, and takes the copy away.
func TestT3TakeOverAndHandBackCarryADesksSessionBetweenItsFolderAndYours(t *testing.T) {
	k := newT3TakeOverRig(t)
	transcript := takenOverFromOwnClaude(t, k)
	copied := filepath.Join(k.home, ".claude", "projects", "-repo-hires-ada-abcdef", t3TakeOverSession+".jsonl")
	_ = os.Remove(copied) // the rig's own, on the default account
	ways := k.ways()
	if _, err := t3TakeOver(k.hire, k.session, ways); err != nil {
		t.Fatal(err)
	}
	if text, err := os.ReadFile(copied); err != nil || string(text) != "{\"desk\":1}\n" {
		t.Fatalf("the copy where T3 Code looks: %q %v", text, err)
	}
	if k.saved.T3Transcript != copied || k.saved.T3Session != t3TakeOverSession || !k.saved.T3Copied {
		t.Errorf("on record: %+v", k.saved)
	}
	// T3 Code goes on writing its copy; Hand back brings it back, then takes the copy away.
	if err := os.WriteFile(copied, []byte("{\"desk\":1}\n{\"t3\":2}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := k.saved
	k.log = nil
	if note, err := t3HandBack(h, ways); err != nil || note != "" {
		t.Fatalf("%q %v", note, err)
	}
	if text, err := os.ReadFile(transcript); err != nil || string(text) != "{\"desk\":1}\n{\"t3\":2}\n" {
		t.Errorf("back in its own folder: %q %v", text, err)
	}
	if exists(copied) {
		t.Error("Take over's copy is still in ~/.claude")
	}
	if !slices.Contains(k.log, "resume "+t3TakeOverSession) || k.saved.T3Copied {
		t.Errorf("not resumed: %q %+v", k.log, k.saved)
	}
	// A second Take over makes the copy anew.
	k.hire.T3Session = ""
	if _, err := t3TakeOver(k.hire, k.session, ways); err != nil || !exists(copied) {
		t.Fatalf("again: %v", err)
	}
}

// RT126SAX9 B1: a Hand back whose resume failed (here X9's own "not signed in"), or whose save before the resume did,
// is refused, and the Hand back again its answer offers resumes the desk, with its own Claude folder as without: Take
// over's copy stays until the resume has run and its save is written, then goes. Your own transcript never goes.
func TestT3HandBackAgainAfterAFailedResumeResumesTheDesk(t *testing.T) {
	for _, own := range []bool{false, true} {
		for _, failed := range []string{"resume", "save"} {
			t.Run(fmt.Sprintf("own=%v,%s", own, failed), func(t *testing.T) {
				k := newT3TakeOverRig(t)
				copied, transcript := k.session.Transcript, k.session.Transcript
				if own {
					transcript = takenOverFromOwnClaude(t, k)
					_ = os.Remove(copied) // the rig's own, on the default account
				}
				ways := k.ways()
				if _, err := t3TakeOver(k.hire, k.session, ways); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(copied, []byte("{\"desk\":1}\n{\"t3\":2}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				works := ways.Save
				if failed == "resume" {
					k.resumed = errors.New(t3NotSignedIn)
				} else {
					ways.Save = func(id string, change func(*Hire)) error {
						probe := Hire{Engine: t3Engine}
						if change(&probe); probe.Engine == "" { // the save before the resume
							return errors.New("disk full")
						}
						return works(id, change)
					}
				}
				k.log = nil
				if _, err := t3HandBack(k.saved, ways); err == nil || slices.Contains(k.log, "resume "+t3TakeOverSession) != (failed == "resume") {
					t.Fatalf("the first Hand back: %v, log %q", err, k.log)
				}
				if k.saved.T3Session == "" || k.saved.T3Transcript != copied || !exists(copied) {
					t.Fatalf("after the first: %+v, the copy there %v", k.saved, exists(copied))
				}
				k.resumed, ways.Save, k.log = nil, works, nil
				if note, err := t3HandBack(k.saved, ways); err != nil || note != "" {
					t.Fatalf("the Hand back again: %q %v", note, err)
				}
				if !slices.Contains(k.log, "resume "+t3TakeOverSession) || k.saved.T3Session != "" || k.saved.T3Transcript != "" {
					t.Errorf("not resumed: %q %+v", k.log, k.saved)
				}
				if text, err := os.ReadFile(transcript); err != nil || string(text) != "{\"desk\":1}\n{\"t3\":2}\n" {
					t.Errorf("its session: %q %v", text, err)
				}
				if exists(copied) == own {
					t.Errorf("own=%v: the file in ~/.claude there: %v", own, exists(copied))
				}
			})
		}
	}
}

// RT126SAX9 N5: a Hand back that resumed a desk with its own Claude folder but couldn't write its last save keeps Take
// over's copy; the Hand back again that finds its session running clears the registry and takes the copy away, so the
// next Take over isn't refused for a file that's there.
func TestT3HandBackWhoseLastSaveFailedTakesTheCopyAwayWhenCleared(t *testing.T) {
	k := newT3TakeOverRig(t)
	transcript := takenOverFromOwnClaude(t, k)
	copied := t3OwnersCopy(k.home, transcript)
	_ = os.Remove(copied)
	ways := k.ways()
	if _, err := t3TakeOver(k.hire, k.session, ways); err != nil {
		t.Fatal(err)
	}
	works := ways.Save
	ways.Save = func(id string, change func(*Hire)) error {
		probe := Hire{T3Session: t3TakeOverSession}
		if change(&probe); probe.T3Session == "" { // the last save
			return errors.New("disk full")
		}
		return works(id, change)
	}
	if note, err := t3HandBack(k.saved, ways); err != nil || !strings.Contains(note, t3ResumedUnsaved("Ada")) || !exists(copied) {
		t.Fatalf("%q %v, the copy there %v", note, err, exists(copied))
	}
	keptMachine, keptStand := newT3Machine, t3HandingStand
	newT3Machine = k.machine
	t3HandingStand = func(*Deck, Hire) t3Handing { return k.ways() }
	t.Cleanup(func() { newT3Machine, t3HandingStand = keptMachine, keptStand })
	before := deskClaudeFolder(supportDir, k.hire.ID)
	d := newTestDeck(t) // a support folder of its own: the desk's Claude folder goes with it
	if err := os.MkdirAll(filepath.Join(supportDir, "hires", k.hire.ID), 0o700); err != nil || os.Rename(before, deskClaudeFolder(supportDir, k.hire.ID)) != nil {
		t.Fatal("the desk's Claude folder wasn't moved")
	}
	transcript = filepath.Join(deskClaudeFolder(supportDir, k.hire.ID), "projects", "-repo-hires-ada-abcdef", t3TakeOverSession+".jsonl")
	t3Registered(t, k.saved)
	d.Ingest(map[string]any{"hook_event_name": "SessionStart", "session_id": t3TakeOverSession, "cwd": k.hire.Worktree.storedOnly(),
		"transcript_path": transcript}, "claude", "tmux:%1@"+k.hire.TmuxSocket, k.hire.Worktree.storedOnly(), nowMillis())
	k.log = nil
	got := serve(t, d, t3PageCall("POST", "/setup/t3code/handback", `{"id":"`+k.hire.ID+`"}`))
	if got.Code != http.StatusOK || k.saved.T3Session != "" || k.saved.T3Copied || slices.Contains(k.log, "resume "+t3TakeOverSession) {
		t.Fatalf("%d %s %+v %q", got.Code, got.Body, k.saved, k.log)
	}
	if exists(copied) {
		t.Error("Take over's copy is still in ~/.claude")
	}
	if !exists(transcript) {
		t.Error("the desk's own transcript went")
	}
}

// RT126SAX9R2 F1: Take over's copy removed by hand after a Hand back whose resume failed (its 4c had brought the copy
// back already) leaves the desk's own file as the session: Hand back again goes on from it and says so, so the desk
// isn't stuck under Fire's refusal. Gone there too, or a link there, is still refused, with nothing resumed.
func TestT3HandBackAfterTheCopyWasRemovedByHandGoesOnFromTheDesksOwnFile(t *testing.T) {
	for _, there := range []string{"plain", "gone", "link"} {
		t.Run(there, func(t *testing.T) {
			k := newT3TakeOverRig(t)
			transcript := takenOverFromOwnClaude(t, k)
			copied := t3OwnersCopy(k.home, transcript)
			_ = os.Remove(copied) // the rig's own, on the default account
			ways := k.ways()
			if _, err := t3TakeOver(k.hire, k.session, ways); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(copied, []byte("{\"desk\":1}\n{\"t3\":2}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			k.resumed = errors.New(t3NotSignedIn)
			if _, err := t3HandBack(k.saved, ways); err == nil {
				t.Fatal("the first Hand back wasn't refused")
			}
			if err := os.Remove(copied); err != nil {
				t.Fatal(err)
			}
			switch there {
			case "gone":
				_ = os.Remove(transcript)
			case "link":
				outside := filepath.Join(k.home, "outside.jsonl")
				if err := os.WriteFile(outside, []byte("{\"yours\":1}\n"), 0o600); err != nil || os.Remove(transcript) != nil || os.Symlink(outside, transcript) != nil {
					t.Fatal("no link")
				}
			}
			k.resumed, k.log = nil, nil
			note, err := t3HandBack(k.saved, ways)
			if there != "plain" {
				if err == nil || !strings.Contains(err.Error(), "its file is gone, so it wasn't resumed") || slices.Contains(k.log, "resume "+t3TakeOverSession) {
					t.Fatalf("%s: %q %v, log %q", there, note, err, k.log)
				}
				if k.saved.T3Session == "" || t3FireRefusal(k.saved) == "" {
					t.Errorf("%s: no longer waiting for Hand back: %+v", there, k.saved)
				}
				return
			}
			if err != nil || !strings.Contains(note, "Ada's session was no longer in ~/.claude, so it went on from the file in its own Claude folder.") {
				t.Fatalf("the Hand back again: %q %v", note, err)
			}
			if !slices.Contains(k.log, "resume "+t3TakeOverSession) || k.saved.T3Session != "" || k.saved.T3Transcript != "" || k.saved.T3Copied {
				t.Errorf("not resumed: %q %+v", k.log, k.saved)
			}
			if why := t3FireRefusal(k.saved); why != "" {
				t.Errorf("Fire still refused: %q", why)
			}
			if text, err := os.ReadFile(transcript); err != nil || string(text) != "{\"desk\":1}\n{\"t3\":2}\n" {
				t.Errorf("its session: %q %v", text, err)
			}
			if exists(copied) {
				t.Error("a file in ~/.claude was made again")
			}
		})
	}
}

// RT126SAX9R2 F2: Hand back moves only a copy Take over made and put on record (T3Copied), never a transcript for where
// it is. A desk taken over on your ~/.claude, then given a Claude folder of its own (as a restart that makes it anew
// could, SA-X14), is resumed with your transcript where it was, as it was, and nothing written in its folder.
func TestT3HandBackMovesOnlyACopyTakeOverMade(t *testing.T) {
	k := newT3TakeOverRig(t)
	yours := k.session.Transcript
	if err := os.WriteFile(yours, []byte("{\"yours\":1}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ways := k.ways()
	if _, err := t3TakeOver(k.hire, k.session, ways); err != nil {
		t.Fatal(err)
	}
	if k.saved.T3Transcript != yours || k.saved.T3Copied {
		t.Fatalf("on record: %+v", k.saved)
	}
	own := deskClaudeFolder(supportDir, k.hire.ID)
	if err := os.MkdirAll(own, 0o700); err != nil {
		t.Fatal(err)
	}
	k.log = nil
	if note, err := t3HandBack(k.saved, ways); err != nil || note != "" || !slices.Contains(k.log, "resume "+t3TakeOverSession) {
		t.Fatalf("%q %v, log %q", note, err, k.log)
	}
	if text, err := os.ReadFile(yours); err != nil || string(text) != "{\"yours\":1}\n" {
		t.Errorf("your transcript: %q %v", text, err)
	}
	if exists(filepath.Join(own, "projects")) {
		t.Error("your transcript was brought into the desk's folder")
	}
}

// Take over never writes over a file in your ~/.claude, nor copies a desk's transcript that's a link; either hands the
// desk back at once, with nothing sent to T3 Code.
func TestT3TakeOverNeverCopiesOverYourFileNorThroughALink(t *testing.T) {
	for _, c := range []string{"yours", "link"} {
		t.Run(c, func(t *testing.T) {
			k := newT3TakeOverRig(t)
			transcript := takenOverFromOwnClaude(t, k)
			copied := filepath.Join(k.home, ".claude", "projects", "-repo-hires-ada-abcdef", t3TakeOverSession+".jsonl")
			if c == "link" {
				_ = os.Remove(copied)
				secret := filepath.Join(k.home, "fake-secret")
				if err := os.WriteFile(secret, []byte("sk-fake-x9"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(transcript); err != nil || os.Symlink(secret, transcript) != nil {
					t.Fatal("no link")
				}
			}
			before, _ := os.ReadFile(copied)
			_, err := t3TakeOver(k.hire, k.session, k.ways())
			if err == nil || !strings.Contains(err.Error(), "Ada's session couldn't be copied where T3 Code looks for it") ||
				!strings.Contains(err.Error(), "was resumed in its tmux session") {
				t.Fatalf("err = %v", err)
			}
			if after, _ := os.ReadFile(copied); string(after) != string(before) {
				t.Errorf("your file became %q", after)
			}
			if len(k.srv.made()) != 0 || slices.ContainsFunc(k.runs(), func(run []string) bool { return len(run) > 1 && run[1] == "add" }) {
				t.Errorf("T3 Code heard of it: %q %q", k.srv.made(), k.runs())
			}
			if k.saved.T3Transcript != "" && k.saved.T3Transcript != transcript {
				t.Errorf("on record: %q", k.saved.T3Transcript)
			}
		})
	}
}

// Hand back writes the desk's folder through an os.Root: a link the desk left at its transcript is replaced, never
// written through, and one at its project's folder that leads out refuses the resume, with your file untouched.
func TestT3HandBackNeverWritesThroughALinkInTheDesksFolder(t *testing.T) {
	k := newT3TakeOverRig(t)
	transcript := takenOverFromOwnClaude(t, k)
	copied := filepath.Join(k.home, ".claude", "projects", "-repo-hires-ada-abcdef", t3TakeOverSession+".jsonl")
	if err := os.WriteFile(copied, []byte("{\"t3\":2}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(k.home, "outside")
	if err := os.WriteFile(outside, []byte("yours"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(transcript); err != nil || os.Symlink(outside, transcript) != nil {
		t.Fatal("no link")
	}
	h := k.takenOver()
	h.T3Transcript, h.T3Copied = copied, true
	if _, err := t3HandBack(h, k.ways()); err != nil {
		t.Fatal(err)
	}
	if text, _ := os.ReadFile(outside); string(text) != "yours" {
		t.Errorf("written through the link: %q", text)
	}
	if info, err := os.Lstat(transcript); err != nil || !info.Mode().IsRegular() {
		t.Errorf("the link wasn't replaced: %v", err)
	}

	// Its project's folder a link out of its folder: refused, not resumed, and Hand back is still offered.
	k = newT3TakeOverRig(t)
	takenOverFromOwnClaude(t, k)
	project := filepath.Join(deskClaudeFolder(supportDir, k.hire.ID), "projects", "-repo-hires-ada-abcdef")
	if err := os.RemoveAll(project); err != nil || os.Symlink(k.home, project) != nil {
		t.Fatal("no link")
	}
	copied = filepath.Join(k.home, ".claude", "projects", "-repo-hires-ada-abcdef", t3TakeOverSession+".jsonl")
	if err := os.WriteFile(copied, []byte("{\"t3\":2}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h = k.takenOver()
	h.T3Transcript, h.T3Copied = copied, true
	k.log = nil
	_, err := t3HandBack(h, k.ways())
	if err == nil || !strings.Contains(err.Error(), "Ada's session couldn't be brought back to its own Claude folder") ||
		slices.Contains(k.log, "resume "+t3TakeOverSession) {
		t.Fatalf("err = %v, log %q", err, k.log)
	}
	if !exists(copied) || exists(filepath.Join(k.home, t3TakeOverSession+".jsonl")) {
		t.Error("the copy went, or was written out of the desk's folder")
	}
}

// Hand back's resume goes on in the desk's own folder, its plans outside it, on the default account's sign-in; one that
// isn't signed in isn't resumed.
func TestT3TheResumeOfADeskGoesOnInItsOwnFolder(t *testing.T) {
	d, r, hire := hiredForFire(t, "")
	r.tmux.up = false
	_ = os.Remove(hire.TmuxSocket)
	own := deskClaudeFolder(supportDir, hire.ID)
	if _, err := d.t3ResumeInTmux(hire, t3TakeOverSession, nil); err != nil {
		t.Fatal(err)
	}
	script := r.script(hire)
	for _, want := range []string{"export CLAUDE_CONFIG_DIR=" + shellQuoted(own) + "\n", "export " + secureStorageEnvironment + "=''\n", "'--resume' '" + t3TakeOverSession + "'"} {
		if !strings.Contains(script, want) {
			t.Errorf("the resume lacks %q:\n%s", want, script)
		}
	}
	plan, _ := issuedPlan(hire.ID)
	settings, err := os.ReadFile(plan.Settings.Path)
	if err != nil || !strings.Contains(string(settings), `"plansDirectory"`) {
		t.Errorf("its settings name no plans folder (%v)", err)
	}
	if info, err := os.Lstat(filepath.Join(hire.storedStart(), filepath.FromSlash(deskPlansFor(hire.ID)))); err != nil || !info.IsDir() {
		t.Errorf("no plans folder: %v", err)
	}

	r.tmux.up = false
	got := signInStandIn(t, `{"loggedIn": false}`)
	if _, err := d.t3ResumeInTmux(hire, t3TakeOverSession, nil); err == nil || err.Error() != t3NotSignedIn {
		t.Fatalf("err = %v", err)
	}
	if lines := askedLines(t, got); len(lines) != 4 || lines[1] != "config: "+own || lines[2] != "secure: " {
		t.Errorf("asked with %q", lines)
	}
}
