// Command mkwinres writes the Windows resources of a Smolcorp .exe (version
// info, icon and manifest) as a COFF object, a .syso file, which the Go
// linker links into any Windows program that has one beside its code:
//
//	go run ./scripts/mkwinres -arch amd64 -version 1.24.0 -build 28 \
//	    -description Smolcorp -original Smolcorp.exe -internal smolcorp \
//	    -icon packaging/linux/smolcorp.png -license LICENSE \
//	    -o cmd/smolcorp/rsrc_windows_amd64.syso
//
// scripts/release-desktop.sh runs it before each Windows build and deletes the
// result afterwards: the output is never committed. It uses only the standard
// library and writes no dates, so the same input gives the same bytes. The
// icon holds four sizes (16, 32, 48 and 256 pixels), all made from the 256-pixel
// PNG it is given.
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

const (
	companyName = "Sauzze"
	productName = "Smolcorp"

	// Resource types.
	rtIcon      = 3
	rtGroupIcon = 14
	rtVersion   = 16
	rtManifest  = 24

	langUS = 0x0409 // English (United States)

	// The icon is made at these sizes from a source of the largest.
	sourceSize = 256

	// The Windows 10 and 11 compatibility id.
	windows10 = "{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"
)

var iconSizes = []int{16, 32, 48, 256}

// manifest asks for no administrator rights, long paths, and per-monitor DPI
// awareness (version 2 where Windows has it, else version 1). Nothing else.
const manifest = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
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
      <supportedOS Id="` + windows10 + `"/>
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

// arches maps GOARCH to the COFF machine type and the relocation type that
// stores an address relative to the image base (ADDR32NB).
var arches = map[string]struct{ machine, reloc uint16 }{
	"amd64": {0x8664, 3},
	"arm64": {0xAA64, 2},
}

type spec struct {
	arch        string
	version     [4]uint16
	description string
	original    string
	internal    string
	copyright   string
	icon        []byte // a 256x256 PNG
}

func main() {
	arch := flag.String("arch", "", "amd64 or arm64")
	version := flag.String("version", "", "the version, like 1.24.0")
	build := flag.String("build", "0", "the build number, the fourth part of the file version")
	description := flag.String("description", "", "FileDescription")
	original := flag.String("original", "", "OriginalFilename, like Smolcorp.exe")
	internal := flag.String("internal", "", "InternalName, like smolcorp")
	iconPath := flag.String("icon", "", "the icon, a PNG file")
	licensePath := flag.String("license", "", "the LICENSE file, for LegalCopyright")
	out := flag.String("o", "", "the .syso file to write")
	flag.Parse()

	if err := run(*arch, *version, *build, *description, *original, *internal, *iconPath, *licensePath, *out); err != nil {
		fmt.Fprintln(os.Stderr, "mkwinres:", err)
		os.Exit(1)
	}
}

func run(arch, version, build, description, original, internal, iconPath, licensePath, out string) error {
	if description == "" || original == "" || internal == "" || out == "" {
		return errors.New("-description, -original, -internal and -o are required")
	}
	v, err := parseVersion(version, build)
	if err != nil {
		return err
	}
	icon, err := os.ReadFile(iconPath)
	if err != nil {
		return err
	}
	license, err := os.ReadFile(licensePath)
	if err != nil {
		return err
	}
	copyright, err := legalCopyright(string(license))
	if err != nil {
		return err
	}
	data, err := generate(spec{arch, v, description, original, internal, copyright, icon})
	if err != nil {
		return err
	}
	return os.WriteFile(out, data, 0o644)
}

// parseVersion turns "1.24.0" and "28" into 1, 24, 0, 28. Each part must fit
// in 16 bits, as a Windows file version does.
func parseVersion(version, build string) ([4]uint16, error) {
	var v [4]uint16
	parts := append(strings.Split(version, "."), build)
	if len(parts) < 2 || len(parts) > 4 {
		return v, fmt.Errorf("version %q: want one to three numbers separated by dots", version)
	}
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return v, fmt.Errorf("version part %q is not a number from 0 to 65535", p)
		}
		// The build always goes last, whatever number of parts came before.
		if i == len(parts)-1 {
			v[3] = uint16(n)
		} else {
			v[i] = uint16(n)
		}
	}
	return v, nil
}

