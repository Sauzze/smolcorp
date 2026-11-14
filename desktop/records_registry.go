package desktop

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
	"sync"
)

// The registry's permissions and config.json's security settings, as sealed records (records.go). Both files hold
// much else that grants nothing (names, folders, colours, layouts), which the Mac app and older tools read: so
// only the part that grants something is sealed, and the seal sits in the file beside it, under recordSeal.

const registryName = "registry.json"

// registryRights is what the registry grants, as its seal covers it: the hires' send keys (their hashes), who may
// write to whom, and each hire's role, state and scope (a GM held to a project).
type registryRights struct {
	Keys     map[string]string `json:"keys"`
	Contacts []Contact         `json:"contacts"`
	Hires    [][5]string       `json:"hires"`
}

// registrySecurityPart is the registry's sealed part, as bytes the same file always gives: none and empty are one,
// as the file leaves both out.
func registrySecurityPart(f registryFile) []byte {
	rights := registryRights{Keys: f.keys, Contacts: f.contacts, Hires: [][5]string{}}
	if len(rights.Keys) == 0 {
		rights.Keys = nil
	}
	if len(rights.Contacts) == 0 {
		rights.Contacts = nil
	}
	for _, h := range f.hires {
		rights.Hires = append(rights.Hires, [5]string{h.ID, h.Role, h.State, h.Scope, h.ScopeRoles})
	}
	data, _ := json.Marshal(rights)
	return data
}

// heldRegistries is the registry as the core first read it or last wrote it, by path, on every system (§3.2, §3.8,
// RT126SK1 B4): the file is never read again while the core runs (readRegistry). broken: a file that couldn't be
// read when it was, still to be set aside at the first edit.
var heldRegistries = struct {
	sync.Mutex
	byPath map[string]registryFile
	broken map[string]bool
}{byPath: map[string]registryFile{}, broken: map[string]bool{}}

// cloneRegistry is a copy of f that shares nothing with it.
func cloneRegistry(f registryFile) registryFile {
	return registryFile{hires: slices.Clone(f.hires), contacts: slices.Clone(f.contacts), keys: maps.Clone(f.keys), rest: maps.Clone(f.rest)}
}

// holdRegistry keeps f as the registry at path.
func holdRegistry(path string, f registryFile) {
	heldRegistries.Lock()
	heldRegistries.byPath[path] = cloneRegistry(f)
	heldRegistries.Unlock()
}

// heldRegistryFor is a copy of the registry the core holds for path, and whether it holds one.
func heldRegistryFor(path string) (registryFile, bool) {
	heldRegistries.Lock()
	defer heldRegistries.Unlock()
	f, ok := heldRegistries.byPath[path]
	return cloneRegistry(f), ok
}

// heldRegistry is a copy of the registry the core holds for path: none if it holds none.
func heldRegistry(path string) registryFile {
	f, _ := heldRegistryFor(path)
	return f
}

func markBrokenRegistry(path string) {
	heldRegistries.Lock()
	heldRegistries.broken[path] = true
	heldRegistries.Unlock()
}

// takeBrokenRegistry is whether the file at path couldn't be read when the core read it, and is still to be set
// aside: only once.
func takeBrokenRegistry(path string) bool {
	heldRegistries.Lock()
	defer heldRegistries.Unlock()
	broken := heldRegistries.broken[path]
	delete(heldRegistries.broken, path)
	return broken
}

// brokenRegistry is whether the file at path couldn't be read when the core read it, and is still to be set aside:
// nothing but that edit may write over it (RT126SK1R3 N4).
func brokenRegistry(path string) bool {
	heldRegistries.Lock()
	defer heldRegistries.Unlock()
	return heldRegistries.broken[path]
}

// forgetHeldRegistries drops every registry held: for tests, which stand for a new start of the core.
func forgetHeldRegistries() {
	heldRegistries.Lock()
	heldRegistries.byPath, heldRegistries.broken = map[string]registryFile{}, map[string]bool{}
	heldRegistries.Unlock()
}

