package guard

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// T307b: what the second review found. Section A of its verdict must be
// refused, section B must run.

// A1: a here document's body is no shell text with quotes in it.
func TestHereDocumentsHideNothing(t *testing.T) {
	mustRefuse(t, "heredoc", []string{
		"cat <<EOF\ndon't\nEOF\ngit push",
		"git commit -m \"$(cat <<'EOF'\nit's done\nEOF\n)\" && git push",
		"cat <<EOF\ndon't\nEOF\ngit push\n",
		"cat <<-EOF\n\tit's\n\tEOF\ngit push",
		"cat <<'EOF' | cat; git push\nit's\nEOF",
		"cat <<EOF <<EOT\na'b\nEOF\nc\"d\nEOT\ngit push",
		"cat <<\"EOF\"\ndon't\nEOF\nvercel deploy",
		"cat <<EOF\n# it's $(git push)\nEOF",     // an unquoted body runs $( ) even after a #
		"cat <<EOF\ndon't `git push`\nEOF",       // ... and backticks
		"x=$(cat <<EOF\ndon't\nEOF\n); git push", // the parenthesis after the body closes the $(
		"echo $(cat <<EOF\n)\nEOF\n); git push",  // a ) in a body closes nothing
		"bash <<EOF\ngit push\nEOF",
		"(cat <<EOF\nit's\nEOF\n) && git push",
	})
	mustRun(t, "heredoc", []string{
		"cat <<EOF\ndon't\nEOF",
		"git commit -m \"$(cat <<'EOF'\nit's done\nEOF\n)\"",
		"cat <<EOF\ndon't\nEOF\necho ok",
		"cat <<<'it'\"'\"'s'",
		"echo $((1<<2))",
		"echo $((1 << 2)) && ls",
		"cat <<-EOF\n\tit's\n\tEOF\nls",
		"cat <<EOF\ngit push\nEOF", // cat only prints its text (T307b fix round 1, B7)
	})
}

// A2: no input makes the guard slow.
func TestNoInputMakesTheGuardSlow(t *testing.T) {
	big := 1 << 20
	for name, line := range map[string]string{
		"braces":        strings.Repeat("{", big) + " ; git push",
		"closed braces": strings.Repeat("{a,", big/3) + " ; git push",
		"start":         "start " + strings.Repeat("a ", big/2) + "; git push",
		"start-process": "Start-Process " + strings.Repeat("a ", big/2),
		"env -S":        "env " + strings.Repeat("-Sx ", big/4),
		"find -exec":    "find . " + strings.Repeat("-exec ", big/6),
		"sh -c -c":      "sh " + strings.Repeat("-c ", big/3),
		"tmux":          "tmux " + strings.Repeat("a ", big/2),
		"sudo":          strings.Repeat("sudo ", big/5) + "ls",
		"nested":        strings.Repeat("$(", big/2),
		"backticks":     strings.Repeat("`", big),
		"quotes":        strings.Repeat("'", big),
		"heredocs":      strings.Repeat("cat <<A ", big/8) + "\nit's",
		"heredoc body":  "cat <<EOF\n" + strings.Repeat("it's\n", big/5) + "EOF\ngit push",
		"redirects":     strings.Repeat(">a ", big/3),
		"long words":    strings.Repeat(strings.Repeat("x", 4000)+"{a,b} ", big/4000),
		"many words":    strings.Repeat("a ", big/2) + "; git push",
	} {
		start := time.Now()
		r := Check(line, DefaultGroups())
		took := time.Since(start)
		// Each is linear: the slowest, tmux, takes about 0.4 s on a Mac and 2.15 s on GitHub's Windows
		// runner, and twice as long for twice the line. A guard that goes quadratic takes minutes here.
		// The race detector makes it 2.1 s on a Mac: see raceSlowdown.
		if took > 5*time.Second*raceSlowdown {
			t.Errorf("%s: took %v", name, took)
		}
		// A deny for each that ends with git push or can't be read; a few are only a long harmless line.
		switch name {
		case "braces", "closed braces", "start", "many words", "heredoc body":
			if r == nil {
				t.Errorf("%s: let through", name)
			}
		}
	}
}