func legalCopyright(license string) (string, error) {
	for _, line := range strings.Split(license, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "Copyright") {
			return line, nil
		}
	}
	return "", errors.New("no Copyright line in the license file")
}

// generate returns the whole .syso file.
func generate(s spec) ([]byte, error) {
	a, ok := arches[s.arch]
	if !ok {
		return nil, fmt.Errorf("arch %q: want amd64 or arm64", s.arch)
	}
	images, err := iconImages(s.icon)
	if err != nil {
		return nil, err
	}
	// One RT_ICON per size, ids 1 to 4, which the one group names.
	var res []resource
	for i, img := range images {
		res = append(res, resource{rtIcon, uint32(i + 1), img})
	}
	res = append(res,
		resource{rtGroupIcon, 1, groupIcon(images)},
		resource{rtVersion, 1, versionInfo(s)},
		resource{rtManifest, 1, []byte(manifest)},
	)
	rsrc, relocs := resourceSection(res)

	// COFF: file header, one section header, the section data, its
	// relocations, a one-entry symbol table (the section's own, which the
	// relocations point to) and an empty string table.
	const headerSize, sectionHeaderSize, relocSize = 20, 40, 10
	dataAt := headerSize + sectionHeaderSize
	relocAt := dataAt + len(rsrc)
	symbolAt := relocAt + relocSize*len(relocs)

	var b bytes.Buffer
	le := binary.LittleEndian
	w := func(v ...any) {
		for _, x := range v {
			_ = binary.Write(&b, le, x)
		}
	}
	w(a.machine, uint16(1), uint32(0), uint32(symbolAt), uint32(1), uint16(0), uint16(0))
	b.WriteString(".rsrc\x00\x00\x00")
	// Virtual size and address 0, then the sizes and places of its data and
	// relocations; flags: initialized data, readable.
	w(uint32(0), uint32(0), uint32(len(rsrc)), uint32(dataAt), uint32(relocAt), uint32(0),
		uint16(len(relocs)), uint16(0), uint32(0x40000040))
	b.Write(rsrc)
	for _, at := range relocs {
		w(uint32(at), uint32(0), a.reloc)
	}
	// Symbol ".rsrc": value 0, section 1, no type, static, no aux entries.
	b.WriteString(".rsrc\x00\x00\x00")
	w(uint32(0), uint16(1), uint16(0), uint8(3), uint8(0))
	w(uint32(4)) // string table: only its own size
	return b.Bytes(), nil
}

type resource struct {
	typ, name uint32
	data      []byte
}

