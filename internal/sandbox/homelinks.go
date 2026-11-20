package sandbox

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// The owner's places kept as links (FXHOMELINKS, RT126RESOLVESWEEP S2): stow, chezmoi and yadm keep `~/.zshrc`,
// `~/.gitconfig` or `~/.claude` as links into a dotfiles folder. A Mac's kernel judges a write or a read through a
// link where the link leads, and Linux's mounts land where it leads too, so a row spelled at the link alone holds the
// link and nothing else: a desk could append to the owner's `.zshrc` through it. So each of the owner's places is
// held, at each start, as the programs are (programs.go's linkHops):
//   - as spelled, by its own row;
//   - where its links lead, by the same row there (its Tail and exceptions with it): every place a row of the
//     owner's names is walked as the system walks it;
//   - every link on the way, where it is met, the ones in the middle too: none can be removed, renamed, made again or
//     changed, so none can be pointed elsewhere (a middle hop put back would lead elsewhere);
//   - a link among the names a row holds by a pattern (`~/.ssh/*`, `authorized_keys*`, `.claude.json*`) or in a folder
//     a row holds whole (`~/.claude`, `~/.config/git`, `Library/LaunchAgents`): what the row denies at the link is
//     denied where it leads, and its links on the way are held. Where it leads is looked into the same way, and so on
//     to the end, so a link two levels in is followed too (`~/.claude/hooks/pre.sh` through a linked `hooks`). The
//     account folders' tokens and history are not looked into: no row of theirs lets a desk read or write anything
//     below them, and transcripts are many;
//   - a place a link leads to that isn't there yet: the first folder missing on the way, whose parent is there, is
//     denied whole, so a desk can't make it (nothing is made for it).
//
// The folders that hold an owner's app or agent state whole (appStateRows: the preferences, the terminals' and the
// agents' folders, the keychains; FXPLANTSTERMR4) and the account folders (`~/.claude` and the other accounts';
// FXPOLISHA) grow with the owner's use, past what any walk at each start could look through: OpenCode's snapshot gits,
// Gemini CLI's checkpoints, Codex's sessions, Claude Code's file history and to-do lists. So in each of them, and where
// a link found there leads, only the top two levels are looked at, and the bulk stores named in bulkStateFolders
// (bulkAccountFolders for an account folder) not at all. An app's state counts on a budget of each folder's own; an
// account folder counts on the budget of the folders held whole, as it did when it was looked into whole itself
// (FXPOLISHAR3), so a heavy ~/.claude that started a hire then still starts one. Every link those two levels
// hold is followed and held as above: a settings file at the top kept as a link into a dotfiles folder
// (`~/.codex/config.toml`, `~/.claude/settings.json`) is the case that matters. A link deeper in, or inside a bulk
// store, isn't followed (a known limit): the desk can't write there, so only the owner or their own app could have
// made it. Two kinds of place deeper in are looked at all the same:
//   - in an account folder, what Claude Code loads or runs, kept by the owner and small (accountWholeFolders: the
//     skills, the commands, the sub-agents, the hooks), looked into whole (FXPOLISHAR1): a skill's SKILL.md, a
//     namespaced command, a sub-agent or a hook in a subfolder, kept as a link, is followed. Where a link in one leads
//     is looked into whole too;
//   - iTerm2's Scripts/AutoLaunch, in its Application Support folder, as a folder of its own: iTerm2 runs those
//     scripts at its every launch, and Smolcorp launches it.
//
// A link loop, a place that can't be read on the way, or a folder too big to look through fails the start: nothing
// could then say where a write through it lands. The last says which folder, in words (HomeTooBigError). This is the
// computer's own walk, so it is done only for the system the core runs on; a Policy made for another system's paths (a
// test's) has none.

// rowHomeLink is the row that holds a link on the way to one of the owner's places.
const rowHomeLink = "a link on the way to the owner's places"

// maxHomeEntries is how many names followLinks looks at, at most, in the folders the owner's rows hold whole, but the
// apps' state (appStateRows), all together: the account folders and where their links lead count here too, both their
// two-level look and their accountWholeFolders, as when they were looked into whole (FXPOLISHAR2, FXPOLISHAR3).
// maxAppStateEntries is how many it looks at in each app's state, and in each folder a link in one leads to. A
// variable each, for the tests.
var (
	maxHomeEntries     = 200_000
	maxAppStateEntries = 50_000
)

