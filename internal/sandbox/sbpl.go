package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

// The Mac's profile, files (SA-M1, SANDBOX-PLAN §A3.1 and §A4.1): a desk's Policy written as the Seatbelt profile
// /usr/bin/sandbox-exec starts it under, kept at <support>/hires/<id>/sandbox.sb. Every row of the Policy is in it:
// the support folder whole with the desk's own exceptions, the fences, the account folders, the plants, the shared git
// places, other desks' places, the socket folder (written whole but the desk's own folder, no links anywhere in it:
// T9), the rename guards on every folder above a denied place, the support folder, the desk's place and the owner's
// checkout, and no new `.claude` anywhere. The rest is normal work: `(allow default)`, but for what SA-M2 adds (below).
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
//   - Move: file-write-unlink, which removing a folder and renaming it away both are, as is replacing it by a rename,
//     and file-write-mode, -flags, -acl and -owner: chmod, chflags, an ACL and chgrp (RT126SWEEP2's F4).
//   - Link: file-link, and file-write-create of a symlink (vnode-type SYMLINK).
//
// A row's place is a literal, a subpath when it holds everything below, and a regex when it has a Tail: `**` is any
// number of folders, `*` and `?` stay within one name. On the Mac's volume, made case-insensitive, Seatbelt matches a
// path in any case, any Unicode normalization and Unicode's full case folding (ſ is s, ß is ss, ﬆ is st), as the
// volume and the Policy's folded key do, a name that isn't there yet included (sbpl_darwin_test.go and
// policy_fold_darwin_test.go check it), so the profile spells each place once, as the Policy does. A place Seatbelt can't be given exactly (a control character
// or bytes that aren't UTF-8 in it, a Tail with a class or an escape) fails the whole profile: the desk doesn't start.
//
// Sockets, services, signals and network (SA-M2, §A3.1 and §A4.2), from the desk's Services. Right after the header,
// before any of the file rules: unix sockets, Mach services, IOKit, signals and terminals denied by default, then what
// the desk may still reach: its own socket by its exact path, the sockets on its list (and the agents on it, while
// Remote Login doesn't answer), the Mach services and IOKit classes on its lists, the terminals it made itself, and
// signals to its own sandbox only (never `(target others)`, which doesn't hold on macOS 26: C1). The file rules come
// after, so the `desk-start` socket's allow (its Policy row) opens it past the default, and the support folder's denies
// still win over any list. Last of all, after every allow, what nothing may open: the local TCP ports on its list, the
// agents while Remote Login answers (C5: the ssh keys are its Policy's), typing into a terminal (TIOCSTI), a terminal's
// key made by the desk itself, Apple Events, opening apps (lsopen), new launchd jobs, and issuing a sandbox extension,
// of any class (file-issue-extension: FXFIREREAPR5), or of any other kind (generic-, mach- and iokit-issue-extension:
// FXFIREREAPR6, which grant no read of the mark, but close the class). A process that takes an extension may do what it
// grants wherever its own sandbox allows by extension, as App Sandbox's does; a desk could otherwise hand another
// sandbox a read of its mark (Places.ReapMark) and have that process taken for its own at its stop. A desk's profile
// allows nothing by extension but its own terminals (macOS's pty key, below, which it can't make or hand on either), so
// one handed to a desk opens nothing else. Each allow and deny names the same operation (network-outbound,
// mach-lookup, iokit-open, signal, and a terminal's reads and writes as Read and Write's; its file-revoke is only
// denied), as above. Seatbelt checks a unix socket where its path's links lead (T8), so a link at the desk's own
// socket to another server is refused.
//
// Terminals (FXPTYOWN, RT126SWEEP2 F2): a desk opens only the ptys it made itself, so it can't read what the owner
// types in a Terminal, iTerm2 or Ghostty window, or in an attach window, or write to their screens. Every pty's slave
// (/dev/ttys<n>, and the old /dev/tty[p-w]<x> names) is denied by default, read, write and revoke, as a path; the desk
// may still open one while it holds macOS's own key to it (the com.apple.sandbox.pty extension), which the kernel gives
// the process that opened the pty's master (posix_openpt and grantpt), and the children it starts after: its tmux's
// panes, its agents' ptys, a nested script(1). Not a pty made outside its sandbox, another desk's, one made in another
// start of its own profile (each sandbox-exec is a sandbox of its own), nor a number it let go and something outside
// took again. Under `(allow default)` a desk could make itself that key for any path, so making one, of any of the four
// kinds, is denied last. A terminal it is handed stays its own to use (its stdin, an attach client's window, a plain
// hire's pty made by its launcher): file-ioctl isn't denied here (tcsetattr on it would be), and /dev/tty, its
// controlling terminal, is another path. A program that opens its own terminal again by its name (zsh, node's libuv)
// falls back to /dev/tty or to what it was handed. Nor may a desk revoke() a terminal by its path, which cuts off every
// other holder of it, a window outside too (FXPTYOWNR1, the review's N1): file-revoke is denied with the reads and
// writes, and isn't allowed back even on its own ptys, since neither its tmux nor script(1) revokes one. Probed on
// macOS 26.6.2 (FXPTYOWN, FXPTYOWNR1).
//
// Each local port is two lines: `(remote tcp4 "localhost:N")`, which Seatbelt reads as every address of this Mac over
// IPv4 (127.0.0.1, 0.0.0.0, its network addresses too), and `(remote tcp6 "*:N")`, any address over IPv6. A host can
// only be `*` or `localhost`, and `localhost` over IPv6 misses an IPv4-mapped address (::ffff:127.0.0.1 reaches a
// server listening on 127.0.0.1 only, as Homebrew's databases and Docker's API do: RT126SAM2 B1), so IPv6 is denied
// on that port whatever the address (SIERRA's ruling): a server reached only over IPv6 on it needs AQ21's opt-in (the
// databases) or an IPv4 address. Never `(remote tcp "localhost:N")` next to a tcp6 line: Seatbelt then drops the IPv4
// half of the localhost lines, of one port or all, in either order (FXSAM2R1's probe; sbpl_darwin_test.go checks it).

