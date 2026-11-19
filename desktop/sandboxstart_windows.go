package desktop

import "io/fs"

// ownedByAnother is never sure on Windows: a folder the link check can't list refuses the start.
func ownedByAnother(fs.FileInfo) bool { return false }
