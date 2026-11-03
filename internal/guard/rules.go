package guard

import (
	"strings"
	"sync"
)

// DefaultGroups is what every hire is refused unless the owner's
// hire-guard.json says otherwise.
func DefaultGroups() []Group {
	return []Group{
		{Name: "push", On: true, Commands: []Rule{
			"git push|send-pack", "git subtree push", "git svn dcommit", "git lfs push",
		}},
		{Name: "deploy", On: true, Commands: []Rule{
			"vercel", "netlify", "wrangler",
			// firebase's topics are written topic:action (hosting:channel:deploy, functions:delete)
			"firebase deploy|hosting|functions|apphosting|firestore|database|extensions|appdistribution|remoteconfig|dataconnect", "fly", "railway", "heroku", "render",
			"eas build|submit|update", "supabase db push", "supabase functions deploy",
			"kubectl apply|delete", "terraform apply|destroy", "pulumi up|destroy",
			"docker push", "docker image push", "docker compose push", "docker-compose push", "docker manifest push",
			"docker buildx build|bake --push", "docker build --push", "docker builder build --push",
			"podman push", "podman image push", "podman manifest push", "podman compose push",
		}},
		{Name: "publish", On: true, Commands: []Rule{
			// npm takes any unambiguous start of a command's name (checked in npm's own cmd-list.js, abbrev): pu, pub, …
			"npm publish|publis|publi|publ|pub|pu", "pnpm publish", "yarn publish", "bun publish", "cargo publish", "twine upload", "gem push", "fastlane",
			"xcrun altool|notarytool",
			"gh pr create|merge|close|edit|comment|review|ready|reopen|update-branch|revert|lock|unlock",
			"gh issue create|close|edit|comment|delete|reopen|transfer|lock|unlock|pin|unpin|develop",
			"gh release", "gh repo create|delete|edit|fork|rename|archive|unarchive|sync|deploy-key|autolink", "gh gist create|edit|delete",
			"gh label create|delete|edit|clone", "gh ssh-key add|delete", "gh gpg-key add|delete", "gh codespace create|delete|ssh", "gh extension install", "gh cache delete",
			// gh's token goes to curl, and with it to everything above
			"gh auth token|refresh|login|logout|setup-git", "gh auth status -t|--show-token",
			"npm unpublish|deprecate|dist-tag|access|owner|token|login|adduser|team|org",
			"gh workflow run|enable|disable", "gh run rerun|cancel|delete", "gh secret", "gh variable",
			"gh api -X|--method|-f|-F|--field|--raw-field|--input",
		}},
		{Name: "cloud", On: true, Commands: []Rule{"aws", "gcloud", "az", "gsutil", "bq", "cdk"}},
		{Name: "remote", On: true, Commands: []Rule{"ssh", "scp", "sftp", "rsync @remote"}},
		{Name: "local", On: true, Commands: []Rule{
			"tmux ... send-keys|send-prefix|kill-session|kill-server|kill-window|kill-pane|respawn-pane|respawn-window|pipe-pane|run-shell|if-shell|paste-buffer|source-file|command-prompt",
			"smolcorp gm hire|fire|brief|tile|watch|unwatch|rotate-token|screen",
			"security", "osascript",
			// stored credentials, printed
			"git credential|credential-osxkeychain|credential-store|credential-cache|credential-manager", "op read|item|inject|signin", "bw", "pass",
		}},
	}
}

// The guard's profiles: a hire's, and the General Manager's own.
const (
	ProfileHire = "hire"
	ProfileGM   = "gm"
)

// ForGM is groups as the General Manager is held to them: the same, except that
// the rules refusing its own `smolcorp gm` actions (hire, fire, brief…) are
// left out. Push, deploy, publish, cloud, remote, tmux send-keys and the rest stay.
func ForGM(groups []Group) []Group {
	out := make([]Group, len(groups))
	for i, g := range groups {
		out[i] = g
		out[i].gm = true
		out[i].Commands = nil
		for _, r := range g.Commands {
			if w := ruleWords(r); len(w) >= 2 && strings.EqualFold(w[0], "smolcorp") && strings.EqualFold(w[1], "gm") {
				continue
			}
			out[i].Commands = append(out[i].Commands, r)
		}
	}
	return out
}

