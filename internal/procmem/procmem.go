// Package procmem keeps a Smolcorp process's memory from the user's other programs, a hire among them, where the
// system lets a process do that for itself. cmd/smolcorp calls it first thing:
//
//   - Linux, NotDumpable: the app, `smolcorp serve` and the guard are made non-dumpable (PR_SET_DUMPABLE 0). Another
//     process of the same user can then neither attach a debugger nor read its /proc/<pid>/mem, environ, maps or fd,
//     and it leaves no core file. It lasts until the process runs another program, which starts dumpable as usual:
//     the app's own restart is a new copy that protects itself first thing, and what they start (desks among them)
//     isn't covered. On Linux the guard also
//     ignores an LD_PRELOAD only because the build is static (scripts/release-desktop.sh).
//   - Windows, DenyMemoryAccess: the process's access list lets the user's other processes wait for it, stop it
//     and ask whether it runs, and nothing else: no reading or writing its memory, no thread started in it, no
//     handle copied out of it. An administrator opens it regardless, by switching on SeDebugPrivilege
//     (procmem_windows_test.go tries it, and says what it found). Only `smolcorp serve` sets it for now: the app's
//     office window is WebView2's, which hasn't been run under this list yet.
//   - Mac: nothing here. The hardened runtime in each signature does it (scripts/check-runtime.sh).
package procmem
