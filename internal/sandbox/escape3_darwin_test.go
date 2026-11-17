package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// MACLAUNCH1, the Mac escape table III (SIERRA's ruling, 15 Nov: "launchctl / XPC to the owner's GUI session from a
// desk must stay denied"). The profile denies new launchd jobs (job-creation), opening apps (lsopen), Apple Events and
// every Mach service off its list (sbpl.go); these rows prove what that leaves a desk, under the real
// /usr/bin/sandbox-exec and the real profile, on SA-M4's stand-in hire (escape_darwin_test.go): a stand-in home,
// support folder and repo in the test's temp folder, the key FAKEKEY.
//
// Each launchd row is checked from outside as well: launchd is asked whether the stand-in job got loaded, in the
// owner's GUI and user domains, and the stand-in's program must not have run. At the end, a job that somehow got
// loaded is booted out (and the test fails), and no process of the stand-in's may be left running. The stand-in job
// only touches a file in the test's folder.
//
// On this Mac nothing is loaded into the owner's session, not even for a control: the controls here are the calls
// that load nothing (launchd answers launchctl from outside; a running agent's kickstart without -k is a no-op; the
// writes on the stand-in's label, which isn't loaded, say "not found"). And nothing goes through LaunchServices or
// Gatekeeper (VICTOR, 15 Nov, after a stand-in app's first run put Gatekeeper's "damaged" dialog in front of the
// owner): no app is built, opened or run here, inside or outside; the one open row is inside the profile, on this
// run's stand-in path, which doesn't exist (openRefusal refuses anything else, an app's name first). The controls that
// load a stand-in job, set a variable in the session or open a stand-in app, the writes against a loaded job, and the
// Apple Event row (which, were the sandbox not to stop it first, could ask for Automation), run only on the Mac Studio
// runner, with SMOLCORP_STUDIO_ROWS=1. Two guards keep it so: TestOnAMacNoRowHereOpensAnAppUngated reads every test
// file of the package as syntax, and startRefusal refuses at run time in every start helper (startFunnels).
//
// What a desk can still do, as these rows find (MACLAUNCH1's report): read the owner's GUI domain with `launchctl
// print` (launchGaps; SIERRA, 15 Nov: a documented gap, pinned read-only), kickstart a running agent without -k (the
// same no-op as from outside), and run `launchctl asuser` and `bsexec`, whose program stays in its sandbox. Here,
// bootout, remove and kill are only shown to load nothing (launchd says "not found" to inside and outside alike on
// the stand-in's label); whether they are refused against a loaded job is the Studio's row. No row shows a print's
// output: a job's environment block is only counted.

// studioRowsVar runs the rows that load a stand-in job or could ask for a permission: the Mac Studio runner's only.
const studioRowsVar = "SMOLCORP_STUDIO_ROWS"

// launchGaps are the launchctl calls a desk isn't refused, each with why it stays open for now. One that becomes
// refused fails its row, so the list stays true.
var launchGaps = map[string]string{
	"print gui": "launchd answers a read of the owner's GUI domain (its services, their pids, its environment) from " +
		"inside; no Seatbelt operation shuts it (deny process-info* breaks launchctl itself), and the read also goes " +
		"over launchd's bootstrap port. It starts nothing, and the plists in ~/Library/LaunchAgents are readable by " +
		"file anyway. SIERRA, 15 Nov: a documented gap, pinned read-only (TestOnAMacADeskReadsTheOwnersGUIDomainAndLoadsNothingIntoIt)",
}

// launchDesk is the stand-in hire with a stand-in launchd job it may try to start. The job's label is new each run.
type launchDesk struct {
	*escapeDesk
	uid, label, plist, ran string
}

// standInLabelPattern is a stand-in's label: the test process's pid, then the run's nanoseconds in base 36.
var standInLabelPattern = regexp.MustCompile(`^local\.smolcorp-standin\.([0-9]+)\.[0-9a-z]+$`)

func newLaunchDesk(t *testing.T) *launchDesk {
	t.Helper()
	d := &launchDesk{escapeDesk: newEscapeDesk(t), uid: strconv.Itoa(os.Getuid())}
	d.label = fmt.Sprintf("local.smolcorp-standin.%d.%s", os.Getpid(), strconv.FormatInt(time.Now().UnixNano(), 36))
	folder := filepath.Join(d.base, "launch")
	d.ran = filepath.Join(folder, "ran")
	d.plist = filepath.Join(folder, d.label+".plist")
	mustWrite(t, d.plist, plistOf(`<key>Label</key><string>`+d.label+`</string>`+
		`<key>ProgramArguments</key><array><string>/usr/bin/touch</string><string>`+d.ran+`</string></array>`+
		`<key>RunAtLoad</key><true/>`))
	t.Cleanup(func() { d.leftNothing(t) })
	return d
}

func plistOf(dict string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n" +
		`<plist version="1.0"><dict>` + dict + `</dict></plist>` + "\n"
}

// runOutside runs program as the core would, outside any sandbox, for at most twenty seconds. startRefusal comes
// first, told the start isn't in a desk.
func runOutside(t *testing.T, program string, args ...string) ran {
	t.Helper()
	if err := startRefusal(program, args, false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := disclaim.Cmd(exec.CommandContext(ctx, program, args...))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := ran{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		t.Fatalf("%s %q didn't end within twenty seconds", program, args)
	case errors.As(err, &exitErr):
		r.status = exitErr.ExitCode()
	case err != nil:
		t.Fatalf("%s %q: %v", program, args, err)
	}
	return r
}

// launchdNotFound is launchctl's status when launchd has no such service (macOS 26).
const launchdNotFound = 113

// loadedIn is the domains of the owner's session that have the stand-in job, as launchd says from outside.
func (d *launchDesk) loadedIn(t *testing.T) []string {
	t.Helper()
	var in []string
	for _, domain := range []string{"gui/" + d.uid, "user/" + d.uid} {
		switch r := runOutside(t, "/bin/launchctl", "print", domain+"/"+d.label); {
		case r.status == 0:
			in = append(in, domain)
		case r.status != launchdNotFound || !strings.Contains(r.stderr, "Could not find service"):
			t.Errorf("launchctl print %s/%s from outside: %+v", domain, d.label, r)
		}
	}
	return in
}

// nothingLoaded fails the row if the stand-in job is in the owner's session, or its program ran.
func (d *launchDesk) nothingLoaded(t *testing.T, row string) {
	t.Helper()
	if in := d.loadedIn(t); len(in) > 0 {
		t.Errorf("%s: the stand-in job %s got loaded in %q", row, d.label, in)
	}
	if _, err := os.Lstat(d.ran); err == nil {
		t.Errorf("%s: the stand-in job's program ran", row)
	}
}

// leftNothing ends every test here: a stand-in job still loaded is booted out (and the test fails), and no process
// with the stand-in's label or folder in its arguments may be left running.
func (d *launchDesk) leftNothing(t *testing.T) {
	t.Helper()
	for _, domain := range d.loadedIn(t) {
		t.Errorf("the stand-in job %s was still loaded in %s at the end: booted out now", d.label, domain)
		if r := runOutside(t, "/bin/launchctl", "bootout", domain+"/"+d.label); r.status != 0 {
			t.Errorf("booting out %s/%s: %+v", domain, d.label, r)
		}
	}
	for _, pattern := range []string{d.label, d.base} {
		if r := runOutside(t, "/usr/bin/pgrep", "-fl", pattern); r.status != 1 {
			t.Errorf("left running, matching %s: %+v", pattern, r)
		}
	}
}

// refused is whether launchctl inside said no: a status other than 0, or the words of `load`, which ends with 0
// when its load fails (macOS 26). For every launchd row, what launchd has afterwards counts as well.
func refused(r ran) bool {
	return r.status != 0 || strings.Contains(r.stderr, "Load failed")
}

// pidOf is the "pid = " line of a launchctl print's output, or "" when the job isn't running.
func pidOf(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if pid, ok := strings.CutPrefix(strings.TrimSpace(line), "pid = "); ok {
			return pid
		}
	}
	return ""
}

// The owner's session as a whole, from inside: `launchctl list` is refused, and asuser and bsexec run nothing outside
// the sandbox. The GUI domain's read and the writes are TestOnAMacADeskReadsTheOwnersGUIDomainAndLoadsNothingIntoIt's.
func TestOnAMacAHiresLaunchctlRunsNothingOutsideItsSandbox(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newLaunchDesk(t)
	gui := "gui/" + d.uid

	// The positive controls, from outside: launchd answers launchctl, and the stand-in job isn't there to begin with.
	if r := runOutside(t, "/bin/launchctl", "print", gui); r.status != 0 || !strings.HasPrefix(r.stdout, gui+" = {") {
		t.Fatalf("control: launchctl print %s from outside: status %d, %q", gui, r.status, r.stderr)
	}
	d.nothingLoaded(t, "control: before")

	// Its status and words only: were it let through, it would name the owner's jobs.
	if r := d.inside(t, nil, "/bin/launchctl", "list"); !refused(r) {
		t.Errorf("launchctl list from inside wasn't refused: status %d, %q", r.status, r.stderr)
	}

	// asuser and bsexec run a program in a launchd domain's name, but the program is launchctl's own child: still in
	// the sandbox. The control: the same program reads what the desk may read.
	for _, words := range [][]string{{"asuser", d.uid}, {"bsexec", "1"}} {
		if r := d.inside(t, nil, "/bin/launchctl", append(words, "/bin/cat", d.key)...); r.status == 0 ||
			strings.Contains(r.stdout, escapeKey) || !strings.Contains(r.stderr, "Operation not permitted") {
			t.Errorf("launchctl %s, the key: %+v", words[0], r)
		}
		if r := d.inside(t, nil, "/bin/launchctl", append(words, "/bin/cat", filepath.Join(d.own, "BRIEF.md"))...); r.status != 0 || r.stdout != "the brief" {
			t.Errorf("control: launchctl %s, its brief: %+v", words[0], r)
		}
	}
	d.nothingLoaded(t, "the end")
}

// SIERRA's ruling (15 Nov): `launchctl print` of the owner's GUI domain is a documented gap, not denied outright (the
// read also goes over launchd's bootstrap port). This row pins what this Mac can show of it: from inside, the print
// works (its status is all that's looked at); the writes that load, start or set something, and stop and unsetenv,
// are refused; bootout, remove and kill on the stand-in's label, which isn't loaded, load nothing (whether they are
// refused against a loaded job is TestOnAMacTheLaunchdRefusalsAreTheSandboxs', on the Studio). A change that lets a
// pinned refusal through fails here. No print's output is ever shown: of a job's, only whether it has an environment
// block, counted.
func TestOnAMacADeskReadsTheOwnersGUIDomainAndLoadsNothingIntoIt(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newLaunchDesk(t)
	gui := "gui/" + d.uid
	d.nothingLoaded(t, "control: before")

	// The read: allowed, with its reason (launchGaps).
	r := d.inside(t, nil, "/bin/launchctl", "print", gui)
	switch why := launchGaps["print gui"]; {
	case r.status != 0 && why != "":
		t.Errorf("launchctl print %s is refused from inside now (status %d, %q): take it off launchGaps", gui, r.status, r.stderr)
	case r.status == 0 && why == "":
		t.Errorf("launchctl print %s from inside wasn't refused", gui)
	case r.status == 0:
		t.Logf("KNOWN GAP, launchctl print %s: %s", gui, why)
	}

	// What the read shows of a job: does the print of one of Apple's always-running agents, from inside, have an
	// environment block? Nothing is loaded for it, and the same print from outside is the control.
	for _, job := range []string{gui + "/com.apple.Dock.agent", "user/" + d.uid + "/com.apple.cfprefsd.xpc.agent"} {
		out := runOutside(t, "/bin/launchctl", "print", job)
		if out.status != 0 {
			t.Errorf("control: launchctl print %s from outside: status %d", job, out.status)
			continue
		}
		in := d.inside(t, nil, "/bin/launchctl", "print", job)
		exact, all := environmentBlocks(in.stdout)
		outExact, outAll := environmentBlocks(out.stdout)
		t.Logf("launchctl print %s from inside: status %d; an \"environment = {\" block: %v (%d, and %d \"… environment = {\" "+
			"blocks in all; from outside %d and %d)", job, in.status, exact > 0, exact, all, outExact, outAll)
	}

	// The writes that load or start a job are refused, and nothing gets loaded. No `load -w` and no enable or disable,
	// even refused: were one let through, it would write the stand-in's label into launchd's override database for
	// good. Both kickstarts name the stand-in, which isn't loaded, and launchd says the same from outside (the
	// control), so here they only show that nothing starts; a loaded job's kickstarts are the Studio's.
	if r := runOutside(t, "/bin/launchctl", "kickstart", gui+"/"+d.label); r.status != launchdNotFound {
		t.Errorf("control: kickstart of the missing stand-in from outside: status %d, %q", r.status, r.stderr)
	}
	for _, row := range []struct {
		row  string
		args []string
	}{
		{"submit", []string{"submit", "-l", d.label, "--", "/usr/bin/touch", d.ran}},
		{"bootstrap", []string{"bootstrap", gui, d.plist}},
		{"load", []string{"load", d.plist}},
		{"kickstart", []string{"kickstart", gui + "/" + d.label}},
		{"kickstart -k", []string{"kickstart", "-k", gui + "/" + d.label}},
	} {
		if r := d.inside(t, nil, "/bin/launchctl", row.args...); !refused(r) {
			t.Errorf("launchctl %s from inside wasn't refused: status %d, %q", row.row, r.status, r.stderr)
		}
		d.nothingLoaded(t, "launchctl "+row.row)
	}

	// The writes that end or signal a job (RT126MACLAUNCH1 B3). stop is refused from inside before launchd looks the
	// label up: from outside, the same call says the label isn't there (the control), from inside "Not privileged".
	if r := runOutside(t, "/bin/launchctl", "stop", d.label); r.status == 0 || strings.Contains(r.stderr, "Not privileged") {
		t.Errorf("control: stop of the missing stand-in from outside: status %d, %q", r.status, r.stderr)
	}
	if r := d.inside(t, nil, "/bin/launchctl", "stop", d.label); r.status == 0 || !strings.Contains(r.stderr, "Not privileged") {
		t.Errorf("launchctl stop from inside wasn't refused (\"Not privileged\"): status %d, %q", r.status, r.stderr)
	}
	d.nothingLoaded(t, "launchctl stop")
	// bootout, remove and kill on the missing label: launchd looks the label up for the desk as for the owner and
	// says "not found" to both (outside, the control), so these only show that nothing gets loaded. Against a loaded
	// job, the Studio's row.
	for _, row := range []struct {
		row  string
		args []string
	}{
		{"bootout gui", []string{"bootout", gui + "/" + d.label}},
		{"bootout user", []string{"bootout", "user/" + d.uid + "/" + d.label}},
		{"remove", []string{"remove", d.label}},
		{"kill SIGCONT", []string{"kill", "SIGCONT", gui + "/" + d.label}},
	} {
		out := runOutside(t, "/bin/launchctl", row.args...)
		in := d.inside(t, nil, "/bin/launchctl", row.args...)
		if out.status == 0 {
			t.Errorf("control: launchctl %s of the missing stand-in from outside: %+v", row.row, out)
		}
		if in.status == 0 {
			t.Errorf("launchctl %s of the missing stand-in from inside went through: %+v", row.row, in)
		}
		d.nothingLoaded(t, "launchctl "+row.row)
		t.Logf("launchctl %s of the missing stand-in: outside status %d, inside status %d (not found, loads nothing; not a refusal)",
			row.row, out.status, in.status)
	}

	// A variable set in the owner's session would reach every program launchd starts for the owner after; one unset
	// would take one away. Both are refused from inside. The control unsets, from outside, a name that was never set:
	// it goes through, and changes nothing.
	name := "SMOLCORP_STANDIN_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	if r := d.inside(t, nil, "/bin/launchctl", "setenv", name, "1"); r.status == 0 || !strings.Contains(r.stderr, "Not privileged") {
		t.Errorf("launchctl setenv from inside: status %d, %q", r.status, r.stderr)
	}
	if r := runOutside(t, "/bin/launchctl", "getenv", name); r.stdout != "" {
		t.Errorf("the owner's session has %s=%q", name, r.stdout)
		runOutside(t, "/bin/launchctl", "unsetenv", name)
	}
	if r := d.inside(t, nil, "/bin/launchctl", "unsetenv", name); r.status == 0 || !strings.Contains(r.stderr, "Not privileged") {
		t.Errorf("launchctl unsetenv from inside wasn't refused (\"Not privileged\"): status %d, %q", r.status, r.stderr)
	}
	if r := runOutside(t, "/bin/launchctl", "unsetenv", name); r.status != 0 {
		t.Errorf("control: unsetenv of a name never set, from outside: %+v", r)
	}

	// A job by file, for the owner's next login (TestOnAMacAHireCantPutAJobInTheOwnersLaunchAgents has the rest).
	agents := filepath.Join(d.home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o700); err != nil {
		t.Fatal(err)
	}
	d.check(t, []escapeRow{
		{"control: a file in ~/Library beside it", attempt{"create", filepath.Join(d.home, "Library", "x"), ""}, "ok"},
		{"a job's plist in ~/Library/LaunchAgents", attempt{"create", filepath.Join(agents, d.label+".plist"), ""}, "EPERM"},
	})
	d.nothingLoaded(t, "the end")
}

