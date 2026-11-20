// Package procmem keeps a Smolcorp process's memory from the user's other programs, a hire among them, where the
// system lets a process do that for itself. It's called first thing: by the app and `smolcorp serve` as their own
// first step, which fails closed (desktop/memory.go), and by cmd/smolcorp for the guard:
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
//     (procmem_windows_test.go tries it, and says what it found). The tray app sets it as `smolcorp serve` does, by
//     the same step (desktop/memory_windows_test.go, which also runs a WebView2 window like the office's under it).
//   - Neither covers a copy another program started. The list lets any program of yours stop Smolcorp, and the
//     program that then starts it again holds the handle CreateProcess gave it, with every right: a list set later
//     doesn't take that back. On Linux, a copy started under a tracer stays traced after PR_SET_DUMPABLE 0. Any
//     wider claim in the user docs keeps this limit: "A program that stops Smolcorp and starts it again itself can
//     read the copy it started."
//   - Mac: nothing here. The hardened runtime in each signature does it (scripts/check-runtime.sh).
package procmem
