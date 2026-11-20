package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/guard"
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
			// RT126FS2b N4: the owner's grant record and the paired phones' seal's key, from above the folder.
			tool("Glob", map[string]any{"path": s.home, "pattern": "**/grants.json?"}),
			tool("Glob", map[string]any{"path": s.home, "pattern": "**/phones-s*"}),
			tool("Grep", map[string]any{"path": s.home, "pattern": "x", "glob": "phones-se?l"}),
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
				// RWIN6 B1: a redirection glued to its target, in the real Mac shape.
				`cat </Users/ada/Library/Application\ Support/AgentDeck/registry.json`, `cat 0<'/Users/ada/Library/Application Support/AgentDeck/registry.json'`,
				`cat<'/Users/ada/Library/Application Support/AgentDeck/registry.json'`, `cat <>'/Users/ada/Library/Application Support/AgentDeck/registry.json'`,
				`echo '{}' >'/Users/ada/Library/Application Support/AgentDeck/config.json'`, `echo '{}' >>'/Users/ada/Library/Application Support/AgentDeck/config.json'`,
				`ls 1>'/Users/ada/Library/Application Support/AgentDeck/config.json'`, `ls 2>'/Users/ada/Library/Application Support/AgentDeck/config.json'`,
				`ls &>'/Users/ada/Library/Application Support/AgentDeck/config.json'`, `echo x>'/Users/ada/Library/Application Support/AgentDeck/config.json'`,
				`cat x >~/Library/Application\ Support/AgentDeck/config.json`, `cat <../../Library/Application\ Support/AgentDeck/registry.json`,
				// B2 and B4, in the Mac shape: the guard the fence asks first refuses them.
				"\u00dfh host", "\ufb02y deploy", "\ufb01rebase deploy", "=git push origin main", "=git -C /elsewhere status",
				"=git worktree add /Users/ada/elsewhere", "=ssh host",
				// RWIN7 B1: a bracket Go's matcher read otherwise ([!x], POSIX's classes): the demo's own fence, the setup
				// key, and the app's folder by a pattern (the fence's own patternInSupport).
				"rm .claude/s[!x]ttings.json", "echo '{}' > .claude/s[!x]ttings.json", "rm -rf .c[!x]aude", "rm -rf .c[[:alpha:]]aude",
				`cat ~/Library/Application\ Support/AgentDeck/setup-k[!x]y`, "cat setup-k[[:alpha:]]y",
				`cat ~/Library/Application\ Support/[!x]gentDeck/registry.json`, `cat ~/Library/Application\ Support/[[:alpha:]]gentDeck/registry.json`,
				`cat /Users/ada/Library/[!x]pplication\ Support/AgentDeck/registry.json`,
				// B2: zsh's precommand modifiers, repeat, emulate -c and short forms.
				"noglob git push origin main", "nocorrect git push origin main", "repeat 1 git push origin main",
				"if [[ -n x ]] git push origin main", "noglob ssh host", "nocorrect git -C /Users/ada/elsewhere status",
				"true; - git push origin main", "emulate zsh -c 'git push origin main'", "noglob git worktree add /Users/ada/elsewhere",
				"noglob smolcorp gm hire --role tester --folder /Users/ada/elsewhere", "if noglob git -C /Users/ada/elsewhere status; then :; fi",
				// B3: zsh's ${(e)…} and bash's @P.
				"echo ${(e):-'$(git push)'}", "echo ${(e):-'$(cat setup-key)'}", `x='$(git push)'; echo "${x@P}"`,
			}, []string{"ls 2>/dev/null", "echo x >notes.md", "git diff HEAD~1 -- bill.py", "cat <notes.md",
				// RWIN7: a - or repeat as an argument, a test with its ;, and a bracket that fills in nothing kept.
				"echo x | cat -", "grep -n repeat notes.md", "if [[ -f notes.md ]]; then cat notes.md; fi", "ls [!.]*", "git log main^"}},
		{"linux", "/home/ada/smolcorp-demo", "/home/ada", "/home/ada/.config/sauzzedeck-pro",
			[]string{`grep -r . "$HOME/.config/sauzzedeck-pro"`, `grep -r . "$XDG_CONFIG_HOME/sauzzedeck-pro"`, `cat $HOME/.config/sauzzedeck-pro/registry.json`,
				`cat <~/.config/sauzzedeck-pro/registry.json`, `echo x >/home/ada/.config/sauzzedeck-pro/config.json`},
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
				// RWIN6 B1 in Git Bash's shape.
				`cat </c/Users/ada/AppData/Roaming/SauzzeDeck\ Pro/registry.json`, `echo x >'/c/Users/ada/AppData/Roaming/SauzzeDeck Pro/config.json'`,
				`ls 2>'C:/Users/ada/AppData/Roaming/SauzzeDeck Pro/config.json'`,
			},
			[]string{
				"git -C /c/Users/ada/smolcorp-demo/app diff main...hire/tester-1 > /dev/null; echo $?",
				"cat /c/Users/ada/smolcorp-demo/app/notes.md", "python3 -m unittest 2>&1 | tail -5 > /dev/null", "ls 2>/dev/null",
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
		if s.goos == "darwin" {
			// RWIN7 B1: a Glob from home whose last part could be the setup key, through a bracket.
			refused = append(refused, tool("Glob", map[string]any{"path": "/Users/ada", "pattern": "**/[!x]etup-key"}),
				tool("Glob", map[string]any{"path": "/Users/ada", "pattern": "**/[[:alpha:]]m-token"}),
				tool("Grep", map[string]any{"path": "/Users/ada/Library", "pattern": "x", "glob": "[!x]etup-key"}),
				tool("Read", map[string]any{"file_path": "/Users/ada/Library/Application Support/[!x]gentDeck/registry.json"}))
			allowed = append(allowed, tool("Glob", map[string]any{"pattern": "[!.]*.py"}))
		}
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

// WIN125j (RWIN8): the demo's fence in its Mac reading refuses every row of RWIN8's four tables, with the guard it
// asks first and its own reading of a pattern (guard.PatternCouldFill): a bracket closed anywhere, zsh's x^y, a
// pattern earlier in the path, and text the shell runs without eval. Everyday lines still run.
func TestTheDemosFenceReadsPatternsAsTheGuardDoes(t *testing.T) {
	f := demoFence{root: "/Users/ada/smolcorp-demo", roles: []string{"tester"}, home: "/Users/ada",
		support: "/Users/ada/Library/Application Support/AgentDeck", goos: "darwin"}
	repo := f.paths().join(f.root, demoRepoName)
	const dir = `~/Library/Application\ Support/AgentDeck/`
	refused := []string{
		// B1: a [ inside a bracket, and zsh's [!].
		"cat " + dir + "setup-k[!x-[:x:]y", "cat " + dir + "setup-k[!x-[:alpha:]y", "cat <" + dir + "setup-k[!x-[:x:]y",
		"echo '{}' > " + dir + "hire-gu[!x-[:x:]rd.json", "rm " + dir + "hire-gu[!x-[:x:]rd.json", "cat " + dir + "gm-t[!x-[:x:]ken",
		"rm .claude/s[!x-[:x:]ttings.local.json", "echo '{}' > .claude/s[!x-[:x:]ttings.json", "rm -rf .c[!x-[:x:]aude",
		"rm .claude/s[!x-[:x:]ttings.json", "cat " + dir + "setup-k[ee-[:x:]y", "rm -rf .c[ll-[:x:]aude",
		"cat " + dir + "setup-k[!]y", "rm -rf .c[!]aude", "cat " + dir + "setup-k[![:x:]y", "cat [[:alpha:]x]etup-ke[y]",
		// B2: zsh's x^y.
		"rm -rf .c^1", "rm -rf .c^", "rm -rf .cl^{tree}", "mv .c^1 /tmp/x", "setopt extendedglob; rm -rf .c^1",
		"setopt extendedglob; mv .c^1 x", "cat s^1", "cat se^1", "cat hire-g^1", "cat gm^1",
		// B3: a pattern earlier in the path.
		"rm .c?aude/settings.json", "echo '{}' > .c?aude/settings.json", "rm .c[!x]aude/settings.json",
		"rm .c[[:alpha:]]aude/settings.json", "rm .cl*/settings.json", "rm .c?aude/settings.local.json",
		// B4: text run as a command without eval.
		"PS4='$(git push origin main)'; set -x; :", "PS4='$(ssh host)'; set -x; :", "export PS4='`git push`'; set -x; :",
		"setopt promptsubst xtrace; PS4='$(git push)'; :", "setopt promptsubst; print -P '$(git push)'",
		"zstyle -e ':a' b 'git push'; zstyle -s ':a' b x", "zmodload zsh/zpty; zpty w 'git push'", "functions[f]='git push'; f",
		"aliases[P]='git push'; eval P", "alias -g P=push; eval 'git P'", "autoload -Uz zargs; zargs -- origin -- git push",
		"zargs -- origin -- git push", "autoload -U zmv; zmv '(*)' '$(git push)'", "eval -- 'git push'",
		`zmodload zsh/mapfile; echo "${mapfile[/Users/ada/Library/Application Support/AgentDeck/setup-key]}"`, "echo ${mapfile[setup-key]}",
		// Found on the way (WIN125j): dot globbing, hash and bash's arithmetic.
		"setopt globdots; rm -rf ?claude", "shopt -s dotglob; rm -rf *", "hash g=/usr/bin/git; g push origin main",
		"x='a[$(git push)]'; (( x ))",
		// A run may take a /: the app's folder by a pattern in an earlier part.
		"cat ~/Lib*/registry.json", "cat ~/Library/App*/registry.json",
	}
	allowed := []string{
		"[ -f notes.md ] && cat notes.md", "[[ -n $x ]] && echo \"$x\"", "grep '[0-9]' notes.md", "ls *.[ch]",
		"git log HEAD^1", "git diff main^..HEAD", "rm -rf build/*", "git log main^", "git show v1.2.0^{commit}",
		"ls [!.]*", "cat docs/pr\u00e9sentation.md", "ls \u00c9quipe/", "git diff HEAD~1 -- bill.py",
	}
	for _, line := range refused {
		if why := f.check(bashCall(line, repo)); !strings.HasPrefix(why, demoFenceRefusal) {
			t.Errorf("let through %s", line)
		}
	}
	for _, line := range allowed {
		if why := f.check(bashCall(line, repo)); why != "" {
			t.Errorf("refused %s: %s", line, why)
		}
	}
	tool := func(name string, input map[string]any) demoCall {
		data, _ := json.Marshal(input)
		return demoCall{ToolName: name, ToolInput: data, Cwd: repo}
	}
	for _, c := range []demoCall{
		tool("Glob", map[string]any{"path": "/Users/ada", "pattern": "**/setup-k[!x-[:x:]y"}),
		tool("Glob", map[string]any{"path": "/Users/ada", "pattern": "**/[[:alpha:]x]etup-ke[y]"}),
		tool("Read", map[string]any{"file_path": "/Users/ada/Lib*/registry.json"}),
		tool("Glob", map[string]any{"path": "/Users/ada", "pattern": "Lib*/registry.json"}),
	} {
		if why := f.check(c); !strings.HasPrefix(why, demoFenceRefusal) {
			t.Errorf("let through %s %s", c.ToolName, c.ToolInput)
		}
	}
	if why := f.check(tool("Glob", map[string]any{"pattern": "[!.]*.py"})); why != "" {
		t.Errorf("a Glob in the project: %s", why)
	}
	// The shared reading itself: a match is never narrower than a shell's.
	for _, c := range []struct {
		pattern, name string
		below, want   bool
	}{
		{"[!x]gentDeck", "AgentDeck", false, true}, {"A[!x-[:x:]entDeck", "AgentDeck", false, true},
		{"{A,B}gentDeck", "AgentDeck", false, true}, {"Lib*", "Library", true, true}, {"Libr", "Library", true, false},
		{"Docs*", "Library", true, false}, {"[!.]*.py", "AgentDeck", false, false},
	} {
		if got := guard.PatternCouldFill(c.pattern, c.name, c.below); got != c.want {
			t.Errorf("PatternCouldFill(%q, %q, %v): %v", c.pattern, c.name, c.below, got)
		}
	}
}

// WIN125k (RWIN9): the demo's fence in its Mac reading refuses every row RWIN9 found it let through: its own fence
// reached by a path cleaned or a move (B1), a value read as a path (B3), unsetopt +4 (B4), bash's runners (B5) and
// find's and git's own patterns (B6); a Bash call whose cwd is a .claude or .git folder, or the app's own folder, as
// in two calls (cd .claude/agents, then rm ../settings.json); and RWIN9's 240 fuzz hits. Everyday lines still run.
func TestTheDemosFenceRefusesRWIN9sRows(t *testing.T) {
	f := demoFence{root: "/Users/ada/smolcorp-demo", roles: []string{"tester"}, home: "/Users/ada",
		support: "/Users/ada/Library/Application Support/AgentDeck", goos: "darwin"}
	repo := f.paths().join(f.root, demoRepoName)
	refused := []string{
		"rm .claude/./settings.json", "echo '{}' > .claude/./settings.json", "rm .claude//settings.json",
		"rm .claude/agents/../settings.json", "echo '{}' > .claude/commands/../settings.json",
		"cd .claude/. && rm settings.json", "cd .claude/agents && rm ../settings.json", "pushd .claude/. && rm settings.json",
		"cd ./.claude/./ && rm settings.json", "git -C .claude/. rm -f settings.json", "HOME=.claude; rm ~/settings.json",
		"OLDPWD=.claude/agents; cd - && rm ../settings.json", "CDPATH=.claude; cd agents && rm ../settings.json",
		"cd .claude/agents", "x=.c?aude/settings.json; rm $x", "x=.cl*/settings.json; echo '{}' > $x",
		"x=.c?aude/settings.json; rm ${~x}", "setopt globsubst; x=.c?aude/settings.json; rm $x",
		"unsetopt +4; rm -rf ?claude", "unsetopt +4 && rm -rf ?claude", "unsetopt +4; rm -rf *",
		"compgen -C 'git push' x", "compgen -C 'ssh host' x", "complete -C 'git push' x", "mapfile -t -C 'git push #' -c 1 a <<< x",
		"readarray -C 'git push #' -c 1 a <<< x", "jobs -x git push", "jobs -x ssh host", "jobs -x vercel deploy",
		"builtin jobs -x git push", `echo "${ git push; }"`, `echo "${| git push; }"`, `echo "${ ssh host; }"`,
		"find . -name 'settings.loc*' -delete", "find . -path '*/.c*/s*' -delete", "find . -name settings.json -delete",
		"git rm -q '.c*/settings.json'", "git rm -q ':(glob).c*/settings.json'", "git rm -q '*settings.json'",
	}
	allowed := []string{
		"find . -name '*.py'", "find . -name __pycache__ -prune", "cd ..", "cd -", "git -C . status", "find . -name '*.pyc' -delete",
		`rm -f "$tmp"`, "ls [!.]*", "cat docs/présentation.md", "git diff HEAD~1 -- bill.py", "cat .claude/settings.json",
	}
	for _, line := range refused {
		if why := f.check(bashCall(line, repo)); !strings.HasPrefix(why, demoFenceRefusal) {
			t.Errorf("let through %s", line)
		}
	}
	for _, line := range allowed {
		if why := f.check(bashCall(line, repo)); why != "" {
			t.Errorf("refused %s: %s", line, why)
		}
	}
	// The second of two calls, from where the first one's cd left it.
	for _, c := range []demoCall{
		bashCall("rm ../settings.json", repo+"/.claude/agents"), bashCall("rm settings.json", repo+"/.claude"),
		bashCall("echo '{}' > settings.json", repo+"/.claude"), bashCall("ls", repo+"/.git/hooks"),
		bashCall("echo '{}' > h.json", "/Users/ada/Library/Application Support/AgentDeck"),
		bashCall("rm settings.json", "/Users/ada/elsewhere/.claude"),
	} {
		if why := f.check(c); !strings.HasPrefix(why, demoFenceRefusal) {
			t.Errorf("let through %s from %s", c.ToolInput, c.Cwd)
		}
	}
	if why := f.check(bashCall("ls", repo+"/src")); why != "" {
		t.Errorf("a call from the project's own folder: %s", why)
	}
	data, err := os.ReadFile("../internal/guard/testdata/rwin9-fuzz-hits.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, pattern := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if why := f.check(bashCall("rm "+pattern, repo)); !strings.HasPrefix(why, demoFenceRefusal) {
			t.Errorf("let through rm %s", pattern)
		}
	}
}

