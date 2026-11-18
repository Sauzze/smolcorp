package sandbox

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// L1APPARMOR1's rows that run anywhere (no AppArmor on a Mac): the profile is the golden text and names Smolcorp's own
// bubblewrap alone, the one command is the golden line and carries the profile whole, the start's check refuses every
// way the copy or the profile can be wrong with no fallback to the packaged bwrap, and the owner's words say why and
// give the command. That the profile lets the view start is VM run 2's (bwrap_linux_test.go).

func TestTheAppArmorProfileIsTheGoldenText(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "linux-apparmor.profile"))
	if err != nil {
		t.Fatal(err)
	}
	if AppArmorProfile != string(want) {
		t.Errorf("the profile isn't testdata/linux-apparmor.profile:\n%s", AppArmorProfile)
	}
}

// profileRules is each profile's header line and its rules, by name, as the text has them.
func profileRules(t *testing.T) (headers map[string][]string, rules map[string][]string, order []string) {
	t.Helper()
	headers, rules = map[string][]string{}, map[string][]string{}
	in := ""
	for _, line := range strings.Split(AppArmorProfile, "\n") {
		switch {
		case strings.HasPrefix(line, "profile "):
			in = strings.Fields(line)[1]
			headers[in], order = strings.Fields(line), append(order, in)
		case in != "" && line == "}":
			in = ""
		case in != "":
			rules[in] = append(rules[in], strings.TrimSpace(line))
		case strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") && line != "abi <abi/4.0>," &&
			line != "include <tunables/global>":
			t.Errorf("a line outside the profiles: %q", line)
		}
	}
	return headers, rules, order
}

// The profile names BwrapPath alone, a single file in folders root has alone: not /usr/bin/bwrap, not a pattern, not
// anything the owner's account can write. bwrap's profile enforces (an unconfined-mode profile's exec rules aren't
// looked at), and its one exec rule stacks the child profile on what it runs: pix, whose ix fallback is what gives a
// program with no profile of its own a label to stack on (a bare px refuses it: RT126L1APPARMOR1R2 B2), with both
// profiles named in the target. The child has no attachment of its own, denies every capability and any user
// namespace, allows neither anywhere, and keeps what it runs under it (ix). RT126L1APPARMOR1 B1: what runs inside
// gets no capabilities and can make no user namespace.
func TestTheProfileNamesSmolcorpsOwnBwrapAlone(t *testing.T) {
	headers, rules, order := profileRules(t)
	flags := "flags=(attach_disconnected,mediate_deleted)"
	if !slices.Equal(order, []string{AppArmorName, AppArmorChild}) {
		t.Fatalf("profiles: %q, want %s and %s", order, AppArmorName, AppArmorChild)
	}
	words := headers[AppArmorName]
	if len(words) != 5 || words[2] != BwrapPath || words[3] != flags || words[4] != "{" {
		t.Errorf("bwrap's profile line: %q", words)
	}
	if strings.ContainsAny(BwrapPath, "*?[]{}^,") || BwrapPath == PackagedBwrap || filepath.Clean(BwrapPath) != BwrapPath {
		t.Errorf("the profile attaches to %q, a pattern or the packaged bwrap", BwrapPath)
	}
	if !strings.HasPrefix(BwrapPath, "/usr/local/lib/smolcorp/") || strings.Contains(AppArmorProfile, PackagedBwrap) {
		t.Errorf("Smolcorp's bwrap is %s, outside the folder its command makes root's", BwrapPath)
	}
	if child := headers[AppArmorChild]; len(child) != 4 || child[2] != flags || child[3] != "{" {
		t.Errorf("the child's profile line: %q, want no attachment", child)
	}
	if strings.Contains(AppArmorProfile, "unconfined") || strings.Contains(AppArmorProfile, "include if exists") {
		t.Error("a profile is unconfined, or can be loosened from another file")
	}
	// What makes a profile's own exec rule: a file rule whose permissions hold an x.
	execs := func(name string) (x []string) {
		for _, r := range rules[name] {
			if f := strings.Fields(strings.TrimSuffix(r, ",")); strings.HasPrefix(r, "/") && len(f) > 1 && strings.ContainsAny(f[1], "xX") {
				x = append(x, r)
			}
		}
		return x
	}
	x := execs(AppArmorName)
	if !slices.Equal(x, []string{"/** pix -> &" + AppArmorName + "//&" + AppArmorChild + ","}) {
		t.Errorf("bwrap's profile runs programs with %q, want each stacked under %s", x, AppArmorChild)
	}
	for _, r := range x {
		f := strings.Fields(strings.TrimSuffix(r, ","))
		if len(f) != 4 || f[1] != "pix" {
			t.Errorf("bwrap's exec rule %q isn't pix: a bare px finds no profile for a program that has none, and refuses it", r)
		}
		if len(f) == 4 && (f[2] != "->" || !strings.HasPrefix(f[3], "&") ||
			!slices.Equal(strings.Split(f[3][1:], "//&"), []string{AppArmorName, AppArmorChild})) {
			t.Errorf("bwrap's exec rule %q doesn't stack %s on %s", r, AppArmorChild, AppArmorName)
		}
	}
	for _, want := range []string{"capability,", "userns,"} {
		if !slices.Contains(rules[AppArmorName], want) {
			t.Errorf("bwrap's profile lacks %q, which it needs to make the view", want)
		}
	}
	child := rules[AppArmorChild]
	for _, want := range []string{"audit deny capability,", "audit deny userns,"} {
		if !slices.Contains(child, want) {
			t.Errorf("the child's profile lacks %q", want)
		}
	}
	for _, r := range child {
		f := strings.Fields(strings.TrimSuffix(r, ","))
		switch {
		case !strings.HasPrefix(r, "audit deny ") && (strings.Contains(r, "capability") || strings.Contains(r, "userns")):
			t.Errorf("the child's profile allows %q", r)
		case slices.Contains([]string{"mount", "umount", "pivot_root", "change_profile", "all"}, f[0]):
			t.Errorf("the child's profile allows %q", r)
		case strings.HasPrefix(r, "/") && strings.ContainsAny(f[1], "xX") && (!strings.HasSuffix(f[1], "ix") || len(f) != 2):
			t.Errorf("the child's profile runs programs other than under itself: %q", r)
		}
	}
	// SandboxCommand carries it inside sh -c '…' and printf's "…": none of these may be in it.
	if strings.ContainsAny(AppArmorProfile, "'\"$`\\") {
		t.Error("the profile holds a quote, $, ` or \\, which the command can't carry as it is")
	}
}

