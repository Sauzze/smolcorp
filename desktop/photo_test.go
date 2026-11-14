package desktop

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Send a photo (1.26 PH1): only generated pictures, no real photo, camera or terminal. The desk is windowDeck's s1,
// a Claude Code session in a fake launcher's window.

// testPicture is a small generated picture with a gradient, so an encoder has something to keep.
func testPicture() image.Image {
	picture := image.NewRGBA(image.Rect(0, 0, 24, 16))
	for y := range 16 {
		for x := range 24 {
			picture.Set(x, y, color.RGBA{uint8(x * 10), uint8(y * 15), 90, 255})
		}
	}
	return picture
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := jpeg.Encode(&out, testPicture(), nil); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, testPicture()); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// pngChunk is a PNG chunk with its length and checksum.
func pngChunk(kind string, data []byte) []byte {
	var chunk bytes.Buffer
	_ = binary.Write(&chunk, binary.BigEndian, uint32(len(data)))
	chunk.WriteString(kind)
	chunk.Write(data)
	_ = binary.Write(&chunk, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(kind), data...)))
	return chunk.Bytes()
}

// withPNGSize is a PNG whose header says it is width by height, its pixels unchanged: a bomb's header.
func withPNGSize(data []byte, width, height uint32) []byte {
	out := bytes.Clone(data)
	binary.BigEndian.PutUint32(out[16:], width)
	binary.BigEndian.PutUint32(out[20:], height)
	binary.BigEndian.PutUint32(out[29:], crc32.ChecksumIEEE(out[12:29]))
	return out
}

// withJPEGSize is a JPEG whose frame header says it is width by height.
func withJPEGSize(t *testing.T, data []byte, width, height uint16) []byte {
	t.Helper()
	out := bytes.Clone(data)
	at := bytes.Index(out, []byte{0xFF, 0xC0})
	if at < 0 {
		t.Fatal("no frame header")
	}
	binary.BigEndian.PutUint16(out[at+5:], height)
	binary.BigEndian.PutUint16(out[at+7:], width)
	return out
}

// sendTestPhoto sends data as the window, straight to the deck's code, and says what it answered.
func sendTestPhoto(d *Deck, from textFrom, kind string, body io.Reader) (int, string) {
	r := httptest.NewRequest(http.MethodPost, "/talk/photo?session=s1", body)
	r.Header.Set("Content-Type", kind)
	return d.sendPhoto(from, &request{Request: r}, "s1")
}

// photosOf are the files in session's photo folder, which the deck knows: the photos folder can't be listed.
func photosOf(t *testing.T, d *Deck, session string) []string {
	t.Helper()
	d.mu.Lock()
	folder := d.photoDesks[session]
	d.mu.Unlock()
	if folder == "" {
		return nil
	}
	names, _ := filepath.Glob(filepath.Join(supportDir, photosFolder, folder, "*"))
	return names
}

// listsAsYou says whether a folder that can't be listed stops this test: not when it runs as root, which reads
// everything.
func listsAsYou() bool { return os.Geteuid() != 0 }

// lookAtLine is the path the fake launcher was given, from its paste.
func lookAtLine(t *testing.T, f *fakeLauncher) string {
	t.Helper()
	got := f.writtenAfter(2)
	if len(got) != 2 || got[1] != keyBytes[Enter] || !strings.HasPrefix(got[0], pasteStart+photoLookAt) {
		t.Fatalf("typed: %q", got)
	}
	path := strings.TrimSuffix(strings.TrimPrefix(got[0], pasteStart+photoLookAt), pasteEnd)
	if len(path) < 2 || path[0] != '"' || path[len(path)-1] != '"' {
		t.Fatalf("not in quotes: %q", path)
	}
	return path[1 : len(path)-1]
}

// A JPEG's EXIF (GPS included) and a PNG's text and eXIf chunks are gone: what's kept is the pixels, encoded again.
func TestAPhotosMetadataIsStripped(t *testing.T) {
	secret := "GPSLatitude 48.8584 SECRET-MARKER"
	original := testJPEG(t)
	app1 := append([]byte("Exif\x00\x00MM\x00\x2a"), secret...)
	exif := append([]byte{0xFF, 0xE1, byte((len(app1) + 2) >> 8), byte(len(app1) + 2)}, app1...)
	withExif := append(append([]byte{0xFF, 0xD8}, exif...), original[2:]...)
	plain := testPNG(t)
	end := bytes.Index(plain, []byte("IDAT")) - 4
	withText := append(append(append(bytes.Clone(plain[:end]), pngChunk("tEXt", []byte("Comment\x00"+secret))...),
		pngChunk("eXIf", []byte("MM\x00\x2a"+secret))...), plain[end:]...)
	for _, c := range []struct {
		kind string
		data []byte
		ext  string
	}{{"image/jpeg", withExif, ".jpg"}, {"image/png", withText, ".png"}} {
		if !bytes.Contains(c.data, []byte(secret)) {
			t.Fatalf("%s: the test picture has no metadata", c.kind)
		}
		clean, ext, status, why := cleanPhoto(c.kind, c.data)
		if why != "" || ext != c.ext {
			t.Fatalf("%s: %d %q %q", c.kind, status, why, ext)
		}
		if bytes.Contains(clean, []byte("SECRET-MARKER")) || bytes.Contains(clean, []byte("GPS")) ||
			bytes.Contains(clean, []byte("Exif")) || bytes.Contains(clean, []byte("tEXt")) || bytes.Contains(clean, []byte("eXIf")) {
			t.Errorf("%s: metadata kept", c.kind)
		}
		picture, _, err := image.Decode(bytes.NewReader(clean))
		if err != nil || picture.Bounds() != testPicture().Bounds() {
			t.Errorf("%s: not the same picture: %v", c.kind, err)
		}
	}
	// A JPEG encoded again has no APP segment at all: only what the encoder writes.
	clean, _, _, _ := cleanPhoto("image/jpeg", withExif)
	if bytes.Contains(clean, []byte{0xFF, 0xE1}) {
		t.Error("an APP1 segment is in the new file")
	}
}

