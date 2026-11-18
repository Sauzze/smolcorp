package sandbox

import (
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// The lists (SA-X2, SANDBOX-PLAN §A3.1, §A3.2, §A4.2): the unix sockets, Mach services, IOKit classes and local TCP
// ports a desk's sandbox names, as data the Mac's profile (M2) and Linux's view (L1, L3) are written from. Sockets,
// services and classes are denied by default, and what is on their list allowed; TCP is allowed anywhere, but the
// ports on its list. Each entry says why, and has its own row in lists_test.go: one added without a row fails there.
// What must never be on a list is a list too, each entry with a row that it isn't.
//
// The Mach and IOKit lists start from Claude Code's own Seatbelt profile (the plan's source, read from its 2.1
// program), less what §A3.1 keeps off (WindowServer, the pasteboard, LaunchServices, TCC, audio, the camera, screen
// capture, CoreSimulator, Spotlight, Apple Events), plus what §A3.1 names (FSEvents, configd, cfprefsd). User lookup
// is proven needed here (P126SBXb S4). cfprefsd's user side is off since FXPLANTSTERM: it wrote a named domain's plist
// in the owner's real ~/Library/Preferences for a desk, a Seatbelt deny of that folder notwithstanding. Its system side
// is off too since FXPLANTSTERMR4: root's daemon, which writes the any-user and system domains, and which no desk needs.
// With neither, CFPreferences in a desk reads and writes the preference files itself, as the desk: it reads what the
// desk may read (the owner's preferences, as `cat` would), and a write meets the desk's own file rules, the Policy's
// row over ~/Library/Preferences and root's /Library/Preferences. git, node, python, go, clang, swift, xcodebuild,
// security and codesign ran the same with and without each on this Mac (FXPLANTSTERM's and FXPLANTSTERMR4's reports);
// the normal-work rows that need headless Chrome, cargo or swift test run only on the Mac Studio and in CI. Keychains are found without it: `security` reads its search list from its plist
// itself (TestOnAMacADesksClaudeSavesNoSignInAndStillFindsItsKeychains). M5's normal-work list (escape2_darwin_test.go)
// runs under both lists as they are: on this Mac a commit signed through the SSH agent and a push, npm, pnpm, bun, go
// test, pip in a venv and swift build need nothing more (cargo and swift test run in CI's Mac sandbox job). Headless
// Chrome needs the WindowServer's and LaunchServices' services, which stay off.
//
// That settles the Mach list (without it the signed commit and git pull fail), not the IOKit list: no normal-work row
// needs any IOKit entry. Every row passes with no IOKit class at all (TestOnAMacNormalWorkNeedsNoIOKitClass, which CI's
// Mac sandbox job runs for cargo, swift test and Chrome too). IOSurface's two entries are named only for headless
// Chrome's and Electron tests' drawing, and Chrome doesn't run in a desk; RootDomainUserClient has no row. The three
// entries are SIERRA's to keep or narrow.

// Need is when an entry is on a desk's list.
type Need int8

const (
	// Always: on every desk's list.
	Always Need = iota
	// GPGSigning: while the owner's git signs with gpg (§A3.1: "the GPG agent when the owner's git uses it").
	GPGSigning
	// DatabaseOptIn: once AQ21's opt-in has passed (C4): the owner said the database asks for a password, and the
	// core's password-less superuser login failed (M12).
	DatabaseOptIn
)

// Socket is a unix socket a desk may connect to, besides its own and the desk-start socket, which its Places name.
type Socket struct {
	Name string
	// At is where: a whole path, `~/…` in the home folder, or `$VAR` or `$VAR/…` from the core's environment.
	At   string
	Need Need
	Why  string
}

// The sockets by system. Linux's are bound back into the view (L1); every other listening socket is masked (L3).
var socketLists = map[string][]Socket{
	"darwin": {
		{"DNS", "/private/var/run/mDNSResponder", Always, "names are looked up through it"},
		{"the system log", "/private/var/run/syslog", Always, "programs log through it"},
		{"the SSH agent", "$SSH_AUTH_SOCK", Always, "git over SSH, and commits signed with an SSH key (AQ12)"},
		{"1Password's SSH agent", "~/Library/Group Containers/2BUA8C4S2C.com.1password/t/agent.sock", Always, "this owner's commits are signed through it (S6, AQ12)"},
		{"the GPG agent", "$GNUPGHOME/S.gpg-agent", GPGSigning, "commits signed with gpg, where GNUPGHOME puts it"},
		{"the GPG agent", "~/.gnupg/S.gpg-agent", GPGSigning, "commits signed with gpg"},
		{"Postgres", "/private/tmp/.s.PGSQL.5432", DatabaseOptIn, "its default login lets a desk read files and run commands as you (C4)"},
		{"MySQL", "/private/tmp/mysql.sock", DatabaseOptIn, "its default login lets a desk read files and run commands as you (C4)"},
	},
	"linux": {
		{"the system log", "/dev/log", Always, "programs log through it"},
		{"the SSH agent", "$SSH_AUTH_SOCK", Always, "git over SSH, and commits signed with an SSH key (AQ12)"},
		{"1Password's SSH agent", "~/.1password/agent.sock", Always, "commits signed through 1Password (AQ12)"},
		{"the GPG agent", "$XDG_RUNTIME_DIR/gnupg/S.gpg-agent", GPGSigning, "commits signed with gpg"},
		{"the GPG agent", "$GNUPGHOME/S.gpg-agent", GPGSigning, "commits signed with gpg, where GNUPGHOME puts it"},
		{"the GPG agent", "~/.gnupg/S.gpg-agent", GPGSigning, "commits signed with gpg, with no runtime folder"},
		{"Postgres", "/var/run/postgresql/.s.PGSQL.5432", DatabaseOptIn, "its default login lets a desk read files and run commands as you (C4)"},
		{"Postgres", "/tmp/.s.PGSQL.5432", DatabaseOptIn, "its default login lets a desk read files and run commands as you (C4)"},
		{"MySQL", "/var/run/mysqld/mysqld.sock", DatabaseOptIn, "its default login lets a desk read files and run commands as you (C4)"},
	},
}

// Sockets is goos's socket list (a copy).
func Sockets(goos string) []Socket { return slices.Clone(socketLists[goos]) }

// SocketPaths is the sockets on list a desk may connect to, whole: each as spelled and, where a link leads elsewhere,
// where it leads (a Mac's profile checks the place a link resolves to: T8), but only when each link on the way, and
// every folder above it, is root's alone: a link the owner's account can change, a desk can too (FXSAL3R1, RT126SAL3
// B3). An entry whose way goes through a session bus, systemd --user's own (busPlace) or a socket Linux's view always
// masks, at any hop, is left out whole (SIERRA's class ruling). getenv is the core's environment. An entry whose
// variable is unset, or not whole, is left out; so is one whose Need isn't met.
func SocketPaths(list []Socket, home string, getenv func(string) string, gpg, database bool) []string {
	return socketPaths(list, home, getenv, gpg, database, osView{})
}

func socketPaths(list []Socket, home string, getenv func(string) string, gpg, database bool, sys viewFS) []string {
	never := func(at string) bool {
		return busPlace(at, []string{getenv("XDG_RUNTIME_DIR")}) ||
			slices.ContainsFunc(linuxMasked, func(m Place) bool { return m.At == at })
	}
	rootsAlone := func(folder string) bool {
		for ; ; folder = path.Dir(folder) {
			st, err := sys.stat(folder, false)
			if err != nil || st.uid != 0 || st.mode.Perm()&0o022 != 0 {
				return false
			}
			if folder == "/" {
				return true
			}
		}
	}
	var out []string
	for _, s := range list {
		if s.Need == GPGSigning && !gpg || s.Need == DatabaseOptIn && !database {
			continue
		}
		at := s.At
		if rest, ok := strings.CutPrefix(at, "~/"); ok {
			at = filepath.Join(home, filepath.FromSlash(rest))
		} else if rest, ok := strings.CutPrefix(at, "$"); ok {
			name, tail, _ := strings.Cut(rest, "/")
			at = getenv(name)
			if tail != "" && at != "" {
				at = filepath.Join(at, filepath.FromSlash(tail))
			}
		}
		if !filepath.IsAbs(at) || home == "" {
			continue
		}
		at = filepath.Clean(at)
		if _, ok := way(sys, at, func(string) bool { return true }, never); !ok {
			continue // a bus, systemd's, or a socket always masked, somewhere on its way
		}
		places := []string{at}
		if real, ok := way(sys, at, rootsAlone, never); ok {
			places = append(places, real)
		}
		for _, p := range places {
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	return out
}

// Place is a folder or a socket Linux's view covers, by path, `$VAR` from the core's environment.
type Place struct {
	At, Why string
}

var linuxCovered = []Place{
	{"/run/screen", "GNU screen's sessions"},
	{"$XDG_RUNTIME_DIR", "the desktop session's sockets: D-Bus's session bus, Wayland, PipeWire, the keyring, systemd --user, rootless Docker and Podman (the agents on Linux's socket list are bound back)"},
}

var linuxMasked = []Place{
	{"/run/dbus/system_bus_socket", "the system D-Bus"},
	{"/var/run/dbus/system_bus_socket", "the system D-Bus, by its old path"},
	{"/run/docker.sock", "Docker: a container reaches your files as you (AQ4)"},
	{"/var/run/docker.sock", "Docker, by its old path"},
	{"/run/containerd/containerd.sock", "containerd, under Docker and others"},
	{"/run/podman/podman.sock", "Podman's system service"},
}

// LinuxCovered is the folders Linux's view covers with an empty tmpfs (L1, L3), whatever is made in them later.
func LinuxCovered() []Place { return slices.Clone(linuxCovered) }

// LinuxMasked is the sockets Linux's view masks (L3), there at the start or not, beside each listening socket off its
// list; the folders root has alone they are in are covered (mask.go), so one made there later isn't seen.
func LinuxMasked() []Place { return slices.Clone(linuxMasked) }

// MachService is a Mach service by its global name.
type MachService struct {
	Name, Why string
}

var machServices = []MachService{
	{"com.apple.system.opendirectoryd.libinfo", "user and group lookup: whoami, ~, getpwuid (S4)"},
	{"com.apple.system.opendirectoryd.membership", "group membership: access checks, id"},
	{"com.apple.system.logger", "the system log (asl)"},
	{"com.apple.logd", "the unified log (os_log)"},
	{"com.apple.trustd.agent", "TLS certificates: Go's, curl's and Node's checks"},
	{"com.apple.SecurityServer", "the Security server: Claude Code's sign-in, kept in the Keychain"},
	{"com.apple.securityd.xpc", "the Security server's XPC side, for the same"},
	{"com.apple.FSEvents", "file watchers (FSEvents)"},
	{"com.apple.SystemConfiguration.configd", "network settings: proxies and DNS settings"},
	{"com.apple.system.notification_center", "notifyd: system notifications between processes"},
	{"com.apple.bsd.dirhelper", "the per-user temporary and cache folders (confstr)"},
	{"com.apple.PowerManagement.control", "keeping the Mac awake through a long run (caffeinate)"},
	{"com.apple.fonts", "fonts: headless Chrome's text"},
	{"com.apple.FontObjectsServer", "fonts: headless Chrome's text"},
}

var neverMachServices = []MachService{
	{"com.apple.windowserver.active", "the WindowServer: a session to read or post events into (S5, B4)"},
	{"com.apple.CARenderServer", "the window server's drawing"},
	{"com.apple.pasteboard.1", "the pasteboard: Setup's keys, a line to plant for the owner to paste (AQ14)"},
	{"com.apple.coreservices.launchservicesd", "LaunchServices: open apps, as the owner (lsopen)"},
	{"com.apple.lsd.mapdb", "LaunchServices' database"},
	{"com.apple.CoreServices.coreservicesd", "LaunchServices' open requests"},
	{"com.apple.coreservices.quarantine-resolver", "LaunchServices' quarantine, for opening"},
	{"com.apple.coreservices.appleevents", "Apple Events: drive other apps"},
	{"com.apple.distributed_notifications@Uv3", "notifications to the owner's apps"},
	{"com.apple.tccd", "TCC: privacy grants"},
	{"com.apple.tccd.system", "TCC: privacy grants, the system's"},
	{"com.apple.audio.coreaudiod", "audio: the microphone"},
	{"com.apple.audio.audiohald", "audio: the microphone"},
	{"com.apple.audio.systemsoundserver", "audio: system sounds"},
	{"com.apple.cmio.registerassistantservice", "the camera"},
	{"com.apple.replayd", "screen capture"},
	{"com.apple.CoreSimulator.CoreSimulatorService", "the iOS Simulator"},
	{"com.apple.metadata.mds", "Spotlight: every file's contents, indexed"},
	{"com.apple.iohideventsystem", "HID events: keystrokes, posted events"},
	{"com.apple.cfprefsd.agent", "the owner's preferences (CFPreferences): it writes an app's named domain in the real " +
		"~/Library/Preferences for the desk, past every file rule, Terminal's profile command included (FXPLANTSTERM)"},
	{"com.apple.cfprefsd.daemon", "the system's preferences (CFPreferences), written by root for the desk: no desk needs it, " +
		"and without it a desk's CFPreferences reads and writes its files itself, as the desk, under its own file rules (FXPLANTSTERMR4)"},
}

// MachServices is the Mach services a desk may look up on a Mac (a copy).
func MachServices() []MachService { return slices.Clone(machServices) }

// NeverMachServices is the Mach services no desk may look up, ever (a copy).
func NeverMachServices() []MachService { return slices.Clone(neverMachServices) }

// IOKitClass is an IOKit class a desk may open: a registry entry's class, or a user client's.
type IOKitClass struct {
	Class      string
	UserClient bool
	Why        string
}

var iokitClasses = []IOKitClass{
	{"IOSurfaceRootUserClient", false, "IOSurface: headless Chrome's and Electron tests' drawing"},
	{"IOSurfaceSendRight", true, "IOSurface: passing a surface between processes"},
	{"RootDomainUserClient", false, "power assertions: keeping the Mac awake through a long run"},
}

var neverIOKitClasses = []IOKitClass{
	{"IOHIDParamUserClient", true, "posts keyboard and mouse events (IOHIDPostEvent): drives Smolcorp's and the owner's apps (N1, M5)"},
	{"IOHIDLibUserClient", true, "reads keyboards and mice: every key the owner types"},
	{"IOHIDEventSystemUserClient", true, "HID events, the event system's side"},
	{"IOFramebufferSharedUserClient", true, "the screen's framebuffer"},
}

// IOKitClasses is the IOKit classes a desk may open on a Mac (a copy). T13: everyday tools need none.
func IOKitClasses() []IOKitClass { return slices.Clone(iokitClasses) }

// NeverIOKitClasses is the IOKit classes no desk may open, ever (a copy).
func NeverIOKitClasses() []IOKitClass { return slices.Clone(neverIOKitClasses) }

// Port is a local TCP port a desk can't connect to on a Mac: on localhost, IPv4 and IPv6 (R5).
type Port struct {
	Port int
	// OpenOnOptIn is whether AQ21's opt-in opens it (C4).
	OpenOnOptIn bool
	Why         string
}

var localPorts = []Port{
	{22, false, "ssh to this Mac as the owner, while Remote Login is on (C5)"},
	{2375, false, "Docker over TCP: a container reaches your files as you (AQ4)"},
	{2376, false, "Docker over TLS, the same"},
	{5432, true, "Postgres: its default login lets a desk read files and run commands as you (C4)"},
	{3306, true, "MySQL: the same (C4)"},
}

// LocalPorts is the local TCP port list (a copy).
func LocalPorts() []Port { return slices.Clone(localPorts) }

// DeniedPorts is the local TCP ports a desk can't connect to: all of the list, but the database ones once database
// (AQ21's opt-in) has passed.
func DeniedPorts(database bool) []int {
	var out []int
	for _, p := range localPorts {
		if !p.OpenOnOptIn || !database {
			out = append(out, p.Port)
		}
	}
	return out
}
