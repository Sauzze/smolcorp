package guard

import (
	"slices"
	"testing"
)

// RT126NS0aR4's blocking findings N1–N4, each a named row in every group set (ns0ar3Sets): a git config write hidden
// by a dash-word after it (N1), a section renamed into one git runs keys of (N2), an option that takes a command
// spelt by its hidden alias or a start of its name (N3), and a wrapper's words joined without their quotes (N4).

// N1–N3: refused by the core in every group set, none included.
func TestNS0aR4RefusedByTheCoreInEverySet(t *testing.T) {
	onFeature := testRepo(t, "feature", "main", "refs/heads/main", "abc\n")
	rows := map[string]string{
		// N1: git config's legacy form takes options only before the key; a later word is a value pattern.
		"config write, then -l":                 `git config core.fsmonitor "git push origin x; false" -l`,
		"config write, then --get":              `git config core.fsmonitor ` + stand + ` --get`,
		"config write, then --list":             `git config core.fsmonitor "git push origin x; false" --list`,
		"config write, then --get-regexp":       `git config core.sshCommand ` + push + ` --get-regexp`,
		"config write, then --unset":            `git config imap.tunnel ` + push + ` --unset`,
		"config write, then -e":                 `git config submodule.sub.update "!git push origin x" -e`,
		"config write, then --rename-section":   `git config core.fsmonitor "git push origin x; false" --rename-section`,
		"config alias, then --get":              `git config alias.p push --get`,
		"config hooksPath, then --get":          `git config core.hooksPath h --get`,
		"config value starting with -":          `git config lfs.customtransfer.x.args "-c touch\ ran"`,
		"config run key, value -x":              `git config core.fsmonitor -x`,
		"config set, value -x":                  `git config set core.fsmonitor -x`,
		"config --list with words":              `git config --list core.fsmonitor x`,
		"config -- before the key":              `git config -- core.fsmonitor x`,
		"config --end-of-options":               `git config --end-of-options core.fsmonitor x`,
		"config set after an option":            `git config --global set core.fsmonitor ` + stand,
		"config set, option after the key":      `git config set core.fsmonitor t --all`,
		"config an option the guard can't read": `git config --frob`,
		// N2: a section renamed or removed outside user, color, advice and branch.
		"rename into gpg.ssh":              `git config --rename-section foo.ssh gpg.ssh`,
		"rename into protocol.ext":         `git config --rename-section foo.ext protocol.ext`,
		"rename-section subcommand, alias": `git config rename-section foo alias`,
		"rename, upper case section":       `git config rename-section foo.ssh GPG.ssh`,
		"rename by a start of its name":    `git config --rename-sec foo.ssh gpg.ssh`,
		"rename out of gpg":                `git config --rename-section gpg foo`,
		"rename, a name it can't tell":     `git config --rename-section 'foo.$x' user.x`,
		"rename, one name":                 `git config --rename-section foo`,
		"remove-section subcommand, gpg":   `git config remove-section gpg`,
		"remove protocol":                  `git config --remove-section protocol`,
		"defaultKeyCommand, any section":   `git config foo.ssh.defaultKeyCommand ` + stand,
		// N3: ls-remote's hidden --exec, and starts of a name git takes as it (git 2.55's --help-all).
		"ls-remote --exec=":           `git ls-remote --exec=` + push + ` .`,
		"ls-remote --exec, next word": `git ls-remote --exec ` + push + ` .`,
		"ls-remote --ex=":             `git ls-remote --ex=` + push + ` .`,
		"ls-remote --exe, stand-in":   `git ls-remote --exe ` + stand + ` .`,
		"ls-remote --u=":              `git ls-remote --u=` + stand + ` .`,
		"clone --u=":                  `git clone --u=` + stand + ` u d`,
		"clone --co=":                 `git clone --co=core.fsmonitor=x u d`,
		"push --e=":                   `git push --e=` + stand + ` origin`,
		"send-pack --e=":              `git send-pack --e=` + stand + ` . x`,
		"archive --e=":                `git archive --remote=. --e=` + stand + ` HEAD`,
		"difftool --e=":               `git difftool --e=` + stand,
		"send-email -to-cmd":          `git send-email -to-cmd=` + stand + ` x`,
		"send-email +TO-CMD":          `git send-email +TO-CMD=` + stand + ` x`,
		"send-email --H":              `git send-email --H=` + stand + ` x`,
		"send-email -sendmail-cmd":    `git send-email -sendmail-cmd ` + stand + ` x`,
	}
	for row, line := range rows {
		for set, groups := range ns0ar3Sets(t) {
			t.Run(row+", "+set, func(t *testing.T) {
				if got := refusedOrNot(line, onFeature, groups); got != "guard" {
					t.Errorf("%q: %q, want guard", line, got)
				}
			})
		}
	}
}