// resourceSection lays out the resource directory tree (type, then name, then
// language, one language per resource) followed by the data entries and the
// data. Windows looks entries up by binary search, so each level is in
// ascending id order: the resources are sorted by type, then name. It also
// returns where each data entry's address field is, as the linker must turn
// each into an address in the image.
func resourceSection(res []resource) (section []byte, relocs []int) {
	res = append([]resource(nil), res...)
	sort.SliceStable(res, func(i, j int) bool {
		if res[i].typ != res[j].typ {
			return res[i].typ < res[j].typ
		}
		return res[i].name < res[j].name
	})
	n := len(res)
	const dirSize, entrySize, dataEntrySize = 16, 8, 16

	// The distinct types, each with its first resource and how many follow.
	type group struct {
		typ              uint32
		first, count, at int
	}
	var groups []group
	for i, r := range res {
		if i > 0 && res[i-1].typ == r.typ {
			groups[len(groups)-1].count++
		} else {
			groups = append(groups, group{typ: r.typ, first: i, count: 1})
		}
	}
	at := dirSize + len(groups)*entrySize // the root directory comes first
	for i := range groups {
		groups[i].at = at
		at += dirSize + groups[i].count*entrySize
	}
	langAt := func(i int) int { return at + i*(dirSize+entrySize) }
	entryAt := func(i int) int { return langAt(n) + i*dataEntrySize }
	dataAt := (entryAt(n) + 7) &^ 7

	var b bytes.Buffer
	le := binary.LittleEndian
	w := func(v ...any) {
		for _, x := range v {
			_ = binary.Write(&b, le, x)
		}
	}
	// A directory with `count` entries by id: no names, no flags, no date, no version.
	dir := func(count int) { w(uint32(0), uint32(0), uint16(0), uint16(0), uint16(0), uint16(count)) }
	const subdir = 0x80000000

	dir(len(groups))
	for _, g := range groups {
		w(g.typ, uint32(g.at)|subdir)
	}
	for _, g := range groups {
		dir(g.count)
		for i := g.first; i < g.first+g.count; i++ {
			w(res[i].name, uint32(langAt(i))|subdir)
		}
	}
	for i := range res {
		dir(1)
		w(uint32(langUS), uint32(entryAt(i)))
	}
	offset := dataAt
	for i, r := range res {
		relocs = append(relocs, entryAt(i))
		w(uint32(offset), uint32(len(r.data)), uint32(0), uint32(0))
		offset += (len(r.data) + 7) &^ 7
	}
	for b.Len() < dataAt {
		b.WriteByte(0)
	}
	for _, r := range res {
		b.Write(r.data)
		for b.Len()%8 != 0 {
			b.WriteByte(0)
		}
	}
	return b.Bytes(), relocs
}

// iconImages returns the PNG of each icon size, smallest first. The largest is
// the source file itself; the others are area-averaged down from it.
func iconImages(src []byte) ([][]byte, error) {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("the icon is not a PNG file: %v", err)
	}
	if b := img.Bounds(); b.Dx() != sourceSize || b.Dy() != sourceSize {
		return nil, fmt.Errorf("the icon is %dx%d: want %dx%d", b.Dx(), b.Dy(), sourceSize, sourceSize)
	}
	var out [][]byte
	for _, size := range iconSizes {
		if size == sourceSize {
			out = append(out, src)
			continue
		}
		var b bytes.Buffer
		if err := png.Encode(&b, downscale(img, size)); err != nil {
			return nil, err
		}
		out = append(out, b.Bytes())
	}
	return out, nil
}

// downscale shrinks a 256x256 image to size x size by area averaging: each new
// pixel is the mean of the source area it covers, weighted by how much of each
// source pixel it covers, and colors count in proportion to their opacity. It
// uses whole numbers only, so every machine gives the same pixels.
func downscale(src image.Image, size int) *image.NRGBA {
	const full = sourceSize
	b := src.Bounds()
	// New pixel i covers the source from i*full/size to (i+1)*full/size. Measured
	// in 1/size of a source pixel, that is from i*full to (i+1)*full, and source
	// pixel p covers from p*size to (p+1)*size.
	overlap := func(i, p int) int { return max(min((i+1)*full, (p+1)*size)-max(i*full, p*size), 0) }
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var weight, alpha, red, green, blue uint64
			for py := y * full / size; py <= ((y+1)*full-1)/size; py++ {
				for px := x * full / size; px <= ((x+1)*full-1)/size; px++ {
					w := uint64(overlap(y, py) * overlap(x, px))
					c := color.NRGBAModel.Convert(src.At(b.Min.X+px, b.Min.Y+py)).(color.NRGBA)
					wa := w * uint64(c.A)
					weight += w
					alpha += wa
					red += wa * uint64(c.R)
					green += wa * uint64(c.G)
					blue += wa * uint64(c.B)
				}
			}
			if alpha == 0 {
				continue
			}
			mean := func(sum uint64) uint8 { return uint8((sum + alpha/2) / alpha) }
			out.SetNRGBA(x, y, color.NRGBA{mean(red), mean(green), mean(blue), uint8((alpha + weight/2) / weight)})
		}
	}
	return out
}

