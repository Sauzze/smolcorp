package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// GITCFG1 (SIERRA, 14 Nov): the places the owner's git config names, for git to run or to read as more config, read
// at a desk's start so the Policy can deny writing them (Places.GitHooks, Places.GitNamed). A desk that wrote one
// would have the owner's next git run its code: a hook in core.hooksPath's folder (husky's `.husky/_`, git-ignored and
// in the working tree), a file an include.path or includeIf names (more config: hooks, an fsmonitor, an ssh command),
// core.fsmonitor's program, or a file core.sshCommand runs or hands ssh (its program, an `-F` config with a
// ProxyCommand). The reader fails closed: a value it can't be sure of fails the desk's start.

// ErrGitConfig is a value in the owner's git config whose place the reader can't be sure of: a `~user` or
// `%(prefix)` path, a relative include that isn't from a file, or a command the shell would expand (`$`, a glob, an
// operator), unbalanced quotes, or too many included files.
var ErrGitConfig = errors.New("sandbox: the owner's git config names a place that can't be told")

// ExcludesFileError is ErrGitConfig for an ignore file (core.excludesFile) spelled in a way the reader can't tell
// the place of: from another account's home folder (`~name/…`) or from where git is installed (`%(prefix)/…`).
// Value is the setting as written, for words that name it and say how to spell it instead (FXAGENTHOMEMACR2,
// RT126AGENTHOMEMACR1 O1).
type ExcludesFileError struct{ Value string }

func (e *ExcludesFileError) Error() string {
	return fmt.Sprintf("%v: core.excludesFile is %q", ErrGitConfig, e.Value)
}

func (e *ExcludesFileError) Unwrap() error { return ErrGitConfig }

// maxGitConfigFiles is how many config files ReadGitConfig reads at most: git's own includes stop at a depth of 10.
const maxGitConfigFiles = 64

// maxHookEntries is how many entries ReadGitConfig looks at in the hooks folders, for links (RT126GITCFG1 B2): a hooks
// folder holds a handful, and one holding more fails the start rather than be half read.
const maxHookEntries = 1000

// ReadGitConfig is what the owner's git config names for the desk at pl, for pl.GitHooks and pl.GitNamed: hooks are
// core.hooksPath's folders, and named the files git reads as config or runs (include.path and every includeIf's path,
// whatever its condition; core.fsmonitor's and core.sshCommand's files), and core.excludesFile's ignore file, which the
// core's reads take too. git is the program the core resolved, run with env, the environment the owner's git runs
// with. M1 calls it at the desk's start, before New, and doesn't start the desk when it fails.
//
// It reads every file the owner's git reads for this repo: the system's, the global ones and the command line's as git
// finds them (`git config --list`), and the repo's own as files (its config and config.worktree, and the desk's
// registration's config.worktree), each of pl.GitConfigs, and then every file an include names, its condition left
// aside, the includes of those too. A file that isn't there is still named: a desk could make it. Each place is given
// as written, with `~` as the home git expands, and with every link in it followed, as far as it exists: the place the
// kernel checks. A relative core.hooksPath, and a command's relative files, are taken in each of the desk's root and
// the owner's checkout, where git runs them. /dev/null is never named.
//
// Husky's hooks folder `_` runs the hook of the same name in the folder above it, so that folder is a hooks folder too,
// and husky's init.sh is named (RT126GITCFG1 B1). A link in a hooks folder, at any depth, has its target named, a
// folder as a hooks folder: git runs a hook through its link (B2). Every registration's config.worktree is read, as the
// owner's git reads it in that worktree (N2).
func ReadGitConfig(ctx context.Context, git string, env []string, pl Places) (hooks, named []string, err error) {
	r := &gitConfigReader{ctx: ctx, git: git, env: env, homes: []string{pl.Home}, seen: map[string]bool{}}
	// git runs at the top of the volume, where no repo is: one a desk made in a folder above would only fail the start.
	r.dir = filepath.VolumeName(pl.Home) + string(filepath.Separator)
	given := env
	if given == nil {
		given = os.Environ()
	}
	var gotHome, gotXDG bool
	for i := len(given) - 1; i >= 0; i-- {
		if home, ok := strings.CutPrefix(given[i], "HOME="); ok && !gotHome {
			if home != pl.Home && home != "" {
				r.homes = append(r.homes, home)
			}
			gotHome = true
		}
		if xdg, ok := strings.CutPrefix(given[i], "XDG_CONFIG_HOME="); ok && !gotXDG {
			r.xdg, gotXDG = xdg, true
		}
	}
	for _, root := range []string{pl.Root, pl.Checkout} {
		if root != "" {
			r.roots = append(r.roots, root)
		}
	}
	// The system's, the global and the command line's files, as git finds them. The repo's own are read as files below:
	// git gives a checkout's own config relative to where it runs.
	if err := r.read("", true); err != nil {
		return nil, nil, err
	}
	queue := slices.Clone(pl.GitConfigs)
	if pl.GitDir != "" {
		queue = append(queue, filepath.Join(pl.GitDir, "config"), filepath.Join(pl.GitDir, "config.worktree"))
		// Every registration's, the desk's own and other desks': the owner's git reads each in its worktree (N2).
		registered := slices.Clone(pl.OtherWorktrees)
		switch entries, err := os.ReadDir(filepath.Join(pl.GitDir, "worktrees")); {
		case err == nil:
			for _, e := range entries {
				registered = append(registered, e.Name())
			}
		case !missing(err):
			return nil, nil, err
		}
		if pl.OwnWorktree != "" {
			registered = append(registered, pl.OwnWorktree)
		}
		slices.Sort(registered)
		for _, name := range slices.Compact(registered) {
			queue = append(queue, filepath.Join(pl.GitDir, "worktrees", name, "config.worktree"))
		}
	}
	for len(queue) > 0 || len(r.includes) > 0 {
		queue, r.includes = append(queue, r.includes...), nil
		file := queue[0]
		queue = queue[1:]
		if r.seen[file] {
			continue
		}
		r.seen[file] = true
		switch _, err := os.Stat(file); {
		case missing(err):
			continue
		case err != nil:
			return nil, nil, err
		}
		if len(r.seen) > maxGitConfigFiles {
			return nil, nil, fmt.Errorf("%w: more than %d files", ErrGitConfig, maxGitConfigFiles)
		}
		if err := r.read(file, false); err != nil {
			return nil, nil, err
		}
	}
	if err := r.followHookLinks(); err != nil {
		return nil, nil, err
	}
	slices.Sort(r.hooks)
	slices.Sort(r.named)
	return slices.Compact(r.hooks), slices.Compact(r.named), nil
}