// The one command, for an owner whose group is 1000: the golden line. It copies the packaged bwrap first, 0750 in
// the owner's group (RT126L1APPARMOR1 B1, option 2: only root and that group can run it), and printf writes the
// profile, line for line, to AppArmorFile, which is then loaded. No kernel setting.
func TestTheSandboxCommandIsTheGoldenLine(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "linux-sandbox-command.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if sandboxCommand(1000)+"\n" != string(want) {
		t.Errorf("the command isn't testdata/linux-sandbox-command.txt:\n%s", sandboxCommand(1000))
	}
	if SandboxCommand() != sandboxCommand(ownerGroup()) || ownerGroup() != os.Getgid() {
		t.Errorf("the command isn't for the group Smolcorp runs as: %s", SandboxCommand())
	}
	cmd := sandboxCommand(1000)
	start := strings.Index(cmd, `printf "%s\n" "`)
	end := strings.Index(cmd, `" > `+AppArmorFile+" && apparmor_parser -r "+AppArmorFile+"; fi'")
	if start < 0 || end < start {
		t.Fatalf("no printf to the profile's file in %s", cmd)
	}
	lines := strings.Split(cmd[start+len(`printf "%s\n" "`):end], `" "`)
	if got := strings.Join(lines, "\n") + "\n"; got != AppArmorProfile {
		t.Errorf("printf writes:\n%s\nwant the profile", got)
	}
	if !strings.HasPrefix(cmd, "sudo sh -c 'umask 022 && install -D -m 0750 -g 1000 "+PackagedBwrap+" "+BwrapPath+" && ") ||
		strings.Count(cmd, "'") != 2 || strings.Count(cmd, "install ") != 1 {
		t.Errorf("the command doesn't copy the packaged bwrap first, 0750 in the owner's group, inside one quoted sh -c: %s", cmd)
	}
	if strings.Contains(cmd, "sysctl") || strings.Contains(cmd, "/proc/sys") {
		t.Errorf("the command touches a kernel setting: %s", cmd)
	}
}