// A picture whose header says it is too big is refused before it's unpacked; any other format, a type that isn't
// the bytes', and a picture that can't be read are refused too.
func TestABombAndOtherFormatsAreRefused(t *testing.T) {
	jpegData, pngData := testJPEG(t), testPNG(t)
	var gifData bytes.Buffer
	if err := gif.Encode(&gifData, testPicture(), nil); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, kind string
		data       []byte
		status     int
		says       string
	}{
		{"a PNG bomb, 50,000 a side", "image/png", withPNGSize(pngData, 50000, 50000), http.StatusRequestEntityTooLarge, photoTooLarge},
		{"a PNG over the pixels", "image/png", withPNGSize(pngData, 8000, 8000), http.StatusRequestEntityTooLarge, photoTooLarge},
		{"a PNG over a side", "image/png", withPNGSize(pngData, 8193, 10), http.StatusRequestEntityTooLarge, photoTooLarge},
		{"a JPEG bomb", "image/jpeg", withJPEGSize(t, jpegData, 65000, 65000), http.StatusRequestEntityTooLarge, photoTooLarge},
		{"a JPEG over the pixels", "image/jpeg", withJPEGSize(t, jpegData, 6000, 6000), http.StatusRequestEntityTooLarge, photoTooLarge},
		{"a GIF", "image/gif", gifData.Bytes(), http.StatusUnsupportedMediaType, photoNotImage},
		{"a GIF called a PNG", "image/png", gifData.Bytes(), http.StatusUnsupportedMediaType, photoNotImage},
		{"a PNG called a JPEG", "image/jpeg", pngData, http.StatusUnsupportedMediaType, photoNotImage},
		{"no type", "", pngData, http.StatusUnsupportedMediaType, photoNotImage},
		{"a WebP", "image/webp", []byte("RIFF\x10\x00\x00\x00WEBPVP8 "), http.StatusUnsupportedMediaType, photoNotImage},
		{"an SVG", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), http.StatusUnsupportedMediaType, photoNotImage},
		{"text", "text/plain", []byte("hello"), http.StatusUnsupportedMediaType, photoNotImage},
		{"a PNG cut short", "image/png", pngData[:len(pngData)/2], http.StatusBadRequest, photoUnread},
		{"a JPEG cut short", "image/jpeg", jpegData[:40], http.StatusBadRequest, photoUnread},
		{"a PNG with no pixels", "image/png", withPNGSize(pngData, 0, 0), http.StatusBadRequest, photoUnread},
	} {
		if _, _, status, says := cleanPhoto(c.kind, c.data); status != c.status || says != c.says {
			t.Errorf("%s: %d %q", c.name, status, says)
		}
	}
	// Through the route: nothing is kept and nothing typed.
	d, f := windowDeck(t)
	if status, says := sendTestPhoto(d, textFromWindow, "image/png", bytes.NewReader(withPNGSize(pngData, 50000, 50000))); status != http.StatusRequestEntityTooLarge || says != photoTooLarge {
		t.Errorf("the bomb through the route: %d %q", status, says)
	}
	if got := offBy(f); len(got) != 0 || len(photosOf(t, d, "s1")) != 0 {
		t.Errorf("typed %q, kept %v", got, photosOf(t, d, "s1"))
	}
}

// Both rows are 8 MB and 30 seconds; a body over 8 MB is refused, and one that takes too long to arrive too.
func TestAPhotosSizeAndTimeLimits(t *testing.T) {
	for _, path := range []string{"/photo", "/talk/photo"} {
		if most, wait := requestLimits(http.MethodPost, path); most != 8<<20 || wait != 30*time.Second {
			t.Errorf("%s: %d bytes in %s", path, most, wait)
		}
	}
	d, f := windowDeck(t)
	d.mu.Lock()
	window := d.access.WindowKey
	d.mu.Unlock()
	big := append(testPNG(t), make([]byte, 8<<20)...)
	got := serve(t, d, call{method: "POST", target: "/talk/photo?session=s1", host: "localhost:7710", from: thisComputer,
		body: string(big), headers: map[string]string{hookKeyHeader: window, "Content-Type": "image/png"}})
	if got.Code != http.StatusRequestEntityTooLarge || strings.TrimSpace(got.Body.String()) != photoTooBig {
		t.Errorf("over 8 MB: %d %q", got.Code, got.Body.String())
	}
	// Too slow: the row's time, cut short for the test, on a real server on this computer's loopback.
	row := phoneRouteFor(http.MethodPost, "/talk/photo")
	saved := row.timeout
	row.timeout = 200 * time.Millisecond
	t.Cleanup(func() { row.timeout = saved })
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	conn, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "POST /talk/photo?session=s1 HTTP/1.1\r\nHost: 127.0.0.1\r\n%s: %s\r\nContent-Type: image/png\r\nContent-Length: 1000\r\n\r\n", hookKeyHeader, window)
	conn.Write(testPNG(t)[:20])
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	answer, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(answer.Body)
	if answer.StatusCode != http.StatusRequestTimeout || strings.TrimSpace(string(body)) != photoTooSlow {
		t.Errorf("too slow: %d %q", answer.StatusCode, body)
	}
	if got := offBy(f); len(got) != 0 || len(photosOf(t, d, "s1")) != 0 {
		t.Errorf("typed %q, kept %v", got, photosOf(t, d, "s1"))
	}
}

