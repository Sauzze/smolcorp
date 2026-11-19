package guard

import (
	"strings"
	"testing"
)

// FXGUARDWIN126: a tester's false positives on Windows (round 4, 6a–6e) and gh release's reads (7). Each row runs for
// a hire and the GM, with the gate groups on, as a Mac's or Linux's line and as Windows reads one (asWindows); each
// false positive has its controls beside it, still refused.

// readings runs f once as a Mac or Linux reads the line, and once as Windows does.
func readings(t *testing.T, f func(t *testing.T)) {
	t.Run("posix", f)
	t.Run("windows", func(t *testing.T) {
		asWindows(t)
		f(t)
	})
}

// runsAndRefused checks that each of runs goes through, and each of refused is refused, for a hire and the GM.
func runsAndRefused(t *testing.T, runs, refused []string) {
	t.Helper()
	for who, groups := range both {
		groups = WithGates(groups)
		for _, line := range runs {
			if r := Check(line, groups); r != nil {
				t.Errorf("%s: refused %q: %v", who, line, r)
			}
		}
		for _, line := range refused {
			if Check(line, groups) == nil {
				t.Errorf("%s: let through %q", who, line)
			}
		}
	}
}

// 6a: the tester's line was refused as claude-install, with no claude-install in it. The skeleton read the = of
// --untracked-files=all as a run that may be anything, a / included, and "all" ends claude-install. An = after a
// letter is a letter now; what follows it is read as a word of its own, and zsh's = at a word's start or after an = or
// a : stays a run.
func TestAnEqualsAfterALetterFillsInNoName(t *testing.T) {
	readings(t, func(t *testing.T) {
		runsAndRefused(t, []string{
			`git -C "C:/Users/someone/project/studio-app/baguette-studio" status --porcelain --untracked-files=all`,
			`git -C /home/someone/project/studio-app/baguette-studio status --porcelain --untracked-files=all`,
			`git status --untracked-files=all`,
			`make all install PREFIX=/usr/local TARGET=install`,
			`git log --format=hold --stat=key`,
		}, []string{
			"claude-install",
			"claude-install --version",
			`echo npm > ~/.config/smolcorp/claude-install`,
			`cp x ~/.config/smolcorp/claude-i*`,
			`cp x --target=claude-i*`,
			`cat --file=setup-k*`,
			`echo x > --x=setup-k*`,
			`cmd /c type x=setup-k*`,
			`cp x a.b=hire-g*`,
			`cat x==set*`,
			`cat x=a:=set*`,
			`cat =setup-key`,
		})
	})
}

// 6b: an interpreter's code read as a path pattern: in python -c '…;print(len(t),t[0])', the , and the [ around the t
// were runs that may be anything, a / included, so …/.gi, the t and /… filled in a .git folder. Code is no pattern
// now, but a git folder it names written out is refused, and so is a pattern in the interpreter's other words. (The
// last line ran before too: a ( ) group around the t is one run, which writes no letter.)
func TestAnInterpretersCodeIsNoGitFolderPattern(t *testing.T) {
	readings(t, func(t *testing.T) {
		runsAndRefused(t, []string{
			`smolcorp gm team --json | python3 -c 'import json,sys; d=json.load(sys.stdin); t=d["team"];print(len(t),t[0])'`,
			`smolcorp gm team --json | python -c "import json,sys; d=json.load(sys.stdin); t=d['team'];print(len(t),t[0])"`,
			`smolcorp gm team --json | py -c "import json,sys;t=json.load(sys.stdin)['team'];print(len(t),t[0]['name'])"`,
			`smolcorp gm team --json | python3 -I -c 'import json,sys;t=json.load(sys.stdin)["team"];print(len(t),t[-1])'`,
			`smolcorp gm team --json | node -e "let s='';process.stdin.on('data',c=>s+=c).on('end',()=>{const t=JSON.parse(s).team;console.log(t.length,t[0])})"`,
			`smolcorp gm team --json | python3 -c 'import json,sys; t=json.load(sys.stdin); print(t)'`,
		}, []string{
			`cp x .g*/HEAD`,
			`python3 -c 'open(".git/HEAD","w").write("x")'`,
			`python -c "open('C:\\repo\\.git\\config','a')"`,
			`python3 -c 'import sys; open(sys.argv[1],"w")' .g?t/HEAD`,
			`python3 -m x .g*/HEAD`,
			`node -e 'require("fs").writeFileSync(".git/config","")'`,
		})
	})
}

