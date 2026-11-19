package guard

import (
	"runtime"
	"slices"
	"strings"
	"testing"
)

// hostWindows is onWindows as the package set it, before TestMain reads every line as a Mac's or Linux's.
var hostWindows bool

// asWindows reads the lines of one test as the hook does on Windows: the line itself as PowerShell's and cmd's too,
// with their wildcards (onWindows).
func asWindows(t *testing.T) {
	t.Helper()
	onWindows = true
	t.Cleanup(func() { onWindows = false })
}

// both is a hire's groups and the GM's: every row below holds for each.
var both = map[string][]Group{"a hire": DefaultGroups(), "the GM": ForGM(DefaultGroups())}

func refusedForBoth(t *testing.T, lines []string) {
	t.Helper()
	for name, groups := range both {
		for _, line := range lines {
			if Check(line, groups) == nil {
				t.Errorf("%s: let through: %q", name, line)
			}
		}
	}
}

func runsForBoth(t *testing.T, lines []string) {
	t.Helper()
	for name, groups := range both {
		for _, line := range lines {
			if r := Check(line, groups); r != nil {
				t.Errorf("%s: refused %q: %v", name, line, r)
			}
		}
	}
}

// FXGUARDPS1R1 B6: the line itself is read with PowerShell's and cmd's wildcards only where the hook runs on Windows.
func TestTheWindowsReadingFollowsTheHost(t *testing.T) {
	if hostWindows != (runtime.GOOS == "windows") {
		t.Errorf("onWindows was %v on %s", hostWindows, runtime.GOOS)
	}
}

