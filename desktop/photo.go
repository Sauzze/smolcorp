package desktop

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"
)

// Send a photo (1.26 PH1): a photo from the phone (POST /photo) or an image pasted in the office window (POST
// /talk/photo) goes to one desk's agent, and nowhere else. The phone's comes over Tailscale only, never the Wi-Fi's
// plain http (answer 6); the window's from this computer only.
//
// Only a JPEG or a PNG is taken, decoded and encoded again with the standard library, so its EXIF, GPS and every
// other kind of metadata are gone: what's kept is the pixels. A picture bigger than photoSideMost a side or
// photoPixelsMost in all is refused from its header, before it is decoded, so a small file can't unpack into
// gigabytes. One is decoded at a time.
//
// It is kept, 0600, in <support>/photos/<folder>/, outside every repo (the folder says to git that nothing in it is
// to be added), deleted once its desk has read it (notePhotoRead), when the desk ends, after photoWait, or when the
// app starts again, whichever comes first: its path opens nothing for long. Answer 6's 24 hours is only the most a
// photo may be kept, so no copy is kept for them.
// <folder> is a random name the app gives the desk and keeps to itself, never its session id, and the photos folder
// can't be listed (photo_unlisted_*.go): another desk's agent can neither find a photo nor guess its path, while the one
// told the exact path opens it. The agent is told its path
// with typeText, every rule of the typed words included: never into a desk that asks. The path, and so the photo,
// never goes into the events log, the viewer, webhooks, push, summaries or the conversation: hidePhotos takes it
// out of everything those are made from.

const (
	// photoBodyMost is the most a photo's request may hold, and photoArrival how long it may take to arrive.
	photoBodyMost = 8 << 20
	photoArrival  = 30 * time.Second
	// A picture's size, at most: a side, and all its pixels (a 6K screen's screenshot fits).
	photoSideMost   = 8192
	photoPixelsMost = 25_000_000
	// photoMemoryMost is the most a picture may take once unpacked, as photoMemory reckons it from its header: a
	// 25 MP picture of 16 bits a channel fits; a progressive CMYK JPEG that size doesn't.
	photoMemoryMost = 200 << 20
	// How long a photo waits for its desk to read it, at most (well inside answer 6's 24 hours: photoNote says it),
	// and how often the photos are looked through.
	photoWait       = 30 * time.Minute
	photoSweepEvery = 10 * time.Minute
	// photoJPEGQuality is the quality a JPEG is encoded again at.
	photoJPEGQuality = 90
	// photosFolder is the photos' folder in the support folder, and photoPrefix how each photo's name starts.
	photosFolder = "photos"
	photoPrefix  = "smolcorp-photo-"
)

const (
	photoTooBig    = "Too big: a photo can be at most 8 MB."
	photoTooSlow   = "The photo took too long to arrive: at most 30 seconds."
	photoNotImage  = "Only a JPEG or PNG photo can be sent."
	photoTooLarge  = "Too large a picture: at most 8,192 pixels a side, 25 million in all."
	photoTooHeavy  = "That picture would take too much memory to open: send a smaller one."
	photoUnread    = "That photo couldn't be read."
	photoBusy      = "Busy with another photo: try again in a moment."
	photoNotKept   = "The photo couldn't be kept on this computer."
	noPictures     = "This agent can't take pictures."
	photoLookAt    = "Look at this image: "
	photoInTheText = "a photo"
)

// photoNote is what a photo sent to a desk of agent's means, said by the window beside its picture button (GET
// /talk's card, `photo.note`). Other agents can't list the photos, and its file goes once its desk has read it or after
// photoWait. Claude Code keeps the photo in the desk's transcript and its path in the prompt history: a hire's fence
// keeps its file tools out of both (photoFenceRules), but not its shell, and an agent without the fence
// (Codex, Gemini CLI, OpenCode, a desk you started yourself) can open any file you can.
func photoNote(agent Agent) string {
	if agent == Claude {
		return "Sent to Claude Code and its provider. The photo's file is deleted once this desk has opened it, or " +
			"after 30 minutes. Claude Code keeps the photo in this desk's history and its path in your prompt history: " +
			"Smolcorp's fence keeps a hire's file tools out of both, but not its shell, nor your agents " +
			"without the fence (Codex, Gemini CLI, OpenCode, and Claude Code desks you started yourself)."
	}
	name := agent.Name()
	return "Sent to " + name + " and its provider. " + name + " runs without Smolcorp's fence: it can open any file " +
		"you can, and whatever it keeps of the photo, your other agents can open too."
}

