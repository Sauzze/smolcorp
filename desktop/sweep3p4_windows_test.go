package desktop

// sweepStartHolds is the saved folder for hires, for row A (sweep3p4_test.go): Windows has no sandbox to hold it yet.
func sweepStartHolds() string {
	folder, _ := existingFolder(savedConfig().HireFolder)
	return folder
}