// ownComputer is a made-up Ubuntu with the restriction on, Smolcorp's bwrap and its two profiles in place: its files'
// modes, owners and groups (for rootsAlone and the copy's check) and their contents (for the copy and the kernel's
// words).
func ownComputer() (fakeFS, fstest.MapFS) {
	sys := fakeFS{"/": {kind: 'd'}, "/usr": {kind: 'd'}, "/usr/bin": {kind: 'd'}, PackagedBwrap: {kind: 'f'},
		"/usr/local": {kind: 'd'}, "/usr/local/lib": {kind: 'd'}, "/usr/local/lib/smolcorp": {kind: 'd'},
		BwrapPath: {kind: 'f', perm: 0o750, gid: int64(ownerGroup())}}
	root := fstest.MapFS{
		PackagedBwrap[1:]:                                {Data: []byte("bwrap 0.9.0")},
		BwrapPath[1:]:                                    {Data: []byte("bwrap 0.9.0")},
		restrictFile:                                     {Data: []byte("1\n")},
		abiFile:                                          {Data: []byte("abi 4.0\n")},
		policyFolder + "/snap.firefox.7/name":            {Data: []byte("snap.firefox\n")},
		policyFolder + "/snap.firefox.7/attach":          {Data: []byte("/snap/firefox/*/firefox\n")},
		policyFolder + "/snap.firefox.7/mode":            {Data: []byte("unconfined\n")},
		policyFolder + "/smolcorp-bwrap.12/name":         {Data: []byte(AppArmorName + "\n")},
		policyFolder + "/smolcorp-bwrap.12/attach":       {Data: []byte(BwrapPath + "\n")},
		policyFolder + "/smolcorp-bwrap.12/mode":         {Data: []byte("enforce\n")},
		policyFolder + "/smolcorp-bwrap-child.13/name":   {Data: []byte(AppArmorChild + "\n")},
		policyFolder + "/smolcorp-bwrap-child.13/attach": {Data: []byte(AppArmorChild + "\n")},
		policyFolder + "/smolcorp-bwrap-child.13/mode":   {Data: []byte("enforce\n")},
	}
	return sys, root
}

// labelIs makes copysLabel give label and err for the rest of t (the stand-in for the copy's exec), and counts its
// calls.
func labelIs(t *testing.T, label string, err error) *int {
	t.Helper()
	saved, calls := copysLabel, new(int)
	t.Cleanup(func() { copysLabel = saved })
	copysLabel = func() (string, error) { *calls++; return label, err }
	return calls
}

