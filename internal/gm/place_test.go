package gm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// workApp is the fixtures' folder /work/app as the CLI makes it absolute: itself on Linux and the Mac,
// with the drive's letter on Windows (D:\work\app there).
func workApp(t *testing.T) string {
	t.Helper()
	folder, err := filepath.Abs("/work/app")
	if err != nil {
		t.Fatal(err)
	}
	return folder
}

// workAppJSON is workApp as a JSON string.
func workAppJSON(t *testing.T) string {
	t.Helper()
	data, _ := json.Marshal(workApp(t))
	return string(data)
}

// placeApp answers the team with the fixture, and a pin, an unpin and a nickname with the given status and text.
func placeApp(t *testing.T, status int, text string, nickBody *string) *fakeApp {
	t.Helper()
	return newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/setup/gm/team":
			// The fixture's folders are the Mac's; on Windows its session works in workApp.
			w.Write(bytes.ReplaceAll(teamAnswer(t), []byte(`"/work/app"`), []byte(workAppJSON(t))))
		case r.URL.Path == "/setup/gm/nickname":
			data, _ := io.ReadAll(r.Body)
			if nickBody != nil {
				*nickBody = string(data)
			}
			serve([]byte(text), status)(w, r)
		default:
			serve([]byte(text), status)(w, r)
		}
	})
}

func TestPinSendsTheDeskAndTheFolderWithTheGMsToken(t *testing.T) {
	app := placeApp(t, 204, "", nil)
	code, out, errOut := runGM(app.env(t, withToken()), "pin", "--desk", "3", "--folder", "/work/app")
	if code != 0 || errOut != "" || !strings.Contains(out, "Pinned "+workApp(t)+" to desk 3.") {
		t.Fatalf("exit %d %q %q", code, out, errOut)
	}
	if len(app.reqs) != 1 {
		t.Fatalf("requests: %+v", app.reqs)
	}
	r := app.reqs[0]
	q, _ := url.ParseQuery(r.RawQuery)
	if r.Method != "POST" || r.Path != "/setup/pin" || q.Get("key") != "2" || q.Get("folder") != workApp(t) {
		t.Errorf("request = %+v", r)
	}
	if r.Header.Get("X-AgentDeck-Setup") != "1" || r.Header.Get(tokenHeader) != fakeToken {
		t.Errorf("headers = %v", r.Header)
	}
}

func TestPinWithoutADeskUsesTheDeskItsSessionSitsAt(t *testing.T) {
	app := placeApp(t, 204, "", nil)
	code, out, errOut := runGM(app.env(t, withToken()), "pin", "--folder", "/work/app")
	if code != 0 || !strings.Contains(out, "to desk 1.") {
		t.Fatalf("exit %d %q %q", code, out, errOut)
	}
	if len(app.reqs) != 2 || app.reqs[0].Path != "/setup/gm/team" {
		t.Fatalf("it should ask the team, then pin: %+v", app.reqs)
	}
	if q, _ := url.ParseQuery(app.reqs[1].RawQuery); q.Get("key") != "0" {
		t.Errorf("the fixture's session sits at desk 0 (desk 1 in the office): %v", q)
	}
	// A folder nobody sits in is refused, in words, and nothing is pinned.
	app = placeApp(t, 204, "", nil)
	code, _, errOut = runGM(app.env(t, withToken()), "pin", "--folder", "/work/api")
	if code != 1 || !strings.Contains(errOut, "--desk N") || len(app.reqs) != 1 {
		t.Errorf("exit %d %q, requests %+v", code, errOut, app.reqs)
	}
}

func TestPinDefaultsToTheCurrentFolderMadeAbsolute(t *testing.T) {
	wd, _ := os.Getwd()
	app := placeApp(t, 204, "", nil)
	if code, out, errOut := runGM(app.env(t, withToken()), "pin", "--desk", "2"); code != 0 {
		t.Fatalf("exit %d %q %q", code, out, errOut)
	}
	if q, _ := url.ParseQuery(app.reqs[0].RawQuery); q.Get("folder") != wd || q.Get("key") != "1" {
		t.Errorf("query = %v, want folder %q", q, wd)
	}
	app = placeApp(t, 204, "", nil)
	runGM(app.env(t, withToken()), "pin", "--desk", "2", "--folder", "sub/dir")
	if q, _ := url.ParseQuery(app.reqs[0].RawQuery); q.Get("folder") != filepath.Join(wd, "sub", "dir") {
		t.Errorf("a relative folder should be made absolute: %v", q)
	}
}

