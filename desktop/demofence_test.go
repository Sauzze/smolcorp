package desktop

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// WIN125c: the fence fails closed. Every row of RWIN2's tables (B1–B5) is refused, read as Windows, the Mac and
// Linux read it, and the demo's own lines, as its task writes them, still go through.
func TestTheDemosFenceRefusesWhatItDoesntRead(t *testing.T) {
	in := demoTestFolder(t)
	hostRoot := filepath.Join(in, demoFolderName)
	hostRepo := demoCheckout(t, hostRoot, "tester-1")
	type system struct {
		goos, root, home, support, supportWord, outside string
		// supportGlob is a pattern that fills in the folder, wildcards in every part (RWIN3 N7).
		supportGlob string
	}
	systems := []system{
		{"windows", `C:\Users\ada\smolcorp-demo`, `C:\Users\ada`, `C:\Users\ada\AppData\Roaming\SauzzeDeck Pro`, `~/AppData/Roaming/SauzzeDeck\ Pro`, "C:/Users/ada/elsewhere",
			"~/App*/Roa*/Sau*"},
		{"darwin", "/Users/ada/smolcorp-demo", "/Users/ada", "/Users/ada/Library/Application Support/AgentDeck", `~/Library/Application\ Support/AgentDeck`, "/Users/Shared/rwin2-nowhere",
			"~/Lib*/App*/A*"},
		{"linux", "/home/ada/smolcorp-demo", "/home/ada", "/home/ada/.config/sauzzedeck-pro", "~/.config/sauzzedeck-pro", "/srv/rwin2-nowhere",
			"~/.config/s*"},
		// This computer's own reading, on its disk: the demo's lines are let through whole here.
		{"", hostRoot, homeDir, resolveExisting(filepath.Join(homeDir, "Library", "Application Support", "AgentDeck")),
			`~/Library/Application\ Support/AgentDeck`, "/Users/Shared/rwin2-nowhere", "~/Lib*/App*/A*"},
	}
	team := func() ([]demoMember, error) {
		return []demoMember{{ID: "tester-1", Folder: hostRepo, Worktree: filepath.Join(hostRoot, "app-hires", "tester-1"), Registered: true}}, nil
	}
	for _, s := range systems {
		f := demoFence{root: s.root, roles: []string{"reviewer", "tester"}, home: s.home, support: s.support, goos: s.goos, team: team}
		repo := f.paths().join(s.root, demoRepoName)
		folder := f.shellFolder(repo) // as the demo writes it for the GM
		out := s.outside
		tool := func(name string, input map[string]any) demoCall {
			data, _ := json.Marshal(input)
			return demoCall{ToolName: name, ToolInput: data, Cwd: repo}
		}
		bash := func(command string) demoCall { return bashCall(command, repo) }
		refused := []demoCall{
			// B1: a # comment or a redirection hides the real --folder, --role or --account.
			bash("smolcorp gm hire --role tester --worktree --folder " + out + " # --folder " + folder),
			bash("smolcorp gm hire --role tester --worktree --folder " + out + " > --folder " + folder),
			bash("smolcorp gm hire --role tester --worktree --folder " + out + " 2> --folder " + folder),
			bash("smolcorp gm hire --role codexer --worktree --folder " + folder + " # --role tester"),
			bash("smolcorp gm hire --role tester --worktree --folder " + folder + " --account work # --account default"),
			// B2: cd, --git-dir or GIT_DIR before git worktree add.
			bash("cd " + out + " && git --git-dir=" + folder + "/.git worktree add wt"),
			bash("cd " + out + " && GIT_DIR=" + folder + "/.git git worktree add wt"),
			bash("cd " + out + " && git worktree add wt -b x"),
			bash("git --work-tree=" + out + " worktree add wt"),
			bash("pushd " + out + " && git worktree add wt"),
			// B3: the setup key by a pattern, and its folder by Bash, Grep and Glob.
			bash(s.supportWord + "/setup-k?y"),
			bash("cat " + s.supportWord + "/setup-k?y"),
			bash("cat " + s.supportWord + "/setup-*"),
			bash("cat " + s.supportWord + `/setup-"k"ey`),
			bash("cat " + s.supportWord + `/setup-\key`),
			bash("grep -r . " + s.supportWord),
			bash("cat ~/.config/Smol*/setup-k?y"),
			tool("Grep", map[string]any{"path": s.support, "glob": "setup-k?y", "output_mode": "content"}),
			tool("Grep", map[string]any{"path": s.support, "pattern": "."}),
			tool("Grep", map[string]any{"path": s.home, "pattern": ".", "glob": "setup-*"}),
			tool("Glob", map[string]any{"pattern": s.support + "/*"}),
			// RWIN3 N7 and N1: patterns that fill in the key, from the line or from above the folder.
			bash("cat " + s.supportGlob + "/s*"),
			bash("cat " + s.supportGlob + "/g?-token*"),
			bash("grep -r . " + s.supportGlob),
			tool("Glob", map[string]any{"pattern": s.supportGlob + "/*"}),
			tool("Glob", map[string]any{"path": s.home, "pattern": "**/s*"}),
			tool("Glob", map[string]any{"pattern": s.home + "/**/*"}),
			tool("Grep", map[string]any{"path": s.home, "pattern": "."}),
			tool("Glob", map[string]any{"path": s.support, "pattern": "*"}),
			tool("Read", map[string]any{"file_path": s.support + "/setup-key"}),
			// B4: eval, env -S and a name built in a nested line.
			bash(`eval smolcorp gm "h'i're" --role tester --worktree --folder ` + out),
			bash(`eval smolcorp gm hi\\re --role tester --worktree --folder ` + out),
			bash(`eval "smol'corp'" mcp < m.txt`),
			bash(`eval smol\\corp mcp`),
			bash(`env -S 'smolcorp\_gm\_hire\_--role\_tester\_--worktree\_--folder\_` + out + `'`),
			bash(`env -S'smolcorp\_mcp' < m.txt`),
			bash(`env --split-string='smolcorp\_mcp'`),
			bash(`bash -c 'x=smol; ${x}corp mcp' < m.txt`),
			bash(`bash -c $'\x73molcorp mcp' < m.txt`),
			bash(`/bin/sh -lc 'smolcorp gm team'`),
			// B5: cmd's ^ and PowerShell, from Bash.
			bash("cmd //c smolcorp gm hi^re --role tester --worktree --folder C:/Users/ada/elsewhere"),
			bash(`cmd //c "smolcorp gm hi^re --role tester --worktree --folder C:/Users/ada/elsewhere"`),
			bash("cmd //c smolcorp gm ^hire --role tester --worktree --folder C:/Users/ada/elsewhere"),
			bash("cmd.exe /c smolcorp gm hi^re --role tester --worktree --folder C:/Users/ada/elsewhere"),
			bash("cmd //c smol^corp mcp"),
			bash("cmd //c 'smolcorp gm se^nd outsider-1 --text hi'"),
			bash(`powershell -Command "& ('smol'+'corp') mcp"`),
			bash(`/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -c "smolcorp mcp"`),
			bash(`pwsh -c "smolcorp mcp"`),
			bash("smolcorp gm hi^re --role tester --folder " + folder),
			// What a smolcorp line holds outside quotes that the fence doesn't read, each alone.
			bash("smolcorp gm team > team.txt"),
			bash("smolcorp gm team # who's here"),
			bash("smolcorp gm send tester-1 --text \"$(cat notes.md)\""),
			bash("smolcorp gm send tester-1 --text `cat notes.md`"),
			bash("smolcorp gm hire --role tester --folder " + folder + " --task-file <(echo x)"),
			bash("GIT_WORK_TREE=" + out + " smolcorp gm hire --role tester --folder " + folder),
			bash("env -C " + out + " smolcorp gm hire --role tester --folder ."),
		}
		for _, c := range refused {
			if why := f.check(c); !strings.HasPrefix(why, demoFenceRefusal) {
				t.Errorf("on %q, let through %s %s", s.goos, c.ToolName, c.ToolInput)
			}
		}
		// The fail-closed refusals say to run the plain command alone.
		if why := f.check(bash("smolcorp gm hire --role tester --folder " + folder + " # x")); !strings.Contains(why, "run the plain command alone") {
			t.Errorf("on %q, the refusal: %q", s.goos, why)
		}
		if why := f.check(bash("eval smolcorp gm team")); !strings.Contains(why, "run the plain command alone") {
			t.Errorf("on %q, the refusal: %q", s.goos, why)
		}

		// The demo's own lines, as its task writes them for the GM, and the work its hires do.
		task, _ := demoGMHire(demoFolder{root: s.root, repo: repo}, f.roles)["task"].(string)
		found := regexp.MustCompile("`(smolcorp gm hire [^`]+)`").FindStringSubmatch(task)
		if found == nil {
			t.Fatalf("no hire line in the GM's task:\n%s", task)
		}
		if s.goos != "" {
			// The task writes the folder for the computer it runs on: here, as this reading would.
			found[1] = strings.Replace(found[1], demoShellFolder(repo), folder, 1)
		}
		worktree := f.shellFolder(f.paths().join(f.paths().join(s.root, "app-hires"), "tester-1"))
		allowed := []demoCall{
			bash(strings.NewReplacer("<role>", "tester", "<what done means, and the proof>", "The lost cent is fixed in split(), and python3 -m unittest passes.").Replace(found[1])),
			bash("smolcorp gm hire --role reviewer --folder " + worktree + " --task 'Review hire/tester-1: git diff main...hire/tester-1 > notes, # the findings'"),
			bash("smolcorp gm hire --role tester \\\n  --folder " + folder + " \\\n  --worktree \\\n  --task 'Fix the rounding (see #3); report back <here>'"),
			bash("smolcorp gm team"),
			bash("smolcorp gm send tester-1 --text 'Run python3 -m unittest; then 2>&1 | tail is fine'"),
			bash("smolcorp gm brief tester-1 --text \"Fix #3, then say done\""),
			bash("cd " + worktree + " && python3 -m unittest 2>&1 | tail -5"),
			bash("git -C " + worktree + " diff main...hire/tester-1 > /dev/null; echo $?"),
			bash("git diff main...hire/tester-1 # the hire's change"),
			bash("git -C " + folder + " worktree list"),
			tool("Read", map[string]any{"file_path": f.paths().join(repo, "TASKS.md")}),
			tool("Grep", map[string]any{"path": repo, "pattern": "def split"}),
			tool("Glob", map[string]any{"pattern": "**/*.py"}),
			// RWIN3 N1: a * in the project, or a pattern Grep reads as a regex, is the project's, not the key.
			tool("Glob", map[string]any{"pattern": "*"}),
			tool("Glob", map[string]any{"pattern": "bill/*"}),
			tool("Grep", map[string]any{"pattern": "set*"}),
			tool("Grep", map[string]any{"pattern": ".", "glob": "setup-*"}),
			tool("Glob", map[string]any{"path": s.home, "pattern": "smolcorp-demo/**/*.py"}),
			tool("Write", map[string]any{"file_path": f.paths().join(s.root, demoReportName)}),
		}
		for _, c := range allowed {
			why := f.check(c)
			// Another system's reading has no disk of its own: a hire's folder, or a team member's, is never held
			// there. Nothing else refuses.
			disk := strings.Contains(why, "hire into the demo's project") || strings.Contains(why, `"tester-1" isn't one of them`)
			if why != "" && (s.goos == "" || !disk) {
				t.Errorf("on %q, refused %s %s: %s", s.goos, c.ToolName, c.ToolInput, why)
			}
		}
	}
	if runtime.GOOS == "windows" {
		t.Log("this computer's reading is Windows'; the Mac's and Linux's are read by their rules alone")
	}
}

