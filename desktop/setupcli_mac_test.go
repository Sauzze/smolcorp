package desktop

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMacBundleIdentifierReadsTheBundle(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Smolcorp.app")
	bin := filepath.Join(app, "Contents", "Resources", "smolcorp")
	os.MkdirAll(filepath.Dir(bin), 0o755)
	os.WriteFile(bin, nil, 0o755)
	os.WriteFile(filepath.Join(app, "Contents", "Info.plist"),
		[]byte("<dict><key>CFBundleIdentifier</key>\n  <string>test.fake.id</string></dict>"), 0o644)
	if got := macBundleIdentifier(bin); got != "test.fake.id" {
		t.Fatalf("got %q", got)
	}
	if got := macBundleIdentifier(filepath.Join(t.TempDir(), "smolcorp")); got != macBundleID {
		t.Fatalf("outside a bundle: got %q", got)
	}
}

func TestStartAppOnMacOpensTheBundle(t *testing.T) {
	oldSystem, oldStart, oldExe := setupSystem, startProgramWith, executable
	defer func() { setupSystem, startProgramWith, executable = oldSystem, oldStart, oldExe }()
	setupSystem = "darwin"
	executable = func() (string, error) { return "/nowhere/smolcorp", nil }
	var program string
	var args []string
	startProgramWith = func(p string, a []string, _ ...string) error { program, args = p, a; return nil }
	if err := startApp(); err != nil {
		t.Fatal(err)
	}
	if program != "/usr/bin/open" || strings.Join(args, " ") != "-b "+macBundleID {
		t.Fatalf("started %s %v", program, args)
	}
}

// printedBy runs f and returns what it printed.
func printedBy(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	f()
	os.Stdout = old
	w.Close()
	said, _ := io.ReadAll(r)
	return string(said)
}

func macSetupTest(t *testing.T) {
	t.Helper()
	isolate(t)
	oldSystem, oldStart, oldExe, oldPort, oldWait, oldMay := setupSystem, startProgramWith, executable, Port, setupStartWait, setupMayOpenApp
	t.Cleanup(func() {
		setupSystem, startProgramWith, executable, Port, setupStartWait, setupMayOpenApp = oldSystem, oldStart, oldExe, oldPort, oldWait, oldMay
	})
	setupSystem = "darwin"
	executable = func() (string, error) { return "/nowhere/smolcorp", nil }
	Port = freePort(t)
	setupStartWait = time.Second
	// The tests can't bind the default port, so they stand in for "aimed at it".
	setupMayOpenApp = func() bool { return true }
}

func TestSetupOnMacOpensAClosedAppOnceThenChecks(t *testing.T) {
	macSetupTest(t)
	writeFile(t, selftestKeyFile, "fake-key\n")
	var opened int
	var server *http.Server
	startProgramWith = func(p string, a []string, _ ...string) error {
		opened++
		if p != "/usr/bin/open" || strings.Join(a, " ") != "-b "+macBundleID {
			t.Errorf("started %s %v", p, a)
		}
		l, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", Port))
		if err != nil {
			return err
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/pair", func(http.ResponseWriter, *http.Request) {})
		mux.HandleFunc("/setup/selftest", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(selftestKeyHeader) != "fake-key" || r.Header.Get(setupKeyHeader) != "" {
				http.Error(w, "no", http.StatusForbidden)
				return
			}
			w.Write([]byte(`{"checks":[{"title":"Everything","status":"ok","detail":"fine"}],"ready":true}`))
		})
		server = &http.Server{Handler: mux}
		go server.Serve(l)
		return nil
	}
	var code int
	said := printedBy(t, func() { code = RunSetup(nil) })
	if server != nil {
		server.Close()
	}
	if opened != 1 {
		t.Fatalf("the app was opened %d times, want once", opened)
	}
	if code != 0 || !strings.Contains(said, "Everything") {
		t.Errorf("code %d, the checklist should follow: %q", code, said)
	}
}

