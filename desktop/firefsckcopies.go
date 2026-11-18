package desktop

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"container/heap"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Every stored copy of a name (RT126FIREFSCK's B1). A repo may keep more than one copy of an object: loose and
// packed, or in two packs. That is harmless in itself (a repack in progress, a fetch that brought back what was
// already here), but git reads only one of them, and which one changes: the walk hashes the copy git reads now, and
// a later `git gc`, `repack` or `prune-packed` can drop it and serve another. So a desk could put a good loose copy
// in front of a rewritten packed one, pass the look, and have the owner's next maintenance give main its bytes.
//
// So the look lists every stored copy, as git would find one: the loose files and every pack's own index (.idx), in
// the repo's object folder and in each other store git names for it (`count-objects -v`), and hashes again, in Go,
// every copy of each name that has more than one and that the walk reached. One that doesn't hold its name, or that
// can't be read, fails the look. Checking each copy, not refusing any second copy, keeps the harmless ones working
// (SIERRA, this round); a name with one copy is the one git read and the walk hashed.
//
// A link anywhere the listing walks fails the look closed (RT126FIREFSCKR2's B1′): a fan-out folder (objects/xx) or a
// loose file, the pack folder, a pack or its index, and another store an info/alternates file names. Git never makes
// one there, yet it reads through each, and a desk's link would put a copy where the listing doesn't see it, or change
// what it points at once the look is done.

// storeCopy is one stored copy of a name: a loose file, or an entry of a pack at its offset.
type storeCopy struct {
	file string     // the loose file, or the pack
	pack *storePack // nil for a loose file
	at   uint64
}

// storePack is a pack and its own index, as git reads it with no multi-pack index. Its file and readers are opened
// once, at its first copy, and kept until closeStoreCopies.
type storePack struct {
	file  string // the .pack
	index storeIndex
	cache map[uint64]packedObject
	held  int // the bytes in cache
	f     *os.File
	br    *bufio.Reader
	z     io.ReadCloser
}

// closeStoreCopies closes the packs twice's copies opened.
func closeStoreCopies(twice map[string][]storeCopy) {
	for _, copies := range twice {
		for _, c := range copies {
			if c.pack != nil && c.pack.f != nil {
				c.pack.f.Close()
				c.pack.f = nil
			}
		}
	}
}

type packedObject struct {
	kind string
	data []byte
}

// storeIndex is a pack's .idx: its names in git's order (hs bytes each) and where each one's entry starts.
type storeIndex struct {
	hs    int
	names []byte
	place func(i int) (uint64, bool)
}

func (x storeIndex) n() int            { return len(x.names) / x.hs }
func (x storeIndex) name(i int) []byte { return x.names[i*x.hs : (i+1)*x.hs] }
func (x storeIndex) find(name []byte) int {
	i := sort.Search(x.n(), func(i int) bool { return bytes.Compare(x.name(i), name) >= 0 })
	if i < x.n() && bytes.Equal(x.name(i), name) {
		return i
	}
	return -1
}

// storeObjectFolders are the object folders git reads repo's objects from: its own, then each other store, as git
// itself names them.
func storeObjectFolders(read func(args ...string) (string, int, error), repo string) ([]string, error) {
	in := func(path string) string {
		if !filepath.IsAbs(path) {
			path = filepath.Join(repo, path)
		}
		return filepath.Clean(path)
	}
	own, status, err := read("rev-parse", "--git-path", "objects")
	if err != nil || status != 0 || own == "" {
		return nil, errors.New("git couldn't say where its objects are")
	}
	folders := []string{in(own)}
	said, status, err := read("count-objects", "-v")
	if err != nil || status != 0 {
		return nil, errors.New("git couldn't say which other stores it reads")
	}
	for _, line := range strings.Split(said, "\n") {
		path, ok := strings.CutPrefix(line, "alternate: ")
		if !ok {
			continue
		}
		if strings.HasPrefix(path, `"`) { // git quotes a path with unusual characters, as C does
			if path, err = strconv.Unquote(path); err != nil {
				return nil, errors.New("git named another store in words Smolcorp can't read: " + line)
			}
		}
		folders = append(folders, in(path))
	}
	return folders, nil
}

// storeSecondCopies lists every stored copy in folders and keeps the names that have more than one, by their bytes.
// hs is the repo's name size: 20 for SHA-1, 32 for SHA-256.
func storeSecondCopies(ctx context.Context, folders []string, hs int) (map[string][]storeCopy, error) {
	var lists []storeList
	for _, folder := range folders {
		if err := alternatesNoLinks(folder); err != nil {
			return nil, err
		}
		loose, err := looseNames(folder, hs)
		if err != nil {
			return nil, err
		}
		lists = append(lists, loose)
		if info, err := os.Lstat(filepath.Join(folder, "pack")); err == nil && aLink(info.Mode()) {
			return nil, storeLink("the pack folder", filepath.Join(folder, "pack"), "")
		}
		packs, err := os.ReadDir(filepath.Join(folder, "pack"))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("couldn't list the packs in %s: %v", folder, err)
		}
		for _, e := range packs {
			if !strings.HasSuffix(e.Name(), ".idx") || e.IsDir() {
				continue
			}
			idx := filepath.Join(folder, "pack", e.Name())
			if aLink(e.Type()) {
				return nil, storeLink("the pack index", idx, "")
			}
			pack := strings.TrimSuffix(idx, ".idx") + ".pack"
			if info, err := os.Lstat(pack); err != nil {
				continue // git uses no index without its pack
			} else if aLink(info.Mode()) {
				return nil, storeLink("the pack", pack, "")
			}
			index, err := readStoreIndex(idx, hs)
			if err != nil {
				return nil, fmt.Errorf("couldn't read the pack index %s: %v", idx, err)
			}
			p := &storePack{file: pack, index: index}
			lists = append(lists, storeList{names: index.names, hs: hs, copy: func(i int) storeCopy {
				at, _ := index.place(i)
				return storeCopy{file: pack, pack: p, at: at}
			}})
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return sameNames(lists, hs), nil
}

// aLink is a link, or on Windows a junction (Go's "irregular" file).
func aLink(mode fs.FileMode) bool { return mode&(fs.ModeSymlink|fs.ModeIrregular) != 0 }

// storeLink is the look's words for what, at path (named, if not "", says where it's named), that is a link: "the
// object folder <path> is a link to <where>, which git never makes".
func storeLink(what, path, named string) error {
	said := what + " " + path + named + " is a link"
	if to, err := os.Readlink(path); err == nil {
		said += " to " + to
	}
	return errors.New(said + ", which git never makes")
}

// alternatesNoLinks fails when a store folder's info/alternates names one through a link. git count-objects names
// each store by its real path, so its own words can't show one: the file is read here, as git reads it (a line
// each, "#" for a comment, a path in quotes as C writes it, a relative one from folder).
func alternatesNoLinks(folder string) error {
	file := filepath.Join(folder, "info", "alternates")
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("couldn't read %s: %v", file, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		path := line
		if path == "" || strings.HasPrefix(path, "#") {
			continue
		}
		if strings.HasPrefix(path, `"`) {
			if path, err = strconv.Unquote(path); err != nil {
				return errors.New(file + " names another store in words Smolcorp can't read: " + line)
			}
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(folder, path)
		}
		path = filepath.Clean(path)
		if info, err := os.Lstat(path); err == nil && aLink(info.Mode()) {
			return storeLink("the other store", path, ", named in "+file+",")
		}
	}
	return nil
}

// storeList is one place's names, in git's order (several may be the same), and the copy each one is.
type storeList struct {
	names []byte
	hs    int
	copy  func(i int) storeCopy
}

// looseNames are the loose objects in folder (objects/xx/<the rest>), in git's order. Any case of hex is taken, as a
// Mac's or a Windows computer's folders give git a file whatever the case of its name.
func looseNames(folder string, hs int) (storeList, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return storeList{}, fmt.Errorf("couldn't list the object folder %s: %v", folder, err)
	}
	type loose struct {
		name []byte
		file string
	}
	var found []loose
	for _, e := range entries {
		if len(e.Name()) != 2 {
			continue
		}
		if _, err := hex.DecodeString(e.Name()); err != nil {
			continue
		}
		if aLink(e.Type()) {
			return storeList{}, storeLink("the object folder", filepath.Join(folder, e.Name()), "")
		}
		if !e.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(folder, e.Name()))
		if err != nil {
			return storeList{}, fmt.Errorf("couldn't list the object folder %s: %v", filepath.Join(folder, e.Name()), err)
		}
		for _, f := range files {
			name, err := hex.DecodeString(e.Name() + f.Name())
			if err != nil || len(name) != hs || f.IsDir() {
				continue
			}
			if aLink(f.Type()) {
				return storeList{}, storeLink("the loose object", filepath.Join(folder, e.Name(), f.Name()), "")
			}
			found = append(found, loose{name, filepath.Join(folder, e.Name(), f.Name())})
		}
	}
	sort.Slice(found, func(i, j int) bool { return bytes.Compare(found[i].name, found[j].name) < 0 })
	names := make([]byte, 0, len(found)*hs)
	for _, l := range found {
		names = append(names, l.name...)
	}
	return storeList{names: names, hs: hs, copy: func(i int) storeCopy { return storeCopy{file: found[i].file} }}, nil
}

