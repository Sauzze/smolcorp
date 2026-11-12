package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The dry run's team (smolcorp demo --dry-run): stand-ins for the office and for Claude Code,
// for the tests and for a look before spending anything. The same steps as the real demo, in
// the same folder layout: a plan, one hire per task in a worktree of its own on its own branch
// (beside the project, as the office makes them), a change and its proof, a review, a report,
// and a stop before anything ships. Each hire goes through the demo's fence first, and the
// fence is shown refusing a hire outside the folder, a push, and a file written outside it.
// Its changes are scripted, and it runs no tests: a real hire's proof is the tests passing.

type standInTeam struct{}

// demoStandInRoles are the default roles on Claude Code that the stand-ins hire.
var demoStandInRoles = []string{"bughunter", "tester", "docs", "reviewer"}

// demoStandInTask is one task of TASKS.md as a stand-in does it: who, and its scripted change.
type demoStandInTask struct {
	role, name, title string
	change            func(worktree string) error
}

var demoStandInTasks = []demoStandInTask{
	{"bughunter", "Bug hunter", "Make the shares add up to the bill", func(tree string) error {
		if err := replaceInFile(filepath.Join(tree, "bill.py"), "    share = with_tip // people\n    return [share] * people\n",
			"    share, left = divmod(with_tip, people)\n    return [share + 1 if person < left else share for person in range(people)]\n"); err != nil {
			return err
		}
		return replaceInFile(filepath.Join(tree, "test_bill.py"), "\n\nclass FormatTests",
			"\n    def test_the_shares_add_up_to_the_bill(self):\n        self.assertEqual(split(10000, 3), [3334, 3333, 3333])\n\n\nclass FormatTests")
	}},
	{"tester", "Tester", "Test parse_amount", func(tree string) error {
		return writeNewFile(filepath.Join(tree, "test_parse_amount.py"), []byte(`import unittest

from bill import parse_amount


class ParseAmountTests(unittest.TestCase):
    def test_the_ways_people_type_an_amount(self):
        for text, cents in [("12", 1200), ("12.5", 1250), ("$12.50", 1250), (" 12.50 ", 1250)]:
            with self.subTest(text=text):
                self.assertEqual(parse_amount(text), cents)


if __name__ == "__main__":
    unittest.main()
`), 0o644)
	}},
	{"docs", "Docs writer", "Say how to use it in the README", func(tree string) error {
		return replaceInFile(filepath.Join(tree, "README.md"), "\nRun the tests with",
			"\n## Usage\n\n    $ python3 bill.py 60 --people 3 --tip 10\n    Person 1: $22.00\n    Person 2: $22.00\n    Person 3: $22.00\n"+
				"    Total:    $66.00\n\n    $ python3 bill.py 12.50\n    Person 1: $6.25\n    Person 2: $6.25\n    Total:    $12.50\n\nRun the tests with")
	}},
}

// replaceInFile replaces the one place old is in a file with new.
func replaceInFile(path, old, new string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Count(string(data), old) != 1 {
		return fmt.Errorf("%s has changed: the stand-in can't find its place", filepath.Base(path))
	}
	return os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0o644)
}

func (standInTeam) ready(out io.Writer) ([]string, error) {
	fmt.Fprintln(out, "(No office is asked and no agent is started: the stand-ins are scripted.)")
	return demoStandInRoles, nil
}

func (standInTeam) off() []demoSwitch    { return nil }
func (standInTeam) leftOn() []demoSwitch { return nil }