// environmentBlocks counts, in launchctl print's output, the lines that open an "environment = {" block, and those
// that open any "… environment = {" block (inherited, default): the counts only, so no variable is ever shown.
func environmentBlocks(out string) (exact, all int) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "environment = {") {
			all++
			if line == "environment = {" {
				exact++
			}
		}
	}
	return exact, all
}

// A running agent's kickstart (without -k) is a no-op from outside: from inside, it must be refused or be the same
// no-op, never a restart. com.apple.cfprefsd.xpc.agent runs in every session, with no window.
func TestOnAMacAHiresKickstartOfARunningAgentRestartsNothing(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newLaunchDesk(t)
	agent := "user/" + d.uid + "/com.apple.cfprefsd.xpc.agent"
	pidNow := func() string {
		t.Helper()
		r := runOutside(t, "/bin/launchctl", "print", agent)
		pid := pidOf(r.stdout)
		if pid == "" {
			t.Skipf("SKIPPED, %s isn't running here (status %d): no running agent to kickstart", agent, r.status)
		}
		return pid
	}
	before := pidNow()
	if r := runOutside(t, "/bin/launchctl", "kickstart", agent); r.status != 0 {
		t.Fatalf("control: kickstart of %s from outside: %+v", agent, r)
	}
	r := d.inside(t, nil, "/bin/launchctl", "kickstart", agent)
	if after := pidNow(); after != before {
		t.Errorf("kickstart from inside (%+v) restarted %s: pid %s, was %s", r, agent, after, before)
	}
	t.Logf("kickstart of a running agent from inside: status %d, %q", r.status, r.stderr)
}

// ~/Library/LaunchAgents is where a job for the owner's next login would go by file, without launchctl: the Policy
// write-denies it and guards the folders above it against a rename.
func TestOnAMacAHireCantPutAJobInTheOwnersLaunchAgents(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newLaunchDesk(t)
	library := filepath.Join(d.home, "Library")
	agents := filepath.Join(library, "LaunchAgents")
	if err := os.MkdirAll(agents, 0o700); err != nil {
		t.Fatal(err)
	}
	d.check(t, []escapeRow{
		{"control: a file in ~/Library beside it", attempt{"create", filepath.Join(library, "x"), ""}, "ok"},
		{"a job's plist in ~/Library/LaunchAgents", attempt{"create", filepath.Join(agents, d.label+".plist"), ""}, "EPERM"},
		{"a link there to the stand-in's plist", attempt{"symlink", filepath.Join(agents, "l.plist"), d.plist}, "EPERM"},
		{"~/Library/LaunchAgents moved aside", attempt{"rename", agents, filepath.Join(library, "old")}, "EPERM"},
		{"~/Library moved aside", attempt{"rename", library, filepath.Join(d.home, "old")}, "EPERM"},
	})
	if entries, err := os.ReadDir(agents); err != nil || len(entries) != 0 {
		t.Errorf("~/Library/LaunchAgents has %v (%v)", entries, err)
	}
}

// Opening an app (LaunchServices) from inside, on this Mac: only `open` of this run's stand-in path, which doesn't
// exist, and which the profile must refuse or fail before anything is opened; nothing here builds, opens or runs an
// app (see above). The profile's `(deny lsopen)`, after every allow, is the audit's
// (TestOnAMacNoMachServiceOnTheListStartsAProcessForADesk). The Studio's rows open a stand-in app from outside as the
// control, run osascript with no Apple Event and with one.
func TestOnAMacAHireCantOpenAnAppOrSendAnAppleEvent(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newLaunchDesk(t)
	if r := d.openApp(t, true, d.appAt("missing")); r.status == 0 || r.stderr == "" {
		t.Errorf("open -g -a of a missing app from inside: %+v", r)
	}

	t.Run("the Studio's rows", func(t *testing.T) {
		if os.Getenv(studioRowsVar) != "1" {
			t.Skipf("SKIPPED here: nothing may go through LaunchServices or Gatekeeper on this Mac (VICTOR, 15 Nov); "+
				"%s=1 on the Mac Studio runner builds a stand-in app, opens it from outside and from inside, and sends "+
				"an Apple Event from inside", studioRowsVar)
		}
		app := standInApp(t, d)
		// The control: from outside, LaunchServices opens the stand-in app (it runs /usr/bin/true and ends).
		if r := d.openApp(t, false, app); r.status != 0 {
			t.Errorf("control: open of the stand-in app from outside: %+v", r)
		}
		if r := d.openApp(t, true, app); r.status == 0 || r.stderr == "" {
			t.Errorf("open -g -a the stand-in app from inside: %+v", r)
		}
		if r, _ := appleEvent(t, d, "return 1"); r.status != 0 || r.stdout != "1\n" {
			t.Errorf("control: osascript with no Apple Event, from inside: %+v", r)
		}
		// From inside, an Apple Event to the Finder: refused by the sandbox before it is sent, so never by TCC
		// (-1743, which would mean it got as far as asking), and the sandbox logs it.
		r, denied := appleEvent(t, d, `tell application "Finder" to get name`)
		if r.status == 0 || strings.Contains(r.stderr, "-1743") || strings.Contains(r.stdout, "Finder") {
			t.Errorf("an Apple Event from inside: %+v", r)
		}
		if !strings.Contains(denied, "appleevent-send") && !strings.Contains(denied, "com.apple.coreservices.appleevents") {
			t.Errorf("the sandbox didn't refuse the Apple Event before it was sent: %q, osascript %+v", denied, r)
		}
	})
}

// openRefusal is why openApp won't open app (RT126MACLAUNCH1 B2). `open -a` takes an app's name as well as a path,
// and LaunchServices looks a name up (Calculator, TextEdit), so app must be an absolute, clean path whose base name is
// a stand-in label of this test process (local.smolcorp-standin.<pid>.<nanos>.app); and on this Mac
// (SMOLCORP_STUDIO_ROWS unset) one that doesn't exist, since LaunchServices and Gatekeeper would look at it and may put
// a dialog in front of the owner.
func openRefusal(app string) error {
	label, isApp := strings.CutSuffix(filepath.Base(app), ".app")
	m := standInLabelPattern.FindStringSubmatch(label)
	switch {
	case !filepath.IsAbs(app) || filepath.Clean(app) != app:
		return fmt.Errorf("%q isn't an absolute, clean path: open -a would look it up as an app's name, through LaunchServices", app)
	case !isApp || m == nil || m[1] != strconv.Itoa(os.Getpid()):
		return fmt.Errorf("%q isn't a stand-in of this run (local.smolcorp-standin.%d.<nanos>.app): only one may be opened", app, os.Getpid())
	case os.Getenv(studioRowsVar) == "1":
		return nil
	}
	if _, err := os.Lstat(app); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s exists (%v): opening it goes through LaunchServices and Gatekeeper, which only the Mac Studio's rows may (%s=1)", app, err, studioRowsVar)
	}
	return nil
}

// appAt is the stand-in app's path in folder of the test's: this run's label, with .app. Only standInApp builds it,
// on the Studio.
func (d *launchDesk) appAt(folder string) string {
	return standInAppAt(filepath.Join(d.base, folder), d.label)
}

// standInAppAt is a stand-in app's path in folder: label, with .app.
func standInAppAt(folder, label string) string {
	return filepath.Join(folder, label+".app")
}

// openApp is `open -g -j -a app`, from inside the desk's sandbox or outside, app being this desk's stand-in: the only
// way any row here runs /usr/bin/open (TestOnAMacNoRowHereOpensAnAppUngated and startRefusal check it).
func (d *launchDesk) openApp(t *testing.T, inside bool, app string) ran {
	t.Helper()
	if err := openRefusal(app); err != nil {
		t.Fatal(err)
	}
	if label := strings.TrimSuffix(filepath.Base(app), ".app"); label != d.label {
		t.Fatalf("%s isn't this desk's stand-in (%s)", app, d.label)
	}
	if inside {
		return d.inside(t, nil, "/usr/bin/open", "-g", "-j", "-a", app)
	}
	return runOutside(t, "/usr/bin/open", "-g", "-j", "-a", app)
}

// standInApp builds a stand-in app for the Studio's rows only: background-only, its program /usr/bin/true, the bundle
// signed ad hoc (a copied program alone in a bundle reads to Gatekeeper as damaged: the dialog of 15 Nov).
func standInApp(t *testing.T, d *launchDesk) string {
	t.Helper()
	if os.Getenv(studioRowsVar) != "1" {
		t.Fatalf("a stand-in app is the Mac Studio's only (%s=1)", studioRowsVar)
	}
	app := d.appAt("launch")
	mustWrite(t, filepath.Join(app, "Contents", "Info.plist"), plistOf(`<key>CFBundleExecutable</key><string>StandIn</string>`+
		`<key>CFBundleIdentifier</key><string>`+d.label+`</string><key>CFBundlePackageType</key><string>APPL</string>`+
		`<key>LSBackgroundOnly</key><true/><key>LSUIElement</key><true/>`))
	program, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(app, "Contents", "MacOS", "StandIn"), string(program))
	if err := os.Chmod(filepath.Join(app, "Contents", "MacOS", "StandIn"), 0o755); err != nil {
		t.Fatal(err)
	}
	if r := runOutside(t, "/usr/bin/codesign", "--force", "--sign", "-", app); r.status != 0 {
		t.Fatalf("signing the stand-in app ad hoc: %+v", r)
	}
	return app
}

// appleEvent runs osascript with script from inside the desk's sandbox, for the Studio's rows only, and gives what
// the sandbox logged it refused osascript meanwhile.
func appleEvent(t *testing.T, d *launchDesk, script string) (ran, string) {
	t.Helper()
	if os.Getenv(studioRowsVar) != "1" {
		t.Fatalf("osascript is the Mac Studio's only (%s=1)", studioRowsVar)
	}
	since := time.Now().Add(-time.Second)
	r := d.inside(t, nil, "/usr/bin/osascript", "-e", script)
	return r, sandboxDenials(t, since, "osascript")
}

// appTools are the programs that open an app or drive one, by base name: none may run here but as openApp runs open.
var appTools = []string{"open", "osascript", "lsappinfo"}

// appSpelling is what text names, in any of its words: an app tool by its base name ("names open"), or an app bundle
// (a path part ending in .app, or a bundle's MacOS folder); "" for neither. Words are split at spaces and at a
// shell's or a list's punctuation, so `sh -c "x; open -a …"` and a JSON argument are read word by word; a subshell's
// opening parenthesis is dropped (`$(open …)`), but a call's isn't, so Python's open(path) isn't the tool.
func appSpelling(text string) string {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("\"'`;&|,=:<>[]{}$", r)
	})
	for _, word := range words {
		if base := path.Base(strings.TrimLeft(word, "(")); slices.Contains(appTools, strings.TrimRight(base, ")")) {
			return "names " + strings.TrimRight(base, ")")
		}
		for _, part := range strings.Split(word, "/") {
			if strings.HasSuffix(strings.ToLower(part), ".app") || part == "MacOS" {
				return "names an app bundle (" + part + ")"
			}
		}
	}
	return ""
}