// matches is whether rule refuses the command name with args.
func matches(rule Rule, name string, args []string) bool {
	parts := ruleWords(rule)
	if len(parts) == 0 || !oneOf(strings.ToLower(parts[0]), name) {
		return false
	}
	var subs, flags []string
	anywhere, remote := false, false
	for _, p := range parts[1:] {
		switch {
		case p == "...":
			anywhere = true
		case p == "@remote":
			remote = true
		case strings.HasPrefix(p, "-"):
			flags = append(flags, strings.Split(p, "|")...)
		default:
			subs = append(subs, strings.ToLower(p)) // a rule's case never matters; the command's is lowered
		}
	}
	if len(subs) > 0 {
		if anywhere {
			if !anywhereIn(subs, args) {
				return false
			}
		} else if subcommandAnywhere[name] && anywhereIn(subs, args) {
			// A global option's value we don't know in front of the subcommand (npm --omit dev publish)
			// is no way round the rule: the subcommand is looked for among all the words too.
		} else {
			pos := positional(name, args)
			if len(pos) < len(subs) {
				return false
			}
			for i, s := range subs {
				// eas build:submit and firebase hosting:channel:deploy: the word before the first : counts too.
				word := strings.ToLower(pos[i])
				if !oneOf(s, word) && !oneOf(s, word[:strings.IndexByte(word+":", ':')]) {
					return false
				}
			}
		}
	}
	if len(flags) > 0 && !hasFlag(flags, args) {
		return false
	}
	if remote && !hasRemote(args) {
		return false
	}
	return true
}

// subcommandAnywhere are the tools whose rules also match their subcommand at
// any place among the arguments, since every tool has global options with a
// value that nobody lists in full. It may refuse npm install publish; that
// is the safe side.
var subcommandAnywhere = map[string]bool{
	"npm": true, "pnpm": true, "yarn": true, "bun": true, "cargo": true, "kubectl": true, "pulumi": true, "supabase": true,
	"terraform": true, "docker": true, "podman": true, "gh": true, "eas": true,
}

func oneOf(alternatives, word string) bool {
	for {
		bar := strings.IndexByte(alternatives, '|')
		if bar < 0 {
			return alternatives == word
		}
		if alternatives[:bar] == word {
			return true
		}
		alternatives = alternatives[bar+1:]
	}
}

// ruleWords is a rule's words, split once: the same rules meet every command.
var ruleCache sync.Map

func ruleWords(rule Rule) []string {
	if v, ok := ruleCache.Load(rule); ok {
		return v.([]string)
	}
	parts := strings.Fields(string(rule))
	ruleCache.Store(rule, parts)
	return parts
}

func anywhereIn(subs, args []string) bool {
	for i := range args {
		ok := i+len(subs) <= len(args)
		for j := 0; ok && j < len(subs); j++ {
			ok = oneOf(subs[j], strings.ToLower(args[i+j]))
		}
		if ok {
			return true
		}
	}
	return false
}

func hasFlag(flags, args []string) bool {
	for _, a := range args {
		for _, f := range flags {
			if a == f || strings.HasPrefix(a, f+"=") || (len(f) == 2 && strings.HasPrefix(a, f) && !strings.HasPrefix(a, "--")) {
				return true
			}
		}
	}
	return false
}

// hasRemote is whether an argument names another host: host:path or
// user@host:path, or an rsync:// address.
func hasRemote(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if strings.Contains(a, "://") {
			return true
		}
		colon := strings.IndexByte(a, ':')
		if colon > 0 && !strings.Contains(a[:colon], "/") {
			return true
		}
	}
	return false
}