func TestAHugeBraceWordIsRefusedNotExpanded(t *testing.T) {
	for _, line := range []string{
		strings.Repeat("{", 100000) + " ; git push",
		strings.Repeat("{a,b}", 100) + " x",          // would be 2^100 words
		"{" + strings.Repeat("x", 5000) + ",y} push", // longer than 4096
		"{1..99999999999999999} x", "{-9223372036854775808..9223372036854775807} x",
	} {
		if r := Check(line, DefaultGroups()); r == nil || !strings.Contains(r.Rule, "can't read") {
			t.Errorf("not refused as unreadable: %.40s… (%v)", line, r)
		}
	}
	// As an argument it is just a word.
	if r := Check("echo "+strings.Repeat("{", 100000), DefaultGroups()); r != nil {
		t.Errorf("refused an echo: %v", r)
	}
}

// A3: git's own programs.
func TestGitsOwnProgramsByName(t *testing.T) {
	mustRefuse(t, "git-*", []string{
		"/usr/libexec/git-core/git-push origin", "git-push", "git-send-pack origin x", "git-subtree push -P x origin main", "git-lfs push origin",
		"sudo git-push", "env FOO=1 /usr/lib/git-core/git-push", "GIT-PUSH", `'C:\Program Files\Git\mingw64\libexec\git-core\git-push.exe' origin`,
		"xargs git-push",
	})
	mustRun(t, "git-*", []string{
		"git-status", "/usr/libexec/git-core/git-log", "git-lfs ls-files", "git-subtree split -P x", "git-diff", "cat git-push.md",
	})
}

// A4: wrapper options that take a value, short and long.
func TestWrapperOptionsWithValues(t *testing.T) {
	mustRefuse(t, "wrapper options", []string{
		"sudo --user root git push", "sudo --group wheel git push", "sudo --host h git push", "sudo --prompt p git push",
		"sudo --chdir /tmp git push", "sudo -r r git push", "doas -u root git push", "sudo --other-user u git push",
		"xargs -a /dev/null git push", "xargs --max-args 1 git push", "xargs --max-procs 2 git push", "xargs --delimiter , git push",
		"xargs --arg-file f git push", "xargs --max-lines 1 git push", "xargs --max-chars 9 git push",
		"/usr/bin/time -f %e git push", "time -o out git push", "time --format %e git push", "time --output o git push", "time -p git push",
		"nice --adjustment 5 git push", "nice -n 5 git push", "env -a x git push", "env --argv0 x git push",
		"stdbuf -o L git push", "stdbuf --output L git push", "arch -arch arm64 git push", "watch -n 1 git push", "watch --interval 1 git push",
		"ionice --class 3 git push", "ionice --classdata 1 git push", "chroot --userspec root:root / git push", "flock -w 5 /tmp/l git push",
		"flock --timeout 5 /tmp/l git push", "parallel --jobs 2 git push", "parallel --colsep , git push", "unshare --map-user 1 git push",
		"timeout --signal KILL 5 git push", "timeout -k 1 5 git push", "unbuffer -p git push", "caffeinate -w 1 git push",
	})
	mustRun(t, "wrapper options", []string{
		"sudo --user root ls", "xargs -a /dev/null ls", "/usr/bin/time -f %e ls", "nice --adjustment 5 ls", "stdbuf -o L ls",
		"watch -n 1 date", "time -p ls", "ionice --class 3 ls",
	})
}