// POST /photo is a paired phone's over Tailscale only: the Wi-Fi's plain http and the window are refused. POST
// /talk/photo is the window's alone.
func TestAPhotoComesOverTailscaleOrFromTheWindowOnly(t *testing.T) {
	saved := tailscaleAddresses
	tailscaleAddresses = func() []string { return []string{"100.64.1.2"} }
	t.Cleanup(func() { tailscaleAddresses = saved })
	d, f := windowDeck(t)
	d.mu.Lock()
	d.config.PhoneMicEnabled = true
	window := d.access.WindowKey
	d.mu.Unlock()
	phone := pair(t, d)
	send := func(path, key, host, from, local string) (int, string) {
		picture := testPNG(t)
		r := httptest.NewRequest(http.MethodPost, path+"?session=s1", bytes.NewReader(picture))
		if host == deckOnWiFi {
			// A phone on the Wi-Fi signs its requests (signed.go, 1.25), as deck.html does.
			r = httptest.NewRequest(http.MethodPost, signedTarget(http.MethodPost, path+"?session=s1&t="+url.QueryEscape(key), string(picture), time.Now()), bytes.NewReader(picture))
		} else {
			r.Header.Set(hookKeyHeader, key)
		}
		r.Host, r.RemoteAddr = host, from
		r.Header.Set("Content-Type", "image/png")
		if local != "" {
			ip, _, _ := net.SplitHostPort(local)
			r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP(ip), Port: 7710}))
		}
		w := httptest.NewRecorder()
		(&router{deck: d}).ServeHTTP(w, r)
		return w.Code, strings.TrimSpace(w.Body.String())
	}
	if status, says := send("/photo", phone, deckOnWiFi, aPhone, ""); status != http.StatusForbidden || says != "Photos are sent only over Tailscale." {
		t.Errorf("a phone on the Wi-Fi: %d %q", status, says)
	}
	// Unsigned, with its key as an old page sent it, it gets no further (1.25).
	unsigned := httptest.NewRequest(http.MethodPost, "/photo?session=s1", bytes.NewReader(testPNG(t)))
	unsigned.Host, unsigned.RemoteAddr = deckOnWiFi, aPhone
	unsigned.Header.Set(hookKeyHeader, phone)
	unsigned.Header.Set("Content-Type", "image/png")
	refused := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(refused, unsigned)
	if refused.Code != http.StatusUnauthorized {
		t.Errorf("a phone on the Wi-Fi, unsigned: %d", refused.Code)
	}
	if status, _ := send("/photo", window, "localhost:7710", thisComputer, ""); status != http.StatusForbidden {
		t.Errorf("the window on /photo: %d", status)
	}
	if status, says := send("/talk/photo", phone, "100.64.1.2:7710", "100.101.102.103:50000", "100.64.1.2:7710"); status != http.StatusForbidden || says != d.shownOnlyInTheWindow("Pasting a picture") {
		t.Errorf("a phone on /talk/photo: %d %q", status, says)
	}
	if got := offBy(f); len(got) != 0 || len(photosOf(t, d, "s1")) != 0 {
		t.Fatalf("typed %q, kept %v", got, photosOf(t, d, "s1"))
	}
	if status, says := send("/photo", phone, "100.64.1.2:7710", "100.101.102.103:50000", "100.64.1.2:7710"); status != http.StatusNoContent {
		t.Fatalf("a phone over Tailscale: %d %q", status, says)
	}
	lookAtLine(t, f)
	d.mu.Lock()
	delete(d.lastReturn, "s1")
	d.mu.Unlock()
	if status, says := send("/talk/photo", window, "localhost:7710", thisComputer, ""); status != http.StatusNoContent {
		t.Fatalf("the window: %d %q", status, says)
	}
	if len(photosOf(t, d, "s1")) != 2 {
		t.Errorf("kept: %v", photosOf(t, d, "s1"))
	}
}

// The photo is a file of its own, 0600, in a 0700 folder of the desk's in the support folder, outside the desk's
// repo and told to git to be left out; the agent is told its path with typeText, and Return.
func TestAPhotoIsAPrivateFileOutsideTheRepo(t *testing.T) {
	d, f := windowDeck(t)
	repo := t.TempDir()
	if status, says := sendTestPhoto(d, textFromWindow, "image/jpeg", bytes.NewReader(testJPEG(t))); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, says)
	}
	path := lookAtLine(t, f)
	kept := photosOf(t, d, "s1")
	if len(kept) != 1 || kept[0] != path || !photoName.MatchString(filepath.Base(path)) || filepath.Ext(path) != ".jpg" {
		t.Fatalf("typed %q, kept %v", path, kept)
	}
	for _, place := range []string{repo, "/work/api"} {
		if rel, err := filepath.Rel(place, path); err == nil && !strings.HasPrefix(rel, "..") {
			t.Errorf("%s is inside %s", path, place)
		}
	}
	// The desk's folder is a random name, never its session id.
	d.mu.Lock()
	folder := d.photoDesks["s1"]
	d.mu.Unlock()
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(folder) || filepath.Dir(path) != filepath.Join(supportDir, photosFolder, folder) {
		t.Errorf("not in the desk's random folder %q: %s", folder, path)
	}
	if runtime.GOOS != "windows" {
		for file, mode := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700 | os.ModeDir, filepath.Join(supportDir, photosFolder): 0o300 | os.ModeDir} {
			if info, err := os.Lstat(file); err != nil || info.Mode() != mode {
				t.Errorf("%s: %v %v, want %v", file, info.Mode(), err, mode)
			}
		}
	}
	if ignore, _ := os.ReadFile(filepath.Join(supportDir, photosFolder, ".gitignore")); string(ignore) != "*\n" {
		t.Errorf(".gitignore: %q", ignore)
	}
	// A link where the desk's folder goes is never followed out of the photos folder (no links made without rights on
	// Windows).
	if runtime.GOOS == "windows" {
		return
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(supportDir, photosFolder, "s2")); err != nil {
		t.Fatal(err)
	}
	if _, err := keepPhoto(supportDir, "s2", ".jpg", []byte("x")); err == nil {
		t.Error("kept through a link")
	}
	if names, _ := os.ReadDir(outside); len(names) != 0 {
		t.Errorf("written outside: %v", names)
	}
	// Nor is the photos folder itself, when a link takes its place.
	support := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(support, photosFolder)); err != nil {
		t.Fatal(err)
	}
	if _, err := keepPhoto(support, randomHex(16), ".jpg", []byte("x")); err == nil {
		t.Error("kept through a linked photos folder")
	}
	if names, _ := os.ReadDir(outside); len(names) != 0 {
		t.Errorf("written outside: %v", names)
	}
}

// A desk's photos go when it ends, and every photo after half an hour (well inside answer 6's 24 hours), whichever
// comes first; the folders of desks no longer here go at the next look.
func TestAPhotoIsDeletedWhenTheDeskEndsAndAfterHalfAnHour(t *testing.T) {
	if photoWait != 30*time.Minute || photoWait > 24*time.Hour {
		t.Fatalf("photoWait is %v: photoNote says 30 minutes", photoWait)
	}
	d, f := windowDeck(t)
	if status, says := sendTestPhoto(d, textFromWindow, "image/png", bytes.NewReader(testPNG(t))); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, says)
	}
	path := lookAtLine(t, f)
	folder := filepath.Base(filepath.Dir(path))
	// Kept while the desk is here and the photo is young.
	sweepPhotosIn(supportDir, []string{folder}, nil, time.Now().Add(photoWait-time.Minute))
	if _, err := os.Stat(path); err != nil {
		t.Fatal("deleted before its time")
	}
	sweepPhotosIn(supportDir, []string{folder}, nil, time.Now().Add(photoWait+time.Minute))
	if _, err := os.Stat(path); err == nil {
		t.Error("kept past half an hour")
	}
	// The folders of the last run, and anything that isn't a folder, deleted as the app starts again; the photos
	// folder is left unlisted, with its .gitignore.
	gone := filepath.Join(supportDir, photosFolder, "gone")
	os.MkdirAll(gone, 0o700)
	os.WriteFile(filepath.Join(gone, photoPrefix+strings.Repeat("a", 32)+".png"), []byte("x"), 0o600)
	stray := filepath.Join(supportDir, photosFolder, "stray")
	os.WriteFile(stray, []byte("x"), 0o600)
	clearPhotos(supportDir)
	for _, name := range []string{gone, stray, filepath.Dir(path)} {
		if _, err := os.Lstat(name); err == nil {
			t.Errorf("%s kept", name)
		}
	}
	if _, err := os.ReadDir(filepath.Join(supportDir, photosFolder)); err == nil && listsAsYou() {
		t.Error("the photos folder can be listed after the start's look")
	}
	d.mu.Lock()
	d.photoDesks = nil
	d.mu.Unlock()
	// A new photo, then the desk ends: its folder goes.
	d.mu.Lock()
	delete(d.lastReturn, "s1")
	d.mu.Unlock()
	if status, says := sendTestPhoto(d, textFromWindow, "image/png", bytes.NewReader(testPNG(t))); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, says)
	}
	if len(photosOf(t, d, "s1")) != 1 {
		t.Fatalf("kept: %v", photosOf(t, d, "s1"))
	}
	folder = filepath.Dir(photosOf(t, d, "s1")[0])
	d.Ingest(map[string]any{"hook_event_name": "SessionEnd", "session_id": "s1", "cwd": "/work/api", "reason": "logout"}, "claude", "launcher:L1", "", 60)
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(folder); err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(folder); err == nil {
		t.Error("the desk ended and its photos stayed")
	}
	if _, err := os.Stat(filepath.Join(supportDir, photosFolder, ".gitignore")); err != nil {
		t.Error("the photos folder's .gitignore went")
	}
}

