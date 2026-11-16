package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The pins: a line for each 1.24 download the job installs, a SHA-256 each, and a download that isn't the one kept,
// or has no line, stops the job before anything of it runs.
func TestTheDownloadMustBeTheOneKept(t *testing.T) {
	sums, err := os.ReadFile("SHA256SUMS")
	if err != nil {
		t.Fatal(err)
	}
	line := regexp.MustCompile(`(?m)^[0-9a-f]{64}  (v1\.24\.[12])/(Smolcorp-(Linux|Windows)-(x86_64|arm64)\.(tar\.gz|zip))$`)
	if got := len(line.FindAll(sums, -1)); got != 8 {
		t.Errorf("SHA256SUMS has %d download lines, not 8 (two versions, four downloads each)", got)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "Smolcorp-Linux-x86_64.tar.gz")
	if err := os.WriteFile(archive, []byte("a stand-in download"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("a stand-in download"))
	kept := []byte(hex.EncodeToString(sum[:]) + "  v1.24.2/Smolcorp-Linux-x86_64.tar.gz\n")
	if err := checkSum(kept, "v1.24.2", archive); err != nil {
		t.Errorf("the download kept: %v", err)
	}
	if err := checkSum(kept, "v1.24.1", archive); err == nil {
		t.Error("a version with no line was taken")
	}
	if err := os.WriteFile(archive, []byte("another download"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkSum(kept, "v1.24.2", archive); err == nil || !strings.Contains(err.Error(), "not the") {
		t.Errorf("a download that isn't the one kept: %v", err)
	}
}

// Both kinds of download unpack into the install's folder without their top folder, and a name that would land
// outside it stops the job.
func TestUnpackPutsTheReleaseInTheInstallFolder(t *testing.T) {
	dir := t.TempDir()
	tarball := func(names ...string) string {
		var data bytes.Buffer
		gz := gzip.NewWriter(&data)
		tw := tar.NewWriter(gz)
		for _, name := range names {
			_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(name)), Typeflag: tar.TypeReg})
			_, _ = tw.Write([]byte(name))
		}
		_ = tw.Close()
		_ = gz.Close()
		path := filepath.Join(dir, "release.tar.gz")
		if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	install := filepath.Join(dir, "linux")
	if err := unpack(tarball("Smolcorp-Linux-x86_64/smolcorp", "Smolcorp-Linux-x86_64/docs/README.md"), install); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(install, "smolcorp")); err != nil || string(data) != "Smolcorp-Linux-x86_64/smolcorp" {
		t.Errorf("the program: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(install, "docs", "README.md")); err != nil {
		t.Error(err)
	}
	if err := unpack(tarball("Smolcorp-Linux-x86_64/../../outside"), filepath.Join(dir, "escape")); err == nil {
		t.Error("a name outside the install was unpacked")
	}

	var data bytes.Buffer
	zw := zip.NewWriter(&data)
	file, _ := zw.Create("Smolcorp-Windows-x86_64/smolcorp-cli.exe")
	_, _ = file.Write([]byte("cli"))
	_ = zw.Close()
	archive := filepath.Join(dir, "release.zip")
	if err := os.WriteFile(archive, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	install = filepath.Join(dir, "windows")
	if err := unpack(archive, install); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(install, "smolcorp-cli.exe")); err != nil || string(data) != "cli" {
		t.Errorf("the Windows program: %q %v", data, err)
	}
}

// snapshotOf is a snapshot from JSON texts.
func snapshotOf(t *testing.T, registry, config, guard string) snapshot {
	t.Helper()
	var s snapshot
	if err := json.Unmarshal([]byte(registry), &s.registry); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(config), &s.config); err != nil {
		t.Fatal(err)
	}
	s.guard = []byte(guard)
	return s
}

const (
	team1242 = `{"version":1,"hires":[{"id":"h1","name":"Builder","nickname":"Ada","role":"builder","folder":"/p","branch":"hire/h1",` +
		`"worktree":"/p/w","repo":"/p","account":"work","state":"working"}],"contacts":[{"from":"h1","to":"h2"}]}`
	settings1242 = `{"colours":{"body":"#102030"},"floors":[{"name":"Lobby","preset":""},{"name":"Lab","preset":"zen"}],"pins":["/p",null],` +
		`"layout":{"keys":"auto"},"roles":[{"id":"builder"}],"listen":"tailscale","hiringEnabled":false,"managerEnabled":false,` +
		`"webhook":{"enabled":false,"url":"http://127.0.0.1:9/hook","outside":false}}`
	guard1242 = `{"groups":[{"name":"deploy","on":false},{"name":"push","on":true}]}`
)