// appStateRows are the rows that hold an owner's app or agent state whole, looked into two levels deep only (above).
var appStateRows = []string{rowTerminals, rowAgents, rowAgentLinks, rowKeychains}

// bulkStateFolders are the folders at the top of an app's state that followLinks never looks into, and why: each
// holds what the agent keeps of every session or download, growing without end (each from the agent's own source: the
// FXPLANTSTERMR4 report). Not all of it is inert: OpenCode runs the plugins in node_modules, so a plugin the owner
// links in there (`bun link`, a `file:` dependency) is code their next session runs, and where that link leads isn't
// held (the known limit above; the desk can't write in node_modules itself). One of them that is itself a link is
// still held where it leads, but not looked into there either.
var bulkStateFolders = map[string]string{
	"sessions":          "Codex's and Grok's sessions, one folder or file each, by date for Codex",
	"archived_sessions": "Codex's archived sessions",
	"history":           "Gemini CLI's checkpoints: a shadow git for each project",
	"tmp":               "Gemini CLI's per-project chats, checkpoints, logs and plans",
	"snapshot":          "OpenCode's snapshots: a git store for each project and worktree",
	"storage":           "OpenCode's sessions, messages and parts, one file each (before its database)",
	"tool-output":       "OpenCode's tool output, one file for each call",
	"node_modules":      "OpenCode's plugin and provider packages, installed from npm",
}

// bulkAccountFolders are, the same way, the folders at the top of an account folder that followLinks never looks
// into: what Claude Code keeps of every session, a file or a folder each, growing without end (FXPOLISHA). The
// transcripts are denied whole besides, read and write. Not all of it is inert either: a session's commands source
// its shell snapshot and its hooks' environment, so a link the owner keeps inside one isn't followed (the known limit
// above).
var bulkAccountFolders = map[string]string{
	"projects":        "the transcripts, a file for each session, by project",
	"file-history":    "the copies of each file a session edited, a folder for each session",
	"todos":           "the to-do lists, a file for each session and agent",
	"shell-snapshots": "the shell's settings, caught at each session's start, a file each",
	"debug":           "the debug logs, a file for each session",
	"session-env":     "the environment hooks give each session, a folder each",
}

// accountWholeFolders are the folders at the top of an account folder that followLinks looks into whole, not two
// levels deep (above): what Claude Code loads or runs from there, each kept by the owner, and small. A skill is a
// folder with its SKILL.md in it, a command or a sub-agent may be in a subfolder of its own (a namespace), and a hook's
// script may be anywhere the settings name.
var accountWholeFolders = []string{"skills", "commands", "agents", "hooks"}

// walkKind is how followLinks looks into a folder: whole, or only two levels deep as an app's state (appStateRows) or
// an account folder, never into its bulk stores (above). walkAccountPart is whole too: one of an account folder's
// accountWholeFolders, or where a link in one leads.
type walkKind int

const (
	walkWhole walkKind = iota
	walkAppState
	walkAccount
	walkAccountPart
)

// whole is whether a folder looked into as k is looked into to the end: such a look sees all another one would.
func (k walkKind) whole() bool { return k == walkWhole || k == walkAccountPart }

// bulk is the bulk stores of a folder looked into as k: none when it is looked into whole.
func (k walkKind) bulk() map[string]string {
	switch k {
	case walkAppState:
		return bulkStateFolders
	case walkAccount:
		return bulkAccountFolders
	}
	return nil
}

// HomeTooBigError is a start followLinks refused because a folder held whole was too big to look through: Inside is
// where the look stopped, Folder or a folder at its top, and Most the names it had looked at, the most it looks at.
// InAll is whether Most counts the names of every folder looked at on the shared budget together (maxHomeEntries), not
// Folder's alone (maxAppStateEntries): Folder's own count may be in Inside's or not. Loaded is whether the look stopped
// in what an agent loads or runs from an account folder (its accountWholeFolders, Folder then the account folder) or
// where a link in one leads (Folder); Account is then that account folder.
type HomeTooBigError struct {
	Folder, Inside, Account string
	Most                    int
	InAll, Loaded           bool
}