// The start's check (§A5.2 step 1): Smolcorp's copy, root's alone, 0750 in the owner's group and the packaged one byte
// for byte, and, where the restriction is on, its two profiles loaded and enforcing, bwrap's for that path (or, as
// Ubuntu 24.04 says it, "<unknown>"), and what the copy runs labelled with both exactly. Every other way is refused,
// and refused with the words that give the command (or, for a missing package, say to install it): the packaged
// bwrap being fine never stands in for Smolcorp's. Where the command can't help, or there's more to say, the words
// say so (RT126L1APPARMOR1 N1, N2, N3).
func TestTheStartChecksSmolcorpsBwrapAndItsProfile(t *testing.T) {
	sys, root := ownComputer()
	calls := labelIs(t, AppArmorStack, nil)
	if err := checkAll(sys, root); err != nil || *calls != 1 {
		t.Fatalf("Smolcorp's bwrap and profile in place: %v, its label read %d times", err, *calls)
	}
	child := policyFolder + "/smolcorp-bwrap-child.13/"
	for _, c := range []struct {
		name   string
		change func(fakeFS, fstest.MapFS)
		want   error
		words  string // also in the owner's words, besides the command
	}{
		// Where the kernel has no such rule (Debian, Fedora, Ubuntu 22.04), or it's off, no profile is needed.
		{"no restriction on this kernel", func(_ fakeFS, r fstest.MapFS) {
			delete(r, restrictFile)
			delete(r, policyFolder+"/smolcorp-bwrap.12/name")
		}, nil, ""},
		{"the restriction off", func(_ fakeFS, r fstest.MapFS) {
			r[restrictFile] = &fstest.MapFile{Data: []byte("0\n")}
			delete(r, policyFolder+"/smolcorp-bwrap.12/name")
		}, nil, ""},
		{"Smolcorp's bwrap missing", func(f fakeFS, r fstest.MapFS) { delete(f, BwrapPath); delete(r, BwrapPath[1:]) }, ErrNoOwnBwrap, ""},
		{"its folder missing (the command makes it)", func(f fakeFS, r fstest.MapFS) {
			delete(f, BwrapPath)
			delete(f, "/usr/local/lib/smolcorp")
		}, ErrNoOwnBwrap, "until Smolcorp's own bubblewrap is set up"},
		{"Smolcorp's bwrap the owner's", func(f fakeFS, _ fstest.MapFS) {
			f[BwrapPath] = fakeEntry{kind: 'f', uid: 1000, perm: 0o750, gid: int64(ownerGroup())}
		}, ErrNoOwnBwrap, ""},
		// RT126L1APPARMOR1 B1: round 1's copy, 0755, which any account could run.
		{"Smolcorp's bwrap anyone can run", func(f fakeFS, _ fstest.MapFS) {
			f[BwrapPath] = fakeEntry{kind: 'f', perm: 0o755, gid: int64(ownerGroup())}
		}, ErrNoOwnBwrap, "only root and your account's group"},
		{"Smolcorp's bwrap root's group's", func(f fakeFS, _ fstest.MapFS) {
			f[BwrapPath] = fakeEntry{kind: 'f', perm: 0o750, gid: int64(ownerGroup()) + 1}
		}, ErrNoOwnBwrap, ""},
		{"Smolcorp's bwrap root's alone to run", func(f fakeFS, _ fstest.MapFS) {
			f[BwrapPath] = fakeEntry{kind: 'f', perm: 0o700, gid: int64(ownerGroup())}
		}, ErrNoOwnBwrap, ""},
		// RT126L1APPARMOR1R2 F3: Perm() is 0750 for these too, and the words say 0750.
		{"Smolcorp's bwrap setgid, 2750", func(f fakeFS, _ fstest.MapFS) {
			f[BwrapPath] = fakeEntry{kind: 'f', perm: 0o750 | fs.ModeSetgid, gid: int64(ownerGroup())}
		}, ErrNoOwnBwrap, ""},
		{"Smolcorp's bwrap setuid, 4750", func(f fakeFS, _ fstest.MapFS) {
			f[BwrapPath] = fakeEntry{kind: 'f', perm: 0o750 | fs.ModeSetuid, gid: int64(ownerGroup())}
		}, ErrNoOwnBwrap, ""},
		{"Smolcorp's bwrap sticky, 1750", func(f fakeFS, _ fstest.MapFS) {
			f[BwrapPath] = fakeEntry{kind: 'f', perm: 0o750 | fs.ModeSticky, gid: int64(ownerGroup())}
		}, ErrNoOwnBwrap, ""},
		{"its folder the owner's", func(f fakeFS, _ fstest.MapFS) {
			f["/usr/local/lib/smolcorp"] = fakeEntry{kind: 'd', uid: 1000}
		}, errBwrapFolder, "/usr/local/lib/smolcorp, a folder above"},
		{"its folder writable by its group", func(f fakeFS, _ fstest.MapFS) {
			f["/usr/local/lib/smolcorp"] = fakeEntry{kind: 'd', perm: 0o775}
		}, errBwrapFolder, "/usr/local/lib/smolcorp, a folder above"},
		{"a Debian /usr/local, 2775 root:staff", func(f fakeFS, _ fstest.MapFS) {
			f["/usr/local"] = fakeEntry{kind: 'd', perm: 0o775, gid: 50}
		}, errBwrapFolder, "/usr/local, a folder above"},
		{"Smolcorp's bwrap a link to the packaged one", func(f fakeFS, _ fstest.MapFS) { f[BwrapPath] = fakeEntry{kind: 'l', to: PackagedBwrap} }, ErrNoOwnBwrap, ""},
		{"Smolcorp's bwrap older than the package", func(_ fakeFS, r fstest.MapFS) {
			r[PackagedBwrap[1:]] = &fstest.MapFile{Data: []byte("bwrap 0.9.1")}
		}, errStaleBwrap, "your bubblewrap was updated since it was made"},
		{"the packaged bwrap gone", func(f fakeFS, _ fstest.MapFS) { delete(f, PackagedBwrap) }, ErrNoBwrap, ""},
		{"the profile not loaded", func(_ fakeFS, r fstest.MapFS) { delete(r, policyFolder+"/smolcorp-bwrap.12/name") }, ErrNoProfile, ""},
		{"the child profile not loaded", func(_ fakeFS, r fstest.MapFS) { delete(r, child+"name") }, ErrNoProfile, ""},
		// Round 1's profile, flags=(unconfined): the kernel would skip its exec rule, so nothing would be stacked.
		{"bwrap's profile unconfined", func(_ fakeFS, r fstest.MapFS) {
			r[policyFolder+"/smolcorp-bwrap.12/mode"] = &fstest.MapFile{Data: []byte("unconfined\n")}
		}, ErrNoProfile, ""},
		{"the child complaining, not denying", func(_ fakeFS, r fstest.MapFS) {
			r[child+"mode"] = &fstest.MapFile{Data: []byte("complain\n")}
		}, ErrNoProfile, ""},
		{"the child's mode unreadable", func(_ fakeFS, r fstest.MapFS) { delete(r, child+"mode") }, ErrNoProfile, ""},
		{"the profile loaded for /usr/bin/bwrap", func(_ fakeFS, r fstest.MapFS) {
			r[policyFolder+"/smolcorp-bwrap.12/attach"] = &fstest.MapFile{Data: []byte(PackagedBwrap + "\n")}
		}, ErrNoProfile, ""},
		{"the profile loaded for a pattern", func(_ fakeFS, r fstest.MapFS) {
			r[policyFolder+"/smolcorp-bwrap.12/attach"] = &fstest.MapFile{Data: []byte("/usr/local/lib/smolcorp/*\n")}
		}, ErrNoProfile, ""},
		{"another profile for Smolcorp's bwrap", func(_ fakeFS, r fstest.MapFS) {
			r[policyFolder+"/smolcorp-bwrap.12/name"] = &fstest.MapFile{Data: []byte("other\n")}
		}, ErrNoProfile, ""},
		{"the restriction at another value", func(_ fakeFS, r fstest.MapFS) {
			r[restrictFile] = &fstest.MapFile{Data: []byte("2\n")}
			delete(r, policyFolder+"/smolcorp-bwrap.12/name")
		}, ErrNoProfile, ""},
		{"the restriction unreadable", func(_ fakeFS, r fstest.MapFS) { r[restrictFile] = &fstest.MapFile{Mode: fs.ModeDir} }, ErrNoProfile, ""},
		{"the loaded profiles unlisted", func(_ fakeFS, r fstest.MapFS) {
			for p := range r {
				if strings.HasPrefix(p, policyFolder) {
					delete(r, p)
				}
			}
		}, ErrNoProfile, ""},
		// The restriction turned on by hand on an AppArmor 3 (Ubuntu 22.04's HWE kernel): the command can't load the
		// profile there, so the words say so, not only "run this".
		{"the restriction on with AppArmor 3", func(_ fakeFS, r fstest.MapFS) {
			delete(r, abiFile)
			delete(r, policyFolder+"/smolcorp-bwrap.12/name")
			delete(r, child+"name")
		}, errOldAppArmor, "its AppArmor is older than 4.0"},
	} {
		sys, root := ownComputer()
		c.change(sys, root)
		err := checkAll(sys, root)
		if c.want == nil && err != nil || c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
		if err == nil {
			continue
		}
		words := RefusalFor(err)
		if c.want == ErrNoBwrap && words != BwrapRefusal || c.want != ErrNoBwrap && !strings.Contains(words, SandboxCommand()) ||
			!strings.Contains(words, c.words) {
			t.Errorf("%s: the owner reads %q", c.name, words)
		}
	}
}

