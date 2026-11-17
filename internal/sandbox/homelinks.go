package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
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
// A link loop, a place that can't be read on the way, or a folder too big to look through fails the start: nothing
// could then say where a write through it lands. This is the computer's own walk, so it is done only for the system
// the core runs on; a Policy made for another system's paths (a test's) has none.

// rowHomeLink is the row that holds a link on the way to one of the owner's places.
const rowHomeLink = "a link on the way to the owner's places"

// maxHomeEntries is how many names followLinks looks at, at most, in the folders the owner's rows hold whole.
const maxHomeEntries = 200_000

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

	// Each row's own place, where its links lead.
	type folder struct {
		at  string
		row Rule
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
			look = append(look, folder{at, r})
		}
	}
	if b.err != nil {
		return
	}

	// The links below them, each once. A link the Policy denies anything at leads somewhere held now, so where it leads
	// is walked too, for the links in there (RT126HOMELINKS B1: `~/.claude/hooks → elsewhere/hooks`, with
	// `elsewhere/hooks/pre.sh → ../scripts/pre.sh`), round after round with the rows each round adds, to the end.
	found := map[string]bool{}
	walked := map[string]bool{}
	budget := maxHomeEntries
	skip := map[string]bool{} // the tokens and history: denied whole, read and write (see above)
	for _, r := range b.rules[first:] {
		if r.Path != "" && r.Tail == "" && r.Below && r.Read == Deny && r.Write == Deny {
			skip[r.key] = true
		}
	}
	for len(look) > 0 {
		var links []string
		for _, f := range look {
			k := key(b.goos, f.at) + "\x00" + f.row.tail
			if walked[k] {
				continue
			}
			walked[k] = true
			info, err := os.Stat(f.at)
			if missing(err) || err == nil && !info.IsDir() {
				continue
			} else if err != nil {
				fail(f.at, err)
				return
			}
			err = filepath.WalkDir(f.at, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if budget--; budget < 0 {
					return errors.New("too many names to look through")
				}
				if p == f.at {
					return nil
				}
				if d.IsDir() && skip[key(b.goos, p)] {
					return filepath.SkipDir
				}
				rel, _ := filepath.Rel(f.at, p)
				if f.row.Tail != "" && !f.row.Below && !matchNames(splitTail(f.row.tail), splitTail(foldName(b.goos, filepath.ToSlash(rel))), false) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if d.Type()&fs.ModeSymlink != 0 && !found[p] {
					found[p] = true
					links = append(links, p)
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
		for _, link := range links {
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
			if held { // next round: the links where it leads (a file, or a place not there, has none)
				look = append(look, folder{real, Rule{Below: true}})
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
