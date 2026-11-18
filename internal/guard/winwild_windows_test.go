package guard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// requirePwshEnv set to 1 turns a shell missing from PATH into a failure, not a skip: the CI windows job sets it, so
// the premise below is checked there and never passes unseen.
const requirePwshEnv = "SMOLCORP_REQUIRE_PWSH"

// The premise windowsWild rests on (GUARDPS1, FXGUARDPS1R1), pinned on a real Windows: PowerShell's -Path and cmd's
// del, type and copy fill in a * or ? written in quotes, double or single, or after a backtick; PowerShell's [ ] too,
// in a here-string, after > and in -LiteralPath after --; cmd's *. fills in a name with no extension, and its *.jso a
// .json by its 8.3 short name. Each shell is handed stand-in files in a temp folder, never a name the guard keeps, and
// the stand-in changing, going or being read is the proof. If this ever fails, the guard's reading of quoted
// wildcards on Windows needs another look. What isn't documented (cmd's < and >, PowerShell's wildcards on a short
// name) is only logged: the guard refuses those either way.
func TestWindowsShellsFillInAWildcardInQuotes(t *testing.T) {
	required := os.Getenv(requirePwshEnv) == "1"
	find := func(t *testing.T, name string) string {
		exe, err := exec.LookPath(name)
		if err != nil {
			if required {
				t.Fatalf("%s isn't on PATH, and %s=1 says it must be", name, requirePwshEnv)
			}
			t.Skipf("%s isn't on PATH: the premise isn't checked here", name)
		}
		return exe
	}
	standIn := func(t *testing.T, dir, name, text string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	read := func(t *testing.T, p string) string {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(p), err)
		}
		return strings.TrimSpace(string(data))
	}
	// short is a file's 8.3 short name, or "" when its volume makes none.
	short := func(p string) string {
		long, err := syscall.UTF16PtrFromString(p)
		if err != nil {
			return ""
		}
		buf := make([]uint16, 1024)
		n, err := syscall.GetShortPathName(long, &buf[0], uint32(len(buf)))
		if err != nil || n == 0 || int(n) > len(buf) {
			return ""
		}
		if name := filepath.Base(syscall.UTF16ToString(buf[:n])); !strings.EqualFold(name, filepath.Base(p)) {
			return name
		}
		return ""
	}

	for _, shell := range []string{"pwsh", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			exe := find(t, shell)
			dir := t.TempDir()
			double := standIn(t, dir, "standin-double.txt", "old")
			single := standIn(t, dir, "standin-single.txt", "old")
			escaped := standIn(t, dir, "standin-escaped.txt", "old")
			gone := standIn(t, dir, "standin-gone.txt", "old")
			standIn(t, dir, "standin-read.txt", "read through a quoted wildcard")
			bracket := standIn(t, dir, "standin-bracket.txt", "old")
			redirected := standIn(t, dir, "standin-redirected.txt", "old")
			here := standIn(t, dir, "standin-here.txt", "old")
			standIn(t, dir, "standin-dashes.txt", "read past the end of the parameters")
			first := standIn(t, dir, "standin-first.txt", "old")
			array := standIn(t, dir, "standin-array.txt", "old")
			script := filepath.Join(dir, "run.ps1")
			lines := []string{
				`Set-Content -Path "` + dir + `\standin-dou*.txt" -Value new`,
				`Set-Content '` + dir + `\standin-sin?le.txt' new`,
				"Set-Content \"" + dir + "\\standin-esc`*.txt\" new",
				`Remove-Item "` + dir + `\standin-go*.txt"`,
				`Get-Content "` + dir + `\standin-rea*.txt"`,
				`Set-Content "` + dir + `\standin-b[r]acket.txt" new`,
				`"new" > "` + dir + `\standin-redir*.txt"`,
				"Set-Content @\"", dir + `\standin-he*.txt`, "\"@ new",
				// After --, -LiteralPath is Select-String's -Pattern and the quoted wildcard its -Path (RT126GUARDPS1 B2).
				`Select-String -NotMatch -- -LiteralPath "` + dir + `\standin-dash*.txt"`,
				// A comma array: -Path fills in the wildcard of each item, the first one's text in front (RT126GUARDPS1R2 B1).
				`Set-Content '` + first + `','` + dir + `\standin-arr*.txt' new`,
			}
			if err := os.WriteFile(script, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(exe, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script).CombinedOutput()
			if err != nil {
				t.Fatalf("%s: %v\n%s", shell, err, out)
			}
			for _, p := range []string{double, single, escaped, bracket, here, first, array} {
				if got := read(t, p); got != "new" {
					t.Errorf("%s: %s holds %q: the quoted wildcard wasn't filled in\n%s", shell, filepath.Base(p), got, out)
				}
			}
			if _, err := os.Stat(gone); !os.IsNotExist(err) {
				t.Errorf("%s: Remove-Item with a quoted wildcard left %s (%v)\n%s", shell, filepath.Base(gone), err, out)
			}
			if !strings.Contains(string(out), "read through a quoted wildcard") {
				t.Errorf("%s: Get-Content with a quoted wildcard didn't read the stand-in\n%s", shell, out)
			}
			// > writes in the shell's own encoding (UTF-16 in Windows PowerShell): the stand-in no longer holds old.
			if got := read(t, redirected); got == "old" {
				t.Errorf("%s: > with a quoted wildcard left %s as it was\n%s", shell, filepath.Base(redirected), out)
			}
			if !strings.Contains(string(out), "read past the end of the parameters") {
				t.Errorf("%s: -LiteralPath after -- didn't fill in the wildcard\n%s", shell, out)
			}
			// Logged only: whether an argument that starts with a quote ends at its close, the rest the next argument
			// (RT126GUARDPS1R2 B1: '…'new as -Path '…' -Value new). The guard reads the word both ways.
			split := standIn(t, dir, "standin-quoted.txt", "old")
			out, _ = exec.Command(exe, "-NoProfile", "-NonInteractive", "-Command", `Set-Content '`+dir+`\standin-quo*.txt'new`).CombinedOutput()
			t.Logf("%s: Set-Content '…\\standin-quo*.txt'new set the stand-in: %v (%s)", shell, read(t, split) == "new", strings.TrimSpace(string(out)))
			// Logged only: whether PowerShell's -Path matches a wildcard against a short name.
			p := standIn(t, dir, "standin-shortname.json", "a short name filled in by PowerShell")
			if s := short(p); s != "" {
				stem, _, _ := strings.Cut(s, "~")
				out, _ := exec.Command(exe, "-NoProfile", "-NonInteractive", "-Command", `Get-Content '`+dir+`\`+stem+`~?.JSO'`).CombinedOutput()
				t.Logf("%s: Get-Content %s~?.JSO read the stand-in: %v", shell, stem, strings.Contains(string(out), "filled in by PowerShell"))
			} else {
				t.Logf("%s: no 8.3 names on this volume", shell)
			}
		})
	}

	t.Run("cmd", func(t *testing.T) {
		exe := find(t, "cmd")
		dir := t.TempDir()
		gone := standIn(t, dir, "standin-gone.txt", "old")
		standIn(t, dir, "standin-typed.txt", "typed through a quoted wildcard")
		standIn(t, dir, "standin-copied.txt", "copied")
		standIn(t, dir, "standin-noext", "typed through *.")
		long := standIn(t, dir, "standin-shortname.json", "typed through a short name's extension")
		copied := filepath.Join(dir, "copy-of-standin.txt")
		script := filepath.Join(dir, "run.cmd")
		lines := []string{
			"@echo off",
			`del "` + dir + `\standin-go*.txt"`,
			`type "` + dir + `\standin-ty?ed.txt"`,
			// /b: a wildcard source to one file is a combination, which /a would end with a Ctrl-Z.
			`copy /b /y "` + dir + `\standin-cop*.txt" "` + copied + `" >nul`,
			`type "` + dir + `\standin-noe*."`,
		}
		shortName := short(long)
		if shortName != "" {
			lines = append(lines, `type "`+dir+`\standin-sho*.jso"`)
		}
		if err := os.WriteFile(script, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(exe, "/d", "/c", script).CombinedOutput()
		if err != nil {
			t.Fatalf("cmd: %v\n%s", err, out)
		}
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("cmd: del with a quoted wildcard left %s (%v)\n%s", filepath.Base(gone), err, out)
		}
		if !strings.Contains(string(out), "typed through a quoted wildcard") {
			t.Errorf("cmd: type with a quoted wildcard didn't read the stand-in\n%s", out)
		}
		if got := read(t, copied); got != "copied" {
			t.Errorf("cmd: copy with a quoted wildcard gave %q\n%s", got, out)
		}
		if !strings.Contains(string(out), "typed through *.") {
			t.Errorf("cmd: *. in quotes didn't fill in a name with no extension\n%s", out)
		}
		if shortName == "" {
			t.Logf("cmd: no 8.3 names on this volume: *.jso by a short name isn't checked here")
		} else if !strings.Contains(string(out), "typed through a short name's extension") {
			t.Errorf("cmd: *.jso in quotes didn't fill in %s by its short name %s\n%s", filepath.Base(long), shortName, out)
		}
		// Logged only: whether cmd's type takes < and > as wildcards (Win32's DOS_STAR and DOS_QM).
		standIn(t, dir, "standin-angle.txt", "typed through < or >")
		for i, pattern := range []string{`standin-ang<`, `standin-angle.tx>`, `standin-angle<.txt`} {
			angle := filepath.Join(dir, fmt.Sprintf("angle%d.cmd", i))
			if err := os.WriteFile(angle, []byte("@echo off\r\ntype \""+dir+`\`+pattern+"\"\r\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			out, _ := exec.Command(exe, "/d", "/c", angle).CombinedOutput()
			t.Logf("cmd: type %q read the stand-in: %v", pattern, strings.Contains(string(out), "typed through < or >"))
		}
	})
}
