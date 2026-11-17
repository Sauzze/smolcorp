package sandbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// FXHOMELINKS on this Mac, with stand-ins only (RT126RESOLVESWEEP S2): the owner's dotfiles kept as links (stow,
// chezmoi, yadm), in a stand-in home folder in the test's temp folder. Seatbelt judges a write or a read through a
// link where the link leads, so the real profile, made by PrepareProfile for a stand-in hire, must deny each home
// place where its links lead as well as where it is spelled: a write through the linked `.zshrc`, `.gitconfig` and
// `.claude` (a folder) must be refused and leave the target as it was, through a chain of two links and a link to a
// link's folder too, and two levels in, a link inside the folder a link leads to (`~/.claude/hooks/pre.sh`,
// `~/.config/git/hooks/pre-commit`: RT126HOMELINKS B1), and every link on the way must stay where it is. A home folder with a link loop on the way to
// one of its places fails the start. Each check is only "ok" or the error's name, and nothing outside the test's temp
// folder is read or written.
func TestOnAMacAHomePlaceKeptAsALinkIsHeldWhereItLeads(t *testing.T) {
	sandboxExec, python := "/usr/bin/sandbox-exec", python3(t)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v) or python3 (%q) missing", err, python)
		}
		t.Skip("no sandbox-exec or python3 here")
	}
	made := t.TempDir()
	base, err := filepath.EvalSymlinks(made)
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "h")
	dots := filepath.Join(home, "dotfiles")
	support := filepath.Join(home, "L", "AD")
	own := filepath.Join(support, "hires", "h1")
	root := filepath.Join(home, "P", "app")
	const was = "as the owner left it"
	targets := map[string]string{
		"zshrc": filepath.Join(dots, "zshrc"), "gitconfig": filepath.Join(dots, "gitconfig"),
		"settings": filepath.Join(dots, "claude", "settings.json"), "transcript": filepath.Join(dots, "claude", "projects", "t.jsonl"),
		"zprofile": filepath.Join(dots, "zprofile"), "bashrc": filepath.Join(dots, "sub", "bashrc"),
		"key": filepath.Join(dots, "keys", "id_x"), "sshconfig": filepath.Join(dots, "ssh_config"),
		"authorized": filepath.Join(dots, "authorized_keys"), "claudejson": filepath.Join(dots, "claude.json"),
		"gitcfg": filepath.Join(dots, "git-config"), "account": filepath.Join(dots, "claude-x", "settings.json"),
		"hook": filepath.Join(dots, "scripts", "pre.sh"), "githook": filepath.Join(dots, "bin", "pc"),
	}
	for _, at := range append(mapValues(targets), filepath.Join(home, ".claude", "CLAUDE.md"), filepath.Join(root, "main.go"),
		filepath.Join(own, "BRIEF.md")) {
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(was), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(home, ".claude", "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(home, ".claude")); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Join(home, ".ssh"), filepath.Join(home, ".config", "git"), filepath.Join(home, "chain"),
		filepath.Join(dots, "hooks"), filepath.Join(dots, "githooks")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	links := [][2]string{
		{".zshrc", "dotfiles/zshrc"}, {".gitconfig", "dotfiles/gitconfig"}, {".claude", "dotfiles/claude"},
		// A chain of two links; a link into a folder that is itself a link; a link to a place not there yet.
		{".zprofile", "chain/zprofile"}, {"chain/zprofile", "../dotfiles/zprofile"},
		{".bashrc", "cfg/bashrc"}, {"cfg", "dotfiles/sub"}, {".zshenv", "dotfiles/gone/zshenv"},
		// Links among the names a row holds by a pattern, and in a folder a row holds whole.
		{".ssh/id_x", "../dotfiles/keys/id_x"}, {".ssh/config", "../dotfiles/ssh_config"},
		{".ssh/authorized_keys", "../dotfiles/authorized_keys"}, {".claude.json", "dotfiles/claude.json"},
		{".config/git/config", "../../dotfiles/git-config"},
		// Two levels in (B1): a link inside the folder a link in a held folder leads to.
		{"dotfiles/claude/hooks", "../hooks"}, {"dotfiles/hooks/pre.sh", "../scripts/pre.sh"},
		{".config/git/hooks", "../../dotfiles/githooks"}, {"dotfiles/githooks/pre-commit", "../bin/pc"},
		// An extra account folder that is a link (S2b), given as the core spells it: the link.
		{".claude-x", "dotfiles/claude-x"},
	}
	for _, l := range links {
		if err := os.Symlink(l[1], filepath.Join(home, l[0])); err != nil {
			t.Fatal(err)
		}
	}
	places := Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: root, RemoteLogin: true,
		Accounts: []string{filepath.Join(home, ".claude-x")}}
	p := mustNew(t, places)
	file := filepath.Join(own, "sandbox.sb")
	if err := PrepareProfile(p, file); err != nil {
		t.Fatal(err)
	}

	type step struct {
		what, at, to, want string
	}
	const ok, no = "ok", "PermissionError"
	steps := []step{
		// Written through each link: refused, the target as it was.
		{"write", home + "/.zshrc", "", no}, {"write", home + "/.gitconfig", "", no}, {"write", home + "/.claude/settings.json", "", no},
		{"write", home + "/.zprofile", "", no}, {"write", home + "/.bashrc", "", no}, {"write", home + "/.ssh/authorized_keys", "", no},
		{"write", home + "/.config/git/config", "", no}, {"write", home + "/.claude-x/settings.json", "", no},
		// …and where they lead, spelled there.
		{"write", dots + "/zshrc", "", no}, {"write", dots + "/claude/new.md", "", no}, {"write", dots + "/sub/bashrc", "", no},
		// Two levels in: the hooks the owner's Claude and git run, through both links and where they lead.
		{"write", home + "/.claude/hooks/pre.sh", "", no}, {"write", dots + "/scripts/pre.sh", "", no},
		{"write", home + "/.config/git/hooks/pre-commit", "", no}, {"write", dots + "/bin/pc", "", no},
		{"unlink", dots + "/hooks/pre.sh", "", no}, {"unlink", dots + "/githooks/pre-commit", "", no},
		// Read through a link: the transcripts, a key, `.claude.json`; not the ssh config, nor the account folder's notes.
		{"read", home + "/.claude/projects/t.jsonl", "", no}, {"read", home + "/.ssh/id_x", "", no},
		{"read", home + "/.claude.json", "", no}, {"read", home + "/.ssh/config", "", ok}, {"read", home + "/.claude/settings.json", "", ok},
		// The rest of the dotfiles folder is the desk's to write, as any folder of the owner's no row names.
		{"write", dots + "/notes", "", ok}, {"write", home + "/.zshrc-notes", "", ok}, {"write", root + "/main.go", "", ok},
		{"write", dots + "/scripts/notes", "", ok}, {"write", dots + "/bin/notes", "", ok},
		// A place a link leads to that isn't there yet can't be made.
		{"mkdir", dots + "/gone", "", no},
		// Every link on the way stays where it is: not removed, not moved, not made again elsewhere.
		{"unlink", home + "/.zshrc", "", no}, {"unlink", home + "/chain/zprofile", "", no}, {"unlink", home + "/cfg", "", no},
		{"rename", home + "/.claude", home + "/claude-old", no}, {"unlink", home + "/.claude-x", "", no},
		{"rename", dots + "/sub", dots + "/sub-old", no}, {"unlink", home + "/.ssh/id_x", "", no},
	}
	var args [][]string
	for _, s := range steps {
		arg := []string{s.what, s.at}
		if s.to != "" {
			arg = append(arg, s.to)
		}
		args = append(args, arg)
	}
	encoded, _ := json.Marshal(args)
	cmd := disclaim.Cmd(exec.Command(sandboxExec, "-f", file, python, "-I", "-c", sbProbe, string(encoded)))
	cmd.Dir, cmd.Env = base, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("the stand-in didn't run under the profile: %v %s", err, stderr.Bytes())
	}
	var said []string
	if err := json.Unmarshal(out.Bytes(), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %q (%v)", out.Bytes(), err)
	}
	for i, s := range steps {
		if said[i] != s.want {
			t.Errorf("%s %s %s: %s, want %s", s.what, strings.TrimPrefix(s.at, base), strings.TrimPrefix(s.to, base), said[i], s.want)
		}
	}
	for name, at := range targets {
		if data, err := os.ReadFile(at); err != nil || string(data) != was {
			t.Errorf("the target of %s, %s, changed: %q %v", name, strings.TrimPrefix(at, base), data, err)
		}
	}

	// A link loop on the way to a home place fails the start: nothing can say where a write through it lands.
	loop := filepath.Join(base, "loop")
	if err := os.MkdirAll(loop, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, l := range [][2]string{{".profile", "a"}, {"a", "b"}, {"b", ".profile"}} {
		if err := os.Symlink(l[1], filepath.Join(loop, l[0])); err != nil {
			t.Fatal(err)
		}
	}
	looped := places
	looped.Home, looped.Support, looped.Root, looped.Accounts = loop, filepath.Join(loop, "L", "AD"), filepath.Join(loop, "P", "app"), nil
	if _, err := New(looped); !errors.Is(err, ErrHomeLinks) {
		t.Errorf("New with a link loop at the home folder's .profile: %v, want %v", err, ErrHomeLinks)
	}
}

func mapValues(m map[string]string) []string {
	var values []string
	for _, v := range m {
		values = append(values, v)
	}
	return values
}
