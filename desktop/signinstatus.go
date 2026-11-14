package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ✨ Sign-in status (1.26 S1): whether Claude, Codex and Gemini are signed in,
// for setup's Accounts. Read only, and as little as can be: each agent's own
// status command is run (a resolved path, a cleaned environment, a short
// deadline, a cut output), its answer is read with an allow-list, and only
// three things leave: is it installed, signed in / signed out / can't tell,
// and the account's name or email when the command printed one. Nothing it
// printed is passed on as it came, so a token it printed can't reach the
// answer. No credential file is read, nor any Keychain item.
//
// TODO(A2): list an ACP agent's `authenticate` methods here once A2's data
// (the methods an agent says it takes, from its `initialize`) is on this branch.

type accountState string

const (
	accountSignedIn  accountState = "signedIn"
	accountSignedOut accountState = "signedOut"
	accountUnknown   accountState = "unknown" // can't tell
)

// agentSignIn is one agent's row of the answer.
type agentSignIn struct {
	Agent     string       `json:"agent"`
	Installed bool         `json:"installed"`
	State     accountState `json:"state"`
	// The account's name or email, only when the agent's own status command printed one.
	Account string `json:"account,omitempty"`
}

// A status command is a quick one: past this it's ended with all it started, and the answer is "can't tell"
// (a variable, for the tests).
var signInDeadline = 5 * time.Second

const (
	// What a status command prints is read up to this much, and no more: a longer one is "can't tell".
	signInOutputMost = 8 << 10
	signInErrorMost  = 2 << 10
	// How long an answer is kept, and the least time between two runs the page's own refresh can ask for.
	signInKeep       = 30 * time.Second
	signInLeastApart = 2 * time.Second
	// The longest account name or email passed on.
	signInAccountMost = 120
)

// signInFind is where an agent's program is, "" nowhere: a variable, for the tests, which look only in a
// folder of their own, so no real program is ever found, let alone run.
var signInFind = programNamed

// signInAgents are the agents asked, in the order shown.
var signInAgents = []string{"claude", "codex", "gemini"}

// signInStatus keeps the last answer for a short time. The zero value is ready.
type signInStatus struct {
	mu     sync.Mutex
	at     time.Time
	agents []agentSignIn
	// For tests: the clock, and what finds out (the real one, when nil).
	now   func() time.Time
	probe func() []agentSignIn
}

func (s *signInStatus) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// get is the answer: the one kept, while it's fresh, else a new one. refresh
// asks for a new one now, though never within signInLeastApart of the last
// (a page asking again and again runs nothing again and again). One caller
// runs the commands while the others wait for its answer.
func (s *signInStatus) get(refresh bool) []agentSignIn {
	s.mu.Lock()
	defer s.mu.Unlock()
	age := s.clock().Sub(s.at)
	if s.agents != nil && age >= 0 && age < signInKeep && (!refresh || age < signInLeastApart) {
		return append([]agentSignIn(nil), s.agents...)
	}
	probe := s.probe
	if probe == nil {
		probe = probeSignIn
	}
	s.agents, s.at = probe(), s.clock()
	return append([]agentSignIn(nil), s.agents...)
}

// probeSignIn asks each agent that's here, the two that have a status command at once.
func probeSignIn() []agentSignIn {
	env := summaryEnvironment(os.Environ())
	rows := make([]agentSignIn, len(signInAgents))
	var wg sync.WaitGroup
	for i, agent := range signInAgents {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows[i] = probeAgentSignIn(agent, signInFind(agent), env)
		}()
	}
	wg.Wait()
	return rows
}