// photoDecoding lets one photo be decoded at a time: a decoded picture can take a few hundred MB (photoMemoryMost,
// and the decoder's own work besides).
var photoDecoding = make(chan struct{}, 1)

// POST /photo?session=: a paired phone's photo, over Tailscale only. POST /talk/photo?session=: the office window's,
// from this computer only. Both 8 MB and 30 seconds at most, a photo's body being the picture itself, with its type.
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/photo", access: accessPhoneSecure,
		refusal: says("Photos are sent only over Tailscale."), maxBody: photoBodyMost, timeout: photoArrival,
		serve: func(d *Deck, w http.ResponseWriter, r *request, session string) {
			writeStatus(w)(d.sendPhoto(textFromPhone, r, session))
		}})
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/talk/photo", access: accessWindow,
		refusal: func(d *Deck) string { return d.shownOnlyInTheWindow("Pasting a picture") }, maxBody: photoBodyMost,
		timeout: photoArrival, serve: func(d *Deck, w http.ResponseWriter, r *request, session string) {
			writeStatus(w)(d.sendPhoto(textFromWindow, r, session))
		}})
}

// takesPictures says whether an agent looks at an image whose path it's given: Claude Code, Codex, Gemini CLI and
// OpenCode do. Any other is told the photo can't go, before it's sent.
func takesPictures(agent Agent) bool {
	return agent == Claude || agent == Codex || agent == Gemini || agent == OpenCode
}

// deskPhoto is whether the window can send a picture to a desk now, for the card GET /talk gives (`photo`): from
// the card's typing, and the agent's taking pictures at all.
type deskPhoto struct {
	OK  bool   `json:"ok"`
	Why string `json:"why"`
	// Note is what sending it a photo means (photoNote), for an agent that takes pictures.
	Note string `json:"note,omitempty"`
}

// photoTaking is what the card says of a desk whose typing is typing and whose agent is agent.
func photoTaking(typing deskTyping, agent Agent) deskPhoto {
	switch {
	case !typing.OK && !takesPictures(agent):
		return deskPhoto{Why: typing.Why}
	case !takesPictures(agent):
		return deskPhoto{Why: noPictures}
	case !typing.OK:
		return deskPhoto{Why: typing.Why, Note: photoNote(agent)}
	}
	return deskPhoto{OK: true, Note: photoNote(agent)}
}

