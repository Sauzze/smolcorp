package guard

import (
	"strings"
	"testing"
	"unicode"
)

// WIN125g (RWIN6 B2–B4): the guard stops chasing spellings and refuses a line not written plainly (plain.go), for
// every managed hire and the GM. Each of RWIN6's probes is refused, and one probe per rule; the refusal asks for
// the command written plainly.
func TestTheGuardRefusesWhatIsntWrittenPlainly(t *testing.T) {
	const dir = `~/Library/Application\ Support/AgentDeck/`
	rwin6 := []string{
		// B2: the Mac's disk folds ß and ẞ to ss, ﬀ ﬁ ﬂ ﬃ ﬄ to ff fi fl ffi ffl, ﬅ and ﬆ to st.
		"ßh host", "ẞH host", "/usr/bin/ßh host", "/usr/bin/ßh -V", "ßcp a host:b", "ßh-keygen -f x",
		"ﬂy deploy", "ﬂyctl deploy", "ﬁrebase deploy", "ﬁrebase-tools deploy", "npx ﬁrebase-tools deploy",
		"faﬆlane beta", "faﬅlane beta", "paß show bank",
		// B3: a NUL in $'…' ends the word as a program reads it; zsh's (a|b) is a glob.
		`cat $'setup-key\0junk'`, `cat $'setup-key\0x'`, `cat setup-key$'\x00.md'`, `cat 'setup-key'$'\0'`, "cat setup-(key|x)",
		// B4: zsh's =cmd is the program's full path.
		"=git push", "=ssh host", "=fly deploy", "=vercel deploy", "=security find-generic-password",
		"=ssh -V", "=git --version",
		// RWIN6's comparisons, refused before too, and still.
		"cat setup-key(N)", "cat (setup-key|nothing)", "cat setup-key(:a)", "cat @(setup-key)", "cat =(git push)", "f() { git push; }",
	}
	// One probe per rule, and its forms.
	rules := []string{
		// A control character, or bytes that aren't UTF-8.
		"cat setup-key\x00x", "cat setup\x1b-key", "ls\rgit push", "cat \xff", "echo x\x7f", "echo \u0085",
		// $'…' outside other quotes, wherever it is.
		`echo $'x'`, `git commit -m $'fix'`, "cat ${X:-$'\\x73etup-key'}", "echo \"$(cat $'x')\"",
		// A word not written plainly (WIN125h: a letter the disk may read as another, another script, an accent
		// typed apart, an invisible one) outside quotes, anywhere; inside quotes, anywhere but data. A program's
		// name with any letter outside ASCII.
		"cat ﬁle.txt", "cat docs/Straße.md", "git commit -m fix\\ ﬁ", "echo caﬁ", "'ßh' host", `"ﬂy" deploy`,
		"cat 'docs/Straße.md'", "cp x 'ﬁle'", "npx 'ﬁrebase-tools' deploy", "env 'ßh' host", "echo x > 'ﬁ.txt'",
		"cat < 'ﬁ.txt'", "X='ﬁ' make", "git log --author='Ｅlodie'", "grep -rn 'ß' .", "sh -c 'echo ß'",
		"git commit -F 'ﬁ.txt'", "git ls-files -m 'ß'", "git commit -m ok 'ﬁ.go'",
		// RWIN7 N4: in -Fm, -tm and -Cm the m is the value of -F, -t or -C: the next word is a path.
		"git commit -Fm 'ﬁle'", "git commit -tm 'ﬁle'", "git commit -Cm 'ﬁ'", "git merge -sm 'ﬁ'", "git notes add -Fm 'ﬁ'",
		"présentation.sh", "./présentation.sh", "'Équipe/run'", "env présentation", "sh -c 'présentation x'",
		"timeout 9 ﬁx", "command été", "exec été",
		// A word that starts with = (zsh's =cmd), even quoted as the command's name.
		"'=git' push", "env =ssh host", "sudo =git push", "cat =(echo x)", "ls =ls",
		// zsh's glob forms: ( glued or opening a word after the name, ^, ~ and # inside a word.
		"cat x(N)", "cat (a|b)", "cat x(:a)", "cat !(x)", "cat @(x)", "f() { echo x; }",
		"npm i lodash@^4.17.0", "git diff origin/^", "cat hire-guard.json~1",
		"git diff origin/main~1", "git diff main~1", "git log main~3..main", "cat hire-guard^1",
		"git diff origin/main^/x",
	}
	// WIN125j: these are refused by the name their skeleton could fill in (skeleton.go), which the guard finds first.
	named := []string{
		"cat " + dir + "^nothing", "cat setup-key~x", "cat " + dir + "setup-key~x", "cat setup-key#",
		"cat " + dir + "s*~x", "git diff --no-index /dev/null " + dir + "setup-key~1",
		"cat setup-key^1", "git log setup-key~1", "cat GM-TOK~1", "rm -rf .claude^1",
		// RMSY126k nit 1: refused as before, now by claude-install, whose short name on Windows it can be.
		"rm -rf CLAUDE~1",
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range append(append([]string{}, rwin6...), rules...) {
			if r := Check(line, groups); r == nil {
				t.Errorf("let through: %q", line)
			}
		}
		for _, line := range rules {
			if r := Check(line, groups); r == nil || r.Rule != notPlainRule || !strings.Contains(r.Error(), "Write the command plainly") {
				t.Errorf("%q: %v, want: %s", line, r, notPlainRule)
			}
		}
		for _, line := range named {
			if r := Check(line, groups); r == nil || r.Group != "guard" {
				t.Errorf("%q: %v, want a guard refusal", line, r)
			}
		}
	}
	// What these rules leave alone: text in quotes as data, a lone = or == (test's), a ~ at a word's start or after =
	// or :, $#, NAME=( ), (( )), a subshell, and git's revisions.
	mustRun(t, "written plainly", []string{
		"echo 'é'", `printf '%s\n' "Résumé"`, `git commit -m "Corrige l'accès"`, "git commit -am 'été'", "git commit -m'été'",
		"git commit --message 'é'", "git commit --message='é'", "git commit -sm 'Corrige ﬁ'", "git tag -sm 'ﬁ' v1", "git commit -vam 'ß'", "git tag -a v1 -m 'Version 1 : réglée'", "git stash push -m 'à moitié'",
		"cat <<'EOF' > notes.md\nRésumé : ß, ﬁ\nEOF", "[ a = b ] && echo y", "[ a == b ] && echo y", "cat ~/x", "ls --prefix=~/x",
		"PATH=$PATH:~/bin make", "echo $#", "arr=(a b); echo ${#arr[@]}", "x+=(c)", "(cd internal && go test .)",
		"if (cd x); then echo y; fi", "(( x = 1 + 2 ))", "echo $(( 1 + 2 ))", "git diff HEAD~1", "git show HEAD^",
		"git log HEAD~3..HEAD", "git show HEAD^:go.mod", "git diff abc1234^2 HEAD^{tree}", "git log origin/main..HEAD~1",
		// RWIN7 N5: a branch's revisions with a ^, as git writes them (zsh fills in main^2 as main at most).
		"git log main^", "git show origin/main^", "git show main^2", "git diff v1.2.0^..v1.3.0", "git rebase -i feature/x^",
		"git log main^..feature", "git show main^{tree}", "git diff main^~1",
		"cat 'setup-key(N)'", "echo 'x#y' \"a~b\" 'c^d'", "git log --format='%h %s' -3", "grep -n '^func ' x.go",
	})
}