// 6c: in Windows' reading, a word of powershell -Command was read piece by piece (psPieces), and the piece claude.exe
// of -Filter "Name='claude.exe'" was the whole line it ran. A command that runs its words as a line gets each piece
// with the word's text around it now, as PowerShell hands it on.
func TestAPowerShellFilterNamesNoCommand(t *testing.T) {
	readings(t, func(t *testing.T) {
		runsAndRefused(t, []string{
			`Get-CimInstance Win32_Process -Filter "Name='claude.exe'" | Select-Object ProcessId,CommandLine`,
			`Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'claude.exe' } | Select-Object ProcessId,CommandLine`,
			`powershell -NoProfile -Command "Get-CimInstance Win32_Process -Filter \"Name='claude.exe'\" | Select-Object ProcessId,CommandLine"`,
			`powershell.exe -NoProfile -Command "Get-CimInstance Win32_Process -Filter 'Name=\"claude.exe\"' | Select-Object ProcessId,CommandLine"`,
			`powershell.exe -NoProfile -Command 'Get-CimInstance Win32_Process -Filter "Name=''claude.exe''" | Select-Object ProcessId,CommandLine'`,
			`pwsh -NoProfile -c "Get-CimInstance Win32_Process | Where-Object { \$_.Name -eq 'claude.exe' } | Select-Object ProcessId,CommandLine"`,
		}, []string{
			`claude.exe -p hi`,
			`& 'claude.exe' -p hi`,
			`powershell -Command "claude.exe -p hi"`,
			`powershell -Command 'x;'claude`,
			`pwsh -c "Get-Process; claude.exe"`,
			`Start-Process claude.exe`,
			`cmd /c "claude.exe -p hi"`,
		})
	})
	// In PowerShell's own reading, a word that starts with a quote ends at its close: 'env'claude is env claude, which
	// -Command runs as a line (bash hands it envclaude, one word).
	asWindows(t)
	runsAndRefused(t, nil, []string{`powershell -Command 'env'claude`, `powershell -Command 'git'push`})
}

// 6d: a GM's brief from a here document was read as commands, and its arrow refused as not written plainly. gm's
// commands that take their text from standard input read the body as text now; a shell, smolcorp run and any other
// gm command still have it read as commands.
func TestAGMsBriefFromAHereDocumentIsText(t *testing.T) {
	brief := "Step 1 → step 2: git push is the owner's"
	readings(t, func(t *testing.T) {
		for _, line := range []string{
			"smolcorp gm brief ada --file - <<'EOF'\n" + brief + "\nEOF",
			"smolcorp gm hire --role dev --task-file - <<'EOF'\n" + brief + "\nEOF",
			"smolcorp gm send ada --file /dev/stdin <<'EOF'\n" + brief + "\nEOF",
		} {
			if r := Check(line, WithGates(ForGM(DefaultGroups()))); r != nil {
				t.Errorf("the GM: refused %q: %v", line, r)
			}
		}
		// A hire may send (gm brief and hire are the GM's).
		if line := "smolcorp gm send ada --file /dev/stdin <<'EOF'\n" + brief + "\nEOF"; Check(line, WithGates(DefaultGroups())) != nil {
			t.Errorf("a hire: refused %q", line)
		}
		runsAndRefused(t, nil, []string{
			"bash <<'EOF'\ngit push\nEOF",
			"sh <<'EOF'\necho →\nEOF",
			"pwsh <<'EOF'\ngit push\nEOF",
			"smolcorp run bash <<'EOF'\ngit push\nEOF",
			"smolcorp gm team <<'EOF'\ngit push\nEOF",
			"smolcorp <<'EOF' gm brief ada --file -\ngit push\nEOF",
			"smolcorp gm brief ada --file - <<EOF\n$(git push)\nEOF",
			"smolcorp gm send ada --file /dev/stdin <<EOF\n`git push`\nEOF",
		})
	})
}

// 6e: a branch the guard can't place (after -C, where it can't read the default) is still taken as a base branch, but
// the refusal says how to run it; main stays a base branch by its name.
func TestABranchTheGuardCantPlaceSaysHowToRunIt(t *testing.T) {
	readings(t, func(t *testing.T) {
		for who, groups := range both {
			groups = WithGates(groups)
			r := Check("git -C ../app branch -d feature/x", groups)
			if r == nil || r.Group != MergeGroup || !strings.Contains(r.Why, "feature/x is the repository's default branch") ||
				!strings.Contains(r.Why, "from the repository's own folder: run it there, without -C") {
				t.Errorf("%s: %v", who, r)
			}
			if r := Check("git -C ../app branch -D main", groups); r == nil || !strings.HasPrefix(r.Why, "This moves main, a base branch") {
				t.Errorf("%s: main: %v", who, r)
			}
		}
	})
}

// 7: gh release's reads run; its writes are refused, plainly, through npx and with -R first. The rest of the publish
// group is as it was.
func TestGhReleaseReadsRunAndItsWritesDont(t *testing.T) {
	readings(t, func(t *testing.T) {
		var refused []string
		for _, sub := range []string{"create v1", "new v1", "upload v1 x.zip", "delete v1 --yes", "edit v1 --draft=false", "delete-asset v1 x.zip"} {
			refused = append(refused, "gh release "+sub, "npx gh release "+sub, "gh release -R ada/app "+sub)
		}
		runsAndRefused(t, []string{
			"gh release list", "gh release ls", "gh release view v1", "gh release download v1 -p '*.zip'",
			"gh release verify v1", "gh release verify-asset v1 x.zip", "npx gh release list", "gh release -R ada/app view v1",
		}, refused)
	})
	var release []Rule
	n := 0
	for _, g := range DefaultGroups() {
		if g.Name == "publish" {
			n = len(g.Commands)
			for _, r := range g.Commands {
				if strings.HasPrefix(string(r), "gh release") {
					release = append(release, r)
				}
			}
		}
	}
	if n != 28 || len(release) != 1 || release[0] != "gh release create|new|delete|delete-asset|edit|upload" {
		t.Errorf("publish: %d rules, gh release's %q", n, release)
	}
}