// readStoreIndex reads a pack's .idx (version 1 or 2), as git checks it before use: its size, its fan-out and its
// names in order.
func readStoreIndex(file string, hs int) (storeIndex, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return storeIndex{}, err
	}
	bad := func(why string) (storeIndex, error) { return storeIndex{}, errors.New(why) }
	be32 := func(at int) uint32 { return binary.BigEndian.Uint32(data[at:]) }
	fanAt, v2 := 0, len(data) >= 8 && bytes.Equal(data[:4], []byte{0xff, 't', 'O', 'c'})
	if v2 {
		if be32(4) != 2 {
			return bad("a version git doesn't write")
		}
		fanAt = 8
	}
	if len(data) < fanAt+256*4 {
		return bad("it is cut short")
	}
	for i := 1; i < 256; i++ {
		if be32(fanAt+4*i) < be32(fanAt+4*(i-1)) {
			return bad("its fan-out goes down")
		}
	}
	n := int(be32(fanAt + 255*4))
	x := storeIndex{hs: hs}
	if !v2 {
		if len(data) != 256*4+n*(4+hs)+2*hs {
			return bad("its size isn't what its names make")
		}
		x.names = make([]byte, 0, n*hs)
		for i := 0; i < n; i++ {
			at := 256*4 + i*(4+hs)
			x.names = append(x.names, data[at+4:at+4+hs]...)
		}
		x.place = func(i int) (uint64, bool) { return uint64(be32(256*4 + i*(4+hs))), true }
	} else {
		names := fanAt + 256*4
		small := names + n*(hs+4)
		large := small + n*4
		if len(data) < large+2*hs {
			return bad("its size isn't what its names make")
		}
		x.names = data[names : names+n*hs]
		x.place = func(i int) (uint64, bool) {
			at := be32(small + 4*i)
			if at&0x80000000 == 0 {
				return uint64(at), true
			}
			j := large + 8*int(at&0x7fffffff)
			if j+8 > len(data)-2*hs {
				return 0, false
			}
			return binary.BigEndian.Uint64(data[j:]), true
		}
	}
	for i := 0; i < n; i++ {
		if i > 0 && bytes.Compare(x.name(i-1), x.name(i)) >= 0 {
			// A name twice in one .idx (F1): git, which never writes one, could read either entry, and Go another.
			return bad("its names aren't in git's order, or one is there twice")
		}
		if _, ok := x.place(i); !ok {
			return bad("an offset points past its end")
		}
		if int(be32(fanAt+4*int(x.name(i)[0]))) <= i || x.name(i)[0] > 0 && int(be32(fanAt+4*(int(x.name(i)[0])-1))) > i {
			return bad("its fan-out doesn't match its names")
		}
	}
	return x, nil
}

