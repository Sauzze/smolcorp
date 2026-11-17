package guard

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
)

// asWindowsPaths reads the place's paths as Windows' for one test (windowsPaths), on whatever runs it.
func asWindowsPaths(t *testing.T, on bool) {
	t.Helper()
	was := windowsPaths
	windowsPaths = on
	t.Cleanup(func() { windowsPaths = was })
}

// FXGUARDWIN (ns0ar2 on windows-latest): a path in a git folder not called .git (a worktree's gitdir: target, a bare
// repository) is found on Windows too. Its folders are Windows paths there, with \ between their parts, and a path
// read from the call's folder is compared with them as Windows reads both. Pure path logic: these run everywhere.
func TestAGitFolderIsFoundByItsWindowsPathsToo(t *testing.T) {
	asWindowsPaths(t, true)
	tmp := `C:\Users\RUNNER~1\AppData\Local\Temp\TestAGitFolderIsWrittenOnlyByGit1`
	tree := &Place{Known: true, cwd: tmp + `\002`, gitDir: tmp + `\003\store`, common: tmp + `\003\store`}
	bare := &Place{Known: true, cwd: tmp + `\004\src.git`, gitDir: tmp + `\004\src.git`, common: tmp + `\004\src.git`}
	otherDrive := &Place{Known: true, cwd: `C:\work\tree`, gitDir: `D:\stores\store`, common: `D:\stores\store`}
	share := &Place{Known: true, cwd: `\\server\share\repos\tree`, gitDir: `\\server\share\repos\store`, common: `\\server\share\repos\store`}
	polish := &Place{Known: true, cwd: `C:\Users\Łukasz\proj\tree`, gitDir: `C:\Users\Łukasz\proj\store`, common: `C:\Users\Łukasz\proj\store`}
	french := &Place{Known: true, cwd: `C:\Users\Hélène\proj\tree`, gitDir: `C:\Users\Hélène\proj\store`, common: `C:\Users\Hélène\proj\store`}
	unread := &Place{Known: true, cwd: `C:\w\tree`, gitDir: `store`, common: `store`} // a git folder Windows' reading can't place
	long := `C:\Users\runneradmin\AppData\Local\Temp\TestAGitFolderIsWrittenOnlyByGit1`
	in := map[*Place][]string{
		tree: {"../003/store/HEAD", `..\003\store\HEAD`, `..\003\STORE\refs\heads\main`, "../003/store", "./x/../../003/store/packed-refs",
			tmp + `\003\store\HEAD`, strings.ToLower(strings.ReplaceAll(tmp, `\`, "/")) + "/003/store/HEAD",
			long + `\003\store\HEAD`, // the long name where the place has the short one
			`\Users\RUNNER~1\AppData\Local\Temp\TestAGitFolderIsWrittenOnlyByGit1\003\store\HEAD`,   // the call's drive
			"/c/Users/RUNNER~1/AppData/Local/Temp/TestAGitFolderIsWrittenOnlyByGit1/003/store/HEAD", // Git Bash's
			`\\?\` + tmp + `\003\store\HEAD`, `\\.\` + tmp + `\003\store\HEAD`,
			"../003/store./HEAD", "../003/store. ./HEAD", "../003/store::$INDEX_ALLOCATION/HEAD", "../003/STORE~1/HEAD",
			`C:..\003\store\HEAD`, // a drive alone, the call's
			"of=../003/store/HEAD", "--output=../003/store/config",
			// FXGUARDWINR1, B1: Win32's NT spelling, and device namespaces the guard can't place (they fail closed).
			`\??\` + tmp + `\003\store\HEAD`, `\??\` + long + `\003\STORE~1\HEAD`, `\\?\GLOBALROOT\??\` + tmp + `\003\store\HEAD`,
			`\\?\Volume{0}\x`, `\\.\HarddiskVolume3\x`, `\\?\GLOBAL??\C:\x`, `\\?\C:`},
		bare:       {"refs/heads/main", "packed-refs", "HEAD", `.\config`, `hooks\pre-commit`, "objects/ab/cd"},
		otherDrive: {`D:\stores\store\HEAD`, `d:/STORES/store/refs/heads/main`, "D:HEAD", "/d/stores/store/HEAD"},
		share: {`..\store\HEAD`, `\\SERVER\share\repos\store\HEAD`, `\\?\UNC\server\share\repos\store\HEAD`, `\repos\store\HEAD`,
			`\??\UNC\server\share\repos\store\HEAD`},
		// B2: the user's folder under a short name Windows may have written for it.
		polish: {`C:\Users\_UKASZ~1\proj\store\HEAD`, `C:\Users\UKASZ~1\proj\store\HEAD`, `C:\Users\LUKASZ~1\proj\store\HEAD`,
			`..\..\..\_UKASZ~1\proj\store\HEAD`},
		french: {`C:\Users\HÉ1A2B~1\proj\store\HEAD`, `C:\Users\H_L_NE~1\proj\store\HEAD`, `\Users\HELENE~1\proj\store\HEAD`},
		unread: {"x", `D:\x`}, // fails closed
	}
	out := map[*Place][]string{
		tree: {"../003/store2/HEAD", "../003/storex/HEAD", "../003/sto/HEAD", "../003/HEAD", "../store/HEAD", "x", "a/b", "HEAD~1",
			"HEAD~1:f", "../003/ab~1/HEAD", `D:\Users\RUNNER~1\AppData\Local\Temp\TestAGitFolderIsWrittenOnlyByGit1\003\store\HEAD`,
			"D:HEAD", `\Temp\003\store\HEAD`, "/d/Users/RUNNER~1/AppData/Local/Temp/TestAGitFolderIsWrittenOnlyByGit1/003/store/HEAD",
			`\\server\share\003\store\HEAD`, "-o", "~/x", `\??\UNC\server\share\x`, `\??\D:\x\003\store\HEAD`},
		bare:       {"../other/HEAD", "../src.git2/HEAD"},
		otherDrive: {`C:\stores\store\HEAD`, "x", "../store/HEAD", "C:HEAD"},
		share:      {`..\store2\HEAD`, `\\server\other\repos\store\HEAD`, `C:\repos\store\HEAD`, `\??\UNC\server\other\store\HEAD`},
		polish:     {`C:\Users\_UKASZ~1\proj\store2\HEAD`, `C:\Users\Other\proj\store\HEAD`, `x`, `\??\D:\Users\Łukasz\proj\store\HEAD`},
		french:     {`C:\Users\HÉ1A2B~1\proj\tree\HEAD`, `..\tree2\x`},
	}
	for p, words := range in {
		for _, w := range words {
			if !gitFolderPath(w, p) {
				t.Errorf("from %s: %q isn't in %s", p.cwd, w, p.gitDir)
			}
		}
	}
	for p, words := range out {
		for _, w := range words {
			if gitFolderPath(w, p) {
				t.Errorf("from %s: %q is in %s", p.cwd, w, p.gitDir)
			}
		}
	}
	// The ns0ar2 rows, through Check, with the place as Windows has it.
	groups := At(WithGates(DefaultGroups()), *tree)
	for _, line := range []string{"cp x ../003/store/HEAD", `cp x '..\003\store\HEAD'`, "echo x > ../003/store/packed-refs",
		`cp x '\??\` + tmp + `\003\store\HEAD'`, `cp x '\\?\GLOBALROOT\??\` + tmp + `\003\store\HEAD'`} {
		if r := Check(line, groups); r == nil || !strings.Contains(r.Why, "git folder") {
			t.Errorf("%q: %v", line, r)
		}
	}
	for _, line := range []string{"cp a b", "cp x ../003/store2/HEAD"} {
		if r := Check(line, groups); r != nil {
			t.Errorf("%q: %v", line, r)
		}
	}
}

// On a Mac and Linux a path is read as it was: \ is a / (as the guard reads .git), and store. or STORE~1 are other
// names there.
func TestAGitFolderIsFoundByItsMacAndLinuxPathsAsBefore(t *testing.T) {
	asWindowsPaths(t, false)
	p := &Place{Known: true, cwd: "/home/u/t/002", gitDir: "/home/u/t/003/store", common: "/home/u/t/003/store"}
	for w, want := range map[string]bool{"../003/store/HEAD": true, `..\003\store\HEAD`: true, "/home/u/t/003/store/refs": true,
		"../003/STORE/HEAD": true, "../003/store": true, "../003/store2/HEAD": false, "../003/store./HEAD": false,
		"../003/STORE~1/HEAD": false, "/home/u/t/003/HEAD": false, "x": false} {
		if got := gitFolderPath(w, p); got != want {
			t.Errorf("%q: %v", w, got)
		}
	}
}

// A short name is read as Windows makes one, so a git revision (HEAD~1) is no short name of .git.
func TestAWindowsShortNameIsReadAsWindowsMakesOne(t *testing.T) {
	for _, c := range []struct {
		short, long string
		want        bool
	}{
		{"store~1", "store", true}, {"runner~1", "runneradmin", true}, {"src~1.git", "src.git", true}, {"progra~2", "program files", true},
		{"st1a2b~1", "storehouse", true}, {"a_b~1", "a+b", true}, {"git~1", ".git", true}, {"mydocu~1.txt", "my.documents.txt", true},
		{"head~1", ".git", false}, {"head~1", "store", false}, {"store~1", "stor", false}, {"src~1.gi", "src.txt", false},
		{"store", "store", false}, {"store~x", "store", false}, {"toolongx~1", "toolongxname", false},
		// FXGUARDWINR1, B2: a long name with a letter outside ASCII takes any short name (Windows writes the letter as
		// _, drops it or writes one like it, by the computer's code page), and Windows drops the spaces in an
		// extension too.
		{"_ukasz~1", "łukasz", true}, {"ukasz~1", "łukasz", true}, {"lukasz~1", "łukasz", true}, {"hé1a2b~1", "hélène", true},
		{"h_l_ne~1", "hélène", true}, {"x~1", "émile", true}, {"x~1.yz", "x.y z", true}, {"ée1a2b~1", "éeee.txt", true},
		{"head~1", "łukasz", true}, // an over-match, which is safe: HEAD~1 against .git stays out (above)
		{"ukasz~1", "lukasz", false}, {"x~1.yz", "x.yw", false},
	} {
		if got := winShortFor(c.short, c.long); got != c.want {
			t.Errorf("%s for %s: %v", c.short, c.long, got)
		}
	}
}

// foldBefore is Fold as it was before its ASCII path (FXGUARDWIN): the full folds, then each letter's least.
func foldBefore(text string) string {
	return strings.Map(func(r rune) rune {
		least := r
		for other := unicode.SimpleFold(r); other != r; other = unicode.SimpleFold(other) {
			least = min(least, other)
		}
		return unicode.ToLower(least)
	}, fullFolds.Replace(text))
}

// FXGUARDWIN's speed-ups read every word as before: Fold, the always names, tmux's short forms, a skeleton's
// letters, git's variable names and the merge group's rules.
func TestTheGuardsFasterPathsReadAsBefore(t *testing.T) {
	var words []string
	for c := 0; c < 0x80; c++ {
		words = append(words, string(rune(c)), "x"+string(rune(c))+"Y")
	}
	words = append(words, "", "Phones.JSON", "GM-TOKEN", "disableAllHooks", "\u212a", "\u017f", "Stra\u00dfe", "\ufb01le", "ı", "A\u0301",
		"SMOLCORP_GM_TOKEN=1", "MiXeD/CaSe\\Path", "\xff")
	for _, w := range words {
		if got, want := Fold(w), foldBefore(w); got != want {
			t.Errorf("Fold(%q) = %q, was %q", w, got, want)
		}
	}
	for _, w := range words {
		want := ""
		for _, name := range always {
			if strings.Contains(foldBefore(w), foldBefore(name)) {
				want = name
				break
			}
		}
		if got := namesAlways(w); got != want {
			t.Errorf("namesAlways(%q) = %q, was %q", w, got, want)
		}
	}

	// tmux: every start of every command, and a few words besides.
	tmuxWords := []string{"", "-t", "a", "zz", "kill-", "send-k", "kill-ses", "attach", "new"}
	for _, c := range tmuxCommands {
		for n := 1; n <= len(c); n++ {
			tmuxWords = append(tmuxWords, c[:n])
		}
	}
	for alias := range tmuxAliases {
		tmuxWords = append(tmuxWords, alias)
	}
	got := tmuxFull(tmuxWords)
	for i, a := range tmuxWords {
		want := a
		if full, ok := tmuxAliases[a]; ok && !strings.HasPrefix(a, "-") && a != "" {
			want = full
		} else if !strings.HasPrefix(a, "-") && a != "" {
			found, n := "", 0
			for _, c := range tmuxCommands {
				if c == a {
					found, n = c, 1
					break
				}
				if strings.HasPrefix(c, a) {
					found, n = c, n+1
				}
			}
			if n == 1 {
				want = found
			}
		}
		if got[i] != want {
			t.Errorf("tmuxFull(%q) = %q, was %q", a, got[i], want)
		}
	}

	// A skeleton's letters against every target the guard keeps.
	var targets []skTarget
	targets = append(targets, gitFolderTargets...)
	targets = append(targets, dirTargetsFolded...)
	targets = append(targets, keptTargets.support...)
	targets = append(targets, keptTargets.supportPart...)
	for _, list := range [][]namedTarget{keptTargets.files[0], keptTargets.files[1], keptTargets.files[3], keptTargets.below[0],
		keptTargets.below[1], keptTargets.below[3], keptTargets.part, keptTargets.folder} {
		for _, n := range list {
			targets = append(targets, n.tg)
		}
	}
	for _, w := range []string{"*", ".g*", "gm-t*", "ph*s.json", "s*-k?y", ".c*/s*", "a[", "{a,b}", "hire-g?ard.json", "*it/refs",
		"Lib*/App*/A*", "x^y", "gm-mem*", "paste-h*", "*.json"} {
		sk := skeletonOf(w, nil)
		for i := range targets {
			have := 0
			for r, n := range targets[i].letters {
				have += min(n, sk.letters[r])
			}
			want := have >= targets[i].min && !(targets[i].hidden && !sk.dot)
			if got := sk.could(&targets[i]); got != want {
				t.Errorf("%q could fill target %d: %v, was %v", w, i, got, want)
			}
		}
	}

	// git's variable names, every one of up to four bytes from these letters (ı and ſ are I and S in upper case).
	letters := []string{"p", "a", "g", "e", "r", "_", "ı", "ſ", "s", "k"}
	names := []string{"PAGER", "pager", "Editor", "git_ssh", "SSH_ASKPASS", "x_askpaſs", "GIT_CONFIG_KEY_0", "pagerx"}
	var grow func(prefix string)
	grow = func(prefix string) {
		if len(prefix) > 4 {
			return
		}
		names = append(names, prefix)
		for _, l := range letters {
			grow(prefix + l)
		}
	}
	grow("")
	for _, n := range names {
		u := strings.ToUpper(n)
		want := gitRunVars[u] || strings.HasSuffix(u, "ASKPASS") || strings.HasPrefix(u, "GIT_CONFIG_KEY_") ||
			oneOf("GIT_CONFIG_PARAMETERS|GIT_CONFIG_GLOBAL|GIT_CONFIG_SYSTEM|GIT_CONFIG|GIT_ALLOW_PROTOCOL", u)
		if got := gitVarName(n); got != want {
			t.Errorf("gitVarName(%q) = %v, was %v", n, got, want)
		}
	}

	// A word the revision check is skipped for (no ^, ~ or #) is never a pattern it would have spent the budget on.
	for _, w := range []string{"main", "origin/main", "a..b", "a...b", "x:y", "@", "v1.2.3", "a-b_c", "HEAD:f", "abc1234"} {
		if gitRevision.MatchString(w) && isWild(w, nil) {
			t.Errorf("%q", w)
		}
	}

	// The merge group's rules.
	for _, groups := range [][]Group{WithGates(DefaultGroups()), ForGM(WithGates(DefaultGroups()))} {
		for _, g := range groups {
			for _, r := range append(slices.Clone(g.Commands), "git x@base", "@base", "x @basey") {
				if got, want := isBaseRule(r), slices.Contains(ruleWords(r), baseRule); got != want {
					t.Errorf("isBaseRule(%q) = %v, was %v", r, got, want)
				}
			}
		}
	}
}

// FXGUARDWINR1, P1: with a place known and Windows' paths, the guard reads its folders once for a line, not for every
// word and folder: a 1 MB cp line and a line of redirections stay as fast as A2 asks (TestNoInputMakesTheGuardSlow).
func TestAKnownPlaceOnWindowsDoesntMakeTheGuardSlow(t *testing.T) {
	asWindowsPaths(t, true)
	tmp := `C:\Users\RUNNER~1\AppData\Local\Temp\TestAKnownPlaceOnWindowsDoesntMakeTheGuardSlow1`
	groups := At(WithGates(DefaultGroups()), Place{Known: true, cwd: tmp + `\002`, gitDir: tmp + `\003\store`, common: tmp + `\003\store`})
	big := 1 << 20
	for name, line := range map[string]string{
		"cp":        "cp " + strings.Repeat("a ", big/2),
		"redirects": strings.Repeat(">a ", big/3),
	} {
		start := time.Now()
		Check(line, groups)
		if took := time.Since(start); took > 5*time.Second*raceSlowdown {
			t.Errorf("%s: took %v", name, took)
		}
	}
	// And still refused when a word is in the git folder, at its end.
	if r := Check("cp "+strings.Repeat("a ", big/2)+"../003/store/HEAD", groups); r == nil || !strings.Contains(r.Why, "git folder") {
		t.Errorf("a long cp line into the git folder: %v", r)
	}
}

// RT126GUARDWIN's M4: FXGUARDWIN's skip of the revision check lists ^, ~ and #, the marks the loop after it flags. A #
// in a word is still a zsh pattern in bash's reading (cat a#), refused as before.
func TestAHashInAWordIsStillAZshPattern(t *testing.T) {
	for _, line := range []string{"cat a#", "ls x#y", "ls *#", "cat a^b", "ls a~b"} {
		if r := Check(line, DefaultGroups()); r == nil || r.Why != plainGlob {
			t.Errorf("%q: %v", line, r)
		}
	}
	for _, line := range []string{"echo $#", "git log HEAD~1", "git show main^", "ls #x", "cat a # x"} {
		if r := Check(line, DefaultGroups()); r != nil {
			t.Errorf("%q: %v", line, r)
		}
	}
}

// RT126GUARDWINR2's follow-up 1: after \??\, as after \\?\ and \\.\, a namespace the guard can't place fails closed,
// \??\GLOBALROOT\??\C:\…\store\HEAD among them.
func TestANamespaceAfterTheNTPrefixFailsClosedToo(t *testing.T) {
	asWindowsPaths(t, true)
	tmp := `C:\Users\RUNNER~1\AppData\Local\Temp\TestANamespaceAfterTheNTPrefixFailsClosedToo1`
	tree := Place{Known: true, cwd: tmp + `\002`, gitDir: tmp + `\003\store`, common: tmp + `\003\store`}
	for _, w := range []string{`\??\GLOBALROOT\??\` + tmp + `\003\store\HEAD`, `\??\GLOBALROOT\x`, `\??\Volume{0}\x`,
		`\??\HarddiskVolume3\x`, `\??\GLOBAL??\C:\x`, `\??\C:`, "/??/globalroot/x", `of=\??\GLOBALROOT\x`} {
		if !gitFolderPath(w, &tree) {
			t.Errorf("%q isn't in %s", w, tree.gitDir)
		}
	}
	for _, w := range []string{`\??\GLOBALROOT\x`, `\??\Volume{0}\x`} {
		if a, ok := winPath(Fold(w), nil); !ok || !a.anywhere {
			t.Errorf("winPath(%q) = %+v, %v", w, a, ok)
		}
	}
	groups := At(WithGates(DefaultGroups()), tree)
	for _, line := range []string{`cp x '\??\GLOBALROOT\??\` + tmp + `\003\store\HEAD'`, `cp x '\??\GLOBALROOT\x'`} {
		if r := Check(line, groups); r == nil || !strings.Contains(r.Why, "git folder") {
			t.Errorf("%q: %v", line, r)
		}
	}
}

// RT126GUARDWINR2's follow-up 2: FXGUARDWINR1's P1 as it is built, not only as fast as A2 asks. At reads a place's
// folders once for a line, and every group shares that reading; gitFolderPath uses it and doesn't read the place again.
func TestAKnownPlacesFoldersAreReadOnceForALine(t *testing.T) {
	for _, c := range []struct {
		name       string
		windows    bool
		here, away Place
		hereIn     string // in here's git folder
		awayIn     string // in away's
	}{
		{"windows", true,
			Place{Known: true, cwd: `C:\w\here\tree`, gitDir: `C:\w\here\store`, common: `C:\w\here\store`},
			Place{Known: true, cwd: `D:\x\away\tree`, gitDir: `D:\x\away\store`, common: `D:\x\away\store`},
			`C:\w\here\store\HEAD`, `D:\x\away\store\HEAD`},
		{"mac and linux", false,
			Place{Known: true, cwd: "/w/here/tree", gitDir: "/w/here/store", common: "/w/here/store"},
			Place{Known: true, cwd: "/x/away/tree", gitDir: "/x/away/store", common: "/x/away/store"},
			"/w/here/store/HEAD", "/x/away/store/HEAD"},
	} {
		t.Run(c.name, func(t *testing.T) {
			asWindowsPaths(t, c.windows)
			groups := At(WithGates(DefaultGroups()), c.here)
			if len(groups) == 0 || groups[0].place == nil || groups[0].place.folders == nil {
				t.Fatal("At didn't read the place's folders")
			}
			for i := range groups {
				if groups[i].place == nil || groups[i].place.folders != groups[0].place.folders {
					t.Errorf("group %d: its own reading of the place's folders", i)
				}
			}
			if got, want := groups[0].place.folders, readFolders(c.here); !reflect.DeepEqual(got, want) {
				t.Errorf("At read %+v, not %+v", got, want)
			}
			// Given another place's reading, gitFolderPath answers by it: it uses what At read.
			p := c.here
			p.folders = readFolders(c.away)
			if !gitFolderPath(c.awayIn, &p) {
				t.Errorf("%q: the reading At stored wasn't used", c.awayIn)
			}
			if gitFolderPath(c.hereIn, &p) {
				t.Errorf("%q: the place was read again", c.hereIn)
			}
			// And with none stored (a bare Place), it reads the place itself.
			if bare := c.here; !gitFolderPath(c.hereIn, &bare) || gitFolderPath(c.awayIn, &bare) {
				t.Errorf("a bare place: %q, %q", c.hereIn, c.awayIn)
			}
		})
	}
}

// RT126GUARDWINR2's follow-up 3: Fold turns some letters outside ASCII into ASCII ones (ſ is s, the Kelvin sign k, ß
// ss), but Windows makes a short name from the raw letters, so it may write TORE~1 for ſtore. A part whose raw name
// holds a letter outside ASCII takes any short name, on either side: the place's folders, or the word.
func TestAShortNameIsReadFromTheRawNameNotItsFold(t *testing.T) {
	asWindowsPaths(t, true)
	long := &Place{Known: true, cwd: `C:\w\tree`, gitDir: "C:\\w\\\u017ftore", common: "C:\\w\\\u017ftore"}             // ſtore
	kelvin := &Place{Known: true, cwd: `C:\w\tree`, gitDir: "C:\\\u212aelvin\\store", common: "C:\\\u212aelvin\\store"} // Kelvin
	german := &Place{Known: true, cwd: "C:\\Stra\u00dfe\\tree", gitDir: "C:\\Stra\u00dfe\\store", common: "C:\\Stra\u00dfe\\store"}
	short := &Place{Known: true, cwd: `C:\w\tree`, gitDir: `C:\w\TORE~1`, common: `C:\w\TORE~1`} // as the hook may give it
	ascii := &Place{Known: true, cwd: `C:\w\tree`, gitDir: `C:\w\store`, common: `C:\w\store`}
	in := map[*Place][]string{
		long:   {`C:\w\TORE~1\HEAD`, `..\TORE~1\HEAD`, `C:\w\STORE~1\HEAD`, `C:\w\_TORE~1\HEAD`, "C:\\w\\\u017ftore\\HEAD", `C:\w\store\HEAD`},
		kelvin: {`C:\ELVIN~1\store\HEAD`, `C:\KELVIN~1\store\HEAD`, `C:\_ELVIN~1\store\HEAD`},
		german: {`C:\STRAE~1\store\HEAD`, `C:\STRA_E~1\store\HEAD`, `C:\STRASS~1\store\HEAD`, `..\store\HEAD`},
		short:  {"C:\\w\\\u017ftore\\HEAD", "..\\\u017ftore\\HEAD", "C:\\\u212aelvin\\..\\w\\\u017ftore\\HEAD"},
	}
	out := map[*Place][]string{
		long:   {`C:\w\tore\HEAD`, `C:\w\TREE\HEAD`, `C:\w\x\TORE~1\HEAD`, `C:\w\TORE~1x\HEAD`, `D:\w\TORE~1\HEAD`},
		kelvin: {`C:\ELVIN~1\tree\HEAD`, `C:\elvin\store\HEAD`},
		german: {`C:\STRAE~1\tree\HEAD`},
		short:  {`C:\w\store\HEAD`, `C:\w\tree\HEAD`, "C:\\x\\\u017ftore\\HEAD"}, // store, ASCII, is no long name of TORE~1
		ascii:  {`C:\w\TORE~1\HEAD`, `..\TORE~1\HEAD`},                           // an ASCII name is still compared
	}
	for p, words := range in {
		for _, w := range words {
			if !gitFolderPath(w, p) {
				t.Errorf("from %s: %q isn't in %s", p.cwd, w, p.gitDir)
			}
		}
	}
	for p, words := range out {
		for _, w := range words {
			if gitFolderPath(w, p) {
				t.Errorf("from %s: %q is in %s", p.cwd, w, p.gitDir)
			}
		}
	}
	for _, c := range []struct {
		raw  string
		want []string
	}{
		{`C:\w\store`, nil}, {"C:\\w\\\u017ftore.", []string{"store"}}, {"\u212aelvin/x:\u017f", []string{"kelvin", "s"}},
		{"C:\\Stra\u00dfe\\H\u00e9l\u00e8ne", []string{"strasse", "hélène"}}, {"of=C:\\\ufb01le\\x", []string{"file"}},
	} {
		if got := wideNames(c.raw, nil); !slices.Equal(got, c.want) {
			t.Errorf("wideNames(%q) = %q", c.raw, got)
		}
	}
	groups := At(WithGates(DefaultGroups()), *long)
	if r := Check(`cp x 'C:\w\TORE~1\HEAD'`, groups); r == nil || !strings.Contains(r.Why, "git folder") {
		t.Errorf("TORE~1 for ſtore: %v", r)
	}
	if r := Check(`cp x 'C:\w\tore\HEAD'`, groups); r != nil {
		t.Errorf("tore for ſtore: %v", r)
	}
}