// sendPhoto is POST /photo and POST /talk/photo: the picture in r's body kept for session id's desk, and its path
// typed into its agent. The desk is checked before the body is read, and the path is typed by typeText, which
// checks everything again; a photo whose words weren't typed is deleted at once.
func (d *Deck) sendPhoto(from textFrom, r *request, id string) (int, string) {
	d.mu.Lock()
	on := d.config.PhoneMicEnabled
	d.mu.Unlock()
	if from == textFromPhone && !on {
		return http.StatusForbidden, phoneTextOff
	}
	aim, status, why := d.typeTarget(from, id)
	if why != "" {
		return status, why
	}
	if !takesPictures(aim.agent) {
		return http.StatusConflict, noPictures
	}
	data, err := io.ReadAll(r.Body)
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		return http.StatusRequestEntityTooLarge, photoTooBig
	case err != nil:
		return http.StatusRequestTimeout, photoTooSlow
	}
	select {
	case photoDecoding <- struct{}{}:
	default:
		return http.StatusTooManyRequests, photoBusy
	}
	clean, extension, status, why := cleanPhoto(r.Header.Get("Content-Type"), data)
	<-photoDecoding
	if why != "" {
		return status, why
	}
	// The desk's folder: the one it has, or a new random name, kept by the deck before anything is made there so its
	// end deletes it.
	d.mu.Lock()
	if d.sessions.Get(id) == nil {
		d.mu.Unlock()
		return http.StatusNotFound, "Session not found"
	}
	folder := d.photoDesks[id]
	if folder == "" {
		folder = randomHex(16)
		if d.photoDesks == nil {
			d.photoDesks = map[string]string{}
		}
		d.photoDesks[id] = folder
	}
	d.mu.Unlock()
	path, err := keepPhoto(supportDir, folder, extension, clean)
	if err != nil {
		return http.StatusInternalServerError, photoNotKept
	}
	d.mu.Lock()
	// Ended while its photo was made: its folder goes at once (its end may have come before the folder was made).
	if d.sessions.Get(id) == nil {
		d.mu.Unlock()
		removePhotoFolder(supportDir, folder)
		return http.StatusNotFound, "Session not found"
	}
	d.mu.Unlock()
	d.rounds.after(photoWait, d.sweepPhotos)
	send := true
	// In quotes: the support folder can hold a space ("Application Support", a Windows user's name).
	status, message := d.typeText(from, photoLookAt+`"`+path+`"`, id, &send)
	// Refused before a word went: the photo goes too. Typed and not sent, or maybe typed, it stays for the desk.
	if status >= http.StatusBadRequest && status < http.StatusInternalServerError && message != from.typedNotSent() {
		_ = os.Remove(path)
	}
	return status, message
}

// cleanPhoto is the picture in data as a new file of the same type, with nothing but its pixels, and its extension;
// or the status and sentence that refuse it. contentType must say the type the bytes are.
func cleanPhoto(contentType string, data []byte) ([]byte, string, int, string) {
	kind, _, _ := mime.ParseMediaType(contentType)
	var config func(io.Reader) (image.Config, error)
	var decode func(io.Reader) (image.Image, error)
	var extension string
	switch {
	case kind == "image/jpeg" && bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		config, decode, extension = jpeg.DecodeConfig, jpeg.Decode, ".jpg"
	case kind == "image/png" && bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		config, decode, extension = png.DecodeConfig, png.Decode, ".png"
	default:
		return nil, "", http.StatusUnsupportedMediaType, photoNotImage
	}
	// The size its header gives, before anything is unpacked.
	size, err := config(bytes.NewReader(data))
	switch {
	case err != nil:
		return nil, "", http.StatusBadRequest, photoUnread
	case size.Width <= 0 || size.Height <= 0:
		return nil, "", http.StatusBadRequest, photoUnread
	case size.Width > photoSideMost || size.Height > photoSideMost || size.Width*size.Height > photoPixelsMost:
		return nil, "", http.StatusRequestEntityTooLarge, photoTooLarge
	case photoMemory(size, extension == ".jpg" && progressiveJPEG(data)) > photoMemoryMost:
		return nil, "", http.StatusRequestEntityTooLarge, photoTooHeavy
	}
	picture, err := decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", http.StatusBadRequest, photoUnread
	}
	if bounds := picture.Bounds(); bounds.Dx() != size.Width || bounds.Dy() != size.Height {
		return nil, "", http.StatusBadRequest, photoUnread
	}
	var out bytes.Buffer
	if extension == ".jpg" {
		err = jpeg.Encode(&out, picture, &jpeg.Options{Quality: photoJPEGQuality})
	} else {
		err = (&png.Encoder{CompressionLevel: png.DefaultCompression}).Encode(&out, picture)
	}
	if err != nil {
		return nil, "", http.StatusBadRequest, photoUnread
	}
	return out.Bytes(), extension, 0, ""
}

