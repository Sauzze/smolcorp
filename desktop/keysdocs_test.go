package desktop

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Design B's docs and release notes (SD1): what the owner reads about the keys, the records and the guard says what
// the code does, names what it can't stop, promises nothing about desks' memory, and never points at a later release.

// repoText is a file of the repository, its whitespace folded to single spaces, as a reader sees the words.
func repoText(t *testing.T, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{".."}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(string(data)), " ")
}

// mustNotSay fails for each of words that text says.
func mustNotSay(t *testing.T, name, text string, words ...string) {
	t.Helper()
	for _, word := range words {
		if strings.Contains(text, word) {
			t.Errorf("%s still says %q", name, word)
		}
	}
}

func mustSay(t *testing.T, name, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("%s doesn't say %q", name, want)
		}
	}
}

// The keys page says each thing the owner will notice, on each system, and its honest limits.
func TestTheKeysPageSaysWhatChangesAndWhatItCantStop(t *testing.T) {
	page := repoText(t, "docs", "keys-and-guard.md")
	mustSay(t, "docs/keys-and-guard.md", page,
		// What the headline holds off a Mac (RT126SD1 B2).
		"A hire can't loosen the guard while Smolcorp runs, and on a Mac not even while it's stopped",
		// Setup's key in memory, the keys still in files and what each opens (RT126SD1 B1), and the link's lifetime
		// per system (setupLinkLifetime).
		"Setup's key is in memory only", "and on a Mac, of each phone's", "A few keys are still in files",
		"`hook-key` (and on a Mac, `hook-header`)", "`selftest-key`: it reads Start here's checklist and opens nothing else",
		"so it opens what the phone opens", "five minutes from the Mac's menu", "one minute on Windows and Linux",
		setupLinkUsedFirst[:strings.Index(setupLinkUsedFirst, ".")+1],
		"while no General Manager is hired", "run `smolcorp gm login` again",
		"only while it holds the lock on Smolcorp's folder", "such as unpairing a phone",
		// Records: the reset, the locked keychain, a build of your own; off a Mac, nothing sealed.
		"**Reset**", "Unlock the keychain and restart Smolcorp", "ad hoc rebuild", "nothing is sealed",
		"readable by you alone",
		// The update: the General Manager's new token comes with its next hire (RT126SD1 S3).
		"Pair each phone again, and each paired Mac", "and the General Manager when you hire it again",
		// The guard, a broken guard file as a desk meets it (S1), and the messages quoted whole (S6).
		"2.1.314 or later", "can only add rules", "Anthropic's own installer", "a hard link",
		"the guard refuses every command a desk runs", "can't answer. No desk was started.\"",
		"Anthropic's installer, then try again.\"",
		// The start: which desks get the ticket, and what still runs for the others (B4); what can't be checked; the
		// app's own seal, on a Mac and for Claude Code desks (S2).
		"The General Manager and the Claude Code hires it manages, in tmux (Mac and Linux)",
		"starts on tmux's usual server, which runs your `~/.tmux.conf`", "so its start files run first",
		"a desk gets a one-time token from Smolcorp's launcher", "**Use this copy**",
		"Claude Code installed with npm, Codex, Gemini CLI, OpenCode and Grok",
		"(on a Mac), it starts no more Claude Code desks", ownFilesChanged+" No desk was started.\"",
		// The grants, and macOS's prompts.
		"Documents, Desktop, Downloads or iCloud Drive", "find devices on your Wi-Fi",
		// Memory: Smolcorp's alone, never the desks'.
		"The tray app doesn't.", "get no such protection",
		// The signing key: the self-test's line, worded as advice.
		signingKeyCheck().Title, "proves nothing", "`SMOLCORP_SIGN_IDENTITY`",
		"doesn't show for it unless the key it's signed with is on your Mac",
		// What it doesn't stop, the Mac's limits scoped to the Mac and the Windows tray app's open memory
		// (RT126SD1R2 N-B1), and the hook key's launcher on every system (nit).
		"A hire can still act as any desk it can reach",
		"On a Mac, it can't open Setup, read your phones' keys, change your guard, or forge a record.",
		"**On Windows, the tray app's memory.** Any program of yours can read it,",
		"Setup's key and the office window's included, and so open either. `smolcorp serve` refuses that.",
		"and connect as a desk's launcher", "OpenCode and Grok aren't checked at all",
		"while Smolcorp is stopped", "system's secret store",
		"from that desk's environment", "runs without the guard",
		// The rest of SANDBOX-PLAN §5: the hook key (5.10), a copy of the app (5.14), the kind of Claude Code (5.12).
		"**The hook key** stays readable in `hook-key`", "a copy of Smolcorp.app started while yours is closed",
		"the first Claude Code Smolcorp sees after the update")
	// What the code doesn't back stays out (RT126SD1 B1–B3, nits).
	mustNotSay(t, "docs/keys-and-guard.md", page, "No file holds a key", "can't be loosened or switched off by a hire",
		"this line never shows for it", "doesn't yet", "starts no more hires", "or forge a record on a Mac",
		"programs nobody signs")
	for _, link := range []string{"README.md", filepath.Join("docs", "setup.md")} {
		if !strings.Contains(repoText(t, strings.Split(link, string(filepath.Separator))...), "keys-and-guard.md") {
			t.Errorf("%s doesn't link the keys page", link)
		}
	}
}