func TestSetupOnMacNeverOpensTheAppForAnotherPort(t *testing.T) {
	macSetupTest(t)
	setupMayOpenApp = setupAimedAtDefaultPort // the real check; Port is a free one, not 7710
	startProgramWith = func(string, []string, ...string) error { t.Error("the app was opened for a test port"); return nil }
	var code int
	said := printedBy(t, func() { code = RunSetup(nil) })
	if code != 1 || !strings.Contains(said, "Applications or Spotlight") {
		t.Errorf("code %d: %q", code, said)
	}
}

func TestSetupOnMacSaysSoWhenTheAppStaysClosed(t *testing.T) {
	macSetupTest(t)
	setupStartWait = 600 * time.Millisecond
	var opened int
	startProgramWith = func(string, []string, ...string) error { opened++; return nil }
	var code int
	said := printedBy(t, func() { code = RunSetup(nil) })
	if opened != 1 || code != 1 {
		t.Fatalf("opened %d, code %d", opened, code)
	}
	if !strings.Contains(said, "Applications or Spotlight") || strings.Contains(said, "without --check") {
		t.Errorf("on a Mac, setup must say to open the app, not to run setup again: %q", said)
	}
}

// smolcorp setup only checks, on every system (1.26, SK2): it holds the check-only key, and what the checklist finds is
// put right in Setup.
func TestSetupNeverRunsTheFixLoopOnAnySystem(t *testing.T) {
	for _, system := range []string{"darwin", "linux", "windows"} {
		t.Run(system, func(t *testing.T) { setupNeverFixes(t, system) })
	}
}

func setupNeverFixes(t *testing.T, system string) {
	macSetupTest(t)
	setupSystem = system
	writeFile(t, selftestKeyFile, "fake-key\n")
	var posted bool
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		w.Write([]byte(`{"checks":[{"title":"Install","status":"fix","fix":"install","detail":"x"}],"ready":false}`))
	}))
	srv.Listener.Close()
	l, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", Port))
	if err != nil {
		t.Fatal(err)
	}
	srv.Listener = l
	srv.Start()
	defer srv.Close()
	startProgramWith = func(string, []string, ...string) error { t.Error("an app that answers was opened"); return nil }
	var code int
	said := printedBy(t, func() { code = RunSetup(nil) })
	if posted || code != 1 || !strings.Contains(said, "This only checks, and changes nothing.") {
		t.Errorf("posted %v, code %d: %q", posted, code, said)
	}
}

func TestSupportFolderNames(t *testing.T) {
	for goos, want := range map[string]string{"darwin": "AgentDeck", "linux": "sauzzedeck-pro", "windows": "SauzzeDeck Pro"} {
		if got := supportDirName(goos); got != want {
			t.Errorf("%s: got %q, want %q", goos, got, want)
		}
	}
}

// The Mac's menu item is "Open Settings in Browser" in the menu bar, not "Open Setup…" in a tray: the command says so there,
// and keeps the tray words on Linux and Windows.
func TestSetupCommandNamesTheMacsOwnMenuItem(t *testing.T) {
	defer func(was string) { setupSystem = was }(setupSystem)
	ready := []check{{Title: "Everything", Status: checkOK, Detail: "fine"}}
	for system, want := range map[string]string{"darwin": "Open Settings in Browser", "linux": "Open Setup…", "windows": "Open Setup…"} {
		setupSystem = system
		var list, open strings.Builder
		printChecklist(&list, ready)
		setupClient{}.openSetup(&open)
		for name, said := range map[string]string{"checklist": list.String(), "open": open.String()} {
			if !strings.Contains(said, want) {
				t.Errorf("%s on %s: %q lacks %q", name, system, said, want)
			}
			if system == "darwin" && (strings.Contains(said, "Open Setup") || strings.Contains(said, "tray")) {
				t.Errorf("%s on the Mac: %q uses the tray's words", name, said)
			}
			if system != "darwin" && strings.Contains(said, "Open Settings in Browser") {
				t.Errorf("%s on %s: %q uses the Mac's words", name, system, said)
			}
		}
	}
}
