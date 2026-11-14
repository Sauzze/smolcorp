// Package disclaim starts programs on a Mac so that they hold none of Smolcorp's permissions (SANDBOX-PLAN §3.5,
// ST1).
//
// macOS charges a program to the app that started it, and Smolcorp.app holds Accessibility, the microphone and
// screen access. A hire can replace any program Smolcorp runs (git, tmux, an agent's CLI, a terminal's tool, or
// whatever a repo's config names for git to run), and that program's next run would read Setup's pairing link and
// click Allow as Smolcorp. So every program the core starts goes through Cmd, which has bin/smolcorp-spawn
// (packaging/macos/smolcorp-spawn.c) start it with responsibility disclaimed: macOS charges it to itself.
//
// A starter that isn't an exec.Cmd (a pseudo-terminal's, go-pty's Command) is handed what Words gives instead.
//
// The only programs the core starts without it are Apple's own, which run no code they're pointed at: /bin/ps,
// /usr/bin/open and /usr/bin/defaults (the app's osascript runs from the app, with its own fixed scripts), and
// Smolcorp itself. desktop's TestEveryProgramStartsDisclaimedOnAMac holds that list, and fails on any other start.
package disclaim

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

// SpawnName is smolcorp-spawn's file name, beside bin/smolcorp in the app.
const SpawnName = "smolcorp-spawn"

// Place is where smolcorp-spawn should be, and whether this program runs from an app, which must then have it.
type Place struct {
	Spawn string // the path smolcorp-spawn should have
	InApp bool   // whether this program is Smolcorp.app's Contents/Resources/bin/smolcorp
	Err   error  // where this program is couldn't be told: treated as in an app
}

// builtForApp is set when the core is linked for Smolcorp.app (scripts/build-app.sh, -X): that core counts as in an
// app wherever it runs from, so it never runs a program charged to Smolcorp, whatever its path says. Empty in go
// test, go run and a build of the command on its own. scripts/release.sh stops on a core without it.
var builtForApp string

// place is smolcorp-spawn's place for this program, worked out once.
var place = sync.OnceValue(func() Place {
	program, err := os.Executable()
	if err != nil {
		return Place{InApp: true, Err: err}
	}
	resolved, err := filepath.EvalSymlinks(program)
	if err != nil {
		resolved = program
	}
	return placeOf(program, resolved, builtForApp != "")
})

// placeOf is smolcorp-spawn's place for a program started as started, which is resolved once its links are
// followed: beside the resolved one. It is in an app if it was built for one, or if either path is in one (the path
// is the second guard, for a core built for the app some other way).
func placeOf(started, resolved string, builtForApp bool) Place {
	where := PlaceBeside(resolved)
	where.InApp = where.InApp || PlaceBeside(started).InApp || builtForApp
	return where
}

// PlaceBeside is smolcorp-spawn's place for program: beside it, in its bin folder.
func PlaceBeside(program string) Place {
	bin := filepath.Dir(program)
	resources := filepath.Dir(bin)
	contents := filepath.Dir(resources)
	inApp := filepath.Base(bin) == "bin" && filepath.Base(resources) == "Resources" &&
		filepath.Base(contents) == "Contents" && strings.HasSuffix(filepath.Dir(contents), ".app")
	return Place{Spawn: filepath.Join(bin, SpawnName), InApp: inApp}
}

// isMac is whether there are grants to disclaim.
const isMac = runtime.GOOS == "darwin"

// standing is a test's stand-in for isMac and place (StandIn), nil for the real ones. Atomic, so goroutines a test
// left behind read it safely.
var standing atomic.Pointer[stoodIn]

type stoodIn struct {
	mac   bool
	where Place
}

// onAMac is whether there are grants to disclaim, a test's stand-in included.
func onAMac() bool {
	if s := standing.Load(); s != nil {
		return s.mac
	}
	return isMac
}

// placeNow is smolcorp-spawn's place, a test's stand-in included.
func placeNow() Place {
	if s := standing.Load(); s != nil {
		return s.where
	}
	return place()
}

