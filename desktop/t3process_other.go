//go:build !linux && !darwin && !windows

package desktop

// processesWorkingIn can't look on this system: Let go keeps the fence and says why.
func processesWorkingIn(string) ([]int, bool) { return nil, false }

func processName(int) string { return "" }