// sameNames merges lists, each in git's order, and keeps every name found more than once, with all its copies.
func sameNames(lists []storeList, hs int) map[string][]storeCopy {
	h := &nameHeap{}
	for i, l := range lists {
		if len(l.names) > 0 {
			*h = append(*h, nameAt{list: lists[i], i: 0})
		}
	}
	heap.Init(h)
	twice := map[string][]storeCopy{}
	var last []byte
	var copies []storeCopy
	done := func() {
		if len(copies) > 1 {
			twice[string(last)] = copies
		}
	}
	for h.Len() > 0 {
		top := (*h)[0]
		name := top.list.names[top.i*hs : (top.i+1)*hs]
		if !bytes.Equal(name, last) {
			done()
			last, copies = name, nil
		}
		copies = append(copies, top.list.copy(top.i))
		if (*h)[0].i++; (*h)[0].i*hs >= len(top.list.names) {
			heap.Pop(h)
		} else {
			heap.Fix(h, 0)
		}
	}
	done()
	return twice
}

type nameAt struct {
	list storeList
	i    int
}

type nameHeap []nameAt

func (h nameHeap) Len() int { return len(h) }
func (h nameHeap) Less(a, b int) bool {
	hs := h[a].list.hs
	return bytes.Compare(h[a].list.names[h[a].i*hs:(h[a].i+1)*hs], h[b].list.names[h[b].i*hs:(h[b].i+1)*hs]) < 0
}
func (h nameHeap) Swap(a, b int) { h[a], h[b] = h[b], h[a] }
func (h *nameHeap) Push(x any)   { *h = append(*h, x.(nameAt)) }
func (h *nameHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// storeObjectLimit is the largest copy the look reads into memory (a delta's base or result): past it, the copy
// can't be checked here, and the look fails closed.
const storeObjectLimit = 1 << 30

// holds hashes c again and says whether it is the object name names; err when it can't be read.
func (c storeCopy) holds(ctx context.Context, name []byte) (bool, error) {
	sum := newObjectHash(len(name))
	if sum == nil {
		return false, errors.New("a name of an unknown size")
	}
	if c.pack == nil {
		f, err := os.Open(c.file)
		if err != nil {
			return false, err
		}
		defer f.Close()
		z, err := zlib.NewReader(bufio.NewReader(f))
		if err != nil {
			return false, err
		}
		// Its size, as its header says it to git ("blob 12\0"): past the limit, it is over 1 GB, as a packed one (F2).
		r := bufio.NewReader(ctxReader{ctx, z})
		head, _ := r.Peek(64)
		if _, size, ok := strings.Cut(string(head[:max(bytes.IndexByte(head, 0), 0)]), " "); ok {
			if n, err := strconv.ParseUint(size, 10, 64); err == nil && n > storeObjectLimit {
				return false, errors.New("it is over 1 GB")
			}
		}
		if _, err := io.Copy(sum, io.LimitReader(r, storeObjectLimit+64)); err != nil {
			return false, err
		}
		return bytes.Equal(sum.Sum(nil), name), nil
	}
	if c.pack.f == nil {
		f, err := os.Open(c.pack.file)
		if err != nil {
			return false, err
		}
		c.pack.f, c.pack.br = f, bufio.NewReader(f)
	}
	kind, data, err := c.pack.object(ctx, c.at, 0)
	if err != nil {
		return false, err
	}
	fmt.Fprintf(sum, "%s %d\x00", kind, len(data))
	sum.Write(data)
	return bytes.Equal(sum.Sum(nil), name), nil
}

func newObjectHash(size int) hash.Hash {
	switch size {
	case sha1.Size:
		return sha1.New()
	case sha256.Size:
		return sha256.New()
	}
	return nil
}

// ctxReader stops a long read once ctx is done.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

var packKinds = map[byte]string{1: "commit", 2: "tree", 3: "blob", 4: "tag"}

// object reads the entry at in p as git does: its kind and bytes, a delta put together from its base (another entry
// of this pack, by offset or by name). An entry is read whole before its base is, so one reader serves them all.
func (p *storePack) object(ctx context.Context, at uint64, depth int) (string, []byte, error) {
	if got, ok := p.cache[at]; ok {
		return got.kind, got.data, nil
	}
	if depth > 10000 {
		return "", nil, errors.New("its deltas go deeper than git's")
	}
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	r := p.br
	r.Reset(io.NewSectionReader(p.f, int64(at), 1<<62))
	b, err := r.ReadByte()
	if err != nil {
		return "", nil, err
	}
	kind, size, shift := b>>4&7, uint64(b&15), 4
	for b&0x80 != 0 {
		if b, err = r.ReadByte(); err != nil || shift > 57 {
			return "", nil, errors.New("an entry's header is broken")
		}
		size |= uint64(b&0x7f) << shift
		shift += 7
	}
	if size > storeObjectLimit {
		return "", nil, errors.New("it is over 1 GB")
	}
	var base uint64
	switch kind {
	case 6: // a delta on the entry so many bytes before it
		b, err := r.ReadByte()
		back := uint64(b & 0x7f)
		for err == nil && b&0x80 != 0 && back < 1<<56 {
			if b, err = r.ReadByte(); err == nil {
				back = (back+1)<<7 | uint64(b&0x7f)
			}
		}
		if err != nil || back == 0 || back >= at {
			return "", nil, errors.New("a delta's base is out of its pack")
		}
		base = at - back
	case 7: // a delta on the entry with that name, in the same pack
		name := make([]byte, p.index.hs)
		if _, err := io.ReadFull(r, name); err != nil {
			return "", nil, err
		}
		i := p.index.find(name)
		if i < 0 {
			return "", nil, errors.New("a delta's base isn't in its pack")
		}
		base, _ = p.index.place(i)
	default:
		if packKinds[kind] == "" {
			return "", nil, errors.New("an entry of a kind git doesn't write")
		}
	}
	if p.z == nil {
		p.z, err = zlib.NewReader(r)
	} else {
		err = p.z.(zlib.Resetter).Reset(r, nil)
	}
	if err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(io.LimitReader(ctxReader{ctx, p.z}, int64(size)+1))
	if err != nil {
		return "", nil, err
	}
	if uint64(len(data)) != size {
		return "", nil, errors.New("an entry holds another size than it says")
	}
	name := packKinds[kind]
	if kind == 6 || kind == 7 {
		var from []byte
		if name, from, err = p.object(ctx, base, depth+1); err != nil {
			return "", nil, err
		}
		if data, err = applyDelta(from, data); err != nil {
			return "", nil, err
		}
	}
	if p.cache == nil || p.held > 64<<20 {
		p.cache, p.held = map[uint64]packedObject{}, 0
	}
	p.cache[at] = packedObject{name, data}
	p.held += len(data)
	return name, data, nil
}

// applyDelta makes an object from its base and a delta, as git does: the sizes, then copies from the base and
// bytes given.
func applyDelta(base, delta []byte) ([]byte, error) {
	broken := errors.New("a delta is broken")
	size := func() (uint64, bool) {
		var n uint64
		for shift := 0; len(delta) > 0 && shift < 64; shift += 7 {
			b := delta[0]
			delta = delta[1:]
			n |= uint64(b&0x7f) << shift
			if b&0x80 == 0 {
				return n, true
			}
		}
		return 0, false
	}
	from, ok1 := size()
	to, ok2 := size()
	if !ok1 || !ok2 || from != uint64(len(base)) || to > storeObjectLimit {
		return nil, broken
	}
	out := make([]byte, 0, min(to, 64<<20))
	for len(delta) > 0 {
		op := delta[0]
		delta = delta[1:]
		switch {
		case op&0x80 != 0:
			var at, n uint64
			for i := 0; i < 7; i++ {
				if op&(1<<i) == 0 {
					continue
				}
				if len(delta) == 0 {
					return nil, broken
				}
				if i < 4 {
					at |= uint64(delta[0]) << (8 * i)
				} else {
					n |= uint64(delta[0]) << (8 * (i - 4))
				}
				delta = delta[1:]
			}
			if n == 0 {
				n = 0x10000
			}
			if at+n > uint64(len(base)) || uint64(len(out))+n > to {
				return nil, broken
			}
			out = append(out, base[at:at+n]...)
		case op != 0:
			if int(op) > len(delta) || uint64(len(out))+uint64(op) > to {
				return nil, broken
			}
			out = append(out, delta[:op]...)
			delta = delta[op:]
		default:
			return nil, broken
		}
	}
	if uint64(len(out)) != to {
		return nil, broken
	}
	return out, nil
}

// storeCopyPlace is where a copy is, for the owner: its file inside the repo, or in full when it is elsewhere.
func storeCopyPlace(repo string, c storeCopy) string {
	if rel, err := filepath.Rel(repo, c.file); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return c.file
}
