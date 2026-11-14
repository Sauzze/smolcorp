package sandbox

import (
	"errors"
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

// The profile is the golden text, byte for byte. To write it again after a change to the Policy or the profile, run
// the test with SMOLCORP_SBPL_GOLDEN=write, and read the diff.
func TestTheMacProfileIsTheGoldenText(t *testing.T) {
	for _, s := range profileSetups() {
		profile, err := mustNew(t, s.pl).Profile()
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

// profileSections is a profile's lines, section by section, in Profile's order: the plain allows, the Wide denies,
// the exceptions, the final denies.
func profileSections(profile string) [][]string {
	var sections [][]string
	for _, section := range strings.Split(profile, "\n;; ")[1:] {
		sections = append(sections, strings.Split(strings.TrimSpace(section), "\n")[1:])
	}
	return sections
}

// profileWithout is p's profile before the rows drop picks were in the Policy: the other rows, and the rename guards
// made again from them alone, as New would make them.
func profileWithout(t *testing.T, p *Policy, drop func(Rule) bool) string {
	t.Helper()
	b := &builder{goos: p.goos}
	for _, r := range p.rules {
		if r.Row != rowAbove && !drop(r) {
			b.rules = append(b.rules, r)
		}
	}
	b.denyAbove()
	profile, err := (&Policy{goos: p.goos, rules: b.rules}).Profile()
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
		p := mustNew(t, s.pl)
		profile, err := p.Profile()
		if err != nil {
			t.Fatal(err)
		}
		if again := profileWithout(t, p, func(Rule) bool { return false }); again != profile {
			t.Fatalf("%s: the Policy made again without dropping a row isn't the same profile", s.name)
		}
		after := profileSections(profile)

		// NS1: one line more, and only that.
		night := `(allow file-read-data file-read-xattr (literal "` + s.pl.Support + `/nightshift.json")) ; the night shift's jobs: read only, as the guard file`
		before := profileSections(profileWithout(t, p, func(r Rule) bool { return r.Row == "the night shift's jobs: read only, as the guard file" }))
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
		before = profileSections(profileWithout(t, p, gitcfg1))
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
	}
}

// The desk's own status and report stay its own to write in every column (STUDIO34: the *.md row is still there
// once NS1's line is in), as an exception to the support folder's Wide deny.
func TestTheDesksOwnStatusAndReportStayWritableInEveryProfile(t *testing.T) {
	for _, s := range profileSetups() {
		profile, err := mustNew(t, s.pl).Profile()
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
	play := seatbeltPlay{rules: slices.Concat(b.plain, b.wide, b.exceptions, b.final), res: map[string]*regexp.Regexp{}}
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
	}
	return s.res[r.place.text].MatchString(k)
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
// plays it: the plain allows, the Wide denies, the exceptions, the final denies. Its sections say so in its text, and
// each heading comes once, in that order, after the header.
func TestTheMacProfilesSectionsAreInTheDocumentedOrder(t *testing.T) {
	profile, err := mustNew(t, hirePlaces(realSupport)).Profile()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(profile, ";; A desk's sandbox") || !strings.Contains(profile, "\n(version 1)\n(allow default)\n") {
		t.Errorf("the header: %q", profile[:min(len(profile), 200)])
	}
	last := -1
	for _, heading := range []string{"Allowed places", "Denied places an exception opens", "The exceptions", "Denied places nothing opens"} {
		at := strings.Index(profile, "\n;; "+heading+"\n")
		if at <= last || strings.Count(profile, "\n;; "+heading+"\n") != 1 {
			t.Errorf("%q is at %d, after %d", heading, at, last)
		}
		last = at
	}
	// Each section holds only its kind of line.
	sections := strings.Split(profile, "\n;; ")[1:]
	for i, want := range []string{"(allow ", "(deny ", "(allow ", "(deny "} {
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