// valueOptions are, by command, the options before its subcommand that
// take the next word as their value: git -C <dir> push.
var valueOptions = map[string][]string{
	"git":      {"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path", "--config-env", "--super-prefix", "--attr-source"},
	"gh":       {"-R", "--repo", "--hostname"},
	"tmux":     {"-L", "-S", "-f", "-c", "-T"},
	"supabase": {"--workdir", "--profile", "--project-ref", "--db-url"},
	"npm": {"--prefix", "-w", "--workspace", "--registry", "--userconfig", "--otp", "--tag", "--access", "--loglevel",
		"--cache", "--scope", "--tolerate-republish"},
	"pnpm":     {"-C", "--dir", "--filter", "-F", "-w", "--workspace", "--reporter", "--loglevel"},
	"yarn":     {"--cwd"},
	"bun":      {"--cwd", "--filter"},
	"smolcorp": {"--port", "--test"},
	"vercel":   {"--cwd", "--scope", "-S", "--token", "-t", "-A", "--local-config", "-Q", "--global-config"},
	"netlify":  {"--cwd", "--auth"},
	"firebase": {"--project", "-P", "--config", "-c", "--token"},
	"wrangler": {"--config", "-c", "--env", "-e"},
	"eas":      {"--profile", "-e", "--platform", "-p"},
	"xcrun":    {"--sdk", "-sdk", "--toolchain", "-toolchain"},
	"kubectl": {"-n", "--namespace", "--context", "--kubeconfig", "--cluster", "--user", "-s", "--server", "--token", "--as",
		"--as-group", "--username", "--password", "--certificate-authority", "--client-certificate", "--client-key", "--request-timeout"},
	"docker": {"-H", "--host", "-c", "--context", "--config", "-l", "--log-level", "--tlscacert", "--tlscert", "--tlskey",
		"-f", "--file", "-p", "--project-name", "--profile", "--env-file", "--project-directory", "--builder"}, // the last are compose's and buildx's
	"docker-compose": {"-f", "--file", "-p", "--project-name", "--profile", "--env-file", "--project-directory"},
	"podman": {"-c", "--connection", "--url", "--identity", "--log-level", "--root", "--runroot", "--cgroup-manager",
		"--storage-driver", "--storage-opt", "--events-backend", "--hooks-dir", "--namespace", "--runtime", "--tmpdir", "--conmon",
		"-f", "--file", "-p", "--project-name", "--env-file"},
	"terraform": {},
	"pulumi":    {"-C", "--cwd", "-s", "--stack"},
}

// positional is a command's words that aren't options or their values.
func positional(name string, args []string) []string {
	var out []string
	takes := valueOptions[name]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case i == 0 && name == "cargo" && strings.HasPrefix(a, "+"):
			// cargo +nightly publish: a toolchain, not a subcommand.
		case a == "--":
			return append(out, args[i+1:]...)
		case strings.HasPrefix(a, "-") && len(a) > 1:
			if !strings.Contains(a, "=") && takesNext(a, takes) {
				i++
			}
		default:
			out = append(out, a)
		}
	}
	return out
}

// inner is what a wrapper runs: a line for the shell, or words.
type inner struct {
	line  string
	words simple
	ps    bool // the line is PowerShell or cmd, not bash
}

var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "fish": true, "csh": true, "tcsh": true}