// WIN125h: the owner's clients are French, and their folders and files carry accents. A path or an argument with
// an accented Latin letter, typed as one letter, runs for a managed hire and for the GM, quoted or not, and as a
// redirection's target. A word the disk may read as another name, or that holds another script, an accent typed
// apart or an invisible letter, is still refused, as is a program's name with any letter outside ASCII.
func TestAccentedPathsRun(t *testing.T) {
	accented := []string{
		"cat docs/présentation.md", `cd "Wonder/Équipe"`, "ls Équipe/", "git add docs/présentation.md",
		"mv présentation.md présentation-v2.md", `git commit -m "Ajoute la présentation de l'équipe"`,
		"cat 'docs/présentation.md'", "cp Équipe/budget.xlsx Archivé/", "mkdir -p Réunions/Été", "git mv Œuvre.md œuvre.md",
		"echo x > 'résumé.txt'", "cat < 'résumé.txt'", "X='é' make", "git log --author='Élodie'", "grep -rn 'é' .",
		"sh -c 'cat présentation.md'", "git commit -F 'message-été.txt'", "git commit -m ok 'é.go'",
		"echo café", "git commit -m fix\\ é", "rm -f Ÿ.md ÿ.md ø.md ñ.md ő.md ž.md ǎ.md", "timeout 9 cat présentation.md",
		"env LANG=fr_FR.UTF-8 cat présentation.md", "find Équipe -name '*.md'", "git diff HEAD~1 -- docs/présentation.md",
	}
	refused := []string{
		// Ligatures, fullwidth and compatibility forms: NFKC changes them.
		"cat ﬁle.md", "cat Ｆile.md", "cat ﬂèche.md", "cat ĳs.md", "cat 'Ｆile.md'", "cat docs/\u212aelvin.md", "cat ŀl.md",
		// Full case folding isn't lower case: ß is ss, ſ is s, ŉ, ǰ.
		"cat Straße.md", "cat ẞ.md", "cat ſetup.md", "cat ŉ.md", "cat ǰ.md",
		// A letter whose other case is ASCII: ı and İ.
		"cat fıle.md", "cat İndex.md",
		// Another script: a Cyrillic а and е, a Greek ο, Latin Extended Additional, IPA, a click that looks like |.
		"cat d\u0430ta.md", "cat docs/pr\u0435sentation.md", "cat d\u03bfcs/x.md", "cat 'd\u0430ta.md'", "cat \u1ebd.md", "cat \u0251.md", "cat a\u01c0b.md",
		// An accent typed apart (NFC would join it), zero-width and bidi characters, a no-break space.
		"cat pre\u0301sentation.md", "cat se\u200btup-key", "cat a\u200db.md", "cat \u202etxt.md", "cat a\u2066b.md", "cat a\u00a0b.md",
		"cat a\ufeffb.md", "cat \u00aa.md",
		// A program's name, wherever it is.
		"présentation.sh", "env présentation", "xargs -0 été",
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range accented {
			if r := Check(line, groups); r != nil {
				t.Errorf("refused %q: %v", line, r)
			}
		}
		for _, line := range refused {
			if r := Check(line, groups); r == nil || r.Rule != notPlainRule {
				t.Errorf("%q: %v, want: %s", line, r, notPlainRule)
			}
		}
	}
}

