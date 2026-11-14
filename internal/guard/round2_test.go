package guard

import (
	"strings"
	"testing"
	"time"
)

// T307b fix round 1: what the first review of the follow-up found
// (T307b-verdict-1). Each A example must be refused, each B example must run.

// A1: a << that bash reads as arithmetic starts no here document.
func TestArithmeticShiftIsNoHereDocument(t *testing.T) {
	mustRefuse(t, "arithmetic <<", []string{
		"(( x = 1 << 2 ))\ngit pu\\sh",
		"(( x = 1<<2 ))\ngit pu\\sh",
		"if (( 1 << 2 )); then\ngit pu\\sh\nfi",
		"for ((i=0; i<<2; i++)); do :; done\ngit pu\\sh",
		"echo $[ 1 << 2 ]\ngit pu\\sh",
		"echo ${s:1<<1:2}\ngit pu\\sh",
		"a[1<<2]=3\ngit pu\\sh",
		"echo ${a[1<<2]}\ngit pu\\sh",
		"echo $(( 1 << 2\n))\ngit pu\\sh",
		"echo \"$(( 1 << 2\n))\"\ngit pu\\sh",
		"echo $( (( 1 << 2 )) )\ngit pu\\sh",
		"(( x = 1 << 2 ))\n$'\\x67it' push",
		"(( x = 1 << 2 ))\ng\\it push",
		"(( x = 1 << 2 ))\ngit $'\\x70ush'",
		"echo $(( $(git push) << 1 ))",
		"echo ${x:-$(git push)}",
		"a[$(git push)]=1",
	})
	// the delimiter $'EOF' and $"EOF" is EOF
	mustRefuse(t, "$'EOF'", []string{
		"cat <<$'EOF'\nx\nEOF\ngit pu\\sh",
		"cat <<$\"EOF\"\nx\nEOF\ngit pu\\sh",
		"cat <<$'EOF'\nit's\nEOF\ngit push",
	})
	mustRun(t, "arithmetic <<", []string{
		"(( x = 1 << 2 ))", "(( x = 1 << 2 ))\nls", "echo $((1<<2))", "echo $[ 1 << 2 ]", "a[1<<2]=3", "echo ${a[1<<2]}\nls",
		"for ((i=0; i<<2; i++)); do :; done", "cat <<$'EOF'\nit's\nEOF\nls", "echo $(( 1 << 2\n))\nls",
	})
	// a (( with no )) is two subshells, and a << in it is a real here document
	mustRefuse(t, "(( without ))", []string{
		"((cat <<EOF\ndon't\nEOF\ngit pu\\sh\n) )",
		"((cat <<EOF\ndon't\nEOF\n) ); git push",
	})
}

