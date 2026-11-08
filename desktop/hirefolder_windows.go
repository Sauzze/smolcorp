package desktop

import "os"

// canWriteIn says whether this user may make files in folder: Windows has no
// access() worth asking, so a file is made there and taken away at once.
func canWriteIn(folder string) bool {
	file, err := os.CreateTemp(folder, ".smolcorp-write-check-*")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true
}
