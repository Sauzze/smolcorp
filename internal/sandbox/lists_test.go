package sandbox

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// SA-X2's lists: one row per entry, here, as the plan names it. An entry added to a list with no row here fails, and a
// row with no entry fails: each entry is thought about once. Each list that must never hold something has a row per
// thing it must not hold.

func TestEachSocketOnAListHasItsRow(t *testing.T) {
	rows := map[string][]Socket{
		"darwin": {
			{"DNS", "/private/var/run/mDNSResponder", Always, ""},
			{"the system log", "/private/var/run/syslog", Always, ""},
			{"the SSH agent", "$SSH_AUTH_SOCK", Always, ""},
			{"1Password's SSH agent", "~/Library/Group Containers/2BUA8C4S2C.com.1password/t/agent.sock", Always, ""},
			{"the GPG agent", "$GNUPGHOME/S.gpg-agent", GPGSigning, ""},
			{"the GPG agent", "~/.gnupg/S.gpg-agent", GPGSigning, ""},
			{"Postgres", "/private/tmp/.s.PGSQL.5432", DatabaseOptIn, ""},
			{"MySQL", "/private/tmp/mysql.sock", DatabaseOptIn, ""},
		},
		"linux": {
			{"the system log", "/dev/log", Always, ""},
			{"the SSH agent", "$SSH_AUTH_SOCK", Always, ""},
			{"1Password's SSH agent", "~/.1password/agent.sock", Always, ""},
			{"the GPG agent", "$XDG_RUNTIME_DIR/gnupg/S.gpg-agent", GPGSigning, ""},
			{"the GPG agent", "$GNUPGHOME/S.gpg-agent", GPGSigning, ""},
			{"the GPG agent", "~/.gnupg/S.gpg-agent", GPGSigning, ""},
			{"Postgres", "/var/run/postgresql/.s.PGSQL.5432", DatabaseOptIn, ""},
			{"Postgres", "/tmp/.s.PGSQL.5432", DatabaseOptIn, ""},
			{"MySQL", "/var/run/mysqld/mysqld.sock", DatabaseOptIn, ""},
		},
		"windows": nil,
	}
	for goos, want := range rows {
		got := Sockets(goos)
		if len(got) != len(want) {
			t.Errorf("%s: %d sockets, %d rows", goos, len(got), len(want))
		}
		for i := range min(len(got), len(want)) {
			g, w := got[i], want[i]
			t.Run(goos+"/"+g.At, func(t *testing.T) {
				if g.Name != w.Name || g.At != w.At || g.Need != w.Need || g.Why == "" {
					t.Errorf("%+v, want %+v with a why", g, w)
				}
				// No docker, screen or desktop session socket is ever on it (AQ4, §A3.2).
				for _, never := range []string{"docker", "containerd", "podman", "screen", "dbus", "/run/user", "$XDG_RUNTIME_DIR/bus", "tmux"} {
					if strings.Contains(strings.ToLower(g.At), never) {
						t.Errorf("%s is on the list", g.At)
					}
				}
			})
		}
	}
}