// A2: no spelling names the GM's token or the guard file.
func TestNoSpellingNamesTheTokenOrTheGuardFile(t *testing.T) {
	const dir = `~/Library/Application\ Support/AgentDeck/`
	mustRefuse(t, "token", []string{
		"cat " + dir + `gm-tok""en`, "cat " + dir + `gm-to\ken`, "cat " + dir + `gm-"token"`, "cat " + dir + `gm-$'t'oken`,
		"cat " + dir + "gm-tok{en,}", "cat " + dir + "{gm-,}token", "cat " + dir + "gm-t*", "cat " + dir + "gm-tok?n", "cat " + dir + "gm-[t]oken",
		"cat " + dir + "GM-TOKEN", "cat " + dir + "gm-Token", "cat " + dir + "gm*", "cat " + dir + "*",
		"printenv SMOLCORP_GM_TOK\"\"EN", "echo $SMOLCORP_GM_TOK''EN", "printenv smolcorp_gm_token", "cat gm-t*", "cat gm-tok?n",
		"sh -c 'cat gm-tok\"\"en'", "echo $(cat gm-to\\ken)", "cat *token", "cat gm-*oken",
		"cat <<EOF\n$(cat gm-tok\"\"en)\nEOF", "echo x > gm-tok\"\"en", "git commit -m x -- gm-t*",
	})
	// Where a computer with no secret store keeps the GM's token (desktop/gmtokenstore.go): no pattern names it either.
	mustRefuse(t, "kept token by pattern", []string{
		"cat ~/.config/smolcorp/*-kept", "cat g?-token-kept", "cat *oken-kept", "cp *-kept /tmp/x",
	})
	mustRefuse(t, "guard file by pattern", []string{
		"cp /tmp/off.json " + dir + "hire-gu*", "rm " + dir + "hire-g*", "cp x hire-guar?.json", "echo '{}' > " + dir + "hire-g*",
		"echo '{}' > hire-gu\"\"ard.json", "echo '{}' > hire-guard.js{on,}", "echo '{}' > HIRE-g*", "tee hire-guard.js*",
	})
	mustRun(t, "ordinary patterns", []string{
		"ls *.json", "cat /tmp/*.json", "ls *", "ls /tmp/*", "ls g*", "ls gm*", "ls docs/gm*.md", "git add internal/*",
		"cat package*.json", "ls ~/Projects/foo*", "rm -f /tmp/a*", "cat gm-notes.md", "ls token", "echo hire",
		// RWIN7 B1: brackets that fill in nothing the guard keeps.
		"ls [!.]*", "ls [[:upper:]]*", "ls x[]y", "cat setup-k[!e]y.md", "ls src/[a-z]*.go", "cat notes[!x].md",
	})
	// RWIN7 B1: [!x] and POSIX's classes, which Go's matcher read otherwise, fill in the guard file and the token
	// file. Each bracket is read as any one letter, whatever it holds; one that starts with ! or holds a class is
	// refused as not written plainly where it fills in such a name, even for a command that only reads.
	odd := []string{
		"echo '{}' > " + dir + "hire-gu[!x]rd.json", "echo '{}' > " + dir + "hire-gu[[:alpha:]]rd.json",
		"cp /dev/null " + dir + "hire-gu[!x]rd.json", "rm " + dir + "hire-gu[!x]rd.json", "cat " + dir + "gm-t[!x]ken",
		"cat " + dir + "gm-t[[:alpha:]]ken", "cat hire-gu[!x]rd.json", "cat ~/.config/smolcorp/gm-token-k[!x]pt",
		// For comparison, refused before too.
		"echo '{}' > " + dir + "hire-gu?rd.json", "rm " + dir + "hire-gu?rd.json", "cat " + dir + "gm-to?en",
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range odd {
			if Check(line, groups) == nil {
				t.Errorf("let through: %s", line)
			}
		}
	}
	if r := Check("cat hire-gu[!x]rd.json", DefaultGroups()); r == nil || r.Rule != notPlainRule || !strings.Contains(r.Error(), "[!x]") {
		t.Errorf("a reader with an odd bracket: %v", r)
	}
}

// A3: a command that names the guard file is refused unless it only reads.
func TestOnlyReadersMayNameTheGuardFile(t *testing.T) {
	const f = "~/Library/Application\\ Support/AgentDeck/hire-guard.json"
	mustRefuse(t, "guard file", []string{
		"cp /tmp/off.json " + f, "sed -Ei 's/true/false/' hire-guard.json", "sed -ni 's/a/b/p' hire-guard.json",
		"sed -i.bak s/a/b/ hire-guard.json", "echo '{}' | sed -n 'w hire-guard.json'", "sed -n p hire-guard.json",
		`php -r 'file_put_contents("hire-guard.json","{}");'`, `nawk 'BEGIN{print "{}" > "hire-guard.json"}'`,
		`mawk 'BEGIN{print "{}" > "hire-guard.json"}'`, `deno eval 'Deno.writeTextFileSync("hire-guard.json","{}")'`,
		"bsdtar -xf x.tar hire-guard.json", "cpio -i hire-guard.json < x", "xxd -r x hire-guard.json", "ex -sc wq hire-guard.json",
		"touch hire-guard.json", "unlink hire-guard.json", `pwsh -c '[IO.File]::WriteAllText("hire-guard.json","{}")'`,
		"git checkout -- hire-guard.json", "git mv hire-guard.json x", "git restore hire-guard.json", "git rm hire-guard.json",
		"env cat hire-guard.json", "sudo tee hire-guard.json", "sh -c 'sed -i s/a/b/ hire-guard.json'", "echo x > " + f,
		"echo x >| hire-guard.json", "smolcorp gm hire x hire-guard.json",
	})
	mustRun(t, "guard file", []string{
		"cat docs/hire-guard.md", "grep -n hire-guard README.md", "git commit -m 'docs: what hire-guard does'", "cat " + f, "head -3 " + f,
		"tail -f hire-guard.json", "less hire-guard.json", "jq . hire-guard.json", "ls -l " + f, "stat hire-guard.json", "wc -l hire-guard.json",
		"diff hire-guard.json hire-guard.json.bak", "rg hire-guard", "echo hire-guard", "printf '%s' hire-guard", "git log -- hire-guard.json",
		"git diff hire-guard.json", "git show HEAD:hire-guard.json", "git status hire-guard.json", "git add hire-guard.json", "git blame hire-guard.json",
		"git -C /tmp log hire-guard.json", "git grep hire-guard", "file hire-guard.json", "smolcorp guard hire-guard.json", "Get-Content hire-guard.json",
		"cmp a hire-guard.json",
	})
}