// photoMemory is about how many bytes a picture of size takes once decoded: its pixels, at the bytes a pixel of its
// colours takes, and for a progressive JPEG the 4-byte coefficients the decoder keeps for each of its channels too.
func photoMemory(size image.Config, progressive bool) int {
	perPixel, channels := 4, 3
	switch size.ColorModel {
	case color.GrayModel:
		perPixel, channels = 1, 1
	case color.Gray16Model:
		perPixel, channels = 2, 1
	case color.YCbCrModel:
		perPixel = 3
	case color.CMYKModel:
		channels = 4
	case color.RGBA64Model, color.NRGBA64Model:
		perPixel = 8
	}
	if _, paletted := size.ColorModel.(color.Palette); paletted {
		perPixel = 1
	}
	if progressive {
		perPixel += 4 * channels
	}
	return size.Width * size.Height * perPixel
}

// progressiveJPEG says whether the JPEG in data has a progressive frame header: its segments are walked up to the
// first scan.
func progressiveJPEG(data []byte) bool {
	for at := 2; at+4 <= len(data) && data[at] == 0xFF; {
		switch marker := data[at+1]; {
		case marker == 0xC2 || marker == 0xC6 || marker == 0xCA || marker == 0xCE:
			return true
		case marker == 0xDA:
			return false
		case marker == 0xFF:
			at++
			continue
		}
		at += 2 + (int(data[at+2])<<8 | int(data[at+3]))
	}
	return false
}

// photosFolderReady makes support's photos folder if it isn't there, refuses one that isn't a folder (a link put in
// its place, say), and has it unlisted again, as it must always be: a sweep that stopped half way leaves it listed.
func photosFolderReady(support string) (string, error) {
	photos := filepath.Join(support, photosFolder)
	if err := os.MkdirAll(support, 0o700); err != nil {
		return "", err
	}
	if err := os.Mkdir(photos, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err
	}
	if info, err := os.Lstat(photos); err != nil || !info.IsDir() {
		return "", errors.New("the photos folder isn't a folder")
	}
	return photos, unlistPhotos(photos, true)
}

// openDeskFolder is the desk's photo folder folder in photos, opened as a root so nothing in it is followed out;
// nil when it isn't a folder of its own there (a link, a file, nothing).
func openDeskFolder(photos, folder string) *os.Root {
	desk := filepath.Join(photos, folder)
	before, err := os.Lstat(desk)
	if err != nil || !before.IsDir() {
		return nil
	}
	root, err := os.OpenRoot(desk)
	if err != nil {
		return nil
	}
	if now, err := root.Stat("."); err != nil || !os.SameFile(before, now) {
		root.Close()
		return nil
	}
	return root
}

// keepPhoto writes data as a new photo in folder, a desk's folder in support's photos folder, private to you, and
// gives its path. Each name is new and random, and opened only if it doesn't exist; nothing is followed out of the
// desk's folder (os.Root), so a link put there can't send a photo anywhere else.
func keepPhoto(support, folder, extension string, data []byte) (string, error) {
	photos, err := photosFolderReady(support)
	if err != nil {
		return "", err
	}
	// git, in a repo that holds the support folder (a home folder kept in git), is told to add nothing here.
	if ignore, err := os.OpenFile(filepath.Join(photos, ".gitignore"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600); err == nil {
		_, _ = ignore.WriteString("*\n")
		ignore.Close()
	}
	if err := os.Mkdir(filepath.Join(photos, folder), 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err
	}
	root := openDeskFolder(photos, folder)
	if root == nil {
		return "", errors.New("the desk's photo folder isn't a folder")
	}
	defer root.Close()
	name := photoPrefix + randomHex(16) + extension
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	_, err = f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = root.Remove(name)
		return "", err
	}
	return filepath.Join(photos, folder, name), nil
}

// forgetEndedPhotos deletes the photos of the desks that have ended since the deck last changed. With the lock held;
// the files go on the deck's rounds.
func (d *Deck) forgetEndedPhotos() {
	var ended []string
	for id, folder := range d.photoDesks {
		if d.sessions.Get(id) == nil {
			ended = append(ended, folder)
			delete(d.photoDesks, id)
		}
	}
	if len(ended) == 0 {
		return
	}
	support := supportDir
	d.rounds.run(func() {
		for _, folder := range ended {
			removePhotoFolder(support, folder)
		}
	})
}

