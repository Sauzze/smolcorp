package desktop

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	t.Cleanup(server.Close) // after the browser is killed: cleanups run last first

	// Opened as the menu opens it: a link with a token that works once.
	d.mu.Lock()
	token := d.access.NewSetupToken()
	d.mu.Unlock()
	// Through the helper (T126CHROMEKILL1): never a window or a system dialog, and killed with all it started however the
	// test ends.
	browser := startHeadlessChrome(t, chrome, "--window-size=1100,700", server.URL+"/setup#o="+token)

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
	case <-browser.ended:
		t.Fatalf("Chrome ended first (%s): %s", browser.why, browser.said())
	case <-time.After(60 * time.Second):
		t.Fatalf("no readings from the page in a minute: %s", browser.said())
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