// Never typed into a desk that asks: refused before the body is read, nothing kept; and if it begins asking while
// the path waits for the key worker, nothing is typed and the photo goes.
func TestAPhotoIsNeverTypedWhileTheDeskAsks(t *testing.T) {
	d, f := windowDeck(t)
	asksAt(d, "launcher:L1", 30)
	read := false
	body := readerFunc(func(p []byte) (int, error) { read = true; return 0, io.EOF })
	if status, says := sendTestPhoto(d, textFromWindow, "image/png", body); status != http.StatusConflict || says != windowAnswerFirst {
		t.Errorf("asking: %d %q", status, says)
	}
	d.mu.Lock()
	d.config.PhoneMicEnabled = true
	d.mu.Unlock()
	if status, says := sendTestPhoto(d, textFromPhone, "image/png", bytes.NewReader(testPNG(t))); status != http.StatusConflict || says != answerFirst {
		t.Errorf("asking, from a phone: %d %q", status, says)
	}
	if read || len(photosOf(t, d, "s1")) != 0 {
		t.Errorf("read %v, kept %v", read, photosOf(t, d, "s1"))
	}
	// Free again, then asking while the path waits for the key worker.
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "launcher:L1", "", 40)
	hold := make(chan struct{})
	d.work <- func() { <-hold }
	answered := make(chan [2]any, 1)
	go func() {
		status, says := sendTestPhoto(d, textFromWindow, "image/png", bytes.NewReader(testPNG(t)))
		answered <- [2]any{status, says}
	}()
	for i := 0; i < 200 && len(photosOf(t, d, "s1")) == 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	asksAt(d, "launcher:L1", 50)
	close(hold)
	if got := <-answered; got[0] != http.StatusConflict || got[1] != windowAnswerFirst {
		t.Errorf("asked meanwhile: %v", got)
	}
	if got := offBy(f); len(got) != 0 {
		t.Errorf("typed: %q", got)
	}
	if kept := photosOf(t, d, "s1"); len(kept) != 0 {
		t.Errorf("a photo never typed was kept: %v", kept)
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

// An agent that can't take pictures is told so, on the card and by the route, and its desk gets nothing.
func TestAnAgentThatCantTakePicturesIsSaidSo(t *testing.T) {
	d, f := windowDeck(t)
	if photo := photoTaking(deskTyping{OK: true, Lines: true}, Claude); photo != (deskPhoto{OK: true, Note: photoNote(Claude)}) {
		t.Errorf("Claude Code: %+v", photo)
	}
	if photo := photoTaking(deskTyping{OK: true}, Grok); photo != (deskPhoto{Why: noPictures}) {
		t.Errorf("Grok: %+v", photo)
	}
	if photo := photoTaking(deskTyping{Why: windowAnswerFirst}, Claude); photo != (deskPhoto{Why: windowAnswerFirst, Note: photoNote(Claude)}) {
		t.Errorf("asking: %+v", photo)
	}
	if _, card := d.talk("s1"); !reflect.DeepEqual(card.(talkCard).Photo, deskPhoto{OK: true, Note: photoNote(Claude)}) {
		t.Errorf("the card: %+v", card.(talkCard).Photo)
	}
	// What a photo means is said plainly: the agents with no fence are named, and none is said to keep it safe.
	for _, agent := range []Agent{Codex, Gemini, OpenCode} {
		if note := photoNote(agent); !strings.Contains(note, agent.Name()+" runs without Smolcorp's fence") ||
			!strings.Contains(note, "your other agents can open too") {
			t.Errorf("%s: %q", agent, note)
		}
	}
	// RT126PH1R2 B1: Claude Code's note says what a fenced hire can and can't reach, and how long the file lasts.
	for _, part := range []string{"Codex, Gemini CLI, OpenCode, and Claude Code desks you started yourself",
		"deleted once this desk has opened it, or after 30 minutes", "its path in your prompt history",
		"keeps a hire's file tools out of both, but not its shell,"} {
		if note := photoNote(Claude); !strings.Contains(note, part) {
			t.Errorf("Claude Code: %q, without %q", note, part)
		}
	}
	d.mu.Lock()
	d.sessions.Get("s1").Agent = Agent("acp-something")
	d.mu.Unlock()
	if _, card := d.talk("s1"); card.(talkCard).Photo != (deskPhoto{Why: noPictures}) {
		t.Errorf("the card of an agent that can't: %+v", card.(talkCard).Photo)
	}
	if status, says := sendTestPhoto(d, textFromWindow, "image/png", bytes.NewReader(testPNG(t))); status != http.StatusConflict || says != noPictures {
		t.Errorf("%d %q", status, says)
	}
	if got := offBy(f); len(got) != 0 || len(photosOf(t, d, "s1")) != 0 {
		t.Errorf("typed %q, kept %v", got, photosOf(t, d, "s1"))
	}
}

// The photo's path never goes into the events log, the phone's or the viewer's snapshot, a prompt's label, a desk's
// activity, a summary or the conversation, though the agent reads it, asks to and says it.
func TestAPhotoIsInNoEventsOrSummaries(t *testing.T) {
	d, f := windowDeck(t)
	if status, says := sendTestPhoto(d, textFromWindow, "image/png", bytes.NewReader(testPNG(t))); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, says)
	}
	path := lookAtLine(t, f)
	name := filepath.Base(path)
	// The agent asks to read it, then reads it: hook events with the path in them.
	read := map[string]any{"file_path": path}
	d.Ingest(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/work/api", "prompt": photoLookAt + path}, "claude", "launcher:L1", "", 50)
	d.Ingest(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/api", "tool_name": "Read", "tool_input": read}, "claude", "launcher:L1", "", 60)
	d.mu.Lock()
	label := approvalLabel("Read", `{"file_path":`+string(must(json.Marshal(path)))+`}`)
	d.mu.Unlock()
	d.Ingest(map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "cwd": "/work/api", "tool_name": "Read", "tool_input": read, "tool_use_id": "t1"}, "claude", "launcher:L1", "", 70)
	d.mu.Lock()
	activity := d.sessions.Get("s1").ToolActivity
	phone := string(d.snapshot())
	viewer := string(must(json.Marshal(d.viewerSnapshot(false, func(s string) string { return s }))))
	d.mu.Unlock()
	events := string(must(json.Marshal(d.events.snapshot())))
	file, _ := os.ReadFile(filepath.Join(supportDir, "events.jsonl"))
	// The transcript as Claude Code writes it: the prompt, the Read and its result, and the agent naming the file.
	transcript := filepath.Join(t.TempDir(), "s1.jsonl")
	lines := []map[string]any{
		{"type": "user", "uuid": "u1", "message": map[string]any{"role": "user", "content": photoLookAt + path}},
		{"type": "assistant", "uuid": "a1", "message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "text", "text": "Reading " + path + " now; the file is " + name + "."},
			map[string]any{"type": "tool_use", "id": "t1", "name": "Read", "input": read}}}},
		{"type": "user", "uuid": "u2", "message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": []any{map[string]any{"type": "text", "text": "Read image " + path}}}}}},
	}
	var jsonl []byte
	var raw [][]byte
	for _, line := range lines {
		data := must(json.Marshal(line))
		raw = append(raw, data)
		jsonl = append(append(jsonl, data...), '\n')
	}
	if err := os.WriteFile(transcript, jsonl, 0o600); err != nil {
		t.Fatal(err)
	}
	summary := extractTranscript(raw, summaryBudget, nil, nil).Text
	page, _ := readConversation(transcript, nil, conversationPageMost)
	conversation := string(must(json.Marshal(page)))
	for what, text := range map[string]string{"the events log": events, "the events file": string(file), "the phone's snapshot": phone,
		"the viewer's snapshot": viewer, "the prompt's label": label, "the desk's activity": activity, "the summary": summary,
		"the conversation": conversation} {
		for _, leak := range []string{path, name, filepath.Join(supportDir, photosFolder), photoPrefix} {
			if strings.Contains(text, leak) {
				t.Errorf("%s holds %q:\n%s", what, leak, text)
			}
		}
	}
	if !strings.Contains(events, "Read: "+photoInTheText) || activity != "Reading "+photoInTheText || label != "Read: "+photoInTheText ||
		!strings.Contains(summary, "TOOL Read: "+photoInTheText) || !strings.Contains(conversation, photoLookAt+photoInTheText) {
		t.Errorf("not said as a photo: events %s, activity %q, label %q, summary %q", events, activity, label, summary)
	}
	// What hidePhotos leaves alone: a word, a path elsewhere, and a name that isn't a photo's.
	for _, text := range []string{"my photos", "/work/api/photos/cat.png", photoPrefix + "nothex.png"} {
		if got := hidePhotos(text); got != text {
			t.Errorf("%q became %q", text, got)
		}
	}
}

