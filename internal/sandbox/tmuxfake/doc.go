// Package tmuxfake is a tmux server that speaks just enough of tmux's client protocol (protocol 8, the one tmux
// 3.8 and the bundled tmux use) to answer a real tmux client the way a hire's own server could: ask it to read or
// write a file by name, run a program, lock the screen, kill its parent. A stock client does each of these outside
// the hire's sandbox (SANDBOX-PLAN §A2, T1-T6); the guarded client (third_party/tmux/guard.patch) must refuse
// them. It runs inside the test's process, so its pid is the test's.
//
// It starts nothing and reads no file: what a client sends is only recorded, for the test to look at. Windows has
// no tmux, so there the package is empty.
package tmuxfake
