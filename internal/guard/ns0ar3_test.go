package guard

import "testing"

// RT126NS0aR3's blocking findings B1 and B2, each a named row in every group set: git runs text the guard never read
// (submodule--helper foreach, instaweb -d, a subcommand it doesn't know), or a git config write stores a command a
// later plain git command runs (a submodule's update, imap-send's tunnel, git-lfs's and instaweb's keys).

// ns0ar3Sets are the group sets each row is pinned in: the review's, and the publish group alone, where only the
// line git runs is read.
func ns0ar3Sets(t *testing.T) map[string][]Group {
	only := func(name string) []Group {
		for _, g := range WithGates(DefaultGroups()) {
			if g.Name == name {
				return []Group{g}
			}
		}
		t.Fatalf("no %s group", name)
		return nil
	}
	return map[string][]Group{
		"push alone": only("push"), "deploy alone": only("deploy"), "merge alone": only(MergeGroup),
		"publish alone": only("publish"), "the defaults": DefaultGroups(), "with the gates": WithGates(DefaultGroups()),
		"the GM's": ForGM(WithGates(DefaultGroups())), "none": nil,
	}
}

// B1: a git subcommand the guard doesn't know, or git's internal one, is refused by the core while the push, merge
// or deploy group is on; with none of them, what it runs from the line is still read (submodule--helper foreach's
// line, instaweb -d's).
func TestNS0aR3GitSubcommandsTheGuardDoesntKnow(t *testing.T) {
	onFeature := testRepo(t, "feature", "main", "refs/heads/main", "abc\n")
	// row: line, and its verdict where neither push, merge nor deploy is on (publish alone, none).
	rows := map[string]struct{ line, publish, none string }{
		"submodule--helper foreach, stand-in":   {`git submodule--helper foreach ` + stand, "", ""},
		"submodule--helper foreach, a push":     {`git submodule--helper foreach ` + push, "", ""},
		"submodule--helper foreach --recursive": {`git submodule--helper foreach --recursive "cd .. && git push origin x"`, "", ""},
		"submodule--helper foreach, a publish":  {`git submodule--helper foreach "gh pr create"`, "publish", ""},
		"git-submodule--helper by its program":  {`git-submodule--helper foreach ` + push, "", ""},
		"instaweb --httpd, a push":              {`git instaweb --httpd=` + push + ` --browser=lynx`, "", ""},
		"instaweb -d, stand-in":                 {`git instaweb -d ` + stand, "", ""},
		"instaweb -d, a publish":                {`git instaweb -d "gh pr create"`, "publish", ""},
		"instaweb --h, a publish":               {`git instaweb --h "gh pr create"`, "publish", ""},
		"another --helper":                      {`git bisect--helper x`, "", ""},
		"web--browse":                           {`git web--browse x`, "", ""},
		"for-each-repo, a push":                 {`git for-each-repo --config=x push origin x`, "", ""},
		"hook run":                              {`git hook run pre-push`, "", ""},
		"an alias":                              {`git p origin x`, "", ""},
		"a subcommand through a variable":       {`git ${EMPTY}push origin x`, "", ""},
		"an extension":                          {`git frobnicate main`, "", ""},
	}
	for row, r := range rows {
		for set, groups := range ns0ar3Sets(t) {
			want := "guard"
			switch set {
			case "publish alone":
				want = r.publish
			case "none":
				want = r.none
			}
			t.Run(row+", "+set, func(t *testing.T) {
				if got := refusedOrNot(r.line, onFeature, groups); got != want {
					t.Errorf("%q: %q, want %q", r.line, got, want)
				}
			})
		}
	}
}

// B2: a git config write of a key git or its helpers run, or of a value that starts with ! or holds a shell sign,
// is refused by the core in every group set.
func TestNS0aR3GitConfigWritesOfWhatGitRuns(t *testing.T) {
	onFeature := testRepo(t, "feature", "main", "refs/heads/main", "abc\n")
	rows := map[string]string{
		"submodule update !, stand-in":     `git config submodule.sub.update "!touch ran; false"`,
		"submodule update !, a push":       `git config submodule.sub.update "!git push origin x"`,
		"submodule update ! after a blank": `git config submodule.sub.update " !git push origin x"`,
		"imap.tunnel, stand-in":            `git config imap.tunnel "touch ran"`,
		"imap.tunnel, a push":              `git config imap.tunnel ` + push,
		"imap.tunnel by set":               `git config set imap.tunnel x`,
		"imap.tunnel by --add":             `git config --add imap.tunnel x`,
		"lfs customtransfer path":          `git config lfs.customtransfer.x.path ` + push,
		"lfs customtransfer args":          `git config lfs.customtransfer.x.args " -c touch\ ran"`,
		"lfs standalonetransferagent":      `git config lfs.standalonetransferagent x`,
		"lfs extension clean":              `git config lfs.extension.x.clean ` + push,
		"lfs extension smudge":             `git config lfs.extension.x.smudge ` + push,
		"lfs, any key":                     `git config lfs.url x`,
		"instaweb.httpd":                   `git config instaweb.httpd ` + push,
		"instaweb, any key":                `git config instaweb.browser x`,
		"a key ending in tunnel":           `git config x.y.tunnel t`,
		"a key ending in httpd":            `git config x.httpd t`,
		"a key ending in clean":            `git config x.y.clean t`,
		"a key ending in smudge":           `git config x.y.smudge t`,
		"a key ending in process":          `git config x.y.process t`,
		"a key ending in path":             `git config x.y.path t`,
		"a key ending in args":             `git config x.y.args t`,
		"a key ending in command":          `git config x.y.command t`,
		"a key ending in cmd":              `git config x.y.cmd t`,
		"a key ending in helper":           `git config x.y.helper t`,
		"a value starting with !":          `git config x.y "!t"`,
		"a value with ;":                   `git config x.y "a; t"`,
		"a value with &":                   `git config x.y "a && t"`,
		"a value with |":                   `git config x.y "a | t"`,
		"a value with a backquote":         "git config x.y 'a `t`'",
		"a value with $(":                  `git config x.y 'a $(t)'`,
		"a value with >":                   `git config x.y "a > f"`,
		"a value with <":                   `git config x.y "a < f"`,
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

// What B1 and B2 must not refuse: ordinary submodule, lfs and config work, and git's own subcommands.
func TestNS0aR3OrdinaryWorkStillRuns(t *testing.T) {
	onFeature := testRepo(t, "feature", "main", "refs/heads/main", "abc\n")
	for _, line := range []string{
		"git submodule update --init", "git submodule foreach git status", "git submodule status", "git lfs pull",
		"git lfs track '*.psd'", "git config submodule.sub.update rebase", "git config submodule.sub.url ../sub",
		"git config pull.rebase true", `git config user.name "A B"`, "git config core.autocrlf input",
		"git config --get imap.tunnel", "git config --unset lfs.url", "git status", "git log --oneline -3",
		"git help push", "git version", "git --version", "git -P show --stat", "git range-diff a...b",
		"git request-pull v1 origin", "git stage f", "git --no-pager diff",
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