// A4: a wrapper's option with a value, in a cluster.
func TestWrapperOptionClusters(t *testing.T) {
	mustRefuse(t, "clusters", []string{
		"sudo -nu root git push", "sudo -Hu root git push", "sudo -ng wheel git push", "sudo -nh h git push", "sudo -np p git push",
		"sudo -nC 3 git push", "sudo -nD /tmp git push", "sudo -nR /r git push", "sudo -nT 5 git push", "sudo -nU u git push",
		"sudo -nr r git push", "sudo -nt t git push", "doas -nu root git push",
		"xargs -tn 1 git push", "xargs -rn 1 git push", "xargs -tI X git push", "xargs -rP 2 git push", "xargs -0n 1 git push",
		"nice -n 5 git push", "watch -dn 1 git push",
		"timeout -vs KILL 5 git push", "flock -nw 5 /tmp/l git push",
	})
	mustRun(t, "clusters", []string{
		"sudo -nu root ls", "sudo -Hu root ls", "xargs -tn 1 echo", "xargs -rn 1 ls", "xargs -n1 echo", "xargs -rn5 ls", "sudo -E ls",
	})
	// the first value letter takes the rest of the cluster: -un is -u with the value n
	mustRefuse(t, "attached value", []string{"sudo -un git push"})
}

// A5: the subcommand is found among all the words of these tools.
func TestSubcommandAnywhereAmongTheArguments(t *testing.T) {
	mustRefuse(t, "global options", []string{
		"npm --omit dev publish", "npm --before 2020-01-01 publish", "npm -C . publish", "npm --omit dev pub --dry-run", "npm -C . pub",
		"yarn --registry https://r publish", "cargo --color always publish", "cargo -Z unstable-options publish", "cargo --config x=1 publish",
		"kubectl -v 6 apply -f x", "pulumi --color always up", "supabase --dns-resolver native db push", "pnpm --color publish",
		"bun --cwd x publish", "terraform -chdir=x apply", "docker -l debug push x", "gh --repo a/b pr create", "eas --non-interactive build",
		"gh -R a/b pr merge 1", "podman --root /r push x", "kubectl --some-new-option v delete pod x",
		"npm --unknown-option value pu",
	})
	mustRun(t, "global options", []string{
		"npm --omit dev install", "cargo --color always build", "kubectl -v 6 get pods", "pulumi --color always preview", "docker -l debug ps",
		"gh --repo a/b pr view 1", "yarn --registry https://r install", "supabase --dns-resolver native db diff", "terraform -chdir=x plan",
		"eas --non-interactive whoami", "npm run build", "npm test", "cargo test", "gh -R a/b issue list",
	})
}

