package desktop

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// screenLogFor turns the screen log on for a row, in the support folder the row has set (isolate's), and clears it
// before and after.
func screenLogFor(t *testing.T) (path, old string) {
	t.Helper()
	path, old = filepath.Join(supportDir, screenLogName), filepath.Join(supportDir, screenLogOld)
	clear := func() { _ = os.Remove(path); _ = os.Remove(old) }
	clear()
	was := screenLogOn
	screenLogOn = true
	t.Cleanup(func() { screenLogOn = was; clear() })
	logged, flags := log.Writer(), log.Flags()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(logged); log.SetFlags(flags) })
	return path, old
}

// On Windows, where the app has no log file, what broke a launcher's screen reaches one in the support folder:
// each reason once per launcher, as the log line is said, and never the screen's text. Off Windows nothing is
// written there.
func TestWhatBrokeALaunchersScreenReachesTheSupportFoldersLogOnce(t *testing.T) {
	d := newTestDeck(t)
	path, _ := screenLogFor(t)

	// The launcher's own answer, from a screen holding text, as it posts it.
	var seen []string
	client := &http.Client{Transport: &captureTransport{seen: &seen}}
	postScreenAnswer(client, "http://127.0.0.1:1", "fake-key", "fake-proof", "L1", "1",
		teedScreen(t, 60, 200, "secret plans\x1b[3b", 4096))
	if len(seen) != 1 {
		t.Fatalf("posted %q", seen)
	}
	var answer screenAnswer
	if err := json.Unmarshal([]byte(seen[0]), &answer); err != nil || answer.Broke != "CSI 3b" {
		t.Fatalf("the answer %q: %v", seen[0], err)
	}

	tab := d.launcherConnected("L1", Claude, "/work/api", "", "")
	other := d.launcherConnected("L2", Claude, "/work/web", "", "")
	sevenL := answer
	sevenL.Broke = "CSI ?7l"
	for _, a := range []screenAnswer{answer, answer, sevenL, answer, sevenL} {
		d.launcherScreenAnswered("L1", tab.screenSecret, a)
	}
	d.launcherScreenAnswered("L2", other.screenSecret, answer)
	d.launcherScreenAnswered("L1", "wrong", screenAnswer{Look: "1", State: "unknown", Unknown: true, Broke: "forged"})
	d.launcherScreenAnswered("L1", tab.screenSecret, screenAnswer{Look: "1", State: "idle", Broke: "known"})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	var reasons []string
	for _, line := range lines {
		_, said, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("a line without its time: %q", line)
		}
		reasons = append(reasons, said)
	}
	want := []string{
		`launcher L1's screen can't be read since "CSI 3b"`,
		`launcher L1's screen can't be read since "CSI ?7l"`,
		`launcher L2's screen can't be read since "CSI 3b"`,
	}
	if fmt.Sprint(reasons) != fmt.Sprint(want) {
		t.Errorf("logged %q, want %q", reasons, want)
	}
	if strings.Contains(text, "secret") || strings.Contains(text, "plans") || strings.Contains(text, "forged") {
		t.Errorf("the log holds more than the reasons: %q", text)
	}
	if st, err := os.Stat(path); err != nil || (runtime.GOOS != "windows" && st.Mode().Perm() != 0o600) {
		t.Errorf("the log's mode: %v %v", st, err)
	}

	// Off Windows, as before: the log line only.
	_ = os.Remove(path)
	screenLogOn = false
	d.launcherScreenAnswered("L1", tab.screenSecret, screenAnswer{Look: "1", State: "unknown", Unknown: true, Broke: "CSI 5b"})
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("written with the log off: %v", err)
	}
}

// The screen log holds: past 64 KB it's moved aside, the one before dropped, so neither passes 64 KB. A link or a
// folder at either name is never written through.
func TestTheScreenLogIsCappedAndWritesOnlyAPlainFile(t *testing.T) {
	isolate(t)
	path, old := screenLogFor(t)
	line := strings.Repeat("r", 99) + "\n"
	for i := range 3 * (screenLogMax / len(line)) {
		appendScreenLog(line)
		for _, p := range []string{path, old} {
			if st, err := os.Stat(p); err == nil && st.Size() > screenLogMax {
				t.Fatalf("after %d lines %s is %d bytes", i+1, filepath.Base(p), st.Size())
			}
		}
	}
	for _, p := range []string{path, old} {
		st, err := os.Stat(p)
		if err != nil || st.Size() < screenLogMax-int64(len(line)) {
			t.Errorf("%s: %v %v, want it nearly full", filepath.Base(p), st, err)
		}
	}
	appendScreenLog("last\n")
	if data, _ := os.ReadFile(path); !strings.HasSuffix(string(data), "last\n") {
		t.Errorf("the newest line isn't last in the log")
	}

	// A link at the log's name, or at the old one's when it's full: what it points at stays as it was.
	target := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(target, []byte("kept\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(path)
	_ = os.Remove(old)
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("no link here: %v", err)
	}
	appendScreenLog("through the link\n")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("r", screenLogMax)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, old); err != nil {
		t.Fatal(err)
	}
	appendScreenLog("past the cap\n")
	if data, _ := os.ReadFile(target); string(data) != "kept\n" {
		t.Errorf("written through a link: %q", data)
	}
	if st, err := os.Lstat(old); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link at the old log's name was replaced: %v %v", st, err)
	}
	if st, _ := os.Stat(path); st == nil || st.Size() != screenLogMax {
		t.Errorf("the full log was moved or grew past the cap: %v", st)
	}

	// A folder at the log's name: nothing is written, and nothing fails.
	_ = os.Remove(path)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	appendScreenLog("into a folder\n")
	if entries, _ := os.ReadDir(path); len(entries) != 0 {
		t.Errorf("wrote into the folder: %v", entries)
	}
}
