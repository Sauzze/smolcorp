package desktop

import (
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

// A desk's sign-in, as its own Claude folder sees it (SA-X9, SANDBOX-PLAN A0.1 C2 and SB15). A desk with a Claude folder
// of its own (SA-X8) uses its account's sign-in through CLAUDE_SECURESTORAGE_CONFIG_DIR: nothing is copied, and no
// token is pasted. Whether that works is asked of Claude Code itself, `claude auth status --json`, with the desk's two
// folders, at every start: a desk that isn't signed in is refused, with what to do (signInYourHires), rather than
// started into a window that asks for a sign-in nobody may see. Setup asks the same question per account, in a folder
// made for the asking (hiresSignIn), so its Claude accounts say whether your hires can use each one.
//
// Only what Claude Code says is read: never a credential file, a Keychain item or a token (signInFrom).

// signInYourHires is a start refused because the desk's Claude folder isn't signed in (SANDBOX-PLAN §A5.2).
const signInYourHires = "Sign in your hires once in Setup (Claude sign-in), then start this hire again."

// deskSignInEnvironment is the hire's environment (hireEnvironment), with the desk's own Claude folder as
// CLAUDE_CONFIG_DIR and its account's as CLAUDE_SECURESTORAGE_CONFIG_DIR ("" for ~/.claude's), as hireStart.plan sets
// them: never what Smolcorp's own environment says.
func deskSignInEnvironment(desk, account string) []string {
	env := slices.DeleteFunc(hireEnvironment(), func(variable string) bool {
		name, _, _ := strings.Cut(variable, "=")
		return strings.EqualFold(name, "CLAUDE_CONFIG_DIR") || strings.EqualFold(name, secureStorageEnvironment)
	})
	return append(env, "CLAUDE_CONFIG_DIR="+desk, secureStorageEnvironment+"="+account)
}

// deskSignInFind is the Claude Code asked, "" none: a variable, for the tests, which never find a real one.
var deskSignInFind = func() string { return claudeFoundAt() }

// deskSignIn asks Claude Code whether a desk whose Claude folder is desk, on the account whose folder is account,
// is signed in.
func deskSignIn(desk, account string) signIn { return deskSignInWith(deskSignInFind, desk, account) }

// deskSignInWith is deskSignIn, asking the Claude Code find names. One that isn't Anthropic's is never run
// (agentcheck.go): it can't say.
func deskSignInWith(find func() string, desk, account string) signIn {
	program := find()
	if program == "" || checkAgentProgram(string(Claude), program) != nil {
		return signInUnknown
	}
	command := signInCommand(program)
	return signInFrom(quickRun(command[0], command[1:], deskSignInEnvironment(desk, account)).output)
}

// deskSignInRefusal is why a desk whose Claude folder is desk, on account, isn't started, or "". Only a sign-in Claude
// Code says is missing refuses: one it can't tell about (no answer in time, an answer not read) goes on, as Setup's
// line says it can (gmPreflightOn). An API-key hire's key isn't asked about here: it comes with its start alone.
func deskSignInRefusal(desk, account string, apiKey bool) string {
	if apiKey || deskSignIn(desk, account) != signedOut {
		return ""
	}
	return signInYourHires
}

// hiresSignIn is whether a hire on the account whose folder is account ("" for ~/.claude's) would start signed in:
// asked as deskSignIn asks it, in a Claude folder made for the asking as a desk's is made (seeded from the account's
// .claude.json, with a read-only copy of its settings.json, where an env or an apiKeyHelper may sign it in: fill), and
// taken away after. The links fill makes aren't needed to ask.
func hiresSignIn(account string) signIn { return hiresSignInWith(deskSignInFind, account) }

// hiresSignInFolderPrefix begins the name of the folder hiresSignIn asks in, in the support folder.
const hiresSignInFolderPrefix = "signin-"

// hiresSignInWith is hiresSignIn, asking the Claude Code find names. The folder is made in Smolcorp's support folder,
// which every desk's sandbox denies, as the GM's workspace is (makeGMWorkspace): never in the user's temp folder, where
// a desk could put a settings.json of its own for the Claude Code asked, outside any sandbox, to read (FXSWEEP3P3 F-4).
func hiresSignInWith(find func() string, account string) signIn {
	folder, err := os.MkdirTemp(supportDir, hiresSignInFolderPrefix)
	if err != nil {
		return signInUnknown
	}
	defer os.RemoveAll(folder)
	desk := deskClaude{Account: account, Trusted: folder}
	seeded, err := desk.seed()
	if err != nil || writeNewFile(claudeJSON(folder), seeded, 0o600) != nil || desk.copySettings(folder) != nil {
		return signInUnknown
	}
	return deskSignInWith(find, folder, account)
}

// hiresSignInRow is one account's line in Setup's answer: its name, and whether hires on it start signed in.
type hiresSignInRow struct {
	Account string       `json:"account"`
	State   accountState `json:"state"`
}

// hiresSignInStatus keeps the last answer for every account, as signInStatus keeps its own. The zero value is ready.
type hiresSignInStatus struct {
	mu     sync.Mutex
	at     time.Time
	asked  string // the accounts it was asked for, as one key: another list is asked again
	rows   []hiresSignInRow
	now    func() time.Time
	signIn func(account string) signIn // hiresSignIn, or a test's
}

// get is the answer for accounts: the one kept while it's fresh and for the same accounts, else a new one, asked of
// each at once. refresh asks again now, though never within signInLeastApart of the last.
func (s *hiresSignInStatus) get(accounts []claudeProfile, refresh bool) []hiresSignInRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	clock, ask := time.Now, hiresSignIn
	if s.now != nil {
		clock = s.now
	}
	if s.signIn != nil {
		ask = s.signIn
	}
	key := ""
	for _, p := range accounts {
		key += p.Name + "\x00" + p.Dir + "\x00"
	}
	age := clock().Sub(s.at)
	if s.rows != nil && key == s.asked && age >= 0 && age < signInKeep && (!refresh || age < signInLeastApart) {
		return slices.Clone(s.rows)
	}
	rows := make([]hiresSignInRow, len(accounts))
	var wg sync.WaitGroup
	for i, p := range accounts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			folder := p.Dir
			if p.Name == claudeDefaultName {
				folder = ""
			}
			state := accountUnknown
			switch ask(folder) {
			case signedIn:
				state = accountSignedIn
			case signedOut:
				state = accountSignedOut
			}
			rows[i] = hiresSignInRow{Account: p.Name, State: state}
		}()
	}
	wg.Wait()
	s.rows, s.at, s.asked = rows, clock(), key
	return slices.Clone(rows)
}