// Errors Profile gives.
var (
	ErrNotAMac      = errors.New("sandbox: a Seatbelt profile is for a Mac's Policy")
	ErrUnspellable  = errors.New("sandbox: a place has a character a Seatbelt profile can't be given exactly")
	ErrTailOutOfSet = errors.New("sandbox: a row's Tail has a pattern a Seatbelt profile doesn't write (a class or an escape)")
	ErrOwnSocket    = errors.New("sandbox: a desk's own socket isn't in its own socket folder")
	ErrListed       = errors.New("sandbox: a listed socket, service, class or port can't be in a desk's profile")
)

// Services is what a desk's profile on a Mac lets it reach beyond its files (SA-M2), from the lists (SA-X2) as its
// start finds them. Whatever isn't here is denied: an empty Services denies every unix socket but the desk-start
// socket, every Mach service and IOKit class, and signals outside its sandbox. The TCP ports are the other way round:
// only DeniedPorts are denied, so an empty Services leaves every local port open.
type Services struct {
	// OwnSocket is the desk's own tmux socket, whole: in its own socket folder (Places.OwnSockets).
	OwnSocket string
	// Sockets are the other unix sockets it may connect to, whole: each as spelled and where its links lead
	// (SocketPaths). None may be in the socket folder: another desk's server is never on a list.
	Sockets []string
	// Agents are the sockets of the SSH, 1Password and GPG agents, as Sockets: allowed while Remote Login doesn't
	// answer, and denied past anything else while it does (C5, AQ12), as the ssh keys are.
	Agents []string
	// Mach are the Mach services it may look up, by global name.
	Mach []string
	// IOKit are the IOKit classes it may open.
	IOKit []IOKitOpen
	// DeniedPorts are the local TCP ports it can't connect to, on every address of this Mac over IPv4 and on any
	// address over IPv6 (DeniedPorts of the lists: :22, Docker's, and the databases' until AQ21's opt-in has passed).
	DeniedPorts []int
}

// IOKitOpen is an IOKit class a desk may open: a registry entry's class, or a user client's.
type IOKitOpen struct {
	Class      string
	UserClient bool
}

// sbRule is one line of a profile: allow or deny ops at a place, for a Policy row.
type sbRule struct {
	allow bool
	ops   []string
	place sbPlace
	row   string
}

// sbPlace is a Seatbelt filter: a literal, a subpath, or a regex, with a vnode type when the rule holds only that,
// and the subpaths it doesn't hold (a Rule's Except), each a `require-not`.
type sbPlace struct {
	kind, text, vnode string
	except            []string
	// exceptRe are the regexes of what the rule doesn't hold (a Rule's ExceptTails), each a `require-not`.
	exceptRe []string
}

// sbRaw is a filter written as it is, or none ("").
const sbRaw = ""