// RT126PH1 B1: a hire's fence is built as it starts, before another desk's first photo. That photo must still be out
// of its reach: the photos folder can't be listed (Glob, LS, ls), the desk's folder isn't its session id, so a path
// guessed from what a hire can see opens nothing, and only the exact path the desk was given opens the photo. Nothing
// in the photos folder can be changed by a hire.
func TestAHiresFenceKeepsOtherDesksPhotos(t *testing.T) {
	d, f := windowDeck(t)
	photos := filepath.Join(supportDir, photosFolder)
	root := absoluteRulePath(supportDir) + "/" + photosFolder
	// The hire's fence, built before any photo.
	fence := photoFenceRules(supportDir, nil, nil)
	if want := append([]string{"Edit(" + root + "/**)"}, promptStoreRules(nil)...); !reflect.DeepEqual(fence, want) {
		t.Errorf("%q, want %q", fence, want)
	}
	if got := photoFenceRules(supportDir, nil, fence); len(got) != 0 {
		t.Errorf("twice: %q", got)
	}
	// Then desk s1's first photo.
	if status, says := sendTestPhoto(d, textFromWindow, "image/png", bytes.NewReader(testPNG(t))); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, says)
	}
	path := lookAtLine(t, f)
	// Listing the photos folder finds nothing: not by reading it, not by a glob, not by walking it.
	if listsAsYou() {
		if names, err := os.ReadDir(photos); err == nil {
			t.Errorf("the photos folder lists: %v", names)
		}
	}
	for _, pattern := range []string{"*", "*/*", "s1/*"} {
		if found, _ := filepath.Glob(filepath.Join(photos, pattern)); len(found) != 0 && listsAsYou() {
			t.Errorf("%s finds %v", pattern, found)
		}
	}
	_ = filepath.WalkDir(photos, func(name string, entry os.DirEntry, err error) error {
		if err == nil && strings.HasPrefix(entry.Name(), photoPrefix) && listsAsYou() {
			t.Errorf("walking the photos folder finds %s", name)
		}
		return nil
	})
	// A path guessed from the session id, as the folder was named before, opens nothing.
	for _, guess := range []string{filepath.Join(photos, "s1"), filepath.Join(photos, "s1", filepath.Base(path))} {
		if _, err := os.Stat(guess); err == nil {
			t.Errorf("guessed: %s", guess)
		}
	}
	// The exact path the desk was told opens the photo.
	if data, err := os.ReadFile(path); err != nil || !bytes.HasPrefix(data, []byte("\x89PNG")) {
		t.Errorf("the desk's own photo: %v", err)
	}
	// Nor is the folder listed again by the deck's sweeps, or a new photo.
	d.sweepPhotos()
	d.mu.Lock()
	delete(d.lastReturn, "s1")
	d.mu.Unlock()
	if status, says := sendTestPhoto(d, textFromWindow, "image/jpeg", bytes.NewReader(testJPEG(t))); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, says)
	}
	if kept := photosOf(t, d, "s1"); len(kept) != 2 || filepath.Dir(kept[0]) != filepath.Dir(path) {
		t.Errorf("the desk's second photo isn't in its folder: %v", kept)
	}
	if _, err := os.ReadDir(photos); err == nil && listsAsYou() {
		t.Error("the photos folder lists after a sweep")
	}
}