// WIN125h: plainLatin's list is Unicode's. notPlainLatin was taken from Python's unicodedata (Unicode 16):
//
//	[c for c in map(chr, range(0xC0, 0x250)) if unicodedata.category(c)[0] == 'L' and
//	 (unicodedata.normalize('NFKC', c) != c or c.casefold() != c.lower())]
//
// and the letters plainLatin keeps are the 371 that are upper or lower case, none of those, and with no case that
// is ASCII. The French letters are among them; ß, ſ, ĳ, ŀ, ı, İ and the clicks aren't.
func TestPlainLatinIsUnicodes(t *testing.T) {
	kept := 0
	for r := rune(0); r < 0x300; r++ {
		if plainLatin(r) {
			kept++
			if r < 0xC0 || r > 0x24F || !unicode.Is(unicode.Latin, r) {
				t.Errorf("%U kept", r)
			}
		}
	}
	if kept != 371 {
		t.Errorf("%d letters kept, want 371", kept)
	}
	for _, r := range "àâäæçéèêëîïôöœùûüÿÀÂÄÆÇÉÈÊËÎÏÔÖŒÙÛÜŸñÑøØåÅőŐžŽǎ" {
		if !plainLatin(r) {
			t.Errorf("%c refused", r)
		}
	}
	for _, r := range notPlainLatin + "ıİǀǁǂǃƻ×÷ʼ" {
		if plainLatin(r) {
			t.Errorf("%c (%U) kept", r, r)
		}
	}
}