// startRefusal is why a row here may not start program with args: the guard's run-time half, first in every start
// helper (startFunnels), which every start of these rows with a program not its own goes through. inDesk is whether
// the start is in a desk's sandbox (work, inside). /usr/bin/open runs only as openApp runs it, on a path openRefusal
// takes, on the Studio as well. In a desk only, two more forms: the rules' probe of open (`open -g -j -n <app> --args
// <word>`), on a path probeRefusal takes, and headless Chrome (headlessChrome) from its bundle, with the words
// chromeRefusal takes; outside a desk they are refused as before (RT126ESCAPE2REFUSAL B1). Anything else that names
// open, osascript or lsappinfo, or an app bundle, in any word of the program or its arguments (launchctl asuser …,
// sh -c …), only on the Mac Studio (SMOLCORP_STUDIO_ROWS=1).
func startRefusal(program string, args []string, inDesk bool) error {
	words := append([]string{program}, args...)
	switch {
	case program == "/usr/bin/open" && len(args) == 4 && slices.Equal(args[:3], []string{"-g", "-j", "-a"}):
		return openRefusal(args[3])
	case inDesk && program == "/usr/bin/open" && len(args) == 6 && slices.Equal(args[:3], []string{"-g", "-j", "-n"}) && args[4] == "--args":
		if err := probeRefusal(args[3]); err != nil {
			return err
		}
		words = args[5:]
	case inDesk && program == headlessChrome() && chromeBundle(program):
		if err := chromeRefusal(args); err != nil {
			if os.Getenv(studioRowsVar) != "1" {
				return err
			}
			break // the Studio's: as any other bundle
		}
		words = args
	}
	for _, word := range words {
		switch why := appSpelling(word); {
		case why == "":
		case why == "names open":
			return fmt.Errorf("%s %q: open runs only as openApp runs it, on this run's stand-in (TestOnAMacNoRowHereOpensAnAppUngated)", program, args)
		case os.Getenv(studioRowsVar) != "1":
			return fmt.Errorf("%s %q %s: only the Mac Studio's rows may (%s=1)", program, args, why, studioRowsVar)
		}
	}
	return nil
}

// probeRefusal is why `open -g -j -n` may not take app: the rules' probe of open takes an absolute, clean path to an
// app bundle in the system's temp folder, where the tests make theirs, that doesn't exist, so LaunchServices and
// Gatekeeper have nothing to reach (TestOnAMacOpenInADeskLaunchesNothing). On the Mac Studio it may exist: the hire's
// copy of the app (TestOnAMacAStandInWithAccessibilityCantUseItInADesk).
func probeRefusal(app string) error {
	switch {
	case !filepath.IsAbs(app) || filepath.Clean(app) != app || filepath.Ext(app) != ".app":
		return fmt.Errorf("%q isn't an absolute, clean path to an app bundle: open would look it up through LaunchServices", app)
	case !inTempFolder(app):
		return fmt.Errorf("%s isn't in the temp folder, where a test makes its own: only a row's probe path may be opened", app)
	case os.Getenv(studioRowsVar) == "1":
		return nil
	}
	if _, err := os.Lstat(app); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s exists (%v): opening it goes through LaunchServices and Gatekeeper, which only the Mac Studio's rows may (%s=1)", app, err, studioRowsVar)
	}
	return nil
}

// inTempFolder is whether path's folder is in the system's temp folder, links followed on both sides.
func inTempFolder(path string) bool {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	dir, err2 := filepath.EvalSymlinks(filepath.Dir(path))
	return err == nil && err2 == nil && strings.HasPrefix(dir, tmp+string(filepath.Separator))
}

// headlessFlags are what headless Chrome starts with, or startRefusal refuses it (_rules.md: never a window, never the
// Keychain, no dialog), with a --user-data-dir of its own (chromeRefusal).
var headlessFlags = []string{"--headless", "--use-mock-keychain", "--password-store=basic",
	"--disable-features=DialMediaRouteProvider", "--no-first-run", "--no-default-browser-check"}

// chromeBundle is whether program is the program of a Chrome's or a Chromium's own bundle, and of no other:
// <folder>/<…Chrome….app or …Chromium….app>/Contents/MacOS/<name>, absolute and clean. Whatever SMOLCORP_TEST_CHROME
// names gets the Chrome form only if it is one (RT126ESCAPE2REFUSAL N3); a program in no bundle needs no form.
func chromeBundle(program string) bool {
	macOS := filepath.Dir(program)
	contents := filepath.Dir(macOS)
	app := filepath.Base(filepath.Dir(contents))
	bundles := 0
	for _, part := range strings.Split(program, "/") {
		if strings.HasSuffix(strings.ToLower(part), ".app") {
			bundles++
		}
	}
	return filepath.IsAbs(program) && filepath.Clean(program) == program && filepath.Base(macOS) == "MacOS" &&
		filepath.Base(contents) == "Contents" && strings.HasSuffix(app, ".app") && bundles == 1 &&
		(strings.Contains(app, "Chrome") || strings.Contains(app, "Chromium"))
}

// chromeRefusal is why headless Chrome may not start with args in a desk: every one of headlessFlags, and exactly one
// profile folder, --user-data-dir=<folder>, absolute, clean, in the system's temp folder (inTempFolder) and no link,
// so never the owner's own profile (RT126ESCAPE2REFUSAL B1). Chrome reads a switch with one dash or two, so any word
// that names user-data-dir counts.
func chromeRefusal(args []string) error {
	if missing := slices.DeleteFunc(slices.Clone(headlessFlags), func(f string) bool { return slices.Contains(args, f) }); len(missing) > 0 {
		return fmt.Errorf("headless Chrome %q without %q: the rules' flags are its only form", args, missing)
	}
	var profiles []string
	for _, a := range args {
		if strings.HasPrefix(strings.TrimLeft(a, "-"), "user-data-dir") {
			profiles = append(profiles, a)
		}
	}
	if len(profiles) != 1 || !strings.HasPrefix(profiles[0], "--user-data-dir=") {
		return fmt.Errorf("headless Chrome %q: its form takes exactly one --user-data-dir=<folder>, not %q", args, profiles)
	}
	folder := strings.TrimPrefix(profiles[0], "--user-data-dir=")
	if !filepath.IsAbs(folder) || filepath.Clean(folder) != folder || !inTempFolder(folder) {
		return fmt.Errorf("headless Chrome's profile folder %q isn't an absolute, clean folder in the temp folder: only a row's own may be its profile", folder)
	}
	if info, err := os.Lstat(folder); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("headless Chrome's profile folder %s is a link: Chrome would follow it out of the temp folder", folder)
	}
	return nil
}

// guardAllowed are the declarations of the package's tests that may spell an app tool or an app bundle, each with
// why; startSites are those that may start a program they don't spell out (exec.Command's program not a literal).
// A key is "file declaration", as guardFindings names it. One no longer used fails the guard, so the lists stay true.
var guardAllowed = map[string]string{
	"escape3_darwin_test.go (*launchDesk).openApp": "/usr/bin/open -g -j -a, on a path openRefusal takes only: this run's stand-in, missing on this Mac",
	"escape3_darwin_test.go standInAppAt":          "a stand-in app's path, its label with .app: only standInApp builds one, on the Studio, and openRefusal takes this run's only",
	"escape3_darwin_test.go openRefusal":           "takes this run's stand-in path only, by its .app",
	"escape3_darwin_test.go standInApp":            "the Studio's stand-in app: it fails the test without SMOLCORP_STUDIO_ROWS=1",
	"escape3_darwin_test.go appleEvent":            "osascript, the Studio's: it fails the test without SMOLCORP_STUDIO_ROWS=1",
	"escape3_darwin_test.go var appTools":          "the names the guard looks for",
	"escape3_darwin_test.go appSpelling":           "the bundle parts the guard looks for",
	"escape3_darwin_test.go startRefusal":          "the run-time gate: openApp's own form of a start, and the open probe's, in a desk only",
	"escape3_darwin_test.go probeRefusal":          "the open probe's .app: takes a missing path in the temp folder only, an existing one on the Studio",
	"escape3_darwin_test.go chromeBundle":          "the parts of a Chrome's bundle it takes in a program's path: words only, it starts nothing",
	"escape3_darwin_test.go var deskFormRows":      "startRefusal's rows for its desk forms: given to it only, never run",
	"escape3_darwin_test.go var refusalProbes":     "missing paths in a folder that isn't there, given to the start helpers, which refuse each before any start",
	"escape3_darwin_test.go var guardDodges":       "the guard's self-test: sources it only parses, never compiled or run",
	"escape3_darwin_test.go var refusalRows":       "openRefusal's and startRefusal's rows: given to them only, never run",

	// The base's (RT126MACLAUNCH1, out of scope: VICTOR's call whether to rename them). Paths for the program
	// resolver (programs.go starts nothing) and the Policy's rows; where a file is made, it is a shell-script
	// stand-in in the test's temp folder, which nothing runs or opens.
	"programs_test.go TestAProgramNotFoundIsHeldInEverySearchFolderAndPlace":                    "kitty.app and other.app: places given to the resolver; only the folder apps is made",
	"programs_test.go TestEachKindOfInstallIsDeniedWhole":                                       "install paths in a table for rootOf: nothing on disk",
	"programs_test.go TestSmolcorpsInstallIsDeniedAndItsTmuxTakenFirst":                         "Smolcorp.app/Contents/MacOS/{smolcorp,tmux}: shell-script stand-ins for the resolver, never run",
	"programs_darwin_test.go TestOnAMacAProgramPlantedUnderAnInstallRootIsRefused":              "Smolcorp.app's stand-in script for the resolver, never run; Other.app: an empty folder the sandbox's file probe makes",
	"programs_darwin_test.go TestOnAMacUsrBinsToolsAreDeniedWithTheDeveloperFolderTheyHandOnTo": "Xcode.app's stand-in git script for the resolver, never run; .app/ in a check of developerRoot's answer",
	"gitconfig_test.go TestACommandsWordsAreTheShells":                                          "\"open and 'open: unclosed quotes commandWords must refuse, never run",
	"policy_test.go var rows":                 "/Applications/Smolcorp.app/Contents/MacOS/AgentDeck in the Policy's rows: a path, nothing on disk",
	"escape3_darwin_test.go var guardAllowed": "this list: its reasons name what each declaration may",
	"policy_test.go hirePlaces":               "/Applications/Smolcorp.app as an install root in the Policy's Places: a path, nothing on disk",

	// The stack's (MSYSTACK3): the pieces signed or stacked beside this one. Strings, Studio-gated rows, or the
	// rules' missing-path probe of open; the Linux-only starts are on startSites.
	"bwrap_test.go (fakeFS).names":                                               "fs.PathError's Op \"open\": the fake file system's refusal, a value only; it starts nothing",
	"bwrap_test.go (rootAbove).names":                                            "fs.PathError's Op \"open\": the fake file system's refusal, a value only; it starts nothing",
	"mask_test.go TestASocketTheOwnersAccountCantReachDecidesNothing":            "\"can't open\" in a row's name, words for a person: the row builds a start's words on a fake file system and runs nothing",
	"reportsh_test.go TestTheAppsReportShIsReadOnlyToEveryDesk":                  "Smolcorp.app/Contents/Resources/{bin/smolcorp,report.sh}: shell-script stand-ins in the test's temp folder for the resolver and the Policy's Decide, never run",
	"shims_test.go TestTheAppsShimsAreReadOnlyToEveryDesk":                       "Smolcorp.app/Contents/Resources/{bin/smolcorp,shims}: a shell-script stand-in and a folder in the test's temp folder for the resolver and the Policy's Decide, never run",
	"escape2_darwin_test.go headlessChrome":                                      "the owner's installed Google Chrome (or SMOLCORP_TEST_CHROME), its program run directly in the desk, never through LaunchServices, headless with a mock keychain and basic password store (_rules.md); it skips where there is none. A signed app the rules allow headless (RT126MACLAUNCH1)",
	"escape2_darwin_test.go TestOnAMacOpenInADeskLaunchesNothing":                "open -g -j -n in a desk on Missing.app, checked missing before and after: the rules' probe of open on a missing path only, which leaves LaunchServices and Gatekeeper nothing to reach",
	"escape2_darwin_test.go TestOnAMacAStandInWithAccessibilityCantUseItInADesk": "the hire's copy of the app, its open and its program: all after the row's SMOLCORP_STUDIO_ROWS=1 skip, which studioGated holds",

	// MSYSTACK4's: SA-L6, built before this guard.
	"proxy_test.go TestTheProxyRow": "\"an open way out\" in a row's name, words for a person: its rows run on a fake system (holding), and Words(\"/bin/echo\") is a start's words, parsed and never run",
}

// studioGated are the declarations on guardAllowed whose reason is the Studio's gate: each spelling in them must come
// after an if, at the top of the body, on os.Getenv(studioRowsVar) != "1" that skips or fails the test (studioGate).
var studioGated = map[string]bool{
	"escape3_darwin_test.go standInApp":                                          true,
	"escape3_darwin_test.go appleEvent":                                          true,
	"escape2_darwin_test.go TestOnAMacAStandInWithAccessibilityCantUseItInADesk": true,
}

