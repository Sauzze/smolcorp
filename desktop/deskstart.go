//go:build !windows

package desktop

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// A desk's start ticket (SANDBOX-PLAN §3.4, SG2; §4 g5). A desk used to start from a script in the support
// folder that held its send key, the GM's token or the API key, run through your login shell: any program of
// yours could read or change it between its writing and its running, and your shell's start files ran first.
// Now nothing is written down, and no shell runs. The core starts the desk's tmux pane on
// `smolcorp desk-start <socket> <core pid> <ticket>`, which tmux runs as it is (several words: no shell), and with
// none of the owner's tmux start files. That helper connects to the core's socket, checks the other end is the
// core by its pid, and gives the ticket. The core takes a ticket once, and only from the very process tmux
// started for it (its pid, said by the call that started it), while the pane still runs that process, and only
// when that process runs Smolcorp's own program (deskPeerRuns: on a Mac, by the code the system says it runs; on
// Linux, by its file). Any program of the owner's can drive the desk's tmux and swap what its pane runs, and can
// read the ticket from `ps`, but a process it starts is never the one the ticket is for (RT126SG2 B1). What it can
// still do through tmux once the desk runs is §5's (SANDBOX-PLAN §5.1). Only then does the core find
// the agent's program and check it (agentcheck.go), check Smolcorp's own files (ownseal.go), and hand over the
// desk's whole start: the program, its words, and its environment, built here, not inherited. The helper execs
// it, keeping only the terminal's own variables from tmux. tmux's session environment holds no key.

// deskStartWait is how long a ticket waits for its pane, as a hire has to report.
const deskStartWait = hireStartWithin

// deskStartAnswerMost is the longest start the helper reads: a desk's settings are its longest part.
const deskStartAnswerMost = 4 << 20