type gitConfigReader struct {
	ctx                    context.Context
	git, dir, xdg          string
	env, homes, roots      []string
	seen                   map[string]bool
	includes, hooks, named []string
}

// read runs `git config --list` on file, or on every file git finds when file is "", its includes left to the
// reader, and takes what each value names. With skipRepo, the values of the repo's own files are left out: they are
// read as files.
func (r *gitConfigReader) read(file string, skipRepo bool) error {
	args := []string{"config", "--list", "-z", "--show-scope", "--show-origin", "--no-includes"}
	if file != "" {
		args = append(args, "--file", file)
	}
	cmd := disclaim.Cmd(exec.CommandContext(r.ctx, r.git, args...))
	cmd.Dir, cmd.Env = r.dir, r.env
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("sandbox: reading the owner's git config %s: %w", file, err)
	}
	fields := strings.Split(string(out), "\x00")
	if len(fields)%3 != 1 || fields[len(fields)-1] != "" {
		return fmt.Errorf("%w: git config's list didn't parse", ErrGitConfig)
	}
	for i := 0; i+2 < len(fields); i += 3 {
		scope, origin, entry := fields[i], fields[i+1], fields[i+2]
		key, value, hasValue := strings.Cut(entry, "\n")
		var from string
		switch {
		case skipRepo && (scope == "local" || scope == "worktree"):
			continue
		case strings.HasPrefix(origin, "file:"):
			from = strings.TrimPrefix(origin, "file:")
			if !filepath.IsAbs(from) {
				return fmt.Errorf("%w: a config file git gives as %q", ErrGitConfig, from)
			}
			r.seen[from] = true
		case origin != "command line:":
			return fmt.Errorf("%w: a value from %q", ErrGitConfig, origin)
		}
		if hasValue {
			if err := r.take(key, value, from); err != nil {
				return fmt.Errorf("%w (%s in %s)", err, key, origin)
			}
		}
	}
	return nil
}