// wrapped is what a command runs in its turn: sh -c's line, eval's words,
// env's, npx's and xargs's command, find's -exec, tmux's new window.
func wrapped(name string, args []string) []inner {
	rest := func(skip []string, from int) []inner {
		for i := from; i < len(args); i++ {
			a := args[i]
			if a == "--" {
				return []inner{{words: args[i+1:]}}
			}
			if strings.HasPrefix(a, "-") && len(a) > 1 {
				if takesNext(a, skip) {
					i++
				}
				continue
			}
			if isAssignment(a) && (name == "env" || name == "sudo") {
				continue
			}
			return []inner{{words: args[i:]}}
		}
		return nil
	}
	switch {
	case shells[name]:
		for i, a := range args {
			if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a, 'c') {
				for _, b := range args[i+1:] {
					if !strings.HasPrefix(b, "-") {
						return []inner{{line: b}}
					}
				}
				break // a later -c has no more words after it than this one
			}
		}
		for _, a := range args {
			if !strings.HasPrefix(a, "-") {
				return nil // a script file: not seen here
			}
		}
		// No line and no script: it runs whatever is piped in.
		return []inner{{words: simple{"\x00stdin"}}}
	case name == "powershell" || name == "pwsh":
		// -Command's line is parsed again; -EncodedCommand can't be read, so it is refused.
		// PowerShell takes any prefix of an option's name: -com, -en.
		for i, a := range args {
			kind, value := psOption(a)
			switch kind {
			case "encoded":
				return []inner{{words: simple{"\x00opaque", name, a}}}
			case "command":
				return []inner{{line: strings.TrimSpace(value + " " + strings.Join(args[i+1:], " ")), ps: true}}
			}
		}
		return nil
	case name == "cmd":
		for i, a := range args {
			if low := strings.ToLower(a); low == "/c" || low == "/k" || low == "/r" {
				return []inner{{line: strings.Join(args[i+1:], " "), ps: true}}
			}
		}
	case name == "invoke-expression" || name == "iex":
		if len(args) == 0 {
			return []inner{{words: simple{"\x00stdin"}}} // ... | iex runs what it is piped
		}
		return []inner{{line: strings.Join(args, " "), ps: true}}
	case name == "start-process" || name == "start" || name == "saps":
		// The program is a plain word or the one after -FilePath (any start of its name: -f, -file),
		// and what follows is split the way -ArgumentList takes it: 'push origin' or push,origin. The first
		// few plain words are each tried, as an option's value may come first (-WindowStyle Hidden git push).
		if len(args) > 200 {
			return []inner{{words: simple{"\x00long", name}}}
		}
		var out []inner
		program := func(first string, more []string) {
			words := simple{strings.Trim(first, `'"`)}
			for _, b := range more {
				for _, f := range strings.Fields(strings.ReplaceAll(b, ",", " ")) {
					words = append(words, strings.Trim(f, `'"`))
				}
			}
			out = append(out, inner{words: words})
		}
		plain := 0
		for i, a := range args {
			if !strings.HasPrefix(a, "-") {
				if plain++; plain <= 8 {
					program(a, args[i+1:])
				}
				continue
			}
			opt, value := strings.ToLower(a[1:]), ""
			if colon := strings.IndexByte(opt, ':'); colon >= 0 {
				opt, value = opt[:colon], a[1+colon+1:]
			}
			if opt != "" && strings.HasPrefix("filepath", opt) {
				// The program is named here; its arguments are the other words, before or after.
				if value != "" {
					program(value, append(append([]string{}, args[:i]...), args[i+1:]...))
				} else if i+1 < len(args) {
					program(args[i+1], append(append([]string{}, args[:i]...), args[i+2:]...))
				}
			}
		}
		return out
	case name == "eval":
		return []inner{{line: strings.Join(args, " ")}}
	case name == "su":
		for i, a := range args {
			if (a == "-c" || a == "--command") && i+1 < len(args) {
				return []inner{{line: args[i+1]}}
			}
		}
	case name == "env":
		// -S 'line' (also -S'line', -iS 'line' and --split-string=line) runs a line.
		var out []inner
		splits := 0
	scan:
		for i := 0; i < len(args); i++ {
			a := args[i]
			if strings.HasPrefix(a, "-") && (strings.HasPrefix(a, "--split-string") || !strings.HasPrefix(a, "--") && strings.Contains(a, "S")) {
				// Each -S makes a line of everything after it: a handful is read, a flood is refused.
				if splits++; splits > 8 {
					return []inner{{words: simple{"\x00long", name}}}
				}
			}
			switch {
			case a == "--" || !strings.HasPrefix(a, "-") && !isAssignment(a):
				break scan
			case strings.HasPrefix(a, "--split-string="):
				out = append(out, inner{line: strings.TrimPrefix(a, "--split-string=") + " " + strings.Join(args[i+1:], " ")})
			case a == "--split-string" && i+1 < len(args):
				out = append(out, inner{line: strings.Join(args[i+1:], " ")})
			case strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "S"):
				if v := a[strings.Index(a, "S")+1:]; v != "" {
					out = append(out, inner{line: v + " " + strings.Join(args[i+1:], " ")})
				} else if i+1 < len(args) {
					out = append(out, inner{line: strings.Join(args[i+1:], " ")})
				}
			}
		}
		return append(out, rest([]string{"-u", "--unset", "-C", "--chdir", "-P", "--path", "-a", "--argv0"}, 0)...)
	case name == "sudo" || name == "doas":
		return rest([]string{"-u", "--user", "-g", "--group", "-h", "--host", "-p", "--prompt", "-C", "--close-from", "-D", "--chdir",
			"-R", "--chroot", "-T", "--command-timeout", "-U", "--other-user", "-r", "--role", "-t", "--type"}, 0)
	case name == "command":
		for _, a := range args {
			if a == "-v" || a == "-V" {
				return nil
			}
		}
		return rest(nil, 0)
	case name == "exec":
		return rest([]string{"-a"}, 0)
	case name == "nice":
		return rest([]string{"-n", "--adjustment"}, 0)
	case name == "caffeinate":
		return rest([]string{"-w", "-t"}, 0)
	case name == "timeout" || name == "gtimeout":
		if r := rest([]string{"-s", "--signal", "-k", "--kill-after"}, 0); len(r) == 1 && len(r[0].words) > 1 {
			return []inner{{words: r[0].words[1:]}}
		}
	case name == "xargs":
		return rest([]string{"-I", "-J", "-L", "-n", "-P", "-s", "-E", "-d", "-R", "-S", "-a", "--arg-file", "--delimiter",
			"--max-lines", "--max-args", "--max-procs", "--max-chars", "--process-slot-var", "--replace-str"}, 0)
	case name == "npx" || name == "bunx" || name == "pnpx":
		for i, a := range args {
			if a == "-c" && i+1 < len(args) {
				return []inner{{line: args[i+1]}}
			}
		}
		return rest([]string{"-p", "--package"}, 0)
	case name == "npm" || name == "pnpm" || name == "yarn" || name == "bun":
		pos := positional(name, args)
		if len(pos) == 0 {
			return nil
		}
		if pos[0] == "exec" || pos[0] == "x" || pos[0] == "dlx" {
			for i, a := range args {
				if a == "-c" && i+1 < len(args) {
					return []inner{{line: args[i+1]}}
				}
			}
			return []inner{{words: pos[1:]}}
		}
		if name != "npm" && (pos[0] == "run" || pos[0] == "run-script") {
			// yarn run vercel, bun run vercel, pnpm run vercel: the same package command, one word later.
			if len(pos) > 1 && !scriptNames[commandName(pos[1])] {
				return []inner{{words: pos[1:]}}
			}
			return nil
		}
		if name != "npm" && !scriptNames[commandName(pos[0])] {
			// pnpm vercel, yarn vercel, bun vercel: a package's own command. Not for names that are
			// more often a project's script (pnpm render, yarn fly): those are only the script.
			return []inner{{words: pos}}
		}
	case name == "find":
		var out []inner
		for i := 0; i < len(args); i++ {
			if a := args[i]; a == "-exec" || a == "-execdir" || a == "-ok" || a == "-okdir" {
				end := i + 1
				for end < len(args) && args[end] != ";" && args[end] != "+" {
					end++
				}
				out = append(out, inner{words: args[i+1 : end]})
				i = end // go on after it, so the words are walked once
			}
		}
		return out
	case name == "tmux":
		// A new session, window or pane, or run-shell, runs its last words as a line.
		for i, a := range args {
			switch a {
			case "new-session", "new", "new-window", "neww", "split-window", "splitw", "respawn-pane", "respawnp",
				"respawn-window", "respawnw", "display-popup", "popup", "run-shell", "run", "if-shell", "if":
				var out []inner
				for _, b := range args[i+1:] {
					if !strings.HasPrefix(b, "-") {
						out = append(out, inner{line: b})
					}
				}
				return out
			}
		}
	case name == "time":
		return rest([]string{"-f", "--format", "-o", "--output"}, 0)
	case name == "watch":
		return rest([]string{"-n", "--interval"}, 0)
	case name == "stdbuf":
		return rest([]string{"-i", "--input", "-o", "--output", "-e", "--error"}, 0)
	case name == "arch":
		return rest([]string{"-arch"}, 0)
	case name == "nohup" || name == "builtin" || name == "unbuffer" ||
		name == "then" || name == "do" || name == "else" || name == "elif" || name == "if" || name == "while" || name == "until":
		return rest(nil, 0)
	case name == "setsid" || name == "chronic" || name == "busybox" || name == "coproc":
		return rest(nil, 0)
	case name == "unshare":
		return rest([]string{"--setgroups", "--propagation", "--map-user", "--map-group", "--map-users", "--map-groups",
			"-R", "--root", "-w", "--wd", "-S", "--setuid", "-G", "--setgid", "--monotonic", "--boottime"}, 0)
	case name == "ionice":
		return rest([]string{"-c", "--class", "-n", "--classdata", "-p", "--pid", "-P", "--pgid", "-u", "--uid"}, 0)
	case name == "chroot" || name == "taskset":
		// The first plain word is a directory or a CPU mask; the command follows.
		return afterFirst(rest([]string{"--userspec", "--groups"}, 0))
	case name == "flock":
		for i, a := range args {
			if (a == "-c" || a == "--command") && i+1 < len(args) {
				return []inner{{line: args[i+1]}}
			}
		}
		return afterFirst(rest([]string{"-w", "-E", "--timeout", "--conflict-exit-code"}, 0))
	case name == "trap":
		// trap 'line' SIGNAL: the first plain word is the line.
		for _, a := range args {
			if !strings.HasPrefix(a, "-") || a == "-" {
				return []inner{{line: a}}
			}
		}
	case name == "parallel":
		// The command is after the options, or a word after :::
		out := rest([]string{"-j", "-n", "-N", "-P", "-a", "-S", "-L", "-I", "-d", "-C", "-E", "--jobs", "--arg-file", "--sshlogin", "--max-args",
			"--max-lines", "--delimiter", "--colsep", "--results", "--res", "--joblog", "--retries", "--timeout", "--workdir", "--wd",
			"--env", "--halt", "--load", "--tagstring", "--tmpdir", "--max-replace-args", "--max-procs"}, 0)
		for i, a := range args {
			if a == ":::" {
				for _, b := range args[i+1:] {
					out = append(out, inner{line: b})
				}
				break
			}
		}
		return out
	case name == "wsl":
		return rest([]string{"-d", "--distribution", "-u", "--user", "--cd"}, 0)
	case name == "alias":
		// alias gp='git push', then gp, in a shell that expands aliases.
		var out []inner
		for _, a := range args {
			if eq := strings.IndexByte(a, '='); eq > 0 {
				out = append(out, inner{line: a[eq+1:]})
			}
		}
		return out
	case name == "python" || name == "python3" || name == "py" || strings.HasPrefix(name, "python3."):
		// python -m twine upload: a module's own command line.
		for i, a := range args {
			if a == "-m" && i+1 < len(args) {
				return []inner{{words: args[i+1:]}}
			}
		}
	case name == "xcrun":
		return rest([]string{"--sdk", "-sdk", "--toolchain", "-toolchain"}, 0)
	case name == "sandbox-exec":
		return rest([]string{"-f", "-p", "-n", "-D"}, 0)
	case name == "ssh-agent":
		return rest([]string{"-a", "-E", "-P", "-t"}, 0)
	case name == "sshpass":
		return rest([]string{"-p", "-f", "-d", "-P"}, 0)
	case name == "corepack":
		// corepack pnpm publish: it starts the package manager.
		return rest(nil, 0)
	case name == "dotenv":
		// dotenv -e .env -- cmd, dotenv -f .env run -- cmd
		for i, a := range args {
			if a == "--" {
				return []inner{{words: args[i+1:]}}
			}
		}
		r := rest([]string{"-e", "-f", "--file", "-v", "--variable", "-p", "-c"}, 0)
		if len(r) == 1 && len(r[0].words) > 0 && r[0].words[0] == "run" {
			return []inner{{words: r[0].words[1:]}}
		}
		return r
	case name == "doppler":
		for i, a := range args {
			if a == "--command" && i+1 < len(args) {
				return []inner{{line: args[i+1]}}
			} else if strings.HasPrefix(a, "--command=") {
				return []inner{{line: strings.TrimPrefix(a, "--command=")}}
			}
		}
		return afterSub(args, []string{"-p", "--project", "-c", "--config", "-t", "--token", "--config-dir", "--mount", "--name-transformer"}, "run")
	case name == "mise":
		// mise exec [tool@version …] -- cmd; mise x -c 'line'
		skip := []string{"-C", "--cd", "-j", "--jobs", "-E", "--env"}
		if i := plainAt(args, skip); i >= 0 && (args[i] == "exec" || args[i] == "x") {
			for j := i + 1; j < len(args); j++ {
				switch {
				case args[j] == "--":
					return []inner{{words: args[j+1:]}}
				case (args[j] == "-c" || args[j] == "--command") && j+1 < len(args):
					return []inner{{line: args[j+1]}}
				}
			}
			return rest(skip, i+1)
		}
	case name == "direnv":
		// direnv exec <dir> cmd
		return afterFirst(afterSub(args, []string{"--log-format"}, "exec"))
	case name == "op":
		return afterSub(args, []string{"--account", "--env-file", "--session", "--vault", "--cache", "--config"}, "run")
	case name == "uv":
		return afterSub(args, []string{"--with", "--with-editable", "--with-requirements", "-w", "-p", "--python", "--project",
			"--directory", "--env-file", "--package", "--group", "--extra", "--index", "--default-index"}, "run")
	case name == "poetry":
		return afterSub(args, []string{"-C", "--directory", "-P", "--project"}, "run")
	case name == "bundle" || name == "bundler":
		return afterSub(args, []string{"--gemfile", "--path"}, "exec")
	case name == "pipenv":
		return afterSub(args, nil, "run")
	case name == "conda":
		return afterSub(args, []string{"-n", "--name", "-p", "--prefix", "--cwd"}, "run")
	case name == "rbenv":
		return afterSub(args, nil, "exec")
	case name == "script":
		// script [-q] [file [command …]]
		if r := rest([]string{"-t", "-T"}, 0); len(r) == 1 && len(r[0].words) > 1 {
			return []inner{{words: r[0].words[1:]}}
		}
	}
	return nil
}