func TestPinAndUnpinRefuseABadDeskBeforeAskingTheApp(t *testing.T) {
	for _, args := range [][]string{{"pin", "--desk", "0"}, {"pin", "--desk", "10"}, {"unpin", "--desk", "0"}, {"unpin"}, {"unpin", "--desk", "10"},
		{"pin", "--desk", "1", "extra"}, {"unpin", "--desk", "1", "--folder", "/x"}} {
		app := placeApp(t, 204, "", nil)
		if code, _, _ := runGM(app.env(t, withToken()), args...); code != exitUsage || len(app.reqs) != 0 {
			t.Errorf("%v: exit %d, requests %+v", args, code, app.reqs)
		}
	}
}

func TestUnpinSendsTheDesk(t *testing.T) {
	app := placeApp(t, 204, "", nil)
	code, out, _ := runGM(app.env(t, withToken()), "unpin", "--desk", "4")
	r := app.reqs[0]
	q, _ := url.ParseQuery(r.RawQuery)
	if code != 0 || r.Method != "POST" || r.Path != "/setup/unpin" || q.Get("key") != "3" || q.Has("folder") || !strings.Contains(out, "desk 4") {
		t.Errorf("exit %d %q, request %+v", code, out, r)
	}
	// --json says ok for a 204.
	app = placeApp(t, 204, "", nil)
	if _, out, _ := runGM(app.env(t, withToken()), "unpin", "--desk", "4", "--json"); !strings.Contains(out, `"ok": true`) {
		t.Errorf("--json: %q", out)
	}
}

func TestPinsRefusalComesWhole(t *testing.T) {
	app := placeApp(t, 400, "", nil)
	code, _, errOut := runGM(app.env(t, withToken()), "pin", "--desk", "2", "--folder", "/work/app")
	if code != 1 || !strings.Contains(errOut, "400") {
		t.Errorf("exit %d %q", code, errOut)
	}
}

func TestNicknameBuildsTheRequest(t *testing.T) {
	var body string
	app := placeApp(t, 200, `{"project":"/work/app","nickname":"Ada"}`, &body)
	code, out, errOut := runGM(app.env(t, withToken()), "nickname", "Ada", "--folder", "/work/app") // flags may follow the name
	if code != 0 || errOut != "" || !strings.Contains(out, "Named whoever works in "+workApp(t)+": Ada.") {
		t.Fatalf("exit %d %q %q", code, out, errOut)
	}
	r := app.reqs[0]
	if r.Method != "POST" || r.Path != "/setup/gm/nickname" || r.Header.Get("X-AgentDeck-Setup") != "1" || r.Header.Get(tokenHeader) != fakeToken {
		t.Errorf("request = %+v", r)
	}
	var got map[string]any
	json.Unmarshal([]byte(body), &got)
	if !reflect.DeepEqual(got, map[string]any{"folder": workApp(t), "name": "Ada"}) {
		t.Errorf("body = %s", body)
	}
	// An empty name takes the nickname away; the name is trimmed.
	app = placeApp(t, 200, `{"project":"/work/app","nickname":""}`, &body)
	code, out, _ = runGM(app.env(t, withToken()), "nickname", "", "--folder", "/work/app")
	if code != 0 || !strings.Contains(out, "away") || !strings.Contains(body, `"name":""`) {
		t.Errorf("empty: exit %d %q %s", code, out, body)
	}
	app = placeApp(t, 200, `{}`, &body)
	runGM(app.env(t, withToken()), "nickname", "  Bo  ", "--folder", "/work/app")
	if !strings.Contains(body, `"name":"Bo"`) {
		t.Errorf("trimmed: %s", body)
	}
	// --json passes the app's answer on.
	app = placeApp(t, 200, `{"project":"/work/app","nickname":"Ada"}`, &body)
	if _, out, _ := runGM(app.env(t, withToken()), "nickname", "Ada", "--folder", "/work/app", "--json"); !strings.Contains(out, `"nickname": "Ada"`) {
		t.Errorf("--json: %q", out)
	}
}

