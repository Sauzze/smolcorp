package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The fence of a hire in T3 Code is its worktree's .claude/settings.local.json
// (T797b). Tests/fixtures/frontdesk/fence-commands.json lists what must be
// refused and what must still run; the Mac's GuardFileTests checks it names
// the very file T3Desk writes, and runs the real guard on it. Every hire is
// held to it, the GM too.
func TestNoHireTakesAwayOrRewritesClaudeCodesSettings(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "Tests", "fixtures", "frontdesk", "fence-commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Fence   string   `json:"fence"`
		Refused []string `json:"refused"`
		Allowed []string `json:"allowed"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if f.Fence != ".claude/settings.local.json" || len(f.Refused) == 0 || len(f.Allowed) == 0 {
		t.Fatalf("the fixture: %+v", f)
	}
	mustRefuse(t, "fence", f.Refused)
	mustRun(t, "fence look-alikes", f.Allowed)
	for _, line := range f.Refused {
		if Check(line, ForGM(DefaultGroups())) == nil {
			t.Errorf("the GM: let through: %s", line)
		}
	}
	// RWIN5 B3: the same names spelled with ſ (the long s), which the Mac's disk reads as s, and in any case. And a
	// program the disk finds by such a name is that program (ſsh is ssh, Kubectl kubectl). git reads its own
	// subcommands by their bytes: git puſh isn't push, and runs.
	odd := []string{
		"echo '{}' > .claude/\u017fettings.local.json", "rm .claude/\u017fettings.local.json", "cp x .claude/\u017fettings.json",
		"echo '{}' > .claude/\u017fettings.json", "rm .claude/\u017fettings.json", "mv x .CLAUDE/SETTINGS.LOCAL.JSON",
		"echo x > ~/.config/sauzzedeck-pro/hire-guard.json", "rm \u017fmolcorp_gm_token", "cat gm-token-kept",
		"cat push-key-kept", "cat ~/.config/sauzzedeck-pro/PUSH-KEY-KEPT",
		// NS1: the night shift's jobs, whose approvals grant (TestNoHireWritesTheNightShiftsJobs).
		"echo '{}' > ~/.config/sauzzedeck-pro/nightshift.json", "cp x NIGHTSHIFT.JSON", "cp x ~/.config/sauzzedeck-pro/nightſhift.json",
		"\u017fsh host", "\u212aubectl apply -f x.yaml", "/usr/bin/\u017f\u017fh host", "git pu\u017fh",
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range odd[:len(odd)-1] {
			if Check(line, groups) == nil {
				t.Errorf("let through: %s", line)
			}
		}
	}
	// RWIN7 B1: a bracket Go's matcher read otherwise ([!x], POSIX's classes) fills in the settings and their folder.
	brackets := []string{
		"rm .claude/s[!x]ttings.local.json", "echo '{}' > .claude/s[!x]ttings.json", "rm -rf .c[!x]aude", "rm -rf .c[[:alpha:]]aude",
		"cp x .claude/s[[:alpha:]]ttings.local.json", "cat .claude/s[!x]ttings.json",
		// For comparison, refused before too.
		"rm .claude/s?ttings.json", "rm .claude/s[e]ttings.json", "noglob rm .claude/settings.json",
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range brackets {
			if Check(line, groups) == nil {
				t.Errorf("let through: %s", line)
			}
		}
	}
	// A bracket that fills in the settings only for a reader, and one in another folder, still run. ls .claude/[!s]*
	// is refused: read wide, [!s] is any letter, and a ! bracket that fills in the settings isn't written plainly.
	mustRun(t, "brackets read wide", []string{"ls .claude/[a-r]*", "cat .claude/agents/[!x]*.md", "ls .c[a-k]*"})
	if r := Check("ls .claude/[!s]*", DefaultGroups()); r == nil || r.Rule != notPlainRule {
		t.Errorf("ls .claude/[!s]*: %v", r)
	}
	for line, rule := range map[string]string{
		"rm .claude/settings.local.json": fenceRule, "echo '{}' > .claude/settings.local.json": fenceRule,
		"git clean -fdx": "git clean -x", "git stash push -a": "git stash --all",
	} {
		if r := Check(line, DefaultGroups()); r == nil || r.Group != "guard" || r.Rule != rule {
			t.Errorf("%q: %+v, want guard: %s", line, r, rule)
		}
	}
}

// NS1, RT126NS1 B1: the night shift's jobs are kept as the guard file is. A hire or the GM writes them by no
// spelling, and by no pattern that fills them in, else one could switch off the owner's approvals or, off a Mac,
// approve a job of its own. They hold no secret: a command that only reads them runs. (A * in the support folder,
// or package*.json, may fill in phones.json too, which no command reads since GUARDPS1: that refusal is phones.json's.)
func TestNoHireWritesTheNightShiftsJobs(t *testing.T) {
	const dir = "~/.config/sauzzedeck-pro/"
	refused := []string{
		"cp x " + dir + "night?hift.json", "echo '{}' > " + dir + "nightsh?ft.json", "cp x " + dir + "nightsh*.json",
		"cp x " + dir + "n*", `echo x > ` + dir + `night"shift".json`, `echo x > ` + dir + `night\shift.json`,
		"f=night; cp x " + dir + "${f}shift.json", "cp x " + dir + "night[s]hift.json", "echo '{}' > " + dir + "*shift.json",
		"rm " + dir + "nightshift.json", "mv x " + dir + "nightshift.json", "tee " + dir + "nightshift.js*",
		"sed -i s/proposed/approved/ " + dir + "nightshift.json", "f=" + dir + "nightshift.json", "echo '{}' > " + dir + "NIGHTSHIFT.JSON",
		"echo '{}' > " + dir + "nightshift.json.tmp", "sh -c 'cp x " + dir + "night?hift.json'",
		`python3 -c "open('nightshift.json','w').write('{}')"`, "cat <<EOF > " + dir + "night?hift.json\n{}\nEOF",
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range refused {
			if r := Check(line, groups); r == nil {
				t.Errorf("let through: %s", line)
			}
		}
		for _, line := range []string{
			"cat " + dir + "nightshift.json", "cat " + dir + "NIGHTSHIFT.JSON", "jq .jobs " + dir + "night?hift.json",
			"grep -n approved " + dir + "nightshift.json", "git add desktop/nightshift.go",
			"sed -i s/a/b/ desktop/nightshift.go", "cp desktop/nightshift_test.go x",
		} {
			if r := Check(line, groups); r != nil {
				t.Errorf("refused %q: %v", line, r)
			}
		}
		// MSYGUARDPS1: these read the night file and ran under NS1; GUARDPS1's phones.json rule refuses them (FXGUARDPS1R1
		// B5), for phones.json and not for the night file.
		for _, line := range []string{"ls " + dir + "nightsh*.json", "cat package*.json"} {
			if r := Check(line, groups); r == nil || r.Rule != phonesFile {
				t.Errorf("%q: %+v, want guard: %s", line, r, phonesFile)
			}
		}
	}
	if r := Check("cp x "+dir+"night?hift.json", DefaultGroups()); r == nil || r.Rule != nightFile {
		t.Errorf("night?hift.json: %+v, want guard: %s", r, nightFile)
	}
}

// RT126NS1R2 B1: git log, show, diff and blame write what they'd print to the file --output names, text of the
// caller's choosing with --format. Such a call is no reader, for the night shift's jobs and the guard file alike.
// --output-indicator-new= and its kin write nothing, and git takes no abbreviation of --output.
func TestGitWritesNoKeptFileThroughOutput(t *testing.T) {
	const dir = "~/.config/sauzzedeck-pro/"
	for _, f := range []struct{ name, pattern string }{{"nightshift.json", "night?hift.json"}, {"hire-guard.json", "hire-g?ard.json"}} {
		refused := []string{
			"git log -1 --format=x --output=" + dir + f.name, "git diff --output " + dir + f.name,
			"git show --format=x --output=" + dir + f.pattern, "git blame --output=" + dir + f.name + " x.go",
			"git -C " + dir + " log --output=" + f.name, "git log --outp''ut=" + dir + f.name, "git log --outp$(true)ut=" + dir + f.name,
			"git log --outpu?=" + dir + f.name, "git log --outp{u,}t=" + dir + f.name,
		}
		runs := []string{
			"git log -1 " + dir + f.name, "git log --output-indicator-new=+ -1 -- " + dir + f.name,
			"git diff --output-indicator-old=- --output-indicator-context=' ' -- " + dir + f.name,
		}
		for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
			for _, line := range refused {
				if r := Check(line, groups); r == nil {
					t.Errorf("let through: %s", line)
				}
			}
			for _, line := range runs {
				if r := Check(line, groups); r != nil {
					t.Errorf("refused %q: %v", line, r)
				}
			}
		}
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range []string{"git diff desktop/nightshift.go", "git log --output-indicator-new=+ -1", "git diff --output=x.patch"} {
			if r := Check(line, groups); r != nil {
				t.Errorf("refused %q: %v", line, r)
			}
		}
	}
}

// RT126NS1R3 B1: a word the shell turns into --output (a variable, a substitution, a pattern that starts the word) makes
// a git reader a writer too, for the hire and the GM, for the night shift's jobs and the guard file alike. The cost is
// only a git call that names a kept file and has such a word; one that names no kept file runs.
func TestGitWritesNoKeptFileThroughAHiddenOutput(t *testing.T) {
	const dir = "~/.config/sauzzedeck-pro/"
	for _, f := range []struct{ name, pattern string }{{"nightshift.json", "night?hift.json"}, {"hire-guard.json", "hire-g?ard.json"}} {
		refused := []string{
			// The reviewer's eleven.
			"o=--output; git diff $o " + dir + f.name, "o=--output; git log -1 --format=x $o=" + dir + f.name,
			`o=--output; git log -1 --format=x "$o" ` + dir + f.name, "o=--output; git show -s --format=x $o=" + dir + f.name,
			"o=--output; git blame $o " + dir + f.name + " x.go", "printf -v o -- --output; git diff $o " + dir + f.name,
			"read o <<< --output; git diff $o " + dir + f.name, `set -- --output; git log -1 --format=x "$1" ` + dir + f.name,
			"git log -1 --format=x $(printf -- --output)=" + dir + f.name, `git log -1 --format=x "$(echo --output)" ` + dir + f.name,
			"git log -1 --format=x `echo --output` " + dir + f.name,
			// A pattern that starts the word, which bash fills in from the current folder.
			"git log -1 --format=x ?-output=d/" + f.name, "git log -1 --format=x [-]-output=d/" + f.name,
			"git log -1 --format=x *output=d/" + f.name,
			// More of the same: ${ }, an expansion after the dashes, a home spelling with no / after it, and a move
			// into the folder.
			"o=--output; git log -1 --format=x ${o}=" + dir + f.name, "git log -1 --format=x --$(echo output)=" + dir + f.name,
			"git log -1 --format=x $HOME--output=" + dir + f.name, "git -C " + dir + " log -1 --format=x $(echo --output) " + f.name,
		}
		runs := []string{
			"git log -1 " + dir + f.name, "git log --output-indicator-new=+ -1 -- " + dir + f.name,
			"git diff --output-indicator-old=- --output-indicator-context=' ' -- " + dir + f.name,
			"git log -1 --format=%H -- " + dir + f.pattern, "git log -1 $HOME/.config/sauzzedeck-pro/" + f.name,
		}
		for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
			for _, line := range refused {
				if r := Check(line, groups); r == nil {
					t.Errorf("let through: %s", line)
				}
			}
			for _, line := range runs {
				if r := Check(line, groups); r != nil {
					t.Errorf("refused %q: %v", line, r)
				}
			}
		}
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		if r := Check("o=--output; git log -1 --format=x $o .claude/settings.json", groups); r == nil {
			t.Errorf("let through: o=--output; git log -1 --format=x $o .claude/settings.json")
		}
		for _, line := range []string{
			"git diff desktop/nightshift.go", "git log --output-indicator-new=+ -1", "git diff --output=x.patch",
			`git log -1 "$REV"`, "git diff $(git merge-base HEAD main)", "git log -1 ${HOME}/x.json", "o=--output; git log -1 --format=x $o x.patch",
		} {
			if r := Check(line, groups); r != nil {
				t.Errorf("refused %q: %v", line, r)
			}
		}
	}
}

// RWIN7 B2: zsh's precommand modifiers (noglob, nocorrect, -), repeat N and emulate -c run a command, and so does a
// word right after ]] or (( )) (its short forms): each is read as that command, or refused as not written plainly.
// B3: zsh's ${(e)…} and bash 5's ${x@P} run the text they're given. For every managed hire and the GM.
func TestZshsOwnWaysToRunACommand(t *testing.T) {
	const dir = `~/Library/Application\ Support/AgentDeck/`
	refused := []string{
		// B2
		"noglob git push", "nocorrect git push", "true; - git push", "repeat 1 git push", "if [[ -n x ]] git push",
		"noglob ssh host", "noglob vercel deploy", "emulate zsh -c 'git push'", "emulate sh -c 'git push'", "emulate zsh -c 'cat setup-key'",
		"while [[ -n x ]] git push", "if [[ -n x ]] then git push; fi", "[[ -n x ]] >/dev/null git push", "(( 1 )) git push",
		"emulate -R zsh -c 'git push'", "if noglob git push; then :; fi", "nocorrect noglob git push",
		// B2's comparisons, refused before too.
		"repeat 1 { git push }", "for i in 1; git push", "for i (1) git push", "() { git push }", "noglob rm .claude/settings.json",
		"repeat 1 cat setup-key", "emulate sh -c 'cat " + dir + "setup-key'",
		// B3
		"echo ${(e):-'$(git push)'}", "echo ${(e):-'$(ssh host)'}", ": ${(e):-'$(vercel deploy)'}", "echo ${(e):-'`git push`'}",
		"echo ${(e)${:-'$(git push)'}}", "print -r -- ${(e):-'$(git push)'}", "echo ${(e):-'$(cat setup-key)'}",
		"echo ${(e):-'$(cat " + dir + "setup-key)'}", `x='$(git push)'; echo "${x@P}"`, "echo ${x@P}", "echo ${(P)x}",
		"echo $(( ${(e)x} ))", "cat <<EOF\n${(e):-x}\nEOF", "cat <<EOF\n${x@P}\nEOF", "echo \"${(e):-x}\"",
		// B3's comparison, refused before too.
		`echo "${(e):-\$(git push)}"`,
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range refused {
			if Check(line, groups) == nil {
				t.Errorf("let through: %q", line)
			}
		}
	}
	for line, want := range map[string]string{
		"if [[ -n x ]] git push": plainShort, "if [[ -n x ]] then git push; fi": plainShort, "(( 1 )) ls": plainShort,
		"echo ${(e):-'$(git push)'}": plainParam, "echo \"${x@P}\"": plainParam, "noglob git push": "",
	} {
		r := Check(line, DefaultGroups())
		if r == nil || want != "" && (r.Rule != notPlainRule || r.Why != want) || want == "" && r.Group != "push" {
			t.Errorf("%q: %v", line, r)
		}
	}
	mustRun(t, "zsh's forms, plainly", []string{
		"if [[ -n x ]]; then git status; fi", "[[ -f go.mod ]] && go test ./...", "while [[ -n $x ]]; do x=; done",
		"for ((i=0;i<3;i++)); do echo $i; done", "(( n += 1 ))", "[[ -n x ]] { git status }", "{ [[ -n x ]] }",
		"echo ${HOME}", `echo "${x:-y}"`, "echo ${#arr[@]}", "echo '${(e)x}'", "git log --format=%P", "echo user@Paris",
		`echo "${HOME}@Paris"`, "noglob git status", "nocorrect ls", "repeat 3 echo hi", "emulate -L zsh",
	})
}
