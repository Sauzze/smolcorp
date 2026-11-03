package desktop

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// T745: the GM names one person in the office with its own token, and nothing else changes.
func TestTheGMNamesOnePersonAndNothingElse(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, hireKey := withGMAndAHire(t, d)
	folder := "/work/app"
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "session_id": "s1", "cwd": folder}, Claude, "tab-s1", "", 1761000540000)
	pinned := "/work/pinned"
	d.config.Pins[1] = &pinned
	d.config.Layout.Room.Team = map[string]TeamMember{"other": TeamMember{}.checked()}
	other := d.config.Layout.Room.Team["other"]
	other.Outfit, other.Name = "suit", "Boss"
	d.config.Layout.Room.Team["other"] = other
	d.mu.Unlock()
	post := func(body string, headers map[string]string) int {
		headers[setupHeader] = "1"
		return serve(t, d, call{method: "POST", target: "/setup/gm/nickname", host: "localhost:7710", from: thisComputer,
			withoutSetupKey: true, body: body, headers: headers}).Code
	}
	team := func() map[string]TeamMember {
		d.mu.Lock()
		defer d.mu.Unlock()
		out := map[string]TeamMember{}
		for k, v := range d.config.Layout.Room.Team {
			out[k] = v
		}
		return out
	}
	asGM := map[string]string{gmTokenHeader: gm}

	if got := post(`{"folder":"/work/app","name":"  Ada "}`, asGM); got != http.StatusOK {
		t.Fatalf("the GM's nickname: %d", got)
	}
	want := TeamMember{}.checked()
	want.Name = "Ada"
	if got := team(); !reflect.DeepEqual(got, map[string]TeamMember{"app": want, "other": other}) {
		t.Errorf("only app should be named, trimmed: %+v", got)
	}
	if loadConfig().Layout.Room.Team["app"].Name != "Ada" {
		t.Error("the nickname is saved")
	}
	// A pinned project with no session is a person too; an empty name takes it away again.
	if got := post(`{"folder":"/work/pinned","name":"Pin"}`, asGM); got != http.StatusOK || team()["pinned"].Name != "Pin" {
		t.Errorf("a pinned project: %d %+v", got, team())
	}
	if got := post(`{"folder":"/work/pinned","name":""}`, asGM); got != http.StatusOK {
		t.Fatalf("empty name: %d", got)
	}
	if _, kept := team()["pinned"]; kept {
		t.Error("a member with nothing set is dropped, as the page does")
	}

	// Refusals change nothing.
	before := team()
	for body, status := range map[string]int{
		`{"folder":"/work/nobody","name":"X"}`:                            http.StatusNotFound,
		`{"folder":"/work/app","name":"` + strings.Repeat("x", 17) + `"}`: http.StatusBadRequest,
		`{"folder":"/work/app","name":"a\nb"}`:                            http.StatusBadRequest,
		`{"folder":"/work/app","name":"a\u0085b"}`:                        http.StatusBadRequest,
		`{"folder":"/work/app","name":"a\u000bb"}`:                        http.StatusBadRequest,
		`{"folder":"/work/app","name":"a\u000cb"}`:                        http.StatusBadRequest,
		`{"folder":"/work/app","name":"a\u2028b"}`:                        http.StatusBadRequest,
		`{"folder":"/work/app","name":"a\u2029b"}`:                        http.StatusBadRequest,
		`{"folder":"/work/app"}`:                                          http.StatusBadRequest,
		`{"name":"X"}`:                                                    http.StatusBadRequest,
		`nonsense`:                                                        http.StatusBadRequest,
	} {
		if got := post(body, asGM); got != status {
			t.Errorf("%s: %d, want %d", body, got, status)
		}
	}
	if got := team(); !reflect.DeepEqual(got, before) {
		t.Errorf("refusals changed the team: %+v", got)
	}
	if got := post(`{"folder":"/work/app","name":"`+strings.Repeat("é", 16)+`"}`, asGM); got != http.StatusOK {
		t.Errorf("16 characters is allowed: %d", got)
	}

	// The page may; a hire with its send key and a tool with the setup-key file may not.
	if got := serve(t, d, call{method: "POST", target: "/setup/gm/nickname", host: "localhost:7710", from: thisComputer,
		body: `{"folder":"/work/app","name":"Page"}`, headers: map[string]string{setupHeader: "1"}}).Code; got != http.StatusOK {
		t.Errorf("the page: %d", got)
	}
	if got := post(`{"folder":"/work/app","name":"Hire"}`, map[string]string{hireKeyHeader: hireKey}); got != http.StatusForbidden {
		t.Errorf("a hire with a send key: %d, want 403", got)
	}
	if got := serve(t, d, call{method: "POST", target: "/setup/gm/nickname", host: "localhost:7710", from: thisComputer, asTool: true,
		body: `{"folder":"/work/app","name":"Tool"}`, headers: map[string]string{setupHeader: "1"}}).Code; got != http.StatusForbidden {
		t.Errorf("a tool: %d, want 403", got)
	}
	if got := team()["app"].Name; got != "Page" {
		t.Errorf("the refused callers changed it: %q", got)
	}
}

func TestTheOfficeRemembersAtMostFiftyPeopleToTheGM(t *testing.T) {
	layout := Layout{}
	layout.Room.Team = map[string]TeamMember{}
	for i := 0; i < teamLimit; i++ {
		layout.Room.Team[string(rune('a'+i%26))+strings.Repeat("z", i/26)] = TeamMember{}.checked()
	}
	_, _, refused := setNickname(&layout, []byte(`{"folder":"/work/app","name":"Ada"}`), []namePlace{{project: "/work/app", folder: "/work/app"}}, nil)
	if refused == nil || refused.status != http.StatusConflict {
		t.Errorf("the 51st person: %+v", refused)
	}
}