// A6: more outward-facing gh commands, credentials and registry changes.
func TestMoreGhNpmAndCredentials(t *testing.T) {
	mustRefuse(t, "gh", []string{
		"gh pr update-branch 3", "gh pr revert 3", "gh pr lock 3", "gh pr unlock 3", "gh issue pin 3", "gh issue unpin 3", "gh issue develop 3",
		"gh issue lock 3", "gh issue unlock 3", "gh label create x", "gh label delete x", "gh label edit x", "gh label clone a/b",
		"gh repo unarchive a/b", "gh repo deploy-key add k", "gh repo autolink create x y", "gh ssh-key add k", "gh ssh-key delete 1",
		"gh gpg-key add k", "gh gpg-key delete 1", "gh codespace create", "gh codespace delete", "gh codespace ssh", "gh extension install a/b",
		"gh cache delete 1", "gh auth token", "gh auth refresh", "gh auth login", "gh auth logout", "gh auth setup-git", "gh auth status --show-token",
		"gh auth status -t",
	})
	mustRefuse(t, "npm", []string{
		"npm unpublish x@1", "npm deprecate x no", "npm dist-tag add x@1 latest", "npm access public", "npm owner add u x", "npm token create",
		"npm login", "npm adduser", "npm team create x", "npm org set x u",
	})
	mustRefuse(t, "credentials", []string{
		"git credential fill", "git credential approve", "git credential reject", "git credential-osxkeychain get", "git-credential-osxkeychain get",
		"/usr/libexec/git-core/git-credential-store get", "op read op://v/i/f", "op item get x", "op inject -i x", "op signin", "bw get password x", "pass show x",
	})
	mustRun(t, "gh", []string{
		"gh pr view 1", "gh pr list", "gh pr diff 1", "gh pr checks 1", "gh issue view 1", "gh issue list", "gh auth status", "gh repo view",
		"gh run list", "gh label list", "gh extension list", "gh cache list", "gh codespace list", "gh ssh-key list", "npm view x", "npm ls", "npm list", "npm whoami",
		"git config user.name", "op --version", "op run -- ls", "git remote -v",
	})
}

// B7: a here document only feeds a command; its text is not run.
func TestHereDocumentsThatOnlyFeedACommandRun(t *testing.T) {
	body := "Fix upload\n\nAWS credentials are now read from the environment.\nRender the chart again.\nSSH keys are not stored.\nSecurity headers were added.\nFly machines wait.\nVercel previews work.\nFastlane lanes run.\nHeroku is not used any more.\n"
	mustRun(t, "heredoc text", []string{
		"git commit -m \"$(cat <<'EOF'\n" + body + "EOF\n)\"",
		"cat > notes.md <<'EOF'\nRender the page.\nHeroku is not used any more.\nEOF",
		"git commit -F - <<'EOF'\nRender fix\nEOF",
		"mysql <<'EOF'\nRender\nEOF",
		"psql <<\"EOF\"\naws s3 ls\nEOF",
		"cat <<\\EOF\ngit push\nEOF",
		"cat <<EOF\nRender the page\nhello $HOME\nEOF",
		"cat <<EOF\ndon't `date`\nEOF",
		"tee x <<'EOF'\nvercel deploy\nEOF",
	})
	mustRefuse(t, "heredoc text", []string{
		"bash <<'EOF'\ngit push\nEOF", "sh <<EOF\nvercel deploy\nEOF", "zsh <<'EOF'\nfly deploy\nEOF", "eval <<'EOF'\ngit push\nEOF",
		"ssh host <<'EOF'\nls\nEOF", "xargs sh <<'EOF'\ngit push\nEOF", "sudo bash <<'EOF'\ngit push\nEOF", "somethingelse <<'EOF'\ngit push\nEOF",
		"<<'EOF' somethingelse\ngit push\nEOF", "cat <<EOF\n$(git push)\nEOF", "cat <<EOF\n`git push`\nEOF", "cat <<EOF\nx $(echo a; git push) y\nEOF",
		"git commit -F - <<EOF\n$(vercel deploy)\nEOF", "cat <<'EOF' | bash\ngit push\nEOF", "bash -s <<'EOF'\ngit push\nEOF",
		"cat <<EOF\nit's $(git push)\nEOF",
	})
	// the quoted delimiter is not expanded, the escaped one neither
	mustRun(t, "heredoc text", []string{"cat <<'EOF'\n$(git push)\nEOF", "cat <<\\EOF\n$(git push)\nEOF"})
}

// B8: a project's script with the name of a tool.
func TestPackageManagerScriptsRun(t *testing.T) {
	mustRun(t, "scripts", []string{
		"pnpm render", "yarn render", "bun render", "pnpm fly", "yarn heroku", "pnpm railway", "pnpm fastlane", "yarn render --watch",
		"pnpm build", "pnpm test", "yarn dev", "bun run dev", "pnpm run render",
	})
	mustRefuse(t, "scripts", []string{
		"pnpm vercel deploy", "yarn vercel --prod", "bun vercel", "pnpm exec fly deploy", "pnpm dlx render", "yarn exec heroku deploy", "bun x fly deploy",
		"pnpm netlify deploy", "pnpm wrangler publish", "pnpm eas build", "pnpm publish",
	})
}

