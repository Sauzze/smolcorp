//go:build !windows

package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// A night shift's file another program changed, from a version where night jobs ran: while they're off, it's still
// left out and the records still rotate, so no copy of it fits again, but Setup names nothing. Not on Windows, as nightstart_test.go's sealed night shift.
func TestOnAMacADormantFeaturesBrokenRecordIsDroppedUnsaid(t *testing.T) {
	turnOn(t, &nightShiftOn)
	_, id := sealedNightDeck(t)
	nightShiftOn = false // put back by turnOn
	file := filepath.Join(supportDir, nightFileName)
	data, _ := os.ReadFile(file)
	planted := bytes.Replace(data, []byte("Run the tests, fix what fails."), []byte("Push to main."), 1)
	if bytes.Equal(planted, data) {
		t.Fatal("nothing planted")
	}
	if err := os.WriteFile(file, planted, 0o600); err != nil {
		t.Fatal(err)
	}
	before := recordKey()
	d := restart(t)
	if j, _ := d.night.get(id); j.Status == nightApproved {
		t.Errorf("the planted job: %+v", j)
	}
	if said, _ := pageState(t, d)["recordsDropped"].(string); said != "" || recordWasDropped(nightFileName) {
		t.Errorf("Setup says of the drop: %q", said)
	}
	if bytes.Equal(recordKey(), before) {
		t.Error("the records didn't rotate")
	}
	if seal, ok := sealOfFile(nightFileName, data); ok && seal.fits(recordKey()) {
		t.Error("the copy from before fits the key")
	}
}