// GUARDPS1 (RT126NS1R2, SIERRA's rules): PowerShell's -Path and cmd's del, copy, type and move fill in a * or ? in
// quotes, where bash doesn't. Read as PowerShell's and cmd's, a quoted wildcard that could fill in a name the guard
// keeps counts as filling it in, for a hire and for the GM; on Windows the guard is the only layer.
func TestAQuotedWildcardOnWindowsIsRefused(t *testing.T) {
	asWindows(t)
	const (
		ps   = `$env:APPDATA\SauzzeDeck Pro\`
		cmd  = `%APPDATA%\SauzzeDeck Pro\`
		full = `C:\Users\x\AppData\Roaming\SauzzeDeck Pro\`
	)
	refusedForBoth(t, []string{
		// The reviewer's commands, on this tree's kept names (the night file and the GM's memory: the next test).
		`Get-Content "` + ps + `gm-t*"`, `Set-Content "` + ps + `gran*.jsonl" x`, `Set-Content "` + ps + `hire-g*" x`,
		`Set-Content "` + ps + `ph*.json" x`, `Remove-Item "` + ps + `hire-g*"`, `del "` + cmd + `hire-g*"`,
		// The other -Path cmdlets, cmd's other commands, single quotes, ?, PowerShell's [ ], a backtick before the *.
		`Clear-Content "` + ps + `hire-g*"`, `Add-Content -Path "` + ps + `hire-g?ard.json" x`, `Copy-Item x "` + ps + `hire-g*"`,
		`Move-Item x "` + ps + `phones.js?n"`, `"x" | Out-File -FilePath "` + ps + `hire-g*"`, `"x" > "` + ps + `hire-g*"`,
		`Set-Content '` + full + `hire-g*' x`, `Remove-Item '` + full + `setup-k*'`, `Get-Content "` + ps + `setup-k?y"`,
		`Set-Content "` + ps + `hire-gu[a]rd.json" x`, "Remove-Item \"" + ps + "hire-g`*\"", "Remove-Item hire-g`*",
		`copy x "` + cmd + `hire-g*"`, `move x "` + cmd + `gran*.jsonl"`, `type "` + cmd + `gm-t*"`, `type "` + cmd + `push-k*"`,
		`del "hire-g*"`, `Remove-Item ".\gm-t*"`, `Set-Content "grants.json?" x`,
		// Through cmd /c and PowerShell's -Command.
		`cmd /c del "` + cmd + `hire-g*"`, `cmd /c "del "` + cmd + `hire-g*""`, `powershell -Command "Remove-Item '` + full + `hire-g*'"`,
		`pwsh -c "Set-Content -Path '.\hire-g*' x"`,
		// RT126GUARDPS1 B1: a here-string, @"…"@ or @'…'@, is one string; the newlines at its ends are no part of it.
		"Set-Content @\"\n" + ps + "hire-g*\n\"@ x", "Remove-Item -Path @\"\n" + ps + "gm-t*\n\"@",
		"Get-Content @'\n" + full + "setup-k*\n'@", "Set-Content @\"\n.cl*\\settings.json\n\"@ x",
		"Set-Content @\"  \r\nC:\\x\\hire-g*\r\n\"@ x", "Set-Content -Path:@\"\nC:\\x\\hire-g*\n\"@ x", "Set-Content @'\nhire-g*", // never closed
		// B2: -LiteralPath is no exemption: after -- it is a positional string and the next word is -Path, and a
		// function can take a cmdlet's name.
		`Select-String -NotMatch -- -LiteralPath "` + ps + `setup-k*"`, `Select-String -NotMatch -- -LiteralPath "` + ps + `push-key-k*"`,
		`Select-String -NotMatch -- -LiteralPath "` + ps + `ph*s.json"`, `Copy-Item -- -LiteralPath "` + full + `hire-g*"`,
		`Microsoft.PowerShell.Utility\Select-String -NotMatch -- -LiteralPath "C:\x\setup-k*"`,
		`${function:Remove-Item} = { Microsoft.PowerShell.Management\Remove-Item -Path $args[1] }; Remove-Item -LiteralPath "C:\x\hire-g*"`,
		`Remove-Item -LiteralPath "C:\proj\hire-g*"`, `Set-Content -LiteralPath:'C:\proj\hire-g*' x`, `remove-item -literalpath 'setup-k*'`,
		`del -LiteralPath "hire-g*"`, `Remove-Item -LP "hire-g*"`, `Remove-Item "-LiteralPath" "hire-g*"`,
		// B3: -Name:value's value is a path of its own.
		`Set-Content -Path:'.cl*\settings.json' x`, `Set-Content -Path:'hire-g*' x`, `Set-Content -Path:hire-g* x`, `Get-Content -Path:"setup-k*"`,
		// B4: cmd matches the Win32 way: *. is a name with no extension, an 8.3 short name is matched too, < and > are
		// wildcards. And a short name written out is the name it stands for.
		`type "` + cmd + `push-key-k*."`, `type "push-key-k*."`, `type "` + cmd + `gm-t*."`, `del "` + cmd + `grants.js*."`,
		`del "` + cmd + `*.jso"`, `del "` + cmd + `hire-g*.jso"`, `del "hire-g*.jso"`, `del gm-t*.*`, `type "` + cmd + `setup-ke??"`,
		`Set-Content "` + ps + `HIRE-G~?.JSO" x`, `type "` + cmd + `gm-t<"`, `type "` + cmd + `setup-ke>"`, `del "` + cmd + `hire-g<"`,
		`type "` + cmd + `PHONES~1.JSO"`, `type "` + cmd + `PUSH-K~1"`, `del "` + cmd + `GRANTS~1.JSO"`, `Set-Content "` + ps + `HIRE-G~1.JSO" x`,
		`Set-Content "$env:APPDATA\SAUZZE~1\HIRE-G~1.JSO" x`, `type "%APPDATA%\SAUZZE~1\*.jso"`, `Set-Content "CLAUDE~1\SETTIN~1.JSO" x`,
		`Set-Content ".claude\SETTIN~1.JSO" x`, `del "` + cmd + `HI1A2B~1.JSO"`, `type "` + cmd + `GM-TOK~2"`, `type "` + cmd + `SETUP-~1"`,
		// RT126GUARDPS1R2 N1: letters past a < with no folder before it, and a ~? on Claude Code's settings, where only the
		// short-name reading finds them.
		`type "push-key-k<x"`, `Set-Content "CLAUDE~?\settings.json" x`, `Set-Content "CLAUDE~*\settings.json" x`,
		`Set-Content ".claude\SETTIN~?.JSO" x`,
		// B5: the paired phones' file by a pattern, reading included.
		`Get-Content "` + ps + `ph*s.json"`, `type "` + cmd + `ph*s.json"`, `Select-String . "` + ps + `ph*s.json"`,
		`Get-ChildItem "` + full[:len(full)-1] + `" -Filter "ph*s.json" | Get-Content`,
		// On Windows the line itself may be PowerShell's, so a quoted pattern given to find or git that could fill in a
		// kept name is refused there (RWIN3's find . -name '*key*' and RWIN10's git rm -r --cached -- '.c*' run on a Mac
		// and on Linux, B6).
		"find . -name '*key*'", "git rm -r --cached -- '.c*'",
		// The unquoted forms, refused before.
		`Set-Content ` + full[:len(full)-len(`SauzzeDeck Pro\`)] + `Sauzze*\hire-g* x`, `Remove-Item hire-g*`, `del gm-t*`,
		`del %APPDATA%\Sauzze*\hire-g*`, `Remove-Item ~\AppData\Roaming\*\gm-t*`,
	})
}

// MSYGUARDPS1: the names the chain keeps besides GUARDPS1's get the same reading on Windows: the night shift's jobs
// (NS1, kept as the guard file is: refused unless the command only reads) and the GM's memory (MEM1, refused to every
// command, reading included, as always says), for a hire and for the GM.
func TestAQuotedWildcardOnWindowsKeepsTheNightFileAndTheMemory(t *testing.T) {
	asWindows(t)
	const (
		ps   = `$env:APPDATA\SauzzeDeck Pro\`
		cmd  = `%APPDATA%\SauzzeDeck Pro\`
		full = `C:\Users\x\AppData\Roaming\SauzzeDeck Pro\`
	)
	refusedForBoth(t, []string{
		// The night shift's jobs.
		`Remove-Item "` + full + `night*"`, `Remove-Item "` + ps + `night*"`, `del "` + cmd + `night*"`, `Set-Content "` + ps + `night*" x`,
		`Set-Content '` + full + `nightshift.js?n' x`, `Copy-Item x "` + ps + `night*"`, `"x" > "` + ps + `night*"`,
		`move x "` + cmd + `nights*.jso"`, `del "` + cmd + `NIGHTS~1.JSO"`, "Remove-Item \"" + ps + "night`*\"",
		`cmd /c del "` + cmd + `night*"`, `pwsh -c "Remove-Item '` + full + `night*'"`,
		// The GM's memory, its folder and the file in it.
		`Set-Content "` + full + `gm-mem*" x`, `Set-Content "` + ps + `gm-mem*" x`, `Set-Content "` + ps + `gm-mem*\manifest.json" x`,
		`Remove-Item "` + ps + `gm-m*" -Recurse`, `del "` + cmd + `gm-mem*"`, `Get-Content "` + ps + `gm-mem*\manifest.json"`,
		`type "` + cmd + `GM-MEM~1\manifest.json"`, `cmd /c del "` + cmd + `gm-mem*"`, `pwsh -c "Set-Content '` + full + `gm-mem*' x"`,
	})
	// Reading the night shift's jobs by its name, as Get-Content hire-guard.json does.
	runsForBoth(t, []string{`Get-Content "` + ps + `nightshift.json"`, `type "` + cmd + `nightshift.json"`})
}

// What still runs on Windows: ordinary commands with quoted wildcards on names the guard doesn't keep, short names
// that aren't one of its names, a here-string that names none, and a read of the guard file by its name or short name.
func TestAQuotedWildcardOnWindowsStillRunsOnOtherNames(t *testing.T) {
	asWindows(t)
	runsForBoth(t, []string{
		`Get-ChildItem "C:\proj\*.go"`, `del "build\*.tmp"`, `del "build\*."`, `Remove-Item "dist\*" -Recurse`, `Get-Content "logs\app-*.log"`,
		`Select-String -Path "src\*.go" -Pattern "TODO"`, `type "notes\*.txt"`, `copy "a\*.txt" b`, `Get-ChildItem 'C:\proj\*.json'`,
		`Remove-Item "$env:TEMP\build-*"`, `Set-Content "out\report-?.txt" x`, `Get-ChildItem -Path:"src\*.go"`, `Get-ChildItem -Path:src\*.go`,
		`cmd /c del "build\*.obj"`, `powershell -Command "Remove-Item 'dist\*.zip'"`,
		// ~ in a name that is no kept one's short name, and git's revisions.
		"git log HEAD~1", "git diff HEAD~2 -- src", `type "C:\PROGRA~1\Git\etc\*.txt"`,
		`Remove-Item "C:\Users\RUNNER~1\AppData\Local\Temp\build-*"`, `Get-Content "$env:TEMP\x~1.log"`,
		"git commit -m @\"\nFix the build\n\"@", "git commit -m @'\nRefuse a pattern on hire-guard.json\n'@", "Write-Output @\"\nhello $name\n\"@",
		// Reading the guard file, as Get-Content hire-guard.json does.
		`Get-Content "$env:APPDATA\SauzzeDeck Pro\hire-guard.json"`, `type "%APPDATA%\SauzzeDeck Pro\HIRE-G~1.JSO"`,
	})
}

// FXGUARDPS1R1 B6: on a Mac or Linux the line is bash's or zsh's, and a pattern in quotes is text to it, so these
// ordinary lines run (RT126GUARDPS1 measured 19 of 67 refused when every line was read with Windows' wildcards). What
// a wrapper hands PowerShell or cmd is still read with theirs, on every host.
func TestAQuotedPatternInABashLineRunsOnAMacOrLinux(t *testing.T) {
	runsForBoth(t, []string{
		"find . -name '*key*'", "find . -name '*token*'", "find . -name '*setup*'", "find . -name '*guard*'", "find . -name '*grants*'",
		"find . -name '*.jsonl'", "find . -name '*settings*'", "find . -name '*phones*'", "find . -name 'phones*'", "find . -iname '*phone*'",
		"find . -path '*/.claude/*'", "find . -name '.c*'", "git ls-files '*setup*'", "git ls-files '*guard*'", "git add -- '*.jsonl'",
		"git status -- '*.jsonl'", "git rm -r --cached -- '.c*'", "go test -run 'Test.*Token' ./desktop/", "pnpm vitest run 'src/*key*'",
	})
	refusedForBoth(t, []string{
		`pwsh -c "Set-Content 'C:\x\hire-g*' x"`, `cmd /c del "C:\x\hire-g*"`, `powershell -Command "del 'C:\x\gm-t*.'"`,
		`iex 'Remove-Item "C:\x\hire-g*"'`, `Start-Process pwsh -ArgumentList '-c', 'Remove-Item "C:\x\hire-g*"'`,
		`sh -c 'pwsh -c "Remove-Item C:\x\hire-g*"'`, "pwsh -c 'Set-Content @\"\nC:\\x\\hire-g*\n\"@ x'",
		`cmd /c type "%APPDATA%\SauzzeDeck Pro\PUSH-K~1"`, `pwsh -c "Select-String -NotMatch -- -LiteralPath 'C:\x\setup-k*'"`,
		`pwsh -c "Set-Content -Path:'.cl*\settings.json' x"`, `cmd /c del "%APPDATA%\SauzzeDeck Pro\*.jso"`,
	})
}

// RT126GUARDPS1R2 B1: text in the same word hides no path from PowerShell's reading. An unquoted comma makes an array,
// and -Path takes each item (x,'.cl*\settings.json'); an argument that starts with a quote may end at its close
// ('.cl*\settings.json'x). Each piece is read as a word of its own (psPieces), so the rule holds however the word is
// spelled around the path, for a hire and for the GM.
func TestPowerShellPiecesOfAWordOnWindows(t *testing.T) {
	pieces := []string{
		// The reviewer's rows, and the quote-start ones.
		`Set-Content x,'.cl*\settings.json' y`, `Set-Content x,".cl*\settings.json" y`, `Set-Content x,'.cl?ude\settings.json' y`,
		`Set-Content '.cl*\settings.json',x y`, `Remove-Item 'x','.cl*\settings.json'`, `Set-Content -Path x,'.cl*\settings.json' -Value y`,
		`Set-Content -Path:x,'.cl*\settings.json' y`, `Remove-Item x,'.cl*' -Recurse -Force`, "Set-Content 'x',@'\n.cl*\\settings.json\n'@ y",
		`Remove-Item x,'SAUZZE~1\*.jso'`, `Set-Content '.cl*\settings.json'x`, `Set-Content ".cl*\settings.json"x`,
		`Set-Content '.cl?ude\settings.json'x`,
		// Nearby: unquoted items, more of them, blanks around the comma, a quote mid-item, other kept names, no wildcard.
		`Set-Content x,.cl?ude\settings.json y`, `Remove-Item x,hire-g*`, `Remove-Item x,y,'hire-g*',z`, `Remove-Item x, '.cl*' -Recurse`,
		`Set-Content x,'.claude\settings.json' y`, `Get-Content x,"$env:APPDATA\SauzzeDeck Pro\gm-token"`, `Get-Content x,'PHONES~1.JSO'`,
		`Set-Content x,'CLAUDE~1\SETTIN~1.JSO' y`, `Remove-Item x,'.cl'*'\settings.json'`, `Set-Content '.cl*\settings.json'"x" y`,
		`Set-Content -Path:'.cl*\settings.json'x y`, `Set-Content -Path:x,.cl*\settings.json y`, "Set-Content x,\n'.cl*\\settings.json' y",
		`Set-Content @(x,'.cl*\settings.json') y`, `del x,"hire-g*"`, `type x,"%APPDATA%\SauzzeDeck Pro\gm-t*"`,
	}
	wrapped := []string{
		`pwsh -c "Set-Content x,'.cl*/settings.json' y"`, `pwsh -c "Remove-Item x,'.cl*' -Recurse -Force"`,
		`powershell -Command "Remove-Item x,'.cl*' -Recurse"`, `cmd /c del x,"hire-g*"`,
	}
	ordinary := []string{
		"Remove-Item a.txt,b.txt", `Get-ChildItem src\*.go,docs\*.md`, `Get-ChildItem "src\*.go","docs\*.md"`, `Copy-Item 'a.txt','b.txt' dist`,
		`Remove-Item "dist\*","build\*" -Recurse`, `Get-Content "logs\a-*.log",'logs\b-?.log'`, `Get-ChildItem -Path:src\*.go,docs\*.md`,
		`Write-Output "a"b`, `git log --format="%h %s" -5`, `git commit -m "Fix the build, again"`, `echo "x"'y'"z"`,
		"gh pr list --json number,title", `git log --pretty=format:"%h,%an,%s"`, "cat a,b.txt",
	}
	// On a Mac or Linux the line is bash's (FXGUARDPS1R1 B6), and what a wrapper hands PowerShell or cmd is theirs.
	refusedForBoth(t, wrapped)
	runsForBoth(t, ordinary)
	asWindows(t)
	refusedForBoth(t, append(pieces, wrapped...))
	runsForBoth(t, ordinary)
}

// RT126GUARDPS1R2 B1: the pieces of a word, from its start, a comma or a quote's edge, to its end, a comma or a
// quote's close, never across a comma, each read by the command in the word's place; too many, and the line is refused.
func TestAPowerShellWordsPiecesAreWhereItMaySplit(t *testing.T) {
	for line, want := range map[string]string{
		`x a,'b' y`:   "x a,b y | x a y | x b y",
		`x 'a'b`:      "x ab | x a | x b",
		`x a,'b'c`:    "x a,bc | x a | x b | x bc | x c",
		"x a`,b":      "x a,b",
		`x "a,b"`:     "x a,b",
		`x 'a''b'`:    "x ab | x a | x b",
		`x --m='a' y`: "x --m=a y | x a y",
		`x > 'a'b`:    "\x00redir ab | \x00redir a | \x00redir b | x",
	} {
		var got []string
		for _, cmd := range commandsRead(line, 0, true, false, true, false) {
			got = append(got, strings.Join(cmd, " "))
		}
		if strings.Join(got, " | ") != want {
			t.Errorf("%s: %q, want %q", line, strings.Join(got, " | "), want)
		}
	}
	if cmds := commandsRead(`x a,'b'`, 0, true, false, false, false); len(cmds) != 1 {
		t.Errorf("read without Windows' wildcards: %q, want the word whole", cmds)
	}
	var edges []int
	for i := 0; i < 30; i++ {
		edges = append(edges, 3*i+1)
	}
	if _, ok := psPieces(strings.Repeat("ab,", 30), slices.Repeat([]bool{false}, 90), edges, edges); ok {
		t.Error("30 quotes' edges in one word: read, want refused")
	}
	asWindows(t)
	refusedForBoth(t, []string{"Remove-Item " + strings.Repeat("'a'", 30)})
}

// RT126GUARDPS1 B1: the here-string's text is one word, the newline after its opening and the one before its close
// dropped; a "@ that doesn't start a line is text.
func TestAHereStringIsOneWord(t *testing.T) {
	for _, tc := range []struct{ line, want string }{
		{"Set-Content @\"\nC:\\x\\a b\n\"@ x", `C:\x\a b`},
		{"Set-Content @'\r\nC:\\x\\a\"@b\r\n'@ x", `C:\x\a"@b`},
		{"Set-Content @\"\n\"@ x", ""},
		{"Set-Content @\"\nC:\\x\\`$a\n\"@ x", `C:\x\$a`},
	} {
		cmds := commandsRead(tc.line, 0, true, false, false, true)
		if len(cmds) != 1 || len(cmds[0]) != 3 || cmds[0][1] != tc.want || cmds[0][2] != "x" {
			t.Errorf("%q: %q, want [Set-Content %q x]", tc.line, cmds, tc.want)
		}
	}
}

// RT126GUARDPS1 B4: an 8.3 short name is read as each name it may stand for.
func TestAShortNameIsReadAsTheNamesItMayBe(t *testing.T) {
	for part, want := range map[string]string{
		"HIRE-G~1.JSO": "hire-guard.json hire-guard", "PUSH-K~1": "push-key-kept", "se1a2b~1": "setup-key settings.local.json settings.json",
		"CLAUDE~1": "claude-install .claude", "SAUZZE~1": "sauzzedeck-pro SauzzeDeck Pro", "HEAD~1": "", "RUNNER~1": "", "~1": "", "x~y": "",
	} {
		if got := strings.Join(shortNameOf(part), " "); got != want {
			t.Errorf("%s: %q, want %q", part, got, want)
		}
	}
	if alts, ok := shortNameAlts(`CLAUDE~1\SETTIN~1.JSO`, slices.Repeat([]bool{false}, 21)); !ok ||
		!slices.ContainsFunc(alts, func(a alt) bool { return a.s == `.claude\settings.json` }) {
		t.Errorf("CLAUDE~1\\SETTIN~1.JSO: %v", alts)
	}
}

// RT126NS1, FXNS1R1, RT126GUARDPS1 B5: phones.json was in always only, which finds the name written out, so a
// pattern reached it. It is in protectedNames now, and a pattern that fills it in is refused as the name itself, to
// every command: off a Mac the file holds each phone's signing secret.
func TestThePairedPhonesFileIsRefusedByAPattern(t *testing.T) {
	const mac, linux = `~/Library/Application\ Support/AgentDeck/`, "~/.config/sauzzedeck-pro/"
	refusedForBoth(t, []string{
		"cp x ph*s.json", "cp x " + linux + "ph*s.json", "cp x " + mac + "p*.json", "rm " + linux + "phones.js?n",
		"tee " + linux + "pho*", "sed -i s/a/b/ " + linux + "ph?nes.json", "echo '[]' > " + linux + "ph*s.json", "x=pho?es.json; cp y $x",
		"sh -c 'cp x " + linux + "ph*s.json'", "cp x " + linux + "p[h]ones.json", "find " + linux + " -name 'ph*s.json' -delete",
		// Reading it, the way the reviewer did (B5).
		"cat " + linux + "ph?nes.json", "head -c 9999 " + linux + "ph?nes.json", "cat < " + linux + "ph?nes.json", "cat " + linux + "*.json",
		"git diff --no-index /dev/null " + linux + "ph?nes.json", "cat " + mac + "ph?nes.json", "jq . " + linux + "ph*s.json",
		"less -o " + linux + "ph?nes.json x", `pwsh -c "Get-Content '$env:APPDATA\SauzzeDeck Pro\ph*s.json'"`,
		// The cost, accepted (B5's option a): "age" is three letters of AgentDeck, so the * may be in the support folder.
		"cat package*.json",
	})
	runsForBoth(t, []string{"ls *.json", "ls desktop/phone*.go", "go test -run Phone ./desktop/", "cat desktop/phones.go", "cat package.json"})
	for _, line := range []string{"cat " + linux + "phones.json", "cp x " + linux + "ph*s.json", "cat " + linux + "ph?nes.json"} {
		if r := Check(line, DefaultGroups()); r == nil || r.Rule != phonesFile || strings.Contains(r.Command, "\x00") {
			t.Errorf("%s: %v", line, r)
		}
	}
}
