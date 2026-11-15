//go:build !windows

package desktop

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// X8SKILLS1 (SIERRA, 15 Nov, from RT126SAM5 F2): a desk's skills and plugins are open to its Read through its own
// Claude folder, and nothing else there is. Stand-in accounts in the test's own home; no Claude Code runs. The rules are
// judged by the repo's model of Claude Code's matcher (readDenied); deskClaudeOthers' patterns were also run through
// Claude Code 2.1.325's own ignore code, in node and in bun, for the report.

// deskSettingsDeny is the deny list of the settings hire hireID started with, and its Edit rules as Read rules, so
// readDenied judges them.
func deskSettingsDeny(t *testing.T, hireID string) (deny, edits []string) {
	t.Helper()
	var settings struct {
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(startedSettings(t, hireID)), &settings); err != nil {
		t.Fatal(err)
	}
	for _, rule := range settings.Permissions.Deny {
		if pattern, ok := strings.CutPrefix(rule, "Edit("); ok {
			edits = append(edits, "Read("+pattern)
		}
	}
	return settings.Permissions.Deny, edits
}

// The rows: a desk reads its own skill's references/x.md, and a plugin skill's, through its folder's links, which its
// settings allow, both as it names the file and as the link leads. It still can't Read or Edit its own transcripts,
// prompt history, todos, paste cache or shell snapshots, nor those of a desk started after it or of the GM; nor any
// other name there, one a later Claude Code may make included. Edit stays denied in the skills and plugins too.
func TestADeskReadsItsSkillsFilesThroughItsClaudeFolderAndNothingElseThere(t *testing.T) {
	r := newEngineRig(t)
	macDeskLinks(t)
	account, _ := deskAccount(t, "")
	skill := filepath.Join("skills", "s", "references", "x.md")
	plugin := filepath.Join("plugins", "cache", "market", "tool", "1.0.0", "skills", "q", "references", "y.md")
	for _, file := range []string{skill, plugin} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(account, file)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(account, file), []byte("what the skill reads"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	first, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	later, err := runManagedHire(r.request("tester"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	deny, edits := deskSettingsDeny(t, first.ID)
	own := deskClaudeFolder(supportDir, first.ID)

	for _, file := range []string{skill, plugin} {
		path := filepath.Join(own, file)
		if text, err := os.ReadFile(path); err != nil || string(text) != "what the skill reads" {
			t.Fatalf("%s isn't the account's through the desk's link: %q %v", path, text, err)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if real, _ := filepath.EvalSymlinks(account); err != nil || !strings.HasPrefix(resolved, real+string(filepath.Separator)) {
			t.Fatalf("%s leads to %q (%v), not into the account", path, resolved, err)
		}
		// Claude Code judges each spelling of a path: as named, and as its links lead.
		for _, spelling := range []string{path, resolved, filepath.Join(own, filepath.Dir(file))} {
			if readDenied(deny, r.home, spelling) {
				t.Errorf("the desk can't Read %s", spelling)
			}
		}
		if !readDenied(edits, r.home, path) {
			t.Errorf("the desk can Edit %s through its folder", path)
		}
	}

	stores := []string{"projects/-work-api/s1.jsonl", "projects/-work-api/memory/MEMORY.md", "history.jsonl",
		"todos/s1-agent-s1.json", "paste-cache/5f3a.txt", "shell-snapshots/snapshot-zsh-1-x.sh", ".claude.json",
		"settings.json", "CLAUDE.md", "file-history/s1/a@v1", "sessions/s1.json", "image-cache/s1/1.png", "statsig/x",
		"plans/calm-river.md", "a-later-store/x", "skill/x.md", "skills2/x.md", "skills.json", "skillsx",
		"plugin/x", "plugins-old/x", "s", "p/x", ".skills/x", "x"}
	for _, desk := range []string{first.ID, later.ID, "gm0000000001"} {
		for _, store := range stores {
			at := filepath.Join(deskClaudeFolder(supportDir, desk), filepath.FromSlash(store))
			if !readDenied(deny, r.home, at) || !readDenied(edits, r.home, at) {
				t.Errorf("%s isn't denied to the first desk's Read and Edit", at)
			}
		}
	}
}

// Wherever the support folder is: in the home folder, the rules from ~ and in full (RT126SAX8R2 R2-b), each form
// alone opening the skills and plugins and nothing else; outside it, the short form.
func TestASkillsFilesAreOpenWhereverTheSupportFolderIs(t *testing.T) {
	saved := homeDir
	t.Cleanup(func() { homeDir = saved })
	homeDir = "/Users/owner"
	for _, support := range []string{"/Users/owner/Library/Application Support/AgentDeck", "/Volumes/other/smolcorp/support"} {
		rules := hireDenyRules("abc123", nil, support, nil, nil, nil, "darwin")
		// The rules of the desks' Claude folders, by how they name the support folder.
		forms := map[string][]string{}
		for _, rule := range rules {
			if form, _, ok := strings.Cut(rule, "/hires/*/"+deskClaudeName+"/"); ok && strings.HasPrefix(rule, "Read(") {
				forms[form] = append(forms[form], rule)
			}
		}
		want := []string{"Read(//**/smolcorp/support"}
		if strings.HasPrefix(support, homeDir+"/") {
			want = []string{"Read(//Users/owner/Library/Application Support/AgentDeck", "Read(~/Library/Application Support/AgentDeck"}
		}
		if got := slices.Sorted(maps.Keys(forms)); !slices.Equal(got, want) {
			t.Errorf("%s: the forms %q, want %q", support, got, want)
		}
		forms["all"] = rules
		for name, form := range forms {
			if len(form) == 0 {
				continue
			}
			for _, desk := range []string{"abc123", "def456", "gm0000000002"} {
				folder := filepath.Join(support, "hires", desk, deskClaudeName)
				for _, open := range []string{"skills/s/references/x.md", "plugins/cache/m/t/1/skills/q/SKILL.md"} {
					if readDenied(form, homeDir, filepath.Join(folder, open)) {
						t.Errorf("%s, %s: %s/%s is denied", support, name, desk, open)
					}
				}
				for _, store := range []string{"projects/-w/s1.jsonl", "history.jsonl", "todos/t.json", "paste-cache/p.txt", "shell-snapshots/s.sh"} {
					if !readDenied(form, homeDir, filepath.Join(folder, store)) {
						t.Errorf("%s, %s: %s/%s is open", support, name, desk, store)
					}
				}
			}
		}
	}
}

// namesOtherThan's patterns match every name but the ones kept, and never those, for every name of up to three
// characters from letters near the kept names' own, both cases, a dot, a dash, a control character and others far up,
// and for each kept name, every start of it, and every one-character change, addition and removal. Their ranges hold
// both cases of a letter or neither (Claude Code's matcher ignores case), never "/", and only characters Claude Code's
// matcher reads as themselves.
func TestNamesOtherThanMatchEveryNameButTheKeptOnes(t *testing.T) {
	var rules []string
	for _, pattern := range deskClaudeOthers {
		rules = append(rules, "Read(//d/"+pattern+")", "Read(//d/"+pattern+"/**)")
	}
	alphabet := []string{"s", "k", "i", "l", "p", "u", "g", "n", "x", "a", "z", "S", "K", "P", "Z", ".", "-", "_", "0", " ", "\x01", "é", "￿"}
	names := map[string]bool{}
	var grow func(prefix string, left int)
	grow = func(prefix string, left int) {
		if prefix != "" {
			names[prefix] = true
		}
		if left > 0 {
			for _, letter := range alphabet {
				grow(prefix+letter, left-1)
			}
		}
	}
	grow("", 3)
	for _, kept := range deskClaudeReadable {
		for i := 0; i <= len(kept); i++ {
			names[kept[:i]] = true
			if i < len(kept) {
				names[kept[:i]+kept[i+1:]] = true
			}
			for _, letter := range alphabet {
				names[kept[:i]+letter+kept[i:]] = true
				if i < len(kept) {
					names[kept[:i]+letter+kept[i+1:]] = true
				}
			}
		}
	}
	delete(names, "")
	// Claude Code's matcher ignores case, the model doesn't: with ranges that hold both cases or neither and patterns in
	// lower case, Claude Code judges a name as the model judges it in lower case (ASCII's alone: JavaScript's ignoring
	// of case never takes a letter beyond ASCII to one in it). So SKILLS is open too, as Claude Code has it.
	lower := func(name string) string {
		return strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' {
				return r - 'A' + 'a'
			}
			return r
		}, name)
	}
	checked := 0
	for name := range names {
		kept := slices.Contains(deskClaudeReadable, lower(name))
		for _, below := range []string{"", "/references/x.md"} {
			checked++
			if denied := readDenied(rules, "/home", "/d/"+lower(name)+below); denied == kept {
				t.Errorf("%q: denied %v", name+below, denied)
			}
		}
	}
	if checked < 20000 {
		t.Errorf("only %d names checked", checked)
	}

	if len(deskClaudeOthers) != 25 {
		t.Errorf("%d patterns, want 25: %q", len(deskClaudeOthers), deskClaudeOthers)
	}
	for _, pattern := range deskClaudeOthers {
		_, inside, ok := strings.Cut(pattern, "[")
		if !ok {
			continue
		}
		inside, _, _ = strings.Cut(inside, "]")
		in := map[rune]bool{}
		for i, ranges := 0, []rune(inside); i < len(ranges); i++ {
			from, to := ranges[i], ranges[i]
			if i+2 < len(ranges) && ranges[i+1] == '-' {
				to, i = ranges[i+2], i+2
			}
			if from > to {
				t.Errorf("%q: range %q-%q backwards", pattern, from, to)
			}
			for _, end := range []rune{from, to} {
				if !(end >= 'a' && end <= 'z' || end >= 'A' && end <= 'Z' || end >= '0' && end <= '9' || end == '.' || end == 1 || end == 0xffff) {
					t.Errorf("%q: %q is read by Claude Code's matcher as more than itself", pattern, end)
				}
			}
			for r := from; r <= to && r < 0x80; r++ {
				in[r] = true
			}
		}
		if in['/'] || in[0] {
			t.Errorf("%q holds / or U+0000", pattern)
		}
		for letter := 'a'; letter <= 'z'; letter++ {
			if in[letter] != in[letter-'a'+'A'] {
				t.Errorf("%q holds %q and not %q, or the other way", pattern, letter, letter-'a'+'A')
			}
		}
	}
}