// A 502 from `tailscale serve` is explained where the owner looks: Setup and the docs each name its three causes and
// their fixes, on the Mac and on Linux (RT126SN1U N1, RT126OLDSERVE1 nit 9).
func TestA502IsExplainedInSetupAndTheDocs(t *testing.T) {
	setup := repoText(t, "Resources", "web", "setup.html")
	start := strings.Index(setup, `id="serve-502"`)
	if start < 0 {
		t.Fatal("setup has no 502 line")
	}
	block := setup[start : start+strings.Index(setup[start:], "</div>")]
	mustSay(t, "setup's 502 line", block, "502 Bad Gateway", "three things",
		"couldn't open its private socket", "Quit Smolcorp and open it again",
		`class="serve-here serve-mac" hidden>Tailscale is the Mac App Store version`, "Install it from tailscale.com",
		`class="serve-here serve-linux" hidden>Smolcorp refused who connected`, "<code>hidepid</code>", "as root",
		"can't reach the socket", "<code>ProtectHome=</code>", "this computer's Tailscale address")
	// Hidden off Mac and Linux with the rest of the section, and each system's line on its own system only.
	if !strings.HasPrefix(setup[strings.LastIndex(setup[:start], "<"):], `<div class="hint serve-here" id="serve-502">`) {
		t.Error("the 502 line isn't one of the section's serve-here lines")
	}
	if strings.Count(setup, "--operator") != 1 {
		t.Error("the operator setting is said more than once")
	}

	phone := repoText(t, "docs", "phone.md")
	mustSay(t, "docs/phone.md", phone, "## HTTPS through tailscale serve (Mac and Linux)",
		"### If the phone shows 502 Bad Gateway", "Quit Smolcorp and open it again", "Mac App Store",
		"doesn't run as root", "`hidepid`", "`ProtectHome=`", "Each of these fails closed",
		"`sudo tailscale serve --bg --https=443 unix:<Smolcorp's folder>/serve.sock`", "but only as an option",
		"turn it on again, once")
	// On Linux, the sudo line comes first and the operator setting only after it, as Setup says them (OLD-SERVE1F).
	if strings.Index(phone, "--operator") < strings.Index(phone, "sudo tailscale serve --bg") {
		t.Error("docs/phone.md says the operator setting before the sudo line")
	}
	mustSay(t, "docs/troubleshooting.md", repoText(t, "docs", "troubleshooting.md"),
		"502 Bad Gateway", "(phone.md#if-the-phone-shows-502-bad-gateway)")
	mustSay(t, "RELEASE-NOTES.md", repoText(t, "RELEASE-NOTES.md"), "If your phone shows 502 Bad Gateway")
}