// A1, belt and braces: a ((, $[, ${ or [ before a << also runs the reading with no here documents.
func TestPessimistReadingOnlyWhenAUnitComesBeforeAHereDocument(t *testing.T) {
	for line, want := range map[string]bool{
		"cat <<EOF": false, "git commit -m \"$(cat <<'EOF'\nx\nEOF\n)\"": false, "echo $((1<<2))": true, "a[1]=2; cat <<EOF": true,
		"echo ${x}\ncat <<EOF": false, "echo ${x} <<EOF": true, "echo $'a' <<EOF": true, "[[ a ]] && cat <<EOF": true, "echo $(( 1 )) <<<x": false,
	} {
		if got := needsPessimist(line); got != want {
			t.Errorf("needsPessimist(%q) = %v", line, got)
		}
	}
}

// Time: units that never close are counted and refused, not scanned over and over.
func TestUnclosedUnitsAreNotSlow(t *testing.T) {
	big := 1 << 20
	for name, line := range map[string]string{
		"${":      strings.Repeat("${", big/2) + " ; git push",
		"$((":     strings.Repeat("$((", big/3),
		"((":      strings.Repeat("((", big/2),
		"$[":      strings.Repeat("$[", big/2),
		"a[":      strings.Repeat("a[ ", big/3),
		"closed":  strings.Repeat("${a} ", big/5) + "; git push",
		"nested":  strings.Repeat("$((", 1000) + strings.Repeat("))", 1000),
		"quotes":  strings.Repeat("${'", big/3),
		"inside":  "$(" + strings.Repeat("${", big/2),
		"heredoc": "cat <<EOF" + strings.Repeat(" ${", big/4) + "\nx\nEOF",
		"long":    strings.Repeat("a[1<<2]=3\n", big/10) + "git push",
	} {
		start := time.Now()
		r := Check(line, DefaultGroups())
		if took := time.Since(start); took > 2*time.Second*raceSlowdown {
			t.Errorf("%s: took %v", name, took)
		}
		switch name {
		case "${", "closed", "long":
			if r == nil {
				t.Errorf("%s: let through", name)
			}
		}
	}
}