// FXL1APPARMOR1VM2 B1, as VM run 2 read Ubuntu 24.04: bwrap's profile's attach file says "<unknown>", and what the
// copy runs is labelled with both profiles. The label is what says the profile is the copy's: anything but
// AppArmorStack exactly, or a copy that can't run its no-op, is refused with the command, whatever attach says. Where
// the restriction is off, or AppArmor is missing, the copy isn't run (a label there would be refused), and where a
// profile is missing or not enforcing, the check has refused before.
func TestTheStartReadsTheLabelOfWhatTheCopyRuns(t *testing.T) {
	cantRun := errors.New("exit status 1: bwrap: setting up uid map: Permission denied")
	for _, c := range []struct {
		name   string
		attach string // bwrap's profile's attach file; "": as ownComputer has it, BwrapPath
		change func(fstest.MapFS)
		label  string
		err    error // the copy's exec
		want   error
		runs   bool // the copy is run for its label
	}{
		{"attach <unknown>, what the copy runs under both", "<unknown>", nil, AppArmorStack, nil, nil, true},
		{"attach the path, what the copy runs under both", "", nil, AppArmorStack, nil, nil, true},
		{"attach <unknown>, what the copy runs unconfined", "<unknown>", nil, "unconfined", nil, ErrNoProfile, true},
		{"attach the path, what the copy runs unconfined", "", nil, "unconfined", nil, ErrNoProfile, true},
		{"under bwrap's profile alone", "<unknown>", nil, "smolcorp-bwrap (enforce)", nil, ErrNoProfile, true},
		{"under the child alone", "<unknown>", nil, "smolcorp-bwrap-child (enforce)", nil, ErrNoProfile, true},
		{"the stack complaining", "<unknown>", nil, "smolcorp-bwrap//&smolcorp-bwrap-child (complain)", nil, ErrNoProfile, true},
		{"a third profile in the stack", "<unknown>", nil, "smolcorp-bwrap//&smolcorp-bwrap-child//&other (enforce)", nil, ErrNoProfile, true},
		{"the stack in another order", "<unknown>", nil, "smolcorp-bwrap-child//&smolcorp-bwrap (enforce)", nil, ErrNoProfile, true},
		{"the label with its line end", "<unknown>", nil, AppArmorStack + "\n", nil, ErrNoProfile, true},
		{"no label printed", "<unknown>", nil, "", nil, ErrNoProfile, true},
		// An exec that fails is refused even with the right label printed before it failed.
		{"the copy can't make its namespace", "<unknown>", nil, AppArmorStack, cantRun, ErrNoProfile, true},
		{"the copy can't be run at all", "<unknown>", nil, AppArmorStack, fs.ErrPermission, ErrNoProfile, true},
		// The attach file still refuses what it does say: another path, or a pattern, even under the right label.
		{"attach /usr/bin/bwrap, the right label", PackagedBwrap, nil, AppArmorStack, nil, ErrNoProfile, false},
		{"attach a pattern, the right label", "/usr/local/lib/smolcorp/*", nil, AppArmorStack, nil, ErrNoProfile, false},
		{"attach unknown otherwise spelt", "unknown", nil, AppArmorStack, nil, ErrNoProfile, false},
		{"the attach file unreadable", "-", nil, AppArmorStack, nil, ErrNoProfile, false},
		{"the child not loaded", "<unknown>", func(r fstest.MapFS) { delete(r, policyFolder+"/smolcorp-bwrap-child.13/name") },
			AppArmorStack, nil, ErrNoProfile, false},
		{"bwrap's profile complaining", "<unknown>", func(r fstest.MapFS) {
			r[policyFolder+"/smolcorp-bwrap.12/mode"] = &fstest.MapFile{Data: []byte("complain\n")}
		}, AppArmorStack, nil, ErrNoProfile, false},
		{"the restriction off, the copy unrunnable", "<unknown>", func(r fstest.MapFS) {
			r[restrictFile] = &fstest.MapFile{Data: []byte("0\n")}
		}, "", cantRun, nil, false},
		{"no restriction on this kernel, the copy unrunnable", "", func(r fstest.MapFS) { delete(r, restrictFile) },
			"", cantRun, nil, false},
	} {
		sys, root := ownComputer()
		switch c.attach {
		case "":
		case "-":
			delete(root, policyFolder+"/smolcorp-bwrap.12/attach")
		default:
			root[policyFolder+"/smolcorp-bwrap.12/attach"] = &fstest.MapFile{Data: []byte(c.attach + "\n")}
		}
		if c.change != nil {
			c.change(root)
		}
		calls := labelIs(t, c.label, c.err)
		err := checkAll(sys, root)
		if c.want == nil && err != nil || c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
		if ran := *calls > 0; ran != c.runs {
			t.Errorf("%s: the copy run for its label: %v, want %v", c.name, ran, c.runs)
		}
		if err != nil && !strings.Contains(RefusalFor(err), SandboxCommand()) {
			t.Errorf("%s: the owner reads %q", c.name, RefusalFor(err))
		}
	}
}