// startSites: see guardAllowed. Most are sandbox-exec, the test binary, git or a word list of the test's own; the
// guard holds a literal app spelling anywhere in them, wherever it is written.
var startSites = map[string]string{
	"escape3_darwin_test.go runOutside":                                                       "the row's program, after startRefusal",
	"escape_darwin_test.go (*escapeDesk).inside":                                              "sandbox-exec, from sandboxExecOrSkip; the program it runs in the sandbox has passed startRefusal",
	"escape3_darwin_test.go TestOnAMacEachStartHelperRefusesBeforeItStarts":                   "the test binary, run again with one of refusalProbes",
	"escape_darwin_test.go TestOnAMacAHireInALinkedWorktreeCantChangeWhatTheOwnersGitRuns":    "git, from gitOrSkip",
	"enter_unix_test.go startEnter":                                                           "the test binary (os.Executable) as Start's words give it, after the row's wrapper",
	"shims_test.go shimSays":                                                                  "the shim under test (swift, xcodebuild), copied from Resources/shims into a plain folder of the test's own (shimFolder, never inside an app), through disclaim.Cmd with a stand-in PATH, HOME and LANG",
	"gitconfig_test.go (*gitStandin).run":                                                     "the stand-in's git, with its own -c words",
	"homelinks_darwin_test.go TestOnAMacAHomePlaceKeptAsALinkIsHeldWhereItLeads":              "sandbox-exec under the row's profile, with the row's own words",
	"plant_darwin_test.go TestOnAMacAPlantAtTheProjectIsHeldBeforeTheOwnersGitRuns":           "git, and sandbox-exec under the row's profile, with the row's own words",
	"policy_darwin_test.go TestOnAMacAHiresPythonAndWriteToolCantMoveTheBaseBranches":         "sandbox-exec under the row's profile, with the row's own words",
	"policy_darwin_test.go TestOnAMacAMissingFolderAboveADeniedPlaceCantBeBroughtInWhole":     "sandbox-exec under the row's profile, with the row's own words",
	"policy_darwin_test.go TestOnAMacNothingInGitModulesIsWrittenNorBroughtIn":                "sandbox-exec under the row's profile, with the row's own words",
	"policy_darwin_test.go TestOnAMacAHireCantMoveWhereGitFindsTheSharedGit":                  "git (gitArgs), and sandbox-exec under the row's profile, with the row's own words",
	"policy_darwin_test.go TestOnAMacAHireAndTheGMReadTheNightShiftsJobsButCantWriteThem":     "sandbox-exec under the row's profile, with the row's own words",
	"policy_darwin_test.go TestOnAMacAHireCantPlantWhatTheOwnersGitConfigNames":               "sandbox-exec under the row's profile, with the row's own words",
	"programs_darwin_test.go TestOnAMacAProgramPlantedUnderAnInstallRootIsRefused":            "sandbox-exec under the row's profile, with the row's own words",
	"programs_darwin_test.go TestOnAMacUserLookupNeedsItsMachServiceAndTheListIsAProfilesOwn": "sandbox-exec with whoami",
	"programs_darwin_test.go TestOnAMacTheXcrunCacheUsrBinGitReadsIsRefusedToADesk":           "/usr/bin/git --version, alone or under sandbox-exec; sandbox-exec with python3",
	"run_darwin_test.go TestOnAMacTheServerCheckKnowsOurServerFromTheOtherTwo":                "the test binary serving, alone or under sandbox-exec",
	"run_darwin_test.go TestTheServerCheckNeedsTheUserThePidAndTheDenial":                     "the test binary serving",
	"run_darwin_test.go TestOnAMacADeskStartedInItsSandboxIsResponsibleForItself":             "smolcorp-spawn, built from packaging/macos in the test's folder",
	"run_darwin_test.go TestSandboxResponsibilityHolder":                                      "the stand-in built from disclaim's testdata, alone and under sandbox-exec",
	"sbpl_darwin_test.go TestOnAMacTheProfileHoldsForAStandInHire":                            "sandbox-exec under the row's profile, with the row's own words",
	"sbpl_darwin_test.go TestOnAMacTheServicesHoldForAStandInDesk":                            "/usr/bin/env or sandbox-exec, then python3 with the row's probe",
	"linuxcaps_linux_test.go systemRows":                                                      "bwrap --version, Linux only",
	"linuxcaps_linux_test.go runBwrap":                                                        "bwrap, Linux only",
	"linuxcaps_linux_test.go runPlain":                                                        "the test binary, Linux only",
	"winjail_windows_test.go TestAJailRefusesAProgramThatBringsItsOwnTokenParentOrBreakaway":  "the test binary, Windows only",
	"winjail_windows_test.go standIn":                                                         "the test binary, Windows only",
	"winjail_windows_test.go probe":                                                           "the test binary, Windows only",
	"winjail_windows_test.go job":                                                             "the test binary, Windows only",

	// The stack's (MSYSTACK3).
	"bwrap_test.go TestTheGitConfigCopyKeepsTheOwnersIdentityOrRefuses":             "git, from exec.LookPath, on the test's throwaway repo",
	"escape2_darwin_test.go (*escapeDesk).work":                                     "sandbox-exec, from sandboxExecOrSkip; the program it runs in the sandbox is its caller's, after startRefusal",
	"escape2_darwin_test.go outside":                                                "its caller's program, outside any sandbox (git, ps, cc, security, the TCC stand-in), after startRefusal",
	"escape2_darwin_test.go startOutside":                                           "its caller's program, outside any sandbox (node, /bin/sleep), after startRefusal",
	"escape2_darwin_test.go TestOnAMacAHireReadsTheItemsSecurityMadeAsTheOwnerDoes": "/usr/bin/security delete-keychain on the row's own keychain, only where SMOLCORP_TEST_KEYCHAIN=1",
	"bwrap_linux_test.go TestTheLinuxViewStandIn":                                   "inside bwrap, the probe's own words and the paths the view must refuse, Linux only",
	"bwrap_linux_test.go TestSmolcorpsBwrapMakesEveryNamespace":                     "bwrap (BwrapPath), Linux only",
	"bwrap_linux_test.go TestTheLinuxViewStartsOnBwrap":                             "bwrap (BwrapPath) with the test binary, Linux only",
	"bwrap_linux_test.go TestTheViewHidesAKeyReplacedOutside":                       "bwrap (BwrapPath) with the test binary, Linux only",
	"bwrap_linux_test.go TestTheViewHoldsBranchesAndGitConfigReplacedOutside":       "git (exec.LookPath) on the test's repo, and bwrap (BwrapPath) with the test binary, Linux only",
	"mask_linux_test.go TestTheMaskStartsOnBwrap":                                   "bwrap (BwrapPath) with the test binary, Linux only",
	"mask_linux_test.go TestNoSessionBusNorSystemdUserInsideTheView":                "busCommands' words (systemd-run, busctl, gdbus) outside, then bwrap (BwrapPath) with the test binary, Linux only",

	// MSYSTACK4's: SA-L7, built before this guard.
	"forward_linux_test.go TestAnAbstractSocketIsNeverReached": "the test binary (os.Executable) as its abstractprobe, outside and then inside bwrap (BwrapPath) with its own network, Linux only",

	// FXPOLICYFOLD's.
	"policy_fold_darwin_test.go TestOnAMacAFullyFoldedSpellingOfADeniedPlaceIsDeniedToo": "sandbox-exec under the row's profile, with the row's own words",
}

// startFunnels are the package's start helpers on a Mac (startHelpers), each with whether it starts in a desk's
// sandbox: each hands a program of its caller's to a start, and calls startRefusal on it first, saying so. One not on
// the list, one on it that doesn't refuse first, or one whose word to startRefusal isn't its own here fails the guard
// (FXESCAPE2REFUSAL, after MSYSTACK3's Decision 3; the flag after RT126ESCAPE2REFUSAL B1).
var startFunnels = map[string]bool{
	"escape3_darwin_test.go runOutside":          false,
	"escape_darwin_test.go (*escapeDesk).inside": true,
	"escape2_darwin_test.go (*escapeDesk).work":  true,
	"escape2_darwin_test.go outside":             false,
	"escape2_darwin_test.go startOutside":        false,
}

// funnel is what the funnel check finds in a start helper. refused: its body opens, before any start, with
// `if err := startRefusal(program, args, <true or false>); err != nil { …; t.Fatal… }` at the top level, program
// and args being the parameters its starts take (args nil when they take none), neither changed after it. inDesk:
// the flag it gives. sandboxed: every start's program is sandbox-exec, from sandboxExecOrSkip and nothing else.
type funnel struct{ refused, inDesk, sandboxed bool }

// startHelpers reads a test file's syntax (src, or the file when nil) and gives its start helpers, by declaration:
// functions with a start whose program isn't spelled out (as guardFindings finds it) and whose words take one of the
// function's string parameters; each with its funnel.
func startHelpers(fset *token.FileSet, file string, src any) (map[string]funnel, error) {
	f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	out := map[string]funnel{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		params := map[string]string{} // a string parameter's name: its type
		for _, field := range fn.Type.Params.List {
			if typ := types.ExprString(field.Type); typ == "string" || typ == "[]string" || typ == "...string" {
				for _, n := range field.Names {
					params[n.Name] = typ
				}
			}
		}
		// sandbox-exec's names: set from sandboxExecOrSkip, and from nothing else.
		sandboxes, others := map[string]bool{}, map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if set, ok := n.(*ast.AssignStmt); ok {
				for i, lhs := range set.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}
					if call, ok := set.Rhs[min(i, len(set.Rhs)-1)].(*ast.CallExpr); ok && len(set.Lhs) == len(set.Rhs) && types.ExprString(call.Fun) == "sandboxExecOrSkip" {
						sandboxes[id.Name] = true
					} else {
						others[id.Name] = true
					}
				}
			}
			return true
		})
		var starts []token.Pos
		inWords := map[string]bool{} // the parameters a start's words take
		sandboxed := true
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			program := -1
			switch types.ExprString(call.Fun) {
			case "exec.Command", "os.StartProcess", "syscall.Exec", "syscall.ForkExec":
				program = 0
			case "exec.CommandContext":
				program = 1
			}
			if program < 0 {
				return true
			}
			starts = append(starts, call.Pos())
			if program >= len(call.Args) {
				sandboxed = false
				return true
			}
			if id, ok := call.Args[program].(*ast.Ident); !ok || !sandboxes[id.Name] || others[id.Name] {
				sandboxed = false
			}
			if _, spelled := folded(call.Args[program]); spelled {
				return true
			}
			for _, a := range call.Args {
				ast.Inspect(a, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok && params[id.Name] != "" {
						inWords[id.Name] = true
					}
					return true
				})
			}
			return true
		})
		if len(inWords) == 0 {
			continue
		}
		var found funnel
		for _, stmt := range fn.Body.List {
			program, args, inDesk, ok := refusalGate(stmt)
			if !ok {
				continue
			}
			found.inDesk = inDesk
			found.refused = params[program] == "string" && inWords[program] && !changedAfter(fn.Body, stmt.End(), program, args)
			for name := range inWords {
				if typ := params[name]; typ != "string" {
					found.refused = found.refused && name == args
				}
			}
			if args != "nil" {
				found.refused = found.refused && params[args] != "" && params[args] != "string" && inWords[args]
			}
			for _, at := range starts {
				found.refused = found.refused && stmt.Pos() < at
			}
			break
		}
		found.sandboxed = sandboxed && len(starts) > 0
		out[declName(decl)] = found
	}
	return out, nil
}

// refusalGate reads stmt as `if err := startRefusal(program, args, inDesk); err != nil { …; t.Fatal… }`: the names
// given (args may be "nil"), and the flag, true or false as written; ok is false for anything else.
func refusalGate(stmt ast.Stmt) (program, args string, inDesk, ok bool) {
	gate, ok := stmt.(*ast.IfStmt)
	if !ok || gate.Else != nil || len(gate.Body.List) == 0 || types.ExprString(gate.Cond) != "err != nil" {
		return "", "", false, false
	}
	init, ok := gate.Init.(*ast.AssignStmt)
	if !ok || init.Tok != token.DEFINE || len(init.Lhs) != 1 || len(init.Rhs) != 1 || types.ExprString(init.Lhs[0]) != "err" {
		return "", "", false, false
	}
	call, ok := init.Rhs[0].(*ast.CallExpr)
	if !ok || types.ExprString(call.Fun) != "startRefusal" || len(call.Args) != 3 || call.Ellipsis.IsValid() {
		return "", "", false, false
	}
	words := make([]string, 3)
	for i, a := range call.Args {
		id, ok := a.(*ast.Ident)
		if !ok {
			return "", "", false, false
		}
		words[i] = id.Name
	}
	if words[2] != "true" && words[2] != "false" {
		return "", "", false, false
	}
	last, ok := gate.Body.List[len(gate.Body.List)-1].(*ast.ExprStmt)
	if !ok {
		return "", "", false, false
	}
	end, ok := last.X.(*ast.CallExpr)
	if !ok {
		return "", "", false, false
	}
	switch types.ExprString(end.Fun) {
	case "t.Fatal", "t.Fatalf", "t.FailNow":
		return words[0], words[1], words[2] == "true", true
	}
	return "", "", false, false
}

// changedAfter is whether body, after at, sets or declares any of names, in any part (args[0] = …, args = append(…),
// program := … in a block, a range's, &program), so what a start takes may not be what startRefusal was given.
func changedAfter(body *ast.BlockStmt, at token.Pos, names ...string) bool {
	root := func(e ast.Expr) string {
		for {
			switch x := e.(type) {
			case *ast.Ident:
				return x.Name
			case *ast.IndexExpr:
				e = x.X
			case *ast.SliceExpr:
				e = x.X
			case *ast.StarExpr:
				e = x.X
			case *ast.ParenExpr:
				e = x.X
			default:
				return ""
			}
		}
	}
	changed := false
	hit := func(e ast.Expr) {
		if e != nil && slices.Contains(names, root(e)) {
			changed = true
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil || n.Pos() < at {
			return true
		}
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				hit(lhs)
			}
		case *ast.RangeStmt:
			hit(n.Key)
			hit(n.Value)
		case *ast.ValueSpec:
			for _, id := range n.Names {
				hit(id)
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				hit(n.X)
			}
		case *ast.FuncLit:
			for _, field := range n.Type.Params.List {
				for _, id := range field.Names {
					hit(id)
				}
			}
		}
		return true
	})
	return changed
}

// helperSource is a test file of one function, a start helper or not, for the funnel's self-test: parsed, never
// compiled or run.
func helperSource(body string) string {
	return "package sandbox\n\nfunc helper(t *testing.T, dir string, program string, args ...string) {\n" + body + "\n}\n"
}

// refuseFirst and refuseInADesk are a funnel's startRefusal, as the helpers write it, outside a desk and in one.
const (
	refuseFirst   = "\tif err := startRefusal(program, args, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n"
	refuseInADesk = "\tif err := startRefusal(program, args, true); err != nil {\n\t\tt.Fatal(err)\n\t}\n"
	sandboxStart  = "\tsandboxExec := sandboxExecOrSkip(t)\n\texec.CommandContext(context.Background(), sandboxExec, append([]string{\"-f\", dir, program}, args...)...).Run()"
)