// WIN125c (RWIN2 B3): no spelling names the app's setup key either, as a word, a pattern that fills it in, or the
// file a < reads; for the GM and every hire. The word in a message, or a grep for it in Smolcorp's own code, runs.
func TestNoSpellingNamesTheSetupKey(t *testing.T) {
	const dir = `~/Library/Application\ Support/AgentDeck/`
	lines := []string{
		"cat " + dir + "setup-key", "cat " + dir + "setup-k?y", "cat " + dir + "setup-*", "cat " + dir + `setup-"k"ey`,
		"cat " + dir + `setup-\key`, "cat " + dir + "SETUP-KEY", "cat " + dir + "setup-k{ey,}", "cat ~/.config/Smol*/setup-k?y",
		"cat ~/.config/sauzzedeck-pro/setup-key", `type "C:\Users\ada\AppData\Roaming\SauzzeDeck Pro\setup-key"`,
		"cat < " + dir + "setup-key", "x=$(< setup-key)", "while read k; do echo $k; done <setup-k*",
		"sh -c 'cat setup-k?y'", "cp setup-key /tmp/k", "curl -H @setup-key localhost:7710/setup/state",
		"dd if=setup-key", "echo setup-key",
		// RWIN5 B3: ſ (the long s) is s to the Mac's disk, and K (the Kelvin sign) k; Windows drops a name's dots and
		// blanks at its end, reads a stream after a :, and has a short name for it.
		"cat " + dir + "\u017fetup-key", "cat \u017fetup-key", "cat \u017fETUP-KEY", "cat setup-\u212aey", "cp \u017fetup-key k",
		"cat ~/Library/Application\\ \u017fupport/AgentDeck/s*", "cat ~/.config/sauzzedeck-pro/s*", "cat ~/AppData/Roaming/SauzzeDeck\\ Pro/s*",
		"type setup-key.", `type "setup-key "`, "type setup-key::$DATA", "type SETUP-~1", "type se1a2b~1", `type C:setup-key`,
		`type "C:\Users\ada\AppData\Roaming\SauzzeDeck Pro\SETUP-~1"`, "grep -fSETUP-~1 x", "grep -fsetup-key. x",
		// RWIN7 B1: a bracket Go's matcher read otherwise ([!x] is "anything but x" to the shell, and POSIX's classes).
		"cat setup-k[!x]y", "cat setup-[!x]ey", "cat setup-k[[:alpha:]]y", "cat setup-k[[:lower:]]y", "cat [[:alpha:]]etup-key",
		"cat setup-k[[=e=]]y", "cat setup-k[[.e.]]y", "cat " + dir + "setup-k[!x]y", "cat < setup-k[!x]y", "cat <setup-k[[:alpha:]]y",
		"grep . setup-k[!x]y", "cp setup-k[!x]y /tmp/k", "cat setup-k[!]x]y", `cat setup-k["!"x]y`,
		// RWIN7 B2 and B3: zsh's emulate -c, and ${(e)…} running its own text.
		"emulate zsh -c 'cat setup-key'", "echo ${(e):-'$(cat setup-key)'}", "echo ${(e):-'$(cat " + dir + "setup-key)'}",
		// RWIN7's comparisons, refused before too.
		"cat setup-k?y", "cat setup-k[e]y", "cat setup-k[a-z]y", "cat setup-k[^x]y", "repeat 1 cat setup-key",
		"emulate sh -c 'cat " + dir + "setup-key'",
	}
	mustRefuse(t, "setup key", lines)
	for _, line := range lines {
		if r := Check(line, ForGM(DefaultGroups())); r == nil {
			t.Errorf("the GM: %q let through", line)
		}
	}
	mustRun(t, "the word as text", []string{
		`git commit -m "the setup-key file is the owner's"`, "grep -n 'setup-key file' docs/setup.md", "cat setup-keys.md", "ls setup-*.md",
		"sort < in.txt", "cat docs/setup.md",
		// Words that only look like it: another file on every disk.
		"cat setup-keys.md", "cat setup-key.md",
	})
	// WIN125g: a ~ inside a word is a pattern to zsh (setup-key~1 is setup-key, but not 1), so these look-alikes
	// are refused with it, as the plain-writing rule refuses every such word (plain.go).
	mustRefuse(t, "a ~ inside a word", []string{"ls setup-~", "cat sexy~1", "cat setup-key~1"})
	// Fold is the disk's folding (WIN125h): ſ and s, K and k, any case, are one letter, and ß is ss, ﬁ is fi, as the
	// Mac's disk reads them (RWIN6 B2). An accented letter stays itself.
	for in, want := range map[string]string{"\u017fetup-KEY": "setup-key", "Setup-\u212aey": "setup-key", "Stra\u00dfe": "strasse", "\u03a3\u03c2": "\u03c3\u03c3",
		"\u1e9eH": "ssh", "\ufb02y": "fly", "\ufb01rebase": "firebase", "fa\ufb06lane": "fastlane", "\ufb00\ufb03\ufb04\ufb05": "ffffifflst", "a\u037eb": "a;b",
		"Pr\u00c9sentation": "pr\u00e9sentation"} {
		if got := Fold(in); got != want {
			t.Errorf("Fold(%q) = %q, want %q", in, got, want)
		}
	}
}

