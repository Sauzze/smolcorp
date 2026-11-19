package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Tests/fixtures/briefs/briefs.json is what the Mac app made of a fixed set
// of inputs, back when its own code made briefs (nothing writes it any more):
// hires' briefs, setup's previews, picked files, a brief's later parts, and
// the guard's state and saves. This side must make the same, byte for byte. Paths in it start with
// ~, for each side's home; dates are UTC. A hire marked "normalized": "NFC"
// is compared with the Mac's brief made precomposed: the Mac decomposes the
// paths it builds with URL, and this side keeps them as given (GO-PARITY.md).

type briefsFixture struct {
	Entries []briefEntry `json:"entries"`
	Hires   []struct {
		Name    string   `json:"name"`
		Role    Role     `json:"role"`
		Folder  string   `json:"folder"`
		Pinned  bool     `json:"pinned"`
		Desk    *int     `json:"desk"`
		Answers []string `json:"answers"`
		Task    string   `json:"task"`
		Managed string   `json:"managed"`
		Kept    string   `json:"kept"`
		Date    float64  `json:"date"`
		Made    *struct {
			Brief, Status, Report, Markdown, FirstMessage string
		} `json:"made"`
	} `json:"hires"`
	Library []struct {
		ID           string `json:"id"`
		Role         Role   `json:"role"`
		Template     string `json:"template"`
		Markdown     string `json:"markdown"`
		FirstMessage string `json:"firstMessage"`
	} `json:"library"`
	Previews []struct {
		Body   string `json:"body"`
		Status int    `json:"status"`
		Answer any    `json:"answer"`
	} `json:"previews"`
	Files []struct {
		Text    string `json:"text"`
		Name    string `json:"name"`
		Kept    *struct{ Text, FileName string }
		Problem string `json:"problem"`
	} `json:"files"`
	Parts []struct {
		Text string  `json:"text"`
		Date float64 `json:"date"`
		Part string  `json:"part"`
		Note string  `json:"note"`
	} `json:"parts"`
	Guard []struct {
		File  *string        `json:"file"`
		State map[string]any `json:"state"`
	} `json:"guard"`
	GuardSaves []struct {
		File          *string `json:"file"`
		MissingFolder bool    `json:"missingFolder"`
		Body          string  `json:"body"`
		Status        int     `json:"status"`
		Answer        any     `json:"answer"`
		After         any     `json:"after"`
	} `json:"guardSaves"`
}

// readBriefsFixture reads the fixture, with home standing in for this side's
// home folder until the test ends. A test of rows that hold paths is skipped
// on Windows, as the Mac's are written with /; the rest run there too.
func readBriefsFixture(t *testing.T, holdsPaths bool) briefsFixture {
	t.Helper()
	if holdsPaths && runtime.GOOS == "windows" {
		t.Skip("the Mac's paths are written with /")
	}
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "briefs", "briefs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture briefsFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	saved := homeDir
	homeDir = "/fixture-home"
	t.Cleanup(func() { homeDir = saved })
	return fixture
}

func fixtureDate(seconds float64) time.Time { return time.Unix(int64(seconds), 0).UTC() }