// Cmd has cmd start through smolcorp-spawn on a Mac, so the program it runs holds none of Smolcorp's permissions,
// and gives cmd back. It is meant around exec.Command or exec.CommandContext, before anything else is set:
//
//	cmd := disclaim.Cmd(exec.CommandContext(ctx, "git", args...))
//
// What's set on cmd afterwards (folder, environment, files, process group, Cancel, WaitDelay) works as before,
// since smolcorp-spawn becomes the program in place: the pid Smolcorp sees, signals and kills, is the program's.
//
// Off a Mac it changes nothing. On a Mac:
//   - in Smolcorp.app, smolcorp-spawn must be in the app's bin folder; if it isn't, cmd won't start (Start gives
//     the refusal) and nothing runs. A release never runs a program charged to Smolcorp.
//   - outside an app (go test, go run, a build of the command on its own), with no smolcorp-spawn beside it, the
//     program runs as before: there is no Smolcorp.app to be charged to. With one beside it, that one is used.
//
// A command that already can't start (cmd.Err, e.g. a program not found on PATH) is given back as it is. A program
// by its full path that isn't there, or is a folder or has no execute bit, fails at Start as it would have on its
// own (fork/exec …). One that is there and still can't run (a file that isn't a program, an execute bit that isn't
// this user's) starts smolcorp-spawn, which ends with exit status 126 and says why on the program's stderr: a caller
// that keeps only Wait's error sees "exit status 126".
func Cmd(cmd *exec.Cmd) *exec.Cmd {
	if !onAMac() || cmd.Err != nil {
		return cmd
	}
	spawn, err := spawnFor(cmd.Path)
	if err != nil || spawn == "" {
		cmd.Err = err
		return cmd
	}
	words := cmd.Args
	if len(words) == 0 {
		words = []string{cmd.Path}
	}
	cmd.Args = append([]string{spawn, cmd.Path}, words...)
	cmd.Path = spawn
	return cmd
}

// Words is Cmd for a starter that isn't an exec.Cmd, such as a pseudo-terminal's: the program and arguments to hand
// it in place of program and args, or why nothing may start. The starter must run the program it's given as an
// exec.Command does (go-pty's does on a Mac), so smolcorp-spawn becomes the program in place, in the starter's
// session and terminal:
//
//	program, args, err := disclaim.Words(program, args)
//	cmd := terminal.Command(program, args...)
//
// On a Mac it gives smolcorp-spawn, then program, program and args; off a Mac, or outside an app with no
// smolcorp-spawn, program and args as they are. A program with no folder in its name is looked for on PATH first,
// as exec.Command looks for it. Errors when a start fails are as for Cmd.
func Words(program string, args []string) (string, []string, error) {
	if !onAMac() {
		return program, args, nil
	}
	if !strings.Contains(program, "/") {
		found, err := exec.LookPath(program)
		if err != nil {
			return "", nil, err
		}
		program = found
	}
	spawn, err := spawnFor(program)
	if err != nil {
		return "", nil, err
	}
	if spawn == "" {
		return program, args, nil
	}
	return spawn, append([]string{program, program}, args...), nil
}

// spawnFor is the smolcorp-spawn to start program through on a Mac, "" to start it as it is (outside an app, with
// none beside the core), or why it mustn't start.
func spawnFor(program string) (string, error) {
	where := placeNow()
	if where.Err != nil {
		return "", refusal(program, fmt.Sprintf("where Smolcorp runs from couldn't be told (%v)", where.Err))
	}
	info, err := os.Stat(where.Spawn)
	usable := err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
	switch {
	case !usable && where.InApp:
		return "", refusal(program, "its "+SpawnName+" is missing")
	case !usable:
		return "", nil // outside an app: nothing of Smolcorp.app's to hold
	}
	// A program by its full path that isn't there, or can't be run, fails here as it would have on its own.
	if filepath.IsAbs(program) {
		info, err := os.Stat(program)
		if err != nil {
			var pathErr *fs.PathError
			if errors.As(err, &pathErr) {
				err = pathErr.Err
			}
			return "", &fs.PathError{Op: "fork/exec", Path: program, Err: err}
		}
		if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			return "", &fs.PathError{Op: "fork/exec", Path: program, Err: syscall.EACCES}
		}
	}
	return where.Spawn, nil
}

// StandIn has Cmd and Words see a Mac (or not) and smolcorp-spawn's place as given, until the function it gives back
// is called. For tests only: desktop's TestEveryProgramStartsDisclaimedOnAMac fails on a call from the core's code.
func StandIn(mac bool, where Place) (restore func()) {
	old := standing.Swap(&stoodIn{mac: mac, where: where})
	return func() { standing.Store(old) }
}

// refusal is the error a program not started gets: Smolcorp's words.
func refusal(program, why string) error {
	return fmt.Errorf("Smolcorp didn't start %s: %s, so it would have held Smolcorp's permissions. Reinstall Smolcorp",
		filepath.Base(program), why)
}