// probeAgentSignIn is one agent's row, from its program (a resolved path, or "" when it isn't here).
func probeAgentSignIn(agent, program string, env []string) agentSignIn {
	row := agentSignIn{Agent: agent, Installed: program != "", State: accountUnknown}
	// A Claude Code that isn't Anthropic's is never run: its state can't be told (agentcheck.go).
	if program == "" || checkAgentProgram(agent, program) != nil {
		return row
	}
	run := func(args ...string) summaryRan {
		return spawnCapped(program, args, nil, env, os.TempDir(), signInDeadline, signInOutputMost, signInErrorMost)
	}
	switch agent {
	case "claude":
		// The way claudeLogin reads it: `claude auth status --json`.
		ran := run("auth", "status", "--json")
		if ran.timedOut || ran.status < 0 {
			return row
		}
		row.State, row.Account = claudeSignInOf(ran.output)
	case "codex":
		// `codex login status` says whether it's logged in, and prints no token (an API key login is
		// printed masked; that part of the line is never read).
		ran := run("login", "status")
		if ran.timedOut || ran.status < 0 {
			return row
		}
		row.State = codexSignInOf(ran.status, ran.output, ran.errors)
	default:
		// Gemini has no documented status command that asks nothing: `gemini` alone opens its
		// interface, and the one thing that would say (its login file) holds a token, which is
		// never read. Installed or not is all that's known; nothing is run.
	}
	return row
}

// claudeSignInOf is a login's state and account from `claude auth status --json`:
// one JSON object with a loggedIn that is true or false, else "can't tell". Only
// "email" is passed on, and only when it reads as an email.
func claudeSignInOf(printed []byte) (accountState, string) {
	if len(printed) >= signInOutputMost {
		return accountUnknown, ""
	}
	var status struct {
		LoggedIn *bool   `json:"loggedIn"`
		Email    *string `json:"email"`
	}
	if json.Unmarshal(printed, &status) != nil || status.LoggedIn == nil {
		return accountUnknown, ""
	}
	if !*status.LoggedIn {
		return accountSignedOut, ""
	}
	// claudeLogin's own reading must agree: a login it calls signed in.
	if claudeLogin(printed) == "" {
		return accountUnknown, ""
	}
	account := ""
	if status.Email != nil {
		account = accountOf(*status.Email)
	}
	return accountSignedIn, account
}

var (
	codexSignedIn  = regexp.MustCompile(`^Logged in using (?:ChatGPT|an API key)(?:$|[\s.:,-])`)
	codexSignedOut = regexp.MustCompile(`^Not logged in\.?$`)
)

// codexSignInOf is a login's state from `codex login status`. Each line is
// read as one of two sentences, and only the sentence's start: "Logged in using
// ChatGPT" or "Logged in using an API key" (whatever follows, a masked key, is
// never kept), or "Not logged in". A login needs the command's success, a
// logout its failure; anything else, or both sentences, or too much, is "can't tell".
func codexSignInOf(status int, output, errors []byte) accountState {
	if len(output) >= signInOutputMost || len(errors) >= signInErrorMost {
		return accountUnknown
	}
	in, out := false, false
	for _, line := range strings.Split(string(output)+"\n"+string(errors), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case codexSignedIn.MatchString(line):
			in = true
		case codexSignedOut.MatchString(line):
			out = true
		}
	}
	switch {
	case in && !out && status == 0:
		return accountSignedIn
	case out && !in && status != 0:
		return accountSignedOut
	}
	return accountUnknown
}

// accountPattern is what an email looks like, and no more.
var accountPattern = regexp.MustCompile(`^[A-Za-z0-9._%+'-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}$`)

// accountOf is an account's email as passed on: short, one email and nothing
// else, and not a line that looks like a secret; else "".
func accountOf(printed string) string {
	account := strings.TrimSpace(printed)
	if len(account) > signInAccountMost || !accountPattern.MatchString(account) || redactText(account) != account {
		return ""
	}
	return account
}

// accountsStatus answers GET /setup/accounts/status: the page's alone (routes.go).
// ?refresh=1 asks to look again, as the page's refresh button does.
func (rt *router) accountsStatus(w http.ResponseWriter, r *request) {
	if r.caller != callerPage {
		writeStatus(w)(http.StatusForbidden, routeRefusal(r.Method, r.URL.Path))
		return
	}
	agents := rt.deck.signInSeen.get(r.URL.Query().Get("refresh") == "1")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
}
