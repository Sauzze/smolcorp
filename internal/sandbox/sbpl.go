package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// The Mac's profile, files (SA-M1, SANDBOX-PLAN §A3.1 and §A4.1): a desk's Policy written as the Seatbelt profile
// /usr/bin/sandbox-exec starts it under, kept at <support>/hires/<id>/sandbox.sb. Every row of the Policy is in it:
// the support folder whole with the desk's own exceptions, the fences, the account folders, the plants, the shared git
// places, other desks' places, the socket folder (written whole but the desk's own folder, no links anywhere in it:
// T9), the rename guards on every folder above a denied place, the support folder, the desk's place and the owner's
// checkout, and no new `.claude` anywhere. The rest is normal work: `(allow default)`. M2 adds the sockets, services,
// signals and network after the header (Profile's sections).
//
// Seatbelt lets the last rule that matches decide, for each operation, but a rule that names the operation itself
// (file-read-data) is read before any that names it by a wildcard (file-read*): the wildcard's rules count only where
// none of the operation's own matches, whatever their order (probed on macOS 26.6.2: a later `(allow file-read* X)`
// doesn't open an earlier `(deny file-read-data X)`, and an earlier `(allow file-read-data X)` isn't shut by a later
// `(deny file-read* X)`). So an allow and a deny of the same Op name the same operations, and the only operations a
// rule names alone are Move's and Link's, which are only ever denied after everything else. The Policy lets every
// deny win but a Wide one an exception opens. So, as Policy.Rules says, each row's allows and denies go in this
// order: the plain allows, then the Wide denies, then the allows of the exceptions (Policy.Exception), then the final
// denies.
//
// Each Op is these operations:
//   - Read: file-read-data (opening a file, listing a folder) and file-read-xattr. A file's metadata stays readable:
//     a path is still looked up and resolved through a denied folder, to the desk's own files in the support folder
//     too (its brief, its Claude folder), and a denied file's size and times say nothing of what is in it.
//   - Write: file-write* (making, changing, removing, renaming in or out, a symlink) and file-link (a hard link, where
//     the path is either end: one to a denied file is refused wherever it would be made).
//   - Connect: network-outbound to the unix socket at the path.
//   - Move: file-write-unlink, which removing a folder and renaming it away both are, as is replacing it by a rename.
//   - Link: file-link, and file-write-create of a symlink (vnode-type SYMLINK).
//
// A row's place is a literal, a subpath when it holds everything below, and a regex when it has a Tail: `**` is any
// number of folders, `*` and `?` stay within one name. On the Mac's volume, made case-insensitive, Seatbelt matches a
// path in any case and any Unicode normalization, as the Policy's folded key does (sbpl_darwin_test.go checks it), so
// the profile spells each place once, as the Policy does. A place Seatbelt can't be given exactly (a control character
// or bytes that aren't UTF-8 in it, a Tail with a class or an escape) fails the whole profile: the desk doesn't start.

// Errors Profile gives.
var (
	ErrNotAMac      = errors.New("sandbox: a Seatbelt profile is for a Mac's Policy")
	ErrUnspellable  = errors.New("sandbox: a place has a character a Seatbelt profile can't be given exactly")
	ErrTailOutOfSet = errors.New("sandbox: a row's Tail has a pattern a Seatbelt profile doesn't write (a class or an escape)")
)

// sbRule is one line of a profile: allow or deny ops at a place, for a Policy row.
type sbRule struct {
	allow bool
	ops   []string
	place sbPlace
	row   string
}

// sbPlace is a Seatbelt filter: a literal, a subpath, or a regex, with a vnode type when the rule holds only that.
type sbPlace struct {
	kind, text, vnode string
}

func (pl sbPlace) String() string {
	filter := fmt.Sprintf("(%s %s)", pl.kind, sbString(pl.text))
	if pl.vnode != "" {
		return fmt.Sprintf("(require-all %s (vnode-type %s))", filter, pl.vnode)
	}
	return filter
}

func (r sbRule) String() string {
	action := "deny"
	if r.allow {
		action = "allow"
	}
	return fmt.Sprintf("(%s %s %s)", action, strings.Join(r.ops, " "), r.place)
}

// The operations each Op is, allowed and denied (see above).
var (
	sbRead    = []string{"file-read-data", "file-read-xattr"}
	sbWrite   = []string{"file-write*", "file-link"}
	sbConnect = []string{"network-outbound"}
	sbMove    = []string{"file-write-unlink"}
)

// sbBuckets are a profile's file rules in the order Seatbelt must read them.
type sbBuckets struct {
	plain, wide, exceptions, final []sbRule
}

// bucket is where a row's allow or deny goes.
func (b *sbBuckets) bucket(r Rule, exception bool, v Verdict) *[]sbRule {
	switch {
	case v == Allow && exception:
		return &b.exceptions
	case v == Allow:
		return &b.plain
	case r.Wide:
		return &b.wide
	}
	return &b.final
}

