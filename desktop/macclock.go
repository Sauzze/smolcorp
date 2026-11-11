package desktop

import (
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
	if d.macHost() == nil {
		return t.Format("15:04")
	}
	return macShortTime(t, macTimePrefs())
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