// WIN125l (RWIN10): the demo's fence in its Mac reading refuses the rows RWIN10 found it let through, as it asks the
// guard first: a move into .claude by a variable or zsh's dirs (B2), an assignment's value in its other forms (B3),
// find's and git's patterns with a $ run in them and a --opt= value (B4), and find's -regex classes (B6). B7's
// find lines run.
func TestTheDemosFenceRefusesRWIN10sRows(t *testing.T) {
	f := demoFence{root: "/Users/ada/smolcorp-demo", roles: []string{"tester"}, home: "/Users/ada",
		support: "/Users/ada/Library/Application Support/AgentDeck", goos: "darwin"}
	repo := f.paths().join(f.root, demoRepoName)
	refused := []string{
		"x=.claude/agents; cd $x && rm ../settings.json", "x=.claude/agents; pushd $x && rm ../settings.json",
		"read x <<< .claude/agents; cd $x && rm ../settings.json", "dirs .claude/agents; popd; rm ../settings.json",
		"dirs .claude/agents; cd +1; rm ../settings.json", "dirs .claude/agents; cd -1; rm ../settings.json",
		"a=('.c?aude/settings.json'); rm ${a[@]}", "a+=('.c?aude/settings.json'); rm ${a[@]}",
		"declare -a a=('.c?aude/settings.json'); rm ${a[@]}", "x+='.c?aude/settings.json'; rm $x",
		"read x <<< '.c?aude/settings.json'; rm $x", "IFS= read -r x <<< '.c?aude/settings.json'; rm $x",
		"mapfile -t a <<< '.c?aude/settings.json'; rm ${a[@]}", "printf -v x '.c?aude/settings.json'; rm $x",
		"for x in '.c?aude/settings.json'; do rm $x; done", "set -- '.c?aude/settings.json'; rm $1",
		`find . -name "sett${x}ings.json" -delete`, `find . -name "s${x}*" -delete`, `find . -name "$(printf s)ettings.json" -delete`,
		`find . -name "set${x}tings.json" -exec cp /dev/null {} \;`, `git rm "*sett${x}ings.json"`,
		"cp settings.json --target-directory=.clau${x}de",
		`find . -regex '.*/\.\S\S\S\S\S\S/\S\S\S\S\S\S\S\S\.\S\S\S\S' -delete`,
		`find . -regex '.*/\.\w\w\w\w\w\w/\w\w\w\w\w\w\w\w\.\w\w\w\w' -delete`,
		`find . -regex '.*/\S\S\S\S\S\S\S\S\.\S\S\S\S\S\.\S\S\S\S' -delete`,
	}
	allowed := []string{
		"find . -name '*.py' -not -path './node_modules/*' -delete", "find . -type f -name '*.py' ! -path './.venv/*' -exec black {} +",
		"find . -path ./node_modules -prune -o -name '*.pyc' -delete",
	}
	for _, line := range refused {
		if why := f.check(bashCall(line, repo)); !strings.HasPrefix(why, demoFenceRefusal) {
			t.Errorf("let through %s", line)
		}
	}
	for _, line := range allowed {
		if why := f.check(bashCall(line, repo)); why != "" {
			t.Errorf("refused %s: %s", line, why)
		}
	}
}