// funnelDodges are start helpers the funnel check must see, each with whether it is one and its funnel; the last
// ones are its controls.
var funnelDodges = []struct {
	what, src string
	helper    bool
	want      funnel
}{
	{"a helper with no startRefusal (escape2's three before FXESCAPE2REFUSAL)", helperSource("\texec.Command(program, args...).Run()"), true, funnel{}},
	{"startRefusal after the start", helperSource("\texec.Command(program, args...).Run()\n" + refuseFirst), true, funnel{}},
	{"startRefusal between two starts", helperSource("\texec.Command(program).Run()\n" + refuseFirst + "\texec.Command(program, args...).Run()"), true, funnel{}},
	{"startRefusal on a word of its own", helperSource("\tif err := startRefusal(\"/usr/bin/true\", nil, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\texec.Command(program, args...).Run()"), true, funnel{}},
	{"startRefusal whose refusal goes on", helperSource("\tif err := startRefusal(program, args, false); err != nil {\n\t\tt.Log(err)\n\t}\n\texec.Command(program, args...).Run()"), true, funnel{}},
	{"startRefusal inside a branch", helperSource("\tif dir != \"\" {\n" + refuseFirst + "\t}\n\texec.Command(program, args...).Run()"), true, funnel{}},
	{"the caller's program in sandbox-exec's words", helperSource(sandboxStart), true, funnel{sandboxed: true}},
	{"the caller's program under another name", "package sandbox\n\nfunc helper(t *testing.T, prog string) {\n\tos.StartProcess(prog, nil, nil)\n}\n", true, funnel{}},
	// RT126ESCAPE2REFUSAL N1 (M5 and its kin): startRefusal must take the words the starts take.
	{"startRefusal without the caller's words (the review's M5)", helperSource("\tif err := startRefusal(program, nil, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\texec.Command(program, args...).Run()"), true, funnel{}},
	{"startRefusal on a parameter no start takes", helperSource("\tif err := startRefusal(dir, args, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\texec.Command(program, args...).Run()"), true, funnel{}},
	{"startRefusal with program and args swapped", helperSource("\tif err := startRefusal(args, program, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\texec.Command(program, args...).Run()"), true, funnel{}},
	{"the program set after the gate", helperSource(refuseFirst + "\tprogram = \"/usr/bin/true\"\n\texec.Command(program, args...).Run()"), true, funnel{}},
	{"the words added to after the gate", helperSource(refuseFirst + "\targs = append(args, \"x\")\n\texec.Command(program, args...).Run()"), true, funnel{}},
	{"a word set after the gate", helperSource(refuseFirst + "\targs[0] = \"x\"\n\texec.Command(program, args...).Run()"), true, funnel{}},
	{"the program declared again in a block after the gate", helperSource(refuseFirst + "\t{\n\t\tprogram := \"/usr/bin/true\"\n\t\texec.Command(program, args...).Run()\n\t}"), true, funnel{}},
	{"the program's address given away after the gate", helperSource(refuseFirst + "\tset(&program)\n\texec.Command(program, args...).Run()"), true, funnel{}},
	{"a second []string parameter in the words", "package sandbox\n\nfunc helper(t *testing.T, more []string, program string, args ...string) {\n" + refuseFirst + "\texec.Command(program, append(more, args...)...).Run()\n}\n", true, funnel{}},
	{"startRefusal's old form, without the flag", helperSource("\tif err := startRefusal(program, args); err != nil {\n\t\tt.Fatal(err)\n\t}\n\texec.Command(program, args...).Run()"), true, funnel{}},
	// RT126ESCAPE2REFUSAL B1: the flag is written out, and a desk's start is sandbox-exec's.
	{"the flag a variable", helperSource("\tif err := startRefusal(program, args, dir != \"\"); err != nil {\n\t\tt.Fatal(err)\n\t}\n\texec.Command(program, args...).Run()"), true, funnel{}},
	{"in a desk, said of a start outside one", helperSource(refuseInADesk + "\texec.Command(program, args...).Run()"), true, funnel{refused: true, inDesk: true}},
	{"in a desk, sandbox-exec's name set again", helperSource(refuseInADesk + "\tsandboxExec := sandboxExecOrSkip(t)\n\tsandboxExec = program\n\texec.CommandContext(context.Background(), sandboxExec, append([]string{\"-f\", dir, program}, args...)...).Run()"), true, funnel{refused: true, inDesk: true}},
	// RT126ESCAPE2REFUSAL N2: names taken from the parameters hide a helper; the static half still holds its start (startSites).
	{"the caller's program and words under new names", helperSource("\tp, a := program, args\n\texec.Command(p, a...).Run()"), false, funnel{}},

	{"control: a funnel as the helpers write it", helperSource(refuseFirst + "\texec.Command(program, args...).Run()"), true, funnel{refused: true}},
	{"control: a funnel into sandbox-exec", helperSource(refuseInADesk + sandboxStart), true, funnel{refused: true, inDesk: true, sandboxed: true}},
	{"control: a funnel of a program alone", "package sandbox\n\nfunc helper(t *testing.T, prog string) {\n\tif err := startRefusal(prog, nil, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tos.StartProcess(prog, nil, nil)\n}\n", true, funnel{refused: true}},
	{"control: the profile, a string of the caller's, in sandbox-exec's words", helperSource(refuseInADesk + "\tif dir == \"\" {\n\t\tdir = \"x\"\n\t}\n" + sandboxStart), true, funnel{refused: true, inDesk: true, sandboxed: true}},
	{"control: a program spelled out, with the caller's words", helperSource("\texec.Command(\"/usr/bin/git\", args...).Run()"), false, funnel{}},
	{"control: a program of the row's own", helperSource("\tgit := gitOrSkip(t)\n\texec.Command(git, \"--version\").Run()"), false, funnel{}},
}

// guardFinding is one thing the guard holds against a test file: at a line, in a declaration, what. start is a start
// whose program isn't spelled out (startSites), chrome a use of headlessChrome (chromeCallers), else a spelling
// (guardAllowed). gated is whether it comes after its declaration's Studio gate (studioGate).
type guardFinding struct {
	file, at, decl, what string
	start, chrome, gated bool
}

// list is the name of the guard's list the finding's declaration must be on.
func (g guardFinding) list() string {
	switch {
	case g.start:
		return "startSites"
	case g.chrome:
		return "chromeCallers"
	}
	return "guardAllowed"
}

// allowed is whether the finding's declaration is on its list, and, for a studioGated one, whether the finding comes
// after its gate.
func (g guardFinding) allowed() (key string, ok bool) {
	key = g.file + " " + g.decl
	switch g.list() {
	case "startSites":
		_, ok = startSites[key]
	case "chromeCallers":
		_, ok = chromeCallers[key]
	default:
		_, ok = guardAllowed[key]
		ok = ok && (!studioGated[key] || g.gated)
	}
	return key, ok
}

// chromeCallers are the declarations that may use headlessChrome, each with why. A use of it is Chrome's program in
// its bundle, which the guard holds as it holds a literal path of it (RT126ESCAPE2REFUSAL B1): without this list, any
// row could start the owner's Chrome without spelling it.
var chromeCallers = map[string]string{
	"escape2_darwin_test.go normalWork":                           "its headless Chrome row, through w.work in a desk, with the rules' flags and a profile folder of the row's in the temp folder",
	"escape3_darwin_test.go startRefusal":                         "the run-time gate: the Chrome form, in a desk only",
	"escape3_darwin_test.go TestOnAMacNoRowHereOpensAnAppUngated": "the refusal rows' <chrome>, a stand-in path set in SMOLCORP_TEST_CHROME: words given to startRefusal only, never run",
}

// messageSinks are the calls whose string arguments are words for a person (a test's message, an error's): they
// start nothing, so the guard passes over the literals given to them directly, never over a call among them.
var messageSinks = map[string]bool{"Error": true, "Errorf": true, "Fatal": true, "Fatalf": true, "Log": true,
	"Logf": true, "Skip": true, "Skipf": true, "Run": true, "New": true}

// predicates are the strings functions that only test their words and give a bool (MSYSTACK3: a log line checked
// for "can't open "+folder): nothing given to them can reach a start, so the guard passes over their literals as over
// a message's, never over a call among them.
var predicates = map[string]bool{"Contains": true, "HasPrefix": true, "HasSuffix": true, "EqualFold": true}

// studioGate is where a function's Studio gate ends: its first statement at the top of the body that is
// `if os.Getenv(studioRowsVar) != "1" { … }` (or the variable's value spelled out), whose block ends by skipping or
// failing the test; token.NoPos when it has none. A gate further down, inverted, or one that lets the test go on
// gates nothing.
func studioGate(decl ast.Decl) token.Pos {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok || fn.Body == nil {
		return token.NoPos
	}
	for _, stmt := range fn.Body.List {
		gate, ok := stmt.(*ast.IfStmt)
		if !ok || gate.Init != nil || gate.Else != nil || len(gate.Body.List) == 0 {
			continue
		}
		cond, ok := gate.Cond.(*ast.BinaryExpr)
		if !ok || cond.Op != token.NEQ || types.ExprString(cond.Y) != `"1"` {
			continue
		}
		if v := types.ExprString(cond.X); v != "os.Getenv(studioRowsVar)" && v != `os.Getenv("`+studioRowsVar+`")` {
			continue
		}
		last, ok := gate.Body.List[len(gate.Body.List)-1].(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := last.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		switch types.ExprString(call.Fun) {
		case "t.Skip", "t.Skipf", "t.SkipNow", "t.Fatal", "t.Fatalf", "t.FailNow":
			return gate.End()
		}
	}
	return token.NoPos
}

// usesChrome is the guard's finding for a use of headlessChrome, called or not.
const usesChrome = "uses headlessChrome, the program of Chrome's bundle"

// guardFindings reads a test file's syntax (src, or the file when nil) and gives, declaration by declaration, every
// string literal, in any quoting, and every sum of literals, folded, that names an app tool or an app bundle
// (appSpelling), but a message's; every use of headlessChrome but its own declaration; and every exec.Command,
// exec.CommandContext, os.StartProcess, syscall.Exec or syscall.ForkExec whose program isn't a literal.
func guardFindings(fset *token.FileSet, file string, src any) ([]guardFinding, error) {
	f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var out []guardFinding
	seen := map[string]bool{}
	var gate token.Pos
	add := func(n ast.Node, decl, what string, start bool) {
		g := guardFinding{file: filepath.Base(file), at: fset.Position(n.Pos()).String(), decl: decl, what: what, start: start,
			chrome: what == usesChrome, gated: gate.IsValid() && n.Pos() > gate}
		if k := g.at + g.what; !seen[k] {
			seen[k] = true
			out = append(out, g)
		}
	}
	for _, decl := range f.Decls {
		if g, ok := decl.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			continue
		}
		name := declName(decl)
		gate = studioGate(decl)
		var own *ast.Ident
		if fn, ok := decl.(*ast.FuncDecl); ok {
			own = fn.Name
		}
		message := map[ast.Expr]bool{}
		var markMessage func(ast.Expr)
		markMessage = func(e ast.Expr) {
			switch e := e.(type) {
			case *ast.BasicLit:
				message[e] = true
			case *ast.ParenExpr:
				markMessage(e.X)
			case *ast.BinaryExpr:
				if e.Op == token.ADD {
					message[e] = true
					markMessage(e.X)
					markMessage(e.Y)
				}
			}
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				if n.Name == "headlessChrome" && n != own {
					add(n, name, usesChrome, false)
				}
			case *ast.CallExpr:
				sel, ok := n.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if messageSinks[sel.Sel.Name] {
					args := n.Args
					if sel.Sel.Name == "Run" && len(args) > 1 {
						args = args[:1]
					}
					for _, a := range args {
						markMessage(a)
					}
				}
				x, _ := sel.X.(*ast.Ident)
				if x != nil && x.Name == "strings" && predicates[sel.Sel.Name] {
					for _, a := range n.Args {
						markMessage(a)
					}
				}
				program := -1
				switch {
				case x == nil:
				case x.Name == "exec" && sel.Sel.Name == "Command", x.Name == "os" && sel.Sel.Name == "StartProcess",
					x.Name == "syscall" && (sel.Sel.Name == "Exec" || sel.Sel.Name == "ForkExec"):
					program = 0
				case x.Name == "exec" && sel.Sel.Name == "CommandContext":
					program = 1
				}
				if program >= 0 && program < len(n.Args) {
					if _, ok := folded(n.Args[program]); !ok {
						add(n, name, "starts "+types.ExprString(n.Args[program])+", a program it doesn't spell out", true)
					}
				}
			case *ast.BasicLit, *ast.BinaryExpr:
				e := n.(ast.Expr)
				if message[e] {
					return true
				}
				if text, ok := folded(e); ok {
					if why := appSpelling(text); why != "" {
						add(n, name, fmt.Sprintf("%q %s", text, why), false)
					}
				}
			}
			return true
		})
	}
	return out, nil
}

// folded is a string literal's text, or a sum of them, folded; ok is false for anything else.
func folded(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(e.Value)
		return text, err == nil
	case *ast.ParenExpr:
		return folded(e.X)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		x, ok := folded(e.X)
		y, ok2 := folded(e.Y)
		return x + y, ok && ok2
	}
	return "", false
}

// declName is a top-level declaration's name: a function's ("standInApp"), a method's ("(*launchDesk).openApp"), or
// a var's, const's or type's with its keyword ("var appTools").
func declName(decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv != nil && len(d.Recv.List) == 1 {
			return "(" + types.ExprString(d.Recv.List[0].Type) + ")." + d.Name.Name
		}
		return d.Name.Name
	case *ast.GenDecl:
		var names []string
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.ValueSpec:
				for _, n := range s.Names {
					names = append(names, n.Name)
				}
			case *ast.TypeSpec:
				names = append(names, s.Name.Name)
			}
		}
		return d.Tok.String() + " " + strings.Join(names, ", ")
	}
	return "?"
}

// guardSource is a test file of one function with body, for the guard's self-test: parsed, never compiled or run.
func guardSource(body string) string {
	return "package sandbox\n\nfunc dodge(t *testing.T, d *launchDesk, app, base, tool string) {\n" + body + "\n}\n"
}

// gatedSource is a test file of standInApp alone, which studioGated holds, with body: parsed, never compiled or run.
func gatedSource(body string) string {
	return "package sandbox\n\nfunc standInApp(t *testing.T, d *launchDesk) string {\n" + body + "\n\treturn \"\"\n}\n"
}

// studioSkip is a Studio gate, as the rows write it.
const studioSkip = "\tif os.Getenv(studioRowsVar) != \"1\" {\n\t\tt.Fatalf(\"the Mac Studio's only\")\n\t}\n"

