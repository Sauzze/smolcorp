//go:build unix

package tmuxfake

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// helperMain, when set (peerpid_darwin_test.go), runs instead of the tests in a copy of this test program started
// with TMUXFAKE_HELPER: the spike's stand-in servers.
var helperMain func(mode string)

func TestMain(m *testing.M) {
	if mode := os.Getenv("TMUXFAKE_HELPER"); mode != "" && helperMain != nil {
		helperMain(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// A tmux client the rows run: the guarded build (third_party/tmux/guard.patch), or a stock one.
type tmuxClient struct {
	name string
	path string
	// off: the guarded build with SMOLCORP_TMUX_SERVER=off, which is stock tmux (the patch's every change is behind
	// guard_on), used as the stock client when no other is given.
	off bool
	// missing says why there is no such client here; the row is then skipped, visibly, or failed under
	// SMOLCORP_SANDBOX_REQUIRED=1 (the Mac runner).
	missing string
}

func (c tmuxClient) guarded() bool { return c.name == "guarded" }

// need skips (or, on the runner, fails) a row this client can't run here.
func (c tmuxClient) need(t *testing.T) {
	t.Helper()
	if c.missing == "" {
		return
	}
	if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
		t.Fatalf("SMOLCORP_SANDBOX_REQUIRED=1 and no %s tmux: %s", c.name, c.missing)
	}
	t.Skipf("SKIPPED, no %s tmux here: %s", c.name, c.missing)
}

// guardedBuild finds the guarded tmux: SMOLCORP_TEST_TMUX, or the newest scripts/build-tmux.sh build in this
// checkout's build/tmux. Off a Mac there is none until a Linux build exists (SA-L5).
func guardedBuild() (string, string) {
	if path := os.Getenv("SMOLCORP_TEST_TMUX"); path != "" {
		if _, err := os.Stat(path); err != nil {
			return "", "SMOLCORP_TEST_TMUX: " + err.Error()
		}
		return path, ""
	}
	found, _ := filepath.Glob("../../../build/tmux/*/tmux")
	sort.Slice(found, func(i, j int) bool { return modTime(found[i]).After(modTime(found[j])) })
	if len(found) == 0 {
		return "", "set SMOLCORP_TEST_TMUX to a guarded tmux, or run scripts/build-tmux.sh (Mac)"
	}
	path, _ := filepath.Abs(found[0])
	return path, ""
}

func modTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// clients is the stock client, then the guarded one. Stock is SMOLCORP_TEST_TMUX_STOCK, else the guarded build with
// the guard off, else the system's tmux (CI's Linux runners have one).
func clients() []tmuxClient {
	guardedPath, missing := guardedBuild()
	guarded := tmuxClient{name: "guarded", path: guardedPath, missing: missing}
	stock := tmuxClient{name: "stock"}
	switch path, err := exec.LookPath("tmux"); {
	case os.Getenv("SMOLCORP_TEST_TMUX_STOCK") != "":
		stock.path = os.Getenv("SMOLCORP_TEST_TMUX_STOCK")
	case guardedPath != "":
		stock.path, stock.off = guardedPath, true
	case err == nil:
		stock.path = path
	default:
		stock.missing = "no tmux on PATH and no guarded build"
	}
	return []tmuxClient{stock, guarded}
}

type result struct {
	status         int
	stdout, stderr string
}

func (r result) refused() bool {
	return r.status == 97 && strings.Contains(r.stderr, "smolcorp guard: refused")
}

// run runs the client on socket with a small, fixed environment plus extra, stdin as given (or /dev/null), and
// gives up after ten seconds. guard is SMOLCORP_TMUX_SERVER for a guarded client; a stock client ignores it. The
// client's parent is a shell, not the test: a client that obeys MSG_DETACHKILL (stock, or a broken guard) sends its
// parent SIGHUP.
func (c tmuxClient) run(t *testing.T, socket, guard, stdin string, extra []string, args ...string) result {
	t.Helper()
	env := []string{"PATH=/usr/bin:/bin", "HOME=" + filepath.Dir(socket), "LANG=C", "TERM=xterm"}
	switch {
	case c.off:
		env = append(env, "SMOLCORP_TMUX_SERVER=off")
	case c.guarded() && guard != "":
		env = append(env, "SMOLCORP_TMUX_SERVER="+guard)
	}
	return runProgram(t, append(env, extra...), stdin, "/bin/sh",
		append([]string{"-c", `"$0" "$@"; exit $?`, c.path, "-f", "/dev/null", "-S", socket}, args...)...)
}

func runProgram(t *testing.T, env []string, stdin, program string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = env
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		t.Fatalf("%s %v didn't end within ten seconds", program, args)
	case errors.As(err, &exitErr):
		r.status = exitErr.ExitCode()
	case err != nil:
		t.Fatalf("%s %v: %v", program, args, err)
	}
	return r
}

// shortDir is a folder of the test's own with a short path: a unix socket's path must fit in 104 bytes.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tf")
	if err != nil {
		t.Fatal(err)
	}
	dir, _ = filepath.EvalSymlinks(dir)
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func listen(t *testing.T, dir string, steps ...Step) *Server {
	t.Helper()
	srv, err := Listen(filepath.Join(dir, "s"), steps...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "(" + err.Error() + ")"
	}
	return string(data)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// The fake server runs in this process, so a guarded client checking it is given this pid.
var ownPid = strconv.Itoa(os.Getpid())

// session is the one connection's record, after the client has ended.
func session(t *testing.T, srv *Server) Session {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		sessions := srv.Sessions()
		if len(sessions) == 1 {
			return sessions[0]
		}
		if len(sessions) > 1 || time.Now().After(deadline) {
			t.Fatalf("%d connections, not one", len(sessions))
		}
		time.Sleep(10 * time.Millisecond)
	}
}