// takesNext is whether the option a takes the next word as its value:
// it is one of the options in skip, or a cluster of short options (-nu,
// -tn) whose first letter that takes a value comes last (-nu root: -u's
// value is the next word; -un root: -u's value is n, attached).
func takesNext(a string, skip []string) bool {
	for _, s := range skip {
		if a == s {
			return true
		}
	}
	if len(a) < 3 || strings.HasPrefix(a, "--") {
		return false
	}
	for k := 1; k < len(a); k++ {
		c := a[k]
		if c >= '0' && c <= '9' {
			continue
		}
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
		for _, s := range skip {
			if s == "-"+string(c) {
				return k == len(a)-1
			}
		}
	}
	return false
}

// scriptNames are tools' names that a project is likely to have a script of
// the same name for: pnpm render runs the script, not Render's CLI.
var scriptNames = map[string]bool{"render": true, "fly": true, "heroku": true, "railway": true, "fastlane": true}

// plainAt is the place of the first word in args that is no option or
// option value (the words in skip take the next word as their value); -1 if none.
func plainAt(args, skip []string) int {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return -1
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			if takesNext(a, skip) {
				i++
			}
			continue
		}
		return i
	}
	return -1
}

// afterSub is what a tool like `uv run`, `bundle exec` or `op run` starts:
// when args' first plain word is sub, the command after it (skip lists the
// options that take a value, which come before and after sub).
func afterSub(args, skip []string, sub string) []inner {
	i := plainAt(args, skip)
	if i < 0 || args[i] != sub {
		return nil
	}
	for j := i + 1; j < len(args); j++ {
		a := args[j]
		switch {
		case a == "--":
			return []inner{{words: args[j+1:]}}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			if takesNext(a, skip) {
				j++
			}
		default:
			return []inner{{words: args[j:]}}
		}
	}
	return nil
}

