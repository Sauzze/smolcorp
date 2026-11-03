package gm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
)

// Millis is a whole number the app sends as JSON (a time in milliseconds, or
// a count of seconds). It also reads a fractional one, like 1793524230038.1179,
// and rounds it down, so an older or another app can't break the CLI (M4).
type Millis int64

// UnmarshalJSON reads 123, 123.9 or 1.2e12; null leaves the value alone.
func (m *Millis) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	var whole int64
	if err := json.Unmarshal(b, &whole); err == nil {
		*m = Millis(whole)
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || math.Abs(f) >= 1<<63 {
		return fmt.Errorf("number %s is out of range", b)
	}
	*m = Millis(math.Floor(f))
	return nil
}
