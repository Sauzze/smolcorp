package guard

import (
	"strings"
	"testing"
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
		// A letter outside ASCII outside quotes, anywhere; inside quotes, anywhere but data.
		"cat ﬁle.txt", "cat docs/Straße.md", "git commit -m fix\\ é", "echo café", "'ßh' host", `"ﬂy" deploy`,
		"cat 'docs/Straße.md'", "cp x 'ﬁle'", "npx 'ﬁrebase-tools' deploy", "env 'ßh' host", "echo x > 'é.txt'",
		"cat < 'é.txt'", "X='é' make", "git log --author='Élodie'", "grep -rn 'é' .", "sh -c 'echo é'",
		"git commit -F 'é.txt'", "git ls-files -m 'é'", "git commit -m ok 'é.go'",
		// A word that starts with = (zsh's =cmd), even quoted as the command's name.
		"'=git' push", "env =ssh host", "sudo =git push", "cat =(echo x)", "ls =ls",
		// zsh's glob forms: ( glued or opening a word after the name, ^, ~ and # inside a word.
		"cat x(N)", "cat (a|b)", "cat x(:a)", "cat !(x)", "cat @(x)", "f() { echo x; }",
		"cat " + dir + "^nothing", "cat setup-key~x", "cat " + dir + "setup-key~x", "cat setup-key#",
		"cat " + dir + "s*~x", "git diff --no-index /dev/null " + dir + "setup-key~1",
		"npm i lodash@^4.17.0", "git log main^", "git diff origin/main~1",
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
	}
	// What these rules leave alone: text in quotes as data, a lone = or == (test's), a ~ at a word's start or after =
	// or :, $#, NAME=( ), (( )), a subshell, and git's revisions.
	mustRun(t, "written plainly", []string{
		"echo 'é'", `printf '%s\n' "Résumé"`, `git commit -m "Corrige l'accès"`, "git commit -am 'été'", "git commit -m'été'",
		"git commit --message 'é'", "git commit --message='é'", "git tag -a v1 -m 'Version 1 : réglée'", "git stash push -m 'à moitié'",
		"cat <<'EOF' > notes.md\nRésumé : ß, ﬁ\nEOF", "[ a = b ] && echo y", "[ a == b ] && echo y", "cat ~/x", "ls --prefix=~/x",
		"PATH=$PATH:~/bin make", "echo $#", "arr=(a b); echo ${#arr[@]}", "x+=(c)", "(cd internal && go test .)",
		"if (cd x); then echo y; fi", "(( x = 1 + 2 ))", "echo $(( 1 + 2 ))", "git diff HEAD~1", "git show HEAD^",
		"git log HEAD~3..HEAD", "git show HEAD^:go.mod", "git diff abc1234^2 HEAD^{tree}", "git log origin/main..HEAD~1",
		"cat 'setup-key(N)'", "echo 'x#y' \"a~b\" 'c^d'", "git log --format='%h %s' -3", "grep -n '^func ' x.go",
	})
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
