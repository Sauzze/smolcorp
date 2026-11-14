package desktop

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// T126SCROLL1: scrolled down the setup page, you stay where you are while it refreshes. Each refresh drew
// the Manager panel again by hiding the team and showing it once the team answered: for that moment the
// page was shorter, and the browser scrolled up to fit it. In a headless Chrome, refreshing every 200 ms
// (the page's test hook), with a team that takes a moment to answer, as a real one with sessions does:
// Hires and roles is scrolled to its end, and where it is is read at every frame through two refreshes.
func TestTheSetupPageStaysWhereYouScrolledThroughItsRefreshes(t *testing.T) {
	chrome := headlessChrome()
	if chrome == "" {
		t.Skip("no Chrome or Chromium on this computer (SMOLCORP_TEST_CHROME names one)")
	}
	d := newTestDeck(t)
	d.config.GuideDone = true
	if on := serve(t, d, call{method: "POST", target: "/setup/gm/manager?enabled=1", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}}); on.Code != http.StatusOK {
		t.Fatalf("turning the manager on: %d %s", on.Code, on.Body)
	}

	// Before the page's own script: the refresh's test hook, and the reader. Once the team is shown, it opens
	// Hires and roles, scrolls to its end, and reads scrollY at every frame until two more refreshes have
	// had the team's answer; then it posts what it read.
	page := `<script>
window.smolcorpSetupRefresh = 200;
(() => {
  const teamAnswers = () => performance.getEntriesByType('resource').filter((e) => e.name.endsWith('/setup/gm/team')).length;
  const report = (r) => fetch('/test-readings', { method: 'POST', body: JSON.stringify(r) });
  const started = performance.now();
  function waitForTeam() {
    const tools = document.getElementById('manager-tools');
    if (tools && !tools.hidden) return scroll();
    if (performance.now() - started > 20000) return report({ error: document.body.dataset.locked ? 'the page is locked' : 'the team never showed' });
    setTimeout(waitForTeam, 50);
  }
  function scroll() {
    document.querySelector('#nav [data-go="team"]').click();
    scrollTo(0, document.documentElement.scrollHeight);
    const at = Math.round(scrollY), from = teamAnswers(), read = [];
    let lowest = document.documentElement.scrollHeight;
    (function frame() {
      read.push(Math.round(scrollY));
      lowest = Math.min(lowest, document.documentElement.scrollHeight);
      if (teamAnswers() >= from + 2 && read.length > 10) return report({ at, read, refreshes: teamAnswers() - from, lowest, height: document.documentElement.scrollHeight });
      requestAnimationFrame(frame);
    })();
  }
  addEventListener('load', waitForTeam);
})();
</script>`
	readings := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/test-readings":
			body, _ := io.ReadAll(r.Body)
			select {
			case readings <- body:
			default:
			}
			return
		case "/setup/gm/team":
			time.Sleep(120 * time.Millisecond)
		case "/setup":
			got := httptest.NewRecorder()
			(&router{deck: d}).ServeHTTP(got, r)
			for name, values := range got.Header() {
				w.Header()[name] = values
			}
			w.WriteHeader(got.Code)
			w.Write(bytes.Replace(got.Body.Bytes(), []byte("<head>"), []byte("<head>"+page), 1))
			return
		}
		(&router{deck: d}).ServeHTTP(w, r)
	}))
	defer server.Close()

	// Opened as the menu opens it: a link with a token that works once.
	d.mu.Lock()
	token := d.access.NewSetupToken()
	d.mu.Unlock()
	dir := t.TempDir()
	// Never a window, the Keychain or any other system dialog (_rules.md): headless, a mock keychain, a profile of its own.
	args := []string{"--headless=new", "--use-mock-keychain", "--password-store=basic", "--disable-features=DialMediaRouteProvider",
		"--no-first-run", "--no-default-browser-check", "--user-data-dir=" + filepath.Join(dir, "profile"), "--window-size=1100,700"}
	if runtime.GOOS == "linux" {
		// A CI runner may not give Chrome its sandbox; the page is the test's own, on 127.0.0.1.
		args = append(args, "--no-sandbox")
	}
	browser := exec.Command(chrome, append(args, server.URL+"/setup#o="+token)...)
	browser.Env = append(os.Environ(), "HOME="+dir)
	var said bytes.Buffer
	browser.Stdout, browser.Stderr = &said, &said
	if err := browser.Start(); err != nil {
		t.Fatalf("starting %s: %v", chrome, err)
	}
	ended := make(chan error, 1)
	go func() { ended <- browser.Wait() }()
	stopped := false
	defer func() {
		if !stopped {
			browser.Process.Kill()
			<-ended
		}
	}()

	var got struct {
		Error          string
		At, Refreshes  int
		Read           []int
		Lowest, Height int
	}
	select {
	case body := <-readings:
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("the page's readings: %v %s", err, body)
		}
	case err := <-ended:
		stopped = true
		t.Fatalf("Chrome ended first (%v): %s", err, said.String())
	case <-time.After(60 * time.Second):
		t.Fatalf("no readings from the page in a minute: %s", said.String())
	}
	if got.Error != "" {
		t.Fatal(got.Error)
	}
	if got.At <= 0 || got.Refreshes < 2 {
		t.Fatalf("scrolled to %d, through %d refreshes: the page wasn't scrolled or didn't refresh", got.At, got.Refreshes)
	}
	for i, y := range got.Read {
		if y != got.At {
			t.Errorf("scrolled to %d, the page was at %d %d frames later (it got as short as %d px of %d); every frame read: %v",
				got.At, y, i, got.Lowest, got.Height, got.Read)
			break
		}
	}
}

// headlessChrome is a Chrome or Chromium to run headless: SMOLCORP_TEST_CHROME, or one where it's usually installed.
func headlessChrome() string {
	if named := os.Getenv("SMOLCORP_TEST_CHROME"); named != "" {
		return named
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if found, err := exec.LookPath(name); err == nil {
			return found
		}
	}
	paths := []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium"}
	for _, folder := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
		if at := os.Getenv(folder); at != "" {
			paths = append(paths, filepath.Join(at, "Google", "Chrome", "Application", "chrome.exe"))
		}
	}
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}