// take is what one value names. from is the file it's in, "" for the command line.
func (r *gitConfigReader) take(key, value, from string) error {
	switch {
	case key == "include.path" || strings.HasPrefix(key, "includeif.") && strings.HasSuffix(key, ".path"):
		var in []string // a relative include on the command line is in no folder: git refuses it too
		if from != "" {
			in = []string{filepath.Dir(from)}
		}
		files, err := r.places(value, in)
		r.named = append(r.named, files...)
		r.includes = append(r.includes, files...)
		return err
	case key == "core.hookspath":
		folders, err := r.places(value, r.roots)
		r.hooks = append(r.hooks, folders...)
		if err != nil {
			return err
		}
		// Husky's `_` (core.hooksPath = .husky/_): each hook there sources `_/h`, which runs the hook of its name in the
		// folder above, `.husky/pre-commit` and its siblings, a missing one being no hook, and first sources husky's own
		// startup files, `${XDG_CONFIG_HOME:-~/.config}/husky/init.sh` and (husky 8) `~/.huskyrc` (B1).
		for _, folder := range folders {
			if filepath.Base(folder) != "_" {
				continue
			}
			above, err := r.places(filepath.Dir(folder), nil)
			if err != nil {
				return err
			}
			r.hooks = append(r.hooks, above...)
			config := r.xdg
			if config == "" {
				config = "~/.config"
			}
			for _, startup := range []string{config + "/husky/init.sh", "~/.huskyrc"} {
				files, err := r.places(startup, r.roots)
				if err != nil {
					return err
				}
				r.named = append(r.named, files...)
			}
		}
		return nil
	case key == "core.excludesfile":
		// The ignore file (FXAGENTHOMEMACR1, RT126AGENTHOMEMAC O2): the core hands the owner's to every read
		// (desktop's askGitHere), so a desk that wrote it would hide its files from them, and from the owner's git.
		files, err := r.ignorePlaces(value)
		r.named = append(r.named, files...)
		return err
	case key == "core.fsmonitor" && gitBool(value):
		return nil // git's own daemon, or none
	case key == "core.fsmonitor" || key == "core.sshcommand":
		words, err := commandWords(value)
		if err != nil {
			return err
		}
		for i, word := range words {
			for _, file := range commandFiles(i, word) {
				if strings.Contains(file, "%") && strings.Contains(file, "/") {
					return ErrGitConfig // ssh's %d and the like
				}
				if strings.Contains(file, "%") {
					continue
				}
				files, err := r.places(file, r.roots)
				if err != nil {
					return err
				}
				r.named = append(r.named, files...)
			}
		}
	}
	return nil
}

// commandFiles are the words of a command that may be files: the program when it has a folder in it (else it's found
// on PATH), and every other word's value. A word's value is what follows its `=` (ssh's `-o Key=Value`), or an
// option's letter (`-F/x`, `-i/k`: ssh's getopt takes a value attached), or the word itself. A word with spaces in it is
// taken whole, and each of its words too, the same way: ssh_config's `Key Value` form (`-o "ProxyCommand /x"`), or a
// command of its own (B3). The whole is left out when it holds a `%` (a command line, `nc %h %p`, not a path).
func commandFiles(i int, word string) []string {
	if i == 0 {
		if strings.Contains(word, "/") || strings.HasPrefix(word, "~") {
			return []string{word}
		}
		return nil
	}
	var files []string
	for j, w := range append([]string{word}, strings.Fields(word)...) {
		if j == 0 && strings.ContainsAny(w, " \t\n") && strings.Contains(w, "%") {
			continue
		}
		if _, value, ok := strings.Cut(w, "="); ok {
			w = value
		} else if strings.HasPrefix(w, "--") {
			w = ""
		} else if strings.HasPrefix(w, "-") {
			w = w[min(2, len(w)):]
		}
		if w != "" && !slices.Contains(files, w) {
			files = append(files, w)
		}
	}
	return files
}