// A picture that would take too much memory once unpacked is refused from its header: a progressive JPEG keeps its
// coefficients as it decodes, 16 bytes a pixel more for CMYK.
func TestAPictureTooHeavyToUnpackIsRefused(t *testing.T) {
	baseline := withJPEGSize(t, testJPEG(t), 5000, 5000)
	progressive := bytes.Clone(baseline)
	progressive[bytes.Index(progressive, []byte{0xFF, 0xC0})+1] = 0xC2
	if progressiveJPEG(baseline) || !progressiveJPEG(progressive) {
		t.Fatal("progressive not told from baseline")
	}
	if _, _, status, says := cleanPhoto("image/jpeg", progressive); status != http.StatusRequestEntityTooLarge || says != photoTooHeavy {
		t.Errorf("a 25 MP progressive JPEG: %d %q", status, says)
	}
	// The same size, baseline, passes the header's checks (and its made-up pixels then can't be read).
	if _, _, status, says := cleanPhoto("image/jpeg", baseline); status != http.StatusBadRequest || says != photoUnread {
		t.Errorf("a 25 MP baseline JPEG: %d %q", status, says)
	}
	full := image.Config{Width: 5000, Height: 5000}
	for _, c := range []struct {
		model       color.Model
		progressive bool
		fits        bool
	}{{color.RGBAModel, false, true}, {color.NRGBA64Model, false, true}, {color.YCbCrModel, false, true},
		{color.YCbCrModel, true, false}, {color.CMYKModel, true, false}} {
		full.ColorModel = c.model
		if fits := photoMemory(full, c.progressive) <= photoMemoryMost; fits != c.fits {
			t.Errorf("%T progressive %v: %d bytes", c.model, c.progressive, photoMemory(full, c.progressive))
		}
	}
}

// A desk's photo goes once the desk itself has read it, the hook's PostToolUse saying so: not when it only asks to,
// not when another desk reads the path, and not for a command that merely names it. Its folder stays unlisted.
func TestAPhotoGoesOnceItsDeskHasReadIt(t *testing.T) {
	d, f := windowDeck(t)
	if status, says := sendTestPhoto(d, textFromWindow, "image/png", bytes.NewReader(testPNG(t))); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, says)
	}
	path := lookAtLine(t, f)
	read := map[string]any{"file_path": path}
	for _, event := range []map[string]any{
		{"hook_event_name": "PreToolUse", "session_id": "s1", "tool_name": "Read", "tool_input": read, "tool_use_id": "t1"},
		{"hook_event_name": "PostToolUseFailure", "session_id": "s1", "tool_name": "Read", "tool_input": read, "tool_use_id": "t1"},
		{"hook_event_name": "PostToolUse", "session_id": "s2", "tool_name": "Read", "tool_input": read, "tool_use_id": "t2"},
		{"hook_event_name": "PostToolUse", "session_id": "s1", "tool_name": "Bash", "tool_input": map[string]any{"command": "ls -l " + path}},
		{"hook_event_name": "PostToolUse", "session_id": "s1", "tool_name": "Read", "tool_input": map[string]any{"file_path": filepath.Join(filepath.Dir(path), "..", "other", filepath.Base(path))}},
	} {
		// Another desk's read goes straight to the deck's code: through s1's tab, s2 would take s1's place, and end it.
		if event["session_id"] == "s2" {
			d.mu.Lock()
			d.notePhotoRead(event)
			d.mu.Unlock()
			continue
		}
		event["cwd"] = "/work/api"
		d.Ingest(event, "claude", "launcher:L1", "", 60)
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("gone before its desk read it: %v", err)
	}
	d.Ingest(map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": "/work/api", "tool_name": "Read", "tool_input": read, "tool_use_id": "t3"}, "claude", "launcher:L1", "", 70)
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(path); err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("its desk read it, and it stayed")
	}
	if _, err := os.ReadDir(filepath.Join(supportDir, photosFolder)); err == nil && listsAsYou() {
		t.Error("the photos folder lists after a read")
	}
	// Another agent's way of reading: Gemini CLI's read_many_files, its paths in a list.
	d.mu.Lock()
	delete(d.lastReturn, "s1")
	d.mu.Unlock()
	if status, says := sendTestPhoto(d, textFromWindow, "image/jpeg", bytes.NewReader(testJPEG(t))); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, says)
	}
	second := photosOf(t, d, "s1")
	if len(second) != 1 {
		t.Fatalf("kept: %v", second)
	}
	d.mu.Lock()
	d.notePhotoRead(map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "tool_name": "read_many_files",
		"tool_input": map[string]any{"paths": []any{"/work/api/README.md", second[0]}}})
	d.mu.Unlock()
	for i := 0; i < 200 && len(photosOf(t, d, "s1")) != 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if kept := photosOf(t, d, "s1"); len(kept) != 0 {
		t.Errorf("read_many_files read it, and it stayed: %v", kept)
	}
}

// readDenied says whether rules, a hire's deny rules, keep its Read tool (and so Grep and Glob, which leave out what
// Read can't open) away from file, for a hire whose home is home: Claude Code's rule paths, ~/ from the home, // from
// the root and **/ anywhere, matched as gitignore matches (* within a folder's name, ** across folders). Paths are
// compared as rules write them: on Windows the drive is a folder (absoluteRulePath).
func readDenied(rules []string, home, file string) bool {
	for _, rule := range rules {
		pattern, ok := strings.CutPrefix(rule, "Read(")
		if !ok || !strings.HasSuffix(pattern, ")") {
			continue
		}
		pattern = strings.TrimSuffix(pattern, ")")
		switch {
		case strings.HasPrefix(pattern, "~/"):
			pattern = absoluteRulePath(home)[1:] + pattern[1:]
		case strings.HasPrefix(pattern, "//"):
			pattern = pattern[1:]
		case strings.HasPrefix(pattern, "**/"):
			pattern = "/" + pattern
		default:
			continue
		}
		var expression strings.Builder
		expression.WriteString("^")
		for i := 0; i < len(pattern); i++ {
			switch {
			case strings.HasPrefix(pattern[i:], "/**/"):
				expression.WriteString("/(?:.*/)?")
				i += 3
			case strings.HasPrefix(pattern[i:], "**"):
				expression.WriteString(".*")
				i++
			case pattern[i] == '*':
				expression.WriteString("[^/]*")
			default:
				expression.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
			}
		}
		expression.WriteString("$")
		if regexp.MustCompile(expression.String()).MatchString(absoluteRulePath(file)[1:]) {
			return true
		}
	}
	return false
}