// What carries is caught field by field, and what 1.26 only adds isn't counted against it.
func TestCarriedCatchesWhatDidntCarry(t *testing.T) {
	before := snapshotOf(t, team1242, settings1242, guard1242)
	// 1.26 adds a role, a layout choice, pins up to its count, and fields to a hire's row: all carried.
	same := snapshotOf(t, strings.Replace(team1242, `"state":"working"`, `"state":"working","tmuxProgram":"/x"`, 1),
		strings.NewReplacer(`"roles":[{"id":"builder"}]`, `"roles":[{"id":"builder"},{"id":"minimanager"}]`,
			`"layout":{"keys":"auto"}`, `"layout":{"keys":"auto","dial":true}`, `"pins":["/p",null]`, `"pins":["/p",null,null]`).Replace(settings1242), guard1242)
	if problems := carried(before, same); len(problems) != 0 {
		t.Errorf("what only adds counted as lost: %q", problems)
	}
	for name, after := range map[string]snapshot{
		"an account":    snapshotOf(t, strings.Replace(team1242, `"work"`, `"default"`, 1), settings1242, guard1242),
		"a hire":        snapshotOf(t, `{"hires":[],"contacts":[{"from":"h1","to":"h2"}]}`, settings1242, guard1242),
		"a contact":     snapshotOf(t, strings.Replace(team1242, `{"from":"h1","to":"h2"}`, ``, 1), settings1242, guard1242),
		"a floor":       snapshotOf(t, team1242, strings.Replace(settings1242, `"zen"`, `""`, 1), guard1242),
		"a colour":      snapshotOf(t, team1242, strings.Replace(settings1242, `#102030`, `#0e0e0e`, 1), guard1242),
		"a pin":         snapshotOf(t, team1242, strings.Replace(settings1242, `"pins":["/p",null]`, `"pins":[null,null]`, 1), guard1242),
		"a role":        snapshotOf(t, team1242, strings.Replace(settings1242, `{"id":"builder"}`, `{"id":"tester"}`, 1), guard1242),
		"the webhook":   snapshotOf(t, team1242, strings.Replace(settings1242, `/hook`, `/other`, 1), guard1242),
		"the guard":     snapshotOf(t, team1242, settings1242, strings.Replace(guard1242, `"push","on":true`, `"push","on":false`, 1)),
		"a setting":     snapshotOf(t, team1242, strings.Replace(settings1242, `"listen":"tailscale"`, `"listen":"all"`, 1), guard1242),
		"the GM's work": snapshotOf(t, strings.Replace(team1242, `"state":"working"`, `"state":"failed"`, 1), settings1242, guard1242),
	} {
		if len(carried(before, after)) == 0 {
			t.Errorf("%s that didn't carry wasn't caught", name)
		}
	}
}

// Each way 1.26 could hold a setting looser than 1.24 had it is caught; the same or stricter isn't.
func TestNotLooserCatchesEachLoosening(t *testing.T) {
	before := snapshotOf(t, team1242, settings1242, guard1242)
	if problems := notLooser(before, before); len(problems) != 0 {
		t.Errorf("the same settings: %q", problems)
	}
	stricter := snapshotOf(t, team1242, settings1242, strings.Replace(guard1242, `"deploy","on":false`, `"deploy","on":true`, 1))
	if problems := notLooser(before, stricter); len(problems) != 0 {
		t.Errorf("a stricter guard: %q", problems)
	}
	for name, change := range map[string][2]string{
		"hiring":            {`"hiringEnabled":false`, `"hiringEnabled":true`},
		"the manager":       {`"managerEnabled":false`, `"managerEnabled":true`},
		"T3 Code":           {`"listen"`, `"t3CodeEnabled":true,"listen"`},
		"a GM by hand":      {`"listen"`, `"gmByHand":true,"listen"`},
		"keys to apps":      {`"listen"`, `"keystrokesAllowed":true,"listen"`},
		"apps to type to":   {`"listen"`, `"keystrokeApps":["com.example.notes"],"listen"`},
		"other Macs":        {`"listen"`, `"floorsFromOtherMacs":true,"listen"`},
		"the mic":           {`"listen"`, `"phoneMicSkipsConfirm":true,"listen"`},
		"the mail socket":   {`"listen"`, `"mailInboxSocket":true,"listen"`},
		"typing over Wi-Fi": {`"listen"`, `"phoneTextOverWiFi":true,"listen"`},
		"every interface":   {`"listen":"tailscale"`, `"listen":"all"`},
		"the webhook":       {`"enabled":false`, `"enabled":true`},
		"outside":           {`"outside":false`, `"outside":true`},
		"a guard group":     {"", ""},
		"the webhook's to":  {"", ""},
	} {
		after := snapshotOf(t, team1242, strings.Replace(settings1242, change[0], change[1], 1), guard1242)
		switch name {
		case "a guard group":
			after = snapshotOf(t, team1242, settings1242, strings.Replace(guard1242, `"push","on":true`, `"push","on":false`, 1))
		case "the webhook's to":
			on := strings.Replace(settings1242, `"enabled":false`, `"enabled":true`, 1)
			before := snapshotOf(t, team1242, on, guard1242)
			after = snapshotOf(t, team1242, strings.Replace(on, `/hook`, `/elsewhere`, 1), guard1242)
			if len(notLooser(before, after)) == 0 {
				t.Errorf("%s: not caught", name)
			}
			continue
		}
		if len(notLooser(before, after)) == 0 {
			t.Errorf("%s turned on: not caught", name)
		}
	}
}

