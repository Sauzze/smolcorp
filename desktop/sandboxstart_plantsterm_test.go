package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXPLANTSTERMR2 (the ruling's part 1, item 2): the link check passes over the Mac's FXPLANTSTERM rows, the owner's
// preferences, terminals' folders, agents' folders and keychains, which no desk writes and which grow with the owner's
// use. A big tree there, and a file with two names there (the owner's own), don't refuse a desk's start; without the
// pass-over, each does, at every start. A key with a second name there is still found by its own name, in the support
// folder, walked whole, and refuses the start. A Mac's Policy, from the stand-in tree's own paths.
func TestTheLinkCheckPassesOverTheOwnersAppFoldersAndStillFindsAKeysSecondName(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("a Mac's Policy needs a Mac's paths: off a Mac, /tmp isn't /private/tmp")
	}
	tr := newLinkTree(t)
	pl := tr.places()
	pl.GOOS = "darwin"
	at := func(rel string) string { return filepath.Join(tr.home, filepath.FromSlash(rel)) }
	write := func(file, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	link := func(file, other string) {
		t.Helper()
		if err := os.Link(file, other); err != nil {
			t.Fatal(err)
		}
	}
	// A long-used Codex's sessions, the preferences and a keychain: more entries than the check looks at, with it set
	// low here, above what the rest of the stand-in tree's rows take (229 in FXPLANTSTERMR3); and the owner's own files with two names, in one folder and across two. Not at the top of ~/.codex or
	// ~/.gemini: the sign-ins' row looks there still (a sign-in's second name is a leak), as it did before.
	was := linkCheckMost
	linkCheckMost = 500
	defer func() { linkCheckMost = was }()
	for i := range 600 {
		write(at(fmt.Sprintf(".codex/sessions/2026/11/%02d/rollout-%d.jsonl", i%28+1, i)), "{}\n")
	}
	// …and an owner's Codex moved by CODEX_HOME (FXPLANTSTERMR3), and OpenCode's cache with its plugins: passed over
	// as ~/.codex is.
	pl.AgentHomes, pl.AgentSignIns = sandbox.AgentPlaces(func(v string) string {
		return map[string]string{"CODEX_HOME": at("work/codex-home")}[v]
	})
	for i := range 600 {
		write(at(fmt.Sprintf("work/codex-home/sessions/2026/11/%02d/rollout-%d.jsonl", i%28+1, i)), "{}\n")
		write(at(fmt.Sprintf(".cache/opencode/node_modules/p%d/index.js", i)), "// a plugin")
	}
	link(at("work/codex-home/sessions/2026/11/01/rollout-0.jsonl"), at(".cache/opencode/node_modules/p0/again.jsonl"))
	write(at("Library/Preferences/com.apple.Terminal.plist"), "standin")
	write(at("Library/Keychains/login.keychain-db"), "standin")
	write(at(".gemini/tmp/app/chats/session-1.jsonl"), "{}\n")
	link(at(".codex/sessions/2026/11/01/rollout-0.jsonl"), at(".codex/sessions/2026/11/01/rollout-again.jsonl"))
	link(at(".gemini/tmp/app/chats/session-1.jsonl"), at("Library/Preferences/gemini-chat-again.jsonl"))
	link(at("Library/Keychains/login.keychain-db"), at("Library/Keychains/login-again.keychain-db"))

	pol, err := sandbox.New(pl)
	if err != nil {
		t.Fatal(err)
	}
	if replaced, err := checkLinks(pol, pl); err != nil || len(replaced) != 0 {
		t.Fatalf("the owner's own folders refused the start: %v (replaced %v)", err, replaced)
	}

	// Without the pass-over, the same tree refuses every start: past the most it looks at, and at the owner's files with
	// two names.
	// FXPLANTSTERM's three rows by name: FXHIREPROJECTS' git-folder rows share their prefix (FXSTACKG G22).
	skipped := map[string]string{}
	for _, row := range []string{"plants the owner's programs run later: what a terminal reads at a window's start",
		"plants the owner's programs run later: a hired agent's own settings", "plants the owner's programs run later: the keychains"} {
		if why, ok := linkCheckSkipped[row]; ok {
			skipped[row] = why
			delete(linkCheckSkipped, row)
		}
	}
	if len(skipped) != 3 {
		t.Errorf("the pass-over names %d of FXPLANTSTERM's rows, not 3", len(skipped))
	}
	if _, err := checkLinks(pol, pl); err == nil || !strings.Contains(err.Error(), "more than 500") {
		t.Errorf("without the pass-over, the big tree: %v", err)
	}
	linkCheckMost = was
	if _, err := checkLinks(pol, pl); err == nil {
		t.Error("without the pass-over, the owner's files with two names don't refuse the start")
	}
	for row, why := range skipped {
		linkCheckSkipped[row] = why
	}

	// The setup key with a second name in the agent's sessions, which the check passes over and a desk reads: still
	// refused, by its own name.
	link(filepath.Join(tr.support, "setup-key"), at(".codex/sessions/2026/11/01/key-again"))
	_, err = checkLinks(pol, pl)
	if err == nil {
		t.Fatal("the setup key with a second name in ~/.codex didn't refuse the start")
	}
	if !strings.Contains(err.Error(), filepath.Join(tr.support, "setup-key")) {
		t.Errorf("the refusal doesn't name the key's own place: %v", err)
	}
}
