//go:build windows

package desktop

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/gm"
)

// On Windows (the CI's windows-latest), in-process: a terminal's ask goes over a real loopback connection to the app,
// Windows' own tables name the program that asked (this test binary, which holds the connection) in Smolcorp's own
// window, and the window's Allow gives a tool's key that reads Setup's state; then, with "Allow a General Manager
// started by hand" on, `smolcorp gm login`'s ask gives the GM a token that reads the team. No window opens: the
// window's session is opened as setup.html opens it there, and the owner's Allow is sent with it.
func TestOnWindowsAnAskGoesThroughToAToolKeyAndAGMLogin(t *testing.T) {
	d, _ := tapDeck(t)
	onWindowsWithWebView2(t)
	managerOn(d)
	window, opened := openWindowSession(t, d, d.newSettingsRun(""))
	if opened != http.StatusNoContent {
		t.Fatalf("the window's session: %d", opened)
	}
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	owner := ownerAtTheWindow(t, d, server, window, "allow")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	var said strings.Builder
	key, err := gm.AskForKey(cliEnv(server, nil), gm.AskTool, `C:\work\baguette`, &said)
	if err != nil || key == "" {
		t.Fatalf("the tool's ask: %v\n%s", err, said.String())
	}
	asks := owner.asks()
	if len(asks) == 0 {
		t.Fatal("the window was shown no ask")
	}
	program, _ := asks[0]["program"].(string)
	t.Logf("the window showed: program %q, started from %v, folder %q, code %v", program, asks[0]["startedFrom"], asks[0]["folder"], asks[0]["code"])
	if !strings.EqualFold(filepath.Clean(program), filepath.Clean(self)) {
		t.Errorf("Windows named %q as the program that asked, want this test %q", program, self)
	}
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/setup/state", nil)
	request.Header.Set(setupHeader, "1")
	request.Header.Set(setupKeyHeader, key)
	response, err := server.Client().Do(request)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("/setup/state with the tool's key: %v %v", err, response)
	}
	response.Body.Close()

	if got := withSession(t, d, window, "POST", "/setup/gm/by-hand?enabled=1", ""); got.Code != http.StatusOK {
		t.Fatalf("the window turns on a GM started by hand: %d %q", got.Code, got.Body)
	}
	said.Reset()
	token, err := gm.AskForKey(cliEnv(server, nil), gm.AskGM, `C:\work\gm`, &said)
	if err != nil || !worksAsGM(d, token) {
		t.Fatalf("gm login's ask: %v, works %v\n%s", err, worksAsGM(d, token), said.String())
	}
	c, err := gm.NewClient(cliEnv(server, map[string]string{"SMOLCORP_GM_TOKEN": token}))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := gm.Team(c); err != nil {
		t.Errorf("gm team with the token: %v", err)
	}
}