// A5: wrappers that were not listed.
func TestMoreWrappersNotInTheList(t *testing.T) {
	mustRefuse(t, "wrappers", []string{
		"xcrun git push", "xcrun --sdk macosx git push", "sandbox-exec -f p git push", "sandbox-exec -p '(version 1)' git push",
		"ssh-agent git push", "ssh-agent -t 1h git push", "sshpass -p x ssh host", "sshpass -e ssh host", "sshpass -f pw ssh host",
		"op run -- git push", "op run --env-file e -- git push", "op --account a run -- git push", "direnv exec . git push",
		"mise exec -- git push", "mise x node@20 -- git push", "mise exec -c 'git push'", "uv run git push", "uv run --with x git push",
		"poetry run git push", "bundle exec git push", "corepack pnpm publish", "corepack npm publish", "corepack yarn publish",
		"doppler run -- git push", "doppler run -p x -c y -- git push", "doppler run --command 'git push'", "dotenv -e .env -- git push",
		"dotenv -f .env run -- git push", "pipenv run git push", "conda run git push", "conda run -n env git push", "rbenv exec git push",
		"corepack pnpm@9 publish",
		// RWIN7 N6: the Mac's taskpolicy, rlwrap, hyperfine's commands, util-linux's script -c.
		"taskpolicy -b git push", "taskpolicy -c utility git push", "taskpolicy -t 4 -l 4 git push", "rlwrap git push",
		"rlwrap -a -H h.txt git push", "hyperfine 'git push'", "hyperfine -w 3 --prepare 'git push' 'ls'",
		"script -c 'git push' /dev/null", "script -qc 'git push' /dev/null", "script --command='git push' /dev/null",
	})
	mustRun(t, "wrappers", []string{
		"xcrun --find git", "sandbox-exec -f p ls", "ssh-agent ls", "op run -- ls", "direnv exec . ls", "mise exec -- ls", "uv run pytest",
		"poetry run pytest", "bundle exec rspec", "corepack enable", "corepack pnpm install", "doppler run -- ls", "dotenv -e .env -- ls",
		"pipenv run pytest", "conda run -n env python x.py", "rbenv exec ruby x.rb", "uv sync", "bundle install", "mise ls",
		"taskpolicy -b make", "rlwrap sqlite3 x.db", "hyperfine 'go test ./...'", "script -q /dev/null ls",
	})
}

// A6: other spellings of rules that exist.
func TestOtherSpellingsOfTheRules(t *testing.T) {
	mustRefuse(t, "spellings", []string{
		"docker image push x", "docker compose push", "docker manifest push x", "docker buildx build --push .", "docker buildx bake --push",
		"docker build --push .", "docker builder build --push .", "podman push x", "podman image push x", "podman manifest push x",
		"docker compose -f x.yml push", "docker --context c image push x", "docker-compose push", "docker buildx --builder b build --push .",
		"firebase hosting:channel:deploy x", "firebase functions:delete f", "firebase hosting:clone a b", "firebase firestore:delete x",
		"eas update:republish", "eas build:submit", "eas build:cancel", "tofu apply", "tofu destroy", "tofu apply -auto-approve",
		"npm pub", "npm pu", "npm publi", "npm publis", "npm --otp 1 pub", "npx npm pub", "pnpm publish",
	})
	mustRun(t, "spellings", []string{
		"docker image ls", "docker compose up", "docker compose -f x.yml up", "docker buildx build .", "docker build -t x .", "docker manifest inspect x",
		"podman ps", "podman image ls", "firebase emulators:start", "firebase use x", "eas whoami", "eas login", "tofu plan", "tofu init",
		"npm pack", "npm prune", "npm p", "npm i", "npm ping", "npm prefix", "npm run publish-docs",
	})
}

