package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// SA-M1: the Mac's profile, files, from the Policy. Data only, on the made-up Mac of policy_test.go: the profile's
// text is the golden one, and the profile, played as Seatbelt reads it (each operation on its own, the last rule that
// matches deciding), says what Decide says for every case of the Policy's table, in every column. Real sandbox-exec
// runs are in sbpl_darwin_test.go.

// profileSetups are the golden columns: policy_test.go's, and a desk at the project.
func profileSetups() []setup {
	atProject := hirePlaces(realSupport)
	atProject.Root, atProject.Checkout, atProject.OwnWorktree = repo, "", ""
	return append(setups(), setup{"project", atProject})
}

// goldenName is a column's file in testdata.
func goldenName(name string) string {
	return filepath.Join("testdata", strings.NewReplacer(" ", "-", "'", "").Replace(name)+".sb")
}

// goldenServices is what a golden column's desk may reach: a stand-in of the lists (SA-X2's lists.go), one socket
// spelled through /var as the core's environment may give it, the agents (denied: every column's Remote Login
// answers), and its own socket.
func goldenServices(pl Places) Services {
	return Services{OwnSocket: pl.OwnSockets + "/s",
		Sockets:     []string{"/var/run/mDNSResponder", "/private/var/run/syslog"},
		Agents:      []string{"/private/tmp/com.apple.launchd.standin/Listeners", home + "/Library/Group Containers/2BUA8C4S2C.com.1password/t/agent.sock"},
		Mach:        []string{"com.apple.system.opendirectoryd.libinfo", "com.apple.trustd.agent"},
		IOKit:       []IOKitOpen{{"IOSurfaceRootUserClient", false}, {"IOSurfaceSendRight", true}},
		DeniedPorts: []int{22, 2375, 2376, 5432, 3306}}
}