// followHookLinks names the target of every link in the hooks folders, at any depth, as far as it exists: git runs a
// hook through its link, and a hook can run what a link beside it leads to (B2). A folder a link leads to is a hooks
// folder too, and its links are followed in turn. The folders are denied whole, so the links are the owner's.
func (r *gitConfigReader) followHookLinks() error {
	looked := 0
	walked := map[string]bool{}
	for i := 0; i < len(r.hooks); i++ {
		folder := r.hooks[i]
		if walked[folder] {
			continue
		}
		walked[folder] = true
		err := filepath.WalkDir(folder, func(at string, d fs.DirEntry, err error) error {
			switch {
			case err != nil && at == folder && missing(err):
				return filepath.SkipAll
			case err != nil:
				return err
			}
			if looked++; looked > maxHookEntries {
				return fmt.Errorf("%w: more than %d entries in the hooks folders", ErrGitConfig, maxHookEntries)
			}
			if d.Type()&fs.ModeSymlink == 0 || at == folder {
				return nil
			}
			real, err := followLinks(at, 0)
			if err != nil {
				return err
			}
			switch info, err := os.Stat(real); {
			case err == nil && info.IsDir():
				r.hooks = append(r.hooks, real)
			case err == nil || missing(err):
				r.named = append(r.named, real)
			default:
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// ignorePlaces is an ignore file's value (core.excludesFile) as git reads it in the desk's root and the checkout
// (places), with ExcludesFileError for a spelling it can't tell.
func (r *gitConfigReader) ignorePlaces(value string) ([]string, error) {
	if strings.HasPrefix(value, "~") && value != "~" && !strings.HasPrefix(value, "~/") || strings.HasPrefix(value, "%(") {
		return nil, &ExcludesFileError{Value: value}
	}
	return r.places(value, r.roots)
}

// CarriedIgnorePlaces are the places of the ignore file the core carries into its reads (core.excludesFile, as its
// git was asked for it once: desktop's gitHere), to add to Places.GitNamed at each desk's start (FXAGENTHOMEMACR2,
// RT126AGENTHOMEMACR1 N2): ReadGitConfig names the file the config names now, and the owner may have changed it since.
// Spelled as git reads it, as ReadGitConfig does: `~` from each of homes, a relative one in each of roots, as written
// and with its links followed. "" and /dev/null are none.
func CarriedIgnorePlaces(value string, homes, roots []string) ([]string, error) {
	r := &gitConfigReader{}
	for _, home := range homes {
		if home != "" && !slices.Contains(r.homes, home) {
			r.homes = append(r.homes, home)
		}
	}
	for _, root := range roots {
		if root != "" && !slices.Contains(r.roots, root) {
			r.roots = append(r.roots, root)
		}
	}
	return r.ignorePlaces(value)
}

// places is a path value as git reads it, `~` expanded, relative to each of bases: as written and with its links
// followed. "" and /dev/null are none.
func (r *gitConfigReader) places(value string, bases []string) ([]string, error) {
	var at []string
	switch {
	case value == "" || value == "/dev/null" || value == os.DevNull:
		return nil, nil
	case value == "~" || strings.HasPrefix(value, "~/"):
		for _, home := range r.homes {
			at = append(at, filepath.Join(home, value[1:]))
		}
	case strings.HasPrefix(value, "~") || strings.HasPrefix(value, "%("):
		return nil, ErrGitConfig
	case filepath.IsAbs(value):
		at = []string{value}
	case len(bases) == 0:
		return nil, ErrGitConfig
	default:
		for _, base := range bases {
			at = append(at, filepath.Join(base, value))
		}
	}
	var out []string
	for _, p := range at {
		p = filepath.Clean(p)
		real, err := followLinks(p, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, p, real)
	}
	return out, nil
}

// followLinks is p with every link in it followed, as far as it exists: a link whose target is missing is followed
// too, so a desk can't make its target.
func followLinks(p string, depth int) (string, error) {
	if depth > 40 {
		return "", fmt.Errorf("%w: too many links in %s", ErrGitConfig, p)
	}
	rest := ""
	for dir := p; ; {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(real, rest), nil
		}
		switch info, err := os.Lstat(dir); {
		case err == nil && info.Mode()&fs.ModeSymlink != 0:
			to, err := os.Readlink(dir)
			if err != nil {
				return "", err
			}
			if !filepath.IsAbs(to) {
				to = filepath.Join(filepath.Dir(dir), to)
			}
			return followLinks(filepath.Join(to, rest), depth+1)
		case err != nil && !missing(err):
			return "", err
		}
		up := filepath.Dir(dir)
		if up == dir {
			return p, nil
		}
		rest, dir = filepath.Join(filepath.Base(dir), rest), up
	}
}

// missing is whether err says a path isn't there, as git takes it: a folder in it that is a file (ENOTDIR) is missing
// too (N1: `core.fsmonitor = .git/hooks/x` in a worktree, whose .git is a file). The path is still named.
func missing(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// gitBool is whether value is one of git's booleans (git_parse_maybe_bool): a word or a number.
func gitBool(value string) bool {
	switch strings.ToLower(value) {
	case "true", "yes", "on", "false", "no", "off", "":
		return true
	}
	_, err := strconv.Atoi(value)
	return err == nil
}

// commandWords are a command's words as git's shell (sh -c) would hand them on, its quotes removed. What only the shell
// could tell, an expansion, a glob, an operator or a comment, is ErrGitConfig, as are unbalanced quotes.
func commandWords(command string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord := false
	for i := 0; i < len(command); i++ {
		switch c := command[i]; {
		case c == ' ' || c == '\t' || c == '\n':
			if inWord {
				words, inWord = append(words, word.String()), false
				word.Reset()
			}
		case c == '\'':
			end := strings.IndexByte(command[i+1:], '\'')
			if end < 0 {
				return nil, ErrGitConfig
			}
			word.WriteString(command[i+1 : i+1+end])
			i, inWord = i+1+end, true
		case c == '"':
			for i++; i < len(command) && command[i] != '"'; i++ {
				switch command[i] {
				case '$', '`':
					return nil, ErrGitConfig
				case '\\':
					if i+1 < len(command) && strings.IndexByte("$`\"\\", command[i+1]) >= 0 {
						i++
					}
				}
				word.WriteByte(command[i])
			}
			if i == len(command) {
				return nil, ErrGitConfig
			}
			inWord = true
		case c == '\\':
			if i+1 == len(command) {
				return nil, ErrGitConfig
			}
			i++
			word.WriteByte(command[i])
			inWord = true
		case strings.IndexByte("$`;&|<>()*?[]{}!", c) >= 0 || c == '#' && !inWord:
			return nil, ErrGitConfig
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}