// groupIcon is the RT_GROUP_ICON that names the RT_ICON images, ids 1 up, in
// the order given: each is a PNG, which Windows accepts in icon resources.
func groupIcon(images [][]byte) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	_ = binary.Write(&b, le, []uint16{0, 1, uint16(len(images))}) // reserved, icon, count
	for i, png := range images {
		size := iconSizes[i]
		// A size of 256 is written as 0; no palette; one plane, 32 bits.
		b.Write([]byte{byte(size & 0xff), byte(size & 0xff), 0, 0})
		_ = binary.Write(&b, le, []uint16{1, 32})
		_ = binary.Write(&b, le, uint32(len(png)))
		_ = binary.Write(&b, le, uint16(i+1)) // the RT_ICON id
	}
	return b.Bytes()
}

// versionInfo is the RT_VERSION resource: a VS_VERSIONINFO with the fixed
// numbers, the strings, and the language they are in.
func versionInfo(s spec) []byte {
	v := s.version
	text := fmt.Sprintf("%d.%d.%d.%d", v[0], v[1], v[2], v[3])
	le := binary.LittleEndian

	var fixed bytes.Buffer
	for _, x := range []uint32{
		0xFEEF04BD, 0x00010000, // signature, structure version
		uint32(v[0])<<16 | uint32(v[1]), uint32(v[2])<<16 | uint32(v[3]), // file version
		uint32(v[0])<<16 | uint32(v[1]), uint32(v[2])<<16 | uint32(v[3]), // product version
		0x3F, 0, // flags mask, flags
		0x00040004, // Windows NT, 32 bits
		1, 0,       // an application
		0, 0, // no date
	} {
		_ = binary.Write(&fixed, le, x)
	}

	var strs [][]byte
	for _, kv := range [][2]string{
		{"CompanyName", companyName},
		{"FileDescription", s.description},
		{"FileVersion", text},
		{"InternalName", s.internal},
		{"LegalCopyright", s.copyright},
		{"OriginalFilename", s.original},
		{"ProductName", productName},
		{"ProductVersion", text},
	} {
		value := utf16z(kv[1])
		strs = append(strs, node(kv[0], len(value)/2, 1, value))
	}
	// Language 0x0409, code page 1200 (Unicode).
	table := node("040904B0", 0, 1, nil, strs...)
	var translation bytes.Buffer
	_ = binary.Write(&translation, le, uint32(1200)<<16|langUS)
	vars := node("VarFileInfo", 0, 1, nil, node("Translation", 4, 0, translation.Bytes()))
	return node("VS_VERSION_INFO", fixed.Len(), 0, fixed.Bytes(), node("StringFileInfo", 0, 1, nil, table), vars)
}

// node is one block of a version resource: its length, the length of its
// value (in characters for text), text or binary, its key, then its value and
// its child blocks, each starting on a 4-byte boundary.
func node(key string, valueLen int, typ uint16, value []byte, children ...[]byte) []byte {
	var b bytes.Buffer
	pad := func() {
		for b.Len()%4 != 0 {
			b.WriteByte(0)
		}
	}
	b.Write(make([]byte, 6))
	b.Write(utf16z(key))
	pad()
	b.Write(value)
	for _, c := range children {
		pad()
		b.Write(c)
	}
	out := b.Bytes()
	binary.LittleEndian.PutUint16(out[0:], uint16(len(out)))
	binary.LittleEndian.PutUint16(out[2:], uint16(valueLen))
	binary.LittleEndian.PutUint16(out[4:], typ)
	return out
}

// utf16z is s in UTF-16, little-endian, with its end mark.
func utf16z(s string) []byte {
	var b []byte
	for _, u := range append(utf16.Encode([]rune(s)), 0) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return b
}
