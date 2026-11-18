package desktop

import "os"

// canWriteIn says whether this user may make files in folder. Windows has no
// access() worth asking, and a folder's read-only attribute only means it was
// customised (Documents has it), so while the folder is only looked at (Start
// here's suggestion, a path as it's typed) it counts as writable. Only when
// it's being kept is a file really made there and taken away at once, so the
// user's folders aren't written in on every look.
func canWriteIn(folder string, keeping bool) bool {
	if !keeping {
		return true
	}
	file, err := os.CreateTemp(folder, ".smolcorp-write-check-*")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true
}
