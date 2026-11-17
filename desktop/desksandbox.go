//go:build !windows

package desktop

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// A desk's sandbox at its start (SA-M6, SANDBOX-PLAN §A3.1 and §A5.2): what the core knows of where the desk works,
// as the sandbox's Places; its Policy, made from them; X12's start-time checks on that Policy; and the checks its
// `smolcorp sandbox enter` runs. The start itself is each system's start-only runner (desksandbox_darwin.go on a
// Mac), which the desk's hireTmux runs for its start call alone (hiretmux.go).
//
// On a Mac every desk the engine starts is started in its sandbox: hires, the GM, and a desk Hand back resumes. One
// that can't be is refused, with §A5.2's words; none falls back to a start outside it (AQ1). Linux's start is SA-L2
// and SA-L4's: until they land, a desk there starts as before.

// deskSandboxed is whether the engine starts its desks in their sandbox on this system.
const deskSandboxed = runtime.GOOS == "darwin"

// deskAt is what a desk's start knows of it, for its Places.
type deskAt struct {
	ID string
	GM bool
	// Root is where it works: its worktree, or the folder it was hired into. Checkout is the owner's checkout Root is
	// a worktree of, "" for none; Hires where the hires' worktrees go, "" for none.
	Root, Checkout, Hires string
	// Socket is its own tmux socket (deskSocketPath).
	Socket string
	// Accounts are the Claude account folders its fence denies besides ~/.claude.
	Accounts []claudeProfile
	// Brief is its brief, "" for none: its check reads it.
	Brief string
	Git   gitRunner
	// Programs are the programs its start runs besides those the core finds (deskFinding): a plain hire's agent, as
	// its launcher resolved it (SA-M10), so its install root is the desk's to read and never to write.
	Programs []sandbox.Program
	// DatabaseOptIn is AQ21's opt-in as the owner set it (sandboxchecks.go): its start checks the databases only then.
	DatabaseOptIn bool
}

// startedInSandbox is whether tmux's start was a desk's in its sandbox, with the server the core checked at it
// (§A5.2 step 9): what a desk's row tags it with (Hire.Sandboxed, SA-X13).
func startedInSandbox(tmux *hireTmux) bool { return tmux.sandboxed && tmux.server > 1 }

// deskSandbox is one desk's sandbox, made before its start: its Places and its Policy, and what its start checks.
type deskSandbox struct {
	at     deskAt
	places sandbox.Places
	policy *sandbox.Policy
}

// errSandboxNotMade is a start refused before anything of it ran: its Places, its Policy or X12's checks.
type errSandboxNotMade struct{ words string }

func (e *errSandboxNotMade) Error() string { return e.words }

// makeDeskSandbox makes desk d's Places and Policy, and runs X12's start-time checks on them: the link check, whose
// words refuse the start (checkLinks), and the old git plants, kept for Setup (findOldGitPlants). Nothing of the desk
// runs yet. The sandbox is kept for the desk's stop (deskSandboxOf).
func makeDeskSandbox(d deskAt) (*deskSandbox, error) {
	pl, err := deskPlaces(d)
	if err != nil {
		log.Printf("sandbox: desk %s's places: %v", d.ID, err)
		return nil, &errSandboxNotMade{sandboxNotMade}
	}
	// SA-M12's checks, at each start: whether Remote Login answers (its SSH keys denied then), and, with AQ21's
	// opt-in on, whether every local database refuses a password-less login (deskServices reads it). Setup's Sandbox
	// row shows both (SA-X14).
	pl.RemoteLogin = deskStartChecks(d.ID, d.DatabaseOptIn).RemoteLogin
	pol, err := sandbox.New(pl)
	if err != nil {
		log.Printf("sandbox: desk %s's policy: %v", d.ID, err)
		return nil, &errSandboxNotMade{sandboxNotMade}
	}
	if _, err := checkLinks(pol, pl); err != nil {
		var words *linkCheckError
		if errors.As(err, &words) {
			return nil, &errSandboxNotMade{words.words}
		}
		log.Printf("sandbox: desk %s's link check: %v", d.ID, err)
		return nil, &errSandboxNotMade{sandboxNotMade}
	}
	if pl.GitDir != "" {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		keepOldGitPlants(d.ID, pl.GitDir, findOldGitPlants(ctx, pl.GitDir))
		cancel()
	}
	s := &deskSandbox{at: d, places: pl, policy: pol}
	deskStartListing(s)
	keptSandboxes.Lock()
	keptSandboxes.of[d.ID] = s
	keptSandboxes.Unlock()
	return s, nil
}