// The owner's words say exactly what the command grants (RT126L1APPARMOR1 B1, RT126L1APPARMOR1R2 B3): who can run the
// copy; that what it starts gets no capabilities and can make no further user namespaces, so hires can't use it; and
// that a program of the owner's can load its own code into the copy and use the copy's capabilities. Then the one
// command, whole, to run once.
func TestTheRefusalSaysWhyAndTheOneCommand(t *testing.T) {
	saved := ownerGroup
	t.Cleanup(func() { ownerGroup = saved })
	ownerGroup = func() int { return 1000 }
	got := RefusalFor(ErrNoProfile)
	want := "Hires can't start on this computer until Smolcorp's own bubblewrap is set up. Hires on this computer start " +
		"in a sandbox that Smolcorp's own copy of bubblewrap makes. This copies your bubblewrap to " +
		"/usr/local/lib/smolcorp/bwrap, where only root can change it, and only root and your account's group (gid " +
		"1000) can run it. Where AppArmor limits user namespaces (Ubuntu 23.10 and later), it also loads a profile that " +
		"lets that copy make them, and every program the copy starts runs under a second profile that gives it no " +
		"capabilities and lets it make no user namespaces of its own, so hires can't use the copy to get past that " +
		"limit, and no other account can run it. A program of yours can: by loading its own code into the copy (with " +
		"LD_PRELOAD, for one), it can use the copy's capabilities in the namespaces it makes, which the limit otherwise " +
		"refuses any program without a profile. It changes no other setting. Run this once in a terminal (it asks for " +
		"your password): " + sandboxCommand(1000)
	if got != want {
		t.Errorf("the refusal:\n%s\nwant\n%s", got, want)
	}
	if RefusalFor(ErrNoOwnBwrap) != want || RefusalFor(errors.Join(ErrNoBwrap)) != BwrapRefusal {
		t.Error("each error doesn't get its words")
	}
	// Round 1's words: "that copy, and nothing else" read as "nothing else on my computer gets this".
	if strings.Contains(SandboxWhy(), "nothing else") || strings.Contains(AppArmorProfile, "nothing else") {
		t.Error("the words still say nothing else gets the grant")
	}
	// RT126L1APPARMOR1R2 B3: round 2's comment said a program that runs the copy gets no capabilities. Code it loads
	// into the copy does get them; the code's own comment says so, as the owner's words do (the installer's is
	// scripts/linuxtmux's row). Only its comment lines are read: apparmor.go's words above hold LD_PRELOAD too. The
	// source is built into the test, so the test runs where it isn't (FXL1APPARMOR1VM2 B2: the VM's pack).
	var comments []string
	for _, line := range strings.Split(apparmorSource, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "//") {
			comments = append(comments, strings.TrimPrefix(line, "//"))
		}
	}
	text := strings.Join(strings.Fields(strings.Join(comments, " ")), " ")
	if strings.Contains(text, "a program that runs the copy gets a namespace with no capabilities") ||
		!strings.Contains(text, "LD_PRELOAD") {
		t.Error("apparmor.go doesn't say a program of the owner's can use the copy's own capabilities")
	}
}