// deskLaunch is the start handed to the pane's helper, once.
type deskLaunch struct {
	Program string   `json:"program,omitempty"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	// Error is why nothing starts, in words the helper prints.
	Error string `json:"error,omitempty"`
}

type deskTicket struct {
	hireID string
	cli    string // Smolcorp's own program, which the pane runs as the helper
	tmux   *hireTmux
	plan   deskPlan
	key    func() string // the API key, for a plan that reads it
	made   time.Time
	// pinned is closed once tmux has said which process it started for the ticket (pin); pid is that process, read
	// only after pinned is closed. Every try waits on it, so a try by another process doesn't take the pid away.
	pinned chan struct{}
	pid    int
}

// deskStarter is the core's side: its socket, and the tickets not yet taken.
type deskStarter struct {
	mu       sync.Mutex
	path     string
	listener net.Listener
	tickets  map[string]*deskTicket
	// refused is why each desk refused at its start didn't start, for its watch to say.
	refused map[string]string
	// unpinned are the tickets whose process tmux hasn't said yet, by hire.
	unpinned map[string]*deskTicket
}

var deskStarts = &deskStarter{tickets: map[string]*deskTicket{}, refused: map[string]string{}, unpinned: map[string]*deskTicket{}}

// deskPinWait is how long a ticket given waits for tmux to say which process it was started for: as long as the
// call that starts the pane may take.
const deskPinWait = tmuxCallWait

// deskProgram is where an agent's program is, checked: "" with nil when it isn't installed. A variable, for the
// tests, which never find a real one.
var deskProgram = func(agent string) (string, error) {
	program := programNamed(agent)
	return program, checkAgentProgram(agent, program)
}

// deskStartSocket is where the core listens: the support folder, or tmux's own folder when the support folder's
// path is too long for a socket (a test copy deep in a worktree).
func deskStartSocket() string {
	if path := filepath.Join(supportDir, "desk-start"); len(path) <= maxSocketPath {
		return path
	}
	return filepath.Join(hireSocketFolder(), "smolcorp.desk-start."+strconv.Itoa(os.Getpid()))
}

// listening makes sure the core listens on its socket, as it is: listened on again when its file is gone or isn't
// the core's (a program of yours removed it, or listens in its place: the helper would refuse that one). With s.mu.
func (s *deskStarter) listening() error {
	if s.listener != nil {
		if info, err := os.Lstat(s.path); err == nil && info.Mode()&os.ModeSocket != 0 && s.path == deskStartSocket() && s.ownSocket() {
			return nil
		}
		s.listener.Close()
		s.listener = nil
	}
	path := deskStartSocket()
	_ = os.Remove(path)
	listener, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return err
	}
	s.path, s.listener = path, listener
	go s.serve(listener)
	return nil
}

// ownSocket is whether the file at s.path is still the one listened on: a connection reaches this core.
func (s *deskStarter) ownSocket() bool {
	conn, err := net.DialTimeout("unix", s.path, time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	pid, err := peerPid(conn)
	return err == nil && pid == os.Getpid()
}

// issue makes hire hireID's ticket for plan, its pane on tmux, and says the pane's command: cli's desk-start.
func (s *deskStarter) issue(cli, hireID string, tmux *hireTmux, plan deskPlan, key func() string) ([]string, error) {
	if cli == "" || !filepath.IsAbs(cli) {
		return nil, errors.New("Smolcorp's own program wasn't found, so the desk can't be started.")
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	ticket := hex.EncodeToString(secret)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.listening(); err != nil {
		return nil, errors.New("Smolcorp couldn't open its start socket, so the desk can't be started.")
	}
	// A desk's new start takes the place of any ticket of its own left untaken; old tickets go.
	for old, t := range s.tickets {
		if t.hireID == hireID || time.Since(t.made) > deskStartWait {
			delete(s.tickets, old)
		}
	}
	delete(s.refused, hireID)
	t := &deskTicket{hireID: hireID, cli: cli, tmux: tmux, plan: plan, key: key, made: time.Now(), pinned: make(chan struct{})}
	s.unpinned[hireID] = t
	s.tickets[ticket] = t
	return []string{cli, "desk-start", s.path, strconv.Itoa(os.Getpid()), ticket}, nil
}

// pin says which process tmux started for hire hireID's ticket: its pid, as the call that started it said. The
// ticket opens for that process alone.
func (s *deskStarter) pin(hireID string, pid int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t := s.unpinned[hireID]; t != nil {
		delete(s.unpinned, hireID)
		t.pid = pid
		close(t.pinned)
	}
}

// forget takes hire hireID's ticket away, taken or not, and says why it was refused, if it was.
func (s *deskStarter) forget(hireID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ticket, t := range s.tickets {
		if t.hireID == hireID {
			delete(s.tickets, ticket)
		}
	}
	delete(s.unpinned, hireID)
	why := s.refused[hireID]
	delete(s.refused, hireID)
	return why
}

func (s *deskStarter) serve(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go s.answer(conn)
	}
}

// answer takes one ticket from conn and answers its start.
func (s *deskStarter) answer(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	line, err := bufio.NewReaderSize(conn, 256).ReadSlice('\n')
	if err != nil {
		return
	}
	peer, err := peerPid(conn)
	if err != nil {
		peer = 0
	}
	launch := s.redeem(strings.TrimSpace(string(line)), peer)
	answer, _ := json.Marshal(launch)
	_, _ = conn.Write(append(answer, '\n'))
}

// Why a start ticket opens nothing.
const (
	deskTicketUnknown = "This desk's start ticket isn't Smolcorp's, or was used already: nothing was started. Hire again."
	deskTicketLate    = "This desk's start ticket came too late: nothing was started. Hire again."
	deskTicketNotPane = "This desk's start ticket was given by a program that isn't the desk's own window, so it was thrown away and nothing was started. Hire again."
	// deskTicketNotTheWindow is a try by a process that isn't the one tmux started for the ticket: the ticket stays
	// for that one (any desk can read another's ticket from ps: RT126SWEEP3 F3).
	deskTicketNotTheWindow = "This desk's start ticket was given by a program that isn't the desk's own window, so nothing was started for it. Hire again."
	deskTicketNotOurs      = "This desk's start ticket was given by a program that isn't Smolcorp's own, so it was thrown away and nothing was started. Hire again."
)

// deskPeerRuns is whether process pid runs Smolcorp's own program, cli (peerRunsProgram: agentcheck_darwin.go,
// peerpid_linux.go). A variable, for the tests whose stand-in helper is the test itself.
var deskPeerRuns = peerRunsProgram

// redeem takes ticket from the process peer, once, and says the desk's start, or why there is none. The ticket is
// spent only by a try of the process tmux started for it (or once it's late, or tmux said no process it could
// have started): any desk can read another's ticket from ps, and its try mustn't stop that desk's start
// (RT126SWEEP3 F3). Another process's try is refused, and the ticket stays for its own.
func (s *deskStarter) redeem(ticket string, peer int) deskLaunch {
	s.mu.Lock()
	t := s.tickets[ticket]
	s.mu.Unlock()
	if t == nil {
		return deskLaunch{Error: deskTicketUnknown}
	}
	// spent is whether this try spent the ticket: then why it opened nothing is the desk's.
	spent := false
	// refuse says why, and tells the desk's watch while the ticket is still the desk's and this try's say is its:
	// a try that spent it, or one while it's there unspent. Another process's try that ends after the desk's own
	// start, or after the desk's new ticket took this one's place, says nothing to the watch (RT126SWEEP3FIXP2 O2).
	refuse := func(why string) deskLaunch {
		s.mu.Lock()
		if spent || s.tickets[ticket] == t {
			s.refused[t.hireID] = why
		}
		s.mu.Unlock()
		return deskLaunch{Error: why}
	}
	// spend takes the ticket away, once: false when another try spent it first.
	spend := func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.tickets[ticket] != t {
			return false
		}
		delete(s.tickets, ticket)
		spent = true
		return true
	}
	if time.Since(t.made) > deskStartWait {
		spend()
		return refuse(deskTicketLate)
	}
	// The process tmux started for the ticket, and no other: one the pane runs in its place since (respawned by a
	// program of the owner's, say) has another pid.
	select {
	case <-t.pinned:
	case <-time.After(deskPinWait):
		// Not said yet: whose try this is can't be known, so the ticket stays (it goes once late).
		return refuse(deskTicketNotTheWindow)
	}
	started := t.pid
	if started <= 1 {
		spend()
		return refuse(deskTicketNotPane)
	}
	if peer != started {
		return refuse(deskTicketNotTheWindow)
	}
	if !spend() {
		return deskLaunch{Error: deskTicketUnknown}
	}
	if pane, ok := t.tmux.panePID(); !ok || pane != started {
		return refuse(deskTicketNotPane)
	}
	if !deskPeerRuns(peer, t.cli) {
		return refuse(deskTicketNotOurs)
	}
	if refusal := ownSealRefusal(); refusal != nil {
		return refuse(refusal.Message)
	}
	// Its settings file, as Smolcorp sealed it, at the last moment before its start (settingsfile.go).
	if t.plan.Settings != (sealedSettings{}) {
		if refusal := t.plan.Settings.refusal(); refusal != nil {
			return refuse(refusal.Message)
		}
	}
	program, err := deskProgram(t.plan.Agent)
	if err != nil {
		return refuse(err.Error())
	}
	if program == "" {
		return refuse(Agent(t.plan.Agent).Name() + " isn't installed where Smolcorp looks for it, so the desk didn't start.")
	}
	env, err := deskEnvironment(deskBaseEnvironment(), t.plan, t.key)
	if err != nil {
		return refuse(err.Error())
	}
	s.mu.Lock()
	delete(s.refused, t.hireID) // another process's try, before this one, isn't why it didn't start
	s.mu.Unlock()
	return deskLaunch{Program: program, Args: slices.Clone(t.plan.Words), Env: env}
}

// deskBaseEnvironment is what every desk's environment starts from: what tmux would give it (hireEnvironment),
// without what wraps hooks, its PATH this program's and then the folders your login shell's PATH names (asked
// once, with nothing of yours around it: opencode.go), so the agent and its tools find what a terminal finds.
func deskBaseEnvironment() []string {
	env := slices.DeleteFunc(hireEnvironment(), func(variable string) bool {
		name, _, _ := strings.Cut(variable, "=")
		return wrapsHooks(name) || name == "PATH"
	})
	folders := filepath.SplitList(os.Getenv("PATH"))
	for _, folder := range askLoginPath() {
		if !slices.Contains(folders, folder) {
			folders = append(folders, folder)
		}
	}
	env = append(env, "PATH="+strings.Join(folders, string(os.PathListSeparator)))
	if deskSandboxed {
		env = sandbox.DeskGitConfig(env) // its MERGE_RR, origin/HEAD and tags are denied (FXGITROWS2, B3; FXGITROWS2R1, O1; FXGITROWS2R4)
	}
	return env
}

// deskEnvironment is base with plan's changes made, in order, and the API key read when the plan says so. On a Mac the
// app's shims folder goes before PathFirst on PATH (shimsBeside).
func deskEnvironment(base []string, plan deskPlan, key func() string) ([]string, error) {
	env := slices.Clone(base)
	drop := func(name string) {
		env = slices.DeleteFunc(env, func(variable string) bool { found, _, _ := strings.Cut(variable, "="); return found == name })
	}
	for _, change := range plan.Changes {
		drop(change.Name)
		if !change.Unset {
			env = append(env, change.Name+"="+change.Value)
		}
	}
	if plan.PathFirst != "" {
		first := plan.PathFirst
		if shims := shimsBeside(plan.PathFirst); deskSandboxed && shims != "" {
			first = shims + string(os.PathListSeparator) + first
		}
		for i, variable := range env {
			if value, ok := strings.CutPrefix(variable, "PATH="); ok {
				env[i] = "PATH=" + first + string(os.PathListSeparator) + value
			}
		}
	}
	if plan.APIKey {
		text := ""
		if key != nil {
			text = key()
		}
		if !isHireKeyText(text) {
			return nil, errors.New("Smolcorp couldn't read the API key from the secret store, so the desk didn't start.")
		}
		drop("ANTHROPIC_API_KEY")
		env = append(env, "ANTHROPIC_API_KEY="+text)
	}
	return env, nil
}

// shimsBeside is the app's dev-tool shims folder (SA-T2, SANDBOX-PLAN §A4.3, AQ9) for cliDir, the folder of the app's
// own smolcorp: <app>.app/Contents/Resources/shims when cliDir is that app's Contents/Resources/bin and the folder is
// there, as a folder and not a link. On a Mac it goes first on every desk's PATH, before cliDir, so a desk's `swift`
// and `xcodebuild` turn off the sandbox of their own that can't start inside the desk's (Resources/shims). Only an
// app's: the Policy denies the app the core runs from whole to every desk (sandbox.ResolvePrograms), so no desk can
// change what its PATH runs first. "" for a smolcorp that isn't in an app (a build of your own: its desks have their
// brief's lines alone, briefSandboxTools), or an app with no shims (one built before them).
func shimsBeside(cliDir string) string {
	resources := filepath.Dir(cliDir)
	contents := filepath.Dir(resources)
	if filepath.Base(cliDir) != "bin" || filepath.Base(resources) != "Resources" || filepath.Base(contents) != "Contents" ||
		!strings.HasSuffix(strings.ToLower(filepath.Dir(contents)), ".app") || !filepath.IsAbs(cliDir) {
		return ""
	}
	shims := filepath.Join(resources, "shims")
	if info, err := os.Lstat(shims); err != nil || !info.IsDir() {
		return ""
	}
	return shims
}

// terminalVariables are what the helper keeps of its own environment: tmux's, saying which terminal it's in.
var terminalVariables = []string{"TERM", "COLORTERM", "TMUX", "TMUX_PANE"}

// RunDeskStart is `smolcorp desk-start <socket> <core pid> <ticket>`, a desk pane's first and only process
// before its agent: it takes the desk's start from the core and execs it, or says why not.
func RunDeskStart(args []string) int {
	launch, err := takeDeskStart(args)
	if err == nil && launch.Error != "" {
		err = errors.New(launch.Error)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "smolcorp:", err)
		return 1
	}
	env := slices.Clone(launch.Env)
	for _, name := range terminalVariables {
		if value, ok := os.LookupEnv(name); ok {
			env = append(slices.DeleteFunc(env, func(v string) bool { return strings.HasPrefix(v, name+"=") }), name+"="+value)
		}
	}
	err = syscall.Exec(launch.Program, append([]string{launch.Program}, launch.Args...), env)
	fmt.Fprintln(os.Stderr, "smolcorp: the desk's agent couldn't be started:", err)
	return 1
}

// takeDeskStart asks the core at args' socket, by its pid, for the start args' ticket opens.
func takeDeskStart(args []string) (deskLaunch, error) {
	var launch deskLaunch
	if len(args) != 3 {
		return launch, errors.New("usage: smolcorp desk-start <socket> <core pid> <ticket> (Smolcorp starts this itself)")
	}
	core, err := strconv.Atoi(args[1])
	if err != nil || core <= 1 {
		return launch, errors.New("that isn't Smolcorp's pid")
	}
	conn, err := net.DialTimeout("unix", args[0], 5*time.Second)
	if err != nil {
		return launch, errors.New("Smolcorp isn't running, so the desk didn't start")
	}
	defer conn.Close()
	// The ticket goes only to the core that made it: a program listening in its place gets nothing.
	if peer, err := peerPid(conn); err != nil || peer != core {
		return launch, errors.New("the start socket isn't Smolcorp's, so the desk didn't start")
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := conn.Write([]byte(args[2] + "\n")); err != nil {
		return launch, errors.New("Smolcorp didn't take the start ticket")
	}
	reader := bufio.NewReaderSize(conn, 64<<10)
	var text []byte
	for {
		part, more, err := reader.ReadLine()
		if err != nil {
			return launch, errors.New("Smolcorp didn't answer")
		}
		if text = append(text, part...); len(text) > deskStartAnswerMost {
			return launch, errors.New("Smolcorp's answer is too long")
		}
		if !more {
			break
		}
	}
	if err := json.Unmarshal(text, &launch); err != nil {
		return deskLaunch{}, errors.New("Smolcorp's answer couldn't be read")
	}
	if launch.Error == "" && !filepath.IsAbs(launch.Program) {
		return deskLaunch{}, errors.New("Smolcorp's answer names no program")
	}
	return launch, nil
}
