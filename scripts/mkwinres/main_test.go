package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

const iconPath = "../../packaging/linux/smolcorp.png"

// The manifest as it must be, written out again here so that a change to the
// generator's text shows up as a failure.
const wantManifest = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security>
      <requestedPrivileges>
        <requestedExecutionLevel level="asInvoker" uiAccess="false"/>
      </requestedPrivileges>
    </security>
  </trustInfo>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <longPathAware xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">true</longPathAware>
      <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">PerMonitorV2,PerMonitor</dpiAwareness>
    </windowsSettings>
  </application>
</assembly>
`

func testSpec(t *testing.T, arch string) spec {
	t.Helper()
	icon, err := os.ReadFile(iconPath)
	if err != nil {
		t.Fatal(err)
	}
	return spec{arch, [4]uint16{1, 24, 0, 28}, "Smolcorp command line", "smolcorp-cli.exe", "smolcorp-cli", "Copyright (c) 2026 Sauzze", icon}
}

// readResources walks a resource directory in section, whose first byte is at
// address base, and returns the data of each (type, name), whatever its language.
// On the way it checks every directory: no flags, no date, no version, no named
// entries, and ids in ascending order, as Windows looks them up by binary search.
func readResources(t *testing.T, section []byte, base uint32) map[[2]uint32][]byte {
	t.Helper()
	le := binary.LittleEndian
	got := map[[2]uint32][]byte{}
	entries := func(at uint32) (ids, offsets []uint32) {
		t.Helper()
		if le.Uint32(section[at:]) != 0 || le.Uint32(section[at+4:]) != 0 || le.Uint32(section[at+8:]) != 0 {
			t.Errorf("directory at %d: characteristics, time stamp or version is not zero: % x", at, section[at:at+12])
		}
		named, byID := le.Uint16(section[at+12:]), le.Uint16(section[at+14:])
		if named != 0 {
			t.Fatalf("directory at %d has named entries", at)
		}
		for i := uint32(0); i < uint32(byID); i++ {
			id := le.Uint32(section[at+16+8*i:])
			if len(ids) > 0 && id <= ids[len(ids)-1] {
				t.Errorf("directory at %d: id %d after id %d, not ascending", at, id, ids[len(ids)-1])
			}
			ids = append(ids, id)
			offsets = append(offsets, le.Uint32(section[at+20+8*i:]))
		}
		return
	}
	types, typeOffsets := entries(0)
	for i, typ := range types {
		names, nameOffsets := entries(typeOffsets[i] &^ 0x80000000)
		for j, name := range names {
			_, langOffsets := entries(nameOffsets[j] &^ 0x80000000)
			entry := langOffsets[0]
			if entry&0x80000000 != 0 {
				t.Fatalf("type %d name %d: a fourth directory level", typ, name)
			}
			rva, size := le.Uint32(section[entry:]), le.Uint32(section[entry+4:])
			got[[2]uint32{typ, name}] = section[rva-base : rva-base+size]
		}
	}
	return got
}

// versionStrings reads the strings and the fixed numbers back out of an
// RT_VERSION resource, checking each block's length on the way.
func versionStrings(t *testing.T, data []byte) (strs map[string]string, fixed [13]uint32) {
	t.Helper()
	le := binary.LittleEndian
	text := func(b []byte) string {
		u := make([]uint16, len(b)/2)
		for i := range u {
			u[i] = le.Uint16(b[2*i:])
		}
		return string(utf16.Decode(u))
	}
	if int(le.Uint16(data)) != len(data) {
		t.Errorf("VS_VERSIONINFO says %d bytes long, the resource has %d", le.Uint16(data), len(data))
	}
	key := "VS_VERSION_INFO\x00"
	if got := text(data[6 : 6+2*len(key)]); got != key {
		t.Fatalf("version key = %q", got)
	}
	at := (6 + 2*len(key) + 3) &^ 3
	if le.Uint16(data[2:]) != 52 {
		t.Fatalf("fixed info length = %d, want 52", le.Uint16(data[2:]))
	}
	for i := range fixed {
		fixed[i] = le.Uint32(data[at+4*i:])
	}
	at = (at + 52 + 3) &^ 3

	// StringFileInfo, then its one table "040904B0", then the strings.
	end := at + int(le.Uint16(data[at:]))
	if got := text(data[at+6 : at+6+30]); got != "StringFileInfo\x00" {
		t.Fatalf("first block = %q", got)
	}
	at = (at + 6 + 30 + 3) &^ 3
	tableEnd := at + int(le.Uint16(data[at:]))
	if got := text(data[at+6 : at+6+18]); got != "040904B0\x00" {
		t.Fatalf("table = %q", got)
	}
	at = (at + 6 + 18 + 3) &^ 3
	strs = map[string]string{}
	for at < tableEnd {
		length, valueChars := int(le.Uint16(data[at:])), int(le.Uint16(data[at+2:]))
		if le.Uint16(data[at+4:]) != 1 {
			t.Fatalf("string at %d is not text", at)
		}
		k := 6
		for le.Uint16(data[at+k:]) != 0 {
			k += 2
		}
		name := text(data[at+6 : at+k])
		k = (k + 2 + 3) &^ 3
		value := text(data[at+k : at+k+2*valueChars])
		if !strings.HasSuffix(value, "\x00") || at+k+2*valueChars != at+length {
			t.Errorf("%s: value length does not match the block length", name)
		}
		strs[name] = strings.TrimSuffix(value, "\x00")
		at = (at + length + 3) &^ 3
	}

	// VarFileInfo: Translation = language 0x0409, code page 1200.
	at = (end + 3) &^ 3
	if got := text(data[at+6 : at+6+24]); got != "VarFileInfo\x00" {
		t.Fatalf("last block = %q", got)
	}
	if at+int(le.Uint16(data[at:])) != len(data) {
		t.Errorf("VarFileInfo does not end with the resource")
	}
	if got := le.Uint32(data[len(data)-4:]); got != 0x04B00409 {
		t.Errorf("translation = %#x, want 0x04b00409", got)
	}
	return strs, fixed
}

func checkResources(t *testing.T, res map[[2]uint32][]byte, s spec) {
	t.Helper()
	if len(res) != 7 {
		t.Errorf("%d resources, want 7 (four icons, group icon, version, manifest)", len(res))
	}
	// RT_ICON ids 1 to 4 are the four sizes, each a PNG of that size, and the
	// 256 one is the source file as it is. The group lists them in that order:
	// reserved, icon, four images; then 14 bytes each: width, height (0 for
	// 256), no palette, reserved, one plane, 32 bits, size, id.
	group := []byte{0, 0, 1, 0, 4, 0}
	for i, size := range []int{16, 32, 48, 256} {
		data := res[[2]uint32{rtIcon, uint32(i + 1)}]
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != size || cfg.Height != size {
			t.Errorf("RT_ICON %d is %dx%d (%v), want a %d px PNG", i+1, cfg.Width, cfg.Height, err, size)
		}
		if size == 256 && !bytes.Equal(data, s.icon) {
			t.Errorf("the 256 px RT_ICON is not the source PNG's bytes")
		}
		side := byte(size & 0xff)
		entry := []byte{side, side, 0, 0, 1, 0, 32, 0, 0, 0, 0, 0, byte(i + 1), 0}
		binary.LittleEndian.PutUint32(entry[8:], uint32(len(data)))
		group = append(group, entry...)
	}
	if got := res[[2]uint32{rtGroupIcon, 1}]; !bytes.Equal(got, group) {
		t.Errorf("RT_GROUP_ICON = % x, want % x", got, group)
	}
	if got := string(res[[2]uint32{rtManifest, 1}]); got != wantManifest {
		t.Errorf("manifest differs from the expected text:\n%s", got)
	}
	strs, fixed := versionStrings(t, res[[2]uint32{rtVersion, 1}])
	wantStrs := map[string]string{
		"CompanyName":      "Sauzze",
		"FileDescription":  s.description,
		"FileVersion":      "1.24.0.28",
		"InternalName":     s.internal,
		"LegalCopyright":   "Copyright (c) 2026 Sauzze",
		"OriginalFilename": s.original,
		"ProductName":      "Smolcorp",
		"ProductVersion":   "1.24.0.28",
	}
	if len(strs) != len(wantStrs) {
		t.Errorf("version strings = %v", strs)
	}
	for k, v := range wantStrs {
		if strs[k] != v {
			t.Errorf("%s = %q, want %q", k, strs[k], v)
		}
	}
	// All 13 words: signature, structure version, file version 1.24.0.28 and
	// product version 1.24.0.28, flags mask and flags (none), OS (Windows NT,
	// 32 bits), file type (an application), no subtype, no date.
	wantFixed := [13]uint32{
		0xFEEF04BD, 0x00010000, 1<<16 | 24, 0<<16 | 28, 1<<16 | 24, 0<<16 | 28,
		0x3F, 0, 0x00040004, 1, 0, 0, 0,
	}
	if fixed != wantFixed {
		t.Errorf("fixed info = %#x, want %#x", fixed, wantFixed)
	}
}

func TestEachArchGetsItsMachineTypeAndAllItsResources(t *testing.T) {
	for _, c := range []struct {
		arch    string
		machine uint16
		reloc   uint16
	}{
		{"amd64", pe.IMAGE_FILE_MACHINE_AMD64, 3}, // IMAGE_REL_AMD64_ADDR32NB
		{"arm64", pe.IMAGE_FILE_MACHINE_ARM64, 2}, // IMAGE_REL_ARM64_ADDR32NB
	} {
		t.Run(c.arch, func(t *testing.T) {
			s := testSpec(t, c.arch)
			data, err := generate(s)
			if err != nil {
				t.Fatal(err)
			}
			f, err := pe.NewFile(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if f.Machine != c.machine || f.TimeDateStamp != 0 {
				t.Errorf("machine %#x, time %d; want %#x, 0", f.Machine, f.TimeDateStamp, c.machine)
			}
			if len(f.Sections) != 1 || f.Sections[0].Name != ".rsrc" {
				t.Fatalf("sections = %v, want just .rsrc", f.Sections)
			}
			sec := f.Sections[0]
			if len(f.Symbols) != 1 || f.Symbols[0].Name != ".rsrc" {
				t.Errorf("symbols = %v", f.Symbols)
			}
			if len(sec.Relocs) != 7 {
				t.Fatalf("%d relocations, want one per resource", len(sec.Relocs))
			}
			section, err := sec.Data()
			if err != nil {
				t.Fatal(err)
			}
			// Each relocation sits on a data entry's address field, which holds
			// the data's place in the section, to be made an address by the linker.
			for _, r := range sec.Relocs {
				if r.Type != c.reloc || r.SymbolTableIndex != 0 {
					t.Errorf("relocation %+v, want type %d on symbol 0", r, c.reloc)
				}
				if rva := binary.LittleEndian.Uint32(section[r.VirtualAddress:]); rva < r.VirtualAddress || int(rva) >= len(section) {
					t.Errorf("relocation at %d: data at %d is outside the section", r.VirtualAddress, rva)
				}
			}
			checkResources(t, readResources(t, section, 0), s)
		})
	}
}

func TestTwoRunsGiveIdenticalBytes(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		a, err := generate(testSpec(t, arch))
		if err != nil {
			t.Fatal(err)
		}
		b, _ := generate(testSpec(t, arch))
		if !bytes.Equal(a, b) {
			t.Errorf("%s: two runs differ", arch)
		}
	}
	a, _ := generate(testSpec(t, "amd64"))
	b, _ := generate(testSpec(t, "arm64"))
	if bytes.Equal(a, b) {
		t.Error("amd64 and arm64 files are the same")
	}
}

func TestVersionNumbersAndLicenseAreRead(t *testing.T) {
	v, err := parseVersion("1.24.0", "28")
	if err != nil || v != [4]uint16{1, 24, 0, 28} {
		t.Errorf("parseVersion = %v, %v", v, err)
	}
	for _, bad := range [][2]string{{"", "28"}, {"1.24.0.1", "28"}, {"1.x.0", "28"}, {"1.24.70000", "28"}, {"1.24.0", "-1"}, {"1.24.0", ""}} {
		if _, err := parseVersion(bad[0], bad[1]); err == nil {
			t.Errorf("parseVersion(%q, %q) was accepted", bad[0], bad[1])
		}
	}
	licence, err := os.ReadFile("../../LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := legalCopyright(string(licence)); err != nil || !strings.HasPrefix(got, "Copyright (c) ") || !strings.HasSuffix(got, "Sauzze") {
		t.Errorf("legalCopyright = %q, %v", got, err)
	}
	if _, err := legalCopyright("MIT License\n"); err == nil {
		t.Error("a license with no Copyright line was accepted")
	}
}

func TestBadInputIsRefused(t *testing.T) {
	s := testSpec(t, "x86")
	if _, err := generate(s); err == nil {
		t.Error("arch x86 was accepted")
	}
	s = testSpec(t, "amd64")
	s.icon = []byte("not a png at all, but long enough to look at")
	if _, err := generate(s); err == nil {
		t.Error("a non-PNG icon was accepted")
	}
}

// The linker must take the file: build a throwaway program for Windows with
// it in a temporary folder, then read the resources back out of the .exe.
func TestTheGoLinkerTakesTheFile(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a program")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go tool on the path")
	}
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			dir := t.TempDir()
			s := testSpec(t, arch)
			syso, err := generate(s)
			if err != nil {
				t.Fatal(err)
			}
			for name, body := range map[string][]byte{
				"go.mod":                         []byte("module throwaway\n\ngo 1.21\n"),
				"main.go":                        []byte("package main\n\nfunc main() {}\n"),
				"rsrc_windows_" + arch + ".syso": syso,
			} {
				if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			exe := filepath.Join(dir, "throwaway.exe")
			cmd := exec.Command(goTool, "build", "-o", exe, ".")
			cmd.Dir = dir
			// A module of its own, so no -mod=vendor (it has no vendor folder), but
			// the owner's limit of two builds at once.
			cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH="+arch, "CGO_ENABLED=0", "GOFLAGS=-p=2", "GOPROXY=off", "GOTOOLCHAIN=local")
			// The memory rules: the nested build must run with -p=2 too.
			flags := exec.Command(goTool, "env", "GOFLAGS")
			flags.Dir, flags.Env = dir, cmd.Env
			if out, err := flags.Output(); err != nil || strings.TrimSpace(string(out)) != "-p=2" {
				t.Fatalf("the nested build runs with GOFLAGS %q (%v), want -p=2", out, err)
			}
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("go build: %v\n%s", err, out)
			}
			f, err := pe.Open(exe)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			var dd pe.DataDirectory
			switch h := f.OptionalHeader.(type) {
			case *pe.OptionalHeader64:
				dd = h.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_RESOURCE]
			default:
				t.Fatalf("optional header %T", h)
			}
			for _, sec := range f.Sections {
				if sec.VirtualAddress <= dd.VirtualAddress && dd.VirtualAddress < sec.VirtualAddress+sec.VirtualSize {
					data, err := sec.Data()
					if err != nil {
						t.Fatal(err)
					}
					start := dd.VirtualAddress - sec.VirtualAddress
					checkResources(t, readResources(t, data[start:], sec.VirtualAddress+start), s)
					return
				}
			}
			t.Fatal("no section holds the resource directory")
		})
	}
}

// run is what the flags ask for: files in, a .syso out, with each flag's value
// in its own place.
func TestRunWritesWhatTheFlagsAsk(t *testing.T) {
	dir := t.TempDir()
	license := filepath.Join(dir, "LICENSE")
	if err := os.WriteFile(license, []byte("MIT License\n\nCopyright (c) 2026 Sauzze\n\nPermission is hereby granted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		out := filepath.Join(dir, "rsrc_windows_"+arch+".syso")
		if err := run(arch, "1.24.0", "28", "Smolcorp command line", "smolcorp-cli.exe", "smolcorp-cli", iconPath, license, out); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		f, err := pe.NewFile(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		section, err := f.Sections[0].Data()
		if err != nil {
			t.Fatal(err)
		}
		checkResources(t, readResources(t, section, 0), testSpec(t, arch))
		// The same bytes as generate gives for the same spec.
		if want, _ := generate(testSpec(t, arch)); !bytes.Equal(data, want) {
			t.Errorf("%s: run wrote other bytes than generate", arch)
		}
	}
	for _, missing := range []int{3, 4, 5, 8} { // description, original, internal, out
		args := []string{"amd64", "1.24.0", "28", "d", "o.exe", "internal", iconPath, license, filepath.Join(dir, "x.syso")}
		args[missing] = ""
		if err := run(args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8]); err == nil {
			t.Errorf("run accepted an empty argument %d", missing)
		}
	}
}

// Windows looks resources up by binary search, so whatever order they are
// given in, the directory must list them in ascending order.
func TestResourcesAreListedInAscendingOrder(t *testing.T) {
	section, relocs := resourceSection([]resource{
		{rtManifest, 1, []byte("m")}, {rtIcon, 4, []byte("i4")}, {rtVersion, 1, []byte("v")},
		{rtIcon, 2, []byte("i2")}, {rtGroupIcon, 1, []byte("g")}, {rtIcon, 3, []byte("i3")}, {rtIcon, 1, []byte("i1")},
	})
	if len(relocs) != 7 {
		t.Errorf("%d relocations, want 7", len(relocs))
	}
	got := readResources(t, section, 0)
	for key, want := range map[[2]uint32]string{
		{rtIcon, 1}: "i1", {rtIcon, 2}: "i2", {rtIcon, 3}: "i3", {rtIcon, 4}: "i4",
		{rtGroupIcon, 1}: "g", {rtVersion, 1}: "v", {rtManifest, 1}: "m",
	} {
		if string(got[key]) != want {
			t.Errorf("resource %v = %q, want %q", key, got[key], want)
		}
	}
}

// A hand-made source: opaque white left of column 100, nothing right of it.
func halfImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 100; x++ {
			img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 255})
		}
	}
	return img
}

func TestDownscaleAveragesAreaAndWeighsColorByOpacity(t *testing.T) {
	for _, size := range []int{16, 32, 48} {
		out := downscale(halfImage(), size)
		if out.Bounds().Dx() != size || out.Bounds().Dy() != size {
			t.Fatalf("size %d: got %v", size, out.Bounds())
		}
		// New pixel x covers source columns x*256/size up to (x+1)*256/size.
		for x := 0; x < size; x++ {
			lo, hi := float64(x)*256/float64(size), float64(x+1)*256/float64(size)
			covered := (min(hi, 100) - min(lo, 100)) / (hi - lo)
			got := out.NRGBAAt(x, size/2)
			if d := float64(got.A) - 255*covered; d < -1 || d > 1 {
				t.Errorf("size %d, column %d: alpha %d, want about %.1f", size, x, got.A, 255*covered)
			}
			// Transparent source pixels must not darken the color.
			if got.A > 0 && (got.R != 255 || got.G != 255 || got.B != 255) {
				t.Errorf("size %d, column %d: color %v, want white", size, x, got)
			}
			if got.A == 0 && got != (color.NRGBA{}) {
				t.Errorf("size %d, column %d: %v, want nothing", size, x, got)
			}
		}
	}
	// Sixteen source pixels per new pixel: column 6 holds source columns 96 to 111, four of them white.
	if got := downscale(halfImage(), 16).NRGBAAt(6, 0); got.A != 64 {
		t.Errorf("column 6 alpha = %d, want 64", got.A)
	}
}

func TestOnlyA256PixelSourceIsTaken(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 128, 128))); err != nil {
		t.Fatal(err)
	}
	s := testSpec(t, "amd64")
	s.icon = b.Bytes()
	if _, err := generate(s); err == nil {
		t.Error("a 128 px icon was accepted")
	}
}