// apparmorSource is apparmor.go, as it was built.
//
//go:embed apparmor.go
var apparmorSource string

// The check's exec (FXL1APPARMOR1VM2 B1), on a stand-in bwrap, a script: it gets labelArgs exactly, from "/", with
// no environment but PATH, and what it prints is the label without the kernel's line end; a bwrap that fails gives
// its own words, and one that can't run an error. The real copy is never run here: that is VM run 3's. Where there
// is no copy, the start's own copysLabel says it ran BwrapPath.
func TestTheCopysLabelIsReadFromWhatItRuns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in bwrap is a shell script")
	}
	dir := t.TempDir()
	t.Setenv("SMOLCORP_STANDIN_LEAK", "leaked")
	standIn := func(name, body string) string {
		at := filepath.Join(dir, name)
		if err := os.WriteFile(at, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
			t.Fatal(err)
		}
		return at
	}
	got := filepath.Join(dir, "got")
	ok := standIn("bwrap", `for a in "$@"; do printf '%s\n' "$a"; done > '`+got+`'
printf 'dir=%s path=%s leak=%s\n' "$(pwd)" "$PATH" "$SMOLCORP_STANDIN_LEAK" >> '`+got+`'
printf '%s\n' 'smolcorp-bwrap//&smolcorp-bwrap-child (enforce)'
`)
	label, err := labelOf(ok)
	if err != nil || label != AppArmorStack {
		t.Errorf("the label: %q, %v, want %q", label, err, AppArmorStack)
	}
	words, _ := os.ReadFile(got)
	if want := strings.Join(labelArgs, "\n") + "\ndir=/ path=/usr/bin:/bin leak=\n"; string(words) != want {
		t.Errorf("the stand-in got:\n%s\nwant\n%s", words, want)
	}
	if !slices.Equal(labelArgs[:6], []string{"--die-with-parent", "--unshare-user", "--ro-bind", "/", "/", "--"}) ||
		!strings.Contains(labelArgs[len(labelArgs)-1], "/proc/self/attr/apparmor/current") {
		t.Errorf("the no-op isn't run in a user namespace of its own, reading its own label: %q", labelArgs)
	}
	refused := standIn("refused", "echo 'bwrap: setting up uid map: Permission denied' >&2\nexit 1\n")
	if _, err := labelOf(refused); err == nil || !strings.Contains(err.Error(), "setting up uid map: Permission denied") {
		t.Errorf("a bwrap that fails: %v", err)
	}
	// RT126L1APPARMOR1VM2 F2: the right label, then a failure that says nothing, is still a failure.
	silent := standIn("silent", "printf '%s\\n' 'smolcorp-bwrap//&smolcorp-bwrap-child (enforce)'\nexit 3\n")
	if label, err := labelOf(silent); err == nil || label != "" {
		t.Errorf("a bwrap that printed the label and failed silently: %q, %v", label, err)
	}
	if _, err := labelOf(filepath.Join(dir, "missing")); err == nil {
		t.Error("a bwrap that can't run gave a label")
	}
	if _, err := os.Lstat(BwrapPath); errors.Is(err, fs.ErrNotExist) {
		if _, err := copysLabel(); err == nil || !strings.Contains(err.Error(), BwrapPath) {
			t.Errorf("the start's copysLabel didn't run %s: %v", BwrapPath, err)
		}
	}
}