func sha256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func TestEveryHiresBriefIsTheMacsByteForByte(t *testing.T) {
	fixture := readBriefsFixture(t, true)
	library := briefLibrary{files: bundledBriefs()}
	if len(fixture.Hires) < 10 {
		t.Fatalf("only %d hires in the fixture", len(fixture.Hires))
	}
	for _, hire := range fixture.Hires {
		made, err := makeBrief(briefRequest{role: hire.Role, library: library, folder: expandTilde(hire.Folder, homeDir), pinned: hire.Pinned,
			desk: hire.Desk, answers: hire.Answers, task: hire.Task, managed: map[bool]string{true: expandTilde(hire.Managed, homeDir)}[hire.Managed != ""],
			date: fixtureDate(hire.Date), kept: expandTilde(hire.Kept, homeDir)})
		if err != nil {
			t.Fatalf("%s: %v", hire.Name, err)
		}
		if (made == nil) != (hire.Made == nil) {
			t.Fatalf("%s: made %v, the Mac %v", hire.Name, made != nil, hire.Made != nil)
		}
		if made == nil {
			continue
		}
		got := map[string]string{"brief": tildePath(made.names.brief), "status": tildePath(made.names.status), "report": tildePath(made.names.report),
			"markdown": made.markdown, "firstMessage": made.firstMessage}
		want := map[string]string{"brief": hire.Made.Brief, "status": hire.Made.Status, "report": hire.Made.Report,
			"markdown": hire.Made.Markdown, "firstMessage": hire.Made.FirstMessage}
		for key := range want {
			if got[key] != want[key] {
				t.Errorf("%s: %s differs from the Mac's\n--- here\n%q\n--- the Mac\n%q", hire.Name, key, got[key], want[key])
			}
		}
	}
}