// The checklist must say every line of Setup's about the update, and name each hire in the re-hire line itself.
func TestSetupSaysItNeedsEveryLineAndEachHire(t *testing.T) {
	checklist := "\n• " + strings.Join(setupSays[:len(setupSays)-1], " ") + " Re-hire your hires from 1.24 (Ada, Bo and the General Manager): they still run as they started.\n"
	if problems := setupSaysIt(checklist, "Ada", "Bo", "the General Manager"); len(problems) != 0 {
		t.Errorf("all there: %q", problems)
	}
	for _, words := range setupSays {
		if len(setupSaysIt(strings.Replace(checklist, words, "", 1), "Ada")) == 0 {
			t.Errorf("without %q: not caught", words)
		}
	}
	// Bo named elsewhere, not in the re-hire line.
	if problems := setupSaysIt(strings.Replace(checklist, "Ada, Bo and", "Ada and", 1)+"Bo was here.", "Bo"); len(problems) != 1 {
		t.Errorf("a hire missing from the re-hire line: %q", problems)
	}
}

// The strict office is at its strictest as seeded, and the open one opened up: the job's two legs check 1.26 from
// each side. Neither has Typing over Wi-Fi, which 1.24 had no switch for.
func TestEachOfficeIsSeededAsItsLegSays(t *testing.T) {
	for _, strict := range []bool{true, false} {
		dir := t.TempDir()
		o := &office{home: dir, support: dir}
		o.seedFiles(filepath.Join(dir, "project"), strict)
		s := o.snapshot()
		want := map[string]any{"listen": "all", "hiringEnabled": true, "managerEnabled": true}
		if strict {
			want = map[string]any{"listen": "tailscale", "hiringEnabled": false, "managerEnabled": false}
		}
		for name, value := range want {
			if s.config[name] != value {
				t.Errorf("strict %v: %s is %v, want %v", strict, name, s.config[name], value)
			}
		}
		if _, ok := s.config["phoneTextOverWiFi"]; ok {
			t.Errorf("strict %v: the 1.24 office has Typing over Wi-Fi", strict)
		}
		if deployOn := bytes.Contains(s.guard, []byte(`{"commands":[],"name":"deploy","on":true}`)); deployOn != strict {
			t.Errorf("strict %v: deploy's guard on is %v: %s", strict, deployOn, s.guard)
		}
	}
}

// The setup link's token is read from each version's own line, the last one if it printed more.
func TestTheSetupLinksTokenIsRead(t *testing.T) {
	for output, want := range map[string]string{
		"Smolcorp's setup: http://127.0.0.1:7746/setup#o=abc_DEF-1\nIt works once, for five minutes.\n":    "abc_DEF-1",
		"setup: http://127.0.0.1:7746/setup#o=first\nsetup: http://127.0.0.1:7746/setup#o=second&pair\n":   "second",
		"Smolcorp's setup: http://127.0.0.1:7746/setup#tap\nYour phone is paired, so Setup opens by a tap": "",
	} {
		if got := linkToken([]byte(output)); got != want {
			t.Errorf("%q: %q, want %q", output, got, want)
		}
	}
}