// afterFirst drops the first word of the one command rest found.
func afterFirst(r []inner) []inner {
	if len(r) == 1 && len(r[0].words) > 1 {
		return []inner{{words: r[0].words[1:]}}
	}
	return nil
}

// psOption says whether a PowerShell argument is -Command ("command") or
// -EncodedCommand ("encoded"), in any of its shortened forms (-c, -com,
// -en, -ec) and with a value attached (-Command:"…"), and that value.
func psOption(arg string) (kind, value string) {
	if !strings.HasPrefix(arg, "-") {
		return "", ""
	}
	name := strings.ToLower(arg[1:])
	if colon := strings.IndexByte(name, ':'); colon >= 0 {
		name, value = name[:colon], arg[1+colon+1:]
	}
	switch {
	case name == "":
		return "", ""
	case name == "ec" || strings.HasPrefix("encodedcommand", name):
		return "encoded", value
	case strings.HasPrefix("command", name):
		return "command", value
	}
	return "", ""
}

// tmuxCommands are tmux's command names; tmux takes any unambiguous prefix
// of one (send-k, kill-ses) and a few short aliases.
var tmuxCommands = strings.Fields(`attach-session bind-key break-pane capture-pane choose-buffer choose-client choose-tree
clear-history clear-prompt-history clock-mode command-prompt confirm-before copy-mode customize-mode delete-buffer detach-client
display-menu display-message display-panes display-popup find-window has-session if-shell join-pane kill-pane kill-server
kill-session kill-window last-pane last-window link-window list-buffers list-clients list-commands list-keys list-panes
list-sessions list-windows load-buffer lock-client lock-server lock-session move-pane move-window new-session new-window
next-layout next-window paste-buffer pipe-pane previous-layout previous-window refresh-client rename-session rename-window
resize-pane resize-window respawn-pane respawn-window rotate-window run-shell save-buffer select-layout select-pane
select-window send-keys send-prefix server-access set-buffer set-environment set-hook set-option set-window-option
show-buffer show-environment show-hooks show-messages show-options show-prompt-history show-window-options source-file
split-window start-server suspend-client swap-pane swap-window switch-client unbind-key unlink-window wait-for`)

var tmuxAliases = map[string]string{
	"attach": "attach-session", "detach": "detach-client", "new": "new-session", "neww": "new-window", "ls": "list-sessions",
	"send": "send-keys", "run": "run-shell", "if": "if-shell", "splitw": "split-window", "respawnp": "respawn-pane",
	"respawnw": "respawn-window", "popup": "display-popup", "source": "source-file", "pasteb": "paste-buffer",
	"pipep": "pipe-pane", "killp": "kill-pane", "killw": "kill-window",
}

// tmuxFull spells out each word that tmux would read as a command's short
// form, so that send-k and kill-ses meet the rules as send-keys and
// kill-session. A word that fits several commands (kill-) is left.
func tmuxFull(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a
		if strings.HasPrefix(a, "-") || a == "" {
			continue
		}
		if full, ok := tmuxAliases[a]; ok {
			out[i] = full
			continue
		}
		var found string
		n := 0
		for _, c := range tmuxCommands {
			if c == a {
				found, n = c, 1
				break
			}
			if strings.HasPrefix(c, a) {
				found, n = c, n+1
			}
		}
		if n == 1 {
			out[i] = found
		}
	}
	return out
}