// fileRules is p's rows as Seatbelt rules, in the order they must be read: for each row, a line of what it allows
// and one of what it denies, the operations in the order of the Ops, then a symlink's own line for a Link.
func (p *Policy) fileRules() (sbBuckets, error) {
	var b sbBuckets
	if p == nil || p.goos != "darwin" {
		return b, ErrNotAMac
	}
	for _, r := range p.rules {
		place, err := sbPlaceOf(r)
		if err != nil {
			return b, fmt.Errorf("%w: the row %q", err, r.Row)
		}
		var ops [Deny + 1][]string
		add := func(v Verdict, allow, deny []string) {
			if v == Allow {
				ops[Allow] = append(ops[Allow], allow...)
			} else if v == Deny {
				ops[Deny] = append(ops[Deny], deny...)
			}
		}
		add(r.Read, sbRead, sbRead)
		add(r.Write, sbWrite, sbWrite)
		add(r.Connect, sbConnect, sbConnect)
		// Move and Link, where a row says more than its Write: Seatbelt reads file-write* as each of its operations, so
		// a later deny of one wins over an earlier allow of all.
		more := func(v Verdict) bool { return v != Unset && v != r.Write && r.Write != Deny }
		if more(r.Move) {
			add(r.Move, sbMove, sbMove)
		}
		if more(r.Link) {
			add(r.Link, []string{"file-link"}, []string{"file-link"})
		}
		exception := p.Exception(r)
		for _, v := range []Verdict{Allow, Deny} {
			if len(ops[v]) > 0 {
				to := b.bucket(r, exception, v)
				*to = append(*to, sbRule{allow: v == Allow, ops: ops[v], place: place, row: r.Row})
			}
		}
		if more(r.Link) {
			symlink := place
			symlink.vnode = "SYMLINK"
			to := b.bucket(r, exception, r.Link)
			*to = append(*to, sbRule{allow: r.Link == Allow, ops: []string{"file-write-create"}, place: symlink, row: r.Row})
		}
	}
	return b, nil
}

// Profile is p, a Mac's Policy, as a Seatbelt profile for /usr/bin/sandbox-exec -f: everything allowed, then each of
// p's rows (see above), a comment naming the row on each line.
func (p *Policy) Profile() (string, error) {
	b, err := p.fileRules()
	if err != nil {
		return "", err
	}
	var s strings.Builder
	s.WriteString(";; A desk's sandbox (Smolcorp, SANDBOX-PLAN Appendix A), made from its Policy at its start.\n")
	s.WriteString("(version 1)\n(allow default)\n")
	for _, section := range []struct {
		heading string
		rules   []sbRule
	}{
		{"Allowed places", b.plain},
		{"Denied places an exception opens", b.wide},
		{"The exceptions", b.exceptions},
		{"Denied places nothing opens", b.final},
	} {
		fmt.Fprintf(&s, "\n;; %s\n", section.heading)
		for _, r := range section.rules {
			fmt.Fprintf(&s, "%s ; %s\n", r, sbComment(r.row))
		}
	}
	return s.String(), nil
}

// sbPlaceOf is r's place as a Seatbelt filter.
func sbPlaceOf(r Rule) (sbPlace, error) {
	if !sbSpellable(r.Path) || !sbSpellable(r.Tail) {
		return sbPlace{}, ErrUnspellable
	}
	if r.Tail == "" {
		if r.Below {
			return sbPlace{kind: "subpath", text: r.Path}, nil
		}
		return sbPlace{kind: "literal", text: r.Path}, nil
	}
	var re strings.Builder
	re.WriteString("^")
	if r.Path != "/" {
		re.WriteString(sbQuoteMeta(r.Path)) // "" for anywhere: the Tail's names start at the top
	}
	for _, name := range strings.Split(r.Tail, "/") {
		if name == "**" {
			re.WriteString("(/[^/]+)*")
			continue
		}
		if strings.ContainsAny(name, `[]\`) || name == "" {
			return sbPlace{}, ErrTailOutOfSet
		}
		re.WriteString("/")
		for _, c := range name {
			switch c {
			case '*':
				re.WriteString("[^/]*")
			case '?':
				re.WriteString("[^/]")
			default:
				re.WriteString(sbQuoteMeta(string(c)))
			}
		}
	}
	if r.Below {
		re.WriteString("(/.*)?")
	}
	re.WriteString("$")
	return sbPlace{kind: "regex", text: re.String()}, nil
}

// sbSpellable is whether s can be written in a profile exactly: UTF-8, with no control character.
func sbSpellable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, c := range s {
		if c < 0x20 || c == 0x7f || (c >= 0x80 && c < 0xa0) {
			return false
		}
	}
	return true
}

// sbQuoteMeta is s with every character a regex reads escaped by a backslash, as Seatbelt's regexes and Go's both take.
func sbQuoteMeta(s string) string {
	var out strings.Builder
	for _, c := range s {
		if strings.ContainsRune(`\.+*?()|[]{}^$`, c) {
			out.WriteByte('\\')
		}
		out.WriteRune(c)
	}
	return out.String()
}

// sbString is s as a Seatbelt string: in double quotes, a backslash and a double quote escaped.
func sbString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// sbComment is a row's name as a comment: on one line.
func sbComment(row string) string {
	return strings.Join(strings.Fields(row), " ")
}

// PrepareProfile is what a desk's start on a Mac does with its Policy before the desk's first process (M6 calls it,
// with <Support>/hires/<OwnID>/sandbox.sb): it makes the folders the Policy holds that are missing (MakeHold), then
// writes p's profile at file, owner-only, through a new file and a rename, so a desk never finds it half written and a
// link at file is replaced, not followed. The folder file goes in must exist. Anything that fails fails the start.
func PrepareProfile(p *Policy, file string) error {
	profile, err := p.Profile()
	if err != nil {
		return err
	}
	if err := MakeHold(p.Hold()); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".sandbox-*.sb")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // gone after the rename; left by a failure before it
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(profile); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}
