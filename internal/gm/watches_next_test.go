package gm

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// A watch that fired and is no longer active has no NEXT; an active one keeps it.
func TestWatchesShowNoNextForAnInactiveWatch(t *testing.T) {
	var out bytes.Buffer
	due := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	list := []Watch{
		{ID: "w-done", Kind: "time", Minutes: 1, Fired: 1, Active: false},
		{ID: "w-quiet", Kind: "idle", Minutes: 7, Active: false},
		{ID: "w-live", Kind: "time", Minutes: 1, Active: true, Due: Millis(due.UnixMilli())},
		{ID: "w-idle", Kind: "idle", Minutes: 5, Active: true},
	}
	if err := RenderWatches(&out, list, time.UTC); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out.String(), "\n") {
		switch {
		case strings.HasPrefix(line, "w-done"), strings.HasPrefix(line, "w-quiet"):
			if strings.Contains(line, "min") || strings.Contains(line, "due") {
				t.Errorf("inactive watch shows a next time: %q", line)
			}
		case strings.HasPrefix(line, "w-live"):
			if !strings.Contains(line, "due 03:04:05") {
				t.Errorf("active time watch lost its time: %q", line)
			}
		case strings.HasPrefix(line, "w-idle"):
			if !strings.Contains(line, "5 min") {
				t.Errorf("active idle watch lost its minutes: %q", line)
			}
		}
	}
}

// A fraction of a minute decodes and shows without a trailing ".0".
func TestWatchesTakeAFractionOfAMinute(t *testing.T) {
	var w Watch
	if err := json.Unmarshal([]byte(`{"id":"w1","kind":"idle","minutes":0.5,"active":true}`), &w); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RenderWatches(&out, []Watch{w, {ID: "w2", Kind: "idle", Minutes: 5, Active: true}}, time.UTC); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "0.5 min") || !strings.Contains(out.String(), "5 min") || strings.Contains(out.String(), "5.0") {
		t.Errorf("rendered %q", out.String())
	}
}