// keptSandboxes are the sandboxes of the desks this core started, by id, for their stop. The old git plants each
// one's start found (X12) are kept for Setup's Sandbox row (oldGitPlants, sandboxrow.go). Gone when the core is.
var keptSandboxes = struct {
	sync.Mutex
	of map[string]*deskSandbox
}{of: map[string]*deskSandbox{}}

// deskSandboxOf is desk id's sandbox as its start made it, kept by this core until the desk's stop. It doesn't say
// whether the desk runs (a desk started before the core's restart has none kept): deskStoppedNow does.
func deskSandboxOf(id string) (*deskSandbox, bool) {
	keptSandboxes.Lock()
	defer keptSandboxes.Unlock()
	s, ok := keptSandboxes.of[id]
	return s, ok
}

// deskStartListing is where a desk's start lists what it may plant before it runs: CLAUDEPLANT1's
// holdClaudePlants(s.places.OwnID, claudeLooks(s.policy, s.places), s.places.Checkout) goes here when it merges.
func deskStartListing(s *deskSandbox) {}

// deskStopped is said once desk h's session is known to have ended: a start that failed, a watch that gave up, an
// Attach failure, Take over's stop, and the engine's fire once it saw the session gone (hirefire.go, beside
// CLAUDEPLANT1's claudeHireEnded(h.ID)). What it may have planted is listed at its stop (deskStopListing), with its
// sandbox as its start made it: the one kept, or, for a desk this core didn't start (one from before the core's
// restart), one made again from its row and not kept. Then its sandbox is let go, and its socket folder taken away
// once nothing answers there: nothing of a stopped desk stays kept.
func deskStopped(h Hire, o engineOptions) {
	keptSandboxes.Lock()
	s, kept := keptSandboxes.of[h.ID]
	delete(keptSandboxes.of, h.ID)
	keptSandboxes.Unlock()
	if !kept {
		s = rowDeskSandbox(h.ID, o)
	}
	if s != nil {
		deskStopListing(s)
	}
	removeDeskSocketFolder(h.TmuxSocket)
	deskChecksForgotten(h.ID)
}

// deskStopListing is where a desk's stop lists what it may have planted, from its sandbox: CLAUDEPLANT1's merge puts
// holdClaudePlants(s.places.OwnID, claudeLooks(s.policy, s.places), s.places.Checkout) here, the home folders the
// Policy let it write as well as its place (its claudeHireEnded lists the place alone). Tests set it.
var deskStopListing = func(s *deskSandbox) {}

// rowDeskSandbox is desk id's sandbox made again from its row, as its start made it, without keeping it: nil for a
// desk whose row records no sandboxed start the core checked (a start refused before its server, or not a sandboxed
// desk), or whose sandbox can't be made now.
func rowDeskSandbox(id string, o engineOptions) *deskSandbox {
	h, ok := registryHire(id)
	if !ok || deskSocketID(h.TmuxSocket, testCopyPort()) != id || h.TmuxServer <= 1 {
		return nil
	}
	if o.Git == nil {
		o.Git = runGit
	}
	d := deskAtOf(h, deskSocketPath(id, testCopyPort()), claudeDenyProfiles(savedConfig().ClaudeProfiles), o)
	pl, err := deskPlaces(d)
	if err != nil {
		log.Printf("sandbox: desk %s's places at its stop: %v", id, err)
		return nil
	}
	// As its start found it. Not known (Smolcorp started again since): off, which denies nothing more, so the stop's
	// listing of what it could write looks at no less (RT126SAX14 nit 5: "on" would narrow the listing).
	pl.RemoteLogin = deskCheckedOf(id).RemoteLogin
	pol, err := sandbox.New(pl)
	if err != nil {
		log.Printf("sandbox: desk %s's policy at its stop: %v", id, err)
		return nil
	}
	return &deskSandbox{at: d, places: pl, policy: pol}
}

// deskAtOf is what desk h's start knows of it, from its row, as the engine's start had it: its worktree (else its
// folder), the checkout that is a worktree of, the hires' folder, its socket, the Claude accounts its fence denies.
func deskAtOf(h Hire, socket string, accounts []claudeProfile, o engineOptions) deskAt {
	d := deskAt{ID: h.ID, GM: h.Role == gmRole, Root: h.Folder, Socket: socket, Accounts: accounts, Brief: h.Brief, Git: o.Git,
		DatabaseOptIn: o.DatabaseOptIn}
	if h.Worktree != "" && h.Repo != "" {
		d.Root, d.Checkout = h.Worktree, h.Repo
	}
	if h.Repo != "" {
		d.Hires = hiresFolder(h.Repo, o.Worktrees)
	}
	return d
}