func (e *HomeTooBigError) Error() string {
	if e.InAll {
		return fmt.Sprintf("too many names to look through: it stopped in %s, after %d in all", e.Inside, e.Most)
	}
	return fmt.Sprintf("too many names to look through: it stopped in %s, after %d in %s", e.Inside, e.Most, e.Folder)
}

// followLinks holds the owner's places, the rows from first on, where their links lead (see above). home and owned
// are the home folder and the account folders: only rows in those are looked into for links below them.
func (b *builder) followLinks(first int, home string, owned []string) {
	if b.goos != runtime.GOOS || b.err != nil {
		return
	}
	fail := func(at string, err error) {
		if b.err == nil {
			b.err = fmt.Errorf("%w: %s: %w", ErrHomeLinks, at, err)
		}
	}
	homeKey := key(b.goos, home)
	inOwned := func(p string) bool {
		k := key(b.goos, p)
		return under(k, homeKey) || slices.ContainsFunc(owned, func(o string) bool { return o != "" && under(k, key(b.goos, o)) })
	}
	seen := map[string]bool{}
	hold := func(hops []string) {
		for _, hop := range hops {
			k := key(b.goos, hop)
			if k == "" || seen[k] || topFolder(hop) || sealed(b.goos, k) || under(homeKey, k) {
				continue
			}
			seen[k] = true
			b.add(Rule{Row: rowHomeLink, Path: hop, Write: Deny})
		}
	}
	notThere := func(at string, read Verdict) {
		if m := firstMissing(at); m != "" && !topFolder(m) {
			r := Rule{Row: rowHomeLink + ": where one leads, not there yet", Path: m, Below: true, Write: Deny}
			if read == Deny {
				r.Read = Deny
			}
			b.add(r)
		}
	}

	// Each row's own place, where its links lead. kind: whole, or two levels deep (above).
	type folder struct {
		at      string
		row     Rule
		kind    walkKind
		account string // the account folder it was reached from, if any: what the too-big words name (Loaded)
	}
	var look []folder
	for _, r := range slices.Clone(b.rules[first:]) {
		if r.Path == "" {
			continue
		}
		hops, real, err := linkHops(filepath.FromSlash(r.Path))
		if err != nil {
			fail(r.Path, err)
			return
		}
		at := r.Path
		if len(hops) > 0 && key(b.goos, real) != r.key {
			hold(hops)
			// A writer's own name in a folder goes where ownName would put it, no further: not into a place the desk
			// works in, nor perl -i's into the home folder itself, when the folder as spelled leads there (F1).
			if slices.Contains(ownersInFolder, path.Base(r.Tail)) && b.leavesOut(key(b.goos, real), r.Tail) {
				continue
			}
			if !b.heldAlready(r, real) {
				moved := r
				moved.Path = real
				b.add(moved)
				if r.denies() {
					notThere(real, r.Read)
				}
			}
			at = real
		}
		if r.denies() && (r.Tail != "" || r.Below) && inOwned(r.Path) && !(r.Tail == "" && r.Read == Deny) {
			kind, account := walkWhole, ""
			switch {
			case slices.Contains(appStateRows, r.Row):
				kind = walkAppState
			case r.Row == rowAccounts:
				kind, account = walkAccount, at
			}
			look = append(look, folder{at, r, kind, account})
			// iTerm2's AutoLaunch scripts, three levels into its Application Support folder: a folder of its own (above).
			// iTerm2 reads them there, never in ~/.config/iterm2, so that row gets no such look (FXPOLISHAR1).
			if r.Row == rowTerminals && strings.EqualFold(path.Base(r.Path), "iterm2") &&
				strings.EqualFold(path.Base(path.Dir(r.Path)), "Application Support") {
				look = append(look, folder{filepath.Join(at, "Scripts", "AutoLaunch"), r, walkAppState, ""})
			}
		}
	}
	if b.err != nil {
		return
	}

	// The links below them, each once. A link the Policy denies anything at leads somewhere held now, so where it leads
	// is walked too, for the links in there (RT126HOMELINKS B1: `~/.claude/hooks → elsewhere/hooks`, with
	// `elsewhere/hooks/pre.sh → ../scripts/pre.sh`), round after round with the rows each round adds, to the end.
	// A link found in an app's state or an account folder is followed as one found elsewhere, but where it leads is
	// looked into as that folder is (kind), and not at all when it is a bulk store (bulk); one found in an account
	// folder's accountWholeFolders, or that is one, leads to a place looked into whole (walkAccountPart).
	type link struct {
		at      string
		kind    walkKind
		bulk    bool
		account string
	}
	found := map[string]walkKind{}  // each link, and how it was found: a whole look finds it again (RT126POLISHAR1's B1)
	walked := map[string]walkKind{} // how each place was looked into: whole covers the others
	budget := maxHomeEntries
	skip := map[string]bool{} // the tokens and history: denied whole, read and write (see above)
	for _, r := range b.rules[first:] {
		if r.Path != "" && r.Tail == "" && r.Below && r.Read == Deny && r.Write == Deny {
			skip[r.key] = true
		}
	}
	for len(look) > 0 {
		// Each round's whole looks first, then its two-level ones, each in the order found (RT126POLISHAR3 N1, ruled
		// (a)): a place reached both ways in one round (a folder linked from the plugins and from the skills) is
		// looked into once, whole, and walked skips the two-level look of it, so the shared count isn't spent on it
		// twice. Links that lead there from another round's look still walk it again.
		slices.SortStableFunc(look, func(x, y folder) int {
			switch {
			case x.kind.whole() == y.kind.whole():
				return 0
			case x.kind.whole():
				return -1
			}
			return 1
		})
		var links []link
		inRound := map[string]int{} // each link's place in links: found again this round, it is looked at once
		for _, f := range look {
			k := key(b.goos, f.at) + "\x00" + f.row.tail
			if was, done := walked[k]; done && (was.whole() || was == f.kind) {
				continue
			}
			walked[k] = f.kind
			info, err := os.Stat(f.at)
			if missing(err) || err == nil && !info.IsDir() {
				continue
			} else if err != nil {
				fail(f.at, err)
				return
			}
			own := maxAppStateEntries
			err = filepath.WalkDir(f.at, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(f.at, p)
				top, _, deeper := strings.Cut(rel, string(filepath.Separator))
				kind := f.kind
				if kind == walkAccount && p != f.at && slices.Contains(accountWholeFolders, foldName(b.goos, top)) {
					kind = walkAccountPart
				}
				// An app's state counts on its own budget. Everything else counts on the shared one: an account folder,
				// its accountWholeFolders, and where their links lead too, as before FXPOLISHA, when ~/.claude was looked
				// into whole: a big skills folder (a skill's node_modules), or a big paste-cache, doesn't stop a start
				// that one had made (FXPOLISHAR2 and FXPOLISHAR3, RT126POLISHAR1's and RT126POLISHAR2's N1).
				most, left := maxHomeEntries, &budget
				if kind == walkAppState {
					most, left = maxAppStateEntries, &own
				}
				if *left--; *left < 0 {
					inside := f.at
					if deeper {
						inside = filepath.Join(f.at, top)
					}
					e := &HomeTooBigError{Folder: f.at, Inside: inside, Most: most, InAll: left == &budget, Loaded: kind == walkAccountPart}
					if e.Loaded {
						e.Account = f.account
					}
					return e
				}
				if p == f.at {
					return nil
				}
				if d.IsDir() && skip[key(b.goos, p)] {
					return filepath.SkipDir
				}
				if f.row.Tail != "" && !onTail(splitTail(f.row.tail), splitTail(foldName(b.goos, filepath.ToSlash(rel))), f.row.Below) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				_, bulk := f.kind.bulk()[top]
				bulk = bulk && !deeper
				// A link a two-level look found first is found again by a whole one, so that where it leads is looked
				// into whole too (RT126POLISHAR1's B1: the same folder reached from the plugins and the skills, or from
				// Codex's skills and Claude Code's). Found again in the same round, it takes the first one's place, so
				// its row is added once (RT126POLISHAR2's O3); in a later round, the row is there already.
				if was, met := found[p]; d.Type()&fs.ModeSymlink != 0 && (!met || kind.whole() && !was.whole()) {
					found[p] = kind
					l := link{p, kind, bulk, f.account}
					if i, again := inRound[p]; again {
						links[i] = l
					} else {
						inRound[p] = len(links)
						links = append(links, l)
					}
				}
				if !kind.whole() && d.IsDir() && (deeper || bulk) { // two levels, and no bulk store
					return filepath.SkipDir
				}
				return nil
			})
			if err != nil {
				fail(f.at, err)
				return
			}
		}
		look = nil
		sofar := &Policy{goos: b.goos, rules: slices.Clone(b.rules)} // with the rows the last round added
		for _, l := range links {
			link := l.at
			hops, real, err := linkHops(link)
			if err != nil {
				fail(link, err)
				return
			}
			r := Rule{Row: rowHomeLink + ": where one in the owner's places leads", Path: real, Below: true}
			held := false
			for _, op := range []Op{Read, Write} {
				if denied, _ := rowDenies(sofar, link, op); !denied {
					continue
				}
				held = true
				if denied, _ := rowDenies(sofar, real, op); denied {
					continue
				}
				if op == Read {
					r.Read = Deny
				} else {
					r.Write = Deny
				}
			}
			if r.denies() {
				b.add(r)
				notThere(real, r.Read)
			}
			if held && !l.bulk { // next round: the links where it leads (a file, or a place not there, has none)
				look = append(look, folder{real, Rule{Below: true}, l.kind, l.account})
			}
			var open []string
			for _, hop := range hops {
				if denied, _ := rowDenies(sofar, hop, Write); !denied {
					open = append(open, hop)
				}
			}
			hold(open)
		}
	}
}

