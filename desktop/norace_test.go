//go:build !race

package desktop

// raceSlowdown is 1 without the race detector: see race_test.go.
const raceSlowdown = 1
