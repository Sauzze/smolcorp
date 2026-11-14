// A stand-in program for disclaim's tests: it says who it is and what it was given, then does what its first word
// asks. It touches nothing but the file it's told to make.
#include <dlfcn.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

typedef pid_t (*responsible_fn)(pid_t);

int main(int argc, char **argv) {
  responsible_fn responsible = (responsible_fn)dlsym(RTLD_DEFAULT, "responsibility_get_pid_responsible_for_pid");
  printf("pid=%d responsible=%d\n", getpid(), responsible ? responsible(getpid()) : -1);
  for (int i = 0; i < argc; i++) printf("arg[%d]=[%s]\n", i, argv[i]);
  fflush(stdout);
  const char *mode = argc > 1 ? argv[1] : "";
  if (strcmp(mode, "exit") == 0 && argc > 2) return atoi(argv[2]);
  if (strcmp(mode, "signal") == 0) { raise(SIGTERM); return 99; }
  if (strcmp(mode, "touch") == 0 && argc > 2) { FILE *f = fopen(argv[2], "w"); if (f) fclose(f); return 0; }
  if (strcmp(mode, "wait") == 0) { for (;;) pause(); }
  return 0;
}