// guardDodges are RT126MACLAUNCH1 B1's spellings the old line-by-line guard missed, and more, each parsed as if it
// were in this file (so guardAllowed applies), with whether the guard must hold it; the last ones are its controls.
var guardDodges = []struct {
	what, src string
	caught    bool
}{
	{"/usr/bin/open in a plain function (the old guard's control)", guardSource(`exec.Command("/usr/bin/open", "-a", app)`), true},
	{"a raw string", guardSource("exec.Command(`/usr/bin/open`, \"-a\", app)"), true},
	{"open by PATH", guardSource(`exec.Command("open", "-a", app)`), true},
	{"filepath.Join of /usr/bin and open", guardSource(`exec.Command(filepath.Join("/usr/bin", "open"), "-a", app)`), true},
	{"Sprintf of /usr/bin/%s and open", guardSource(`exec.Command(fmt.Sprintf("/usr/bin/%s", "open"), "-a", app)`), true},
	{"sh -c with open first", guardSource(`exec.Command("/bin/sh", "-c", "open -g -a "+app)`), true},
	{"sh -c with open after a ;", guardSource(`exec.Command("/bin/sh", "-c", "true;open -a "+app)`), true},
	{"sh -c with open in a $(…)", guardSource(`exec.Command("/bin/sh", "-c", "x=$(open -a "+app+")")`), true},
	{"a bundle built by filepath.Join", guardSource(`mustWrite(t, filepath.Join(base, "StandIn.app", "Contents/MacOS/StandIn"), "")`), true},
	{"a bundle's MacOS folder alone", guardSource(`mustWrite(t, filepath.Join(base, "Contents", "MacOS", "x"), "")`), true},
	{"a bundle by a sum of literals", guardSource(`mustWrite(t, filepath.Join(base, "StandIn.ap"+"p"), "")`), true},
	{"open by a sum of literals", guardSource(`exec.Command("/usr/bin/op"+"en", "-a", app)`), true},
	{".APP in capitals", guardSource(`exec.Command("/usr/bin/true", "/Applications/Calculator.APP")`), true},
	{"allowed := exec.Command(…) anywhere", guardSource(`allowed := exec.Command("/usr/bin/open", "-a", app); _ = allowed`), true},
	{"a var after openApp's body", "package sandbox\n\nfunc (d *launchDesk) openApp(t *testing.T, inside bool, app string) ran {\n" +
		"\treturn d.inside(t, nil, \"/usr/bin/open\", \"-g\", \"-j\", \"-a\", app)\n}\n\nvar openTool = \"/usr/bin/open\"\n", true},
	{"a const", "package sandbox\n\nconst tool = \"open\"\n", true},
	{"an openApp of another type", "package sandbox\n\nfunc (d *escapeDesk) openApp(app string) {\n" +
		"\texec.Command(\"/usr/bin/open\", \"-a\", app).Run()\n}\n", true},
	{"osascript", guardSource(`exec.Command("/usr/bin/osascript", "-e", "return 1")`), true},
	{"lsappinfo", guardSource(`exec.Command("/usr/bin/lsappinfo", "list")`), true},
	{"launchctl asuser, open as its argument", guardSource(`d.inside(t, nil, "/bin/launchctl", "asuser", d.uid, "/usr/bin/open", "-a", app)`), true},
	{"a start inside a message", guardSource(`t.Errorf("%v", exec.Command("/usr/bin/open", "-a", app).Run())`), true},
	{"a program not spelled out", guardSource(`exec.Command(tool, "-a", app)`), true},
	{"a program not spelled out, with a context", guardSource(`exec.CommandContext(context.Background(), tool, "-a", app)`), true},
	{"os.StartProcess", guardSource(`os.StartProcess(tool, []string{tool, app}, nil)`), true},
	{"syscall.Exec", guardSource(`syscall.Exec(tool, []string{tool, app}, nil)`), true},
	// MSYSTACK3's narrowing: strings' predicates pass over their literals, and nothing more.
	{"a start inside strings.Contains", guardSource(`_ = strings.Contains(exec.Command("/usr/bin/open", "-a", app).String(), "x")`), true},
	{"open in sh -c beside a strings.Contains", guardSource(`if strings.Contains(app, "can't open ") { exec.Command("/bin/sh", "-c", "open -a "+app).Run() }`), true},
	{"open in a word given to strings.Contains by name", guardSource(`tool := "open"; _ = strings.Contains(app, tool)`), true},
	{"open given back by strings.ReplaceAll", guardSource(`exec.Command("/bin/sh", "-c", strings.ReplaceAll(app, "x", "open -a"))`), true},
	{"a bundle in bytes.Contains, not strings'", guardSource(`_ = bytes.Contains([]byte(app), []byte("/x/StandIn.app"))`), true},
	// MSYSTACK3's studioGated: a Studio row's spelling holds only after its gate.
	{"a bundle before the Studio gate", gatedSource("\tmustWrite(t, \"/x/StandIn.app/Contents/MacOS/StandIn\", \"\")\n" + studioSkip), true},
	{"open before the Studio gate", gatedSource("\texec.Command(\"/usr/bin/open\", \"-a\", d.label)\n" + studioSkip), true},
	{"a gate inverted", gatedSource("\tif os.Getenv(studioRowsVar) == \"1\" {\n\t\tt.Fatalf(\"x\")\n\t}\n\texec.Command(\"/usr/bin/open\", \"-a\", d.label)"), true},
	{"a gate that lets the test go on", gatedSource("\tif os.Getenv(studioRowsVar) != \"1\" {\n\t\tt.Log(\"x\")\n\t}\n\texec.Command(\"/usr/bin/open\", \"-a\", d.label)"), true},
	{"a gate on another variable", gatedSource("\tif os.Getenv(\"SMOLCORP_TEST_CHROME\") != \"1\" {\n\t\tt.Skip(\"x\")\n\t}\n\texec.Command(\"/usr/bin/open\", \"-a\", d.label)"), true},
	{"a gate inside another block", gatedSource("\tif d != nil {\n" + studioSkip + "\t}\n\texec.Command(\"/usr/bin/open\", \"-a\", d.label)"), true},
	// RT126ESCAPE2REFUSAL B1: headlessChrome is Chrome's program in its bundle, held as a literal path of it is.
	{"runOutside on headlessChrome with an outside profile (the review's M8 stand-in)", guardSource("\trunOutside(t, headlessChrome(), append(slices.Clone(headlessFlags), \"--user-data-dir=/nonexistent-rt126/Library/Chrome\")...)"), true},
	{"headlessChrome taken, not called", guardSource("\tchrome := headlessChrome\n\td.inside(t, nil, chrome())"), true},
	{"headlessChrome in a var", "package sandbox\n\nvar chrome = headlessChrome()\n", true},

	{"control: openApp itself", "package sandbox\n\nfunc (d *launchDesk) openApp(t *testing.T, inside bool, app string) ran {\n" +
		"\treturn d.inside(t, nil, \"/usr/bin/open\", \"-g\", \"-j\", \"-a\", app)\n}\n", false},
	{"control: a message", guardSource(`t.Errorf("open -g -j -a of %s from inside, in its MacOS folder: %v", app, "x.app")`), false},
	{"control: a program spelled out", guardSource(`exec.Command("/bin/launchctl", "print", "gui/501")`), false},
	{"control: Python's open(path)", guardSource(`exec.Command("/usr/bin/python3", "-I", "-c", "import sys; open(sys.argv[1], 'w').write('x')")`), false},
	{"control: words that only look alike", guardSource(`exec.Command("/usr/bin/openssl", "lsd.open", "opened", "macOS", "x.apple", "app")`), false},
	{"control: a log line checked by strings.Contains", guardSource(`if !strings.Contains(app, "can't open "+base+",") { t.Errorf("%s", app) }`), false},
	{"control: a bundle after the Studio gate", gatedSource(studioSkip + "\tmustWrite(t, \"/x/StandIn.app/Contents/MacOS/StandIn\", \"\")"), false},
	{"control: open after a Studio skip, the variable spelled out", gatedSource("\tif os.Getenv(\"SMOLCORP_STUDIO_ROWS\") != \"1\" {\n\t\tt.Skip(\"x\")\n\t}\n\texec.Command(\"/usr/bin/open\", \"-a\", d.label)"), false},
	{"control: headlessChrome's own declaration", "package sandbox\n\nfunc headlessChrome() string { return os.Getenv(\"SMOLCORP_TEST_CHROME\") }\n", false},
	{"control: headlessChrome by name in a message", guardSource("\tt.Log(\"headlessChrome is normalWork's\")"), false},
}

// refusalRows are what openRefusal and startRefusal must say to a start: given to them only, never run. A "<missing>"
// in a path is this run's stand-in label with .app, in a temp folder, and "<existing>" the same path made there.
var refusalRows = []struct {
	program string
	args    []string
	refused bool // on this Mac; the Studio's differ only where studio says
	studio  bool // refused on the Studio
}{
	// B2: open -a takes an app's name, which LaunchServices looks up.
	{"/usr/bin/open", []string{"-g", "-j", "-a", "Calculator"}, true, true},
	{"/usr/bin/open", []string{"-g", "-j", "-a", "TextEdit"}, true, true},
	{"/usr/bin/open", []string{"-g", "-j", "-a", "Calculator.app"}, true, true},
	{"/usr/bin/open", []string{"-g", "-j", "-a", "/nonexistent-rt126/Calculator.app"}, true, true},
	{"/usr/bin/open", []string{"-g", "-j", "-a", "/System/Applications/Calculator.app"}, true, true},
	{"/usr/bin/open", []string{"-g", "-j", "-a", "local.smolcorp-standin.<pid>.x.app"}, true, true},
	{"/usr/bin/open", []string{"-g", "-j", "-a", "/tmp/../<missing>"}, true, true},
	{"/usr/bin/open", []string{"-g", "-j", "-a", "/x/local.smolcorp-standin.1.x.app"}, true, true},
	{"/usr/bin/open", []string{"-g", "-j", "-a", "<existing>"}, true, false},
	{"/usr/bin/open", []string{"-g", "-j", "-a", "<missing>"}, false, false},
	// open in any other form or place, on either Mac.
	{"/usr/bin/open", []string{"-a", "<missing>"}, true, true},
	{"/usr/bin/open", []string{"-g", "-j", "-a", "<missing>", "--args", "x"}, true, true},
	{"open", []string{"-g", "-j", "-a", "<missing>"}, true, true},
	{"/bin/sh", []string{"-c", "open -g -a Calculator"}, true, true},
	{"/bin/sh", []string{"-c", "true; /usr/bin/open -a TextEdit"}, true, true},
	{"/bin/launchctl", []string{"asuser", "501", "/usr/bin/open", "-g", "-j", "-a", "<missing>"}, true, true},
	// The rest: the Studio's only.
	{"/usr/bin/osascript", []string{"-e", "return 1"}, true, false},
	{"/usr/bin/lsappinfo", []string{"list"}, true, false},
	{"/usr/bin/codesign", []string{"--force", "--sign", "-", "<missing>"}, true, false},
	{"/x/StandIn.app/Contents/MacOS/StandIn", nil, true, false},
	{"/stand-in", []string{`[{"Op":"read","At":"/x/StandIn.app/Contents/Info.plist"}]`}, true, false},
	// Controls: what the rows here do start.
	{"/bin/launchctl", []string{"print", "gui/501"}, false, false},
	{"/usr/bin/pgrep", []string{"-fl", "local.smolcorp-standin.1.x"}, false, false},
	{"/stand-in", []string{`[{"Op":"create","At":"/h/Library/LaunchAgents/x.plist"}]`}, false, false},
}

