package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// An engine hire's start environment, on the launcher's way (hirelauncher.go).
// The tmux engine puts a hire's secrets in its start script; a launcher has no
// script, and a command line or a file would show them. So the launcher asks
// the app for them once, before it starts the agent, over loopback, proving it
// is that hire's with the hire's one-time token (hireToken's shape, good for
// hiredTabWait), and sets them on the agent's process only. Nothing here is
// logged, put in an event, or written down. A launcher that can't get them
// starts nothing.

// hireStartPath is where a launcher asks for its start environment.
const hireStartPath = "/launcher/start"

// hireStartEnv is what the launcher sets on the agent, and nowhere else.
type hireStartEnv struct {
	// Set are variables to set (the hire's send key, its ports, the GM's token, a named account).
	Set map[string]string `json:"set,omitempty"`
	// Unset are variables to take out of what the launcher inherited (any GM token a
	// hire inherits; CLAUDE_CONFIG_DIR for the default account).
	Unset []string `json:"unset,omitempty"`
	// PathFirst is a folder to put first on the agent's PATH: the app's own smolcorp.
	PathFirst string `json:"pathFirst,omitempty"`
}

// startEnvironment is base with env applied: unset names removed, set names
// replaced (names compared as the OS does: Windows ignores case), PathFirst
// first on PATH. base isn't changed.
func startEnvironment(base []string, env hireStartEnv, caseless bool) []string {
	same := func(a, b string) bool { return a == b || caseless && strings.EqualFold(a, b) }
	out := slices.DeleteFunc(slices.Clone(base), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		if slices.ContainsFunc(env.Unset, func(u string) bool { return same(u, name) }) {
			return true
		}
		for set := range env.Set {
			if same(set, name) {
				return true
			}
		}
		return false
	})
	if env.PathFirst != "" {
		for i, entry := range out {
			if name, value, _ := strings.Cut(entry, "="); same(name, "PATH") {
				out[i] = name + "=" + env.PathFirst + string(os.PathListSeparator) + value
			}
		}
	}
	names := make([]string, 0, len(env.Set))
	for name := range env.Set {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		out = append(out, name+"="+env.Set[name])
	}
	return out
}

// takeStartEnv gives the start environment of the engine hire hireID to the
// launcher that proves it with token, once: the token must be a hire's, made
// for exactly that hire, fresh, and not used. status says why not.
func (d *Deck) takeStartEnv(token, hireID string) (env *hireStartEnv, status int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	pending, ok := d.hires[token]
	switch {
	case !ok || !hireToken.MatchString(token) || pending.hireID == "":
		return nil, http.StatusForbidden
	case pending.hireID != hireID:
		return nil, http.StatusForbidden
	case time.Since(pending.at) > hiredTabWait:
		delete(d.hires, token)
		return nil, http.StatusGone
	case pending.env == nil:
		return nil, http.StatusGone // used already
	}
	env, pending.env = pending.env, nil
	return env, http.StatusOK
}

// startEnvRoute answers GET /launcher/start?hire=<token>&id=<hire id>. The
// caller has checked it is local and carries the hook key.
func (d *Deck) startEnvRoute(w http.ResponseWriter, query url.Values) {
	env, status := d.takeStartEnv(query.Get("hire"), query.Get("id"))
	if env == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(env)
}

// hireStartTimeout is how long the launcher waits for the app's answer (a var so a test can shorten it).
var hireStartTimeout = 10 * time.Second

// hireStartClient is the launcher's client for asking the app: an app that
// takes the call and never answers must not hold the window for good.
func hireStartClient() *http.Client {
	return &http.Client{Timeout: hireStartTimeout}
}

// fetchHireStartEnv is the launcher's side: it asks the app at base for the
// hire's environment. The error says why, never with the token.
func fetchHireStartEnv(client *http.Client, base, hookKey, token, hireID string) (hireStartEnv, error) {
	var env hireStartEnv
	if strings.TrimSpace(hookKey) == "" {
		return env, errors.New("Smolcorp isn't running, or its key can't be read")
	}
	request, err := http.NewRequest(http.MethodGet, base+hireStartPath+"?"+url.Values{"hire": {token}, "id": {hireID}}.Encode(), nil)
	if err != nil {
		return env, errors.New("no request could be made")
	}
	request.Header.Set(hookKeyHeader, strings.TrimSpace(hookKey))
	response, err := client.Do(request)
	if err != nil {
		return env, errors.New("Smolcorp didn't answer")
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusGone:
		return env, errors.New("this hire's start settings were taken already, or too long ago. Hire again")
	case http.StatusForbidden, http.StatusUnauthorized, http.StatusNotFound:
		return env, errors.New("Smolcorp doesn't know this hire. Hire again")
	default:
		return env, fmt.Errorf("Smolcorp answered %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<16)).Decode(&env); err != nil {
		return hireStartEnv{}, errors.New("Smolcorp's answer couldn't be read")
	}
	return env, nil
}

// settingsArgument is the one extra argument a hire's launcher takes after its
// role: the hire's own Claude Code settings file, by an absolute path.
func settingsArgument(arg string) bool {
	path, ok := strings.CutPrefix(arg, "--settings=")
	return ok && filepath.IsAbs(path) && !strings.ContainsAny(path, "\x00\r\n")
}