func (standInTeam) work(ctx context.Context, f demoFolder, roles []string, out io.Writer) error {
	fence := demoFence{root: f.root, roles: roles, home: homeDir}
	say := func(who, format string, a ...any) { fmt.Fprintf(out, "%-14s %s\n", who+":", fmt.Sprintf(format, a...)) }
	plan := "# The plan (stand-in General Manager)\n\n"
	for i, t := range demoStandInTasks {
		plan += fmt.Sprintf("%d. %s: a %s, on its own branch. Proof: `python3 -m unittest` passes.\n", i+1, t.title, t.name)
	}
	plan += fmt.Sprintf("%d. A reviewer reviews each branch. Nothing is merged, pushed or published.\n", len(demoStandInTasks)+1)
	if err := writeNewFile(filepath.Join(f.root, "PLAN.md"), []byte(plan), 0o644); err != nil {
		return err
	}
	say("GM", "read TASKS.md and wrote the plan: %s", filepath.Join(f.root, "PLAN.md"))
	say("GM", "plan approved (in the real demo, you approve it in the GM's terminal)")
	var report []string
	hiresFolder := filepath.Join(f.root, demoRepoName+"-hires")
	for i, t := range demoStandInTasks {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		hire := fmt.Sprintf("smolcorp gm hire --role %s --folder %s --worktree --task %s", t.role, shellPath(f.repo), shellPath(t.title))
		if why := fence.command(hire, f.repo); why != "" {
			return fmt.Errorf("the stand-in GM's hire was refused: %s", why)
		}
		name := fmt.Sprintf("%s-%d", t.role, i+1)
		tree, branch := filepath.Join(hiresFolder, name), "hire/"+name
		if _, err := demoGit(f.repo, "worktree", "add", "-q", "-b", branch, tree); err != nil {
			return err
		}
		say("GM", "hired a %s: %s, in %s", t.name, branch, tree)
		if err := t.change(tree); err != nil {
			return err
		}
		if _, err := demoGit(tree, "add", "-A"); err != nil {
			return err
		}
		if _, err := demoGit(tree, "commit", "-q", "-m", t.title); err != nil {
			return err
		}
		stat, err := demoGit(tree, "diff", "--shortstat", "main...HEAD")
		if err != nil {
			return err
		}
		say(name, "%s: done (stand-in work)", t.title)
		say("GM", "checked %s's proof: %s (a real hire's tests run; a stand-in's don't)", name, strings.TrimSpace(stat))
		report = append(report, fmt.Sprintf("- %s: %s, %s. Review: approved (stand-in).", t.title, branch, strings.TrimSpace(stat)))
	}
	review := fmt.Sprintf("smolcorp gm hire --role reviewer --folder %s --task %s", shellPath(f.repo), shellPath("Review each hire/ branch"))
	if why := fence.command(review, f.repo); why != "" {
		return fmt.Errorf("the stand-in GM's hire was refused: %s", why)
	}
	for i, t := range demoStandInTasks {
		say("reviewer", "reviewed hire/%s-%d: approved (stand-in)", t.role, i+1)
	}
	// The fence at work: each of these must be refused.
	outside, _ := json.Marshal(map[string]string{"file_path": filepath.Join(filepath.Dir(f.root), "outside.txt")})
	for _, try := range []struct {
		shown string
		call  demoCall
	}{
		{"a hire into your home folder", bashCall("smolcorp gm hire --role tester --folder ~ --worktree", f.repo)},
		{"a push", bashCall("git push origin hire/bughunter-1", f.repo)},
		{"a file written outside the demo", demoCall{ToolName: "Write", ToolInput: outside, Cwd: f.repo}},
	} {
		why := fence.check(try.call)
		if why == "" {
			return fmt.Errorf("the demo's fence let %s through", try.shown)
		}
		say("fence", "refused %s: %s", try.shown, strings.TrimPrefix(why, demoFenceRefusal))
	}
	text := "# Demo report (dry run, stand-ins)\n\n" + strings.Join(report, "\n") +
		"\n\nWaiting for your OK: nothing was merged, pushed or published.\n"
	if err := writeNewFile(f.report(), []byte(text), 0o644); err != nil {
		return err
	}
	say("GM", "wrote %s and stopped before anything ships", f.report())
	return nil
}

func bashCall(command, cwd string) demoCall {
	input, _ := json.Marshal(map[string]string{"command": command})
	return demoCall{ToolName: "Bash", ToolInput: input, Cwd: cwd}
}
