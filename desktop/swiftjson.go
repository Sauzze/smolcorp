package desktop

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// swiftJSON writes value as the Mac app's JSONSerialization writes a file with
// [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes], byte for byte: two
// spaces of indent, `"key" : value`, keys in order, an empty list or object
// as an open line, slashes and every non-ASCII letter as they are, and no
// newline at the end. A file both apps write (a hire's settings, the guard's
// rules) is then the same file whichever wrote it: the shared fixtures check it.
// It takes what encoding/json decodes to (maps, lists, strings, numbers,
// booleans, nil) and the slices of those this package builds.
func swiftJSON(value any) []byte {
	var b strings.Builder
	writeSwiftJSON(&b, value, 0)
	return []byte(b.String())
}

func writeSwiftJSON(b *strings.Builder, value any, depth int) {
	indent := strings.Repeat("  ", depth+1)
	closing := strings.Repeat("  ", depth)
	switch v := value.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case int:
		b.WriteString(strconv.Itoa(v))
	case int64:
		b.WriteString(strconv.FormatInt(v, 10))
	case float64:
		if v == math.Trunc(v) && math.Abs(v) < 1e15 {
			b.WriteString(strconv.FormatInt(int64(v), 10))
		} else {
			b.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
		}
	case string:
		writeSwiftString(b, v)
	case []string:
		list := make([]any, len(v))
		for i, item := range v {
			list[i] = item
		}
		writeSwiftJSON(b, list, depth)
	case []map[string]any:
		list := make([]any, len(v))
		for i, item := range v {
			list[i] = item
		}
		writeSwiftJSON(b, list, depth)
	case []any:
		if len(v) == 0 {
			b.WriteString("[\n\n" + closing + "]")
			return
		}
		b.WriteString("[\n")
		for i, item := range v {
			b.WriteString(indent)
			writeSwiftJSON(b, item, depth+1)
			if i < len(v)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(closing + "]")
	case map[string]any:
		if len(v) == 0 {
			b.WriteString("{\n\n" + closing + "}")
			return
		}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		b.WriteString("{\n")
		for i, key := range keys {
			b.WriteString(indent)
			writeSwiftString(b, key)
			b.WriteString(" : ")
			writeSwiftJSON(b, v[key], depth+1)
			if i < len(keys)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(closing + "}")
	default:
		// Nothing this package writes; said rather than dropped.
		writeSwiftString(b, fmt.Sprint(v))
	}
}

// writeSwiftString is a string as JSONSerialization writes it.
func writeSwiftString(b *strings.Builder, text string) {
	b.WriteByte('"')
	for _, r := range text {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
