//go:build race

package desktop

// raceSlowdown is how many times longer a time limit on heavy work is under the race detector,
// which makes such work up to 20 times slower (Go's documentation says 2 to 20, and redacting a
// long line here is about 20 times slower). Stretched so, a limit still catches work gone
// quadratic, which takes minutes.
const raceSlowdown = 20