// deskFormRows are what startRefusal must say to its two forms that hold in a desk only, in a desk and outside one
// (RT126ESCAPE2REFUSAL B1): given to it only, never run. chrome is SMOLCORP_TEST_CHROME for the row ("" for the
// installed Google Chrome's path), and "<chrome>" in program what headlessChrome then gives; "<there>" is a temp
// folder of the row's, holding "link", a link out of the temp folder. Each pair is {this Mac, the Studio}: refused?
var deskFormRows = []struct {
	chrome, program string
	args            []string
	inDesk, outside [2]bool
}{
	// The rules' probe of open (FXESCAPE2REFUSAL): -g -j -n on a missing .app in a folder of the temp folder, one word
	// after --args; an existing one on the Studio only (its hire's copy of the app). Outside a desk, never.
	{"", "/usr/bin/open", []string{"-g", "-j", "-n", "<there>/Missing.app", "--args", "<there>/opened.json"}, [2]bool{false, false}, [2]bool{true, true}},
	{"", "/usr/bin/open", []string{"-g", "-j", "-n", "<existing>", "--args", "<there>/opened.json"}, [2]bool{true, false}, [2]bool{true, true}},
	{"", "/usr/bin/open", []string{"-g", "-j", "-n", "<there>/../Missing.app", "--args", "x"}, [2]bool{true, true}, [2]bool{true, true}},
	{"", "/usr/bin/open", []string{"-g", "-j", "-n", "<missing>", "--args", "x"}, [2]bool{true, true}, [2]bool{true, true}},
	{"", "/usr/bin/open", []string{"-g", "-j", "-n", "Missing.app", "--args", "x"}, [2]bool{true, true}, [2]bool{true, true}},
	{"", "/usr/bin/open", []string{"-g", "-j", "-n", "/nonexistent-rt126/Missing.app", "--args", "x"}, [2]bool{true, true}, [2]bool{true, true}},
	{"", "/usr/bin/open", []string{"-g", "-j", "-n", "/System/Applications/Calculator.app", "--args", "x"}, [2]bool{true, true}, [2]bool{true, true}},
	{"", "/usr/bin/open", []string{"-g", "-j", "-n", "<there>/Missing", "--args", "x"}, [2]bool{true, true}, [2]bool{true, true}},
	{"", "/usr/bin/open", []string{"-g", "-j", "-n", "<there>/Missing.app", "--args", "/usr/bin/open"}, [2]bool{true, true}, [2]bool{true, true}},
	{"", "/usr/bin/open", []string{"-g", "-j", "-n", "<there>/Missing.app", "--args"}, [2]bool{true, true}, [2]bool{true, true}},
	{"", "/usr/bin/open", []string{"-n", "<there>/Missing.app", "--args", "x"}, [2]bool{true, true}, [2]bool{true, true}},
	{"", "/usr/bin/open", []string{"-g", "-j", "-n", "<there>/Missing.app", "--args", "x", "y"}, [2]bool{true, true}, [2]bool{true, true}},
	// Headless Chrome from its bundle (normalWork), in a desk, with the rules' flags and one profile folder in the temp
	// folder; outside a desk, a bundle as any other: the Studio's.
	{"", "<chrome>", []string{"--headless", "--no-sandbox", "--use-mock-keychain", "--password-store=basic",
		"--disable-features=DialMediaRouteProvider", "--user-data-dir=<there>/chrome", "--no-first-run",
		"--no-default-browser-check", "--dump-dom", "data:text/html,<p>x</p>"}, [2]bool{false, false}, [2]bool{true, false}},
	{"", "<chrome>", []string{"--no-sandbox", "--use-mock-keychain", "--password-store=basic",
		"--disable-features=DialMediaRouteProvider", "--user-data-dir=<there>/chrome", "--no-first-run", "--no-default-browser-check"}, [2]bool{true, false}, [2]bool{true, false}},
	{"", "<chrome>", []string{"--headless", "--use-mock-keychain", "--password-store=basic",
		"--disable-features=DialMediaRouteProvider", "--no-first-run", "--no-default-browser-check"}, [2]bool{true, false}, [2]bool{true, false}},
	{"", "/Applications/Safari.app/Contents/MacOS/Safari", []string{"--headless", "--use-mock-keychain", "--password-store=basic",
		"--disable-features=DialMediaRouteProvider", "--user-data-dir=<there>/chrome", "--no-first-run", "--no-default-browser-check"}, [2]bool{true, false}, [2]bool{true, false}},
	{"", "<chrome>", []string{"--headless", "--use-mock-keychain", "--password-store=basic", "--disable-features=DialMediaRouteProvider",
		"--user-data-dir=<there>/chrome", "--no-first-run", "--no-default-browser-check", "/usr/bin/open"}, [2]bool{true, true}, [2]bool{true, true}},
	// RT126ESCAPE2REFUSAL B1: the profile folder is exactly one, and the row's own, in the temp folder (the review's
	// M8 probe first: a profile folder outside it).
	{"", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=/nonexistent-rt126/Library/Chrome"), [2]bool{true, false}, [2]bool{true, false}},
	{"", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=chrome"), [2]bool{true, false}, [2]bool{true, false}},
	{"", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=<there>/x/../chrome"), [2]bool{true, false}, [2]bool{true, false}},
	{"", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=<there>/gone/chrome"), [2]bool{true, false}, [2]bool{true, false}},
	{"", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=<there>/link"), [2]bool{true, false}, [2]bool{true, false}},
	{"", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=<there>/chrome", "--user-data-dir=/nonexistent-rt126/Library/Chrome"), [2]bool{true, false}, [2]bool{true, false}},
	{"", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=<there>/chrome", "-user-data-dir=/nonexistent-rt126/Library/Chrome"), [2]bool{true, false}, [2]bool{true, false}},
	{"", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir", "<there>/chrome"), [2]bool{true, false}, [2]bool{true, false}},
	{"", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir="), [2]bool{true, false}, [2]bool{true, false}},
	// RT126ESCAPE2REFUSAL N3: SMOLCORP_TEST_CHROME gets the form only for a Chrome's or a Chromium's own bundle.
	{"/Applications/Chromium.app/Contents/MacOS/Chromium", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=<there>/chrome"), [2]bool{false, false}, [2]bool{true, false}},
	{"/System/Applications/Calculator.app/Contents/MacOS/Calculator", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=<there>/chrome"), [2]bool{true, false}, [2]bool{true, false}},
	{"/Applications/Other.app/Contents/Google Chrome.app/Contents/MacOS/Google Chrome", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=<there>/chrome"), [2]bool{true, false}, [2]bool{true, false}},
	{"/Applications/Google Chrome.app/Contents/MacOS/../MacOS/Google Chrome", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=<there>/chrome"), [2]bool{true, false}, [2]bool{true, false}},
	{"/Applications/Google Chrome.app/Contents/Frameworks/Google Chrome", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=<there>/chrome"), [2]bool{true, false}, [2]bool{true, false}},
	{"<there>/chrome-headless-shell", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=/nonexistent-rt126/Library/Chrome"), [2]bool{false, false}, [2]bool{false, false}},
}

// The guard (VICTOR, 15 Nov; made structural after RT126MACLAUNCH1 B1): no row of the package's tests runs open,
// osascript or lsappinfo, or builds or names an app bundle, but in a declaration on guardAllowed, each Studio-gated or
// a path only, a Studio-gated one's spellings after its gate (studioGated); no start's program goes unspelled but on
// startSites; the guard holds every dodge the review found; and the run-time half, startRefusal, is first in every
// start helper, each on startFunnels.
func TestOnAMacNoRowHereOpensAnAppUngated(t *testing.T) {
	fset := token.NewFileSet()
	for _, c := range guardDodges {
		found, err := guardFindings(fset, "escape3_darwin_test.go", c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		var held []string
		for _, g := range found {
			if _, ok := g.allowed(); !ok {
				held = append(held, g.what)
			}
		}
		if caught := len(held) > 0; caught != c.caught {
			t.Errorf("the guard on %s: held %v, want held %v (%q)", c.what, caught, c.caught, held)
		}
	}

	files, err := filepath.Glob("*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("the package's tests: %v %v", files, err)
	}
	used := map[string]bool{} // by list and key
	for _, file := range files {
		found, err := guardFindings(fset, file, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range found {
			key, ok := g.allowed()
			if ok {
				used[g.list()+" "+key] = true
				continue
			}
			if _, on := guardAllowed[key]; on && g.list() == "guardAllowed" {
				t.Errorf("%s, in %s: %s, before the declaration's %s gate (studioGate): a Studio row's spelling goes after it", g.at, g.decl, g.what, studioRowsVar)
				continue
			}
			t.Errorf("%s, in %s: %s; only a declaration on %s may, with its reason", g.at, g.decl, g.what, g.list())
		}
	}
	for name, list := range map[string]map[string]string{"guardAllowed": guardAllowed, "startSites": startSites, "chromeCallers": chromeCallers} {
		for key, why := range list {
			if !used[name+" "+key] || why == "" {
				t.Errorf("%q (%s) is on %s with nothing of its own to allow, or no reason: take it off", key, why, name)
			}
		}
	}
	for key := range studioGated {
		if _, ok := guardAllowed[key]; !ok {
			t.Errorf("%q is on studioGated but not on guardAllowed", key)
		}
	}

	// startRefusal, before every start, in every start helper here (startFunnels), which the guard finds itself.
	for _, c := range funnelDodges {
		helpers, err := startHelpers(fset, "escape3_darwin_test.go", c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		got, helper := helpers["helper"]
		if helper != c.helper || got != c.want {
			t.Errorf("the funnel check on %s: a helper %v, %+v; want %v, %+v", c.what, helper, got, c.helper, c.want)
		}
	}
	funnels := map[string]bool{}
	for _, file := range files {
		if !strings.HasSuffix(file, "_darwin_test.go") {
			continue
		}
		helpers, err := startHelpers(fset, file, nil)
		if err != nil {
			t.Fatal(err)
		}
		for decl, got := range helpers {
			key := file + " " + decl
			funnels[key] = true
			inDesk, on := startFunnels[key]
			if !on {
				t.Errorf("%s hands a program of its caller's to a start: only a declaration on startFunnels may, with startRefusal first", key)
			}
			if !got.refused {
				t.Errorf("%s: startRefusal, given its caller's program and words and failing the test, isn't the first thing before each of its starts, or they change after it", key)
			}
			if on && got.inDesk != inDesk {
				t.Errorf("%s tells startRefusal it starts in a desk: %v; startFunnels says %v", key, got.inDesk, inDesk)
			}
			if got.inDesk && !got.sandboxed {
				t.Errorf("%s tells startRefusal it starts in a desk, but a start of its isn't sandbox-exec's (sandboxExecOrSkip)", key)
			}
		}
	}
	for key := range startFunnels {
		if !funnels[key] {
			t.Errorf("%q is on startFunnels but hands no program of its caller's to a start: take it off", key)
		}
	}

	// openRefusal's and startRefusal's rows (B2): an app's name is refused before anything runs, on either Mac.
	there := t.TempDir()
	missing := standInAppAt(filepath.Join(there, "missing"), fmt.Sprintf("local.smolcorp-standin.%d.x", os.Getpid()))
	existing := standInAppAt(there, fmt.Sprintf("local.smolcorp-standin.%d.y", os.Getpid()))
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/nonexistent-rt126", filepath.Join(there, "link")); err != nil {
		t.Fatal(err)
	}
	fill := strings.NewReplacer("/tmp/../<missing>", "/tmp/../"+strings.TrimPrefix(missing, "/"), "<missing>", missing,
		"<existing>", existing, "<pid>", strconv.Itoa(os.Getpid()), "<there>", there)
	studio := 0
	if os.Getenv(studioRowsVar) == "1" {
		studio = 1
	}
	for _, row := range refusalRows {
		args := make([]string, len(row.args))
		for i, a := range row.args {
			args[i] = fill.Replace(a)
		}
		want := row.refused
		if studio == 1 {
			want = row.studio
		}
		for _, inDesk := range []bool{true, false} {
			if err := startRefusal(row.program, args, inDesk); (err != nil) != want {
				t.Errorf("startRefusal(%s %q, in a desk %v) = %v, want refused %v", row.program, args, inDesk, err, want)
			}
		}
		if row.program == filepath.Join("/usr/bin", appTools[0]) && len(args) == 4 {
			if err := openRefusal(args[3]); (err != nil) != want {
				t.Errorf("openRefusal(%q) = %v, want refused %v", args[3], err, want)
			}
		}
	}
	for _, row := range deskFormRows {
		t.Setenv("SMOLCORP_TEST_CHROME", fill.Replace(row.chrome))
		program := strings.ReplaceAll(row.program, "<chrome>", headlessChrome())
		args := make([]string, len(row.args))
		for i, a := range row.args {
			args[i] = fill.Replace(a)
		}
		for inDesk, want := range map[bool]bool{true: row.inDesk[studio], false: row.outside[studio]} {
			if err := startRefusal(program, args, inDesk); (err != nil) != want {
				t.Errorf("startRefusal(%s %q, in a desk %v) = %v, want refused %v", program, args, inDesk, err, want)
			}
		}
	}
}

// refusalProbeVar makes the test binary, run again by TestOnAMacEachStartHelperRefusesBeforeItStarts, give one
// probe to one start helper: "<helper>|<probe>|<folder>".
const refusalProbeVar = "SMOLCORP_REFUSAL_PROBE"

// refusalProbes are starts every start helper must refuse before it starts anything (the review's M6 and M8, as
// rows): each program a missing path in <gone>, a folder that isn't there, so were a refusal missing, the start could
// only fail, "no such file". chrome is SMOLCORP_TEST_CHROME for the probe; outsideOnly is a form a desk may start
// (so a desk's helper isn't given it); studio whether the Studio refuses it too; said what the refusal says.
var refusalProbes = []struct {
	chrome, program     string
	args                []string
	outsideOnly, studio bool
	said                string
}{
	{"", "<gone>/open", []string{"-a", "x"}, false, true, "open runs only as openApp runs it"},
	{"", "<gone>/Y.app/Contents/MacOS/Y", nil, false, false, "names an app bundle (Y.app): only the Mac Studio's rows may"},
	{"<gone>/Google Chrome.app/Contents/MacOS/Google Chrome", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=/nonexistent-rt126/Library/Chrome"), false, false, "/nonexistent-rt126/Library/Chrome"},
	{"<gone>/Google Chrome.app/Contents/MacOS/Google Chrome", "<chrome>", append(slices.Clone(headlessFlags), "--user-data-dir=<there>/chrome"), true, false, "names an app bundle (Chrome.app): only the Mac Studio's rows may"},
}

// refusalProbeHelpers are startFunnels' helpers, each given a probe's program and words, and whether it starts in a
// desk. A helper that gives back has started its program.
var refusalProbeHelpers = map[string]struct {
	inDesk bool
	run    func(t *testing.T, folder, program string, args []string)
}{
	"runOutside": {false, func(t *testing.T, folder, program string, args []string) { runOutside(t, program, args...) }},
	"inside": {true, func(t *testing.T, folder, program string, args []string) {
		(&escapeDesk{root: folder}).inside(t, nil, program, args...)
	}},
	"work": {true, func(t *testing.T, folder, program string, args []string) {
		(&escapeDesk{}).work(t, "", folder, 5*time.Second, nil, program, args...)
	}},
	"outside": {false, func(t *testing.T, folder, program string, args []string) { outside(t, folder, nil, program, args...) }},
	"startOutside": {false, func(t *testing.T, folder, program string, args []string) {
		startOutside(t, folder, nil, program, args...)
	}},
}

// Each start helper on startFunnels refuses, at run time, before it starts anything: an app tool, an app bundle,
// headless Chrome with a profile folder outside the temp folder, and, outside a desk, headless Chrome at all
// (RT126ESCAPE2REFUSAL B1, M6 and M8). Each probe runs in the test binary run again, so a helper's t.Fatal is that
// run's alone; its program is missing, so nothing could start were a refusal gone.
func TestOnAMacEachStartHelperRefusesBeforeItStarts(t *testing.T) {
	if probe := os.Getenv(refusalProbeVar); probe != "" {
		words := strings.SplitN(probe, "|", 3)
		if len(words) != 3 {
			t.Fatalf("no such probe: %q", probe)
		}
		helper := refusalProbeHelpers[words[0]]
		n, err := strconv.Atoi(words[1])
		if helper.run == nil || err != nil || n < 0 || n >= len(refusalProbes) {
			t.Fatalf("no such probe: %q", probe)
		}
		folder := words[2]
		p := refusalProbes[n]
		fill := strings.NewReplacer("<gone>", filepath.Join(folder, "gone"), "<there>", folder)
		t.Setenv("SMOLCORP_TEST_CHROME", fill.Replace(p.chrome))
		args := make([]string, len(p.args))
		for i, a := range p.args {
			args[i] = fill.Replace(a)
		}
		helper.run(t, folder, strings.ReplaceAll(fill.Replace(p.program), "<chrome>", os.Getenv("SMOLCORP_TEST_CHROME")), args)
		t.Errorf("REACHED THE START in %s", words[0])
		return
	}
	if len(refusalProbeHelpers) != len(startFunnels) {
		t.Errorf("the probes' helpers (%d) aren't startFunnels' (%d): give each one its probes", len(refusalProbeHelpers), len(startFunnels))
	}
	for key, inDesk := range startFunnels {
		name := key[strings.LastIndex(key, " ")+1:]
		name = name[strings.LastIndex(name, ".")+1:]
		if h, ok := refusalProbeHelpers[name]; !ok || h.inDesk != inDesk {
			t.Errorf("%s (in a desk %v) has no probe helper of its own kind", key, inDesk)
		}
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	studio := os.Getenv(studioRowsVar) == "1"
	probes := 0
	for name, helper := range refusalProbeHelpers {
		for n, p := range refusalProbes {
			if p.outsideOnly && helper.inDesk || studio && !p.studio {
				continue
			}
			probes++
			folder := t.TempDir()
			cmd := disclaim.Cmd(exec.Command(self, "-test.run=^TestOnAMacEachStartHelperRefusesBeforeItStarts$", "-test.count=1", "-test.v"))
			cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s|%d|%s", refusalProbeVar, name, n, folder))
			out, err := cmd.CombinedOutput()
			said := string(out)
			switch {
			case err == nil:
				t.Errorf("%s, probe %d: the run passed, so nothing was refused:\n%s", name, n, said)
			case strings.Contains(said, "REACHED THE START"), strings.Contains(said, "no such file"), strings.Contains(said, "fork/exec"):
				t.Errorf("%s, probe %d: its start was reached:\n%s", name, n, said)
			case !strings.Contains(said, p.said):
				t.Errorf("%s, probe %d: refused, but not by startRefusal (%q):\n%s", name, n, p.said, said)
			}
			if _, err := os.Stat(filepath.Join(folder, "gone")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("%s, probe %d: the missing folder is there (%v)", name, n, err)
			}
		}
	}
	t.Logf("%d probes run, each in its own run of the test binary", probes)
}

// sandboxDenials is what the sandbox logged it refused process (by name) since since: its "deny(1) …" lines.
func sandboxDenials(t *testing.T, since time.Time, process string) string {
	t.Helper()
	time.Sleep(1500 * time.Millisecond) // the log is written a little after
	r := runOutside(t, "/usr/bin/log", "show", "--style", "compact", "--start", since.Format("2006-01-02 15:04:05"),
		"--predicate", fmt.Sprintf(`sender == "Sandbox" AND eventMessage CONTAINS "%s("`, process))
	var out []string
	for _, line := range strings.Split(r.stdout, "\n") {
		if i := strings.Index(line, "deny("); i >= 0 {
			out = append(out, line[i:])
		}
	}
	return strings.Join(out, "\n")
}

// appears is whether at exists within limit.
func appears(at string, limit time.Duration) bool {
	for end := time.Now().Add(limit); ; time.Sleep(100 * time.Millisecond) {
		if _, err := os.Lstat(at); err == nil {
			return true
		}
		if time.Now().After(end) {
			return false
		}
	}
}

// The Studio's launchd rows. The controls: from outside, the calls the rows above make from inside load the stand-in
// job (bootstrap, submit, load), start a loaded one (kickstart) and set or unset a variable in the session, so the
// refusals inside are the sandbox's. Then, against a loaded stand-in job: from inside, kickstart (with and without -k)
// starts nothing; bootout (gui and user), remove, stop and kill SIGCONT leave it loaded with the same pid, an outside
// kill SIGCONT and bootout being the controls (RT126MACLAUNCH1 B3). One that goes through is a STOP for SIERRA, not a
// list change. Each load is undone at once; the end checks that nothing is left.
func TestOnAMacTheLaunchdRefusalsAreTheSandboxs(t *testing.T) {
	sandboxExecOrSkip(t)
	if os.Getenv(studioRowsVar) != "1" {
		t.Skipf("SKIPPED here: %s=1 on the Mac Studio runner loads a stand-in job into its session for a moment", studioRowsVar)
	}
	d := newLaunchDesk(t)
	gui := "gui/" + d.uid
	job := gui + "/" + d.label
	unload := func(row string) {
		t.Helper()
		for _, domain := range d.loadedIn(t) {
			if r := runOutside(t, "/bin/launchctl", "bootout", domain+"/"+d.label); r.status != 0 {
				t.Fatalf("%s: booting out the stand-in: %+v", row, r)
			}
		}
		os.Remove(d.ran)
	}
	if r := runOutside(t, "/bin/launchctl", "bootstrap", gui, d.plist); r.status != 0 || len(d.loadedIn(t)) == 0 {
		t.Errorf("control: bootstrap from outside: %+v", r)
	}
	unload("bootstrap")
	if r := runOutside(t, "/bin/launchctl", "submit", "-l", d.label, "--", "/usr/bin/touch", d.ran); r.status != 0 || len(d.loadedIn(t)) == 0 {
		t.Errorf("control: submit from outside: %+v", r)
	}
	unload("submit")
	if r := runOutside(t, "/bin/launchctl", "load", d.plist); refused(r) || len(d.loadedIn(t)) == 0 {
		t.Errorf("control: load from outside: %+v", r)
	}
	unload("load")

	// A loaded stand-in job that doesn't run at load: from inside, kickstart can't start it; from outside it does.
	quiet := filepath.Join(d.base, "launch", "quiet.plist")
	mustWrite(t, quiet, plistOf(`<key>Label</key><string>`+d.label+`</string>`+
		`<key>ProgramArguments</key><array><string>/usr/bin/touch</string><string>`+d.ran+`</string></array>`))
	if r := runOutside(t, "/bin/launchctl", "bootstrap", gui, quiet); r.status != 0 {
		t.Fatalf("control: bootstrap of the quiet stand-in from outside: %+v", r)
	}
	for _, words := range [][]string{{"kickstart"}, {"kickstart", "-k"}} {
		r := d.inside(t, nil, "/bin/launchctl", append(words, job)...)
		if appears(d.ran, time.Second) || !refused(r) {
			t.Errorf("launchctl %s of a loaded job from inside: %+v, it ran: %v", strings.Join(words, " "), r, appears(d.ran, 0))
		}
	}
	if r := runOutside(t, "/bin/launchctl", "kickstart", job); r.status != 0 || !appears(d.ran, 5*time.Second) {
		t.Errorf("control: kickstart of the loaded job from outside: %+v, it ran: %v", r, appears(d.ran, 0))
	}
	unload("kickstart")

	// A running stand-in job (a sleep, no window): the writes that end or signal a job, from inside.
	sleeper := filepath.Join(d.base, "launch", "sleeper.plist")
	mustWrite(t, sleeper, plistOf(`<key>Label</key><string>`+d.label+`</string>`+
		`<key>ProgramArguments</key><array><string>/bin/sleep</string><string>600</string></array><key>RunAtLoad</key><true/>`))
	if r := runOutside(t, "/bin/launchctl", "bootstrap", gui, sleeper); r.status != 0 {
		t.Fatalf("control: bootstrap of the running stand-in from outside: %+v", r)
	}
	pidNow := func() string { t.Helper(); return pidOf(runOutside(t, "/bin/launchctl", "print", job).stdout) }
	pid := ""
	for end := time.Now().Add(5 * time.Second); pid == "" && time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
		pid = pidNow()
	}
	if pid == "" {
		t.Fatalf("control: the running stand-in has no pid")
	}
	for _, words := range [][]string{{"bootout", job}, {"bootout", "user/" + d.uid + "/" + d.label}, {"remove", d.label},
		{"stop", d.label}, {"kill", "SIGCONT", job}} {
		r := d.inside(t, nil, "/bin/launchctl", words...)
		time.Sleep(time.Second)
		if now := pidNow(); !refused(r) || now != pid {
			t.Errorf("launchctl %s of a loaded job from inside: %+v; its pid %q, was %q: a STOP for SIERRA, not a list change",
				strings.Join(words, " "), r, now, pid)
		}
	}
	if r := runOutside(t, "/bin/launchctl", "kill", "SIGCONT", job); r.status != 0 || pidNow() != pid {
		t.Errorf("control: kill SIGCONT of the loaded job from outside: %+v", r)
	}
	if r := runOutside(t, "/bin/launchctl", "bootout", job); r.status != 0 || len(d.loadedIn(t)) != 0 {
		t.Errorf("control: bootout of the loaded job from outside: %+v", r)
	}
	if n, err := strconv.Atoi(pid); err == nil {
		for end := time.Now().Add(5 * time.Second); syscall.Kill(n, 0) == nil; time.Sleep(100 * time.Millisecond) {
			if time.Now().After(end) {
				t.Errorf("the running stand-in's pid %d is still there after its bootout", n)
				break
			}
		}
	}
	unload("bootout")

	name := "SMOLCORP_STANDIN_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	if r := runOutside(t, "/bin/launchctl", "setenv", name, "1"); r.status != 0 {
		t.Errorf("control: setenv from outside: %+v", r)
	}
	if r := runOutside(t, "/bin/launchctl", "unsetenv", name); r.status != 0 {
		t.Errorf("control: unsetenv from outside: %+v", r)
	}
	if r := runOutside(t, "/bin/launchctl", "getenv", name); r.stdout != "" {
		t.Errorf("%s is still set: %q", name, r.stdout)
	}
}

// launchStarters are the Mach services that start a process for whoever asks, or put a dialog in front of the owner
// on its behalf: none may be on a desk's list but machGaps'. A name ending in * is every name it starts. Each says
// which program serves it on macOS 26 and why (MACLAUNCH1's report lists the sources).
var launchStarters = []MachService{
	{"com.apple.xpc.launchd.*", "launchd's own endpoints: jobs, domains"},
	{"com.apple.xpc.smd", "smd: SMJobBless installs and starts a privileged helper"},
	{"com.apple.xpc.loginitemregisterd", "loginitemregisterd: SMLoginItemSetEnabled starts a login item"},
	{"com.apple.backgroundtaskmanagement*", "backgroundtaskmanagementd and its agent: SMAppService registers agents, daemons and login items"},
	{"com.apple.coreservices.launchservicesd", "LaunchServices: opens apps and documents (lsopen)"},
	{"com.apple.lsd.*", "lsd: LaunchServices' database and its launch requests (lsd.open, lsd.mapdb, lsd.modifydb)"},
	{"com.apple.CoreServices.coreservicesd", "coreservicesd: LaunchServices' launch requests"},
	{"com.apple.coreservices.quarantine-resolver", "LaunchServices' quarantine, for opening"},
	{"com.apple.coreservices.uiagent", "CoreServicesUIAgent: LaunchServices' dialogs"},
	{"com.apple.coreservices.appleevents", "appleeventsd: Apple Events, which launch and drive apps"},
	{"com.apple.coreservices.sharedfilelistd", "sharedfilelistd: the login items list"},
	{"com.apple.runningboard*", "runningboardd: launches and keeps apps and XPC services"},
	{"com.apple.dock.*", "the Dock: opens apps"},
	{"com.apple.loginwindow*", "loginwindow: relaunches apps at login, logs out"},
	{"com.apple.ScreenTime*", "Screen Time's agents: the owner's session"},
	{"com.apple.sysextd", "sysextd: activates system extensions (their daemons)"},
	{"com.apple.authd", "authd: AuthorizationExecuteWithPrivileges runs a tool as root, after its dialog"},
	{"com.apple.security.agent*", "SecurityAgent: securityd's and authd's password dialogs"},
	{"com.apple.security.authhost*", "authhost: authd's dialogs and plugins"},
	{"com.apple.UNCUserNotification", "UserNotificationCenter: a dialog in front of the owner"},
	{"com.apple.windowserver*", "the WindowServer: the owner's session"},
	{"com.apple.SecurityServer", "securityd: a Keychain dialog in front of the owner, put up by SecurityAgent, which launchd starts on demand"},
	{"com.apple.system.notification_center", "notifyd: a post wakes a job already installed with a notifyd LaunchEvent"},
}

// machGaps are the Mach services on the desk's list that meet launchStarters' criterion and stay on it for now, each
// with why (RT126MACLAUNCH1 B4). They aren't waived: each is a STOP item for SIERRA (VICTOR asked, 15 Nov), and the
// audit fails if one is dropped from here while it's on the list, if a new one appears, or if one no longer matches.
var machGaps = map[string]string{
	"com.apple.SecurityServer": "securityd asks SecurityAgent, which launchd starts on demand, to put a Keychain dialog " +
		"in front of the owner for the caller (com.apple.security.agent* itself is denied, but it's securityd that asks). " +
		"On the list for Claude Code's sign-in, kept in the Keychain. STOP for SIERRA: to rule",
	"com.apple.system.notification_center": "a notifyd post wakes a job already installed with a notifyd LaunchEvent " +
		"(com.apple.notifyd.matching), which may start one of the owner's installed jobs that isn't running. A desk can't " +
		"install one (job-creation and ~/Library/LaunchAgents are denied), and the programs are fixed. On the list as " +
		"everyday plumbing between processes. STOP for SIERRA: to rule",
}

// machGapsRuled is machGaps' names as FXMACLAUNCH1R1 pinned them: a change to either is SIERRA's to rule, not a test
// edit.
var machGapsRuled = []string{"com.apple.SecurityServer", "com.apple.system.notification_center"}

// startsAProcess is the entry of launchStarters that name matches, if any.
func startsAProcess(name string) (MachService, bool) {
	for _, s := range launchStarters {
		if prefix, ok := strings.CutSuffix(s.Name, "*"); ok && strings.HasPrefix(name, prefix) || s.Name == name {
			return s, true
		}
	}
	return MachService{}, false
}

// The audit: no Mach service a desk may look up starts a process or puts a dialog up for it, except machGaps', and
// the desk's real profile allows each by its exact name, nothing by a prefix or a pattern. A list entry this fails on
// is a STOP-and-report, not a quiet change of either list.
func TestOnAMacNoMachServiceOnTheListStartsAProcessForADesk(t *testing.T) {
	var gaps []string
	for _, m := range MachServices() {
		s, ok := startsAProcess(m.Name)
		switch why := machGaps[m.Name]; {
		case ok && why == "":
			t.Errorf("%s is on the Mach list (%s), but %s: stop and report", m.Name, m.Why, s.Why)
		case ok:
			gaps = append(gaps, m.Name)
			t.Logf("KNOWN EXCEPTION, STOP for SIERRA: %s (%s): %s", m.Name, s.Why, why)
		case why != "":
			t.Errorf("%s is on machGaps, but not on the Mach list or not a launch starter any more: take it off", m.Name)
		}
	}
	sort.Strings(gaps)
	var pinned []string
	for name := range machGaps {
		pinned = append(pinned, name)
	}
	sort.Strings(pinned)
	if !slices.Equal(gaps, machGapsRuled) || !slices.Equal(pinned, machGapsRuled) {
		t.Errorf("the Mach list's exceptions are %q, machGaps %q; pinned %q: dropping or adding one is SIERRA's ruling", gaps, pinned, machGapsRuled)
	}
	// The checker's own controls: it catches what each entry names, and every entry is well formed.
	for _, name := range []string{"com.apple.xpc.launchd.domain.system", "com.apple.coreservices.launchservicesd",
		"com.apple.lsd.open", "com.apple.backgroundtaskmanagement.agent", "com.apple.security.agent.login",
		"com.apple.SecurityServer", "com.apple.system.notification_center"} {
		if _, ok := startsAProcess(name); !ok {
			t.Errorf("the checker misses %s", name)
		}
	}
	for _, s := range launchStarters {
		if strings.Contains(strings.TrimSuffix(s.Name, "*"), "*") || s.Why == "" {
			t.Errorf("launchStarters' entry %q: a * only at the end, and a reason", s.Name)
		}
	}

	d := newEscapeDesk(t)
	data, err := os.ReadFile(d.profile)
	if err != nil {
		t.Fatal(err)
	}
	var list, allowed []string
	for _, m := range MachServices() {
		list = append(list, m.Name)
	}
	for _, line := range strings.Split(string(data), "\n") {
		rule, _, _ := strings.Cut(line, " ;")
		if !strings.Contains(rule, "mach-lookup") {
			continue
		}
		name, ok := strings.CutPrefix(rule, `(allow mach-lookup (global-name "`)
		switch {
		case rule == "(deny mach-lookup)":
		case !ok || !strings.HasSuffix(name, `"))`):
			t.Errorf("a mach-lookup rule that isn't a deny of all or an exact name: %s", line)
		default:
			allowed = append(allowed, strings.TrimSuffix(name, `"))`))
		}
	}
	if fmt.Sprint(allowed) != fmt.Sprint(list) {
		t.Errorf("the profile allows %q, the list is %q", allowed, list)
	}
	for _, never := range []string{"(deny job-creation)", "(deny lsopen)", "(deny appleevent-send)"} {
		if !strings.Contains(string(data), never+" ;") || strings.Index(string(data), never) < strings.LastIndex(string(data), "(allow ") {
			t.Errorf("%s isn't in the profile after every allow", never)
		}
	}
}