// The release notes cover design B on every system, say Smolcorp's memory is covered and the desks' isn't, and the
// owner's one release leaves no "1.27" anywhere the owner reads.
func TestTheReleaseNotesCoverTheKeysOnEverySystem(t *testing.T) {
	notes := repoText(t, "RELEASE-NOTES.md")
	this := notes[:strings.Index(notes, "# Smolcorp 1.25.0")]
	mustSay(t, "1.26's release notes", this,
		"## Smolcorp's keys stay out of your agents' reach", "Pair your phones again, once",
		"one minute on Windows and Linux", "**Records nobody else can write (Mac).**", "2.1.314 or later",
		"**Use this copy**", "iCloud Drive", "`smolcorp gm login`", "but the tray app doesn't.",
		// The headline's "can't read" holds on a Mac, on Linux, and on Windows under `smolcorp serve` only
		// (RT126SD1R2 N-B1).
		"they can't loosen Smolcorp's guard while it runs, and on a Mac or Linux they can't read Setup's key",
		"On Windows, only `smolcorp serve` keeps them out of that memory",
		"the tray app leaves its own readable by every program of yours, Setup's key included",
		"Windows and Linux can seal less",
		"**Setup's key stays in memory.**", "the check-only `selftest-key`", "in tmux on a Mac or Linux",
		"Other hires in tmux still run your `~/.tmux.conf`", "OpenCode and Grok", "with no General Manager hired",
		"get no such protection", "**On Windows and Linux, less can be sealed.**", "Start here says so",
		"Claude Code installed with npm", "(docs/keys-and-guard.md)", "(docs/RELEASING.md)")
	mustNotSay(t, "1.26's release notes", this, "No key in a file", "can't read Smolcorp's keys", "doesn't yet",
		"hires and the General Manager get new keys", "From 1.26 they can't read Setup's key",
		"can't read Setup's key or loosen", "aren't signed, so they can't be checked")
	// Nothing promises the desks' memory: each line that says memory is protected names Smolcorp, never a desk.
	for _, sentence := range regexp.MustCompile(`[^.]*memory[^.]*\.`).FindAllString(this, -1) {
		if regexp.MustCompile(`(?i)(desk|agent|hire)s?'? (own )?memory (is|are) (protected|safe|covered)`).MatchString(sentence) {
			t.Errorf("the release notes promise a desk's memory: %q", sentence)
		}
	}
	for _, name := range []string{"RELEASE-NOTES.md", "README.md", filepath.Join("Resources", "web", "setup.html")} {
		if strings.Contains(repoText(t, strings.Split(name, string(filepath.Separator))...), "1.27") {
			t.Errorf("%s names a release after 1.26", name)
		}
	}
	docs, _ := filepath.Glob(filepath.Join("..", "docs", "*.md"))
	for _, doc := range docs {
		if data, _ := os.ReadFile(doc); strings.Contains(string(data), "1.27") {
			t.Errorf("%s names a release after 1.26", doc)
		}
	}
}

// Every link into a page's heading that SD1 added lands on a heading that's there, as GitHub makes its anchors.
func TestTheKeysDocsLinksLand(t *testing.T) {
	anchors := func(doc string) map[string]bool {
		data, err := os.ReadFile(filepath.Join("..", "docs", doc))
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, line := range strings.Split(string(data), "\n") {
			if heading, ok := strings.CutPrefix(strings.TrimLeft(line, "#"), " "); ok && strings.HasPrefix(line, "#") {
				slug := strings.ToLower(heading)
				slug = regexp.MustCompile(`[^a-z0-9 _-]`).ReplaceAllString(slug, "")
				found[strings.ReplaceAll(slug, " ", "-")] = true
			}
		}
		return found
	}
	for _, link := range []struct{ doc, anchor string }{
		{"phone.md", "https-through-tailscale-serve-mac-and-linux"},
		{"phone.md", "if-the-phone-shows-502-bad-gateway"},
		{"keys-and-guard.md", "what-this-doesnt-stop"},
		{"keys-and-guard.md", "if-you-build-smolcorp-yourself-your-signing-key"},
		{"team.md", "the-fence-and-who-gets-it"},
	} {
		if !anchors(link.doc)[link.anchor] {
			t.Errorf("docs/%s has no heading for #%s", link.doc, link.anchor)
		}
	}
}

// Off a Mac, Start here ends with what another program of yours can still do (the owner's Q7: "with Setup saying
// so"): worth knowing, nothing to press, and never in the way of being ready.
func TestStartHereSaysWhatAnotherProgramCanStillDo(t *testing.T) {
	c := limitsCheck()
	if c.ID != "limits" || c.Status != checkInfo || c.Fix != "" || c.Label != "" {
		t.Errorf("the limits line: %+v", c)
	}
	mustSay(t, "the limits line", c.Detail, "While Smolcorp is stopped", "add a phone", "loosen the guard",
		"at its next start", "secret store", "environment", "Keys, records and the guard")
	if !ready([]check{c}) {
		t.Error("the limits line stands in the way of being ready")
	}
}
