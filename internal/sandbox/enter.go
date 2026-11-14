package sandbox

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// `smolcorp sandbox enter` (SA-X3, SANDBOX-PLAN §A5.2): the first thing every desk runs inside its sandbox, before
// anything of the desk's. It checks the sandbox holds, with rows the core chose for this start: the start's own canary
// can't be read, with the system's exact error; the fences can't be opened for writing; a stand-in other server can't
// be reached; and, so a sandbox that denies everything doesn't pass, the desk's brief reads and its status writes.
// Every file descriptor above 2 is marked to close at the exec. It writes the rows' results and the start's nonce to a
// pipe only the core holds, closes it, and becomes the desk's program in place. A row that fails: the report says
// which, nothing runs, and the owner sees one sentence (Refusal).
//
// The core's side is Start: it makes the canary fresh for each start (NewCanary, write-new-then-rename), the nonce,
// the pipe and the other server, gives the words and the environment to start `enter` with, and marks the desk
// "starting" only when Wait has read a full, passing report for this start. The report is compared whole with the one
// the core expects, so a row missing, added, reordered or failed, another start's nonce, or anything after the end, is
// refused like no report at all. Starting `enter` inside the desk's sandbox, and checking the server is ours after it
// (§A5.2 steps 8 and 9), are each system's runner's (SA-M3, SA-L2) and the start wiring's (SA-M6).

const (
	// nonceVar is where the core puts the start's nonce for `enter`, which takes it out before the exec.
	nonceVar = "SMOLCORP_SANDBOX_NONCE"
	// tmuxGuardVar is the tmux guard's variable (third_party/tmux/guard.patch): "off" only for what `enter` starts,
	// the desk's own server, inside its sandbox (§A3.1).
	tmuxGuardVar = "SMOLCORP_TMUX_SERVER"
	// handshakeFD is the pipe's descriptor in `enter`: the first of the command's ExtraFiles (Start.Files).
	handshakeFD = 3
	reportHead  = "smolcorp sandbox enter 1"
	reportEnd   = "end"
	// reportMost is the longest report the core reads: a row a path long, a few dozen rows.
	reportMost = 64 << 10
)

// The kinds of row, as the core passes them and the report names them. The canary is always first, and the
// descriptors always last.
const (
	rowCanary    = "canary"     // the canary can't be read, with the system's exact error
	rowNoWrite   = "no-write"   // a fence that's there can't be opened for writing (no O_CREAT)
	rowNoConnect = "no-connect" // the stand-in other server can't be reached
	rowRead      = "read"       // its brief reads
	rowWrite     = "write"      // its status writes
	rowFDs       = "fds"        // nothing above 2 reaches the program
)

// row is one check: its kind and the path it's tried on ("" for the descriptors).
type checkRow struct{ kind, path string }

// line is the row's line in a report.
func (r checkRow) line(result string) string {
	return r.kind + " " + strconv.Quote(r.path) + " " + result
}

// passed is a row's result when it held. Any other result is a failure, said in words.
const passed = "ok"

// report is the whole report for rows with results, in order.
func report(nonce string, rows []checkRow, results []string) []byte {
	var b bytes.Buffer
	b.WriteString(reportHead + " " + nonce + "\n")
	for i, r := range rows {
		b.WriteString(r.line(results[i]) + "\n")
	}
	b.WriteString(reportEnd + "\n")
	return b.Bytes()
}

// Refused is a start the check refused: why, for the log. The owner sees Refusal.
type Refused struct{ Why string }

func (r *Refused) Error() string { return "sandbox: the start's check didn't pass: " + r.Why }

// Refusal is what the owner sees, on the card and in Setup's Sandbox row, when a check row fails or no handshake
// comes (§A5.2's table).
func Refusal(goos string) string {
	if goos == "darwin" {
		return "The sandbox didn't hold on this Mac, so the hire wasn't started."
	}
	return "The sandbox didn't hold on this computer, so the hire wasn't started."
}

// Canary is one start's canary: a file in the support folder the desk's sandbox must deny with no exception (the
// Policy's "the sandbox's canary" row), by its path, device and inode as the core made it.
type Canary struct {
	Path     string
	Dev, Ino uint64
}

// canaryText is all a canary holds: nothing secret.
const canaryText = "Smolcorp's sandbox canary: a desk's sandbox must not let it be read. It holds nothing secret.\n"

// ErrNoCanary is NewCanary's answer on a computer where `enter` has no sandbox to check (HasCanary is false): nothing
// is made.
var ErrNoCanary = errors.New("sandbox: no canary on this computer: `enter` has no sandbox to check here")

// NewCanary makes a fresh canary in support, under a new random name: written to a new file beside it, then renamed
// into place, so the canary the desk is checked against is always this start's.
func NewCanary(support string) (Canary, error) {
	if !HasCanary {
		return Canary{}, ErrNoCanary
	}
	word := make([]byte, 16)
	if _, err := rand.Read(word); err != nil {
		return Canary{}, err
	}
	return makeCanary(filepath.Join(support, "sandbox-canary-"+hex.EncodeToString(word)))
}

