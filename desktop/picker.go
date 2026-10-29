package desktop

// The folder new hires start in, chosen with the system's own picker from
// the setup page, which a web page can't open: as the Mac app's
// chooseHireFolder.

const pickerTitle = "New hires from the office start their work in this folder."

// hasFolderPicker and pickFolder are this system's picker: variables, for
// the tests. pickFolder waits for a choice, and says false when there was none.
var (
	hasFolderPicker = func() bool { return canOpenWindows() && folderPicker("", pickerTitle) != nil }
	pickFolder      = func(start string) (string, bool, error) {
		cmd := folderPicker(start, pickerTitle)
		if cmd == nil {
			return "", false, errUnsupported
		}
		return pickedFolder(cmd.Output())
	}
)

// chooseHireFolder asks for the new hires' folder with the system's picker,
// and keeps the one chosen. Asked again while the picker is open, it answers
// at once rather than opening another.
func (d *Deck) chooseHireFolder() error {
	d.mu.Lock()
	if d.picking {
		d.mu.Unlock()
		return nil
	}
	d.picking = true
	start, _ := existingFolder(d.config.HireFolder)
	if start == "" {
		start = homeDir
	}
	d.mu.Unlock()
	folder, chosen, err := pickFolder(start)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.picking = false
	if checked, ok := existingFolder(folder); chosen && ok {
		d.config.HireFolder = checked
		d.config.save()
		d.changed()
	}
	return err
}