func TestASocketIsGivenWhereItIsAndWhereItsLinkLeads(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the socket lists are a Mac's and Linux's")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "home")
	agent := filepath.Join(base, "1p", "agent.sock")
	for _, dir := range []string{filepath.Join(home, ".1password"), filepath.Dir(agent)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(agent, filepath.Join(home, ".1password", "agent.sock")); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"SSH_AUTH_SOCK": filepath.Join(base, "ssh", "agent.sock"), "XDG_RUNTIME_DIR": "run/user/501", "GNUPGHOME": ""}
	getenv := func(name string) string { return env[name] }
	for _, c := range []struct {
		gpg, database bool
		want          []string
	}{
		{false, false, []string{"/dev/log", env["SSH_AUTH_SOCK"], filepath.Join(home, ".1password", "agent.sock"), agent}},
		// The runtime folder's isn't whole, and GNUPGHOME is unset: only the home folder's gpg socket.
		{true, false, []string{"/dev/log", env["SSH_AUTH_SOCK"], filepath.Join(home, ".1password", "agent.sock"), agent, filepath.Join(home, ".gnupg", "S.gpg-agent")}},
		{false, true, []string{"/dev/log", env["SSH_AUTH_SOCK"], filepath.Join(home, ".1password", "agent.sock"), agent,
			"/var/run/postgresql/.s.PGSQL.5432", "/tmp/.s.PGSQL.5432", "/var/run/mysqld/mysqld.sock"}},
	} {
		got := SocketPaths(Sockets("linux"), home, getenv, c.gpg, c.database)
		// The system's own may be links on this computer (/var/run, /tmp): their own spellings are what is checked.
		got = slices.DeleteFunc(got, func(p string) bool {
			return !strings.HasPrefix(p, base) && !slices.Contains(c.want, p)
		})
		if !slices.Equal(got, c.want) {
			t.Errorf("gpg %v, database %v: %q, want %q", c.gpg, c.database, got, c.want)
		}
	}
	if got := SocketPaths(Sockets("darwin"), "", getenv, true, true); len(got) != 0 {
		t.Errorf("with no home folder: %q", got)
	}
}

func TestEachMachServiceHasItsRowAndNoneOfTheNeverListIsOnIt(t *testing.T) {
	rows := []string{
		"com.apple.system.opendirectoryd.libinfo", "com.apple.system.opendirectoryd.membership", "com.apple.system.logger",
		"com.apple.logd", "com.apple.trustd.agent", "com.apple.SecurityServer", "com.apple.securityd.xpc", "com.apple.FSEvents",
		"com.apple.SystemConfiguration.configd", "com.apple.system.notification_center", "com.apple.cfprefsd.daemon",
		"com.apple.cfprefsd.agent", "com.apple.bsd.dirhelper", "com.apple.PowerManagement.control", "com.apple.fonts",
		"com.apple.FontObjectsServer",
	}
	never := []string{
		"com.apple.windowserver.active", "com.apple.CARenderServer", "com.apple.pasteboard.1",
		"com.apple.coreservices.launchservicesd", "com.apple.lsd.mapdb", "com.apple.CoreServices.coreservicesd",
		"com.apple.coreservices.quarantine-resolver", "com.apple.coreservices.appleevents",
		"com.apple.distributed_notifications@Uv3", "com.apple.tccd", "com.apple.tccd.system", "com.apple.audio.coreaudiod",
		"com.apple.audio.audiohald", "com.apple.audio.systemsoundserver", "com.apple.cmio.registerassistantservice",
		"com.apple.replayd", "com.apple.CoreSimulator.CoreSimulatorService", "com.apple.metadata.mds", "com.apple.iohideventsystem",
	}
	allowed := MachServices()
	if len(allowed) != len(rows) {
		t.Errorf("%d services, %d rows", len(allowed), len(rows))
	}
	for i, s := range allowed {
		t.Run(s.Name, func(t *testing.T) {
			if i >= len(rows) || s.Name != rows[i] || s.Why == "" || strings.ContainsAny(s.Name, ` "()*`) {
				t.Errorf("%+v isn't its row", s)
			}
		})
	}
	nevers := NeverMachServices()
	if len(nevers) != len(never) {
		t.Errorf("%d never-services, %d rows", len(nevers), len(never))
	}
	for i, s := range nevers {
		t.Run("never "+s.Name, func(t *testing.T) {
			if i >= len(never) || s.Name != never[i] || s.Why == "" {
				t.Errorf("%+v isn't its row", s)
			}
			if slices.ContainsFunc(allowed, func(a MachService) bool { return strings.EqualFold(a.Name, s.Name) }) {
				t.Errorf("%s is on the list", s.Name)
			}
		})
	}
}