// B: ordinary commands that were refused.
func TestOrdinaryCommandsAreNotRefused(t *testing.T) {
	mustRun(t, "false positives", []string{
		"git commit -m \"$(cat <<'EOF'\nfix: a thing\n\n**Bold** note about it\n*emphasis* here\nEOF\n)\"",
		"cat <<'EOF'\n*note* about a thing\nEOF",
		"cat docs/hire-guard.md", "cat internal/guard/hire-guard.json", "grep -n hire-guard README.md", "head -5 hire-guard.json",
		"ls ~/Library/Application\\ Support/AgentDeck/hire-guard.json", "git commit -m 'docs: what hire-guard does'", "git add internal/guard/hire-guard.json",
		"echo hire-guard", `git commit -m "fix: guard rejects g?t and {a,b}"`, `grep -rn "func.*{" --include='*.go' .`, `grep -E 'foo|bar[0-9]+'`,
		"rg -g '*.go' TODO", "find . -name '*.go'", "find . -name '*.go' -exec wc -l {} +", "echo {a,b,c}", "echo ${HOME}", "go test ./...", "swift test",
		"npm run build", "git status", "git diff -- '*.go'", "ls *.md", "ls internal/*/", "[ -f x ]", "[[ -d x ]]", "for f in *.go; do echo $f; done",
		"sed -i '' s/a/b/ x.txt", "awk '{print $1}' x", "cp -r src/{a,b} dst/", "tar --exclude='*.log' -cf x.tar .", "diff <(ls a) <(ls b)",
		"*", "** note", "curl localhost", "gh pr view 1", "gh pr list", "docker ps", "docker build .", "kubectl get pods", "terraform plan",
		"python3 -c 'print(1)'", "tmux capture-pane -p",
	})
	mustRefuse(t, "guard file", []string{
		"echo x > hire-guard.json", "echo x >> ./hire-guard.json", "printf x >hire-guard.json", `echo x > "hire-guard.json"`, "echo x > HIRE-GUARD.json",
		"echo x | tee hire-guard.json", "sed -i s/true/false/ hire-guard.json", "sed -i'' s/a/b/ hire-guard.json", "mv x hire-guard.json",
		"cp x hire-guard.json", "rm hire-guard.json", "ln -sf /dev/null hire-guard.json", "truncate -s 0 hire-guard.json", "dd of=hire-guard.json",
		"python3 -c \"open('hire-guard.json','w').write('{}')\"", "perl -pi -e 's/1/0/' hire-guard.json", "sudo rm hire-guard.json",
		"sh -c 'echo x > hire-guard.json'", "chmod 000 hire-guard.json", "install x hire-guard.json", ": > hire-guard.json", "exec 3>hire-guard.json",
		"echo x &> hire-guard.json", "echo x 2> hire-guard.json", "xargs rm hire-guard.json", "vim hire-guard.json", "nano hire-guard.json",
		"Set-Content hire-guard.json x", "Remove-Item hire-guard.json", "echo x | Out-File hire-guard.json", `echo x > '/a/Hire-Guard.json'`,
		"cat <<EOF > hire-guard.json\n{}\nEOF",
	})
}

func TestLongLinesAreRefusedAsTooManyWords(t *testing.T) {
	long := strings.Repeat("a ", 300)
	for _, line := range []string{"Start-Process " + long, "env " + strings.Repeat("-Sx ", 20) + "ls"} {
		if r := Check(line, DefaultGroups()); r == nil || !strings.Contains(r.Rule, "more words") {
			t.Errorf("%.30s…: %v", line, r)
		}
	}
	for _, line := range []string{"env -S 'ls -l'", "Start-Process notepad", "Start-Process -FilePath notepad -ArgumentList a,b"} {
		mustRun(t, "short", []string{line})
	}
	mustRefuse(t, "start", []string{
		"Start-Process git push", "Start-Process -FilePath git -ArgumentList push", "Start-Process -File git -Arg push", "Start-Process -f git push",
		"Start-Process -FilePath:git push", "Start-Process -WindowStyle Hidden git push", "start git push", "saps git 'push origin'",
		"Start-Process -ArgumentList push -FilePath git", "Start-Process -ArgumentList push,origin -FilePath:git",
	})
	_ = fmt.Sprint
}