// deskStoppedNow is whether desk h is known to be stopped now, from its row alone, whatever this core kept (for
// CLAUDEPLANT1's "remove only while the desk is stopped"): a desk in its sandbox whose server, as the core checked it
// at its start, is gone, and whose socket doesn't answer. Nothing is run. Anything else isn't known stopped: another
// kind of desk, a row with no checked server, a pid that's there (its own or one used again), a socket that answers.
func deskStoppedNow(h Hire) bool {
	testPort := testCopyPort()
	if deskSocketID(h.TmuxSocket, testPort) != h.ID || h.TmuxServer <= 1 {
		return false
	}
	if err := syscall.Kill(h.TmuxServer, 0); !errors.Is(err, syscall.ESRCH) {
		return false
	}
	conn, err := net.DialTimeout("unix", deskSocketPath(h.ID, testPort), 100*time.Millisecond)
	if err == nil {
		conn.Close()
		return false
	}
	return true
}

// deskPlaces is what the core knows of where desk d works, as the sandbox's Places: every path resolved, as the
// kernel checks it.
func deskPlaces(d deskAt) (sandbox.Places, error) {
	own, err := deskOwnID(runtime.GOOS, d.ID)
	if err != nil {
		return sandbox.Places{}, err
	}
	pl := sandbox.Places{Subject: sandbox.Hire, Home: physical(homeDir), Support: physical(supportDir), OwnID: own,
		Root: physical(d.Root), SocketParent: physical(deskSocketParent()), OwnSockets: physical(filepath.Dir(d.Socket))}
	if d.GM {
		pl.Subject = sandbox.GM
	}
	start := deskStartSocket()
	pl.DeskStart = filepath.Join(physical(filepath.Dir(start)), filepath.Base(start))
	if testCopyPort() != 0 {
		// A test copy's desks are denied the owner's own folder with no exception.
		if base, err := os.UserConfigDir(); err == nil {
			pl.OtherSupports = []string{physical(filepath.Join(base, supportDirName(runtime.GOOS)))}
		}
	}
	if d.Checkout != "" {
		pl.Checkout = physical(d.Checkout)
	}
	if d.Hires != "" {
		pl.HiresFolder = physical(d.Hires)
	}
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.Role == gmRole && h.ID != d.ID && isOpen(h) {
			pl.GMFolder = physical(startedIn(h))
		}
	}
	pl.Accounts = deskAccounts(runtime.GOOS, d.Accounts)
	if err := deskGitPlaces(d, &pl); err != nil {
		return pl, err
	}
	finding := deskFinding()
	finding.Also = append(finding.Also, d.Programs...)
	programs, err := sandbox.ResolvePrograms(finding)
	if err != nil {
		return pl, err
	}
	pl.InstallRoots = programs.InstallRoots()
	git := programs.Path("git")
	if git == "" {
		return pl, nil // no git: nothing reads a git config
	}
	pl.GitConfigs = gitConfigFiles(git)
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	pl.GitHooks, pl.GitNamed, err = sandbox.ReadGitConfig(ctx, git, nil, pl)
	return pl, err
}

// deskAccounts are the Claude account folders besides ~/.claude, for Places.Accounts on goos: each resolved, and on a
// Mac one kept as a link as it is spelled too (RT126RESOLVESWEEP S2b), so the Policy holds the link itself, which no
// desk can then point elsewhere for the owner's later sessions, as well as where it leads. On Linux the view already
// makes such a link again, read-only, in the covered home folder and covers where it leads whole; given as spelled, it
// would be a place the view must hold that is a link, and every start would fail (RT126HOMELINKS B3).
func deskAccounts(goos string, accounts []claudeProfile) []string {
	var dirs []string
	for _, account := range accounts {
		if account.Dir == "" || physical(account.Dir) == physical(filepath.Join(homeDir, ".claude")) {
			continue
		}
		dirs = append(dirs, physical(account.Dir))
		if spelled, err := filepath.Abs(account.Dir); goos == "darwin" && err == nil && spelled != physical(account.Dir) {
			dirs = append(dirs, spelled)
		}
	}
	return dirs
}

// deskOwnID is the id desk id's sandbox knows it by (Places.OwnID): its own folder hires/<id>, and on Linux its kept
// copy and its branch folder refs/heads/hire/<id>, the only one its view lets it write. There it is the slug hireBranch
// names its branch by, the GM's as well as a hire's (RMSYLCHAIN F4), or the desk commits nowhere. The core's ids are
// their own slug; a desk whose id isn't has no sandbox on Linux, rather than one whose folders don't match.
func deskOwnID(goos, id string) (string, error) {
	if goos != "linux" || hireSlug(id) == id {
		return id, nil
	}
	return "", errors.New("desk " + strconv.Quote(id) + "'s id isn't the slug its branch is named by (" + hireSlug(id) + ")")
}