// WIN125g: normal work still runs for a managed hire (and for the GM): the lines a developer runs every day, French
// commit messages with accents among them. Each is allowed.
var everydayLines = []string{
	"git status",
	"git status --short",
	"git diff",
	"git diff --stat HEAD~1",
	"git diff HEAD~1 -- desktop/demofence.go",
	"git diff --cached",
	"git add desktop/demofence.go internal/guard/plain.go",
	"git add -p",
	`git commit -m "Corrige l'affichage des accents dans le résumé"`,
	"git commit -m 'Ajoute la gestion des fichiers téléchargés'",
	`git commit -am "Réécrit le module de facturation — étape 2"`,
	"git commit -m \"$(cat <<'EOF'\nCorrige le calcul de la TVA\n\nLe total était arrondi deux fois, d'où l'écart.\nEOF\n)\"",
	"git log --oneline -10",
	"git log -p -- README.md",
	"git log --author=ada --since=2.weeks",
	"git log --pretty=format:'%h %an %s' -5",
	"git show HEAD",
	"git show HEAD^ --stat",
	"git branch -a",
	"git restore --staged README.md",
	"git stash list",
	"git blame -L 10,40 desktop/hire.go",
	"go test ./...",
	"go test -count=1 -race ./internal/guard/",
	"go test -run 'TestNoSpelling' ./internal/guard/ -v",
	"go build ./... && go vet ./...",
	"go test ./... 2>&1 | tail -20",
	"gofmt -l .",
	"cd internal/guard && go test .",
	"npm test",
	"npm run lint -- --fix",
	"npm install",
	"npm run build && ls dist/",
	"npx vitest run src/utils.test.ts",
	"ls -la",
	"ls src/components/",
	"cat package.json",
	"cat README.md | head -50",
	"head -20 CHANGELOG.md && tail -5 CHANGELOG.md",
	`grep -rn "TODO" src/`,
	"grep -n 'func ' internal/guard/guard.go",
	"grep -rl --include='*.go' 'Fold(' .",
	"sed -n '1,80p' internal/guard/lex.go",
	"sed -i '' 's/oldName/newName/g' src/app.ts",
	"awk -F, '{print $2}' data/users.csv | sort | uniq -c",
	"awk '/^func /{print FILENAME\": \"$0}' desktop/*.go",
	"make",
	"make test",
	"curl -s http://localhost:3000/api/health",
	`curl -X POST localhost:8080/api/items -H 'Content-Type: application/json' -d '{"name":"x"}'`,
	"find . -name '*.go' -newer go.mod | head",
	"wc -l internal/guard/*.go",
	"mkdir -p build/out && cp dist/app.js build/out/",
	"rm -rf node_modules/.cache",
	"jq '.scripts' package.json",
	"diff -u old.txt new.txt",
	"python3 -m pytest tests/ -q",
	`echo "Déploiement prévu demain" >> notes.md`,
	"printf '%s\\n' 'Résumé des tests'",
	"git tag -a v1.2.0 -m 'Version 1.2.0 : corrections diverses'",
}

func TestEverydayWorkStillRuns(t *testing.T) {
	if len(everydayLines) < 40 {
		t.Fatalf("only %d lines", len(everydayLines))
	}
	mustRun(t, "everyday work", everydayLines)
	for _, line := range everydayLines {
		if r := Check(line, ForGM(DefaultGroups())); r != nil {
			t.Errorf("the GM: refused %q: %v", line, r)
		}
	}
}
