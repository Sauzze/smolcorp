package guard

import (
	"strings"
	"testing"
)

// RT126GUARDWIN126's follow-ups: what PowerShell and cmd start through a word before the command (N1), the setup key
// written before cmd's = (N5), and the words for a branch the guard couldn't place where the command runs (N4).

// N1: PowerShell's dot (. claude.exe), cmd's call, @ and if, Invoke-Item (ii) and Windows PowerShell's first plain word
// each start the word after them, so Claude Code started through one is refused as starting it again, and git push
// through one is refused as a push. A line written plainly and the same line glued from quoted pieces (Windows'
// reading) are both refused; the controls, the same words starting nothing the guard keeps, still run.
func TestADotCallAtIfOrInvokeItemStartsTheWordAfterIt(t *testing.T) {
	readings(t, func(t *testing.T) {
		runsAndRefused(t, []string{
			`. ./venv/bin/activate`,
			`. "$HOME/.cargo/env"`,
			`call ls`,
			`@echo off`,
			`cmd /c "if exist build rmdir /s /q build"`,
			`cmd /c "if defined GIT echo x"`,
			`if ($x -eq 1) { git status }`,
			`ii README.md`,
			`ii .`,
			`Invoke-Item -Filter claude.exe .`,
			`powershell Get-Date`,
			`powershell -NoProfile -Command Get-Date`,
			`powershell -File x.ps1`,
		}, []string{
			`. claude.exe`,
			`. claude`,
			`powershell -Command ". claude.exe -p hi"`,
			`call claude`,
			`cmd /c "call claude"`,
			`cmd /c call claude`,
			`cmd /c "call git push"`,
			`cmd /c "@ claude"`,
			`cmd /c "@claude"`,
			`cmd /c "@echo off & claude"`,
			`cmd /c "if 1==1 claude"`,
			`cmd /c "if 1==1 git push"`,
			`cmd /c "if /i not a==b claude"`,
			`cmd /c "if a == b claude"`,
			`cmd /c "if a ==b claude"`,
			`cmd /c "if a== b claude"`,
			`cmd /c "if a equ b claude"`,
			`cmd /c "if exist x claude"`,
			`cmd /c "if exist x (claude)"`,
			`cmd /c "if errorlevel 1 claude"`,
			`cmd /c "if not defined X claude -p hi"`,
			`ii claude.exe`,
			`ii -Path claude.exe`,
			`ii -LiteralPath:claude.exe`,
			`ii x,claude.exe`,
			`Invoke-Item claude.exe`,
			`powershell -Command "ii claude.exe"`,
			`powershell claude`,
			`powershell -NoProfile claude -p hi`,
			`powershell -WindowStyle Hidden claude -p hi`,
			`powershell -ExecutionPolicy Bypass claude`,
			`powershell -Command "powershell claude"`,
			`powershell git push`,
		})
	})
	// Glued from quoted pieces, as PowerShell hands each piece on (bash hands cmd callclaude, one word).
	asWindows(t)
	runsAndRefused(t, nil, []string{
		`cmd /c 'call'"claude"`,
		`powershell -Command '. '"claude"`,
		`iex '@'"claude"`,
		`iex '. '"claude"`,
		`powershell -Command 'ii '"claude.exe"`,
	})
}

// N5: cmd splits a word at its =, so the setup key (or the window key) written before one is the key. Written after
// one, as before, it is the key too; a name that only starts with setup-key, or a value that only ends with it, isn't.
func TestTheSetupKeyWrittenBeforeAnEqualsIsTheKey(t *testing.T) {
	readings(t, func(t *testing.T) {
		runsAndRefused(t, []string{
			`echo key=value`,
			`make setup-keys=1`,
			`git config user.name=setup`,
		}, []string{
			`cmd /c del %APPDATA%\smolcorp\setup-key=x`,
			`cmd /c del setup-key=x`,
			`cmd /c type setup-key=x`,
			`cmd /c type setup-key.=x`,
			`cmd /c del SETUP-~1=x`,
			`cmd /c del window-key=x`,
			`grep -fsetup-key=x`,
			`dd if=setup-key of=x`,
		})
		for who, groups := range both {
			if r := Check(`cmd /c del setup-key=x`, WithGates(groups)); r == nil || r.Rule != setupKeyFile {
				t.Errorf("%s: %v, want the setup key's rule", who, r)
			}
		}
	})
}

// N4: a branch the guard couldn't place, run alone from the folder it runs in, says the guard couldn't read the
// repository there (telling it to run there would say nothing); with -C, cd or a wrapper the words say how to run it.
func TestABranchTheGuardCouldntPlaceWhereItRunsSaysItCouldntReadTheRepository(t *testing.T) {
	readings(t, func(t *testing.T) {
		for who, groups := range both {
			groups = WithGates(groups)
			r := Check("git branch -d feature/x", groups)
			if r == nil || r.Group != MergeGroup || !strings.Contains(r.Why, "feature/x is the repository's default branch") ||
				!strings.HasSuffix(r.Why, defaultUnreadWhy) || strings.Contains(r.Why, "without -C") {
				t.Errorf("%s: alone: %v", who, r)
			}
			for _, line := range []string{"git -C ../app branch -d feature/x", "cd ../app && git branch -d feature/x", "env git branch -d feature/x"} {
				if r := Check(line, groups); r == nil || !strings.HasSuffix(r.Why, defaultHowWhy) {
					t.Errorf("%s: %s: %v", who, line, r)
				}
			}
		}
	})
}