func TestEachIOKitClassHasItsRowAndNoHIDClientIsOnIt(t *testing.T) {
	rows := []IOKitClass{{"IOSurfaceRootUserClient", false, ""}, {"IOSurfaceSendRight", true, ""}, {"RootDomainUserClient", false, ""}}
	never := []string{"IOHIDParamUserClient", "IOHIDLibUserClient", "IOHIDEventSystemUserClient", "IOFramebufferSharedUserClient"}
	allowed := IOKitClasses()
	if len(allowed) != len(rows) {
		t.Errorf("%d classes, %d rows", len(allowed), len(rows))
	}
	for i, c := range allowed {
		t.Run(c.Class, func(t *testing.T) {
			if i >= len(rows) || c.Class != rows[i].Class || c.UserClient != rows[i].UserClient || c.Why == "" {
				t.Errorf("%+v isn't its row", c)
			}
			if strings.Contains(c.Class, "HID") || strings.Contains(c.Class, "Framebuffer") {
				t.Errorf("%s reaches the keyboard, the mouse or the screen", c.Class)
			}
		})
	}
	nevers := NeverIOKitClasses()
	if len(nevers) != len(never) {
		t.Errorf("%d never-classes, %d rows", len(nevers), len(never))
	}
	for i, c := range nevers {
		t.Run("never "+c.Class, func(t *testing.T) {
			if i >= len(never) || c.Class != never[i] || c.Why == "" {
				t.Errorf("%+v isn't its row", c)
			}
			if slices.ContainsFunc(allowed, func(a IOKitClass) bool { return a.Class == c.Class }) {
				t.Errorf("%s is on the list", c.Class)
			}
		})
	}
}

func TestEachLocalPortHasItsRowAndOnlyTheDatabasesOpenOnTheOptIn(t *testing.T) {
	rows := []Port{{22, false, ""}, {2375, false, ""}, {2376, false, ""}, {5432, true, ""}, {3306, true, ""}}
	got := LocalPorts()
	if len(got) != len(rows) {
		t.Errorf("%d ports, %d rows", len(got), len(rows))
	}
	for i, p := range got {
		t.Run(strconv.Itoa(p.Port), func(t *testing.T) {
			if i >= len(rows) || p.Port != rows[i].Port || p.OpenOnOptIn != rows[i].OpenOnOptIn || p.Why == "" {
				t.Errorf("%+v isn't its row", p)
			}
			if slices.Contains(DeniedPorts(false), p.Port) != true {
				t.Errorf("%d is open with no opt-in", p.Port)
			}
			if slices.Contains(DeniedPorts(true), p.Port) == p.OpenOnOptIn {
				t.Errorf("%d after the opt-in: denied %v", p.Port, !p.OpenOnOptIn)
			}
		})
	}
}

func TestEachPlaceLinuxCoversOrMasksHasItsRow(t *testing.T) {
	covered := []string{"/run/screen", "$XDG_RUNTIME_DIR"}
	masked := []string{"/run/dbus/system_bus_socket", "/var/run/dbus/system_bus_socket", "/run/docker.sock", "/var/run/docker.sock",
		"/run/containerd/containerd.sock", "/run/podman/podman.sock"}
	for _, list := range []struct {
		name string
		got  []Place
		want []string
	}{{"covered", LinuxCovered(), covered}, {"masked", LinuxMasked(), masked}} {
		if len(list.got) != len(list.want) {
			t.Errorf("%s: %d places, %d rows", list.name, len(list.got), len(list.want))
		}
		for i, p := range list.got {
			t.Run(list.name+p.At, func(t *testing.T) {
				if i >= len(list.want) || p.At != list.want[i] || p.Why == "" {
					t.Errorf("%+v isn't its row", p)
				}
				// Nothing Linux covers or masks is on its socket list.
				if slices.ContainsFunc(Sockets("linux"), func(s Socket) bool { return s.At == p.At }) {
					t.Errorf("%s is covered and allowed", p.At)
				}
			})
		}
	}
}