// deskKeptFolder is the kept copy a desk's guard is told of (--held), for a desk the engine starts in its sandbox on
// goos (sandboxed): a Linux desk's start keeps a copy of the places no mount holds in its view (sandbox.SaveKept, from
// PrepareBwrap), and its guard checks them against it before and after each call (L1GUARD1, the release gate's item
// 4b). "" for a desk that keeps none: on a Mac, or a desk not started in its sandbox.
func deskKeptFolder(goos string, sandboxed bool, id string) string {
	if goos != "linux" || !sandboxed {
		return ""
	}
	return sandbox.KeptFolder(physical(supportDir), hireSlug(id))
}

// deskGitPlaces fills the repo d works in: its shared .git, its own registration in it and the others', and the
// default branch. A folder that isn't in a repo has none.
func deskGitPlaces(d deskAt, pl *sandbox.Places) error {
	status, out, ran := d.Git(pl.Root, "rev-parse", "--path-format=absolute", "--git-common-dir", "--git-dir")
	if !ran {
		return errors.New("git couldn't be asked where the repo is")
	}
	lines := strings.Split(out, "\n")
	if status != 0 || len(lines) != 2 || !filepath.IsAbs(lines[0]) || !filepath.IsAbs(lines[1]) {
		return nil
	}
	pl.GitDir = physical(lines[0])
	own := physical(lines[1])
	worktrees := filepath.Join(pl.GitDir, "worktrees")
	if filepath.Dir(own) == worktrees {
		pl.OwnWorktree = filepath.Base(own)
	}
	entries, err := os.ReadDir(worktrees)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, e := range entries {
		if e.Name() != pl.OwnWorktree {
			pl.OtherWorktrees = append(pl.OtherWorktrees, e.Name())
		}
	}
	pl.DefaultBranch = sandbox.DefaultBranch(pl.GitDir)
	return nil
}

// deskFinding is where the core finds the programs it runs, for their install roots (SA-X2): its PATH, then the one
// your login shell sets, itself, and every hire's recorded tmux.
func deskFinding() sandbox.Finding {
	f := sandbox.Finding{Home: physical(homeDir), Search: append(filepath.SplitList(os.Getenv("PATH")), tmuxLoginPath()...),
		DeveloperDir: os.Getenv("DEVELOPER_DIR"), XcrunDB: os.Getenv("xcrun_db")}
	if program, err := executable(); err == nil {
		f.Executable = program
	}
	if temp := os.Getenv("TMPDIR"); filepath.IsAbs(temp) {
		f.UserTemp = temp // the TMPDIR launchd gives the app: confstr's _CS_DARWIN_USER_TEMP_DIR
	}
	for _, h := range readRegistry(registryPath(), false).hires {
		if filepath.IsAbs(h.TmuxProgram) && !slices.ContainsFunc(f.Also, func(p sandbox.Program) bool { return p.Path == h.TmuxProgram }) {
			f.Also = append(f.Also, sandbox.Program{Name: "tmux", Path: h.TmuxProgram})
		}
	}
	return f
}

// systemConfigNamed is the file git names when it lists, or can't read, its system config.
var systemConfigNamed = regexp.MustCompile(`(?m)^file:([^\t\n]+)\t|unable to read config file '([^'\n]+)'`)

// gitConfigFiles are the config files git reads for every repo besides the home folder's (Places.GitConfigs): its
// system config, named whether it is there or not (git names a missing one in its error), and
// $XDG_CONFIG_HOME/git/config when that isn't ~/.config's.
func gitConfigFiles(git string) []string {
	var files []string
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := disclaim.Cmd(exec.CommandContext(ctx, git, "config", "--system", "--list", "--show-origin"))
	cmd.Dir = "/"
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	_ = cmd.Run()
	if m := systemConfigNamed.FindSubmatch(out.Bytes()); m != nil {
		if named := string(append(m[1], m[2]...)); filepath.IsAbs(named) {
			files = append(files, named)
		}
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(xdg) && physical(xdg) != physical(filepath.Join(homeDir, ".config")) {
		files = append(files, filepath.Join(xdg, "git", "config"))
	}
	return files
}

// sandboxStartWait is how long a sandboxed start may take: its handshake, then tmux's start call.
var sandboxStartWait = 10 * time.Second