func TestEveryLibraryBriefIsTheMacsByteForByte(t *testing.T) {
	fixture := readBriefsFixture(t, false)
	library := briefLibrary{files: bundledBriefs()}
	if got, _ := json.Marshal(library.entries()); string(got) != mustJSON(t, fixture.Entries) {
		t.Errorf("the library's entries differ from the Mac's:\n%s\n%s", got, mustJSON(t, fixture.Entries))
	}
	if len(fixture.Library) != len(briefLibraryIDs) {
		t.Fatalf("the fixture has %d library briefs, this side %d", len(fixture.Library), len(briefLibraryIDs))
	}
	for _, brief := range fixture.Library {
		template, ok := library.template(brief.ID)
		if !ok || sha256Hex(template) != brief.Template {
			t.Errorf("%s: the template differs from the Mac's", brief.ID)
		}
		if runtime.GOOS == "windows" {
			continue // the brief made from it names its files, with the Mac's /
		}
		desk := 1
		made, err := makeBrief(briefRequest{role: brief.Role, library: library, folder: expandTilde("~/Projects", homeDir), desk: &desk,
			date: fixtureDate(1_800_000_000), kept: expandTilde("~/Support/briefs", homeDir)})
		if err != nil || made == nil {
			t.Fatalf("%s: %v", brief.ID, err)
		}
		if sha256Hex(made.markdown) != brief.Markdown || made.firstMessage != brief.FirstMessage {
			t.Errorf("%s: the brief differs from the Mac's:\n%s", brief.ID, made.markdown)
		}
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// readBack is value as JSON reads it back, so answers compare as the page reads them.
func readBack(t *testing.T, value any) any {
	t.Helper()
	var back any
	if err := json.Unmarshal([]byte(mustJSON(t, value)), &back); err != nil {
		t.Fatal(err)
	}
	return back
}

func TestSetupsPreviewAnswersAsTheMacDoes(t *testing.T) {
	fixture := readBriefsFixture(t, true)
	roles := []Role{shippedRoles[0], {ID: "plain", Name: "Plain", Agent: "claude", Questions: []string{"First?", "Second?"},
		Instructions: "Just do it.", Brief: &RoleBrief{Kind: "text", Text: "Just do it."}}}
	if roles[0].ID != "reviewer" {
		t.Fatalf("the first shipped role is %s", roles[0].ID)
	}
	in := briefPreviewInput{roles: roles, library: briefLibrary{files: bundledBriefs()}, date: fixtureDate(1_800_000_000),
		kept: expandTilde("~/Support/briefs", homeDir)}
	// The page sends folders with ~, which the route expands with this side's home.
	for _, preview := range fixture.Previews {
		answer := briefPreview([]byte(preview.Body), in)
		if answer.status != preview.Status {
			t.Errorf("%s: %d, the Mac %d (%s)", preview.Body, answer.status, preview.Status, answer.message)
			continue
		}
		var got any = answer.message
		if answer.json != nil {
			got = readBack(t, answer.json)
		}
		if !reflect.DeepEqual(got, preview.Answer) {
			t.Errorf("%s: answered\n%v\nthe Mac\n%v", preview.Body, got, preview.Answer)
		}
	}
}

func TestPickedFilesAreKeptOrRefusedAsTheMacDoes(t *testing.T) {
	fixture := readBriefsFixture(t, false)
	long := regexp.MustCompile(`^x\*(\d+)$`)
	for _, file := range fixture.Files {
		text := file.Text
		if m := long.FindStringSubmatch(text); m != nil {
			n, _ := strconv.Atoi(m[1])
			text = strings.Repeat("x", n)
		}
		kept, err := briefFile(text, file.Name)
		switch {
		case file.Kept != nil && (err != nil || kept.Text != file.Kept.Text || kept.FileName != file.Kept.FileName):
			t.Errorf("%q: kept %q %q (%v), the Mac %q %q", file.Name, kept.Text, kept.FileName, err, file.Kept.Text, file.Kept.FileName)
		case file.Kept == nil && (err == nil || err.Error() != file.Problem):
			t.Errorf("%q: %v, the Mac %q", file.Name, err, file.Problem)
		}
	}
}

func TestABriefsLaterPartIsTheMacs(t *testing.T) {
	fixture := readBriefsFixture(t, false)
	for _, part := range fixture.Parts {
		if got := briefMorePart(part.Text, fixtureDate(part.Date)); got != part.Part {
			t.Errorf("%q: %q, the Mac %q", part.Text, got, part.Part)
		}
		if got := briefMoreNote("~/BRIEF.md"); got != part.Note {
			t.Errorf("note %q, the Mac %q", got, part.Note)
		}
	}
}

func TestTheGuardsStateIsTheMacs(t *testing.T) {
	fixture := readBriefsFixture(t, false)
	for i, file := range fixture.Guard {
		path := filepath.Join(t.TempDir(), "hire-guard.json")
		if file.File != nil {
			if err := os.WriteFile(path, []byte(*file.File), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if got := readBack(t, guardState(path, "darwin")); !reflect.DeepEqual(got, any(file.State)) {
			t.Errorf("file %d: %v\nthe Mac: %v", i, got, file.State)
		}
	}
	// Elsewhere it says computer, where the Mac says Mac.
	state := guardState(filepath.Join(t.TempDir(), "none.json"), "linux")
	local := state["groups"].([]map[string]any)[5]
	if local["name"] != "local" || !strings.Contains(local["about"].(string), "Driving your computer") ||
		!strings.Contains(local["allows"].(string), "drive your computer") {
		t.Errorf("the local group on Linux: %v", local)
	}
}

func TestTheGuardIsSavedAsTheMacSavesIt(t *testing.T) {
	fixture := readBriefsFixture(t, false)
	if len(fixture.GuardSaves) < 30 {
		t.Fatalf("only %d guard saves in the fixture", len(fixture.GuardSaves))
	}
	for i, save := range fixture.GuardSaves {
		path := filepath.Join(t.TempDir(), "hire-guard.json")
		if save.MissingFolder {
			path = filepath.Join(t.TempDir(), "missing", "hire-guard.json")
		}
		if save.File != nil {
			if err := os.WriteFile(path, []byte(*save.File), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		answer := saveGuard([]byte(save.Body), path, "darwin")
		var got any = answer.message
		if answer.json != nil {
			got = readBack(t, answer.json)
		}
		if answer.status != save.Status || !reflect.DeepEqual(got, save.Answer) {
			t.Errorf("save %d (%s): %d %v\nthe Mac: %d %v", i, save.Body, answer.status, got, save.Status, save.Answer)
		}
		// The file it leaves, read as JSON: the same groups, in the same order, as the Mac writes.
		var after any
		if data, err := os.ReadFile(path); err == nil {
			if json.Unmarshal(data, &after) != nil {
				after = string(data)
			}
		}
		if !reflect.DeepEqual(after, save.After) {
			t.Errorf("save %d (%s): the file\n%v\nthe Mac's\n%v", i, save.Body, after, save.After)
		}
	}
}
