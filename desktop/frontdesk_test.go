package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var frontDeskFixtures = filepath.Join("..", "Tests", "fixtures", "frontdesk")

func frontDeskFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(frontDeskFixtures, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// asJSON turns any value into what a client would read, so numbers compare alike.
func asJSONValue(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTheTeamIsTheRegistryMergedWithLiveSessionsAsTeamJSONSays(t *testing.T) {
	var registry struct{ Hires []Hire }
	if err := json.Unmarshal(frontDeskFixture(t, "registry.json"), &registry); err != nil {
		t.Fatal(err)
	}
	var team struct {
		Answer any
		Input  struct {
			Live []struct {
				ID, Agent, Folder, Project, State, Model string
				Floor, Desk                              *int
				LastEventAt                              float64
			}
			Now    float64
			Status map[string]string
		}
	}
	if err := json.Unmarshal(frontDeskFixture(t, "team.json"), &team); err != nil {
		t.Fatal(err)
	}
	var live []liveSession
	for _, l := range team.Input.Live {
		live = append(live, liveSession{l.ID, l.Agent, l.Folder, l.Project, l.State, l.Model, l.Floor, l.Desk, l.LastEventAt})
	}
	rows := buildTeam(registry.Hires, live, team.Input.Now, func(p string) string { return team.Input.Status[p] })
	want := team.Answer.(map[string]any)["team"]
	if got := asJSONValue(t, rows); !reflect.DeepEqual(got, want) {
		g, _ := json.MarshalIndent(got, "", " ")
		w, _ := json.MarshalIndent(want, "", " ")
		t.Fatalf("team rows differ from team.json\n got: %s\nwant: %s", g, w)
	}
}

func TestTheTeamRouteAnswersFromTheRegistryFileForThePageAndAToolAlikeAndNobodyWithoutAKey(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	if err := writeRegistryFile(registryPath(), frontDeskFixture(t, "registry.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	get := func(c call) *httptest.ResponseRecorder {
		c.method, c.target, c.host, c.from = "GET", "/setup/gm/team", "localhost:7710", thisComputer
		return serve(t, d, c)
	}
	if got := get(call{withoutSetupKey: true}).Code; got != http.StatusUnauthorized {
		t.Fatalf("with no key: %d", got)
	}
	for _, c := range []call{{asTool: true}, {}} {
		w := get(c)
		var reply struct {
			At       int64
			Team     []map[string]any
			Contacts []any
		}
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &reply) != nil {
			t.Fatalf("asTool=%v: %d %s", c.asTool, w.Code, w.Body)
		}
		if len(reply.Team) != 3 || reply.At == 0 || reply.Contacts == nil || len(reply.Contacts) != 0 {
			t.Fatalf("asTool=%v: unexpected team %s", c.asTool, w.Body)
		}
		for _, row := range reply.Team {
			if row["registered"] != true || row["live"] != false || row["liveState"] != "gone" || row["floor"] != nil {
				t.Fatalf("a hire with no session should be gone: %v", row)
			}
		}
	}
}

func TestARegistryEditAnswersAndRefusesAsRegistryEditJSONSays(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	original := frontDeskFixture(t, "registry.json")
	if err := writeRegistryFile(registryPath(), original, 0o600); err != nil {
		t.Fatal(err)
	}
	var edit struct {
		Answer   any
		Request  any
		Refusals []struct {
			Request any
			Status  int
		}
	}
	if err := json.Unmarshal(frontDeskFixture(t, "registry-edit.json"), &edit); err != nil {
		t.Fatal(err)
	}
	post := func(c call, request any) *httptest.ResponseRecorder {
		body, _ := json.Marshal(request)
		c.method, c.target, c.host, c.from, c.body = "POST", "/setup/gm/registry", "localhost:7710", thisComputer, string(body)
		c.headers = map[string]string{setupHeader: "1"}
		return serve(t, d, c)
	}
	// A tool, with only the setup key, may not edit: nothing changes.
	if w := post(call{asTool: true}, edit.Request); w.Code != http.StatusForbidden {
		t.Fatalf("a tool's edit: %d, want 403", w.Code)
	}
	for _, field := range []string{"nickname", "notes"} {
		id := edit.Request.(map[string]any)["id"]
		if w := post(call{}, map[string]any{"id": id, field: nil}); w.Code != http.StatusBadRequest {
			t.Errorf("a null %s for a real hire: %d, want 400", field, w.Code)
		}
	}
	for _, r := range edit.Refusals {
		if w := post(call{}, r.Request); w.Code != r.Status {
			t.Errorf("refusal %.80v: %d, want %d", r.Request, w.Code, r.Status)
		}
	}
	if now, _ := os.ReadFile(registryPath()); string(now) != string(original) {
		t.Fatal("a refused edit changed the registry")
	}
	w := post(call{}, edit.Request)
	var got any
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &got) != nil || !reflect.DeepEqual(got, edit.Answer) {
		t.Fatalf("the edit answered %d %s, want %v", w.Code, w.Body, edit.Answer)
	}
	var saved struct{ Hires []Hire }
	data, _ := os.ReadFile(registryPath())
	if json.Unmarshal(data, &saved) != nil || len(saved.Hires) != 3 || saved.Hires[1].Nickname != "Mop" || saved.Hires[1].State != "working" ||
		saved.Hires[0].Nickname != "Rev" || !strings.Contains(string(data), `"version": 1`) {
		t.Fatalf("the edit wasn't kept as it should: %s", data)
	}
}

func TestAGetWithABrokenRegistryLeavesTheFileWhereItIs(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	broken := []byte(`{"version": 99, "hires": []}`)
	if err := writeRegistryFile(registryPath(), broken, 0o600); err != nil {
		t.Fatal(err)
	}
	w := serve(t, d, call{method: "GET", target: "/setup/gm/team", host: "localhost:7710", from: thisComputer})
	var reply struct{ Team []any }
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &reply) != nil {
		t.Fatalf("a broken registry: %d %s", w.Code, w.Body)
	}
	if now, err := os.ReadFile(registryPath()); err != nil || string(now) != string(broken) {
		t.Fatalf("a read moved or changed the registry: %v %s", err, now)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(registryPath()), "registry.broken.json")); err == nil {
		t.Fatal("a read set the registry aside")
	}
}