func TestNicknameDefaultsToTheCurrentFolder(t *testing.T) {
	wd, _ := os.Getwd()
	var body string
	app := placeApp(t, 200, `{}`, &body)
	runGM(app.env(t, withToken()), "nickname", "Ada")
	var got map[string]string
	json.Unmarshal([]byte(body), &got)
	if got["folder"] != wd {
		t.Errorf("folder = %q, want %q", got["folder"], wd)
	}
}

func TestNicknameRefusesBeforeAskingTheApp(t *testing.T) {
	for _, args := range [][]string{{"nickname"}, {"nickname", strings.Repeat("x", 17)}, {"nickname", "a\nb"}, {"nickname", "Ada", "extra"}, {"nickname", "Ada", "--desk", "2"}} {
		app := placeApp(t, 200, `{}`, nil)
		if code, _, _ := runGM(app.env(t, withToken()), args...); code != exitUsage || len(app.reqs) != 0 {
			t.Errorf("%v: exit %d, requests %+v", args, code, app.reqs)
		}
	}
	// 16 characters is the most, counted as characters.
	app := placeApp(t, 200, `{}`, nil)
	if code, _, _ := runGM(app.env(t, withToken()), "nickname", strings.Repeat("é", 16)); code != 0 {
		t.Errorf("16 characters: exit %d", code)
	}
}

func TestNicknameShowsTheAppsRefusalWhole(t *testing.T) {
	why := "Nobody works in that folder and nothing is pinned to it, so there is no one to name."
	app := placeApp(t, 404, why, nil)
	code, _, errOut := runGM(app.env(t, withToken()), "nickname", "Ada", "--folder", "/work/nobody")
	if code != 1 || !strings.Contains(errOut, why) {
		t.Errorf("exit %d %q", code, errOut)
	}
}

// A hire never acts as the GM: these commands are the GM's.
func TestAHireCantPinOrNameAnyone(t *testing.T) {
	for _, args := range [][]string{{"pin", "--desk", "1"}, {"unpin", "--desk", "1"}, {"nickname", "Ada"}} {
		app := placeApp(t, 204, "", nil)
		code, _, errOut := runGM(app.env(t, map[string]string{hireKeyEnv: "fakehirekey"}), args...)
		if code != 1 || !strings.Contains(errOut, "for the GM") || len(app.reqs) != 0 {
			t.Errorf("%v: exit %d %q, requests %+v", args, code, errOut, app.reqs)
		}
	}
}

func TestTheHelpListsThePlaceCommands(t *testing.T) {
	_, out, _ := runGM(Env{Getenv: func(string) string { return "" }}, "help")
	for _, want := range []string{"pin [--desk N] [--folder F]", "unpin --desk N", "nickname <name> [--folder F]"} {
		if !strings.Contains(out, want) {
			t.Errorf("help lacks %q", want)
		}
	}
}

// The CLI says desks as the office numbers them, from 1, while the app's JSON counts from 0.
func TestTheCLIShowsDesksFromOne(t *testing.T) {
	zero, two := 0, 2
	for _, c := range []struct {
		m    Member
		want string
	}{
		{Member{Desk: &zero}, "1"},
		{Member{Desk: &two}, "3"},
		{Member{Desk: &zero, Floor: &two}, "2/1"},
		{Member{}, "-"},
	} {
		if got := deskLabel(c.m); got != c.want {
			t.Errorf("deskLabel(%+v) = %q, want %q", c.m, got, c.want)
		}
	}
	if got := deskNote([]byte(`{"desk":0}`)); got != " At desk 1." {
		t.Errorf("deskNote = %q", got)
	}
}

func TestNicknameNameAfterTheFlagsSaysPutTheNameFirst(t *testing.T) {
	app := placeApp(t, 200, `{}`, nil)
	code, _, errOut := runGM(app.env(t, withToken()), "nickname", "--folder", "/work/app", "Ada")
	if code != exitUsage || !strings.Contains(errOut, "name first") || len(app.reqs) != 0 {
		t.Errorf("exit %d %q, requests %+v", code, errOut, app.reqs)
	}
}