// N4: a wrapper's words are read as the shell gets them, so the push inside is the push group's wherever it is on, and
// runs where it isn't (none of these is the core's).
func TestNS0aR4WrappedWordsKeepTheirQuotes(t *testing.T) {
	onFeature := testRepo(t, "feature", "main", "refs/heads/main", "abc\n")
	rows := map[string]string{
		"bisect run sh -c":                   `git bisect run sh -c ` + push,
		"bisect run sh -c, more after":       `git bisect run sh -c "git push origin x; true"`,
		"bisect run sh -c, a quote inside":   `git bisect run sh -c 'git push origin x'"'"'s'`,
		"submodule foreach sh -c":            `git submodule foreach sh -c ` + push,
		"submodule foreach -q sh -c":         `git submodule foreach -q sh -c ` + push,
		"submodule foreach --recursive bash": `git submodule foreach --recursive bash -c ` + push,
		"submodule foreach, a line first":    `git submodule foreach 'git push origin x;' y`,
		"env -S, then words":                 `env -S 'sh -c' ` + push,
		"env -S attached, then words":        `env -S"sh -c" ` + push,
		"env --split-string=, then words":    `env --split-string='sh -c' ` + push,
		"env --split-string, then words":     `env --split-string 'sh -c' ` + push,
		"cmd //c sh -c":                      `cmd //c sh -c ` + push,
		"cmd /c bash -c":                     `cmd /c bash -c ` + push,
	}
	for row, line := range rows {
		for set, groups := range ns0ar3Sets(t) {
			want := ""
			if slices.ContainsFunc(groups, func(g Group) bool { return g.Name == "push" && g.On }) {
				want = "push"
			}
			t.Run(row+", "+set, func(t *testing.T) {
				if got := refusedOrNot(line, onFeature, groups); got != want {
					t.Errorf("%q: %q, want %q", line, got, want)
				}
			})
		}
	}
	// A word the guard can't read keeps its mark inside the quotes: refused in every set.
	for _, line := range []string{`git submodule foreach env /usr/bin/g?t push origin x`, `git bisect run env /usr/bin/g?t push origin x`} {
		for set, groups := range ns0ar3Sets(t) {
			if got := refusedOrNot(line, onFeature, groups); got == "" {
				t.Errorf("%s, %q: runs, want refused", set, line)
			}
		}
	}
}

// What N1–N4's fixes must not refuse: git config's reads and plain writes in both forms, plain sections renamed or
// removed, the program names git's own options take, and wrappers' ordinary words.
func TestNS0aR4OrdinaryWorkStillRuns(t *testing.T) {
	onFeature := testRepo(t, "feature", "main", "refs/heads/main", "abc\n")
	for _, line := range []string{
		"git config --global user.name x", "git config core.compression -1", "git config get user.name",
		"git config list", "git config list --show-origin", "git config --list --show-origin", "git config -l",
		"git config user.name", `git config --get-regexp '^alias\.'`, `git config set user.name "A B"`,
		"git config unset user.name", "git config --unset lfs.url", "git config --global --add safe.directory /x",
		"git config --file .gitmodules submodule.x.url ../x", "git config --get-colorbool color.diff",
		"git config --rename-section branch.a branch.b", "git config rename-section branch.a branch.b",
		"git config --remove-section branch.x", "git bisect run go test ./...", "git bisect run make test",
		`git bisect run ./test.sh arg "two words"`, `git submodule foreach --recursive "git status"`,
		"git submodule foreach git status --short", "env -S 'echo hi' there", `cmd /c dir "C:\Program Files"`,
		"git clone --upload-pack=git-upload-pack u d", "git send-email --to=a@b x.patch",
	} {
		for _, gm := range []bool{false, true} {
			groups := WithGates(DefaultGroups())
			if gm {
				groups = ForGM(groups)
			}
			if got := refusedOrNot(line, onFeature, groups); got != "" {
				t.Errorf("gm %v, %q: refused by %q", gm, line, got)
			}
		}
	}
}

// readConfig reads git config as git does (N1): the legacy form's options stop at the key, and the subcommand form
// is the first word's.
func TestNS0aR4ReadConfig(t *testing.T) {
	for _, r := range []struct {
		words  []string
		writes int
		read   bool
	}{
		{[]string{"x.b", "v2", "-l"}, 1, false},
		{[]string{"--get", "x.b"}, 0, true},
		{[]string{"--get", "x.b", "v"}, 0, true},
		{[]string{"-l"}, 0, true},
		{[]string{"-l", "x.b", "v"}, 1, false},
		{[]string{"x.b"}, 0, true},
		{[]string{"set", "x.b", "v"}, 1, false},
		{[]string{"get", "x.b"}, 0, true},
		{[]string{"--global", "set", "x.b", "v"}, 2, false}, // both forms: the subcommand's, and the legacy key "set"
		{[]string{"--rename-section", "a", "b"}, 0, false},
	} {
		c, ok := readConfig(r.words)
		read := !c.edit && len(c.writes) == 0 && len(c.unsets) == 0 && len(c.sections) == 0
		if !ok || len(c.writes) != r.writes || read != r.read {
			t.Errorf("%q: ok %v, %d writes, read %v; want %d writes, read %v", r.words, ok, len(c.writes), read, r.writes, r.read)
		}
	}
}