func (pl sbPlace) String() string {
	if pl.kind == sbRaw {
		return pl.text
	}
	filters := []string{fmt.Sprintf("(%s %s)", pl.kind, sbString(pl.text))}
	for _, e := range pl.except {
		filters = append(filters, fmt.Sprintf("(require-not (subpath %s))", sbString(e)))
	}
	for _, re := range pl.exceptRe {
		filters = append(filters, fmt.Sprintf("(require-not (regex %s))", sbString(re)))
	}
	if pl.vnode != "" {
		filters = append(filters, fmt.Sprintf("(vnode-type %s)", pl.vnode))
	}
	if len(filters) > 1 {
		return "(require-all " + strings.Join(filters, " ") + ")"
	}
	return filters[0]
}

func (r sbRule) String() string {
	action := "deny"
	if r.allow {
		action = "allow"
	}
	if place := r.place.String(); place != "" {
		return fmt.Sprintf("(%s %s %s)", action, strings.Join(r.ops, " "), place)
	}
	return fmt.Sprintf("(%s %s)", action, strings.Join(r.ops, " "))
}

// The operations each Op is, allowed and denied (see above).
var (
	sbRead    = []string{"file-read-data", "file-read-xattr"}
	sbWrite   = []string{"file-write*", "file-link"}
	sbConnect = []string{"network-outbound"}
	sbMove    = []string{"file-write-unlink", "file-write-mode", "file-write-flags", "file-write-acl", "file-write-owner"}
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

// The rules of SA-M2 that don't depend on a desk: what is denied by default, and what nothing may open.
var (
	sbDefaults = []sbRule{
		{ops: []string{"network-outbound"}, place: sbPlace{text: "(remote unix-socket)"}, row: "unix sockets: only those allowed below"},
		{ops: []string{"mach-lookup"}, row: "Mach services: only those on its list"},
		{ops: []string{"iokit-open"}, row: "IOKit: only the classes on its list"},
		{ops: []string{"signal"}, row: "signals: only to its own sandbox (C1)"},
		{ops: slices.Concat(sbRead, sbWrite, []string{"file-revoke"}), place: sbPlace{text: `(regex "^/dev/tty([p-w][0-9a-f]|s[0-9]+)$")`},
			row: "terminals: only those it made itself, and never revoked"},
		// A debugger's hold on a process (FXSWEEP3P1, RT126SWEEP3 P2's B1): lldb, or any program with the right to ask, takes
		// over an unhardened program of the owner's that runs outside every sandbox (Homebrew's, one the owner built), and
		// runs what it likes there as the owner. `(deny signal)` doesn't stop it; this does.
		{ops: []string{"mach-priv-task-port"}, row: "debugging a process: only its own sandbox's"},
	}
	sbOwnTerminals = sbRule{allow: true, ops: slices.Concat(sbRead, sbWrite),
		place: sbPlace{text: `(require-all (regex "^/dev/ttys[0-9]+$") (extension "com.apple.sandbox.pty"))`},
		row:   "the terminals it made itself: macOS's key to each"}
	sbSameSandbox = sbRule{allow: true, ops: []string{"signal"}, place: sbPlace{text: "(target same-sandbox)"}, row: "its own processes (C1)"}
	sbNever       = []sbRule{
		{ops: []string{"file-ioctl"}, place: sbPlace{text: `(ioctl-command (_IO "t" 114))`}, row: "typing into a terminal (TIOCSTI)"},
		{ops: []string{"file-issue-extension", "generic-issue-extension", "iokit-issue-extension", "mach-issue-extension"},
			place: sbPlace{text: `(extension-class "com.apple.sandbox.pty")`}, row: "a terminal's key, made by the desk itself"},
		{ops: []string{"appleevent-send"}, row: "Apple Events"},
		{ops: []string{"lsopen"}, row: "opening apps (LaunchServices)"},
		{ops: []string{"job-creation"}, row: "new launchd jobs"},
		// FXSWEEP3P1 (RT126SWEEP3 P1's F1): a mount over a folder it may not write (`mount -t devfs devfs .git/hooks`)
		// hides the owner's files there from the owner's programs and the core, and outlives the desk's processes; and
		// an unmount would show what the owner's own mount hides. No desk's work mounts anything.
		{ops: []string{"file-mount", "file-unmount"}, row: "mounting or unmounting a file system"},
		// FXSWEEP3P1 (RT126SWEEP3 P2's F2): capturing packets, where the owner's group may (Wireshark's ChmodBPF), reads
		// the desks' and the owner's keys as they cross loopback to Smolcorp.
		{ops: slices.Concat(sbRead, sbWrite), place: sbPlace{text: `(regex "^/dev/bpf[0-9]*$")`}, row: "capturing packets (/dev/bpf)"},
		{ops: []string{"file-issue-extension", "generic-issue-extension", "mach-issue-extension", "iokit-issue-extension"}, row: "handing its rights to another process (sandbox extensions, of every kind): its mark stays its sandbox's alone"},
	}
)

// sbOwnDebugging lets a desk debug what it started itself, in its own sandbox: no other desk's, nor the owner's
// (FXSWEEP3P1, after sbDefaults' deny).
var sbOwnDebugging = sbRule{allow: true, ops: []string{"mach-priv-task-port"}, place: sbPlace{text: "(target same-sandbox)"},
	row: "debugging its own processes"}

// serviceRules is s for p's desk: the allows that follow the defaults, and the denies that come last.
func (p *Policy) serviceRules(s Services) (open, shut []sbRule, err error) {
	var sockets, own string
	remoteLogin := false
	for _, r := range p.rules {
		switch r.Row {
		case rowSockets:
			sockets = key(p.goos, r.Path)
		case rowOwnSockets:
			own = key(p.goos, r.Path)
		case rowSSH:
			remoteLogin = true
		}
	}
	// connect is a rule on the unix socket at at: whole and spelled exactly, the desk's own in its own folder, and
	// none other in the socket folder.
	connect := func(allow, ownSocket bool, at, row string) (sbRule, error) {
		if !sbSpellable(at) || !filepath.IsAbs(at) || filepath.Clean(at) != at {
			return sbRule{}, fmt.Errorf("%w: the socket %q", ErrListed, at)
		}
		k := key(p.goos, at)
		if ownSocket && (own == "" || k == own || !under(k, own)) {
			return sbRule{}, ErrOwnSocket
		}
		if !ownSocket && sockets != "" && under(k, sockets) {
			return sbRule{}, fmt.Errorf("%w: %q is in the socket folder", ErrListed, at)
		}
		return sbRule{allow: allow, ops: sbConnect, place: sbPlace{kind: "literal", text: spell(p.goos, at)}, row: row}, nil
	}
	named := func(op, kind, name, row string) (sbRule, error) {
		if name == "" || !sbSpellable(name) {
			return sbRule{}, fmt.Errorf("%w: %q", ErrListed, name)
		}
		return sbRule{allow: true, ops: []string{op}, place: sbPlace{kind: kind, text: name}, row: row}, nil
	}
	var ports []sbRule
	add := func(r sbRule, err error) error {
		switch {
		case err != nil:
			return err
		case r.allow:
			open = append(open, r)
		default:
			shut = append(shut, r)
		}
		return nil
	}
	if s.OwnSocket != "" {
		if err := add(connect(true, true, s.OwnSocket, "its own socket")); err != nil {
			return nil, nil, err
		}
	}
	for _, at := range s.Sockets {
		if err := add(connect(true, false, at, "a socket on its list")); err != nil {
			return nil, nil, err
		}
	}
	for _, at := range s.Agents {
		row := "an agent on its list: Remote Login doesn't answer"
		if remoteLogin {
			row = "an agent, while Remote Login answers (C5)"
		}
		if err := add(connect(!remoteLogin, false, at, row)); err != nil {
			return nil, nil, err
		}
	}
	for _, name := range s.Mach {
		if err := add(named("mach-lookup", "global-name", name, "a Mach service on its list")); err != nil {
			return nil, nil, err
		}
	}
	for _, c := range s.IOKit {
		kind := "iokit-registry-entry-class"
		if c.UserClient {
			kind = "iokit-user-client-class"
		}
		if err := add(named("iokit-open", kind, c.Class, "an IOKit class on its list")); err != nil {
			return nil, nil, err
		}
	}
	for _, port := range s.DeniedPorts {
		if port < 1 || port > 65535 {
			return nil, nil, fmt.Errorf("%w: the port %d", ErrListed, port)
		}
		ports = append(ports,
			sbRule{ops: sbConnect, place: sbPlace{kind: "remote tcp4", text: fmt.Sprintf("localhost:%d", port)}, row: "a local port on its list: any address of this Mac"},
			sbRule{ops: sbConnect, place: sbPlace{kind: "remote tcp6", text: fmt.Sprintf("*:%d", port)}, row: "a local port on its list: any IPv6 address, ::ffff:127.0.0.1 too (C4)"})
	}
	return append(open, sbOwnTerminals, sbSameSandbox, sbOwnDebugging), slices.Concat(ports, shut, sbNever), nil
}

// sbSection is a profile's lines under one heading.
type sbSection struct {
	heading string
	rules   []sbRule
}

// sections is p's profile with svc, section by section, in the order Seatbelt must read them.
func (p *Policy) sections(svc Services) ([]sbSection, error) {
	b, err := p.fileRules()
	if err != nil {
		return nil, err
	}
	open, shut, err := p.serviceRules(svc)
	if err != nil {
		return nil, err
	}
	return []sbSection{
		{"Denied by default: unix sockets, Mach services, IOKit, signals, terminals", sbDefaults},
		{"What it may still reach", open},
		{"Allowed places", b.plain},
		{"Denied places an exception opens", b.wide},
		{"The exceptions", b.exceptions},
		{"Denied places nothing opens", b.final},
		{"Denied, whatever allows it", shut},
	}, nil
}

// Profile is p, a Mac's Policy, as a Seatbelt profile for /usr/bin/sandbox-exec -f, with what svc lets its desk
// reach: everything allowed, then SA-M2's defaults and svc's allows, each of p's rows (see above), and what nothing
// opens, a comment naming the row on each line. With no svc it reaches no unix socket but the desk-start socket, no
// Mach service or IOKit class and no signal outside its sandbox, but every local TCP port: that isn't the strictest
// profile, so a desk's start passes its Services (SA-M3, M6). More than one is refused.
func (p *Policy) Profile(svc ...Services) (string, error) {
	if len(svc) > 1 {
		return "", fmt.Errorf("%w: %d Services for one desk", ErrListed, len(svc))
	}
	sections, err := p.sections(append(svc, Services{})[0])
	if err != nil {
		return "", err
	}
	var s strings.Builder
	s.WriteString(";; A desk's sandbox (Smolcorp, SANDBOX-PLAN Appendix A), made from its Policy at its start.\n")
	s.WriteString("(version 1)\n(allow default)\n")
	for _, section := range sections {
		fmt.Fprintf(&s, "\n;; %s\n", section.heading)
		for _, r := range section.rules {
			fmt.Fprintf(&s, "%s ; %s\n", r, sbComment(r.row))
		}
	}
	return s.String(), nil
}

// sbPlaceOf is r's place as a Seatbelt filter, with the places it doesn't hold.
func sbPlaceOf(r Rule) (sbPlace, error) {
	place, err := sbFilterOf(r)
	if err != nil {
		return place, err
	}
	for _, e := range r.Except {
		if !sbSpellable(e) {
			return sbPlace{}, ErrUnspellable
		}
		place.except = append(place.except, e)
	}
	for _, tail := range r.ExceptTails {
		except, err := sbFilterOf(Rule{Path: r.Path, Tail: tail, Below: true})
		if err != nil {
			return sbPlace{}, err
		}
		place.exceptRe = append(place.exceptRe, except.text)
	}
	return place, nil
}

// sbFilterOf is r's Path and Tail as a Seatbelt filter.
func sbFilterOf(r Rule) (sbPlace, error) {
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
// with <Support>/hires/<OwnID>/sandbox.sb and the desk's Services): it refuses a link in the shared .git's refs/ and logs/ or
// on the way to a base branch's name (CheckRefPlaces), makes the folders the Policy holds that are missing (MakeHold; in the shared .git's refs/ and
// logs/, never through a link) and the keepers of the ref folders git would remove (Keep), then writes p's profile
// with svc (Profile) at file, owner-only, through a new file and a rename, so a desk never finds it half written and
// a link at file is replaced, not followed. A held folder the owner can't make either (on a volume that isn't there,
// under a folder of root's) doesn't fail the start: the profile denies writing at and below its first missing folder
// instead (makeHeld, FXAGENTHOMEMACR1). The folder file goes in must exist. Anything else that fails fails the start.
func PrepareProfile(p *Policy, file string, svc ...Services) error {
	if _, err := p.Profile(svc...); err != nil { // before anything is made
		return err
	}
	if err := p.CheckRefPlaces(); err != nil { // no link in refs/ or logs/ (RT126FIXREVIEW-GITROWS2R1's and R2's B1)
		return err
	}
	unmade, err := p.makeHold(p.Hold())
	if err != nil {
		return err
	}
	kept, err := p.makeHold(p.Keep()) // the ref folders' keepers, and the folders when missing (FXGITROWS2R1)
	if err != nil {
		return err
	}
	q, err := p.unmadeHeld(append(unmade, kept...))
	if err != nil {
		return err
	}
	profile, err := q.Profile(svc...)
	if err != nil {
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