// WIN125m (RWIN11 PART 1): a brace sequence in a for list, an array, set -- or an assignment runs in the demo, as
// it did at WIN125h; a brace form that could reach the fence is still refused.
func TestTheDemosFenceRunsBraceSequences(t *testing.T) {
	f := demoFence{root: "/Users/ada/smolcorp-demo", roles: []string{"tester"}, home: "/Users/ada",
		support: "/Users/ada/Library/Application Support/AgentDeck", goos: "darwin"}
	repo := f.paths().join(f.root, demoRepoName)
	allowed := []string{
		"for i in {1..5}; do echo $i; done", "for i in {01..12}; do echo $i; done", "for i in {a..e}; do echo $i; done",
		"for i in {0..100..10}; do echo $i; done", "for i in x{1..3}y; do echo $i; done", "x=({1..5})", "a=({1..3})",
		"set -- {1..3}", "x={1..3}", "for i in {1..100}; do echo $i; done",
	}
	refused := []string{
		"x=.{a..z}laude/settings.json; rm $x", "for d in .{a..z}laude; do rm $d/settings.json; done",
		"set -- .c{a..z}aude/settings.json; rm $1", "for i in {a..b..2}claude; do rm -r $i; done",
		"rm .{a..z}laude/{a..z}{a..z}ttings.json",
	}
	for _, line := range allowed {
		if why := f.check(bashCall(line, repo)); why != "" {
			t.Errorf("refused %s: %s", line, why)
		}
	}
	for _, line := range refused {
		if why := f.check(bashCall(line, repo)); !strings.HasPrefix(why, demoFenceRefusal) {
			t.Errorf("let through %s", line)
		}
	}
}
