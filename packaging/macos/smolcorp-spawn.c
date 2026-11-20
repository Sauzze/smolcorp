// smolcorp-spawn: starts a program on its own account, so it holds none of Smolcorp's permissions
// (SANDBOX-PLAN §3.5, ST1).
//
//   smolcorp-spawn PROGRAM ARGV0 [ARG...]
//
// macOS charges a program to the app that started it: whatever Smolcorp.app starts could use its Accessibility,
// its microphone and its screen access. Every program the core and the app start goes through this one, which
// becomes PROGRAM with "responsibility disclaimed": macOS then charges PROGRAM to itself, and asks for itself.
//
// It becomes PROGRAM in place (POSIX_SPAWN_SETEXEC), as exec does, so nothing stands between Smolcorp and it: the
// same process, the same pid, the same files and folder, so every signal reaches PROGRAM itself and Smolcorp sees
// PROGRAM's own exit status, a signal death included. ARGV0 and the ARGs are PROGRAM's words as they are, with no
// shell; the environment passes as it is.
//
// The call that disclaims is private to macOS, so it's looked up as this runs. If it isn't there, or refuses,
// PROGRAM isn't started, and this says why: never started charged to Smolcorp.
//
// Built into Smolcorp.app as Contents/Resources/bin/smolcorp-spawn by scripts/build-app.sh, signed with the
// hardened runtime and sealed with the app. desktop's tests build it on their own with cc.

#include <dlfcn.h>
#include <errno.h>
#include <spawn.h>
#include <stdio.h>
#include <string.h>

extern char **environ;

typedef int (*setdisclaim_fn)(posix_spawnattr_t *attrs, int disclaim);

// The call that disclaims. Only a test build names another, one that isn't there, to see the refusal.
#ifndef DISCLAIM_CALL
#define DISCLAIM_CALL "responsibility_spawnattrs_setdisclaim"
#endif

int main(int argc, char **argv) {
  if (argc < 3) {
    fprintf(stderr, "Usage: smolcorp-spawn PROGRAM ARGV0 [ARG...]\n");
    return 2;
  }
  const char *program = argv[1];
  setdisclaim_fn setdisclaim = (setdisclaim_fn)dlsym(RTLD_DEFAULT, DISCLAIM_CALL);
  if (setdisclaim == NULL) {
    fprintf(stderr, "smolcorp-spawn: this Mac can't start a program on its own account (it has no "
                    DISCLAIM_CALL "), so %s wasn't started: it would have held Smolcorp's permissions.\n",
            program);
    return 126;
  }
  posix_spawnattr_t attrs;
  int err = posix_spawnattr_init(&attrs);
  if (err == 0) err = posix_spawnattr_setflags(&attrs, POSIX_SPAWN_SETEXEC);
  if (err == 0) err = setdisclaim(&attrs, 1);
  if (err != 0) {
    fprintf(stderr, "smolcorp-spawn: macOS wouldn't start %s on its own account (%s), so it wasn't started: it would "
                    "have held Smolcorp's permissions.\n", program, strerror(err));
    return 126;
  }
  // Returns only if PROGRAM couldn't take this one's place.
  err = posix_spawn(NULL, program, NULL, &attrs, &argv[2], environ);
  fprintf(stderr, "smolcorp-spawn: couldn't start %s: %s\n", program, strerror(err));
  return err == ENOENT ? 127 : 126;
}