// WIN125e (RWIN4 B2): the Mac's disk and Windows' read names case aside, so the app's support folder written in
// another case is the same folder there: every support rule holds for it, read as each system reads it. A name
// whose letter changes length as it folds (the Kelvin sign K is k) too. Linux's disk reads names by case: there
// it's another folder, and the fence lets a Read of it through.
func TestTheDemosFenceReadsTheSupportFolderInAnyCase(t *testing.T) {
	for _, s := range []struct {
		goos, root, home, support, other, tilde string
		caseAside                               bool
	}{
		{"windows", `C:\Users\ada\smolcorp-demo`, `C:\Users\ada`, `C:\Users\ada\AppData\Roaming\SauzzeDeck Pro`,
			`c:\USERS\Ada\appdata\ROAMING\sauzzedeck pro`, `~/APPDATA/roaming/SAUZZEDECK\ PRO`, true},
		{"darwin", "/Users/ada/smolcorp-demo", "/Users/ada", "/Users/ada/Library/Application Support/AgentDeck",
			"/users/ADA/library/APPLICATION SUPPORT/agentdec\u212a", `~/LIBRARY/application\ support/AGENTDECK`, true},
		{"linux", "/home/ada/smolcorp-demo", "/home/ada", "/home/ada/.config/sauzzedeck-pro",
			"/home/ada/.CONFIG/Sauzzedeck-Pro", "~/.CONFIG/Sauzzedeck-Pro", false},
	} {
		f := demoFence{root: s.root, roles: []string{"tester"}, home: s.home, support: s.support, goos: s.goos}
		repo := f.paths().join(s.root, demoRepoName)
		tool := func(name string, input map[string]any) demoCall {
			data, _ := json.Marshal(input)
			return demoCall{ToolName: name, ToolInput: data, Cwd: repo}
		}
		registry := f.paths().join(s.other, "registry.json")
		for _, c := range []demoCall{
			tool("Read", map[string]any{"file_path": registry}),
			tool("Read", map[string]any{"file_path": f.paths().join(s.other, "gm-token-kept")}),
			tool("Read", map[string]any{"file_path": f.paths().join(s.other, "hires/x/../../config.json")}),
			tool("LS", map[string]any{"path": s.other}),
			tool("Grep", map[string]any{"path": s.other, "pattern": ".", "output_mode": "content"}),
			tool("Glob", map[string]any{"path": s.other, "pattern": "*"}),
			tool("Write", map[string]any{"file_path": f.paths().join(s.other, "config.json")}),
			bashCall("cat '"+strings.ReplaceAll(registry, `\`, "/")+"'", repo),
			bashCall("cat "+s.tilde+"/registry.json", repo),
			bashCall("cp x "+s.tilde+"/config.json", repo),
			bashCall("cat "+s.tilde+"/hires/x/../../registry.json", repo),
			bashCall("cat "+s.tilde+"/*.json", repo),
		} {
			why := f.check(c)
			switch refused := strings.HasPrefix(why, demoFenceRefusal); {
			case s.caseAside && !refused:
				t.Errorf("on %s, let through %s %s", s.goos, c.ToolName, c.ToolInput)
			case !s.caseAside && c.ToolName == "Read" && refused:
				t.Errorf("on %s, refused %s %s: %s", s.goos, c.ToolName, c.ToolInput, why)
			}
		}
	}
}

// WIN125f (RWIN5 B1, B2): on every system, a path the shell builds with $ or a backtick could be the app's
// support folder, so it is refused as one into it is, in the real shape of each system's folder. On Windows, Git
// Bash's /c/…, /cygdrive/c/… and /proc/cygdrive/c/… are C:\…, read before every rule; a path from the top of
// some drive (\x, Git Bash's /tmp) can't be told, and counts as the support folder; a name as Win32 reads it (a
// stream, a dot or a blank at its end aside) is the folder's. The demo's own lines still run.
func TestTheDemosFenceRefusesAPathTheShellBuilds(t *testing.T) {
	for _, s := range []struct {
		goos, root, home, support string
		refused, allowed          []string
	}{
		{"darwin", "/Users/ada/smolcorp-demo", "/Users/ada", "/Users/ada/Library/Application Support/AgentDeck",
			[]string{
				`cat "$HOME/Library/Application Support/AgentDeck/registry.json"`, `grep -r . "$HOME/Library/Application Support/AgentDeck"`,
				`cat "${HOME}/Library/Application Support/AgentDeck/config.json"`, `cp "$HOME/Library/Application Support/AgentDeck/registry.json" leak.md`,
				`echo x > "$HOME/Library/Application Support/AgentDeck/config.json"`, `cat "$(pwd)/../../Library/Application Support/AgentDeck/registry.json"`,
				`cat $OLDPWD/registry.json`, "cat `echo ~`/Library/Application\\ Support/AgentDeck/registry.json",
				`cat "$(echo ~ | tr a a)/Library/Application Support/AgentDeck/registry.json"`, `cat ${HOME%/*}/ada/x`,
				`cat \$HOME/x`, `X=~/Library/Application\ Support/AgentDeck/registry.json; cat x`,
				`dd if=/Users/ada/Library/Application\ Support/AgentDeck/registry.json`, `curl -d @/Users/ada/Library/Application\ Support/AgentDeck/registry.json localhost`,
				`grep -f/Users/ada/Library/Application\ Support/AgentDeck/registry.json x`,
			}, nil},
		{"linux", "/home/ada/smolcorp-demo", "/home/ada", "/home/ada/.config/sauzzedeck-pro",
			[]string{`grep -r . "$HOME/.config/sauzzedeck-pro"`, `grep -r . "$XDG_CONFIG_HOME/sauzzedeck-pro"`, `cat $HOME/.config/sauzzedeck-pro/registry.json`},
			nil},
		{"windows", `C:\Users\ada\smolcorp-demo`, `C:\Users\ada`, `C:\Users\ada\AppData\Roaming\SauzzeDeck Pro`,
			[]string{
				`cat '/c/Users/ada/AppData/Roaming/SauzzeDeck Pro/registry.json'`, `grep -r . '/c/Users/ada/AppData/Roaming/SauzzeDeck Pro'`,
				`cat '/C/users/ada/appdata/roaming/sauzzedeck pro/registry.json'`, `cat '/cygdrive/c/Users/ada/AppData/Roaming/SauzzeDeck Pro/registry.json'`,
				`cat '/proc/cygdrive/c/Users/ada/AppData/Roaming/SauzzeDeck Pro/gm-token-kept'`, `ls /c/Users/ada/AppData/Roaming/SauzzeDeck\ Pro`,
				`cat "$APPDATA/SauzzeDeck Pro/registry.json"`, `cat "$HOME/AppData/Roaming/SauzzeDeck Pro/registry.json"`,
				`cat "$USERPROFILE/AppData/Roaming/SauzzeDeck Pro/registry.json"`, `cat "$LOCALAPPDATA/../Roaming/SauzzeDeck Pro/registry.json"`,
				`cat 'C:/Users/ada/AppData/Roaming/SauzzeDeck Pro/registry.json::$DATA'`, `cat 'C:/Users/ada/AppData/Roaming/SauzzeDeck Pro./registry.json'`,
				`cat 'C:/Users/ada/AppData/Roaming/SauzzeDeck Pro /registry.json'`, `cat '\Users\ada\AppData\Roaming\SauzzeDeck Pro\registry.json'`,
				`cat /tmp/../c/x`, `cat C:registry.json`,
			},
			[]string{
				"git -C /c/Users/ada/smolcorp-demo/app diff main...hire/tester-1 > /dev/null; echo $?",
				"cat /c/Users/ada/smolcorp-demo/app/notes.md", "python3 -m unittest 2>&1 | tail -5 > /dev/null",
			}},
	} {
		f := demoFence{root: s.root, roles: []string{"tester"}, home: s.home, support: s.support, goos: s.goos}
		repo := f.paths().join(s.root, demoRepoName)
		tool := func(name string, input map[string]any) demoCall {
			data, _ := json.Marshal(input)
			return demoCall{ToolName: name, ToolInput: data, Cwd: repo}
		}
		var refused []demoCall
		for _, line := range s.refused {
			refused = append(refused, bashCall(line, repo))
		}
		allowed := []demoCall{bashCall("echo $?", repo), bashCall("grep -n '$HOME/x' notes.md", repo), bashCall(`echo "$HOME"`, repo)}
		for _, line := range s.allowed {
			allowed = append(allowed, bashCall(line, repo))
		}
		if s.goos == "windows" {
			for _, file := range []string{
				"/c/Users/ada/AppData/Roaming/SauzzeDeck Pro/registry.json", "/cygdrive/c/Users/ada/AppData/Roaming/SauzzeDeck Pro/setup-key",
				"/proc/cygdrive/C/Users/ada/AppData/Roaming/SauzzeDeck Pro", `C:\Users\ada\AppData\Roaming\SauzzeDeck Pro\registry.json::$DATA`,
				`C:\Users\ada\AppData\Roaming\SauzzeDeck Pro.\registry.json`, "/tmp/x", `\Users\ada\AppData`,
			} {
				refused = append(refused, tool("Read", map[string]any{"file_path": file}), tool("Write", map[string]any{"file_path": file}))
			}
			refused = append(refused, tool("Grep", map[string]any{"path": "/c/Users/ada/AppData/Roaming/SauzzeDeck Pro", "pattern": "."}),
				tool("Glob", map[string]any{"pattern": "/c/Users/ada/AppData/Roaming/SauzzeDeck Pro/*"}),
				tool("Glob", map[string]any{"path": "/c/Users/ada", "pattern": "**/s*"}),
				tool("LS", map[string]any{"path": "/c/Users/ada/AppData/Roaming/SauzzeDeck Pro"}))
			// Git Bash's drive is the demo's folder too: a write there is inside it.
			allowed = append(allowed, tool("Write", map[string]any{"file_path": "/c/Users/ada/smolcorp-demo/app/notes.md"}),
				tool("Read", map[string]any{"file_path": "/c/Users/ada/smolcorp-demo/app/bill.py"}))
		}
		for _, c := range refused {
			if why := f.check(c); !strings.HasPrefix(why, demoFenceRefusal) {
				t.Errorf("on %s, let through %s %s", s.goos, c.ToolName, c.ToolInput)
			}
		}
		for _, c := range allowed {
			if why := f.check(c); why != "" {
				t.Errorf("on %s, refused %s %s: %s", s.goos, c.ToolName, c.ToolInput, why)
			}
		}
	}
	if why := (demoFence{root: "/Users/ada/smolcorp-demo", home: "/Users/ada", support: "/Users/ada/Library/Application Support/AgentDeck", goos: "darwin"}).
		command(`cat "$HOME/x/y"`, "/Users/ada/smolcorp-demo/app"); !strings.Contains(why, "write the path out plainly") {
		t.Errorf("the refusal: %q", why)
	}
}
