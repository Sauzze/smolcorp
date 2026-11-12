//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// A GM held to one project (gmscope.go) through the gate service: the GM, an MCP key and a night job are held
// to its scope; the owner's own and a hire aren't; what the route is given is the request as checked.
func TestTheGMsScopeHoldsTheGMAnMCPKeyAndANightJobAlike(t *testing.T) {
	d, _, _, _, project := scopedGM(t)
	held := []actor{actorGM, mcpActor("claude-desktop"), nightActor("nightly")}
	ask := func(method, path, body string) action {
		return action{route: &routeAsk{method: method, path: path, body: strings.NewReader(body)}}
	}
	for _, a := range held {
		// Beyond the scope: refused in the scope's own words.
		v := d.decide(a, ask("POST", "/setup/gm/tile", `{}`))
		if v.allowed() || v.status != http.StatusForbidden || v.why != "This General Manager works only in "+project+": it doesn't move the office's windows." {
			t.Errorf("tile by %s: %+v", a, v)
		}
		// Into its project, allowed, with the folder as checked and the account it was hired on filled in.
		v = d.decide(a, ask("POST", "/setup/gm/hire", `{"role":"janitor","folder":"`+filepath.Join(project, ".")+`/"}`))
		body, _ := json.Marshal(v.body)
		var given map[string]any
		_ = json.Unmarshal(body, &given)
		if !v.allowed() || given["folder"] != project || given["account"] != claudeDefaultName || given["role"] != "janitor" {
			t.Errorf("hire by %s: %+v, given %s", a, v, body)
		}
		// A read isn't held, and its body is left alone.
		if v := d.decide(a, ask("GET", "/setup/gm/team", "")); !v.allowed() || v.body != nil {
			t.Errorf("a read by %s: %+v", a, v)
		}
	}
	// The page isn't held to the GM's scope, nor is a hire (the table keeps it from tile), nor the body read.
	if v := d.decide(actorPage, ask("POST", "/setup/gm/tile", `{}`)); !v.allowed() || v.body != nil {
		t.Errorf("tile by the page: %+v", v)
	}
	if v := d.decide(actorHire, ask("POST", "/setup/gm/tile", `{}`)); v.allowed() || v.why != routeRefusal("POST", "/setup/gm/tile") {
		t.Errorf("tile by a hire: %+v", v)
	}
}