func makeCanary(path string) (Canary, error) {
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		return Canary{}, fmt.Errorf("sandbox: a canary can't be made at %s: something is there (%v)", path, err)
	}
	fresh := path + ".new" // a canary's name too: denied by the same row
	f, err := os.OpenFile(fresh, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Canary{}, err
	}
	defer os.Remove(fresh) // gone after the rename; left by a failure before it
	_, err = f.WriteString(canaryText)
	if err == nil {
		err = f.Sync()
	}
	made, statErr := f.Stat()
	if closeErr := f.Close(); err == nil {
		err = errors.Join(statErr, closeErr)
	}
	if err != nil {
		return Canary{}, err
	}
	if err := os.Rename(fresh, path); err != nil {
		return Canary{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return Canary{}, err
	}
	dev, ino, ok := fileID(info)
	if !ok || !info.Mode().IsRegular() || !os.SameFile(made, info) {
		os.Remove(path)
		return Canary{}, fmt.Errorf("sandbox: the canary at %s isn't the file just made", path)
	}
	return Canary{Path: path, Dev: dev, Ino: ino}, nil
}

// Remove takes the canary away when its desk ends, only while it is still the file the core made.
func (c Canary) Remove() error {
	info, err := os.Lstat(c.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if dev, ino, ok := fileID(info); !ok || dev != c.Dev || ino != c.Ino {
		return fmt.Errorf("sandbox: %s isn't the canary the core made: left as it is", c.Path)
	}
	return os.Remove(c.Path)
}

// Checks are what one start's `enter` tries. The core chooses them all; none can be left out.
type Checks struct {
	// Support is the support folder of the core that starts the desk: the canary is made there.
	Support string
	// NoWrite are fences that are there and that the desk must not open for writing: the guard file, the desk's
	// brief, and the worktree's .claude/settings.json when it has one. Each must exist when the start is made.
	NoWrite []string
	// Other is where the core listens as a stand-in other server for the start, a unix socket the desk's sandbox
	// doesn't let it reach (§A5.2 step 4). It must not exist yet.
	Other string
	// Read are files the desk must read (its brief); Write, files it must open for writing (its status).
	Read, Write []string
}

// Start is the core's side of one desk's start through `enter`.
type Start struct {
	Canary Canary
	nonce  string
	rows   []checkRow
	other  net.Listener
	// read is the core's end of the handshake; write is `enter`'s, which the core closes once `enter` has started.
	read, write *os.File
}

// NewStart makes a start's canary, nonce, other server and handshake. Close it when the start is over, whichever
// way; the canary stays until the desk ends (Canary.Remove), since the server check uses it.
func NewStart(c Checks) (*Start, error) {
	if c.Support == "" || len(c.NoWrite) == 0 || c.Other == "" || len(c.Read) == 0 || len(c.Write) == 0 {
		return nil, errors.New("sandbox: a start's check needs its support folder, a fence, the other server, and a file to read and one to write")
	}
	for _, fence := range c.NoWrite {
		if _, err := os.Lstat(fence); err != nil {
			return nil, fmt.Errorf("sandbox: the fence %s isn't there to check: %w", fence, err)
		}
	}
	word := make([]byte, 16)
	if _, err := rand.Read(word); err != nil {
		return nil, err
	}
	s := &Start{nonce: hex.EncodeToString(word)}
	var err error
	if s.Canary, err = NewCanary(c.Support); err != nil {
		return nil, err
	}
	s.rows = append(s.rows, checkRow{rowCanary, s.Canary.Path})
	for _, list := range []struct {
		kind  string
		paths []string
	}{{rowNoWrite, c.NoWrite}, {rowNoConnect, []string{c.Other}}, {rowRead, c.Read}, {rowWrite, c.Write}} {
		for _, at := range list.paths {
			s.rows = append(s.rows, checkRow{list.kind, at})
		}
	}
	s.rows = append(s.rows, checkRow{kind: rowFDs})
	if s.other, err = net.Listen("unix", c.Other); err != nil {
		s.Canary.Remove()
		return nil, err
	}
	if s.read, s.write, err = os.Pipe(); err != nil {
		s.other.Close()
		s.Canary.Remove()
		return nil, err
	}
	return s, nil
}

// Words are the words after Smolcorp's own program that start `enter`, which then becomes program with args.
// program must be a whole path: `enter` looks nothing up.
func (s *Start) Words(program string, args ...string) []string {
	words := []string{"sandbox", "enter", "--handshake", strconv.Itoa(handshakeFD),
		"--canary-id", strconv.FormatUint(s.Canary.Dev, 10) + ":" + strconv.FormatUint(s.Canary.Ino, 10)}
	for _, r := range s.rows {
		if r.kind != rowFDs {
			words = append(words, "--"+r.kind, r.path)
		}
	}
	return append(append(words, "--", program), args...)
}

// Env is the variable to add to `enter`'s environment: the start's nonce.
func (s *Start) Env() string { return nonceVar + "=" + s.nonce }

// Files are the command's ExtraFiles, as they are: the handshake's end for `enter`.
func (s *Start) Files() []*os.File { return []*os.File{s.write} }

// Wait reads `enter`'s report, once the command has started, for up to within. It is nil only for a full, passing
// report of this start's; anything else is a *Refused.
func (s *Start) Wait(within time.Duration) error {
	if s.write != nil {
		s.write.Close() // `enter` holds its own copy now: the report ends when it closes it
		s.write = nil
	}
	if err := s.read.SetReadDeadline(time.Now().Add(within)); err != nil {
		return &Refused{Why: "the handshake can't be waited for: " + err.Error()}
	}
	got, err := io.ReadAll(io.LimitReader(s.read, reportMost+1))
	if err != nil {
		return &Refused{Why: "no handshake: " + err.Error()}
	}
	results := make([]string, len(s.rows))
	for i := range results {
		results[i] = passed
	}
	want := report(s.nonce, s.rows, results)
	if subtle.ConstantTimeCompare(got, want) == 1 {
		return nil
	}
	return &Refused{Why: whyNot(got, want)}
}

// whyNot says, for the log, where a report isn't the one wanted: the first line that differs. A report for another
// start says so and no more.
func whyNot(got, want []byte) string {
	if len(got) == 0 {
		return "no handshake"
	}
	if len(got) > reportMost {
		return "a report too long"
	}
	gotLines, wantLines := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	if gotLines[0] != wantLines[0] {
		return "not this start's report"
	}
	for i := 1; i < len(wantLines); i++ {
		if i >= len(gotLines) {
			return "the report stops before " + strconv.Quote(wantLines[i])
		}
		if gotLines[i] != wantLines[i] {
			return "the report says " + strconv.Quote(gotLines[i]) + ", not " + strconv.Quote(wantLines[i])
		}
	}
	return "the report goes on after its end"
}

// Close ends the start's other server and the core's ends of the handshake. The canary stays.
func (s *Start) Close() {
	s.other.Close()
	s.read.Close()
	if s.write != nil {
		s.write.Close()
	}
}

// entering is `enter`'s words, read.
type entering struct {
	handshake int
	dev, ino  uint64
	rows      []checkRow
	program   string
	args      []string
}

// parseEnter reads `enter`'s words: the handshake, the canary's device and inode, the rows in order (the canary's
// first, as the core gives them), then -- and the program, by its whole path, with its words.
func parseEnter(args []string) (entering, error) {
	var e entering
	kinds := map[string]bool{rowCanary: true, rowNoWrite: true, rowNoConnect: true, rowRead: true, rowWrite: true}
	seen := map[string]bool{}
	for len(args) > 0 && args[0] != "--" {
		if len(args) < 2 || !strings.HasPrefix(args[0], "--") {
			return e, fmt.Errorf("sandbox enter: %q has no value", args[0])
		}
		name, value := strings.TrimPrefix(args[0], "--"), args[1]
		args = args[2:]
		switch {
		case name == "handshake":
			fd, err := strconv.Atoi(value)
			if err != nil || fd <= 2 {
				return e, fmt.Errorf("sandbox enter: the handshake %q isn't a descriptor above 2", value)
			}
			e.handshake = fd
		case name == "canary-id":
			dev, ino, ok := strings.Cut(value, ":")
			var errDev, errIno error
			e.dev, errDev = strconv.ParseUint(dev, 10, 64)
			e.ino, errIno = strconv.ParseUint(ino, 10, 64)
			if !ok || errDev != nil || errIno != nil {
				return e, fmt.Errorf("sandbox enter: the canary's id %q isn't device:inode", value)
			}
		case kinds[name]:
			if (name == rowCanary) != (len(e.rows) == 0) || !filepath.IsAbs(value) {
				return e, fmt.Errorf("sandbox enter: --%s %q is out of place, or not a whole path", name, value)
			}
			e.rows = append(e.rows, checkRow{name, value})
			seen[name] = true
		default:
			return e, fmt.Errorf("sandbox enter: %q isn't a word it takes", args[0])
		}
	}
	if len(args) < 2 || !filepath.IsAbs(args[1]) {
		return e, errors.New("sandbox enter: no program after --, by its whole path")
	}
	for kind := range kinds {
		if !seen[kind] {
			return e, fmt.Errorf("sandbox enter: no --%s row: a check left out isn't a check", kind)
		}
	}
	if e.handshake == 0 || e.ino == 0 {
		return e, errors.New("sandbox enter: no handshake, or no canary id")
	}
	e.rows = append(e.rows, checkRow{kind: rowFDs})
	e.program, e.args = args[1], args[1:]
	return e, nil
}

// Run is `smolcorp sandbox …`. Not in the usage: Smolcorp starts it, inside a desk's sandbox.
func Run(args []string, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "enter" {
		fmt.Fprintln(stderr, "smolcorp sandbox enter: Smolcorp runs this itself, inside a desk's sandbox")
		return 2
	}
	return enter(args[1:], stderr)
}

// refuseHere is what `enter` says before it stops, on the desk's terminal.
func refuseHere(stderr io.Writer, why string) int {
	fmt.Fprintln(stderr, Refusal(runtime.GOOS), "("+why+")")
	return 1
}