// RT126PH1R2 B1, the way it happens: Claude Code writes every prompt typed into a desk, a photo's path with it, to
// its prompt history, and the other stores in its config folder. A hire, fenced as it starts, uses Read or Grep alone
// on everything in a stand-in home where desk s1's photo prompt was written as each store keeps it, on the default
// account and on a profile: every file that holds the path is denied to it, and the desk's own photo still opens. A
// stand-in claude, which only prints what it was started with, shows where the hire's own Claude Code keeps its
// stores: the profile its start script names, or ~/.claude. A path found where nothing denies it (the desk's own
// plan) opens nothing once the desk has read its photo.
func TestAHireCantFindAPhotosPathInAnyPromptStore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in home and claude are POSIX: the rules are the same, written //c/… (absoluteRulePath)")
	}
	d, f := windowDeck(t)
	home := t.TempDir()
	profile := claudeProfile{Name: "work", Dir: filepath.Join(home, "accounts", "work")}
	// The hire's settings as its command line carries them (hireSettingsLine, SG1): never a file a hire can write.
	settings, err := hireSettingsLine(hireSettingsOptions{HireID: "hire1", Auto: true, CLI: "/app/smolcorp", Support: supportDir,
		Profiles: []claudeProfile{profile}, Run: func(string, []string) (int, string, bool) { return 0, hireGuardPrinted, true },
		IsExec: func(string) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(settings), &file); err != nil {
		t.Fatalf("%s: %v", settings, err)
	}
	deny := file.Permissions.Deny
	// The stand-in claude: what the hire's start script runs, on the profile and on the default account.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho \"config=$CLAUDE_CONFIG_DIR\"\nfor word in \"$@\"; do echo \"arg=$word\"; done\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	configs := map[string]string{}
	for name, start := range map[string]hireStart{
		"profile": {Role: testHireRole(), Message: "go", ConfigDir: profile.Dir, Settings: settings},
		"default": {Role: testHireRole(), Message: "go", DefaultAccount: true, Settings: settings},
	} {
		script, err := start.startText()
		if err != nil {
			t.Fatal(err)
		}
		at := filepath.Join(t.TempDir(), "start.sh")
		if err := os.WriteFile(at, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		run := exec.Command("/bin/sh", at)
		run.Env = []string{"HOME=" + home, "PATH=" + bin + ":/usr/bin:/bin", "CLAUDE_CONFIG_DIR=/left/over"}
		out, err := run.Output()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		printed := string(out)
		if !strings.Contains(printed, "arg=--settings="+settings+"\n") {
			t.Fatalf("%s: started without its fence:\n%s", name, printed)
		}
		config, _, _ := strings.Cut(strings.TrimPrefix(printed, "config="), "\n")
		configs[name] = config
	}
	if configs["profile"] != profile.Dir || configs["default"] != "" {
		t.Fatalf("the hire's Claude Code keeps its stores in %q", configs)
	}
	defaultFolder := filepath.Join(home, ".claude")
	// Desk s1's photo, its prompt as typed.
	if status, says := sendTestPhoto(d, textFromWindow, "image/png", bytes.NewReader(testPNG(t))); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, says)
	}
	path := lookAtLine(t, f)
	prompt := photoLookAt + `"` + path + `"`
	line := string(must(json.Marshal(map[string]any{"display": prompt, "pastedContents": map[string]any{}, "project": "/work/api", "sessionId": "s1", "timestamp": 1})))
	// Each store as Claude Code keeps it, on both accounts: what holds the prompt, or words the agent wrote from it.
	stores := map[string]string{
		"history.jsonl": line + "\n", "history.jsonl.staging-1": line + "\n", "paste-cache/5f3a.txt": prompt,
		"projects/-work-api/s1.jsonl":            string(must(json.Marshal(map[string]any{"type": "user", "message": map[string]any{"content": prompt}}))),
		"projects/-work-api/s1/subagents/a.json": prompt, "todos/s1-agent-s1.json": `[{"content":"Look at ` + path + `"}]`,
		"tasks/s1/1.json": `{"subject":"` + path + `"}`, "file-history/s1/3e1f@v1": "notes: " + path, "sessions/4242.json": `{"sessionId":"s1","name":"` + prompt + `"}`,
		"jobs/j1/state.json": prompt, "teams/t1/inboxes/lead.json": prompt, "debug/s1.txt": "UserPromptSubmit " + prompt,
		"logs/s1.log": prompt, "traces/s1.jsonl": prompt, "telemetry/1p_failed_events.s1.json": prompt, "dump-prompts/s1.jsonl": prompt,
		"api-dumps/s1.json": prompt, "feedback-bundles/f1.zip": prompt, "usage-data/facets/s1.json": prompt,
		"backups/.claude.json.backup.1": `{"projects":{"/work/api":{"history":[` + line + `]}}}`,
		".claude.json":                  `{"projects":{"/work/api":{"history":[` + line + `]}}}`,
	}
	written := map[string]string{}
	for _, folder := range []string{defaultFolder, profile.Dir} {
		for name, content := range stores {
			written[filepath.Join(folder, name)] = content
		}
	}
	// The default account's config beside its folder, and the other agents' histories and sessions.
	for name, content := range map[string]string{".claude.json": stores[".claude.json"], ".claude.json.backup": stores[".claude.json"],
		".codex/history.jsonl": line, ".codex/sessions/2026/11/13/rollout-1.jsonl": prompt, ".codex/archived_sessions/r.jsonl": prompt,
		".codex/log/codex-tui.log": prompt, ".gemini/tmp/ab12/logs.json": prompt, ".gemini/tmp/ab12/chats/c.json": prompt,
		".local/share/opencode/storage/part/p1.json": prompt, ".local/state/opencode/prompt-history.jsonl": line} {
		written[filepath.Join(home, name)] = content
	}
	// Where nothing is denied: the desk's own plan, which it writes with Write.
	plan := filepath.Join(defaultFolder, "plans", "calm-river.md")
	written[plan] = "Look at the photo at " + path
	for name, content := range written {
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Grep over the whole stand-in home, as the hire would: each file holding the photo's name is denied to it, but the
	// plan.
	found := 0
	err = filepath.WalkDir(home, func(name string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, _ := os.ReadFile(name)
		if !bytes.Contains(data, []byte(filepath.Base(path))) {
			return nil
		}
		found++
		if denied := readDenied(deny, home, name); denied == (name == plan) {
			t.Errorf("%s: denied %v", name, denied)
		}
		return nil
	})
	if err != nil || found != len(written) {
		t.Fatalf("found the path in %d files of %d: %v", found, len(written), err)
	}
	// Grep or Read aimed at the history itself, on either account, is refused; the desk's own photo opens.
	var edits []string
	for _, rule := range deny {
		if pattern, ok := strings.CutPrefix(rule, "Edit("); ok {
			edits = append(edits, "Read("+pattern)
		}
	}
	for _, store := range []string{filepath.Join(defaultFolder, "history.jsonl"), filepath.Join(profile.Dir, "history.jsonl"),
		filepath.Join(defaultFolder, "paste-cache", "5f3a.txt"), filepath.Join(profile.Dir, "paste-cache", "5f3a.txt"), filepath.Join(home, ".claude.json")} {
		if !readDenied(deny, home, store) || !readDenied(edits, home, store) {
			t.Errorf("%s isn't denied to Read and Edit", store)
		}
	}
	if readDenied(deny, home, path) {
		t.Error("the desk's own photo is denied")
	}
	// The path from the plan, the one store left readable: once the desk has read its photo, it opens nothing.
	d.Ingest(map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": "/work/api", "tool_name": "Read", "tool_input": map[string]any{"file_path": path}}, "claude", "launcher:L1", "", 70)
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(path); err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.ReadFile(path); err == nil {
		t.Error("the path found in the plan still opens the photo after its desk read it")
	}
}

// FXCMDLEN: a desk's settings ride in its command line, which holds at most 32767 characters on Windows, so the prompt
// stores' rules don't grow with the accounts in ~/.claude* (one rule per store covers ~/.claude and every
// ~/.claude-<name> at once), and an account elsewhere adds one rule per store, written short: from ~ in the home
// folder, by its last two folders outside it. Every account's stores are still denied to Read and Edit; its plans and
// pictures, and a project's own folders of the same names, are not.
func TestPromptStoreRulesCoverEveryAccountAtOnce(t *testing.T) {
	saved := homeDir
	homeDir = t.TempDir()
	t.Cleanup(func() { homeDir = saved })
	outside := t.TempDir()
	defaultAccount, work, alt := filepath.Join(homeDir, ".claude"), filepath.Join(homeDir, ".claude-work"), filepath.Join(homeDir, ".claude-alt")
	elsewhere, far := filepath.Join(homeDir, "accounts", "home"), filepath.Join(outside, "claude", "old")
	// The disk's root, or on Windows the temp folder's drive (C:\), which rules write as a folder (//c).
	root, drive := string(filepath.Separator), ""
	if volume := filepath.VolumeName(outside); volume != "" {
		root, drive = volume+root, strings.ToLower(volume[:1])+"/"
	}
	for dir, want := range map[string]string{
		defaultAccount: "", work: "", alt: "",
		elsewhere:                            "~/accounts/home",
		filepath.Join(work, "nested"):        "~/.claude-work/nested",
		far:                                  "//" + drive + "**/claude/old",
		filepath.Join(root, "claude"):        "//" + drive + "claude",
		filepath.Join(root, "claude", "old"): "//" + drive + "claude/old",
		root:                                 strings.TrimSuffix("//"+drive, "/"),
	} {
		got, ok := claudeStoreFolder(dir)
		if got != want || ok != (want != "") {
			t.Errorf("%s: %q %v, want %q", dir, got, ok, want)
		}
	}
	if got, ok := claudeStoreFolder("accounts/home"); ok {
		t.Errorf("a relative folder: %q", got)
	}
	if runtime.GOOS == "windows" {
		if got, _ := claudeStoreFolder(`D:\Data\claude\old`); got != "//d/**/claude/old" {
			t.Errorf("on Windows: %q", got)
		}
	}
	none := promptStoreRules(nil)
	if got := promptStoreRules([]claudeProfile{{Name: "default", Dir: defaultAccount}, {Name: "work", Dir: work}, {Name: "alt", Dir: alt}}); !reflect.DeepEqual(got, none) {
		t.Errorf("the ~/.claude* accounts added rules:\n%q\nwant\n%q", got, none)
	}
	accounts := []claudeProfile{{Name: "default", Dir: defaultAccount}, {Name: "work", Dir: work}, {Name: "alt", Dir: alt},
		{Name: "home", Dir: elsewhere}, {Name: "old", Dir: far}, {Name: "again", Dir: elsewhere}}
	deny := promptStoreRules(accounts)
	if want := len(none) + 2*2*len(claudePromptStores); len(deny) != want {
		t.Errorf("%d rules, want %d", len(deny), want)
	}
	var edits []string
	for _, rule := range deny {
		if pattern, ok := strings.CutPrefix(rule, "Edit("); ok {
			edits = append(edits, "Read("+pattern)
		}
	}
	for _, account := range accounts {
		for _, store := range []string{"history.jsonl", "history.jsonl.tmp.42", "paste-cache/5f3a.txt", "projects/-work-api/s1.jsonl",
			"file-history/s1/3e1f@v1", "todos/s1.json", "sessions/4242.json", "debug/s1.txt", "backups/.claude.json.backup.1", ".claude.json"} {
			file := filepath.Join(account.Dir, filepath.FromSlash(store))
			if !readDenied(deny, homeDir, file) || !readDenied(edits, homeDir, file) {
				t.Errorf("%s isn't denied to Read and Edit", file)
			}
		}
		for _, kept := range []string{"plans/calm-river.md", "image-cache/s1/1.png", "uploads/a.pdf"} {
			if file := filepath.Join(account.Dir, filepath.FromSlash(kept)); readDenied(deny, homeDir, file) || readDenied(edits, homeDir, file) {
				t.Errorf("%s is denied", file)
			}
		}
	}
	for _, kept := range []string{filepath.Join(homeDir, "code", "projects", "api", "main.go"), filepath.Join(homeDir, "accounts", "projects", "x.md"),
		filepath.Join(homeDir, "notes", "history.jsonl"), filepath.Join(outside, "claude", "logs", "a.log")} {
		if readDenied(deny, homeDir, kept) || readDenied(edits, homeDir, kept) {
			t.Errorf("%s is denied", kept)
		}
	}
}
