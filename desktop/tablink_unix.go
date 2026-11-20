//go:build !windows

package desktop

// inDeskSocketFolder is whether place, a path with no links, is inside the socket folder every sandboxed desk's own
// folder is in (deskSocketParent), however spelt.
func inDeskSocketFolder(place string) bool { return insideOnDisk(place, deskSocketParent()) }