// The profile is the golden text, byte for byte. To write it again after a change to the Policy or the profile, run
// the test with SMOLCORP_SBPL_GOLDEN=write, and read the diff.
func TestTheMacProfileIsTheGoldenText(t *testing.T) {
	for _, s := range profileSetups() {
		profile, err := mustNew(t, s.pl).Profile(goldenServices(s.pl))
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		file := goldenName(s.name)
		if os.Getenv("SMOLCORP_SBPL_GOLDEN") == "write" {
			if err := os.WriteFile(file, []byte(profile), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		golden, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("%s: %v (SMOLCORP_SBPL_GOLDEN=write makes it)", s.name, err)
		}
		if profile != string(golden) {
			got, want := strings.Split(profile, "\n"), strings.Split(string(golden), "\n")
			for i := 0; i < max(len(got), len(want)); i++ {
				if i >= len(got) || i >= len(want) || got[i] != want[i] {
					t.Fatalf("%s: line %d differs from %s:\n got %q\nwant %q", s.name, i+1, file,
						strings.Join(got[i:min(i+1, len(got))], ""), strings.Join(want[i:min(i+1, len(want))], ""))
				}
			}
		}
	}
}

// fileHeadings are the headings of the file rules' sections (SA-M1), in Profile's order.
var fileHeadings = []string{"Allowed places", "Denied places an exception opens", "The exceptions", "Denied places nothing opens"}

// profileSections is a profile's lines of the file rules, section by section, in Profile's order: the plain allows,
// the Wide denies, the exceptions, the final denies. SA-M2's sections, before and after them, are left out.
func profileSections(profile string) [][]string {
	var sections [][]string
	for _, section := range strings.Split(profile, "\n;; ")[1:] {
		lines := strings.Split(strings.TrimSpace(section), "\n")
		if slices.Contains(fileHeadings, lines[0]) {
			sections = append(sections, lines[1:])
		}
	}
	return sections
}

// profileWithout is p's profile before the rows drop picks were in the Policy: the other rows, and the rename guards
// made again from them alone, as New would make them.
func profileWithout(t *testing.T, p *Policy, svc Services, drop func(Rule) bool) string {
	t.Helper()
	b := &builder{goos: p.goos}
	for _, r := range p.rules {
		if r.Row != rowAbove && !drop(r) {
			b.rules = append(b.rules, r)
		}
	}
	b.denyAbove()
	profile, err := (&Policy{goos: p.goos, rules: b.rules}).Profile(svc)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

// A row added to the Policy only adds lines to the profile (STUDIO34): no line is taken out or changed, so nothing
// the profile denied is opened and nothing it allowed is widened. NS1's row is one line more in every column, read
// only, in the exceptions, and the only line naming nightshift.json; every other line stays where it was. GITCFG1's
// rows (the hooks folders, what git's config names, a reftable repo's refs) add final denies and their rename guards
// only. A guard a new row needs too comes earlier than before, still in the final denies, where order says nothing.
func TestARowAddedToThePolicyOnlyAddsLinesToTheProfile(t *testing.T) {
	gitcfg1 := func(r Rule) bool {
		return r.Row == "the base branches: a reftable repo's refs" ||
			r.Row == "plants the owner's programs run later: git's hooks folder" ||
			r.Row == "plants the owner's programs run later: what git's config names"
	}
	for _, s := range profileSetups() {
		p, svc := mustNew(t, s.pl), goldenServices(s.pl)
		profile, err := p.Profile(svc)
		if err != nil {
			t.Fatal(err)
		}
		if again := profileWithout(t, p, svc, func(Rule) bool { return false }); again != profile {
			t.Fatalf("%s: the Policy made again without dropping a row isn't the same profile", s.name)
		}
		after := profileSections(profile)

		// NS1: one line more, and only that.
		night := `(allow file-read-data file-read-xattr (literal "` + s.pl.Support + `/nightshift.json")) ; the night shift's jobs: read only, as the guard file`
		before := profileSections(profileWithout(t, p, svc, func(r Rule) bool { return r.Row == "the night shift's jobs: read only, as the guard file" }))
		if n := strings.Count(profile, "nightshift.json"); n != 1 {
			t.Errorf("%s: %d lines name nightshift.json, want only its read-only exception", s.name, n)
		}
		for i := range after {
			got := after[i]
			if i == 2 {
				if at := slices.Index(got, night); at < 0 {
					t.Errorf("%s: the exceptions have no %s", s.name, night)
				} else {
					got = slices.Delete(slices.Clone(got), at, at+1)
				}
			}
			if !slices.Equal(got, before[i]) {
				t.Errorf("%s: NS1's row changes section %d by more than its read-only line:\n got %q\nwant %q", s.name, i, got, before[i])
			}
		}

		// GITCFG1: final denies and their guards added; nothing else changes, nothing is lost.
		before = profileSections(profileWithout(t, p, svc, gitcfg1))
		for i := range 3 {
			if !slices.Equal(after[i], before[i]) {
				t.Errorf("%s: GITCFG1's rows change section %d", s.name, i)
			}
		}
		added := slices.Clone(after[3])
		for _, line := range before[3] {
			at := slices.Index(added, line)
			if at < 0 {
				t.Errorf("%s: GITCFG1's rows take out %s", s.name, line)
				continue
			}
			added = slices.Delete(added, at, at+1)
		}
		for _, line := range added {
			if !strings.HasPrefix(line, "(deny ") || !(strings.HasSuffix(line, " ; "+rowAbove) || gitcfg1(Rule{Row: line[strings.LastIndex(line, " ; ")+3:]})) {
				t.Errorf("%s: GITCFG1's rows add %s", s.name, line)
			}
		}
		for _, folder := range s.pl.GitHooks {
			if want := `(deny file-write* file-link (subpath "` + folder + `")) ; plants the owner's programs run later: git's hooks folder`; !slices.Contains(added, want) {
				t.Errorf("%s: no %s", s.name, want)
			}
		}
		for _, file := range s.pl.GitNamed {
			if want := `(deny file-write* file-link (literal "` + file + `")) ; plants the owner's programs run later: what git's config names`; !slices.Contains(added, want) {
				t.Errorf("%s: no %s", s.name, want)
			}
		}

		// FXTMUXDIR: tmux's own folder, one final deny (and any guard it needs); nothing else changes, nothing is lost.
		before = profileSections(profileWithout(t, p, svc, func(r Rule) bool { return r.Row == rowTmuxFolder }))
		for i := range 3 {
			if !slices.Equal(after[i], before[i]) {
				t.Errorf("%s: tmux's own folder's row changes section %d", s.name, i)
			}
		}
		added = slices.Clone(after[3])
		for _, line := range before[3] {
			if at := slices.Index(added, line); at < 0 {
				t.Errorf("%s: tmux's own folder's row takes out %s", s.name, line)
			} else {
				added = slices.Delete(added, at, at+1)
			}
		}
		want := `(deny file-write* file-link (subpath "` + s.pl.TmuxFolder + `")) ; tmux's own folder`
		if !slices.Contains(added, want) {
			t.Errorf("%s: no %s", s.name, want)
		}
		for _, line := range added {
			if line != want && !(strings.HasPrefix(line, "(deny ") && strings.HasSuffix(line, " ; "+rowAbove)) {
				t.Errorf("%s: tmux's own folder's row adds %s", s.name, line)
			}
		}
	}
}

// The desk's own status and report stay its own to write in every column (STUDIO34: the *.md row is still there
// once NS1's line is in), as an exception to the support folder's Wide deny.
func TestTheDesksOwnStatusAndReportStayWritableInEveryProfile(t *testing.T) {
	for _, s := range profileSetups() {
		profile, err := mustNew(t, s.pl).Profile(goldenServices(s.pl))
		if err != nil {
			t.Fatal(err)
		}
		own := strings.ReplaceAll(regexp.QuoteMeta(s.pl.Support+"/hires/"+s.pl.OwnID), `\`, `\\`)
		want := `(allow file-read-data file-read-xattr file-write* file-link (regex "^` + own + `/[^/]*\\.md$")) ; its status and report`
		if !slices.Contains(profileSections(profile)[2], want) {
			t.Errorf("%s: the exceptions have no %s", s.name, want)
		}
	}
}

// sbCheck is one Seatbelt operation on a file of one kind ("" for a regular file).
type sbCheck struct{ op, vnode string }

// The Seatbelt operations a desk's Op is checked as. A Write the Policy allows is every one that changes the file; one
// it denies is those, and its removal, renames and links too (which Move and Link check where the Write is allowed).
// A Link is a hard link or a new symlink.
var (
	sbWriteChecks = []sbCheck{{"file-write-create", ""}, {"file-write-data", ""}, {"file-write-mode", ""}, {"file-write-flags", ""},
		{"file-write-owner", ""}, {"file-write-times", ""}, {"file-write-xattr", ""}}
	sbChecks = [Link + 1][]sbCheck{
		Read:    {{"file-read-data", ""}, {"file-read-xattr", ""}},
		Write:   sbWriteChecks,
		Connect: {{"network-outbound", ""}},
		Move:    {{"file-write-unlink", "DIRECTORY"}},
		Link:    {{"file-link", ""}, {"file-write-create", "SYMLINK"}},
	}
	sbDeniedWriteChecks = append(slices.Clone(sbWriteChecks), sbCheck{"file-write-unlink", ""}, sbCheck{"file-link", ""},
		sbCheck{"file-write-create", "SYMLINK"})
)

// seatbeltPlay is a profile's rules read as Seatbelt reads them, on the Mac's volume: case and normalization folded,
// and for each operation the rules that name it read first, the last that matches deciding; those that name it by a
// wildcard (file-write*) only where none of its own matches (sbpl.go). Everything else is allowed.
type seatbeltPlay struct {
	rules []sbRule
	res   map[string]*regexp.Regexp
}

func playOf(t *testing.T, p *Policy) seatbeltPlay {
	t.Helper()
	b, err := p.fileRules()
	if err != nil {
		t.Fatal(err)
	}
	return newPlay(slices.Concat(b.plain, b.wide, b.exceptions, b.final))
}

// playWith is p's whole profile with svc, SA-M2's sections too, played as Seatbelt reads it.
func playWith(t *testing.T, p *Policy, svc Services) seatbeltPlay {
	t.Helper()
	sections, err := p.sections(svc)
	if err != nil {
		t.Fatal(err)
	}
	var rules []sbRule
	for _, s := range sections {
		rules = append(rules, s.rules...)
	}
	return newPlay(rules)
}

func newPlay(rules []sbRule) seatbeltPlay {
	play := seatbeltPlay{rules: rules, res: map[string]*regexp.Regexp{}}
	for _, r := range play.rules {
		if r.place.kind == "regex" {
			play.res[r.place.text] = regexp.MustCompile(foldName("darwin", r.place.text))
		}
	}
	return play
}

// matches is whether r holds folded path k for check c, naming its operation itself (own) or by a wildcard.
func (s seatbeltPlay) matches(r sbRule, k string, c sbCheck, own bool) bool {
	if r.place.vnode != "" && r.place.vnode != c.vnode {
		return false
	}
	if !slices.ContainsFunc(r.ops, func(op string) bool {
		if own {
			return op == c.op
		}
		return strings.HasSuffix(op, "*") && strings.HasPrefix(c.op, strings.TrimSuffix(op, "*"))
	}) {
		return false
	}
	switch r.place.kind {
	case "literal":
		return k == key("darwin", r.place.text)
	case "subpath":
		return under(k, key("darwin", r.place.text))
	case "regex":
		return s.res[r.place.text].MatchString(k)
	case sbRaw:
		// SA-M2's defaults: a connect to any unix socket's path; the rest name no path.
		return r.place.text == "(remote unix-socket)"
	}
	return false // a TCP port, a Mach service, an IOKit class: no path
}

// says is what Seatbelt does with check c at path at.
func (s seatbeltPlay) says(at string, c sbCheck) Verdict {
	k := key("darwin", at)
	for _, own := range []bool{true, false} {
		said := Unset
		for _, r := range s.rules {
			if s.matches(r, k, c, own) {
				said = Deny
				if r.allow {
					said = Allow
				}
			}
		}
		if said != Unset {
			return said
		}
	}
	return Allow
}

// sbExtraPaths are cases the Policy's table doesn't spell: the profile's regexes and the case of a name.
var sbExtraPaths = []string{"$ROOT/sub/xclaude", "$ROOT/sub/.CLAUDE", "$ROOT/sub/.claude/SETTINGS.JSON", "$ROOT/.claudeX",
	"$H/.sshx/id", "$H/.ssh/id.pubx", "$H/.ssh/x.pub/inner", "$H/.ssh/KNOWN_HOSTS", "$H/.claudeXjson", "$H/.claude.jsonl",
	"$S/hires/$ID/BRIEFmd", "$S/hires/$ID/BRIEF.md/inner", "$S/hires/$ID/a/b.md", "$S/sandbox-canary-", "$S/sandbox-canaryX1",
	"$G/worktrees/$WT/sub/commondir", "$G/worktrees/commondir", "$G/refs/heads/MAIN", "$H/.ZSHRC", "$S/hires/$ID/claude/SETTINGS.JSON",
	"$S/Hires/$ID/STATUS.md", "/", "/Users", "$H/Library/Application Support/AgentDeck2/x"}

// The profile, played as Seatbelt reads it, says what Decide says: for each Op, every Seatbelt operation it is checked
// as, for every path of the Policy's table and more, in every column, a desk at the project, and the roots New refuses
// for holding desks. A file's metadata is never denied.
func TestTheMacProfilePlayedAsSeatbeltSaysWhatDecideSays(t *testing.T) {
	type policy struct {
		name string
		p    *Policy
	}
	var policies []policy
	var paths []string
	for _, s := range profileSetups() {
		policies = append(policies, policy{s.name, mustNew(t, s.pl)})
		for _, r := range rows {
			paths = append(paths, s.path(r.path))
		}
		for _, at := range sbExtraPaths {
			paths = append(paths, s.path(at))
		}
	}
	for _, c := range holdingCases {
		paths = append(paths, c.at)
		pl := hirePlaces(realSupport)
		pl.Root = c.root
		policies = append(policies, policy{"root " + c.root, mustBuild(t, pl)})
	}
	checked := 0
	for _, p := range policies {
		play := playOf(t, p.p)
		for _, at := range paths {
			if got := play.says(at, sbCheck{"file-read-metadata", ""}); got != Allow {
				t.Errorf("%s: %s's metadata is denied", p.name, at)
			}
			for op := Read; op <= Link; op++ {
				want, by := p.p.Decide(at, op)
				checks := sbChecks[op]
				if op == Write && want == Deny {
					checks = sbDeniedWriteChecks
				}
				for _, c := range checks {
					checked++
					if got := play.says(at, c); got != want {
						t.Errorf("%s: %v %s as %s %s: the profile says %v, Decide %v by %q", p.name, op, at, c.op, c.vnode, got, want, by.Row)
					}
				}
			}
		}
	}
	if checked < 100000 {
		t.Errorf("only %d cases checked", checked)
	}
}

// The profile is in the documented order, as policy_test.go's TestAProfileInTheDocumentedOrderSaysWhatDecideSays
// plays it: the plain allows, the Wide denies, the exceptions, the final denies; SA-M2's defaults and what it may
// still reach before them, and what nothing opens after. Its sections say so in its text, and each heading comes
// once, in that order, after the header.
func TestTheMacProfilesSectionsAreInTheDocumentedOrder(t *testing.T) {
	profile, err := mustNew(t, hirePlaces(realSupport)).Profile(goldenServices(hirePlaces(realSupport)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(profile, ";; A desk's sandbox") || !strings.Contains(profile, "\n(version 1)\n(allow default)\n") {
		t.Errorf("the header: %q", profile[:min(len(profile), 200)])
	}
	last := -1
	for _, heading := range slices.Concat([]string{"Denied by default: unix sockets, Mach services, IOKit, signals", "What it may still reach"},
		fileHeadings, []string{"Denied, whatever allows it"}) {
		at := strings.Index(profile, "\n;; "+heading+"\n")
		if at <= last || strings.Count(profile, "\n;; "+heading+"\n") != 1 {
			t.Errorf("%q is at %d, after %d", heading, at, last)
		}
		last = at
	}
	// Each section holds only its kind of line.
	sections := strings.Split(profile, "\n;; ")[1:]
	for i, want := range []string{"(deny ", "(allow ", "(allow ", "(deny ", "(allow ", "(deny ", "(deny "} {
		for _, line := range strings.Split(strings.TrimSpace(sections[i]), "\n")[1:] {
			if !strings.HasPrefix(line, want) {
				t.Errorf("section %d holds %q", i, line)
			}
		}
	}
}

// A place a profile can't be given exactly fails the whole profile, and so does a Policy that isn't a Mac's: the desk
// then doesn't start. A double quote and a backslash are escaped, in a literal and in a regex.
func TestAProfileIsRefusedForWhatItCantSpell(t *testing.T) {
	for _, bad := range []string{"/Users/a\nb", "/Users/a\tb", "/Users/a\x7fb", "/Users/a\u0085b", "/Users/a\xffb"} {
		pl := hirePlaces(realSupport)
		pl.Accounts = []string{bad}
		if _, err := mustNew(t, pl).Profile(); !errors.Is(err, ErrUnspellable) {
			t.Errorf("an account at %q: %v, want ErrUnspellable", bad, err)
		}
	}
	for _, tail := range []string{"[ab]", `a\*`, "a//b"} {
		b := &builder{goos: "darwin"}
		b.add(Rule{Row: "a pattern", Path: "/Users/ada", Tail: tail, Write: Deny})
		if _, err := (&Policy{goos: "darwin", rules: b.rules}).Profile(); !errors.Is(err, ErrTailOutOfSet) {
			t.Errorf("a Tail %q: %v, want ErrTailOutOfSet", tail, err)
		}
	}
	linux := hirePlaces(realSupport)
	linux.GOOS = "linux"
	if _, err := mustNew(t, linux).Profile(); !errors.Is(err, ErrNotAMac) {
		t.Errorf("a Linux Policy: %v", err)
	}
	if _, err := (*Policy)(nil).Profile(); !errors.Is(err, ErrNotAMac) {
		t.Errorf("no Policy: %v", err)
	}

	odd := `/Users/a"b\c (d) [e] {f} +g ^h $i |j .k ?l *m`
	b := &builder{goos: "darwin"}
	b.add(Rule{Row: "odd", Path: odd, Write: Deny})
	b.add(Rule{Row: "odd", Path: odd, Tail: "*.md", Write: Deny})
	profile, err := (&Policy{goos: "darwin", rules: b.rules}).Profile()
	if err != nil {
		t.Fatal(err)
	}
	if want := `(deny file-write* file-link (literal "/Users/a\"b\\c (d) [e] {f} +g ^h $i |j .k ?l *m"))`; !strings.Contains(profile, want) {
		t.Errorf("the literal isn't %s in:\n%s", want, profile)
	}
	if want := `(regex "^/Users/a\"b\\\\c \\(d\\) \\[e\\] \\{f\\} \\+g \\^h \\$i \\|j \\.k \\?l \\*m/[^/]*\\.md$")`; !strings.Contains(profile, want) {
		t.Errorf("the regex isn't %s in:\n%s", want, profile)
	}
	re := regexp.MustCompile(sbQuoteMeta(odd) + "/[^/]*" + sbQuoteMeta(".md"))
	if !re.MatchString(odd+"/x.md") || re.MatchString(strings.ReplaceAll(odd, ".k", "Xk")+"/x.md") {
		t.Errorf("sbQuoteMeta(%q) = %q doesn't match only itself", odd, sbQuoteMeta(odd))
	}
}

// SA-M2: sockets, services, signals and network, on the made-up Mac.

// m2Ops are the operations SA-M2's sections name.
var m2Ops = []string{"network-outbound", "mach-lookup", "iokit-open", "signal"}

// What the desk may reach comes after what is denied by default, every allow of an operation after its default deny,
// and what nothing opens after every allow (SIERRA's order rule: last match wins). Each operation is named exactly,
// by its allows and its denies alike, never by a wildcard a named one would beat (sbpl.go). Services add lines to
// SA-M2's sections only: the file rules' are the same with an empty Services, in every column.
func TestTheServicesComeAfterTheirDefaultsAndWhatNothingOpensComesLast(t *testing.T) {
	for _, s := range profileSetups() {
		p := mustNew(t, s.pl)
		sections, err := p.sections(goldenServices(s.pl))
		if err != nil {
			t.Fatal(err)
		}
		var lines []sbRule
		for _, section := range sections {
			lines = append(lines, section.rules...)
		}
		firstDeny, lastAllow := map[string]int{}, -1
		for i, r := range lines {
			for _, op := range r.ops {
				if !r.allow {
					if _, ok := firstDeny[op]; !ok {
						firstDeny[op] = i
					}
				} else if slices.Contains(m2Ops, op) {
					if at, ok := firstDeny[op]; !ok || at > i {
						t.Errorf("%s: %s comes before %s's default deny", s.name, r, op)
					}
				}
				if strings.HasSuffix(op, "*") && slices.Contains(m2Ops, strings.TrimSuffix(op, "*")) {
					t.Errorf("%s: %s names a wildcard", s.name, r)
				}
			}
			if r.allow {
				lastAllow = i
			}
		}
		if finalDenies := len(lines) - len(sections[5].rules) - len(sections[6].rules); lastAllow >= finalDenies {
			t.Errorf("%s: %s comes after the final denies began", s.name, lines[lastAllow])
		}
		for i, r := range sbDefaults {
			if sections[0].rules[i].String() != r.String() || r.allow || r.ops[0] != m2Ops[i] {
				t.Errorf("%s: default %d is %s", s.name, i, sections[0].rules[i])
			}
		}
		if got := sections[1].rules[len(sections[1].rules)-1].String(); got != "(allow signal (target same-sandbox))" {
			t.Errorf("%s: the signal allow is %s", s.name, got)
		}
		if strings.Contains(fmtRules(lines), "(target others)") {
			t.Errorf("%s: a signal rule names (target others), which doesn't hold on macOS 26", s.name)
		}
		bare, err := p.sections(Services{})
		if err != nil {
			t.Fatal(err)
		}
		for i := 2; i <= 5; i++ {
			if fmtRules(bare[i].rules) != fmtRules(sections[i].rules) {
				t.Errorf("%s: the Services change the file rules' section %q", s.name, sections[i].heading)
			}
		}
		if fmtRules(bare[1].rules) != "(allow signal (target same-sandbox))\n" || fmtRules(bare[6].rules) != fmtRules(sbNever) {
			t.Errorf("%s: an empty Services allows %q and denies last %q", s.name, fmtRules(bare[1].rules), fmtRules(bare[6].rules))
		}
	}
}

func fmtRules(rules []sbRule) string {
	var s strings.Builder
	for _, r := range rules {
		s.WriteString(r.String() + "\n")
	}
	return s.String()
}

// A desk connects to a unix socket only where its profile names it: its own socket, the desk-start socket and the
// sockets on its list, and the agents' only while Remote Login doesn't answer. Never to one its Policy denies (the
// support folder's, the owner's tap), another desk's socket, or one not named. Played as Seatbelt reads the whole
// profile, in every column, with Remote Login answering and not.
func TestADeskConnectsOnlyToItsOwnSocketTheDeskStartAndItsList(t *testing.T) {
	type column struct {
		name string
		pl   Places
	}
	var columns []column
	for _, s := range profileSetups() {
		columns = append(columns, column{s.name, s.pl})
		off := s.pl
		off.RemoteLogin = false
		columns = append(columns, column{s.name + ", Remote Login off", off})
	}
	checked := 0
	for _, c := range columns {
		p, svc := mustNew(t, c.pl), goldenServices(c.pl)
		play := playWith(t, p, svc)
		named := []string{svc.OwnSocket, c.pl.DeskStart}
		named = append(named, svc.Sockets...)
		if !c.pl.RemoteLogin {
			named = append(named, svc.Agents...)
		}
		var paths []string
		s := setup{c.name, c.pl}
		for _, r := range rows {
			paths = append(paths, s.path(r.path))
		}
		paths = append(paths, svc.Sockets...)
		paths = append(paths, svc.Agents...)
		paths = append(paths, svc.OwnSocket, c.pl.DeskStart, c.pl.Support+"/serve.sock", sockets+"/h2/s", sockets+"/gm1/s", sockets+"/h1/t",
			sockets+"/s", "/private/var/run/mDNSResponderX", "/private/tmp/.s.PGSQL.5432", "/var/run/docker.sock", home+"/.ssh/agent.sock")
		for _, at := range paths {
			want := Deny
			if decided, _ := p.Decide(at, Connect); decided == Allow && slices.ContainsFunc(named, func(n string) bool { return key("darwin", n) == key("darwin", at) }) {
				want = Allow
			}
			checked++
			if got := play.says(at, sbCheck{"network-outbound", ""}); got != want {
				t.Errorf("%s: connect %s: the profile says %v, want %v", c.name, at, got, want)
			}
		}
		for _, at := range []string{svc.OwnSocket, "/private/var/run/syslog", c.pl.DeskStart} {
			if got := play.says(at, sbCheck{"network-outbound", ""}); got != Allow {
				t.Errorf("%s: %s is refused", c.name, at)
			}
		}
		for _, at := range svc.Agents {
			if got, want := play.says(at, sbCheck{"network-outbound", ""}), map[bool]Verdict{true: Deny, false: Allow}[c.pl.RemoteLogin]; got != want {
				t.Errorf("%s: the agent %s: %v, want %v", c.name, at, got, want)
			}
		}
	}
	if checked < 1000 {
		t.Errorf("only %d cases checked", checked)
	}
}

// While Remote Login answers, the agents are denied after every allow, as the ssh keys are (C5, AQ12): no list and no
// place opens them. While it doesn't, they are on the list like any socket, and nothing denies them last.
func TestTheAgentsAreDeniedLastOnlyWhileRemoteLoginAnswers(t *testing.T) {
	for _, on := range []bool{true, false} {
		pl := hirePlaces(realSupport)
		pl.RemoteLogin = on
		svc := goldenServices(pl)
		sections, err := mustNew(t, pl).sections(svc)
		if err != nil {
			t.Fatal(err)
		}
		reach, last := fmtRules(sections[1].rules), fmtRules(sections[6].rules)
		for _, agent := range svc.Agents {
			line := `network-outbound (literal "` + spell("darwin", agent) + `"))`
			if on && (!strings.Contains(last, "(deny "+line) || strings.Contains(reach, line)) {
				t.Errorf("Remote Login on: %s isn't denied last only:\n%s\n%s", agent, reach, last)
			}
			if !on && (!strings.Contains(reach, "(allow "+line) || strings.Contains(last, line)) {
				t.Errorf("Remote Login off: %s isn't allowed only:\n%s\n%s", agent, reach, last)
			}
		}
	}
}

// Each local TCP port is denied last on every address of this Mac over IPv4 and on any address over IPv6, so an
// IPv4-mapped address (::ffff:127.0.0.1) is refused too (RT126SAM2 B1, SIERRA's ruling): two lines a port, in its
// list's order, and never `(remote tcp "localhost:N")`, which Seatbelt reads wrong next to a tcp6 line (sbpl.go). A
// port AQ21's opt-in lifts has neither line; a port that can't be one fails the profile.
func TestTheLocalPortsAreDeniedLastOnLocalhostAndAnyIPv6(t *testing.T) {
	for _, s := range profileSetups() {
		svc := goldenServices(s.pl)
		p := mustNew(t, s.pl)
		sections, err := p.sections(svc)
		if err != nil {
			t.Fatal(err)
		}
		var want string
		for _, port := range svc.DeniedPorts {
			want += fmt.Sprintf("(deny network-outbound (remote tcp4 \"localhost:%d\"))\n(deny network-outbound (remote tcp6 \"*:%d\"))\n", port, port)
		}
		if got := fmtRules(sections[6].rules[:2*len(svc.DeniedPorts)]); got != want {
			t.Errorf("%s: the ports:\n%s\nwant\n%s", s.name, got, want)
		}
		var all []sbRule
		for _, section := range sections {
			all = append(all, section.rules...)
		}
		if text := fmtRules(all); strings.Contains(text, "(remote tcp ") || strings.Count(text, "(remote tcp") != 2*len(svc.DeniedPorts) {
			t.Errorf("%s: a port is named outside its two lines:\n%s", s.name, text)
		}
		// AQ21's opt-in: the databases' ports are left off the list, and both their lines go.
		svc.DeniedPorts = []int{22, 2375, 2376}
		profile, err := p.Profile(svc)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(profile, ":5432") || strings.Contains(profile, ":3306") || strings.Count(profile, "(remote tcp") != 6 {
			t.Errorf("%s: with the databases opted in, the profile still names them", s.name)
		}
	}
}

// Only SA-M2's own rules are written as they are (sbRaw); every file rule names its filter, so one made with no kind
// can't be written unquoted (RT126SAM2 N4).
func TestOnlySAM2sOwnRulesAreWrittenRaw(t *testing.T) {
	own := map[string]bool{sbSameSandbox.String(): true}
	for _, r := range slices.Concat(sbDefaults, sbNever) {
		own[r.String()] = true
	}
	for _, s := range profileSetups() {
		p := mustNew(t, s.pl)
		b, err := p.fileRules()
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range slices.Concat(b.plain, b.wide, b.exceptions, b.final) {
			if r.place.kind == sbRaw {
				t.Errorf("%s: the file rule %s has no kind", s.name, r)
			}
		}
		sections, err := p.sections(goldenServices(s.pl))
		if err != nil {
			t.Fatal(err)
		}
		raw := 0
		for _, section := range sections {
			for _, r := range section.rules {
				if r.place.kind == sbRaw {
					raw++
					if !own[r.String()] {
						t.Errorf("%s: %s is written raw", s.name, r)
					}
				}
			}
		}
		if raw != len(own) {
			t.Errorf("%s: %d raw rules, want SA-M2's %d", s.name, raw, len(own))
		}
	}
}

// What a profile can't say exactly, or would open another desk's server with, fails it: the desk doesn't start. Its
// own socket must be in its own socket folder (not the folder, its parent or another desk's); no listed socket or
// agent may be in the socket folder; every socket is whole and clean; a Mach service, an IOKit class and a port must
// be ones. A Policy with no socket folder of its own takes no own socket.
func TestServicesAProfileCantSayAreRefused(t *testing.T) {
	pl := hirePlaces(realSupport)
	p := mustNew(t, pl)
	good := goldenServices(pl)
	for name, c := range map[string]struct {
		change func(*Services)
		want   error
	}{
		"its own socket folder":            {func(s *Services) { s.OwnSocket = sockets + "/h1" }, ErrOwnSocket},
		"the socket folder":                {func(s *Services) { s.OwnSocket = sockets + "/s" }, ErrOwnSocket},
		"another desk's socket as its own": {func(s *Services) { s.OwnSocket = sockets + "/h2/s" }, ErrOwnSocket},
		"its own socket elsewhere":         {func(s *Services) { s.OwnSocket = home + "/s" }, ErrOwnSocket},
		"its own socket, relative":         {func(s *Services) { s.OwnSocket = "s" }, ErrListed},
		"another desk's socket listed":     {func(s *Services) { s.Sockets = append(s.Sockets, sockets+"/h2/s") }, ErrListed},
		"another desk's socket, in case":   {func(s *Services) { s.Sockets = append(s.Sockets, "/tmp/SMOLCORP-501/h2/s") }, ErrListed},
		"its own socket listed":            {func(s *Services) { s.Sockets = append(s.Sockets, sockets+"/h1/s") }, ErrListed},
		"an agent in the socket folder":    {func(s *Services) { s.Agents = append(s.Agents, sockets+"/gm1/s") }, ErrListed},
		"a socket with ..":                 {func(s *Services) { s.Sockets = append(s.Sockets, "/private/var/run/../tmp/x") }, ErrListed},
		"a relative socket":                {func(s *Services) { s.Sockets = append(s.Sockets, "var/run/syslog") }, ErrListed},
		"no socket":                        {func(s *Services) { s.Agents = append(s.Agents, "") }, ErrListed},
		"a socket with a newline":          {func(s *Services) { s.Sockets = append(s.Sockets, "/private/tmp/a\nb") }, ErrListed},
		"no Mach service":                  {func(s *Services) { s.Mach = append(s.Mach, "") }, ErrListed},
		"a Mach service with a control":    {func(s *Services) { s.Mach = append(s.Mach, "com.apple.x\x00") }, ErrListed},
		"no IOKit class":                   {func(s *Services) { s.IOKit = append(s.IOKit, IOKitOpen{}) }, ErrListed},
		"port 0":                           {func(s *Services) { s.DeniedPorts = append(s.DeniedPorts, 0) }, ErrListed},
		"port 65536":                       {func(s *Services) { s.DeniedPorts = append(s.DeniedPorts, 65536) }, ErrListed},
	} {
		svc := goldenServices(pl)
		svc.Sockets, svc.Agents, svc.Mach, svc.IOKit, svc.DeniedPorts = slices.Clone(good.Sockets), slices.Clone(good.Agents),
			slices.Clone(good.Mach), slices.Clone(good.IOKit), slices.Clone(good.DeniedPorts)
		c.change(&svc)
		if _, err := p.Profile(svc); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
	noFolder := pl
	noFolder.OwnSockets = ""
	if _, err := mustNew(t, noFolder).Profile(Services{OwnSocket: sockets + "/h1/s"}); !errors.Is(err, ErrOwnSocket) {
		t.Errorf("an own socket with no own socket folder: %v", err)
	}
	if _, err := mustNew(t, noFolder).Profile(); err != nil {
		t.Errorf("no own socket and no folder: %v", err)
	}
	if _, err := p.Profile(good); err != nil {
		t.Errorf("the golden Services: %v", err)
	}
	// No Services is the empty one: what the pieces written before SA-M2 call. It denies no TCP port (RT126SAM2 N1), so
	// it isn't the strictest profile. Two are refused.
	none, _ := p.Profile()
	if empty, _ := p.Profile(Services{}); none != empty || !strings.Contains(none, "(deny mach-lookup)") || strings.Contains(none, "(remote tcp") {
		t.Error("a profile with no Services isn't the one with an empty Services")
	}
	if _, err := p.Profile(good, good); !errors.Is(err, ErrListed) {
		t.Errorf("two Services: %v", err)
	}
}