// WIN125e (RWIN4 B1): a search of other files for the word is refused, for the GM and every hire, as WIN125c
// refused it. WIN125d let grep, rg and git grep run with setup-key as their pattern, and RWIN4 read the key
// through options the guard didn't know: grep's --reg (any unique prefix of --regexp), GREP_OPTIONS, rg's
// RIPGREP_CONFIG_PATH. Telling the pattern from a file means knowing every option spelling, variable and config
// file of every search tool, so the word is refused wherever it stands. The cost: searching other files for
// the word is refused too.
func TestTheGuardRefusesASearchForTheSetupKeyWord(t *testing.T) {
	const support = `~/Library/Application\ Support/AgentDeck`
	runs := []string{
		// RWIN3's "these still run": the word inside a word, or not the word.
		`git commit -m "the setup-key file"`, "find . -name '*key*'", "ls setup-*.md", "sort < in.txt", "cat < notes.md",
		"wc -l < setup-keys.md", "grep -rn hook-key desktop/", "git log -S setupKey --oneline",
		// A word that holds setup-key but isn't it: as a file, it isn't the key either.
		"egrep -c 'setup-key|hook-key' notes.md",
	}
	refused := []string{
		// RWIN4 B1's table: grep reads setup-key as a file, through an option or a variable the guard can't know.
		"grep setup-key --reg .", "grep setup-key --reg=.", "GREP_OPTIONS='-e .' grep setup-key",
		"RIPGREP_CONFIG_PATH=rgcfg rg setup-key", "cd docs && grep setup-key --reg .", // WIN125k: a cd into the support folder is refused first
		"egrep setup-key --reg .", "grep -E setup-key --rege=.", "grep --exclude-f setup-key x .",
		"env GREP_OPTIONS='-e .' grep setup-key", "rg --no-config setup-key",
		// WIN125d let these run as searches of other files for the word: refused again (RWIN4 B1), as the
		// guard can't tell for sure that setup-key is the pattern and not a file.
		"grep -rn setup-key desktop/", `grep -rn "setup-key" .`, "git grep setup-key", "rg -l setup-key Sources/",
		"git log -S setup-key --oneline", "git log --grep=setup-key",
		"go test ./internal/guard/ 2>&1 | grep setup-key",
		"grep -e setup-key -r desktop/", "grep --regexp setup-key desktop/x.go", "grep -i -- setup-key README.md",
		"rg -n --glob '*.go' setup-key", "rg -C 2 setup-key internal/",
		"git log -G setup-key -p", "git log --all --grep setup-key", "git -C ../app grep -n setup-key -- '*.go'",
		"git grep -e setup-key HEAD~1", "grep -esetup-key notes.md", "git log -Ssetup-key",
		// RWIN3's table: they read the key, or name it as a file.
		"shasum -a 256 setup-key | cut -c1-64 | shasum | cut -c1-12",
		"stat -f '%N %z %m' " + support + "/setup-key", "ls -la " + support + "/setup-key", `test -f "$S/setup-key"`,
		"ls *key*", "cat set*", "ls -d setup*", "rm -f *-key", "find . -name *key*", "ls docs/setup-*",
		// A search whose files hold the key: by its path, by -f, --glob, --include, a pattern that fills it in.
		"grep setup-key " + support + "/setup-key", "grep -r x " + support + "/setup-key", "grep -e x setup-key",
		"grep -f setup-key notes.md", "grep -fsetup-key notes.md", "grep --file=setup-key notes.md",
		"rg -g setup-key x .", "rg --glob=setup-key x .", "grep -r --include=setup-key x .", "grep -r --include setup-key x .",
		"grep x set*", "grep set* .", "grep -e x set*", "rg x " + support + "/setup-k?y",
		"git grep x -- setup-key", "git grep -e x HEAD -- " + support + "/setup-key", "git log -S x -- setup-key",
		"git log setup-key", "grep -- x setup-key", "grep setup-key < setup-key", "grep -c setup-key setup-key",
		"grep -r x setup-key", "rg -r y x setup-key",
		// Not a search: the word names the key.
		"setup-key", "cat setup-key", "xargs grep setup-key", "echo setup-key", "git show setup-key",
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range runs {
			if r := Check(line, groups); r != nil {
				t.Errorf("refused %q: %v", line, r)
			}
		}
		for _, line := range refused {
			// *key* fills in the push key's file too (PUSH1), which the always list names first: refused either way.
			r := Check(line, groups)
			if r == nil || r.Rule != setupKeyFile && !(strings.Contains(line, "*key*") && r.Rule == "push-key-kept") {
				t.Errorf("%q: %v", line, r)
			}
		}
	}
	// RWIN3 N8: a > to the key shows as a >.
	if r := Check("echo x > setup-key", DefaultGroups()); r == nil || r.Command != "> setup-key" {
		t.Errorf("a > to the key: %v", r)
	}
}