// heldAlready is whether row r, moved to real, would hold nothing more than the rows so far: a deny of r's own place
// (no Tail, no allow) that a row with no Tail already denies at real, for each Op, and below it when r says Below.
// `/usr/lib/cron/tabs` leads into `/private/var/at`, which its own row holds whole.
func (b *builder) heldAlready(r Rule, real string) bool {
	if r.Tail != "" || r.Read == Allow || r.Write == Allow || r.Connect == Allow || r.Move == Allow || r.Link == Allow {
		return false
	}
	sofar := &Policy{goos: b.goos, rules: b.rules}
	for _, op := range []Op{Read, Write, Connect, Move, Link} {
		if r.verdict(op) != Deny {
			continue
		}
		if denied, by := rowDenies(sofar, real, op); !denied || by.Wide || by.Tail != "" || r.Below && !by.Below {
			return false
		}
	}
	return true
}

// rowDenies is whether a row so far denies op at p, and that row. On a Mac, Decide denies a path with a code point the
// fold's Unicode doesn't assign by a row of its own that is in no profile (rowUnknownCodePoint): that deny holds
// nothing, so it isn't counted here (RT126POLICYFOLDR2 F1). Such a place, where a link leads or on the way, then gets
// its row, and builder.add refuses it, so New does: on a later macOS whose volume takes the name, Seatbelt would
// otherwise leave it open.
func rowDenies(sofar *Policy, p string, op Op) (bool, Rule) {
	v, by := sofar.Decide(p, op)
	return v == Deny && by.Row != rowUnknownCodePoint, by
}

// onTail is whether names, below a Tail row's Path, are what the row holds, or, for a row that holds everything below
// what matches (Below), on the way there: followLinks looks nowhere else for that row. A `**` may be on the way to
// anything. `~/.claude`'s `.credentials.json*` row, which holds a temporary copy beside the file and what is below it,
// is a look at the top of `~/.claude`, not at all of it (FXPOLISHA): the account folder's own row looks into the rest.
func onTail(pattern, names []string, below bool) bool {
	if !below {
		return matchNames(pattern, names, false)
	}
	if slices.Contains(pattern, "**") || matchNames(pattern, names, true) {
		return true
	}
	return len(names) < len(pattern) && matchNames(pattern[:len(names)], names, false)
}

// splitTail is a tail's names, none for "".
func splitTail(tail string) []string {
	if tail == "" {
		return nil
	}
	return strings.Split(tail, "/")
}

// firstMissing is the first folder missing on the way to p, whose parent is there: "" when p is there.
func firstMissing(p string) string {
	if _, err := os.Lstat(p); !missing(err) {
		return ""
	}
	for at := p; ; {
		up := filepath.Dir(at)
		if up == at {
			return at
		}
		if _, err := os.Lstat(up); !missing(err) {
			return at
		}
		at = up
	}
}
