//go:build !windows

package gm

import "syscall"

// execGM puts the GM's command in this process's place, so nothing of this one stays, its copy of the token included.
func execGM(program string, args, environment []string) error {
	return syscall.Exec(program, args, environment)
}