// RT126L1APPARMOR1VM2 F1: a bwrap that hangs while something it started holds its output open is stopped at the
// timeout, and the check stops waiting a second later, not when that child ends. The stand-in's sleep is the child;
// it outlives the call by about 9 s, harmlessly.
func TestTheCopysLabelCheckDoesntWaitForAChildThatHangs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in bwrap is a shell script")
	}
	hangs := filepath.Join(t.TempDir(), "bwrap")
	if err := os.WriteFile(hangs, []byte("#!/bin/sh\nsleep 20\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	label, err := labelOf(hangs)
	took := time.Since(start)
	if err == nil || label != "" {
		t.Errorf("a bwrap that hung gave %q, %v", label, err)
	}
	if took < 9*time.Second || took > 15*time.Second {
		t.Errorf("a bwrap that hung, its child holding the output, took %v, want about 11s", took)
	}
}

// `smolcorp sandbox check`'s exits, which the installer reads: 0 when hires can start, 3 when the bubblewrap package
// is missing (the command can't help), 1 with the words and the command otherwise.
func TestTheChecksExitTellsTheInstallerWhatToDo(t *testing.T) {
	for _, c := range []struct {
		err  error
		code int
		out  string
	}{
		{nil, 0, ""},
		{fmt.Errorf("%w: gone", ErrNoBwrap), 3, BwrapRefusal + "\n"},
		{fmt.Errorf("%w: not loaded", ErrNoProfile), 1, SandboxRefusal() + "\n"},
		{errStaleBwrap, 1, RefusalFor(errStaleBwrap) + "\n"},
	} {
		var out, errs bytes.Buffer
		if code := checkExit(c.err, &out, &errs); code != c.code || out.String() != c.out ||
			c.err != nil && errs.String() != c.err.Error()+"\n" {
			t.Errorf("%v: %d %q %q", c.err, code, out.String(), errs.String())
		}
	}
}

// `smolcorp sandbox command` and `check` are Linux's: elsewhere they say so and do nothing.
func TestTheInstallersCommandsAreLinuxOnes(t *testing.T) {
	var out, errs bytes.Buffer
	code := Run([]string{"command"}, &out, &errs)
	if runtime.GOOS != "linux" {
		if code != 2 || out.Len() != 0 || !strings.Contains(errs.String(), "for Linux only") {
			t.Errorf("command off Linux: %d %q %q", code, out.String(), errs.String())
		}
		if Run([]string{"check"}, &out, &errs) != 2 {
			t.Error("check off Linux didn't refuse")
		}
		return
	}
	if code != 0 || out.String() != SandboxCommand()+"\n" {
		t.Errorf("command: %d %q", code, out.String())
	}
}

// stackedUnderBoth is whether an AppArmor label (as /proc/self/attr/current says it) is AppArmorName and AppArmorChild
// stacked, and both enforcing: "smolcorp-bwrap//&smolcorp-bwrap-child (enforce)".
func stackedUnderBoth(label string) bool {
	names, mode, ok := strings.Cut(label, " ")
	parts := strings.Split(names, "//&")
	slices.Sort(parts)
	return ok && mode == "(enforce)" && slices.Equal(parts, []string{AppArmorName, AppArmorChild})
}

// The Linux row's reading of the label: round 1's "smolcorp-bwrap (unconfined)" (B1, as built) isn't the stack, nor
// is either profile alone, nor the stack complaining.
func TestTheLabelInsideIsReadAsTheStack(t *testing.T) {
	for label, want := range map[string]bool{
		"smolcorp-bwrap//&smolcorp-bwrap-child (enforce)":  true,
		"smolcorp-bwrap-child//&smolcorp-bwrap (enforce)":  true,
		"smolcorp-bwrap (unconfined)":                      false,
		"smolcorp-bwrap (enforce)":                         false,
		"smolcorp-bwrap-child (enforce)":                   false,
		"smolcorp-bwrap//&smolcorp-bwrap-child (complain)": false,
		"unconfined": false,
		"smolcorp-bwrap//&smolcorp-bwrap-child//&other (enforce)": false,
	} {
		if stackedUnderBoth(label) != want {
			t.Errorf("%q read as the stack: %v", label, !want)
		}
	}
}
