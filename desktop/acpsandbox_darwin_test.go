package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/acp"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// SA-M11: an ACP desk on a Mac. Its agent starts through the sandbox's runner, its fs/* is put to its Policy and the
// core's reader, and its terminal/create runs through the runner too: first on stand-ins of each step, then end to end
// through the real /usr/bin/sandbox-exec, the desk's real profile and `enter`, with this test program as the agent
// (owner Q5 (a), as SA-M3's and SA-M6's tests).

// needSandboxExec skips a test that runs the real sandbox-exec where it isn't Apple's alone, except on the runner.
func needSandboxExec(t *testing.T) {
	t.Helper()
	if err := sandbox.CheckSandboxExec(); err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal(err)
		}
		t.Skipf("this Mac's sandbox-exec isn't Apple's alone: %v", err)
	}
}

// acpDeskFiles is a desk's stand-in places for its agent's files: a readme in its folder, a stand-in key in the
// support folder, a link and a second name for that key in the desk's folder, and a stand-in token in the support
// folder with no other name (so only the Policy, not the reader's one-name rule, keeps it from the desk).
type acpDeskFiles struct{ readme, key, link, hard, token string }

func standInACPFiles(t *testing.T, s *deskSandbox) acpDeskFiles {
	t.Helper()
	root := s.places.Root
	f := acpDeskFiles{readme: filepath.Join(root, "readme.txt"), key: filepath.Join(physical(supportDir), "stand-in-key"),
		link: filepath.Join(root, "key-link"), hard: filepath.Join(root, "key-hard"),
		token: filepath.Join(physical(supportDir), "stand-in-token")}
	for path, text := range map[string]string{f.readme: "hello\nfrom\nthe desk\n", f.key: "FAKEKEY", f.token: "FAKEKEY-TOKEN"} {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(f.key, f.link); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(f.key, f.hard); err != nil {
		t.Skipf("no second name for the key here (another volume?): %v", err)
	}
	return f
}

// keyIntact fails t unless the stand-in key is as it was made.
func keyIntact(t *testing.T, f acpDeskFiles) {
	t.Helper()
	if text, err := os.ReadFile(f.key); err != nil || string(text) != "FAKEKEY" {
		t.Errorf("the stand-in key changed: %q %v", text, err)
	}
}

func TestAnACPDesksFileReadsKeepToItsPolicyAndFollowNoLink(t *testing.T) {
	s := standInDesk(t)
	f := standInACPFiles(t, s)
	files := acpFiles{desk: s}
	ctx := context.Background()
	read := func(path string, line, limit *int) (string, error) {
		resp, err := files.ReadTextFile(ctx, acp.ReadTextFileRequest{SessionID: "s1", Path: path, Line: line, Limit: limit})
		return resp.Content, err
	}
	if text, err := read(f.readme, nil, nil); err != nil || text != "hello\nfrom\nthe desk\n" {
		t.Errorf("its own readme: %q %v", text, err)
	}
	two, one := 2, 1
	if text, err := read(f.readme, &two, &one); err != nil || text != "from\n" {
		t.Errorf("line 2, one line: %q %v", text, err)
	}
	// Its folder spelled through /var's link: the same file.
	if text, err := read(strings.Replace(f.readme, "/private/var/", "/var/", 1), nil, nil); err != nil || text == "" {
		t.Errorf("its readme by another spelling: %q %v", text, err)
	}
	up, _ := filepath.Rel(s.places.Root, f.key)
	for name, path := range map[string]string{
		"the stand-in key":              f.key,
		"the one-name stand-in token":   f.token,
		"a link to it in its folder":    f.link,
		"a second name for it":          f.hard,
		"the stand-in key through ..":   s.places.Root + "/" + up,
		"a path that isn't whole":       "readme.txt",
		"a file that isn't there":       filepath.Join(s.places.Root, "nothing"),
		"the support folder's registry": filepath.Join(physical(supportDir), "sessions.json"),
	} {
		text, err := read(path, nil, nil)
		var refusal *acp.RPCError
		if !errors.As(err, &refusal) || !strings.HasPrefix(refusal.Message, "Smolcorp's sandbox doesn't let this desk read ") {
			t.Errorf("%s: read %q, %v", name, text, err)
		}
		if strings.Contains(text, "FAKEKEY") {
			t.Errorf("%s: the stand-in key was read", name)
		}
	}
	keyIntact(t, f)
}

func TestAnACPDesksFileWritesKeepToItsPolicyAndNeverWriteThroughAName(t *testing.T) {
	s := standInDesk(t)
	f := standInACPFiles(t, s)
	files := acpFiles{desk: s}
	ctx := context.Background()
	write := func(path, text string) error {
		return files.WriteTextFile(ctx, acp.WriteTextFileRequest{SessionID: "s1", Path: path, Content: text})
	}
	made := filepath.Join(s.places.Root, "made.txt")
	if err := write(made, "made"); err != nil {
		t.Fatalf("a new file in its folder: %v", err)
	}
	if err := os.Chmod(f.readme, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := write(f.readme, "changed"); err != nil {
		t.Errorf("its readme: %v", err)
	}
	if text, _ := os.ReadFile(f.readme); string(text) != "changed" {
		t.Errorf("its readme holds %q", text)
	}
	if info, err := os.Stat(f.readme); err != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("its readme's mode: %v %v", info, err)
	}
	// A second name for the key is replaced, never written through: the key stays as it was.
	if err := write(f.hard, "overwritten"); err != nil {
		t.Errorf("its second name for the key: %v", err)
	}
	keyIntact(t, f)
	if text, _ := os.ReadFile(f.hard); string(text) != "overwritten" {
		t.Errorf("the name in its folder holds %q", text)
	}
	if err := os.MkdirAll(filepath.Join(s.places.Root, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	guard := filepath.Join(physical(supportDir), hireGuardFile)
	if err := os.WriteFile(guard, []byte(`{"fence":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	up, _ := filepath.Rel(s.places.Root, physical(supportDir))
	for name, path := range map[string]string{
		"the stand-in key":                    f.key,
		"a new file in the support folder":    filepath.Join(physical(supportDir), "planted"),
		"a link to the key":                   f.link,
		"the guard file, which it may read":   guard,
		"its own .claude settings":            filepath.Join(s.places.Root, ".claude", "settings.json"),
		"its profile, which it may read":      filepath.Join(hireFolder(physical(supportDir), "ab12"), "sandbox.sb"),
		"a folder that isn't there":           filepath.Join(s.places.Root, "no", "such.txt"),
		"a file in the support folder via ..": s.places.Root + "/" + up + "/planted",
		"its folder itself":                   s.places.Root,
	} {
		err := write(path, "planted")
		var refusal *acp.RPCError
		if !errors.As(err, &refusal) || !strings.HasPrefix(refusal.Message, "Smolcorp's sandbox doesn't let this desk write ") {
			t.Errorf("%s: %v", name, err)
		}
	}
	keyIntact(t, f)
	if text, _ := os.ReadFile(guard); string(text) != `{"fence":1}` {
		t.Errorf("the guard file holds %q", text)
	}
	if _, err := os.Lstat(filepath.Join(physical(supportDir), "planted")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a file was planted in the support folder: %v", err)
	}
	if info, err := os.Lstat(f.link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link to the key was changed: %v %v", info, err)
	}
	if left, _ := filepath.Glob(filepath.Join(s.places.Root, ".*.smolcorp-*")); len(left) != 0 {
		t.Errorf("new files left behind: %q", left)
	}
}

func TestAnACPWriteRenamedNoLinkAndNoFolderInPlace(t *testing.T) {
	folder := physical(t.TempDir())
	target := filepath.Join(folder, "target")
	if err := os.WriteFile(target, []byte("as it was"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A folder above the file swapped for a link after the resolve: refused, nothing written where it leads.
	moved := filepath.Join(folder, "moved")
	if err := os.Mkdir(moved, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(folder, filepath.Join(moved, "in")); err != nil {
		t.Fatal(err)
	}
	if err := writeHireFile(filepath.Join(moved, "in", "target"), filepath.Join(moved, "in", ".t.new"), []byte("x")); err == nil {
		t.Error("a write through a link in its folders went ahead")
	}
	if err := os.Mkdir(filepath.Join(folder, "a-folder"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeHireFile(filepath.Join(folder, "a-folder"), filepath.Join(folder, ".a.new"), []byte("x")); err == nil {
		t.Error("a folder was written over")
	}
	if text, _ := os.ReadFile(target); string(text) != "as it was" {
		t.Errorf("the target holds %q", text)
	}
}

func TestACPLinesAndTerminalEnvironments(t *testing.T) {
	n := func(i int) *int { return &i }
	for _, c := range []struct {
		line, limit *int
		want        string
	}{{nil, nil, "a\nb\nc"}, {n(2), nil, "b\nc"}, {nil, n(1), "a\n"}, {n(3), n(5), "c"}, {n(9), n(1), ""}, {n(0), n(0), ""}} {
		if got := acpLines("a\nb\nc", c.line, c.limit); got != c.want {
			t.Errorf("acpLines(%v, %v) = %q, not %q", c.line, c.limit, got, c.want)
		}
	}
	env := acpTerminalEnv([]string{"PATH=/bin", "HOME=/h", "SMOLCORP_HIRE=1"}, []acp.EnvVariable{
		{Name: "PATH", Value: "/usr/bin"}, {Name: "LANG", Value: "C"}, {Name: "SMOLCORP_SANDBOX_NONCE", Value: "x"},
		{Name: "SMOLCORP_TMUX_SERVER", Value: "off"}, {Name: "DYLD_INSERT_LIBRARIES", Value: "/x.dylib"},
		{Name: "A=B", Value: "c"}, {Name: "", Value: "d"}, {Name: "NUL", Value: "a\x00b"},
	})
	if want := []string{"HOME=/h", "SMOLCORP_HIRE=1", "PATH=/usr/bin", "LANG=C"}; strings.Join(env, " ") != strings.Join(want, " ") {
		t.Errorf("the terminal's environment: %q, not %q", env, want)
	}
}

func TestACPTerminalOutputKeepsItsLastBytesWhole(t *testing.T) {
	o := &acpOutput{most: 5}
	o.Write([]byte("abc"))
	if text, cut := o.text(); text != "abc" || cut {
		t.Errorf("under the cap: %q %v", text, cut)
	}
	o.Write([]byte("dé")) // é is two bytes: "abcdé" is six
	if text, cut := o.text(); text != "cdé" && text != "bcdé" || !cut {
		t.Errorf("over the cap: %q %v", text, cut)
	}
	o.Write([]byte("éé"))
	if text, _ := o.text(); text != "éé" && text != "éé"[1:] {
		t.Errorf("cut inside a character: %q", text)
	}
	if text, _ := o.text(); !strings.HasPrefix(text, "é") {
		t.Errorf("the kept text starts inside a character: %q", text)
	}
}

// acpTestHandler is an ACP desk's stand-in, as ACP4b's desk will be: the sandbox answers the agent's files and
// terminals; what the agent says is kept; no ask is answered.
type acpTestHandler struct {
	*acpSandbox
	mu   sync.Mutex
	said string
}

func (h *acpTestHandler) Update(n acp.SessionNotification) {
	if n.Update.Content != nil {
		h.mu.Lock()
		h.said += n.Update.Content.Text
		h.mu.Unlock()
	}
}

func (h *acpTestHandler) RequestPermission(context.Context, acp.PermissionRequest) (acp.PermissionOutcome, error) {
	return acp.PermissionOutcome{}, errors.New("no")
}

// acpAnswer is one step's answer, as the stand-in agent said it.
type acpAnswer struct {
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

// startACPStandIn starts this test program as an ACP agent for desk s, through runner (s's own when nil), and opens
// its session.
func startACPStandIn(t *testing.T, sb *acpSandbox, runner acp.Runner) (*acp.Client, *acpTestHandler, error) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if runner == nil {
		runner = sb.Runner()
	}
	h := &acpTestHandler{acpSandbox: sb}
	c, err := acp.Start(acp.Command{Path: self, Args: []string{"acp-agent"}, Dir: sb.desk.places.Root,
		Env: append(hireEnvironment(), deskStandInVariable+"=1"), Runner: runner}, h, acp.Options{})
	if err != nil {
		return nil, h, err
	}
	t.Cleanup(func() { c.Close(); sb.close() })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.NewSession(ctx, sb.desk.places.Root); err != nil {
		t.Fatal(err)
	}
	return c, h, nil
}

// run has the stand-in agent take steps, and gives their answers.
func (h *acpTestHandler) run(t *testing.T, c *acp.Client, steps ...map[string]any) []acpAnswer {
	t.Helper()
	script, _ := json.Marshal(steps)
	h.mu.Lock()
	h.said = ""
	h.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := c.Prompt(ctx, "s1", []acp.ContentBlock{acp.Text(string(script))}); err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	var answers []acpAnswer
	if err := json.Unmarshal([]byte(h.said), &answers); err != nil || len(answers) != len(steps) {
		t.Fatalf("the agent said %q (%v)", h.said, err)
	}
	return answers
}

func step(method string, params map[string]any) map[string]any {
	return map[string]any{"method": method, "params": params}
}

func TestOnAMacAnACPAgentRunsInItsSandboxAndSoDoItsFilesAndTerminals(t *testing.T) {
	needSandboxExec(t)
	t.Setenv(deskStandInVariable, "1") // `enter`, run as this program, is the desk stand-in's
	s := standInDesk(t)
	f := standInACPFiles(t, s)
	self, _ := os.Executable()
	sb := newACPSandbox(s, self)
	c, h, err := startACPStandIn(t, sb, nil)
	if err != nil {
		t.Fatalf("the agent's start was refused: %v", err)
	}
	if left := leftCanaries(); len(left) != 0 {
		t.Errorf("its start's canary stayed: %q", left)
	}
	support := physical(supportDir)
	answers := h.run(t, c,
		// The agent itself, in its sandbox: its own folder reads, the support folder doesn't.
		step("self/read", map[string]any{"path": f.readme}),
		step("self/read", map[string]any{"path": f.key}),
		// Its files, through the core: by the desk's Policy and the reader.
		step("fs/read_text_file", map[string]any{"path": f.readme}),
		step("fs/read_text_file", map[string]any{"path": f.key}),
		step("fs/read_text_file", map[string]any{"path": f.link}),
		step("fs/read_text_file", map[string]any{"path": f.hard}),
		step("fs/read_text_file", map[string]any{"path": f.token}),
		step("fs/write_text_file", map[string]any{"path": filepath.Join(s.places.Root, "made.txt"), "content": "made"}),
		step("fs/write_text_file", map[string]any{"path": filepath.Join(support, "planted"), "content": "planted"}),
		step("fs/write_text_file", map[string]any{"path": f.hard, "content": "overwritten"}),
		// A terminal reading the key runs in the sandbox, and is refused it there.
		step("terminal/create", map[string]any{"command": "/bin/cat", "args": []string{f.key}}),
		step("terminal/wait_for_exit", map[string]any{"terminalId": "$last"}),
		step("terminal/output", map[string]any{"terminalId": "$last"}),
		step("terminal/release", map[string]any{"terminalId": "$last"}),
		// A terminal on the desk's own file, by name on the agent's PATH, in its own folder: it runs.
		step("terminal/create", map[string]any{"command": "cat", "args": []string{"readme.txt"}}),
		step("terminal/wait_for_exit", map[string]any{"terminalId": "$last"}),
		step("terminal/output", map[string]any{"terminalId": "$last"}),
		// A terminal asked to work in the support folder, or with the start's nonce: refused before it starts.
		step("terminal/create", map[string]any{"command": "/bin/ls", "cwd": support}),
	)
	ok := func(i int) string {
		t.Helper()
		if len(answers[i].Error) != 0 {
			t.Errorf("step %d was refused: %s", i, answers[i].Error)
		}
		return string(answers[i].Result)
	}
	refused := func(i int, words string) {
		t.Helper()
		if !strings.Contains(string(answers[i].Error), words) || strings.Contains(string(answers[i].Result), "FAKEKEY") {
			t.Errorf("step %d: %s %s", i, answers[i].Result, answers[i].Error)
		}
	}
	if got := ok(0); !strings.Contains(got, "hello") {
		t.Errorf("the agent's own readme: %s", got)
	}
	refused(1, "operation not permitted")
	if got := ok(2); !strings.Contains(got, `hello\nfrom`) {
		t.Errorf("its readme through the core: %s", got)
	}
	refused(3, "doesn't let this desk read")
	refused(4, "doesn't let this desk read")
	refused(5, "doesn't let this desk read")
	refused(6, "doesn't let this desk read")
	ok(7)
	refused(8, "doesn't let this desk write")
	ok(9)
	keyIntact(t, f)
	ok(10)
	if got := ok(11); !strings.Contains(got, `"exitCode":1`) {
		t.Errorf("cat on the key ended %s", got)
	}
	if got := ok(12); !strings.Contains(got, "Operation not permitted") || strings.Contains(got, "FAKEKEY") {
		t.Errorf("cat on the key said %s", got)
	}
	ok(13)
	ok(14)
	if got := ok(15); !strings.Contains(got, `"exitCode":0`) {
		t.Errorf("cat on its readme ended %s", got)
	}
	if got := ok(16); !strings.Contains(got, `hello\nfrom`) {
		t.Errorf("cat on its readme said %s", got)
	}
	refused(17, "doesn't let this desk work in")
	if left := leftCanaries(); len(left) != 0 {
		t.Errorf("its terminals' canaries stayed: %q", left)
	}
}

// standInRunner is a start of the agent through its sandbox's runner that notes the pid it was asked to check.
type standInRunner struct {
	acp.Runner
	started *int
}

func (r standInRunner) Started(pid int) error {
	*r.started = pid
	return r.Runner.Started(pid)
}

func TestOnAMacAnACPAgentThatIsntInItsSandboxIsNeverStarted(t *testing.T) {
	needSandboxExec(t)
	t.Setenv(deskStandInVariable, "1")
	s := standInDesk(t)
	self, _ := os.Executable()

	// Its handshake doesn't come (Smolcorp's program here is one that runs no `enter`): refused, nothing left.
	sb := newACPSandbox(s, "/usr/bin/true")
	var pid int
	if _, _, err := startACPStandIn(t, sb, standInRunner{sb.Runner(), &pid}); err == nil || !strings.Contains(err.Error(), sandbox.Refusal("darwin")) {
		t.Errorf("no handshake: %v", err)
	}
	gone := func(what string) {
		t.Helper()
		if pid <= 0 || syscall.Kill(pid, 0) == nil {
			t.Errorf("%s: its process %d is still there", what, pid)
		}
		if left := leftCanaries(); len(left) != 0 {
			t.Errorf("%s: its canary stayed: %q", what, left)
		}
	}
	gone("no handshake")

	// Its handshake passes, and macOS says the process may read the canary (here a stand-in says so): refused too.
	saved := acpChildIsOurs
	acpChildIsOurs = func(int, sandbox.Canary) error { return &sandbox.Refused{Why: "a stand-in's"} }
	t.Cleanup(func() { acpChildIsOurs = saved })
	sb = newACPSandbox(s, self)
	pid = 0
	if _, _, err := startACPStandIn(t, sb, standInRunner{sb.Runner(), &pid}); err == nil || !strings.Contains(err.Error(), sandbox.Refusal("darwin")) {
		t.Errorf("a process that may read the canary: %v", err)
	}
	gone("not in its sandbox")
	// No terminal either: its agent never started, so it has no environment to give one.
	if _, err := sb.CreateTerminal(context.Background(), acp.CreateTerminalRequest{SessionID: "s1", Command: "/bin/echo"}); err == nil {
		t.Error("a terminal was made for an agent that never started")
	}
	acpChildIsOurs = saved

	// sandbox-exec not Apple's alone: refused before anything runs.
	sb = newACPSandbox(s, "relative/smolcorp")
	pid = 0
	if _, _, err := startACPStandIn(t, sb, standInRunner{sb.Runner(), &pid}); err == nil || !strings.Contains(err.Error(), sandbox.SandboxExecRefusal) || pid != 0 {
		t.Errorf("a start that can't be made: %v (pid %d)", err, pid)
	}
}

func TestOnAMacAnACPDesksTerminalsAreItsOwnAndEndWithIt(t *testing.T) {
	needSandboxExec(t)
	t.Setenv(deskStandInVariable, "1")
	s := standInDesk(t)
	self, _ := os.Executable()
	sb := newACPSandbox(s, self)
	if _, _, err := startACPStandIn(t, sb, nil); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	made, err := sb.CreateTerminal(ctx, acp.CreateTerminalRequest{SessionID: "s1", Command: "/bin/sleep", Args: []string{"60"}})
	if err != nil {
		t.Fatal(err)
	}
	// Another session's ask doesn't reach it.
	if _, err := sb.TerminalOutput(ctx, acp.TerminalRequest{SessionID: "s2", TerminalID: made.TerminalID}); err == nil {
		t.Error("another session read the terminal")
	}
	if err := sb.KillTerminal(ctx, acp.TerminalRequest{SessionID: "s2", TerminalID: made.TerminalID}); err == nil {
		t.Error("another session killed the terminal")
	}
	sb.mu.Lock()
	term := sb.terms[made.TerminalID]
	sb.mu.Unlock()
	// Killed: it ends by its signal, and stays until released.
	if err := sb.KillTerminal(ctx, acp.TerminalRequest{SessionID: "s1", TerminalID: made.TerminalID}); err != nil {
		t.Fatal(err)
	}
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	status, err := sb.WaitForTerminalExit(wctx, acp.TerminalRequest{SessionID: "s1", TerminalID: made.TerminalID})
	if err != nil || status.Signal == nil || *status.Signal != "SIGKILL" {
		t.Errorf("killed: %+v %v", status, err)
	}
	// Up to its cap of terminals; then the desk ends and every one ends with it.
	var pids []int
	for range acpTerminalsMost - 1 {
		made, err := sb.CreateTerminal(ctx, acp.CreateTerminalRequest{SessionID: "s1", Command: "/bin/sleep", Args: []string{"60"}})
		if err != nil {
			t.Fatal(err)
		}
		sb.mu.Lock()
		pids = append(pids, sb.terms[made.TerminalID].cmd.Process.Pid)
		sb.mu.Unlock()
	}
	if _, err := sb.CreateTerminal(ctx, acp.CreateTerminalRequest{SessionID: "s1", Command: "/bin/sleep", Args: []string{"1"}}); err == nil {
		t.Errorf("a terminal past the cap of %d", acpTerminalsMost)
	}
	sb.close()
	for _, pid := range append(pids, term.cmd.Process.Pid) {
		if syscall.Kill(pid, 0) == nil {
			t.Errorf("terminal %d outlived its desk", pid)
		}
	}
	if _, err := sb.CreateTerminal(ctx, acp.CreateTerminalRequest{SessionID: "s1", Command: "/bin/echo"}); err == nil {
		t.Error("a terminal made after the desk ended")
	}
}

// FXPOLICYFOLD: the core reads and writes an ACP desk's files as the owner, outside its sandbox, by what the Policy
// says of the path as the agent spelled it. The Mac's volume takes a name in Unicode's full case folding, so the
// support folder spelled with ſ (the long s) for its s is the support folder, and a plant spelled with ſ or ﬁ is the
// plant: each is refused as its plain spelling is, and nothing there changes.
func TestAnACPDesksFilesKeepToItsPolicyInEverySpellingTheMacTakes(t *testing.T) {
	s := standInDesk(t)
	f := standInACPFiles(t, s)
	files := acpFiles{desk: s}
	ctx := context.Background()
	support := physical(supportDir)
	name := filepath.Base(support)
	if !strings.Contains(name, "s") {
		t.Fatalf("the stand-in support folder %q has no s to spell another way", name)
	}
	folded := filepath.Join(filepath.Dir(support), strings.Replace(name, "s", "ſ", 1))
	key, token := filepath.Join(folded, "stand-in-key"), filepath.Join(folded, "stand-in-token")
	for _, at := range []string{key, token} {
		want, err1 := os.Stat(filepath.Join(support, filepath.Base(at)))
		got, err2 := os.Stat(at)
		if err1 != nil || err2 != nil || !os.SameFile(want, got) {
			t.Skipf("this volume doesn't take %q for the support folder (%v, %v)", folded, err1, err2)
		}
	}
	for _, at := range []string{key, token, filepath.Join(support, "stand-in-Key")} {
		resp, err := files.ReadTextFile(ctx, acp.ReadTextFileRequest{SessionID: "s1", Path: at})
		var refusal *acp.RPCError
		if !errors.As(err, &refusal) || strings.Contains(resp.Content, "FAKEKEY") {
			t.Errorf("read %q: the stand-in was read (%v)", strings.TrimPrefix(at, filepath.Dir(support)), err)
		}
	}
	home := physical(homeDir)
	for _, at := range []string{key, filepath.Join(home, ".zſhrc"), filepath.Join(home, ".proﬁle")} {
		err := files.WriteTextFile(ctx, acp.WriteTextFileRequest{SessionID: "s1", Path: at, Content: "overwritten"})
		var refusal *acp.RPCError
		if !errors.As(err, &refusal) {
			t.Errorf("write %q: written (%v)", strings.TrimPrefix(at, filepath.Dir(support)), err)
		}
	}
	keyIntact(t, f)
	for _, plant := range []string{".zshrc", ".profile"} {
		if _, err := os.Lstat(filepath.Join(home, plant)); err == nil {
			t.Errorf("the stand-in home got a %s", plant)
		}
	}
}
