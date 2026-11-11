package desktop

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// The time setup says a webhook delivery was sent at: under the Mac app in the
// Mac's own short time, as the Swift engine says it (DateFormatter's .short,
// Deck.described), "1:13 PM" or "13:13" as your Region settings have it;
// elsewhere 24-hour, as Linux and Windows always had it.

// deliveryClock is t as setup says a delivery's time. Without the lock.
func (d *Deck) deliveryClock(t time.Time) string {
	h := d.macHost()
	if h == nil {
		return t.Format("15:04")
	}
	// The Mac's own pattern, asked of the app, so it's exact; the Region settings' reading below only where the app
	// doesn't say or the pattern has a part this doesn't know.
	if format, err := hostTimeFormat(h); err == nil {
		if text, ok := format.format(t); ok {
			return text
		}
	}
	return macShortTime(t, macTimePrefs())
}

// macTimeFormat is how the Mac writes a short time: DateFormatter's .short time style as an ICU pattern ("h:mm a",
// "HH:mm"), and its words for AM and PM (HostBridge.shortTimeFormat, the bridge's time-format).
type macTimeFormat struct {
	Pattern string `json:"pattern"`
	AM      string `json:"am"`
	PM      string `json:"pm"`
}

// format is t as the pattern writes it, and false when there's no pattern or it has a part this doesn't write
// (another calendar's, a day period): the caller then reads the Region settings instead. It writes the hours, the
// minutes and seconds, AM/PM, quoted text and the punctuation between, which is all a short time has in any language
// whose numbers are the Latin digits.
func (f macTimeFormat) format(t time.Time) (string, bool) {
	if f.Pattern == "" {
		return "", false
	}
	runes := []rune(f.Pattern)
	var out strings.Builder
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case r == '\'':
			// Quoted text, up to the closing quote; two quotes together are one.
			if i+1 < len(runes) && runes[i+1] == '\'' {
				out.WriteRune('\'')
				i += 2
				continue
			}
			i++
			for i < len(runes) {
				if runes[i] == '\'' {
					if i+1 < len(runes) && runes[i+1] == '\'' {
						out.WriteRune('\'') // two quotes inside quoted text are one
						i += 2
						continue
					}
					break
				}
				out.WriteRune(runes[i])
				i++
			}
			i++
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
			n := 1
			for i+n < len(runes) && runes[i+n] == r {
				n++
			}
			i += n
			var value int
			switch r {
			case 'H':
				value = t.Hour()
			case 'h':
				value = t.Hour() % 12
				if value == 0 {
					value = 12
				}
			case 'K':
				value = t.Hour() % 12
			case 'k':
				value = t.Hour()
				if value == 0 {
					value = 24
				}
			case 'm':
				value = t.Minute()
			case 's':
				value = t.Second()
			case 'a':
				symbol := f.AM
				if t.Hour() >= 12 {
					symbol = f.PM
				}
				if symbol == "" {
					return "", false
				}
				out.WriteString(symbol)
				continue
			default:
				return "", false
			}
			if n >= 2 {
				fmt.Fprintf(&out, "%02d", value)
			} else {
				fmt.Fprintf(&out, "%d", value)
			}
		default:
			out.WriteRune(r)
			i++
		}
	}
	return out.String(), true
}

// macTimeSettings is what macOS's Region settings say of the time: its 24-hour
// and 12-hour switches (AppleICUForce24HourTime, AppleICUForce12HourTime, "1"
// when on) and the locale (AppleLocale, as "en_US" or "en_GB@rg=frzzzz").
type macTimeSettings struct {
	force24, force12, locale string
}

// macTimePrefs reads them: a variable, for the tests.
var macTimePrefs = func() macTimeSettings {
	read := func(key string) string {
		out, err := exec.Command("/usr/bin/defaults", "read", "-g", key).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	return macTimeSettings{force24: read("AppleICUForce24HourTime"), force12: read("AppleICUForce12HourTime"), locale: read("AppleLocale")}
}

// twelveHourRegions are the regions whose English short time is 12-hour (CLDR's
// hour cycle "h"), as macOS writes it for an English locale with neither switch on.
var twelveHourRegions = []string{"US", "CA", "AU", "NZ", "IN", "PH", "PK", "BD", "SG", "MY", "HK", "EG", "SA", "AE"}

// macShortTime is t as macOS's short time style writes it with these settings:
// the switches first, then the locale's own way; 12-hour with AM or PM after a
// narrow no-break space, as macOS writes it. Only English says AM/PM: another
// language with neither switch on is 24-hour.
func macShortTime(t time.Time, s macTimeSettings) string {
	twelve := false
	switch {
	case s.force24 == "1":
	case s.force12 == "1":
		twelve = true
	default:
		language, region, _ := strings.Cut(s.locale, "_")
		region, override, _ := strings.Cut(region, "@")
		if rest, found := strings.CutPrefix(override, "rg="); found && len(rest) >= 2 {
			region = strings.ToUpper(rest[:2])
		}
		twelve = language == "en" && contains(twelveHourRegions, region)
	}
	if twelve {
		return t.Format("3:04") + " " + t.Format("PM")
	}
	return t.Format("15:04")
}
