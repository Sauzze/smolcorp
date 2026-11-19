package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXRC2FIXB (RT126SWEEP4-P4 F1, claims check): the GM asking to move the folder for hires is told the hires' sandbox
// holds it only where hires start in one, a Mac with the sandbox on. With the owner's switch off, and on Windows and
// Linux, where no hire has a sandbox yet, it gets the plain words.
func TestTheGMIsToldOfAHiresSandboxOnlyWhereHiresHaveOne(t *testing.T) {
	d := newTestDeck(t)
	gm := d.newGMToken()
	elsewhere := filepath.Join(homeDir, "Elsewhere")
	if err := os.MkdirAll(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	asGM := func() (int, string) {
		w := serve(t, d, call{method: "POST", target: "/setup/hiring?folder=" + url.QueryEscape(elsewhere), host: "localhost:7710",
			from: thisComputer, withoutSetupKey: true, headers: map[string]string{gmTokenHeader: gm, setupHeader: "1"}})
		return w.Code, strings.TrimSpace(w.Body.String())
	}
	sandboxOff := func(off bool) {
		d.mu.Lock()
		d.config.SandboxOff = off
		d.mu.Unlock()
	}
	for _, c := range []struct {
		name string
		off  bool
		want string
	}{
		{"the sandbox on", false, map[bool]string{true: sweepFolderPageOnly, false: sweepFolderOwnersOnly}[deskSandboxed]},
		{"the owner's switch off", true, sweepFolderOwnersOnly},
	} {
		sandboxOff(c.off)
		if code, text := asGM(); code != http.StatusForbidden || text != c.want {
			t.Errorf("%s: %d %q, want 403 %q", c.name, code, text, c.want)
		}
	}
	sandboxOff(false)
	if gmHireFolderWords(false) != sweepFolderOwnersOnly || strings.Contains(gmHireFolderWords(false), "sandbox") {
		t.Errorf("without a sandbox: %q", gmHireFolderWords(false))
	}
}

// FXRC2FIXB (RT126SWEEP4-P4 F2, F3, F4, F6): the release notes and the team's guide say the General Manager's own
// worktree stays beside a repository it's hired into, that only a managed hire from Setup or from the GM tells you
// where its work is (FXRC3FIX, RT126RC3's F4: Setup's plain way, at a pinned desk, doesn't, though it's by role too:
// FXRC3FIXR1, RT126RC3FIX's F-a), and that the GM's plans folder may hold a git folder it moved in; GO-PARITY says
// the GM's route refuses T3. Compared with the lines joined, as the guide wraps them.
func TestTheDocsSayTheGMsOwnWorktreeItsPlansAndWhichHireTellsYou(t *testing.T) {
	for _, doc := range [][]string{{"RELEASE-NOTES.md"}, {"docs", "team.md"}} {
		text := strings.Join(strings.Fields(repoText(t, doc...)), " ")
		mustSay(t, doc[len(doc)-1], text,
			"the General Manager's own", "when you hire it into a repository.",
			"A managed hire from Setup or from",
			"from the office may not",
			"for a managed hire from Setup or from the General Manager, to the hire",
			"It can't make a git folder in it, but it can move in a folder that holds one, and Smolcorp doesn't check that at its next start: don't run git in its plans folder, read or copy the plans instead.")
	}
	parity := strings.Join(strings.Fields(repoText(t, "docs", "GO-PARITY.md")), " ")
	mustSay(t, "GO-PARITY.md", parity, `from the GM's token it's refused (403): only the owner hires into T3 Code`)
	mustNotSay(t, "GO-PARITY.md", parity, "`engine` as the Mac's: `\"t3\"` hires into T3 Code")
}