// removePhotoFolder deletes folder, a desk's photo folder or anything else in the photos folder, never anything
// outside it: a link is removed itself, not followed. The photos folder can't be listed, so its RemoveAll can't be
// used; a desk's folder holds files only, each removed inside its root.
func removePhotoFolder(support, folder string) {
	if folder == "" || folder == "." || folder == ".." || strings.ContainsAny(folder, `/\`) {
		return
	}
	photos := filepath.Join(support, photosFolder)
	if root := openDeskFolder(photos, folder); root != nil {
		if dir, err := root.Open("."); err == nil {
			names, _ := dir.Readdirnames(-1)
			dir.Close()
			for _, name := range names {
				_ = root.RemoveAll(name)
			}
		}
		root.Close()
	}
	_ = os.Remove(filepath.Join(photos, folder))
}

// sweepPhotos deletes every photo older than photoWait, and the folders of desks no longer here. Without the lock.
// The deck knows its desks' folders: the photos folder is never listed here.
func (d *Deck) sweepPhotos() {
	d.mu.Lock()
	var here, gone []string
	for id, folder := range d.photoDesks {
		if d.sessions.Get(id) == nil {
			gone = append(gone, folder)
			delete(d.photoDesks, id)
		} else {
			here = append(here, folder)
		}
	}
	d.mu.Unlock()
	sweepPhotosIn(supportDir, here, gone, time.Now())
}

// sweepPhotosIn is sweepPhotos for the photos folder in support: the photos in the folders here older than photoWait
// at now, and the folders gone, deleted; the photos folder unlisted again.
func sweepPhotosIn(support string, here, gone []string, now time.Time) {
	photos := filepath.Join(support, photosFolder)
	if info, err := os.Lstat(photos); err != nil || !info.IsDir() {
		return
	}
	_ = unlistPhotos(photos, true)
	for _, folder := range gone {
		removePhotoFolder(support, folder)
	}
	for _, folder := range here {
		root := openDeskFolder(photos, folder)
		if root == nil {
			continue
		}
		if dir, err := root.Open("."); err == nil {
			files, _ := dir.ReadDir(-1)
			dir.Close()
			for _, file := range files {
				info, err := file.Info()
				if err != nil || !info.Mode().IsRegular() || now.Sub(info.ModTime()) >= photoWait {
					_ = root.RemoveAll(file.Name())
				}
			}
		}
		root.Close()
	}
}

// clearPhotos deletes every photo in support's photos folder, as the app starts: the folders' names went with the
// last run, and a desk restored is sent its photos again. The photos folder is listed only here, and only for that
// one read; it is unlisted again before anything is deleted, and at once if the app stops half way (photosFolderReady,
// sweepPhotosIn and the next start).
func clearPhotos(support string) {
	photos := filepath.Join(support, photosFolder)
	info, err := os.Lstat(photos)
	if err != nil {
		return
	}
	if !info.IsDir() {
		_ = os.Remove(photos)
		return
	}
	var names []string
	if unlistPhotos(photos, false) == nil {
		if dir, err := os.Open(photos); err == nil {
			names, _ = dir.Readdirnames(-1)
			dir.Close()
		}
	}
	_ = unlistPhotos(photos, true)
	for _, name := range names {
		if name != ".gitignore" {
			removePhotoFolder(support, name)
		}
	}
}

// photoFenceRules are the deny rules a hire's settings get for the photos (hiresettings.go), beside deny's, for a hire
// whose Claude accounts are profiles: nothing in the photos folder can be changed, and no prompt store, where another
// desk's photo's path is written, can be read or changed (promptStoreRules). Reading the photos isn't denied by name: a
// desk's own photo must open, and no rule can say "all but its own"; the others' are kept by the folder that can't be
// listed, their random names, and their going once read.
func photoFenceRules(support string, profiles []claudeProfile, deny []string) []string {
	have := map[string]bool{}
	for _, rule := range deny {
		have[rule] = true
	}
	var rules []string
	for _, rule := range append([]string{"Edit(" + absoluteRulePath(support) + "/" + photosFolder + "/**)"}, promptStoreRules(profiles)...) {
		if !have[rule] {
			have[rule] = true
			rules = append(rules, rule)
		}
	}
	return rules
}

// claudePromptStores are where Claude Code writes what is typed into a desk, a photo's path with it, or words its
// agent wrote from it, in its config folder (~/.claude, or a profile's CLAUDE_CONFIG_DIR): its own list of a project's
// data (the prompt history and its staging copies, the transcripts, the tasks, debug logs and file history of a
// session, and the config whose project entries keep a history, with its backups), and the other stores of its own
// beside them that can hold a prompt or a tool's input. Claude Code reads and writes them itself, never through a
// tool, so denying them to a hire's tools takes nothing from it. Left readable: plans (the desk's own plan, which it
// writes with Write), image-cache and uploads (pictures pasted or attached in its own pane, which it opens with Read;
// a photo sent here is typed as a path and never becomes one), and stores that hold no prompt (shell-snapshots,
// session-env, ide, statsig, cache, plugins, skills…).
var claudePromptStores = []string{
	"*history.jsonl*", "paste-cache/**", "projects/**", "todos/**", "tasks/**", "file-history/**", "sessions/**",
	"jobs/**", "teams/**", "debug/**", "logs/**", "traces/**", "telemetry/**", "dump-prompts/**", "api-dumps/**",
	"feedback-bundles/**", "usage-data/**", "backups/**", ".claude*.json*",
}

// otherPromptStores are the other agents' prompt histories and sessions, where a photo sent to one of their desks has
// its path: Codex's, Gemini CLI's and OpenCode's, in their own folders.
var otherPromptStores = []string{
	"~/.codex/history.jsonl", "~/.codex/sessions/**", "~/.codex/archived_sessions/**", "~/.codex/log/**",
	"~/.gemini/tmp/**", "~/.local/share/opencode/**", "~/.local/state/opencode/**",
}

// promptStoreRules deny a hire's tools every prompt store (claudePromptStores, otherPromptStores), reading and
// changing alike. A desk's settings ride in its command line, which holds at most 32767 characters on Windows, so
// they are few whatever the number of accounts: one per store for ~/.claude and every ~/.claude-<name> at once
// (~/.claude*/), with its .claude.json beside them, and more only for an account whose folder is elsewhere
// (claudeStoreFolder). A Read rule keeps Grep and Glob out too: Claude Code leaves what it denies out of their search.
func promptStoreRules(profiles []claudeProfile) []string {
	patterns := []string{"~/.claude*.json*"}
	folders := []string{"~/.claude*"}
	for _, profile := range profiles {
		if folder, ok := claudeStoreFolder(profile.Dir); ok && !slices.Contains(folders, folder) {
			folders = append(folders, folder)
		}
	}
	for _, folder := range folders {
		for _, store := range claudePromptStores {
			patterns = append(patterns, folder+"/"+store)
		}
	}
	patterns = append(patterns, otherPromptStores...)
	var rules []string
	for _, pattern := range patterns {
		rules = append(rules, "Read("+pattern+")", "Edit("+pattern+")")
	}
	return rules
}

// claudeStoreFolder is how a hire's rules name the folder of an account (a CLAUDE_CONFIG_DIR, which can be any
// folder) beside ~/.claude*/: none for ~/.claude or a ~/.claude-<name>, which that covers; from ~ for one elsewhere in
// the home folder (~/accounts/work); and for one outside it, whose full path can be of any length, by its last two
// folders anywhere on its drive (//**/claude/work, //d/**/claude/work on Windows), which takes in that folder and
// any other of the same two names. ok is false for a folder that isn't a full path. Claude Code roots ~ at the hire's
// HOME, which no launch sets, so it is homeDir: a launcher that set another would have to name these folders in full.
func claudeStoreFolder(dir string) (folder string, ok bool) {
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) || strings.ContainsRune(dir, 0) {
		return "", false
	}
	if home := filepath.Clean(homeDir); filepath.IsAbs(home) {
		if rel, err := filepath.Rel(home, dir); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			rel = filepath.ToSlash(rel)
			switch {
			case rel == ".":
				return "~", true
			case !strings.Contains(rel, "/") && strings.HasPrefix(rel, ".claude"):
				return "", false
			}
			return "~/" + rel, true
		}
	}
	path := strings.TrimPrefix(absoluteRulePath(dir), "//")
	drive := ""
	if len(dir) >= 2 && dir[1] == ':' {
		drive, path, _ = strings.Cut(path, "/")
		drive += "/"
	}
	names := strings.Split(path, "/")
	if len(names) <= 2 {
		// A drive's or the disk's root (D:\, /) without its last slash: the store is written after one.
		return strings.TrimSuffix(absoluteRulePath(dir), "/"), true
	}
	return "//" + drive + "**/" + strings.Join(names[len(names)-2:], "/"), true
}

// photoReadTools are the tools that open a file, as each agent names them: Claude Code's Read, Gemini CLI's
// read_file and read_many_files, OpenCode's read and Codex's view_image.
var photoReadTools = map[string]bool{"read": true, "read_file": true, "read_many_files": true, "view_image": true}

// photoFile is a photo's file name, whole.
var photoFile = regexp.MustCompile(`^` + photoPrefix + `[0-9a-f]{32}\.(?:jpg|png)$`)

// notePhotoRead deletes a desk's photo once the desk has read it: event is one of its tools having opened a file
// (PostToolUse), and a path in its input is a photo in the desk's own folder. Another desk's reading it changes
// nothing. With the lock held; the file goes on the deck's rounds.
func (d *Deck) notePhotoRead(event map[string]any) {
	if event["hook_event_name"] != "PostToolUse" {
		return
	}
	tool, _ := event["tool_name"].(string)
	id, _ := event["session_id"].(string)
	folder := d.photoDesks[id]
	if !photoReadTools[strings.ToLower(tool)] || folder == "" || !filepath.IsAbs(supportDir) {
		return
	}
	desk := filepath.Join(supportDir, photosFolder, folder)
	var read []string
	var look func(value any)
	look = func(value any) {
		switch value := value.(type) {
		case string:
			path := filepath.Clean(value)
			same := filepath.Dir(path) == desk || runtime.GOOS == "windows" && strings.EqualFold(filepath.Dir(path), desk)
			if same && photoFile.MatchString(filepath.Base(path)) {
				read = append(read, filepath.Base(path))
			}
		case []any:
			for _, item := range value {
				look(item)
			}
		case map[string]any:
			for _, item := range value {
				look(item)
			}
		}
	}
	look(event["tool_input"])
	if len(read) == 0 {
		return
	}
	photos := filepath.Join(supportDir, photosFolder)
	d.rounds.run(func() {
		if root := openDeskFolder(photos, folder); root != nil {
			for _, name := range read {
				_ = root.Remove(name)
			}
			root.Close()
		}
	})
}

// photoName is a photo's file name, wherever its path is written.
var photoName = regexp.MustCompile(`[^\s"'` + "`" + `]*` + photoPrefix + `[0-9a-f]{32}\.(?:jpg|png)`)

// hidePhotos is text with every photo's path, and every path into the photos folder, said as "a photo": for what
// leaves the agent's own conversation, the events log, a desk's activity, a prompt's label and the summaries.
func hidePhotos(text string) string {
	photos := filepath.Join(supportDir, photosFolder)
	written := []string{photos, filepath.ToSlash(photos), strings.ReplaceAll(photos, `\`, `\\`)}
	if !filepath.IsAbs(supportDir) {
		written = nil
	}
	for _, written := range written {
		for start := strings.Index(text, written); start >= 0; start = strings.Index(text, written) {
			end := start + len(written)
			for end < len(text) && !strings.ContainsRune(" \t\r\n\"'`", rune(text[end])) {
				end++
			}
			text = text[:start] + photoInTheText + text[end:]
		}
	}
	if strings.Contains(text, photoPrefix) {
		text = photoName.ReplaceAllString(text, photoInTheText)
	}
	return text
}