// vouchRegistry is the registry the core goes by, as it first reads the file, given its bytes and what they say: the
// file's own when its seal fits (or off a Mac, where nothing is sealed); otherwise the team without anything it
// grants (withoutGrants). What another program wrote then grants nothing: no key, no contact, no GM, no scope.
func vouchRegistry(data []byte, f registryFile) registryFile {
	if partCounts(registryName, data) {
		return f
	}
	recordDropped(registryName, "registry.json was changed by another program, or its seal doesn't fit: what it says now grants nothing. "+
		"Your team is kept, but each hire gets a new key when it next starts, and the General Manager and any hire held "+
		"to a project are let go: hire them again.")
	return withoutGrants(f)
}

// registryDropped is a registry file that is there but can't be read as the core starts: the core goes by an empty
// one, so its copy from before mustn't fit again (records.go's recordDropped, RT126SK1R2 N1).
func registryDropped() {
	recordDropped(registryName, "registry.json can't be read: Smolcorp goes by an empty team, and sets the file aside as "+
		"registry.broken.json at the next change.")
}

// withoutGrants is the team f lists, kept with every field that grants nothing (names, folders, branches,
// worktrees: RT126SK1 B5), without what it grants: no send key, no contact, and a GM or a hire held to a project
// no longer open (failed), so none comes back unscoped from what the file says. The owner's reset then seals it.
func withoutGrants(f registryFile) registryFile {
	f = cloneRegistry(f)
	f.keys, f.contacts = nil, nil
	for i, h := range f.hires {
		if isOpen(h) && (h.Role == gmRole || h.Scope != "" || h.ScopeRoles != "") {
			f.hires[i].State = "failed"
		}
	}
	return f
}

// registryTightens is whether the registry going from old to now takes authority away (§3.2, RT126SK1 B3): a send
// key renewed or removed, a contact removed, a hire gone or let go, or a hire's role or scope changed. Those are
// written under a new record key, so a copy from before no longer fits.
func registryTightens(old, now registryFile) bool {
	for id, hash := range old.keys {
		if now.keys[id] != hash {
			return true
		}
	}
	for _, contact := range old.contacts {
		if !slices.Contains(now.contacts, contact) {
			return true
		}
	}
	for _, h := range old.hires {
		i := slices.IndexFunc(now.hires, func(n Hire) bool { return n.ID == h.ID })
		if i < 0 {
			return true
		}
		n := now.hires[i]
		if n.Role != h.Role || n.Scope != h.Scope || n.ScopeRoles != h.ScopeRoles || isOpen(h) && !isOpen(n) {
			return true
		}
	}
	return false
}

// configSecurityPart is config.json's sealed part: who can reach the server, keys to other apps, a GM started by
// hand, the mail socket, the manager and hiring switches, and where session news goes.
func configSecurityPart(c *Config) []byte {
	apps := c.KeystrokeApps
	if len(apps) == 0 {
		apps = nil // left out of the file when empty
	}
	data, _ := json.Marshal(map[string]any{
		"listen": c.Listen, "keystrokesAllowed": c.KeystrokesAllowed, "keystrokeApps": apps,
		"gmByHand": c.GMByHand, "mailInboxSocket": c.MailInboxSocket, "managerEnabled": c.ManagerEnabled,
		"hiringEnabled": c.HiringEnabled, "webhook": c.Webhook,
	})
	return data
}

// vouchConfig puts config's security settings to their safe defaults when the file's seal doesn't fit.
func vouchConfig(c *Config, data []byte) {
	if partCounts("config.json", data) {
		return
	}
	recordDropped("config.json", "config.json's security settings were changed by another program, or their seal doesn't fit: they're "+
		"back to their safe defaults (this computer and Tailscale only, no keys to other apps, no GM started by hand, "+
		"the front desk and hiring off, no webhook). Set them again in Setup.")
	c.Listen, c.KeystrokesAllowed, c.KeystrokeApps, c.GMByHand = "tailscale", false, nil, false
	c.MailInboxSocket, c.ManagerEnabled, c.HiringEnabled, c.Webhook = false, false, false, standardWebhook()
}

// configChanges is whether the security settings going from old to now are changed: any change, loosening
// included, is written under a new record key, so a copy from before a tightening never fits (RT126SK1 B3).
func configChanges(old, now []byte) bool { return old != nil && !bytes.Equal(old, now) }
