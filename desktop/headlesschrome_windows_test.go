//go:build windows

package desktop

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// browserGroup is the browser's job object (ownGroup and joinGroup, summary_process_windows.go): started suspended and
// put in the job before it runs, so nothing it starts is outside it. Only this test program holds the job, made to kill
// all of it once it's closed, so the system kills the browser when the program ends, killed or timed out.
type browserGroup struct {
	mu      *sync.Mutex
	group   *processGroup
	browser *headlessBrowser
}

// kill ends the job; with no job (one couldn't be made), the browser's tree by its pid, only while it isn't reaped.
func (g browserGroup) kill() {
	if g.mu == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.group.job == 0 {
		select {
		case <-g.browser.ended:
			return
		default:
		}
	}
	g.group.kill()
}

// release closes the job, which kills whatever is left in it.
func (g browserGroup) release() {
	if g.mu == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.group.close()
	g.group.job = 0
}

// start starts the browser in its job. Past ctx's deadline, the whole job is killed, not only the browser.
func (b *headlessBrowser) start(ctx context.Context, program string, args, env []string) error {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = &b.output, &b.output
	ownGroup(cmd)
	g := browserGroup{mu: &sync.Mutex{}, group: &processGroup{}, browser: b}
	cmd.Cancel = func() error { g.kill(); return nil }
	cmd.WaitDelay = 5 * time.Second // what a helper of the browser's printed mustn't hold Wait
	if err := cmd.Start(); err != nil {
		return err
	}
	g.mu.Lock()
	*g.group = joinGroup(cmd)
	g.mu.Unlock()
	b.cmd, b.group = cmd, g
	go func() {
		err := cmd.Wait()
		b.end(fmt.Sprint("ended: ", err))
		b.waited <- err
	}()
	return nil
}

// browserLeftovers: on Windows the job object holds all the browser started, and closing it ends them all.
func browserLeftovers(string, time.Duration) string { return "" }

// killProfile: on Windows nothing the browser starts leaves its job (the job allows no breakaway), and closing the job
// ends them all.
func killProfile(string) {}
